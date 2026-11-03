package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"catalogizer/database"
	"catalogizer/models"
)

type UserRepository struct {
	db *database.DB
}

func NewUserRepository(db *database.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(user *models.User) (int, error) {
	query := `
		INSERT INTO users (username, email, password_hash, salt, role_id, first_name, last_name,
						  display_name, avatar_url, time_zone, language, is_active, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	now := time.Now()
	id, err := r.db.InsertReturningID(context.Background(), query,
		user.Username, user.Email, user.PasswordHash, user.Salt, user.RoleID,
		user.FirstName, user.LastName, user.DisplayName, user.AvatarURL,
		user.TimeZone, user.Language, user.IsActive, now, now)

	if err != nil {
		return 0, fmt.Errorf("failed to create user: %w", err)
	}

	return int(id), nil
}

func (r *UserRepository) GetByID(id int) (*models.User, error) {
	query := `
		SELECT id, username, email, password_hash, salt, role_id, first_name, last_name,
			   display_name, avatar_url, time_zone, language, is_active, is_locked,
			   locked_until, failed_login_attempts, last_login_at, last_login_ip,
			   created_at, updated_at, settings
		FROM users WHERE id = ?
	`

	user := &models.User{}
	var settings sql.NullString

	err := r.db.QueryRow(query, id).Scan(
		&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.Salt,
		&user.RoleID, &user.FirstName, &user.LastName, &user.DisplayName,
		&user.AvatarURL, &user.TimeZone, &user.Language, &user.IsActive,
		&user.IsLocked, &user.LockedUntil, &user.FailedLoginAttempts,
		&user.LastLoginAt, &user.LastLoginIP, &user.CreatedAt, &user.UpdatedAt,
		&settings)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	if settings.Valid {
		user.Settings = settings.String
	}

	return user, nil
}

func (r *UserRepository) GetByUsername(username string) (*models.User, error) {
	query := `
		SELECT id, username, email, password_hash, salt, role_id, first_name, last_name,
			   display_name, avatar_url, time_zone, language, is_active, is_locked,
			   locked_until, failed_login_attempts, last_login_at, last_login_ip,
			   created_at, updated_at, settings
		FROM users WHERE username = ?
	`

	user := &models.User{}
	var settings sql.NullString

	err := r.db.QueryRow(query, username).Scan(
		&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.Salt,
		&user.RoleID, &user.FirstName, &user.LastName, &user.DisplayName,
		&user.AvatarURL, &user.TimeZone, &user.Language, &user.IsActive,
		&user.IsLocked, &user.LockedUntil, &user.FailedLoginAttempts,
		&user.LastLoginAt, &user.LastLoginIP, &user.CreatedAt, &user.UpdatedAt,
		&settings)

	if err != nil {
		return nil, err
	}

	if settings.Valid {
		user.Settings = settings.String
	}

	return user, nil
}

func (r *UserRepository) GetByEmail(email string) (*models.User, error) {
	query := `
		SELECT id, username, email, password_hash, salt, role_id, first_name, last_name,
			   display_name, avatar_url, time_zone, language, is_active, is_locked,
			   locked_until, failed_login_attempts, last_login_at, last_login_ip,
			   created_at, updated_at, settings
		FROM users WHERE email = ?
	`

	user := &models.User{}
	var settings sql.NullString

	err := r.db.QueryRow(query, email).Scan(
		&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.Salt,
		&user.RoleID, &user.FirstName, &user.LastName, &user.DisplayName,
		&user.AvatarURL, &user.TimeZone, &user.Language, &user.IsActive,
		&user.IsLocked, &user.LockedUntil, &user.FailedLoginAttempts,
		&user.LastLoginAt, &user.LastLoginIP, &user.CreatedAt, &user.UpdatedAt,
		&settings)

	if err != nil {
		return nil, err
	}

	if settings.Valid {
		user.Settings = settings.String
	}

	return user, nil
}

func (r *UserRepository) GetByUsernameOrEmail(usernameOrEmail string) (*models.User, error) {
	query := `
		SELECT id, username, email, password_hash, salt, role_id, first_name, last_name,
			   display_name, avatar_url, time_zone, language, is_active, is_locked,
			   locked_until, failed_login_attempts, last_login_at, last_login_ip,
			   created_at, updated_at, settings
		FROM users WHERE username = ? OR email = ?
	`

	user := &models.User{}
	var settings sql.NullString

	err := r.db.QueryRow(query, usernameOrEmail, usernameOrEmail).Scan(
		&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.Salt,
		&user.RoleID, &user.FirstName, &user.LastName, &user.DisplayName,
		&user.AvatarURL, &user.TimeZone, &user.Language, &user.IsActive,
		&user.IsLocked, &user.LockedUntil, &user.FailedLoginAttempts,
		&user.LastLoginAt, &user.LastLoginIP, &user.CreatedAt, &user.UpdatedAt,
		&settings)

	if err != nil {
		return nil, err
	}

	if settings.Valid {
		user.Settings = settings.String
	}

	return user, nil
}

func (r *UserRepository) Update(user *models.User) error {
	query := `
		UPDATE users SET
			username = ?, email = ?, first_name = ?, last_name = ?, display_name = ?,
			avatar_url = ?, time_zone = ?, language = ?, is_active = ?, settings = ?,
			role_id = ?, updated_at = ?
		WHERE id = ?
	`

	_, err := r.db.Exec(query,
		user.Username, user.Email, user.FirstName, user.LastName, user.DisplayName,
		user.AvatarURL, user.TimeZone, user.Language, user.IsActive, user.Settings,
		user.RoleID, time.Now(), user.ID)

	return err
}

func (r *UserRepository) UpdatePassword(userID int, passwordHash, salt string) error {
	query := `UPDATE users SET password_hash = ?, salt = ?, updated_at = ? WHERE id = ?`
	_, err := r.db.Exec(query, passwordHash, salt, time.Now(), userID)
	return err
}

func (r *UserRepository) UpdateLastLogin(userID int, ipAddress string) error {
	query := `UPDATE users SET last_login_at = ?, last_login_ip = ? WHERE id = ?`
	_, err := r.db.Exec(query, time.Now(), ipAddress, userID)
	return err
}

func (r *UserRepository) IncrementFailedLoginAttempts(userID int) error {
	query := `UPDATE users SET failed_login_attempts = failed_login_attempts + 1 WHERE id = ?`
	_, err := r.db.Exec(query, userID)
	return err
}

func (r *UserRepository) ResetFailedLoginAttempts(userID int) error {
	query := `UPDATE users SET failed_login_attempts = 0 WHERE id = ?`
	_, err := r.db.Exec(query, userID)
	return err
}

func (r *UserRepository) LockAccount(userID int, lockUntil time.Time) error {
	query := `UPDATE users SET is_locked = 1, locked_until = ? WHERE id = ?`
	_, err := r.db.Exec(query, lockUntil, userID)
	return err
}

func (r *UserRepository) UnlockAccount(userID int) error {
	query := `UPDATE users SET is_locked = 0, locked_until = NULL WHERE id = ?`
	_, err := r.db.Exec(query, userID)
	return err
}

func (r *UserRepository) Delete(id int) error {
	query := `DELETE FROM users WHERE id = ?`
	_, err := r.db.Exec(query, id)
	return err
}

func (r *UserRepository) List(limit, offset int) ([]models.User, error) {
	query := `
		SELECT id, username, email, password_hash, salt, role_id, first_name, last_name,
			   display_name, avatar_url, time_zone, language, is_active, is_locked,
			   locked_until, failed_login_attempts, last_login_at, last_login_ip,
			   created_at, updated_at, settings
		FROM users
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.Query(query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var user models.User
		var settings sql.NullString

		err := rows.Scan(
			&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.Salt,
			&user.RoleID, &user.FirstName, &user.LastName, &user.DisplayName,
			&user.AvatarURL, &user.TimeZone, &user.Language, &user.IsActive,
			&user.IsLocked, &user.LockedUntil, &user.FailedLoginAttempts,
			&user.LastLoginAt, &user.LastLoginIP, &user.CreatedAt, &user.UpdatedAt,
			&settings)

		if err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}

		if settings.Valid {
			user.Settings = settings.String
		}

		users = append(users, user)
	}

	return users, nil
}

