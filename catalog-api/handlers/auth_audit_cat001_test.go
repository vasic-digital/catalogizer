package handlers

// CAT-001 review F3: the security audit report reads auth_audit_log, so the PRODUCTION login must write it. Drives the real login
// route (AuthHandler.LoginGin) with the production DB schema and the production AuthService constructor, then reads the report.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"catalogizer/models"
	"catalogizer/repository"
	"catalogizer/services"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCAT001_F3_ProductionLoginWritesAuthAuditLogAndTheReportCountsIt(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := openProductionFileDB(t)
	repo := repository.NewUserRepository(db)
	auth := services.NewAuthServiceWithAudit(repo, "f3-secret-0123456789", db)
	hash, salt, err := auth.HashPasswordForUser("Corr3ct-Horse-Battery")
	require.NoError(t, err)
	_, err = repo.Create(&models.User{Username: "alice", Email: "a@example.test", PasswordHash: hash, Salt: salt, RoleID: 2, IsActive: true})
	require.NoError(t, err)

	r := gin.New()
	r.POST("/auth/login", NewAuthHandler(auth).LoginGin)
	login := func(user, pass string) int {
		body, _ := json.Marshal(map[string]string{"username": user, "password": pass})
		req := httptest.NewRequest("POST", "/auth/login", strings.NewReader(string(body)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "f3-test")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w.Code
	}
	assert.Equal(t, http.StatusUnauthorized, login("alice", "wrong-1"))
	assert.Equal(t, http.StatusUnauthorized, login("alice", "wrong-2"))
	assert.Equal(t, http.StatusUnauthorized, login("nobody", "whatever"))
	assert.Equal(t, http.StatusOK, login("alice", "Corr3ct-Horse-Battery"))

	var failed, ok int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM auth_audit_log WHERE event_type LIKE 'failed_login%'`).Scan(&failed))
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM auth_audit_log WHERE event_type = 'login_success'`).Scan(&ok))
	assert.Equal(t, 3, failed)
	assert.Equal(t, 1, ok)
	var leaked int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM auth_audit_log WHERE details LIKE '%wrong-%' OR details LIKE '%Horse%' OR details LIKE '%nobody%' OR details LIKE '%alice%'`).Scan(&leaked))
	assert.Equal(t, 0, leaked, "no password or login name is written to the audit trail")

	rep := services.NewReportingService(nil, repo, db)
	out, err := rep.GenerateReport("security_audit", "json", map[string]interface{}{
		"start_date": time.Now().Add(-24 * time.Hour).Format("2006-01-02"), "end_date": time.Now().Add(48 * time.Hour).Format("2006-01-02")})
	require.NoError(t, err)
	var parsed models.SecurityAuditReport
	require.NoError(t, json.Unmarshal(out.Content, &parsed))
	assert.Equal(t, 3, parsed.FailedLoginAttempts)
	assert.Equal(t, 1, parsed.SuccessfulLogins)
}
