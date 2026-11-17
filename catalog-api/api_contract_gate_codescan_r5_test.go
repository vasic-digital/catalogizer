package main

// ADMD1F1-S1 review round 5 (STRUCTURAL). The probe classes of the round-4 Opus review (A10, A20/21, A22/23, A25-A28, A31, A32,
// A38 and the wrapper / minor classes m1-m3) are pinned here as pairs: the scanner's source text and a Go twin that builds the same
// routes on a REAL gin engine. The one property every probe must satisfy is SOUNDNESS against the real route table:
//
//	the scanner reports the shape (routeScan.unresolved) OR its routes and gate states equal the real table.
//
// Probe names are reconstructed from the review summary (the reviewer's originals were not supplied; UNCONFIRMED mapping).

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	root_middleware "catalogizer/middleware"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func probeH(c *gin.Context) {}

type soundProbe struct {
	name  string
	src   string // appended to fcHeader
	build func(e *gin.Engine, gate gin.HandlerFunc)
}

func soundProbes() []soundProbe {
	return []soundProbe{
		{"A10 method expression with &r.RouterGroup", `func Run(m mw) { r := gin.New(); f := (*gin.RouterGroup).GET; f(&r.RouterGroup, "/x", h) }`,
			func(e *gin.Engine, gate gin.HandlerFunc) { e.GET("/x", probeH) }},
		{"A10b method expression Use then route", `func Run(m mw) { r := gin.New(); u := (*gin.RouterGroup).Use; u(&r.RouterGroup, m.RequireAdmin()); r.GET("/x", h) }`,
			func(e *gin.Engine, gate gin.HandlerFunc) { e.Use(gate); e.GET("/x", probeH) }},
		{"A20 sub-engine mounted through gin.WrapH(sub.Handler())", `func Run(m mw) { r := gin.New(); sub := gin.New(); sub.GET("/in", h); r.Any("/sub/*p", gin.WrapH(sub.Handler())) }`,
			func(e *gin.Engine, gate gin.HandlerFunc) {
				sub := gin.New()
				sub.GET("/in", probeH)
				e.Any("/sub/*p", gin.WrapH(sub.Handler()))
			}},
		{"A21 sub-engine mounted through gin.WrapF(sub.ServeHTTP)", `func Run(m mw) { r := gin.New(); sub := gin.New(); sub.GET("/in", h); r.GET("/sub/*p", gin.WrapF(sub.ServeHTTP)) }`,
			func(e *gin.Engine, gate gin.HandlerFunc) {
				sub := gin.New()
				sub.GET("/in", probeH)
				e.GET("/sub/*p", gin.WrapF(sub.ServeHTTP))
			}},
		{"A25 goto past the gate Use", `func Run(m mw) { r := gin.New(); g := r.Group("/a"); goto L; g.Use(m.RequireAdmin()); L: g.GET("/x", h) }`,
			func(e *gin.Engine, gate gin.HandlerFunc) { e.Group("/a").GET("/x", probeH) }},
		{"A26 gate after the handler", `func Run(m mw) { r := gin.New(); r.GET("/x", h, m.RequireAdmin()) }`,
			func(e *gin.Engine, gate gin.HandlerFunc) { e.GET("/x", probeH, gate) }},
		{"A26b gate after the handler on a group route", `func Run(m mw) { r := gin.New(); g := r.Group("/a"); g.POST("/x", h, m.RequireAdmin()) }`,
			func(e *gin.Engine, gate gin.HandlerFunc) { e.Group("/a").POST("/x", probeH, gate) }},
		{"A27 gate variable reassigned", `func Run(m mw) { r := gin.New(); gate := m.RequireAdmin(); gate = h; r.GET("/x", gate, h) }`,
			func(e *gin.Engine, gate gin.HandlerFunc) { g := gate; g = probeH; e.GET("/x", g, probeH) }},
		{"A28 gate variable shadowed in a block", `func Run(m mw) { r := gin.New(); gate := m.RequireAdmin(); if true { gate := h; r.GET("/x", gate, h) }; r.GET("/y", gate, h) }`,
			func(e *gin.Engine, gate gin.HandlerFunc) {
				func() { g := gin.HandlerFunc(probeH); e.GET("/x", g, probeH) }()
				e.GET("/y", gate, probeH)
			}},
		{"A31 local variable shadows a package constant", "const p = \"/glob\"\n" + `func dyn() string { return "" }
func Run(m mw) { r := gin.New(); p := dyn(); r.GET(p, h) }`,
			func(e *gin.Engine, gate gin.HandlerFunc) { e.GET("/real-dynamic", probeH) }},
		{"A31b x.Name() is not the package engine factory Name()", `type other struct{}
func (other) Mk() *gin.Engine { return gin.New() }
func Mk() *gin.Engine { return gin.New() }
func Run(o other) { r := o.Mk(); r.GET("/x", h) }`,
			func(e *gin.Engine, gate gin.HandlerFunc) { a := e.Group("/adm"); a.Use(gate); a.GET("/x", probeH) }},
		{"A32 registration in a closure that runs after a later Use", `func Run(m mw) { r := gin.New(); g := r.Group("/a"); reg := func() { g.GET("/x", h) }; g.Use(m.RequireAdmin()); reg() }`,
			func(e *gin.Engine, gate gin.HandlerFunc) { g := e.Group("/a"); g.Use(gate); g.GET("/x", probeH) }},
		{"A32b helper that calls Use on its group parameter", `func gateIt(g *gin.RouterGroup, m mw) { g.Use(m.RequireAdmin()) }
func Run(m mw) { r := gin.New(); g := r.Group("/a"); gateIt(g, m); g.GET("/x", h) }`,
			func(e *gin.Engine, gate gin.HandlerFunc) { g := e.Group("/a"); g.Use(gate); g.GET("/x", probeH) }},
		{"A32c aliased group pointer carries the Use", `func Run(m mw) { r := gin.New(); g := r.Group("/a"); k := g; k.Use(m.RequireAdmin()); g.GET("/x", h) }`,
			func(e *gin.Engine, gate gin.HandlerFunc) {
				g := e.Group("/a")
				k := g
				k.Use(gate)
				g.GET("/x", probeH)
			}},
		{"A32d variadic handler list", `func Run(m mw) { r := gin.New(); hs := []gin.HandlerFunc{m.RequireAdmin(), h}; r.GET("/x", hs...) }`,
			func(e *gin.Engine, gate gin.HandlerFunc) { e.GET("/x", []gin.HandlerFunc{gate, probeH}...) }},
		{"A32e deferred registration runs after a later Use", `func Run(m mw) { r := gin.New(); g := r.Group("/a"); defer g.GET("/x", h); g.Use(m.RequireAdmin()) }`,
			func(e *gin.Engine, gate gin.HandlerFunc) { g := e.Group("/a"); g.Use(gate); g.GET("/x", probeH) }},
		{"A38 Any is nine routes", `func Run(m mw) { r := gin.New(); r.Any("/x", h) }`,
			func(e *gin.Engine, gate gin.HandlerFunc) { e.Any("/x", probeH) }},
		{"A38b Match is the listed methods", `func Run(m mw) { r := gin.New(); r.Match([]string{"GET", "POST"}, "/y", h) }`,
			func(e *gin.Engine, gate gin.HandlerFunc) { e.Match([]string{"GET", "POST"}, "/y", probeH) }},
		{"A38c Any on a gated group", `func Run(m mw) { r := gin.New(); g := r.Group("/a"); g.Use(m.RequireAdmin()); g.Any("/x", h) }`,
			func(e *gin.Engine, gate gin.HandlerFunc) { g := e.Group("/a"); g.Use(gate); g.Any("/x", probeH) }},
		{"A29 a call that merely receives the gate is not a gate", `func ignore(g gin.HandlerFunc) gin.HandlerFunc { return h }
func Run(m mw) { r := gin.New(); r.GET("/x", ignore(m.RequireAdmin()), h) }`,
			func(e *gin.Engine, gate gin.HandlerFunc) { e.GET("/x", probeH, probeH) }},
	}
}

