package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"catalogizer/internal/logging"
	"catalogizer/models"
	"catalogizer/services"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

// pxLogSite describes one error-mapping site of LogManagementHandler.
type pxLogSite struct {
	name   string
	setup  func(s *MockLogManagementService, e error)
	call   func(h *LogManagementHandler, rr *httptest.ResponseRecorder, r *http.Request)
	body   string
	vars   map[string]string
	query  string
	anon   bool // public endpoint: no user_id / permission gate
	status int  // documented fixed status for fixed-status sites; 0 = mapped from the error class
}

func pxMappedSites() []pxLogSite {
	return []pxLogSite{
		{name: "GetLogEntries", setup: func(s *MockLogManagementService, e error) { s.On("GetLogEntries", 1, 1, mock.Anything).Return(nil, e) },
			call: func(h *LogManagementHandler, rr *httptest.ResponseRecorder, r *http.Request) { h.GetLogEntries(rr, r) }, vars: map[string]string{"id": "1"}},
		{name: "CreateLogShare", setup: func(s *MockLogManagementService, e error) { s.On("CreateLogShare", 1, mock.Anything).Return(nil, e) },
			call: func(h *LogManagementHandler, rr *httptest.ResponseRecorder, r *http.Request) { h.CreateLogShare(rr, r) }, body: `{"collection_id":1}`},
		{name: "RevokeLogShare", setup: func(s *MockLogManagementService, e error) { s.On("RevokeLogShare", 7, 1).Return(e) },
			call: func(h *LogManagementHandler, rr *httptest.ResponseRecorder, r *http.Request) { h.RevokeLogShare(rr, r) }, vars: map[string]string{"id": "7"}},
		{name: "ExportLogs", setup: func(s *MockLogManagementService, e error) { s.On("ExportLogs", 1, 1, "json").Return(nil, e) },
			call: func(h *LogManagementHandler, rr *httptest.ResponseRecorder, r *http.Request) { h.ExportLogs(rr, r) }, vars: map[string]string{"id": "1"}, query: "?format=json"},
		{name: "AnalyzeLogs", setup: func(s *MockLogManagementService, e error) { s.On("AnalyzeLogs", 1, 1).Return(nil, e) },
			call: func(h *LogManagementHandler, rr *httptest.ResponseRecorder, r *http.Request) { h.AnalyzeLogs(rr, r) }, vars: map[string]string{"id": "1"}},
	}
}

func pxFixedSites() []pxLogSite {
	okShare := &models.LogShare{CollectionID: 3, UserID: 2}
	return []pxLogSite{
		{name: "CreateLogCollection", setup: func(s *MockLogManagementService, e error) { s.On("CollectLogs", 1, mock.Anything).Return(nil, e) },
			call: func(h *LogManagementHandler, rr *httptest.ResponseRecorder, r *http.Request) {
				h.CreateLogCollection(rr, r)
			}, body: `{}`, status: 500},
		{name: "GetLogCollection", setup: func(s *MockLogManagementService, e error) { s.On("GetLogCollection", 1, 1).Return(nil, e) },
			call: func(h *LogManagementHandler, rr *httptest.ResponseRecorder, r *http.Request) {
				h.GetLogCollection(rr, r)
			}, vars: map[string]string{"id": "1"}, status: 404},
		{name: "ListLogCollections", setup: func(s *MockLogManagementService, e error) {
			s.On("GetLogCollectionsByUser", 1, mock.Anything, mock.Anything).Return(nil, e)
		},
			call: func(h *LogManagementHandler, rr *httptest.ResponseRecorder, r *http.Request) {
				h.ListLogCollections(rr, r)
			}, status: 500},
		{name: "GetLogShare/share", setup: func(s *MockLogManagementService, e error) { s.On("GetLogShare", "tok").Return(nil, e) },
			call: func(h *LogManagementHandler, rr *httptest.ResponseRecorder, r *http.Request) { h.GetLogShare(rr, r) }, vars: map[string]string{"token": "tok"}, anon: true, status: 404},
		{name: "GetLogShare/collection", setup: func(s *MockLogManagementService, e error) {
			s.On("GetLogShare", "tok").Return(okShare, nil)
			s.On("GetLogCollection", 3, 2).Return(nil, e)
		},
			call: func(h *LogManagementHandler, rr *httptest.ResponseRecorder, r *http.Request) { h.GetLogShare(rr, r) }, vars: map[string]string{"token": "tok"}, anon: true, status: 404},
		{name: "StreamLogs", setup: func(s *MockLogManagementService, e error) { s.On("StreamLogs", 1, mock.Anything).Return(nil, e) },
			call: func(h *LogManagementHandler, rr *httptest.ResponseRecorder, r *http.Request) { h.StreamLogs(rr, r) }, status: 500},
		{name: "GetLogStatistics", setup: func(s *MockLogManagementService, e error) { s.On("GetLogStatistics", 1).Return(nil, e) },
			call: func(h *LogManagementHandler, rr *httptest.ResponseRecorder, r *http.Request) {
				h.GetLogStatistics(rr, r)
			}, status: 500},
		{name: "UpdateConfiguration", setup: func(s *MockLogManagementService, e error) { s.On("UpdateConfiguration", mock.Anything).Return(e) },
			call: func(h *LogManagementHandler, rr *httptest.ResponseRecorder, r *http.Request) {
				h.UpdateConfiguration(rr, r)
			}, body: `{}`, status: 500},
		{name: "CleanupOldLogs", setup: func(s *MockLogManagementService, e error) { s.On("CleanupOldLogs").Return(e) },
			call: func(h *LogManagementHandler, rr *httptest.ResponseRecorder, r *http.Request) { h.CleanupOldLogs(rr, r) }, status: 500},
	}
}

