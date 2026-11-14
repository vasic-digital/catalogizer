package services

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

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
	// Run from an empty temp dir so the result does not depend on test order or
	// on the package directory's own ./config (CAT-OPB-FU2 T3).
	cwd := t.TempDir()
	t.Chdir(cwd)
	tracked := filepath.Join(cwd, "config", "config.json")
	before, _ := os.ReadFile(tracked)
	sess := &models.WizardSession{UserID: 1, ConfigType: "basic", Configuration: map[string]interface{}{"server_port": 8080}}
	require.NoError(t, s.finalizeConfiguration(sess))
	after, _ := os.ReadFile(tracked)
	assert.Equal(t, string(before), string(after), "tracked ./config/config.json unchanged")
	_, cfgErr := os.Stat(filepath.Join(cwd, "config"))
	assert.True(t, os.IsNotExist(cfgErr), "nothing written under CWD ./config")
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

// CAT-OPB-FU2 (S1): an unreadable pre-existing config.json must fail finalize
// BEFORE writing, and must never be removed by the rollback.
func TestCAT_OPB_FU2_S1_UnreadableExistingConfigFailsBeforeWrite(t *testing.T) {
	s := newTemplateTestService(t)
	require.NoError(t, os.MkdirAll(s.configPath, 0o755))
	p := filepath.Join(s.configPath, "config.json")
	require.NoError(t, os.WriteFile(p, []byte(`{"old":true}`), 0o200))
	t.Cleanup(func() { _ = os.Chmod(p, 0o600) })
	err := s.finalizeConfiguration(&models.WizardSession{ConfigType: "basic", Configuration: map[string]interface{}{}})
	require.Error(t, err)
	require.NoError(t, os.Chmod(p, 0o600))
	got, rerr := os.ReadFile(p)
	require.NoError(t, rerr, "existing file must still exist")
	assert.Equal(t, `{"old":true}`, string(got), "existing file untouched")
}

// CAT-OPB-FU2 (S1): rollback restores the original file mode.
func TestCAT_OPB_FU2_S1_RollbackPreservesMode(t *testing.T) {
	s := newTemplateTestService(t)
	s.configTemplates["failing"] = ConfigTemplate{TemplateID: "failing", PostInstall: []PostInstallAction{
		{ActionType: "file_create", Required: true, Parameters: map[string]interface{}{"file_path": "x.json", "template": "missing_template.json"}},
	}}
	require.NoError(t, os.MkdirAll(s.configPath, 0o755))
	p := filepath.Join(s.configPath, "config.json")
	require.NoError(t, os.WriteFile(p, []byte(`{"old":true}`), 0o600))
	require.NoError(t, os.Chmod(p, 0o600))
	require.Error(t, s.finalizeConfiguration(&models.WizardSession{ConfigType: "failing", Configuration: map[string]interface{}{}}))
	fi, err := os.Stat(p)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), fi.Mode().Perm())
}

// CAT-OPB-FU2 (S1): rollback failures are reported in the returned error.
func TestCAT_OPB_FU2_S1_RollbackErrorsAreWrapped(t *testing.T) {
	s := newTemplateTestService(t)
	dir := t.TempDir()
	require.NoError(t, os.Chmod(dir, 0o500))
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
	f := filepath.Join(dir, "config.json")
	require.NoError(t, os.Chmod(dir, 0o700))
	require.NoError(t, os.WriteFile(f, []byte("new"), 0o644))
	require.NoError(t, os.Chmod(dir, 0o500))
	err := s.rollbackConfig(f, nil, 0, false)
	require.Error(t, err, "remove in a read-only dir must surface")
	err = s.rollbackConfig(f, []byte("old"), 0o644, true)
	assert.NoError(t, err, "rewriting an existing writable file in a read-only dir succeeds")
}

