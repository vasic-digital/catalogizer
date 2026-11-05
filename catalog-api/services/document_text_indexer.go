package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"catalogizer/database"
)

// D4: full-text search over extracted document text. After a scan, DocumentTextIndexer extracts
// the text of the document files of a LOCAL storage root (pdf through the isolated CAT-002
// extractor, txt and md read directly, all bounded) into file_text_index, and
// FileRepository.SearchFiles matches `q` against it as well as the file name and path.
//
// Honest limits: only protocol "local" roots are indexed (a remote client does not hand out a
// local path for the isolated PDF extractor); only pdf, txt and md; matching is a substring LIKE
// (no stemming or ranking; SQLite LIKE is case-insensitive for ASCII only).

const (
	// docIndexMaxPlainBytes bounds the bytes read from a txt or md file.
	docIndexMaxPlainBytes = 4 << 20
	// docIndexMaxPDFBytes: a larger PDF file is not indexed (status failed).
	docIndexMaxPDFBytes = 64 << 20
	// docIndexMaxPDFPages bounds the pages extracted from one PDF.
	docIndexMaxPDFPages = 200
	// docIndexMaxStoredBytes bounds the text stored for one file.
	docIndexMaxStoredBytes = 2 << 20
)

// DocumentTextIndexer builds file_text_index.
type DocumentTextIndexer struct {
	db        *database.DB
	extractor pdfTextExtractor

	// single-flight per storage root: repeated scans coalesce into one running indexer plus one
	// follow-up pass (F1), so indexers never stack up.
	mu      sync.Mutex
	running map[int64]bool
	pending map[int64]bool
}

// NewDocumentTextIndexer returns an indexer that uses the production isolated PDF extractor.
func NewDocumentTextIndexer(db *database.DB) *DocumentTextIndexer {
	return &DocumentTextIndexer{db: db, extractor: defaultPDFTextExtractor(), running: map[int64]bool{}, pending: map[int64]bool{}}
}

// IndexStorageRoot indexes every new or changed document of the root. It returns how many
// documents were indexed and how many failed. A root that is not local is not indexed (0, 0, nil).
func (x *DocumentTextIndexer) IndexStorageRoot(ctx context.Context, rootID int64) (indexed, failed int, err error) {
	x.mu.Lock()
	if x.running[rootID] {
		x.pending[rootID] = true // the running pass will run once more; this call coalesces into it
		x.mu.Unlock()
		return 0, 0, nil
	}
	x.running[rootID] = true
	x.mu.Unlock()
	for {
		i, f, e := x.indexOnce(ctx, rootID)
		indexed, failed = indexed+i, failed+f
		// decide "again" and clear "running" in ONE critical section: a call landing after this
		// either sees running (and sets pending before we read it) or starts its own pass (R2-2).
		x.mu.Lock()
		again := x.pending[rootID] && e == nil && ctx.Err() == nil
		delete(x.pending, rootID)
		if !again {
			delete(x.running, rootID)
		}
		x.mu.Unlock()
		if e != nil {
			return indexed, failed, e
		}
		if !again {
			return indexed, failed, nil
		}
	}
}

func (x *DocumentTextIndexer) indexOnce(ctx context.Context, rootID int64) (indexed, failed int, err error) {
	var protocol string
	var rootPath sql.NullString
	if err = x.db.QueryRowContext(ctx, `SELECT protocol, path FROM storage_roots WHERE id = ?`, rootID).Scan(&protocol, &rootPath); err != nil {
		return 0, 0, fmt.Errorf("load storage root %d: %w", rootID, err)
	}
	if protocol != "local" || !rootPath.Valid || rootPath.String == "" {
		return 0, 0, nil
	}
	rows, err := x.db.QueryContext(ctx, `
		SELECT f.id, f.path, LOWER(COALESCE(f.extension, '')), f.size, CAST(f.modified_at AS TEXT)
		FROM files f
		LEFT JOIN file_text_index t ON t.file_id = f.id
		WHERE f.storage_root_id = ? AND f.deleted = 0 AND f.is_directory = 0
		  AND LOWER(COALESCE(f.extension, '')) IN ('pdf', 'txt', 'md')
		  AND (t.file_id IS NULL OR t.source_size <> f.size OR t.source_modified <> CAST(f.modified_at AS TEXT))`, rootID)
	if err != nil {
		return 0, 0, fmt.Errorf("list unindexed documents: %w", err)
	}
	type doc struct {
		id       int64
		path     string
		ext      string
		size     int64
		modified string
	}
	var docs []doc
	for rows.Next() {
		var d doc
		if err := rows.Scan(&d.id, &d.path, &d.ext, &d.size, &d.modified); err != nil {
			rows.Close()
			return 0, 0, fmt.Errorf("scan unindexed document: %w", err)
		}
		docs = append(docs, d)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, 0, err
	}
	rows.Close() // cursor closed before the writes below (no nested pooled query)

	for _, d := range docs {
		if ctx.Err() != nil {
			return indexed, failed, ctx.Err()
		}
		text, xerr := x.extract(ctx, rootPath.String, d.path, d.ext)
		status := "ok"
		if xerr != nil && (isEnvironmentalErr(xerr) || ctx.Err() != nil) {
			// F5: a missing tool or a cancelled run says nothing about the file; do not persist it as failed.
			failed++
			continue
		}
		if xerr != nil {
			status, text = "failed", ""
			failed++
		} else {
			indexed++
		}
		// The SQLCipher build has no ON CONFLICT DO UPDATE: the dialect helper builds the portable upsert.
		q := x.db.Dialect().UpsertSQL("file_text_index",
			[]string{"file_id", "content", "status", "source_size", "source_modified", "indexed_at"}, []string{"file_id"})
		if _, err := x.db.ExecContext(ctx, q, d.id, text, status, d.size, d.modified, time.Now().UTC()); err != nil {
			return indexed, failed, fmt.Errorf("store text of file %d: %w", d.id, err)
		}
	}
	return indexed, failed, nil
}

