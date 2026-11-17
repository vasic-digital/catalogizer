package handlers

// SECQ-1 (operator decision 2026-11-17): the WebDAV STORAGE-ROOT client is guarded like the sync client. Through the real request path
// (POST /storage/roots, POST /scans, the real scanner and client factory, a real loopback WebDAV server): a loopback server is refused by
// default and reachable only when CATALOGIZER_OUTBOUND_ALLOW_CIDRS names it; a private-class destination also needs confirm_private_network.

import (
	"net/http"
	"testing"

	"catalogizer/internal/netguard"

	"github.com/stretchr/testify/require"
)

func TestSECQ1_Live_LoopbackRootRefusedByDefault_AllowedWithAllowList(t *testing.T) {
	srv := davServer(t, "alice", "pw", map[string][]string{"/": {"a.txt", "b.txt"}})
	h := newAPIHarness(t, true)

	// default: no allow-list, loopback refused at creation, nothing stored.
	t.Setenv(netguard.AllowCIDRsEnv, "")
	code, body := h.do("POST", "/storage/roots", map[string]interface{}{"name": "lb", "protocol": "webdav", "url": srv.URL, "username": "alice", "password": "pw", "path": "/"})
	require.Equal(t, http.StatusBadRequest, code, "%v", body)
	require.Equal(t, 0, rootCount(t, h.db))

	// allow-listed: created (the harness adds confirm_private_network), scanned for real.
	t.Setenv(netguard.AllowCIDRsEnv, "127.0.0.0/8")
	code, body = h.do("POST", "/storage/roots", map[string]interface{}{"name": "lb", "protocol": "webdav", "url": srv.URL, "username": "alice", "password": "pw", "path": "/"})
	require.Equal(t, http.StatusCreated, code, "%v", body)
	id := int64(body["id"].(float64))
	code, body = h.do("POST", "/scans", map[string]interface{}{"storage_root_id": id})
	require.Equal(t, http.StatusAccepted, code, "%v", body)
	st := h.waitScan(body["job_id"].(string))
	require.Equal(t, "completed", st["status"], "reason: %v", st["reason"])
	require.Equal(t, float64(2), st["files_found"])

	// the allow-list is withdrawn after the root exists: the CLIENT (not only the create check) refuses the dial, the scan fails.
	t.Setenv(netguard.AllowCIDRsEnv, "")
	code, body = h.do("POST", "/scans", map[string]interface{}{"storage_root_id": id})
	if code == http.StatusAccepted {
		st = h.waitScan(body["job_id"].(string))
		require.Equal(t, "failed", st["status"], "client must refuse a loopback destination without the allow-list: %v", st)
		require.Contains(t, st["reason"], "outbound destination not allowed", "the refusal is the netguard one, not an unrelated failure")
	} else {
		require.GreaterOrEqual(t, code, 400, "scan refused up front is also acceptable: %v", body)
	}
}

// The allow-list alone is not the confirmation: an allow-listed loopback (private class) without confirm_private_network is refused with the code.
func TestSECQ1_Live_AllowListedPrivateStillNeedsConfirmation(t *testing.T) {
	t.Setenv(netguard.AllowCIDRsEnv, "127.0.0.0/8")
	srv := davServer(t, "alice", "pw", map[string][]string{"/": {"a.txt"}})
	h := secq1cHarness(t) // noAutoConfirm
	code, body := h.do("POST", "/storage/roots", map[string]interface{}{"name": "lb2", "protocol": "webdav", "url": srv.URL, "username": "alice", "password": "pw", "path": "/"})
	require.Equal(t, http.StatusBadRequest, code, "%v", body)
	require.Equal(t, secq1cNoConfirm, body["code"])
	require.Equal(t, 0, rootCount(t, h.db))
}
