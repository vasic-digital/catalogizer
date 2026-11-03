package challenges

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"digital.vasic.challenges/pkg/challenge"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// comprehensiveMockServer creates a mock API server that covers all
// endpoints used by challenge Execute() methods across the entire
// challenge bank. It handles auth, health, metrics, admin, entities,
// playlists, media, search, sync, conversion, collections, and more.
func comprehensiveMockServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	// --- Health and root endpoints ---

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("X-Request-Id", "req-test-001")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "ok",
			"checks": map[string]interface{}{
				"database": "healthy",
				"cache":    "healthy",
			},
		})
	})

	mux.HandleFunc("/api/v1/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok"})
	})

	mux.HandleFunc("/discovery", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=60")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"service": "catalogizer",
		})
	})

	// --- Prometheus metrics ---

	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `# HELP go_goroutines Number of goroutines
# TYPE go_goroutines gauge
go_goroutines 15
# HELP go_gc_duration_seconds GC duration
# TYPE go_gc_duration_seconds summary
go_gc_duration_seconds{quantile="0.5"} 0.0001
# HELP http_request_duration_seconds HTTP request latency
# TYPE http_request_duration_seconds histogram
http_request_duration_seconds_bucket{le="0.1"} 100
# HELP db_query_duration_seconds Database query latency
# TYPE db_query_duration_seconds histogram
db_query_duration_seconds_bucket{le="0.05"} 50
# HELP promhttp_metric_handler_requests_total total
promhttp_metric_handler_requests_total 42
`)
	})

	// --- Auth endpoints ---

	mux.HandleFunc("/api/v1/auth/login", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"session_token": "test-jwt-token",
			"user":          map[string]interface{}{"id": 1, "username": "admin", "role": "admin"},
		})
	})

	mux.HandleFunc("/api/v1/auth/me", func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth == "" || !strings.HasPrefix(auth, "Bearer ") || auth == "Bearer completely-invalid-token-value" || auth == "Bearer non-admin-fake-token" || auth == "Bearer invalid-token-here" {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": "unauthorized"})
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id": 1, "username": "admin", "role": "admin",
		})
	})

	mux.HandleFunc("/api/v1/auth/refresh", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"session_token": "refreshed-jwt-token",
		})
	})

	mux.HandleFunc("/api/v1/auth/change-password", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok"})
	})

	mux.HandleFunc("/api/v1/auth/permissions", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"permissions": []string{"read", "write", "admin"},
		})
	})

	// --- Search endpoints ---

	mux.HandleFunc("/api/v1/search", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"results": []interface{}{},
			"data":    []interface{}{},
			"items":   []interface{}{},
		})
	})

	// --- Entities endpoints ---

	mux.HandleFunc("/api/v1/entities/duplicates", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"duplicates": []interface{}{},
		})
	})

	mux.HandleFunc("/api/v1/entities", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"items": []interface{}{
				map[string]interface{}{
					"id": 1, "title": "Test Movie", "type": "movie",
					"year": 2024, "created_at": "2024-01-01T00:00:00Z",
				},
			},
			"total": 1,
		})
	})

	mux.HandleFunc("/api/v1/entities/1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPut {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id": 1, "title": "Updated Movie",
			})
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id": 1, "title": "Test Movie", "type": "movie",
			"parent_id": nil,
		})
	})

	mux.HandleFunc("/api/v1/entities/1/children", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"items": []interface{}{},
		})
	})

	mux.HandleFunc("/api/v1/entities/1/metadata", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"metadata": map[string]interface{}{},
		})
	})

	mux.HandleFunc("/api/v1/entities/1/user-metadata", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"metadata": map[string]interface{}{},
		})
	})

	mux.HandleFunc("/api/v1/entities/1/files", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"files": []interface{}{},
		})
	})

	// --- Storage roots ---

	mux.HandleFunc("/api/v1/storage-roots", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode([]interface{}{
			map[string]interface{}{"id": 1, "path": "/media", "name": "Media"},
		})
	})

	mux.HandleFunc("/api/v1/storage/roots", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"roots": []interface{}{
				map[string]interface{}{"id": 1, "path": "/media"},
			},
		})
	})

	// --- Files ---

	mux.HandleFunc("/api/v1/files", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"files": []interface{}{},
		})
	})

	// --- Sync endpoints ---

	mux.HandleFunc("/api/v1/sync/endpoints", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusCreated)
		} else {
			w.WriteHeader(http.StatusOK)
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"endpoints": []interface{}{},
		})
	})

	mux.HandleFunc("/api/v1/sync/providers", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"providers": []interface{}{},
		})
	})

	// --- Upload ---

	mux.HandleFunc("/api/v1/upload", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "invalid file format",
		})
	})

	// --- Admin endpoints ---

	mux.HandleFunc("/api/v1/admin/system-info", func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth == "" || auth == "Bearer non-admin-fake-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"version":        "2.0.0",
			"uptime":         "1h30m",
			"uptime_seconds": 5400,
			"go_version":     "1.24",
		})
	})

	mux.HandleFunc("/api/v1/admin/users", func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode([]interface{}{
			map[string]interface{}{"id": 1, "username": "admin"},
		})
	})

	mux.HandleFunc("/api/v1/admin/users/1", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id": 1, "username": "admin",
		})
	})

	mux.HandleFunc("/api/v1/admin/storage", func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"total_space": 1000000, "used_space": 500000,
		})
	})

	mux.HandleFunc("/api/v1/admin/backups", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode([]interface{}{
			map[string]interface{}{"id": 1, "name": "backup-1"},
		})
	})

	mux.HandleFunc("/api/v1/admin/backups/1/restore", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "restoring"})
	})

	mux.HandleFunc("/api/v1/admin/storage/scan", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "scanning"})
	})

	mux.HandleFunc("/api/v1/admin/logs", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"logs": []interface{}{},
		})
	})

	mux.HandleFunc("/api/v1/admin/errors", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"errors": []interface{}{},
		})
	})

	mux.HandleFunc("/api/v1/admin/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "healthy",
		})
	})

	mux.HandleFunc("/api/v1/admin/config", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"config": map[string]interface{}{},
		})
	})

	mux.HandleFunc("/api/v1/admin/challenges", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"challenges": []interface{}{},
		})
	})

	// --- Conversion endpoints ---

	mux.HandleFunc("/api/v1/conversion/jobs", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"jobs": []interface{}{},
		})
	})

	mux.HandleFunc("/api/v1/conversion/formats", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"formats": []string{"mp4", "mkv", "avi"},
		})
	})

	mux.HandleFunc("/api/v1/conversion/submit", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{"job_id": 1})
	})

	mux.HandleFunc("/api/v1/conversion/jobs/1", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id": 1, "status": "completed",
		})
	})

	mux.HandleFunc("/api/v1/conversion/jobs/1/cancel", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "cancelled"})
	})

	mux.HandleFunc("/api/v1/conversion/queue", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"items": []interface{}{},
		})
	})

	mux.HandleFunc("/api/v1/conversion/history", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"items": []interface{}{},
		})
	})

	mux.HandleFunc("/api/v1/conversion/config", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"config": map[string]interface{}{},
		})
	})

	mux.HandleFunc("/api/v1/conversion/stats", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"total": 0, "completed": 0,
		})
	})

	// --- Media endpoints ---

	mux.HandleFunc("/api/v1/media/recent", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"items": []interface{}{},
		})
	})

	mux.HandleFunc("/api/v1/media/popular", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"items": []interface{}{},
		})
	})

	mux.HandleFunc("/api/v1/media/by-path", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"items": []interface{}{},
		})
	})

	mux.HandleFunc("/api/v1/media/analysis", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"results": []interface{}{},
		})
	})

	mux.HandleFunc("/api/v1/media/metadata/refresh", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok"})
	})

	mux.HandleFunc("/api/v1/media/quality", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"quality": map[string]interface{}{},
		})
	})

	mux.HandleFunc("/api/v1/media/detect", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"type": "movie",
		})
	})

	mux.HandleFunc("/api/v1/media/search", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"files": []interface{}{
				map[string]interface{}{"id": 1, "name": "test.mp4"},
			},
		})
	})

	mux.HandleFunc("/api/v1/media/stats", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"total": 100, "by_quality": map[string]interface{}{"hd": 50},
		})
	})

	mux.HandleFunc("/api/v1/media/scan", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "scanning"})
	})

	// --- Playlists ---

	mux.HandleFunc("/api/v1/playlists", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id": 1, "name": "Test Playlist",
			})
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"items": []interface{}{
				map[string]interface{}{"id": 1, "name": "Test Playlist"},
			},
		})
	})

	mux.HandleFunc("/api/v1/playlists/1", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Method == http.MethodPut {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id": 1, "name": "Updated Playlist",
			})
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id": 1, "name": "Test Playlist",
			"items": []interface{}{},
		})
	})

	mux.HandleFunc("/api/v1/playlists/1/items", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok"})
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"items": []interface{}{},
		})
	})

	mux.HandleFunc("/api/v1/playlists/1/items/1", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok"})
	})

	mux.HandleFunc("/api/v1/playlists/1/order", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok"})
	})

	mux.HandleFunc("/api/v1/playlists/public", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"items": []interface{}{},
		})
	})

	mux.HandleFunc("/api/v1/playlists/search", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"items": []interface{}{},
		})
	})

	// --- Collections ---

	mux.HandleFunc("/api/v1/collections", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id": 1, "name": "Test Collection",
			})
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"collections": []interface{}{},
		})
	})

	mux.HandleFunc("/api/v1/collections/1/items", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"items": []interface{}{},
		})
	})

	mux.HandleFunc("/api/v1/collections/search", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"items": []interface{}{},
		})
	})

	// --- Stats ---

	mux.HandleFunc("/api/v1/stats/overall", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"total_files": 100,
		})
	})

	// --- Favorites ---

	mux.HandleFunc("/api/v1/favorites", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"items": []interface{}{},
		})
	})

	// --- Backup ---

	mux.HandleFunc("/api/v1/backups", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]interface{}{"id": 1})
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode([]interface{}{})
	})

	mux.HandleFunc("/api/v1/backups/1/restore", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "restoring"})
	})

	// --- Detection rules and directory analysis ---

	mux.HandleFunc("/api/v1/detection-rules", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"rules": []interface{}{},
		})
	})

	mux.HandleFunc("/api/v1/directory-analysis", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"results": []interface{}{},
		})
	})

	mux.HandleFunc("/api/v1/media-types", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"types": []string{"movie", "tv_show", "music_album"},
		})
	})

	mux.HandleFunc("/api/v1/scan/history", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"history": []interface{}{},
		})
	})

	// --- Catch-all ---

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Default: return 200 with empty JSON for unknown endpoints
		// This ensures challenges don't fail due to missing routes
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []interface{}{}, "items": []interface{}{},
		})
	})

	return httptest.NewServer(mux)
}

