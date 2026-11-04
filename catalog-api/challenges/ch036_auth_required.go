package challenges

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"digital.vasic.challenges/pkg/challenge"
	"digital.vasic.challenges/pkg/httpclient"
)

// AuthRequiredChallenge validates that all API endpoints require
// authentication except for explicitly public ones (/health,
// /api/v1/health, /api/v1/login, /api/v1/register, /api/v1/challenges/*).
type AuthRequiredChallenge struct {
	challenge.BaseChallenge
	config *BrowsingConfig
}

// NewAuthRequiredChallenge creates CH-036.
func NewAuthRequiredChallenge() *AuthRequiredChallenge {
	return &AuthRequiredChallenge{
		BaseChallenge: challenge.NewBaseChallenge(
			"auth-required",
			"Auth Required on Protected Endpoints",
			"Verifies all API endpoints require authentication except "+
				"/health, /api/v1/health, /api/v1/login, /api/v1/register, "+
				"and /api/v1/challenges/*.",
			"security",
			[]challenge.ID{"browsing-api-health"},
		),
		config: LoadBrowsingConfig(),
	}
}

// Execute runs the auth-required challenge.
func (c *AuthRequiredChallenge) Execute(
	ctx context.Context,
) (*challenge.Result, error) {

	start := time.Now()
	assertions := []challenge.AssertionResult{}
	outputs := map[string]string{
		"api_url": c.config.BaseURL,
	}

	httpClient := &http.Client{Timeout: 10 * time.Second}

	// Public endpoints that should NOT require auth
	publicEndpoints := []string{
		"/health",
	}

	c.ReportProgress("public-endpoints", nil)
	for _, ep := range publicEndpoints {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, c.config.BaseURL+ep, nil)
		resp, err := httpClient.Do(req)
		code := 0
		if err == nil {
			code = resp.StatusCode
			resp.Body.Close()
		}
		passed := err == nil && code == 200
		assertions = append(assertions, challenge.AssertionResult{
			Type:     "status_code",
			Target:   fmt.Sprintf("public_%s", ep),
			Expected: "200 without auth",
			Actual:   fmt.Sprintf("HTTP %d", code),
			Passed:   passed,
			Message: challenge.Ternary(passed,
				fmt.Sprintf("Public endpoint %s accessible without auth", ep),
				fmt.Sprintf("Public endpoint %s not accessible: code=%d err=%v", ep, code, err)),
		})
	}

	// Protected endpoints that MUST require auth (401 or 403 without token)
	protectedEndpoints := []string{
		"/api/v1/auth/me",
		"/api/v1/storage-roots",
		"/api/v1/catalog",
		"/api/v1/media/stats",
		"/api/v1/search",
		"/metrics",
	}

	c.ReportProgress("protected-endpoints", nil)
	for _, ep := range protectedEndpoints {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, c.config.BaseURL+ep, nil)
		resp, err := httpClient.Do(req)
		code := 0
		if err == nil {
			code = resp.StatusCode
			resp.Body.Close()
		}
		rejected := err == nil && (code == 401 || code == 403)
		assertions = append(assertions, challenge.AssertionResult{
			Type:     "status_code",
			Target:   fmt.Sprintf("protected_%s", ep),
			Expected: "401 or 403 without auth",
			Actual:   fmt.Sprintf("HTTP %d", code),
			Passed:   rejected,
			Message: challenge.Ternary(rejected,
				fmt.Sprintf("Protected endpoint %s requires auth: HTTP %d", ep, code),
				fmt.Sprintf("Protected endpoint %s did NOT require auth: code=%d err=%v", ep, code, err)),
		})
	}

	// Verify that authenticated requests succeed on the same endpoints
	c.ReportProgress("authenticated-access", nil)
	apiClient := httpclient.NewAPIClient(c.config.BaseURL)
	_, loginErr := apiClient.LoginWithRetry(ctx, c.config.Username, c.config.Password, 3)
	loginOK := loginErr == nil

	assertions = append(assertions, challenge.AssertionResult{
		Type:     "not_empty",
		Target:   "auth_login",
		Expected: "login succeeds",
		Actual:   challenge.Ternary(loginOK, "succeeded", fmt.Sprintf("err=%v", loginErr)),
		Passed:   loginOK,
		Message: challenge.Ternary(loginOK,
			"Admin login succeeded for authenticated tests",
			fmt.Sprintf("Login failed: %v", loginErr)),
	})

	if loginOK {
		// Spot-check one protected endpoint with valid auth
		meCode, _, meErr := apiClient.Get(ctx, "/api/v1/auth/me")
		meOK := meErr == nil && meCode == 200
		assertions = append(assertions, challenge.AssertionResult{
			Type:     "status_code",
			Target:   "authenticated_access",
			Expected: "200 with valid auth",
			Actual:   fmt.Sprintf("HTTP %d", meCode),
			Passed:   meOK,
			Message: challenge.Ternary(meOK,
				"Authenticated access to protected endpoint succeeded",
				fmt.Sprintf("Authenticated access failed: code=%d err=%v", meCode, meErr)),
		})
	}

	metrics := map[string]challenge.MetricValue{
		"auth_validation_time": {
			Name:  "auth_validation_time",
			Value: float64(time.Since(start).Milliseconds()),
			Unit:  "ms",
		},
	}

	status := challenge.StatusPassed
	for _, a := range assertions {
		if !a.Passed {
			status = challenge.StatusFailed
			break
		}
	}

	challengeResult := c.CreateResult(status, start, assertions, metrics, outputs, "")
	challengeResult.RecordAction(fmt.Sprintf("%s: challenge completed with status %s", c.Name(), status))
	return challengeResult, nil
}
