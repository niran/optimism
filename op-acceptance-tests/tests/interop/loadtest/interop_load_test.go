package loadtest

import (
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/ethereum-optimism/optimism/op-devstack/devtest"
	"github.com/ethereum-optimism/optimism/op-devstack/dsl"
	"github.com/ethereum-optimism/optimism/op-devstack/presets"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
	"github.com/ethereum/go-ethereum/common"
)

const numInitTxsEnvVar = "NAT_LOADTEST_INITTXS"

func TestMain(m *testing.M) {
	presets.DoMain(m, presets.WithSimpleInterop())
}

type L2 struct {
	EL *dsl.L2ELNode
	// Mempool implementations may limit the number of concurrent transactions per account.
	// We spam transactions from multiple EOAs to mitigate the possibility of mempool
	// implementations being a limiting factor.
	EOAPool     *EOAPool
	eventLogger common.Address
}

func NewL2(t devtest.T, numEOAs int, chain *dsl.L2Network, wallet *dsl.HDWallet, faucet *dsl.Faucet) *L2 {
	l2EL := chain.PublicRPC()
	eoa := dsl.NewFunder(wallet, faucet, l2EL).NewFundedEOA(eth.MillionEther)
	faucetAddress := eoa.DeployFaucet()
	eventLoggerAddress := eoa.DeployEventLogger()
	return &L2{
		EL:          l2EL,
		EOAPool:     NewEOAPool(t, numEOAs, eth.OneEther, eoa, wallet, faucetAddress, l2EL),
		eventLogger: eventLoggerAddress,
	}
}

func TestLoad(gt *testing.T) {
	if testing.Short() {
		gt.Skip("skipping load test in short mode")
	}
	t := devtest.SerialT(gt)
	sys := presets.NewSimpleInterop(t)

	numInitTxs := 1
	if numInitTxsStr, ok := os.LookupEnv(numInitTxsEnvVar); ok {
		numInitTxs64, err := strconv.ParseInt(numInitTxsStr, 10, 0)
		t.Require().NoError(err)
		t.Require().Positive(numInitTxs64)
		numInitTxs = int(numInitTxs64)
	}

	numEOAs := numInitTxs * 10
	if numEOAs > 10_000 {
		numEOAs = 10_000
	}
	l2A := NewL2(t, numEOAs, sys.L2ChainA, sys.Wallet, sys.FaucetA)
	l2B := NewL2(t, numEOAs, sys.L2ChainB, sys.Wallet, sys.FaucetB)

	var wg sync.WaitGroup
	defer wg.Wait()
	wg.Add(1)
	go func() {
		defer wg.Done()
		l2A.SpamInteropTxs(t, numInitTxs, l2B, sys.Supervisor)
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		l2B.SpamInteropTxs(t, numInitTxs, l2A, sys.Supervisor)
	}()
}

func (source *L2) SpamInteropTxs(t devtest.T, numInitTxs int, dest *L2, supervisor *dsl.Supervisor) {
	var wg sync.WaitGroup
	defer wg.Wait()
	msgsCh := make(chan []types.Message, 100)
	defer close(msgsCh)

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
		for msgs := range msgsCh {
			for _, relayer := range relayers {
				wg.Add(1)
				go func() {
					defer wg.Done()
					// It may be better to just add the EOAPool to the relayers.
					// That way the relayers can return the eoa to the pool at the earliest opportunity.
					eoa := dest.EOAPool.Borrow()
					defer dest.EOAPool.Return(eoa)
					relayer.Relay(t, msgs, eoa)
				}()
			}
		}
	}()

	// Spam initiating messages.
	initiators := []Initiator{
		NewManyMsgsInitiator(source.EL, source.eventLogger),
		NewLargeMsgInitiator(source.EL, source.eventLogger),
	}
	for i := range numInitTxs {
		eoa := source.EOAPool.Borrow()
		msgsCh <- initiators[i%len(initiators)].Initiate(t, eoa)
		source.EOAPool.Return(eoa)
	}
}
