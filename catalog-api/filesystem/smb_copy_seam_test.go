package filesystem

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"strings"
	"testing"
	"time"
)

// Hermetic tests of the real copy logic (copyFileOn) over an in-memory copyFS. Gate-logic/unit class (11.4.289(C)): they prove the
// error-handling decisions of the copy path; the live_smb tests prove behaviour against a real server.

type fakeInfo struct {
	name string
	mode fs.FileMode
}

func (i fakeInfo) Name() string       { return i.name }
func (i fakeInfo) Size() int64        { return 0 }
func (i fakeInfo) Mode() fs.FileMode  { return i.mode }
func (i fakeInfo) ModTime() time.Time { return time.Time{} }
func (i fakeInfo) IsDir() bool        { return i.mode.IsDir() }
func (i fakeInfo) Sys() any           { return nil }

type fakeFile struct {
	r        io.Reader
	buf      bytes.Buffer
	name     string
	writeErr error
	closeErr error
	closed   int
	info     fs.FileInfo
	onWrite  func()
}

func (f *fakeFile) Read(p []byte) (int, error) { return f.r.Read(p) }
func (f *fakeFile) Write(p []byte) (int, error) {
	if f.onWrite != nil {
		f.onWrite()
	}
	if f.writeErr != nil {
		return 0, f.writeErr
	}
	return f.buf.Write(p)
}
func (f *fakeFile) Close() error               { f.closed++; return f.closeErr }
func (f *fakeFile) Stat() (fs.FileInfo, error) { return f.info, nil }

type removeCall struct {
	name    string
	ctxErr  error
	hasDL   bool
	dl      time.Time
	created bool
}

type fakeFS struct {
	files      map[string]*fakeFile
	infos      map[string]fs.FileInfo
	srcBody    string
	tmpWriteEr error
	tmpCloseEr error
	srcCloseEr error
	onWrite    func()
	removes    *[]removeCall
	renames    *[][2]string
	renameErr  error
	cx         context.Context
	removeFail string // Remove of this name returns an error
	creates    *[]createCall
}

type createCall struct {
	name string
	dl   time.Time
	has  bool
}

func (s *fakeFS) Open(n string) (copyFile, error) {
	f := &fakeFile{r: strings.NewReader(s.srcBody), name: n, closeErr: s.srcCloseEr, info: fakeInfo{n, 0o644}}
	s.files["src:"+n] = f
	return f, nil
}
func (s *fakeFS) Create(n string) (copyFile, error) {
	dl, has := s.cx.Deadline()
	*s.creates = append(*s.creates, createCall{name: n, dl: dl, has: has})
	f := &fakeFile{name: n, writeErr: s.tmpWriteEr, closeErr: s.tmpCloseEr, onWrite: s.onWrite}
	s.files["tmp:"+n] = f
	return f, nil
}
func (s *fakeFS) Stat(n string) (fs.FileInfo, error) {
	if i, ok := s.infos[n]; ok {
		return i, nil
	}
	return nil, fs.ErrNotExist
}
func (s *fakeFS) Remove(n string) error {
	dl, has := s.cx.Deadline()
	*s.removes = append(*s.removes, removeCall{name: n, ctxErr: s.cx.Err(), hasDL: has, dl: dl})
	if s.removeFail != "" && n == s.removeFail {
		return errors.New("remove failed")
	}
	return nil
}
func (s *fakeFS) Rename(o, n string) error {
	*s.renames = append(*s.renames, [2]string{o, n})
	return s.renameErr
}

type harness struct {
	base    *fakeFS
	removes []removeCall
	renames [][2]string
	creates []createCall
}

func newHarness() *harness {
	h := &harness{base: &fakeFS{files: map[string]*fakeFile{}, infos: map[string]fs.FileInfo{}, srcBody: "data"}}
	h.base.removes, h.base.renames, h.base.creates = &h.removes, &h.renames, &h.creates
	return h
}

