//go:build realfs

package realfs

import (
	"bufio"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"catalogizer/database"
	"catalogizer/filesystem"
	"catalogizer/internal/services"
	"catalogizer/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// PA-03 against REAL servers (constitution 11.4.27: no fakes outside unit tests). Environment: the per-run env file of the WP-13 stack
// (TI_FTP_USER / TI_FTP_PASSWORD / TI_WEBDAV_USER / TI_WEBDAV_PASSWORD), the service names on the compose network (override
// TI_PROBE_HOST_FTP / TI_PROBE_HOST_WEBDAV) and the corpus manifest at /manifest.sha256 (override TI_MANIFEST). Missing environment FAILS.

func need(t *testing.T, k string) string {
	t.Helper()
	v := os.Getenv(k)
	if v == "" {
		t.Fatalf("%s is not set: this test runs only against the WP-13 stack (see docs/scripts/generic_scanner.md); it never skips", k)
	}
	return v
}

func hostOf(proto, def string) string {
	if v := os.Getenv("TI_PROBE_HOST_" + proto); v != "" {
		return v
	}
	return def
}

// manifest returns every file of the seeded corpus as "/rel/path" and the set of every ancestor directory.
func manifest(t *testing.T) (files []string, dirs map[string]bool) {
	t.Helper()
	p := os.Getenv("TI_MANIFEST")
	if p == "" {
		p = "/manifest.sha256"
	}
	f, err := os.Open(p)
	require.NoError(t, err, "the corpus manifest is the oracle of this test")
	defer f.Close()
	dirs = map[string]bool{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		line := sc.Text()
		if len(line) < 67 || line[64:66] != "  " {
			continue
		}
		rel := strings.TrimPrefix(line[66:], "./")
		files = append(files, "/"+rel)
		for d := path.Dir("/" + rel); d != "/" && d != "."; d = path.Dir(d) {
			dirs[d] = true
		}
	}
	require.NoError(t, sc.Err())
	require.NotEmpty(t, files, "empty manifest")
	sort.Strings(files)
	return files, dirs
}

type capture struct {
	mu    sync.Mutex
	files []string
	dirs  []string
}

func (c *capture) record(_ context.Context, p string, fi *filesystem.FileInfo, _ services.ScanJob, _ *services.ScanStatus) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if fi.IsDir {
		c.dirs = append(c.dirs, p)
	} else {
		c.files = append(c.files, p)
	}
	return nil
}

func rootFTP(t *testing.T, pathSetting string) *models.StorageRoot {
	h, port, u, pw := hostOf("FTP", "ftp"), 21, need(t, "TI_FTP_USER"), need(t, "TI_FTP_PASSWORD")
	r := &models.StorageRoot{Name: "realfs-ftp", Protocol: "ftp", Host: &h, Port: &port, Username: &u, Password: &pw}
	if pathSetting != "" {
		r.Path = &pathSetting
	}
	return r
}

func rootWebDAV(t *testing.T, pathSetting string) *models.StorageRoot {
	url, u, pw := "http://"+hostOf("WEBDAV", "webdav")+":80", need(t, "TI_WEBDAV_USER"), need(t, "TI_WEBDAV_PASSWORD")
	r := &models.StorageRoot{Name: "realfs-webdav", Protocol: "webdav", URL: &url, Username: &u, Password: &pw}
	if pathSetting != "" {
		r.Path = &pathSetting
	}
	return r
}

func connect(t *testing.T, root *models.StorageRoot) filesystem.FileSystemClient {
	t.Helper()
	settings := filesystem.SettingsFromRoot(root, nil)
	c, err := filesystem.NewDefaultClientFactory().CreateClient(&filesystem.StorageConfig{ID: root.Name, Name: root.Name, Protocol: root.Protocol, Settings: settings})
	require.NoError(t, err, "the settings the scanner side writes must be accepted by the factory (PA-01)")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	require.NoError(t, c.Connect(ctx))
	t.Cleanup(func() { _ = c.Disconnect(context.Background()) })
	return c
}

