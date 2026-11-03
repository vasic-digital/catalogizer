// Package modules provides a centralized registry for all external Go modules
// wired into catalog-api. Each module from the digital.vasic.* family is
// imported, verified at compile time, and optionally initialized during
// application startup.
//
// This ensures that all 12 previously-unused modules are compiled into the
// binary and available for deeper integration in future phases.
package modules

import (
	"context"
	"time"

	"catalogizer/internal/logging"
	"go.uber.org/zap"

	// Database module — generic connection pool, query builder, repository pattern
	dbhelpers "digital.vasic.database/pkg/helpers"
	dbpool "digital.vasic.database/pkg/pool"

	// Lazy module — generic lazy-loading primitives (Value[T], Service[T])
	"digital.vasic.lazy/pkg/lazy"

	// Media module — media type detection, metadata extraction, provider registry
	"digital.vasic.media/pkg/detector"
	mediamodels "digital.vasic.media/pkg/models"

	// Memory module — memory leak detection, goroutine monitoring
	"digital.vasic.memory/pkg/memory"

	// Middleware module — reusable net/http middleware (CORS, logging, recovery, chain)
	"digital.vasic.middleware/pkg/chain"
	mwheaders "digital.vasic.middleware/pkg/requestid"

	// Observability module — structured logging, Prometheus metrics, health checks
	obshealth "digital.vasic.observability/pkg/health"
	obslogging "digital.vasic.observability/pkg/logging"
	obsmetrics "digital.vasic.observability/pkg/metrics"

	// RateLimiter module — rate limiting interfaces and in-memory implementation
	"digital.vasic.ratelimiter/pkg/limiter"

	// Recovery module — circuit breaker manager, health checker, resilience facade
	"digital.vasic.recovery/pkg/breaker"
	"digital.vasic.recovery/pkg/facade"

	// Security module — content guardrails, PII detection, security headers
	"digital.vasic.security/pkg/guardrails"
	secheaders "digital.vasic.security/pkg/headers"
	"digital.vasic.security/pkg/scanner"

	// Storage module — object storage interface, S3/local backends, resolver
	"digital.vasic.storage/pkg/resolver"

	// Streaming module — SSE, WebSocket hub, transport factory
	"digital.vasic.streaming/pkg/transport"

	// Watcher module — filesystem change monitoring with debouncing
	"digital.vasic.watcher/pkg/watcher"
)

// ModuleInfo holds metadata about a registered module.
type ModuleInfo struct {
	Name        string
	Description string
	Initialized bool
}

// Registry holds references to initialized module components.
type Registry struct {
	// HealthAggregator aggregates health checks from all modules.
	// Uses digital.vasic.observability/pkg/health.
	HealthAggregator *obshealth.Aggregator

	// MemoryMonitor tracks memory usage and detects potential leaks.
	// Uses digital.vasic.memory/pkg/memory.
	MemoryMonitor *memory.MemoryMonitor

	// ResilienceFacade provides circuit breakers and health checks.
	// Uses digital.vasic.recovery/pkg/facade.
	ResilienceFacade *facade.Resilience

	// GuardrailEngine validates content against configured rules.
	// Uses digital.vasic.security/pkg/guardrails.
	GuardrailEngine *guardrails.Engine

	// StorageResolver routes asset paths to storage backends.
	// Uses digital.vasic.storage/pkg/resolver.
	StorageResolver *resolver.Resolver

	// TransportFactory creates transport instances by protocol type.
	// Uses digital.vasic.streaming/pkg/transport.
	TransportFactory *transport.Factory

	// MediaDetector detects media types from filenames and paths.
	// Uses digital.vasic.media/pkg/detector.
	MediaDetector *detector.Engine

	// Modules lists all registered modules with their status.
	Modules []ModuleInfo
}

// registryLogger adapts log.Printf to the breaker.Logger interface
// required by the Recovery module.
type registryLogger struct{}

