package repository

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"catalogizer/database"
	"catalogizer/models"
)

type AnalyticsRepository struct {
	db *database.DB
}

func NewAnalyticsRepository(db *database.DB) *AnalyticsRepository {
	return &AnalyticsRepository{db: db}
}

func (r *AnalyticsRepository) LogMediaAccess(access *models.MediaAccessLog) error {
	query := `
		INSERT INTO media_access_logs (user_id, media_id, action, device_info, location,
									  ip_address, user_agent, playback_duration, access_time)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	var deviceInfoJSON, locationJSON *string

	if access.DeviceInfo != nil {
		data, err := json.Marshal(access.DeviceInfo)
		if err != nil {
			return fmt.Errorf("failed to marshal device info: %w", err)
		}
		deviceInfoStr := string(data)
		deviceInfoJSON = &deviceInfoStr
	}

	if access.Location != nil {
		data, err := json.Marshal(access.Location)
		if err != nil {
			return fmt.Errorf("failed to marshal location: %w", err)
		}
		locationStr := string(data)
		locationJSON = &locationStr
	}

	var playbackDurationSeconds *int64
	if access.PlaybackDuration != nil {
		seconds := int64(access.PlaybackDuration.Seconds())
		playbackDurationSeconds = &seconds
	}

	_, err := r.db.Exec(query,
		access.UserID, access.MediaID, access.Action, deviceInfoJSON, locationJSON,
		access.IPAddress, access.UserAgent, playbackDurationSeconds, access.AccessTime)

	return err
}

func (r *AnalyticsRepository) LogEvent(event *models.AnalyticsEvent) error {
	query := `
		INSERT INTO analytics_events (user_id, event_type, event_category, data,
									 device_info, location, ip_address, user_agent, timestamp)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	var deviceInfoJSON, locationJSON *string

	if event.DeviceInfo != nil {
		data, err := json.Marshal(event.DeviceInfo)
		if err != nil {
			return fmt.Errorf("failed to marshal device info: %w", err)
		}
		deviceInfoStr := string(data)
		deviceInfoJSON = &deviceInfoStr
	}

	if event.Location != nil {
		data, err := json.Marshal(event.Location)
		if err != nil {
			return fmt.Errorf("failed to marshal location: %w", err)
		}
		locationStr := string(data)
		locationJSON = &locationStr
	}

	_, err := r.db.Exec(query,
		event.UserID, event.EventType, event.EventCategory, event.Data,
		deviceInfoJSON, locationJSON, event.IPAddress, event.UserAgent, event.Timestamp)

	return err
}

func (r *AnalyticsRepository) GetMediaAccessLogs(userID int, mediaID *int, limit, offset int) ([]models.MediaAccessLog, error) {
	var query string
	var args []interface{}

	if mediaID != nil {
		if userID > 0 {
			query = `
				SELECT id, user_id, media_id, action, device_info, location, ip_address,
					   user_agent, playback_duration, access_time
				FROM media_access_logs
				WHERE user_id = ? AND media_id = ?
				ORDER BY access_time DESC
				LIMIT ? OFFSET ?
			`
			args = []interface{}{userID, *mediaID, limit, offset}
		} else {
			query = `
				SELECT id, user_id, media_id, action, device_info, location, ip_address,
					   user_agent, playback_duration, access_time
				FROM media_access_logs
				WHERE media_id = ?
				ORDER BY access_time DESC
				LIMIT ? OFFSET ?
			`
			args = []interface{}{*mediaID, limit, offset}
		}
	} else if userID > 0 {
		query = `
			SELECT id, user_id, media_id, action, device_info, location, ip_address,
				   user_agent, playback_duration, access_time
			FROM media_access_logs
			WHERE user_id = ?
			ORDER BY access_time DESC
			LIMIT ? OFFSET ?
		`
		args = []interface{}{userID, limit, offset}
	} else {
		query = `
			SELECT id, user_id, media_id, action, device_info, location, ip_address,
				   user_agent, playback_duration, access_time
			FROM media_access_logs
			ORDER BY access_time DESC
			LIMIT ? OFFSET ?
		`
		args = []interface{}{limit, offset}
	}

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get media access logs: %w", err)
	}
	defer rows.Close()

	var logs []models.MediaAccessLog
	for rows.Next() {
		var log models.MediaAccessLog
		var deviceInfoJSON, locationJSON sql.NullString
		var playbackDurationSeconds sql.NullInt64

		err := rows.Scan(
			&log.ID, &log.UserID, &log.MediaID, &log.Action, &deviceInfoJSON,
			&locationJSON, &log.IPAddress, &log.UserAgent, &playbackDurationSeconds,
			&log.AccessTime)

		if err != nil {
			return nil, fmt.Errorf("failed to scan media access log: %w", err)
		}

		if deviceInfoJSON.Valid {
			var deviceInfo models.DeviceInfo
			if err := json.Unmarshal([]byte(deviceInfoJSON.String), &deviceInfo); err == nil {
				log.DeviceInfo = &deviceInfo
			}
		}

		if locationJSON.Valid {
			var location models.Location
			if err := json.Unmarshal([]byte(locationJSON.String), &location); err == nil {
				log.Location = &location
			}
		}

		if playbackDurationSeconds.Valid {
			duration := time.Duration(playbackDurationSeconds.Int64) * time.Second
			log.PlaybackDuration = &duration
		}

		logs = append(logs, log)
	}

	return logs, nil
}

