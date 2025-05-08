package interop

import (
	"testing"
	"time"

	"github.com/ethereum-optimism/optimism/devnet-sdk/devstack/devtest"
	"github.com/ethereum-optimism/optimism/devnet-sdk/devstack/presets"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/stretchr/testify/require"
)

func TestL2CLResync(gt *testing.T) {
	var SimpleInterop presets.TestSetup[*presets.SimpleInterop]
	presets.InitDefaultOrchestrator(presets.NewSimpleInterop(&SimpleInterop))

	t := devtest.SerialT(gt)
	sys := SimpleInterop(t)

	logger := sys.T.Logger()

	logger = logger.With("XXX", "XXX")
	logger.Info("check unsafe chains are advancing")

	waitTime := time.Second * 3
	logger.Info("wait until passing genesis")
	var prevBlockA, prevBlockB eth.BlockRef
	require.Eventually(t, func() bool {
		blockA := sys.L2ELA.BlockRefByLabel(eth.Unsafe)
		blockB := sys.L2ELB.BlockRefByLabel(eth.Unsafe)
		logger.Info("chain A", "blockNum", blockA.Number, "tip", blockA)
		logger.Info("chain B", "blockNum", blockB.Number, "tip", blockB)

		prevBlockA, prevBlockB = blockA, blockB
		return blockA.Number > 0 && blockB.Number > 0
	}, 16*time.Second, waitTime)

	time.Sleep(waitTime)
	logger.Info("check unsafe chains are advancing")
	require.Never(t, func() bool {
		blockA := sys.L2ELA.BlockRefByLabel(eth.Unsafe)
		blockB := sys.L2ELB.BlockRefByLabel(eth.Unsafe)
		logger.Info("chain A", "blockNum", blockA.Number, "tip", blockA)
		logger.Info("chain B", "blockNum", blockB.Number, "tip", blockB)

		advanced := prevBlockA.Number < blockA.Number && prevBlockB.Number < blockB.Number
		prevBlockA, prevBlockB = blockA, blockB
		return !advanced
	}, 10*time.Second, waitTime)

	logger.Info("stop L2CL nodes")
	sys.L2CLA.Stop()
	sys.L2CLB.Stop()

	logger.Info("make sure L2ELs does not advance")
	require.Eventually(t, func() bool {
		blockA := sys.L2ELA.BlockRefByLabel(eth.Unsafe)
		blockB := sys.L2ELB.BlockRefByLabel(eth.Unsafe)
		logger.Info("chain A", "blockNum", blockA.Number, "tip", blockA)
		logger.Info("chain B", "blockNum", blockB.Number, "tip", blockB)
		isStatic := prevBlockA.Hash == blockA.Hash && prevBlockB.Hash == blockB.Hash
		prevBlockA, prevBlockB = blockA, blockB
		return isStatic
	}, 10*time.Second, waitTime)

	logger.Info("restart L2CL nodes")
	sys.L2CLA.Restart()
	sys.L2CLB.Restart()

	// L2CL may advance a few blocks without supervisor connection, but eventually it will stop without the connection
	// we must check that unsafe head is advancing due to reconnection
	logger.Info("boot up L2CL nodes")
	require.Eventually(t, func() bool {
		blockA := sys.L2ELA.BlockRefByLabel(eth.Unsafe)
		blockB := sys.L2ELB.BlockRefByLabel(eth.Unsafe)
		logger.Info("chain A", "blockNum", blockA.Number, "tip", blockA)
		logger.Info("chain B", "blockNum", blockB.Number, "tip", blockB)
		advanced := prevBlockA.Number < blockA.Number && prevBlockB.Number < blockB.Number
		prevBlockA, prevBlockB = blockA, blockB
		return advanced
	}, 15*time.Second, waitTime)

	// supervisor will attempt to reconnect with L2CLs at this point because L2CL ws endpoint is recovered
	logger.Info("check unsafe chains are advancing again")
	require.Never(t, func() bool {
		blockA := sys.L2ELA.BlockRefByLabel(eth.Unsafe)
		blockB := sys.L2ELB.BlockRefByLabel(eth.Unsafe)
		logger.Info("chain A", "blockNum", blockA.Number, "tip", blockA)
		logger.Info("chain B", "blockNum", blockB.Number, "tip", blockB)
		advanced := prevBlockA.Number < blockA.Number && prevBlockB.Number < blockB.Number
		prevBlockA, prevBlockB = blockA, blockB
		return !advanced
	}, 15*time.Second, waitTime)

	// supervisor successfully connected with managed L2CLs
	logger.Info("done")
}

// condition may be composable
// wait time
