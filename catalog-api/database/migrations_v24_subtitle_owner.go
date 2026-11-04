package database

import (
	"context"
	"fmt"
)

// addSubtitleUploadedBy (migration v24) records who uploaded a subtitle track so
// that DELETE /subtitles/:id can be limited to the uploader or an administrator.
// Existing rows keep working: they have no uploader (NULL) and are administrator-only.
func (db *DB) addSubtitleUploadedBy(ctx context.Context) error {
	if db.dialect.IsPostgres() {
		if _, err := db.ExecContext(ctx, `ALTER TABLE subtitle_tracks ADD COLUMN IF NOT EXISTS uploaded_by INTEGER`); err != nil {
			return fmt.Errorf("postgres: subtitle_tracks.uploaded_by: %w", err)
		}
		return nil
	}
	has, err := db.columnExistsSQLite(ctx, "subtitle_tracks", "uploaded_by")
	if err != nil {
		return fmt.Errorf("sqlite: probe subtitle_tracks.uploaded_by: %w", err)
	}
	if has {
		return nil
	}
	if _, err := db.ExecContext(ctx, `ALTER TABLE subtitle_tracks ADD COLUMN uploaded_by INTEGER`); err != nil {
		return fmt.Errorf("sqlite: add subtitle_tracks.uploaded_by: %w", err)
	}
	return nil
}
