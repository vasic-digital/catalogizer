package handlers

// SSE-OPQ-1 (operator decision 2026-11-14): (a) GET /scans and GET /scans/:job_id redact the scan failure text for non-admins
// (fixed class text, admins see raw); (b) GET /storage/roots and /storage-roots are admin-only; /browse/roots stays auth-only
// and returns no host/path/username/domain/password/url/options. Real handlers, real JWT middleware and route registration, the
// production-migrated SQLite schema; only the scanner (an interface) is a stand-in that holds a failed ScanStatus.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"catalogizer/filesystem"
	"catalogizer/internal/services"
	"catalogizer/middleware"
	"catalogizer/repository"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	opqRawReason = "smb://nas.internal/share failed for user bob: NT_STATUS_LOGON_FAILURE"
	opqHost      = "nas-secret-host.lan"
	opqPath      = "secretshare/private"
	opqUser      = "opquser"
	opqDomain    = "OPQDOMAIN"
)

type opqHarness struct {
	t    *testing.T
	r    *gin.Engine
	h    *apiHarness
	scan *mockUniversalScanner
	sh   *ScanHandler
}

func newOPQHarness(t *testing.T, checker func(context.Context, int) (bool, error)) *opqHarness {
	t.Helper()
	h := newAPIHarness(t, false)
	t.Setenv(filesystem.AllowedLocalBasesEnv, "/tmp")
	ms := newMockUniversalScanner()
	ms.activeStatuses["job1"] = &services.ScanStatus{JobID: "job1", StorageRootName: "r", Protocol: "smb", StartTime: time.Now(), Status: "failed", Reason: opqRawReason}
	sh := NewScanHandler(ms, h.db)
	jwtmw := middleware.NewJWTMiddleware(accessSecret)
	if checker != nil {
		sh.WithAdminChecker(checker)
		jwtmw.SetAdminChecker(checker)
	}
	r := gin.New()
	api := r.Group("/api/v1")
	api.Use(jwtmw.RequireAuth())
	RegisterStorageRoutes(api, sh, jwtmw)
	RegisterScanRoutes(api.Group("/scans"), sh, jwtmw)
	bh := NewBrowseHandler(repository.NewFileRepository(h.db))
	api.GET("/browse/roots", bh.GetStorageRoots)
	api.GET("/browse", bh.GetStorageRoots)
	_, err := h.db.Exec(`INSERT INTO storage_roots (id, name, protocol, host, path, username, domain, enabled) VALUES (1, 'opqroot', 'smb', ?, ?, ?, ?, 1)`,
		opqHost, opqPath, opqUser, opqDomain)
	require.NoError(t, err)
	return &opqHarness{t: t, r: r, h: h, scan: ms, sh: sh}
}

