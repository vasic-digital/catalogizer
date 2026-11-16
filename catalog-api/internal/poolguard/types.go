package poolguard

// Type-based call resolution for the head scanner.
//
// The base scanner resolves every helper call by its bare name inside the caller's directory, which is
// conservative but reports a pure method of an unrelated concrete type (models.Permissions.HasPermission)
// as if it were the DB-backed method of the same name. The head scanner type-checks every package of the
// scanned tree with the standard library go/types (no extra module dependency) and uses the result for
// exactly ONE kind of call: a method value selection (types.MethodVal) of a method declared on a
// concrete, non-interface type, whose receiver embeds no unresolved type, and whose method is CLOSED
// (nothing it can run through static calls is chosen at run time or lies outside the reliably
// type-checked tree; see scan). Such a call runs exactly that method and cannot call back, so:
//
//   - the method lives in another package: the call cannot run a same-package helper, it is ruled out;
//   - the method lives in the same package: the call runs exactly that method, which is followed alone.
//
// Every other call (plain function, interface method, type parameter, func-typed field, method value,
// method expression, open method, anything in a package that could not be type-checked reliably) keeps
// the base by-name resolution. A package is unreliable when it has build-constrained files, files the go
// tool ignores (name starting with _ or .), cgo files (import "C": dropped when CGO_ENABLED=0), more than
// one package clause, or a redeclaration (a platform variant always redeclares something, so the
// build-constraint rule and the redeclaration rule overlap on purpose). An import that is not a reliable
// package of the importer's OWN module (nearest go.mod, nested modules included) is left unresolved, so
// every type depending on it is invalid and those calls stay on the by-name path.

