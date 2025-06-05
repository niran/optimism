package loadtest

import (
	"context"
	"math/rand"
	"sync/atomic"
	"time"

	"github.com/ethereum-optimism/optimism/devnet-sdk/contracts/constants"
	"github.com/ethereum-optimism/optimism/op-acceptance-tests/tests/interop"
	"github.com/ethereum-optimism/optimism/op-devstack/devtest"
	"github.com/ethereum-optimism/optimism/op-devstack/dsl"
	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/plan"
	"github.com/ethereum-optimism/optimism/op-service/txintent"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/params"
)

type L2 struct {
	Config       *params.ChainConfig
	RollupConfig *rollup.Config

	budget      *Budget
	eoas        *EOAPool
	el          *dsl.L2ELNode
	eventLogger common.Address

	unsafe atomic.Value
}

func (l2 *L2) PollUnsafe(ctx context.Context, t devtest.T, blockTime time.Duration) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(blockTime):
			info, err := l2.el.Escape().EthClient().InfoByLabel(ctx, eth.Unsafe)
			if ctx.Err() != nil {
				return
			}
			t.Require().NoError(err)
			l2.unsafe.Store(info)
		}
	}
}

func (l2 *L2) Unsafe() eth.BlockInfo {
	return l2.unsafe.Load().(eth.BlockInfo)
}

func (l2 *L2) SendInitiatingMsg(t devtest.T, rng *rand.Rand) (*types.Message, error) {
	start := time.Now()
	tx := txintent.NewIntent[txintent.Call, *txintent.InteropOutput](l2.eoas.Plan(), l2.budget.Plan())
	tx.Content.Set(interop.RandomInitTrigger(rng, l2.eventLogger, rng.Intn(2), rng.Intn(5)))
	if _, err := tx.PlannedTx.Included.Eval(t.Ctx()); err != nil {
		return nil, err
	}
	_, err := tx.PlannedTx.Success.Eval(t.Ctx())
	t.Require().NoError(err)
	messageLatency.WithLabelValues("init").Observe(time.Since(start).Seconds())
	out, err := tx.Result.Eval(t.Ctx())
	t.Require().NoError(err)
	t.Require().Len(out.Entries, 1)
	return &out.Entries[0], nil
}

func (l2 *L2) SendExecutingMsg(t devtest.T, initMsg *types.Message) error {
	start := time.Now()
	tx := txintent.NewIntent[*txintent.ExecTrigger, txintent.Result](l2.eoas.Plan(), l2.budget.Plan())
	tx.Content.Set(&txintent.ExecTrigger{
		Executor: constants.CrossL2Inbox,
		Msg:      *initMsg,
	})
	// The tx is invalid until we know it will be included at a higher timestamp than any of the initiating messages, modulo reorgs.
	// Wait to plan the relay tx against a target block until the timestamp elapses.
	// NOTE: this should be `<`, but the mempool filtering in op-geth currently uses the unsafe head's timestamp instead of
	// the pending timestamp. See https://github.com/ethereum-optimism/op-geth/issues/603.
	tx.PlannedTx.AgainstBlock.Wrap(func(fn plan.Fn[eth.BlockInfo]) plan.Fn[eth.BlockInfo] {
		for l2.Unsafe().Time() <= initMsg.Identifier.Timestamp {
			l2.el.WaitForBlock()
		}
		return fn
	})
	if _, err := tx.PlannedTx.Included.Eval(t.Ctx()); err != nil {
		return err
	}
	_, err := tx.PlannedTx.Success.Eval(t.Ctx())
	t.Require().NoError(err)
	messageLatency.WithLabelValues("exec").Observe(time.Since(start).Seconds())
	return nil
}
