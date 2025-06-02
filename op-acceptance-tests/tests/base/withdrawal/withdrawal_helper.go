package withdrawal

import (
	"math/big"

	"github.com/ethereum-optimism/optimism/op-acceptance-tests/tests/base/withdrawal/utils"
	"github.com/ethereum-optimism/optimism/op-devstack/devtest"
	"github.com/ethereum-optimism/optimism/op-devstack/dsl"
	"github.com/ethereum-optimism/optimism/op-devstack/dsl/contract"
	"github.com/ethereum-optimism/optimism/op-devstack/presets"

	"github.com/ethereum-optimism/optimism/op-chain-ops/crossdomain"

	"github.com/ethereum-optimism/optimism/op-service/apis"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/predeploys"
	"github.com/ethereum-optimism/optimism/op-service/txintent/bindings"
	"github.com/ethereum-optimism/optimism/op-service/txplan"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"
)

const SolErrClaimAlreadyResolved = "0xf1a94581"

type ClientProvider interface {
	NodeClient(name string) apis.EthClient
}

func SendWithdrawal(t devtest.T, alice *dsl.EOA, applyOpts WithdrawalTxOptsFn) *types.Receipt {
	opts := defaultWithdrawalTxOpts()
	applyOpts(opts)

	l2Client := alice.EthClient()

	// Create L2ToL1MessagePasser binding
	factory := bindings.NewL2ToL1MessagePasserFactory(
		bindings.WithClient(l2Client),
		bindings.WithTo(predeploys.L2ToL1MessagePasserAddr),
		bindings.WithTest(t),
	)
	l2withdrawer := bindings.NewL2ToL1MessagePasser(factory)

	// Initiate Withdrawal

	// Create the withdrawal transaction
	args := l2withdrawer.InitiateWithdrawal(alice.Address(), big.NewInt(int64(opts.Gas)), opts.Data)
	tx := contract.Write(alice, args, txplan.WithValue(opts.Value))

	require.Equal(t, uint64(1), tx.Status, "withdrawal tx failed")

	for i, client := range opts.VerifyClients {
		t.Logf("Waiting for tx %v on verification client %d", tx.TxHash, i)
		receiptVerif, err := client.TransactionReceipt(t.Ctx(), tx.TxHash)
		require.Nilf(t, err, "Waiting for L2 tx on verification client %d", i)
		require.Equalf(t, tx, receiptVerif, "Receipts should be the same on sequencer and verification client %d", i)
	}
	return tx
}

type WithdrawalTxOptsFn func(opts *WithdrawalTxOpts)

type WithdrawalTxOpts struct {
	ToAddr         *common.Address
	Nonce          uint64
	Value          *big.Int
	Gas            uint64
	Data           []byte
	ExpectedStatus uint64
	VerifyClients  []apis.EthClient
}

// VerifyOnClients adds additional l2 clients that should sync the block the tx is included in
// Checks that the receipt received from these clients is equal to the receipt received from the sequencer
func (o *WithdrawalTxOpts) VerifyOnClients(clients ...apis.EthClient) {
	o.VerifyClients = append(o.VerifyClients, clients...)
}

func defaultWithdrawalTxOpts() *WithdrawalTxOpts {
	return &WithdrawalTxOpts{
		ToAddr:         nil,
		Nonce:          0,
		Value:          common.Big0,
		Gas:            21_000,
		Data:           nil,
		ExpectedStatus: types.ReceiptStatusSuccessful,
	}
}

func ProveAndFinalizeWithdrawal(
	t devtest.T,
	sys *presets.Minimal,
	alice *dsl.EOA,
	l2WithdrawalReceipt *types.Receipt,
	usesProofs bool,
) (*types.Receipt, *types.Receipt, *types.Receipt, *types.Receipt) {
	params, proveReceipt := ProveWithdrawal(t, sys, alice, l2WithdrawalReceipt, usesProofs)
	finalizeReceipt, resolveClaimReceipt, resolveReceipt := FinalizeWithdrawal(t, sys, alice, proveReceipt, params, usesProofs)
	return proveReceipt, finalizeReceipt, resolveClaimReceipt, resolveReceipt
}