import (
	"errors"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// callKind is how a call made inside a function body is resolved.
type callKind int

const (
	byNameCall callKind = iota // base behaviour: every same-directory decl of that bare name
	exactCall                  // static dispatch to one same-package method decl
	ruledOut                   // static dispatch to a method of a concrete type in another package
)

type typedPkg struct {
	dir     string
	files   []*ast.File
	modDir  string
	path    string
	pkg     *types.Package
	info    *types.Info
	ok      bool // type information is reliable
	state   int  // 0 new, 1 checking, 2 done
	failure string
}

type typeWorld struct {
	fset  *token.FileSet
	byDir map[string]*typedPkg
	// byPath maps module dir + "\x00" + import path to the directories carrying that path.
	byPath map[string][]string
}

var knownOS = map[string]bool{
	"aix": true, "android": true, "darwin": true, "dragonfly": true, "freebsd": true, "hurd": true,
	"illumos": true, "ios": true, "js": true, "linux": true, "nacl": true, "netbsd": true, "openbsd": true,
	"plan9": true, "solaris": true, "wasip1": true, "windows": true, "zos": true,
}

var knownArch = map[string]bool{
	"386": true, "amd64": true, "amd64p32": true, "arm": true, "armbe": true, "arm64": true, "arm64be": true,
	"loong64": true, "mips": true, "mipsle": true, "mips64": true, "mips64le": true, "mips64p32": true,
	"mips64p32le": true, "ppc": true, "ppc64": true, "ppc64le": true, "riscv": true, "riscv64": true,
	"s390": true, "s390x": true, "sparc": true, "sparc64": true, "wasm": true,
}

// buildConstrained reports whether the file is only part of some builds (a //go:build or // +build line
// before the package clause, or a _GOOS/_GOARCH file name suffix).
func buildConstrained(path string, f *ast.File) bool {
	for _, cg := range f.Comments {
		if cg.Pos() >= f.Package {
			break
		}
		for _, c := range cg.List {
			if strings.HasPrefix(c.Text, "//go:build") || strings.HasPrefix(c.Text, "// +build") {
				return true
			}
		}
	}
	name := strings.TrimSuffix(filepath.Base(path), ".go")
	parts := strings.Split(name, "_")
	if n := len(parts); n >= 2 {
		last := parts[n-1]
		if knownOS[last] || knownArch[last] {
			return true
		}
		if n >= 3 && knownOS[parts[n-2]] && knownArch[last] {
			return true
		}
	}
	return false
}

// moduleOf finds the go.mod at or above dir and returns its directory and module path.
func moduleOf(dir string, cache map[string][2]string) (string, string) {
	if v, ok := cache[dir]; ok {
		return v[0], v[1]
	}
	var res [2]string
	if b, err := os.ReadFile(filepath.Join(dir, "go.mod")); err == nil {
		for _, ln := range strings.Split(string(b), "\n") {
			ln = strings.TrimSpace(ln)
			if strings.HasPrefix(ln, "module ") {
				p := strings.TrimSpace(strings.TrimPrefix(ln, "module "))
				if i := strings.Index(p, "//"); i >= 0 {
					p = strings.TrimSpace(p[:i])
				}
				res = [2]string{dir, strings.Trim(p, "\"`")}
				break
			}
		}
	} else if parent := filepath.Dir(dir); parent != dir {
		res[0], res[1] = moduleOf(parent, cache)
	}
	cache[dir] = res
	return res[0], res[1]
}

func newTypeWorld(fset *token.FileSet, files map[string][]*ast.File, paths map[string][]string) *typeWorld {
	w := &typeWorld{fset: fset, byDir: map[string]*typedPkg{}, byPath: map[string][]string{}}
	modCache := map[string][2]string{}
	for dir, fl := range files {
		tp := &typedPkg{dir: dir, files: fl, ok: true}
		w.byDir[dir] = tp
		abs, err := filepath.Abs(dir)
		if err != nil {
			tp.ok, tp.failure = false, "abs: "+err.Error()
			continue
		}
		modDir, modPath := moduleOf(abs, modCache)
		if modPath == "" {
			tp.ok, tp.failure = false, "no module"
			continue
		}
		rel, err := filepath.Rel(modDir, abs)
		if err != nil {
			tp.ok, tp.failure = false, "rel: "+err.Error()
			continue
		}
		tp.modDir = modDir
		tp.path = modPath
		if rel != "." {
			tp.path = modPath + "/" + filepath.ToSlash(rel)
		}
		k := modDir + "\x00" + tp.path
		w.byPath[k] = append(w.byPath[k], dir)
		name := ""
		for i, f := range fl {
			if buildConstrained(paths[dir][i], f) {
				tp.ok, tp.failure = false, "build-constrained file "+paths[dir][i]
			}
			if b := filepath.Base(paths[dir][i]); strings.HasPrefix(b, "_") || strings.HasPrefix(b, ".") {
				// The go tool ignores such files; the scanner would type-check declarations no build sees.
				tp.ok, tp.failure = false, "file ignored by the go tool "+paths[dir][i]
			}
			if importsC(f) {
				// A cgo file is dropped by the go tool when CGO_ENABLED=0, so a build may not see it.
				tp.ok, tp.failure = false, "cgo file "+paths[dir][i]
			}
			if name == "" {
				name = f.Name.Name
			} else if name != f.Name.Name {
				tp.ok, tp.failure = false, "several package clauses"
			}
		}
	}
	return w
}

type dirImporter struct {
	w    *typeWorld
	from *typedPkg
}

func (im dirImporter) Import(path string) (*types.Package, error) {
	return im.ImportFrom(path, "", 0)
}

func (im dirImporter) ImportFrom(path, _ string, _ types.ImportMode) (*types.Package, error) {
	if path == "unsafe" {
		return types.Unsafe, nil
	}
	dirs := im.w.byPath[im.from.modDir+"\x00"+path]
	if len(dirs) != 1 {
		return nil, errors.New("not a package of the scanned module")
	}
	tp := im.w.check(dirs[0])
	if tp == nil || !tp.ok || tp.pkg == nil {
		return nil, errors.New("package not reliably type-checked")
	}
	return tp.pkg, nil
}

// check type-checks the package in dir once (lazily, following imports) and returns it.
func (w *typeWorld) check(dir string) *typedPkg {
	tp := w.byDir[dir]
	if tp == nil {
		return nil
	}
	switch tp.state {
	case 1:
		tp.ok, tp.failure = false, "import cycle"
		return tp
	case 2:
		return tp
	}
	tp.state = 1
	if tp.ok {
		info := &types.Info{
			Selections: map[*ast.SelectorExpr]*types.Selection{},
			Defs:       map[*ast.Ident]types.Object{},
			Uses:       map[*ast.Ident]types.Object{},
			Types:      map[ast.Expr]types.TypeAndValue{},
		}
		conf := types.Config{
			Importer:    dirImporter{w: w, from: tp},
			FakeImportC: true,
			Error: func(err error) {
				te, ok := err.(types.Error)
				if !ok {
					return
				}
				if strings.Contains(te.Msg, "redeclared") || strings.Contains(te.Msg, "already declared") ||
					strings.Contains(te.Msg, "method on invalid type") {
					tp.ok, tp.failure = false, te.Msg
				}
			},
		}
		pkg, _ := conf.Check(tp.path, w.fset, tp.files, info)
		if tp.state == 1 && tp.ok { // a cycle may have flipped ok while checking
			tp.pkg, tp.info = pkg, info
		}
	}
	tp.state = 2
	return tp
}

// incompleteMethodSet reports whether the method set of t may be incomplete because some type that
// can contribute methods to it is unresolved (invalid). The checker silently drops the methods of an
// invalid type, so a promoted method it reports could be shadowed in a real build by one it cannot see.
// Walked, transitively: alias targets, pointer elements, the underlying type of a defined type (a
// defined type over an unresolved type has an invalid underlying), embedded struct fields (instantiated
// generic structs are substituted), embedded interfaces and type-set terms of interfaces, and the
// constraint of a type parameter. Non-embedded fields and method signatures do not affect a method set.
// The union and type-parameter branches are defensive: in a program that compiles, neither can sit in a
// method-set-contributing position of a receiver staticMethod accepts (unions occur only in
// constraints, a type parameter cannot be embedded, and interface and type-parameter receivers are
// rejected before this walk), so no fixture can exercise them.
func incompleteMethodSet(t types.Type, seen map[types.Type]bool) bool {
	if t == nil {
		return true
	}
	t = types.Unalias(t)
	if seen[t] {
		return false
	}
	seen[t] = true
	switch v := t.(type) {
	case *types.Basic:
		return v.Kind() == types.Invalid
	case *types.Pointer:
		return incompleteMethodSet(v.Elem(), seen)
	case *types.Named:
		return incompleteMethodSet(v.Underlying(), seen)
	case *types.TypeParam:
		return incompleteMethodSet(v.Constraint(), seen)
	case *types.Struct:
		for i := 0; i < v.NumFields(); i++ {
			if f := v.Field(i); f.Embedded() && incompleteMethodSet(f.Type(), seen) {
				return true
			}
		}
	case *types.Interface:
		for i := 0; i < v.NumEmbeddeds(); i++ {
			if incompleteMethodSet(v.EmbeddedType(i), seen) {
				return true
			}
		}
	case *types.Union:
		for i := 0; i < v.Len(); i++ {
			if incompleteMethodSet(v.Term(i).Type(), seen) {
				return true
			}
		}
	}
	return false
}

// staticMethod returns the method a call dispatches to statically: a method value selection
// (types.MethodVal) of a method declared on a concrete, non-interface type whose receiver's method set
// is complete (incompleteMethodSet). It returns nil for every other call, and for every call in an unreliable
// package.
func (tp *typedPkg) staticMethod(c *ast.CallExpr) *types.Func {
	if tp == nil || !tp.ok || tp.info == nil {
		return nil
	}
	s, ok := c.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil
	}
	sel := tp.info.Selections[s]
	if sel == nil || sel.Kind() != types.MethodVal {
		return nil
	}
	m, ok := sel.Obj().(*types.Func)
	if !ok || m.Pkg() == nil {
		return nil
	}
	sig, ok := m.Type().(*types.Signature)
	if !ok || sig.Recv() == nil || types.IsInterface(sig.Recv().Type()) {
		return nil
	}
	if _, isTP := types.Unalias(sel.Recv()).(*types.TypeParam); isTP {
		return nil
	}
	if incompleteMethodSet(sel.Recv(), map[types.Type]bool{}) {
		return nil
	}
	return m.Origin()
}

