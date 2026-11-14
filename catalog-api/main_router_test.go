package main

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
	"unsafe"

	root_config "catalogizer/config"
	root_handlers "catalogizer/handlers"
	"catalogizer/internal/firebase"
	root_middleware "catalogizer/middleware"

	"digital.vasic.discovery/pkg/broadcast"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// CAT-CHPW-RL-2 runtime tier: the test builds the REAL gin engine through
// buildRouter (the function main() uses) and reads, from the engine's own route
// trees, the full ordered handler chain of every route. Unit-class: it never
// starts a server and opens no connection; the limiters are tagging fakes so that
// each route's limiter identity is observable, everything else (engine middleware,
// JWT middleware, handler method values) is the production code.

func routerTestLoginRate(c *gin.Context)   { c.Next() }
func routerTestAuthRate(c *gin.Context)    { c.Next() }
func routerTestDefaultRate(c *gin.Context) { c.Next() }

func routerTestDeps() routerDeps {
	gin.SetMode(gin.TestMode)
	nop := zap.NewNop()
	return routerDeps{
		Logger:       nop,
		Cfg:          &root_config.Config{},
		ServiceInfo:  broadcast.ServiceInfo{},
		StartTime:    time.Now(),
		FirebaseApp:  firebase.New(firebase.Config{Logger: nop}),
		JWT:          root_middleware.NewJWTMiddleware("router-test-secret"),
		WS:           root_handlers.NewWebSocketHandler(nop),
		LoginRate:    routerTestLoginRate,
		AuthRate:     routerTestAuthRate,
		DefaultRate:  routerTestDefaultRate,
		TrustedProxy: "",
	}
}

// routeChains reads every route's ordered handler-name chain from the engine's
// private route trees (gin v1.12: Engine.trees []methodTree{method, root *node},
// node.handlers). The reflection is test-only; sanity controls below prove it sees
// the same routes as Engine.Routes().
func routeChains(t *testing.T, e *gin.Engine) map[string][]string {
	t.Helper()
	out := map[string][]string{}
	trees := reflect.ValueOf(e).Elem().FieldByName("trees")
	require.True(t, trees.IsValid(), "gin Engine.trees not found: the gin layout changed, update routeChains")
	var walk func(method string, n reflect.Value)
	walk = func(method string, n reflect.Value) {
		if n.IsNil() {
			return
		}
		nn := n.Elem()
		hv := nn.FieldByName("handlers")
		fp := nn.FieldByName("fullPath")
		require.True(t, hv.IsValid() && fp.IsValid(), "gin node layout changed")
		hv = reflect.NewAt(hv.Type(), unsafe.Pointer(hv.UnsafeAddr())).Elem()
		fp = reflect.NewAt(fp.Type(), unsafe.Pointer(fp.UnsafeAddr())).Elem()
		if hv.Len() > 0 {
			var names []string
			for i := 0; i < hv.Len(); i++ {
				names = append(names, handlerKey(hv.Index(i).Interface().(gin.HandlerFunc)))
			}
			key := method + " " + fp.String()
			require.NotContains(t, out, key, "route listed twice in the trees")
			out[key] = names
		}
		ch := nn.FieldByName("children")
		ch = reflect.NewAt(ch.Type(), unsafe.Pointer(ch.UnsafeAddr())).Elem()
		for i := 0; i < ch.Len(); i++ {
			walk(method, ch.Index(i))
		}
	}
	for i := 0; i < trees.Len(); i++ {
		mt := trees.Index(i)
		m := reflect.NewAt(mt.FieldByName("method").Type(), unsafe.Pointer(mt.FieldByName("method").UnsafeAddr())).Elem().String()
		root := reflect.NewAt(mt.FieldByName("root").Type(), unsafe.Pointer(mt.FieldByName("root").UnsafeAddr())).Elem()
		walk(m, root)
	}
	// Control: the tree walk and Engine.Routes() describe the same route set.
	var fromRoutes []string
	for _, ri := range e.Routes() {
		fromRoutes = append(fromRoutes, ri.Method+" "+ri.Path)
	}
	var fromTrees []string
	for k := range out {
		fromTrees = append(fromTrees, k)
	}
	sort.Strings(fromRoutes)
	sort.Strings(fromTrees)
	require.Equal(t, fromRoutes, fromTrees, "tree walk must see exactly the routes Engine.Routes() lists")
	return out
}

// keyedChains is the identity now: routeChains already returns file-qualified keys.
func keyedChains(chains map[string][]string) map[string][]string { return chains }

// wantEngineLevel is the engine-level handler chain in force when the auth group
// is created (the chain prefix of every auth route). Changing it is a deliberate
// edit of this list.
var wantEngineLevel = []string{
	"github.com/gin-gonic/gin@v1.12.0/logger.go:LoggerWithConfig",
	"internal/middleware/recovery.go:Recovery",
	"middleware/security_headers.go:SecurityHeadersWithConfig",
	"middleware/concurrency_limiter.go:ConcurrencyLimiter",
	"middleware/timeout.go:RequestTimeout",
	"middleware/request.go:CORS",
	"internal/metrics/middleware.go:GinMiddleware",
	"internal/middleware/middleware.go:Logger",
	"internal/middleware/middleware.go:ErrorHandler",
	"internal/firebase/firebase.go:GinMiddleware",
	"middleware/request.go:RequestID",
	"middleware/input_validation.go:InputValidation",
	"internal/middleware/compression.go:CompressionMiddleware",
}

// Keys are "<source file relative to the module root>:<identifier>" (production
// handlerKey); the root is derived at run time, so they carry no directory name. A
// method value wrapper ("-fm") has no source file and keeps its full import-path name.
const (
	kLoginRate   = "main_router_test.go:routerTestLoginRate"
	kAuthRate    = "main_router_test.go:routerTestAuthRate"
	kDefaultRate = "main_router_test.go:routerTestDefaultRate"
	kRequireAuth = "middleware/auth.go:(*JWTMiddleware).RequireAuth"
	kChpwLimiter = "middleware/failed_attempt_limiter.go:(*FailedAttemptLimiter).Middleware"
	kHandlers    = "<generated>:catalogizer/handlers."
)

