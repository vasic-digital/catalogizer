package handlers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"catalogizer/models"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// permAuth has REAL permission semantics (exact match against a per-user set);
// nothing is stubbed to "true". Tokens: admin(1), plain(2), viewer(3),
// updater(4), manager(5), reporter(6), reportviewer(7).
type permAuth struct {
	got map[string]string
}

var tokenUsers = map[string]int{"creator": 12, "mgr2": 8, "admin": 1, "plain": 2, "viewer": 3, "updater": 4, "manager": 5, "reporter": 6, "reportviewer": 7}

var userPerms = map[int][]string{
	12: {models.PermissionUserCreate},
	8:  {models.PermissionUserManage, models.PermissionUserDelete, models.PermissionUserUpdate},
	1: {models.PermissionSystemAdmin, models.PermissionUserView, models.PermissionUserUpdate, models.PermissionUserDelete,
		models.PermissionUserManage, models.PermissionUserCreate, models.PermissionReportView, models.PermissionReportCreate, models.PermissionSystemConfig},
	2: {},
	3: {models.PermissionUserView},
	4: {models.PermissionUserUpdate},
	5: {models.PermissionUserUpdate, models.PermissionUserManage},
	6: {models.PermissionReportView, models.PermissionReportCreate},
	7: {models.PermissionReportView},
}

func (a *permAuth) CheckPermission(uid int, perm string) (bool, error) {
	for _, p := range userPerms[uid] {
		if p == perm {
			return true, nil
		}
	}
	return false, nil
}
func (a *permAuth) GetCurrentUser(tok string) (*models.User, error) {
	id, ok := tokenUsers[tok]
	if !ok {
		return nil, models.ErrUnauthorized
	}
	return &models.User{ID: id}, nil
}
func (a *permAuth) ValidateToken(tok string) (*models.User, error) { return a.GetCurrentUser(tok) }
func (a *permAuth) ValidatePassword(string) error                  { return nil }
func (a *permAuth) ResetPassword(id int, _ string) error {
	a.got["ResetPassword"] = strconv.Itoa(id)
	return nil
}
func (a *permAuth) LockAccount(id int, _ time.Time) error {
	a.got["LockAccount"] = strconv.Itoa(id)
	return nil
}
func (a *permAuth) UnlockAccount(id int) error {
	a.got["UnlockAccount"] = strconv.Itoa(id)
	return nil
}

// stateful in-memory user/role store (the handler under test is the real one).
type memStore struct {
	users map[int]*models.User
	roles map[int]*models.Role
}

func newMemStore() *memStore {
	s := &memStore{users: map[int]*models.User{}, roles: map[int]*models.Role{}}
	for id := 1; id <= 11; id++ {
		s.users[id] = &models.User{ID: id, Username: "u" + strconv.Itoa(id), Email: "u" + strconv.Itoa(id) + "@x.io", RoleID: 2, IsActive: true}
	}
	s.roles[2] = &models.Role{ID: 2, Name: "user"}
	s.roles[1] = &models.Role{ID: 1, Name: "admin", Permissions: models.Permissions{models.PermissionSystemAdmin}}
	s.users[1].RoleID = 1                                                                              // admin caller
	s.users[6].RoleID = 1                                                                              // a second admin: the escalation target
	s.roles[3] = &models.Role{ID: 3, Name: "seeded-admin", Permissions: models.Permissions{"*"}}       // real seed shape
	s.roles[4] = &models.Role{ID: 4, Name: "system-wild", Permissions: models.Permissions{"system.*"}} // wildcard-pattern shape
	s.users[9].RoleID = 3
	s.users[10].RoleID = 4
	s.users[11].RoleID = 666 // role that cannot be resolved
	s.roles[9] = &models.Role{ID: 9, Name: "other"}
	s.roles[7] = &models.Role{ID: 7, Name: "custom"}
	return s
}
func (s *memStore) GetByID(id int) (*models.User, error) {
	u, ok := s.users[id]
	if !ok {
		return nil, assert.AnError
	}
	c := *u
	return &c, nil
}
func (s *memStore) Update(u *models.User) error { c := *u; s.users[u.ID] = &c; return nil }
func (s *memStore) Delete(id int) error         { delete(s.users, id); return nil }
func (s *memStore) GetRole(id int) (*models.Role, error) {
	if id == 666 {
		return nil, assert.AnError // unresolvable role
	}
	if r, ok := s.roles[id]; ok {
		c := *r
		return &c, nil
	}
	return nil, errors.New("role not found") // like the real repository (CAT-052)
}
func (s *memStore) CreateRole(*models.Role) (int, error) { return 0, nil }
func (s *memStore) UpdateRole(r *models.Role) error      { c := *r; s.roles[r.ID] = &c; return nil }
func (s *memStore) DeleteRole(id int) error              { delete(s.roles, id); return nil }
func (s *memStore) ListRoles() ([]models.Role, error)    { return nil, nil }

