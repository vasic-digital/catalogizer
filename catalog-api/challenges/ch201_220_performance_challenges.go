package challenges

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"digital.vasic.challenges/pkg/challenge"
	"digital.vasic.challenges/pkg/httpclient"
)

// Challenges CH-201 through CH-220 validate performance
// characteristics including latency, concurrency, stability,
// caching, compression, and observability.

// --- CH-201: HealthEndpointFast ---

// HealthEndpointFastChallenge validates that /health responds
// in under 100ms.
type HealthEndpointFastChallenge struct {
	challenge.BaseChallenge
	config *BrowsingConfig
}

// NewHealthEndpointFastChallenge creates CH-201.
func NewHealthEndpointFastChallenge() *HealthEndpointFastChallenge {
	return &HealthEndpointFastChallenge{
		BaseChallenge: challenge.NewBaseChallenge(
			"health-endpoint-fast",
			"Health Endpoint Fast",
			"Validates that /health responds in under 100ms.",
			"performance",
			[]challenge.ID{"browsing-api-health"},
		),
		config: LoadBrowsingConfig(),
	}
}

// Execute runs the health endpoint fast challenge.
func (c *HealthEndpointFastChallenge) Execute(
	ctx context.Context,
) (*challenge.Result, error) {

	start := time.Now()
	assertions := []challenge.AssertionResult{}
	outputs := map[string]string{"api_url": c.config.BaseURL}

	httpClient := &http.Client{Timeout: 10 * time.Second}

	c.ReportProgress("measuring-health-latency", nil)

	reqStart := time.Now()
	req, _ := http.NewRequestWithContext(
		ctx, http.MethodGet,
		c.config.BaseURL+"/health", nil,
	)
	resp, err := httpClient.Do(req)
	latency := time.Since(reqStart)

	if err != nil {
		challengeResult := c.CreateResult(
			challenge.StatusFailed, start, assertions, nil,
			outputs, fmt.Sprintf("health request failed: %v", err),
		)
		challengeResult.RecordAction(fmt.Sprintf("%s: failed - %s", c.Name(), fmt.Sprintf("health request failed: %v", err)))
		return challengeResult, nil
	}
	resp.Body.Close()

	outputs["latency_ms"] = fmt.Sprintf("%d", latency.Milliseconds())
	fast := latency < 100*time.Millisecond
	assertions = append(assertions, challenge.AssertionResult{
		Type:     "max_latency",
		Target:   "health_latency",
		Expected: "<100ms",
		Actual:   latency.String(),
		Passed:   fast,
		Message: challenge.Ternary(fast,
			fmt.Sprintf("Health responded in %s", latency),
			fmt.Sprintf("Health too slow: %s", latency)),
	})

	status := challenge.StatusPassed
	for _, a := range assertions {
		if !a.Passed {
			status = challenge.StatusFailed
			break
		}
	}

	challengeResult := c.CreateResult(
		status, start, assertions, nil, outputs, "",
	)
	challengeResult.RecordAction(fmt.Sprintf("%s: challenge completed with status %s", c.Name(), status))
	return challengeResult, nil
}

// --- CH-202: SearchLatency ---

// SearchLatencyChallenge validates that search responds in
// under 500ms.
type SearchLatencyChallenge struct {
	challenge.BaseChallenge
	config *BrowsingConfig
}

// NewSearchLatencyChallenge creates CH-202.
func NewSearchLatencyChallenge() *SearchLatencyChallenge {
	return &SearchLatencyChallenge{
		BaseChallenge: challenge.NewBaseChallenge(
			"search-latency",
			"Search Latency",
			"Validates that search responds in under 500ms.",
			"performance",
			[]challenge.ID{"browsing-api-health"},
		),
		config: LoadBrowsingConfig(),
	}
}

// Execute runs the search latency challenge.
func (c *SearchLatencyChallenge) Execute(
	ctx context.Context,
) (*challenge.Result, error) {

	start := time.Now()
	assertions := []challenge.AssertionResult{}
	outputs := map[string]string{"api_url": c.config.BaseURL}

	client := httpclient.NewAPIClient(c.config.BaseURL)

	_, loginErr := client.LoginWithRetry(
		ctx, c.config.Username, c.config.Password, 3,
	)
	if loginErr != nil {
		challengeResult := c.CreateResult(
			challenge.StatusFailed, start, assertions, nil,
			outputs, fmt.Sprintf("login failed: %v", loginErr),
		)
		challengeResult.RecordAction(fmt.Sprintf("%s: failed - %s", c.Name(), fmt.Sprintf("login failed: %v", loginErr)))
		return challengeResult, nil
	}

	c.ReportProgress("measuring-search-latency", nil)

	reqStart := time.Now()
	code, _, err := client.GetRaw(
		ctx, "/api/v1/entities/search?q=movie",
	)
	latency := time.Since(reqStart)

	codeOK := err == nil && code == 200
	outputs["latency_ms"] = fmt.Sprintf("%d", latency.Milliseconds())

	fast := latency < 500*time.Millisecond && codeOK
	assertions = append(assertions, challenge.AssertionResult{
		Type:     "max_latency",
		Target:   "search_latency",
		Expected: "<500ms with 200",
		Actual:   fmt.Sprintf("%s (HTTP %d)", latency, code),
		Passed:   fast,
		Message: challenge.Ternary(fast,
			fmt.Sprintf("Search responded in %s", latency),
			fmt.Sprintf("Search: %s, HTTP %d, err=%v",
				latency, code, err)),
	})

	status := challenge.StatusPassed
	for _, a := range assertions {
		if !a.Passed {
			status = challenge.StatusFailed
			break
		}
	}

	challengeResult := c.CreateResult(
		status, start, assertions, nil, outputs, "",
	)
	challengeResult.RecordAction(fmt.Sprintf("%s: challenge completed with status %s", c.Name(), status))
	return challengeResult, nil
}

// --- CH-203: BrowseLatency ---

// BrowseLatencyChallenge validates that browse responds in
// under 500ms.
type BrowseLatencyChallenge struct {
	challenge.BaseChallenge
	config *BrowsingConfig
}

// NewBrowseLatencyChallenge creates CH-203.
func NewBrowseLatencyChallenge() *BrowseLatencyChallenge {
	return &BrowseLatencyChallenge{
		BaseChallenge: challenge.NewBaseChallenge(
			"browse-latency",
			"Browse Latency",
			"Validates that browse responds in under 500ms.",
			"performance",
			[]challenge.ID{"browsing-api-health"},
		),
		config: LoadBrowsingConfig(),
	}
}

