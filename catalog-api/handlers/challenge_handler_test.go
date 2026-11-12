package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"catalogizer/services"
	"digital.vasic.challenges/pkg/challenge"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// mockChallengeService implements challengeService for testing.
type mockChallengeService struct {
	listChallengesFunc func() []services.ChallengeSummary
	runChallengeFunc   func(ctx context.Context, id string) (*challenge.Result, error)
	runAllFunc         func(ctx context.Context) ([]*challenge.Result, error)
	runByCategoryFunc  func(ctx context.Context, category string) ([]*challenge.Result, error)
	getResultsFunc     func() []*challenge.Result
}

func (m *mockChallengeService) ListChallenges() []services.ChallengeSummary {
	if m.listChallengesFunc != nil {
		return m.listChallengesFunc()
	}
	return nil
}

func (m *mockChallengeService) RunChallenge(ctx context.Context, id string) (*challenge.Result, error) {
	if m.runChallengeFunc != nil {
		return m.runChallengeFunc(ctx, id)
	}
	return nil, nil
}

func (m *mockChallengeService) RunAll(ctx context.Context) ([]*challenge.Result, error) {
	if m.runAllFunc != nil {
		return m.runAllFunc(ctx)
	}
	return nil, nil
}

func (m *mockChallengeService) RunByCategory(ctx context.Context, category string) ([]*challenge.Result, error) {
	if m.runByCategoryFunc != nil {
		return m.runByCategoryFunc(ctx, category)
	}
	return nil, nil
}

func (m *mockChallengeService) RunAllStreaming(ctx context.Context) <-chan *challenge.Result {
	ch := make(chan *challenge.Result)
	close(ch)
	return ch
}

func (m *mockChallengeService) RunByCategoryStreaming(ctx context.Context, category string) <-chan *challenge.Result {
	ch := make(chan *challenge.Result)
	close(ch)
	return ch
}

func (m *mockChallengeService) GetResults() []*challenge.Result {
	if m.getResultsFunc != nil {
		return m.getResultsFunc()
	}
	return nil
}

// setupChallengeTest creates a handler with a fresh mock and gin router.
func setupChallengeTest() (*ChallengeHandler, *mockChallengeService, *gin.Engine) {
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

// makeResult is a helper to build a challenge.Result with minimal fields.
func makeResult(id string, name string, status string) *challenge.Result {
	return &challenge.Result{
		ChallengeID:   challenge.ID(id),
		ChallengeName: name,
		Status:        status,
		StartTime:     time.Now().Add(-time.Second),
		EndTime:       time.Now(),
		Duration:      time.Second,
	}
}

// --- NewChallengeHandler ---

func TestChallengeHandler_NewChallengeHandler(t *testing.T) {
	t.Run("creates handler with service", func(t *testing.T) {
		mock := &mockChallengeService{}
		handler := NewChallengeHandler(mock)
		assert.NotNil(t, handler)
		assert.NotNil(t, handler.service)
	})

	t.Run("creates handler with nil service", func(t *testing.T) {
		handler := NewChallengeHandler(nil)
		assert.NotNil(t, handler)
		assert.Nil(t, handler.service)
	})
}

// --- ListChallenges ---

func TestChallengeHandler_ListChallenges(t *testing.T) {
	tests := []struct {
		name            string
		challenges      []services.ChallengeSummary
		expectedCount   int
		expectedSuccess bool
	}{
		{
			name: "returns multiple challenges",
			challenges: []services.ChallengeSummary{
				{ID: "ch-001", Name: "Challenge One", Description: "First", Category: "setup", Dependencies: []string{}},
				{ID: "ch-002", Name: "Challenge Two", Description: "Second", Category: "scan", Dependencies: []string{"ch-001"}},
				{ID: "ch-003", Name: "Challenge Three", Description: "Third", Category: "scan", Dependencies: []string{"ch-001", "ch-002"}},
			},
			expectedCount:   3,
			expectedSuccess: true,
		},
		{
			name:            "returns empty list",
			challenges:      []services.ChallengeSummary{},
			expectedCount:   0,
			expectedSuccess: true,
		},
		{
			name:            "returns nil list",
			challenges:      nil,
			expectedCount:   0,
			expectedSuccess: true,
		},
		{
			name: "single challenge",
			challenges: []services.ChallengeSummary{
				{ID: "ch-001", Name: "Only One", Description: "Solo", Category: "test", Dependencies: []string{}},
			},
			expectedCount:   1,
			expectedSuccess: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, mock, router := setupChallengeTest()
			mock.listChallengesFunc = func() []services.ChallengeSummary {
				return tt.challenges
			}

			req := httptest.NewRequest(http.MethodGet, "/api/v1/challenges", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedSuccess, response["success"])
			assert.Equal(t, float64(tt.expectedCount), response["count"])

			data, ok := response["data"].([]interface{})
			if tt.challenges == nil {
				// nil serializes to JSON null
				assert.Nil(t, response["data"])
			} else {
				assert.True(t, ok)
				assert.Len(t, data, tt.expectedCount)
			}
		})
	}
}

