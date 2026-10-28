package pathguard

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfine(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	// symlink inside root pointing outside
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name  string
		path  string
		roots []string
		ok    bool
	}{
		{"inside", filepath.Join(root, "sub", "f.txt"), []string{root}, true},
		{"root itself", root, []string{root}, true},
		{"dotdot escape", filepath.Join(root, "sub", "..", "..", "etc", "passwd"), []string{root}, false},
		{"outside", outside, []string{root}, false},
		{"sibling prefix", root + "-evil", []string{root}, false},
		{"symlink escape", filepath.Join(root, "escape", "x"), []string{root}, false},
		{"empty", "", []string{root}, false},
		{"nul", filepath.Join(root, "a\x00b"), []string{root}, false},
		{"relative", "relative/path", []string{root}, false},
		{"no roots", filepath.Join(root, "f"), nil, false},
		{"second root", filepath.Join(outside, "f"), []string{root, outside}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := Confine(c.path, c.roots)
			if c.ok && err != nil {
				t.Fatalf("want ok, got %v", err)
			}
			if !c.ok && err == nil {
				t.Fatalf("want error, got %q", got)
			}
		})
	}
}

func TestRootsFromEnv(t *testing.T) {
	t.Setenv("PG_TEST_ROOTS", "/a:/b::/c")
	got := RootsFromEnv("PG_TEST_ROOTS", "/d")
	if len(got) != 3 || got[0] != "/a" || got[2] != "/c" {
		t.Fatalf("got %v", got)
	}
	got = RootsFromEnv("PG_TEST_UNSET_X", "/d")
	if len(got) != 1 || !filepath.IsAbs(got[0]) {
		t.Fatalf("default not absolutised: %v", got)
	}
}

func TestSafeComponent(t *testing.T) {
	for _, bad := range []string{"", ".", "..", "a/b", "a\\b", "../x", "a\x00"} {
		if SafeComponent(bad) {
			t.Errorf("%q should be unsafe", bad)
		}
	}
	for _, good := range []string{"42", "movie", "a.b", "...And"} {
		if !SafeComponent(good) {
			t.Errorf("%q should be safe", good)
		}
	}
}
