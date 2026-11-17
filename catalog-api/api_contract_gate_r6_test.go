package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	root_middleware "catalogizer/middleware"

	"github.com/stretchr/testify/require"
)

// routeSetViolations (S1G5-F3): the REAL route set (every method+path of the real engine) must equal the route set of
// docs/ROUTE_AUTHZ_TABLE.md in both directions, after parameter normalisation.
func routeSetViolations(real []realRoute, rows []tableRow) []string {
	realSet := map[string]bool{}
	for _, r := range real {
		realSet[normRoute(r.method, r.path)] = true
	}
	tabSet := map[string]bool{}
	for _, r := range rows {
		tabSet[normRoute(r.method, r.path)] = true
	}
	var out []string
	for k := range realSet {
		if !tabSet[k] {
			out = append(out, "real route has no authorization table row: "+k)
		}
	}
	for k := range tabSet {
		if !realSet[k] {
			out = append(out, "authorization table row has no real route: "+k)
		}
	}
	sort.Strings(out)
	return out
}

// ---- round 6 tests (S1G5-F1..F4) ----

func TestADMD1F1S1R6_RouteSetEqualsTableBothDirections(t *testing.T) {
	gateH := funcIdentity(root_middleware.NewJWTMiddleware("r6").RequireAdmin())
	plain := funcIdentity(probeH)
	sc := scanSrc(t, fcHeader+`func Run(m mw) { r := gin.New(); r.GET("/x", m.RequireAdmin(), h); r.GET("/y", h) }`+"\n")
	real := []realRoute{{"GET", "/x", []routeHandler{gateH, plain}}, {"GET", "/y", []routeHandler{plain}}}
	rows := []tableRow{{"GET", "/x", "RequireAdmin (route)"}, {"GET", "/y", "auth"}}
	require.Empty(t, groundTruthViolations(real, sc, rows), "faithful: equal sets")
	// a real route the table does not list (ungated, so the admin-only comparison never sees it)
	v := strings.Join(groundTruthViolations(real, sc, rows[:1]), "\n")
	require.Contains(t, v, "real route has no authorization table row: get /y")
	// a table row for a route that does not exist
	v = strings.Join(groundTruthViolations(real, sc, append(append([]tableRow{}, rows...), tableRow{"GET", "/ghost/:id", "auth"})), "\n")
	require.Contains(t, v, "authorization table row has no real route: get /ghost/{}")
	// parameter spelling is normalised
	real2 := []realRoute{{"GET", "/p/:id", []routeHandler{plain}}}
	sc2 := scanSrc(t, fcHeader+`func Run() { r := gin.New(); r.GET("/p/:id", h) }`+"\n")
	require.Empty(t, groundTruthViolations(real2, sc2, []tableRow{{"GET", "/p/{id}", "auth"}}))
}

func TestADMD1F1S1R6_RouteLevelHandlerBeforeGateIsReported(t *testing.T) {
	gateH := funcIdentity(root_middleware.NewJWTMiddleware("r6").RequireAdmin())
	plain := funcIdentity(probeH)
	authMw := routeHandler{Name: "main.main.(*JWTMiddleware).RequireAuth.func54", File: filepath.Join(repoRel("middleware", "auth.go"))}
	// real chain side
	sc := scanSrc(t, fcHeader+`func Run(m mw) { r := gin.New(); r.GET("/x", m.RequireAdmin(), h) }`+"\n")
	bad := []realRoute{{"GET", "/x", []routeHandler{plain, gateH, plain}}}
	require.Contains(t, strings.Join(groundTruthViolations(bad, sc, nil), "\n"), "route-level handler runs before the gate")
	good := []realRoute{{"GET", "/x", []routeHandler{authMw, gateH, plain}}}
	require.Empty(t, groundTruthViolations(good, sc, nil), "pinned middleware before the gate is fine")
	// scanner side
	s1 := scanSrc(t, fcHeader+`func Run(m mw) { r := gin.New(); r.GET("/x", h, m.RequireAdmin(), h) }`+"\n")
	require.NotEmpty(t, s1.unresolved, "a handler value before the gate is reported")
	s2 := scanSrc(t, fcHeader+`func Run(m mw) { r := gin.New(); r.GET("/x", func(c *gin.Context) {}, m.RequireAdmin(), h) }`+"\n")
	require.NotEmpty(t, s2.unresolved, "a literal handler before the gate is reported")
	s3 := scanSrc(t, fcHeader+`func Run(m mw) { r := gin.New(); r.GET("/x", mk(), m.RequireAdmin(), h) }`+"\nfunc mk() gin.HandlerFunc { return nil }\n")
	require.Empty(t, s3.unresolved, "golden-FALSE: a middleware factory call before the gate is not a route-level handler")
	require.ElementsMatch(t, []string{"GET /x gated"}, routeKeys(s3))
}

