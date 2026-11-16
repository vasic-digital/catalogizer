package main

// ADMD1F1-S1-G1 (§11.4.276 class-exhaustive, replaces the earlier 3-file line scan): the code side of the admin route cross-check is
// derived from the Go AST of EVERY route-registering source file (discovered, not listed). It resolves group prefixes
// (`x := y.Group("/p")`), group-level gates (`x.Use(adminGate)` / `x.Use(mw.RequireAdmin())`), per-route gates (an `adminGate` value,
// a `RequireAdmin()` call, a gate wrapper such as AliasHandler.syncHandlers once the caller installed WithSyncGate(adminGate)),
// multi-line registrations, and group/gate parameters bound at call sites (RegisterSubtitleRoutes(api, ..., adminGate)). Routes are
// compared with the table by FULL path. A registration whose full path cannot be resolved is reported, never skipped.

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type scanEnv struct {
	prefix  string
	gated   bool
	unknown bool
}

type scanSeed struct {
	groups    map[string]scanEnv
	gates     map[string]bool
	wrapGated bool
}

func (s scanSeed) key() string {
	var ks []string
	for k, v := range s.groups {
		ks = append(ks, k+"="+v.prefix+"|"+strconv.FormatBool(v.gated))
	}
	for k, v := range s.gates {
		ks = append(ks, "g:"+k+strconv.FormatBool(v))
	}
	sort.Strings(ks)
	return strings.Join(ks, ",") + strconv.FormatBool(s.wrapGated)
}

type scanRoute struct {
	method, path, file string
	gated              bool
}

type routeScan struct {
	routes     []scanRoute
	unresolved []string
	files      map[string]bool // files in which at least one route registration was seen
}

var routeMethods = map[string]bool{"GET": true, "POST": true, "PUT": true, "DELETE": true, "PATCH": true}

func exprStr(e ast.Expr) string {
	var b bytes.Buffer
	_ = printer.Fprint(&b, token.NewFileSet(), e)
	return b.String()
}

func isGroupType(e ast.Expr) bool {
	s := exprStr(e)
	return strings.Contains(s, "RouterGroup") || strings.Contains(s, "IRoutes") || strings.Contains(s, "IRouter") || strings.HasSuffix(s, "gin.Engine")
}

func isGateType(e ast.Expr) bool { return exprStr(e) == "gin.HandlerFunc" }

type fnInfo struct {
	decl *ast.FuncDecl
	file string
}

func discoverGoSources(root string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			n := d.Name()
			if p != root && (n == "vendor" || n == "testdata" || n == "node_modules" || strings.HasPrefix(n, ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(p, ".go") && !strings.HasSuffix(p, "_test.go") {
			out = append(out, p)
		}
		return nil
	})
	sort.Strings(out)
	return out, err
}

func joinRoute(prefix, lit string) string {
	return prefix + lit
}

