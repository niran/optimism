package bindings

import (
	"math/big"
	"reflect"
	"strings"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/predeploys"
	"github.com/ethereum-optimism/optimism/op-service/txintent"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/holiman/uint256"
)

type WETHCallFactory struct {
	BaseCallFactory
}

// TODO: fix hardcode
var WETHMetaData = bind.MetaData{
	ABI: "[{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"src\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"guy\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"wad\",\"type\":\"uint256\"}],\"name\":\"Approval\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"dst\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"wad\",\"type\":\"uint256\"}],\"name\":\"Deposit\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"src\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"dst\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"wad\",\"type\":\"uint256\"}],\"name\":\"Transfer\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"src\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"wad\",\"type\":\"uint256\"}],\"name\":\"Withdrawal\",\"type\":\"event\"},{\"stateMutability\":\"payable\",\"type\":\"fallback\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"owner\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"spender\",\"type\":\"address\"}],\"name\":\"allowance\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"guy\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"wad\",\"type\":\"uint256\"}],\"name\":\"approve\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"src\",\"type\":\"address\"}],\"name\":\"balanceOf\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"decimals\",\"outputs\":[{\"internalType\":\"uint8\",\"name\":\"\",\"type\":\"uint8\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"deposit\",\"outputs\":[],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"name\",\"outputs\":[{\"internalType\":\"string\",\"name\":\"name_\",\"type\":\"string\"}],\"stateMutability\":\"pure\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"symbol\",\"outputs\":[{\"internalType\":\"string\",\"name\":\"symbol_\",\"type\":\"string\"}],\"stateMutability\":\"pure\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"totalSupply\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"dst\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"wad\",\"type\":\"uint256\"}],\"name\":\"transfer\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"src\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"dst\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"wad\",\"type\":\"uint256\"}],\"name\":\"transferFrom\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"version\",\"outputs\":[{\"internalType\":\"string\",\"name\":\"\",\"type\":\"string\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"wad\",\"type\":\"uint256\"}],\"name\":\"withdraw\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"stateMutability\":\"payable\",\"type\":\"receive\"}]",
	Bin: "0x608060405234801561001057600080fd5b50611125806100206000396000f3fe6080604052600436106100ab5760003560e01c806354fd4d501161006457806354fd4d50146101e857806370a082311461021357806395d89b4114610250578063a9059cbb1461027b578063d0e30db0146102b8578063dd62ed3e146102c2576100ba565b806306fdde03146100c4578063095ea7b3146100ef57806318160ddd1461012c57806323b872dd146101575780632e1a7d4d14610194578063313ce567146101bd576100ba565b366100ba576100b86102ff565b005b6100c26102ff565b005b3480156100d057600080fd5b506100d96103a4565b6040516100e69190610b65565b60405180910390f35b3480156100fb57600080fd5b5061011660048036038101906101119190610c2f565b610452565b6040516101239190610c8a565b60405180910390f35b34801561013857600080fd5b50610141610544565b60405161014e9190610cb4565b60405180910390f35b34801561016357600080fd5b5061017e60048036038101906101799190610ccf565b61054c565b60405161018b9190610c8a565b60405180910390f35b3480156101a057600080fd5b506101bb60048036038101906101b69190610d22565b6107c4565b005b3480156101c957600080fd5b506101d26108fc565b6040516101df9190610d6b565b60405180910390f35b3480156101f457600080fd5b506101fd610901565b60405161020a9190610b65565b60405180910390f35b34801561021f57600080fd5b5061023a60048036038101906102359190610d86565b61093a565b6040516102479190610cb4565b60405180910390f35b34801561025c57600080fd5b50610265610982565b6040516102729190610b65565b60405180910390f35b34801561028757600080fd5b506102a2600480360381019061029d9190610c2f565b610a30565b6040516102af9190610c8a565b60405180910390f35b6102c06102ff565b005b3480156102ce57600080fd5b506102e960048036038101906102e49190610db3565b610a45565b6040516102f69190610cb4565b60405180910390f35b346000803373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020600082825461034d9190610e22565b925050819055503373ffffffffffffffffffffffffffffffffffffffff167fe1fffcc4923d04b559f4d29a8bfc6cda04eb5b0d3c460751c2402c5c5cc9109c3460405161039a9190610cb4565b60405180910390a2565b606073420000000000000000000000000000000000001573ffffffffffffffffffffffffffffffffffffffff1663d84447156040518163ffffffff1660e01b8152600401600060405180830381865afa158015610405573d6000803e3d6000fd5b505050506040513d6000823e3d601f19601f8201168201806040525081019061042e9190610f9e565b60405160200161043e9190611049565b604051602081830303815290604052905090565b600081600160003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060008573ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020819055508273ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff167f8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925846040516105329190610cb4565b60405180910390a36001905092915050565b600047905090565b6000816000808673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002054101561059957600080fd5b60006105a58533610a45565b90503373ffffffffffffffffffffffffffffffffffffffff168573ffffffffffffffffffffffffffffffffffffffff161415801561060357507fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff8114155b156106a9578281101561061557600080fd5b82600160008773ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060008282546106a1919061106f565b925050819055505b826000808773ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060008282546106f7919061106f565b92505081905550826000808673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020600082825461074c9190610e22565b925050819055508373ffffffffffffffffffffffffffffffffffffffff168573ffffffffffffffffffffffffffffffffffffffff167fddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef856040516107b09190610cb4565b60405180910390a360019150509392505050565b806000803373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002054101561080f57600080fd5b806000803373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020600082825461085d919061106f565b925050819055503373ffffffffffffffffffffffffffffffffffffffff166108fc829081150290604051600060405180830381858888f193505050501580156108aa573d6000803e3d6000fd5b503373ffffffffffffffffffffffffffffffffffffffff167f7fcf532c15f0a6db0bd6d0e038bea71d30d808c7d98cb3bf7268a95bf5081b65826040516108f19190610cb4565b60405180910390a250565b601281565b6040518060400160405280600581526020017f312e312e3100000000000000000000000000000000000000000000000000000081525081565b60008060008373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020549050919050565b606073420000000000000000000000000000000000001573ffffffffffffffffffffffffffffffffffffffff1663550fcdc96040518163ffffffff1660e01b8152600401600060405180830381865afa1580156109e3573d6000803e3d6000fd5b505050506040513d6000823e3d601f19601f82011682018060405250810190610a0c9190610f9e565b604051602001610a1c91906110c9565b604051602081830303815290604052905090565b6000610a3d33848461054c565b905092915050565b6000600160008473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060008373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002054905092915050565b600081519050919050565b600082825260208201905092915050565b60005b83811015610b06578082015181840152602081019050610aeb565b83811115610b15576000848401525b50505050565b6000601f19601f8301169050919050565b6000610b3782610acc565b610b418185610ad7565b9350610b51818560208601610ae8565b610b5a81610b1b565b840191505092915050565b60006020820190508181036000830152610b7f8184610b2c565b905092915050565b6000604051905090565b600080fd5b600080fd5b600073ffffffffffffffffffffffffffffffffffffffff82169050919050565b6000610bc682610b9b565b9050919050565b610bd681610bbb565b8114610be157600080fd5b50565b600081359050610bf381610bcd565b92915050565b6000819050919050565b610c0c81610bf9565b8114610c1757600080fd5b50565b600081359050610c2981610c03565b92915050565b60008060408385031215610c4657610c45610b91565b5b6000610c5485828601610be4565b9250506020610c6585828601610c1a565b9150509250929050565b60008115159050919050565b610c8481610c6f565b82525050565b6000602082019050610c9f6000830184610c7b565b92915050565b610cae81610bf9565b82525050565b6000602082019050610cc96000830184610ca5565b92915050565b600080600060608486031215610ce857610ce7610b91565b5b6000610cf686828701610be4565b9350506020610d0786828701610be4565b9250506040610d1886828701610c1a565b9150509250925092565b600060208284031215610d3857610d37610b91565b5b6000610d4684828501610c1a565b91505092915050565b600060ff82169050919050565b610d6581610d4f565b82525050565b6000602082019050610d806000830184610d5c565b92915050565b600060208284031215610d9c57610d9b610b91565b5b6000610daa84828501610be4565b91505092915050565b60008060408385031215610dca57610dc9610b91565b5b6000610dd885828601610be4565b9250506020610de985828601610be4565b9150509250929050565b7f4e487b7100000000000000000000000000000000000000000000000000000000600052601160045260246000fd5b6000610e2d82610bf9565b9150610e3883610bf9565b9250827fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff03821115610e6d57610e6c610df3565b5b828201905092915050565b600080fd5b600080fd5b7f4e487b7100000000000000000000000000000000000000000000000000000000600052604160045260246000fd5b610eba82610b1b565b810181811067ffffffffffffffff82111715610ed957610ed8610e82565b5b80604052505050565b6000610eec610b87565b9050610ef88282610eb1565b919050565b600067ffffffffffffffff821115610f1857610f17610e82565b5b610f2182610b1b565b9050602081019050919050565b6000610f41610f3c84610efd565b610ee2565b905082815260208101848484011115610f5d57610f5c610e7d565b5b610f68848285610ae8565b509392505050565b600082601f830112610f8557610f84610e78565b5b8151610f95848260208601610f2e565b91505092915050565b600060208284031215610fb457610fb3610b91565b5b600082015167ffffffffffffffff811115610fd257610fd1610b96565b5b610fde84828501610f70565b91505092915050565b7f5772617070656420000000000000000000000000000000000000000000000000815250565b600081905092915050565b600061102382610acc565b61102d818561100d565b935061103d818560208601610ae8565b80840191505092915050565b600061105482610fe7565b6008820191506110648284611018565b915081905092915050565b600061107a82610bf9565b915061108583610bf9565b92508282101561109857611097610df3565b5b828203905092915050565b7f5700000000000000000000000000000000000000000000000000000000000000815250565b60006110d4826110a3565b6001820191506110e48284611018565b91508190509291505056fea2646970667358221220ba7d2bf0634b627d9304d2cbad2779403dd010d23c9cf4faad42d00f7a8bb21a64736f6c634300080f0033",
}