func (l *registryLogger) Info(msg string, keysAndValues ...interface{}) {
	if logging.SugaredLogger != nil {
		logging.SugaredLogger.With(logging.String("component", "modules")).Infow(msg, keysAndValues...)
	}
}

func (l *registryLogger) Warn(msg string, keysAndValues ...interface{}) {
	if logging.SugaredLogger != nil {
		logging.SugaredLogger.With(logging.String("component", "modules")).Warnw(msg, keysAndValues...)
	}
}

func (l *registryLogger) Debug(msg string, keysAndValues ...interface{}) {
	// Debug messages are suppressed in production
}

// RegisterModules initializes all external Go modules and returns a Registry
// containing their active instances. This function should be called during
// application startup, after the database and logger are initialized.
func RegisterModules() *Registry {
	reg := &Registry{}

	// --- 1. Database module ---
	// Verify pool configuration defaults and transaction helper availability.
	poolCfg := dbpool.DefaultPoolConfig()
	if err := poolCfg.Validate(); err != nil {
		if logging.Logger != nil {
			logging.Warnf("Database pool config validation failed: %v", err)
		}
	}
	// dbhelpers.WithTransaction is available for wrapping repository operations.
	_ = dbhelpers.WithTransaction // compile-time verification
	reg.Modules = append(reg.Modules, ModuleInfo{
		Name:        "digital.vasic.database",
		Description: "Generic connection pool, query builder, transaction helpers",
		Initialized: true,
	})

	// --- 2. Lazy module ---
	// Verify lazy loading primitives compile correctly with generic types.
	_ = lazy.NewValue(func() (string, error) { return "catalogizer", nil })
	_ = lazy.NewService(func() (int, error) { return 0, nil })
	reg.Modules = append(reg.Modules, ModuleInfo{
		Name:        "digital.vasic.lazy",
		Description: "Generic lazy-loading primitives (Value[T], Service[T])",
		Initialized: true,
	})

	// --- 3. Media module ---
	// Initialize media detection engine for filename-based media type detection.
	// This augments the existing internal/media/detector pipeline.
	reg.MediaDetector = detector.NewEngine()
	// Verify media model types are available for entity mapping.
	_ = mediamodels.MediaItem{}
	_ = mediamodels.SearchRequest{}
	reg.Modules = append(reg.Modules, ModuleInfo{
		Name:        "digital.vasic.media",
		Description: "Media type detection, metadata extraction, provider registry",
		Initialized: true,
	})

	// --- 4. Memory module ---
	// Initialize memory monitor for production leak detection.
	// Samples every 60 seconds, alerts if heap grows beyond 10× baseline.
	//
	// DEFER-QA-2026-04-21-002 follow-up: the previous 3× threshold
	// was too tight for legitimate burst workloads. During the
	// 2026-04-20 RunAll (508 challenges buffered as in-memory Result
	// slices + transient HTTP buffers), heap peaked at 53.5× baseline
	// and fired 26 false-positive alerts, settling back under 2×
	// within 5 minutes of the burst. 10× retains early-warning
	// coverage while silencing legitimate burst workloads.
	reg.MemoryMonitor = memory.NewMemoryMonitor(60*time.Second, memoryHeapThresholdRatio)
	reg.MemoryMonitor.SetAlertCallback(func(report memory.LeakReport) {
		trigger := classifyMemoryAlert(report, memoryHeapThresholdRatio)
		fields := []zap.Field{
			logging.String("component", "modules"),
			logging.String("trigger", trigger),
			logging.Float64("heap_growth_ratio", report.HeapGrowthRatio),
			logging.Float64("heap_threshold_ratio", memoryHeapThresholdRatio),
			logging.Int("goroutine_count", report.GoroutineCount),
			logging.Int("initial_goroutines", report.InitialGoroutines),
		}
		if trigger == memoryTriggerHeap {
			logging.Warn("MEMORY ALERT: potential leak detected", fields...)
			return
		}
		// S1: the goroutine criterion (>50% over the baseline taken at start-up) fires on plain
		// concurrent load - every in-flight/keep-alive connection owns a goroutine - while the heap
		// ratio is far below its threshold. Measured: 29 goroutines idle, 207-256 during 64 clients,
		// back to 29 afterwards. Reported as a notice, not as a leak alert.
		logging.Info("MEMORY NOTICE: goroutine count above start-up baseline (load-correlated; heap ratio below leak threshold)", fields...)
	})
	reg.Modules = append(reg.Modules, ModuleInfo{
		Name:        "digital.vasic.memory",
		Description: "Memory leak detection, goroutine monitoring",
		Initialized: true,
	})

	// --- 5. Middleware module ---
	// Verify middleware chain and request ID generator are available.
	// These are net/http middleware; catalog-api uses Gin, but they can
	// wrap the underlying net/http handler when needed.
	_ = chain.Chain   // middleware chaining utility
	_ = mwheaders.New // request ID generator
	reg.Modules = append(reg.Modules, ModuleInfo{
		Name:        "digital.vasic.middleware",
		Description: "Reusable net/http middleware (CORS, logging, recovery, request ID, chain)",
		Initialized: true,
	})

	// --- 6. Observability module ---
	// Initialize health check aggregator for structured health reporting.
	reg.HealthAggregator = obshealth.NewAggregator(obshealth.DefaultAggregatorConfig())
	// Verify structured logger and metrics collector interfaces are available.
	_ = obslogging.DefaultConfig()
	_ = obsmetrics.DefaultPrometheusConfig()
	reg.Modules = append(reg.Modules, ModuleInfo{
		Name:        "digital.vasic.observability",
		Description: "Structured logging, Prometheus metrics, health check aggregation, tracing",
		Initialized: true,
	})

	// --- 7. RateLimiter module ---
	// Verify rate limiter configuration is available. The module provides
	// in-memory and Redis-backed implementations that complement the existing
	// auth-based rate limiting in middleware.
	rlCfg := limiter.DefaultConfig()
	_ = rlCfg.EffectiveBurst()
	reg.Modules = append(reg.Modules, ModuleInfo{
		Name:        "digital.vasic.ratelimiter",
		Description: "Rate limiting interfaces, in-memory and Redis implementations",
		Initialized: true,
	})

	// --- 8. Recovery module ---
	// Initialize resilience facade with circuit breakers and health checking.
	rl := &registryLogger{}
	reg.ResilienceFacade = facade.New(rl)
	// Pre-create circuit breakers for known failure domains.
	reg.ResilienceFacade.GetOrCreateBreaker("smb", breaker.CircuitBreakerConfig{
		MaxFailures:  5,
		ResetTimeout: 30 * time.Second,
	})
	reg.ResilienceFacade.GetOrCreateBreaker("metadata-providers", breaker.CircuitBreakerConfig{
		MaxFailures:  3,
		ResetTimeout: 60 * time.Second,
	})
	reg.Modules = append(reg.Modules, ModuleInfo{
		Name:        "digital.vasic.recovery",
		Description: "Circuit breaker manager, health checker, resilience facade",
		Initialized: true,
	})

	// --- 9. Security module ---
	// Initialize content guardrail engine with default rules.
	reg.GuardrailEngine = guardrails.NewEngine(guardrails.DefaultConfig())
	reg.GuardrailEngine.AddRule(guardrails.NewMaxLengthRule(1_000_000)) // 1MB content limit
	// Verify security headers and scanner interfaces are available.
	_ = secheaders.DefaultConfig()
	_ = scanner.NewReport(nil, "catalogizer", "", 0)
	reg.Modules = append(reg.Modules, ModuleInfo{
		Name:        "digital.vasic.security",
		Description: "Content guardrails, PII detection, security headers, vulnerability scanning",
		Initialized: true,
	})

	// --- 10. Storage module ---
	// Initialize storage path resolver for future S3/local backend routing.
	reg.StorageResolver = resolver.New()
	reg.Modules = append(reg.Modules, ModuleInfo{
		Name:        "digital.vasic.storage",
		Description: "Object storage interface, S3/local backends, path resolver",
		Initialized: true,
	})

	// --- 11. Streaming module ---
	// Initialize transport factory with built-in HTTP/WebSocket/gRPC creators.
	reg.TransportFactory = transport.NewFactory()
	reg.Modules = append(reg.Modules, ModuleInfo{
		Name:        "digital.vasic.streaming",
		Description: "SSE, WebSocket hub, gRPC streaming, transport factory",
		Initialized: true,
	})

	// --- 12. Watcher module ---
	// Verify filesystem watcher configuration is available. The watcher
	// provides fsnotify-backed file monitoring with debouncing — complementing
	// the existing SMB change watcher in internal/smb.
	watchCfg := watcher.DefaultConfig()
	_ = watchCfg.Recursive
	_ = watchCfg.DebounceDelay
	reg.Modules = append(reg.Modules, ModuleInfo{
		Name:        "digital.vasic.watcher",
		Description: "Filesystem change monitoring with debouncing and filtering",
		Initialized: true,
	})

	if logging.Logger != nil {
		logging.With(logging.String("component", "modules")).Info("All external modules registered successfully", logging.Int("count", len(reg.Modules)))
	}

	// Use digital.vasic.observability structured logger to record module registration.
	// This provides JSON-formatted, correlation-ID-aware logging alongside the
	// standard log.Printf calls above.
	structuredLogger := obslogging.NewLogrusAdapter(&obslogging.Config{
		Level:       obslogging.InfoLevel,
		Format:      "json",
		ServiceName: "catalogizer-api",
	})
	structuredLogger.WithField("module_count", len(reg.Modules)).Info("All external modules registered successfully")

	return reg
}

