package services

import (
	"catalogizer/database"
	"catalogizer/internal/config"
	"catalogizer/internal/models"
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"go.uber.org/zap"
)

// CatalogServiceInterface defines the interface for catalog operations
type CatalogServiceInterface interface {
	SetDB(db *database.DB)
	ListPath(path string, sortBy string, sortOrder string, limit, offset int) ([]models.FileInfo, error)
	GetFileInfo(pathOrID string) (*models.FileInfo, error)
	SearchFiles(req *models.SearchRequest) ([]models.FileInfo, int64, error)
	GetDirectoriesBySize(smbRoot string, limit int) ([]models.DirectoryStats, error)
	GetDuplicateGroups(smbRoot string, minCount int, limit int) ([]models.DuplicateGroup, error)
	GetSMBRoots() ([]string, error)
	ListDirectory(path string) ([]models.FileInfo, error)
	Search(query string, fileType string, limit int, offset int) ([]models.FileInfo, error)
	SearchDuplicates() ([]models.DuplicateGroup, error)
	GetFileInfoByPath(path string) (*models.FileInfo, error)
	GetDuplicatesCount() (int64, error)
	GetDirectoriesBySizeLimited(limit int) ([]models.DirectoryStats, error)
}

type CatalogService struct {
	db     *database.DB
	config *config.Config
	logger *zap.Logger
}

func NewCatalogService(cfg *config.Config, logger *zap.Logger) *CatalogService {
	return &CatalogService{
		config: cfg,
		logger: logger,
	}
}

func (s *CatalogService) SetDB(db *database.DB) {
	s.db = db
}

func (s *CatalogService) ListPath(path string, sortBy string, sortOrder string, limit, offset int) ([]models.FileInfo, error) {
	var query string
	var args []interface{}

	// Check if path exists in database
	var parentID sql.NullInt64
	err := s.db.QueryRow(`SELECT id FROM files WHERE path = ? LIMIT 1`, path).Scan(&parentID)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to check path: %w", err)
	}

	if err == sql.ErrNoRows {
		// Path not in database
		if path == "/" {
			// Root, return top-level directories
			query = `
				SELECT f.id, f.name, f.path, f.is_directory, f.size, f.modified_at, f.quick_hash, f.extension, f.mime_type, f.parent_id, sr.name as smb_root, f.created_at, f.last_scan_at
				FROM files f
				JOIN storage_roots sr ON f.storage_root_id = sr.id
				WHERE f.parent_id IS NULL
			`
		} else {
			return nil, fmt.Errorf("path not found: %s", path)
		}
	} else {
		// Path exists, list its children
		query = `
			SELECT f.id, f.name, f.path, f.is_directory, f.size, f.modified_at, f.quick_hash, f.extension, f.mime_type, f.parent_id, sr.name as smb_root, f.created_at, f.last_scan_at
			FROM files f
			JOIN storage_roots sr ON f.storage_root_id = sr.id
			WHERE f.parent_id = ?
		`
		args = []interface{}{parentID.Int64}
	}

	// Add sorting
	switch sortBy {
	case "name":
		query += " ORDER BY f.name"
	case "size":
		query += " ORDER BY f.size"
	case "modified":
		query += " ORDER BY f.modified_at"
	default:
		query += " ORDER BY f.is_directory DESC, f.name"
	}

	if sortOrder == "desc" {
		query += " DESC"
	} else {
		query += " ASC"
	}

	// Add pagination
	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}
	if offset > 0 {
		query += " OFFSET ?"
		args = append(args, offset)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query files: %w", err)
	}
	defer rows.Close()

	var files []models.FileInfo
	for rows.Next() {
		var file models.FileInfo
		var lastModified sql.NullTime
		var createdAt sql.NullTime
		var updatedAt sql.NullTime
		err := rows.Scan(
			&file.ID, &file.Name, &file.Path, &file.IsDirectory, &file.Size,
			&lastModified, &file.Hash, &file.Extension, &file.MimeType,
			&file.ParentID, &file.SmbRoot, &createdAt, &updatedAt,
		)
		if lastModified.Valid {
			file.LastModified = lastModified.Time
		}
		if createdAt.Valid {
			file.CreatedAt = createdAt.Time
		}
		if updatedAt.Valid {
			file.UpdatedAt = updatedAt.Time
		}
		if err != nil {
			return nil, fmt.Errorf("failed to scan file: %w", err)
		}
		if file.IsDirectory {
			file.Type = "directory"
		} else {
			file.Type = "file"
		}
		files = append(files, file)
	}

	return files, nil
}

