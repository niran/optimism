// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package bindings

import (
	"errors"
	"math/big"
	"strings"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
)

// Reference imports to suppress errors if they are not otherwise used.
var (
	_ = errors.New
	_ = big.NewInt
	_ = strings.NewReader
	_ = ethereum.NotFound
	_ = bind.Bind
	_ = common.Big1
	_ = types.BloomLookup
	_ = event.NewSubscription
)

// Identifier is an auto generated low-level Go binding around an user-defined struct.
// type Identifier struct {
// 	Origin      common.Address
// 	BlockNumber *big.Int
// 	LogIndex    *big.Int
// 	Timestamp   *big.Int
// 	ChainId     *big.Int
// }

// EventloggerMetaData contains all meta data concerning the Eventlogger contract.
var EventloggerMetaData = &bind.MetaData{
	ABI: "[{\"inputs\":[{\"internalType\":\"bytes32[]\",\"name\":\"_topics\",\"type\":\"bytes32[]\"},{\"internalType\":\"bytes\",\"name\":\"_data\",\"type\":\"bytes\"}],\"name\":\"emitLog\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"components\":[{\"internalType\":\"address\",\"name\":\"origin\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"blockNumber\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"logIndex\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"timestamp\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"chainId\",\"type\":\"uint256\"}],\"internalType\":\"structIdentifier\",\"name\":\"_id\",\"type\":\"tuple\"},{\"internalType\":\"bytes32\",\"name\":\"_msgHash\",\"type\":\"bytes32\"}],\"name\":\"validateMessage\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"}]",
	Bin: "0x6080604052348015600e575f80fd5b506102ac8061001c5f395ff3fe608060405234801561000f575f80fd5b5060043610610034575f3560e01c8063ab4d6f7514610038578063edebc13b1461004d575b5f80fd5b61004b61004636600461013e565b610060565b005b61004b61005b36600461016c565b6100bd565b60405163ab4d6f7560e01b81526022602160991b019063ab4d6f759061008c9085908590600401610226565b5f604051808303815f87803b1580156100a3575f80fd5b505af11580156100b5573d5f803e3d5ffd5b505050505050565b80604051818482378486356020880135604089013560608a0135848015610102576001811461010a5760028114610113576003811461011d5760048114610128575f80fd5b8787a0610130565b848888a1610130565b83858989a2610130565b8284868a8aa3610130565b818385878b8ba45b505050505050505050505050565b5f8082840360c0811215610150575f80fd5b60a081121561015d575f80fd5b50919360a08501359350915050565b5f805f806040858703121561017f575f80fd5b843567ffffffffffffffff80821115610196575f80fd5b818701915087601f8301126101a9575f80fd5b8135818111156101b7575f80fd5b8860208260051b85010111156101cb575f80fd5b6020928301965094509086013590808211156101e5575f80fd5b818701915087601f8301126101f8575f80fd5b813581811115610206575f80fd5b886020828501011115610217575f80fd5b95989497505060200194505050565b60c0810183356001600160a01b038116808214610241575f80fd5b8352506020848101359083015260408085013590830152606080850135908301526080938401359382019390935260a001529056fea26469706673582212206da9bc84d514e1a78e2b4160f99f93aa58672040ece82f45ac2a878aeefdfbe164736f6c63430008190033",
}

// EventloggerABI is the input ABI used to generate the binding from.
// Deprecated: Use EventloggerMetaData.ABI instead.
var EventloggerABI = EventloggerMetaData.ABI

// EventloggerBin is the compiled bytecode used for deploying new contracts.
// Deprecated: Use EventloggerMetaData.Bin instead.
var EventloggerBin = EventloggerMetaData.Bin

// DeployEventlogger deploys a new Ethereum contract, binding an instance of Eventlogger to it.
func DeployEventlogger(auth *bind.TransactOpts, backend bind.ContractBackend) (common.Address, *types.Transaction, *Eventlogger, error) {
	parsed, err := EventloggerMetaData.GetAbi()
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	if parsed == nil {
		return common.Address{}, nil, nil, errors.New("GetABI returned nil")
	}

	address, tx, contract, err := bind.DeployContract(auth, *parsed, common.FromHex(EventloggerBin), backend)
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	return address, tx, &Eventlogger{EventloggerCaller: EventloggerCaller{contract: contract}, EventloggerTransactor: EventloggerTransactor{contract: contract}, EventloggerFilterer: EventloggerFilterer{contract: contract}}, nil
}

// Eventlogger is an auto generated Go binding around an Ethereum contract.
type Eventlogger struct {
	EventloggerCaller     // Read-only binding to the contract
	EventloggerTransactor // Write-only binding to the contract
	EventloggerFilterer   // Log filterer for contract events
}

// EventloggerCaller is an auto generated read-only Go binding around an Ethereum contract.
type EventloggerCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// EventloggerTransactor is an auto generated write-only Go binding around an Ethereum contract.
type EventloggerTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// EventloggerFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type EventloggerFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// EventloggerSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type EventloggerSession struct {
	Contract     *Eventlogger      // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// EventloggerCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type EventloggerCallerSession struct {
	Contract *EventloggerCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts      // Call options to use throughout this session
}

// EventloggerTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type EventloggerTransactorSession struct {
	Contract     *EventloggerTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts      // Transaction auth options to use throughout this session
}

