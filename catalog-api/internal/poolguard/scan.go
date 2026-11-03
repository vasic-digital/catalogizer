// Package poolguard holds the AST scanner behind the class-level guard for the
// "nested pooled query while an outer rows cursor is open" pool-deadlock class.
//
// The class: a function iterates `for rows.Next()` and, inside the loop body, runs
// another query/exec through the connection pool while the outer cursor still holds
// its pooled connection. With more concurrent callers than pool connections every
// caller holds one connection and waits for a second: a deadlock that ends only when
// the client gives up (see repository/file_repository_pool_test.go).
package poolguard

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
)

// queryMethods are the database/sql (and wrapper) methods that take a pooled connection.
var queryMethods = map[string]bool{
	"Query": true, "QueryRow": true, "Exec": true,
	"QueryContext": true, "QueryRowContext": true, "ExecContext": true,
	"Prepare": true, "PrepareContext": true, "InsertReturningID": true,
}

// Finding is one occurrence of a pooled call inside a `for X.Next()` body.
type Finding struct {
	File     string // path relative to the scanned root
	Func     string // "Recv.Method" or "Func"
	Line     int    // line of the offending call
	Call     string // method name of the pooled call (direct) or the helper name (indirect)
	Receiver string // source text of the receiver of the pooled call
	Indirect bool   // true when the pooled call is inside a helper called from the loop body
	Via      string // helper chain for indirect findings
}

// Key is the allowlist key: file:function.
func (f Finding) Key() string { return f.File + ":" + f.Func }

func (f Finding) String() string {
	k := "direct"
	if f.Indirect {
		k = "via " + f.Via
	}
	return fmt.Sprintf("%s:%d %s: %s on %q (%s)", f.File, f.Line, f.Func, f.Call, f.Receiver, k)
}

type decl struct {
	name   string
	dir    string
	fn     *ast.FuncDecl
	fset   *token.FileSet
	direct bool // contains a pooled call anywhere in its body
	calls  []string
}

func pkgKey(cross bool, dir, name string) string {
	if cross {
		return name
	}
	return dir + "\x00" + name
}

func funcName(fn *ast.FuncDecl) string {
	if fn.Recv != nil && len(fn.Recv.List) == 1 {
		t := fn.Recv.List[0].Type
		if s, ok := t.(*ast.StarExpr); ok {
			t = s.X
		}
		if id, ok := t.(*ast.Ident); ok {
			return id.Name + "." + fn.Name.Name
		}
	}
	return fn.Name.Name
}

func exprText(fset *token.FileSet, e ast.Expr) string {
	switch v := e.(type) {
	case *ast.Ident:
		return v.Name
	case *ast.SelectorExpr:
		return exprText(fset, v.X) + "." + v.Sel.Name
	case *ast.CallExpr:
		return exprText(fset, v.Fun) + "()"
	case *ast.StarExpr:
		return "*" + exprText(fset, v.X)
	case *ast.ParenExpr:
		return exprText(fset, v.X)
	case *ast.IndexExpr:
		return exprText(fset, v.X) + "[]"
	}
	return fmt.Sprintf("%T", e)
}

func isNextCall(e ast.Expr) bool {
	c, ok := e.(*ast.CallExpr)
	if !ok {
		return false
	}
	s, ok := c.Fun.(*ast.SelectorExpr)
	return ok && s.Sel.Name == "Next" && len(c.Args) == 0
}

// pooledCall returns (method, receiver text, true) when n is a pooled call.
func pooledCall(fset *token.FileSet, n ast.Node) (string, string, bool) {
	c, ok := n.(*ast.CallExpr)
	if !ok {
		return "", "", false
	}
	s, ok := c.Fun.(*ast.SelectorExpr)
	if !ok || !queryMethods[s.Sel.Name] {
		return "", "", false
	}
	return s.Sel.Name, exprText(fset, s.X), true
}

func calleeName(c *ast.CallExpr) string {
	switch f := c.Fun.(type) {
	case *ast.Ident:
		return f.Name
	case *ast.SelectorExpr:
		return f.Sel.Name
	}
	return ""
}

