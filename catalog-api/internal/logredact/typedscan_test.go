package logredact

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// Type-aware leak scanner (PX-F17 structural round 3, replaces the line-pattern scanner of rounds 1-2).
//
// It type-checks every non-test package of the module (go/parser + go/types, dependencies from the compiler export data that
// `go list -export` produces) and runs a small intra-procedural taint analysis with inter-procedural return summaries:
//
//	SOURCES  expressions whose resolved TYPE/OBJECT is request-derived: http.Request.URL (and so every field / method of it:
//	         Path, RawPath, RawQuery, String(), EscapedPath(), Query() ...), http.Request.RequestURI, Request.Referer/FormValue/Cookie..,
//	         and key-addressed reads (gin Context.Param/Query/GetHeader, Header.Get, url.Values.Get, mux.Vars[...]) whose literal key
//	         is credential-named (logredact.SensitiveKey / secret path keys) or not a literal;
//	CLEANERS any call into package logredact (the only sanctioned redaction), and non-string basic types (len, status codes);
//	PROPAGATION assignments, :=, var, range, + concatenation, conversions, indexing, composite literals, calls of stdlib or
//	         third-party helpers with a tainted argument, and module functions whose return statements are inherently tainted;
//	SINKS    calls into fmt.Print*/Fprint*/Errorf, log, log/slog, errors.New, zap, internal/logging, logrus, prometheus label
//	         constructors, and Crashlytics ReportNonFatal - flagged when an argument is tainted OR is statically a whole
//	         request-like object (*http.Request, *url.URL, http.Header, *gin.Context);
//	BANNED   gin.Default/Recovery/Logger* constructors whose default writers dump the request (R2-1).
//
// A finding is reported with file + enclosing function; the reviewed non-leaking uses are an allow-list keyed by file+function
// (NOT by line text), every entry must still be hit (stale entries fail), and the scanner is validated by control needles:
// fixtures that MUST be reported and clean fixtures that MUST NOT.
//
// Honest limits (also in the final report): intra-procedural flow (no heap/aliasing, no flow through struct fields read in
// another function, no channels, closures share the enclosing function's variables), interface-dispatch sinks outside the sink
// package list are not seen, and a reviewed allow-list entry is a human claim, not a proof.

var (
	sinkPkgs = map[string]bool{
		"log": true, "log/slog": true, "go.uber.org/zap": true, "go.uber.org/zap/zapcore": true,
		"catalogizer/internal/logging": true, "github.com/sirupsen/logrus": true,
		"github.com/prometheus/client_golang/prometheus": true,
		// in-house module loggers (round 4, F2): every digital.vasic.* logging package the API imports, plus the prefix rule in
		// sinkPackage for any logging/logger/log package of an own module.
		"digital.vasic.observability/pkg/logging": true, "digital.vasic.containers/pkg/logging": true,
	}
	loggerMethods = map[string]bool{"Info": true, "Infof": true, "Warn": true, "Warnf": true, "Warning": true, "Error": true, "Errorf": true,
		"Debug": true, "Debugf": true, "Fatal": true, "Fatalf": true, "Panic": true, "Panicf": true, "Trace": true, "Tracef": true,
		"Log": true, "Logf": true, "WithField": true, "WithFields": true, "WithError": true, "Print": true, "Printf": true, "Println": true}
	fmtSinks      = map[string]bool{"Print": true, "Printf": true, "Println": true, "Fprint": true, "Fprintf": true, "Fprintln": true, "Errorf": true}
	promSinkNames = map[string]bool{"WithLabelValues": true, "With": true, "GetMetricWithLabelValues": true, "GetMetricWith": true, "CurryWith": true, "MustCurryWith": true}
	bannedGin     = map[string]bool{"Default": true, "Recovery": true, "RecoveryWithWriter": true, "CustomRecovery": true, "CustomRecoveryWithWriter": true,
		"Logger": true, "LoggerWithWriter": true, "LoggerWithConfig": true}
	// keyed readers: taint only for a credential-named or non-literal key.
	keyedReaders = map[string]bool{
		"github.com/gin-gonic/gin.Context.Param": true, "github.com/gin-gonic/gin.Context.Query": true,
		"github.com/gin-gonic/gin.Context.DefaultQuery": true, "github.com/gin-gonic/gin.Context.GetQuery": true,
		"github.com/gin-gonic/gin.Context.GetHeader": true, "github.com/gin-gonic/gin.Context.PostForm": true,
		"github.com/gin-gonic/gin.Context.Cookie": true,
		"net/http.Header.Get":                     true, "net/http.Header.Values": true, "net/url.Values.Get": true, "net/http.Request.FormValue": true,
		"net/http.Request.PostFormValue": true,
	}
	// inherent sources: methods whose result is request-derived for any argument.
	methodSources = map[string]bool{
		"net/http.Request.Referer": true, "net/http.Request.Cookies": true,
		"net/http.Request.BasicAuth": true,
	}
	urlFields   = map[string]bool{"Path": true, "RawPath": true, "RawQuery": true, "Fragment": true, "RawFragment": true, "User": true, "Opaque": true}
	requestLike = map[string]bool{
		"net/http.Request": true, "net/url.URL": true, "net/http.Header": true, "github.com/gin-gonic/gin.Context": true,
		"github.com/gin-gonic/gin.Params": true,
	}
)

type tpkg struct {
	path  string
	rel   string // module-relative dir
	files []*ast.File
	names []string // file paths relative to module root
	info  *types.Info
	pkg   *types.Package
}

