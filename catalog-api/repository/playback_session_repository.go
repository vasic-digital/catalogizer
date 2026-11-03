package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"catalogizer/database"
)

// ErrPlaybackSessionNotFound is returned by End for an unknown session id.
var ErrPlaybackSessionNotFound = errors.New("playback session not found")

// PlaybackStart is the payload for PlaybackSessionRepository.Start.
type PlaybackStart struct {
	UserID        int64
	MediaItemID   int64
	FileID        *int64
	PositionUnit  string
	StartPosition int64
	StartedAt     time.Time
}

// PlaybackProgress updates the rolling end_position / total_amount
// on an active session without finalising it.
type PlaybackProgress struct {
	SessionID   int64
	EndPosition int64
	TotalAmount int64
}

// PlaybackEnd finalises a session and triggers the
// media_progress upsert in the same transaction. DurationTotal
// is optional — players report it once they've parsed the
// underlying media (duration is not always known at start(),
// e.g. HLS streams deliver it via the manifest). Passing nil
// leaves the column unchanged.
type PlaybackEnd struct {
	SessionID     int64
	EndPosition   int64
	TotalAmount   int64
	DurationTotal *int64
	EndedAt       time.Time
	Completed     bool
}

// PlaybackSession is one reproduction session row. end_position
// and ended_at are nullable while the session is still active;
// ListHistory / Get treat zero as "unset" for in-flight rows.
type PlaybackSession struct {
	ID            int64      `json:"id"`
	UserID        int64      `json:"user_id"`
	MediaItemID   int64      `json:"media_item_id"`
	FileID        *int64     `json:"file_id"`
	StartedAt     time.Time  `json:"started_at"`
	EndedAt       *time.Time `json:"ended_at"`
	PositionUnit  string     `json:"position_unit"`
	StartPosition int64      `json:"start_position"`
	EndPosition   int64      `json:"end_position"`
	TotalAmount   int64      `json:"total_amount"`
	Completed     bool       `json:"completed"`
}

// MediaProgress is the denormalised per-user, per-item summary
// used by the cards UI.
type MediaProgress struct {
	UserID             int64      `json:"user_id"`
	MediaItemID        int64      `json:"media_item_id"`
	PositionUnit       string     `json:"position_unit"`
	DurationTotal      *int64     `json:"duration_total"`
	LastPosition       int64      `json:"last_position"`
	LastSessionAmount  int64      `json:"last_session_amount"`
	TotalReproductions int64      `json:"total_reproductions"`
	AggregateAmount    int64      `json:"aggregate_amount"`
	LastSessionEndedAt *time.Time `json:"last_session_ended_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

// PlaybackSessionRepository persists reproduction sessions and
// the rolled-up media_progress snapshot used to render the
// ProgressBadge on every media card across apps.
type PlaybackSessionRepository struct {
	db *database.DB
}

// NewPlaybackSessionRepository wires a repository over the given
// database handle. The caller retains ownership of the DB.
func NewPlaybackSessionRepository(db *database.DB) *PlaybackSessionRepository {
	return &PlaybackSessionRepository{db: db}
}

// Start inserts a new playback_sessions row and returns its id.
// The session remains "open" until End is called — Progress can
// be invoked any number of times to bump end_position/total_amount
// without closing the session.
func (r *PlaybackSessionRepository) Start(ctx context.Context, s PlaybackStart) (int64, error) {
	if s.PositionUnit == "" {
		s.PositionUnit = "seconds"
	}
	if s.StartedAt.IsZero() {
		s.StartedAt = time.Now().UTC()
	}
	return r.db.InsertReturningID(ctx,
		`INSERT INTO playback_sessions
		    (user_id, media_item_id, file_id, started_at, position_unit,
		     start_position, end_position, total_amount, completed)
		 VALUES (?, ?, ?, ?, ?, ?, ?, 0, 0)`,
		s.UserID, s.MediaItemID, s.FileID, s.StartedAt, s.PositionUnit,
		s.StartPosition, s.StartPosition)
}

// Progress bumps end_position and total_amount on an open
// session. Silently no-ops if the session id doesn't exist — the
// caller already has start/end so an orphaned progress call is
// not fatal.
func (r *PlaybackSessionRepository) Progress(ctx context.Context, p PlaybackProgress) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE playback_sessions
		 SET end_position = ?, total_amount = ?
		 WHERE id = ?`,
		p.EndPosition, p.TotalAmount, p.SessionID)
	if err != nil {
		return fmt.Errorf("playback progress: %w", err)
	}
	if n, rerr := res.RowsAffected(); rerr == nil && n == 0 {
		return ErrPlaybackSessionNotFound
	}
	return nil
}

