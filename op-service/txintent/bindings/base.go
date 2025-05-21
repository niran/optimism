package bindings

import (
	"github.com/ethereum-optimism/optimism/op-devstack/devtest"
	"github.com/ethereum-optimism/optimism/op-service/apis"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

type BaseCall struct {
	target     common.Address
	accessList types.AccessList
}

func (c *BaseCall) To() (*common.Address, error) {
	return &c.target, nil
}

func (c *BaseCall) AccessList() (types.AccessList, error) {
	return c.accessList, nil
}

type BaseCallView struct {
	client apis.EthClient
}

func (c *BaseCallView) Client() apis.EthClient {
	return c.client
}

type BaseCallTest struct {
	t devtest.T
}

func (c *BaseCallTest) Test() devtest.T {
	return c.t
}

type BaseCallFactory struct {
	BaseCall
	BaseCallView
	BaseCallTest
}

type CallFactoryOption func(*BaseCallFactory)

func WithTo(target common.Address) CallFactoryOption {
	return func(f *BaseCallFactory) {
		f.target = target
	}
}

func WithClient(client apis.EthClient) CallFactoryOption {
	return func(f *BaseCallFactory) {
		f.client = client
	}
}

func WithTest(t devtest.T) CallFactoryOption {
	return func(f *BaseCallFactory) {
		f.t = t
	}
}

func NewBaseCallFactory(opts ...CallFactoryOption) *BaseCallFactory {
	b := &BaseCallFactory{}
	for _, opt := range opts {
		opt(b)
	}
	return b
}
