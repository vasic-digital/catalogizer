// Package errleak is a source-level instrument that finds HTTP response sinks which put the text of an
// error value on the wire (CAT-WP02-ERRLEAK, §11.4.276(C) class-wide). It lives in its own small package so
// it can be run (and mutated) without compiling the large handlers package.
package errleak

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
	"strings"
)

var wrapperSinkRE = regexp.MustCompile(`(?i)^(send|write|respond|fail|abort|reply)[a-z]*(error|err)[a-z]*$`)

// safeWrappers map an error to fixed texts themselves (read and classified individually).
var safeWrappers = map[string]bool{"writeJobError": true, "writeLogServiceError": true, "writeFixedLogError": true,
	"respondPDFError": true, "respondInternal": true, "respondBadBody": true, "stdError": true, "syncFail": true,
	// respondSyncClassError matches sentinel errors with errors.Is and answers authored literals only; its own body is
	// still swept, so a leak added inside it is reported (MAIN-ERRLEAK).
	"respondSyncClassError": true}

var sinkNames = map[string]bool{"Error": true, "JSON": true, "String": true, "AbortWithStatusJSON": true, "Write": true,
	"Encode": true, "Fprintf": true, "Fprint": true, "Fprintln": true, "IndentedJSON": true, "SendErrorResponse": true,
	"WriteString": true, "Data": true}

// mappers turn an error into authored client text (each is unit-tested elsewhere).
var mappers = map[string]bool{"validationText": true, "paramRequiredText": true, "localRootMessage": true,
	"rootURLMessage": true, "authClientMessage": true, "sentinelText": true, "syncDetails": true, "rootSpecMessage": true,
	"clientErrorText": true, "clientValidationDetails": true}

// pkgMappers lists the sanctioned mappers that live in another package, with the EXACT import path that must
// qualify them (ERRLEAK-FU-D, ELFU-m1): `services.ScrubErrorFor` counts only when the qualifier is an imported
// package whose path is exactly catalogizer/internal/services, not a local variable, not a look-alike path
// (catalogizer/services, x/services, a vendored copy).
var pkgMappers = map[string]string{"ScrubErrorFor": "catalogizer/internal/services"}

// fileCtx carries what the single file tells us about identity: the local name of every import and its path.
type fileCtx struct{ imports map[string]string }

func newFileCtx(af *ast.File) *fileCtx {
	c := &fileCtx{imports: map[string]string{}}
	for _, im := range af.Imports {
		path := strings.Trim(im.Path.Value, "\"`")
		name := path[strings.LastIndex(path, "/")+1:]
		if im.Name != nil {
			name = im.Name.Name
		}
		c.imports[name] = path
	}
	return c
}

// sanctionedIdent reports whether id, used as a bare callee, still names the package-level function the sanctioned
// lists mean. The parser's file-level resolution binds an identifier to a local declaration (parameter, named
// result, :=, var, range variable, closure) or to a same-file declaration; only an unresolved identifier (declared
// in another file of the package) or one bound to a top-level func qualifies. A local or package-level variable of
// that name is a look-alike (ERRLEAK-FU-C, shape S7). A same-package declaration in ANOTHER file cannot be seen.
func sanctionedIdent(id *ast.Ident) bool {
	if id.Obj == nil {
		return true
	}
	_, isDecl := id.Obj.Decl.(*ast.FuncDecl)
	return isDecl
}

// sanctionedPkgSel reports whether s is `pkg.Name` where pkg is an unshadowed import whose path is exactly want.
func (c *fileCtx) sanctionedPkgSel(s *ast.SelectorExpr, want string) bool {
	x, ok := s.X.(*ast.Ident)
	if !ok || x.Obj != nil { // a local/parameter/var of that name is not the package
		return false
	}
	path, imported := c.imports[x.Name]
	return imported && path == want
}

// isMapperCall: a call whose callee is a sanctioned mapper by identity, not by bare name.
func (c *fileCtx) isMapperCall(v *ast.CallExpr) bool {
	switch f := v.Fun.(type) {
	case *ast.Ident:
		return mappers[f.Name] && sanctionedIdent(f)
	case *ast.SelectorExpr:
		if pk, ok := pkgMappers[f.Sel.Name]; ok {
			return c.sanctionedPkgSel(f, pk)
		}
	}
	return false
}

// isSafeWrapperCall: a bare call to the sanctioned package-level wrapper. A method call (x.writeJobError) or a
// shadowed name is some other function and is judged as an ordinary sink (ERRLEAK-FU-D).
func isSafeWrapperCall(v *ast.CallExpr) bool {
	id, ok := v.Fun.(*ast.Ident)
	return ok && safeWrappers[id.Name] && sanctionedIdent(id)
}