func NewWETHCallFactory(opts ...CallFactoryOption) *WETHCallFactory {
	base := *NewBaseCallFactory(&WETHMetaData, opts...)
	return &WETHCallFactory{BaseCallFactory: base}
}

func (f *WETHCallFactory) WithDefaultAddr() {
	f.ApplyFactoryOptions(WithTo(common.HexToAddress(predeploys.WETH)))
}

func (f *WETHCallFactory) BalanceOf(addr common.Address) txintent.CallView[eth.ETH] {
	return &Call_balanceOf[eth.ETH]{Addr: addr, WETHCallFactory: *f}
}

func (f *WETHCallFactory) Transfer(dest common.Address, amount eth.ETH) txintent.CallView[bool] {
	return &Call_transfer[bool]{Dest: dest, Amount: amount, WETHCallFactory: *f}
}

type WETH struct {
	WETHCallFactory

	BalanceOf func(addr common.Address) txintent.CallView[eth.ETH]
	Transfer  func(dest common.Address, amount eth.ETH) txintent.CallView[bool]
}

func NewWETH(f *WETHCallFactory) *WETH {
	// a := &WETH{}
	// a.BalanceOf

	return &WETH{
		WETHCallFactory: *f,
		BalanceOf:       f.BalanceOf,
		Transfer:        f.Transfer,
	}
}

