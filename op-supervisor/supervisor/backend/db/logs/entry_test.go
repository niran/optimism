package logs

import (
	"testing"
)

func TestEntryTypeFlagString(t *testing.T) {
	tests := []struct {
		name     string
		flag     EntryTypeFlag
		expected string
	}{
		{
			name:     "single flag - search checkpoint",
			flag:     FlagSearchCheckpoint,
			expected: "searchCheckpoint",
		},
		{
			name:     "single flag - canonical hash",
			flag:     FlagCanonicalHash,
			expected: "canonicalHash",
		},
		{
			name:     "single flag - initiating event",
			flag:     FlagInitiatingEvent,
			expected: "initiatingEvent",
		},
		{
			name:     "single flag - exec chain ID",
			flag:     FlagExecChainID,
			expected: "execChainID",
		},
		{
			name:     "single flag - exec position",
			flag:     FlagExecPosition,
			expected: "execPosition",
		},
		{
			name:     "single flag - exec checksum",
			flag:     FlagExecChecksum,
			expected: "execChecksum",
		},
		{
			name:     "single flag - padding",
			flag:     FlagPadding,
			expected: "padding",
		},
		{
			name:     "multiple flags",
			flag:     FlagSearchCheckpoint | FlagCanonicalHash | FlagInitiatingEvent,
			expected: "searchCheckpoint|canonicalHash|initiatingEvent",
		},
		{
			name:     "padding flags",
			flag:     FlagPadding | FlagPadding2 | FlagPadding3,
			expected: "padding|padding|padding",
		},
		{
			name:     "empty flag",
			flag:     0,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.flag.String()
			if got != tt.expected {
				t.Errorf("EntryTypeFlag.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}
