package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func accessLogEngine(buf *bytes.Buffer) *gin.Engine {
	gin.SetMode(gin.TestMode)
	e := gin.New()
	e.Use(gin.LoggerWithConfig(gin.LoggerConfig{Formatter: AccessLogFormatter, Output: buf}), AccessLogRouteParams())
	return e
}

// HYG-F3: a sensitive route parameter in a path position no secret-introducing segment precedes is removed from the access log
// (the formatter must hand PathKnown the route parameters, not nil).
func TestAccessLog_PathKnownRedactsSensitiveRouteParam(t *testing.T) {
	const tok = "Qm4Zt8Wv1Nc6Hy3P" // synthetic marker
	var buf bytes.Buffer
	e := accessLogEngine(&buf)
	e.GET("/zz/items/:token/detail", func(c *gin.Context) { c.Status(http.StatusOK) })
	e.GET("/zz/other/:name/plain", func(c *gin.Context) { c.Status(http.StatusOK) })
	e.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/zz/items/"+tok+"/detail", nil))
	out := buf.String()
	if !strings.Contains(out, "/zz/items/") || !strings.Contains(out, "/detail") {
		t.Fatalf("control needle: path missing from access log: %q", out)
	}
	if strings.Contains(out, tok) {
		t.Fatalf("sensitive route parameter reached the access log: %q", out)
	}
	// negative control: a non-sensitive route parameter is NOT removed.
	buf.Reset()
	e.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/zz/other/benignvalue/plain", nil))
	if !strings.Contains(buf.String(), "benignvalue") {
		t.Fatalf("non-sensitive route parameter was removed (over-redaction): %q", buf.String())
	}
}

// HYG-F4: a %2F inside a malformed token decodes into extra path segments; the raw segment is redacted as a whole.
func TestAccessLog_EncodedSlashInsideTokenDoesNotLeakLaterSegments(t *testing.T) {
	for _, target := range []string{
		"/share/log_share_AAA%2FBBBleak1234",
		"/api/v1/logs/share/Tok%2Fmore%2Fleak5678",
		"/api/v1/logs/share/Tok/more",
	} {
		var buf bytes.Buffer
		e := accessLogEngine(&buf)
		e.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, target, nil))
		out := buf.String()
		if !strings.Contains(out, "/share/") || !strings.Contains(out, ":redacted") {
			t.Fatalf("control needle (%s): redaction marker missing: %q", target, out)
		}
		for _, leak := range []string{"BBBleak1234", "leak5678", "AAA", "Tok"} {
			if strings.Contains(out, leak) {
				t.Errorf("%s: %q leaked into access log: %q", target, leak, out)
			}
		}
	}
	// the consistent behaviour for a plain extra segment is unchanged (SecretPath keeps it).
	var buf bytes.Buffer
	e := accessLogEngine(&buf)
	e.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/logs/share/Tok/more", nil))
	if !strings.Contains(buf.String(), "/share/:redacted/more") {
		t.Errorf("plain extra segment behaviour changed: %q", buf.String())
	}
}
