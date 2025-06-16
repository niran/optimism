package sources

import (
	"context"
	"fmt"

	"github.com/ethereum-optimism/optimism/op-node/metrics"
	"github.com/ethereum-optimism/optimism/op-service/apis"
	"github.com/ethereum-optimism/optimism/op-service/client"
)

type InteropMonitorClient struct {
	client client.RPC
}

// This type-check keeps the Server API and Client API in sync.
var _ apis.InteropMonitorAPI = (*InteropMonitorClient)(nil)

func NewInteropMonitorClient(client client.RPC) *InteropMonitorClient {
	return &InteropMonitorClient{
		client: client,
	}
}

func (cl *InteropMonitorClient) Activity() apis.InteropMonitorActivity {
	return &InteropMonitorActivityClient{client: cl.client}
}

func (cl *InteropMonitorClient) Metrics() apis.InteropMonitorMetrics {
	return &InteropMonitorMetricsClient{client: cl.client}
}

func (cl *InteropMonitorClient) Close() {
	cl.client.Close()
}

// InteropMonitorActivityClient implements the InteropMonitorActivity interface
type InteropMonitorActivityClient struct {
	client client.RPC
}

var _ apis.InteropMonitorActivity = (*InteropMonitorActivityClient)(nil)

func (cl *InteropMonitorActivityClient) Start(ctx context.Context) error {
	err := cl.client.CallContext(ctx, nil, "admin_start")
	if err != nil {
		return fmt.Errorf("failed to start InteropMonitor: %w", err)
	}
	return nil
}

func (cl *InteropMonitorActivityClient) Stop(ctx context.Context) error {
	err := cl.client.CallContext(ctx, nil, "admin_stop")
	if err != nil {
		return fmt.Errorf("failed to stop InteropMonitor: %w", err)
	}
	return nil
}

// InteropMonitorMetricsClient implements the InteropMonitorMetrics interface
type InteropMonitorMetricsClient struct {
	client client.RPC
}

var _ apis.InteropMonitorMetrics = (*InteropMonitorMetricsClient)(nil)

func (cl *InteropMonitorMetricsClient) Metrics() metrics.Metricer {
	// For an RPC client, we return the noop metrics as the actual metrics
	// are exposed via HTTP endpoints on the server side for Prometheus scraping
	return metrics.NoopMetrics
}
