package testutils

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type channelReader struct {
	valuesRead atomic.Int32
}

func (j *channelReader) RequireValuesRead(t *testing.T, n int) {
	require.Eventually(t, func() bool {
		return j.valuesRead.Load() == int32(n)
	}, time.Second, 100*time.Millisecond)
}

func start[T any](j *channelReader, newJobsChan chan T) {
	go func() {
		for range newJobsChan {
			j.valuesRead.Add(1)
		}
	}()
}

func LaunchNewChannelReader[T any]() (*channelReader, chan T) {
	c := make(chan T)
	cr := &channelReader{}
	start(cr, c)
	return cr, c
}
