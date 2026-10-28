package services

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The SMB download path must create the local file via pathsafe.CreateNoFollow:
// a symlink planted at the destination must not redirect the write.
func TestCopyRemoteToLocalNoFollow_RefusesSymlinkDestination(t *testing.T) {
	dir := t.TempDir()
	victim := filepath.Join(dir, "victim.txt")
	if err := os.WriteFile(victim, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "dest.bin")
	if err := os.Symlink(victim, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	err := copyRemoteToLocalNoFollow(link, strings.NewReader("ATTACKER-CONTROLLED"), 4096)
	if err == nil {
		t.Fatal("expected error writing through a symlink destination, got nil")
	}
	got, rerr := os.ReadFile(victim)
	if rerr != nil {
		t.Fatal(rerr)
	}
	if string(got) != "original" {
		t.Fatalf("symlink target was overwritten: %q", got)
	}
}

func TestCopyRemoteToLocalNoFollow_WritesRegularDestination(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "out.bin")
	if err := copyRemoteToLocalNoFollow(dest, strings.NewReader("payload-bytes"), 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, err := os.ReadFile(dest)
	if err != nil || string(got) != "payload-bytes" {
		t.Fatalf("content mismatch: %q err=%v", got, err)
	}
}

// Structural wiring guard (the live SMB path needs a real share): DownloadFile
// must route its local write through the no-follow helper and never call
// os.Create/os.OpenFile directly.
func TestSMBDownloadFile_UsesNoFollowHelper(t *testing.T) {
	src, err := os.ReadFile("smb.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(src)
	start := strings.Index(s, "func (s *SMBService) DownloadFile(")
	end := strings.Index(s, "func copyRemoteToLocalNoFollow(")
	if start < 0 || end < start {
		t.Fatal("could not locate DownloadFile body")
	}
	body := s[start:end]
	if !strings.Contains(body, "copyRemoteToLocalNoFollow(") {
		t.Error("DownloadFile does not call copyRemoteToLocalNoFollow")
	}
	for _, bad := range []string{"os.Create(", "os.OpenFile("} {
		if strings.Contains(body, bad) {
			t.Errorf("DownloadFile must not call %s directly", bad)
		}
	}
}
