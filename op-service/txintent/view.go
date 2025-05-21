package txintent

import (
	"context"

	"github.com/ethereum-optimism/optimism/op-devstack/devtest"
	"github.com/ethereum-optimism/optimism/op-service/apis"
	"github.com/ethereum-optimism/optimism/op-service/txplan"
	"github.com/ethereum/go-ethereum/common"
)

type Output[O any] interface {
	DecodeOutput(data []byte) (dest O, err error)
}

type View[O any] interface {
	Call
	Client() apis.EthClient
	Output[O]
}

type TestView[O any] interface {
	View[O]
	Test() devtest.T
}

func Read[O any](view View[O], opts ...txplan.Option) (O, error) {
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

	// fixme for context
	res, err := tx.Read.Eval(context.Background())
	if err != nil {
		return *new(O), err
	}
	decoded, err := view.DecodeOutput(res)
	if err != nil {
		return *new(O), err
	}
	return decoded, nil
}
