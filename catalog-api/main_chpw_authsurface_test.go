package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// CAT-CHPW-RL-2: the AST scan is now only the DENY-BY-DEFAULT BACKSTOP behind the
// runtime tier (TestBuildRouterAuthSurface walks the real engine buildRouter
// returns and pins every auth route's full handler chain; main() additionally
// verifies the finished engine's auth route table at startup). The scan does not
// try to enumerate the ways a route or middleware can reach the auth group;
// instead, in every non-test file of package main it refuses, class-wide:
//   - any selector access to gin's `.Handlers` / `.RouterGroup` fields;
//   - any method value of Use / Group / GET / POST / PUT / PATCH / DELETE / HEAD /
//     OPTIONS / Any / Handle / Match / Static* (called directly or nothing);
//   - any mention of gin.Engine / RouterGroup / IRoutes / IRouter outside the six
//     sanctioned helper functions;
//   - any use of the engine value (`router`, or a gin-typed parameter) other than
//     as a method-call receiver, `return`, `Handler:` or an argument of a
//     sanctioned helper;
//   - any registration call (all 13 methods and Group) whose path argument cannot
//     be constant-folded, or whose folded path, joined with the receiver's folded
//     group prefix exactly as gin joins (path.Join), is under /api/v1/auth, /auth
//     or a segment prefix of /api/v1/auth other than /api/v1; a registration on a
//     receiver whose prefix cannot be resolved outside the sanctioned helpers;
//   - `.Use` on a receiver whose prefix cannot be resolved, on a group under the
//     auth prefix, or on the engine anywhere but the recorded pre-group list in
//     buildRouter and main()'s late Alt-Svc middleware.
// A tree that matches nothing fails closed. Unit-class source scan, never runs main().

// nonTestFiles parses every non-test .go file of dir.
func nonTestFiles(t *testing.T, dir string) (*token.FileSet, map[string]*ast.File) {
	t.Helper()
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, func(fi os.FileInfo) bool { return !strings.HasSuffix(fi.Name(), "_test.go") }, 0)
	require.NoError(t, err, "parse failure must fail the test")
	pkg := pkgs["main"]
	require.NotNil(t, pkg, "package main not found in "+dir)
	require.NotEmpty(t, pkg.Files)
	return fset, pkg.Files
}

func buildBinds(files map[string]*ast.File) map[string]string {
	binds := map[string]string{}
	for pass := 0; pass < 3; pass++ {
		for _, f := range files {
			ast.Inspect(f, func(n ast.Node) bool {
				switch v := n.(type) {
				case *ast.ValueSpec:
					for i, name := range v.Names {
						if i < len(v.Values) {
							if s, ok := foldString(v.Values[i], binds); ok {
								binds[name.Name] = s
							}
						}
					}
				case *ast.AssignStmt:
					if len(v.Lhs) == len(v.Rhs) {
						for i, l := range v.Lhs {
							if id, ok := l.(*ast.Ident); ok {
								if s, ok := foldString(v.Rhs[i], binds); ok {
									binds[id.Name] = s
								}
							}
						}
					}
				}
				return true
			})
		}
	}
	return binds
}

const authPrefix = "api/v1/auth"

func segHasPrefix(q, p string) bool { return q == p || strings.HasPrefix(q, p+"/") }
func trimSlashes(s string) string   { return strings.Trim(s, "/") }

func exprText(fset *token.FileSet, e ast.Expr) string {
	var sb strings.Builder
	_ = printer.Fprint(&sb, fset, e)
	return sb.String()
}

func calleeDesc(fset *token.FileSet, e ast.Expr) string {
	switch v := e.(type) {
	case *ast.CallExpr:
		return exprText(fset, v.Fun)
	case *ast.FuncLit:
		return "<funclit>"
	}
	return exprText(fset, e)
}

// ginJoin joins a group prefix and a relative path the way gin does (path.Join,
// which also cleans "//", "." and ".."), lower-cased and without outer slashes.
func ginJoin(prefix, rel string) string {
	return strings.ToLower(strings.Trim(path.Join("/"+prefix, rel), "/"))
}

