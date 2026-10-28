package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"time"

	"catalogizer/database"

	jwtmod "digital.vasic.auth/pkg/jwt"
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

// AuthService handles authentication operations.
// Token generation delegates to digital.vasic.auth/pkg/jwt.Manager;
// validation keeps typed Claims parsing via ParseWithClaims.
type AuthService struct {
	db            *database.DB
	jwtSecret     []byte
	jwtMgr        *jwtmod.Manager // access-token manager (24h TTL)
	jwtRefreshMgr *jwtmod.Manager // refresh-token manager (7d TTL)
	logger        *zap.Logger
	tokenTTL      time.Duration
}

// NewAuthService creates a new authentication service backed by
// digital.vasic.auth/pkg/jwt for token generation.
func NewAuthService(db *database.DB, jwtSecret string, logger *zap.Logger) *AuthService {
	accessCfg := jwtmod.DefaultConfig(jwtSecret)
	accessCfg.Expiration = 24 * time.Hour

	refreshCfg := jwtmod.DefaultConfig(jwtSecret)
	refreshCfg.Expiration = 7 * 24 * time.Hour

	return &AuthService{
		db:            db,
		jwtSecret:     []byte(jwtSecret),
		jwtMgr:        jwtmod.NewManager(accessCfg),
		jwtRefreshMgr: jwtmod.NewManager(refreshCfg),
		logger:        logger,
		tokenTTL:      24 * time.Hour,
	}
}

// Initialize creates the authentication tables and default admin user
func (s *AuthService) Initialize() error {
	// Create tables
	if err := s.createTables(); err != nil {
		return fmt.Errorf("failed to create auth tables: %w", err)
	}

	// Create default admin user if none exists
	if err := s.createDefaultAdmin(); err != nil {
		return fmt.Errorf("failed to create default admin: %w", err)
	}

	s.logger.Info("Authentication system initialized")
	return nil
}

// createTables creates authentication-related tables.
// On PostgreSQL, tables are created by the migration system so this is a no-op.
func (s *AuthService) createTables() error {
	if s.db.Dialect().IsPostgres() {
		return nil // migrations handle PostgreSQL schema
	}
	schema := `
	-- Users table
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT NOT NULL UNIQUE,
		email TEXT NOT NULL UNIQUE,
		password_hash TEXT NOT NULL,
		first_name TEXT NOT NULL,
		last_name TEXT NOT NULL,
		role TEXT NOT NULL DEFAULT 'user',
		is_active BOOLEAN NOT NULL DEFAULT 1,
		last_login DATETIME,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- Roles table
	CREATE TABLE IF NOT EXISTS roles (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL UNIQUE,
		description TEXT,
		permissions TEXT, -- JSON array
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- Sessions table
	CREATE TABLE IF NOT EXISTS sessions (
		id TEXT PRIMARY KEY,
		user_id INTEGER NOT NULL,
		token TEXT NOT NULL UNIQUE,
		expires_at DATETIME NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		ip_address TEXT,
		user_agent TEXT,
		FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
	);

	-- Permissions table
	CREATE TABLE IF NOT EXISTS permissions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL UNIQUE,
		resource TEXT NOT NULL,
		action TEXT NOT NULL,
		description TEXT
	);

	-- User permissions (for custom permissions beyond role)
	CREATE TABLE IF NOT EXISTS user_permissions (
		user_id INTEGER NOT NULL,
		permission_id INTEGER NOT NULL,
		granted_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		granted_by INTEGER,
		PRIMARY KEY (user_id, permission_id),
		FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
		FOREIGN KEY (permission_id) REFERENCES permissions(id) ON DELETE CASCADE,
		FOREIGN KEY (granted_by) REFERENCES users(id)
	);

	-- Audit log for authentication events
	CREATE TABLE IF NOT EXISTS auth_audit_log (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER,
		event_type TEXT NOT NULL, -- login, logout, failed_login, password_change, etc.
		ip_address TEXT,
		user_agent TEXT,
		details TEXT, -- JSON
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (user_id) REFERENCES users(id)
	);

	-- Indexes
	CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
	CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
	CREATE INDEX IF NOT EXISTS idx_users_role ON users(role);
	CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id);
	CREATE INDEX IF NOT EXISTS idx_sessions_token ON sessions(token);
	CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at);
	CREATE INDEX IF NOT EXISTS idx_auth_audit_user_id ON auth_audit_log(user_id);
	CREATE INDEX IF NOT EXISTS idx_auth_audit_event_type ON auth_audit_log(event_type);
	CREATE INDEX IF NOT EXISTS idx_auth_audit_created_at ON auth_audit_log(created_at);

	-- Insert default roles
	INSERT OR IGNORE INTO roles (name, description, permissions) VALUES
	('admin', 'System Administrator', '["admin:system", "manage:users", "manage:roles", "read:media", "write:media", "delete:media", "read:catalog", "write:catalog", "delete:catalog", "trigger:analysis", "view:analysis", "view:logs", "access:api", "write:api"]'),
	('moderator', 'Content Moderator', '["read:media", "write:media", "read:catalog", "write:catalog", "trigger:analysis", "view:analysis", "access:api", "write:api"]'),
	('user', 'Regular User', '["read:media", "write:media", "read:catalog", "write:catalog", "view:analysis", "access:api"]'),
	('viewer', 'Read-only Viewer', '["read:media", "read:catalog", "view:analysis", "access:api"]');

	-- Insert default permissions
	INSERT OR IGNORE INTO permissions (name, resource, action, description) VALUES
	('read:media', 'media', 'read', 'View media items and metadata'),
	('write:media', 'media', 'write', 'Create and update media items'),
	('delete:media', 'media', 'delete', 'Delete media items'),
	('read:catalog', 'catalog', 'read', 'Browse file catalog'),
	('write:catalog', 'catalog', 'write', 'Modify file catalog'),
	('delete:catalog', 'catalog', 'delete', 'Delete from catalog'),
	('trigger:analysis', 'analysis', 'trigger', 'Start media analysis'),
	('view:analysis', 'analysis', 'view', 'View analysis results'),
	('manage:users', 'users', 'manage', 'Create, update, delete users'),
	('manage:roles', 'roles', 'manage', 'Create, update, delete roles'),
	('view:logs', 'logs', 'view', 'View system logs'),
	('admin:system', 'system', 'admin', 'Full system administration'),
	('access:api', 'api', 'access', 'Access API endpoints'),
	('write:api', 'api', 'write', 'Modify data via API');
	`

	_, err := s.db.Exec(schema)
	return err
}

