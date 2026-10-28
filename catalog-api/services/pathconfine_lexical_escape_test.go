package services

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"catalogizer/internal/pathguard"
	"catalogizer/models"
)

// the raw request path is confined kernel-style, but the local sync driver
// then builds per-file paths with filepath.Join(rawDir, rel), which Cleans the raw string
// lexically. With an intra-root symlink lnk -> d1/d2, "root/lnk/../../out" resolves (kernel)
// to root/out but Cleans to <parent-of-root>/out.
func TestPathConfine_LocalMirrorDestDirLexicalEscape(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "root")
	for _, d := range []string{"d1/d2", "src"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink("d1/d2", filepath.Join(root, "lnk")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "src", "f.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(pathGuardEnvSync, root)
	t.Setenv(pathGuardEnvRead, root)
	dest := root + "/lnk/../../out"
	settings, _ := json.Marshal(map[string]string{
		"source_directory": filepath.Join(root, "src"), "destination_directory": dest, "sync_mode": "mirror"})
	ss := string(settings)
	ep := &models.SyncEndpoint{Type: models.SyncTypeLocal, SyncDirection: models.SyncDirectionDownload,
		LocalPath: filepath.Join(root, "src"), SyncSettings: &ss}
	s := &SyncService{}
	cerr := s.confineSyncPaths(ep)
	r, _ := pathguard.Confine(dest, []string{root})
	t.Logf("confineSyncPaths err=%v ; confine(dest)=%q", cerr, r)
	if cerr != nil {
		return // refused: no escape
	}
	_ = s.performLocalSync(&models.SyncSession{}, ep)
	if _, err := os.Stat(filepath.Join(base, "out", "f.txt")); err == nil {
		t.Fatalf("ESCAPE: local mirror wrote %s outside root %s", filepath.Join(base, "out", "f.txt"), root)
	}
}

// Same class for the upload/read side: source_directory confined raw, filepath.Walk joins
// children lexically, so files outside the read root are opened and uploaded.
func TestPathConfine_WalkOverAcceptedDirStaysInside(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "root")
	_ = os.MkdirAll(filepath.Join(root, "d1/d2/sub"), 0o755)
	_ = os.MkdirAll(filepath.Join(base, "sub"), 0o755)
	_ = os.Symlink("d1/d2", filepath.Join(root, "lnk"))
	_ = os.WriteFile(filepath.Join(root, "sub-placeholder"), nil, 0o644)
	_ = os.MkdirAll(filepath.Join(root, "sub"), 0o755)
	_ = os.WriteFile(filepath.Join(root, "sub", "secret"), []byte("in"), 0o644)
	_ = os.WriteFile(filepath.Join(base, "sub", "secret"), []byte("OUTSIDE"), 0o644)
	raw := root + "/lnk/../../sub"
	if _, err := pathguard.Confine(raw, []string{root}); err != nil {
		t.Logf("refused: %v", err)
		return // refused: nothing is walked
	}
	var read string
	_ = filepath.Walk(raw, func(p string, fi os.FileInfo, err error) error {
		if err == nil && fi.Mode().IsRegular() {
			b, _ := os.ReadFile(p)
			read += p + "=" + string(b) + ";"
		}
		return nil
	})
	t.Logf("walk read: %s", read)
	if len(read) > 0 && filepath.Base(filepath.Dir(base)) != "" && strings.Contains(read, "OUTSIDE") {
		t.Fatalf("ESCAPE: Walk over confined raw dir read a file outside the root: %s", read)
	}
}
