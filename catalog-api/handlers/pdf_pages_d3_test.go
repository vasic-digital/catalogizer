package handlers

// D3 (docs/qa/live-conv-docs-subs-r1-20261104.md): /entities/:id/pdf-pages must never answer
// 500 with parser internals for an untrusted PDF, and must not parse the PDF in the API process.
// These tests drive the real gin router, a real migrated database, the real poppler tools and the
// repository's real PDF fixtures (services/testdata/pdf).

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/rc4"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"catalogizer/filesystem"
	"catalogizer/repository"
	"catalogizer/services"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pdfFromObjects assembles a PDF from numbered object bodies (1-based, in order) with a correct
// xref table, plus an optional extra trailer text.
func pdfFromObjects(objs []string, trailerExtra string) []byte {
	var buf bytes.Buffer
	buf.WriteString("%PDF-1.4\n")
	offs := make([]int, len(objs))
	for i, o := range objs {
		offs[i] = buf.Len()
		buf.WriteString(fmt.Sprintf("%d 0 obj\n%s\nendobj\n", i+1, o))
	}
	x := buf.Len()
	buf.WriteString(fmt.Sprintf("xref\n0 %d\n0000000000 65535 f \n", len(objs)+1))
	for _, o := range offs {
		buf.WriteString(fmt.Sprintf("%010d 00000 n \n", o))
	}
	buf.WriteString(fmt.Sprintf("trailer\n<< /Size %d /Root 1 0 R %s>>\nstartxref\n%d\n%%%%EOF\n", len(objs)+1, trailerExtra, x))
	return buf.Bytes()
}

// makeDistinctPDF builds an n-page 200x200 pt PDF whose page k draws a filled square of a
// different size, so every rendered page has different pixels.
func makeDistinctPDF(n int) []byte {
	kids := make([]string, n)
	for i := range kids {
		kids[i] = fmt.Sprintf("%d 0 R", 3+2*i)
	}
	objs := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", strings.Join(kids, " "), n),
	}
	for i := 0; i < n; i++ {
		c := fmt.Sprintf("0 0 0 rg 10 10 %d %d re f\n", 20+30*i, 20+30*i)
		objs = append(objs,
			fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 200] /Contents %d 0 R /Resources << >> >>", 4+2*i),
			fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", len(c), c))
	}
	return pdfFromObjects(objs, "")
}

// makeHugePagePDF declares a 14400x14400 pt page (the PDF maximum): 30000x30000 px at 150 dpi.
func makeHugePagePDF() []byte {
	c := "0 0 0 rg 10 10 100 100 re f\n"
	return pdfFromObjects([]string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 14400 14400] /Contents 4 0 R /Resources << >> >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", len(c), c),
	}, "")
}

// makeHugeCountPDF declares /Count 1000000 over a single real page.
func makeHugeCountPDF() []byte {
	c := "0 0 0 rg 10 10 100 100 re f\n"
	return pdfFromObjects([]string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1000000 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 200] /Contents 4 0 R /Resources << >> >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", len(c), c),
	}, "")
}

var pdfPad = []byte{0x28, 0xBF, 0x4E, 0x5E, 0x4E, 0x75, 0x8A, 0x41, 0x64, 0x00, 0x4E, 0x56, 0xFF, 0xFA, 0x01, 0x08,
	0x2E, 0x2E, 0x00, 0xB6, 0xD0, 0x68, 0x3E, 0x80, 0x2F, 0x0C, 0xA9, 0xFE, 0x64, 0x53, 0x69, 0x7A}

func padPW(pw string) []byte { return append([]byte(pw), pdfPad...)[:32] }

// makeEncryptedPDF builds a PDF with a standard security handler (RC4-40, revision 2) whose user
// password is non-empty, so a viewer that has no password must refuse it ("Incorrect password").
func makeEncryptedPDF() []byte {
	owner, user := "owner-pw", "user-pw"
	ok := md5.Sum(padPW(owner))
	oc, _ := rc4.NewCipher(ok[:5])
	O := make([]byte, 32)
	oc.XORKeyStream(O, padPW(user))
	P := int32(-4)
	id := []byte("0123456789abcdef")
	h := md5.New()
	h.Write(padPW(user))
	h.Write(O)
	var pb [4]byte
	binary.LittleEndian.PutUint32(pb[:], uint32(P))
	h.Write(pb[:])
	h.Write(id)
	key := h.Sum(nil)[:5]
	uc, _ := rc4.NewCipher(key)
	U := make([]byte, 32)
	uc.XORKeyStream(U, pdfPad)
	c := "0 0 0 rg 10 10 100 100 re f\n"
	return pdfFromObjects([]string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 200] /Contents 4 0 R /Resources << >> >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", len(c), c),
		fmt.Sprintf("<< /Filter /Standard /V 1 /R 2 /O <%s> /U <%s> /P %d >>", hex.EncodeToString(O), hex.EncodeToString(U), P),
	}, fmt.Sprintf("/Encrypt 5 0 R /ID [<%s><%s>] ", hex.EncodeToString(id), hex.EncodeToString(id)))
}

