package handlers

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"catalogizer/models"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- real handlers, recording (non-handler) service collaborators ----------

type recLogSvc struct {
	LogManagementServiceInterface // unimplemented methods would nil-panic: only used ones are overridden
	got                           map[string]string
}

func (s *recLogSvc) GetLogCollection(id, _ int) (*models.LogCollection, error) {
	s.got["GetLogCollection"] = strconv.Itoa(id)
	return &models.LogCollection{ID: id}, nil
}
func (s *recLogSvc) GetLogEntries(id, _ int, _ *models.LogEntryFilters) ([]models.LogEntry, error) {
	s.got["GetLogEntries"] = strconv.Itoa(id)
	return nil, nil
}
func (s *recLogSvc) GetLogShare(token string) (*models.LogShare, error) {
	s.got["GetLogShare"] = token
	return nil, assert.AnError // 404 is fine: the token reached the service
}
func (s *recLogSvc) RevokeLogShare(id, _ int) error {
	s.got["RevokeLogShare"] = strconv.Itoa(id)
	return nil
}
func (s *recLogSvc) ExportLogs(id, _ int, _ string) ([]byte, error) {
	s.got["ExportLogs"] = strconv.Itoa(id)
	return []byte("x"), nil
}
func (s *recLogSvc) AnalyzeLogs(id, _ int) (*models.LogAnalysis, error) {
	s.got["AnalyzeLogs"] = strconv.Itoa(id)
	return &models.LogAnalysis{}, nil
}

type recErrSvc struct {
	ErrorReportingServiceInterface
	got map[string]string
}

func (s *recErrSvc) GetErrorReport(id, _ int) (*models.ErrorReport, error) {
	s.got["GetErrorReport"] = strconv.Itoa(id)
	return &models.ErrorReport{}, nil
}
func (s *recErrSvc) GetCrashReport(id, _ int) (*models.CrashReport, error) {
	s.got["GetCrashReport"] = strconv.Itoa(id)
	return &models.CrashReport{}, nil
}
func (s *recErrSvc) UpdateErrorStatus(id, _ int, _ string) error {
	s.got["UpdateErrorStatus"] = strconv.Itoa(id)
	return nil
}
func (s *recErrSvc) UpdateCrashStatus(id, _ int, _ string) error {
	s.got["UpdateCrashStatus"] = strconv.Itoa(id)
	return nil
}

type recCfgSvc struct {
	ConfigurationServiceInterface
	got map[string]string
}

func (s *recCfgSvc) GetWizardStep(step string) (*models.WizardStep, error) {
	s.got["GetWizardStep"] = step
	return &models.WizardStep{}, nil
}
func (s *recCfgSvc) ValidateWizardStep(step string, _ map[string]interface{}) (*models.ValidationResult, error) {
	s.got["ValidateWizardStep"] = step
	return &models.ValidationResult{}, nil
}
func (s *recCfgSvc) SaveWizardProgress(_ int, step string, _ map[string]interface{}) error {
	s.got["SaveWizardProgress"] = step
	return nil
}

type allowAuth struct{}

func (allowAuth) CheckPermission(int, string) (bool, error)  { return true, nil }
func (allowAuth) ValidateToken(string) (*models.User, error) { return &models.User{}, nil }

// muxVarRoute: every route in main.go that is wrap()'d onto a handler reading mux.Vars.
type muxVarRoute struct {
	method, pattern, service, concrete string // concrete = URL with real values
	value                              string // expected value seen by the service
	body                               string
}

var muxVarRoutes = []muxVarRoute{
	{"GET", "/api/v1/logs/collections/:id", "GetLogCollection", "/api/v1/logs/collections/42", "42", ""},
	{"GET", "/api/v1/logs/collections/:id/entries", "GetLogEntries", "/api/v1/logs/collections/42/entries", "42", ""},
	{"POST", "/api/v1/logs/collections/:id/export", "ExportLogs", "/api/v1/logs/collections/42/export", "42", `{}`},
	{"GET", "/api/v1/logs/collections/:id/analyze", "AnalyzeLogs", "/api/v1/logs/collections/42/analyze", "42", ""},
	{"GET", "/api/v1/logs/share/:token", "GetLogShare", "/api/v1/logs/share/tok-abc", "tok-abc", ""},
	{"DELETE", "/api/v1/logs/share/:id", "RevokeLogShare", "/api/v1/logs/share/7", "7", ""},
	{"GET", "/api/v1/errors/reports/:id", "GetErrorReport", "/api/v1/errors/reports/11", "11", ""},
	{"PUT", "/api/v1/errors/reports/:id/status", "UpdateErrorStatus", "/api/v1/errors/reports/11/status", "11", `{"status":"resolved"}`},
	{"GET", "/api/v1/errors/crashes/:id", "GetCrashReport", "/api/v1/errors/crashes/12", "12", ""},
	{"PUT", "/api/v1/errors/crashes/:id/status", "UpdateCrashStatus", "/api/v1/errors/crashes/12/status", "12", `{"status":"resolved"}`},
	{"GET", "/api/v1/configuration/wizard/step/:step_id", "GetWizardStep", "/api/v1/configuration/wizard/step/welcome", "welcome", ""},
	{"POST", "/api/v1/configuration/wizard/step/:step_id/validate", "ValidateWizardStep", "/api/v1/configuration/wizard/step/welcome/validate", "welcome", `{}`},
	{"POST", "/api/v1/configuration/wizard/step/:step_id/save", "SaveWizardProgress", "/api/v1/configuration/wizard/step/welcome/save", "welcome", `{}`},
}

