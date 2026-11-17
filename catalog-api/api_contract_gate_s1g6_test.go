package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	root_middleware "catalogizer/middleware"

	"github.com/stretchr/testify/require"
)

// ---- S1G6 (round 7): F1 receiver-qualified symbol, F2/F3 standing real-chain backstop for the scanner-blind class, F4 symbol-level stale check ----

func chainWith(t *testing.T, pre ...routeHandler) string {
	t.Helper()
	gateH := funcIdentity(root_middleware.NewJWTMiddleware("s1g6").RequireAdmin())
	plain := funcIdentity(probeH)
	sc := scanSrc(t, fcHeader+`func Run(m mw) { r := gin.New(); r.GET("/x", m.RequireAdmin(), h) }`+"\n")
	chain := append(append([]routeHandler{}, pre...), gateH, plain)
	return strings.Join(groundTruthViolations([]realRoute{{"GET", "/x", chain}}, sc, nil), "\n")
}

// S1G6-F1: an allow-list entry without a receiver ("Logger") must not match a METHOD of the same name ((*X).Logger) in an allowed file.
func TestS1G6F1_ReceiverlessAllowEntryDoesNotMatchAMethodOfTheSameName(t *testing.T) {
	mw := repoRel("internal", "middleware", "middleware.go")
	for _, name := range []string{
		"catalogizer/internal/middleware.(*X).Logger.func1",
		"catalogizer/internal/middleware.(*X).Logger",
		"catalogizer/internal/middleware.(*Y[...]).ErrorHandler.func2",
	} {
		v := chainWith(t, routeHandler{Name: name, File: mw})
		require.Contains(t, v, "route-level handler runs before the gate", name)
	}
	// golden-FALSE: the real shapes stay allowed (package-qualified function, closure suffixes, compiler-inlined prefixes, receiver entries)
	require.Empty(t, chainWith(t, routeHandler{Name: "catalogizer/internal/middleware.Logger.func1", File: mw}))
	require.Empty(t, chainWith(t, routeHandler{Name: "main.buildRouter.Logger.func1", File: mw}))
	require.Empty(t, chainWith(t, routeHandler{Name: "main.main.(*AuthMiddleware).RateLimitByUser.func2", File: repoRel("internal", "auth", "middleware.go")}))
	require.Empty(t, chainWith(t, routeHandler{Name: "catalogizer/middleware.(*JWTMiddleware).RequireAuth.func1", File: repoRel("middleware", "auth.go")}))
}

// S1G6-F4: the stale-symbol check is per symbol, not per file.
func TestS1G6F4_AllowListHasNoStaleSymbol(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs the server binary")
	}
	used := map[string]bool{}
	for _, r := range realBinaryRoutes(t) {
		for i, n := range r.chain {
			if !isAdminGateFunc(n) {
				continue
			}
			for _, p := range r.chain[:i] {
				if k, s, ok := preGateMatch(p); ok {
					used[k+"|"+s] = true
				}
			}
			break
		}
	}
	for k, syms := range preGateAllowedSymbols {
		for _, s := range syms {
			require.True(t, used[k+"|"+s], "allow-list symbol %s in %s is not used by any real gated chain (remove it)", s, k)
		}
	}
}

// S1G6-F2/F3: a STANDING real-chain test for the scanner-blind class. Each mutant is a copy of the real main.go (applied with go build
// -overlay, the checkout is never edited) that puts a handler in front of an admin gate in a shape the AST scanner does not model:
// gin.HandlerFunc(named), an aliased gin.WrapF, a group-level Use(gin.WrapF(..)) ahead of the group gate. The REAL engine's chain is
// the oracle: groundTruthViolations must report each one. Skipped under -short (it builds and runs the binary three times).
func TestS1G6F3_RealChainCatchesScannerBlindHandlersBeforeTheGate(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs the server binary")
	}
	srcPath, err := filepath.Abs("main.go")
	require.NoError(t, err)
	src, err := os.ReadFile(srcPath)
	require.NoError(t, err)
	g := `api.POST("/admin/covers/generate-thumbnails", jwtMiddleware.RequireAdmin(), `
	require.Contains(t, string(src), g)
	grp := "adminGroup.Use(jwtMiddleware.RequireAdmin())"
	require.Contains(t, string(src), grp)
	decl := "\nfunc s1g6BlindEarly(c *gin.Context) {}\n\nvar s1g6AliasWrapF = gin.WrapF\n"
	for name, mut := range map[string]string{
		"gin.HandlerFunc(named) before RequireAdmin":   strings.Replace(string(src), g, `api.POST("/admin/covers/generate-thumbnails", gin.HandlerFunc(s1g6BlindEarly), jwtMiddleware.RequireAdmin(), `, 1) + decl,
		"aliased WrapF before RequireAdmin":            strings.Replace(string(src), g, `api.POST("/admin/covers/generate-thumbnails", s1g6AliasWrapF(http.NotFound), jwtMiddleware.RequireAdmin(), `, 1) + decl,
		"group-level Use(WrapF) before the group gate": strings.Replace(string(src), grp, "adminGroup.Use(s1g6AliasWrapF(http.NotFound))\n\t"+grp, 1) + decl,
	} {
		routes, err := dumpRoutesFromBuild(t, mut)
		require.NoError(t, err, name)
		v := strings.Join(groundTruthViolations(routes, scanRouteSources(t, realRouteFiles(t)), nil), "\n")
		require.Contains(t, v, "route-level handler runs before the gate", name)
	}
	// control: the same builder on the UNEDITED source reports nothing
	routes, err := dumpRoutesFromBuild(t, string(src))
	require.NoError(t, err)
	require.Empty(t, groundTruthViolations(routes, scanRouteSources(t, realRouteFiles(t)), nil))
}