func (o *opqHarness) get(tok, path string) (int, string) {
	o.t.Helper()
	req := httptest.NewRequest("GET", "/api/v1"+path, nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	o.r.ServeHTTP(w, req)
	return w.Code, w.Body.String()
}

func TestSSEOPQ1_ScanReason_NonAdminRedacted_AdminRaw(t *testing.T) {
	o := newOPQHarness(t, nil)
	user, adm := cat001Token(t, 2, "2"), cat001Token(t, 1, "1")
	for _, p := range []string{"/scans", "/scans/job1"} {
		code, body := o.get(adm, p)
		require.Equal(t, http.StatusOK, code, p)
		require.Contains(t, body, "nas.internal", "control needle: admin sees the raw text at %s", p)
		code, body = o.get(user, p)
		require.Equal(t, http.StatusOK, code, p)
		assert.NotContains(t, body, "nas.internal", "%s leaks the scan failure text to a non-admin", p)
		assert.NotContains(t, body, "bob")
		assert.Contains(t, body, scanErrorClassText, p)
		assert.Contains(t, body, `"status":"failed"`, "non-failure fields stay")
	}
}

func TestSSEOPQ1_StorageRoots_AdminOnly(t *testing.T) {
	o := newOPQHarness(t, nil)
	user, adm := cat001Token(t, 2, "2"), cat001Token(t, 1, "1")
	for _, p := range []string{"/storage/roots", "/storage-roots"} {
		code, body := o.get(adm, p)
		require.Equal(t, http.StatusOK, code, p)
		require.Contains(t, body, opqHost, "control needle: admin sees the root at %s", p)
		code, body = o.get(user, p)
		assert.Equal(t, http.StatusForbidden, code, p)
		for _, leak := range []string{opqHost, opqPath, opqUser, opqDomain} {
			assert.NotContains(t, body, leak, "%s leaks %q to a non-admin", p, leak)
		}
	}
}

func TestSSEOPQ1_BrowseRoots_AuthOnly_NoConnectionData(t *testing.T) {
	o := newOPQHarness(t, nil)
	user, adm := cat001Token(t, 2, "2"), cat001Token(t, 1, "1")
	for _, tok := range []string{user, adm} {
		for _, p := range []string{"/browse/roots", "/browse"} {
			code, body := o.get(tok, p)
			require.Equal(t, http.StatusOK, code, p)
			require.True(t, strings.Contains(body, "opqroot"), "control needle: the root is listed at %s: %s", p, body)
			for _, leak := range []string{opqHost, opqPath, opqUser, opqDomain, `"password"`, `"url"`, `"options"`} {
				assert.NotContains(t, body, leak, "%s leaks %q", p, leak)
			}
		}
	}
	code, _ := o.get("", "/browse/roots")
	assert.Equal(t, http.StatusUnauthorized, code)
}

// Fail-closed: the live checker decides (the token's role_id is not trusted), and a checker error means non-admin / refused.
func TestSSEOPQ1_CheckerDecides_FailsClosed(t *testing.T) {
	always := func(context.Context, int) (bool, error) { return true, nil }
	never := func(context.Context, int) (bool, error) { return false, nil }
	broken := func(context.Context, int) (bool, error) { return true, errors.New("db down") }

	// checker says non-admin although the token claims role 1: redacted + 403.
	o := newOPQHarness(t, never)
	tok := cat001Token(t, 1, "5")
	_, body := o.get(tok, "/scans/job1")
	assert.NotContains(t, body, "nas.internal")
	code, _ := o.get(tok, "/storage/roots")
	assert.Equal(t, http.StatusForbidden, code)

	// checker says admin although the token claims role 2: raw text + 200.
	o = newOPQHarness(t, always)
	tok = cat001Token(t, 2, "5")
	_, body = o.get(tok, "/scans/job1")
	assert.Contains(t, body, "nas.internal")
	code, _ = o.get(tok, "/storage/roots")
	assert.Equal(t, http.StatusOK, code)

	// checker errors (even with admin=true): scan text redacted, roots refused (403 or 503, never 200).
	o = newOPQHarness(t, broken)
	tok = cat001Token(t, 1, "5")
	_, body = o.get(tok, "/scans")
	assert.NotContains(t, body, "nas.internal")
	assert.Contains(t, body, scanErrorClassText)
	for _, p := range []string{"/storage/roots", "/storage-roots"} {
		code, body = o.get(tok, p)
		assert.Contains(t, []int{http.StatusForbidden, http.StatusServiceUnavailable}, code, p)
		assert.NotContains(t, body, opqHost)
	}
}

// A scan with no failure reason is untouched for a non-admin (the redaction is not unconditional).
func TestSSEOPQ1_NoReason_NotRewritten(t *testing.T) {
	o := newOPQHarness(t, nil)
	o.scan.activeStatuses["job1"].Reason = ""
	o.scan.activeStatuses["job1"].Status = "running"
	_, body := o.get(cat001Token(t, 2, "2"), "/scans/job1")
	assert.NotContains(t, body, scanErrorClassText)
	assert.Contains(t, body, `"reason":""`)
}

// F1 (review round 1): GET /storage-roots/:id/status is open to any authenticated user; its error names the host / base path of the root.
func TestSSEOPQ1_F1_RootStatusErrorRedactedForNonAdmin(t *testing.T) {
	o := newOPQHarness(t, nil)
	missing := filepath.Join(os.TempDir(), "opq-secret-dir-does-not-exist-7731")
	_, err := o.h.db.Exec(`INSERT INTO storage_roots (id, name, protocol, path, enabled) VALUES (2, 'opqlocal', 'local', ?, 1)`, missing)
	require.NoError(t, err)
	user, adm := cat001Token(t, 2, "2"), cat001Token(t, 1, "1")

	code, body := o.get(adm, "/storage-roots/2/status")
	require.Equal(t, http.StatusOK, code, body)
	require.Contains(t, body, "opq-secret-dir-does-not-exist-7731", "control needle: the admin sees the raw probe error naming the path: %s", body)

	code, body = o.get(user, "/storage-roots/2/status")
	require.Equal(t, http.StatusOK, code, body)
	assert.NotContains(t, body, "opq-secret-dir-does-not-exist-7731", "non-admin sees the base path")
	assert.NotContains(t, body, os.TempDir())
	var out map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(body), &out))
	assert.Equal(t, scanErrorClassText, out["error"])
	assert.Equal(t, false, out["connected"], "the non-error fields stay")
	assert.NotEmpty(t, out["status"])
	// a healthy / disabled root carries no error key either way (the redaction is not unconditional)
	code, body = o.get(user, "/storage-roots/1/status")
	assert.NotContains(t, body, opqHost)
	_ = code
}