// scanRouteSources parses the given files and returns every route registration with its resolved full path and gate state.
func scanRouteSources(t *testing.T, files []string) *routeScan {
	t.Helper()
	fset := token.NewFileSet()
	funcs := map[string][]fnInfo{}
	for _, f := range files {
		af, err := parser.ParseFile(fset, f, nil, 0)
		require.NoError(t, err, f)
		for _, d := range af.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok && fd.Body != nil {
				funcs[fd.Name.Name] = append(funcs[fd.Name.Name], fnInfo{fd, filepath.ToSlash(f)})
			}
		}
	}
	// gate wrappers: methods that return a handler chain built from a *Gate field (AliasHandler.syncHandlers)
	wrappers := map[string]bool{}
	for name, infos := range funcs {
		for _, fi := range infos {
			if fi.decl.Recv == nil || fi.decl.Type.Results == nil || !strings.Contains(exprStr(fi.decl.Type.Results.List[0].Type), "HandlerFunc") {
				continue
			}
			ast.Inspect(fi.decl.Body, func(n ast.Node) bool {
				if s, ok := n.(*ast.SelectorExpr); ok && strings.HasSuffix(s.Sel.Name, "Gate") {
					wrappers[name] = true
				}
				return true
			})
		}
	}
	// functions that build the root engine (their result is a root group with an empty prefix)
	engineFuncs := map[string]bool{}
	for name, infos := range funcs {
		for _, fi := range infos {
			if fi.decl.Type.Results != nil {
				for _, r := range fi.decl.Type.Results.List {
					if strings.HasSuffix(exprStr(r.Type), "gin.Engine") {
						engineFuncs[name] = true
					}
				}
			}
		}
	}
	res := &routeScan{files: map[string]bool{}}
	seeds := map[string]map[string]scanSeed{} // callee -> key -> seed
	type job struct {
		fi   fnInfo
		seed *scanSeed
	}
	done := map[string]bool{}
	var queue []job
	// every function is walked unseeded first (functions without group params are the roots)
	var names []string
	for n := range funcs {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		for _, fi := range funcs[n] {
			queue = append(queue, job{fi, nil})
		}
	}
	walk := func(j job) {
		fd := j.fi.decl
		id := j.fi.file + ":" + fd.Name.Name + ":" + strconv.Itoa(int(fd.Pos()))
		if j.seed != nil {
			id += "#" + j.seed.key()
		}
		if done[id] {
			return
		}
		done[id] = true
		groups := map[string]scanEnv{}
		gates := map[string]bool{}
		hasGroupParam := false
		for _, fl := range fd.Type.Params.List {
			for _, nm := range fl.Names {
				switch {
				case isGroupType(fl.Type):
					hasGroupParam = true
					if j.seed != nil {
						if e, ok := j.seed.groups[nm.Name]; ok {
							groups[nm.Name] = e
							continue
						}
					}
					groups[nm.Name] = scanEnv{unknown: true}
				case isGateType(fl.Type):
					gates[nm.Name] = j.seed != nil && j.seed.gates[nm.Name]
				}
			}
		}
		if hasGroupParam && j.seed == nil {
			// a function with group parameters is analysed only through its call-site seeds; the unseeded walk is still run so a
			// registration that no caller binds is reported (fail closed) instead of silently skipped
		}
		wrapGated := j.seed != nil && j.seed.wrapGated
		var isGate func(e ast.Expr) bool
		isGate = func(e ast.Expr) bool {
			switch x := e.(type) {
			case *ast.Ident:
				return gates[x.Name]
			case *ast.ParenExpr:
				return isGate(x.X)
			case *ast.CallExpr:
				if s, ok := x.Fun.(*ast.SelectorExpr); ok {
					if s.Sel.Name == "RequireAdmin" {
						return true
					}
					if wrappers[s.Sel.Name] && wrapGated {
						return true
					}
				}
			}
			return false
		}
		var recv func(e ast.Expr) (scanEnv, bool)
		recv = func(e ast.Expr) (scanEnv, bool) {
			switch x := e.(type) {
			case *ast.Ident:
				v, ok := groups[x.Name]
				return v, ok
			case *ast.ParenExpr:
				return recv(x.X)
			case *ast.CallExpr:
				if id, isID := x.Fun.(*ast.Ident); isID && engineFuncs[id.Name] {
					return scanEnv{}, true
				}
				s, ok := x.Fun.(*ast.SelectorExpr)
				if !ok {
					return scanEnv{}, false
				}
				if s.Sel.Name == "Group" && len(x.Args) >= 1 {
					b, ok := recv(s.X)
					if !ok {
						return scanEnv{}, false
					}
					n := scanEnv{prefix: b.prefix, gated: b.gated, unknown: b.unknown}
					if bl, ok := x.Args[0].(*ast.BasicLit); ok && bl.Kind == token.STRING {
						lit, _ := strconv.Unquote(bl.Value)
						n.prefix = joinRoute(b.prefix, lit)
					} else {
						n.unknown = true
					}
					for _, a := range x.Args[1:] {
						if isGate(a) {
							n.gated = true
						}
					}
					return n, true
				}
				if s.Sel.Name == "Use" {
					return recv(s.X)
				}
				if engineFuncs[s.Sel.Name] {
					return scanEnv{}, true
				}
				if (s.Sel.Name == "New" || s.Sel.Name == "Default") && exprStr(s.X) == "gin" {
					return scanEnv{}, true
				}
			}
			return scanEnv{}, false
		}
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.AssignStmt:
				if len(x.Rhs) == 1 && len(x.Lhs) > 1 {
					if id, ok := x.Lhs[0].(*ast.Ident); ok {
						if e, ok := recv(x.Rhs[0]); ok {
							groups[id.Name] = e
						}
					}
				}
				if len(x.Lhs) == len(x.Rhs) {
					for i := range x.Lhs {
						id, ok := x.Lhs[i].(*ast.Ident)
						if !ok {
							continue
						}
						if e, ok := recv(x.Rhs[i]); ok {
							groups[id.Name] = e
						} else if isGate(x.Rhs[i]) {
							gates[id.Name] = true
						}
					}
				}
			case *ast.CallExpr:
				s, ok := x.Fun.(*ast.SelectorExpr)
				if ok && s.Sel.Name == "Use" {
					if id, isID := s.X.(*ast.Ident); isID {
						if e, has := groups[id.Name]; has {
							for _, a := range x.Args {
								if isGate(a) {
									e.gated = true
								}
							}
							groups[id.Name] = e
						}
					}
				}
				if ok && routeMethods[s.Sel.Name] && len(x.Args) >= 1 {
					if base, has := recv(s.X); has {
						gated := base.gated
						for _, a := range x.Args[1:] {
							if isGate(a) {
								gated = true
							}
						}
						res.files[j.fi.file] = true
						if bl, isLit := x.Args[0].(*ast.BasicLit); isLit && bl.Kind == token.STRING && !base.unknown {
							lit, _ := strconv.Unquote(bl.Value)
							res.routes = append(res.routes, scanRoute{s.Sel.Name, joinRoute(base.prefix, lit), j.fi.file, gated})
						} else if j.seed == nil || !hasGroupParam || base.unknown {
							if base.unknown && hasGroupParam && j.seed != nil {
								// seeded walk but this receiver is still unbound
							}
							res.unresolved = append(res.unresolved, j.fi.file+": "+fd.Name.Name+": "+exprStr(x.Fun)+"("+exprStr(x.Args[0])+") full path unresolved")
						}
					}
				}
				// call-site binding of group / gate arguments to the callee's parameters
				var cname string
				switch f := x.Fun.(type) {
				case *ast.Ident:
					cname = f.Name
				case *ast.SelectorExpr:
					cname = f.Sel.Name
				}
				for _, ci := range funcs[cname] {
					var pnames []string
					var ptypes []ast.Expr
					for _, fl := range ci.decl.Type.Params.List {
						for _, nm := range fl.Names {
							pnames = append(pnames, nm.Name)
							ptypes = append(ptypes, fl.Type)
						}
					}
					if len(pnames) != len(x.Args) {
						continue
					}
					sd := scanSeed{groups: map[string]scanEnv{}, gates: map[string]bool{}}
					bound := false
					for i, a := range x.Args {
						switch {
						case isGroupType(ptypes[i]):
							if e, ok := recv(a); ok {
								sd.groups[pnames[i]] = e
								bound = true
							}
						case isGateType(ptypes[i]):
							if isGate(a) {
								sd.gates[pnames[i]] = true
							}
						}
					}
					// receiver chain WithXxxGate(<gate>) turns the callee's gate wrappers on
					for r := x.Fun; ; {
						se, ok := r.(*ast.SelectorExpr)
						if !ok {
							break
						}
						ce, ok := se.X.(*ast.CallExpr)
						if !ok {
							break
						}
						if cs, ok := ce.Fun.(*ast.SelectorExpr); ok && strings.HasPrefix(cs.Sel.Name, "With") && strings.HasSuffix(cs.Sel.Name, "Gate") {
							for _, a := range ce.Args {
								if isGate(a) {
									sd.wrapGated = true
								}
							}
						}
						r = ce.Fun
					}
					if bound {
						if seeds[cname] == nil {
							seeds[cname] = map[string]scanSeed{}
						}
						if _, dup := seeds[cname][sd.key()]; !dup {
							seeds[cname][sd.key()] = sd
							cp := sd
							queue = append(queue, job{ci, &cp})
						}
					}
				}
			}
			return true
		})
	}
	for i := 0; i < len(queue); i++ {
		j := queue[i]
		if j.seed == nil {
			// functions with group params are walked only when seeded (unseeded ones are checked below)
			hasGP := false
			for _, fl := range j.fi.decl.Type.Params.List {
				if isGroupType(fl.Type) {
					hasGP = true
				}
			}
			if hasGP {
				continue
			}
		}
		walk(j)
	}
	// group-param functions that register routes but were never seeded: fail closed
	for _, n := range names {
		for _, fi := range funcs[n] {
			hasGP := false
			for _, fl := range fi.decl.Type.Params.List {
				if isGroupType(fl.Type) {
					hasGP = true
				}
			}
			if !hasGP || len(seeds[n]) > 0 {
				continue
			}
			ast.Inspect(fi.decl.Body, func(nd ast.Node) bool {
				if c, ok := nd.(*ast.CallExpr); ok {
					if s, ok := c.Fun.(*ast.SelectorExpr); ok && routeMethods[s.Sel.Name] && len(c.Args) >= 1 {
						res.unresolved = append(res.unresolved, fi.file+": "+n+": registers routes but no call site binds its group parameter")
					}
				}
				return true
			})
		}
	}
	sort.Strings(res.unresolved)
	var uniq []string
	for i, u := range res.unresolved {
		if i == 0 || u != res.unresolved[i-1] {
			uniq = append(uniq, u)
		}
	}
	res.unresolved = uniq
	return res
}