var (
	// Registration methods of gin groups/engines and the index of their path argument.
	regPathIdx = map[string]int{
		"GET": 0, "POST": 0, "PUT": 0, "PATCH": 0, "DELETE": 0, "HEAD": 0, "OPTIONS": 0, "Any": 0,
		"Handle": 1, "Match": 1, "Static": 0, "StaticFS": 0, "StaticFile": 0, "StaticFileFS": 0, "Group": 0,
	}
	// Functions allowed to name gin's engine/group types (they ARE the wiring).
	sanctionedFuncs = map[string]bool{
		"buildRouter": true, "registerAuthRoutes": true, "registerChangePasswordRoute": true,
		"registerWebSocketRoute": true, "applyTrustedProxies": true, "setTrustedProxies": true, "verifyAuthRouteTable": true,
	}
	// Callees an engine/group-typed value may be handed to.
	handoffCallees = map[string]bool{
		"applyTrustedProxies": true, "setTrustedProxies": true, "registerWebSocketRoute": true,
		"registerAuthRoutes": true, "registerChangePasswordRoute": true, "root_middleware.RegisterChangePassword": true,
		"verifyAuthRouteTable": true,
	}
	// Engine-level middleware buildRouter installs before the auth group exists
	// (the runtime tier pins the resulting chains; this list is the cheap source-level twin).
	allowedRouterUseBeforeAuth = []string{
		"gin.LoggerWithFormatter,middleware.Recovery",
		"root_middleware.SecurityHeaders",
		"root_middleware.ConcurrencyLimiter",
		"root_middleware.RequestTimeout",
		"root_middleware.CORS",
		"metrics.GinMiddleware",
		"middleware.Logger",
		"middleware.ErrorHandler",
		"firebase.GinMiddleware",
		"root_middleware.RequestID",
		"root_middleware.InputValidation",
		"middleware.CompressionMiddleware",
	}
	// authGroup identifier count per file: definition + registerAuthRoutes arg in
	// main_router.go; parameter + RegisterChangePassword arg in main_chpw.go.
	wantAuthGroupIdents = map[string]int{"main_router.go": 2, "main_chpw.go": 2}
	// Groups whose folded path is a proper segment prefix of /api/v1/auth.
	allowedPrefixGroups = map[string]int{"api/v1": 1}
)

func isGinTypeSel(se *ast.SelectorExpr) bool {
	id, ok := se.X.(*ast.Ident)
	if !ok || id.Name != "gin" {
		return false
	}
	switch se.Sel.Name {
	case "Engine", "RouterGroup", "IRoutes", "IRouter":
		return true
	}
	return false
}

