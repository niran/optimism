package loadtest2

import (
	"sync/atomic"

	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

// Bus holds the latest batch of initiating messages.
type Bus struct {
	snapshot atomic.Value // stores []types.Message
}

func NewBus() *Bus {
	return &Bus{}
}

// Publish replaces the current snapshot with msgs.
func (b *Bus) Publish(msgs []types.Message) {
	b.snapshot.Store(msgs)
}

// Latest returns the last published slice (may be nil).
func (b *Bus) Latest() []types.Message {
	return b.snapshot.Load().([]types.Message)
}
