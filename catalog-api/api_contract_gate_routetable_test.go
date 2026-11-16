package main

// ADMD1F1-S1 round 2 (F1): the PRIMARY cross-check of the admin marker. The marker is compared with an independent oracle, the
// route authorization table docs/ROUTE_AUTHZ_TABLE.md (a static extraction of the router registrations plus a live probe of the real
// binary, see its header), which is itself cross-checked here against the per-route admin registrations in the Go sources.
//
//	table RequireAdmin rows  ⊆  spec markers          (a new admin route cannot be added unmarked silently)
//	spec markers  \  table RequireAdmin rows  ⊆  permissionGatedMarkedExceptions   (explicit, reasoned, reviewed list)
//	code per-route admin registrations  ⊆  table RequireAdmin rows   (a new admin route cannot skip the table)
//
// The code scan (api_contract_gate_codescan_test.go) is AST based over every discovered route-registering file and compares FULL
// paths in both directions. Limits (UNCONFIRMED beyond them): gates it cannot see in the AST (a gate applied by a helper it does not
// model) are reported as table rows without a code registration. /metrics and /debug/pprof/* are not in the spec and are not
// marked (tracked follow-up ADMD1F1-S1-FU-NONSPEC-ADMIN).

import (
	"bufio"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

var tableParamRe = []*regexp.Regexp{regexp.MustCompile(`:[A-Za-z_][A-Za-z0-9_]*`), regexp.MustCompile(`\*[A-Za-z_][A-Za-z0-9_]*`), regexp.MustCompile(`\{[^}]*\}`)}

// normRoute lower-cases the method and replaces every parameter spelling ({id}, :id, *path) with {}.
func normRoute(method, path string) string {
	for _, r := range tableParamRe {
		path = r.ReplaceAllString(path, "{}")
	}
	return strings.ToLower(method) + " " + path
}

type tableRow struct{ method, path, gate string }

func readAuthzTable(t *testing.T) []tableRow {
	t.Helper()
	f, err := os.Open(filepath.Join("..", "docs", "ROUTE_AUTHZ_TABLE.md"))
	require.NoError(t, err)
	defer f.Close()
	var rows []tableRow
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		c := strings.Split(strings.Trim(strings.TrimSpace(sc.Text()), "|"), "|")
		if len(c) < 6 {
			continue
		}
		m := strings.TrimSpace(c[0])
		switch m {
		case "GET", "POST", "PUT", "DELETE", "PATCH":
			rows = append(rows, tableRow{m, strings.Trim(strings.TrimSpace(c[1]), "`"), strings.TrimSpace(c[2])})
		}
	}
	require.NoError(t, sc.Err())
	return rows
}

// tableAdminRoutes: rows gated by RequireAdmin (route or group), excluding RequireSelfOrAdmin, /metrics (scrape token) and
// /debug/pprof (not in the spec).
func tableAdminRoutes(rows []tableRow) map[string]bool {
	out := map[string]bool{}
	for _, r := range rows {
		if (strings.HasPrefix(r.gate, "RequireAdmin (route)") || strings.HasPrefix(r.gate, "RequireAdmin (group)")) && !strings.HasPrefix(r.path, "/debug/") && r.path != "/metrics" {
			out[normRoute(r.method, r.path)] = true
		}
	}
	return out
}

// permissionGatedMarkedExceptions: marked operations whose gate is NOT RequireAdmin but an in-handler permission / configGate check
// (a plain user is refused with 403 in the live probe, so the sweep's non-admin leg asserts them). Reviewed for ADMD1F1-S1; tracked
// item ADMD1F1-S1-FU-PERM-GATES (move them to a RequireAdmin-equivalent or give them their own marker).
var permissionGatedMarkedExceptions = []string{
	"delete /api/v1/logs/share/{}", "delete /api/v1/roles/{}", "delete /api/v1/users/{}",
	"get /api/v1/errors/crashes/{}", "get /api/v1/errors/reports/{}", "get /api/v1/logs/collections/{}",
	"get /api/v1/logs/collections/{}/analyze", "get /api/v1/logs/collections/{}/entries", "get /api/v1/roles/{}",
	"post /api/v1/configuration/wizard/complete", "post /api/v1/configuration/wizard/step/{}/save", "post /api/v1/configuration/wizard/step/{}/validate",
	"post /api/v1/logs/collections/{}/export", "post /api/v1/logs/share", "post /api/v1/users",
	"post /api/v1/users/{}/lock", "post /api/v1/users/{}/reset-password", "post /api/v1/users/{}/unlock",
	"put /api/v1/errors/crashes/{}/status", "put /api/v1/errors/reports/{}/status", "put /api/v1/roles/{}",
}

