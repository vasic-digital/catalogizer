package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"os"
	"net/http"
	"strconv"
	"strings"

	"catalogizer/models"
	"catalogizer/repository"
	"catalogizer/services"

	"github.com/gin-gonic/gin"
)

// Gin handler functions for auth

// LoginGin handles login request with gin
func (h *AuthHandler) LoginGin(c *gin.Context) {
	var req models.LoginRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	ipAddress := c.ClientIP()
	userAgent := c.GetHeader("User-Agent")

	result, err := h.authService.Login(req, ipAddress, userAgent)
	if err != nil {
		if errors.Is(err, services.ErrAccountPending) {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error(), "code": "account_pending_approval"})
			return
		}
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// RefreshTokenGin handles token refresh with gin
func (h *AuthHandler) RefreshTokenGin(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	result, err := h.authService.RefreshToken(req.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// LogoutGin handles logout with gin
func (h *AuthHandler) LogoutGin(c *gin.Context) {
	token := extractTokenFromGin(c)
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization token required"})
		return
	}

	err := h.authService.Logout(token)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
}

// GetCurrentUserGin returns current user info with gin
func (h *AuthHandler) GetCurrentUserGin(c *gin.Context) {
	token := extractTokenFromGin(c)
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization token required"})
		return
	}

	user, err := h.authService.GetCurrentUser(token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
		return
	}

	c.JSON(http.StatusOK, user)
}

// GetAuthStatusGin returns the authentication status of the current user.
// Used by the frontend to check if the stored token is still valid.
func (h *AuthHandler) GetAuthStatusGin(c *gin.Context) {
	token := extractTokenFromGin(c)
	if token == "" {
		c.JSON(http.StatusOK, gin.H{
			"authenticated": false,
		})
		return
	}

	user, err := h.authService.GetCurrentUser(token)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"authenticated": false,
			"error":         "invalid or expired token",
		})
		return
	}

	perms := []string{}
	if user.Role != nil {
		perms = []string(user.Role.Permissions)
	}

	c.JSON(http.StatusOK, gin.H{
		"authenticated": true,
		"user":          user,
		"permissions":   perms,
	})
}

// GetPermissionsGin returns the permissions for the current user.
func (h *AuthHandler) GetPermissionsGin(c *gin.Context) {
	token := extractTokenFromGin(c)
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization token required"})
		return
	}

	user, err := h.authService.GetCurrentUser(token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
		return
	}

	roleName := "User"
	perms := []string{}
	isAdmin := false
	if user.Role != nil {
		roleName = user.Role.Name
		perms = []string(user.Role.Permissions)
		isAdmin = user.Role.Permissions.HasPermission("*")
	}

	c.JSON(http.StatusOK, gin.H{
		"role":        roleName,
		"permissions": perms,
		"is_admin":    isAdmin,
	})
}

// Registration modes (REGISTRATION_MODE).
const (
	RegistrationModeApproval = "approval" // default: account is created pending, an admin approves it
	RegistrationModeOpen     = "open"     // explicit opt-in: account is active immediately
	RegistrationModeDisabled = "disabled" // self-registration is refused
)

// RegistrationMode resolves REGISTRATION_MODE. Unset or empty means approval.
// An unrecognised value fails closed to approval (and is logged), never to open.
func RegistrationMode() string {
	switch v := strings.ToLower(strings.TrimSpace(os.Getenv("REGISTRATION_MODE"))); v {
	case "", RegistrationModeApproval:
		return RegistrationModeApproval
	case RegistrationModeOpen:
		return RegistrationModeOpen
	case RegistrationModeDisabled:
		return RegistrationModeDisabled
	default:
		log.Printf("auth: unknown REGISTRATION_MODE %q, using %q", v, RegistrationModeApproval)
		return RegistrationModeApproval
	}
}

