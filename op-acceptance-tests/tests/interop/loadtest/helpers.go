package loadtest

import (
	"math"

	"github.com/ethereum-optimism/optimism/op-service/retry"
	"github.com/ethereum-optimism/optimism/op-service/txplan"
)

func retryForever(g txplan.ReceiptGetter) txplan.Option {
	return txplan.WithRetryInclusion(g, math.MaxInt, retry.Exponential())
}
