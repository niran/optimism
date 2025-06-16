package stack

import (
	"log/slog"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
)

type InteropMonitor interface {
	Common
	v1.API
	ID() InteropMonitorID
}
type InteropMonitorID string

func (id InteropMonitorID) LogValue() slog.Value {
	return slog.StringValue(string(id))
}
