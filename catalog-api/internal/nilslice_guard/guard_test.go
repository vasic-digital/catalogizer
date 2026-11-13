// Package nilslice_guard holds a structural guard for the nil-slice-serialises-as-JSON-null defect class.
//
// A function that declares `var out []T` (no initialiser) and returns it unchanged yields a nil slice when no row matched, and
// encoding/json writes a nil slice as null instead of []. Clients iterating the documented array then fail. The guard parses the
// listing packages and fails on every such declaration whose variable is returned directly.
package nilslice_guard

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// scanDirs are the layers that build list results returned to HTTP handlers (relative to this package).
// repository: every function. Service/handler layers: exported functions only; unexported helpers return nil by design
// (docs/qa/nil-slice-null-census-20261104.md lists them as scoped out).
var scanDirs = []string{"../../repository", "../../services", "../../handlers", "../../internal/services", "../../internal/handlers"}

func exportedOnly(dir string) bool { return !strings.HasSuffix(dir, "repository") }

func isSliceOrMap(e ast.Expr) bool {
	switch tt := e.(type) {
	case *ast.ArrayType:
		return tt.Len == nil // fixed-size array is never nil
	case *ast.MapType:
		return true
	}
	return false
}

// goFiles lists non-test Go files under dir, recursively (testdata and vendor skipped).
func goFiles(t *testing.T, dir string) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if n := d.Name(); n == "testdata" || n == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(p, ".go") && !strings.HasSuffix(p, "_test.go") {
			files = append(files, p)
		}
		return nil
	})
	if err != nil || len(files) == 0 {
		t.Fatalf("control needle: no Go files found under %s (err=%v)", dir, err)
	}
	return files
}

