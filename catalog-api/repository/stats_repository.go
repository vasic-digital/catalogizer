package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"catalogizer/database"
	"catalogizer/internal/models"
)

// StatsRepository handles statistics-related database operations
type StatsRepository struct {
	db *database.DB
}

// NewStatsRepository creates a new stats repository
func NewStatsRepository(db *database.DB) *StatsRepository {
	return &StatsRepository{db: db}
}

// GetOverallStats retrieves overall catalog statistics
func (r *StatsRepository) GetOverallStats(ctx context.Context) (*models.OverallStats, error) {
	// Dialect-aware timestamp extraction
	lastScanExpr := "COALESCE(CAST(strftime('%s', MAX(last_scan_at)) AS INTEGER), 0)"
	enabledExpr := "enabled = 1"
	deletedExpr := "deleted = 0"
	isDirFalse := "is_directory = 0"
	isDirTrue := "is_directory = 1"
	isDupTrue := "is_duplicate = 1"
	if r.db.Dialect().IsPostgres() {
		lastScanExpr = "COALESCE(EXTRACT(EPOCH FROM MAX(last_scan_at))::BIGINT, 0)"
		enabledExpr = "enabled = true"
		deletedExpr = "deleted = false"
		isDirFalse = "is_directory = false"
		isDirTrue = "is_directory = true"
		isDupTrue = "is_duplicate = true"
	}

	query := fmt.Sprintf(`
		SELECT
			COUNT(CASE WHEN %s AND %s THEN 1 END) as total_files,
			COUNT(CASE WHEN %s AND %s THEN 1 END) as total_directories,
			COALESCE(SUM(CASE WHEN %s AND %s THEN size ELSE 0 END), 0) as total_size,
			COUNT(CASE WHEN %s AND %s THEN 1 END) as total_duplicates,
			COUNT(DISTINCT duplicate_group_id) as duplicate_groups,
			(SELECT COUNT(*) FROM storage_roots) as storage_roots_count,
			(SELECT COUNT(*) FROM storage_roots WHERE %s) as active_storage_roots,
			%s as last_scan_time
		FROM files`,
		isDirFalse, deletedExpr,
		isDirTrue, deletedExpr,
		isDirFalse, deletedExpr,
		isDupTrue, deletedExpr,
		enabledExpr,
		lastScanExpr)

	var stats models.OverallStats
	err := r.db.QueryRowContext(ctx, query).Scan(
		&stats.TotalFiles,
		&stats.TotalDirectories,
		&stats.TotalSize,
		&stats.TotalDuplicates,
		&stats.DuplicateGroups,
		&stats.StorageRootsCount,
		&stats.ActiveStorageRoots,
		&stats.LastScanTime,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get overall stats: %w", err)
	}

	return &stats, nil
}

// GetStorageRootStats retrieves statistics for a specific storage root
func (r *StatsRepository) GetStorageRootStats(ctx context.Context, storageRootName string) (*models.StorageRootStats, error) {
	lastScanExpr := "COALESCE(CAST(strftime('%s', MAX(f.last_scan_at)) AS INTEGER), 0)"
	if r.db.Dialect().IsPostgres() {
		lastScanExpr = "COALESCE(EXTRACT(EPOCH FROM MAX(f.last_scan_at))::BIGINT, 0)"
	}

	query := fmt.Sprintf(`
		SELECT
			sr.name,
			COUNT(CASE WHEN f.is_directory = 0 AND f.deleted = 0 THEN 1 END) as total_files,
			COUNT(CASE WHEN f.is_directory = 1 AND f.deleted = 0 THEN 1 END) as total_directories,
			COALESCE(SUM(CASE WHEN f.is_directory = 0 AND f.deleted = 0 THEN f.size ELSE 0 END), 0) as total_size,
			COUNT(CASE WHEN f.is_duplicate = 1 AND f.deleted = 0 THEN 1 END) as duplicate_files,
			COUNT(DISTINCT f.duplicate_group_id) as duplicate_groups,
			%s as last_scan_time,
			sr.enabled as is_online
		FROM storage_roots sr
		LEFT JOIN files f ON sr.id = f.storage_root_id
		WHERE sr.name = ?
		GROUP BY sr.id, sr.name, sr.enabled`, lastScanExpr)

	var stats models.StorageRootStats
	err := r.db.QueryRowContext(ctx, query, storageRootName).Scan(
		&stats.Name,
		&stats.TotalFiles,
		&stats.TotalDirectories,
		&stats.TotalSize,
		&stats.DuplicateFiles,
		&stats.DuplicateGroups,
		&stats.LastScanTime,
		&stats.IsOnline,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("storage root not found")
		}
		return nil, fmt.Errorf("failed to get storage root stats: %w", err)
	}

	return &stats, nil
}

