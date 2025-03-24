package interop

import (
	"math/big"
	"sync"
	"testing"
	"time"

	"github.com/ethereum-optimism/optimism/devnet-sdk/contracts/constants"
	"github.com/ethereum-optimism/optimism/devnet-sdk/system"
	"github.com/ethereum-optimism/optimism/devnet-sdk/testing/systest"
	"github.com/ethereum-optimism/optimism/devnet-sdk/testing/testlib/validators"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/txintent"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"
)

// initAndExecMsg tests below scenario:
// Transaction initiates, and then executes message
func initAndExecMsg(
	l2ChainNums int,
	walletGetters []validators.WalletGetter,
) systest.InteropSystemTestFunc {
	return func(t systest.T, sys system.InteropSystem) {
		ctx, rng, logger, _, wallets, opts := DefaultInteropSetup(t, sys, l2ChainNums, walletGetters)

		eventLoggerAddress, err := DeployEventLogger(ctx, wallets[0], logger)
		require.NoError(t, err)

		// Intent to initiate message(or emit event) on chain A
		txA := txintent.NewIntent[*txintent.InitTrigger, *txintent.InteropOutput](opts[0])
		randomInitTrigger := RandomInitTrigger(rng, eventLoggerAddress, 3, 10)
		txA.Content.Set(randomInitTrigger)

		// Trigger single event
		receiptA, err := txA.PlannedTx.Included.Eval(ctx)
		require.NoError(t, err)
		logger.Info("initiate message included", "block", receiptA.BlockHash)

		// Intent to validate message on chain B
		txB := txintent.NewIntent[*txintent.ExecTrigger, *txintent.InteropOutput](opts[1])
		txB.Content.DependOn(&txA.Result)

		// Single event in tx so index is 0
		txB.Content.Fn(txintent.ExecuteIndexed(constants.CrossL2Inbox, &txA.Result, 0))

		receiptB, err := txB.PlannedTx.Included.Eval(ctx)
		require.NoError(t, err)
		logger.Info("validate message included", "block", receiptB.BlockHash)

		// Check single ExecutingMessage triggered
		require.Equal(t, 1, len(receiptB.Logs))
	}
}

// initAndExecMultipleMsg tests below scenario:
// Transaction initiates and executes multiple messages of self
func initAndExecMultipleMsg(
	l2ChainNums int,
	walletGetters []validators.WalletGetter,
) systest.InteropSystemTestFunc {
	return func(t systest.T, sys system.InteropSystem) {
		ctx, rng, logger, _, wallets, opts := DefaultInteropSetup(t, sys, l2ChainNums, walletGetters)

		eventLoggerAddress, err := DeployEventLogger(ctx, wallets[0], logger)
		require.NoError(t, err)

		// Intent to initiate two message(or emit event) on chain A
		initCalls := []txintent.Call{
			RandomInitTrigger(rng, eventLoggerAddress, 1, 15),
			RandomInitTrigger(rng, eventLoggerAddress, 2, 13),
		}
		txA := txintent.NewIntent[*txintent.MultiTrigger, *txintent.InteropOutput](opts[0])
		txA.Content.Set(&txintent.MultiTrigger{Emitter: constants.MultiCall3, Calls: initCalls})

		// Trigger two events
		receiptA, err := txA.PlannedTx.Included.Eval(ctx)
		require.NoError(t, err)
		logger.Info("initiate messages included", "block", receiptA.BlockHash)
		require.Equal(t, 2, len(receiptA.Logs))

		// Intent to validate messages on chain B
		txB := txintent.NewIntent[*txintent.MultiTrigger, *txintent.InteropOutput](opts[1])
		txB.Content.DependOn(&txA.Result)

		// Two events in tx so use every index
		indexes := []int{0, 1}
		txB.Content.Fn(txintent.ExecuteIndexeds(constants.MultiCall3, constants.CrossL2Inbox, &txA.Result, indexes))

		receiptB, err := txB.PlannedTx.Included.Eval(ctx)
		require.NoError(t, err)
		logger.Info("validate messages included", "block", receiptB.BlockHash)

		// Check two ExecutingMessage triggered
		require.Equal(t, 2, len(receiptB.Logs))
	}
}