// wantAuthRoutes pins, per auth route, the handlers after the engine-level ones:
// which limiter (the tagging fakes of routerTestDeps), whether RequireAuth runs,
// the change-password failed-attempt limiter, and the final handler.
var wantAuthRoutes = map[string][]string{
	"POST /api/v1/auth/login":           {kLoginRate, kHandlers + "(*AuthHandler).LoginGin-fm"},
	"POST /api/v1/auth/register":        {kLoginRate, "main_router.go:buildRouter"},
	"POST /api/v1/auth/refresh":         {kAuthRate, kHandlers + "(*AuthHandler).RefreshTokenGin-fm"},
	"POST /api/v1/auth/logout":          {kAuthRate, kHandlers + "(*AuthHandler).LogoutGin-fm"},
	"GET /api/v1/auth/me":               {kAuthRate, kRequireAuth, kHandlers + "(*AuthHandler).GetCurrentUserGin-fm"},
	"GET /api/v1/auth/status":           {kAuthRate, kHandlers + "(*AuthHandler).GetAuthStatusGin-fm"},
	"GET /api/v1/auth/permissions":      {kAuthRate, kRequireAuth, kHandlers + "(*AuthHandler).GetPermissionsGin-fm"},
	"GET /api/v1/auth/profile":          {kAuthRate, kRequireAuth, kHandlers + "(*AuthHandler).GetCurrentUserGin-fm"},
	"GET /api/v1/auth/init-status":      {kDefaultRate, kHandlers + "(*MediaQueryHandler).GetInitStatus-fm"},
	"POST /api/v1/auth/change-password": {kDefaultRate, kRequireAuth, kChpwLimiter, kHandlers + "(*MediaQueryHandler).ChangePassword-fm"},
}

// inAuthSurface is the test's OWN oracle, independent of production
// patternReachesAuthPaths: it instantiates the (gin-cleaned) pattern with concrete
// witness values, keeps the candidates that are /api/v1/auth paths (case-folded),
// and asks a real gin engine, with the pattern registered alone, whether it answers
// that candidate.
func inAuthSurface(pattern string) bool {
	clean := path.Clean("/" + pattern)
	segs := strings.Split(strings.Trim(clean, "/"), "/")
	if clean == "/" {
		return false
	}
	// Conservative extras kept from the earlier oracle: a bare /auth and the
	// /api/auth shape are treated as auth-surface spellings too.
	if l := strings.ToLower(segs[0]); l == "auth" || (l == "api" && len(segs) >= 2 && strings.EqualFold(segs[1], "auth")) {
		return true
	}
	paramVals := []string{"api", "v1", "auth", "zz", ""}
	partialVals := []string{"h", "zz"}
	tails := []string{"", "zz", "v1/auth/zz", "auth/zz", "api/v1/auth/zz"}
	cands := []string{""}
	for _, sg := range segs {
		w := strings.IndexAny(sg, ":*")
		var opts []string
		switch {
		case w < 0:
			opts = []string{sg}
		case sg[w] == ':' && w == 0:
			opts = paramVals
		case sg[w] == ':':
			for _, v := range partialVals {
				opts = append(opts, sg[:w]+v)
			}
		default: // catch-all: the rest of the path
			for _, tl := range tails {
				opts = append(opts, strings.TrimPrefix(sg[:w]+tl, "/"))
			}
		}
		var next []string
		for _, c := range cands {
			for _, o := range opts {
				next = append(next, c+"/"+o)
			}
		}
		cands = next
		if w >= 0 && sg[w] == '*' {
			break // nothing can follow a catch-all
		}
	}
	gin.SetMode(gin.TestMode)
	for _, c := range cands {
		// The auth surface is every request path whose CLEANED form (empty segments
		// dropped, as path.Clean, RedirectFixedPath and normalising proxies do) is under
		// /api/v1/auth; gin itself routes empty parameter values, so "//api/v1/auth/x"
		// can be answered by "/:p/api/v1/auth/x" (round-4 review R4-1/R4-2).
		var cs []string
		for _, sg := range strings.Split(strings.ToLower(c), "/") {
			if sg != "" {
				cs = append(cs, sg)
			}
		}
		if len(cs) < 3 || cs[0] != "api" || cs[1] != "v1" || cs[2] != "auth" {
			continue
		}
		answered := false
		func() {
			defer func() { _ = recover() }() // gin refuses some patterns: then the candidate alone decides
			e := gin.New()
			e.GET(clean, func(*gin.Context) {})
			rec := httptest.NewRecorder()
			req := httptest.NewRequest("GET", c, nil)
			e.ServeHTTP(rec, req)
			answered = rec.Code != 404
		}()
		if answered {
			return true
		}
		if registrationPanics(func() { gin.New().GET(clean, func(*gin.Context) {}) }) {
			return true
		}
	}
	return false
}

// authSurfaceDiff compares the keyed chains of every route on the auth surface
// with the pinned table. Deny-by-default: an unknown route, a missing route, a
// changed chain or a different engine-level prefix is a finding.
func authSurfaceDiff(chains map[string][]string) []string {
	var out []string
	seen := map[string]bool{}
	var keys []string
	for k := range chains {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		p := strings.SplitN(k, " ", 2)[1]
		if !inAuthSurface(p) {
			continue
		}
		want, ok := wantAuthRoutes[k]
		if !ok {
			out = append(out, fmt.Sprintf("unknown route on the auth surface: %s (chain %v)", k, chains[k]))
			continue
		}
		seen[k] = true
		wantFull := append(append([]string{}, wantEngineLevel...), want...)
		if !reflect.DeepEqual(chains[k], wantFull) {
			out = append(out, fmt.Sprintf("%s: handler chain %v, want %v", k, chains[k], wantFull))
		}
	}
	for k := range wantAuthRoutes {
		if !seen[k] {
			out = append(out, "missing auth route: "+k)
		}
	}
	sort.Strings(out)
	return out
}

