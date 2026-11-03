package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"catalogizer/config"
	"catalogizer/database"
)

// Pure helpers for the LIVE scaling/stress/chaos suite (scaling_live_test.go, build tag `live`).
// Nothing here talks to a server; these are the only parts of the suite for which unit tests with
// synthetic inputs are legitimate (§11.4.289(C)). The live tests feed them REAL measurements.

// percentile returns the p-th percentile (0..100, nearest-rank) of the samples; 0 when empty.
func percentile(samples []time.Duration, p float64) time.Duration {
	if len(samples) == 0 {
		return 0
	}
	s := append([]time.Duration(nil), samples...)
	sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
	rank := int(p/100*float64(len(s)) + 0.9999999)
	if rank < 1 {
		rank = 1
	}
	if rank > len(s) {
		rank = len(s)
	}
	return s[rank-1]
}

// roundResult is what one load round measured.
type roundResult struct {
	Clients         int            `json:"clients"`
	DurationMs      int64          `json:"duration_ms"`
	Requests        int            `json:"requests"`
	Status          map[string]int `json:"status_histogram"`
	TransportErrors int            `json:"transport_errors"`
	P50Ms           float64        `json:"p50_ms"`
	P95Ms           float64        `json:"p95_ms"`
	P99Ms           float64        `json:"p99_ms"`
	RSSBeforeKB     int64          `json:"rss_before_kb"`
	RSSAfterKB      int64          `json:"rss_after_kb"`
	FDsAfter        int            `json:"fds_after"`
	Threads         int            `json:"threads_after"`
	Goroutines      int            `json:"goroutines_after"`
}

// thresholds are the pass criteria for one round.
type thresholds struct {
	MaxP99Ms float64
	MinOK    int // minimum number of 200 responses (a run answering nothing must not pass)
	// MaxClientErrShare is the largest tolerated fraction (0..1) of ALL responses that are 401/403/404.
	// The load workload sends a valid token to endpoints that exist, so after the login none of these
	// is expected: the bound is 0 unless a caller justifies a larger one (SCALING_MAX_CLIENT_ERR_SHARE).
	// Without it an expired/rejected token or a wrong path still "passes" as long as MinOK 200s were
	// served and the latency of those 200s is good (a 401-heavy round was a PASS).
	MaxClientErrShare float64
}

// clientErrCodes are the statuses that mean the load did not exercise what it claims to
// (rejected credential, forbidden, missing route) rather than the server being overloaded.
var clientErrCodes = []string{"401", "403", "404"}

// evaluateRound returns the list of violations (empty == PASS). 5xx and transport errors are
// never tolerated; 429 is the server's documented rate limit and is not an error, but the
// latency samples that count are the 200 responses only (caller passes those in p99).
func evaluateRound(r roundResult, th thresholds) []string {
	var v []string
	total := 0
	codes := make([]string, 0, len(r.Status))
	for code := range r.Status {
		codes = append(codes, code)
	}
	sort.Strings(codes) // deterministic violation order
	for _, code := range codes {
		n := r.Status[code]
		total += n
		c, err := strconv.Atoi(code)
		if err != nil || c < 100 || c > 599 {
			v = append(v, fmt.Sprintf("%d responses with unparseable/out-of-range status key %q", n, code))
			continue
		}
		if c >= 500 && n > 0 {
			v = append(v, fmt.Sprintf("%d responses with status %d (5xx forbidden)", n, c))
		}
	}
	clientErr := 0
	for _, code := range clientErrCodes {
		clientErr += r.Status[code]
	}
	if total > 0 && float64(clientErr)/float64(total) > th.MaxClientErrShare {
		v = append(v, fmt.Sprintf("%d of %d responses were 401/403/404 (%.1f%% > allowed %.1f%%): the load did not exercise authenticated, existing endpoints",
			clientErr, total, 100*float64(clientErr)/float64(total), 100*th.MaxClientErrShare))
	}
	if r.TransportErrors > 0 {
		v = append(v, fmt.Sprintf("%d transport errors", r.TransportErrors))
	}
	if r.Status["200"] < th.MinOK {
		v = append(v, fmt.Sprintf("only %d x 200 responses, need >= %d", r.Status["200"], th.MinOK))
	}
	if r.P99Ms > th.MaxP99Ms {
		v = append(v, fmt.Sprintf("p99 %.1fms > threshold %.1fms", r.P99Ms, th.MaxP99Ms))
	}
	return v
}