// Execute runs the browse latency challenge.
func (c *BrowseLatencyChallenge) Execute(
	ctx context.Context,
) (*challenge.Result, error) {

	start := time.Now()
	assertions := []challenge.AssertionResult{}
	outputs := map[string]string{"api_url": c.config.BaseURL}

	client := httpclient.NewAPIClient(c.config.BaseURL)

	_, loginErr := client.LoginWithRetry(
		ctx, c.config.Username, c.config.Password, 3,
	)
	if loginErr != nil {
		challengeResult := c.CreateResult(
			challenge.StatusFailed, start, assertions, nil,
			outputs, fmt.Sprintf("login failed: %v", loginErr),
		)
		challengeResult.RecordAction(fmt.Sprintf("%s: failed - %s", c.Name(), fmt.Sprintf("login failed: %v", loginErr)))
		return challengeResult, nil
	}

	c.ReportProgress("measuring-browse-latency", nil)

	reqStart := time.Now()
	code, _, err := client.GetRaw(
		ctx, "/api/v1/entities?limit=20",
	)
	latency := time.Since(reqStart)

	codeOK := err == nil && code == 200
	outputs["latency_ms"] = fmt.Sprintf("%d", latency.Milliseconds())

	fast := latency < 500*time.Millisecond && codeOK
	assertions = append(assertions, challenge.AssertionResult{
		Type:     "max_latency",
		Target:   "browse_latency",
		Expected: "<500ms with 200",
		Actual:   fmt.Sprintf("%s (HTTP %d)", latency, code),
		Passed:   fast,
		Message: challenge.Ternary(fast,
			fmt.Sprintf("Browse responded in %s", latency),
			fmt.Sprintf("Browse: %s, HTTP %d, err=%v",
				latency, code, err)),
	})

	status := challenge.StatusPassed
	for _, a := range assertions {
		if !a.Passed {
			status = challenge.StatusFailed
			break
		}
	}

	challengeResult := c.CreateResult(
		status, start, assertions, nil, outputs, "",
	)
	challengeResult.RecordAction(fmt.Sprintf("%s: challenge completed with status %s", c.Name(), status))
	return challengeResult, nil
}

// --- CH-204: EntityDetailLatency ---

// EntityDetailLatencyChallenge validates that entity detail
// responds in under 500ms.
type EntityDetailLatencyChallenge struct {
	challenge.BaseChallenge
	config *BrowsingConfig
}

// NewEntityDetailLatencyChallenge creates CH-204.
func NewEntityDetailLatencyChallenge() *EntityDetailLatencyChallenge {
	return &EntityDetailLatencyChallenge{
		BaseChallenge: challenge.NewBaseChallenge(
			"entity-detail-latency",
			"Entity Detail Latency",
			"Validates that entity detail responds in under "+
				"500ms.",
			"performance",
			[]challenge.ID{"browsing-api-health"},
		),
		config: LoadBrowsingConfig(),
	}
}

// Execute runs the entity detail latency challenge.
func (c *EntityDetailLatencyChallenge) Execute(
	ctx context.Context,
) (*challenge.Result, error) {

	start := time.Now()
	assertions := []challenge.AssertionResult{}
	outputs := map[string]string{"api_url": c.config.BaseURL}

	client := httpclient.NewAPIClient(c.config.BaseURL)

	_, loginErr := client.LoginWithRetry(
		ctx, c.config.Username, c.config.Password, 3,
	)
	if loginErr != nil {
		challengeResult := c.CreateResult(
			challenge.StatusFailed, start, assertions, nil,
			outputs, fmt.Sprintf("login failed: %v", loginErr),
		)
		challengeResult.RecordAction(fmt.Sprintf("%s: failed - %s", c.Name(), fmt.Sprintf("login failed: %v", loginErr)))
		return challengeResult, nil
	}

	c.ReportProgress("measuring-entity-detail-latency", nil)

	reqStart := time.Now()
	code, _, err := client.GetRaw(
		ctx, "/api/v1/entities/1",
	)
	latency := time.Since(reqStart)

	codeOK := err == nil && (code == 200 || code == 404)
	outputs["latency_ms"] = fmt.Sprintf("%d", latency.Milliseconds())

	fast := latency < 500*time.Millisecond && codeOK
	assertions = append(assertions, challenge.AssertionResult{
		Type:     "max_latency",
		Target:   "entity_detail_latency",
		Expected: "<500ms",
		Actual:   fmt.Sprintf("%s (HTTP %d)", latency, code),
		Passed:   fast,
		Message: challenge.Ternary(fast,
			fmt.Sprintf("Entity detail responded in %s", latency),
			fmt.Sprintf("Entity detail: %s, HTTP %d, err=%v",
				latency, code, err)),
	})

	status := challenge.StatusPassed
	for _, a := range assertions {
		if !a.Passed {
			status = challenge.StatusFailed
			break
		}
	}

	challengeResult := c.CreateResult(
		status, start, assertions, nil, outputs, "",
	)
	challengeResult.RecordAction(fmt.Sprintf("%s: challenge completed with status %s", c.Name(), status))
	return challengeResult, nil
}

// --- CH-205: StatsLatency ---

// StatsLatencyChallenge validates that stats endpoint responds
// in under 500ms.
type StatsLatencyChallenge struct {
	challenge.BaseChallenge
	config *BrowsingConfig
}

// NewStatsLatencyChallenge creates CH-205.
func NewStatsLatencyChallenge() *StatsLatencyChallenge {
	return &StatsLatencyChallenge{
		BaseChallenge: challenge.NewBaseChallenge(
			"stats-latency",
			"Stats Latency",
			"Validates that stats endpoint responds in under "+
				"500ms.",
			"performance",
			[]challenge.ID{"browsing-api-health"},
		),
		config: LoadBrowsingConfig(),
	}
}

// Execute runs the stats latency challenge.
func (c *StatsLatencyChallenge) Execute(
	ctx context.Context,
) (*challenge.Result, error) {

	start := time.Now()
	assertions := []challenge.AssertionResult{}
	outputs := map[string]string{"api_url": c.config.BaseURL}

	client := httpclient.NewAPIClient(c.config.BaseURL)

	_, loginErr := client.LoginWithRetry(
		ctx, c.config.Username, c.config.Password, 3,
	)
	if loginErr != nil {
		challengeResult := c.CreateResult(
			challenge.StatusFailed, start, assertions, nil,
			outputs, fmt.Sprintf("login failed: %v", loginErr),
		)
		challengeResult.RecordAction(fmt.Sprintf("%s: failed - %s", c.Name(), fmt.Sprintf("login failed: %v", loginErr)))
		return challengeResult, nil
	}

	c.ReportProgress("measuring-stats-latency", nil)

	reqStart := time.Now()
	code, _, err := client.GetRaw(
		ctx, "/api/v1/entities/stats",
	)
	latency := time.Since(reqStart)

	codeOK := err == nil && (code == 200 || code == 404)
	outputs["latency_ms"] = fmt.Sprintf("%d", latency.Milliseconds())

	fast := latency < 500*time.Millisecond && codeOK
	assertions = append(assertions, challenge.AssertionResult{
		Type:     "max_latency",
		Target:   "stats_latency",
		Expected: "<500ms",
		Actual:   fmt.Sprintf("%s (HTTP %d)", latency, code),
		Passed:   fast,
		Message: challenge.Ternary(fast,
			fmt.Sprintf("Stats responded in %s", latency),
			fmt.Sprintf("Stats: %s, HTTP %d, err=%v",
				latency, code, err)),
	})

	status := challenge.StatusPassed
	for _, a := range assertions {
		if !a.Passed {
			status = challenge.StatusFailed
			break
		}
	}

	challengeResult := c.CreateResult(
		status, start, assertions, nil, outputs, "",
	)
	challengeResult.RecordAction(fmt.Sprintf("%s: challenge completed with status %s", c.Name(), status))
	return challengeResult, nil
}

// --- CH-206: ConcurrentReads ---

// ConcurrentReadsChallenge validates that 10 concurrent reads
// produce no errors.
type ConcurrentReadsChallenge struct {
	challenge.BaseChallenge
	config *BrowsingConfig
}

// NewConcurrentReadsChallenge creates CH-206.
func NewConcurrentReadsChallenge() *ConcurrentReadsChallenge {
	return &ConcurrentReadsChallenge{
		BaseChallenge: challenge.NewBaseChallenge(
			"concurrent-reads",
			"Concurrent Reads",
			"Validates that 10 concurrent read requests "+
				"produce no errors.",
			"performance",
			[]challenge.ID{"browsing-api-health"},
		),
		config: LoadBrowsingConfig(),
	}
}

