package monitoring

import (
	"catalogizer/internal/logredact"
	"context"
	"fmt"
	"net"
	"net/http"
	"runtime"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

// MonitoringConfig holds configuration for monitoring
type MonitoringConfig struct {
	// Prometheus
	EnablePrometheus bool
	PrometheusPort   int

	// Service Info
	ServiceName    string
	ServiceVersion string

	// Health Checks
	EnableHealthCheck bool
	HealthCheckPath   string
}

// DefaultMonitoringConfig returns default configuration
func DefaultMonitoringConfig() MonitoringConfig {
	return MonitoringConfig{
		EnablePrometheus:  true,
		PrometheusPort:    9090,
		ServiceName:       "catalog-api",
		ServiceVersion:    "1.0.0",
		EnableHealthCheck: true,
		HealthCheckPath:   "/health",
	}
}

// Metrics holds Prometheus metrics
type Metrics struct {
	// HTTP metrics
	RequestDuration *prometheus.HistogramVec
	RequestTotal    *prometheus.CounterVec
	RequestSize     *prometheus.SummaryVec
	ResponseSize    *prometheus.SummaryVec

	// Application metrics
	ActiveConnections prometheus.Gauge
	DatabaseQueries   *prometheus.CounterVec
	CacheHits         *prometheus.CounterVec
	CacheMisses       *prometheus.CounterVec

	// System metrics
	MemoryUsage prometheus.Gauge
	Goroutines  prometheus.Gauge
	GCCount     prometheus.Counter
}

// NewMetrics creates new metrics
func NewMetrics() *Metrics {
	m := &Metrics{
		RequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_request_duration_seconds",
				Help:    "HTTP request duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "path", "status"},
		),
		RequestTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_requests_total",
				Help: "Total HTTP requests",
			},
			[]string{"method", "path", "status"},
		),
		RequestSize: prometheus.NewSummaryVec(
			prometheus.SummaryOpts{
				Name: "http_request_size_bytes",
				Help: "HTTP request size in bytes",
			},
			[]string{"method", "path"},
		),
		ResponseSize: prometheus.NewSummaryVec(
			prometheus.SummaryOpts{
				Name: "http_response_size_bytes",
				Help: "HTTP response size in bytes",
			},
			[]string{"method", "path"},
		),
		ActiveConnections: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "active_connections",
				Help: "Number of active connections",
			},
		),
		DatabaseQueries: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "database_queries_total",
				Help: "Total database queries",
			},
			[]string{"operation", "table"},
		),
		CacheHits: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "cache_hits_total",
				Help: "Total cache hits",
			},
			[]string{"cache_type"},
		),
		CacheMisses: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "cache_misses_total",
				Help: "Total cache misses",
			},
			[]string{"cache_type"},
		),
		MemoryUsage: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "memory_usage_bytes",
				Help: "Current memory usage in bytes",
			},
		),
		Goroutines: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "goroutines_count",
				Help: "Number of goroutines",
			},
		),
		GCCount: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "gc_count_total",
				Help: "Total garbage collection cycles",
			},
		),
	}

	// Register metrics
	prometheus.MustRegister(
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

// Monitor starts system monitoring goroutine
func (m *Metrics) Monitor(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.updateSystemMetrics()
		case <-ctx.Done():
			return
		}
	}
}

// updateSystemMetrics updates system-level metrics
func (m *Metrics) updateSystemMetrics() {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	m.MemoryUsage.Set(float64(memStats.Alloc))
	m.Goroutines.Set(float64(runtime.NumGoroutine()))
	m.GCCount.Add(float64(memStats.NumGC))
}

// PrometheusMiddleware returns Gin middleware for Prometheus metrics
func PrometheusMiddleware(metrics *Metrics) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.FullPath()
		if path == "" {
			path = logredact.SecretPath(c.Request.URL.Path)
		}

		metrics.ActiveConnections.Inc()
		defer metrics.ActiveConnections.Dec()

		c.Next()

		duration := time.Since(start).Seconds()
		status := fmt.Sprintf("%d", c.Writer.Status())

		metrics.RequestDuration.WithLabelValues(c.Request.Method, path, status).Observe(duration)
		metrics.RequestTotal.WithLabelValues(c.Request.Method, path, status).Inc()
		metrics.RequestSize.WithLabelValues(c.Request.Method, path).Observe(float64(c.Request.ContentLength))
		metrics.ResponseSize.WithLabelValues(c.Request.Method, path).Observe(float64(c.Writer.Size()))
	}
}

// HealthCheck represents health check status
type HealthCheck struct {
	Status    string                 `json:"status"`
	Timestamp time.Time              `json:"timestamp"`
	Version   string                 `json:"version"`
	Checks    map[string]interface{} `json:"checks"`
}

// HealthChecker performs health checks
type HealthChecker struct {
	version string
	checks  map[string]func() error
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(version string) *HealthChecker {
	return &HealthChecker{
		version: version,
		checks:  make(map[string]func() error),
	}
}

// RegisterCheck registers a health check
func (h *HealthChecker) RegisterCheck(name string, check func() error) {
	h.checks[name] = check
}

// Check performs all health checks
func (h *HealthChecker) Check() *HealthCheck {
	result := &HealthCheck{
		Status:    "healthy",
		Timestamp: time.Now(),
		Version:   h.version,
		Checks:    make(map[string]interface{}),
	}

	for name, check := range h.checks {
		if err := check(); err != nil {
			result.Status = "unhealthy"
			result.Checks[name] = map[string]interface{}{
				"status": "unhealthy",
				"error":  err.Error(),
			}
		} else {
			result.Checks[name] = map[string]interface{}{
				"status": "healthy",
			}
		}
	}

	return result
}

// healthCheckTimeout is the maximum time a deep health check may take
// before the handler returns a degraded/timeout response.
const healthCheckTimeout = 100 * time.Millisecond

// GinHealthHandler returns Gin handler for health checks.
// The handler enforces a 100 ms timeout so that a slow database or
// external dependency never blocks callers for an unreasonable time.
func (h *HealthChecker) GinHealthHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		type result struct {
			health *HealthCheck
		}

		ch := make(chan result, 1)
		go func() {
			ch <- result{health: h.Check()}
		}()

		select {
		case r := <-ch:
			status := http.StatusOK
			if r.health.Status == "unhealthy" {
				status = http.StatusServiceUnavailable
			}
			c.JSON(status, r.health)

		case <-time.After(healthCheckTimeout):
			c.JSON(http.StatusOK, &HealthCheck{
				Status:    "degraded",
				Timestamp: time.Now(),
				Version:   h.version,
				Checks: map[string]interface{}{
					"timeout": map[string]interface{}{
						"status": "degraded",
						"error":  "health check exceeded 100ms timeout",
					},
				},
			})
		}
	}
}

// StartPrometheusServer starts the Prometheus metrics server.
// This listener has NO authentication (the main API serves /metrics behind the administrator
// gate), so it is bound to the loopback address only: a scraper on another host must go through
// the authenticated /metrics route of the main server. It returns the running server (nil when the
// port could not be bound) so callers and tests can read the bound address and shut it down.
func StartPrometheusServer(port int, logger *zap.Logger) *http.Server {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		logger.Error("Prometheus server cannot listen", zap.Error(err))
		return nil
	}
	server := &http.Server{
		Addr:              ln.Addr().String(),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	logger.Info("Starting Prometheus server", zap.String("addr", server.Addr))

	go func() {
		if err := server.Serve(ln); err != nil && err != http.ErrServerClosed {
			logger.Error("Prometheus server error", zap.Error(err))
		}
	}()
	return server
}
