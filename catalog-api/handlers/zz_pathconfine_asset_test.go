package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"catalogizer/config"
	"catalogizer/database"
	"catalogizer/internal/services"
	"catalogizer/repository"

	"digital.vasic.assets/pkg/manager"
	"digital.vasic.assets/pkg/resolver"
	"digital.vasic.assets/pkg/store"
	"github.com/gin-gonic/gin"
)

// §11.4.115: POST /assets/request {entity_id:"../../../secret"} drove CachedFileResolver to
// read <anything>.jpg|png|webp outside the cover cache and GET /assets/:id served it back.
func TestPathConfine_AssetEntityIDTraversal(t *testing.T) {
	gin.SetMode(gin.TestMode)
	base := t.TempDir()
	cacheDir := filepath.Join(base, "cache", "cover_art")
	if err := os.MkdirAll(filepath.Join(cacheDir, "file"), 0o755); err != nil {
		t.Fatal(err)
	}
	secret := []byte("SECRET-OUTSIDE-CACHE")
	if err := os.WriteFile(filepath.Join(base, "secret.jpg"), secret, 0o600); err != nil {
		t.Fatal(err)
	}
	// legit cached cover for contrast
	if err := os.WriteFile(filepath.Join(cacheDir, "file", "7.jpg"), []byte("LEGIT-COVER"), 0o600); err != nil {
		t.Fatal(err)
	}

	db, err := database.NewConnection(&config.DatabaseConfig{Path: filepath.Join(t.TempDir(), "t.db")})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.RunMigrations(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	mgr := manager.New(
		manager.WithStore(store.NewMemoryStore()),
		manager.WithResolver(resolver.NewChain(services.NewCachedFileResolver(cacheDir, 1))),
		manager.WithWorkers(1),
	)
	t.Cleanup(mgr.Stop)
	h := NewAssetHandler(mgr, repository.NewAssetRepository(db))
	r := gin.New()
	r.GET("/api/v1/assets/:id", h.ServeAsset)
	r.POST("/api/v1/assets/request", h.RequestAsset)

	fetch := func(entityID string) string {
		b, _ := json.Marshal(map[string]string{"type": "image", "entity_type": "file", "entity_id": entityID})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/assets/request", bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		var resp map[string]interface{}
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		id, _ := resp["asset_id"].(string)
		if id == "" {
			return ""
		}
		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			g := httptest.NewRecorder()
			r.ServeHTTP(g, httptest.NewRequest(http.MethodGet, "/api/v1/assets/"+id, nil))
			if g.Header().Get("X-Asset-Status") == "ready" {
				return g.Body.String()
			}
			time.Sleep(50 * time.Millisecond)
		}
		return ""
	}

	if got := fetch("7"); got != "LEGIT-COVER" {
		t.Fatalf("control: legit cached cover not served (got %q) - test harness invalid", got)
	}
	if got := fetch("../../../secret"); got == string(secret) {
		t.Fatalf("path traversal: served file outside cover cache: %q", got)
	}
}