// GetFileTypeStats retrieves file type statistics
func (r *StatsRepository) GetFileTypeStats(ctx context.Context, storageRootName string, limit int) ([]models.FileTypeStats, error) {
	baseQuery := `
		SELECT
			COALESCE(file_type, 'unknown') as file_type,
			COALESCE(extension, 'none') as extension,
			COUNT(*) as count,
			SUM(size) as total_size,
			AVG(size) as average_size
		FROM files f`

	args := []interface{}{}
	whereClause := " WHERE f.is_directory = 0 AND f.deleted = 0"

	if storageRootName != "" {
		whereClause += " AND f.smb_root_id = (SELECT id FROM smb_roots WHERE name = ?)"
		args = append(args, storageRootName)
	}

	query := baseQuery + whereClause + `
		GROUP BY file_type, extension
		ORDER BY count DESC
		LIMIT ?`

	args = append(args, limit)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get file type stats: %w", err)
	}
	defer rows.Close()

	stats := []models.FileTypeStats{}
	for rows.Next() {
		var stat models.FileTypeStats
		err := rows.Scan(
			&stat.FileType,
			&stat.Extension,
			&stat.Count,
			&stat.TotalSize,
			&stat.AverageSize,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan file type stats: %w", err)
		}
		stats = append(stats, stat)
	}

	return stats, nil
}

// GetSizeDistribution retrieves file size distribution
func (r *StatsRepository) GetSizeDistribution(ctx context.Context, storageRootName string) (*models.SizeDistribution, error) {
	baseQuery := `
		SELECT
			COUNT(CASE WHEN size = 0 THEN 1 END) as empty,
			COUNT(CASE WHEN size > 0 AND size < 1024 THEN 1 END) as tiny,
			COUNT(CASE WHEN size >= 1024 AND size < 1048576 THEN 1 END) as small,
			COUNT(CASE WHEN size >= 1048576 AND size < 10485760 THEN 1 END) as medium,
			COUNT(CASE WHEN size >= 10485760 AND size < 104857600 THEN 1 END) as large,
			COUNT(CASE WHEN size >= 104857600 AND size < 1073741824 THEN 1 END) as huge,
			COUNT(CASE WHEN size >= 1073741824 THEN 1 END) as massive
		FROM files f`

	args := []interface{}{}
	whereClause := " WHERE f.is_directory = 0 AND f.deleted = 0"

	if storageRootName != "" {
		whereClause += " AND f.smb_root_id = (SELECT id FROM smb_roots WHERE name = ?)"
		args = append(args, storageRootName)
	}

	query := baseQuery + whereClause

	var distribution models.SizeDistribution
	var empty int64
	err := r.db.QueryRowContext(ctx, query, args...).Scan(
		&empty,
		&distribution.Tiny,
		&distribution.Small,
		&distribution.Medium,
		&distribution.Large,
		&distribution.Huge,
		&distribution.Massive,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get size distribution: %w", err)
	}

	// Add empty files to tiny category
	distribution.Tiny += empty

	return &distribution, nil
}

// GetDuplicateStats retrieves duplicate file statistics
func (r *StatsRepository) GetDuplicateStats(ctx context.Context, storageRootName string) (*models.DuplicateStats, error) {
	baseQuery := `
		WITH duplicate_analysis AS (
			SELECT
				duplicate_group_id,
				COUNT(*) as group_size,
				MAX(size) as file_size
			FROM files f
			WHERE f.is_duplicate = 1 AND f.deleted = 0`

	args := []interface{}{}
	if storageRootName != "" {
		args = append(args, storageRootName)
	}

	baseQuery += `
			GROUP BY duplicate_group_id
		)
		SELECT
			(SELECT COUNT(*) FROM files f WHERE f.is_duplicate = 1 AND f.deleted = 0` +
		(func() string {
			if storageRootName != "" {
				return " AND f.storage_root_id = (SELECT id FROM storage_roots WHERE name = ?)"
			}
			return ""
		})() + `) as total_duplicates,
			COUNT(*) as duplicate_groups,
			COALESCE(SUM((group_size - 1) * file_size), 0) as wasted_space,
			COALESCE(MAX(group_size), 0) as largest_group,
			COALESCE(AVG(group_size), 0) as average_group_size
		FROM duplicate_analysis`

	if storageRootName != "" {
		args = append(args, storageRootName) // For the subquery
	}

	var stats models.DuplicateStats
	err := r.db.QueryRowContext(ctx, baseQuery, args...).Scan(
		&stats.TotalDuplicates,
		&stats.DuplicateGroups,
		&stats.WastedSpace,
		&stats.LargestDuplicateGroup,
		&stats.AverageGroupSize,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get duplicate stats: %w", err)
	}

	return &stats, nil
}

