package proofs

import (
	"testing"

	"github.com/ethereum-optimism/optimism/op-chain-ops/genesis"
	actionsHelpers "github.com/ethereum-optimism/optimism/op-e2e/actions/helpers"
	"github.com/ethereum-optimism/optimism/op-e2e/actions/proofs/helpers"
	"github.com/ethereum-optimism/optimism/op-e2e/bindings"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/predeploys"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/stretchr/testify/require"
)

func Test_ProgramAction_DataGasPerTokenConsistency(gt *testing.T) {
	type testCase int64

	const (
		NormalTx testCase = iota
		JovianTransitionBlock
	)

	runJovianDerivationTest := func(gt *testing.T, testCfg *helpers.TestCfg[testCase]) {
		t := actionsHelpers.NewDefaultTesting(gt)
		deployConfigOverrides := func(dp *genesis.DeployConfig) {}

		var testDataGasPerToken uint32 = 16 // Default value

		if testCfg.Custom == JovianTransitionBlock {
			deployConfigOverrides = func(dp *genesis.DeployConfig) {
				dp.L1PragueTimeOffset = ptrCalldataGas(hexutil.Uint64(0))
				dp.L2GenesisJovianTimeOffset = ptrCalldataGas(hexutil.Uint64(13))
			}
		}

		env := helpers.NewL2FaultProofEnv(t, testCfg, helpers.NewTestParams(), helpers.NewBatcherCfg(), deployConfigOverrides)

		sysCfgContract, err := bindings.NewSystemConfig(env.Sd.RollupCfg.L1SystemConfigAddress, env.Miner.EthClient())
		require.NoError(t, err)

		sysCfgOwner, err := bind.NewKeyedTransactorWithChainID(env.Dp.Secrets.Deployer, env.Sd.RollupCfg.L1ChainID)
		require.NoError(t, err)

		// Update the calldata gas per compressed byte parameter
		testDataGasPerToken = 32 // Update to new value
		_, err = sysCfgContract.SetDataGasPerToken(sysCfgOwner, testDataGasPerToken)
		require.NoError(t, err)

		env.Miner.ActL1StartBlock(12)(t)
		env.Miner.ActL1IncludeTx(env.Dp.Addresses.Deployer)(t)
		env.Miner.ActL1EndBlock(t)

		// sequence L2 blocks, and submit with new batcher
		env.Sequencer.ActL1HeadSignal(t)
		env.Sequencer.ActBuildToL1Head(t)
		env.BatchAndMine(t)

		env.Sequencer.ActL1HeadSignal(t)

		// Check initial L1Block state
		l1BlockContract, err := bindings.NewL1Block(predeploys.L1BlockAddr, env.Engine.EthClient())
		require.NoError(t, err)

		initialDataGas, err := l1BlockContract.DataGasPerToken(nil)
		require.NoError(t, err)

		switch testCfg.Custom {
		case NormalTx, JovianTransitionBlock:
			// Send an L2 tx
			env.Sequencer.ActL2StartBlock(t)
			env.Alice.L2.ActResetTxOpts(t)
			env.Alice.L2.ActSetTxToAddr(&env.Dp.Addresses.Bob)(t)
			env.Alice.L2.ActMakeTx(t)
			// we usually don't include txs in the transition block, so we force-include it
			env.Engine.ActL2IncludeTxIgnoreForcedEmpty(env.Alice.Address())(t)
			env.Sequencer.ActL2EndBlock(t)

			if testCfg.Custom == JovianTransitionBlock {
				require.True(t, env.Sd.RollupCfg.IsJovianActivationBlock(env.Sequencer.L2Unsafe().Time))
			}
		}

		env.BatchAndMine(t)
		env.Sequencer.ActL1HeadSignal(t)
		env.Sequencer.ActL2PipelineFull(t)

		l2SafeHead := env.Engine.L2Chain().CurrentSafeBlock()
		l2UnsafeHead := env.Engine.L2Chain().CurrentHeader()

		// Verify that the parameter was properly updated in L1Block after sequencing
		finalDataGas, err := l1BlockContract.DataGasPerToken(nil)
		require.NoError(t, err)

		if env.Sd.RollupCfg.IsJovian(l2UnsafeHead.Time) {
			// After Jovian activation, the parameter should be updated
			require.Equal(t, testDataGasPerToken, finalDataGas)
		} else {
			// Before Jovian activation, the parameter might still be the initial value
			require.Equal(t, initialDataGas, finalDataGas)
		}

		require.Equal(t, eth.HeaderBlockID(l2SafeHead), eth.HeaderBlockID(l2UnsafeHead), "derivation leads to the same block")

		env.RunFaultProofProgramFromGenesis(t, l2SafeHead.Number.Uint64(), testCfg.CheckResult, testCfg.InputParams...)
	}

	matrix := helpers.NewMatrix[testCase]()
	matrix.AddDefaultTestCasesWithName("NormalTx", NormalTx, helpers.NewForkMatrix(helpers.Isthmus), runJovianDerivationTest)
	matrix.AddDefaultTestCasesWithName("JovianTransitionBlock", JovianTransitionBlock, helpers.NewForkMatrix(helpers.Isthmus), runJovianDerivationTest)
	matrix.Run(gt)
}

func ptrCalldataGas[T any](v T) *T {
	return &v
}