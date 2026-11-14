package handlers

// CAT-PXF23-R5B: the remaining client-reaching err.Error() sites (playback, scan, playlist, service_handlers,
// conversion, asset, sync, configuration, error reporting, collection smart rules) must not carry the text of an
// internal error. Real stack where the harness allows it (production migrations + SQLite, the trigger / drop-table
// technique of pxf23_r5_leak_test.go); the scanner and the configuration service are unit-class doubles and are
// labelled as such. TestPXF23R5b_SourceSweep is the class-exhaustive regression guard (go/ast over the handler
// packages), with a positive and a negative control.

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"catalogizer/database"
	"catalogizer/filesystem"
	"catalogizer/internal/smartrules"
	"catalogizer/repository"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// r5bBadBodyLeaks are fragments that only appear when a gin / encoding/json binding error is echoed.
var r5bBadBodyLeaks = []string{"Go struct field", "cannot unmarshal", "Key: '", "Field validation", "json:", "unexpected end of JSON", "invalid character"}

func r5bNoLeak(t *testing.T, label string, w *httptest.ResponseRecorder, extra ...string) {
	t.Helper()
	all := w.Body.String()
	for k, v := range w.Header() {
		all += "\n" + k + ": " + strings.Join(v, ",")
	}
	for _, bad := range append(append([]string{r5Marker, "connection refused", "no such table", "UNIQUE constraint", "sql:", "no rows in result"}, r5bBadBodyLeaks...), extra...) {
		assert.NotContains(t, all, bad, "%s: internal text %q reached the client (status %d, body %q)", label, bad, w.Code, w.Body.String())
	}
}

func r5bDo(r http.Handler, method, url, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, url, bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestPXF23R5b_PlaybackAndScanNeverLeak(t *testing.T) {
	e := newRTEnv(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("user_id", int64(1)); c.Next() })
	ph := NewPlaybackHandler(repository.NewPlaybackSessionRepository(e.db))
	r.POST("/pb/start", ph.StartSession)
	r.POST("/pb/progress", ph.ProgressSession)
	r.POST("/pb/end", ph.EndSession)
	r.GET("/pb/:id/progress", ph.GetProgressForEntity)
	r.GET("/pb/:id/history", ph.ListHistoryForEntity)
	base := t.TempDir()
	t.Setenv(filesystem.AllowedLocalBasesEnv, base)
	sh := NewScanHandler(newMockUniversalScanner(), e.db)
	r.POST("/sr", sh.CreateStorageRoot)
	r.GET("/sr", sh.GetStorageRoots)
	r.POST("/scans", sh.QueueScan)

	// binding errors: a type mismatch makes encoding/json name the Go struct and field
	badBodies := []struct{ name, method, url, body string }{
		{"playback start", "POST", "/pb/start", `{"media_item_id":"x"}`},
		{"playback progress", "POST", "/pb/progress", `{"session_id":"x"}`},
		{"playback end", "POST", "/pb/end", `{"session_id":"x"}`},
		{"storage root create", "POST", "/sr", `{"name":5}`},
		{"queue scan", "POST", "/scans", `{"storage_root_id":"x"}`},
	}
	for _, b := range badBodies {
		w := r5bDo(r, b.method, b.url, b.body)
		assert.Equal(t, 400, w.Code, b.name+": "+w.Body.String())
		r5bNoLeak(t, b.name, w)
	}

	// store failures
	e.r5Break(t, "INSERT", "playback_sessions")
	w := r5bDo(r, "POST", "/pb/start", `{"media_item_id":1,"file_id":1,"position_unit":"seconds","start_position":0}`)
	assert.Equal(t, 500, w.Code, w.Body.String())
	r5bNoLeak(t, "playback start store failure", w)

	_, err := e.db.Exec("DROP TABLE media_progress")
	require.NoError(t, err)
	w = r5bDo(r, "GET", "/pb/7/progress", ``)
	assert.Equal(t, 500, w.Code, w.Body.String())
	r5bNoLeak(t, "playback progress store failure", w)
	_, err = e.db.Exec("DROP TABLE playback_sessions")
	require.NoError(t, err)
	w = r5bDo(r, "GET", "/pb/7/history", ``)
	assert.Equal(t, 500, w.Code, w.Body.String())
	r5bNoLeak(t, "playback history store failure", w)

	e.r5Break(t, "INSERT", "storage_roots")
	w = r5bDo(r, "POST", "/sr", `{"name":"r5b","protocol":"local","path":"`+base+`"}`)
	assert.Equal(t, 500, w.Code, w.Body.String())
	r5bNoLeak(t, "storage root create store failure", w)

	// not-found root: the sql "no rows" text must not be echoed; the intentional message stays
	w = r5bDo(r, "POST", "/scans", `{"storage_root_id":4242}`)
	assert.Equal(t, 404, w.Code, w.Body.String())
	r5bNoLeak(t, "queue scan unknown root", w)
	assert.Contains(t, w.Body.String(), "torage root not found")

	// validation message that is intentional must survive (positive control)
	w = r5bDo(r, "POST", "/sr", `{"name":"r5b2","protocol":"ftp","host":"h","url":"http://x"}`)
	assert.Equal(t, 400, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "not used by this protocol", "operator-validation text must stay client-visible")

	_, err = e.db.Exec("DROP TABLE storage_roots")
	require.NoError(t, err)
	w = r5bDo(r, "GET", "/sr", ``)
	assert.Equal(t, 500, w.Code, w.Body.String())
	r5bNoLeak(t, "storage roots list store failure", w)
}

