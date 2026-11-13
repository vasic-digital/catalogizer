package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"catalogizer/internal/logging"
	"catalogizer/internal/logredact"
	"catalogizer/models"
	"catalogizer/services"

	"github.com/gorilla/mux"
	"go.uber.org/zap"
)

// logServiceStatus maps a log-management service error to the HTTP status the API contract documents:
// an unknown collection or share is 404, someone else's is 403, an unknown export format is 400; anything else is 500.
func logServiceStatus(err error) int {
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return http.StatusNotFound
	case errors.Is(err, services.ErrLogAccessDenied):
		return http.StatusForbidden
	case errors.Is(err, services.ErrUnsupportedLogFormat):
		return http.StatusBadRequest
	}
	return http.StatusInternalServerError
}

// Error-text redaction (PX-F17, structural round 3): the service error text is redacted by logredact.ErrorText - ONE tokenizing
// pass (userinfo, credential headers, secret path segments, key/value pairs, auth schemes) driven by the same sensitive-name
// list as the access-log query filter, 64 KiB pre-cap before redaction, 2048-byte cap after it. See internal/logredact/text.go
// for the shapes and the documented over-redaction policy.
const logErrMaxBytes = logredact.ErrorTextMaxBytes

// redactLogErrorText removes credential-shaped substrings from a service error text before it is written to the server log.
func redactLogErrorText(s string) string { return logredact.ErrorText(s) }

// logLogServiceError records the service error server-side (PX-F17) before the fixed body is written: the redacted error text,
// the response status, the handler site and the request path (no query string; secret path segments such as /logs/share/<token> are redacted).
func logLogServiceError(r *http.Request, err error, status int) {
	if err == nil || logging.Logger == nil {
		return
	}
	site, path := "", ""
	if r != nil {
		site = r.Method
		if r.URL != nil {
			path = logredact.SecretPath(r.URL.Path) // a share token in the path never reaches the log
		}
	}
	logging.Logger.Error("log management request failed",
		zap.String("error", redactLogErrorText(err.Error())),
		zap.Int("status", status),
		zap.String("site", site+" "+path),
	)
}

// writeLogServiceError writes the mapped status with a fixed, generic body: the service error text (repository wrapping,
// `sql: no rows in result set`, driver or path details) stays server-side (logged redacted) and never reaches the client.
func writeLogServiceError(w http.ResponseWriter, r *http.Request, err error) {
	status := logServiceStatus(err)
	var msg string
	switch status {
	case http.StatusNotFound:
		msg = "Not found"
	case http.StatusForbidden:
		msg = "Access denied"
	case http.StatusBadRequest:
		msg = "Unsupported format"
	default:
		msg = "Internal server error"
	}
	logLogServiceError(r, err, status)
	http.Error(w, msg, status)
}

// writeFixedLogError writes the given status with a fixed, generic body for handlers whose documented status does not
// depend on the error class (the service error text stays server-side, logged redacted).
func writeFixedLogError(w http.ResponseWriter, r *http.Request, err error, status int) {
	msg := "Internal server error"
	if status == http.StatusNotFound {
		msg = "Not found"
	}
	logLogServiceError(r, err, status)
	http.Error(w, msg, status)
}

// LogManagementServiceInterface defines interface for log management service operations
type LogManagementServiceInterface interface {
	CollectLogs(userID int, request *models.LogCollectionRequest) (*models.LogCollection, error)
	GetLogCollection(collectionID int, userID int) (*models.LogCollection, error)
	GetLogCollectionsByUser(userID int, limit, offset int) ([]models.LogCollection, error)
	GetLogEntries(collectionID int, userID int, filters *models.LogEntryFilters) ([]models.LogEntry, error)
	CreateLogShare(userID int, request *models.LogShareRequest) (*models.LogShare, error)
	GetLogShare(token string) (*models.LogShare, error)
	RevokeLogShare(shareID int, userID int) error
	ExportLogs(collectionID int, userID int, format string) ([]byte, error)
	StreamLogs(userID int, filters *models.LogStreamFilters) (<-chan models.LogEntry, chan<- struct{}, error)
	AnalyzeLogs(collectionID int, userID int) (*models.LogAnalysis, error)
	GetLogStatistics(userID int) (*models.LogStatistics, error)
	GetConfiguration() *services.LogManagementConfig
	UpdateConfiguration(config *services.LogManagementConfig) error
	CleanupOldLogs() error
}

