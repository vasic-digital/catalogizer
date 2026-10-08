package handlers

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"catalogizer/database"
	"catalogizer/filesystem"
	"catalogizer/internal/services"
	root_models "catalogizer/models"
	"catalogizer/repository"
	"catalogizer/utils"

	"github.com/gin-gonic/gin"
	rardecode "github.com/nwaples/rardecode/v2"
)

// ComicPagesHandler serves individual image pages out of comic-book
// archive files so a reader client (e.g. the Android TV comic reader)
// can page through a comic without downloading the whole archive.
//
// Multi-protocol storage: a comic's file path stored in the DB is
// RELATIVE to its storage root (an SMB share path, an FTP/WebDAV/NFS
// path, or a local base-path subtree) — it is NOT an absolute local
// filesystem path. The handler therefore resolves the archive through
// the SAME filesystem.ClientFactory the StreamHandler uses, opening the
// archive over whatever protocol the storage root declares (SMB, FTP,
// NFS, WebDAV, local). Opening it with a bare local archive/zip reader
// (as an earlier version did) only worked for local-absolute paths and
// failed for every comic on an SMB/FTP/NFS/WebDAV share.
//
// Supported archive formats:
//   - .cbz  (ZIP container)  — fully supported via the stdlib
//     archive/zip reader.
//   - .cbr  (RAR container)  — fully supported via the pure-Go (no CGO)
//     github.com/nwaples/rardecode/v2 streaming decoder. RAR has no
//     central directory, so listing pages is a single forward pass over
//     the entry headers and extracting page n is a fresh forward pass to
//     that entry (only one page is buffered — bounded memory, §12.6). The
//     SAME multi-protocol byte resolution, page-image filter, and natural
//     sort as .cbz are reused.
//
// Page indexing is ZERO-BASED: the `index` field returned by
// ListComicPages starts at 0, and GET .../pages/:n expects the same
// 0-based n. n in the closed range [0, total_pages-1].
type ComicPagesHandler struct {
	fileRepo      *repository.MediaFileRepository
	db            *database.DB
	clientFactory filesystem.ClientFactory
}

// NewComicPagesHandler constructs the handler. It depends on:
//   - the MediaFileRepository (resolves an entity id to its concrete
//     file via GetPrimaryStreamableFile — the same path StreamEntity
//     uses);
//   - the *database.DB (looks up the file's storage root: protocol,
//     host, share, credentials — exactly as StreamHandler does);
//   - the filesystem.ClientFactory (opens the archive bytes over the
//     storage root's protocol — SMB/FTP/NFS/WebDAV/local).
func NewComicPagesHandler(fileRepo *repository.MediaFileRepository, db *database.DB, clientFactory filesystem.ClientFactory) *ComicPagesHandler {
	return &ComicPagesHandler{fileRepo: fileRepo, db: db, clientFactory: clientFactory}
}

// comicImageExts are the in-archive entry extensions treated as comic
// pages (lower-cased, leading dot included).
var comicImageExts = map[string]struct{}{
	".jpg": {}, ".jpeg": {}, ".png": {}, ".webp": {}, ".gif": {},
}

// comicContentType maps a page entry name to its image MIME type.
func comicContentType(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	default:
		return "application/octet-stream"
	}
}

// resolveArchive parses the :id param, resolves the entity's primary
// file through the same GetPrimaryStreamableFile path StreamEntity
// uses, and verifies the resolved file is a comic archive (.cbz/.cbr).
// On any failure it writes the HTTP error response and returns ok=false.
//
// The archive path is the DB-resolved item path, never derived from
// user input, so there is no path-traversal surface from the :id.
func (h *ComicPagesHandler) resolveArchive(c *gin.Context) (sf *repository.StreamableFile, ok bool) {
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
		utils.SendErrorResponse(c, http.StatusInternalServerError, "Failed to resolve comic file", err)
		return nil, false
	}

	switch strings.ToLower(filepath.Ext(primary.Path)) {
	case ".cbz", ".cbr":
		return primary, true
	default:
		utils.SendErrorResponse(c, http.StatusBadRequest, "Entity is not a comic archive (.cbz/.cbr)", nil)
		return nil, false
	}
}

// isCBR reports whether the resolved archive is a RAR-backed comic.
func isCBR(path string) bool {
	return strings.EqualFold(filepath.Ext(path), ".cbr")
}