func TestPXF23R5b_ScannerQueueFailureNeverLeaks(t *testing.T) { // unit-class: the scanner is a double
	e := newRTEnv(t)
	base := t.TempDir()
	t.Setenv(filesystem.AllowedLocalBasesEnv, base)
	sc := newMockUniversalScanner()
	sc.queueScanError = fmt.Errorf("queue: %s", r5Marker)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	sh := NewScanHandler(sc, e.db)
	r.POST("/scans", sh.QueueScan)
	_, err := e.db.Exec(`INSERT INTO storage_roots (name, protocol, path, enabled, max_depth, allow_empty) VALUES ('q','local',?,1,3,0)`, base)
	require.NoError(t, err)
	w := r5bDo(r, "POST", "/scans", `{"storage_root_id":1}`)
	assert.Equal(t, 503, w.Code, w.Body.String())
	r5bNoLeak(t, "queue scan failure", w)
}

func TestPXF23R5b_ConfigurationStdHandlerNeverLeaks(t *testing.T) { // unit-class: the service is a double
	svc := new(MockConfigurationService)
	svc.On("GetWizardStep", "s1").Return(nil, fmt.Errorf("%s", r5Marker))
	svc.On("ValidateWizardStep", "s1", mock.Anything).Return(nil, fmt.Errorf("%s", r5Marker))
	h := NewConfigurationHandler(svc, nil)
	req := httptest.NewRequest("GET", "/x", nil)
	req = mux.SetURLVars(req, map[string]string{"step_id": "s1"})
	w := httptest.NewRecorder()
	h.GetWizardStep(w, req)
	assert.Equal(t, 404, w.Code)
	r5bNoLeak(t, "wizard step", w)

	req = httptest.NewRequest("POST", "/x", strings.NewReader(`{}`))
	req = mux.SetURLVars(req, map[string]string{"step_id": "s1"})
	w = httptest.NewRecorder()
	h.ValidateWizardStep(w, req)
	assert.Equal(t, 500, w.Code)
	r5bNoLeak(t, "wizard validate", w)
}

func TestPXF23R5b_SyncDetailsAreFixedClassText(t *testing.T) {
	s := new(SyncHandlerTestSuite)
	s.SetT(t)
	s.SetupSuite()
	s.SetupTest()
	defer s.TearDownTest()
	_, err := s.db.Exec("DROP TABLE sync_endpoints")
	require.NoError(t, err)
	w := s.doRequest("GET", "/sync/endpoints", nil, true)
	assert.Equal(t, 500, w.Code, w.Body.String())
	r5bNoLeak(t, "sync list store failure", w)
	assert.Contains(t, w.Body.String(), "Failed to get sync endpoints", "the fixed error message stays")

	w = s.doRequest("POST", "/sync/endpoints", map[string]interface{}{"name": 5}, true)
	assert.Equal(t, 400, w.Code, w.Body.String())
	r5bNoLeak(t, "sync create bad body", w)
}

