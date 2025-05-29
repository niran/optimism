package l2

import (
	"math/big"

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

	accountProofs := o.oracle.BlockAncestorsByNumbers(o.head.Hash(), blockNums, chainID)
	stateRoots := make(map[common.Hash]common.Hash, len(accountProofs))
	for blockHash := range accountProofs {
		block := o.oracle.BlockByHash(blockHash, chainID)
		stateRoots[blockHash] = block.Root()
	}

	// verify all the account proofs are valid
	for blockHash, result := range accountProofs {
		if result.Verify(stateRoots[blockHash]) != nil {
			panic("failed to verify historical block proof")
		}
	}

	currBlockHash := o.head.Hash()
	currBlockNum := o.head.Number.Uint64()

	o.canonicalBlockHashes[currBlockNum] = currBlockHash

	// verify the merkle proofs indicate the correct block hashes, and store the historical blocks
	for {
		firstAccountProof := accountProofs[currBlockHash]
		currentBlockStorageIdx := currBlockNum % params.HistoryServeWindow
		earliestBlockStorageIdx := (currentBlockStorageIdx + 1) % params.HistoryServeWindow
		earliestBlockNum := currBlockNum - (params.HistoryServeWindow - 1)

		storageKeys := make(map[common.Hash]common.Hash)
		for _, proof := range firstAccountProof.StorageProof {
			key := common.BytesToHash(common.LeftPadBytes(proof.Key[:], 32))
			// big endian
			storageKeys[key] = common.Hash(proof.Value.ToInt().Bytes())
		}

		for historyIdxKey, historyBlockHash := range storageKeys {
			historyIdx := historyIdxKey.Big().Uint64()
			blockNum := ((historyIdx + params.HistoryServeWindow) - earliestBlockStorageIdx) + earliestBlockNum
			blockHash := common.BigToHash(historyBlockHash.Big())

			if blockHash == (common.Hash{}) {
				panic("missing historical block hash in storage keys")
			}

			o.canonicalBlockHashes[blockNum] = blockHash
		}

		// next block is the storage slot of the earliest block in the history serve window
		nextBlockHash, ok := storageKeys[common.BigToHash(big.NewInt(int64(earliestBlockStorageIdx)))]
		if !ok {
			break
		}
		if nextBlockHash == (common.Hash{}) {
			// If we don't have a next block hash, we cannot serve historical blocks
			return
		}

		currBlockNum = earliestBlockNum
		currBlockHash = nextBlockHash
	}

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
