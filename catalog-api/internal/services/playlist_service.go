package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"catalogizer/database"

	"go.uber.org/zap"
)

type PlaylistService struct {
	db     *database.DB
	logger *zap.Logger
}

type Playlist struct {
	ID              int64      `json:"id" db:"id"`
	UserID          int64      `json:"user_id" db:"user_id"`
	Name            string     `json:"name" db:"name"`
	Description     string     `json:"description" db:"description"`
	IsPublic        bool       `json:"is_public" db:"is_public"`
	IsSmartPlaylist bool       `json:"is_smart_playlist" db:"is_smart_playlist"`
	SmartCriteria   string     `json:"smart_criteria" db:"smart_criteria"`
	CoverArtURL     string     `json:"cover_art_url" db:"cover_art_url"`
	TrackCount      int        `json:"track_count" db:"track_count"`
	TotalDuration   int64      `json:"total_duration" db:"total_duration"`
	PlayCount       int64      `json:"play_count" db:"play_count"`
	LastPlayed      *time.Time `json:"last_played" db:"last_played"`
	CreatedAt       time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at" db:"updated_at"`
	Tags            []string   `json:"tags"`
	CollaboratorIDs []int64    `json:"collaborator_ids"`
}

type PlaylistItem struct {
	ID          int64     `json:"id" db:"id"`
	PlaylistID  int64     `json:"playlist_id" db:"playlist_id"`
	MediaItemID int64     `json:"media_item_id" db:"media_item_id"`
	Position    int       `json:"position" db:"position"`
	AddedBy     int64     `json:"added_by" db:"added_by"`
	AddedAt     time.Time `json:"added_at" db:"added_at"`
	CustomTitle string    `json:"custom_title" db:"custom_title"`
	StartTime   *int64    `json:"start_time" db:"start_time"`
	EndTime     *int64    `json:"end_time" db:"end_time"`
}

