package loadtest2

import (
	"context"
	"math/rand"
	"sync"
	"sync/atomic"

	"github.com/ethereum-optimism/optimism/devnet-sdk/contracts/constants"
	"github.com/ethereum-optimism/optimism/op-acceptance-tests/tests/interop"
	"github.com/ethereum-optimism/optimism/op-devstack/devtest"
	"github.com/ethereum-optimism/optimism/op-devstack/dsl"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/txintent"
	"github.com/ethereum-optimism/optimism/op-service/txplan"
	"github.com/ethereum/go-ethereum/common"
)

type ManyMsgsInitiator struct {
	t           devtest.T
	el          *dsl.L2ELNode
	eoa         *dsl.EOA
	eventLogger common.Address
	nonce       atomic.Uint64
	rngMu       sync.Mutex
	rng         *rand.Rand
	msgSink     *Bus
}

func NewManyMsgsInitiator(t devtest.T, bus *Bus, funder *dsl.Funder, el *dsl.L2ELNode, eventLogger common.Address) *ManyMsgsInitiator {
	return &ManyMsgsInitiator{
		msgSink:     bus,
		t:           t,
		eoa:         funder.NewFundedEOA(eth.MillionEther),
		el:          el,
		eventLogger: eventLogger,
		rng:         rand.New(rand.NewSource(1234)),
	}
}

func (in *ManyMsgsInitiator) randomInitTriggers() []txintent.Call {
	const numMsgs = 275 // About the max number of msgs we can create before hitting tx size limits.
	initCalls := make([]txintent.Call, 0, numMsgs)
	in.rngMu.Lock()
	defer in.rngMu.Unlock()
	for range numMsgs {
		initCalls = append(initCalls, interop.RandomInitTrigger(in.rng, in.eventLogger, in.rng.Intn(5), in.rng.Intn(10)))
	}
	return initCalls
}

func (in *ManyMsgsInitiator) SendTx(ctx context.Context) bool {
	initMsgsTx := txintent.NewIntent[txintent.Call, *txintent.InteropOutput](in.eoa.Plan(), txplan.WithStaticNonce(in.nonce.Add(1)-1))
	initMsgsTx.Content.Set(&txintent.MultiTrigger{
		Emitter: constants.MultiCall3,
		Calls:   in.randomInitTriggers(),
	})
	if _, err := initMsgsTx.PlannedTx.Success.Eval(ctx); err != nil {
		return false
	}
	out, err := initMsgsTx.Result.Eval(ctx)
	if err != nil {
		return false
	}
	in.msgSink.Publish(out.Entries)
	return true
}
