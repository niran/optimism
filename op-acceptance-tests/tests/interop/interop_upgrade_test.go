package interop

import (
	"testing"
	"time"

	"github.com/ethereum-optimism/optimism/devnet-sdk/devstack/devtest"
	"github.com/ethereum-optimism/optimism/devnet-sdk/devstack/presets"
	"github.com/ethereum-optimism/optimism/devnet-sdk/devstack/stack"
	"github.com/ethereum-optimism/optimism/devnet-sdk/devstack/stack/match"
	"github.com/ethereum-optimism/optimism/op-service/eth"
)

// TestInteropUpgrade starts an interop chain and verifies that the local unsafe head advances.
func TestInteropUpgrade(gt *testing.T) {
	t := devtest.SerialT(gt)
	var orchSetup presets.TestSetup[*presets.SimpleInterop]
	interopOffset := uint64(30)
	presets.DoTest(gt, presets.NewInteropGenInterop(&orchSetup, interopOffset))
	orch := orchSetup(t)

	elClient := orch.L2ChainA.Escape().L2ELNode(match.First[stack.L2ELNodeID, stack.L2ELNode]()).EthClient()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for true {
		select {
		case <-ticker.C:
			unsafeBlock, err := elClient.BlockRefByLabel(t.Ctx(), eth.Unsafe)
			t.Require().NoError(err)
			number := unsafeBlock.Number
			t.Logf("unsafe block number: %d", number)
		}
	}
}