// Methods the handlers' interfaces require but these tests never reach.
func (s *memStore) List(int, int) ([]models.User, error) { return nil, nil }
func (s *memStore) Count() (int, error)                  { return 0, nil }

// DeactivateAllUserSessions: the handlers require it; these tests keep no sessions.
func (s *memStore) DeactivateAllUserSessions(int) error { return nil }

type authzEnv struct {
	r     *gin.Engine
	store *memStore
	got   map[string]string
}

func do(e *authzEnv, tok, method, url, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, url, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	w := httptest.NewRecorder()
	e.r.ServeHTTP(w, req)
	return w
}

func newUserRoleEnv() *authzEnv {
	gin.SetMode(gin.TestMode)
	e := &authzEnv{store: newMemStore(), got: map[string]string{}}
	auth := &permAuth{got: e.got}
	uh := NewUserHandler(e.store, auth)
	rh := NewRoleHandler(e.store, auth)
	r := gin.New()
	r.Use(gin.Recovery())
	r.POST("/api/v1/users", WrapHTTPHandler(uh.CreateUser))
	r.GET("/api/v1/users/:id", WrapHTTPHandler(uh.GetUser))
	r.PUT("/api/v1/users/:id", WrapHTTPHandler(uh.UpdateUser))
	r.DELETE("/api/v1/users/:id", WrapHTTPHandler(uh.DeleteUser))
	r.POST("/api/v1/users/:id/reset-password", WrapHTTPHandler(uh.ResetPassword))
	r.POST("/api/v1/users/:id/lock", WrapHTTPHandler(uh.LockAccount))
	r.POST("/api/v1/users/:id/unlock", WrapHTTPHandler(uh.UnlockAccount))
	r.GET("/api/v1/roles", WrapHTTPHandler(rh.ListRoles))
	r.GET("/api/v1/roles/:id", WrapHTTPHandler(rh.GetRole))
	r.PUT("/api/v1/roles/:id", WrapHTTPHandler(rh.UpdateRole))
	r.DELETE("/api/v1/roles/:id", WrapHTTPHandler(rh.DeleteRole))
	e.r = r
	return e
}

