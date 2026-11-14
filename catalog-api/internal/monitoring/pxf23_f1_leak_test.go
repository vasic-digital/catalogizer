package monitoring

import (
	"bytes"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// PXF23-F1: a failing check's error text (driver / host / path internals) must not reach the health response; it is logged.
func TestHealthChecker_FailingCheckErrorTextNotInResponse(t *testing.T) {
	const marker = "pq: connection refused host=db-internal-7 user=svc"
	var logbuf bytes.Buffer
	old, oldFlags := log.Writer(), log.Flags()
	log.SetOutput(&logbuf)
	defer func() { log.SetOutput(old); log.SetFlags(oldFlags) }()

	hc := NewHealthChecker("t")
	hc.RegisterCheck("db", func() error { return errors.New(marker) })
	hc.RegisterCheck("ok", func() error { return nil })

	gin.SetMode(gin.TestMode)
	e := gin.New()
	e.GET("/h", hc.GinHealthHandler())
	w := httptest.NewRecorder()
	e.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/h", nil))
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("control: want 503, got %d body=%s", w.Code, w.Body.String())
	}
	var doc map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	full := w.Body.String() + strings.Join(w.Header().Values("X-Error"), "")
	if strings.Contains(full, "db-internal-7") || strings.Contains(full, "connection refused") {
		t.Fatalf("error text leaked in response: %s", full)
	}
	detail := doc["checks"].(map[string]interface{})["db"].(map[string]interface{})
	if detail["status"] != "unhealthy" || detail["error"] == "" || detail["error"] == nil {
		t.Fatalf("unhealthy detail lost: %v", detail)
	}
	if !strings.Contains(logbuf.String(), "db") || !strings.Contains(logbuf.String(), "connection refused") {
		t.Fatalf("error not logged server-side: %q", logbuf.String())
	}
}

// F7: the logged line is itself redacted (credential-shaped text in a check error never reaches the log verbatim).
func TestHealthChecker_LoggedErrorTextIsRedacted(t *testing.T) {
	var logbuf bytes.Buffer
	old, oldFlags := log.Writer(), log.Flags()
	log.SetOutput(&logbuf)
	defer func() { log.SetOutput(old); log.SetFlags(oldFlags) }()
	hc := NewHealthChecker("t")
	hc.RegisterCheck("db", func() error { return errors.New("dial failed password=hunter2secretvalue port=5432") })
	_ = hc.Check()
	got := logbuf.String()
	if !strings.Contains(got, "dial failed") {
		t.Fatalf("control needle: error not logged: %q", got)
	}
	if strings.Contains(got, "hunter2secretvalue") {
		t.Fatalf("credential reached the log: %q", got)
	}
}