type Call_balanceOf[ReturnType eth.ETH] struct {
	WETHCallFactory

	Addr common.Address
}

func extract(c any) string {
	typ := reflect.TypeOf(c)
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}
	typeName := typ.Name()
	// trim prefix
	base := strings.TrimPrefix(typeName, "Call_")
	// trim type
	if idx := strings.Index(base, "["); idx != -1 {
		base = base[:idx]
	}
	return base
}

func encodeType(arg any) any {
	argsTyped, ok := arg.(eth.ETH)
	if ok {
		return argsTyped.ToBig()
	}
	return arg
}

func (c *Call_balanceOf[ReturnType]) EncodeInput() ([]byte, error) {
	name := extract(c)
	return c.ABI.Pack(name, encodeType(c.Addr))
}

func (c *Call_balanceOf[ReturnType]) DecodeOutput(data []byte) (ReturnType, error) {
	name := extract(c)
	out, err := c.ABI.Unpack(name, data)
	if err != nil {
		return *new(ReturnType), err
	}

	out0 := abi.ConvertType(out[0], new(big.Int)).(*big.Int)
	var concrete eth.ETH
	if (*uint256.Int)(&concrete).SetFromBig(out0) {
		panic("result conversion failure: does not fit in uint256")
	}
	return ReturnType(concrete), err
}

type Call_transfer[ReturnType bool] struct {
	WETHCallFactory

	Dest   common.Address
	Amount eth.ETH
}

func (c *Call_transfer[ReturnType]) EncodeInput() ([]byte, error) {
	name := extract(c)

	return c.ABI.Pack(name, encodeType(c.Dest), encodeType(c.Amount))
}

func (c *Call_transfer[ReturnType]) DecodeOutput(data []byte) (ReturnType, error) {
	name := extract(c)
	out, err := c.ABI.Unpack(name, data)
	if err != nil {
		return *new(ReturnType), err
	}

	out0 := *abi.ConvertType(out[0], new(ReturnType)).(*ReturnType)

	return ReturnType(out0), err
}

var _ txintent.CallView[eth.ETH] = (*Call_balanceOf[eth.ETH])(nil)
var _ txintent.CallView[bool] = (*Call_transfer[bool])(nil)
