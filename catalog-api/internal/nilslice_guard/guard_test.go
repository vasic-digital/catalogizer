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
			decl := map[string]token.Pos{}
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				if ds, ok := n.(*ast.DeclStmt); ok {
					if gd, ok := ds.Decl.(*ast.GenDecl); ok && gd.Tok == token.VAR {
						for _, sp := range gd.Specs {
							vs := sp.(*ast.ValueSpec)
							if len(vs.Values) == 0 && isSliceOrMap(vs.Type) {
								for _, nm := range vs.Names {
									decl[nm.Name] = nm.Pos()
								}
							}
						}
					}
				}
				return true
			})
			// named results of slice/map type, and assignments to any identifier
			named := map[string]token.Pos{}
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
							named[nm.Name] = nm.Pos()
						}
					}
				}
			}
			assigned := map[string]bool{}
			checked := map[string]bool{}
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				switch x := n.(type) {
				case *ast.AssignStmt:
					for _, l := range x.Lhs {
						if id, ok := l.(*ast.Ident); ok {
							assigned[id.Name] = true
						}
					}
				case *ast.UnaryExpr: // &name hands the variable to a callee that may fill it
					if id, ok := x.X.(*ast.Ident); ok && x.Op == token.AND {
						assigned[id.Name] = true
					}
				case *ast.BinaryExpr: // `if x == nil { x = []T{} }` normalises x
					if x.Op == token.EQL {
						for _, side := range []ast.Expr{x.X, x.Y} {
							if id, ok := side.(*ast.Ident); ok && id.Name != "nil" {
								checked[id.Name] = true
							}
						}
					}
				case *ast.RangeStmt:
					if id, ok := x.Key.(*ast.Ident); ok {
						assigned[id.Name] = true
					}
				}
				return true
			})
			for nm := range checked {
				delete(decl, nm)
				assigned[nm] = true
			}
			flagged := map[string]bool{}
			add := func(pos token.Pos, msg string) {
				out = append(out, fset.Position(pos).String()+" "+msg+" in "+fd.Name.Name)
			}
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				switch x := n.(type) {
				case *ast.ReturnStmt:
					if len(x.Results) == 0 {
						for nm, pos := range named { // (a)
							if !assigned[nm] && !flagged["a"+nm] {
								flagged["a"+nm] = true
								add(pos, "named result "+nm+" never assigned, bare return")
							}
						}
					}
					for i, r := range x.Results {
						if id, ok := r.(*ast.Ident); ok { // (0)
							if pos, ok := decl[id.Name]; ok {
								add(pos, "var "+id.Name+" returned unnormalised")
								delete(decl, id.Name)
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
							if pos, ok := decl[id.Name]; ok {
								add(pos, "var "+id.Name+" used as struct-literal field value unnormalised")
								delete(decl, id.Name)
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
// Each entry is a substring of the finding and carries its reason; a stale entry (no longer matching) fails the test.
var accepted = map[string]string{
	"media_collection_repository.go:193:6 var externalIDs": "MediaCollection.ExternalIDs is `json:\"external_ids,omitempty\"`: a nil map is omitted, not null",
	"admin_handler.go:117:6 var unavailable":               "GetSystemInfo always appends `totalRequests`, so the slice is never nil when serialised",
	"collection_handler.go:290:3 var items":                "items is assigned from smartrules.Evaluate / ListManualItems, which return a non-nil slice whenever err == nil; the err path returns before serialising",
	"service_adapters.go:83:6 var errs":                    "models.ValidationResult.Errors is `json:\"errors,omitempty\"`: nil is omitted, not null",
	"service_adapters.go:87:6 var warns":                   "models.ValidationResult.Warnings is `json:\"warnings,omitempty\"`: nil is omitted, not null",
	"music_player_service.go:412:6 var tracks":             "PlayArtist returns an error when tracks is empty, so a nil slice is never serialised",
	"translation_service.go:168:6 var results":             "TranslateBatch has no HTTP caller in the tree (service-internal); tracked, not on a served path",
	"localization_handlers.go:452:6 var providers":         "unrouted: LocalizationHandlers is never constructed outside tests; not a served path today",
	"internal/handlers/media.go:280:6 var mediaItems":      "unrouted: internal/handlers.MediaHandler is never constructed outside tests; not a served path today",
	"internal/handlers/media.go:56:6 var mediaTypes":       "unrouted: internal/handlers.MediaHandler is never constructed outside tests; not a served path today",
}

func filterAccepted(got []string, used map[string]bool) []string {
	var rest []string
	for _, g := range got {
		hit := false
		for k := range accepted {
			if strings.Contains(g, k) {
				used[k] = true
				hit = true
			}
		}
		if !hit {
			rest = append(rest, g)
		}
	}
	return rest
}

func TestNoUnnormalisedNilListReturns(t *testing.T) {
	used := map[string]bool{}
	t.Cleanup(func() {
		if t.Failed() {
			return
		}
		for k, why := range accepted {
			if !used[k] {
				t.Errorf("stale accepted entry %q (%s): no longer matches a finding", k, why)
			}
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