// codeAdminRoutes projects a scan to the admin-gated routes (/metrics and /debug/ are excluded exactly as the table side does).
func codeAdminRoutes(s *routeScan) [][2]string {
	seen := map[string]bool{}
	var out [][2]string
	for _, r := range s.routes {
		if !r.gated || r.path == "/metrics" || strings.HasPrefix(r.path, "/debug/") {
			continue
		}
		k := strings.ToLower(r.method) + " " + r.path
		if !seen[k] {
			seen[k] = true
			out = append(out, [2]string{strings.ToLower(r.method), r.path})
		}
	}
	return out
}

func normFull(method, path string) string {
	k := normRoute(method, path)
	return strings.TrimRight(k, "/")
}

// tableMissingFromCode: table RequireAdmin rows that no admin-gated code registration explains (the converse direction, so that a
// file dropped from the scan or a gate the scan stopped seeing is detected).
func tableMissingFromCode(code [][2]string, rows []tableRow) []string {
	have := map[string]bool{}
	for _, c := range code {
		have[normFull(c[0], c[1])] = true
	}
	var out []string
	for k := range tableAdminRoutes(rows) {
		if !have[strings.TrimRight(k, "/")] {
			out = append(out, "table RequireAdmin row with no admin-gated code registration: "+k)
		}
	}
	sort.Strings(out)
	return out
}

