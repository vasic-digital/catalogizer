package main

import (
	"testing"
)

func csrfTestEnv(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestAdminCSRFOptions_PassesToggleThrough(t *testing.T) {
	if got := adminCSRFOptions(csrfTestEnv(map[string]string{"CSRF_INSECURE_DEV": "true", "APP_ENV": "development"}), nil); len(got) != 1 {
		t.Fatalf("toggle must reach the middleware options, got %d", len(got))
	}
	if got := adminCSRFOptions(csrfTestEnv(map[string]string{"APP_ENV": "development"}), nil); len(got) != 0 {
		t.Fatal("no toggle must mean secure default")
	}
}

// F2: -test-mode forces the logger env to development, but the toggle must judge the RAW APP_ENV.
func TestAdminCSRFOptions_UsesRawAppEnvNotTestModeOverride(t *testing.T) {
	for _, e := range []string{"production", "", "prod"} {
		if got := adminCSRFOptions(csrfTestEnv(map[string]string{"CSRF_INSECURE_DEV": "true", "APP_ENV": e}), nil); len(got) != 0 {
			t.Fatalf("APP_ENV=%q must refuse the toggle", e)
		}
	}
}
