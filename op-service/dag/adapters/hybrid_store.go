package adapters

import (
	"context"

	"github.com/ethereum-optimism/optimism/op-service/dag"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
)

// HybridStore combines local indexing with RPC client fallback.
// This is useful for scenarios where certain blocks (like reorged blocks)
// need to be indexed locally because they're not reliably retrievable from RPC.
type HybridStore struct {
	localIndex map[common.Hash]*Block // Local index for non-canonical/reorged blocks
	rpcClient  EthClientLike          // Fallback to RPC for canonical blocks
	ctx        context.Context
	log        log.Logger
}

// NewHybridStore creates a store that checks local index first, then falls back to RPC
func NewHybridStore(ctx context.Context, rpcClient EthClientLike, logger log.Logger) *HybridStore {
	return &HybridStore{
		localIndex: make(map[common.Hash]*Block),
		rpcClient:  rpcClient,
		ctx:        ctx,
		log:        logger,
	}
}

// IndexBlock adds a block to the local index (e.g., for reorged blocks)
func (hs *HybridStore) IndexBlock(block *Block) {
	hs.localIndex[block.ID()] = block
	hs.log.Debug("Indexed block locally", "hash", block.ID(), "number", block.BlockRef.Number)
}

// IndexBlockRef is a convenience method to index from a BlockRef
func (hs *HybridStore) IndexBlockRef(ref eth.BlockRef) {
	block := &Block{BlockRef: ref}
	hs.IndexBlock(block)
}

// RemoveFromIndex removes a block from local index (e.g., when it becomes canonical again)
func (hs *HybridStore) RemoveFromIndex(hash common.Hash) {
	delete(hs.localIndex, hash)
	hs.log.Debug("Removed block from local index", "hash", hash)
}

// Node implements the Store interface with hybrid lookup
func (hs *HybridStore) Node(id common.Hash) (dag.DAGNode[common.Hash], bool) {
	// First check local index
	if block, exists := hs.localIndex[id]; exists {
		hs.log.Trace("Found block in local index", "hash", id)
		return block, true
	}

	// Fallback to RPC client
	ref, err := hs.rpcClient.BlockRefByHash(hs.ctx, id)
	if err != nil {
		hs.log.Trace("Block not found via RPC", "hash", id, "error", err)
		return nil, false
	}

	block := &Block{BlockRef: ref}
	hs.log.Trace("Retrieved block via RPC", "hash", id, "number", ref.Number)
	return block, true
}

// NodesAtDepth returns nodes at the specified depth from both local index and RPC
// This includes both canonical blocks and locally indexed reorged blocks
func (hs *HybridStore) NodesAtDepth(depth uint64) (map[common.Hash]struct{}, error) {
	result := make(map[common.Hash]struct{})

	// First, get locally indexed blocks at this depth
	localCount := 0
	for hash, block := range hs.localIndex {
		if block.BlockRef.Number == depth {
			result[hash] = struct{}{}
			localCount++
		}
	}

	// Then, get the canonical block at this depth via RPC
	canonicalRef, err := hs.rpcClient.BlockRefByNumber(hs.ctx, depth)
	if err == nil {
		// Only add if it's not already in our local index
		if _, alreadyLocal := result[canonicalRef.Hash]; !alreadyLocal {
			result[canonicalRef.Hash] = struct{}{}
		}
		hs.log.Trace("Found blocks at depth", "depth", depth, "local", localCount, "canonical", canonicalRef.Hash, "total", len(result))
	} else {
		hs.log.Trace("Found locally indexed blocks at depth", "depth", depth, "local", localCount, "canonical_error", err)
	}

	return result, nil
}

// Query creates a StoreQuery for the hybrid store
func (hs *HybridStore) Query(hash common.Hash) *StoreQuery {
	return &StoreQuery{
		store: &EthClientStore{
			client: hs.rpcClient,
			ctx:    hs.ctx,
			log:    hs.log,
		},
		hash: hash,
	}
}

// GetLocallyIndexedBlocks returns all blocks in the local index
func (hs *HybridStore) GetLocallyIndexedBlocks() map[common.Hash]*Block {
	result := make(map[common.Hash]*Block)
	for hash, block := range hs.localIndex {
		result[hash] = block
	}
	return result
}

// IsLocallyIndexed checks if a block is in the local index
func (hs *HybridStore) IsLocallyIndexed(hash common.Hash) bool {
	_, exists := hs.localIndex[hash]
	return exists
}