// StartBackgroundServices starts long-running module services (memory monitor,
// health checks). Call this after RegisterModules during application startup.
// The returned cancel function stops all background services.
func (r *Registry) StartBackgroundServices(ctx context.Context) {
	// Start memory monitoring after a short delay so the baseline is captured
	// after the application has finished initializing its background goroutines
	// (HTTP server, WebSocket handler, cache cleanup, etc.). This prevents
	// false-positive leak alerts from normal startup goroutines.
	if r.MemoryMonitor != nil {
		go func() {
			timer := time.NewTimer(30 * time.Second)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
			}
			if err := r.MemoryMonitor.Start(ctx); err != nil {
				if logging.Logger != nil {
					logging.With(logging.String("component", "modules"), logging.ErrorField(err)).Warn("Failed to start memory monitor")
				}
			} else {
				if logging.Logger != nil {
					logging.With(logging.String("component", "modules")).Info("Memory monitor started", logging.String("interval", "60s"), logging.String("threshold", "10x"), logging.String("delay", "30s"))
				}
			}
		}()
	}
}

// Stop gracefully shuts down all module background services.
func (r *Registry) Stop() {
	if r.MemoryMonitor != nil {
		r.MemoryMonitor.Stop()
	}
	if r.ResilienceFacade != nil {
		r.ResilienceFacade.Stop()
	}
	if logging.Logger != nil {
		logging.With(logging.String("component", "modules")).Info("All module services stopped")
	}
}

// GetModuleInfo returns information about all registered modules.
func (r *Registry) GetModuleInfo() []ModuleInfo {
	result := make([]ModuleInfo, len(r.Modules))
	copy(result, r.Modules)
	return result
}

// memoryHeapThresholdRatio is the heap-growth ratio (HeapAlloc / baseline HeapAlloc) above which the
// memory monitor reports a potential leak.
const memoryHeapThresholdRatio = 10.0

const (
	memoryTriggerHeap       = "heap"
	memoryTriggerGoroutines = "goroutines"
)

// classifyMemoryAlert names the criterion that made the memory monitor flag a report: the heap ratio
// above its threshold is a leak signal; a goroutine-only trigger is load-correlated.
func classifyMemoryAlert(report memory.LeakReport, heapThreshold float64) string {
	if report.HeapGrowthRatio > heapThreshold {
		return memoryTriggerHeap
	}
	return memoryTriggerGoroutines
}