// closedEdge classifies a call for the closedness analysis: harmless (a builtin or a conversion: no user
// code runs), a static call to the function or method obj, or open (anything that may run code chosen at
// run time or code outside the reliably type-checked tree).
func (tp *typedPkg) closedEdge(c *ast.CallExpr) (harmless bool, obj *types.Func) {
	if tp == nil || !tp.ok || tp.info == nil {
		return false, nil
	}
	fun := ast.Unparen(c.Fun)
	if tv, ok := tp.info.Types[fun]; ok && tv.IsType() {
		return true, nil
	}
	var id *ast.Ident
	switch f := fun.(type) {
	case *ast.Ident:
		id = f
	case *ast.SelectorExpr:
		if m := tp.staticMethod(c); m != nil {
			return false, m
		}
		if tp.info.Selections[f] != nil {
			return false, nil // field, interface method, method expression: open
		}
		id = f.Sel // qualified identifier pkg.F
	default:
		return false, nil
	}
	switch o := tp.info.Uses[id].(type) {
	case *types.Builtin:
		return true, nil
	case *types.Func:
		if sig, ok := o.Type().(*types.Signature); ok && sig.Recv() == nil && o.Pkg() != nil {
			return false, o.Origin()
		}
	}
	return false, nil
}

// rangesOverFunc reports whether a range statement iterates a function value (range-over-func calls it).
func (tp *typedPkg) rangesOverFunc(r *ast.RangeStmt) bool {
	if tp == nil || tp.info == nil {
		return true
	}
	tv, ok := tp.info.Types[r.X]
	if !ok || tv.Type == nil {
		return true
	}
	switch tv.Type.Underlying().(type) {
	case *types.Signature, *types.Interface:
		return true
	case *types.Basic:
		return tv.Type.Underlying().(*types.Basic).Kind() == types.Invalid
	}
	return false
}

// importsC reports whether f is a cgo file (it imports the pseudo-package "C").
func importsC(f *ast.File) bool {
	for _, im := range f.Imports {
		if im.Path == nil {
			continue
		}
		// The literal may be interpreted ("C") or raw (`C`): compare the unquoted value.
		if v, err := strconv.Unquote(im.Path.Value); err == nil && v == "C" {
			return true
		}
	}
	return false
}
