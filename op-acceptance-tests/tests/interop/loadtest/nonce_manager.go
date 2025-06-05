package loadtest

import (
	"context"
	"slices"
	"sync"

	"github.com/ethereum-optimism/optimism/op-service/plan"
	"github.com/ethereum-optimism/optimism/op-service/txplan"
)

// NonceManager tracks nonces for an account and handles gaps in the nonce sequence.
// When transactions fail to be included, their nonces are will be used preferentially
// for future transactions.
//
// Limitation: NonceManager assumes that all submitted transactions are included forever.
type NonceManager struct {
	mu        sync.Mutex
	nextNonce uint64
	gaps      []uint64 // sorted list of nonce gaps
}

// NewNonceManager creates a new nonce manager starting at the given nonce.
func NewNonceManager(startNonce uint64) *NonceManager {
	return &NonceManager{
		nextNonce: startNonce,
		gaps:      make([]uint64, 0),
	}
}

func (nm *NonceManager) Plan() txplan.Option {
	return txplan.Combine(nm.setNonce, nm.onSubmit)
}

func (nm *NonceManager) setNonce(tx *txplan.PlannedTx) {
	tx.Nonce.Fn(func(ctx context.Context) (uint64, error) {
		nm.mu.Lock()
		defer nm.mu.Unlock()
		if len(nm.gaps) > 0 {
			nonce := nm.gaps[0]
			nm.gaps = nm.gaps[1:]
			return nonce, nil
		}
		nonce := nm.nextNonce
		nm.nextNonce++
		return nonce, nil
	})
}

func (nm *NonceManager) onSubmit(tx *txplan.PlannedTx) {
	tx.Submitted.DependOn(&tx.Nonce)
	tx.Submitted.Wrap(func(inner plan.Fn[struct{}]) plan.Fn[struct{}] {
		return func(ctx context.Context) (struct{}, error) {
			result, err := inner(ctx)
			if err != nil {
				nm.insertGap(tx.Nonce.Value())
			}
			return result, err
		}
	})
}

func (nm *NonceManager) insertGap(nonce uint64) {
	nm.mu.Lock()
	defer nm.mu.Unlock()
	i, _ := slices.BinarySearch(nm.gaps, nonce)
	nm.gaps = slices.Insert(nm.gaps, i, nonce)
}
