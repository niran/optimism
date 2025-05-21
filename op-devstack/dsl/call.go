package dsl

import (
	"context"
	"math/big"

	"github.com/ethereum-optimism/optimism/op-devstack/devtest"
	"github.com/ethereum-optimism/optimism/op-service/apis"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/txintent"
	"github.com/ethereum-optimism/optimism/op-service/txplan"
	"github.com/ethereum/go-ethereum/common"
	gethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/holiman/uint256"
	"github.com/lmittmann/w3"
)

type BaseCallFactory struct {
	Target common.Address
	Client apis.EthClient
	T      devtest.T // optional
}

func (f *BaseCallFactory) WithTo(target common.Address) *BaseCallFactory {
	f.Target = target
	return f
}

func (f *BaseCallFactory) WithClient(client apis.EthClient) *BaseCallFactory {
	f.Client = client
	return f
}

func (f *BaseCallFactory) WithTest(t devtest.T) *BaseCallFactory {
	f.T = t
	return f
}

type WETHCallFactory struct {
	BaseCallFactory
}

func (f *WETHCallFactory) WithTo(addr common.Address) *WETHCallFactory {
	f.BaseCallFactory.WithTo(addr)
	return f
}

func (f *WETHCallFactory) WithClient(c apis.EthClient) *WETHCallFactory {
	f.BaseCallFactory.WithClient(c)
	return f
}

func (f *WETHCallFactory) WithTest(t devtest.T) *WETHCallFactory {
	f.BaseCallFactory.WithTest(t)
	return f
}

func (f *WETHCallFactory) BalanceOf(addr common.Address) txintent.View[eth.ETH] {
	return BalanceOfCall{Addr: addr, target: f.Target, client: f.Client, t: f.T}
}

func (f *WETHCallFactory) Transfer(dest common.Address, amount eth.ETH) txintent.View[bool] {
	return TransferCall{Dest: dest, Amount: amount, target: f.Target, client: f.Client, t: f.T}
}

type WETH struct {
	// Each field is a function, that is set up automatically with some reflection
	BalanceOf func(addr common.Address) txintent.View[eth.ETH]
	Transfer  func(dest common.Address, amount eth.ETH) txintent.View[bool]
}

func NewWETH(f *WETHCallFactory) *WETH {
	return &WETH{
		BalanceOf: f.BalanceOf,
		Transfer:  f.Transfer,
	}
}

type BalanceOfCall struct {
	Addr common.Address

	target common.Address
	client apis.EthClient
	t      devtest.T // optional
}

func (c BalanceOfCall) EncodeInput() ([]byte, error) {
	// no full type safety yet
	// TODO: rid of w3
	abi := w3.MustNewFunc("balanceOf(address)", "uint256")
	calldata, err := abi.EncodeArgs(c.Addr)
	return calldata, err
}

func (c BalanceOfCall) DecodeOutput(data []byte) (eth.ETH, error) {
	// no full type safety yet
	// TODO: rid of w3
	abi := w3.MustNewFunc("balanceOf(address)", "uint256")
	var result *big.Int // w3 does not like static types and panics
	err := abi.DecodeReturns(data, &result)
	var res eth.ETH
	// TODO: fix manual conversion
	if (*uint256.Int)(&res).SetFromBig(result) {
		panic("not fit in uint256")
	}
	return res, err
}

func (c BalanceOfCall) WithTo(target common.Address) txintent.View[eth.ETH] {
	c.target = target
	return c
}

func (c BalanceOfCall) To() (*common.Address, error) {
	return &c.target, nil
}

func (c BalanceOfCall) WithClient(client apis.EthClient) txintent.View[eth.ETH] {
	c.client = client
	return c
}

func (c BalanceOfCall) Client() apis.EthClient {
	return c.client
}

func (c BalanceOfCall) AccessList() (gethTypes.AccessList, error) {
	return gethTypes.AccessList{}, nil
}

func (c BalanceOfCall) Test() devtest.T {
	return c.t
}

type TransferCall struct {
	Dest   common.Address
	Amount eth.ETH

	target common.Address
	client apis.EthClient
	t      devtest.T // optional
}

func (c TransferCall) EncodeInput() ([]byte, error) {
	// no full type safety yet
	// TODO: fix manual conversioin
	// TODO: rid of w3
	amount := c.Amount.ToBig()
	abi := w3.MustNewFunc("transfer(address, uint256)", "bool")
	calldata, err := abi.EncodeArgs(c.Dest, amount)
	return calldata, err
}

func (c TransferCall) DecodeOutput(data []byte) (bool, error) {
	// no full type safety yet
	// TODO: rid of w3
	abi := w3.MustNewFunc("transfer(address, uint256)", "bool")
	var result bool
	err := abi.DecodeReturns(data, &result)
	return result, err
}

func (c TransferCall) WithTo(target common.Address) txintent.View[bool] {
	c.target = target
	return c
}

