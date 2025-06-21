package adapters

import (
	"context"
	"fmt"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/op-service/dag"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
)

func TestFluentDSL(t *testing.T) {
	logger := testlog.Logger(t, log.LevelDebug)
	ctx := context.Background()

	// Create a simple chain: 0 -> 1 -> 2 -> 3
	mockClient := NewMockEthClient()
	hashes := make([]common.Hash, 4)
	for i := 0; i < 4; i++ {
		hashes[i] = common.HexToHash(fmt.Sprintf("0x100%d", i))
	}

	mockClient.AddBlock(0, hashes[0], common.Hash{}) // Genesis
	for i := 1; i < 4; i++ {
		mockClient.AddBlock(uint64(i), hashes[i], hashes[i-1])
	}

	store := NewEthClientStore(ctx, mockClient, logger)

	t.Run("BlockQuery fluent interface", func(t *testing.T) {
		// Test From() and basic operations
		latestBlock := From(hashes[3])
		require.Equal(t, hashes[3], latestBlock.Hash())

		// Test ancestors query
		ancestorsQuery := latestBlock.Ancestors()
		require.True(t, ancestorsQuery.Contains(store, hashes[0]), "Block 0 should be ancestor of block 3")
		require.True(t, ancestorsQuery.Contains(store, hashes[1]), "Block 1 should be ancestor of block 3")
		require.False(t, ancestorsQuery.Contains(store, hashes[3]), "Block 3 should NOT be its own ancestor")

		// Test descendants query
		firstBlock := From(hashes[0])
		descendantsQuery := firstBlock.Descendants()
		require.True(t, descendantsQuery.Contains(store, hashes[3]), "Block 3 should be descendant of block 0")
		require.False(t, descendantsQuery.Contains(store, hashes[0]), "Block 0 should NOT be its own descendant")
	})

	t.Run("StoreQuery immediate execution", func(t *testing.T) {
		// Test store.Query() method
		storeQuery := store.Query(hashes[3])
		require.Equal(t, hashes[3], storeQuery.Hash())
		require.Equal(t, store, storeQuery.Store())

		// Test immediate execution
		ancestorsSet := storeQuery.Ancestors()
		require.True(t, ancestorsSet.Contains(hashes[0]), "Block 0 should be ancestor")
		require.False(t, ancestorsSet.Contains(hashes[3]), "Block 3 should NOT be its own ancestor")

		// Test evaluation (note: ancestors predicates return ErrEnumerationDisabled by design)
		ancestors, err := ancestorsSet.Eval()
		require.Equal(t, dag.ErrEnumerationDisabled, err)
		require.Nil(t, ancestors)
	})

	t.Run("Slice operations", func(t *testing.T) {
		// Test SliceTo
		slice := From(hashes[1]).SliceTo(hashes[3])
		require.True(t, slice.Contains(store, hashes[1]), "Slice should include start")
		require.True(t, slice.Contains(store, hashes[2]), "Slice should include middle")
		require.True(t, slice.Contains(store, hashes[3]), "Slice should include end")
		require.False(t, slice.Contains(store, hashes[0]), "Slice should NOT include before start")

		// Test SliceFrom
		sliceFrom := From(hashes[3]).SliceFrom(hashes[1])
		require.True(t, sliceFrom.Contains(store, hashes[1]), "SliceFrom should include start")
		require.True(t, sliceFrom.Contains(store, hashes[3]), "SliceFrom should include end")
	})

	t.Run("Union operations", func(t *testing.T) {
		// Test UnionWith
		union := From(hashes[0]).UnionWith(hashes[2])
		require.True(t, union.Contains(store, hashes[0]), "Union should contain first block")
		require.False(t, union.Contains(store, hashes[1]), "Union should NOT contain middle block")
		require.True(t, union.Contains(store, hashes[2]), "Union should contain second block")
	})

	t.Run("FromBlock and FromBlockRef", func(t *testing.T) {
		// Test FromBlock
		node, ok := store.Node(hashes[2])
		require.True(t, ok)
		block := node.(*Block)

		blockQuery := FromBlock(block)
		require.Equal(t, hashes[2], blockQuery.Hash())

		// Test FromBlockRef
		refQuery := FromBlockRef(block.BlockRef)
		require.Equal(t, hashes[2], refQuery.Hash())
	})
}