func isSentinel(e ast.Expr) bool {
	switch v := e.(type) {
	case *ast.Ident:
		return strings.HasPrefix(v.Name, "Err")
	case *ast.SelectorExpr:
		return strings.HasPrefix(v.Sel.Name, "Err")
	}
	return false
}

func errName(n string) bool {
	l := strings.ToLower(n)
	if l == "stderr" || l == "errout" {
		return false
	}
	return l == "err" || strings.HasSuffix(l, "err")
}

// isStdStream reports whether e names a process stream (CLI output, not an HTTP response).
func isStdStream(e ast.Expr) bool {
	r := strings.ToLower(recv(e))
	return strings.Contains(r, "stderr") || strings.Contains(r, "stdout") || r == "out" || r == "errout"
}

// usesErr reports whether n carries an error value or the text of one; tainted holds local variables that were
// assigned from such an expression (variable-carried errors, R5B-F5).
func (c *fileCtx) usesErr(n ast.Node, tainted map[string]bool) bool {
	found := false
	ast.Inspect(n, func(x ast.Node) bool {
		switch v := x.(type) {
		case *ast.CallExpr:
			if c.isMapperCall(v) {
				return false
			}
			if s, ok := v.Fun.(*ast.SelectorExpr); ok {
				if s.Sel.Name == "Error" && len(v.Args) == 0 && !isSentinel(s.X) {
					found = true
				}
			}
		case *ast.Ident:
			if errName(v.Name) || tainted[v.Name] {
				found = true
			}
		}
		return true
	})
	return found
}

func recv(e ast.Expr) string {
	switch v := e.(type) {
	case *ast.Ident:
		return v.Name
	case *ast.SelectorExpr:
		return recv(v.X) + "." + v.Sel.Name
	}
	return "?"
}

// Site is one offending sink.
type Site struct {
	File, Func string
	Line       int
}

func (s Site) String() string { return fmt.Sprintf("%s:%d (%s)", s.File, s.Line, s.Func) }

func (c *fileCtx) taintOf(body *ast.BlockStmt) map[string]bool {
	t := map[string]bool{}
	for changed := true; changed; {
		changed = false
		ast.Inspect(body, func(n ast.Node) bool {
			as, ok := n.(*ast.AssignStmt)
			if !ok {
				return true
			}
			for i, l := range as.Lhs {
				// m[k] = <error text> taints the map/slice m (a response body assembled before it is written)
				if ix, isIx := l.(*ast.IndexExpr); isIx {
					if base, okb := ix.X.(*ast.Ident); okb && !t[base.Name] && len(as.Rhs) == len(as.Lhs) && c.usesErr(as.Rhs[i], t) {
						t[base.Name] = true
						changed = true
					}
					continue
				}
				id, ok := l.(*ast.Ident)
				if !ok || id.Name == "_" || t[id.Name] || errName(id.Name) {
					continue
				}
				var r ast.Node
				if len(as.Rhs) == len(as.Lhs) {
					r = as.Rhs[i]
				} else if len(as.Rhs) == 1 {
					continue // multi-value call: results are not error text
				}
				if r != nil && c.usesErr(r, t) {
					t[id.Name] = true
					changed = true
				}
			}
			return true
		})
	}
	return t
}

// Sweep lists every response sink in src whose arguments reference an error value or a variable derived from one.
func Sweep(name, src string) ([]Site, error) {
	fset := token.NewFileSet()
	af, err := parser.ParseFile(fset, name, src, 0)
	if err != nil {
		return nil, err
	}
	ctx := newFileCtx(af)
	var out []Site
	for _, d := range af.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok || fd.Body == nil {
			continue
		}
		tainted := ctx.taintOf(fd.Body)
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			c, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			var callee, rc string
			switch f := c.Fun.(type) {
			case *ast.SelectorExpr:
				callee, rc = f.Sel.Name, recv(f.X)
			case *ast.Ident:
				callee = f.Name
			default:
				return true
			}
			isSink := sinkNames[callee] || wrapperSinkRE.MatchString(callee)
			if !isSink || isSafeWrapperCall(c) || strings.Contains(strings.ToLower(rc), "logger") || rc == "zap" || rc == "log" {
				return true
			}
			if rc == "" && !wrapperSinkRE.MatchString(callee) {
				return true
			}
			if strings.HasPrefix(callee, "Fprint") && len(c.Args) > 0 && isStdStream(c.Args[0]) {
				return true
			}
			for i, a := range c.Args {
				if callee == "SendErrorResponse" && i != 2 {
					continue
				}
				if ctx.usesErr(a, tainted) {
					out = append(out, Site{File: name, Func: fd.Name.Name, Line: fset.Position(c.Pos()).Line})
					break
				}
			}
			return true
		})
	}
	return out, nil
}
