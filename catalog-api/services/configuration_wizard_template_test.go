package services

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"catalogizer/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// CAT-QA-SONAR-5: a declared `template` of a file_create post-install step
// must really be loaded from the wizard's templates directory.
func newTemplateTestService(t *testing.T) *ConfigurationWizardService {
	t.Helper()
	t.Setenv("CONFIG_PATH", filepath.Join(t.TempDir(), "cfg"))
	return NewConfigurationWizardService(nil)
}

func readWrittenConfig(t *testing.T, path string) map[string]interface{} {
	t.Helper()
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	var doc map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &doc))
	cfg, ok := doc["config"].(map[string]interface{})
	require.True(t, ok, "written file has a config object: %s", raw)
	return cfg
}

func TestCAT_QA_SONAR_5_FileCreateLoadsDeclaredTemplate(t *testing.T) {
	s := newTemplateTestService(t)
	require.NoError(t, os.WriteFile(filepath.Join(s.templatesPath, "config_template.json"),
		[]byte(`{"log_level":"warn","from_template":"yes","nested":{"a":1,"b":2}}`), 0o644))
	out := filepath.Join(t.TempDir(), "out", "config.json")
	sess := &models.WizardSession{Configuration: map[string]interface{}{
		"log_level": "debug", "nested": map[string]interface{}{"b": 3},
	}}

	err := s.createConfigurationFile(map[string]interface{}{"file_path": out, "template": "config_template.json"}, sess)
	require.NoError(t, err)

	cfg := readWrittenConfig(t, out)
	assert.Equal(t, "yes", cfg["from_template"], "template key is present")
	assert.Equal(t, "debug", cfg["log_level"], "session value overrides template")
	nested := cfg["nested"].(map[string]interface{})
	assert.EqualValues(t, 1, nested["a"], "template nested key kept")
	assert.EqualValues(t, 3, nested["b"], "session nested value overrides")
}

func TestCAT_QA_SONAR_5_DeclaredTemplateMissingIsAnError(t *testing.T) {
	s := newTemplateTestService(t)
	out := filepath.Join(t.TempDir(), "config.json")
	err := s.createConfigurationFile(map[string]interface{}{"file_path": out, "template": "nope_template.json"},
		&models.WizardSession{Configuration: map[string]interface{}{"k": "v"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nope_template.json")
	_, statErr := os.Stat(out)
	assert.True(t, os.IsNotExist(statErr), "nothing is written when the declared template cannot be loaded")
}

func TestCAT_QA_SONAR_5_DeclaredTemplateInvalidJSONIsAnError(t *testing.T) {
	s := newTemplateTestService(t)
	require.NoError(t, os.WriteFile(filepath.Join(s.templatesPath, "bad.json"), []byte(`{not json`), 0o644))
	err := s.createConfigurationFile(map[string]interface{}{"file_path": filepath.Join(t.TempDir(), "c.json"), "template": "bad.json"},
		&models.WizardSession{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bad.json")
}

func TestCAT_QA_SONAR_5_TemplateNameCannotEscapeTemplatesDir(t *testing.T) {
	s := newTemplateTestService(t)
	outside := filepath.Join(filepath.Dir(s.templatesPath), "secret.json")
	require.NoError(t, os.WriteFile(outside, []byte(`{"leak":true}`), 0o644))
	for _, name := range []string{"../secret.json", outside, "sub/x.json"} {
		err := s.createConfigurationFile(map[string]interface{}{"file_path": filepath.Join(t.TempDir(), "c.json"), "template": name},
			&models.WizardSession{})
		require.Error(t, err, name)
	}
}

func TestCAT_QA_SONAR_5_NoTemplateKeepsSessionConfig(t *testing.T) {
	s := newTemplateTestService(t)
	out := filepath.Join(t.TempDir(), "config.json")
	require.NoError(t, s.createConfigurationFile(map[string]interface{}{"file_path": out},
		&models.WizardSession{Configuration: map[string]interface{}{"k": "v"}}))
	assert.Equal(t, "v", readWrittenConfig(t, out)["k"])
}

// The built-in template named by the wizard's PostInstall step must load even
// when the operator's templates directory has no copy (default deployments).
func TestCAT_QA_SONAR_5_BuiltinTemplateLoadsAndWizardStepWritesIt(t *testing.T) {
	s := newTemplateTestService(t)
	out := filepath.Join(t.TempDir(), "config.json")
	step := s.configTemplates["basic"].PostInstall[0]
	require.Equal(t, "config_template.json", step.Parameters["template"])
	params := map[string]interface{}{"file_path": out, "template": step.Parameters["template"]}
	require.NoError(t, s.createConfigurationFile(params, &models.WizardSession{Configuration: map[string]interface{}{"log_level": "debug"}}))
	cfg := readWrittenConfig(t, out)
	assert.EqualValues(t, 8080, cfg["server_port"], "built-in template default present")
	assert.Equal(t, "debug", cfg["log_level"], "session wins")
}
