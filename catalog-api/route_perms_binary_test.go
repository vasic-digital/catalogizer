package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// P2 (authz review r3): the storage-root / scan / SMB / copy routes sat behind
// RequireAuth alone while /auth/register is open, so any self-registered user
// could create storage roots, queue scans, probe SMB hosts and ingest. These
// real-binary tests (real router assembly, real SQLCipher DB, real register +
// login) pin the intended gate per route: anonymous 401, plain user 403,
// holder of the permission (or admin) reaches the handler, and the decision is
// live (a role that loses the permission is refused with the SAME token).

type permRoute struct {
	method, path string
	// adminOnly: needs system.admin (storage.manage alone must NOT pass).
	adminOnly bool
}

// storageRoutes are gated by storage.manage (system.admin and "*" also pass).
var storageRoutes = []permRoute{
	{"GET", "/api/v1/storage/roots", false},
	{"POST", "/api/v1/storage/roots", false},
	{"GET", "/api/v1/storage-roots", false},
	{"GET", "/api/v1/storage-roots/1/status", false},
	{"GET", "/api/v1/storage/list/x", false},
	{"POST", "/api/v1/scans", false},
	{"POST", "/api/v1/smb/discover", false},
	{"GET", "/api/v1/smb/discover", false},
	{"POST", "/api/v1/smb/test", false},
	{"GET", "/api/v1/smb/test", false},
	{"POST", "/api/v1/smb/browse", false},
	{"GET", "/api/v1/smb/identities", false},
	{"POST", "/api/v1/smb/probe", false},
	{"POST", "/api/v1/smb/probe-and-ingest", false},
	{"POST", "/api/v1/copy/storage", false},
	{"POST", "/api/v1/copy/local", false},
	{"POST", "/api/v1/copy/upload", false},
	// challenge execution runs server-side code paths: admin only.
	{"POST", "/api/v1/challenges/nope/run", true},
	{"POST", "/api/v1/challenges/run/category/zz-none", true},
}

func startPermServer(t *testing.T, readRoot string) (*apiClient, string, func()) {
	t.Helper()
	c, adm, _, stop := startPermServerDir(t, t.TempDir(), readRoot)
	return c, adm, stop
}

// startPermServerDir runs the server with cwd == dir (so ./data/catalogizer.db and
// keys/db.key live below dir) and returns dir for tests that aim at those files.
func startPermServerDir(t *testing.T, dir, readRoot string, extraEnv ...string) (*apiClient, string, string, func()) {
	t.Helper()
	bin := filepath.Join(dir, "catalog-api")
	if out, err := exec.Command("go", "build", "-buildvcs=false", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	b := &binEnv{t: t, bin: bin, dir: dir, port: port}
	const adminPw = "Op3rator-Supplied-Value-9x"
	cmd, done, out := b.start(append([]string{"GIN_MODE=release", "ADMIN_PASSWORD=" + adminPw, "CATALOGIZER_READ_ROOTS=" + readRoot}, extraEnv...)...)
	require.True(t, b.waitHealthy(done), "server must come up:\n%s", out.String())
	c := &apiClient{t: t, base: fmt.Sprintf("http://127.0.0.1:%d", port)}
	return c, c.login("admin", adminPw), dir, func() { b.stop(cmd, done) }
}

func selfRegister(t *testing.T, c *apiClient, name string) string {
	t.Helper()
	code, raw := c.do("POST", "/api/v1/auth/register", "", map[string]interface{}{
		"username": name, "email": name + "@example.invalid", "password": "Contract-Value-Strong-77",
		"first_name": "Self", "last_name": "Registered"})
	require.Equal(t, 201, code, "%s", raw)
	return c.login(name, "Contract-Value-Strong-77")
}

func TestRoutePerms_RealBinary_StorageScanSMBCopy(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs the server binary")
	}
	media := t.TempDir()
	c, adm, stop := startPermServer(t, media)
	defer stop()
	status := func(r permRoute, tok string) int {
		code, _ := c.do(r.method, r.path, tok, map[string]interface{}{})
		return code
	}

	plain := selfRegister(t, c, "p2plain") // the real threat: open self-registration, role 2
	storRole := c.id("POST", "/api/v1/roles", adm, map[string]interface{}{"name": "p2-storage", "permissions": []string{"storage.manage"}}, 201)
	c.id("POST", "/api/v1/users", adm, map[string]interface{}{"username": "p2stor", "email": "p2stor@example.invalid", "password": "Contract-Value-Strong-77", "role_id": storRole}, 201)
	stor := c.login("p2stor", "Contract-Value-Strong-77")

	for _, r := range storageRoutes {
		name := r.method + " " + r.path
		code, _ := c.do(r.method, r.path, "", map[string]interface{}{})
		assert.Equal(t, 401, code, "anonymous "+name)
		assert.Equal(t, 403, status(r, plain), "self-registered user must be refused on "+name)
		assert.NotContains(t, []int{401, 403, 503}, status(r, adm), "admin must reach "+name)
		if r.adminOnly {
			assert.Equal(t, 403, status(r, stor), "storage.manage alone must not run challenges: "+name)
		} else {
			assert.NotContains(t, []int{401, 403, 503}, status(r, stor), "storage.manage holder must reach "+name)
		}
	}

	// Live decision: the role loses storage.manage, the SAME token is refused at once.
	code, raw := c.do("PUT", fmt.Sprintf("/api/v1/roles/%d", storRole), adm, map[string]interface{}{"name": "p2-storage", "permissions": []string{"media.view"}})
	require.Equal(t, 200, code, "%s", raw)
	for _, r := range storageRoutes {
		assert.Equal(t, 403, status(r, stor), "stale claim after the role lost storage.manage: "+r.method+" "+r.path)
	}
}