// routeTableViolations is the comparison, separated from the real files so mutants can feed it modified inputs.
func routeTableViolations(rows []tableRow, marked []string) []string {
	admin := tableAdminRoutes(rows)
	markedSet := map[string]bool{}
	for _, m := range marked {
		sp := strings.SplitN(m, " ", 2)
		markedSet[normRoute(sp[0], sp[1])] = true
	}
	exc := map[string]bool{}
	for _, e := range permissionGatedMarkedExceptions {
		exc[e] = true
	}
	gateOf := map[string]string{}
	for _, r := range rows {
		gateOf[normRoute(r.method, r.path)] = r.gate
	}
	var out []string
	for r := range admin {
		if !markedSet[r] {
			out = append(out, "RequireAdmin route in the table is not marked in the spec: "+r)
		}
	}
	for r := range markedSet {
		if admin[r] {
			continue
		}
		if !exc[r] {
			out = append(out, "marked in the spec but not a RequireAdmin route in the table and not a reviewed exception: "+r)
			continue
		}
		if g := gateOf[r]; g != "auth + in-handler permission" && g != "configGate" {
			out = append(out, "exception "+r+" has table gate "+g+", expected an in-handler permission gate")
		}
	}
	for e := range exc {
		if !markedSet[e] {
			out = append(out, "stale exception (not marked in the spec): "+e)
		}
	}
	sort.Strings(out)
	return out
}

func codeVsTableViolations(code [][2]string, rows []tableRow) []string {
	have := map[string]bool{}
	for _, r := range rows {
		if strings.HasPrefix(r.gate, "RequireAdmin (") {
			have[normFull(r.method, r.path)] = true
		}
	}
	var out []string
	for _, c := range code {
		if !have[normFull(c[0], c[1])] {
			out = append(out, "code registers an admin route with no RequireAdmin table row: "+c[0]+" "+c[1])
		}
	}
	return out
}

func TestADMD1F1S1_MarkersEqualCodeDerivedAdminRoutes(t *testing.T) {
	rows := readAuthzTable(t)
	marked := adminGatedSet(loadRealSpec(t))
	require.Empty(t, routeTableViolations(rows, marked))
	require.Empty(t, codeVsTableViolations(codeAdminRoutes(scanRouteSources(t, realRouteFiles(t))), rows))
}

// control needles: the instrument finds a known admin route, does not find a known user route or a fabricated one.
func TestADMD1F1S1_RouteTableInstrumentControlNeedles(t *testing.T) {
	rows := readAuthzTable(t)
	require.Greater(t, len(rows), 200)
	admin := tableAdminRoutes(rows)
	require.True(t, admin[normRoute("post", "/api/v1/smb/probe")], "known group-gated admin route")
	require.True(t, admin[normRoute("PUT", "/api/v1/sync/endpoints/:id")], "param spelling :id == {id}")
	require.True(t, admin[normRoute("get", "/api/v1/storage/list/*path")], "wildcard == {path}")
	require.False(t, admin[normRoute("get", "/api/v1/browse")], "known user route")
	require.False(t, admin[normRoute("get", "/api/v1/zz/fabricated")], "fabricated route")
	require.False(t, admin[normRoute("get", "/metrics")], "excluded by design")
	code := codeAdminRoutes(scanRouteSources(t, realRouteFiles(t)))
	require.GreaterOrEqual(t, len(code), 60)
	require.Contains(t, code, [2]string{"put", "/api/v1/sync/endpoints/:id"})
}

