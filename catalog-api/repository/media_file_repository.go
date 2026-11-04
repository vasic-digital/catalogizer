package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"catalogizer/database"
)

// MediaFileRecord represents a row in the media_files junction table
type MediaFileRecord struct {
	ID          int64
	MediaItemID int64
	FileID      int64
	QualityInfo *string
	Language    *string
	IsPrimary   bool
	CreatedAt   time.Time
}

// DuplicateFileGroup represents a file linked to multiple media items
type DuplicateFileGroup struct {
	FileID    int64
	ItemCount int64
	ItemIDs   []int64
}

// MediaFileRepository handles media_files junction table operations
type MediaFileRepository struct {
	db *database.DB
}

// NewMediaFileRepository creates a new media file repository
func NewMediaFileRepository(db *database.DB) *MediaFileRepository {
	return &MediaFileRepository{db: db}
}

// LinkFileToItem creates an association between a media item and a file
func (r *MediaFileRepository) LinkFileToItem(ctx context.Context, mediaItemID, fileID int64, qualityInfo, language *string, isPrimary bool) (int64, error) {
	query := `
		INSERT INTO media_files (media_item_id, file_id, quality_info, language, is_primary, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`

	id, err := r.db.InsertReturningID(ctx, query,
		mediaItemID, fileID, qualityInfo, language, isPrimary, time.Now().UTC())

	if err != nil {
		return 0, fmt.Errorf("failed to link file to media item: %w", err)
	}

	return id, nil
}

// UnlinkFile removes the association between a media item and a file
func (r *MediaFileRepository) UnlinkFile(ctx context.Context, mediaItemID, fileID int64) error {
	query := `DELETE FROM media_files WHERE media_item_id = ? AND file_id = ?`

	_, err := r.db.ExecContext(ctx, query, mediaItemID, fileID)
	if err != nil {
		return fmt.Errorf("failed to unlink file from media item: %w", err)
	}

	return nil
}

// StreamableFile is a media_file joined with its concrete file
// metadata (name/extension/size). Used by the stream and download
// entity endpoints to pick the best playable file for an entity
// without pulling every row through a second round-trip.
type StreamableFile struct {
	MediaFileID int64
	MediaItemID int64
	FileID      int64
	IsPrimary   bool
	Filename    string
	Extension   string
	Size        int64
	MimeType    string
	Path        string
}

// ErrNoStreamableFile is returned by GetPrimaryStreamableFile
// when the entity has at least one linked file but none of the
// linked files are playable (everything is .DS_Store / Thumbs.db
// / empty). Callers should translate to HTTP 404.
var ErrNoStreamableFile = fmt.Errorf("no streamable file for media item")

