package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"catalogizer/models"

	"github.com/gorilla/mux"
)

// ConfigurationServiceInterface defines the interface for configuration service operations
type ConfigurationServiceInterface interface {
	GetWizardStep(stepID string) (*models.WizardStep, error)
	ValidateWizardStep(stepID string, data map[string]interface{}) (*models.ValidationResult, error)
	SaveWizardProgress(userID int, stepID string, data map[string]interface{}) error
	GetWizardProgress(userID int) (*models.WizardProgress, error)
	CompleteWizard(userID int, finalData map[string]interface{}) (*models.SystemConfiguration, error)
	GetConfiguration() (*models.Configuration, error)
	TestConfiguration(config *models.Configuration) (*models.ValidationResult, error)
	GetConfigurationSchema() (*models.ConfigurationSchema, error)
}

// AuthServiceInterface defines the interface for authentication service operations
type AuthServiceInterface interface {
	ValidatePermissions(userID int, resource, action string) bool
	ValidateJWT(tokenString string) (*models.User, error)
}

// ConfigurationAuthServiceInterface defines the interface for authentication service operations
type ConfigurationAuthServiceInterface interface {
	ValidateToken(tokenString string) (*models.User, error)
	CheckPermission(userID int, permission string) (bool, error)
}

type ConfigurationHandler struct {
	configurationService ConfigurationServiceInterface
	authService          ConfigurationAuthServiceInterface
}

func NewConfigurationHandler(configurationService ConfigurationServiceInterface, authService ConfigurationAuthServiceInterface) *ConfigurationHandler {
	return &ConfigurationHandler{
		configurationService: configurationService,
		authService:          authService,
	}
}

// Wizard endpoints

func (h *ConfigurationHandler) GetWizardStep(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	stepID := vars["step_id"]

	step, err := h.configurationService.GetWizardStep(stepID)
	if err != nil {
		stdError(w, http.StatusNotFound, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(step)
}

func (h *ConfigurationHandler) ValidateWizardStep(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	stepID := vars["step_id"]

	var data map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	validation, err := h.configurationService.ValidateWizardStep(stepID, data)
	if err != nil {
		stdError(w, http.StatusInternalServerError, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(validation)
}

func (h *ConfigurationHandler) SaveWizardProgress(w http.ResponseWriter, r *http.Request) {
	userID, ok := configUserID(r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	vars := mux.Vars(r)
	stepID := vars["step_id"]

	var finalData map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&finalData); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	err := h.configurationService.SaveWizardProgress(userID, stepID, finalData)
	if err != nil {
		stdError(w, http.StatusInternalServerError, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Progress saved successfully",
	})
}

func (h *ConfigurationHandler) GetWizardProgress(w http.ResponseWriter, r *http.Request) {
	userID, ok := configUserID(r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	progress, err := h.configurationService.GetWizardProgress(userID)
	if err != nil {
		stdError(w, http.StatusNotFound, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(progress)
}

func (h *ConfigurationHandler) CompleteWizard(w http.ResponseWriter, r *http.Request) {
	userID, ok := configUserID(r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var finalData map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&finalData); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// In a real implementation, this would come from wizard form
	config, err := h.configurationService.CompleteWizard(userID, finalData)
	if err != nil {
		stdError(w, http.StatusInternalServerError, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":       "Wizard completed successfully",
		"configuration": config,
	})
}

// Configuration endpoints

func (h *ConfigurationHandler) GetConfiguration(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value("user_id").(int)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	hasPermission, err := h.authService.CheckPermission(userID, models.PermissionSystemConfig)
	if err != nil || !hasPermission {
		http.Error(w, "Insufficient permissions", http.StatusForbidden)
		return
	}

	schema, err := h.configurationService.GetConfigurationSchema()
	if err != nil {
		stdError(w, http.StatusInternalServerError, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(schema)
}

func (h *ConfigurationHandler) TestConfiguration(w http.ResponseWriter, r *http.Request) {
	userID, ok := configUserID(r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	hasPermission, err := h.authService.CheckPermission(userID, models.PermissionSystemAdmin)
	if err != nil || !hasPermission {
		http.Error(w, "Insufficient permissions", http.StatusForbidden)
		return
	}

	var config models.Configuration
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Test the configuration through the service
	result, err := h.configurationService.TestConfiguration(&config)
	if err != nil {
		stdError(w, http.StatusInternalServerError, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (h *ConfigurationHandler) DeleteBackup(w http.ResponseWriter, r *http.Request) {
	userID, ok := configUserID(r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	vars := mux.Vars(r)
	_, err := strconv.Atoi(vars["backup_id"])
	if err != nil {
		http.Error(w, "Invalid backup ID", http.StatusBadRequest)
		return
	}

	hasPermission, err := h.authService.CheckPermission(userID, models.PermissionSystemAdmin)
	if err != nil || !hasPermission {
		http.Error(w, "Insufficient permissions", http.StatusForbidden)
		return
	}

	// No backup store sits behind this endpoint, so nothing was deleted;
	// reporting success would be a lie (CAT-001).
	http.Error(w, "Backup deletion is not implemented", http.StatusNotImplemented)
}

// Health check endpoint

func (h *ConfigurationHandler) GetSystemStatus(w http.ResponseWriter, r *http.Request) {
	userID, ok := configUserID(r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	hasPermission, err := h.authService.CheckPermission(userID, models.PermissionSystemConfig)
	if err != nil || !hasPermission {
		http.Error(w, "Insufficient permissions", http.StatusForbidden)
		return
	}

	// CAT-001 (R4-3): this endpoint used to answer 200 with status "healthy", a fixed
	// version, a fixed uptime and five components it never checked. No component health
	// source sits behind this handler, so it answers 501 instead of inventing one. The real,
	// measured health endpoint is GET /api/v1/health.
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	json.NewEncoder(w).Encode(map[string]string{
		"error": "component health is not reported by this endpoint: no health source is wired to it. Nothing was checked. Use GET /api/v1/health.",
	})
}

// configUserID reads the authenticated user id placed in the request context.
// A missing or wrongly typed value is "not authenticated", never a panic.
func configUserID(r *http.Request) (int, bool) {
	id, ok := r.Context().Value("user_id").(int)
	return id, ok
}
