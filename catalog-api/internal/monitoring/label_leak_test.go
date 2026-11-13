package monitoring

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
)

// R2-2 (kills "PrometheusMiddleware fallback uses the raw path"): an unmatched request carrying a share token in its path must not
// reach any label of the request metrics; the redacted label proves the instrument sees the labels.
func TestPrometheusMiddleware_UnmatchedShareRouteLabelHasNoSecret(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const secret = "MONSECRETSEG77"
	m := newTestMetrics(t)
	router := gin.New()
	router.Use(PrometheusMiddleware(m))
	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/logs/share/"+secret, nil))

	reg := prometheus.NewRegistry()
	reg.MustRegister(m.RequestDuration, m.RequestTotal, m.RequestSize, m.ResponseSize)
	mfs, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	var saw bool
	for _, mf := range mfs {
		for _, mt := range mf.GetMetric() {
			for _, l := range mt.GetLabel() {
				if strings.Contains(l.GetValue(), secret) {
					t.Fatalf("%s label %s leaks the share token: %q", mf.GetName(), l.GetName(), l.GetValue())
				}
				if l.GetValue() == "/api/v1/logs/share/:redacted" {
					saw = true
				}
			}
		}
	}
	if !saw {
		t.Fatal("control needle: redacted fallback label not found (instrument blind)")
	}
}
