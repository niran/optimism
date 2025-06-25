package syncnode

import (
	"context"
	"fmt"
	"testing"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/require"
)

// mockResetBackend implements the resetBackend interface for testing
type mockResetBackend struct {
	// nodeBlocks represents blocks known to the node
	nodeBlocks map[uint64]eth.BlockID
	// safeBlocks represents blocks marked as safe in the local DB
	safeBlocks map[uint64]eth.BlockID

	unsafeBlocks map[uint64]eth.L2BlockRef
	l1Blocks     map[uint64]eth.BlockID
	unsafeHead   eth.BlockID
}

func (m *mockResetBackend) reset() {
	m.nodeBlocks = make(map[uint64]eth.BlockID)
	m.safeBlocks = make(map[uint64]eth.BlockID)

	m.unsafeBlocks = make(map[uint64]eth.L2BlockRef)
	m.l1Blocks = make(map[uint64]eth.BlockID)
	m.unsafeHead = eth.BlockID{}
}

func (m *mockResetBackend) BlockIDByNumber(ctx context.Context, n uint64) (eth.BlockID, error) {
	if block, ok := m.nodeBlocks[n]; ok {
		return block, nil
	}
	return eth.BlockID{}, ethereum.NotFound
}

func (m *mockResetBackend) IsLocalSafe(ctx context.Context, block eth.BlockID) error {
	if safeBlock, ok := m.safeBlocks[block.Number]; ok {
		if safeBlock == block {
			return nil
		}
		return types.ErrConflict
	}
	return types.ErrFuture
}

func (m *mockResetBackend) L2BlockRefByNumber(ctx context.Context, n uint64) (eth.L2BlockRef, error) {
	if unsafeBlock, ok := m.unsafeBlocks[n]; ok {
		return unsafeBlock, nil
	}
	return eth.L2BlockRef{}, ethereum.NotFound
}

func (m *mockResetBackend) L1BlockIDByNumber(ctx context.Context, n uint64) (eth.BlockID, error) {
	if block, ok := m.l1Blocks[n]; ok {
		return block, nil
	}
	return eth.BlockID{}, ethereum.NotFound
}

func (m *mockResetBackend) LocalUnsafe(ctx context.Context) (eth.BlockID, error) {
	if (m.unsafeHead != eth.BlockID{}) {
		return m.unsafeHead, nil
	}
	return eth.BlockID{}, ethereum.NotFound
}

