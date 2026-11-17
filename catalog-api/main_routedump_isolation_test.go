package main

// G1FIXFU3 (STRUCTURAL round, §11.4.276(E), §11.4.250). Two review rounds showed that policing the bodies of routeDumpHook and
// routeChainFuncNames with an AST name scanner cannot prove they are read-only (reviewer survivors X1, X2/X3, X4, X5b, X9h, X10).
// The redesign removes the thing to police from the shipped binary:
//
//   - routeChainFuncNames, funcIdentity, routeChain, routeHandler (every reflect/unsafe use for route dumping) live in files that
//     carry `//go:build routedump` (main_routechains.go, main_routedump_on.go). A default build does not compile them.
//   - routeDumpHook in the default build (main_routedump_off.go) must have an empty body in a file that holds nothing else.
//   - a real `go build` of the default tag is inspected with `go tool nm` and `go tool objdump`: no dump symbol, no routeDumpHook
//     code, no reference from package main to reflect Value Call/CallSlice/Method/MethodByName/Set*.
//
// Honest boundary: the routedump build is test-only and never shipped, so a mutation inside a routedump-tagged file changes only
// that test build (TestAuthSurfaceBinary_RoutedumpFileMutantsDoNotReachShippedBinary shows it). Mutations of OTHER shipped code
// (for example a slice write through an unsafe pointer in a default file) are not reached by the symbol gate; the remaining
// scanner rules and the startup verifier (verifyAuthRouteTable) own those. Unit-class source scan plus real-build inspection.

import (
	"encoding/json"
	"go/ast"
	"go/build/constraint"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"fmt"

	"github.com/stretchr/testify/require"
)

// dumpSymbols are the identifiers that exist only to walk the engine's route trees.
var dumpSymbols = map[string]bool{"routeChainFuncNames": true, "funcIdentity": true, "routeChain": true, "routeHandler": true}

// fileConstraint returns the file's //go:build expression, nil when it has none.
func fileConstraint(f *ast.File) constraint.Expr {
	for _, cg := range f.Comments {
		if cg.Pos() >= f.Package {
			break
		}
		for _, c := range cg.List {
			if constraint.IsGoBuild(c.Text) {
				if e, err := constraint.Parse(c.Text); err == nil {
					return e
				}
			}
		}
	}
	return nil
}

// evalConstraint evaluates e with routedump = rd and every other tag true (the conservative reading for a "compiled in the default
// build" question: a file is only treated as absent when routedump alone excludes it).
func evalConstraint(e constraint.Expr, rd bool) bool {
	if e == nil {
		return true
	}
	return e.Eval(func(tag string) bool {
		if tag == "routedump" {
			return rd
		}
		return true
	})
}

var reflectSetNames = map[string]bool{"Set": true, "SetLen": true, "SetCap": true, "SetMapIndex": true, "SetPointer": true, "SetZero": true,
	"SetIterKey": true, "SetIterValue": true, "SetBool": true, "SetInt": true, "SetUint": true, "SetFloat": true, "SetString": true,
	"SetBytes": true, "SetComplex": true}