// testConfig creates a BrowsingConfig pointing at the given mock server.
func testConfig(serverURL string) *BrowsingConfig {
	return &BrowsingConfig{
		BaseURL:   serverURL,
		Username:  "admin",
		Password:  testAdminPassword,
		WebAppURL: serverURL,
		WebAppDir: "/tmp/catalog-web-test",
	}
}

// ============================================================
// CH-061 to CH-070: Search & Browse Challenges
// ============================================================

func TestSearchAPIBasicQueryChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewSearchAPIBasicQueryChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.Assertions)
}

func TestSearchAPIDuplicateDetectionChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewSearchAPIDuplicateDetectionChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.Assertions)
}

func TestSearchAPIAdvancedFiltersChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewSearchAPIAdvancedFiltersChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.Assertions)
}

func TestBrowseAPIStorageRootsChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewBrowseAPIStorageRootsChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.Assertions)
}

func TestBrowseAPIDirectoryListingChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewBrowseAPIDirectoryListingChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.Assertions)
}

func TestSyncAPIEndpointCreationChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewSyncAPIEndpointCreationChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.Assertions)
}

func TestSyncAPICloudProvidersChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewSyncAPICloudProvidersChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.Assertions)
}

func TestSyncAPIUserEndpointsChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewSyncAPIUserEndpointsChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.Assertions)
}

func TestSecurityHeadersAllChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewSecurityHeadersAllChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, challenge.StatusPassed, result.Status)
}

func TestCORSRejectsUnauthorizedChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewCORSRejectsUnauthorizedChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.Assertions)
}

// ============================================================
// CH-071 to CH-080: Security & Performance Challenges
// ============================================================

func TestInputValidationRejectsInjectionChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewInputValidationRejectsInjectionChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, challenge.StatusPassed, result.Status)
}

func TestRateLimitAuthEndpointsChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewRateLimitAuthEndpointsChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.Assertions)
}

func TestJWTTokenLifecycleChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewJWTTokenLifecycleChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.GreaterOrEqual(t, len(result.Assertions), 3)
}

func TestFileUploadMagicBytesChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewFileUploadMagicBytesChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.Assertions)
}

func TestConversionRejectsPathTraversalChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewConversionRejectsPathTraversalChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, challenge.StatusPassed, result.Status)
}

func TestAPIResponseLatencyChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewAPIResponseLatencyChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.Assertions)
}

func TestAPIConcurrentRequestsChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewAPIConcurrentRequestsChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, challenge.StatusPassed, result.Status)
}

func TestGracefulDegradationChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewGracefulDegradationChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, challenge.StatusPassed, result.Status)
}

func TestMemoryStableDuringLoadChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewMemoryStableDuringLoadChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, challenge.StatusPassed, result.Status)
}

func TestDBPoolRecoveryChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewDBPoolRecoveryChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.Assertions)
}

// ============================================================
// CH-081 to CH-088: WebSocket, Lazy Init, Semaphore, Prometheus
// ============================================================

func TestWebSocketReconnectionChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewWebSocketReconnectionChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
	// Mock httptest server doesn't expose a WebSocket endpoint, so the
	// challenge now honestly returns Skipped with a structured reason
	// instead of a false-positive "passed as stub".
	assert.Equal(t, challenge.StatusSkipped, result.Status)
}

func TestLazyInitOnFirstRequestChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewLazyInitOnFirstRequestChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.Assertions)
}

func TestSemaphorePreventsOverloadChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewSemaphorePreventsOverloadChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, challenge.StatusPassed, result.Status)
}

func TestPrometheusMetricsEndpointChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewPrometheusMetricsEndpointChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, challenge.StatusPassed, result.Status)
}

func TestHTTPRequestMetricsIncrementChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewHTTPRequestMetricsIncrementChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.Assertions)
}

func TestRuntimeMetricsCurrentChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewRuntimeMetricsCurrentChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, challenge.StatusPassed, result.Status)
}

func TestDBQueryDurationTrackedChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewDBQueryDurationTrackedChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.Assertions)
}

func TestGrafanaDashboardRendersChallenge_Execute(t *testing.T) {
	ch := NewGrafanaDashboardRendersChallenge()
	// This challenge checks for local filesystem monitoring directory,
	// so it doesn't need a mock server, just run it.
	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.Assertions)
}

// ============================================================
// CH-094 to CH-098: Provider Verification
// ============================================================

func TestOpenLibraryProviderChallenge_Execute(t *testing.T) {
	ch := NewOpenLibraryProviderChallenge()
	// Provider challenges check source files, no mock server needed.
	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.Assertions)
}

func TestMusicBrainzProviderChallenge_Execute(t *testing.T) {
	ch := NewMusicBrainzProviderChallenge()
	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.Assertions)
}

func TestProviderGracefulDegradationChallenge_Execute(t *testing.T) {
	ch := NewProviderGracefulDegradationChallenge()
	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.Assertions)
}

func TestProviderManagerRoutingChallenge_Execute(t *testing.T) {
	ch := NewProviderManagerRoutingChallenge()
	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.Assertions)
}

func TestLazyServiceRegistryChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewLazyServiceRegistryChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.Assertions)
}

// ============================================================
// CH-100 to CH-110: Admin Challenges
// ============================================================

func TestAdminSystemInfoChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewAdminSystemInfoChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.Assertions)
}

func TestAdminUsersListChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewAdminUsersListChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestAdminUserUpdateChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewAdminUserUpdateChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestAdminStorageInfoChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewAdminStorageInfoChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestAdminBackupsListChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewAdminBackupsListChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestAdminBackupCreateChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewAdminBackupCreateChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestAdminBackupRestoreChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewAdminBackupRestoreChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestAdminStorageScanChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewAdminStorageScanChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestAdminAuthRequiredChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewAdminAuthRequiredChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestAdminNonAdminRejectedChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewAdminNonAdminRejectedChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestAdminSystemInfoVersionChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewAdminSystemInfoVersionChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

// ============================================================
// CH-111 to CH-120: Conversion Challenges
// ============================================================

func TestConversionJobsListChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewConversionJobsListChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestConversionJobCreateChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewConversionJobCreateChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestConversionJobCancelChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewConversionJobCancelChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestConversionJobRetryChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewConversionJobRetryChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestConversionJobDownloadChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewConversionJobDownloadChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestConversionAuthRequiredChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewConversionAuthRequiredChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestConversionJobStatusTransitionChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewConversionJobStatusTransitionChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestConversionFormatsListChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewConversionFormatsListChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestConversionInvalidJobIDChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewConversionInvalidJobIDChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestConversionRateLimitChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewConversionRateLimitChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

// ============================================================
// CH-121 to CH-130: Entity Challenges
// ============================================================

func TestEntityListChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewEntityListChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestEntityDetailChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewEntityDetailChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestEntityTypesChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewEntityTypesChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestEntityStatsChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewEntityStatsChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestEntitySearchByTitleChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewEntitySearchByTitleChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestEntityFilterByTypeChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewEntityFilterByTypeChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestEntityHierarchyCorrectChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewEntityHierarchyCorrectChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestEntityDuplicateDetectionChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewEntityDuplicateDetectionChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestEntityPaginationChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewEntityPaginationChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestEntitySortingChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewEntitySortingChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

// ============================================================
// CH-131 to CH-140: Middleware Challenges
// ============================================================

func TestMiddlewareSecurityHeadersChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewMiddlewareSecurityHeadersChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, challenge.StatusPassed, result.Status)
}

func TestMiddlewareRequestIDChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewMiddlewareRequestIDChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestMiddlewareCacheHeadersHealthChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewMiddlewareCacheHeadersHealthChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestMiddlewareCacheHeadersDiscoveryChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewMiddlewareCacheHeadersDiscoveryChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestMiddlewareRateLimitingChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewMiddlewareRateLimitingChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestMiddlewareRequestTimeoutChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewMiddlewareRequestTimeoutChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestMiddlewareCORSHeadersChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewMiddlewareCORSHeadersChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestMiddlewareCompressionChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewMiddlewareCompressionChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestMiddlewareSQLInjectionChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewMiddlewareSQLInjectionChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestMiddlewareXSSChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewMiddlewareXSSChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

// ============================================================
// CH-141 to CH-152: Phase 1 Endpoint Challenges
// ============================================================

func TestRecentMediaAPIChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewRecentMediaAPIChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestPopularMediaAPIChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewPopularMediaAPIChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestMediaByPathAPIChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewMediaByPathAPIChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestMediaAnalysisChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewMediaAnalysisChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestMetadataRefreshChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewMetadataRefreshChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestMediaQualityChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewMediaQualityChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestChangePasswordChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewChangePasswordChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestBackupCreateChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewBackupCreateChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestBackupListChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewBackupListChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestBackupRestoreChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewBackupRestoreChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestStorageScanAdminChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewStorageScanAdminChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestBackupSemaphoreChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewBackupSemaphoreChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

// ============================================================
// CH-153 to CH-158: Module Integration Challenges
// ============================================================

func TestModuleRegistryInitChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewModuleRegistryInitChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestMemoryMonitorActiveChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewMemoryMonitorActiveChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestHealthAggregatorActiveChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewHealthAggregatorActiveChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestResilienceFacadeActiveChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewResilienceFacadeActiveChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestGuardrailEngineActiveChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewGuardrailEngineActiveChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestStorageResolverActiveChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewStorageResolverActiveChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