// offenders returns "file:line ..." for every nil-slice/map-to-JSON-null shape found, recursively under dir:
//
//	(0) `var x []T` / `var x map[K]V` without initialiser returned unchanged;
//	(a) a named slice/map result that is never assigned, with a bare `return`;
//	(b) `return nil, nil` where the first result is a slice/map and the last is error (success path);
//	(c) a struct-literal field initialised from an unset `var x []T` / map.
//
// Residual (d), documented not detected: `x := f()` where f returns a nil slice is a passthrough and needs type/flow analysis.
func offenders(t *testing.T, dir string, exported bool) []string {
	t.Helper()
	fset := token.NewFileSet()
	var out []string
	for _, f := range goFiles(t, dir) {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		af, err := parser.ParseFile(fset, f, src, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range af.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Body == nil || (exported && !fd.Name.IsExported()) {
				continue
			}
			decl := map[*ast.Object]token.Pos{} // keyed by object so same-named declarations in different scopes are all reported
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				if ds, ok := n.(*ast.DeclStmt); ok {
					if gd, ok := ds.Decl.(*ast.GenDecl); ok && gd.Tok == token.VAR {
						for _, sp := range gd.Specs {
							vs := sp.(*ast.ValueSpec)
							if len(vs.Values) == 0 && isSliceOrMap(vs.Type) {
								for _, nm := range vs.Names {
									if nm.Obj != nil {
										decl[nm.Obj] = nm.Pos()
									}
								}
							}
						}
					}
				}
				return true
			})
			// named results of slice/map type, and assignments to any identifier
			named := map[*ast.Object]token.Pos{}
			var resultTypes []ast.Expr
			if fd.Type.Results != nil {
				for _, fl := range fd.Type.Results.List {
					n := len(fl.Names)
					if n == 0 {
						n = 1
					}
					for i := 0; i < n; i++ {
						resultTypes = append(resultTypes, fl.Type)
					}
					for _, nm := range fl.Names {
						if isSliceOrMap(fl.Type) && nm.Name != "_" {
							named[nm.Obj] = nm.Pos()
						}
					}
				}
			}
			// keyed by *ast.Object (not name) so an inner closure that declares its own `out` does not mask the outer one
			assigned := map[*ast.Object]bool{}
			checked := map[*ast.Object]bool{}
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				switch x := n.(type) {
				case *ast.AssignStmt:
					for _, l := range x.Lhs {
						if id, ok := l.(*ast.Ident); ok {
							assigned[id.Obj] = true
						}
					}
				case *ast.UnaryExpr: // &name hands the variable to a callee that may fill it
					if id, ok := x.X.(*ast.Ident); ok && x.Op == token.AND {
						assigned[id.Obj] = true
					}
				case *ast.BinaryExpr: // `if x == nil { x = []T{} }` normalises x
					if x.Op == token.EQL {
						for _, side := range []ast.Expr{x.X, x.Y} {
							if id, ok := side.(*ast.Ident); ok && id.Name != "nil" {
								checked[id.Obj] = true
							}
						}
					}
				case *ast.RangeStmt:
					if id, ok := x.Key.(*ast.Ident); ok {
						assigned[id.Obj] = true
					}
					if id, ok := x.Value.(*ast.Ident); ok { // PX-F22: `for _, out = range c` assigns out too
						assigned[id.Obj] = true
					}
				}
				return true
			})
			for o := range checked {
				delete(decl, o)
				assigned[o] = true
			}
			flagged := map[string]bool{}
			add := func(pos token.Pos, msg string) {
				out = append(out, fset.Position(pos).String()+" "+msg+" in "+fd.Name.Name)
			}
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				switch x := n.(type) {
				case *ast.ReturnStmt:
					if len(x.Results) == 0 {
						for o, pos := range named { // (a)
							if !assigned[o] && !flagged["a"+o.Name] {
								flagged["a"+o.Name] = true
								add(pos, "named result "+o.Name+" never assigned, bare return")
							}
						}
					}
					for i, r := range x.Results {
						if id, ok := r.(*ast.Ident); ok { // (0)
							if pos, ok := decl[id.Obj]; ok {
								add(pos, "var "+id.Name+" returned unnormalised")
								delete(decl, id.Obj)
							}
						}
						// (b) return nil, nil
						if id, ok := r.(*ast.Ident); ok && id.Name == "nil" && i == 0 && len(x.Results) >= 2 && len(resultTypes) == len(x.Results) && isSliceOrMap(resultTypes[0]) {
							if last, ok := x.Results[len(x.Results)-1].(*ast.Ident); ok && last.Name == "nil" {
								add(x.Pos(), "return nil, nil with slice/map first result")
							}
						}
					}
				case *ast.CompositeLit: // (c)
					for _, el := range x.Elts {
						kv, ok := el.(*ast.KeyValueExpr)
						if !ok {
							continue
						}
						if id, ok := kv.Value.(*ast.Ident); ok {
							if pos, ok := decl[id.Obj]; ok {
								add(pos, "var "+id.Name+" used as struct-literal field value unnormalised")
								delete(decl, id.Obj)
							}
						}
					}
				}
				return true
			})
		}
	}
	sort.Strings(out)
	return out
}

