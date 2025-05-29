package prefetcher

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"slices"

	"github.com/ethereum-optimism/optimism/op-program/client/l2"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/predeploys"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/params"
)

var (
	ErrBlockNumberPastParent = errors.New("block number is past parent block")
)

func (p *Prefetcher) fetchBlockNumberProofs(ctx context.Context, hint l2.BlockAncestorHint) error {
	chainID := hint.ChainID
	blockNumbers := hint.BlockNumbers
	parent := hint.FromBlockHash

	l2Source, err := p.l2Sources.ForChainID(chainID)
	if err != nil {
		return err
	}

	blockInfo, _, err := l2Source.InfoAndTxsByHash(ctx, parent)
	if err != nil {
		return err
	}

	// Fetch proof of block-8191 and any blocks needed in the history serve window
	currentBlockNum := blockInfo.NumberU64()
	currentHistoryStorageIdx := currentBlockNum % params.HistoryServeWindow

	lastHistoryStorageIdx := (currentHistoryStorageIdx + 1) % params.HistoryServeWindow
	lastHistoryStorageBlockNum := currentBlockNum - (params.HistoryServeWindow - 1)

	keysToFetchPerBlockHash := make(map[common.Hash][]common.Hash)
	slices.SortFunc(blockNumbers, func(a, b uint64) int {
		return int(b) - int(a)
	})

	currHead := parent

	// find all keys in the blockNumberSet that are in the history serve window
	keysToFetch := make([]common.Hash, 0, len(blockNumbers))
	for _, n := range blockNumbers {
		if n > currentBlockNum {
			return ErrBlockNumberPastParent
		}

		if n < lastHistoryStorageBlockNum {
			// we're past the history serve window, so store what we need to fetch for this batch
			// add the last history storage index which will be the first key to fetch
			var key common.Hash
			binary.BigEndian.PutUint64(key[:], n%params.HistoryServeWindow)
			keysToFetch = append(keysToFetch, key)

			keysToFetch = append(keysToFetch, key)
			keysToFetchPerBlockHash[currHead] = keysToFetch
			keysToFetch = nil

			nextBlockInfo, err := l2Source.InfoByNumber(ctx, lastHistoryStorageBlockNum)
			if err != nil {
				return err
			}

			currHead = nextBlockInfo.Hash()
			lastHistoryStorageIdx = (lastHistoryStorageIdx + 1) % params.HistoryServeWindow
			lastHistoryStorageBlockNum = nextBlockInfo.NumberU64() - (params.HistoryServeWindow - 1)
			currentBlockNum = nextBlockInfo.NumberU64()
			currentHistoryStorageIdx = currentBlockNum % params.HistoryServeWindow
			continue
		}

		if n != lastHistoryStorageBlockNum && n != currentBlockNum {
			// if the block number is not the last history storage block number or the current block number,
			// we need to fetch the key for this block number
			var key common.Hash
			binary.BigEndian.PutUint64(key[:], n%params.HistoryServeWindow)
			keysToFetch = append(keysToFetch, key)
		}

	}

	keysToFetchPerBlockHash[currHead] = keysToFetch

	// batch get proof for all keys to fetch
	batchRpcs := make([]eth.ProofParams, 0, len(keysToFetchPerBlockHash))
	for blockHash, keys := range keysToFetchPerBlockHash {
		batchRpcs = append(batchRpcs, eth.ProofParams{
			Address:   predeploys.EIP2935ContractAddr,
			Storage:   keys,
			BlockHash: blockHash,
		})
	}

	proofs, err := l2Source.BatchGetProofs(ctx, batchRpcs)
	if err != nil {
		return err
	}

	// TODO: should we store individual keys here?
	proofsJson, err := json.Marshal(proofs)
	if err != nil {
		return err
	}

	return p.kvStore.Put(hint.Hash(), proofsJson)
}
