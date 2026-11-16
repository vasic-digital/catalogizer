package handlers

// ADMD1RRRRRRRRRR-m1/n3/n4: the merged 403 of GET /sync/endpoints/{id} and GET /sync/sessions/{id} pinned to the bodies the REAL
// SyncHandler.GetEndpoint / GetSession emit (a real SyncService + AuthService on a real SQLite schema, a non-admin user without the
// view-shares permission) and to the REAL production admin gate (RequireAdmin with the live adminCheck installed exactly as
// main_jwtwire.go newAuthMiddleware does), compared with the description in docs/api/openapi.yaml.

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"catalogizer/middleware"
	"catalogizer/models"
	"catalogizer/repository"
)

// addRestrictedUser inserts a non-admin role (no permissions), a user on it and an active session, and returns its bearer token.
func (s *SyncHandlerTestSuite) addRestrictedUser(userID, roleID int) string {
	t := s.T()
	_, err := s.db.Exec(`INSERT OR IGNORE INTO roles (id, name, description, permissions, is_system) VALUES (?, ?, ?, ?, ?)`, roleID, fmt.Sprintf("restricted%d", roleID), "no permissions", `[]`, false)
	require.NoError(t, err)
	_, err = s.db.Exec(`INSERT INTO users (id, username, email, password_hash, salt, role_id) VALUES (?, ?, ?, ?, ?, ?)`, userID, fmt.Sprintf("u%d", userID), fmt.Sprintf("u%d@example.test", userID), "$2a$10$fakehashfortest", "fakesalt", roleID)
	require.NoError(t, err)
	sid := 100 + userID
	_, err = s.db.Exec(`INSERT INTO user_sessions (id, user_id, session_token, refresh_token, device_info, is_active, expires_at) VALUES (?, ?, ?, ?, ?, ?, ?)`, sid, userID, fmt.Sprintf("tok%d", userID), fmt.Sprintf("ref%d", userID), "{}", true, time.Now().Add(24*time.Hour))
	require.NoError(t, err)
	return s.generateTestToken(userID, fmt.Sprintf("u%d", userID), roleID, fmt.Sprintf("%d", sid))
}

func (s *SyncHandlerTestSuite) getAs(token, path string) (int, map[string]interface{}) {
	req := httptest.NewRequest("GET", path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)
	return w.Code, s.parseResponse(w)
}

