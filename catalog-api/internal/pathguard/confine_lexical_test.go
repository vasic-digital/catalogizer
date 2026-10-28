package pathguard

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// Callers keep the raw string and derive more paths from it with filepath.Join, Dir and Walk,
// which clean lexically ("lnk/.." disappears without looking at lnk). Confine therefore refuses
// a raw path whose lexical reading and kernel reading name different files (review round 4,
// H1: with lnk -> d1/d2, root/lnk/../../out is root/out to the kernel but <parent>/out to Join).
func TestConfine_RefusesLexicalKernelDivergence(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "root")
	for _, d := range []string{"d1/d2", "sub"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink("d1/d2", filepath.Join(root, "lnk")); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{
		root + "/lnk/../../out", // kernel root/out, lexical <base>/out (outside)
		root + "/lnk/../x",      // kernel root/d1/x, lexical root/x (both inside, different files)
		root + "/lnk/..",        // kernel root/d1, lexical root
	} {
		if _, err := Confine(p, []string{root}); !errors.Is(err, ErrAmbiguousPath) {
			t.Errorf("Confine(%q) = %v, want ErrAmbiguousPath", p, err)
		}
	}
	// ".." that does not follow a symlink means the same thing either way and stays accepted.
	for _, p := range []string{root + "/sub/../d1/x", root + "/lnk/x", root + "/./sub//y/"} {
		if _, err := Confine(p, []string{root}); err != nil {
			t.Errorf("Confine(%q) = %v, want nil", p, err)
		}
	}
}

// Dir(raw) and every other lexical prefix of an accepted path must stay inside the root too:
// with a -> /outside/x and /outside/x/b -> root/b, root/a/b resolves inside the root, but
// MkdirAll(Dir(root/a/b)) and an output directory of Dir(...) are /outside/x.
func TestConfine_RefusesTransientEscapeThroughAPrefix(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "root")
	outside := filepath.Join(base, "outside", "x")
	for _, d := range []string{root + "/b", outside} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(outside, filepath.Join(root, "a")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "b"), filepath.Join(outside, "b")); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(root, "a", "b", "file.pdf")
	if r, err := resolve(p); err != nil || !within(root, r) {
		t.Fatalf("fixture: %q resolves to %q (%v), want inside the root", p, r, err)
	}
	if _, err := Confine(p, []string{root}); !errors.Is(err, ErrOutsideRoots) {
		t.Errorf("Confine(%q) = %v, want ErrOutsideRoots (Dir passes through %s)", p, err, outside)
	}
	// A link from outside the root that lands below it: the path resolves inside, its Dir does
	// not (found by the extended differential oracle, mode 3).
	if err := os.Symlink(filepath.Join(root, "b", "new"), filepath.Join(base, "l1")); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{filepath.Join(base, "l1"), root + "/../l1"} {
		if _, err := Confine(p, []string{root}); !errors.Is(err, ErrOutsideRoots) {
			t.Errorf("Confine(%q) = %v, want ErrOutsideRoots (Dir is %s)", p, err, base)
		}
	}
	// A symlinked root itself, and a link inside the root that stays inside, are fine.
	if err := os.Symlink(filepath.Join(root, "b"), filepath.Join(root, "in")); err != nil {
		t.Fatal(err)
	}
	if _, err := Confine(filepath.Join(root, "in", "f"), []string{root}); err != nil {
		t.Errorf("link staying inside the root refused: %v", err)
	}
}

// The default database is protected by package initialisation alone, with no call to
// ProtectDatabase and no test hook replacing the list.
func TestProtectedPaths_DefaultDatabaseFromInit(t *testing.T) {
	want, err := filepath.Abs(DefaultDatabaseFile)
	if err != nil {
		t.Fatal(err)
	}
	got := ProtectedPaths()
	for _, suf := range []string{"", "-wal", "-shm", "-journal"} {
		if !slices.Contains(got, want+suf) {
			t.Errorf("ProtectedPaths() = %v, missing %q", got, want+suf)
		}
	}
}

// A relative link target is resolved from the link's resolved directory, also when the link is
// reached through another (directory) symlink; compared with the kernel via EvalSymlinks.
func TestResolve_RelativeLinkReachedThroughALinkedDir(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "root")
	for _, d := range []string{"sub/deep", "sub/x", "other"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	links := map[string]string{
		"s":            "sub/deep",
		"sub/deep/rl":  "../x",
		"sub/deep/rl2": "../../other",
		"o":            "s/rl",
	}
	for l, tgt := range links {
		if err := os.Symlink(tgt, filepath.Join(root, l)); err != nil {
			t.Fatal(err)
		}
	}
	for _, p := range []string{"s/rl", "s/rl2", "o", "s/rl/f"} {
		full := filepath.Join(root, p)
		got, err := resolve(full)
		if err != nil {
			t.Fatalf("resolve(%q): %v", full, err)
		}
		dir, last := full, ""
		if p == "s/rl/f" {
			dir, last = filepath.Dir(full), "f"
		}
		kern, err := filepath.EvalSymlinks(dir)
		if err != nil {
			t.Fatal(err)
		}
		if last != "" {
			kern = filepath.Join(kern, last)
		}
		if got != kern {
			t.Errorf("resolve(%q) = %q, kernel %q", full, got, kern)
		}
		if _, err := Confine(full, []string{root}); err != nil {
			t.Errorf("Confine(%q) = %v, want nil", full, err)
		}
	}
}

// SetProtected replaces the protected list and returns the previous one, so tests in other
// packages can restore it (review round 4: a services test leaked its temp database into the
// global list, and every later test that named /tmp got ErrProtected).
func TestSetProtected_ReturnsPreviousForRestore(t *testing.T) {
	before := ProtectedPaths()
	prev := SetProtected([]string{"/nonexistent/a.db"})
	if got := ProtectedPaths(); len(got) != 1 || got[0] != "/nonexistent/a.db" {
		t.Errorf("after SetProtected: %v", got)
	}
	SetProtected(prev)
	if got := ProtectedPaths(); !slices.Equal(got, before) {
		t.Errorf("restore: got %v, want %v", got, before)
	}
}

// A protected path is compared after symlink resolution even when the protected file does not
// exist yet: a key file registered as base/linkdir/key (linkdir -> real) is refused when a
// request names base/real/key. The file-identity check cannot help here, there is no file.
func TestConfineData_ProtectedPathResolvedThroughALinkedParentBeforeItExists(t *testing.T) {
	base := t.TempDir()
	real := filepath.Join(base, "real")
	if err := os.MkdirAll(real, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, filepath.Join(base, "linkdir")); err != nil {
		t.Fatal(err)
	}
	prev := SetProtected([]string{filepath.Join(base, "linkdir", "key")})
	t.Cleanup(func() { SetProtected(prev) })

	if _, err := ConfineData(filepath.Join(real, "key"), []string{base}); !errors.Is(err, ErrProtected) {
		t.Fatalf("real/key: got %v, want ErrProtected", err)
	}
	if _, err := ConfineData(filepath.Join(real, "other"), []string{base}); err != nil {
		t.Fatalf("real/other is not protected: %v", err)
	}
}

// Any resolution error other than "does not exist" is a rejection: a path through a regular
// file (ENOTDIR) is refused, never treated as a path still to be created.
func TestConfine_RefusesPathThroughARegularFile(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "file"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, err := Confine(filepath.Join(root, "file", "x"), []string{root}); err == nil {
		t.Fatalf("accepted %q through a regular file", got)
	}
}
