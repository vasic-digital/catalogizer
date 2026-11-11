package services

import (
	"context"
	"strings"
	"testing"

	"digital.vasic.assets/pkg/resolver"
)

// TestFanartTVResolver_B1_RequiresIMDBForMovies locks in B1 so the bug
// never regresses: Fanart.tv's movie endpoint expects an IMDB id, not
// a TMDB id. The resolver must stay inert when only tmdb_id is
// available.
func TestFanartTVResolver_B1_RequiresIMDBForMovies(t *testing.T) {
	t.Setenv("FANART_TV_API_KEY", "test-key")
	r := NewFanartTVResolver(11)

	// tmdb_id alone must NOT trigger Fanart.tv — the old bug used to
	// call GetMovieImages with a tmdb id and then 404.
	req := &resolver.ResolveRequest{
		EntityType: "movie",
		Metadata:   map[string]string{"tmdb_id": "603"},
	}
	if r.CanResolve(context.Background(), req) {
		t.Error("B1 regression: tmdb_id alone must not trigger Fanart.tv")
	}

	// imdb_id activates the resolver.
	req.Metadata["imdb_id"] = "tt0133093"
	if !r.CanResolve(context.Background(), req) {
		t.Error("imdb_id must activate Fanart.tv for movies")
	}

	// TV needs tvdb_id, not imdb_id.
	tvReq := &resolver.ResolveRequest{
		EntityType: "tv_show",
		Metadata:   map[string]string{"imdb_id": "tt0903747"},
	}
	if r.CanResolve(context.Background(), tvReq) {
		t.Error("B1 regression: imdb_id must not trigger TV endpoint")
	}
	tvReq.Metadata["tvdb_id"] = "81189"
	if !r.CanResolve(context.Background(), tvReq) {
		t.Error("tvdb_id must activate Fanart.tv for TV")
	}
}

// TestFanartTVResolver_ProviderIDPicker covers the field selection
// helper end-to-end. It guards against subtle regressions where the
// entity_type switch might lose a case.
func TestFanartTVResolver_ProviderIDPicker(t *testing.T) {
	t.Setenv("FANART_TV_API_KEY", "key")
	r := NewFanartTVResolver(11)

	cases := []struct {
		entityType string
		metadata   map[string]string
		want       string
	}{
		{"movie", map[string]string{"imdb_id": "tt1"}, "tt1"},
		{"movies", map[string]string{"imdb_id": "tt2"}, "tt2"},
		{"tv_show", map[string]string{"tvdb_id": "42"}, "42"},
		{"tv_season", map[string]string{"tvdb_id": "43"}, "43"},
		{"tv_episode", map[string]string{"tvdb_id": "44"}, "44"},
		{"book", map[string]string{"imdb_id": "tt3"}, ""},
		{"movie", nil, ""},
	}
	for _, c := range cases {
		req := &resolver.ResolveRequest{EntityType: c.entityType, Metadata: c.metadata}
		if got := r.providerID(req); got != c.want {
			t.Errorf("%s + %v = %q, want %q", c.entityType, c.metadata, got, c.want)
		}
	}
}

// TestFanartTVResolver_ResolveRejectsMissingID locks in the explicit
// error when metadata carries neither imdb_id nor tvdb_id. A silent
// 404 would confuse operators.
func TestFanartTVResolver_ResolveRejectsMissingID(t *testing.T) {
	t.Setenv("FANART_TV_API_KEY", "key")
	r := NewFanartTVResolver(11)
	_, err := r.Resolve(context.Background(), &resolver.ResolveRequest{
		EntityType: "movie",
		Metadata:   map[string]string{"tmdb_id": "603"},
	})
	if err == nil || !strings.Contains(err.Error(), "platform-appropriate") {
		t.Errorf("expected platform-appropriate id error, got %v", err)
	}
}

// TestIGDBResolver_B2_HeadersSentOnV4 proves the IGDB v4 fix: both
// Client-ID and Authorization: Bearer headers land on every request.
// We wire a tiny stub server as the IGDB base URL, intercept the call,
// and assert the headers are present.
func TestIGDBResolver_B2_HeadersSentOnV4(t *testing.T) {
	t.Setenv("IGDB_CLIENT_ID", "client-123")
	t.Setenv("IGDB_APP_ACCESS_TOKEN", "secret-token")
	r := NewIGDBResolver(22)

	captured := make(chan struct {
		clientID string
		auth     string
		body     string
	}, 4)

	srv := newStubServer(func(clientID, auth, body string) {
		captured <- struct {
			clientID string
			auth     string
			body     string
		}{clientID, auth, body}
	})
	defer srv.Close()
	r.baseURL = srv.URL
	r.http = srv.Client()

	// Query path: resolve game by title -> resolve cover -> download
	// the URL. Our stub returns a canned game id + cover image id then
	// serves the jpg as a no-op.
	_, _ = r.Resolve(context.Background(), &resolver.ResolveRequest{
		EntityType: "game",
		Metadata:   map[string]string{"title": "Portal"},
	})

	seen := 0
	for {
		select {
		case h := <-captured:
			seen++
			if h.clientID != "client-123" {
				t.Errorf("Client-ID = %q", h.clientID)
			}
			if h.auth != "Bearer secret-token" {
				t.Errorf("Authorization = %q", h.auth)
			}
			if h.body == "" {
				t.Error("empty Apicalypse body")
			}
		default:
			if seen < 2 {
				t.Errorf("captured only %d requests, want at least 2 (games + covers)", seen)
			}
			return
		}
	}
}

// TestEscapeApicalypse locks in the escape behaviour so future edits
// do not silently strip backslashes or quotes from search queries.
func TestEscapeApicalypse(t *testing.T) {
	cases := map[string]string{
		`Portal`:             `Portal`,
		`"Portal" 2`:         `\"Portal\" 2`,
		`path\to\game`:       `path\\to\\game`,
		`\"weird\"`:          `\\\"weird\\\"`,
	}
	for in, want := range cases {
		if got := escapeApicalypse(in); got != want {
			t.Errorf("escapeApicalypse(%q) = %q, want %q", in, got, want)
		}
	}
}
