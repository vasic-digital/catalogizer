package services

import (
	"encoding/json"
	"strings"
	"testing"
)

// WIZENV-1: the real service path (collectSystemInfo, then the JSON the session
// would carry) must not contain non-allow-listed environment values.
func TestWIZENV1_CollectSystemInfoDoesNotLeakEnv(t *testing.T) {
	const sentinel = "wizenv1-sentinel-value-7f3a"
	for _, k := range []string{"WIZENV1_OPENAI_API_KEY", "WIZENV1_DATABASE_DSN", "WIZENV1_AUTH_BEARER", "WIZENV1_PLAIN"} {
		t.Setenv(k, sentinel)
	}
	t.Setenv("TZ", "UTC") // control needle: allow-listed key must still pass
	svc := NewConfigurationWizardService(nil)
	info := svc.collectSystemInfo()
	raw, _ := json.Marshal(info)
	if strings.Contains(string(raw), sentinel) {
		t.Fatalf("sentinel env value present in serialized SystemInfo")
	}
	for k := range info.EnvironmentVars {
		if _, ok := wizardSafeEnvKeys[k]; !ok {
			t.Fatalf("non-allow-listed key %q exposed", k)
		}
	}
	if info.EnvironmentVars["TZ"] != "UTC" {
		t.Fatalf("control needle missing: allow-listed TZ not reported")
	}
}

func TestWIZENV1_SafeEnvironmentMalformed(t *testing.T) {
	got := safeEnvironment([]string{"NOEQUALS", "TZ=UTC", "X_API_KEY=v", "LANG="})
	if len(got) != 2 || got["TZ"] != "UTC" {
		t.Fatalf("unexpected %v", got)
	}
}
