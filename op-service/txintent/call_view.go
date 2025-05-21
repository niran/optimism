package txintent

import (
	"context"

	"github.com/ethereum-optimism/optimism/op-service/apis"
	"github.com/ethereum-optimism/optimism/op-service/txplan"
	"github.com/ethereum/go-ethereum/common"
)

// CallView expresses minimal representation to plan transaction to view, embedding Call interface.
// It is typed for interpreting the read result, and binds client for viewing.
type CallView[O any] interface {
	Call
	Output[O]
	Client() apis.EthClient
}

type Output[O any] interface {
	DecodeOutput(data []byte) (dest O, err error)
}

// Read receives a CallView and uses to plan transaction, and attempts to read.
func Read[O any](view CallView[O], ctx context.Context, opts ...txplan.Option) (O, error) {
	plan, err := Plan(view)
	if err != nil {
		return *new(O), err
	}
	client := view.Client()
	tx := txplan.NewPlannedTx(
		plan,
		txplan.WithAgainstLatestBlock(client),
		txplan.WithReader(client),
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
