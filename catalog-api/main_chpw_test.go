package main

import (
	"context"
	"database/sql"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"catalogizer/database"
	"catalogizer/repository"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// Production route registration + REAL RequireAuth + REAL session check + real
// session revocation on a migrated sqlite DB (CAT-CHPW-RL-1). The handler is a
// stub that answers 401 (wrong password) unless X-Right is set.
func TestRegisterChangePasswordRoute_LockRevokesAttackerSessionOwnerSurvives(t *testing.T) {
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
	for _, id := range []int{1, 2} { // 1 = attacker's stolen session, 2 = owner's own
		_, err = db.Exec(`INSERT INTO user_sessions (id, user_id, session_token, refresh_token, is_active, expires_at) VALUES (?, 7, ?, ?, 1, datetime('now','+1 hour'))`,
			id, "t"+strconv.Itoa(id), "r"+strconv.Itoa(id))
		require.NoError(t, err)
	}
	mw := newAuthMiddleware("wire-test-secret", db, repo)
	attacker := mintTestToken(t, mw, "7", 2, "1")
	owner := mintTestToken(t, mw, "7", 2, "2")

	r := gin.New()
	g := r.Group("/api/v1/auth")
	reached := 0
	registerChangePasswordRoute(g, func(c *gin.Context) { c.Next() }, mw.RequireAuth(), func(c *gin.Context) {
		reached++
		if c.GetHeader("X-Right") != "" {
			c.Status(http.StatusOK)
			return
		}
		c.Status(http.StatusUnauthorized)
	}, repo, zap.NewNop())

	call := func(tok string, right bool) int {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/change-password", nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		if right {
			req.Header.Set("X-Right", "1")
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w.Code
	}
	for i := 0; i < 5; i++ {
		require.Equal(t, http.StatusUnauthorized, call(attacker, false))
	}
	reachedAfterFive := reached
	// The 5th failure exhausted the session's budget: the session is revoked,
	// so RequireAuth rejects the attacker's token before the limiter or handler.
	require.Equal(t, http.StatusUnauthorized, call(attacker, true), "attacker session revoked, even with the right password")
	require.Equal(t, reachedAfterFive, reached, "handler not reached by the revoked session")
	var active bool
	require.NoError(t, db.QueryRow(`SELECT is_active FROM user_sessions WHERE id = 1`).Scan(&active))
	require.False(t, active)
	require.Equal(t, http.StatusOK, call(owner, true), "owner's own session unaffected")
}

// main.go must register /change-password only through registerChangePasswordRoute:
// no direct `.POST("/change-password", ...)` call and at least one call of the helper.
func TestMainGoRegistersChangePasswordOnlyViaHelper(t *testing.T) {
	f, err := parser.ParseFile(token.NewFileSet(), "main.go", nil, 0)
	require.NoError(t, err)
	helperCalls, directPosts := 0, 0
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch fn := call.Fun.(type) {
		case *ast.Ident:
			if fn.Name == "registerChangePasswordRoute" {
				helperCalls++
			}
		case *ast.SelectorExpr:
			if len(call.Args) > 0 {
				if lit, ok := call.Args[0].(*ast.BasicLit); ok && lit.Kind == token.STRING {
					if s, _ := strconv.Unquote(lit.Value); s == "/change-password" || s == "/api/v1/auth/change-password" {
						directPosts++
					}
				}
			}
		}
		return true
	})
	require.Equal(t, 1, helperCalls, "main.go must call registerChangePasswordRoute exactly once")
	require.Zero(t, directPosts, "no route registration with the literal change-password path outside the helper")
}