// scanDumpIsolation: source-level rules that keep the route-dump machinery out of the default build. Sound over the structure
// they check (build constraint, declared names, body emptiness, file contents), not over what a body does.
func scanDumpIsolation(fset *token.FileSet, files map[string]*ast.File) []string {
	var out []string
	names := make([]string, 0, len(files))
	for n := range files {
		names = append(names, n)
	}
	sort.Strings(names)
	hookDefault := 0
	for _, name := range names {
		f := files[name]
		con := fileConstraint(f)
		inDefault := evalConstraint(con, false)
		inDump := evalConstraint(con, true)
		// go:linkname in package main is refused in every file, tagged or not.
		for _, cg := range f.Comments {
			for _, c := range cg.List {
				if strings.HasPrefix(c.Text, "//go:linkname") {
					out = append(out, fmt.Sprintf("%s:%d: //go:linkname is refused in package main", name, fset.Position(c.Pos()).Line))
				}
			}
		}
		importsReflect := false
		for _, im := range f.Imports {
			if im.Path.Value == `"reflect"` {
				importsReflect = true
			}
		}
		if importsReflect {
			// No code of package main may mutate or call through reflection, in any form (call, method value, method expression,
			// package-level var). Name-based over selectors, so it also sees uncalled forms. `.Method` is refused only as a call or
			// as reflect.Value.Method (gin.RouteInfo.Method is a plain field).
			ast.Inspect(f, func(n ast.Node) bool {
				switch x := n.(type) {
				case *ast.SelectorExpr:
					nm := x.Sel.Name
					if nm == "Call" || nm == "CallSlice" || nm == "MethodByName" || strings.HasPrefix(nm, "Set") && reflectSetNames[nm] {
						out = append(out, fmt.Sprintf("%s:%d: reflect mutation/call selector .%s is refused in package main (file imports reflect)", name, fset.Position(x.Pos()).Line, nm))
					}
					if nm == "Method" {
						if inner, ok := x.X.(*ast.SelectorExpr); ok && inner.Sel.Name == "Value" {
							out = append(out, fmt.Sprintf("%s:%d: reflect.Value.Method expression is refused in package main", name, fset.Position(x.Pos()).Line))
						}
					}
				case *ast.CallExpr:
					if se, ok := x.Fun.(*ast.SelectorExpr); ok && se.Sel.Name == "Method" {
						out = append(out, fmt.Sprintf("%s:%d: .Method(...) call is refused in package main (file imports reflect)", name, fset.Position(x.Pos()).Line))
					}
				}
				return true
			})
		}
		declaresDump := false
		for _, d := range f.Decls {
			switch dd := d.(type) {
			case *ast.FuncDecl:
				if dd.Recv == nil && dumpSymbols[dd.Name.Name] {
					declaresDump = true
				}
				if dd.Recv == nil && dd.Name.Name == "routeDumpHook" && inDefault {
					hookDefault++
					if name != "main_routedump_off.go" {
						out = append(out, fmt.Sprintf("%s: the default-build routeDumpHook must live in main_routedump_off.go", name))
					}
					if dd.Body == nil || len(dd.Body.List) != 0 {
						out = append(out, fmt.Sprintf("%s:%d: the default-build routeDumpHook body must be empty", name, fset.Position(dd.Pos()).Line))
					}
					for _, d2 := range f.Decls {
						switch x := d2.(type) {
						case *ast.FuncDecl:
							if x != dd {
								out = append(out, fmt.Sprintf("%s: main_routedump_off.go may hold nothing but routeDumpHook (found func %s)", name, x.Name.Name))
							}
						case *ast.GenDecl:
							if x.Tok != token.IMPORT {
								out = append(out, fmt.Sprintf("%s: main_routedump_off.go may hold nothing but routeDumpHook (found %s declaration)", name, x.Tok))
								continue
							}
							for _, sp := range x.Specs {
								if p := sp.(*ast.ImportSpec).Path.Value; p != `"github.com/gin-gonic/gin"` {
									out = append(out, fmt.Sprintf("%s: main_routedump_off.go may import only gin (found %s)", name, p))
								}
							}
						}
					}
				}
			case *ast.GenDecl:
				for _, sp := range dd.Specs {
					if ts, ok := sp.(*ast.TypeSpec); ok && dumpSymbols[ts.Name.Name] {
						declaresDump = true
					}
					if vs, ok := sp.(*ast.ValueSpec); ok {
						for _, n := range vs.Names {
							if dumpSymbols[n.Name] {
								declaresDump = true
							}
						}
					}
				}
			}
		}
		if declaresDump && (inDefault || !inDump) {
			out = append(out, fmt.Sprintf("%s: declares route-dump machinery (routeChainFuncNames/funcIdentity/routeChain/routeHandler) but its build constraint does not exclude it from the default build (routedump must be required)", name))
		}
	}
	if hookDefault != 1 {
		out = append(out, fmt.Sprintf("the default build must contain exactly one routeDumpHook (found %d)", hookDefault))
	}
	return out
}