// setupD3 creates the DB/router and stores each named file (content) under base; returns ids.
func setupD3(t *testing.T, files map[string][]byte) (ids map[string]int64, get func(id int64, suffix string) *httptest.ResponseRecorder) {
	t.Helper()
	ids, r := setupD3R(t, files)
	get = func(id int64, suffix string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/entities/%d/pdf-pages%s", id, suffix), nil)
		r.ServeHTTP(w, req)
		return w
	}
	return ids, get
}

// setupD3R is setupD3 returning the router, for tests that need their own request context.
func setupD3R(t *testing.T, files map[string][]byte) (ids map[string]int64, r http.Handler) {
	t.Helper()
	db, cleanup := setupEntityTestDB(t)
	t.Cleanup(cleanup)
	itemRepo := repository.NewMediaItemRepository(db)
	h := newPdfHandler(t, db)
	r = newPdfRouter(h)
	base := t.TempDir()
	ids = map[string]int64{}
	for name, data := range files {
		require.NoError(t, os.WriteFile(filepath.Join(base, name), data, 0o600))
		ids[name] = insertComicItemOnRoot(t, db, itemRepo, "local", base, name, name)
	}
	return ids, r
}

type errBody struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
	Details string `json:"details"`
}

// assertNoInternals fails when a response leaks parser or path internals.
func assertNoInternals(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	body := strings.ToLower(w.Body.String())
	for _, bad := range []string{"fitz", "mupdf", "open pdf document", "poppler", "pdfinfo", "pdftoppm", "prlimit",
		"exit status", "/tmp", "catalogizer-pdf", "syntax error", "/home/", "signal:"} {
		assert.NotContains(t, body, bad, "response leaks internals: %s", w.Body.String())
	}
	var eb errBody
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &eb), w.Body.String())
	assert.False(t, eb.Success)
	assert.Empty(t, eb.Details, "no internal detail may be returned")
}

func TestD3_MalformedPDFs_Are422_NoInternals(t *testing.T) {
	valid := makeDistinctPDF(2)
	pngHeader := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")
	ids, get := setupD3(t, map[string][]byte{
		"junk.pdf":        []byte("this is not a pdf at all\n"),
		"zero.pdf":        {},
		"truncated.pdf":   valid[:len(valid)/3],
		"png-as.pdf":      pngHeader,
		"header-only.pdf": []byte("%PDF-1.4\n"),
	})
	for name, id := range ids {
		for _, suffix := range []string{"", "/0"} {
			t.Run(name+suffix, func(t *testing.T) {
				w := get(id, suffix)
				require.Equal(t, http.StatusUnprocessableEntity, w.Code, w.Body.String())
				assertNoInternals(t, w)
			})
		}
	}
}

func TestD3_EncryptedPDF_Is422_Generic(t *testing.T) {
	ids, get := setupD3(t, map[string][]byte{"enc.pdf": makeEncryptedPDF()})
	for _, suffix := range []string{"", "/0"} {
		w := get(ids["enc.pdf"], suffix)
		require.Equal(t, http.StatusUnprocessableEntity, w.Code, w.Body.String())
		assertNoInternals(t, w)
		assert.Contains(t, strings.ToLower(w.Body.String()), "encrypted")
	}
}