func TestADMD1F1S1R6_GateIdentityComparesFullFilePath(t *testing.T) {
	id := funcIdentity(root_middleware.NewJWTMiddleware("r6").RequireAdmin())
	decoy := routeHandler{Name: id.Name, File: "/elsewhere/middleware/auth.go", Line: id.Line}
	require.False(t, isAdminGateFunc(decoy), "same symbol, same last two path elements, different directory")
	require.True(t, isAdminGateFunc(routeHandler{Name: id.Name, File: id.File, Line: id.Line}), "golden-TRUE: the real file")
	gateH := funcIdentity(root_middleware.NewJWTMiddleware("r6").RequireAdmin())
	plain := funcIdentity(probeH)
	sc := scanSrc(t, fcHeader+`func Run(m mw) { r := gin.New(); r.GET("/x", m.RequireAdmin(), h) }`+"\n")
	fakeChain := []realRoute{{"GET", "/x", []routeHandler{decoy, plain}}}
	require.Contains(t, strings.Join(groundTruthViolations(fakeChain, sc, nil), "\n"), "gate state differs")
	_ = gateH
}

func TestADMD1F1S1R6_ShadowedImportAliasIsNotAnEngineFactory(t *testing.T) {
	src := "package x\nimport (\"github.com/gin-gonic/gin\"; mk \"example.com/mk\")\nfunc h(c *gin.Context) {}\nfunc NewEngine() *gin.Engine { return gin.New() }\n" +
		"type local struct{}\nfunc (local) NewEngine() *gin.Engine { return nil }\n" +
		"func Run() { mk := local{}; r := mk.NewEngine(); r.GET(\"/x\", h) }\n"
	s := scanSrc(t, src)
	require.NotEmpty(t, s.unresolved, "the local mk shadows the import alias: mk.NewEngine() is a method, its result is not a known engine")
	require.Empty(t, s.routes)
	// golden-FALSE: the unshadowed package alias IS the factory
	src2 := "package x\nimport (\"github.com/gin-gonic/gin\"; mk \"example.com/mk\")\nfunc h(c *gin.Context) {}\nfunc NewEngine() *gin.Engine { return gin.New() }\n" +
		"func Run() { r := mk.NewEngine(); r.GET(\"/x\", h) }\n"
	s2 := scanSrc(t, src2)
	require.Empty(t, s2.unresolved)
	require.ElementsMatch(t, []string{"GET /x open"}, routeKeys(s2))
}

// repoRel builds the absolute path of a catalog-api file for this checkout.
func repoRel(parts ...string) string {
	wd, _ := os.Getwd()
	return filepath.Join(append([]string{wd}, parts...)...)
}

func TestADMD1F1S1R6_WrapperClosureIsBySymbolAndFile(t *testing.T) {
	require.True(t, isWrapperClosure(routeHandler{Name: "catalogizer.metricsSkipIfScrape.func1", File: "/x/catalog-api/main_metricsauth.go"}))
	require.False(t, isWrapperClosure(routeHandler{Name: "catalogizer.metricsSkipIfScrape.func1", File: "/x/catalog-api/other.go"}), "decoy name in another file")
	require.False(t, isWrapperClosure(routeHandler{Name: "catalogizer.notmetricsSkipIfScrapeX.func1", File: "/x/catalog-api/main_metricsauth.go"}), "name only contains the symbol")
}
