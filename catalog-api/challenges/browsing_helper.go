package challenges

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"digital.vasic.challenges/pkg/env"
)

// BrowsingConfig holds configuration for browsing challenges,
// loaded from environment variables with sensible defaults.
type BrowsingConfig struct {
	BaseURL   string // API base URL
	Username  string // Admin login username
	Password  string // Admin login password
	WebAppURL string // Web app URL for Playwright
	WebAppDir string // Path to catalog-web directory
}

// LoadBrowsingConfig reads browsing challenge configuration
// from environment variables, falling back to defaults.
func LoadBrowsingConfig() *BrowsingConfig {
	return &BrowsingConfig{
		BaseURL:   env.GetOrDefault("BROWSING_API_URL", "http://localhost:28080"),
		Username:  env.GetOrDefault("ADMIN_USERNAME", "admin"),
		Password:  env.GetOrDefault("ADMIN_PASSWORD", ""),
		WebAppURL: env.GetOrDefault("BROWSING_WEB_URL", "http://localhost:3000"),
		WebAppDir: env.GetOrDefault("CATALOG_WEB_DIR", "../catalog-web"),
	}
}

// invalidTitlePatterns lists title values that indicate missing
// or placeholder content in catalog entries.
var invalidTitlePatterns = []string{
	"unknown", "untitled", "placeholder", "n/a", "tbd", "",
}

// IsInvalidTitle checks whether a title matches any known
// invalid/placeholder pattern (case-insensitive).
func IsInvalidTitle(title string) bool {
	normalized := strings.TrimSpace(strings.ToLower(title))
	for _, pattern := range invalidTitlePatterns {
		if normalized == pattern {
			return true
		}
	}
	return false
}

// requiredMediaTypes lists the content categories that must
// be present in a fully cataloged collection.
var requiredMediaTypes = []string{
	"music", "tv_show", "movie", "software", "comic",
}

// isEndpointReachable performs a quick TCP dial to check if the
// NAS endpoint is network-reachable. Returns false if the host
// cannot be contacted within 3 seconds.
func isEndpointReachable(host string, port int) bool {
	addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	conn, err := net.DialTimeout("tcp", addr, 3*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// isWebAppReachable performs a quick HTTP GET to check if the
// web application is running. Returns false if the request fails
// or times out within 3 seconds.
func isWebAppReachable(url string) bool {
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode < 500
}
