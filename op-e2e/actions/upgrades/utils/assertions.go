package utils

import (
	"math/big"

	actionhelpers "github.com/ethereum-optimism/optimism/op-e2e/actions/helpers"
	"github.com/ethereum-optimism/optimism/op-e2e/actions/interop/dsl"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/lmittmann/w3"
	"github.com/stretchr/testify/require"
)

var ProxyImplGetterFunc = w3.MustNewFunc(`implementation()`, `address`)

func RequireUpgradeSuccessful(t actionhelpers.Testing, actors *dsl.InteropActors, implAddr common.Address, proxyAddress common.Address, activationBlockNumber *big.Int) {
	code, err := actors.ChainA.SequencerEngine.EthClient().CodeAt(t.Ctx(), implAddr, activationBlockNumber)
	require.NoError(t, err)
	require.NotEmpty(t, code, "CrossL2Inbox contract should be deployed")
	selector, err := ProxyImplGetterFunc.EncodeArgs()
	require.NoError(t, err)
	implAddrBytes, err := actors.ChainA.SequencerEngine.EthClient().CallContract(t.Ctx(), ethereum.CallMsg{
		To:   &proxyAddress,
		Data: selector,
	}, activationBlockNumber)
	require.NoError(t, err)
	var implAddrActual common.Address
	err = ProxyImplGetterFunc.DecodeReturns(implAddrBytes, &implAddrActual)
	require.NoError(t, err)
	require.Equal(t, implAddr, implAddrActual)
}
