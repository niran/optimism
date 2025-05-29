package base

import (
	"context"
	"testing"
	"time"

	"github.com/ethereum-optimism/optimism/op-devstack/devtest"
	"github.com/ethereum-optimism/optimism/op-devstack/presets"
	"github.com/ethereum-optimism/optimism/op-devstack/shim"
	"github.com/ethereum-optimism/optimism/op-devstack/stack"
	"github.com/ethereum-optimism/optimism/op-devstack/stack/match"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/require"
)

type systemHelper struct {
	T      devtest.T
	system stack.ExtensibleSystem
	orch   stack.Orchestrator

	elA, elB stack.L2ELNode
	clA, clB stack.L2CLNode

	Tick time.Duration
}

func newSystemHelper(gt *testing.T) *systemHelper {
	orch := presets.Orchestrator()

	t := devtest.SerialT(gt)
	system := shim.NewSystem(t)
	orch.Hydrate(system)

	blockTime := 2
	waitTime := time.Duration(blockTime+1) * time.Second

	// identify the L2 EL/CL nodes of interest
	elA := system.L2Network(match.L2ChainA).L2ELNode(match.FirstL2EL)
	clA := system.L2Network(match.L2ChainA).L2CLNode(match.WithEngine(elA.ID()))
	elB := system.L2Network(match.L2ChainB).L2ELNode(match.FirstL2EL)
	clB := system.L2Network(match.L2ChainB).L2CLNode(match.WithEngine(elB.ID()))
	return &systemHelper{
		T:      t,
		orch:   orch,
		system: system,
		elA:    elA,
		elB:    elB,
		clA:    clA,
		clB:    clB,
		Tick:   waitTime,
	}
}

func (h *systemHelper) Logger() log.Logger {
	return h.system.T().Logger()
}

// query the latest block numbers of the two chains
func (h *systemHelper) query() (eth.BlockRef, eth.BlockRef) {
	ctx := h.system.T().Ctx()
	ctx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()
	logger := h.system.T().Logger()
	t := h.system.T()
	blockA, err := h.elA.EthClient().BlockRefByLabel(ctx, "latest")
	require.NoError(t, err)
	blockB, err := h.elB.EthClient().BlockRefByLabel(ctx, "latest")
	require.NoError(t, err)
	logger.Info("chain A", "blockNum", blockA.Number, "tip", blockA)
	logger.Info("chain B", "blockNum", blockB.Number, "tip", blockB)
	return blockA, blockB
}

// check that the two chains are advancing
func (h *systemHelper) CheckAdvancing() func() bool {
	prevBlockA, prevBlockB := h.query()
	return func() bool {
		blockA, blockB := h.query()
		advanced := blockA.Number > prevBlockA.Number && blockB.Number > prevBlockB.Number
		prevBlockA, prevBlockB = blockA, blockB
		return advanced
	}
}

// check that the two chains are not advancing
func (h *systemHelper) CheckStalled() func() bool {
	pred := h.CheckAdvancing()
	return func() bool {
		return !pred()
	}
}

func (h *systemHelper) SetCLState(state stack.ControlAction) {
	h.system.T().Logger().Info("setting L2CL state", "state", state)
	h.orch.ControlPlane().L2CLNodeState(h.clA.ID(), state)
	h.orch.ControlPlane().L2CLNodeState(h.clB.ID(), state)
}

func TestL2CLResync(gt *testing.T) {
	h := newSystemHelper(gt)
	logger := h.Logger()

	logger.Info("check unsafe chains are advancing")
	require.Eventually(h.T, h.CheckAdvancing(), 10*time.Second, h.Tick)

	logger.Info("stop nodes")
	h.SetCLState(stack.Stop)

	logger.Info("check unsafe chains stopped advancing")
	require.Eventually(h.T, h.CheckStalled(), 10*time.Second, h.Tick)

	logger.Info("restart nodes")
	h.SetCLState(stack.Start)

	logger.Info("check unsafe chains are advancing again")
	require.Eventually(h.T, h.CheckAdvancing(), 30*time.Second, h.Tick)
}
