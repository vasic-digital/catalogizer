package handlers

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"catalogizer/database"
	"catalogizer/filesystem"
	"catalogizer/internal/logging"
	root_models "catalogizer/models"
	"catalogizer/repository"
	"catalogizer/services"
	"catalogizer/utils"

	"github.com/gin-gonic/gin"
)

// PdfPagesHandler serves a PDF book one rendered page at a time so a
// reader client (e.g. the Android TV book reader) can page through a PDF
// without the client needing a PDF rasteriser of its own.
//
// Multi-protocol storage: a book's file path stored in the DB is RELATIVE
// to its storage root (an SMB share path, an FTP/WebDAV/NFS path, or a
// local base-path subtree) — it is NOT an absolute local filesystem path.
// The handler therefore resolves the PDF through the SAME
// filesystem.ClientFactory the StreamHandler + ComicPagesHandler use,
// opening the file over whatever protocol the storage root declares (SMB,
// FTP, NFS, WebDAV, local). Opening it with a bare local path (as a naive
// version would) only works for local-absolute paths and fails for every
// book on an SMB/FTP/NFS/WebDAV share — the exact bug the comic handler
// was fixed for.
//
// Rendering (D3): poppler (pdfinfo, pdftoppm) run as isolated child processes under prlimit via
// services.PDFPageCount / services.RenderPDFPagePNG, the same isolation as the CAT-002 text
// extraction. No PDF parser runs in the API process (the former MuPDF/go-fitz renderer was removed).
// Poppler needs the WHOLE document, so the handler buffers the PDF once to a private temp file
// (bounded to disk, not RAM, §12.6, and to pdfMaxBufferBytes) and renders ONE page per request
// (§12.6). The temp file is removed on cleanup (§11.4.14). A file poppler cannot open is a client
// error (422) with a generic body; parser detail is logged, never returned.
//
// Page indexing is ZERO-BASED: GET .../pdf-pages/:n expects 0-based n in
// the closed range [0, total_pages-1].
type PdfPagesHandler struct {
	fileRepo      *repository.MediaFileRepository
	db            *database.DB
	clientFactory filesystem.ClientFactory
}

// pdfRenderDPI is the rasterisation resolution for a rendered page. 150
// DPI on a typical page gives a crisp, legible image for a TV reader while
// keeping each rendered PNG bounded in size (§12.6).
const pdfRenderDPI = services.PDFRenderDPI

// pdfMaxBufferBytes bounds the PDF copied to the temp file; a larger document is refused (413).
// Known limit: every page view re-copies the document (no cache); concurrency is bounded by
// services.PDFReaderSlots (429 when saturated).
var pdfMaxBufferBytes int64 = 1 << 30 // var so tests can lower it

var errPDFTooBig = errors.New("pdf file exceeds buffer bound")

// NewPdfPagesHandler constructs the handler. It depends on the same three
// collaborators the ComicPagesHandler uses: the MediaFileRepository
// (resolves an entity id to its concrete file via
// GetPrimaryStreamableFile — the same path StreamEntity uses), the
// *database.DB (looks up the file's storage root: protocol, host, share,
// credentials), and the filesystem.ClientFactory (opens the file bytes
// over the storage root's protocol — SMB/FTP/NFS/WebDAV/local).
func NewPdfPagesHandler(fileRepo *repository.MediaFileRepository, db *database.DB, clientFactory filesystem.ClientFactory) *PdfPagesHandler {
	return &PdfPagesHandler{fileRepo: fileRepo, db: db, clientFactory: clientFactory}
}

// resolvePDF parses the :id param, resolves the entity's primary file
// through the same GetPrimaryStreamableFile path StreamEntity uses, and
// verifies the resolved file is a PDF. On any failure it writes the HTTP
// error response and returns ok=false.
//
// The PDF path is the DB-resolved item path, never derived from user
// input, so there is no path-traversal surface from the :id.
func (h *PdfPagesHandler) resolvePDF(c *gin.Context) (sf *repository.StreamableFile, ok bool) {
	ctx := c.Request.Context()

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		utils.SendErrorResponse(c, http.StatusBadRequest, "Invalid entity ID", nil)
		return nil, false
	}

	primary, err := h.fileRepo.GetPrimaryStreamableFile(ctx, id)
	if err != nil {
		if err == repository.ErrNoStreamableFile {
			utils.SendErrorResponse(c, http.StatusNotFound, "No file available for this entity", nil)
			return nil, false
		}
		respondPDFError(c, "resolve", err)
		return nil, false
	}

	if !strings.EqualFold(filepath.Ext(primary.Path), ".pdf") {
		utils.SendErrorResponse(c, http.StatusBadRequest, "Entity is not a PDF document (.pdf)", nil)
		return nil, false
	}
	return primary, true
}