// scanAuthSurface returns the violations over the parsed files.
func scanAuthSurface(fset *token.FileSet, files map[string]*ast.File) []string {
	var out []string
	add := func(format string, a ...any) { out = append(out, fmt.Sprintf(format, a...)) }
	binds := buildBinds(files)
	byBase := map[string]*ast.File{}
	for n, f := range files {
		byBase[filepath.Base(n)] = f
	}
	files = byBase
	var names []string
	for n := range files {
		names = append(names, n)
	}
	sort.Strings(names)

	// Group prefixes by variable name across all files ("" is the engine root;
	// ambiguous or unresolvable names are absent).
	prefixes := map[string]string{}
	ambiguous := map[string]bool{}
	var resolve func(e ast.Expr) (string, bool)
	resolve = func(e ast.Expr) (string, bool) {
		switch v := e.(type) {
		case *ast.ParenExpr:
			return resolve(v.X)
		case *ast.Ident:
			if ambiguous[v.Name] {
				return "", false
			}
			p, ok := prefixes[v.Name]
			return p, ok
		case *ast.CallExpr:
			if se, ok := v.Fun.(*ast.SelectorExpr); ok && se.Sel.Name == "Group" && len(v.Args) == 1 {
				base, ok := resolve(se.X)
				rel, ok2 := foldString(v.Args[0], binds)
				if ok && ok2 {
					return ginJoin(base, rel), true
				}
			}
		}
		return "", false
	}
	setPrefix := func(name, p string) {
		if old, ok := prefixes[name]; ok && old != p {
			ambiguous[name] = true
		}
		prefixes[name] = p
	}
	for pass := 0; pass < 4; pass++ {
		for _, n := range names {
			ast.Inspect(files[n], func(x ast.Node) bool {
				var lhs []ast.Expr
				var rhs []ast.Expr
				switch v := x.(type) {
				case *ast.AssignStmt:
					lhs, rhs = v.Lhs, v.Rhs
				case *ast.ValueSpec:
					for _, nm := range v.Names {
						lhs = append(lhs, nm)
					}
					rhs = v.Values
				}
				if len(lhs) != len(rhs) {
					return true
				}
				for i := range lhs {
					id, ok := lhs[i].(*ast.Ident)
					if !ok {
						continue
					}
					if c, ok := rhs[i].(*ast.CallExpr); ok {
						if t := exprText(fset, c.Fun); (t == "gin.New" || t == "buildRouter") && id.Name == "router" {
							setPrefix("router", "")
							continue
						}
					}
					if p, ok := resolve(rhs[i]); ok {
						if _, isCall := rhs[i].(*ast.CallExpr); isCall {
							setPrefix(id.Name, p)
						}
					}
				}
				return true
			})
		}
	}

	defines := 0
	sanctionedDef := map[*ast.CallExpr]bool{}
	prefixGroups := map[string]int{}
	identCount := map[string]int{}
	matched := 0

	for _, name := range names {
		f := files[name]
		// authGroup: never a package-level declaration.
		for _, d := range f.Decls {
			if gd, ok := d.(*ast.GenDecl); ok {
				for _, sp := range gd.Specs {
					if vs, ok := sp.(*ast.ValueSpec); ok {
						for _, n := range vs.Names {
							if n.Name == "authGroup" {
								add("%s: package-level declaration of authGroup is forbidden", name)
							}
						}
					}
				}
			}
		}
		ast.Inspect(f, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok && id.Name == "authGroup" {
				identCount[name]++
			}
			return true
		})

		// Per-function walk.
		walkFn := func(fname string, root ast.Node, params *ast.FieldList) {
			sanctioned := sanctionedFuncs[fname]
			callee := map[*ast.SelectorExpr]bool{}
			okIdent := map[*ast.Ident]bool{}
			engineIdents := map[string]bool{"router": true}
			if params != nil {
				for _, fl := range params.List {
					for _, nm := range fl.Names {
						okIdent[nm] = true
						ast.Inspect(fl.Type, func(x ast.Node) bool {
							if se, ok := x.(*ast.SelectorExpr); ok && isGinTypeSel(se) {
								engineIdents[nm.Name] = true
							}
							return true
						})
					}
				}
			}
			ast.Inspect(root, func(n ast.Node) bool {
				switch v := n.(type) {
				case *ast.CallExpr:
					if se, ok := v.Fun.(*ast.SelectorExpr); ok {
						callee[se] = true
						if id, ok := se.X.(*ast.Ident); ok {
							okIdent[id] = true
						}
					}
					if handoffCallees[exprText(fset, v.Fun)] {
						for _, a := range v.Args {
							if id, ok := a.(*ast.Ident); ok {
								okIdent[id] = true
							}
						}
					}
				case *ast.SelectorExpr:
					if id, ok := v.X.(*ast.Ident); ok && !callee[v] {
						// non-call selector on the engine (field read/write or method value):
						// flagged below by name; the X ident itself is acceptable here.
						okIdent[id] = true
					}
				case *ast.AssignStmt:
					if v.Tok == token.DEFINE && len(v.Lhs) == 1 && len(v.Rhs) == 1 {
						if id, ok := v.Lhs[0].(*ast.Ident); ok {
							okIdent[id] = true
						}
					}
				case *ast.KeyValueExpr:
					if k, ok := v.Key.(*ast.Ident); ok && k.Name == "Handler" {
						if id, ok := v.Value.(*ast.Ident); ok {
							okIdent[id] = true
						}
					}
				case *ast.ReturnStmt:
					for _, r := range v.Results {
						if id, ok := r.(*ast.Ident); ok {
							okIdent[id] = true
						}
					}
				}
				return true
			})
			// callee set must be complete before the second walk.
			ast.Inspect(root, func(n ast.Node) bool {
				switch v := n.(type) {
				case *ast.Ident:
					if engineIdents[v.Name] && !okIdent[v] {
						add("%s: %s:%d: engine/group value %q used outside a method-call receiver, return, Handler: or sanctioned helper argument (alias/handoff)", fname, name, fset.Position(v.Pos()).Line, v.Name)
					}
				case *ast.SelectorExpr:
					line := fset.Position(v.Pos()).Line
					if v.Sel.Name == "Handlers" || v.Sel.Name == "RouterGroup" {
						add("%s:%d: direct access to gin's .%s field", name, line, v.Sel.Name)
					}
					if _, isReg := regPathIdx[v.Sel.Name]; (isReg || v.Sel.Name == "Use") && !callee[v] {
						add("%s:%d: method value %s of Use/Group/registration methods is forbidden (could register or add middleware outside the checked call shapes)", name, line, exprText(fset, v))
					}
					if isGinTypeSel(v) && !sanctioned {
						add("%s:%d: %s mentioned outside the sanctioned helper functions", name, line, exprText(fset, v))
					}
				case *ast.CallExpr:
					se, ok := v.Fun.(*ast.SelectorExpr)
					if !ok {
						return true
					}
					method := se.Sel.Name
					line := fset.Position(v.Pos()).Line
					if method == "Use" {
						matched++
						base, known := resolve(se.X)
						switch {
						case !known:
							add("%s:%d: %s.Use on a receiver whose group prefix cannot be resolved (could be the auth group)", name, line, exprText(fset, se.X))
						case base == "" && fname == "buildRouter":
							var ds []string
							for _, a := range v.Args {
								ds = append(ds, calleeDesc(fset, a))
							}
							d := strings.Join(ds, ",")
							if v.Ellipsis.IsValid() {
								d += "..."
							}
							okU := false
							for _, a := range allowedRouterUseBeforeAuth {
								if a == d {
									okU = true
								}
							}
							if !okU {
								add("%s:%d: router.Use(%s) in buildRouter is not on the recorded pre-group allow-list", name, line, d)
							}
						case base == "" && name == "main.go" && fname == "main":
						case base == "":
							add("%s:%d: engine .Use outside buildRouter / main()", name, line)
						case segHasPrefix(base, authPrefix):
							add("%s:%d: .Use on a group under the auth prefix", name, line)
						}
						return true
					}
					idx, isReg := regPathIdx[method]
					if !isReg {
						return true
					}
					matched++
					if len(v.Args) <= idx {
						add("%s:%d: %s call without a path argument; fail closed", name, line, method)
						return true
					}
					rel, folded := foldString(v.Args[idx], binds)
					if !folded {
						add("%s:%d: %s path argument cannot be constant-folded; fail closed", name, line, method)
						return true
					}
					base, known := resolve(se.X)
					if !known && !sanctioned {
						add("%s:%d: %s(%q) on a receiver whose group prefix cannot be resolved outside the sanctioned helpers", name, line, method, rel)
						return true
					}
					full := ginJoin(base, rel)
					if sanctionedDef[v] {
						// the one sanctioned definition (shape checked where it is assigned)
						return true
					}
					if segHasPrefix(full, authPrefix) || full == "auth" || strings.HasPrefix(full, "auth/") || (!known && (segHasPrefix(strings.ToLower(strings.Trim(rel, "/")), "auth"))) {
						add("%s:%d: %s(%q) registers under the auth surface outside registerAuthRoutes", name, line, method, rel)
					}
					if method == "Group" && full != "" && segHasPrefix(authPrefix, full) && full != authPrefix {
						prefixGroups[full]++
					}
				}
				return true
			})
		}
		for _, d := range f.Decls {
			switch fd := d.(type) {
			case *ast.FuncDecl:
				if fd.Body == nil {
					continue
				}
				// authGroup definition shape (first so Use ordering can refer to its position).
				ast.Inspect(fd.Body, func(n ast.Node) bool {
					as, ok := n.(*ast.AssignStmt)
					if !ok {
						return true
					}
					for _, l := range as.Lhs {
						id, ok := l.(*ast.Ident)
						if !ok || id.Name != "authGroup" {
							continue
						}
						defines++
						okShape := as.Tok == token.DEFINE && name == "main_router.go" && fd.Name.Name == "buildRouter" && len(as.Lhs) == 1 && len(as.Rhs) == 1
						if okShape {
							c, isCall := as.Rhs[0].(*ast.CallExpr)
							if isCall {
								if _, isG := isCallSel(c, "router", "Group"); isG && len(c.Args) == 1 {
									if p, ok := foldString(c.Args[0], binds); ok && trimSlashes(p) == authPrefix {
										sanctionedDef[c] = true
										continue
									}
								}
							}
						}
						add("%s: authGroup must be `authGroup := router.Group(\"/api/v1/auth\")` inside buildRouter of main_router.go (found `%s`)", name, as.Tok)
					}
					return true
				})
				walkFn(fd.Name.Name, fd.Body, fd.Type.Params)
				{
					for _, fl := range []*ast.FieldList{fd.Recv, fd.Type.Params, fd.Type.Results} {
						if fl == nil {
							continue
						}
						ast.Inspect(fl, func(x ast.Node) bool {
							if se, ok := x.(*ast.SelectorExpr); ok && isGinTypeSel(se) && !sanctionedFuncs[fd.Name.Name] {
								add("%s:%d: %s in the signature of %s, outside the sanctioned helpers", name, fset.Position(se.Pos()).Line, exprText(fset, se), fd.Name.Name)
							}
							return true
						})
					}
				}
			case *ast.GenDecl:
				walkFn("", d, nil)
			}
		}
	}
	if defines != 1 {
		add("authGroup must be defined exactly once, found %d", defines)
	}
	for _, name := range names {
		if identCount[name] != wantAuthGroupIdents[name] {
			add("%s: authGroup identifier count %d, want %d (any extra use is an alias, hook, Use or Group on the auth group)", name, identCount[name], wantAuthGroupIdents[name])
		}
	}
	for q, n := range prefixGroups {
		if allowedPrefixGroups[q] != n {
			add("Group(%q) is a segment prefix of /api/v1/auth, %d time(s), allowed %d (nested/folded prefix)", q, n, allowedPrefixGroups[q])
		}
	}
	if prefixGroups["api/v1"] == 0 || matched < 20 {
		add("expected groups/registrations not found (api/v1 group %d, matched calls %d): scan matched nothing, fail closed", prefixGroups["api/v1"], matched)
	}
	return out
}

