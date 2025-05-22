package depset

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/ethereum-optimism/optimism/op-node/params"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/stretchr/testify/require"
)

func TestDependencySet(t *testing.T) {
	t.Run("JSON serialization", func(t *testing.T) {
		testDependencySetSerialization(t, "json",
			func(depSet *StaticConfigDependencySet) ([]byte, error) { return json.Marshal(depSet) },
			func(data []byte, depSet *StaticConfigDependencySet) error { return json.Unmarshal(data, depSet) },
		)
	})

	t.Run("TOML serialization", func(t *testing.T) {
		testDependencySetSerialization(t, "toml",
			func(depSet *StaticConfigDependencySet) ([]byte, error) {
				var buf bytes.Buffer
				encoder := toml.NewEncoder(&buf)
				if err := encoder.Encode(depSet); err != nil {
					return nil, err
				}
				return buf.Bytes(), nil
			},
			func(data []byte, depSet *StaticConfigDependencySet) error {
				_, err := toml.Decode(string(data), depSet)
				return err
			},
		)
	})

	t.Run("invalid TOML", func(t *testing.T) {
		bad := []byte(`dependencies = { bad = 1 }`)
		var ds StaticConfigDependencySet
		_, err := toml.Decode(string(bad), &ds)
		require.Error(t, err)
	})

	t.Run("duplicate index", func(t *testing.T) {
		_, err := NewStaticConfigDependencySet(map[eth.ChainID]*StaticConfigDependency{
			eth.ChainIDFromUInt64(900): {ChainIndex: 1},
			eth.ChainIDFromUInt64(901): {ChainIndex: 1}, // duplicate
		})
		require.ErrorIs(t, err, errDuplicateChainIndex)
	})
}

func testDependencySetSerialization(
	t *testing.T,
	fileExt string,
	marshal func(*StaticConfigDependencySet) ([]byte, error),
	unmarshal func([]byte, *StaticConfigDependencySet) error,
) {
	d := path.Join(t.TempDir(), "tmp_dep_set."+fileExt)

	depSet, err := NewStaticConfigDependencySet(
		map[eth.ChainID]*StaticConfigDependency{
			eth.ChainIDFromUInt64(900): {
				ChainIndex:     900,
				ActivationTime: 42,
				HistoryMinTime: 100,
			},
			eth.ChainIDFromUInt64(901): {
				ChainIndex:     901,
				ActivationTime: 30,
				HistoryMinTime: 20,
			},
		})
	require.NoError(t, err)

	t.Run("DefaultExpiryWindow", func(t *testing.T) {
		data, err := marshal(depSet)
		require.NoError(t, err)

		require.NoError(t, os.WriteFile(d, data, 0o644))

		// For JSON, use the loader. For TOML, unmarshal directly
		var result DependencySet
		if fileExt == "json" {
			loader := &JSONDependencySetLoader{Path: d}
			result, err = loader.LoadDependencySet(context.Background())
			require.NoError(t, err)
		} else {
			fileData, err := os.ReadFile(d)
			require.NoError(t, err)

			var newDepSet StaticConfigDependencySet
			err = unmarshal(fileData, &newDepSet)
			require.NoError(t, err)
			result = &newDepSet
		}

		chainIDs := result.Chains()
		require.ElementsMatch(t, []eth.ChainID{
			eth.ChainIDFromUInt64(900),
			eth.ChainIDFromUInt64(901),
		}, chainIDs)

		require.Equal(t, uint64(params.MessageExpiryTimeSecondsInterop), result.MessageExpiryWindow())
		testChainCapabilities(t, result)
	})

	t.Run("CustomExpiryWindow", func(t *testing.T) {
		depSet.overrideMessageExpiryWindow = 15

		data, err := marshal(depSet)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(d, data, 0o644))

		var result DependencySet
		if fileExt == "json" {
			loader := &JSONDependencySetLoader{Path: d}
			result, err = loader.LoadDependencySet(context.Background())
			require.NoError(t, err)
		} else {
			fileData, err := os.ReadFile(d)
			require.NoError(t, err)

			var newDepSet StaticConfigDependencySet
			err = unmarshal(fileData, &newDepSet)
			require.NoError(t, err)
			result = &newDepSet
		}

		require.Equal(t, uint64(15), result.MessageExpiryWindow())
		testChainCapabilities(t, result)
	})

	t.Run("chain index round trip", func(t *testing.T) {
		id900 := eth.ChainIDFromUInt64(900)
		idx, _ := depSet.ChainIndexFromID(id900)
		idBack, _ := depSet.ChainIDFromIndex(idx)
		require.Equal(t, id900, idBack)

		_, err := depSet.ChainIndexFromID(eth.ChainIDFromUInt64(999))
		require.ErrorContains(t, err, "unknown chain")
	})

	t.Run("HasChain", func(t *testing.T) {
		require.True(t, depSet.HasChain(eth.ChainIDFromUInt64(900)))
		require.False(t, depSet.HasChain(eth.ChainIDFromUInt64(902)))
	})
}