func (r *UserRepository) Count() (int, error) {
	query := `SELECT COUNT(*) FROM users`
	var count int
	err := r.db.QueryRow(query).Scan(&count)
	return count, err
}

func (r *UserRepository) GetRole(roleID int) (*models.Role, error) {
	query := `
		SELECT id, name, description, permissions, is_system, created_at, updated_at
		FROM roles WHERE id = ?
	`

	role := &models.Role{}
	var permissionsJSON string

	err := r.db.QueryRow(query, roleID).Scan(
		&role.ID, &role.Name, &role.Description, &permissionsJSON,
		&role.IsSystem, &role.CreatedAt, &role.UpdatedAt)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("role not found")
		}
		return nil, fmt.Errorf("failed to get role: %w", err)
	}

	err = json.Unmarshal([]byte(permissionsJSON), &role.Permissions)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal permissions: %w", err)
	}

	return role, nil
}

func (r *UserRepository) CreateSession(session *models.UserSession) (int, error) {
	query := `
		INSERT INTO user_sessions (user_id, session_token, refresh_token, device_info,
								  ip_address, user_agent, is_active, expires_at, created_at, last_activity_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	deviceInfoJSON, err := json.Marshal(session.DeviceInfo)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal device info: %w", err)
	}

	id, err := r.db.InsertReturningID(context.Background(), query,
		session.UserID, session.SessionToken, session.RefreshToken, string(deviceInfoJSON),
		session.IPAddress, session.UserAgent, session.IsActive, session.ExpiresAt,
		session.CreatedAt, session.LastActivityAt)

	if err != nil {
		return 0, fmt.Errorf("failed to create session: %w", err)
	}

	return int(id), nil
}

func (r *UserRepository) GetSession(sessionID string) (*models.UserSession, error) {
	query := `
		SELECT id, user_id, session_token, refresh_token, device_info, ip_address,
			   user_agent, is_active, expires_at, created_at, last_activity_at
		FROM user_sessions WHERE id = ?
	`

	session := &models.UserSession{}
	var deviceInfoJSON string

	err := r.db.QueryRow(query, sessionID).Scan(
		&session.ID, &session.UserID, &session.SessionToken, &session.RefreshToken,
		&deviceInfoJSON, &session.IPAddress, &session.UserAgent, &session.IsActive,
		&session.ExpiresAt, &session.CreatedAt, &session.LastActivityAt)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("session not found")
		}
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	err = json.Unmarshal([]byte(deviceInfoJSON), &session.DeviceInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal device info: %w", err)
	}

	return session, nil
}

func (r *UserRepository) GetSessionByRefreshToken(refreshToken string) (*models.UserSession, error) {
	query := `
		SELECT id, user_id, session_token, refresh_token, device_info, ip_address,
			   user_agent, is_active, expires_at, created_at, last_activity_at
		FROM user_sessions WHERE refresh_token = ? AND is_active = 1
	`

	session := &models.UserSession{}
	var deviceInfoJSON string

	err := r.db.QueryRow(query, refreshToken).Scan(
		&session.ID, &session.UserID, &session.SessionToken, &session.RefreshToken,
		&deviceInfoJSON, &session.IPAddress, &session.UserAgent, &session.IsActive,
		&session.ExpiresAt, &session.CreatedAt, &session.LastActivityAt)

	if err != nil {
		return nil, err
	}

	err = json.Unmarshal([]byte(deviceInfoJSON), &session.DeviceInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal device info: %w", err)
	}

	return session, nil
}

func (r *UserRepository) UpdateSessionTokens(sessionID int, sessionToken, refreshToken string) error {
	query := `UPDATE user_sessions SET session_token = ?, refresh_token = ? WHERE id = ?`
	_, err := r.db.Exec(query, sessionToken, refreshToken, sessionID)
	return err
}

func (r *UserRepository) UpdateSessionTokensAndExpiry(sessionID int, sessionToken, refreshToken string, expiresAt time.Time) error {
	query := `UPDATE user_sessions SET session_token = ?, refresh_token = ?, expires_at = ? WHERE id = ?`
	_, err := r.db.Exec(query, sessionToken, refreshToken, expiresAt, sessionID)
	return err
}

func (r *UserRepository) UpdateSessionActivity(sessionID int) error {
	query := `UPDATE user_sessions SET last_activity_at = ? WHERE id = ?`
	_, err := r.db.Exec(query, time.Now(), sessionID)
	return err
}

func (r *UserRepository) DeactivateSession(sessionID int) error {
	query := `UPDATE user_sessions SET is_active = 0 WHERE id = ?`
	_, err := r.db.Exec(query, sessionID)
	return err
}

func (r *UserRepository) DeactivateAllUserSessions(userID int) error {
	query := `UPDATE user_sessions SET is_active = 0 WHERE user_id = ?`
	_, err := r.db.Exec(query, userID)
	return err
}

func (r *UserRepository) GetActiveUserSessions(userID int) ([]models.UserSession, error) {
	query := `
		SELECT id, user_id, session_token, refresh_token, device_info, ip_address,
			   user_agent, is_active, expires_at, created_at, last_activity_at
		FROM user_sessions
		WHERE user_id = ? AND is_active = 1 AND expires_at > ?
		ORDER BY last_activity_at DESC
	`

	rows, err := r.db.Query(query, userID, time.Now())
	if err != nil {
		return nil, fmt.Errorf("failed to get active sessions: %w", err)
	}
	defer rows.Close()

	var sessions []models.UserSession
	for rows.Next() {
		var session models.UserSession
		var deviceInfoJSON string

		err := rows.Scan(
			&session.ID, &session.UserID, &session.SessionToken, &session.RefreshToken,
			&deviceInfoJSON, &session.IPAddress, &session.UserAgent, &session.IsActive,
			&session.ExpiresAt, &session.CreatedAt, &session.LastActivityAt)

		if err != nil {
			return nil, fmt.Errorf("failed to scan session: %w", err)
		}

		err = json.Unmarshal([]byte(deviceInfoJSON), &session.DeviceInfo)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal device info: %w", err)
		}

		sessions = append(sessions, session)
	}

	return sessions, nil
}

func (r *UserRepository) CleanupExpiredSessions() error {
	query := `DELETE FROM user_sessions WHERE expires_at < ? OR (is_active = 0 AND created_at < ?)`
	cutoff := time.Now().Add(-30 * 24 * time.Hour) // Remove inactive sessions older than 30 days
	_, err := r.db.Exec(query, time.Now(), cutoff)
	return err
}

func (r *UserRepository) CreateRole(role *models.Role) (int, error) {
	query := `
		INSERT INTO roles (name, description, permissions, is_system, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`

	permissionsJSON, err := json.Marshal(role.Permissions)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal permissions: %w", err)
	}

	now := time.Now()
	id, err := r.db.InsertReturningID(context.Background(), query, role.Name, role.Description, string(permissionsJSON),
		role.IsSystem, now, now)

	if err != nil {
		return 0, fmt.Errorf("failed to create role: %w", err)
	}

	return int(id), nil
}

func (r *UserRepository) UpdateRole(role *models.Role) error {
	query := `
		UPDATE roles SET name = ?, description = ?, permissions = ?, updated_at = ?
		WHERE id = ? AND is_system = 0
	`

	permissionsJSON, err := json.Marshal(role.Permissions)
	if err != nil {
		return fmt.Errorf("failed to marshal permissions: %w", err)
	}

	result, err := r.db.Exec(query, role.Name, role.Description, string(permissionsJSON),
		time.Now(), role.ID)

	if err != nil {
		return fmt.Errorf("failed to update role: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return errors.New("role not found or is system role")
	}

	return nil
}

func (r *UserRepository) DeleteRole(roleID int) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	var userCount int
	err = tx.QueryRow("SELECT COUNT(*) FROM users WHERE role_id = ?", roleID).Scan(&userCount)
	if err != nil {
		return fmt.Errorf("failed to check role usage: %w", err)
	}

	if userCount > 0 {
		return errors.New("cannot delete role that is assigned to users")
	}

	result, err := tx.Exec("DELETE FROM roles WHERE id = ? AND is_system = 0", roleID)
	if err != nil {
		return fmt.Errorf("failed to delete role: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return errors.New("role not found or is system role")
	}

	return tx.Commit()
}

func (r *UserRepository) ListRoles() ([]models.Role, error) {
	query := `
		SELECT id, name, description, permissions, is_system, created_at, updated_at
		FROM roles ORDER BY is_system DESC, name ASC
	`

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to list roles: %w", err)
	}
	defer rows.Close()

	var roles []models.Role
	for rows.Next() {
		var role models.Role
		var permissionsJSON string

		err := rows.Scan(&role.ID, &role.Name, &role.Description, &permissionsJSON,
			&role.IsSystem, &role.CreatedAt, &role.UpdatedAt)

		if err != nil {
			return nil, fmt.Errorf("failed to scan role: %w", err)
		}

		err = json.Unmarshal([]byte(permissionsJSON), &role.Permissions)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal permissions: %w", err)
		}

		roles = append(roles, role)
	}

	return roles, nil
}
