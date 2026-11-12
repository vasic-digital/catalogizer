package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"net/http/httptest"
	"testing"

	"catalogizer/models"
	"catalogizer/services"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
)

// PX-F3: the mapped error status keeps its documented code, but the body never carries the internal error text
// (`sql: no rows in result set`, wrapped repository messages, driver errors).
func TestRevokeLogShare_ErrorBodiesDoNotLeakInternalText(t *testing.T) {
	cases := []struct {
		name       string
		err        error
		wantStatus int
		leak       string
	}{
		{"unknown share", fmt.Errorf("failed to get log share: %w", sql.ErrNoRows), 404, "no rows"},
		{"someone else's", services.ErrLogAccessDenied, 403, "failed"},
		{"internal", fmt.Errorf("update failed: disk I/O error at /var/lib/x.db"), 500, "/var/lib"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc := new(MockLogManagementService)
			auth := new(MockLogManagementAuthService)
			h := &LogManagementHandler{logManagementService: svc, authService: auth}
			auth.On("CheckPermission", 1, models.PermissionSystemAdmin).Return(true, nil)
			svc.On("RevokeLogShare", 999999, 1).Return(c.err)
			req := httptest.NewRequest("DELETE", "/api/v1/logs/share/999999", nil)
			req = mux.SetURLVars(req.WithContext(context.WithValue(context.Background(), "user_id", 1)), map[string]string{"id": "999999"})
			rr := httptest.NewRecorder()
			h.RevokeLogShare(rr, req)
			assert.Equal(t, c.wantStatus, rr.Code)
			body := rr.Body.String()
			assert.NotContains(t, body, c.leak, "body leaks internal error text: %q", body)
			if c.wantStatus != 403 { // the 403 body is the sentinel text itself, which is not internal
				assert.NotContains(t, body, c.err.Error())
			}
			assert.NotEmpty(t, body)
		})
	}
}
