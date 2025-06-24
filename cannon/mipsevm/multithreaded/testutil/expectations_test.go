package testutil

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ethereum-optimism/optimism/cannon/mipsevm"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/arch"
	"github.com/ethereum-optimism/optimism/cannon/mipsevm/multithreaded"
)

type ExpectationMutator func(e *ExpectedState, st *multithreaded.State)

func TestValidate_shouldCatchMutations(t *testing.T) {
	states := []*multithreaded.State{
		RandomState(0),
		RandomState(1),
		RandomState(2),
	}
	var emptyHash [32]byte
	someThread := RandomThread(123)

	cases := []struct {
		name string
		mut  ExpectationMutator
	}{
		{name: "PreimageKey", mut: func(e *ExpectedState, st *multithreaded.State) { e.PreimageKey = emptyHash }},
		{name: "PreimageOffset", mut: func(e *ExpectedState, st *multithreaded.State) { e.PreimageOffset += 1 }},
		{name: "Heap", mut: func(e *ExpectedState, st *multithreaded.State) { e.Heap += 1 }},
		{name: "LLReservationStatus", mut: func(e *ExpectedState, st *multithreaded.State) { e.LLReservationStatus = e.LLReservationStatus + 1 }},
		{name: "LLAddress", mut: func(e *ExpectedState, st *multithreaded.State) { e.LLAddress += 1 }},
		{name: "LLOwnerThread", mut: func(e *ExpectedState, st *multithreaded.State) { e.LLOwnerThread += 1 }},
		{name: "ExitCode", mut: func(e *ExpectedState, st *multithreaded.State) { e.ExitCode += 1 }},
		{name: "Exited", mut: func(e *ExpectedState, st *multithreaded.State) { e.Exited = !e.Exited }},
		{name: "Step", mut: func(e *ExpectedState, st *multithreaded.State) { e.Step += 1 }},
		{name: "LastHint", mut: func(e *ExpectedState, st *multithreaded.State) { e.LastHint = []byte{7, 8, 9, 10} }},
		{name: "MemoryRoot", mut: func(e *ExpectedState, st *multithreaded.State) { e.MemoryRoot = emptyHash }},
		{name: "StepsSinceLastContextSwitch", mut: func(e *ExpectedState, st *multithreaded.State) { e.StepsSinceLastContextSwitch += 1 }},
		{name: "TraverseRight", mut: func(e *ExpectedState, st *multithreaded.State) { e.TraverseRight = !e.TraverseRight }},
		{name: "NextThreadId", mut: func(e *ExpectedState, st *multithreaded.State) { e.NextThreadId += 1 }},
		{name: "ThreadCount", mut: func(e *ExpectedState, st *multithreaded.State) { e.ThreadCount += 1 }},
		{name: "RightStackSize", mut: func(e *ExpectedState, st *multithreaded.State) { e.RightStackSize += 1 }},
		{name: "LeftStackSize", mut: func(e *ExpectedState, st *multithreaded.State) { e.LeftStackSize += 1 }},
		{name: "ActiveThreadId", mut: func(e *ExpectedState, st *multithreaded.State) { e.ActiveThreadId += 1 }},
		{name: "Empty thread expectations", mut: func(e *ExpectedState, st *multithreaded.State) {
			e.threadExpectations = map[arch.Word]*ExpectedThreadState{}
		}},
		{name: "Mismatched thread expectations", mut: func(e *ExpectedState, st *multithreaded.State) {
			e.threadExpectations = map[arch.Word]*ExpectedThreadState{someThread.ThreadId: newExpectedThreadState(someThread)}
		}},
		{name: "Active threadId", mut: func(e *ExpectedState, st *multithreaded.State) {
			e.threadExpectations[st.GetCurrentThread().ThreadId].ThreadId += 1
		}},
		{name: "Active thread exitCode", mut: func(e *ExpectedState, st *multithreaded.State) {
			e.threadExpectations[st.GetCurrentThread().ThreadId].ExitCode += 1
		}},
		{name: "Active thread exited", mut: func(e *ExpectedState, st *multithreaded.State) {
			e.threadExpectations[st.GetCurrentThread().ThreadId].Exited = !st.GetCurrentThread().Exited
		}},
		{name: "Active thread PC", mut: func(e *ExpectedState, st *multithreaded.State) {
			e.threadExpectations[st.GetCurrentThread().ThreadId].PC += 1
		}},
		{name: "Active thread NextPC", mut: func(e *ExpectedState, st *multithreaded.State) {
			e.threadExpectations[st.GetCurrentThread().ThreadId].NextPC += 1
		}},
		{name: "Active thread HI", mut: func(e *ExpectedState, st *multithreaded.State) {
			e.threadExpectations[st.GetCurrentThread().ThreadId].HI += 1
		}},
		{name: "Active thread LO", mut: func(e *ExpectedState, st *multithreaded.State) {
			e.threadExpectations[st.GetCurrentThread().ThreadId].LO += 1
		}},
		{name: "Active thread Registers", mut: func(e *ExpectedState, st *multithreaded.State) {
			e.threadExpectations[st.GetCurrentThread().ThreadId].Registers[0] += 1
		}},
		{name: "Active thread dropped", mut: func(e *ExpectedState, st *multithreaded.State) {
			e.threadExpectations[st.GetCurrentThread().ThreadId].Dropped = true
		}},
		{name: "Inactive threadId", mut: func(e *ExpectedState, st *multithreaded.State) {
			e.threadExpectations[FindNextThread(st).ThreadId].ThreadId += 1
		}},
		{name: "Inactive thread exitCode", mut: func(e *ExpectedState, st *multithreaded.State) {
			e.threadExpectations[FindNextThread(st).ThreadId].ExitCode += 1
		}},
		{name: "Inactive thread exited", mut: func(e *ExpectedState, st *multithreaded.State) {
			e.threadExpectations[FindNextThread(st).ThreadId].Exited = !FindNextThread(st).Exited
		}},
		{name: "Inactive thread PC", mut: func(e *ExpectedState, st *multithreaded.State) {
			e.threadExpectations[FindNextThread(st).ThreadId].PC += 1
		}},
		{name: "Inactive thread NextPC", mut: func(e *ExpectedState, st *multithreaded.State) {
			e.threadExpectations[FindNextThread(st).ThreadId].NextPC += 1
		}},
		{name: "Inactive thread HI", mut: func(e *ExpectedState, st *multithreaded.State) {
			e.threadExpectations[FindNextThread(st).ThreadId].HI += 1
		}},
		{name: "Inactive thread LO", mut: func(e *ExpectedState, st *multithreaded.State) {
			e.threadExpectations[FindNextThread(st).ThreadId].LO += 1
		}},
		{name: "Inactive thread Registers", mut: func(e *ExpectedState, st *multithreaded.State) {
			e.threadExpectations[FindNextThread(st).ThreadId].Registers[0] += 1
		}},
		{name: "Inactive thread dropped", mut: func(e *ExpectedState, st *multithreaded.State) {
			e.threadExpectations[FindNextThread(st).ThreadId].Dropped = true
		}},
	}
	for _, c := range cases {
		for i, state := range states {
			testName := fmt.Sprintf("%v (state #%v)", c.name, i)
			t.Run(testName, func(t *testing.T) {
				expected := NewExpectedState(t, state)
				c.mut(expected, state)

				// We should detect the change and fail
				mockT := &MockTestingT{}
				expected.Validate(mockT, state)
				mockT.RequireFailed(t)
			})
		}

	}
}

