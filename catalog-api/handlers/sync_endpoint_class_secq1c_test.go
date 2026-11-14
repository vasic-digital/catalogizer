package handlers

// CAT-SECQ1-C (W4), sync endpoints: private-network confirmation at create, step-up to widen on update, class stored by the production path.
// Real SQLite, real SyncService / SyncRepository / AuthService, a real WebDAV fixture server on loopback (allow-listed, class private).

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"catalogizer/database"
	"catalogizer/internal/netguard"
	"catalogizer/services"

	"github.com/stretchr/testify/require"
)

const syncStepPW = "Sync-Step-Up-9"

func (s *SyncHandlerTestSuite) syncClassOf(id int) (string, string) {
	var cls, src string
	require.NoError(s.T(), s.db.QueryRow(`SELECT approved_class, approved_class_source FROM sync_endpoints WHERE id = ?`, id).Scan(&cls, &src))
	return cls, src
}

func (s *SyncHandlerTestSuite) giveUserPassword() {
	salt := "ffeeddccbbaa99887766554433221100"
	hash, err := services.HashPasswordWithSalt(syncStepPW, salt)
	require.NoError(s.T(), err)
	_, err = s.db.Exec(`UPDATE users SET password_hash = ?, salt = ? WHERE id = ?`, hash, salt, s.testUserID)
	require.NoError(s.T(), err)
}

func (s *SyncHandlerTestSuite) davBody(url string) map[string]interface{} {
	return map[string]interface{}{"name": "dav-sync", "type": "webdav", "url": url, "username": "alice", "password": "pw",
		"sync_direction": "download", "local_path": "/tmp/secq1c-sync", "remote_path": "/"}
}

func (s *SyncHandlerTestSuite) TestSECQ1C_CreatePrivateEndpointNeedsConfirmation() {
	s.T().Setenv(netguard.AllowCIDRsEnv, "127.0.0.0/8")
	srv := openDavSecq1c(s.T())
	body := s.davBody(srv.URL)
	w := s.doRequest("POST", "/sync/endpoints", body, true)
	require.Equal(s.T(), http.StatusConflict, w.Code, "%s", w.Body.String())
	require.Contains(s.T(), w.Body.String(), "private_network_confirmation_required")
	var n int
	require.NoError(s.T(), s.db.QueryRow(`SELECT COUNT(*) FROM sync_endpoints`).Scan(&n))
	require.Equal(s.T(), 0, n, "an unconfirmed private endpoint is not stored")

	body["confirm_private_network"] = true
	w = s.doRequest("POST", "/sync/endpoints", body, true)
	require.Equal(s.T(), http.StatusCreated, w.Code, "%s", w.Body.String())
	cls, src := s.syncClassOf(1)
	require.Equal(s.T(), "private", cls)
	require.Equal(s.T(), "operator", src)
}

func (s *SyncHandlerTestSuite) TestSECQ1C_UpdateWideningNeedsStepUp() {
	s.T().Setenv(netguard.AllowCIDRsEnv, "127.0.0.0/8")
	s.giveUserPassword()
	srv := openDavSecq1c(s.T())
	body := s.davBody(srv.URL)
	body["confirm_private_network"] = true
	require.Equal(s.T(), http.StatusCreated, s.doRequest("POST", "/sync/endpoints", body, true).Code)
	// a change that does not touch the URL does not re-classify and needs no step-up
	w := s.doRequest("PUT", "/sync/endpoints/1", map[string]interface{}{"name": "renamed"}, true)
	require.Equal(s.T(), http.StatusOK, w.Code, "%s", w.Body.String())
	cls, _ := s.syncClassOf(1)
	require.Equal(s.T(), "private", cls)

	// the operator then narrows this endpoint to public
	require.NoError(s.T(), s.db.SetApprovedClass(ctxBG(), database.TableSyncEndpoints, 1, netguard.ClassPublic, database.ApprovedSourceOperator))

	// a new URL that classifies private would widen the approval: step-up
	other := openDavSecq1c(s.T())
	up := map[string]interface{}{"url": other.URL, "confirm_private_network": true}
	w = s.doRequest("PUT", "/sync/endpoints/1", up, true)
	require.Equal(s.T(), http.StatusForbidden, w.Code, "%s", w.Body.String())
	require.Contains(s.T(), w.Body.String(), "step_up_required")
	up["step_up_password"] = "wrong"
	w = s.doRequest("PUT", "/sync/endpoints/1", up, true)
	require.Equal(s.T(), http.StatusForbidden, w.Code, "%s", w.Body.String())
	require.Contains(s.T(), w.Body.String(), "step_up_failed")
	cls, _ = s.syncClassOf(1)
	require.Equal(s.T(), "public", cls, "a refused widening changes nothing")

	up["step_up_password"] = syncStepPW
	w = s.doRequest("PUT", "/sync/endpoints/1", up, true)
	require.Equal(s.T(), http.StatusOK, w.Code, "%s", w.Body.String())
	cls, src := s.syncClassOf(1)
	require.Equal(s.T(), "private", cls)
	require.Equal(s.T(), "operator", src)
}

func (s *SyncHandlerTestSuite) TestSECQ1C_UpdateToPrivateURLNeedsConfirmation() {
	s.T().Setenv(netguard.AllowCIDRsEnv, "127.0.0.0/8")
	srv := openDavSecq1c(s.T())
	body := s.davBody(srv.URL)
	body["confirm_private_network"] = true
	require.Equal(s.T(), http.StatusCreated, s.doRequest("POST", "/sync/endpoints", body, true).Code)
	other := openDavSecq1c(s.T())
	w := s.doRequest("PUT", "/sync/endpoints/1", map[string]interface{}{"url": other.URL}, true)
	require.Equal(s.T(), http.StatusConflict, w.Code, "%s", w.Body.String())
	require.Contains(s.T(), w.Body.String(), "private_network_confirmation_required")
}

func ctxBG() context.Context { return context.Background() }

// openDavSecq1c is a WebDAV fixture without authentication (the sync endpoint password is never part of the JSON body).
func openDavSecq1c(t testing.TB) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PROPFIND" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusMultiStatus)
		_, _ = io.WriteString(w, `<?xml version="1.0"?><D:multistatus xmlns:D="DAV:"><D:response><D:href>/</D:href><D:propstat><D:prop><D:resourcetype><D:collection/></D:resourcetype></D:prop><D:status>HTTP/1.1 200 OK</D:status></D:propstat></D:response></D:multistatus>`)
	}))
	t.Cleanup(srv.Close)
	return srv
}
