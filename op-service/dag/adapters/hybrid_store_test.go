package adapters

import (
	"context"
	"fmt"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
)

func TestHybridStore(t *testing.T) {
	logger := testlog.Logger(t, log.LevelDebug)
	ctx := context.Background()

	// Create a mock client
	mockClient := NewMockEthClient()
	hashes := make([]common.Hash, 4)
	for i := 0; i < 4; i++ {
		hashes[i] = common.HexToHash(fmt.Sprintf("0x900%d", i))
	}

	// Add canonical blocks to mock client
	mockClient.AddBlock(0, hashes[0], common.Hash{}) // Genesis
	mockClient.AddBlock(1, hashes[1], hashes[0])     // Block 1
	mockClient.AddBlock(2, hashes[2], hashes[1])     // Block 2

	// Create hybrid store
	hybridStore := NewHybridStore(ctx, mockClient, logger)

	t.Run("Fallback to RPC for canonical blocks", func(t *testing.T) {
		// Should be able to retrieve canonical blocks via RPC fallback
		node, ok := hybridStore.Node(hashes[1])
		require.True(t, ok)
		require.Equal(t, hashes[1], node.ID())
	})

	t.Run("Local indexing for reorged blocks", func(t *testing.T) {
		// Index a reorged block locally
		reorgedRef := eth.L1BlockRef{
			Hash:       hashes[3],
			Number:     2,
			ParentHash: hashes[1],
			Time:       1234567890,
		}
		hybridStore.IndexBlockRef(reorgedRef)

		// Should be able to retrieve the locally indexed block
		node, ok := hybridStore.Node(hashes[3])
		require.True(t, ok)
		require.Equal(t, hashes[3], node.ID())

		// Check if it's locally indexed
		require.True(t, hybridStore.IsLocallyIndexed(hashes[3]))
		require.False(t, hybridStore.IsLocallyIndexed(hashes[1])) // RPC block not locally indexed
	})

	t.Run("Local index management", func(t *testing.T) {
		// Add a block directly
		block := &Block{
			BlockRef: eth.L1BlockRef{
				Hash:       common.HexToHash("0x1234"),
				Number:     5,
				ParentHash: hashes[2],
				Time:       1234567891,
			},
		}
		hybridStore.IndexBlock(block)

		// Verify it's indexed
		require.True(t, hybridStore.IsLocallyIndexed(block.ID()))

		// Get all locally indexed blocks
		localBlocks := hybridStore.GetLocallyIndexedBlocks()
		require.Contains(t, localBlocks, block.ID())
		require.Contains(t, localBlocks, hashes[3]) // From previous test

		// Remove from index
		hybridStore.RemoveFromIndex(block.ID())
		require.False(t, hybridStore.IsLocallyIndexed(block.ID()))
	})

	t.Run("NodesAtDepth", func(t *testing.T) {
		// Create a fresh hybrid store for this test to avoid interference
		freshStore := NewHybridStore(ctx, mockClient, logger)

		// Add a reorged block at depth 2 (same as canonical block)
		reorgedRef := eth.L1BlockRef{
			Hash:       common.HexToHash("0x8888"),
			Number:     2,
			ParentHash: hashes[1],
			Time:       1234567890,
		}
		freshStore.IndexBlockRef(reorgedRef)

		// Should return both canonical and reorged blocks at depth 2
		nodes, err := freshStore.NodesAtDepth(2)
		require.NoError(t, err)
		require.Len(t, nodes, 2)                               // Canonical + reorged
		require.Contains(t, nodes, hashes[2])                  // Canonical block
		require.Contains(t, nodes, common.HexToHash("0x8888")) // Reorged block

		// Test depth with only canonical block
		nodes, err = freshStore.NodesAtDepth(1)
		require.NoError(t, err)
		require.Len(t, nodes, 1)
		require.Contains(t, nodes, hashes[1])

		// Test non-existing depth
		nodes, err = freshStore.NodesAtDepth(999)
		require.NoError(t, err)
		require.Len(t, nodes, 0)
	})

	t.Run("Query interface", func(t *testing.T) {
		// Test that Query() returns a working StoreQuery
		query := hybridStore.Query(hashes[1])
		require.Equal(t, hashes[1], query.Hash())
		require.NotNil(t, query.Store())
	})
}
