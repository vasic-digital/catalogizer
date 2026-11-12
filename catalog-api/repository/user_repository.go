package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"catalogizer/database"
	"catalogizer/models"
)

// ErrLastAdmin is returned when a write would leave the system without any
// active, unlocked administrator (demotion, deactivation or deletion of the
// last one). The check and the write are one atomic statement (D9).
var ErrLastAdmin = errors.New("operation would remove the last active administrator")

type UserRepository struct {
	db *database.DB
}

func NewUserRepository(db *database.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(user *models.User) (int, error) {
	query := `
		INSERT INTO users (username, email, password_hash, salt, role_id, first_name, last_name,
						  display_name, avatar_url, time_zone, language, is_active, created_at, updated_at, settings)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	// An empty Settings keeps the column default ('{}').
	settings := user.Settings
	if settings == "" {
		settings = "{}"
	}

	now := time.Now()
	id, err := r.db.InsertReturningID(context.Background(), query,
		user.Username, user.Email, user.PasswordHash, user.Salt, user.RoleID,
		user.FirstName, user.LastName, user.DisplayName, user.AvatarURL,
		user.TimeZone, user.Language, user.IsActive, now, now, settings)

	if err != nil {
		return 0, fmt.Errorf("failed to create user: %w", err)
	}

	return int(id), nil
}

func (r *UserRepository) GetByID(id int) (*models.User, error) {
	query := `
		SELECT id, username, email, password_hash, salt, role_id, first_name, last_name,
			   display_name, avatar_url, time_zone, language, is_active, COALESCE(is_locked, '0'),
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
			   display_name, avatar_url, time_zone, language, is_active, COALESCE(is_locked, '0'),
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
			   display_name, avatar_url, time_zone, language, is_active, COALESCE(is_locked, '0'),
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
			   display_name, avatar_url, time_zone, language, is_active, COALESCE(is_locked, '0'),
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

// UpdateGuarded is a compare-and-set update: it saves user only while the
// stored role_id and is_active still equal expectedRoleID / expectedActive,
// the values the caller's authorization decision was based on. It closes the
// check-then-write window in handlers.UserHandler.UpdateUser (CAT-049 R3-M1):
// without it a save built from a stale read silently reverts a concurrent role
// or activation change (e.g. demotes an administrator just promoted). It
// reports whether a row was updated; (false, nil) means the guard no longer
// holds and the caller must re-read and decide again.
func (r *UserRepository) UpdateGuarded(user *models.User, expectedRoleID int, expectedActive bool) (bool, error) {
	adminIDs, err := r.adminRoleIDs()
	if err != nil {
		return false, err
	}
	isAdmin := func(id int) bool { return containsInt(adminIDs, id) }
	// The write removes an administrator when the stored row is an active
	// admin and the new state is not (role without admin rights, or inactive).
	// The lock columns are NOT written here: a suspension is written only by
	// SetSuspended (and LockAccount/UnlockAccount), never from a caller's
	// possibly stale read of the row.
	removesAdmin := expectedActive && isAdmin(expectedRoleID) && !(user.IsActive && isAdmin(user.RoleID))

	query := `
		UPDATE users SET
			username = ?, email = ?, first_name = ?, last_name = ?, display_name = ?,
			avatar_url = ?, time_zone = ?, language = ?, is_active = ?, settings = ?,
			role_id = ?, updated_at = ?
		WHERE id = ? AND role_id = ? AND is_active = ?`
	args := []interface{}{
		user.Username, user.Email, user.FirstName, user.LastName, user.DisplayName,
		user.AvatarURL, user.TimeZone, user.Language, user.IsActive, user.Settings,
		user.RoleID, time.Now(), user.ID, expectedRoleID, expectedActive}
	if removesAdmin {
		clause, cargs := otherActiveAdminsClause(user.ID, adminIDs)
		query += " AND " + clause
		args = append(args, cargs...)
	}

	exec := func(q Execer, qs string) (int64, error) {
		res, xerr := q.Exec(qs, args...)
		if xerr != nil {
			return 0, xerr
		}
		return res.RowsAffected()
	}
	var n int64
	if removesAdmin && r.db.Dialect().Type == database.DialectPostgres {
		// READ COMMITTED would let two transactions each see the other admin;
		// serialise admin-removing writes with a transaction-scoped advisory lock.
		tx, berr := r.db.Begin()
		if berr != nil {
			return false, berr
		}
		defer tx.Rollback()
		if _, lerr := tx.Exec(`SELECT pg_advisory_xact_lock(7340001)`); lerr != nil {
			return false, lerr
		}
		if n, err = exec(tx, r.db.Rewrite(query)); err != nil {
			return false, err
		}
		if err = tx.Commit(); err != nil {
			return false, err
		}
	} else if n, err = exec(r.db, query); err != nil {
		return false, err
	}
	if n > 0 {
		return true, nil
	}
	if removesAdmin {
		// Distinguish "the guard held the write back" from "the row changed".
		var rid int
		var act bool
		if qerr := r.db.QueryRow(`SELECT role_id, is_active FROM users WHERE id = ?`, user.ID).Scan(&rid, &act); qerr == nil &&
			rid == expectedRoleID && act == expectedActive {
			return false, ErrLastAdmin
		}
	}
	return false, nil
}

// Execer is the Exec surface shared by *database.DB and *sql.Tx.
type Execer interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
}

func containsInt(xs []int, v int) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}