type lpkg struct {
	ImportPath string
	Dir        string
	GoFiles    []string
	CgoFiles   []string
	Export     string
	Standard   bool
	DepOnly    bool
	ImportMap  map[string]string
	Module     *struct{ Path string }
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	wd, _ := os.Getwd()
	root, err := filepath.Abs(filepath.Join(wd, "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func loadTyped(t *testing.T, root string, patterns ...string) ([]*tpkg, *token.FileSet) {
	t.Helper()
	args := append([]string{"list", "-e", "-export", "-deps", "-json=ImportPath,Dir,GoFiles,CgoFiles,Export,Standard,DepOnly,ImportMap,Module"}, patterns...)
	cmd := exec.Command("go", args...)
	cmd.Dir = root
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		t.Fatalf("go list: %v\n%s", err, errb.String())
	}
	exports := map[string]string{}
	var mods []lpkg
	dec := json.NewDecoder(&out)
	for dec.More() {
		var p lpkg
		if err := dec.Decode(&p); err != nil {
			t.Fatal(err)
		}
		for k, v := range p.ImportMap {
			if _, ok := exports[k]; !ok {
				if e := exports[v]; e != "" {
					exports[k] = e
				}
			}
		}
		if p.Export != "" {
			exports[p.ImportPath] = p.Export
		}
		if !p.DepOnly && !p.Standard && p.Module != nil && p.Module.Path == "catalogizer" {
			mods = append(mods, p)
		}
	}
	fset := token.NewFileSet()
	imp := importer.ForCompiler(fset, "gc", func(path string) (io.ReadCloser, error) {
		f := exports[path]
		if f == "" {
			return nil, fmt.Errorf("no export data for %s", path)
		}
		return os.Open(f)
	})
	var res []*tpkg
	for _, p := range mods {
		tp := &tpkg{path: p.ImportPath, info: &types.Info{
			Types: map[ast.Expr]types.TypeAndValue{}, Defs: map[*ast.Ident]types.Object{}, Uses: map[*ast.Ident]types.Object{},
			Selections: map[*ast.SelectorExpr]*types.Selection{},
		}}
		for _, gf := range append(append([]string{}, p.GoFiles...), p.CgoFiles...) {
			full := filepath.Join(p.Dir, gf)
			f, err := parser.ParseFile(fset, full, nil, parser.SkipObjectResolution)
			if err != nil {
				t.Fatalf("parse %s: %v", full, err)
			}
			tp.files = append(tp.files, f)
			rel, _ := filepath.Rel(root, full)
			tp.names = append(tp.names, filepath.ToSlash(rel))
		}
		var typeErrs []string
		conf := types.Config{Importer: imp.(types.Importer), FakeImportC: true, Error: func(err error) { typeErrs = append(typeErrs, err.Error()) }}
		tp.pkg, _ = conf.Check(p.ImportPath, fset, tp.files, tp.info)
		if len(typeErrs) > 0 {
			t.Errorf("type errors in analysed package %s (%d): %s (a package that does not type-check is analysed blind)", p.ImportPath, len(typeErrs), typeErrs[0])
		}
		res = append(res, tp)
	}
	return res, fset
}

// ---- analysis ----

type finding struct{ File, Func, Kind, Detail, Pos string }

func (f finding) key() string { return f.File + ":" + f.Func }

type analyzer struct {
	fset      *token.FileSet
	pkgs      []*tpkg
	summaries map[string]bool // funcKey -> returns request-derived data
	findings  []finding
	// round 4 (F2): cross-function sink summaries. sinkParams[funcKey][i] = parameter i reaches a logging/metric/error sink inside
	// the function (directly or through another summarised function), so a call passing request data there is itself a sink.
	sinkParams map[string]map[int]bool
	seed       map[types.Object]bool // when set, runFunc starts with exactly these objects tainted (summary mode)
	sinkHit    bool                  // set by runFunc when a tainted value reached a sink
}

func derefNamed(t types.Type) (pkg, name string) {
	for {
		p, ok := t.(*types.Pointer)
		if !ok {
			break
		}
		t = p.Elem()
	}
	if n, ok := t.(*types.Named); ok && n.Obj() != nil && n.Obj().Pkg() != nil {
		return n.Obj().Pkg().Path(), n.Obj().Name()
	}
	return "", ""
}

// callee returns "pkg.Recv.Name" (methods) or "pkg.Name" (functions); "" when unresolved.
func (a *analyzer) callee(info *types.Info, call *ast.CallExpr) (pkg, recv, name string) {
	fun := ast.Unparen(call.Fun)
	switch f := fun.(type) {
	case *ast.SelectorExpr:
		if sel := info.Selections[f]; sel != nil {
			if fn, ok := sel.Obj().(*types.Func); ok {
				rp, rn := derefNamed(sel.Recv())
				if rn == "" && fn.Pkg() != nil {
					rp = fn.Pkg().Path()
				}
				return rp, rn, fn.Name()
			}
			return "", "", ""
		}
		if fn, ok := info.Uses[f.Sel].(*types.Func); ok && fn.Pkg() != nil {
			return fn.Pkg().Path(), "", fn.Name()
		}
	case *ast.Ident:
		if fn, ok := info.Uses[f].(*types.Func); ok && fn.Pkg() != nil {
			return fn.Pkg().Path(), "", fn.Name()
		}
	}
	return "", "", ""
}

func qual(pkg, recv, name string) string {
	if recv != "" {
		return pkg + "." + recv + "." + name
	}
	return pkg + "." + name
}

func nonStringBasic(t types.Type) bool {
	if t == nil {
		return false
	}
	b, ok := t.Underlying().(*types.Basic)
	if !ok {
		return false
	}
	return b.Info()&types.IsString == 0
}

func literalKey(e ast.Expr) (string, bool) {
	if bl, ok := ast.Unparen(e).(*ast.BasicLit); ok && bl.Kind == token.STRING {
		s, err := strconv.Unquote(bl.Value)
		return s, err == nil
	}
	return "", false
}

// routeParamKey: path parameters named like an id are identifiers, not bearer secrets ("sessionId" of a playback session).
func routeParamKey(k string) bool {
	l := strings.ToLower(k)
	if strings.Contains(l, "session") {
		return false
	}
	return credentialKey(k)
}

func credentialKey(k string) bool {
	l := strings.ToLower(k)
	return SensitiveKey(k) || secretSegmentKeys[l] || headerNames[l]
}

type fnCtx struct {
	info    *types.Info
	tainted map[types.Object]bool
}

func (a *analyzer) taint(c *fnCtx, e ast.Expr) bool {
	if e == nil {
		return false
	}
	e = ast.Unparen(e)
	if tv, ok := c.info.Types[e]; ok {
		if tv.Value != nil || nonStringBasic(tv.Type) {
			return false
		}
	}
	switch x := e.(type) {
	case *ast.Ident:
		if obj := c.info.Uses[x]; obj != nil {
			return c.tainted[obj]
		}
		if obj := c.info.Defs[x]; obj != nil {
			return c.tainted[obj]
		}
	case *ast.FuncLit:
		// a closure that returns request data taints the variable it is assigned to (calling it yields that data)
		ret := false
		ast.Inspect(x.Body, func(n ast.Node) bool {
			if rs, ok := n.(*ast.ReturnStmt); ok {
				for _, r := range rs.Results {
					if a.taint(c, r) {
						ret = true
					}
				}
			}
			return !ret
		})
		return ret
	case *ast.SelectorExpr:
		if sel := c.info.Selections[x]; sel != nil && sel.Kind() == types.MethodVal {
			// method value (f := r.Referer / u.String): calling f later yields what the call would have
			if fn, ok := sel.Obj().(*types.Func); ok {
				rp, rn := derefNamed(sel.Recv())
				if rn == "" && fn.Pkg() != nil {
					rp = fn.Pkg().Path()
				}
				q := qual(rp, rn, fn.Name())
				if methodSources[q] || keyedReaders[q] {
					return true
				}
			}
			return a.taint(c, x.X)
		}
		if sel := c.info.Selections[x]; sel != nil && sel.Kind() == types.FieldVal {
			rp, rn := derefNamed(sel.Recv())
			// An inbound request's URL and target are sources; fields/methods of ANY url.URL carry data only when the URL
			// itself is request-derived (so an outbound client URL being logged is not a request leak).
			if rp == "net/http" && rn == "Request" && (x.Sel.Name == "URL" || x.Sel.Name == "RequestURI") {
				return true
			}
			if rp == "github.com/gin-gonic/gin" && rn == "LogFormatterParams" && (x.Sel.Name == "Path" || x.Sel.Name == "ErrorMessage") {
				return true // gin's access-log inputs: the raw path and the handler error text
			}
			if rp == "net/url" && rn == "URL" && !urlFields[x.Sel.Name] {
				return false // Scheme / Host carry no path, query or userinfo
			}
			return a.taint(c, x.X) // field of a tainted aggregate
		}
	case *ast.CallExpr:
		return a.taintCall(c, x)
	case *ast.BinaryExpr:
		return a.taint(c, x.X) || a.taint(c, x.Y)
	case *ast.StarExpr:
		return a.taint(c, x.X)
	case *ast.UnaryExpr:
		return a.taint(c, x.X)
	case *ast.IndexExpr:
		if call, ok := ast.Unparen(x.X).(*ast.CallExpr); ok {
			if p, r, n := a.callee(c.info, call); p == "github.com/gorilla/mux" && r == "" && n == "Vars" {
				if k, lit := literalKey(x.Index); lit {
					return routeParamKey(k)
				}
				return true
			}
		}
		return a.taint(c, x.X)
	case *ast.SliceExpr:
		return a.taint(c, x.X)
	case *ast.TypeAssertExpr:
		return a.taint(c, x.X)
	case *ast.CompositeLit:
		for _, el := range x.Elts {
			if kv, ok := el.(*ast.KeyValueExpr); ok {
				el = kv.Value
			}
			if a.taint(c, el) {
				return true
			}
		}
	}
	return false
}

func (a *analyzer) taintCall(c *fnCtx, call *ast.CallExpr) bool {
	// type conversion string(x), []byte(x)
	if tv, ok := c.info.Types[call.Fun]; ok && tv.IsType() {
		return len(call.Args) == 1 && a.taint(c, call.Args[0])
	}
	pkg, recv, name := a.callee(c.info, call)
	if pkg == "catalogizer/internal/logredact" {
		return false // the sanctioned redaction
	}
	if pkg == "" && a.taint(c, call.Fun) {
		return true // call of a closure / method value that yields request data
	}
	q := qual(pkg, recv, name)
	if pkg == "net/url" && recv == "URL" && (name == "Hostname" || name == "Port" || name == "Scheme") {
		return false // host and port carry no path, query or userinfo (same rule as the Host/Scheme fields)
	}
	if methodSources[q] {
		return true
	}
	if keyedReaders[q] {
		if len(call.Args) == 0 {
			return true
		}
		if k, lit := literalKey(call.Args[0]); lit {
			if q == "github.com/gin-gonic/gin.Context.Param" {
				return routeParamKey(k)
			}
			return credentialKey(k)
		}
		return true
	}
	if pkg == "github.com/gorilla/mux" && recv == "" && name == "Vars" {
		return false // taint is decided at the index expression
	}
	if a.summaries[funcKey(pkg, recv, name)] {
		return true
	}
	// propagation through helpers: any tainted argument or receiver taints a TEXT-LIKE result (string, []byte, []string, maps,
	// any) and the error of a stdlib parser (strconv/url errors quote their input); struct results and errors of module or
	// third-party calls do not carry the text (keeps the analysis precise enough to have an empty, reviewed allow-list).
	if !textLike(c.info.TypeOf(call), pkg) {
		return false
	}
	for _, arg := range call.Args {
		if a.taint(c, arg) {
			return true
		}
	}
	if sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr); ok && c.info.Selections[sel] != nil {
		return a.taint(c, sel.X)
	}
	return false
}

