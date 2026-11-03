package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"catalogizer/config"
	"catalogizer/database"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

// ---- F1: 401/403/404 share after login ----

func TestEvaluateRound_Fails401HeavyRound(t *testing.T) {
	// 300 x 200 satisfies MinOK and p99, but 70% of the answers are 401: the load never ran as an
	// authenticated user for most of the round. This used to PASS.
	r := roundResult{Status: map[string]int{"200": 300, "401": 700}, P99Ms: 5}
	v := evaluateRound(r, thresholds{MaxP99Ms: 100, MinOK: 200})
	assert.NotEmpty(t, v)
	assert.Contains(t, v[0], "401/403/404")
}

func TestEvaluateRound_Fails403And404Share(t *testing.T) {
	for _, code := range []string{"403", "404"} {
		r := roundResult{Status: map[string]int{"200": 500, code: 5}, P99Ms: 5}
		assert.NotEmpty(t, evaluateRound(r, thresholds{MaxP99Ms: 100, MinOK: 200}), "any %s with the default zero bound fails", code)
	}
}

func TestEvaluateRound_ClientErrShareBoundRespected(t *testing.T) {
	r := roundResult{Status: map[string]int{"200": 990, "401": 10}, P99Ms: 5} // 1.0%
	assert.NotEmpty(t, evaluateRound(r, thresholds{MaxP99Ms: 100, MinOK: 200, MaxClientErrShare: 0.005}))
	assert.Empty(t, evaluateRound(r, thresholds{MaxP99Ms: 100, MinOK: 200, MaxClientErrShare: 0.01}), "exactly at the bound passes")
	assert.Empty(t, evaluateRound(r, thresholds{MaxP99Ms: 100, MinOK: 200, MaxClientErrShare: 0.02}))
}

// ---- F4: 503 / 599 and malformed status keys ----

func TestEvaluateRound_Fails503And599(t *testing.T) {
	for _, code := range []string{"500", "502", "503", "504", "599"} {
		r := roundResult{Status: map[string]int{"200": 1000, code: 1}, P99Ms: 5}
		v := evaluateRound(r, thresholds{MaxP99Ms: 100, MinOK: 10})
		if assert.NotEmpty(t, v, "status %s must fail the round", code) {
			assert.Contains(t, v[0], "5xx forbidden")
		}
	}
}

func TestEvaluateRound_Accepts499And200AndRateLimit(t *testing.T) {
	// 499/429 are not 5xx and not in the client-error set; they must not trip the 5xx rule.
	r := roundResult{Status: map[string]int{"200": 1000, "429": 50}, P99Ms: 5}
	assert.Empty(t, evaluateRound(r, thresholds{MaxP99Ms: 100, MinOK: 10}))
}

func TestEvaluateRound_FailsUnparseableOrOutOfRangeStatusKeys(t *testing.T) {
	for _, code := range []string{"abc", "", "0", "99", "600", "-1"} {
		r := roundResult{Status: map[string]int{"200": 1000, code: 3}, P99Ms: 5}
		assert.NotEmpty(t, evaluateRound(r, thresholds{MaxP99Ms: 100, MinOK: 10}), "status key %q must not be silently ignored", code)
	}
}

// ---- F6: the SIGKILL integrity oracle has a negative control ----

func makeSQLCipherDB(t *testing.T) (dir string) {
	t.Helper()
	dir = t.TempDir()
	dbp, keyp := dbPaths(dir)
	db, err := database.NewConnection(&config.DatabaseConfig{Type: "sqlite", Path: dbp, EncryptionKeyFile: keyp, MaxOpenConnections: 2})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE ctl (id INTEGER PRIMARY KEY, v TEXT)`); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3000; i++ {
		if _, err := db.Exec(`INSERT INTO ctl (v) VALUES (?)`, strings.Repeat("x", 200)); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestSigkillIntegrityOracle_GoldenGoodIsOK(t *testing.T) {
	dir := makeSQLCipherDB(t)
	ok, detail := sigkillIntegrityOK(dbPaths(dir))
	assert.True(t, ok, detail)
}

func TestSigkillIntegrityOracle_GoldenBadCorruptCopyIsNotOK(t *testing.T) {
	good := makeSQLCipherDB(t)
	bad := t.TempDir()
	gdb, gkey := dbPaths(good)
	bdb, bkey := dbPaths(bad)
	require.NoError(t, os.MkdirAll(filepath.Dir(bkey), 0o755))
	raw, err := os.ReadFile(gdb)
	require.NoError(t, err)
	key, err := os.ReadFile(gkey)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(bkey, key, 0o600))
	require.Greater(t, len(raw), 8*4096, "control database must span several pages")
	// Corrupt whole pages in the middle (not the header): overwrite 3 pages with a fixed pattern.
	for off := 4096 * 4; off < 4096*7; off++ {
		raw[off] ^= 0xA5
	}
	require.NoError(t, os.WriteFile(bdb, raw, 0o600))

	ok, detail := sigkillIntegrityOK(bdb, bkey)
	assert.False(t, ok, "a corrupted copy must be reported not-ok (oracle would be a PASS-bluff otherwise); detail=%q", detail)
	assert.NotEqual(t, "ok", detail)

	// and the untouched original is still reported ok (the oracle discriminates, it is not always-false)
	ok, detail = sigkillIntegrityOK(gdb, gkey)
	assert.True(t, ok, detail)
}

func TestSigkillIntegrityOracle_MissingDBIsNotOK(t *testing.T) {
	d := t.TempDir()
	ok, _ := sigkillIntegrityOK(filepath.Join(d, "none.db"), filepath.Join(d, "keys", "db.key"))
	assert.False(t, ok)
}
