package adapters

import (
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum/common"
)

// Block implements the DAGNode interface for blockchain blocks
type Block struct {
	BlockRef eth.BlockRef
}

// ID returns the block hash as the node identifier
func (b *Block) ID() common.Hash {
	return b.BlockRef.Hash
}

// Parents returns the parent block hash as a slice
func (b *Block) Parents() []common.Hash {
	if b.BlockRef.ParentHash == (common.Hash{}) {
		return []common.Hash{} // Genesis block has no parents
	}
	return []common.Hash{b.BlockRef.ParentHash}
}

// Depth returns the block number as the depth from genesis
// In blockchain contexts, block number naturally represents depth
func (b *Block) Depth() uint64 {
	return b.BlockRef.Number
}
