package monitoring

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestMetrics creates a Metrics struct registered against a custom
// Prometheus registry so that tests never collide with the global
// default registry (or with each other).
func newTestMetrics(t *testing.T) *Metrics {
	t.Helper()

	reg := prometheus.NewRegistry()

	m := &Metrics{
		RequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "test_http_request_duration_seconds",
				Help:    "HTTP request duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "path", "status"},
		),
		RequestTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "test_http_requests_total",
				Help: "Total HTTP requests",
			},
			[]string{"method", "path", "status"},
		),
		RequestSize: prometheus.NewSummaryVec(
			prometheus.SummaryOpts{
				Name: "test_http_request_size_bytes",
				Help: "HTTP request size in bytes",
			},
			[]string{"method", "path"},
		),
		ResponseSize: prometheus.NewSummaryVec(
			prometheus.SummaryOpts{
				Name: "test_http_response_size_bytes",
				Help: "HTTP response size in bytes",
			},
			[]string{"method", "path"},
		),
		ActiveConnections: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "test_active_connections",
				Help: "Number of active connections",
			},
		),
		DatabaseQueries: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "test_database_queries_total",
				Help: "Total database queries",
			},
			[]string{"operation", "table"},
		),
		CacheHits: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "test_cache_hits_total",
				Help: "Total cache hits",
			},
			[]string{"cache_type"},
		),
		CacheMisses: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "test_cache_misses_total",
				Help: "Total cache misses",
			},
			[]string{"cache_type"},
		),
		MemoryUsage: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "test_memory_usage_bytes",
				Help: "Current memory usage in bytes",
			},
		),
		Goroutines: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "test_goroutines_count",
				Help: "Number of goroutines",
			},
		),
		GCCount: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "test_gc_count_total",
				Help: "Total garbage collection cycles",
			},
		),
	}

	reg.MustRegister(
		m.RequestDuration,
		m.RequestTotal,
		m.RequestSize,
		m.ResponseSize,
		m.ActiveConnections,
		m.DatabaseQueries,
		m.CacheHits,
		m.CacheMisses,
		m.MemoryUsage,
		m.Goroutines,
		m.GCCount,
	)

	return m
}

func gaugeValue(g prometheus.Gauge) float64 {
	m := &dto.Metric{}
	g.Write(m)
	return m.GetGauge().GetValue()
}

func counterValue(c prometheus.Counter) float64 {
	m := &dto.Metric{}
	c.Write(m)
	return m.GetCounter().GetValue()
}

func counterVecValue(cv *prometheus.CounterVec, labels ...string) float64 {
	return counterValue(cv.WithLabelValues(labels...))
}

// ---------------------------------------------------------------------------
// NewMetrics
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// PrometheusMiddleware
// ---------------------------------------------------------------------------

func TestPrometheusMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	m := newTestMetrics(t)

	router := gin.New()
	router.Use(PrometheusMiddleware(m))
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	beforeTotal := counterVecValue(m.RequestTotal, "GET", "/test", "200")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	afterTotal := counterVecValue(m.RequestTotal, "GET", "/test", "200")
	assert.Equal(t, beforeTotal+1, afterTotal, "request counter should increment by 1")

	// ActiveConnections should be back to 0 after the request finishes.
	assert.Equal(t, float64(0), gaugeValue(m.ActiveConnections),
		"active connections should return to 0 after the request completes")
}

// ---------------------------------------------------------------------------
// HealthChecker — all healthy
// ---------------------------------------------------------------------------

func TestHealthChecker_AllHealthy(t *testing.T) {
	hc := NewHealthChecker("1.0.0")
	hc.RegisterCheck("db", func() error { return nil })
	hc.RegisterCheck("cache", func() error { return nil })

	result := hc.Check()

	assert.Equal(t, "healthy", result.Status)
	assert.Equal(t, "1.0.0", result.Version)
	require.Len(t, result.Checks, 2)

	for name, detail := range result.Checks {
		m, ok := detail.(map[string]interface{})
		require.True(t, ok, "check %q should be a map", name)
		assert.Equal(t, "healthy", m["status"])
	}
}

// ---------------------------------------------------------------------------
// HealthChecker — one unhealthy
// ---------------------------------------------------------------------------

