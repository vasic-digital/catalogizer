package main

// ADMD1F1-S1 review round 4 (STRUCTURAL, §11.4.276(E)). Re-derived from the ground truth: (1) the set of ALL methods on
// gin.RouterGroup / gin.Engine and what each does to the route table, (2) every function, not only the ones that look like route
// registrars, (3) the real type of every parameter. Probe names P11..P27 / Q1..Q4 / U1 / U2 are the review's probes, reconstructed here
// as small sources (the reviewer's originals were not supplied; each test states the shape it pins).

import (
	"testing"

	"github.com/stretchr/testify/require"
)

const r4Imp = "package x\nimport (\"github.com/gin-gonic/gin\"; ext \"example.com/ext\")\ntype mw struct{}\nfunc (mw) RequireAdmin() gin.HandlerFunc { return nil }\nfunc h(c *gin.Context) {}\n"

// I1: Static / StaticFile / StaticFS / StaticFileFS register GET+HEAD; gated by the group's gate.
func TestADMD1F1S1R4_StaticFamilyRegistersGetAndHead(t *testing.T) {
	t.Run("P11 Static on a gated group", func(t *testing.T) {
		s := scanSrc(t, fcHeader+"func Run(m mw) { r := gin.New(); g := r.Group(\"/files\"); g.Use(m.RequireAdmin()); g.Static(\"/dl\", \"./data\") }\n")
		require.Empty(t, s.unresolved)
		require.ElementsMatch(t, []string{"GET /files/dl/*filepath gated", "HEAD /files/dl/*filepath gated"}, routeKeys(s))
	})
	t.Run("P13 StaticFile on the engine is open", func(t *testing.T) {
		s := scanSrc(t, fcHeader+"func Run() { r := gin.New(); r.StaticFile(\"/favicon.ico\", \"./f.ico\") }\n")
		require.Empty(t, s.unresolved)
		require.ElementsMatch(t, []string{"GET /favicon.ico open", "HEAD /favicon.ico open"}, routeKeys(s))
	})
	t.Run("P14 StaticFS and StaticFileFS", func(t *testing.T) {
		s := scanSrc(t, fcHeader+"func Run(fs http.FileSystem) { r := gin.New(); r.StaticFS(\"/a\", fs); r.StaticFileFS(\"/b\", \"i.html\", fs) }\n")
		require.Empty(t, s.unresolved)
		require.ElementsMatch(t, []string{"GET /a/*filepath open", "HEAD /a/*filepath open", "GET /b open", "HEAD /b open"}, routeKeys(s))
	})
	t.Run("P15 Static under an unresolved prefix is reported, emits no route", func(t *testing.T) {
		s := scanSrc(t, fcHeader+"func Run(p string) { r := gin.New(); r.Group(dyn()).Static(\"/dl\", \"d\") }\n")
		require.NotEmpty(t, s.unresolved)
		require.Empty(t, s.routes)
	})
	t.Run("Static method value is reported", func(t *testing.T) {
		s := scanSrc(t, fcHeader+"func Run() { r := gin.New(); f := r.Static; f(\"/x\", \"d\") }\n")
		require.NotEmpty(t, s.unresolved)
	})
}

// I1 class-wide: a method on a resolved group/engine that is neither modelled nor pinned inert is reported.
func TestADMD1F1S1R4_UnmodelledMethodOnGroupIsReported(t *testing.T) {
	s := scanSrc(t, fcHeader+"func Run() { r := gin.New(); g := r.Group(\"/a\"); g.RegisterSomethingNew(\"/op\", h) }\n")
	require.NotEmpty(t, s.unresolved)
	for _, n := range inertGroupMethodNames() { // pinned inert methods are not reported
		s = scanSrc(t, fcHeader+"func Run() { r := gin.New(); r."+n+"(h) }\n")
		require.Empty(t, s.unresolved, n)
	}
	// a same-named method on a value that is not a group is not reported
	s = scanSrc(t, fcHeader+"type T struct{}\nfunc (T) Other(a int) {}\nfunc Run(t T) { t.Other(1) }\n")
	require.Empty(t, s.unresolved)
}

