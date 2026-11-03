package main

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
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
}

// evaluateRound returns the list of violations (empty == PASS). 5xx and transport errors are
// never tolerated; 429 is the server's documented rate limit and is not an error, but the
// latency samples that count are the 200 responses only (caller passes those in p99).
func evaluateRound(r roundResult, th thresholds) []string {
	var v []string
	for code, n := range r.Status {
		c, err := strconv.Atoi(code)
		if err == nil && c >= 500 && n > 0 {
			v = append(v, fmt.Sprintf("%d responses with status %d (5xx forbidden)", n, c))
		}
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
