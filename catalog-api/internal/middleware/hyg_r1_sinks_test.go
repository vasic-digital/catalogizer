package middleware

import (
	"bytes"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

// HYG-R1-F1/F2: every path sink (access-log formatter, Logger, ErrorHandler) redacts the same credentials whatever the
// percent-encoding of the request: non-canonical escapes (%73hare, %54OKEN) and %2F inside a token.
type sinkCase struct{ target, route string }

var sinkCases = []sinkCase{
	{"/%73hare/%54OKENabcd1234", "/share/:token"},
	{"/zz/items/%51m4Zt8Wv1Nc6/detail", "/zz/items/:token/detail"},
	{"/share/log_share_AAA%2FBBBleak1234", "/share/:token"},
}

var sinkLeaks = []string{"TOKENabcd1234", "Qm4Zt8Wv1Nc6", "AAA", "BBBleak1234"}

func sinkEngine(extra ...gin.HandlerFunc) *gin.Engine {
	gin.SetMode(gin.TestMode)
	e := gin.New()
	e.Use(extra...)
	for _, rt := range []string{"/share/:token", "/zz/items/:token/detail"} {
		e.GET(rt, func(c *gin.Context) {
			_ = c.Error(errors.New("boom"))
			c.Status(http.StatusInternalServerError)
		})
	}
	e.NoRoute(func(c *gin.Context) {
		_ = c.Error(errors.New("boom"))
		c.Status(http.StatusNotFound)
	})
	return e
}

func checkNoLeak(t *testing.T, sink, target, out string) {
	t.Helper()
	if !strings.Contains(out, ":redacted") && !strings.Contains(out, "REDACTED") {
		t.Fatalf("%s %s: control needle: no redaction marker in %q", sink, target, out)
	}
	for _, leak := range sinkLeaks {
		if strings.Contains(out, leak) {
			t.Errorf("%s %s: %q leaked: %q", sink, target, leak, out)
		}
	}
}

func TestPathSinks_FormatterNoLeak(t *testing.T) {
	for _, sc := range sinkCases {
		var buf bytes.Buffer
		e := sinkEngine(gin.LoggerWithConfig(gin.LoggerConfig{Formatter: AccessLogFormatter, Output: &buf}), AccessLogRouteParams())
		e.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, sc.target, nil))
		checkNoLeak(t, "formatter", sc.target, buf.String())
	}
}

func TestPathSinks_LoggerNoLeak(t *testing.T) {
	for _, sc := range sinkCases {
		core, logs := observer.New(zap.InfoLevel)
		e := sinkEngine(Logger(zap.New(core)))
		e.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, sc.target, nil))
		if logs.Len() != 1 {
			t.Fatalf("control needle: want 1 log entry, got %d", logs.Len())
		}
		checkNoLeak(t, "Logger", sc.target, logs.All()[0].ContextMap()["path"].(string))
	}
}

func TestPathSinks_ErrorHandlerNoLeak(t *testing.T) {
	for _, sc := range sinkCases {
		var buf bytes.Buffer
		prev := log.Writer()
		log.SetOutput(&buf)
		e := sinkEngine(ErrorHandler())
		e.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, sc.target, nil))
		log.SetOutput(prev)
		checkNoLeak(t, "ErrorHandler", sc.target, buf.String())
	}
}
