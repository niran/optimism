package processors

import (
	"context"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum-optimism/optimism/op-service/testutils"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/activation"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/depset"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/superevents"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

type mockLogProcessor struct {
	processLogs func(ctx context.Context, block eth.BlockRef, receipts gethtypes.Receipts) error
}

func (m *mockLogProcessor) ProcessLogs(ctx context.Context, block eth.BlockRef, receipts gethtypes.Receipts) error {
	if m.processLogs != nil {
		return m.processLogs(ctx, block, receipts)
	}
	return nil
}

type mockRewinderActivation struct {
	rewindFunc        func(chain eth.ChainID, headBlock eth.BlockID) error
	latestBlockNum    func(chain eth.ChainID) (num uint64, ok bool)
	acceptedBlockFunc func(chainID eth.ChainID, id eth.BlockID) error
}

func (m *mockRewinderActivation) Rewind(chain eth.ChainID, headBlock eth.BlockID) error {
	if m.rewindFunc != nil {
		return m.rewindFunc(chain, headBlock)
	}
	return nil
}

func (m *mockRewinderActivation) LatestBlockNum(chain eth.ChainID) (num uint64, ok bool) {
	if m.latestBlockNum != nil {
		return m.latestBlockNum(chain)
	}
	return 0, false
}

func (m *mockRewinderActivation) AcceptedBlock(chainID eth.ChainID, id eth.BlockID) error {
	if m.acceptedBlockFunc != nil {
		return m.acceptedBlockFunc(chainID, id)
	}
	return nil
}

// TestProcessActivationDetection verifies that the ChainProcessor correctly detects
// interop activation and emits the appropriate event
func TestProcessActivationDetection(t *testing.T) {
	logger := testlog.Logger(t, log.LvlInfo)
	ctx := context.Background()

	// Create chain processor with interop dependency set
	chainID := eth.ChainIDFromUInt64(1)
	activationTime := uint64(1000)

	// Create dependency set with activation time
	deps := map[eth.ChainID]*depset.StaticConfigDependency{
		chainID: {
			ChainIndex:     types.ChainIndex(1),
			ActivationTime: activationTime,
		},
	}

	depSet, err := depset.NewStaticConfigDependencySet(deps)
	require.NoError(t, err)

	// Create activation checker
	activationCheck := activation.NewCheck(depSet, logger)

	// Create processor components
	logProcessor := &mockLogProcessor{}
	rewinder := &mockRewinderActivation{
		latestBlockNum: func(chain eth.ChainID) (uint64, bool) {
			return 0, true
		},
	}

	// Create processor
	processor := NewChainProcessor(ctx, logger, chainID, logProcessor, rewinder)

	// Set activation checker
	processor.SetActivationCheck(activationCheck)

	// Create emitter to capture events
	emitter := new(testutils.MockEmitter)
	processor.AttachEmitter(emitter)

	// Create test blocks
	preActivationBlock := eth.BlockRef{
		Hash:   common.HexToHash("0x1111"),
		Number: 99,
		Time:   activationTime - 10,
	}

	activationBlock := eth.BlockRef{
		Hash:       common.HexToHash("0x2222"),
		Number:     100,
		ParentHash: preActivationBlock.Hash,
		Time:       activationTime + 1, // Just past activation threshold
	}

	postActivationBlock := eth.BlockRef{
		Hash:       common.HexToHash("0x3333"),
		Number:     101,
		ParentHash: activationBlock.Hash,
		Time:       activationTime + 10,
	}

	// Create empty receipts for testing
	emptyReceipts := gethtypes.Receipts{}

	// Process pre-activation block
	err = processor.process(ctx, preActivationBlock, emptyReceipts)
	require.NoError(t, err, "Processing pre-activation block should not error")

	// Set up expectation for activation event before processing the block
	emitter.On("Emit", mock.MatchedBy(func(ev interface{}) bool {
		activationEvent, ok := ev.(superevents.InteropActivationEvent)
		if !ok {
			return false
		}
		return activationEvent.ChainID == chainID &&
			activationEvent.ActivationBlock == activationBlock &&
			activationEvent.PreviousBlock == preActivationBlock
	})).Once()

	// Process activation block
	err = processor.process(ctx, activationBlock, emptyReceipts)
	require.NoError(t, err, "Processing activation block should not error")

	// Reset expectations for post-activation block (no event expected)
	emitter.AssertExpectations(t)

	// Process post-activation block - no activation event expected
	err = processor.process(ctx, postActivationBlock, emptyReceipts)
	require.NoError(t, err, "Processing post-activation block should not error")
}

// TestActivationWithNoPreBlock tests activation detection when there's no previous block
func TestProcessActivationWithNoPreBlock(t *testing.T) {
	logger := testlog.Logger(t, log.LvlInfo)
	ctx := context.Background()

	// Create chain processor with interop dependency set
	chainID := eth.ChainIDFromUInt64(1)
	activationTime := uint64(1000)

	// Create dependency set with activation time
	deps := map[eth.ChainID]*depset.StaticConfigDependency{
		chainID: {
			ChainIndex:     types.ChainIndex(1),
			ActivationTime: activationTime,
		},
	}

	depSet, err := depset.NewStaticConfigDependencySet(deps)
	require.NoError(t, err)

	// Create activation checker
	activationCheck := activation.NewCheck(depSet, logger)

	// Create processor components
	logProcessor := &mockLogProcessor{}
	rewinder := &mockRewinderActivation{
		latestBlockNum: func(chain eth.ChainID) (uint64, bool) {
			return 0, true
		},
	}

	// Create processor
	processor := NewChainProcessor(ctx, logger, chainID, logProcessor, rewinder)

	// Set activation checker
	processor.SetActivationCheck(activationCheck)

	// Create emitter to capture events
	emitter := new(testutils.MockEmitter)
	processor.AttachEmitter(emitter)

	// Create activation block (first block with no previous block)
	activationBlock := eth.BlockRef{
		Hash:   common.HexToHash("0x2222"),
		Number: 100,
		Time:   activationTime + 1, // Just past activation threshold
	}

	// Create empty receipts for testing
	emptyReceipts := gethtypes.Receipts{}

	// Set up expectation for activation event
	emitter.On("Emit", mock.MatchedBy(func(ev interface{}) bool {
		activationEvent, ok := ev.(superevents.InteropActivationEvent)
		if !ok {
			return false
		}
		return activationEvent.ChainID == chainID &&
			activationEvent.ActivationBlock == activationBlock &&
			activationEvent.PreviousBlock == eth.BlockRef{}
	})).Once()

	// Process activation block with no previous block
	err = processor.process(ctx, activationBlock, emptyReceipts)
	require.NoError(t, err, "Processing activation block should not error")
	emitter.AssertExpectations(t)
}
