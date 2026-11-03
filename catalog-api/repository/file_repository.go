package repository

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"

	"catalogizer/database"
	"catalogizer/models"
)

// FileRepository handles file-related database operations
type FileRepository struct {
	db *database.DB
}

// NewFileRepository creates a new file repository
func NewFileRepository(db *database.DB) *FileRepository {
	return &FileRepository{db: db}
}

// GetFileByID retrieves a file by its ID
func (r *FileRepository) GetFileByID(ctx context.Context, id int64) (*models.FileWithMetadata, error) {
	query := `
		SELECT f.id, f.storage_root_id, sr.name as storage_root_name, f.path, f.name, f.extension,
			   f.mime_type, f.file_type, f.size, f.is_directory, f.created_at, f.modified_at,
			   f.accessed_at, f.deleted, f.deleted_at, f.last_scan_at, f.last_verified_at,
			   f.md5, f.sha256, f.sha1, f.blake3, f.quick_hash, f.is_duplicate,
			   f.duplicate_group_id, f.parent_id
		FROM files f
		JOIN storage_roots sr ON f.storage_root_id = sr.id
		WHERE f.id = ?`

	var file models.File
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&file.ID, &file.StorageRootID, &file.StorageRootName, &file.Path, &file.Name,
		&file.Extension, &file.MimeType, &file.FileType, &file.Size, &file.IsDirectory,
		&file.CreatedAt, &file.ModifiedAt, &file.AccessedAt, &file.Deleted,
		&file.DeletedAt, &file.LastScanAt, &file.LastVerifiedAt, &file.MD5,
		&file.SHA256, &file.SHA1, &file.BLAKE3, &file.QuickHash, &file.IsDuplicate,
		&file.DuplicateGroupID, &file.ParentID,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("file not found")
		}
		return nil, fmt.Errorf("failed to get file: %w", err)
	}

	// Get metadata
	metadata, err := r.getFileMetadata(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get file metadata: %w", err)
	}

	return &models.FileWithMetadata{
		File:     file,
		Metadata: metadata,
	}, nil
}

// GetDirectoryContents retrieves files and directories within a path
func (r *FileRepository) GetDirectoryContents(ctx context.Context, storageRootName, path string, pagination models.PaginationOptions, sort models.SortOptions) (*models.SearchResult, error) {
	// Build the base query
	baseQuery := `
		FROM files f
		JOIN storage_roots sr ON f.storage_root_id = sr.id
		WHERE sr.name = ? AND f.deleted = 0`

	args := []interface{}{storageRootName}

	// Handle path filtering
	if path == "/" || path == "" {
		baseQuery += " AND f.parent_id IS NULL"
	} else {
		baseQuery += " AND f.path LIKE ?"
		args = append(args, path+"/%")
		// Ensure we only get direct children, not deeper descendants
		baseQuery += " AND f.path NOT LIKE ?"
		args = append(args, path+"/%/%")
	}

	// Count total records
	countQuery := "SELECT COUNT(*) " + baseQuery
	var totalCount int64
	err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&totalCount)
	if err != nil {
		return nil, fmt.Errorf("failed to count files: %w", err)
	}

	// Build the main query with sorting and pagination
	selectQuery := `
		SELECT f.id, f.storage_root_id, sr.name as storage_root_name, f.path, f.name, f.extension,
			   f.mime_type, f.file_type, f.size, f.is_directory, f.created_at, f.modified_at,
			   f.accessed_at, f.deleted, f.deleted_at, f.last_scan_at, f.last_verified_at,
			   f.md5, f.sha256, f.sha1, f.blake3, f.quick_hash, f.is_duplicate,
			   f.duplicate_group_id, f.parent_id ` + baseQuery

	// Add sorting
	selectQuery += r.buildSortClause(sort)

	// Add pagination
	selectQuery += " LIMIT ? OFFSET ?"
	offset := (pagination.Page - 1) * pagination.Limit
	args = append(args, pagination.Limit, offset)

	rows, err := r.db.QueryContext(ctx, selectQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query files: %w", err)
	}
	defer rows.Close()

	var files []models.FileWithMetadata
	for rows.Next() {
		var file models.File
		err := rows.Scan(
			&file.ID, &file.StorageRootID, &file.StorageRootName, &file.Path, &file.Name,
			&file.Extension, &file.MimeType, &file.FileType, &file.Size, &file.IsDirectory,
			&file.CreatedAt, &file.ModifiedAt, &file.AccessedAt, &file.Deleted,
			&file.DeletedAt, &file.LastScanAt, &file.LastVerifiedAt, &file.MD5,
			&file.SHA256, &file.SHA1, &file.BLAKE3, &file.QuickHash, &file.IsDuplicate,
			&file.DuplicateGroupID, &file.ParentID,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan file: %w", err)
		}

		files = append(files, models.FileWithMetadata{File: file})
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	totalPages := int((totalCount + int64(pagination.Limit) - 1) / int64(pagination.Limit))

	return &models.SearchResult{
		Files:      files,
		TotalCount: totalCount,
		Page:       pagination.Page,
		Limit:      pagination.Limit,
		TotalPages: totalPages,
	}, nil
}