func (r *AnalyticsRepository) GetUserMediaAccessLogs(userID int, startDate, endDate time.Time) ([]models.MediaAccessLog, error) {
	query := `
		SELECT id, user_id, media_id, action, device_info, location, ip_address,
			   user_agent, playback_duration, access_time
		FROM media_access_logs
		WHERE user_id = ? AND access_time BETWEEN ? AND ?
		ORDER BY access_time DESC
	`

	rows, err := r.db.Query(query, userID, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get user media access logs: %w", err)
	}
	defer rows.Close()

	return r.scanMediaAccessLogs(rows)
}

func (r *AnalyticsRepository) GetUserEvents(userID int, startDate, endDate time.Time) ([]models.AnalyticsEvent, error) {
	query := `
		SELECT id, user_id, event_type, event_category, data, device_info, location,
			   ip_address, user_agent, timestamp
		FROM analytics_events
		WHERE user_id = ? AND timestamp BETWEEN ? AND ?
		ORDER BY timestamp DESC
	`

	rows, err := r.db.Query(query, userID, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get user events: %w", err)
	}
	defer rows.Close()

	return r.scanAnalyticsEvents(rows)
}

func (r *AnalyticsRepository) GetTotalUsers() (int, error) {
	query := `SELECT COUNT(*) FROM users`
	var count int
	err := r.db.QueryRow(query).Scan(&count)
	return count, err
}

func (r *AnalyticsRepository) GetActiveUsers(startDate, endDate time.Time) (int, error) {
	query := `
		SELECT COUNT(DISTINCT user_id)
		FROM media_access_logs
		WHERE access_time BETWEEN ? AND ?
	`
	var count int
	err := r.db.QueryRow(query, startDate, endDate).Scan(&count)
	return count, err
}

func (r *AnalyticsRepository) GetTotalMediaAccesses(startDate, endDate time.Time) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM media_access_logs
		WHERE access_time BETWEEN ? AND ?
	`
	var count int
	err := r.db.QueryRow(query, startDate, endDate).Scan(&count)
	return count, err
}

func (r *AnalyticsRepository) GetTotalEvents(startDate, endDate time.Time) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM analytics_events
		WHERE timestamp BETWEEN ? AND ?
	`
	var count int
	err := r.db.QueryRow(query, startDate, endDate).Scan(&count)
	return count, err
}

func (r *AnalyticsRepository) GetTopAccessedMedia(startDate, endDate time.Time, limit int) ([]models.MediaAccessCount, error) {
	query := `
		SELECT media_id, COUNT(*) as access_count
		FROM media_access_logs
		WHERE access_time BETWEEN ? AND ?
		GROUP BY media_id
		ORDER BY access_count DESC
		LIMIT ?
	`

	rows, err := r.db.Query(query, startDate, endDate, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get top accessed media: %w", err)
	}
	defer rows.Close()

	// Non-nil so an empty result serialises as [] rather than null.
	results := make([]models.MediaAccessCount, 0)
	for rows.Next() {
		var result models.MediaAccessCount
		err := rows.Scan(&result.MediaID, &result.AccessCount)
		if err != nil {
			return nil, fmt.Errorf("failed to scan media access count: %w", err)
		}
		results = append(results, result)
	}

	return results, nil
}

