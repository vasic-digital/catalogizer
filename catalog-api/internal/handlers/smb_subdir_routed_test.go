package handlers

import (
	"bytes"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"catalogizer/internal/config"
	"catalogizer/internal/services"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// fakeShare stands in for the SMB server only (unit-class); the routed handler, SMBService, SplitShare and the sub-directory confinement are real.
type fakeShare struct {
	fsys  fstest.MapFS
	calls []string
	mount []string
}

type fakeFile struct{ fs.File }

func (f fakeFile) Seek(o int64, w int) (int64, error) { return f.File.(io.Seeker).Seek(o, w) }
func (f fakeFile) Stat() (os.FileInfo, error)         { return f.File.Stat() }

func (s *fakeShare) ReadDir(n string) ([]os.FileInfo, error) {
	s.calls = append(s.calls, n)
	return nil, fs.ErrNotExist
}
func (s *fakeShare) Open(n string) (services.RemoteFile, error) {
	s.calls = append(s.calls, n)
	f, err := s.fsys.Open(n)
	if err != nil {
		return nil, err
	}
	return fakeFile{f}, nil
}
func (s *fakeShare) Create(n string) (io.WriteCloser, error) {
	s.calls = append(s.calls, n)
	return nil, fs.ErrPermission
}
func (s *fakeShare) Stat(n string) (os.FileInfo, error) {
	s.calls = append(s.calls, n)
	return fs.Stat(s.fsys, n)
}
func (s *fakeShare) Mkdir(n string, _ os.FileMode) error { s.calls = append(s.calls, n); return nil }
func (s *fakeShare) Remove(n string) error               { s.calls = append(s.calls, n); return nil }
func (s *fakeShare) Umount() error                       { return nil }

// CAT-SMB-SUBDIR-1 F1: the ROUTED endpoint POST /copy/local downloads from an SMB storage root whose path is share/sub/dir.
func TestRoutedCopyToLocal_SMBSubdirRoot(t *testing.T) {
	fk := &fakeShare{fsys: fstest.MapFS{
		"Testing/docs/book.txt": {Data: []byte("book")},
		"Misc/book.txt":         {Data: []byte("OUTSIDE")},
		"docs/book.txt":         {Data: []byte("share-root-book")},
	}}
	cfg := &config.Config{}
	cfg.SMB.Hosts = []config.SMBHost{{Name: "nas", Host: "x", Port: 445, Share: "Public/Testing"}}
	cfg.SMB.ChunkSize = 1024
	svc := services.NewSMBServiceWithOpener(cfg, zap.NewNop(), func(host, share string) (services.ShareAPI, func(), error) {
		fk.mount = append(fk.mount, share)
		return fk, func() {}, nil
	})
	root := t.TempDir()
	h := NewCopyHandler(nil, svc, root, zap.NewNop())
	r := newProductionCopyRouterWith(h)

	w := postJSON(r, "/api/v1/copy/local", map[string]any{"source_path": "nas:docs/book.txt", "destination_path": "out/book.txt"})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	got, err := os.ReadFile(filepath.Join(root, "out", "book.txt"))
	require.NoError(t, err)
	require.Equal(t, "book", string(got), "the file must come from the sub-directory, not from the share root")
	require.Equal(t, []string{"Public"}, fk.mount, "only the share part is mounted (go-smb2 refuses a mount path with a slash)")

	for _, src := range []string{"nas:../Misc/book.txt", `nas:..\Misc\book.txt`, "nas:docs/../../Misc/book.txt"} {
		w = postJSON(r, "/api/v1/copy/local", map[string]any{"source_path": src, "destination_path": "out/x.txt", "overwrite": true})
		require.NotEqual(t, http.StatusOK, w.Code, src)
		b, _ := os.ReadFile(filepath.Join(root, "out", "x.txt"))
		require.False(t, bytes.Contains(b, []byte("OUTSIDE")), src)
	}
	for _, c := range fk.calls {
		require.True(t, c == "Testing" || strings.HasPrefix(c, "Testing/"), "share path %q left the sub-directory (%v)", c, fk.calls)
	}
}