func testChainCapabilities(t *testing.T, result DependencySet) {
	// Test chain 900
	v, err := result.CanExecuteAt(eth.ChainIDFromUInt64(900), 42)
	require.NoError(t, err)
	require.True(t, v)

	v, err = result.CanExecuteAt(eth.ChainIDFromUInt64(900), 41)
	require.NoError(t, err)
	require.False(t, v)

	v, err = result.CanInitiateAt(eth.ChainIDFromUInt64(900), 100)
	require.NoError(t, err)
	require.True(t, v)

	v, err = result.CanInitiateAt(eth.ChainIDFromUInt64(900), 99)
	require.NoError(t, err)
	require.False(t, v)

	v, err = result.HasCrossL2Inbox(eth.ChainIDFromUInt64(900), 42)
	require.NoError(t, err)
	require.True(t, v)

	v, err = result.HasCrossL2Inbox(eth.ChainIDFromUInt64(900), 41)
	require.NoError(t, err)
	require.False(t, v)

	// Test chain 901
	v, err = result.CanExecuteAt(eth.ChainIDFromUInt64(901), 30)
	require.NoError(t, err)
	require.True(t, v)

	v, err = result.CanExecuteAt(eth.ChainIDFromUInt64(901), 29)
	require.NoError(t, err)
	require.False(t, v)

	v, err = result.CanInitiateAt(eth.ChainIDFromUInt64(901), 20)
	require.NoError(t, err)
	require.True(t, v)

	v, err = result.CanInitiateAt(eth.ChainIDFromUInt64(901), 19)
	require.NoError(t, err)
	require.False(t, v)

	v, err = result.HasCrossL2Inbox(eth.ChainIDFromUInt64(901), 30)
	require.NoError(t, err)
	require.False(t, v, "should have activate CrossL2 with only one active chain")

	v, err = result.HasCrossL2Inbox(eth.ChainIDFromUInt64(901), 42)
	require.NoError(t, err)
	require.True(t, v, "should have activate CrossL2 when at least two chains active")

	v, err = result.HasCrossL2Inbox(eth.ChainIDFromUInt64(901), 41)
	require.NoError(t, err)
	require.False(t, v)

	v, err = result.HasCrossL2Inbox(eth.ChainIDFromUInt64(901), 29)
	require.NoError(t, err)
	require.False(t, v)

	// Test non-existent chain
	v, err = result.CanExecuteAt(eth.ChainIDFromUInt64(902), 100000)
	require.NoError(t, err)
	require.False(t, v, "902 not a dependency")

	v, err = result.CanInitiateAt(eth.ChainIDFromUInt64(902), 100000)
	require.NoError(t, err)
	require.False(t, v, "902 not a dependency")

	v, err = result.HasCrossL2Inbox(eth.ChainIDFromUInt64(902), 100000)
	require.NoError(t, err)
	require.False(t, v, "902 not a dependency")
}