func TestADMD1F1S1R4_InertMethodListPinned(t *testing.T) {
	require.ElementsMatch(t, []string{"BasePath", "Delims", "LoadHTMLFS", "LoadHTMLFiles", "LoadHTMLGlob", "NoMethod", "NoRoute", "Routes", "Run", "RunFd", "RunListener", "RunTLS", "RunUnix", "SetFuncMap", "SetHTMLTemplate", "SetTrustedProxies"}, inertGroupMethodNames())
	for _, n := range inertGroupMethodNames() {
		require.False(t, mountMethods[n], n+" is a mount method (reported), never inert")
		require.False(t, routeMethods[n] || n == "Use" || n == "Group" || func() bool { _, ok := staticRoutes[n]; return ok }(), n+" must not be both inert and modelled")
	}
}

// I2: every function is walked; group parameters are classified from the real type expression.
func TestADMD1F1S1R4_EveryFunctionIsWalked(t *testing.T) {
	t.Run("Q1 func-typed parameter: group handed to it escapes", func(t *testing.T) {
		s := scanSrc(t, fcHeader+"func Run(reg func(*gin.RouterGroup)) { r := gin.New(); reg(r.Group(\"/a\")) }\n")
		require.NotEmpty(t, s.unresolved)
	})
	t.Run("Q2 slice-of-group parameter: registration through an element is reported", func(t *testing.T) {
		s := scanSrc(t, fcHeader+"func Mount(gs []*gin.RouterGroup) { for _, g := range gs { g.GET(\"/x\", h) }; gs[0].POST(\"/y\", h) }\n")
		require.NotEmpty(t, s.unresolved)
	})
	t.Run("Q2b map-of-group parameter", func(t *testing.T) {
		s := scanSrc(t, fcHeader+"func Mount(gs map[string]*gin.RouterGroup) { gs[\"a\"].GET(\"/x\", h) }\n")
		require.NotEmpty(t, s.unresolved)
	})
	t.Run("Q3 alias and named group types resolve (no false report, correct prefix)", func(t *testing.T) {
		s := scanSrc(t, fcHeader+"type G = *gin.RouterGroup\ntype R gin.IRoutes\nfunc Mount(g G) { g.GET(\"/x\", h) }\nfunc Mount2(g R) { g.GET(\"/y\", h) }\nfunc Run() { r := gin.New(); Mount(r.Group(\"/a\")); Mount2(r.Group(\"/b\")) }\n")
		require.Empty(t, s.unresolved)
		require.ElementsMatch(t, []string{"GET /a/x open", "GET /b/y open"}, routeKeys(s))
	})
	t.Run("Q3b aliased gin import", func(t *testing.T) {
		s := scanSrc(t, "package x\nimport g2 \"github.com/gin-gonic/gin\"\nfunc h(c *g2.Context) {}\nfunc Mount(g *g2.RouterGroup) { g.GET(\"/x\", h) }\nfunc Run() { r := g2.New(); Mount(r.Group(\"/a\")) }\n")
		require.Empty(t, s.unresolved)
		require.ElementsMatch(t, []string{"GET /a/x open"}, routeKeys(s))
	})
	t.Run("Q4 a foreign type merely NAMED RouterGroup is not a group (m3)", func(t *testing.T) {
		s := scanSrc(t, "package x\nimport other \"example.com/other\"\nfunc Mount(g *other.RouterGroup, n int) { _ = g }\nfunc Run(g *other.RouterGroup) { Mount(g, 1); g.Frobnicate() }\n")
		require.Empty(t, s.unresolved)
		require.Empty(t, s.routes)
	})
	t.Run("Q4b func-typed root parameter does not become an unseeded group function (m3)", func(t *testing.T) {
		s := scanSrc(t, fcHeader+"func Run(reg func(*gin.RouterGroup)) { _ = reg }\n")
		require.Empty(t, s.unresolved)
	})
	t.Run("unseeded group-parameter function: escape reported even with no route in it", func(t *testing.T) {
		s := scanSrc(t, fcHeader+"func Orphan(g *gin.RouterGroup) *gin.RouterGroup { return g }\n")
		require.NotEmpty(t, s.unresolved)
	})
	t.Run("unseeded group-parameter function: unknown prefix reported for every route", func(t *testing.T) {
		s := scanSrc(t, fcHeader+"func Orphan(g *gin.RouterGroup) { g.Static(\"/d\", \"x\"); g.GET(\"/y\", h) }\n")
		require.NotEmpty(t, s.unresolved)
		require.Empty(t, s.routes)
	})
	t.Run("receiver that is a group type is unknown", func(t *testing.T) {
		s := scanSrc(t, fcHeader+"type G gin.RouterGroup\nfunc (g *G) Add() { g.GET(\"/x\", h) }\n")
		require.NotEmpty(t, s.unresolved)
	})
}

