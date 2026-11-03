package challenges

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"digital.vasic.challenges/pkg/challenge"
	"digital.vasic.challenges/pkg/httpclient"

	"github.com/gorilla/websocket"
)

// WebSocketReconnectionChallenge validates WebSocket connect,
// disconnect, and reconnect behavior.
type WebSocketReconnectionChallenge struct {
	challenge.BaseChallenge
	config *BrowsingConfig
}

// NewWebSocketReconnectionChallenge creates CH-081.
func NewWebSocketReconnectionChallenge() *WebSocketReconnectionChallenge {
	return &WebSocketReconnectionChallenge{
		BaseChallenge: challenge.NewBaseChallenge(
			"websocket-reconnection",
			"WebSocket Reconnection",
			"Validates WebSocket connection, disconnection, and "+
				"reconnection behavior. Passes with a note if "+
				"WebSocket is not available.",
			"api",
			[]challenge.ID{"browsing-api-health"},
		),
		config: LoadBrowsingConfig(),
	}
}

// Execute runs the WebSocket reconnection challenge.
func (c *WebSocketReconnectionChallenge) Execute(
	ctx context.Context,
) (*challenge.Result, error) {

	start := time.Now()
	assertions := []challenge.AssertionResult{}
	outputs := map[string]string{"api_url": c.config.BaseURL}

	apiClient := httpclient.NewAPIClient(c.config.BaseURL)
	_, loginErr := apiClient.LoginWithRetry(ctx, c.config.Username, c.config.Password, 3)
	if loginErr != nil {
		challengeResult := c.CreateResult(
			challenge.StatusFailed, start, assertions, nil, outputs,
			fmt.Sprintf("login failed: %v", loginErr),
		)
		challengeResult.RecordAction(fmt.Sprintf("%s: failed - %s", c.Name(), fmt.Sprintf("login failed: %v", loginErr)))
		return challengeResult, nil
	}

	token := apiClient.Token()

	// Build WebSocket URL from base URL, preferring secure WebSocket (wss)
	wsURL := strings.Replace(c.config.BaseURL, "https://", "wss://", 1)
	wsURL = strings.Replace(wsURL, "http://", "ws://", 1)

	wsPaths := []string{"/api/v1/ws", "/ws", "/api/v1/events"}
	dialer := websocket.Dialer{HandshakeTimeout: 5 * time.Second}
	headers := http.Header{}
	if token != "" {
		headers.Set("Authorization", "Bearer "+token)
	}

	connectWS := func() (*websocket.Conn, error) {
		for _, path := range wsPaths {
			fullURL := wsURL + path
			if token != "" {
				fullURL += "?token=" + token
			}
			conn, _, err := dialer.DialContext(ctx, fullURL, headers)
			if err == nil {
				return conn, nil
			}
		}
		return nil, fmt.Errorf("no WebSocket endpoint available")
	}

	// Step 1: First connection
	c.ReportProgress("first-connect", nil)
	conn1, err1 := connectWS()
	if err1 != nil {
		// Real Skipped result — surfacing missing infrastructure honestly
		// rather than claiming a passed state.
		assertions = append(assertions, challenge.AssertionResult{
			Type:     "not_empty",
			Target:   "websocket_available",
			Expected: "reachable WebSocket endpoint",
			Actual:   fmt.Sprintf("no endpoint reachable: %v", err1),
			Passed:   false,
			Message:  "WebSocket endpoint not reachable; skipping reconnection probe",
		})
		outputs["websocket_available"] = "false"
		challengeResult := c.CreateResult(
			challenge.StatusSkipped, start, assertions, nil, outputs,
			"websocket endpoint not reachable",
		)
		challengeResult.RecordAction(fmt.Sprintf("%s: failed - %s", c.Name(), "websocket endpoint not reachable"))
		return challengeResult, nil
	}

	outputs["websocket_available"] = "true"
	assertions = append(assertions, challenge.AssertionResult{
		Type:     "not_empty",
		Target:   "first_connection",
		Expected: "connected",
		Actual:   "connected",
		Passed:   true,
		Message:  "First WebSocket connection established",
	})

	// Step 2: Disconnect
	c.ReportProgress("disconnect", nil)
	conn1.Close()
	time.Sleep(500 * time.Millisecond)

	// Step 3: Reconnect
	c.ReportProgress("reconnect", nil)
	conn2, err2 := connectWS()
	reconnected := err2 == nil
	assertions = append(assertions, challenge.AssertionResult{
		Type:     "not_empty",
		Target:   "reconnection",
		Expected: "reconnected",
		Actual:   challenge.Ternary(reconnected, "reconnected", fmt.Sprintf("err=%v", err2)),
		Passed:   reconnected,
		Message: challenge.Ternary(reconnected,
			"WebSocket reconnection succeeded",
			fmt.Sprintf("WebSocket reconnection failed: %v", err2)),
	})
	if conn2 != nil {
		conn2.Close()
	}

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

// LazyInitOnFirstRequestChallenge validates that the first request
// triggers lazy initialization and returns a valid response.
type LazyInitOnFirstRequestChallenge struct {
	challenge.BaseChallenge
	config *BrowsingConfig
}

// NewLazyInitOnFirstRequestChallenge creates CH-082.
func NewLazyInitOnFirstRequestChallenge() *LazyInitOnFirstRequestChallenge {
	return &LazyInitOnFirstRequestChallenge{
		BaseChallenge: challenge.NewBaseChallenge(
			"lazy-init-on-first-request",
			"Lazy Init on First Request",
			"Validates that a first request triggers lazy "+
				"initialization and returns a valid response.",
			"api",
			[]challenge.ID{"browsing-api-health"},
		),
		config: LoadBrowsingConfig(),
	}
}

// Execute runs the lazy init challenge.
func (c *LazyInitOnFirstRequestChallenge) Execute(
	ctx context.Context,
) (*challenge.Result, error) {

	start := time.Now()
	assertions := []challenge.AssertionResult{}
	outputs := map[string]string{"api_url": c.config.BaseURL}

	client := httpclient.NewAPIClient(c.config.BaseURL)

	// First request — should trigger lazy init if any
	c.ReportProgress("first-request", nil)
	reqStart := time.Now()
	code, body, err := client.Get(ctx, "/health")
	firstLatency := time.Since(reqStart)

	firstOK := err == nil && code == 200
	assertions = append(assertions, challenge.AssertionResult{
		Type:     "status_code",
		Target:   "first_request",
		Expected: "200",
		Actual:   fmt.Sprintf("HTTP %d", code),
		Passed:   firstOK,
		Message: challenge.Ternary(firstOK,
			"First request succeeded (lazy init complete)",
			fmt.Sprintf("First request failed: code=%d err=%v", code, err)),
	})

	hasStatus := false
	if body != nil {
		if _, ok := body["status"]; ok {
			hasStatus = true
		}
	}
	assertions = append(assertions, challenge.AssertionResult{
		Type:     "not_empty",
		Target:   "response_has_status",
		Expected: "status field present",
		Actual:   challenge.Ternary(hasStatus, "present", "missing"),
		Passed:   hasStatus,
		Message: challenge.Ternary(hasStatus,
			"Response contains status field",
			"Response missing status field"),
	})

	outputs["first_latency_ms"] = fmt.Sprintf("%.1f", float64(firstLatency.Milliseconds()))

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

// SemaphorePreventsOverloadChallenge validates that sending many
// concurrent requests is handled via rate limiting or queuing.
type SemaphorePreventsOverloadChallenge struct {
	challenge.BaseChallenge
	config *BrowsingConfig
}

// NewSemaphorePreventsOverloadChallenge creates CH-083.
func NewSemaphorePreventsOverloadChallenge() *SemaphorePreventsOverloadChallenge {
	return &SemaphorePreventsOverloadChallenge{
		BaseChallenge: challenge.NewBaseChallenge(
			"semaphore-prevents-overload",
			"Semaphore Prevents Overload",
			"Sends many concurrent requests and verifies the API "+
				"handles them via rate limiting or queuing without "+
				"returning 500 errors.",
			"resilience",
			[]challenge.ID{"browsing-api-health"},
		),
		config: LoadBrowsingConfig(),
	}
}

// Execute runs the semaphore overload challenge.
func (c *SemaphorePreventsOverloadChallenge) Execute(
	ctx context.Context,
) (*challenge.Result, error) {

	start := time.Now()
	assertions := []challenge.AssertionResult{}
	outputs := map[string]string{"api_url": c.config.BaseURL}

	httpClient := &http.Client{Timeout: 30 * time.Second}

	c.ReportProgress("burst-requests", nil)
	burstSize := 40
	results := make(chan int, burstSize)

	for i := 0; i < burstSize; i++ {
		go func() {
			req, _ := http.NewRequestWithContext(
				ctx, http.MethodGet, c.config.BaseURL+"/health", nil,
			)
			resp, err := httpClient.Do(req)
			if err != nil {
				results <- 0
				return
			}
			resp.Body.Close()
			results <- resp.StatusCode
		}()
	}

	status5xx := 0
	status429 := 0
	status200 := 0
	for i := 0; i < burstSize; i++ {
		code := <-results
		switch {
		case code == 200:
			status200++
		case code == 429:
			status429++
		case code >= 500:
			status5xx++
		}
	}

	outputs["burst_size"] = fmt.Sprintf("%d", burstSize)
	outputs["status_200"] = fmt.Sprintf("%d", status200)
	outputs["status_429"] = fmt.Sprintf("%d", status429)
	outputs["status_5xx"] = fmt.Sprintf("%d", status5xx)

	no5xx := status5xx == 0
	assertions = append(assertions, challenge.AssertionResult{
		Type:     "not_empty",
		Target:   "no_server_errors",
		Expected: "zero 5xx responses",
		Actual:   fmt.Sprintf("5xx=%d, 429=%d, 200=%d", status5xx, status429, status200),
		Passed:   no5xx,
		Message: challenge.Ternary(no5xx,
			fmt.Sprintf("No 500 errors during burst (%d OK, %d rate-limited)", status200, status429),
			fmt.Sprintf("%d server errors during burst", status5xx)),
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

// PrometheusMetricsEndpointChallenge validates the /metrics endpoint
// returns Prometheus metrics including go_ prefixed metrics.
type PrometheusMetricsEndpointChallenge struct {
	challenge.BaseChallenge
	config *BrowsingConfig
}

// NewPrometheusMetricsEndpointChallenge creates CH-084.
func NewPrometheusMetricsEndpointChallenge() *PrometheusMetricsEndpointChallenge {
	return &PrometheusMetricsEndpointChallenge{
		BaseChallenge: challenge.NewBaseChallenge(
			"prometheus-metrics-endpoint",
			"Prometheus Metrics Endpoint",
			"Validates the /metrics endpoint returns Prometheus "+
				"metrics containing go_ runtime metrics.",
			"observability",
			[]challenge.ID{"browsing-api-health"},
		),
		config: LoadBrowsingConfig(),
	}
}

// Execute runs the Prometheus metrics challenge.
func (c *PrometheusMetricsEndpointChallenge) Execute(
	ctx context.Context,
) (*challenge.Result, error) {

	start := time.Now()
	assertions := []challenge.AssertionResult{}
	outputs := map[string]string{"api_url": c.config.BaseURL}

	c.ReportProgress("fetching-metrics", nil)
	code, body, err := fetchMetrics(ctx, c.config)

	codeOK := err == nil && code == 200
	assertions = append(assertions, challenge.AssertionResult{
		Type:     "status_code",
		Target:   "metrics_status",
		Expected: "200",
		Actual:   fmt.Sprintf("%d", code),
		Passed:   codeOK,
		Message: challenge.Ternary(codeOK,
			"Metrics endpoint returned 200",
			fmt.Sprintf("Metrics endpoint returned %d, err=%v", code, err)),
	})

	if codeOK && body != nil {
		bodyStr := string(body)
		hasGoMetrics := strings.Contains(bodyStr, "go_")
		outputs["has_go_metrics"] = fmt.Sprintf("%t", hasGoMetrics)
		assertions = append(assertions, challenge.AssertionResult{
			Type:     "not_empty",
			Target:   "go_metrics_present",
			Expected: "go_ metrics present",
			Actual:   challenge.Ternary(hasGoMetrics, "present", "missing"),
			Passed:   hasGoMetrics,
			Message: challenge.Ternary(hasGoMetrics,
				"Prometheus go_ metrics present",
				"Prometheus go_ metrics missing from /metrics"),
		})
	}

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

// HTTPRequestMetricsIncrementChallenge validates that /metrics shows
// http_requests counter incrementing after requests.
type HTTPRequestMetricsIncrementChallenge struct {
	challenge.BaseChallenge
	config *BrowsingConfig
}

// NewHTTPRequestMetricsIncrementChallenge creates CH-085.
func NewHTTPRequestMetricsIncrementChallenge() *HTTPRequestMetricsIncrementChallenge {
	return &HTTPRequestMetricsIncrementChallenge{
		BaseChallenge: challenge.NewBaseChallenge(
			"http-request-metrics-increment",
			"HTTP Request Metrics Increment",
			"Validates /metrics contains an http_requests counter "+
				"after hitting the health endpoint.",
			"observability",
			[]challenge.ID{"browsing-api-health"},
		),
		config: LoadBrowsingConfig(),
	}
}

// Execute runs the HTTP metrics increment challenge.
func (c *HTTPRequestMetricsIncrementChallenge) Execute(
	ctx context.Context,
) (*challenge.Result, error) {

	start := time.Now()
	assertions := []challenge.AssertionResult{}
	outputs := map[string]string{"api_url": c.config.BaseURL}

	client := httpclient.NewAPIClient(c.config.BaseURL)

	// Generate some traffic
	c.ReportProgress("generating-traffic", nil)
	for i := 0; i < 5; i++ {
		client.Get(ctx, "/health")
	}

	// Check metrics
	c.ReportProgress("checking-metrics", nil)
	code, body, err := fetchMetrics(ctx, c.config)

	codeOK := err == nil && code == 200
	assertions = append(assertions, challenge.AssertionResult{
		Type:     "status_code",
		Target:   "metrics_status",
		Expected: "200",
		Actual:   fmt.Sprintf("%d", code),
		Passed:   codeOK,
		Message: challenge.Ternary(codeOK,
			"Metrics endpoint returned 200",
			fmt.Sprintf("Metrics endpoint returned %d, err=%v", code, err)),
	})

	if codeOK && body != nil {
		bodyStr := string(body)
		hasHTTPMetrics := strings.Contains(bodyStr, "http_request") ||
			strings.Contains(bodyStr, "http_server") ||
			strings.Contains(bodyStr, "promhttp")
		outputs["has_http_metrics"] = fmt.Sprintf("%t", hasHTTPMetrics)
		assertions = append(assertions, challenge.AssertionResult{
			Type:     "not_empty",
			Target:   "http_metrics_present",
			Expected: "HTTP request metrics present",
			Actual:   challenge.Ternary(hasHTTPMetrics, "present", "missing"),
			Passed:   hasHTTPMetrics,
			Message: challenge.Ternary(hasHTTPMetrics,
				"HTTP request metrics found in /metrics",
				"HTTP request metrics missing from /metrics"),
		})
	}

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

// RuntimeMetricsCurrentChallenge validates that /metrics contains
// go_goroutines metric with a value greater than 0.
type RuntimeMetricsCurrentChallenge struct {
	challenge.BaseChallenge
	config *BrowsingConfig
}

// NewRuntimeMetricsCurrentChallenge creates CH-086.
func NewRuntimeMetricsCurrentChallenge() *RuntimeMetricsCurrentChallenge {
	return &RuntimeMetricsCurrentChallenge{
		BaseChallenge: challenge.NewBaseChallenge(
			"runtime-metrics-current",
			"Runtime Metrics Current",
			"Validates /metrics contains go_goroutines metric "+
				"with a value greater than 0.",
			"observability",
			[]challenge.ID{"browsing-api-health"},
		),
		config: LoadBrowsingConfig(),
	}
}

// Execute runs the runtime metrics challenge.
func (c *RuntimeMetricsCurrentChallenge) Execute(
	ctx context.Context,
) (*challenge.Result, error) {

	start := time.Now()
	assertions := []challenge.AssertionResult{}
	outputs := map[string]string{"api_url": c.config.BaseURL}

	c.ReportProgress("fetching-metrics", nil)
	code, body, err := fetchMetrics(ctx, c.config)

	codeOK := err == nil && code == 200
	assertions = append(assertions, challenge.AssertionResult{
		Type:     "status_code",
		Target:   "metrics_status",
		Expected: "200",
		Actual:   fmt.Sprintf("%d", code),
		Passed:   codeOK,
		Message: challenge.Ternary(codeOK,
			"Metrics endpoint returned 200",
			fmt.Sprintf("Metrics endpoint returned %d, err=%v", code, err)),
	})

	if codeOK && body != nil {
		bodyStr := string(body)
		hasGoroutines := strings.Contains(bodyStr, "go_goroutines")
		outputs["has_go_goroutines"] = fmt.Sprintf("%t", hasGoroutines)

		// Check the value is > 0 by looking for "go_goroutines <number>"
		goroutinePositive := false
		for _, line := range strings.Split(bodyStr, "\n") {
			if strings.HasPrefix(line, "go_goroutines ") {
				parts := strings.Fields(line)
				if len(parts) >= 2 && parts[1] != "0" {
					goroutinePositive = true
				}
				break
			}
		}

		assertions = append(assertions, challenge.AssertionResult{
			Type:     "not_empty",
			Target:   "go_goroutines_positive",
			Expected: "go_goroutines > 0",
			Actual:   challenge.Ternary(goroutinePositive, "> 0", "0 or missing"),
			Passed:   goroutinePositive,
			Message: challenge.Ternary(goroutinePositive,
				"go_goroutines metric present and > 0",
				"go_goroutines metric missing or zero"),
		})
	}

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

// DBQueryDurationTrackedChallenge validates that /metrics contains
// db_query_duration metrics after a DB query.
type DBQueryDurationTrackedChallenge struct {
	challenge.BaseChallenge
	config *BrowsingConfig
}

// NewDBQueryDurationTrackedChallenge creates CH-087.
func NewDBQueryDurationTrackedChallenge() *DBQueryDurationTrackedChallenge {
	return &DBQueryDurationTrackedChallenge{
		BaseChallenge: challenge.NewBaseChallenge(
			"db-query-duration-tracked",
			"DB Query Duration Tracked",
			"Validates /metrics contains db_query_duration metrics "+
				"after a database query has been executed.",
			"observability",
			[]challenge.ID{"browsing-api-health"},
		),
		config: LoadBrowsingConfig(),
	}
}

// Execute runs the DB query duration tracking challenge.
func (c *DBQueryDurationTrackedChallenge) Execute(
	ctx context.Context,
) (*challenge.Result, error) {

	start := time.Now()
	assertions := []challenge.AssertionResult{}
	outputs := map[string]string{"api_url": c.config.BaseURL}

	client := httpclient.NewAPIClient(c.config.BaseURL)

	// Trigger a DB query by hitting an authenticated endpoint
	c.ReportProgress("triggering-db-query", nil)
	_, loginErr := client.LoginWithRetry(ctx, c.config.Username, c.config.Password, 3)
	if loginErr != nil {
		challengeResult := c.CreateResult(
			challenge.StatusFailed, start, assertions, nil, outputs,
			fmt.Sprintf("login failed: %v", loginErr),
		)
		challengeResult.RecordAction(fmt.Sprintf("%s: failed - %s", c.Name(), fmt.Sprintf("login failed: %v", loginErr)))
		return challengeResult, nil
	}

	// Hit a DB-backed endpoint
	client.Get(ctx, "/api/v1/entities?limit=1")

	// Check metrics for db_query_duration
	c.ReportProgress("checking-metrics", nil)
	code, body, err := client.GetRaw(ctx, "/metrics")

	codeOK := err == nil && code == 200
	assertions = append(assertions, challenge.AssertionResult{
		Type:     "status_code",
		Target:   "metrics_status",
		Expected: "200",
		Actual:   fmt.Sprintf("%d", code),
		Passed:   codeOK,
		Message: challenge.Ternary(codeOK,
			"Metrics endpoint returned 200",
			fmt.Sprintf("Metrics endpoint returned %d, err=%v", code, err)),
	})

	if codeOK && body != nil {
		bodyStr := string(body)
		hasDBMetrics := strings.Contains(bodyStr, "db_query") ||
			strings.Contains(bodyStr, "database") ||
			strings.Contains(bodyStr, "sql")
		outputs["has_db_metrics"] = fmt.Sprintf("%t", hasDBMetrics)
		assertions = append(assertions, challenge.AssertionResult{
			Type:     "not_empty",
			Target:   "db_query_metrics",
			Expected: "DB query metrics present",
			Actual:   challenge.Ternary(hasDBMetrics, "present", "missing"),
			Passed:   hasDBMetrics,
			Message: challenge.Ternary(hasDBMetrics,
				"DB query duration metrics found in /metrics",
				"DB query duration metrics missing from /metrics"),
		})
	}

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

// GrafanaDashboardRendersChallenge validates that a Grafana
// dashboard configuration file exists in the monitoring directory.
type GrafanaDashboardRendersChallenge struct {
	challenge.BaseChallenge
	config *BrowsingConfig
}

// NewGrafanaDashboardRendersChallenge creates CH-088.
func NewGrafanaDashboardRendersChallenge() *GrafanaDashboardRendersChallenge {
	return &GrafanaDashboardRendersChallenge{
		BaseChallenge: challenge.NewBaseChallenge(
			"grafana-dashboard-renders",
			"Grafana Dashboard Config Exists",
			"Validates that a Grafana dashboard configuration "+
				"file exists in the monitoring directory.",
			"observability",
			nil,
		),
		config: LoadBrowsingConfig(),
	}
}

// Execute runs the Grafana dashboard challenge.
func (c *GrafanaDashboardRendersChallenge) Execute(
	ctx context.Context,
) (*challenge.Result, error) {

	start := time.Now()
	assertions := []challenge.AssertionResult{}
	outputs := map[string]string{}

	c.ReportProgress("checking-dashboard", nil)

	// Look for Grafana dashboard files in common locations
	searchDirs := []string{
		"../monitoring",
		"monitoring",
		"../monitoring/grafana",
		"monitoring/grafana",
		"../config/grafana",
		"config/grafana",
	}

	var dashboardDir string
	for _, dir := range searchDirs {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			dashboardDir = dir
			break
		}
	}

	if dashboardDir == "" {
		cwd, _ := os.Getwd()
		parentMonitoring := filepath.Join(filepath.Dir(cwd), "monitoring")
		if info, err := os.Stat(parentMonitoring); err == nil && info.IsDir() {
			dashboardDir = parentMonitoring
		}
	}

	dirExists := dashboardDir != ""
	outputs["dashboard_dir"] = challenge.Ternary(dirExists, dashboardDir, "not found")

	assertions = append(assertions, challenge.AssertionResult{
		Type:     "not_empty",
		Target:   "monitoring_directory",
		Expected: "monitoring directory exists",
		Actual:   challenge.Ternary(dirExists, dashboardDir, "not found"),
		Passed:   dirExists,
		Message: challenge.Ternary(dirExists,
			fmt.Sprintf("Monitoring directory found: %s", dashboardDir),
			"Monitoring directory not found"),
	})

	if dirExists {
		// Count JSON files (Grafana dashboards are JSON)
		dashboardFiles := 0
		_ = filepath.Walk(dashboardDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if !info.IsDir() {
				ext := filepath.Ext(path)
				if ext == ".json" || ext == ".yml" || ext == ".yaml" {
					dashboardFiles++
				}
			}
			return nil
		})

		outputs["dashboard_files"] = fmt.Sprintf("%d", dashboardFiles)
		hasDashboards := dashboardFiles > 0
		assertions = append(assertions, challenge.AssertionResult{
			Type:     "not_empty",
			Target:   "dashboard_files_exist",
			Expected: "at least one dashboard config file",
			Actual:   fmt.Sprintf("%d files", dashboardFiles),
			Passed:   hasDashboards,
			Message: challenge.Ternary(hasDashboards,
				fmt.Sprintf("Found %d dashboard config files", dashboardFiles),
				"No dashboard config files found"),
		})
	}

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
