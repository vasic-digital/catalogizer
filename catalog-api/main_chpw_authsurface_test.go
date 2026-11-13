package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// CAT-CHPW-RL-1-FU structural round: ONE enumerator over every non-test file of
// package main that closes the class "something can still alter the
// /api/v1/auth route group or add /api/v1/auth routes without going through
// registerAuthRoutes with the rate limiter".
//
// Re-derived model (every way middleware or routes can reach the surface):
//  1. the group value itself: package-level var, var-form definition, `=`
//     re-assignment (also from another file), alias/field/slice/closure
//     storage, `.Use` / `.Group` on it, a hook slice spread into `.Use`;
//  2. the parent: global `router.Use` before the group is created (gin copies
//     the engine's handlers into a group at creation), the router handed to
//     anything else, a router alias, `router.Handlers` / `router.RouterGroup`
//     style field access;
//  3. a second path to the same prefix: another `.Group` whose folded path is
//     the prefix, a prefix split across nested groups ("/api" then "/v1/auth"),
//     a relative "/auth" group or route under the "/api/v1" group;
//  4. a direct route: GET/POST/.../Any/Handle/Match/Static* whose constant-folded
//     path is under /api/v1/auth, whatever the method.
// A path argument that cannot be folded fails closed, as does an empty match
// set. Unit-class source scan; it never runs main().

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

// Recorded allow-lists (justified; anything else is a finding).
var (
	// Global middleware main() legitimately installs on the engine BEFORE the
	// auth group exists (so it applies to /api/v1/auth too). Exactly the
	// callees present in main.go today, verified by reading it; adding another
	// requires a deliberate edit here.
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
	// `.Use` receivers that are provably NOT the auth group or its parent
	// engine (non-auth groups created in main.go). Any other receiver, in any
	// file, is a finding.
	allowedUseReceivers = map[string]bool{"pprofGroup": true, "api": true, "statsGroup": true, "smbGroup": true, "adminGroup": true, "entityGroup": true}
	// Members main.go may call or read on the engine.
	allowedRouterMembers = map[string]bool{"Use": true, "GET": true, "POST": true, "PUT": true, "DELETE": true, "Group": true}
	// Groups whose folded path is a proper segment prefix of /api/v1/auth.
	allowedPrefixGroups = map[string]int{"api/v1": 1}
	// authGroup identifier count per file: definition + registerAuthRoutes arg
	// in main.go; parameter + RegisterChangePassword arg in main_chpw.go.
	wantAuthGroupIdents = map[string]int{"main.go": 2, "main_chpw.go": 2}
)

const authPrefix = "api/v1/auth"

func segHasPrefix(q, p string) bool { return q == p || strings.HasPrefix(q, p+"/") }
func trimSlashes(s string) string   { return strings.Trim(s, "/") }

func exprText(fset *token.FileSet, e ast.Expr) string {
	var sb strings.Builder
	_ = printer.Fprint(&sb, fset, e)
	return sb.String()
}

// calleeDesc names what a router.Use argument is: the called function for a
// call, "<funclit>" for a function literal, otherwise the expression text.
func calleeDesc(fset *token.FileSet, e ast.Expr) string {
	switch v := e.(type) {
	case *ast.CallExpr:
		return exprText(fset, v.Fun)
	case *ast.FuncLit:
		return "<funclit>"
	}
	return exprText(fset, e)
}

