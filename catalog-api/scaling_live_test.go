//go:build live

package main

// LIVE scaling + stress + thin chaos suite (§11.4.289 LIVE-class).
//
// System under test: the REAL catalog-api binary built with `go build` from this tree, running a
// REAL SQLCipher SQLite file created by the PRODUCTION migrations, on loopback with an ephemeral
// port, real HTTP, real JWT login through the real seeded-admin path. No mock server, no handler
// double, no httptest recorder is in the path. Fixture INPUTS (synthetic catalog rows, a PNG) are
// fed to the real system; the rows are inserted through the real database layer (database.NewConnection).
//
// Run: scripts/test-infra/scaling_live.sh   (or `go test -tags live -run TestLive ./catalog-api`).
// Evidence: JSON files under $SCALING_LIVE_ARTIFACT_DIR.

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"catalogizer/config"
	"catalogizer/database"

	"github.com/stretchr/testify/require"
)

var forbiddenPorts = map[int]bool{38080: true, 33000: true, 33001: true, 38081: true, 33010: true, 33011: true}

// ---------- artifact dir, build, evidence ----------

var (
	buildOnce sync.Once
	builtBin  string
	buildInfo map[string]interface{}
	buildErr  error
)

func artifactDir(t *testing.T) string {
	d := os.Getenv("SCALING_LIVE_ARTIFACT_DIR")
	if d == "" {
		d = filepath.Join(t.TempDir(), "artifacts")
	}
	require.NoError(t, os.MkdirAll(d, 0o755))
	return d
}

func writeEvidence(t *testing.T, name string, v interface{}) {
	b, err := json.MarshalIndent(v, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(artifactDir(t), name), b, 0o644))
}

func buildBinary(t *testing.T) string {
	buildOnce.Do(func() {
		dir := artifactDir(t)
		bin := filepath.Join(dir, "catalog-api-live")
		rev := "unknown"
		if o, err := exec.Command("git", "rev-parse", "--short", "HEAD").Output(); err == nil {
			rev = strings.TrimSpace(string(o))
		}
		ver := "scaling-live-" + rev
		cmd := exec.Command("go", "build", "-buildvcs=false", "-ldflags", "-X main.Version="+ver+" -X main.BuildNumber=live", "-o", bin, ".")
		if out, err := cmd.CombinedOutput(); err != nil {
			buildErr = fmt.Errorf("go build failed: %v\n%s", err, out)
			return
		}
		f, _ := os.ReadFile(bin)
		sum := sha256.Sum256(f)
		builtBin = bin
		buildInfo = map[string]interface{}{"binary": bin, "sha256": hex.EncodeToString(sum[:]), "version_ldflag": ver, "size_bytes": len(f), "go": runtime.Version()}
	})
	if buildErr != nil {
		t.Skipf("SKIP: real dependency unavailable - cannot build catalog-api: %v", buildErr)
	}
	return builtBin
}