func TestMainPackageAuthSurfaceClosed(t *testing.T) {
	fset, files := nonTestFiles(t, ".")
	require.Contains(t, files, "main.go")
	require.Contains(t, files, "main_chpw.go")
	require.Contains(t, files, "main_router.go")
	require.Empty(t, scanAuthSurface(fset, files))
}

// mutate copies every non-test .go file to a fresh temp dir, applies fn, and
// returns the scan result. All writes stay inside t.TempDir().
func mutate(t *testing.T, fn func(dir string)) []string {
	t.Helper()
	root := t.TempDir()
	require.NotEmpty(t, root)
	require.True(t, strings.HasPrefix(root, os.TempDir()), "mutation root must be under the temp dir: %s", root)
	srcs, err := filepath.Glob("*.go")
	require.NoError(t, err)
	n := 0
	for _, s := range srcs {
		if strings.HasSuffix(s, "_test.go") {
			continue
		}
		b, err := os.ReadFile(s)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(root, s), b, 0o600))
		n++
	}
	require.Greater(t, n, 5)
	fn(root)
	fset, files := nonTestFiles(t, root)
	return scanAuthSurface(fset, files)
}

// edit replaces exactly one occurrence of old in dir/file (control: the mutator
// must have applied, otherwise the mutant would be a no-op).
func edit(t *testing.T, dir, file, old, repl string) {
	t.Helper()
	p := filepath.Join(dir, file)
	b, err := os.ReadFile(p)
	require.NoError(t, err)
	require.Equal(t, 1, strings.Count(string(b), old), "mutation anchor must occur exactly once in "+file+": "+old)
	require.NoError(t, os.WriteFile(p, []byte(strings.Replace(string(b), old, repl, 1)), 0o600))
}