// scanAuthSurface returns the list of violations over the parsed files.
func scanAuthSurface(fset *token.FileSet, files map[string]*ast.File) []string {
	var out []string
	add := func(format string, a ...any) { out = append(out, fmt.Sprintf(format, a...)) }
	binds := buildBinds(files)

	names := make([]string, 0, len(files))
	for n := range files {
		names = append(names, n)
	}
	sort.Strings(names)
	// Key every file by basename so a copied tree scans identically.
	byBase := map[string]*ast.File{}
	for n, f := range files {
		byBase[filepath.Base(n)] = f
	}
	files = byBase
	names = names[:0]
	for n := range files {
		names = append(names, n)
	}
	sort.Strings(names)

	identCount := map[string]int{}
	defines := 0
	prefixGroups := map[string]int{}
	var authDefPos token.Pos
	sanctioned := map[*ast.CallExpr]bool{}

	// Pass 1: the authGroup definition (V1/V7/V8 and aliases).
	for _, name := range names {
		f := files[name]
		for _, d := range f.Decls {
			if gd, ok := d.(*ast.GenDecl); ok {
				for _, sp := range gd.Specs {
					if vs, ok := sp.(*ast.ValueSpec); ok {
						for _, n := range vs.Names {
							if n.Name == "authGroup" {
								add("%s: package-level declaration of authGroup is forbidden (V1)", name)
							}
						}
					}
				}
			}
		}
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				switch v := n.(type) {
				case *ast.Ident:
					if v.Name == "authGroup" {
						identCount[name]++
					}
				case *ast.DeclStmt:
					if gd, ok := v.Decl.(*ast.GenDecl); ok {
						for _, sp := range gd.Specs {
							if vs, ok := sp.(*ast.ValueSpec); ok {
								for _, nm := range vs.Names {
									if nm.Name == "authGroup" {
										add("%s: authGroup must be defined with `:=`, the `var` form is rejected (V7)", name)
									}
								}
							}
						}
					}
				case *ast.AssignStmt:
					for _, l := range v.Lhs {
						id, ok := l.(*ast.Ident)
						if !ok || id.Name != "authGroup" {
							continue
						}
						if v.Tok != token.DEFINE {
							add("%s: authGroup assigned with `%s`; only `:=` is allowed (V1/V8)", name, v.Tok)
							continue
						}
						defines++
						okShape := name == "main.go" && fd.Name.Name == "main" && len(v.Lhs) == 1 && len(v.Rhs) == 1
						if okShape {
							c, isCall := v.Rhs[0].(*ast.CallExpr)
							if isCall {
								if _, isG := isCallSel(c, "router", "Group"); isG && len(c.Args) == 1 {
									if p, ok := foldString(c.Args[0], binds); ok && trimSlashes(p) == authPrefix {
										sanctioned[c] = true
										authDefPos = v.Pos()
										continue
									}
								}
							}
						}
						add("%s: authGroup must be `authGroup := router.Group(\"/api/v1/auth\")` inside main() of main.go", name)
					}
				}
				return true
			})
		}
	}
	// Idents outside function bodies (package-level, params, struct fields).
	for _, name := range names {
		ast.Inspect(files[name], func(n ast.Node) bool {
			if fd, ok := n.(*ast.FuncDecl); ok {
				// parameters / receivers are outside Body: count them here
				for _, fl := range []*ast.FieldList{fd.Recv, fd.Type.Params, fd.Type.Results} {
					if fl == nil {
						continue
					}
					for _, fld := range fl.List {
						for _, nm := range fld.Names {
							if nm.Name == "authGroup" {
								identCount[name]++
							}
						}
					}
				}
				return true
			}
			return true
		})
		for _, d := range files[name].Decls {
			if gd, ok := d.(*ast.GenDecl); ok {
				ast.Inspect(gd, func(n ast.Node) bool {
					if id, ok := n.(*ast.Ident); ok && id.Name == "authGroup" {
						identCount[name]++
					}
					return true
				})
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

	// Pass 2: registrations, Use, router handling.
	for _, name := range names {
		f := files[name]
		okRouter := map[*ast.Ident]bool{}
		ast.Inspect(f, func(n ast.Node) bool {
			switch v := n.(type) {
			case *ast.SelectorExpr:
				if id, ok := v.X.(*ast.Ident); ok && id.Name == "router" && name == "main.go" {
					okRouter[id] = true
					if !allowedRouterMembers[v.Sel.Name] {
						add("%s: router.%s is not an allowed engine member (V2 class)", name, v.Sel.Name)
					}
				}
			case *ast.KeyValueExpr:
				if k, ok := v.Key.(*ast.Ident); ok && k.Name == "Handler" {
					if id, ok := v.Value.(*ast.Ident); ok && id.Name == "router" {
						okRouter[id] = true
					}
				}
			case *ast.AssignStmt:
				if v.Tok == token.DEFINE && len(v.Lhs) == 1 && len(v.Rhs) == 1 {
					if id, ok := v.Lhs[0].(*ast.Ident); ok && id.Name == "router" {
						if c, ok := v.Rhs[0].(*ast.CallExpr); ok && exprText(fset, c.Fun) == "gin.New" {
							okRouter[id] = true
						}
					}
				}
			case *ast.CallExpr:
				if fn, ok := v.Fun.(*ast.Ident); ok && (fn.Name == "applyTrustedProxies" || fn.Name == "registerWebSocketRoute") && len(v.Args) > 0 {
					if id, ok := v.Args[0].(*ast.Ident); ok && id.Name == "router" {
						okRouter[id] = true
					}
				}
			}
			return true
		})
		if name == "main.go" {
			ast.Inspect(f, func(n ast.Node) bool {
				if id, ok := n.(*ast.Ident); ok && id.Name == "router" && !okRouter[id] {
					add("%s:%d: router used outside a method call, gin.New() definition, Handler: field or the two sanctioned helper calls (alias/handoff, V2 class)", name, fset.Position(id.Pos()).Line)
				}
				return true
			})
		}

		ast.Inspect(f, func(n ast.Node) bool {
			c, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := c.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			method := sel.Sel.Name
			line := fset.Position(c.Pos()).Line
			if method == "Use" {
				recv := exprText(fset, sel.X)
				id, isIdent := sel.X.(*ast.Ident)
				switch {
				case isIdent && id.Name == "router" && name == "main.go":
					if authDefPos != token.NoPos && c.Pos() < authDefPos {
						var ds []string
						for _, a := range c.Args {
							ds = append(ds, calleeDesc(fset, a))
						}
						d := strings.Join(ds, ",")
						if c.Ellipsis.IsValid() {
							d += "..."
						}
						ok := false
						for _, a := range allowedRouterUseBeforeAuth {
							if a == d {
								ok = true
							}
						}
						if !ok {
							add("%s:%d: router.Use(%s) before the auth group exists applies to /api/v1/auth and is not on the recorded allow-list (V2)", name, line, d)
						}
					} else if authDefPos == token.NoPos {
						add("%s:%d: router.Use present but the auth group definition was not found; cannot order it", name, line)
					}
				case isIdent && allowedUseReceivers[id.Name] && name == "main.go":
				default:
					add("%s:%d: %s.Use is not on a recorded non-auth receiver (V1/V8: could be the auth group or its parent)", name, line, recv)
				}
			}
			idx := -1
			switch method {
			case "GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS", "Any", "Group", "Static", "StaticFS", "StaticFile", "StaticFileFS":
				idx = 0
			case "Handle", "Match":
				idx = 1
			}
			if idx < 0 || len(c.Args) <= idx || sanctioned[c] {
				return true
			}
			// Only calls that look like gin registrations: skip http.Handle-style
			// receivers by requiring a path-like first path argument below.
			p, folded := foldString(c.Args[idx], binds)
			if !folded {
				if method == "GET" || method == "POST" || method == "PUT" || method == "DELETE" || method == "Group" || method == "Any" {
					add("%s:%d: %s path argument cannot be constant-folded; fail closed (V3/V9)", name, line, method)
				}
				return true
			}
			q := trimSlashes(p)
			if segHasPrefix(q, "auth") || segHasPrefix(q, authPrefix) {
				add("%s:%d: %s(%q) registers under the auth surface outside registerAuthRoutes (V3/V9)", name, line, method, p)
			}
			if method == "Group" {
				if segHasPrefix(authPrefix, q) && q != authPrefix {
					prefixGroups[q]++
				}
			}
			return true
		})
	}
	for q, n := range prefixGroups {
		if allowedPrefixGroups[q] != n {
			add("Group(%q) is a segment prefix of /api/v1/auth, %d time(s), allowed %d (nested/folded prefix, V3)", q, n, allowedPrefixGroups[q])
		}
	}
	if prefixGroups["api/v1"] == 0 {
		add("expected the /api/v1 group not found: enumerator matched nothing (fail closed)")
	}
	return out
}

func TestMainPackageAuthSurfaceClosed(t *testing.T) {
	fset, files := nonTestFiles(t, ".")
	require.Contains(t, files, "main.go")
	require.Contains(t, files, "main_chpw.go")
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
	cases := []struct {
		name string
		want string // substring that must appear in a finding
		fn   func(dir string)
	}{
		{"V1_package_level_var_and_assign", "package-level declaration of authGroup", func(d string) {
			addFile(t, d, "zz_v1.go", "var authGroup gin.IRoutes\nfunc init() { authGroup = nil }\n")
		}},
		{"V1_assign_from_other_file_only", "authGroup assigned with `=`", func(d string) {
			edit(t, d, "main.go", defn, defn+"\n\tauthGroup = router.Group(\"/x\")")
		}},
		{"V8_hook_slice_spread_into_Use", "authGroup identifier count", func(d string) {
			addFile(t, d, "zz_v8.go", "var authHooks []gin.HandlerFunc\nfunc wire() { authGroup.Use(authHooks...) }\n")
		}},
		{"V8_Use_on_authGroup_in_main", "authGroup.Use", func(d string) {
			edit(t, d, "main.go", defn, defn+"\n\tauthGroup.Use(evil)")
		}},
		{"alias_ag_equals_authGroup", "authGroup identifier count", func(d string) {
			edit(t, d, "main.go", defn, defn+"\n\tag := authGroup\n\tag.Use(evil)")
		}},
		{"V2_global_router_Use_before_group", "router.Use(authBypass) before", func(d string) {
			edit(t, d, "main.go", "router.Use(root_middleware.RequestID())", "router.Use(authBypass())\n\trouter.Use(root_middleware.RequestID())")
		}},
		{"V2_router_Use_with_spread_slice", "router.Use(hooks...) before", func(d string) {
			edit(t, d, "main.go", "router.Use(root_middleware.RequestID())", "router.Use(hooks...)\n\trouter.Use(root_middleware.RequestID())")
		}},
		{"V2_router_alias", "router used outside", func(d string) {
			edit(t, d, "main.go", defn, "r2 := router\n\tr2.Use(evil)\n\t"+defn)
		}},
		{"V2_router_Handlers_field", "not an allowed engine member", func(d string) {
			edit(t, d, "main.go", defn, "router.Handlers = nil\n\t"+defn)
		}},
		{"V3a_second_group_same_prefix", "registers under the auth surface", func(d string) {
			edit(t, d, "main.go", defn, defn+"\n\tag2 := router.Group(\"/api/v1/auth\")\n\t_ = ag2")
		}},
		{"V3b_relative_auth_group_under_api", "registers under the auth surface", func(d string) {
			edit(t, d, "main.go", "recGroup := api.Group(\"/recommendations\")", "authB := api.Group(\"/auth\")\n\t_ = authB\n\trecGroup := api.Group(\"/recommendations\")")
		}},
		{"V3c_folded_prefix_concat", "registers under the auth surface", func(d string) {
			edit(t, d, "main.go", defn, defn+"\n\tag3 := router.Group(\"/api/v1\" + \"/au\" + \"th/\")\n\t_ = ag3")
		}},
		{"V3d_prefix_split_across_nested_groups", "segment prefix of /api/v1/auth", func(d string) {
			edit(t, d, "main.go", defn, defn+"\n\tpa := router.Group(\"/api\")\n\tag4 := pa.Group(\"/v1/auth\")\n\t_ = ag4")
		}},
		{"V3e_unfoldable_group_path", "cannot be constant-folded", func(d string) {
			edit(t, d, "main.go", defn, defn+"\n\tag5 := router.Group(os.Getenv(\"P\"))\n\t_ = ag5")
		}},
		{"V7_var_form_definition", "`var` form is rejected", func(d string) {
			edit(t, d, "main.go", defn, "var authGroup = router.Group(\"/api/v1/auth\")")
		}},
		{"V9_router_Any_auth_path", "registers under the auth surface", func(d string) {
			edit(t, d, "main.go", defn, defn+"\n\trouter.Any(\"/api/v1/auth/login\", evil)")
		}},
		{"V9_router_Handle_auth_path", "registers under the auth surface", func(d string) {
			edit(t, d, "main.go", "router.GET(\"/discovery\"", "router.Handle(\"POST\", \"/api/v1/auth/x\", evil)\n\trouter.GET(\"/discovery\"")
		}},
		{"V9_api_GET_relative_auth", "registers under the auth surface", func(d string) {
			edit(t, d, "main.go", "recGroup := api.Group(\"/recommendations\")", "api.GET(\"/auth/login\", evil)\n\trecGroup := api.Group(\"/recommendations\")")
		}},
		{"V9_route_in_other_file", "registers under the auth surface", func(d string) {
			addFile(t, d, "zz_v9.go", "const p = \"/api/v1/auth/\"\nfunc extra(r gin.IRoutes) { r.POST(p+\"login\", nil) }\n")
		}},
		{"Use_on_g_inside_registerAuthRoutes", "g.Use is not on a recorded non-auth receiver", func(d string) {
			edit(t, d, "main_chpw.go", "g.POST(\"/login\", d.LoginRate, d.Login)", "g.Use(evil)\n\tg.POST(\"/login\", d.LoginRate, d.Login)")
		}},
		{"other_file_Use_on_engine_param", "is not on a recorded non-auth receiver", func(d string) {
			addFile(t, d, "zz_use.go", "func tamper(e *gin.Engine) { e.Use(evil) }\n")
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
	for name, fn := range map[string]func(dir string){
		"harmless_concat_not_a_route": func(d string) {
			edit(t, d, "main.go", defn, defn+"\n\tharmless := \"/api/v1\" + \"/auth\"\n\t_ = harmless")
		},
		"sibling_prefix_authors_is_not_auth": func(d string) {
			edit(t, d, "main.go", "recGroup := api.Group(\"/recommendations\")", "api.GET(\"/authors\", evil)\n\trecGroup := api.Group(\"/recommendations\")")
		},
		"router_Use_after_group_does_not_reach_it": func(d string) {
			edit(t, d, "main.go", "// Add Alt-Svc header to advertise HTTP/3 support", "router.Use(evilLater())\n\t// Add Alt-Svc header to advertise HTTP/3 support")
		},
	} {
		fn := fn
		t.Run(name, func(t *testing.T) {
			require.Empty(t, mutate(t, fn))
		})
	}
}