func realRouteFiles(t *testing.T) []string {
	t.Helper()
	files, err := discoverGoSources(".")
	require.NoError(t, err)
	return files
}

func TestADMD1F1S1G1_CodeScanEqualsTableBothDirections(t *testing.T) {
	rows := readAuthzTable(t)
	s := scanRouteSources(t, realRouteFiles(t))
	require.Empty(t, s.unresolved, "every route registration must resolve to a full path")
	code := codeAdminRoutes(s)
	require.Empty(t, codeVsTableViolations(code, rows))
	require.Empty(t, tableMissingFromCode(code, rows))
}

// control needles (§11.4.273): positive and negative, on the real sources.
func TestADMD1F1S1G1_CodeScanControlNeedles(t *testing.T) {
	s := scanRouteSources(t, realRouteFiles(t))
	require.Greater(t, len(s.routes), 150, "instrument must see the real route set")
	code := codeAdminRoutes(s)
	has := func(m, p string) bool {
		for _, c := range code {
			if normFull(c[0], c[1]) == normFull(m, p) {
				return true
			}
		}
		return false
	}
	require.True(t, has("get", "/api/v1/sync/status"), "known gated route registered in alias_routes.go (via syncHandlers + WithSyncGate)")
	require.True(t, has("get", "/api/v1/sync/conflicts"), "alias_routes.go")
	require.True(t, has("post", "/api/v1/smb/probe"), "group-level smbGroup.Use(adminGate)")
	require.True(t, has("post", "/api/v1/challenges/run"), "group-level challengeGroup.Use(adminGate)")
	require.True(t, has("get", "/api/v1/subtitles/search"), "RegisterSubtitleRoutes: group + gate bound at the call site")
	require.True(t, has("post", "/api/v1/scans"), "RegisterScanRoutes with an empty path on the /scans group")
	require.True(t, has("put", "/api/v1/sync/endpoints/:id"), "multi-segment group + param spelling")
	require.False(t, has("get", "/api/v1/sync/devices/zz-fabricated"), "fabricated route")
	require.False(t, has("get", "/api/v1/browse"), "known ungated route")
	require.False(t, has("put", "/:id"), "a bare generic suffix is never a full path")
	for _, f := range []string{"main.go", "handlers/alias_routes.go", "handlers/subtitle_routes.go", "handlers/storage_routes.go", "main_chpw.go"} {
		require.True(t, s.files[f], "route-registering file discovered: "+f)
	}
}