// execSameMsgTwice tests below scenario:
// Transaction that executes the same message twice.
func execSameMsgTwice(
	l2ChainNums int,
	walletGetters []validators.WalletGetter,
) systest.InteropSystemTestFunc {
	return func(t systest.T, sys system.InteropSystem) {
		ctx, rng, logger, _, wallets, opts := DefaultInteropSetup(t, sys, l2ChainNums, walletGetters)

		eventLoggerAddress, err := DeployEventLogger(ctx, wallets[0], logger)
		require.NoError(t, err)

		// Intent to initiate message(or emit event) on chain A
		txA := txintent.NewIntent[*txintent.InitTrigger, *txintent.InteropOutput](opts[0])
		randomInitTrigger := RandomInitTrigger(rng, eventLoggerAddress, 3, 10)
		txA.Content.Set(randomInitTrigger)

		// Trigger single event
		receiptA, err := txA.PlannedTx.Included.Eval(ctx)
		require.NoError(t, err)
		logger.Info("initiate message included", "block", receiptA.BlockHash)

		// Intent to validate same message two times on chain B
		txB := txintent.NewIntent[*txintent.MultiTrigger, *txintent.InteropOutput](opts[1])
		txB.Content.DependOn(&txA.Result)

		// Single event in tx so indexes are 0, 0
		indexes := []int{0, 0}
		txB.Content.Fn(txintent.ExecuteIndexeds(constants.MultiCall3, constants.CrossL2Inbox, &txA.Result, indexes))

		receiptB, err := txB.PlannedTx.Included.Eval(ctx)
		require.NoError(t, err)
		logger.Info("validate messages included", "block", receiptB.BlockHash)

		// Check two ExecutingMessage triggered
		require.Equal(t, 2, len(receiptB.Logs))
	}
}

// initAndExecSameTimestamp tests below scenario:
// Transaction executes message within same block (same timestamp)
func initAndExecSameTimestamp(
	l2ChainNums int,
	walletGetters []validators.WalletGetter,
) systest.InteropSystemTestFunc {
	return func(t systest.T, sys system.InteropSystem) {
		ctx, rng, logger, _, wallets, opts := DefaultInteropSetup(t, sys, l2ChainNums, walletGetters)

		eventLoggerAddress, err := DeployEventLogger(ctx, wallets[0], logger)
		require.NoError(t, err)

		// Intent to initiate message(or emit event) on chain A
		txA := txintent.NewIntent[*txintent.InitTrigger, *txintent.InteropOutput](opts[0])
		randomInitTrigger := RandomInitTrigger(rng, eventLoggerAddress, 3, 10)
		txA.Content.Set(randomInitTrigger)

		// Fetch chainA's current head to guess where initiate message is included
		againstBlock, err := txA.PlannedTx.AgainstBlock.Eval(ctx)
		require.NoError(t, err)
		logger.Info("againstBlock", "blockNumber", againstBlock.NumberU64())
		chainID, err := txA.PlannedTx.ChainID.Eval(ctx)
		require.NoError(t, err)

		// estimate the result blocknumber, logindex, and timestamp
		receipt := types.Receipt{}
		receipt.Logs = []*types.Log{}
		topics := []common.Hash{}
		for _, topic := range randomInitTrigger.Topics {
			topics = append(topics, common.Hash(topic))
		}
		targetBlockNumber := againstBlock.NumberU64() + 2
		logger.Info("guessing initiate message included", "blockNumber", targetBlockNumber)
		log := &types.Log{
			Address:     eventLoggerAddress,
			BlockNumber: targetBlockNumber,
			Topics:      topics,
			Data:        randomInitTrigger.OpaqueData,
			Index:       0,
		}
		receipt.Logs = append(receipt.Logs, log)
		includedIn := eth.BlockRefFromHeader(&types.Header{Time: againstBlock.Time() + 2*2, Number: new(big.Int).SetUint64(uint64(targetBlockNumber))})

		// never use opts to preserve sets
		txB := txintent.NewIntent[*txintent.ExecTrigger, *txintent.InteropOutput]()
		txB.PlannedTx.IncludedBlock.Set(*includedIn)
		txB.PlannedTx.Included.Set(&receipt)
		txB.PlannedTx.ChainID.Set(chainID)
		txB.Content.Set(&txintent.ExecTrigger{})

		txC := txintent.NewIntent[*txintent.ExecTrigger, *txintent.InteropOutput](opts[1])
		txC.Content.DependOn(&txB.Result)

		// Single event in tx so index is 0
		txC.Content.Fn(txintent.ExecuteIndexed(constants.CrossL2Inbox, &txB.Result, 0))

		var wg sync.WaitGroup

		// waitTime must be zero for making initiate/validate message land on same timestamp
		waitTime := 4
		wg.Add(2)
		go func() {
			defer wg.Done()
			receiptA, err := txA.PlannedTx.Included.Eval(ctx)
			require.NoError(t, err)
			logger.Info("initiate message included", "block", receiptA.BlockHash, "blockNumber", receiptA.BlockNumber)
			require.Equal(t, targetBlockNumber, receiptA.BlockNumber.Uint64())
			logger.Info("guessed block number is correct")
		}()
		go func() {
			time.Sleep(time.Duration(waitTime) * time.Second)
			defer wg.Done()
			receiptC, err := txC.PlannedTx.Included.Eval(ctx)
			require.NoError(t, err)
			logger.Info("validate message included", "block", receiptC.BlockHash, "blockNumber", receiptC.BlockNumber)
		}()
		wg.Wait()
	}
}

