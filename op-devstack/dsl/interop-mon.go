package dsl

import (
	"context"
	"time"

	"github.com/ethereum-optimism/optimism/op-devstack/stack"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

type InteropMonitor struct {
	commonImpl
	inner stack.InteropMonitor
}

func NewInteropMonitor(inner stack.InteropMonitor) *InteropMonitor {
	return &InteropMonitor{
		commonImpl: commonFromT(inner.T()),
		inner:      inner,
	}
}

func (i *InteropMonitor) Escape() stack.InteropMonitor {
	return i.inner
}

func (i *InteropMonitor) Query(ctx context.Context, query string, ts time.Time) (model.Value, v1.Warnings, error) {
	return i.inner.Query(ctx, query, ts)
}
