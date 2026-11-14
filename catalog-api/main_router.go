package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/pprof"
	"os"
	"path"
	"sort"
	"strings"
	"time"

	root_config "catalogizer/config"
	"catalogizer/database"
	root_handlers "catalogizer/handlers"
	"catalogizer/internal/firebase"
	"catalogizer/internal/logging"
	"catalogizer/internal/logredact"
	"catalogizer/internal/metrics"
	"catalogizer/internal/middleware"
	"catalogizer/internal/services"
	root_middleware "catalogizer/middleware"
	root_repository "catalogizer/repository"

	"digital.vasic.discovery/pkg/broadcast"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

// routerDeps are the collaborators buildRouter wires into the engine up to and
// including the /api/v1/auth group (CAT-CHPW-RL-2). main() builds them; a unit
// test passes fakes for the non-auth ones and walks the REAL engine.
type routerDeps struct {
	Logger       *zap.Logger
	Cfg          *root_config.Config
	ServiceInfo  broadcast.ServiceInfo
	StartTime    time.Time
	FirebaseApp  *firebase.App
	JWT          *root_middleware.JWTMiddleware
	DB           *database.DB
	WS           *root_handlers.WebSocketHandler
	Asset        *root_handlers.AssetHandler
	Cover        *root_handlers.CoverHandler
	Auth         *root_handlers.AuthHandler
	MediaQuery   *root_handlers.MediaQueryHandler
	UserRepo     *root_repository.UserRepository
	LoginRate    gin.HandlerFunc
	AuthRate     gin.HandlerFunc
	DefaultRate  gin.HandlerFunc
	TrustedProxy string // TRUSTED_PROXIES value
}

