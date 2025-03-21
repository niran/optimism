package derive

import (
	"bytes"
	"math/big"
	"math/rand"
	"testing"

	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/core/types"

	"github.com/ethereum-optimism/optimism/op-service/testutils"
)

type txTypeTest struct {
	name   string
	mkTx   func(rng *rand.Rand, signer types.Signer) *types.Transaction
	signer types.Signer
}

func TestSpanBatchTxsContractCreationBits(t *testing.T) {
	rng := rand.New(rand.NewSource(0x1234567))
	chainID := big.NewInt(rng.Int63n(1000))

	rawSpanBatch := RandomRawSpanBatch(rng, chainID)
	contractCreationBits := rawSpanBatch.Txs.ContractCreationBits
	totalBlockTxCount := rawSpanBatch.Txs.TotalBlockTxCount

	var sbt SpanBatchTxs
	sbt.ContractCreationBits = contractCreationBits
	sbt.TotalBlockTxCount = totalBlockTxCount

	var buf bytes.Buffer
	err := sbt.encodeContractCreationBits(&buf)
	require.NoError(t, err)

	// contractCreationBit field is fixed length: single bit
	contractCreationBitBufferLen := totalBlockTxCount / 8
	if totalBlockTxCount%8 != 0 {
		contractCreationBitBufferLen++
	}
	require.Equal(t, buf.Len(), int(contractCreationBitBufferLen))

	result := buf.Bytes()
	sbt.ContractCreationBits = nil

	r := bytes.NewReader(result)
	err = sbt.decodeContractCreationBits(r)
	require.NoError(t, err)

	require.Equal(t, contractCreationBits, sbt.ContractCreationBits)
}

func TestSpanBatchTxsContractCreationCount(t *testing.T) {
	rng := rand.New(rand.NewSource(0x1337))
	chainID := big.NewInt(rng.Int63n(1000))

	rawSpanBatch := RandomRawSpanBatch(rng, chainID)

	contractCreationBits := rawSpanBatch.Txs.ContractCreationBits
	contractCreationCount, err := rawSpanBatch.Txs.contractCreationCount()
	require.NoError(t, err)
	totalBlockTxCount := rawSpanBatch.Txs.TotalBlockTxCount

	var sbt SpanBatchTxs
	sbt.ContractCreationBits = contractCreationBits
	sbt.TotalBlockTxCount = totalBlockTxCount

	var buf bytes.Buffer
	err = sbt.encodeContractCreationBits(&buf)
	require.NoError(t, err)

	result := buf.Bytes()
	sbt.ContractCreationBits = nil

	r := bytes.NewReader(result)
	err = sbt.decodeContractCreationBits(r)
	require.NoError(t, err)

	contractCreationCount2, err := sbt.contractCreationCount()
	require.NoError(t, err)

	require.Equal(t, contractCreationCount, contractCreationCount2)
}

func TestSpanBatchTxsYParityBits(t *testing.T) {
	rng := rand.New(rand.NewSource(0x7331))
	chainID := big.NewInt(rng.Int63n(1000))

	rawSpanBatch := RandomRawSpanBatch(rng, chainID)
	yParityBits := rawSpanBatch.Txs.YParityBits
	totalBlockTxCount := rawSpanBatch.Txs.TotalBlockTxCount

	var sbt SpanBatchTxs
	sbt.YParityBits = yParityBits
	sbt.TotalBlockTxCount = totalBlockTxCount

	var buf bytes.Buffer
	err := sbt.encodeYParityBits(&buf)
	require.NoError(t, err)

	// yParityBit field is fixed length: single bit
	yParityBitBufferLen := totalBlockTxCount / 8
	if totalBlockTxCount%8 != 0 {
		yParityBitBufferLen++
	}
	require.Equal(t, buf.Len(), int(yParityBitBufferLen))

	result := buf.Bytes()
	sbt.YParityBits = nil

	r := bytes.NewReader(result)
	err = sbt.decodeYParityBits(r)
	require.NoError(t, err)

	require.Equal(t, yParityBits, sbt.YParityBits)
}

