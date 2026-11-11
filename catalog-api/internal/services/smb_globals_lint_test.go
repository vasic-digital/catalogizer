package services

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// SMB-FU-14: lint-style test. A Test function that touches an SMB global (directly, or through a with*/rig helper) must not call
// t.Parallel(), and no test file other than the helper file may assign the globals directly.
var smbGlobalNames = map[string]bool{
	"SMBHostResolver": true, "SMBDialer": true, "smbWarnf": true, "smbWarnedRaw": true,
	"SetSMBConfiguredRoots": true, "ConfigureSMBPortsFromConfig": true, "withSMBHostResolver": true, "withSMBDialer": true, "withSMBWarnf": true,
	"withSMBConfiguredRoots": true, "resetSMBWarnedRaw": true, "withResolver": true, "smbNow": true, "withSMBNow": true, "newRebindRig": true,
}

const smbHelperFile = "smb_globals_testhelper_test.go"

// smbGlobalsViolations parses the given test sources (name -> content) and returns violations.
func smbGlobalsViolations(t *testing.T, files map[string]string) []string {
	t.Helper()
	var out []string
	fset := token.NewFileSet()
	for name, src := range files {
		f, err := parser.ParseFile(fset, name, src, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			uses, parallel, assigns := false, false, false
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				switch x := n.(type) {
				case *ast.Ident:
					if smbGlobalNames[x.Name] {
						uses = true
					}
				case *ast.CallExpr:
					if se, ok := x.Fun.(*ast.SelectorExpr); ok && se.Sel.Name == "Parallel" {
						parallel = true
					}
				case *ast.AssignStmt:
					for _, l := range x.Lhs {
						id := l
						if se, ok := l.(*ast.SelectorExpr); ok {
							id = se.Sel
						}
						if i, ok := id.(*ast.Ident); ok && (i.Name == "SMBHostResolver" || i.Name == "SMBDialer" || i.Name == "smbWarnf" || i.Name == "smbWarnedRaw" || i.Name == "smbNow") {
							assigns = true
						}
					}
				}
				return true
			})
			if strings.HasPrefix(fd.Name.Name, "Test") && uses && parallel {
				out = append(out, name+": "+fd.Name.Name+" uses SMB globals and calls t.Parallel()")
			}
			if assigns && filepath.Base(name) != smbHelperFile {
				out = append(out, name+": "+fd.Name.Name+" assigns an SMB global directly (use the with* helper)")
			}
		}
	}
	return out
}

func TestSMBGlobalsTestsNotParallel(t *testing.T) {
	files := map[string]string{}
	matches, _ := filepath.Glob("*_test.go")
	for _, m := range matches {
		if m == "smb_globals_lint_test.go" {
			continue
		}
		b, err := os.ReadFile(m)
		if err != nil {
			t.Fatal(err)
		}
		files[m] = string(b)
	}
	if len(files) < 5 {
		t.Fatalf("lint saw only %d test files: instrument blind", len(files))
	}
	if v := smbGlobalsViolations(t, files); len(v) != 0 {
		t.Fatalf("SMB-FU-14 violations:\n%s", strings.Join(v, "\n"))
	}
}

// Golden-bad / golden-good fixtures prove the lint itself can fail and does not over-fire.
func TestSMBGlobalsLint_SelfCheck(t *testing.T) {
	bad := `package p
import "testing"
func TestA(t *testing.T) { t.Parallel(); withSMBHostResolver(t, nil) }
func TestB(t *testing.T) { t.Run("x", func(t *testing.T) { t.Parallel(); _ = SMBDialer }) }
func TestC(t *testing.T) { SMBHostResolver = nil }
`
	good := `package p
import "testing"
func TestD(t *testing.T) { t.Parallel() }
func TestE(t *testing.T) { withSMBHostResolver(t, nil) }
`
	if v := smbGlobalsViolations(t, map[string]string{"bad_test.go": bad}); len(v) != 3 {
		t.Fatalf("golden-bad: want 3 violations, got %v", v)
	}
	if v := smbGlobalsViolations(t, map[string]string{"good_test.go": good}); len(v) != 0 {
		t.Fatalf("golden-good fired: %v", v)
	}
}
