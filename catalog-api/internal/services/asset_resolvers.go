package services

import (
	"catalogizer/internal/pathguard"
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"catalogizer/database"

	"digital.vasic.assets/pkg/resolver"
)

// CachedFileResolver checks if an asset already exists in the local cover art cache.
type CachedFileResolver struct {
	cacheDir string
	priority int
}

// NewCachedFileResolver creates a resolver that checks a local cache directory.
func NewCachedFileResolver(cacheDir string, priority int) *CachedFileResolver {
	return &CachedFileResolver{cacheDir: cacheDir, priority: priority}
}

func (r *CachedFileResolver) Name() string  { return "cached_file" }
func (r *CachedFileResolver) Priority() int { return r.priority }

func (r *CachedFileResolver) CanResolve(_ context.Context, req *resolver.ResolveRequest) bool {
	if r.cacheDir == "" {
		return false
	}
	path := r.cachePath(req)
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir() && info.Size() > 0
}

func (r *CachedFileResolver) Resolve(_ context.Context, req *resolver.ResolveRequest) (*resolver.ResolveResult, error) {
	path := r.cachePath(req)
	if path == "" {
		return nil, fmt.Errorf("no cache path for asset %s", req.AssetID)
	}

	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat cached file: %w", err)
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open cached file: %w", err)
	}

	ct := "image/jpeg"
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".png":
		ct = "image/png"
	case ".webp":
		ct = "image/webp"
	case ".gif":
		ct = "image/gif"
	}

	return &resolver.ResolveResult{
		Content:     f,
		ContentType: ct,
		Size:        info.Size(),
	}, nil
}

func (r *CachedFileResolver) cachePath(req *resolver.ResolveRequest) string {
	// entity type/id come straight from POST /assets/request: single path elements only
	// (§11.4.115 traversal), and the joined result must still sit under cacheDir.
	if !pathguard.SafeComponent(req.EntityType) || !pathguard.SafeComponent(req.EntityID) {
		return ""
	}
	for _, ext := range []string{".jpg", ".jpeg", ".png", ".webp"} {
		path := filepath.Join(r.cacheDir, req.EntityType, req.EntityID+ext)
		if !pathguard.Within(r.cacheDir, path) {
			return ""
		}
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

// ExternalMetadataResolver looks up cover_url from the file_metadata table
// and fetches it via HTTP.
type ExternalMetadataResolver struct {
	db       *database.DB
	client   *http.Client
	priority int
}

// NewExternalMetadataResolver creates a resolver that reads cover URLs from metadata.
func NewExternalMetadataResolver(db *database.DB, priority int) *ExternalMetadataResolver {
	return &ExternalMetadataResolver{
		db:       db,
		client:   &http.Client{Timeout: 30 * time.Second},
		priority: priority,
	}
}

func (r *ExternalMetadataResolver) Name() string  { return "external_metadata" }
func (r *ExternalMetadataResolver) Priority() int { return r.priority }

func (r *ExternalMetadataResolver) CanResolve(ctx context.Context, req *resolver.ResolveRequest) bool {
	if req.SourceHint != "" && (strings.HasPrefix(req.SourceHint, "http://") || strings.HasPrefix(req.SourceHint, "https://")) {
		return true
	}
	url := r.lookupCoverURL(ctx, req.EntityType, req.EntityID)
	return url != ""
}

func (r *ExternalMetadataResolver) Resolve(ctx context.Context, req *resolver.ResolveRequest) (*resolver.ResolveResult, error) {
	url := req.SourceHint
	if !strings.HasPrefix(url, "http") {
		url = r.lookupCoverURL(ctx, req.EntityType, req.EntityID)
	}
	if url == "" {
		return nil, fmt.Errorf("no cover URL found for %s/%s", req.EntityType, req.EntityID)
	}
	if err := GuardProviderURL(url, SSRFGuardConfig{}); err != nil {
		return nil, fmt.Errorf("external_metadata: unsafe URL: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := r.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("fetch cover: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("fetch cover: HTTP %d", resp.StatusCode)
	}

	return &resolver.ResolveResult{
		Content:     resp.Body,
		ContentType: resp.Header.Get("Content-Type"),
		Size:        resp.ContentLength,
	}, nil
}

func (r *ExternalMetadataResolver) lookupCoverURL(ctx context.Context, entityType, entityID string) string {
	if r.db == nil || entityType != "file" {
		return ""
	}
	var url string
	err := r.db.QueryRowContext(ctx,
		`SELECT value FROM file_metadata WHERE file_id = ? AND key = 'cover_url' LIMIT 1`,
		entityID,
	).Scan(&url)
	if err != nil {
		return ""
	}
	return url
}

// LocalScanResolver scans the media directory for cover images
// (cover.jpg, folder.jpg, etc.).
type LocalScanResolver struct {
	priority int
}

// NewLocalScanResolver creates a resolver that scans directories for cover images.
func NewLocalScanResolver(priority int) *LocalScanResolver {
	return &LocalScanResolver{priority: priority}
}

func (r *LocalScanResolver) Name() string  { return "local_scan" }
func (r *LocalScanResolver) Priority() int { return r.priority }

var coverFilenames = []string{
	"cover.jpg", "cover.jpeg", "cover.png",
	"folder.jpg", "folder.jpeg", "folder.png",
	"poster.jpg", "poster.jpeg", "poster.png",
	"thumb.jpg", "thumb.jpeg", "thumb.png",
	"artwork.jpg", "artwork.jpeg", "artwork.png",
}

func (r *LocalScanResolver) CanResolve(_ context.Context, req *resolver.ResolveRequest) bool {
	if req.SourceHint == "" || !strings.HasPrefix(req.SourceHint, "/") {
		return false
	}
	dir := filepath.Dir(req.SourceHint)
	for _, name := range coverFilenames {
		path := filepath.Join(dir, name)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return true
		}
	}
	return false
}

func (r *LocalScanResolver) Resolve(_ context.Context, req *resolver.ResolveRequest) (*resolver.ResolveResult, error) {
	dir := filepath.Dir(req.SourceHint)
	for _, name := range coverFilenames {
		path := filepath.Join(dir, name)
		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			continue
		}

		f, err := os.Open(path)
		if err != nil {
			continue
		}

		ct := "image/jpeg"
		if strings.HasSuffix(name, ".png") {
			ct = "image/png"
		}

		return &resolver.ResolveResult{
			Content:     f,
			ContentType: ct,
			Size:        info.Size(),
		}, nil
	}

	return nil, fmt.Errorf("no cover image found in %s", dir)
}

// Verify interface compliance at compile time.
var (
	_ resolver.Resolver = (*CachedFileResolver)(nil)
	_ resolver.Resolver = (*ExternalMetadataResolver)(nil)
	_ resolver.Resolver = (*LocalScanResolver)(nil)
)
