package dsl

import (
	"fmt"

	"github.com/ethereum-optimism/optimism/op-service/txintent"
	"github.com/ethereum-optimism/optimism/op-service/txplan"
	"github.com/ethereum/go-ethereum/core/types"
)

func checkTestable[O any](call txintent.CallView[O]) txintent.TestCallView[O] {
	callTest, ok := call.(txintent.TestCallView[O])
	if !ok || callTest.Test() == nil {
		panic(fmt.Sprintf("call of type %T does not support testing", call))
	}
	return callTest
}

func Read[O any](call txintent.CallView[O], opts ...txplan.Option) O {
	callTest := checkTestable(call)
	o, err := txintent.Read(call, opts...)
	callTest.Test().Require().NoError(err)
	return o
}

func Write[O any](user *EOA, call txintent.CallView[O], opts ...txplan.Option) *types.Receipt {
	callTest := checkTestable(call)
	finalOpts := txplan.Combine(user.Plan(), txplan.Combine(opts...))
	o, err := txintent.Write(call, finalOpts)
	callTest.Test().Require().NoError(err)
	return o
}