func TestSpanBatchTxsProtectedBits(t *testing.T) {
	rng := rand.New(rand.NewSource(0x7331))
	chainID := big.NewInt(rng.Int63n(1000))

	rawSpanBatch := RandomRawSpanBatch(rng, chainID)
	protectedBits := rawSpanBatch.Txs.ProtectedBits
	txTypes := rawSpanBatch.Txs.txTypes
	totalBlockTxCount := rawSpanBatch.Txs.TotalBlockTxCount
	totalLegacyTxCount := rawSpanBatch.Txs.totalLegacyTxCount

	var sbt SpanBatchTxs
	sbt.ProtectedBits = protectedBits
	sbt.TotalBlockTxCount = totalBlockTxCount
	sbt.txTypes = txTypes
	sbt.totalLegacyTxCount = totalLegacyTxCount

	var buf bytes.Buffer
	err := sbt.encodeProtectedBits(&buf)
	require.NoError(t, err)

	// protectedBit field is fixed length: single bit
	protectedBitBufferLen := totalLegacyTxCount / 8
	require.NoError(t, err)
	if totalLegacyTxCount%8 != 0 {
		protectedBitBufferLen++
	}
	require.Equal(t, buf.Len(), int(protectedBitBufferLen))

	result := buf.Bytes()
	sbt.ProtectedBits = nil

	r := bytes.NewReader(result)
	err = sbt.decodeProtectedBits(r)
	require.NoError(t, err)

	require.Equal(t, protectedBits, sbt.ProtectedBits)
}

func TestSpanBatchTxsTxSigs(t *testing.T) {
	rng := rand.New(rand.NewSource(0x73311337))
	chainID := big.NewInt(rng.Int63n(1000))

	rawSpanBatch := RandomRawSpanBatch(rng, chainID)
	txSigs := rawSpanBatch.Txs.TxSigs
	totalBlockTxCount := rawSpanBatch.Txs.TotalBlockTxCount

	var sbt SpanBatchTxs
	sbt.TotalBlockTxCount = totalBlockTxCount
	sbt.TxSigs = txSigs

	var buf bytes.Buffer
	err := sbt.encodeTxSigsRS(&buf)
	require.NoError(t, err)

	// txSig field is fixed length: 32 byte + 32 byte = 64 byte
	require.Equal(t, buf.Len(), 64*int(totalBlockTxCount))

	result := buf.Bytes()
	sbt.TxSigs = nil

	r := bytes.NewReader(result)
	err = sbt.decodeTxSigsRS(r)
	require.NoError(t, err)

	// v field is not set
	for i := 0; i < int(totalBlockTxCount); i++ {
		require.Equal(t, txSigs[i].r, sbt.TxSigs[i].r)
		require.Equal(t, txSigs[i].s, sbt.TxSigs[i].s)
	}
}

func TestSpanBatchTxsTxNonces(t *testing.T) {
	rng := rand.New(rand.NewSource(0x123456))
	chainID := big.NewInt(rng.Int63n(1000))

	rawSpanBatch := RandomRawSpanBatch(rng, chainID)
	txNonces := rawSpanBatch.Txs.TxNonces
	totalBlockTxCount := rawSpanBatch.Txs.TotalBlockTxCount

	var sbt SpanBatchTxs
	sbt.TotalBlockTxCount = totalBlockTxCount
	sbt.TxNonces = txNonces

	var buf bytes.Buffer
	err := sbt.encodeTxNonces(&buf)
	require.NoError(t, err)

	result := buf.Bytes()
	sbt.TxNonces = nil

	r := bytes.NewReader(result)
	err = sbt.decodeTxNonces(r)
	require.NoError(t, err)

	require.Equal(t, txNonces, sbt.TxNonces)
}

