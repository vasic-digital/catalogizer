package main

// ADMD1F1-S1 round 5 (STRUCTURAL, §11.4.276(E)). Rounds 1-4 each added cases to an AST scanner that reasons about SOURCE TEXT, and each
// review found another source shape the scanner mis-modelled. Round 5 replaces the primary oracle: the gate state of a route is read
// from the REAL engine's route table (the ordered handler chain of every route, by runtime function identity), and the AST scanner
// is only a secondary check that must either agree with that table or report (fail closed).
//
// Ground truth, per route:
//
//	gated(route) = the chain holds a handler whose runtime function is the closure of
//	               (*catalogizer/middleware.JWTMiddleware).RequireAdmin AND that handler sits BEFORE the terminal (last) handler.
//
// The function is identified from the method expression (*JWTMiddleware).RequireAdmin through runtime.FuncForPC, never from the
// spelling "RequireAdmin" in source. Scope, stated: the table sees every route the gin engine serves. It does NOT see what a
// handler does after it runs (in-handler permission checks, a sub-engine or http.Handler mounted under a wildcard route), nor any
// non-gin server (net/http ServeMux); those are the scanner's job and it reports them (nonGinAllowed, mountMethods).

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	root_middleware "catalogizer/middleware"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type realRoute struct {
	method, path string
	chain        []routeHandler // runtime function identities, terminal handler last
}

// adminGateIdentity is the identity of (*JWTMiddleware).RequireAdmin itself, taken from the method expression (a function value,
// not source text). A handler is that gate's closure when its symbol is "<anything>.(*JWTMiddleware).RequireAdmin.funcN" (the prefix
// is the caller's name when the compiler inlined RequireAdmin) AND it was written in the same source file as the real function.
func adminGateIdentity() routeHandler {
	return funcIdentity((*root_middleware.JWTMiddleware).RequireAdmin)
}

var adminGateClosureRe = regexp.MustCompile(`\.\(\*JWTMiddleware\)\.RequireAdmin\.func\d+(\.\d+)*$`)

// pathTail returns the last two elements of a slash path ("middleware/auth.go").
func pathTail(p string) string {
	p = strings.ReplaceAll(p, "\\", "/")
	parts := strings.Split(p, "/")
	if len(parts) > 2 {
		parts = parts[len(parts)-2:]
	}
	return strings.Join(parts, "/")
}

func isAdminGateFunc(h routeHandler) bool {
	return adminGateClosureRe.MatchString(h.Name) && pathTail(h.File) == pathTail(adminGateIdentity().File)
}

// realGateState: gated = a gate handler sits before the terminal handler; gateAtOrAfterTerminal = a gate handler is the terminal
// handler (nothing runs after it that it could protect) which is the "gate after handler" mistake.
func realGateState(chain []routeHandler) (gated, gateAtOrAfterTerminal bool) {
	for i, n := range chain {
		if !isAdminGateFunc(n) {
			continue
		}
		if i < len(chain)-1 {
			gated = true
		} else {
			gateAtOrAfterTerminal = true
		}
	}
	return
}

func realRoutesOf(t *testing.T, e *gin.Engine) []realRoute {
	t.Helper()
	rc, err := routeChainFuncNames(e)
	require.NoError(t, err)
	var out []realRoute
	for _, r := range rc {
		out = append(out, realRoute{r.Method, r.Path, r.Chain})
	}
	return out
}

// ginAnyMethods is the method set gin.RouterGroup.Any registers.
var ginAnyMethods = []string{"GET", "POST", "PUT", "PATCH", "HEAD", "OPTIONS", "DELETE", "CONNECT", "TRACE"}

