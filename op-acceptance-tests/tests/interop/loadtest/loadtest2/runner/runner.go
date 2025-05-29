package loadtest2

import (
	"context"
	"sync"

	"github.com/ethereum-optimism/optimism/op-acceptance-tests/tests/interop/loadtest/loadtest2/schedule"
)

type Sender interface {
	SendTx(ctx context.Context) bool
}

type Runner struct {
	sched         schedule.Schedule
	metrics       schedule.Metrics
	metricsMu     sync.Mutex
	sender        Sender
	numSubmitters uint
}

func NewRunner(sched schedule.Schedule, prod Sender, numSubmitters uint) *Runner {
	return &Runner{
		sched:         sched,
		sender:        prod,
		numSubmitters: numSubmitters,
	}
}

func (r *Runner) Start(ctx context.Context) {
	var wg sync.WaitGroup
	defer wg.Wait()

	wg.Add(1)
	go func() {
		defer wg.Done()
		r.sched.Start(ctx)
	}()

	// Send transactions on schedule, adjusting the schedule based on the result.
	// Because waiting for inclusion takes a while even in the happy path, we use
	// r.numSubmitters worker goroutines.
	for range r.numSubmitters {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range r.sched.Ready() {
				r.sendAndAdjust(ctx)
			}
		}()
	}
}

func (r *Runner) sendAndAdjust(ctx context.Context) {
	success := r.sender.SendTx(ctx)
	r.metricsMu.Lock()
	defer r.metricsMu.Unlock()
	r.metrics.Submitted++
	if !success {
		r.metrics.Failed++
	}
	if r.metrics.Submitted == 50 { // Adjust the schedule every 50 txs.
		r.sched.Adjust(r.metrics)
		r.metrics = schedule.Metrics{}
	}
}

// TODO track how many txs that are pending
// TODO need to adjsut the gas ratio