func TestAuthSurfaceDumpIsolation_RealSourcesPass(t *testing.T) {
	fset, files := nonTestFiles(t, ".")
	require.Empty(t, scanDumpIsolation(fset, files))
	// Control: the rule saw the tagged files and the off hook (a scan over nothing is blind).
	require.Contains(t, files, "main_routechains.go")
	require.Contains(t, files, "main_routedump_on.go")
	require.Contains(t, files, "main_routedump_off.go")
	require.False(t, evalConstraint(fileConstraint(files["main_routechains.go"]), false))
	require.True(t, evalConstraint(fileConstraint(files["main_routechains.go"]), true))
	require.True(t, evalConstraint(fileConstraint(files["main_routedump_off.go"]), false))
}

// TestRouteChainsLockstep: the test-build copy (main_routechains_test.go, default tag) and the routedump copy (main_routechains.go)
// are the same code from the package clause on; the dump binary and the unit tests must walk the engine identically.
func TestRouteChainsLockstep(t *testing.T) {
	body := func(p string) string {
		b, err := os.ReadFile(p)
		require.NoError(t, err)
		s := string(b)
		i := strings.Index(s, "\npackage main\n")
		require.GreaterOrEqual(t, i, 0, p)
		return s[i:]
	}
	require.Equal(t, body("main_routechains.go"), body("main_routechains_test.go"))
}

// --- real-binary gate -------------------------------------------------------------------------------------------------------------

var (
	dumpSymRe   = regexp.MustCompile(`main\.(routeChainFuncNames|funcIdentity|routeChain|routeHandler)\b`)
	bannedRefRe = regexp.MustCompile(`reflect\.Value\.(Call|CallSlice|Method|MethodByName|Set[A-Za-z]*)\b`)
)

type binReport struct {
	dumpSyms   []string // symbols naming route-dump machinery
	bannedRefs []string // references from package main to banned reflect operations
	hookSize   int      // size of main.routeDumpHook in bytes, -1 when absent (inlined)
	hookCalls  int      // CALL instructions inside main.routeDumpHook
	nmLines    int
	hasMain    bool
}

// buildMain builds the catalog-api main package (tags, overlay: absolute path -> replacement content) into a temp dir.
func buildMain(t *testing.T, tags string, overlay map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "catalog-api-inspect")
	args := []string{"build", "-buildvcs=false", "-o", bin}
	if tags != "" {
		args = append(args, "-tags", tags)
	}
	if len(overlay) > 0 {
		repl := map[string]string{}
		i := 0
		for p, content := range overlay {
			abs, err := filepath.Abs(p)
			require.NoError(t, err)
			f := filepath.Join(dir, fmt.Sprintf("ov%d.go.txt", i))
			i++
			require.NoError(t, os.WriteFile(f, []byte(content), 0o600))
			repl[abs] = f
		}
		raw, _ := json.Marshal(map[string]map[string]string{"Replace": repl})
		ovf := filepath.Join(dir, "overlay.json")
		require.NoError(t, os.WriteFile(ovf, raw, 0o600))
		args = append(args, "-overlay", ovf)
	}
	args = append(args, ".")
	cmd := exec.Command("go", args...)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "go build %v\n%s", args, out)
	return bin
}

func inspectBinary(t *testing.T, bin string) binReport {
	t.Helper()
	r := binReport{hookSize: -1}
	nm, err := exec.Command("go", "tool", "nm", "-size", bin).CombinedOutput()
	require.NoError(t, err, string(nm))
	seen := map[string]bool{}
	for _, ln := range strings.Split(string(nm), "\n") {
		if strings.TrimSpace(ln) == "" {
			continue
		}
		r.nmLines++
		fs := strings.Fields(ln)
		if len(fs) >= 4 && fs[3] == "main.main" {
			r.hasMain = true
		}
		if m := dumpSymRe.FindString(ln); m != "" && !seen[m] {
			seen[m] = true
			r.dumpSyms = append(r.dumpSyms, m)
		}
		if len(fs) >= 4 && fs[3] == "main.routeDumpHook" {
			fmt.Sscan(fs[1], &r.hookSize)
		}
	}
	sort.Strings(r.dumpSyms)
	od, err := exec.Command("go", "tool", "objdump", "-s", `^main\.`, bin).CombinedOutput()
	require.NoError(t, err, string(od[:min(len(od), 400)]))
	require.Contains(t, string(od), "TEXT main.main(SB)", "control: objdump must show main.main")
	cur := ""
	refs := map[string]bool{}
	for _, ln := range strings.Split(string(od), "\n") {
		if strings.HasPrefix(ln, "TEXT ") {
			cur = strings.Fields(ln)[1]
			continue
		}
		if m := bannedRefRe.FindString(ln); m != "" && !refs[cur+" -> "+m] {
			refs[cur+" -> "+m] = true
			r.bannedRefs = append(r.bannedRefs, cur+" -> "+m)
		}
		if strings.HasPrefix(cur, "main.routeDumpHook(") && strings.Contains(ln, "\tCALL ") {
			r.hookCalls++
		}
	}
	sort.Strings(r.bannedRefs)
	return r
}

