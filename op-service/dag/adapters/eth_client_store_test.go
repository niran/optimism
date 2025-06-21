package adapters

import (
	"context"
	"fmt"
	"testing"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/op-service/dag"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
)

// MockEthClient implements the EthClientLike interface for testing
type MockEthClient struct {
	blocks map[common.Hash]eth.L1BlockRef
}

func NewMockEthClient() *MockEthClient {
	return &MockEthClient{
		blocks: make(map[common.Hash]eth.L1BlockRef),
	}
}

func (m *MockEthClient) AddBlock(number uint64, hash, parentHash common.Hash) {
	ref := eth.L1BlockRef{
		Hash:       hash,
		Number:     number,
		ParentHash: parentHash,
		Time:       uint64(1000000 + number), // Mock timestamp
	}
	m.blocks[hash] = ref
}

func (m *MockEthClient) BlockRefByHash(ctx context.Context, hash common.Hash) (eth.BlockRef, error) {
	if block, ok := m.blocks[hash]; ok {
		return block, nil
	}
	return eth.L1BlockRef{}, ethereum.NotFound
}

func (m *MockEthClient) BlockRefByNumber(ctx context.Context, number uint64) (eth.BlockRef, error) {
	for _, block := range m.blocks {
		if block.Number == number {
			return block, nil
		}
	}
	return eth.L1BlockRef{}, ethereum.NotFound
}

// Verify interface compliance
var _ EthClientLike = (*MockEthClient)(nil)

func TestBlock(t *testing.T) {
	hash := common.HexToHash("0x1234")
	parentHash := common.HexToHash("0x5678")

	ref := eth.L1BlockRef{
		Hash:       hash,
		Number:     100,
		ParentHash: parentHash,
		Time:       1000000,
	}

	node := &Block{
		BlockRef: ref,
	}

	// Test ID
	require.Equal(t, hash, node.ID())

	// Test Parents
	parents := node.Parents()
	require.Len(t, parents, 1)
	require.Equal(t, parentHash, parents[0])
}

func TestBlockGenesis(t *testing.T) {
	hash := common.HexToHash("0x1234")

	ref := eth.L1BlockRef{
		Hash:   hash,
		Number: 0,
		Time:   1000000,
		// ParentHash is zero value for genesis
	}

	node := &Block{
		BlockRef: ref,
	}

	// Test Parents for genesis block
	parents := node.Parents()
	require.Empty(t, parents)
}

func TestEthClientStore(t *testing.T) {
	logger := testlog.Logger(t, log.LevelDebug)
	ctx := context.Background()

	// Create mock client with a simple chain: 0 -> 1 -> 2
	mockClient := NewMockEthClient()
	hash0 := common.HexToHash("0x1000")
	hash1 := common.HexToHash("0x1001")
	hash2 := common.HexToHash("0x1002")

	mockClient.AddBlock(0, hash0, common.Hash{}) // Genesis
	mockClient.AddBlock(1, hash1, hash0)         // Block 1
	mockClient.AddBlock(2, hash2, hash1)         // Block 2

	store := NewEthClientStore(ctx, mockClient, logger)

	t.Run("Node retrieval by hash", func(t *testing.T) {
		// Test existing block
		node, ok := store.Node(hash1)
		require.True(t, ok)
		require.NotNil(t, node)
		require.Equal(t, hash1, node.ID())

		blockNode := node.(*Block)
		require.Equal(t, uint64(1), blockNode.BlockRef.Number)
		require.Equal(t, []common.Hash{hash0}, node.Parents())

		// Test non-existing block
		nonExistentHash := common.HexToHash("0x9999")
		node, ok = store.Node(nonExistentHash)
		require.False(t, ok)
		require.Nil(t, node)
	})

	t.Run("NodesAtDepth", func(t *testing.T) {
		// Test existing depth
		nodes, err := store.NodesAtDepth(1)
		require.NoError(t, err)
		require.Len(t, nodes, 1)
		require.Contains(t, nodes, hash1)

		// Test non-existing depth
		nodes, err = store.NodesAtDepth(999)
		require.NoError(t, err)
		require.Len(t, nodes, 0) // Should return empty set, not error
	})

	t.Run("WithContext", func(t *testing.T) {
		newCtx := context.WithValue(ctx, "test", "value")
		newStore := store.WithContext(newCtx)
		require.Equal(t, newCtx, newStore.ctx)
		require.Equal(t, store.client, newStore.client)
		require.Equal(t, store.log, newStore.log)
	})
}