// bufferPDF resolves the PDF file's storage backend (SMB / FTP / NFS / WebDAV / local) via the same
// ClientFactory the StreamHandler + ComicPagesHandler use and copies the document bytes into a
// private temp file (mode 0600). It returns the temp file path plus a cleanup func that removes it
// and releases the client connection. The caller MUST defer the returned cleanup.
//
// The reader is obtained random-access (OpenSeekable, for local + SMB) when available and streaming
// (ReadFile, for FTP/WebDAV/NFS) otherwise; either way it is copied once, bounded to
// pdfMaxBufferBytes.
func (h *PdfPagesHandler) bufferPDF(ctx context.Context, sf *repository.StreamableFile) (string, func(), error) {
	root, err := h.pdfStorageRootForFile(ctx, sf.FileID)
	if err != nil {
		return "", nil, err
	}

	// comicStorageRootSettings (package-level, defined in
	// comic_pages_handler.go) maps a StorageRoot to the per-protocol
	// settings the ClientFactory expects — identical for both readers, so
	// it is reused rather than duplicated.
	settings, err := comicStorageRootSettings(root)
	if err != nil {
		return "", nil, fmt.Errorf("storage root %q settings: %w", root.Name, err)
	}
	fsClient, err := h.clientFactory.CreateClient(&filesystem.StorageConfig{
		ID:       root.Name,
		Name:     root.Name,
		Protocol: root.Protocol,
		Settings: settings,
	})
	if err != nil {
		return "", nil, fmt.Errorf("create %s filesystem client: %w", root.Protocol, err)
	}
	if err := fsClient.Connect(ctx); err != nil {
		return "", nil, fmt.Errorf("connect to %s storage: %w", root.Protocol, err)
	}
	disconnect := func() { _ = fsClient.Disconnect(ctx) }

	tmp, err := os.CreateTemp("", "catalogizer-pdf-*.pdf")
	if err != nil {
		disconnect()
		return "", nil, fmt.Errorf("create temp pdf buffer: %w", err)
	}
	// removeAndDisconnect releases the temp file then the client. os.Remove
	// runs after the *os.File is closed so the fd is freed first (§11.4.14 —
	// leave no orphan temp files).
	removeAndDisconnect := func() {
		_ = os.Remove(tmp.Name())
		disconnect()
	}

	// Prefer the random-access handle (local + SMB are SeekableClient);
	// fall back to the streaming reader (FTP/WebDAV/NFS). Both are copied
	// once into the temp file. ReadSeekCloser and io.ReadCloser both satisfy
	// io.ReadCloser.
	var reader io.ReadCloser
	if sc, ok := fsClient.(filesystem.SeekableClient); ok {
		if rs, oerr := sc.OpenSeekable(ctx, sf.Path); oerr == nil {
			reader = rs
		}
	}
	if reader == nil {
		r, rerr := fsClient.ReadFile(ctx, sf.Path)
		if rerr != nil {
			_ = tmp.Close()
			removeAndDisconnect()
			return "", nil, fmt.Errorf("open pdf file on %s storage: %w", root.Protocol, rerr)
		}
		reader = r
	}

	nCopied, copyErr := io.Copy(tmp, io.LimitReader(reader, pdfMaxBufferBytes+1))
	_ = reader.Close()
	// Close the temp file so the poppler child reads all flushed bytes by path.
	closeErr := tmp.Close()
	if copyErr == nil && nCopied > pdfMaxBufferBytes {
		copyErr = errPDFTooBig
	}
	if copyErr != nil {
		removeAndDisconnect()
		return "", nil, fmt.Errorf("buffer pdf file from %s storage: %w", root.Protocol, copyErr)
	}
	if closeErr != nil {
		removeAndDisconnect()
		return "", nil, fmt.Errorf("flush temp pdf buffer: %w", closeErr)
	}

	return tmp.Name(), removeAndDisconnect, nil
}

// pdfStorageRootForFile looks up the storage root (protocol, host, share,
// credentials) that backs a given file id — the PDF-handler analogue of
// ComicPagesHandler.getStorageRootForFile, resolved directly from the file
// id the MediaFileRepository already gave us.
func (h *PdfPagesHandler) pdfStorageRootForFile(ctx context.Context, fileID int64) (*root_models.StorageRoot, error) {
	query := `
		SELECT sr.id, sr.name, sr.protocol, sr.host, sr.port, sr.path, sr.username,
		       sr.password, sr.domain, sr.mount_point, sr.options, sr.url, sr.enabled,
		       sr.max_depth, sr.enable_duplicate_detection, sr.enable_metadata_extraction,
		       sr.include_patterns, sr.exclude_patterns, sr.created_at, sr.updated_at, sr.last_scan_at
		FROM files f
		JOIN storage_roots sr ON sr.id = f.storage_root_id
		WHERE f.id = ?`

	var root root_models.StorageRoot
	err := h.db.QueryRowContext(ctx, query, fileID).Scan(
		&root.ID, &root.Name, &root.Protocol, &root.Host, &root.Port, &root.Path,
		&root.Username, &root.Password, &root.Domain, &root.MountPoint, &root.Options,
		&root.URL, &root.Enabled, &root.MaxDepth, &root.EnableDuplicateDetection,
		&root.EnableMetadataExtraction, &root.IncludePatterns, &root.ExcludePatterns,
		&root.CreatedAt, &root.UpdatedAt, &root.LastScanAt,
	)
	if err != nil {
		return nil, fmt.Errorf("storage root for pdf file %d not found: %w", fileID, err)
	}
	return &root, nil
}