// shippedViolations is the gate: what must be true of the default-tag binary.
func shippedViolations(r binReport) []string {
	var v []string
	if !r.hasMain || r.nmLines < 1000 {
		v = append(v, fmt.Sprintf("BLIND: symbol table not read (hasMain=%v lines=%d)", r.hasMain, r.nmLines))
	}
	for _, s := range r.dumpSyms {
		v = append(v, "route-dump symbol in shipped binary: "+s)
	}
	for _, s := range r.bannedRefs {
		v = append(v, "package main references a banned reflect operation: "+s)
	}
	if r.hookSize > 16 {
		v = append(v, fmt.Sprintf("main.routeDumpHook is %d bytes (must be absent or a bare return)", r.hookSize))
	}
	if r.hookCalls > 0 {
		v = append(v, fmt.Sprintf("main.routeDumpHook contains %d CALL instructions", r.hookCalls))
	}
	return v
}

func readSrc(t *testing.T, name string) string {
	b, err := os.ReadFile(name)
	require.NoError(t, err)
	return string(b)
}

func TestAuthSurfaceBinary_ShippedBinaryHasNoRouteDumpCode(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the real binary")
	}
	// Negative control: the shipped (default tag) binary is clean.
	def := inspectBinary(t, buildMain(t, "", nil))
	require.Empty(t, shippedViolations(def), "default binary: %+v", def)
	// Positive control: the same assertions on a routedump build MUST find the machinery, or the detector is blind.
	dump := inspectBinary(t, buildMain(t, "routedump", nil))
	require.Contains(t, dump.dumpSyms, "main.routeChainFuncNames", "routedump build must carry the symbol: %+v", dump)
	require.NotEmpty(t, shippedViolations(dump))
	// Negative control for the needle: a name nothing defines is not found.
	require.NotContains(t, strings.Join(def.dumpSyms, " "), "main.noSuchFunc")
}

const x1Src = `package main

import "reflect"

// X1-shaped: index-assign through Interface+len, reached from the default hook.
func routeChainFuncNames(eng any) ([]int, error) {
	h := reflect.ValueOf(eng).Elem().FieldByName("Handlers").Interface().([]func())
	if len(h) > 0 {
		h[0] = h[len(h)-1]
	}
	return nil, nil
}
`

func offHook(body string, imports string) string {
	return "//go:build !routedump\n\npackage main\n\nimport (\n\t\"github.com/gin-gonic/gin\"\n" + imports + ")\n\nfunc routeDumpHook(e *gin.Engine) {\n" + body + "\n}\n"
}