func (c TransferCall) To() (*common.Address, error) {
	return &c.target, nil
}

func (c TransferCall) WithClient(client apis.EthClient) txintent.View[bool] {
	c.client = client
	return c
}

func (c TransferCall) Client() apis.EthClient {
	return c.client
}

func (c TransferCall) AccessList() (gethTypes.AccessList, error) {
	return gethTypes.AccessList{}, nil
}

func (c TransferCall) Test() devtest.T {
	return c.t
}

// type check
var _ txintent.View[eth.ETH] = (*BalanceOfCall)(nil)
var _ txintent.View[bool] = (*TransferCall)(nil)

// // TODO: fill me
// type EventLogger struct {
// 	// no return type
// 	EmitLog func(topics []eth.Bytes32, data []byte)
// }

// type EmitLogCall struct {
// 	Topics     []eth.Bytes32
// 	OpaqueData []byte

// 	Target common.Address
// }

// func (c EmitLogCall) EncodeInput() ([]byte, error) {
// 	abi := w3.MustNewFunc("emitLog(bytes32[] topics, bytes data)", "")
// 	calldata, err := abi.EncodeArgs(c.Topics, c.OpaqueData)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to construct calldata: %w", err)
// 	}
// 	return calldata, nil
// }

// func (c EmitLogCall) DecodeOutput(data []byte) (any, error) {
// 	// no full type safety yet
// 	// TODO: rid of w3
// 	abi := w3.MustNewFunc("emitLog(bytes32[] topics, bytes data)", "")
// 	var result bool
// 	err := abi.DecodeReturns(data, &result)
// 	return result, err
// }

// func (c EmitLogCall) To(target common.Address) {
// 	c.Target = target
// }

// func (c EmitLogCall) WithTo() common.Address {
// 	return c.Target
// }

// var _ Call[any] = (*EmitLogCall)(nil)

// TODO: fill me
// type CrossL2Inbox struct {
// 	// no return type
// 	ValidateMessage func(identifier stypes.Identifier, msgHash eth.Bytes32)
// }

// // TODO: fill me
// type L2ToL2CrossDomainMessenger struct {
// 	SendMessage  func(dest eth.ChainID, target common.Address, message []byte) Call[eth.Bytes32]
// 	RelayMessage func(identifier stypes.Identifier, sentMessage []byte) Call[[]byte]
// }

// Read calls
func View[O any](call txintent.View[O], opts ...txplan.Option) (O, error) {
	target, _ := call.To()
	calldata, err := call.EncodeInput()
	if err != nil {
		return *new(O), err
	}
	// TODO: abstract away below tx planner
	elClient := call.Client()
	tx := txplan.NewPlannedTx(
		txplan.WithAgainstLatestBlock(elClient),
		txplan.WithContractCall(elClient),
		// can be filled in
		txplan.WithData(calldata),
		txplan.WithTo(target),
		// sender is optional when view
		txplan.WithSender(common.Address{}),
		// add optional tx options
		txplan.Combine(opts...),
	)

	// fixme for context
	res, err := tx.Called.Eval(context.Background())
	if err != nil {
		return *new(O), err
	}
	decoded, err := call.DecodeOutput(res)
	if err != nil {
		return *new(O), err
	}
	return decoded, nil
}

func TestView[O any](call txintent.View[O], opts ...txplan.Option) O {
	callTest, ok := call.(txintent.TestView[O])
	if !ok || callTest.Test() == nil {
		panic("call does not support testing")
	}
	o, err := View(call, opts...)
	callTest.Test().Require().NoError(err)
	return o
}

// Write calls does not return values. just success/failure
func Write[O any](user *EOA, call txintent.View[O], opts ...txplan.Option) (*gethTypes.Receipt, error) {
	target, _ := call.To()
	calldata, err := call.EncodeInput()
	if err != nil {
		return nil, err
	}
	user.t.Require().NoError(err)
	// TODO: abstract away below tx planner
	tx := txplan.NewPlannedTx(
		user.Plan(),
		txplan.WithData(calldata),
		txplan.WithTo(target),
		// add optional tx options
		txplan.Combine(opts...),
	)
	receipt, err := tx.Included.Eval(user.ctx)
	if err != nil {
		return nil, err
	}
	return receipt, nil
}

func TestWrite[O any](user *EOA, call txintent.View[O], opts ...txplan.Option) *gethTypes.Receipt {
	callTest, ok := call.(txintent.TestView[O])
	if !ok || callTest.Test() == nil {
		panic("call does not support testing")
	}
	o, err := Write(user, call, opts...)
	callTest.Test().Require().NoError(err)
	return o
}

func Plan[O any](call txintent.View[O]) (txplan.Option, error) {
	target, _ := call.To()
	calldata, err := call.EncodeInput()
	if err != nil {
		return nil, err
	}
	opt := txplan.Combine(
		txplan.WithData(calldata),
		txplan.WithTo(target),
	)
	return opt, nil
}