func TestBuildRouterAuthSurface(t *testing.T) {
	e := buildRouter(routerTestDeps())
	chains := keyedChains(routeChains(t, e))
	// Controls: the walk sees routes outside the auth surface too, with the same
	// engine-level prefix, and does not see a route that was never registered.
	require.Contains(t, chains, "GET /health")
	require.Equal(t, wantEngineLevel, chains["GET /health"][:len(wantEngineLevel)])
	require.NotContains(t, chains, "POST /api/v1/auth/never-registered")
	require.Len(t, wantAuthRoutes, len(authRouteTable), "the test's table and the production authRouteTable must list the same routes")
	for _, k := range authRouteTable {
		require.Contains(t, wantAuthRoutes, k, "authRouteTable entry missing from the pinned chains")
	}
	require.Empty(t, authSurfaceDiff(chains))
	require.NoError(t, verifyAuthRouteTable(e))
}

// Comparator golden-bad set: each hand-made engine/chain shape must be reported.
func TestBuildRouterAuthSurface_ComparatorDetects(t *testing.T) {
	base := func() map[string][]string {
		return keyedChains(routeChains(t, buildRouter(routerTestDeps())))
	}
	login := "POST /api/v1/auth/login"
	cases := map[string]func(m map[string][]string){
		"limiter_dropped_from_one_route": func(m map[string][]string) {
			c := m[login]
			m[login] = append(append([]string{}, c[:len(wantEngineLevel)]...), c[len(wantEngineLevel)+1:]...)
		},
		"limiters_swapped_login_uses_auth_rate": func(m map[string][]string) {
			m[login] = append([]string{}, m[login]...)
			m[login][len(wantEngineLevel)] = kAuthRate
		},
		"route_moved_outside_the_group": func(m map[string][]string) {
			m["POST /api/v1/login"] = m[login]
			delete(m, login)
		},
		"evil_handler_injected_after_engine_level": func(m map[string][]string) {
			c := m[login]
			m[login] = append(append(append([]string{}, c[:len(wantEngineLevel)]...), "evil"), c[len(wantEngineLevel):]...)
		},
		"evil_engine_level_prefix": func(m map[string][]string) {
			m[login] = append([]string{"evil"}, m[login]...)
		},
		"new_unpinned_auth_route": func(m map[string][]string) { m["GET /api/v1/auth/evil"] = []string{"evil"} },
		"unclean_path_spelling":   func(m map[string][]string) { m["POST /api//v1/auth/evil"] = []string{"evil"} },
		"relative_dot_path":       func(m map[string][]string) { m["GET ./auth/evil"] = []string{"evil"} },
		"param_segment_capturing_auth": func(m map[string][]string) {
			m["GET /api/v1/:x/login"] = []string{"evil"}
		},
		"upper_case_spelling": func(m map[string][]string) { m["GET /API/V1/AUTH/evil"] = []string{"evil"} },
		"route_removed":       func(m map[string][]string) { delete(m, login) },
	}
	require.Empty(t, authSurfaceDiff(base()), "golden-good control")
	for name, mut := range cases {
		mut := mut
		t.Run(name, func(t *testing.T) {
			m := base()
			mut(m)
			require.NotEmpty(t, authSurfaceDiff(m), "comparator missed this shape")
		})
	}
}

// registrationPanics reports whether fn panicked (gin panics on route conflicts).
func registrationPanics(fn func()) (panicked bool) {
	defer func() {
		if recover() != nil {
			panicked = true
		}
	}()
	fn()
	return false
}

