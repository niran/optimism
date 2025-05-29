package l2

import (
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
)

type FastCanonicalBlockHeaderOracle struct {
	head                    *types.Header
	blockByHashFn           func(common.Hash) *types.Block
	config                  *params.ChainConfig
	fallback                *CanonicalBlockHeaderOracle
	blockNumberSet          map[uint64]struct{}
	canonicalBlockHashes    map[uint64]common.Hash
	fetchedHistoricalProofs bool
	oracle                  Oracle
}

func NewFastCanonicalBlockHeaderOracle(
	head *types.Header,
	blockByHashFn func(common.Hash) *types.Block,
	chainCfg *params.ChainConfig,
	oracle Oracle,
	fallback *CanonicalBlockHeaderOracle,
	blockNumberSet map[uint64]struct{},
) *FastCanonicalBlockHeaderOracle {
	return &FastCanonicalBlockHeaderOracle{
		head:                    head,
		config:                  chainCfg,
		blockByHashFn:           blockByHashFn,
		fallback:                fallback,
		blockNumberSet:          blockNumberSet,
		oracle:                  oracle,
		fetchedHistoricalProofs: false,
		canonicalBlockHashes:    make(map[uint64]common.Hash),
	}
}

func (o *FastCanonicalBlockHeaderOracle) CurrentHeader() *types.Header {
	return o.head
}

func (o *FastCanonicalBlockHeaderOracle) fetchHistoricalBlockProofs() {
	o.fetchedHistoricalProofs = true
	chainID := eth.ChainIDFromBig(o.config.ChainID)
	blockNums := make([]uint64, 0, len(o.blockNumberSet))
	for n := range o.blockNumberSet {
		blockNums = append(blockNums, n)
	}

	provenBlocks := o.oracle.BlockAncestorsByNumbers(eth.BlockID{
		Hash:   o.head.Hash(),
		Number: o.head.Number.Uint64(),
	}, blockNums, chainID)

	o.canonicalBlockHashes = provenBlocks
}

func (o *FastCanonicalBlockHeaderOracle) GetHeaderByNumber(n uint64) *types.Header {
	if !o.fetchedHistoricalProofs {
		o.fetchHistoricalBlockProofs()
	}

	if o.head.Number.Uint64() < n {
		return nil
	}
	if o.head.Number.Uint64() == n {
		return o.head
	}
	if _, ok := o.blockNumberSet[n]; !ok {
		// If the block number is not in the set, we cannot serve it from the cache
		return o.fallback.GetHeaderByNumber(n)
	}

	h := o.head
	if !o.config.IsIsthmus(h.Time) {
		return o.fallback.GetHeaderByNumber(n)
	}

	blockHash, ok := o.canonicalBlockHashes[n]
	if !ok {
		return o.fallback.GetHeaderByNumber(n)
	}

	block := o.blockByHashFn(blockHash)

	return block.Header()
}

func (o *FastCanonicalBlockHeaderOracle) SetCanonical(head *types.Header) common.Hash {
	o.head = head
	o.fallback.SetCanonical(head)
	o.fetchedHistoricalProofs = false
	o.canonicalBlockHashes = make(map[uint64]common.Hash)
	return head.Hash()
}
