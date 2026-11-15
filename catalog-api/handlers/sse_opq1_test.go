package handlers

// SSE-OPQ-1 (operator decision 2026-11-14): (a) GET /scans and GET /scans/:job_id redact the scan failure text for non-admins
// (fixed class text, admins see raw); (b) GET /storage/roots and /storage-roots are admin-only; /browse/roots stays auth-only
// and returns no host/path/username/domain/password/url/options. Real handlers, real JWT middleware and route registration, the
// production-migrated SQLite schema; only the scanner (an interface) is a stand-in that holds a failed ScanStatus.

import (
	"context"
	"encoding/json"
	"errors"
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
	return &opqHarness{t: t, r: r, h: h, scan: ms}
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