func TestVerifyAuthRouteTable_RealEngines(t *testing.T) {
	gin.SetMode(gin.TestMode)
	good := buildRouter(routerTestDeps())
	require.NoError(t, verifyAuthRouteTable(good))
	for name, mut := range map[string]func(e *gin.Engine){
		"extra_auth_route":       func(e *gin.Engine) { e.GET("/api/v1/auth/evil", func(*gin.Context) {}) },
		"unclean_double_slash":   func(e *gin.Engine) { e.POST("/api//v1/auth/evil", func(*gin.Context) {}) },
		"relative_dot_under_api": func(e *gin.Engine) { e.Group("/api/v1").GET("./auth/evil", func(*gin.Context) {}) },
		"patch_method":           func(e *gin.Engine) { e.PATCH("/api/v1/auth/evil", func(*gin.Context) {}) },
		"static_file":            func(e *gin.Engine) { e.StaticFile("/api/v1/auth/f", "x") },
		"no_route_handler":       func(e *gin.Engine) { e.NoRoute(func(c *gin.Context) { c.Status(200) }) },
		"no_method_handler":      func(e *gin.Engine) { e.NoMethod(func(c *gin.Context) { c.Status(200) }) },
		// Round-2 review F1a: capture / partial-parameter segments OUTSIDE segment 3 that
		// gin matches against real /api/v1/auth requests (the reviewer's six shapes).
		"F1a1_capture_v_get":         func(e *gin.Engine) { e.GET("/api/:v/auth/evil", func(*gin.Context) {}) },
		"F1a2_capture_v_put_login":   func(e *gin.Engine) { e.PUT("/api/:v/auth/login", func(*gin.Context) {}) },
		"F1a3_root_capture_a":        func(e *gin.Engine) { e.GET("/:a/v1/auth/evil", func(*gin.Context) {}) },
		"F1a4_partial_param_segment": func(e *gin.Engine) { e.GET("/api/v1/aut:x/evil", func(*gin.Context) {}) },
		"F1a5_group_api_capture":     func(e *gin.Engine) { e.Group("/api").GET("/:v/auth/login", func(*gin.Context) {}) },
		"F1a6_two_captures_delete":   func(e *gin.Engine) { e.DELETE("/:a/:b/auth/login", func(*gin.Context) {}) },
		// Round-2 review F1b: a global middleware that answers auth paths.
		"F1b_global_use_answering": func(e *gin.Engine) { e.Use(func(c *gin.Context) { c.AbortWithStatus(200) }) },
		// Own mutants in the same class (structural round).
		"M1_any_capture":            func(e *gin.Engine) { e.Any("/api/:v/auth/any", func(*gin.Context) {}) },
		"M2_custom_method_partial":  func(e *gin.Engine) { e.Handle("PROPFIND", "/api/v1/aut:z/x", func(*gin.Context) {}) },
		"M3_catch_all_under_api":    func(e *gin.Engine) { e.GET("/api/*x", func(*gin.Context) {}) },
		"M4_catch_all_root":         func(e *gin.Engine) { e.GET("/*rest", func(*gin.Context) {}) },
		"M5_param_prefix_group":     func(e *gin.Engine) { e.Group("/:a").Group("/v1").GET("/auth/x", func(*gin.Context) {}) },
		"M6_trailing_slash_variant": func(e *gin.Engine) { e.GET("/api/v1/auth/", func(*gin.Context) {}) },
		"M7_capture_under_auth":     func(e *gin.Engine) { e.GET("/api/v1/auth/:x", func(*gin.Context) {}) },
		"M8_upper_case_partial":     func(e *gin.Engine) { e.GET("/API/:v/AUTH/x", func(*gin.Context) {}) },
		"M9_redirect_fixed_path":    func(e *gin.Engine) { e.RedirectFixedPath = true },
		"M10_remove_extra_slash":    func(e *gin.Engine) { e.RemoveExtraSlash = true },
		"M11_handle_405":            func(e *gin.Engine) { e.HandleMethodNotAllowed = true },
		"M12_redirect_trailing_off": func(e *gin.Engine) { e.RedirectTrailingSlash = false },
		"M13_use_raw_path":          func(e *gin.Engine) { e.UseRawPath = true },
		"M14_unescape_values_off":   func(e *gin.Engine) { e.UnescapePathValues = false },
		// Round-3 review F-A: the %2F probe. With UseEscapedPath a route spelled
		// "/api/v1/auth%2Fevil" answers a request whose URL.Path is /api/v1/auth/evil.
		"FA_use_escaped_path": func(e *gin.Engine) {
			e.UseEscapedPath = true
			e.GET("/api/v1/auth%2Fevil", func(*gin.Context) {})
		},
		"FA2_use_escaped_path_flag_only": func(e *gin.Engine) { e.UseEscapedPath = true },
		// Round-4 review R4-1: an EMPTY parameter value shifts the alignment. gin answers
		// "//api/v1/auth/me" with "/:p/api/v1/auth/me" (p == "").
		"X1_leading_param_shift":      func(e *gin.Engine) { e.GET("/:p/api/v1/auth/me", func(*gin.Context) {}) },
		"X2_group_param_any_catchall": func(e *gin.Engine) { e.Group("/:p").Any("/api/v1/auth/*rest", func(*gin.Context) {}) },
		"X3_param_between_api_v1":     func(e *gin.Engine) { e.GET("/api/:p/v1/auth/me", func(*gin.Context) {}) },
		"X4_param_between_v1_auth":    func(e *gin.Engine) { e.GET("/api/v1/:p/auth/me", func(*gin.Context) {}) },
		"X5_two_leading_params":       func(e *gin.Engine) { e.GET("/:p/:q/api/v1/auth/me", func(*gin.Context) {}) },
		"X6_leading_param_then_param": func(e *gin.Engine) { e.GET("/:p/api/:q/auth/me", func(*gin.Context) {}) },
		"X7_leading_catchall":         func(e *gin.Engine) { e.GET("/*rest", func(*gin.Context) {}) },
		"X8_param_then_catchall":      func(e *gin.Engine) { e.GET("/:p/*rest", func(*gin.Context) {}) },
		"X9_param_at_auth_position":   func(e *gin.Engine) { e.GET("/api/v1/:p/me", func(*gin.Context) {}) },
		"X10_param_at_root_only":      func(e *gin.Engine) { e.GET("/:p/v1/auth/me", func(*gin.Context) {}) },
		// Round-3 review F-C: mixed-case partial parameter (drops ToLower in RM2).
		"FC_mixed_case_partial_param": func(e *gin.Engine) { e.GET("/api/v1/AUT:x/evil", func(*gin.Context) {}) },
	} {
		mut := mut
		t.Run(name, func(t *testing.T) {
			e := buildRouter(routerTestDeps())
			// gin itself refuses some shapes at registration (a panic at startup, which
			// is fail-closed); that counts as a kill, any other outcome must be an error
			// from the verifier.
			if registrationPanics(func() { mut(e) }) {
				t.Logf("gin refused the registration itself (startup panic): killed by gin")
				return
			}
			err := verifyAuthRouteTable(e)
			require.Error(t, err)
			// Every mutant above is refused for one of these specific reasons.
			require.Regexp(t, `auth route table mismatch|path-matching flags differ|differs from the pinned chain|chain differs from the global middleware`, err.Error())
		})
	}
	// golden-FALSE: a sibling prefix and a non-auth api route must not fire.
	e := buildRouter(routerTestDeps())
	e.GET("/api/v1/authors", func(*gin.Context) {})
	e.GET("/api/v1/catalog/x", func(*gin.Context) {})
	require.NoError(t, verifyAuthRouteTable(e))
	// golden-FALSE: main() adds exactly one late global Use, altSvcMiddleware; the
	// verifier accepts the chain with it appended and nothing else.
	e.Use(altSvcMiddleware)
	require.NoError(t, verifyAuthRouteTable(e))
	e.Use(func(c *gin.Context) { c.Next() })
	require.Error(t, verifyAuthRouteTable(e), "a second late Use is refused")
	// golden-FALSE: other registered shapes that cannot answer an /api/v1/auth path.
	e2 := buildRouter(routerTestDeps())
	e2.GET("/api/:v/authors", func(*gin.Context) {})
	e2.GET("/api/v2/:x/auth", func(*gin.Context) {})
	e2.GET("/:a/health", func(*gin.Context) {})
	e2.GET("/api/v1/files/:id/auth", func(*gin.Context) {})
	require.NoError(t, verifyAuthRouteTable(e2))
}

func TestAltSvcMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	e := gin.New()
	e.Use(altSvcMiddleware)
	called := false
	e.GET("/x", func(c *gin.Context) { called = true; c.Status(204) })
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest("GET", "/x", nil))
	require.True(t, called, "altSvcMiddleware must call Next")
	require.Equal(t, `h3=":28443"; ma=86400`, rec.Header().Get("Alt-Svc"))
	require.Equal(t, altSvcKey, handlerKey(altSvcMiddleware))
}

// handlerKey must not depend on the checkout directory (review round 2, F4).
func TestHandlerKeyIndependentOfCheckoutPath(t *testing.T) {
	require.Equal(t, "main_router.go:altSvcMiddleware", handlerKey(altSvcMiddleware))
	require.Equal(t, altSvcKey, handlerKey(altSvcMiddleware))
	require.Equal(t, "main_router_test.go:routerTestLoginRate", handlerKey(routerTestLoginRate))
	require.Equal(t, "middleware/request.go:CORS", handlerKey(root_middleware.CORS()))
	require.Equal(t, "middleware/auth.go:(*JWTMiddleware).RequireAuth", handlerKey(root_middleware.NewJWTMiddleware("k").RequireAuth()))
	require.NotContains(t, strings.Join(wantEngineLevel, ","), "catalog-api", "keys must carry no directory name")
	require.NotContains(t, strings.Join(wantGlobalChain, ","), "catalog-api")
	require.Equal(t, wantEngineLevel, wantGlobalChain, "the test's independent pin and the production pin list the same chain")
	// keyFile: the module root and a dependency path are cut, anything else stays full.
	root := moduleRootDir()
	require.NotEmpty(t, root)
	require.Equal(t, "x/y.go", keyFile(root+"x/y.go"))
	require.Equal(t, "github.com/gin-gonic/gin@v1.12.0/logger.go", keyFile("/any/cache/dir/github.com/gin-gonic/gin@v1.12.0/logger.go"))
	require.Equal(t, "/elsewhere/z.go", keyFile("/elsewhere/z.go"))
}

// ---- runtime-tier source mutants (opt-in) -------------------------------------
//
// Each mutant edits the real sources through `go test -overlay` (nothing in the
// work tree is written) and runs TestBuildRouterAuthSurface on the compiled
// mutated package: the test MUST fail. Opt-in because every case compiles the
// package (about a minute each): CHPW_RUNTIME_MUTANTS=1.
func runtimeMutant(t *testing.T, edits map[string][][2]string) string {
	t.Helper()
	return runtimeMutantRun(t, edits, "TestBuildRouterAuthSurface")
}

// runtimeMutantRun runs the named test (regexp-anchored name) on the mutated package.
func runtimeMutantRun(t *testing.T, edits map[string][][2]string, testName string, extra ...string) string {
	t.Helper()
	tmp := t.TempDir()
	require.True(t, strings.HasPrefix(tmp, os.TempDir()))
	overlay := map[string]map[string]string{"Replace": {}}
	for file, es := range edits {
		abs, err := filepath.Abs(file)
		require.NoError(t, err)
		b, err := os.ReadFile(file)
		require.NoError(t, err)
		s := string(b)
		for _, e := range es {
			require.Equal(t, 1, strings.Count(s, e[0]), "mutation anchor must occur exactly once in %s: %s", file, e[0])
			s = strings.Replace(s, e[0], e[1], 1)
		}
		dst := filepath.Join(tmp, filepath.Base(file))
		require.NoError(t, os.WriteFile(dst, []byte(s), 0o600))
		overlay["Replace"][abs] = dst
	}
	oj, err := json.Marshal(overlay)
	require.NoError(t, err)
	op := filepath.Join(tmp, "overlay.json")
	require.NoError(t, os.WriteFile(op, oj, 0o600))
	args := append([]string{"-n", "10", "go", "test", "-buildvcs=false", "-p", "1", "-count=1", "-overlay=" + op}, extra...)
	cmd := exec.Command("nice", append(args, "-run", "^"+testName+"$", ".")...)
	cmd.Env = append(os.Environ(), "GOMAXPROCS=2", "CHPW_RUNTIME_MUTANTS=")
	out, err := cmd.CombinedOutput()
	require.Error(t, err, "mutant SURVIVED the runtime test:\n%s", out)
	require.NotContains(t, string(out), "[build failed]", "mutant did not compile (not a kill):\n%s", out)
	require.Contains(t, string(out), "--- FAIL: "+testName, string(out))
	return string(out)
}