// binMutants: each is applied to the REAL files through go build -overlay. wantBin: the shipped-binary gate must fire;
// wantScan: the source rules must fire (substring). A mutant that only targets a routedump-tagged file is the boundary case.
func TestAuthSurfaceBinary_ReviewerMutantsKilled(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the real binary")
	}
	abs := func(n string) string { p, _ := filepath.Abs(n); return p }
	cases := []struct {
		name     string
		overlay  map[string]string
		wantBin  string // substring in a shippedViolations finding; "" = not required
		wantScan string
	}{
		{"X1_index_assign_via_Interface_len_reached_from_hook",
			map[string]string{abs("main_routedump_off.go"): offHook("\t_, _ = routeChainFuncNames(e)", ""), abs("main_x1.go"): x1Src},
			"main.routeChainFuncNames", "default-build routeDumpHook body must be empty"},
		{"X2X3_off_hook_walks_engine_and_sets",
			map[string]string{abs("main_routedump_off.go"): offHook("\treflect.ValueOf(e).Elem().FieldByName(\"Handlers\").Set(reflect.Zero(reflect.TypeOf(e.Handlers)))", "\t\"reflect\"\n")},
			"reflect.Value.Set", "body must be empty"},
		{"X5b_method_expression_of_reflect_Value_Call",
			map[string]string{abs("main_x5b.go"): "package main\n\nimport (\n\t\"os\"\n\t\"reflect\"\n)\n\nfunc init() {\n\tif len(os.Args) > 99 {\n\t\tf := reflect.Value.Call\n\t\tf(reflect.ValueOf(1), nil)\n\t}\n}\n"},
			"", "reflect mutation/call selector .Call"}, // local method expression compiles to an indirect CALL through a data funcval: the objdump text does not name it, the selector rule does
		{"X5c_method_expression_in_package_level_var",
			map[string]string{abs("main_x5c.go"): "package main\n\nimport \"reflect\"\n\nvar x5cSink = reflect.Value.Call\n"},
			"", "reflect mutation/call selector .Call"},
		{"X10_funcIdentity_in_default_file_does_reflect_Set_and_is_called_by_hook",
			map[string]string{
				abs("main_routedump_off.go"): offHook("\t_ = funcIdentity(e)", ""),
				abs("main_x10.go"):           "package main\n\nimport \"reflect\"\n\nfunc funcIdentity(fn any) int {\n\treflect.ValueOf(fn).Elem().FieldByName(\"Handlers\").SetLen(0)\n\treturn 0\n}\n",
			},
			"main.funcIdentity", "body must be empty"},
		{"X9h_rebind_walk_moved_to_default_file",
			map[string]string{
				abs("main_routedump_off.go"): offHook("\t_, _ = routeChainFuncNames(e)", ""),
				abs("main_x9.go"):            "package main\n\nvar walkRebound = 0\n\nfunc routeChainFuncNames(eng any) ([]int, error) {\n\twalkRebound++\n\treturn nil, nil\n}\n",
			},
			"", "body must be empty"}, // tiny body: inlined into main, no symbol survives; the source rules own this shape (binary gate is a backstop, not the only layer)
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			rep := inspectBinary(t, buildMain(t, "", c.overlay))
			v := strings.Join(shippedViolations(rep), "\n")
			if c.wantBin != "" {
				require.Contains(t, v, c.wantBin, "binary gate must kill %s; report %+v", c.name, rep)
			}
			// The same overlay applied to a copy of the sources must also trip the source rules.
			got := mutate(t, func(d string) {
				for p, content := range c.overlay {
					require.NoError(t, os.WriteFile(filepath.Join(d, filepath.Base(p)), []byte(content), 0o600))
				}
			})
			if c.wantScan != "" {
				require.Contains(t, strings.Join(got, "\n"), c.wantScan)
			}
			require.True(t, c.wantBin != "" || c.wantScan != "", "every mutant needs at least one killing layer")
			require.True(t, v != "" || len(got) > 0, "mutant %s survived both layers", c.name)
		})
	}
}

// Limit (stated, not hidden): a method expression stored in a PACKAGE-LEVEL var is a data relocation, not a text reference, so the
// objdump scan does not see it (observed: X5b as `var sink = reflect.Value.Call` passed the gate). The local form above is seen.