// GetPrimaryStreamableFile picks the best file to stream for a
// given media item. It joins media_files to files so the ranking
// can use real filename/extension/size metadata, filters out
// metadata scratch files, and prefers the largest file with a
// known media extension. Selection order:
//
//  1. A file flagged is_primary=true whose name is not a metadata
//     file — trust the scanner's choice when it's sane.
//  2. The largest file with a recognised media extension.
//  3. The largest non-metadata file regardless of extension (for
//     entity types like games/software where we don't know every
//     extension).
//
// Returns ErrNoStreamableFile only when the entity has no linked
// files at all or every linked file is on the metadata blacklist.
func (r *MediaFileRepository) GetPrimaryStreamableFile(ctx context.Context, mediaItemID int64) (*StreamableFile, error) {
	query := `
		SELECT mf.id, mf.media_item_id, mf.file_id, mf.is_primary,
		       f.name, f.extension, f.size, f.mime_type, f.path
		FROM media_files mf
		JOIN files f ON f.id = mf.file_id
		WHERE mf.media_item_id = ?
		ORDER BY mf.is_primary DESC, f.size DESC`

	rows, err := r.db.QueryContext(ctx, query, mediaItemID)
	if err != nil {
		return nil, fmt.Errorf("query streamable files: %w", err)
	}
	defer rows.Close()

	files := []StreamableFile{}
	for rows.Next() {
		var f StreamableFile
		var name, ext, mime, path sql.NullString
		var size sql.NullInt64
		if err := rows.Scan(
			&f.MediaFileID, &f.MediaItemID, &f.FileID, &f.IsPrimary,
			&name, &ext, &size, &mime, &path,
		); err != nil {
			return nil, fmt.Errorf("scan streamable file: %w", err)
		}
		f.Filename = name.String
		f.Extension = ext.String
		f.Size = size.Int64
		f.MimeType = mime.String
		f.Path = path.String
		files = append(files, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate streamable files: %w", err)
	}
	if len(files) == 0 {
		return nil, ErrNoStreamableFile
	}

	pick := pickBestStreamableFile(files)
	if pick == nil {
		return nil, ErrNoStreamableFile
	}
	return pick, nil
}

// mediaExts is the extension whitelist used by
// pickBestStreamableFile to decide whether a file "looks" like
// something worth streaming.
var mediaExts = map[string]struct{}{
	// Video
	"mp4": {}, "mkv": {}, "avi": {}, "mov": {}, "wmv": {},
	"flv": {}, "webm": {}, "m4v": {}, "mpg": {}, "mpeg": {},
	"ts": {}, "m2ts": {}, "vob": {}, "3gp": {}, "ogv": {},
	// Audio
	"mp3": {}, "flac": {}, "wav": {}, "m4a": {}, "aac": {},
	"ogg": {}, "opus": {}, "wma": {}, "alac": {}, "ape": {},
	// E-books / comics — full-file download + client-side reader.
	"pdf": {}, "epub": {}, "mobi": {}, "azw3": {}, "djvu": {},
	"cbr": {}, "cbz": {}, "cb7": {}, "cbt": {},
}

// nonMediaNames lists filenames we never consider streamable —
// macOS, Windows and common scanner scratch files.
var nonMediaNames = map[string]struct{}{
	".ds_store":    {},
	"thumbs.db":    {},
	".thumbs.db":   {},
	"desktop.ini":  {},
	".nomedia":     {},
	".localized":   {},
	".appledouble": {},
}

// pickBestStreamableFile applies the selection rules described
// on GetPrimaryStreamableFile. Exported unit tests in
// media_file_repository_test.go exercise each branch.
func pickBestStreamableFile(files []StreamableFile) *StreamableFile {
	if len(files) == 0 {
		return nil
	}

	isNonMedia := func(f *StreamableFile) bool {
		name := strings.ToLower(strings.TrimSpace(f.Filename))
		if _, ok := nonMediaNames[name]; ok {
			return true
		}
		if strings.HasPrefix(name, "._") {
			return true
		}
		ext := strings.ToLower(strings.TrimPrefix(f.Extension, "."))
		// .nfo and subtitle-only extensions never play as the
		// primary stream for a movie/episode. We still keep them
		// in the files array for UI listing, just not as primary.
		switch ext {
		case "nfo", "srt", "sub", "idx", "vtt", "ass", "ssa":
			return true
		}
		// Zero-byte files are always wrong choices.
		if f.Size == 0 {
			return true
		}
		return false
	}

	isMedia := func(f *StreamableFile) bool {
		ext := strings.ToLower(strings.TrimPrefix(f.Extension, "."))
		if _, ok := mediaExts[ext]; ok {
			return true
		}
		// Fallback on MIME type when extension is missing but the
		// scanner still tagged the file.
		mime := strings.ToLower(f.MimeType)
		return strings.HasPrefix(mime, "video/") ||
			strings.HasPrefix(mime, "audio/") ||
			mime == "application/pdf" ||
			mime == "application/epub+zip"
	}

	// Pass 1: honour is_primary if the flagged file is sane.
	for i := range files {
		if files[i].IsPrimary && !isNonMedia(&files[i]) {
			return &files[i]
		}
	}

	// Pass 2: largest media-extension file that survives the
	// blacklist.
	var best *StreamableFile
	for i := range files {
		f := &files[i]
		if isNonMedia(f) || !isMedia(f) {
			continue
		}
		if best == nil || f.Size > best.Size {
			best = f
		}
	}
	if best != nil {
		return best
	}

	// Pass 3: largest non-blacklisted file regardless of
	// extension.
	for i := range files {
		f := &files[i]
		if isNonMedia(f) {
			continue
		}
		if best == nil || f.Size > best.Size {
			best = f
		}
	}
	return best
}

// GetFilesByItem retrieves all media_file records for a given media item
func (r *MediaFileRepository) GetFilesByItem(ctx context.Context, mediaItemID int64) ([]MediaFileRecord, error) {
	query := `
		SELECT id, media_item_id, file_id, quality_info, language, is_primary, created_at
		FROM media_files
		WHERE media_item_id = ?
		ORDER BY is_primary DESC, created_at ASC`

	rows, err := r.db.QueryContext(ctx, query, mediaItemID)
	if err != nil {
		return nil, fmt.Errorf("failed to get files for media item: %w", err)
	}
	defer rows.Close()

	records := []MediaFileRecord{}
	for rows.Next() {
		var rec MediaFileRecord
		err := rows.Scan(
			&rec.ID, &rec.MediaItemID, &rec.FileID,
			&rec.QualityInfo, &rec.Language, &rec.IsPrimary, &rec.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan media file record: %w", err)
		}
		records = append(records, rec)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating media file rows: %w", err)
	}

	return records, nil
}

// GetItemByFile returns all media item IDs linked to a given file
func (r *MediaFileRepository) GetItemByFile(ctx context.Context, fileID int64) ([]int64, error) {
	query := `
		SELECT media_item_id
		FROM media_files
		WHERE file_id = ?
		ORDER BY created_at ASC`

	rows, err := r.db.QueryContext(ctx, query, fileID)
	if err != nil {
		return nil, fmt.Errorf("failed to get media items for file: %w", err)
	}
	defer rows.Close()

	itemIDs := []int64{}
	for rows.Next() {
		var itemID int64
		if err := rows.Scan(&itemID); err != nil {
			return nil, fmt.Errorf("failed to scan media item ID: %w", err)
		}
		itemIDs = append(itemIDs, itemID)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating media item rows: %w", err)
	}

	return itemIDs, nil
}

// GetDuplicateFiles finds files that are linked to multiple media items
func (r *MediaFileRepository) GetDuplicateFiles(ctx context.Context) ([]DuplicateFileGroup, error) {
	query := `
		SELECT file_id, COUNT(*) as item_count
		FROM media_files
		GROUP BY file_id
		HAVING COUNT(*) > 1`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query duplicate files: %w", err)
	}
	defer rows.Close()

	groups := []DuplicateFileGroup{}
	for rows.Next() {
		var group DuplicateFileGroup
		if err := rows.Scan(&group.FileID, &group.ItemCount); err != nil {
			return nil, fmt.Errorf("failed to scan duplicate file group: %w", err)
		}
		groups = append(groups, group)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating duplicate file rows: %w", err)
	}

	// For each duplicate file, fetch the linked item IDs
	itemQuery := `
		SELECT media_item_id
		FROM media_files
		WHERE file_id = ?
		ORDER BY created_at ASC`

	for i := range groups {
		itemRows, err := r.db.QueryContext(ctx, itemQuery, groups[i].FileID)
		if err != nil {
			return nil, fmt.Errorf("failed to query item IDs for file %d: %w", groups[i].FileID, err)
		}

		for itemRows.Next() {
			var itemID int64
			if err := itemRows.Scan(&itemID); err != nil {
				itemRows.Close()
				return nil, fmt.Errorf("failed to scan item ID for file %d: %w", groups[i].FileID, err)
			}
			groups[i].ItemIDs = append(groups[i].ItemIDs, itemID)
		}

		if err = itemRows.Err(); err != nil {
			itemRows.Close()
			return nil, fmt.Errorf("error iterating item IDs for file %d: %w", groups[i].FileID, err)
		}
		itemRows.Close()
	}

	return groups, nil
}

// CountByItem returns the number of files linked to a media item
func (r *MediaFileRepository) CountByItem(ctx context.Context, mediaItemID int64) (int64, error) {
	query := `SELECT COUNT(*) FROM media_files WHERE media_item_id = ?`

	var count int64
	err := r.db.QueryRowContext(ctx, query, mediaItemID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count files for media item: %w", err)
	}

	return count, nil
}

// SetPrimary sets a specific file as the primary file for a media item,
// clearing the primary flag on all other files for that item
func (r *MediaFileRepository) SetPrimary(ctx context.Context, mediaItemID, fileID int64) error {
	// First, clear is_primary for all files of this item
	clearQuery := `UPDATE media_files SET is_primary = ? WHERE media_item_id = ?`
	_, err := r.db.ExecContext(ctx, clearQuery, false, mediaItemID)
	if err != nil {
		return fmt.Errorf("failed to clear primary flag for media item: %w", err)
	}

	// Then, set is_primary for the specified file
	setQuery := `UPDATE media_files SET is_primary = ? WHERE media_item_id = ? AND file_id = ?`
	result, err := r.db.ExecContext(ctx, setQuery, true, mediaItemID, fileID)
	if err != nil {
		return fmt.Errorf("failed to set primary file: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	return nil
}
