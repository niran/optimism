package pipeline

import (
	"fmt"

	"github.com/ethereum-optimism/optimism/op-chain-ops/interopgen/deployers"
	"github.com/ethereum-optimism/optimism/op-deployer/pkg/deployer/state"
)

func PreinstallL1DevGenesis(env *Env, intent *state.Intent, st *state.State) error {
	lgr := env.Logger.New("stage", "preinstall-l1-dev-genesis")
	lgr.Info("Adding preinstalls to L1 dev genesis")

	if err := deployers.InsertPreinstalls(env.L1ScriptHost); err != nil {
		return fmt.Errorf("failed to add preinstalls to L1 dev state: %w", err)
	}
	env.L1ScriptHost.Wipe(env.Deployer)

	return nil
}