// I3 (mutant M9: `case base.unknown:` -> `case false && base.unknown:`)
func TestADMD1F1S1R4_UnknownPrefixIsReported(t *testing.T) {
	t.Run("U1 Group with a non-constant path then a route", func(t *testing.T) {
		s := scanSrc(t, fcHeader+"func Run() { r := gin.New(); g := r.Group(dyn()); g.POST(\"/x\", h) }\n")
		require.Contains(t, joinAll(s), "full path is unresolved")
		require.Empty(t, s.routes)
	})
	t.Run("U2 Group(p) with a parameter nobody seeds, inside a walked function", func(t *testing.T) {
		s := scanSrc(t, fcHeader+"func Run(p string) { r := gin.New(); r.Group(p).POST(\"/x\", h) }\n")
		require.Contains(t, joinAll(s), "full path is unresolved")
		require.Empty(t, s.routes)
	})
}

func joinAll(s *routeScan) string {
	out := ""
	for _, u := range s.unresolved {
		out += u + "\n"
	}
	return out
}

// m1: only net/http and quic-go/http3 Server literals serve a finished engine without registering anything.
func TestADMD1F1S1R4_ServerExemptionByImportPath(t *testing.T) {
	t.Run("P26 foreign Server{Handler: g} is reported", func(t *testing.T) {
		s := scanSrc(t, r4Imp+"func Run() { r := gin.New(); g := r.Group(\"/a\"); s := ext.Server{Handler: g}; _ = s }\n")
		require.NotEmpty(t, s.unresolved)
	})
	t.Run("http3 Server is exempt", func(t *testing.T) {
		s := scanSrc(t, "package x\nimport (\"github.com/gin-gonic/gin\"; \"github.com/quic-go/quic-go/http3\")\nfunc Run() { r := gin.New(); s := &http3.Server{Handler: r}; _ = s }\n")
		require.Empty(t, s.unresolved)
	})
	t.Run("a net/http type that is not Server is not exempt", func(t *testing.T) {
		s := scanSrc(t, "package x\nimport (\"github.com/gin-gonic/gin\"; \"net/http\")\nfunc Run() { r := gin.New(); s := http.Client{Handler: r}; _ = s }\n")
		require.NotEmpty(t, s.unresolved)
	})
	t.Run("local type named Server with a net/http-looking spelling is not exempt", func(t *testing.T) {
		s := scanSrc(t, "package x\nimport (\"github.com/gin-gonic/gin\"; http \"example.com/fakehttp\")\nfunc Run() { r := gin.New(); s := http.Server{Handler: r}; _ = s }\n")
		require.NotEmpty(t, s.unresolved)
	})
}

// m2: the sink allow-list applies to an unqualified call of a scanned function only.
func TestADMD1F1S1R4_AllowListIsUnqualifiedAndScannedOnly(t *testing.T) {
	t.Run("P27 qualified foreign verifyAuthRouteTable / engineChain", func(t *testing.T) {
		s := scanSrc(t, r4Imp+"func Run() { r := gin.New(); g := r.Group(\"/a\"); ext.verifyAuthRouteTable(g); ext.engineChain(r, \"x\") }\n")
		require.NotEmpty(t, s.unresolved)
	})
	t.Run("unqualified call to a name that is NOT declared in the scanned set", func(t *testing.T) {
		s := scanSrc(t, fcHeader+"func Run() { r := gin.New(); verifyAuthRouteTable(r) }\n")
		require.NotEmpty(t, s.unresolved)
	})
	t.Run("unqualified call to the scanned declaration is allowed", func(t *testing.T) {
		s := scanSrc(t, fcHeader+"func verifyAuthRouteTable(e *gin.Engine) error { return nil }\nfunc Run() { r := gin.New(); _ = verifyAuthRouteTable(r) }\n")
		require.Empty(t, s.unresolved)
	})
}