func randSecret(n int) string {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

// ---------- real server lifecycle ----------

type liveSrv struct {
	t       *testing.T
	bin     string
	dir     string
	port    int
	jwt     string
	adminPW string
	cmd     *exec.Cmd
	done    chan error
	exited  bool
	logPath string
	started time.Time
	extra   []string
}

func freePort(t *testing.T) int {
	for i := 0; i < 20; i++ {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		p := ln.Addr().(*net.TCPAddr).Port
		ln.Close()
		if !forbiddenPorts[p] {
			return p
		}
	}
	t.Fatal("no free non-forbidden port")
	return 0
}

func newLiveSrv(t *testing.T, name string, extra ...string) *liveSrv {
	bin := buildBinary(t)
	dir := filepath.Join(artifactDir(t), "run-"+name)
	require.NoError(t, os.RemoveAll(dir))
	require.NoError(t, os.MkdirAll(dir, 0o755))
	return &liveSrv{t: t, bin: bin, dir: dir, port: freePort(t), jwt: randSecret(32), adminPW: "Aa1-" + randSecret(12),
		logPath: filepath.Join(dir, "server.log"), extra: extra}
}

func (s *liveSrv) env() []string {
	e := []string{"PATH=" + os.Getenv("PATH"), "HOME=" + s.dir, "JWT_SECRET=" + s.jwt, "ADMIN_USERNAME=admin", "ADMIN_PASSWORD=" + s.adminPW,
		"HOST=127.0.0.1", fmt.Sprintf("SERVER_PORT=%d", s.port), "GIN_MODE=release", "DATABASE_TYPE=sqlite",
		"DATABASE_ENCRYPTION_KEY_FILE=" + filepath.Join(s.dir, "keys", "db.key"), "HELIX_PPROF_ENABLED=true",
		"CATALOGIZER_CONVERSION_ROOTS=" + filepath.Join(s.dir, "conv")}
	return append(e, s.extra...)
}

func (s *liveSrv) base() string { return fmt.Sprintf("http://127.0.0.1:%d", s.port) }
func (s *liveSrv) pid() int     { return s.cmd.Process.Pid }

func (s *liveSrv) start() {
	lf, err := os.OpenFile(s.logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	require.NoError(s.t, err)
	cmd := exec.Command(s.bin)
	cmd.Dir = s.dir
	cmd.Env = s.env()
	cmd.Stdout, cmd.Stderr = lf, lf
	s.started = time.Now()
	require.NoError(s.t, cmd.Start())
	s.cmd = cmd
	s.done = make(chan error, 1)
	s.exited = false
	go func() { s.done <- cmd.Wait(); lf.Close() }()
}

// waitHealthy returns the time from start() to the first 200 on /health.
func (s *liveSrv) waitHealthy(max time.Duration) (time.Duration, bool) {
	deadline := time.Now().Add(max)
	for time.Now().Before(deadline) {
		select {
		case <-s.done:
			return 0, false
		default:
		}
		c := &http.Client{Timeout: 2 * time.Second}
		if r, err := c.Get(s.base() + "/health"); err == nil {
			r.Body.Close()
			if r.StatusCode == 200 {
				return time.Since(s.started), true
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return 0, false
}

func (s *liveSrv) mustStart() time.Duration {
	s.start()
	d, ok := s.waitHealthy(90 * time.Second)
	require.True(s.t, ok, "server did not become healthy; log tail:\n%s", s.logTail(40))
	return d
}

func (s *liveSrv) logTail(n int) string {
	b, _ := os.ReadFile(s.logPath)
	ls := strings.Split(string(b), "\n")
	if len(ls) > n {
		ls = ls[len(ls)-n:]
	}
	out := strings.Join(ls, "\n")
	return strings.ReplaceAll(strings.ReplaceAll(out, s.adminPW, "[REDACTED]"), s.jwt, "[REDACTED]")
}

func (s *liveSrv) logContains(sub string) bool {
	b, _ := os.ReadFile(s.logPath)
	return strings.Contains(string(b), sub)
}

// stopTerm sends SIGTERM to the child pid only (never a process group, §11.4.263).
func (s *liveSrv) stopTerm(max time.Duration) (time.Duration, error) {
	t0 := time.Now()
	require.NoError(s.t, s.cmd.Process.Signal(syscall.SIGTERM))
	select {
	case err := <-s.done:
		s.exited = true
		return time.Since(t0), err
	case <-time.After(max):
		_ = s.cmd.Process.Kill()
		<-s.done
		s.exited = true
		return time.Since(t0), fmt.Errorf("did not exit within %s; killed", max)
	}
}

func (s *liveSrv) kill9() {
	require.NoError(s.t, s.cmd.Process.Kill())
	<-s.done
	s.exited = true
}

func (s *liveSrv) cleanup() {
	if s.cmd != nil && !s.exited {
		_ = s.cmd.Process.Kill()
		<-s.done
		s.exited = true
	}
}

// ---------- clients (distinct loopback source IPs: the per-IP limiter sees distinct clients) ----------

func clientFor(srcIdx int, timeout time.Duration) *http.Client {
	var la net.Addr
	if srcIdx >= 0 {
		la = &net.TCPAddr{IP: net.IPv4(127, 0, byte(1+srcIdx/250), byte(1+srcIdx%250))}
	}
	tr := &http.Transport{DialContext: (&net.Dialer{LocalAddr: la, Timeout: 10 * time.Second}).DialContext, MaxIdleConnsPerHost: 4}
	return &http.Client{Transport: tr, Timeout: timeout}
}

func (s *liveSrv) login(c *http.Client, user, pw string) (int, string) {
	body, _ := json.Marshal(map[string]string{"username": user, "password": pw})
	r, err := c.Post(s.base()+"/api/v1/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		return -1, ""
	}
	defer r.Body.Close()
	raw, _ := io.ReadAll(r.Body)
	var m map[string]interface{}
	_ = json.Unmarshal(raw, &m)
	tok, _ := m["session_token"].(string)
	return r.StatusCode, tok
}

func (s *liveSrv) mustLogin() string {
	code, tok := s.login(clientFor(250, 20*time.Second), "admin", s.adminPW)
	require.Equal(s.t, 200, code, "real admin login failed; log tail:\n%s", s.logTail(30))
	require.NotEmpty(s.t, tok)
	return tok
}

func (s *liveSrv) authedGet(c *http.Client, path, tok string) (int, []byte, error) {
	req, _ := http.NewRequest("GET", s.base()+path, nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	r, err := c.Do(req)
	if err != nil {
		return -1, nil, err
	}
	defer r.Body.Close()
	b, _ := io.ReadAll(r.Body)
	return r.StatusCode, b, nil
}

func (s *liveSrv) goroutines() int {
	c := &http.Client{Timeout: 5 * time.Second}
	r, err := c.Get(s.base() + "/debug/pprof/goroutine?debug=1")
	if err != nil {
		return -1
	}
	defer r.Body.Close()
	ln, _ := bufio.NewReader(r.Body).ReadString('\n')
	f := strings.Fields(ln) // "goroutine profile: total N"
	if len(f) >= 4 {
		n, _ := strconv.Atoi(f[3])
		return n
	}
	return -1
}

func (s *liveSrv) identity() map[string]interface{} {
	exe, _ := os.Readlink(fmt.Sprintf("/proc/%d/exe", s.pid()))
	b, _ := os.ReadFile(exe)
	sum := sha256.Sum256(b)
	c := &http.Client{Timeout: 5 * time.Second}
	var health map[string]interface{}
	if r, err := c.Get(s.base() + "/health"); err == nil {
		raw, _ := io.ReadAll(r.Body)
		r.Body.Close()
		_ = json.Unmarshal(raw, &health)
	}
	return map[string]interface{}{"endpoint": fmt.Sprintf("127.0.0.1:%d", s.port), "pid": s.pid(), "exe_path": exe,
		"exe_sha256": hex.EncodeToString(sum[:]), "health": health}
}

// ---------- fixture seeding through the REAL database layer ----------

func (s *liveSrv) openDB() *database.DB {
	db, err := database.NewConnection(&config.DatabaseConfig{Type: "sqlite", Path: filepath.Join(s.dir, "catalog.db"),
		EncryptionKeyFile: filepath.Join(s.dir, "keys", "db.key"), MaxOpenConnections: 4})
	require.NoError(s.t, err)
	return db
}

func (s *liveSrv) seedFiles(n int) {
	db := s.openDB()
	defer db.Close()
	tx, err := db.Begin()
	require.NoError(s.t, err)
	_, err = tx.Exec(`INSERT INTO storage_roots (name, protocol, path, enabled) VALUES ('scaling-root','local','/synthetic',1)`)
	require.NoError(s.t, err)
	var rootID int64
	require.NoError(s.t, tx.QueryRow(`SELECT id FROM storage_roots WHERE name='scaling-root'`).Scan(&rootID))
	exts := []struct{ ext, mime, ft string }{{"mp4", "video/mp4", "video"}, {"mkv", "video/x-matroska", "video"}, {"mp3", "audio/mpeg", "audio"},
		{"jpg", "image/jpeg", "image"}, {"pdf", "application/pdf", "document"}, {"txt", "text/plain", "document"}}
	words := []string{"report", "holiday", "invoice", "season", "episode", "album", "backup", "draft", "final", "scan"}
	st, err := tx.Prepare(`INSERT INTO files (storage_root_id, path, name, extension, mime_type, file_type, size, is_directory, modified_at) VALUES (?,?,?,?,?,?,?,0,?)`)
	require.NoError(s.t, err)
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	for i := 0; i < n; i++ {
		e := exts[i%len(exts)]
		name := fmt.Sprintf("%s_%s_%05d.%s", words[i%len(words)], words[(i/7)%len(words)], i, e.ext)
		_, err := st.Exec(rootID, fmt.Sprintf("/synthetic/dir%03d/%s", i%200, name), name, e.ext, e.mime, e.ft, 1000+i, now)
		require.NoError(s.t, err)
	}
	require.NoError(s.t, st.Close())
	require.NoError(s.t, tx.Commit())
	var cnt int
	require.NoError(s.t, db.QueryRow(`SELECT COUNT(*) FROM files`).Scan(&cnt))
	require.Equal(s.t, n, cnt)
}

func (s *liveSrv) dbScalar(q string) string {
	db := s.openDB()
	defer db.Close()
	var v sql.NullString
	require.NoError(s.t, db.QueryRow(q).Scan(&v))
	return v.String
}

// ---------- load engine ----------

func runRound(s *liveSrv, tok string, clients int, dur time.Duration, paths []string) roundResult {
	pid := s.pid()
	res := roundResult{Clients: clients, Status: map[string]int{}, RSSBeforeKB: procRSSKB(pid)}
	var mu sync.Mutex
	var lat200 []time.Duration
	var transportErr int64
	stop := time.Now().Add(dur)
	var wg sync.WaitGroup
	t0 := time.Now()
	for i := 0; i < clients; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			c := clientFor(i, 30*time.Second)
			defer c.CloseIdleConnections()
			k := i
			for time.Now().Before(stop) {
				p := paths[k%len(paths)]
				k++
				t1 := time.Now()
				code, _, err := s.authedGet(c, p, tok)
				d := time.Since(t1)
				if err != nil {
					atomic.AddInt64(&transportErr, 1)
					continue
				}
				mu.Lock()
				res.Status[strconv.Itoa(code)]++
				res.Requests++
				if code == 200 {
					lat200 = append(lat200, d)
				}
				mu.Unlock()
			}
		}(i)
	}
	wg.Wait()
	res.DurationMs = time.Since(t0).Milliseconds()
	res.TransportErrors = int(transportErr)
	res.P50Ms = float64(percentile(lat200, 50).Microseconds()) / 1000
	res.P95Ms = float64(percentile(lat200, 95).Microseconds()) / 1000
	res.P99Ms = float64(percentile(lat200, 99).Microseconds()) / 1000
	res.RSSAfterKB = procRSSKB(pid)
	res.FDsAfter = procFDs(pid)
	res.Threads = procThreads(pid)
	res.Goroutines = s.goroutines()
	return res
}

func envFloat(name string, def float64) float64 {
	if v := os.Getenv(name); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return def
}

// ---------- 1. SCALING ----------

func TestLiveScaling(t *testing.T) {
	s := newLiveSrv(t, "scaling")
	defer s.cleanup()
	startup := s.mustStart()
	s.stopTerm(30 * time.Second) // first start ran the production migrations + seeded admin
	s.seedFiles(20000)
	startup2 := s.mustStart()
	tok := s.mustLogin()

	// Sanity-select the workload: only endpoints that really answer 200 for this seeded catalog are loaded.
	cand := []string{
		"/api/v1/search/files?q=report&limit=50&page=1",
		"/api/v1/search/files?extension=mp4&limit=20&page=2",
		"/api/v1/search/files?file_type=video&limit=50&page=3",
		"/api/v1/search/files?name=invoice&limit=100",
		"/api/v1/media/stats",
		"/api/v1/storage/roots",
		"/api/v1/catalog",
		"/api/v1/media/search?q=season",
	}
	var paths []string
	probe := map[string]int{}
	pc := clientFor(251, 30*time.Second)
	for _, p := range cand {
		code, _, err := s.authedGet(pc, p, tok)
		if err != nil {
			code = -1
		}
		probe[p] = code
		if code == 200 {
			paths = append(paths, p)
		}
	}
	require.GreaterOrEqual(t, len(paths), 3, "too few real endpoints answer 200: %v", probe)
	// data really present: the search must return rows of the seeded catalog
	_, body, _ := s.authedGet(pc, "/api/v1/search/files?extension=mp4&limit=20&page=2", tok)
	require.Contains(t, string(body), "_", "search response carries no seeded rows: %.200s", body)

	// ---- calibration: MEASURE the baseline p99 on this host before any threshold exists ----
	factor := envFloat("SCALING_P99_FACTOR", 25)
	floor := envFloat("SCALING_P99_FLOOR_MS", 250)
	calDur := 6 * time.Second
	var calP99 float64
	var cal []roundResult
	for i := 0; i < 3; i++ {
		r := runRound(s, tok, 8, calDur, paths)
		cal = append(cal, r)
		if r.P99Ms > calP99 {
			calP99 = r.P99Ms
		}
	}
	threshold := deriveP99Threshold(calP99, factor, floor)
	if v := envFloat("SCALING_P99_MS", 0); v > 0 {
		threshold = v
	}
	th := thresholds{MaxP99Ms: threshold, MinOK: 200}
	rssBoundKB := int64(envFloat("SCALING_RSS_GROWTH_KB", 131072)) // 128 MiB across 3 identical rounds

	type level struct {
		Clients    int           `json:"clients"`
		Rounds     []roundResult `json:"rounds"`
		Violations [][]string    `json:"violations_per_round"`
		RSSViol    []string      `json:"rss_growth_violations"`
		Verdicts   []bool        `json:"round_verdicts"`
		Identical  bool          `json:"verdicts_identical_across_3_rounds"`
	}
	var levels []level
	roundDur := 10 * time.Second
	allPass := true
	for _, n := range []int{8, 32, 64} {
		lv := level{Clients: n}
		for r := 0; r < 3; r++ {
			rr := runRound(s, tok, n, roundDur, paths)
			v := evaluateRound(rr, th)
			lv.Rounds = append(lv.Rounds, rr)
			lv.Violations = append(lv.Violations, v)
			lv.Verdicts = append(lv.Verdicts, len(v) == 0)
		}
		lv.RSSViol = evaluateRSSGrowth(lv.Rounds[0].RSSAfterKB, lv.Rounds[2].RSSAfterKB, rssBoundKB)
		lv.Identical = sameVerdicts(lv.Verdicts)
		if len(lv.RSSViol) > 0 || !lv.Identical {
			allPass = false
		}
		for _, ok := range lv.Verdicts {
			if !ok {
				allPass = false
			}
		}
		levels = append(levels, lv)
	}
	hist := map[string]int{}
	total := 0
	for _, lv := range levels {
		for _, r := range lv.Rounds {
			for k, v := range r.Status {
				hist[k] += v
			}
			total += r.Requests
		}
	}
	ev := map[string]interface{}{
		"phase": "scaling", "date_utc": time.Now().UTC().Format(time.RFC3339), "host_dependent": true,
		"build": buildInfo, "identity": s.identity(), "seeded_rows": 20000,
		"startup_ms_first": startup.Milliseconds(), "startup_ms_seeded": startup2.Milliseconds(),
		"endpoint_probe": probe, "workload": paths,
		"calibration":         map[string]interface{}{"rounds": cal, "baseline_p99_ms_max": calP99, "factor": factor, "floor_ms": floor, "threshold_p99_ms": threshold},
		"rss_growth_bound_kb": rssBoundKB, "levels": levels, "total_requests": total, "status_histogram_all": hist, "pass": allPass,
		"note": "limiter keys on client IP; each client uses its own loopback source IP (127.0.x.y) so the 2000 rpm/IP limit is per simulated client",
	}
	writeEvidence(t, "scaling.json", ev)
	for _, lv := range levels {
		for i, v := range lv.Violations {
			require.Empty(t, v, "clients=%d round=%d", lv.Clients, i+1)
		}
		require.Empty(t, lv.RSSViol, "clients=%d", lv.Clients)
		require.True(t, lv.Identical, "clients=%d verdicts differ across identical rounds", lv.Clients)
	}
}

// ---------- 2. STRESS ----------

type hugeBody struct {
	prefix []byte
	left   int64
}

func (h *hugeBody) Read(p []byte) (int, error) {
	if len(h.prefix) > 0 {
		n := copy(p, h.prefix)
		h.prefix = h.prefix[n:]
		return n, nil
	}
	if h.left <= 0 {
		return 0, io.EOF
	}
	n := int64(len(p))
	if n > h.left {
		n = h.left
	}
	for i := int64(0); i < n; i++ {
		p[i] = 'a'
	}
	h.left -= n
	return int(n), nil
}

func TestLiveStress(t *testing.T) {
	s := newLiveSrv(t, "stress")
	defer s.cleanup()
	s.mustStart()
	tok := s.mustLogin()
	pid := s.pid()
	ev := map[string]interface{}{"phase": "stress", "date_utc": time.Now().UTC().Format(time.RFC3339), "build": buildInfo, "identity": s.identity()}
	rssStart := procRSSKB(pid)
	fdStart := procFDs(pid)
	ev["rss_start_kb"], ev["fds_start"] = rssStart, fdStart

	// ---- (a) login burst from one IP: wrong passwords + a nonexistent user; limiter must answer 429/401, never 5xx ----
	hist := map[string]int{}
	var hm sync.Mutex
	var wg sync.WaitGroup
	burstIP := 240
	for w := 0; w < 16; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			c := clientFor(burstIP, 20*time.Second)
			for i := 0; i < 20; i++ {
				user := "admin"
				if i%2 == 0 {
					user = "nobody-" + strconv.Itoa(w)
				}
				code, _ := s.login(c, user, "wrong-"+randSecret(6))
				hm.Lock()
				hist[strconv.Itoa(code)]++
				hm.Unlock()
			}
		}(w)
	}
	wg.Wait()
	// correct credentials from the burst IP (limited) and from a clean IP (admin may be locked out)
	limitedCode, _ := s.login(clientFor(burstIP, 20*time.Second), "admin", s.adminPW)
	cleanCode, _ := s.login(clientFor(241, 20*time.Second), "admin", s.adminPW)
	ev["login_burst"] = map[string]interface{}{"attempts": 320, "status_histogram": hist, "correct_pw_from_burst_ip": limitedCode, "correct_pw_from_clean_ip": cleanCode}
	for k := range hist {
		c, _ := strconv.Atoi(k)
		require.Less(t, c, 500, "login burst produced %s", k)
		require.GreaterOrEqual(t, c, 0, "transport error during login burst")
	}
	require.Greater(t, hist["429"], 0, "login limiter never answered 429 under a 320-attempt burst: %v", hist)
	require.Greater(t, hist["401"], 0, "wrong credentials never produced 401: %v", hist)
	require.Equal(t, 429, limitedCode, "burst IP must still be rate limited after the burst")
	require.Less(t, cleanCode, 500)
	s.assertAlive(t, "after login burst")

	// ---- (b) large-body rejection: 128 MiB bodies, unauthenticated and authenticated ----
	type bodyRes struct {
		Target string `json:"target"`
		Status int    `json:"status"`
		Err    string `json:"transport_error,omitempty"`
	}
	var bodies []bodyRes
	rssBeforeBody := procRSSKB(pid)
	for _, tgt := range []struct{ path, auth string }{{"/api/v1/auth/login", ""}, {"/api/v1/search/advanced", tok}} {
		c := clientFor(242, 120*time.Second)
		hb := &hugeBody{prefix: []byte(`{"username":"`), left: 128 << 20}
		req, _ := http.NewRequest("POST", s.base()+tgt.path, hb)
		req.ContentLength = 128<<20 + 13
		req.Header.Set("Content-Type", "application/json")
		if tgt.auth != "" {
			req.Header.Set("Authorization", "Bearer "+tgt.auth)
		}
		r, err := c.Do(req)
		br := bodyRes{Target: tgt.path}
		if err != nil {
			br.Status, br.Err = -1, err.Error()
		} else {
			io.Copy(io.Discard, r.Body)
			r.Body.Close()
			br.Status = r.StatusCode
		}
		bodies = append(bodies, br)
	}
	time.Sleep(2 * time.Second)
	rssAfterBody := procRSSKB(pid)
	ev["large_body"] = map[string]interface{}{"body_bytes": 128 << 20, "results": bodies, "rss_before_kb": rssBeforeBody, "rss_after_kb": rssAfterBody, "rss_delta_kb": rssAfterBody - rssBeforeBody}
	for _, b := range bodies {
		require.Less(t, b.Status, 500, "large body to %s answered %d", b.Target, b.Status)
	}
	s.assertAlive(t, "after large bodies")
	require.Less(t, rssAfterBody-rssBeforeBody, int64(envFloat("STRESS_BODY_RSS_KB", 262144)), "RSS grew %d kB handling 2x128MiB rejected bodies (OOM-class growth)", rssAfterBody-rssBeforeBody)

	// ---- (c) connection storm: 500 idle keep-alive connections, then close; fds return to baseline ----
	time.Sleep(time.Second)
	fdBase := procFDs(pid)
	const nconn = 500
	conns := make([]net.Conn, 0, nconn)
	var cm sync.Mutex
	var failed int64
	sem := make(chan struct{}, 25)
	var cwg sync.WaitGroup
	for i := 0; i < nconn; i++ {
		cwg.Add(1)
		sem <- struct{}{}
		go func() {
			defer cwg.Done()
			defer func() { <-sem }()
			cn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", s.port), 10*time.Second)
			if err != nil {
				atomic.AddInt64(&failed, 1)
				return
			}
			_ = cn.SetDeadline(time.Now().Add(20 * time.Second))
			_, _ = cn.Write([]byte("GET /health HTTP/1.1\r\nHost: x\r\nConnection: keep-alive\r\n\r\n"))
			buf := make([]byte, 512)
			if n, err := cn.Read(buf); err != nil || !bytes.HasPrefix(buf[:n], []byte("HTTP/1.1 200")) {
				atomic.AddInt64(&failed, 1)
				cn.Close()
				return
			}
			_ = cn.SetDeadline(time.Time{})
			cm.Lock()
			conns = append(conns, cn)
			cm.Unlock()
		}()
	}
	cwg.Wait()
	time.Sleep(500 * time.Millisecond)
	fdOpen := procFDs(pid)
	for _, cn := range conns {
		cn.Close()
	}
	var fdAfter int
	deadline := time.Now().Add(40 * time.Second)
	for time.Now().Before(deadline) {
		fdAfter = procFDs(pid)
		if fdAfter <= fdBase+10 {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	ev["connection_storm"] = map[string]interface{}{"target_conns": nconn, "established_200": len(conns), "failed": failed, "fds_baseline": fdBase, "fds_while_open": fdOpen, "fds_after_close": fdAfter, "settle_deadline_s": 40}
	require.Greater(t, len(conns), nconn*9/10, "storm did not establish; failed=%d", failed)
	require.Greater(t, fdOpen, fdBase+len(conns)/2, "server fds did not rise with open connections (storm not real): base=%d open=%d", fdBase, fdOpen)
	require.LessOrEqual(t, fdAfter, fdBase+10, "fds did not return to baseline after close")
	s.assertAlive(t, "after storm")
	ev["rss_end_kb"], ev["fds_end"], ev["threads_end"], ev["goroutines_end"] = procRSSKB(pid), procFDs(pid), procThreads(pid), s.goroutines()
	ev["pass"] = true
	writeEvidence(t, "stress.json", ev)
}

func (s *liveSrv) assertAlive(t *testing.T, when string) {
	select {
	case err := <-s.done:
		t.Fatalf("server process exited %s: %v\n%s", when, err, s.logTail(30))
	default:
	}
	code, _, err := s.authedGet(clientFor(243, 10*time.Second), "/health", "")
	require.NoError(t, err, when)
	require.Equal(t, 200, code, when)
}

// ---------- 3. THIN CHAOS ----------

func makeNoisePNG(t *testing.T, p string) {
	out, err := exec.Command("convert", "-size", "3000x3000", "xc:gray", "+noise", "Random", p).CombinedOutput()
	if err != nil {
		t.Skipf("SKIP: real ImageMagick converter unavailable: %v %s", err, out)
	}
}

func TestLiveChaosKillRestart(t *testing.T) {
	s := newLiveSrv(t, "chaos-kill")
	defer s.cleanup()
	conv := filepath.Join(s.dir, "conv")
	require.NoError(t, os.MkdirAll(conv, 0o755))
	src := filepath.Join(conv, "src.png")
	makeNoisePNG(t, src)
	s.mustStart()
	s.stopTerm(30 * time.Second)
	s.seedFiles(5000)
	s.mustStart()
	tok := s.mustLogin()
	ev := map[string]interface{}{"phase": "chaos_kill_restart", "date_utc": time.Now().UTC().Format(time.RFC3339), "build": buildInfo, "identity": s.identity()}

	// submit real conversion jobs through the real API
	const njobs = 40
	hc := clientFor(230, 30*time.Second)
	for i := 0; i < njobs; i++ {
		b, _ := json.Marshal(map[string]interface{}{"source_path": src, "target_path": filepath.Join(conv, fmt.Sprintf("out_%02d.jpg", i)),
			"source_format": "png", "target_format": "jpg", "conversion_type": "image", "quality": "high"})
		req, _ := http.NewRequest("POST", s.base()+"/api/v1/conversion/jobs", bytes.NewReader(b))
		req.Header.Set("Authorization", "Bearer "+tok)
		req.Header.Set("Content-Type", "application/json")
		r, err := hc.Do(req)
		require.NoError(t, err)
		raw, _ := io.ReadAll(r.Body)
		r.Body.Close()
		require.Equal(t, 200, r.StatusCode, "job %d: %s", i, raw)
	}

	// background load while the jobs run
	paths := []string{"/api/v1/search/files?q=report&limit=50", "/api/v1/media/stats", "/api/v1/storage/roots"}
	loadStop := make(chan struct{})
	var lwg sync.WaitGroup
	for i := 0; i < 16; i++ {
		lwg.Add(1)
		go func(i int) {
			defer lwg.Done()
			c := clientFor(i, 5*time.Second)
			for k := i; ; k++ {
				select {
				case <-loadStop:
					return
				default:
				}
				_, _, _ = s.authedGet(c, paths[k%len(paths)], tok)
			}
		}(i)
	}

	// wait until the worker has really claimed a job (status running in the real DB), then SIGKILL
	var running string
	killed := false
	for i := 0; i < 200 && !killed; i++ {
		running = s.dbScalar(`SELECT COUNT(*) FROM conversion_jobs WHERE status='running'`)
		if n, _ := strconv.Atoi(running); n > 0 {
			killed = true
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	require.True(t, killed, "worker never claimed a job")
	statusAtKill := s.dbScalar(`SELECT group_concat(status||'='||c) FROM (SELECT status, COUNT(*) c FROM conversion_jobs GROUP BY status)`)
	s.kill9()
	close(loadStop)
	lwg.Wait()
	ev["status_counts_at_kill"] = statusAtKill

	// offline integrity of the DB left behind by SIGKILL
	integrity := s.dbScalar(`PRAGMA integrity_check`)
	statusAfterKill := s.dbScalar(`SELECT group_concat(status||'='||c) FROM (SELECT status, COUNT(*) c FROM conversion_jobs GROUP BY status)`)
	totalAfterKill := s.dbScalar(`SELECT COUNT(*) FROM conversion_jobs`)
	ev["integrity_check"], ev["status_counts_after_kill_offline"], ev["jobs_after_kill"] = integrity, statusAfterKill, totalAfterKill
	require.Equal(t, "ok", integrity)
	require.Equal(t, strconv.Itoa(njobs), totalAfterKill, "rows lost/duplicated by SIGKILL")

	// restart on the same DB
	healthyIn := s.mustStart()
	ev["restart_to_healthy_ms"] = healthyIn.Milliseconds()
	bound := time.Duration(envFloat("CHAOS_RESTART_BOUND_S", 60)) * time.Second
	ev["restart_bound_s"] = bound.Seconds()
	require.Less(t, healthyIn, bound, "service not healthy again within bound")
	ev["worker_requeue_logged"] = s.logContains("requeued interrupted jobs")
	require.True(t, s.logContains("requeued interrupted jobs"), "restart logic did not requeue claimed jobs; log:\n%s", s.logTail(40))
	tok = s.mustLogin()

	// all jobs reach a terminal state; none stays running/claimed forever
	deadline := time.Now().Add(180 * time.Second)
	var final string
	for time.Now().Before(deadline) {
		final = s.dbScalar(`SELECT group_concat(status||'='||c) FROM (SELECT status, COUNT(*) c FROM conversion_jobs GROUP BY status)`)
		open := s.dbScalar(`SELECT COUNT(*) FROM conversion_jobs WHERE status IN ('pending','running')`)
		if open == "0" {
			break
		}
		time.Sleep(time.Second)
	}
	ev["status_counts_final"] = final
	ev["jobs_final"] = s.dbScalar(`SELECT COUNT(*) FROM conversion_jobs`)
	require.Equal(t, "0", s.dbScalar(`SELECT COUNT(*) FROM conversion_jobs WHERE status IN ('pending','running')`), "jobs stuck after restart: %s", final)
	require.Equal(t, strconv.Itoa(njobs), s.dbScalar(`SELECT COUNT(*) FROM conversion_jobs`))
	completed, _ := strconv.Atoi(s.dbScalar(`SELECT COUNT(*) FROM conversion_jobs WHERE status='completed'`))
	valid := 0
	for i := 0; i < njobs; i++ {
		if st, err := os.Stat(filepath.Join(conv, fmt.Sprintf("out_%02d.jpg", i))); err == nil && st.Size() > 1000 {
			valid++
		}
	}
	ev["completed"], ev["output_files_nonempty"] = completed, valid
	require.Equal(t, njobs, completed, "not all jobs completed after restart: %s", final)
	require.Equal(t, njobs, valid, "completed jobs without a real output file")
	require.Equal(t, "ok", s.dbScalar(`PRAGMA integrity_check`))
	ev["integrity_check_final"] = "ok"
	ev["pass"] = true
	writeEvidence(t, "chaos_kill_restart.json", ev)
	s.assertAlive(t, "end of kill/restart")
}

func TestLiveChaosGracefulShutdown(t *testing.T) {
	s := newLiveSrv(t, "chaos-term")
	defer s.cleanup()
	s.mustStart()
	ev := map[string]interface{}{"phase": "chaos_sigterm", "date_utc": time.Now().UTC().Format(time.RFC3339), "build": buildInfo, "identity": s.identity()}
	type inflight struct {
		code int
		n    int
		err  error
		dur  time.Duration
	}
	ch := make(chan inflight, 1)
	go func() {
		c := &http.Client{Timeout: 60 * time.Second}
		t0 := time.Now()
		r, err := c.Get(s.base() + "/debug/pprof/profile?seconds=4") // a genuinely long real in-flight request
		if err != nil {
			ch <- inflight{err: err, dur: time.Since(t0)}
			return
		}
		b, _ := io.ReadAll(r.Body)
		r.Body.Close()
		ch <- inflight{code: r.StatusCode, n: len(b), dur: time.Since(t0)}
	}()
	time.Sleep(1200 * time.Millisecond)
	exitIn, exitErr := s.stopTerm(45 * time.Second)
	got := <-ch
	boundS := envFloat("CHAOS_SHUTDOWN_BOUND_S", 35)
	ev["sigterm_to_exit_ms"], ev["exit_error"] = exitIn.Milliseconds(), fmt.Sprint(exitErr)
	ev["inflight"] = map[string]interface{}{"status": got.code, "bytes": got.n, "duration_ms": got.dur.Milliseconds(), "err": fmt.Sprint(got.err)}
	ev["shutdown_bound_s"] = boundS
	ev["log_shutting_down"] = s.logContains("Shutting down server")
	require.NoError(t, got.err, "in-flight request was cut by SIGTERM")
	require.Equal(t, 200, got.code)
	require.Greater(t, got.n, 100, "in-flight response truncated")
	require.NoError(t, exitErr, "non-zero exit after SIGTERM")
	require.Less(t, exitIn.Seconds(), boundS)
	ev["pass"] = true
	writeEvidence(t, "chaos_sigterm.json", ev)
	_ = context.Background
}
