package shim

import (
	"context"

	"github.com/ethereum-optimism/optimism/op-devstack/stack"
	"github.com/ethereum-optimism/optimism/op-service/apis"
	"github.com/ethereum-optimism/optimism/op-service/client"
	"github.com/ethereum-optimism/optimism/op-service/sources"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
)

type InteropMonitorConfig struct {
	CommonConfig
	ID     stack.InteropMonitorID
	Client client.RPC
}

type rpcInteropMonitor struct {
	commonImpl
	id      stack.InteropMonitorID
	api     apis.InteropMonitorAPI
	metrics v1.API
}

var _ stack.InteropMonitor = (*rpcInteropMonitor)(nil)

func NewInteropMonitor(cfg InteropMonitorConfig) stack.InteropMonitor {
	cfg.T = cfg.T.WithCtx(stack.ContextWithID(cfg.T.Ctx(), cfg.ID))
	return &rpcInteropMonitor{
		commonImpl: newCommon(cfg.CommonConfig),
		id:         cfg.ID,
		client:     cfg.Client,
		api:        sources.NewInteropMonitorClient(cfg.Client),
	}
}

func (r *rpcInteropMonitor) ID() stack.InteropMonitorID {
	return r.id
}

func (r *rpcInteropMonitor) Start(ctx context.Context) error {
	return r.api.Activity().Start(ctx)
}

func (r *rpcInteropMonitor) Stop(ctx context.Context) error {
	return r.api.Activity().Stop(ctx)
}

// Metrics returns the metrics interface for Prometheus scraping
func (r *rpcInteropMonitor) Metrics() apis.InteropMonitorMetrics {
	return r.api.Metrics()
}