func TestUserRoutes_Authorization(t *testing.T) {
	t.Run("unauthenticated is 401", func(t *testing.T) {
		e := newUserRoleEnv()
		assert.Equal(t, 401, do(e, "", "DELETE", "/api/v1/users/5", "").Code)
		assert.Contains(t, e.store.users, 5)
	})
	t.Run("DeleteUser: non-authorized 403 and user kept; admin 204 and gone", func(t *testing.T) {
		e := newUserRoleEnv()
		for _, tok := range []string{"plain", "viewer", "updater", "manager"} {
			assert.Equal(t, 403, do(e, tok, "DELETE", "/api/v1/users/5", "").Code, tok)
		}
		require.Contains(t, e.store.users, 5)
		assert.Equal(t, 204, do(e, "admin", "DELETE", "/api/v1/users/5", "").Code)
		assert.NotContains(t, e.store.users, 5)
	})
	t.Run("DeleteUser: self-delete is 400 and the account survives (M10)", func(t *testing.T) {
		e := newUserRoleEnv()
		assert.Equal(t, 400, do(e, "admin", "DELETE", "/api/v1/users/1", "").Code)
		assert.Contains(t, e.store.users, 1)
	})
	t.Run("ResetPassword: 403 without user.manage, nothing reset (M11); admin ok", func(t *testing.T) {
		e := newUserRoleEnv()
		for _, tok := range []string{"plain", "viewer", "updater"} {
			assert.Equal(t, 403, do(e, tok, "POST", "/api/v1/users/5/reset-password", `{"new_password":"x"}`).Code, tok)
		}
		assert.Empty(t, e.got["ResetPassword"])
		assert.Equal(t, 200, do(e, "manager", "POST", "/api/v1/users/5/reset-password", `{"new_password":"x"}`).Code)
		assert.Equal(t, "5", e.got["ResetPassword"])
	})
	t.Run("Lock/Unlock: 403 without user.manage; self-lock 400", func(t *testing.T) {
		e := newUserRoleEnv()
		body := `{"lock_until":"2030-01-01T00:00:00Z"}`
		for _, tok := range []string{"plain", "viewer", "updater"} {
			assert.Equal(t, 403, do(e, tok, "POST", "/api/v1/users/5/lock", body).Code, tok)
			assert.Equal(t, 403, do(e, tok, "POST", "/api/v1/users/5/unlock", "").Code, tok)
		}
		assert.Empty(t, e.got["LockAccount"])
		assert.Empty(t, e.got["UnlockAccount"])
		assert.Equal(t, 400, do(e, "manager", "POST", "/api/v1/users/5/lock", body).Code, "self-lock")
		assert.Empty(t, e.got["LockAccount"])
		assert.Equal(t, 200, do(e, "manager", "POST", "/api/v1/users/4/lock", body).Code)
		assert.Equal(t, "4", e.got["LockAccount"])
		assert.Equal(t, 200, do(e, "manager", "POST", "/api/v1/users/4/unlock", "").Code)
		assert.Equal(t, "4", e.got["UnlockAccount"])
	})
	t.Run("GetUser: others need user.view, self is allowed", func(t *testing.T) {
		e := newUserRoleEnv()
		assert.Equal(t, 403, do(e, "plain", "GET", "/api/v1/users/5", "").Code)
		assert.Equal(t, 200, do(e, "plain", "GET", "/api/v1/users/2", "").Code, "self")
		assert.Equal(t, 200, do(e, "viewer", "GET", "/api/v1/users/5", "").Code)
	})
	t.Run("UpdateUser: others need user.update", func(t *testing.T) {
		e := newUserRoleEnv()
		assert.Equal(t, 403, do(e, "plain", "PUT", "/api/v1/users/5", `{"email":"h@x.io"}`).Code)
		assert.Equal(t, "u5@x.io", e.store.users[5].Email)
	})
	t.Run("UpdateUser: self-update ignores role_id/is_active but applies profile fields (M12)", func(t *testing.T) {
		e := newUserRoleEnv()
		// plain (id 2) edits itself, trying to escalate role and flip active.
		w := do(e, "plain", "PUT", "/api/v1/users/2", `{"email":"new@x.io","role_id":9,"is_active":false}`)
		require.Equal(t, 200, w.Code, w.Body.String())
		assert.Equal(t, "new@x.io", e.store.users[2].Email)
		assert.Equal(t, 2, e.store.users[2].RoleID, "stored role must be unchanged")
		assert.True(t, e.store.users[2].IsActive, "stored is_active must be unchanged")
		// manager (id 5) self-update must not change its own role either.
		w = do(e, "manager", "PUT", "/api/v1/users/5", `{"role_id":9}`)
		require.Equal(t, 200, w.Code)
		assert.Equal(t, 2, e.store.users[5].RoleID)
	})
	t.Run("UpdateUser: role_id/is_active on others only with user.manage", func(t *testing.T) {
		e := newUserRoleEnv()
		require.Equal(t, 200, do(e, "updater", "PUT", "/api/v1/users/3", `{"email":"a@x.io","role_id":9,"is_active":false}`).Code)
		assert.Equal(t, "a@x.io", e.store.users[3].Email)
		assert.Equal(t, 2, e.store.users[3].RoleID, "updater lacks user.manage")
		assert.True(t, e.store.users[3].IsActive)
		require.Equal(t, 200, do(e, "manager", "PUT", "/api/v1/users/3", `{"role_id":9,"is_active":false}`).Code)
		assert.Equal(t, 9, e.store.users[3].RoleID, "positive control: manager may change role")
		assert.False(t, e.store.users[3].IsActive)
	})
}

