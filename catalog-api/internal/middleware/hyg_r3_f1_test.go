package middleware

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

// HYG-R3-F1: a secret-key segment with a ";" path parameter, trailing dots, a different case or (double) percent-encoding is
// still a secret key, so the NEXT segment is redacted by every path sink on an UNROUTED request (the router has no route for
// these shapes, so no route parameter helps: only the key recognition does).
var r3f1Probes = []string{
	"/share;x=1/LIVEsecret",
	"/share./LIVEsecret",
	"/share../LIVEsecret",
	"/SHARE./LIVEsecret",
	"/Share;a=b;c=d/LIVEsecret",
	"/share%3Bx=1/LIVEsecret",
	"/share%3bx=1/LIVEsecret",
	"/share%2E/LIVEsecret",
	"/%73hare;x=1/LIVEsecret",
	"/share%253Bx=1/LIVEsecret",
	"/api/v1/logs/token./LIVEsecret",
	// round-1 review F-1: dot / param-only segments between the key and the secret
	"/api/v1/logs/share/./LIVEsecret",
	"/share/;/LIVEsecret",
	"/share/%2e/LIVEsecret",
	"/share/..;/LIVEsecret",
	"/share/./LIVEsecret",
	"/share/;x=1/LIVEsecret",
	"/share/../LIVEsecret",
	"/share/.;/LIVEsecret",
	"/share./%2E/LIVEsecret",
	// F-2: escapes deeper than the decode cap are keys in all three sinks
	"/share%2525253Bx/LIVEsecret",
	"/share/%25252525LIVEsecret",
}

const r3f1Leak = "LIVEsecret"

func r3f1Engine(extra ...gin.HandlerFunc) *gin.Engine {
	gin.SetMode(gin.TestMode)
	e := gin.New()
	e.Use(extra...)
	e.NoRoute(func(c *gin.Context) { c.Status(http.StatusNotFound) })
	return e
}

func r3f1Sinks(t *testing.T, target string) map[string]string {
	t.Helper()
	out := map[string]string{}
	var fb bytes.Buffer
	e := r3f1Engine(gin.LoggerWithConfig(gin.LoggerConfig{Formatter: AccessLogFormatter, Output: &fb}), AccessLogRouteParams())
	e.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, target, nil))
	out["gin formatter"] = fb.String()

	core, logs := observer.New(zap.InfoLevel)
	e = r3f1Engine(Logger(zap.New(core)))
	e.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, target, nil))
	if logs.Len() != 1 {
		t.Fatalf("control needle: zap Logger wrote %d entries for %s", logs.Len(), target)
	}
	out["zap Logger"] = logs.All()[0].ContextMap()["path"].(string)

	// ErrorHandler logs only when a handler error exists: register a NoRoute that attaches one.
	var eb bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&eb)
	defer log.SetOutput(prev)
	gin.SetMode(gin.TestMode)
	e = gin.New()
	e.Use(ErrorHandler())
	e.NoRoute(func(c *gin.Context) { _ = c.Error(errBoom{}); c.Status(http.StatusNotFound) })
	e.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, target, nil))
	out["ErrorHandler"] = eb.String()
	return out
}

type errBoom struct{}

func (errBoom) Error() string { return "boom" }

func TestHygR3F1_SecretKeyVariantsRedactedInAllSinks(t *testing.T) {
	for _, target := range r3f1Probes {
		for sink, text := range r3f1Sinks(t, target) {
			if text == "" {
				t.Fatalf("%s %s: control needle: sink wrote nothing", sink, target)
			}
			if strings.Contains(text, r3f1Leak) {
				t.Errorf("%s %s: secret leaked: %q", sink, target, text)
			}
			if !strings.Contains(text, ":redacted") && !strings.Contains(text, "REDACTED") {
				t.Errorf("%s %s: no redaction marker: %q", sink, target, text)
			}
		}
	}
}

// Negative control: a segment that is not a secret key stays unchanged (the fix is not a blanket redaction), also with ";" and ".".
func TestHygR3F1_NonSecretSegmentUnchanged(t *testing.T) {
	for _, target := range []string{"/other;x=1/LIVEsecret", "/other./LIVEsecret", "/sharex;x=1/LIVEsecret", "/xshare./LIVEsecret"} {
		for sink, text := range r3f1Sinks(t, target) {
			if !strings.Contains(text, r3f1Leak) {
				t.Errorf("%s %s: non-secret segment was redacted: %q", sink, target, text)
			}
			if strings.Contains(text, ":redacted") {
				t.Errorf("%s %s: unexpected marker: %q", sink, target, text)
			}
		}
	}
}
