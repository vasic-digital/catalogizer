package services

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"catalogizer/internal/pathguard"

	"digital.vasic.assets/pkg/resolver"
)

// F7 (§11.4.115): LocalScanResolver takes its directory from the request-fed source hint.
func TestPathConfine_LocalScanResolverConfinedToReadRoots(t *testing.T) {
	media := t.TempDir()
	outside := t.TempDir()
	t.Setenv(pathguard.EnvReadRoots, media)
	t.Setenv(pathguard.EnvMediaRoot, "")
	r := NewLocalScanResolver(1)

	// control: cover inside the read root is served
	if err := os.MkdirAll(filepath.Join(media, "Movie"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(media, "Movie", "cover.jpg"), []byte("LEGIT"), 0o600); err != nil {
		t.Fatal(err)
	}
	req := &resolver.ResolveRequest{SourceHint: filepath.Join(media, "Movie", "movie.mkv")}
	if !r.CanResolve(context.Background(), req) {
		t.Fatal("control: cover in read root must resolve")
	}
	res, err := r.Resolve(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(res.Content)
	res.Content.(io.Closer).Close()
	if string(b) != "LEGIT" {
		t.Fatalf("control content %q", b)
	}

	// absolute hint outside the roots: not served, existence not revealed
	if err := os.WriteFile(filepath.Join(outside, "cover.jpg"), []byte("SECRET"), 0o600); err != nil {
		t.Fatal(err)
	}
	evil := &resolver.ResolveRequest{SourceHint: filepath.Join(outside, "x.mkv")}
	if r.CanResolve(context.Background(), evil) {
		t.Error("CanResolve true for a directory outside the read roots (existence oracle)")
	}
	if res, err := r.Resolve(context.Background(), evil); err == nil {
		res.Content.(io.Closer).Close()
		t.Error("Resolve served a cover from outside the read roots")
	} else if got := err.Error(); containsPath(got, outside) {
		t.Errorf("error leaks the probed path: %q", got)
	}

	// symlinked cover inside the root pointing outside: refused
	if err := os.MkdirAll(filepath.Join(media, "Linked"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "cover.jpg"), filepath.Join(media, "Linked", "cover.jpg")); err != nil {
		t.Fatal(err)
	}
	link := &resolver.ResolveRequest{SourceHint: filepath.Join(media, "Linked", "x.mkv")}
	if r.CanResolve(context.Background(), link) {
		t.Error("symlinked cover file accepted")
	}
}

func containsPath(msg, p string) bool { return len(p) > 0 && (len(msg) >= len(p)) && (indexOf(msg, p) >= 0) }

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