// isEnvironmentalErr reports errors caused by the host (missing tool, cancellation), not by the file.
func isEnvironmentalErr(err error) bool {
	// a per-file timeout is NOT environmental: it is recorded failed (R2-5); a cancelled parent
	// context is handled by the caller through ctx.Err().
	return errors.Is(err, errPrlimitMissing) || errors.Is(err, exec.ErrNotFound)
}

// openInRoot opens a regular file inside the root through ONE handle and verifies, on that handle,
// that it is inside the root and regular (F6: no check-then-use window; a parent swapped for a
// symlink before the open is caught by the /proc/self/fd re-verification). The final component is
// opened O_NOFOLLOW|O_NONBLOCK so a FIFO or device cannot block the open.
func openInRoot(root, rel string) (*os.File, error) {
	rootAbs, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	full := filepath.Join(rootAbs, filepath.FromSlash(strings.TrimLeft(rel, "/")))
	if lb, err := filepath.Rel(rootAbs, full); err != nil || lb == ".." || strings.HasPrefix(lb, ".."+string(filepath.Separator)) {
		return nil, errors.New("path escapes the storage root")
	}
	// the leaf is opened without following a symlink; a symlinked leaf is resolved once, lexically
	// checked below, and then opened by its resolved path.
	target := full
	if resolved, err := filepath.EvalSymlinks(full); err == nil {
		target = resolved
	} else {
		return nil, err
	}
	f, err := os.OpenFile(target, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	ok := false
	defer func() {
		if !ok {
			f.Close()
		}
	}()
	// re-verify on the opened handle: where does the fd really point?
	real, err := os.Readlink(fmt.Sprintf("/proc/self/fd/%d", f.Fd()))
	if err != nil {
		return nil, err
	}
	back, err := filepath.Rel(rootAbs, real)
	if err != nil || back == ".." || strings.HasPrefix(back, ".."+string(filepath.Separator)) {
		return nil, errors.New("path escapes the storage root")
	}
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !st.Mode().IsRegular() {
		return nil, errors.New("not a regular file")
	}
	ok = true
	return f, nil
}

func (x *DocumentTextIndexer) extract(ctx context.Context, root, rel, ext string) (string, error) {
	f, err := openInRoot(root, rel)
	if err != nil {
		return "", err
	}
	defer f.Close()
	switch ext {
	case "txt", "md":
		return readPlainCapped(f)
	case "pdf":
		st, err := f.Stat()
		if err != nil {
			return "", err
		}
		if st.Size() > docIndexMaxPDFBytes {
			return "", errors.New("pdf too large to index")
		}
		// PDF extraction shares the reader slot budget (F1): the slot is counted in pdfToolLimits.
		// R2-1: the indexer holds at most ONE reader slot globally, so one slot always stays free
		// for interactive (non-blocking) HTTP readers.
		release, err := acquireIndexerPDFSlot(ctx)
		if err != nil {
			return "", err
		}
		defer release()
		pctx, cancel := context.WithTimeout(ctx, pdfTextTimeout)
		defer cancel()
		// the child reopens the SAME inode through the already verified descriptor (F6).
		path := fmt.Sprintf("/proc/%d/fd/%d", os.Getpid(), f.Fd())
		n, err := x.extractor.NumPages(pctx, path)
		if err != nil {
			return "", err
		}
		if n > docIndexMaxPDFPages {
			n = docIndexMaxPDFPages
		}
		if n < 1 {
			return "", nil
		}
		pages, err := x.extractor.ExtractPages(pctx, path, 1, n)
		if err != nil {
			return "", err
		}
		return clampText(strings.Join(pages, "\n")), nil
	}
	return "", fmt.Errorf("unsupported extension %q", ext)
}

// docIndexerSlots is the number of reader slots the indexer may hold at once across all roots.
// It must stay below PDFReaderSlots so interactive readers are never starved. The pure-Go child the
// extractor may use is counted by pdfChildLimits through this constant.
const docIndexerSlots = 1

var docIndexerGate = make(chan struct{}, docIndexerSlots)

// acquireIndexerPDFSlot takes the global indexer gate, then a shared reader slot.
func acquireIndexerPDFSlot(ctx context.Context) (func(), error) {
	select {
	case docIndexerGate <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	rel, err := acquirePDFReaderSlotWait(ctx)
	if err != nil {
		<-docIndexerGate
		return nil, err
	}
	return func() { rel(); <-docIndexerGate }, nil
}

// acquirePDFReaderSlotWait blocks (bounded by ctx) for a reader slot of the shared PDF budget.
func acquirePDFReaderSlotWait(ctx context.Context) (func(), error) {
	select {
	case pdfReaderSem <- struct{}{}:
		var once sync.Once
		return func() { once.Do(func() { <-pdfReaderSem }) }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// clampText bounds the stored text and makes it valid UTF-8 without NUL bytes.
func clampText(s string) string {
	if len(s) > docIndexMaxStoredBytes {
		s = s[:docIndexMaxStoredBytes]
	}
	s = strings.ToValidUTF8(s, "")
	return strings.ReplaceAll(s, "\x00", "")
}

// readPlainCapped reads at most docIndexMaxPlainBytes from r and returns the clamped text.
func readPlainCapped(r io.Reader) (string, error) {
	b, err := io.ReadAll(io.LimitReader(r, docIndexMaxPlainBytes))
	if err != nil {
		return "", err
	}
	return clampText(string(b)), nil
}
