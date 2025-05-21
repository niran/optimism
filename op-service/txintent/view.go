package txintent

import (
	"context"

	"github.com/ethereum-optimism/optimism/op-service/apis"
	"github.com/ethereum-optimism/optimism/op-service/txplan"
	"github.com/ethereum/go-ethereum/common"
)

type Output[O any] interface {
	DecodeOutput(data []byte) (dest O, err error)
}

type CallView[O any] interface {
	Call
	Client() apis.EthClient
	Output[O]
}

func Read[O any](view CallView[O], ctx context.Context, opts ...txplan.Option) (O, error) {
	target, err := view.To()
	if err != nil {
		return *new(O), err
	}
	calldata, err := view.EncodeInput()
	if err != nil {
		return *new(O), err
	}
	client := view.Client()
	tx := txplan.NewPlannedTx(
		txplan.WithAgainstLatestBlock(client),
		txplan.WithContractCall(client),
		txplan.WithData(calldata),
		txplan.WithTo(target),
		// use default sender as null
		txplan.WithSender(common.Address{}),
		txplan.Combine(opts...),
	)
	res, err := tx.Read.Eval(ctx)
	if err != nil {
		return *new(O), err
	}
	decoded, err := view.DecodeOutput(res)
	if err != nil {
		return *new(O), err
	}
	return decoded, nil
}
