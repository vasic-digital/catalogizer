package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	root_middleware "catalogizer/middleware"

	"github.com/stretchr/testify/require"
)

// ---- round 6b tests (R7, F-msg, F-allow) ----

func ginAt(name, file string) routeHandler {
	return routeHandler{Name: name, File: "/root/go/pkg/mod/github.com/gin-gonic/gin@v1.12.0/" + file}
}

func TestADMD1F1S1R7_ScannerReportsHandlerWrappingCallBeforeGate(t *testing.T) {
	for name, src := range map[string]string{
		"R7 gin.WrapF(func literal)":                   `func Run(m mw) { r := gin.New(); r.GET("/x", gin.WrapF(func(w http.ResponseWriter, q *http.Request) {}), m.RequireAdmin(), h) }`,
		"R7b gin.WrapH(handler value)":                 `func Run(m mw) { r := gin.New(); r.GET("/x", gin.WrapH(http.NotFoundHandler()), m.RequireAdmin(), h) }`,
		"R7c WrapF through a group":                    `func Run(m mw) { r := gin.New(); g := r.Group("/a"); g.POST("/x", gin.WrapF(nil), m.RequireAdmin(), h) }`,
		"R7d unknown wrapper given a function literal": `func Run(m mw) { r := gin.New(); r.GET("/x", wrap(func(c *gin.Context) {}), m.RequireAdmin(), h) }` + "\nfunc wrap(f gin.HandlerFunc) gin.HandlerFunc { return f }",
	} {
		s := scanSrc(t, fcHeader+src+"\n")
		require.NotEmpty(t, s.unresolved, name)
		require.Contains(t, strings.Join(s.unresolved, "\n"), "handler-wrapping call before the gate", name)
	}
	// golden-FALSE: a plain middleware factory call (no wrapped handler) before the gate stays legal
	s := scanSrc(t, fcHeader+`func Run(m mw) { r := gin.New(); r.GET("/x", mk(), m.RequireAdmin(), h) }`+"\nfunc mk() gin.HandlerFunc { return nil }\n")
	require.Empty(t, s.unresolved)
	s = scanSrc(t, fcHeader+`func Run(m mw) { r := gin.New(); r.GET("/x", mk(100), m.RequireAdmin(), h) }`+"\nfunc mk(n int) gin.HandlerFunc { return nil }\n")
	require.Empty(t, s.unresolved, "a factory call with a constant argument is not a wrapped handler")
}

func TestADMD1F1S1R7_RealSourceWrapFBeforeRequireAdminIsReported(t *testing.T) {
	src, err := os.ReadFile("main.go")
	require.NoError(t, err)
	old := `api.POST("/admin/covers/generate-thumbnails", jwtMiddleware.RequireAdmin(), `
	require.Contains(t, string(src), old, "the real registration this probe edits")
	mut := strings.Replace(string(src), old, `api.POST("/admin/covers/generate-thumbnails", gin.WrapF(func(w http.ResponseWriter, r *http.Request) {}), jwtMiddleware.RequireAdmin(), `, 1)
	tmp := filepath.Join(t.TempDir(), "main.go")
	require.NoError(t, os.WriteFile(tmp, []byte(mut), 0o600))
	var files []string
	for _, f := range realRouteFiles(t) {
		if f == "main.go" || filepath.Base(f) == "main.go" && filepath.Dir(f) == "." {
			f = tmp
		}
		files = append(files, f)
	}
	s := scanRouteSources(t, files)
	require.Contains(t, strings.Join(s.unresolved, "\n"), "handler-wrapping call before the gate", "WrapF in front of RequireAdmin in the real main.go must be reported")
	// control: the unedited real source is clean
	require.Empty(t, scanRouteSources(t, realRouteFiles(t)).unresolved)
}