func TestDAGIntegration(t *testing.T) {
	logger := testlog.Logger(t, log.LevelDebug)
	ctx := context.Background()

	// Create a more complex chain for dag operations
	// 0 -> 1 -> 2 -> 3 -> 4
	mockClient := NewMockEthClient()
	hashes := make([]common.Hash, 5)
	for i := 0; i < 5; i++ {
		hashes[i] = common.HexToHash(fmt.Sprintf("0x100%d", i))
	}

	mockClient.AddBlock(0, hashes[0], common.Hash{}) // Genesis
	for i := 1; i < 5; i++ {
		mockClient.AddBlock(uint64(i), hashes[i], hashes[i-1])
	}

	store := NewEthClientStore(ctx, mockClient, logger)

	t.Run("Ancestors query", func(t *testing.T) {
		// Test if block 0 is an ancestor of block 4
		ancestorsOfBlock4 := dag.Ancestors(dag.ID(hashes[4]))
		isAncestor := ancestorsOfBlock4.Contains(store, hashes[0])
		require.True(t, isAncestor, "Block 0 should be ancestor of block 4")

		// Test if block 2 is an ancestor of block 4
		isAncestor = ancestorsOfBlock4.Contains(store, hashes[2])
		require.True(t, isAncestor, "Block 2 should be ancestor of block 4")

		// Test if block 4 is an ancestor of itself (should be false per spec)
		isAncestor = ancestorsOfBlock4.Contains(store, hashes[4])
		require.False(t, isAncestor, "Block 4 should NOT be ancestor of itself")

		// Test if block 4 is NOT an ancestor of block 2
		ancestorsOfBlock2 := dag.Ancestors(dag.ID(hashes[2]))
		isAncestor = ancestorsOfBlock2.Contains(store, hashes[4])
		require.False(t, isAncestor, "Block 4 should NOT be ancestor of block 2")
	})

	t.Run("Descendants query", func(t *testing.T) {
		// Test if block 4 is a descendant of block 0
		descendantsOfBlock0 := dag.Descendants(dag.ID(hashes[0]))
		isDescendant := descendantsOfBlock0.Contains(store, hashes[4])
		require.True(t, isDescendant, "Block 4 should be descendant of block 0")

		// Test if block 2 is a descendant of block 0
		isDescendant = descendantsOfBlock0.Contains(store, hashes[2])
		require.True(t, isDescendant, "Block 2 should be descendant of block 0")

		// Test if block 0 is NOT a descendant of block 2
		descendantsOfBlock2 := dag.Descendants(dag.ID(hashes[2]))
		isDescendant = descendantsOfBlock2.Contains(store, hashes[0])
		require.False(t, isDescendant, "Block 0 should NOT be descendant of block 2")
	})

	t.Run("Parents and Children", func(t *testing.T) {
		// Test parents of block 2
		parentsOfBlock2 := dag.Parents(dag.ID(hashes[2]))
		isParent := parentsOfBlock2.Contains(store, hashes[1])
		require.True(t, isParent, "Block 1 should be parent of block 2")

		isParent = parentsOfBlock2.Contains(store, hashes[0])
		require.False(t, isParent, "Block 0 should NOT be direct parent of block 2")

		// Test children of block 1
		childrenOfBlock1 := dag.Children(dag.ID(hashes[1]))
		isChild := childrenOfBlock1.Contains(store, hashes[2])
		require.True(t, isChild, "Block 2 should be child of block 1")

		isChild = childrenOfBlock1.Contains(store, hashes[3])
		require.False(t, isChild, "Block 3 should NOT be direct child of block 1")
	})

	t.Run("Slice operation", func(t *testing.T) {
		// Test slice from block 1 to block 4 (should include 1, 2, 3, 4)
		slice := dag.Slice(dag.ID(hashes[1]), dag.ID(hashes[4]))

		require.True(t, slice.Contains(store, hashes[1]), "Slice should contain start block")
		require.True(t, slice.Contains(store, hashes[2]), "Slice should contain middle block")
		require.True(t, slice.Contains(store, hashes[3]), "Slice should contain middle block")
		require.True(t, slice.Contains(store, hashes[4]), "Slice should contain end block")
		require.False(t, slice.Contains(store, hashes[0]), "Slice should NOT contain block before start")
	})

	t.Run("Enumerable Descendants and Children", func(t *testing.T) {
		// Test that Descendants can now be enumerated
		descendantsOfBlock0 := dag.Descendants(dag.ID(hashes[0]))
		descendants, err := descendantsOfBlock0.Eval(store)
		require.NoError(t, err)
		require.Contains(t, descendants, hashes[1])
		require.Contains(t, descendants, hashes[2])
		require.Contains(t, descendants, hashes[3])
		require.Contains(t, descendants, hashes[4])
		require.NotContains(t, descendants, hashes[0]) // Self not included

		// Test that Children can now be enumerated
		childrenOfBlock1 := dag.Children(dag.ID(hashes[1]))
		children, err := childrenOfBlock1.Eval(store)
		require.NoError(t, err)
		require.Contains(t, children, hashes[2])
		require.NotContains(t, children, hashes[3]) // Not direct child
	})

	t.Run("Union operations", func(t *testing.T) {
		// Union of block 1 and block 3
		union := dag.Union(dag.ID(hashes[1]), dag.ID(hashes[3]))

		require.True(t, union.Contains(store, hashes[1]))
		require.False(t, union.Contains(store, hashes[2]))
		require.True(t, union.Contains(store, hashes[3]))
		require.False(t, union.Contains(store, hashes[4]))
	})
}