// SearchFiles performs advanced file search
func (r *FileRepository) SearchFiles(ctx context.Context, filter models.SearchFilter, pagination models.PaginationOptions, sort models.SortOptions) (*models.SearchResult, error) {
	// Build the base query
	baseQuery := `
		FROM files f
		JOIN storage_roots sr ON f.storage_root_id = sr.id
		WHERE 1=1`

	args := []interface{}{}

	// Apply filters
	baseQuery, args = r.applySearchFilters(baseQuery, args, filter)

	// Count total records
	countQuery := "SELECT COUNT(*) " + baseQuery
	var totalCount int64
	err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&totalCount)
	if err != nil {
		return nil, fmt.Errorf("failed to count search results: %w", err)
	}

	// Build the main query with sorting and pagination
	selectQuery := `
		SELECT f.id, f.storage_root_id, sr.name as storage_root_name, f.path, f.name, f.extension,
			   f.mime_type, f.file_type, f.size, f.is_directory, f.created_at, f.modified_at,
			   f.accessed_at, f.deleted, f.deleted_at, f.last_scan_at, f.last_verified_at,
			   f.md5, f.sha256, f.sha1, f.blake3, f.quick_hash, f.is_duplicate,
			   f.duplicate_group_id, f.parent_id ` + baseQuery

	// Add sorting
	selectQuery += r.buildSortClause(sort)

	// Add pagination
	selectQuery += " LIMIT ? OFFSET ?"
	offset := (pagination.Page - 1) * pagination.Limit
	args = append(args, pagination.Limit, offset)

	rows, err := r.db.QueryContext(ctx, selectQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute search query: %w", err)
	}
	defer rows.Close()

	// Read every row of the page FIRST and close the cursor, then fetch per-file metadata.
	// Running the metadata query while this cursor is still open made each in-flight search hold
	// one pooled connection and wait for a second: with more concurrent searches than pool
	// connections every search waited on another (pool deadlock, found by the live scaling suite).
	var page []models.File
	for rows.Next() {
		var file models.File
		err := rows.Scan(
			&file.ID, &file.StorageRootID, &file.StorageRootName, &file.Path, &file.Name,
			&file.Extension, &file.MimeType, &file.FileType, &file.Size, &file.IsDirectory,
			&file.CreatedAt, &file.ModifiedAt, &file.AccessedAt, &file.Deleted,
			&file.DeletedAt, &file.LastScanAt, &file.LastVerifiedAt, &file.MD5,
			&file.SHA256, &file.SHA1, &file.BLAKE3, &file.QuickHash, &file.IsDuplicate,
			&file.DuplicateGroupID, &file.ParentID,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan search result: %w", err)
		}
		page = append(page, file)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating search results: %w", err)
	}
	rows.Close()

	var files []models.FileWithMetadata
	for _, file := range page {
		// Get metadata for search results if needed
		metadata, err := r.getFileMetadata(ctx, file.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get file metadata: %w", err)
		}

		files = append(files, models.FileWithMetadata{
			File:     file,
			Metadata: metadata,
		})
	}

	totalPages := int((totalCount + int64(pagination.Limit) - 1) / int64(pagination.Limit))

	return &models.SearchResult{
		Files:      files,
		TotalCount: totalCount,
		Page:       pagination.Page,
		Limit:      pagination.Limit,
		TotalPages: totalPages,
	}, nil
}

