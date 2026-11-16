package main

// ADMD1F1-S1 review round 1 (F1-F6): the route scanner is fail-closed. Every gin route-method call whose receiver, full path or gate
// cannot be resolved is reported in routeScan.unresolved (which fails the real-source test); nothing is silently dropped.

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

const fcHeader = "package x\nimport \"github.com/gin-gonic/gin\"\ntype mw struct{}\nfunc (mw) RequireAdmin() gin.HandlerFunc { return nil }\nfunc h(c *gin.Context) {}\n"

func scanSrc(t *testing.T, srcs ...string) *routeScan {
	t.Helper()
	dir := t.TempDir()
	var fs []string
	for i, s := range srcs {
		f := filepath.Join(dir, "f"+string(rune('a'+i))+"_routes.go")
		require.NoError(t, os.WriteFile(f, []byte(s), 0o600))
		fs = append(fs, f)
	}
	return scanRouteSources(t, fs)
}

func routeKeys(s *routeScan) []string {
	var out []string
	for _, r := range s.routes {
		g := "open"
		if r.gated {
			g = "gated"
		}
		out = append(out, r.method+" "+r.path+" "+g)
	}
	return out
}

func TestADMD1F1S1R1_FailClosedProbes(t *testing.T) {
	t.Run("A helper returning a gated group is reported", func(t *testing.T) {
		s := scanSrc(t, fcHeader+"func mk(r *gin.Engine, m mw) *gin.RouterGroup { z := r.Group(\"/zz\"); z.Use(m.RequireAdmin()); return z }\nfunc Run(m mw) { r := gin.New(); z := mk(r, m); z.GET(\"/op\", h) }\n")
		require.NotEmpty(t, s.unresolved)
	})
	t.Run("B group in a struct field is reported", func(t *testing.T) {
		s := scanSrc(t, fcHeader+"type S struct{ g *gin.RouterGroup }\nfunc (s *S) Add() { s.g.GET(\"/op\", h) }\n")
		require.NotEmpty(t, s.unresolved)
	})
	t.Run("C group param of a func literal is reported", func(t *testing.T) {
		s := scanSrc(t, fcHeader+"func Run() { r := gin.New(); f := func(g *gin.RouterGroup) { g.GET(\"/op\", h) }; f(r.Group(\"/a\")) }\n")
		require.NotEmpty(t, s.unresolved)
	})
	t.Run("D Handle and Any are routes", func(t *testing.T) {
		s := scanSrc(t, fcHeader+"func Run(m mw) { r := gin.New(); r.Handle(\"DELETE\", \"/op\", m.RequireAdmin(), h); r.Any(\"/x\", m.RequireAdmin(), h) }\n")
		require.Empty(t, s.unresolved)
		require.ElementsMatch(t, []string{"DELETE /op gated", "ANY /x gated"}, routeKeys(s))
	})
	t.Run("F path param resolved through the call site", func(t *testing.T) {
		s := scanSrc(t, fcHeader+"func reg(g gin.IRoutes, path string, m mw) { g.POST(path, m.RequireAdmin(), h) }\nfunc Run(m mw) { r := gin.New(); reg(r.Group(\"/a\"), \"/op\", m) }\n")
		require.Empty(t, s.unresolved)
		require.Equal(t, []string{"POST /a/op gated"}, routeKeys(s))
	})
	t.Run("F2 path param that does not resolve is reported", func(t *testing.T) {
		s := scanSrc(t, fcHeader+"func reg(g gin.IRoutes, path string) { g.POST(path, h) }\nfunc Run(p string) { r := gin.New(); reg(r.Group(\"/a\"), p) }\n")
		require.NotEmpty(t, s.unresolved)
	})
	t.Run("L var-declared group", func(t *testing.T) {
		s := scanSrc(t, fcHeader+"func Run(m mw) { r := gin.New(); var g = r.Group(\"/zz\"); g.Use(m.RequireAdmin()); g.GET(\"/op\", h) }\n")
		require.Empty(t, s.unresolved)
		require.Equal(t, []string{"GET /zz/op gated"}, routeKeys(s))
	})
	t.Run("G two same-name methods are both walked", func(t *testing.T) {
		s := scanSrc(t, fcHeader+"type A struct{}\ntype B struct{}\nfunc (A) Mount(g *gin.RouterGroup) { g.GET(\"/a\", h) }\nfunc (B) Mount(g *gin.RouterGroup, ) { g.POST(\"/b\", h) }\nfunc Run(a A, b B) { r := gin.New(); a.Mount(r.Group(\"/x\")); b.Mount(r.Group(\"/x\")) }\n")
		require.Empty(t, s.unresolved)
		require.ElementsMatch(t, []string{"GET /x/a open", "POST /x/b open"}, routeKeys(s))
	})
	t.Run("E gate through a struct field is reported", func(t *testing.T) {
		s := scanSrc(t, fcHeader+"type D struct{ AdminGate gin.HandlerFunc }\nfunc Run(d D) { r := gin.New(); r.GET(\"/op\", d.AdminGate, h) }\n")
		require.NotEmpty(t, s.unresolved)
	})
	t.Run("M9 unknown prefix emits no route but is reported", func(t *testing.T) {
		s := scanSrc(t, fcHeader+"func Orphan(g *gin.RouterGroup) { g.GET(\"/op\", h) }\n")
		require.Empty(t, s.routes)
		require.NotEmpty(t, s.unresolved)
	})
	t.Run("F6 M5 Group(path, gate) args gate the group", func(t *testing.T) {
		s := scanSrc(t, fcHeader+"func Run(m mw) { r := gin.New(); g := r.Group(\"/a\", m.RequireAdmin()); g.GET(\"/op\", h) }\n")
		require.Equal(t, []string{"GET /a/op gated"}, routeKeys(s))
	})
	t.Run("F6 M8 child group inherits the parent gate", func(t *testing.T) {
		s := scanSrc(t, fcHeader+"func Run(m mw) { r := gin.New(); p := r.Group(\"/p\"); p.Use(m.RequireAdmin()); c := p.Group(\"/c\"); c.GET(\"/op\", h) }\n")
		require.Equal(t, []string{"GET /p/c/op gated"}, routeKeys(s))
	})
	t.Run("F4 path join follows gin", func(t *testing.T) {
		require.Equal(t, "/api/op", joinRoute("/api/", "/op"))
		require.Equal(t, "/x/", joinRoute("/", "/x/"))
		require.Equal(t, "/x", joinRoute("/", "/x"))
		require.Equal(t, "/api/v1", joinRoute("/api/v1", ""))
		s := scanSrc(t, fcHeader+"func Run(m mw) { r := gin.New(); g := r.Group(\"/api/\"); g.GET(\"/op\", m.RequireAdmin(), h); g.GET(\"/x/\", h); g.GET(\"/x\", h) }\n")
		require.ElementsMatch(t, []string{"GET /api/op gated", "GET /api/x/ open", "GET /api/x open"}, routeKeys(s))
	})
	t.Run("redefinition of a group name with a different value is reported", func(t *testing.T) {
		s := scanSrc(t, fcHeader+"func Run() { r := gin.New(); g := r.Group(\"/a\"); g.GET(\"/1\", h); { g := r.Group(\"/b\"); g.GET(\"/2\", h) }; g.GET(\"/3\", h) }\n")
		require.NotEmpty(t, s.unresolved)
	})
}

func TestADMD1F1S1R1_RealChangePasswordRouteIsResolved(t *testing.T) {
	s := scanRouteSources(t, realRouteFiles(t))
	var found bool
	for _, r := range s.routes {
		if r.method == "POST" && r.path == "/api/v1/auth/change-password" {
			found = true
			require.False(t, r.gated)
		}
	}
	require.True(t, found, "RegisterChangePassword(rg, path, ...) must resolve through its call sites")
}
