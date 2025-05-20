package txintent

import (
	"context"
	"github.com/ethereum-optimism/optimism/op-devstack/devtest"
	"github.com/ethereum-optimism/optimism/op-devstack/dsl"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/plan"
	"github.com/ethereum-optimism/optimism/op-service/txplan"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
)

type Contract struct {
	BalanceOf func(targetAccount common.Address) Call2[eth.ETH]
}

type UnboundContract[C Contract] struct{}

func (c *UnboundContract[C]) To(dest common.Address) C {
	var c C
	// hydrate C
	return c
}

type TypedCall[O any] struct {
	Inner Call2
}

type TestTypedCall[O] struct {
	TC TypedCall[O]
	T devtest.T
}

func todo() {
	binding := MakeBinding[Todo]()
	binding2 := binding.To(addr) // TypedCall[O]
	binding3 := dslcall.Test(t, binding2) // TestTypedCall[O] -> remove all error returns
	// return TypedCall, not generic call,
	// so that follow-up functions can infer output typing
	preparedInputObj := binding2.BalanceOf(alice.Addr())
	planOptions := dsl.Plan(preparedInputObj)  // tx.PlanOption
	out := dsl.View(preparedInputObj, ....)
	receipt := dsl.Write(eoa, preparedInputObj)
}

type Input interface {
	EncodeInput() ([]byte, error)
}

type Output interface {
	DecodeOutput(dest any, data []byte) (err error)
}

type Call interface {
	To() (*common.Address, error)
	AccessList() (types.AccessList, error)
	Input
}

// to be able to attach to txplan, no generics here
type Call2 interface {
	Call
	Output
}

type Result interface {
	FromReceipt(ctx context.Context, rec *types.Receipt, includedIn eth.BlockRef, chainID eth.ChainID) error
	Init() Result
}

type IntentTx[V Call, R Result] struct {
	PlannedTx *txplan.PlannedTx
	Content   plan.Lazy[V]
	Result    plan.Lazy[R]
}

func NewIntent[V Call, R Result](opts ...txplan.Option) *IntentTx[V, R] {
	v := &IntentTx[V, R]{
		PlannedTx: txplan.NewPlannedTx(opts...),
	}
	v.PlannedTx.To.DependOn(&v.Content)
	v.PlannedTx.To.Fn(func(ctx context.Context) (*common.Address, error) {
		return v.Content.Value().To()
	})
	v.PlannedTx.Data.DependOn(&v.Content)
	v.PlannedTx.Data.Fn(func(ctx context.Context) (hexutil.Bytes, error) {
		return v.Content.Value().Data()
	})
	v.PlannedTx.AccessList.DependOn(&v.Content)
	v.PlannedTx.AccessList.Fn(func(ctx context.Context) (types.AccessList, error) {
		return v.Content.Value().AccessList()
	})
	v.Result.DependOn(&v.PlannedTx.Included, &v.PlannedTx.IncludedBlock, &v.PlannedTx.ChainID)
	v.Result.Fn(func(ctx context.Context) (R, error) {
		r := (*new(R)).Init().(R)
		err := r.FromReceipt(ctx, v.PlannedTx.Included.Value(), v.PlannedTx.IncludedBlock.Value(), v.PlannedTx.ChainID.Value())
		return r, err
	})
	return v
}
