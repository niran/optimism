package loadtest2

import (
	"context"
	"math/big"
	"slices"
	"sync/atomic"

	"github.com/ethereum-optimism/optimism/devnet-sdk/contracts/constants"
	"github.com/ethereum-optimism/optimism/op-devstack/devtest"
	"github.com/ethereum-optimism/optimism/op-devstack/dsl"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/txintent"
	"github.com/ethereum-optimism/optimism/op-service/txplan"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
)

type ValidRelayer struct {
	el  *dsl.L2ELNode
	eoa *dsl.EOA
	// supervisor is used to check if executing messages are cross-safe.
	supervisor    *dsl.Supervisor
	nonce         atomic.Uint64
	t             devtest.T
	initMsgSource *Bus
}

func NewValidRelayer(t devtest.T, bus *Bus, funder *dsl.Funder, el *dsl.L2ELNode, supervisor *dsl.Supervisor) *ValidRelayer {
	return &ValidRelayer{
		t:             t,
		el:            el,
		eoa:           funder.NewFundedEOA(eth.MillionEther),
		supervisor:    supervisor,
		initMsgSource: bus,
	}
}

func (e *ValidRelayer) SendTx(ctx context.Context) bool {
	tx := buildRelayTx(e.eoa, e.el, e.initMsgSource.Latest(), txplan.WithStaticNonce(e.nonce.Add(1)-1), txplan.WithGasRatio(2))
	if _, err := tx.Included.Eval(ctx); err != nil {
		return false
	}
	_, err := tx.Success.Eval(ctx)
	e.t.Require().NoError(err)
	return true
}

type ToInvalidMsgFn func(types.Message) types.Message

type InvalidRelayer struct {
	eoa           *dsl.EOA
	el            *dsl.L2ELNode
	makeInvalid   ToInvalidMsgFn
	initMsgSource *Bus
	t             devtest.T
}

func NewInvalidRelayer(t devtest.T, bus *Bus, eoa *dsl.EOA, el *dsl.L2ELNode, makeInvalid ToInvalidMsgFn) *InvalidRelayer {
	return &InvalidRelayer{
		t:             t,
		el:            el,
		eoa:           eoa,
		makeInvalid:   makeInvalid,
		initMsgSource: bus,
	}
}

func (ie *InvalidRelayer) SendTx(ctx context.Context) {
	validMsgs := ie.initMsgSource.Latest()
	msgs := make([]types.Message, len(validMsgs))
	copy(msgs, validMsgs)
	// Replace the last message with the invalid message.
	// Merely appending can cause us to hit tx size limits if the original slice has a lot of messages.
	msgs = append(msgs[:len(msgs)-1], ie.makeInvalid(msgs[len(msgs)-1]))
	tx := buildRelayTx(ie.eoa, ie.el, msgs)
	_, err := tx.Submitted.Eval(ctx)
	ie.t.Require().ErrorContains(err, core.ErrTxFilteredOut.Error())
}

func makeInvalidChainID(msg types.Message) types.Message {
	bigChainID := msg.Identifier.ChainID.ToBig()
	bigChainID.Add(bigChainID, big.NewInt(1))
	msg.Identifier.ChainID = eth.ChainIDFromBig(bigChainID)
	return msg
}

func makeInvalidOrigin(msg types.Message) types.Message {
	bigOrigin := msg.Identifier.Origin.Big()
	bigOrigin.Add(bigOrigin, big.NewInt(1))
	msg.Identifier.Origin = common.BigToAddress(bigOrigin)
	return msg
}

func makeInvalidBlockNumber(msg types.Message) types.Message {
	msg.Identifier.BlockNumber++
	return msg
}

func makeInvalidLogIndex(msg types.Message) types.Message {
	msg.Identifier.LogIndex++
	return msg
}

func makeInvalidTimestamp(msg types.Message) types.Message {
	msg.Identifier.Timestamp++
	return msg
}

func makeInvalidPayloadHash(msg types.Message) types.Message {
	bigPayloadHash := msg.PayloadHash.Big()
	bigPayloadHash.Add(bigPayloadHash, big.NewInt(1))
	msg.PayloadHash = common.BigToHash(bigPayloadHash)
	return msg
}

func buildRelayTx(eoa *dsl.EOA, el *dsl.L2ELNode, msgs []types.Message, opts ...txplan.Option) *txplan.PlannedTx {
	execCalls := make([]txintent.Call, 0, len(msgs))
	for _, msg := range msgs {
		execCalls = append(execCalls, &txintent.ExecTrigger{
			Executor: constants.CrossL2Inbox,
			Msg:      msg,
		})
	}
	tx := txintent.NewIntent[*txintent.MultiTrigger, txintent.Result](eoa.Plan(), txplan.Combine(opts...))
	tx.Content.Set(&txintent.MultiTrigger{
		Emitter: constants.MultiCall3,
		Calls:   execCalls,
	})

	maxTimestamp := slices.MaxFunc(msgs, func(x, y types.Message) int {
		if x.Identifier.Timestamp > y.Identifier.Timestamp {
			return 1
		} else if x.Identifier.Timestamp < y.Identifier.Timestamp {
			return -1
		}
		return 0
	}).Identifier.Timestamp
	// The relay tx is invalid until we know it will be included at a higher timestamp than any of the initiating messages, modulo reorgs.
	// NOTE: this should be `<`, but the mempool filtering in op-geth currently uses the unsafe head's timestamp instead of
	// the pending timestamp. See https://github.com/ethereum-optimism/op-geth/issues/603.
	for el.BlockRefByLabel(eth.Unsafe).Time <= maxTimestamp {
		el.WaitForBlock()
	}
	return tx.PlannedTx
}