// pxRun drives one site with the given service error and returns the recorder and the captured server-side log.
func pxRun(t *testing.T, c pxLogSite, svcErr error) (*httptest.ResponseRecorder, *observer.ObservedLogs) {
	t.Helper()
	core, logs := observer.New(zap.DebugLevel)
	prev := logging.Logger
	logging.Logger = zap.New(core)
	t.Cleanup(func() { logging.Logger = prev })

	svc := new(MockLogManagementService)
	auth := new(MockLogManagementAuthService)
	h := &LogManagementHandler{logManagementService: svc, authService: auth}
	auth.On("CheckPermission", 1, models.PermissionSystemAdmin).Return(true, nil)
	c.setup(svc, svcErr)
	req := httptest.NewRequest("POST", "/x"+c.query, bytes.NewBufferString(c.body))
	if !c.anon {
		req = req.WithContext(context.WithValue(context.Background(), "user_id", 1))
	}
	if c.vars != nil {
		req = mux.SetURLVars(req, c.vars)
	}
	rr := httptest.NewRecorder()
	c.call(h, rr, req)
	return rr, logs
}

// PX-F13: the exact fixed body (http.Error appends one newline) for every error class at every mapped site.
func TestPXF13_MappedSites_ExactFixedBodies(t *testing.T) {
	const marker = "LEAKMARKER-pxf13"
	classes := []struct {
		name   string
		err    error
		status int
		body   string
	}{
		{"404", fmt.Errorf("repo %s: %w", marker, sql.ErrNoRows), 404, "Not found\n"},
		{"403", fmt.Errorf("ownership %s: %w", marker, services.ErrLogAccessDenied), 403, "Access denied\n"},
		{"400", fmt.Errorf("format %s: %w", marker, services.ErrUnsupportedLogFormat), 400, "Unsupported format\n"},
		{"500", fmt.Errorf("write failed %s", marker), 500, "Internal server error\n"},
	}
	for _, s := range pxMappedSites() {
		for _, c := range classes {
			t.Run(s.name+"/"+c.name, func(t *testing.T) {
				rr, _ := pxRun(t, s, c.err)
				assert.Equal(t, c.status, rr.Code)
				assert.Equal(t, c.body, rr.Body.String())
			})
		}
	}
}

// PX-F13 + PX-F14: the fixed-status sites answer their documented status with the exact body, for a sentinel error AND for a
// plain non-sentinel error (a mapping through the error class would turn the 404 sites into 500).
func TestPXF13_PXF14_FixedSites_ExactBodiesForSentinelAndPlainErrors(t *testing.T) {
	errs := map[string]error{
		"sentinel": fmt.Errorf("db: %w", sql.ErrNoRows),
		"plain":    errors.New("plain failure unrelated to any sentinel"),
	}
	for _, s := range pxFixedSites() {
		for en, e := range errs {
			t.Run(s.name+"/"+en, func(t *testing.T) {
				rr, _ := pxRun(t, s, e)
				want := "Internal server error\n"
				if s.status == 404 {
					want = "Not found\n"
				}
				assert.Equal(t, s.status, rr.Code)
				assert.Equal(t, want, rr.Body.String())
			})
		}
	}
}

// PX-F17: every error site logs the service error server-side (redacted) before writing the fixed body; the response
// never carries it. Credentials inside the error text are redacted in the log entry.
func TestPXF17_ServiceErrorLoggedServerSideRedacted(t *testing.T) {
	const marker = "LEAKMARKER-pxf17"
	const secret = "hunter2-s3cr3t"
	mk := func() error {
		return fmt.Errorf("dial postgres://svc:%s@db.internal:5432/x failed password=%s token=%s %s: %w", secret, secret, secret, marker, sql.ErrNoRows)
	}
	sites := append(pxMappedSites(), pxFixedSites()...)
	for _, s := range sites {
		t.Run(s.name, func(t *testing.T) {
			rr, logs := pxRun(t, s, mk())
			body := rr.Body.String()
			assert.NotContains(t, body, marker)
			assert.NotContains(t, body, secret)
			entries := logs.FilterLevelExact(zap.ErrorLevel).All()
			require.Len(t, entries, 1, "exactly one server-side error entry for %s", s.name)
			e := entries[0]
			fm := e.ContextMap()
			errText, _ := fm["error"].(string)
			assert.Contains(t, errText, marker, "the service error is logged server-side")
			assert.NotContains(t, errText, secret, "credentials in the error text are redacted")
			assert.Contains(t, errText, "REDACTED")
			assert.EqualValues(t, rr.Code, fm["status"])
			assert.NotEmpty(t, fm["site"])
		})
	}
}

// PX-F17: the log entry does not carry request query credentials either (path only).
func TestPXF17_LogEntryOmitsQueryString(t *testing.T) {
	s := pxMappedSites()[3] // ExportLogs, query "?format=json"
	s.query = "?format=json&token=abc123secret"
	_, logs := pxRun(t, s, errors.New("boom"))
	entries := logs.FilterLevelExact(zap.ErrorLevel).All()
	require.Len(t, entries, 1)
	for k, v := range entries[0].ContextMap() {
		assert.NotContains(t, fmt.Sprint(v), "abc123secret", "field %s", k)
	}
}
