package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/pprof"
	"os"
	"path"
	"reflect"
	"regexp"
	"runtime"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"time"
	"unsafe"

	root_config "catalogizer/config"
	"catalogizer/database"
	root_handlers "catalogizer/handlers"
	"catalogizer/internal/firebase"
	"catalogizer/internal/logging"
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
// test passes fakes for the non-auth ones and walks the REAL engine. Boundary:
// main() still registers the remaining /api/v1 routes inline on the engine this
// function returns (too large to extract); those are covered at source level by the
// package-main AST scan and at startup by verifyAuthRouteTable, not by a unit test
// that builds them.
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
	router.Use(gin.LoggerWithFormatter(middleware.AccessLogFormatter), middleware.AccessLogRouteParams(), middleware.Recovery(logger))

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

	// WebSocket endpoint: authenticated (Bearer header or a single-use ?ticket= from POST /api/v1/ws/ticket, session-checked) and Origin-checked; see registerWebSocketRoute
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
	// route table and every route's handler chain are pinned by
	// TestBuildRouterAuthSurface, CAT-CHPW-RL-2).
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
// engine main() finished building.
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

// altSvcMiddleware advertises the HTTP/3 endpoint. main() installs it with the one
// late engine Use (after every route is registered, so in gin it only joins the
// NoRoute/NoMethod chains, not the chains of already registered routes). It is a
// named function so the startup verifier and the AST scan can pin its identity.
func altSvcMiddleware(c *gin.Context) {
	c.Header("Alt-Svc", `h3=":28443"; ma=86400`)
	c.Next()
}

// wantGlobalChain is the exact engine-level handler chain (the chain prefix of every
// route registered after buildRouter's Use calls, and the NoRoute/NoMethod fallback
// chain). Entries are handlerKey values. verifyAuthRouteTable requires the live
// chain to equal it, optionally followed by altSvcMiddleware, so a global middleware
// added anywhere (answering an /api/v1/auth path without the route's limiter) fails
// startup. Changing it is a deliberate edit of this list.
var wantGlobalChain = []string{
	"github.com/gin-gonic/gin@v1.12.0/logger.go:LoggerWithConfig",
	"internal/middleware/middleware.go:AccessLogRouteParams",
	"internal/middleware/recovery.go:Recovery",
	"middleware/security_headers.go:SecurityHeadersWithConfig",
	"middleware/concurrency_limiter.go:ConcurrencyLimiter",
	"middleware/timeout.go:RequestTimeout",
	"middleware/request.go:CORS",
	"internal/metrics/middleware.go:GinMiddleware",
	"internal/middleware/middleware.go:Logger",
	"internal/middleware/middleware.go:ErrorHandler",
	"internal/firebase/firebase.go:GinMiddleware",
	"middleware/request.go:RequestID",
	"middleware/input_validation.go:InputValidation",
	"internal/middleware/compression.go:CompressionMiddleware",
}

const altSvcKey = "main_router.go:altSvcMiddleware"

var funcNRe = regexp.MustCompile(`(\.func\d+(\.\d+)*|\.gowrap\d+)+$`)

// handlerKey identifies a handler by SOURCE FILE plus its last identifier, so a
// lookalike from another file or package is a different key, and inlining (which
// renames a closure after its caller) does not change it. The file is made
// relative to the module root, which is derived at run time from the location of
// this package's own main_router.go (so it is the same under any checkout
// directory, with -trimpath, and in a scratch clone); files of dependencies are
// cut at "<module>@<version>" using the build info. Method-value wrappers have no
// source file ("-fm", generated): they keep their full import-path name. Anything
// whose location cannot be resolved keeps its full path, which can never equal a
// pinned key, so it fails closed.
func handlerKey(h gin.HandlerFunc) string {
	f := runtime.FuncForPC(reflect.ValueOf(h).Pointer())
	if f == nil {
		return "<unknown>"
	}
	name := funcNRe.ReplaceAllString(f.Name(), "")
	file, _ := f.FileLine(f.Entry())
	if file == "" || strings.HasPrefix(file, "<") || strings.HasSuffix(file, "<autogenerated>") {
		return "<generated>:" + name
	}
	ident := name
	if i := strings.LastIndex(ident, "(*"); i >= 0 {
		ident = ident[i:]
	} else if i := strings.LastIndex(ident, "."); i >= 0 {
		ident = ident[i+1:]
	}
	return keyFile(file) + ":" + ident
}

var moduleRootOnce struct {
	sync.Once
	dir string
}

// moduleRootDir returns the directory prefix (with trailing slash) of this
// package's sources, derived from the file that defines buildRouter.
func moduleRootDir() string {
	moduleRootOnce.Do(func() {
		if f := runtime.FuncForPC(reflect.ValueOf(buildRouter).Pointer()); f != nil {
			file, _ := f.FileLine(f.Entry())
			if strings.HasSuffix(file, "main_router.go") {
				moduleRootOnce.dir = strings.TrimSuffix(file, "main_router.go")
			}
		}
	})
	return moduleRootOnce.dir
}

func keyFile(file string) string {
	if root := moduleRootDir(); root != "" && strings.HasPrefix(file, root) {
		return file[len(root):]
	}
	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, d := range bi.Deps {
			if d.Replace != nil {
				continue
			}
			if i := strings.Index(file, d.Path+"@"+d.Version+"/"); i >= 0 {
				return file[i:]
			}
		}
	}
	return file
}

func handlerKeys(ch gin.HandlersChain) []string {
	out := make([]string, len(ch))
	for i, h := range ch {
		out[i] = handlerKey(h)
	}
	return out
}

