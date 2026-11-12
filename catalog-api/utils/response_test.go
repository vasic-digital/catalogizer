package utils

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestSendErrorResponse(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		message    string
		err        error
		wantStatus int
		wantBody   string
	}{
		{
			name:       "error text not leaked as details",
			statusCode: http.StatusBadRequest,
			message:    "bad request",
			err:        errors.New("invalid input"),
			wantStatus: http.StatusBadRequest,
			wantBody:   `{"success":false,"error":"bad request"}`,
		},
		{
			name:       "error without details",
			statusCode: http.StatusInternalServerError,
			message:    "internal error",
			err:        nil,
			wantStatus: http.StatusInternalServerError,
			wantBody:   `{"success":false,"error":"internal error"}`,
		},
		{
			name:       "not found",
			statusCode: http.StatusNotFound,
			message:    "resource not found",
			err:        nil,
			wantStatus: http.StatusNotFound,
			wantBody:   `{"success":false,"error":"resource not found"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			SendErrorResponse(c, tt.statusCode, tt.message, tt.err)

			assert.Equal(t, tt.wantStatus, w.Code)
			assert.JSONEq(t, tt.wantBody, w.Body.String())
		})
	}
}

func TestSendSuccessResponse(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		data       interface{}
		message    string
		wantStatus int
		wantBody   string
	}{
		{
			name:       "success with data and message",
			statusCode: http.StatusOK,
			data:       map[string]string{"key": "value"},
			message:    "operation successful",
			wantStatus: http.StatusOK,
			wantBody:   `{"success":true,"data":{"key":"value"},"message":"operation successful"}`,
		},
		{
			name:       "success with only data",
			statusCode: http.StatusOK,
			data:       []int{1, 2, 3},
			message:    "",
			wantStatus: http.StatusOK,
			wantBody:   `{"success":true,"data":[1,2,3]}`,
		},
		{
			name:       "created response",
			statusCode: http.StatusCreated,
			data:       map[string]int{"id": 123},
			message:    "resource created",
			wantStatus: http.StatusCreated,
			wantBody:   `{"success":true,"data":{"id":123},"message":"resource created"}`,
		},
		{
			name:       "success with nil data",
			statusCode: http.StatusOK,
			data:       nil,
			message:    "no content",
			wantStatus: http.StatusOK,
			wantBody:   `{"success":true,"message":"no content"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			SendSuccessResponse(c, tt.statusCode, tt.data, tt.message)

			assert.Equal(t, tt.wantStatus, w.Code)
			assert.JSONEq(t, tt.wantBody, w.Body.String())
		})
	}
}

func TestStringPtr(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "regular string",
			input: "hello",
			want:  "hello",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "string with spaces",
			input: "hello world",
			want:  "hello world",
		},
		{
			name:  "unicode string",
			input: "こんにちは",
			want:  "こんにちは",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := StringPtr(tt.input)
			assert.NotNil(t, result)
			assert.Equal(t, tt.want, *result)
		})
	}
}

func TestErrorResponseStruct(t *testing.T) {
	// Test that the struct can be properly marshaled
	response := ErrorResponse{
		Success: false,
		Error:   "test error",
		Details: "additional details",
	}

	assert.False(t, response.Success)
	assert.Equal(t, "test error", response.Error)
	assert.Equal(t, "additional details", response.Details)
}

func TestSuccessResponseStruct(t *testing.T) {
	// Test that the struct can be properly marshaled
	response := SuccessResponse{
		Success: true,
		Data:    map[string]int{"count": 42},
		Message: "success",
	}

	assert.True(t, response.Success)
	assert.Equal(t, "success", response.Message)
	assert.NotNil(t, response.Data)
}

// PX-F23 (CAT-PXF23): internal error text must never reach the client body.
func TestSendErrorResponse_DoesNotLeakInternalErrorText(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const marker = "pq: connection refused host=db"
	for _, status := range []int{400, 401, 403, 404, 409, 422, 500, 502, 503} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		SendErrorResponse(c, status, "generic message", errors.New(marker))
		assert.Equal(t, status, w.Code)
		assert.NotContains(t, w.Body.String(), "connection refused", "status %d", status)
		assert.NotContains(t, w.Body.String(), "host=db", "status %d", status)
		assert.NotContains(t, w.Body.String(), `"details"`, "status %d", status)
		assert.Contains(t, w.Body.String(), "generic message")
	}
}
