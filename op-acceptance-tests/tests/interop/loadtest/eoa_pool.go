package loadtest

import (
	"sync"
	"sync/atomic"

	"github.com/ethereum-optimism/optimism/op-devstack/dsl"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/txplan"
)

type syncEOA struct {
	inner        *dsl.EOA
	nonceManager *NonceManager
}

func (eoa *syncEOA) Plan() txplan.Option {
	return txplan.Combine(eoa.inner.Plan(), eoa.nonceManager.Plan())
}

type EOAPool struct {
	eoas  []*syncEOA
	index atomic.Uint64
}

func NewEOAPool(funder *dsl.Funder, total eth.ETH) *EOAPool {
	eoas := make([]*syncEOA, 300)
	amountPerEOA := total.Div(uint64(len(eoas)))
	var wg sync.WaitGroup
	defer wg.Wait()
	for i := range len(eoas) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			eoas[i] = &syncEOA{
				inner:        funder.NewFundedEOA(amountPerEOA),
				nonceManager: NewNonceManager(0),
			}
		}()
	}
	return &EOAPool{
		eoas: eoas,
	}
}

func (p *EOAPool) Plan() txplan.Option {
	next := (p.index.Add(1) - 1) % uint64(len(p.eoas))
	return p.eoas[next].Plan()
}
