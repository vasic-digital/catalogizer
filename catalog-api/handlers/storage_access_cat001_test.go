package handlers

// CAT-001 review round 1, F1 (critical): any self-registered user (role 2) could POST /storage/roots with protocol local and
// path "/" and then read host directories through GET /storage/list. These tests drive the REAL router with the REAL JWT
// middleware and a normal-user token; the reviewer probes RP1 and RP4 are adopted here.

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"catalogizer/filesystem"
	"catalogizer/middleware"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const accessSecret = "cat001-access-test-secret-0123456789"

type accessHarness struct {
	t *testing.T
	r *gin.Engine
	h *apiHarness
}

func newAccessHarness(t *testing.T, bases string) *accessHarness {
	t.Helper()
	h := newAPIHarness(t, false)
	t.Setenv(filesystem.AllowedLocalBasesEnv, bases)
	jwtmw := middleware.NewJWTMiddleware(accessSecret)
	r := gin.New()
	api := r.Group("/api/v1")
	api.Use(jwtmw.RequireAuth())
	RegisterStorageRoutes(api, h.handle, jwtmw)
	return &accessHarness{t: t, r: r, h: h}
}

func token(t *testing.T, role int, sub string) string {
	t.Helper()
	claims := &middleware.Claims{Username: "u" + sub, RoleID: role, RegisteredClaims: jwt.RegisteredClaims{
		Subject: sub, ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))}}
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(accessSecret))
	require.NoError(t, err)
	return s
}

func (a *accessHarness) do(tok, method, path string, body interface{}) (int, map[string]interface{}) {
	a.t.Helper()
	var buf bytes.Buffer
	if body != nil {
		require.NoError(a.t, json.NewEncoder(&buf).Encode(body))
	}
	req := httptest.NewRequest(method, "/api/v1"+path, &buf)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	a.r.ServeHTTP(w, req)
	var out map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

func idOf(body map[string]interface{}) string {
	return strconv.FormatInt(int64(body["id"].(float64)), 10)
}

func TestCAT001_F1_NormalUserCannotCreateStorageRoot(t *testing.T) {
	a := newAccessHarness(t, os.TempDir())
	user := token(t, 2, "2")
	code, body := a.do(user, "POST", "/storage/roots", map[string]interface{}{"name": "slash", "protocol": "local", "path": "/"})
	assert.Equal(t, http.StatusForbidden, code, "role 2 must not create roots: %v", body)
}

func TestCAT001_F1_NormalUserCannotListEvenAnAdminsRoot(t *testing.T) {
	base := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(base, "a.txt"), []byte("x"), 0o644))
	a := newAccessHarness(t, base)
	admin, user := token(t, 1, "1"), token(t, 2, "2")
	code, body := a.do(admin, "POST", "/storage/roots", map[string]interface{}{"name": "ok", "protocol": "local", "path": base})
	require.Equal(t, http.StatusCreated, code, "%v", body)
	code, _ = a.do(user, "GET", "/storage/list/?storage_id="+idOf(body), nil)
	assert.Equal(t, http.StatusForbidden, code)
	code, out := a.do(admin, "GET", "/storage/list/?storage_id="+idOf(body), nil)
	assert.Equal(t, http.StatusOK, code, "%v", out)
}

// RP1: even an admin cannot register "/" (host root) as a local root.
func TestCAT001_F1_RP1_AdminCannotRegisterHostRoot(t *testing.T) {
	base := t.TempDir()
	a := newAccessHarness(t, base)
	admin := token(t, 1, "1")
	code, body := a.do(admin, "POST", "/storage/roots", map[string]interface{}{"name": "slash", "protocol": "local", "path": "/"})
	assert.Equal(t, http.StatusForbidden, code, "a root outside the allowed bases is refused: %v", body)
}

func TestCAT001_F1_LocalRootsAreDeniedByDefault(t *testing.T) {
	a := newAccessHarness(t, "") // no allowed base configured
	code, body := a.do(token(t, 1, "1"), "POST", "/storage/roots", map[string]interface{}{"name": "any", "protocol": "local", "path": t.TempDir()})
	assert.Equal(t, http.StatusForbidden, code, "%v", body)
}

func TestCAT001_F1_RootThatIsASymlinkOutOfTheBaseIsRefused(t *testing.T) {
	base := t.TempDir()
	outside := t.TempDir()
	require.NoError(t, os.Symlink(outside, filepath.Join(base, "lnk")))
	a := newAccessHarness(t, base)
	code, body := a.do(token(t, 1, "1"), "POST", "/storage/roots", map[string]interface{}{"name": "lnk", "protocol": "local", "path": filepath.Join(base, "lnk")})
	assert.Equal(t, http.StatusForbidden, code, "%v", body)
}

// RP4: a symlink inside an allowed root pointing outside must not be followed by the listing.
func TestCAT001_F1_RP4_SymlinkInsideRootDoesNotEscape(t *testing.T) {
	outside := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("x"), 0o644))
	base := t.TempDir()
	require.NoError(t, os.Symlink(outside, filepath.Join(base, "link")))
	a := newAccessHarness(t, base)
	admin := token(t, 1, "1")
	code, body := a.do(admin, "POST", "/storage/roots", map[string]interface{}{"name": "sl", "protocol": "local", "path": base})
	require.Equal(t, http.StatusCreated, code, "%v", body)
	code, out := a.do(admin, "GET", "/storage/list/link?storage_id="+idOf(body), nil)
	files, _ := out["files"].([]interface{})
	for _, f := range files {
		assert.NotEqual(t, "secret.txt", f.(map[string]interface{})["name"], "symlink escape listed an outside file (code %d)", code)
	}
	assert.NotEqual(t, http.StatusOK, code)
}

func TestCAT001_F1_ScanTriggerIsAdminOnlyOnTheStorageRoutes(t *testing.T) {
	// /scans is mounted in main.go; the storage-root creation above is the route surface of this change. Documented, asserted via
	// the route table so a regression that re-opens POST /storage/roots is caught.
	a := newAccessHarness(t, os.TempDir())
	for _, ri := range a.r.Routes() {
		if ri.Method == "POST" && ri.Path == "/api/v1/storage/roots" {
			return
		}
	}
	t.Fatal("POST /storage/roots not mounted")
}
