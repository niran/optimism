package txplan

import (
	"context"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum-optimism/optimism/devnet-sdk/contracts/constants"
	"github.com/ethereum-optimism/optimism/op-service/client"
	"github.com/ethereum-optimism/optimism/op-service/predeploys"
	"github.com/ethereum-optimism/optimism/op-service/retry"
	"github.com/ethereum-optimism/optimism/op-service/sources"
	"github.com/lmittmann/w3"

	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rpc"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/plan"
	suptypes "github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

type InitTrigger struct {
	Emitter    common.Address // address of the EventLogger contract
	Topics     []common.Hash
	OpaqueData []byte
}

func (v *InitTrigger) To() (*common.Address, error) {
	return &v.Emitter, nil
}

func (v *InitTrigger) Data() ([]byte, error) {
	// TODO format call
	// return nil, nil
	// temp fix
	return v.OpaqueData, nil
}

func (v *InitTrigger) AccessList() (types.AccessList, error) {
	return nil, nil
}

type ExecTrigger struct {
	Executor common.Address // address of the EventLogger contract
	Msg      suptypes.Message
}

func (v *ExecTrigger) To() (*common.Address, error) {
	return &v.Executor, nil
}

func (v *ExecTrigger) Data() ([]byte, error) {
	// TODO format call to CrossL2Inbox
	// Need to do better. very ugly
	// construct call input, ugly but no bindings...
	validateMessage := w3.MustNewFunc("validateMessage((address Origin, uint256 BlockNumber, uint256 LogIndex, uint256 Timestamp, uint256 ChainId), bytes32)", "")
	type Identifier struct {
		Origin      common.Address
		BlockNumber *big.Int
		LogIndex    *big.Int
		Timestamp   *big.Int
		ChainId     *big.Int
	}
	identifier := &Identifier{
		v.Msg.Identifier.Origin,
		big.NewInt(int64(v.Msg.Identifier.BlockNumber)),
		big.NewInt(int64(v.Msg.Identifier.LogIndex)),
		big.NewInt(int64(v.Msg.Identifier.Timestamp)),
		v.Msg.Identifier.ChainID.ToBig(),
	}
	validateMessageCalldata, err := validateMessage.EncodeArgs(
		identifier,
		v.Msg.PayloadHash,
	)
	if err != nil {
		return nil, err
	}
	return validateMessageCalldata, nil
}

func (v *ExecTrigger) AccessList() (types.AccessList, error) {
	access := v.Msg.Access()
	accessList := types.AccessList{{
		Address:     constants.CrossL2Inbox,
		StorageKeys: suptypes.EncodeAccessList([]suptypes.Access{access}),
	}}
	return accessList, nil
}

type Call interface {
	To() (*common.Address, error)
	Data() ([]byte, error)
	AccessList() (types.AccessList, error)
}

type MultiTrigger struct {
	Calls []Call
}

func (v *MultiTrigger) Data() ([]byte, error) {
	// TODO format multi-call
	return nil, nil
}

type Result interface {
	FromReceipt(ctx context.Context, rec *types.Receipt, includedIn eth.BlockRef, chainID eth.ChainID) error
	Init() Result
}

type InteropOutput struct {
	Entries []suptypes.Message
}

func (i *InteropOutput) Init() Result {
	return &InteropOutput{}
}

func (i *InteropOutput) FromReceipt(ctx context.Context, rec *types.Receipt, includedIn eth.BlockRef, chainID eth.ChainID) error {
	entries := []suptypes.Message{}
	for _, logEvent := range rec.Logs {
		payload := suptypes.LogToMessagePayload(logEvent)
		id := suptypes.Identifier{
			Origin:      logEvent.Address,
			BlockNumber: logEvent.BlockNumber,
			LogIndex:    uint32(logEvent.Index),
			Timestamp:   includedIn.Time,
			ChainID:     chainID,
		}
		payloadHash := crypto.Keccak256Hash(payload)
		entries = append(entries, suptypes.Message{
			Identifier:  id,
			PayloadHash: payloadHash,
		})
	}
	i.Entries = entries
	return nil
}

type IntentTx[V Call, R Result] struct {
	PlannedTx *PlannedTx
	Content   plan.Lazy[V]
	Result    plan.Lazy[R]
}

func NewIntent[V Call, R Result](opts ...Option) *IntentTx[V, R] {
	v := &IntentTx[V, R]{
		PlannedTx: NewPlannedTx(opts...),
	}
	v.PlannedTx.To.DependOn(&v.Content)
	v.PlannedTx.To.Fn(func(ctx context.Context) (*common.Address, error) {
		return v.Content.Value().To()
	})
	v.PlannedTx.Data.DependOn(&v.Content)
	v.PlannedTx.Data.Fn(func(ctx context.Context) (hexutil.Bytes, error) {
		return v.Content.Value().Data()
	})
	v.PlannedTx.AccessList.DependOn(&v.Content)
	v.PlannedTx.AccessList.Fn(func(ctx context.Context) (types.AccessList, error) {
		return v.Content.Value().AccessList()
	})
	v.Result.DependOn(&v.PlannedTx.Included, &v.PlannedTx.IncludedBlock, &v.PlannedTx.ChainID)
	v.Result.Fn(func(ctx context.Context) (R, error) {
		r := (*new(R)).Init().(R)
		err := r.FromReceipt(ctx, v.PlannedTx.Included.Value(), v.PlannedTx.IncludedBlock.Value(), v.PlannedTx.ChainID.Value())
		return r, err
	})
	return v
}

func executeIndexed(executor common.Address, events *plan.Lazy[*InteropOutput], index int) func(ctx context.Context) (*ExecTrigger, error) {
	return func(ctx context.Context) (*ExecTrigger, error) {
		if x := len(events.Value().Entries); x <= index {
			return nil, fmt.Errorf("invalid index: %d, only have %d events", index, x)
		}
		return &ExecTrigger{
			Executor: executor,
			Msg:      events.Value().Entries[index],
		}, nil
	}
}

func TestSimpleTx(t *testing.T) {
	// priv, err := crypto.GenerateKey()
	// wallets must contain private key and rpc
	privHex := "0xf214f2b2cd398c806f84e317254e0f0b801d0643303237d97a22a48e01628897"
	rpcURL := "http://127.0.0.1:32948"

	privRaw, err := hexutil.Decode(privHex)
	require.NoError(t, err)
	priv, err := crypto.ToECDSA(privRaw)
	require.NoError(t, err)

	rpcClient, err := rpc.DialContext(context.Background(), rpcURL)
	require.NoError(t, err)

	ethClCfg := sources.EthClientConfig{
		MaxRequestsPerBatch:   10,
		MaxConcurrentRequests: 10,
		ReceiptsCacheSize:     10,
		TransactionsCacheSize: 10,
		HeadersCacheSize:      10,
		PayloadsCacheSize:     10,
		BlockRefsCacheSize:    10,
		TrustRPC:              false,
		MustBePostMerge:       true,
		RPCProviderKind:       sources.RPCKindStandard,
		MethodResetDuration:   time.Minute,
	}
	cl, err := sources.NewEthClient(client.NewBaseRPCClient(rpcClient), log.Root(), nil, &ethClCfg)
	require.NoError(t, err)

	to := common.HexToAddress("0x0000000000000000000000000000000000001235")
	opts := Combine(
		WithPrivateKey(priv),
		WithChainID(cl),
		WithAgainstLatestBlock(cl),
		WithPendingNonce(cl),
		WithEstimator(cl, false),
		WithTo(&to),
		WithTransactionSubmitter(cl),
		WithRetryInclusion(cl, 10, retry.Exponential()),
		WithBlockInclusionInfo(cl),
	)

	txSimple := NewPlannedTx(opts, WithValue(big.NewInt(1000)))
	res, err := txSimple.IncludedBlock.Eval(context.Background())
	require.NoError(t, err)
	t.Logf("included simple tx in block %s", res)
}
func TestInteropTx(t *testing.T) {
	// priv, err := crypto.GenerateKey()
	privHexRawA := "0xf214f2b2cd398c806f84e317254e0f0b801d0643303237d97a22a48e01628897"
	rpcURLA := "http://127.0.0.1:32948"

	privHexRawB := "0xeaa861a9a01391ed3d587d8a5a84ca56ee277629a8b02c22093a419bf240e65d"
	rpcURLB := "http://127.0.0.1:32961"

	ethClCfg := sources.EthClientConfig{
		MaxRequestsPerBatch:   10,
		MaxConcurrentRequests: 10,
		ReceiptsCacheSize:     10,
		TransactionsCacheSize: 10,
		HeadersCacheSize:      10,
		PayloadsCacheSize:     10,
		BlockRefsCacheSize:    10,
		TrustRPC:              false,
		MustBePostMerge:       true,
		RPCProviderKind:       sources.RPCKindStandard,
		MethodResetDuration:   time.Minute,
	}

	optsFunc := func(rpcURL string, privHexRaw string) Option {
		privRaw, err := hexutil.Decode(privHexRaw)
		require.NoError(t, err)
		priv, err := crypto.ToECDSA(privRaw)
		require.NoError(t, err)
		rpcClient, err := rpc.DialContext(context.Background(), rpcURL)
		require.NoError(t, err)
		cl, err := sources.NewEthClient(client.NewBaseRPCClient(rpcClient), log.Root(), nil, &ethClCfg)
		require.NoError(t, err)

		return Combine(
			WithPrivateKey(priv),
			WithChainID(cl),
			WithAgainstLatestBlock(cl),
			WithPendingNonce(cl),
			WithEstimator(cl, false),
			WithTransactionSubmitter(cl),
			WithRetryInclusion(cl, 10, retry.Exponential()),
			WithBlockInclusionInfo(cl),
		)
	}
	optsA := optsFunc(rpcURLA, privHexRawA)
	optsB := optsFunc(rpcURLB, privHexRawB)

	// eventLogger := common.Address{} // TODO deploy tx

	sha256PrecompileAddr := common.BytesToAddress([]byte{0x2})
	dummyMessage := []byte("l33t message")
	destChainID := big.NewInt(2151909) // TODO: remove hardcode
	// construct call input, ugly but no bindings...
	sendMessage := w3.MustNewFunc("sendMessage(uint256,address,bytes calldata)", "bytes32")
	sendMessageCalldata, err := sendMessage.EncodeArgs(
		destChainID,
		sha256PrecompileAddr,
		dummyMessage,
	)
	require.NoError(t, err)

	opagueData := sendMessageCalldata
	L2toL2CDM := common.HexToAddress(predeploys.L2toL2CrossDomainMessenger)
	txA := NewIntent[*InitTrigger, *InteropOutput](optsA)
	txA.Content.Set(&InitTrigger{
		// Emitter:    eventLogger,
		Emitter:    L2toL2CDM,
		Topics:     []common.Hash{},
		OpaqueData: opagueData,
	})

	txB := NewIntent[*ExecTrigger, *InteropOutput](optsB)
	txB.Content.DependOn(&txA.Result)
	CrossL2InboxAddr := common.HexToAddress(predeploys.CrossL2Inbox)
	txB.Content.Fn(executeIndexed(CrossL2InboxAddr, &txA.Result, 0))

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	// recA, err := txA.PlannedTx.Success.Eval(ctx)
	recA, err := txA.Result.Eval(ctx)

	require.NoError(t, err)

	t.Log(recA)
	t.Log("included initiating tx")

	recB, err := txB.PlannedTx.Included.Eval(ctx)
	require.NoError(t, err)
	t.Logf("included executing tx in block %s", recB.BlockHash)
}
