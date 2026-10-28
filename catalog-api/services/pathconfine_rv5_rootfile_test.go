package services

import (
	"os"
	"path/filepath"
	"testing"
)

// When a configured sync root is a regular FILE, filepath.Walk(root) visits the root itself as a
// non-directory, and the relative path is ".": the walkers then hand the root itself to
// uploadableFile / copyFile, and the destination becomes the destination root itself. Dir of that
// destination is the destination root's PARENT (review round 5, RV5-B). These tests pin that the
// per-file checks treat a path equal to its root as "not a file below the root".

func TestSync_CopyFileRefusesSourceEqualToItsRoot(t *testing.T) {
	svc, _ := newSyncSvc(t)
	base := t.TempDir()
	srcRoot := filepath.Join(base, "single.bin")
	if err := os.WriteFile(srcRoot, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}
	dstRoot := filepath.Join(base, "dst")
	if err := os.MkdirAll(dstRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(dstRoot, "out.bin")
	if err := svc.copyFile(srcRoot, dst, 0o644, srcRoot, dstRoot); err == nil {
		t.Fatalf("copyFile accepted a source equal to its root %q", srcRoot)
	}
	if _, err := os.Lstat(dst); !os.IsNotExist(err) {
		t.Fatalf("destination %q was written (err=%v)", dst, err)
	}
	// positive control: the same copy from a file strictly below a directory root succeeds
	srcDir := filepath.Join(base, "srcdir")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(srcDir, "f.bin")
	if err := os.WriteFile(src, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := svc.copyFile(src, dst, 0o644, srcDir, dstRoot); err != nil {
		t.Fatalf("positive control: copyFile refused a legitimate copy: %v", err)
	}
	if b, err := os.ReadFile(dst); err != nil || string(b) != "payload" {
		t.Fatalf("positive control: destination content %q err=%v", b, err)
	}
}

func TestSync_CopyFileRefusesDestinationEqualToItsRoot(t *testing.T) {
	svc, _ := newSyncSvc(t)
	base := t.TempDir()
	srcDir := filepath.Join(base, "srcdir")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(srcDir, "f.bin")
	if err := os.WriteFile(src, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}
	// the destination root does not exist yet; it is the path the copy would write as a file
	parent := filepath.Join(base, "parent")
	if err := os.MkdirAll(parent, 0o755); err != nil {
		t.Fatal(err)
	}
	dstRoot := filepath.Join(parent, "dstroot")
	if err := svc.copyFile(src, dstRoot, 0o644, srcDir, dstRoot); err == nil {
		t.Fatalf("copyFile accepted a destination equal to its root %q", dstRoot)
	}
	if _, err := os.Lstat(dstRoot); !os.IsNotExist(err) {
		t.Fatalf("destination root %q was written as a file (err=%v)", dstRoot, err)
	}
	if ents, _ := os.ReadDir(parent); len(ents) != 0 {
		t.Fatalf("something was created in the destination root's parent: %v", ents)
	}
}

func TestSync_UploadableFileRefusesARootThatIsItselfAFile(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "single.bin")
	if err := os.WriteFile(root, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(root)
	if err != nil {
		t.Fatal(err)
	}
	if uploadableFile(root, info, root) {
		t.Fatalf("uploadableFile accepted the root itself %q as a file below it", root)
	}
	// positive control: a regular file strictly below a directory root is uploadable
	dir := filepath.Join(base, "d")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	f := filepath.Join(dir, "f.bin")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Lstat(f)
	if err != nil {
		t.Fatal(err)
	}
	if !uploadableFile(f, fi, dir) {
		t.Fatalf("positive control: uploadableFile refused %q below %q", f, dir)
	}
}
