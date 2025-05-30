package eth

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb/memorydb"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/trie"
)

type StorageProofEntry struct {
	Key   StorageKey      `json:"key"`
	Value hexutil.Big     `json:"value"`
	Proof []hexutil.Bytes `json:"proof"`
}

type AccountResult struct {
	AccountProof []hexutil.Bytes `json:"accountProof"`

	Address     common.Address `json:"address"`
	Balance     *hexutil.Big   `json:"balance"`
	CodeHash    common.Hash    `json:"codeHash"`
	Nonce       hexutil.Uint64 `json:"nonce"`
	StorageHash common.Hash    `json:"storageHash"`

	// Optional
	StorageProof []StorageProofEntry `json:"storageProof,omitempty"`
}

// Verify an account (and optionally storage) proof from the getProof RPC. See https://eips.ethereum.org/EIPS/eip-1186
func (res *AccountResult) Verify(stateRoot common.Hash) error {
	// verify storage proof values, if any, against the storage trie root hash of the account
	for i, entry := range res.StorageProof {
		// load all MPT nodes into a DB
		db := memorydb.New()
		for j, encodedNode := range entry.Proof {
			nodeKey := encodedNode
			if len(encodedNode) >= 32 { // small MPT nodes are not hashed
				nodeKey = crypto.Keccak256(encodedNode)
			}
			if err := db.Put(nodeKey, encodedNode); err != nil {
				return fmt.Errorf("failed to load storage proof node %d of storage value %d into mem db: %w", j, i, err)
			}
		}
		path := crypto.Keccak256(entry.Key)
		val, err := trie.VerifyProof(res.StorageHash, path, db)
		if err != nil {
			return fmt.Errorf("failed to verify storage value %d with key %s (path %x) in storage trie %s: %w", i, entry.Key.String(), path, res.StorageHash, err)
		}
		if val == nil && entry.Value.ToInt().Cmp(common.Big0) == 0 { // empty storage is zero by default
			continue
		}
		comparison, err := rlp.EncodeToBytes(entry.Value.ToInt().Bytes())
		if err != nil {
			return fmt.Errorf("failed to encode storage value %d with key %s (path %x) in storage trie %s: %w", i, entry.Key.String(), path, res.StorageHash, err)
		}
		if !bytes.Equal(val, comparison) {
			return fmt.Errorf("value %d in storage proof does not match proven value at key %s (path %x)", i, entry.Key.String(), path)
		}
	}

	accountClaimed := []any{uint64(res.Nonce), res.Balance.ToInt().Bytes(), res.StorageHash, res.CodeHash}
	accountClaimedValue, err := rlp.EncodeToBytes(accountClaimed)
	if err != nil {
		return fmt.Errorf("failed to encode account from retrieved values: %w", err)
	}

	// create a db with all account trie nodes
	db := memorydb.New()
	for i, encodedNode := range res.AccountProof {
		nodeKey := encodedNode
		if len(encodedNode) >= 32 { // small MPT nodes are not hashed
			nodeKey = crypto.Keccak256(encodedNode)
		}
		if err := db.Put(nodeKey, encodedNode); err != nil {
			return fmt.Errorf("failed to load account proof node %d into mem db: %w", i, err)
		}
	}
	path := crypto.Keccak256(res.Address[:])
	accountProofValue, err := trie.VerifyProof(stateRoot, path, db)
	if err != nil {
		return fmt.Errorf("failed to verify account value with key %s (path %x) in account trie %s: %w", res.Address, path, stateRoot, err)
	}

	if !bytes.Equal(accountClaimedValue, accountProofValue) {
		return fmt.Errorf("L1 RPC is tricking us, account proof does not match provided deserialized values:\n"+
			"  claimed: %x\n"+
			"  proof:   %x", accountClaimedValue, accountProofValue)
	}
	return err
}

type ProofParams struct {
	BlockHash common.Hash
	Address   common.Address
	Storage   []common.Hash
}

var (
	ErrInvalidProof              = errors.New("failed to verify historical block proof")
	ErrInvalidStorageHistoryHash = errors.New(
		"invalid storage history hash, expected to be a valid historical block hash",
	)
)

// CheckProofChain verifies a chain of account proofs and returns a map of block numbers to their hashes
// using EIP-2935 block hash history contracts to jump 8190 blocks back in history.
func CheckProofChain(accountProofs map[common.Hash]AccountResult, chainHead BlockID, fetchStateRoot func(common.Hash) common.Hash) (map[uint64]common.Hash, error) {
	provenBlockHashes := make(map[uint64]common.Hash)

	// verify all the account proofs are valid
	for blockHash, result := range accountProofs {
		if result.Verify(fetchStateRoot(blockHash)) != nil {
			return nil, ErrInvalidProof
		}
	}

	currBlockHash := chainHead.Hash
	currBlockNum := chainHead.Number

	provenBlockHashes[currBlockNum] = currBlockHash

	// starting with the first block, we will iterate backwards through the history serve window
	for {
		currAccountProof := accountProofs[currBlockHash]
		currBlockStorageIdx := currBlockNum % params.HistoryServeWindow
		earliestBlockStorageIdx := (currBlockStorageIdx + 1) % params.HistoryServeWindow
		earliestBlockNum := currBlockNum - (params.HistoryServeWindow - 1)

		storageKeys := make(map[common.Hash]common.Hash)
		for _, proof := range currAccountProof.StorageProof {
			key := common.BytesToHash(common.LeftPadBytes(proof.Key[:], 32))
			// big endian
			storageKeys[key] = common.Hash(common.LeftPadBytes(proof.Value.ToInt().Bytes(), 32))
		}

		for historyIdxKey, historyBlockHash := range storageKeys {
			historyIdx := historyIdxKey.Big().Uint64()
			blockNum := ((historyIdx + params.HistoryServeWindow) - earliestBlockStorageIdx) + earliestBlockNum
			blockHash := common.BigToHash(historyBlockHash.Big())

			if blockHash == (common.Hash{}) {
				return nil, fmt.Errorf(
					"block %d: %w",
					blockNum,
					ErrInvalidStorageHistoryHash,
				)
			}

			provenBlockHashes[blockNum] = blockHash
		}

		// next block is the storage slot of the earliest block in the history serve window
		nextBlockHash, ok := storageKeys[common.BigToHash(big.NewInt(int64(earliestBlockStorageIdx)))]
		if !ok {
			break
		}
		if nextBlockHash == (common.Hash{}) {
			// If we don't have a next block hash, we cannot serve historical blocks
			return nil, fmt.Errorf("block %d: %w",
				earliestBlockNum, ErrInvalidStorageHistoryHash)
		}

		currBlockNum = earliestBlockNum
		currBlockHash = nextBlockHash
	}

	return provenBlockHashes, nil
}