// F1 RED (generic-suffix hole): the old suffix match accepted ("put", "/:id"); comparison is now by FULL path.
func TestADMD1F1S1G1_GenericSuffixIsAViolation(t *testing.T) {
	rows := readAuthzTable(t)
	for _, c := range [][2]string{{"put", "/:id"}, {"post", "/run"}, {"get", "/status"}, {"post", "/api/v1/zz-no-row"}} {
		require.NotEmpty(t, codeVsTableViolations([][2]string{c}, rows), "%v", c)
	}
	require.Empty(t, codeVsTableViolations([][2]string{{"put", "/api/v1/sync/endpoints/:id"}}, rows), "a full path that has a table row passes")
}

func filesWithout(files []string, base string) []string {
	var out []string
	for _, f := range files {
		if filepath.Base(f) != base {
			out = append(out, f)
		}
	}
	return out
}

func TestADMD1F1S1G1_CodeScanMutantsAreKilled(t *testing.T) {
	rows := readAuthzTable(t)
	files := realRouteFiles(t)
	check := func(fs []string) (fwd, conv []string, unres []string) {
		s := scanRouteSources(t, fs)
		c := codeAdminRoutes(s)
		return codeVsTableViolations(c, rows), tableMissingFromCode(c, rows), s.unresolved
	}
	t.Run("Q8 subtitle_routes.go dropped from the scanned set", func(t *testing.T) {
		_, conv, _ := check(filesWithout(files, "subtitle_routes.go"))
		require.NotEmpty(t, conv)
	})
	t.Run("Q8b alias_routes.go dropped (alias syncGate gate invisible)", func(t *testing.T) {
		_, conv, _ := check(filesWithout(files, "alias_routes.go"))
		require.NotEmpty(t, conv)
	})
	t.Run("Q8c storage_routes.go dropped", func(t *testing.T) {
		_, conv, _ := check(filesWithout(files, "storage_routes.go"))
		require.NotEmpty(t, conv)
	})
	t.Run("Q9 a route in an unlisted file whose group param no caller binds is reported, not skipped", func(t *testing.T) {
		dir := t.TempDir()
		src := "package x\nimport \"github.com/gin-gonic/gin\"\nfunc R(r *gin.Engine, adminGate gin.HandlerFunc) {\n\tz := r.Group(\"/zz\")\n\tz.Use(adminGate)\n\tz.GET(\n\t\t\"/new-op\",\n\t\tfunc(c *gin.Context) {})\n}\n"
		f := filepath.Join(dir, "zz_routes.go")
		require.NoError(t, os.WriteFile(f, []byte(src), 0o600))
		require.NotEmpty(t, scanRouteSources(t, []string{f}).unresolved)
	})
	t.Run("Q10 gate passed as a gin.HandlerFunc param at a call site gates a multi-line registration", func(t *testing.T) {
		dir := t.TempDir()
		src := "package x\nimport \"github.com/gin-gonic/gin\"\nfunc Mount(g *gin.RouterGroup, gate gin.HandlerFunc) {\n\tg.GET(\n\t\t\"/new-op\",\n\t\tgate,\n\t\tfunc(c *gin.Context) {})\n}\nfunc Run(adminGate gin.HandlerFunc) {\n\tr := gin.New()\n\tMount(r.Group(\"/api/v1/zz\"), adminGate)\n}\n"
		f := filepath.Join(dir, "zz_routes.go")
		require.NoError(t, os.WriteFile(f, []byte(src), 0o600))
		s := scanRouteSources(t, []string{f})
		require.Empty(t, s.unresolved)
		// Run's adminGate is a gate-typed param WITHOUT a seed (unseeded root), so it is not a known gate: the route is seen but ungated.
		require.Empty(t, codeAdminRoutes(s))
		require.Len(t, s.routes, 1)
		require.Equal(t, "/api/v1/zz/new-op", s.routes[0].path)
	})
	t.Run("Q11 group-level Use + multi-line + RequireAdmin call", func(t *testing.T) {
		dir := t.TempDir()
		src := "package x\nimport \"github.com/gin-gonic/gin\"\ntype mw struct{}\nfunc (mw) RequireAdmin() gin.HandlerFunc { return nil }\nfunc Run(m mw) {\n\tr := gin.New()\n\tg := r.Group(\"/api/v1\")\n\tz := g.Group(\"/zz\")\n\tz.Use(m.RequireAdmin())\n\tz.GET(\n\t\t\"/new-op\",\n\t\tfunc(c *gin.Context) {})\n\tg.POST(\"/zz2\",\n\t\tm.RequireAdmin(),\n\t\tfunc(c *gin.Context) {})\n}\n"
		f := filepath.Join(dir, "zz_routes.go")
		require.NoError(t, os.WriteFile(f, []byte(src), 0o600))
		c := codeAdminRoutes(scanRouteSources(t, []string{f}))
		require.ElementsMatch(t, [][2]string{{"get", "/api/v1/zz/new-op"}, {"post", "/api/v1/zz2"}}, c)
		require.NotEmpty(t, codeVsTableViolations(c, rows), "both have no table row")
	})
	t.Run("Q12 route registered before the group Use is not gated", func(t *testing.T) {
		dir := t.TempDir()
		src := "package x\nimport \"github.com/gin-gonic/gin\"\ntype mw struct{}\nfunc (mw) RequireAdmin() gin.HandlerFunc { return nil }\nfunc Run(m mw) {\n\tr := gin.New()\n\tz := r.Group(\"/zz\")\n\tz.GET(\"/early\", func(c *gin.Context) {})\n\tz.Use(m.RequireAdmin())\n\tz.GET(\"/late\", func(c *gin.Context) {})\n}\n"
		f := filepath.Join(dir, "zz_routes.go")
		require.NoError(t, os.WriteFile(f, []byte(src), 0o600))
		c := codeAdminRoutes(scanRouteSources(t, []string{f}))
		require.Equal(t, [][2]string{{"get", "/zz/late"}}, c)
	})
	t.Run("Q13 an unbound group parameter that registers routes is reported", func(t *testing.T) {
		dir := t.TempDir()
		src := "package x\nimport \"github.com/gin-gonic/gin\"\nfunc Orphan(g *gin.RouterGroup) {\n\tg.GET(\"/x\", func(c *gin.Context) {})\n}\n"
		f := filepath.Join(dir, "zz_routes.go")
		require.NoError(t, os.WriteFile(f, []byte(src), 0o600))
		require.NotEmpty(t, scanRouteSources(t, []string{f}).unresolved)
	})
}

