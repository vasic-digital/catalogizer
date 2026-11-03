package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"catalogizer/models"
)

// RoleUserServiceInterface defines interface for user repository operations
type RoleUserServiceInterface interface {
	CreateRole(role *models.Role) (int, error)
	GetRole(roleID int) (*models.Role, error)
	UpdateRole(role *models.Role) error
	DeleteRole(roleID int) error
	ListRoles() ([]models.Role, error)
}

// RoleAuthServiceInterface defines interface for authentication service operations
type RoleAuthServiceInterface interface {
	CheckPermission(userID int, permission string) (bool, error)
	GetCurrentUser(token string) (*models.User, error)
}

type RoleHandler struct {
	userRepo    RoleUserServiceInterface
	authService RoleAuthServiceInterface
}

func NewRoleHandler(userRepo RoleUserServiceInterface, authService RoleAuthServiceInterface) *RoleHandler {
	return &RoleHandler{
		userRepo:    userRepo,
		authService: authService,
	}
}

func (h *RoleHandler) CreateRole(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	currentUser, err := h.getCurrentUser(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	hasPermission, err := h.authService.CheckPermission(currentUser.ID, models.PermissionSystemAdmin)
	if err != nil {
		http.Error(w, "Failed to check permissions", http.StatusInternalServerError)
		return
	}

	if !hasPermission {
		http.Error(w, "Insufficient permissions", http.StatusForbidden)
		return
	}

	var req models.CreateRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	role := &models.Role{
		Name:        req.Name,
		Description: req.Description,
		Permissions: req.Permissions,
		IsSystem:    false,
	}

	id, err := h.userRepo.CreateRole(role)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			http.Error(w, "Role name already exists", http.StatusConflict)
			return
		}
		http.Error(w, "Failed to create role", http.StatusInternalServerError)
		return
	}

	role.ID = id

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(role)
}

func (h *RoleHandler) GetRole(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	currentUser, err := h.getCurrentUser(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	hasPermission, err := h.authService.CheckPermission(currentUser.ID, models.PermissionSystemAdmin)
	if err != nil {
		http.Error(w, "Failed to check permissions", http.StatusInternalServerError)
		return
	}

	if !hasPermission {
		http.Error(w, "Insufficient permissions", http.StatusForbidden)
		return
	}

	roleIDStr := routedID(r, "roles")
	roleID, err := strconv.Atoi(roleIDStr)
	if err != nil {
		http.Error(w, "Invalid role ID", http.StatusBadRequest)
		return
	}

	role, err := h.userRepo.GetRole(roleID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, "Role not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Failed to get role", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(role)
}

func (h *RoleHandler) UpdateRole(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	currentUser, err := h.getCurrentUser(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	hasPermission, err := h.authService.CheckPermission(currentUser.ID, models.PermissionSystemAdmin)
	if err != nil {
		http.Error(w, "Failed to check permissions", http.StatusInternalServerError)
		return
	}

	if !hasPermission {
		http.Error(w, "Insufficient permissions", http.StatusForbidden)
		return
	}

	roleIDStr := routedID(r, "roles")
	roleID, err := strconv.Atoi(roleIDStr)
	if err != nil {
		http.Error(w, "Invalid role ID", http.StatusBadRequest)
		return
	}

	var req models.UpdateRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Name == nil {
		http.Error(w, "Role name is required", http.StatusBadRequest)
		return
	}

	role := &models.Role{
		ID:          roleID,
		Name:        *req.Name,
		Description: req.Description,
		Permissions: req.Permissions,
	}

	err = h.userRepo.UpdateRole(role)
	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "system role") {
			http.Error(w, "Role not found or is system role", http.StatusNotFound)
			return
		}
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			http.Error(w, "Role name already exists", http.StatusConflict)
			return
		}
		http.Error(w, "Failed to update role", http.StatusInternalServerError)
		return
	}

	updatedRole, err := h.userRepo.GetRole(roleID)
	if err != nil {
		http.Error(w, "Failed to get updated role", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(updatedRole)
}

func (h *RoleHandler) DeleteRole(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	currentUser, err := h.getCurrentUser(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	hasPermission, err := h.authService.CheckPermission(currentUser.ID, models.PermissionSystemAdmin)
	if err != nil {
		http.Error(w, "Failed to check permissions", http.StatusInternalServerError)
		return
	}

	if !hasPermission {
		http.Error(w, "Insufficient permissions", http.StatusForbidden)
		return
	}

	roleIDStr := routedID(r, "roles")
	roleID, err := strconv.Atoi(roleIDStr)
	if err != nil {
		http.Error(w, "Invalid role ID", http.StatusBadRequest)
		return
	}

	err = h.userRepo.DeleteRole(roleID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "system role") {
			http.Error(w, "Role not found or is system role", http.StatusNotFound)
			return
		}
		if strings.Contains(err.Error(), "assigned to users") {
			http.Error(w, "Cannot delete role that is assigned to users", http.StatusConflict)
			return
		}
		http.Error(w, "Failed to delete role", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *RoleHandler) ListRoles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	currentUser, err := h.getCurrentUser(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	hasPermission, err := h.authService.CheckPermission(currentUser.ID, models.PermissionSystemAdmin)
	if err != nil {
		http.Error(w, "Failed to check permissions", http.StatusInternalServerError)
		return
	}

	if !hasPermission {
		http.Error(w, "Insufficient permissions", http.StatusForbidden)
		return
	}

	roles, err := h.userRepo.ListRoles()
	if err != nil {
		http.Error(w, "Failed to list roles", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(roles)
}

func (h *RoleHandler) GetPermissions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	currentUser, err := h.getCurrentUser(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	hasPermission, err := h.authService.CheckPermission(currentUser.ID, models.PermissionSystemAdmin)
	if err != nil {
		http.Error(w, "Failed to check permissions", http.StatusInternalServerError)
		return
	}

	if !hasPermission {
		http.Error(w, "Insufficient permissions", http.StatusForbidden)
		return
	}

	permissions := map[string]interface{}{
		"user_management": map[string]string{
			"create_user":  models.PermissionUserCreate,
			"view_user":    models.PermissionUserView,
			"edit_user":    models.PermissionUserUpdate,
			"delete_user":  models.PermissionUserDelete,
			"manage_users": models.PermissionUserManage,
		},
		"role_management": map[string]string{
			"view_roles":   models.PermissionSystemAdmin,
			"manage_roles": models.PermissionSystemAdmin,
		},
		"media_management": map[string]string{
			"view_media":   models.PermissionMediaView,
			"upload_media": models.PermissionMediaUpload,
			"edit_media":   models.PermissionMediaEdit,
			"delete_media": models.PermissionMediaDelete,
		},
		"share_management": map[string]string{
			"view_shares":   models.PermissionShareView,
			"create_shares": models.PermissionShareCreate,
			"edit_shares":   models.PermissionShareEdit,
			"delete_shares": models.PermissionShareDelete,
		},
		// storage.manage is deliberately not offered: no route is gated by it (storage, scan,
		// SMB, copy and sync-endpoint routes are admin only), so granting it would change nothing.
		"system": map[string]string{
			"system_admin":    models.PermissionSystemAdmin,
			"view_analytics":  models.PermissionAnalyticsView,
			"export_data":     models.PermissionAnalyticsExport,
			"manage_settings": models.PermissionSystemConfig,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(permissions)
}

func (h *RoleHandler) getCurrentUser(r *http.Request) (*models.User, error) {
	token := extractToken(r)
	if token == "" {
		return nil, models.ErrUnauthorized
	}

	return h.authService.GetCurrentUser(token)
}