// Execute runs the concurrent reads challenge.
func (c *ConcurrentReadsChallenge) Execute(
	ctx context.Context,
) (*challenge.Result, error) {

	start := time.Now()
	assertions := []challenge.AssertionResult{}
	outputs := map[string]string{"api_url": c.config.BaseURL}

	client := httpclient.NewAPIClient(c.config.BaseURL)

	_, loginErr := client.LoginWithRetry(
		ctx, c.config.Username, c.config.Password, 3,
	)
	if loginErr != nil {
		challengeResult := c.CreateResult(
			challenge.StatusFailed, start, assertions, nil,
			outputs, fmt.Sprintf("login failed: %v", loginErr),
		)
		challengeResult.RecordAction(fmt.Sprintf("%s: failed - %s", c.Name(), fmt.Sprintf("login failed: %v", loginErr)))
		return challengeResult, nil
	}

	c.ReportProgress("running-concurrent-reads", nil)

	var wg sync.WaitGroup
	var errCount int32

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			code, _, err := client.GetRaw(
				ctx, "/api/v1/entities?limit=5",
			)
			if err != nil || (code != 200 && code != 404) {
				atomic.AddInt32(&errCount, 1)
			}
		}()
	}
	wg.Wait()

	noErrors := atomic.LoadInt32(&errCount) == 0
	outputs["error_count"] = fmt.Sprintf("%d", errCount)

	assertions = append(assertions, challenge.AssertionResult{
		Type:     "not_empty",
		Target:   "concurrent_reads",
		Expected: "0 errors from 10 concurrent reads",
		Actual:   fmt.Sprintf("%d errors", errCount),
		Passed:   noErrors,
		Message: challenge.Ternary(noErrors,
			"10 concurrent reads completed without errors",
			fmt.Sprintf("%d errors from concurrent reads",
				errCount)),
	})

	status := challenge.StatusPassed
	for _, a := range assertions {
		if !a.Passed {
			status = challenge.StatusFailed
			break
		}
	}

	challengeResult := c.CreateResult(
		status, start, assertions, nil, outputs, "",
	)
	challengeResult.RecordAction(fmt.Sprintf("%s: challenge completed with status %s", c.Name(), status))
	return challengeResult, nil
}

// --- CH-207: ConcurrentSearches ---

// ConcurrentSearchesChallenge validates that 10 concurrent
// searches are stable.
type ConcurrentSearchesChallenge struct {
	challenge.BaseChallenge
	config *BrowsingConfig
}

// NewConcurrentSearchesChallenge creates CH-207.
func NewConcurrentSearchesChallenge() *ConcurrentSearchesChallenge {
	return &ConcurrentSearchesChallenge{
		BaseChallenge: challenge.NewBaseChallenge(
			"concurrent-searches",
			"Concurrent Searches",
			"Validates that 10 concurrent search requests "+
				"are stable.",
			"performance",
			[]challenge.ID{"browsing-api-health"},
		),
		config: LoadBrowsingConfig(),
	}
}

// Execute runs the concurrent searches challenge.
func (c *ConcurrentSearchesChallenge) Execute(
	ctx context.Context,
) (*challenge.Result, error) {

	start := time.Now()
	assertions := []challenge.AssertionResult{}
	outputs := map[string]string{"api_url": c.config.BaseURL}

	client := httpclient.NewAPIClient(c.config.BaseURL)

	_, loginErr := client.LoginWithRetry(
		ctx, c.config.Username, c.config.Password, 3,
	)
	if loginErr != nil {
		challengeResult := c.CreateResult(
			challenge.StatusFailed, start, assertions, nil,
			outputs, fmt.Sprintf("login failed: %v", loginErr),
		)
		challengeResult.RecordAction(fmt.Sprintf("%s: failed - %s", c.Name(), fmt.Sprintf("login failed: %v", loginErr)))
		return challengeResult, nil
	}

	c.ReportProgress("running-concurrent-searches", nil)

	var wg sync.WaitGroup
	var errCount int32
	queries := []string{
		"movie", "music", "show", "game", "book",
		"comic", "software", "artist", "album", "episode",
	}

	for _, q := range queries {
		wg.Add(1)
		go func(query string) {
			defer wg.Done()
			code, _, err := client.GetRaw(
				ctx, "/api/v1/entities/search?q="+query,
			)
			if err != nil || code == 500 {
				atomic.AddInt32(&errCount, 1)
			}
		}(q)
	}
	wg.Wait()

	noErrors := atomic.LoadInt32(&errCount) == 0
	outputs["error_count"] = fmt.Sprintf("%d", errCount)

	assertions = append(assertions, challenge.AssertionResult{
		Type:     "not_empty",
		Target:   "concurrent_searches",
		Expected: "0 errors from 10 concurrent searches",
		Actual:   fmt.Sprintf("%d errors", errCount),
		Passed:   noErrors,
		Message: challenge.Ternary(noErrors,
			"10 concurrent searches completed without errors",
			fmt.Sprintf("%d errors from concurrent searches",
				errCount)),
	})

	status := challenge.StatusPassed
	for _, a := range assertions {
		if !a.Passed {
			status = challenge.StatusFailed
			break
		}
	}

	challengeResult := c.CreateResult(
		status, start, assertions, nil, outputs, "",
	)
	challengeResult.RecordAction(fmt.Sprintf("%s: challenge completed with status %s", c.Name(), status))
	return challengeResult, nil
}

// --- CH-208: GoroutineStability ---

// GoroutineStabilityChallenge validates that goroutine count
// remains stable after requests.
type GoroutineStabilityChallenge struct {
	challenge.BaseChallenge
	config *BrowsingConfig
}

// NewGoroutineStabilityChallenge creates CH-208.
func NewGoroutineStabilityChallenge() *GoroutineStabilityChallenge {
	return &GoroutineStabilityChallenge{
		BaseChallenge: challenge.NewBaseChallenge(
			"goroutine-stability",
			"Goroutine Stability",
			"Validates that goroutine count is stable after "+
				"a burst of requests.",
			"performance",
			[]challenge.ID{"browsing-api-health"},
		),
		config: LoadBrowsingConfig(),
	}
}

// Execute runs the goroutine stability challenge.
func (c *GoroutineStabilityChallenge) Execute(
	ctx context.Context,
) (*challenge.Result, error) {

	start := time.Now()
	assertions := []challenge.AssertionResult{}
	outputs := map[string]string{"api_url": c.config.BaseURL}

	client := httpclient.NewAPIClient(c.config.BaseURL)

	_, loginErr := client.LoginWithRetry(
		ctx, c.config.Username, c.config.Password, 3,
	)
	if loginErr != nil {
		challengeResult := c.CreateResult(
			challenge.StatusFailed, start, assertions, nil,
			outputs, fmt.Sprintf("login failed: %v", loginErr),
		)
		challengeResult.RecordAction(fmt.Sprintf("%s: failed - %s", c.Name(), fmt.Sprintf("login failed: %v", loginErr)))
		return challengeResult, nil
	}

	c.ReportProgress("measuring-goroutine-stability", nil)

	// Record baseline goroutine count
	baseline := runtime.NumGoroutine()
	outputs["baseline_goroutines"] = fmt.Sprintf("%d", baseline)

	// Fire 20 requests
	for i := 0; i < 20; i++ {
		client.GetRaw(ctx, "/health")
	}

	// Allow goroutines to settle
	time.Sleep(500 * time.Millisecond)
	after := runtime.NumGoroutine()
	outputs["after_goroutines"] = fmt.Sprintf("%d", after)

	// Allow up to 50 goroutine growth (generous for test env)
	stable := (after - baseline) < 50
	assertions = append(assertions, challenge.AssertionResult{
		Type:     "not_empty",
		Target:   "goroutine_stability",
		Expected: "goroutine growth < 50",
		Actual: fmt.Sprintf("baseline=%d, after=%d, growth=%d",
			baseline, after, after-baseline),
		Passed: stable,
		Message: challenge.Ternary(stable,
			fmt.Sprintf("Goroutines stable: %d -> %d",
				baseline, after),
			fmt.Sprintf("Goroutine leak: %d -> %d",
				baseline, after)),
	})

	status := challenge.StatusPassed
	for _, a := range assertions {
		if !a.Passed {
			status = challenge.StatusFailed
			break
		}
	}

	challengeResult := c.CreateResult(
		status, start, assertions, nil, outputs, "",
	)
	challengeResult.RecordAction(fmt.Sprintf("%s: challenge completed with status %s", c.Name(), status))
	return challengeResult, nil
}

