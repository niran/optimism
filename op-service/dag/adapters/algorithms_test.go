package adapters

import (
	"context"
	"fmt"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/op-service/testlog"
)

func TestLatestCommonAncestorAlgorithms(t *testing.T) {
	logger := testlog.Logger(t, log.LevelDebug)
	ctx := context.Background()

	// Create a diamond DAG:
	//     0
	//    / \
	//   1   2
	//   |   |
	//   3   4
	//    \ /
	//     5
	mockClient := NewMockEthClient()
	hashes := make([]common.Hash, 6)
	for i := 0; i < 6; i++ {
		hashes[i] = common.HexToHash(fmt.Sprintf("0x700%d", i))
	}

	mockClient.AddBlock(0, hashes[0], common.Hash{}) // Genesis
	mockClient.AddBlock(1, hashes[1], hashes[0])     // Left branch
	mockClient.AddBlock(1, hashes[2], hashes[0])     // Right branch (same depth)
	mockClient.AddBlock(2, hashes[3], hashes[1])     // Continue left
	mockClient.AddBlock(2, hashes[4], hashes[2])     // Continue right
	mockClient.AddBlock(3, hashes[5], hashes[3])     // Merge (using left parent for simplicity)

	store := NewEthClientStore(ctx, mockClient, logger)

	t.Run("BlockQuery LCA methods", func(t *testing.T) {
		// LCA of blocks 3 and 4 should be block 0
		lca, found := From(hashes[3]).LatestCommonAncestor(store, hashes[4])
		require.True(t, found)
		require.Equal(t, hashes[0], lca)

		// LCA of blocks 1 and 3 should be block 1 (same branch)
		lca2, found2 := From(hashes[1]).LatestCommonAncestor(store, hashes[3])
		require.True(t, found2)
		require.Equal(t, hashes[1], lca2)
	})

	t.Run("StoreQuery LCA methods", func(t *testing.T) {
		// LCA using store query
		lca, found := store.Query(hashes[3]).LatestCommonAncestor(hashes[4])
		require.True(t, found)
		require.Equal(t, hashes[0], lca)
	})

	t.Run("DSL-level LCA convenience methods", func(t *testing.T) {
		// Test LCA using DSL
		lca, found := store.Query(hashes[3]).LatestCommonAncestor(hashes[4])
		require.True(t, found)
		require.Equal(t, hashes[0], lca)

		// Test with single block (should return itself)
		single, foundSingle := store.Query(hashes[2]).LatestCommonAncestor()
		require.True(t, foundSingle)
		require.Equal(t, hashes[2], single)

		// Test with multiple blocks
		multi, foundMulti := store.Query(hashes[3]).LatestCommonAncestor(hashes[4], hashes[5])
		require.True(t, foundMulti)
		require.Equal(t, hashes[0], multi)
	})

	t.Run("BlockBuilder with LCA", func(t *testing.T) {
		// Test AddLatestCommonAncestor
		builder := NewBlockBuilder().
			Add(hashes[5]).
			AddLatestCommonAncestor(hashes[3], hashes[4])

		querySet := builder.QueryWith(store)
		require.True(t, querySet.Contains(hashes[5])) // From Add
		require.True(t, querySet.Contains(hashes[0])) // From LCA

		// Test multiple LCA operations
		builder2 := NewBlockBuilder().
			AddLatestCommonAncestor(hashes[1], hashes[2])

		querySet2 := builder2.QueryWith(store)
		require.True(t, querySet2.Contains(hashes[0])) // From LCA of hashes[1] and hashes[2]

		// Test empty case
		builder3 := NewBlockBuilder().AddLatestCommonAncestor()
		querySet3 := builder3.QueryWith(store)
		require.False(t, querySet3.Contains(hashes[0])) // Should be empty
	})
}

