package apis

import (
	"context"

	"github.com/ethereum/go-ethereum/common/hexutil"
)

type ProposerActivity interface {
	StartProposer(ctx context.Context) error
	StopProposer(ctx context.Context) error
	// ProposeOutput submits the output for the given block number. If no block is provided, the latest synced block is used.
	ProposeOutput(ctx context.Context, blockNum *hexutil.Uint64) error
}

type ProposerAdminServer interface {
	CommonAdminServer
	ProposerActivity
}

type ProposerAdminClient interface {
	CommonAdminClient
	ProposerActivity
}