func TestADMD1F1S1R5_ScannerIsSoundAgainstTheRealRouteTable(t *testing.T) {
	gate := root_middleware.NewJWTMiddleware("round5-probe-secret").RequireAdmin()
	gin.SetMode(gin.TestMode)
	for _, p := range soundProbes() {
		p := p
		t.Run(p.name, func(t *testing.T) {
			e := gin.New()
			p.build(e, gate)
			real := realRoutesOf(t, e)
			s := scanSrc(t, fcHeader+p.src+"\n")
			if len(s.unresolved) > 0 {
				return // fail closed: the shape is reported
			}
			require.Empty(t, groundTruthViolations(real, s, nil), "the scanner is silent AND disagrees with the real route table: %v", routeKeys(s))
		})
	}
}

// Non-gin routers: the route table cannot see them, so the scanner must report each one that is not on the pinned allow-list.
func TestADMD1F1S1R5_NonGinRoutersAreReported(t *testing.T) {
	cases := map[string]string{
		"A22 http.NewServeMux mounted under a wildcard":   "package x\nimport (\"github.com/gin-gonic/gin\"; \"net/http\")\nfunc Run() { r := gin.New(); mux := http.NewServeMux(); mux.HandleFunc(\"/adm\", func(w http.ResponseWriter, q *http.Request) {}); r.GET(\"/m/*p\", gin.WrapH(mux)) }\n",
		"A23 http.HandleFunc on the default mux":          "package x\nimport \"net/http\"\nfunc Run() { http.HandleFunc(\"/adm\", func(w http.ResponseWriter, q *http.Request) {}) }\n",
		"A23b http.Handle on the default mux":             "package x\nimport \"net/http\"\nfunc Run(h http.Handler) { http.Handle(\"/adm\", h) }\n",
		"A23c a ServeMux handed in as a parameter":        "package x\nimport \"net/http\"\nfunc Mount(m *http.ServeMux, h http.Handler) { m.Handle(\"/adm\", h) }\n",
		"A23d http.ListenAndServe starts a second server": "package x\nimport \"net/http\"\nfunc Run(h http.Handler) { _ = http.ListenAndServe(\":9\", h) }\n",
		"A23e aliased net/http import":                    "package x\nimport nh \"net/http\"\nfunc Run() { nh.HandleFunc(\"/adm\", func(w nh.ResponseWriter, q *nh.Request) {}) }\n",
	}
	for name, src := range cases {
		src := src
		t.Run(name, func(t *testing.T) {
			require.NotEmpty(t, scanSrc(t, src).unresolved)
		})
	}
	t.Run("a package named http that is not net/http is not a router", func(t *testing.T) {
		require.Empty(t, scanSrc(t, "package x\nimport http \"example.com/fakehttp\"\nfunc Run() { http.HandleFunc(\"/x\", nil) }\n").unresolved)
	})
	_ = http.StatusOK
}

