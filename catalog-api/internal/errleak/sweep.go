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
	"respondPDFError": true, "respondInternal": true, "respondBadBody": true, "stdError": true, "syncFail": true}

var sinkNames = map[string]bool{"Error": true, "JSON": true, "String": true, "AbortWithStatusJSON": true, "Write": true,
	"Encode": true, "Fprintf": true, "Fprint": true, "Fprintln": true, "IndentedJSON": true, "SendErrorResponse": true,
	"WriteString": true, "Data": true}

// mappers turn an error into authored client text (each is unit-tested elsewhere).
var mappers = map[string]bool{"validationText": true, "paramRequiredText": true, "localRootMessage": true,
	"rootURLMessage": true, "authClientMessage": true, "sentinelText": true, "syncDetails": true, "rootSpecMessage": true,
	"clientErrorText": true, "clientValidationDetails": true, "ScrubErrorFor": true}

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
func usesErr(n ast.Node, tainted map[string]bool) bool {
	found := false
	ast.Inspect(n, func(x ast.Node) bool {
		switch v := x.(type) {
		case *ast.CallExpr:
			if id, ok := v.Fun.(*ast.Ident); ok && mappers[id.Name] {
				return false
			}
			if s, ok := v.Fun.(*ast.SelectorExpr); ok {
				if mappers[s.Sel.Name] && s.Sel.Name == "ScrubErrorFor" {
					return false // package-qualified mapper: scrubs secrets from storage probe errors (R5B-3)
				}
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

func taintOf(body *ast.BlockStmt) map[string]bool {
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
					if base, okb := ix.X.(*ast.Ident); okb && !t[base.Name] && len(as.Rhs) == len(as.Lhs) && usesErr(as.Rhs[i], t) {
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
				if r != nil && usesErr(r, t) {
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
	var out []Site
	for _, d := range af.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok || fd.Body == nil {
			continue
		}
		tainted := taintOf(fd.Body)
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
			if !isSink || safeWrappers[callee] || strings.Contains(strings.ToLower(rc), "logger") || rc == "zap" || rc == "log" {
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
				if usesErr(a, tainted) {
					out = append(out, Site{File: name, Func: fd.Name.Name, Line: fset.Position(c.Pos()).Line})
					break
				}
			}
			return true
		})
	}
	return out, nil
}