func TestHasCrossL2InboxDeployed(t *testing.T) {
	requireNotDeployed := func(t *testing.T, depSet DependencySet, chainID eth.ChainID, l2BlockTime uint64) {
		deployed, err := depSet.HasCrossL2Inbox(chainID, l2BlockTime)
		require.NoError(t, err)
		require.False(t, deployed)
	}
	requireDeployed := func(t *testing.T, depSet DependencySet, chainID eth.ChainID, l2BlockTime uint64) {
		deployed, err := depSet.HasCrossL2Inbox(chainID, l2BlockTime)
		require.NoError(t, err)
		require.True(t, deployed)
	}
	t.Run("NotDeployedForDependencySetOf1", func(t *testing.T) {
		chainID := eth.ChainIDFromUInt64(900)
		depSet, err := NewStaticConfigDependencySet(
			map[eth.ChainID]*StaticConfigDependency{
				chainID: {
					ChainIndex:     900,
					ActivationTime: 42,
					HistoryMinTime: 100,
				},
			})
		require.NoError(t, err)
		requireNotDeployed(t, depSet, chainID, 0)
		requireNotDeployed(t, depSet, chainID, 41)
		requireNotDeployed(t, depSet, chainID, 42)
		requireNotDeployed(t, depSet, chainID, 98298248)
	})

	t.Run("DeployedWhenSecondChainActivates", func(t *testing.T) {
		chainID1 := eth.ChainIDFromUInt64(900)
		chainID2 := eth.ChainIDFromUInt64(901)
		depSet, err := NewStaticConfigDependencySet(
			map[eth.ChainID]*StaticConfigDependency{
				chainID1: {
					ChainIndex:     900,
					ActivationTime: 50,
					HistoryMinTime: 100,
				},
				chainID2: {
					ChainIndex:     901,
					ActivationTime: 42,
					HistoryMinTime: 100,
				},
			})
		require.NoError(t, err)
		requireNotDeployed(t, depSet, chainID1, 0)
		requireNotDeployed(t, depSet, chainID2, 0)

		requireNotDeployed(t, depSet, chainID1, 41)
		requireNotDeployed(t, depSet, chainID2, 41)

		requireNotDeployed(t, depSet, chainID1, 42)
		requireNotDeployed(t, depSet, chainID2, 42)

		requireNotDeployed(t, depSet, chainID1, 49)
		requireNotDeployed(t, depSet, chainID2, 49)

		requireDeployed(t, depSet, chainID1, 50)
		requireDeployed(t, depSet, chainID2, 50)
	})

	t.Run("NotDeployedUntilChainActivates", func(t *testing.T) {
		chainID1 := eth.ChainIDFromUInt64(900)
		chainID2 := eth.ChainIDFromUInt64(901)
		chainID3 := eth.ChainIDFromUInt64(902)
		depSet, err := NewStaticConfigDependencySet(
			map[eth.ChainID]*StaticConfigDependency{
				chainID1: {
					ChainIndex:     900,
					ActivationTime: 0,
					HistoryMinTime: 100,
				},
				chainID2: {
					ChainIndex:     901,
					ActivationTime: 0,
					HistoryMinTime: 100,
				},
				chainID3: {
					ChainIndex:     902,
					ActivationTime: 60,
					HistoryMinTime: 100,
				},
			})
		require.NoError(t, err)
		requireDeployed(t, depSet, chainID1, 0)
		requireDeployed(t, depSet, chainID2, 0)
		requireNotDeployed(t, depSet, chainID3, 0)

		requireDeployed(t, depSet, chainID1, 59)
		requireDeployed(t, depSet, chainID2, 59)
		requireNotDeployed(t, depSet, chainID3, 59)

		requireDeployed(t, depSet, chainID1, 60)
		requireDeployed(t, depSet, chainID2, 60)
		requireDeployed(t, depSet, chainID3, 60)
	})

	t.Run("DeployedAtGenesis", func(t *testing.T) {
		chainID1 := eth.ChainIDFromUInt64(900)
		chainID2 := eth.ChainIDFromUInt64(901)
		depSet, err := NewStaticConfigDependencySet(
			map[eth.ChainID]*StaticConfigDependency{
				chainID1: {
					ChainIndex:     900,
					ActivationTime: 0,
					HistoryMinTime: 100,
				},
				chainID2: {
					ChainIndex:     901,
					ActivationTime: 0,
					HistoryMinTime: 100,
				},
			})
		require.NoError(t, err)
		requireDeployed(t, depSet, chainID1, 0)
		requireDeployed(t, depSet, chainID2, 0)
	})
}
