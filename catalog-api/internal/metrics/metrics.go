package metrics

import (
	"runtime"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// LegacyPasswordHashes is the number of user rows whose stored password hash
	// is still the legacy hex SHA-256 shape. It is set at startup; the sunset
	// plan (docs/security/LEGACY_PASSWORD_HASH_SUNSET.md) retires the legacy
	// verify path once it stays at 0.
	LegacyPasswordHashes = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "catalogizer",
		Subsystem: "auth",
		Name:      "legacy_password_hashes",
		Help:      "User rows whose password hash is still the legacy hex SHA-256 shape.",
	})

	// HTTPRequestDuration tracks the duration of HTTP requests.
	HTTPRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "catalogizer",
		Subsystem: "http",
		Name:      "request_duration_seconds",
		Help:      "Duration of HTTP requests in seconds.",
		Buckets:   prometheus.DefBuckets,
	}, []string{"method", "path", "status"})

	// HTTPRequestsTotal counts the total number of HTTP requests.
	HTTPRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "catalogizer",
		Subsystem: "http",
		Name:      "requests_total",
		Help:      "Total number of HTTP requests.",
	}, []string{"method", "path", "status"})

	// HTTPActiveConnections tracks the number of currently active HTTP connections.
	HTTPActiveConnections = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "catalogizer",
		Subsystem: "http",
		Name:      "active_connections",
		Help:      "Number of currently active HTTP connections.",
	})

	// WebSocketConnections tracks the number of active WebSocket connections.
	WebSocketConnections = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "catalogizer",
		Subsystem: "websocket",
		Name:      "connections",
		Help:      "Number of active WebSocket connections.",
	})

	// SMBHealthStatus tracks the health status of SMB sources.
	// Values: 1 = healthy, 0.5 = degraded, 0 = offline.
	SMBHealthStatus = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "catalogizer",
		Subsystem: "smb",
		Name:      "health_status",
		Help:      "Health status of SMB sources (1=healthy, 0.5=degraded, 0=offline).",
	}, []string{"source"})

	// DBQueryDuration tracks the duration of database queries.
	DBQueryDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "catalogizer",
		Subsystem: "db",
		Name:      "query_duration_seconds",
		Help:      "Duration of database queries in seconds.",
		Buckets:   []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
	}, []string{"operation", "table"})

	// GoroutineCount tracks the number of goroutines.
	GoroutineCount = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "catalogizer",
		Subsystem: "runtime",
		Name:      "goroutines",
		Help:      "Number of goroutines currently running.",
	})

	// MemoryAlloc tracks the bytes of allocated heap objects.
	MemoryAlloc = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "catalogizer",
		Subsystem: "runtime",
		Name:      "memory_alloc_bytes",
		Help:      "Bytes of allocated heap objects.",
	})

	// MemorySys tracks the total bytes of memory obtained from the OS.
	MemorySys = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "catalogizer",
		Subsystem: "runtime",
		Name:      "memory_sys_bytes",
		Help:      "Total bytes of memory obtained from the OS.",
	})

	// MemoryHeapInuse tracks bytes in in-use heap spans.
	MemoryHeapInuse = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "catalogizer",
		Subsystem: "runtime",
		Name:      "memory_heap_inuse_bytes",
		Help:      "Bytes in in-use heap spans.",
	})
)

var (
	collectorOnce sync.Once
	stopChan      chan struct{}
)

// StartRuntimeCollector starts a background goroutine that periodically
// collects runtime metrics (goroutines, memory). Call StopRuntimeCollector
// to stop it during shutdown.
func StartRuntimeCollector(interval time.Duration) {
	collectorOnce.Do(func() {
		stopChan = make(chan struct{})
		go collectRuntimeMetrics(interval, stopChan)
	})
}

// StopRuntimeCollector stops the background runtime metrics collector.
func StopRuntimeCollector() {
	if stopChan != nil {
		close(stopChan)
	}
}

func collectRuntimeMetrics(interval time.Duration, stop <-chan struct{}) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Collect once immediately.
	updateRuntimeMetrics()

	for {
		select {
		case <-ticker.C:
			updateRuntimeMetrics()
		case <-stop:
			return
		}
	}
}

func updateRuntimeMetrics() {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	GoroutineCount.Set(float64(runtime.NumGoroutine()))
	MemoryAlloc.Set(float64(memStats.Alloc))
	MemorySys.Set(float64(memStats.Sys))
	MemoryHeapInuse.Set(float64(memStats.HeapInuse))
}

// SetSMBHealth sets the health status for an SMB source.
// Use the constants Healthy (1), Degraded (0.5), Offline (0).
func SetSMBHealth(source string, status float64) {
	SMBHealthStatus.WithLabelValues(source).Set(status)
}

// SMB health status constants.
const (
	SMBHealthy  = 1.0
	SMBDegraded = 0.5
	SMBOffline  = 0.0
)

// ObserveDBQuery records the duration of a database query.
func ObserveDBQuery(operation, table string, duration time.Duration) {
	DBQueryDuration.WithLabelValues(operation, table).Observe(duration.Seconds())
}
