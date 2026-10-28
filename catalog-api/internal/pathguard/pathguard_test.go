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

// F1 (review round 1): a DANGLING symlink inside a root must not be treated as a missing
// path element; writing through it would create the target outside the root.
func TestConfine_DanglingSymlinkEscapes(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(filepath.Join(outside, "newfile"), filepath.Join(root, "dangling")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "newdir"), filepath.Join(root, "danglingdir")); err != nil {
		t.Fatal(err)
	}
	for name, p := range map[string]string{
		"dangling leaf":   filepath.Join(root, "dangling"),
		"dangling parent": filepath.Join(root, "danglingdir", "x"),
	} {
		if got, err := Confine(p, []string{root}); err == nil {
			t.Errorf("%s: dangling symlink to outside accepted as %q", name, got)
		}
	}
	// control: a dangling link whose target is inside the root is still confined (and allowed)
	if err := os.Symlink(filepath.Join(root, "later"), filepath.Join(root, "inlink")); err != nil {
		t.Fatal(err)
	}
	if _, err := Confine(filepath.Join(root, "inlink"), []string{root}); err != nil {
		t.Errorf("control: dangling link to a path inside the root must be allowed: %v", err)
	}
	// control: plain missing elements still allowed
	if _, err := Confine(filepath.Join(root, "new", "deep", "f"), []string{root}); err != nil {
		t.Errorf("control: missing path under root must be allowed: %v", err)
	}
}

// A symlink loop must fail closed.
func TestConfine_SymlinkLoopRejected(t *testing.T) {
	root := t.TempDir()
	a, b := filepath.Join(root, "a"), filepath.Join(root, "b")
	_ = os.Symlink(b, a)
	_ = os.Symlink(a, b)
	if _, err := Confine(filepath.Join(root, "a", "x"), []string{root}); err == nil {
		t.Error("symlink loop accepted")
	}
}

func TestEffectiveRoots(t *testing.T) {
	t.Setenv(EnvReadRoots, "")
	t.Setenv(EnvSyncRoots, "")
	t.Setenv(EnvConversionRoots, "")
	t.Setenv(EnvMediaRoot, "")
	abs := func(p string) string { a, _ := filepath.Abs(p); return a }
	if got := ReadRoots(); len(got) != 1 || got[0] != abs("data") {
		t.Fatalf("default read roots = %v", got)
	}
	t.Setenv(EnvMediaRoot, "/media")
	if got := ReadRoots(); len(got) != 2 || got[0] != "/media" || got[1] != abs("data") {
		t.Fatalf("read roots with MEDIA_ROOT_PATH = %v", got)
	}
	t.Setenv(EnvReadRoots, "/x:/y")
	if got := ReadRoots(); len(got) != 2 || got[0] != "/x" {
		t.Fatalf("explicit read roots must replace defaults: %v", got)
	}
	if got := SyncWriteRoots(); len(got) != 1 || got[0] != abs("data") {
		t.Fatalf("sync write default = %v", got)
	}
	t.Setenv(EnvConversionRoots, "/w")
	if got := Readable(ConversionWriteRoots()); got[len(got)-1] != "/w" || len(got) != 3 {
		t.Fatalf("readable must include write roots: %v", got)
	}
	if d := Describe(); d == "" {
		t.Fatal("empty describe")
	}
}
