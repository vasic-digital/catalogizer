package handlers

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"catalogizer/internal/pathguard"
	"catalogizer/models"
	"github.com/gin-gonic/gin"
)

// ConversionServiceInterface defines the interface for conversion service operations
type ConversionServiceInterface interface {
	CreateConversionJob(userID int, request *models.ConversionRequest) (*models.ConversionJob, error)
	GetJob(jobID int, userID int) (*models.ConversionJob, error)
	GetUserJobs(userID int, status *string, limit, offset int) ([]models.ConversionJob, error)
	CancelJob(jobID int, userID int) error
	RetryJob(jobID int, userID int) error
	GetSupportedFormats() *models.SupportedFormats
}

// ConversionAuthServiceInterface defines the interface for authentication service operations
type ConversionAuthServiceInterface interface {
	CheckPermission(userID int, permission string) (bool, error)
	GetCurrentUser(token string) (*models.User, error)
}

type ConversionHandler struct {
	conversionService ConversionServiceInterface
	authService       ConversionAuthServiceInterface
}

func NewConversionHandler(conversionService ConversionServiceInterface, authService ConversionAuthServiceInterface) *ConversionHandler {
	return &ConversionHandler{
		conversionService: conversionService,
		authService:       authService,
	}
}

func (h *ConversionHandler) CreateJob(c *gin.Context) {
	currentUser, err := h.getCurrentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	hasPermission, err := h.authService.CheckPermission(currentUser.ID, models.PermissionConversionCreate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check permissions"})
		return
	}

	if !hasPermission {
		c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions"})
		return
	}

	var request models.ConversionRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	job, err := h.conversionService.CreateConversionJob(currentUser.ID, &request)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create conversion job"})
		return
	}

	c.JSON(http.StatusOK, job)
}

func (h *ConversionHandler) GetJob(c *gin.Context) {
	currentUser, err := h.getCurrentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	jobIDStr := c.Param("id")
	jobID, err := strconv.Atoi(jobIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid job ID"})
		return
	}

	job, err := h.conversionService.GetJob(jobID, currentUser.ID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": "Job not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get job"})
		return
	}

	c.JSON(http.StatusOK, job)
}

func (h *ConversionHandler) ListJobs(c *gin.Context) {
	currentUser, err := h.getCurrentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	hasPermission, err := h.authService.CheckPermission(currentUser.ID, models.PermissionConversionView)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check permissions"})
		return
	}

	if !hasPermission {
		c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions"})
		return
	}

	limit := 50
	offset := 0
	status := c.Query("status")

	if limitStr := c.Query("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 && parsedLimit <= 100 {
			limit = parsedLimit
		}
	}

	if offsetStr := c.Query("offset"); offsetStr != "" {
		if parsedOffset, err := strconv.Atoi(offsetStr); err == nil && parsedOffset >= 0 {
			offset = parsedOffset
		}
	}

	jobs, err := h.conversionService.GetUserJobs(currentUser.ID, &status, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get jobs"})
		return
	}

	c.JSON(http.StatusOK, jobs)
}

func (h *ConversionHandler) CancelJob(c *gin.Context) {
	currentUser, err := h.getCurrentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	hasPermission, err := h.authService.CheckPermission(currentUser.ID, models.PermissionConversionManage)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check permissions"})
		return
	}

	if !hasPermission {
		c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions"})
		return
	}

	jobIDStr := c.Param("id")
	jobID, err := strconv.Atoi(jobIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid job ID"})
		return
	}

	err = h.conversionService.CancelJob(jobID, currentUser.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to cancel job"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Job cancelled successfully"})
}

func (h *ConversionHandler) GetSupportedFormats(c *gin.Context) {
	currentUser, err := h.getCurrentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	hasPermission, err := h.authService.CheckPermission(currentUser.ID, models.PermissionConversionView)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check permissions"})
		return
	}

	if !hasPermission {
		c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions"})
		return
	}

	formats := h.conversionService.GetSupportedFormats()

	c.JSON(http.StatusOK, formats)
}

// DeleteJob cancels and removes a conversion job.
// The frontend calls DELETE /conversion/jobs/:id which maps to CancelJob.
func (h *ConversionHandler) DeleteJob(c *gin.Context) {
	currentUser, err := h.getCurrentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	hasPermission, err := h.authService.CheckPermission(currentUser.ID, models.PermissionConversionManage)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check permissions"})
		return
	}

	if !hasPermission {
		c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions"})
		return
	}

	jobIDStr := c.Param("id")
	jobID, err := strconv.Atoi(jobIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid job ID"})
		return
	}

	err = h.conversionService.CancelJob(jobID, currentUser.ID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": "Job not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete job"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Job deleted successfully"})
}

// RetryJob retries a failed conversion job.
// POST /conversion/jobs/:id/retry
func (h *ConversionHandler) RetryJob(c *gin.Context) {
	currentUser, err := h.getCurrentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	hasPermission, err := h.authService.CheckPermission(currentUser.ID, models.PermissionConversionManage)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check permissions"})
		return
	}

	if !hasPermission {
		c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions"})
		return
	}

	jobIDStr := c.Param("id")
	jobID, err := strconv.Atoi(jobIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid job ID"})
		return
	}

	err = h.conversionService.RetryJob(jobID, currentUser.ID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": "Job not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retry job"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Job retry started"})
}

// DownloadJobFile serves the output file of a completed conversion job.
// GET /conversion/jobs/:id/download
func (h *ConversionHandler) DownloadJobFile(c *gin.Context) {
	currentUser, err := h.getCurrentUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	jobIDStr := c.Param("id")
	jobID, err := strconv.Atoi(jobIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid job ID"})
		return
	}

	job, err := h.conversionService.GetJob(jobID, currentUser.ID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": "Job not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get job"})
		return
	}

	if job.Status != "completed" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Job is not completed"})
		return
	}

	if job.TargetPath == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "No output file available"})
		return
	}

	// Serve only a regular file inside the conversion write roots. c.File follows symlinks and
	// re-opens by path, so a job stored before the confinement existed, edited in the database
	// or with a symlink planted at its target would otherwise serve any readable file (review
	// round 6, class-exhaustive sink pass).
	if _, err := pathguard.ConfineDataFile(job.TargetPath, pathguard.ConversionWriteRoots()); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Output file is outside the conversion roots"})
		return
	}
	if fi, err := os.Lstat(job.TargetPath); err != nil || !fi.Mode().IsRegular() {
		c.JSON(http.StatusNotFound, gin.H{"error": "No output file available"})
		return
	}
	c.File(job.TargetPath)
}

func (h *ConversionHandler) getCurrentUser(c *gin.Context) (*models.User, error) {
	token := c.GetHeader("Authorization")
	if token == "" {
		return nil, models.ErrUnauthorized
	}

	// Remove "Bearer " prefix if present
	if len(token) > 7 && token[:7] == "Bearer " {
		token = token[7:]
	}

	// Use auth service to get current user from token
	user, err := h.authService.GetCurrentUser(token)
	if err != nil {
		return nil, fmt.Errorf("auth error: %w", err)
	}
	return user, nil
}
