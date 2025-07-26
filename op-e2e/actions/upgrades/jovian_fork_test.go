package upgrades

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/op-e2e/actions/helpers"
	upgradeHelpers "github.com/ethereum-optimism/optimism/op-e2e/actions/upgrades/helpers"
	"github.com/ethereum-optimism/optimism/op-e2e/bindings"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils"
	"github.com/ethereum-optimism/optimism/op-node/rollup/derive"
	"github.com/ethereum-optimism/optimism/op-node/rollup/sync"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/predeploys"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
)

// DataGasPerTokenChange tests that the data gas per token parameter can be 
// updated to adjust the L1 data fee calculation for compressed calldata, and that the L2 node properly
// adopts the new parameter value when the L1 change is processed.
func DataGasPerTokenChange(gt *testing.T, deltaTimeOffset *hexutil.Uint64) {
	t := helpers.NewDefaultTesting(gt)
	dp := e2eutils.MakeDeployParams(t, helpers.DefaultRollupTestParams())
	upgradeHelpers.ApplyDeltaTimeOffset(dp, deltaTimeOffset)

	// Activate Jovian fork to enable data gas per token parameter
	dp.DeployConfig.L2GenesisJovianTimeOffset = ptr(hexutil.Uint64(0))

	sd := e2eutils.Setup(t, dp, helpers.DefaultAlloc)
	log := testlog.Logger(t, log.LevelDebug)
	miner, seqEngine, sequencer := helpers.SetupSequencerTest(t, sd, log)
	batcher := helpers.NewL2Batcher(log, sd.RollupCfg, helpers.DefaultBatcherCfg(dp),
		sequencer.RollupClient(), miner.EthClient(), seqEngine.EthClient(), seqEngine.EngineClient(t, sd.RollupCfg))

	sequencer.ActL2PipelineFull(t)

	// new L1 block, with new L2 chain
	miner.ActEmptyBlock(t)
	sequencer.ActL1HeadSignal(t)
	sequencer.ActBuildToL1Head(t)

	// Check initial data gas per token value
	l1BlockContract, err := bindings.NewL1Block(predeploys.L1BlockAddr, seqEngine.EthClient())
	require.NoError(t, err)

	initialDataGas, err := l1BlockContract.DataGasPerToken(nil)
	require.NoError(t, err)
	require.Equal(t, uint32(16), initialDataGas, "initial data gas per token should be 16")

	// confirm L2 chain on L1
	batcher.ActSubmitAll(t)
	miner.ActL1StartBlock(12)(t)
	miner.ActL1IncludeTx(dp.Addresses.Batcher)(t)
	miner.ActL1EndBlock(t)

	sysCfgContract, err := bindings.NewSystemConfig(sd.RollupCfg.L1SystemConfigAddress, miner.EthClient())
	require.NoError(t, err)

	sysCfgOwner, err := bind.NewKeyedTransactorWithChainID(dp.Secrets.Deployer, sd.RollupCfg.L1ChainID)
	require.NoError(t, err)

	// Update data gas per token from 16 (default) to 32
	newDataGas := uint32(32)
	_, err = sysCfgContract.SetDataGasPerToken(sysCfgOwner, newDataGas)
	require.NoError(t, err)

	// include the calldata gas parameter change tx in L1
	miner.ActL1StartBlock(12)(t)
	miner.ActL1IncludeTx(dp.Addresses.Deployer)(t)
	miner.ActL1EndBlock(t)

	// build empty L2 chain, up to but excluding the L2 block with the L1 origin that processes the parameter change
	sequencer.ActL1HeadSignal(t)
	sequencer.ActBuildToL1HeadExcl(t)

	engCl := seqEngine.EngineClient(t, sd.RollupCfg)
	envelope, err := engCl.PayloadByLabel(t.Ctx(), eth.Unsafe)
	require.NoError(t, err)
	sysCfg, err := derive.PayloadToSystemConfig(sd.RollupCfg, envelope.ExecutionPayload)
	require.NoError(t, err)
	require.Equal(t, sd.RollupCfg.Genesis.SystemConfig, sysCfg, "still have genesis system config before we adopt the L1 block with parameter change")

	// Verify L1Block still has the old value
	currentDataGas, err := l1BlockContract.DataGasPerToken(nil)
	require.NoError(t, err)
	require.Equal(t, initialDataGas, currentDataGas, "data gas per token should still be the initial value")

	// Now build a block that adopts the L1 origin with the parameter change
	sequencer.ActL2StartBlock(t)
	sequencer.ActL2EndBlock(t)

	envelope, err = engCl.PayloadByLabel(t.Ctx(), eth.Unsafe)
	require.NoError(t, err)
	sysCfg, err = derive.PayloadToSystemConfig(sd.RollupCfg, envelope.ExecutionPayload)
	require.NoError(t, err)

	// Verify the L1Block contract now has the updated value
	updatedDataGas, err := l1BlockContract.DataGasPerToken(nil)
	require.NoError(t, err)
	require.Equal(t, newDataGas, updatedDataGas, "data gas per token should be updated")

	// build more L2 blocks, with new L1 origin
	miner.ActEmptyBlock(t)
	sequencer.ActL1HeadSignal(t)
	sequencer.ActBuildToL1Head(t)

	// Verify the new parameter is persistent
	persistentDataGas, err := l1BlockContract.DataGasPerToken(nil)
	require.NoError(t, err)
	require.Equal(t, newDataGas, persistentDataGas, "data gas per token should remain updated")

	// Submit everything to L1 and verify that a verifier can sync and reproduce it
	batcher.ActSubmitAll(t)
	miner.ActL1StartBlock(12)(t)
	miner.ActL1IncludeTx(dp.Addresses.Batcher)(t)
	miner.ActL1EndBlock(t)

	verifierEngine, verifier := helpers.SetupVerifier(t, sd, log, miner.L1Client(t, sd.RollupCfg), miner.BlobStore(), &sync.Config{})
	verifier.ActL2PipelineFull(t)

	require.Equal(t, sequencer.L2Unsafe(), verifier.L2Safe(), "verifier stays in sync, even with calldata gas parameter changes")

	// Verify that the verifier also has the correct parameter value
	verifierL1BlockContract, err := bindings.NewL1Block(predeploys.L1BlockAddr, verifierEngine.EthClient())
	require.NoError(t, err)

	verifierDataGas, err := verifierL1BlockContract.DataGasPerToken(nil)
	require.NoError(t, err)
	require.Equal(t, newDataGas, verifierDataGas, "verifier should have the same updated data gas parameter")
}

func ptr[T any](v T) *T {
	return &v
}