// openComicArchive resolves the comic file's storage backend (SMB / FTP /
// NFS / WebDAV / local) via the same ClientFactory the StreamHandler uses,
// obtains a random-access view of the archive bytes, and returns a
// *zip.Reader plus a cleanup func that releases every underlying resource
// (file handle / temp file / client connection). The caller MUST defer the
// returned cleanup.
//
// Two access strategies, chosen from the real client capabilities (§11.4.6),
// never guessed:
//
//   - Random-access (local + SMB): the protocol client implements
//     filesystem.SeekableClient and the handle it returns is an io.ReaderAt
//     (os.File and smb2.File both implement ReadAt). archive/zip.NewReader
//     reads only the central directory + the bytes of the page actually
//     requested — the whole archive is NEVER downloaded. Bounded memory and
//     bounded network (§12.6), which matters because comics are browsed
//     page-by-page.
//   - Buffered fallback (FTP / WebDAV / NFS, or any seekable handle that is
//     not an io.ReaderAt): the only view is a streaming io.Reader, so the
//     archive is copied to a temp file (os.CreateTemp), which IS an
//     io.ReaderAt. Bounded to disk rather than RAM (§12.6); the temp file is
//     removed on cleanup (§11.4.14).
func (h *ComicPagesHandler) openComicArchive(ctx context.Context, sf *repository.StreamableFile) (*zip.Reader, func(), error) {
	ra, size, cleanup, err := h.resolveComicReaderAt(ctx, sf, ".cbz")
	if err != nil {
		return nil, nil, err
	}
	zr, zerr := zip.NewReader(ra, size)
	if zerr != nil {
		cleanup()
		return nil, nil, fmt.Errorf("read comic zip directory: %w", zerr)
	}
	return zr, cleanup, nil
}

// resolveComicReaderAt performs the multi-protocol byte resolution shared by
// every comic format (.cbz and .cbr): it resolves the file's storage backend
// (SMB / FTP / NFS / WebDAV / local) via the same ClientFactory the
// StreamHandler uses and returns an io.ReaderAt over the archive bytes, the
// archive size, and a cleanup func that releases every underlying resource
// (handle / temp file / client connection). The caller MUST defer cleanup.
//
// Two access strategies, chosen from the real client capabilities (§11.4.6),
// never guessed:
//
//   - Random-access (local + SMB): the protocol client implements
//     filesystem.SeekableClient and the handle it returns is an io.ReaderAt
//     (os.File and smb2.File both implement ReadAt). The whole archive is
//     NEVER downloaded for a .cbz (zip reads only the central directory + the
//     requested page); for a .cbr the decoder streams forward over a
//     SectionReader on the same ReaderAt. Bounded memory (§12.6).
//   - Buffered fallback (FTP / WebDAV / NFS, or any seekable handle that is
//     not an io.ReaderAt): the only view is a streaming io.Reader, so the
//     archive is copied to a temp file (os.CreateTemp), which IS an
//     io.ReaderAt. Bounded to disk rather than RAM (§12.6); the temp file is
//     removed on cleanup (§11.4.14). tmpSuffix only labels the temp file with
//     the archive's extension.
func (h *ComicPagesHandler) resolveComicReaderAt(ctx context.Context, sf *repository.StreamableFile, tmpSuffix string) (io.ReaderAt, int64, func(), error) {
	root, err := h.getStorageRootForFile(ctx, sf.FileID)
	if err != nil {
		return nil, 0, nil, err
	}

	fsClient, err := h.clientFactory.CreateClient(&filesystem.StorageConfig{
		ID:       root.Name,
		Name:     root.Name,
		Protocol: root.Protocol,
		Settings: comicStorageRootSettings(root),
	})
	if err != nil {
		return nil, 0, nil, fmt.Errorf("create %s filesystem client: %w", root.Protocol, err)
	}
	if err := fsClient.Connect(ctx); err != nil {
		return nil, 0, nil, fmt.Errorf("connect to %s storage: %w", root.Protocol, err)
	}
	// The base cleanup disconnects the client; later strategies prepend their
	// own resource releases so cleanup always tears everything down in order.
	disconnect := func() { _ = fsClient.Disconnect(ctx) }

	// Strategy 1 — random-access, no full download.
	if sc, ok := fsClient.(filesystem.SeekableClient); ok {
		if rs, oerr := sc.OpenSeekable(ctx, sf.Path); oerr == nil {
			size, serr := rs.Seek(0, io.SeekEnd)
			if ra, isReaderAt := rs.(io.ReaderAt); isReaderAt && serr == nil && size > 0 {
				return ra, size, func() { _ = rs.Close(); disconnect() }, nil
			}
			// Seekable handle is not a usable io.ReaderAt (or size unknown):
			// release it and fall through to the buffered path.
			_ = rs.Close()
		}
		// OpenSeekable failed: fall through to ReadFile + buffering.
	}

	// Strategy 2 — buffer the streaming reader into a temp file.
	reader, err := fsClient.ReadFile(ctx, sf.Path)
	if err != nil {
		disconnect()
		return nil, 0, nil, fmt.Errorf("open comic file on %s storage: %w", root.Protocol, err)
	}
	tmp, err := os.CreateTemp("", "catalogizer-comic-*"+tmpSuffix)
	if err != nil {
		_ = reader.Close()
		disconnect()
		return nil, 0, nil, fmt.Errorf("create temp comic buffer: %w", err)
	}
	// Removes the temp file AND disconnects. os.Remove after Close so the fd
	// is released first (§11.4.14 — leave no orphan temp files).
	removeAndDisconnect := func() {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		disconnect()
	}
	n, err := io.Copy(tmp, reader)
	_ = reader.Close()
	if err != nil {
		removeAndDisconnect()
		return nil, 0, nil, fmt.Errorf("buffer comic file from %s storage: %w", root.Protocol, err)
	}
	// *os.File is an io.ReaderAt; ReadAt is offset-independent, so the
	// end-of-file write offset left by io.Copy does not matter here.
	return tmp, n, removeAndDisconnect, nil
}

