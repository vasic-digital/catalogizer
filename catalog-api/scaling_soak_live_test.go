//go:build live

package main

// LIVE soak mode (S1 investigation): repeats an identical load round N times against ONE real
// catalog-api process and records per-round process + Go-runtime memory evidence so that
// "plateau vs unbounded growth" is decided from data. Same real-system rules as scaling_live_test.go.
//
// Env: SCALING_SOAK_ROUNDS (required, >0 to run), SCALING_SOAK_MODES (comma list of
// distinct32,fewips,manyips; default all), SCALING_SOAK_CLIENTS (default 32),
// SCALING_SOAK_ROUND_SECS (default 10), SCALING_SOAK_PROFILE_ROUNDS (comma list, default "3,<N>").

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type soakRound struct {
	Round        int            `json:"round"`
	Requests     int            `json:"requests"`
	Status       map[string]int `json:"status"`
	RSSKB        int64          `json:"rss_kb"`
	HeapInuseKB  int64          `json:"heap_inuse_kb"`
	HeapAllocKB  int64          `json:"heap_alloc_postgc_kb"`
	HeapObjects  int64          `json:"heap_objects_postgc"`
	HeapSysKB    int64          `json:"heap_sys_kb"`
	HeapIdleKB   int64          `json:"heap_idle_kb"`
	HeapRelKB    int64          `json:"heap_released_kb"`
	Goroutines   int            `json:"goroutines"`
	FDs          int            `json:"fds"`
	DistinctIPs  int            `json:"cumulative_distinct_source_ips"`
	IdleGoroutns int            `json:"goroutines_after_idle_5s"`
	RssAnonKB    int64          `json:"rss_anon_kb"`
	RssFileKB    int64          `json:"rss_file_kb"`
	VmDataKB     int64          `json:"vm_data_kb"`
	Threads      int            `json:"threads"`
	PeakGorMid   int            `json:"peak_goroutines_mid_round"`
}

// heapStats reads the runtime MemStats trailer of /debug/pprof/heap?debug=1&gc=1 (gc=1 forces a GC first).
func (s *liveSrv) heapStats() map[string]int64 {
	c := &http.Client{Timeout: 30 * time.Second}
	r, err := c.Get(s.base() + "/debug/pprof/heap?debug=1&gc=1")
	if err != nil {
		return nil
	}
	defer r.Body.Close()
	raw, _ := io.ReadAll(r.Body)
	m := map[string]int64{}
	for _, ln := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(ln, "# ") && strings.Contains(ln, " = ") {
			kv := strings.SplitN(strings.TrimPrefix(ln, "# "), " = ", 2)
			if n, err := strconv.ParseInt(strings.TrimSpace(kv[1]), 10, 64); err == nil {
				m[strings.TrimSpace(kv[0])] = n
			}
		}
	}
	return m
}

func (s *liveSrv) saveHeapProfile(path string) {
	c := &http.Client{Timeout: 60 * time.Second}
	r, err := c.Get(s.base() + "/debug/pprof/heap?gc=1")
	if err != nil {
		return
	}
	defer r.Body.Close()
	b, _ := io.ReadAll(r.Body)
	_ = os.WriteFile(path, b, 0o600)
}

// runRoundIP is runRound with an explicit client->source-IP-index mapping.
func runRoundIP(s *liveSrv, tok string, clients int, dur time.Duration, paths []string, ipOf func(i int) int) (int, map[string]int, int) {
	var peak int64
	samplerDone := make(chan struct{})
	go func() { // sample the server's goroutine count while the load is running
		defer close(samplerDone)
		for end := time.Now().Add(dur); time.Now().Before(end); time.Sleep(700 * time.Millisecond) {
			if g := int64(s.goroutines()); g > atomic.LoadInt64(&peak) {
				atomic.StoreInt64(&peak, g)
			}
		}
	}()
	status := map[string]int{}
	var mu sync.Mutex
	var reqs int64
	stop := time.Now().Add(dur)
	var wg sync.WaitGroup
	for i := 0; i < clients; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			c := clientFor(ipOf(i), 30*time.Second)
			defer c.CloseIdleConnections()
			k := i
			for time.Now().Before(stop) {
				code, _, err := s.authedGet(c, paths[k%len(paths)], tok)
				k++
				if err != nil {
					code = -1
				}
				atomic.AddInt64(&reqs, 1)
				mu.Lock()
				status[strconv.Itoa(code)]++
				mu.Unlock()
			}
		}(i)
	}
	wg.Wait()
	<-samplerDone
	return int(reqs), status, int(atomic.LoadInt64(&peak))
}