// Scan walks root (a directory) and returns every finding in non-test Go files.
// Directories named testdata, vendor, node_modules and hidden dirs are skipped
// unless they are the root itself.
func Scan(root string) ([]Finding, error) { return scan(root, false) }

// ScanCrossPackage additionally resolves helpers by bare name across every package
// (noisy; used for exploration and for the class-completeness census).
func ScanCrossPackage(root string) ([]Finding, error) { return scan(root, true) }

func scan(root string, cross bool) ([]Finding, error) {
	var files []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			n := d.Name()
			if p != root && (n == "testdata" || n == "vendor" || n == "node_modules" || strings.HasPrefix(n, ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(p, ".go") && !strings.HasSuffix(p, "_test.go") {
			files = append(files, p)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)

	fset := token.NewFileSet()
	type parsed struct {
		path string
		f    *ast.File
	}
	var all []parsed
	byPkgName := map[string][]*decl{} // dir + "\x00" + bare func name -> decls
	for _, p := range files {
		f, err := parser.ParseFile(fset, p, nil, 0)
		if err != nil {
			return nil, err
		}
		all = append(all, parsed{p, f})
		for _, dcl := range f.Decls {
			fn, ok := dcl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			d := &decl{name: funcName(fn), dir: filepath.Dir(p), fn: fn, fset: fset}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				if _, _, ok := pooledCall(fset, n); ok {
					d.direct = true
				}
				if c, ok := n.(*ast.CallExpr); ok {
					if nm := calleeName(c); nm != "" {
						d.calls = append(d.calls, nm)
					}
				}
				return true
			})
			k := pkgKey(cross, d.dir, fn.Name.Name)
			byPkgName[k] = append(byPkgName[k], d)
		}
	}

	// reaches reports (chain, line-less) whether a helper named name in dir transitively
	// runs a pooled call. depth-limited to avoid cycles.
	var reaches func(dir, name string, seen map[string]bool, depth int) (string, bool)
	reaches = func(dir, name string, seen map[string]bool, depth int) (string, bool) {
		k := pkgKey(cross, dir, name)
		if seen[k] || depth > 3 {
			return "", false
		}
		seen[k] = true
		for _, d := range byPkgName[k] {
			if d.direct {
				return d.name, true
			}
			for _, c := range d.calls {
				if ch, ok := reaches(dir, c, seen, depth+1); ok {
					return d.name + " -> " + ch, true
				}
			}
		}
		return "", false
	}

	var out []Finding
	for _, pf := range all {
		rel, _ := filepath.Rel(root, pf.path)
		for _, dcl := range pf.f.Decls {
			fn, ok := dcl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			fname := funcName(fn)
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				loop, ok := n.(*ast.ForStmt)
				if !ok {
					return true
				}
				cursorLoop := loop.Cond != nil && isNextCall(loop.Cond)
				if !cursorLoop && loop.Cond == nil {
					// `for { if !rows.Next() { break } ... }`
					for _, st := range loop.Body.List {
						if is, ok := st.(*ast.IfStmt); ok {
							if u, ok := is.Cond.(*ast.UnaryExpr); ok && u.Op == token.NOT && isNextCall(u.X) {
								cursorLoop = true
							}
						}
					}
				}
				if !cursorLoop {
					return true
				}
				seenHelper := map[string]bool{}
				ast.Inspect(loop.Body, func(m ast.Node) bool {
					if meth, recv, ok := pooledCall(fset, m); ok {
						out = append(out, Finding{File: rel, Func: fname, Line: fset.Position(m.Pos()).Line, Call: meth, Receiver: recv})
					} else if c, ok := m.(*ast.CallExpr); ok {
						nm := calleeName(c)
						if nm != "" && !seenHelper[nm] {
							if ch, ok := reaches(filepath.Dir(pf.path), nm, map[string]bool{}, 0); ok {
								seenHelper[nm] = true
								out = append(out, Finding{File: rel, Func: fname, Line: fset.Position(m.Pos()).Line, Call: nm, Receiver: exprText(fset, c.Fun), Indirect: true, Via: ch})
							}
						}
					}
					return true
				})
				return true
			})
		}
	}
	return out, nil
}