func TestRoleRoutes_Authorization(t *testing.T) {
	e := newUserRoleEnv()
	for _, tok := range []string{"plain", "viewer", "updater", "manager"} {
		assert.Equal(t, 403, do(e, tok, "GET", "/api/v1/roles", "").Code, "list "+tok)
		assert.Equal(t, 403, do(e, tok, "GET", "/api/v1/roles/7", "").Code, "get "+tok)
		assert.Equal(t, 403, do(e, tok, "PUT", "/api/v1/roles/7", `{"name":"hacked"}`).Code, "put "+tok)
		assert.Equal(t, 403, do(e, tok, "DELETE", "/api/v1/roles/7", "").Code, "delete "+tok)
	}
	assert.Equal(t, "custom", e.store.roles[7].Name, "role untouched by denied callers")
	assert.Contains(t, e.store.roles, 7, "role not deleted by denied callers (M13)")
	assert.Equal(t, 200, do(e, "admin", "PUT", "/api/v1/roles/7", `{"name":"renamed"}`).Code)
	assert.Equal(t, "renamed", e.store.roles[7].Name)
	assert.Equal(t, 204, do(e, "admin", "DELETE", "/api/v1/roles/7", "").Code)
	assert.NotContains(t, e.store.roles, 7)
}

// F5: PUT without "name" used to dereference a nil *string and panic (500).
func TestUpdateRole_MissingNameIs400NotPanic(t *testing.T) {
	e := newUserRoleEnv()
	w := do(e, "admin", "PUT", "/api/v1/roles/7", `{"description":"d"}`)
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Equal(t, "custom", e.store.roles[7].Name)
}

// --- log / error / config-wizard handler authorization ---------------------

func newHandlersEnv() *authzEnv {
	gin.SetMode(gin.TestMode)
	e := &authzEnv{got: map[string]string{}}
	auth := &permAuth{got: e.got}
	lsvc := &recLogSvc{got: e.got}
	lh := NewLogManagementHandler(lsvc, auth)
	eh := NewErrorReportingHandler(&recErrSvc{got: e.got}, auth)
	ch := NewConfigurationHandler(&recCfgSvc{got: e.got}, auth)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(func(c *gin.Context) { // stands in for the JWT middleware: token -> user_id
		if id, ok := tokenUsers[strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")]; ok {
			c.Set("user_id", id)
		}
		c.Next()
	})
	r.GET("/api/v1/logs/collections/:id", WrapHTTPHandler(lh.GetLogCollection))
	r.GET("/api/v1/logs/collections/:id/entries", WrapHTTPHandler(lh.GetLogEntries))
	r.POST("/api/v1/logs/collections/:id/export", WrapHTTPHandler(lh.ExportLogs))
	r.GET("/api/v1/logs/collections/:id/analyze", WrapHTTPHandler(lh.AnalyzeLogs))
	r.GET("/api/v1/logs/share/:token", WrapHTTPHandler(lh.GetLogShare))
	r.DELETE("/api/v1/logs/share/:id", WrapHTTPHandler(lh.RevokeLogShare))
	r.GET("/api/v1/errors/reports/:id", WrapHTTPHandler(eh.GetErrorReport))
	r.PUT("/api/v1/errors/reports/:id/status", WrapHTTPHandler(eh.UpdateErrorStatus))
	r.GET("/api/v1/errors/crashes/:id", WrapHTTPHandler(eh.GetCrashReport))
	r.PUT("/api/v1/errors/crashes/:id/status", WrapHTTPHandler(eh.UpdateCrashStatus))
	r.GET("/api/v1/configuration/wizard/step/:step_id", WrapHTTPHandler(ch.GetWizardStep))
	r.POST("/api/v1/configuration/wizard/step/:step_id/save", WrapHTTPHandler(ch.SaveWizardProgress))
	r.GET("/api/v1/configuration/wizard/progress", WrapHTTPHandler(ch.GetWizardProgress))
	r.POST("/api/v1/configuration/wizard/complete", WrapHTTPHandler(ch.CompleteWizard))
	e.r = r
	return e
}

