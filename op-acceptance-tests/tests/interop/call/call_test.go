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
	sys := presets.NewSimpleInterop(t)

	alice := sys.FunderA.NewFundedEOA(eth.ThousandEther)
	bob := sys.FunderA.NewFundedEOA(eth.ThousandEther)

	wethAddr := common.HexToAddress("0x4200000000000000000000000000000000000006")
	// dsl prep
	// TODO: delegate this initialization to somewhere else
	balanceOf := func(addr common.Address) dsl.Call[eth.ETH] {
		return dsl.BalanceOfCall{Addr: addr}
	}
	transfer := func(dest common.Address, amount eth.ETH) dsl.Call[bool] {
		return dsl.TransferCall{Dest: dest, Amount: amount}
	}
	weth := dsl.WETH{BalanceOf: balanceOf, Transfer: transfer}

	// alice and bob has zero WETH
	t.Require().NotEqual(alice.Address(), bob.Address())
	t.Require().Equal(eth.ZeroWei, dsl.View(alice, wethAddr, weth.BalanceOf(alice.Address())))
	t.Require().Equal(eth.ZeroWei, dsl.View(bob, wethAddr, weth.BalanceOf(bob.Address())))

	// TODO: abstract away: alice wraps 1 eth to 1 WETH
	{
		val := eth.OneEther
		tx := txplan.NewPlannedTx(
			alice.Plan(),
			txplan.WithValue(val.ToBig()),
			txplan.WithTo(&wethAddr),
		)
		_, err := tx.Included.Eval(t.Ctx())
		t.Require().NoError(err, "tx inclusion error")
	}

	// view
	// alice has 1 WETH
	t.Require().Equal(eth.OneEther, dsl.View(alice, wethAddr, weth.BalanceOf(alice.Address())))
	// bob has 0 WETH.
	t.Require().Equal(eth.ZeroWei, dsl.View(bob, wethAddr, weth.BalanceOf(bob.Address())))

	// write
	// alice sends bob 1 WETH
	dsl.Write(alice, wethAddr, weth.Transfer(bob.Address(), eth.OneEther))

	// view
	// alice has 0 WETH
	t.Require().Equal(eth.ZeroWei, dsl.View(alice, wethAddr, weth.BalanceOf(alice.Address())))
	// bob has 1 WETH
	t.Require().Equal(eth.OneEther, dsl.View(bob, wethAddr, weth.BalanceOf(bob.Address())))
}
