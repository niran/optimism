package apis

import (
	"context"

	"github.com/ethereum-optimism/optimism/op-node/metrics"
)

type InteropMonitorActivity interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

type InteropMonitorMetrics interface {
	Metrics() metrics.Metricer
}

type InteropMonitorAPI interface {
	Activity() InteropMonitorActivity
	Metrics() InteropMonitorMetrics
}
