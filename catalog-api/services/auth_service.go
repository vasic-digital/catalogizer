package services

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"catalogizer/models"
	"catalogizer/repository"
)

// AuthService handles authentication and authorization
type AuthService struct {
	userRepo   *repository.UserRepository
	jwtSecret  []byte
	jwtExpiry  time.Duration
	refreshExp time.Duration
}

// NewAuthService creates a new authentication service
func NewAuthService(userRepo *repository.UserRepository, jwtSecret string) *AuthService {
	return &AuthService{
		userRepo:   userRepo,
		jwtSecret:  []byte(jwtSecret),
		jwtExpiry:  24 * time.Hour,     // 24 hours
		refreshExp: 7 * 24 * time.Hour, // 7 days
	}
}

// JWTClaims represents the claims in our JWT tokens
type JWTClaims struct {
	UserID    int    `json:"user_id"`
	Username  string `json:"username"`
	RoleID    int    `json:"role_id"`
	SessionID string `json:"session_id"`
	jwt.RegisteredClaims
}

// AuthResult represents the result of authentication
type AuthResult struct {
	User         *models.User `json:"user"`
	SessionToken string       `json:"session_token"`
	RefreshToken string       `json:"refresh_token"`
	ExpiresAt    time.Time    `json:"expires_at"`
}

