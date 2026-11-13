package main

import (
	"context"
	"database/sql"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
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
	noop := func(c *gin.Context) { c.Status(http.StatusOK) }
	pass := func(c *gin.Context) { c.Next() }
	registerAuthRoutes(g, authRoutesDeps{
		LoginRate: pass, AuthRate: pass, DefaultRate: pass, RequireAuth: mw.RequireAuth,
		Login: noop, Register: noop, Refresh: noop, Logout: noop, Me: noop, Status: noop, Permissions: noop, InitStatus: noop,
		ChangePassword: func(c *gin.Context) {
			reached++
			if c.GetHeader("X-Right") != "" {
				c.Status(http.StatusOK)
				return
			}
			c.Status(http.StatusUnauthorized)
		},
		Sessions: repo, Logger: zap.NewNop(),
	})

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

// The auth route table is pinned: any added/removed/re-methoded route in
// registerAuthRoutes fails here and must be reviewed (only change-password
// accepts a current password behind a JWT; login's own limiter is the 30 rpm
// per-IP one plus the service lockout). Runs on the engine built through the
// SAME registerAuthRoutes main() calls, with the real RequireAuth.
func TestRegisterAuthRoutes_RouteTablePinnedAndProtectedRoutesNeedAuth(t *testing.T) {
	rg := newChpwRig(t)
	defer rg.close()
	got := map[string]bool{}
	for _, r := range rg.engine.Routes() {
		got[r.Method+" "+r.Path] = true
	}
	want := []string{"POST /api/v1/auth/login", "POST /api/v1/auth/register", "POST /api/v1/auth/refresh",
		"POST /api/v1/auth/logout", "GET /api/v1/auth/me", "GET /api/v1/auth/status", "GET /api/v1/auth/permissions",
		"GET /api/v1/auth/profile", "GET /api/v1/auth/init-status", "POST /api/v1/auth/change-password"}
	require.Len(t, got, len(want), "route table changed: review every new route for credential verification")
	for _, w := range want {
		require.True(t, got[w], w)
	}
	for _, pr := range []struct{ m, p string }{{"GET", "me"}, {"GET", "permissions"}, {"GET", "profile"}, {"POST", "change-password"}} {
		w := httptest.NewRecorder()
		rg.engine.ServeHTTP(w, httptest.NewRequest(pr.m, "/api/v1/auth/"+pr.p, nil))
		require.Equal(t, http.StatusUnauthorized, w.Code, pr.p+" without a token")
	}
	require.Zero(t, *rg.reached, "change-password handler never reached without a token")
	// with a valid token the request reaches the handler THROUGH the limiter
	// (the limiter revokes the session after 5 failures: see the lock test above).
	require.Equal(t, http.StatusUnauthorized, rg.call(rg.attacker, false))
	require.Equal(t, 1, *rg.reached)
}

// Pins WHICH middleware runs, in order, on every route of registerAuthRoutes
// (CAT-CHPW-RL-1-FU F1). Every dependency is a distinguishable marker that
// appends its name to a per-request trace; the handlers append "h:<name>".
// change-password's own failed-attempt limiter is internal to the helper (it
// sits between "auth" and the handler and is pinned by the lock tests); the
// trace here pins the externally supplied limiter (DefaultRate) and auth.
func TestRegisterAuthRoutes_MiddlewareChainPerRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var trace []string
	mark := func(name string) gin.HandlerFunc {
		return func(c *gin.Context) { trace = append(trace, name); c.Next() }
	}
	h := func(name string) gin.HandlerFunc {
		return func(c *gin.Context) { trace = append(trace, "h:"+name); c.Status(http.StatusOK) }
	}
	auth := func() gin.HandlerFunc {
		return func(c *gin.Context) {
			trace = append(trace, "auth")
			c.Set("user_id", 7)
			c.Set("session_id", "1")
			c.Next()
		}
	}
	r := gin.New()
	registerAuthRoutes(r.Group("/api/v1/auth"), authRoutesDeps{
		LoginRate: mark("LoginRate"), AuthRate: mark("AuthRate"), DefaultRate: mark("DefaultRate"), RequireAuth: auth,
		Login: h("Login"), Register: h("Register"), Refresh: h("Refresh"), Logout: h("Logout"), Me: h("Me"),
		Status: h("Status"), Permissions: h("Permissions"), InitStatus: h("InitStatus"), ChangePassword: h("ChangePassword"),
		Sessions: nopSessions{}, Logger: zap.NewNop(),
	})
	want := []struct {
		method, path string
		chain        []string
	}{
		{"POST", "login", []string{"LoginRate", "h:Login"}},
		{"POST", "register", []string{"LoginRate", "h:Register"}},
		{"POST", "refresh", []string{"AuthRate", "h:Refresh"}},
		{"POST", "logout", []string{"AuthRate", "h:Logout"}},
		{"GET", "me", []string{"AuthRate", "auth", "h:Me"}},
		{"GET", "status", []string{"AuthRate", "h:Status"}},
		{"GET", "permissions", []string{"AuthRate", "auth", "h:Permissions"}},
		{"GET", "profile", []string{"AuthRate", "auth", "h:Me"}},
		{"GET", "init-status", []string{"DefaultRate", "h:InitStatus"}},
		{"POST", "change-password", []string{"DefaultRate", "auth", "h:ChangePassword"}},
	}
	require.Len(t, r.Routes(), len(want), "route count changed: extend the chain table")
	for _, w := range want {
		trace = nil
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(w.method, "/api/v1/auth/"+w.path, nil))
		require.Equal(t, http.StatusOK, rec.Code, w.path)
		require.Equal(t, w.chain, trace, w.method+" "+w.path)
	}
}