func TestChallengeHandler_ListChallenges_VerifiesFields(t *testing.T) {
	_, mock, router := setupChallengeTest()
	mock.listChallengesFunc = func() []services.ChallengeSummary {
		return []services.ChallengeSummary{
			{
				ID:           "ch-010",
				Name:         "Full Field Check",
				Description:  "Verifies all fields serialize",
				Category:     "integration",
				Dependencies: []string{"ch-001", "ch-005"},
			},
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
	assert.NoError(t, err)
	assert.True(t, response.Success)
	assert.Equal(t, 1, response.Count)
	assert.Equal(t, "ch-010", response.Data[0].ID)
	assert.Equal(t, "Full Field Check", response.Data[0].Name)
	assert.Equal(t, "Verifies all fields serialize", response.Data[0].Description)
	assert.Equal(t, "integration", response.Data[0].Category)
	assert.Equal(t, []string{"ch-001", "ch-005"}, response.Data[0].Dependencies)
}

// --- GetChallenge ---

func TestChallengeHandler_GetChallenge(t *testing.T) {
	tests := []struct {
		name           string
		challengeID    string
		challenges     []services.ChallengeSummary
		expectedStatus int
		expectSuccess  bool
		expectFound    bool
	}{
		{
			name:        "found by ID",
			challengeID: "ch-001",
			challenges: []services.ChallengeSummary{
				{ID: "ch-001", Name: "First", Description: "D1", Category: "test", Dependencies: []string{}},
				{ID: "ch-002", Name: "Second", Description: "D2", Category: "test", Dependencies: []string{}},
			},
			expectedStatus: http.StatusOK,
			expectSuccess:  true,
			expectFound:    true,
		},
		{
			name:        "found last in list",
			challengeID: "ch-003",
			challenges: []services.ChallengeSummary{
				{ID: "ch-001", Name: "First", Description: "D1", Category: "test", Dependencies: []string{}},
				{ID: "ch-002", Name: "Second", Description: "D2", Category: "test", Dependencies: []string{}},
				{ID: "ch-003", Name: "Third", Description: "D3", Category: "scan", Dependencies: []string{}},
			},
			expectedStatus: http.StatusOK,
			expectSuccess:  true,
			expectFound:    true,
		},
		{
			name:        "not found",
			challengeID: "ch-999",
			challenges: []services.ChallengeSummary{
				{ID: "ch-001", Name: "First", Description: "D1", Category: "test", Dependencies: []string{}},
			},
			expectedStatus: http.StatusNotFound,
			expectSuccess:  false,
			expectFound:    false,
		},
		{
			name:           "not found in empty list",
			challengeID:    "ch-001",
			challenges:     []services.ChallengeSummary{},
			expectedStatus: http.StatusNotFound,
			expectSuccess:  false,
			expectFound:    false,
		},
		{
			name:           "not found when service returns nil",
			challengeID:    "ch-001",
			challenges:     nil,
			expectedStatus: http.StatusNotFound,
			expectSuccess:  false,
			expectFound:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, mock, router := setupChallengeTest()
			mock.listChallengesFunc = func() []services.ChallengeSummary {
				return tt.challenges
			}

			req := httptest.NewRequest(http.MethodGet, "/api/v1/challenges/"+tt.challengeID, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectSuccess, response["success"])

			if tt.expectFound {
				data, ok := response["data"].(map[string]interface{})
				assert.True(t, ok)
				assert.Equal(t, tt.challengeID, data["id"])
			}
		})
	}
}

func TestChallengeHandler_GetChallenge_EmptyID(t *testing.T) {
	handler, _, _ := setupChallengeTest()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = []gin.Param{{Key: "id", Value: ""}}
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/challenges/", nil)

	handler.GetChallenge(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, false, response["success"])
	assert.Contains(t, response["error"], "Challenge ID is required")
}

func TestChallengeHandler_GetChallenge_MissingIDParam(t *testing.T) {
	handler, _, _ := setupChallengeTest()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = []gin.Param{} // no id param at all
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/challenges/", nil)

	handler.GetChallenge(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// --- RunChallenge ---

func TestChallengeHandler_RunChallenge(t *testing.T) {
	tests := []struct {
		name           string
		challengeID    string
		result         *challenge.Result
		serviceErr     error
		expectedStatus int
		expectSuccess  bool
	}{
		{
			name:           "success - passed",
			challengeID:    "ch-001",
			result:         makeResult("ch-001", "Test Challenge", challenge.StatusPassed),
			serviceErr:     nil,
			expectedStatus: http.StatusOK,
			expectSuccess:  true,
		},
		{
			name:           "success - failed challenge result",
			challengeID:    "ch-002",
			result:         makeResult("ch-002", "Failing Challenge", challenge.StatusFailed),
			serviceErr:     nil,
			expectedStatus: http.StatusOK,
			expectSuccess:  true,
		},
		{
			name:           "service error",
			challengeID:    "ch-001",
			result:         nil,
			serviceErr:     errors.New("challenge not found"),
			expectedStatus: http.StatusInternalServerError,
			expectSuccess:  false,
		},
		{
			name:           "service returns error with timeout",
			challengeID:    "ch-003",
			result:         nil,
			serviceErr:     errors.New("challenge execution timed out"),
			expectedStatus: http.StatusInternalServerError,
			expectSuccess:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, mock, router := setupChallengeTest()
			capturedID := ""
			mock.runChallengeFunc = func(ctx context.Context, id string) (*challenge.Result, error) {
				capturedID = id
				return tt.result, tt.serviceErr
			}

			req := httptest.NewRequest(http.MethodPost, "/api/v1/challenges/"+tt.challengeID+"/run", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.Equal(t, tt.challengeID, capturedID)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectSuccess, response["success"])

			if tt.expectSuccess && tt.result != nil {
				data, ok := response["data"].(map[string]interface{})
				assert.True(t, ok)
				assert.Equal(t, tt.challengeID, data["challenge_id"])
				assert.Equal(t, tt.result.Status, data["status"])
			}

			if !tt.expectSuccess && tt.serviceErr != nil {
				assert.Contains(t, response["error"], "Failed to run challenge")
			}
		})
	}
}

func TestChallengeHandler_RunChallenge_EmptyID(t *testing.T) {
	handler, _, _ := setupChallengeTest()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = []gin.Param{}
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/challenges//run", nil)

	handler.RunChallenge(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, false, response["success"])
	assert.Contains(t, response["error"], "Challenge ID is required")
}

// --- RunAll ---

func TestChallengeHandler_RunAll(t *testing.T) {
	tests := []struct {
		name           string
		results        []*challenge.Result
		serviceErr     error
		expectedStatus int
		expectSuccess  bool
		expectedTotal  int
		expectedPassed int
		expectedFailed int
	}{
		{
			name: "all passed",
			results: []*challenge.Result{
				makeResult("ch-001", "C1", challenge.StatusPassed),
				makeResult("ch-002", "C2", challenge.StatusPassed),
				makeResult("ch-003", "C3", challenge.StatusPassed),
			},
			serviceErr:     nil,
			expectedStatus: http.StatusOK,
			expectSuccess:  true,
			expectedTotal:  3,
			expectedPassed: 3,
			expectedFailed: 0,
		},
		{
			name: "all failed",
			results: []*challenge.Result{
				makeResult("ch-001", "C1", challenge.StatusFailed),
				makeResult("ch-002", "C2", challenge.StatusFailed),
			},
			serviceErr:     nil,
			expectedStatus: http.StatusOK,
			expectSuccess:  true,
			expectedTotal:  2,
			expectedPassed: 0,
			expectedFailed: 2,
		},
		{
			name: "mixed results",
			results: []*challenge.Result{
				makeResult("ch-001", "C1", challenge.StatusPassed),
				makeResult("ch-002", "C2", challenge.StatusFailed),
				makeResult("ch-003", "C3", challenge.StatusPassed),
				makeResult("ch-004", "C4", challenge.StatusSkipped),
			},
			serviceErr:     nil,
			expectedStatus: http.StatusOK,
			expectSuccess:  true,
			expectedTotal:  4,
			expectedPassed: 2,
			expectedFailed: 2, // failed + skipped both count as non-passed
		},
		{
			name:           "empty results",
			results:        []*challenge.Result{},
			serviceErr:     nil,
			expectedStatus: http.StatusOK,
			expectSuccess:  true,
			expectedTotal:  0,
			expectedPassed: 0,
			expectedFailed: 0,
		},
		{
			name:           "service error",
			results:        nil,
			serviceErr:     errors.New("runner busy"),
			expectedStatus: http.StatusInternalServerError,
			expectSuccess:  false,
		},
		{
			name: "timed out and error statuses count as failed",
			results: []*challenge.Result{
				makeResult("ch-001", "C1", challenge.StatusPassed),
				makeResult("ch-002", "C2", challenge.StatusTimedOut),
				makeResult("ch-003", "C3", challenge.StatusError),
				makeResult("ch-004", "C4", challenge.StatusStuck),
			},
			serviceErr:     nil,
			expectedStatus: http.StatusOK,
			expectSuccess:  true,
			expectedTotal:  4,
			expectedPassed: 1,
			expectedFailed: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, mock, router := setupChallengeTest()
			mock.runAllFunc = func(ctx context.Context) ([]*challenge.Result, error) {
				return tt.results, tt.serviceErr
			}

			req := httptest.NewRequest(http.MethodPost, "/api/v1/challenges/run/all", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectSuccess, response["success"])

			if tt.expectSuccess {
				summary, ok := response["summary"].(map[string]interface{})
				assert.True(t, ok, "expected summary in response")
				assert.Equal(t, float64(tt.expectedTotal), summary["total"])
				assert.Equal(t, float64(tt.expectedPassed), summary["passed"])
				assert.Equal(t, float64(tt.expectedFailed), summary["failed"])
			}

			if !tt.expectSuccess && tt.serviceErr != nil {
				assert.Contains(t, response["error"], "Failed to run challenges")
			}
		})
	}
}

// --- RunByCategory ---

func TestChallengeHandler_RunByCategory(t *testing.T) {
	tests := []struct {
		name           string
		category       string
		results        []*challenge.Result
		serviceErr     error
		expectedStatus int
		expectSuccess  bool
		expectedTotal  int
		expectedPassed int
		expectedFailed int
	}{
		{
			name:     "all passed in category",
			category: "setup",
			results: []*challenge.Result{
				makeResult("ch-001", "Setup 1", challenge.StatusPassed),
				makeResult("ch-002", "Setup 2", challenge.StatusPassed),
			},
			serviceErr:     nil,
			expectedStatus: http.StatusOK,
			expectSuccess:  true,
			expectedTotal:  2,
			expectedPassed: 2,
			expectedFailed: 0,
		},
		{
			name:     "mixed results in category",
			category: "scan",
			results: []*challenge.Result{
				makeResult("ch-010", "Scan 1", challenge.StatusPassed),
				makeResult("ch-011", "Scan 2", challenge.StatusFailed),
			},
			serviceErr:     nil,
			expectedStatus: http.StatusOK,
			expectSuccess:  true,
			expectedTotal:  2,
			expectedPassed: 1,
			expectedFailed: 1,
		},
		{
			name:           "empty results for category",
			category:       "nonexistent",
			results:        []*challenge.Result{},
			serviceErr:     nil,
			expectedStatus: http.StatusOK,
			expectSuccess:  true,
			expectedTotal:  0,
			expectedPassed: 0,
			expectedFailed: 0,
		},
		{
			name:           "service error for category",
			category:       "broken",
			results:        nil,
			serviceErr:     errors.New("category not found"),
			expectedStatus: http.StatusInternalServerError,
			expectSuccess:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, mock, router := setupChallengeTest()
			capturedCategory := ""
			mock.runByCategoryFunc = func(ctx context.Context, category string) ([]*challenge.Result, error) {
				capturedCategory = category
				return tt.results, tt.serviceErr
			}

			req := httptest.NewRequest(http.MethodPost, "/api/v1/challenges/run/category/"+tt.category, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.Equal(t, tt.category, capturedCategory)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectSuccess, response["success"])

			if tt.expectSuccess {
				summary, ok := response["summary"].(map[string]interface{})
				assert.True(t, ok, "expected summary in response")
				assert.Equal(t, float64(tt.expectedTotal), summary["total"])
				assert.Equal(t, float64(tt.expectedPassed), summary["passed"])
				assert.Equal(t, float64(tt.expectedFailed), summary["failed"])
			}

			if !tt.expectSuccess && tt.serviceErr != nil {
				assert.Contains(t, response["error"], "Failed to run challenges")
			}
		})
	}
}

