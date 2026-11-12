package middleware

import (
	"bytes"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	root_middleware "catalogizer/middleware"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// main.go registers ErrorHandler first and root_middleware.RequestID() after it; the logged detail must carry the
// real request id (the key root_middleware sets), and a client-controlled id must not forge log lines.
func TestErrorHandler_LogsRealRequestID(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(log.Writer())
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(ErrorHandler())
	r.Use(root_middleware.RequestID())
	r.GET("/t", func(c *gin.Context) { _ = c.Error(errors.New(errLeakMarker)) })

	req := httptest.NewRequest("GET", "/t", nil)
	req.Header.Set("X-Request-ID", "req-abc-123")
	r.ServeHTTP(httptest.NewRecorder(), req)
	assert.Contains(t, buf.String(), "req-abc-123")
	assert.Contains(t, buf.String(), "connection refused", "detail is logged server-side")

	buf.Reset()
	req = httptest.NewRequest("GET", "/t", nil)
	req.Header.Set("X-Request-ID", "x\nFORGED 2026 admin login ok")
	r.ServeHTTP(httptest.NewRecorder(), req)
	for _, l := range strings.Split(strings.TrimRight(buf.String(), "\n"), "\n") {
		assert.False(t, strings.HasPrefix(l, "FORGED"), "forged log line: %q", l)
	}
	_ = http.StatusOK
}