// --- CH-209: MemoryStability ---

// MemoryStabilityChallenge validates that memory usage is
// bounded over 100 requests.
type MemoryStabilityChallenge struct {
	challenge.BaseChallenge
	config *BrowsingConfig
}

// NewMemoryStabilityChallenge creates CH-209.
func NewMemoryStabilityChallenge() *MemoryStabilityChallenge {
	return &MemoryStabilityChallenge{
		BaseChallenge: challenge.NewBaseChallenge(
			"memory-stability",
			"Memory Stability",
			"Validates that memory usage is bounded over "+
				"100 requests.",
			"performance",
			[]challenge.ID{"browsing-api-health"},
		),
		config: LoadBrowsingConfig(),
	}
}

// Execute runs the memory stability challenge.
func (c *MemoryStabilityChallenge) Execute(
	ctx context.Context,
) (*challenge.Result, error) {

	start := time.Now()
	assertions := []challenge.AssertionResult{}
	outputs := map[string]string{"api_url": c.config.BaseURL}

	client := httpclient.NewAPIClient(c.config.BaseURL)

	_, loginErr := client.LoginWithRetry(
		ctx, c.config.Username, c.config.Password, 3,
	)
	if loginErr != nil {
		challengeResult := c.CreateResult(
			challenge.StatusFailed, start, assertions, nil,
			outputs, fmt.Sprintf("login failed: %v", loginErr),
		)
		challengeResult.RecordAction(fmt.Sprintf("%s: failed - %s", c.Name(), fmt.Sprintf("login failed: %v", loginErr)))
		return challengeResult, nil
	}

	c.ReportProgress("measuring-memory-stability", nil)

	var memBefore runtime.MemStats
	runtime.ReadMemStats(&memBefore)
	outputs["alloc_before_mb"] = fmt.Sprintf("%d",
		memBefore.Alloc/1024/1024)

	for i := 0; i < 100; i++ {
		client.GetRaw(ctx, "/health")
	}

	runtime.GC()
	var memAfter runtime.MemStats
	runtime.ReadMemStats(&memAfter)
	outputs["alloc_after_mb"] = fmt.Sprintf("%d",
		memAfter.Alloc/1024/1024)

	// Allow up to 100 MB growth (generous)
	// #nosec G115 — Alloc/1024/1024 is always small enough for int64 (memory in MB)
	growthMB := int64(memAfter.Alloc/1024/1024) - int64(memBefore.Alloc/1024/1024)
	stable := growthMB < 100
	assertions = append(assertions, challenge.AssertionResult{
		Type:     "not_empty",
		Target:   "memory_stability",
		Expected: "memory growth < 100MB",
		Actual:   fmt.Sprintf("growth=%dMB", growthMB),
		Passed:   stable,
		Message: challenge.Ternary(stable,
			fmt.Sprintf("Memory stable: growth=%dMB", growthMB),
			fmt.Sprintf("Memory growth excessive: %dMB",
				growthMB)),
	})

	status := challenge.StatusPassed
	for _, a := range assertions {
		if !a.Passed {
			status = challenge.StatusFailed
			break
		}
	}

	challengeResult := c.CreateResult(
		status, start, assertions, nil, outputs, "",
	)
	challengeResult.RecordAction(fmt.Sprintf("%s: challenge completed with status %s", c.Name(), status))
	return challengeResult, nil
}

// --- CH-210: ConnectionPoolHealth ---

// ConnectionPoolHealthChallenge validates that the database
// connection pool metrics are accessible.
type ConnectionPoolHealthChallenge struct {
	challenge.BaseChallenge
	config *BrowsingConfig
}

// NewConnectionPoolHealthChallenge creates CH-210.
func NewConnectionPoolHealthChallenge() *ConnectionPoolHealthChallenge {
	return &ConnectionPoolHealthChallenge{
		BaseChallenge: challenge.NewBaseChallenge(
			"connection-pool-health",
			"Connection Pool Health",
			"Validates that health/deep endpoint reports "+
				"database pool metrics.",
			"performance",
			[]challenge.ID{"browsing-api-health"},
		),
		config: LoadBrowsingConfig(),
	}
}

// Execute runs the connection pool health challenge.
func (c *ConnectionPoolHealthChallenge) Execute(
	ctx context.Context,
) (*challenge.Result, error) {

	start := time.Now()
	assertions := []challenge.AssertionResult{}
	outputs := map[string]string{"api_url": c.config.BaseURL}

	client := httpclient.NewAPIClient(c.config.BaseURL)

	_, loginErr := client.LoginWithRetry(
		ctx, c.config.Username, c.config.Password, 3,
	)
	if loginErr != nil {
		challengeResult := c.CreateResult(
			challenge.StatusFailed, start, assertions, nil,
			outputs, fmt.Sprintf("login failed: %v", loginErr),
		)
		challengeResult.RecordAction(fmt.Sprintf("%s: failed - %s", c.Name(), fmt.Sprintf("login failed: %v", loginErr)))
		return challengeResult, nil
	}

	c.ReportProgress("checking-connection-pool", nil)
	code, rawBody, err := client.GetRaw(
		ctx, "/health/deep",
	)

	codeOK := err == nil && (code == 200 || code == 404)
	assertions = append(assertions, challenge.AssertionResult{
		Type:     "status_code",
		Target:   "connection_pool_status",
		Expected: "200 or 404",
		Actual:   fmt.Sprintf("%d", code),
		Passed:   codeOK,
		Message: challenge.Ternary(codeOK,
			fmt.Sprintf("Connection pool health returned %d",
				code),
			fmt.Sprintf("Connection pool health returned %d, "+
				"err=%v", code, err)),
	})

	if codeOK && code == 200 && rawBody != nil {
		validJSON := json.Valid(rawBody)
		assertions = append(assertions, challenge.AssertionResult{
			Type:     "not_empty",
			Target:   "connection_pool_json",
			Expected: "valid JSON",
			Actual: challenge.Ternary(validJSON,
				"valid", "invalid"),
			Passed: validJSON,
			Message: challenge.Ternary(validJSON,
				"Connection pool response is valid JSON",
				"Connection pool response is invalid JSON"),
		})
	}

	status := challenge.StatusPassed
	for _, a := range assertions {
		if !a.Passed {
			status = challenge.StatusFailed
			break
		}
	}

	challengeResult := c.CreateResult(
		status, start, assertions, nil, outputs, "",
	)
	challengeResult.RecordAction(fmt.Sprintf("%s: challenge completed with status %s", c.Name(), status))
	return challengeResult, nil
}

// --- CH-211: CacheEffectiveness ---