func TestPXF23R5b_StorageRootPolicyMessages(t *testing.T) {
	e := newRTEnv(t)
	base := t.TempDir()
	t.Setenv(filesystem.AllowedLocalBasesEnv, base)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	sh := NewScanHandler(newMockUniversalScanner(), e.db)
	r.POST("/sr", sh.CreateStorageRoot)

	// outbound-policy refusal of a loopback URL: generic text, no resolved address / DNS detail
	w := r5bDo(r, "POST", "/sr", `{"name":"wd","protocol":"webdav","url":"http://169.254.169.254/dav"}`)
	assert.Equal(t, 400, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "webdav url refused by the outbound network policy")
	assert.NotContains(t, w.Body.String(), "169.254")
	// the authored "needs a url" text is intentional
	w = r5bDo(r, "POST", "/sr", `{"name":"wd2","protocol":"webdav"}`)
	assert.Equal(t, 400, w.Code, w.Body.String())

	// a local root that does not resolve: the policy sentence stays, the operating-system tail does not
	assert.Equal(t, "local storage root is outside the allowed base directories: /x: cannot resolve path",
		localRootMessage(fmt.Errorf("local storage root is outside the allowed base directories: /x: cannot resolve path: lstat /x: permission denied")))
	assert.Equal(t, "path must be absolute", localRootMessage(fmt.Errorf("path must be absolute")))
	// outside the base: intentional text stays (positive control)
	w = r5bDo(r, "POST", "/sr", `{"name":"l","protocol":"local","path":"/proc"}`)
	assert.Equal(t, 403, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "outside the allowed base directories")
}

func TestPXF23R5b_ValidationTextHelpers(t *testing.T) {
	// a ValidationError wrapped in a chain that also carries driver text: only the authored text may come out
	ve := &smartrules.ValidationError{Msg: "rule 1: unknown field"}
	wrapped := fmt.Errorf("evaluate: %w (%s)", ve, r5Marker)
	assert.Equal(t, "rule 1: unknown field", validationText(wrapped))
	assert.NotContains(t, validationText(fmt.Errorf("%s", r5Marker)), "connection")
	assert.Equal(t, "start_date parameter required", paramRequiredText(fmt.Errorf("report: start_date parameter required: %s", r5Marker)))
	assert.NotContains(t, paramRequiredText(fmt.Errorf("parameter required but %s", r5Marker)), "connection")
}

// ---- class-exhaustive source sweep (go/ast) -------------------------------------------------------------------

// r5bWrapperSinkRE matches project helper functions that write an error body (h.sendError, writeError, respondX...).
var r5bWrapperSinkRE = regexp.MustCompile(`(?i)^(send|write|respond|fail|abort|reply)[a-z]*(error|err)[a-z]*$`)

// r5bSafeWrappers map an error to fixed texts themselves (each one is read and classified in the R5B sweep).
var r5bSafeWrappers = map[string]bool{"writeJobError": true, "writeLogServiceError": true, "writeFixedLogError": true, "respondPDFError": true,
	"respondInternal": true, "respondBadBody": true, "stdError": true, "syncFail": true}

var r5bSinkNames = map[string]bool{"Error": true, "JSON": true, "String": true, "AbortWithStatusJSON": true, "Write": true, "Encode": true,
	"Fprintf": true, "Fprint": true, "Fprintln": true, "IndentedJSON": true, "SendErrorResponse": true}

// r5bMappers are the functions that turn an error into authored client text (each is unit-tested); an error that only
// flows through one of them does not reach the wire.
var r5bMappers = map[string]bool{"validationText": true, "paramRequiredText": true, "localRootMessage": true, "rootURLMessage": true,
	"authClientMessage": true, "sentinelText": true, "syncDetails": true, "rootSpecMessage": true}

// r5bIsSentinel: pkg.ErrXxx / ErrXxx are package-level sentinel values whose text is authored (errors.New at package scope).
func r5bIsSentinel(e ast.Expr) bool {
	switch v := e.(type) {
	case *ast.Ident:
		return strings.HasPrefix(v.Name, "Err")
	case *ast.SelectorExpr:
		return strings.HasPrefix(v.Sel.Name, "Err")
	}
	return false
}

func r5bUsesErr(n ast.Node, errLike map[string]bool) bool {
	found := false
	ast.Inspect(n, func(x ast.Node) bool {
		switch v := x.(type) {
		case *ast.CallExpr:
			if id, ok := v.Fun.(*ast.Ident); ok && r5bMappers[id.Name] {
				return false
			}
			if s, ok := v.Fun.(*ast.SelectorExpr); ok && s.Sel.Name == "Error" && len(v.Args) == 0 && !r5bIsSentinel(s.X) {
				found = true
			}
		case *ast.Ident:
			l := strings.ToLower(v.Name)
			if l == "err" || strings.HasSuffix(l, "err") || errLike[v.Name] {
				found = true
			}
		}
		return true
	})
	return found
}