// M7 (review): the redaction must not depend on the text containing a URL; a bare host / account name is redacted too.
func TestSSEOPQ1_M7_BareHostAndAccountRedacted(t *testing.T) {
	o := newOPQHarness(t, nil)
	o.scan.activeStatuses["job1"].Reason = "login failed for svc_backup on fileserver01"
	for _, p := range []string{"/scans", "/scans/job1"} {
		_, body := o.get(cat001Token(t, 2, "2"), p)
		assert.NotContains(t, body, "svc_backup", p)
		assert.NotContains(t, body, "fileserver01", p)
		assert.Contains(t, body, scanErrorClassText, p)
		_, body = o.get(cat001Token(t, 1, "1"), p)
		assert.Contains(t, body, "svc_backup", "control: admin raw at "+p)
	}
}

// M11 (review): callerIsAdmin fails closed on a missing / invalid user id even when the checker would say admin, and on a non-int role.
func TestSSEOPQ1_M11_CallerIsAdminFailClosed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	yes := func(context.Context, int) (bool, error) { return true, nil }
	mk := func(set func(*gin.Context)) *gin.Context {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest("GET", "/", nil)
		set(c)
		return c
	}
	h := NewScanHandler(nil, nil).WithAdminChecker(yes)
	assert.False(t, h.callerIsAdmin(mk(func(*gin.Context) {})), "no user_id")
	assert.False(t, h.callerIsAdmin(mk(func(c *gin.Context) { c.Set("user_id", 0) })), "user_id 0")
	assert.False(t, h.callerIsAdmin(mk(func(c *gin.Context) { c.Set("user_id", "5") })), "string user_id")
	assert.True(t, h.callerIsAdmin(mk(func(c *gin.Context) { c.Set("user_id", 5) })), "control: valid id + checker yes")
	nc := NewScanHandler(nil, nil)
	assert.False(t, nc.callerIsAdmin(mk(func(c *gin.Context) { c.Set("role_id", "1") })), "string role")
	assert.False(t, nc.callerIsAdmin(mk(func(c *gin.Context) { c.Set("role_id", int64(1)) })), "int64 role")
	assert.True(t, nc.callerIsAdmin(mk(func(c *gin.Context) { c.Set("role_id", 1) })), "control: role 1")
}

// opqProbeClient connects fine and fails TestConnection with an error that names the share (unit-class double: it only drives the handler's
// TestConnection-failure branch, which a real local/ftp/smb client cannot reach after a successful Connect on demand).
type opqProbeClient struct {
	filesystem.FileSystemClient
	testErr error
}

func (c *opqProbeClient) Connect(context.Context) error        { return nil }
func (c *opqProbeClient) TestConnection(context.Context) error { return c.testErr }
func (c *opqProbeClient) Disconnect(context.Context) error     { return nil }

type opqProbeFactory struct{ c *opqProbeClient }

func (f opqProbeFactory) CreateClient(*filesystem.StorageConfig) (filesystem.FileSystemClient, error) {
	return f.c, nil
}
func (f opqProbeFactory) SupportedProtocols() []string { return []string{"local"} }

const opqProbeNeedle = "opqprobe-secret-share-4419"

func opqAddProbeRoot(t *testing.T, o *opqHarness) {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "opq-probe-")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	_, err = o.h.db.Exec(`INSERT INTO storage_roots (id, name, protocol, path, enabled) VALUES (3, 'opqprobe', 'local', ?, 1)`, dir)
	require.NoError(t, err)
	o.sh.clientFactory = opqProbeFactory{&opqProbeClient{testErr: errors.New("share " + opqProbeNeedle + " refused the probe")}}
}

// F1 (SSEOPQ-F1): the TestConnection-failure branch of GET /storage-roots/:id/status.
func TestSSEOPQ1_StorageRootStatus_TestConnectionFailure(t *testing.T) {
	o := newOPQHarness(t, nil)
	opqAddProbeRoot(t, o)
	user, adm := cat001Token(t, 2, "2"), cat001Token(t, 1, "1")

	code, body := o.get(adm, "/storage-roots/3/status")
	require.Equal(t, http.StatusOK, code, body)
	require.Contains(t, body, opqProbeNeedle, "control needle: the admin sees the raw TestConnection error (proves the branch was reached): %s", body)
	require.Contains(t, body, `"status":"offline"`)

	code, body = o.get(user, "/storage-roots/3/status")
	require.Equal(t, http.StatusOK, code, body)
	assert.NotContains(t, body, opqProbeNeedle, "non-admin sees the TestConnection error")
	var out map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(body), &out))
	assert.Equal(t, scanErrorClassText, out["error"])
	assert.Equal(t, "offline", out["status"])
	assert.Equal(t, false, out["connected"])
}