func TestD3_OversizedPage_Is413_CountStillWorks(t *testing.T) {
	ids, get := setupD3(t, map[string][]byte{"huge.pdf": makeHugePagePDF()})
	w := get(ids["huge.pdf"], "/0")
	require.Equal(t, http.StatusRequestEntityTooLarge, w.Code, w.Body.String())
	assertNoInternals(t, w)
	w = get(ids["huge.pdf"], "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
}

func TestD3_HugePageCount_Bounded(t *testing.T) {
	ids, get := setupD3(t, map[string][]byte{"count.pdf": makeHugeCountPDF()})
	for _, suffix := range []string{"", "/0", "/999999", "/1000000"} {
		w := get(ids["count.pdf"], suffix)
		assert.Less(t, w.Code, 500, "suffix %q -> %d %s", suffix, w.Code, w.Body.String())
		if w.Code >= 400 {
			assertNoInternals(t, w)
		}
	}
}

func TestD3_ValidPages_DistinctPixels_AndDimensions(t *testing.T) {
	ids, get := setupD3(t, map[string][]byte{"book.pdf": makeDistinctPDF(3)})
	id := ids["book.pdf"]
	w := get(id, "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var cnt struct {
		TotalPages int `json:"total_pages"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &cnt))
	require.Equal(t, 3, cnt.TotalPages)
	seen := map[string]int{}
	for n := 0; n < 3; n++ {
		w := get(id, fmt.Sprintf("/%d", n))
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.Equal(t, "image/png", w.Header().Get("Content-Type"))
		img, err := png.Decode(bytes.NewReader(w.Body.Bytes()))
		require.NoError(t, err)
		// 200 pt at 150 dpi = 416.67 px (same scale the former renderer used).
		assert.InDelta(t, 417, img.Bounds().Dx(), 1)
		assert.InDelta(t, 417, img.Bounds().Dy(), 1)
		sum := sha256.Sum256(w.Body.Bytes())
		seen[hex.EncodeToString(sum[:])] = n
	}
	assert.Len(t, seen, 3, "every page must render to different pixels")
	assert.Equal(t, http.StatusNotFound, get(id, "/3").Code)
	assert.Equal(t, http.StatusBadRequest, get(id, "/-1").Code)
	assert.Equal(t, http.StatusBadRequest, get(id, "/abc").Code)
}

func TestD3_RealFixture_Large146(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "services", "testdata", "pdf", "large.pdf"))
	require.NoError(t, err)
	ids, get := setupD3(t, map[string][]byte{"large.pdf": data})
	id := ids["large.pdf"]
	w := get(id, "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var cnt struct {
		TotalPages int `json:"total_pages"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &cnt))
	assert.Equal(t, 146, cnt.TotalPages)
	for _, n := range []int{0, 145} {
		w := get(id, fmt.Sprintf("/%d", n))
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		_, err := png.Decode(bytes.NewReader(w.Body.Bytes()))
		require.NoError(t, err)
	}
	assert.Equal(t, http.StatusNotFound, get(id, "/146").Code)
}

func TestD3_OptionLikeFilename(t *testing.T) {
	ids, get := setupD3(t, map[string][]byte{"-x.pdf": makeDistinctPDF(1)})
	w := get(ids["-x.pdf"], "/0")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	_, err := png.Decode(bytes.NewReader(w.Body.Bytes()))
	require.NoError(t, err)
}

// No temp file or directory of the handler or the renderer survives a request (success or error).
func TestD3_NoTempLeak(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	ids, get := setupD3(t, map[string][]byte{
		"ok.pdf":   makeDistinctPDF(2),
		"junk.pdf": []byte("junk"),
		"enc.pdf":  makeEncryptedPDF(),
	})
	for _, name := range []string{"ok.pdf", "junk.pdf", "enc.pdf"} {
		get(ids[name], "")
		get(ids[name], "/1")
	}
	var left []string
	filepath.Walk(tmp, func(p string, info os.FileInfo, err error) error { //nolint:errcheck
		if err == nil && p != tmp && (strings.Contains(p, "catalogizer-pdf")) {
			left = append(left, p)
		}
		return nil
	})
	assert.Empty(t, left, "temp artefacts survived")
}

// makeBoxPDF builds a one-page PDF with the given page-dictionary box entries.
func makeBoxPDF(boxes string) []byte {
	c := "0 0 0 rg 10 10 100 100 re f\n"
	return pdfFromObjects([]string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R " + boxes + " /Contents 4 0 R /Resources << >> >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", len(c), c),
	}, "")
}

