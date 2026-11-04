package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"catalogizer/models"
	"catalogizer/repository"
	"catalogizer/services"
)

// UserServiceInterface defines interface for user repository operations
type UserServiceInterface interface {
	Create(user *models.User) (int, error)
	GetByID(id int) (*models.User, error)
	Update(user *models.User) error
	Delete(id int) error
	List(limit, offset int) ([]models.User, error)
	GetRole(roleID int) (*models.Role, error)
	Count() (int, error)
	// DeactivateAllUserSessions is part of the interface (not an optional
	// extra) so a repository that cannot revoke sessions fails to COMPILE
	// instead of silently leaving old tokens valid after a role change.
	DeactivateAllUserSessions(userID int) error
}

// UserAuthServiceInterface defines interface for authentication service operations
type UserAuthServiceInterface interface {
	CheckPermission(userID int, permission string) (bool, error)
	GetCurrentUser(token string) (*models.User, error)
	HashPassword(password string) (string, error)
	ValidatePassword(password string) error
	GenerateSecureToken(length int) (string, error)
	ResetPassword(userID int, newPassword string) error
	LockAccount(userID int, lockUntil time.Time) error
	UnlockAccount(userID int) error
}

type UserHandler struct {
	userRepo    UserServiceInterface
	authService UserAuthServiceInterface
}

func NewUserHandler(userRepo UserServiceInterface, authService UserAuthServiceInterface) *UserHandler {
	return &UserHandler{
		userRepo:    userRepo,
		authService: authService,
	}
}