func TestBuildRouterAuthSurface_RuntimeMutants(t *testing.T) {
	if os.Getenv("CHPW_RUNTIME_MUTANTS") != "1" {
		t.Skip("opt-in (compiles the package per mutant): set CHPW_RUNTIME_MUTANTS=1")
	}
	const defn = `authGroup := router.Group("/api/v1/auth")`
	m := func(file, old, repl string) map[string][][2]string {
		return map[string][][2]string{file: {{old, repl}}}
	}
	cases := []struct {
		name  string
		edits map[string][][2]string
	}{
		{"RV1_method_value_Use_before_group", m("main_router.go", defn, "u := router.Use\n\tu(func(c *gin.Context) { c.Next() })\n\t"+defn)},
		{"RV2_method_value_Group_auth_route", m("main_router.go", defn, defn+"\n\tgrp := router.Group\n\tgrp(\"/api/v1/auth\").POST(\"/evil\", func(c *gin.Context) {})")},
		{"RV3_unclean_double_slash_route", m("main_router.go", defn, defn+"\n\trouter.POST(\"/api//v1/auth/evil\", func(c *gin.Context) {})")},
		{"RV5_g_Handlers_prepend_in_registerAuthRoutes", m("main_chpw.go", `g.POST("/login", d.LoginRate, d.Login)`, "g.Handlers = append(gin.HandlersChain{func(c *gin.Context) { c.Next() }}, g.Handlers...)\n\tg.POST(\"/login\", d.LoginRate, d.Login)")},
		{"RV6_engine_Handlers_append_in_applyTrustedProxies", m("trusted_proxies.go", "if err := setTrustedProxies(r, tp); err != nil {", "r.Handlers = append(r.Handlers, func(c *gin.Context) { c.Next() })\n\tif err := setTrustedProxies(r, tp); err != nil {")},
		{"RV8_method_value_Use_in_ws_helper", m("main_wsroute.go", `router.GET("/ws", mw.RequireAuth(), h.HandleConnection)`, "u := router.Use\n\tu(func(c *gin.Context) { c.Next() })\n\trouter.GET(\"/ws\", mw.RequireAuth(), h.HandleConnection)")},
		{"RV9_RouterGroup_field_Use_in_trusted_proxies", m("trusted_proxies.go", "if err := setTrustedProxies(r, tp); err != nil {", "rg := &r.RouterGroup\n\tf := rg.Use\n\tf(func(c *gin.Context) { c.Next() })\n\tif err := setTrustedProxies(r, tp); err != nil {")},
		{"N1_limiter_dropped_from_login", m("main_chpw.go", `g.POST("/login", d.LoginRate, d.Login)`, `g.POST("/login", d.Login)`)},
		{"N2_limiters_swapped_login_uses_auth_rate", m("main_chpw.go", `g.POST("/login", d.LoginRate, d.Login)`, `g.POST("/login", d.AuthRate, d.Login)`)},
		{"N3_route_moved_outside_group", m("main_chpw.go", `g.POST("/logout", d.AuthRate, d.Logout)`, "")},
		{"N4_duplicate_allowlisted_engine_Use", m("main_router.go", defn, "router.Use(root_middleware.RequestID())\n\t"+defn)},
		{"N5_engine_Use_lookalike_swapped_order", m("main_router.go", "router.Use(root_middleware.CORS())\n\trouter.Use(metrics.GinMiddleware())", "router.Use(metrics.GinMiddleware())\n\trouter.Use(root_middleware.CORS())")},
		{"N6_require_auth_dropped_from_me", m("main_chpw.go", `g.GET("/me", d.AuthRate, d.RequireAuth(), d.Me)`, `g.GET("/me", d.AuthRate, d.Me)`)},
		{"R1_NoRoute_handler_in_buildRouter", m("main_router.go", defn, "router.NoRoute(func(c *gin.Context) { c.Status(200) })\n\t"+defn)},
		{"R4_lookalike_AuthHandler_wired_as_login", m("main_chpw.go", `g.POST("/login", d.LoginRate, d.Login)`, `g.POST("/login", d.LoginRate, gin.HandlerFunc(func(c *gin.Context) { c.Status(200) }))`)},
		{"R13_lookalike_CORS_other_package", m("main_router.go", "router.Use(root_middleware.CORS())", "router.Use(middleware.CORS())")},
		{"R2_swap_metrics_and_firebase_GinMiddleware", map[string][][2]string{"main_router.go": {
			{"router.Use(metrics.GinMiddleware())", "router.Use(firebase.GinMiddleware(firebaseApp)) //M"},
			{"router.Use(firebase.GinMiddleware(firebaseApp))\n\trouter.Use(root_middleware.RequestID())", "router.Use(metrics.GinMiddleware())\n\trouter.Use(root_middleware.RequestID())"}}}},
		{"R6_InputValidation_after_group", map[string][][2]string{"main_router.go": {
			{"router.Use(root_middleware.InputValidation(root_middleware.DefaultInputValidationConfig()))\n", ""},
			{defn, defn + "\n\trouter.Use(root_middleware.InputValidation(root_middleware.DefaultInputValidationConfig()))"}}}},
		{"R8_NoMethod_handler_in_buildRouter", m("main_router.go", defn, "router.NoMethod(func(c *gin.Context) { c.Status(200) })\n\t"+defn)},
		{"F1a1_capture_v_route_in_buildRouter", m("main_router.go", defn, defn+"\n\trouter.GET(\"/api/:v/auth/evil\", func(c *gin.Context) {})")},
		{"F1a2_PUT_capture_login_in_buildRouter", m("main_router.go", defn, defn+"\n\trouter.PUT(\"/api/:v/auth/login\", func(c *gin.Context) {})")},
		{"F1a3_root_capture_in_buildRouter", m("main_router.go", defn, defn+"\n\trouter.GET(\"/:a/v1/auth/evil\", func(c *gin.Context) {})")},
		{"F1a4_partial_param_in_buildRouter", m("main_router.go", defn, defn+"\n\trouter.GET(\"/api/v1/aut:x/evil\", func(c *gin.Context) {})")},
		{"F1a5_group_api_capture_in_buildRouter", m("main_router.go", defn, defn+"\n\trouter.Group(\"/api\").GET(\"/:v/auth/login\", func(c *gin.Context) {})")},
		{"F1a6_two_captures_in_buildRouter", m("main_router.go", defn, defn+"\n\trouter.DELETE(\"/:a/:b/auth/login\", func(c *gin.Context) {})")},
		{"F1b_global_Use_after_group_in_buildRouter", m("main_router.go", defn, defn+"\n\trouter.Use(func(c *gin.Context) { c.AbortWithStatus(200) })")},
		{"M1_Any_capture_in_buildRouter", m("main_router.go", defn, defn+"\n\trouter.Any(\"/api/:v/auth/any\", func(c *gin.Context) {})")},
		{"M2_flag_RedirectFixedPath_in_buildRouter", m("main_router.go", defn, "router.RedirectFixedPath = true\n\t"+defn)},
		{"M3_flag_HandleMethodNotAllowed_in_buildRouter", m("main_router.go", defn, "router.HandleMethodNotAllowed = true\n\t"+defn)},
		{"M4_flag_RemoveExtraSlash_in_buildRouter", m("main_router.go", defn, "router.RemoveExtraSlash = true\n\t"+defn)},
		{"M5_global_chain_pin_dropped_middleware", m("main_router.go", "router.Use(root_middleware.RequestID())\n", "")},
		{"N7_changepassword_limiter_dropped", m("main_chpw.go", `root_middleware.RegisterChangePassword(authGroup, "/change-password", rate, requireAuth, handler, lim)`, `authGroup.POST("/change-password", rate, requireAuth, handler)`)},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			out := runtimeMutant(t, tc.edits)
			t.Logf("KILLED by runtime test")
			_ = out
		})
	}
}

