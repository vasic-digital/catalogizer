package services

import (
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"testing/fstest"

	"catalogizer/internal/config"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// memShare is an in-process share (unit-class: it stands in for the SMB server only; SplitShare, the sub-directory confinement and every
// SMBService method above the mount are the real code). It records every share-relative path the service asks for.
type memShare struct {
	mu    sync.Mutex
	fsys  fstest.MapFS
	calls []string
	wrote map[string][]byte
}

func (m *memShare) rec(n string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, n)
	if n == "" {
		return "."
	}
	return n
}
func (m *memShare) ReadDir(n string) ([]os.FileInfo, error) {
	ents, err := fs.ReadDir(m.fsys, m.rec(n))
	if err != nil {
		return nil, err
	}
	var out []os.FileInfo
	for _, e := range ents {
		fi, _ := e.Info()
		out = append(out, fi)
	}
	return out, nil
}

type memFile struct{ fs.File }

func (f memFile) Seek(o int64, w int) (int64, error) { return f.File.(io.Seeker).Seek(o, w) }
func (f memFile) Stat() (os.FileInfo, error)         { return f.File.Stat() }

func (m *memShare) Open(n string) (RemoteFile, error) {
	f, err := m.fsys.Open(m.rec(n))
	if err != nil {
		return nil, err
	}
	return memFile{f}, nil
}

type nopWC struct {
	io.Writer
	done func()
}

func (n nopWC) Close() error { n.done(); return nil }

func (m *memShare) Create(n string) (io.WriteCloser, error) {
	m.rec(n)
	var sb strings.Builder
	return nopWC{&sb, func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		m.wrote[n] = []byte(sb.String())
	}}, nil
}
func (m *memShare) Stat(n string) (os.FileInfo, error) { return fs.Stat(m.fsys, m.rec(n)) }
func (m *memShare) Mkdir(n string, _ os.FileMode) error {
	m.rec(n)
	return nil
}
func (m *memShare) Remove(n string) error { m.rec(n); return nil }
func (m *memShare) Umount() error         { return nil }

func newMem() *memShare {
	return &memShare{wrote: map[string][]byte{}, fsys: fstest.MapFS{
		"Testing/docs/book.txt": {Data: []byte("book")},
		"Testing/docs":          {Mode: fs.ModeDir},
		"Testing":               {Mode: fs.ModeDir},
		"Misc/secret.txt":       {Data: []byte("secret")},
		"Misc":                  {Mode: fs.ModeDir},
		"docs/book.txt":         {Data: []byte("plain-book")},
		"docs":                  {Mode: fs.ModeDir},
		"escape.txt":            {Data: []byte("outside")},
	}}
}

func svcFor(t *testing.T, m *memShare, share string, opened *[]string) *SMBService {
	t.Helper()
	cfg := &config.Config{}
	cfg.SMB.Hosts = []config.SMBHost{{Name: "h", Host: "x", Port: 445, Share: share}}
	cfg.SMB.ChunkSize = 1024
	cfg.Catalog.TempDir = t.TempDir()
	return NewSMBServiceWithOpener(cfg, zap.NewNop(), func(host, shareName string) (ShareAPI, func(), error) {
		if opened != nil {
			*opened = append(*opened, shareName)
		}
		return m, func() {}, nil
	})
}

func inside(t *testing.T, m *memShare, root string) {
	t.Helper()
	for _, c := range m.calls {
		cc := path.Clean(c)
		require.True(t, cc == root || strings.HasPrefix(cc, root+"/"), "share path %q escaped %q (all: %v)", c, root, m.calls)
	}
}

func TestSMBSubdir_RoutedOperationsAreConfined(t *testing.T) {
	m := newMem()
	var opened []string
	s := svcFor(t, m, "Public/Testing", &opened)
	hostile := []string{"", ".", "..", "../..", `..\..`, "docs/../../..", "/docs", "../Misc/secret.txt", `..\Misc\secret.txt`}

	for _, p := range hostile {
		_, _ = s.ListFiles("h", p)
		_, _ = s.FileExists("h", p)
		_, _ = s.DirectoryExists("h", p)
		_ = s.CreateDirectory("h", p)
		_ = s.DeleteDirectory("h", p)
		if rc, _, err := s.StreamFile("h", p); err == nil {
			rc.Close()
		}
		_ = s.DownloadFile("h", p, filepath.Join(t.TempDir(), "o"))
	}
	local := filepath.Join(t.TempDir(), "up.txt")
	require.NoError(t, os.WriteFile(local, []byte("up"), 0o600))
	require.NoError(t, s.UploadFile("h", local, "../escape.txt"))
	require.NoError(t, s.UploadFile("h", local, `..\..\escape2.txt`))
	require.Contains(t, m.wrote, "Testing/escape.txt")
	require.Contains(t, m.wrote, "Testing/escape2.txt")
	require.NotEmpty(t, m.calls)
	inside(t, m, "Testing")
	for _, o := range opened {
		require.Equal(t, "Public", o, "only the share part is mounted")
	}
	require.NotEmpty(t, opened)
}

func TestSMBSubdir_RoutedContentComesFromSubtree(t *testing.T) {
	m := newMem()
	s := svcFor(t, m, "Public/Testing", nil)
	rc, size, err := s.StreamFile("h", "docs/book.txt")
	require.NoError(t, err)
	b, _ := io.ReadAll(rc)
	rc.Close()
	require.Equal(t, "book", string(b))
	require.EqualValues(t, 4, size)
	_, _, err = s.StreamFile("h", "../Misc/secret.txt")
	require.Error(t, err, "a file outside the subtree is unreachable")
	files, err := s.ListFiles("h", "")
	require.NoError(t, err)
	require.Len(t, files, 1)
	require.Equal(t, "docs", files[0].Name())

	dl := filepath.Join(t.TempDir(), "d", "book.txt")
	require.NoError(t, s.DownloadFile("h", "docs/book.txt", dl))
	got, _ := os.ReadFile(dl)
	require.Equal(t, "book", string(got))

	// CopyFile: source sub root -> destination sub root, through the real temp-file path
	require.NoError(t, s.CopyFile("h", "docs/book.txt", "h", "copy/book.txt"))
	require.Equal(t, "book", string(m.wrote["Testing/copy/book.txt"]))
	inside(t, m, "Testing")
}

func TestSMBSubdir_PlainShareUnchangedAndBadPathRefused(t *testing.T) {
	m := newMem()
	var opened []string
	s := svcFor(t, m, "Public", &opened)
	rc, _, err := s.StreamFile("h", "docs/book.txt")
	require.NoError(t, err)
	b, _ := io.ReadAll(rc)
	rc.Close()
	require.Equal(t, "plain-book", string(b))
	require.Equal(t, []string{"docs/book.txt"}, m.calls, "a plain share passes the path through unchanged")

	for _, bad := range []string{"Public/../Misc", `Public\Testing`, "Public//x", "IPC$", "C$/x"} {
		opened = nil
		bs := svcFor(t, m, bad, &opened)
		_, err := bs.ListFiles("h", "")
		require.Error(t, err, bad)
		_, _, err = bs.StreamFile("h", "x")
		require.Error(t, err, bad)
		require.Error(t, bs.DownloadFile("h", "x", filepath.Join(t.TempDir(), "o")), bad)
		require.Empty(t, opened, "%s: an invalid root path is refused before any mount", bad)
	}
}
