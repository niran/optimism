package loadtest

import (
	"math/big"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/ethereum-optimism/optimism/devnet-sdk/contracts/bindings"
	"github.com/ethereum-optimism/optimism/op-devstack/devtest"
	"github.com/ethereum-optimism/optimism/op-devstack/dsl"
	"github.com/ethereum-optimism/optimism/op-devstack/presets"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	opbindings "github.com/ethereum-optimism/optimism/op-service/txintent/bindings"
	"github.com/ethereum-optimism/optimism/op-service/txintent/contractio"
	"github.com/ethereum-optimism/optimism/op-service/txplan"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
)

const numInitTxsEnvVar = "NAT_LOADTEST_INITTXS"

func TestMain(m *testing.M) {
	presets.DoMain(m, presets.WithSimpleInterop())
}

const num = 1_650

func TestDeployAirdrop(gt *testing.T) {
	t := devtest.SerialT(gt)
	sys := presets.NewSimpleInterop(t)

	master := sys.FunderA.NewFundedEOA(eth.MillionEther.Mul(2))

	tx := txplan.NewPlannedTx(master.Plan(), txplan.WithData(common.FromHex(bindings.FaucetBin)))
	receipt, err := tx.Included.Eval(t.Ctx())
	t.Require().NoError(err)
	_, err = tx.Success.Eval(t.Ctx())
	t.Require().NoError(err)
	faucetAddress := receipt.ContractAddress

	faucet := opbindings.NewFaucet(opbindings.NewFaucetFactory(
		opbindings.WithClient(sys.L2ELA.Escape().EthClient()),
		opbindings.WithTest(t),
		opbindings.WithTo(faucetAddress),
	))

	eoas := make([]*dsl.EOA, 0, num)
	addrs := make([]common.Address, 0, num)
	for range num {
		eoa := sys.Wallet.NewEOA(sys.L2ELA)
		eoas = append(eoas, eoa)
		addrs = append(addrs, eoa.Address())
	}

	receipt, err = contractio.Write(faucet.Fund(addrs, eth.OneEther.ToBig()), t.Ctx(), master.Plan(), txplan.WithValue(eth.MillionEther.ToBig()))
	t.Require().NoError(err)
	t.Require().Equal(ethtypes.ReceiptStatusSuccessful, receipt.Status)

	for _, eoa := range eoas {
		t.Require().Equal(eth.OneEther, eoa.GetBalance())
	}

	t.Logf("master balance: %s", master.GetBalance().EtherString())

	// Ensure there is no eth left in the contract.
	balance, err := sys.L2ELA.Escape().EthClient().BalanceAt(t.Ctx(), faucetAddress, nil)
	t.Require().NoError(err)
	t.Logf("contract balance: %s", eth.WeiBig(balance).EtherString())
	t.Require().Zero(balance.Cmp(new(big.Int)))
}

type L2 struct {
	EL     *dsl.L2ELNode
	Funder *dsl.Funder
}

func TestLoad(gt *testing.T) {
	if testing.Short() {
		gt.Skip("skipping load test in short mode")
	}
	t := devtest.SerialT(gt)
	sys := presets.NewSimpleInterop(t)

	numInitTxs := uint64(1)
	if numInitTxsStr, ok := os.LookupEnv(numInitTxsEnvVar); ok {
		var err error
		numInitTxs, err = strconv.ParseUint(numInitTxsStr, 10, 64)
		t.Require().NoError(err)
	}

	l2ELA := sys.L2ChainA.PublicRPC()
	L2A := &L2{
		EL:     l2ELA,
		Funder: dsl.NewFunder(sys.Wallet, sys.FaucetA, l2ELA),
	}
	l2ELB := sys.L2ChainB.PublicRPC()
	L2B := &L2{
		EL:     l2ELB,
		Funder: dsl.NewFunder(sys.Wallet, sys.FaucetB, l2ELB),
	}

	var wg sync.WaitGroup
	defer wg.Wait()
	wg.Add(1)
	go func() {
		defer wg.Done()
		SpamInteropTxs(t, numInitTxs, L2A, L2B, sys.Supervisor)
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		SpamInteropTxs(t, numInitTxs, L2B, L2A, sys.Supervisor)
	}()
}

func fundEOAs(num uint64, funder *dsl.Funder) []*dsl.EOA {
	eoas := make([]*dsl.EOA, 0, num)
	for range num {
		eoas = append(eoas, funder.NewFundedEOA(eth.OneEther))
	}
	return eoas
}

func SpamInteropTxs(t devtest.T, numInitTxs uint64, source *L2, dest *L2, supervisor *dsl.Supervisor) {
	var wg sync.WaitGroup
	defer wg.Wait()
	msgsCh := make(chan []types.Message, 100)
	defer close(msgsCh)

	// Mempool implementations may limit the number of concurrent transactions per account.
	// We spam transactions from multiple EOAs to mitigate the possibility of mempool
	// implementations being a limiting factor.

	// Spam executing messages.
	wg.Add(1)
	go func() {
		defer wg.Done()
		relayers := []Relayer{
			NewValidRelayer(dest.EL, supervisor),
			NewDelayedRelayer(NewValidRelayer(dest.EL, supervisor), &wg, time.Minute),
			NewInvalidRelayer(dest.EL, makeInvalidChainID),
			NewInvalidRelayer(dest.EL, makeInvalidBlockNumber),
			NewInvalidRelayer(dest.EL, makeInvalidLogIndex),
			NewInvalidRelayer(dest.EL, makeInvalidOrigin),
			NewInvalidRelayer(dest.EL, makeInvalidPayloadHash),
			NewInvalidRelayer(dest.EL, makeInvalidTimestamp),
		}
		eoas := fundEOAs(uint64(len(relayers))*numInitTxs, dest.Funder) // Fund EOAs before spamming relay transactions.
		var eoaIdx int
		for msgs := range msgsCh {
			for _, relayer := range relayers {
				plan := eoas[eoaIdx].Plan()
				eoaIdx++
				wg.Add(1)
				go func() {
					defer wg.Done()
					relayer.Relay(t, msgs, plan)
				}()
			}
		}
	}()

	// Spam initiating messages.
	eventLogger := source.Funder.NewFundedEOA(eth.OneEther).DeployEventLogger()
	initiators := []Initiator{
		NewManyMsgsInitiator(source.EL, eventLogger),
		NewLargeMsgInitiator(source.EL, eventLogger),
	}
	eoas := fundEOAs(numInitTxs, source.Funder) // Fund EOAs before spamming initiating transactions.
	for i := range numInitTxs {
		msgsCh <- initiators[i%uint64(len(initiators))].Initiate(t, eoas[i].Plan())
	}
}
