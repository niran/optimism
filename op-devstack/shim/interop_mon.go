package shim

import (
	"fmt"

	"github.com/ethereum-optimism/optimism/op-devstack/stack"
	promAPI "github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
)

type InteropMonitorConfig struct {
	CommonConfig
	ID              stack.InteropMonitorID
	MetricsEndpoint string
}

type rpcInteropMonitor struct {
	id stack.InteropMonitorID
	commonImpl
	v1.API
}

func (r *rpcInteropMonitor) ID() stack.InteropMonitorID {
	return r.id
}

var _ stack.InteropMonitor = (*rpcInteropMonitor)(nil)

func NewInteropMonitor(cfg InteropMonitorConfig) stack.InteropMonitor {
	cfg.T = cfg.T.WithCtx(stack.ContextWithID(cfg.T.Ctx(), cfg.ID))
	client, err := promAPI.NewClient(promAPI.Config{
		Address: cfg.MetricsEndpoint,
	})
	if err != nil {
		panic(fmt.Errorf("failed to create prometheus client at %s: %w", cfg.MetricsEndpoint, err))
	}
	metrics := v1.NewAPI(client)

	return &rpcInteropMonitor{
		commonImpl: newCommon(cfg.CommonConfig),
		id:         cfg.ID,
		API:        metrics,
	}
}
