package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"catalogizer/services"
	"digital.vasic.challenges/pkg/challenge"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Dedicated challenge handler tests ---
// This file provides additional coverage beyond challenge_handler_test.go,
// focusing on concurrency, rich payloads, edge cases, and HTTP method enforcement.

// setupDedicatedChallengeTest creates a handler, mock, and router for dedicated tests.
func setupDedicatedChallengeTest() (*ChallengeHandler, *mockChallengeService, *gin.Engine) {
	gin.SetMode(gin.TestMode)
	mock := &mockChallengeService{}
	handler := NewChallengeHandler(mock)
	router := gin.New()
	router.GET("/api/v1/challenges", handler.ListChallenges)
	router.GET("/api/v1/challenges/results", handler.GetResults)
	router.GET("/api/v1/challenges/:id", handler.GetChallenge)
	router.POST("/api/v1/challenges/:id/run", handler.RunChallenge)
	router.POST("/api/v1/challenges/run/all", handler.RunAll)
	router.POST("/api/v1/challenges/run/category/:category", handler.RunByCategory)
	return handler, mock, router
}

// makeRichResult builds a challenge.Result with metrics, outputs, and logs.
func makeRichResult(id string, name string, status string) *challenge.Result {
	now := time.Now()
	return &challenge.Result{
		ChallengeID:   challenge.ID(id),
		ChallengeName: name,
		Status:        status,
		StartTime:     now.Add(-2 * time.Second),
		EndTime:       now,
		Duration:      2 * time.Second,
		Assertions: []challenge.AssertionResult{
			{Type: "not_empty", Target: "response", Expected: true, Actual: true, Passed: true, Message: "response not empty"},
		},
		Metrics: map[string]challenge.MetricValue{
			"latency": {Name: "latency", Value: 42.5, Unit: "ms"},
			"count":   {Name: "count", Value: 100, Unit: "items"},
		},
		Outputs: map[string]string{
			"summary": "all checks passed",
			"detail":  "verified 100 items",
		},
		Logs: challenge.LogPaths{
			ChallengeLog: "/tmp/challenge.log",
			OutputLog:    "/tmp/output.log",
			APIRequests:  "/tmp/api-requests.log",
			APIResponses: "/tmp/api-responses.log",
		},
	}
}

// --- Concurrent request handling ---

func TestChallengeHandler_ListChallenges_ConcurrentRequests(t *testing.T) {
	_, mock, router := setupDedicatedChallengeTest()
	mock.listChallengesFunc = func() []services.ChallengeSummary {
		return []services.ChallengeSummary{
			{ID: "ch-001", Name: "C1", Description: "D1", Category: "test", Dependencies: []string{}},
			{ID: "ch-002", Name: "C2", Description: "D2", Category: "test", Dependencies: []string{}},
		}
	}

	const concurrency = 10
	var wg sync.WaitGroup
	wg.Add(concurrency)

	results := make([]int, concurrency)
	for i := 0; i < concurrency; i++ {
		go func(idx int) {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodGet, "/api/v1/challenges", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			results[idx] = w.Code
		}(i)
	}

	wg.Wait()
	for i, code := range results {
		assert.Equal(t, http.StatusOK, code, "request %d should succeed", i)
	}
}

func TestChallengeHandler_RunChallenge_ConcurrentRequests(t *testing.T) {
	_, mock, router := setupDedicatedChallengeTest()

	var mu sync.Mutex
	callCount := 0
	mock.runChallengeFunc = func(ctx context.Context, id string) (*challenge.Result, error) {
		mu.Lock()
		callCount++
		mu.Unlock()
		return makeResult(id, "Concurrent Test", challenge.StatusPassed), nil
	}

	const concurrency = 5
	var wg sync.WaitGroup
	wg.Add(concurrency)

	for i := 0; i < concurrency; i++ {
		go func(idx int) {
			defer wg.Done()
			id := fmt.Sprintf("ch-%03d", idx+1)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/challenges/"+id+"/run", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code)
		}(i)
	}

	wg.Wait()
	mu.Lock()
	assert.Equal(t, concurrency, callCount)
	mu.Unlock()
}

// --- Rich payload tests ---

