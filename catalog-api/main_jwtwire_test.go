package main

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"catalogizer/database"
	"catalogizer/repository"

	"github.com/gin-gonic/gin"
	_ "github.com/mutecomm/go-sqlcipher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// B1: production wiring of the JWT middleware. The helper is what main.go
// calls; the first test proves it installs the live admin check on a real
// migrated DB, the second proves main.go really goes through the helper.
func TestNewAuthMiddleware_InstallsLiveAdminCheck(t *testing.T) {
	gin.SetMode(gin.TestMode)
	sqlDB, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	defer sqlDB.Close()
	sqlDB.SetMaxOpenConns(1)
	db := database.WrapDB(sqlDB, database.DialectSQLite)
	require.NoError(t, db.RunMigrations(context.Background()))
	repo := repository.NewUserRepository(db)

	_, err = db.Exec(`INSERT INTO users (id, username, email, password_hash, salt, role_id, is_active) VALUES (7, 'dem', 'd@x.test', 'h', 's', 2, 1)`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO user_sessions (id, user_id, session_token, refresh_token, is_active, expires_at) VALUES (1, 7, 't', 'r', 1, datetime('now','+1 hour'))`)
	require.NoError(t, err)

	mw := newAuthMiddleware("wire-test-secret", db, repo)
	// Token still says role_id 1 (admin) although the DB says role 2: the gate must follow the DB.
	tok := mintTestToken(t, mw, "7", 1, "1")
	r := gin.New()
	r.GET("/admin/x", mw.RequireAuth(), mw.RequireAdmin(), func(c *gin.Context) { c.Status(http.StatusOK) })
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/x", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code, "stale admin claim must not pass the production gate")

	_, err = db.Exec(`UPDATE users SET role_id = 1 WHERE id = 7`)
	require.NoError(t, err)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code, "DB admin passes")
}

func TestMainGoWiresJWTMiddlewareThroughHelper(t *testing.T) {
	src, err := os.ReadFile("main.go")
	require.NoError(t, err)
	s := string(src)
	assert.Contains(t, s, "newAuthMiddleware(", "main.go must build the JWT middleware through newAuthMiddleware")
	assert.False(t, strings.Contains(s, "root_middleware.NewJWTMiddleware("), "main.go must not construct a bare JWT middleware (it would skip the live admin/session checks)")
}