// F1 (review round 1): pdfinfo reports the CropBox size, so pdftoppm must rasterise the CropBox
// too. MediaBox 9000x9000 pt with a CropBox of 100x100 pt used to pass the pixel check and then
// render 18750x18750 px (1.4 GB RSS).
func TestD3_CropBoxBomb_RendersCropBoxOnly(t *testing.T) {
	ids, get := setupD3(t, map[string][]byte{
		"crop.pdf":    makeBoxPDF("/MediaBox [0 0 9000 9000] /CropBox [0 0 100 100]"),
		"cropbig.pdf": makeBoxPDF("/MediaBox [0 0 100 100] /CropBox [0 0 14400 14400]"),
	})
	w := get(ids["crop.pdf"], "/0")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	img, err := png.Decode(bytes.NewReader(w.Body.Bytes()))
	require.NoError(t, err)
	assert.InDelta(t, 208, img.Bounds().Dx(), 2, "CropBox 100 pt at 150 dpi")
	assert.InDelta(t, 208, img.Bounds().Dy(), 2)
	// A huge CropBox over a small MediaBox is clipped to the MediaBox by poppler; it must stay bounded
	// and never be a 500.
	w = get(ids["cropbig.pdf"], "/0")
	assert.Less(t, w.Code, 500, w.Body.String())
}

// Status mapping per cause (review F2, F3, F4, F5): each observable status has its own test.
func TestD3_StatusPerCause(t *testing.T) {
	ids, r := setupD3R(t, map[string][]byte{"ok.pdf": makeDistinctPDF(2)})
	do := func(ctx context.Context, suffix string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/entities/%d/pdf-pages%s", ids["ok.pdf"], suffix), nil).WithContext(ctx)
		r.ServeHTTP(w, req)
		return w
	}

	t.Run("saturated readers -> 429 with Retry-After", func(t *testing.T) {
		var rel []func()
		for i := 0; i < services.PDFReaderSlots; i++ {
			f, ok := services.AcquirePDFReaderSlot()
			require.True(t, ok)
			rel = append(rel, f)
		}
		defer func() {
			for _, f := range rel {
				f()
			}
		}()
		for _, suffix := range []string{"", "/0"} {
			w := do(context.Background(), suffix)
			require.Equal(t, http.StatusTooManyRequests, w.Code, w.Body.String())
			assert.NotEmpty(t, w.Header().Get("Retry-After"))
			assertNoInternals(t, w)
		}
	})
	t.Run("slot released after a request", func(t *testing.T) {
		for i := 0; i < services.PDFReaderSlots+2; i++ {
			require.Equal(t, http.StatusOK, do(context.Background(), "").Code)
		}
	})
	t.Run("buffer bound -> 413", func(t *testing.T) {
		old := pdfMaxBufferBytes
		pdfMaxBufferBytes = 100
		defer func() { pdfMaxBufferBytes = old }()
		for _, suffix := range []string{"", "/0"} {
			w := do(context.Background(), suffix)
			require.Equal(t, http.StatusRequestEntityTooLarge, w.Code, w.Body.String())
			assertNoInternals(t, w)
		}
	})
	t.Run("poppler missing -> 503", func(t *testing.T) {
		bin := t.TempDir()
		pl, err := exec.LookPath("prlimit")
		require.NoError(t, err)
		require.NoError(t, os.Symlink(pl, filepath.Join(bin, "prlimit")))
		t.Setenv("PATH", bin)
		for _, suffix := range []string{"", "/0"} {
			w := do(context.Background(), suffix)
			require.Equal(t, http.StatusServiceUnavailable, w.Code, w.Body.String())
			assertNoInternals(t, w)
		}
	})
	t.Run("expired deadline -> 504", func(t *testing.T) {
		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
		defer cancel()
		for _, suffix := range []string{"", "/0"} {
			w := do(ctx, suffix)
			require.Equal(t, http.StatusGatewayTimeout, w.Code, w.Body.String())
			assertNoInternals(t, w)
		}
	})
	t.Run("client cancel -> 499 not 504", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		for _, suffix := range []string{"", "/0"} {
			w := do(ctx, suffix)
			require.Equal(t, 499, w.Code, w.Body.String())
		}
	})
}

// endlessClient streams zero bytes forever and counts what was read from it.
type endlessClient struct {
	streamingOnlyClient
	read *int64
}

type endlessReader struct{ n *int64 }

func (e endlessReader) Read(p []byte) (int, error) { *e.n += int64(len(p)); return len(p), nil }
func (e endlessReader) Close() error               { return nil }

func (c *endlessClient) ReadFile(context.Context, string) (io.ReadCloser, error) {
	return endlessReader{c.read}, nil
}

type endlessFactory struct{ read *int64 }

