package middleware

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// PKGB-FU2: the access log removes the request's own credentials from c.Error text like every other error-text sink.
func TestAccessLog_ErrorTextRemovesKnownSecrets(t *testing.T) {
	const tok = "Zq9Xv7Lm2Rt5Bw8K" // synthetic marker
	var buf bytes.Buffer
	gin.SetMode(gin.TestMode)
	e := gin.New()
	e.Use(gin.LoggerWithConfig(gin.LoggerConfig{Formatter: AccessLogFormatter, Output: &buf}))
	e.GET("/zz/share/:token", func(c *gin.Context) {
		_ = c.Error(fmt.Errorf("lookup share %s", c.Param("token")))
		c.Status(http.StatusInternalServerError)
	})
	e.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/zz/share/"+tok, nil))
	out := buf.String()
	if !strings.Contains(out, "lookup share") {
		t.Fatalf("control needle: error text missing from access log: %q", out)
	}
	if strings.Contains(out, tok) {
		t.Fatalf("share token reached the access log: %q", out)
	}
}