// ADMD1F1-S1-G2: the live loader's marker check is pinned by its AST shape, not by string presence. The loader must contain
// `if v := adminGateLoaderViolations(p, item); len(v) > 0 { t.Fatalf(...) }`: the exact call, the comparison `len(v) > 0` and a
// fatal in the body (mutants: `> 99`, `>= 99`, `< 0`, Fatalf replaced by Logf, call removed).
func loaderViolationsFatal(t *testing.T, src string) bool {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), "api_contract_live_test.go", src, 0)
	require.NoError(t, err)
	found := false
	ast.Inspect(f, func(n ast.Node) bool {
		is, ok := n.(*ast.IfStmt)
		if !ok {
			return true
		}
		as, ok := is.Init.(*ast.AssignStmt)
		if !ok || len(as.Lhs) != 1 || len(as.Rhs) != 1 {
			return true
		}
		call, ok := as.Rhs[0].(*ast.CallExpr)
		if !ok || exprStr(call.Fun) != "adminGateLoaderViolations" || len(call.Args) != 2 || exprStr(call.Args[0]) != "p" || exprStr(call.Args[1]) != "item" {
			return true
		}
		v, ok := as.Lhs[0].(*ast.Ident)
		if !ok {
			return true
		}
		cmp, ok := is.Cond.(*ast.BinaryExpr)
		if !ok || cmp.Op != token.GTR || exprStr(cmp.X) != "len("+v.Name+")" || exprStr(cmp.Y) != "0" {
			return true
		}
		fatal := false
		ast.Inspect(is.Body, func(b ast.Node) bool {
			if c, ok := b.(*ast.CallExpr); ok {
				if e := exprStr(c.Fun); e == "t.Fatalf" || e == "t.Fatal" {
					fatal = true
				}
			}
			return true
		})
		if fatal {
			found = true
		}
		return true
	})
	return found
}

