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

func (f *BaseCallFactory) WithTo(target common.Address) CallFactory {
	f.target = target
	return f
}

func (f *BaseCallFactory) WithClient(client apis.EthClient) CallFactory {
	f.client = client
	return f
}

func (f *BaseCallFactory) WithTest(t devtest.T) CallFactory {
	f.t = t
	return f
}

type CallFactory interface {
	WithTo(common.Address) CallFactory
	WithClient(apis.EthClient) CallFactory
	WithTest(devtest.T) CallFactory
}

var _ CallFactory = (*BaseCallFactory)(nil)
