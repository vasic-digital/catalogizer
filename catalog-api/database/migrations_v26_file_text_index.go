package database

import (
	"context"
	"fmt"
)

// createFileTextIndex (migration v26) adds file_text_index: the text extracted from document files
// (pdf, txt, md) by the post-scan indexer, so that GET /search/files?q= finds words inside the
// documents and not only file names and paths (D4 of docs/qa/live-conv-docs-subs-r1-20261104.md).
// source_size and source_modified record the files row the text was extracted from, so a changed
// file is detected and re-indexed. status is "ok" or "failed" (a failed extraction stores no text).
func (db *DB) createFileTextIndex(ctx context.Context) error {
	ts := "DATETIME"
	if db.dialect.IsPostgres() {
		ts = "TIMESTAMP"
	}
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS file_text_index (
			file_id INTEGER PRIMARY KEY,
			content TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'ok',
			source_size INTEGER NOT NULL DEFAULT 0,
			source_modified TEXT NOT NULL DEFAULT '',
			indexed_at ` + ts + ` DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (file_id) REFERENCES files(id) ON DELETE CASCADE
		)`,
	}
	for _, q := range stmts {
		if _, err := db.ExecContext(ctx, q); err != nil {
			return fmt.Errorf("create file_text_index: %w", err)
		}
	}
	return nil
}
