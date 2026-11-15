package errleak

// ERRLEAK-FU-A: the allow-list entry for GetStorageRootStatus (`=c.JSON(http.StatusOK, resp)`) is pinned by statement, not by function, so
// the sweep alone would accept any new resp[...] = err.Error() inside that function. This test narrows the pin structurally: it parses the
// real handlers/scan_handler.go and demands that every value placed into resp is a literal, a bool, or a probeErr(...) call, that resp is only
// ever handed to c.JSON, and that probeErr's non-admin branch returns the fixed scanErrorClassText.

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func checkStorageRootStatusPin(src string) []string {
	var probs []string
	bad := func(format string, a ...any) { probs = append(probs, fmt.Sprintf(format, a...)) }
	fset := token.NewFileSet()
	af, err := parser.ParseFile(fset, "scan_handler.go", src, 0)
	if err != nil {
		return []string{"parse: " + err.Error()}
	}
	text := func(n ast.Node) string { return src[fset.Position(n.Pos()).Offset:fset.Position(n.End()).Offset] }
	line := func(n ast.Node) int { return fset.Position(n.Pos()).Line }
	var fd *ast.FuncDecl
	for _, d := range af.Decls {
		if f, ok := d.(*ast.FuncDecl); ok && f.Name.Name == "GetStorageRootStatus" && f.Body != nil {
			fd = f
		}
	}
	if fd == nil {
		return []string{"GetStorageRootStatus not found"}
	}
	isIdent := func(e ast.Expr, name string) bool { id, ok := e.(*ast.Ident); return ok && id.Name == name }
	accounted := map[*ast.Ident]bool{}
	defs, probeDefs := 0, 0
	allowedValue := func(e ast.Expr) bool {
		switch v := e.(type) {
		case *ast.BasicLit:
			return true
		case *ast.Ident:
			return v.Name == "true" || v.Name == "false"
		case *ast.CallExpr:
			return isIdent(v.Fun, "probeErr") && len(v.Args) == 1 && isIdent(v.Args[0], "err")
		}
		return false
	}
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.AssignStmt:
			for i, l := range v.Lhs {
				switch lv := l.(type) {
				case *ast.IndexExpr:
					base, ok := lv.X.(*ast.Ident)
					if !ok || base.Name != "resp" {
						continue
					}
					accounted[base] = true
					if _, lit := lv.Index.(*ast.BasicLit); !lit {
						bad("line %d: resp key is not a string literal", line(v))
					}
					if len(v.Rhs) != len(v.Lhs) || !allowedValue(v.Rhs[i]) {
						bad("line %d: resp value is not a literal, a bool or probeErr(err): %s", line(v), text(v))
					}
				case *ast.Ident:
					switch lv.Name {
					case "resp":
						accounted[lv] = true
						defs++
						cl, ok := v.Rhs[0].(*ast.CompositeLit)
						if v.Tok != token.DEFINE || len(v.Rhs) != 1 || !ok {
							bad("line %d: resp is not defined once as a composite literal", line(v))
							continue
						}
						for _, el := range cl.Elts {
							kv, ok := el.(*ast.KeyValueExpr)
							if !ok {
								bad("line %d: resp literal element is not key: value", line(el))
								continue
							}
							if !allowedValue(kv.Value) && !isIdent(kv.Value, "id") && text(kv.Value) != "time.Now().Format(time.RFC3339)" {
								bad("line %d: resp literal value not allowed: %s", line(kv), text(kv))
							}
						}
					case "probeErr":
						probeDefs++
						fl, ok := v.Rhs[0].(*ast.FuncLit)
						if v.Tok != token.DEFINE || len(v.Rhs) != 1 || !ok {
							bad("line %d: probeErr is not defined once as a closure", line(v))
							continue
						}
						st := fl.Body.List
						ifs, ok1 := (ast.Stmt)(nil), false
						if len(st) == 2 {
							ifs = st[0]
							ok1 = true
						}
						okShape := false
						if ok1 {
							is, isIf := ifs.(*ast.IfStmt)
							ret2, isRet := st[1].(*ast.ReturnStmt)
							if isIf && isRet && is.Else == nil && is.Init == nil && len(is.Body.List) == 1 {
								u, isU := is.Cond.(*ast.UnaryExpr)
								r1, isR1 := is.Body.List[0].(*ast.ReturnStmt)
								okShape = isU && u.Op == token.NOT && isIdent(u.X, "admin") && isR1 && len(r1.Results) == 1 &&
									isIdent(r1.Results[0], "scanErrorClassText") && len(ret2.Results) == 1 &&
									text(ret2.Results[0]) == "services.ScrubErrorFor(err, root)"
							}
						}
						if !okShape {
							bad("line %d: probeErr must be exactly `if !admin { return scanErrorClassText }; return services.ScrubErrorFor(err, root)`", line(v))
						}
					}
				}
			}
		case *ast.CallExpr:
			if sel, ok := v.Fun.(*ast.SelectorExpr); ok && isIdent(sel.X, "c") && sel.Sel.Name == "JSON" && len(v.Args) == 2 {
				if id, ok := v.Args[1].(*ast.Ident); ok && id.Name == "resp" {
					accounted[id] = true
				}
			}
		}
		return true
	})
	if defs != 1 {
		bad("resp defined %d times, want 1", defs)
	}
	if probeDefs != 1 {
		bad("probeErr assigned %d times, want 1", probeDefs)
	}
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && id.Name == "resp" && !accounted[id] {
			bad("line %d: resp used outside the pinned shapes (only resp[...] = literal/probeErr and c.JSON(status, resp))", line(id))
		}
		return true
	})
	return probs
}

