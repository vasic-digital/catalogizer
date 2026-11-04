package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Reviewer-authored: a strictly periodic pattern is ambiguous (period 5 s) and must be rejected
// by the margin rule; an early (negative) shift must be measured with the right sign.
func TestReviewer_PeriodicAmbiguousAndNegativeShift(t *testing.T) {
	n := 3000
	audio := make([]float64, n)
	for p := 0; p < n; p += 50 {
		for i := p; i < p+20 && i < n; i++ {
			audio[i] = 1
		}
	}
	cues := make([]float64, n+1200)
	copy(cues, audio)
	_, peak, runner, ok := estimateDelay(cues, audio)
	require.True(t, ok)
	assert.Less(t, peak-runner, syncMinMargin, "periodic pattern must be ambiguous")

	// irregular audio, subtitle EARLY by 3 s
	irr := make([]float64, n)
	for _, b := range [][2]int{{100, 130}, {170, 180}, {240, 300}, {350, 357}, {420, 470}, {600, 612}, {700, 790}} {
		for i := b[0]; i < b[1]; i++ {
			irr[i] = 1
		}
	}
	c2 := make([]float64, n+1200)
	for i, v := range irr {
		if i-30 >= 0 {
			c2[i-30] = v
		}
	}
	d, _, _, ok := estimateDelay(c2, irr)
	require.True(t, ok)
	assert.InDelta(t, -3.0, d, 0.15)
}
