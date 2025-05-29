package loadtest2

import (
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ethereum-optimism/optimism/op-acceptance-tests/tests/interop/loadtest2/runner"
	"github.com/ethereum-optimism/optimism/op-acceptance-tests/tests/interop/loadtest2/schedule"
	"github.com/ethereum-optimism/optimism/op-devstack/devtest"
	"github.com/ethereum-optimism/optimism/op-devstack/dsl"
	"github.com/ethereum-optimism/optimism/op-devstack/presets"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"golang.org/x/net/context"
)

const timeoutSecondsVar = "NAT_LOADTEST_TIMEOUT"

func TestMain(m *testing.M) {
	presets.DoMain(m, presets.WithSimpleInterop())
}

func TestLoad(gt *testing.T) {
	if testing.Short() {
		gt.Skip("skipping load test in short mode")
	}
	t := devtest.SerialT(gt)
	sys := presets.NewSimpleInterop(t)

	timeout := time.Minute
	if timeoutStr, exists := os.LookupEnv(timeoutSecondsVar); exists {
		timeoutSeconds, err := strconv.ParseUint(timeoutStr, 10, 0)
		t.Require().NoError(err)
		timeout = time.Duration(timeoutSeconds) * time.Second
	}
	schedCtx, schedCancel := context.WithTimeout(t.Ctx(), timeout)
	defer schedCancel() // This call won't matter, the waitgroup below will block until after the timeout.

	var wg sync.WaitGroup
	defer wg.Wait()

	initMsgsBus := NewBus()

	l2ELA := sys.L2ChainA.PublicRPC()
	eventLoggerAddress := sys.FunderA.NewFundedEOA(eth.OneEther).DeployEventLogger()
	initiator := NewManyMsgsInitiator(t, initMsgsBus, dsl.NewFunder(sys.Wallet, sys.FaucetA, l2ELA), l2ELA, eventLoggerAddress)
	t.Require().True(initiator.SendTx(t.Ctx())) // Initialize the bus.
	constant := schedule.NewConstant(time.Second)
	wg.Add(1)
	go func() {
		defer wg.Done()
		constant.Start(schedCtx)
	}()
	initiatorRunner := runner.New(constant, initiator, 2)

	l2ELB := sys.L2ChainB.PublicRPC()
	relayer := NewValidRelayer(t, initMsgsBus, sys.FunderB, l2ELB, sys.Supervisor)
	blockTime := time.Duration(sys.L2ChainB.Escape().RollupConfig().BlockTime) * time.Second
	aimd := schedule.NewAIMD(20, blockTime)
	wg.Add(1)
	go func() {
		defer wg.Done()
		aimd.Start(schedCtx)
	}()
	relayerRunner := runner.New(aimd, relayer, 20)

	// We use t.Ctx() instead of schedCtx since the runners may call into DSL
	// functions that have require.NoError, which would cause the test to fail when schedCtx is canceled on timeout.
	wg.Add(1)
	go func() {
		defer wg.Done()
		initiatorRunner.Start(t.Ctx())
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		relayerRunner.Start(t.Ctx())
	}()

	for {
		info, txs, err := l2ELB.Escape().EthClient().InfoAndTxsByLabel(schedCtx, eth.Unsafe)
		if err != nil {
			if strings.Contains(err.Error(), context.DeadlineExceeded.Error()) {
				return
			}
			t.Require().NoError(err)
		}
		t.Require().NoError(err)
		t.Logf("GAS LIMIT: %d", info.GasLimit())
		t.Logf("GAS USED:  %d", info.GasUsed())
		t.Logf("NUM TXS:   %d", len(txs))
		select {
		case <-schedCtx.Done():
			return
		case <-time.After(blockTime):
		}
	}
}