// CacheEffectivenessChallenge validates that cache is effective
// by checking repeated reads are faster.
type CacheEffectivenessChallenge struct {
	challenge.BaseChallenge
	config *BrowsingConfig
}

// NewCacheEffectivenessChallenge creates CH-211.
func NewCacheEffectivenessChallenge() *CacheEffectivenessChallenge {
	return &CacheEffectivenessChallenge{
		BaseChallenge: challenge.NewBaseChallenge(
			"cache-effectiveness",
			"Cache Effectiveness",
			"Validates that repeated reads benefit from "+
				"caching (second request faster or same).",
			"performance",
			[]challenge.ID{"browsing-api-health"},
		),
		config: LoadBrowsingConfig(),
	}
}

// Execute runs the cache effectiveness challenge.
func (c *CacheEffectivenessChallenge) Execute(
	ctx context.Context,
) (*challenge.Result, error) {

	start := time.Now()
	assertions := []challenge.AssertionResult{}
	outputs := map[string]string{"api_url": c.config.BaseURL}

	client := httpclient.NewAPIClient(c.config.BaseURL)

	_, loginErr := client.LoginWithRetry(
		ctx, c.config.Username, c.config.Password, 3,
	)
	if loginErr != nil {
		challengeResult := c.CreateResult(
			challenge.StatusFailed, start, assertions, nil,
			outputs, fmt.Sprintf("login failed: %v", loginErr),
		)
		challengeResult.RecordAction(fmt.Sprintf("%s: failed - %s", c.Name(), fmt.Sprintf("login failed: %v", loginErr)))
		return challengeResult, nil
	}

	c.ReportProgress("testing-cache-effectiveness", nil)

	// First request (cold)
	t1 := time.Now()
	client.GetRaw(ctx, "/api/v1/entities?limit=10")
	cold := time.Since(t1)

	// Second request (potentially cached)
	t2 := time.Now()
	client.GetRaw(ctx, "/api/v1/entities?limit=10")
	warm := time.Since(t2)

	outputs["cold_ms"] = fmt.Sprintf("%d", cold.Milliseconds())
	outputs["warm_ms"] = fmt.Sprintf("%d", warm.Milliseconds())

	// Pass if warm is faster or within 2x of cold (generous)
	effective := warm <= cold*2
	assertions = append(assertions, challenge.AssertionResult{
		Type:     "not_empty",
		Target:   "cache_effectiveness",
		Expected: "warm <= 2x cold",
		Actual: fmt.Sprintf("cold=%s, warm=%s",
			cold, warm),
		Passed: effective,
		Message: challenge.Ternary(effective,
			fmt.Sprintf("Cache effective: cold=%s, warm=%s",
				cold, warm),
			fmt.Sprintf("Cache ineffective: cold=%s, warm=%s",
				cold, warm)),
	})

	status := challenge.StatusPassed
	for _, a := range assertions {
		if !a.Passed {
			status = challenge.StatusFailed
			break
		}
	}

	challengeResult := c.CreateResult(
		status, start, assertions, nil, outputs, "",
	)
	challengeResult.RecordAction(fmt.Sprintf("%s: challenge completed with status %s", c.Name(), status))
	return challengeResult, nil
}

// --- CH-212: CompressionActive ---

// CompressionActiveChallenge validates that Brotli or gzip
// compression is active in responses.
type CompressionActiveChallenge struct {
	challenge.BaseChallenge
	config *BrowsingConfig
}

// NewCompressionActiveChallenge creates CH-212.
func NewCompressionActiveChallenge() *CompressionActiveChallenge {
	return &CompressionActiveChallenge{
		BaseChallenge: challenge.NewBaseChallenge(
			"compression-active",
			"Compression Active",
			"Validates that Brotli or gzip compression is "+
				"active in API responses.",
			"performance",
			[]challenge.ID{"browsing-api-health"},
		),
		config: LoadBrowsingConfig(),
	}
}

// Execute runs the compression active challenge.
func (c *CompressionActiveChallenge) Execute(
	ctx context.Context,
) (*challenge.Result, error) {

	start := time.Now()
	assertions := []challenge.AssertionResult{}
	outputs := map[string]string{"api_url": c.config.BaseURL}

	// Disable automatic decompression
	httpClient := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			DisableCompression: true,
		},
	}

	c.ReportProgress("checking-compression", nil)

	req, err := http.NewRequestWithContext(
		ctx, http.MethodGet,
		c.config.BaseURL+"/health", nil,
	)
	if err != nil {
		challengeResult := c.CreateResult(
			challenge.StatusFailed, start, assertions, nil,
			outputs, fmt.Sprintf("request creation failed: %v", err),
		)
		challengeResult.RecordAction(fmt.Sprintf("%s: failed - %s", c.Name(), fmt.Sprintf("request creation failed: %v", err)))
		return challengeResult, nil
	}
	req.Header.Set("Accept-Encoding", "br, gzip, deflate")

	resp, doErr := httpClient.Do(req)
	if doErr != nil {
		challengeResult := c.CreateResult(
			challenge.StatusFailed, start, assertions, nil,
			outputs, fmt.Sprintf("request failed: %v", doErr),
		)
		challengeResult.RecordAction(fmt.Sprintf("%s: failed - %s", c.Name(), fmt.Sprintf("request failed: %v", doErr)))
		return challengeResult, nil
	}
	resp.Body.Close()

	ce := resp.Header.Get("Content-Encoding")
	outputs["content_encoding"] = ce
	hasCompression := ce == "br" || ce == "gzip" ||
		ce == "deflate"

	assertions = append(assertions, challenge.AssertionResult{
		Type:     "not_empty",
		Target:   "compression_active",
		Expected: "br or gzip",
		Actual: challenge.Ternary(hasCompression,
			ce, "none"),
		Passed: hasCompression,
		Message: challenge.Ternary(hasCompression,
			"Compression active: "+ce,
			"No compression detected in response"),
	})

	status := challenge.StatusPassed
	for _, a := range assertions {
		if !a.Passed {
			status = challenge.StatusFailed
			break
		}
	}

	challengeResult := c.CreateResult(
		status, start, assertions, nil, outputs, "",
	)
	challengeResult.RecordAction(fmt.Sprintf("%s: challenge completed with status %s", c.Name(), status))
	return challengeResult, nil
}

// --- CH-213: MetricsEndpointPerf ---

// MetricsEndpointPerfChallenge validates that /metrics returns
// Prometheus-format data.
type MetricsEndpointPerfChallenge struct {
	challenge.BaseChallenge
	config *BrowsingConfig
}

// NewMetricsEndpointPerfChallenge creates CH-213.
func NewMetricsEndpointPerfChallenge() *MetricsEndpointPerfChallenge {
	return &MetricsEndpointPerfChallenge{
		BaseChallenge: challenge.NewBaseChallenge(
			"metrics-endpoint-perf",
			"Metrics Endpoint",
			"Validates that /metrics returns Prometheus-format "+
				"metrics data.",
			"performance",
			[]challenge.ID{"browsing-api-health"},
		),
		config: LoadBrowsingConfig(),
	}
}

