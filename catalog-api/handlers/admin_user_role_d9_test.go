package handlers

// D9 (docs/qa/live-conv-docs-subs-r1-20261104.md): PUT /admin/users/:id parsed
// "role" but never applied it, and GET /admin/users never loaded the role.
// These tests drive the REAL gin router, REAL migrated SQLite, REAL
// UserRepository/AuthService/AdminHandler (no mocks).

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type d9Env struct {
	*rtEnv
	userTok string
	userID  int
}

func newD9Env(t *testing.T) *d9Env {
	t.Helper()
	e := newRTEnv(t)
	adm := NewAdminHandler(&AuthServiceAdapter{Inner: e.authSvc}, e.repo, e.db, "test")
	e.r.GET("/api/v1/admin/users", adm.GetUsers)

	require.Equal(t, 201, e.do(e.adminTok, "POST", "/api/v1/users", createUserBody("dave")).Code)
	u, err := e.repo.GetByUsername("dave")
	require.NoError(t, err)
	w := e.login("dave", rtUserPass)
	require.Equal(t, 200, w.Code, w.Body.String())
	var res struct {
		SessionToken string `json:"session_token"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &res))
	return &d9Env{rtEnv: e, userTok: res.SessionToken, userID: u.ID}
}

func (e *d9Env) roleOf(t *testing.T, username string) string {
	t.Helper()
	w := e.do(e.adminTok, "GET", "/api/v1/admin/users", "")
	require.Equal(t, 200, w.Code, w.Body.String())
	var list []struct {
		Username string `json:"username"`
		Role     string `json:"role"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &list))
	for _, u := range list {
		if u.Username == username {
			return u.Role
		}
	}
	t.Fatalf("user %s not in list", username)
	return ""
}

func (e *d9Env) storedRoleID(t *testing.T, username string) int {
	u, err := e.repo.GetByUsername(username)
	require.NoError(t, err)
	return u.RoleID
}

func put(e *d9Env, tok string, id int, body string) *httptest.ResponseRecorder {
	return e.do(tok, "PUT", "/api/v1/admin/users/"+rtItoa(id), body)
}

func TestD9_ListShowsRealRole(t *testing.T) {
	e := newD9Env(t)
	assert.Equal(t, "admin", e.roleOf(t, "rtadmin"), "the seeded administrator must be listed as admin, not the default user")
	assert.Equal(t, "user", e.roleOf(t, "dave"))
}

func TestD9_PutAppliesRole(t *testing.T) {
	e := newD9Env(t)
	w := put(e, e.adminTok, e.userID, `{"role":"admin"}`)
	require.Equal(t, 200, w.Code, w.Body.String())
	assert.Equal(t, 1, e.storedRoleID(t, "dave"), "role_id must be persisted")
	assert.Equal(t, "admin", e.roleOf(t, "dave"))
	// case-insensitive, and demotion of another admin works while another admin remains
	require.Equal(t, 200, put(e, e.adminTok, e.userID, `{"role":"User"}`).Code)
	assert.Equal(t, 2, e.storedRoleID(t, "dave"))
	assert.Equal(t, "user", e.roleOf(t, "dave"))
}

func TestD9_PutRoleOnlyChangeRevokesSessionsAndAdminGateFollows(t *testing.T) {
	e := newD9Env(t)
	require.Equal(t, 200, put(e, e.adminTok, e.userID, `{"role":"admin"}`).Code)
	// dave's pre-change session is revoked: the old token no longer works.
	assert.Equal(t, 401, put(e, e.userTok, e.userID, `{"status":"active"}`).Code, "old session must be revoked")
	w := e.login("dave", rtUserPass)
	require.Equal(t, 200, w.Code)
	var res struct {
		SessionToken string `json:"session_token"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &res))
	assert.Equal(t, 200, e.do(res.SessionToken, "GET", "/api/v1/admin/ping", "").Code, "promoted user passes the live admin gate after re-login")
}

func TestD9_UnknownRoleRejected(t *testing.T) {
	e := newD9Env(t)
	for _, body := range []string{`{"role":"superuser"}`, `{"role":""}`, `{"role":"  "}`, `{"role":"admin\u0000"}`} {
		w := put(e, e.adminTok, e.userID, body)
		assert.Equal(t, 400, w.Code, body+" -> "+w.Body.String())
		assert.NotContains(t, w.Body.String(), "superuser", "generic body, no echo")
	}
	assert.Equal(t, 2, e.storedRoleID(t, "dave"), "rejected request must not change the role")
}

func TestD9_NonAdminCannotChangeRole(t *testing.T) {
	e := newD9Env(t)
	w := put(e, e.userTok, e.userID, `{"role":"admin"}`)
	assert.Equal(t, 403, w.Code, w.Body.String())
	assert.Equal(t, 2, e.storedRoleID(t, "dave"), "no self-escalation")
	assert.Equal(t, 401, put(e, "", e.userID, `{"role":"admin"}`).Code)
}

func TestD9_LastAdminCannotBeDemoted(t *testing.T) {
	e := newD9Env(t)
	admin, err := e.repo.GetByUsername("rtadmin")
	require.NoError(t, err)
	w := put(e, e.adminTok, admin.ID, `{"role":"user"}`)
	assert.Equal(t, 409, w.Code, w.Body.String())
	assert.Equal(t, 1, e.storedRoleID(t, "rtadmin"))
	assert.Equal(t, 200, e.do(e.adminTok, "GET", "/api/v1/admin/ping", "").Code, "admin must still hold a valid session")
}

func TestD9_AdminCannotDemoteSelfEvenWithAnotherAdmin(t *testing.T) {
	e := newD9Env(t)
	require.Equal(t, 200, put(e, e.adminTok, e.userID, `{"role":"admin"}`).Code)
	admin, _ := e.repo.GetByUsername("rtadmin")
	w := put(e, e.adminTok, admin.ID, `{"role":"user"}`)
	assert.Equal(t, 409, w.Code, w.Body.String())
	assert.Equal(t, 1, e.storedRoleID(t, "rtadmin"))
	// another admin may demote them
	w = e.login("dave", rtUserPass)
	var res struct {
		SessionToken string `json:"session_token"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &res))
	require.Equal(t, 200, put(e, res.SessionToken, admin.ID, `{"role":"user"}`).Code)
	assert.Equal(t, 2, e.storedRoleID(t, "rtadmin"))
	// the demoted admin's sessions were revoked, the old token is dead
	assert.Equal(t, 401, e.do(e.adminTok, "GET", "/api/v1/admin/users", "").Code)
}

func TestD9_SameRoleAndOtherFieldsStillWork(t *testing.T) {
	e := newD9Env(t)
	assert.Equal(t, 200, put(e, e.adminTok, e.userID, `{"role":"user","email":"dave2@example.test"}`).Code)
	u, _ := e.repo.GetByUsername("dave")
	assert.Equal(t, "dave2@example.test", u.Email)
	assert.Equal(t, 2, u.RoleID)
	// admin re-asserting own admin role is a no-op, not an error
	admin, _ := e.repo.GetByUsername("rtadmin")
	assert.Equal(t, 200, put(e, e.adminTok, admin.ID, `{"role":"admin"}`).Code)
}