func (h *harness) run(ctx context.Context, src, dst string) error {
	return copyFileOn(ctx, func(cx context.Context) copyFS {
		c := *h.base
		c.cx = cx
		return &c
	}, src, dst, src, dst)
}

func TestSeamSamePathCleaned(t *testing.T) {
	for _, p := range [][2]string{{"x", "./x"}, {"a/../x", "x"}, {"a//x", "a/x"}, {`a\x`, "a/x"}, {"a/x", `a\x`}} {
		h := newHarness()
		if err := h.run(context.Background(), p[0], p[1]); !errors.Is(err, ErrCopySameFile) {
			t.Fatalf("%v: err=%v want ErrCopySameFile", p, err)
		}
	}
}

func TestSeamReadOnlyDestinationRefused(t *testing.T) {
	h := newHarness()
	h.base.infos["d"] = fakeInfo{"d", 0o444}
	err := h.run(context.Background(), "s", "d")
	if !errors.Is(err, ErrCopyDestReadOnly) || len(h.removes) != 0 || len(h.renames) != 0 || len(h.base.files) != 1 {
		t.Fatalf("err=%v removes=%v renames=%v files=%d", err, h.removes, h.renames, len(h.base.files))
	}
}

func TestSeamSuccessReplacesAndNamesTemp(t *testing.T) {
	h := newHarness()
	h.base.infos["d"] = fakeInfo{"d", 0o644}
	if err := h.run(context.Background(), "s", "d"); err != nil {
		t.Fatal(err)
	}
	if len(h.renames) != 1 || h.renames[0][1] != "d" || !strings.HasPrefix(h.renames[0][0], ".copy-") || !strings.HasSuffix(h.renames[0][0], ".tmp") {
		t.Fatalf("renames=%v", h.renames)
	}
	if len(h.removes) != 1 || h.removes[0].name != "d" {
		t.Fatalf("removes=%v", h.removes)
	}
}

func assertTempRemoved(t *testing.T, h *harness, err error, wantErr error) {
	t.Helper()
	if err == nil || (wantErr != nil && !errors.Is(err, wantErr)) {
		t.Fatalf("err=%v want %v", err, wantErr)
	}
	if len(h.renames) != 0 {
		t.Fatalf("destination must not be replaced: %v", h.renames)
	}
	if len(h.removes) != 1 || !strings.HasPrefix(h.removes[0].name, ".copy-") {
		t.Fatalf("temp must be removed exactly once, removes=%v", h.removes)
	}
}

func TestSeamChunkWriteErrorRemovesTemp(t *testing.T) { // kills R6
	h := newHarness()
	we := errors.New("disk full")
	h.base.tmpWriteEr = we
	err := h.run(context.Background(), "s", "d")
	assertTempRemoved(t, h, err, we)
}

func TestSeamTempCloseErrorRemovesTemp(t *testing.T) { // kills R7
	h := newHarness()
	ce := errors.New("close failed")
	h.base.tmpCloseEr = ce
	err := h.run(context.Background(), "s", "d")
	assertTempRemoved(t, h, err, ce)
}

func TestSeamSourceCloseErrorFailsBeforeReplace(t *testing.T) { // kills R4
	h := newHarness()
	h.base.infos["d"] = fakeInfo{"d", 0o644}
	ce := errors.New("src close failed")
	h.base.srcCloseEr = ce
	err := h.run(context.Background(), "s", "d")
	if !errors.Is(err, ce) || len(h.renames) != 0 {
		t.Fatalf("err=%v renames=%v", err, h.renames)
	}
	if len(h.removes) != 1 || h.removes[0].name == "d" {
		t.Fatalf("only the temp may be removed, never the destination: %v", h.removes)
	}
}

// Cancel arriving with the last chunk (after EOF) must fail the copy and keep the destination (kills M5).
func TestSeamCancelAtEOFFailsBeforeReplace(t *testing.T) {
	h := newHarness()
	h.base.infos["d"] = fakeInfo{"d", 0o644}
	ctx, cancel := context.WithCancel(context.Background())
	h.base.onWrite = cancel
	err := h.run(ctx, "s", "d")
	assertTempRemoved(t, h, err, context.Canceled)
}

func TestCopyChunkedEOFChecksContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	r := eofWithData{}
	err := copyChunked(ctx, writerFunc(func(p []byte) (int, error) { cancel(); return len(p), nil }), r)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v", err)
	}
}

// eofWithData returns the last bytes together with io.EOF, so no later loop-top check can see the cancel.
type eofWithData struct{}

func (eofWithData) Read(p []byte) (int, error) { p[0] = 'x'; return 1, io.EOF }

type writerFunc func([]byte) (int, error)

func (w writerFunc) Write(p []byte) (int, error) { return w(p) }

// SMBCOPYR-m3: cleanup must not run on the expired caller context.
func TestSeamCleanupUsesFreshBoundedContext(t *testing.T) {
	h := newHarness()
	h.base.tmpWriteEr = errors.New("boom")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	h.base.onWrite = func() { time.Sleep(40 * time.Millisecond) }
	_ = h.run(ctx, "s", "d")
	if len(h.removes) != 1 {
		t.Fatalf("removes=%v", h.removes)
	}
	if h.removes[0].ctxErr != nil || !h.removes[0].hasDL {
		t.Fatalf("cleanup ctx must be live and bounded: %+v", h.removes[0])
	}
	// F1 (kills X7: cleanup on the transfer ctx): the cleanup ctx must differ from the transfer ctx and carry its own ~copyCleanupBudget
	// deadline, not the transfer deadline (caller + 15s).
	if len(h.creates) != 1 || !h.creates[0].has {
		t.Fatalf("creates=%v", h.creates)
	}
	if h.removes[0].dl.Equal(h.creates[0].dl) {
		t.Fatalf("cleanup must not reuse the transfer context deadline: %v", h.removes[0].dl)
	}
	if rem := time.Until(h.removes[0].dl); rem > copyCleanupBudget+time.Second {
		t.Fatalf("cleanup deadline must be ~%v from now, got %v", copyCleanupBudget, rem)
	}
}

// F2 (kills X3: temp not beside the destination): the temp lives in the destination's directory.
func TestSeamTempBesideDestinationInSubdirectory(t *testing.T) {
	h := newHarness()
	h.base.infos["a/b/d"] = fakeInfo{"d", 0o644}
	if err := h.run(context.Background(), "a/s", "a/b/d"); err != nil {
		t.Fatal(err)
	}
	if len(h.creates) != 1 || !strings.HasPrefix(h.creates[0].name, "a/b/.copy-") {
		t.Fatalf("creates=%v", h.creates)
	}
	if len(h.renames) != 1 || !strings.HasPrefix(h.renames[0][0], "a/b/.copy-") || h.renames[0][1] != "a/b/d" {
		t.Fatalf("renames=%v", h.renames)
	}
}

// F3 (kills X8: no temp cleanup when removing the old destination fails).
func TestSeamDestinationRemoveFailureRemovesTemp(t *testing.T) {
	h := newHarness()
	h.base.infos["d"] = fakeInfo{"d", 0o644}
	h.base.removeFail = "d"
	err := h.run(context.Background(), "s", "d")
	if err == nil || len(h.renames) != 0 {
		t.Fatalf("err=%v renames=%v", err, h.renames)
	}
	if len(h.removes) != 2 || h.removes[0].name != "d" || !strings.HasPrefix(h.removes[1].name, ".copy-") {
		t.Fatalf("destination remove then temp remove expected, removes=%v", h.removes)
	}
}

// SMBCOPYR-m4: the failed-rename error must not carry the server-relative temp path.
func TestSeamRenameErrorHidesServerPath(t *testing.T) {
	h := newHarness()
	h.base.renameErr = errors.New("nope")
	err := h.run(context.Background(), "secret/dir/s", "secret/dir/d")
	if err == nil || strings.Contains(err.Error(), "secret/dir/.copy-") || !strings.Contains(err.Error(), ".copy-") {
		t.Fatalf("err=%v", err)
	}
}
