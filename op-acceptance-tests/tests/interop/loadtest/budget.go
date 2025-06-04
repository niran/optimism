package loadtest

import (
	"context"
	"fmt"
	"sync"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/plan"
	"github.com/ethereum-optimism/optimism/op-service/txplan"
)

type InsufficientBudgetError struct {
	Remaining eth.ETH
	Requested eth.ETH
}

var _ error = (*InsufficientBudgetError)(nil)

func (e *InsufficientBudgetError) Error() string {
	return fmt.Sprintf("insufficient budget: requested %s, remaining %s", e.Requested, e.Remaining)
}

// Budget tracks the amount of ETH spent and begins returning errors when it would exceed a certain amount.
//
// Limitation: Budget assumes that all submitted transactions are included forever.
type Budget struct {
	mu        sync.Mutex
	remaining eth.ETH
}

func NewBudget(amount eth.ETH) *Budget {
	return &Budget{
		remaining: amount,
	}
}

func (b *Budget) Plan() txplan.Option {
	return b.onSubmit()
}

func (b *Budget) onSubmit() txplan.Option {
	return func(tx *txplan.PlannedTx) {
		tx.Submitted.DependOn(&tx.Signed)
		tx.Submitted.Wrap(func(inner plan.Fn[struct{}]) plan.Fn[struct{}] {
			return func(ctx context.Context) (struct{}, error) {
				cost := eth.WeiBig(tx.Signed.Value().Cost())
				if err := b.debit(cost); err != nil {
					return struct{}{}, err
				}
				// Assumption: if inner fails, then the transaction will never be included.
				if _, err := inner(ctx); err != nil {
					b.credit(cost)
					return struct{}{}, err
				}
				return struct{}{}, nil
			}
		})
	}
}

func (b *Budget) debit(amount eth.ETH) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	result, underflow := b.remaining.SubUnderflow(amount)
	if underflow {
		return &InsufficientBudgetError{
			Remaining: b.remaining,
			Requested: amount,
		}
	}
	b.remaining = result
	return nil
}

// credit does not check for underflows or overflows.
func (b *Budget) credit(amount eth.ETH) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.remaining = b.remaining.Add(amount)
}
