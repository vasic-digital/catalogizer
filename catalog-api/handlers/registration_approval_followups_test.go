package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"catalogizer/database"
	"catalogizer/models"
	"catalogizer/repository"
	"catalogizer/services"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Follow-ups F4, F5, F7 and nit N2 of the registration-approval review. TEST CLASS: integration
// on the REAL stack (production migrations on SQLite, real UserRepository, real AuthService with
// the production audit hook, real handlers); no test doubles.

type apprEnv struct {
	r        *gin.Engine
	db       *database.DB
	repo     *repository.UserRepository
	adminTok string
	adminID  int
}

func newApprEnv(t *testing.T) *apprEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)
	sqlDB, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	sqlDB.SetMaxOpenConns(1)
	db := database.WrapDB(sqlDB, database.DialectSQLite)
	require.NoError(t, db.RunMigrations(context.Background()))
	repo := repository.NewUserRepository(db)
	authSvc := services.NewAuthServiceWithAudit(repo, "test-jwt-secret-throwaway", db)
	adapter := &AuthServiceAdapter{Inner: authSvc}
	hash, salt, err := authSvc.HashPasswordForUser(rtAdminPass)
	require.NoError(t, err)
	adminID, err := repo.Create(&models.User{Username: "apadmin", Email: "apadmin@example.test",
		PasswordHash: hash, Salt: salt, RoleID: 1, IsActive: true})
	require.NoError(t, err)

	ah := NewAuthHandler(authSvc)
	uh := NewUserHandler(repo, adapter)
	adm := NewAdminHandler(adapter, repo, db, "test")
	r := gin.New()
	r.POST("/api/v1/auth/login", ah.LoginGin)
	r.POST("/api/v1/auth/register", func(c *gin.Context) { ah.RegisterGin(c, repo) })
	r.PUT("/api/v1/users/:id", WrapHTTPHandler(uh.UpdateUser))
	r.PUT("/api/v1/admin/users/:id", adm.UpdateUser)
	e := &apprEnv{r: r, db: db, repo: repo, adminID: adminID}
	w := e.do("", "POST", "/api/v1/auth/login", `{"username":"apadmin","password":"`+rtAdminPass+`"}`)
	require.Equal(t, 200, w.Code, w.Body.String())
	var res struct {
		SessionToken string `json:"session_token"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &res))
	e.adminTok = res.SessionToken
	return e
}

func (e *apprEnv) do(tok, method, url, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, url, bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	if tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	w := httptest.NewRecorder()
	e.r.ServeHTTP(w, req)
	return w
}

func (e *apprEnv) register(t *testing.T, name string) (int, *httptest.ResponseRecorder) {
	t.Helper()
	w := e.do("", "POST", "/api/v1/auth/register",
		`{"username":"`+name+`","email":"`+name+`@example.test","password":"`+rtUserPass+`","first_name":"A","last_name":"B"}`)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	u, err := e.repo.GetByUsername(name)
	require.NoError(t, err)
	return u.ID, w
}

func (e *apprEnv) audit(t *testing.T, event string) []struct {
	UserID  sql.NullInt64
	Details string
} {
	t.Helper()
	rows, err := e.db.Query(`SELECT user_id, COALESCE(details,'') FROM auth_audit_log WHERE event_type = ? ORDER BY id`, event)
	require.NoError(t, err)
	defer rows.Close()
	var out []struct {
		UserID  sql.NullInt64
		Details string
	}
	for rows.Next() {
		var r struct {
			UserID  sql.NullInt64
			Details string
		}
		require.NoError(t, rows.Scan(&r.UserID, &r.Details))
		out = append(out, r)
	}
	return out
}

func (e *apprEnv) adminStatus(t *testing.T, id int, status string) *httptest.ResponseRecorder {
	return e.do(e.adminTok, "PUT", "/api/v1/admin/users/"+strconv.Itoa(id), `{"status":"`+status+`"}`)
}

// F4: approving through PUT /users/:id {is_active:true} ends the pending state, so a later
// deactivation shows the user as inactive, never as pending again.
func TestApproval_F4_ActiveChangeClearsPendingMarker(t *testing.T) {
	t.Setenv("REGISTRATION_MODE", "approval")
	e := newApprEnv(t)
	id, _ := e.register(t, "f4user")
	u, _ := e.repo.GetByID(id)
	require.True(t, u.IsRegistrationPending())

	w := e.do(e.adminTok, "PUT", "/api/v1/users/"+strconv.Itoa(id), `{"is_active":true}`)
	require.Equal(t, 200, w.Code, w.Body.String())
	u, _ = e.repo.GetByID(id)
	assert.True(t, u.IsActive)
	assert.NotContains(t, u.Settings, "registration_status", "the pending marker must be gone once the user is activated")

	w = e.do(e.adminTok, "PUT", "/api/v1/users/"+strconv.Itoa(id), `{"is_active":false}`)
	require.Equal(t, 200, w.Code, w.Body.String())
	u, _ = e.repo.GetByID(id)
	assert.False(t, u.IsActive)
	assert.False(t, u.IsRegistrationPending(), "a deactivated approved user must not show as pending again")

	// a rejection by is_active:false on a still-pending user also ends the pending state
	id2, _ := e.register(t, "f4user2")
	w = e.do(e.adminTok, "PUT", "/api/v1/users/"+strconv.Itoa(id2), `{"is_active":false}`)
	require.Equal(t, 200, w.Code, w.Body.String())
	u2, _ := e.repo.GetByID(id2)
	assert.False(t, u2.IsRegistrationPending())

	// golden-FALSE: an update that does not change is_active leaves another user's marker alone
	id3, _ := e.register(t, "f4user3")
	w = e.do(e.adminTok, "PUT", "/api/v1/users/"+strconv.Itoa(id3), `{"display_name":"Renamed"}`)
	require.Equal(t, 200, w.Code, w.Body.String())
	u3, _ := e.repo.GetByID(id3)
	assert.True(t, u3.IsRegistrationPending(), "an unrelated update must not approve or reject")
}

// F5: the admin status field is a closed set; an invalid value is a 400 and changes nothing.
func TestApproval_F5_StatusIsValidated(t *testing.T) {
	t.Setenv("REGISTRATION_MODE", "approval")
	e := newApprEnv(t)
	id, _ := e.register(t, "f5user")
	for _, bad := range []string{"bogus", "", "ACTIVE", "pending", "active ", "'; drop table users;--"} {
		w := e.adminStatus(t, id, bad)
		assert.Equal(t, http.StatusBadRequest, w.Code, "status %q: %s", bad, w.Body.String())
		u, _ := e.repo.GetByID(id)
		assert.True(t, u.IsRegistrationPending(), "status %q must not clear the pending marker", bad)
		assert.False(t, u.IsActive, "status %q", bad)
	}
	// golden-FALSE: every valid value still works
	w := e.adminStatus(t, id, "active")
	assert.Equal(t, 200, w.Code, w.Body.String())
	w = e.adminStatus(t, id, "inactive")
	assert.Equal(t, 200, w.Code, w.Body.String())
	w = e.adminStatus(t, id, "suspended")
	assert.Equal(t, 200, w.Code, w.Body.String())
	// an update without a status field is not a status decision
	id2, _ := e.register(t, "f5user2")
	w = e.do(e.adminTok, "PUT", "/api/v1/admin/users/"+strconv.Itoa(id2), `{"email":"new@example.test"}`)
	assert.Equal(t, 200, w.Code, w.Body.String())
	u2, _ := e.repo.GetByID(id2)
	assert.True(t, u2.IsRegistrationPending())
}

// F7: registration, approval and rejection are recorded in auth_audit_log (the table the security
// audit report reads), without any login name, address or password.
func TestApproval_F7_AuditTrail(t *testing.T) {
	t.Setenv("REGISTRATION_MODE", "approval")
	e := newApprEnv(t)
	id, _ := e.register(t, "audituser")
	rej, _ := e.register(t, "rejecteduser")

	got := e.audit(t, "registration_created")
	require.Len(t, got, 2)
	assert.EqualValues(t, id, got[0].UserID.Int64)
	assert.Contains(t, got[0].Details, "status=pending")

	require.Equal(t, 200, e.adminStatus(t, id, "active").Code)
	require.Equal(t, 200, e.adminStatus(t, rej, "inactive").Code)

	appr := e.audit(t, "registration_approved")
	require.Len(t, appr, 1)
	assert.EqualValues(t, id, appr[0].UserID.Int64)
	assert.Contains(t, appr[0].Details, "actor="+strconv.Itoa(e.adminID))
	rejected := e.audit(t, "registration_rejected")
	require.Len(t, rejected, 1)
	assert.EqualValues(t, rej, rejected[0].UserID.Int64)

	// no PII or secrets in any audit row
	rows, err := e.db.Query(`SELECT COALESCE(details,'') FROM auth_audit_log`)
	require.NoError(t, err)
	defer rows.Close()
	for rows.Next() {
		var d string
		require.NoError(t, rows.Scan(&d))
		for _, forbidden := range []string{"audituser", "rejecteduser", "@example.test", rtUserPass} {
			assert.NotContains(t, d, forbidden)
		}
	}

	// golden-FALSE: a status change of a user that is not pending is not an approval
	require.Equal(t, 200, e.adminStatus(t, id, "inactive").Code)
	assert.Len(t, e.audit(t, "registration_approved"), 1)
	assert.Len(t, e.audit(t, "registration_rejected"), 1)
	assert.Len(t, e.audit(t, "admin_user_status_changed"), 1)
}

func TestApproval_F7_OpenModeRecordsActiveRegistration(t *testing.T) {
	t.Setenv("REGISTRATION_MODE", "open")
	e := newApprEnv(t)
	id, _ := e.register(t, "openaudit")
	got := e.audit(t, "registration_created")
	require.Len(t, got, 1)
	assert.EqualValues(t, id, got[0].UserID.Int64)
	assert.Contains(t, got[0].Details, "status=active")
}

// N2: the registration response does not echo the raw pending-marker settings.
func TestApproval_N2_ResponseDoesNotEchoSettingsMarker(t *testing.T) {
	t.Setenv("REGISTRATION_MODE", "approval")
	e := newApprEnv(t)
	_, w := e.register(t, "n2user")
	assert.NotContains(t, w.Body.String(), "registration_status\":\"pending\"}", "raw settings marker leaked")
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &m))
	assert.Equal(t, "pending", m["registration_status"], "the documented top-level field stays")
	if s, ok := m["settings"]; ok {
		assert.False(t, strings.Contains(strings.ToLower(toStr(s)), "registration_status"), "settings leaked: %v", s)
	}
}

func toStr(v interface{}) string { b, _ := json.Marshal(v); return string(b) }

// Review round 1 F4: approving or rejecting through PUT /users/:id {is_active} is audited exactly
// like the admin status route (registration_approved / registration_rejected, actor = the admin),
// and a change of an already decided user is not.
func TestApproval_F4_UserUpdateRecordsAuditEvent(t *testing.T) {
	t.Setenv("REGISTRATION_MODE", "approval")
	e := newApprEnv(t)
	okID, _ := e.register(t, "uaudit1")
	noID, _ := e.register(t, "uaudit2")

	require.Equal(t, 200, e.do(e.adminTok, "PUT", "/api/v1/users/"+strconv.Itoa(okID), `{"is_active":true}`).Code)
	require.Equal(t, 200, e.do(e.adminTok, "PUT", "/api/v1/users/"+strconv.Itoa(noID), `{"is_active":false}`).Code)

	appr := e.audit(t, "registration_approved")
	require.Len(t, appr, 1)
	assert.EqualValues(t, okID, appr[0].UserID.Int64)
	assert.Contains(t, appr[0].Details, "actor="+strconv.Itoa(e.adminID))
	rej := e.audit(t, "registration_rejected")
	require.Len(t, rej, 1)
	assert.EqualValues(t, noID, rej[0].UserID.Int64)

	// golden-FALSE: deactivating an already approved user is not a registration decision
	require.Equal(t, 200, e.do(e.adminTok, "PUT", "/api/v1/users/"+strconv.Itoa(okID), `{"is_active":false}`).Code)
	assert.Len(t, e.audit(t, "registration_approved"), 1)
	assert.Len(t, e.audit(t, "registration_rejected"), 1)
}