// accepted lists reviewed findings that are not a null-on-the-wire defect (NS-3 dispositions, docs/qa/nil-slice-null-census-20261104.md).
// Each entry is the whole finding (scan-relative path, line:col removed) and carries its reason; a stale entry (no longer matching) fails the test.
var accepted = map[string]string{
	"repository/media_collection_repository.go var externalIDs used as struct-literal field value unnormalised in scanCollection":      "MediaCollection.ExternalIDs is `json:\"external_ids,omitempty\"`: a nil map is omitted, not null",
	"handlers/admin_handler.go var unavailable used as struct-literal field value unnormalised in GetSystemInfo":                       "GetSystemInfo always appends `totalRequests`, so the slice is never nil when serialised",
	"handlers/collection_handler.go var items used as struct-literal field value unnormalised in ListCollectionItems":                  "items is assigned from smartrules.Evaluate / ListManualItems, which return a non-nil slice whenever err == nil; the err path returns before serialising",
	"handlers/service_adapters.go var errs used as struct-literal field value unnormalised in ValidateWizardStep":                      "models.ValidationResult.Errors is `json:\"errors,omitempty\"`: nil is omitted, not null",
	"handlers/service_adapters.go var warns used as struct-literal field value unnormalised in ValidateWizardStep":                     "models.ValidationResult.Warnings is `json:\"warnings,omitempty\"`: nil is omitted, not null",
	"internal/services/music_player_service.go var tracks used as struct-literal field value unnormalised in PlayArtist":               "PlayArtist returns an error when tracks is empty, so a nil slice is never serialised",
	"internal/services/translation_service.go var results used as struct-literal field value unnormalised in TranslateBatch":           "TranslateBatch has no HTTP caller in the tree (service-internal); tracked, not on a served path",
	"internal/handlers/localization_handlers.go var providers used as struct-literal field value unnormalised in CheckLanguageSupport": "unrouted: LocalizationHandlers is never constructed outside tests; not a served path today",
	"internal/handlers/media.go var mediaItems used as struct-literal field value unnormalised in SearchMedia":                         "unrouted: internal/handlers.MediaHandler is never constructed outside tests; not a served path today",
	"internal/handlers/media.go var mediaTypes used as struct-literal field value unnormalised in GetMediaTypes":                       "unrouted: internal/handlers.MediaHandler is never constructed outside tests; not a served path today",
}

var lineCol = regexp.MustCompile(`:\d+:\d+ `)

// filterAccepted removes findings covered by an accepted entry. An entry is the WHOLE finding (shape and function included) with
// only line:col and the leading ../../ removed, matched exactly against the full scan path (the same file name in another directory never matches), and it may cover exactly ONE finding: a second finding with the same key (for
// example a new unnormalised var of the same name in the same function) is itself reported.
func filterAccepted(got []string, used map[string]int) []string {
	var rest []string
	for _, g := range got {
		norm := strings.TrimPrefix(lineCol.ReplaceAllString(g, " "), "../../")
		hit := false
		for k := range accepted {
			if norm == k {
				used[k]++
				if used[k] == 1 {
					hit = true
				}
			}
		}
		if !hit {
			rest = append(rest, g)
		}
	}
	return rest
}