// GetDirectoriesSortedBySize retrieves directories sorted by total size
func (r *FileRepository) GetDirectoriesSortedBySize(ctx context.Context, storageRootName string, pagination models.PaginationOptions, ascending bool) ([]models.DirectoryInfo, error) {
	order := "DESC"
	if ascending {
		order = "ASC"
	}

	query := `
		WITH RECURSIVE directory_tree AS (
			SELECT f.path, f.name, sr.name as storage_root_name,
				   COUNT(CASE WHEN f2.is_directory = 0 THEN 1 END) as file_count,
				   COUNT(CASE WHEN f2.is_directory = 1 THEN 1 END) as directory_count,
				   COALESCE(SUM(CASE WHEN f2.is_directory = 0 THEN f2.size ELSE 0 END), 0) as total_size,
				   COUNT(CASE WHEN f2.is_duplicate = 1 THEN 1 END) as duplicate_count,
				   MAX(f.modified_at) as modified_at
			FROM files f
			JOIN storage_roots sr ON f.storage_root_id = sr.id
			LEFT JOIN files f2 ON f2.path LIKE f.path || '/%' AND f2.storage_root_id = f.storage_root_id AND f2.deleted = 0
			WHERE f.is_directory = 1 AND f.deleted = 0 AND sr.name = ?
			GROUP BY f.path, f.name, sr.name
		)
		SELECT path, name, storage_root_name, file_count, directory_count, total_size, duplicate_count, modified_at
		FROM directory_tree
		ORDER BY total_size ` + order + `
		LIMIT ? OFFSET ?`

	offset := (pagination.Page - 1) * pagination.Limit
	rows, err := r.db.QueryContext(ctx, query, storageRootName, pagination.Limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query directories by size: %w", err)
	}
	defer rows.Close()

	var directories []models.DirectoryInfo
	for rows.Next() {
		var dir models.DirectoryInfo
		err := rows.Scan(
			&dir.Path, &dir.Name, &dir.StorageRootName, &dir.FileCount,
			&dir.DirectoryCount, &dir.TotalSize, &dir.DuplicateCount, &dir.ModifiedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan directory info: %w", err)
		}
		directories = append(directories, dir)
	}

	return directories, nil
}

// GetDirectoriesSortedByDuplicates retrieves directories sorted by duplicate count
func (r *FileRepository) GetDirectoriesSortedByDuplicates(ctx context.Context, storageRootName string, pagination models.PaginationOptions, ascending bool) ([]models.DirectoryInfo, error) {
	order := "DESC"
	if ascending {
		order = "ASC"
	}

	query := `
		WITH RECURSIVE directory_tree AS (
			SELECT f.path, f.name, sr.name as storage_root_name,
				   COUNT(CASE WHEN f2.is_directory = 0 THEN 1 END) as file_count,
				   COUNT(CASE WHEN f2.is_directory = 1 THEN 1 END) as directory_count,
				   COALESCE(SUM(CASE WHEN f2.is_directory = 0 THEN f2.size ELSE 0 END), 0) as total_size,
				   COUNT(CASE WHEN f2.is_duplicate = 1 THEN 1 END) as duplicate_count,
				   MAX(f.modified_at) as modified_at
			FROM files f
			JOIN storage_roots sr ON f.storage_root_id = sr.id
			LEFT JOIN files f2 ON f2.path LIKE f.path || '/%' AND f2.storage_root_id = f.storage_root_id AND f2.deleted = 0
			WHERE f.is_directory = 1 AND f.deleted = 0 AND sr.name = ?
			GROUP BY f.path, f.name, sr.name
		)
		SELECT path, name, storage_root_name, file_count, directory_count, total_size, duplicate_count, modified_at
		FROM directory_tree
		ORDER BY duplicate_count ` + order + `
		LIMIT ? OFFSET ?`

	offset := (pagination.Page - 1) * pagination.Limit
	rows, err := r.db.QueryContext(ctx, query, storageRootName, pagination.Limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query directories by duplicates: %w", err)
	}
	defer rows.Close()

	var directories []models.DirectoryInfo
	for rows.Next() {
		var dir models.DirectoryInfo
		err := rows.Scan(
			&dir.Path, &dir.Name, &dir.StorageRootName, &dir.FileCount,
			&dir.DirectoryCount, &dir.TotalSize, &dir.DuplicateCount, &dir.ModifiedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan directory info: %w", err)
		}
		directories = append(directories, dir)
	}

	return directories, nil
}

