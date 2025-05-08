package presets

import (
	"fmt"
	"os"
	"runtime/debug"
	"sync/atomic"
	"testing"

	"github.com/ethereum/go-ethereum/log"

	"github.com/ethereum-optimism/optimism/devnet-sdk/devstack/devtest"
	"github.com/ethereum-optimism/optimism/devnet-sdk/devstack/stack"
	"github.com/ethereum-optimism/optimism/devnet-sdk/devstack/sysext"
	"github.com/ethereum-optimism/optimism/devnet-sdk/devstack/sysgo"
	"github.com/ethereum-optimism/optimism/op-service/locks"
	oplog "github.com/ethereum-optimism/optimism/op-service/log"
)

// lockedOrchestrator is the global variable that stores
// the global orchestrator that tests may use.
// Presets are expected to use the global orchestrator,
// unless explicitly told otherwise using a WithOrchestrator option.
var lockedOrchestrator locks.RWValue[stack.Orchestrator]

// DoMain runs M with the pre- and post-processing of tests,
// to setup the default global orchestrator and global logger.
// This will os.Exit(code) and not return.
func DoMain(m *testing.M, opts ...stack.Option) {
	// nest the function, so we can defer-recover and defer-cleanup, before os.Exit
	code := func() (errCode int) {
		failed := new(atomic.Bool)
		defer func() {
			if failed.Load() {
				errCode = 1
			}
		}()
		defer func() {
			if x := recover(); x != nil {
				_, _ = fmt.Fprintf(os.Stderr, "Panic during test Main: %v\n", x)
				failed.Store(true)
			}
		}()

		// This may be tuned with env or CLI flags in the future, to customize test output
		logger := oplog.NewLogger(os.Stdout, oplog.CLIConfig{
			Level:  log.LevelInfo,
			Color:  true,
			Format: oplog.FormatTerminal,
			Pid:    false,
		})
		p := devtest.NewP(logger, func() {
			debug.PrintStack()
			failed.Store(true)
			panic("setup fail")
		})
		defer p.Close()

		p.Require().NotEmpty(opts, "Expecting orchestrator options")

		// For the global geth logs,
		// capture them in the global test logger.
		// No other tool / test should change the global logger.
		// TODO(#15139): set log-level filter, reduce noise
		//log.SetDefault(t.Log.New("logger", "global"))

		initGlobalOrchestrator(p, opts...)

		errCode = m.Run()
		return
	}()
	_, _ = fmt.Fprintf(os.Stderr, "\nExiting, code: %d\n", code)
	os.Exit(code)
}

func initGlobalOrchestrator(p devtest.P, opts ...stack.Option) {
	lockedOrchestrator.Lock()
	defer lockedOrchestrator.Unlock()
	if lockedOrchestrator.Value != nil {
		return
	}
	lockedOrchestrator.Value = InitOrchestrator(p, opts...)
}

func InitOrchestrator(p devtest.P, opts ...stack.Option) stack.Orchestrator {
	var orch stack.Orchestrator
	kind, ok := os.LookupEnv("DEVSTACK_ORCHESTRATOR")
	if !ok {
		p.Logger().Warn("Selecting sysgo as default devstack orchestrator")
		kind = "sysgo"
	}
	switch kind {
	case "sysgo":
		orch = sysgo.NewOrchestrator(p)
	case "syskt":
		orch = sysext.NewOrchestrator(p)
	default:
		p.Logger().Crit("Unknown devstack backend", "kind", kind)
	}
	for _, opt := range opts {
		opt(orch)
	}
	return orch
}

func InitDefaultOrchestrator(opts ...stack.Option) stack.Orchestrator {
	logger := oplog.NewLogger(os.Stdout, oplog.CLIConfig{
		Level:  log.LevelInfo,
		Color:  true,
		Format: oplog.FormatTerminal,
		Pid:    false,
	})
	p := devtest.NewP(logger, func() {
		debug.PrintStack()
	})
	return InitOrchestrator(p, opts...)
}

// Orchestrator returns the globally configured orchestrator.
//
// Add a TestMain to your test package init the orchestrator:
//
//	func TestMain(m *testing.M) {
//	    presets.DoMain(m)
//	}
func GlobalOrchestrator() stack.Orchestrator {
	out := lockedOrchestrator.Get()
	if out == nil {
		panic(`
Add a TestMain to your test package init the global orchestrator:

	func TestMain(m *testing.M) {
		presets.DoMain(m)
	}
`)
	}
	return out
}