type PlaybackQueue struct {
	ID              int64     `json:"id" db:"id"`
	UserID          int64     `json:"user_id" db:"user_id"`
	Name            string    `json:"name" db:"name"`
	CurrentItemID   *int64    `json:"current_item_id" db:"current_item_id"`
	CurrentPosition int       `json:"current_position" db:"current_position"`
	ShuffleEnabled  bool      `json:"shuffle_enabled" db:"shuffle_enabled"`
	RepeatMode      string    `json:"repeat_mode" db:"repeat_mode"` // none, track, playlist
	ShuffleHistory  string    `json:"shuffle_history" db:"shuffle_history"`
	CreatedAt       time.Time `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time `json:"updated_at" db:"updated_at"`
}

type QueueItem struct {
	ID               int64     `json:"id" db:"id"`
	QueueID          int64     `json:"queue_id" db:"queue_id"`
	MediaItemID      int64     `json:"media_item_id" db:"media_item_id"`
	Position         int       `json:"position" db:"position"`
	OriginalPosition int       `json:"original_position" db:"original_position"`
	PlayCount        int       `json:"play_count" db:"play_count"`
	AddedAt          time.Time `json:"added_at" db:"added_at"`
}

type SmartPlaylistCriteria struct {
	Rules []SmartRule `json:"rules"`
	Logic string      `json:"logic"` // "AND" or "OR"
	Limit int         `json:"limit"`
	Order string      `json:"order"` // "added_desc", "added_asc", "play_count_desc", "random", etc.
}

type SmartRule struct {
	Field    string      `json:"field"`    // "genre", "artist", "album", "year", "rating", "play_count", etc.
	Operator string      `json:"operator"` // "equals", "contains", "greater_than", "less_than", "in", etc.
	Value    interface{} `json:"value"`
}

type CreatePlaylistRequest struct {
	UserID          int64                  `json:"user_id"`
	Name            string                 `json:"name"`
	Description     string                 `json:"description"`
	IsPublic        bool                   `json:"is_public"`
	IsSmartPlaylist bool                   `json:"is_smart_playlist"`
	SmartCriteria   *SmartPlaylistCriteria `json:"smart_criteria"`
	Tags            []string               `json:"tags"`
	CollaboratorIDs []int64                `json:"collaborator_ids"`
}

type UpdatePlaylistRequest struct {
	ID              int64    `json:"id"`
	UserID          int64    `json:"user_id"`
	Name            *string  `json:"name"`
	Description     *string  `json:"description"`
	IsPublic        *bool    `json:"is_public"`
	CoverArtURL     *string  `json:"cover_art_url"`
	Tags            []string `json:"tags"`
	CollaboratorIDs []int64  `json:"collaborator_ids"`
}

type AddToPlaylistRequest struct {
	PlaylistID   int64            `json:"playlist_id"`
	MediaItemIDs []int64          `json:"media_item_ids"`
	UserID       int64            `json:"user_id"`
	Position     *int             `json:"position"`
	CustomTitles map[int64]string `json:"custom_titles"`
}

type ReorderPlaylistRequest struct {
	PlaylistID  int64 `json:"playlist_id"`
	UserID      int64 `json:"user_id"`
	ItemID      int64 `json:"item_id"`
	NewPosition int   `json:"new_position"`
}

type PlaylistSearchRequest struct {
	UserID   int64    `json:"user_id"`
	Query    string   `json:"query"`
	IsPublic *bool    `json:"is_public"`
	Tags     []string `json:"tags"`
	Limit    int      `json:"limit"`
	Offset   int      `json:"offset"`
}

func NewPlaylistService(db *database.DB, logger *zap.Logger) *PlaylistService {
	return &PlaylistService{
		db:     db,
		logger: logger,
	}
}

func (s *PlaylistService) CreatePlaylist(ctx context.Context, req *CreatePlaylistRequest) (*Playlist, error) {
	s.logger.Info("Creating playlist",
		zap.Int64("user_id", req.UserID),
		zap.String("name", req.Name),
		zap.Bool("is_smart", req.IsSmartPlaylist))

	var smartCriteriaJSON string
	if req.SmartCriteria != nil {
		criteriaBytes, err := json.Marshal(req.SmartCriteria)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal smart criteria: %w", err)
		}
		smartCriteriaJSON = string(criteriaBytes)
	}

	query := `
		INSERT INTO playlists (
			user_id, name, description, is_public, is_smart_playlist,
			smart_criteria, track_count, total_duration, play_count,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, 0, 0, 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`

	var playlist Playlist
	id, err := s.db.InsertReturningID(ctx, query,
		req.UserID, req.Name, req.Description, req.IsPublic,
		req.IsSmartPlaylist, smartCriteriaJSON)
	if err != nil {
		s.logger.Error("Failed to create playlist", zap.Error(err))
		return nil, fmt.Errorf("failed to create playlist: %w", err)
	}

	playlist.ID = id
	playlist.CreatedAt = time.Now()
	playlist.UpdatedAt = time.Now()

	playlist.UserID = req.UserID
	playlist.Name = req.Name
	playlist.Description = req.Description
	playlist.IsPublic = req.IsPublic
	playlist.IsSmartPlaylist = req.IsSmartPlaylist
	playlist.SmartCriteria = smartCriteriaJSON
	playlist.Tags = req.Tags
	playlist.CollaboratorIDs = req.CollaboratorIDs

	if len(req.Tags) > 0 {
		if err := s.updatePlaylistTags(ctx, playlist.ID, req.Tags); err != nil {
			s.logger.Warn("Failed to add tags to playlist", zap.Error(err))
		}
	}

	if len(req.CollaboratorIDs) > 0 {
		if err := s.updatePlaylistCollaborators(ctx, playlist.ID, req.CollaboratorIDs); err != nil {
			s.logger.Warn("Failed to add collaborators to playlist", zap.Error(err))
		}
	}

	if req.IsSmartPlaylist && req.SmartCriteria != nil {
		if err := s.RefreshSmartPlaylist(ctx, playlist.ID); err != nil {
			s.logger.Warn("Failed to populate smart playlist", zap.Error(err))
		}
	}

	return &playlist, nil
}

func (s *PlaylistService) GetPlaylist(ctx context.Context, playlistID, userID int64) (*Playlist, error) {
	s.logger.Debug("Getting playlist",
		zap.Int64("playlist_id", playlistID),
		zap.Int64("user_id", userID))

	query := `
		SELECT p.id, p.user_id, p.name, p.description, p.is_public,
			   p.is_smart_playlist, p.smart_criteria, p.cover_art_url,
			   p.track_count, p.total_duration, p.play_count, p.last_played,
			   p.created_at, p.updated_at
		FROM playlists p
		LEFT JOIN playlist_collaborators pc ON p.id = pc.playlist_id
		WHERE p.id = ?
		  AND (p.user_id = ? OR p.is_public = 1 OR pc.user_id = ?)
	`

	var playlist Playlist
	err := s.db.QueryRowContext(ctx, query, playlistID, userID, userID).Scan(
		&playlist.ID, &playlist.UserID, &playlist.Name, &playlist.Description,
		&playlist.IsPublic, &playlist.IsSmartPlaylist, &playlist.SmartCriteria,
		&playlist.CoverArtURL, &playlist.TrackCount, &playlist.TotalDuration,
		&playlist.PlayCount, &playlist.LastPlayed, &playlist.CreatedAt, &playlist.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("playlist not found or access denied")
	}
	if err != nil {
		s.logger.Error("Failed to get playlist", zap.Error(err))
		return nil, fmt.Errorf("failed to get playlist: %w", err)
	}

	playlist.Tags, _ = s.getPlaylistTags(ctx, playlistID)
	playlist.CollaboratorIDs, _ = s.getPlaylistCollaborators(ctx, playlistID)

	return &playlist, nil
}

func (s *PlaylistService) GetUserPlaylists(ctx context.Context, userID int64, includePublic bool) ([]Playlist, error) {
	s.logger.Debug("Getting user playlists",
		zap.Int64("user_id", userID),
		zap.Bool("include_public", includePublic))

	baseQuery := `
		SELECT DISTINCT p.id, p.user_id, p.name, p.description, p.is_public,
			   p.is_smart_playlist, p.smart_criteria, p.cover_art_url,
			   p.track_count, p.total_duration, p.play_count, p.last_played,
			   p.created_at, p.updated_at
		FROM playlists p
		LEFT JOIN playlist_collaborators pc ON p.id = pc.playlist_id
		WHERE p.user_id = ? OR pc.user_id = ?
	`

	if includePublic {
		baseQuery += " OR p.is_public = 1"
	}

	baseQuery += " ORDER BY p.updated_at DESC"

	rows, err := s.db.QueryContext(ctx, baseQuery, userID, userID)
	if err != nil {
		s.logger.Error("Failed to get user playlists", zap.Error(err))
		return nil, fmt.Errorf("failed to get user playlists: %w", err)
	}
	defer rows.Close()

	// Buffer the page and release the cursor (and its pooled connection) BEFORE the per-playlist
	// tag/collaborator queries: a query issued while this cursor is open needs a second pooled
	// connection, so concurrent calls exceeding the pool would all hold one and wait for another.
	var page []Playlist
	for rows.Next() {
		var playlist Playlist
		err := rows.Scan(
			&playlist.ID, &playlist.UserID, &playlist.Name, &playlist.Description,
			&playlist.IsPublic, &playlist.IsSmartPlaylist, &playlist.SmartCriteria,
			&playlist.CoverArtURL, &playlist.TrackCount, &playlist.TotalDuration,
			&playlist.PlayCount, &playlist.LastPlayed, &playlist.CreatedAt, &playlist.UpdatedAt,
		)
		if err != nil {
			s.logger.Error("Failed to scan playlist", zap.Error(err))
			continue
		}
		page = append(page, playlist)
	}
	rows.Close()

	var playlists []Playlist
	for _, playlist := range page {
		playlist.Tags, _ = s.getPlaylistTags(ctx, playlist.ID)
		playlist.CollaboratorIDs, _ = s.getPlaylistCollaborators(ctx, playlist.ID)

		playlists = append(playlists, playlist)
	}

	return playlists, nil
}

func (s *PlaylistService) AddToPlaylist(ctx context.Context, req *AddToPlaylistRequest) error {
	s.logger.Info("Adding items to playlist",
		zap.Int64("playlist_id", req.PlaylistID),
		zap.Int("item_count", len(req.MediaItemIDs)))

	if !s.canModifyPlaylist(ctx, req.PlaylistID, req.UserID) {
		return fmt.Errorf("permission denied to modify playlist")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	position := 0
	if req.Position != nil {
		position = *req.Position
	} else {
		err := tx.QueryRowContext(ctx,
			"SELECT COALESCE(MAX(position), 0) + 1 FROM playlist_items WHERE playlist_id = ?",
			req.PlaylistID).Scan(&position)
		if err != nil {
			return fmt.Errorf("failed to get next position: %w", err)
		}
	}

	if req.Position != nil {
		_, err := tx.ExecContext(ctx,
			"UPDATE playlist_items SET position = position + ? WHERE playlist_id = ? AND position >= ?",
			len(req.MediaItemIDs), req.PlaylistID, *req.Position)
		if err != nil {
			return fmt.Errorf("failed to shift positions: %w", err)
		}
	}

	for i, mediaItemID := range req.MediaItemIDs {
		customTitle := ""
		if req.CustomTitles != nil {
			customTitle = req.CustomTitles[mediaItemID]
		}

		_, err := tx.ExecContext(ctx, `
			INSERT INTO playlist_items (playlist_id, media_item_id, position, added_by, added_at, custom_title)
			VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP, ?)
		`, req.PlaylistID, mediaItemID, position+i, req.UserID, customTitle)

		if err != nil {
			s.logger.Error("Failed to add item to playlist", zap.Error(err))
			return fmt.Errorf("failed to add item to playlist: %w", err)
		}
	}

	if err := s.updatePlaylistStats(ctx, tx, req.PlaylistID); err != nil {
		return fmt.Errorf("failed to update playlist stats: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (s *PlaylistService) RemoveFromPlaylist(ctx context.Context, playlistID, itemID, userID int64) error {
	s.logger.Info("Removing item from playlist",
		zap.Int64("playlist_id", playlistID),
		zap.Int64("item_id", itemID))

	if !s.canModifyPlaylist(ctx, playlistID, userID) {
		return fmt.Errorf("permission denied to modify playlist")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	var position int
	err = tx.QueryRowContext(ctx,
		"SELECT position FROM playlist_items WHERE id = ? AND playlist_id = ?",
		itemID, playlistID).Scan(&position)
	if err != nil {
		return fmt.Errorf("playlist item not found: %w", err)
	}

	_, err = tx.ExecContext(ctx,
		"DELETE FROM playlist_items WHERE id = ? AND playlist_id = ?",
		itemID, playlistID)
	if err != nil {
		return fmt.Errorf("failed to remove item: %w", err)
	}

	_, err = tx.ExecContext(ctx,
		"UPDATE playlist_items SET position = position - 1 WHERE playlist_id = ? AND position > ?",
		playlistID, position)
	if err != nil {
		return fmt.Errorf("failed to update positions: %w", err)
	}

	if err := s.updatePlaylistStats(ctx, tx, playlistID); err != nil {
		return fmt.Errorf("failed to update playlist stats: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (s *PlaylistService) ReorderPlaylist(ctx context.Context, req *ReorderPlaylistRequest) error {
	s.logger.Info("Reordering playlist",
		zap.Int64("playlist_id", req.PlaylistID),
		zap.Int64("item_id", req.ItemID),
		zap.Int("new_position", req.NewPosition))

	if !s.canModifyPlaylist(ctx, req.PlaylistID, req.UserID) {
		return fmt.Errorf("permission denied to modify playlist")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	var currentPosition int
	err = tx.QueryRowContext(ctx,
		"SELECT position FROM playlist_items WHERE id = ? AND playlist_id = ?",
		req.ItemID, req.PlaylistID).Scan(&currentPosition)
	if err != nil {
		return fmt.Errorf("playlist item not found: %w", err)
	}

	if currentPosition == req.NewPosition {
		return nil
	}

	// Step 1: Move the item to a temporary position outside the valid range
	// This frees up its current position
	_, err = tx.ExecContext(ctx,
		"UPDATE playlist_items SET position = -10000 WHERE id = ?",
		req.ItemID)
	if err != nil {
		return fmt.Errorf("failed to move item to temp position: %w", err)
	}

	// Step 2: Shift items to close the gap and make room at new position
	if currentPosition < req.NewPosition {
		// Moving down: shift items between currentPosition and newPosition up by 1
		_, err = tx.ExecContext(ctx, `
			UPDATE playlist_items
			SET position = position - 1
			WHERE playlist_id = ? AND position > ? AND position <= ?
		`, req.PlaylistID, currentPosition, req.NewPosition)
	} else {
		// Moving up: shift items between newPosition and currentPosition down by 1
		_, err = tx.ExecContext(ctx, `
			UPDATE playlist_items
			SET position = position + 1
			WHERE playlist_id = ? AND position >= ? AND position < ?
		`, req.PlaylistID, req.NewPosition, currentPosition)
	}

	if err != nil {
		return fmt.Errorf("failed to shift positions: %w", err)
	}

	// Step 3: Move the item to its final position
	_, err = tx.ExecContext(ctx,
		"UPDATE playlist_items SET position = ? WHERE id = ?",
		req.NewPosition, req.ItemID)
	if err != nil {
		return fmt.Errorf("failed to set final position: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (s *PlaylistService) GetPlaylistItems(ctx context.Context, playlistID, userID int64, limit, offset int) ([]PlaylistItem, error) {
	s.logger.Debug("Getting playlist items",
		zap.Int64("playlist_id", playlistID),
		zap.Int("limit", limit),
		zap.Int("offset", offset))

	query := `
		SELECT pi.id, pi.playlist_id, pi.media_item_id, pi.position,
			   pi.added_by, pi.added_at, pi.custom_title, pi.start_time, pi.end_time
		FROM playlist_items pi
		INNER JOIN playlists p ON pi.playlist_id = p.id
		LEFT JOIN playlist_collaborators pc ON p.id = pc.playlist_id
		WHERE pi.playlist_id = ?
		  AND (p.user_id = ? OR p.is_public = 1 OR pc.user_id = ?)
		ORDER BY pi.position ASC
		LIMIT ? OFFSET ?
	`

	rows, err := s.db.QueryContext(ctx, query, playlistID, userID, userID, limit, offset)
	if err != nil {
		s.logger.Error("Failed to get playlist items", zap.Error(err))
		return nil, fmt.Errorf("failed to get playlist items: %w", err)
	}
	defer rows.Close()

	var items []PlaylistItem
	for rows.Next() {
		var item PlaylistItem
		err := rows.Scan(
			&item.ID, &item.PlaylistID, &item.MediaItemID, &item.Position,
			&item.AddedBy, &item.AddedAt, &item.CustomTitle, &item.StartTime, &item.EndTime,
		)
		if err != nil {
			s.logger.Error("Failed to scan playlist item", zap.Error(err))
			continue
		}
		items = append(items, item)
	}

	return items, nil
}

func (s *PlaylistService) RefreshSmartPlaylist(ctx context.Context, playlistID int64) error {
	s.logger.Info("Refreshing smart playlist", zap.Int64("playlist_id", playlistID))

	playlist, err := s.GetPlaylist(ctx, playlistID, 0)
	if err != nil {
		return fmt.Errorf("failed to get playlist: %w", err)
	}

	if !playlist.IsSmartPlaylist {
		return fmt.Errorf("playlist is not a smart playlist")
	}

	var criteria SmartPlaylistCriteria
	if err := json.Unmarshal([]byte(playlist.SmartCriteria), &criteria); err != nil {
		return fmt.Errorf("failed to parse smart criteria: %w", err)
	}

	query, args := s.buildSmartPlaylistQuery(&criteria)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, "DELETE FROM playlist_items WHERE playlist_id = ?", playlistID)
	if err != nil {
		return fmt.Errorf("failed to clear playlist: %w", err)
	}

	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to execute smart query: %w", err)
	}
	defer rows.Close()

	position := 1
	for rows.Next() {
		var mediaItemID int64
		err := rows.Scan(&mediaItemID)
		if err != nil {
			continue
		}

		_, err = tx.ExecContext(ctx, `
			INSERT INTO playlist_items (playlist_id, media_item_id, position, added_by, added_at)
			VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
		`, playlistID, mediaItemID, position, playlist.UserID)

		if err != nil {
			s.logger.Error("Failed to add smart playlist item", zap.Error(err))
			continue
		}
		position++
	}

	if err := s.updatePlaylistStats(ctx, tx, playlistID); err != nil {
		return fmt.Errorf("failed to update playlist stats: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (s *PlaylistService) CreateQueue(ctx context.Context, userID int64, name string) (*PlaybackQueue, error) {
	s.logger.Info("Creating playback queue",
		zap.Int64("user_id", userID),
		zap.String("name", name))

	query := `
		INSERT INTO playback_queues (user_id, name, current_position, shuffle_enabled, repeat_mode, created_at, updated_at)
		VALUES (?, ?, 0, false, 'none', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`

	var queue PlaybackQueue
	queueID, err := s.db.InsertReturningID(ctx, query, userID, name)
	if err != nil {
		s.logger.Error("Failed to create queue", zap.Error(err))
		return nil, fmt.Errorf("failed to create queue: %w", err)
	}

	queue.ID = queueID
	queue.CreatedAt = time.Now()
	queue.UpdatedAt = time.Now()

	queue.UserID = userID
	queue.Name = name
	queue.CurrentPosition = 0
	queue.ShuffleEnabled = false
	queue.RepeatMode = "none"

	return &queue, nil
}

func (s *PlaylistService) AddToQueue(ctx context.Context, queueID int64, mediaItemIDs []int64) error {
	s.logger.Info("Adding items to queue",
		zap.Int64("queue_id", queueID),
		zap.Int("item_count", len(mediaItemIDs)))

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	var position int
	err = tx.QueryRowContext(ctx,
		"SELECT COALESCE(MAX(position), 0) + 1 FROM queue_items WHERE queue_id = ?",
		queueID).Scan(&position)
	if err != nil {
		return fmt.Errorf("failed to get next position: %w", err)
	}

	for i, mediaItemID := range mediaItemIDs {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO queue_items (queue_id, media_item_id, position, original_position, play_count, added_at)
			VALUES (?, ?, ?, ?, 0, CURRENT_TIMESTAMP)
		`, queueID, mediaItemID, position+i, position+i)

		if err != nil {
			s.logger.Error("Failed to add item to queue", zap.Error(err))
			return fmt.Errorf("failed to add item to queue: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (s *PlaylistService) ShuffleQueue(ctx context.Context, queueID int64, enabled bool) error {
	s.logger.Info("Setting queue shuffle",
		zap.Int64("queue_id", queueID),
		zap.Bool("enabled", enabled))

	_, err := s.db.ExecContext(ctx,
		"UPDATE playback_queues SET shuffle_enabled = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
		enabled, queueID)

	if err != nil {
		s.logger.Error("Failed to update queue shuffle", zap.Error(err))
		return fmt.Errorf("failed to update queue shuffle: %w", err)
	}

	return nil
}

func (s *PlaylistService) canModifyPlaylist(ctx context.Context, playlistID, userID int64) bool {
	var count int
	query := `
		SELECT COUNT(*)
		FROM playlists p
		LEFT JOIN playlist_collaborators pc ON p.id = pc.playlist_id
		WHERE p.id = ? AND (p.user_id = ? OR pc.user_id = ?)
	`

	err := s.db.QueryRowContext(ctx, query, playlistID, userID, userID).Scan(&count)
	return err == nil && count > 0
}

func (s *PlaylistService) updatePlaylistStats(ctx context.Context, tx *sql.Tx, playlistID int64) error {
	query := `
		UPDATE playlists
		SET track_count = (
				SELECT COUNT(*) FROM playlist_items WHERE playlist_id = ?
			),
			total_duration = (
				SELECT COALESCE(SUM(mi.duration), 0)
				FROM playlist_items pi
				JOIN media_items mi ON pi.media_item_id = mi.id
				WHERE pi.playlist_id = ?
			),
			updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`

	_, err := tx.ExecContext(ctx, query, playlistID, playlistID, playlistID)
	return err
}

func (s *PlaylistService) updatePlaylistTags(ctx context.Context, playlistID int64, tags []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, "DELETE FROM playlist_tags WHERE playlist_id = ?", playlistID)
	if err != nil {
		return err
	}

	for _, tag := range tags {
		_, err = tx.ExecContext(ctx,
			"INSERT INTO playlist_tags (playlist_id, tag) VALUES (?, ?)",
			playlistID, tag)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *PlaylistService) updatePlaylistCollaborators(ctx context.Context, playlistID int64, collaboratorIDs []int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, "DELETE FROM playlist_collaborators WHERE playlist_id = ?", playlistID)
	if err != nil {
		return err
	}

	for _, userID := range collaboratorIDs {
		_, err = tx.ExecContext(ctx,
			"INSERT INTO playlist_collaborators (playlist_id, user_id, added_at) VALUES (?, ?, CURRENT_TIMESTAMP)",
			playlistID, userID)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *PlaylistService) getPlaylistTags(ctx context.Context, playlistID int64) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT tag FROM playlist_tags WHERE playlist_id = ?", playlistID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []string
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err == nil {
			tags = append(tags, tag)
		}
	}
	return tags, nil
}

