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
	// rps can be thought of to mean "requests per slot", although the unit and quantity are flexible.
	rps    atomic.Uint64
	maxRPS uint64

	increaseDelta  uint64  // additive delta
	decreaseFactor float64 // multiplicative factor

	failRateThreshold float64 // when to start decreasing (e.g., 0.05 of all requests are failures)

	slotTime time.Duration
	ready    chan struct{}
}

var _ Schedule = (*AIMD)(nil)

func NewAIMD(baseRPS uint64, slotTime time.Duration, opts ...AIMDOption) *AIMD {
	aimd := &AIMD{
		maxRPS:            10 * baseRPS,
		increaseDelta:     max(baseRPS/10, 1),
		decreaseFactor:    0.5,
		failRateThreshold: 0.05,
		ready:             make(chan struct{}),
		slotTime:          slotTime,
	}
	aimd.rps.Store(baseRPS)
	for _, opt := range opts {
		opt(aimd)
	}
	return aimd
}

type AIMDOption func(*AIMD)

func WithMaxRPS(maxRPS uint64) AIMDOption {
	return func(aimd *AIMD) {
		aimd.maxRPS = maxRPS
	}
}

func WithIncreaseDelta(delta uint64) AIMDOption {
	return func(aimd *AIMD) {
		aimd.increaseDelta = delta
	}
}

func WithDecreaseFactor(factor float64) AIMDOption {
	return func(aimd *AIMD) {
		aimd.decreaseFactor = factor
	}
}

func WithFailRateThreshold(threshold float64) AIMDOption {
	return func(aimd *AIMD) {
		aimd.failRateThreshold = threshold
	}
}

func (c *AIMD) Start(ctx context.Context) {
	defer close(c.ready)
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(c.slotTime / time.Duration(c.rps.Load())):
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
		newRPS := max(uint64(float64(c.rps.Load())*c.decreaseFactor), 1)
		c.rps.Store(newRPS)
	} else {
		newRPS := min(c.rps.Load()+c.increaseDelta, c.maxRPS)
		c.rps.Store(newRPS)
	}
}

func (c *AIMD) Ready() <-chan struct{} {
	return c.ready
}