func TestResetTracker(t *testing.T) {
	logger := testlog.Logger(t, log.LvlDebug)
	backend := new(mockResetBackend)
	tracker := newResetTracker(logger, backend)
	ctx := context.Background()

	// Helper to create a block ID with a specific hash
	mkBlock := func(n uint64, nodeDivHash bool) eth.BlockID {
		hash := common.Hash{byte(n)}
		if nodeDivHash {
			hash[1] = 0xff
		}
		return eth.BlockID{Number: n, Hash: hash}
	}

	// Helper to set up a range of blocks
	// start: first block number in range
	// endNode: last block number in node
	// endLocal: last block number in local DB
	// divergence: block number at which node and safe DB hashes start to differ
	setupRange := func(start, endNode, endLocal, divergence uint64) {
		for i := start; i <= endNode; i++ {
			backend.nodeBlocks[i] = mkBlock(i, i >= divergence)
		}

		for i := start; i <= endLocal; i++ {
			backend.safeBlocks[i] = mkBlock(i, false)
		}
	}

	t.Run("pre-interop start block not found in node", func(t *testing.T) {
		backend.reset()
		target, err := tracker.FindResetTarget(ctx, mkBlock(1, false), mkBlock(10, false))
		require.NoError(t, err)
		require.True(t, target.PreInterop, "target is instead %v", target.Target)
	})

	t.Run("pre-interop start block inconsistent", func(t *testing.T) {
		backend.reset()
		setupRange(1, 10, 10, 1) // divergence at start, so all blocks are inconsistent
		target, err := tracker.FindResetTarget(ctx, mkBlock(1, false), mkBlock(10, false))
		require.NoError(t, err)
		require.True(t, target.PreInterop, "target is instead %v", target.Target)
	})

	t.Run("target found when end block is consistent", func(t *testing.T) {
		backend.reset()
		setupRange(1, 10, 10, 11) // divergence after range, so all blocks are consistent
		target, err := tracker.FindResetTarget(ctx, mkBlock(1, false), mkBlock(10, false))
		require.NoError(t, err)
		require.False(t, target.PreInterop)
		require.Equal(t, uint64(10), target.Target.Number)
		require.Equal(t, common.Hash{0x0a}, target.Target.Hash)
	})

	t.Run("bisection finds last consistent block", func(t *testing.T) {
		const rangeEnd = uint64(17)
		for divergence := uint64(2); divergence <= rangeEnd; divergence++ {
			t.Run(fmt.Sprintf("divergence at %d", divergence), func(t *testing.T) {
				backend.reset()
				setupRange(1, rangeEnd, rangeEnd, divergence)
				target, err := tracker.FindResetTarget(ctx, mkBlock(1, false), mkBlock(rangeEnd, false))
				require.NoError(t, err)
				require.False(t, target.PreInterop)
				require.Equal(t, divergence-1, target.Target.Number)
				require.Equal(t, common.Hash{byte(divergence - 1)}, target.Target.Hash)
			})
		}
	})

	t.Run("converges to start when range is small", func(t *testing.T) {
		backend.reset()
		// Set up a small range where only the start is consistent
		setupRange(1, 2, 2, 2)
		target, err := tracker.FindResetTarget(ctx, mkBlock(1, false), mkBlock(2, false))
		require.NoError(t, err)
		require.False(t, target.PreInterop)
		require.Equal(t, uint64(1), target.Target.Number)
		require.Equal(t, common.Hash{0x01}, target.Target.Hash)
	})

	t.Run("handles node ahead of local DB", func(t *testing.T) {
		backend.reset()
		// Node has more blocks than local DB
		setupRange(1, 10, 5, 11) // node has 1-10, local has 1-5
		target, err := tracker.FindResetTarget(ctx, mkBlock(1, false), mkBlock(5, false))
		require.NoError(t, err)
		require.False(t, target.PreInterop)
		require.Equal(t, uint64(5), target.Target.Number)
		require.Equal(t, common.Hash{0x05}, target.Target.Hash)
	})

	t.Run("handles local DB ahead of node", func(t *testing.T) {
		backend.reset()
		// Local DB has more blocks than node
		setupRange(1, 5, 10, 11) // node has 1-5, local has 1-10
		target, err := tracker.FindResetTarget(ctx, mkBlock(1, false), mkBlock(10, false))
		require.NoError(t, err)
		require.False(t, target.PreInterop)
		require.Equal(t, uint64(5), target.Target.Number)
		require.Equal(t, common.Hash{0x05}, target.Target.Hash)
	})
}

