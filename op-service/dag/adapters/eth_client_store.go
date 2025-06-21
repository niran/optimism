package adapters

import (
	"context"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-service/dag"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/sources"
)

// EthClientLike defines the interface needed for the dag Store adapter
type EthClientLike interface {
	BlockRefByHash(ctx context.Context, hash common.Hash) (eth.BlockRef, error)
	BlockRefByNumber(ctx context.Context, number uint64) (eth.BlockRef, error)
}

// EthClientStore implements the dag.Store interface using an EthClient
type EthClientStore struct {
	client EthClientLike
	ctx    context.Context
	log    log.Logger
}

// NewEthClientStore creates a new dag Store adapter for EthClient
func NewEthClientStore(ctx context.Context, client EthClientLike, log log.Logger) *EthClientStore {
	return &EthClientStore{
		client: client,
		ctx:    ctx,
		log:    log,
	}
}

// Node retrieves a block node by its hash
func (s *EthClientStore) Node(id common.Hash) (dag.DAGNode[common.Hash], bool) {
	blockRef, err := s.client.BlockRefByHash(s.ctx, id)
	if err != nil {
		s.log.Debug("Failed to fetch block by hash", "hash", id, "error", err)
		return nil, false
	}

	node := &Block{
		BlockRef: blockRef,
	}

	return node, true
}

// NodesAtDepth returns the canonical block at the specified depth (block number)
func (s *EthClientStore) NodesAtDepth(depth uint64) (map[common.Hash]struct{}, error) {
	// For blockchain, depth corresponds to block number
	// There's typically only one canonical block per number
	blockRef, err := s.client.BlockRefByNumber(s.ctx, depth)
	if err != nil {
		s.log.Debug("Failed to fetch block by number", "number", depth, "error", err)
		// Return empty set rather than error - this depth might not exist yet
		return make(map[common.Hash]struct{}), nil
	}

	result := make(map[common.Hash]struct{})
	result[blockRef.Hash] = struct{}{}

	s.log.Trace("Found canonical block at depth", "depth", depth, "hash", blockRef.Hash)
	return result, nil
}

// WithContext creates a store with a specific context
func (s *EthClientStore) WithContext(ctx context.Context) *EthClientStore {
	return &EthClientStore{
		client: s.client,
		ctx:    ctx,
		log:    s.log,
	}
}

// Verify interface compliance at compile time
var _ dag.Store[common.Hash] = (*EthClientStore)(nil)
var _ dag.DAGNode[common.Hash] = (*Block)(nil)
var _ EthClientLike = (*sources.EthClient)(nil)