// CAT-OPB-FU2 (T1): templates dir reached through a symlinked parent is accepted.
func TestCAT_OPB_FU2_T1_SymlinkedTemplatesParentAllowed(t *testing.T) {
	s := newTemplateTestService(t)
	real := t.TempDir()
	realTpl := filepath.Join(real, "tpl")
	require.NoError(t, os.Mkdir(realTpl, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(realTpl, "t.json"), []byte(`{"a":1}`), 0o644))
	linkParent := filepath.Join(t.TempDir(), "lnk")
	require.NoError(t, os.Symlink(real, linkParent))
	s.templatesPath = filepath.Join(linkParent, "tpl")
	doc, err := s.loadConfigTemplate("t.json")
	require.NoError(t, err)
	assert.Equal(t, float64(1), doc["a"])
}

// CAT-OPB-FU2 (T2): an empty-object template is valid.
func TestCAT_OPB_FU2_T2_EmptyObjectTemplateLoads(t *testing.T) {
	s := newTemplateTestService(t)
	require.NoError(t, os.MkdirAll(s.templatesPath, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(s.templatesPath, "e.json"), []byte(`{}`), 0o644))
	doc, err := s.loadConfigTemplate("e.json")
	require.NoError(t, err)
	assert.NotNil(t, doc)
	assert.Empty(t, doc)
}

// CAT-OPB-FU2 / OBF-4: a Required step that fails AFTER an earlier Required
// step succeeded still rolls config.json back (multi-step rollback).
func TestOBF4_MultiStepRequiredFailureRollsBackConfig(t *testing.T) {
	for _, tc := range []struct {
		name     string
		previous string
	}{{"no previous file", ""}, {"previous file", `{"old":true}`}} {
		t.Run(tc.name, func(t *testing.T) {
			s := newTemplateTestService(t)
			s.configTemplates["multi"] = ConfigTemplate{TemplateID: "multi", PostInstall: []PostInstallAction{
				{ActionType: "file_create", Required: true, Parameters: map[string]interface{}{"file_path": "step1.json"}},
				{ActionType: "file_create", Required: true, Parameters: map[string]interface{}{"file_path": "step2.json", "template": "missing_template.json"}},
			}}
			require.NoError(t, os.MkdirAll(s.configPath, 0o755))
			p := filepath.Join(s.configPath, "config.json")
			if tc.previous != "" {
				require.NoError(t, os.WriteFile(p, []byte(tc.previous), 0o644))
			}
			err := s.finalizeConfiguration(&models.WizardSession{ConfigType: "multi", Configuration: map[string]interface{}{"k": "v"}})
			require.Error(t, err)
			_, step1Err := os.Stat(filepath.Join(s.configPath, "step1.json"))
			require.NoError(t, step1Err, "control: the first Required step really ran before the second failed")
			got, readErr := os.ReadFile(p)
			if tc.previous == "" {
				assert.True(t, os.IsNotExist(readErr), "new config.json removed after the later step failed")
			} else {
				require.NoError(t, readErr)
				assert.Equal(t, tc.previous, string(got))
			}
		})
	}
}

