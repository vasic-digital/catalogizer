package pathsafe

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestContainedJoin_SiblingPrefixRejected(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "x")
	sibling := filepath.Join(base, "xy")
	for _, d := range []string{root, sibling} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// "../xy/f" resolves to <base>/xy/f which shares the string prefix of root.
	if got, err := ContainedJoin(root, "../xy/f"); err == nil {
		t.Fatalf("sibling-prefix escape accepted: %q", got)
	}
	if got, err := ContainedJoin(root, filepath.Join(sibling, "f")); err == nil {
		t.Fatalf("absolute sibling-prefix path accepted: %q", got)
	}
	if _, err := ContainedJoin(root, "sub/f"); err != nil {
		t.Fatalf("legit child rejected: %v", err)
	}
}

func TestWithin_FilesystemRoot(t *testing.T) {
	if !within("/", "/etc/passwd") {
		t.Fatal(`within("/", "/etc/passwd") must be true`)
	}
	if !within("/", "/") {
		t.Fatal(`within("/", "/") must be true`)
	}
	if within("/tmp/x", "/tmp/xy") {
		t.Fatal("sibling prefix must not be within")
	}
}

func TestContainedJoin_RootSlash(t *testing.T) {
	got, err := ContainedJoin("/", "tmp/some-new-file")
	if err != nil || got != "/tmp/some-new-file" {
		t.Fatalf("root / join = %q, %v", got, err)
	}
}

func TestContainedJoin_SymlinkEscapeRejected(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	if got, err := ContainedJoin(root, "link/f"); err == nil {
		t.Fatalf("symlink escape accepted: %q", got)
	}
}

func TestSafeChildName(t *testing.T) {
	for _, bad := range []string{"", ".", "..", "a/b", `a\b`, "a\x00b", "../x", `..\x`} {
		if _, err := SafeChildName(bad); err == nil {
			t.Errorf("SafeChildName(%q) accepted", bad)
		}
	}
	if n, err := SafeChildName("movie.mkv"); err != nil || n != "movie.mkv" {
		t.Errorf("legit name rejected: %v", err)
	}
}

func TestSafeRemotePath(t *testing.T) {
	for _, bad := range []string{"a/../b", `a\..\b`, "..", "x\x00"} {
		if err := SafeRemotePath(bad); err == nil {
			t.Errorf("SafeRemotePath(%q) accepted", bad)
		}
	}
	for _, ok := range []string{"a/b", `a\b`, "/share/dir/file.txt", "a..b"} {
		if err := SafeRemotePath(ok); err != nil {
			t.Errorf("SafeRemotePath(%q) rejected: %v", ok, err)
		}
	}
}

// OpenNoFollow must refuse a final-component symlink (check-to-use race hardening).
func TestCreateNoFollow_RefusesSymlinkLeaf(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(t.TempDir(), "victim")
	if err := os.WriteFile(target, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	leaf := filepath.Join(dir, "f")
	if err := os.Symlink(target, leaf); err != nil {
		t.Fatal(err)
	}
	if f, err := CreateNoFollow(leaf, true); err == nil {
		f.Close()
		t.Fatal("CreateNoFollow followed a symlink leaf")
	}
	b, _ := os.ReadFile(target)
	if string(b) != "keep" {
		t.Fatalf("victim modified: %q", b)
	}
	f, err := CreateNoFollow(filepath.Join(dir, "new"), false)
	if err != nil {
		t.Fatalf("plain create failed: %v", err)
	}
	f.Close()
}

// The exported Within must be safe on UNCLEANED input: a lexical prefix test on
// raw strings would call "/a/b/../../etc" within "/a/b".
func TestWithin_ExportedCleansBothArguments(t *testing.T) {
	cases := []struct {
		root, p string
		want    bool
	}{
		{"/a/b", "/a/b/../../etc", false},
		{"/a/b", "/a/b/c/../d", true},
		{"/a/b/", "/a/b/c", true},
		{"/a/b/../b", "/a/b/c", true},
		{"/a/b", "/a/b/..", false},
		{"/a/b", "/a/b/", true},
	}
	for _, c := range cases {
		if got := Within(c.root, c.p); got != c.want {
			t.Errorf("Within(%q, %q) = %v, want %v", c.root, c.p, got, c.want)
		}
	}
}

// Ported from the removed root handlers/copy_security_test.go (live confinement guard, direct calls).

func TestContainedJoin_RejectsTraversal(t *testing.T) {
	root := t.TempDir()
	for _, p := range []string{"../escape", "a/../../escape", "a/b/../../../escape", "/etc/passwd", root + "/../escape", "a\x00b", ".."} {
		if got, err := ContainedJoin(root, p); err == nil {
			t.Errorf("path %q must be rejected (got %q)", p, got)
		}
	}
}

func TestContainedJoin_AcceptsContained(t *testing.T) {
	root := t.TempDir()
	for _, p := range []string{"a.txt", "sub/dir/a.txt", root + "/x/y", "./a/./b"} {
		got, err := ContainedJoin(root, p)
		if err != nil {
			t.Fatalf("%q rejected: %v", p, err)
		}
		if got != root && !strings.HasPrefix(got, root+string(os.PathSeparator)) {
			t.Errorf("%q escaped root: %q", p, got)
		}
	}
}

func TestContainedJoin_SymlinkEscapeNonexistentTail(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"link/file.txt", "link/new/dir/file.txt"} {
		if got, err := ContainedJoin(root, p); err == nil {
			t.Errorf("%q through escaping symlink accepted: %q", p, got)
		}
	}
}

func TestContainedJoin_EmptyRootRejected(t *testing.T) {
	if _, err := ContainedJoin("", "a.txt"); err == nil {
		t.Fatal("empty root accepted")
	}
}

func TestSafeChildName_Extended(t *testing.T) {
	for _, n := range []string{"", ".", "..", "../x", "a/b", `a\b`, `..\x`, "x\x00y", "/abs"} {
		if _, err := SafeChildName(n); err == nil {
			t.Errorf("name %q must be rejected", n)
		}
	}
	for _, n := range []string{"movie.mkv", "a b.txt", "..hidden", "ü.mp3"} {
		if got, err := SafeChildName(n); err != nil || got != n {
			t.Errorf("%q: %q, %v", n, got, err)
		}
	}
}

func TestSafeRemotePath_Extended(t *testing.T) {
	for _, p := range []string{"../x", "a/../../x", `a\..\x`, "a\x00b", "/a/b/../../.."} {
		if err := SafeRemotePath(p); err == nil {
			t.Errorf("remote path %q must be rejected", p)
		}
	}
	for _, p := range []string{"/media/movies", "a/b", "a/..b/c", "/"} {
		if err := SafeRemotePath(p); err != nil {
			t.Errorf("%q rejected: %v", p, err)
		}
	}
}