func (s *PlaylistService) getPlaylistCollaborators(ctx context.Context, playlistID int64) ([]int64, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT user_id FROM playlist_collaborators WHERE playlist_id = ?", playlistID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var collaborators []int64
	for rows.Next() {
		var userID int64
		if err := rows.Scan(&userID); err == nil {
			collaborators = append(collaborators, userID)
		}
	}
	return collaborators, nil
}

func (s *PlaylistService) buildSmartPlaylistQuery(criteria *SmartPlaylistCriteria) (string, []interface{}) {
	baseQuery := "SELECT DISTINCT mi.id FROM media_items mi WHERE "
	var conditions []string
	var args []interface{}
	argIndex := 1

	logic := "AND"
	if criteria.Logic == "OR" {
		logic = "OR"
	}

	for _, rule := range criteria.Rules {
		condition, ruleArgs := s.buildRuleCondition(rule, &argIndex)
		if condition != "" {
			conditions = append(conditions, condition)
			args = append(args, ruleArgs...)
		}
	}

	if len(conditions) == 0 {
		return "SELECT id FROM media_items LIMIT 0", []interface{}{}
	}

	query := baseQuery + "(" + strings.Join(conditions, " "+logic+" ") + ")"

	if criteria.Order != "" {
		query += " ORDER BY " + s.getOrderClause(criteria.Order)
	}

	if criteria.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", criteria.Limit)
	}

	return query, args
}

