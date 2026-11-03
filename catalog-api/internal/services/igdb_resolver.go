package services

import (
	"bytes"
	"catalogizer/internal/netguard"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"digital.vasic.assets/pkg/resolver"
)

// IGDBResolver resolves game cover art from the Internet Game Database
// via Twitch's IGDB API v4. The resolver activates when both
// IGDB_CLIENT_ID and IGDB_APP_ACCESS_TOKEN env vars are set.
//
// Fixes B2 from docs/nexus/remaining-work.md: IGDB v4 requires the
// Client-ID header and Authorization: Bearer token. The older
// Henry-Sarabia/igdb v1 library only sends the legacy v3 "user-key"
// header, so every request against the current API returns 401. This
// implementation talks raw HTTP + Apicalypse queries so the dependency
// surface stays tight.
//
// Priority 22 — after existing providers, before the LLM last-resort.
type IGDBResolver struct {
	clientID string
	token    string
	priority int
	http     *http.Client // IGDB API (vendor base URL)
	dl       *http.Client // image downloads: URLs come from provider data
	baseURL  string
}

// NewIGDBResolver returns a resolver bound to Twitch credentials. If
// either env var is missing the resolver remains inert so fresh
// installs never call IGDB without explicit configuration.
func NewIGDBResolver(priority int) *IGDBResolver {
	return &IGDBResolver{
		clientID: strings.TrimSpace(envFirstNonEmpty("IGDB_CLIENT_ID")),
		token:    strings.TrimSpace(envFirstNonEmpty("IGDB_APP_ACCESS_TOKEN", "IGDB_BEARER_TOKEN")),
		priority: priority,
		http:     &http.Client{Timeout: 10 * time.Second},
		dl:       netguard.StrictClient(10*time.Second, 3, netguard.HTTPOrHTTPS),
		baseURL:  "https://api.igdb.com/v4",
	}
}

// Name returns the resolver identifier.
func (r *IGDBResolver) Name() string { return "igdb" }

// Priority returns the chain priority.
func (r *IGDBResolver) Priority() int { return r.priority }

// CanResolve reports whether the resolver is configured + the request
// carries a game / software entity.
func (r *IGDBResolver) CanResolve(_ context.Context, req *resolver.ResolveRequest) bool {
	if r.clientID == "" || r.token == "" {
		return false
	}
	if req == nil {
		return false
	}
	switch strings.ToLower(req.EntityType) {
	case "game", "software":
		return true
	}
	return false
}

// Resolve queries IGDB v4 for a cover matching the request and
// downloads the high-resolution image after SSRF screening.
func (r *IGDBResolver) Resolve(ctx context.Context, req *resolver.ResolveRequest) (*resolver.ResolveResult, error) {
	if r.clientID == "" || r.token == "" {
		return nil, errors.New("igdb: resolver not configured")
	}
	gameID, err := r.resolveGameID(ctx, req)
	if err != nil {
		return nil, err
	}
	url, err := r.coverURL(ctx, gameID)
	if err != nil {
		return nil, err
	}
	if err := allowPublicURLLocal(url); err != nil {
		return nil, fmt.Errorf("igdb: unsafe URL: %w", err)
	}
	return r.download(ctx, url)
}

// post sends an Apicalypse query against the named IGDB endpoint
// and returns the raw JSON reply. IGDB v4 requires Client-ID +
// Bearer auth headers on every call.
func (r *IGDBResolver) post(ctx context.Context, endpoint, apicalypse string) ([]byte, error) {
	target := r.baseURL + "/" + endpoint
	if err := GuardProviderURL(target, SSRFGuardConfig{}); err != nil {
		return nil, fmt.Errorf("igdb: unsafe endpoint: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, strings.NewReader(apicalypse))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "text/plain")
	req.Header.Set("Client-ID", r.clientID)
	req.Header.Set("Authorization", "Bearer "+r.token)

	resp, err := r.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("igdb %s: %w", endpoint, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("igdb %s: HTTP %d: %s", endpoint, resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	return raw, nil
}

func (r *IGDBResolver) resolveGameID(ctx context.Context, req *resolver.ResolveRequest) (int, error) {
	if raw := req.Metadata["igdb_id"]; raw != "" {
		id, err := strconv.Atoi(raw)
		if err == nil && id > 0 {
			return id, nil
		}
	}
	title := req.Metadata["title"]
	if title == "" {
		return 0, errors.New("igdb: need igdb_id or title metadata")
	}
	q := fmt.Sprintf(`search "%s"; fields id,name; limit 1;`, escapeApicalypse(title))
	raw, err := r.post(ctx, "games", q)
	if err != nil {
		return 0, err
	}
	var rows []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(raw, &rows); err != nil {
		return 0, fmt.Errorf("igdb games decode: %w", err)
	}
	if len(rows) == 0 {
		return 0, errors.New("igdb: no matches")
	}
	return rows[0].ID, nil
}

func (r *IGDBResolver) coverURL(ctx context.Context, gameID int) (string, error) {
	q := fmt.Sprintf(`fields url,image_id; where game = %d; limit 1;`, gameID)
	raw, err := r.post(ctx, "covers", q)
	if err != nil {
		return "", err
	}
	var rows []struct {
		URL     string `json:"url"`
		ImageID string `json:"image_id"`
	}
	if err := json.Unmarshal(raw, &rows); err != nil {
		return "", fmt.Errorf("igdb covers decode: %w", err)
	}
	if len(rows) == 0 {
		return "", errors.New("igdb: no covers")
	}
	// Prefer the image_id path because it supports explicit sizes.
	// The URL field is always protocol-relative and fixed at t_thumb.
	if rows[0].ImageID != "" {
		return fmt.Sprintf("https://images.igdb.com/igdb/image/upload/t_cover_big/%s.jpg", rows[0].ImageID), nil
	}
	return normalizeIGDBImageURL(rows[0].URL), nil
}

// normalizeIGDBImageURL promotes an IGDB protocol-relative URL to
// https and swaps t_thumb for t_cover_big for higher-resolution art.
// Retained for callers that pass legacy cached URLs.
func normalizeIGDBImageURL(raw string) string {
	if strings.HasPrefix(raw, "//") {
		raw = "https:" + raw
	}
	raw = strings.Replace(raw, "t_thumb", "t_cover_big", 1)
	return raw
}

func (r *IGDBResolver) download(ctx context.Context, url string) (*resolver.ResolveResult, error) {
	if err := GuardProviderURL(url, SSRFGuardConfig{}); err != nil {
		return nil, fmt.Errorf("igdb: unsafe download URL: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := r.dl.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("igdb download: HTTP %d", resp.StatusCode)
	}
	return &resolver.ResolveResult{
		Content:     resp.Body,
		ContentType: resp.Header.Get("Content-Type"),
		Size:        resp.ContentLength,
	}, nil
}

// escapeApicalypse quotes a title string for the Apicalypse search
// clause. Backslash and double-quote are the only characters that
// carry syntactic meaning inside the literal.
func escapeApicalypse(s string) string {
	var b bytes.Buffer
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
