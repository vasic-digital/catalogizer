// gin_routes_ast.go - prints every Gin route registered from main() of one Go file, one "METHOD /path" per line.
// Stdlib only. Used by scripts/check_api_route_vs_spec.py (docs/scripts/check_api_route_vs_spec.md).
//
// Understood: gin.Default/New roots, X.Group("lit"), chained .Use(...), GET/POST/PUT/DELETE/PATCH/HEAD/OPTIONS, Any (the 7 methods
// above), Handle("LIT", path, ...), Match([]string{...}, path, ...), multi-line calls, and same-file and same-package (sibling *.go files next to FILE.go) helper functions whose
// parameter is *gin.RouterGroup / *gin.Engine / gin.IRouter / gin.IRoutes (interpreted at each call site with the caller's prefix).
// Anything that looks like a route registration but cannot be resolved is printed as "UNRESOLVED <pos> <why>" on stdout.
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

var verbs = map[string]bool{"GET": true, "POST": true, "PUT": true, "DELETE": true, "PATCH": true, "HEAD": true, "OPTIONS": true}
var fset = token.NewFileSet()
var funcs = map[string]*ast.FuncDecl{}

// ext holds the router-registering functions and methods of the extra package directories passed on the command line
// (cross-package registration such as handlers.RegisterStorageRoutes(api, ...) or h.RegisterAliasRoutes(api)).
var ext = map[string][]*ast.FuncDecl{}
var routes = map[string]bool{}
var unresolved []string

func lit(e ast.Expr) (string, bool) {
	switch v := e.(type) {
	case *ast.BasicLit:
		if v.Kind == token.STRING {
			s, err := strconv.Unquote(v.Value)
			return s, err == nil
		}
	case *ast.BinaryExpr:
		if v.Op == token.ADD {
			a, ok1 := lit(v.X)
			b, ok2 := lit(v.Y)
			return a + b, ok1 && ok2
		}
	case *ast.ParenExpr:
		return lit(v.X)
	}
	return "", false
}

func isRouterType(t ast.Expr) bool {
	s := types(t)
	return s == "*gin.RouterGroup" || s == "*gin.Engine" || s == "gin.IRouter" || s == "gin.IRoutes" || s == "gin.RouterGroup"
}

func types(t ast.Expr) string {
	switch v := t.(type) {
	case *ast.StarExpr:
		return "*" + types(v.X)
	case *ast.SelectorExpr:
		return types(v.X) + "." + v.Sel.Name
	case *ast.Ident:
		return v.Name
	}
	return ""
}

type env map[string]string

func (e env) clone() env {
	n := env{}
	for k, v := range e {
		n[k] = v
	}
	return n
}

// group resolves an expression to a route prefix.
func group(x ast.Expr, e env) (string, bool) {
	switch v := x.(type) {
	case *ast.Ident:
		p, ok := e[v.Name]
		return p, ok
	case *ast.ParenExpr:
		return group(v.X, e)
	case *ast.CallExpr:
		if sel, ok := v.Fun.(*ast.SelectorExpr); ok {
			if id, ok := sel.X.(*ast.Ident); ok && id.Name == "gin" && (sel.Sel.Name == "Default" || sel.Sel.Name == "New") {
				return "", true
			}
			if sel.Sel.Name == "Use" {
				return group(sel.X, e)
			}
			if sel.Sel.Name == "Group" && len(v.Args) > 0 {
				base, ok := group(sel.X, e)
				if !ok {
					return "", false
				}
				s, ok := lit(v.Args[0])
				if !ok {
					unresolved = append(unresolved, fmt.Sprintf("UNRESOLVED %s non-literal Group path", fset.Position(v.Pos())))
					return "", false
				}
				return base + s, true
			}
		}
	}
	return "", false
}

func add(m, p string) { routes[m+" "+p] = true }