// getStorageRootForFile looks up the storage root (protocol, host, share,
// credentials) that backs a given file id — the comic-handler analogue of
// StreamHandler.getStorageRootByName, resolved directly from the file id the
// MediaFileRepository already gave us.
func (h *ComicPagesHandler) getStorageRootForFile(ctx context.Context, fileID int64) (*root_models.StorageRoot, error) {
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
		return nil, fmt.Errorf("storage root for comic file %d not found: %w", fileID, err)
	}
	return &root, nil
}

// comicStorageRootSettings converts a StorageRoot into the settings map the
// filesystem ClientFactory expects. PA-01: one shared mapping,
// filesystem.SettingsFromRoot; SMB credentials are resolved via
// services.ResolveSMBIdentity (env-var identities; §11.4.10 - the password is
// never logged here).
func comicStorageRootSettings(root *root_models.StorageRoot) map[string]interface{} {
	return filesystem.SettingsFromRoot(root, services.ResolveSMBIdentity)
}

// comicPageEntries returns the image entries of a .cbz, filtered to
// page images and sorted into natural reading order. Directory entries,
// non-image entries, and macOS resource-fork junk (__MACOSX/, ._*) are
// excluded.
func comicPageEntries(zr *zip.Reader) []*zip.File {
	out := make([]*zip.File, 0, len(zr.File))
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		name := f.Name
		base := filepath.Base(name)
		if strings.HasPrefix(name, "__MACOSX/") || strings.HasPrefix(base, "._") {
			continue
		}
		if _, isImg := comicImageExts[strings.ToLower(filepath.Ext(name))]; !isImg {
			continue
		}
		out = append(out, f)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return naturalLess(out[i].Name, out[j].Name)
	})
	return out
}

// isComicCbrPage applies the SAME page-image filter as comicPageEntries (skip
// directories, __MACOSX/ resource forks, ._ dotfiles, and non-image entries)
// to a streaming RAR file header. rardecode normalises '\' to '/' in entry
// names, so the path-prefix checks match the .cbz path exactly, and the page
// extension set (comicImageExts) is the same — keeping cbz/cbr behaviour
// uniform.
func isComicCbrPage(hdr *rardecode.FileHeader) bool {
	if hdr.IsDir {
		return false
	}
	name := hdr.Name
	base := filepath.Base(name)
	if strings.HasPrefix(name, "__MACOSX/") || strings.HasPrefix(base, "._") {
		return false
	}
	_, isImg := comicImageExts[strings.ToLower(filepath.Ext(name))]
	return isImg
}