// Pins: the non-gin allow-list and the engine data fields are reasoned, exact, and apply only to their own function.
func TestADMD1F1S1R5_NonGinAllowListPinned(t *testing.T) {
	require.Equal(t, []string{
		"internal/monitoring/monitoring.go:StartPrometheusServer:NewServeMux",
		"tests/mocks/webdav_mock_server.go:Start:NewServeMux",
	}, nonGinAllowedKeys())
	for k, why := range nonGinAllowed {
		require.NotEmpty(t, why, k)
	}
	dir := t.TempDir()
	mk := func(rel, src string) []string {
		f := filepath.Join(dir, filepath.FromSlash(rel))
		require.NoError(t, os.MkdirAll(filepath.Dir(f), 0o755))
		require.NoError(t, os.WriteFile(f, []byte(src), 0o600))
		return []string{f}
	}
	body := func(fn string) string {
		return "package x\nimport \"net/http\"\nfunc " + fn + "() { _ = http.NewServeMux() }\n"
	}
	require.Empty(t, scanRouteSources(t, mk("internal/monitoring/monitoring.go", body("StartPrometheusServer"))).unresolved, "the pinned file+function")
	require.NotEmpty(t, scanRouteSources(t, mk("internal/monitoring/monitoring.go", body("StartOther"))).unresolved, "another function in the pinned file")
	require.NotEmpty(t, scanRouteSources(t, mk("internal/other/monitoring.go", body("StartPrometheusServer"))).unresolved, "the pinned function name in another file")
	// only the pinned net/http NAME is allowed inside the pinned function
	require.NotEmpty(t, scanRouteSources(t, mk("internal/monitoring/monitoring.go", "package x\nimport \"net/http\"\nfunc StartPrometheusServer() { http.HandleFunc(\"/x\", nil) }\n")).unresolved)
	require.NotEmpty(t, scanRouteSources(t, mk("internal/monitoring/monitoring.go", body("StartPrometheusServer")+"func Evil() { http.HandleFunc(\"/x\", nil) }\n")).unresolved)
}

func TestADMD1F1S1R5_EngineDataFieldsPinned(t *testing.T) {
	require.Equal(t, []string{"ContextWithFallback", "ForwardedByClientIP", "HandleMethodNotAllowed", "MaxMultipartMemory", "RedirectFixedPath", "RedirectTrailingSlash", "RemoteIPHeaders", "RemoveExtraSlash", "TrustedPlatform", "UnescapePathValues", "UseEscapedPath", "UseH2C", "UseRawPath"}, engineDataFieldNames())
	for _, f := range engineDataFieldNames() {
		require.Empty(t, scanSrc(t, fcHeader+"func Run() { r := gin.New(); _ = r."+f+" }\n").unresolved, f)
	}
	for _, f := range []string{"Handlers", "RouterGroup", "HTMLRender", "Frobnicate"} {
		require.NotEmpty(t, scanSrc(t, fcHeader+"func Run() { r := gin.New(); _ = r."+f+" }\n").unresolved, f)
	}
}

