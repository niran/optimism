package db

import (
	"path/filepath"
	"testing"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/db/fromda"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/db/logs"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/depset"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/require"
)

// Create hash consistent with test hashes
func createTestHash(i int) common.Hash {
	return common.Hash{byte(i)}
}

// NoopMetrics for the logs DB
type NoopMetrics struct{}

func (m *NoopMetrics) RecordDBEntryCount(kind string, count int64) {}
func (m *NoopMetrics) RecordDBSearchEntriesRead(count int64)       {}

// NoopChainMetrics for the derivation DB
type NoopChainMetrics struct{}
func (m *NoopChainMetrics) RecordDBDerivedEntryCount(count int64) {}

// setupChainsDB creates a ChainsDB instance with real database files in a temp directory
func setupChainsDB(t *testing.T) (*ChainsDB, eth.ChainID) {
	logger := testlog.Logger(t, log.LvlDebug)
	dbDir := t.TempDir()
	
	chainID := eth.ChainID{1}
	
	// Create a dependency set with the test chain
	depSet, err := depset.NewStaticConfigDependencySet(
		map[eth.ChainID]*depset.StaticConfigDependency{
			chainID: {
				ChainIndex:     0,
				ActivationTime: 0,
				HistoryMinTime: 0,
			},
		},
	)
	require.NoError(t, err)
	
	// Create a new ChainsDB
	db := NewChainsDB(logger, depSet, nil)

	// Set up the logs database
	logDB, err := logs.NewFromFile(logger, &NoopMetrics{}, chainID, filepath.Join(dbDir, "logs.db"), false)
	require.NoError(t, err)
	db.AddLogDB(chainID, logDB)
	
	// Set up the local derivation database
	localDB, err := fromda.NewFromFile(logger, &NoopChainMetrics{}, filepath.Join(dbDir, "local.db"))
	require.NoError(t, err)
	db.AddLocalDerivationDB(chainID, localDB)
	
	// Set up the cross derivation database
	crossDB, err := fromda.NewFromFile(logger, &NoopChainMetrics{}, filepath.Join(dbDir, "cross.db"))
	require.NoError(t, err)
	db.AddCrossDerivationDB(chainID, crossDB)
	
	// Set up cross tracker
	db.AddCrossUnsafeTracker(chainID)
	
	return db, chainID
}

func TestPreActivationMode(t *testing.T) {
	// Set up the test database
	db, chainID := setupChainsDB(t)
	
	// Make sure logDB is available
	_, ok := db.logDBs.Get(chainID)
	require.True(t, ok, "LogDB should be available")
	
	// Clean up at the end of the test
	t.Cleanup(func() {
		if logdb, ok := db.logDBs.Get(chainID); ok {
			_ = logdb.Close()
		}
	})
	
	// Create test block references
	blockRef := eth.BlockRef{
		Hash:       common.Hash{1, 2, 3},
		Number:     100,
		Time:       200,
		ParentHash: common.Hash{0, 0, 0},
	}
	
	// Initialize in pre-activation mode
	db.InitializePreActivation(chainID, blockRef)
	
	// Verify pre-activation state
	require.True(t, db.IsInPreActivationMode(chainID))
	require.True(t, db.IsInitialized(chainID))
	
	head, ok := db.GetPreActivationHead(chainID)
	require.True(t, ok)
	require.Equal(t, blockRef, head)
	
	// Check query methods in pre-activation mode
	// LocalUnsafe should return the head block
	localUnsafe, err := db.LocalUnsafe(chainID)
	require.NoError(t, err)
	require.Equal(t, blockRef.Hash, localUnsafe.Hash)
	require.Equal(t, blockRef.Number, localUnsafe.Number)
	require.Equal(t, blockRef.Time, localUnsafe.Timestamp)
	
	// CrossUnsafe should return the head block
	crossUnsafe, err := db.CrossUnsafe(chainID)
	require.NoError(t, err)
	require.Equal(t, blockRef.Hash, crossUnsafe.Hash)
	require.Equal(t, blockRef.Number, crossUnsafe.Number)
	require.Equal(t, blockRef.Time, crossUnsafe.Timestamp)
	
	// LocalSafe should return a self-derived pair with the head block
	localSafe, err := db.LocalSafe(chainID)
	require.NoError(t, err)
	require.Equal(t, blockRef.Hash, localSafe.Derived.Hash)
	require.Equal(t, blockRef.Number, localSafe.Derived.Number)
	require.Equal(t, blockRef.Time, localSafe.Derived.Timestamp)
	require.Equal(t, blockRef.Hash, localSafe.Source.Hash)
	require.Equal(t, blockRef.Number, localSafe.Source.Number)
	require.Equal(t, blockRef.Time, localSafe.Source.Timestamp)
	
	// CrossSafe should return a self-derived pair with the head block
	crossSafe, err := db.CrossSafe(chainID)
	require.NoError(t, err)
	require.Equal(t, blockRef.Hash, crossSafe.Derived.Hash)
	require.Equal(t, blockRef.Number, crossSafe.Derived.Number)
	require.Equal(t, blockRef.Time, crossSafe.Derived.Timestamp)
	require.Equal(t, blockRef.Hash, crossSafe.Source.Hash)
	require.Equal(t, blockRef.Number, crossSafe.Source.Number)
	require.Equal(t, blockRef.Time, crossSafe.Source.Timestamp)
	
	// Update the head in pre-activation mode
	newBlockRef := eth.BlockRef{
		Hash:       common.Hash{3, 2, 1},
		Number:     101,
		Time:       201,
		ParentHash: blockRef.Hash,
	}
	db.UpdatePreActivationHead(chainID, newBlockRef)
	
	// Verify the head was updated
	head, ok = db.GetPreActivationHead(chainID)
	require.True(t, ok)
	require.Equal(t, newBlockRef, head)
	
	// Verify queries return the updated head
	localUnsafe, err = db.LocalUnsafe(chainID)
	require.NoError(t, err)
	require.Equal(t, newBlockRef.Hash, localUnsafe.Hash)
	require.Equal(t, newBlockRef.Number, localUnsafe.Number)
	require.Equal(t, newBlockRef.Time, localUnsafe.Timestamp)
}