func TestADMD1F1S1R7_RealChainWrapAndTerminalHandlersAreReported(t *testing.T) {
	gateH := funcIdentity(root_middleware.NewJWTMiddleware("r7").RequireAdmin())
	plain := funcIdentity(probeH)
	sc := scanSrc(t, fcHeader+`func Run(m mw) { r := gin.New(); r.GET("/x", m.RequireAdmin(), h) }`+"\n")
	run := func(pre ...routeHandler) string {
		chain := append(append([]routeHandler{}, pre...), gateH, plain)
		return strings.Join(groundTruthViolations([]realRoute{{"GET", "/x", chain}}, sc, nil), "\n")
	}
	// R7: gin.WrapF / gin.WrapH closures live in gin's own utils.go, which the old file-wildcard accepted
	require.Contains(t, run(ginAt("github.com/gin-gonic/gin.WrapF.func1", "utils.go")), "route-level handler runs before the gate")
	require.Contains(t, run(ginAt("github.com/gin-gonic/gin.WrapH.func1", "utils.go")), "route-level handler runs before the gate")
	// F-allow: MintWSTicket is a terminal handler that lives in an allowed file
	mint := routeHandler{Name: "catalogizer/middleware.(*JWTMiddleware).MintWSTicket-fm", File: repoRel("middleware", "auth.go")}
	require.Contains(t, run(mint), "route-level handler runs before the gate")
	mintClosure := routeHandler{Name: "main.main.(*JWTMiddleware).MintWSTicket.func3", File: repoRel("middleware", "auth.go")}
	require.Contains(t, run(mintClosure), "route-level handler runs before the gate")
	// a closure with a made-up name in an allowed file
	require.Contains(t, run(routeHandler{Name: "main.main.respondEarly.func1", File: repoRel("middleware", "request.go")}), "route-level handler runs before the gate")
	// golden-FALSE: the real pre-gate middleware (gin's own Logger, the pinned RequireAuth closure, compiler-inlined prefixes) is fine
	require.Empty(t, run(ginAt("github.com/gin-gonic/gin.LoggerWithConfig.func1", "logger.go")))
	require.Empty(t, run(routeHandler{Name: "main.buildRouter.(*JWTMiddleware).RequireAuth.func24", File: repoRel("middleware", "auth.go")}))
	require.Empty(t, run(routeHandler{Name: "main.main.(*JWTMiddleware).RequireAuth.func54", File: repoRel("middleware", "auth.go")}))
	require.Empty(t, run(routeHandler{Name: "catalogizer/middleware.CORS.func1", File: repoRel("middleware", "request.go")}))
}

func TestADMD1F1S1R7_ViolationMessageNamesTheRemedy(t *testing.T) {
	gateH := funcIdentity(root_middleware.NewJWTMiddleware("r7").RequireAdmin())
	plain := funcIdentity(probeH)
	sc := scanSrc(t, fcHeader+`func Run(m mw) { r := gin.New(); r.GET("/x", m.RequireAdmin(), h) }`+"\n")
	v := strings.Join(groundTruthViolations([]realRoute{{"GET", "/x", []routeHandler{plain, gateH, plain}}}, sc, nil), "\n")
	require.Contains(t, v, "if this is new router/group middleware, add its file and symbol to preGateAllowedSymbols")
}

func TestADMD1F1S1R7_PreGateAllowListPinsExactlyTheRealChains(t *testing.T) {
	// every real pre-gate handler is allowed (no false positive on the real tree) and the allow-list has no entry the real tree never uses
	used := map[string]bool{}
	for _, r := range realBinaryRoutes(t) {
		for i, n := range r.chain {
			if !isAdminGateFunc(n) {
				continue
			}
			for _, p := range r.chain[:i] {
				require.True(t, preGateAllowed(p), "real pre-gate handler not allow-listed: %s in %s", p.Name, p.File)
				used[preGateKey(p.File)] = true
			}
			break
		}
	}
	for k := range preGateAllowedSymbols {
		require.True(t, used[k], "allow-list file %s is not used by any real gated chain (remove it)", k)
	}
}