func textLike(t types.Type, calleePkg string) bool {
	if t == nil {
		return false
	}
	if tup, ok := t.(*types.Tuple); ok {
		for i := 0; i < tup.Len(); i++ {
			if textLike(tup.At(i).Type(), calleePkg) {
				return true
			}
		}
		return false
	}
	if n, ok := t.(*types.Named); ok && n.Obj().Name() == "error" && n.Obj().Pkg() == nil {
		if calleePkg == "database/sql" || calleePkg == "context" {
			return false // driver errors name the constraint/column, never the bound argument values
		}
		return !strings.Contains(strings.SplitN(calleePkg, "/", 2)[0], ".") && !strings.HasPrefix(calleePkg, "catalogizer")
	}
	switch u := t.Underlying().(type) {
	case *types.Basic:
		return u.Info()&types.IsString != 0
	case *types.Slice:
		return textLike(u.Elem(), calleePkg)
	case *types.Map:
		return textLike(u.Elem(), calleePkg)
	case *types.Interface:
		return true
	}
	return false
}

func funcKey(pkg, recv, name string) string { return pkg + "|" + recv + "|" + name }

func requestLikeType(t types.Type) bool {
	if t == nil {
		return false
	}
	p, n := derefNamed(t)
	return requestLike[p+"."+n]
}