// patternReachesAuthPaths reports whether a route pattern, as gin matches it, can
// answer SOME request path under /api/v1/auth (including "/api/v1/auth" itself),
// whatever its spelling: the pattern is cleaned the way gin cleans it, then its
// segments are aligned with api / v1 / auth, case-insensitively. A segment is a
// literal, a literal prefix plus ":param" (gin parameters run to the next "/", so
// "aut:x" matches "auth"), or "*catchall" (matches everything after it). Segments
// after the third are free (any request path can continue with them), so every
// pattern whose first three segments are compatible with api/v1/auth, or that has a
// catch-all earlier, counts. This is a property of the pattern alone: it does not
// depend on which method or which other routes exist.
func patternReachesAuthPaths(p string) bool {
	clean := path.Clean("/" + p)
	want := [...]string{"api", "v1", "auth"}
	if clean == "/" {
		return false
	}
	segs := strings.Split(strings.Trim(clean, "/"), "/")
	// Gin routes an EMPTY parameter value (":p" matches "" in "//api/v1/auth"), so a
	// whole-parameter segment may consume zero request segments and shift the
	// alignment of everything after it. match(i, j): pattern segment i against wanted
	// segment j.
	var match func(i, j int) bool
	match = func(i, j int) bool {
		if j == len(want) {
			return true // segments after the third are free
		}
		if i == len(segs) {
			return false
		}
		s := segs[i]
		w := strings.IndexAny(s, ":*")
		switch {
		case w >= 0 && s[w] == '*':
			return true // catch-all: matches the whole rest of the path
		case w == 0:
			return match(i+1, j) || match(i+1, j+1) // empty value, or one real segment
		case w > 0:
			return strings.HasPrefix(want[j], strings.ToLower(s[:w])) && match(i+1, j+1)
		default:
			return strings.EqualFold(s, want[j]) && match(i+1, j+1)
		}
	}
	return match(0, 0)
}

// engineChain reads a HandlersChain field of the engine by name (Handlers,
// allNoRoute, allNoMethod). Reflection with unsafe is the only way gin 1.12 exposes them.
func engineChain(eng any, field string) (gin.HandlersChain, error) {
	f := reflect.ValueOf(eng).Elem().FieldByName(field)
	if !f.IsValid() || f.Type() != reflect.TypeOf(gin.HandlersChain(nil)) {
		return nil, fmt.Errorf("gin Engine.%s not found: gin layout changed", field)
	}
	return *(*gin.HandlersChain)(unsafe.Pointer(f.UnsafeAddr())), nil
}

func sameHandlers(a, b gin.HandlersChain) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if reflect.ValueOf(a[i]).Pointer() != reflect.ValueOf(b[i]).Pointer() {
			return false
		}
	}
	return true
}

// verifyAuthRouteTable returns an error unless, for the finished engine, ALL of:
//   - the engine's path-matching flags are pinned to the values gin.New() sets (every gin.Engine field is classified in TestGinEngineFieldsClassified: no fixed-path redirect,
//     no extra-slash removal, no 405 handling, no raw-path matching, no h2c, no escaped-path
//     matching, trailing slash redirect on, path values unescaped);
//   - the global handler chain is exactly wantGlobalChain (optionally followed by
//     altSvcMiddleware), and the NoRoute and NoMethod chains equal it (so no
//     fallback or global handler can answer an unmatched /api/v1/auth path);
//   - the routes whose PATTERN can match some /api/v1/auth path (see
//     patternReachesAuthPaths) are exactly authRouteTable, for every method.
//
// Scope, stated truthfully: gin's route trees are the only registry; it does not
// see the handler chain of a route (TestBuildRouterAuthSurface pins that for the
// routes buildRouter registers; the AST scan pins every other registration shape in
// package main) nor anything outside gin's engine (a wrapper around the engine in
// http.Server.Handler is refused by the AST scan, not here).
func verifyAuthRouteTable(e *gin.Engine) error {
	if !e.RedirectTrailingSlash || e.RedirectFixedPath || e.HandleMethodNotAllowed || e.UseRawPath || e.UseEscapedPath ||
		e.RemoveExtraSlash || !e.UnescapePathValues || e.UseH2C {
		return fmt.Errorf("engine path-matching flags differ from the pinned values (RedirectTrailingSlash=%t RedirectFixedPath=%t HandleMethodNotAllowed=%t UseRawPath=%t UseEscapedPath=%t RemoveExtraSlash=%t UnescapePathValues=%t UseH2C=%t)",
			e.RedirectTrailingSlash, e.RedirectFixedPath, e.HandleMethodNotAllowed, e.UseRawPath, e.UseEscapedPath, e.RemoveExtraSlash, e.UnescapePathValues, e.UseH2C)
	}
	global, err := engineChain(e, "Handlers")
	if err != nil {
		return err
	}
	gk := handlerKeys(global)
	if !reflect.DeepEqual(gk, wantGlobalChain) && !reflect.DeepEqual(gk, append(append([]string{}, wantGlobalChain...), altSvcKey)) {
		return fmt.Errorf("engine global handler chain %q differs from the pinned chain %q (optionally followed by %s)", gk, wantGlobalChain, altSvcKey)
	}
	for _, field := range []string{"allNoRoute", "allNoMethod"} {
		ch, err := engineChain(e, field)
		if err != nil {
			return err
		}
		if !sameHandlers(ch, global) {
			return fmt.Errorf("engine %s chain differs from the global middleware: a NoRoute/NoMethod handler is installed", field)
		}
	}
	var got []string
	for _, ri := range e.Routes() {
		if patternReachesAuthPaths(ri.Path) {
			got = append(got, ri.Method+" "+ri.Path)
		}
	}
	sort.Strings(got)
	if strings.Join(got, "\n") != strings.Join(authRouteTable, "\n") {
		return fmt.Errorf("auth route table mismatch: got %q, want %q", got, authRouteTable)
	}
	return nil
}
