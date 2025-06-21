package adapters

import (
	"github.com/ethereum-optimism/optimism/op-service/dag"
	"github.com/ethereum/go-ethereum/common"
)

// Blockchain-specific convenience functions

// CommonAncestors finds blocks that are ancestors of ALL given blocks.
// This is useful for finding merge points in blockchain forks.
func CommonAncestors(hashes ...common.Hash) dag.NodeSet[common.Hash] {
	if len(hashes) == 0 {
		return dag.Union[common.Hash]()
	}

	sets := make([]dag.NodeSet[common.Hash], len(hashes))
	for i, hash := range hashes {
		sets[i] = dag.Ancestors(dag.ID(hash))
	}
	return dag.Intersect(sets...)
}

// CommonDescendants finds blocks that are descendants of ALL given blocks.
// This is less common in blockchain scenarios but useful for certain analyses.
func CommonDescendants(hashes ...common.Hash) dag.NodeSet[common.Hash] {
	if len(hashes) == 0 {
		return dag.Union[common.Hash]()
	}

	sets := make([]dag.NodeSet[common.Hash], len(hashes))
	for i, hash := range hashes {
		sets[i] = dag.Descendants(dag.ID(hash))
	}
	return dag.Intersect(sets...)
}

// AnyAncestors finds blocks that are ancestors of ANY of the given blocks.
// This is useful for finding all blocks that could have influenced a set of blocks.
func AnyAncestors(hashes ...common.Hash) dag.NodeSet[common.Hash] {
	if len(hashes) == 0 {
		return dag.Union[common.Hash]()
	}

	sets := make([]dag.NodeSet[common.Hash], len(hashes))
	for i, hash := range hashes {
		sets[i] = dag.Ancestors(dag.ID(hash))
	}
	return dag.Union(sets...)
}

// AnyDescendants finds blocks that are descendants of ANY of the given blocks.
// This is useful for finding all blocks that could be affected by a set of blocks.
func AnyDescendants(hashes ...common.Hash) dag.NodeSet[common.Hash] {
	if len(hashes) == 0 {
		return dag.Union[common.Hash]()
	}

	sets := make([]dag.NodeSet[common.Hash], len(hashes))
	for i, hash := range hashes {
		sets[i] = dag.Descendants(dag.ID(hash))
	}
	return dag.Union(sets...)
}