// staleAccepted lists accepted entries that matched no finding.
func staleAccepted(used map[string]int) []string {
	var out []string
	for k := range accepted {
		if used[k] == 0 {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

func TestNoUnnormalisedNilListReturns(t *testing.T) {
	used := map[string]int{}
	t.Cleanup(func() {
		if t.Failed() {
			return
		}
		for _, k := range staleAccepted(used) {
			t.Errorf("stale accepted entry %q (%s): no longer matches a finding", k, accepted[k])
		}
	})
	for _, d := range scanDirs {
		if got := filterAccepted(offenders(t, d, exportedOnly(d)), used); len(got) != 0 {
			t.Errorf("%s: %d list function(s) can return a nil slice/map (JSON null); initialise with `x := []T{}`:\n  %s", d, len(got), strings.Join(got, "\n  "))
		}
	}
}

// Control needle: the detector must see a known offender (otherwise a zero above is blindness, not absence).
func TestDetectorSeesKnownOffender(t *testing.T) {
	dir := t.TempDir()
	src := "package x\nfunc A() ([]int, error) {\n\tvar out []int\n\treturn out, nil\n}\nfunc B() []int {\n\tout := []int{}\n\treturn out\n}\nfunc C() []int {\n\tvar args []int\n\t_ = args\n\treturn nil\n}\n"
	if err := os.WriteFile(filepath.Join(dir, "x.go"), []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	got := offenders(t, dir, false)
	if len(got) != 1 || !strings.Contains(got[0], "in A") {
		t.Fatalf("detector must flag exactly A, got %v", got)
	}
}

// Control needles for the NS-3 detectors: each shape must be flagged, and the clean twin of each must not (no false positive).
func TestDetectorsSeeNS3Shapes(t *testing.T) {
	cases := []struct {
		name, src string
		want      int // number of findings
		contains  string
	}{
		{"named-bare-return", "package x\nfunc A() (out []int, err error) {\n\treturn\n}\n", 1, "named result out"},
		{"named-assigned-clean", "package x\nfunc A() (out []int, err error) {\n\tout = []int{}\n\treturn\n}\n", 0, ""},
		{"return-nil-nil", "package x\nfunc A() ([]int, error) {\n\treturn nil, nil\n}\n", 1, "return nil, nil"},
		{"return-nil-err-clean", "package x\nimport \"errors\"\nfunc A() ([]int, error) {\n\treturn nil, errors.New(\"x\")\n}\nfunc B() (*int, error) {\n\treturn nil, nil\n}\n", 0, ""},
		{"struct-literal-field", "package x\ntype T struct{ L []int }\nfunc A() *T {\n\tvar l []int\n\treturn &T{L: l}\n}\n", 1, "struct-literal field"},
		{"inner-closure-shadow-does-not-mask-outer", "package x\nfunc A() (out []int, err error) {\n\tf := func() {\n\t\tout := []int{}\n\t\t_ = out\n\t}\n\tf()\n\treturn\n}\n", 1, "named result out"},
		{"inner-closure-nil-check-shadow-does-not-mask-var", "package x\nfunc A() []int {\n\tvar l []int\n\tf := func() {\n\t\tl := []int{}\n\t\tif l == nil {\n\t\t\tl = []int{}\n\t\t}\n\t}\n\tf()\n\treturn l\n}\n", 1, "var l returned"},
		{"struct-literal-clean", "package x\ntype T struct{ L []int }\nfunc A() *T {\n\tl := []int{}\n\treturn &T{L: l}\n}\nfunc B() *T {\n\tvar l []int\n\tif l == nil {\n\t\tl = []int{}\n\t}\n\treturn &T{L: l}\n}\n", 0, ""},
	}
	for _, c := range cases {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "x.go"), []byte(c.src), 0o600); err != nil {
			t.Fatal(err)
		}
		got := offenders(t, dir, false)
		if len(got) != c.want || (c.want > 0 && !strings.Contains(got[0], c.contains)) {
			t.Errorf("%s: want %d finding(s) containing %q, got %v", c.name, c.want, c.contains, got)
		}
	}
}

// PX-F15: clean twins (0 findings) and positive twins (1 finding) for the "assigned"/"checked" evidence shapes. Each pair pins one
// branch of the detector so a reviewer mutant that drops that branch turns a case red.
func TestDetectorAssignmentEvidenceShapes(t *testing.T) {
	cases := []struct {
		name, src string
		want      int
		contains  string
	}{
		// `out, e := g()` reuses the named result `out` (e is the new name): it is an assignment of out
		{"redeclare-reuses-named-result-clean", "package x\nfunc g() ([]int, error) { return []int{}, nil }\nfunc A() (out []int, err error) {\n\tout, e := g()\n\t_ = e\n\treturn\n}\n", 0, ""},
		{"redeclare-other-name-does-not-assign-out", "package x\nfunc g() ([]int, error) { return []int{}, nil }\nfunc A() (out []int, err error) {\n\to2, e := g()\n\t_, _ = o2, e\n\treturn\n}\n", 1, "named result out"},
		// `fill(&out)` hands the variable to a callee
		{"address-of-named-result-clean", "package x\nfunc fill(p *[]int) { *p = []int{} }\nfunc A() (out []int) {\n\tfill(&out)\n\treturn\n}\n", 0, ""},
		{"address-of-other-variable-does-not-assign-out", "package x\nfunc fill(p *int) {}\nfunc A() (out []int) {\n\tx := 1\n\tfill(&x)\n\treturn\n}\n", 1, "named result out"},
		// a range key that is the outer variable assigns it on every iteration
		{"range-key-assigns-outer-clean", "package x\nfunc A(c chan []int) (out []int) {\n\tfor out = range c {\n\t}\n\treturn\n}\n", 0, ""},
		{"range-key-other-variable-does-not-assign-out", "package x\nfunc A(c chan int) (out []int) {\n\tfor i := range c {\n\t\t_ = i\n\t}\n\treturn\n}\n", 1, "named result out"},
		// `!= nil` is not a normalisation of a nil slice; only `== nil` (the guarded assignment idiom) counts
		{"not-equal-nil-check-is-not-normalisation", "package x\nfunc A() []int {\n\tvar l []int\n\tif l != nil {\n\t\t_ = l\n\t}\n\treturn l\n}\n", 1, "var l returned"},
		{"equal-nil-check-is-normalisation-clean", "package x\nfunc A() []int {\n\tvar l []int\n\tif l == nil {\n\t\tl = []int{}\n\t}\n\treturn l\n}\n", 0, ""},
		// PX-F20: a named result that is only nil-checked (== nil) counts as normalised, EVEN WITH AN EMPTY BODY. That is a KNOWN
		// FALSE NEGATIVE of the current detector (the check evidence is not tied to an assignment in the body); this case pins
		// today's behaviour, it does not endorse it. Tracked as PX-F20: when the detector requires an assignment, flip want to 1.
		{"KNOWN-FALSE-NEGATIVE-checked-only-empty-body-currently-clean", "package x\nfunc A() (out []int) {\n\tif out == nil {\n\t}\n\treturn\n}\n", 0, ""},
		// PX-F21: reversed comparison `nil == out` is the same evidence as `out == nil`; `nil != out` is not a normalisation
		{"reversed-nil-equals-named-result-clean", "package x\nfunc A() (out []int) {\n\tif nil == out {\n\t}\n\treturn\n}\n", 0, ""}, // no assignment: only the comparison evidence makes it clean (same pinned false negative as PX-F20)
		{"reversed-nil-not-equal-named-result-flagged", "package x\nfunc A() (out []int) {\n\tif nil != out {\n\t}\n\treturn\n}\n", 1, "named result out"},
		// PX-F21: the named result assigned in a NON-FIRST left-hand position (assign and define forms)
		{"named-result-second-lhs-assign-clean", "package x\nfunc g() (int, []int) { return 1, []int{} }\nfunc A() (out []int) {\n\tvar x int\n\tx, out = g()\n\t_ = x\n\treturn\n}\n", 0, ""},
		{"named-result-second-lhs-define-clean", "package x\nfunc g() (error, []int) { return nil, []int{} }\nfunc A() (out []int) {\n\te, out := g()\n\t_ = e\n\treturn\n}\n", 0, ""},
		{"second-lhs-other-variable-does-not-assign-out", "package x\nfunc g() (int, []int) { return 1, []int{} }\nfunc A() (out []int) {\n\tvar x int\n\tvar y []int\n\tx, y = g()\n\t_, _ = x, y\n\treturn\n}\n", 1, "named result out"},
		// PX-F22: the range VALUE variable assigns the outer variable on every iteration (like the key)
		{"range-value-assigns-outer-clean", "package x\nfunc A(c [][]int) (out []int) {\n\tfor _, out = range c {\n\t}\n\treturn\n}\n", 0, ""},
		{"not-equal-checked-only-named-result-flagged", "package x\nfunc A() (out []int) {\n\tif out != nil {\n\t}\n\treturn\n}\n", 1, "named result out"},
		// `:=` (define) and `=` (assign) are both assignments
		{"plain-assign-named-result-clean", "package x\nfunc A() (out []int) {\n\tout = []int{}\n\treturn\n}\n", 0, ""},
		{"define-assign-in-inner-scope-does-not-assign-out", "package x\nfunc A() (out []int) {\n\tif true {\n\t\tout := []int{}\n\t\t_ = out\n\t}\n\treturn\n}\n", 1, "named result out"},
	}
	for _, c := range cases {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "x.go"), []byte(c.src), 0o600); err != nil {
			t.Fatal(err)
		}
		got := offenders(t, dir, false)
		if len(got) != c.want || (c.want > 0 && !strings.Contains(got[0], c.contains)) {
			t.Errorf("%s: want %d finding(s) containing %q, got %v", c.name, c.want, c.contains, got)
		}
	}
}

