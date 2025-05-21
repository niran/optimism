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
	target, _ := call.To()
	calldata, err := call.EncodeInput()
	if err != nil {
		return nil, err
	}
	tx := txplan.NewPlannedTx(
		txplan.WithData(calldata),
		txplan.WithTo(target),
		txplan.Combine(opts...),
	)
	receipt, err := tx.Included.Eval(ctx)
	if err != nil {
		return nil, err
	}
	return receipt, nil
}
