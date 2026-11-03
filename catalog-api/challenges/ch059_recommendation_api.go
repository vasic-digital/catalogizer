package challenges

import (
	"context"
	"fmt"
	"time"

	"digital.vasic.challenges/pkg/challenge"
	"digital.vasic.challenges/pkg/httpclient"
)

// RecommendationAPIChallenge validates the recommendation engine API:
// getting recommendations by media type, similar items, and
// personalized suggestions.
type RecommendationAPIChallenge struct {
	challenge.BaseChallenge
	config *BrowsingConfig
}

// NewRecommendationAPIChallenge creates CH-059.
func NewRecommendationAPIChallenge() *RecommendationAPIChallenge {
	return &RecommendationAPIChallenge{
		BaseChallenge: challenge.NewBaseChallenge(
			"recommendation-api",
			"Recommendation Engine API",
			"Validates the recommendation endpoints: get recommendations "+
				"for media type, find similar items, and personalized "+
				"suggestions based on user activity.",
			"api",
			[]challenge.ID{"entity-browsing"},
		),
		config: LoadBrowsingConfig(),
	}
}

// Execute runs the recommendation API challenge.
func (c *RecommendationAPIChallenge) Execute(
	ctx context.Context,
) (*challenge.Result, error) {

	start := time.Now()
	assertions := []challenge.AssertionResult{}
	outputs := map[string]string{}

	client := httpclient.NewAPIClient(c.config.BaseURL)

	// Login
	c.ReportProgress("authenticating", nil)
	_, err := client.LoginWithRetry(ctx, c.config.Username, c.config.Password, 5)
	if err != nil {
		challengeResult := c.CreateResult(
			challenge.StatusFailed, start, assertions, nil, outputs,
			fmt.Sprintf("login failed: %v", err),
		)
		challengeResult.RecordAction(fmt.Sprintf("%s: failed - %s", c.Name(), fmt.Sprintf("login failed: %v", err)))
		return challengeResult, nil
	}

	// CAT-001: no recommendation engine / trending source exists, so the recommendation
	// routes answer an HONEST 501. The challenge pins exactly 501: a regression to an
	// invented 200 (fabricated titles) must FAIL it, and so must a 404/500.
	pin501 := func(progress, target, path, okMsg, badMsg string) {
		c.ReportProgress(progress, nil)
		st, _, _ := client.Get(ctx, path)
		ok := st == 501
		assertions = append(assertions, challenge.AssertionResult{
			Type:     "status_code",
			Target:   target,
			Expected: "501",
			Actual:   fmt.Sprintf("%d", st),
			Passed:   ok,
			Message:  challenge.Ternary(ok, okMsg, badMsg),
		})
	}

	// Test 1: Recommendations list (root alias)
	pin501("testing-recommendations", "recommendations_list", "/api/v1/recommendations",
		"Recommendations root answers an honest 501", "Recommendations root did not answer 501")

	// Test 2: Recommendations by type (root alias with filters)
	pin501("testing-recs-by-type", "recommendations_by_type", "/api/v1/recommendations?type=movie&limit=5",
		"Recommendations by type answers an honest 501", "Recommendations by type did not answer 501")

	// Test 2b: Trending
	pin501("testing-recs-trending", "recommendations_trending", "/api/v1/recommendations/trending",
		"Trending recommendations answer an honest 501", "Trending recommendations did not answer 501")

	// Test 3: Similar items for non-existent entity
	c.ReportProgress("testing-similar-not-found", nil)
	statusSimilar, _, _ := client.Get(ctx, "/api/v1/entities/99999999/similar")

	similarOK := statusSimilar == 404 || statusSimilar == 200
	assertions = append(assertions, challenge.AssertionResult{
		Type:     "status_code",
		Target:   "similar_items",
		Expected: "200 or 404",
		Actual:   fmt.Sprintf("%d", statusSimilar),
		Passed:   similarOK,
		Message:  challenge.Ternary(similarOK, "Similar items endpoint responds correctly", "Similar items endpoint error"),
	})

	resultStatus := challenge.StatusPassed
	for _, a := range assertions {
		if !a.Passed {
			resultStatus = challenge.StatusFailed
			break
		}
	}

	challengeResult := c.CreateResult(resultStatus, start, assertions, nil, outputs, "")
	challengeResult.RecordAction(fmt.Sprintf("%s: challenge completed with status %s", c.Name(), resultStatus))
	return challengeResult, nil
}