func (s *SyncHandlerTestSuite) TestADMD1RRRRRRRRRR_SyncMerged403ContentPinnedToRealBodies() {
	t := s.T()
	// owner-branch bodies: produced by the real handlers for a caller that is neither owner nor holder of share.view
	token := s.addRestrictedUser(20, 20)
	epID := s.createTestEndpointInDB(s.testUserID, "EP", "local", "active")
	sessID := s.createTestSessionInDB(epID, s.testUserID, "completed")
	type ownerBody struct {
		code int
		body map[string]interface{}
	}
	got := map[string]ownerBody{}
	code, b := s.getAs(token, fmt.Sprintf("/sync/endpoints/%d", epID))
	got["/api/v1/sync/endpoints/{id}"] = ownerBody{code, b}
	code, b = s.getAs(token, fmt.Sprintf("/sync/sessions/%d", sessID))
	got["/api/v1/sync/sessions/{id}"] = ownerBody{code, b}

	// admin-branch body: the production gate, the live adminCheck branch (main_jwtwire.go newAuthMiddleware) with a stale role_id=1 claim
	userRepo := repository.NewUserRepository(s.db)
	jm := middleware.NewJWTMiddleware("k")
	jm.SetAdminChecker(userRepo.IsAdmin)
	require.NotNil(t, jm.AdminCheck(), "the live admin check is installed, as in production")
	gate := func(userID int) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/x", nil)
		c.Set("user_id", userID)
		c.Set("role_id", 1) // a stale/forged admin claim: only the database may decide
		jm.RequireAdmin()(c)
		return w
	}
	wDenied := gate(20)
	require.Equal(t, http.StatusForbidden, wDenied.Code, "restricted user is refused although the claim says role_id 1")
	adminBody := s.parseResponse(wDenied)
	require.Equal(t, "Admin role required", adminBody["error"])
	require.Equal(t, false, adminBody["success"])
	require.NotEqual(t, http.StatusForbidden, gate(s.testUserID).Code, "positive control: the real admin user passes the live check")
	// without the live checker the claim-only fallback would have admitted role_id 1: the live branch is what refused above
	fallback := middleware.NewJWTMiddleware("k")
	wf := httptest.NewRecorder()
	cf, _ := gin.CreateTestContext(wf)
	cf.Request = httptest.NewRequest("GET", "/x", nil)
	cf.Set("user_id", 20)
	cf.Set("role_id", 1)
	fallback.RequireAdmin()(cf)
	require.NotEqual(t, http.StatusForbidden, wf.Code, "control: the claim-only fallback trusts role_id and would not have refused")

	d := loadOpenAPI(t)
	paths := d["paths"].(map[string]interface{})
	for _, tc := range []struct{ path, owner, msg string }{
		{"/api/v1/sync/endpoints/{id}", "endpoint owner", "Failed to get sync endpoint"},
		{"/api/v1/sync/sessions/{id}", "session owner", "Failed to get sync session"},
	} {
		ob := got[tc.path]
		require.Equal(t, http.StatusForbidden, ob.code, "%s: %v", tc.path, ob.body)
		require.Equal(t, tc.msg, ob.body["error"], "%s: the real handler's owner-branch message", tc.path)
		require.Equal(t, "not permitted", ob.body["details"], tc.path)
		r := paths[tc.path].(map[string]interface{})["get"].(map[string]interface{})["responses"].(map[string]interface{})["403"].(map[string]interface{})
		desc := r["description"].(string)
		require.Contains(t, desc, `"error":"`+adminBody["error"].(string)+`"`, "%s: admin branch body", tc.path)
		require.Contains(t, desc, fmt.Sprintf(`"error":%q`, ob.body["error"]), "%s: owner branch body", tc.path)
		require.Contains(t, desc, fmt.Sprintf(`"details":%q`, ob.body["details"]), "%s: owner branch details", tc.path)
		require.Contains(t, desc, "not an administrator", tc.path)
		require.Contains(t, desc, "not the "+tc.owner, tc.path)
		require.Contains(t, desc, "view-shares permission", tc.path)
		require.Contains(t, desc, models.PermissionViewShares, tc.path)
		// ADMD1RRRRRRRRRR-n4: the owner branch also fires when the permission lookup itself fails
		require.Contains(t, desc, "permission lookup", tc.path)
		sch := r["content"].(map[string]interface{})["application/json"].(map[string]interface{})["schema"].(map[string]interface{})
		require.Equal(t, "#/components/schemas/Error", sch["$ref"], tc.path)
	}
}

// ADMD1RRRRRRRRRR-n4: the owner-branch 403 is also what a FAILED permission lookup produces (auth_service.go CheckPermission error ->
// "unauthorized"): a user whose role row is gone makes CheckPermission return an error, and the real service answers "unauthorized".
func (s *SyncHandlerTestSuite) TestADMD1RRRRRRRRRR_OwnerBranch403AlsoOnPermissionLookupError() {
	t := s.T()
	s.addRestrictedUser(21, 21)
	epID := s.createTestEndpointInDB(s.testUserID, "EP", "local", "active")
	sessID := s.createTestSessionInDB(epID, s.testUserID, "completed")
	_, err := s.db.Exec(`DELETE FROM roles WHERE id = 21`)
	require.NoError(t, err)
	_, lookupErr := s.authService.CheckPermission(21, models.PermissionViewShares)
	require.Error(t, lookupErr, "precondition: the permission lookup itself fails (dangling role)")
	_, err = s.syncService.GetEndpoint(epID, 21)
	require.ErrorContains(t, err, "unauthorized")
	_, err = s.syncService.GetSession(sessID, 21)
	require.ErrorContains(t, err, "unauthorized")
}