// RunAll executes every registered challenge: never reached by a non-admin
// (kept out of the table above so a RED run cannot trigger it with plain/stor tokens).
func TestRoutePerms_RealBinary_RunAllChallengesAdminOnly(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs the server binary")
	}
	c, adm, stop := startPermServer(t, t.TempDir())
	defer stop()
	_ = adm
	code, _ := c.do("POST", "/api/v1/challenges/run", "", map[string]interface{}{})
	assert.Equal(t, 401, code, "anonymous")
	plain := selfRegister(t, c, "p2plainb")
	code, _ = c.do("POST", "/api/v1/challenges/run", plain, map[string]interface{}{})
	assert.Equal(t, 403, code, "self-registered user must not run all challenges")
}

// Read-only scan progress stays auth-only (the Android TV client polls it): documented, pinned.
func TestRoutePerms_RealBinary_ScanProgressStaysAuthOnly(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs the server binary")
	}
	c, _, stop := startPermServer(t, t.TempDir())
	defer stop()
	for _, p := range []string{"/api/v1/scans", "/api/v1/scans/none"} {
		code, _ := c.do("GET", p, "", nil)
		assert.Equal(t, 401, code, "anonymous "+p)
		code, _ = c.do("GET", p, selfRegister(t, c, "p2prog"+fmt.Sprint(len(p))), nil)
		assert.NotContains(t, []int{401, 403}, code, "authenticated user may read "+p)
	}
}

// Local storage roots: the path must lie inside the configured read roots
// (CATALOGIZER_READ_ROOTS / MEDIA_ROOT_PATH / ./data); anything else is refused.
func TestRoutePerms_RealBinary_LocalRootConfinement(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs the server binary")
	}
	media := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(media, "movies"), 0o755))
	c, adm, stop := startPermServer(t, media)
	defer stop()
	create := func(name, path string) (int, string) {
		code, raw := c.do("POST", "/api/v1/storage/roots", adm, map[string]interface{}{"name": name, "protocol": "local", "path": path})
		return code, string(raw)
	}
	for i, bad := range []string{"/etc", "/", filepath.Join(media, "..", "etc"), "relative/dir", filepath.Join(media, "movies") + "/../../.."} {
		code, raw := create(fmt.Sprintf("p2out%d", i), bad)
		assert.Equal(t, 400, code, "local root %q outside the configured base must be refused: %s", bad, raw)
	}
	// a symlink inside the base that points outside must be refused too
	require.NoError(t, os.Symlink("/etc", filepath.Join(media, "escape")))
	code, raw := create("p2sym", filepath.Join(media, "escape"))
	assert.Equal(t, 400, code, "symlink escape must be refused: %s", raw)
	// inside the base is accepted (admin flow intact)
	code, raw = create("p2in", filepath.Join(media, "movies"))
	assert.Equal(t, 201, code, "%s", raw)
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(raw), &m))

	// N2: the stored path is the RESOLVED one. A symlink inside the base that points at
	// another directory inside the base is accepted, but the database holds the target, so
	// swapping the link afterwards cannot redirect the scanner (F8).
	real := filepath.Join(media, "real")
	require.NoError(t, os.MkdirAll(real, 0o755))
	require.NoError(t, os.Symlink(real, filepath.Join(media, "link")))
	code, raw = create("p2link", filepath.Join(media, "link"))
	require.Equal(t, 201, code, "%s", raw)
	code, raw2 := c.do("GET", "/api/v1/storage/roots", adm, nil)
	require.Equal(t, 200, code, "%s", raw2)
	var listed map[string]interface{}
	require.NoError(t, json.Unmarshal(raw2, &listed))
	var stored string
	for _, r := range listed["roots"].([]interface{}) {
		if rm := r.(map[string]interface{}); rm["name"] == "p2link" {
			stored, _ = rm["path"].(string)
		}
	}
	resolvedReal, err := filepath.EvalSymlinks(real)
	require.NoError(t, err)
	assert.Equal(t, resolvedReal, stored, "the resolved path, not the symlink, must be stored: %s", raw2)
}

// F2: a local storage root must not be (or contain, or lie under) the application database,
// its SQLite companions or the key file, even when it is inside an allowed read root.
func TestRoutePerms_RealBinary_LocalRootRefusesProtectedFiles(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs the server binary")
	}
	dir := t.TempDir()
	c, adm, _, stop := startPermServerDir(t, dir, dir)
	defer stop()
	require.FileExists(t, filepath.Join(dir, "catalog.db"), "server must have created its database")
	cases := map[string]string{
		"dir containing the db":      dir,
		"db file":                    filepath.Join(dir, "catalog.db"),
		"wal companion":              filepath.Join(dir, "catalog.db-wal"),
		"shm companion":              filepath.Join(dir, "catalog.db-shm"),
		"key dir (contains the key)": filepath.Join(dir, "keys"),
		"key file":                   filepath.Join(dir, "keys", "db.key"),
	}
	i := 0
	for name, p := range cases {
		i++
		code, raw := c.do("POST", "/api/v1/storage/roots", adm, map[string]interface{}{"name": fmt.Sprintf("prot%d", i), "protocol": "local", "path": p})
		assert.Equal(t, 400, code, "%s (%s) must be refused: %s", name, p, raw)
	}
	other := filepath.Join(dir, "movies")
	require.NoError(t, os.MkdirAll(other, 0o755))
	code, raw := c.do("POST", "/api/v1/storage/roots", adm, map[string]interface{}{"name": "okroot", "protocol": "local", "path": other})
	assert.Equal(t, 201, code, "an ordinary directory must still be accepted: %s", raw)
}