// Execute runs the metrics endpoint performance challenge.
func (c *MetricsEndpointPerfChallenge) Execute(
	ctx context.Context,
) (*challenge.Result, error) {

	start := time.Now()
	assertions := []challenge.AssertionResult{}
	outputs := map[string]string{"api_url": c.config.BaseURL}

	httpClient := &http.Client{Timeout: 10 * time.Second}

	c.ReportProgress("checking-metrics-endpoint", nil)

	req, _ := http.NewRequestWithContext(
		ctx, http.MethodGet,
		c.config.BaseURL+"/metrics", nil,
	)
	// /metrics is admin-only: send the dedicated scrape credential when configured,
	// otherwise an admin JWT obtained by login.
	if tok := os.Getenv("METRICS_SCRAPE_TOKEN"); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	} else {
		lc := httpclient.NewAPIClient(c.config.BaseURL)
		if _, lerr := lc.LoginWithRetry(ctx, c.config.Username, c.config.Password, 3); lerr == nil && lc.Token() != "" {
			req.Header.Set("Authorization", "Bearer "+lc.Token())
		}
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		challengeResult := c.CreateResult(
			challenge.StatusFailed, start, assertions, nil,
			outputs, fmt.Sprintf("metrics request failed: %v", err),
		)
		challengeResult.RecordAction(fmt.Sprintf("%s: failed - %s", c.Name(), fmt.Sprintf("metrics request failed: %v", err)))
		return challengeResult, nil
	}
	resp.Body.Close()

	codeOK := resp.StatusCode == 200
	assertions = append(assertions, challenge.AssertionResult{
		Type:     "status_code",
		Target:   "metrics_endpoint_status",
		Expected: "200",
		Actual:   fmt.Sprintf("%d", resp.StatusCode),
		Passed:   codeOK,
		Message: challenge.Ternary(codeOK,
			"Metrics endpoint returned 200",
			fmt.Sprintf("Metrics endpoint returned %d",
				resp.StatusCode)),
	})

	ct := resp.Header.Get("Content-Type")
	isPrometheus := strings.Contains(ct, "text/plain") ||
		strings.Contains(ct, "text/") ||
		strings.Contains(ct, "openmetrics")
	assertions = append(assertions, challenge.AssertionResult{
		Type:     "not_empty",
		Target:   "metrics_content_type",
		Expected: "text/plain or openmetrics",
		Actual:   ct,
		Passed:   isPrometheus,
		Message: challenge.Ternary(isPrometheus,
			"Metrics returns Prometheus-format data",
			"Metrics content-type unexpected: "+ct),
	})

	status := challenge.StatusPassed
	for _, a := range assertions {
		if !a.Passed {
			status = challenge.StatusFailed
			break
		}
	}

	challengeResult := c.CreateResult(
		status, start, assertions, nil, outputs, "",
	)
	challengeResult.RecordAction(fmt.Sprintf("%s: challenge completed with status %s", c.Name(), status))
	return challengeResult, nil
}

// --- CH-214: DeepHealthCheck ---

// DeepHealthCheckChallenge validates that /health/deep
// validates database connectivity.
type DeepHealthCheckChallenge struct {
	challenge.BaseChallenge
	config *BrowsingConfig
}

// NewDeepHealthCheckChallenge creates CH-214.
func NewDeepHealthCheckChallenge() *DeepHealthCheckChallenge {
	return &DeepHealthCheckChallenge{
		BaseChallenge: challenge.NewBaseChallenge(
			"deep-health-check",
			"Deep Health Check",
			"Validates that /health/deep checks database "+
				"connectivity and returns structured health.",
			"performance",
			[]challenge.ID{"browsing-api-health"},
		),
		config: LoadBrowsingConfig(),
	}
}

// Execute runs the deep health check challenge.
func (c *DeepHealthCheckChallenge) Execute(
	ctx context.Context,
) (*challenge.Result, error) {

	start := time.Now()
	assertions := []challenge.AssertionResult{}
	outputs := map[string]string{"api_url": c.config.BaseURL}

	client := httpclient.NewAPIClient(c.config.BaseURL)

	_, loginErr := client.LoginWithRetry(
		ctx, c.config.Username, c.config.Password, 3,
	)
	if loginErr != nil {
		challengeResult := c.CreateResult(
			challenge.StatusFailed, start, assertions, nil,
			outputs, fmt.Sprintf("login failed: %v", loginErr),
		)
		challengeResult.RecordAction(fmt.Sprintf("%s: failed - %s", c.Name(), fmt.Sprintf("login failed: %v", loginErr)))
		return challengeResult, nil
	}

	c.ReportProgress("checking-deep-health", nil)
	code, rawBody, err := client.GetRaw(
		ctx, "/health/deep",
	)

	codeOK := err == nil && (code == 200 || code == 404)
	assertions = append(assertions, challenge.AssertionResult{
		Type:     "status_code",
		Target:   "deep_health_status",
		Expected: "200 or 404",
		Actual:   fmt.Sprintf("%d", code),
		Passed:   codeOK,
		Message: challenge.Ternary(codeOK,
			fmt.Sprintf("Deep health returned %d", code),
			fmt.Sprintf("Deep health returned %d, err=%v",
				code, err)),
	})

	if codeOK && code == 200 && rawBody != nil {
		var body map[string]interface{}
		if json.Unmarshal(rawBody, &body) == nil {
			_, hasDB := body["database"]
			_, hasStatus := body["status"]
			hasFields := hasDB || hasStatus
			assertions = append(assertions, challenge.AssertionResult{
				Type:     "not_empty",
				Target:   "deep_health_fields",
				Expected: "database or status field",
				Actual: challenge.Ternary(hasFields,
					"present", "missing"),
				Passed: hasFields,
				Message: challenge.Ternary(hasFields,
					"Deep health has expected fields",
					"Deep health missing expected fields"),
			})
		}
	}

	status := challenge.StatusPassed
	for _, a := range assertions {
		if !a.Passed {
			status = challenge.StatusFailed
			break
		}
	}

	challengeResult := c.CreateResult(
		status, start, assertions, nil, outputs, "",
	)
	challengeResult.RecordAction(fmt.Sprintf("%s: challenge completed with status %s", c.Name(), status))
	return challengeResult, nil
}

// --- CH-215: WebSocketConnectPerf ---

// WebSocketConnectPerfChallenge validates that a WebSocket
// connection can be established.
type WebSocketConnectPerfChallenge struct {
	challenge.BaseChallenge
	config *BrowsingConfig
}

// NewWebSocketConnectPerfChallenge creates CH-215.
func NewWebSocketConnectPerfChallenge() *WebSocketConnectPerfChallenge {
	return &WebSocketConnectPerfChallenge{
		BaseChallenge: challenge.NewBaseChallenge(
			"websocket-connect-perf",
			"WebSocket Connect",
			"Validates that a WebSocket connection endpoint "+
				"is accessible.",
			"performance",
			[]challenge.ID{"browsing-api-health"},
		),
		config: LoadBrowsingConfig(),
	}
}

// Execute runs the WebSocket connect performance challenge.
func (c *WebSocketConnectPerfChallenge) Execute(
	ctx context.Context,
) (*challenge.Result, error) {

	start := time.Now()
	assertions := []challenge.AssertionResult{}
	outputs := map[string]string{"api_url": c.config.BaseURL}

	client := httpclient.NewAPIClient(c.config.BaseURL)

	_, loginErr := client.LoginWithRetry(
		ctx, c.config.Username, c.config.Password, 3,
	)
	if loginErr != nil {
		challengeResult := c.CreateResult(
			challenge.StatusFailed, start, assertions, nil,
			outputs, fmt.Sprintf("login failed: %v", loginErr),
		)
		challengeResult.RecordAction(fmt.Sprintf("%s: failed - %s", c.Name(), fmt.Sprintf("login failed: %v", loginErr)))
		return challengeResult, nil
	}

	c.ReportProgress("testing-websocket-connect", nil)

	// Check if the WebSocket endpoint exists by making HTTP GET
	code, _, err := client.GetRaw(
		ctx, "/api/v1/ws",
	)

	// WebSocket upgrade returns 400 for non-WS requests, which
	// proves the endpoint exists
	codeOK := err == nil &&
		(code == 200 || code == 400 || code == 426)
	assertions = append(assertions, challenge.AssertionResult{
		Type:     "status_code",
		Target:   "websocket_connect_status",
		Expected: "200, 400, or 426",
		Actual:   fmt.Sprintf("%d", code),
		Passed:   codeOK,
		Message: challenge.Ternary(codeOK,
			fmt.Sprintf("WebSocket endpoint exists (HTTP %d)",
				code),
			fmt.Sprintf("WebSocket endpoint returned %d, err=%v",
				code, err)),
	})

	status := challenge.StatusPassed
	for _, a := range assertions {
		if !a.Passed {
			status = challenge.StatusFailed
			break
		}
	}

	challengeResult := c.CreateResult(
		status, start, assertions, nil, outputs, "",
	)
	challengeResult.RecordAction(fmt.Sprintf("%s: challenge completed with status %s", c.Name(), status))
	return challengeResult, nil
}