func addFile(t *testing.T, dir, name, src string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("package main\n"+src), 0o600))
}
func TestAuthSurfaceGuard_Mutants(t *testing.T) {
	const defn = "authGroup := router.Group(\"/api/v1/auth\")"
	const recDefn = "recGroup := api.Group(\"/recommendations\")"
	const setTP = "if err := setTrustedProxies(r, tp); err != nil {"
	cases := []struct {
		name string
		want string // substring that must appear in a finding
		fn   func(dir string)
	}{
		// Round-4 reviewer mutants, adopted as named acceptance mutants.
		{"RV1_method_value_router_Use_before_group", "method value", func(d string) {
			edit(t, d, "main_router.go", defn, "u := router.Use\n\tu(evil())\n\t"+defn)
		}},
		{"RV2_method_value_router_Group_auth_route", "method value", func(d string) {
			edit(t, d, "main_router.go", defn, defn+"\n\tgrp := router.Group\n\tgrp(\"/api/v1/auth\").POST(\"/evil\", evil)")
		}},
		{"RV3_unclean_path_double_slash", "registers under the auth surface", func(d string) {
			edit(t, d, "main_router.go", defn, defn+"\n\trouter.POST(\"/api//v1/auth/evil\", evil)")
		}},
		{"RV3b_unclean_path_double_slash_in_main", "registers under the auth surface", func(d string) {
			edit(t, d, "main.go", recDefn, "router.POST(\"/api//v1/auth/evil\", evil)\n\t"+recDefn)
		}},
		{"RV4_relative_dot_path_under_api", "registers under the auth surface", func(d string) {
			edit(t, d, "main.go", recDefn, "api.GET(\"./auth/evil\", evil)\n\t"+recDefn)
		}},
		{"RV5_g_Handlers_append_in_registerAuthRoutes", "direct access to gin's .Handlers", func(d string) {
			edit(t, d, "main_chpw.go", "g.POST(\"/login\", d.LoginRate, d.Login)", "g.Handlers = append(gin.HandlersChain{evil}, g.Handlers...)\n\tg.POST(\"/login\", d.LoginRate, d.Login)")
		}},
		{"RV6_engine_Handlers_append_in_applyTrustedProxies", "direct access to gin's .Handlers", func(d string) {
			edit(t, d, "trusted_proxies.go", setTP, "r.Handlers = append(r.Handlers, evil)\n\t"+setTP)
		}},
		{"RV6b_engine_Handlers_write_in_registerWebSocketRoute", "direct access to gin's .Handlers", func(d string) {
			edit(t, d, "main_wsroute.go", "router.GET(\"/ws\"", "router.(*gin.Engine).Handlers = nil\n\trouter.GET(\"/ws\"")
		}},
		{"RV7_unfoldable_PATCH_on_api", "cannot be constant-folded", func(d string) {
			edit(t, d, "main.go", recDefn, "api.PATCH(os.Getenv(\"P\"), evil)\n\t"+recDefn)
		}},
		{"RV8_method_value_Use_in_ws_helper", "method value", func(d string) {
			edit(t, d, "main_wsroute.go", "router.GET(\"/ws\", mw.RequireAuth(), h.HandleConnection)", "u := router.Use\n\tu(evil)\n\trouter.GET(\"/ws\", mw.RequireAuth(), h.HandleConnection)")
		}},
		{"RV9_RouterGroup_field_Use_in_other_file_value", "direct access to gin's .RouterGroup", func(d string) {
			edit(t, d, "trusted_proxies.go", setTP, "rg := &r.RouterGroup\n\tf := rg.Use\n\tf(evil)\n\t"+setTP)
		}},
		// Own mutants, one per class in the structural model.
		{"N1_unfoldable_OPTIONS", "cannot be constant-folded", func(d string) {
			edit(t, d, "main_router.go", defn, defn+"\n\trouter.OPTIONS(os.Getenv(\"P\"), evil)")
		}},
		{"N2_unfoldable_HEAD", "cannot be constant-folded", func(d string) {
			edit(t, d, "main.go", recDefn, "api.HEAD(os.Getenv(\"P\"), evil)\n\t"+recDefn)
		}},
		{"N3_Handle_unclean_auth_path", "registers under the auth surface", func(d string) {
			edit(t, d, "main_router.go", defn, defn+"\n\trouter.Handle(\"POST\", \"/api//v1//auth/./z\", evil)")
		}},
		{"N4_Match_auth_path", "registers under the auth surface", func(d string) {
			edit(t, d, "main_router.go", defn, defn+"\n\trouter.Match([]string{\"GET\"}, \"/api/v1/auth/m\", evil)")
		}},
		{"N5_StaticFile_auth_path", "registers under the auth surface", func(d string) {
			edit(t, d, "main_router.go", defn, defn+"\n\trouter.StaticFile(\"/api/v1/auth/f\", \"x\")")
		}},
		{"N6_other_file_func_taking_engine", "in the signature of tamper", func(d string) {
			addFile(t, d, "zz_n6.go", "func tamper(e *gin.Engine) {}\n")
		}},
		{"N7_deref_copy_of_engine", "engine/group value \"router\" used outside", func(d string) {
			edit(t, d, "main_router.go", defn, "cp := *router\n\t_ = cp\n\t"+defn)
		}},
		{"N8_RouterGroup_embedded_Use_call", "direct access to gin's .RouterGroup", func(d string) {
			edit(t, d, "main_router.go", defn, "router.RouterGroup.Use(evil)\n\t"+defn)
		}},
		{"N9_closure_wraps_router_Use", "not on the recorded pre-group allow-list", func(d string) {
			edit(t, d, "main_router.go", defn, "useIt := func(h gin.HandlerFunc) { router.Use(h) }\n\tuseIt(evil)\n\t"+defn)
		}},
		{"N10_router_alias", "engine/group value \"router\" used outside", func(d string) {
			edit(t, d, "main_router.go", defn, "r2 := router\n\tr2.Use(evil)\n\t"+defn)
		}},
		{"N11_second_group_same_prefix", "registers under the auth surface", func(d string) {
			edit(t, d, "main_router.go", defn, defn+"\n\tag2 := router.Group(\"/api/v1/auth\")\n\t_ = ag2")
		}},
		{"N12_relative_auth_group_under_api", "registers under the auth surface", func(d string) {
			edit(t, d, "main.go", recDefn, "authB := api.Group(\"/auth\")\n\t_ = authB\n\t"+recDefn)
		}},
		{"N13_prefix_split_across_nested_groups", "segment prefix of /api/v1/auth", func(d string) {
			edit(t, d, "main_router.go", defn, defn+"\n\tpa := router.Group(\"/api\")\n\tag4 := pa.Group(\"/v1/auth\")\n\t_ = ag4")
		}},
		{"N14_Use_on_g_inside_registerAuthRoutes", "group prefix cannot be resolved", func(d string) {
			edit(t, d, "main_chpw.go", "g.POST(\"/login\", d.LoginRate, d.Login)", "g.Use(evil)\n\tg.POST(\"/login\", d.LoginRate, d.Login)")
		}},
		{"N15_Use_on_authGroup", "authGroup identifier count", func(d string) {
			edit(t, d, "main_router.go", defn, defn+"\n\tauthGroup.Use(evil)")
		}},
		{"N16_authGroup_var_form", "authGroup must be", func(d string) {
			edit(t, d, "main_router.go", defn, "var authGroup = router.Group(\"/api/v1/auth\")")
		}},
		{"N17_package_level_authGroup", "package-level declaration of authGroup", func(d string) {
			addFile(t, d, "zz_n17.go", "var authGroup gin.IRoutes\n")
		}},
		{"N18_unallowlisted_engine_Use", "not on the recorded pre-group allow-list", func(d string) {
			edit(t, d, "main_router.go", "router.Use(root_middleware.RequestID())", "router.Use(authBypass())\n\trouter.Use(root_middleware.RequestID())")
		}},
		{"N19_hook_slice_spread_into_engine_Use", "not on the recorded pre-group allow-list", func(d string) {
			edit(t, d, "main_router.go", "router.Use(root_middleware.RequestID())", "router.Use(hooks...)\n\trouter.Use(root_middleware.RequestID())")
		}},
		{"N20_auth_route_in_other_file_via_const", "registers under the auth surface", func(d string) {
			addFile(t, d, "zz_n20.go", "const p = \"/api/v1/auth/\"\nfunc extra() { registerWebSocketRoute(nil, nil, nil); var r gin.IRoutes; _ = r }\n")
			edit(t, d, "main_wsroute.go", "router.GET(\"/ws\"", "router.POST(\"/api/v1/auth/x\", nil)\n\trouter.GET(\"/ws\"")
		}},
		{"N21_unfoldable_engine_route_in_helper", "cannot be constant-folded", func(d string) {
			edit(t, d, "main_wsroute.go", "router.GET(\"/ws\"", "router.POST(os.Getenv(\"X\"), nil)\n\trouter.GET(\"/ws\"")
		}},
	}
	// Control: the unmutated tree is clean (so every FAIL below is caused by its mutation).
	fset, files := nonTestFiles(t, ".")
	require.Empty(t, scanAuthSurface(fset, files), "baseline must be clean")
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := mutate(t, tc.fn)
			require.NotEmpty(t, got, "mutant SURVIVED: guard did not fail")
			require.Contains(t, strings.Join(got, "\n"), tc.want, "mutant killed, but not by the expected rule")
		})
	}
}

