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

func TestConvenienceFunctions(t *testing.T) {
	logger := testlog.Logger(t, log.LevelDebug)
	ctx := context.Background()

	// Create a diamond DAG: 0 -> 1 -> 3, 0 -> 2 -> 3
	mockClient := NewMockEthClient()
	hashes := make([]common.Hash, 4)
	for i := 0; i < 4; i++ {
		hashes[i] = common.HexToHash(fmt.Sprintf("0x300%d", i))
	}

	mockClient.AddBlock(0, hashes[0], common.Hash{}) // Genesis
	mockClient.AddBlock(1, hashes[1], hashes[0])     // Branch 1
	mockClient.AddBlock(1, hashes[2], hashes[0])     // Branch 2 (same depth)
	// For this test, we'll simulate block 3 having both 1 and 2 as parents
	// Note: Our mock doesn't support multiple parents, so we'll just use one
	mockClient.AddBlock(2, hashes[3], hashes[1])

	store := NewEthClientStore(ctx, mockClient, logger)

	t.Run("CommonAncestors", func(t *testing.T) {
		// Common ancestors of blocks 1 and 2 should include block 0
		commonAncestors := CommonAncestors(hashes[1], hashes[2])
		require.True(t, commonAncestors.Contains(store, hashes[0]))

		// Empty case
		emptyCommon := CommonAncestors()
		require.False(t, emptyCommon.Contains(store, hashes[0]))
	})

	t.Run("AnyAncestors", func(t *testing.T) {
		// Any ancestors of blocks 1 and 2 should include block 0
		anyAncestors := AnyAncestors(hashes[1], hashes[2])
		require.True(t, anyAncestors.Contains(store, hashes[0]))

		// Empty case
		emptyAny := AnyAncestors()
		require.False(t, emptyAny.Contains(store, hashes[0]))
	})

	t.Run("CommonDescendants", func(t *testing.T) {
		// Common descendants of blocks 1 and 2 (none in our simple case)
		commonDescendants := CommonDescendants(hashes[1], hashes[2])
		// Since our mock doesn't have true convergence, we won't have common descendants
		require.False(t, commonDescendants.Contains(store, hashes[3]))

		// Empty case
		emptyCommon := CommonDescendants()
		require.False(t, emptyCommon.Contains(store, hashes[0]))
	})

	t.Run("AnyDescendants", func(t *testing.T) {
		// Any descendants of blocks 0 should include blocks 1, 2, 3
		anyDescendants := AnyDescendants(hashes[0])
		require.True(t, anyDescendants.Contains(store, hashes[1]))
		require.True(t, anyDescendants.Contains(store, hashes[2]))
		require.True(t, anyDescendants.Contains(store, hashes[3]))
		require.False(t, anyDescendants.Contains(store, hashes[0])) // Not its own descendant

		// Empty case
		emptyAny := AnyDescendants()
		require.False(t, emptyAny.Contains(store, hashes[0]))
	})

	t.Run("Multiple block convenience functions", func(t *testing.T) {
		// Test with multiple blocks
		anyAncestorsMultiple := AnyAncestors(hashes[1], hashes[2], hashes[3])
		require.True(t, anyAncestorsMultiple.Contains(store, hashes[0]))

		// Test single block case
		singleAncestors := AnyAncestors(hashes[2])
		require.True(t, singleAncestors.Contains(store, hashes[0]))
		require.False(t, singleAncestors.Contains(store, hashes[2])) // Not its own ancestor
	})
}
