package call

import (
	"testing"

	"github.com/ethereum-optimism/optimism/op-devstack/devtest"
	"github.com/ethereum-optimism/optimism/op-devstack/dsl"
	"github.com/ethereum-optimism/optimism/op-devstack/presets"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/txintent"
	"github.com/ethereum-optimism/optimism/op-service/txplan"
	"github.com/ethereum/go-ethereum/common"
)

func TestMain(m *testing.M) {
	presets.DoMain(m, presets.WithSimpleInterop())
}

func TestCallViewWriteWETH(gt *testing.T) {
	t := devtest.SerialT(gt)
	sys := presets.NewSimpleInterop(t)

	alice := sys.FunderA.NewFundedEOA(eth.ThousandEther)
	bob := sys.FunderA.NewFundedEOA(eth.ThousandEther)

	wethAddr := common.HexToAddress("0x4200000000000000000000000000000000000006")
	// dsl prep
	// TODO: delegate this initialization to somewhere else
	balanceOf := func(addr common.Address) txintent.View[eth.ETH] {
		return dsl.BalanceOfCall{Addr: addr}
	}
	transfer := func(dest common.Address, amount eth.ETH) txintent.View[bool] {
		return dsl.TransferCall{Dest: dest, Amount: amount}
	}
	unboundWETH := dsl.UnboundWETH{BalanceOf: balanceOf, Transfer: transfer}

	weth := &dsl.WETH{UnboundWETH: unboundWETH}
	// hydration phase
	client := sys.L2ELA.Escape().EthClient()
	weth = weth.WithTo(wethAddr).WithClient(client)

	var balance eth.ETH
	var err error
	// alice and bob has zero WETH
	t.Require().NotEqual(alice.Address(), bob.Address())

	balance, err = dsl.View(weth.BalanceOf(alice.Address()))
	t.Require().NoError(err)
	t.Require().Equal(eth.ZeroWei, balance)
	balance, err = dsl.View(weth.BalanceOf(bob.Address()))
	t.Require().NoError(err)
	t.Require().Equal(eth.ZeroWei, balance)

	// alice wraps 1 WETH
	alice.Transfer(wethAddr, eth.OneEther)

	// view
	// alice has 1 WETH
	balance, err = dsl.View(weth.BalanceOf(alice.Address()))
	t.Require().NoError(err)
	t.Require().Equal(eth.OneEther, balance)
	// bob has 0 WETH.
	balance, err = dsl.View(weth.BalanceOf(bob.Address()))
	t.Require().NoError(err)
	t.Require().Equal(eth.ZeroWei, balance)

	// alice wraps 1 WETH again
	alice.Transfer(wethAddr, eth.OneEther)

	// view
	// alice has 2 WETH
	balance, err = dsl.View(weth.BalanceOf(alice.Address()))
	t.Require().NoError(err)
	t.Require().Equal(eth.Ether(2), balance)
	// bob has 0 WETH.
	balance, err = dsl.View(weth.BalanceOf(bob.Address()))
	t.Require().NoError(err)
	t.Require().Equal(eth.ZeroWei, balance)

	// view
	// without address sender so failure
	_, err = dsl.View(weth.Transfer(bob.Address(), eth.OneEther))
	t.Require().Error(err)
	// with address, msg.sender set
	res, err := dsl.View(weth.Transfer(bob.Address(), eth.OneEther), txplan.WithSender(alice.Address()))
	t.Require().NoError(err)
	t.Require().True(res)

	// write
	// alice sends bob 1 WETH
	dsl.Write(alice, weth.Transfer(bob.Address(), eth.OneEther))

	// view
	// alice has 1 WETH
	balance, err = dsl.View(weth.BalanceOf(alice.Address()))
	t.Require().NoError(err)
	t.Require().Equal(eth.OneEther, balance)
	// bob has 1 WETH.
	balance, err = dsl.View(weth.BalanceOf(bob.Address()))
	t.Require().NoError(err)
	t.Require().Equal(eth.OneEther, balance)
}