// GetStorageRoots retrieves all storage roots
func (r *FileRepository) GetStorageRoots(ctx context.Context) ([]models.StorageRoot, error) {
	query := `
		SELECT id, name, protocol, host, port, path, username, password, domain,
			   mount_point, options, url, enabled, max_depth,
			   enable_duplicate_detection, enable_metadata_extraction, include_patterns,
			   exclude_patterns, created_at, updated_at, last_scan_at
		FROM storage_roots
		ORDER BY name`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query storage roots: %w", err)
	}
	defer rows.Close()

	var roots []models.StorageRoot
	for rows.Next() {
		var root models.StorageRoot
		err := rows.Scan(
			&root.ID, &root.Name, &root.Protocol, &root.Host, &root.Port, &root.Path,
			&root.Username, &root.Password, &root.Domain, &root.MountPoint, &root.Options,
			&root.URL, &root.Enabled, &root.MaxDepth, &root.EnableDuplicateDetection,
			&root.EnableMetadataExtraction, &root.IncludePatterns, &root.ExcludePatterns,
			&root.CreatedAt, &root.UpdatedAt, &root.LastScanAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan storage root: %w", err)
		}
		roots = append(roots, root)
	}

	return roots, nil
}

// Helper methods

func (r *FileRepository) getFileMetadata(ctx context.Context, fileID int64) ([]models.FileMetadata, error) {
	query := `
		SELECT id, file_id, key, value, data_type
		FROM file_metadata
		WHERE file_id = ?
		ORDER BY key`

	rows, err := r.db.QueryContext(ctx, query, fileID)
	if err != nil {
		return nil, fmt.Errorf("failed to query file metadata: %w", err)
	}
	defer rows.Close()

	var metadata []models.FileMetadata
	for rows.Next() {
		var meta models.FileMetadata
		err := rows.Scan(&meta.ID, &meta.FileID, &meta.Key, &meta.Value, &meta.DataType)
		if err != nil {
			return nil, fmt.Errorf("failed to scan metadata: %w", err)
		}
		metadata = append(metadata, meta)
	}

	return metadata, nil
}

func (r *FileRepository) buildSortClause(sort models.SortOptions) string {
	var clause strings.Builder
	clause.WriteString(" ORDER BY ")

	switch sort.Field {
	case "name":
		clause.WriteString("f.name")
	case "size":
		clause.WriteString("f.size")
	case "modified_at":
		clause.WriteString("f.modified_at")
	case "created_at":
		clause.WriteString("f.created_at")
	case "path":
		clause.WriteString("f.path")
	case "extension":
		clause.WriteString("f.extension")
	default:
		clause.WriteString("f.name") // Default sort
	}

	if sort.Order == "desc" {
		clause.WriteString(" DESC")
	} else {
		clause.WriteString(" ASC")
	}

	return clause.String()
}

func (r *FileRepository) applySearchFilters(baseQuery string, args []interface{}, filter models.SearchFilter) (string, []interface{}) {
	if !filter.IncludeDeleted {
		baseQuery += " AND f.deleted = 0"
	}

	if filter.Query != "" {
		baseQuery += " AND (f.name LIKE ? OR f.path LIKE ?)"
		searchPattern := "%" + filter.Query + "%"
		args = append(args, searchPattern, searchPattern)
	}

	if filter.Path != "" {
		baseQuery += " AND f.path LIKE ?"
		args = append(args, "%"+filter.Path+"%")
	}

	if filter.Name != "" {
		baseQuery += " AND f.name LIKE ?"
		args = append(args, "%"+filter.Name+"%")
	}

	if filter.Extension != "" {
		baseQuery += " AND f.extension = ?"
		args = append(args, filter.Extension)
	}

	if filter.FileType != "" {
		baseQuery += " AND f.file_type = ?"
		args = append(args, filter.FileType)
	}

	if filter.MimeType != "" {
		baseQuery += " AND f.mime_type = ?"
		args = append(args, filter.MimeType)
	}

	if len(filter.StorageRoots) > 0 {
		placeholders := strings.Repeat("?,", len(filter.StorageRoots))
		placeholders = placeholders[:len(placeholders)-1] // Remove trailing comma
		baseQuery += " AND sr.name IN (" + placeholders + ")"
		for _, root := range filter.StorageRoots {
			args = append(args, root)
		}
	}

	if filter.MinSize != nil {
		baseQuery += " AND f.size >= ?"
		args = append(args, *filter.MinSize)
	}

	if filter.MaxSize != nil {
		baseQuery += " AND f.size <= ?"
		args = append(args, *filter.MaxSize)
	}

	if filter.ModifiedAfter != nil {
		baseQuery += " AND f.modified_at >= ?"
		args = append(args, *filter.ModifiedAfter)
	}

	if filter.ModifiedBefore != nil {
		baseQuery += " AND f.modified_at <= ?"
		args = append(args, *filter.ModifiedBefore)
	}

	if filter.OnlyDuplicates {
		baseQuery += " AND f.is_duplicate = 1"
	} else if filter.ExcludeDuplicates {
		baseQuery += " AND f.is_duplicate = 0"
	}

	if !filter.IncludeDirectories {
		baseQuery += " AND f.is_directory = 0"
	}

	return baseQuery, args
}