// Login authenticates a user and creates a session
func (s *AuthService) Login(req models.LoginRequest, ipAddress string, userAgent string) (*AuthResult, error) {
	// Find user by username or email
	user, err := s.userRepo.GetByUsernameOrEmail(req.Username)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("invalid credentials")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// Check if user can login
	if !user.CanLogin() {
		if user.IsLocked {
			return nil, errors.New("account is temporarily locked")
		}
		return nil, errors.New("account is disabled")
	}

	// Verify password
	if !s.verifyPassword(req.Password, user.Salt, user.PasswordHash) {
		// Increment failed login attempts
		s.userRepo.IncrementFailedLoginAttempts(user.ID)
		return nil, errors.New("invalid credentials")
	}

	// Reset failed login attempts on successful login
	s.userRepo.ResetFailedLoginAttempts(user.ID)

	// Create session
	session, err := s.createSession(user, req.DeviceInfo, ipAddress, userAgent, req.RememberMe)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	// Update last login information
	s.userRepo.UpdateLastLogin(user.ID, ipAddress)

	// Load user role
	role, err := s.userRepo.GetRole(user.RoleID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user role: %w", err)
	}
	user.Role = role

	// Generate JWT token
	token, err := s.generateJWT(user, session.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	// Generate refresh token
	refreshToken, err := s.generateRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	// Update session with tokens
	err = s.userRepo.UpdateSessionTokens(session.ID, token, refreshToken)
	if err != nil {
		return nil, fmt.Errorf("failed to update session tokens: %w", err)
	}

	return &AuthResult{
		User:         user,
		SessionToken: token,
		RefreshToken: refreshToken,
		ExpiresAt:    session.ExpiresAt,
	}, nil
}

// RefreshToken refreshes an authentication token
func (s *AuthService) RefreshToken(refreshToken string) (*AuthResult, error) {
	// Find session by refresh token
	session, err := s.userRepo.GetSessionByRefreshToken(refreshToken)
	if err != nil {
		return nil, errors.New("invalid refresh token")
	}

	// Check if session is still valid
	if !session.IsActive || session.ExpiresAt.Before(time.Now()) {
		return nil, errors.New("session expired")
	}

	// Get user
	user, err := s.userRepo.GetByID(session.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// Check if user can still login
	if !user.CanLogin() {
		// Deactivate session
		s.userRepo.DeactivateSession(session.ID)
		return nil, errors.New("account is disabled")
	}

	// Load user role
	role, err := s.userRepo.GetRole(user.RoleID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user role: %w", err)
	}
	user.Role = role

	// Generate new JWT token
	newToken, err := s.generateJWT(user, session.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	// Generate new refresh token
	newRefreshToken, err := s.generateRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	// Update session with new tokens and extend expiry
	newExpiry := time.Now().Add(s.refreshExp)
	err = s.userRepo.UpdateSessionTokensAndExpiry(session.ID, newToken, newRefreshToken, newExpiry)
	if err != nil {
		return nil, fmt.Errorf("failed to update session: %w", err)
	}

	// Update last activity
	s.userRepo.UpdateSessionActivity(session.ID)

	return &AuthResult{
		User:         user,
		SessionToken: newToken,
		RefreshToken: newRefreshToken,
		ExpiresAt:    newExpiry,
	}, nil
}

// Logout terminates a user session
func (s *AuthService) Logout(sessionToken string) error {
	claims, err := s.validateToken(sessionToken)
	if err != nil {
		return err
	}

	sessionID, _ := strconv.Atoi(claims.SessionID)
	return s.userRepo.DeactivateSession(sessionID)
}

// LogoutAll terminates all sessions for a user
func (s *AuthService) LogoutAll(userID int) error {
	return s.userRepo.DeactivateAllUserSessions(userID)
}

// ValidateToken validates a JWT token and returns the claims
func (s *AuthService) ValidateToken(tokenString string) (*JWTClaims, error) {
	return s.validateToken(tokenString)
}

// GetCurrentUser gets the current user from a JWT token
func (s *AuthService) GetCurrentUser(tokenString string) (*models.User, error) {
	claims, err := s.validateToken(tokenString)
	if err != nil {
		return nil, err
	}

	// Check if session is still active
	session, err := s.userRepo.GetSession(claims.SessionID)
	if err != nil {
		return nil, errors.New("session not found")
	}

	if !session.IsActive || session.ExpiresAt.Before(time.Now()) {
		return nil, errors.New("session expired")
	}

	// Get user
	user, err := s.userRepo.GetByID(claims.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// Load user role
	role, err := s.userRepo.GetRole(user.RoleID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user role: %w", err)
	}
	user.Role = role

	// Update session activity
	sessionID, _ := strconv.Atoi(claims.SessionID)
	s.userRepo.UpdateSessionActivity(sessionID)

	return user, nil
}

// ChangePassword changes a user's password
func (s *AuthService) ChangePassword(userID int, currentPassword, newPassword string) error {
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	// Verify current password
	if !s.verifyPassword(currentPassword, user.Salt, user.PasswordHash) {
		return errors.New("current password is incorrect")
	}

	// Generate new salt and hash
	salt, err := s.generateSalt()
	if err != nil {
		return fmt.Errorf("failed to generate salt: %w", err)
	}

	passwordHash, err := s.hashPassword(newPassword, salt)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	// Update password
	err = s.userRepo.UpdatePassword(userID, passwordHash, salt)
	if err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	// Deactivate all sessions except current one (force re-login on other devices)
	// This is a security best practice when password changes
	return s.userRepo.DeactivateAllUserSessions(userID)
}

// ResetPassword resets a user's password (admin function)
func (s *AuthService) ResetPassword(userID int, newPassword string) error {
	// Generate new salt and hash
	salt, err := s.generateSalt()
	if err != nil {
		return fmt.Errorf("failed to generate salt: %w", err)
	}

	passwordHash, err := s.hashPassword(newPassword, salt)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	// Update password
	err = s.userRepo.UpdatePassword(userID, passwordHash, salt)
	if err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	// Deactivate all sessions (force re-login)
	return s.userRepo.DeactivateAllUserSessions(userID)
}

// CheckPermission checks if a user has a specific permission
func (s *AuthService) CheckPermission(userID int, permission string) (bool, error) {
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return false, fmt.Errorf("failed to get user: %w", err)
	}

	role, err := s.userRepo.GetRole(user.RoleID)
	if err != nil {
		return false, fmt.Errorf("failed to get user role: %w", err)
	}

	return role.Permissions.HasPermission(permission), nil
}

// Private helper methods

func (s *AuthService) createSession(user *models.User, deviceInfo models.DeviceInfo, ipAddress, userAgent string, rememberMe bool) (*models.UserSession, error) {
	sessionToken, err := s.generateSessionToken()
	if err != nil {
		return nil, err
	}

	// Set expiry based on remember me
	expiry := time.Now().Add(s.jwtExpiry)
	if rememberMe {
		expiry = time.Now().Add(s.refreshExp)
	}

	session := &models.UserSession{
		UserID:         user.ID,
		SessionToken:   sessionToken,
		DeviceInfo:     deviceInfo,
		IPAddress:      &ipAddress,
		UserAgent:      &userAgent,
		IsActive:       true,
		ExpiresAt:      expiry,
		CreatedAt:      time.Now(),
		LastActivityAt: time.Now(),
	}

	id, err := s.userRepo.CreateSession(session)
	if err != nil {
		return nil, err
	}

	session.ID = id
	return session, nil
}

func (s *AuthService) generateJWT(user *models.User, sessionID int) (string, error) {
	now := time.Now()
	claims := JWTClaims{
		UserID:    user.ID,
		Username:  user.Username,
		RoleID:    user.RoleID,
		SessionID: fmt.Sprintf("%d", sessionID),
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(s.jwtExpiry)),
			IssuedAt:  jwt.NewNumericDate(now),
			Issuer:    "catalogizer",
			Subject:   fmt.Sprintf("%d", user.ID),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtSecret)
}

func (s *AuthService) validateToken(tokenString string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.jwtSecret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))

	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*JWTClaims)
	if !ok {
		// This should never happen since we passed &JWTClaims{} to ParseWithClaims
		return nil, errors.New("invalid token claims")
	}
	return claims, nil
}