func (f *endlessFactory) CreateClient(*filesystem.StorageConfig) (filesystem.FileSystemClient, error) {
	return &endlessClient{read: f.read}, nil
}
func (f *endlessFactory) SupportedProtocols() []string { return []string{"webdav"} }

// The buffer bound must stop the COPY, not just reject afterwards (review F5/RB): an endless
// source is read only up to the bound (plus one read chunk) and the answer is 413.
func TestD3_BufferBoundStopsTheCopy(t *testing.T) {
	db, cleanup := setupEntityTestDB(t)
	defer cleanup()
	itemRepo := repository.NewMediaItemRepository(db)
	var read int64
	h := NewPdfPagesHandler(repository.NewMediaFileRepository(db), db, &endlessFactory{read: &read})
	r := newPdfRouter(h)
	id := insertComicItemOnRoot(t, db, itemRepo, "webdav", "/dav", "endless.pdf", "endless.pdf")
	old := pdfMaxBufferBytes
	pdfMaxBufferBytes = 1000
	defer func() { pdfMaxBufferBytes = old }()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", fmt.Sprintf("/api/v1/entities/%d/pdf-pages", id), nil).WithContext(ctx))
	require.Equal(t, http.StatusRequestEntityTooLarge, w.Code, w.Body.String())
	assert.LessOrEqual(t, read, int64(1001), "the copy must stop at the bound")
}

// blockingClient serves a valid PDF but only after gate is closed; it announces each read start.
type blockingClient struct {
	streamingOnlyClient
	started chan<- struct{}
	gate    <-chan struct{}
}

func (c *blockingClient) ReadFile(context.Context, string) (io.ReadCloser, error) {
	c.started <- struct{}{}
	<-c.gate
	return io.NopCloser(bytes.NewReader(c.data)), nil
}

type blockingFactory struct {
	data    []byte
	started chan struct{}
	gate    chan struct{}
}

func (f *blockingFactory) CreateClient(*filesystem.StorageConfig) (filesystem.FileSystemClient, error) {
	return &blockingClient{streamingOnlyClient: streamingOnlyClient{data: f.data}, started: f.started, gate: f.gate}, nil
}
func (f *blockingFactory) SupportedProtocols() []string { return []string{"webdav"} }

// R2-1: the reader slot is HELD for the whole request (buffer + count + render), not only while it
// is taken. PDFReaderSlots requests blocked in the storage read must make the next one 429 for
// both endpoints, and once they finish the slots are free again (200).
func TestD3_SlotHeldWhileWorkRuns(t *testing.T) {
	for _, suffix := range []string{"", "/0"} {
		t.Run("endpoint"+suffix, func(t *testing.T) {
			db, cleanup := setupEntityTestDB(t)
			defer cleanup()
			itemRepo := repository.NewMediaItemRepository(db)
			f := &blockingFactory{data: makeDistinctPDF(1), started: make(chan struct{}, 16), gate: make(chan struct{})}
			r := newPdfRouter(NewPdfPagesHandler(repository.NewMediaFileRepository(db), db, f))
			id := insertComicItemOnRoot(t, db, itemRepo, "webdav", "/dav", "slow.pdf", "slow.pdf")
			do := func() *httptest.ResponseRecorder {
				w := httptest.NewRecorder()
				r.ServeHTTP(w, httptest.NewRequest("GET", fmt.Sprintf("/api/v1/entities/%d/pdf-pages%s", id, suffix), nil))
				return w
			}

			done := make(chan *httptest.ResponseRecorder, services.PDFReaderSlots)
			for i := 0; i < services.PDFReaderSlots; i++ {
				go func() { done <- do() }()
			}
			for i := 0; i < services.PDFReaderSlots; i++ {
				select {
				case <-f.started:
				case <-time.After(20 * time.Second):
					t.Fatal("in-flight request never reached the storage read")
				}
			}
			w := do()
			require.Equal(t, http.StatusTooManyRequests, w.Code, w.Body.String())
			assert.NotEmpty(t, w.Header().Get("Retry-After"))

			close(f.gate)
			for i := 0; i < services.PDFReaderSlots; i++ {
				select {
				case ww := <-done:
					assert.Equal(t, http.StatusOK, ww.Code, ww.Body.String())
				case <-time.After(30 * time.Second):
					t.Fatal("blocked request never finished")
				}
			}
			assert.Equal(t, http.StatusOK, do().Code, "slots must be free again")
		})
	}
}
