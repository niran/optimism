package loadtest

import (
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ethereum-optimism/optimism/op-devstack/devtest"
	"github.com/ethereum-optimism/optimism/op-devstack/dsl"
	"github.com/ethereum-optimism/optimism/op-devstack/presets"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/txplan"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

const numInitTxsEnvVar = "NAT_LOADTEST_INITTXS"

func TestMain(m *testing.M) {
	presets.DoMain(m, presets.WithSimpleInterop())
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

type EOA struct {
	inner *dsl.EOA
	nonce atomic.Uint64
}

func NewEOA(eoa *dsl.EOA) *EOA {
	return &EOA{
		inner: eoa,
	}
}

func (eoa *EOA) PlanOnRejection() txplan.Option {
	nonce := eoa.nonce.Load()
	return txplan.Combine(eoa.inner.Plan(), txplan.WithStaticNonce(nonce))
}

func (eoa *EOA) PlanOnInclusion() txplan.Option {
	nonce := eoa.nonce.Add(1) - 1
	return txplan.Combine(eoa.inner.Plan(), txplan.WithStaticNonce(nonce)) // TODO retry submission and inclusion?
}

type EOAPool struct {
	queueMu sync.Mutex
	queue   []*EOA
}

func NewEOAPool(funder *dsl.Funder, size int) *EOAPool {
	if size < 1 {
		panic("expected positive size")
	}
	queue := make([]*EOA, size)
	var wg sync.WaitGroup
	for i := range size {
		wg.Add(1)
		go func() {
			defer wg.Done()
			queue[i] = NewEOA(funder.NewFundedEOA(eth.OneEther))
		}()
	}
	return &EOAPool{
		queue: queue,
	}
}

func (p *EOAPool) Borrow() *EOA {
	p.queueMu.Lock()
	defer p.queueMu.Unlock()
	if len(p.queue) == 0 {
		return nil // Pool exhausted.
	}
	eoa := p.queue[0]
	p.queue[0] = nil
	p.queue = p.queue[1:]
	return eoa
}

func (p *EOAPool) Return(eoa *EOA) {
	p.queueMu.Lock()
	defer p.queueMu.Unlock()
	p.queue = append(p.queue, eoa)
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
