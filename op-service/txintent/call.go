package txintent

import (
	"context"

	"github.com/ethereum-optimism/optimism/op-service/txplan"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

type Call interface {
	To() (*common.Address, error)
	AccessList() (types.AccessList, error)
	Input
}

type Input interface {
	EncodeInput() ([]byte, error)
}

func Write(call Call, ctx context.Context, opts ...txplan.Option) (*types.Receipt, error) {
	plan, err := Plan(call)
	if err != nil {
		return nil, err
	}
	tx := txplan.NewPlannedTx(plan, txplan.Combine(opts...))
	receipt, err := tx.Included.Eval(ctx)
	if err != nil {
		return nil, err
	}
	return receipt, nil
}

func Plan(call Call) (txplan.Option, error) {
	target, err := call.To()
	if err != nil {
		return nil, err
	}
	calldata, err := call.EncodeInput()
	if err != nil {
		return nil, err
	}
	tx := txplan.Combine(
		txplan.WithData(calldata),
		txplan.WithTo(target),
	)
	return tx, nil
}