func TestWrapHTTPHandler_PathParamsReachMuxVars_EveryRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	got := map[string]string{}
	lh := NewLogManagementHandler(&recLogSvc{got: got}, allowAuth{})
	eh := NewErrorReportingHandler(&recErrSvc{got: got}, allowAuth{})
	ch := NewConfigurationHandler(&recCfgSvc{got: got}, allowAuth{})
	byName := map[string]func(http.ResponseWriter, *http.Request){
		"GetLogCollection": lh.GetLogCollection, "GetLogEntries": lh.GetLogEntries,
		"ExportLogs": lh.ExportLogs, "AnalyzeLogs": lh.AnalyzeLogs,
		"GetLogShare": lh.GetLogShare, "RevokeLogShare": lh.RevokeLogShare,
		"GetErrorReport": eh.GetErrorReport, "UpdateErrorStatus": eh.UpdateErrorStatus,
		"GetCrashReport": eh.GetCrashReport, "UpdateCrashStatus": eh.UpdateCrashStatus,
		"GetWizardStep": ch.GetWizardStep, "ValidateWizardStep": ch.ValidateWizardStep,
		"SaveWizardProgress": ch.SaveWizardProgress,
	}
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("user_id", 1); c.Next() })
	for _, rt := range muxVarRoutes {
		h, ok := byName[rt.service]
		require.True(t, ok, rt.service)
		r.Handle(rt.method, rt.pattern, WrapHTTPHandler(h))
	}
	for _, rt := range muxVarRoutes {
		t.Run(rt.method+" "+rt.pattern, func(t *testing.T) {
			delete(got, rt.service)
			req := httptest.NewRequest(rt.method, rt.concrete, strings.NewReader(rt.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			assert.NotEqual(t, http.StatusBadRequest, w.Code, "path param lost: %s", w.Body.String())
			assert.Equal(t, rt.value, got[rt.service], "service must receive the path param")
		})
	}
}

func TestWrapHTTPHandler_NoParamsLeavesMuxVarsUsable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/plain", WrapHTTPHandler(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/plain", nil))
	assert.Equal(t, http.StatusNoContent, w.Code)
	_ = context.Background()
}

// --- static gate -----------------------------------------------------------

var ginParamRe = regexp.MustCompile(`:([A-Za-z0-9_]+)`)

// recvOf maps the recorded service name of a muxVarRoutes case to its handler
// receiver type (handlers are keyed by receiver type + method in the gate).
func recvOf(method string) string {
	switch method {
	case "GetLogCollection", "GetLogEntries", "ExportLogs", "AnalyzeLogs", "GetLogShare", "RevokeLogShare":
		return "LogManagementHandler"
	case "GetErrorReport", "UpdateErrorStatus", "GetCrashReport", "UpdateCrashStatus":
		return "ErrorReportingHandler"
	default:
		return "ConfigurationHandler"
	}
}

// coverageKeys is every "VERB full-pattern|Type.Method" with a behavioural case.
func coverageKeys() map[string]bool {
	m := map[string]bool{}
	for _, rt := range muxVarRoutes {
		m[rt.method+" "+rt.pattern+"|"+recvOf(rt.service)+"."+rt.service] = false
	}
	for _, k := range userRoleGateKeys {
		m[k] = false
	}
	return m
}

var userRoleGateKeys = []string{
	"GET /api/v1/users/:id|UserHandler.GetUser",
	"PUT /api/v1/users/:id|UserHandler.UpdateUser",
	"DELETE /api/v1/users/:id|UserHandler.DeleteUser",
	"POST /api/v1/users/:id/reset-password|UserHandler.ResetPassword",
	"POST /api/v1/users/:id/lock|UserHandler.LockAccount",
	"POST /api/v1/users/:id/unlock|UserHandler.UnlockAccount",
	"GET /api/v1/roles/:id|RoleHandler.GetRole",
	"PUT /api/v1/roles/:id|RoleHandler.UpdateRole",
	"DELETE /api/v1/roles/:id|RoleHandler.DeleteRole",
}

