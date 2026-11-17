package main

// ADMD1F1-S1 review round 3 (STRUCTURAL, §11.4.276(E)): the scanner is fail-closed for the whole class "a group value reaches a sink the
// scanner does not model". Group-carrying go/ast kinds enumerated: call argument, composite-literal element, assignment to a
// non-name target (index / star), return value, channel send, method value (selector not called), go/defer (their CallExpr), struct
// field (selector binding), map/slice element (composite + index assignment). Each is pinned here (RED before the rule, GREEN after).

import (
	"testing"

	"github.com/stretchr/testify/require"
)

const escImp = "package x\nimport (\"github.com/gin-gonic/gin\"; ext \"example.com/ext\")\ntype mw struct{}\nfunc (mw) RequireAdmin() gin.HandlerFunc { return nil }\nfunc h(c *gin.Context) {}\n"

func TestADMD1F1S1R3_GroupEscapeIsReported(t *testing.T) {
	cases := map[string]string{
		"N7 call to package function outside scanned set":      escImp + "func Run() { r := gin.New(); ext.Mount(r.Group(\"/api\")) }\n",
		"N7b call to interface method outside scanned set":     fcHeader + "type X interface{ Mount(*gin.RouterGroup) }\nfunc Run(x X) { r := gin.New(); x.Mount(r.Group(\"/api\")) }\n",
		"N7c resolved group var passed out":                    escImp + "func Run() { r := gin.New(); g := r.Group(\"/api\"); ext.Mount(g) }\n",
		"N7d arity mismatch with the only scanned callee":      fcHeader + "func Mount(g *gin.RouterGroup, n int) { g.GET(\"/op\", h) }\nfunc Run() { r := gin.New(); Mount(r.Group(\"/api\")) }\n",
		"N7e group passed to a scanned func in non-group slot": fcHeader + "func Mount(g *gin.RouterGroup, v interface{}) { g.GET(\"/op\", h) }\nfunc Run() { r := gin.New(); Mount(r.Group(\"/api\"), r.Group(\"/b\")) }\n",
		"N7f defer of unscanned call":                          escImp + "func Run() { r := gin.New(); defer ext.Mount(r.Group(\"/api\")) }\n",
		"N7g go statement":                                     escImp + "func Run() { r := gin.New(); go ext.Mount(r.Group(\"/api\")) }\n",
		"N2 method value":                                      fcHeader + "func Run(m mw) { r := gin.New(); g := r.Group(\"/api\"); reg := g.GET; reg(\"/op\", m.RequireAdmin(), h) }\n",
		"N2b method value passed as argument":                  escImp + "func Run() { r := gin.New(); g := r.Group(\"/api\"); ext.Hook(g.POST) }\n",
		"N2c Group method value":                               fcHeader + "func Run() { r := gin.New(); mk := r.Group; g := mk(\"/zz\"); g.GET(\"/op\", h) }\n",
		"E1 composite literal element":                         fcHeader + "type S struct{ G *gin.RouterGroup }\nfunc Run() { r := gin.New(); s := S{G: r.Group(\"/a\")}; _ = s }\n",
		"E2 slice literal element":                             fcHeader + "func Run() { r := gin.New(); gs := []*gin.RouterGroup{r.Group(\"/a\")}; _ = gs }\n",
		"E3 map literal element":                               fcHeader + "func Run() { r := gin.New(); gs := map[string]*gin.RouterGroup{\"a\": r.Group(\"/a\")}; _ = gs }\n",
		"E4 index assignment":                                  fcHeader + "func Run() { r := gin.New(); gs := map[string]*gin.RouterGroup{}; gs[\"a\"] = r.Group(\"/a\") }\n",
		"E5 return value":                                      fcHeader + "func mk(r *gin.Engine) *gin.RouterGroup { return r.Group(\"/a\") }\nfunc Run() { r := gin.New(); mk(r).GET(\"/op\", h) }\n",
		"E6 channel send":                                      fcHeader + "func Run() { r := gin.New(); ch := make(chan *gin.RouterGroup, 1); ch <- r.Group(\"/a\") }\n",
		"E7 append":                                            fcHeader + "func Run() { r := gin.New(); var gs []*gin.RouterGroup; gs = append(gs, r.Group(\"/a\")); _ = gs }\n",
		"E8 star/address assignment":                           fcHeader + "func Run(p *interface{}) { r := gin.New(); *p = r.Group(\"/a\") }\n",
		"E9 group in a non-Handler key of a Server literal":    escImp + "func Run() { r := gin.New(); s := ext.Server{Other: r.Group(\"/a\")}; _ = s }\n",
		"E5b returned group in a walked (seeded) function":     fcHeader + "func mk(g *gin.RouterGroup) *gin.RouterGroup { return g }\nfunc Run() { r := gin.New(); _ = mk(r.Group(\"/a\")) }\n",
		"E10 group in a Handler key of a non-Server literal":   fcHeader + "type S struct{ Handler *gin.RouterGroup }\nfunc Run() { r := gin.New(); s := S{Handler: r.Group(\"/a\")}; _ = s }\n",
		"N17b Use chain then escape":                           escImp + "func Run(m mw) { r := gin.New(); ext.Mount(r.Group(\"/api\").Use(m.RequireAdmin())) }\n",
		"N19 conditional Use":                                  fcHeader + "func Run(m mw, on bool) { r := gin.New(); g := r.Group(\"/api\"); if on { g.Use(m.RequireAdmin()) }; g.GET(\"/op\", h) }\n",
		"N19b Use in a loop":                                   fcHeader + "func Run(m mw) { r := gin.New(); g := r.Group(\"/api\"); for i := 0; i < 1; i++ { g.Use(m.RequireAdmin()) }; g.GET(\"/op\", h) }\n",
		"N19c Use in a closure":                                fcHeader + "func Run(m mw) { r := gin.New(); g := r.Group(\"/api\"); func() { g.Use(m.RequireAdmin()) }(); g.GET(\"/op\", h) }\n",
		"N19d Use in a switch case":                            fcHeader + "func Run(m mw, k int) { r := gin.New(); g := r.Group(\"/api\"); switch k { case 1: g.Use(m.RequireAdmin()) }; g.GET(\"/op\", h) }\n",
	}
	for name, src := range cases {
		name, src := name, src
		t.Run(name, func(t *testing.T) {
			s := scanSrc(t, src)
			require.NotEmpty(t, s.unresolved, "silent drop: routes=%v", routeKeys(s))
		})
	}
}

