package challenges

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"digital.vasic.challenges/pkg/httpclient"
)

// fetchMetrics reads GET /metrics, which is admin-only. It prefers the dedicated
// scrape credential (METRICS_SCRAPE_TOKEN, scrape-only) and otherwise logs in as the
// configured admin. The token value is never logged.
func fetchMetrics(ctx context.Context, cfg *BrowsingConfig) (int, []byte, error) {
	if tok := os.Getenv("METRICS_SCRAPE_TOKEN"); tok != "" {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, cfg.BaseURL+"/metrics", nil)
		if err != nil {
			return 0, nil, err
		}
		req.Header.Set("Authorization", "Bearer "+tok)
		resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
		if err != nil {
			return 0, nil, err
		}
		defer resp.Body.Close()
		b, err := io.ReadAll(resp.Body)
		return resp.StatusCode, b, err
	}
	client := httpclient.NewAPIClient(cfg.BaseURL)
	if _, err := client.LoginWithRetry(ctx, cfg.Username, cfg.Password, 3); err != nil {
		return 0, nil, fmt.Errorf("metrics admin login failed: %w", err)
	}
	return client.GetRaw(ctx, "/metrics")
}