// ============================================================
// CH-159 to CH-170: Playlist Challenges
// ============================================================

func TestCreatePlaylistChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewCreatePlaylistChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestListPlaylistsChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewListPlaylistsChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestGetPlaylistChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewGetPlaylistChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestUpdatePlaylistChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewUpdatePlaylistChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestDeletePlaylistChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewDeletePlaylistChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestAddPlaylistItemChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewAddPlaylistItemChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestRemovePlaylistItemChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewRemovePlaylistItemChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestPlaylistOrderingChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewPlaylistOrderingChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestPlaylistAccessControlChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewPlaylistAccessControlChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestPublicPlaylistAccessChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewPublicPlaylistAccessChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestPlaylistPaginationChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewPlaylistPaginationChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestPlaylistSearchChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewPlaylistSearchChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

// ============================================================
// CH-171 to CH-185: Media Analysis Challenges
// ============================================================

func TestMediaTypeDetectionChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewMediaTypeDetectionChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestTVShowHierarchyChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewTVShowHierarchyChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestMusicHierarchyChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewMusicHierarchyChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestDuplicateDetectionChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewDuplicateDetectionChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestEntitySearchAPIChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewEntitySearchAPIChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestEntityPaginationAPIChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewEntityPaginationAPIChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestMediaItemCreateChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewMediaItemCreateChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestMediaItemUpdateChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewMediaItemUpdateChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestMediaFileLinkChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewMediaFileLinkChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestExternalMetadataChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewExternalMetadataChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestUserMetadataAPIChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewUserMetadataAPIChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestCollectionCreateAPIChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewCollectionCreateAPIChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestCollectionItemsAPIChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewCollectionItemsAPIChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestCollectionSearchAPIChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewCollectionSearchAPIChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestDirectoryAnalysisChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewDirectoryAnalysisChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

// ============================================================
// CH-186 to CH-200: Security Challenges
// ============================================================

func TestJWTValidationChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewJWTValidationChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestTokenRefreshSecurityChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewTokenRefreshSecurityChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestRoleBasedAccessSecurityChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewRoleBasedAccessSecurityChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestInputValidationSecurityChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewInputValidationSecurityChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestSQLInjectionSecurityChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewSQLInjectionSecurityChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestPathTraversalSecurityChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewPathTraversalSecurityChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestRateLimitingSecurityChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewRateLimitingSecurityChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestCORSHeadersSecurityChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewCORSHeadersSecurityChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestSecurityHeadersPresentChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewSecurityHeadersPresentChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestPasswordHashingChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewPasswordHashingChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestSessionTimeoutChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewSessionTimeoutChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestBruteForceProtectionChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewBruteForceProtectionChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestContentTypeValidationChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewContentTypeValidationChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestMaxBodySizeChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewMaxBodySizeChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestErrorInfoLeakChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewErrorInfoLeakChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

// ============================================================
// CH-201 to CH-220: Performance Challenges
// ============================================================

func TestHealthEndpointFastChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewHealthEndpointFastChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestSearchLatencyChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewSearchLatencyChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestBrowseLatencyChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewBrowseLatencyChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestEntityDetailLatencyChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewEntityDetailLatencyChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestStatsLatencyChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewStatsLatencyChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestConcurrentReadsChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewConcurrentReadsChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestConcurrentSearchesChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewConcurrentSearchesChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestGoroutineStabilityChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewGoroutineStabilityChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestMemoryStabilityChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewMemoryStabilityChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestConnectionPoolHealthChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewConnectionPoolHealthChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestCacheEffectivenessChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewCacheEffectivenessChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestCompressionActiveChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewCompressionActiveChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestMetricsEndpointPerfChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewMetricsEndpointPerfChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestDeepHealthCheckChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewDeepHealthCheckChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestWebSocketConnectPerfChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewWebSocketConnectPerfChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestWebSocketMessageChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewWebSocketMessageChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestScanQueueingChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewScanQueueingChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestBackupPerformanceChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewBackupPerformanceChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestPaginationPerformanceChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewPaginationPerformanceChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestFilterPerformanceChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewFilterPerformanceChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

