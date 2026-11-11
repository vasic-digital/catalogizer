package services

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"digital.vasic.assets/pkg/resolver"

	"github.com/pborman/uuid"
	caa "gopkg.in/mineo/gocaa.v1"
)

// CoverArtArchiveResolver resolves album / artist art from the
// MusicBrainz Cover Art Archive. The resolver requires a MusicBrainz
// release UUID stored in req.Metadata["musicbrainz_release_id"] —
// without one the archive cannot be queried.
//
// Priority 21 — after the existing metadata resolver, before IGDB.
type CoverArtArchiveResolver struct {
	priority int
	userAgent string
	http     *http.Client
	client   *caa.CAAClient
}

// NewCoverArtArchiveResolver returns a resolver using the shared
// Cover Art Archive service. A User-Agent is required by MusicBrainz's
// rate-limiting policy; the resolver falls back to a sensible default
// if the CAA_USER_AGENT env var is unset.
func NewCoverArtArchiveResolver(priority int) *CoverArtArchiveResolver {
	ua := strings.TrimSpace(envFirstNonEmpty("CAA_USER_AGENT", "MUSICBRAINZ_USER_AGENT"))
	if ua == "" {
		ua = "Catalogizer/nexus (+https://catalogizer.vasic.digital)"
	}
	client := caa.NewCAAClient(ua)
	return &CoverArtArchiveResolver{
		priority:  priority,
		userAgent: ua,
		http:      &http.Client{Timeout: 10 * time.Second},
		client:    client,
	}
}

// Name returns the resolver identifier.
func (r *CoverArtArchiveResolver) Name() string { return "cover_art_archive" }

// Priority returns the chain priority.
func (r *CoverArtArchiveResolver) Priority() int { return r.priority }

// CanResolve reports whether we have a MusicBrainz release id and the
// entity is a music artifact.
func (r *CoverArtArchiveResolver) CanResolve(_ context.Context, req *resolver.ResolveRequest) bool {
	if req == nil || req.Metadata == nil {
		return false
	}
	if req.Metadata["musicbrainz_release_id"] == "" && req.Metadata["musicbrainz_release_group_id"] == "" {
		return false
	}
	switch strings.ToLower(req.EntityType) {
	case "music_album", "song", "music_artist":
		return true
	}
	return false
}

// Resolve fetches the "front" image of the release at 500 px width.
func (r *CoverArtArchiveResolver) Resolve(ctx context.Context, req *resolver.ResolveRequest) (*resolver.ResolveResult, error) {
	_ = ctx
	raw := firstNonEmpty(
		req.Metadata["musicbrainz_release_id"],
		req.Metadata["musicbrainz_release_group_id"],
	)
	if raw == "" {
		return nil, errors.New("cover art archive: no musicbrainz release id")
	}
	id := uuid.Parse(raw)
	if id == nil {
		return nil, fmt.Errorf("cover art archive: invalid UUID %q", raw)
	}
	img, err := r.client.GetReleaseFront(id, caa.ImageSize500)
	if err != nil {
		return nil, fmt.Errorf("cover art archive fetch: %w", err)
	}
	if len(img.Data) == 0 {
		return nil, errors.New("cover art archive: empty image payload")
	}
	contentType := img.Mimetype
	if contentType == "" {
		contentType = "image/jpeg"
	}
	return &resolver.ResolveResult{
		Content:     io.NopCloser(bytes.NewReader(img.Data)),
		ContentType: contentType,
		Size:        int64(len(img.Data)),
	}, nil
}

// firstNonEmpty is a small helper used by the CAA lookup to pick the
// release id first, falling back to the release-group id. We keep it
// local to this file so the services package does not grow yet
// another utility namespace.
func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if v := strings.TrimSpace(s); v != "" {
			return v
		}
	}
	return ""
}

// The http.Client field is currently unused but preserved for
// consistency with the sibling resolvers; MusicBrainz's gocaa library
// handles its own transport.
var _ = http.StatusOK
