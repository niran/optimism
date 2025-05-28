package loadtest

import (
	"math"

	"github.com/ethereum-optimism/optimism/op-service/retry"
	"github.com/ethereum-optimism/optimism/op-service/txplan"
)

func retryInclusionForever(g txplan.ReceiptGetter) txplan.Option {
	return txplan.WithRetryInclusion(g, math.MaxInt, retry.Exponential())
}

func retrySubmissionForever(s txplan.TransactionSubmitter) txplan.Option {
	return txplan.WithRetrySubmission(s, math.MaxInt, retry.Exponential())
}