func (a *analyzer) isSink(pkg, recv, name string) bool {
	if pkg == "fmt" && fmtSinks[name] {
		return true
	}
	if pkg == "errors" && name == "New" {
		return true
	}
	if sinkPackage(pkg) {
		if pkg == "github.com/prometheus/client_golang/prometheus" {
			return promSinkNames[name]
		}
		return true
	}
	return name == "ReportNonFatal"
}

// sinkPackage: a listed logging package, or any logging/logger/log package of an own module (digital.vasic.*).
func sinkPackage(pkg string) bool {
	if sinkPkgs[pkg] {
		return true
	}
	if strings.HasPrefix(pkg, "digital.vasic.") {
		last := pkg[strings.LastIndex(pkg, "/")+1:]
		return last == "logging" || last == "logger" || last == "log"
	}
	return false
}

// loggerLikeMethod: a call of a logging-shaped method (Info, Warn, WithField ...) on an interface or on a type whose name or
// package says "log" - module-defined logger interfaces and wrappers (round-4 F2) that no package list can enumerate.
func loggerLikeMethod(recvPkg, recv, name string) bool {
	if !loggerMethods[name] || recv == "" {
		return false
	}
	if strings.Contains(strings.ToLower(recv), "log") {
		return true
	}
	last := recvPkg[strings.LastIndex(recvPkg, "/")+1:]
	return strings.Contains(strings.ToLower(last), "log")
}

func isStdStream(info *types.Info, e ast.Expr) bool {
	sel, ok := ast.Unparen(e).(*ast.SelectorExpr)
	if !ok || (sel.Sel.Name != "Stderr" && sel.Sel.Name != "Stdout") {
		return false
	}
	o := info.Uses[sel.Sel]
	return o != nil && o.Pkg() != nil && o.Pkg().Path() == "os"
}

// stdStreamSink: direct writes of text to stderr/stdout that bypass every logger (os.Stderr.WriteString, io.WriteString(os.Stderr,..),
// the println builtin).
func stdStreamSink(info *types.Info, call *ast.CallExpr, pkg, recv, name string) bool {
	if pkg == "os" && recv == "File" && (name == "Write" || name == "WriteString") {
		if sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr); ok {
			return isStdStream(info, sel.X)
		}
	}
	if pkg == "io" && recv == "" && name == "WriteString" && len(call.Args) > 0 {
		return isStdStream(info, call.Args[0])
	}
	if id, ok := ast.Unparen(call.Fun).(*ast.Ident); ok && (id.Name == "println" || id.Name == "print") {
		_, isBuiltin := info.Uses[id].(*types.Builtin)
		return isBuiltin
	}
	return false
}