func (s *PlaylistService) buildRuleCondition(rule SmartRule, argIndex *int) (string, []interface{}) {
	var condition string
	var args []interface{}

	switch rule.Field {
	case "genre":
		if rule.Operator == "equals" {
			condition = "mi.genre = ?"
			args = append(args, rule.Value)
			*argIndex++
		} else if rule.Operator == "contains" {
			condition = "mi.genre LIKE ?"
			args = append(args, "%"+rule.Value.(string)+"%")
			*argIndex++
		}
	case "artist":
		if rule.Operator == "equals" {
			condition = "mi.artist = ?"
			args = append(args, rule.Value)
			*argIndex++
		} else if rule.Operator == "contains" {
			condition = "mi.artist LIKE ?"
			args = append(args, "%"+rule.Value.(string)+"%")
			*argIndex++
		}
	case "year":
		if rule.Operator == "equals" {
			condition = "mi.year = ?"
			args = append(args, rule.Value)
			*argIndex++
		} else if rule.Operator == "greater_than" {
			condition = "mi.year > ?"
			args = append(args, rule.Value)
			*argIndex++
		} else if rule.Operator == "less_than" {
			condition = "mi.year < ?"
			args = append(args, rule.Value)
			*argIndex++
		}
	case "rating":
		if rule.Operator == "greater_than" {
			condition = "mi.rating > ?"
			args = append(args, rule.Value)
			*argIndex++
		}
	}

	return condition, args
}

func (s *PlaylistService) getOrderClause(order string) string {
	switch order {
	case "added_desc":
		return "mi.created_at DESC"
	case "added_asc":
		return "mi.created_at ASC"
	case "play_count_desc":
		return "mi.play_count DESC"
	case "rating_desc":
		return "mi.rating DESC"
	case "random":
		return "RANDOM()"
	case "title_asc":
		return "mi.title ASC"
	case "artist_asc":
		return "mi.artist ASC, mi.album ASC, mi.track_number ASC"
	default:
		return "mi.created_at DESC"
	}
}