func (r *AnalyticsRepository) GetUserGrowthData(startDate, endDate time.Time) ([]models.UserGrowthPoint, error) {
	query := `
		SELECT DATE(created_at) as date, COUNT(*) as user_count
		FROM users
		WHERE created_at BETWEEN ? AND ?
		GROUP BY DATE(created_at)
		ORDER BY date ASC
	`

	rows, err := r.db.Query(query, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get user growth data: %w", err)
	}
	defer rows.Close()

	var results []models.UserGrowthPoint
	for rows.Next() {
		var result models.UserGrowthPoint
		var dateStr string
		err := rows.Scan(&dateStr, &result.UserCount)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user growth point: %w", err)
		}

		// Article XI §11.5: PostgreSQL returns DATE() as a full
		// RFC3339 timestamp ("2026-04-29T00:00:00Z") when scanned
		// into a string column, while SQLite returns a bare
		// "2026-04-29". Try the bare-date layout first (matches the
		// SELECT clause), then fall back to RFC3339 / RFC3339Nano
		// for PostgreSQL. Caught by FQA-API-278 in the 2026-04-29
		// real-binary bank verification (PostgreSQL deployment on
		// amber.local).
		date, err := parseDateFlexible(dateStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse date %q: %w", dateStr, err)
		}
		result.Date = date

		results = append(results, result)
	}

	return results, nil
}

// parseDateFlexible accepts a date string in any of the formats
// SQLite or PostgreSQL might emit for a DATE() projection:
// "2006-01-02" (SQLite), or RFC3339 with zero time component
// (PostgreSQL). Returns a normalized time.Time at UTC midnight.
func parseDateFlexible(s string) (time.Time, error) {
	for _, layout := range []string{"2006-01-02", time.RFC3339, time.RFC3339Nano} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("no recognized date layout matches %q (tried 2006-01-02, RFC3339)", s)
}

func (r *AnalyticsRepository) GetSessionData(startDate, endDate time.Time) ([]models.SessionData, error) {
	durationExpr := "(julianday(MAX(last_activity_at)) - julianday(MIN(last_activity_at))) * 24 * 60 * 60"
	if r.db.Dialect().IsPostgres() {
		durationExpr = "EXTRACT(EPOCH FROM (MAX(last_activity_at) - MIN(last_activity_at)))"
	}

	query := fmt.Sprintf(`
		SELECT user_id,
			   MIN(last_activity_at) as session_start,
			   MAX(last_activity_at) as session_end,
			   %s as duration_seconds
		FROM user_sessions
		WHERE created_at BETWEEN ? AND ? AND is_active = 1
		GROUP BY user_id, DATE(created_at)
		ORDER BY session_start ASC
	`, durationExpr)

	rows, err := r.db.Query(query, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get session data: %w", err)
	}
	defer rows.Close()

	var results []models.SessionData
	for rows.Next() {
		var result models.SessionData
		var durationSeconds float64

		err := rows.Scan(&result.UserID, &result.StartTime, &result.EndTime, &durationSeconds)
		if err != nil {
			return nil, fmt.Errorf("failed to scan session data: %w", err)
		}

		result.Duration = time.Duration(durationSeconds) * time.Second
		results = append(results, result)
	}

	return results, nil
}

func (r *AnalyticsRepository) GetAllMediaAccessLogs(startDate, endDate time.Time) ([]models.MediaAccessLog, error) {
	query := `
		SELECT id, user_id, media_id, action, device_info, location, ip_address,
			   user_agent, playback_duration, access_time
		FROM media_access_logs
		WHERE access_time BETWEEN ? AND ?
		ORDER BY access_time DESC
	`

	rows, err := r.db.Query(query, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get all media access logs: %w", err)
	}
	defer rows.Close()

	return r.scanMediaAccessLogs(rows)
}

func (r *AnalyticsRepository) GetFileTypeData(startDate, endDate time.Time) (map[string]int, error) {
	query := `
		SELECT
			CASE
				WHEN LOWER(data) LIKE '%"file_type":"video%' THEN 'video'
				WHEN LOWER(data) LIKE '%"file_type":"audio%' THEN 'audio'
				WHEN LOWER(data) LIKE '%"file_type":"image%' THEN 'image'
				WHEN LOWER(data) LIKE '%"file_type":"document%' THEN 'document'
				ELSE 'other'
			END as file_type,
			COUNT(*) as count
		FROM analytics_events
		WHERE timestamp BETWEEN ? AND ? AND event_type = 'media_access'
		GROUP BY file_type
		ORDER BY count DESC
	`

	rows, err := r.db.Query(query, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get file type data: %w", err)
	}
	defer rows.Close()

	fileTypes := make(map[string]int)
	for rows.Next() {
		var fileType string
		var count int
		err := rows.Scan(&fileType, &count)
		if err != nil {
			return nil, fmt.Errorf("failed to scan file type data: %w", err)
		}
		fileTypes[fileType] = count
	}

	return fileTypes, nil
}