func lhsObj(c *fnCtx, e ast.Expr) types.Object {
	switch x := ast.Unparen(e).(type) {
	case *ast.Ident:
		if o := c.info.Defs[x]; o != nil {
			return o
		}
		return c.info.Uses[x]
	case *ast.SelectorExpr:
		return lhsObj(c, x.X)
	case *ast.IndexExpr:
		return lhsObj(c, x.X)
	case *ast.StarExpr:
		return lhsObj(c, x.X)
	}
	return nil
}

func (a *analyzer) runFunc(tp *tpkg, file, fname string, body *ast.BlockStmt, ftype *ast.FuncType, report bool) (returnsTainted bool) {
	if body == nil {
		return false
	}
	c := &fnCtx{info: tp.info, tainted: map[types.Object]bool{}}
	for o := range a.seed {
		c.tainted[o] = true
	}
	mark := func(e ast.Expr) bool {
		if o := lhsObj(c, e); o != nil && !c.tainted[o] && !isErrorType(o.Type()) { // error variables: in-order pass below
			c.tainted[o] = true
			return true
		}
		return false
	}
	for pass := 0; pass < 4; pass++ {
		changed := false
		ast.Inspect(body, func(n ast.Node) bool {
			switch s := n.(type) {
			case *ast.AssignStmt:
				if len(s.Lhs) == len(s.Rhs) {
					for i := range s.Lhs {
						if a.taint(c, s.Rhs[i]) && mark(s.Lhs[i]) {
							changed = true
						}
					}
				} else if len(s.Rhs) == 1 && a.taint(c, s.Rhs[0]) {
					cp := ""
					if call, ok := ast.Unparen(s.Rhs[0]).(*ast.CallExpr); ok {
						cp, _, _ = a.callee(c.info, call)
					}
					for _, l := range s.Lhs {
						if textLike(c.info.TypeOf(l), cp) && mark(l) { // only TEXT-LIKE results carry the data
							changed = true
						}
					}
				}
			case *ast.ValueSpec:
				for i, nm := range s.Names {
					var rhs ast.Expr
					if len(s.Values) == len(s.Names) {
						rhs = s.Values[i]
					} else if len(s.Values) == 1 {
						rhs = s.Values[0]
					}
					if rhs != nil && a.taint(c, rhs) && mark(nm) {
						changed = true
					}
				}
			case *ast.SendStmt:
				if a.taint(c, s.Value) && mark(s.Chan) { // ch <- tainted
					changed = true
				}
			case *ast.CallExpr:
				// a builder / buffer / writer that is fed request data carries it: sb.WriteString(t), buf.Write(t), fmt.Fprintf(&sb, .., t)
				cp, cr, cn := a.callee(c.info, s)
				feeds := false
				for _, arg := range s.Args {
					if a.taint(c, arg) {
						feeds = true
					}
				}
				if feeds {
					if sel, ok := ast.Unparen(s.Fun).(*ast.SelectorExpr); ok && c.info.Selections[sel] != nil &&
						(cp == "strings" || cp == "bytes" || cp == "bufio" || cp == "net/url" || cp == "encoding/json") && cr != "" {
						if mark(sel.X) {
							changed = true
						}
					}
					if cp == "fmt" && (cn == "Fprint" || cn == "Fprintf" || cn == "Fprintln") && len(s.Args) > 0 {
						if mark(s.Args[0]) {
							changed = true
						}
					}
				}
			case *ast.RangeStmt:
				if a.taint(c, s.X) {
					if s.Key != nil && mark(s.Key) {
						changed = true
					}
					if s.Value != nil && mark(s.Value) {
						changed = true
					}
				}
			}
			return true
		})
		if !changed {
			break
		}
	}
	// Reporting / summary walk in source order. `error` variables are re-assigned all the time, so they get a STRONG update
	// (taint := taint of the right-hand side, applied after the statement's own sinks are checked); every other variable keeps
	// the union from the fixpoint above (sound for branches).
	var stack []ast.Node
	ast.Inspect(body, func(n ast.Node) bool {
		if n == nil {
			top := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if as, ok := top.(*ast.AssignStmt); ok {
				a.errUpdate(c, as)
			}
			return true
		}
		stack = append(stack, n)
		switch s := n.(type) {
		case *ast.ReturnStmt:
			for _, r := range s.Results {
				if a.taint(c, r) {
					returnsTainted = true
				}
			}
		case *ast.CallExpr:
			pkg, recv, name := a.callee(tp.info, s)
			pos := a.fset.Position(s.Pos()).String()
			if report && pkg == "github.com/gin-gonic/gin" && recv == "" && bannedGin[name] {
				a.findings = append(a.findings, finding{file, fname, "banned-gin-constructor", "gin." + name, pos})
			}
			if report && pkg == "github.com/gin-gonic/gin" && recv == "" && name == "LoggerWithFormatter" && len(s.Args) == 1 {
				// the formatter's return value IS the access-log line: a tainted return is a leak
				if fl, ok := ast.Unparen(s.Args[0]).(*ast.FuncLit); ok {
					ast.Inspect(fl.Body, func(m ast.Node) bool {
						if rs, ok := m.(*ast.ReturnStmt); ok {
							for _, r := range rs.Results {
								if a.taint(c, r) {
									a.findings = append(a.findings, finding{file, fname, "tainted-into-log-formatter", "gin.LoggerWithFormatter", pos})
								}
							}
						}
						return true
					})
				}
			}
			sink := a.isSink(pkg, recv, name) || loggerLikeMethod(pkg, recv, name) || stdStreamSink(tp.info, s, pkg, recv, name)
			wrapper := a.sinkParams[funcKey(pkg, recv, name)]
			if sink || len(wrapper) > 0 {
				args := s.Args
				off := 0
				if sel, ok := ast.Unparen(s.Fun).(*ast.SelectorExpr); ok && tp.info.Selections[sel] != nil && requestLikeType(tp.info.TypeOf(sel.X)) {
					args = append([]ast.Expr{sel.X}, args...)
					off = 1
				}
				for i, arg := range args {
					kind := ""
					switch {
					case a.taint(c, arg):
						if sink {
							kind = "tainted-into-sink"
						} else if wrapper[i-off] || (i-off >= 0 && wrapperVariadic(wrapper, i-off)) {
							kind = "tainted-into-wrapper-sink"
						}
					case sink && requestLikeType(tp.info.TypeOf(arg)):
						kind = "whole-request-into-sink"
					}
					if kind != "" {
						a.sinkHit = true
						if report {
							a.findings = append(a.findings, finding{file, fname, kind, qual(pkg, recv, name), pos})
						}
					}
				}
			}
		}
		return true
	})
	return returnsTainted
}

