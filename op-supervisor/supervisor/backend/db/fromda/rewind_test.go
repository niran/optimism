package fromda

import (
	"path/filepath"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

func TestFromDBRewindToEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	logger := testlog.Logger(t, log.LvlInfo)

	// Setup a test metrics stub
	metrics := &testMetrics{}

	// Create fromda database
	dbPath := filepath.Join(tmpDir, "fromda.db")
	db, err := NewFromFile(logger, metrics, dbPath)
	require.NoError(t, err, "Failed to create fromda database")
	t.Cleanup(func() {
		db.Close()
	})

	// Add some entries to the database
	sourceRef := eth.BlockRef{
		Hash:   common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000001"),
		Number: 100,
		Time:   1000,
	}

	derivedRef := eth.BlockRef{
		Hash:   common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000002"),
		Number: 200,
		Time:   2000,
	}

	// Add entry to the database using the AddDerived method
	err = db.AddDerived(sourceRef, derivedRef, FirstRevision)
	require.NoError(t, err, "Failed to add derived entry")

	// Check that data was added
	latestEntry, err := db.Last()
	require.NoError(t, err, "Failed to get latest entry")
	require.Equal(t, sourceRef.Hash, latestEntry.Source.Hash, "Latest source hash should match what we added")
	require.Equal(t, derivedRef.Hash, latestEntry.Derived.Hash, "Latest derived hash should match what we added")

	// Now rewind to empty
	err = db.RewindToEmpty()
	require.NoError(t, err, "RewindToEmpty should not error")

	// Verify the database is empty after rewind
	_, err = db.Last()
	require.Error(t, err, "Should error when getting latest entry after rewind")
	require.ErrorIs(t, err, types.ErrFuture, "Should get ErrFuture when database is empty")
}

// testMetrics is a simple metrics implementation for testing
type testMetrics struct {
	derivedEntryCount int64
}

// RecordDBDerivedEntryCount records the number of derived entries
func (m *testMetrics) RecordDBDerivedEntryCount(count int64) {
	m.derivedEntryCount = count
}
