package handlers

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"catalogizer/database"
	"catalogizer/middleware"
	"catalogizer/models"
	"catalogizer/repository"
	"catalogizer/services"

	"github.com/gin-gonic/gin"
	_ "github.com/mutecomm/go-sqlcipher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Throwaway test-only credentials (never real).
const (
	rtAdminPass = "Adm1n!Throwaway#1"
	rtUserPass  = "Us3r!Throwaway#2"
)

type rtEnv struct {
	r        *gin.Engine
	db       *database.DB
	repo     *repository.UserRepository
	adminTok string
	authSvc  *services.AuthService
}

// newRTEnv builds the REAL stack: production migrations on SQLite, real
// UserRepository, real AuthService (bcrypt), real handlers, and the real
// InputValidation middleware, wired with the same routes main.go uses.
func newRTEnv(t *testing.T) *rtEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)
	sqlDB, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	sqlDB.SetMaxOpenConns(1)
	db := database.WrapDB(sqlDB, database.DialectSQLite)
	require.NoError(t, db.RunMigrations(context.Background()))

	repo := repository.NewUserRepository(db)
	authSvc := services.NewAuthService(repo, "test-jwt-secret-throwaway")
	adapter := &AuthServiceAdapter{Inner: authSvc}

	// Admin created through the canonical hasher (as the seed does).
	hash, salt, err := authSvc.HashPasswordForUser(rtAdminPass)
	require.NoError(t, err)
	_, err = repo.Create(&models.User{Username: "rtadmin", Email: "rtadmin@example.test",
		PasswordHash: hash, Salt: salt, RoleID: 1, IsActive: true})
	require.NoError(t, err)

	uh := NewUserHandler(repo, adapter)
	rh := NewRoleHandler(repo, adapter)
	ah := NewAuthHandler(authSvc)

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.InputValidation(middleware.DefaultInputValidationConfig()))
	r.POST("/api/v1/auth/login", ah.LoginGin)
	r.POST("/api/v1/users", WrapHTTPHandler(uh.CreateUser))
	r.GET("/api/v1/users/:id", WrapHTTPHandler(uh.GetUser))
	r.PUT("/api/v1/users/:id", WrapHTTPHandler(uh.UpdateUser))
	r.POST("/api/v1/users/:id/reset-password", WrapHTTPHandler(uh.ResetPassword))
	mq := NewMediaQueryHandler(db, nil, repo)
	r.POST("/api/v1/auth/change-password", func(c *gin.Context) {
		if uid := c.GetHeader("X-Test-User-ID"); uid != "" {
			n := 0
			for _, ch := range uid {
				n = n*10 + int(ch-'0')
			}
			c.Set("user_id", n)
		}
		mq.ChangePassword(c)
	})
	r.POST("/api/v1/roles", WrapHTTPHandler(rh.CreateRole))
	r.PUT("/api/v1/roles/:id", WrapHTTPHandler(rh.UpdateRole))

	e := &rtEnv{r: r, db: db, repo: repo, authSvc: authSvc}
	w := e.do("", "POST", "/api/v1/auth/login", `{"username":"rtadmin","password":"`+rtAdminPass+`"}`)
	require.Equal(t, 200, w.Code, w.Body.String())
	var res struct {
		SessionToken string `json:"session_token"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &res))
	require.NotEmpty(t, res.SessionToken)
	e.adminTok = res.SessionToken
	return e
}

func (e *rtEnv) do(tok, method, url, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, url, bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	if tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	w := httptest.NewRecorder()
	e.r.ServeHTTP(w, req)
	return w
}

func (e *rtEnv) login(user, pass string) *httptest.ResponseRecorder {
	return e.do("", "POST", "/api/v1/auth/login", `{"username":"`+user+`","password":"`+pass+`"}`)
}

func createUserBody(name string) string {
	return `{"username":"` + name + `","email":"` + name + `@example.test","password":"` + rtUserPass + `","role_id":2}`
}

func TestRT_D1_UserCreatedViaPostUsersCanLogin(t *testing.T) {
	e := newRTEnv(t)
	w := e.do(e.adminTok, "POST", "/api/v1/users", createUserBody("alice"))
	require.Equal(t, 201, w.Code, w.Body.String())

	// The stored hash must be the canonical bcrypt scheme, not SHA-256 hex.
	u, err := e.repo.GetByUsername("alice")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(u.PasswordHash, "$2"), "stored hash must be bcrypt")

	assert.Equal(t, 200, e.login("alice", rtUserPass).Code, "POST /users user must be able to log in")
	assert.Equal(t, 401, e.login("alice", rtUserPass+"x").Code, "wrong password must still be rejected")
}

func TestRT_D1_LoginAfterAdminResetStillWorks(t *testing.T) {
	e := newRTEnv(t)
	require.Equal(t, 201, e.do(e.adminTok, "POST", "/api/v1/users", createUserBody("bob")).Code)
	u, err := e.repo.GetByUsername("bob")
	require.NoError(t, err)
	body := `{"new_password":"N3w!Throwaway#3"}`
	require.Equal(t, 200, e.do(e.adminTok, "POST", "/api/v1/users/"+rtItoa(u.ID)+"/reset-password", body).Code)
	assert.Equal(t, 200, e.login("bob", "N3w!Throwaway#3").Code)
	assert.Equal(t, 401, e.login("bob", rtUserPass).Code)
}

// Existing rows written by the buggy CreateUser (hex SHA-256 of password+salt)
// must still log in once and are upgraded to bcrypt on that login.
func TestRT_D1_LegacySHA256HashVerifiedAndUpgraded(t *testing.T) {
	e := newRTEnv(t)
	salt := "00112233445566778899aabbccddeeff"
	sum := sha256.Sum256([]byte(rtUserPass + salt))
	legacy := hex.EncodeToString(sum[:])
	_, err := e.repo.Create(&models.User{Username: "carol", Email: "carol@example.test",
		PasswordHash: legacy, Salt: salt, RoleID: 2, IsActive: true})
	require.NoError(t, err)

	assert.Equal(t, 401, e.login("carol", rtUserPass+"x").Code, "wrong password rejected for legacy hash")
	u, _ := e.repo.GetByUsername("carol")
	assert.Equal(t, legacy, u.PasswordHash, "failed login must not upgrade")

	assert.Equal(t, 200, e.login("carol", rtUserPass).Code, "legacy SHA-256 user logs in")
	u, err = e.repo.GetByUsername("carol")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(u.PasswordHash, "$2"), "hash upgraded to bcrypt after login")
	assert.Equal(t, 200, e.login("carol", rtUserPass).Code, "still logs in after upgrade")
}

func TestRT_D1_CanonicalHasherUnit(t *testing.T) {
	h, err := services.HashPasswordWithSalt("pw-Throwaway1!", "salt1")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(h, "$2"))
	ok, up := services.CheckPassword("pw-Throwaway1!", "salt1", h)
	assert.True(t, ok)
	assert.False(t, up)
	ok, _ = services.CheckPassword("other", "salt1", h)
	assert.False(t, ok)
	ok, _ = services.CheckPassword("pw-Throwaway1!", "salt2", h)
	assert.False(t, ok)
	// garbage / empty never verify
	for _, bad := range []string{"", "x", strings.Repeat("g", 64), strings.Repeat("0", 64)} {
		ok, _ = services.CheckPassword("pw-Throwaway1!", "salt1", bad)
		assert.False(t, ok, bad)
	}
	sum := sha256.Sum256([]byte("pw-Throwaway1!" + "salt1"))
	ok, up = services.CheckPassword("pw-Throwaway1!", "salt1", hex.EncodeToString(sum[:]))
	assert.True(t, ok)
	assert.True(t, up, "legacy match must request upgrade")
}

func TestRT_D2_RoleWithDottedPermissionsAccepted(t *testing.T) {
	e := newRTEnv(t)
	body := `{"name":"Editors","description":"Can create and update items from the library","permissions":["user.create","user.update","user.delete","media.view"]}`
	w := e.do(e.adminTok, "POST", "/api/v1/roles", body)
	require.Equal(t, 201, w.Code, w.Body.String())
	var role models.Role
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &role))
	assert.ElementsMatch(t, []string{"user.create", "user.update", "user.delete", "media.view"}, role.Permissions)
	got, err := e.repo.GetRole(role.ID)
	require.NoError(t, err)
	assert.ElementsMatch(t, role.Permissions, got.Permissions)

	// same false positive on update
	w = e.do(e.adminTok, "PUT", "/api/v1/roles/"+rtItoa(role.ID), `{"name":"Editors","permissions":["user.delete","user.update"]}`)
	assert.Equal(t, 200, w.Code, w.Body.String())
}

// Exactly the reported case: only the permission names are "suspicious".
func TestRT_D2_PermissionNamesAloneAccepted(t *testing.T) {
	e := newRTEnv(t)
	for i, perm := range []string{"user.create", "user.update", "user.delete", "role.create", "media.delete", "select", "from.where"} {
		body := `{"name":"Role` + rtItoa(i) + `","description":"plain","permissions":["` + perm + `"]}`
		w := e.do(e.adminTok, "POST", "/api/v1/roles", body)
		assert.Equal(t, 201, w.Code, "%s: %s", perm, w.Body.String())
	}
}

func TestRT_D2_RealInjectionPayloadsStillRejected(t *testing.T) {
	e := newRTEnv(t)
	payloads := []string{
		`'; DROP TABLE users; --`,
		`' OR '1'='1`,
		`x' OR 1=1 --`,
		`1 UNION SELECT password_hash FROM users`,
		`1 UNION ALL SELECT NULL,NULL`,
		`SELECT * FROM users`,
		`admin'--`,
		`1; DELETE FROM roles`,
		`1; SLEEP(5)`,
		`WAITFOR DELAY '0:0:5'`,
		`EXEC(xp_cmdshell 'dir')`,
		`user.create'; DROP TABLE roles; --`,
		`INSERT INTO roles VALUES (9,'x')`,
		`UPDATE users SET role_id=1`,
		`1 AND 1=1`,
	}
	for _, p := range payloads {
		for _, field := range []string{"name", "perm"} {
			j, _ := json.Marshal(p)
			var body string
			if field == "name" {
				body = `{"name":` + string(j) + `,"permissions":["media.view"]}`
			} else {
				body = `{"name":"Probe","permissions":[` + string(j) + `]}`
			}
			w := e.do(e.adminTok, "POST", "/api/v1/roles", body)
			assert.Equal(t, 400, w.Code, "%s in %s must be rejected: %s", p, field, w.Body.String())
		}
	}
}