func TestADMD1F1S1G2_LoaderConditionPinnedByAST(t *testing.T) {
	raw, err := os.ReadFile("api_contract_live_test.go")
	require.NoError(t, err)
	src := string(raw)
	require.True(t, loaderViolationsFatal(t, src), "the loader must fail fatally when adminGateLoaderViolations reports anything")
	for name, mut := range map[string][2]string{
		"Q7 len(v) > 99":     {"len(v) > 0", "len(v) > 99"},
		"Q7b len(v) >= 1":    {"len(v) > 0", "len(v) >= 1"},
		"Q7c len(v) < 0":     {"len(v) > 0", "len(v) < 0"},
		"Q7d Fatalf -> Logf": {"t.Fatalf(\"admin-gate marker violations", "t.Logf(\"admin-gate marker violations"},
		"Q7e wrong args":     {"adminGateLoaderViolations(p, item)", "adminGateLoaderViolations(p, nil)"},
	} {
		m := strings.Replace(src, mut[0], mut[1], 1)
		require.NotEqual(t, src, m, name+": the mutation must apply")
		require.False(t, loaderViolationsFatal(t, m), name+" must be detected")
	}
	// the function the loader calls really reports a malformed or misplaced marker (behaviour, not text)
	require.NotEmpty(t, adminGateLoaderViolations("/x", map[string]interface{}{adminGateMarker: true}))
	require.NotEmpty(t, adminGateLoaderViolations("/x", map[string]interface{}{"get": map[string]interface{}{adminGateMarker: "yes"}}))
	require.Empty(t, adminGateLoaderViolations("/x", map[string]interface{}{"get": map[string]interface{}{adminGateMarker: true}}))
}

// ADMD1F1-S1-G5: the drift exclusion hides only the real RequireSelfOrAdmin wording, not any 403 that merely mentions "self".
func TestADMD1F1S1G5_DriftExclusionDoesNotHideAdminOnly403(t *testing.T) {
	mk := func(desc string) map[string]interface{} {
		return map[string]interface{}{"paths": map[string]interface{}{"/x": map[string]interface{}{"get": map[string]interface{}{
			"responses": map[string]interface{}{"403": map[string]interface{}{"description": desc}}}}}}
	}
	// a genuine admin-only 403 that mentions "self" and has no marker is drift
	require.NotEmpty(t, adminGateSpecViolations(mk("Administrator role required; a self-registered user is refused")))
	require.NotEmpty(t, adminGateSpecViolations(mk("Admin role required (not for self service)")))
	// the real self-or-admin wording stays excluded
	require.Empty(t, adminGateSpecViolations(mk("Forbidden; the target user is not the caller (and the caller is not an administrator)")))
}