func (h *UserHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	currentUser, err := h.getCurrentUser(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	hasPermission, err := h.authService.CheckPermission(currentUser.ID, models.PermissionUserCreate)
	if err != nil {
		http.Error(w, "Failed to check permissions", http.StatusInternalServerError)
		return
	}

	if !hasPermission {
		http.Error(w, "Insufficient permissions", http.StatusForbidden)
		return
	}

	var req models.CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Article XI §11.5: validate required fields before bouncing the
	// payload into userRepo.Create. The previous flow let an empty
	// email / role_id / username through, the repository's Create
	// method blew up against the NOT NULL constraint, and the handler
	// returned a generic "Failed to create user" 500 with no
	// diagnostic. Caught by FQA-API-244 and FQA-API-248 in the
	// 2026-04-29 real-binary bank verification.
	missing := []string{}
	if req.Username == "" {
		missing = append(missing, "username")
	}
	if req.Email == "" {
		missing = append(missing, "email")
	}
	if req.Password == "" {
		missing = append(missing, "password")
	}
	if req.RoleID == 0 {
		missing = append(missing, "role_id")
	}
	if req.RoleID < 0 {
		http.Error(w, "role_id must be a positive integer", http.StatusBadRequest)
		return
	}
	if len(missing) > 0 {
		http.Error(w,
			`{"error":"missing required fields","details":"required: `+strings.Join(missing, ", ")+`"}`,
			http.StatusBadRequest)
		return
	}

	if !h.allowedToAssignRole(w, currentUser, req.RoleID) {
		return
	}

	if err := h.authService.ValidatePassword(req.Password); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	salt, err := h.authService.GenerateSecureToken(16)
	if err != nil {
		http.Error(w, "Failed to generate salt", http.StatusInternalServerError)
		return
	}

	passwordHash, err := services.HashPasswordWithSalt(req.Password, salt)
	if err != nil {
		http.Error(w, "Failed to hash password", http.StatusInternalServerError)
		return
	}

	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	user := &models.User{
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: passwordHash,
		Salt:         salt,
		RoleID:       req.RoleID,
		FirstName:    req.FirstName,
		LastName:     req.LastName,
		DisplayName:  req.DisplayName,
		TimeZone:     req.TimeZone,
		Language:     req.Language,
		IsActive:     isActive,
	}

	id, err := h.userRepo.Create(user)
	if err != nil {
		// Article XI §11.5: detect uniqueness-constraint violation
		// across both SQLite and PostgreSQL dialects. SQLite reports
		// "UNIQUE constraint failed: ...", PostgreSQL reports
		// "duplicate key value violates unique constraint ...". Before
		// this fix, PG-backed deployments silently bucketed every
		// duplicate-username/email retry as 500 "Failed to create user"
		// — exactly the empty-diagnostic 500 §11.9 forbids. Caught by
		// FQA-API-244 / FQA-API-248 in the 2026-04-29 real-binary bank
		// verification.
		msg := err.Error()
		if strings.Contains(msg, "UNIQUE constraint failed") ||
			strings.Contains(msg, "duplicate key value") ||
			strings.Contains(msg, "violates unique constraint") {
			http.Error(w,
				`{"error":"Username or email already exists","details":"`+strings.ReplaceAll(msg, `"`, `'`)+`"}`,
				http.StatusConflict)
			return
		}
		http.Error(w,
			`{"error":"Failed to create user","details":"`+strings.ReplaceAll(msg, `"`, `'`)+`"}`,
			http.StatusInternalServerError)
		return
	}

	user.ID = id
	user.PasswordHash = ""
	user.Salt = ""

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(user)
}

func (h *UserHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	currentUser, err := h.getCurrentUser(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	userIDStr := routedID(r, "users")
	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	if currentUser.ID != userID {
		hasPermission, err := h.authService.CheckPermission(currentUser.ID, models.PermissionUserView)
		if err != nil {
			http.Error(w, "Failed to check permissions", http.StatusInternalServerError)
			return
		}

		if !hasPermission {
			http.Error(w, "Insufficient permissions", http.StatusForbidden)
			return
		}
	}

	user, err := h.userRepo.GetByID(userID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, "User not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Failed to get user", http.StatusInternalServerError)
		return
	}

	role, err := h.userRepo.GetRole(user.RoleID)
	if err != nil {
		http.Error(w, "Failed to get user role", http.StatusInternalServerError)
		return
	}
	user.Role = role

	user.PasswordHash = ""
	user.Salt = ""

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

func (h *UserHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	currentUser, err := h.getCurrentUser(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	userIDStr := routedID(r, "users")
	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	if currentUser.ID != userID {
		hasPermission, err := h.authService.CheckPermission(currentUser.ID, models.PermissionUserUpdate)
		if err != nil {
			http.Error(w, "Failed to check permissions", http.StatusInternalServerError)
			return
		}

		if !hasPermission {
			http.Error(w, "Insufficient permissions", http.StatusForbidden)
			return
		}
	}

	var req models.UpdateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.RoleID != nil && *req.RoleID <= 0 {
		http.Error(w, "role_id must be a positive integer", http.StatusBadRequest)
		return
	}

	user, err := h.userRepo.GetByID(userID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, "User not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Failed to get user", http.StatusInternalServerError)
		return
	}

	if currentUser.ID != userID && !h.allowedToActOn(w, currentUser, userID) {
		return
	}

	oldRoleID, oldActive := user.RoleID, user.IsActive
	wasPending := user.IsRegistrationPending()
	decidedActive := false
	// privilegedChangeRequested: the caller is allowed to and did name role_id or
	// is_active. A retry after a failed post-update revoke sees no value change, so
	// revocation is keyed on the request too, not only on the diff.
	privilegedChangeRequested := false

	if req.Username != nil {
		user.Username = *req.Username
	}
	if req.Email != nil {
		user.Email = *req.Email
	}
	if req.FirstName != nil {
		user.FirstName = req.FirstName
	}
	if req.LastName != nil {
		user.LastName = req.LastName
	}
	if req.DisplayName != nil {
		user.DisplayName = req.DisplayName
	}
	if req.AvatarURL != nil {
		user.AvatarURL = req.AvatarURL
	}
	if req.TimeZone != nil {
		user.TimeZone = req.TimeZone
	}
	if req.Language != nil {
		user.Language = req.Language
	}

	if currentUser.ID != userID {
		hasAdminPermission, err := h.authService.CheckPermission(currentUser.ID, models.PermissionUserManage)
		if err != nil {
			http.Error(w, "Failed to check permissions", http.StatusInternalServerError)
			return
		}

		if hasAdminPermission {
			privilegedChangeRequested = req.RoleID != nil || req.IsActive != nil
			if req.RoleID != nil {
				if !h.allowedToAssignRole(w, currentUser, *req.RoleID) {
					return
				}
				user.RoleID = *req.RoleID
			}
			if req.IsActive != nil {
				user.IsActive = *req.IsActive
				// An explicit activation decision ends the pending-registration state: otherwise a
				// later deactivation would show the approved user as pending again (F4).
				user.ClearRegistrationPending()
				decidedActive = true
			}
		}
	}

	// Handle Settings update
	if req.Settings != nil {
		// Marshal settings to JSON string
		settingsJSON, err := json.Marshal(req.Settings)
		if err != nil {
			http.Error(w, "Failed to marshal settings", http.StatusInternalServerError)
			return
		}
		user.Settings = string(settingsJSON)
	}

	// A role or activation change must take effect now: access tokens carry
	// role_id and live for hours (RequireAdmin trusts the claim), so revoke every
	// session of the target first; the user logs in again and gets a fresh claim.
	// Revoking BEFORE the update fails closed: if revocation fails nothing is
	// changed, so a retry still revokes (after a persisted change the retry would
	// see no change and skip revocation).
	if privilegedChangeRequested || user.RoleID != oldRoleID || user.IsActive != oldActive {
		if err := revokeUserSessions(h.userRepo, user.ID); err != nil {
			http.Error(w, "Failed to revoke the user's sessions; user not updated", http.StatusInternalServerError)
			return
		}
	}

	// Compare-and-set on the role/activation the authorization decisions above
	// were made against (CAT-049 R3-M1): a concurrent role change must not be
	// overwritten by a save built from a stale read.
	if g, ok := h.userRepo.(guardedUserUpdater); ok {
		applied, gerr := g.UpdateGuarded(user, oldRoleID, oldActive)
		if gerr != nil {
			if errors.Is(gerr, repository.ErrLastAdmin) {
				http.Error(w, "The last administrator cannot be demoted or deactivated", http.StatusConflict)
				return
			}
			if strings.Contains(gerr.Error(), "UNIQUE constraint failed") {
				http.Error(w, "Username or email already exists", http.StatusConflict)
				return
			}
			http.Error(w, "Failed to update user", http.StatusInternalServerError)
			return
		}
		if !applied {
			http.Error(w, "User was modified concurrently; reload and retry", http.StatusConflict)
			return
		}
		err = nil
	} else {
		err = h.userRepo.Update(user)
	}
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			http.Error(w, "Username or email already exists", http.StatusConflict)
			return
		}
		http.Error(w, "Failed to update user", http.StatusInternalServerError)
		return
	}

	// Second revoke: a login that landed between the first revoke and the update
	// read the OLD row and holds a token with the old role claim. The user is
	// already updated, so a retry of this request is idempotent.
	if privilegedChangeRequested || user.RoleID != oldRoleID || user.IsActive != oldActive {
		if err := revokeUserSessions(h.userRepo, user.ID); err != nil {
			log.Printf("handlers: user %d updated but the post-update session revoke failed: %v", user.ID, err)
			http.Error(w, "User updated but revoking the user's sessions failed; repeat the request to finish revoking", http.StatusInternalServerError)
			return
		}
	}

	if decidedActive && wasPending {
		event := "registration_rejected"
		if user.IsActive {
			event = "registration_approved"
		}
		recordAuth(h.authService, user.ID, event, r.RemoteAddr, r.UserAgent(),
			"actor="+strconv.Itoa(currentUser.ID)+" is_active="+strconv.FormatBool(user.IsActive))
	}

	role, err := h.userRepo.GetRole(user.RoleID)
	if err != nil {
		http.Error(w, "Failed to get user role", http.StatusInternalServerError)
		return
	}
	user.Role = role

	user.PasswordHash = ""
	user.Salt = ""

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

func (h *UserHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	currentUser, err := h.getCurrentUser(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	hasPermission, err := h.authService.CheckPermission(currentUser.ID, models.PermissionUserDelete)
	if err != nil {
		http.Error(w, "Failed to check permissions", http.StatusInternalServerError)
		return
	}

	if !hasPermission {
		http.Error(w, "Insufficient permissions", http.StatusForbidden)
		return
	}

	userIDStr := routedID(r, "users")
	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	if !h.allowedToActOn(w, currentUser, userID) {
		return
	}

	if currentUser.ID == userID {
		http.Error(w, "Cannot delete your own account", http.StatusBadRequest)
		return
	}

	err = h.userRepo.Delete(userID)
	if err != nil {
		if errors.Is(err, repository.ErrLastAdmin) {
			http.Error(w, "The last administrator cannot be deleted", http.StatusConflict)
			return
		}
		http.Error(w, "Failed to delete user", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *UserHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	currentUser, err := h.getCurrentUser(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	hasPermission, err := h.authService.CheckPermission(currentUser.ID, models.PermissionUserView)
	if err != nil {
		http.Error(w, "Failed to check permissions", http.StatusInternalServerError)
		return
	}

	if !hasPermission {
		http.Error(w, "Insufficient permissions", http.StatusForbidden)
		return
	}

	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	limit := 50
	offset := 0

	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 && parsedLimit <= 100 {
			limit = parsedLimit
		}
	}

	if offsetStr != "" {
		if parsedOffset, err := strconv.Atoi(offsetStr); err == nil && parsedOffset >= 0 {
			offset = parsedOffset
		}
	}

	users, err := h.userRepo.List(limit, offset)
	if err != nil {
		http.Error(w, "Failed to list users", http.StatusInternalServerError)
		return
	}

	for i := range users {
		users[i].PasswordHash = ""
		users[i].Salt = ""

		role, err := h.userRepo.GetRole(users[i].RoleID)
		if err == nil {
			users[i].Role = role
		}
	}

	totalCount, err := h.userRepo.Count()
	if err != nil {
		http.Error(w, "Failed to get user count", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"users":       users,
		"total_count": totalCount,
		"limit":       limit,
		"offset":      offset,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *UserHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	currentUser, err := h.getCurrentUser(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	hasPermission, err := h.authService.CheckPermission(currentUser.ID, models.PermissionUserManage)
	if err != nil {
		http.Error(w, "Failed to check permissions", http.StatusInternalServerError)
		return
	}

	if !hasPermission {
		http.Error(w, "Insufficient permissions", http.StatusForbidden)
		return
	}

	userIDStr := routedID(r, "users")
	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	if !h.allowedToActOn(w, currentUser, userID) {
		return
	}

	var req struct {
		NewPassword string `json:"new_password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.authService.ValidatePassword(req.NewPassword); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err = h.authService.ResetPassword(userID, req.NewPassword)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Password reset successfully"})
}

func (h *UserHandler) LockAccount(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	currentUser, err := h.getCurrentUser(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	hasPermission, err := h.authService.CheckPermission(currentUser.ID, models.PermissionUserManage)
	if err != nil {
		http.Error(w, "Failed to check permissions", http.StatusInternalServerError)
		return
	}

	if !hasPermission {
		http.Error(w, "Insufficient permissions", http.StatusForbidden)
		return
	}

	userIDStr := routedID(r, "users")
	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	if !h.allowedToActOn(w, currentUser, userID) {
		return
	}

	if currentUser.ID == userID {
		http.Error(w, "Cannot lock your own account", http.StatusBadRequest)
		return
	}

	var req struct {
		LockUntil string `json:"lock_until"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	lockUntil, err := parseTime(req.LockUntil)
	if err != nil {
		http.Error(w, "Invalid lock_until format", http.StatusBadRequest)
		return
	}

	err = h.authService.LockAccount(userID, lockUntil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Account locked successfully"})
}

func (h *UserHandler) UnlockAccount(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	currentUser, err := h.getCurrentUser(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	hasPermission, err := h.authService.CheckPermission(currentUser.ID, models.PermissionUserManage)
	if err != nil {
		http.Error(w, "Failed to check permissions", http.StatusInternalServerError)
		return
	}

	if !hasPermission {
		http.Error(w, "Insufficient permissions", http.StatusForbidden)
		return
	}

	userIDStr := routedID(r, "users")
	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	if !h.allowedToActOn(w, currentUser, userID) {
		return
	}

	err = h.authService.UnlockAccount(userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Account unlocked successfully"})
}

func (h *UserHandler) UpdateUserSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut && r.Method != http.MethodPatch {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	currentUser, err := h.getCurrentUser(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	userIDStr := routedID(r, "users")
	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	// Users can only update their own settings, or admins can update any user's settings
	if currentUser.ID != userID {
		hasPermission, err := h.authService.CheckPermission(currentUser.ID, models.PermissionUserManage)
		if err != nil {
			http.Error(w, "Failed to check permissions", http.StatusInternalServerError)
			return
		}

		if !hasPermission {
			http.Error(w, "Insufficient permissions", http.StatusForbidden)
			return
		}
	}

	var req struct {
		Settings map[string]interface{} `json:"settings"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	user, err := h.userRepo.GetByID(userID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, "User not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Failed to get user", http.StatusInternalServerError)
		return
	}

	// Marshal settings to JSON string
	settingsJSON, err := json.Marshal(req.Settings)
	if err != nil {
		http.Error(w, "Failed to marshal settings", http.StatusInternalServerError)
		return
	}
	user.Settings = string(settingsJSON)

	err = h.userRepo.Update(user)
	if err != nil {
		http.Error(w, "Failed to update user settings", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Settings updated successfully"})
}

func (h *UserHandler) getCurrentUser(r *http.Request) (*models.User, error) {
	token := extractToken(r)
	if token == "" {
		return nil, models.ErrUnauthorized
	}

	return h.authService.GetCurrentUser(token)
}

func parseTime(timeStr string) (time.Time, error) {
	return time.Parse(time.RFC3339, timeStr)
}

// roleIsAdmin reports whether a role carries system administration rights.
func roleIsAdmin(role *models.Role) bool {
	// HasPermission honours "*" and prefix wildcards such as "system.*".
	return role != nil && role.Permissions.HasPermission(models.PermissionSystemAdmin)
}

// callerIsAdmin reports whether the caller holds system.admin. On error it has
// already written a 500 and returns ok=false.
func (h *UserHandler) callerIsAdmin(w http.ResponseWriter, caller *models.User) (admin, ok bool) {
	admin, err := h.authService.CheckPermission(caller.ID, models.PermissionSystemAdmin)
	if err != nil {
		http.Error(w, "Failed to check permissions", http.StatusInternalServerError)
		return false, false
	}
	return admin, true
}

// allowedToActOn enforces: only an admin may act on an admin account
// (reset/lock/unlock/delete/update). Fails closed (403) when the target's role
// cannot be resolved. It writes the error response itself and returns false
// when the action must stop.
func (h *UserHandler) allowedToActOn(w http.ResponseWriter, caller *models.User, targetID int) bool {
	admin, ok := h.callerIsAdmin(w, caller)
	if !ok {
		return false
	}
	target, err := h.userRepo.GetByID(targetID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			// The same 404 for an administrator as for anyone else: acting on a user that does not exist is not a success
			// (delete answered 204 and unlock 200 for an unknown id before this check applied to administrators too).
			http.Error(w, "User not found", http.StatusNotFound)
			return false
		}
		http.Error(w, "Failed to get user", http.StatusInternalServerError)
		return false
	}
	if admin {
		return true
	}
	role, err := h.userRepo.GetRole(target.RoleID)
	if err != nil || roleIsAdmin(role) {
		http.Error(w, "Insufficient permissions", http.StatusForbidden)
		return false
	}
	return true
}

// allowedToAssignRole: a non-admin may not assign an admin role (or a role
// that cannot be resolved), and may not assign a role carrying any permission
// the caller's own role does not already cover (CAT-050 R3-M2: no escalation
// among non-admin roles; a caller can only hand out what it holds).
func (h *UserHandler) allowedToAssignRole(w http.ResponseWriter, caller *models.User, roleID int) bool {
	admin, ok := h.callerIsAdmin(w, caller)
	if !ok {
		return false
	}
	if admin {
		return true
	}
	role, err := h.userRepo.GetRole(roleID)
	if err != nil || roleIsAdmin(role) {
		http.Error(w, "Insufficient permissions", http.StatusForbidden)
		return false
	}
	// Every permission of the target role must be one the caller itself holds
	// (resolved by the same CheckPermission the other guards use). A wildcard
	// permission in the target ("media.*", "*") is covered only by an equal
	// permission or a wildcard spanning it, never by individual grants.
	//
	// The standard user role (what self-registration already hands out) is the
	// baseline and stays assignable by any user manager: it grants nothing a new
	// account does not get anyway.
	if roleID == defaultUserRoleID {
		return true
	}
	for _, p := range role.Permissions {
		held, err := h.authService.CheckPermission(caller.ID, p)
		if err != nil {
			http.Error(w, "Failed to check permissions", http.StatusInternalServerError)
			return false
		}
		if !held {
			http.Error(w, "Insufficient permissions", http.StatusForbidden)
			return false
		}
	}
	return true
}

// defaultUserRoleID is the seeded standard user role (see RegisterGin).
const defaultUserRoleID = 2

// guardedUserUpdater is implemented by the production user repository.
type guardedUserUpdater interface {
	UpdateGuarded(user *models.User, expectedRoleID int, expectedActive bool) (bool, error)
}

// sessionRevoker is the revocation capability. UserServiceInterface and
// AdminUserRepoInterface both embed it by method, so every repository handed to
// these handlers must implement it: revocation cannot fail open.
type sessionRevoker interface {
	DeactivateAllUserSessions(userID int) error
}

// revokeUserSessions deactivates every session of userID. Callers abort with an
// error response when it fails (fail closed).
func revokeUserSessions(repo sessionRevoker, userID int) error {
	return repo.DeactivateAllUserSessions(userID)
}
