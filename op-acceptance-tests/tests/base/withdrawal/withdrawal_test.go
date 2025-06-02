package withdrawal

import (
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/op-devstack/devtest"
	"github.com/ethereum-optimism/optimism/op-devstack/dsl/contract"
	"github.com/ethereum-optimism/optimism/op-devstack/presets"
	"github.com/ethereum-optimism/optimism/op-devstack/stack/match"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/predeploys"
	"github.com/ethereum-optimism/optimism/op-service/txintent/bindings"
	"github.com/ethereum-optimism/optimism/op-service/txplan"
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

	// Get the L2ToL1MessagePasser address from predeploys
	l2ToL1MessagePasserAddr := predeploys.L2ToL1MessagePasserAddr

	// Get L2 client
	l2Client := sys.L2EL.Escape().EthClient()
	l1Client := sys.L1Network.Escape().L1ELNode(match.FirstL1EL).EthClient()

	// Amount to withdraw
	withdrawalAmount := eth.OneEther
	factory := bindings.NewL2ToL1MessagePasserFactory(bindings.WithClient(l2Client), bindings.WithTo(l2ToL1MessagePasserAddr), bindings.WithTest(t))
	l2ToL1MessagePasser := bindings.NewL2ToL1MessagePasser(factory)

	args := l2ToL1MessagePasser.InitiateWithdrawal(alice.Address(), big.NewInt(500_000), []byte{})
	signedTx := contract.Write(alice, args, txplan.WithValue(withdrawalAmount.ToBig()))

	// Wait for the withdrawal transaction to be included in a block
	require.Eventually(t, func() bool {
		receipt, err := l2Client.TransactionReceipt(t.Ctx(), signedTx.TxHash)
		return err == nil && receipt != nil
	}, 30*time.Second, 500*time.Millisecond, "withdrawal transaction not mined")

	receipt, err := l2Client.TransactionReceipt(t.Ctx(), signedTx.TxHash)
	require.NoError(t, err)
	require.Equal(t, uint64(1), receipt.Status, "withdrawal tx failed")

	// Challenge period is only one block in the test environment
	sys.L2Chain.WaitForBlock()

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
