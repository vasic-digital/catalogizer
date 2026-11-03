package main

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Unit tests of the PURE assertion helpers (legal double-free unit logic, §11.4.289(C)(1)).
// The paired mutation of this file: make evaluateRound always return nil -> the *Fails* tests fail.

func TestPercentile_NearestRank(t *testing.T) {
	var s []time.Duration
	for i := 1; i <= 100; i++ {
		s = append(s, time.Duration(i)*time.Millisecond)
	}
	assert.Equal(t, 50*time.Millisecond, percentile(s, 50))
	assert.Equal(t, 95*time.Millisecond, percentile(s, 95))
	assert.Equal(t, 99*time.Millisecond, percentile(s, 99))
	assert.Equal(t, time.Duration(0), percentile(nil, 99))
	assert.Equal(t, 1*time.Millisecond, percentile(s, 0))
}

func TestEvaluateRound_Fails5xx(t *testing.T) {
	r := roundResult{Status: map[string]int{"200": 1000, "500": 1}, P99Ms: 5}
	v := evaluateRound(r, thresholds{MaxP99Ms: 100, MinOK: 10})
	assert.NotEmpty(t, v, "a single 5xx must fail the round")
}

func TestEvaluateRound_FailsP99OverThreshold(t *testing.T) {
	r := roundResult{Status: map[string]int{"200": 1000}, P99Ms: 150}
	assert.NotEmpty(t, evaluateRound(r, thresholds{MaxP99Ms: 100, MinOK: 10}))
}

func TestEvaluateRound_FailsWhenNothingAnswered(t *testing.T) {
	r := roundResult{Status: map[string]int{"429": 5000}, P99Ms: 1}
	assert.NotEmpty(t, evaluateRound(r, thresholds{MaxP99Ms: 100, MinOK: 10}), "all-429 is not a pass")
}

func TestEvaluateRound_FailsTransportErrors(t *testing.T) {
	r := roundResult{Status: map[string]int{"200": 100}, TransportErrors: 1, P99Ms: 1}
	assert.NotEmpty(t, evaluateRound(r, thresholds{MaxP99Ms: 100, MinOK: 10}))
}

func TestEvaluateRound_PassesHealthyAnd429IsNotAnError(t *testing.T) {
	r := roundResult{Status: map[string]int{"200": 1000, "429": 400}, P99Ms: 50}
	assert.Empty(t, evaluateRound(r, thresholds{MaxP99Ms: 100, MinOK: 10}))
}

func TestEvaluateRSSGrowth(t *testing.T) {
	assert.NotEmpty(t, evaluateRSSGrowth(100_000, 400_000, 100_000))
	assert.Empty(t, evaluateRSSGrowth(100_000, 150_000, 100_000))
}

func TestSameVerdicts(t *testing.T) {
	assert.True(t, sameVerdicts([]bool{true, true, true}))
	assert.False(t, sameVerdicts([]bool{true, false, true}))
	assert.False(t, sameVerdicts(nil))
}

func TestDeriveP99Threshold(t *testing.T) {
	assert.Equal(t, 500.0, deriveP99Threshold(20, 25, 100))
	assert.Equal(t, 100.0, deriveP99Threshold(1, 25, 100), "floor applies")
}

func TestProcReaders_OnOwnPid(t *testing.T) {
	pid := os.Getpid()
	assert.Greater(t, procRSSKB(pid), int64(0))
	assert.Greater(t, procFDs(pid), 2)
	assert.Greater(t, procThreads(pid), 0)
	assert.Equal(t, int64(-1), procRSSKB(0x7ffffff0), "unknown pid reports -1, not 0")
}
