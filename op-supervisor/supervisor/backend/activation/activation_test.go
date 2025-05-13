package activation

import (
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/depset"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

func testDependencySet(chains []uint64, activationTimes map[uint64]uint64) depset.DependencySet {
	deps := make(map[eth.ChainID]*depset.StaticConfigDependency)
	for _, chainIDUint := range chains {
		chainID := eth.ChainIDFromUInt64(chainIDUint)
		activationTime := uint64(0)
		if activationTimes != nil {
			if time, ok := activationTimes[chainIDUint]; ok {
				activationTime = time
			}
		}
		deps[chainID] = &depset.StaticConfigDependency{
			ChainIndex:     0, // Not important for this test
			ActivationTime: activationTime,
			HistoryMinTime: 0,
		}
	}
	depSet, _ := depset.NewStaticConfigDependencySet(deps)
	return depSet
}

func TestIsActivationBlockCases(t *testing.T) {
	logger := testlog.Logger(t, log.LvlInfo)

	// Create a mock activation check with activation time of 1000
	ds := testDependencySet([]uint64{1}, map[uint64]uint64{1: 1000})
	check := NewCheck(ds, logger)

	// Test cases
	testCases := []struct {
		name         string
		block        eth.BlockRef
		prevBlock    eth.BlockRef
		chainID      eth.ChainID
		isActivation bool
	}{
		{
			name: "Not active block",
			block: eth.BlockRef{
				Number: 100,
				Time:   900, // Before activation
			},
			prevBlock: eth.BlockRef{
				Number: 99,
				Time:   890, // Before activation
			},
			chainID:      eth.ChainIDFromUInt64(1),
			isActivation: false,
		},
		{
			name: "Activation block",
			block: eth.BlockRef{
				Number: 100,
				Time:   1001, // After activation
			},
			prevBlock: eth.BlockRef{
				Number: 99,
				Time:   999, // Before activation
			},
			chainID:      eth.ChainIDFromUInt64(1),
			isActivation: true,
		},
		{
			name: "Already active block",
			block: eth.BlockRef{
				Number: 101,
				Time:   1100, // After activation
			},
			prevBlock: eth.BlockRef{
				Number: 100,
				Time:   1050, // After activation
			},
			chainID:      eth.ChainIDFromUInt64(1),
			isActivation: false,
		},
		{
			name: "First block after startup, active",
			block: eth.BlockRef{
				Number: 100,
				Time:   1100, // After activation
			},
			prevBlock:    eth.BlockRef{}, // Empty struct, first block
			chainID:      eth.ChainIDFromUInt64(1),
			isActivation: true,
		},
		{
			name: "First block after startup, not active",
			block: eth.BlockRef{
				Number: 100,
				Time:   900, // Before activation
			},
			prevBlock:    eth.BlockRef{}, // Empty struct, first block
			chainID:      eth.ChainIDFromUInt64(1),
			isActivation: false,
		},
		{
			name: "Unknown chain",
			block: eth.BlockRef{
				Number: 100,
				Time:   1001,
			},
			prevBlock: eth.BlockRef{
				Number: 99,
				Time:   999,
			},
			chainID:      eth.ChainIDFromUInt64(2), // Not in the dep set
			isActivation: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := check.IsActivationBlock(tc.block, tc.prevBlock, tc.chainID)
			require.Equal(t, tc.isActivation, result, "IsActivationBlock should return expected result")
		})
	}
}

func TestNilCheck(t *testing.T) {
	// Test with nil checker
	var check *Check

	block := eth.BlockRef{
		Number: 100,
		Time:   1001,
	}
	prevBlock := eth.BlockRef{
		Number: 99,
		Time:   999,
	}
	chainID := eth.ChainIDFromUInt64(1)

	result := check.IsActivationBlock(block, prevBlock, chainID)
	require.False(t, result, "Nil checker should return false")
}

func TestCheckWithNilDependencySet(t *testing.T) {
	logger := testlog.Logger(t, log.LvlInfo)

	// Create a check with nil dependency set
	check := &Check{
		depSet: nil,
		logger: logger,
	}

	block := eth.BlockRef{
		Number: 100,
		Time:   1001,
	}
	prevBlock := eth.BlockRef{
		Number: 99,
		Time:   999,
	}
	chainID := eth.ChainIDFromUInt64(1)

	result := check.IsActivationBlock(block, prevBlock, chainID)
	require.False(t, result, "Check with nil dependency set should return false")
}

func TestIsActivationBlock(t *testing.T) {
	baseTime := uint64(time.Now().Unix() + 60)
	chainID := eth.ChainIDFromUInt64(1)

	createDepSet := func(activationTimes map[eth.ChainID]uint64, messageExpiryWindow uint64) (depset.DependencySet, error) {
		deps := make(map[eth.ChainID]*depset.StaticConfigDependency)
		for chainID, activationTime := range activationTimes {
			deps[chainID] = &depset.StaticConfigDependency{
				ChainIndex:     types.ChainIndex(int(chainID[0])),
				ActivationTime: activationTime,
				HistoryMinTime: 0,
			}
		}
		return depset.NewStaticConfigDependencySetWithMessageExpiryOverride(deps, messageExpiryWindow)
	}

	depSet, err := createDepSet(map[eth.ChainID]uint64{
		chainID: baseTime,
	}, 3600)
	require.NoError(t, err)

	logger := testlog.Logger(t, log.LvlInfo)
	activationCheck := NewCheck(depSet, logger)

	// Create test blocks
	preActivationBlock := eth.BlockRef{
		Number: 100,
		Time:   baseTime - 1,
		Hash:   [32]byte{0x1},
	}
	activationBlock := eth.BlockRef{
		Number:     101,
		Time:       baseTime + 1,
		Hash:       [32]byte{0x2},
		ParentHash: preActivationBlock.Hash,
	}
	postActivationBlock := eth.BlockRef{
		Number:     102,
		Time:       baseTime + 10,
		Hash:       [32]byte{0x3},
		ParentHash: activationBlock.Hash,
	}

	// Test cases
	testCases := []struct {
		name        string
		block       eth.BlockRef
		prevBlock   eth.BlockRef
		expectation bool
	}{
		{
			"Pre-activation block is not activation",
			preActivationBlock,
			eth.BlockRef{},
			false,
		},
		{
			"Activation block with no previous block is considered activation",
			activationBlock,
			eth.BlockRef{},
			true,
		},
		{
			"Activation block with previous block is detected as activation",
			activationBlock,
			preActivationBlock,
			true,
		},
		{
			"Post-activation block with activation block as parent is not activation",
			postActivationBlock,
			activationBlock,
			false,
		},
		{
			"Post-activation block with no previous block reference is considered activation",
			postActivationBlock,
			eth.BlockRef{},
			true,
		},
		{
			"Post-activation block with pre-activation block is considered activation",
			postActivationBlock,
			preActivationBlock,
			true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			isActivation := activationCheck.IsActivationBlock(tc.block, tc.prevBlock, chainID)
			require.Equal(t, tc.expectation, isActivation)
		})
	}
}