func TestChallengeHandler_RunChallenge_WithMetricsAndOutputs(t *testing.T) {
	_, mock, router := setupDedicatedChallengeTest()
	mock.runChallengeFunc = func(ctx context.Context, id string) (*challenge.Result, error) {
		return makeRichResult(id, "Rich Result", challenge.StatusPassed), nil
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/challenges/ch-001/run", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, true, response["success"])

	data, ok := response["data"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "ch-001", data["challenge_id"])
	assert.Equal(t, challenge.StatusPassed, data["status"])

	// Verify metrics are present
	metrics, ok := data["metrics"].(map[string]interface{})
	require.True(t, ok)
	latency, ok := metrics["latency"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, 42.5, latency["value"])
	assert.Equal(t, "ms", latency["unit"])

	// Verify outputs are present
	outputs, ok := data["outputs"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "all checks passed", outputs["summary"])
	assert.Equal(t, "verified 100 items", outputs["detail"])

	// Verify logs are present
	logs, ok := data["logs"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "/tmp/challenge.log", logs["challenge_log"])
	assert.Equal(t, "/tmp/output.log", logs["output_log"])
}

func TestChallengeHandler_RunChallenge_WithErrorField(t *testing.T) {
	_, mock, router := setupDedicatedChallengeTest()
	mock.runChallengeFunc = func(ctx context.Context, id string) (*challenge.Result, error) {
		return &challenge.Result{
			ChallengeID:   challenge.ID(id),
			ChallengeName: "Error Challenge",
			Status:        challenge.StatusFailed,
			StartTime:     time.Now().Add(-time.Second),
			EndTime:       time.Now(),
			Duration:      time.Second,
			Error:         "assertion failed: expected 200, got 500",
		}, nil
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/challenges/ch-010/run", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, true, response["success"])

	data := response["data"].(map[string]interface{})
	assert.Equal(t, challenge.StatusFailed, data["status"])
	assert.Equal(t, "assertion failed: expected 200, got 500", data["error"])
}

func TestChallengeHandler_GetResults_WithRichResults(t *testing.T) {
	_, mock, router := setupDedicatedChallengeTest()
	mock.getResultsFunc = func() []*challenge.Result {
		return []*challenge.Result{
			makeRichResult("ch-001", "Rich 1", challenge.StatusPassed),
			makeRichResult("ch-002", "Rich 2", challenge.StatusFailed),
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/challenges/results", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response struct {
		Success bool               `json:"success"`
		Data    []challenge.Result `json:"data"`
		Count   int                `json:"count"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.True(t, response.Success)
	assert.Equal(t, 2, response.Count)
	assert.Len(t, response.Data, 2)

	// Verify first result has metrics
	assert.NotNil(t, response.Data[0].Metrics)
	assert.Contains(t, response.Data[0].Metrics, "latency")
	assert.Equal(t, 42.5, response.Data[0].Metrics["latency"].Value)

	// Verify first result has outputs
	assert.NotNil(t, response.Data[0].Outputs)
	assert.Equal(t, "all checks passed", response.Data[0].Outputs["summary"])

	// Verify first result has logs
	assert.Equal(t, "/tmp/challenge.log", response.Data[0].Logs.ChallengeLog)
}

// --- Large payload tests ---

func TestChallengeHandler_ListChallenges_LargeList(t *testing.T) {
	_, mock, router := setupDedicatedChallengeTest()

	const challengeCount = 100
	mock.listChallengesFunc = func() []services.ChallengeSummary {
		challenges := make([]services.ChallengeSummary, challengeCount)
		for i := 0; i < challengeCount; i++ {
			challenges[i] = services.ChallengeSummary{
				ID:           fmt.Sprintf("ch-%03d", i+1),
				Name:         fmt.Sprintf("Challenge %d", i+1),
				Description:  fmt.Sprintf("Description for challenge %d", i+1),
				Category:     "load-test",
				Dependencies: []string{},
			}
		}
		return challenges
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/challenges", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, float64(challengeCount), response["count"])

	data := response["data"].([]interface{})
	assert.Len(t, data, challengeCount)
}

func TestChallengeHandler_RunAll_LargeResultSet(t *testing.T) {
	_, mock, router := setupDedicatedChallengeTest()

	const resultCount = 50
	mock.runAllFunc = func(ctx context.Context) ([]*challenge.Result, error) {
		results := make([]*challenge.Result, resultCount)
		for i := 0; i < resultCount; i++ {
			status := challenge.StatusPassed
			if i%3 == 0 {
				status = challenge.StatusFailed
			}
			results[i] = makeResult(
				fmt.Sprintf("ch-%03d", i+1),
				fmt.Sprintf("C%d", i+1),
				status,
			)
		}
		return results, nil
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/challenges/run/all", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	summary := response["summary"].(map[string]interface{})
	assert.Equal(t, float64(resultCount), summary["total"])

	// Every 3rd result (indices 0,3,6,...) is failed: 0,3,6,...,48 = 17 failed
	expectedFailed := 17
	expectedPassed := resultCount - expectedFailed
	assert.Equal(t, float64(expectedPassed), summary["passed"])
	assert.Equal(t, float64(expectedFailed), summary["failed"])
}

// --- RunByCategory context pass-through ---

func TestChallengeHandler_RunByCategory_PassesContext(t *testing.T) {
	_, mock, router := setupDedicatedChallengeTest()
	contextReceived := false
	mock.runByCategoryFunc = func(ctx context.Context, category string) ([]*challenge.Result, error) {
		if ctx != nil {
			contextReceived = true
		}
		return []*challenge.Result{}, nil
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/challenges/run/category/setup", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, contextReceived, "handler should pass request context to service")
}

// --- Case sensitivity in IDs ---

func TestChallengeHandler_GetChallenge_CaseSensitiveID(t *testing.T) {
	_, mock, router := setupDedicatedChallengeTest()
	mock.listChallengesFunc = func() []services.ChallengeSummary {
		return []services.ChallengeSummary{
			{ID: "CH-001", Name: "Upper", Description: "Uppercase ID", Category: "test", Dependencies: []string{}},
		}
	}

	// Request with lowercase should not match uppercase
	req := httptest.NewRequest(http.MethodGet, "/api/v1/challenges/ch-001", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	// Request with exact case should match
	req = httptest.NewRequest(http.MethodGet, "/api/v1/challenges/CH-001", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, true, response["success"])
}

// --- HTTP method enforcement ---

func TestChallengeHandler_WrongHTTPMethods(t *testing.T) {
	_, _, router := setupDedicatedChallengeTest()

	// Gin returns 404 when no matching route exists for the given method.
	// Note: GET /api/v1/challenges/:id catches many GET paths via the param.
	tests := []struct {
		name   string
		method string
		path   string
	}{
		{"POST to list challenges", http.MethodPost, "/api/v1/challenges"},
		{"DELETE to list challenges", http.MethodDelete, "/api/v1/challenges"},
		{"PUT to list challenges", http.MethodPut, "/api/v1/challenges"},
		{"DELETE to run challenge", http.MethodDelete, "/api/v1/challenges/ch-001/run"},
		{"PUT to run all", http.MethodPut, "/api/v1/challenges/run/all"},
		{"DELETE to run by category", http.MethodDelete, "/api/v1/challenges/run/category/test"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Wrong method should not return 200 OK
			assert.NotEqual(t, http.StatusOK, w.Code,
				"wrong method %s on %s should not return 200", tt.method, tt.path)
		})
	}
}

// --- RunAll with nil vs empty results ---

func TestChallengeHandler_RunAll_NilResults(t *testing.T) {
	_, mock, router := setupDedicatedChallengeTest()
	mock.runAllFunc = func(ctx context.Context) ([]*challenge.Result, error) {
		return nil, nil
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/challenges/run/all", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, true, response["success"])

	summary := response["summary"].(map[string]interface{})
	assert.Equal(t, float64(0), summary["total"])
	assert.Equal(t, float64(0), summary["passed"])
	assert.Equal(t, float64(0), summary["failed"])
}

// --- RunByCategory with nil results ---

func TestChallengeHandler_RunByCategory_NilResults(t *testing.T) {
	_, mock, router := setupDedicatedChallengeTest()
	mock.runByCategoryFunc = func(ctx context.Context, category string) ([]*challenge.Result, error) {
		return nil, nil
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/challenges/run/category/scan", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, true, response["success"])

	summary := response["summary"].(map[string]interface{})
	assert.Equal(t, float64(0), summary["total"])
	assert.Equal(t, float64(0), summary["passed"])
	assert.Equal(t, float64(0), summary["failed"])
}

// --- RunChallenge nil result with no error ---

func TestChallengeHandler_RunChallenge_NilResultNoError(t *testing.T) {
	_, mock, router := setupDedicatedChallengeTest()
	mock.runChallengeFunc = func(ctx context.Context, id string) (*challenge.Result, error) {
		return nil, nil
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/challenges/ch-001/run", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, true, response["success"])
	// data should be null when result is nil
	assert.Nil(t, response["data"])
}

// --- GetChallenge with special characters in ID ---

func TestChallengeHandler_GetChallenge_SpecialCharactersInID(t *testing.T) {
	_, mock, router := setupDedicatedChallengeTest()
	mock.listChallengesFunc = func() []services.ChallengeSummary {
		return []services.ChallengeSummary{
			{ID: "uf-api-001", Name: "UF API 1", Description: "User flow", Category: "userflow", Dependencies: []string{}},
			{ID: "mod-001", Name: "Module 1", Description: "Module check", Category: "module", Dependencies: []string{}},
		}
	}

	tests := []struct {
		name           string
		id             string
		expectedStatus int
	}{
		{"hyphenated ID found", "uf-api-001", http.StatusOK},
		{"module ID found", "mod-001", http.StatusOK},
		{"unknown ID not found", "xyz-999", http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/challenges/"+tt.id, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}

// --- RunChallenge verifies ID is forwarded correctly ---

func TestChallengeHandler_RunChallenge_IDForwarding(t *testing.T) {
	tests := []struct {
		name string
		id   string
	}{
		{"simple ID", "ch-001"},
		{"hyphenated ID", "uf-web-042"},
		{"module ID", "mod-015"},
		{"long ID", "ch-001-extended-challenge-id"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, mock, router := setupDedicatedChallengeTest()
			var capturedID string
			mock.runChallengeFunc = func(ctx context.Context, id string) (*challenge.Result, error) {
				capturedID = id
				return makeResult(id, "Test", challenge.StatusPassed), nil
			}

			req := httptest.NewRequest(http.MethodPost, "/api/v1/challenges/"+tt.id+"/run", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, tt.id, capturedID)
		})
	}
}

// --- RunByCategory verifies category is forwarded correctly ---

func TestChallengeHandler_RunByCategory_CategoryForwarding(t *testing.T) {
	tests := []struct {
		name     string
		category string
	}{
		{"simple category", "setup"},
		{"hyphenated category", "post-scan"},
		{"underscore category", "media_entity"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, mock, router := setupDedicatedChallengeTest()
			var capturedCategory string
			mock.runByCategoryFunc = func(ctx context.Context, category string) ([]*challenge.Result, error) {
				capturedCategory = category
				return []*challenge.Result{}, nil
			}

			req := httptest.NewRequest(http.MethodPost, "/api/v1/challenges/run/category/"+tt.category, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, tt.category, capturedCategory)
		})
	}
}

// --- RunAll summary with all status types ---

func TestChallengeHandler_RunAll_AllStatusTypes(t *testing.T) {
	_, mock, router := setupDedicatedChallengeTest()
	mock.runAllFunc = func(ctx context.Context) ([]*challenge.Result, error) {
		return []*challenge.Result{
			makeResult("ch-001", "Passed", challenge.StatusPassed),
			makeResult("ch-002", "Failed", challenge.StatusFailed),
			makeResult("ch-003", "Skipped", challenge.StatusSkipped),
			makeResult("ch-004", "Timed Out", challenge.StatusTimedOut),
			makeResult("ch-005", "Stuck", challenge.StatusStuck),
			makeResult("ch-006", "Error", challenge.StatusError),
			makeResult("ch-007", "Pending", challenge.StatusPending),
			makeResult("ch-008", "Running", challenge.StatusRunning),
		}, nil
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/challenges/run/all", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	summary := response["summary"].(map[string]interface{})
	assert.Equal(t, float64(8), summary["total"])
	assert.Equal(t, float64(1), summary["passed"])
	assert.Equal(t, float64(7), summary["failed"]) // Everything except "passed" counts as failed
}

// --- RunByCategory summary with mixed statuses ---

func TestChallengeHandler_RunByCategory_MixedStatuses(t *testing.T) {
	_, mock, router := setupDedicatedChallengeTest()
	mock.runByCategoryFunc = func(ctx context.Context, category string) ([]*challenge.Result, error) {
		return []*challenge.Result{
			makeResult("ch-001", "P1", challenge.StatusPassed),
			makeResult("ch-002", "P2", challenge.StatusPassed),
			makeResult("ch-003", "F1", challenge.StatusFailed),
			makeResult("ch-004", "T1", challenge.StatusTimedOut),
			makeResult("ch-005", "S1", challenge.StatusSkipped),
		}, nil
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/challenges/run/category/integration", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	summary := response["summary"].(map[string]interface{})
	assert.Equal(t, float64(5), summary["total"])
	assert.Equal(t, float64(2), summary["passed"])
	assert.Equal(t, float64(3), summary["failed"])
}

// --- Error wrapping in service errors ---

func TestChallengeHandler_RunChallenge_WrappedError(t *testing.T) {
	_, mock, router := setupDedicatedChallengeTest()
	innerErr := errors.New("connection refused")
	wrappedErr := fmt.Errorf("SMB client: %w", innerErr)
	mock.runChallengeFunc = func(ctx context.Context, id string) (*challenge.Result, error) {
		return nil, wrappedErr
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/challenges/ch-001/run", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, false, response["success"])
	assert.Equal(t, "Failed to run challenge", response["error"])
	assert.NotContains(t, response, "details", "PX-F23: internal error text must not reach the client")
}

func TestChallengeHandler_RunAll_WrappedError(t *testing.T) {
	_, mock, router := setupDedicatedChallengeTest()
	mock.runAllFunc = func(ctx context.Context) ([]*challenge.Result, error) {
		return nil, fmt.Errorf("runner: %w", errors.New("already running"))
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/challenges/run/all", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.NotContains(t, response, "details", "PX-F23: internal error text must not reach the client")
}

func TestChallengeHandler_RunByCategory_WrappedError(t *testing.T) {
	_, mock, router := setupDedicatedChallengeTest()
	mock.runByCategoryFunc = func(ctx context.Context, category string) ([]*challenge.Result, error) {
		return nil, fmt.Errorf("category %s: %w", category, errors.New("no challenges registered"))
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/challenges/run/category/missing", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.NotContains(t, response, "details", "PX-F23: internal error text must not reach the client")
}

// --- GetChallenge with multiple matching IDs (first match wins) ---

func TestChallengeHandler_GetChallenge_FirstMatchWins(t *testing.T) {
	_, mock, router := setupDedicatedChallengeTest()
	mock.listChallengesFunc = func() []services.ChallengeSummary {
		return []services.ChallengeSummary{
			{ID: "ch-001", Name: "First Match", Description: "Should be returned", Category: "test", Dependencies: []string{}},
			{ID: "ch-001", Name: "Duplicate", Description: "Should not be returned", Category: "test", Dependencies: []string{}},
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/challenges/ch-001", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response struct {
		Success bool                      `json:"success"`
		Data    services.ChallengeSummary `json:"data"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "First Match", response.Data.Name)
	assert.Equal(t, "Should be returned", response.Data.Description)
}

// --- ListChallenges with dependencies chain ---

func TestChallengeHandler_ListChallenges_DependencyChain(t *testing.T) {
	_, mock, router := setupDedicatedChallengeTest()
	mock.listChallengesFunc = func() []services.ChallengeSummary {
		return []services.ChallengeSummary{
			{ID: "ch-001", Name: "Base", Description: "No deps", Category: "setup", Dependencies: []string{}},
			{ID: "ch-002", Name: "Level 1", Description: "Depends on base", Category: "scan", Dependencies: []string{"ch-001"}},
			{ID: "ch-003", Name: "Level 2", Description: "Depends on L1", Category: "scan", Dependencies: []string{"ch-001", "ch-002"}},
			{ID: "ch-004", Name: "Level 3", Description: "Depends on all", Category: "verify", Dependencies: []string{"ch-001", "ch-002", "ch-003"}},
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/challenges", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response struct {
		Success bool                        `json:"success"`
		Data    []services.ChallengeSummary `json:"data"`
		Count   int                         `json:"count"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, 4, response.Count)

	// Verify dependency chain is preserved in response
	assert.Empty(t, response.Data[0].Dependencies)
	assert.Equal(t, []string{"ch-001"}, response.Data[1].Dependencies)
	assert.Equal(t, []string{"ch-001", "ch-002"}, response.Data[2].Dependencies)
	assert.Equal(t, []string{"ch-001", "ch-002", "ch-003"}, response.Data[3].Dependencies)
}

// --- RunAll with single result ---

func TestChallengeHandler_RunAll_SingleResult(t *testing.T) {
	_, mock, router := setupDedicatedChallengeTest()
	mock.runAllFunc = func(ctx context.Context) ([]*challenge.Result, error) {
		return []*challenge.Result{
			makeResult("ch-001", "Only One", challenge.StatusPassed),
		}, nil
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/challenges/run/all", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	summary := response["summary"].(map[string]interface{})
	assert.Equal(t, float64(1), summary["total"])
	assert.Equal(t, float64(1), summary["passed"])
	assert.Equal(t, float64(0), summary["failed"])

	data := response["data"].([]interface{})
	assert.Len(t, data, 1)
}

// --- Response JSON structure completeness ---

func TestChallengeHandler_ListChallenges_ResponseHasAllKeys(t *testing.T) {
	_, mock, router := setupDedicatedChallengeTest()
	mock.listChallengesFunc = func() []services.ChallengeSummary {
		return []services.ChallengeSummary{
			{ID: "ch-001", Name: "Test", Description: "D", Category: "c", Dependencies: []string{}},
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/challenges", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	// Must have exactly these top-level keys
	assert.Contains(t, response, "success")
	assert.Contains(t, response, "data")
	assert.Contains(t, response, "count")
	assert.Len(t, response, 3)
}

func TestChallengeHandler_GetResults_ResponseHasAllKeys(t *testing.T) {
	_, mock, router := setupDedicatedChallengeTest()
	mock.getResultsFunc = func() []*challenge.Result {
		return []*challenge.Result{}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/challenges/results", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Contains(t, response, "success")
	assert.Contains(t, response, "data")
	assert.Contains(t, response, "count")
	// FIX-QA-2026-04-21-004: GetResults also reports total_count so
	// clients can tell when the limit truncated the list.
	assert.Contains(t, response, "total_count")
	assert.Len(t, response, 4)
}

// TestChallengeHandler_RunAll_CtxInheritsValuesButSurvivesRequestCancel
// is the regression test for FIX-QA-2026-04-21-008. RunAll is
// long-running by design (508 challenges × seconds each) and must
// not cancel when the outer RequestTimeout middleware or client
// disconnect fires. But it DOES need to inherit the request-scoped
// values (request_id, trace IDs, auth subject) so downstream logging
// stays coherent. The contract is:
//
//   - RunAll's ctx must carry the same values the request ctx
//     carried (proved by a custom key/value).
//   - Cancelling the request ctx must NOT cancel RunAll's ctx.
//
// This is exactly what context.WithoutCancel gives us — verified here.
func TestChallengeHandler_RunAll_CtxInheritsValuesButSurvivesRequestCancel(t *testing.T) {
	_, mock, router := setupDedicatedChallengeTest()

	type ctxKey string
	const probeKey ctxKey = "u-cycle-probe"
	const probeVal = "inherited-value"

	var gotCtx context.Context
	mock.runAllFunc = func(ctx context.Context) ([]*challenge.Result, error) {
		gotCtx = ctx
		return []*challenge.Result{}, nil
	}

	// Build a request ctx carrying a probe value, then cancel it
	// BEFORE the handler finishes.
	reqCtx, cancel := context.WithCancel(context.WithValue(context.Background(), probeKey, probeVal))
	cancel() // already cancelled when request is served.

	req := httptest.NewRequest(http.MethodPost, "/api/v1/challenges/run/all", nil)
	req = req.WithContext(reqCtx)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.NotNil(t, gotCtx, "RunAll must have been invoked")

	// (a) Values inherited.
	assert.Equal(t, probeVal, gotCtx.Value(probeKey),
		"RunAll ctx must inherit values from the request ctx")

	// (b) Cancel did NOT propagate — the ctx should NOT be done even
	//     though the source request ctx was explicitly cancelled.
	select {
	case <-gotCtx.Done():
		t.Fatalf("RunAll ctx should survive a request-ctx cancel (WithoutCancel), but Done() fired: %v", gotCtx.Err())
	default:
		// good — not done.
	}
	assert.NoError(t, gotCtx.Err(), "gotCtx.Err must be nil after request ctx cancel")
}

// TestChallengeHandler_RunChallenge_ObservesClientDisconnect is the
// end-to-end regression test for DEFER-QA-2026-04-21-001 #5. It fires
// a POST /:id/run whose request context is cancelled mid-flight and
// asserts the service's ctx.Done channel fires before the mock
// returns — proving the handler + service layer honour client
// disconnect, not just that the deadline-carrying ctx was passed.
//
// Paired with TestChallengeHandler_RunChallenge_PropagatesRequestContext:
// that test proves the ctx is RECEIVED; this one proves it's
// ACTIONABLE.
func TestChallengeHandler_RunChallenge_ObservesClientDisconnect(t *testing.T) {
	_, mock, router := setupDedicatedChallengeTest()

	// Mock blocks until either ctx.Done fires (good) or a 5s timeout
	// elapses (the client disconnect never propagated → bug).
	type observation struct {
		ctxDone   bool
		ctxErr    error
		elapsedMs int64
	}
	obsCh := make(chan observation, 1)

	mock.runChallengeFunc = func(ctx context.Context, id string) (*challenge.Result, error) {
		start := time.Now()
		select {
		case <-ctx.Done():
			obsCh <- observation{ctxDone: true, ctxErr: ctx.Err(),
				elapsedMs: time.Since(start).Milliseconds()}
			return nil, ctx.Err()
		case <-time.After(5 * time.Second):
			obsCh <- observation{ctxDone: false,
				elapsedMs: time.Since(start).Milliseconds()}
			return makeResult(id, "timeout", challenge.StatusFailed), nil
		}
	}

	// Fire the request with a ctx we cancel after 100 ms — simulates
	// a curl timeout or closed TCP.
	reqCtx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodPost, "/api/v1/challenges/slow-ch/run", nil)
	req = req.WithContext(reqCtx)
	w := httptest.NewRecorder()

	var serveErr error
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer func() {
			if r := recover(); r != nil {
				serveErr = fmt.Errorf("ServeHTTP panic: %v", r)
			}
		}()
		router.ServeHTTP(w, req)
	}()

	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case obs := <-obsCh:
		require.True(t, obs.ctxDone,
			"mock must see ctx.Done within the test budget — handler did not propagate the client disconnect (elapsed=%dms)", obs.elapsedMs)
		assert.ErrorIs(t, obs.ctxErr, context.Canceled,
			"ctx.Err must be Canceled, got %v", obs.ctxErr)
		assert.Less(t, obs.elapsedMs, int64(500),
			"disconnect should propagate fast, not after the 5s safety timeout")
	case <-time.After(6 * time.Second):
		t.Fatalf("mock never observed ctx.Done; serve err: %v", serveErr)
	}
	<-done
}

// TestChallengeHandler_RunChallenge_PropagatesRequestContext is the
// regression test for FIX-QA-2026-04-21-005. Before the handler was
// updated to pass c.Request.Context() (it used to pass
// context.Background()), a curl that disconnected mid-challenge left
// the challenge running server-side — the concurrency limiter then
// accumulated zombie handlers and the whole /challenges/* surface
// appeared to hang (DEFER-QA-2026-04-21-001).
//
// The mock asserts that the ctx received by RunChallenge is the
// request's ctx (not a background one) by checking its deadline and
// Done channel behaviour.
func TestChallengeHandler_RunChallenge_PropagatesRequestContext(t *testing.T) {
	_, mock, router := setupDedicatedChallengeTest()

	var gotCtx context.Context
	mock.runChallengeFunc = func(ctx context.Context, id string) (*challenge.Result, error) {
		gotCtx = ctx
		return makeResult(id, "Ctx propagation test", challenge.StatusPassed), nil
	}

	// Build a request whose context carries a deadline — if the
	// handler was using context.Background(), the mock would see a
	// ctx without a deadline.
	deadline := time.Now().Add(5 * time.Second)
	reqCtx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/challenges/ch-001/run", nil)
	req = req.WithContext(reqCtx)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.NotNil(t, gotCtx, "RunChallenge must have been invoked with a ctx")

	gotDeadline, hasDeadline := gotCtx.Deadline()
	assert.True(t, hasDeadline, "handler must propagate request ctx (which has a deadline), not background")
	assert.WithinDuration(t, deadline, gotDeadline, 50*time.Millisecond)
}

// TestChallengeHandler_GetResults_LimitTruncates is the regression test
// for FIX-QA-2026-04-21-004. Without the limit, a 508-challenge RunAll
// followed by GET /challenges/results serialised tens of MB of embedded
// assertion output through gin's JSON encoder and appeared to "hang"
// past the 60s RequestTimeout. GetResults now defaults to the last 100
// entries and exposes total_count so callers can page.
func TestChallengeHandler_GetResults_LimitTruncates(t *testing.T) {
	_, mock, router := setupDedicatedChallengeTest()

	// Fake 250 stored results; default limit is 100.
	all := make([]*challenge.Result, 250)
	for i := range all {
		all[i] = &challenge.Result{ChallengeID: challenge.ID("ch-" + strconv.Itoa(i)), Status: challenge.StatusPassed}
	}
	mock.getResultsFunc = func() []*challenge.Result { return all }

	// Default: returns the last 100, reports total_count=250.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/challenges/results", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, float64(100), resp["count"], "default limit = 100")
	assert.Equal(t, float64(250), resp["total_count"])

	// Explicit limit=10: returns last 10.
	req = httptest.NewRequest(http.MethodGet, "/api/v1/challenges/results?limit=10", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, float64(10), resp["count"])
	assert.Equal(t, float64(250), resp["total_count"])

	// limit=0: unlimited, returns all.
	req = httptest.NewRequest(http.MethodGet, "/api/v1/challenges/results?limit=0", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, float64(250), resp["count"])
	assert.Equal(t, float64(250), resp["total_count"])
}

func TestChallengeHandler_RunAll_ResponseHasAllKeys(t *testing.T) {
	_, mock, router := setupDedicatedChallengeTest()
	mock.runAllFunc = func(ctx context.Context) ([]*challenge.Result, error) {
		return []*challenge.Result{}, nil
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/challenges/run/all", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Contains(t, response, "success")
	assert.Contains(t, response, "data")
	assert.Contains(t, response, "summary")
	assert.Len(t, response, 3)

	summary := response["summary"].(map[string]interface{})
	assert.Contains(t, summary, "total")
	assert.Contains(t, summary, "passed")
	assert.Contains(t, summary, "failed")
	assert.Len(t, summary, 3)
}

func TestChallengeHandler_RunByCategory_ResponseHasAllKeys(t *testing.T) {
	_, mock, router := setupDedicatedChallengeTest()
	mock.runByCategoryFunc = func(ctx context.Context, category string) ([]*challenge.Result, error) {
		return []*challenge.Result{}, nil
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/challenges/run/category/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Contains(t, response, "success")
	assert.Contains(t, response, "data")
	assert.Contains(t, response, "summary")
	assert.Len(t, response, 3)

	summary := response["summary"].(map[string]interface{})
	assert.Contains(t, summary, "total")
	assert.Contains(t, summary, "passed")
	assert.Contains(t, summary, "failed")
	assert.Len(t, summary, 3)
}

// --- GetChallenge response has exactly two keys ---

func TestChallengeHandler_GetChallenge_SuccessResponseHasTwoKeys(t *testing.T) {
	_, mock, router := setupDedicatedChallengeTest()
	mock.listChallengesFunc = func() []services.ChallengeSummary {
		return []services.ChallengeSummary{
			{ID: "ch-001", Name: "Test", Description: "D", Category: "c", Dependencies: []string{}},
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/challenges/ch-001", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response, "success")
	assert.Contains(t, response, "data")
	assert.Len(t, response, 2)
}

func TestChallengeHandler_GetChallenge_NotFoundResponseHasTwoKeys(t *testing.T) {
	_, mock, router := setupDedicatedChallengeTest()
	mock.listChallengesFunc = func() []services.ChallengeSummary {
		return []services.ChallengeSummary{}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/challenges/ch-missing", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response, "success")
	assert.Contains(t, response, "error")
	assert.Len(t, response, 2)
}

// --- RunChallenge response has exactly two keys on success ---

func TestChallengeHandler_RunChallenge_SuccessResponseHasTwoKeys(t *testing.T) {
	_, mock, router := setupDedicatedChallengeTest()
	mock.runChallengeFunc = func(ctx context.Context, id string) (*challenge.Result, error) {
		return makeResult(id, "Test", challenge.StatusPassed), nil
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/challenges/ch-001/run", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response, "success")
	assert.Contains(t, response, "data")
	assert.Len(t, response, 2)
}

// --- Multiple sequential calls return independent results ---

func TestChallengeHandler_GetResults_SequentialCallsAreIndependent(t *testing.T) {
	_, mock, router := setupDedicatedChallengeTest()
	callCount := 0
	mock.getResultsFunc = func() []*challenge.Result {
		callCount++
		results := make([]*challenge.Result, callCount)
		for i := 0; i < callCount; i++ {
			results[i] = makeResult(
				fmt.Sprintf("ch-%03d", i+1),
				fmt.Sprintf("C%d", i+1),
				challenge.StatusPassed,
			)
		}
		return results
	}

	// First call: 1 result
	req := httptest.NewRequest(http.MethodGet, "/api/v1/challenges/results", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var response1 map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response1)
	require.NoError(t, err)
	assert.Equal(t, float64(1), response1["count"])

	// Second call: 2 results
	req = httptest.NewRequest(http.MethodGet, "/api/v1/challenges/results", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var response2 map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response2)
	require.NoError(t, err)
	assert.Equal(t, float64(2), response2["count"])
}

// --- RunChallenge with empty ID via direct handler call ---

func TestChallengeHandler_RunByCategory_EmptyCategory_Direct(t *testing.T) {
	handler, _, _ := setupDedicatedChallengeTest()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = []gin.Param{{Key: "category", Value: ""}}
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/challenges/run/category/", nil)

	handler.RunByCategory(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, false, response["success"])
	assert.Contains(t, response["error"], "Category is required")
}

func TestChallengeHandler_RunChallenge_EmptyID_Direct(t *testing.T) {
	handler, _, _ := setupDedicatedChallengeTest()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = []gin.Param{{Key: "id", Value: ""}}
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/challenges//run", nil)

	handler.RunChallenge(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, false, response["success"])
	assert.Contains(t, response["error"], "Challenge ID is required")
}

func TestChallengeHandler_GetChallenge_EmptyID_Direct(t *testing.T) {
	handler, _, _ := setupDedicatedChallengeTest()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = []gin.Param{{Key: "id", Value: ""}}
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/challenges/", nil)

	handler.GetChallenge(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, false, response["success"])
	assert.Contains(t, response["error"], "Challenge ID is required")
}

// --- Verify RunChallenge and RunAll return correct JSON content type ---

func TestChallengeHandler_RunChallenge_ContentType(t *testing.T) {
	_, mock, router := setupDedicatedChallengeTest()
	mock.runChallengeFunc = func(ctx context.Context, id string) (*challenge.Result, error) {
		return makeResult(id, "Test", challenge.StatusPassed), nil
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/challenges/ch-001/run", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
}

func TestChallengeHandler_GetChallenge_ContentType(t *testing.T) {
	_, mock, router := setupDedicatedChallengeTest()
	mock.listChallengesFunc = func() []services.ChallengeSummary {
		return []services.ChallengeSummary{
			{ID: "ch-001", Name: "T", Description: "D", Category: "c", Dependencies: []string{}},
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/challenges/ch-001", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
}

func TestChallengeHandler_GetChallenge_NotFound_ContentType(t *testing.T) {
	_, mock, router := setupDedicatedChallengeTest()
	mock.listChallengesFunc = func() []services.ChallengeSummary {
		return []services.ChallengeSummary{}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/challenges/ch-missing", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
}

func TestChallengeHandler_RunChallenge_Error_ContentType(t *testing.T) {
	_, mock, router := setupDedicatedChallengeTest()
	mock.runChallengeFunc = func(ctx context.Context, id string) (*challenge.Result, error) {
		return nil, errors.New("test error")
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/challenges/ch-001/run", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
}
