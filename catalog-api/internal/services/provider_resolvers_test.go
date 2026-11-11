package services

import (
	"context"
	"testing"

	"digital.vasic.assets/pkg/resolver"
)

func TestFanartTVResolver_InertWithoutKey(t *testing.T) {
	t.Setenv("FANART_TV_API_KEY", "")
	r := NewFanartTVResolver(11)
	if r.Name() != "fanarttv" {
		t.Errorf("name = %q", r.Name())
	}
	if r.Priority() != 11 {
		t.Errorf("priority = %d", r.Priority())
	}
	req := &resolver.ResolveRequest{EntityType: "movie", Metadata: map[string]string{"tmdb_id": "123"}}
	if r.CanResolve(context.Background(), req) {
		t.Error("resolver must not activate without FANART_TV_API_KEY")
	}
}

func TestFanartTVResolver_ActivatesWithKey(t *testing.T) {
	t.Setenv("FANART_TV_API_KEY", "test-key")
	r := NewFanartTVResolver(11)
	// Post-B1 fix: Fanart.tv movies require imdb_id (not tmdb_id).
	// See docs/nexus/remaining-work.md and the B1 regression test in
	// provider_bug_fixes_test.go for the underlying API constraint.
	req := &resolver.ResolveRequest{EntityType: "movie", Metadata: map[string]string{"imdb_id": "tt0133093"}}
	if !r.CanResolve(context.Background(), req) {
		t.Error("resolver should activate with key + imdb_id for movies")
	}
	req2 := &resolver.ResolveRequest{EntityType: "movie", Metadata: map[string]string{}}
	if r.CanResolve(context.Background(), req2) {
		t.Error("resolver should not activate without an id")
	}
	// Unsupported entity types never activate even with an id.
	req3 := &resolver.ResolveRequest{EntityType: "book", Metadata: map[string]string{"imdb_id": "tt1"}}
	if r.CanResolve(context.Background(), req3) {
		t.Error("Fanart.tv should not claim 'book' entities")
	}
}

func TestIGDBResolver_InertWithoutCreds(t *testing.T) {
	t.Setenv("IGDB_CLIENT_ID", "")
	t.Setenv("IGDB_APP_ACCESS_TOKEN", "")
	r := NewIGDBResolver(22)
	if r.Name() != "igdb" {
		t.Errorf("name = %q", r.Name())
	}
	req := &resolver.ResolveRequest{EntityType: "game", Metadata: map[string]string{"title": "Halo"}}
	if r.CanResolve(context.Background(), req) {
		t.Error("igdb must not activate without both credentials")
	}
}

func TestIGDBResolver_PartialCredsStayInert(t *testing.T) {
	t.Setenv("IGDB_CLIENT_ID", "id")
	t.Setenv("IGDB_APP_ACCESS_TOKEN", "")
	r := NewIGDBResolver(22)
	req := &resolver.ResolveRequest{EntityType: "game"}
	if r.CanResolve(context.Background(), req) {
		t.Error("igdb must need both client id + token")
	}
}

func TestIGDBResolver_ActivatesForGameEntities(t *testing.T) {
	t.Setenv("IGDB_CLIENT_ID", "id")
	t.Setenv("IGDB_APP_ACCESS_TOKEN", "tok")
	r := NewIGDBResolver(22)
	for _, et := range []string{"game", "software"} {
		req := &resolver.ResolveRequest{EntityType: et}
		if !r.CanResolve(context.Background(), req) {
			t.Errorf("igdb should handle entity %q", et)
		}
	}
	req := &resolver.ResolveRequest{EntityType: "movie"}
	if r.CanResolve(context.Background(), req) {
		t.Error("igdb should not claim movies")
	}
}

func TestNormalizeIGDBImageURL(t *testing.T) {
	cases := map[string]string{
		"//images.igdb.com/igdb/image/upload/t_thumb/abc.jpg": "https://images.igdb.com/igdb/image/upload/t_cover_big/abc.jpg",
		"https://x.test/y.png":                               "https://x.test/y.png",
	}
	for in, want := range cases {
		if got := normalizeIGDBImageURL(in); got != want {
			t.Errorf("normalizeIGDBImageURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCoverArtArchiveResolver_RequiresMusicBrainzID(t *testing.T) {
	r := NewCoverArtArchiveResolver(21)
	if r.Name() != "cover_art_archive" {
		t.Errorf("name = %q", r.Name())
	}
	req := &resolver.ResolveRequest{EntityType: "music_album", Metadata: map[string]string{}}
	if r.CanResolve(context.Background(), req) {
		t.Error("CAA must not activate without a MusicBrainz release id")
	}
	req2 := &resolver.ResolveRequest{
		EntityType: "music_album",
		Metadata:   map[string]string{"musicbrainz_release_id": "abcd-ef"},
	}
	if !r.CanResolve(context.Background(), req2) {
		t.Error("CAA should activate with a MusicBrainz release id")
	}
}

func TestCoverArtArchiveResolver_RejectsNonMusicEntities(t *testing.T) {
	r := NewCoverArtArchiveResolver(21)
	req := &resolver.ResolveRequest{
		EntityType: "movie",
		Metadata:   map[string]string{"musicbrainz_release_id": "abcd-ef"},
	}
	if r.CanResolve(context.Background(), req) {
		t.Error("CAA should only handle music entities")
	}
}

func TestAllowPublicURLLocal(t *testing.T) {
	cases := []struct {
		url string
		bad bool
	}{
		{"https://upload.wikimedia.org/x.jpg", false},
		{"http://127.0.0.1/x", true},
		{"http://localhost/x", true},
		{"http://10.0.0.1/x", true},
		{"http://192.168.0.1/x", true},
		{"http://169.254.1.1/x", true},
		{"file:///etc/passwd", true},
		{"javascript:alert(1)", true},
		{"data:text/html,x", true},
	}
	for _, c := range cases {
		err := allowPublicURLLocal(c.url)
		if c.bad && err == nil {
			t.Errorf("expected reject for %q", c.url)
		}
		if !c.bad && err != nil {
			t.Errorf("expected allow for %q, got %v", c.url, err)
		}
	}
}

func TestFirstNonEmpty(t *testing.T) {
	if firstNonEmpty("", " ", "x", "y") != "x" {
		t.Error("firstNonEmpty should skip blanks")
	}
	if firstNonEmpty("", " ") != "" {
		t.Error("firstNonEmpty should return empty when nothing is set")
	}
}