func TestReorgDetectionAlgorithms(t *testing.T) {
	logger := testlog.Logger(t, log.LevelDebug)
	ctx := context.Background()

	// Create two chains: main 0->1->2 and fork 0->3->4
	mockClient := NewMockEthClient()
	hashes := make([]common.Hash, 5)
	for i := 0; i < 5; i++ {
		hashes[i] = common.HexToHash(fmt.Sprintf("0x400%d", i))
	}

	mockClient.AddBlock(0, hashes[0], common.Hash{}) // Genesis
	mockClient.AddBlock(1, hashes[1], hashes[0])     // Main chain
	mockClient.AddBlock(2, hashes[2], hashes[1])     // Main chain
	mockClient.AddBlock(1, hashes[3], hashes[0])     // Fork
	mockClient.AddBlock(2, hashes[4], hashes[3])     // Fork

	store := NewEthClientStore(ctx, mockClient, logger)

	t.Run("IsReorg using ReorgQuery", func(t *testing.T) {
		// Test reorg between different chains (block 2 to block 4)
		reorg := store.Reorg(hashes[2], hashes[4])
		require.True(t, reorg.IsReorg(), "Should detect reorg between different chains")

		// Test no reorg on same chain (block 1 to block 2)
		notReorg := store.Reorg(hashes[1], hashes[2])
		require.False(t, notReorg.IsReorg(), "Should NOT detect reorg on same chain")
	})

	t.Run("LCA using DSL method", func(t *testing.T) {
		// Common ancestor of blocks 2 and 4 should be block 0
		lca, found := store.Query(hashes[2]).LatestCommonAncestor(hashes[4])
		require.True(t, found)
		require.Equal(t, hashes[0], lca)
	})

	t.Run("ReorgQuery depth and chains", func(t *testing.T) {
		reorg := store.Reorg(hashes[2], hashes[4])

		// Test LCA
		lca, ok := reorg.LCA()
		require.True(t, ok)
		require.Equal(t, hashes[0], lca)

		// Test depth (should be 2 - distance from block 2 to block 0)
		depth := reorg.Depth()
		require.Equal(t, 2, depth)

		// Test old chain
		oldChain, err := reorg.OldChain()
		require.NoError(t, err)
		require.Equal(t, []common.Hash{hashes[2], hashes[1]}, oldChain)

		// Test new chain
		newChain, err := reorg.NewChain()
		require.NoError(t, err)
		require.Equal(t, []common.Hash{hashes[4], hashes[3]}, newChain)
	})
}

func TestReorgQueryDSL(t *testing.T) {
	logger := testlog.Logger(t, log.LevelDebug)
	ctx := context.Background()

	// Create a fork scenario:
	//     0
	//    / \
	//   1   2
	//   |   |
	//   3   4
	//   |   |
	//   5   6
	mockClient := NewMockEthClient()
	hashes := make([]common.Hash, 7)
	for i := 0; i < 7; i++ {
		hashes[i] = common.HexToHash(fmt.Sprintf("0x800%d", i))
	}

	mockClient.AddBlock(0, hashes[0], common.Hash{}) // Genesis
	mockClient.AddBlock(1, hashes[1], hashes[0])     // Left branch
	mockClient.AddBlock(1, hashes[2], hashes[0])     // Right branch
	mockClient.AddBlock(2, hashes[3], hashes[1])     // Continue left
	mockClient.AddBlock(2, hashes[4], hashes[2])     // Continue right
	mockClient.AddBlock(3, hashes[5], hashes[3])     // Continue left
	mockClient.AddBlock(3, hashes[6], hashes[4])     // Continue right

	store := NewEthClientStore(ctx, mockClient, logger)

	t.Run("ReorgQuery basic functionality", func(t *testing.T) {
		// Test chain extension (not a reorg)
		reorg1 := store.Reorg(hashes[3], hashes[5])
		require.False(t, reorg1.IsReorg())

		// Test actual reorg
		reorg2 := store.Reorg(hashes[5], hashes[6])
		require.True(t, reorg2.IsReorg())

		// Test LCA
		lca, ok := reorg2.LCA()
		require.True(t, ok)
		require.Equal(t, hashes[0], lca)

		// Test depth
		depth := reorg2.Depth()
		require.Equal(t, 3, depth) // Distance from block 5 to block 0

		// Test summary
		summary := reorg2.Summary()
		require.Contains(t, summary, "Reorganization detected")
		require.Contains(t, summary, "depth=3")
	})

	t.Run("ReorgQuery chain analysis", func(t *testing.T) {
		reorg := store.Reorg(hashes[5], hashes[6])

		// Test old chain
		oldChain, err := reorg.OldChain()
		require.NoError(t, err)
		expected := []common.Hash{hashes[5], hashes[3], hashes[1]}
		require.Equal(t, expected, oldChain)

		// Test new chain
		newChain, err := reorg.NewChain()
		require.NoError(t, err)
		expected2 := []common.Hash{hashes[6], hashes[4], hashes[2]}
		require.Equal(t, expected2, newChain)
	})

	t.Run("ReorgQuery edge cases", func(t *testing.T) {
		// Test same block (no reorg)
		reorg := store.Reorg(hashes[3], hashes[3])
		require.False(t, reorg.IsReorg())
		require.Equal(t, "No reorganization - simple chain extension", reorg.Summary())

		// Test with caching
		reorg2 := store.Reorg(hashes[5], hashes[6])

		// Call LCA twice to test caching
		lca1, ok1 := reorg2.LCA()
		lca2, ok2 := reorg2.LCA()
		require.True(t, ok1)
		require.True(t, ok2)
		require.Equal(t, lca1, lca2)

		// Call Depth twice to test caching
		depth1 := reorg2.Depth()
		depth2 := reorg2.Depth()
		require.Equal(t, depth1, depth2)
	})
}
