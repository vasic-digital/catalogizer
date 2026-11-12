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

// CAT-OPB-FU (OPB-M2): a symlink in the operator templates dir that resolves
// outside that dir must not be followed.
func TestCAT_OPB_FU_M2_SymlinkTemplateOutsideDirRejected(t *testing.T) {
	s := newTemplateTestService(t)
	outside := filepath.Join(t.TempDir(), "outside.json")
	require.NoError(t, os.WriteFile(outside, []byte(`{"leak":true}`), 0o644))
	require.NoError(t, os.Symlink(outside, filepath.Join(s.templatesPath, "link.json")))
	_, err := s.loadConfigTemplate("link.json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "link.json")
}

// A symlink that stays inside the templates dir is still fine.
func TestCAT_OPB_FU_M2_SymlinkTemplateInsideDirAllowed(t *testing.T) {
	s := newTemplateTestService(t)
	require.NoError(t, os.WriteFile(filepath.Join(s.templatesPath, "real.json"), []byte(`{"ok":1}`), 0o644))
	require.NoError(t, os.Symlink("real.json", filepath.Join(s.templatesPath, "alias.json")))
	doc, err := s.loadConfigTemplate("alias.json")
	require.NoError(t, err)
	assert.EqualValues(t, 1, doc["ok"])
}

// CAT-OPB-FU (OPB-M3): an operator template that exists but cannot be read
// must error and never fall back to a same-named built-in template.
func TestCAT_OPB_FU_M3_UnreadableOperatorTemplateDoesNotFallBack(t *testing.T) {
	s := newTemplateTestService(t)
	// a directory named like the built-in: ReadFile fails with EISDIR (not IsNotExist), even as root
	require.NoError(t, os.Mkdir(filepath.Join(s.templatesPath, "config_template.json"), 0o755))
	_, err := s.loadConfigTemplate("config_template.json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot read")
}

func TestCAT_OPB_FU_M3_BackslashNameRejected(t *testing.T) {
	s := newTemplateTestService(t)
	_, err := s.loadConfigTemplate(`a\b.json`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "plain file name")
}

// CAT-OPB-FU (OPB-N1): JSON `null` is not a template object.
func TestCAT_OPB_FU_N1_NullTemplateIsAnError(t *testing.T) {
	s := newTemplateTestService(t)
	require.NoError(t, os.WriteFile(filepath.Join(s.templatesPath, "null.json"), []byte(`null`), 0o644))
	_, err := s.loadConfigTemplate("null.json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "null.json")
}

// CAT-OPB-FU (OPB-I1): a relative file_path of a file_create step is resolved
// against the wizard's configPath, not the process CWD.
func TestCAT_OPB_FU_I1_RelativeFilePathResolvedAgainstConfigPath(t *testing.T) {
	s := newTemplateTestService(t)
	cwd, err := os.Getwd()
	require.NoError(t, err)
	err = s.createConfigurationFile(map[string]interface{}{"file_path": "rel/out.json"},
		&models.WizardSession{Configuration: map[string]interface{}{"k": "v"}})
	require.NoError(t, err)
	_, statErr := os.Stat(filepath.Join(s.configPath, "rel", "out.json"))
	assert.NoError(t, statErr, "written under configPath")
	_, statErr = os.Stat(filepath.Join(cwd, "rel"))
	assert.True(t, os.IsNotExist(statErr), "nothing written under CWD")
}

// The basic template's own step must not target the CWD-relative tracked file.
func TestCAT_OPB_FU_I1_FinalizeBasicLeavesCWDUntouched(t *testing.T) {
	s := newTemplateTestService(t)
	cwd, _ := os.Getwd()
	tracked := filepath.Join(cwd, "config", "config.json")
	before, _ := os.ReadFile(tracked)
	sess := &models.WizardSession{UserID: 1, ConfigType: "basic", Configuration: map[string]interface{}{"server_port": 8080}}
	require.NoError(t, s.finalizeConfiguration(sess))
	after, _ := os.ReadFile(tracked)
	assert.Equal(t, string(before), string(after), "tracked ./config/config.json unchanged")
	_, err := os.Stat(filepath.Join(s.configPath, "config.json"))
	assert.NoError(t, err)
}

// CAT-OPB-FU (OPB-M1): a failing Required post-install step must not leave the
// freshly written config.json behind (a pre-existing one is restored).
func TestCAT_OPB_FU_M1_RequiredStepFailureRollsBackConfig(t *testing.T) {
	failing := func(s *ConfigurationWizardService) {
		s.configTemplates["failing"] = ConfigTemplate{TemplateID: "failing", PostInstall: []PostInstallAction{
			{ActionType: "file_create", Required: true, Parameters: map[string]interface{}{"file_path": "x.json", "template": "missing_template.json"}},
		}}
	}
	t.Run("no previous file: removed", func(t *testing.T) {
		s := newTemplateTestService(t)
		failing(s)
		err := s.finalizeConfiguration(&models.WizardSession{ConfigType: "failing", Configuration: map[string]interface{}{}})
		require.Error(t, err)
		_, statErr := os.Stat(filepath.Join(s.configPath, "config.json"))
		assert.True(t, os.IsNotExist(statErr))
	})
	t.Run("previous file: restored", func(t *testing.T) {
		s := newTemplateTestService(t)
		failing(s)
		p := filepath.Join(s.configPath, "config.json")
		require.NoError(t, os.WriteFile(p, []byte(`{"old":true}`), 0o644))
		err := s.finalizeConfiguration(&models.WizardSession{ConfigType: "failing", Configuration: map[string]interface{}{}})
		require.Error(t, err)
		got, _ := os.ReadFile(p)
		assert.Equal(t, `{"old":true}`, string(got))
	})
	t.Run("optional step failure keeps config", func(t *testing.T) {
		s := newTemplateTestService(t)
		s.configTemplates["opt"] = ConfigTemplate{TemplateID: "opt", PostInstall: []PostInstallAction{
			{ActionType: "bogus", Required: false},
		}}
		require.NoError(t, s.finalizeConfiguration(&models.WizardSession{ConfigType: "opt", Configuration: map[string]interface{}{}}))
		_, statErr := os.Stat(filepath.Join(s.configPath, "config.json"))
		assert.NoError(t, statErr)
	})
}