// OBF-4: CONFIG_PATH that is a symlink to a real directory: finalize writes
// through it and a failing Required step restores the previous file in the
// real directory without replacing the link.
func TestOBF4_SymlinkedConfigPathRollbackRestoresThroughLink(t *testing.T) {
	s := newTemplateTestService(t)
	real := t.TempDir()
	link := filepath.Join(t.TempDir(), "cfglink")
	require.NoError(t, os.Symlink(real, link))
	s.configPath = link
	s.configTemplates["failing"] = ConfigTemplate{TemplateID: "failing", PostInstall: []PostInstallAction{
		{ActionType: "file_create", Required: true, Parameters: map[string]interface{}{"file_path": "x.json", "template": "missing_template.json"}},
	}}
	require.NoError(t, os.WriteFile(filepath.Join(real, "config.json"), []byte(`{"old":true}`), 0o600))
	require.Error(t, s.finalizeConfiguration(&models.WizardSession{ConfigType: "failing", Configuration: map[string]interface{}{}}))
	got, err := os.ReadFile(filepath.Join(real, "config.json"))
	require.NoError(t, err)
	assert.Equal(t, `{"old":true}`, string(got), "previous content restored in the real directory")
	fi, err := os.Lstat(link)
	require.NoError(t, err)
	assert.NotZero(t, fi.Mode()&os.ModeSymlink, "the CONFIG_PATH link itself is still a symlink")

	// success path writes through the link too
	s.configTemplates["ok"] = ConfigTemplate{TemplateID: "ok"}
	require.NoError(t, s.finalizeConfiguration(&models.WizardSession{ConfigType: "ok", UserID: 7, Configuration: map[string]interface{}{}}))
	raw, err := os.ReadFile(filepath.Join(real, "config.json"))
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"user_id": 7`)
}

// OBF-5 (operator decision 2026-11-13): the basic template's file_create step
// targets the same config.json finalize wrote; it must MERGE, never overwrite,
// the finalize metadata (version, generated_at, generated_by, user_id,
// configuration). Finalize keys win over template content.
func TestOBF5_FileCreateMergesUnderFinalizeMetadata(t *testing.T) {
	s := newTemplateTestService(t)
	sess := &models.WizardSession{UserID: 42, ConfigType: "basic", Configuration: map[string]interface{}{"server_port": 8080}}
	require.NoError(t, s.finalizeConfiguration(sess))
	raw, err := os.ReadFile(filepath.Join(s.configPath, "config.json"))
	require.NoError(t, err)
	var doc map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &doc))
	for _, k := range []string{"version", "generated_at", "generated_by", "user_id", "configuration", "config"} {
		assert.Contains(t, doc, k, "key %q present after finalize + file_create", k)
	}
	assert.Equal(t, float64(42), doc["user_id"])
	assert.Equal(t, "3.0.0", doc["version"])
	cfg, _ := doc["config"].(map[string]interface{})
	assert.Equal(t, float64(8080), cfg["server_port"], "template/session content still written under config")
}

// OBF-5: when the existing file already holds a "config" key, the existing
// (finalize-written) value wins over the freshly merged content.
func TestOBF5_ExistingKeysWinOnDirectMerge(t *testing.T) {
	s := newTemplateTestService(t)
	require.NoError(t, os.MkdirAll(s.configPath, 0o755))
	p := filepath.Join(s.configPath, "m.json")
	require.NoError(t, os.WriteFile(p, []byte(`{"version":"3.0.0","user_id":7}`), 0o644))
	require.NoError(t, s.createConfigurationFile(map[string]interface{}{"file_path": "m.json"},
		&models.WizardSession{Configuration: map[string]interface{}{"k": "v"}}))
	raw, err := os.ReadFile(p)
	require.NoError(t, err)
	var doc map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &doc))
	assert.Equal(t, "3.0.0", doc["version"])
	assert.Equal(t, float64(7), doc["user_id"])
	assert.Equal(t, "v", doc["config"].(map[string]interface{})["k"])
}

// OBF-5: an existing file that is not a JSON object is never clobbered.
func TestOBF5_NonJSONExistingFileIsNotClobbered(t *testing.T) {
	s := newTemplateTestService(t)
	require.NoError(t, os.MkdirAll(s.configPath, 0o755))
	p := filepath.Join(s.configPath, "n.json")
	require.NoError(t, os.WriteFile(p, []byte("not json"), 0o644))
	err := s.createConfigurationFile(map[string]interface{}{"file_path": "n.json"},
		&models.WizardSession{Configuration: map[string]interface{}{"k": "v"}})
	require.Error(t, err)
	got, _ := os.ReadFile(p)
	assert.Equal(t, "not json", string(got))
}

// Nit: a relative file_path with ".." must stay inside configPath.
func TestOBF_FU_RelativeFilePathCannotEscapeConfigPath(t *testing.T) {
	s := newTemplateTestService(t)
	parent := filepath.Dir(s.configPath)
	for _, fp := range []string{"../escape.json", "a/../../escape.json"} {
		err := s.createConfigurationFile(map[string]interface{}{"file_path": fp},
			&models.WizardSession{Configuration: map[string]interface{}{"k": "v"}})
		require.Error(t, err, fp)
		_, statErr := os.Stat(filepath.Join(parent, "escape.json"))
		assert.True(t, os.IsNotExist(statErr), "nothing written outside configPath for %q", fp)
	}
	// control: an inner ".." that stays inside is fine
	require.NoError(t, s.createConfigurationFile(map[string]interface{}{"file_path": "a/../ok.json"},
		&models.WizardSession{Configuration: map[string]interface{}{"k": "v"}}))
	_, statErr := os.Stat(filepath.Join(s.configPath, "ok.json"))
	assert.NoError(t, statErr)
}

// Nit: constructing the service with the process defaults (tests pass nil)
// must not create ./config. Robust to a pre-existing package-dir ./config: it
// runs from an empty temp CWD and with CONFIG_PATH unset (the default path).
func TestOBF_FU_NilServiceDoesNotCreateRelativeConfigDir(t *testing.T) {
	cwd := t.TempDir()
	t.Chdir(cwd)
	t.Setenv("CONFIG_PATH", "")
	s := NewConfigurationWizardService(nil)
	require.NotNil(t, s)
	// Control: with the default path the constructor DOES create ./config under
	// the CWD, so the next assertion on the TestMain-configured path is meaningful.
	_, err := os.Stat(filepath.Join(cwd, "config"))
	require.NoError(t, err, "control: default CONFIG_PATH creates ./config in the CWD")
	t.Setenv("CONFIG_PATH", os.Getenv("OBF_TESTMAIN_CONFIG_PATH"))
	assert.NotEmpty(t, os.Getenv("OBF_TESTMAIN_CONFIG_PATH"), "TestMain exports its temp CONFIG_PATH")
	cwd2 := t.TempDir()
	t.Chdir(cwd2)
	s2 := NewConfigurationWizardService(nil)
	assert.NotEqual(t, "./config", s2.configPath)
	_, err = os.Stat(filepath.Join(cwd2, "config"))
	assert.True(t, os.IsNotExist(err), "no ./config created when CONFIG_PATH points at the temp dir")
}

// OBF-2: the I1 guard with a sentinel: a pre-existing ./config/config.json in
// the CWD must be byte- and mtime-identical after finalize (works on a dirty tree).
func TestOBF_FU_I2_SentinelCWDConfigUntouched(t *testing.T) {
	s := newTemplateTestService(t)
	cwd := t.TempDir()
	t.Chdir(cwd)
	require.NoError(t, os.MkdirAll("config", 0o755))
	sentinel := filepath.Join(cwd, "config", "config.json")
	require.NoError(t, os.WriteFile(sentinel, []byte(`{"sentinel":true}`), 0o644))
	old := time.Unix(1_000_000_000, 0)
	require.NoError(t, os.Chtimes(sentinel, old, old))
	require.NoError(t, s.finalizeConfiguration(&models.WizardSession{UserID: 1, ConfigType: "basic", Configuration: map[string]interface{}{"server_port": 1}}))
	fi, err := os.Stat(sentinel)
	require.NoError(t, err)
	got, _ := os.ReadFile(sentinel)
	assert.Equal(t, `{"sentinel":true}`, string(got))
	assert.True(t, fi.ModTime().Equal(old), "mtime untouched")
}

// OBF-5: on a key clash the EXISTING (finalize) value wins over the template
// content, also for a clashing "config" key.
func TestOBF5_ExistingValueWinsOnClash(t *testing.T) {
	s := newTemplateTestService(t)
	require.NoError(t, os.MkdirAll(s.configPath, 0o755))
	p := filepath.Join(s.configPath, "c.json")
	require.NoError(t, os.WriteFile(p, []byte(`{"config":{"k":"existing"}}`), 0o644))
	require.NoError(t, s.createConfigurationFile(map[string]interface{}{"file_path": "c.json"},
		&models.WizardSession{Configuration: map[string]interface{}{"k": "template", "extra": 1}}))
	got := readWrittenConfig(t, p)
	assert.Equal(t, "existing", got["k"], "existing value wins")
	assert.Equal(t, float64(1), got["extra"], "new keys merge in")
}

// OBF-1: a failing restore must be surfaced, never discarded.
func TestOBF1_RestoreWriteErrorIsSurfaced(t *testing.T) {
	s := newTemplateTestService(t)
	dir := t.TempDir()
	f := filepath.Join(dir, "config.json")
	require.NoError(t, os.WriteFile(f, []byte("new"), 0o400))
	t.Cleanup(func() { _ = os.Chmod(f, 0o600) })
	if err := os.WriteFile(f, []byte("probe"), 0o400); err == nil {
		t.Skip("file writable despite mode 0400 (running as root?)")
	}
	err := s.rollbackConfig(f, []byte("old"), 0o400, true)
	require.Error(t, err, "restore of a non-writable file must surface its error")
	assert.Contains(t, err.Error(), "restore")
}

// F1: an existing file whose content is JSON `null` is not an object: refuse.
func TestOBF_FU_ExistingNullFileIsNotClobbered(t *testing.T) {
	s := newTemplateTestService(t)
	require.NoError(t, os.MkdirAll(s.configPath, 0o755))
	p := filepath.Join(s.configPath, "null.json")
	require.NoError(t, os.WriteFile(p, []byte("null"), 0o644))
	require.Error(t, s.createConfigurationFile(map[string]interface{}{"file_path": "null.json"},
		&models.WizardSession{Configuration: map[string]interface{}{"k": "v"}}))
	got, _ := os.ReadFile(p)
	assert.Equal(t, "null", string(got))
}

// F2: an existing but unreadable file is refused, not overwritten.
func TestOBF_FU_ExistingUnreadableFileIsNotClobbered(t *testing.T) {
	s := newTemplateTestService(t)
	require.NoError(t, os.MkdirAll(s.configPath, 0o755))
	p := filepath.Join(s.configPath, "u.json")
	require.NoError(t, os.WriteFile(p, []byte(`{"a":1}`), 0o200))
	t.Cleanup(func() { _ = os.Chmod(p, 0o600) })
	if _, err := os.ReadFile(p); err == nil {
		t.Skip("file readable despite mode 0200 (running as root?)")
	}
	require.Error(t, s.createConfigurationFile(map[string]interface{}{"file_path": "u.json"},
		&models.WizardSession{Configuration: map[string]interface{}{"k": "v"}}))
	require.NoError(t, os.Chmod(p, 0o600))
	got, _ := os.ReadFile(p)
	assert.Equal(t, `{"a":1}`, string(got))
}

// F3: positive control: a name that merely starts with ".." is not an escape.
func TestOBF_FU_DotDotPrefixedNameIsAllowed(t *testing.T) {
	s := newTemplateTestService(t)
	require.NoError(t, s.createConfigurationFile(map[string]interface{}{"file_path": "..foo/ok.json"},
		&models.WizardSession{Configuration: map[string]interface{}{"k": "v"}}))
	_, err := os.Stat(filepath.Join(s.configPath, "..foo", "ok.json"))
	assert.NoError(t, err)
}

// F4: a symlink inside configPath that points outside must not be followed.
func TestOBF_FU_SymlinkInsideConfigPathCannotEscape(t *testing.T) {
	s := newTemplateTestService(t)
	require.NoError(t, os.MkdirAll(s.configPath, 0o755))
	outside := t.TempDir()
	require.NoError(t, os.Symlink(outside, filepath.Join(s.configPath, "link")))
	sess := &models.WizardSession{Configuration: map[string]interface{}{"k": "v"}}
	require.Error(t, s.createConfigurationFile(map[string]interface{}{"file_path": "link/escape.json"}, sess))
	require.Error(t, s.createConfigurationFile(map[string]interface{}{"file_path": "link/newdir/escape.json"}, sess))
	// a file symlink pointing outside is refused too
	victim := filepath.Join(outside, "victim.json")
	require.NoError(t, os.WriteFile(victim, []byte(`{"v":1}`), 0o644))
	require.NoError(t, os.Symlink(victim, filepath.Join(s.configPath, "f.json")))
	require.Error(t, s.createConfigurationFile(map[string]interface{}{"file_path": "f.json"}, sess))
	entries, _ := os.ReadDir(outside)
	assert.Len(t, entries, 1, "nothing created outside configPath")
	got, _ := os.ReadFile(victim)
	assert.Equal(t, `{"v":1}`, string(got))
	// control: a symlink that stays inside is fine
	require.NoError(t, os.MkdirAll(filepath.Join(s.configPath, "real"), 0o755))
	require.NoError(t, os.Symlink(filepath.Join(s.configPath, "real"), filepath.Join(s.configPath, "inlink")))
	require.NoError(t, s.createConfigurationFile(map[string]interface{}{"file_path": "inlink/ok.json"}, sess))
}

// F5: a non-object existing "config" value is refused, not silently replaced.
func TestOBF_FU_NonObjectExistingConfigValueRefused(t *testing.T) {
	s := newTemplateTestService(t)
	require.NoError(t, os.MkdirAll(s.configPath, 0o755))
	for i, content := range []string{`{"config":null}`, `{"config":"x"}`, `{"config":[1]}`} {
		p := filepath.Join(s.configPath, "nc.json")
		require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
		require.Error(t, s.createConfigurationFile(map[string]interface{}{"file_path": "nc.json"},
			&models.WizardSession{Configuration: map[string]interface{}{"k": "v"}}), "case %d", i)
		got, _ := os.ReadFile(p)
		assert.Equal(t, content, string(got))
	}
}

// F7: the writability probes never touch an existing test.tmp (file or symlink)
// and leave no probe file behind.
func TestOBF_FU_ProbesDoNotTouchExistingTestTmp(t *testing.T) {
	s := newTemplateTestService(t)
	dir := t.TempDir()
	victim := filepath.Join(t.TempDir(), "victim")
	require.NoError(t, os.WriteFile(victim, []byte("keep"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "test.tmp"), []byte("mine"), 0o644))
	symDir := t.TempDir()
	require.NoError(t, os.Symlink(victim, filepath.Join(symDir, "test.tmp"))) // real symlink named test.tmp
	require.NoError(t, s.testSQLiteConnection(filepath.Join(dir, "db.sqlite")))
	require.NoError(t, s.testMediaStorage(map[string]interface{}{"storage_type": "local", "media_path": dir}))
	got, err := os.ReadFile(filepath.Join(dir, "test.tmp"))
	require.NoError(t, err, "pre-existing test.tmp survives")
	assert.Equal(t, "mine", string(got))
	v, _ := os.ReadFile(victim)
	assert.Equal(t, "keep", string(v))
	entries, _ := os.ReadDir(dir)
	assert.Len(t, entries, 1, "no probe file left behind")
	// the symlink named test.tmp, in its own directory, is untouched by both probes
	require.NoError(t, s.testSQLiteConnection(filepath.Join(symDir, "db.sqlite")))
	require.NoError(t, s.testMediaStorage(map[string]interface{}{"storage_type": "local", "media_path": symDir}))
	fi, err := os.Lstat(filepath.Join(symDir, "test.tmp"))
	require.NoError(t, err)
	assert.NotZero(t, fi.Mode()&os.ModeSymlink, "test.tmp is still a symlink")
	v, _ = os.ReadFile(victim)
	assert.Equal(t, "keep", string(v), "victim behind the symlink untouched")
	entries, _ = os.ReadDir(symDir)
	assert.Len(t, entries, 1, "no probe file left behind (symlink dir)")
}

// Probe error paths: an unwritable directory is reported, not swallowed.
func TestOBF_FU_ProbesReportUnwritableDirectory(t *testing.T) {
	s := newTemplateTestService(t)
	dir := t.TempDir()
	require.NoError(t, os.Chmod(dir, 0o500))
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
	if f, err := os.CreateTemp(dir, "x"); err == nil {
		f.Close()
		t.Skip("directory writable despite mode 0500 (running as root?)")
	}
	assert.Error(t, s.testSQLiteConnection(filepath.Join(dir, "db.sqlite")))
	assert.Error(t, s.testMediaStorage(map[string]interface{}{"storage_type": "local", "media_path": dir}))
}

// Symlink class table (F4 round 3): every shape of link under configPath.
func TestOBF_FU_SymlinkShapesTable(t *testing.T) {
	sess := func() *models.WizardSession {
		return &models.WizardSession{Configuration: map[string]interface{}{"k": "v"}}
	}
	type tc struct {
		name    string
		setup   func(t *testing.T, cfg, outside string) string // returns file_path
		wantErr bool
	}
	cases := []tc{
		{"leaf file symlink outside (existing target)", func(t *testing.T, cfg, out string) string {
			require.NoError(t, os.WriteFile(filepath.Join(out, "t.json"), []byte(`{}`), 0o644))
			require.NoError(t, os.Symlink(filepath.Join(out, "t.json"), filepath.Join(cfg, "l.json")))
			return "l.json"
		}, true},
		{"leaf DANGLING symlink outside", func(t *testing.T, cfg, out string) string {
			require.NoError(t, os.Symlink(filepath.Join(out, "planted.json"), filepath.Join(cfg, "d.json")))
			return "d.json"
		}, true},
		{"leaf DANGLING symlink inside", func(t *testing.T, cfg, out string) string {
			require.NoError(t, os.Symlink(filepath.Join(cfg, "nowhere.json"), filepath.Join(cfg, "d.json")))
			return "d.json"
		}, true},
		{"dangling intermediate dir link", func(t *testing.T, cfg, out string) string {
			require.NoError(t, os.Symlink(filepath.Join(out, "nodir"), filepath.Join(cfg, "dd")))
			return "dd/x.json"
		}, true},
		{"intermediate dir symlink outside", func(t *testing.T, cfg, out string) string {
			require.NoError(t, os.Symlink(out, filepath.Join(cfg, "dl")))
			return "dl/x.json"
		}, true},
		{"link loop", func(t *testing.T, cfg, out string) string {
			require.NoError(t, os.Symlink(filepath.Join(cfg, "b"), filepath.Join(cfg, "a")))
			require.NoError(t, os.Symlink(filepath.Join(cfg, "a"), filepath.Join(cfg, "b")))
			return "a/x.json"
		}, true},
		{"chain of 2 links ending outside", func(t *testing.T, cfg, out string) string {
			require.NoError(t, os.Symlink(out, filepath.Join(cfg, "c2")))
			require.NoError(t, os.Symlink(filepath.Join(cfg, "c2"), filepath.Join(cfg, "c1")))
			return "c1/x.json"
		}, true},
		{"relative-target link escaping", func(t *testing.T, cfg, out string) string {
			rel, err := filepath.Rel(cfg, out)
			require.NoError(t, err)
			require.NoError(t, os.Symlink(rel, filepath.Join(cfg, "rl")))
			return "rl/x.json"
		}, true},
		{"relative dangling leaf escaping", func(t *testing.T, cfg, out string) string {
			rel, err := filepath.Rel(cfg, filepath.Join(out, "p.json"))
			require.NoError(t, err)
			require.NoError(t, os.Symlink(rel, filepath.Join(cfg, "rd.json")))
			return "rd.json"
		}, true},
		{"link to config dir itself (inside)", func(t *testing.T, cfg, out string) string {
			require.NoError(t, os.Symlink(cfg, filepath.Join(cfg, "self")))
			return "self/x.json"
		}, false},
		{"link inside pointing inside", func(t *testing.T, cfg, out string) string {
			require.NoError(t, os.MkdirAll(filepath.Join(cfg, "real"), 0o755))
			require.NoError(t, os.Symlink(filepath.Join(cfg, "real"), filepath.Join(cfg, "in")))
			return "in/x.json"
		}, false},
		{"leaf file symlink inside existing", func(t *testing.T, cfg, out string) string {
			require.NoError(t, os.WriteFile(filepath.Join(cfg, "real.json"), []byte(`{}`), 0o644))
			require.NoError(t, os.Symlink(filepath.Join(cfg, "real.json"), filepath.Join(cfg, "li.json")))
			return "li.json"
		}, false},
		{"no links at all", func(t *testing.T, cfg, out string) string { return "plain/x.json" }, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := newTemplateTestService(t)
			outside := t.TempDir()
			fp := c.setup(t, s.configPath, outside)
			before, _ := os.ReadDir(outside)
			err := s.createConfigurationFile(map[string]interface{}{"file_path": fp}, sess())
			after, _ := os.ReadDir(outside)
			if c.wantErr {
				require.Error(t, err)
				assert.Equal(t, len(before), len(after), "nothing created outside configPath")
				for _, e := range after {
					assert.NotEqual(t, "planted.json", e.Name())
					assert.NotEqual(t, "x.json", e.Name())
					assert.NotEqual(t, "p.json", e.Name())
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// Reviewer probe (round 2), adopted: relative write through a symlinked CONFIG_PATH succeeds
// and lands in the real directory.
func TestOBF_FU_SymlinkedConfigPathRelativeWrite(t *testing.T) {
	s := newTemplateTestService(t)
	real := t.TempDir()
	link := filepath.Join(t.TempDir(), "cfglink")
	require.NoError(t, os.Symlink(real, link))
	s.configPath = link
	require.NoError(t, s.createConfigurationFile(map[string]interface{}{"file_path": "x.json"},
		&models.WizardSession{Configuration: map[string]interface{}{"k": "v"}}))
	_, err := os.Stat(filepath.Join(real, "x.json"))
	assert.NoError(t, err, "written into the real directory behind the link")
}

// Symlinked CONFIG_PATH plus an in-config link that points into the REAL
// directory: allowed (resolved against the resolved base, not the link name).
func TestOBF_FU_SymlinkedConfigPathWithInternalLink(t *testing.T) {
	s := newTemplateTestService(t)
	real := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(real, "sub"), 0o755))
	link := filepath.Join(t.TempDir(), "cfglink")
	require.NoError(t, os.Symlink(real, link))
	require.NoError(t, os.Symlink(filepath.Join(real, "sub"), filepath.Join(real, "in")))
	s.configPath = link
	require.NoError(t, s.createConfigurationFile(map[string]interface{}{"file_path": "in/x.json"},
		&models.WizardSession{Configuration: map[string]interface{}{"k": "v"}}))
	_, err := os.Stat(filepath.Join(real, "sub", "x.json"))
	assert.NoError(t, err)
}

// OBF-FU-F1: with a RELATIVE CONFIG_PATH (the production default ./config) an in-config link whose target is ABSOLUTE and
// inside the configuration directory must be accepted (it was compared absolute-vs-relative and wrongly refused).
func TestOBF_FU_F1_RelativeConfigPathAbsoluteInsideLink(t *testing.T) {
	s := newTemplateTestService(t)
	root, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Join(root, "cfg", "sub"), 0o755))
	require.NoError(t, os.Symlink(filepath.Join(root, "cfg", "sub"), filepath.Join(root, "cfg", "in")))
	old, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(root))
	t.Cleanup(func() { _ = os.Chdir(old) })
	s.configPath = "./cfg"
	require.NoError(t, s.createConfigurationFile(map[string]interface{}{"file_path": "in/x.json"},
		&models.WizardSession{Configuration: map[string]interface{}{"k": "v"}}), "inside link with absolute target")
	_, err = os.Stat(filepath.Join(root, "cfg", "sub", "x.json"))
	assert.NoError(t, err)

	// control: an absolute link target OUTSIDE the directory is still refused with a relative CONFIG_PATH
	outside := t.TempDir()
	require.NoError(t, os.Symlink(outside, filepath.Join(root, "cfg", "out")))
	err = s.createConfigurationFile(map[string]interface{}{"file_path": "out/y.json"},
		&models.WizardSession{Configuration: map[string]interface{}{"k": "v"}})
	assert.Error(t, err, "outside link must stay refused")
	_, serr := os.Stat(filepath.Join(outside, "y.json"))
	assert.True(t, os.IsNotExist(serr), "nothing written outside")
}
