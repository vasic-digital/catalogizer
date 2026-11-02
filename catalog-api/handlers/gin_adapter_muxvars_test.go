package handlers

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
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
	{"GET", "/api/v1/config/wizard/step/:step_id", "GetWizardStep", "/api/v1/config/wizard/step/welcome", "welcome", ""},
	{"POST", "/api/v1/config/wizard/step/:step_id/validate", "ValidateWizardStep", "/api/v1/config/wizard/step/welcome/validate", "welcome", `{}`},
	{"POST", "/api/v1/config/wizard/step/:step_id/save", "SaveWizardProgress", "/api/v1/config/wizard/step/welcome/save", "welcome", `{}`},
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

// TestWrappedHandlersReadOnlyRoutePatternMuxVars statically fails if a handler
// registered through wrap(...) in main.go reads a mux var that is not a
// parameter of its route pattern (the name-mismatch class), or if a wrap()'d
// route that reads mux vars is missing from the behavioural table above.
func TestWrappedHandlersReadOnlyRoutePatternMuxVars(t *testing.T) {
	fset := token.NewFileSet()
	// method name -> set of mux var keys read in any method of that name
	reads := map[string]map[string]bool{}
	files, err := filepath.Glob("*.go")
	require.NoError(t, err)
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		af, err := parser.ParseFile(fset, f, nil, 0)
		require.NoError(t, err)
		for _, d := range af.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Recv == nil || fd.Body == nil {
				continue
			}
			keys := muxKeysIn(fd.Body)
			if len(keys) > 0 {
				if reads[fd.Name.Name] == nil {
					reads[fd.Name.Name] = map[string]bool{}
				}
				for k := range keys {
					reads[fd.Name.Name][k] = true
				}
			}
		}
	}
	require.NotEmpty(t, reads, "control needle: instrument must see mux.Vars readers")

	mf, err := parser.ParseFile(fset, "../main.go", nil, 0)
	require.NoError(t, err)
	verbs := map[string]bool{"GET": true, "POST": true, "PUT": true, "DELETE": true, "PATCH": true}
	seen := 0
	covered := map[string]bool{}
	for _, rt := range muxVarRoutes {
		covered[rt.method+" "+rt.pattern] = false
	}
	ast.Inspect(mf, func(n ast.Node) bool {
		ce, ok := n.(*ast.CallExpr)
		if !ok || len(ce.Args) != 2 {
			return true
		}
		sel, ok := ce.Fun.(*ast.SelectorExpr)
		if !ok || !verbs[sel.Sel.Name] {
			return true
		}
		lit, ok := ce.Args[0].(*ast.BasicLit)
		if !ok {
			return true
		}
		w, ok := ce.Args[1].(*ast.CallExpr)
		if !ok || len(w.Args) != 1 {
			return true
		}
		if id, ok := w.Fun.(*ast.Ident); !ok || id.Name != "wrap" {
			return true
		}
		hs, ok := w.Args[0].(*ast.SelectorExpr)
		if !ok {
			return true
		}
		rel, _ := strconv.Unquote(lit.Value)
		params := map[string]bool{}
		for _, m := range ginParamRe.FindAllStringSubmatch(rel, -1) {
			params[m[1]] = true
		}
		keys := reads[hs.Sel.Name]
		if len(keys) == 0 {
			return true
		}
		seen++
		var names []string
		for k := range keys {
			names = append(names, k)
		}
		sort.Strings(names)
		for _, k := range names {
			assert.True(t, params[k], "route %s %q wraps %s which reads mux var %q not present in the route pattern", sel.Sel.Name, rel, hs.Sel.Name, k)
		}
		found := false
		for _, rt := range muxVarRoutes {
			if rt.method == sel.Sel.Name && strings.HasSuffix(rt.pattern, rel) && rt.service == hs.Sel.Name {
				found = true
				covered[rt.method+" "+rt.pattern] = true
			}
		}
		assert.True(t, found, "wrap()'d route %s %q (%s) reads mux vars but has no behavioural case in muxVarRoutes", sel.Sel.Name, rel, hs.Sel.Name)
		return true
	})
	assert.GreaterOrEqual(t, seen, len(muxVarRoutes), "gate must see every table route in main.go")
	for k, v := range covered {
		assert.True(t, v, "table route %s no longer matches main.go", k)
	}
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