// groundTruthViolations compares the scanner's routes with the real route table, both directions, and the gate state of every
// route. A scanner route recorded more than once is gated only if every copy is. rows == nil skips the authorization-table check.
func groundTruthViolations(real []realRoute, scan *routeScan, rows []tableRow) []string {
	var out []string
	realBy := map[string]realRoute{}
	for _, r := range real {
		realBy[r.method+" "+r.path] = r
	}
	scanGated := map[string]bool{}
	for _, r := range scan.routes {
		k := strings.ToUpper(r.method) + " " + r.path
		if g, seen := scanGated[k]; seen {
			scanGated[k] = g && r.gated
		} else {
			scanGated[k] = r.gated
		}
	}
	for k, sg := range scanGated {
		r, ok := realBy[k]
		if !ok {
			out = append(out, "scanner route is not in the real route table: "+k)
			continue
		}
		rg, after := realGateState(r.chain)
		if after {
			out = append(out, "gate is the terminal handler (it runs after the handler it should protect, so it protects nothing): "+k)
		}
		if why, wrapped := wrappedGateRoutes[k]; wrapped && !rg {
			// The scanner sees RequireAdmin inside a pinned wrapper; at run time the chain holds the wrapper closures, not the gate
			// itself. Accept only when BOTH wrapper closures are really in the chain, before the terminal handler.
			n := 0
			for i := 0; i+1 < len(r.chain); i++ {
				if strings.Contains(r.chain[i].Name, wrappedGateSymbol) {
					n++
				}
			}
			if n >= 2 {
				rg = true
			}
			_ = why
		}
		if sg != rg {
			out = append(out, "gate state differs for "+k+": scanner gated="+boolStr(sg)+", real chain gated="+boolStr(rg))
		}
	}
	for k, r := range realBy {
		if _, ok := scanGated[k]; !ok {
			out = append(out, "real route is not seen by the scanner: "+k)
		}
		if _, after := realGateState(r.chain); after {
			out = append(out, "gate is the terminal handler (it runs after the handler it should protect, so it protects nothing): "+k)
		}
	}
	if rows != nil {
		adminTable := tableAdminRoutes(rows)
		realGated := map[string]bool{}
		for _, r := range real {
			if g, _ := realGateState(r.chain); g && !strings.HasPrefix(r.path, "/debug/") && r.path != "/metrics" {
				realGated[normRoute(r.method, r.path)] = true
			}
		}
		for k := range realGated {
			if !adminTable[k] {
				out = append(out, "real route is gated by RequireAdmin but the authorization table has no RequireAdmin row: "+k)
			}
		}
		for k := range adminTable {
			if !realGated[k] {
				out = append(out, "authorization table lists a RequireAdmin route that is not gated in the real chain: "+k)
			}
		}
	}
	sort.Strings(out)
	var uniq []string
	for i, u := range out {
		if i == 0 || u != out[i-1] {
			uniq = append(uniq, u)
		}
	}
	return uniq
}

// wrappedGateRoutes: routes whose admin gate is wrapped by a pinned function so that a scrape token can bypass it. The reason is the pin.
var wrappedGateRoutes = map[string]string{
	"GET /metrics": "metricsSkipIfScrape(RequireAuth) + metricsSkipIfScrape(RequireAdmin): runs the gate unless the scrape token matched",
}

const wrappedGateSymbol = "metricsSkipIfScrape"

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// realBinaryRoutes builds the REAL main() with the test-only build tag routedump (main_routedump_on.go), runs it with the same
// environment the other real-binary tests use, and returns the route table it dumps right after the last route registration (the
// hook sits next to verifyAuthRouteTable in main.go) before it listens. The binary is built once per test process.
var realRoutesCache struct {
	once   sync.Once
	routes []realRoute
	err    error
}

func realBinaryRoutes(t *testing.T) []realRoute {
	t.Helper()
	realRoutesCache.once.Do(func() {
		dir, err := os.MkdirTemp("", "routedump")
		if err != nil {
			realRoutesCache.err = err
			return
		}
		defer os.RemoveAll(dir)
		bin := filepath.Join(dir, "catalog-api-routedump")
		if out, err := exec.Command("go", "build", "-buildvcs=false", "-tags", "routedump", "-o", bin, ".").CombinedOutput(); err != nil {
			realRoutesCache.err = fmt.Errorf("build with -tags routedump failed: %v\n%s", err, out)
			return
		}
		dump := filepath.Join(dir, "routes.json")
		b := &binEnv{t: t, bin: bin, dir: dir, port: 0}
		cmd := exec.Command(bin)
		cmd.Dir = dir
		cmd.Env = b.env("GIN_MODE=release", "ADMIN_PASSWORD=Op3rator-Supplied-Value-9x", "ROUTE_DUMP_FILE="+dump, "HELIX_PPROF_ENABLED=true")
		out := &safeBuf{}
		cmd.Stdout, cmd.Stderr = out, out
		if err := cmd.Start(); err != nil {
			realRoutesCache.err = err
			return
		}
		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()
		select {
		case err := <-done:
			if err != nil {
				realRoutesCache.err = fmt.Errorf("route dump run failed: %v\n%s", err, out.String())
				return
			}
		case <-time.After(180 * time.Second):
			_ = cmd.Process.Kill()
			<-done
			realRoutesCache.err = fmt.Errorf("route dump run timed out\n%s", out.String())
			return
		}
		raw, err := os.ReadFile(dump)
		if err != nil {
			realRoutesCache.err = fmt.Errorf("no route dump written: %v\n%s", err, out.String())
			return
		}
		var rc []routeChain
		if err := json.Unmarshal(raw, &rc); err != nil {
			realRoutesCache.err = err
			return
		}
		for _, r := range rc {
			realRoutesCache.routes = append(realRoutesCache.routes, realRoute{r.Method, r.Path, r.Chain})
		}
	})
	require.NoError(t, realRoutesCache.err)
	return realRoutesCache.routes
}