func rtItoa(i int) string {
	b, _ := json.Marshal(i)
	return string(b)
}

var _ = http.StatusOK

// A POST /users user (and a legacy-hash user) can change their own password via
// the gin change-password endpoint, and the new password logs in.
func TestRT_D1_ChangePasswordForCreatedAndLegacyUsers(t *testing.T) {
	e := newRTEnv(t)
	require.Equal(t, 201, e.do(e.adminTok, "POST", "/api/v1/users", createUserBody("dave")).Code)
	u, err := e.repo.GetByUsername("dave")
	require.NoError(t, err)

	salt := "ffeeddccbbaa99887766554433221100"
	sum := sha256.Sum256([]byte(rtUserPass + salt))
	_, err = e.repo.Create(&models.User{Username: "erin", Email: "erin@example.test",
		PasswordHash: hex.EncodeToString(sum[:]), Salt: salt, RoleID: 2, IsActive: true})
	require.NoError(t, err)
	erin, err := e.repo.GetByUsername("erin")
	require.NoError(t, err)

	for _, tc := range []struct {
		name string
		id   int
	}{{"dave", u.ID}, {"erin", erin.ID}} {
		req := httptest.NewRequest("POST", "/api/v1/auth/change-password",
			strings.NewReader(`{"current_password":"`+rtUserPass+`","new_password":"Ch4nged!Throwaway#4"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Test-User-ID", rtItoa(tc.id))
		w := httptest.NewRecorder()
		e.r.ServeHTTP(w, req)
		require.Equal(t, 200, w.Code, tc.name+": "+w.Body.String())
		assert.Equal(t, 200, e.login(tc.name, "Ch4nged!Throwaway#4").Code, tc.name)
		assert.Equal(t, 401, e.login(tc.name, rtUserPass).Code, tc.name)
	}
}

// D4: passwords are hashed, never interpolated, so SQL/XSS-looking words in a
// password must not be rejected by the validator; other fields still are.
func TestRT_D4_PasswordsWithSQLKeywordsAccepted(t *testing.T) {
	e := newRTEnv(t)
	for i, pw := range []string{"Create!Throwaway1", "Select-From-Where#9a", "Drop--Table;Aa1", "x'OR'1'='1Aa1!", "Uni0n SELECT *aA", "<script>Aa1!x"} {
		name := "pwuser" + rtItoa(i)
		body := `{"username":"` + name + `","email":"` + name + `@example.test","password":` + jsonStr(pw) + `,"role_id":2}`
		w := e.do(e.adminTok, "POST", "/api/v1/users", body)
		require.Equal(t, 201, w.Code, pw+": "+w.Body.String())
		assert.Equal(t, 200, e.login(name, pw).Code, "login with "+pw)
	}
	// reset-password new_password too
	u, _ := e.repo.GetByUsername("pwuser0")
	w := e.do(e.adminTok, "POST", "/api/v1/users/"+rtItoa(u.ID)+"/reset-password", `{"new_password":"Create-Update-Delete#1"}`)
	assert.Equal(t, 200, w.Code, w.Body.String())
	// a non-credential field with a real payload is still rejected
	w = e.do(e.adminTok, "POST", "/api/v1/users", `{"username":"x'; DROP TABLE users; --","email":"q@example.test","password":"`+rtUserPass+`","role_id":2}`)
	assert.Equal(t, 400, w.Code)
	w = e.do(e.adminTok, "POST", "/api/v1/users", `{"username":"okname","email":"q@example.test","password":"`+rtUserPass+`","role_id":2,"first_name":"' OR '1'='1"}`)
	assert.Equal(t, 400, w.Code)
}

func jsonStr(s string) string { b, _ := json.Marshal(s); return string(b) }

func (e *rtEnv) roleOf(t *testing.T, username string) int {
	t.Helper()
	u, err := e.repo.GetByUsername(username)
	require.NoError(t, err)
	return u.RoleID
}

// D3: PUT /users/{id} must persist role_id; GET must show it; privilege rules hold.
func TestRT_D3_UpdateUserPersistsRole(t *testing.T) {
	e := newRTEnv(t)
	w := e.do(e.adminTok, "POST", "/api/v1/roles", `{"name":"Staff","permissions":["media.view"]}`)
	require.Equal(t, 201, w.Code, w.Body.String())
	var staff models.Role
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &staff))
	w = e.do(e.adminTok, "POST", "/api/v1/roles", `{"name":"UserManagers","permissions":["user.view","user.update","user.manage"]}`)
	require.Equal(t, 201, w.Code, w.Body.String())
	var mgrRole models.Role
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &mgrRole))

	require.Equal(t, 201, e.do(e.adminTok, "POST", "/api/v1/users", createUserBody("target")).Code)
	require.Equal(t, 201, e.do(e.adminTok, "POST", "/api/v1/users",
		`{"username":"mgr","email":"mgr@example.test","password":"`+rtUserPass+`","role_id":`+rtItoa(mgrRole.ID)+`}`).Code)
	target, _ := e.repo.GetByUsername("target")
	tid := rtItoa(target.ID)

	// admin changes role -> persisted and visible via GET
	w = e.do(e.adminTok, "PUT", "/api/v1/users/"+tid, `{"role_id":`+rtItoa(staff.ID)+`}`)
	require.Equal(t, 200, w.Code, w.Body.String())
	assert.Equal(t, staff.ID, e.roleOf(t, "target"), "role_id must be stored")
	w = e.do(e.adminTok, "GET", "/api/v1/users/"+tid, "")
	require.Equal(t, 200, w.Code, w.Body.String())
	var got models.User
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, staff.ID, got.RoleID)

	// non-admin user manager: may assign a non-admin role, may NOT assign admin
	lw := e.login("mgr", rtUserPass)
	require.Equal(t, 200, lw.Code, lw.Body.String())
	var lr struct {
		SessionToken string `json:"session_token"`
	}
	require.NoError(t, json.Unmarshal(lw.Body.Bytes(), &lr))
	w = e.do(lr.SessionToken, "PUT", "/api/v1/users/"+tid, `{"role_id":1}`)
	assert.Equal(t, 403, w.Code, w.Body.String())
	assert.Equal(t, staff.ID, e.roleOf(t, "target"), "forbidden admin assignment must not persist")
	w = e.do(lr.SessionToken, "PUT", "/api/v1/users/"+tid, `{"role_id":2}`)
	assert.Equal(t, 200, w.Code, w.Body.String())
	assert.Equal(t, 2, e.roleOf(t, "target"))

	// a user-manager updating THEMSELVES cannot escalate to admin either
	mgrUser, _ := e.repo.GetByUsername("mgr")
	w = e.do(lr.SessionToken, "PUT", "/api/v1/users/"+rtItoa(mgrUser.ID), `{"role_id":1}`)
	assert.Equal(t, 200, w.Code, w.Body.String())
	assert.Equal(t, mgrRole.ID, e.roleOf(t, "mgr"), "manager self role escalation ignored")

	// self-update ignores role_id / is_active
	tw := e.login("target", rtUserPass)
	require.Equal(t, 200, tw.Code)
	var tr struct {
		SessionToken string `json:"session_token"`
	}
	require.NoError(t, json.Unmarshal(tw.Body.Bytes(), &tr))
	w = e.do(tr.SessionToken, "PUT", "/api/v1/users/"+tid, `{"role_id":1,"is_active":false,"first_name":"Tgt"}`)
	assert.Equal(t, 200, w.Code, w.Body.String())
	after, _ := e.repo.GetByUsername("target")
	assert.Equal(t, 2, after.RoleID, "self role escalation ignored")
	assert.True(t, after.IsActive, "self deactivation ignored")
	require.NotNil(t, after.FirstName)
	assert.Equal(t, "Tgt", *after.FirstName)
}