// F2 (SSEOPQ-F2): the status endpoint obeys the live checker, not the token claim, in both directions (and fails closed on a checker error).
func TestSSEOPQ1_StorageRootStatus_CheckerDecides(t *testing.T) {
	never := func(context.Context, int) (bool, error) { return false, nil }
	always := func(context.Context, int) (bool, error) { return true, nil }
	broken := func(context.Context, int) (bool, error) { return true, errors.New("db down") }
	cases := []struct {
		name    string
		checker func(context.Context, int) (bool, error)
		role    int
		raw     bool
	}{
		{"checker-no-token-claims-admin", never, 1, false},
		{"checker-yes-token-claims-user", always, 2, true},
		{"checker-error-token-claims-admin", broken, 1, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o := newOPQHarness(t, tc.checker)
			opqAddProbeRoot(t, o)
			code, body := o.get(cat001Token(t, tc.role, "5"), "/storage-roots/3/status")
			require.Equal(t, http.StatusOK, code, body)
			if tc.raw {
				assert.Contains(t, body, opqProbeNeedle)
			} else {
				assert.NotContains(t, body, opqProbeNeedle)
				assert.Contains(t, body, scanErrorClassText)
			}
		})
	}
}

// F3 (SSEOPQ-F3): redaction is keyed on a non-empty reason, not on status == failed (a cancelled scan with a reason is redacted too).
func TestSSEOPQ1_NonFailedStatusReasonRedacted(t *testing.T) {
	o := newOPQHarness(t, nil)
	o.scan.activeStatuses["job1"].Status = "cancelled"
	user, adm := cat001Token(t, 2, "2"), cat001Token(t, 1, "1")
	for _, p := range []string{"/scans", "/scans/job1"} {
		_, body := o.get(adm, p)
		require.Contains(t, body, "nas.internal", "control: admin raw at %s", p)
		require.Contains(t, body, `"status":"cancelled"`, p)
		_, body = o.get(user, p)
		assert.NotContains(t, body, "nas.internal", p)
		assert.Contains(t, body, scanErrorClassText, p)
		assert.Contains(t, body, `"status":"cancelled"`, p)
	}
}

// F5 (SSEOPQ-F5, adopted from the review): every protocol's status error, all branches; non-admin class text only, admin raw (control).
func TestSSEOPQ1_StatusAllProtocols(t *testing.T) {
	o := newOPQHarness(t, nil)
	user, adm := cat001Token(t, 2, "2"), cat001Token(t, 1, "1")
	rows := []struct {
		id                int
		proto, host, path string
		url               string
		port              int
	}{
		{10, "smb", "rev2needle-smb.invalid", "rev2share", "", 445},
		{11, "smb", "127.0.0.1", "rev2share", "", 1},
		{12, "ftp", "127.0.0.1", "/rev2ftp", "", 1},
		{13, "ftp", "rev2needle-ftp.invalid", "/rev2ftp", "", 21},
		{14, "webdav", "", "/rev2dav", "http://127.0.0.1:1/rev2dav", 0},
		{15, "nfs", "rev2needle-nfs.invalid", "/rev2nfs", "", 0},
		{16, "local", "", "/tmp/rev2-missing-local-dir", "", 0},
	}
	for _, r := range rows {
		_, err := o.h.db.Exec(`INSERT INTO storage_roots (id, name, protocol, host, port, path, url, username, enabled) VALUES (?, ?, ?, ?, ?, ?, ?, 'rev2user', 1)`,
			r.id, fmt.Sprintf("rev2-%d", r.id), r.proto, r.host, r.port, r.path, r.url)
		require.NoError(t, err)
	}
	for _, r := range rows {
		p := fmt.Sprintf("/storage-roots/%d/status", r.id)
		_, ab := o.get(adm, p)
		_, ub := o.get(user, p)
		var a, u map[string]interface{}
		require.NoError(t, json.Unmarshal([]byte(ab), &a), ab)
		require.NoError(t, json.Unmarshal([]byte(ub), &u), ub)
		require.NotNil(t, a["error"], "control: the admin sees an error for %s: %s", r.proto, ab)
		assert.Equal(t, a["status"], u["status"], p)
		assert.Equal(t, scanErrorClassText, u["error"], p)
		for _, leak := range []string{"rev2needle", "127.0.0.1", "rev2share", "rev2ftp", "rev2dav", "rev2nfs", "rev2user", "rev2-missing"} {
			assert.NotContains(t, ub, leak, "%s leaks %q", p, leak)
		}
	}
}
