package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Source-class wiring check (honest label: this reads main.go's syntax tree, it does not run the
// server): the production subtitle routes must be registered through RegisterSubtitleRoutes (the
// function whose route table and admin gate are exercised behaviourally in the handlers package), the
// subtitle handler must be built with WithAdminChecker(userRepo.IsAdmin) (without it the owner/admin
// decision for DELETE would fall back to a context role), and no second hand-written /subtitles
// route table may exist beside it.
func TestMainWiresSubtitleRoutesAndAdminChecker(t *testing.T) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "main.go", nil, 0)
	require.NoError(t, err)

	var registerCalls, adminCheckerOK, handCrafted int
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		switch sel.Sel.Name {
		case "RegisterSubtitleRoutes":
			if len(call.Args) == 3 {
				if id, ok := call.Args[1].(*ast.Ident); ok && id.Name == "getSubtitleHandler" {
					if g, ok := call.Args[2].(*ast.Ident); ok && g.Name == "adminGate" {
						registerCalls++
					}
				}
			}
		case "WithAdminChecker":
			if inner, ok := sel.X.(*ast.CallExpr); ok {
				if isel, ok := inner.Fun.(*ast.SelectorExpr); ok && isel.Sel.Name == "NewSubtitleHandler" && len(call.Args) == 1 {
					if a, ok := call.Args[0].(*ast.SelectorExpr); ok && a.Sel.Name == "IsAdmin" {
						if x, ok := a.X.(*ast.Ident); ok && x.Name == "userRepo" {
							adminCheckerOK++
						}
					}
				}
			}
		case "Group":
			if len(call.Args) == 1 {
				if lit, ok := call.Args[0].(*ast.BasicLit); ok && lit.Value == `"/subtitles"` {
					handCrafted++
				}
			}
		}
		return true
	})
	assert.Equal(t, 1, registerCalls, "main.go must register subtitle routes exactly once via RegisterSubtitleRoutes(api, getSubtitleHandler, adminGate)")
	assert.Equal(t, 1, adminCheckerOK, "the subtitle handler must be built with WithAdminChecker(userRepo.IsAdmin)")
	assert.Equal(t, 0, handCrafted, "no hand-written /subtitles group may exist beside the shared route table")
}