// ============================================================
// CH-221 to CH-250: Multiplatform / API Consistency Challenges
// ============================================================

func TestConsistentJSONStructureChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewConsistentJSONStructureChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestConsistentErrorFormatChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewConsistentErrorFormatChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestConsistentPaginationChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewConsistentPaginationChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestConsistentStatusCodesChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewConsistentStatusCodesChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestConsistentContentTypeChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewConsistentContentTypeChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestConsistentAuthRequiredChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewConsistentAuthRequiredChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestConsistentTimestampFormatChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewConsistentTimestampFormatChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestConsistentIDFormatChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewConsistentIDFormatChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestConsistentMethodSupportChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewConsistentMethodSupportChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestConsistentEmptyResponseChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewConsistentEmptyResponseChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestEntityRelationshipsValidChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewEntityRelationshipsValidChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestNoOrphanMediaFilesChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewNoOrphanMediaFilesChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestEntityTypeConsistencyChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewEntityTypeConsistencyChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestCollectionIntegrityChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewCollectionIntegrityChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestStorageRootIntegrityChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewStorageRootIntegrityChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestScanHistoryIntegrityChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewScanHistoryIntegrityChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestUserDataIntegrityChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewUserDataIntegrityChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestFavoritesIntegrityChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewFavoritesIntegrityChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestMediaStatsIntegrityChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewMediaStatsIntegrityChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestDetectionRulesIntegrityChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewDetectionRulesIntegrityChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestAdminSystemInfoAccessChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewAdminSystemInfoAccessChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestAdminUserListAccessChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewAdminUserListAccessChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestAdminStorageOverviewChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewAdminStorageOverviewChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestAdminLogCollectionChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewAdminLogCollectionChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestAdminErrorReportingChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewAdminErrorReportingChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestAdminBackupListMultiplatformChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewAdminBackupListChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestAdminUserUpdateOpsChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewAdminUserUpdateOpsChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestAdminChallengesListChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewAdminChallengesListChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestAdminConfigAccessChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewAdminConfigAccessChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestAdminHealthDashboardChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewAdminHealthDashboardChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

// ============================================================
// CH-251 to CH-401: httpChallenge-based challenges
// ============================================================

func TestDebounceMapBoundedChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewDebounceMapBoundedChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestScanCleanupActiveChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewScanCleanupActiveChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestIPBucketBoundedChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewIPBucketBoundedChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestEventBusUnsubscribeChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewEventBusUnsubscribeChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestMediaQualityRealDataChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewMediaQualityRealDataChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestGovulncheckCleanChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewGovulncheckCleanChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestNpmAuditCleanChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewNpmAuditCleanChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestSemgrepCleanChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewSemgrepCleanChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestSonarQubeGatePassChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewSonarQubeGatePassChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestCustomRulesPassChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewCustomRulesPassChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestGoCoverage95Challenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewGoCoverage95Challenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestAllPackagesDocumentedChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewAllPackagesDocumentedChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestChallengeTestsPassChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewChallengeTestsPassChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestFuzzTests20Challenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewFuzzTests20Challenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestBenchmarks50Challenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewBenchmarks50Challenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestFrontendCoverage90Challenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewFrontendCoverage90Challenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestAllComponentsTestedChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewAllComponentsTestedChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestAccessibilityCleanChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewAccessibilityCleanChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestPrometheusCompleteChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewPrometheusCompleteChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestDiagramsUpdatedChallenge_Execute(t *testing.T) {
	srv := comprehensiveMockServer(t)
	defer srv.Close()

	ch := NewDiagramsUpdatedChallenge()
	ch.config = testConfig(srv.URL)

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}