func TestSpanBatchTxsTxGases(t *testing.T) {
	rng := rand.New(rand.NewSource(0x12345))
	chainID := big.NewInt(rng.Int63n(1000))

	rawSpanBatch := RandomRawSpanBatch(rng, chainID)
	txGases := rawSpanBatch.Txs.TxGases
	totalBlockTxCount := rawSpanBatch.Txs.TotalBlockTxCount

	var sbt SpanBatchTxs
	sbt.TotalBlockTxCount = totalBlockTxCount
	sbt.TxGases = txGases

	var buf bytes.Buffer
	err := sbt.encodeTxGases(&buf)
	require.NoError(t, err)

	result := buf.Bytes()
	sbt.TxGases = nil

	r := bytes.NewReader(result)
	err = sbt.decodeTxGases(r)
	require.NoError(t, err)

	require.Equal(t, txGases, sbt.TxGases)
}

func TestSpanBatchTxsTxTos(t *testing.T) {
	rng := rand.New(rand.NewSource(0x54321))
	chainID := big.NewInt(rng.Int63n(1000))

	rawSpanBatch := RandomRawSpanBatch(rng, chainID)
	txTos := rawSpanBatch.Txs.TxTos
	contractCreationBits := rawSpanBatch.Txs.ContractCreationBits
	totalBlockTxCount := rawSpanBatch.Txs.TotalBlockTxCount

	var sbt SpanBatchTxs
	sbt.TxTos = txTos
	// creation bits and block tx count must be se to decode tos
	sbt.ContractCreationBits = contractCreationBits
	sbt.TotalBlockTxCount = totalBlockTxCount

	var buf bytes.Buffer
	err := sbt.encodeTxTos(&buf)
	require.NoError(t, err)

	// to field is fixed length: 20 bytes
	require.Equal(t, buf.Len(), 20*len(txTos))

	result := buf.Bytes()
	sbt.TxTos = nil

	r := bytes.NewReader(result)
	err = sbt.decodeTxTos(r)
	require.NoError(t, err)

	require.Equal(t, txTos, sbt.TxTos)
}

func TestSpanBatchTxsTxDatas(t *testing.T) {
	rng := rand.New(rand.NewSource(0x1234))
	chainID := big.NewInt(rng.Int63n(1000))

	rawSpanBatch := RandomRawSpanBatch(rng, chainID)
	txDatas := rawSpanBatch.Txs.TxDatas
	txTypes := rawSpanBatch.Txs.txTypes
	totalBlockTxCount := rawSpanBatch.Txs.TotalBlockTxCount

	var sbt SpanBatchTxs
	sbt.TotalBlockTxCount = totalBlockTxCount

	sbt.TxDatas = txDatas

	var buf bytes.Buffer
	err := sbt.encodeTxDatas(&buf)
	require.NoError(t, err)

	result := buf.Bytes()
	sbt.TxDatas = nil
	sbt.txTypes = nil

	r := bytes.NewReader(result)
	err = sbt.decodeTxDatas(r)
	require.NoError(t, err)

	require.Equal(t, txDatas, sbt.TxDatas)
	require.Equal(t, txTypes, sbt.txTypes)
}
func TestSpanBatchTxsAddTxs(t *testing.T) {
	rng := rand.New(rand.NewSource(0x1234))
	chainID := big.NewInt(rng.Int63n(1000))
	// make batches to extract txs from
	batches := RandomValidConsecutiveSingularBatches(rng, chainID)
	allTxs := [][]byte{}

	iterativeSBTX, err := newSpanBatchTxs([][]byte{}, chainID)
	require.NoError(t, err)
	for i := 0; i < len(batches); i++ {
		// explicitly extract txs due to mismatch of [][]byte to []hexutil.Bytes
		txs := [][]byte{}
		for j := 0; j < len(batches[i].Transactions); j++ {
			txs = append(txs, batches[i].Transactions[j])
		}
		err = iterativeSBTX.AddTxs(txs, chainID)
		require.NoError(t, err)
		allTxs = append(allTxs, txs...)
	}

	fullSBTX, err := newSpanBatchTxs(allTxs, chainID)
	require.NoError(t, err)

	require.Equal(t, iterativeSBTX, fullSBTX)
}