func isAnyType(t types.Type) bool {
	i, ok := t.Underlying().(*types.Interface)
	return ok && i.NumMethods() == 0
}

func isErrorType(t types.Type) bool {
	n, ok := t.(*types.Named)
	return ok && n.Obj().Name() == "error" && n.Obj().Pkg() == nil
}

func (a *analyzer) errUpdate(c *fnCtx, as *ast.AssignStmt) {
	rhsTaint := func(i int) bool {
		switch {
		case len(as.Lhs) == len(as.Rhs):
			return a.taint(c, as.Rhs[i])
		case len(as.Rhs) == 1:
			return a.taint(c, as.Rhs[0])
		}
		return false
	}
	for i, l := range as.Lhs {
		id, ok := ast.Unparen(l).(*ast.Ident)
		if !ok {
			continue
		}
		o := c.info.Defs[id]
		if o == nil {
			o = c.info.Uses[id]
		}
		if o == nil || !isErrorType(o.Type()) {
			continue
		}
		t := rhsTaint(i)
		if t && len(as.Lhs) != len(as.Rhs) && len(as.Rhs) == 1 {
			// tuple result: the error only carries the data when the callee's errors quote their input
			cp := ""
			if call, ok := ast.Unparen(as.Rhs[0]).(*ast.CallExpr); ok {
				cp, _, _ = a.callee(c.info, call)
			}
			t = textLike(o.Type(), cp)
		}
		if t {
			c.tainted[o] = true
		} else {
			delete(c.tainted, o)
		}
	}
}

func recvName(fd *ast.FuncDecl) string {
	if fd.Recv == nil || len(fd.Recv.List) == 0 {
		return ""
	}
	t := fd.Recv.List[0].Type
	if st, ok := t.(*ast.StarExpr); ok {
		t = st.X
	}
	if ix, ok := t.(*ast.IndexExpr); ok {
		t = ix.X
	}
	if id, ok := t.(*ast.Ident); ok {
		return id.Name
	}
	return ""
}

func scanTyped(t *testing.T, root string, patterns ...string) []finding {
	pkgs, fset := loadTyped(t, root, patterns...)
	a := &analyzer{fset: fset, pkgs: pkgs, summaries: map[string]bool{}, sinkParams: map[string]map[int]bool{}}
	// summaries to a fixpoint (no reporting)
	for round := 0; round < 6; round++ {
		changed := false
		for _, tp := range pkgs {
			for _, f := range tp.files {
				for _, d := range f.Decls {
					fd, ok := d.(*ast.FuncDecl)
					if !ok || fd.Body == nil || fd.Type.Results == nil {
						continue
					}
					k := funcKey(tp.path, recvName(fd), fd.Name.Name)
					if !a.summaries[k] && a.runFunc(tp, "", fd.Name.Name, fd.Body, fd.Type, false) {
						a.summaries[k] = true
						changed = true
					}
				}
			}
		}
		if !changed {
			break
		}
	}
	// sink summaries to a fixpoint: seed ONE text-like parameter at a time and see whether it reaches a sink (round-4 F2:
	// module wrappers around a logger are sinks for their callers, transitively).
	for round := 0; round < 4; round++ {
		changed := false
		for _, tp := range pkgs {
			for _, f := range tp.files {
				for _, d := range f.Decls {
					fd, ok := d.(*ast.FuncDecl)
					if !ok || fd.Body == nil {
						continue
					}
					k := funcKey(tp.path, recvName(fd), fd.Name.Name)
					for i, po := range paramObjs(tp, fd.Type) {
						if po == nil || a.sinkParams[k][i] || !(textLike(po.Type(), "") || isAnyType(po.Type())) {
							continue
						}
						a.seed = map[types.Object]bool{po: true}
						a.sinkHit = false
						a.runFunc(tp, "", fd.Name.Name, fd.Body, fd.Type, false)
						a.seed = nil
						if a.sinkHit {
							if a.sinkParams[k] == nil {
								a.sinkParams[k] = map[int]bool{}
							}
							a.sinkParams[k][i] = true
							changed = true
						}
					}
				}
			}
		}
		if !changed {
			break
		}
	}
	for _, tp := range pkgs {
		for i, f := range tp.files {
			for _, d := range f.Decls {
				switch fd := d.(type) {
				case *ast.FuncDecl:
					name := fd.Name.Name
					if r := recvName(fd); r != "" {
						name = r + "." + name
					}
					a.runFunc(tp, tp.names[i], name, fd.Body, fd.Type, true)
				case *ast.GenDecl: // package-level initialisers with func literals
					for _, sp := range d.(*ast.GenDecl).Specs {
						if vs, ok := sp.(*ast.ValueSpec); ok {
							for _, v := range vs.Values {
								if fl, ok := v.(*ast.FuncLit); ok {
									a.runFunc(tp, tp.names[i], "<init>", fl.Body, fl.Type, true)
								}
								if call, ok := v.(*ast.CallExpr); ok {
									wrap := &ast.BlockStmt{List: []ast.Stmt{&ast.ExprStmt{X: call}}}
									a.runFunc(tp, tp.names[i], "<init>", wrap, nil, true)
								}
							}
						}
					}
				}
			}
		}
	}
	// de-duplicate
	seen := map[string]bool{}
	var out []finding
	for _, f := range a.findings {
		k := f.File + "|" + f.Func + "|" + f.Kind + "|" + f.Detail
		if !seen[k] {
			seen[k] = true
			out = append(out, f)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].key()+out[i].Detail < out[j].key()+out[j].Detail })
	return out
}