// LogManagementAuthServiceInterface defines interface for authentication service operations
type LogManagementAuthServiceInterface interface {
	CheckPermission(userID int, permission string) (bool, error)
}

type LogManagementHandler struct {
	logManagementService LogManagementServiceInterface
	authService          LogManagementAuthServiceInterface
}

func NewLogManagementHandler(logManagementService LogManagementServiceInterface, authService LogManagementAuthServiceInterface) *LogManagementHandler {
	return &LogManagementHandler{
		logManagementService: logManagementService,
		authService:          authService,
	}
}

func (h *LogManagementHandler) CreateLogCollection(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value("user_id").(int)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	hasPermission, err := h.authService.CheckPermission(userID, models.PermissionSystemAdmin)
	if err != nil || !hasPermission {
		http.Error(w, "Insufficient permissions", http.StatusForbidden)
		return
	}

	var request models.LogCollectionRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	collection, err := h.logManagementService.CollectLogs(userID, &request)
	if err != nil {
		writeFixedLogError(w, r, err, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(collection)
}

func (h *LogManagementHandler) GetLogCollection(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value("user_id").(int)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	vars := mux.Vars(r)
	collectionID, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid collection ID", http.StatusBadRequest)
		return
	}

	hasPermission, err := h.authService.CheckPermission(userID, models.PermissionSystemAdmin)
	if err != nil || !hasPermission {
		http.Error(w, "Insufficient permissions", http.StatusForbidden)
		return
	}

	collection, err := h.logManagementService.GetLogCollection(collectionID, userID)
	if err != nil {
		writeFixedLogError(w, r, err, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(collection)
}

func (h *LogManagementHandler) ListLogCollections(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value("user_id").(int)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	hasPermission, err := h.authService.CheckPermission(userID, models.PermissionSystemAdmin)
	if err != nil || !hasPermission {
		http.Error(w, "Insufficient permissions", http.StatusForbidden)
		return
	}

	// Parse query parameters
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	limit := 20 // Default limit
	offset := 0 // Default offset

	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	if offsetStr != "" {
		if parsedOffset, err := strconv.Atoi(offsetStr); err == nil && parsedOffset >= 0 {
			offset = parsedOffset
		}
	}

	collections, err := h.logManagementService.GetLogCollectionsByUser(userID, limit, offset)
	if err != nil {
		writeFixedLogError(w, r, err, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"collections": collections,
		"limit":       limit,
		"offset":      offset,
	})
}

func (h *LogManagementHandler) GetLogEntries(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value("user_id").(int)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	vars := mux.Vars(r)
	collectionID, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid collection ID", http.StatusBadRequest)
		return
	}

	hasPermission, err := h.authService.CheckPermission(userID, models.PermissionSystemAdmin)
	if err != nil || !hasPermission {
		http.Error(w, "Insufficient permissions", http.StatusForbidden)
		return
	}

	filters := h.parseLogEntryFilters(r)
	entries, err := h.logManagementService.GetLogEntries(collectionID, userID, filters)
	if err != nil {
		writeLogServiceError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"entries": entries,
		"filters": filters,
	})
}

func (h *LogManagementHandler) CreateLogShare(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value("user_id").(int)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	hasPermission, err := h.authService.CheckPermission(userID, models.PermissionSystemAdmin)
	if err != nil || !hasPermission {
		http.Error(w, "Insufficient permissions", http.StatusForbidden)
		return
	}

	var request models.LogShareRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	share, err := h.logManagementService.CreateLogShare(userID, &request)
	if err != nil {
		writeLogServiceError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(share)
}

func (h *LogManagementHandler) GetLogShare(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	token := vars["token"]

	share, err := h.logManagementService.GetLogShare(token)
	if err != nil {
		writeFixedLogError(w, r, err, http.StatusNotFound)
		return
	}

	// Get the collection
	collection, err := h.logManagementService.GetLogCollection(share.CollectionID, share.UserID)
	if err != nil {
		writeFixedLogError(w, r, err, http.StatusNotFound)
		return
	}

	// Check permissions
	canRead := false
	for _, permission := range share.Permissions {
		if permission == "read" {
			canRead = true
			break
		}
	}

	if !canRead {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"share":      share,
		"collection": collection,
	})
}

func (h *LogManagementHandler) RevokeLogShare(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value("user_id").(int)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	// The permission gate runs before the id is parsed, so a caller without it learns nothing about id validity.
	hasPermission, err := h.authService.CheckPermission(userID, models.PermissionSystemAdmin)
	if err != nil || !hasPermission {
		http.Error(w, "Insufficient permissions", http.StatusForbidden)
		return
	}

	vars := mux.Vars(r)
	shareID, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid share ID", http.StatusBadRequest)
		return
	}

	err = h.logManagementService.RevokeLogShare(shareID, userID)
	if err != nil {
		writeLogServiceError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *LogManagementHandler) ExportLogs(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value("user_id").(int)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	vars := mux.Vars(r)
	collectionID, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid collection ID", http.StatusBadRequest)
		return
	}

	hasPermission, err := h.authService.CheckPermission(userID, models.PermissionSystemAdmin)
	if err != nil || !hasPermission {
		http.Error(w, "Insufficient permissions", http.StatusForbidden)
		return
	}

	format := r.URL.Query().Get("format")
	if format == "" {
		format = "json" // Default format
	}

	data, err := h.logManagementService.ExportLogs(collectionID, userID, format)
	if err != nil {
		writeLogServiceError(w, r, err)
		return
	}

	// Set appropriate content type and headers
	switch format {
	case "json":
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", "attachment; filename=logs.json")
	case "csv":
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", "attachment; filename=logs.csv")
	case "txt":
		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("Content-Disposition", "attachment; filename=logs.txt")
	case "zip":
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", "attachment; filename=logs.zip")
	default:
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", "attachment; filename=logs.bin")
	}

	w.Write(data)
}

