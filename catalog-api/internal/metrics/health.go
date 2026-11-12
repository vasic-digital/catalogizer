package metrics

import (
	"context"
	"net/http"
	"os/exec"
	"sync"
	"time"

	"catalogizer/database"
	"catalogizer/internal/logging"
)

// HealthStatus represents the health status of a component
type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusDegraded  HealthStatus = "degraded"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
)

// ComponentHealth represents the health of a single component
type ComponentHealth struct {
	Status  HealthStatus `json:"status"`
	Message string       `json:"message,omitempty"`
	Latency string       `json:"latency,omitempty"`
}

// HealthCheckResponse represents the overall health check response
type HealthCheckResponse struct {
	Status     HealthStatus               `json:"status"`
	Timestamp  time.Time                  `json:"timestamp"`
	Version    string                     `json:"version"`
	Uptime     string                     `json:"uptime"`
	Components map[string]ComponentHealth `json:"components"`
	// Converters reports, from real PATH lookups at request time, which external
	// converter tools the conversion worker needs are installed on this server.
	Converters map[string]bool `json:"converters"`
}

// ConverterTools are the external tools the conversion/PDF features exec:
// ffmpeg (audio/video), convert (ImageMagick: images, PDF rasterising) and
// pdftotext (poppler: PDF text extraction).
var ConverterTools = []string{"ffmpeg", "convert", "pdftotext"}

// ConverterPDFRaster is the extra capability key reported next to the tools:
// ImageMagick rasterises a PDF (bmp/tiff/gif/webp output) only through
// Ghostscript, so it is true only when both `convert` and `gs` are on PATH.
// `convert: true` alone means images, never PDF rasterising. The values are
// booleans only: no paths or versions are disclosed on this unauthenticated
// endpoint.
const ConverterPDFRaster = "convert_pdf"

// ConverterSVGRender is true only when `convert` AND a known SVG renderer (Inkscape or
// rsvg-convert) are on PATH. `convert: true` alone does not mean SVG input works: ImageMagick's
// built-in MSVG renderer is unreliable and the policy-hardened path is only exercised with an
// external renderer. Booleans only, like the other keys.
const ConverterSVGRender = "convert_svg"

// ProbeConverters looks every ConverterTools entry up on PATH (exec.LookPath,
// the same resolution the conversion worker uses when it runs the tool) and
// derives ConverterPDFRaster.
func ProbeConverters() map[string]bool {
	out := make(map[string]bool, len(ConverterTools)+2)
	for _, t := range ConverterTools {
		_, err := exec.LookPath(t)
		out[t] = err == nil
	}
	_, gsErr := exec.LookPath("gs")
	out[ConverterPDFRaster] = out["convert"] && gsErr == nil
	_, inkErr := exec.LookPath("inkscape")
	_, rsvgErr := exec.LookPath("rsvg-convert")
	out[ConverterSVGRender] = out["convert"] && (inkErr == nil || rsvgErr == nil)
	return out
}

// HealthChecker performs health checks on various components
type HealthChecker struct {
	db        *database.DB
	startTime time.Time
	version   string
	mu        sync.RWMutex
	checks    map[string]func(context.Context) ComponentHealth
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(db *database.DB, version string) *HealthChecker {
	hc := &HealthChecker{
		db:        db,
		startTime: time.Now(),
		version:   version,
		checks:    make(map[string]func(context.Context) ComponentHealth),
	}

	// Register default checks
	hc.RegisterCheck("database", hc.checkDatabase)

	return hc
}

// RegisterCheck registers a custom health check
func (hc *HealthChecker) RegisterCheck(name string, check func(context.Context) ComponentHealth) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	hc.checks[name] = check
}

// Check performs all health checks and returns the overall health status
func (hc *HealthChecker) Check(ctx context.Context) HealthCheckResponse {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	components := make(map[string]ComponentHealth)
	overallStatus := HealthStatusHealthy

	// Run all checks
	for name, checkFunc := range hc.checks {
		componentHealth := checkFunc(ctx)
		components[name] = componentHealth

		// Determine overall status
		if componentHealth.Status == HealthStatusUnhealthy {
			overallStatus = HealthStatusUnhealthy
		} else if componentHealth.Status == HealthStatusDegraded && overallStatus != HealthStatusUnhealthy {
			overallStatus = HealthStatusDegraded
		}
	}

	return HealthCheckResponse{
		Status:     overallStatus,
		Timestamp:  time.Now(),
		Version:    hc.version,
		Uptime:     time.Since(hc.startTime).String(),
		Components: components,
		Converters: ProbeConverters(),
	}
}

// checkDatabase checks database connectivity
func (hc *HealthChecker) checkDatabase(ctx context.Context) ComponentHealth {
	if hc.db == nil {
		return ComponentHealth{
			Status:  HealthStatusUnhealthy,
			Message: "Database not configured",
		}
	}

	start := time.Now()

	// Create context with timeout
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := hc.db.PingContext(pingCtx); err != nil {
		// PX-F23: the ping error (driver/host text) must not reach the
		// unauthenticated /health/deep body; log server-side only.
		if logging.Logger != nil {
			logging.With(logging.ErrorField(err)).Error("Database ping failed")
		}
		return ComponentHealth{
			Status:  HealthStatusUnhealthy,
			Message: "Database ping failed",
		}
	}

	latency := time.Since(start)

	// Check connection pool stats
	stats := hc.db.Stats()
	if stats.OpenConnections >= stats.MaxOpenConnections-1 {
		return ComponentHealth{
			Status:  HealthStatusDegraded,
			Message: "Database connection pool near limit",
			Latency: latency.String(),
		}
	}

	return ComponentHealth{
		Status:  HealthStatusHealthy,
		Latency: latency.String(),
	}
}

// LivenessProbe returns a simple liveness check (is the service running?)
func (hc *HealthChecker) LivenessProbe() int {
	// Always return 200 if the service is running
	return http.StatusOK
}

// ReadinessProbe returns a readiness check (is the service ready to serve traffic?)
func (hc *HealthChecker) ReadinessProbe(ctx context.Context) int {
	health := hc.Check(ctx)

	if health.Status == HealthStatusHealthy || health.Status == HealthStatusDegraded {
		return http.StatusOK
	}

	return http.StatusServiceUnavailable
}

// StartupProbe returns a startup check (has the service finished initializing?)
func (hc *HealthChecker) StartupProbe(ctx context.Context) int {
	health := hc.Check(ctx)

	if health.Status != HealthStatusUnhealthy {
		return http.StatusOK
	}

	return http.StatusServiceUnavailable
}