// pathParamReaders returns "RecvType.Method" -> set of path-param names the
// method reads, via mux.Vars(r)[k] (direct or through a variable) or routedID
// (which reads the "id" param).
func pathParamReaders(t *testing.T, files []string) map[string]map[string]bool {
	fset := token.NewFileSet()
	out := map[string]map[string]bool{}
	for _, f := range files {
		af, err := parser.ParseFile(fset, f, nil, 0)
		require.NoError(t, err)
		for _, d := range af.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Recv == nil || fd.Body == nil || len(fd.Recv.List) == 0 {
				continue
			}
			typ := recvTypeName(fd.Recv.List[0].Type)
			keys := muxKeysIn(fd.Body)
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				if ce, ok := n.(*ast.CallExpr); ok {
					if id, ok := ce.Fun.(*ast.Ident); ok && id.Name == "routedID" {
						keys["id"] = true
					}
				}
				return true
			})
			if len(keys) > 0 {
				out[typ+"."+fd.Name.Name] = keys
			}
		}
	}
	return out
}

func recvTypeName(e ast.Expr) string {
	switch x := e.(type) {
	case *ast.StarExpr:
		return recvTypeName(x.X)
	case *ast.Ident:
		return x.Name
	}
	return ""
}

type mainRoute struct {
	verb, pattern, key string
}

// wrappedRoutes extracts every route in main.go registered through the wrap
// adapter (alias-aware, inline-middleware-aware, group-prefix-aware).
func wrappedRoutes(t *testing.T, src string) []mainRoute {
	fset := token.NewFileSet()
	mf, err := parser.ParseFile(fset, src, nil, 0)
	require.NoError(t, err)
	verbs := map[string]bool{"GET": true, "POST": true, "PUT": true, "DELETE": true, "PATCH": true}
	wrapNames := map[string]bool{"wrap": true}
	prefix := map[string]string{}
	typeOf := map[string]string{}
	isWrapExpr := func(e ast.Expr) bool {
		switch x := e.(type) {
		case *ast.Ident:
			return wrapNames[x.Name]
		case *ast.SelectorExpr:
			return x.Sel.Name == "WrapHTTPHandler"
		}
		return false
	}
	var groupPrefix func(e ast.Expr) string
	groupPrefix = func(e ast.Expr) string {
		switch x := e.(type) {
		case *ast.Ident:
			return prefix[x.Name]
		case *ast.CallExpr:
			if sel, ok := x.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Group" && len(x.Args) >= 1 {
				if lit, ok := x.Args[0].(*ast.BasicLit); ok {
					v, _ := strconv.Unquote(lit.Value)
					return groupPrefix(sel.X) + v
				}
			}
		}
		return ""
	}
	var routes []mainRoute
	ast.Inspect(mf, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.AssignStmt:
			if len(x.Lhs) == 1 && len(x.Rhs) == 1 {
				id, ok := x.Lhs[0].(*ast.Ident)
				if !ok {
					return true
				}
				if isWrapExpr(x.Rhs[0]) {
					wrapNames[id.Name] = true
				}
				if ce, ok := x.Rhs[0].(*ast.CallExpr); ok {
					if sel, ok := ce.Fun.(*ast.SelectorExpr); ok {
						if sel.Sel.Name == "Group" {
							prefix[id.Name] = groupPrefix(ce)
						} else if strings.HasPrefix(sel.Sel.Name, "New") {
							typeOf[id.Name] = strings.TrimPrefix(sel.Sel.Name, "New")
						}
					}
				}
			}
		case *ast.CallExpr:
			sel, ok := x.Fun.(*ast.SelectorExpr)
			if !ok || !verbs[sel.Sel.Name] || len(x.Args) < 2 {
				return true
			}
			lit, ok := x.Args[0].(*ast.BasicLit)
			if !ok {
				return true
			}
			w, ok := x.Args[len(x.Args)-1].(*ast.CallExpr)
			if !ok || len(w.Args) != 1 || !isWrapExpr(w.Fun) {
				return true
			}
			hs, ok := w.Args[0].(*ast.SelectorExpr)
			if !ok {
				return true
			}
			recv := ""
			if id, ok := hs.X.(*ast.Ident); ok {
				recv = typeOf[id.Name]
			}
			pat, _ := strconv.Unquote(lit.Value)
			routes = append(routes, mainRoute{sel.Sel.Name, groupPrefix(sel.X) + pat, recv + "." + hs.Sel.Name})
		}
		return true
	})
	return routes
}