// execMsgDifferentTopicCount tests below scenario:
// Execute message that links with initiating message with: 0, 1, 2, 3, or 4 topics in it
func execMsgDifferentTopicCount(
	l2ChainNums int,
	walletGetters []validators.WalletGetter,
) systest.InteropSystemTestFunc {
	return func(t systest.T, sys system.InteropSystem) {
		ctx, rng, logger, _, wallets, opts := DefaultInteropSetup(t, sys, l2ChainNums, walletGetters)

		eventLoggerAddress, err := DeployEventLogger(ctx, wallets[0], logger)
		require.NoError(t, err)

		// Intent to initiate message with differet topic counts on chain A
		initCalls := make([]txintent.Call, 5)
		for topicCnt := range 5 {
			index := topicCnt
			initCalls[index] = RandomInitTrigger(rng, eventLoggerAddress, topicCnt, 10)
		}
		txA := txintent.NewIntent[*txintent.MultiTrigger, *txintent.InteropOutput](opts[0])
		txA.Content.Set(&txintent.MultiTrigger{Emitter: constants.MultiCall3, Calls: initCalls})

		// Trigger five events, each have {0, 1, 2, 3, 4} topics in it
		receiptA, err := txA.PlannedTx.Included.Eval(ctx)
		require.NoError(t, err)
		logger.Info("initiate messages included", "block", receiptA.BlockHash)
		require.Equal(t, 5, len(receiptA.Logs))

		for topicCnt := range 5 {
			index := topicCnt
			require.Equal(t, topicCnt, len(receiptA.Logs[index].Topics))
		}

		// Intent to validate message on chain B
		txB := txintent.NewIntent[*txintent.MultiTrigger, *txintent.InteropOutput](opts[1])
		txB.Content.DependOn(&txA.Result)

		// Five events in tx so use every index
		indexes := []int{0, 1, 2, 3, 4}
		txB.Content.Fn(txintent.ExecuteIndexeds(constants.MultiCall3, constants.CrossL2Inbox, &txA.Result, indexes))

		receiptB, err := txB.PlannedTx.Included.Eval(ctx)
		require.NoError(t, err)
		logger.Info("validate message included", "block", receiptB.BlockHash)

		// Check five ExecutingMessage triggered
		require.Equal(t, 5, len(receiptB.Logs))
	}
}

// execMsgOpagueData tests below scenario:
// Execute message that links with initiating message with: 0, 10KB of opaque event data in it
func execMsgOpagueData(
	l2ChainNums int,
	walletGetters []validators.WalletGetter,
) systest.InteropSystemTestFunc {
	return func(t systest.T, sys system.InteropSystem) {
		ctx, rng, logger, _, wallets, opts := DefaultInteropSetup(t, sys, l2ChainNums, walletGetters)

		eventLoggerAddress, err := DeployEventLogger(ctx, wallets[0], logger)
		require.NoError(t, err)

		// Intent to initiate message with two messages: 0, 10KB of opaque event data
		initCalls := make([]txintent.Call, 2)
		emptyInitTrigger := RandomInitTrigger(rng, eventLoggerAddress, 2, 0)      // 0B
		largeInitTrigger := RandomInitTrigger(rng, eventLoggerAddress, 3, 10_000) // 10KB
		initCalls[0] = emptyInitTrigger
		initCalls[1] = largeInitTrigger

		txA := txintent.NewIntent[*txintent.MultiTrigger, *txintent.InteropOutput](opts[0])
		txA.Content.Set(&txintent.MultiTrigger{Emitter: constants.MultiCall3, Calls: initCalls})

		// Trigger two events
		receiptA, err := txA.PlannedTx.Included.Eval(ctx)
		require.NoError(t, err)
		logger.Info("initiate messages included", "block", receiptA.BlockHash)
		require.Equal(t, 2, len(receiptA.Logs))
		require.Equal(t, emptyInitTrigger.OpaqueData, receiptA.Logs[0].Data)
		require.Equal(t, largeInitTrigger.OpaqueData, receiptA.Logs[1].Data)

		// Intent to validate messages on chain B
		txB := txintent.NewIntent[*txintent.MultiTrigger, *txintent.InteropOutput](opts[1])
		txB.Content.DependOn(&txA.Result)

		// Two events in tx so use every index
		indexes := []int{0, 1}
		txB.Content.Fn(txintent.ExecuteIndexeds(constants.MultiCall3, constants.CrossL2Inbox, &txA.Result, indexes))

		receiptB, err := txB.PlannedTx.Included.Eval(ctx)
		require.NoError(t, err)
		logger.Info("validate messages included", "block", receiptB.BlockHash)

		// Check two ExecutingMessage triggered
		require.Equal(t, 2, len(receiptB.Logs))
	}
}

