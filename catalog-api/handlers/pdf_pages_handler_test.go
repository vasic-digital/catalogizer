package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"catalogizer/database"
	"catalogizer/filesystem"
	"catalogizer/internal/media/models"
	"catalogizer/repository"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makePDF builds a syntactically valid, multi-page PDF entirely in Go,
// with a correct cross-reference table (byte offsets computed from the
// growing buffer) so poppler opens it WITHOUT relying on its
// repair/reconstruction fallback. Each page carries a tiny content
// stream (a stroked rectangle) so every page is genuinely renderable —
// a blank-page guess (§11.4.6) is avoided. Returns the PDF bytes.
//
// Object layout:
//
//	1            -> /Catalog
//	2            -> /Pages (Kids = page objects)
//	3,5,7,...    -> /Page objects (one per page)
//	4,6,8,...    -> the matching /Contents stream object
func makePDF(t *testing.T, numPages int) []byte {
	t.Helper()
	require.Greater(t, numPages, 0)

	var buf bytes.Buffer
	var offsets []int // offsets[k] = byte offset of object number (k+1)

	obj := func(body string) {
		offsets = append(offsets, buf.Len())
		buf.WriteString(body)
	}

	buf.WriteString("%PDF-1.4\n")

	// 1: catalog
	obj("1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n")

	// 2: pages tree
	var kids []string
	for i := 0; i < numPages; i++ {
		kids = append(kids, fmt.Sprintf("%d 0 R", 3+2*i))
	}
	obj(fmt.Sprintf("2 0 obj\n<< /Type /Pages /Kids [%s] /Count %d >>\nendobj\n",
		strings.Join(kids, " "), numPages))

	// Per-page: a /Page object + its /Contents stream object.
	content := "1 0 0 RG 5 w 20 20 160 160 re S\n"
	for i := 0; i < numPages; i++ {
		pageNo := 3 + 2*i
		contentNo := 4 + 2*i
		obj(fmt.Sprintf(
			"%d 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 200] /Contents %d 0 R /Resources << >> >>\nendobj\n",
			pageNo, contentNo))
		obj(fmt.Sprintf(
			"%d 0 obj\n<< /Length %d >>\nstream\n%sendstream\nendobj\n",
			contentNo, len(content), content))
	}

	// Cross-reference table.
	xrefOff := buf.Len()
	total := len(offsets) + 1 // +1 for the free object 0
	buf.WriteString(fmt.Sprintf("xref\n0 %d\n", total))
	buf.WriteString("0000000000 65535 f \n")
	for _, off := range offsets {
		buf.WriteString(fmt.Sprintf("%010d 00000 n \n", off))
	}
	buf.WriteString(fmt.Sprintf("trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n",
		total, xrefOff))

	return buf.Bytes()
}

// writePDF writes a freshly built numPages PDF into dir/name and returns
// the absolute path.
func writePDF(t *testing.T, dir, name string, numPages int) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, makePDF(t, numPages), 0o600))
	return path
}

func newPdfHandler(t *testing.T, db *database.DB) *PdfPagesHandler {
	t.Helper()
	return NewPdfPagesHandler(repository.NewMediaFileRepository(db), db, filesystem.NewDefaultClientFactory())
}

func newPdfRouter(h *PdfPagesHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	g := r.Group("/api/v1/entities")
	g.GET("/:id/pdf-pages", h.ListPdfPages)
	g.GET("/:id/pdf-pages/:n", h.GetPdfPage)
	return r
}

