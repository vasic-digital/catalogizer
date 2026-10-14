package handlers

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"catalogizer/database"
	"catalogizer/filesystem"
	root_models "catalogizer/models"
	"catalogizer/repository"
	"catalogizer/utils"

	"github.com/gen2brain/go-fitz"
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
// Rendering: MuPDF via github.com/gen2brain/go-fitz (already a project
// dependency, used by services/conversion_service.go). Unlike a .cbz
// (where archive/zip reads only the central directory + the requested
// entry over an io.ReaderAt), MuPDF needs the WHOLE document to parse it,
// so the handler buffers the PDF once to a temp file (bounded to disk, not
// RAM, §12.6) and renders ONE page at a time (§12.6 — never all pages at
// once). The temp file is removed on cleanup (§11.4.14).
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
// keeping each rendered PNG bounded in size (§12.6). It matches the
// default used by services/conversion_service.go.
const pdfRenderDPI = 150.0

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
		utils.SendErrorResponse(c, http.StatusBadRequest, "Invalid entity ID", err)
		return nil, false
	}

	primary, err := h.fileRepo.GetPrimaryStreamableFile(ctx, id)
	if err != nil {
		if err == repository.ErrNoStreamableFile {
			utils.SendErrorResponse(c, http.StatusNotFound, "No file available for this entity", err)
			return nil, false
		}
		utils.SendErrorResponse(c, http.StatusInternalServerError, "Failed to resolve PDF file", err)
		return nil, false
	}

	if !strings.EqualFold(filepath.Ext(primary.Path), ".pdf") {
		utils.SendErrorResponse(c, http.StatusBadRequest, "Entity is not a PDF document (.pdf)", nil)
		return nil, false
	}
	return primary, true
}

// openPDFDocument resolves the PDF file's storage backend (SMB / FTP / NFS
// / WebDAV / local) via the same ClientFactory the StreamHandler +
// ComicPagesHandler use, buffers the document bytes into a temp file, and
// opens it with MuPDF. It returns the *fitz.Document plus a cleanup func
// that releases every underlying resource (MuPDF document, temp file,
// client connection). The caller MUST defer the returned cleanup.
//
// Why buffer (unlike the comic .cbz random-access path, §11.4.6 — not
// guessed): archive/zip can read a single entry out of an io.ReaderAt
// without downloading the whole archive, but MuPDF parses the entire
// document object graph + cross-reference table to answer NumPage() or
// render any page, so the whole PDF must be present. Buffering once to a
// temp file bounds this to DISK rather than RAM (§12.6); rendering then
// happens one page at a time. The reader is obtained random-access
// (OpenSeekable, for local + SMB) when available and streaming (ReadFile,
// for FTP/WebDAV/NFS) otherwise — either way it is copied once into the
// temp file.
func (h *PdfPagesHandler) openPDFDocument(ctx context.Context, sf *repository.StreamableFile) (*fitz.Document, func(), error) {
	root, err := h.pdfStorageRootForFile(ctx, sf.FileID)
	if err != nil {
		return nil, nil, err
	}

	// comicStorageRootSettings (package-level, defined in
	// comic_pages_handler.go) maps a StorageRoot to the per-protocol
	// settings the ClientFactory expects — identical for both readers, so
	// it is reused rather than duplicated.
	settings, err := comicStorageRootSettings(root)
	if err != nil {
		return nil, nil, fmt.Errorf("storage root %q settings: %w", root.Name, err)
	}
	fsClient, err := h.clientFactory.CreateClient(&filesystem.StorageConfig{
		ID:       root.Name,
		Name:     root.Name,
		Protocol: root.Protocol,
		Settings: settings,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("create %s filesystem client: %w", root.Protocol, err)
	}
	if err := fsClient.Connect(ctx); err != nil {
		return nil, nil, fmt.Errorf("connect to %s storage: %w", root.Protocol, err)
	}
	disconnect := func() { _ = fsClient.Disconnect(ctx) }

	tmp, err := os.CreateTemp("", "catalogizer-pdf-*.pdf")
	if err != nil {
		disconnect()
		return nil, nil, fmt.Errorf("create temp pdf buffer: %w", err)
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
			return nil, nil, fmt.Errorf("open pdf file on %s storage: %w", root.Protocol, rerr)
		}
		reader = r
	}

	_, copyErr := io.Copy(tmp, reader)
	_ = reader.Close()
	// Close the temp file so MuPDF reopens it cleanly by path with all
	// buffered bytes flushed.
	closeErr := tmp.Close()
	if copyErr != nil {
		removeAndDisconnect()
		return nil, nil, fmt.Errorf("buffer pdf file from %s storage: %w", root.Protocol, copyErr)
	}
	if closeErr != nil {
		removeAndDisconnect()
		return nil, nil, fmt.Errorf("flush temp pdf buffer: %w", closeErr)
	}

	doc, err := fitz.New(tmp.Name())
	if err != nil {
		removeAndDisconnect()
		return nil, nil, fmt.Errorf("open pdf document: %w", err)
	}
	cleanup := func() {
		_ = doc.Close()
		removeAndDisconnect()
	}
	return doc, cleanup, nil
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

	doc, cleanup, err := h.openPDFDocument(c.Request.Context(), sf)
	if err != nil {
		utils.SendErrorResponse(c, http.StatusInternalServerError, "Failed to open PDF document", err)
		return
	}
	defer cleanup()

	c.JSON(http.StatusOK, gin.H{"total_pages": doc.NumPage()})
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
		utils.SendErrorResponse(c, http.StatusBadRequest, "Invalid page index", err)
		return
	}

	doc, cleanup, err := h.openPDFDocument(c.Request.Context(), sf)
	if err != nil {
		utils.SendErrorResponse(c, http.StatusInternalServerError, "Failed to open PDF document", err)
		return
	}
	defer cleanup()

	if n >= doc.NumPage() {
		utils.SendErrorResponse(c, http.StatusNotFound, "Page index out of range", nil)
		return
	}

	// Render exactly one page to PNG bytes — bounded memory (§12.6).
	png, err := doc.ImagePNG(n, pdfRenderDPI)
	if err != nil {
		utils.SendErrorResponse(c, http.StatusInternalServerError, "Failed to render PDF page", err)
		return
	}

	c.Data(http.StatusOK, "image/png", png)
}