func newScanner(proto string, cap *capture) services.ProtocolScanner {
	switch proto {
	case "ftp":
		return services.NewFTPScanner(zap.NewNop()).WithRecorder(cap.record)
	default:
		return services.NewWebDAVScanner(zap.NewNop()).WithRecorder(cap.record)
	}
}

func job(root *models.StorageRoot, p string) services.ScanJob {
	return services.ScanJob{ID: "realfs", StorageRoot: root, Path: p, MaxDepth: 20, ScanType: "full", Context: context.Background()}
}

func underPrefix(files []string, prefix string) []string {
	var out []string
	for _, f := range files {
		if strings.HasPrefix(f, prefix+"/") {
			out = append(out, strings.TrimPrefix(f, prefix))
		}
	}
	return out
}

func runWholeCorpus(t *testing.T, root *models.StorageRoot) {
	files, dirs := manifest(t)
	c := connect(t, root)
	cap := &capture{}
	require.NoError(t, newScanner(root.Protocol, cap).ScanPath(context.Background(), c, job(root, "/"), &services.ScanStatus{JobID: "realfs", Protocol: root.Protocol}))

	sort.Strings(cap.files)
	assert.Equal(t, files, cap.files, "the scan must find EXACTLY the files of the seeded corpus manifest (%d)", len(files))
	got := map[string]bool{}
	for _, d := range cap.dirs {
		got[d] = true
	}
	for d := range dirs {
		assert.True(t, got[d], "directory %s of the corpus was not recorded", d)
	}
	t.Logf("realfs %s: %d files, %d directories recorded; manifest has %d files", root.Protocol, len(cap.files), len(cap.dirs), len(files))
}

func TestRealFTP_ScanFindsExactlyTheCorpus(t *testing.T)    { runWholeCorpus(t, rootFTP(t, "")) }
func TestRealWebDAV_ScanFindsExactlyTheCorpus(t *testing.T) { runWholeCorpus(t, rootWebDAV(t, "")) }

func runSubtree(t *testing.T, root *models.StorageRoot) {
	files, _ := manifest(t)
	want := underPrefix(files, "/movies")
	require.NotEmpty(t, want, "corpus has a movies directory")
	c := connect(t, root)
	cap := &capture{}
	require.NoError(t, newScanner(root.Protocol, cap).ScanPath(context.Background(), c, job(root, "/"), &services.ScanStatus{}))
	sort.Strings(cap.files)
	assert.Equal(t, want, cap.files, "root path /movies must reach the client: only that subtree is scanned (before PA-01 the path was never forwarded)")
}

func TestRealFTP_RootPathIsForwarded(t *testing.T)    { runSubtree(t, rootFTP(t, "/movies")) }
func TestRealWebDAV_RootPathIsForwarded(t *testing.T) { runSubtree(t, rootWebDAV(t, "/movies")) }

func runMissingRoot(t *testing.T, root *models.StorageRoot) {
	c := connect(t, root)
	cap := &capture{}
	err := newScanner(root.Protocol, cap).ScanPath(context.Background(), c, job(root, "/definitely-not-there"), &services.ScanStatus{})
	require.Error(t, err, "a scan of a root that does not exist must fail, never complete with 0 files")
	var se *services.ScanError
	require.True(t, errors.As(err, &se), "%v", err)
	assert.Equal(t, services.FailRootUnreadable, se.Kind, "a path that does not exist is unreadable, not empty (WF22 T3: the oracle no longer accepts empty_root too)")
	assert.Empty(t, cap.files)
	t.Logf("realfs %s missing root: kind=%s", root.Protocol, se.Kind)
}

func TestRealFTP_MissingRootFails(t *testing.T)    { runMissingRoot(t, rootFTP(t, "")) }
func TestRealWebDAV_MissingRootFails(t *testing.T) { runMissingRoot(t, rootWebDAV(t, "")) }

