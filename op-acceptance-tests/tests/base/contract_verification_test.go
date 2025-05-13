package base

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/op-devstack/devtest"
	"github.com/ethereum-optimism/optimism/op-devstack/dsl"
	"github.com/ethereum-optimism/optimism/op-devstack/presets"
	"github.com/ethereum-optimism/optimism/op-devstack/stack/match"
)

// TestContractVerification ensures all deployed contracts are verified against their artifacts.
func TestContractVerification(gt *testing.T) {
	t := devtest.SerialT(gt)
	sys := presets.NewSimpleInterop(t)

	// TODO: Add L1 contract verification when deployment info is available.
	// verifyL1Contracts(t, sys.L1Network)

	for _, l2 := range sys.L2Networks() {
		verifyL2Contracts(t, l2)
	}
}

// func verifyL1Contracts(t devtest.T, l1 *dsl.L1Network) {
// 	// TODO: Implement L1 contract verification when deployment info is available.
// }

func verifyL2Contracts(t devtest.T, l2 *dsl.L2Network) {
	stackL2 := l2.Escape()
	el := stackL2.L2ELNode(match.FirstL2EL)
	ethCl := el.EthClient()
	ctx := context.Background()

	blockRef, err := ethCl.BlockRefByLabel(ctx, "latest")
	require.NoError(t, err, "failed to get latest block ref")

	coreContracts := map[string]common.Address{
		"SystemConfigProxy":       stackL2.Deployment().SystemConfigProxyAddr(),
		"DisputeGameFactoryProxy": stackL2.Deployment().DisputeGameFactoryProxyAddr(),
		// Add more as needed
	}
	for name, addr := range coreContracts {
		if addr == (common.Address{}) {
			continue // skip zero addresses
		}
		t.Logf("Verifying L2 contract %s at %s", name, addr.Hex())
		deployed, err := ethCl.CodeAtHash(ctx, addr, blockRef.Hash)
		require.NoError(t, err, "failed to fetch code for %s", name)
		artifactPath := findArtifactPath(name)
		artifactBytecode, err := loadDeployedBytecodeFromArtifact(artifactPath)
		require.NoError(t, err, "failed to load artifact for %s", name)
		match := compareBytecode(deployed, artifactBytecode)
		require.True(t, match, "bytecode mismatch for %s at %s", name, addr.Hex())
	}
}

// findArtifactPath returns the path to the artifact JSON for a contract name.
func findArtifactPath(contractName string) string {
	// TODO: Implement logic to map contract name to artifact path.
	// This may require a mapping or convention based on contractName.
	return filepath.Join("../../..", "packages/contracts-bedrock/forge-artifacts", contractName+".sol", contractName+".json")
}

// loadDeployedBytecodeFromArtifact loads the deployed bytecode from a Foundry artifact JSON file.
func loadDeployedBytecodeFromArtifact(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var artifact struct {
		DeployedBytecode struct {
			Object string `json:"object"`
		} `json:"deployedBytecode"`
	}
	if err := json.NewDecoder(f).Decode(&artifact); err != nil {
		return nil, err
	}
	return common.FromHex(artifact.DeployedBytecode.Object), nil
}

// compareBytecode compares two bytecode slices, allowing for metadata differences if needed.
func compareBytecode(onchain, artifact []byte) bool {
	// TODO: Implement normalization/stripping of metadata if needed.
	return strings.EqualFold(fmt.Sprintf("%x", onchain), fmt.Sprintf("%x", artifact))
}