func (s *AuthService) generateSessionToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func (s *AuthService) generateRefreshToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func (s *AuthService) generateSalt() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func (s *AuthService) hashPassword(password, salt string) (string, error) {
	// Combine password and salt
	combined := password + salt

	// Use bcrypt for additional security
	hash, err := bcrypt.GenerateFromPassword([]byte(combined), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}

	return string(hash), nil
}

func (s *AuthService) verifyPassword(password, salt, hash string) bool {
	// Combine password and salt
	combined := password + salt

	// Compare with bcrypt
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(combined))
	return err == nil
}

// Session management methods

// GetActiveSessions returns all active sessions for a user
func (s *AuthService) GetActiveSessions(userID int) ([]models.UserSession, error) {
	return s.userRepo.GetActiveUserSessions(userID)
}

// DeactivateSession deactivates a specific session
func (s *AuthService) DeactivateSession(sessionID int) error {
	return s.userRepo.DeactivateSession(sessionID)
}

// CleanupExpiredSessions removes expired sessions from the database
func (s *AuthService) CleanupExpiredSessions() error {
	return s.userRepo.CleanupExpiredSessions()
}

// UpdateSessionActivity updates the last activity time for a session
func (s *AuthService) UpdateSessionActivity(sessionID int) error {
	return s.userRepo.UpdateSessionActivity(sessionID)
}

// Security utilities

// GenerateSecureToken generates a cryptographically secure random token
func (s *AuthService) GenerateSecureToken(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// HashData creates a SHA-256 hash of the given data
func (s *AuthService) HashData(data string) string {
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

// HashPasswordForUser hashes a password with a generated salt (public method for registration)
func (s *AuthService) HashPasswordForUser(password string) (passwordHash string, saltStr string, err error) {
	// Generate salt
	saltStr, err = s.generateSalt()
	if err != nil {
		return "", "", err
	}

	// Hash password with salt
	hash, err := s.hashPassword(password, saltStr)
	if err != nil {
		return "", "", err
	}

	return hash, saltStr, nil
}

// ValidatePassword checks if a password meets security requirements
func (s *AuthService) ValidatePassword(password string) error {
	if len(password) < 8 {
		return errors.New("password must be at least 8 characters long")
	}
	if len(password) > 128 {
		return errors.New("password must be at most 128 characters long")
	}
	// At least one uppercase letter
	uppercase := regexp.MustCompile(`[A-Z]`)
	if !uppercase.MatchString(password) {
		return errors.New("password must contain at least one uppercase letter")
	}
	// At least one lowercase letter
	lowercase := regexp.MustCompile(`[a-z]`)
	if !lowercase.MatchString(password) {
		return errors.New("password must contain at least one lowercase letter")
	}
	// At least one digit
	digit := regexp.MustCompile(`[0-9]`)
	if !digit.MatchString(password) {
		return errors.New("password must contain at least one digit")
	}
	// At least one special character (non-alphanumeric, printable)
	special := regexp.MustCompile(`[^a-zA-Z0-9]`)
	if !special.MatchString(password) {
		return errors.New("password must contain at least one special character")
	}
	return nil
}

// Account security methods

// LockAccount locks a user account until the specified time
func (s *AuthService) LockAccount(userID int, lockUntil time.Time) error {
	return s.userRepo.LockAccount(userID, lockUntil)
}

// UnlockAccount unlocks a user account
func (s *AuthService) UnlockAccount(userID int) error {
	return s.userRepo.UnlockAccount(userID)
}

// CheckAccountLockout checks if an account should be locked due to failed attempts
func (s *AuthService) CheckAccountLockout(userID int) error {
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return err
	}

	// Lock account if too many failed attempts
	maxAttempts := 5 // This should be configurable
	if user.FailedLoginAttempts >= maxAttempts {
		lockDuration := 30 * time.Minute // This should be configurable
		lockUntil := time.Now().Add(lockDuration)
		return s.LockAccount(userID, lockUntil)
	}

	return nil
}