func TestBlockBuilder(t *testing.T) {
	logger := testlog.Logger(t, log.LevelDebug)
	ctx := context.Background()

	// Create a simple chain
	mockClient := NewMockEthClient()
	hashes := make([]common.Hash, 4)
	for i := 0; i < 4; i++ {
		hashes[i] = common.HexToHash(fmt.Sprintf("0x200%d", i))
	}

	mockClient.AddBlock(0, hashes[0], common.Hash{})
	for i := 1; i < 4; i++ {
		mockClient.AddBlock(uint64(i), hashes[i], hashes[i-1])
	}

	store := NewEthClientStore(ctx, mockClient, logger)

	t.Run("Builder operations", func(t *testing.T) {
		builder := NewBlockBuilder()

		// Test Add
		builder.Add(hashes[0]).Add(hashes[2])
		union := builder.Union()
		require.True(t, union.Contains(store, hashes[0]))
		require.False(t, union.Contains(store, hashes[1]))
		require.True(t, union.Contains(store, hashes[2]))

		// Test AddAncestors
		builder2 := NewBlockBuilder().AddAncestors(hashes[3])
		ancestorsUnion := builder2.Union()
		require.True(t, ancestorsUnion.Contains(store, hashes[0]))
		require.True(t, ancestorsUnion.Contains(store, hashes[1]))
		require.True(t, ancestorsUnion.Contains(store, hashes[2]))
		require.False(t, ancestorsUnion.Contains(store, hashes[3])) // Not its own ancestor

		// Test AddSlice
		builder3 := NewBlockBuilder().AddSlice(hashes[1], hashes[3])
		sliceUnion := builder3.Union()
		require.False(t, sliceUnion.Contains(store, hashes[0]))
		require.True(t, sliceUnion.Contains(store, hashes[1]))
		require.True(t, sliceUnion.Contains(store, hashes[2]))
		require.True(t, sliceUnion.Contains(store, hashes[3]))
	})

	t.Run("QueryWith", func(t *testing.T) {
		builder := NewBlockBuilder().Add(hashes[1]).Add(hashes[3])
		querySet := builder.QueryWith(store)

		require.True(t, querySet.Contains(hashes[1]))
		require.False(t, querySet.Contains(hashes[2]))
		require.True(t, querySet.Contains(hashes[3]))
	})

	t.Run("Empty builder", func(t *testing.T) {
		emptyBuilder := NewBlockBuilder()
		emptyUnion := emptyBuilder.Union()
		emptyIntersect := emptyBuilder.Intersect()

		// Both should be empty sets
		require.False(t, emptyUnion.Contains(store, hashes[0]))
		require.False(t, emptyIntersect.Contains(store, hashes[0]))
	})
}

func TestStoreQuerySetOperations(t *testing.T) {
	logger := testlog.Logger(t, log.LevelDebug)
	ctx := context.Background()

	mockClient := NewMockEthClient()
	hashes := make([]common.Hash, 4)
	for i := 0; i < 4; i++ {
		hashes[i] = common.HexToHash(fmt.Sprintf("0x500%d", i))
	}

	mockClient.AddBlock(0, hashes[0], common.Hash{})
	for i := 1; i < 4; i++ {
		mockClient.AddBlock(uint64(i), hashes[i], hashes[i-1])
	}

	store := NewEthClientStore(ctx, mockClient, logger)

	t.Run("UnionWith", func(t *testing.T) {
		ancestorsSet := store.Query(hashes[2]).Ancestors()
		unionSet := ancestorsSet.UnionWith(dag.ID(hashes[3]))

		require.True(t, unionSet.Contains(hashes[0]))  // From ancestors
		require.True(t, unionSet.Contains(hashes[1]))  // From ancestors
		require.True(t, unionSet.Contains(hashes[3]))  // From union
		require.False(t, unionSet.Contains(hashes[2])) // Not its own ancestor
	})

	t.Run("IntersectWith", func(t *testing.T) {
		// Test intersection with enumerable sets only
		// Create two simple ID sets that can intersect properly
		set1 := &StoreQuerySet{store: store, nodeSet: dag.Union(dag.ID(hashes[0]), dag.ID(hashes[1]), dag.ID(hashes[2]))}
		set2 := dag.Union(dag.ID(hashes[0]), dag.ID(hashes[1]))
		intersectSet := set1.IntersectWith(set2)

		require.True(t, intersectSet.Contains(hashes[0]))  // In both sets
		require.True(t, intersectSet.Contains(hashes[1]))  // In both sets
		require.False(t, intersectSet.Contains(hashes[2])) // Only in first set
		require.False(t, intersectSet.Contains(hashes[3])) // In neither set
	})

	t.Run("Except", func(t *testing.T) {
		ancestorsSet := store.Query(hashes[3]).Ancestors()
		exceptSet := ancestorsSet.Except(dag.ID(hashes[1]))

		require.True(t, exceptSet.Contains(hashes[0]))  // Not removed
		require.False(t, exceptSet.Contains(hashes[1])) // Removed
		require.True(t, exceptSet.Contains(hashes[2]))  // Not removed
	})
}

