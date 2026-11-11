package services

import (
	"testing"
	"time"

	"catalogizer/models"

	"github.com/stretchr/testify/require"
)

// Tests for the pure helper methods on AnalyticsService that don't touch
// the repository — they operate entirely on in-memory models. This drives
// up coverage without requiring DB mocks.

func newAnalyticsPureTestService() *AnalyticsService {
	// All the functions we test here are pure receivers — the repo is
	// never called. A nil service is safe.
	return &AnalyticsService{}
}

func TestCalculateSystemHealthScore_Empty(t *testing.T) {
	s := newAnalyticsPureTestService()
	score := s.calculateSystemHealthScore(&models.SystemAnalytics{})
	require.Equal(t, 0.0, score)
}

func TestCalculateSystemHealthScore_FullScore(t *testing.T) {
	s := newAnalyticsPureTestService()
	analytics := &models.SystemAnalytics{
		TotalUsers:             10,
		ActiveUsers:            10, // 100% active → +40
		TotalMediaAccesses:     500, // +30
		TotalEvents:            200, // +20
		AverageSessionDuration: 10 * time.Minute, // +10
	}
	score := s.calculateSystemHealthScore(analytics)
	require.InDelta(t, 100.0, score, 0.01)
}

func TestCalculateSystemHealthScore_HalfActive(t *testing.T) {
	s := newAnalyticsPureTestService()
	analytics := &models.SystemAnalytics{
		TotalUsers:  10,
		ActiveUsers: 5, // 50% → +20
	}
	score := s.calculateSystemHealthScore(analytics)
	require.InDelta(t, 20.0, score, 0.01)
}

func TestCalculateSystemHealthScore_NoUsers(t *testing.T) {
	s := newAnalyticsPureTestService()
	// TotalUsers=0 means activeUserRatio path skipped.
	analytics := &models.SystemAnalytics{
		TotalMediaAccesses: 1,
		TotalEvents:        1,
	}
	score := s.calculateSystemHealthScore(analytics)
	require.InDelta(t, 50.0, score, 0.01) // 30 + 20
}

func TestCalculateSystemHealthScore_ShortSessionsNoBonus(t *testing.T) {
	s := newAnalyticsPureTestService()
	analytics := &models.SystemAnalytics{
		AverageSessionDuration: 2 * time.Minute, // < 5min, no bonus
	}
	score := s.calculateSystemHealthScore(analytics)
	require.Equal(t, 0.0, score)
}

func TestCalculateGrowthRate_Empty(t *testing.T) {
	s := newAnalyticsPureTestService()
	require.Equal(t, 0.0, s.calculateGrowthRate(nil))
	require.Equal(t, 0.0, s.calculateGrowthRate([]models.UserGrowthPoint{}))
	require.Equal(t, 0.0, s.calculateGrowthRate([]models.UserGrowthPoint{
		{UserCount: 100},
	}))
}

func TestCalculateGrowthRate_ZeroStart(t *testing.T) {
	s := newAnalyticsPureTestService()
	growth := []models.UserGrowthPoint{
		{UserCount: 0},
		{UserCount: 10},
	}
	require.Equal(t, 0.0, s.calculateGrowthRate(growth),
		"division by zero must return 0, not Inf")
}

func TestCalculateGrowthRate_Positive(t *testing.T) {
	s := newAnalyticsPureTestService()
	growth := []models.UserGrowthPoint{
		{Date: time.Now().AddDate(0, -1, 0), UserCount: 100},
		{Date: time.Now(), UserCount: 150},
	}
	// (150 - 100) / 100 * 100 = 50.0%
	require.InDelta(t, 50.0, s.calculateGrowthRate(growth), 0.01)
}

func TestCalculateGrowthRate_Negative(t *testing.T) {
	s := newAnalyticsPureTestService()
	growth := []models.UserGrowthPoint{
		{UserCount: 200},
		{UserCount: 150},
	}
	require.InDelta(t, -25.0, s.calculateGrowthRate(growth), 0.01)
}

func TestCalculateEngagementLevel_NoUsers(t *testing.T) {
	s := newAnalyticsPureTestService()
	require.Equal(t, "low", s.calculateEngagementLevel(&models.SystemAnalytics{}))
}

func TestCalculateEngagementLevel_High(t *testing.T) {
	s := newAnalyticsPureTestService()
	analytics := &models.SystemAnalytics{
		TotalUsers:         10,
		TotalMediaAccesses: 500, // 50 per user
	}
	require.Equal(t, "high", s.calculateEngagementLevel(analytics))
}

func TestCalculateEngagementLevel_Medium(t *testing.T) {
	s := newAnalyticsPureTestService()
	analytics := &models.SystemAnalytics{
		TotalUsers:         10,
		TotalMediaAccesses: 250, // 25 per user
	}
	require.Equal(t, "medium", s.calculateEngagementLevel(analytics))
}

func TestCalculateEngagementLevel_Low(t *testing.T) {
	s := newAnalyticsPureTestService()
	analytics := &models.SystemAnalytics{
		TotalUsers:         10,
		TotalMediaAccesses: 50, // 5 per user
	}
	require.Equal(t, "low", s.calculateEngagementLevel(analytics))
}

func TestCalculateEngagementLevel_ExactBoundaries(t *testing.T) {
	s := newAnalyticsPureTestService()
	// Exactly 50 accesses/user → high boundary
	require.Equal(t, "high", s.calculateEngagementLevel(&models.SystemAnalytics{
		TotalUsers: 1, TotalMediaAccesses: 50,
	}))
	// Exactly 20 accesses/user → medium boundary
	require.Equal(t, "medium", s.calculateEngagementLevel(&models.SystemAnalytics{
		TotalUsers: 1, TotalMediaAccesses: 20,
	}))
	// 19 → low
	require.Equal(t, "low", s.calculateEngagementLevel(&models.SystemAnalytics{
		TotalUsers: 1, TotalMediaAccesses: 19,
	}))
}

func TestGetTopLocations_Empty(t *testing.T) {
	s := newAnalyticsPureTestService()
	result := s.getTopLocations(nil, 5)
	require.Empty(t, result)
}

func TestGetTopLocations_MissingKey(t *testing.T) {
	s := newAnalyticsPureTestService()
	// No "locations" key → empty result.
	data := map[string]interface{}{"other": "value"}
	result := s.getTopLocations(data, 5)
	require.Empty(t, result)
}

func TestGetTopLocations_WithData(t *testing.T) {
	s := newAnalyticsPureTestService()
	data := map[string]interface{}{
		"locations": []map[string]interface{}{
			{"country": "US", "count": 100},
			{"country": "DE", "count": 50},
			{"country": "FR", "count": 30},
			{"country": "JP", "count": 20},
			{"country": "BR", "count": 10},
		},
	}
	// Limit 3 → first 3 entries.
	result := s.getTopLocations(data, 3)
	require.Len(t, result, 3)
	require.Equal(t, "US", result[0]["country"])
	require.Equal(t, "DE", result[1]["country"])
	require.Equal(t, "FR", result[2]["country"])
}

func TestGetTopLocations_LimitLargerThanData(t *testing.T) {
	s := newAnalyticsPureTestService()
	data := map[string]interface{}{
		"locations": []map[string]interface{}{
			{"country": "US", "count": 100},
			{"country": "DE", "count": 50},
		},
	}
	result := s.getTopLocations(data, 10)
	require.Len(t, result, 2, "should return all available entries when limit > len")
}
