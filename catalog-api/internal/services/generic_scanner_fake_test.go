package services

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"catalogizer/filesystem"
)

// memFS is the in-memory FileSystemClient of the generic-scanner UNIT tests (PA-03). It is a fake and lives only in unit tests
// (constitution 11.4.27); the integration test of this scanner runs against the real FTP and WebDAV containers.
type memFS struct {
	mu       sync.Mutex
	dirs     map[string][]*filesystem.FileInfo // key: cleaned directory path
	listErr  map[string]error                  // error returned when listing that directory
	listHook func(dir string)                  // called at the start of every ListDirectory
	// returnHook is called with the number of entries a listing RETURNS (after the list limit the scanner passed was applied)
	returnHook func(dir string, returned int)
	listed     []string
	connect    error

	inflight, maxInflight int32
	listDelay             time.Duration

	// every mutating call is counted so the read-only guarantee can be asserted
	mutations int32
}

func newMemFS() *memFS {
	return &memFS{dirs: map[string][]*filesystem.FileInfo{}, listErr: map[string]error{}}
}

var t0 = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

func (m *memFS) addDir(dir string) {
	dir = path.Clean(dir)
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.dirs[dir]; !ok {
		m.dirs[dir] = nil
	}
}

func (m *memFS) addFile(dir, name string, size int64, mod time.Time) {
	dir = path.Clean(dir)
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dirs[dir] = append(m.dirs[dir], &filesystem.FileInfo{Name: name, Size: size, ModTime: mod})
}

func (m *memFS) addSubdir(dir, name string) {
	dir = path.Clean(dir)
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dirs[dir] = append(m.dirs[dir], &filesystem.FileInfo{Name: name, IsDir: true, ModTime: t0})
	child := path.Join(dir, name)
	if _, ok := m.dirs[child]; !ok {
		m.dirs[child] = nil
	}
}

func (m *memFS) setFile(dir, name string, size int64, mod time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, e := range m.dirs[path.Clean(dir)] {
		if e.Name == name {
			e.Size, e.ModTime = size, mod
		}
	}
}

func (m *memFS) Connect(context.Context) error    { return m.connect }
func (m *memFS) Disconnect(context.Context) error { return nil }
func (m *memFS) IsConnected() bool                { return true }
func (m *memFS) TestConnection(context.Context) error {
	return nil
}
func (m *memFS) GetProtocol() string    { return "memfs" }
func (m *memFS) GetConfig() interface{} { return nil }

func (m *memFS) ListDirectory(ctx context.Context, p string) ([]*filesystem.FileInfo, error) {
	cur := atomic.AddInt32(&m.inflight, 1)
	defer atomic.AddInt32(&m.inflight, -1)
	for {
		mx := atomic.LoadInt32(&m.maxInflight)
		if cur <= mx || atomic.CompareAndSwapInt32(&m.maxInflight, mx, cur) {
			break
		}
	}
	if m.listHook != nil {
		m.listHook(p)
	}
	if m.listDelay > 0 {
		select {
		case <-time.After(m.listDelay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	p = path.Clean(p)
	m.mu.Lock()
	defer m.mu.Unlock()
	m.listed = append(m.listed, p)
	if err, ok := m.listErr[p]; ok {
		return nil, err
	}
	ents, ok := m.dirs[p]
	if !ok {
		return nil, fmt.Errorf("list %s: %w", p, os.ErrNotExist)
	}
	out := make([]*filesystem.FileInfo, 0, len(ents))
	for _, e := range ents {
		c := *e
		out = append(out, &c)
	}
	// Like a client that honours filesystem.WithListLimit: return at most that many entries.
	if lim, ok := filesystem.ListLimit(ctx); ok && len(out) > lim {
		out = out[:lim]
	}
	if m.returnHook != nil {
		m.returnHook(p, len(out))
	}
	return out, nil
}

func (m *memFS) listedPaths() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := append([]string(nil), m.listed...)
	sort.Strings(out)
	return out
}

var errMutation = errors.New("memfs: mutation reached the inner client")

func (m *memFS) mutate() error { atomic.AddInt32(&m.mutations, 1); return errMutation }

func (m *memFS) ReadFile(context.Context, string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("x")), nil
}
func (m *memFS) WriteFile(context.Context, string, io.Reader) error { return m.mutate() }
func (m *memFS) GetFileInfo(context.Context, string) (*filesystem.FileInfo, error) {
	return &filesystem.FileInfo{}, nil
}
func (m *memFS) FileExists(context.Context, string) (bool, error) { return true, nil }
func (m *memFS) DeleteFile(context.Context, string) error         { return m.mutate() }
func (m *memFS) CopyFile(context.Context, string, string) error   { return m.mutate() }
func (m *memFS) CreateDirectory(context.Context, string) error    { return m.mutate() }
func (m *memFS) DeleteDirectory(context.Context, string) error    { return m.mutate() }

var _ filesystem.FileSystemClient = (*memFS)(nil)