func TestChallengeHandler_RunByCategory_EmptyCategory(t *testing.T) {
	handler, _, _ := setupChallengeTest()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = []gin.Param{}
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/challenges/run/category/", nil)

	handler.RunByCategory(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, false, response["success"])
	assert.Contains(t, response["error"], "Category is required")
}

func TestChallengeHandler_RunByCategory_CategoryWithSpecialChars(t *testing.T) {
	_, mock, router := setupChallengeTest()
	capturedCategory := ""
	mock.runByCategoryFunc = func(ctx context.Context, category string) ([]*challenge.Result, error) {
		capturedCategory = category
		return []*challenge.Result{}, nil
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/challenges/run/category/my-category_v2", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "my-category_v2", capturedCategory)
}

// --- GetResults ---

func TestChallengeHandler_GetResults_TableDriven(t *testing.T) {
	tests := []struct {
		name          string
		results       []*challenge.Result
		expectedCount int
	}{
		{
			name: "returns stored results",
			results: []*challenge.Result{
				makeResult("ch-001", "C1", challenge.StatusPassed),
				makeResult("ch-002", "C2", challenge.StatusFailed),
				makeResult("ch-003", "C3", challenge.StatusSkipped),
			},
			expectedCount: 3,
		},
		{
			name:          "returns empty results",
			results:       []*challenge.Result{},
			expectedCount: 0,
		},
		{
			name:          "returns nil results",
			results:       nil,
			expectedCount: 0,
		},
		{
			name: "single result",
			results: []*challenge.Result{
				makeResult("ch-050", "Final", challenge.StatusPassed),
			},
			expectedCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, mock, router := setupChallengeTest()
			mock.getResultsFunc = func() []*challenge.Result {
				return tt.results
			}

			req := httptest.NewRequest(http.MethodGet, "/api/v1/challenges/results", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			assert.NoError(t, err)
			assert.Equal(t, true, response["success"])
			assert.Equal(t, float64(tt.expectedCount), response["count"])
		})
	}
}

func TestChallengeHandler_GetResults_VerifiesResultFields(t *testing.T) {
	_, mock, router := setupChallengeTest()
	now := time.Now()
	mock.getResultsFunc = func() []*challenge.Result {
		return []*challenge.Result{
			{
				ChallengeID:   "ch-001",
				ChallengeName: "Detailed Result",
				Status:        challenge.StatusPassed,
				StartTime:     now.Add(-5 * time.Second),
				EndTime:       now,
				Duration:      5 * time.Second,
				Error:         "",
			},
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
	assert.NoError(t, err)
	assert.True(t, response.Success)
	assert.Equal(t, 1, response.Count)
	assert.Equal(t, challenge.ID("ch-001"), response.Data[0].ChallengeID)
	assert.Equal(t, "Detailed Result", response.Data[0].ChallengeName)
	assert.Equal(t, challenge.StatusPassed, response.Data[0].Status)
}

// --- RunChallenge with result containing assertions ---

func TestChallengeHandler_RunChallenge_WithAssertions(t *testing.T) {
	_, mock, router := setupChallengeTest()
	mock.runChallengeFunc = func(ctx context.Context, id string) (*challenge.Result, error) {
		return &challenge.Result{
			ChallengeID:   challenge.ID(id),
			ChallengeName: "Assertion Challenge",
			Status:        challenge.StatusPassed,
			StartTime:     time.Now().Add(-time.Second),
			EndTime:       time.Now(),
			Duration:      time.Second,
			Assertions: []challenge.AssertionResult{
				{Type: "not_empty", Target: "output", Expected: true, Actual: true, Passed: true, Message: "output is not empty"},
				{Type: "contains", Target: "body", Expected: "hello", Actual: "hello world", Passed: true, Message: "body contains hello"},
			},
		}, nil
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/challenges/ch-005/run", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, true, response["success"])

	data := response["data"].(map[string]interface{})
	assertions := data["assertions"].([]interface{})
	assert.Len(t, assertions, 2)
}

// --- RunAll summary counting edge cases ---

func TestChallengeHandler_RunAll_OnlyPendingResults(t *testing.T) {
	_, mock, router := setupChallengeTest()
	mock.runAllFunc = func(ctx context.Context) ([]*challenge.Result, error) {
		return []*challenge.Result{
			makeResult("ch-001", "Pending 1", challenge.StatusPending),
			makeResult("ch-002", "Running 1", challenge.StatusRunning),
		}, nil
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/challenges/run/all", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	summary := response["summary"].(map[string]interface{})
	assert.Equal(t, float64(2), summary["total"])
	// pending and running are not "passed", so they count as failed
	assert.Equal(t, float64(0), summary["passed"])
	assert.Equal(t, float64(2), summary["failed"])
}

// --- RunChallenge passes context through ---

func TestChallengeHandler_RunChallenge_PassesContext(t *testing.T) {
	_, mock, router := setupChallengeTest()
	contextReceived := false
	mock.runChallengeFunc = func(ctx context.Context, id string) (*challenge.Result, error) {
		if ctx != nil {
			contextReceived = true
		}
		return makeResult(id, "Ctx Test", challenge.StatusPassed), nil
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/challenges/ch-001/run", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, contextReceived, "handler should pass request context to service")
}

// --- RunAll passes context through ---

func TestChallengeHandler_RunAll_PassesContext(t *testing.T) {
	_, mock, router := setupChallengeTest()
	contextReceived := false
	mock.runAllFunc = func(ctx context.Context) ([]*challenge.Result, error) {
		if ctx != nil {
			contextReceived = true
		}
		return []*challenge.Result{}, nil
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/challenges/run/all", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, contextReceived, "handler should pass request context to service")
}

// --- Error response structure ---

func TestChallengeHandler_RunChallenge_ErrorResponseStructure(t *testing.T) {
	_, mock, router := setupChallengeTest()
	mock.runChallengeFunc = func(ctx context.Context, id string) (*challenge.Result, error) {
		return nil, errors.New("specific error detail")
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/challenges/ch-001/run", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, false, response["success"])
	assert.Equal(t, "Failed to run challenge", response["error"])
	assert.NotContains(t, response, "details", "PX-F23: internal error text must not reach the client")
}

func TestChallengeHandler_RunAll_ErrorResponseStructure(t *testing.T) {
	_, mock, router := setupChallengeTest()
	mock.runAllFunc = func(ctx context.Context) ([]*challenge.Result, error) {
		return nil, errors.New("all runner busy")
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/challenges/run/all", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, false, response["success"])
	assert.Equal(t, "Failed to run challenges", response["error"])
	assert.NotContains(t, response, "details", "PX-F23: internal error text must not reach the client")
}

func TestChallengeHandler_RunByCategory_ErrorResponseStructure(t *testing.T) {
	_, mock, router := setupChallengeTest()
	mock.runByCategoryFunc = func(ctx context.Context, category string) ([]*challenge.Result, error) {
		return nil, errors.New("category runner error")
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/challenges/run/category/setup", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, false, response["success"])
	assert.Equal(t, "Failed to run challenges", response["error"])
	assert.NotContains(t, response, "details", "PX-F23: internal error text must not reach the client")
}

// --- GetChallenge response includes correct data shape ---

func TestChallengeHandler_GetChallenge_ResponseShape(t *testing.T) {
	_, mock, router := setupChallengeTest()
	mock.listChallengesFunc = func() []services.ChallengeSummary {
		return []services.ChallengeSummary{
			{
				ID:           "ch-007",
				Name:         "Shape Test",
				Description:  "Check response shape",
				Category:     "validation",
				Dependencies: []string{"ch-001"},
			},
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/challenges/ch-007", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response struct {
		Success bool                      `json:"success"`
		Data    services.ChallengeSummary `json:"data"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.True(t, response.Success)
	assert.Equal(t, "ch-007", response.Data.ID)
	assert.Equal(t, "Shape Test", response.Data.Name)
	assert.Equal(t, "Check response shape", response.Data.Description)
	assert.Equal(t, "validation", response.Data.Category)
	assert.Equal(t, []string{"ch-001"}, response.Data.Dependencies)
}

// --- GetChallenge not-found error shape ---

func TestChallengeHandler_GetChallenge_NotFoundErrorShape(t *testing.T) {
	_, mock, router := setupChallengeTest()
	mock.listChallengesFunc = func() []services.ChallengeSummary {
		return []services.ChallengeSummary{}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/challenges/ch-missing", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, false, response["success"])
	assert.Equal(t, "Challenge not found", response["error"])
}

// --- Content-Type header ---

func TestChallengeHandler_ResponseContentType(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
	}{
		{"list challenges", http.MethodGet, "/api/v1/challenges"},
		{"get results", http.MethodGet, "/api/v1/challenges/results"},
		{"run all", http.MethodPost, "/api/v1/challenges/run/all"},
		{"run by category", http.MethodPost, "/api/v1/challenges/run/category/test"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, mock, router := setupChallengeTest()
			mock.listChallengesFunc = func() []services.ChallengeSummary {
				return []services.ChallengeSummary{}
			}
			mock.getResultsFunc = func() []*challenge.Result {
				return []*challenge.Result{}
			}
			mock.runAllFunc = func(ctx context.Context) ([]*challenge.Result, error) {
				return []*challenge.Result{}, nil
			}
			mock.runByCategoryFunc = func(ctx context.Context, category string) ([]*challenge.Result, error) {
				return []*challenge.Result{}, nil
			}

			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
		})
	}
}
