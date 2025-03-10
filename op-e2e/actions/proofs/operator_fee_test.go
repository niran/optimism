package proofs

import (
	"math/big"
	"testing"

	"github.com/ethereum-optimism/optimism/op-chain-ops/genesis"
	actionsHelpers "github.com/ethereum-optimism/optimism/op-e2e/actions/helpers"
	"github.com/ethereum-optimism/optimism/op-e2e/actions/proofs/helpers"
	"github.com/ethereum-optimism/optimism/op-e2e/bindings"
	"github.com/ethereum-optimism/optimism/op-program/client/claim"
	"github.com/ethereum-optimism/optimism/op-service/predeploys"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"
)

func Test_ProgramAction_OperatorFeeConstistency(gt *testing.T) {
	type testCase int64

	const (
		NormalTx testCase = iota
		DepositTx
		StateRefund
		IsthmusTransitionBlock
	)

	const testOperatorFeeScalar = uint32(20000)
	const testOperatorFeeConstant = uint64(500)
	testStorageUpdateContractAddress := common.HexToAddress("0xffffffff")
	// contract TestSetter {
	//   uint x;
	//   function set(uint _x) public { x = _x }
	// }
	testStorageUpdateContractCode := common.FromHex("0x6080604052348015600e575f80fd5b5060d980601a5f395ff3fe6080604052348015600e575f80fd5b50600436106026575f3560e01c806360fe47b114602a575b5f80fd5b60406004803603810190603c9190607d565b6042565b005b805f8190555050565b5f80fd5b5f819050919050565b605f81604f565b81146068575f80fd5b50565b5f813590506077816058565b92915050565b5f60208284031215608f57608e604b565b5b5f609a84828501606b565b9150509291505056fea2646970667358221220fafa0f1a19c76eab34a04a6b16474af0eadca44700dc8b07a9184a36fa03742a64736f6c634300081a0033")

	runIsthmusDerivationTest := func(gt *testing.T, testCfg *helpers.TestCfg[testCase]) {
		t := actionsHelpers.NewDefaultTesting(gt)
		var deployConfigOverrides = func(dp *genesis.DeployConfig) {}

		if testCfg.Custom == StateRefund {
			testCfg.Allocs = actionsHelpers.DefaultAlloc
			testCfg.Allocs.L2Alloc = make(map[common.Address]types.Account)
			testCfg.Allocs.L2Alloc[testStorageUpdateContractAddress] = types.Account{
				Code:    testStorageUpdateContractCode,
				Nonce:   1,
				Balance: new(big.Int),
			}
		}

		if testCfg.Custom == IsthmusTransitionBlock {
			deployConfigOverrides = func(dp *genesis.DeployConfig) {
				isthmusTimeOffset := hexutil.Uint64(12)
				dp.L2GenesisIsthmusTimeOffset = &isthmusTimeOffset
			}
		}

		env := helpers.NewL2FaultProofEnv(t, testCfg, helpers.NewTestParams(), helpers.NewBatcherCfg(), deployConfigOverrides)

		balanceAt := func(a common.Address) *big.Int {
			t.Helper()
			bal, err := env.Engine.EthClient().BalanceAt(t.Ctx(), a, nil)
			require.NoError(t, err)
			return bal
		}

		t.Logf("L2 Genesis Time: %d, IsthmusTime: %d ", env.Sequencer.RollupCfg.Genesis.L2Time, *env.Sequencer.RollupCfg.IsthmusTime)

		sysCfgContract, err := bindings.NewSystemConfig(env.Sd.RollupCfg.L1SystemConfigAddress, env.Miner.EthClient())
		require.NoError(t, err)

		sysCfgOwner, err := bind.NewKeyedTransactorWithChainID(env.Dp.Secrets.Deployer, env.Sd.RollupCfg.L1ChainID)
		require.NoError(t, err)

		// Update the operator fee parameters
		_, err = sysCfgContract.SetOperatorFeeScalars(sysCfgOwner, testOperatorFeeScalar, testOperatorFeeConstant)
		require.NoError(t, err)

		env.Miner.ActL1StartBlock(12)(t)
		env.Miner.ActL1IncludeTx(env.Dp.Addresses.Deployer)(t)
		env.Miner.ActL1EndBlock(t)

		// sequence L2 blocks, and submit with new batcher
		env.Sequencer.ActL1HeadSignal(t)
		env.Sequencer.ActBuildToL1Head(t)
		env.Batcher.ActSubmitAll(t)
		env.Miner.ActL1StartBlock(12)(t)
		env.Miner.ActL1EndBlock(t)

		env.Sequencer.ActL1HeadSignal(t)

		var aliceInitialBalance *big.Int
		var baseFeeVaultInitialBalance *big.Int
		var l1FeeVaultInitialBalance *big.Int
		var sequencerFeeVaultInitialBalance *big.Int
		var operatorFeeVaultInitialBalance *big.Int

		var receipt *types.Receipt

		switch testCfg.Custom {
		case NormalTx, IsthmusTransitionBlock:

			aliceInitialBalance = balanceAt(env.Alice.Address())
			l1FeeVaultInitialBalance = balanceAt(predeploys.L1FeeVaultAddr)
			baseFeeVaultInitialBalance = balanceAt(predeploys.BaseFeeVaultAddr)
			sequencerFeeVaultInitialBalance = balanceAt(predeploys.SequencerFeeVaultAddr)
			operatorFeeVaultInitialBalance = balanceAt(predeploys.OperatorFeeVaultAddr)

			require.Equal(t, operatorFeeVaultInitialBalance.Sign(), 0)

			// Send an L2 tx
			env.Sequencer.ActL2StartBlock(t)
			env.Alice.L2.ActResetTxOpts(t)
			env.Alice.L2.ActSetTxToAddr(&env.Dp.Addresses.Bob)
			env.Alice.L2.ActMakeTx(t)
			env.Engine.ActL2IncludeTx(env.Alice.Address())(t)
			env.Sequencer.ActL2EndBlock(t)

		case StateRefund:
			env.Sequencer.ActL2StartBlock(t)
			env.Alice.L2.ActResetTxOpts(t)
			env.Alice.L2.ActSetTxToAddr(&testStorageUpdateContractAddress)(t)
			env.Alice.L2.ActSetTxCalldata(common.FromHex("0x60fe47b10000000000000000000000000000000000000000000000000000000000000001"))(t)
			env.Alice.L2.ActMakeTx(t)
			env.Engine.ActL2IncludeTx(env.Alice.Address())(t)
			env.Sequencer.ActL2EndBlock(t)

			aliceInitialBalance = balanceAt(env.Alice.Address())
			l1FeeVaultInitialBalance = balanceAt(predeploys.L1FeeVaultAddr)
			baseFeeVaultInitialBalance = balanceAt(predeploys.BaseFeeVaultAddr)
			sequencerFeeVaultInitialBalance = balanceAt(predeploys.SequencerFeeVaultAddr)
			operatorFeeVaultInitialBalance = balanceAt(predeploys.OperatorFeeVaultAddr)

			env.Sequencer.ActL2StartBlock(t)
			env.Alice.L2.ActResetTxOpts(t)
			env.Alice.L2.ActSetTxToAddr(&testStorageUpdateContractAddress)
			env.Alice.L2.ActSetTxCalldata(common.FromHex("0x60fe47b10000000000000000000000000000000000000000000000000000000000000000"))(t)
			env.Alice.L2.ActMakeTx(t)
			env.Engine.ActL2IncludeTx(env.Alice.Address())(t)
			env.Sequencer.ActL2EndBlock(t)
		case DepositTx:
			// regular L2 tx, in new L2 block
			env.Alice.L2.ActResetTxOpts(t)
			env.Alice.L2.ActSetTxToAddr(&env.Dp.Addresses.Bob)(t)
			env.Alice.L2.ActMakeTx(t)
			env.Sequencer.ActL2StartBlock(t)
			env.Engine.ActL2IncludeTx(env.Alice.Address())(t)
			env.Sequencer.ActL2EndBlock(t)
			env.Alice.L2.ActCheckReceiptStatusOfLastTx(true)(t)

			aliceInitialBalance = balanceAt(env.Alice.Address())
			l1FeeVaultInitialBalance = balanceAt(predeploys.L1FeeVaultAddr)
			baseFeeVaultInitialBalance = balanceAt(predeploys.BaseFeeVaultAddr)
			sequencerFeeVaultInitialBalance = balanceAt(predeploys.SequencerFeeVaultAddr)
			operatorFeeVaultInitialBalance = balanceAt(predeploys.OperatorFeeVaultAddr)

			// regular Deposit, in new L1 block
			env.Alice.L1.ActResetTxOpts(t)
			env.Alice.ActDeposit(t)
			env.Miner.ActL1StartBlock(12)(t)
			env.Miner.ActL1IncludeTx(env.Alice.Address())(t)
			env.Miner.ActL1EndBlock(t)

			// sync sequencer build enough blocks to adopt latest L1 origin
			for env.Sequencer.SyncStatus().UnsafeL2.L1Origin.Number < env.Miner.L1Chain().CurrentBlock().Number.Uint64() {
				env.Sequencer.ActL2StartBlock(t)
				env.Sequencer.ActL2EndBlock(t)
			}

			receipt, err = env.Alice.GetLatestDepositL2Receipt(t)
			require.NoError(t, err)
		}

		l1FeeVaultFinalBalance := balanceAt(predeploys.L1FeeVaultAddr)
		baseFeeVaultFinalBalance := balanceAt(predeploys.BaseFeeVaultAddr)
		sequencerFeeVaultFinalBalance := balanceAt(predeploys.SequencerFeeVaultAddr)
		operatorFeeVaultFinalBalance := balanceAt(predeploys.OperatorFeeVaultAddr)
		aliceFinalBalance := balanceAt(env.Alice.Address())

		if receipt == nil {
			receipt = env.Alice.L2.LastTxReceipt(t)
		}

		if testCfg.Custom == DepositTx || testCfg.Custom == IsthmusTransitionBlock {
			require.Nil(t, receipt.OperatorFeeScalar)
			require.Nil(t, receipt.OperatorFeeConstant)

			// Nothing should has been sent to operator fee vault
			require.Equal(t, operatorFeeVaultInitialBalance, operatorFeeVaultFinalBalance)

		} else {
			// Check that the operator fee was applied
			require.Equal(t, testOperatorFeeScalar, uint32(*receipt.OperatorFeeScalar))
			require.Equal(t, testOperatorFeeConstant, *receipt.OperatorFeeConstant)

			// Check that the operator fee sent to the vault is correct
			require.Equal(t,
				new(big.Int).Add(
					new(big.Int).Div(
						new(big.Int).Mul(
							new(big.Int).SetUint64(receipt.GasUsed),
							new(big.Int).SetUint64(uint64(testOperatorFeeScalar)),
						),
						new(big.Int).SetUint64(1e6),
					),
					new(big.Int).SetUint64(testOperatorFeeConstant),
				),
				new(big.Int).Sub(operatorFeeVaultFinalBalance, operatorFeeVaultInitialBalance),
			)

			require.True(t, aliceFinalBalance.Cmp(aliceInitialBalance) < 0, "Alice's balance should decrease")
		}

		// Check that no Ether has been minted or burned
		finalTotalBalance := new(big.Int).Add(
			aliceFinalBalance,
			new(big.Int).Add(
				new(big.Int).Add(
					new(big.Int).Sub(l1FeeVaultFinalBalance, l1FeeVaultInitialBalance),
					new(big.Int).Sub(sequencerFeeVaultFinalBalance, sequencerFeeVaultInitialBalance),
				),
				new(big.Int).Add(
					new(big.Int).Sub(operatorFeeVaultFinalBalance, operatorFeeVaultInitialBalance),
					new(big.Int).Sub(baseFeeVaultFinalBalance, baseFeeVaultInitialBalance),
				),
			),
		)

		require.Equal(t, aliceInitialBalance, finalTotalBalance)

		l2SafeHead := env.Sequencer.L2Safe()

		env.RunFaultProofProgram(t, l2SafeHead.Number, testCfg.CheckResult, testCfg.InputParams...)
	}

	matrix := helpers.NewMatrix[testCase]()
	defer matrix.Run(gt)

	matrix.AddTestCase(
		"HonestClaim-OperatorFeeConstistency-NormalTx",
		NormalTx,
		helpers.NewForkMatrix(helpers.Isthmus),
		runIsthmusDerivationTest,
		helpers.ExpectNoError(),
	)

	matrix.AddTestCase(
		"HonestClaim-OperatorFeeConstistency-DepositTx",
		DepositTx,
		helpers.NewForkMatrix(helpers.Isthmus),
		runIsthmusDerivationTest,
		helpers.ExpectNoError(),
	)

	matrix.AddTestCase(
		"HonestClaim-OperatorFeeConstistency-StateRefund",
		StateRefund,
		helpers.NewForkMatrix(helpers.Isthmus),
		runIsthmusDerivationTest,
		helpers.ExpectNoError(),
	)

	matrix.AddTestCase(
		"HonestClaim-OperatorFeeConstistency-IsthmusTransitionBlock",
		IsthmusTransitionBlock,
		helpers.NewForkMatrix(helpers.Holocene),
		runIsthmusDerivationTest,
		helpers.ExpectNoError(),
	)

	matrix.AddTestCase(
		"JunkClaim-OperatorFeeConstistency",
		NormalTx,
		helpers.NewForkMatrix(helpers.Isthmus),
		runIsthmusDerivationTest,
		helpers.ExpectError(claim.ErrClaimNotValid),
		helpers.WithL2Claim(common.HexToHash("0xdeadbeef")),
	)
}
