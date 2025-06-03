package bindings

import (
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/stretchr/testify/require"
)

type TestBaseCallContractFactory struct {
	BaseCallFactory
}

func NewTestBaseContractCallFactory(opts ...CallFactoryOption) *TestBaseCallContractFactory {
	return &TestBaseCallContractFactory{BaseCallFactory: *NewBaseCallFactory(opts...)}
}

type TestBaseContract struct {
	TestBaseCallContractFactory

	FinalizeWithdrawalTransaction func(tx struct {
		Nonce    *big.Int
		Sender   common.Address
		Target   common.Address
		Value    *big.Int
		GasLimit *big.Int
		Data     []byte
	}) TypedCall[any] `sol:"finalizeWithdrawalTransaction"`

	ProveWithdrawalTransaction func(tx struct {
		Nonce    *big.Int
		Sender   common.Address
		Target   common.Address
		Value    *big.Int
		GasLimit *big.Int
		Data     []byte
	}, disputeGameIndex *big.Int, outputRootProof struct {
		Version                  [32]byte
		StateRoot                [32]byte
		MessagePasserStorageRoot [32]byte
		LatestBlockhash          [32]byte
	}, withdrawalProof [][]byte) TypedCall[any] `sol:"proveWithdrawalTransaction"`
}

func NewTestBaseContract(f *TestBaseCallContractFactory) *TestBaseContract {
	testBase := TestBaseContract{TestBaseCallContractFactory: *f}
	InitImpl(&testBase)
	return &testBase
}

func TestEncode(t *testing.T) {
	factory := NewTestBaseContractCallFactory()
	testBaseContract := NewTestBaseContract(factory)

	call := testBaseContract.FinalizeWithdrawalTransaction(
		struct {
			Nonce    *big.Int
			Sender   common.Address
			Target   common.Address
			Value    *big.Int
			GasLimit *big.Int
			Data     []byte
		}{
			Nonce:    new(big.Int).Lsh(big.NewInt(1), 240),
			Sender:   common.HexToAddress("0x15d34AAf54267DB7D7c367839AAf71A00a2C6A65"),
			Target:   common.HexToAddress("0x15d34AAf54267DB7D7c367839AAf71A00a2C6A65"),
			Value:    big.NewInt(500000000000),
			GasLimit: big.NewInt(21000),
			Data:     []byte(""),
		},
	)

	calldata, err := call.EncodeInputLambda()
	require.NoError(t, err)
	require.Equal(t, "8c3152e90000000000000000000000000000000000000000000000000000000000000020000100000000000000000000000000000000000000000000000000000000000000000000000000000000000015d34aaf54267db7d7c367839aaf71a00a2c6a6500000000000000000000000015d34aaf54267db7d7c367839aaf71a00a2c6a65000000000000000000000000000000000000000000000000000000746a528800000000000000000000000000000000000000000000000000000000000000520800000000000000000000000000000000000000000000000000000000000000c0000000000000000000000000000000000000000000000000000000000000000",
		hex.EncodeToString(calldata),
	)

	call = testBaseContract.ProveWithdrawalTransaction(
		struct {
			Nonce    *big.Int
			Sender   common.Address
			Target   common.Address
			Value    *big.Int
			GasLimit *big.Int
			Data     []byte
		}{
			Nonce:    new(big.Int).Lsh(big.NewInt(1), 240),
			Sender:   common.HexToAddress("0x15d34AAf54267DB7D7c367839AAf71A00a2C6A65"),
			Target:   common.HexToAddress("0x15d34AAf54267DB7D7c367839AAf71A00a2C6A65"),
			Value:    big.NewInt(500000000000),
			GasLimit: big.NewInt(21000),
			Data:     []byte(""),
		},
		big.NewInt(1),
		struct {
			Version                  [32]byte
			StateRoot                [32]byte
			MessagePasserStorageRoot [32]byte
			LatestBlockhash          [32]byte
		}{
			Version:                  *(*[32]byte)(hexutil.MustDecode("0x0000000000000000000000000000000000000000000000000000000000000000")),
			StateRoot:                *(*[32]byte)(hexutil.MustDecode("0x73aa3ddeddee968a18a19312efccd06ebe116f86e3f23961cc83ef26346894ba")),
			MessagePasserStorageRoot: *(*[32]byte)(hexutil.MustDecode("0xe3f2a88ce530a8dab9f8cafac0ef934b1f126da1041d89e41cb84d46dfa5e841")),
			LatestBlockhash:          *(*[32]byte)(hexutil.MustDecode("0xf79e208e723e8ca525558786b4fc73c1c889e9eb0e25917ba5c5ec7640ffc257")),
		},
		[][]byte{
			hexutil.MustDecode("0xf8718080808080a08e9a5e2311b6926cff4a3b9b50fd0500e2d68f2d70c62f7b294aec18b62e94d980a08c82f7353a759f9fdf815a3065d8e8b1282d1383398e53f11f8f03bf64f50cfa808080a0f4984a11f61a2921456141df88de6e1a710d28681b91af794c5a721e47839cd78080808080"),
			hexutil.MustDecode("0xf8518080a0999c5deb49aff57f74c1a5871afb58461105ec7bf684c9716f8ee2c30221bfd78080808080808080a05219be3ea6e6c12cfa6927fd85a1548be9922594ebbd7d8ad717600fbd64f7fe8080808080"),
			hexutil.MustDecode("0xe2a0206c4fd0e580d501e7a56378cab19a4875bba79b4639cdbd1db734feb96f87dd01"),
		},
	)

	calldata, err = call.EncodeInputLambda()
	require.NoError(t, err)
	require.Equal(t, "4870496f00000000000000000000000000000000000000000000000000000000000000e00000000000000000000000000000000000000000000000000000000000000001000000000000000000000000000000000000000000000000000000000000000073aa3ddeddee968a18a19312efccd06ebe116f86e3f23961cc83ef26346894bae3f2a88ce530a8dab9f8cafac0ef934b1f126da1041d89e41cb84d46dfa5e841f79e208e723e8ca525558786b4fc73c1c889e9eb0e25917ba5c5ec7640ffc25700000000000000000000000000000000000000000000000000000000000001c0000100000000000000000000000000000000000000000000000000000000000000000000000000000000000015d34aaf54267db7d7c367839aaf71a00a2c6a6500000000000000000000000015d34aaf54267db7d7c367839aaf71a00a2c6a65000000000000000000000000000000000000000000000000000000746a528800000000000000000000000000000000000000000000000000000000000000520800000000000000000000000000000000000000000000000000000000000000c0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000030000000000000000000000000000000000000000000000000000000000000060000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000001800000000000000000000000000000000000000000000000000000000000000073f8718080808080a08e9a5e2311b6926cff4a3b9b50fd0500e2d68f2d70c62f7b294aec18b62e94d980a08c82f7353a759f9fdf815a3065d8e8b1282d1383398e53f11f8f03bf64f50cfa808080a0f4984a11f61a2921456141df88de6e1a710d28681b91af794c5a721e47839cd78080808080000000000000000000000000000000000000000000000000000000000000000000000000000000000000000053f8518080a0999c5deb49aff57f74c1a5871afb58461105ec7bf684c9716f8ee2c30221bfd78080808080808080a05219be3ea6e6c12cfa6927fd85a1548be9922594ebbd7d8ad717600fbd64f7fe8080808080000000000000000000000000000000000000000000000000000000000000000000000000000000000000000023e2a0206c4fd0e580d501e7a56378cab19a4875bba79b4639cdbd1db734feb96f87dd010000000000000000000000000000000000000000000000000000000000",
		hex.EncodeToString(calldata),
	)
}
