package main

// ADMD1F1-S1-G1 (§11.4.276 class-exhaustive, replaces the earlier 3-file line scan): the code side of the admin route cross-check is
// derived from the Go AST of EVERY route-registering source file (discovered, not listed). It resolves group prefixes
// (`x := y.Group("/p")`), group-level gates (`x.Use(adminGate)` / `x.Use(mw.RequireAdmin())`), per-route gates (an `adminGate` value,
// a `RequireAdmin()` call, a gate wrapper such as AliasHandler.syncHandlers once the caller installed WithSyncGate(adminGate)),
// multi-line registrations, and group/gate parameters bound at call sites (RegisterSubtitleRoutes(api, ..., adminGate)). Routes are
// compared with the table by FULL path. A registration whose full path cannot be resolved is reported, never skipped.

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io/fs"
	"os"
	"path"
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
	strs      map[string]string
	wrapGated bool
}

func (s scanSeed) key() string {
	var ks []string
	for k, v := range s.groups {
		ks = append(ks, k+"="+v.prefix+"|"+strconv.FormatBool(v.gated)+strconv.FormatBool(v.unknown))
	}
	for k, v := range s.gates {
		ks = append(ks, "g:"+k+strconv.FormatBool(v))
	}
	for k, v := range s.strs {
		ks = append(ks, "s:"+k+"="+v)
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

// every gin route-registering method; Handle and Match carry the path as the SECOND argument.
var routeMethods = map[string]bool{"GET": true, "POST": true, "PUT": true, "DELETE": true, "PATCH": true, "HEAD": true, "OPTIONS": true, "Any": true, "Handle": true, "Match": true}

func routePathIdx(name string) int {
	if name == "Handle" || name == "Match" {
		return 1
	}
	return 0
}

func exprStr(e ast.Expr) string {
	var b bytes.Buffer
	_ = printer.Fprint(&b, token.NewFileSet(), e)
	return b.String()
}

const ginImportPath = "github.com/gin-gonic/gin"

// Round 4 (I2/m3): parameter/result types are classified from the REAL type expression, never by substring. A type is a group type when
// it resolves (through type aliases and named types declared in the scanned set, parens, pointers and embedded interface elements) to
// gin.RouterGroup / IRoutes / IRouter / Engine imported from github.com/gin-gonic/gin; a gate type resolves to gin.HandlerFunc.
// Containers (slice, map, chan, func types) are NOT group types: a function whose only "group" is inside a container is an ordinary
// function and is walked like one (its element receivers do not resolve, so every registration through them is reported).
var (
	groupTypeNames  = map[string]bool{"RouterGroup": true, "IRoutes": true, "IRouter": true, "Engine": true}
	engineTypeNames = map[string]bool{"Engine": true}
	gateTypeNames   = map[string]bool{"HandlerFunc": true}
)

// staticRoutes (round 4, I1): gin.RouterGroup methods that register GET+HEAD routes without being named GET/HEAD. `wild` = the
// registered pattern is relativePath + "/*filepath" (Static, StaticFS); the File variants register relativePath itself.
var staticRoutes = map[string]struct {
	nargs int
	wild  bool
}{"Static": {2, true}, "StaticFS": {2, true}, "StaticFile": {2, false}, "StaticFileFS": {3, false}}

// inertGroupMethods: methods of gin.Engine / RouterGroup that are known NOT to register a route (pinned by
// TestADMD1F1S1R4_InertMethodListPinned). NoRoute and NoMethod install the 404/405 handler chains: they are not routes of the table
// and are deliberately not reported (documented non-route, round 4 nit). ANY other method called on a resolved group/engine receiver
// that is neither modelled (routeMethods, Use, Group, staticRoutes) nor listed here is reported (class-wide rule).
var inertGroupMethods = map[string]bool{
	"NoRoute": true, "NoMethod": true, "Run": true, "RunTLS": true, "RunUnix": true, "RunFd": true, "RunListener": true,
	"Routes": true, "BasePath": true, "SetTrustedProxies": true, "SetHTMLTemplate": true,
	"LoadHTMLGlob": true, "LoadHTMLFiles": true, "LoadHTMLFS": true, "Delims": true, "SetFuncMap": true,
}

// mountMethods (round 5, A20/A21): methods that turn a group/engine into an http.Handler or re-dispatch a request through it. A
// sub-engine mounted under a wildcard route of the main engine (gin.WrapH(sub.Handler()), gin.WrapF(sub.ServeHTTP), sub.HandleContext(c))
// registers its routes in ITS OWN table, which the main engine's route table does not show; every use is reported, call or value.
// (The finished engine served through http.Server{Handler: router} is a composite-literal element, exempt by import path below.)
// engineDataFields: bool / config fields of gin.Engine read (not called) on a resolved engine; none registers or gates a route.
// RouterGroup (embedded group) and Handlers (the middleware slice, assignable) are deliberately NOT here.
var engineDataFields = map[string]bool{"RedirectTrailingSlash": true, "RedirectFixedPath": true, "HandleMethodNotAllowed": true,
	"ForwardedByClientIP": true, "UseRawPath": true, "UnescapePathValues": true, "RemoveExtraSlash": true, "UseEscapedPath": true,
	"UseH2C": true, "ContextWithFallback": true, "MaxMultipartMemory": true, "TrustedPlatform": true, "RemoteIPHeaders": true}

func engineDataFieldNames() []string {
	var out []string
	for k := range engineDataFields {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

var mountMethods = map[string]bool{"Handler": true, "ServeHTTP": true, "HandleContext": true}

// nonGinNames (round 5, A22/A23): net/http names that register or serve routes outside the gin engine, so outside the route table.
// Every use through a package whose import path resolves to net/http is reported unless pinned in nonGinAllowed.
var nonGinNames = map[string]bool{"ServeMux": true, "NewServeMux": true, "DefaultServeMux": true, "HandleFunc": true, "Handle": true,
	"ListenAndServe": true, "ListenAndServeTLS": true, "Serve": true, "ServeTLS": true}

// nonGinAllowed pins, by "<file suffix>:<function>:<net/http name>", each non-gin router use with its reason (TestADMD1F1S1R5_NonGinAllowListPinned).
var nonGinAllowed = map[string]string{
	"internal/monitoring/monitoring.go:StartPrometheusServer:NewServeMux": "separate Prometheus listener bound to 127.0.0.1 only, serves /metrics and nothing else (documented in its doc comment); the authenticated /metrics of the main engine is the route-table row",
	"tests/mocks/webdav_mock_server.go:Start:NewServeMux":                 "test double server under tests/mocks, not part of the shipped binary",
}

func nonGinAllowedKeys() []string {
	var out []string
	for k := range nonGinAllowed {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// gateWrapperFuncs: callees that wrap a gate and still apply it (metricsSkipIfScrape skips only on a valid scrape token, /metrics is
// outside the admin table by design). Any OTHER call that merely receives a gate as an argument is not a gate and is reported (round 5).
var gateWrapperFuncs = map[string]bool{"metricsSkipIfScrape": true}

func inertGroupMethodNames() []string {
	var out []string
	for k := range inertGroupMethods {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// serverImportPaths: composite literals of these packages' Server type serve a finished engine (Handler: <engine>) and register no
// route (m1: matched by the resolved import path, never by the spelling of the type).
var serverImportPaths = map[string]bool{"net/http": true, "github.com/quic-go/quic-go/http3": true}

// allowedGroupSinkNames: callees that legitimately receive an engine/group value without registering routes on it (round 3, I1: any
// OTHER callee that is handed a group value and is not a scanned function with a group parameter is reported unresolved).
//   - verifyAuthRouteTable(e *gin.Engine): reads the finished engine's route table at startup (main_router.go), registers nothing.
//   - engineChain(eng any, field string): reflectively reads a HandlersChain field of the finished engine for verifyAuthRouteTable.
//   - applyTrustedProxies / setTrustedProxies: configure X-Forwarded-For trust on the engine (trusted_proxies.go), register nothing.
//
// Pinned by TestADMD1F1S1R3_AllowListPinned.
//   - routeChainFuncNames(eng any): reads the finished engine's route trees reflectively (main_routechains.go), used by the routedump test build only.
var allowedGroupSink = map[string]bool{"verifyAuthRouteTable": true, "applyTrustedProxies": true, "setTrustedProxies": true, "engineChain": true, "routeChainFuncNames": true}

func allowedGroupSinkNames() []string {
	var out []string
	for k := range allowedGroupSink {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

type fnInfo struct {
	decl *ast.FuncDecl
	file string
	id   string
	gp   bool // has a group-typed parameter or receiver (resolved by typeIs)
}

func (f fnInfo) hasGroupParam() bool { return f.gp }

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

// joinRoute is gin's joinPaths(absolutePath, relativePath) (RouterGroup.calculateAbsolutePath); the root group's base is "/".
func joinRoute(abs, rel string) string {
	if rel == "" {
		return abs
	}
	final := path.Join(abs, rel)
	if rel[len(rel)-1] == '/' && final[len(final)-1] != '/' {
		return final + "/"
	}
	return final
}

// scanRouteSources parses the given files and returns every route registration with its resolved full path and gate state. Anything
// it cannot resolve (receiver, path, gate, redefined group name, group parameter no caller binds) is put in routeScan.unresolved.
func scanRouteSources(t *testing.T, files []string) *routeScan {
	t.Helper()
	fset := token.NewFileSet()
	var all []fnInfo
	byName := map[string][]fnInfo{}
	imports := map[string]map[string]string{} // file -> local package name -> import path: `zap.Any(...)` is a package function, never a gin route method
	type typeDecl struct {
		expr ast.Expr
		file string
	}
	typeDecls := map[string][]typeDecl{}
	type pkgDecl struct {
		file string
		d    *ast.GenDecl
	}
	var pkgDecls []pkgDecl
	globals := map[string]string{}
	badGlobal := map[string]bool{}
	for _, f := range files {
		af, err := parser.ParseFile(fset, f, nil, 0)
		require.NoError(t, err, f)
		imps := map[string]string{}
		for _, im := range af.Imports {
			ip, _ := strconv.Unquote(im.Path.Value)
			if im.Name != nil {
				imps[im.Name.Name] = ip
			} else {
				imps[path.Base(ip)] = ip
			}
		}
		imports[filepath.ToSlash(f)] = imps
		for _, d := range af.Decls {
			switch dd := d.(type) {
			case *ast.FuncDecl:
				if dd.Body != nil {
					fi := fnInfo{dd, filepath.ToSlash(f), filepath.ToSlash(f) + "#" + strconv.Itoa(int(dd.Pos())), false}
					all = append(all, fi)
					byName[dd.Name.Name] = append(byName[dd.Name.Name], fi)
				}
			case *ast.GenDecl:
				pkgDecls = append(pkgDecls, pkgDecl{filepath.ToSlash(f), dd})
				for _, sp := range dd.Specs {
					if ts, ok := sp.(*ast.TypeSpec); ok {
						typeDecls[ts.Name.Name] = append(typeDecls[ts.Name.Name], typeDecl{ts.Type, filepath.ToSlash(f)})
					}
					if vs, ok := sp.(*ast.ValueSpec); ok && len(vs.Names) == len(vs.Values) {
						for i, nm := range vs.Names {
							if bl, ok := vs.Values[i].(*ast.BasicLit); ok && bl.Kind == token.STRING {
								v, _ := strconv.Unquote(bl.Value)
								if old, dup := globals[nm.Name]; dup && old != v {
									badGlobal[nm.Name] = true
								}
								globals[nm.Name] = v
							}
						}
					}
				}
			}
		}
	}
	for n := range badGlobal {
		delete(globals, n)
	}
	var typeIs func(file string, e ast.Expr, names map[string]bool, depth int) bool
	typeIs = func(file string, e ast.Expr, names map[string]bool, depth int) bool {
		if depth > 8 {
			return false
		}
		switch x := e.(type) {
		case *ast.StarExpr:
			return typeIs(file, x.X, names, depth)
		case *ast.ParenExpr:
			return typeIs(file, x.X, names, depth)
		case *ast.SelectorExpr:
			id, ok := x.X.(*ast.Ident)
			return ok && imports[file][id.Name] == ginImportPath && names[x.Sel.Name]
		case *ast.Ident:
			for _, d := range typeDecls[x.Name] {
				if typeIs(d.file, d.expr, names, depth+1) {
					return true
				}
			}
		case *ast.InterfaceType:
			for _, m := range x.Methods.List {
				if len(m.Names) == 0 && typeIs(file, m.Type, names, depth+1) {
					return true
				}
			}
		}
		return false
	}
	isGroupType := func(file string, e ast.Expr) bool { return typeIs(file, e, groupTypeNames, 0) }
	isGateType := func(file string, e ast.Expr) bool { return typeIs(file, e, gateTypeNames, 0) }
	for i := range all {
		fi := &all[i]
		for _, fl := range fi.decl.Type.Params.List {
			if isGroupType(fi.file, fl.Type) {
				fi.gp = true
			}
		}
		if fi.decl.Recv != nil {
			for _, fl := range fi.decl.Recv.List {
				if isGroupType(fi.file, fl.Type) {
					fi.gp = true
				}
			}
		}
	}
	byName = map[string][]fnInfo{} // rebuilt now that fnInfo.gp is known
	for _, fi := range all {
		byName[fi.decl.Name.Name] = append(byName[fi.decl.Name.Name], fi)
	}
	// gate wrappers: methods that return a handler chain built from a *Gate field (AliasHandler.syncHandlers)
	wrappers := map[string]bool{}
	engineFuncs := map[string]bool{}
	for _, fi := range all {
		if fi.decl.Type.Results == nil {
			continue
		}
		for _, r := range fi.decl.Type.Results.List {
			if fi.decl.Recv == nil && typeIs(fi.file, r.Type, engineTypeNames, 0) { // round 5 (m1): a METHOD named like a factory is not the factory
				engineFuncs[fi.decl.Name.Name] = true
			}
		}
		if fi.decl.Recv != nil && strings.Contains(exprStr(fi.decl.Type.Results.List[0].Type), "HandlerFunc") {
			ast.Inspect(fi.decl.Body, func(n ast.Node) bool {
				if s, ok := n.(*ast.SelectorExpr); ok && strings.HasSuffix(s.Sel.Name, "Gate") {
					wrappers[fi.decl.Name.Name] = true
				}
				return true
			})
		}
	}
	res := &routeScan{files: map[string]bool{}}
	seeded := map[string]bool{} // fn id -> has at least one call-site seed
	queued := map[string]bool{} // fn id + seed key
	type job struct {
		fi   fnInfo
		seed *scanSeed
	}
	var queue []job
	for _, fi := range all {
		if !fi.hasGroupParam() {
			queue = append(queue, job{fi, nil})
		}
	}
	report := func(file, fn, msg string) { res.unresolved = append(res.unresolved, file+": "+fn+": "+msg) }
	// A22/A23 (round 5): a net/http router or server is outside the gin route table, so the table cannot show it. Every use of the
	// names in nonGinNames through a package that resolves to net/http is reported unless pinned in nonGinAllowed with its reason.
	checkNonGin := func(file, fn string, root ast.Node) {
		ast.Inspect(root, func(n ast.Node) bool {
			se, ok := n.(*ast.SelectorExpr)
			if !ok || !nonGinNames[se.Sel.Name] {
				return true
			}
			id, ok := se.X.(*ast.Ident)
			if !ok || imports[file][id.Name] != "net/http" {
				return true
			}
			for k := range nonGinAllowed {
				parts := strings.Split(k, ":")
				if len(parts) == 3 && strings.HasSuffix(file, parts[0]) && parts[1] == fn && parts[2] == se.Sel.Name {
					return true
				}
			}
			report(file, fn, "non-gin router or server use net/http."+se.Sel.Name+" (outside the gin route table; pin it in nonGinAllowed with a reason)")
			return true
		})
	}
	for _, fi := range all {
		checkNonGin(fi.file, fi.decl.Name.Name, fi.decl)
	}
	for _, pd := range pkgDecls {
		checkNonGin(pd.file, "<package level>", pd.d)
	}
	walk := func(j job) {
		fd := j.fi.decl
		fname := fd.Name.Name
		groups := map[string]scanEnv{}
		gates := map[string]bool{}
		strs := map[string]string{}
		for _, fl := range fd.Type.Params.List {
			for _, nm := range fl.Names {
				switch {
				case isGroupType(j.fi.file, fl.Type):
					if j.seed != nil {
						if e, ok := j.seed.groups[nm.Name]; ok {
							groups[nm.Name] = e
							continue
						}
					}
					groups[nm.Name] = scanEnv{unknown: true}
				case isGateType(j.fi.file, fl.Type):
					gates[nm.Name] = j.seed != nil && j.seed.gates[nm.Name]
				case exprStr(fl.Type) == "string":
					if j.seed != nil {
						if v, ok := j.seed.strs[nm.Name]; ok {
							strs[nm.Name] = v
						}
					}
				}
			}
		}
		if fd.Recv != nil { // a method whose receiver is itself a group/engine type: unknown unless a caller binds it (not modelled: unknown)
			for _, fl := range fd.Recv.List {
				if isGroupType(j.fi.file, fl.Type) {
					for _, nm := range fl.Names {
						groups[nm.Name] = scanEnv{unknown: true}
					}
				}
			}
		}
		wrapGated := j.seed != nil && j.seed.wrapGated
		// round 5: names declared in this function (parameters, results, :=, var/const, range, func literal parameters, type switch).
		// A name declared more than once that the walk tracks (group, gate or string) is reported: block scoping is not modelled (A28).
		// A locally declared name never falls back to a package-level constant of the same name (A31).
		declCount := map[string]int{}
		noteDecl := func(id *ast.Ident) {
			if id != nil && id.Name != "_" {
				declCount[id.Name]++
			}
		}
		noteFields := func(fl *ast.FieldList) {
			if fl != nil {
				for _, f := range fl.List {
					for _, nm := range f.Names {
						noteDecl(nm)
					}
				}
			}
		}
		noteFields(fd.Type.Params)
		noteFields(fd.Type.Results)
		noteFields(fd.Recv)
		parent := map[ast.Node]ast.Node{}
		var stack []ast.Node
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			if n == nil {
				stack = stack[:len(stack)-1]
				return true
			}
			if len(stack) > 0 {
				parent[n] = stack[len(stack)-1]
			}
			stack = append(stack, n)
			switch c := n.(type) {
			case *ast.AssignStmt:
				if c.Tok == token.DEFINE {
					for _, l := range c.Lhs {
						if id, ok := l.(*ast.Ident); ok {
							noteDecl(id)
						}
					}
				}
			case *ast.ValueSpec:
				for _, nm := range c.Names {
					noteDecl(nm)
				}
			case *ast.RangeStmt:
				if c.Tok == token.DEFINE {
					for _, e := range []ast.Expr{c.Key, c.Value} {
						if id, ok := e.(*ast.Ident); ok {
							noteDecl(id)
						}
					}
				}
			case *ast.FuncLit:
				noteFields(c.Type.Params)
				noteFields(c.Type.Results)
			case *ast.TypeSwitchStmt:
				if as, ok := c.Assign.(*ast.AssignStmt); ok && as.Tok == token.DEFINE {
					for _, l := range as.Lhs {
						if id, ok := l.(*ast.Ident); ok {
							noteDecl(id)
						}
					}
				}
			}
			return true
		})
		pathIdents := map[string]bool{} // identifiers used inside a route / Group path argument
		notePath := func(e ast.Expr) {
			ast.Inspect(e, func(n ast.Node) bool {
				if id, ok := n.(*ast.Ident); ok {
					pathIdents[id.Name] = true
				}
				return true
			})
		}
		tracked := map[string]bool{}
		paramGroups := map[string]bool{}
		for k := range groups {
			tracked[k] = true
			paramGroups[k] = true
		}
		for k := range gates {
			tracked[k] = true
		}
		for k := range strs {
			tracked[k] = true
		}
		var isGate func(e ast.Expr) bool
		isGate = func(e ast.Expr) bool {
			switch x := e.(type) {
			case *ast.Ident:
				return gates[x.Name]
			case *ast.ParenExpr:
				return isGate(x.X)
			case *ast.CallExpr:
				if s, ok := x.Fun.(*ast.SelectorExpr); ok {
					if s.Sel.Name == "RequireAdmin" || (wrappers[s.Sel.Name] && wrapGated) {
						return true
					}
				}
				for _, a := range x.Args { // a pinned wrapper around an admin gate (metricsSkipIfScrape(RequireAdmin())) still carries it
					if isGate(a) {
						name := ""
						switch f := x.Fun.(type) {
						case *ast.Ident:
							name = f.Name
						case *ast.SelectorExpr:
							name = f.Sel.Name
						}
						if gateWrapperFuncs[name] {
							return true
						}
						// round 5 (A29): a call that merely RECEIVES a gate does not apply it
						report(j.fi.file, fname, "call "+exprStr(x.Fun)+" receives a gate as an argument but is not a pinned gate wrapper: "+exprStr(x))
						return false
					}
				}
			}
			return false
		}
		var evalStr func(e ast.Expr) (string, bool)
		evalStr = func(e ast.Expr) (string, bool) {
			switch x := e.(type) {
			case *ast.BasicLit:
				if x.Kind == token.STRING {
					v, err := strconv.Unquote(x.Value)
					return v, err == nil
				}
			case *ast.Ident:
				if v, ok := strs[x.Name]; ok {
					return v, true
				}
				if declCount[x.Name] > 0 { // round 5 (A31): a local of the same name hides the package constant
					return "", false
				}
				v, ok := globals[x.Name]
				return v, ok
			case *ast.ParenExpr:
				return evalStr(x.X)
			case *ast.BinaryExpr:
				if x.Op == token.ADD {
					l, ok1 := evalStr(x.X)
					r, ok2 := evalStr(x.Y)
					return l + r, ok1 && ok2
				}
			}
			return "", false
		}
		var recv func(e ast.Expr) (scanEnv, bool)
		recv = func(e ast.Expr) (scanEnv, bool) {
			switch x := e.(type) {
			case *ast.Ident:
				v, ok := groups[x.Name]
				return v, ok
			case *ast.SelectorExpr:
				v, ok := groups[exprStr(x)]
				return v, ok
			case *ast.ParenExpr:
				return recv(x.X)
			case *ast.CallExpr:
				if id, isID := x.Fun.(*ast.Ident); isID && engineFuncs[id.Name] {
					return scanEnv{prefix: "/"}, true
				}
				s, ok := x.Fun.(*ast.SelectorExpr)
				if !ok {
					return scanEnv{}, false
				}
				switch {
				case s.Sel.Name == "Group" && len(x.Args) >= 1:
					b, ok := recv(s.X)
					if !ok {
						return scanEnv{}, false
					}
					n := scanEnv{prefix: b.prefix, gated: b.gated, unknown: b.unknown}
					notePath(x.Args[0])
					if lit, ok := evalStr(x.Args[0]); ok {
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
				case s.Sel.Name == "Use":
					b, ok := recv(s.X)
					if !ok {
						return scanEnv{}, false
					}
					for _, a := range x.Args { // r.Group("/api").Use(gate).GET(...): the gate rides the chain (round 3, I2)
						if isGate(a) {
							b.gated = true
						}
					}
					return b, true
				case (s.Sel.Name == "New" || s.Sel.Name == "Default") && imports[j.fi.file][exprStr(s.X)] == ginImportPath,
					engineFuncs[s.Sel.Name] && imports[j.fi.file][exprStr(s.X)] != "" && declCount[exprStr(s.X)] == 0: // m1: pkg.Factory(), never x.Factory()
					return scanEnv{prefix: "/"}, true
				}
			}
			return scanEnv{}, false
		}
		// fail-closed escape sinks (round 3): a group value that leaves the walk's model is reported, never dropped
		escape := func(e ast.Expr, why string) {
			if _, ok := recv(e); ok {
				report(j.fi.file, fname, "group value escapes the scanner ("+why+"): "+exprStr(e))
			}
		}
		// useCheck (round 5, structural): every use of a tracked group VARIABLE must be in a position the walk models (receiver of a
		// method, assignment/declaration, call argument, return/composite/send, ==/!= comparison). Address-of, dereference, type
		// assertion, indexing, slicing, range and every other position leave the model and are reported.
		useCheck := func(n ast.Expr) {
			if _, ok := groups[exprStr(n)]; !ok {
				return
			}
			var c ast.Node = n
			p := parent[n]
			for {
				pe, ok := p.(*ast.ParenExpr)
				if !ok {
					break
				}
				c, p = pe, parent[pe]
			}
			switch pp := p.(type) {
			case nil:
				return
			case *ast.SelectorExpr:
				return // receiver of a method (modelled elsewhere) or the field-name identifier itself
			case *ast.CallExpr, *ast.AssignStmt, *ast.ValueSpec, *ast.ReturnStmt, *ast.CompositeLit, *ast.SendStmt, *ast.ExprStmt, *ast.Field:
				return
			case *ast.KeyValueExpr:
				if pp.Value == c {
					return
				}
				return // a key identifier is not a use of the variable
			case *ast.BinaryExpr:
				if pp.Op == token.EQL || pp.Op == token.NEQ {
					return
				}
			}
			report(j.fi.file, fname, fmt.Sprintf("group value %s is used in a position the walk does not model (%T)", exprStr(n), p))
		}
		// ctxOf: innermost if/for/switch/select/closure/go/defer construct enclosing a call or declaration (nil = straight-line body).
		// A gate Use(...) is unconditional for a group only when it sits in the SAME context in which the group was declared.
		ctxOf := map[ast.Node]ast.Node{}
		declCtx := map[string]ast.Node{}
		calledSel := map[*ast.SelectorExpr]bool{}
		var mark func(n ast.Node, ctx ast.Node)
		mark = func(n ast.Node, ctx ast.Node) {
			ast.Inspect(n, func(c ast.Node) bool {
				if c == n {
					return true
				}
				switch cx := c.(type) {
				case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt, *ast.FuncLit, *ast.CaseClause, *ast.CommClause, *ast.GoStmt, *ast.DeferStmt:
					mark(c, c)
					return false
				case *ast.CallExpr:
					if sl, ok := cx.Fun.(*ast.SelectorExpr); ok {
						calledSel[sl] = true
					}
					ctxOf[cx] = ctx
				case *ast.AssignStmt, *ast.ValueSpec:
					ctxOf[c] = ctx
				}
				return true
			})
		}
		mark(fd.Body, nil)
		// execCtx: the nearest func literal, defer or go statement around a node (nil = the function body itself). Code in such a
		// context runs at a different time than its text position suggests (round 5, A32 / defer ordering).
		execCtx := func(n ast.Node) ast.Node {
			for p := parent[n]; p != nil; p = parent[p] {
				switch p.(type) {
				case *ast.FuncLit, *ast.DeferStmt, *ast.GoStmt:
					return p
				}
			}
			return nil
		}
		execDecl := map[string]ast.Node{}
		// spreadWrapper: `g.GET(path, h.syncHandlers(handler)...)`: the one spread form that is modelled, a call of a gate wrapper (a
		// method returning a handler chain built from a *Gate field) whose result is the whole handler list.
		spreadWrapper := func(c *ast.CallExpr) bool {
			if !c.Ellipsis.IsValid() || len(c.Args) == 0 {
				return false
			}
			ce, ok := c.Args[len(c.Args)-1].(*ast.CallExpr)
			if !ok {
				return false
			}
			sl, ok := ce.Fun.(*ast.SelectorExpr)
			return ok && wrappers[sl.Sel.Name]
		}
		rootKey := func(e ast.Expr) string { // the tracked variable at the root of a receiver chain (r.Group("/a").Use(g) -> r)
			for {
				if _, ok := groups[exprStr(e)]; ok {
					return exprStr(e)
				}
				switch c := e.(type) {
				case *ast.ParenExpr:
					e = c.X
				case *ast.CallExpr:
					if sl, ok := c.Fun.(*ast.SelectorExpr); ok {
						e = sl.X
					} else {
						return exprStr(e)
					}
				case *ast.SelectorExpr:
					e = c.X
				default:
					return exprStr(e)
				}
			}
		}
		bind := func(lhs, rhs ast.Expr, ctx ast.Node) {
			switch lhs.(type) {
			case *ast.Ident, *ast.SelectorExpr:
			default:
				escape(rhs, "assigned to a non-name target "+exprStr(lhs))
				return
			}
			k := exprStr(lhs)
			if k == "_" {
				return
			}
			if _, has := gates[k]; has { // round 5 (A27): a gate name bound twice is not tracked by value
				report(j.fi.file, fname, "gate variable "+k+" is reassigned (flow of the gate value is not modelled): "+k+" = "+exprStr(rhs))
				gates[k] = false
				return
			}
			if e, ok := recv(rhs); ok {
				tracked[k] = true
				switch u := ast.Unparen(rhs).(type) { // round 5 (A32c): a plain copy of a group shares the pointer; a later Use on either changes both
				case *ast.Ident, *ast.SelectorExpr:
					if exprStr(u) != k {
						report(j.fi.file, fname, "group value is aliased ("+k+" = "+exprStr(rhs)+"): a Use through one name gates the other, not modelled")
					}
				}
				if old, has := groups[k]; has && old != e {
					report(j.fi.file, fname, "group name "+k+" is redefined with a different value (block scoping is not modelled)")
				}
				groups[k] = e
				if _, has := declCtx[k]; !has {
					declCtx[k] = ctx
					execDecl[k] = execCtx(lhs)
				}
			} else if isGate(rhs) {
				gates[k] = true
				tracked[k] = true
			} else if v, ok := evalStr(rhs); ok {
				strs[k] = v
				tracked[k] = true
			}
		}
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.CompositeLit:
				isSrv := false // m1: <pkg>.Server where <pkg> resolves to net/http or quic-go http3: Handler: <engine> serves, registers nothing
				if st, ok := x.Type.(*ast.SelectorExpr); ok && st.Sel.Name == "Server" {
					if id, ok := st.X.(*ast.Ident); ok && serverImportPaths[imports[j.fi.file][id.Name]] {
						isSrv = true
					}
				}
				for _, el := range x.Elts {
					if kv, ok := el.(*ast.KeyValueExpr); ok {
						if k, isID := kv.Key.(*ast.Ident); isSrv && isID && k.Name == "Handler" {
							continue // serving the finished engine as an http.Handler registers no route
						}
						el = kv.Value
					}
					escape(el, "composite literal element")
				}
			case *ast.ReturnStmt:
				if !engineFuncs[fname] {
					for _, r := range x.Results {
						escape(r, "returned")
					}
				}
			case *ast.SendStmt:
				escape(x.Value, "channel send")
			case *ast.SelectorExpr:
				useCheck(x)
				if _, ok := recv(x.X); ok {
					switch {
					case mountMethods[x.Sel.Name]: // A20/A21: a sub-engine exposed as an http.Handler is mounted under a route of another engine
						report(j.fi.file, fname, "mount method "+x.Sel.Name+" on a resolved group/engine (a sub-engine's routes are not in the outer route table): "+exprStr(x))
					case x.Sel.Name == "RouterGroup":
						report(j.fi.file, fname, "embedded RouterGroup of an engine is exposed (its address can be taken and used as a group): "+exprStr(x))
					case !calledSel[x] && !engineDataFields[x.Sel.Name]:
						report(j.fi.file, fname, "method value or unmodelled field is not called directly (calls through it are not modelled): "+exprStr(x))
					}
				} else if _, isIdent := x.X.(*ast.Ident); !isIdent { // A10: method EXPRESSION (*gin.RouterGroup).GET / gin.Engine.Use
					if typeIs(j.fi.file, x.X, groupTypeNames, 0) {
						report(j.fi.file, fname, "method expression on a gin group/engine type (calls through it are not modelled): "+exprStr(x))
					}
				}
			case *ast.BranchStmt:
				if x.Tok == token.GOTO {
					report(j.fi.file, fname, "goto is not modelled (a Use or registration can be jumped over): goto "+x.Label.Name)
				}
			case *ast.Ident:
				useCheck(x)
			case *ast.FuncLit:
				for _, fl := range x.Type.Params.List {
					for _, nm := range fl.Names {
						if isGroupType(j.fi.file, fl.Type) {
							groups[nm.Name] = scanEnv{unknown: true}
						}
					}
				}
			case *ast.AssignStmt:
				if len(x.Lhs) == len(x.Rhs) {
					for i := range x.Lhs {
						bind(x.Lhs[i], x.Rhs[i], ctxOf[x])
					}
				} else if len(x.Rhs) == 1 && len(x.Lhs) > 1 {
					bind(x.Lhs[0], x.Rhs[0], ctxOf[x])
				}
			case *ast.ValueSpec:
				if len(x.Names) == len(x.Values) {
					for i := range x.Names {
						bind(x.Names[i], x.Values[i], ctxOf[x])
					}
				}
			case *ast.CallExpr:
				s, isSel := x.Fun.(*ast.SelectorExpr)
				if isSel {
					if _, has := recv(s.X); has && (routeMethods[s.Sel.Name] || s.Sel.Name == "Use" || s.Sel.Name == "Group" || func() bool { _, st := staticRoutes[s.Sel.Name]; return st }()) {
						if x.Ellipsis.IsValid() && !spreadWrapper(x) { // A32d: a spread handler list is not modelled
							report(j.fi.file, fname, "variadic handler list (...) is not modelled: "+exprStr(x))
						}
						if k := rootKey(s.X); execCtx(x) != execDecl[k] { // A32 / defer ordering: runs at another time than its text position
							report(j.fi.file, fname, "registration, Use or Group inside a closure, defer or go statement on a group declared outside it (execution order is not modelled): "+exprStr(x.Fun))
						}
					}
				}
				if isSel && s.Sel.Name == "Use" {
					if e, has := recv(s.X); has {
						for _, a := range x.Args {
							if isGate(a) {
								if paramGroups[exprStr(s.X)] { // A32b: Use on a group received as a parameter changes the CALLER's group
									report(j.fi.file, fname, "gate Use on a group received as a parameter (the caller's group changes; not modelled): "+exprStr(x))
								}
								e.gated = true
								if dc := declCtx[exprStr(s.X)]; ctxOf[x] != dc {
									report(j.fi.file, fname, "gate Use(...) in a conditional, loop, closure, go or defer context other than the group's declaration is not unconditional: "+exprStr(x))
								}
							}
						}
						k := exprStr(s.X)
						if _, direct := groups[k]; direct {
							groups[k] = e
						}
					}
				}
				if id, isID := x.Fun.(*ast.SelectorExpr); isID && imports[j.fi.file][exprStr(id.X)] != "" {
					if _, isGrp := recv(id.X); !isGrp { // a local group may shadow an import name (round 3, N11)
						isSel = false // package-qualified function (zap.Any), not a method on a router value
					}
				}
				if isSel {
					if _, modelled := staticRoutes[s.Sel.Name]; !modelled && !routeMethods[s.Sel.Name] && s.Sel.Name != "Use" && s.Sel.Name != "Group" && !inertGroupMethods[s.Sel.Name] {
						// I1 (round 4, class-wide): any method on a resolved group/engine that the scanner neither models nor pins as inert
						if _, ok := recv(s.X); ok {
							report(j.fi.file, fname, "unmodelled method "+s.Sel.Name+" called on a resolved group/engine receiver: "+exprStr(x.Fun))
						}
					}
				}
				var sr struct {
					nargs int
					wild  bool
				}
				isStatic := false
				if isSel {
					sr, isStatic = staticRoutes[s.Sel.Name]
				}
				if isStatic && len(x.Args) >= sr.nargs {
					call := exprStr(s.X) + "." + s.Sel.Name + "(" + exprStr(x.Args[0]) + ")"
					base, has := recv(s.X)
					notePath(x.Args[0])
					switch {
					case !has:
						report(j.fi.file, fname, call+" receiver does not resolve to a known group")
					case base.unknown:
						report(j.fi.file, fname, call+" full path is unresolved (group prefix unknown)")
					default:
						lit, okp := evalStr(x.Args[0])
						if !okp {
							report(j.fi.file, fname, call+" path is not a constant after call-site binding")
						} else {
							res.files[j.fi.file] = true
							if sr.wild {
								lit = path.Join(lit, "/*filepath")
							}
							for _, m := range []string{"GET", "HEAD"} {
								res.routes = append(res.routes, scanRoute{m, joinRoute(base.prefix, lit), j.fi.file, base.gated})
							}
						}
					}
				}
				if isSel && routeMethods[s.Sel.Name] {
					pi := routePathIdx(s.Sel.Name)
					if len(x.Args) >= pi+2 { // path plus at least one handler: a gin registration, not a lookalike
						call := exprStr(s.X) + "." + s.Sel.Name + "(" + exprStr(x.Args[pi]) + ")"
						base, has := recv(s.X)
						notePath(x.Args[pi])
						switch {
						case !has:
							report(j.fi.file, fname, call+" receiver does not resolve to a known group")
						case base.unknown:
							report(j.fi.file, fname, call+" full path is unresolved (group prefix unknown)")
						default:
							res.files[j.fi.file] = true
							lit, okp := evalStr(x.Args[pi])
							method := strings.ToUpper(s.Sel.Name)
							switch s.Sel.Name {
							case "Any":
								method = "ANY"
							case "Match":
								method = "MATCH"
							case "Handle":
								if m, ok := evalStr(x.Args[0]); ok {
									method = strings.ToUpper(m)
								} else {
									report(j.fi.file, fname, call+" HTTP method is not a constant")
								}
							}
							gated := base.gated
							handlers := x.Args[pi+1:]
							for i, a := range handlers {
								if isGate(a) {
									if i == len(handlers)-1 && spreadWrapper(x) { // the wrapper's chain is [gate, handlers...]: the gate precedes the terminal handler
										gated = true
									} else if i == len(handlers)-1 { // A26: the gate is the terminal handler, so it runs after the handler it should protect
										report(j.fi.file, fname, call+" has the gate as its LAST handler (it protects nothing): "+exprStr(a))
									} else {
										gated = true
									}
								} else if _, isLit := a.(*ast.FuncLit); !isLit && i < len(handlers)-1 && strings.Contains(strings.ToLower(exprStr(a)), "admin") && !strings.Contains(exprStr(a), "RequireSelfOrAdmin") {
									report(j.fi.file, fname, call+" has an admin-looking middleware the scanner cannot model: "+exprStr(a))
								}
							}
							methods := []string{method}
							switch s.Sel.Name {
							case "Any": // A38: gin registers nine routes
								methods = ginAnyMethods
							case "Match":
								methods = nil
								if cl, ok := ast.Unparen(x.Args[0]).(*ast.CompositeLit); ok {
									for _, el := range cl.Elts {
										if m, ok := evalStr(el); ok {
											methods = append(methods, strings.ToUpper(m))
										} else {
											methods = nil
											break
										}
									}
								}
								if len(methods) == 0 {
									report(j.fi.file, fname, call+" method list is not a constant string slice literal")
								}
							}
							if !okp {
								report(j.fi.file, fname, call+" path is not a constant after call-site binding")
							} else {
								for _, m := range methods {
									res.routes = append(res.routes, scanRoute{m, joinRoute(base.prefix, lit), j.fi.file, gated})
								}
							}
						}
					}
				}
				// call-site binding of group / gate / string arguments to every same-name, same-arity callee with group parameters
				var cname string
				switch f := x.Fun.(type) {
				case *ast.Ident:
					cname = f.Name
				case *ast.SelectorExpr:
					cname = f.Sel.Name
				}
				bound := make([]bool, len(x.Args)) // argument positions that a scanned callee receives in a group-typed parameter
				for _, ci := range byName[cname] {
					if !ci.hasGroupParam() {
						continue
					}
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
					sd := scanSeed{groups: map[string]scanEnv{}, gates: map[string]bool{}, strs: map[string]string{}}
					for i, a := range x.Args {
						switch {
						case isGroupType(ci.file, ptypes[i]):
							if e, ok := recv(a); ok {
								sd.groups[pnames[i]] = e
							}
						case isGateType(ci.file, ptypes[i]):
							if isGate(a) {
								sd.gates[pnames[i]] = true
							}
						case exprStr(ptypes[i]) == "string":
							if v, ok := evalStr(a); ok {
								sd.strs[pnames[i]] = v
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
					for i := range x.Args {
						if isGroupType(ci.file, ptypes[i]) {
							bound[i] = true
						}
					}
					seeded[ci.id] = true
					qk := ci.id + "#" + sd.key()
					if !queued[qk] {
						queued[qk] = true
						cp := sd
						queue = append(queue, job{ci, &cp})
					}
				}
				// I1 (round 3): a group value handed to any callee that is not a scanned function receiving it in a group parameter (other
				// package, interface method, func value, arity/type mismatch) leaves the scanner's model: reported unless allow-listed.
				ownMethod := false
				if isSel {
					if _, ok := recv(s.X); ok {
						_, st := staticRoutes[s.Sel.Name]
						ownMethod = routeMethods[s.Sel.Name] || st || inertGroupMethods[s.Sel.Name] || s.Sel.Name == "Use" || s.Sel.Name == "Group"
					}
				}
				// m2: the allow-list applies only to an UNQUALIFIED call of a function declared in the scanned set (ext.verifyAuthRouteTable(g) is a foreign function)
				allowedSink := false
				if id, isID := x.Fun.(*ast.Ident); isID && allowedGroupSink[id.Name] && len(byName[id.Name]) > 0 {
					allowedSink = true
				}
				if !ownMethod && !allowedSink {
					for i, a := range x.Args {
						if !bound[i] {
							escape(a, "passed to "+exprStr(x.Fun)+", which is not a scanned function with a group parameter at this position")
						}
					}
				}
			}
			return true
		})
		for k := range tracked { // A28: the same name declared twice in one function (shadowing across blocks) is not modelled
			_, isGrp := groups[k]
			_, isGt := gates[k]
			if declCount[k] > 1 && (isGrp || isGt || pathIdents[k]) { // a string name only matters when it feeds a route or group path
				report(j.fi.file, fname, "name "+k+" is declared "+strconv.Itoa(declCount[k])+" times in the function (block scoping is not modelled)")
			}
		}
	}
	// I2 (round 4): EVERY function is walked. A function with a group parameter that no call site binds is walked with all its group
	// parameters unknown, so each registration through them reports "full path is unresolved" and each escape reports.
	for done := 0; ; {
		for ; done < len(queue); done++ {
			walk(queue[done])
		}
		grew := false // queue drained: walk the never-seeded group-parameter functions (walking one can seed further callees)
		for _, fi := range all {
			if fi.hasGroupParam() && !seeded[fi.id] && !queued[fi.id+"#unseeded"] {
				queued[fi.id+"#unseeded"] = true
				queue = append(queue, job{fi, nil})
				grew = true
			}
		}
		if !grew {
			break
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
	return normRoute(method, path)
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
		if !have[k] {
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
