package bindings

import (
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/txintent"
	"github.com/ethereum/go-ethereum/common"
	"github.com/lmittmann/w3"
)

type OptimismPortal2CallFactory struct {
	BaseCallFactory
}

func NewOptimismPortal2Factory(opts ...CallFactoryOption) *OptimismPortal2CallFactory {
	return &OptimismPortal2CallFactory{BaseCallFactory: *NewBaseCallFactory(opts...)}
}

func (f *OptimismPortal2CallFactory) DepositTransaction(to common.Address, value eth.ETH, gaslimit uint64, isCreation bool, data []byte) txintent.CallView[any] {
	return &DepositTransactionCall{to: to, OptimismPortal2CallFactory: *f}
}

type OptimismPortal2 struct {
	OptimismPortal2CallFactory

	DepositTransaction func(to common.Address, value eth.ETH, gaslimit uint64, isCreation bool, data []byte) txintent.CallView[any]
}

func NewOptimismPortal2(f *OptimismPortal2CallFactory) *OptimismPortal2 {
	return &OptimismPortal2{
		OptimismPortal2CallFactory: *f,
		DepositTransaction:         f.DepositTransaction,
	}
}

type DepositTransactionCall struct {
	OptimismPortal2CallFactory

	to         common.Address
	value      eth.ETH
	gasLimit   uint64
	isCreation bool
	data       []byte
}

func (c *DepositTransactionCall) EncodeInput() ([]byte, error) {
	abi := w3.MustNewFunc("depositTransaction(address,uint256,uint64,bool,bytes)", "")
	calldata, err := abi.EncodeArgs(c.to, c.value.ToBig(), c.gasLimit, c.isCreation, c.data)
	return calldata, err
}

func (c *DepositTransactionCall) DecodeOutput(data []byte) (any, error) {
	return nil, nil
}