// UpdateFilePath updates a file's path and related metadata efficiently
func (r *FileRepository) UpdateFilePath(ctx context.Context, fileID int64, newPath string) error {
	// Extract new filename and directory info
	newName := filepath.Base(newPath)
	newDir := filepath.Dir(newPath)

	// Get parent directory ID
	var parentID *int64
	if newDir != "/" && newDir != "." {
		parentQuery := `SELECT id FROM files WHERE path = ? AND is_directory = 1 LIMIT 1`
		err := r.db.QueryRowContext(ctx, parentQuery, newDir).Scan(&parentID)
		if err != nil && err != sql.ErrNoRows {
			return fmt.Errorf("failed to get parent directory: %w", err)
		}
	}

	// Update file record
	updateQuery := `
		UPDATE files
		SET path = ?, name = ?, parent_id = ?, modified_at = CURRENT_TIMESTAMP,
		    last_scan_at = CURRENT_TIMESTAMP
		WHERE id = ?`

	_, err := r.db.ExecContext(ctx, updateQuery, newPath, newName, parentID, fileID)
	if err != nil {
		return fmt.Errorf("failed to update file path: %w", err)
	}

	return nil
}

// UpdateDirectoryPaths updates a directory and all its children paths efficiently
func (r *FileRepository) UpdateDirectoryPaths(ctx context.Context, oldPath, newPath, storageRootName string) error {
	// Get all files/directories that need to be updated
	query := `
		SELECT id, path, is_directory
		FROM files
		WHERE storage_root_id = (SELECT id FROM storage_roots WHERE name = ?)
		  AND (path = ? OR path LIKE ?)
		ORDER BY LENGTH(path) ASC` // Process parents before children

	oldPathPattern := oldPath + "/%"
	rows, err := r.db.QueryContext(ctx, query, storageRootName, oldPath, oldPathPattern)
	if err != nil {
		return fmt.Errorf("failed to query directory contents: %w", err)
	}
	defer rows.Close()

	type fileUpdate struct {
		ID          int64
		OldPath     string
		IsDirectory bool
	}

	var updates []fileUpdate
	for rows.Next() {
		var update fileUpdate
		if err := rows.Scan(&update.ID, &update.OldPath, &update.IsDirectory); err != nil {
			return fmt.Errorf("failed to scan file for update: %w", err)
		}
		updates = append(updates, update)
	}

	// Update each file/directory path
	for _, update := range updates {
		var updatedPath string
		if update.OldPath == oldPath {
			// This is the directory itself
			updatedPath = newPath
		} else {
			// This is a child - replace the old path prefix with new path
			relativePath := update.OldPath[len(oldPath):]
			updatedPath = newPath + relativePath
		}

		// Update the file record using the existing method
		if err := r.UpdateFilePath(ctx, update.ID, updatedPath); err != nil {
			return fmt.Errorf("failed to update path for file ID %d: %w", update.ID, err)
		}
	}

	return nil
}

