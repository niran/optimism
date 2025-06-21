package adapters

import (
	"github.com/ethereum-optimism/optimism/op-service/dag"
	"github.com/ethereum/go-ethereum/common"
)

// findBlockchainLCA efficiently finds the latest common ancestor of the given blockchain hashes.
// This uses a blockchain-specific O(d) algorithm optimized for tree structures.
func findBlockchainLCA(store dag.Store[common.Hash], hashes []common.Hash) (common.Hash, bool) {
	if len(hashes) == 0 {
		return common.Hash{}, false
	}
	if len(hashes) == 1 {
		return hashes[0], true
	}

	// For blockchain trees, we can use a much more efficient algorithm
	// than the general DAG approach

	// Start with the first two hashes
	lca := hashes[0]
	for i := 1; i < len(hashes); i++ {
		var found bool
		lca, found = findLCAOfTwo(store, lca, hashes[i])
		if !found {
			return common.Hash{}, false // No common ancestor exists
		}
	}

	return lca, true
}

// findLCAOfTwo efficiently finds the LCA of exactly two blockchain nodes using O(d) algorithm
func findLCAOfTwo(store dag.Store[common.Hash], hash1, hash2 common.Hash) (common.Hash, bool) {
	if hash1 == hash2 {
		return hash1, true
	}

	// Get nodes and their depths
	node1, ok1 := store.Node(hash1)
	node2, ok2 := store.Node(hash2)
	if !ok1 || !ok2 {
		return common.Hash{}, false
	}

	depth1 := node1.Depth()
	depth2 := node2.Depth()

	// Walk both nodes backwards until they meet
	current1, current2 := hash1, hash2

	// First, bring both nodes to the same depth
	for depth1 > depth2 {
		parent, hasParent := getParent(store, current1)
		if !hasParent {
			return common.Hash{}, false // Reached genesis without finding common ancestor
		}
		current1 = parent
		depth1--
	}

	for depth2 > depth1 {
		parent, hasParent := getParent(store, current2)
		if !hasParent {
			return common.Hash{}, false // Reached genesis without finding common ancestor
		}
		current2 = parent
		depth2--
	}

	// Now both are at the same depth, walk backwards until they meet
	for current1 != current2 {
		parent1, hasParent1 := getParent(store, current1)
		parent2, hasParent2 := getParent(store, current2)

		if !hasParent1 || !hasParent2 {
			return common.Hash{}, false // Reached genesis without finding common ancestor
		}

		current1 = parent1
		current2 = parent2
	}

	return current1, true // They met at the LCA
}

// getParent is a helper function to get the single parent of a blockchain node
func getParent(store dag.Store[common.Hash], hash common.Hash) (common.Hash, bool) {
	node, ok := store.Node(hash)
	if !ok {
		return common.Hash{}, false
	}

	parents := node.Parents()
	if len(parents) == 0 {
		return common.Hash{}, false // Genesis block
	}

	// Blockchain assumption: single parent
	return parents[0], true
}