func ProveWithdrawal(t devtest.T, sys *presets.Minimal, alice *dsl.EOA, l2WithdrawalReceipt *types.Receipt, usesProofs bool) (utils.ProvenWithdrawalParameters, *types.Receipt) {
	l2ToL1MessagePasserAddr := predeploys.L2ToL1MessagePasserAddr
	rollupConfig := sys.L2Chain.Escape().RollupConfig()
	optimismPortalAddr := rollupConfig.DepositContractAddress
	l1Client := sys.L1EL.Escape().EthClient()
	l2Client := sys.L2EL.Escape().EthClient()
	var blockNumber uint64
	var err error
	if usesProofs {
		blockNumber, err = ForGamePublished(t, sys.L2Chain, l1Client, optimismPortalAddr, l2ToL1MessagePasserAddr, l2WithdrawalReceipt.BlockNumber)
		require.NoError(t, err)
	} else {
		block, err := l2Client.BlockRefByLabel(t.Ctx(), "finalized")
		require.NoError(t, err)
		blockNumber = block.Number
	}

	// Wait for another block to be mined so that the timestamp increases. Otherwise,
	// proveWithdrawalTransaction gas estimation may fail because the current timestamp is the same
	// as the dispute game creation timestamp.
	sys.L1Network.WaitForBlock()

	// Get the latest header
	header, err := l1Client.BlockRefByNumber(t.Ctx(), blockNumber)
	require.NoError(t, err)

	portalFactory := bindings.NewOptimismPortal2Factory(bindings.WithClient(l1Client), bindings.WithTo(optimismPortalAddr), bindings.WithTest(t))
	portal := bindings.NewOptimismPortal2(portalFactory)

	latestGame, err := utils.FindLatestGame(t, sys.L2Chain, l1Client)
	require.NoError(t, err)

	params, err := ProveWithdrawalParameters(t, sys.L2Chain, l1Client, l2Client, l2WithdrawalReceipt, &header, latestGame.Index, false)
	require.NoError(t, err)

	// Prove withdrawal
	args := portal.ProveWithdrawalTransaction(
		struct {
			Nonce    *big.Int
			Sender   common.Address
			Target   common.Address
			Value    *big.Int
			GasLimit *big.Int
			Data     []byte
		}{
			Nonce:    params.Nonce,
			Sender:   params.Sender,
			Target:   params.Target,
			Value:    params.Value,
			GasLimit: params.GasLimit,
			Data:     params.Data,
		},
		params.L2OutputIndex,
		params.OutputRootProof,
		params.WithdrawalProof,
	)
	proveReceipt := contract.Write(alice.AsEL(sys.L1EL), args)
	require.NoError(t, err, "prove withdrawal")
	require.Equal(t, types.ReceiptStatusSuccessful, proveReceipt.Status)
	return params, proveReceipt
}

func ProveWithdrawalParameters(t devtest.T, l2Chain *dsl.L2Network, l1Client apis.EthClient, l2Client apis.EthClient, l2WithdrawalReceipt *types.Receipt, header *eth.BlockRef, l2OutputIndex *big.Int, useFaultProofs bool) (utils.ProvenWithdrawalParameters, error) {
	if useFaultProofs {
		return utils.ProveWithdrawalParametersFaultProofs(t, l2Chain, l1Client, l2Client, l2WithdrawalReceipt)
	} else {
		return utils.ProveWithdrawalParameters(t, l2Client, l2WithdrawalReceipt, header, l2OutputIndex)
	}
}

