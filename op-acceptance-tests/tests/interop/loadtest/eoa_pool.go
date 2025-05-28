package loadtest

import (
	"sync"
	"sync/atomic"

	"github.com/ethereum-optimism/optimism/op-devstack/devtest"
	"github.com/ethereum-optimism/optimism/op-devstack/dsl"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	opbindings "github.com/ethereum-optimism/optimism/op-service/txintent/bindings"
	"github.com/ethereum-optimism/optimism/op-service/txintent/contractio"
	"github.com/ethereum-optimism/optimism/op-service/txplan"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
)

type SyncEOA struct {
	inner *dsl.EOA
	nonce atomic.Uint64
}

func NewSyncEOA(inner *dsl.EOA) *SyncEOA {
	return &SyncEOA{
		inner: inner,
	}
}

func (s *SyncEOA) SetNonce(nonce uint64) {
	s.nonce.Store(nonce)
}

func (eoa *SyncEOA) PlanOnRejection() txplan.Option {
	nonce := eoa.nonce.Load()
	return txplan.Combine(eoa.inner.Plan(), txplan.WithStaticNonce(nonce))
}

func (eoa *SyncEOA) PlanOnInclusion() txplan.Option {
	nonce := eoa.nonce.Add(1) - 1
	return txplan.Combine(eoa.inner.Plan(), txplan.WithStaticNonce(nonce)) // TODO retry submission and inclusion?
}

type EOAPool struct {
	queueMu sync.Mutex
	queue   []*SyncEOA
}

func NewEOAPool(t devtest.T, numEOAs int, amount eth.ETH, master *dsl.EOA, wallet *dsl.HDWallet, faucetAddress common.Address, el *dsl.L2ELNode) *EOAPool {
	t.Require().Positive(numEOAs)
	faucet := opbindings.NewFaucet(opbindings.NewFaucetFactory(
		opbindings.WithTo(faucetAddress),
		opbindings.WithTest(t),
		opbindings.WithClient(el.Escape().EthClient()),
	))

	syncMaster := NewSyncEOA(master)
	masterNonce, err := el.Escape().EthClient().NonceAt(t.Ctx(), master.Address(), nil)
	t.Require().NoError(err)
	syncMaster.SetNonce(masterNonce)

	eoas := make([]*SyncEOA, 0, numEOAs)
	for range numEOAs {
		eoas = append(eoas, NewSyncEOA(wallet.NewEOA(el)))
	}

	b, err := el.Escape().EthClient().InfoByLabel(t.Ctx(), eth.Unsafe)
	t.Require().NoError(err)
	maxRecipientsPerTx := int(b.GasLimit() / 40_000) // This magic number was determined empirically against a gas limit of 60M.

	var wg sync.WaitGroup
	defer wg.Wait()
	for start, end := 0, 0; end < len(eoas); {
		start = end
		end += maxRecipientsPerTx
		if end > len(eoas) {
			end = len(eoas)
		}

		eoaBatch := eoas[start:end]
		addresses := make([]common.Address, 0, len(eoaBatch))
		for _, eoa := range eoaBatch {
			addresses = append(addresses, eoa.inner.Address())
		}

		wg.Add(1)
		go func(addresses []common.Address) {
			defer wg.Done()
			receipt, err := contractio.Write(
				faucet.Fund(addresses, amount.ToBig()),
				t.Ctx(),
				syncMaster.PlanOnInclusion(),
				txplan.WithValue(amount.Mul(uint64(len(addresses))).ToBig()),
				retrySubmissionForever(el.Escape().EthClient()),
				retryInclusionForever(el.Escape().EthClient()),
			)
			t.Require().NoError(err)
			t.Require().Equal(ethtypes.ReceiptStatusSuccessful, receipt.Status)
		}(addresses)
	}

	return &EOAPool{
		queue: eoas,
	}
}

func (p *EOAPool) Borrow() *SyncEOA {
	p.queueMu.Lock()
	defer p.queueMu.Unlock()
	if len(p.queue) == 0 {
		return nil // Pool exhausted. Should this be a panic or Require?
	}
	eoa := p.queue[0]
	p.queue[0] = nil
	p.queue = p.queue[1:]
	return eoa
}

func (p *EOAPool) Return(eoa *SyncEOA) {
	p.queueMu.Lock()
	defer p.queueMu.Unlock()
	p.queue = append(p.queue, eoa) // Need to check if there is a more efficient way to handle the queue.
}