// buildRouter creates the gin engine, installs the global middleware, registers
// the public routes and the /api/v1/auth group (through registerAuthRoutes), and
// returns the engine. main() registers the remaining /api/v1 routes on it. It is
// the single function both main() and TestBuildRouter* use, so the auth surface a
// test inspects is the surface production serves. It starts no server and opens
// no connection.
func buildRouter(d routerDeps) *gin.Engine {
	logger := d.Logger
	cfg := d.Cfg
	serviceInfo := d.ServiceInfo
	startTime := d.StartTime
	firebaseApp := d.FirebaseApp
	jwtMiddleware := d.JWT
	databaseDB := d.DB
	wsHandler := d.WS
	assetHandler := d.Asset
	coverHandler := d.Cover
	authHandler := d.Auth
	mediaQueryHandler := d.MediaQuery
	userRepo := d.UserRepo
	loginRateLimiter, authRateLimiter, defaultRateLimiter := d.LoginRate, d.AuthRate, d.DefaultRate

	// Setup Gin router
	// gin.Default() equivalent whose access log redacts credential-bearing query parameters (ticket, access_token, token).
	router := gin.New()
	router.Use(gin.LoggerWithFormatter(func(p gin.LogFormatterParams) string {
		return fmt.Sprintf("[GIN] %s | %3d | %13v | %15s | %-7s %#v\n%s",
			p.TimeStamp.Format("2006/01/02 - 15:04:05"), p.StatusCode, p.Latency, p.ClientIP, p.Method, logredact.Path(p.Request.URL.Path, p.Request.URL.RawQuery), logredact.ErrorText(p.ErrorMessage))
	}), middleware.Recovery(logger))

	// RULE-GO-005 / Master Plan §4.2 — lock down X-Forwarded-For
	// handling. Gin's default TrustedProxies is 0.0.0.0/0, which
	// lets any peer spoof the client IP via X-Forwarded-For and
	// bypass per-IP rate limits. Policy:
	//   - TRUSTED_PROXIES unset:    refuse to trust any proxy
	//     header; c.ClientIP() returns the actual peer IP.
	//   - TRUSTED_PROXIES=auto:     DEPRECATED, client-forgeable
	//     (CAT-TRUSTEDPROXIES-AUTO-1); trusts loopback + RFC-1918;
	//     refused in release mode unless TRUSTED_PROXIES_ALLOW_AUTO=true.
	//   - TRUSTED_PROXIES=<cidr>,...: explicit allow-list.
	warnTrustedProxies(logger, d.TrustedProxy)
	if err := applyTrustedProxies(router, d.TrustedProxy); err != nil {
		// applyTrustedProxies has already failed closed (no proxy trusted).
		logger.Error("TRUSTED_PROXIES rejected; trusting NO proxy header (every client behind a reverse proxy will share its address)", zap.Error(err))
	}

	// Service discovery (PUBLIC - must respond in < 2 seconds for Android TV)
	// Register BEFORE middleware chain to avoid slow middleware (CORS, metrics, compression)
	// This ensures the /discovery endpoint responds quickly for LAN discovery probes
	router.GET("/discovery", func(c *gin.Context) {
		host := serviceInfo.Host
		port := cfg.Server.Port
		c.JSON(200, gin.H{
			"service":        serviceInfo.Service,
			"name":           serviceInfo.Name,
			"version":        Version,
			"build":          BuildNumber,
			"build_date":     BuildDate,
			"host":           host,
			"port":           port,
			"protocol":       "http",
			"websocket_url":  fmt.Sprintf("ws://%s:%d/ws", host, port),
			"api_base_url":   fmt.Sprintf("http://%s:%d/api/v1", host, port),
			"capabilities":   serviceInfo.Capabilities,
			"database":       cfg.Database.Type,
			"instance_id":    serviceInfo.InstanceID,
			"uptime_seconds": int(time.Since(startTime).Seconds()),
		})
	})

	// Middleware
	router.Use(root_middleware.SecurityHeaders())
	router.Use(root_middleware.ConcurrencyLimiter(100))
	router.Use(root_middleware.RequestTimeout(60 * time.Second))
	router.Use(root_middleware.CORS())
	router.Use(metrics.GinMiddleware())
	router.Use(middleware.Logger(logger))
	router.Use(middleware.ErrorHandler())
	// Crashlytics middleware: captures HTTP 5xx responses as non-fatal
	// errors to Firebase. Safe no-op when Firebase is disabled.
	router.Use(firebase.GinMiddleware(firebaseApp))
	router.Use(root_middleware.RequestID())
	router.Use(root_middleware.InputValidation(root_middleware.DefaultInputValidationConfig()))
	router.Use(middleware.CompressionMiddleware(middleware.DefaultCompressionConfig()))

	// Prometheus metrics endpoint
	// Authz round 2: the exposition lists routes, hosts, runtime internals and
	// per-endpoint traffic, so it is administrator-only (a scraper presents an
	// administrator bearer token or the dedicated METRICS_SCRAPE_TOKEN, valid on this route only). /health and /health/deep stay public.
	router.GET("/metrics", metricsScrapeDetect(metricsScrapeToken()), metricsSkipIfScrape(jwtMiddleware.RequireAuth()), metricsSkipIfScrape(jwtMiddleware.RequireAdmin()), gin.WrapH(promhttp.Handler()))

	// pprof endpoints — OPT-IN via HELIX_PPROF_ENABLED=true.
	//
	// DEFER-QA-2026-04-21-002 follow-up: heap/goroutine profiling was
	// previously unavailable, so the 2026-04-20 RunAll memory burst
	// (peak 53.5× baseline heap) could only be assessed from logs. With
	// this flag set, /debug/pprof/{heap,goroutine,profile,trace,…}
	// become available (standard net/http/pprof handlers). The flag
	// defaults OFF so untrusted networks never see the endpoints by
	// accident — operators flip it on, capture profiles, flip it off.
	if os.Getenv("HELIX_PPROF_ENABLED") == "true" {
		pprofGroup := router.Group("/debug/pprof")
		// Authz round 2: heap/goroutine/cmdline output exposes memory contents and
		// process arguments; the flag turns the routes on, the admin gate decides who
		// may read them.
		pprofGroup.Use(jwtMiddleware.RequireAuth(), jwtMiddleware.RequireAdmin(), root_middleware.PprofSamplingGuard(60)) // SECQ-3: sampling window capped at 60 s, one at a time
		pprofGroup.GET("/", gin.WrapF(pprof.Index))
		pprofGroup.GET("/cmdline", gin.WrapF(pprof.Cmdline))
		pprofGroup.GET("/profile", gin.WrapF(pprof.Profile))
		pprofGroup.POST("/symbol", gin.WrapF(pprof.Symbol))
		pprofGroup.GET("/symbol", gin.WrapF(pprof.Symbol))
		pprofGroup.GET("/trace", gin.WrapF(pprof.Trace))
		pprofGroup.GET("/allocs", gin.WrapH(pprof.Handler("allocs")))
		pprofGroup.GET("/block", gin.WrapH(pprof.Handler("block")))
		pprofGroup.GET("/goroutine", gin.WrapH(pprof.Handler("goroutine")))
		pprofGroup.GET("/heap", gin.WrapH(pprof.Handler("heap")))
		pprofGroup.GET("/mutex", gin.WrapH(pprof.Handler("mutex")))
		pprofGroup.GET("/threadcreate", gin.WrapH(pprof.Handler("threadcreate")))
		logging.Info("pprof endpoints enabled at /debug/pprof/*")
	}

	// Health check (short-lived cache to reduce redundant health polling).
	//
	// /api/v1/health is registered as an alias here — several HelixQA
	// test banks + external monitors probe that path. Before the alias,
	// the 2026-04-20 Article VII RunAll log showed 11× 404 on it
	// (bank/monitor noise, not product failures). Keeping both paths
	// unauthenticated matches the canonical /health contract.
	healthHandler := func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":       "healthy",
			"time":         time.Now().UTC(),
			"version":      Version,
			"build_number": BuildNumber,
			"build_date":   BuildDate,
		})
	}
	router.GET("/health", root_middleware.CacheHeaders(5), healthHandler)
	router.GET("/api/v1/health", root_middleware.CacheHeaders(5), healthHandler)

	// Deep health check — pings the database with a 100ms timeout so
	// a slow DB never blocks callers for an unreasonable duration.
	deepHealthChecker := metrics.NewHealthChecker(databaseDB, Version)
	router.GET("/health/deep", root_middleware.CacheHeaders(5), deepHealthHandler(deepHealthChecker, Version, 100*time.Millisecond))

	// WebSocket endpoint: authenticated (Bearer header or ?token=, session-checked) and Origin-checked; see registerWebSocketRoute
	registerWebSocketRoute(router, jwtMiddleware, wsHandler)

	// Image proxy — serves external images (TMDB, etc.) through the API.
	// When the upstream CDN is unreachable, falls back to a type-specific
	// placeholder SVG so client apps still render something useful.
	router.GET("/api/v1/image-proxy", func(c *gin.Context) {
		imageURL := c.Query("url")
		if imageURL == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "url parameter required"})
			return
		}
		// Exact parsed-host match, https only (review round 1, F4): a substring test let
		// http://127.0.0.1/internal?x=image.tmdb.org through.
		parsedImage, perr := validateImageProxyURL(imageURL)
		if perr != nil {
			c.JSON(http.StatusForbidden, gin.H{"error": "domain not allowed"})
			return
		}
		imageURL = parsedImage.String()

		// Build a proxy client that can work around DNS hijacking.
		proxyClient := newImageProxyClient(imageURL, cfg.Proxy)
		resp, err := proxyClient.Get(imageURL)
		if err != nil || resp.StatusCode != http.StatusOK {
			if resp != nil && resp.Body != nil {
				resp.Body.Close()
			}
			mediaType := c.DefaultQuery("type", "movie")
			svg := services.GeneratePlaceholderSVG(mediaType)
			c.Header("Content-Type", "image/svg+xml")
			c.Header("Cache-Control", "public, max-age=3600")
			c.Data(http.StatusOK, "image/svg+xml", svg)
			return
		}
		defer resp.Body.Close()
		c.Header("Content-Type", resp.Header.Get("Content-Type"))
		c.Header("Cache-Control", "public, max-age=86400") // Cache 24h
		c.Status(resp.StatusCode)
		if _, copyErr := io.Copy(c.Writer, resp.Body); copyErr != nil {
			logger.Warn("Image proxy stream copy failed", zap.Error(copyErr))
		}
	})

	// Asset serving (public — no auth needed for serving images)
	// StaticCacheHeaders() sets Cache-Control: public, max-age=31536000, immutable
	// for fingerprinted/content-hashed assets. Also suitable for future static file
	// server routes (e.g., router.Static("/static", "./public", root_middleware.StaticCacheHeaders())).
	router.GET("/api/v1/assets/:id", root_middleware.StaticCacheHeaders(), assetHandler.ServeAsset)

	// Cover image serving (public — no auth needed for serving cover images)
	router.GET("/api/v1/cover/placeholder/:type", root_middleware.CacheHeaders(86400), coverHandler.ServePlaceholder)
	router.GET("/api/v1/cover/url/:id", root_middleware.CacheHeaders(300), coverHandler.GetCoverURL)
	router.GET("/api/v1/cover/:id", root_middleware.CacheHeaders(86400), coverHandler.ServeCover)

	// Authentication routes (registered by registerAuthRoutes, main_chpw.go; the
	// route table is pinned by TestRegisterAuthRoutes_*, CAT-CHPW-RL-1).
	authGroup := router.Group("/api/v1/auth")
	registerAuthRoutes(authGroup, authRoutesDeps{
		LoginRate:      loginRateLimiter,
		AuthRate:       authRateLimiter,
		DefaultRate:    defaultRateLimiter,
		RequireAuth:    jwtMiddleware.RequireAuth,
		Login:          authHandler.LoginGin,
		Register:       func(c *gin.Context) { authHandler.RegisterGin(c, userRepo) },
		Refresh:        authHandler.RefreshTokenGin,
		Logout:         authHandler.LogoutGin,
		Me:             authHandler.GetCurrentUserGin,
		Status:         authHandler.GetAuthStatusGin,
		Permissions:    authHandler.GetPermissionsGin,
		InitStatus:     mediaQueryHandler.GetInitStatus,
		ChangePassword: mediaQueryHandler.ChangePassword,
		Sessions:       userRepo,
		Logger:         logger,
	})
	return router
}

