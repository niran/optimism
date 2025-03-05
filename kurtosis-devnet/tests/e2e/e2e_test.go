package e2e

import (
	"testing"

	"github.com/ethereum-optimism/optimism/devnet-sdk/system"
	"github.com/ethereum-optimism/optimism/devnet-sdk/testing/systest"
	"github.com/ethereum-optimism/optimism/devnet-sdk/testing/testlib/validators"
	"github.com/stretchr/testify/require"
)

func e2eTestScenario(sysGetter validators.LowLevelSystemGetter) systest.SystemTestFunc {
	return func(t systest.T, sys system.System) {
		ctx := t.Context()
		t.Log("e2e test scenario")
		lsys := sysGetter(ctx)

		l1 := lsys.L1()
		cl1, err := l1.Client()
		require.NoError(t, err)

		chainID, err := cl1.ChainID(ctx)
		require.NoError(t, err)
		t.Logf("chain ID: %d", chainID)

		l2s := lsys.L2s()
		l2 := l2s[0]
		cl2, err := l2.Client()
		require.NoError(t, err)

		chainID, err = cl2.ChainID(ctx)
		require.NoError(t, err)
		t.Logf("chain ID: %d", chainID)
	}
}

func TestE2ESystem(t *testing.T) {
	lowLevelSys, validator := validators.AcquireLowLevelSystem()
	systest.SystemTest(t,
		e2eTestScenario(lowLevelSys),
		validator,
	)
}