func (s *CatalogService) GetFileInfo(pathOrID string) (*models.FileInfo, error) {
	var query string
	var arg interface{}

	// Try to parse as ID first
	if id, err := strconv.ParseInt(pathOrID, 10, 64); err == nil {
		query = `
			SELECT f.id, f.name, f.path, f.is_directory, f.size, f.modified_at, f.quick_hash, f.extension, f.mime_type, f.parent_id, sr.name as smb_root, f.created_at, f.last_scan_at
			FROM files f
			JOIN storage_roots sr ON f.storage_root_id = sr.id
			WHERE f.id = ?
		`
		arg = id
	} else {
		// Treat as path
		query = `
			SELECT f.id, f.name, f.path, f.is_directory, f.size, f.modified_at, f.quick_hash, f.extension, f.mime_type, f.parent_id, sr.name as smb_root, f.created_at, f.last_scan_at
			FROM files f
			JOIN storage_roots sr ON f.storage_root_id = sr.id
			WHERE f.path = ?
		`
		arg = pathOrID
	}

	var file models.FileInfo
	var lastModified sql.NullTime
	var createdAt sql.NullTime
	var updatedAt sql.NullTime
	err := s.db.QueryRow(query, arg).Scan(
		&file.ID, &file.Name, &file.Path, &file.IsDirectory, &file.Size,
		&lastModified, &file.Hash, &file.Extension, &file.MimeType,
		&file.ParentID, &file.SmbRoot, &createdAt, &updatedAt,
	)
	if lastModified.Valid {
		file.LastModified = lastModified.Time
	}
	if createdAt.Valid {
		file.CreatedAt = createdAt.Time
	}
	if updatedAt.Valid {
		file.UpdatedAt = updatedAt.Time
	}

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	if file.IsDirectory {
		file.Type = "directory"
	} else {
		file.Type = "file"
	}

	return &file, nil
}

func (s *CatalogService) SearchFiles(req *models.SearchRequest) ([]models.FileInfo, int64, error) {
	baseQuery := `
		SELECT f.id, f.name, f.path, f.is_directory, f.size, f.modified_at, f.quick_hash, f.extension, f.mime_type, f.parent_id, sr.name as smb_root, f.created_at, f.last_scan_at
		FROM files f
		JOIN storage_roots sr ON f.storage_root_id = sr.id
		WHERE 1=1
	`

	countQuery := `
		SELECT COUNT(*)
		FROM files f
		JOIN storage_roots sr ON f.storage_root_id = sr.id
		WHERE 1=1
	`

	var conditions []string
	var args []interface{}

	// Add search conditions
	if req.Query != "" {
		conditions = append(conditions, "f.name LIKE ?")
		args = append(args, "%"+req.Query+"%")
	}

	if req.Path != "" {
		conditions = append(conditions, "f.path LIKE ?")
		args = append(args, req.Path+"%")
	}

	if req.Extension != "" {
		conditions = append(conditions, "f.extension = ?")
		args = append(args, req.Extension)
	}

	if req.MimeType != "" {
		conditions = append(conditions, "f.mime_type = ?")
		args = append(args, req.MimeType)
	}

	if req.MinSize != nil {
		conditions = append(conditions, "f.size >= ?")
		args = append(args, *req.MinSize)
	}

	if req.MaxSize != nil {
		conditions = append(conditions, "f.size <= ?")
		args = append(args, *req.MaxSize)
	}

	if len(req.SmbRoots) > 0 {
		placeholders := strings.Repeat("?,", len(req.SmbRoots))
		placeholders = placeholders[:len(placeholders)-1]
		conditions = append(conditions, fmt.Sprintf("sr.name IN (%s)", placeholders))
		for _, root := range req.SmbRoots {
			args = append(args, root)
		}
	}

	if req.IsDirectory != nil {
		conditions = append(conditions, "f.is_directory = ?")
		args = append(args, *req.IsDirectory)
	}

	// Build final queries
	whereClause := ""
	if len(conditions) > 0 {
		whereClause = " AND " + strings.Join(conditions, " AND ")
	}

	// Get total count
	var total int64
	err := s.db.QueryRow(countQuery+whereClause, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count results: %w", err)
	}

	// Add sorting and pagination to main query
	query := baseQuery + whereClause

	// Add sorting
	switch req.SortBy {
	case "name":
		query += " ORDER BY f.name"
	case "size":
		query += " ORDER BY f.size"
	case "modified":
		query += " ORDER BY f.modified_at"
	default:
		query += " ORDER BY f.is_directory DESC, f.name"
	}

	if req.SortOrder == "desc" {
		query += " DESC"
	} else {
		query += " ASC"
	}

	// Add pagination
	if req.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, req.Limit)
	}
	if req.Offset > 0 {
		query += " OFFSET ?"
		args = append(args, req.Offset)
	}

	// Execute main query
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to search files: %w", err)
	}
	defer rows.Close()

	var files []models.FileInfo
	for rows.Next() {
		var file models.FileInfo
		var lastModified sql.NullTime
		var createdAt sql.NullTime
		var updatedAt sql.NullTime
		err := rows.Scan(
			&file.ID, &file.Name, &file.Path, &file.IsDirectory, &file.Size,
			&lastModified, &file.Hash, &file.Extension, &file.MimeType,
			&file.ParentID, &file.SmbRoot, &createdAt, &updatedAt,
		)
		if lastModified.Valid {
			file.LastModified = lastModified.Time
		}
		if createdAt.Valid {
			file.CreatedAt = createdAt.Time
		}
		if updatedAt.Valid {
			file.UpdatedAt = updatedAt.Time
		}
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan file: %w", err)
		}
		if file.IsDirectory {
			file.Type = "directory"
		} else {
			file.Type = "file"
		}
		files = append(files, file)
	}

	return files, total, nil
}

