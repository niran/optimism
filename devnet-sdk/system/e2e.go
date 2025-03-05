package system

import (
	"testing"

	"github.com/ethereum-optimism/optimism/op-e2e/system/e2esys"
)

type e2eSystem struct {
	sys *e2esys.System
}

var _ System = (*e2eSystem)(nil)

func (s *e2eSystem) Identifier() string {
	return "e2e"
}

func (s *e2eSystem) L1() Chain {
	rpc := s.sys.NodeEndpoint("l1").RPC()
	// TODO(yann): populate wallets
	return newChain("l1", rpc, nil)
}

func (s *e2eSystem) L2s() []Chain {
	l2Endpoint := s.sys.NodeEndpoint("sequencer").RPC()

	return []Chain{
		newChain("l2", l2Endpoint, nil),
	}
}

func NewE2ESystem(t *testing.T) (System, error) {
	t.Helper()
	cfg := e2esys.DefaultSystemConfig(t)
	sys, err := cfg.Start(t)
	if err != nil {
		return nil, err
	}
	return &e2eSystem{sys: sys}, nil
}