// TestPdfPages_CountAndRender is the primary RED->GREEN test: a real
// 3-page PDF must report total_pages=3 and render each in-range page to a
// decodable PNG, while out-of-range and malformed indices are rejected.
func TestPdfPages_CountAndRender(t *testing.T) {
	db, cleanup := setupEntityTestDB(t)
	defer cleanup()

	itemRepo := repository.NewMediaItemRepository(db)
	h := newPdfHandler(t, db)
	r := newPdfRouter(h)

	dir := t.TempDir()
	pdfPath := writePDF(t, dir, "book.pdf", 3)
	// Absolute local path: storage root base_path "/" so the stored path
	// IS the absolute path.
	itemID := insertComicItemOnRoot(t, db, itemRepo, "local", "/", pdfPath, "book.pdf")

	t.Run("count returns total_pages", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/entities/%d/pdf-pages", itemID), nil)
		r.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		var resp struct {
			TotalPages int `json:"total_pages"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.Equal(t, 3, resp.TotalPages)
	})

	for _, n := range []int{0, 1, 2} {
		t.Run(fmt.Sprintf("page %d renders a decodable PNG", n), func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/entities/%d/pdf-pages/%d", itemID, n), nil)
			r.ServeHTTP(w, req)

			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			assert.Equal(t, "image/png", w.Header().Get("Content-Type"))
			img, err := png.Decode(bytes.NewReader(w.Body.Bytes()))
			require.NoError(t, err, "rendered page %d must be a valid PNG", n)
			b := img.Bounds()
			assert.Positive(t, b.Dx(), "rendered page width must be > 0")
			assert.Positive(t, b.Dy(), "rendered page height must be > 0")
		})
	}

	t.Run("out-of-range page -> 404", func(t *testing.T) {
		for _, n := range []string{"3", "99"} {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/entities/%d/pdf-pages/%s", itemID, n), nil)
			r.ServeHTTP(w, req)
			assert.Equal(t, http.StatusNotFound, w.Code, "n=%s", n)
		}
	})

	t.Run("invalid page index -> 400", func(t *testing.T) {
		for _, n := range []string{"abc", "-1"} {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/entities/%d/pdf-pages/%s", itemID, n), nil)
			r.ServeHTTP(w, req)
			assert.Equal(t, http.StatusBadRequest, w.Code, "n=%s", n)
		}
	})
}

// TestPdfPages_NonPdfItem proves a non-PDF item is rejected rather than
// mis-served.
func TestPdfPages_NonPdfItem(t *testing.T) {
	db, cleanup := setupEntityTestDB(t)
	defer cleanup()

	itemRepo := repository.NewMediaItemRepository(db)
	h := newPdfHandler(t, db)
	r := newPdfRouter(h)

	itemID := insertComicItemOnRoot(t, db, itemRepo, "local", "/", "/quality/movie.mkv", "movie.mkv")

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/entities/%d/pdf-pages", itemID), nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
}

// TestPdfPages_NoFiles proves an entity with no linked files yields 404.
func TestPdfPages_NoFiles(t *testing.T) {
	db, cleanup := setupEntityTestDB(t)
	defer cleanup()

	itemRepo := repository.NewMediaItemRepository(db)
	h := newPdfHandler(t, db)
	r := newPdfRouter(h)

	ctx := context.Background()
	_, typeID, err := itemRepo.GetMediaTypeByName(ctx, "movie")
	require.NoError(t, err)
	itemID, err := itemRepo.Create(ctx, &models.MediaItem{
		MediaTypeID: typeID, Title: "orphan-pdf", Status: "detected",
	})
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/entities/%d/pdf-pages", itemID), nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
}

// TestPdfPages_StorageRootRelativePath is the multi-protocol resolution
// test: the PDF's stored path is RELATIVE to its storage root's base —
// exactly how an SMB share path (the real-world book corpus on the SMB
// share) and FTP/WebDAV/NFS paths are stored. The handler MUST resolve it
// through the ClientFactory against the storage root, not as a bare local
// path. Local protocol stands in for SMB (both are SeekableClient -> the
// random-access path) because a real SMB server is unavailable in a unit
// test (honest §11.4.3 gap — real-SMB rendering is verified on-device).
func TestPdfPages_StorageRootRelativePath(t *testing.T) {
	db, cleanup := setupEntityTestDB(t)
	defer cleanup()

	itemRepo := repository.NewMediaItemRepository(db)
	h := newPdfHandler(t, db)
	r := newPdfRouter(h)

	base := t.TempDir()
	// writePDF writes <base>/book.pdf; the STORED path is the bare relative
	// "book.pdf", resolved against the storage root's base_path (= base).
	writePDF(t, base, "book.pdf", 2)
	itemID := insertComicItemOnRoot(t, db, itemRepo, "local", base, "book.pdf", "book.pdf")

	t.Run("count resolves relative path via factory", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/entities/%d/pdf-pages", itemID), nil)
		r.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		var resp struct {
			TotalPages int `json:"total_pages"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.Equal(t, 2, resp.TotalPages)
	})

	t.Run("render page 0 over factory client", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/entities/%d/pdf-pages/0", itemID), nil)
		r.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.Equal(t, "image/png", w.Header().Get("Content-Type"))
		_, err := png.Decode(bytes.NewReader(w.Body.Bytes()))
		require.NoError(t, err)
	})
}

// TestPdfPages_BufferedFallback exercises the temp-file buffering path for
// streaming-only protocols (FTP/WebDAV/NFS): a client that gives only an
// io.Reader (streamingOnlyFactory, defined in comic_pages_handler_test.go).
// Proves the handler buffers the PDF to a temp file, opens it, and renders.
func TestPdfPages_BufferedFallback(t *testing.T) {
	db, cleanup := setupEntityTestDB(t)
	defer cleanup()

	itemRepo := repository.NewMediaItemRepository(db)

	pdfBytes := makePDF(t, 2)
	h := NewPdfPagesHandler(repository.NewMediaFileRepository(db), db, &streamingOnlyFactory{data: pdfBytes})
	r := newPdfRouter(h)

	itemID := insertComicItemOnRoot(t, db, itemRepo, "webdav", "/dav", "book.pdf", "book.pdf")

	t.Run("count via buffered temp file", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/entities/%d/pdf-pages", itemID), nil)
		r.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		var resp struct {
			TotalPages int `json:"total_pages"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.Equal(t, 2, resp.TotalPages)
	})

	t.Run("render page 1 via buffered temp file", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/entities/%d/pdf-pages/1", itemID), nil)
		r.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.Equal(t, "image/png", w.Header().Get("Content-Type"))
		_, err := png.Decode(bytes.NewReader(w.Body.Bytes()))
		require.NoError(t, err)
	})
}

// TestPdfPages_RoutesCoexistWithComic proves the /:id/pdf-pages routes can
// be registered on the SAME engine alongside the existing /:id/pages comic
// routes (sibling static segments under the /:id param) without a Gin
// routing panic — the exact coexistence main.go relies on.
func TestPdfPages_RoutesCoexistWithComic(t *testing.T) {
	db, cleanup := setupEntityTestDB(t)
	defer cleanup()

	itemRepo := repository.NewMediaItemRepository(db)
	pdfH := newPdfHandler(t, db)
	comicH := newComicHandler(t, db)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	g := r.Group("/api/v1/entities")
	// Register both families together; a conflicting wildcard would panic here.
	g.GET("/:id/pages", comicH.ListComicPages)
	g.GET("/:id/pages/:n", comicH.GetComicPage)
	g.GET("/:id/pdf-pages", pdfH.ListPdfPages)
	g.GET("/:id/pdf-pages/:n", pdfH.GetPdfPage)

	dir := t.TempDir()
	pdfPath := writePDF(t, dir, "book.pdf", 1)
	itemID := insertComicItemOnRoot(t, db, itemRepo, "local", "/", pdfPath, "book.pdf")

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/entities/%d/pdf-pages", itemID), nil)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
}