// X4 (//go:linkname) and the structural own mutants are checked on the source rules (they are about declarations and constraints,
// not about code that ends up in a binary).
func TestAuthSurfaceGuard_DumpIsolationMutants(t *testing.T) {
	cases := []struct {
		name string
		want string
		fn   func(d string)
	}{
		{"X4_go_linkname_in_default_file", "//go:linkname is refused", func(d string) {
			addFile(t, d, "main_x4.go", "import _ \"unsafe\"\n\n//go:linkname x4Sink runtime.x4\nvar x4Sink int\n")
		}},
		{"O1_build_tag_removed_from_routechains", "does not exclude it from the default build", func(d string) {
			edit(t, d, "main_routechains.go", "//go:build routedump\n", "")
		}},
		{"O2_off_hook_body_not_empty", "body must be empty", func(d string) {
			edit(t, d, "main_routedump_off.go", "func routeDumpHook(_ *gin.Engine) {}", "func routeDumpHook(e *gin.Engine) { _ = e }")
		}},
		{"O3_init_in_off_file", "may hold nothing but routeDumpHook", func(d string) {
			edit(t, d, "main_routedump_off.go", "func routeDumpHook(_ *gin.Engine) {}", "func routeDumpHook(_ *gin.Engine) {}\n\nfunc init() {}")
		}},
		{"O4_off_file_tag_flipped_to_routedump", "exactly one routeDumpHook (found 0)", func(d string) {
			edit(t, d, "main_routedump_off.go", "//go:build !routedump", "//go:build routedump")
		}},
		{"O5_dump_file_constraint_true_in_default_build", "does not exclude it from the default build", func(d string) {
			edit(t, d, "main_routechains.go", "//go:build routedump\n", "//go:build routedump || linux\n")
		}},
		{"O6_off_hook_moved_to_other_file", "must live in main_routedump_off.go", func(d string) {
			edit(t, d, "main_routedump_off.go", "func routeDumpHook(_ *gin.Engine) {}", "")
			addFile(t, d, "main_hookelsewhere.go", "import \"github.com/gin-gonic/gin\"\n\nfunc routeDumpHook(_ *gin.Engine) {}\n")
		}},
		{"O7_off_file_imports_reflect", "may import only gin", func(d string) {
			edit(t, d, "main_routedump_off.go", "import \"github.com/gin-gonic/gin\"", "import (\n\t\"github.com/gin-gonic/gin\"\n\t\"reflect\"\n)")
		}},
		{"O9_reflect_Set_in_helper_in_default_file", "reflect mutation/call selector .Set", func(d string) {
			addFile(t, d, "main_o9.go", "import \"reflect\"\n\nfunc o9(v reflect.Value) { v.Set(reflect.Zero(v.Type())) }\n")
		}},
		{"O10_reflect_Method_call", ".Method(...) call is refused", func(d string) {
			addFile(t, d, "main_o10.go", "import \"reflect\"\n\nfunc o10(v reflect.Value) { _ = v.Method(0) }\n")
		}},
		{"O11_reflect_Value_Method_expression", "reflect.Value.Method expression is refused", func(d string) {
			addFile(t, d, "main_o11.go", "import \"reflect\"\n\nvar o11 = reflect.Value.Method\n")
		}},
		{"O8_dump_type_declared_in_default_file", "does not exclude it from the default build", func(d string) {
			addFile(t, d, "main_o8.go", "type routeHandler struct{}\n")
		}},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			got := mutate(t, c.fn)
			require.Contains(t, strings.Join(got, "\n"), c.want)
		})
	}
	// Golden-false: the unmodified sources produce no dump-isolation finding (also covered by RealSourcesPass).
	require.Empty(t, mutate(t, func(string) {}))
}

// The boundary, executed: a mutation inside a routedump-tagged file (the X9h/X10 shape in the test-only files) is invisible to the
// shipped binary, which stays clean; it exists only in the test-only build.
func TestAuthSurfaceBinary_RoutedumpFileMutantsDoNotReachShippedBinary(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the real binary")
	}
	abs, _ := filepath.Abs("main_routechains.go")
	orig := readSrc(t, "main_routechains.go")
	// X9h/X10 shape: a write through the walked slice (and a reflect Set) inside the routedump-only file.
	mut := strings.Replace(orig, "if hv.Len() > 0 {", "if hv.Len() > 0 {\n\t\t\thv.Index(0).Set(hv.Index(hv.Len() - 1))", 1)
	require.NotEqual(t, orig, mut, "mutation must apply")
	shipped := inspectBinary(t, buildMain(t, "", map[string]string{abs: mut}))
	require.Empty(t, shippedViolations(shipped), "a mutated routedump-only file must not change the shipped binary: %+v", shipped)
	// And the lockstep rule sees the divergence from the test copy (the one place the two files are compared).
	require.NotEqual(t, strings.Index(mut, "\npackage main\n"), -1)
	require.NotEqual(t, mut[strings.Index(mut, "\npackage main\n"):], readSrc(t, "main_routechains_test.go")[strings.Index(readSrc(t, "main_routechains_test.go"), "\npackage main\n"):])
}