func TestValidate_shouldPassUnchangedExpectations(t *testing.T) {
	states := []*multithreaded.State{
		RandomState(0),
		RandomState(1),
		RandomState(2),
	}

	for i, state := range states {
		testName := fmt.Sprintf("State #%v", i)
		t.Run(testName, func(t *testing.T) {
			expected := NewExpectedState(t, state)

			mockT := &MockTestingT{}
			expected.Validate(mockT, state)
			mockT.RequireNoFailure(t)
		})
	}
}

func TestExpectMemoryWrite(t *testing.T) {
	memoryWriteFns := []struct {
		name string
		fn   func(t *testing.T, expectedState *ExpectedState, initialState mipsevm.FPVMState, addr arch.Word, val arch.Word)
	}{
		{
			name: "ExpectMemoryWrite",
			fn: func(t *testing.T, expectedState *ExpectedState, initialState mipsevm.FPVMState, addr arch.Word, val arch.Word) {
				expectedState.ExpectMemoryWrite(t, initialState, addr, val)
			},
		},
		{
			name: "ExpectMemoryWriteUint32",
			fn: func(t *testing.T, expectedState *ExpectedState, initialState mipsevm.FPVMState, addr arch.Word, val arch.Word) {
				expectedState.ExpectMemoryWriteUint32(t, initialState, addr, uint32(val))
			},
		},
	}

	cases := []struct {
		name                           string
		llAddress                      arch.Word
		llStatus                       multithreaded.LLReservationStatus
		llOwnerThread                  arch.Word
		targetAddress                  arch.Word
		shouldModifyReservation        bool
		shouldExpectReservationCleared bool
	}{
		{name: "Reservation has non-empty address, target does not match", llAddress: 0xF000, targetAddress: 0xF008},
		{name: "Reservation has non-empty status", llStatus: multithreaded.LLStatusActive64bit, targetAddress: 0xF008},
		{name: "Reservation has non-empty owner", llOwnerThread: 1, targetAddress: 0xF008},
		{name: "Reservation is empty", targetAddress: 0xF008, shouldModifyReservation: true, shouldExpectReservationCleared: true},
		{name: "Reservation is non-empty, target matches", llAddress: 0xF004, llStatus: multithreaded.LLStatusActive32bit, targetAddress: 0xF000, shouldExpectReservationCleared: true},
	}

	for _, fnCase := range memoryWriteFns {
		for i, c := range cases {

			caseName := fmt.Sprintf("%v: %v", fnCase.name, c.name)
			t.Run(caseName, func(t *testing.T) {
				state := RandomState(i)
				state.LLReservationStatus = c.llStatus
				state.LLAddress = c.llAddress
				state.LLOwnerThread = c.llOwnerThread

				expected := NewExpectedState(t, state)
				fnCase.fn(t, expected, state, c.targetAddress, 0x1234)

				if c.shouldModifyReservation {
					// We should have created a reservation that matches the target address
					targetEffAddr := c.targetAddress & arch.AddressMask
					llEffAddr := state.LLAddress & arch.AddressMask
					require.Equal(t, targetEffAddr, llEffAddr)
					require.NotEqual(t, multithreaded.LLStatusNone, state.LLReservationStatus)
				} else {
					require.Equal(t, c.llStatus, state.LLReservationStatus)
					require.Equal(t, c.llAddress, state.LLAddress)
					require.Equal(t, c.llOwnerThread, state.LLOwnerThread)
				}

				if c.shouldExpectReservationCleared {
					require.Equal(t, multithreaded.LLStatusNone, expected.LLReservationStatus)
					require.Equal(t, arch.Word(0), expected.LLAddress)
					require.Equal(t, arch.Word(0), expected.LLOwnerThread)
				} else {
					require.Equal(t, c.llStatus, expected.LLReservationStatus)
					require.Equal(t, c.llAddress, expected.LLAddress)
					require.Equal(t, c.llOwnerThread, expected.LLOwnerThread)
				}
			})
		}
	}
}

type MockTestingT struct {
	errCount int
}

var _ require.TestingT = (*MockTestingT)(nil)

func (m *MockTestingT) Errorf(format string, args ...interface{}) {
	m.errCount += 1
}

func (m *MockTestingT) FailNow() {
	m.errCount += 1
}

func (m *MockTestingT) RequireFailed(t require.TestingT) {
	require.Greater(t, m.errCount, 0, "Should have tracked a failure")
}

func (m *MockTestingT) RequireNoFailure(t require.TestingT) {
	require.Equal(t, m.errCount, 0, "Should not have tracked a failure")
}