func (s *CatalogService) GetDirectoriesBySize(smbRoot string, limit int) ([]models.DirectoryStats, error) {
	query := `
		WITH RECURSIVE dir_sizes AS (
			SELECT
				f.id, f.path, f.name, f.is_directory,
				CASE WHEN f.is_directory THEN 0 ELSE f.size END as file_size,
				CASE WHEN f.is_directory THEN 0 ELSE 1 END as file_count
			FROM files f
			WHERE f.storage_root_id = (SELECT id FROM storage_roots WHERE name = ? LIMIT 1) AND f.is_directory = 1

			UNION ALL

			SELECT
				f2.id, f2.path, f2.name, f2.is_directory,
				CASE WHEN f2.is_directory THEN 0 ELSE f2.size END,
				CASE WHEN f2.is_directory THEN 0 ELSE 1 END
			FROM files f2
			JOIN dir_sizes ds ON f2.parent_id = ds.id
		)
		SELECT
			path,
			SUM(file_size) as total_size,
			SUM(file_count) as file_count,
			COUNT(CASE WHEN is_directory THEN 1 END) as directory_count
		FROM dir_sizes
		WHERE is_directory = 1
		GROUP BY path
		ORDER BY total_size DESC
		LIMIT ?
	`

	rows, err := s.db.Query(query, smbRoot, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get directories by size: %w", err)
	}
	defer rows.Close()

	var stats []models.DirectoryStats
	for rows.Next() {
		var stat models.DirectoryStats
		err := rows.Scan(&stat.Path, &stat.TotalSize, &stat.FileCount, &stat.DirectoryCount)
		if err != nil {
			return nil, fmt.Errorf("failed to scan directory stats: %w", err)
		}
		stats = append(stats, stat)
	}

	return stats, nil
}

func (s *CatalogService) GetDuplicateGroups(smbRoot string, minCount int, limit int) ([]models.DuplicateGroup, error) {
	query := `
		SELECT
			f.quick_hash, f.size, COUNT(*) as count
		FROM files f
		WHERE f.quick_hash IS NOT NULL
			AND f.is_directory = 0
	`
	args := []interface{}{}

	if smbRoot != "" {
		query += " AND f.storage_root_id = (SELECT id FROM storage_roots WHERE name = ? LIMIT 1)"
		args = append(args, smbRoot)
	}

	query += `
		GROUP BY f.quick_hash, f.size
		HAVING COUNT(*) >= ?
		ORDER BY COUNT(*) DESC, f.size DESC
	`
	args = append(args, minCount)

	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get duplicate groups: %w", err)
	}
	defer rows.Close()

	// Buffer the group headers and release the cursor (and its pooled connection) BEFORE the
	// per-group file queries: a query issued while this cursor is open needs a second pooled
	// connection, so concurrent calls exceeding the pool would all hold one and wait for another.
	var page []models.DuplicateGroup
	for rows.Next() {
		var group models.DuplicateGroup
		err := rows.Scan(&group.Hash, &group.Size, &group.Count)
		if err != nil {
			return nil, fmt.Errorf("failed to scan duplicate group: %w", err)
		}
		page = append(page, group)
	}
	rows.Close()

	var groups []models.DuplicateGroup
	for _, group := range page {
		// Get files in this duplicate group
		filesQuery := `
			SELECT f.id, f.name, f.path, f.is_directory, f.size, f.modified_at, f.quick_hash, f.extension, f.mime_type, f.parent_id, sr.name as smb_root, f.created_at, f.last_scan_at
			FROM files f
			JOIN storage_roots sr ON f.storage_root_id = sr.id
			WHERE f.quick_hash = ? AND f.size = ?
		`
		args2 := []interface{}{group.Hash, group.Size}

		if smbRoot != "" {
			filesQuery += " AND f.storage_root_id = (SELECT id FROM storage_roots WHERE name = ? LIMIT 1)"
			args2 = append(args2, smbRoot)
		}

		filesQuery += " ORDER BY f.path"

		fileRows, err := s.db.Query(filesQuery, args2...)
		if err != nil {
			s.logger.Error("Failed to get files for duplicate group", zap.Error(err))
			continue
		}

		for fileRows.Next() {
			var file models.FileInfo
			var lastModified sql.NullTime
			var createdAt sql.NullTime
			var updatedAt sql.NullTime
			err := fileRows.Scan(
				&file.ID, &file.Name, &file.Path, &file.IsDirectory, &file.Size,
				&lastModified, &file.Hash, &file.Extension, &file.MimeType,
				&file.ParentID, &file.SmbRoot, &createdAt, &updatedAt,
			)
			if lastModified.Valid {
				file.LastModified = lastModified.Time
			}
			if createdAt.Valid {
				file.CreatedAt = createdAt.Time
			}
			if updatedAt.Valid {
				file.UpdatedAt = updatedAt.Time
			}
			if err != nil {
				s.logger.Error("Failed to scan duplicate file", zap.Error(err))
				continue
			}
			group.Files = append(group.Files, file)
		}
		fileRows.Close()

		group.TotalSize = group.Size * int64(group.Count)
		groups = append(groups, group)
	}

	return groups, nil
}

