package bindings

import (
	"context"
	"errors"
	"reflect"

	"github.com/ethereum-optimism/optimism/op-service/apis"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"
)

// BaseCall contains fields to populate fields of txplan
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

// BaseCall contains client for reading the blockchain
type BaseCallView struct {
	client apis.EthClient
}

func (c *BaseCallView) Client() apis.EthClient {
	return c.client
}

// BaseCall represents minimal testing interface
type BaseTest interface {
	Require() *require.Assertions
	Ctx() context.Context
}

// BaseCallTest contains tester to embed for the CallFactory
type BaseCallTest struct {
	t BaseTest
}

func (c *BaseCallTest) Test() BaseTest {
	return c.t
}

// BaseCallFactory composes building blocks for initializing contract factory.
// Intended to be embedded while adding contract binding factory.
type BaseCallFactory struct {
	BaseCall
	BaseCallView
	BaseCallTest
	BaseCodec
}

// TODO: implement better checker
func CheckImpl(parent any) {
	t := reflect.TypeOf(parent)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	for i := range t.NumField() {
		field := t.Field(i)
		fieldType := field.Type

		if fieldType.Kind() != reflect.Func {
			continue
		}
		// panic when when lambda, they must have `sol` tag
		if len(field.Tag.Get("sol")) == 0 {
			panic("all function arguments must have sol tags")
		}
	}
}

// Options to populate the factory
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

func WithTest(t BaseTest) CallFactoryOption {
	return func(f *BaseCallFactory) {
		f.t = t
	}
}

func NewBaseCallFactory(bind *bind.MetaData, opts ...CallFactoryOption) *BaseCallFactory {
	b := &BaseCallFactory{}
	b.GetABI(bind)
	b.ApplyFactoryOptions(opts...)
	return b
}

func (b *BaseCallFactory) ApplyFactoryOptions(opts ...CallFactoryOption) {
	for _, opt := range opts {
		opt(b)
	}
}

// need to be eventually removed
type BaseCodec struct {
	ABI *abi.ABI
}

func (bc *BaseCodec) GetABI(bind *bind.MetaData) {
	abi, err := bind.GetAbi()
	if err != nil {
		panic(errors.New("invalid ABI: " + err.Error()))
	}
	bc.ABI = abi
}