func TestLogRoutes_Authorization(t *testing.T) {
	e := newHandlersEnv()
	type rt struct{ method, url, svc, body string }
	for _, x := range []rt{
		{"GET", "/api/v1/logs/collections/42", "GetLogCollection", ""},
		{"GET", "/api/v1/logs/collections/42/entries", "GetLogEntries", ""},
		{"POST", "/api/v1/logs/collections/42/export", "ExportLogs", `{}`},
		{"GET", "/api/v1/logs/collections/42/analyze", "AnalyzeLogs", ""},
		{"DELETE", "/api/v1/logs/share/7", "RevokeLogShare", ""},
	} {
		delete(e.got, x.svc)
		assert.Equal(t, 401, do(e, "", x.method, x.url, x.body).Code, "anon "+x.url)
		assert.Equal(t, 403, do(e, "plain", x.method, x.url, x.body).Code, "plain "+x.url)
		assert.Equal(t, 403, do(e, "reporter", x.method, x.url, x.body).Code, "reporter "+x.url)
		assert.Empty(t, e.got[x.svc], "service must not be reached when denied: "+x.url)
		assert.Less(t, do(e, "admin", x.method, x.url, x.body).Code, 400, "admin "+x.url)
		assert.NotEmpty(t, e.got[x.svc], x.url)
	}
}

// GetLogShare is deliberately token-gated (the share token is the capability):
// the handler itself needs no user_id. In production the whole /api/v1 group
// sits behind jwtMiddleware.RequireAuth() (main.go, api.Use(...)), so callers
// are authenticated there; this test mounts the handler WITHOUT that middleware
// to prove the handler does not itself depend on user_id. Unknown token -> 404
// comes from the recLogSvc stub.
func TestGetLogShare_IsTokenGatedNotUserGated(t *testing.T) {
	e := newHandlersEnv()
	w := do(e, "", "GET", "/api/v1/logs/share/tok-abc", "")
	assert.Equal(t, 404, w.Code, "unknown token, anonymous caller reaches the service")
	assert.Equal(t, "tok-abc", e.got["GetLogShare"])
}

func TestErrorRoutes_Authorization(t *testing.T) {
	e := newHandlersEnv()
	for _, u := range []string{"/api/v1/errors/reports/11", "/api/v1/errors/crashes/12"} {
		assert.Equal(t, 401, do(e, "", "GET", u, "").Code, u)
		assert.Equal(t, 403, do(e, "plain", "GET", u, "").Code, u)
		assert.Equal(t, 200, do(e, "reportviewer", "GET", u, "").Code, u)
		// status updates need report.create, view alone is not enough
		assert.Equal(t, 403, do(e, "reportviewer", "PUT", u+"/status", `{"status":"resolved"}`).Code, u)
		assert.Equal(t, 403, do(e, "plain", "PUT", u+"/status", `{"status":"resolved"}`).Code, u)
		assert.Equal(t, 204, do(e, "reporter", "PUT", u+"/status", `{"status":"resolved"}`).Code, u)
	}
	assert.Equal(t, "11", e.got["UpdateErrorStatus"])
	assert.Equal(t, "12", e.got["UpdateCrashStatus"])
}