// adminRoleIDs returns the ids of every role that grants system administration.
func (r *UserRepository) adminRoleIDs() ([]int, error) {
	roles, err := r.ListRoles()
	if err != nil {
		return nil, err
	}
	ids := []int{}
	for i := range roles {
		if roles[i].Permissions.HasPermission(models.PermissionSystemAdmin) {
			ids = append(ids, roles[i].ID)
		}
	}
	return ids, nil
}

// otherActiveAdminsClause is a SQL condition true when at least one active,
// unlocked administrator other than excludeID exists. Evaluated inside the
// same statement as the write it guards, so check and write are atomic.
func otherActiveAdminsClause(excludeID int, adminIDs []int) (string, []interface{}) {
	if len(adminIDs) == 0 {
		return "1 = 0", nil
	}
	ph := make([]string, len(adminIDs))
	args := []interface{}{excludeID, true, false, time.Now()}
	for i, id := range adminIDs {
		ph[i] = "?"
		args = append(args, id)
	}
	return "(SELECT COUNT(*) FROM users o WHERE o.id <> ? AND o.is_active = ? AND (o.is_locked IS NULL OR o.is_locked = ? OR (o.locked_until IS NOT NULL AND o.locked_until <= ?)) AND o.role_id IN (" +
		strings.Join(ph, ",") + ")) >= 1", args
}

// IsAdmin reports, from the database as it is NOW, whether the user is active
// and holds system administration rights through its CURRENT role's
// permissions ("system.admin", "*" or a covering prefix wildcard). It is the
// source of truth for the admin gate (middleware RequireAdmin): a role_id claim
// baked into an access token is never trusted for this decision (CAT-051 F4).
// Any lookup problem returns an error so the caller fails closed.
func (r *UserRepository) IsAdmin(ctx context.Context, userID int) (bool, error) {
	perms, active, err := r.livePermissions(ctx, userID)
	if err != nil || !active {
		return false, err
	}
	return perms.HasPermission(models.PermissionSystemAdmin), nil
}

// HasPermission reports whether the user is, right now, an active user whose
// CURRENT role carries permission, or system.admin (administrators pass every
// permission gate, as they pass RequireAdmin). Decided from the database on
// every call so a role that lost the permission stops working at once, whatever
// the token claims. An unknown user or dangling role is (false, nil); a failure
// to read is an error and callers fail closed on it.
func (r *UserRepository) HasPermission(ctx context.Context, userID int, permission string) (bool, error) {
	perms, active, err := r.livePermissions(ctx, userID)
	if err != nil || !active {
		return false, err
	}
	return perms.HasPermission(permission) || perms.HasPermission(models.PermissionSystemAdmin), nil
}

// livePermissions reads the user's current role permissions; active is false
// for an unknown, deactivated or role-less user.
func (r *UserRepository) livePermissions(ctx context.Context, userID int) (perms models.Permissions, active bool, err error) {
	var (
		isActive        bool
		permissionsJSON sql.NullString
		isLocked        bool
		lockedUntil     *time.Time
	)
	qerr := r.db.QueryRowContext(ctx, `
		SELECT u.is_active, ro.permissions, COALESCE(u.is_locked, '0'), u.locked_until
		FROM users u JOIN roles ro ON ro.id = u.role_id
		WHERE u.id = ?`, userID).Scan(&isActive, &permissionsJSON, &isLocked, &lockedUntil)
	if qerr != nil {
		if qerr == sql.ErrNoRows {
			return nil, false, nil // unknown user or dangling role
		}
		return nil, false, fmt.Errorf("failed to resolve permissions: %w", qerr)
	}
	// A locked account (permanent, or with an unexpired locked_until) is as unentitled as
	// a deactivated one, exactly as at login (User.CanLogin) - review round 1, F11.
	locked := (&models.User{IsLocked: isLocked, LockedUntil: lockedUntil}).IsAccountLocked()
	if !isActive || locked || !permissionsJSON.Valid {
		return nil, false, nil
	}
	if uerr := json.Unmarshal([]byte(permissionsJSON.String), &perms); uerr != nil {
		return nil, false, fmt.Errorf("failed to unmarshal permissions: %w", uerr)
	}
	return perms, true, nil
}