func TestAuthSurfaceGuard_GoldenFalse(t *testing.T) {
	const defn = "authGroup := router.Group(\"/api/v1/auth\")"
	const recDefn = "recGroup := api.Group(\"/recommendations\")"
	for name, fn := range map[string]func(dir string){
		"harmless_concat_not_a_route": func(d string) {
			edit(t, d, "main_router.go", defn, defn+"\n\tharmless := \"/api/v1\" + \"/auth\"\n\t_ = harmless")
		},
		"sibling_prefix_authors_is_not_auth": func(d string) {
			edit(t, d, "main.go", recDefn, "api.GET(\"/authors\", evil)\n\t"+recDefn)
		},
		"router_Use_after_group_in_main_does_not_reach_it": func(d string) {
			edit(t, d, "main.go", "// Add Alt-Svc header to advertise HTTP/3 support", "router.Use(evilLater())\n\t// Add Alt-Svc header to advertise HTTP/3 support")
		},
		"direct_call_on_non_auth_api_group": func(d string) {
			edit(t, d, "main.go", recDefn, "api.PATCH(\"/golden-false\", evil)\n\t"+recDefn)
		},
	} {
		fn := fn
		t.Run(name, func(t *testing.T) {
			require.Empty(t, mutate(t, fn))
		})
	}
}