// evaluateRSSGrowth checks the RSS after the last identical round against the first round.
func evaluateRSSGrowth(afterFirstKB, afterLastKB, maxGrowthKB int64) []string {
	if afterLastKB-afterFirstKB > maxGrowthKB {
		return []string{fmt.Sprintf("RSS grew %d kB across identical rounds (bound %d kB)", afterLastKB-afterFirstKB, maxGrowthKB)}
	}
	return nil
}

// sameVerdicts reports whether every round produced the identical pass/fail verdict.
func sameVerdicts(pass []bool) bool {
	for _, p := range pass {
		if p != pass[0] {
			return false
		}
	}
	return len(pass) > 0
}

// threshold derivation: measured baseline p99 x factor, never below floor.
func deriveP99Threshold(baselineMs, factor, floorMs float64) float64 {
	t := baselineMs * factor
	if t < floorMs {
		t = floorMs
	}
	return t
}

func readProcStatusKB(pid int, key string) int64 {
	b, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return -1
	}
	for _, ln := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(ln, key+":") {
			f := strings.Fields(strings.TrimPrefix(ln, key+":"))
			if len(f) > 0 {
				n, _ := strconv.ParseInt(f[0], 10, 64)
				return n
			}
		}
	}
	return -1
}

func procRSSKB(pid int) int64 { return readProcStatusKB(pid, "VmRSS") }
func procThreads(pid int) int { return int(readProcStatusKB(pid, "Threads")) }
func procFDs(pid int) int {
	ents, err := os.ReadDir(fmt.Sprintf("/proc/%d/fd", pid))
	if err != nil {
		return -1
	}
	return len(ents)
}

// sigkillIntegrityOK is the oracle for "the database left behind by SIGKILL is intact": it opens the
// SQLCipher file with the production key file and runs PRAGMA integrity_check. It returns ok only when
// the open succeeds and the check yields exactly one row, "ok". A failure to open, a query error (a
// corrupt encrypted page fails its HMAC check) or any other row is NOT ok. Its own negative control
// (a deliberately corrupted copy must be reported not-ok) lives in the unit test file.
func sigkillIntegrityOK(dbPath, keyFile string) (bool, string) {
	// The production opener creates (and migrates) a fresh database when the file is absent, which
	// would make a lost database read as "ok": require an existing, non-empty file first.
	if st, err := os.Stat(dbPath); err != nil || st.Size() == 0 {
		return false, "database file missing or empty: " + dbPath
	}
	db, err := database.NewConnection(&config.DatabaseConfig{Type: "sqlite", Path: dbPath, EncryptionKeyFile: keyFile, MaxOpenConnections: 2})
	if err != nil {
		return false, "open: " + err.Error()
	}
	defer db.Close()
	rows, err := db.Query(`PRAGMA integrity_check`)
	if err != nil {
		return false, "integrity_check: " + err.Error()
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			return false, "scan: " + err.Error()
		}
		got = append(got, line)
	}
	if err := rows.Err(); err != nil {
		return false, "rows: " + err.Error()
	}
	if len(got) == 1 && got[0] == "ok" {
		return true, "ok"
	}
	return false, strings.Join(got, "; ")
}

// dbPaths returns the database file and key file of a liveSrv-style work directory.
func dbPaths(dir string) (string, string) {
	return filepath.Join(dir, "catalog.db"), filepath.Join(dir, "keys", "db.key")
}