func (r *AnalyticsRepository) GetGeographicData(startDate, endDate time.Time) (map[string]interface{}, error) {
	query := `
		SELECT location, COUNT(*) as access_count
		FROM media_access_logs
		WHERE access_time BETWEEN ? AND ? AND location IS NOT NULL
		GROUP BY location
		ORDER BY access_count DESC
		LIMIT 100
	`

	rows, err := r.db.Query(query, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get geographic data: %w", err)
	}
	defer rows.Close()

	var locations []map[string]interface{}
	countryCount := make(map[string]int)

	for rows.Next() {
		var locationJSON string
		var accessCount int

		err := rows.Scan(&locationJSON, &accessCount)
		if err != nil {
			return nil, fmt.Errorf("failed to scan geographic data: %w", err)
		}

		var location models.Location
		if err := json.Unmarshal([]byte(locationJSON), &location); err == nil {
			locationData := map[string]interface{}{
				"latitude":     location.Latitude,
				"longitude":    location.Longitude,
				"access_count": accessCount,
			}

			if location.Country != nil {
				locationData["country"] = *location.Country
				countryCount[*location.Country] += accessCount
			}

			if location.City != nil {
				locationData["city"] = *location.City
			}

			locations = append(locations, locationData)
		}
	}

	return map[string]interface{}{
		"locations": locations,
		"countries": countryCount,
	}, nil
}

func (r *AnalyticsRepository) scanMediaAccessLogs(rows *sql.Rows) ([]models.MediaAccessLog, error) {
	var logs []models.MediaAccessLog

	for rows.Next() {
		var log models.MediaAccessLog
		var deviceInfoJSON, locationJSON sql.NullString
		var playbackDurationSeconds sql.NullInt64

		err := rows.Scan(
			&log.ID, &log.UserID, &log.MediaID, &log.Action, &deviceInfoJSON,
			&locationJSON, &log.IPAddress, &log.UserAgent, &playbackDurationSeconds,
			&log.AccessTime)

		if err != nil {
			return nil, fmt.Errorf("failed to scan media access log: %w", err)
		}

		if deviceInfoJSON.Valid {
			var deviceInfo models.DeviceInfo
			if err := json.Unmarshal([]byte(deviceInfoJSON.String), &deviceInfo); err == nil {
				log.DeviceInfo = &deviceInfo
			}
		}

		if locationJSON.Valid {
			var location models.Location
			if err := json.Unmarshal([]byte(locationJSON.String), &location); err == nil {
				log.Location = &location
			}
		}

		if playbackDurationSeconds.Valid {
			duration := time.Duration(playbackDurationSeconds.Int64) * time.Second
			log.PlaybackDuration = &duration
		}

		logs = append(logs, log)
	}

	return logs, nil
}

func (r *AnalyticsRepository) scanAnalyticsEvents(rows *sql.Rows) ([]models.AnalyticsEvent, error) {
	var events []models.AnalyticsEvent

	for rows.Next() {
		var event models.AnalyticsEvent
		var deviceInfoJSON, locationJSON sql.NullString

		err := rows.Scan(
			&event.ID, &event.UserID, &event.EventType, &event.EventCategory,
			&event.Data, &deviceInfoJSON, &locationJSON, &event.IPAddress,
			&event.UserAgent, &event.Timestamp)

		if err != nil {
			return nil, fmt.Errorf("failed to scan analytics event: %w", err)
		}

		if deviceInfoJSON.Valid {
			var deviceInfo models.DeviceInfo
			if err := json.Unmarshal([]byte(deviceInfoJSON.String), &deviceInfo); err == nil {
				event.DeviceInfo = &deviceInfo
			}
		}

		if locationJSON.Valid {
			var location models.Location
			if err := json.Unmarshal([]byte(locationJSON.String), &location); err == nil {
				event.Location = &location
			}
		}

		events = append(events, event)
	}

	return events, nil
}