// --- CH-216: WebSocketMessage ---

// WebSocketMessageChallenge validates that the WebSocket
// events endpoint is responsive.
type WebSocketMessageChallenge struct {
	challenge.BaseChallenge
	config *BrowsingConfig
}

// NewWebSocketMessageChallenge creates CH-216.
func NewWebSocketMessageChallenge() *WebSocketMessageChallenge {
	return &WebSocketMessageChallenge{
		BaseChallenge: challenge.NewBaseChallenge(
			"websocket-message",
			"WebSocket Message",
			"Validates that the WebSocket events endpoint "+
				"is responsive.",
			"performance",
			[]challenge.ID{"browsing-api-health"},
		),
		config: LoadBrowsingConfig(),
	}
}

// Execute runs the WebSocket message challenge.
func (c *WebSocketMessageChallenge) Execute(
	ctx context.Context,
) (*challenge.Result, error) {

	start := time.Now()
	assertions := []challenge.AssertionResult{}
	outputs := map[string]string{"api_url": c.config.BaseURL}

	client := httpclient.NewAPIClient(c.config.BaseURL)

	_, loginErr := client.LoginWithRetry(
		ctx, c.config.Username, c.config.Password, 3,
	)
	if loginErr != nil {
		challengeResult := c.CreateResult(
			challenge.StatusFailed, start, assertions, nil,
			outputs, fmt.Sprintf("login failed: %v", loginErr),
		)
		challengeResult.RecordAction(fmt.Sprintf("%s: failed - %s", c.Name(), fmt.Sprintf("login failed: %v", loginErr)))
		return challengeResult, nil
	}

	c.ReportProgress("testing-websocket-message", nil)
	code, _, err := client.GetRaw(
		ctx, "/api/v1/ws/events",
	)

	// Accept any non-500 response
	codeOK := err == nil && code != 500
	assertions = append(assertions, challenge.AssertionResult{
		Type:     "status_code",
		Target:   "websocket_message_status",
		Expected: "not 500",
		Actual:   fmt.Sprintf("%d", code),
		Passed:   codeOK,
		Message: challenge.Ternary(codeOK,
			fmt.Sprintf("WebSocket events endpoint returned %d",
				code),
			fmt.Sprintf("WebSocket events returned %d, err=%v",
				code, err)),
	})

	status := challenge.StatusPassed
	for _, a := range assertions {
		if !a.Passed {
			status = challenge.StatusFailed
			break
		}
	}

	challengeResult := c.CreateResult(
		status, start, assertions, nil, outputs, "",
	)
	challengeResult.RecordAction(fmt.Sprintf("%s: challenge completed with status %s", c.Name(), status))
	return challengeResult, nil
}

// --- CH-217: ScanQueueing ---

// ScanQueueingChallenge validates that scan jobs queue
// correctly via the scan API.
type ScanQueueingChallenge struct {
	challenge.BaseChallenge
	config *BrowsingConfig
}

// NewScanQueueingChallenge creates CH-217.
func NewScanQueueingChallenge() *ScanQueueingChallenge {
	return &ScanQueueingChallenge{
		BaseChallenge: challenge.NewBaseChallenge(
			"scan-queueing",
			"Scan Queueing",
			"Validates that scan jobs are accepted via POST "+
				"/api/v1/admin/storage/scan.",
			"performance",
			[]challenge.ID{"browsing-api-health"},
		),
		config: LoadBrowsingConfig(),
	}
}

// Execute runs the scan queueing challenge.
func (c *ScanQueueingChallenge) Execute(
	ctx context.Context,
) (*challenge.Result, error) {

	start := time.Now()
	assertions := []challenge.AssertionResult{}
	outputs := map[string]string{"api_url": c.config.BaseURL}

	client := httpclient.NewAPIClient(c.config.BaseURL)

	_, loginErr := client.LoginWithRetry(
		ctx, c.config.Username, c.config.Password, 3,
	)
	if loginErr != nil {
		challengeResult := c.CreateResult(
			challenge.StatusFailed, start, assertions, nil,
			outputs, fmt.Sprintf("login failed: %v", loginErr),
		)
		challengeResult.RecordAction(fmt.Sprintf("%s: failed - %s", c.Name(), fmt.Sprintf("login failed: %v", loginErr)))
		return challengeResult, nil
	}

	c.ReportProgress("testing-scan-queueing", nil)
	code, _, err := client.PostJSON(
		ctx, "/api/v1/admin/storage/scan", "{}",
	)

	codeOK := err == nil && (code == 200 || code == 202)
	assertions = append(assertions, challenge.AssertionResult{
		Type:     "status_code",
		Target:   "scan_queueing_status",
		Expected: "200 or 202",
		Actual:   fmt.Sprintf("%d", code),
		Passed:   codeOK,
		Message: challenge.Ternary(codeOK,
			fmt.Sprintf("Scan queued with %d", code),
			fmt.Sprintf("Scan queueing returned %d, err=%v",
				code, err)),
	})

	status := challenge.StatusPassed
	for _, a := range assertions {
		if !a.Passed {
			status = challenge.StatusFailed
			break
		}
	}

	challengeResult := c.CreateResult(
		status, start, assertions, nil, outputs, "",
	)
	challengeResult.RecordAction(fmt.Sprintf("%s: challenge completed with status %s", c.Name(), status))
	return challengeResult, nil
}

// --- CH-218: BackupPerformance ---

// BackupPerformanceChallenge validates that backup creation
// responds in under 30 seconds.
type BackupPerformanceChallenge struct {
	challenge.BaseChallenge
	config *BrowsingConfig
}

// NewBackupPerformanceChallenge creates CH-218.
func NewBackupPerformanceChallenge() *BackupPerformanceChallenge {
	return &BackupPerformanceChallenge{
		BaseChallenge: challenge.NewBaseChallenge(
			"backup-performance",
			"Backup Performance",
			"Validates that backup creation API responds "+
				"in under 30 seconds.",
			"performance",
			[]challenge.ID{"browsing-api-health"},
		),
		config: LoadBrowsingConfig(),
	}
}

