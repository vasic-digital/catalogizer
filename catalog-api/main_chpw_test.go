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

type chpwRig struct {
	db            *database.DB
	call          func(tok string, right bool) int
	attacker, own string
	reached       *int
	close         func()
}

func newChpwRig(t *testing.T) *chpwRig {
	gin.SetMode(gin.TestMode)
	sqlDB, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
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
	return &chpwRig{db: db, call: call, attacker: attacker, own: owner, reached: &reached, close: func() { sqlDB.Close() }}
}

// Production route registration + REAL RequireAuth + REAL session check + real
// session revocation on a migrated sqlite DB (CAT-CHPW-RL-1). The handler is a
// stub that answers 401 (wrong password) unless X-Right is set.
func TestRegisterChangePasswordRoute_LockRevokesAttackerSessionOwnerSurvives(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rg := newChpwRig(t)
	defer rg.close()
	db, call, attacker, owner := rg.db, rg.call, rg.attacker, rg.own
	reached := rg.reached
	for i := 0; i < 5; i++ {
		require.Equal(t, http.StatusUnauthorized, call(attacker, false))
	}
	reachedAfterFive := *reached
	// The 5th failure exhausted the session's budget: the session is revoked,
	// so RequireAuth rejects the attacker's token before the limiter or handler.
	require.Equal(t, http.StatusUnauthorized, call(attacker, true), "attacker session revoked, even with the right password")
	require.Equal(t, reachedAfterFive, *reached, "handler not reached by the revoked session")
	var active bool
	require.NoError(t, db.QueryRow(`SELECT is_active FROM user_sessions WHERE id = 1`).Scan(&active))
	require.False(t, active)
	require.Equal(t, http.StatusOK, call(owner, true), "owner's own session unaffected")
}

// Failures spread over two sessions of one user (3+2) exhaust neither session's
// budget: neither session may be revoked (end to end, real RequireAuth + DB).
func TestRegisterChangePasswordRoute_SpreadFailuresRevokeNobody(t *testing.T) {
	rg := newChpwRig(t)
	defer rg.close()
	for i := 0; i < 3; i++ {
		require.Equal(t, http.StatusUnauthorized, rg.call(rg.attacker, false))
	}
	for i := 0; i < 2; i++ {
		require.Equal(t, http.StatusUnauthorized, rg.call(rg.own, false))
	}
	var n int
	require.NoError(t, rg.db.QueryRow(`SELECT COUNT(*) FROM user_sessions WHERE is_active = 1`).Scan(&n))
	require.Equal(t, 2, n, "no session revoked")
	require.Equal(t, http.StatusOK, rg.call(rg.own, true))
}

// main.go must register /change-password only through registerChangePasswordRoute:
// no registration call (POST/Handle/Any/...) whose arguments carry the change-password
// path, as a literal or through a const/var, and exactly one call of the helper.
func TestMainGoRegistersChangePasswordOnlyViaHelper(t *testing.T) {
	f, err := parser.ParseFile(token.NewFileSet(), "main.go", nil, 0)
	require.NoError(t, err)
	strs := map[string]string{} // package-level and local const/var string bindings
	ast.Inspect(f, func(n ast.Node) bool {
		if vs, ok := n.(*ast.ValueSpec); ok {
			for i, name := range vs.Names {
				if i < len(vs.Values) {
					if lit, ok := vs.Values[i].(*ast.BasicLit); ok && lit.Kind == token.STRING {
						strs[name.Name], _ = strconv.Unquote(lit.Value)
					}
				}
			}
		}
		if as, ok := n.(*ast.AssignStmt); ok && len(as.Lhs) == len(as.Rhs) {
			for i, l := range as.Lhs {
				if id, ok := l.(*ast.Ident); ok {
					if lit, ok := as.Rhs[i].(*ast.BasicLit); ok && lit.Kind == token.STRING {
						strs[id.Name], _ = strconv.Unquote(lit.Value)
					}
				}
			}
		}
		return true
	})
	isPath := func(e ast.Expr) bool {
		var s string
		switch v := e.(type) {
		case *ast.BasicLit:
			if v.Kind != token.STRING {
				return false
			}
			s, _ = strconv.Unquote(v.Value)
		case *ast.Ident:
			s = strs[v.Name]
		}
		return s == "/change-password" || s == "/api/v1/auth/change-password"
	}
	helperCalls, direct := 0, 0
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
		case *ast.SelectorExpr: // .POST(path,...), .Handle("POST", path,...), .Any(path,...)
			for _, a := range call.Args {
				if isPath(a) {
					direct++
				}
			}
		}
		return true
	})
	require.Equal(t, 1, helperCalls, "main.go must call registerChangePasswordRoute exactly once")
	require.Zero(t, direct, "no route registration carrying the change-password path outside the helper")
}