func realScanHandler(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "handlers", "scan_handler.go"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestGetStorageRootStatusRespPinned_Real(t *testing.T) {
	if p := checkStorageRootStatusPin(realScanHandler(t)); len(p) != 0 {
		t.Fatalf("real handler violates the pin: %v", p)
	}
}

func TestGetStorageRootStatusRespPinned_Mutants(t *testing.T) {
	src := realScanHandler(t)
	mut := map[string][2]string{
		"respErrorFromErr":      {`"misconfigured", probeErr(err)`, `"misconfigured", err.Error()`},
		"extraKeyFromErr":       {`resp["connected"], resp["status"] = true, "online"`, `resp["connected"], resp["status"] = true, "online"` + "\n\tresp[\"detail\"] = fmt.Sprintf(\"%v\", err)"},
		"nonAdminReturnsRaw":    {"\t\tif !admin {\n\t\t\treturn scanErrorClassText\n\t\t}", "\t\tif !admin {\n\t\t\treturn err.Error()\n\t\t}"},
		"respHandedToHelper":    {"\tif !root.Enabled {\n\t\t// A disabled root", "\tmergeErr(resp, err)\n\tif !root.Enabled {\n\t\t// A disabled root"},
		"respReassigned":        {"\tresp := gin.H{\"id\": id, \"checked_at\": time.Now().Format(time.RFC3339)}", "\tresp := gin.H{\"id\": id, \"checked_at\": err.Error()}"},
		"extraReturnInProbeErr": {"\t\treturn services.ScrubErrorFor(err, root)\n\t}\n\tif !root.Enabled", "\t\tif err != nil {\n\t\t\treturn err.Error()\n\t\t}\n\t\treturn services.ScrubErrorFor(err, root)\n\t}\n\tif !root.Enabled"},
	}
	for name, m := range mut {
		if !strings.Contains(src, m[0]) {
			t.Fatalf("mutant %s: anchor text not found in the real handler (fixture drifted)", name)
		}
		got := checkStorageRootStatusPin(strings.Replace(src, m[0], m[1], 1))
		if len(got) == 0 {
			t.Errorf("mutant %s not detected by the pin", name)
		}
	}
}