// GetTopDuplicateGroups retrieves the top duplicate groups
func (r *StatsRepository) GetTopDuplicateGroups(ctx context.Context, sortBy string, limit int, storageRootName string) ([]models.DuplicateGroupStats, error) {
	baseQuery := `
		SELECT
			dg.id as group_id,
			dg.file_count,
			dg.total_size,
			(dg.file_count - 1) * (dg.total_size / dg.file_count) as wasted_space,
			(SELECT f.path FROM files f WHERE f.duplicate_group_id = dg.id AND f.deleted = 0 LIMIT 1) as sample_path
		FROM duplicate_groups dg`

	args := []interface{}{}
	whereClause := ""

	if storageRootName != "" {
		whereClause = ` WHERE EXISTS (
			SELECT 1 FROM files f
			WHERE f.duplicate_group_id = dg.id
			AND f.storage_root_id = (SELECT id FROM storage_roots WHERE name = ?)
		)`
		args = append(args, storageRootName)
	}

	orderClause := " ORDER BY "
	if sortBy == "size" {
		orderClause += "dg.total_size DESC"
	} else {
		orderClause += "dg.file_count DESC"
	}

	query := baseQuery + whereClause + orderClause + " LIMIT ?"
	args = append(args, limit)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get top duplicate groups: %w", err)
	}
	defer rows.Close()

	groups := []models.DuplicateGroupStats{}
	for rows.Next() {
		var group models.DuplicateGroupStats
		err := rows.Scan(
			&group.GroupID,
			&group.FileCount,
			&group.TotalSize,
			&group.WastedSpace,
			&group.SamplePath,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan duplicate group: %w", err)
		}
		groups = append(groups, group)
	}

	return groups, nil
}

// GetAccessPatterns retrieves file access patterns
func (r *StatsRepository) GetAccessPatterns(ctx context.Context, storageRootName string, days int) (*models.AccessPatterns, error) {
	// This is a simplified implementation
	// In a real scenario, you'd need to track actual access times
	baseQuery := `
		SELECT
			COUNT(CASE WHEN accessed_at IS NOT NULL AND accessed_at > ? THEN 1 END) as recently_accessed,
			COUNT(CASE WHEN accessed_at IS NULL THEN 1 END) as never_accessed
		FROM files f`

	// PostgreSQL TIMESTAMP columns require time.Time values, not Unix epoch integers.
	// SQLite stores timestamps as integers and compares them numerically.
	cutoff := time.Now().AddDate(0, 0, -days)
	var args []interface{}
	if r.db.Dialect().IsPostgres() {
		args = []interface{}{cutoff}
	} else {
		args = []interface{}{cutoff.Unix()}
	}
	whereClause := " WHERE f.is_directory = 0 AND f.deleted = 0"

	if storageRootName != "" {
		whereClause += " AND f.storage_root_id = (SELECT id FROM storage_roots WHERE name = ?)"
		args = append(args, storageRootName)
	}

	query := baseQuery + whereClause

	var patterns models.AccessPatterns
	err := r.db.QueryRowContext(ctx, query, args...).Scan(
		&patterns.RecentlyAccessed,
		&patterns.NeverAccessed,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get access patterns: %w", err)
	}

	// Initialize arrays with default values
	patterns.AccessFrequency = make([]int64, days)
	patterns.PopularExtensions = []string{}
	patterns.PopularDirectories = []string{}

	return &patterns, nil
}