type nopSessions struct{}

func (nopSessions) DeactivateSession(int) error { return nil }

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

// Source guard (cheap, static; the route-table test above is the runtime proof
// for the auth group). Across ALL non-test files of package main:
//   - no expression evaluates to the change-password path except the one inside
//     main_chpw.go's RegisterChangePassword call (catches another file or a
//     second registration with a literal/concatenated/const path);
//   - a selector `.ChangePassword` occurs only as mediaQueryHandler.ChangePassword
//     once (the ChangePassword: field of authRoutesDeps in main.go) and
//     d.ChangePassword once (4th argument of registerChangePasswordRoute);
//   - registerChangePasswordRoute is called once, with g, d.RequireAuth() as 3rd
//     and d.ChangePassword as 4th argument; registerAuthRoutes is called once,
//     with the /api/v1/auth group and RequireAuth: jwtMiddleware.RequireAuth.
//
// NOT covered statically: a route built from a runtime-computed path that does
// not reference the handler by name (none can reach the handler, see above).
func TestMainPackageChangePasswordWiringSource(t *testing.T) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(fi os.FileInfo) bool { return !strings.HasSuffix(fi.Name(), "_test.go") }, 0)
	require.NoError(t, err)
	pkg := pkgs["main"]
	require.NotNil(t, pkg)
	binds := map[string]string{}
	for pass := 0; pass < 3; pass++ {
		for _, f := range pkg.Files {
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
	}
	isPath := func(s string) bool { return s == "/change-password" || s == "/api/v1/auth/change-password" }
	var pathSites []string
	selMQ, selD := 0, 0
	var helperCalls, authCalls []*ast.CallExpr
	var mqSelParentOK bool
	for name, f := range pkg.Files {
		ast.Inspect(f, func(n ast.Node) bool {
			if e, ok := n.(ast.Expr); ok {
				if s, ok := foldString(e, binds); ok && isPath(s) {
					if _, isIdent := e.(*ast.Ident); !isIdent {
						pathSites = append(pathSites, name)
					}
				}
			}
			if kv, ok := n.(*ast.KeyValueExpr); ok {
				if sel, ok := kv.Value.(*ast.SelectorExpr); ok && sel.Sel.Name == "ChangePassword" {
					if k, ok := kv.Key.(*ast.Ident); ok && k.Name == "ChangePassword" {
						if x, ok := sel.X.(*ast.Ident); ok && x.Name == "mediaQueryHandler" {
							mqSelParentOK = true
						}
					}
				}
			}
			if sel, ok := n.(*ast.SelectorExpr); ok && sel.Sel.Name == "ChangePassword" {
				if x, ok := sel.X.(*ast.Ident); ok {
					switch x.Name {
					case "mediaQueryHandler":
						selMQ++
					case "d":
						selD++
					default:
						selMQ += 100 // any other receiver of a ChangePassword selector in package main
					}
				} else {
					selMQ += 100
				}
			}
			if call, ok := n.(*ast.CallExpr); ok {
				if id, ok := call.Fun.(*ast.Ident); ok {
					switch id.Name {
					case "registerChangePasswordRoute":
						helperCalls = append(helperCalls, call)
					case "registerAuthRoutes":
						authCalls = append(authCalls, call)
					}
				}
			}
			return true
		})
	}
	require.Equal(t, []string{"main_chpw.go"}, pathSites, "exactly one evaluation of the change-password path in package main")
	require.Equal(t, 1, selMQ, "mediaQueryHandler.ChangePassword referenced exactly once (and no other ChangePassword selector receiver)")
	require.True(t, mqSelParentOK, "...as the ChangePassword: field of authRoutesDeps")
	require.Equal(t, 1, selD)
	require.Len(t, helperCalls, 1)
	args := helperCalls[0].Args
	require.Len(t, args, 6)
	require.Equal(t, "g", args[0].(*ast.Ident).Name)
	require.True(t, isSel(args[1], "d", "DefaultRate"), "2nd argument (rate limiter) must be d.DefaultRate")
	rq, ok := args[2].(*ast.CallExpr)
	require.True(t, ok, "3rd argument must be the d.RequireAuth() call")
	rqs, ok := rq.Fun.(*ast.SelectorExpr)
	require.True(t, ok && rqs.Sel.Name == "RequireAuth" && rqs.X.(*ast.Ident).Name == "d")
	cp, ok := args[3].(*ast.SelectorExpr)
	require.True(t, ok && cp.Sel.Name == "ChangePassword" && cp.X.(*ast.Ident).Name == "d")

	require.Len(t, authCalls, 1, "registerAuthRoutes called exactly once")
	lit, ok := authCalls[0].Args[1].(*ast.CompositeLit)
	require.True(t, ok)
	found := false
	for _, el := range lit.Elts {
		kv := el.(*ast.KeyValueExpr)
		if kv.Key.(*ast.Ident).Name == "RequireAuth" {
			sel, ok := kv.Value.(*ast.SelectorExpr)
			require.True(t, ok && sel.Sel.Name == "RequireAuth" && sel.X.(*ast.Ident).Name == "jwtMiddleware", "RequireAuth must be jwtMiddleware.RequireAuth")
			found = true
		}
	}
	require.True(t, found)
	require.Equal(t, "authGroup", authCalls[0].Args[0].(*ast.Ident).Name)
}

