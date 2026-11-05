package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

// P2-g: GET /smb/discover and GET /smb/test take username and password in the query string.
// The real access-log middleware must never write those values (or other credential-shaped
// parameters) to the log; non-secret parameters stay readable for diagnosis.
func TestLogger_CredentialQueryParametersAreRedacted(t *testing.T) {
	gin.SetMode(gin.TestMode)
	core, logs := observer.New(zap.InfoLevel)
	r := gin.New()
	r.Use(Logger(zap.New(core)))
	r.GET("/api/v1/smb/discover", func(c *gin.Context) { c.Status(http.StatusOK) })

	for _, name := range []string{"password", "passwd", "secret", "client_secret", "refresh_token", "api_key", "apikey", "access_token", "ticket", "token", "pass", "pwd", "authorization", "PASSWORD"} {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/v1/smb/discover?host=nas.lan&"+name+"=hunter2-SECRET&domain=WORK", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			require.Equal(t, 200, w.Code)
			entries := logs.TakeAll()
			require.Len(t, entries, 1, "control: exactly one access-log entry")
			path, _ := entries[0].ContextMap()["path"].(string)
			assert.NotContains(t, path, "hunter2-SECRET", "credential value leaked into the access log: %s", path)
			assert.True(t, strings.Contains(path, "host=nas.lan") && strings.Contains(path, "domain=WORK"), "non-secret parameters must stay readable: %s", path)
		})
	}
}
