package bindings

import (
	"github.com/ethereum-optimism/optimism/op-devstack/devtest"
	"github.com/ethereum-optimism/optimism/op-service/apis"
	"github.com/ethereum/go-ethereum/common"
)

type BaseCallFactory struct {
	Target common.Address
	Client apis.EthClient
	T      devtest.T
}

func (f *BaseCallFactory) WithTo(target common.Address) CallFactory {
	f.Target = target
	return f
}

func (f *BaseCallFactory) WithClient(client apis.EthClient) CallFactory {
	f.Client = client
	return f
}

func (f *BaseCallFactory) WithTest(t devtest.T) CallFactory {
	f.T = t
	return f
}

type CallFactory interface {
	WithTo(common.Address) CallFactory
	WithClient(apis.EthClient) CallFactory
	WithTest(devtest.T) CallFactory
}

var _ CallFactory = (*BaseCallFactory)(nil)
