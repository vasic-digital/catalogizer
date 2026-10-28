package services

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"catalogizer/internal/pathguard"
	"digital.vasic.assets/pkg/asset"
	"digital.vasic.assets/pkg/resolver"
)

func TestCachedFileResolver_CanResolve(t *testing.T) {
	dir := t.TempDir()
	r := NewCachedFileResolver(dir, 1)

	// No match
	req := &resolver.ResolveRequest{
		AssetID:    asset.NewID(),
		EntityType: "file",
		EntityID:   "99",
	}
	if r.CanResolve(context.Background(), req) {
		t.Error("should not resolve when no file exists")
	}

	// Create a cached file
	entityDir := filepath.Join(dir, "file")
	os.MkdirAll(entityDir, 0755)
	os.WriteFile(filepath.Join(entityDir, "42.jpg"), []byte("cached-image"), 0644)

	req.EntityID = "42"
	if !r.CanResolve(context.Background(), req) {
		t.Error("should resolve when cached file exists")
	}
}

func TestCachedFileResolver_Resolve(t *testing.T) {
	dir := t.TempDir()
	r := NewCachedFileResolver(dir, 1)

	entityDir := filepath.Join(dir, "file")
	os.MkdirAll(entityDir, 0755)
	content := []byte("cached-cover-art")
	os.WriteFile(filepath.Join(entityDir, "42.jpg"), content, 0644)

	req := &resolver.ResolveRequest{
		AssetID:    asset.NewID(),
		EntityType: "file",
		EntityID:   "42",
	}

	result, err := r.Resolve(context.Background(), req)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	defer result.Content.Close()

	data, _ := io.ReadAll(result.Content)
	if string(data) != "cached-cover-art" {
		t.Errorf("expected 'cached-cover-art', got %q", string(data))
	}
	if result.ContentType != "image/jpeg" {
		t.Errorf("expected image/jpeg, got %s", result.ContentType)
	}
}

func TestExternalMetadataResolver_CanResolve_WithSourceHint(t *testing.T) {
	r := NewExternalMetadataResolver(nil, 2)

	// With HTTP source hint
	req := &resolver.ResolveRequest{
		SourceHint: "https://image.tmdb.org/t/p/w500/poster.jpg",
	}
	if !r.CanResolve(context.Background(), req) {
		t.Error("should resolve when source_hint is an HTTP URL")
	}

	// Without source hint
	req2 := &resolver.ResolveRequest{}
	if r.CanResolve(context.Background(), req2) {
		t.Error("should not resolve without source_hint and db")
	}
}

func TestExternalMetadataResolver_Resolve(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.Write([]byte("tmdb-poster"))
	}))
	defer server.Close()

	r := NewExternalMetadataResolver(nil, 2)

	req := &resolver.ResolveRequest{
		AssetID:    asset.NewID(),
		SourceHint: server.URL + "/poster.jpg",
		EntityType: "file",
		EntityID:   "1",
	}

	result, err := r.Resolve(context.Background(), req)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	defer result.Content.Close()

	data, _ := io.ReadAll(result.Content)
	if string(data) != "tmdb-poster" {
		t.Errorf("expected 'tmdb-poster', got %q", string(data))
	}
}

func TestLocalScanResolver_CanResolve(t *testing.T) {
	r := NewLocalScanResolver(4)
	dir := t.TempDir()
	t.Setenv(pathguard.EnvMediaRoot, "")
	t.Setenv(pathguard.EnvReadRoots, dir)

	// No source hint
	if r.CanResolve(context.Background(), &resolver.ResolveRequest{}) {
		t.Error("should not resolve without source hint")
	}

	// Non-local path
	if r.CanResolve(context.Background(), &resolver.ResolveRequest{SourceHint: "http://example.com"}) {
		t.Error("should not resolve HTTP URLs")
	}

	// Local path with cover.jpg
	mediaFile := filepath.Join(dir, "movie.mkv")
	os.WriteFile(filepath.Join(dir, "cover.jpg"), []byte("cover"), 0644)

	if !r.CanResolve(context.Background(), &resolver.ResolveRequest{SourceHint: mediaFile}) {
		t.Error("should resolve when cover.jpg exists in same directory")
	}
}

func TestLocalScanResolver_Resolve(t *testing.T) {
	r := NewLocalScanResolver(4)
	dir := t.TempDir()
	t.Setenv(pathguard.EnvMediaRoot, "")
	t.Setenv(pathguard.EnvReadRoots, dir)

	coverContent := []byte("folder-cover-art")
	os.WriteFile(filepath.Join(dir, "folder.jpg"), coverContent, 0644)

	req := &resolver.ResolveRequest{
		AssetID:    asset.NewID(),
		SourceHint: filepath.Join(dir, "movie.mkv"),
	}

	result, err := r.Resolve(context.Background(), req)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	defer result.Content.Close()

	data, _ := io.ReadAll(result.Content)
	if string(data) != "folder-cover-art" {
		t.Errorf("expected 'folder-cover-art', got %q", string(data))
	}
}