// The wizard-step read handler has no permission check by design: any
// authenticated user may read a step definition (the auth middleware guards it).
func TestWizardStep_AuthenticatedUserMayRead(t *testing.T) {
	e := newHandlersEnv()
	w := do(e, "plain", "GET", "/api/v1/configuration/wizard/step/welcome", "")
	assert.Equal(t, 200, w.Code, w.Body.String())
	assert.Equal(t, "welcome", e.got["GetWizardStep"])
}

func (s *memStore) Create(u *models.User) (int, error) {
	id := 100 + len(s.users)
	c := *u
	c.ID = id
	s.users[id] = &c
	return id, nil
}

func (a *permAuth) HashPassword(p string) (string, error)   { return p, nil }
func (a *permAuth) GenerateSecureToken(int) (string, error) { return "t", nil }

// S1: a non-admin with user.manage must not act on an Admin user nor mint an
// admin. Admin callers and plain-user targets are the positive controls.
func TestUserManage_CannotTouchAdminsOrPromoteToAdmin(t *testing.T) {
	body := `{"lock_until":"2030-01-01T00:00:00Z"}`
	for _, tok := range []string{"manager", "mgr2"} {
		t.Run(tok, func(t *testing.T) {
			e := newUserRoleEnv()
			assert.Equal(t, 403, do(e, tok, "POST", "/api/v1/users/6/reset-password", `{"new_password":"x"}`).Code)
			assert.Equal(t, 403, do(e, tok, "POST", "/api/v1/users/6/lock", body).Code)
			assert.Equal(t, 403, do(e, tok, "POST", "/api/v1/users/6/unlock", "").Code)
			assert.Equal(t, 403, do(e, tok, "PUT", "/api/v1/users/6", `{"email":"pwn@x.io"}`).Code)
			assert.Empty(t, e.got["ResetPassword"]+e.got["LockAccount"]+e.got["UnlockAccount"])
			assert.Equal(t, "u6@x.io", e.store.users[6].Email)
			// promotion: assigning the admin role to a plain user is refused
			assert.Equal(t, 403, do(e, tok, "PUT", "/api/v1/users/3", `{"role_id":1}`).Code)
			assert.Equal(t, 2, e.store.users[3].RoleID, "plain user must not be promoted")
			// positive controls: a plain target and a non-admin role still work
			assert.Equal(t, 200, do(e, tok, "POST", "/api/v1/users/3/unlock", "").Code)
			assert.Equal(t, 200, do(e, tok, "PUT", "/api/v1/users/3", `{"role_id":9}`).Code)
			assert.Equal(t, 9, e.store.users[3].RoleID)
		})
	}
	t.Run("DeleteUser: user.delete without admin cannot delete an admin", func(t *testing.T) {
		e := newUserRoleEnv()
		assert.Equal(t, 403, do(e, "mgr2", "DELETE", "/api/v1/users/6", "").Code)
		assert.Contains(t, e.store.users, 6)
		assert.Equal(t, 204, do(e, "mgr2", "DELETE", "/api/v1/users/3", "").Code, "plain target still deletable")
	})
	t.Run("admin caller unchanged", func(t *testing.T) {
		e := newUserRoleEnv()
		assert.Equal(t, 200, do(e, "admin", "POST", "/api/v1/users/6/reset-password", `{"new_password":"x"}`).Code)
		assert.Equal(t, 200, do(e, "admin", "POST", "/api/v1/users/6/lock", body).Code)
		assert.Equal(t, 200, do(e, "admin", "POST", "/api/v1/users/6/unlock", "").Code)
		assert.Equal(t, 200, do(e, "admin", "PUT", "/api/v1/users/3", `{"role_id":1}`).Code)
		assert.Equal(t, 1, e.store.users[3].RoleID)
		assert.Equal(t, 204, do(e, "admin", "DELETE", "/api/v1/users/6", "").Code)
	})
}