// isSel reports whether e is the selector expression x.sel.
func isSel(e ast.Expr, x, sel string) bool {
	se, ok := e.(*ast.SelectorExpr)
	if !ok || se.Sel.Name != sel {
		return false
	}
	id, ok := se.X.(*ast.Ident)
	return ok && id.Name == x
}

// isCallSel reports whether e is a call x.sel(args...) and returns the call.
func isCallSel(e ast.Expr, x, sel string) (*ast.CallExpr, bool) {
	c, ok := e.(*ast.CallExpr)
	if !ok || !isSel(c.Fun, x, sel) {
		return nil, false
	}
	return c, true
}

func intLit(e ast.Expr) (int, bool) {
	bl, ok := e.(*ast.BasicLit)
	if !ok || bl.Kind != token.INT {
		return 0, false
	}
	n, err := strconv.Atoi(bl.Value)
	return n, err == nil
}

// Pins, in main.go, (1) which REAL limiter feeds each limiter-bearing field of
// authRoutesDeps and how each limiter is constructed, and (2) the exact shape of
// the auth group (CAT-CHPW-RL-1-FU). The limiter-bearing fields of authRoutesDeps
// are LoginRate, AuthRate and DefaultRate (the only fields of that struct typed as
// middleware that is not RequireAuth; the test fails if a new *Rate* field appears
// without a pinned mapping). Pinned construction (tolerant of formatting, not of
// values): loginRateLimiter := authMiddleware.RateLimitByUser(30, "1m"),
// authRateLimiter ... (600, "1m"), defaultRateLimiter ... (2000, "1m"); each
// ident is assigned exactly once, except that authRateLimiter and
// defaultRateLimiter are reassigned exactly once, together, from
// root_middleware.NewRedisTierLimiters(redisClient, authRateLimiter,
// defaultRateLimiter); loginRateLimiter is never reassigned. The auth group is
// `authGroup := router.Group("/api/v1/auth")` with that single argument, and no
// middleware is added to authGroup afterwards (no authGroup.Use / .Group / any
// other method call on it besides being passed to registerAuthRoutes).
// Unit-class static guard: it reads source text, it does not run main().
func TestMainGoAuthLimiterAndGroupWiringSource(t *testing.T) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "main.go", nil, 0)
	require.NoError(t, err)

	// --- struct: the only *Rate* fields are the three pinned ones.
	pf, err := parser.ParseFile(fset, "main_chpw.go", nil, 0)
	require.NoError(t, err)
	var rateFields []string
	ast.Inspect(pf, func(n ast.Node) bool {
		ts, ok := n.(*ast.TypeSpec)
		if !ok || ts.Name.Name != "authRoutesDeps" {
			return true
		}
		st, ok := ts.Type.(*ast.StructType)
		if !ok {
			t.Fatalf("authRoutesDeps is not a struct")
		}
		for _, fl := range st.Fields.List {
			for _, nm := range fl.Names {
				if strings.Contains(nm.Name, "Rate") || strings.Contains(nm.Name, "Limit") {
					rateFields = append(rateFields, nm.Name)
				}
			}
		}
		return false
	})
	require.ElementsMatch(t, []string{"LoginRate", "AuthRate", "DefaultRate"}, rateFields,
		"authRoutesDeps limiter-bearing fields changed: pin the new field's mapping here")

	// --- composite literal mapping in main.go.
	want := map[string]string{"LoginRate": "loginRateLimiter", "AuthRate": "authRateLimiter", "DefaultRate": "defaultRateLimiter"}
	var lits, groupAssigns int
	var groupCall *ast.CallExpr
	ast.Inspect(f, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.CompositeLit:
			id, ok := v.Type.(*ast.Ident)
			if !ok || id.Name != "authRoutesDeps" {
				return true
			}
			lits++
			got := map[string]string{}
			for _, el := range v.Elts {
				kv, ok := el.(*ast.KeyValueExpr)
				if !ok {
					t.Fatalf("authRoutesDeps literal must use keyed fields")
				}
				k, ok := kv.Key.(*ast.Ident)
				if !ok {
					t.Fatalf("authRoutesDeps literal key is not an identifier")
				}
				if _, isRate := want[k.Name]; isRate {
					vi, ok := kv.Value.(*ast.Ident)
					if !ok {
						t.Fatalf("%s must be a plain identifier, got %T", k.Name, kv.Value)
					}
					got[k.Name] = vi.Name
				}
			}
			require.Equal(t, want, got, "authRoutesDeps limiter fields must map to their exact limiter idents")
		case *ast.AssignStmt:
			if len(v.Lhs) == 1 && len(v.Rhs) == 1 {
				if id, ok := v.Lhs[0].(*ast.Ident); ok && id.Name == "authGroup" {
					groupAssigns++
					c, ok := v.Rhs[0].(*ast.CallExpr)
					if !ok {
						t.Fatalf("authGroup must be assigned from a call")
					}
					groupCall = c
				}
			}
		}
		return true
	})
	require.Equal(t, 1, lits, "exactly one authRoutesDeps literal in main.go")

	// --- limiter construction: classify every assignment to the three idents.
	type asg struct {
		define bool
		lhs    []string
		rhs    []ast.Expr
	}
	byName := map[string][]asg{}
	ast.Inspect(f, func(n ast.Node) bool {
		as, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}
		var names []string
		touched := false
		for _, l := range as.Lhs {
			nm := ""
			if id, ok := l.(*ast.Ident); ok {
				nm = id.Name
			}
			names = append(names, nm)
			if nm == "loginRateLimiter" || nm == "authRateLimiter" || nm == "defaultRateLimiter" {
				touched = true
			}
		}
		if touched {
			a := asg{define: as.Tok == token.DEFINE, lhs: names, rhs: as.Rhs}
			for _, nm := range names {
				byName[nm] = append(byName[nm], a)
			}
		}
		return true
	})
	for nm, n := range map[string]int{"loginRateLimiter": 30, "authRateLimiter": 600, "defaultRateLimiter": 2000} {
		as := byName[nm]
		require.NotEmpty(t, as, nm+" not assigned")
		first := as[0]
		require.True(t, first.define && len(first.lhs) == 1 && len(first.rhs) == 1, nm+" must be defined by a single `:=`")
		c, ok := isCallSel(first.rhs[0], "authMiddleware", "RateLimitByUser")
		require.True(t, ok && len(c.Args) == 2, nm+" must be authMiddleware.RateLimitByUser(N, \"1m\")")
		got, ok := intLit(c.Args[0])
		require.True(t, ok && got == n, "%s limit must be %d", nm, n)
		per, ok := foldString(c.Args[1], map[string]string{})
		require.True(t, ok && per == "1m", nm+" window must be \"1m\"")
		if nm == "loginRateLimiter" {
			require.Len(t, as, 1, "loginRateLimiter must never be reassigned")
			continue
		}
		require.Len(t, as, 2, nm+": one definition + the single Redis tier reassignment")
		re := as[1]
		require.True(t, !re.define && len(re.lhs) == 2 && re.lhs[0] == "authRateLimiter" && re.lhs[1] == "defaultRateLimiter" && len(re.rhs) == 1,
			nm+": reassignment must be `authRateLimiter, defaultRateLimiter = ...`")
		rc, ok := isCallSel(re.rhs[0], "root_middleware", "NewRedisTierLimiters")
		require.True(t, ok && len(rc.Args) == 3, "reassignment must call root_middleware.NewRedisTierLimiters(redisClient, authRateLimiter, defaultRateLimiter)")
		for i, an := range []string{"redisClient", "authRateLimiter", "defaultRateLimiter"} {
			id, ok := rc.Args[i].(*ast.Ident)
			require.True(t, ok && id.Name == an, "NewRedisTierLimiters arg %d must be %s", i, an)
		}
	}

	// --- group shape.
	require.Equal(t, 1, groupAssigns, "authGroup assigned exactly once")
	require.NotNil(t, groupCall)
	require.True(t, func() bool { _, ok := isCallSel(groupCall, "router", "Group"); return ok }(),
		"authGroup must come directly from router.Group(...)")
	require.Len(t, groupCall.Args, 1, "router.Group for auth must have no middleware argument")
	p, ok := foldString(groupCall.Args[0], map[string]string{})
	require.True(t, ok && p == "/api/v1/auth", "authGroup prefix must be /api/v1/auth, got %q", p)
	// every use of the ident authGroup other than its definition must be the
	// first argument of registerAuthRoutes.
	uses := 0
	ast.Inspect(f, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.SelectorExpr:
			if id, ok := v.X.(*ast.Ident); ok && id.Name == "authGroup" {
				t.Fatalf("method/field %q used on authGroup: no middleware may be added to the auth group", v.Sel.Name)
			}
		case *ast.CallExpr:
			for _, a := range v.Args {
				if id, ok := a.(*ast.Ident); ok && id.Name == "authGroup" {
					if fn, ok := v.Fun.(*ast.Ident); !ok || fn.Name != "registerAuthRoutes" {
						t.Fatalf("authGroup passed to something other than registerAuthRoutes")
					}
					uses++
				}
			}
		}
		return true
	})
	require.Equal(t, 1, uses, "authGroup used exactly once, as the registerAuthRoutes argument")
}
