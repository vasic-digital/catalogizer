package nfsemu

// CAT-NFS-EMU-2. Evidence class: EMULATOR. These tests drive the project's real user-space NFSv3 client
// (digital.vasic.filesystem/pkg/nfs3, the same package the factory registers) against the unfs3 (unfsd 0.11.0)
// container built by scripts/test-infra/nfs_build.sh. The server tree is seeded and its manifest (path, size, sha256)
// is produced SERVER-SIDE by scripts/test-infra/nfs_emulator_live.sh; the client must reproduce it. A real Synology
// NFS export is a separate leg (honest SKIP hardware_not_present). Without NFS_EMU_ADDR the tests SKIP (no stand-in).

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"digital.vasic.filesystem/pkg/nfs3"
)

type entry struct {
	Path   string `json:"path"`
	Dir    bool   `json:"dir"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

func connect(t *testing.T) (*nfs3.Client, []entry) {
	t.Helper()
	addr, mf := os.Getenv("NFS_EMU_ADDR"), os.Getenv("NFS_EMU_MANIFEST")
	if addr == "" || mf == "" {
		t.Skip("NFS_EMU_ADDR/NFS_EMU_MANIFEST not set: run scripts/test-infra/nfs_emulator_live.sh (no stand-in server is used)")
	}
	host, ps, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatal(err)
	}
	p, _ := strconv.Atoi(ps)
	pm, _ := strconv.Atoi(os.Getenv("NFS_EMU_PORTMAP_PORT"))
	raw, err := os.ReadFile(mf)
	if err != nil {
		t.Fatal(err)
	}
	var m []entry
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	cfg := nfs3.Config{Host: host, Export: "/export", MountPort: p, NFSPort: p, PortmapPort: pm, UID: 1000, GID: 1000, CallTimeout: 10 * time.Second}
	if os.Getenv("NFS_EMU_USE_PORTMAP") == "1" {
		cfg.MountPort, cfg.NFSPort = 0, 0 // ask the portmapper (only valid when the container ports are reachable as published)
	}
	c, err := nfs3.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := c.Connect(ctx); err != nil {
		t.Fatalf("Connect to unfs3 emulator: %v", err)
	}
	t.Cleanup(func() { _ = c.Disconnect(context.Background()) })
	return c, m
}

func TestUnfs3ExportsListsSeededExport(t *testing.T) {
	c, _ := connect(t)
	ex, err := c.Exports(context.Background())
	if err != nil {
		t.Fatalf("MOUNT EXPORT: %v", err)
	}
	for _, e := range ex {
		if e.Path == "/export" {
			return
		}
	}
	t.Fatalf("export /export not offered: %+v", ex)
}

func TestUnfs3ListStatReadMatchServerManifest(t *testing.T) {
	c, man := connect(t)
	ctx := context.Background()
	if len(man) == 0 {
		t.Fatal("empty manifest")
	}
	for _, m := range man {
		fi, err := c.GetFileInfo(ctx, m.Path)
		if err != nil {
			t.Fatalf("stat %s: %v", m.Path, err)
		}
		if fi.IsDir != m.Dir {
			t.Errorf("%s: IsDir=%v want %v", m.Path, fi.IsDir, m.Dir)
		}
		if m.Dir {
			continue
		}
		if fi.Size != m.Size {
			t.Errorf("%s: size %d want %d", m.Path, fi.Size, m.Size)
		}
		rc, err := c.ReadFile(ctx, m.Path)
		if err != nil {
			t.Fatalf("read %s: %v", m.Path, err)
		}
		h := sha256.New()
		n, err := io.Copy(h, rc)
		_ = rc.Close()
		if err != nil || n != m.Size {
			t.Errorf("%s: read %d bytes err=%v want %d", m.Path, n, err, m.Size)
		}
		if got := hex.EncodeToString(h.Sum(nil)); got != m.SHA256 {
			t.Errorf("%s: sha256 %s want %s", m.Path, got, m.SHA256)
		}
	}
}

// TestUnfs3ListDirectoryMatchesServerManifest: unfs3 0.11.0 does not implement READDIRPLUS (RFC 1813 3.3.17) and answers
// NFS3ERR_NOTSUPP; pkg/nfs3 falls back to plain READDIR (3.3.16) and fills attributes via LOOKUP. This replaces the former
// known-gap pin (CAT-NFS-EMU-2 finding, operator decision 2026-11-16, Roadmap-Item NFS3-READDIR). Every directory of the
// server-side manifest must list exactly the manifest's children with matching type and size.
func TestUnfs3ListDirectoryMatchesServerManifest(t *testing.T) {
	c, man := connect(t)
	ctx := context.Background()
	want := map[string]map[string]entry{"/": {}}
	for _, m := range man {
		if m.Dir {
			want[m.Path] = map[string]entry{}
		}
	}
	for _, m := range man {
		parent := m.Path[:strings.LastIndex(m.Path, "/")]
		if parent == "" {
			parent = "/"
		}
		want[parent][m.Path[strings.LastIndex(m.Path, "/")+1:]] = m
	}
	for dir, kids := range want {
		l, err := c.ListDirectory(ctx, dir)
		if err != nil {
			t.Fatalf("ListDirectory(%s) against unfs3: %v", dir, err)
		}
		if len(l) != len(kids) {
			t.Errorf("%s: %d entries, manifest has %d", dir, len(l), len(kids))
		}
		for _, f := range l {
			m, ok := kids[f.Name]
			if !ok {
				t.Errorf("%s: unexpected entry %q", dir, f.Name)
				continue
			}
			if f.IsDir != m.Dir || (!m.Dir && f.Size != m.Size) {
				t.Errorf("%s/%s: IsDir=%v size=%d, manifest dir=%v size=%d", dir, f.Name, f.IsDir, f.Size, m.Dir, m.Size)
			}
		}
	}
	if _, ok := want["/"]["readme.txt"]; !ok {
		t.Fatal("control needle: seeded readme.txt missing from the manifest, test would be vacuous")
	}
}

func TestUnfs3MissingPathAndWritesRefused(t *testing.T) {
	c, _ := connect(t)
	ctx := context.Background()
	ok, err := c.FileExists(ctx, "/definitely/not/there")
	if err != nil || ok {
		t.Errorf("FileExists(missing)=%v err=%v want false,nil", ok, err)
	}
	if err := c.WriteFile(ctx, "/x", nil); err != nfs3.ErrReadOnly {
		t.Errorf("WriteFile err=%v want ErrReadOnly", err)
	}
}
