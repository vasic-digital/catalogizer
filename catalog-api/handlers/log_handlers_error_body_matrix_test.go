package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"catalogizer/models"
	"catalogizer/services"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// PX-F8: the fixed-body contract holds for every handler that maps a service error, not only RevokeLogShare.
// Each error wraps a marker string; neither the marker nor the full error text may reach the response body.
func TestLogHandlers_ErrorBodiesNeverCarryServiceErrorText(t *testing.T) {
	const marker = "LEAKMARKER-zq81"
	errs := []struct {
		name   string
		err    error
		status int
	}{
		{"wrapped 404", fmt.Errorf("repo %s: %w", marker, sql.ErrNoRows), 404},
		{"wrapped 403", fmt.Errorf("ownership check %s: %w", marker, services.ErrLogAccessDenied), 403},
		{"wrapped 400 with marker in format", fmt.Errorf("format %q: %w", marker, services.ErrUnsupportedLogFormat), 400},
		{"500", fmt.Errorf("write failed %s at /var/lib/x.db", marker), 500},
	}
	handlers := []struct {
		name  string
		setup func(svc *MockLogManagementService, err error)
		call  func(h *LogManagementHandler, rr *httptest.ResponseRecorder, r *http.Request)
		body  string
		vars  map[string]string
		query string
	}{
		{"GetLogEntries", func(s *MockLogManagementService, e error) { s.On("GetLogEntries", 1, 1, mock.Anything).Return(nil, e) },
			func(h *LogManagementHandler, rr *httptest.ResponseRecorder, r *http.Request) { h.GetLogEntries(rr, r) }, "", map[string]string{"id": "1"}, ""},
		{"CreateLogShare", func(s *MockLogManagementService, e error) { s.On("CreateLogShare", 1, mock.Anything).Return(nil, e) },
			func(h *LogManagementHandler, rr *httptest.ResponseRecorder, r *http.Request) { h.CreateLogShare(rr, r) }, `{"collection_id":1}`, nil, ""},
		{"RevokeLogShare", func(s *MockLogManagementService, e error) { s.On("RevokeLogShare", 7, 1).Return(e) },
			func(h *LogManagementHandler, rr *httptest.ResponseRecorder, r *http.Request) { h.RevokeLogShare(rr, r) }, "", map[string]string{"id": "7"}, ""},
		{"ExportLogs", func(s *MockLogManagementService, e error) { s.On("ExportLogs", 1, 1, "json").Return(nil, e) },
			func(h *LogManagementHandler, rr *httptest.ResponseRecorder, r *http.Request) { h.ExportLogs(rr, r) }, "", map[string]string{"id": "1"}, "?format=json"},
		{"AnalyzeLogs", func(s *MockLogManagementService, e error) { s.On("AnalyzeLogs", 1, 1).Return(nil, e) },
			func(h *LogManagementHandler, rr *httptest.ResponseRecorder, r *http.Request) { h.AnalyzeLogs(rr, r) }, "", map[string]string{"id": "1"}, ""},
	}
	for _, hd := range handlers {
		for _, c := range errs {
			t.Run(hd.name+"/"+c.name, func(t *testing.T) {
				svc := new(MockLogManagementService)
				auth := new(MockLogManagementAuthService)
				h := &LogManagementHandler{logManagementService: svc, authService: auth}
				auth.On("CheckPermission", 1, models.PermissionSystemAdmin).Return(true, nil)
				hd.setup(svc, c.err)
				req := httptest.NewRequest("POST", "/x"+hd.query, bytes.NewBufferString(hd.body))
				req = req.WithContext(context.WithValue(context.Background(), "user_id", 1))
				if hd.vars != nil {
					req = mux.SetURLVars(req, hd.vars)
				}
				rr := httptest.NewRecorder()
				hd.call(h, rr, req)
				assert.Equal(t, c.status, rr.Code)
				body := rr.Body.String()
				assert.NotEmpty(t, body)
				assert.NotContains(t, body, marker, "body leaks marker: %q", body)
				assert.NotContains(t, body, c.err.Error())
				assert.NotContains(t, body, "/var/lib")
				assert.NotContains(t, body, "no rows")
			})
		}
	}
}
