package schedule

import (
	"context"
	"sync/atomic"
	"time"
)

type Metrics struct {
	Submitted uint64
	Failed    uint64
}

type Schedule interface {
	// Start runs the schedule. It is blocking.
	Start(context.Context)
	// Ready returns a channel that fires on schedule.
	Ready() <-chan struct{}
	// Adjust provides the latest metrics for feedback-based schedules.
	Adjust(Metrics)
}

type Constant struct {
	rate  time.Duration
	ready chan struct{}
}

var _ Schedule = (*Constant)(nil)

func NewConstant(rate time.Duration) *Constant {
	return &Constant{
		rate:  rate,
		ready: make(chan struct{}),
	}
}

func (c *Constant) Start(ctx context.Context) {
	defer close(c.ready)
	ticker := time.NewTicker(c.rate)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			select {
			case c.ready <- struct{}{}:
			default: // Skip if readers are not ready.
			}
		}
	}
}

func (c *Constant) Ready() <-chan struct{} {
	return c.ready
}

func (c *Constant) Adjust(Metrics) {}

// AIMD scheduler (additive-increase, multiplicative-decrease).
type AIMD struct {
	rps               atomic.Uint64
	maxRPS            uint64
	addRPS            uint64  // additive delta
	multRPS           float64 // multiplicative factor (<1.0)
	failRateThreshold float64 // when to start decreasing (e.g., 0.05 of all requests are failures)
	slot              time.Duration
	ready             chan struct{}
}

var _ Schedule = (*AIMD)(nil)

func NewAIMD(baseRPS, maxRPS, addRPS uint64, multRPS float64, failRate float64, slot time.Duration) *AIMD {
	c := &AIMD{
		maxRPS:  maxRPS,
		addRPS:  addRPS,
		multRPS: multRPS,
		ready:   make(chan struct{}),
		slot:    slot,
	}
	c.rps.Store(baseRPS)
	return c
}

func (c *AIMD) Start(ctx context.Context) {
	defer close(c.ready)
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(c.slot / time.Duration(c.rps.Load())):
			select {
			case c.ready <- struct{}{}:
			default: // Skip if readers are not ready.
			}
		}
	}
}

func (c *AIMD) Adjust(m Metrics) {
	failRate := float64(m.Failed) / float64(m.Submitted+1)
	if failRate > c.failRateThreshold {
		newRPS := max(uint64(float64(c.rps.Load())*c.multRPS), 1)
		c.rps.Store(newRPS)
	} else {
		newRPS := min(c.rps.Load()+c.addRPS, c.maxRPS)
		c.rps.Store(newRPS)
	}
}

func (c *AIMD) Ready() <-chan struct{} {
	return c.ready
}