// GetFileByPathAndStorage retrieves a file by path and storage root
func (r *FileRepository) GetFileByPathAndStorage(ctx context.Context, path, storageRootName string) (*models.File, error) {
	query := `
		SELECT f.id, f.storage_root_id, sr.name as storage_root_name, f.path, f.name, f.extension,
		       f.mime_type, f.file_type, f.size, f.is_directory, f.created_at, f.modified_at,
		       f.accessed_at, f.deleted, f.deleted_at, f.last_scan_at, f.last_verified_at,
		       f.md5, f.sha256, f.sha1, f.blake3, f.quick_hash, f.is_duplicate,
		       f.duplicate_group_id, f.parent_id
		FROM files f
		JOIN storage_roots sr ON f.storage_root_id = sr.id
		WHERE f.path = ? AND sr.name = ?`

	var file models.File
	err := r.db.QueryRowContext(ctx, query, path, storageRootName).Scan(
		&file.ID, &file.StorageRootID, &file.StorageRootName, &file.Path, &file.Name,
		&file.Extension, &file.MimeType, &file.FileType, &file.Size, &file.IsDirectory,
		&file.CreatedAt, &file.ModifiedAt, &file.AccessedAt, &file.Deleted,
		&file.DeletedAt, &file.LastScanAt, &file.LastVerifiedAt, &file.MD5,
		&file.SHA256, &file.SHA1, &file.BLAKE3, &file.QuickHash, &file.IsDuplicate,
		&file.DuplicateGroupID, &file.ParentID,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get file: %w", err)
	}

	return &file, nil
}

// MarkFileAsDeleted marks a file as deleted instead of removing it immediately
func (r *FileRepository) MarkFileAsDeleted(ctx context.Context, fileID int64) error {
	query := `
		UPDATE files
		SET deleted = 1, deleted_at = CURRENT_TIMESTAMP
		WHERE id = ?`

	_, err := r.db.ExecContext(ctx, query, fileID)
	if err != nil {
		return fmt.Errorf("failed to mark file as deleted: %w", err)
	}

	return nil
}

// RestoreDeletedFile restores a file that was marked as deleted
func (r *FileRepository) RestoreDeletedFile(ctx context.Context, fileID int64) error {
	query := `
		UPDATE files
		SET deleted = 0, deleted_at = NULL, last_scan_at = CURRENT_TIMESTAMP
		WHERE id = ?`

	_, err := r.db.ExecContext(ctx, query, fileID)
	if err != nil {
		return fmt.Errorf("failed to restore deleted file: %w", err)
	}

	return nil
}

// GetFilesWithHash finds files by their hash for duplicate detection and move tracking
func (r *FileRepository) GetFilesWithHash(ctx context.Context, hash string, storageRootName string) ([]models.File, error) {
	query := `
		SELECT f.id, f.storage_root_id, sr.name as storage_root_name, f.path, f.name, f.extension,
		       f.mime_type, f.file_type, f.size, f.is_directory, f.created_at, f.modified_at,
		       f.accessed_at, f.deleted, f.deleted_at, f.last_scan_at, f.last_verified_at,
		       f.md5, f.sha256, f.sha1, f.blake3, f.quick_hash, f.is_duplicate,
		       f.duplicate_group_id, f.parent_id
		FROM files f
		JOIN storage_roots sr ON f.storage_root_id = sr.id
		WHERE (f.md5 = ? OR f.sha256 = ? OR f.sha1 = ? OR f.blake3 = ? OR f.quick_hash = ?)
		  AND sr.name = ?
		  AND f.deleted = 0`

	rows, err := r.db.QueryContext(ctx, query, hash, hash, hash, hash, hash, storageRootName)
	if err != nil {
		return nil, fmt.Errorf("failed to query files by hash: %w", err)
	}
	defer rows.Close()

	var files []models.File
	for rows.Next() {
		var file models.File
		err := rows.Scan(
			&file.ID, &file.StorageRootID, &file.StorageRootName, &file.Path, &file.Name,
			&file.Extension, &file.MimeType, &file.FileType, &file.Size, &file.IsDirectory,
			&file.CreatedAt, &file.ModifiedAt, &file.AccessedAt, &file.Deleted,
			&file.DeletedAt, &file.LastScanAt, &file.LastVerifiedAt, &file.MD5,
			&file.SHA256, &file.SHA1, &file.BLAKE3, &file.QuickHash, &file.IsDuplicate,
			&file.DuplicateGroupID, &file.ParentID,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan file: %w", err)
		}
		files = append(files, file)
	}

	return files, nil
}

// UpdateFileMetadata updates file metadata without triggering a full rescan
func (r *FileRepository) UpdateFileMetadata(ctx context.Context, fileID int64, size int64, hash *string) error {
	query := `
		UPDATE files
		SET size = ?, quick_hash = ?, last_scan_at = CURRENT_TIMESTAMP, modified_at = CURRENT_TIMESTAMP
		WHERE id = ?`

	_, err := r.db.ExecContext(ctx, query, size, hash, fileID)
	if err != nil {
		return fmt.Errorf("failed to update file metadata: %w", err)
	}

	return nil
}