func (r *UserRepository) UpdatePassword(userID int, passwordHash, salt string) error {
	query := `UPDATE users SET password_hash = ?, salt = ?, updated_at = ? WHERE id = ?`
	_, err := r.db.Exec(query, passwordHash, salt, time.Now(), userID)
	return err
}

// UpdatePasswordIfHash is a compare-and-swap: it rewrites the hash only while
// the stored hash still equals oldHash, so a background rewrite (legacy hash
// upgrade) can never revert a password changed in the meantime. It reports
// whether a row was updated.
func (r *UserRepository) UpdatePasswordIfHash(userID int, newHash, salt, oldHash string) (bool, error) {
	query := `UPDATE users SET password_hash = ?, salt = ?, updated_at = ? WHERE id = ? AND password_hash = ?`
	res, err := r.db.Exec(query, newHash, salt, time.Now(), userID, oldHash)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
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

// ErrAccountSuspended is returned when an operation would lift (or the stored
// state is) a permanent administrative suspension. Only SetSuspended(id, false)
// (admin status "active") lifts a suspension.
var ErrAccountSuspended = errors.New("account is suspended")

// LockAccount applies a timed lock and revokes the user's sessions. A permanent suspension (is_locked with no
// locked_until) is never replaced by a timed lock: it is already stricter.
func (r *UserRepository) LockAccount(userID int, lockUntil time.Time) error {
	query := `UPDATE users SET is_locked = 1, locked_until = ? WHERE id = ? AND NOT (COALESCE(is_locked, '0') = ? AND locked_until IS NULL)`
	if _, err := r.db.Exec(query, lockUntil, userID, true); err != nil {
		return err
	}
	// A locked account keeps no session: JWT-only routes check the session row
	// alone. The lock is written first so no new session can be issued, then
	// every existing one is revoked.
	if err := r.DeactivateAllUserSessions(userID); err != nil {
		return fmt.Errorf("account locked but session revocation failed: %w", err)
	}
	return nil
}

// UnlockAccount lifts a timed lock. It refuses to lift a permanent suspension
// (ErrAccountSuspended); that is done only by SetSuspended(id, false).
func (r *UserRepository) UnlockAccount(userID int) error {
	query := `UPDATE users SET is_locked = 0, locked_until = NULL WHERE id = ? AND NOT (COALESCE(is_locked, '0') = ? AND locked_until IS NULL)`
	res, err := r.db.Exec(query, userID, true)
	if err != nil {
		return err
	}
	if n, nerr := res.RowsAffected(); nerr == nil && n == 0 {
		var locked bool
		var until *time.Time
		if qerr := r.db.QueryRow(`SELECT COALESCE(is_locked, '0'), locked_until FROM users WHERE id = ?`, userID).Scan(&locked, &until); qerr == nil && locked && until == nil {
			return ErrAccountSuspended
		}
	}
	return nil
}

// SetSuspended writes ONLY the lock columns: suspended=true sets a permanent
// suspension (is_locked, no locked_until), suspended=false clears the lock and
// any expiry. Suspending an active administrator is refused with ErrLastAdmin
// unless another active, effectively unlocked administrator exists; the check
// is part of the same statement, so two concurrent suspensions cannot remove
// the last two administrators.
func (r *UserRepository) SetSuspended(userID int, suspended bool) error {
	if !suspended {
		_, err := r.db.Exec(`UPDATE users SET is_locked = ?, locked_until = NULL, updated_at = ? WHERE id = ?`, false, time.Now(), userID)
		return err
	}
	adminIDs, err := r.adminRoleIDs()
	if err != nil {
		return err
	}
	query := `UPDATE users SET is_locked = ?, locked_until = NULL, updated_at = ? WHERE id = ?`
	args := []interface{}{true, time.Now(), userID}
	guarded := len(adminIDs) > 0
	if guarded {
		ph := make([]string, len(adminIDs))
		for i, id := range adminIDs {
			ph[i] = "?"
			args = append(args, id)
		}
		clause, cargs := otherActiveAdminsClause(userID, adminIDs)
		query += " AND (role_id NOT IN (" + strings.Join(ph, ",") + ") OR " + clause + ")"
		args = append(args, cargs...)
	}
	var n int64
	if guarded && r.db.Dialect().Type == database.DialectPostgres {
		tx, berr := r.db.Begin()
		if berr != nil {
			return berr
		}
		defer tx.Rollback()
		if _, lerr := tx.Exec(`SELECT pg_advisory_xact_lock(7340001)`); lerr != nil {
			return lerr
		}
		res, xerr := tx.Exec(r.db.Rewrite(query), args...)
		if xerr != nil {
			return xerr
		}
		n, _ = res.RowsAffected()
		if err = tx.Commit(); err != nil {
			return err
		}
	} else {
		res, xerr := r.db.Exec(query, args...)
		if xerr != nil {
			return xerr
		}
		n, _ = res.RowsAffected()
	}
	if n > 0 {
		return nil
	}
	var exists int
	if qerr := r.db.QueryRow(`SELECT COUNT(*) FROM users WHERE id = ?`, userID).Scan(&exists); qerr != nil {
		return qerr
	}
	if exists == 0 {
		return sql.ErrNoRows
	}
	return ErrLastAdmin
}

// Delete removes a user. Audit history is evidence and is retained: auth_audit_log rows and the user_permissions.granted_by
// attribution are detached (user_id / granted_by set NULL) in the same transaction, because both columns reference users(id)
// without ON DELETE and would otherwise make every user that ever logged in undeletable (HTTP 500).
func (r *UserRepository) Delete(id int) error {
	// Read before the transaction: a second pooled connection taken while the
	// transaction holds the only one would deadlock a single-connection pool.
	adminIDs, err := r.adminRoleIDs()
	if err != nil {
		return err
	}
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()
	// Detaching the audit rows must not make them anonymous: stamp who they belonged to into details first.
	var username string
	if err := tx.QueryRow(r.db.Rewrite(`SELECT username FROM users WHERE id = ?`), id).Scan(&username); err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("failed to read user before delete: %w", err)
	}
	stamp := fmt.Sprintf(" [former_user_id=%d username=%s]", id, username)
	if _, err := tx.Exec(r.db.Rewrite(`UPDATE auth_audit_log SET details = COALESCE(details, '') || ? WHERE user_id = ?`), stamp, id); err != nil {
		return err
	}
	for _, q := range []string{
		`UPDATE auth_audit_log SET user_id = NULL WHERE user_id = ?`,
		`UPDATE user_permissions SET granted_by = NULL WHERE granted_by = ?`,
	} {
		if _, err := tx.Exec(r.db.Rewrite(q), id); err != nil {
			return err
		}
	}
	// Deleting an active administrator is refused (atomically, in the same
	// statement) when no other active administrator would remain.
	del := `DELETE FROM users WHERE id = ?`
	delArgs := []interface{}{id}
	var rid int
	var act bool
	protected := false
	if qerr := tx.QueryRow(r.db.Rewrite(`SELECT role_id, is_active FROM users WHERE id = ?`), id).Scan(&rid, &act); qerr == nil &&
		act && containsInt(adminIDs, rid) {
		protected = true
		clause, cargs := otherActiveAdminsClause(id, adminIDs)
		del += " AND " + clause
		delArgs = append(delArgs, cargs...)
		if r.db.Dialect().Type == database.DialectPostgres {
			if _, lerr := tx.Exec(`SELECT pg_advisory_xact_lock(7340001)`); lerr != nil {
				return lerr
			}
		}
	}
	res, err := tx.Exec(r.db.Rewrite(del), delArgs...)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 && protected {
		return ErrLastAdmin // the deferred Rollback also undoes the audit detach
	}
	return tx.Commit()
}

func (r *UserRepository) List(limit, offset int) ([]models.User, error) {
	query := `
		SELECT id, username, email, password_hash, salt, role_id, first_name, last_name,
			   display_name, avatar_url, time_zone, language, is_active, COALESCE(is_locked, '0'),
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

	users := []models.User{}
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

	sessions := []models.UserSession{}
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

	roles := []models.Role{}
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