// respondPDFError maps an error from buffering or the isolated renderer to an HTTP response. The
// response body is generic (no parser, tool or path detail); the detail is logged.
func respondPDFError(c *gin.Context, op string, err error) {
	var status int
	var msg string
	switch {
	case errors.Is(err, services.ErrPDFPageOutOfRange):
		status, msg = http.StatusNotFound, "Page index out of range"
	case errors.Is(err, services.ErrPDFEncrypted):
		status, msg = http.StatusUnprocessableEntity, "PDF document is encrypted and cannot be read"
	case errors.Is(err, services.ErrPDFInvalid):
		status, msg = http.StatusUnprocessableEntity, "File is not a readable PDF document"
	case errors.Is(err, services.ErrPDFPageTooLarge), errors.Is(err, errPDFTooBig):
		status, msg = http.StatusRequestEntityTooLarge, "PDF document or page is too large to render"
	case errors.Is(err, services.ErrPDFBusy):
		c.Header("Retry-After", "2")
		status, msg = http.StatusTooManyRequests, "PDF reader is busy, retry shortly"
	case errors.Is(err, services.ErrPDFRenderCanceled), errors.Is(err, context.Canceled):
		// The client went away: not a server timeout. 499 (client closed request) is for logs only.
		status, msg = 499, "Request canceled"
	case errors.Is(err, services.ErrPDFRenderTimeout), errors.Is(err, context.DeadlineExceeded):
		status, msg = http.StatusGatewayTimeout, "PDF rendering timed out"
	case errors.Is(err, services.ErrPDFRenderUnavailable):
		status, msg = http.StatusServiceUnavailable, "PDF rendering is not available"
	default:
		status, msg = http.StatusInternalServerError, "Failed to read PDF document"
	}
	if logging.Logger != nil {
		logging.With(logging.String("op", op), logging.Int("status", status), logging.ErrorField(err)).Warn("pdf-pages request failed")
	}
	utils.SendErrorResponse(c, status, msg, nil)
}

// ListPdfPages handles GET /api/v1/entities/:id/pdf-pages.
//
// It opens the entity's resolved PDF over its storage protocol and returns
// the page count:
//
//	{"total_pages": N}
func (h *PdfPagesHandler) ListPdfPages(c *gin.Context) {
	sf, ok := h.resolvePDF(c)
	if !ok {
		return
	}

	release, ok := services.AcquirePDFReaderSlot()
	if !ok {
		respondPDFError(c, "slot", services.ErrPDFBusy)
		return
	}
	defer release()

	path, cleanup, err := h.bufferPDF(c.Request.Context(), sf)
	if err != nil {
		respondPDFError(c, "buffer", err)
		return
	}
	defer cleanup()

	total, err := services.PDFPageCount(c.Request.Context(), path)
	if err != nil {
		respondPDFError(c, "count", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"total_pages": total})
}

// GetPdfPage handles GET /api/v1/entities/:id/pdf-pages/:n.
//
// It renders the single 0-based page n of the entity's PDF (opened over
// its storage protocol) to a PNG and streams the image bytes with
// Content-Type image/png. Only the requested page is rendered — memory
// stays bounded to one page's raster (§12.6).
func (h *PdfPagesHandler) GetPdfPage(c *gin.Context) {
	sf, ok := h.resolvePDF(c)
	if !ok {
		return
	}

	n, err := strconv.Atoi(c.Param("n"))
	if err != nil || n < 0 {
		utils.SendErrorResponse(c, http.StatusBadRequest, "Invalid page index", nil)
		return
	}

	release, ok := services.AcquirePDFReaderSlot()
	if !ok {
		respondPDFError(c, "slot", services.ErrPDFBusy)
		return
	}
	defer release()

	path, cleanup, err := h.bufferPDF(c.Request.Context(), sf)
	if err != nil {
		respondPDFError(c, "buffer", err)
		return
	}
	defer cleanup()

	png, err := services.RenderPDFPagePNG(c.Request.Context(), path, n, pdfRenderDPI)
	if err != nil {
		respondPDFError(c, "render", err)
		return
	}
	c.Data(http.StatusOK, "image/png", png)
}
