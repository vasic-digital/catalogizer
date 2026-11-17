//go:build !routedump

// Route-table dump support (ADMD1F1-S1 round 5, G1FIXFU3 structural redesign). This file is compiled ONLY in test builds (this is a _test.go file; the
// shipped binary never contains it), so the shipped (default-tag) binary contains no engine-walking code at all: no routeChainFuncNames, no funcIdentity and
// no routeChain/routeHandler types. TestShippedBinaryHasNoRouteDumpCode proves that on a real `go build` of the default tag.
// main_routechains_test.go carries a byte-identical test-build copy (go test never uses -tags routedump); TestRouteChainsLockstep
// refuses any difference between the two.

package main

import (
	"fmt"
	"reflect"
	"runtime"
	"sort"
	"unsafe"

	"github.com/gin-gonic/gin"
)

// routeHandler identifies one handler of a chain by its runtime function: the symbol name AND the source file and line of the
// function's entry. Both come from the function pointer. The file matters because the compiler names an inlined closure after its
// caller ("pkg.caller.(*T).Method.func1"), so the name alone does not say where the closure was written; its position does.
type routeHandler struct {
	Name string `json:"name"`
	File string `json:"file"`
	Line int    `json:"line"`
}

func funcIdentity(fn any) routeHandler {
	f := runtime.FuncForPC(reflect.ValueOf(fn).Pointer())
	if f == nil {
		return routeHandler{Name: "<unknown>"}
	}
	file, line := f.FileLine(f.Entry())
	return routeHandler{Name: f.Name(), File: file, Line: line}
}

// routeChain is one route of a finished engine with the ordered handlers of its chain (engine-level and group-level middleware
// first, the terminal handler last).
type routeChain struct {
	Method string         `json:"method"`
	Path   string         `json:"path"`
	Chain  []routeHandler `json:"chain"`
}

// routeChainFuncNames reads, from the engine's own route trees (gin v1.12: Engine.trees []methodTree{method, root *node},
// node.handlers, node.fullPath), every route with the runtime.FuncForPC identity of each handler. Names come from the function
// pointers, never from source text (ADMD1F1-S1 round 5: ground truth for the admin gate). eng is the *gin.Engine.
// The result is cross-checked against Engine.Routes(): any difference is an error (the gin layout changed).
func routeChainFuncNames(eng any) ([]routeChain, error) {
	e, ok := eng.(*gin.Engine)
	if !ok {
		return nil, fmt.Errorf("routeChainFuncNames: not a *gin.Engine")
	}
	trees := reflect.ValueOf(e).Elem().FieldByName("trees")
	if !trees.IsValid() {
		return nil, fmt.Errorf("gin Engine.trees not found: the gin layout changed")
	}
	var out []routeChain
	var werr error
	var walk func(method string, n reflect.Value)
	walk = func(method string, n reflect.Value) {
		if n.IsNil() || werr != nil {
			return
		}
		nn := n.Elem()
		hv, fp, ch := nn.FieldByName("handlers"), nn.FieldByName("fullPath"), nn.FieldByName("children")
		if !hv.IsValid() || !fp.IsValid() || !ch.IsValid() {
			werr = fmt.Errorf("gin node layout changed")
			return
		}
		hv = reflect.NewAt(hv.Type(), unsafe.Pointer(hv.UnsafeAddr())).Elem()
		fp = reflect.NewAt(fp.Type(), unsafe.Pointer(fp.UnsafeAddr())).Elem()
		ch = reflect.NewAt(ch.Type(), unsafe.Pointer(ch.UnsafeAddr())).Elem()
		if hv.Len() > 0 {
			rc := routeChain{Method: method, Path: fp.String()}
			for i := 0; i < hv.Len(); i++ {
				rc.Chain = append(rc.Chain, funcIdentity(hv.Index(i).Interface()))
			}
			out = append(out, rc)
		}
		for i := 0; i < ch.Len(); i++ {
			walk(method, ch.Index(i))
		}
	}
	for i := 0; i < trees.Len(); i++ {
		mt := trees.Index(i)
		mf, rf := mt.FieldByName("method"), mt.FieldByName("root")
		if !mf.IsValid() || !rf.IsValid() {
			return nil, fmt.Errorf("gin methodTree layout changed")
		}
		m := reflect.NewAt(mf.Type(), unsafe.Pointer(mf.UnsafeAddr())).Elem().String()
		root := reflect.NewAt(rf.Type(), unsafe.Pointer(rf.UnsafeAddr())).Elem()
		walk(m, root)
	}
	if werr != nil {
		return nil, werr
	}
	var a, b []string
	for _, ri := range e.Routes() {
		a = append(a, ri.Method+" "+ri.Path)
	}
	for _, r := range out {
		b = append(b, r.Method+" "+r.Path)
	}
	sort.Strings(a)
	sort.Strings(b)
	if !reflect.DeepEqual(a, b) {
		return nil, fmt.Errorf("route tree walk (%d routes) differs from Engine.Routes() (%d routes)", len(b), len(a))
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		return out[i].Method < out[j].Method
	})
	return out, nil
}