// authRouteTable is the exact set of routes allowed on the /api/v1/auth surface
// ("METHOD path", sorted). verifyAuthRouteTable enforces it at startup over the
// engine main() finished building, so a route added anywhere in main() (or in a
// helper main() hands a group to) under /api/v1/auth, however its path is spelled,
// stops the process instead of serving unrated. TestBuildRouterAuthSurface pins the
// same table together with every route's handler chain (CAT-CHPW-RL-2).
var authRouteTable = []string{
	"GET /api/v1/auth/init-status",
	"GET /api/v1/auth/me",
	"GET /api/v1/auth/permissions",
	"GET /api/v1/auth/profile",
	"GET /api/v1/auth/status",
	"POST /api/v1/auth/change-password",
	"POST /api/v1/auth/login",
	"POST /api/v1/auth/logout",
	"POST /api/v1/auth/refresh",
	"POST /api/v1/auth/register",
}

// onAuthSurface reports whether a registered route path is on, or could be
// mistaken for, the auth surface: after gin's own path cleaning and ignoring
// case, anything under /api/v1/auth or a bare /auth, and any parameter or wildcard
// segment directly under /api/v1 (which could capture an auth path).
func onAuthSurface(p string) bool {
	segs := strings.Split(strings.Trim(path.Clean("/"+p), "/"), "/")
	for i := range segs {
		segs[i] = strings.ToLower(segs[i])
	}
	if len(segs) >= 3 && segs[0] == "api" && segs[1] == "v1" {
		return segs[2] == "auth" || strings.HasPrefix(segs[2], ":") || strings.HasPrefix(segs[2], "*")
	}
	return segs[0] == "auth"
}

// verifyAuthRouteTable returns an error when the engine's auth-surface routes
// differ from authRouteTable.
func verifyAuthRouteTable(e *gin.Engine) error {
	var got []string
	for _, ri := range e.Routes() {
		if onAuthSurface(ri.Path) {
			got = append(got, ri.Method+" "+ri.Path)
		}
	}
	sort.Strings(got)
	if strings.Join(got, "\n") != strings.Join(authRouteTable, "\n") {
		return fmt.Errorf("auth route table mismatch: got %q, want %q", got, authRouteTable)
	}
	return nil
}
