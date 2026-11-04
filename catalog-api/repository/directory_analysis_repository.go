package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"catalogizer/database"
	"catalogizer/internal/media/models"
)

// DirectoryAnalysisRepository handles directory_analyses table operations.
type DirectoryAnalysisRepository struct {
	db *database.DB
}

// NewDirectoryAnalysisRepository creates a new directory analysis repository.
func NewDirectoryAnalysisRepository(db *database.DB) *DirectoryAnalysisRepository {
	return &DirectoryAnalysisRepository{db: db}
}

// Create inserts a directory analysis and returns its ID.
func (r *DirectoryAnalysisRepository) Create(ctx context.Context, da *models.DirectoryAnalysis) (int64, error) {
	analysisDataJSON, _ := json.Marshal(da.AnalysisData)

	query := `INSERT INTO directory_analyses (
		directory_path, smb_root, media_item_id, confidence_score,
		detection_method, analysis_data, last_analyzed, files_count, total_size
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`

	now := time.Now()
	id, err := r.db.InsertReturningID(ctx, query,
		da.DirectoryPath, da.SmbRoot, da.MediaItemID, da.ConfidenceScore,
		da.DetectionMethod, string(analysisDataJSON), now, da.FilesCount, da.TotalSize,
	)
	if err != nil {
		return 0, fmt.Errorf("insert directory analysis: %w", err)
	}
	da.ID = id
	da.LastAnalyzed = now
	return id, nil
}

// GetByPath returns the analysis for a directory path.
func (r *DirectoryAnalysisRepository) GetByPath(ctx context.Context, path string) (*models.DirectoryAnalysis, error) {
	query := `SELECT id, directory_path, smb_root, media_item_id, confidence_score,
		detection_method, analysis_data, last_analyzed, files_count, total_size
	FROM directory_analyses WHERE directory_path = ? LIMIT 1`

	da, err := r.scanAnalysis(r.db.QueryRowContext(ctx, query, path))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get by path: %w", err)
	}
	return da, nil
}

// Update updates a directory analysis.
func (r *DirectoryAnalysisRepository) Update(ctx context.Context, da *models.DirectoryAnalysis) error {
	analysisDataJSON, _ := json.Marshal(da.AnalysisData)

	query := `UPDATE directory_analyses SET
		smb_root = ?, media_item_id = ?, confidence_score = ?,
		detection_method = ?, analysis_data = ?, last_analyzed = ?,
		files_count = ?, total_size = ?
	WHERE id = ?`

	now := time.Now()
	_, err := r.db.ExecContext(ctx, query,
		da.SmbRoot, da.MediaItemID, da.ConfidenceScore,
		da.DetectionMethod, string(analysisDataJSON), now,
		da.FilesCount, da.TotalSize, da.ID,
	)
	if err != nil {
		return fmt.Errorf("update directory analysis: %w", err)
	}
	da.LastAnalyzed = now
	return nil
}

// GetUnprocessed returns directory analyses that haven't been linked to a media item.
func (r *DirectoryAnalysisRepository) GetUnprocessed(ctx context.Context, limit int) ([]*models.DirectoryAnalysis, error) {
	query := `SELECT id, directory_path, smb_root, media_item_id, confidence_score,
		detection_method, analysis_data, last_analyzed, files_count, total_size
	FROM directory_analyses WHERE media_item_id IS NULL
	ORDER BY confidence_score DESC LIMIT ?`

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("get unprocessed: %w", err)
	}
	defer rows.Close()

	items := []*models.DirectoryAnalysis{}
	for rows.Next() {
		da, err := r.scanAnalysisFromRows(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, da)
	}
	return items, rows.Err()
}

// --- internal helpers ---

func (r *DirectoryAnalysisRepository) scanAnalysis(row *sql.Row) (*models.DirectoryAnalysis, error) {
	da := &models.DirectoryAnalysis{}
	var analysisDataJSON sql.NullString
	err := row.Scan(
		&da.ID, &da.DirectoryPath, &da.SmbRoot, &da.MediaItemID, &da.ConfidenceScore,
		&da.DetectionMethod, &analysisDataJSON, &da.LastAnalyzed, &da.FilesCount, &da.TotalSize,
	)
	if err != nil {
		return nil, err
	}
	if analysisDataJSON.Valid {
		da.AnalysisData = &models.AnalysisData{}
		json.Unmarshal([]byte(analysisDataJSON.String), da.AnalysisData)
	}
	return da, nil
}

func (r *DirectoryAnalysisRepository) scanAnalysisFromRows(rows *sql.Rows) (*models.DirectoryAnalysis, error) {
	da := &models.DirectoryAnalysis{}
	var analysisDataJSON sql.NullString
	err := rows.Scan(
		&da.ID, &da.DirectoryPath, &da.SmbRoot, &da.MediaItemID, &da.ConfidenceScore,
		&da.DetectionMethod, &analysisDataJSON, &da.LastAnalyzed, &da.FilesCount, &da.TotalSize,
	)
	if err != nil {
		return nil, err
	}
	if analysisDataJSON.Valid {
		da.AnalysisData = &models.AnalysisData{}
		json.Unmarshal([]byte(analysisDataJSON.String), da.AnalysisData)
	}
	return da, nil
}
