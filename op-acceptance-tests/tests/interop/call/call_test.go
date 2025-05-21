package call

import (
	"testing"

	"github.com/ethereum-optimism/optimism/op-devstack/devtest"
	"github.com/ethereum-optimism/optimism/op-devstack/dsl"
	"github.com/ethereum-optimism/optimism/op-devstack/presets"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/txplan"
	"github.com/ethereum/go-ethereum/common"
)

func TestMain(m *testing.M) {
	presets.DoMain(m, presets.WithSimpleInterop())
}

func TestCallViewWriteWETH(gt *testing.T) {
	t := devtest.SerialT(gt)
	require := t.Require()
	sys := presets.NewSimpleInterop(t)

	alice := sys.FunderA.NewFundedEOA(eth.ThousandEther)
	bob := sys.FunderA.NewFundedEOA(eth.ThousandEther)

	client := sys.L2ELA.Escape().EthClient()
	wethAddr := common.HexToAddress("0x4200000000000000000000000000000000000006")

	// dsl prep
	factory := dsl.NewWETHCallFactory(&dsl.BaseCallFactory{Target: wethAddr, Client: client})
	weth := dsl.NewWETH(factory.WithTest(t))

	var err error
	// alice and bob has zero WETH
	require.NotEqual(alice.Address(), bob.Address())

	require.Equal(eth.ZeroWei, dsl.TestView(weth.BalanceOf(alice.Address())))
	require.Equal(eth.ZeroWei, dsl.TestView(weth.BalanceOf(bob.Address())))

	// alice wraps 1 WETH
	alice.Transfer(wethAddr, eth.OneEther)

	// view
	// alice has 1 WETH
	require.Equal(eth.OneEther, dsl.TestView(weth.BalanceOf(alice.Address())))
	// bob has 0 WETH.
	require.Equal(eth.ZeroWei, dsl.TestView(weth.BalanceOf(bob.Address())))

	// alice wraps 1 WETH again
	alice.Transfer(wethAddr, eth.OneEther)

	// view
	// alice has 2 WETH
	require.Equal(eth.Ether(2), dsl.TestView(weth.BalanceOf(alice.Address())))
	// bob has 0 WETH.
	require.Equal(eth.ZeroWei, dsl.TestView(weth.BalanceOf(bob.Address())))

	// view(manual error handling)
	// without address sender so failure
	_, err = dsl.View(weth.Transfer(bob.Address(), eth.OneEther))
	t.Require().Error(err)
	// with address, msg.sender set
	require.True(dsl.TestView(weth.Transfer(bob.Address(), eth.OneEther), txplan.WithSender(alice.Address())))

	// write
	// alice sends bob 1 WETH
	dsl.TestWrite(alice, weth.Transfer(bob.Address(), eth.OneEther))

	// view
	// alice has 1 WETH
	require.Equal(eth.OneEther, dsl.TestView(weth.BalanceOf(alice.Address())))
	// bob has 1 WETH.
	require.Equal(eth.OneEther, dsl.TestView(weth.BalanceOf(bob.Address())))

	// write(manual error handling)
	// alice sends bob 1 WETH
	_, err = dsl.Write(alice, weth.Transfer(bob.Address(), eth.OneEther))
	require.NoError(err)
	// alice has 0 WETH
	require.Equal(eth.ZeroWei, dsl.TestView(weth.BalanceOf(alice.Address())))
	// bob has 2 WETH.
	require.Equal(eth.Ether(2), dsl.TestView(weth.BalanceOf(bob.Address())))
}
