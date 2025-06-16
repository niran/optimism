package stack

import (
	"log/slog"

	"github.com/ethereum-optimism/optimism/op-service/apis"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
)

type InteropMonitor interface {
	Common
	ID() InteropMonitorID
	ActivityAPI() apis.InteropMonitorActivity
	Metrics() v1.API
}
type InteropMonitorID string

func (id InteropMonitorID) LogValue() slog.Value {
	return slog.StringValue(string(id))
}
