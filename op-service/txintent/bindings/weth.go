package bindings

import (
	"errors"
	"fmt"
	"math/big"
	"reflect"
	"strings"

	"github.com/ethereum-optimism/optimism/op-chain-ops/script"
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

	BalanceOf func(addr common.Address) txintent.CallView[eth.ETH]              `sol:"balanceOf"`
	Transfer  func(dest common.Address, amount eth.ETH) txintent.CallView[bool] `sol:"transfer"`

	BalanceOf2 func(addr common.Address) Call                 `sol:"balanceOf"`
	Transfer2  func(dest common.Address, amount eth.ETH) Call `sol:"transfer"`

	// TODO: solidity methods which starts with lowercase must be also exportable. Think about convention
	BalanceOf3 func(addr common.Address) TypedCall[eth.ETH]              `sol:"balanceOf"`
	Transfer3  func(dest common.Address, amount eth.ETH) TypedCall[bool] `sol:"transfer"`

	// not using op-service types yet
	BalanceOf4 func(addr common.Address) TypedCall[eth.ETH]              `sol:"balanceOf"`
	Transfer4  func(dest common.Address, amount eth.ETH) TypedCall[bool] `sol:"transfer"`
}

func extractGenericArg(t reflect.Type) string {
	s := t.String() // e.g., "txintent.CallView[bool]"
	start := strings.Index(s, "[")
	end := strings.LastIndex(s, "]")
	if start == -1 || end == -1 || start > end {
		return ""
	}
	return s[start+1 : end] // returns "bool"
}

// elective
var typeRegistry = map[string]reflect.Type{
	"bool":           reflect.TypeOf(true),
	"string":         reflect.TypeOf(""),
	"int":            reflect.TypeOf(0),
	"common.Address": reflect.TypeOf(common.Address{}),
	"github.com/ethereum-optimism/optimism/op-service/eth.ETH": reflect.TypeOf(eth.ETH{}),
	"bindings.Call": reflect.TypeOf(Call{}),
	"*math/big.Int": reflect.TypeOf(*big.NewInt(0)),
	// add more as needed
}

// elective
func lookupType(typeStr string) reflect.Type {
	return typeRegistry[typeStr]
}

func encoder(name string, args ...any) ([]byte, error) {
	// fillme fixme: do not care about types
	// use geth ABI.pack

	// something like makeArgs
	inputs, outputs := []abi.Argument{}, []abi.Argument{}
	args_translated := []any{}
	for i, arg := range args {
		var typ reflect.Type
		// handle op service types
		switch v := arg.(type) {
		case eth.ETH:
			argsTyped := v.ToBig()
			typ = reflect.TypeOf(argsTyped)
			args_translated = append(args_translated, argsTyped)
		default:
			typ = reflect.TypeOf(arg)
			args_translated = append(args_translated, arg)
		}
		abiTyp, err := script.GoTypeToABIType(typ)
		if err != nil {
			panic("go type to abi type")
		}
		input := abi.Argument{
			Name: fmt.Sprintf("arg_%d", i),
			Type: abiTyp,
		}
		inputs = append(inputs, input)
	}

	// no need return types to build calldata
	// internally initializes sig and ID
	// use dummy vars but calldata does not care
	method := abi.NewMethod(name, name, abi.Function, "payable", false, false, inputs, outputs)
	arguments, err := method.Inputs.Pack(args_translated...)
	if err != nil {
		panic(err)
	}

	// fmt.Println(len(args))
	// fmt.Println(len(inputs))
	// fmt.Println(name)
	// fmt.Println(len(arguments))
	// fmt.Println(hex.EncodeToString(arguments))

	result := append(method.ID, arguments...)

	// fmt.Println(hex.EncodeToString(result))

	return result, err

	// addr, ok := args[0].(common.Address)
	// if !ok {
	// 	panic("this is only for test. fix me okay?")
	// }

	// ret := addr.Bytes()
	// ret = append(ret, []byte{0x41, 0x42, 0x43}...)
	// ret = append(ret, []byte(name)...)
	// return ret, nil
}

func decoder(data []byte) (any, error) {
	// fillme fixme: use geth ABI.unpack
	// use geth ABI.unpack
	// no type yet

	// TODO: at this point, we need the return type's type to correctly decode.
	// we do not need a lambda for decoder

	// NOT used in version 3

	// example: fixme
	return string(data), nil
}

