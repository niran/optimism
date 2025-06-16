package sysgo

import (
	"github.com/ethereum-optimism/optimism/op-devstack/shim"
	"github.com/ethereum-optimism/optimism/op-devstack/stack"
	"github.com/ethereum-optimism/optimism/op-interop-mon/metrics"
	"github.com/ethereum-optimism/optimism/op-interop-mon/monitor"
)

type InteropMonitorService struct {
	metricsEndpoint string
	service         *monitor.InteropMonitorService
}

func (m *InteropMonitorService) Metrics() metrics.Metricer {
	return m.service.Metrics
}

func (m *InteropMonitorService) hydrate(system stack.ExtensibleSystem) {
	frontend := shim.NewInteropMonitor(shim.InteropMonitorConfig{
		CommonConfig:    shim.NewCommonConfig(system.T()),
		ID:              stack.InteropMonitorID("interop-mon"),
		MetricsEndpoint: m.metricsEndpoint,
	})
	system.AddInteropMonitor(frontend)
	m.metricsEndpoint = m.service.MetricsEndpoint
}

func WithInteropMonitor(elids ...stack.L2ELNodeID) stack.Option[*Orchestrator] {
	return stack.AfterDeploy(func(orch *Orchestrator) {
		interopMonID := stack.InteropMonitorID("interop-mon") // This is a singleton for now.
		p := orch.P().WithCtx(stack.ContextWithID(orch.P().Ctx(), interopMonID))
		require := p.Require()
		cliConfig := &monitor.CLIConfig{}
		for _, elid := range elids {
			l2EL, ok := orch.l2ELs.Get(elid)
			require.True(ok)
			cliConfig.L2Rpcs = append(cliConfig.L2Rpcs, l2EL.userRPC)
		}
		cliConfig.MetricsConfig.Enabled = true
		service, err := monitor.InteropMonitorServiceFromCLIConfig(
			p.Ctx(),
			"test",
			cliConfig,
			p.Logger(),
		)
		require.NoError(err)
		orch.interopMon = &InteropMonitorService{service: service}
		orch.interopMon.service.Start(p.Ctx())
	})
}
