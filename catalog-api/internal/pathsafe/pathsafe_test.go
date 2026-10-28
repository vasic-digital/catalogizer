package pathsafe

import (
	"os"
	"path/filepath"
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