func TestADMD1F1S1_RouteTableMutantsAreKilled(t *testing.T) {
	rows := readAuthzTable(t)
	marked := adminGatedSet(loadRealSpec(t))
	t.Run("N1 a new RequireAdmin row with no spec marker (usual wording, no marker)", func(t *testing.T) {
		v := routeTableViolations(append(append([]tableRow{}, rows...), tableRow{"POST", "/api/v1/zz/new-admin-op", "RequireAdmin (route)"}), marked)
		require.Len(t, v, 1)
		require.Contains(t, v[0], "not marked")
	})
	t.Run("N2 a marker removed from a RequireAdmin route", func(t *testing.T) {
		var m2 []string
		for _, m := range marked {
			if m != "post /api/v1/smb/probe" {
				m2 = append(m2, m)
			}
		}
		require.Len(t, m2, len(marked)-1)
		require.NotEmpty(t, routeTableViolations(rows, m2))
	})
	t.Run("N3 a marker on a plain user route", func(t *testing.T) {
		v := routeTableViolations(rows, append(append([]string{}, marked...), "get /api/v1/browse"))
		require.Len(t, v, 1)
		require.Contains(t, v[0], "not a reviewed exception")
	})
	t.Run("N4 stale exception", func(t *testing.T) {
		var m2 []string
		for _, m := range marked {
			if m != "post /api/v1/users" {
				m2 = append(m2, m)
			}
		}
		require.NotEmpty(t, routeTableViolations(rows, m2))
	})
	t.Run("N5 admin registration in code with no table row", func(t *testing.T) {
		v := codeVsTableViolations(append(codeAdminRoutes(scanRouteSources(t, realRouteFiles(t))), [2]string{"post", "/api/v1/zz-new-admin"}), rows)
		require.Len(t, v, 1)
	})
	t.Run("N6 exception whose table gate is RequireAdmin-unrelated", func(t *testing.T) {
		rows2 := append([]tableRow{}, rows...)
		for i := range rows2 {
			if normRoute(rows2[i].method, rows2[i].path) == "post /api/v1/users" {
				rows2[i].gate = "auth"
			}
		}
		require.NotEmpty(t, routeTableViolations(rows2, marked))
	})
}

// F4 (R6/R7 re-kill): the default suite (no live tag) proves structurally, on the AST of the live sweep, that the loader fills
// cOp.AdminGated from adminGatedOp and that the sweep's gate decision is op.AdminGated, nothing else.
func liveSweepGateWiring(t *testing.T, src string) (loaderFills, sweepUses bool) {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "api_contract_live_test.go", src, 0)
	require.NoError(t, err)
	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.KeyValueExpr:
			if k, ok := x.Key.(*ast.Ident); ok && k.Name == "AdminGated" {
				if c, ok := x.Value.(*ast.CallExpr); ok {
					if fn, ok := c.Fun.(*ast.Ident); ok && fn.Name == "adminGatedOp" {
						loaderFills = true
					}
				}
			}
		case *ast.AssignStmt:
			if len(x.Lhs) == 1 && len(x.Rhs) == 1 {
				if l, ok := x.Lhs[0].(*ast.Ident); ok && l.Name == "gated" {
					if s, ok := x.Rhs[0].(*ast.SelectorExpr); ok {
						if id, ok := s.X.(*ast.Ident); ok && id.Name == "op" && s.Sel.Name == "AdminGated" {
							sweepUses = true
						}
					}
				}
			}
		}
		return true
	})
	return
}

func TestADMD1F1S1_LiveSweepGateWiring(t *testing.T) {
	raw, err := os.ReadFile("api_contract_live_test.go")
	require.NoError(t, err)
	src := string(raw)
	a, b := liveSweepGateWiring(t, src)
	require.True(t, a, "loadSpec must set AdminGated from adminGatedOp(od)")
	require.True(t, b, "the sweep must decide `gated := op.AdminGated`")
	require.Contains(t, src, "adminGateLoaderViolations(p, item)", "loader must reject malformed markers loudly")
	// mutants R6 / R7
	a, b = liveSweepGateWiring(t, strings.Replace(src, "gated := op.AdminGated", "gated := false", 1))
	require.True(t, a)
	require.False(t, b, "R6: gated := false must be detected")
	a, b = liveSweepGateWiring(t, strings.Replace(src, "AdminGated: adminGatedOp(od), ", "", 1))
	require.False(t, a, "R7: loader no longer fills AdminGated must be detected")
	require.True(t, b)
	a, _ = liveSweepGateWiring(t, strings.Replace(src, "AdminGated: adminGatedOp(od)", "AdminGated: false", 1))
	require.False(t, a, "loader hard-coded false must be detected")
}