func r5bIsErrorType(e ast.Expr) bool {
	id, ok := e.(*ast.Ident)
	return ok && id.Name == "error"
}

// r5bErrSynonymRE: names that, in this code base, only ever hold an error value (PXF23-F2). go/ast has no type
// information, so a value of type error that is not declared as such and has no such name stays invisible.
var r5bErrSynonymRE = regexp.MustCompile(`^(e[0-9]?|fail|failure|cause|fault|exc|exception|boom|oops)$`)

// r5bErrLikeNames returns the identifiers of fn that hold an error value although their name does not say so:
// parameters, named results and `var x error` declarations of type error (exact: read from the declaration), and
// assignment targets with an error-synonym name (`failure := do()`, `_, boom := pair()`). A bare nil comparison or the
// last result of a multi-value call is NOT used: measured on the handler packages, both flag slices, maps and
// authored-text results (jobs, roots, msg) and would need an allow-list entry per site.
func r5bErrLikeNames(fn *ast.FuncDecl) map[string]bool {
	set := map[string]bool{}
	fields := func(fl *ast.FieldList) {
		if fl == nil {
			return
		}
		for _, f := range fl.List {
			if r5bIsErrorType(f.Type) {
				for _, nm := range f.Names {
					set[nm.Name] = true
				}
			}
		}
	}
	fields(fn.Type.Params)
	fields(fn.Type.Results)
	if fn.Body == nil {
		return set
	}
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.ValueSpec:
			if v.Type != nil && r5bIsErrorType(v.Type) {
				for _, nm := range v.Names {
					set[nm.Name] = true
				}
			}
		case *ast.AssignStmt:
			for _, l := range v.Lhs {
				if id, ok := l.(*ast.Ident); ok && r5bErrSynonymRE.MatchString(id.Name) {
					set[id.Name] = true
				}
			}
		}
		return true
	})
	return set
}

func r5bRecv(e ast.Expr) string {
	switch v := e.(type) {
	case *ast.Ident:
		return v.Name
	case *ast.SelectorExpr:
		return r5bRecv(v.X) + "." + v.Sel.Name
	}
	return "?"
}

// r5bSweepSource lists "file:line: recv.Sink" for every response sink whose arguments reference an error value.
func r5bSweepSource(name, src string) ([]string, error) {
	fset := token.NewFileSet()
	af, err := parser.ParseFile(fset, name, src, 0)
	if err != nil {
		return nil, err
	}
	var out []string
	errLike := map[string]bool{}
	var inspect func(n ast.Node) bool
	inspect = func(n ast.Node) bool {
		if fd, ok := n.(*ast.FuncDecl); ok {
			errLike = r5bErrLikeNames(fd)
		}
		c, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		var callee, recv string
		switch f := c.Fun.(type) {
		case *ast.SelectorExpr:
			callee, recv = f.Sel.Name, r5bRecv(f.X)
		case *ast.Ident:
			callee = f.Name
		default:
			return true
		}
		isSink := r5bSinkNames[callee] || r5bWrapperSinkRE.MatchString(callee)
		if !isSink || r5bSafeWrappers[callee] || strings.Contains(strings.ToLower(recv), "logger") || recv == "zap" || recv == "log" {
			return true
		}
		// a bare identifier call (no receiver) only counts when it is a wrapper-style sink
		if recv == "" && !r5bWrapperSinkRE.MatchString(callee) {
			return true
		}
		for i, a := range c.Args {
			if callee == "SendErrorResponse" && i != 2 {
				continue
			}
			if r5bUsesErr(a, errLike) {
				out = append(out, fmt.Sprintf("%s:%d", filepath.Base(name), fset.Position(c.Pos()).Line))
				break
			}
		}
		return true
	}
	ast.Inspect(af, inspect)
	return out, nil
}

// r5bSweepAllowed: sites that were classified and are intentionally client-visible. Key = "file:line-content".
// SCRUBBED = services.ScrubErrorFor removes secrets and addresses (storage probe errors are the feature);
// CONST = the identifier only ends in "Err" but is a constant string; MAPPED = the text comes from a mapper that
// returns authored text only.
var r5bSweepAllowed = map[string]string{
	"scan_handler.go|services.ScrubErrorFor(err, root)": "SCRUBBED: storage probe error, secrets/addresses scrubbed by services.ScrubErrorFor (UNCONFIRMED completeness of the scrubber)",
	"media_browse_handler.go|c.Error(rowsErr)":          "PRIVATE-GIN-ERROR: gin.ErrorTypePrivate; internal/middleware.ErrorHandler answers a fixed 'Internal server error' for it",
	"binding_ingestion_handler.go|hostAllowedErr":       "CONST",
	"smb_discovery.go|hostAllowedErr":                   "CONST",
}

