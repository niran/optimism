package withdrawal

import (
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/state"
	"github.com/ethereum-optimism/optimism/op-devstack/devtest"
	"github.com/ethereum-optimism/optimism/op-devstack/presets"
	"github.com/ethereum-optimism/optimism/op-e2e/system/helpers"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	supervisorTypes "github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

// bypass DeployOPChain -> assertValidPermissionedDisputeGame()
var defaultPrestate = common.HexToHash("0x038512e02c4c3f7bdaec27d00edf55b7155e0905301e1a88083e4e0a6764d54c")

func TestMain(m *testing.M) {
	//	presets.DoMain(m, presets.WithMinimal(), presets.WithFinalizationPeriodSeconds(2))
	presets.DoMain(m, presets.WithMinimal(), presets.WithFinalizationPeriodSeconds(2),
		presets.WithProofMaturity(12),
		presets.WithFaultGameAbsolutePrestate(defaultPrestate),
		presets.WithDisputeGameFinalityDelaySeconds(6),
		presets.WithAdditonalDisputeGames([]state.AdditionalDisputeGame{
			{
				ChainProofParams: state.ChainProofParams{
					// Fast game
					DisputeGameType:         254,
					DisputeAbsolutePrestate: defaultPrestate,
					DisputeMaxGameDepth:     14 + 3 + 1,
					DisputeSplitDepth:       14,
					DisputeClockExtension:   0,
					DisputeMaxClockDuration: 0,
				},
				VMType:                       state.VMTypeAlphabet,
				UseCustomOracle:              true,
				OracleMinProposalSize:        10000,
				OracleChallengePeriodSeconds: 0,
				MakeRespected:                true, // this is important
			},
		}),
	)
}

func TestL2ToL1Withdrawal(gt *testing.T) {
	// Create a test environment using op-devstack with a shorter finalization period
	t := devtest.SerialT(gt)
	sys := presets.NewMinimal(t)

	// Wait for initial blocks
	sys.L1Network.WaitForBlock()
	sys.L2Chain.WaitForBlock()

	// Fund Alice on L2
	fundingAmount := eth.ThousandEther
	alice := sys.Funder.NewFundedEOA(fundingAmount)
	alice.VerifyBalanceExact(fundingAmount)
	// fund Alice at L1: 10 ETH
	sys.FunderL1.FundAtLeast(alice.AsEL(sys.L1EL), eth.TenEther)

	// Alice L1: 10 ETH, L2: 1000 ETH

	// FIXME
	// // first deposit 1 ETH : L1 -> L2. This fills L1 ETHlockbox
	// mintAmount := eth.OneEther
	// // this will fix everything
	// or we may call portal2::migrateLiquidity to fill in lockETH

	// Start L2 balance for withdrawal
	l1Client := sys.L1EL.Escape().EthClient()
	startBalanceBeforeWithdrawal, err := sys.L2EL.Escape().EthClient().BalanceAt(t.Ctx(), alice.Address(), nil)
	require.True(t, startBalanceBeforeWithdrawal.Cmp(fundingAmount.ToBig()) == 0)

	// Alice withdraws 1 ETH from L2 to L1
	// spends some ETH at L2 for withdrawal transactions

	// withdrawalAmount := eth.OneEther

	//////// FIXE ME //////////
	withdrawalAmount := eth.ZeroWei
	//////// FIXE ME //////////

	t.Logf("WithdrawalsTest: sending L2 withdrawal for %v...", withdrawalAmount.String())
	receipt, tx := SendWithdrawal(t, alice, func(opts *WithdrawalTxOpts) {
		opts.Gas = 500_000
		opts.Data = []byte{}
		opts.Value = withdrawalAmount.ToBig()
	})

	t.Require().Eventually(func() bool {
		head := sys.L2CL.HeadBlockRef(supervisorTypes.LocalUnsafe)
		return head.L1Origin.Number >= receipt.BlockNumber.Uint64()
	}, time.Second*60, time.Second, "awaiting withdrawal to be processed by L2")

	sys.L2Chain.WaitForBlock()
	sys.L1Network.WaitForBlock()

	var endBalanceAfterWithdrawal *big.Int
	t.Log("WithdrawalsTest: waiting for L2 balance change...")
	t.Require().Eventually(func() bool {
		endBalanceAfterWithdrawal, _ = sys.L2EL.Escape().EthClient().BalanceAt(t.Ctx(), alice.Address(), nil)
		return endBalanceAfterWithdrawal.Cmp(startBalanceBeforeWithdrawal) != 0
	}, time.Second*60, time.Second)
	require.NoError(t, err)

	header, err := sys.L2EL.Escape().L2EthClient().InfoByNumber(t.Ctx(), receipt.BlockNumber.Uint64())
	require.NoError(t, err)

	{
		// Take fee into account
		diff := new(big.Int).Sub(startBalanceBeforeWithdrawal, endBalanceAfterWithdrawal)
		fees := helpers.CalcGasFees(receipt.GasUsed, tx.GasTipCap.Value(), tx.GasFeeCap.Value(), header.BaseFee())
		fees = fees.Add(fees, receipt.L1Fee)
		diff = diff.Sub(diff, fees)

		require.True(t, withdrawalAmount.ToBig().Cmp(diff) == 0)
	}

	// Get the L1 balance before finalization
	startBalanceBeforeFinalize, err := l1Client.BalanceAt(t.Ctx(), alice.Address(), nil)
	require.NoError(t, err)
	// still 10 ETH at L1
	require.True(t, startBalanceBeforeFinalize.Cmp(eth.TenEther.ToBig()) == 0, startBalanceBeforeFinalize.String())

	proveReceipt, finalizeReceipt, resolveClaimReceipt, resolveReceipt := ProveAndFinalizeWithdrawal(t, sys, alice, receipt, false)

	// aaa, err := l1Client.BalanceAt(t.Ctx(), alice.Address(), nil)
	// require.NoError(t, err)
	// panic(aaa.String())

	// Calculate the expected balance change
	proveFee := new(big.Int).Mul(new(big.Int).SetUint64(proveReceipt.GasUsed), proveReceipt.EffectiveGasPrice)
	finalizeFee := new(big.Int).Mul(new(big.Int).SetUint64(finalizeReceipt.GasUsed), finalizeReceipt.EffectiveGasPrice)
	fees := new(big.Int).Add(proveFee, finalizeFee)
	if resolveClaimReceipt != nil && resolveReceipt != nil {
		resolveClaimFee := new(big.Int).Mul(new(big.Int).SetUint64(resolveClaimReceipt.GasUsed), resolveClaimReceipt.EffectiveGasPrice)
		resolveFee := new(big.Int).Mul(new(big.Int).SetUint64(resolveReceipt.GasUsed), resolveReceipt.EffectiveGasPrice)
		fees = new(big.Int).Add(fees, resolveClaimFee)
		fees = new(big.Int).Add(fees, resolveFee)
	}
	expectedAmount := new(big.Int).Sub(withdrawalAmount.ToBig(), fees)

	var endBalanceAfterFinalize *big.Int
	require.Eventually(t, func() bool {
		endBalanceAfterFinalize, err = l1Client.BalanceAt(t.Ctx(), alice.Address(), nil)
		if err != nil {
			return false
		}
		diff := new(big.Int).Sub(endBalanceAfterFinalize, startBalanceBeforeFinalize)
		return diff.Cmp(expectedAmount) == 0
	}, time.Second*60, time.Second, "awaiting balance to be changed")
}
