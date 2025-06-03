package withdrawal

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/op-devstack/devtest"
	"github.com/ethereum-optimism/optimism/op-devstack/presets"
	"github.com/ethereum-optimism/optimism/op-service/eth"
)

func TestMain(m *testing.M) {
	presets.DoMain(m, presets.WithMinimal(), presets.WithFinalizationPeriodSeconds(2))
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

	l1Client := sys.L1EL.Escape().EthClient()

	withdrawalAmount := eth.OneEther
	receipt := SendWithdrawal(t, alice, func(opts *WithdrawalTxOpts) {
		opts.Gas = 500_000
		opts.Data = []byte{}
		opts.Value = withdrawalAmount.ToBig()
	})

	// Get the L1 balance before finalization
	startBalanceBeforeFinalize, err := l1Client.BalanceAt(t.Ctx(), alice.Address(), nil)
	require.NoError(t, err)

	proveReceipt, finalizeReceipt, resolveClaimReceipt, resolveReceipt := ProveAndFinalizeWithdrawal(t, sys, alice, receipt, false)

	// Verify the L1 balance after finalization
	endBalanceAfterFinalize, err := l1Client.BalanceAt(t.Ctx(), alice.Address(), nil)
	require.NoError(t, err)

	// Calculate the expected balance change
	diff := new(big.Int).Sub(endBalanceAfterFinalize, startBalanceBeforeFinalize)
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

	require.Equal(t, expectedAmount, diff, "L1 balance change does not match expected withdrawal amount")
}
