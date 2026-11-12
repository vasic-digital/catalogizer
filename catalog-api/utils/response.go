package utils

import (
	"catalogizer/internal/logging"
	"go.uber.org/zap"

	"github.com/gin-gonic/gin"
)

// ErrorResponse represents an error response
type ErrorResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
	Details string `json:"details,omitempty"`
}

// SuccessResponse represents a success response
type SuccessResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Message string      `json:"message,omitempty"`
}

// SendErrorResponse sends an error response
func SendErrorResponse(c *gin.Context, statusCode int, message string, err error) {
	response := ErrorResponse{
		Success: false,
		Error:   message,
	}

	// PX-F23 (CAT-PXF23): err.Error() is internal text (driver errors, JWT
	// parse text, file paths) and is NEVER sent to the client; it is logged
	// server-side only. Details stays unset so the body carries message only.
	if err != nil {
		if logging.Logger != nil {
			fields := []zap.Field{
				logging.String("message", message),
				logging.Int("status", statusCode),
				logging.ErrorField(err),
			}
			if rid, ok := c.Get("request_id"); ok {
				if s, ok := rid.(string); ok {
					fields = append(fields, logging.String("request_id", s))
				}
			}
			logging.With(fields...).Error("Error response sent")
		}
	}

	c.JSON(statusCode, response)
}

// SendSuccessResponse sends a success response
func SendSuccessResponse(c *gin.Context, statusCode int, data interface{}, message string) {
	response := SuccessResponse{
		Success: true,
		Data:    data,
		Message: message,
	}

	c.JSON(statusCode, response)
}

// StringPtr returns a pointer to the given string
func StringPtr(s string) *string {
	return &s
}