// reviewedAllow lists the reviewed non-leaking uses, keyed by file + function (no line text). Every entry carries its reason.
// Entry format "file:Func|kind".
var reviewedAllow = map[string]string{
	"internal/middleware/middleware.go:Logger|tainted-into-sink":                                 "path is re-assigned to logredact.Path(path, raw) before the zap field is built (flow-insensitive scanner keeps the first assignment's taint)",
	"tests/mocks/webdav_mock_server.go:MockWebDAVServer.handleRequest|tainted-into-wrapper-sink": "same mock server: its handlers log the mock's own debug path (handleGet/handlePropfind wrappers); test infrastructure",
	"internal/tests/protocol_helper.go:WebDAVMockServer.handleRequest|tainted-into-wrapper-sink": "test-only WebDAV mock in a _test-support package (never linked into the API binary); logs its own debug path",
	"tests/mocks/webdav_mock_server.go:MockWebDAVServer.handleRequest|tainted-into-sink":         "test-infrastructure mock WebDAV server (never compiled into the API binary); logs its own debug path at Debug level, no credential path",
}

// evaluateFindings returns the failures: findings not on the allow-list, and allow-list entries no finding hits any more.
func evaluateFindings(got []finding, allow map[string]string) []string {
	var bad []string
	hit := map[string]bool{}
	for _, f := range got {
		k := f.key() + "|" + f.Kind
		if _, ok := allow[k]; ok {
			hit[k] = true
			continue
		}
		bad = append(bad, fmt.Sprintf("%s %s in %s (%s)", f.Kind, f.Detail, f.key(), f.Pos))
	}
	for k := range allow {
		if !hit[k] {
			bad = append(bad, "stale allow-list entry (no longer hit): "+k)
		}
	}
	sort.Strings(bad)
	return bad
}

func TestEvaluateFindings_StaleAndUnlisted(t *testing.T) {
	f := finding{File: "a.go", Func: "F", Kind: "tainted-into-sink", Detail: "log.Print", Pos: "a.go:1"}
	if bad := evaluateFindings([]finding{f}, nil); len(bad) != 1 {
		t.Fatalf("unlisted finding not reported: %v", bad)
	}
	if bad := evaluateFindings([]finding{f}, map[string]string{"a.go:F|tainted-into-sink": "reviewed"}); len(bad) != 0 {
		t.Fatalf("allow-listed finding reported: %v", bad)
	}
	if bad := evaluateFindings(nil, map[string]string{"a.go:F|tainted-into-sink": "reviewed"}); len(bad) != 1 || !strings.Contains(bad[0], "stale") {
		t.Fatalf("stale entry not reported: %v", bad)
	}
	// an allow-list entry is keyed by file+function+kind: the same function with another kind, or another function, is NOT covered
	g := finding{File: "a.go", Func: "G", Kind: "tainted-into-sink", Detail: "log.Print", Pos: "a.go:2"}
	if bad := evaluateFindings([]finding{g}, map[string]string{"a.go:F|tainted-into-sink": "reviewed"}); len(bad) != 2 {
		t.Fatalf("allow-list leaked to another function: %v", bad)
	}
}

func TestTypedScan_ServerCodeHasNoRawRequestDataInSinks(t *testing.T) {
	if testing.Short() {
		t.Skip("type-checks the whole module (go list -export); run without -short")
	}
	root := moduleRoot(t)
	got := scanTyped(t, root, "./...")
	bad := evaluateFindings(got, reviewedAllow)
	if len(bad) > 0 {
		t.Fatalf("request-derived data reaches a logging/metrics/error sink without logredact (or stale allow-list):\n%s", strings.Join(bad, "\n"))
	}
	if len(got) == 0 && len(reviewedAllow) > 0 {
		t.Fatal("scanner found nothing but an allow-list exists: blind")
	}
}

