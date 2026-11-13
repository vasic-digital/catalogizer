package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
)

const labelSecret = "METRICSECRETSEG9"

func gatherLabelValues(t *testing.T) []string {
	t.Helper()
	mfs, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatal(err)
	}
	var vals []string
	for _, mf := range mfs {
		if !strings.HasPrefix(mf.GetName(), "catalogizer_http_") {
			continue
		}
		for _, m := range mf.GetMetric() {
			for _, l := range m.GetLabel() {
				vals = append(vals, l.GetValue())
			}
		}
	}
	return vals
}

// R2-2 (kills "normalizePath returns the raw path"): a request to an UNMATCHED route (FullPath empty, so the fallback runs)
// carrying a share token in its path must not put the token into any metric label; the redacted form must be there
// (control needle: the gatherer sees the labels this middleware writes).
func TestGinMiddleware_UnmatchedShareRouteLabelHasNoSecret(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(GinMiddleware())
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/logs/share/"+labelSecret, nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("status %d", w.Code)
	}
	var sawRedacted bool
	for _, v := range gatherLabelValues(t) {
		if strings.Contains(v, labelSecret) {
			t.Fatalf("metric label leaks the share token: %q", v)
		}
		if v == "/api/v1/logs/share/:redacted" {
			sawRedacted = true
		}
	}
	if !sawRedacted {
		t.Fatal("control needle: the redacted fallback label was not found (instrument blind or fallback changed)")
	}
}

func TestNormalizePath_FallbackRedactsSecretSegments(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	var got string
	router.NoRoute(func(c *gin.Context) { got = normalizePath(c) })
	for in, want := range map[string]string{
		"/api/v1/logs/share/" + labelSecret:       "/api/v1/logs/share/:redacted",
		"/auth/reset/" + labelSecret + "/confirm": "/auth/reset/:redacted/confirm",
		"/plain/path": "/plain/path",
	} {
		router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, in, nil))
		if got != want {
			t.Errorf("normalizePath(%q) = %q, want %q", in, got, want)
		}
	}
}