// EventloggerRaw is an auto generated low-level Go binding around an Ethereum contract.
type EventloggerRaw struct {
	Contract *Eventlogger // Generic contract binding to access the raw methods on
}

// EventloggerCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type EventloggerCallerRaw struct {
	Contract *EventloggerCaller // Generic read-only contract binding to access the raw methods on
}

// EventloggerTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type EventloggerTransactorRaw struct {
	Contract *EventloggerTransactor // Generic write-only contract binding to access the raw methods on
}

// NewEventlogger creates a new instance of Eventlogger, bound to a specific deployed contract.
func NewEventlogger(address common.Address, backend bind.ContractBackend) (*Eventlogger, error) {
	contract, err := bindEventlogger(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &Eventlogger{EventloggerCaller: EventloggerCaller{contract: contract}, EventloggerTransactor: EventloggerTransactor{contract: contract}, EventloggerFilterer: EventloggerFilterer{contract: contract}}, nil
}

// NewEventloggerCaller creates a new read-only instance of Eventlogger, bound to a specific deployed contract.
func NewEventloggerCaller(address common.Address, caller bind.ContractCaller) (*EventloggerCaller, error) {
	contract, err := bindEventlogger(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &EventloggerCaller{contract: contract}, nil
}

// NewEventloggerTransactor creates a new write-only instance of Eventlogger, bound to a specific deployed contract.
func NewEventloggerTransactor(address common.Address, transactor bind.ContractTransactor) (*EventloggerTransactor, error) {
	contract, err := bindEventlogger(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &EventloggerTransactor{contract: contract}, nil
}

// NewEventloggerFilterer creates a new log filterer instance of Eventlogger, bound to a specific deployed contract.
func NewEventloggerFilterer(address common.Address, filterer bind.ContractFilterer) (*EventloggerFilterer, error) {
	contract, err := bindEventlogger(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &EventloggerFilterer{contract: contract}, nil
}

// bindEventlogger binds a generic wrapper to an already deployed contract.
func bindEventlogger(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(EventloggerABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Eventlogger *EventloggerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _Eventlogger.Contract.EventloggerCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Eventlogger *EventloggerRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Eventlogger.Contract.EventloggerTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Eventlogger *EventloggerRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Eventlogger.Contract.EventloggerTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Eventlogger *EventloggerCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _Eventlogger.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Eventlogger *EventloggerTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Eventlogger.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Eventlogger *EventloggerTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Eventlogger.Contract.contract.Transact(opts, method, params...)
}

// EmitLog is a paid mutator transaction binding the contract method 0xedebc13b.
//
// Solidity: function emitLog(bytes32[] _topics, bytes _data) returns()
func (_Eventlogger *EventloggerTransactor) EmitLog(opts *bind.TransactOpts, _topics [][32]byte, _data []byte) (*types.Transaction, error) {
	return _Eventlogger.contract.Transact(opts, "emitLog", _topics, _data)
}

// EmitLog is a paid mutator transaction binding the contract method 0xedebc13b.
//
// Solidity: function emitLog(bytes32[] _topics, bytes _data) returns()
func (_Eventlogger *EventloggerSession) EmitLog(_topics [][32]byte, _data []byte) (*types.Transaction, error) {
	return _Eventlogger.Contract.EmitLog(&_Eventlogger.TransactOpts, _topics, _data)
}

// EmitLog is a paid mutator transaction binding the contract method 0xedebc13b.
//
// Solidity: function emitLog(bytes32[] _topics, bytes _data) returns()
func (_Eventlogger *EventloggerTransactorSession) EmitLog(_topics [][32]byte, _data []byte) (*types.Transaction, error) {
	return _Eventlogger.Contract.EmitLog(&_Eventlogger.TransactOpts, _topics, _data)
}

// ValidateMessage is a paid mutator transaction binding the contract method 0xab4d6f75.
//
// Solidity: function validateMessage((address,uint256,uint256,uint256,uint256) _id, bytes32 _msgHash) returns()
func (_Eventlogger *EventloggerTransactor) ValidateMessage(opts *bind.TransactOpts, _id Identifier, _msgHash [32]byte) (*types.Transaction, error) {
	return _Eventlogger.contract.Transact(opts, "validateMessage", _id, _msgHash)
}

// ValidateMessage is a paid mutator transaction binding the contract method 0xab4d6f75.
//
// Solidity: function validateMessage((address,uint256,uint256,uint256,uint256) _id, bytes32 _msgHash) returns()
func (_Eventlogger *EventloggerSession) ValidateMessage(_id Identifier, _msgHash [32]byte) (*types.Transaction, error) {
	return _Eventlogger.Contract.ValidateMessage(&_Eventlogger.TransactOpts, _id, _msgHash)
}

// ValidateMessage is a paid mutator transaction binding the contract method 0xab4d6f75.
//
// Solidity: function validateMessage((address,uint256,uint256,uint256,uint256) _id, bytes32 _msgHash) returns()
func (_Eventlogger *EventloggerTransactorSession) ValidateMessage(_id Identifier, _msgHash [32]byte) (*types.Transaction, error) {
	return _Eventlogger.Contract.ValidateMessage(&_Eventlogger.TransactOpts, _id, _msgHash)
}
