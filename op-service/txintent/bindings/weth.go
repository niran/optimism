package bindings

import (
	"math/big"

	"github.com/ethereum-optimism/optimism/op-devstack/devtest"
	"github.com/ethereum-optimism/optimism/op-service/apis"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/txintent"
	"github.com/ethereum/go-ethereum/common"
	"github.com/holiman/uint256"
	"github.com/lmittmann/w3"
)

type WETHCallFactory struct {
	BaseCallFactory
}

func NewWETHCallFactory(b *BaseCallFactory) *WETHCallFactory {
	return &WETHCallFactory{*b}
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

func (f *WETHCallFactory) BalanceOf(addr common.Address) txintent.CallView[eth.ETH] {
	return &BalanceOfCall{Addr: addr,
		BaseCall: f.BaseCall, BaseCallView: f.BaseCallView, BaseCallTest: f.BaseCallTest}
}

func (f *WETHCallFactory) Transfer(dest common.Address, amount eth.ETH) txintent.CallView[bool] {
	return &TransferCall{Dest: dest, Amount: amount,
		BaseCall: f.BaseCall, BaseCallView: f.BaseCallView, BaseCallTest: f.BaseCallTest}
}

type WETH struct {
	// Each field is a function, that is set up automatically with some reflection
	BalanceOf func(addr common.Address) txintent.CallView[eth.ETH]
	Transfer  func(dest common.Address, amount eth.ETH) txintent.CallView[bool]
}

func NewWETH(f *WETHCallFactory) *WETH {
	return &WETH{
		BalanceOf: f.BalanceOf,
		Transfer:  f.Transfer,
	}
}

type BalanceOfCall struct {
	BaseCall
	BaseCallView
	BaseCallTest

	Addr common.Address
}

func (c *BalanceOfCall) EncodeInput() ([]byte, error) {
	abi := w3.MustNewFunc("balanceOf(address)", "uint256")
	calldata, err := abi.EncodeArgs(c.Addr)
	return calldata, err
}

func (c *BalanceOfCall) DecodeOutput(data []byte) (eth.ETH, error) {
	abi := w3.MustNewFunc("balanceOf(address)", "uint256")
	var result *big.Int // w3 does not like static types and panics
	err := abi.DecodeReturns(data, &result)
	var res eth.ETH
	if (*uint256.Int)(&res).SetFromBig(result) {
		panic("not fit in uint256")
	}
	return res, err
}

type TransferCall struct {
	BaseCall
	BaseCallView
	BaseCallTest

	Dest   common.Address
	Amount eth.ETH
}

func (c *TransferCall) EncodeInput() ([]byte, error) {
	amount := c.Amount.ToBig()
	abi := w3.MustNewFunc("transfer(address, uint256)", "bool")
	calldata, err := abi.EncodeArgs(c.Dest, amount)
	return calldata, err
}

func (c *TransferCall) DecodeOutput(data []byte) (bool, error) {
	abi := w3.MustNewFunc("transfer(address, uint256)", "bool")
	var result bool
	err := abi.DecodeReturns(data, &result)
	return result, err
}

var _ txintent.CallView[eth.ETH] = (*BalanceOfCall)(nil)
var _ txintent.CallView[bool] = (*TransferCall)(nil)