// End finalises a playback session and upserts the corresponding
// media_progress row in the same transaction so a crash between
// the two writes doesn't leave the summary stale.
func (r *PlaybackSessionRepository) End(ctx context.Context, e PlaybackEnd) error {
	if e.EndedAt.IsZero() {
		e.EndedAt = time.Now().UTC()
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	completedInt := 0
	if e.Completed {
		completedInt = 1
	}
	res, err := tx.ExecContext(ctx,
		`UPDATE playback_sessions
		 SET ended_at = ?, end_position = ?, total_amount = ?, completed = ?
		 WHERE id = ?`,
		e.EndedAt, e.EndPosition, e.TotalAmount, completedInt, e.SessionID)
	if err != nil {
		return fmt.Errorf("end session: %w", err)
	}
	if n, rerr := res.RowsAffected(); rerr == nil && n == 0 {
		return ErrPlaybackSessionNotFound
	}

	var userID, mediaItemID, endPos, totalAmount int64
	var posUnit string
	if err := tx.QueryRowContext(ctx,
		`SELECT user_id, media_item_id, position_unit,
		        COALESCE(end_position, 0), total_amount
		 FROM playback_sessions WHERE id = ?`, e.SessionID).Scan(
		&userID, &mediaItemID, &posUnit, &endPos, &totalAmount); err != nil {
		return fmt.Errorf("reload session: %w", err)
	}

	// media_progress upsert — implemented as a SELECT / UPDATE /
	// INSERT sequence rather than ON CONFLICT DO UPDATE because
	// the project's go-sqlcipher build is based on a pre-3.24
	// SQLite that doesn't recognise that syntax. Using the
	// portable pattern also works unchanged on PostgreSQL, which
	// is the other dialect we ship.
	var existingReps, existingAgg int64
	var existingDuration sql.NullInt64
	selErr := tx.QueryRowContext(ctx,
		`SELECT total_reproductions, aggregate_amount, duration_total
		 FROM media_progress WHERE user_id = ? AND media_item_id = ?`,
		userID, mediaItemID).Scan(&existingReps, &existingAgg, &existingDuration)

	// Pick the duration to persist: use the caller-provided value
	// when present, otherwise keep the existing one so a later
	// session without duration info doesn't wipe a previously-
	// learned value.
	var durToStore sql.NullInt64
	if e.DurationTotal != nil {
		durToStore = sql.NullInt64{Int64: *e.DurationTotal, Valid: true}
	} else {
		durToStore = existingDuration
	}

	switch {
	case selErr == sql.ErrNoRows:
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO media_progress
			    (user_id, media_item_id, position_unit, duration_total,
			     last_position, last_session_amount, total_reproductions,
			     aggregate_amount, last_session_ended_at, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, 1, ?, ?, ?)`,
			userID, mediaItemID, posUnit, durToStore,
			endPos, totalAmount, totalAmount,
			e.EndedAt, e.EndedAt); err != nil {
			return fmt.Errorf("insert media_progress: %w", err)
		}
	case selErr == nil:
		if _, err := tx.ExecContext(ctx,
			`UPDATE media_progress
			 SET position_unit         = ?,
			     duration_total        = ?,
			     last_position         = ?,
			     last_session_amount   = ?,
			     total_reproductions   = ?,
			     aggregate_amount      = ?,
			     last_session_ended_at = ?,
			     updated_at            = ?
			 WHERE user_id = ? AND media_item_id = ?`,
			posUnit, durToStore, endPos, totalAmount,
			existingReps+1, existingAgg+totalAmount,
			e.EndedAt, e.EndedAt,
			userID, mediaItemID); err != nil {
			return fmt.Errorf("update media_progress: %w", err)
		}
	default:
		return fmt.Errorf("load media_progress: %w", selErr)
	}

	return tx.Commit()
}

// Get loads a single playback_sessions row by id.
func (r *PlaybackSessionRepository) Get(ctx context.Context, id int64) (*PlaybackSession, error) {
	var s PlaybackSession
	var completed int
	var fileID sql.NullInt64
	var endedAt sql.NullTime
	var endPos sql.NullInt64
	if err := r.db.QueryRowContext(ctx,
		`SELECT id, user_id, media_item_id, file_id, started_at, ended_at,
		        position_unit, start_position, end_position, total_amount, completed
		 FROM playback_sessions WHERE id = ?`, id).Scan(
		&s.ID, &s.UserID, &s.MediaItemID, &fileID, &s.StartedAt, &endedAt,
		&s.PositionUnit, &s.StartPosition, &endPos, &s.TotalAmount, &completed,
	); err != nil {
		return nil, err
	}
	if fileID.Valid {
		v := fileID.Int64
		s.FileID = &v
	}
	if endedAt.Valid {
		t := endedAt.Time
		s.EndedAt = &t
	}
	if endPos.Valid {
		s.EndPosition = endPos.Int64
	}
	s.Completed = completed != 0
	return &s, nil
}

// GetProgress returns the per-user, per-entity progress summary
// or sql.ErrNoRows if the user has never reproduced the item.
func (r *PlaybackSessionRepository) GetProgress(ctx context.Context, userID, mediaItemID int64) (*MediaProgress, error) {
	var p MediaProgress
	var durTotal sql.NullInt64
	var lastEndedAt sql.NullTime
	if err := r.db.QueryRowContext(ctx,
		`SELECT user_id, media_item_id, position_unit, duration_total,
		        last_position, last_session_amount, total_reproductions,
		        aggregate_amount, last_session_ended_at, updated_at
		 FROM media_progress WHERE user_id = ? AND media_item_id = ?`,
		userID, mediaItemID).Scan(
		&p.UserID, &p.MediaItemID, &p.PositionUnit, &durTotal,
		&p.LastPosition, &p.LastSessionAmount, &p.TotalReproductions,
		&p.AggregateAmount, &lastEndedAt, &p.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if durTotal.Valid {
		v := durTotal.Int64
		p.DurationTotal = &v
	}
	if lastEndedAt.Valid {
		t := lastEndedAt.Time
		p.LastSessionEndedAt = &t
	}
	return &p, nil
}

// ListHistory returns up to `limit` sessions for (userID,
// mediaItemID) ordered by started_at DESC. Clamps limit to a
// safe range so clients can't DoS the endpoint with a huge
// limit parameter.
func (r *PlaybackSessionRepository) ListHistory(ctx context.Context, userID, mediaItemID int64, limit int) ([]PlaybackSession, error) {
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, user_id, media_item_id, file_id, started_at, ended_at,
		        position_unit, start_position, end_position, total_amount, completed
		 FROM playback_sessions
		 WHERE user_id = ? AND media_item_id = ?
		 ORDER BY started_at DESC
		 LIMIT ?`,
		userID, mediaItemID, limit)
	if err != nil {
		return nil, fmt.Errorf("list history: %w", err)
	}
	defer rows.Close()

	var out []PlaybackSession
	for rows.Next() {
		var s PlaybackSession
		var completed int
		var fileID sql.NullInt64
		var endedAt sql.NullTime
		var endPos sql.NullInt64
		if err := rows.Scan(&s.ID, &s.UserID, &s.MediaItemID, &fileID,
			&s.StartedAt, &endedAt, &s.PositionUnit, &s.StartPosition,
			&endPos, &s.TotalAmount, &completed); err != nil {
			return nil, err
		}
		if fileID.Valid {
			v := fileID.Int64
			s.FileID = &v
		}
		if endedAt.Valid {
			t := endedAt.Time
			s.EndedAt = &t
		}
		if endPos.Valid {
			s.EndPosition = endPos.Int64
		}
		s.Completed = completed != 0
		out = append(out, s)
	}
	return out, rows.Err()
}
