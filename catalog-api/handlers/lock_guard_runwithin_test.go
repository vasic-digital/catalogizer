package handlers

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TH3-n4: after a guard timeout the abandoned goroutine must not write into variables the test goroutine still
// reads; runWithin hands the result over a buffered channel instead.
func TestRunWithin_ReturnsValueAndTimesOutWithoutSharedWrites(t *testing.T) {
	v, ok := runWithin(time.Second, func() int { return 7 })
	require.True(t, ok)
	require.Equal(t, 7, v)

	release := make(chan struct{})
	_, ok = runWithin(50*time.Millisecond, func() int { <-release; return 9 })
	require.False(t, ok, "a blocked fn must be reported as timed out")
	close(release) // the leaked goroutine finishes and sends into the buffered channel without blocking
}
