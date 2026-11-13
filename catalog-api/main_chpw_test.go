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
	engine        *gin.Engine
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
	return &chpwRig{db: db, call: call, attacker: attacker, own: owner, reached: &reached, close: func() { sqlDB.Close() }, engine: r}
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

// foldString evaluates e to a string when it is a literal, a concatenation of
// foldable parts, a parenthesised foldable, or an identifier bound to one.
func foldString(e ast.Expr, binds map[string]string) (string, bool) {
	switch v := e.(type) {
	case *ast.BasicLit:
		if v.Kind == token.STRING {
			s, err := strconv.Unquote(v.Value)
			return s, err == nil
		}
	case *ast.ParenExpr:
		return foldString(v.X, binds)
	case *ast.Ident:
		s, ok := binds[v.Name]
		return s, ok
	case *ast.BinaryExpr:
		if v.Op == token.ADD {
			l, ok1 := foldString(v.X, binds)
			r, ok2 := foldString(v.Y, binds)
			return l + r, ok1 && ok2
		}
	}
	return "", false
}

// main.go must register /change-password only through registerChangePasswordRoute:
// NO expression in main.go (literal, "a"+"b" concatenation, const/var, composite
// literal or struct field value, call argument) may evaluate to the change-password
// path, there is exactly one helper call, and its first argument is authGroup,
// which is the /api/v1/auth group.
func TestMainGoRegistersChangePasswordOnlyViaHelper(t *testing.T) {
	f, err := parser.ParseFile(token.NewFileSet(), "main.go", nil, 0)
	require.NoError(t, err)
	binds := map[string]string{}
	for pass := 0; pass < 3; pass++ { // let bindings built from other bindings settle
		ast.Inspect(f, func(n ast.Node) bool {
			switch v := n.(type) {
			case *ast.ValueSpec:
				for i, name := range v.Names {
					if i < len(v.Values) {
						if s, ok := foldString(v.Values[i], binds); ok {
							binds[name.Name] = s
						}
					}
				}
			case *ast.AssignStmt:
				if len(v.Lhs) == len(v.Rhs) {
					for i, l := range v.Lhs {
						if id, ok := l.(*ast.Ident); ok {
							if s, ok := foldString(v.Rhs[i], binds); ok {
								binds[id.Name] = s
							}
						}
					}
				}
			}
			return true
		})
	}
	isPath := func(s string) bool { return s == "/change-password" || s == "/api/v1/auth/change-password" }
	var helperCalls []*ast.CallExpr
	direct := 0
	var authGroupInit ast.Expr
	ast.Inspect(f, func(n ast.Node) bool {
		switch v := n.(type) {
		case ast.Expr:
			if s, ok := foldString(v, binds); ok && isPath(s) {
				if _, isIdent := v.(*ast.Ident); !isIdent { // count each evaluation site once
					direct++
				}
			}
		case *ast.AssignStmt:
			if len(v.Lhs) == 1 && len(v.Rhs) == 1 {
				if id, ok := v.Lhs[0].(*ast.Ident); ok && id.Name == "authGroup" {
					authGroupInit = v.Rhs[0]
				}
			}
		}
		if call, ok := n.(*ast.CallExpr); ok {
			if id, ok := call.Fun.(*ast.Ident); ok && id.Name == "registerChangePasswordRoute" {
				helperCalls = append(helperCalls, call)
			}
		}
		return true
	})
	require.Len(t, helperCalls, 1, "main.go must call registerChangePasswordRoute exactly once")
	require.Zero(t, direct, "no expression in main.go may evaluate to the change-password path")
	first, ok := helperCalls[0].Args[0].(*ast.Ident)
	require.True(t, ok && first.Name == "authGroup", "helper's first argument must be authGroup")
	init, ok := authGroupInit.(*ast.CallExpr)
	require.True(t, ok, "authGroup must be initialised by a call")
	sel, ok := init.Fun.(*ast.SelectorExpr)
	require.True(t, ok && sel.Sel.Name == "Group")
	grp, ok := foldString(init.Args[0], binds)
	require.True(t, ok && grp == "/api/v1/auth", "authGroup must be the /api/v1/auth group")
}

// Runtime: walk the real gin engine built through the helper: exactly one POST
// /api/v1/auth/change-password route, and that route sits behind the auth
// middleware (no token: 401, handler not reached) and the limiter (revoked
// session after the budget, covered end to end above).
func TestRegisterChangePasswordRoute_EngineHasOneRouteBehindAuth(t *testing.T) {
	rg := newChpwRig(t)
	defer rg.close()
	require.Equal(t, http.StatusUnauthorized, rg.call("", false))
	require.Zero(t, *rg.reached, "no token: handler never reached")
	n := 0
	for _, r := range rg.engine.Routes() {
		if r.Method == http.MethodPost && r.Path == "/api/v1/auth/change-password" {
			n++
		}
	}
	require.Equal(t, 1, n)
}
