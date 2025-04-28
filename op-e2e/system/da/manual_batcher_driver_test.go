package da

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum-optimism/optimism/op-batcher/batcher"
	op_e2e "github.com/ethereum-optimism/optimism/op-e2e"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/geth"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/wait"
	"github.com/ethereum-optimism/optimism/op-e2e/system/e2esys"
	"github.com/ethereum-optimism/optimism/op-e2e/system/helpers"
	"github.com/ethereum-optimism/optimism/op-service/txmgr"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"
)

func TestManualBatcherDriver(t *testing.T) {
	op_e2e.InitParallel(t)

	cfg := e2esys.DefaultSystemConfig(t)
	sys, err := cfg.Start(t)
	require.NoError(t, err, "Error starting up system")

	rollupClient := sys.RollupClient("verifier")

	l2Seq := sys.NodeClient("sequencer")
	l2Verif := sys.NodeClient("verifier")

	// retrieve the initial sync status
	seqStatus, err := rollupClient.SyncStatus(context.Background())
	require.NoError(t, err)

	nonce := uint64(0)
	sendTx := func() *types.Receipt {
		// Submit TX to L2 sequencer node
		receipt := helpers.SendL2Tx(t, cfg, l2Seq, cfg.Secrets.Alice, func(opts *helpers.TxOpts) {
			opts.ToAddr = &common.Address{0xff, 0xff}
			opts.Value = big.NewInt(1_000_000_000)
			opts.Nonce = nonce
		})
		nonce++
		return receipt
	}
	// send a transaction
	receipt := sendTx()

	// wait until the block the tx was first included in shows up in the safe chain on the verifier
	safeBlockInclusionDuration := time.Duration(6*cfg.DeployConfig.L1BlockTime) * time.Second
	_, err = geth.WaitForBlock(receipt.BlockNumber, l2Verif)
	require.NoError(t, err, "Waiting for block on verifier")
	require.NoError(t, wait.ForProcessingFullBatch(context.Background(), rollupClient))

	// ensure the safe chain advances
	newSeqStatus, err := rollupClient.SyncStatus(context.Background())
	require.NoError(t, err)
	require.Greater(t, newSeqStatus.SafeL2.Number, seqStatus.SafeL2.Number, "Safe chain did not advance")

	driver := sys.BatchSubmitter.TestDriver()
	// stop the batch submission
	err = driver.StopBatchSubmitting(context.Background())
	require.NoError(t, err)

	// wait for any old safe blocks being submitted / derived
	time.Sleep(safeBlockInclusionDuration)

	// get the initial sync status
	seqStatus, err = rollupClient.SyncStatus(context.Background())
	require.NoError(t, err)

	// send another tx
	sendTx()
	time.Sleep(safeBlockInclusionDuration)

	// ensure that the safe chain does not advance while the batcher is stopped
	newSeqStatus, err = rollupClient.SyncStatus(context.Background())
	require.NoError(t, err)
	require.Equal(t, newSeqStatus.SafeL2.Number, seqStatus.SafeL2.Number, "Safe chain advanced while batcher was stopped")

	// send a third tx
	receipt = sendTx()

	_, err = wait.ForReceiptOK(context.Background(), l2Seq, receipt.TxHash)
	require.NoError(t, err, "Waiting for receipt on sequencer")
	// wait for the tx to be included in an unsafe block

	// manually drive the batcher
	// manually load the blocks into the batcher state
	// We should manually reset state here
	err = driver.LoadBlocksIntoState(context.Background(), receipt.BlockNumber.Uint64(), receipt.BlockNumber.Uint64())
	require.NoError(t, err, "Could not load blocks into batcher state") // fails with "block does not extend existing hain"
	// manually publish data, we need to force the current channel to close
	txQueue := txmgr.NewQueue[batcher.TxRef](context.Background(), driver.Txmgr, driver.Config.MaxPendingTransactions)
	driver.PublishStateToL1(context.Background(), txQueue, make(chan txmgr.TxReceipt[batcher.TxRef]), nil)

	// wait until the block the tx was first included in shows up in the safe chain on the verifier
	_, err = geth.WaitForBlock(receipt.BlockNumber, l2Verif)
	require.NoError(t, err, "Waiting for block on verifier")
	require.NoError(t, wait.ForProcessingFullBatch(context.Background(), rollupClient))

	// ensure that the safe chain advances after driving the batcher manually
	newSeqStatus, err = rollupClient.SyncStatus(context.Background())
	require.NoError(t, err)
	require.Greater(t, newSeqStatus.SafeL2.Number, seqStatus.SafeL2.Number, "Safe chain did not advance after batcher was driven manually")
}
