package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"catalogizer/database"
	"catalogizer/internal/metrics"
	"catalogizer/internal/services"
	"catalogizer/repository"

	"github.com/gin-gonic/gin"
	_ "github.com/mutecomm/go-sqlcipher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

// PXF23-R5a / PXF23-F4: HTTP-level tests of the admin closures and /health/deep that used to be inline in main.go.
// They use the real handlers, the real image-quality repository and cover-art service over a real migrated SQLite
// database, and real HTTP (httptest); failures are real ones (closed database, absent database), not doubles.

func newAdminRoutesDB(t *testing.T) *database.DB {
	t.Helper()
	sqlDB, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	db := database.WrapDB(sqlDB, database.DialectSQLite)
	require.NoError(t, db.RunMigrations(context.Background()))
	t.Cleanup(func() { sqlDB.Close() })
	return db
}

func doPOST(t *testing.T, h gin.HandlerFunc, body string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/x", h)
	req := httptest.NewRequest(http.MethodPost, "/x", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	return rr
}

func seedStaleAssessments(t *testing.T, db *database.DB, repo *repository.ImageQualityRepository, n int, age time.Duration) {
	t.Helper()
	for i := 0; i < n; i++ {
		require.NoError(t, repo.Upsert(context.Background(), &repository.ImageQualityAssessment{
			EntityType: "media", EntityID: int64(100 + i), Variant: "poster", Source: "tmdb",
			Width: 1000, Height: 1500, BlurVar: 120, BPP: 0.6, AspectRatio: 0.667, Verdict: "pass", Format: "jpeg", CachePath: "/c/x.jpg",
		}))
	}
	_, err := db.Exec(`UPDATE image_quality_assessments SET last_checked_at = ?`, time.Now().Add(-age))
	require.NoError(t, err)
}

func TestAdminRevalidate_SamplesAndTouchesStaleRows(t *testing.T) {
	db := newAdminRoutesDB(t)
	repo := repository.NewImageQualityRepository(db)
	seedStaleAssessments(t, db, repo, 3, 30*24*time.Hour)
	rr := doPOST(t, imageQualityRevalidateHandler(repo, zap.NewNop()), `{"limit":2}`)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	var got map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &got))
	assert.EqualValues(t, 2, got["sampled"], "limit override honoured")
	assert.EqualValues(t, 2, got["touched"])
	assert.EqualValues(t, 7*24*3600, got["stale_age_seconds"], "default stale age")
	assert.EqualValues(t, 2, got["limit"])
	var fresh int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM image_quality_assessments WHERE last_checked_at > ?`, time.Now().Add(-time.Hour)).Scan(&fresh))
	assert.Equal(t, 2, fresh, "exactly the sampled rows had last_checked_at touched")
}

func TestAdminRevalidate_DefaultsAndBounds(t *testing.T) {
	db := newAdminRoutesDB(t)
	repo := repository.NewImageQualityRepository(db)
	for _, tc := range []struct {
		name, body string
		wantLimit  float64
		wantAge    float64
	}{
		{"empty body", ``, 256, 7 * 24 * 3600},
		{"malformed body", `{nope`, 256, 7 * 24 * 3600},
		{"limit above cap ignored", `{"limit":4097}`, 256, 7 * 24 * 3600},
		{"limit at cap kept", `{"limit":4096,"stale_age_seconds":60}`, 4096, 60},
		{"negative ignored", `{"limit":-5,"stale_age_seconds":-1}`, 256, 7 * 24 * 3600},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rr := doPOST(t, imageQualityRevalidateHandler(repo, zap.NewNop()), tc.body)
			require.Equal(t, http.StatusOK, rr.Code)
			var got map[string]any
			require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &got))
			assert.Equal(t, tc.wantLimit, got["limit"])
			assert.Equal(t, tc.wantAge, got["stale_age_seconds"])
		})
	}
}

// A real repository failure (the database is closed) answers the fixed 500 body: the driver text never reaches the client,
// and it is logged server-side.
func TestAdminRevalidate_StoreFailureFixedBodyAndLogged(t *testing.T) {
	db := newAdminRoutesDB(t)
	repo := repository.NewImageQualityRepository(db)
	require.NoError(t, db.Close())
	core, logs := observer.New(zap.WarnLevel)
	rr := doPOST(t, imageQualityRevalidateHandler(repo, zap.New(core)), `{}`)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.JSONEq(t, `{"error":"Failed to sample images for revalidation"}`, rr.Body.String())
	for _, bad := range []string{"closed", "sample for revalidation", "sql", "image_quality"} {
		assert.NotContains(t, strings.ToLower(rr.Body.String()), bad)
	}
	require.Equal(t, 1, logs.Len())
	assert.Contains(t, logs.All()[0].ContextMap()["error"], "sample for revalidation", "the real error is logged server-side")
}

// A real service failure (no database) answers the fixed 500 body and logs the cause.
func TestAdminThumbnails_ServiceFailureFixedBodyAndLogged(t *testing.T) {
	core, logs := observer.New(zap.WarnLevel)
	lg := zap.New(core)
	svc := services.NewCoverArtService(nil, lg)
	rr := doPOST(t, generateThumbnailsHandler(svc, lg), `{"limit":5}`)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.JSONEq(t, `{"error":"Failed to generate video thumbnails"}`, rr.Body.String())
	assert.NotContains(t, rr.Body.String(), "no database connection")
	require.Equal(t, 1, logs.Len())
	assert.Contains(t, logs.All()[0].ContextMap()["error"], "no database connection")
}

func TestAdminThumbnails_SuccessWithEmptyCatalogue(t *testing.T) {
	db := newAdminRoutesDB(t)
	svc := services.NewCoverArtService(db, zap.NewNop())
	for _, body := range []string{``, `{"limit":10}`, `{"limit":9999}`} { // empty and invalid limits fall back to 50
		rr := doPOST(t, generateThumbnailsHandler(svc, zap.NewNop()), body)
		require.Equal(t, http.StatusOK, rr.Code, "%q: %s", body, rr.Body.String())
		assert.JSONEq(t, `{"generated":0,"message":"thumbnail generation completed"}`, rr.Body.String())
	}
}

type recordingThumbs struct{ gotLimit int }

func (r *recordingThumbs) GenerateMissingVideoThumbnails(_ context.Context, limit int) (int, error) {
	r.gotLimit = limit
	return 3, nil
}

// The limit reaching the generator: body limit when valid (1..500), default 50 otherwise (unit-level double: the generator is
// an interface, the HTTP path is real).
func TestAdminThumbnails_LimitPassedToGenerator(t *testing.T) {
	for body, want := range map[string]int{``: 50, `{"limit":7}`: 7, `{"limit":500}`: 500, `{"limit":501}`: 50, `{"limit":0}`: 50} {
		rec := &recordingThumbs{}
		rr := doPOST(t, generateThumbnailsHandler(rec, zap.NewNop()), body)
		require.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, want, rec.gotLimit, "body %q", body)
		assert.JSONEq(t, `{"generated":3,"message":"thumbnail generation completed"}`, rr.Body.String())
	}
}

func getDeep(t *testing.T, h gin.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/health/deep", h)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/health/deep", nil))
	return rr
}

func TestDeepHealth_HealthyDatabase200(t *testing.T) {
	sqlDB, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	defer sqlDB.Close()
	sqlDB.SetMaxOpenConns(10)
	hc := metrics.NewHealthChecker(database.WrapDB(sqlDB, database.DialectSQLite), "9.9.9")
	rr := getDeep(t, deepHealthHandler(hc, "9.9.9", 2*time.Second))
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	var got metrics.HealthCheckResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &got))
	assert.Equal(t, metrics.HealthStatusHealthy, got.Status)
	assert.Equal(t, "9.9.9", got.Version)
	assert.Equal(t, metrics.HealthStatusHealthy, got.Components["database"].Status)
}

// An unreachable database is 503 with the fixed component message; the driver error text is not in the body.
func TestDeepHealth_UnhealthyDatabase503NoDriverText(t *testing.T) {
	sqlDB, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
	hc := metrics.NewHealthChecker(database.WrapDB(sqlDB, database.DialectSQLite), "1.0.0")
	rr := getDeep(t, deepHealthHandler(hc, "1.0.0", 2*time.Second))
	require.Equal(t, http.StatusServiceUnavailable, rr.Code, rr.Body.String())
	var got metrics.HealthCheckResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &got))
	assert.Equal(t, metrics.HealthStatusUnhealthy, got.Status)
	assert.Equal(t, "Database ping failed", got.Components["database"].Message)
	assert.NotContains(t, rr.Body.String(), "closed")
	assert.NotContains(t, rr.Body.String(), "sql:")

	// no database at all: the fixed "not configured" message
	rr = getDeep(t, deepHealthHandler(metrics.NewHealthChecker(nil, "1.0.0"), "1.0.0", 2*time.Second))
	assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
	assert.Contains(t, rr.Body.String(), "Database not configured")
}

// A check slower than the timeout answers 200 "degraded" with the fixed timeout component, carrying the build version.
func TestDeepHealth_SlowCheckDegraded200(t *testing.T) {
	hc := metrics.NewHealthChecker(nil, "1.0.0")
	hc.RegisterCheck("database", func(ctx context.Context) metrics.ComponentHealth {
		time.Sleep(400 * time.Millisecond)
		return metrics.ComponentHealth{Status: metrics.HealthStatusHealthy}
	})
	start := time.Now()
	rr := getDeep(t, deepHealthHandler(hc, "7.7.7", 100*time.Millisecond))
	assert.Less(t, time.Since(start), 350*time.Millisecond, "the route returns at the timeout, not at the slow check")
	require.Equal(t, http.StatusOK, rr.Code)
	var got map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &got))
	assert.Equal(t, "degraded", got["status"])
	assert.Equal(t, "7.7.7", got["version"])
	assert.Equal(t, "health check exceeded 100ms timeout", got["message"])
	comp := got["components"].(map[string]any)["timeout"].(map[string]any)
	assert.Equal(t, "degraded", comp["status"])
	assert.Equal(t, "deep health check took too long", comp["message"])
}

// main.go mounts the three routes through these constructors with the production wiring (pin against a return of the
// untestable inline closures).
func TestMainGoMountsAdminAndDeepHealthRoutesThroughHandlers(t *testing.T) {
	// /health/deep is registered by buildRouter (main_router.go), the admin routes
	// by main(): the pin reads both files (review round 2, F2).
	var s string
	for _, f := range []string{"main.go", "main_router.go"} {
		src, err := os.ReadFile(f)
		require.NoError(t, err)
		s += string(src)
	}
	assert.Contains(t, s, `router.GET("/health/deep", root_middleware.CacheHeaders(5), deepHealthHandler(deepHealthChecker, Version, 100*time.Millisecond))`)
	assert.Contains(t, s, `api.POST("/admin/image-quality/revalidate", jwtMiddleware.RequireAdmin(), root_middleware.RateLimiter(6), imageQualityRevalidateHandler(imageQualityRepo, logger))`)
	assert.Contains(t, s, `api.POST("/admin/covers/generate-thumbnails", jwtMiddleware.RequireAdmin(), generateThumbnailsHandler(coverArtService, logger))`)
	assert.NotContains(t, s, "SampleForRevalidation(", "the revalidation logic lives in the tested handler")
	assert.NotContains(t, s, "GenerateMissingVideoThumbnails(", "the thumbnail logic lives in the tested handler")
}

// F5 (review round 1): the timeout message reports the configured deepTimeout, not a hard-coded 100ms.
func TestDeepHealth_TimeoutMessageUsesConfiguredTimeout(t *testing.T) {
	hc := metrics.NewHealthChecker(nil, "1.0.0")
	hc.RegisterCheck("database", func(ctx context.Context) metrics.ComponentHealth {
		time.Sleep(300 * time.Millisecond)
		return metrics.ComponentHealth{Status: metrics.HealthStatusHealthy}
	})
	rr := getDeep(t, deepHealthHandler(hc, "1.0.0", 40*time.Millisecond))
	var got map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &got))
	assert.Equal(t, "health check exceeded 40ms timeout", got["message"])
}
