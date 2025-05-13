package db

import (
	"path/filepath"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/op-node/rollup/event"
	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-service/testlog"
	"github.com/ethereum-optimism/optimism/op-service/testutils"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/activation"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/db/fromda"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/db/logs"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/depset"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/superevents"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

// stubMetrics for tracking metrics in tests
type stubMetrics struct {
	dbEntryCount         int64
	entriesReadForSearch int64
	derivedEntryCount    int64
}

func (s *stubMetrics) RecordDBEntryCount(kind string, count int64) {
	s.dbEntryCount = count
}

func (s *stubMetrics) RecordDBSearchEntriesRead(count int64) {
	s.entriesReadForSearch = count
}

func (s *stubMetrics) RecordDBDerivedEntryCount(count int64) {
	s.derivedEntryCount = count
}

func (s *stubMetrics) RecordCrossUnsafeRef(chainID eth.ChainID, ref eth.BlockRef) {}
func (s *stubMetrics) RecordCrossSafeRef(chainID eth.ChainID, ref eth.BlockRef)   {}

// testDependencySet creates a dependency set with the specified chains and activation times
func testDependencySet(chainIndices []uint64, activationTimes map[uint64]uint64) depset.DependencySet {
	deps := make(map[eth.ChainID]*depset.StaticConfigDependency)
	for _, idx := range chainIndices {
		chainID := eth.ChainIDFromUInt64(idx)
		activation := uint64(0)
		if activationTimes != nil {
			activation = activationTimes[idx]
		}
		deps[chainID] = &depset.StaticConfigDependency{
			ChainIndex:     types.ChainIndex(idx),
			ActivationTime: activation,
		}
	}

	depSet, err := depset.NewStaticConfigDependencySet(deps)
	if err != nil {
		panic(err)
	}
	return depSet
}

// setupRealDatabases creates real database instances for testing
func setupRealDatabases(t *testing.T, chainID eth.ChainID) (*logs.DB, *fromda.DB, *fromda.DB) {
	tmpDir := t.TempDir()
	logger := testlog.Logger(t, log.LvlInfo)
	metrics := &stubMetrics{}

	// Create real LogStorage
	logDbPath := filepath.Join(tmpDir, "logs.db")
	logStorage, err := logs.NewFromFile(logger, metrics, chainID, logDbPath, true)
	require.NoError(t, err, "Failed to create log storage")
	t.Cleanup(func() {
		logStorage.Close()
	})

	// Create real local DerivationDB
	localDbPath := filepath.Join(tmpDir, "local_safe.db")
	localDB, err := fromda.NewFromFile(logger, metrics, localDbPath)
	require.NoError(t, err, "Failed to create local derivation storage")
	t.Cleanup(func() {
		localDB.Close()
	})

	// Create real cross DerivationDB
	crossDbPath := filepath.Join(tmpDir, "cross_safe.db")
	crossDB, err := fromda.NewFromFile(logger, metrics, crossDbPath)
	require.NoError(t, err, "Failed to create cross derivation storage")
	t.Cleanup(func() {
		crossDB.Close()
	})

	return logStorage, localDB, crossDB
}

// setupTestChainDB creates a ChainsDB with real database instances for testing
func setupTestChainDB(t *testing.T, chainIndices []uint64, activationTimes map[uint64]uint64) (*ChainsDB, eth.ChainID, *testutils.MockEmitter) {
	logger := testlog.Logger(t, log.LvlInfo)
	depSet := testDependencySet(chainIndices, activationTimes)
	metrics := &stubMetrics{}

	db := NewChainsDB(logger, depSet, metrics)
	chainID := eth.ChainIDFromUInt64(chainIndices[0])

	// Set up real databases
	logStorage, localDB, crossDB := setupRealDatabases(t, chainID)
	db.AddLogDB(chainID, logStorage)
	db.AddLocalDerivationDB(chainID, localDB)
	db.AddCrossDerivationDB(chainID, crossDB)
	db.AddCrossUnsafeTracker(chainID)

	// Create and attach the mock emitter
	emitter := new(testutils.MockEmitter)
	db.AttachEmitter(emitter)

	return db, chainID, emitter
}

// TestRewindToEmpty tests the basic RewindToEmpty functionality
func TestRewindToEmpty(t *testing.T) {
	chainIndices := []uint64{1}
	db, chainID, _ := setupTestChainDB(t, chainIndices, nil)

	// Test rewinding to empty
	err := db.RewindToEmpty(chainID)
	require.NoError(t, err, "RewindToEmpty should not return an error")

	// Verify that the database is properly reset by checking initialization status
	require.False(t, db.isInitialized(chainID), "Database should not be initialized after RewindToEmpty")

	// Additional verification: try to get latest sealed block from logs DB
	logStorage, ok := db.logDBs.Get(chainID)
	require.True(t, ok, "Should have log storage for chain")
	latestBlock, ok := logStorage.LatestSealedBlock()
	require.False(t, ok, "LogsDB should be empty after RewindToEmpty")
	require.Equal(t, eth.BlockID{}, latestBlock, "LatestSealedBlock should return zero block after RewindToEmpty")
}

func TestActivationFullFlow(t *testing.T) {
	chainIndices := []uint64{1}
	activationTime := uint64(1000)
	activationTimes := map[uint64]uint64{
		1: activationTime,
	}

	db, chainID, emitter := setupTestChainDB(t, chainIndices, activationTimes)

	// Create a pre-activation block and add it to the database
	preActivationBlock := eth.BlockRef{
		Hash:   common.HexToHash("0x1111"),
		Number: 99,
		Time:   activationTime - 10,
	}

	// Create an activation block
	activationBlock := eth.BlockRef{
		Hash:       common.HexToHash("0x2222"),
		Number:     100,
		ParentHash: preActivationBlock.Hash,
		Time:       activationTime,
	}

	// We need to make sure we match both LocalUnsafeUpdateEvent events
	// One from handleInteropActivation directly and one from initFromAnchor's call to initializedSealBlock
	emitter.On("Emit", mock.MatchedBy(func(ev event.Event) bool {
		_, ok := ev.(superevents.LocalUnsafeUpdateEvent)
		return ok
	})).Twice()

	// ChainRewoundEvent from handleInteropActivation
	emitter.On("Emit", mock.MatchedBy(func(ev event.Event) bool {
		if e, ok := ev.(superevents.ChainRewoundEvent); ok {
			return e.ChainID == chainID
		}
		return false
	})).Once()

	// CrossSafeUpdateEvent from initFromAnchor's call to initializedUpdateCrossSafe
	emitter.On("Emit", mock.MatchedBy(func(ev event.Event) bool {
		_, ok := ev.(superevents.CrossSafeUpdateEvent)
		return ok
	})).Once()

	// LocalSafeUpdateEvent from initFromAnchor's call to initializedUpdateLocalSafe
	emitter.On("Emit", mock.MatchedBy(func(ev event.Event) bool {
		_, ok := ev.(superevents.LocalSafeUpdateEvent)
		return ok
	})).Once()

	// Create the activation event
	activationEvent := superevents.InteropActivationEvent{
		ChainID:         chainID,
		ActivationBlock: activationBlock,
		PreviousBlock:   preActivationBlock,
	}

	// Test that the event is handled
	handled := db.OnEvent(activationEvent)
	require.True(t, handled, "Event should have been handled")

	// Verify events were emitted
	emitter.AssertExpectations(t)

	// Verify database was reset and reinitialized
	require.True(t, db.isInitialized(chainID), "Database should be initialized after handling activation event")
}

// TestActivationDetection tests that the activation detection works properly
// and correctly generates activation events at the right time
func TestActivationDetection(t *testing.T) {
	logger := testlog.Logger(t, log.LvlInfo)
	activationTime := uint64(1000)
	chainID := eth.ChainIDFromUInt64(1)

	// Create dependency set with activation time
	deps := map[eth.ChainID]*depset.StaticConfigDependency{
		chainID: {
			ChainIndex:     types.ChainIndex(1),
			ActivationTime: activationTime,
		},
	}

	depSet, err := depset.NewStaticConfigDependencySet(deps)
	require.NoError(t, err)

	// Create activation checker
	activationCheck := activation.NewCheck(depSet, logger)

	// Create blocks for testing
	preActivationBlock := eth.BlockRef{
		Hash:   common.HexToHash("0x1111"),
		Number: 99,
		Time:   activationTime - 10,
	}

	// The CanExecuteAt function checks for timestamp > activationTime (not >=)
	// So we need to make sure our "exact" block is at activationTime + 1
	exactActivationBlock := eth.BlockRef{
		Hash:       common.HexToHash("0x2222"),
		Number:     100,
		ParentHash: preActivationBlock.Hash,
		Time:       activationTime + 1, // +1 because execution is active when timestamp > activationTime
	}

	postActivationBlock := eth.BlockRef{
		Hash:       common.HexToHash("0x3333"),
		Number:     101,
		ParentHash: exactActivationBlock.Hash,
		Time:       activationTime + 10,
	}

	// Test checking individual blocks
	isPreActive := activationCheck.Check(chainID, preActivationBlock.Time)
	require.False(t, isPreActive, "Block before activation time should not be active")

	// This should now pass because we're using activationTime + 1
	isExactActive := activationCheck.Check(chainID, exactActivationBlock.Time)
	require.True(t, isExactActive, "Block at activation time should be active")

	isPostActive := activationCheck.Check(chainID, postActivationBlock.Time)
	require.True(t, isPostActive, "Block after activation time should be active")

	// Test activation detection (transition point)
	isPreActivation := activationCheck.IsActivationBlock(preActivationBlock, eth.BlockRef{}, chainID)
	require.False(t, isPreActivation, "Pre-activation block should not be detected as activation block")

	isExactActivation := activationCheck.IsActivationBlock(exactActivationBlock, preActivationBlock, chainID)
	require.True(t, isExactActivation, "Exact activation block should be detected as activation block")

	isPostActivation := activationCheck.IsActivationBlock(postActivationBlock, exactActivationBlock, chainID)
	require.False(t, isPostActivation, "Post-activation block following activation block should not be detected as activation")
}

// TestActivationRewindToEmptyAndReInitialize tests full flow
func TestActivationRewindToEmptyAndReInitialize(t *testing.T) {
	logger := testlog.Logger(t, log.LvlInfo)
	depSet := testDependencySet([]uint64{1}, nil)
	metrics := &stubMetrics{}

	db := NewChainsDB(logger, depSet, metrics)
	chainID := eth.ChainIDFromUInt64(1)

	// Set up real databases
	logStorage, localDB, crossDB := setupRealDatabases(t, chainID)
	db.AddLogDB(chainID, logStorage)
	db.AddLocalDerivationDB(chainID, localDB)
	db.AddCrossDerivationDB(chainID, crossDB)
	db.AddCrossUnsafeTracker(chainID)

	// Create mock emitter
	emitter := new(testutils.MockEmitter)
	db.AttachEmitter(emitter)

	// Add some data to the log database to verify reset works
	blockID := eth.BlockID{
		Hash:   common.HexToHash("0xabc123"),
		Number: 10,
	}

	// Seal a block in the log database
	err := logStorage.SealBlock(common.Hash{}, blockID, 1000)
	require.NoError(t, err, "Should be able to seal block")

	// Mark database as initialized
	db.initialized.Set(chainID, struct{}{})

	// Create activation block
	activationBlock := eth.BlockRef{
		Hash:       common.HexToHash("0x1234"),
		Number:     100,
		ParentHash: common.HexToHash("0xabcd"),
		Time:       1000,
	}

	// We need to make sure we match both LocalUnsafeUpdateEvent events
	// One from handleInteropActivation directly and one from initFromAnchor's call to initializedSealBlock
	emitter.On("Emit", mock.MatchedBy(func(ev event.Event) bool {
		_, ok := ev.(superevents.LocalUnsafeUpdateEvent)
		return ok
	})).Twice()

	// ChainRewoundEvent from handleInteropActivation
	emitter.On("Emit", mock.MatchedBy(func(ev event.Event) bool {
		if e, ok := ev.(superevents.ChainRewoundEvent); ok {
			return e.ChainID == chainID
		}
		return false
	})).Once()

	// CrossSafeUpdateEvent from initFromAnchor's call to initializedUpdateCrossSafe
	emitter.On("Emit", mock.MatchedBy(func(ev event.Event) bool {
		_, ok := ev.(superevents.CrossSafeUpdateEvent)
		return ok
	})).Once()

	// LocalSafeUpdateEvent from initFromAnchor's call to initializedUpdateLocalSafe
	emitter.On("Emit", mock.MatchedBy(func(ev event.Event) bool {
		_, ok := ev.(superevents.LocalSafeUpdateEvent)
		return ok
	})).Once()

	// Test rewinding to empty and reinitializing
	db.handleInteropActivation(chainID, activationBlock)

	// Verify events were emitted
	emitter.AssertExpectations(t)

	// Verify that the database was properly reset and reinitialized
	require.True(t, db.isInitialized(chainID), "Database should be initialized after handleInteropActivation")
}

// TestActivationRewindToEmpty tests that the database correctly handles activation
// by rewinding to empty and reinitializing with the activation block as the anchor
func TestActivationRewindToEmpty(t *testing.T) {
	logger := testlog.Logger(t, log.LvlInfo)
	depSet := testDependencySet([]uint64{1}, nil)
	metrics := &stubMetrics{}

	db := NewChainsDB(logger, depSet, metrics)
	chainID := eth.ChainIDFromUInt64(1)

	// Set up real databases
	logStorage, localDB, crossDB := setupRealDatabases(t, chainID)
	db.AddLogDB(chainID, logStorage)
	db.AddLocalDerivationDB(chainID, localDB)
	db.AddCrossDerivationDB(chainID, crossDB)
	db.AddCrossUnsafeTracker(chainID)

	// Add some data to the log database to verify reset works
	blockID := eth.BlockID{
		Hash:   common.HexToHash("0xabc123"),
		Number: 10,
	}

	// Seal a block in the log database
	err := logStorage.SealBlock(common.Hash{}, blockID, 1000)
	require.NoError(t, err, "Should be able to seal block")

	// Verify the block was added
	latestBlock, ok := logStorage.LatestSealedBlock()
	require.True(t, ok, "Should have a latest sealed block")
	require.Equal(t, blockID, latestBlock, "Latest sealed block should match what we added")

	// Directly test RewindToEmpty
	err = db.RewindToEmpty(chainID)
	require.NoError(t, err, "RewindToEmpty should not error")

	// Verify the database was reset by checking for the latest sealed block
	latestBlock, ok = logStorage.LatestSealedBlock()
	require.False(t, ok, "Should not have a latest sealed block after reset")
	require.Equal(t, eth.BlockID{}, latestBlock, "Latest sealed block should be empty after reset")

	// Verify that the database is not initialized after reset
	require.False(t, db.isInitialized(chainID), "Database should not be initialized after reset")
}

// TestActivationAnchorInitialization tests that after resetting the database,
// it can be properly reinitialized with the activation block as the anchor point
func TestActivationAnchorInitialization(t *testing.T) {
	logger := testlog.Logger(t, log.LvlInfo)
	depSet := testDependencySet([]uint64{1}, nil)
	metrics := &stubMetrics{}

	db := NewChainsDB(logger, depSet, metrics)
	chainID := eth.ChainIDFromUInt64(1)

	// Set up real databases
	logStorage, localDB, crossDB := setupRealDatabases(t, chainID)
	db.AddLogDB(chainID, logStorage)
	db.AddLocalDerivationDB(chainID, localDB)
	db.AddCrossDerivationDB(chainID, crossDB)
	db.AddCrossUnsafeTracker(chainID)

	// Mark database as initialized
	db.initialized.Set(chainID, struct{}{})

	// Create activation block
	activationBlock := eth.BlockRef{
		Hash:       common.HexToHash("0x1234"),
		Number:     100,
		ParentHash: common.HexToHash("0xabcd"),
		Time:       1000,
	}

	// First manually trigger reset
	err := db.RewindToEmpty(chainID)
	require.NoError(t, err, "RewindToEmpty should not error")

	// Database should no longer be initialized
	require.False(t, db.isInitialized(chainID), "Database should not be initialized after reset")

	// Create anchor point
	anchor := types.DerivedBlockRefPair{
		Source:  activationBlock,
		Derived: activationBlock,
	}

	// Set up mock expectations for CrossSafeUpdateEvent
	emitter := new(testutils.MockEmitter)
	db.AttachEmitter(emitter)

	// The initFromAnchor will emit CrossSafeUpdateEvent, LocalSafeUpdateEvent, and LocalUnsafeUpdateEvent
	emitter.On("Emit", mock.MatchedBy(func(ev event.Event) bool {
		_, ok := ev.(superevents.CrossSafeUpdateEvent)
		return ok
	})).Once()

	emitter.On("Emit", mock.MatchedBy(func(ev event.Event) bool {
		_, ok := ev.(superevents.LocalSafeUpdateEvent)
		return ok
	})).Once()

	emitter.On("Emit", mock.MatchedBy(func(ev event.Event) bool {
		_, ok := ev.(superevents.LocalUnsafeUpdateEvent)
		return ok
	})).Once()

	// Initialize from anchor
	db.initFromAnchor(chainID, anchor)

	// Verify database is now initialized
	require.True(t, db.isInitialized(chainID), "Database should be initialized after anchor initialization")

	// Verify events were emitted
	emitter.AssertExpectations(t)

	// We cannot directly test AddDerived since it's an interface method on DerivationStorage
	// and we would need to manually add to both databases
}

// TestHandleInteropActivation tests that handleInteropActivation method
// properly resets all databases and forwards events
func TestHandleInteropActivation(t *testing.T) {
	chainIndices := []uint64{1}
	db, chainID, emitter := setupTestChainDB(t, chainIndices, nil)

	// We need to make sure we match all the emitted events
	// Two LocalUnsafeUpdateEvent, one ChainRewoundEvent, one CrossSafeUpdateEvent, one LocalSafeUpdateEvent
	emitter.On("Emit", mock.MatchedBy(func(ev event.Event) bool {
		_, ok := ev.(superevents.LocalUnsafeUpdateEvent)
		return ok
	})).Twice()

	emitter.On("Emit", mock.MatchedBy(func(ev event.Event) bool {
		if e, ok := ev.(superevents.ChainRewoundEvent); ok {
			return e.ChainID == chainID
		}
		return false
	})).Once()

	// The initFromAnchor will emit CrossSafeUpdateEvent and LocalSafeUpdateEvent
	emitter.On("Emit", mock.MatchedBy(func(ev event.Event) bool {
		_, ok := ev.(superevents.CrossSafeUpdateEvent)
		return ok
	})).Once()

	emitter.On("Emit", mock.MatchedBy(func(ev event.Event) bool {
		_, ok := ev.(superevents.LocalSafeUpdateEvent)
		return ok
	})).Once()

	// Create an activation block
	activationBlock := eth.BlockRef{
		Hash:       common.HexToHash("0x1234"),
		Number:     100,
		ParentHash: common.HexToHash("0xabcd"),
		Time:       1000,
	}

	// Mark database as initialized
	db.initialized.Set(chainID, struct{}{})

	// Test handling interop activation
	db.handleInteropActivation(chainID, activationBlock)

	// Verify database was reset and reinitialized
	require.True(t, db.isInitialized(chainID), "Database should be initialized after handleInteropActivation")

	// Verify events were emitted
	emitter.AssertExpectations(t)
}

// TestOnEventInteropActivation tests the OnEvent method with an InteropActivationEvent
func TestOnEventInteropActivation(t *testing.T) {
	chainIndices := []uint64{1}
	db, chainID, emitter := setupTestChainDB(t, chainIndices, nil)

	// We need to make sure we match both LocalUnsafeUpdateEvent events
	// One from handleInteropActivation directly and one from initFromAnchor's call to initializedSealBlock
	emitter.On("Emit", mock.MatchedBy(func(ev event.Event) bool {
		_, ok := ev.(superevents.LocalUnsafeUpdateEvent)
		return ok
	})).Twice()

	// ChainRewoundEvent from handleInteropActivation
	emitter.On("Emit", mock.MatchedBy(func(ev event.Event) bool {
		if e, ok := ev.(superevents.ChainRewoundEvent); ok {
			return e.ChainID == chainID
		}
		return false
	})).Once()

	// CrossSafeUpdateEvent from initFromAnchor's call to initializedUpdateCrossSafe
	emitter.On("Emit", mock.MatchedBy(func(ev event.Event) bool {
		_, ok := ev.(superevents.CrossSafeUpdateEvent)
		return ok
	})).Once()

	// LocalSafeUpdateEvent from initFromAnchor's call to initializedUpdateLocalSafe
	emitter.On("Emit", mock.MatchedBy(func(ev event.Event) bool {
		_, ok := ev.(superevents.LocalSafeUpdateEvent)
		return ok
	})).Once()

	// Create an activation block
	activationBlock := eth.BlockRef{
		Hash:       common.HexToHash("0x1234"),
		Number:     100,
		ParentHash: common.HexToHash("0xabcd"),
		Time:       1000,
	}

	previousBlock := eth.BlockRef{
		Hash:       common.HexToHash("0xabcd"),
		Number:     99,
		ParentHash: common.HexToHash("0x9876"),
		Time:       990,
	}

	// Create the activation event
	activationEvent := superevents.InteropActivationEvent{
		ChainID:         chainID,
		ActivationBlock: activationBlock,
		PreviousBlock:   previousBlock,
	}

	// Test that the event is handled
	handled := db.OnEvent(activationEvent)
	require.True(t, handled, "Event should have been handled")

	// Verify events were emitted
	emitter.AssertExpectations(t)

	// Verify the database was properly reset and reinitialized
	require.True(t, db.isInitialized(chainID), "Database should be initialized after handling activation event")
}

// Custom event for testing unhandled events
type customEvent struct {
	name string
}

func (e customEvent) String() string {
	return e.name
}

// TestOnEventUnhandledEvent tests that unrecognized events are not handled
func TestOnEventUnhandledEvent(t *testing.T) {
	chainIndices := []uint64{1}
	db, _, _ := setupTestChainDB(t, chainIndices, nil)

	// Create a different type of event
	unhandledEvent := customEvent{name: "UnhandledEvent"}

	// Test that the event is not handled
	handled := db.OnEvent(unhandledEvent)
	require.False(t, handled, "Event should not have been handled")
}