func (s *CatalogService) GetSMBRoots() ([]string, error) {
	query := `SELECT DISTINCT sr.name as smb_root FROM files f JOIN storage_roots sr ON f.storage_root_id = sr.id ORDER BY sr.name`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get SMB roots: %w", err)
	}
	defer rows.Close()

	var roots []string
	for rows.Next() {
		var root string
		if err := rows.Scan(&root); err != nil {
			return nil, fmt.Errorf("failed to scan SMB root: %w", err)
		}
		roots = append(roots, root)
	}

	return roots, nil
}

// ListDirectory lists files in a directory (alias for ListPath)
func (s *CatalogService) ListDirectory(path string) ([]models.FileInfo, error) {
	return s.ListPath(path, "name", "asc", 0, 0)
}

// Search searches files by query (simplified version)
func (s *CatalogService) Search(query string, fileType string, limit int, offset int) ([]models.FileInfo, error) {
	isDirectory := false
	req := &models.SearchRequest{
		Query:       query,
		MimeType:    fileType,
		IsDirectory: &isDirectory,
		Limit:       limit,
		Offset:      offset,
	}
	files, _, err := s.SearchFiles(req)
	return files, err
}

// SearchDuplicates searches for duplicate files
func (s *CatalogService) SearchDuplicates() ([]models.DuplicateGroup, error) {
	return s.GetDuplicateGroups("", 2, 0)
}

// GetFileInfoByPath gets file info by path (for test compatibility)
func (s *CatalogService) GetFileInfoByPath(path string) (*models.FileInfo, error) {
	query := `
		SELECT f.id, f.name, f.path, f.is_directory, f.size, f.modified_at, f.quick_hash, f.extension, f.mime_type, f.parent_id, sr.name as smb_root, f.created_at, f.last_scan_at
		FROM files f
		JOIN storage_roots sr ON f.storage_root_id = sr.id
		WHERE f.path = ?
	`

	var file models.FileInfo
	var lastModified sql.NullTime
	var createdAt sql.NullTime
	var updatedAt sql.NullTime
	err := s.db.QueryRow(query, path).Scan(
		&file.ID, &file.Name, &file.Path, &file.IsDirectory, &file.Size,
		&lastModified, &file.Hash, &file.Extension, &file.MimeType,
		&file.ParentID, &file.SmbRoot, &createdAt, &updatedAt,
	)
	if lastModified.Valid {
		file.LastModified = lastModified.Time
	}
	if createdAt.Valid {
		file.CreatedAt = createdAt.Time
	}
	if updatedAt.Valid {
		file.UpdatedAt = updatedAt.Time
	}

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get file info by path: %w", err)
	}

	if file.IsDirectory {
		file.Type = "directory"
	} else {
		file.Type = "file"
	}

	return &file, nil
}

// GetDuplicatesCount gets the count of duplicate files
func (s *CatalogService) GetDuplicatesCount() (int64, error) {
	query := `
		SELECT COUNT(*) FROM (
			SELECT quick_hash, COUNT(*) as count
			FROM files
			WHERE quick_hash IS NOT NULL AND quick_hash != '' AND is_directory = 0
			GROUP BY quick_hash
			HAVING COUNT(*) > 1
		) AS dup_groups
	`

	var count int64
	err := s.db.QueryRow(query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get duplicates count: %w", err)
	}

	return count, nil
}

// GetDirectoriesBySizeLimited gets directories by size with default limit
func (s *CatalogService) GetDirectoriesBySizeLimited(limit int) ([]models.DirectoryStats, error) {
	return s.GetDirectoriesBySize("", limit)
}
