package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"catalogizer/database"
	"catalogizer/models"
	"catalogizer/repository"
	"catalogizer/services"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// N7-FU2: the log-management `:id` routes (and the collection id in the share body) are admin-only, and a foreign id used to answer
// 403 "Access denied" while a missing id answered 404 "Not found": an existence oracle for another user's collections and shares.
// Real gin router, real handler, real service and repository on a migrated SQLite database (unit-class, section 11.4.289: the auth
// service is the only stand-in). A foreign id and a missing id must be byte-identical: status, body and headers.
func TestN7FU2_LogRoutes_ForeignIDIsIndistinguishableFromMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	raw, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	raw.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = raw.Close() })
	db := database.WrapDB(raw, database.DialectSQLite)
	require.NoError(t, db.RunMigrations(context.Background()))
	svc := services.NewLogManagementService(repository.NewLogManagementRepository(db))
	auth := new(MockLogManagementAuthService)
	auth.On("CheckPermission", mock.Anything, models.PermissionSystemAdmin).Return(true, nil)
	h := NewLogManagementHandler(&LogManagementServiceAdapter{Inner: svc}, auth)

	r := gin.New()
	r.Use(func(c *gin.Context) { // the caller is user 1 (an administrator)
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), "user_id", 1))
	})
	g := r.Group("/api/v1/logs")
	g.GET("/collections/:id", WrapHTTPHandler(h.GetLogCollection))
	g.GET("/collections/:id/entries", WrapHTTPHandler(h.GetLogEntries))
	g.POST("/collections/:id/export", WrapHTTPHandler(h.ExportLogs))
	g.GET("/collections/:id/analyze", WrapHTTPHandler(h.AnalyzeLogs))
	g.POST("/share", WrapHTTPHandler(h.CreateLogShare))
	g.DELETE("/share/:id", WrapHTTPHandler(h.RevokeLogShare))

	// user 2 owns a collection and a share; user 1 (the caller) owns neither
	foreignCol, err := svc.CollectLogs(2, &models.LogCollectionRequest{Name: "other", Components: []string{"api"}, LogLevel: "info"})
	require.NoError(t, err)
	foreignShare, err := svc.CreateLogShare(2, &models.LogShareRequest{CollectionID: foreignCol.ID, ShareType: "link", Permissions: []string{"read"}})
	require.NoError(t, err)
	const missingCol, missingShare = 987654, 987655

	do := func(method, path string, body []byte) *httptest.ResponseRecorder {
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, httptest.NewRequest(method, path, bytes.NewReader(body)))
		return rr
	}
	shareBody := func(col int) []byte {
		b, _ := json.Marshal(models.LogShareRequest{CollectionID: col, ShareType: "link", Permissions: []string{"read"}})
		return b
	}
	routes := []struct {
		name            string
		method, pattern string
		foreignID       int
		missingID       int
		body            func(id int) []byte
	}{
		{"get collection", "GET", "/api/v1/logs/collections/%d", foreignCol.ID, missingCol, nil},
		{"entries", "GET", "/api/v1/logs/collections/%d/entries", foreignCol.ID, missingCol, nil},
		{"export", "POST", "/api/v1/logs/collections/%d/export", foreignCol.ID, missingCol, nil},
		{"analyze", "GET", "/api/v1/logs/collections/%d/analyze", foreignCol.ID, missingCol, nil},
		{"create share (collection id in body)", "POST", "/api/v1/logs/share%.0d", foreignCol.ID, missingCol, shareBody},
		{"revoke share", "DELETE", "/api/v1/logs/share/%d", foreignShare.ID, missingShare, nil},
	}
	for _, rt := range routes {
		t.Run(rt.name, func(t *testing.T) {
			req := func(id int) *httptest.ResponseRecorder {
				var body []byte
				if rt.body != nil {
					body = rt.body(id)
				}
				path := fmt.Sprintf(rt.pattern, id)
				if rt.body != nil { // the id travels in the body
					path = "/api/v1/logs/share"
				}
				return do(rt.method, path, body)
			}
			foreign, missing := req(rt.foreignID), req(rt.missingID)
			require.Equal(t, http.StatusNotFound, missing.Code, "control: a missing id is 404: %s", missing.Body.String())
			assert.Equal(t, missing.Code, foreign.Code, "foreign id must answer like a missing one: %s", foreign.Body.String())
			assert.Equal(t, missing.Body.String(), foreign.Body.String(), "bodies must be identical")
			assert.Equal(t, missing.Header(), foreign.Header(), "headers must be identical")
		})
	}

	// the owner still gets their own collection (the fix must not turn everything into 404)
	own := httptest.NewRecorder()
	r2 := gin.New()
	r2.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), "user_id", 2))
	})
	r2.GET("/api/v1/logs/collections/:id", WrapHTTPHandler(h.GetLogCollection))
	r2.ServeHTTP(own, httptest.NewRequest("GET", fmt.Sprintf("/api/v1/logs/collections/%d", foreignCol.ID), nil))
	assert.Equal(t, http.StatusOK, own.Code, own.Body.String())
}
