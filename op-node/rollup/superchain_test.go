package rollup

import (
	"fmt"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/superchain"
	"github.com/lmittmann/w3"
	"github.com/lmittmann/w3/module/eth"
	"github.com/lmittmann/w3/w3types"
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

func TestSuperchainGasLimit(t *testing.T) {

	var sysCfgAddressGetter AddressGetter = func(cfg superchain.ChainConfig) *common.Address {
		return cfg.Addresses.SystemConfigProxy
	}

	var callfunc = w3.MustNewFunc("gasLimit()", "uint64")

	r := GetL1SuperchainInformation([]call{
		{
			a: sysCfgAddressGetter,
			f: callfunc,
			r: new(uint64),
		},
	})
	require.NotNil(t, r)

	// gl := *r[10]["gasLimit()"].(*uint64)
	// t.Logf("%d", gl)
	for chainID, chainResults := range r {
		t.Log("Chain:", chainID, "GasLimit", fmt.Sprintf("%.1fM", float64(*chainResults["gasLimit()"].(*uint64))/1000000))
		require.GreaterOrEqual(t, *chainResults["gasLimit()"].(*uint64), uint64(30_000_000))
	}
}

type AddressGetter func(superchain.ChainConfig) *common.Address
type call struct {
	a AddressGetter
	f *w3.Func
	r any // return type
}

// maps chainId to function signature to result
type results map[uint64]map[string]interface{}

func GetL1SuperchainInformation(calls []call) results {
	results := make(results)

	superchains := []string{"sepolia", "mainnet"}

	for _, sc := range superchains {
		for _, chain := range superchain.ChainNames() {

			id, err := superchain.ChainIDByName(chain)
			if err != nil {
				panic(err)
			}
			ch, err := superchain.GetChain(id)
			if err != nil {
				panic(err)
			}
			if ch.Network != sc {
				continue
			}
			cfg, err := ch.Config()
			if err != nil {
				panic(err)
			}
			results[cfg.ChainID] = make(map[string]interface{})
			superC, err := superchain.GetSuperchain(ch.Network)
			if err != nil {
				panic(err)
			}
			client := w3.MustDial(superC.L1.PublicRPC)
			defer client.Close()

			// loop over calls
			rawCalls := make([]w3types.RPCCaller, len(calls))
			for i, call := range calls {
				results[cfg.ChainID][call.f.Signature] = call.r
				rawCalls[i] = eth.CallFunc(*(call.a(*cfg)), call.f).Returns(results[cfg.ChainID][call.f.Signature])
			}
			fmt.Printf("%+v", rawCalls)
			if err != nil {
				panic(err)
			}
		}
	}
	return results
}