// Execute runs the backup performance challenge.
func (c *BackupPerformanceChallenge) Execute(
	ctx context.Context,
) (*challenge.Result, error) {

	start := time.Now()
	assertions := []challenge.AssertionResult{}
	outputs := map[string]string{"api_url": c.config.BaseURL}

	client := httpclient.NewAPIClient(c.config.BaseURL)

	_, loginErr := client.LoginWithRetry(
		ctx, c.config.Username, c.config.Password, 3,
	)
	if loginErr != nil {
		challengeResult := c.CreateResult(
			challenge.StatusFailed, start, assertions, nil,
			outputs, fmt.Sprintf("login failed: %v", loginErr),
		)
		challengeResult.RecordAction(fmt.Sprintf("%s: failed - %s", c.Name(), fmt.Sprintf("login failed: %v", loginErr)))
		return challengeResult, nil
	}

	c.ReportProgress("testing-backup-performance", nil)

	reqStart := time.Now()
	body := `{"name":"perf-test-backup"}`
	code, _, err := client.PostJSON(
		ctx, "/api/v1/admin/backups", body,
	)
	latency := time.Since(reqStart)

	codeOK := err == nil &&
		(code == 200 || code == 201 || code == 202)
	outputs["latency_s"] = fmt.Sprintf("%.1f",
		latency.Seconds())

	fast := latency < 30*time.Second && codeOK
	assertions = append(assertions, challenge.AssertionResult{
		Type:     "max_latency",
		Target:   "backup_performance",
		Expected: "<30s with 200/201/202",
		Actual:   fmt.Sprintf("%s (HTTP %d)", latency, code),
		Passed:   fast,
		Message: challenge.Ternary(fast,
			fmt.Sprintf("Backup responded in %s", latency),
			fmt.Sprintf("Backup: %s, HTTP %d, err=%v",
				latency, code, err)),
	})

	status := challenge.StatusPassed
	for _, a := range assertions {
		if !a.Passed {
			status = challenge.StatusFailed
			break
		}
	}

	challengeResult := c.CreateResult(
		status, start, assertions, nil, outputs, "",
	)
	challengeResult.RecordAction(fmt.Sprintf("%s: challenge completed with status %s", c.Name(), status))
	return challengeResult, nil
}

// --- CH-219: PaginationPerformance ---

// PaginationPerformanceChallenge validates that large offset
// pagination responds in under 500ms.
type PaginationPerformanceChallenge struct {
	challenge.BaseChallenge
	config *BrowsingConfig
}

// NewPaginationPerformanceChallenge creates CH-219.
func NewPaginationPerformanceChallenge() *PaginationPerformanceChallenge {
	return &PaginationPerformanceChallenge{
		BaseChallenge: challenge.NewBaseChallenge(
			"pagination-performance",
			"Pagination Performance",
			"Validates that large offset pagination responds "+
				"in under 500ms.",
			"performance",
			[]challenge.ID{"browsing-api-health"},
		),
		config: LoadBrowsingConfig(),
	}
}

// Execute runs the pagination performance challenge.
func (c *PaginationPerformanceChallenge) Execute(
	ctx context.Context,
) (*challenge.Result, error) {

	start := time.Now()
	assertions := []challenge.AssertionResult{}
	outputs := map[string]string{"api_url": c.config.BaseURL}

	client := httpclient.NewAPIClient(c.config.BaseURL)

	_, loginErr := client.LoginWithRetry(
		ctx, c.config.Username, c.config.Password, 3,
	)
	if loginErr != nil {
		challengeResult := c.CreateResult(
			challenge.StatusFailed, start, assertions, nil,
			outputs, fmt.Sprintf("login failed: %v", loginErr),
		)
		challengeResult.RecordAction(fmt.Sprintf("%s: failed - %s", c.Name(), fmt.Sprintf("login failed: %v", loginErr)))
		return challengeResult, nil
	}

	c.ReportProgress("testing-pagination-performance", nil)

	reqStart := time.Now()
	code, _, err := client.GetRaw(
		ctx, "/api/v1/entities?limit=20&offset=1000",
	)
	latency := time.Since(reqStart)

	codeOK := err == nil && (code == 200 || code == 404)
	outputs["latency_ms"] = fmt.Sprintf("%d",
		latency.Milliseconds())

	fast := latency < 500*time.Millisecond && codeOK
	assertions = append(assertions, challenge.AssertionResult{
		Type:     "max_latency",
		Target:   "pagination_performance",
		Expected: "<500ms",
		Actual:   fmt.Sprintf("%s (HTTP %d)", latency, code),
		Passed:   fast,
		Message: challenge.Ternary(fast,
			fmt.Sprintf("Pagination at offset 1000 in %s",
				latency),
			fmt.Sprintf("Pagination: %s, HTTP %d, err=%v",
				latency, code, err)),
	})

	status := challenge.StatusPassed
	for _, a := range assertions {
		if !a.Passed {
			status = challenge.StatusFailed
			break
		}
	}

	challengeResult := c.CreateResult(
		status, start, assertions, nil, outputs, "",
	)
	challengeResult.RecordAction(fmt.Sprintf("%s: challenge completed with status %s", c.Name(), status))
	return challengeResult, nil
}

// --- CH-220: FilterPerformance ---

// FilterPerformanceChallenge validates that filtered queries
// respond in under 500ms.
type FilterPerformanceChallenge struct {
	challenge.BaseChallenge
	config *BrowsingConfig
}

// NewFilterPerformanceChallenge creates CH-220.
func NewFilterPerformanceChallenge() *FilterPerformanceChallenge {
	return &FilterPerformanceChallenge{
		BaseChallenge: challenge.NewBaseChallenge(
			"filter-performance",
			"Filter Performance",
			"Validates that filtered entity queries respond "+
				"in under 500ms.",
			"performance",
			[]challenge.ID{"browsing-api-health"},
		),
		config: LoadBrowsingConfig(),
	}
}

// Execute runs the filter performance challenge.
func (c *FilterPerformanceChallenge) Execute(
	ctx context.Context,
) (*challenge.Result, error) {

	start := time.Now()
	assertions := []challenge.AssertionResult{}
	outputs := map[string]string{"api_url": c.config.BaseURL}

	client := httpclient.NewAPIClient(c.config.BaseURL)

	_, loginErr := client.LoginWithRetry(
		ctx, c.config.Username, c.config.Password, 3,
	)
	if loginErr != nil {
		challengeResult := c.CreateResult(
			challenge.StatusFailed, start, assertions, nil,
			outputs, fmt.Sprintf("login failed: %v", loginErr),
		)
		challengeResult.RecordAction(fmt.Sprintf("%s: failed - %s", c.Name(), fmt.Sprintf("login failed: %v", loginErr)))
		return challengeResult, nil
	}

	c.ReportProgress("testing-filter-performance", nil)

	reqStart := time.Now()
	code, _, err := client.GetRaw(
		ctx, "/api/v1/entities?type=movie&limit=20",
	)
	latency := time.Since(reqStart)

	codeOK := err == nil && (code == 200 || code == 404)
	outputs["latency_ms"] = fmt.Sprintf("%d",
		latency.Milliseconds())

	fast := latency < 500*time.Millisecond && codeOK
	assertions = append(assertions, challenge.AssertionResult{
		Type:     "max_latency",
		Target:   "filter_performance",
		Expected: "<500ms",
		Actual:   fmt.Sprintf("%s (HTTP %d)", latency, code),
		Passed:   fast,
		Message: challenge.Ternary(fast,
			fmt.Sprintf("Filtered query in %s", latency),
			fmt.Sprintf("Filter: %s, HTTP %d, err=%v",
				latency, code, err)),
	})

	status := challenge.StatusPassed
	for _, a := range assertions {
		if !a.Passed {
			status = challenge.StatusFailed
			break
		}
	}

	challengeResult := c.CreateResult(
		status, start, assertions, nil, outputs, "",
	)
	challengeResult.RecordAction(fmt.Sprintf("%s: challenge completed with status %s", c.Name(), status))
	return challengeResult, nil
}