func TestADMD1F1S1R3_UseChainGatesTheRoute(t *testing.T) {
	// I2 / N17: a gate passed to .Use inside a chain gates the route it returns.
	s := scanSrc(t, fcHeader+"func Run(m mw) { r := gin.New(); r.Group(\"/api\").Use(m.RequireAdmin()).GET(\"/op\", h) }\n")
	require.Empty(t, s.unresolved)
	require.Equal(t, []string{"GET /api/op gated"}, routeKeys(s))
	// a non-gate Use does not gate it
	s = scanSrc(t, fcHeader+"func cors() gin.HandlerFunc { return nil }\nfunc Run() { r := gin.New(); r.Group(\"/api\").Use(cors()).GET(\"/op\", h) }\n")
	require.Equal(t, []string{"GET /api/op open"}, routeKeys(s))
	// N18: a gated Use result bound to a name is gated
	s = scanSrc(t, fcHeader+"func Run(m mw) { r := gin.New(); x := r.Group(\"/api\").Use(m.RequireAdmin()); x.GET(\"/op\", h) }\n")
	require.Equal(t, []string{"GET /api/op gated"}, routeKeys(s))
}

func TestADMD1F1S1R3_RealNamesStillResolve(t *testing.T) {
	// receiver named like an imported package resolves when it is a known group (N11 must not be reported as a silent drop either way)
	src := "package x\nimport (\"github.com/gin-gonic/gin\"; \"example.com/router\")\ntype mw struct{}\nfunc (mw) RequireAdmin() gin.HandlerFunc { return nil }\nvar _ = router.X\nfunc Run(m mw) { router := gin.New(); router.GET(\"/op\", m.RequireAdmin(), func(c *gin.Context){}) }\n"
	s := scanSrc(t, src)
	require.Empty(t, s.unresolved)
	require.Equal(t, []string{"GET /op gated"}, routeKeys(s))
	// serving the finished engine (Handler: router in a *.Server literal) registers nothing and is not an escape
	s = scanSrc(t, "package x\nimport (\"github.com/gin-gonic/gin\"; \"net/http\")\nfunc Run() { r := gin.New(); s := http.Server{Handler: r}; _ = s }\n")
	require.Empty(t, s.unresolved)
	// an engine constructor returning its engine is not an escape
	s = scanSrc(t, fcHeader+"func newEngine() *gin.Engine { r := gin.New(); r.GET(\"/x\", h); return r }\nfunc Run() { e := newEngine(); e.GET(\"/y\", h) }\n")
	require.Empty(t, s.unresolved)
	require.ElementsMatch(t, []string{"GET /x open", "GET /y open"}, routeKeys(s))
	// a package function that merely shares a route-method name is still not a route
	s = scanSrc(t, "package x\nimport \"go.uber.org/zap\"\nfunc Run() { _ = zap.Any(\"k\", 1) }\n")
	require.Empty(t, s.unresolved)
	require.Empty(t, s.routes)
}

// the allow-list of callees that legitimately receive an engine/group without registering routes; each is pinned.
func TestADMD1F1S1R3_AllowListPinned(t *testing.T) {
	require.ElementsMatch(t, []string{"verifyAuthRouteTable", "applyTrustedProxies", "setTrustedProxies", "engineChain", "routeChainFuncNames"}, allowedGroupSinkNames())
	for _, n := range allowedGroupSinkNames() {
		s := scanSrc(t, fcHeader+"func "+n+"(e *gin.Engine) error { return nil }\nfunc Run() { r := gin.New(); _ = "+n+"(r) }\n")
		require.Empty(t, s.unresolved, n)
	}
	// the same shape under a name that is not on the list is reported
	s := scanSrc(t, fcHeader+"func Run() { r := gin.New(); _ = notAllowed(r) }\n")
	require.NotEmpty(t, s.unresolved)
}

// m2 (documented over-approximation, pinned): a call site binds every same-name, same-arity callee that has a group parameter, so a
// never-called homonym is walked with the seed. The result is a SUPERSET of the real routes (never a subset); phantom routes are
// possible but a route can not be hidden by it.
func TestADMD1F1S1R3_HomonymBindingIsASuperset(t *testing.T) {
	s := scanSrc(t, fcHeader+"type A struct{}\ntype B struct{}\nfunc (A) Mount(g *gin.RouterGroup) { g.GET(\"/a\", h) }\nfunc (B) Mount(g *gin.RouterGroup) { g.GET(\"/b\", h) }\nfunc Run(a A) { r := gin.New(); a.Mount(r.Group(\"/x\")) }\n")
	require.Empty(t, s.unresolved)
	require.ElementsMatch(t, []string{"GET /x/a open", "GET /x/b open"}, routeKeys(s), "documented superset: B.Mount is never called but is walked with the A.Mount call site seed")
}
