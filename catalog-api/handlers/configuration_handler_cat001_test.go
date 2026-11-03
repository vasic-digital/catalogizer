package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
)

func newCfgHandlerNoService() *ConfigurationHandler {
	return NewConfigurationHandler(&MockConfigurationService{}, &MockAuthService{})
}

// F10: handlers read user_id from the request context. A request that reaches a
// handler without it must be refused with 401, never panic (unchecked .(int)).
func TestCAT001_ConfigurationHandlers_MissingUserIDIs401NotPanic(t *testing.T) {
	h := newCfgHandlerNoService()
	handlers := map[string]http.HandlerFunc{
		"SaveWizardProgress": h.SaveWizardProgress,
		"GetWizardProgress":  h.GetWizardProgress,
		"CompleteWizard":     h.CompleteWizard,
		"TestConfiguration":  h.TestConfiguration,
		"DeleteBackup":       h.DeleteBackup,
		"GetSystemStatus":    h.GetSystemStatus,
	}
	for name, fn := range handlers {
		req := httptest.NewRequest("POST", "/x/1", strings.NewReader("{}"))
		req = mux.SetURLVars(req, map[string]string{"step_id": "s", "backup_id": "1"})
		w := httptest.NewRecorder()
		assert.NotPanics(t, func() { fn(w, req) }, name)
		assert.Equal(t, http.StatusUnauthorized, w.Code, name)
	}
}

// F10: no backup store exists behind this endpoint; saying "deleted" would be a lie.
func TestCAT001_DeleteBackup_IsHonestlyNotImplemented(t *testing.T) {
	auth := &MockAuthService{}
	auth.On("CheckPermission", 7, "system.admin").Return(true, nil)
	h := NewConfigurationHandler(&MockConfigurationService{}, auth)
	req := httptest.NewRequest("DELETE", "/backups/1", nil)
	req = req.WithContext(context.WithValue(req.Context(), "user_id", 7))
	req = mux.SetURLVars(req, map[string]string{"backup_id": "1"})
	w := httptest.NewRecorder()
	h.DeleteBackup(w, req)
	assert.Equal(t, http.StatusNotImplemented, w.Code)
	assert.NotContains(t, w.Body.String(), "deleted successfully")
}

// R4-3: GET /configuration/status used to answer 200 "healthy" for five components it never
// checked. With no health source behind it, it must answer 501 and name nothing as healthy.
func TestCAT001_GetSystemStatus_DoesNotInventHealth(t *testing.T) {
	auth := &MockAuthService{}
	auth.On("CheckPermission", 7, "system.configure").Return(true, nil)
	h := NewConfigurationHandler(&MockConfigurationService{}, auth)
	req := httptest.NewRequest("GET", "/status", nil)
	req = req.WithContext(context.WithValue(req.Context(), "user_id", 7))
	w := httptest.NewRecorder()
	h.GetSystemStatus(w, req)
	assert.Equal(t, http.StatusNotImplemented, w.Code)
	for _, invented := range []string{"healthy", "3.0.0", "24h 30m", "components"} {
		assert.NotContains(t, w.Body.String(), invented)
	}
	assert.Contains(t, w.Body.String(), "/api/v1/health")
}
