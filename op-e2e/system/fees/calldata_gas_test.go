package fees

import (
	"context"
	"math/big"
	"testing"
	"time"

	op_e2e "github.com/ethereum-optimism/optimism/op-e2e"

	"github.com/ethereum-optimism/optimism/op-e2e/bindings"
	"github.com/ethereum-optimism/optimism/op-e2e/e2eutils/geth"
	"github.com/ethereum-optimism/optimism/op-e2e/system/e2esys"
	"github.com/ethereum-optimism/optimism/op-service/predeploys"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)


// TestCalldataGasPerCompressedByteChange tests that the calldata gas per compressed byte 
// parameter can be updated via SystemConfig and is properly reflected in the L1Block contract
func TestCalldataGasPerCompressedByteChange(t *testing.T) {
	t.Run("jovian", func(t *testing.T) {
		op_e2e.InitParallel(t)
		cfg := e2esys.JovianSystemConfig(t, nil)
		testCalldataGasPerCompressedByteChange(t, &cfg)
	})
}

func testCalldataGasPerCompressedByteChange(t *testing.T, cfg *e2esys.SystemConfig) {
	ctx := context.Background()

	sys, err := cfg.Start(t)
	require.NoError(t, err, "Error starting up system")
	defer sys.Close()

	l1Client := sys.NodeClient("l1")
	l2Seq := sys.NodeClient("sequencer")
	l2Verif := sys.NodeClient("verifier")

	// Transactor accounts
	ethPrivKey := cfg.Secrets.Alice

	// Bind to the SystemConfig contract
	sysCfgContract, err := bindings.NewSystemConfig(cfg.L1Deployments.SystemConfigProxy, l1Client)
	require.NoError(t, err)

	sysCfgOwner, err := bind.NewKeyedTransactorWithChainID(cfg.Secrets.SysCfgOwner, cfg.L1ChainIDBig())
	require.NoError(t, err)

	// Bind to the L1Block contract  
	l1BlockContract, err := bindings.NewL1Block(predeploys.L1BlockAddr, l2Seq)
	require.NoError(t, err)

	// Get initial calldata gas per compressed byte value
	initialCalldataGas, err := l1BlockContract.CalldataGasPerCompressedByte(nil)
	require.NoError(t, err)
	t.Logf("Initial calldata gas per compressed byte: %d", initialCalldataGas)

	// Update the parameter to a new value
	newCalldataGas := uint32(32)
	tx, err := sysCfgContract.SetCalldataGasPerCompressedByte(sysCfgOwner, newCalldataGas)
	require.NoError(t, err)
	t.Logf("Sent SystemConfig update tx: %s", tx.Hash())

	// Mine the transaction
	receipt, err := geth.WaitForTransaction(tx.Hash(), l1Client, 10*time.Second)
	require.NoError(t, err)
	require.Equal(t, types.ReceiptStatusSuccessful, receipt.Status, "SystemConfig update should succeed")

	// Wait for the L2 system to process the L1 block
	_, err = geth.WaitForBlock(big.NewInt(int64(receipt.BlockNumber.Uint64()+4)), l2Seq)
	require.NoError(t, err)

	// Verify the parameter was updated in L1Block
	updatedCalldataGas, err := l1BlockContract.CalldataGasPerCompressedByte(nil)
	require.NoError(t, err)
	require.Equal(t, newCalldataGas, updatedCalldataGas, "Calldata gas per compressed byte should be updated")

	// Verify that the verifier also sees the update
	l1BlockContractVerif, err := bindings.NewL1Block(predeploys.L1BlockAddr, l2Verif)
	require.NoError(t, err)

	verifCalldataGas, err := l1BlockContractVerif.CalldataGasPerCompressedByte(nil)
	require.NoError(t, err)
	require.Equal(t, newCalldataGas, verifCalldataGas, "Verifier should see the same updated value")

	// Send a transaction to make sure the system is still working
	fromAddr := crypto.PubkeyToAddress(ethPrivKey.PublicKey)
	nonce, err := l2Seq.PendingNonceAt(ctx, fromAddr)
	require.NoError(t, err)

	signer := types.NewEIP155Signer(cfg.L2ChainIDBig())
	tx = types.MustSignNewTx(ethPrivKey, signer, &types.DynamicFeeTx{
		ChainID:   cfg.L2ChainIDBig(),
		Nonce:     nonce,
		To:        &common.Address{0xff},
		Value:     big.NewInt(0),
		GasTipCap: big.NewInt(10),
		GasFeeCap: big.NewInt(200),
		Gas:       21000,
	})

	err = l2Seq.SendTransaction(ctx, tx)
	require.NoError(t, err)

	receipt, err = geth.WaitForTransaction(tx.Hash(), l2Seq, 10*time.Second)
	require.NoError(t, err)
	require.Equal(t, types.ReceiptStatusSuccessful, receipt.Status, "test tx should succeed after parameter update")

	t.Logf("Test transaction succeeded with updated calldata gas parameter")
}