func TestLiveSoak(t *testing.T) {
	rounds, _ := strconv.Atoi(os.Getenv("SCALING_SOAK_ROUNDS"))
	if rounds <= 0 {
		t.Skip("SKIP: soak mode off (set SCALING_SOAK_ROUNDS=N)")
	}
	clients := int(envFloat("SCALING_SOAK_CLIENTS", 32))
	secs := int(envFloat("SCALING_SOAK_ROUND_SECS", 10))
	modes := strings.Split(os.Getenv("SCALING_SOAK_MODES"), ",")
	if os.Getenv("SCALING_SOAK_MODES") == "" {
		modes = []string{"distinct32", "fewips", "manyips"}
	}
	profRounds := map[int]bool{3: true, rounds: true}
	if v := os.Getenv("SCALING_SOAK_PROFILE_ROUNDS"); v != "" {
		profRounds = map[int]bool{}
		for _, f := range strings.Split(v, ",") {
			n, _ := strconv.Atoi(f)
			profRounds[n] = true
		}
	}
	for _, mode := range modes {
		mode := mode
		t.Run(mode, func(t *testing.T) {
			var extra []string
			if v := os.Getenv("SCALING_SOAK_SERVER_ENV"); v != "" { // e.g. MALLOC_ARENA_MAX=2 (diagnostic experiments)
				extra = strings.Split(v, ",")
			}
			s := newLiveSrv(t, "soak-"+mode, extra...)
			defer s.cleanup()
			s.mustStart()
			s.stopTerm(30 * time.Second)
			s.seedFiles(20000)
			s.mustStart()
			tok := s.mustLogin()
			paths := []string{
				"/api/v1/search/files?q=report&limit=50&page=1",
				"/api/v1/search/files?extension=mp4&limit=20&page=2",
				"/api/v1/search/files?file_type=video&limit=50&page=3",
				"/api/v1/media/stats", "/api/v1/storage/roots", "/api/v1/catalog",
			}
			pc := clientFor(251, 30*time.Second)
			for _, p := range paths {
				code, _, err := s.authedGet(pc, p, tok)
				require.NoError(t, err)
				require.Equal(t, 200, code, p)
			}
			seen := map[int]bool{}
			var out []soakRound
			for r := 1; r <= rounds; r++ {
				ipOf := func(i int) int { return i }
				switch mode {
				case "fewips":
					ipOf = func(i int) int { return i % 4 }
				case "manyips":
					base := r * clients
					ipOf = func(i int) int { return base + i }
				}
				for i := 0; i < clients; i++ {
					seen[ipOf(i)] = true
				}
				n, st, peakG := runRoundIP(s, tok, clients, time.Duration(secs)*time.Second, paths, ipOf)
				hs := s.heapStats()
				row := soakRound{Round: r, Requests: n, Status: st, RSSKB: procRSSKB(s.pid()), HeapInuseKB: hs["HeapInuse"] / 1024,
					HeapAllocKB: hs["HeapAlloc"] / 1024, HeapObjects: hs["HeapObjects"], HeapSysKB: hs["HeapSys"] / 1024,
					HeapIdleKB: hs["HeapIdle"] / 1024, HeapRelKB: hs["HeapReleased"] / 1024, Goroutines: s.goroutines(),
					FDs: procFDs(s.pid()), DistinctIPs: len(seen),
					RssAnonKB: readProcStatusKB(s.pid(), "RssAnon"), RssFileKB: readProcStatusKB(s.pid(), "RssFile"),
					VmDataKB: readProcStatusKB(s.pid(), "VmData"), Threads: procThreads(s.pid()), PeakGorMid: peakG}
				if r == rounds || r == 1 {
					time.Sleep(5 * time.Second)
					row.IdleGoroutns = s.goroutines()
				}
				out = append(out, row)
				if r == rounds {
					if sm, err := os.ReadFile(fmt.Sprintf("/proc/%d/smaps", s.pid())); err == nil {
						_ = os.WriteFile(filepath.Join(artifactDir(t), "smaps-"+mode+".txt"), sm, 0o600)
					}
				}
				if profRounds[r] {
					s.saveHeapProfile(filepath.Join(artifactDir(t), fmt.Sprintf("heap-%s-round%02d.pb.gz", mode, r)))
				}
				t.Logf("%s round %d: reqs=%d rss=%dKB heapInuse=%dKB heapAllocPostGC=%dKB objs=%d gor=%d fds=%d ips=%d", mode, r, n, row.RSSKB,
					row.HeapInuseKB, row.HeapAllocKB, row.HeapObjects, row.Goroutines, row.FDs, row.DistinctIPs)
			}
			b, _ := json.MarshalIndent(map[string]interface{}{"mode": mode, "clients": clients, "round_secs": secs, "rounds": out, "identity": s.identity()}, "", " ")
			require.NoError(t, os.WriteFile(filepath.Join(artifactDir(t), "soak-"+mode+".json"), b, 0o600))
		})
	}
}