func r5bAllowedLine(file, line string) bool {
	for k := range r5bSweepAllowed {
		p := strings.SplitN(k, "|", 2)
		if p[0] == file && strings.Contains(line, p[1]) {
			return true
		}
	}
	return false
}

func TestPXF23R5b_SourceSweep(t *testing.T) {
	// controls: the instrument must see a known leak and must not flag a clean file
	leakSrc := "package p\nfunc f(c *gin.Context, err error) {\n\tc.JSON(500, gin.H{\"error\": err.Error()})\n}\n"
	hits, err := r5bSweepSource("needle.go", leakSrc)
	require.NoError(t, err)
	require.Equal(t, []string{"needle.go:3"}, hits, "positive control: the sweep must see a leaking sink")
	cleanSrc := "package p\nfunc f(c *gin.Context, err error) {\n\tlog.Printf(\"%v\", err)\n\tc.JSON(500, gin.H{\"error\": \"internal error\"})\n}\n"
	hits, err = r5bSweepSource("clean.go", cleanSrc)
	require.NoError(t, err)
	require.Empty(t, hits, "negative control: a clean handler must not be flagged")
	wrapSrc := "package p\nfunc f(c *gin.Context, err error) {\n\tutils.SendErrorResponse(c, 400, \"bad: \"+err.Error(), nil)\n}\n"
	hits, err = r5bSweepSource("wrap.go", wrapSrc)
	require.NoError(t, err)
	require.Len(t, hits, 1, "positive control: message concatenation into SendErrorResponse")

	mapSrc := "package p\nfunc f(c *gin.Context, err error) {\n\tc.JSON(400, gin.H{\"error\": \"bad: \" + validationText(err)})\n}\n"
	hits, err = r5bSweepSource("mapped.go", mapSrc)
	require.NoError(t, err)
	require.Empty(t, hits, "negative control: an error that only flows through an authored-text mapper is not a leak")

	senSrc := "package p\nfunc f(c *gin.Context) {\n\tc.JSON(409, gin.H{\"error\": services.ErrSmartLimit.Error()})\n}\n"
	hits, err = r5bSweepSource("sentinel.go", senSrc)
	require.NoError(t, err)
	require.Empty(t, hits, "negative control: a package sentinel's own text is authored")

	wrapSinkSrc := "package p\nfunc (h *H) f(w http.ResponseWriter, err error) {\n\th.sendError(w, fmt.Sprintf(\"x: %v\", err), 400)\n}\n"
	hits, err = r5bSweepSource("wrapsink.go", wrapSinkSrc)
	require.NoError(t, err)
	require.Len(t, hits, 1, "positive control: a project helper that writes an error body is a sink")
	okWrapSrc := "package p\nfunc (h *H) f(w http.ResponseWriter, err error) {\n\th.sendError(w, \"fixed\", 400)\n\twriteJobError(c, err, \"x\")\n}\n"
	hits, err = r5bSweepSource("okwrap.go", okWrapSrc)
	require.NoError(t, err)
	require.Empty(t, hits, "negative control: fixed text and a classified mapper wrapper are not leaks")

	var offenders []string
	for _, dir := range []string{".", "../internal/handlers"} {
		files, _ := filepath.Glob(filepath.Join(dir, "*.go"))
		sort.Strings(files)
		for _, f := range files {
			if strings.HasSuffix(f, "_test.go") {
				continue
			}
			b, err := os.ReadFile(f)
			require.NoError(t, err)
			sites, err := r5bSweepSource(f, string(b))
			require.NoError(t, err)
			lines := strings.Split(string(b), "\n")
			for _, s := range sites {
				var ln int
				fmt.Sscanf(s[strings.Index(s, ":")+1:], "%d", &ln)
				if r5bAllowedLine(filepath.Base(f), lines[ln-1]) {
					continue
				}
				offenders = append(offenders, fmt.Sprintf("%s:%d %s", f, ln, strings.TrimSpace(lines[ln-1])))
			}
		}
	}
	assert.Empty(t, offenders, "response sinks that put an error value on the wire: replace with a fixed message + server-side log, or classify in r5bSweepAllowed")
}

var _ = database.DialectSQLite