// Opt-in: the pins hold under -trimpath (no checkout directory in any key) and from a
// different working copy path (round-2 review F4).
func TestBuildRouterAuthSurface_Trimpath(t *testing.T) {
	if os.Getenv("CHPW_RUNTIME_MUTANTS") != "1" {
		t.Skip("opt-in (compiles the package): set CHPW_RUNTIME_MUTANTS=1")
	}
	cmd := exec.Command("nice", "-n", "10", "go", "test", "-buildvcs=false", "-trimpath", "-p", "1", "-count=1",
		"-run", "^(TestBuildRouterAuthSurface|TestHandlerKeyIndependentOfCheckoutPath|TestVerifyAuthRouteTable_RealEngines)$", ".")
	cmd.Env = append(os.Environ(), "GOMAXPROCS=2", "CHPW_RUNTIME_MUTANTS=")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "pins must hold under -trimpath:\n%s", out)
}

// Opt-in: altSvcMiddleware that stops calling Next, and a handlerKey that depends on a
// directory name, are both killed by their own tests.
func TestAltSvcAndHandlerKey_RuntimeMutants(t *testing.T) {
	if os.Getenv("CHPW_RUNTIME_MUTANTS") != "1" {
		t.Skip("opt-in (compiles the package per mutant): set CHPW_RUNTIME_MUTANTS=1")
	}
	// (c.Next() itself is redundant in gin: the dispatch loop continues after a handler
	// returns. The mutants are the ones that change behaviour: abort, or no header.)
	hdr := "\tc.Header(\"Alt-Svc\", `h3=\":28443\"; ma=86400`)\n\tc.Next()"
	t.Run("AS1_altsvc_aborts_the_chain", func(t *testing.T) {
		runtimeMutantRun(t, map[string][][2]string{"main_router.go": {{hdr, "\tc.Header(\"Alt-Svc\", `h3=\":28443\"; ma=86400`)\n\tc.Abort()"}}}, "TestAltSvcMiddleware")
	})
	t.Run("AS2_altsvc_header_dropped", func(t *testing.T) {
		runtimeMutantRun(t, map[string][][2]string{"main_router.go": {{hdr, "\tc.Next()"}}}, "TestAltSvcMiddleware")
	})
	t.Run("HK1_handlerKey_depends_on_directory_name_under_trimpath", func(t *testing.T) {
		runtimeMutantRun(t, map[string][][2]string{"main_router.go": {{"\tif root := moduleRootDir(); root != \"\" && strings.HasPrefix(file, root) {\n\t\treturn file[len(root):]", "\tif i := strings.LastIndex(file, \"/catalog-api/\"); i >= 0 {\n\t\treturn file[i+len(\"/catalog-api/\"):]"}}}, "TestHandlerKeyIndependentOfCheckoutPath", "-trimpath")
	})
}

// Round-3 review F-A (class-wide): every field of gin.Engine, exported or not, is
// classified. A matching-relevant field must be pinned to its default by the verifier
// (each is flipped here and must be refused); an irrelevant one carries its reason.
// A field gin adds later is unclassified and fails this test.
var ginEngineFieldClass = map[string]string{
	// matching-relevant bool flags: (name -> default value)
	"RedirectTrailingSlash": "match:true", "RedirectFixedPath": "match:false", "HandleMethodNotAllowed": "match:false",
	"UseRawPath": "match:false", "UseEscapedPath": "match:false", "UnescapePathValues": "match:true",
	"RemoveExtraSlash": "match:false", "UseH2C": "match:false",
	// matching-relevant chains/trees: pinned by the chain checks and the route table
	"RouterGroup": "pinned: Handlers chain via wantGlobalChain, basePath only prefixes registrations",
	"allNoRoute":  "pinned: must equal the global chain", "allNoMethod": "pinned: must equal the global chain",
	"noRoute": "pinned: folded into allNoRoute", "noMethod": "pinned: folded into allNoMethod",
	"trees": "the route table itself (verifyAuthRouteTable reads it through Routes())",
	// irrelevant to which handler a path reaches
	"routeTreesUpdated": "once-guard", "ForwardedByClientIP": "client IP only (F-D, separate item)",
	"AppEngine": "client IP header trust only (deprecated)", "RemoteIPHeaders": "client IP only (F-D)",
	"TrustedPlatform": "client IP only (F-D)", "trustedProxies": "client IP only (F-D)", "trustedCIDRs": "client IP only (F-D)",
	"MaxMultipartMemory": "body parsing only", "ContextWithFallback": "context deadline/value fallback only",
	"delims": "html templates", "secureJSONPrefix": "JSON rendering", "HTMLRender": "html templates", "FuncMap": "html templates",
	"pool": "context pool", "maxParams": "allocation sizing", "maxSections": "allocation sizing",
}

func TestGinEngineFieldsClassified(t *testing.T) {
	rt := reflect.TypeOf(gin.Engine{})
	seen := map[string]bool{}
	for i := 0; i < rt.NumField(); i++ {
		n := rt.Field(i).Name
		seen[n] = true
		require.Contains(t, ginEngineFieldClass, n, "gin.Engine field %q is not classified: decide whether it changes which handler a path reaches, and pin it in verifyAuthRouteTable if so", n)
	}
	for n := range ginEngineFieldClass {
		require.True(t, seen[n], "classified field %q no longer exists in gin.Engine", n)
	}
	gin.SetMode(gin.TestMode)
	for n, c := range ginEngineFieldClass {
		if !strings.HasPrefix(c, "match:") {
			continue
		}
		def := c == "match:true"
		require.NoError(t, verifyAuthRouteTable(buildRouter(routerTestDeps())), "control")
		e := buildRouter(routerTestDeps())
		reflect.ValueOf(e).Elem().FieldByName(n).SetBool(!def)
		require.Error(t, verifyAuthRouteTable(e), "flipping %s must be refused", n)
		// and gin's own New() default equals the table (the claim "default" is checked, not asserted)
		require.Equal(t, def, reflect.ValueOf(gin.New()).Elem().FieldByName(n).Bool(), "default of %s", n)
	}
}