func (h *LogManagementHandler) StreamLogs(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value("user_id").(int)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	hasPermission, err := h.authService.CheckPermission(userID, models.PermissionSystemAdmin)
	if err != nil || !hasPermission {
		http.Error(w, "Insufficient permissions", http.StatusForbidden)
		return
	}

	// Parse stream filters
	filters := h.parseLogStreamFilters(r)

	// Setup Server-Sent Events
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	origin := r.Header.Get("Origin")
	if origin != "" {
		allowedOrigins := os.Getenv("CORS_ALLOWED_ORIGINS")
		if allowedOrigins == "" {
			allowedOrigins = "http://localhost:5173,http://localhost:3000"
		}
		for _, o := range strings.Split(allowedOrigins, ",") {
			if strings.TrimSpace(o) == origin {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				break
			}
		}
	}

	// Get log stream
	logChannel, done, err := h.logManagementService.StreamLogs(userID, filters)
	if err != nil {
		writeFixedLogError(w, r, err, http.StatusInternalServerError)
		return
	}
	// Ensure the streaming goroutine is stopped when the client disconnects.
	defer close(done)

	// Stream logs to client
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	for {
		select {
		case entry, ok := <-logChannel:
			if !ok {
				return
			}

			data, err := json.Marshal(entry)
			if err != nil {
				continue
			}

			w.Write([]byte("data: "))
			w.Write(data)
			w.Write([]byte("\n\n"))
			flusher.Flush()

		case <-r.Context().Done():
			return
		}
	}
}