func TestRealFTP_WrongPasswordNeverReachesAScan(t *testing.T) {
	root := rootFTP(t, "")
	bad := "wrong-credential-for-the-negative-test"
	root.Password = &bad
	c, err := filesystem.NewDefaultClientFactory().CreateClient(&filesystem.StorageConfig{Protocol: "ftp", Settings: filesystem.SettingsFromRoot(root, nil)})
	require.NoError(t, err)
	cerr := c.Connect(context.Background())
	require.Error(t, cerr)
	assert.NotContains(t, cerr.Error(), bad, "a credential never appears in an error")
}

// queue -> worker -> processScanJob -> client factory -> connect -> scanner -> status. It runs with db=nil (counting only); the database half is
// covered by the TestRealDB_* tests below, the HTTP handler half by the handlers package tests (WF22 T1: this comment used to say "the whole path").
func runEndToEnd(t *testing.T, root *models.StorageRoot, missing bool) *services.ScanStatus {
	us := services.NewUniversalScanner(nil, zap.NewNop(), nil, filesystem.NewDefaultClientFactory())
	us.Start()
	defer us.Stop()
	p := "/"
	if missing {
		p = "/definitely-not-there"
	}
	j := job(root, p)
	j.ID = fmt.Sprintf("e2e-%s-%v", root.Protocol, missing)
	require.NoError(t, us.QueueScan(j))
	deadline := time.Now().Add(120 * time.Second)
	for time.Now().Before(deadline) {
		if st, ok := us.GetActiveScanStatus(j.ID); ok {
			if snap := st.GetSnapshot(); snap.Status != "running" && snap.Status != "" {
				return &snap
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatal("scan job did not finish within 120 s")
	return nil
}

func TestRealFTP_JobEndsCompletedWithTheCorpus(t *testing.T) {
	files, dirs := manifest(t)
	st := runEndToEnd(t, rootFTP(t, ""), false)
	assert.Equal(t, "completed", st.Status, st.Reason)
	assert.GreaterOrEqual(t, st.FilesFound, int64(len(files)+len(dirs)), "the job found at least every manifest file and ancestor directory (empty directories add more)")
}

func TestRealWebDAV_JobEndsCompletedWithTheCorpus(t *testing.T) {
	files, dirs := manifest(t)
	st := runEndToEnd(t, rootWebDAV(t, ""), false)
	assert.Equal(t, "completed", st.Status, st.Reason)
	assert.GreaterOrEqual(t, st.FilesFound, int64(len(files)+len(dirs)))
}

func TestRealFTP_JobOnMissingRootEndsFailed(t *testing.T) {
	st := runEndToEnd(t, rootFTP(t, ""), true)
	assert.Equal(t, "failed", st.Status, "never completed with 0 files")
	assert.NotEmpty(t, st.Reason)
	assert.Equal(t, int64(0), st.FilesFound)
}

func TestRealWebDAV_JobOnMissingRootEndsFailed(t *testing.T) {
	st := runEndToEnd(t, rootWebDAV(t, ""), true)
	assert.Equal(t, "failed", st.Status)
	assert.NotEmpty(t, st.Reason)
	assert.Equal(t, int64(0), st.FilesFound)
}

// An EMPTY directory of the real corpus is a root that lists fine and holds nothing: the scan must end failed (empty_root), not completed with 0 files.
func runEmptyDirRoot(t *testing.T, root *models.StorageRoot) {
	c := connect(t, root)
	all := &capture{}
	require.NoError(t, newScanner(root.Protocol, all).ScanPath(context.Background(), c, job(root, "/"), &services.ScanStatus{}))
	has := func(d string) bool {
		for _, p := range append(append([]string{}, all.files...), all.dirs...) {
			if strings.HasPrefix(p, d+"/") {
				return true
			}
		}
		return false
	}
	var empty string
	sort.Strings(all.dirs)
	for _, d := range all.dirs {
		if !has(d) {
			empty = d
			break
		}
	}
	require.NotEmpty(t, empty, "the seeded corpus contains empty directories (seed_corpus.sh); none was found")

	cap := &capture{}
	err := newScanner(root.Protocol, cap).ScanPath(context.Background(), c, job(root, empty), &services.ScanStatus{})
	require.Error(t, err, "scan of the empty directory %s must not end completed", empty)
	var se *services.ScanError
	require.True(t, errors.As(err, &se), "%v", err)
	assert.Equal(t, services.FailEmptyRoot, se.Kind)
	assert.Empty(t, cap.files)
	t.Logf("realfs %s empty root %s: kind=%s", root.Protocol, empty, se.Kind)
}

func TestRealFTP_EmptyDirectoryRootFails(t *testing.T)    { runEmptyDirRoot(t, rootFTP(t, "")) }
func TestRealWebDAV_EmptyDirectoryRootFails(t *testing.T) { runEmptyDirRoot(t, rootWebDAV(t, "")) }

// ---- WF22 fix round 2: the database half and the path/credential findings, against the REAL servers -----------------------------------------

func realDB(t *testing.T) *database.DB {
	t.Helper()
	sqlDB, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	db := database.WrapDB(sqlDB, database.DialectSQLite)
	require.NoError(t, db.RunMigrations(context.Background()))
	t.Cleanup(func() { db.Close() })
	return db
}

type dbRow struct {
	id         int64
	isDir      bool
	parentPath string
}

func dbRows(t *testing.T, db *database.DB) map[string]dbRow {
	t.Helper()
	rows, err := db.Query(`SELECT f.id, f.path, f.is_directory, COALESCE(p.path, '') FROM files f LEFT JOIN files p ON p.id = f.parent_id`)
	require.NoError(t, err)
	defer rows.Close()
	out := map[string]dbRow{}
	for rows.Next() {
		var r dbRow
		var p string
		require.NoError(t, rows.Scan(&r.id, &p, &r.isDir, &r.parentPath))
		out[p] = r
	}
	require.NoError(t, rows.Err())
	return out
}

func runJobWithDB(t *testing.T, db *database.DB, root *models.StorageRoot, id string) *services.ScanStatus {
	t.Helper()
	return runJobAtPath(t, db, root, id, "/")
}

func runJobAtPath(t *testing.T, db *database.DB, root *models.StorageRoot, id, p string) *services.ScanStatus {
	t.Helper()
	us := services.NewUniversalScanner(db, zap.NewNop(), nil, filesystem.NewDefaultClientFactory())
	us.Start()
	defer us.Stop()
	j := job(root, p)
	j.ID = id
	require.NoError(t, us.QueueScan(j))
	deadline := time.Now().Add(180 * time.Second)
	for time.Now().Before(deadline) {
		if st, ok := us.GetActiveScanStatus(j.ID); ok {
			if snap := st.GetSnapshot(); snap.Status != "running" && snap.Status != "" {
				return &snap
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatal("scan job did not finish within 180 s")
	return nil
}

// runDBBacked: a real job against a real server writes the corpus into a real database ONCE: one row per entry, slashless catalog paths, every
// parent the row of its directory; a rescan keeps every id.
func runDBBacked(t *testing.T, root *models.StorageRoot) {
	files, dirs := manifest(t)
	db := realDB(t)
	st := runJobWithDB(t, db, root, "db-"+root.Protocol)
	require.Equal(t, "completed", st.Status, st.Reason)
	assert.Equal(t, int64(0), st.ErrorCount, "a clean scan of a clean corpus reports zero errors (an MLSD server's . and .. entries are not errors, WF22 R16)")
	rows := dbRows(t, db)
	for _, f := range files {
		r, ok := rows[strings.TrimPrefix(f, "/")]
		require.True(t, ok, "file %s has no row", f)
		assert.False(t, r.isDir, f)
		want := strings.TrimPrefix(path.Dir(f), "/")
		if want == "" || want == "." {
			assert.Equal(t, "", r.parentPath, f)
		} else {
			assert.Equal(t, want, r.parentPath, "%s: its parent is the row of its directory (no phantom parent)", f)
		}
	}
	for d := range dirs {
		r, ok := rows[strings.TrimPrefix(d, "/")]
		require.True(t, ok, "directory %s has no row", d)
		assert.True(t, r.isDir, d)
	}
	for p := range rows {
		assert.False(t, strings.HasPrefix(p, "/"), "a catalog path has no leading slash: %q", p)
	}
	nDirs := 0
	for _, r := range rows {
		if r.isDir {
			nDirs++
		}
	}
	assert.Equal(t, len(rows), len(files)+nDirs, "rows = files + directories: nothing stored twice")
	assert.GreaterOrEqual(t, nDirs, len(dirs))

	first := map[string]int64{}
	for p, r := range rows {
		first[p] = r.id
	}
	st = runJobWithDB(t, db, root, "db2-"+root.Protocol)
	require.Equal(t, "completed", st.Status, st.Reason)
	again := dbRows(t, db)
	assert.Equal(t, len(rows), len(again), "a rescan adds no rows")
	for p, r := range again {
		assert.Equal(t, first[p], r.id, "%s keeps its id across a rescan", p)
	}
	t.Logf("realdb %s: %d rows (%d files, %d directories), rescan kept all ids", root.Protocol, len(rows), len(files), nDirs)
}

func TestRealDB_FTP_JobWritesTheCorpusOnceAndRescanKeepsIds(t *testing.T) {
	runDBBacked(t, rootFTP(t, ""))
}
func TestRealDB_WebDAV_JobWritesTheCorpusOnceAndRescanKeepsIds(t *testing.T) {
	runDBBacked(t, rootWebDAV(t, ""))
}

// R18: the FTP root path in every spelling reaches the same subtree on the real pure-ftpd.
func TestRealFTP_RelativeAndAbsoluteRootPathsScanTheSameSubtree(t *testing.T) {
	files, _ := manifest(t)
	want := underPrefix(files, "/movies")
	require.NotEmpty(t, want)
	for _, spelling := range []string{"/movies", "movies", "/movies/", "movies/"} {
		root := rootFTP(t, spelling)
		c := connect(t, root)
		cap := &capture{}
		require.NoError(t, newScanner("ftp", cap).ScanPath(context.Background(), c, job(root, "/"), &services.ScanStatus{}), spelling)
		sort.Strings(cap.files)
		assert.Equal(t, want, cap.files, "root path %q", spelling)
	}
}

// F5: the WebDAV URL's own path and the root path are JOINED, whichever of them carries the subtree.
func TestRealWebDAV_URLPathAndRootPathAreJoined(t *testing.T) {
	files, dirs := manifest(t)
	want := underPrefix(files, "/movies")
	require.NotEmpty(t, want)
	base := "http://" + hostOf("WEBDAV", "webdav") + ":80"
	check := func(rawURL, p string) {
		u, user, pw := rawURL, need(t, "TI_WEBDAV_USER"), need(t, "TI_WEBDAV_PASSWORD")
		root := &models.StorageRoot{Name: "realfs-webdav-join", Protocol: "webdav", URL: &u, Username: &user, Password: &pw}
		if p != "" {
			root.Path = &p
		}
		c := connect(t, root)
		cap := &capture{}
		require.NoError(t, newScanner("webdav", cap).ScanPath(context.Background(), c, job(root, "/"), &services.ScanStatus{}), "url=%s path=%s", rawURL, p)
		sort.Strings(cap.files)
		assert.Equal(t, want, cap.files, "url=%s path=%s", rawURL, p)
	}
	check(base+"/movies", "")
	check(base+"/movies/", "/")
	check(base, "/movies")
	check(base+"/", "movies")
	// both carry a part: the URL path is a top-level directory of the corpus, the root path a directory directly below it
	var sub string
	for d := range dirs {
		if strings.Count(d, "/") == 2 && (sub == "" || d < sub) {
			sub = d
		}
	}
	require.NotEmpty(t, sub, "the corpus has a directory two levels deep")
	top := "/" + strings.Split(strings.TrimPrefix(sub, "/"), "/")[0]
	wantSub := underPrefix(files, sub)
	require.NotEmpty(t, wantSub, "%s holds files", sub)
	u, user, pw := base+top, need(t, "TI_WEBDAV_USER"), need(t, "TI_WEBDAV_PASSWORD")
	p := strings.TrimPrefix(sub, top)
	root := &models.StorageRoot{Name: "realfs-webdav-join2", Protocol: "webdav", URL: &u, Username: &user, Password: &pw, Path: &p}
	c := connect(t, root)
	cap := &capture{}
	require.NoError(t, newScanner("webdav", cap).ScanPath(context.Background(), c, job(root, "/"), &services.ScanStatus{}))
	sort.Strings(cap.files)
	assert.Equal(t, wantSub, cap.files, "url path %s + root path %s must address %s (WF22 F5: the root path used to REPLACE the url path)", top, p, sub)
}

// R9 on the real server: the credential sits in the URL userinfo AND in the root fields, the listing fails (the path does not exist), and the
// client error names the URL of that listing: the job's reason - returned by the API - must not carry the real password.
func TestRealWebDAV_JobReasonNeverCarriesTheCredential(t *testing.T) {
	user, pw := need(t, "TI_WEBDAV_USER"), need(t, "TI_WEBDAV_PASSWORD")
	u := "http://" + user + ":" + pw + "@" + hostOf("WEBDAV", "webdav") + ":80"
	root := &models.StorageRoot{Name: "realfs-webdav-userinfo", Protocol: "webdav", URL: &u, Username: &user, Password: &pw}
	st := runJobAtPath(t, realDB(t), root, "userinfo-webdav", "/definitely-not-there")
	require.Equal(t, "failed", st.Status)
	assert.Contains(t, st.Reason, "404", "control: the failure is the listing's, whose error names the URL")
	assert.Contains(t, st.Reason, hostOf("WEBDAV", "webdav"), "control: the reason does name the host")
	assert.NotContains(t, st.Reason, pw, "the reason is returned by the API")
	t.Logf("realfs webdav listing failure: reason=%q", st.Reason)
}

// R3 on the real servers: an empty directory completes when (and only when) the root says allow_empty.
func TestRealEmptyDirectoryCompletesOnlyWithAllowEmpty(t *testing.T) {
	for _, proto := range []string{"ftp", "webdav"} {
		var root *models.StorageRoot
		if proto == "ftp" {
			root = rootFTP(t, "")
		} else {
			root = rootWebDAV(t, "")
		}
		c := connect(t, root)
		all := &capture{}
		require.NoError(t, newScanner(proto, all).ScanPath(context.Background(), c, job(root, "/"), &services.ScanStatus{}))
		has := func(d string) bool {
			for _, p := range append(append([]string{}, all.files...), all.dirs...) {
				if strings.HasPrefix(p, d+"/") {
					return true
				}
			}
			return false
		}
		var empty string
		sort.Strings(all.dirs)
		for _, d := range all.dirs {
			if !has(d) {
				empty = d
				break
			}
		}
		require.NotEmpty(t, empty, "%s: the corpus has empty directories", proto)

		cap := &capture{}
		err := newScanner(proto, cap).ScanPath(context.Background(), c, job(root, empty), &services.ScanStatus{})
		var se *services.ScanError
		require.True(t, errors.As(err, &se), "%s: default must fail: %v", proto, err)
		assert.Equal(t, services.FailEmptyRoot, se.Kind)

		root.AllowEmpty = true
		cap = &capture{}
		require.NoError(t, newScanner(proto, cap).ScanPath(context.Background(), c, job(root, empty), &services.ScanStatus{}), "%s: allow_empty completes", proto)
		assert.Empty(t, cap.files)
	}
}
