package handlers

// SECQ-1 live-tests review follow-ups (m1..m3, n1..n2). Same request path as storage_root_secq1_live_test.go (real router, real SQLite,
// real scanner and client factory, a real loopback server); each scenario pins one layer of the WebDAV storage-root client guard so that
// removing that layer fails here and not only in the filesystem unit tests:
//   - redirect check (netguard.OperatorCheckRedirectFor in newWebDAVHTTPClient),
//   - the stored approved_class pin (effectiveClass -> operatorStorageTransportFor).

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"catalogizer/internal/netguard"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// m1 / mutant B: the allow-listed origin answers a PROPFIND with a 307 to a scheme the guard never permits. Only the client's redirect
// check can refuse this with the netguard reason (a client without it fails with Go's own "unsupported protocol scheme").
func TestSECQ1_Live_RedirectToForbiddenSchemeRefusedByClientRedirectCheck(t *testing.T) {
	t.Setenv(netguard.AllowCIDRsEnv, "127.0.0.0/8")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "gopher://127.0.0.1:70/x")
		w.WriteHeader(http.StatusTemporaryRedirect)
	}))
	t.Cleanup(srv.Close)
	h := newAPIHarness(t, true)
	code, body := h.do("POST", "/storage/roots", map[string]interface{}{"name": "redir", "protocol": "webdav", "url": srv.URL, "username": "alice", "password": "pw", "path": "/"})
	require.Equal(t, http.StatusCreated, code, "%v", body)
	id := int64(body["id"].(float64))
	code, body = h.do("POST", "/scans", map[string]interface{}{"storage_root_id": id})
	require.Equal(t, http.StatusAccepted, code, "%v", body)
	st := h.waitScan(body["job_id"].(string))
	require.Equal(t, "failed", st["status"], "%v", st)
	require.Contains(t, st["reason"], `redirect to scheme "gopher" not permitted`, "the refusal must come from the client redirect check: %v", st)
}

// m1 / mutant C: a root whose stored class is public is pinned to public destinations. The same allow-listed loopback server that a
// private-approved root reaches (first scan) is refused once the operator narrows the root to public through the real route. A client that
// ignored the stored class (treating every root as private) would keep scanning it.
func TestSECQ1_Live_PublicApprovedRootRefusesAllowListedLoopback(t *testing.T) {
	t.Setenv(netguard.AllowCIDRsEnv, "127.0.0.0/8")
	srv := davServer(t, "alice", "pw", map[string][]string{"/": {"a.txt", "b.txt"}})
	h := newAPIHarness(t, true)
	h.r.PUT("/storage/roots/:id/approved-class", func(c *gin.Context) { c.Set("user_id", 7) }, h.handle.SetStorageRootApprovedClass)
	code, body := h.do("POST", "/storage/roots", map[string]interface{}{"name": "pubpin", "protocol": "webdav", "url": srv.URL, "username": "alice", "password": "pw", "path": "/"})
	require.Equal(t, http.StatusCreated, code, "%v", body)
	id := int64(body["id"].(float64))

	code, body = h.do("POST", "/scans", map[string]interface{}{"storage_root_id": id})
	require.Equal(t, http.StatusAccepted, code, "%v", body)
	st := h.waitScan(body["job_id"].(string))
	require.Equal(t, "completed", st["status"], "private-approved root reaches the allow-listed server: %v", st)

	code, body = h.do("PUT", "/storage/roots/"+strconv.FormatInt(id, 10)+"/approved-class", map[string]interface{}{"class": "public"})
	require.Equal(t, http.StatusOK, code, "%v", body)
	require.Equal(t, "public", body["approved_class"])

	code, body = h.do("POST", "/scans", map[string]interface{}{"storage_root_id": id})
	require.Equal(t, http.StatusAccepted, code, "%v", body)
	st = h.waitScan(body["job_id"].(string))
	require.Equal(t, "failed", st["status"], "a public-approved root must not reach a loopback destination: %v", st)
	require.Contains(t, st["reason"], "approved class", "the refusal names the class pin: %v", st)
}