func NewWETH(f *WETHCallFactory) *WETH {
	// infer here
	weth := WETH{WETHCallFactory: *f}
	CheckImpl(weth)

	v := reflect.ValueOf(&weth).Elem()

	t := reflect.TypeOf(weth)

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fieldType := field.Type

		if fieldType.Kind() == reflect.Func {

			methodName := field.Tag.Get("sol")

			fmt.Printf("Field %q is a function with %d input(s) and %d output(s):\n",
				field.Name, fieldType.NumIn(), fieldType.NumOut())

			inputTypes := []reflect.Type{}
			for j := 0; j < fieldType.NumIn(); j++ {
				fmt.Printf("    Param %d: %s\n", j, fieldType.In(j))
				inputTypes = append(inputTypes, fieldType.In(j))
			}
			outputTypes := []reflect.Type{}
			var originalOutputType reflect.Type
			for j := 0; j < fieldType.NumOut(); j++ {
				t := fieldType.Out(j)
				originalOutputType = t
				fmt.Printf("    Return %d: %s\n", j, t)
				genericArg := extractGenericArg(t)
				if genericArg == "" { // non generic typed
					genericArg = t.String()
				}
				fmt.Printf("    Return %d: %s\n", j, genericArg)
				v := reflect.New(lookupType(genericArg)).Elem()
				fmt.Printf("    Return %d: %s\n", j, v.Type())
				// outputTypes = append(outputTypes, t)
				outputTypes = append(outputTypes, v.Type())
			}
			fmt.Println("originalOutputType", originalOutputType)

			// outer: func(...args) -> <inner: (func() -> (bytes[], error))>
			// inner: func() -> (bytes[], error)
			funcInput := reflect.FuncOf([]reflect.Type{}, []reflect.Type{reflect.TypeOf([]byte{}), reflect.TypeOf((*error)(nil)).Elem()}, false)
			funcInputWrapper := reflect.FuncOf(inputTypes, []reflect.Type{funcInput}, false)

			fmt.Println("funcInput", funcInput)
			fmt.Println("funcInputWrapper", funcInputWrapper)

			outputType := outputTypes[0]
			_ = outputType

			outputAnyTypes := []reflect.Type{reflect.TypeOf((*any)(nil)).Elem(), reflect.TypeOf((*error)(nil)).Elem()}

			// outer:
			// inner: func([]byte) -> (any, error)
			funcOutput := reflect.FuncOf([]reflect.Type{reflect.TypeOf([]byte{})}, outputAnyTypes, false)
			fmt.Println("funcOutput", funcOutput)

			// λ

			// closure: higher order function: outer: bind args to inner λ
			encoderLambdaLambda := reflect.MakeFunc(funcInputWrapper, func(argsOuter []reflect.Value) []reflect.Value {
				encoderLambda := reflect.MakeFunc(funcInput, func(argsInner []reflect.Value) []reflect.Value {
					callArgs := make([]any, len(argsOuter))
					for i, a := range argsOuter {
						callArgs[i] = a.Interface()
					}
					methodName := field.Tag.Get("sol") // TODO: make tag as constant
					if len(methodName) == 0 {
						panic("invalid method name")
					}
					v0, v1 := encoder(methodName, callArgs...)

					// guard
					var val0 reflect.Value
					if v0 == nil {
						val0 = reflect.Zero(reflect.TypeOf([]byte{}))
					} else {
						val0 = reflect.ValueOf(v0)
					}
					var val1 reflect.Value
					if v1 == nil {
						val1 = reflect.Zero(reflect.TypeOf((*error)(nil)).Elem())
					} else {
						val1 = reflect.ValueOf(v1)
					}

					return []reflect.Value{val0, val1}
				})
				inner := encoderLambda.Interface().(func() ([]byte, error))
				return []reflect.Value{reflect.ValueOf(inner)}
			})

			decoderLambda := reflect.MakeFunc(funcOutput, func(args []reflect.Value) []reflect.Value {
				data := args[0].Interface().([]byte)
				v0, v1 := decoder(data)

				// guard
				var val0 reflect.Value
				if v0 == nil {
					val0 = reflect.Zero(reflect.TypeOf((*any)(nil)).Elem())
				} else {
					val0 = reflect.ValueOf(v0)
				}
				var val1 reflect.Value
				if v1 == nil {
					val1 = reflect.Zero(reflect.TypeOf((*error)(nil)).Elem())
				} else {
					val1 = reflect.ValueOf(v1)
				}

				return []reflect.Value{val0, val1}
			})

			// decodeLambda := decodeUnwrap()

			// test
			// TODO: remove hardcode
			if field.Name == "BalanceOf2" || field.Name == "Transfer2" {
				// func(...args) -> Call
				λ := reflect.MakeFunc(fieldType, func(args []reflect.Value) []reflect.Value {
					innerResults := encoderLambdaLambda.Call(args)
					if len(innerResults) != 1 {
						panic("expected one return value")
					}
					innerλ := innerResults[0].Interface().(func() ([]byte, error))
					decoderλ := decoderLambda.Interface().(func([]byte) (any, error))
					realcall := Call{
						BaseCallFactory:    &f.BaseCallFactory,
						MethodName:         methodName,
						EncodeInputLambda:  innerλ,
						DecodeOutputLambda: decoderλ,
					}
					return []reflect.Value{reflect.ValueOf(realcall)}
				})

				// panic when type mismatch
				v.FieldByName(field.Name).Set(λ)
			}
			// TODO: remove hardcode
			if field.Name == "BalanceOf3" || field.Name == "Transfer3" {
				// func(...args) -> TypedCall[ReturnValue]
				λ := reflect.MakeFunc(fieldType, func(args []reflect.Value) []reflect.Value {
					innerResults := encoderLambdaLambda.Call(args)
					if len(innerResults) != 1 {
						panic("expected one return value")
					}
					innerλ := innerResults[0].Interface().(func() ([]byte, error))
					decoderλ := decoderLambda.Interface().(func([]byte) (any, error))

					wrap := reflect.New(originalOutputType).Elem()

					wrap.FieldByName("MethodName").Set(reflect.ValueOf(methodName))
					wrap.FieldByName("EncodeInputLambda").Set(reflect.ValueOf(innerλ))
					wrap.FieldByName("DecodeOutputLambda").Set(reflect.ValueOf(decoderλ))
					wrap.FieldByName("BaseCallFactory").Set(reflect.ValueOf(&f.BaseCallFactory))

					return []reflect.Value{wrap}
				})
				// panic when type mismatch
				v.FieldByName(field.Name).Set(λ)

			}
		}
	}

	weth.BalanceOf = f.BalanceOf
	weth.Transfer = f.Transfer

	// a := weth.BalanceOf3(common.HexToAddress("0x30313233"))
	// ret, _ := a.EncodeInput()
	// fmt.Printf("calldata: %s\n", hex.EncodeToString(ret))
	// ret2, _ := a.DecodeOutput([]byte{0x41, 0x42, 0x41})
	// fmt.Println(ret2)
	// fmt.Println(a.MethodName)

	// TODO: check field lambdas are not nil and properly initialized

	return &weth
}