func TestBlockchainSpecificMethods(t *testing.T) {
	logger := testlog.Logger(t, log.LevelDebug)
	ctx := context.Background()

	// Create a simple chain: 0 -> 1 -> 2 -> 3
	mockClient := NewMockEthClient()
	hashes := make([]common.Hash, 4)
	for i := 0; i < 4; i++ {
		hashes[i] = common.HexToHash(fmt.Sprintf("0x600%d", i))
	}

	mockClient.AddBlock(0, hashes[0], common.Hash{}) // Genesis
	for i := 1; i < 4; i++ {
		mockClient.AddBlock(uint64(i), hashes[i], hashes[i-1])
	}

	store := NewEthClientStore(ctx, mockClient, logger)

	t.Run("Parent method", func(t *testing.T) {
		// Test getting single parent
		query := store.Query(hashes[2])
		parent, hasParent := query.Parent()
		require.True(t, hasParent)
		require.Equal(t, hashes[1], parent)

		// Test genesis has no parent
		genesisQuery := store.Query(hashes[0])
		_, hasGenesisParent := genesisQuery.Parent()
		require.False(t, hasGenesisParent)

		// Test BlockQuery.Parent method too
		blockQuery := From(hashes[2])
		parent2, hasParent2 := blockQuery.Parent(store)
		require.True(t, hasParent2)
		require.Equal(t, hashes[1], parent2)
	})

	t.Run("IsGenesis method", func(t *testing.T) {
		// Genesis block should return true
		genesisQuery := store.Query(hashes[0])
		require.True(t, genesisQuery.IsGenesis())

		// Non-genesis block should return false
		nonGenesisQuery := store.Query(hashes[2])
		require.False(t, nonGenesisQuery.IsGenesis())
	})

	t.Run("BlockNumber method", func(t *testing.T) {
		// Test block numbers
		for i, hash := range hashes {
			query := store.Query(hash)
			blockNum, ok := query.BlockNumber()
			require.True(t, ok)
			require.Equal(t, uint64(i), blockNum)
		}
	})

	t.Run("IsOnMainChain method", func(t *testing.T) {
		// All blocks should be on main chain when tip is block 3
		for i := 0; i < 3; i++ {
			query := store.Query(hashes[i])
			require.True(t, query.IsOnChain(hashes[3]))
		}

		// Block 3 should be on its own main chain
		tipQuery := store.Query(hashes[3])
		require.True(t, tipQuery.IsOnChain(hashes[3]))
	})

	t.Run("DistanceFrom method", func(t *testing.T) {
		// Distance from block 3 to block 1 should be 2
		query := store.Query(hashes[3])
		distance, ok := query.DistanceFrom(hashes[1])
		require.True(t, ok)
		require.Equal(t, 2, distance)

		// Distance from block 1 to block 3 should be 2
		query2 := store.Query(hashes[1])
		distance2, ok2 := query2.DistanceFrom(hashes[3])
		require.True(t, ok2)
		require.Equal(t, 2, distance2)

		// Distance from block to itself should be 0
		distance3, ok3 := query.DistanceFrom(hashes[3])
		require.True(t, ok3)
		require.Equal(t, 0, distance3)
	})

	t.Run("GetChainSegment method", func(t *testing.T) {
		// Get chain segment from block 3 to block 1
		query := store.Query(hashes[3])
		segment, err := query.GetChainSegment(hashes[1], 10)
		require.NoError(t, err)

		// Should return [block3, block2, block1] in reverse chronological order
		expected := []common.Hash{hashes[3], hashes[2], hashes[1]}
		require.Equal(t, expected, segment)

		// Get chain segment from block 2 to genesis
		query2 := store.Query(hashes[2])
		segment2, err2 := query2.GetChainSegment(hashes[0], 10)
		require.NoError(t, err2)

		// Should return [block2, block1, block0]
		expected2 := []common.Hash{hashes[2], hashes[1], hashes[0]}
		require.Equal(t, expected2, segment2)
	})
}
