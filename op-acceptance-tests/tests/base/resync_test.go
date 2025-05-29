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

	el stack.L2ELNode
	cl stack.L2CLNode

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
	el := system.L2Network(match.L2ChainA).L2ELNode(match.FirstL2EL)
	cl := system.L2Network(match.L2ChainA).L2CLNode(match.WithEngine(el.ID()))
	return &systemHelper{
		T:      t,
		orch:   orch,
		system: system,
		el:     el,
		cl:     cl,
		Tick:   waitTime,
	}
}

func (h *systemHelper) Logger() log.Logger {
	return h.system.T().Logger()
}

// query the latest block numbers of the two chains
func (h *systemHelper) query() eth.BlockRef {
	ctx := h.system.T().Ctx()
	ctx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()
	logger := h.system.T().Logger()
	t := h.system.T()
	block, err := h.el.EthClient().BlockRefByLabel(ctx, "latest")
	require.NoError(t, err)
	logger.Info("chain", "blockNum", block.Number, "tip", block)
	return block
}

// check that the two chains are advancing
func (h *systemHelper) CheckAdvancing() func() bool {
	prevBlock := h.query()
	return func() bool {
		block := h.query()
		advanced := block.Number > prevBlock.Number
		prevBlock = block
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
	h.orch.ControlPlane().L2CLNodeState(h.cl.ID(), state)
}

func TestL2CLResync(gt *testing.T) {
	h := newSystemHelper(gt)
	logger := h.Logger()

	logger.Info("check unsafe chain is advancing")
	require.Eventually(h.T, h.CheckAdvancing(), 10*time.Second, h.Tick)

	logger.Info("stop node")
	h.SetCLState(stack.Stop)

	logger.Info("check unsafe chain stopped advancing")
	require.Eventually(h.T, h.CheckStalled(), 10*time.Second, h.Tick)

	logger.Info("restart node")
	h.SetCLState(stack.Start)

	logger.Info("check unsafe chain is advancing again")
	require.Eventually(h.T, h.CheckAdvancing(), 30*time.Second, h.Tick)
}