// RegisterGin handles user registration with gin.
// Default: the user is created inactive (pending administrator approval), at
// the minimal role, and no session is issued. See REGISTRATION_MODE.
func (h *AuthHandler) RegisterGin(c *gin.Context, userRepo *repository.UserRepository) {
	mode := RegistrationMode()
	if mode == RegistrationModeDisabled {
		c.JSON(http.StatusForbidden, gin.H{"error": "registration is disabled"})
		return
	}
	var req struct {
		Username  string `json:"username" binding:"required"`
		Email     string `json:"email" binding:"required,email"`
		Password  string `json:"password" binding:"required,min=8"`
		FirstName string `json:"first_name" binding:"required"`
		LastName  string `json:"last_name" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if user exists
	_, err := userRepo.GetByUsername(req.Username)
	if err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Username already exists"})
		return
	}

	_, err = userRepo.GetByEmail(req.Email)
	if err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Email already exists"})
		return
	}

	// Generate salt and hash password using auth service
	passwordHash, salt, err := h.authService.HashPasswordForUser(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	// Create user
	user := &models.User{
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: passwordHash,
		Salt:         salt,
		FirstName:    &req.FirstName,
		LastName:     &req.LastName,
		RoleID:       2,   // Default user role
		Role:         nil, // Will be loaded after creation
		IsActive:     mode == RegistrationModeOpen,
	}
	if mode != RegistrationModeOpen {
		user.Settings = models.RegistrationPendingSettings
	}

	userID, err := userRepo.Create(user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	// Fetch the created user with role
	createdUser, err := userRepo.GetByID(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve created user"})
		return
	}

	resp := struct {
		*models.User
		RegistrationStatus string `json:"registration_status"`
		Message            string `json:"message"`
	}{User: createdUser, RegistrationStatus: "active", Message: "Account created."}
	if !createdUser.IsActive {
		resp.RegistrationStatus = "pending"
		resp.Message = "Account created and pending administrator approval. You cannot log in until it is approved."
	}
	c.JSON(http.StatusCreated, resp)
}

// Helper function to extract token from gin context
func extractTokenFromGin(c *gin.Context) string {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		return ""
	}

	// Extract token from "Bearer <token>"
	parts := strings.Split(authHeader, " ")
	if len(parts) == 2 && parts[0] == "Bearer" {
		return parts[1]
	}

	return ""
}

// AuthHandler struct
type AuthHandler struct {
	authService *services.AuthService
}

func NewAuthHandler(authService *services.AuthService) *AuthHandler {
	return &AuthHandler{
		authService: authService,
	}
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req models.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	ipAddress := getClientIP(r)
	userAgent := r.Header.Get("User-Agent")

	result, err := h.authService.Login(req, ipAddress, userAgent)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (h *AuthHandler) RefreshToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		RefreshToken string `json:"refresh_token"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	result, err := h.authService.RefreshToken(req.RefreshToken)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	token := extractToken(r)
	if token == "" {
		http.Error(w, "Authorization token required", http.StatusUnauthorized)
		return
	}

	err := h.authService.Logout(token)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Logged out successfully"})
}

func (h *AuthHandler) LogoutAll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	user, err := h.getCurrentUser(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	err = h.authService.LogoutAll(user.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "All sessions terminated"})
}

func (h *AuthHandler) GetCurrentUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	user, err := h.getCurrentUser(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	user.PasswordHash = ""
	user.Salt = ""

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	user, err := h.getCurrentUser(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.authService.ValidatePassword(req.NewPassword); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err = h.authService.ChangePassword(user.ID, req.CurrentPassword, req.NewPassword)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Password changed successfully"})
}

func (h *AuthHandler) GetActiveSessions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	user, err := h.getCurrentUser(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	sessions, err := h.authService.GetActiveSessions(user.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	for i := range sessions {
		sessions[i].SessionToken = ""
		sessions[i].RefreshToken = nil
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sessions)
}

func (h *AuthHandler) DeactivateSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	_, err := h.getCurrentUser(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	sessionIDStr := r.URL.Query().Get("session_id")
	if sessionIDStr == "" {
		http.Error(w, "Session ID required", http.StatusBadRequest)
		return
	}

	sessionID, err := strconv.Atoi(sessionIDStr)
	if err != nil {
		http.Error(w, "Invalid session ID", http.StatusBadRequest)
		return
	}

	err = h.authService.DeactivateSession(sessionID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Session deactivated"})
}

func (h *AuthHandler) ValidateToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	token := extractToken(r)
	if token == "" {
		http.Error(w, "Authorization token required", http.StatusUnauthorized)
		return
	}

	claims, err := h.authService.ValidateToken(token)
	if err != nil {
		http.Error(w, "Invalid token", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"valid":      true,
		"user_id":    claims.UserID,
		"username":   claims.Username,
		"role_id":    claims.RoleID,
		"session_id": claims.SessionID,
		"expires_at": claims.ExpiresAt,
	})
}

func (h *AuthHandler) getCurrentUser(r *http.Request) (*models.User, error) {
	token := extractToken(r)
	if token == "" {
		return nil, models.ErrUnauthorized
	}

	return h.authService.GetCurrentUser(token)
}

func extractToken(r *http.Request) string {
	bearerToken := r.Header.Get("Authorization")
	if bearerToken == "" {
		return ""
	}

	parts := strings.Split(bearerToken, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		return ""
	}

	return parts[1]
}

func getClientIP(r *http.Request) string {
	ip := r.Header.Get("X-Real-IP")
	if ip == "" {
		ip = r.Header.Get("X-Forwarded-For")
		if ip != "" {
			parts := strings.Split(ip, ",")
			ip = strings.TrimSpace(parts[0])
		}
	}
	if ip == "" {
		ip = r.RemoteAddr
		if strings.Contains(ip, ":") {
			parts := strings.Split(ip, ":")
			if len(parts) > 0 {
				ip = parts[0]
			}
		}
	}
	return ip
}