// Golden-FALSE (§11.4.201(1)): the forms the real tree uses, and their neighbours, must NOT be reported and must keep their gate state.
func TestADMD1F1S1R5_GoldenFalseShapes(t *testing.T) {
	ok := func(name, src string, want ...string) {
		t.Run(name, func(t *testing.T) {
			s := scanSrc(t, fcHeader+src+"\n")
			require.Empty(t, s.unresolved)
			require.ElementsMatch(t, want, routeKeys(s))
		})
	}
	ok("gate before the terminal handler", `func Run(m mw) { r := gin.New(); r.GET("/x", m.RequireAdmin(), h) }`, "GET /x gated")
	ok("gate between two handlers", `func Run(m mw) { r := gin.New(); r.GET("/x", h, m.RequireAdmin(), h) }`, "GET /x gated")
	ok("gate variable declared once, used twice", `func Run(m mw) { r := gin.New(); gate := m.RequireAdmin(); r.GET("/x", gate, h); r.POST("/y", gate, h) }`, "GET /x gated", "POST /y gated")
	ok("group Use then route, registrations in the same context", `func Run(m mw) { r := gin.New(); g := r.Group("/a"); g.Use(m.RequireAdmin()); g.GET("/x", h) }`, "GET /a/x gated")
	ok("group compared with nil", `func Run() { r := gin.New(); g := r.Group("/a"); if g == nil { return }; g.GET("/x", h) }`, "GET /a/x open")
	ok("registration in a closure on a group declared in the same closure", `func Run() { f := func() { r := gin.New(); g := r.Group("/a"); g.GET("/x", h) }; f() }`, "GET /a/x open")
	ok("Match with a literal method list", `func Run() { r := gin.New(); r.Match([]string{"GET", "PUT"}, "/y", h) }`, "GET /y open", "PUT /y open")
	ok("Server literal serving the finished engine", `func Run() { r := gin.New(); r.GET("/x", h); _ = r }`, "GET /x open")
	ok("a local string constant used as a path", `func Run() { r := gin.New(); const p = "/loc"; r.GET(p, h) }`, "GET /loc open")
	ok("pinned wrapper around a gate", `func metricsSkipIfScrape(g gin.HandlerFunc) gin.HandlerFunc { return g }
func Run(m mw) { r := gin.New(); r.GET("/metrics", metricsSkipIfScrape(m.RequireAdmin()), h) }`, "GET /metrics gated")
	ok("the http.Handler type is not a router", `func Mount(h http.Handler) { _ = h }`)
	// a package-level function named like a method elsewhere, called through the package, is still the engine factory
	t.Run("pkg.Factory() engine", func(t *testing.T) {
		s := scanSrc(t, "package x\nimport (\"github.com/gin-gonic/gin\"; \"net/http\")\nfunc NewEngine() *gin.Engine { return gin.New() }\nfunc h(c *gin.Context) {}\nfunc Run() { r := NewEngine(); r.GET(\"/x\", h); var _ http.Handler = r }\n")
		require.Empty(t, s.unresolved)
		require.ElementsMatch(t, []string{"GET /x open"}, routeKeys(s))
	})
}

// Gate identity (ground truth side): by function identity (symbol + defining file), never by the spelling in source.
func TestADMD1F1S1R5_GateIdentityIsByFunctionNotName(t *testing.T) {
	gate := root_middleware.NewJWTMiddleware("round5-identity").RequireAdmin()
	id := funcIdentity(gate)
	require.True(t, isAdminGateFunc(id), "the real closure %+v must be recognised (its symbol may carry the inlining caller's name)", id)
	require.False(t, isAdminGateFunc(funcIdentity(probeH)), "an ordinary handler")
	require.False(t, isAdminGateFunc(funcIdentity(root_middleware.NewJWTMiddleware("x").RequireAuth())), "RequireAuth is not the admin gate")
	fake := funcIdentity(fakeGateFactory())
	require.Contains(t, fake.Name, "RequireAdmin", "the decoy carries the spelling")
	require.False(t, isAdminGateFunc(fake), "a decoy spelled RequireAdmin but defined in another file is not the gate")
	require.False(t, isAdminGateFunc(routeHandler{Name: id.Name, File: "/elsewhere/other/auth.go", Line: id.Line}), "same symbol, other file")
	require.False(t, isAdminGateFunc(routeHandler{Name: "catalogizer/middleware.(*JWTMiddleware).RequireSelfOrAdmin.func1", File: id.File, Line: id.Line}), "same file, other function")
}

type fakeGate struct{}

func (fakeGate) RequireAdmin() gin.HandlerFunc { return func(c *gin.Context) { c.Next() } }

func fakeGateFactory() gin.HandlerFunc { return fakeGate{}.RequireAdmin() }