// comicCbrPageNames streams once through a .cbr (RAR) archive collecting the
// names of its page-image entries, then sorts them into the SAME natural
// reading order .cbz uses. RAR is a streaming format with no central
// directory, so listing is a single forward pass over the entry headers; the
// entry bodies are skipped (the decoder discards them), never buffered, so
// memory stays bounded (§12.6). A genuinely undecodable entry header surfaces
// as an honest error rather than a crash (§11.4.1).
func comicCbrPageNames(ra io.ReaderAt, size int64) ([]string, error) {
	rr, err := rardecode.NewReader(io.NewSectionReader(ra, 0, size))
	if err != nil {
		return nil, fmt.Errorf("open comic rar: %w", err)
	}
	var names []string
	for {
		hdr, nerr := rr.Next()
		if nerr == io.EOF {
			break
		}
		if nerr != nil {
			return nil, fmt.Errorf("read comic rar entry: %w", nerr)
		}
		if isComicCbrPage(hdr) {
			names = append(names, hdr.Name)
		}
	}
	sort.SliceStable(names, func(i, j int) bool {
		return naturalLess(names[i], names[j])
	})
	return names, nil
}

// naturalLess compares two strings so that embedded digit runs are
// ordered numerically ("page2.png" < "page10.png"), giving correct
// comic page ordering even when filenames are not zero-padded.
func naturalLess(a, b string) bool {
	a = strings.ToLower(a)
	b = strings.ToLower(b)
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		ca, cb := a[i], b[j]
		if isDigit(ca) && isDigit(cb) {
			si, sj := i, j
			for i < len(a) && isDigit(a[i]) {
				i++
			}
			for j < len(b) && isDigit(b[j]) {
				j++
			}
			na := strings.TrimLeft(a[si:i], "0")
			nb := strings.TrimLeft(b[sj:j], "0")
			if len(na) != len(nb) {
				return len(na) < len(nb)
			}
			if na != nb {
				return na < nb
			}
			// Equal numeric value — continue with the trailing text.
			continue
		}
		if ca != cb {
			return ca < cb
		}
		i++
		j++
	}
	return (len(a) - i) < (len(b) - j)
}

func isDigit(b byte) bool { return b >= '0' && b <= '9' }

// ListComicPages handles GET /api/v1/entities/:id/pages.
//
// It opens the entity's resolved comic archive over its storage protocol,
// lists its image page entries in natural order, and returns:
//
//	{"total_pages": N, "pages": [{"index": 0, "name": "001.png"}, ...]}
//
// Both .cbz (ZIP) and .cbr (RAR) archives are supported; the response shape is
// identical for both.
func (h *ComicPagesHandler) ListComicPages(c *gin.Context) {
	sf, ok := h.resolveArchive(c)
	if !ok {
		return
	}
	if isCBR(sf.Path) {
		h.listCBRPages(c, sf)
		return
	}

	zr, cleanup, err := h.openComicArchive(c.Request.Context(), sf)
	if err != nil {
		utils.SendErrorResponse(c, http.StatusInternalServerError, "Failed to open comic archive", err)
		return
	}
	defer cleanup()

	entries := comicPageEntries(zr)
	pages := make([]gin.H, 0, len(entries))
	for i, f := range entries {
		pages = append(pages, gin.H{"index": i, "name": f.Name})
	}
	c.JSON(http.StatusOK, gin.H{
		"total_pages": len(entries),
		"pages":       pages,
	})
}

// GetComicPage handles GET /api/v1/entities/:id/pages/:n.
//
// It extracts the single 0-based page n from the entity's comic archive
// (opened over its storage protocol) and streams its image bytes with the
// correct Content-Type. Only the requested entry is read out, so memory stays
// bounded to one page (and, for .cbz on SMB/local, only that page's bytes
// cross the network; .cbr is a streaming format — see getCBRPage).
//
// Both .cbz (ZIP) and .cbr (RAR) archives are supported.
func (h *ComicPagesHandler) GetComicPage(c *gin.Context) {
	sf, ok := h.resolveArchive(c)
	if !ok {
		return
	}

	n, err := strconv.Atoi(c.Param("n"))
	if err != nil || n < 0 {
		utils.SendErrorResponse(c, http.StatusBadRequest, "Invalid page index", err)
		return
	}

	if isCBR(sf.Path) {
		h.getCBRPage(c, sf, n)
		return
	}

	zr, cleanup, err := h.openComicArchive(c.Request.Context(), sf)
	if err != nil {
		utils.SendErrorResponse(c, http.StatusInternalServerError, "Failed to open comic archive", err)
		return
	}
	defer cleanup()

	entries := comicPageEntries(zr)
	if n >= len(entries) {
		utils.SendErrorResponse(c, http.StatusNotFound, "Page index out of range", nil)
		return
	}

	entry := entries[n]
	rc, err := entry.Open()
	if err != nil {
		utils.SendErrorResponse(c, http.StatusInternalServerError, "Failed to open comic page", err)
		return
	}
	defer rc.Close()

	// Stream the single entry — bounded memory, never unzips the whole
	// archive. DataFromReader sets status, Content-Type and
	// Content-Length and copies the reader to the response.
	c.DataFromReader(
		http.StatusOK,
		int64(entry.UncompressedSize64),
		comicContentType(entry.Name),
		rc,
		nil,
	)
}