func handlerFiles(t *testing.T) []string {
	var out []string
	for _, dir := range []string{".", "internal/handlers"} {
		fs, err := filepath.Glob(filepath.Join(dir, "*.go"))
		require.NoError(t, err)
		for _, f := range fs {
			if !strings.HasSuffix(f, "_test.go") {
				out = append(out, f)
			}
		}
	}
	return out
}

// TestWrappedHandlersReadOnlyRoutePatternMuxVars statically fails if a handler
// registered through the wrap adapter in main.go reads a path param (mux.Vars
// or routedID) that is not in its full route pattern, or if such a route has
// no behavioural case in this package (and vice versa).
func TestWrappedHandlersReadOnlyRoutePatternMuxVars(t *testing.T) {
	readers := pathParamReaders(t, handlerFiles(t))
	require.NotEmpty(t, readers, "control needle: instrument must see path-param readers")
	routes := wrappedRoutes(t, "../main.go")
	require.NotEmpty(t, routes, "control needle: instrument must see wrapped routes")
	covered := coverageKeys()
	seen := 0
	for _, rt := range routes {
		keys := readers[rt.key]
		if len(keys) == 0 {
			continue
		}
		seen++
		params := map[string]bool{}
		for _, m := range ginParamRe.FindAllStringSubmatch(rt.pattern, -1) {
			params[m[1]] = true
		}
		var names []string
		for k := range keys {
			names = append(names, k)
		}
		sort.Strings(names)
		for _, k := range names {
			assert.True(t, params[k], "route %s %q wraps %s which reads path param %q not present in the route pattern", rt.verb, rt.pattern, rt.key, k)
		}
		ck := rt.verb + " " + rt.pattern + "|" + rt.key
		if _, ok := covered[ck]; ok {
			covered[ck] = true
		} else {
			assert.Fail(t, "uncovered route", "%s reads path params but has no behavioural case", ck)
		}
	}
	assert.Equal(t, len(covered), seen, "gate must see exactly the routes the behavioural tables cover")
	for k, v := range covered {
		assert.True(t, v, "behavioural case %s no longer matches any main.go route", k)
	}
}

// The instrument itself: alias, inline middleware, chained Group and an
// unrelated-arg shape must all be seen (synthetic positive controls).
func TestWrappedRoutesExtractor_SyntheticControls(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "m.go")
	src := `package main
func f() {
	h := handlers.NewFooHandler()
	w2 := root.WrapHTTPHandler
	api := r.Group("/v1")
	g := api.Group("/g")
	g.GET("/a/:id", w2(h.A))
	g.POST("/b/:x", mw1, mw2, wrap(h.B))
	api.Group("/c").DELETE("/:k", wrap(h.C))
	g.GET("/plain", h.NotWrapped)
}`
	require.NoError(t, os.WriteFile(f, []byte(src), 0o600))
	got := wrappedRoutes(t, f)
	assert.Equal(t, []mainRoute{
		{"GET", "/v1/g/a/:id", "FooHandler.A"},
		{"POST", "/v1/g/b/:x", "FooHandler.B"},
		{"DELETE", "/v1/c/:k", "FooHandler.C"},
	}, got)
}

func muxKeysIn(body *ast.BlockStmt) map[string]bool {
	isMuxVars := func(e ast.Expr) bool {
		c, ok := e.(*ast.CallExpr)
		if !ok {
			return false
		}
		s, ok := c.Fun.(*ast.SelectorExpr)
		if !ok {
			return false
		}
		x, ok := s.X.(*ast.Ident)
		return ok && x.Name == "mux" && s.Sel.Name == "Vars"
	}
	vars := map[string]bool{}
	ast.Inspect(body, func(n ast.Node) bool {
		if as, ok := n.(*ast.AssignStmt); ok && len(as.Rhs) == 1 && isMuxVars(as.Rhs[0]) {
			for _, l := range as.Lhs {
				if id, ok := l.(*ast.Ident); ok {
					vars[id.Name] = true
				}
			}
		}
		return true
	})
	keys := map[string]bool{}
	ast.Inspect(body, func(n ast.Node) bool {
		ie, ok := n.(*ast.IndexExpr)
		if !ok {
			return true
		}
		match := isMuxVars(ie.X)
		if id, ok := ie.X.(*ast.Ident); ok && vars[id.Name] {
			match = true
		}
		if bl, ok := ie.Index.(*ast.BasicLit); ok && match {
			k, _ := strconv.Unquote(bl.Value)
			keys[k] = true
		}
		return true
	})
	return keys
}