func call(c *ast.CallExpr, e env, depth int) {
	if sel, ok := c.Fun.(*ast.SelectorExpr); ok {
		name := sel.Sel.Name
		isVerb := verbs[name]
		if isVerb || name == "Any" || name == "Handle" || name == "Match" {
			base, ok := group(sel.X, e)
			if !ok {
				if len(c.Args) > 0 {
					if s, ok := lit(c.Args[0]); ok && strings.HasPrefix(s, "/") {
						unresolved = append(unresolved, fmt.Sprintf("UNRESOLVED %s %s %q on unknown receiver", fset.Position(c.Pos()), name, s))
					}
				}
				return
			}
			switch name {
			case "Handle":
				if len(c.Args) < 2 {
					return
				}
				m, ok1 := lit(c.Args[0])
				p, ok2 := lit(c.Args[1])
				if !ok1 || !ok2 {
					unresolved = append(unresolved, fmt.Sprintf("UNRESOLVED %s Handle with non-literal method/path", fset.Position(c.Pos())))
					return
				}
				add(strings.ToUpper(m), base+p)
			case "Match":
				if len(c.Args) < 2 {
					return
				}
				cl, ok1 := c.Args[0].(*ast.CompositeLit)
				p, ok2 := lit(c.Args[1])
				if !ok1 || !ok2 {
					unresolved = append(unresolved, fmt.Sprintf("UNRESOLVED %s Match with non-literal methods/path", fset.Position(c.Pos())))
					return
				}
				for _, el := range cl.Elts {
					if m, ok := lit(el); ok {
						add(strings.ToUpper(m), base+p)
					}
				}
			default:
				if len(c.Args) == 0 {
					return
				}
				p, ok := lit(c.Args[0])
				if !ok {
					unresolved = append(unresolved, fmt.Sprintf("UNRESOLVED %s %s with non-literal path", fset.Position(c.Pos()), name))
					return
				}
				if name == "Any" {
					for m := range verbs {
						add(m, base+p)
					}
				} else {
					add(name, base+p)
				}
			}
			return
		}
	}
	// helper call: f(group, ...) for a same-file function, or pkg.F(group, ...) / x.M(group, ...) resolved by name among the
	// extra package directories, with router-typed params
	var name string
	var cands []*ast.FuncDecl
	if id, ok := c.Fun.(*ast.Ident); ok {
		name = id.Name
		if fd := funcs[name]; fd != nil {
			cands = append(cands, fd)
		}
	} else if sel, ok := c.Fun.(*ast.SelectorExpr); ok {
		name = sel.Sel.Name
		cands = append(cands, ext[name]...)
	}
	if name == "" || depth >= 20 {
		return
	}
	anyBound := false
	for _, fd := range cands {
		if fd.Body == nil {
			continue
		}
		ne := env{}
		bound := false
		i := 0
		for _, f := range fd.Type.Params.List {
			names := f.Names
			if len(names) == 0 {
				names = []*ast.Ident{nil}
			}
			for _, n := range names {
				if i < len(c.Args) && n != nil && isRouterType(f.Type) {
					if p, ok := group(c.Args[i], e); ok {
						ne[n.Name] = p
						bound = true
					}
				}
				i++
			}
		}
		if bound {
			anyBound = true
			walk(fd.Body, ne, depth+1)
		}
	}
	// a resolved router group handed to a callee we cannot see must not read as "no routes there": report it
	if !anyBound && name != "Use" && name != "Group" && !verbs[name] && name != "Any" && name != "Handle" && name != "Match" {
		for _, a := range c.Args {
			if id, ok := a.(*ast.Ident); ok {
				if _, isGroup := e[id.Name]; isGroup {
					unresolved = append(unresolved, fmt.Sprintf("UNRESOLVED %s router group %q passed to %s, which is not defined in the scanned packages", fset.Position(c.Pos()), id.Name, name))
					return
				}
			}
		}
	}
}

func walk(body ast.Node, e env, depth int) {
	e = e.clone()
	ast.Inspect(body, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.AssignStmt:
			if len(v.Lhs) == len(v.Rhs) {
				for i := range v.Lhs {
					if id, ok := v.Lhs[i].(*ast.Ident); ok {
						if p, ok := group(v.Rhs[i], e); ok {
							e[id.Name] = p
						}
					}
				}
			}
		case *ast.ValueSpec:
			if len(v.Names) == len(v.Values) {
				for i := range v.Names {
					if p, ok := group(v.Values[i], e); ok {
						e[v.Names[i].Name] = p
					}
				}
			}
		case *ast.CallExpr:
			call(v, e, depth)
		}
		return true
	})
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: gin_routes_ast FILE.go [PKGDIR...]")
		os.Exit(2)
	}
	f, err := parser.ParseFile(fset, os.Args[1], nil, 0)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	for _, d := range f.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok && fd.Recv == nil {
			funcs[fd.Name.Name] = fd
		}
	}
	for _, dir := range os.Args[2:] {
		ents, err := os.ReadDir(dir)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		for _, en := range ents {
			if en.IsDir() || !strings.HasSuffix(en.Name(), ".go") || strings.HasSuffix(en.Name(), "_test.go") {
				continue
			}
			pf, err := parser.ParseFile(fset, dir+"/"+en.Name(), nil, 0)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(2)
			}
			for _, d := range pf.Decls {
				if fd, ok := d.(*ast.FuncDecl); ok {
					ext[fd.Name.Name] = append(ext[fd.Name.Name], fd)
				}
			}
		}
	}
	// same-package helpers: every other non-test .go file in the directory of FILE.go declaring the same package
	// (e.g. main_wsroute.go next to main.go). A function already declared in FILE.go wins.
	dir := filepath.Dir(os.Args[1])
	if ents, err := os.ReadDir(dir); err == nil {
		for _, en := range ents {
			full := filepath.Join(dir, en.Name())
			if en.IsDir() || !strings.HasSuffix(en.Name(), ".go") || strings.HasSuffix(en.Name(), "_test.go") || filepath.Base(full) == filepath.Base(os.Args[1]) {
				continue
			}
			pf, err := parser.ParseFile(fset, full, nil, 0)
			if err != nil || pf.Name.Name != f.Name.Name {
				continue
			}
			for _, d := range pf.Decls {
				if fd, ok := d.(*ast.FuncDecl); ok && fd.Recv == nil && funcs[fd.Name.Name] == nil {
					funcs[fd.Name.Name] = fd
				}
			}
		}
	}
	m := funcs["main"]
	if m == nil || m.Body == nil {
		fmt.Fprintln(os.Stderr, "no func main in file")
		os.Exit(2)
	}
	walk(m.Body, env{}, 0)
	var out []string
	for r := range routes {
		out = append(out, r)
	}
	sort.Strings(out)
	out = append(out, unresolved...)
	fmt.Println(strings.Join(out, "\n"))
}