func TestHealthChecker_OneUnhealthy(t *testing.T) {
	hc := NewHealthChecker("2.0.0")
	hc.RegisterCheck("db", func() error { return nil })
	hc.RegisterCheck("cache", func() error { return errors.New("connection refused") })

	result := hc.Check()

	assert.Equal(t, "unhealthy", result.Status)
	require.Len(t, result.Checks, 2)

	cacheDetail, ok := result.Checks["cache"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "unhealthy", cacheDetail["status"])
	assert.Equal(t, "check failed", cacheDetail["error"], "PXF23-F1: error text is logged, not returned")
}

// ---------------------------------------------------------------------------
// HealthChecker — RegisterMultiple
// ---------------------------------------------------------------------------

func TestHealthChecker_RegisterMultiple(t *testing.T) {
	hc := NewHealthChecker("1.0.0")

	called := map[string]bool{}
	for _, name := range []string{"a", "b", "c", "d"} {
		n := name
		hc.RegisterCheck(n, func() error {
			called[n] = true
			return nil
		})
	}

	result := hc.Check()

	assert.Equal(t, "healthy", result.Status)
	require.Len(t, result.Checks, 4)
	for _, name := range []string{"a", "b", "c", "d"} {
		assert.True(t, called[name], "check %q should have been invoked", name)
	}
}

// ---------------------------------------------------------------------------
// GinHealthHandler — healthy
// ---------------------------------------------------------------------------

func TestGinHealthHandler_Healthy(t *testing.T) {
	gin.SetMode(gin.TestMode)

	hc := NewHealthChecker("1.0.0")
	hc.RegisterCheck("db", func() error { return nil })

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/health", nil)

	handler := hc.GinHealthHandler()
	handler(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var body HealthCheck
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Equal(t, "healthy", body.Status)
}

// ---------------------------------------------------------------------------
// GinHealthHandler — unhealthy
// ---------------------------------------------------------------------------

func TestGinHealthHandler_Unhealthy(t *testing.T) {
	gin.SetMode(gin.TestMode)

	hc := NewHealthChecker("1.0.0")
	hc.RegisterCheck("cache", func() error { return errors.New("down") })

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/health", nil)

	handler := hc.GinHealthHandler()
	handler(c)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	var body HealthCheck
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Equal(t, "unhealthy", body.Status)
}

// ---------------------------------------------------------------------------
// updateSystemMetrics
// ---------------------------------------------------------------------------

func TestUpdateSystemMetrics(t *testing.T) {
	m := newTestMetrics(t)

	// Before calling updateSystemMetrics the gauges should be zero.
	assert.Equal(t, float64(0), gaugeValue(m.MemoryUsage))
	assert.Equal(t, float64(0), gaugeValue(m.Goroutines))

	m.updateSystemMetrics()

	// After the call, MemoryUsage should be > 0 (a running process
	// always has some allocated memory).
	assert.Greater(t, gaugeValue(m.MemoryUsage), float64(0),
		"memory usage should be greater than 0 after update")

	// Goroutines count should be at least 1 (the test goroutine itself).
	assert.GreaterOrEqual(t, gaugeValue(m.Goroutines), float64(1),
		"goroutine count should be at least 1")

	// GCCount is a counter — after one call it should have been
	// incremented by the total number of GC cycles so far.
	assert.GreaterOrEqual(t, counterValue(m.GCCount), float64(0),
		"GC count should be non-negative")
}

// ---------------------------------------------------------------------------
// DefaultMonitoringConfig
// ---------------------------------------------------------------------------

func TestDefaultMonitoringConfig(t *testing.T) {
	cfg := DefaultMonitoringConfig()

	assert.True(t, cfg.EnablePrometheus)
	assert.Equal(t, 9090, cfg.PrometheusPort)
	assert.Equal(t, "catalog-api", cfg.ServiceName)
	assert.Equal(t, "1.0.0", cfg.ServiceVersion)
	assert.True(t, cfg.EnableHealthCheck)
	assert.Equal(t, "/health", cfg.HealthCheckPath)
}

// ---------------------------------------------------------------------------
// Goroutines gauge matches runtime.NumGoroutine (sanity check)
// ---------------------------------------------------------------------------

func TestUpdateSystemMetrics_GoroutinesMatchRuntime(t *testing.T) {
	m := newTestMetrics(t)
	m.updateSystemMetrics()

	// The exact number may vary between the call and the assertion
	// so we just check that it is within a reasonable range.
	numGoroutines := runtime.NumGoroutine()
	recorded := gaugeValue(m.Goroutines)

	// Allow +/- 20 goroutines of drift (runtime may schedule or
	// finish goroutines between the two calls).
	assert.InDelta(t, float64(numGoroutines), recorded, 20,
		"recorded goroutine count should be close to runtime value")
}
