package bindings

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

type FaucetFactory struct {
	BaseCallFactory
}

func NewFaucetFactory(opts ...CallFactoryOption) *FaucetFactory {
	return &FaucetFactory{BaseCallFactory: *NewBaseCallFactory(opts...)}
}

type Faucet struct {
	FaucetFactory

	Fund func(recipients []common.Address, amount *big.Int) TypedCall[any] `sol:"fund"`
}

func NewFaucet(f *FaucetFactory) *Faucet {
	faucet := &Faucet{
		FaucetFactory: *f,
	}
	InitImpl(faucet)
	return faucet
}
