package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

// HANDLERS-RED-1-F3 (a): a route parameter with a sensitive NAME under a segment that does not introduce a secret
// ("/items/:secret") must not reach the "path" field of the request log or the recovery log.
func TestLoggerAndRecovery_PathFieldRemovesSensitiveRouteParam(t *testing.T) {
	const sec = "Rt5Bw8KZq9Xv7Lm2" // synthetic marker
	gin.SetMode(gin.TestMode)
	core, logs := observer.New(zap.InfoLevel)
	lg := zap.New(core)
	e := gin.New()
	e.Use(Logger(lg), Recovery(lg))
	e.GET("/zz/items/:secret/x", func(c *gin.Context) { panic("boom") })
	e.GET("/zz/ok/:secret", func(c *gin.Context) { c.Status(200) })
	for _, p := range []string{"/zz/items/" + sec + "/x", "/zz/ok/" + sec} {
		w := httptest.NewRecorder()
		e.ServeHTTP(w, httptest.NewRequest(http.MethodGet, p, nil))
	}
	n := 0
	for _, ent := range logs.All() {
		for _, f := range ent.Context {
			if f.Key == "path" {
				n++
				if strings.Contains(f.String, sec) {
					t.Errorf("%q path field leaks the route param: %q", ent.Message, f.String)
				}
			}
		}
	}
	if n < 3 {
		t.Fatalf("control needle: expected >=3 path fields, saw %d", n)
	}
}