// Round-3 review F-C: patternReachesAuthPaths agrees with the independent oracle
// (inAuthSurface: witness instantiation + a real gin engine) over a generated corpus
// of patterns (mixed case, partial parameters, params, catch-alls).
func TestPatternReachesAuthPaths_AgreesWithOracleOnCorpus(t *testing.T) {
	alpha := []string{"api", "API", "v1", "V1", "auth", "AUTH", "aut:x", "AUT:x", ":p", "*r", "zz"}
	var corpus []string
	var gen func(prefix []string, depth int)
	gen = func(prefix []string, depth int) {
		if len(prefix) > 0 {
			corpus = append(corpus, "/"+strings.Join(prefix, "/"))
		}
		if depth == 0 {
			return
		}
		for _, a := range alpha {
			if len(prefix) > 0 && strings.HasPrefix(prefix[len(prefix)-1], "*") {
				continue // nothing follows a catch-all
			}
			gen(append(append([]string{}, prefix...), a), depth-1)
		}
	}
	gen(nil, 4)
	n := 0
	for _, p := range corpus {
		segs := strings.Split(strings.Trim(p, "/"), "/")
		if l := strings.ToLower(segs[0]); l == "auth" || (l == "api" && len(segs) > 1 && strings.EqualFold(segs[1], "auth")) {
			continue // the oracle's conservative root-/auth extras are not part of the reach property
		}
		n++
		require.Equal(t, inAuthSurface(p), patternReachesAuthPaths(p), "pattern %q", p)
	}
	require.Greater(t, n, 5000, "corpus control")
}

// Round-4 (review F-A/F-C): runtime overlay mutants of the verifier itself. Opt-in
// (CHPW_RUNTIME_MUTANTS=1); each compiles the package with one production edit and the
// named test must FAIL.
func TestVerifier_RuntimeMutants(t *testing.T) {
	if os.Getenv("CHPW_RUNTIME_MUTANTS") != "1" {
		t.Skip("opt-in (compiles the package): set CHPW_RUNTIME_MUTANTS=1")
	}
	const defn = `authGroup := router.Group("/api/v1/auth")`
	cases := []struct {
		name  string
		test  string
		edits map[string][][2]string
	}{
		{"RA3_UseEscapedPath_set_in_buildRouter", "TestGinEngineFieldsClassified",
			map[string][][2]string{"main_router.go": {{defn, "router.UseEscapedPath = true\n\t" + defn}}}},
		{"RA4_verifier_ignores_UseEscapedPath", "TestVerifyAuthRouteTable_RealEngines",
			map[string][][2]string{"main_router.go": {{"e.UseRawPath || e.UseEscapedPath ||", "e.UseRawPath ||"}}}},
		{"RM2_drop_ToLower_in_partial_param_branch", "TestPatternReachesAuthPaths_AgreesWithOracleOnCorpus",
			map[string][][2]string{"main_router.go": {{"strings.HasPrefix(want[j], strings.ToLower(s[:w]))", "strings.HasPrefix(want[j], s[:w])"}}}},
		{"R5a_empty_param_skip_removed_corpus", "TestPatternReachesAuthPaths_AgreesWithOracleOnCorpus",
			map[string][][2]string{"main_router.go": {{"return match(i+1, j) || match(i+1, j+1) // empty value, or one real segment", "return match(i+1, j+1)"}}}},
		{"R5b_empty_param_skip_removed_class", "TestPatternReachesAuthPaths_EmptyParamShiftClass",
			map[string][][2]string{"main_router.go": {{"return match(i+1, j) || match(i+1, j+1) // empty value, or one real segment", "return match(i+1, j+1)"}}}},
		{"R5c_empty_param_skip_removed_verifier_X1", "TestVerifyAuthRouteTable_RealEngines",
			map[string][][2]string{"main_router.go": {{"return match(i+1, j) || match(i+1, j+1) // empty value, or one real segment", "return match(i+1, j+1)"}}}},
		{"R5d_X1_route_in_buildRouter", "TestGinEngineFieldsClassified",
			map[string][][2]string{"main_router.go": {{defn, defn + "\n\trouter.GET(\"/:p/api/v1/auth/me\", func(c *gin.Context) {})"}}}},
		{"RM2b_caught_by_verifier_case_FC", "TestVerifyAuthRouteTable_RealEngines",
			map[string][][2]string{"main_router.go": {{"strings.HasPrefix(want[j], strings.ToLower(s[:w]))", "strings.HasPrefix(want[j], s[:w])"}}}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			_ = runtimeMutantRun(t, tc.edits, tc.test)
			t.Logf("KILLED by %s", tc.test)
		})
	}
}

// Round-4 review R4-1: the empty-parameter alignment class, pattern by pattern (also
// the shapes gin refuses on the full engine, which the engine-level test counts as a
// gin kill and so does not exercise the verifier on).
func TestPatternReachesAuthPaths_EmptyParamShiftClass(t *testing.T) {
	for _, p := range []string{
		"/:p/api/v1/auth/me", "/:p/api/v1/auth/*rest", "/api/:p/v1/auth/me", "/api/v1/:p/auth/me",
		"/:p/:q/api/v1/auth/me", "/:p/api/:q/auth/me", "/*rest", "/:p/*rest", "/api/v1/:p/me",
		"/:p/v1/auth/me", "/:a/:b/:c/api/v1/auth", "/api/:p/:q/v1/auth/x",
	} {
		require.True(t, patternReachesAuthPaths(p), "pattern %q must be treated as reaching /api/v1/auth", p)
	}
	for _, p := range []string{"/:a/health", "/api/v2/:x/auth", "/api/:v/authors", "/api/v1/files/:id/auth", "/:p/api/v2/auth", "/:p"} {
		require.False(t, patternReachesAuthPaths(p), "pattern %q must not be treated as reaching /api/v1/auth", p)
	}
}