func (h *LogManagementHandler) AnalyzeLogs(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value("user_id").(int)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	vars := mux.Vars(r)
	collectionID, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid collection ID", http.StatusBadRequest)
		return
	}

	hasPermission, err := h.authService.CheckPermission(userID, models.PermissionSystemAdmin)
	if err != nil || !hasPermission {
		http.Error(w, "Insufficient permissions", http.StatusForbidden)
		return
	}

	analysis, err := h.logManagementService.AnalyzeLogs(collectionID, userID)
	if err != nil {
		writeLogServiceError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(analysis)
}

func (h *LogManagementHandler) GetLogStatistics(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value("user_id").(int)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	hasPermission, err := h.authService.CheckPermission(userID, models.PermissionSystemAdmin)
	if err != nil || !hasPermission {
		http.Error(w, "Insufficient permissions", http.StatusForbidden)
		return
	}

	stats, err := h.logManagementService.GetLogStatistics(userID)
	if err != nil {
		writeFixedLogError(w, r, err, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func (h *LogManagementHandler) GetConfiguration(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value("user_id").(int)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	hasPermission, err := h.authService.CheckPermission(userID, models.PermissionSystemAdmin)
	if err != nil || !hasPermission {
		http.Error(w, "Insufficient permissions", http.StatusForbidden)
		return
	}

	config := h.logManagementService.GetConfiguration()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(config)
}

func (h *LogManagementHandler) UpdateConfiguration(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value("user_id").(int)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	hasPermission, err := h.authService.CheckPermission(userID, models.PermissionSystemAdmin)
	if err != nil || !hasPermission {
		http.Error(w, "Insufficient permissions", http.StatusForbidden)
		return
	}

	var config services.LogManagementConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	err = h.logManagementService.UpdateConfiguration(&config)
	if err != nil {
		writeFixedLogError(w, r, err, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Configuration updated successfully",
	})
}

func (h *LogManagementHandler) CleanupOldLogs(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value("user_id").(int)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	hasPermission, err := h.authService.CheckPermission(userID, models.PermissionSystemAdmin)
	if err != nil || !hasPermission {
		http.Error(w, "Insufficient permissions", http.StatusForbidden)
		return
	}

	err = h.logManagementService.CleanupOldLogs()
	if err != nil {
		writeFixedLogError(w, r, err, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Cleanup completed successfully",
	})
}

// Helper methods

func (h *LogManagementHandler) parseLogEntryFilters(r *http.Request) *models.LogEntryFilters {
	filters := &models.LogEntryFilters{}

	if level := r.URL.Query().Get("level"); level != "" {
		filters.Level = level
	}

	if component := r.URL.Query().Get("component"); component != "" {
		filters.Component = component
	}

	if search := r.URL.Query().Get("search"); search != "" {
		filters.Search = search
	}

	if startTime := r.URL.Query().Get("start_time"); startTime != "" {
		if date, err := time.Parse(time.RFC3339, startTime); err == nil {
			filters.StartTime = &date
		}
	}

	if endTime := r.URL.Query().Get("end_time"); endTime != "" {
		if date, err := time.Parse(time.RFC3339, endTime); err == nil {
			filters.EndTime = &date
		}
	}

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 {
			filters.Limit = limit
		}
	}

	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil && offset >= 0 {
			filters.Offset = offset
		}
	}

	return filters
}

func (h *LogManagementHandler) parseLogStreamFilters(r *http.Request) *models.LogStreamFilters {
	filters := &models.LogStreamFilters{}

	if level := r.URL.Query().Get("level"); level != "" {
		filters.Level = level
	}

	if component := r.URL.Query().Get("component"); component != "" {
		filters.Component = component
	}

	if search := r.URL.Query().Get("search"); search != "" {
		filters.Search = search
	}

	return filters
}