// createDefaultAdmin creates a default admin user if none exists
func (s *AuthService) createDefaultAdmin() error {
	// Check if any admin users exist
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM users WHERE role = 'admin'").Scan(&count)
	if err != nil {
		return err
	}

	if count > 0 {
		s.logger.Info("Admin user already exists")
		return nil
	}

	// Read admin credentials from environment variables
	adminUsername := os.Getenv("ADMIN_USERNAME")
	if adminUsername == "" {
		adminUsername = "admin"
	}
	adminPassword := os.Getenv("ADMIN_PASSWORD")
	if adminPassword == "" {
		// Generate a cryptographically secure random password
		passwordBytes := make([]byte, 12)
		if _, err := rand.Read(passwordBytes); err != nil {
			return fmt.Errorf("failed to generate admin password: %w", err)
		}
		adminPassword = hex.EncodeToString(passwordBytes)
		s.logger.Warn("No ADMIN_PASSWORD set. Generated random password. Set ADMIN_PASSWORD environment variable for production.",
			zap.String("username", adminUsername),
			zap.String("password", adminPassword))
	} else if adminPassword == "admin123" {
		s.logger.Warn("Using default password 'admin123'. Change ADMIN_PASSWORD for production security.")
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(adminPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	query := `
		INSERT INTO users (username, email, password_hash, first_name, last_name, role, is_active)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	_, err = s.db.Exec(query, adminUsername, adminUsername+"@catalogizer.local", string(passwordHash), "System", "Administrator", "admin", 1)
	if err != nil {
		return err
	}

	s.logger.Info("Default admin user created",
		zap.String("username", adminUsername))

	return nil
}

// Login authenticates a user and returns a token
func (s *AuthService) Login(username, password, ipAddress, userAgent string) (*LoginResponse, error) {
	// Get user by username or email
	user, err := s.getUserByUsernameOrEmail(username)
	if err != nil {
		s.logAuthEvent(0, "failed_login", ipAddress, userAgent, fmt.Sprintf("user not found: %s", username))
		return nil, fmt.Errorf("invalid credentials")
	}

	// Check if user is active
	if !user.IsActive {
		s.logAuthEvent(user.ID, "failed_login_inactive", ipAddress, userAgent, "account disabled")
		return nil, fmt.Errorf("account is disabled")
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		s.logAuthEvent(user.ID, "failed_login", ipAddress, userAgent, "invalid password")
		return nil, fmt.Errorf("invalid credentials")
	}

	// Load user permissions
	user.Permissions = GetRolePermissions(user.Role)

	// Generate tokens
	accessToken, err := s.generateToken(user, "access")
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	refreshToken, err := s.generateToken(user, "refresh")
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	// Create session
	sessionID, err := s.createSession(user.ID, accessToken, ipAddress, userAgent)
	if err != nil {
		s.logger.Error("Failed to create session", zap.Error(err))
	}

	// Update last login
	_, err = s.db.Exec("UPDATE users SET last_login = ? WHERE id = ?", time.Now(), user.ID)
	if err != nil {
		s.logger.Error("Failed to update last login", zap.Error(err))
	}

	s.logAuthEvent(user.ID, "login_success", ipAddress, userAgent, fmt.Sprintf("session: %s", sessionID))

	return &LoginResponse{
		User:         *user,
		Token:        accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(s.tokenTTL.Seconds()),
	}, nil
}

// Logout invalidates a user session
func (s *AuthService) Logout(token string) error {
	// Delete session
	_, err := s.db.Exec("DELETE FROM sessions WHERE token = ?", token)
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	// Parse token to get user ID for logging
	claims, err := s.validateToken(token)
	if err == nil {
		s.logAuthEvent(claims.UserID, "logout", "", "", "")
	}

	return nil
}

// ValidateToken validates a JWT token and returns user information
func (s *AuthService) ValidateToken(tokenString string) (*User, error) {
	claims, err := s.validateToken(tokenString)
	if err != nil {
		return nil, err
	}

	// Check if session exists and is valid
	var sessionExists bool
	err = s.db.QueryRow("SELECT EXISTS(SELECT 1 FROM sessions WHERE token = ? AND expires_at > ?)",
		tokenString, time.Now()).Scan(&sessionExists)
	if err != nil || !sessionExists {
		return nil, fmt.Errorf("session expired or invalid")
	}

	// Get current user data
	user, err := s.GetUserByID(claims.UserID)
	if err != nil {
		return nil, err
	}

	return user, nil
}

// GetUserByID retrieves a user by ID
func (s *AuthService) GetUserByID(userID int64) (*User, error) {
	query := `
		SELECT id, username, email, password_hash, first_name, last_name, role, is_active, last_login, created_at, updated_at
		FROM users WHERE id = ?
	`

	user := &User{}
	err := s.db.QueryRow(query, userID).Scan(
		&user.ID, &user.Username, &user.Email, &user.PasswordHash,
		&user.FirstName, &user.LastName, &user.Role, &user.IsActive,
		&user.LastLogin, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	// Load permissions
	user.Permissions = GetRolePermissions(user.Role)

	return user, nil
}

// CreateUser creates a new user
func (s *AuthService) CreateUser(req *RegisterRequest) (*User, error) {
	// Check if username or email already exists
	var exists bool
	err := s.db.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE username = ? OR email = ?)",
		req.Username, req.Email).Scan(&exists)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, fmt.Errorf("username or email already exists")
	}

	// Hash password
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Insert user
	query := `
		INSERT INTO users (username, email, password_hash, first_name, last_name, role, is_active)
		VALUES (?, ?, ?, ?, ?, 'user', 1)
	`

	userID, err := s.db.InsertReturningID(context.Background(), query, req.Username, req.Email, string(passwordHash), req.FirstName, req.LastName)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return s.GetUserByID(userID)
}

// Helper methods

func (s *AuthService) getUserByUsernameOrEmail(identifier string) (*User, error) {
	query := `
		SELECT id, username, email, password_hash, first_name, last_name, role, is_active, last_login, created_at, updated_at
		FROM users WHERE username = ? OR email = ?
	`

	user := &User{}
	err := s.db.QueryRow(query, identifier, identifier).Scan(
		&user.ID, &user.Username, &user.Email, &user.PasswordHash,
		&user.FirstName, &user.LastName, &user.Role, &user.IsActive,
		&user.LastLogin, &user.CreatedAt, &user.UpdatedAt,
	)
	return user, err
}

// generateToken creates a signed JWT using digital.vasic.auth/pkg/jwt.Manager.
// Access tokens use the 24h manager; refresh tokens use the 7-day manager.
func (s *AuthService) generateToken(user *User, tokenType string) (string, error) {
	mgr := s.jwtMgr
	if tokenType == "refresh" {
		mgr = s.jwtRefreshMgr
	}

	return mgr.Create(map[string]interface{}{
		"user_id":     user.ID,
		"username":    user.Username,
		"role":        user.Role,
		"permissions": user.Permissions,
		"type":        tokenType,
	})
}

func (s *AuthService) validateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.jwtSecret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token")
}

func (s *AuthService) createSession(userID int64, token, ipAddress, userAgent string) (string, error) {
	sessionID, err := generateRandomString(32)
	if err != nil {
		return "", err
	}

	expiresAt := time.Now().Add(s.tokenTTL)

	query := `
		INSERT INTO sessions (id, user_id, token, expires_at, ip_address, user_agent)
		VALUES (?, ?, ?, ?, ?, ?)
	`

	_, err = s.db.Exec(query, sessionID, userID, token, expiresAt, ipAddress, userAgent)
	return sessionID, err
}

func (s *AuthService) logAuthEvent(userID int64, eventType, ipAddress, userAgent, details string) {
	query := `
		INSERT INTO auth_audit_log (user_id, event_type, ip_address, user_agent, details)
		VALUES (?, ?, ?, ?, ?)
	`

	_, err := s.db.Exec(query, userID, eventType, ipAddress, userAgent, details)
	if err != nil {
		s.logger.Error("Failed to log auth event", zap.Error(err))
	}
}

func generateRandomString(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// Additional methods for user management...

// validUserColumns is a whitelist of column names allowed in dynamic UPDATE queries.
// Defense-in-depth: even though setParts are built from hardcoded strings,
// this prevents any future refactoring from accidentally introducing SQL injection.
var validUserColumns = map[string]bool{
	"first_name": true,
	"last_name":  true,
	"email":      true,
	"role":       true,
	"is_active":  true,
	"updated_at": true,
}

// validateSetParts checks that every SET clause in setParts references only
// whitelisted column names. Returns an error if any column is not allowed.
func validateSetParts(setParts []string) error {
	for _, part := range setParts {
		// Each part is "column_name = ?"; extract the column name
		eqIdx := strings.Index(part, " = ?")
		if eqIdx < 0 {
			return fmt.Errorf("invalid SET clause format: %s", part)
		}
		col := strings.TrimSpace(part[:eqIdx])
		if !validUserColumns[col] {
			return fmt.Errorf("disallowed column in UPDATE: %s", col)
		}
	}
	return nil
}

// UpdateUser updates user information
func (s *AuthService) UpdateUser(userID int64, req *UpdateUserRequest) (*User, error) {
	var setParts []string
	var args []interface{}

	if req.FirstName != nil {
		setParts = append(setParts, "first_name = ?")
		args = append(args, *req.FirstName)
	}
	if req.LastName != nil {
		setParts = append(setParts, "last_name = ?")
		args = append(args, *req.LastName)
	}
	if req.Email != nil {
		setParts = append(setParts, "email = ?")
		args = append(args, *req.Email)
	}
	if req.Role != nil {
		setParts = append(setParts, "role = ?")
		args = append(args, *req.Role)
	}
	if req.IsActive != nil {
		setParts = append(setParts, "is_active = ?")
		args = append(args, *req.IsActive)
	}

	if len(setParts) == 0 {
		return s.GetUserByID(userID)
	}

	setParts = append(setParts, "updated_at = ?")
	args = append(args, time.Now())
	args = append(args, userID)

	// Defense-in-depth: validate all column names against whitelist
	if err := validateSetParts(setParts); err != nil {
		return nil, fmt.Errorf("security validation failed: %w", err)
	}

	query := fmt.Sprintf("UPDATE users SET %s WHERE id = ?", strings.Join(setParts, ", "))
	_, err := s.db.Exec(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to update user: %w", err)
	}

	return s.GetUserByID(userID)
}

// ChangePassword changes user password
func (s *AuthService) ChangePassword(userID int64, currentPassword, newPassword string) error {
	user, err := s.GetUserByID(userID)
	if err != nil {
		return err
	}

	// Verify current password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(currentPassword)); err != nil {
		return fmt.Errorf("current password is incorrect")
	}

	// Hash new password
	newPasswordHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash new password: %w", err)
	}

	// Update password
	_, err = s.db.Exec("UPDATE users SET password_hash = ?, updated_at = ? WHERE id = ?",
		string(newPasswordHash), time.Now(), userID)
	if err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	s.logAuthEvent(userID, "password_changed", "", "", "")
	return nil
}

// ListUsers returns paginated list of users
func (s *AuthService) ListUsers(limit, offset int) ([]User, int64, error) {
	// Get total count
	var total int64
	err := s.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	// Get users
	query := `
		SELECT id, username, email, first_name, last_name, role, is_active, last_login, created_at, updated_at
		FROM users
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`

	rows, err := s.db.Query(query, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var user User
		err := rows.Scan(
			&user.ID, &user.Username, &user.Email, &user.FirstName, &user.LastName,
			&user.Role, &user.IsActive, &user.LastLogin, &user.CreatedAt, &user.UpdatedAt,
		)
		if err != nil {
			continue
		}
		user.Permissions = GetRolePermissions(user.Role)
		users = append(users, user)
	}

	return users, total, nil
}
