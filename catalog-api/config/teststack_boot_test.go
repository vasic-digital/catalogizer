package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestLoadConfig_TestStackEnv_NoConfigFile boots LoadConfig the way the tracked
// test/QA stack does: no config.json, secrets only from the environment that
// scripts/tests/test_test_stack_boots.sh resolves from docker-compose.test.yml.
// It is driven by that script (STACK_BOOT_CHECK=1) and skipped otherwise.
func TestLoadConfig_TestStackEnv_NoConfigFile(t *testing.T) {
	if os.Getenv("STACK_BOOT_CHECK") != "1" {
		t.Skip("driven by scripts/tests/test_test_stack_boots.sh (STACK_BOOT_CHECK=1)")
	}
	cfg, err := LoadConfig(filepath.Join(t.TempDir(), "config.json"))
	require.NoError(t, err, "the tracked test stack's environment must boot with no config.json")
	require.NotNil(t, cfg)
}