// The scan is recursive: an offender in a nested package directory must be found.
func TestScanIsRecursive(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "backups", "deep")
	if err := os.MkdirAll(sub, 0o700); err != nil {
		t.Fatal(err)
	}
	src := "package deep\nfunc A() ([]int, error) {\n\tvar out []int\n\treturn out, nil\n}\n"
	if err := os.WriteFile(filepath.Join(sub, "d.go"), []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := offenders(t, dir, false); len(got) != 1 {
		t.Fatalf("recursive scan must find the nested offender, got %v", got)
	}
}

// Mutation checks for the accepted list: a new unnormalised var must not be waved through by an existing entry.
func TestAcceptedList_RejectsNewFindings(t *testing.T) {
	const tail = " used as struct-literal field value unnormalised in "
	known := "../../handlers/admin_handler.go:121:6 var unavailable" + tail + "GetSystemInfo"
	// baseline: the known finding (any line) is accepted exactly once
	used := map[string]int{}
	if rest := filterAccepted([]string{known}, used); len(rest) != 0 {
		t.Fatalf("baseline finding must be accepted, got %v", rest)
	}
	// M2: a second finding with the same file/var/function (a new `var unavailable []string` in GetSystemInfo) is reported
	used = map[string]int{}
	rest := filterAccepted([]string{known, "../../handlers/admin_handler.go:200:6 var unavailable" + tail + "GetSystemInfo"}, used)
	if len(rest) != 1 {
		t.Fatalf("M2: duplicate of an accepted key must be reported, got %v", rest)
	}
	// M3: same file and var name in a different function, and a different var in the same function, are reported
	used = map[string]int{}
	rest = filterAccepted([]string{
		"../../handlers/collection_handler.go:50:6 var items" + tail + "ListSomethingElse",
		"../../handlers/admin_handler.go:50:6 var other" + tail + "GetSystemInfo",
		"../../handlers/admin_handler.go:50:6 var unavailable returned unnormalised in GetSystemInfo",
	}, used)
	if len(rest) != 3 {
		t.Fatalf("M3: findings differing in function, var or shape must be reported, got %v", rest)
	}
	// stale: with nothing matched every entry is stale; with the known finding matched, that entry is not
	used = map[string]int{}
	if got := staleAccepted(used); len(got) != len(accepted) {
		t.Fatalf("stale check must flag all %d unused entries, got %d", len(accepted), len(got))
	}
	filterAccepted([]string{known}, used)
	for _, k := range staleAccepted(used) {
		if strings.Contains(k, "admin_handler.go var unavailable") {
			t.Fatalf("matched entry must not be stale: %s", k)
		}
	}
}