// GetGrowthTrends retrieves storage growth trends
func (r *StatsRepository) GetGrowthTrends(ctx context.Context, storageRootName string, months int) (*models.GrowthTrends, error) {
	// This is a simplified implementation
	// In a real scenario, you'd need historical data tracking
	monthExpr := "COALESCE(strftime('%Y-%m', datetime(created_at, 'unixepoch')), 'unknown')"
	if r.db.Dialect().IsPostgres() {
		monthExpr = "COALESCE(to_char(created_at, 'YYYY-MM'), 'unknown')"
	}

	baseQuery := fmt.Sprintf(`
		SELECT
			%s as month,
			COUNT(*) as files_added,
			COALESCE(SUM(size), 0) as size_added
		FROM files f`, monthExpr)

	// PostgreSQL TIMESTAMP columns require time.Time values, not Unix epoch integers.
	// SQLite stores timestamps as integers and compares them numerically.
	cutoff := time.Now().AddDate(0, -months, 0)
	var cutoffArg interface{}
	if r.db.Dialect().IsPostgres() {
		cutoffArg = cutoff
	} else {
		cutoffArg = cutoff.Unix()
	}

	args := []interface{}{}
	createdAtPositive := "created_at > 0"
	if r.db.Dialect().IsPostgres() {
		createdAtPositive = "created_at > '1970-01-01'"
	}
	whereClause := fmt.Sprintf(" WHERE f.is_directory = 0 AND f.deleted = 0 AND created_at IS NOT NULL AND %s AND created_at > ?", createdAtPositive)

	args = append(args, cutoffArg)

	if storageRootName != "" {
		whereClause += " AND f.storage_root_id = (SELECT id FROM storage_roots WHERE name = ?)"
		args = append(args, storageRootName)
	}

	query := baseQuery + whereClause + " GROUP BY month ORDER BY month"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get growth trends: %w", err)
	}
	defer rows.Close()

	var trends models.GrowthTrends
	monthlyGrowth := []models.MonthlyGrowth{}

	for rows.Next() {
		var growth models.MonthlyGrowth
		err := rows.Scan(
			&growth.Month,
			&growth.FilesAdded,
			&growth.SizeAdded,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan growth trend: %w", err)
		}
		monthlyGrowth = append(monthlyGrowth, growth)
	}

	trends.MonthlyGrowth = monthlyGrowth
	trends.TotalGrowthRate = 0.0 // Calculate based on data
	trends.FileGrowthRate = 0.0  // Calculate based on data
	trends.SizeGrowthRate = 0.0  // Calculate based on data

	return &trends, nil
}

// GetScanHistory retrieves scan operation history
func (r *StatsRepository) GetScanHistory(ctx context.Context, storageRootName string, limit, offset int) ([]models.ScanHistoryItem, int64, error) {
	countQuery := `
		SELECT COUNT(*)
		FROM scan_history sh
		JOIN storage_roots sr ON sh.storage_root_id = sr.id`

	selectQuery := `
		SELECT
			sh.id,
			sr.name as storage_root_name,
			sh.scan_type,
			sh.status,
			sh.start_time,
			sh.end_time,
			sh.files_processed,
			sh.files_added,
			sh.files_updated,
			sh.files_deleted,
			sh.error_count,
			sh.error_message
		FROM scan_history sh
		JOIN storage_roots sr ON sh.storage_root_id = sr.id`

	args := []interface{}{}
	whereClause := ""

	if storageRootName != "" {
		whereClause += " AND f.storage_root_id = (SELECT id FROM storage_roots WHERE name = ?)"
		args = append(args, storageRootName)
	}

	// Get total count
	var totalCount int64
	err := r.db.QueryRowContext(ctx, countQuery+whereClause, args...).Scan(&totalCount)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count scan history: %w", err)
	}

	// Get paginated results
	query := selectQuery + whereClause + " ORDER BY sh.start_time DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get scan history: %w", err)
	}
	defer rows.Close()

	history := []models.ScanHistoryItem{}
	for rows.Next() {
		var item models.ScanHistoryItem
		err := rows.Scan(
			&item.ID,
			&item.SmbRootName,
			&item.ScanType,
			&item.Status,
			&item.StartTime,
			&item.EndTime,
			&item.FilesProcessed,
			&item.FilesAdded,
			&item.FilesUpdated,
			&item.FilesDeleted,
			&item.ErrorCount,
			&item.ErrorMessage,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan history item: %w", err)
		}
		history = append(history, item)
	}

	return history, totalCount, nil
}
