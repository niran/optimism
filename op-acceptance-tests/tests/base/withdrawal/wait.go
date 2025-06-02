package withdrawal

import (
	"context"
	"math/big"
	"time"

	"github.com/ethereum-optimism/optimism/op-acceptance-tests/tests/base/withdrawal/utils"
	"github.com/ethereum-optimism/optimism/op-chain-ops/crossdomain"
	"github.com/ethereum-optimism/optimism/op-devstack/devtest"
	"github.com/ethereum-optimism/optimism/op-devstack/dsl"
	"github.com/ethereum-optimism/optimism/op-devstack/dsl/contract"
	"github.com/ethereum-optimism/optimism/op-service/apis"
	"github.com/ethereum-optimism/optimism/op-service/txintent/bindings"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

// ForGamePublished waits until a game is published on L1 for the given l2BlockNumber.
func ForGamePublished(t devtest.T, l2Chain *dsl.L2Network, l1Client apis.EthClient, optimismPortalAddr common.Address, disputeGameFactoryAddr common.Address, l2BlockNumber *big.Int) (uint64, error) {
	_, cancel := context.WithTimeout(t.Ctx(), 2*time.Minute)
	defer cancel()
	l2BlockNumber = new(big.Int).Set(l2BlockNumber) // Don't clobber caller owned l2BlockNumber

	var outputBlockNum *big.Int
	require.Eventually(t, func() bool {
		latestGame, err := utils.FindLatestGame(t, l2Chain, l1Client)
		if err != nil {
			return false
		}
		outputBlockNum = new(big.Int).SetBytes(latestGame.ExtraData[0:32])
		return outputBlockNum.Cmp(l2BlockNumber) >= 0
	}, 30*time.Second, 500*time.Millisecond, "latest game not found")
	return outputBlockNum.Uint64(), nil
}

// ForWithdrawalCheck waits until the withdrawal check in the portal succeeds.
func ForWithdrawalCheck(t devtest.T, alice *dsl.EOA, withdrawal crossdomain.Withdrawal, optimismPortalAddr common.Address, proofSubmitter common.Address) error {
	_, cancel := context.WithTimeout(t.Ctx(), 2*time.Minute)
	defer cancel()
	portalFactory := bindings.NewOptimismPortal2Factory(bindings.WithClient(alice.EthClient()), bindings.WithTo(optimismPortalAddr), bindings.WithTest(t))
	portal := bindings.NewOptimismPortal2(portalFactory)

	var err error
	require.Eventually(t, func() bool {
		wdHash, err2 := withdrawal.Hash()
		if err != nil {
			err = err2
			return false
		}
		return contract.Read(portal.CheckWithdrawal(wdHash, proofSubmitter)).(bool)
	}, 30*time.Second, 500*time.Millisecond, "withdrawal check not found")
	return err
}