func FinalizeWithdrawal(t devtest.T, sys *presets.Minimal, alice *dsl.EOA, l2WithdrawalReceipt *types.Receipt, params utils.ProvenWithdrawalParameters, usesProofs bool) (*types.Receipt, *types.Receipt, *types.Receipt) {
	wd := crossdomain.Withdrawal{
		Nonce:    params.Nonce,
		Sender:   &params.Sender,
		Target:   &params.Target,
		Value:    params.Value,
		GasLimit: params.GasLimit,
		Data:     params.Data,
	}

	l1Client := sys.L1EL.Escape().EthClient()
	rollupConfig := sys.L2Chain.Escape().RollupConfig()
	optimismPortalAddr := rollupConfig.DepositContractAddress
	portalFactory := bindings.NewOptimismPortal2Factory(bindings.WithClient(l1Client), bindings.WithTo(optimismPortalAddr), bindings.WithTest(t))
	portal := bindings.NewOptimismPortal2(portalFactory)
	var resolveClaimReceipt *types.Receipt
	var resolveReceipt *types.Receipt
	if usesProofs {
		/*
			wdHash, err := wd.Hash()
			require.NoError(t, err)

			game := contract.Read(portal.ProvenWithdrawals(wdHash, alice.Address()))
			require.NotNil(t, game, "withdrawal should be proven")

			gameContract, err := contracts.NewFaultDisputeGameContract(context.Background(), metrics.NoopContractMetrics, game.DisputeGameProxy, batching.NewMultiCaller(l1Client, batching.DefaultBatchSize))
			require.NoError(t, err)

			timedCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			require.NoError(t, wait.For(timedCtx, time.Second, func() (bool, error) {
				err := gameContract.CallResolveClaim(context.Background(), 0)
				if err != nil {
					t.Logf("Could not resolve dispute game claim: %v", err)
				}
				return err == nil, nil
			}))

			t.Logf("FinalizeWithdrawal: resolveClaim...")
			tx, err := gameContract.ResolveClaimTx(0)
			require.NoError(t, err, "create resolveClaim tx")
			_, resolveClaimReceipt, err = transactions.SendTx(ctx, l1Client, tx, privKey)
			var rsErr *wait.ReceiptStatusError
			if errors.As(err, &rsErr) && rsErr.TxTrace.Output.String() == SolErrClaimAlreadyResolved {
				t.Logf("resolveClaim failed (tx: %s) because claim got already resolved", resolveClaimReceipt.TxHash)
			} else {
				require.NoError(t, err)
			}

			t.Logf("FinalizeWithdrawal: resolve...")
			tx, err = gameContract.ResolveTx()
			require.NoError(t, err, "create resolve tx")
			_, resolveReceipt = transactions.RequireSendTx(t, ctx, l1Client, tx, privKey, transactions.WithReceiptStatusIgnore())
			if resolveReceipt.Status == types.ReceiptStatusFailed {
				t.Logf("resolve failed (tx: %s)! But game may have resolved already. Checking now...", resolveReceipt.TxHash)
				// it may have failed because someone else front-ran this by calling `resolve()` first.
				status, err := gameContract.GetStatus(ctx)
				require.NoError(t, err)
				require.Equal(t, gameTypes.GameStatusDefenderWon, status, "game must have resolved with defender won")
				t.Logf("resolve was not needed, the game was already resolved")
			}

			t.Logf("FinalizeWithdrawal: waiting for successful withdrawal check...")
			err = wait.ForWithdrawalCheck(ctx, l1Client, wd, optimismPortalAddr, alice.Address())
			require.NoError(t, err)*/
	}

	// Finalize withdrawal
	t.Logf("FinalizeWithdrawal: finalizing withdrawal...")
	tx := contract.Write(alice, portal.FinalizeWithdrawalTransaction(wd.WithdrawalTransaction()))

	// Ensure that our withdrawal was finalized successfully
	require.Equal(t, types.ReceiptStatusSuccessful, tx.Status)
	return tx, resolveClaimReceipt, resolveReceipt
}
