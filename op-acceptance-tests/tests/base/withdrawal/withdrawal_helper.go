package withdrawal

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum-optimism/optimism/op-acceptance-tests/tests/base/withdrawal/utils"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/contracts"
	"github.com/ethereum-optimism/optimism/op-challenger/game/fault/contracts/metrics"
	"github.com/ethereum-optimism/optimism/op-devstack/devtest"
	"github.com/ethereum-optimism/optimism/op-devstack/dsl"
	"github.com/ethereum-optimism/optimism/op-devstack/dsl/contract"
	"github.com/ethereum-optimism/optimism/op-devstack/presets"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/wait"

	"github.com/ethereum-optimism/optimism/op-chain-ops/crossdomain"

	gameTypes "github.com/ethereum-optimism/optimism/op-challenger/game/types"
	"github.com/ethereum-optimism/optimism/op-service/apis"
	"github.com/ethereum-optimism/optimism/op-service/predeploys"
	"github.com/ethereum-optimism/optimism/op-service/sources/batching"
	"github.com/ethereum-optimism/optimism/op-service/txintent/bindings"
	"github.com/ethereum-optimism/optimism/op-service/txplan"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"
)

const SolErrClaimAlreadyResolved = "0xf1a94581"

func SendWithdrawal(t devtest.T, alice *dsl.EOA, applyOpts WithdrawalTxOptsFn) (*types.Receipt, *txplan.PlannedTx) {
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
	receipt, tx := contract.Write2(alice, args, txplan.WithValue(opts.Value))

	require.Equal(t, uint64(1), receipt.Status, "withdrawal tx failed")

	for i, client := range opts.VerifyClients {
		t.Logf("Waiting for tx %v on verification client %d", receipt.TxHash, i)
		receiptVerif, err := client.TransactionReceipt(t.Ctx(), receipt.TxHash)
		require.Nilf(t, err, "Waiting for L2 tx on verification client %d", i)
		require.Equalf(t, receipt, receiptVerif, "Receipts should be the same on sequencer and verification client %d", i)
		// harden
		require.Equal(t, 1, len(receiptVerif.Logs))
	}
	return receipt, tx
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
	rollupConfig := sys.L2Chain.Escape().RollupConfig()
	optimismPortalAddr := rollupConfig.DepositContractAddress
	l1Client := sys.L1EL.Escape().EthClient()
	l2Client := sys.L2EL.Escape().EthClient()

	// Wait for another block to be mined so that the timestamp increases. Otherwise,
	// proveWithdrawalTransaction gas estimation may fail because the current timestamp is the same
	// as the dispute game creation timestamp.
	sys.L1Network.WaitForBlock()
	sys.L2Chain.WaitForBlock()

	// Get the latest header
	portalFactory := bindings.NewOptimismPortal2Factory(bindings.WithClient(l1Client), bindings.WithTo(optimismPortalAddr), bindings.WithTest(t))
	portal := bindings.NewOptimismPortal2(portalFactory)

	params, err := ProveWithdrawalParameters(t, sys.L2Chain, l1Client, l2Client, l2WithdrawalReceipt)
	// b, _ := json.Marshal(params)
	// panic(string(b))

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
		struct {
			Version                  [32]byte
			StateRoot                [32]byte
			MessagePasserStorageRoot [32]byte
			LatestBlockhash          [32]byte
		}{
			Version:                  [32]byte{},
			StateRoot:                params.OutputRootProof.StateRoot,
			MessagePasserStorageRoot: params.OutputRootProof.MessagePasserStorageRoot,
			LatestBlockhash:          params.OutputRootProof.LatestBlockhash,
		},
		params.WithdrawalProof,
	)
	var proveReceipt *types.Receipt
	require.Eventually(t, func() bool {
		proveReceipt, err = contract.WriteWithError(alice.AsEL(sys.L1EL), args)
		if err != nil {
			return false
		}
		return proveReceipt.Status == types.ReceiptStatusSuccessful
	}, 120*time.Second, time.Second, "withdrawal proof failed")

	require.Equal(t, 2, len(proveReceipt.Logs))

	// b, _ := json.Marshal(proveReceipt)
	// panic(string(b))

	return params, proveReceipt
}

