package dsl

import (
	"math/big"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/txplan"
	stypes "github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/holiman/uint256"
	"github.com/lmittmann/w3"
)

type Call[O any] interface {
	EncodeInput() ([]byte, error)
	DecodeOutput([]byte) (O, error)
}

// Simple Go types, with strong typing (e.g. eth.ETH instead of *big.Int)
type WETH struct {
	// Each field is a function, that is set up automatically with some reflection
	BalanceOf func(addr common.Address) Call[eth.ETH]
	Transfer  func(dest common.Address, amount eth.ETH) Call[bool]
}

type BalanceOfCall struct {
	Addr common.Address
}

func (c BalanceOfCall) EncodeInput() ([]byte, error) {
	// no full type safety yet
	// TODO: rid of w3
	abi := w3.MustNewFunc("balanceOf(address)", "uint256")
	calldata, err := abi.EncodeArgs(c.Addr)
	return calldata, err
}

func (c BalanceOfCall) DecodeOutput(data []byte) (eth.ETH, error) {
	// no full type safety yet
	// TODO: rid of w3
	abi := w3.MustNewFunc("balanceOf(address)", "uint256")
	var result *big.Int // w3 does not like static types and panics
	err := abi.DecodeReturns(data, &result)
	var res eth.ETH
	// TODO: fix manual conversion
	if (*uint256.Int)(&res).SetFromBig(result) {
		panic("not fit in uint256")
	}
	return res, err
}

type TransferCall struct {
	Dest   common.Address
	Amount eth.ETH
}

func (c TransferCall) EncodeInput() ([]byte, error) {
	// no full type safety yet
	// TODO: fix manual conversioin
	// TODO: rid of w3
	amount := c.Amount.ToBig()
	abi := w3.MustNewFunc("transfer(address, uint256)", "bool")
	calldata, err := abi.EncodeArgs(c.Dest, amount)
	return calldata, err
}

func (c TransferCall) DecodeOutput(data []byte) (bool, error) {
	// no full type safety yet
	// TODO: rid of w3
	abi := w3.MustNewFunc("transfer(address, uint256)", "bool")
	var result bool
	err := abi.DecodeReturns(data, &result)
	return result, err
}

// type check
var _ Call[eth.ETH] = (*BalanceOfCall)(nil)
var _ Call[bool] = (*TransferCall)(nil)

// TODO: fill me
type EventLogger struct {
	// no return type
	EmitLog func(topics []eth.Bytes32, data []byte)
}

// TODO: fill me
type CrossL2Inbox struct {
	// no return type
	ValidateMessage func(identifier stypes.Identifier, msgHash eth.Bytes32)
}

// TODO: fill me
type L2ToL2CrossDomainMessenger struct {
	SendMessage  func(dest eth.ChainID, target common.Address, message []byte) Call[eth.Bytes32]
	RelayMessage func(identifier stypes.Identifier, sentMessage []byte) Call[[]byte]
}

// Read calls
func View[O any](user *EOA, address common.Address, call Call[O]) O {
	calldata, err := call.EncodeInput()
	user.t.Require().NoError(err)
	// TODO: abstract away below tx planner
	elClient := user.el.stackEL().EthClient()
	tx := txplan.NewPlannedTx(
		user.key.Plan(),
		txplan.WithAgainstLatestBlock(elClient),
		txplan.WithChainID(elClient),
		txplan.WithData(calldata),
		txplan.WithTo(&address),
		txplan.WithContractCall(elClient),
	)
	res, err := tx.Called.Eval(user.ctx)
	user.t.Require().NoError(err, "call error")
	decoded, err := call.DecodeOutput(res)
	user.t.Require().NoError(err, "result decoding error")
	return decoded
}

// Write calls does not return values. just success/failure
func Write[O any](user *EOA, address common.Address, call Call[O]) {
	calldata, err := call.EncodeInput()
	user.t.Require().NoError(err)
	// TODO: abstract away below tx planner
	tx := txplan.NewPlannedTx(
		user.Plan(),
		txplan.WithData(calldata),
		txplan.WithTo(&address),
	)
	_, err = tx.Included.Eval(user.ctx)
	user.t.Require().NoError(err, "tx inclusion error")
}