func TestSpanBatchTxsRecoverV(t *testing.T) {
	rng := rand.New(rand.NewSource(0x123))

	chainID := big.NewInt(rng.Int63n(1000))
	isthmusSigner := types.NewIsthmusSigner(chainID)
	totalblockTxCount := 20 + rng.Intn(100)

	cases := []txTypeTest{
		{"unprotected legacy tx", testutils.RandomLegacyTx, types.HomesteadSigner{}},
		{"legacy tx", testutils.RandomLegacyTx, isthmusSigner},
		{"access list tx", testutils.RandomAccessListTx, isthmusSigner},
		{"dynamic fee tx", testutils.RandomDynamicFeeTx, isthmusSigner},
		{"setcode tx", testutils.RandomSetCodeTx, isthmusSigner},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			var spanBatchTxs SpanBatchTxs
			var txTypes []int
			var txSigs []spanBatchSignature
			var originalVs []uint64
			yParityBits := new(big.Int)
			protectedBits := new(big.Int)
			totalLegacyTxCount := 0
			for idx := 0; idx < totalblockTxCount; idx++ {
				tx := testCase.mkTx(rng, testCase.signer)
				txType := tx.Type()
				txTypes = append(txTypes, int(txType))
				var txSig spanBatchSignature
				v, r, s := tx.RawSignatureValues()
				if txType == types.LegacyTxType {
					protectedBit := uint(0)
					if tx.Protected() {
						protectedBit = uint(1)
					}
					protectedBits.SetBit(protectedBits, int(totalLegacyTxCount), protectedBit)
					totalLegacyTxCount++
				}
				// Do not fill in txSig.V
				txSig.r, _ = uint256.FromBig(r)
				txSig.s, _ = uint256.FromBig(s)
				txSigs = append(txSigs, txSig)
				originalVs = append(originalVs, v.Uint64())
				yParityBit, err := convertVToYParity(v.Uint64(), int(tx.Type()))
				require.NoError(t, err)
				yParityBits.SetBit(yParityBits, idx, yParityBit)
			}

			spanBatchTxs.YParityBits = yParityBits
			spanBatchTxs.TxSigs = txSigs
			spanBatchTxs.txTypes = txTypes
			spanBatchTxs.ProtectedBits = protectedBits
			// recover txSig.v
			err := spanBatchTxs.recoverV(chainID)
			require.NoError(t, err)

			var recoveredVs []uint64
			for _, txSig := range spanBatchTxs.TxSigs {
				recoveredVs = append(recoveredVs, txSig.v)
			}
			require.Equal(t, originalVs, recoveredVs, "recovered v mismatch")
		})
	}
}

func TestSpanBatchTxsRoundTrip(t *testing.T) {
	rng := rand.New(rand.NewSource(0x73311337))
	chainID := big.NewInt(rng.Int63n(1000))

	for i := 0; i < 4; i++ {
		rawSpanBatch := RandomRawSpanBatch(rng, chainID)
		sbt := rawSpanBatch.Txs
		totalBlockTxCount := sbt.TotalBlockTxCount

		var buf bytes.Buffer
		err := sbt.encode(&buf)
		require.NoError(t, err)

		result := buf.Bytes()
		r := bytes.NewReader(result)

		var sbt2 SpanBatchTxs
		sbt2.TotalBlockTxCount = totalBlockTxCount
		err = sbt2.decode(r)
		require.NoError(t, err)

		err = sbt2.recoverV(chainID)
		require.NoError(t, err)

		require.Equal(t, sbt, &sbt2)
	}
}