// execMsgDifferEventIndexInSingleTx tests below scenario:
// Execute message that links with initiating message with: first, random or last event of a tx.
func execMsgDifferEventIndexInSingleTx(
	l2ChainNums int,
	walletGetters []validators.WalletGetter,
) systest.InteropSystemTestFunc {
	return func(t systest.T, sys system.InteropSystem) {
		ctx, rng, logger, _, wallets, opts := DefaultInteropSetup(t, sys, l2ChainNums, walletGetters)

		eventLoggerAddress, err := DeployEventLogger(ctx, wallets[0], logger)
		require.NoError(t, err)

		// Intent to initiate message with multiple messages, all included in single tx
		eventCnt := 10
		initCalls := make([]txintent.Call, eventCnt)
		for index := range eventCnt {
			initCalls[index] = RandomInitTrigger(rng, eventLoggerAddress, rng.Intn(5), rng.Intn(100))
		}

		txA := txintent.NewIntent[*txintent.MultiTrigger, *txintent.InteropOutput](opts[0])
		txA.Content.Set(&txintent.MultiTrigger{Emitter: constants.MultiCall3, Calls: initCalls})

		// Trigger multiple events
		receiptA, err := txA.PlannedTx.Included.Eval(ctx)
		require.NoError(t, err)
		logger.Info("initiate messages included", "block", receiptA.BlockHash)
		require.Equal(t, eventCnt, len(receiptA.Logs))

		// Intent to validate messages on chain B
		txB := txintent.NewIntent[*txintent.MultiTrigger, *txintent.InteropOutput](opts[1])
		txB.Content.DependOn(&txA.Result)

		// Two events in tx so use every index
		// first, random or last event of a tx.
		indexes := []int{0, 1 + rng.Intn(eventCnt-1), eventCnt - 1}
		txB.Content.Fn(txintent.ExecuteIndexeds(constants.MultiCall3, constants.CrossL2Inbox, &txA.Result, indexes))

		receiptB, err := txB.PlannedTx.Included.Eval(ctx)
		require.NoError(t, err)
		logger.Info("validate messages included", "block", receiptB.BlockHash)

		// Check three ExecutingMessage triggered
		require.Equal(t, len(indexes), len(receiptB.Logs))
	}
}

func TestInteropTxTest(t *testing.T) {
	l2ChainNums := 2
	walletGetters, totalValidators := SetupDefaultInteropSystemTest(l2ChainNums)

	tests := []struct {
		name     string
		testFunc systest.InteropSystemTestFunc
	}{
		{"initAndExecMsg", initAndExecMsg(l2ChainNums, walletGetters)},
		{"initAndExecMultipleMsg", initAndExecMultipleMsg(l2ChainNums, walletGetters)},
		{"execSameMsgTwice", execSameMsgTwice(l2ChainNums, walletGetters)},
		{"initAndExecSameTimestamp", initAndExecSameTimestamp(l2ChainNums, walletGetters)},

		{"execMsgDifferentTopicCount", execMsgDifferentTopicCount(l2ChainNums, walletGetters)},
		{"execMsgOpagueData", execMsgOpagueData(l2ChainNums, walletGetters)},
		{"execMsgDifferEventIndexInSingleTx", execMsgDifferEventIndexInSingleTx(l2ChainNums, walletGetters)},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			systest.InteropSystemTest(t,
				test.testFunc,
				totalValidators...,
			)
		})
	}
}