// S2: wizard handlers must answer 401, not panic, when user_id is absent.
func TestWizardHandlers_MissingUserIDIs401(t *testing.T) {
	e := newHandlersEnv()
	for _, x := range [][3]string{
		{"POST", "/api/v1/configuration/wizard/step/welcome/save", `{}`},
		{"GET", "/api/v1/configuration/wizard/progress", ""},
		{"POST", "/api/v1/configuration/wizard/complete", `{}`},
	} {
		assert.Equal(t, 401, do(e, "", x[0], x[1], x[2]).Code, x[1])
	}
	assert.Equal(t, 200, do(e, "plain", "POST", "/api/v1/configuration/wizard/step/welcome/save", `{}`).Code)
	assert.Equal(t, "welcome", e.got["SaveWizardProgress"])
}

func newUserBody(role string) string {
	return `{"username":"newbie","email":"n@x.io","password":"Password1!","role_id":` + role + `}`
}

// R3-S1: CreateUser must not let a user.create-only caller mint an admin.
func TestCreateUser_CannotMintAdmin(t *testing.T) {
	for _, role := range []string{"1", "3", "4", "666"} { // Admin, ["*"], "system.*", unresolvable
		e := newUserRoleEnv()
		before := len(e.store.users)
		assert.Equal(t, 403, do(e, "creator", "POST", "/api/v1/users", newUserBody(role)).Code, "role "+role)
		assert.Equal(t, before, len(e.store.users), "nothing created for role "+role)
	}
	e := newUserRoleEnv()
	assert.Equal(t, 201, do(e, "creator", "POST", "/api/v1/users", newUserBody("9")).Code, "non-admin role is fine")
	assert.Equal(t, 201, do(e, "admin", "POST", "/api/v1/users", newUserBody("1")).Code, "admin may create an admin")
	assert.Equal(t, 403, do(e, "plain", "POST", "/api/v1/users", newUserBody("9")).Code, "needs user.create")
	// R3-N1: role_id must be positive
	assert.Equal(t, 400, do(e, "admin", "POST", "/api/v1/users", newUserBody("-5")).Code)
}

// R3-T1/T2: real admin role shapes and unresolvable roles, as targets and as role_id.
func TestUserManage_AdminShapesAndUnresolvableRoles(t *testing.T) {
	body := `{"lock_until":"2030-01-01T00:00:00Z"}`
	e := newUserRoleEnv()
	for _, target := range []string{"6", "9", "10", "11"} { // Admin, ["*"], "system.*", unresolvable role
		for _, tok := range []string{"manager", "mgr2"} {
			assert.Equal(t, 403, do(e, tok, "POST", "/api/v1/users/"+target+"/reset-password", `{"new_password":"x"}`).Code, tok+" reset "+target)
			assert.Equal(t, 403, do(e, tok, "POST", "/api/v1/users/"+target+"/lock", body).Code, tok+" lock "+target)
			assert.Equal(t, 403, do(e, tok, "POST", "/api/v1/users/"+target+"/unlock", "").Code, tok+" unlock "+target)
			assert.Equal(t, 403, do(e, tok, "PUT", "/api/v1/users/"+target, `{"email":"p@x.io"}`).Code, tok+" update "+target)
		}
		assert.Equal(t, 403, do(e, "mgr2", "DELETE", "/api/v1/users/"+target, "").Code, "delete "+target)
	}
	assert.Empty(t, e.got["ResetPassword"]+e.got["LockAccount"]+e.got["UnlockAccount"])
	for _, role := range []string{"1", "3", "4", "666"} {
		assert.Equal(t, 403, do(e, "manager", "PUT", "/api/v1/users/3", `{"role_id":`+role+`}`).Code, "assign role "+role)
	}
	assert.Equal(t, 2, e.store.users[3].RoleID)
	// R3-N1: non-positive role ids are rejected for everyone
	for _, role := range []string{"0", "-1"} {
		assert.Equal(t, 400, do(e, "admin", "PUT", "/api/v1/users/3", `{"role_id":`+role+`}`).Code, "role_id "+role)
	}
}