// listCBRPages lists the page-image entries of a .cbr (RAR) archive over its
// storage protocol and writes the same {"total_pages":N,"pages":[...]} shape
// as the .cbz path.
func (h *ComicPagesHandler) listCBRPages(c *gin.Context, sf *repository.StreamableFile) {
	ra, size, cleanup, err := h.resolveComicReaderAt(c.Request.Context(), sf, ".cbr")
	if err != nil {
		utils.SendErrorResponse(c, http.StatusInternalServerError, "Failed to open comic archive", err)
		return
	}
	defer cleanup()

	names, err := comicCbrPageNames(ra, size)
	if err != nil {
		utils.SendErrorResponse(c, http.StatusInternalServerError, "Failed to read comic archive", err)
		return
	}

	pages := make([]gin.H, 0, len(names))
	for i, name := range names {
		pages = append(pages, gin.H{"index": i, "name": name})
	}
	c.JSON(http.StatusOK, gin.H{
		"total_pages": len(names),
		"pages":       pages,
	})
}

// getCBRPage extracts the 0-based page n from a .cbr (RAR) archive. RAR is a
// streaming format with no central directory, so this takes two forward
// passes over the SAME resolved bytes (one resolution per request):
//
//	pass 1 — list every page entry and sort into natural reading order, to map
//	         the 0-based n to a concrete entry name (the archive's stored order
//	         may differ from reading order);
//	pass 2 — a fresh forward pass to that entry, then read just that one page.
//
// Only the single requested page is buffered (one image — bounded memory,
// §12.6), and it is fully read BEFORE any response body is written so a
// genuine per-entry decode/CRC failure surfaces as an honest 500 rather than a
// truncated 200 (§11.4.1).
func (h *ComicPagesHandler) getCBRPage(c *gin.Context, sf *repository.StreamableFile, n int) {
	ra, size, cleanup, err := h.resolveComicReaderAt(c.Request.Context(), sf, ".cbr")
	if err != nil {
		utils.SendErrorResponse(c, http.StatusInternalServerError, "Failed to open comic archive", err)
		return
	}
	defer cleanup()

	names, err := comicCbrPageNames(ra, size)
	if err != nil {
		utils.SendErrorResponse(c, http.StatusInternalServerError, "Failed to read comic archive", err)
		return
	}
	if n >= len(names) {
		utils.SendErrorResponse(c, http.StatusNotFound, "Page index out of range", nil)
		return
	}
	target := names[n]

	rr, err := rardecode.NewReader(io.NewSectionReader(ra, 0, size))
	if err != nil {
		utils.SendErrorResponse(c, http.StatusInternalServerError, "Failed to open comic archive", err)
		return
	}
	for {
		hdr, nerr := rr.Next()
		if nerr == io.EOF {
			// Listed in pass 1 but absent in pass 2: the archive changed
			// underneath us. Honest error, never a silent 200 (§11.4.1).
			utils.SendErrorResponse(c, http.StatusInternalServerError, "Comic page not found in archive", nil)
			return
		}
		if nerr != nil {
			utils.SendErrorResponse(c, http.StatusInternalServerError, "Failed to read comic archive", nerr)
			return
		}
		if hdr.Name != target {
			continue
		}
		// rr reads the current entry; rardecode verifies the entry CRC at EOF,
		// so a corrupt page yields an honest decode error (§11.4.1).
		data, rerr := io.ReadAll(rr)
		if rerr != nil {
			utils.SendErrorResponse(c, http.StatusInternalServerError, "Failed to decode comic page", rerr)
			return
		}
		c.Data(http.StatusOK, comicContentType(hdr.Name), data)
		return
	}
}