func ProveWithdrawalParameters(t devtest.T, l2Chain *dsl.L2Network, l1Client apis.EthClient, l2Client apis.EthClient, l2WithdrawalReceipt *types.Receipt) (utils.ProvenWithdrawalParameters, error) {
	return utils.ProveWithdrawalParameters(t, l2Chain, l1Client, l2Client, l2WithdrawalReceipt)
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
	wdHash, err := wd.Hash()
	require.NoError(t, err)

	game := contract.Read(portal.ProvenWithdrawals(wdHash, alice.Address()))
	// basic sanity check
	require.Greater(t, game.Timestamp, uint64(1700000000))

	fdgf := bindings.NewFaultDisputeGameFactory(bindings.WithClient(l1Client), bindings.WithTo(game.DisputeGameProxy), bindings.WithTest(t))
	fdg := bindings.NewFaultDisputeGame(fdgf)

	gameData := contract.Read(fdg.GameData())
	require.Equal(t, uint32(254), gameData.GameType)

	// 6abfe88c84df81b51d5483025ef7696c1cdb61eaa9ca5b655170683d7e8264a8, 0000000000000000000000000000000000000000000000000000000000000017, 254
	// gametype, root, extradataa(blocknumber)
	// l := fmt.Sprintf(">wow> %s, %s, %d", hex.EncodeToString(gameData.RootClaim[:]), hex.EncodeToString(gameData.ExtraData[:]), gameData.GameType)

	// game.DisputeGameProxy

	require.NotNil(t, game, "withdrawal should be proven")

	gameContract, err := contracts.NewFaultDisputeGameContract(t.Ctx(), metrics.NoopContractMetrics, game.DisputeGameProxy, l1Client.NewMultiCaller(batching.DefaultBatchSize))
	require.NoError(t, err)

	timedCtx, cancel := context.WithTimeout(t.Ctx(), 120*time.Second)
	defer cancel()
	require.NoError(t, wait.For(timedCtx, time.Second, func() (bool, error) {
		// First check if the game is in a resolvable state
		status, err := gameContract.GetStatus(t.Ctx())
		if err != nil {
			return false, err
		}
		if status != gameTypes.GameStatusInProgress {
			return false, fmt.Errorf("game is not in progress: %v", status)
		}

		// Try to resolve the claim
		err = gameContract.CallResolveClaim(t.Ctx(), 0)
		if err != nil {
			t.Logf("Could not resolve dispute game claim: %v", err)
			return false, nil
		}
		return true, nil
	}))

	t.Logf("FinalizeWithdrawal: resolveClaim...")
	tx, err := gameContract.ResolveClaimTx(0)
	require.NoError(t, err)
	resolveClaimReceipt := alice.AsEL(sys.L1EL).Transact(
		alice.AsEL(sys.L1EL).Plan(),
		txplan.WithTo(tx.To),
		txplan.WithValue(tx.Value),
		txplan.WithGasLimit(tx.GasLimit),
		txplan.WithData(tx.TxData),
	)
	// rcr := resolveClaimReceipt.Included.Value()
	// b, _ := json.Marshal(rcr)
	// panic(string(b))

	t.Logf("FinalizeWithdrawal: resolve...")
	tx, err = gameContract.ResolveTx()
	require.NoError(t, err)

	// "resolve"
	resolveReceipt := alice.AsEL(sys.L1EL).Transact(
		alice.AsEL(sys.L1EL).Plan(),
		txplan.WithTo(tx.To),
		txplan.WithValue(tx.Value),
		txplan.WithGasLimit(tx.GasLimit),
		txplan.WithData(tx.TxData),
	)

	receipt := resolveReceipt.Included.Value()
	// bb, _ := json.Marshal(receipt)

	require.Equal(t, 1, len(receipt.Logs))

	if resolveReceipt.Included.Value().Status == types.ReceiptStatusFailed {
		t.Logf("resolve failed (tx: %s)! But game may have resolved already. Checking now...", resolveReceipt.Included.Value().TxHash)
		// it may have failed because someone else front-ran this by calling `resolve()` first.
		status, err := gameContract.GetStatus(t.Ctx())
		require.NoError(t, err)
		require.Equal(t, gameTypes.GameStatusDefenderWon, status, "game must have resolved with defender won")
		t.Logf("resolve was not needed, the game was already resolved")
	}

	// must send to L1
	t.Logf("FinalizeWithdrawal: waiting for successful withdrawal check...")
	// err = ForWithdrawalCheck(t, alice.AsEL(sys.L1EL), wd, optimismPortalAddr, alice.Address())
	// err = ForWithdrawalCheck(t, alice, wd, optimismPortalAddr, alice.Address())
	// require.NoError(t, err)

	// Finalize withdrawal
	t.Logf("FinalizeWithdrawal: finalizing withdrawal...")
	// 0xd9bc01be -> OptimismPortal_ProofNotOldEnough()
	time.Sleep(15 * time.Second) // set maturity and do this
	// 0x332a57f8 -> FinalizeWithdrawalTransaction -> checkWithdrawal-> anchorstatereg::isGameClaimValid -> OptimismPortal_InvalidRootClaim()

	anchorStateRegistryAddr := contract.Read(portal.AnchorStateRegistryAddr())
	asrf := bindings.NewAnchorStateRegistryFactory(bindings.WithClient(l1Client), bindings.WithTo(anchorStateRegistryAddr), bindings.WithTest(t))
	asr := bindings.NewAnchorStateRegistry(asrf)

	a := contract.Read(asr.IsGameClaimValid(game.DisputeGameProxy))
	b := contract.Read(asr.IsGameProper(game.DisputeGameProxy))
	c := contract.Read(asr.IsGameRespected(game.DisputeGameProxy))
	d := contract.Read(asr.IsGameFinalized(game.DisputeGameProxy))
	e := contract.Read(asr.IsGameResolved(game.DisputeGameProxy))

	delaySec := contract.Read(asr.DisputeGameFinalityDelaySeconds())
	//  true true true true true 6(overridden)
	st := fmt.Sprintf("%t %t %t %t %t %s", a, b, c, d, e, delaySec)
	fmt.Println(st)

	contract.Read(portal.CheckWithdrawal(wdHash, alice.Address()))

	// 0xe1ba9227 -> what is this? likely came from ethLockbox
	// it was ETHLockbox_InsufficientBalance() error!
	// fund the ETHLockbox by depositing

	// calldata, _ := portal.FinalizeWithdrawalTransaction(wd.WithdrawalTransaction()).EncodeInputLambda()
	// panic

	panic(contract.Read(portal.ETHLockboxAddr()))

	finalizeWithdrawalReceipt := contract.Write(alice.AsEL(sys.L1EL), portal.FinalizeWithdrawalTransaction(wd.WithdrawalTransaction()))

	require.Equal(t, 1, len(finalizeWithdrawalReceipt.Logs))
	// Ensure that our withdrawal was finalized successfully
	require.Equal(t, types.ReceiptStatusSuccessful, finalizeWithdrawalReceipt.Status)
	return finalizeWithdrawalReceipt, resolveClaimReceipt.Included.Value(), resolveReceipt.Included.Value()
}
