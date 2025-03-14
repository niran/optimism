package proofs_test

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"

	actionsHelpers "github.com/ethereum-optimism/optimism/op-e2e/actions/helpers"
	"github.com/ethereum-optimism/optimism/op-e2e/actions/proofs/helpers"
	"github.com/ethereum-optimism/optimism/op-program/client/claim"
)

func runSetCodeTxTypeTest(gt *testing.T, testCfg *helpers.TestCfg[any]) {
	t := actionsHelpers.NewDefaultTesting(gt)
	env := helpers.NewL2FaultProofEnv(t, testCfg, helpers.NewTestParams(), helpers.NewBatcherCfg())

	cl := env.Engine.EthClient()
	sequencer := env.Sequencer
	miner := env.Miner

	miner.ActEmptyBlock(t)
	sequencer.ActL1HeadSignal(t)

	sequencer.ActL2EmptyBlock(t)

	batcher := env.Batcher

	aliceSecret := env.Alice.L2.Secret()

	chainID := env.Sequencer.RollupCfg.L2ChainID

	txdata := &types.SetCodeTx{
		ChainID:   uint256.MustFromBig(chainID),
		Nonce:     0,
		To:        env.Alice.Address(),
		Gas:       500000,
		GasFeeCap: uint256.NewInt(5000000000),
		GasTipCap: uint256.NewInt(2),
		AuthList:  []types.SetCodeAuthorization{},
	}
	signer := types.NewIsthmusSigner(chainID)
	tx := types.MustSignNewTx(aliceSecret, signer, txdata)

	batcher.ActL2BatchBuffer(t, func(block *types.Block) *types.Block {
		// inject user tx into upgrade batch
		return block.WithBody(types.Body{Transactions: append(block.Transactions(), tx)})
	})

	batcher.ActL2ChannelClose(t)
	batcher.ActL2BatchSubmit(t)

	env.Miner.ActL1StartBlock(12)(t)
	env.Miner.ActL1IncludeTxByHash(env.Batcher.LastSubmitted.Hash())(t)
	env.Miner.ActL1EndBlock(t)

	env.Sequencer.ActL1HeadSignal(t)
	env.Sequencer.ActL2PipelineFull(t)

	env.Sequencer.ActL1HeadSignal(t)
	env.Sequencer.ActL2PipelineFull(t)

	latestBlock, err := cl.BlockByNumber(t.Ctx(), nil)
	require.NoError(t, err, "error fetching latest block")

	env.RunFaultProofProgram(t, latestBlock.NumberU64(), testCfg.CheckResult, testCfg.InputParams...)
}

func TestSetCodeTx(gt *testing.T) {
	matrix := helpers.NewMatrix[any]()
	defer matrix.Run(gt)

	cases := []struct {
		name          string
		expectSuccess bool
		hardfork      helpers.Hardfork
	}{
		{
			name:          "PreIsthmus",
			expectSuccess: false,
			hardfork:      *helpers.Holocene,
		},
		{
			name:          "Isthmus",
			expectSuccess: true,
			hardfork:      *helpers.Isthmus,
		},
	}

	for _, c := range cases {
		matrix.AddTestCase(
			"HonestClaim-"+c.name+"-Failure",
			nil,
			helpers.NewForkMatrix(&c.hardfork),
			runSetCodeTxTypeTest,
			helpers.ExpectError(claim.ErrClaimNotValid),
		)

		if c.expectSuccess {
			matrix.AddTestCase(
				"JunkClaim-"+c.name,
				nil,
				helpers.NewForkMatrix(&c.hardfork),
				runSetCodeTxTypeTest,
				helpers.ExpectError(claim.ErrClaimNotValid),
				helpers.WithL2Claim(common.HexToHash("0xdeadbeef")),
			)
		}
	}
}
