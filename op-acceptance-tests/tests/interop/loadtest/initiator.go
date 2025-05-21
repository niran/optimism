package loadtest

import (
	"math/rand"

	"github.com/ethereum-optimism/optimism/devnet-sdk/contracts/constants"
	"github.com/ethereum-optimism/optimism/op-acceptance-tests/tests/interop"
	"github.com/ethereum-optimism/optimism/op-devstack/devtest"
	"github.com/ethereum-optimism/optimism/op-devstack/dsl"
	"github.com/ethereum-optimism/optimism/op-service/txintent"
	"github.com/ethereum-optimism/optimism/op-service/txplan"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
	"github.com/ethereum/go-ethereum/common"
)

var rng = rand.New(rand.NewSource(1234))

type Initiator interface {
	Initiate(devtest.T, txplan.Option) []types.Message
}

type ManyMsgsInitiator struct {
	el          *dsl.L2ELNode
	eventLogger common.Address
}

var _ Initiator = (*ManyMsgsInitiator)(nil)

func NewManyMsgsInitiator(el *dsl.L2ELNode, eventLogger common.Address) *ManyMsgsInitiator {
	return &ManyMsgsInitiator{
		el:          el,
		eventLogger: eventLogger,
	}
}

func (in *ManyMsgsInitiator) Initiate(t devtest.T, opt txplan.Option) []types.Message {
	const numMsgs = 275 // About the max number of msgs we can create before hitting tx size limits.
	initCalls := make([]txintent.Call, 0, numMsgs)
	for range numMsgs {
		initCalls = append(initCalls, interop.RandomInitTrigger(rng, in.eventLogger, rng.Intn(5), rng.Intn(10)))
	}
	return buildAndSendInitTx(t, in.el, &txintent.MultiTrigger{
		Emitter: constants.MultiCall3,
		Calls:   initCalls,
	}, opt)
}

type LargeMsgInitiator struct {
	el          *dsl.L2ELNode
	eventLogger common.Address
}

var _ Initiator = (*LargeMsgInitiator)(nil)

func NewLargeMsgInitiator(el *dsl.L2ELNode, eventLogger common.Address) *LargeMsgInitiator {
	return &LargeMsgInitiator{
		el:          el,
		eventLogger: eventLogger,
	}
}

func (lin *LargeMsgInitiator) Initiate(t devtest.T, opt txplan.Option) []types.Message {
	// TODO(#16039): can we create an even larger event without the event logger?
	return buildAndSendInitTx(t, lin.el, interop.RandomInitTrigger(rng, lin.eventLogger, 4, 75_000), opt)
}

func buildAndSendInitTx(t devtest.T, el *dsl.L2ELNode, initCall txintent.Call, opts ...txplan.Option) []types.Message {
	initMsgsTx := txintent.NewIntent[txintent.Call, *txintent.InteropOutput](txplan.Combine(opts...), retryForever(el.Escape().EthClient()))
	initMsgsTx.Content.Set(initCall)
	_, err := initMsgsTx.PlannedTx.Success.Eval(t.Ctx())
	t.Require().NoError(err)
	out, err := initMsgsTx.Result.Eval(t.Ctx())
	t.Require().NoError(err)
	return out.Entries
}