// The ground truth: the REAL engine's route table (built from the real main(), reflection over gin's trees) against the scanner and
// the authorization table, in both directions, with the gate identified by function identity and required before the terminal handler.
func TestADMD1F1S1R5_RealRouteTableMatchesScannerAndTable(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs the server binary")
	}
	real := realBinaryRoutes(t)
	require.NotEmpty(t, real)
	s := scanRouteSources(t, realRouteFiles(t))
	require.Empty(t, s.unresolved)
	require.Empty(t, groundTruthViolations(real, s, readAuthzTable(t)))
	var gated int
	for _, r := range real {
		if g, _ := realGateState(r.chain); g {
			gated++
		}
	}
	require.Greater(t, gated, 20, "control needle: the real chains carry identified admin gates")
	fmt.Println("REAL ROUTES", len(real), "SCAN ROUTES", len(s.routes), "REAL GATED", gated)
}

// The comparator itself, on synthetic route tables (no binary): each defect class must be reported, the faithful case must not.
func TestADMD1F1S1R5_ComparatorDetectsEachDefectClass(t *testing.T) {
	gateH := funcIdentity(root_middleware.NewJWTMiddleware("cmp").RequireAdmin())
	plain := funcIdentity(probeH)
	scanOf := func(src string) *routeScan { return scanSrc(t, fcHeader+src+"\n") }
	good := scanOf(`func Run(m mw) { r := gin.New(); r.GET("/x", m.RequireAdmin(), h); r.GET("/y", h) }`)
	realGood := []realRoute{{"GET", "/x", []routeHandler{gateH, plain}}, {"GET", "/y", []routeHandler{plain}}}
	require.Empty(t, groundTruthViolations(realGood, good, nil), "faithful")
	has := func(real []realRoute, sc *routeScan, frag string) {
		t.Helper()
		v := strings.Join(groundTruthViolations(real, sc, nil), "\n")
		require.Contains(t, v, frag)
	}
	has(realGood[:1], good, "scanner route is not in the real route table")
	has(append(append([]realRoute{}, realGood...), realRoute{"GET", "/z", []routeHandler{plain}}), good, "real route is not seen by the scanner")
	has([]realRoute{{"GET", "/x", []routeHandler{plain, plain}}, realGood[1]}, good, "gate state differs")
	has([]realRoute{{"GET", "/x", []routeHandler{plain, gateH}}, realGood[1]}, good, "gate is the terminal handler")
	has([]realRoute{{"GET", "/x", []routeHandler{gateH, plain}}, {"GET", "/y", []routeHandler{gateH, plain}}}, good, "gate state differs")
	fake := funcIdentity(fakeGateFactory())
	has([]realRoute{{"GET", "/x", []routeHandler{fake, plain}}, realGood[1]}, good, "gate state differs")
	adm := []tableRow{{"GET", "/x", "RequireAdmin (route)"}}
	require.Empty(t, groundTruthViolations(realGood, good, adm), "table agrees")
	require.Contains(t, strings.Join(groundTruthViolations(realGood, good, []tableRow{{"GET", "/y", "RequireAdmin (route)"}}), "\n"), "authorization table lists a RequireAdmin route that is not gated")
	require.Contains(t, strings.Join(groundTruthViolations(realGood, good, []tableRow{{"GET", "/y", "RequireAdmin (route)"}}), "\n"), "real route is gated by RequireAdmin but the authorization table has no")
}

// Pins: the gate-wrapper allow-lists are exact, and a wrapped route is accepted only when the wrapper closures are really in the chain.
func TestADMD1F1S1R5_WrapperPinsAreExact(t *testing.T) {
	require.Equal(t, map[string]bool{"metricsSkipIfScrape": true}, gateWrapperFuncs)
	require.Equal(t, 1, len(wrappedGateRoutes))
	require.NotEmpty(t, wrappedGateRoutes["GET /metrics"])
	plain := funcIdentity(probeH)
	wrap := routeHandler{Name: "catalogizer.metricsSkipIfScrape.func1", File: "main_metricsauth.go"}
	sc := scanSrc(t, fcHeader+"func metricsSkipIfScrape(g gin.HandlerFunc) gin.HandlerFunc { return g }\nfunc Run(m mw) { r := gin.New(); r.GET(\"/metrics\", metricsSkipIfScrape(m.RequireAdmin()), h) }\n")
	both := []realRoute{{"GET", "/metrics", []routeHandler{wrap, wrap, plain}}}
	require.Empty(t, groundTruthViolations(both, sc, nil))
	one := []realRoute{{"GET", "/metrics", []routeHandler{wrap, plain}}}
	require.Contains(t, strings.Join(groundTruthViolations(one, sc, nil), "\n"), "gate state differs")
	none := []realRoute{{"GET", "/metrics", []routeHandler{plain}}}
	require.Contains(t, strings.Join(groundTruthViolations(none, sc, nil), "\n"), "gate state differs")
}