// Control needles: every shape below MUST be reported on the leaky fixtures (>= 12, including every shape the round-2 review
// used), and nothing may be reported on the clean fixtures. An instrument that cannot see proves nothing.
func TestTypedScan_ControlNeedles(t *testing.T) {
	if testing.Short() {
		t.Skip("type-checks fixtures (go list -export); run without -short")
	}
	root := moduleRoot(t)
	leaky := scanTyped(t, root, "./internal/logredact/testdata/fixtures/leaky")
	want := []string{
		"Needle01DirectPrintf", "Needle02AssignThenLog", "Needle03RequestURI", "Needle04RawQuery", "Needle05WholeRequestV",
		"Needle06WholeURL", "Needle07ParamToken", "Needle08EscapedPath", "Needle09ReqVarName", "Needle10Referer",
		"Needle11MuxVars", "Needle12NormalizePathReturn", "Needle13ZapField", "Needle14MetricLabel", "Needle15Concat",
		"Needle16Errorf", "Needle17HeaderGet", "Needle18GinDefault", "Needle19GinRecovery", "Needle20WholeContext",
		"Needle21QueryMethodThenGet", "Needle22Range", "Needle23LogFormatterRawPath", "Needle24LogFormatterErrorMessage", "Needle25HeaderNameOnlyInHeaderList",
		"Needle26ObsLoggingWithField", "Needle27ContainersLogging", "Needle28ModuleWrapper", "Needle29TransitiveWrapper",
		"Needle30ModuleInterfaceLogger", "Needle31ClosureReturningTainted", "Needle32MethodValue", "Needle33StringsBuilder",
		"Needle34FprintfIntoBuilder", "Needle35OsStderrWriteString", "Needle36OsStderrWrite", "Needle37Channel",
		"Needle38PrintlnBuiltin", "Needle39ErrorfWrapIntoLog",
	}
	byFunc := map[string]bool{}
	for _, f := range leaky {
		byFunc[f.Func] = true
	}
	for _, w := range want {
		if !byFunc[w] {
			t.Errorf("scanner BLIND to %s", w)
		}
	}
	if len(want) < 12 {
		t.Fatal("need >= 12 needles")
	}
	clean := scanTyped(t, root, "./internal/logredact/testdata/fixtures/clean")
	for _, f := range clean {
		t.Errorf("false positive on clean fixture: %s %s in %s", f.Kind, f.Detail, f.key())
	}
	// the clean package must actually have been analysed (a package that failed to load would pass vacuously)
	pk, _ := loadTyped(t, root, "./internal/logredact/testdata/fixtures/clean")
	if len(pk) != 1 || len(pk[0].files) == 0 {
		t.Fatal("clean fixture package not loaded: vacuous pass")
	}
}

// Middleware / logger inventory (R2-1): every module package importing a request-logging or HTTP-instrumentation library is
// listed with the reason it does not log raw request data. A NEW such import fails the test until it is reviewed (behaviourally
// where it can log). The third-party chain in main is gin.LoggerWithFormatter (own format, redacted), the own Recovery, and
// in-house middleware; none of the libraries below wraps a router.
var reviewedInstrumentationImports = map[string]string{
	"net/http/pprof": "admin+auth guarded profiler handlers; they log nothing about the request target",
	"github.com/prometheus/client_golang/prometheus/promhttp": "serves /metrics; no ErrorLog configured, so it never logs the request",
}

func TestInstrumentationImportsAreReviewed(t *testing.T) {
	if testing.Short() {
		t.Skip("type-checks the module")
	}
	pkgs, _ := loadTyped(t, moduleRoot(t), "./...")
	watch := []string{"gin-contrib", "otel", "opentelemetry", "rs/cors", "swaggo", "sentry", "newrelic", "datadog", "logrus", "zerolog",
		"net/http/httputil", "net/http/pprof", "promhttp", "go-chi", "negroni", "alice", "handlers\"", "gorilla/handlers"}
	seen := map[string]bool{}
	var bad []string
	for _, tp := range pkgs {
		for i, f := range tp.files {
			for _, im := range f.Imports {
				p, _ := strconv.Unquote(im.Path.Value)
				for _, w := range watch {
					if strings.Contains(p, strings.Trim(w, "\"")) && p != "catalogizer/handlers" && !strings.HasPrefix(p, "catalogizer/") {
						seen[p] = true
						if _, ok := reviewedInstrumentationImports[p]; !ok {
							bad = append(bad, tp.names[i]+" imports unreviewed "+p)
						}
					}
				}
			}
		}
	}
	for p := range reviewedInstrumentationImports {
		if !seen[p] {
			bad = append(bad, "stale reviewed import (no longer imported): "+p)
		}
	}
	if len(bad) > 0 {
		sort.Strings(bad)
		t.Fatalf("%s", strings.Join(bad, "\n"))
	}
	if len(seen) == 0 {
		t.Fatal("inventory blind: no instrumentation import seen at all")
	}
}

// wrapperVariadic: an argument beyond the last summarised parameter of a variadic wrapper counts as that last parameter.
func wrapperVariadic(w map[int]bool, i int) bool {
	max := -1
	for k := range w {
		if k > max {
			max = k
		}
	}
	return max >= 0 && i > max && w[max]
}

// paramObjs returns the parameter objects of a function declaration, in order (blank and unnamed ones as nil).
func paramObjs(tp *tpkg, ft *ast.FuncType) []types.Object {
	var out []types.Object
	if ft == nil || ft.Params == nil {
		return nil
	}
	for _, fld := range ft.Params.List {
		if len(fld.Names) == 0 {
			out = append(out, nil)
			continue
		}
		for _, n := range fld.Names {
			out = append(out, tp.info.Defs[n])
		}
	}
	return out
}