func TestResetTrackerLocalUnsafe(t *testing.T) {
	logger := testlog.Logger(t, log.LvlDebug)
	backend := new(mockResetBackend)
	tracker := newResetTracker(logger, backend)
	ctx := context.Background()

	tests := []struct {
		name           string
		l2Unsafe       uint64   // starting point (trusted valid)
		latestUnsafe   uint64   // current chain tip
		validBlocks    []uint64 // blocks with valid L1 origins
		expectedResult uint64
		expectedError  string
	}{
		{
			name:           "target_equals_latest",
			l2Unsafe:       100,
			latestUnsafe:   100,
			validBlocks:    []uint64{100},
			expectedResult: 100,
		},
		{
			name:           "all_blocks_valid",
			l2Unsafe:       100,
			latestUnsafe:   105,
			validBlocks:    []uint64{100, 101, 102, 103, 104, 105},
			expectedResult: 105,
		},
		{
			name:           "all_blocks_invalid",
			l2Unsafe:       100,
			latestUnsafe:   105,
			validBlocks:    []uint64{100}, // only l2Unsafe is valid
			expectedResult: 100,
		},
		{
			name:           "mixed_validity_case1",
			l2Unsafe:       100,
			latestUnsafe:   105,
			validBlocks:    []uint64{100, 101, 102}, // 103-105 invalid
			expectedResult: 102,
		},
		{
			name:           "single_block_ahead_valid",
			l2Unsafe:       100,
			latestUnsafe:   101,
			validBlocks:    []uint64{100, 101},
			expectedResult: 101,
		},
		{
			name:           "single_block_ahead_invalid",
			l2Unsafe:       100,
			latestUnsafe:   101,
			validBlocks:    []uint64{100}, // 101 invalid
			expectedResult: 100,
		},
		{
			name:           "target_not_at_100",
			l2Unsafe:       95,
			latestUnsafe:   100,
			validBlocks:    []uint64{95, 96, 97}, // 98-100 invalid
			expectedResult: 97,
		},
		{
			name:           "target_is_invalid",
			l2Unsafe:       100,
			latestUnsafe:   100,
			validBlocks:    []uint64{96, 97, 98, 99}, // 96-99 valid
			expectedResult: 99,
		},
		{
			name:           "target_is_larger_than_latest",
			l2Unsafe:       101,
			latestUnsafe:   100,
			validBlocks:    []uint64{96, 97, 98, 99}, // 96-99 valid
			expectedResult: 99,
		},
		{
			name:           "walkback_after_binary_search",
			l2Unsafe:       95,
			latestUnsafe:   105,
			validBlocks:    []uint64{92, 93}, // 92-93 valid
			expectedResult: 93,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend.reset()

			// Setup l2Unsafe block
			l2UnsafeHash := common.HexToHash(fmt.Sprintf("0x%x", tt.l2Unsafe))

			latestHash := common.HexToHash(fmt.Sprintf("0x%x", tt.latestUnsafe))
			latestL1Origin := eth.BlockID{Hash: common.HexToHash("0xf"), Number: 10 + tt.latestUnsafe - tt.l2Unsafe}
			latestBlockRef := createL2BlockRef(tt.latestUnsafe, latestHash.Hex(), latestL1Origin)

			backend.unsafeHead = latestBlockRef.ID()

			// Setup blocks for binary search
			validBlocksMap := make(map[uint64]bool)
			for _, block := range tt.validBlocks {
				validBlocksMap[block] = true
			}
			// Setup specific expectations for each possible block
			for blockNum := tt.l2Unsafe - 10; blockNum <= tt.latestUnsafe; blockNum++ {
				l1OriginNum := 10 + blockNum - tt.l2Unsafe
				l1OriginHash := fmt.Sprintf("0x%x", l1OriginNum)
				l1Origin := eth.BlockID{Hash: common.HexToHash(l1OriginHash), Number: l1OriginNum}
				l2Block := createL2BlockRef(blockNum, fmt.Sprintf("0x%x", blockNum), l1Origin)
				logger.Info("Setting up block", "l2Block", l2Block, "l1Origin", l1Origin)
				backend.unsafeBlocks[blockNum] = l2Block
				if validBlocksMap[blockNum] {
					// Valid: return matching hash
					backend.l1Blocks[l1OriginNum] = createL1BlockRef(l1OriginNum, l1OriginHash).ID()
				} else {
					// Invalid: return different hash (reorg)
					backend.l1Blocks[l1OriginNum] = createL1BlockRef(l1OriginNum, fmt.Sprintf("0x%x", l1OriginNum+1000)).ID()
				}
			}

			lunsafe, err := tracker.FindResetUnsafeHeadTarget(ctx,
				eth.BlockID{
					Hash:   l2UnsafeHash,
					Number: tt.l2Unsafe,
				})

			if tt.expectedError != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.expectedError)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedResult, lunsafe.Number)
			}
		})
	}
}

// Helper functions to create test data
func createL1BlockRef(number uint64, hash string) eth.L1BlockRef {
	return eth.L1BlockRef{
		Hash:       common.HexToHash(hash),
		Number:     number,
		ParentHash: common.HexToHash("0x0"),
		Time:       1000000 + number*12, // 12 second block time
	}
}

func createL2BlockRef(number uint64, hash string, l1Origin eth.BlockID) eth.L2BlockRef {
	return eth.L2BlockRef{
		Hash:           common.HexToHash(hash),
		Number:         number,
		ParentHash:     common.HexToHash("0x0"),
		Time:           1000000 + number*2, // 2 second block time
		L1Origin:       l1Origin,
		SequenceNumber: 0,
	}
}
