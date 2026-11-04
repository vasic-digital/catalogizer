package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"catalogizer/database"
	"catalogizer/models"
)

type LogManagementRepository struct {
	db *database.DB
}

func NewLogManagementRepository(db *database.DB) *LogManagementRepository {
	return &LogManagementRepository{db: db}
}

func (r *LogManagementRepository) CreateLogCollection(collection *models.LogCollection) error {
	componentsJSON, err := json.Marshal(collection.Components)
	if err != nil {
		return fmt.Errorf("failed to marshal components: %w", err)
	}

	filtersJSON, err := json.Marshal(collection.Filters)
	if err != nil {
		return fmt.Errorf("failed to marshal filters: %w", err)
	}

	query := `
		INSERT INTO log_collections (
			user_id, name, description, components, log_level, start_time,
			end_time, created_at, status, filters
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	id, err := r.db.InsertReturningID(context.Background(), query,
		collection.UserID, collection.Name, collection.Description,
		string(componentsJSON), collection.LogLevel, collection.StartTime,
		collection.EndTime, collection.CreatedAt, collection.Status,
		string(filtersJSON))

	if err != nil {
		return fmt.Errorf("failed to create log collection: %w", err)
	}

	collection.ID = int(id)
	return nil
}

func (r *LogManagementRepository) GetLogCollection(id int) (*models.LogCollection, error) {
	query := `
		SELECT id, user_id, name, description, components, log_level, start_time,
			   end_time, created_at, completed_at, status, entry_count, filters
		FROM log_collections WHERE id = ?`

	var collection models.LogCollection
	var componentsJSON, filtersJSON string
	var startTime, endTime, completedAt sql.NullTime

	err := r.db.QueryRow(query, id).Scan(
		&collection.ID, &collection.UserID, &collection.Name,
		&collection.Description, &componentsJSON, &collection.LogLevel,
		&startTime, &endTime, &collection.CreatedAt, &completedAt,
		&collection.Status, &collection.EntryCount, &filtersJSON)

	if err != nil {
		return nil, fmt.Errorf("failed to get log collection: %w", err)
	}

	if startTime.Valid {
		collection.StartTime = &startTime.Time
	}
	if endTime.Valid {
		collection.EndTime = &endTime.Time
	}
	if completedAt.Valid {
		collection.CompletedAt = &completedAt.Time
	}

	if err := json.Unmarshal([]byte(componentsJSON), &collection.Components); err != nil {
		return nil, fmt.Errorf("failed to unmarshal components: %w", err)
	}

	if err := json.Unmarshal([]byte(filtersJSON), &collection.Filters); err != nil {
		return nil, fmt.Errorf("failed to unmarshal filters: %w", err)
	}

	return &collection, nil
}

func (r *LogManagementRepository) UpdateLogCollection(collection *models.LogCollection) error {
	componentsJSON, err := json.Marshal(collection.Components)
	if err != nil {
		return fmt.Errorf("failed to marshal components: %w", err)
	}

	filtersJSON, err := json.Marshal(collection.Filters)
	if err != nil {
		return fmt.Errorf("failed to marshal filters: %w", err)
	}

	query := `
		UPDATE log_collections SET
			name = ?, description = ?, components = ?, log_level = ?,
			start_time = ?, end_time = ?, completed_at = ?, status = ?,
			entry_count = ?, filters = ?
		WHERE id = ?`

	_, err = r.db.Exec(query,
		collection.Name, collection.Description, string(componentsJSON),
		collection.LogLevel, collection.StartTime, collection.EndTime,
		collection.CompletedAt, collection.Status, collection.EntryCount,
		string(filtersJSON), collection.ID)

	if err != nil {
		return fmt.Errorf("failed to update log collection: %w", err)
	}

	return nil
}

func (r *LogManagementRepository) GetLogCollectionsByUser(userID int, limit, offset int) ([]*models.LogCollection, error) {
	query := `
		SELECT id, user_id, name, description, components, log_level, start_time,
			   end_time, created_at, completed_at, status, entry_count, filters
		FROM log_collections
		WHERE user_id = ?
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?`

	rows, err := r.db.Query(query, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get log collections: %w", err)
	}
	defer rows.Close()

	collections := []*models.LogCollection{}
	for rows.Next() {
		var collection models.LogCollection
		var componentsJSON, filtersJSON string
		var startTime, endTime, completedAt sql.NullTime

		err := rows.Scan(
			&collection.ID, &collection.UserID, &collection.Name,
			&collection.Description, &componentsJSON, &collection.LogLevel,
			&startTime, &endTime, &collection.CreatedAt, &completedAt,
			&collection.Status, &collection.EntryCount, &filtersJSON)

		if err != nil {
			return nil, fmt.Errorf("failed to scan log collection: %w", err)
		}

		if startTime.Valid {
			collection.StartTime = &startTime.Time
		}
		if endTime.Valid {
			collection.EndTime = &endTime.Time
		}
		if completedAt.Valid {
			collection.CompletedAt = &completedAt.Time
		}

		if err := json.Unmarshal([]byte(componentsJSON), &collection.Components); err != nil {
			return nil, fmt.Errorf("failed to unmarshal components: %w", err)
		}

		if err := json.Unmarshal([]byte(filtersJSON), &collection.Filters); err != nil {
			return nil, fmt.Errorf("failed to unmarshal filters: %w", err)
		}

		collections = append(collections, &collection)
	}

	return collections, nil
}

func (r *LogManagementRepository) DeleteLogCollection(id int) error {
	// First delete associated log entries
	if err := r.DeleteLogEntriesByCollection(id); err != nil {
		return fmt.Errorf("failed to delete log entries: %w", err)
	}

	query := "DELETE FROM log_collections WHERE id = ?"
	_, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete log collection: %w", err)
	}

	return nil
}

func (r *LogManagementRepository) CreateLogEntry(entry *models.LogEntry) error {
	contextJSON, err := json.Marshal(entry.Context)
	if err != nil {
		return fmt.Errorf("failed to marshal context: %w", err)
	}

	query := `
		INSERT INTO log_entries (
			collection_id, timestamp, level, component, message, context
		) VALUES (?, ?, ?, ?, ?, ?)`

	id, err := r.db.InsertReturningID(context.Background(), query,
		entry.CollectionID, entry.Timestamp, entry.Level,
		entry.Component, entry.Message, string(contextJSON))

	if err != nil {
		return fmt.Errorf("failed to create log entry: %w", err)
	}

	entry.ID = int(id)
	return nil
}

func (r *LogManagementRepository) GetLogEntries(collectionID int, filters *models.LogEntryFilters) ([]*models.LogEntry, error) {
	query := `
		SELECT id, collection_id, timestamp, level, component, message, context
		FROM log_entries
		WHERE collection_id = ?`

	args := []interface{}{collectionID}

	if filters != nil {
		if filters.Level != "" {
			query += " AND level = ?"
			args = append(args, filters.Level)
		}

		if filters.Component != "" {
			query += " AND component = ?"
			args = append(args, filters.Component)
		}

		if filters.StartTime != nil {
			query += " AND timestamp >= ?"
			args = append(args, *filters.StartTime)
		}

		if filters.EndTime != nil {
			query += " AND timestamp <= ?"
			args = append(args, *filters.EndTime)
		}

		if filters.Search != "" {
			query += " AND message LIKE ?"
			args = append(args, "%"+filters.Search+"%")
		}
	}

	query += " ORDER BY timestamp ASC"

	if filters != nil && filters.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, filters.Limit)

		if filters.Offset > 0 {
			query += " OFFSET ?"
			args = append(args, filters.Offset)
		}
	}

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get log entries: %w", err)
	}
	defer rows.Close()

	entries := []*models.LogEntry{}
	for rows.Next() {
		var entry models.LogEntry
		var contextJSON string

		err := rows.Scan(
			&entry.ID, &entry.CollectionID, &entry.Timestamp,
			&entry.Level, &entry.Component, &entry.Message, &contextJSON)

		if err != nil {
			return nil, fmt.Errorf("failed to scan log entry: %w", err)
		}

		if err := json.Unmarshal([]byte(contextJSON), &entry.Context); err != nil {
			return nil, fmt.Errorf("failed to unmarshal context: %w", err)
		}

		entries = append(entries, &entry)
	}

	return entries, nil
}

func (r *LogManagementRepository) DeleteLogEntriesByCollection(collectionID int) error {
	query := "DELETE FROM log_entries WHERE collection_id = ?"
	_, err := r.db.Exec(query, collectionID)
	if err != nil {
		return fmt.Errorf("failed to delete log entries: %w", err)
	}
	return nil
}

func (r *LogManagementRepository) GetRecentLogEntries(component string, limit int) ([]*models.LogEntry, error) {
	query := `
		SELECT id, collection_id, timestamp, level, component, message, context
		FROM log_entries
		WHERE component = ?
		ORDER BY timestamp DESC
		LIMIT ?`

	rows, err := r.db.Query(query, component, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent log entries: %w", err)
	}
	defer rows.Close()

	entries := []*models.LogEntry{}
	for rows.Next() {
		var entry models.LogEntry
		var contextJSON string

		err := rows.Scan(
			&entry.ID, &entry.CollectionID, &entry.Timestamp,
			&entry.Level, &entry.Component, &entry.Message, &contextJSON)

		if err != nil {
			return nil, fmt.Errorf("failed to scan log entry: %w", err)
		}

		if err := json.Unmarshal([]byte(contextJSON), &entry.Context); err != nil {
			return nil, fmt.Errorf("failed to unmarshal context: %w", err)
		}

		entries = append(entries, &entry)
	}

	return entries, nil
}

func (r *LogManagementRepository) CreateLogShare(share *models.LogShare) error {
	permissionsJSON, err := json.Marshal(share.Permissions)
	if err != nil {
		return fmt.Errorf("failed to marshal permissions: %w", err)
	}

	recipientsJSON, err := json.Marshal(share.Recipients)
	if err != nil {
		return fmt.Errorf("failed to marshal recipients: %w", err)
	}

	query := `
		INSERT INTO log_shares (
			collection_id, user_id, share_token, share_type, expires_at,
			created_at, is_active, permissions, recipients
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`

	id, err := r.db.InsertReturningID(context.Background(), query,
		share.CollectionID, share.UserID, share.ShareToken, share.ShareType,
		share.ExpiresAt, share.CreatedAt, share.IsActive,
		string(permissionsJSON), string(recipientsJSON))

	if err != nil {
		return fmt.Errorf("failed to create log share: %w", err)
	}

	share.ID = int(id)
	return nil
}

func (r *LogManagementRepository) GetLogShare(id int) (*models.LogShare, error) {
	query := `
		SELECT id, collection_id, user_id, share_token, share_type, expires_at,
			   created_at, accessed_at, is_active, permissions, recipients
		FROM log_shares WHERE id = ?`

	var share models.LogShare
	var permissionsJSON, recipientsJSON string
	var accessedAt sql.NullTime

	err := r.db.QueryRow(query, id).Scan(
		&share.ID, &share.CollectionID, &share.UserID, &share.ShareToken,
		&share.ShareType, &share.ExpiresAt, &share.CreatedAt, &accessedAt,
		&share.IsActive, &permissionsJSON, &recipientsJSON)

	if err != nil {
		return nil, fmt.Errorf("failed to get log share: %w", err)
	}

	if accessedAt.Valid {
		share.AccessedAt = &accessedAt.Time
	}

	if err := json.Unmarshal([]byte(permissionsJSON), &share.Permissions); err != nil {
		return nil, fmt.Errorf("failed to unmarshal permissions: %w", err)
	}

	if err := json.Unmarshal([]byte(recipientsJSON), &share.Recipients); err != nil {
		return nil, fmt.Errorf("failed to unmarshal recipients: %w", err)
	}

	return &share, nil
}

func (r *LogManagementRepository) GetLogShareByToken(token string) (*models.LogShare, error) {
	query := `
		SELECT id, collection_id, user_id, share_token, share_type, expires_at,
			   created_at, accessed_at, is_active, permissions, recipients
		FROM log_shares WHERE share_token = ?`

	var share models.LogShare
	var permissionsJSON, recipientsJSON string
	var accessedAt sql.NullTime

	err := r.db.QueryRow(query, token).Scan(
		&share.ID, &share.CollectionID, &share.UserID, &share.ShareToken,
		&share.ShareType, &share.ExpiresAt, &share.CreatedAt, &accessedAt,
		&share.IsActive, &permissionsJSON, &recipientsJSON)

	if err != nil {
		return nil, fmt.Errorf("failed to get log share by token: %w", err)
	}

	if accessedAt.Valid {
		share.AccessedAt = &accessedAt.Time
	}

	if err := json.Unmarshal([]byte(permissionsJSON), &share.Permissions); err != nil {
		return nil, fmt.Errorf("failed to unmarshal permissions: %w", err)
	}

	if err := json.Unmarshal([]byte(recipientsJSON), &share.Recipients); err != nil {
		return nil, fmt.Errorf("failed to unmarshal recipients: %w", err)
	}

	return &share, nil
}

func (r *LogManagementRepository) UpdateLogShare(share *models.LogShare) error {
	permissionsJSON, err := json.Marshal(share.Permissions)
	if err != nil {
		return fmt.Errorf("failed to marshal permissions: %w", err)
	}

	recipientsJSON, err := json.Marshal(share.Recipients)
	if err != nil {
		return fmt.Errorf("failed to marshal recipients: %w", err)
	}

	query := `
		UPDATE log_shares SET
			share_type = ?, expires_at = ?, accessed_at = ?, is_active = ?,
			permissions = ?, recipients = ?
		WHERE id = ?`

	_, err = r.db.Exec(query,
		share.ShareType, share.ExpiresAt, share.AccessedAt, share.IsActive,
		string(permissionsJSON), string(recipientsJSON), share.ID)

	if err != nil {
		return fmt.Errorf("failed to update log share: %w", err)
	}

	return nil
}

func (r *LogManagementRepository) GetLogSharesByUser(userID int) ([]*models.LogShare, error) {
	query := `
		SELECT id, collection_id, user_id, share_token, share_type, expires_at,
			   created_at, accessed_at, is_active, permissions, recipients
		FROM log_shares
		WHERE user_id = ?
		ORDER BY created_at DESC`

	rows, err := r.db.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get log shares: %w", err)
	}
	defer rows.Close()

	shares := []*models.LogShare{}
	for rows.Next() {
		var share models.LogShare
		var permissionsJSON, recipientsJSON string
		var accessedAt sql.NullTime

		err := rows.Scan(
			&share.ID, &share.CollectionID, &share.UserID, &share.ShareToken,
			&share.ShareType, &share.ExpiresAt, &share.CreatedAt, &accessedAt,
			&share.IsActive, &permissionsJSON, &recipientsJSON)

		if err != nil {
			return nil, fmt.Errorf("failed to scan log share: %w", err)
		}

		if accessedAt.Valid {
			share.AccessedAt = &accessedAt.Time
		}

		if err := json.Unmarshal([]byte(permissionsJSON), &share.Permissions); err != nil {
			return nil, fmt.Errorf("failed to unmarshal permissions: %w", err)
		}

		if err := json.Unmarshal([]byte(recipientsJSON), &share.Recipients); err != nil {
			return nil, fmt.Errorf("failed to unmarshal recipients: %w", err)
		}

		shares = append(shares, &share)
	}

	return shares, nil
}

func (r *LogManagementRepository) CleanupOldCollections(olderThan time.Time) error {
	query := "DELETE FROM log_collections WHERE created_at < ?"
	_, err := r.db.Exec(query, olderThan)
	if err != nil {
		return fmt.Errorf("failed to cleanup old collections: %w", err)
	}
	return nil
}

func (r *LogManagementRepository) CleanupExpiredShares() error {
	query := "UPDATE log_shares SET is_active = 0 WHERE expires_at < ? AND is_active = 1"
	_, err := r.db.Exec(query, time.Now())
	if err != nil {
		return fmt.Errorf("failed to cleanup expired shares: %w", err)
	}
	return nil
}

func (r *LogManagementRepository) GetLogStatistics(userID int) (*models.LogStatistics, error) {
	stats := &models.LogStatistics{}

	// Total collections
	err := r.db.QueryRow("SELECT COUNT(*) FROM log_collections WHERE user_id = ?", userID).Scan(&stats.TotalCollections)
	if err != nil {
		return nil, fmt.Errorf("failed to get total collections: %w", err)
	}

	// Total entries
	err = r.db.QueryRow(`
		SELECT COALESCE(SUM(lc.entry_count), 0)
		FROM log_collections lc
		WHERE lc.user_id = ?`, userID).Scan(&stats.TotalEntries)
	if err != nil {
		return nil, fmt.Errorf("failed to get total entries: %w", err)
	}

	// Active shares
	err = r.db.QueryRow(`
		SELECT COUNT(*)
		FROM log_shares
		WHERE user_id = ? AND is_active = 1 AND expires_at > ?`, userID, time.Now()).Scan(&stats.ActiveShares)
	if err != nil {
		return nil, fmt.Errorf("failed to get active shares: %w", err)
	}

	// Collections by status
	statusQuery := `
		SELECT status, COUNT(*)
		FROM log_collections
		WHERE user_id = ?
		GROUP BY status`

	rows, err := r.db.Query(statusQuery, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get status counts: %w", err)
	}
	defer rows.Close()

	stats.CollectionsByStatus = make(map[string]int)
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, fmt.Errorf("failed to scan status count: %w", err)
		}
		stats.CollectionsByStatus[status] = count
	}

	// Recent collections (last 7 days)
	cutoffTime := time.Now().Add(-7 * 24 * time.Hour)
	err = r.db.QueryRow(`
		SELECT COUNT(*)
		FROM log_collections
		WHERE user_id = ? AND created_at > ?`, userID, cutoffTime).Scan(&stats.RecentCollections)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent collections: %w", err)
	}

	return stats, nil
}
