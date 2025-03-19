package rollup

import (
	"fmt"
	"testing"

	"github.com/ethereum/go-ethereum/superchain"
	"github.com/lmittmann/w3"
	"github.com/lmittmann/w3/module/eth"
	"github.com/stretchr/testify/require"
)

func TestUpgradeTxGas(t *testing.T) {
	for _, chain := range superchain.ChainNames() {
		id, err := superchain.ChainIDByName(chain)
		if err != nil {
			t.Errorf("Error: %v", err)
		}
		ch, err := superchain.GetChain(id)
		if err != nil {
			t.Errorf("Error: %v", err)
		}

		cfg, err := ch.Config()
		if err != nil {
			t.Errorf("Error: %v", err)
		}

		sysCfg := cfg.Addresses.SystemConfigProxy
		funcGasLimit := w3.MustNewFunc("gasLimit()", "uint64")
		sc, err := superchain.GetSuperchain(ch.Network)
		if err != nil {
			t.Errorf("Error: %v", err)
		}

		client := w3.MustDial(sc.L1.PublicRPC)
		defer client.Close()
		var gasLimit uint64
		err = client.Call(eth.CallFunc(*sysCfg, funcGasLimit).Returns(&gasLimit))
		if err != nil {
			t.Errorf("Error: %v", err)
		}
		t.Log("Chain:", chain, "GasLimit", fmt.Sprintf("%.1fM", float64(gasLimit)/1000000))
		require.GreaterOrEqual(t, gasLimit, uint64(30_000_000))
	}
}
