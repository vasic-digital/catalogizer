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
var scanDirs = []string{"../../repository", "../../services", "../../handlers", "../../internal/services"}

func exportedOnly(dir string) bool { return !strings.HasSuffix(dir, "repository") }

// offenders returns "file:line var" for every `var x []T` / `var x map[K]V` declaration without initialiser whose name is
// returned unchanged by a return statement of the same function.
func offenders(t *testing.T, dir string, exported bool) []string {
	t.Helper()
	fset := token.NewFileSet()
	files, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil || len(files) == 0 {
		t.Fatalf("control needle: no Go files found in %s", dir)
	}
	var out []string
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
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
							if len(vs.Values) != 0 {
								continue
							}
							switch vs.Type.(type) {
							case *ast.ArrayType, *ast.MapType:
								if at, ok := vs.Type.(*ast.ArrayType); ok && at.Len != nil {
									continue // fixed-size array is never nil
								}
								for _, nm := range vs.Names {
									decl[nm.Name] = nm.Pos()
								}
							}
						}
					}
				}
				return true
			})
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				if rs, ok := n.(*ast.ReturnStmt); ok {
					for _, r := range rs.Results {
						if id, ok := r.(*ast.Ident); ok {
							if pos, ok := decl[id.Name]; ok {
								out = append(out, fset.Position(pos).String()+" var "+id.Name+" returned unnormalised in "+fd.Name.Name)
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

func TestNoUnnormalisedNilListReturns(t *testing.T) {
	for _, d := range scanDirs {
		if got := offenders(t, d, exportedOnly(d)); len(got) != 0 {
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
