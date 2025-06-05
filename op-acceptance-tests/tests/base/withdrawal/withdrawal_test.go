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
	"github.com/ethereum-optimism/optimism/op-service/eth"
	supervisorTypes "github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

func TestMain(m *testing.M) {
	presets.DoMain(m, presets.WithMinimal(), presets.WithFinalizationPeriodSeconds(2), presets.WithAdditonalDisputeGames([]state.AdditionalDisputeGame{
		{
			ChainProofParams: state.ChainProofParams{
				// Fast game
				DisputeGameType:         254,
				DisputeAbsolutePrestate: common.HexToHash("0x03c7ae758795765c6664a5d39bf63841c71ff191e9189522bad8ebff5d4eca98"),
				DisputeMaxGameDepth:     14 + 3 + 1,
				DisputeSplitDepth:       14,
				DisputeClockExtension:   0,
				DisputeMaxClockDuration: 0,
			},
			VMType:                       state.VMTypeAlphabet,
			UseCustomOracle:              true,
			OracleMinProposalSize:        10000,
			OracleChallengePeriodSeconds: 0,
			MakeRespected:                true,
		},
	}))
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
	sys.FunderL1.FundAtLeast(alice.AsEL(sys.L1EL), eth.OneHundredthEther)

	l1Client := sys.L1EL.Escape().EthClient()

	withdrawalAmount := eth.OneEther
	receipt := SendWithdrawal(t, alice, func(opts *WithdrawalTxOpts) {
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

	// Get the L1 balance before finalization
	startBalanceBeforeFinalize, err := l1Client.BalanceAt(t.Ctx(), alice.Address(), nil)
	require.NoError(t, err)

	proveReceipt, finalizeReceipt, resolveClaimReceipt, resolveReceipt := ProveAndFinalizeWithdrawal(t, sys, alice, receipt, false)

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