// PX-F1: an accepted entry names the full scan path; the same file name in another directory must not take over its slot.
func TestAcceptedList_DoesNotCrossDirectories(t *testing.T) {
	const tail = " used as struct-literal field value unnormalised in GetSystemInfo"
	used := map[string]int{}
	rest := filterAccepted([]string{"../../internal/handlers/admin_handler.go:9:6 var unavailable" + tail}, used)
	if len(rest) != 1 {
		t.Fatalf("PX-F1: same file name under another directory must be reported, got %v", rest)
	}
	used = map[string]int{}
	if rest := filterAccepted([]string{"../../handlers/admin_handler.go:9:6 var unavailable" + tail}, used); len(rest) != 0 {
		t.Fatalf("PX-F1: the real path must stay accepted, got %v", rest)
	}
}

// PX-F2: every same-named declaration is reported (declarations are keyed by object, not by name).
func TestDetectorReportsEverySameNamedDeclaration(t *testing.T) {
	dir := t.TempDir()
	src := "package x\nfunc A() ([]int, func() []int) {\n\tvar out []int\n\tf := func() []int {\n\t\tvar out []int\n\t\treturn out\n\t}\n\treturn out, f\n}\n"
	if err := os.WriteFile(filepath.Join(dir, "x.go"), []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := offenders(t, dir, false); len(got) != 2 {
		t.Fatalf("PX-F2: want both declarations reported, got %v", got)
	}
}