func TestSpanBatchTxsRoundTripFullTxs(t *testing.T) {
	rng := rand.New(rand.NewSource(0x13377331))
	chainID := big.NewInt(rng.Int63n(1000))
	isthmusSigner := types.NewIsthmusSigner(chainID)

	cases := []txTypeTest{
		{"unprotected legacy tx", testutils.RandomLegacyTx, types.HomesteadSigner{}},
		{"legacy tx", testutils.RandomLegacyTx, isthmusSigner},
		{"access list tx", testutils.RandomAccessListTx, isthmusSigner},
		{"dynamic fee tx", testutils.RandomDynamicFeeTx, isthmusSigner},
		{"setcode tx", testutils.RandomSetCodeTx, isthmusSigner},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			for i := 0; i < 4; i++ {
				totalblockTxCounts := uint64(1 + rng.Int()&0xFF)
				var txs [][]byte
				for i := 0; i < int(totalblockTxCounts); i++ {
					tx := testCase.mkTx(rng, testCase.signer)
					rawTx, err := tx.MarshalBinary()
					require.NoError(t, err)
					txs = append(txs, rawTx)
				}
				sbt, err := newSpanBatchTxs(txs, chainID)
				require.NoError(t, err)

				txs2, err := sbt.fullTxs(chainID)
				require.NoError(t, err)

				require.Equal(t, txs, txs2)
			}
		})
	}
}

func TestSpanBatchTxsRecoverVInvalidTxType(t *testing.T) {
	rng := rand.New(rand.NewSource(0x321))
	chainID := big.NewInt(rng.Int63n(1000))

	var sbt SpanBatchTxs

	sbt.txTypes = []int{types.DepositTxType}
	sbt.TxSigs = []spanBatchSignature{{v: 0, r: nil, s: nil}}
	sbt.YParityBits = new(big.Int)
	sbt.ProtectedBits = new(big.Int)

	err := sbt.recoverV(chainID)
	require.ErrorContains(t, err, "invalid tx type")
}

func TestSpanBatchTxsFullTxNotEnoughTxTos(t *testing.T) {
	rng := rand.New(rand.NewSource(0x13572468))
	chainID := big.NewInt(rng.Int63n(1000))
	isthmusSigner := types.NewIsthmusSigner(chainID)

	cases := []txTypeTest{
		{"unprotected legacy tx", testutils.RandomLegacyTx, types.HomesteadSigner{}},
		{"legacy tx", testutils.RandomLegacyTx, isthmusSigner},
		{"access list tx", testutils.RandomAccessListTx, isthmusSigner},
		{"dynamic fee tx", testutils.RandomDynamicFeeTx, isthmusSigner},
		{"setcode tx", testutils.RandomSetCodeTx, isthmusSigner},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			totalblockTxCounts := uint64(1 + rng.Int()&0xFF)
			var txs [][]byte
			for i := 0; i < int(totalblockTxCounts); i++ {
				tx := testCase.mkTx(rng, testCase.signer)
				rawTx, err := tx.MarshalBinary()
				require.NoError(t, err)
				txs = append(txs, rawTx)
			}
			sbt, err := newSpanBatchTxs(txs, chainID)
			require.NoError(t, err)

			// drop single to field
			sbt.TxTos = sbt.TxTos[:len(sbt.TxTos)-2]

			_, err = sbt.fullTxs(chainID)
			require.EqualError(t, err, "tx to not enough")
		})
	}
}

func TestSpanBatchTxsMaxContractCreationBitsLength(t *testing.T) {
	var sbt SpanBatchTxs
	sbt.TotalBlockTxCount = 0xFFFFFFFFFFFFFFFF

	r := bytes.NewReader([]byte{})
	err := sbt.decodeContractCreationBits(r)
	require.ErrorIs(t, err, ErrTooBigSpanBatchSize)
}

func TestSpanBatchTxsMaxYParityBitsLength(t *testing.T) {
	var sb RawSpanBatch
	sb.BlockCount = 0xFFFFFFFFFFFFFFFF

	r := bytes.NewReader([]byte{})
	err := sb.decodeOriginBits(r)
	require.ErrorIs(t, err, ErrTooBigSpanBatchSize)
}

func TestSpanBatchTxsMaxProtectedBitsLength(t *testing.T) {
	var sb RawSpanBatch
	sb.Txs = &SpanBatchTxs{}
	sb.Txs.totalLegacyTxCount = 0xFFFFFFFFFFFFFFFF

	r := bytes.NewReader([]byte{})
	err := sb.Txs.decodeProtectedBits(r)
	require.ErrorIs(t, err, ErrTooBigSpanBatchSize)
}
