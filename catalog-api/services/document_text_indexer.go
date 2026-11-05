package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

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
}

// NewDocumentTextIndexer returns an indexer that uses the production isolated PDF extractor.
func NewDocumentTextIndexer(db *database.DB) *DocumentTextIndexer {
	return &DocumentTextIndexer{db: db, extractor: defaultPDFTextExtractor()}
}

// IndexStorageRoot indexes every new or changed document of the root. It returns how many
// documents were indexed and how many failed. A root that is not local is not indexed (0, 0, nil).
func (x *DocumentTextIndexer) IndexStorageRoot(ctx context.Context, rootID int64) (indexed, failed int, err error) {
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

// resolveInRoot maps a catalog path to a regular file inside the root, refusing a path that
// escapes the root (lexically or through a symlink).
func resolveInRoot(root, rel string) (string, error) {
	rootAbs, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	full := filepath.Join(rootAbs, filepath.FromSlash(strings.TrimLeft(rel, "/")))
	resolved, err := filepath.EvalSymlinks(full)
	if err != nil {
		return "", err
	}
	back, err := filepath.Rel(rootAbs, resolved)
	if err != nil || back == ".." || strings.HasPrefix(back, ".."+string(filepath.Separator)) {
		return "", errors.New("path escapes the storage root")
	}
	st, err := os.Stat(resolved)
	if err != nil {
		return "", err
	}
	if !st.Mode().IsRegular() {
		return "", errors.New("not a regular file")
	}
	return resolved, nil
}

func (x *DocumentTextIndexer) extract(ctx context.Context, root, rel, ext string) (string, error) {
	path, err := resolveInRoot(root, rel)
	if err != nil {
		return "", err
	}
	switch ext {
	case "txt", "md":
		f, err := os.Open(path)
		if err != nil {
			return "", err
		}
		defer f.Close()
		b, err := io.ReadAll(io.LimitReader(f, docIndexMaxPlainBytes))
		if err != nil {
			return "", err
		}
		return clampText(string(b)), nil
	case "pdf":
		st, err := os.Stat(path)
		if err != nil {
			return "", err
		}
		if st.Size() > docIndexMaxPDFBytes {
			return "", errors.New("pdf too large to index")
		}
		pctx, cancel := context.WithTimeout(ctx, pdfTextTimeout)
		defer cancel()
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

// clampText bounds the stored text and makes it valid UTF-8 without NUL bytes.
func clampText(s string) string {
	if len(s) > docIndexMaxStoredBytes {
		s = s[:docIndexMaxStoredBytes]
	}
	s = strings.ToValidUTF8(s, "")
	if !utf8.ValidString(s) {
		s = ""
	}
	return strings.ReplaceAll(s, "\x00", "")
}
