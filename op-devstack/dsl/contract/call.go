package contract

import (
	"fmt"

	"github.com/ethereum-optimism/optimism/op-devstack/dsl"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/txintent"
	"github.com/ethereum-optimism/optimism/op-service/txintent/bindings"
	"github.com/ethereum-optimism/optimism/op-service/txintent/contractio"
	"github.com/ethereum-optimism/optimism/op-service/txplan"
	"github.com/ethereum/go-ethereum/core/types"
)

// TestCallView is used in devstack for wrapping errors
type TestCallView[O any] interface {
	txintent.CallView[O]
	Test() bindings.BaseTest
}

// checkTestable checks whether the Call can be used as a DSL using the testing context
func checkTestable[O any](call txintent.CallView[O]) TestCallView[O] {
	callTest, ok := call.(TestCallView[O])
	if !ok || callTest.Test() == nil {
		panic(fmt.Sprintf("call of type %T does not support testing", call))
	}
	return callTest
}

// Read executes a new message call without creating a transaction on the blockchain
func Read[O any](call txintent.CallView[O], opts ...txplan.Option) O {
	callTest := checkTestable(call)
	o, err := contractio.Read(call, callTest.Test().Ctx(), opts...)
	callTest.Test().Require().NoError(err)
	return o
}

// Write makes a user to write a tx by using the planned contract bindings
func Write[O any](user *dsl.EOA, call txintent.CallView[O], opts ...txplan.Option) *types.Receipt {
	callTest := checkTestable(call)
	finalOpts := txplan.Combine(user.Plan(), txplan.Combine(opts...))
	o, err := contractio.Write(call, callTest.Test().Ctx(), finalOpts)
	callTest.Test().Require().NoError(err)
	return o
}

type TestCallView3[O any] interface {
	Test() bindings.BaseTest
}

func checkTestable3[O any](call bindings.TypedCall[O]) {
	callTest, ok := any(call).(TestCallView3[O])
	if !ok || callTest.Test() == nil {
		panic(fmt.Sprintf("call of type %T does not support testing", call))
	}
}

func Read3[O any](call bindings.TypedCall[O], opts ...txplan.Option) O {
	checkTestable3(call)
	o, err := contractio.Read3(call, call.Test().Ctx(), opts...)
	call.Test().Require().NoError(err)
	return o
}

func Write3[O any](user *dsl.EOA, call bindings.TypedCall[O], opts ...txplan.Option) *types.Receipt {
	checkTestable3(call)
	finalOpts := txplan.Combine(user.Plan(), txplan.Combine(opts...))
	o, err := contractio.Write3(call, call.Test().Ctx(), finalOpts)
	call.Test().Require().NoError(err)
	return o
}

var _ TestCallView[eth.ETH] = (*bindings.Call_balanceOf[eth.ETH])(nil)
var _ TestCallView[bool] = (*bindings.Call_transfer[bool])(nil)
