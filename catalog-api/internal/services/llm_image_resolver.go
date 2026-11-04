package services

import (
	"bytes"
	"catalogizer/internal/netguard"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"digital.vasic.assets/pkg/resolver"
)

// LLMImageSearchResolver is the last-resort cover-image source. It uses an LLM
// with web-search grounding to locate publicly licensed high-resolution images
// for a given entity. The resolver is disabled by default; set
// CATALOGIZER_LLM_IMAGE_SEARCH_ENABLED=true and CATALOGIZER_LLM_IMAGE_SEARCH_ENDPOINT
// to the orchestrator URL to activate it.
//
// Activation is strictly gated so the resolver never runs in CI, unit tests,
// or fresh installs. It is intended to run only after every standard provider
// has returned no_result.
//
// The resolver enforces SSRF guards on the image URL the LLM returns: private
// IP ranges, loopback, and link-local addresses are rejected before download.
type LLMImageSearchResolver struct {
	priority int
	enabled  bool
	endpoint string
	apiKey   string
	client   *http.Client // operator-configured orchestrator endpoint (may be on the LAN)
	imgs     *http.Client // image downloads: URLs come from the model, strict

	mu           sync.Mutex
	lastAttempts map[string]time.Time // key = entityType|entityID|variant
}

// NewLLMImageSearchResolver returns a configured resolver. Setting
// priority to a value higher than every other resolver ensures the chain
// tries it last.
func NewLLMImageSearchResolver(priority int) *LLMImageSearchResolver {
	endpoint := strings.TrimRight(os.Getenv("CATALOGIZER_LLM_IMAGE_SEARCH_ENDPOINT"), "/")
	enabled := strings.EqualFold(os.Getenv("CATALOGIZER_LLM_IMAGE_SEARCH_ENABLED"), "true") && endpoint != ""
	return &LLMImageSearchResolver{
		priority:     priority,
		enabled:      enabled,
		endpoint:     endpoint,
		apiKey:       os.Getenv("CATALOGIZER_LLM_IMAGE_SEARCH_API_KEY"),
		client:       &http.Client{Timeout: 15 * time.Second},
		imgs:         netguard.StrictClient(15*time.Second, 3, netguard.HTTPOrHTTPS),
		lastAttempts: make(map[string]time.Time),
	}
}

// Name returns the resolver's identifier.
func (r *LLMImageSearchResolver) Name() string { return "llm_image_search" }

// Priority returns the order in the chain; set high so this runs last.
func (r *LLMImageSearchResolver) Priority() int { return r.priority }

// CanResolve reports whether the resolver is activated. It also enforces a
// once-per-entity budget so we never burn LLM quota retrying the same miss.
func (r *LLMImageSearchResolver) CanResolve(_ context.Context, req *resolver.ResolveRequest) bool {
	if !r.enabled || r.endpoint == "" || req == nil {
		return false
	}
	key := req.EntityType + "|" + req.EntityID + "|" + req.Metadata["variant"]
	r.mu.Lock()
	defer r.mu.Unlock()
	if last, ok := r.lastAttempts[key]; ok && time.Since(last) < 24*time.Hour {
		return false
	}
	return true
}

// ErrLLMResolverDisabled is returned when Resolve is called while the resolver
// is not configured. Callers should allow the chain to fall through to the
// placeholder.
var ErrLLMResolverDisabled = errors.New("llm image search resolver disabled")

// Resolve asks the configured LLM endpoint for an image URL, downloads the
// bytes after SSRF screening, and returns them for the gate to assess.
func (r *LLMImageSearchResolver) Resolve(ctx context.Context, req *resolver.ResolveRequest) (*resolver.ResolveResult, error) {
	if !r.enabled {
		return nil, ErrLLMResolverDisabled
	}

	key := req.EntityType + "|" + req.EntityID + "|" + req.Metadata["variant"]
	r.mu.Lock()
	r.lastAttempts[key] = time.Now()
	r.mu.Unlock()

	imageURL, err := r.askLLMForURL(ctx, req)
	if err != nil {
		return nil, err
	}
	if err := allowPublicURL(imageURL); err != nil {
		return nil, fmt.Errorf("llm returned unsafe url: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build image request: %w", err)
	}
	resp, err := r.imgs.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("download image: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download image: HTTP %d", resp.StatusCode)
	}

	buf, err := io.ReadAll(io.LimitReader(resp.Body, 32*1024*1024)) // 32 MB cap
	if err != nil {
		return nil, fmt.Errorf("read image body: %w", err)
	}

	return &resolver.ResolveResult{
		Content:     io.NopCloser(bytes.NewReader(buf)),
		ContentType: resp.Header.Get("Content-Type"),
		Size:        int64(len(buf)),
	}, nil
}

func (r *LLMImageSearchResolver) askLLMForURL(ctx context.Context, req *resolver.ResolveRequest) (string, error) {
	body := strings.NewReader(fmt.Sprintf(
		`{"entity_type":%q,"entity_id":%q,"variant":%q,"hint":%q}`,
		req.EntityType, req.EntityID, req.Metadata["variant"], req.Metadata["title"],
	))
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, r.endpoint+"/v1/cover-search", body)
	if err != nil {
		return "", fmt.Errorf("build orchestrator request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if r.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+r.apiKey)
	}

	resp, err := r.client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("call orchestrator: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("orchestrator: HTTP %d", resp.StatusCode)
	}

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return "", fmt.Errorf("read orchestrator response: %w", err)
	}
	// Extremely narrow parser: expect {"url": "https://..."}
	s := string(raw)
	idx := strings.Index(s, `"url"`)
	if idx < 0 {
		return "", errors.New("orchestrator returned no url")
	}
	// Advance past "url": and find the first ". This is deliberately simple
	// to avoid pulling in a JSON dependency; the orchestrator contract is
	// owned by our own service so the shape is stable.
	rest := s[idx+5:]
	start := strings.Index(rest, `"`)
	if start < 0 {
		return "", errors.New("orchestrator response malformed")
	}
	rest = rest[start+1:]
	end := strings.Index(rest, `"`)
	if end < 0 {
		return "", errors.New("orchestrator response truncated")
	}
	return rest[:end], nil
}

// allowPublicURL rejects URLs that resolve to private, loopback, or
// link-local addresses. This is the basic SSRF defense mandated by the
// security section of the quality-gate design. A full implementation may
// delegate to the Security submodule, but for now we keep it self-contained
// to avoid widening the import graph.
func allowPublicURL(raw string) error {
	return GuardProviderURL(raw, SSRFGuardConfig{})
}

// Verify interface compliance at compile time.
var _ resolver.Resolver = (*LLMImageSearchResolver)(nil)