type Call struct {
	*BaseCallFactory

	MethodName string

	EncodeInputLambda  func() ([]byte, error)
	DecodeOutputLambda func(data []byte) (dest any, err error)
}

func (c *Call) EncodeInput() ([]byte, error) {
	return c.EncodeInputLambda()
}

func (c *Call) DecodeOutput(data []byte) (any, error) {
	return c.DecodeOutputLambda(data)
}

var _ txintent.CallView[any] = (*Call)(nil)

type TypedCall[ReturnType any] struct {
	Call
}

func (c *TypedCall[ReturnType]) EncodeInput() ([]byte, error) {
	return c.EncodeInputLambda()
}

func (c *TypedCall[ReturnType]) DecodeOutput(data []byte) (ReturnType, error) {
	// we have the type here: as Return
	// we do not need a decodeinput lambda here; no lazy evaluation
	// _, _ = c.DecodeOutputLambda(data)

	var zero ReturnType
	retTyp := reflect.TypeOf(zero)

	// Special handling for eth.ETH
	var abiTargetType reflect.Type
	if retTyp == reflect.TypeOf(eth.ETH{}) {
		abiTargetType = reflect.TypeOf(big.NewInt(0))
	} else {
		abiTargetType = retTyp
	}

	abiType, err := script.GoTypeToABIType(abiTargetType)
	if err != nil {
		return zero, fmt.Errorf("failed to convert Go type to ABI type: %w", err)
	}

	outputs := abi.Arguments{{Type: abiType}}
	decoded, err := outputs.Unpack(data)
	if err != nil {
		return zero, fmt.Errorf("ABI unpack error: %w", err)
	}

	// TODO: handle multiple returns
	val := decoded[0]

	// Special handling for eth.ETH
	switch retTyp {
	case reflect.TypeOf(eth.ETH{}):
		bigVal := abi.ConvertType(val, new(big.Int)).(*big.Int)
		var concrete eth.ETH
		if (*uint256.Int)(&concrete).SetFromBig(bigVal) {
			return zero, errors.New("result conversion failure: does not fit in uint256")
		}
		return any(concrete).(ReturnType), nil
	default:
		ptr := abi.ConvertType(val, new(ReturnType)).(*ReturnType)
		return *ptr, nil
	}
}

var _ txintent.CallView[any] = (*TypedCall[any])(nil)

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
