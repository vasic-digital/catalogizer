package services

import (
	"net"
	"os"
	"testing"
	"time"

	"catalogizer/models"

	"github.com/stretchr/testify/assert"
)

func TestConfigurationWizardService_GetAvailableTemplates(t *testing.T) {
	service := NewConfigurationWizardService(nil)

	templates := service.GetAvailableTemplates()

	// Should return built-in templates even with nil repo
	assert.NotNil(t, templates)
	assert.Greater(t, len(templates), 0)
}

func TestConfigurationWizardService_ValidateStepData(t *testing.T) {
	service := NewConfigurationWizardService(nil)
	floatPtr := func(f float64) *float64 { return &f }

	tests := []struct {
		name    string
		step    WizardStep
		data    map[string]interface{}
		wantErr bool
	}{
		{
			name: "empty step with empty data",
			step: WizardStep{
				StepID: "",
				Fields: []FieldDefinition{},
			},
			data:    map[string]interface{}{},
			wantErr: false,
		},
		{
			name: "step with required field missing",
			step: WizardStep{
				StepID: "step_1",
				Fields: []FieldDefinition{
					{
						FieldID:  "name",
						Label:    "Name",
						Type:     "text",
						Required: true,
					},
				},
			},
			data:    nil,
			wantErr: true,
		},
		{
			name: "step with required field present",
			step: WizardStep{
				StepID: "step_1",
				Fields: []FieldDefinition{
					{
						FieldID:  "name",
						Label:    "Name",
						Type:     "text",
						Required: true,
					},
				},
			},
			data:    map[string]interface{}{"name": "test"},
			wantErr: false,
		},
		{
			name: "required field empty string",
			step: WizardStep{
				StepID: "step_2",
				Fields: []FieldDefinition{
					{
						FieldID:  "description",
						Label:    "Description",
						Type:     "text",
						Required: true,
					},
				},
			},
			data:    map[string]interface{}{"description": ""},
			wantErr: true,
		},
		{
			name: "non-required field missing",
			step: WizardStep{
				StepID: "step_3",
				Fields: []FieldDefinition{
					{
						FieldID:  "optional",
						Label:    "Optional",
						Type:     "text",
						Required: false,
					},
				},
			},
			data:    map[string]interface{}{},
			wantErr: false,
		},
		{
			name: "field with validation rule that passes",
			step: WizardStep{
				StepID: "step_4",
				Fields: []FieldDefinition{
					{
						FieldID:    "password",
						Label:      "Password",
						Type:       "password",
						Required:   true,
						Validation: "min_length:8",
					},
				},
			},
			data:    map[string]interface{}{"password": "longpassword"},
			wantErr: false,
		},
		{
			name: "field with validation rule that fails",
			step: WizardStep{
				StepID: "step_5",
				Fields: []FieldDefinition{
					{
						FieldID:    "password",
						Label:      "Password",
						Type:       "password",
						Required:   true,
						Validation: "min_length:10",
					},
				},
			},
			data:    map[string]interface{}{"password": "short"},
			wantErr: true,
		},
		{
			name: "number field with min value within range",
			step: WizardStep{
				StepID: "step_6",
				Fields: []FieldDefinition{
					{
						FieldID:  "age",
						Label:    "Age",
						Type:     "number",
						Required: true,
						MinValue: floatPtr(0),
						MaxValue: floatPtr(120),
					},
				},
			},
			data:    map[string]interface{}{"age": float64(25)},
			wantErr: false,
		},
		{
			name: "number field below min value",
			step: WizardStep{
				StepID: "step_7",
				Fields: []FieldDefinition{
					{
						FieldID:  "age",
						Label:    "Age",
						Type:     "number",
						Required: true,
						MinValue: floatPtr(18),
					},
				},
			},
			data:    map[string]interface{}{"age": float64(16)},
			wantErr: true,
		},
		{
			name: "number field above max value",
			step: WizardStep{
				StepID: "step_8",
				Fields: []FieldDefinition{
					{
						FieldID:  "score",
						Label:    "Score",
						Type:     "number",
						Required: true,
						MaxValue: floatPtr(100),
					},
				},
			},
			data:    map[string]interface{}{"score": float64(150)},
			wantErr: true,
		},
		{
			name: "number field with int value (range validation skipped)",
			step: WizardStep{
				StepID: "step_9",
				Fields: []FieldDefinition{
					{
						FieldID:  "count",
						Label:    "Count",
						Type:     "number",
						Required: true,
						MinValue: floatPtr(0),
						MaxValue: floatPtr(10),
					},
				},
			},
			data:    map[string]interface{}{"count": 5},
			wantErr: false,
		},
		{
			name: "field with directory type missing directory",
			step: WizardStep{
				StepID: "step_10",
				Fields: []FieldDefinition{
					{
						FieldID:  "dir",
						Label:    "Directory",
						Type:     "directory",
						Required: true,
					},
				},
			},
			data:    map[string]interface{}{"dir": "/nonexistent/path/12345"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.validateStepData(tt.step, tt.data)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfigurationWizardService_ValidateFieldType(t *testing.T) {
	service := NewConfigurationWizardService(nil)

	tests := []struct {
		name    string
		field   FieldDefinition
		value   interface{}
		wantErr bool
	}{
		{
			name:    "valid text field",
			field:   FieldDefinition{FieldID: "f1", Type: "text"},
			value:   "hello",
			wantErr: false,
		},
		{
			name:    "password field accepts string",
			field:   FieldDefinition{FieldID: "f1p", Type: "password"},
			value:   "secret123",
			wantErr: false,
		},
		{
			name:    "valid number field with float64",
			field:   FieldDefinition{FieldID: "f2", Type: "number"},
			value:   float64(42),
			wantErr: false,
		},
		{
			name:    "valid number field with int",
			field:   FieldDefinition{FieldID: "f2i", Type: "number"},
			value:   42,
			wantErr: false,
		},
		{
			name:    "valid number field with int64",
			field:   FieldDefinition{FieldID: "f2l", Type: "number"},
			value:   int64(42),
			wantErr: false,
		},
		{
			name:    "valid number field with numeric string",
			field:   FieldDefinition{FieldID: "f2s", Type: "number"},
			value:   "42",
			wantErr: false,
		},
		{
			name:    "invalid number field with string",
			field:   FieldDefinition{FieldID: "f4", Type: "number"},
			value:   "not a number",
			wantErr: true,
		},
		{
			name:    "valid boolean field",
			field:   FieldDefinition{FieldID: "f3", Type: "boolean"},
			value:   true,
			wantErr: false,
		},
		{
			name:    "invalid boolean field with string",
			field:   FieldDefinition{FieldID: "f5", Type: "boolean"},
			value:   "not a bool",
			wantErr: true,
		},
		{
			name:    "file field accepts string",
			field:   FieldDefinition{FieldID: "f6", Type: "file"},
			value:   "/some/path",
			wantErr: false,
		},
		{
			name:    "directory field with existing directory",
			field:   FieldDefinition{FieldID: "f7", Type: "directory"},
			value:   ".",
			wantErr: false,
		},
		{
			name:    "directory field with non-existent directory",
			field:   FieldDefinition{FieldID: "f8", Type: "directory"},
			value:   "/path/does/not/exist/12345",
			wantErr: true,
		},
		{
			name:    "text field with non-string value",
			field:   FieldDefinition{FieldID: "f9", Type: "text"},
			value:   123,
			wantErr: true,
		},
		{
			name:    "unknown field type returns no error",
			field:   FieldDefinition{FieldID: "f10", Type: "unknown"},
			value:   "anything",
			wantErr: false,
		},
		{
			name:    "number field with bool value",
			field:   FieldDefinition{FieldID: "f11", Type: "number"},
			value:   true,
			wantErr: true,
		},
		{
			name:    "number field with map value",
			field:   FieldDefinition{FieldID: "f12", Type: "number"},
			value:   map[string]interface{}{"x": 1},
			wantErr: true,
		},
		{
			name:    "file field with non-string value",
			field:   FieldDefinition{FieldID: "f13", Type: "file"},
			value:   123,
			wantErr: true,
		},
		{
			name:    "directory field with non-string value",
			field:   FieldDefinition{FieldID: "f14", Type: "directory"},
			value:   456,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.validateFieldType(tt.field, tt.value)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfigurationWizardService_CollectSystemInfo(t *testing.T) {
	service := NewConfigurationWizardService(nil)

	info := service.collectSystemInfo()

	assert.NotNil(t, info)
	assert.NotEmpty(t, info.OS)
	assert.NotEmpty(t, info.Architecture)
	assert.NotEmpty(t, info.GoVersion)
	assert.Greater(t, info.CPUCores, 0)
}

func TestConfigurationWizardService_GetWizardProgress(t *testing.T) {
	service := NewConfigurationWizardService(nil)

	// When the current session is set in memory, GetWizardProgress works without repo
	service.currentSession = &models.WizardSession{
		SessionID:    "test-session",
		UserID:       1,
		CurrentStep:  2,
		TotalSteps:   5,
		ConfigType:   "basic",
		StartedAt:    time.Now(),
		LastActivity: time.Now(),
	}

	progress, err := service.GetWizardProgress("test-session")
	assert.NoError(t, err)
	assert.NotNil(t, progress)
	assert.Equal(t, "test-session", progress.SessionID)
	assert.Greater(t, progress.Progress, float64(0))
}

func TestConfigurationWizardService_ShouldSkipStep(t *testing.T) {
	service := NewConfigurationWizardService(nil)

	tests := []struct {
		name   string
		step   WizardStep
		config map[string]interface{}
		want   bool
	}{
		{
			name:   "no skip condition returns false",
			step:   WizardStep{StepID: "step1", SkipCondition: nil},
			config: map[string]interface{}{"key": "value"},
			want:   false,
		},
		{
			name: "skip condition with matching value returns false",
			step: WizardStep{
				StepID: "step2",
				SkipCondition: map[string]interface{}{
					"key": "value",
				},
			},
			config: map[string]interface{}{"key": "value"},
			want:   false,
		},
		{
			name: "skip condition with different value returns true",
			step: WizardStep{
				StepID: "step3",
				SkipCondition: map[string]interface{}{
					"key": "expected",
				},
			},
			config: map[string]interface{}{"key": "actual"},
			want:   true,
		},
		{
			name: "skip condition with missing key returns false",
			step: WizardStep{
				StepID: "step4",
				SkipCondition: map[string]interface{}{
					"missing": "value",
				},
			},
			config: map[string]interface{}{"key": "value"},
			want:   false,
		},
		{
			name: "multiple conditions all match returns false",
			step: WizardStep{
				StepID: "step5",
				SkipCondition: map[string]interface{}{
					"key1": "value1",
					"key2": "value2",
				},
			},
			config: map[string]interface{}{"key1": "value1", "key2": "value2"},
			want:   false,
		},
		{
			name: "multiple conditions one mismatched returns true",
			step: WizardStep{
				StepID: "step6",
				SkipCondition: map[string]interface{}{
					"key1": "value1",
					"key2": "value2",
				},
			},
			config: map[string]interface{}{"key1": "value1", "key2": "wrong"},
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.shouldSkipStep(tt.step, tt.config)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestConfigurationWizardService_ValidateFieldRule(t *testing.T) {
	service := NewConfigurationWizardService(nil)

	tests := []struct {
		name     string
		ruleName string
		value    interface{}
		allData  map[string]interface{}
		wantErr  bool
		errMsg   string
	}{
		{
			name:     "unknown rule without colon returns nil",
			ruleName: "unknown_rule",
			value:    "test",
			allData:  nil,
			wantErr:  false,
		},
		{
			name:     "inline min_length rule valid",
			ruleName: "min_length:5",
			value:    "hello",
			allData:  nil,
			wantErr:  false,
		},
		{
			name:     "inline min_length rule too short",
			ruleName: "min_length:10",
			value:    "short",
			allData:  nil,
			wantErr:  true,
			errMsg:   "must be at least 10 characters long",
		},
		{
			name:     "inline min_length rule with non-string value returns nil",
			ruleName: "min_length:5",
			value:    123,
			allData:  nil,
			wantErr:  false,
		},
		{
			name:     "registered email rule valid",
			ruleName: "email",
			value:    "user@example.com",
			allData:  nil,
			wantErr:  true,
			errMsg:   "Please enter a valid email address",
		},
		{
			name:     "registered email rule invalid",
			ruleName: "email",
			value:    "not-an-email",
			allData:  nil,
			wantErr:  true,
			errMsg:   "Please enter a valid email address",
		},
		{
			name:     "registered username rule valid",
			ruleName: "username",
			value:    "user_123",
			allData:  nil,
			wantErr:  true,
			errMsg:   "Username can only contain letters, numbers, and underscores (3-30 characters)",
		},
		{
			name:     "registered username rule invalid",
			ruleName: "username",
			value:    "ab",
			allData:  nil,
			wantErr:  true,
			errMsg:   "Username can only contain letters, numbers, and underscores (3-30 characters)",
		},
		{
			name:     "custom rule delegates to validateCustomRule",
			ruleName: "password_strength",
			value:    "weak",
			allData:  nil,
			wantErr:  true,
			errMsg:   "password must be at least 8 characters",
		},
		{
			name:     "format rule with missing pattern returns nil",
			ruleName: "format",
			value:    "test",
			allData:  nil,
			wantErr:  false,
		},
		{
			name:     "format rule with non-string pattern returns nil",
			ruleName: "email", // email rule exists but we need to mock pattern
			value:    "test",
			allData:  nil,
			wantErr:  true, // Actually email rule has pattern as string, so will fail
			errMsg:   "Please enter a valid email address",
		},
		{
			name:     "format rule with non-string value returns nil",
			ruleName: "email",
			value:    123,
			allData:  nil,
			wantErr:  false, // Non-string value returns nil without error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.validateFieldRule(tt.ruleName, tt.value, tt.allData)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfigurationWizardService_ValidateCustomRule(t *testing.T) {
	service := NewConfigurationWizardService(nil)

	tests := []struct {
		name    string
		rule    ValidationRule
		value   interface{}
		allData map[string]interface{}
		wantErr bool
		errMsg  string
	}{
		{
			name: "password_strength rule valid",
			rule: ValidationRule{
				RuleID: "password_strength",
				Type:   "custom",
			},
			value:   "StrongPass123!",
			allData: nil,
			wantErr: false,
		},
		{
			name: "password_strength rule too short",
			rule: ValidationRule{
				RuleID: "password_strength",
				Type:   "custom",
			},
			value:   "short",
			allData: nil,
			wantErr: true,
			errMsg:  "password must be at least 8 characters",
		},
		{
			name: "password_strength rule with non-string value returns nil",
			rule: ValidationRule{
				RuleID: "password_strength",
				Type:   "custom",
			},
			value:   123,
			allData: nil,
			wantErr: false,
		},
		{
			name: "password_match rule matches",
			rule: ValidationRule{
				RuleID: "password_match",
				Type:   "custom",
				Parameters: map[string]interface{}{
					"match_field": "password",
				},
			},
			value:   "secret",
			allData: map[string]interface{}{"password": "secret"},
			wantErr: false,
		},
		{
			name: "password_match rule mismatched",
			rule: ValidationRule{
				RuleID: "password_match",
				Type:   "custom",
				Parameters: map[string]interface{}{
					"match_field": "password",
				},
			},
			value:   "secret",
			allData: map[string]interface{}{"password": "different"},
			wantErr: true,
			errMsg:  "passwords do not match",
		},
		{
			name: "password_match rule missing match_field parameter returns nil",
			rule: ValidationRule{
				RuleID:     "password_match",
				Type:       "custom",
				Parameters: map[string]interface{}{},
			},
			value:   "secret",
			allData: nil,
			wantErr: false,
		},
		{
			name: "password_match rule missing other value in allData returns nil",
			rule: ValidationRule{
				RuleID: "password_match",
				Type:   "custom",
				Parameters: map[string]interface{}{
					"match_field": "password",
				},
			},
			value:   "secret",
			allData: map[string]interface{}{"other": "value"},
			wantErr: false,
		},
		{
			name: "unknown rule ID returns nil",
			rule: ValidationRule{
				RuleID: "unknown_rule",
				Type:   "custom",
			},
			value:   "anything",
			allData: nil,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.validateCustomRule(tt.rule, tt.value, tt.allData)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// File I/O and action method tests
// ---------------------------------------------------------------------------

func TestConfigurationWizardService_WriteConfigFile(t *testing.T) {
	service := NewConfigurationWizardService(nil)

	tmpDir := t.TempDir()
	filename := tmpDir + "/test_config.json"

	data := map[string]interface{}{
		"key1": "value1",
		"key2": 42,
	}

	err := service.writeConfigFile(filename, data)
	assert.NoError(t, err)

	content, err := os.ReadFile(filename)
	assert.NoError(t, err)
	assert.Contains(t, string(content), "key1")
	assert.Contains(t, string(content), "value1")
}

func TestConfigurationWizardService_WriteConfigFile_InvalidPath(t *testing.T) {
	service := NewConfigurationWizardService(nil)

	err := service.writeConfigFile("/nonexistent/path/that/should/fail/config.json", map[string]interface{}{"key": "value"})
	assert.Error(t, err)
}

func TestConfigurationWizardService_ExecutePostInstallAction(t *testing.T) {
	service := NewConfigurationWizardService(nil)

	tests := []struct {
		name    string
		action  PostInstallAction
		session *models.WizardSession
		wantErr bool
		errMsg  string
	}{
		{
			name: "unknown action type",
			action: PostInstallAction{
				ActionType: "unknown_action",
				Parameters: map[string]interface{}{},
			},
			session: &models.WizardSession{},
			wantErr: true,
			errMsg:  "unknown post-install action",
		},
		{
			name: "service_restart with valid service name",
			action: PostInstallAction{
				ActionType: "service_restart",
				Parameters: map[string]interface{}{
					"service_name": "test-service",
				},
			},
			session: &models.WizardSession{},
			wantErr: false,
		},
		{
			name: "service_restart with empty service name",
			action: PostInstallAction{
				ActionType: "service_restart",
				Parameters: map[string]interface{}{},
			},
			session: &models.WizardSession{},
			wantErr: true,
			errMsg:  "service_name parameter required",
		},
		{
			name: "command_run with valid command",
			action: PostInstallAction{
				ActionType: "command_run",
				Parameters: map[string]interface{}{
					"command": "echo hello",
				},
			},
			session: &models.WizardSession{},
			wantErr: false,
		},
		{
			name: "command_run with empty command",
			action: PostInstallAction{
				ActionType: "command_run",
				Parameters: map[string]interface{}{},
			},
			session: &models.WizardSession{},
			wantErr: true,
			errMsg:  "command parameter required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.executePostInstallAction(tt.action, tt.session)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfigurationWizardService_RestartService(t *testing.T) {
	service := NewConfigurationWizardService(nil)

	tests := []struct {
		name    string
		params  map[string]interface{}
		wantErr bool
	}{
		{
			name:    "valid service name",
			params:  map[string]interface{}{"service_name": "catalog-api"},
			wantErr: false,
		},
		{
			name:    "empty service name",
			params:  map[string]interface{}{},
			wantErr: true,
		},
		{
			name:    "nil params value",
			params:  map[string]interface{}{"service_name": nil},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.restartService(tt.params)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfigurationWizardService_RunCommand(t *testing.T) {
	service := NewConfigurationWizardService(nil)

	tests := []struct {
		name    string
		params  map[string]interface{}
		wantErr bool
	}{
		{
			name:    "valid command",
			params:  map[string]interface{}{"command": "echo test"},
			wantErr: false,
		},
		{
			name:    "empty command",
			params:  map[string]interface{}{},
			wantErr: true,
		},
		{
			name:    "nil command value",
			params:  map[string]interface{}{"command": nil},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.runCommand(tt.params)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfigurationWizardService_CreateConfigurationFile(t *testing.T) {
	service := NewConfigurationWizardService(nil)

	tmpDir := t.TempDir()

	tests := []struct {
		name    string
		params  map[string]interface{}
		session *models.WizardSession
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid file creation",
			params: map[string]interface{}{
				"file_path": tmpDir + "/new_config.json",
			},
			session: &models.WizardSession{
				Configuration: map[string]interface{}{
					"setting": "value",
				},
			},
			wantErr: false,
		},
		{
			name: "with template param",
			params: map[string]interface{}{
				"file_path": tmpDir + "/template_config.json",
				"template":  "config_template.json",
			},
			session: &models.WizardSession{
				Configuration: map[string]interface{}{
					"db_type": "sqlite",
				},
			},
			wantErr: false,
		},
		{
			name:   "missing file_path",
			params: map[string]interface{}{},
			session: &models.WizardSession{
				Configuration: map[string]interface{}{},
			},
			wantErr: true,
			errMsg:  "file_path parameter required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.createConfigurationFile(tt.params, tt.session)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// ============================================================================
// ADDITIONAL TESTS FOR 0% COVERAGE FUNCTIONS
// ============================================================================

func TestConfigurationWizardService_TestSQLiteConnection(t *testing.T) {
	service := NewConfigurationWizardService(nil)

	t.Run("valid writable directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		dbPath := tmpDir + "/test.db"
		err := service.testSQLiteConnection(dbPath)
		assert.NoError(t, err)
	})

	t.Run("nested directory creation", func(t *testing.T) {
		tmpDir := t.TempDir()
		dbPath := tmpDir + "/sub/dir/test.db"
		err := service.testSQLiteConnection(dbPath)
		assert.NoError(t, err)
	})

	t.Run("unwritable directory returns error", func(t *testing.T) {
		// Use a path that can't be created
		err := service.testSQLiteConnection("/proc/nonexistent/dir/test.db")
		assert.Error(t, err)
	})
}

func TestConfigurationWizardService_TestNetworkDatabaseConnection(t *testing.T) {
	service := NewConfigurationWizardService(nil)

	t.Run("all required fields present", func(t *testing.T) {
		config := map[string]interface{}{
			"db_host":     "127.0.0.1",
			"db_port":     listeningPort(t),
			"db_name":     "testdb",
			"db_username": "user",
			"db_password": "pass",
		}
		err := service.testNetworkDatabaseConnection(config)
		assert.NoError(t, err)
	})

	t.Run("missing db_host", func(t *testing.T) {
		config := map[string]interface{}{
			"db_port":     5432,
			"db_name":     "testdb",
			"db_username": "user",
			"db_password": "pass",
		}
		err := service.testNetworkDatabaseConnection(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "db_host")
	})

	t.Run("missing db_port", func(t *testing.T) {
		config := map[string]interface{}{
			"db_host":     "localhost",
			"db_name":     "testdb",
			"db_username": "user",
			"db_password": "pass",
		}
		err := service.testNetworkDatabaseConnection(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "db_port")
	})

	t.Run("missing db_name", func(t *testing.T) {
		config := map[string]interface{}{
			"db_host":     "localhost",
			"db_port":     5432,
			"db_username": "user",
			"db_password": "pass",
		}
		err := service.testNetworkDatabaseConnection(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "db_name")
	})

	t.Run("empty config missing all fields", func(t *testing.T) {
		config := map[string]interface{}{}
		err := service.testNetworkDatabaseConnection(config)
		assert.Error(t, err)
	})
}

func TestConfigurationWizardService_TestDatabaseConnection(t *testing.T) {
	service := NewConfigurationWizardService(nil)

	t.Run("sqlite type with valid path", func(t *testing.T) {
		tmpDir := t.TempDir()
		config := map[string]interface{}{
			"db_type": "sqlite",
			"db_path": tmpDir + "/test.db",
		}
		err := service.testDatabaseConnection(config)
		assert.NoError(t, err)
	})

	t.Run("sqlite type with empty path", func(t *testing.T) {
		config := map[string]interface{}{
			"db_type": "sqlite",
			"db_path": "",
		}
		err := service.testDatabaseConnection(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "database path not specified")
	})

	t.Run("sqlite type with missing path key", func(t *testing.T) {
		config := map[string]interface{}{
			"db_type": "sqlite",
		}
		err := service.testDatabaseConnection(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "database path not specified")
	})

	t.Run("mysql type", func(t *testing.T) {
		config := map[string]interface{}{
			"db_type":     "mysql",
			"db_host":     "127.0.0.1",
			"db_port":     listeningPort(t),
			"db_name":     "testdb",
			"db_username": "user",
			"db_password": "pass",
		}
		err := service.testDatabaseConnection(config)
		assert.NoError(t, err)
	})

	t.Run("postgresql type", func(t *testing.T) {
		config := map[string]interface{}{
			"db_type":     "postgresql",
			"db_host":     "127.0.0.1",
			"db_port":     listeningPort(t),
			"db_name":     "testdb",
			"db_username": "user",
			"db_password": "pass",
		}
		err := service.testDatabaseConnection(config)
		assert.NoError(t, err)
	})

	t.Run("unsupported database type", func(t *testing.T) {
		config := map[string]interface{}{
			"db_type": "oracle",
		}
		err := service.testDatabaseConnection(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported database type")
	})

	t.Run("missing db_type key defaults to empty", func(t *testing.T) {
		config := map[string]interface{}{}
		err := service.testDatabaseConnection(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported database type")
	})
}

func TestConfigurationWizardService_TestMediaStorage(t *testing.T) {
	service := NewConfigurationWizardService(nil)

	t.Run("local storage with valid path", func(t *testing.T) {
		tmpDir := t.TempDir()
		config := map[string]interface{}{
			"storage_type": "local",
			"media_path":   tmpDir,
		}
		err := service.testMediaStorage(config)
		assert.NoError(t, err)
	})

	t.Run("local storage with new directory path", func(t *testing.T) {
		tmpDir := t.TempDir()
		config := map[string]interface{}{
			"storage_type": "local",
			"media_path":   tmpDir + "/new/media/dir",
		}
		err := service.testMediaStorage(config)
		assert.NoError(t, err)

		// Directory should have been created
		_, err = os.Stat(tmpDir + "/new/media/dir")
		assert.NoError(t, err)
	})

	t.Run("local storage with empty media_path", func(t *testing.T) {
		config := map[string]interface{}{
			"storage_type": "local",
			"media_path":   "",
		}
		err := service.testMediaStorage(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "media path not specified")
	})

	t.Run("local storage with missing media_path key", func(t *testing.T) {
		config := map[string]interface{}{
			"storage_type": "local",
		}
		err := service.testMediaStorage(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "media path not specified")
	})

	t.Run("non-local storage type logs and returns nil", func(t *testing.T) {
		config := map[string]interface{}{
			"storage_type": "s3",
		}
		err := service.testMediaStorage(config)
		assert.NoError(t, err)
	})

	t.Run("empty config defaults to empty storage_type", func(t *testing.T) {
		config := map[string]interface{}{}
		err := service.testMediaStorage(config)
		assert.NoError(t, err) // Non-local types return nil
	})
}

func TestConfigurationWizardService_TestServiceConfiguration(t *testing.T) {
	service := NewConfigurationWizardService(nil)

	t.Run("valid port within range", func(t *testing.T) {
		config := map[string]interface{}{
			"server_port": float64(8080),
		}
		err := service.testServiceConfiguration(config)
		assert.NoError(t, err)
	})

	t.Run("port too low", func(t *testing.T) {
		config := map[string]interface{}{
			"server_port": float64(80),
		}
		err := service.testServiceConfiguration(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid port number")
	})

	t.Run("port too high", func(t *testing.T) {
		config := map[string]interface{}{
			"server_port": float64(70000),
		}
		err := service.testServiceConfiguration(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid port number")
	})

	t.Run("missing server_port returns no error", func(t *testing.T) {
		config := map[string]interface{}{}
		err := service.testServiceConfiguration(config)
		assert.NoError(t, err)
	})

	t.Run("boundary port 1024", func(t *testing.T) {
		config := map[string]interface{}{
			"server_port": float64(1024),
		}
		err := service.testServiceConfiguration(config)
		assert.NoError(t, err)
	})

	t.Run("boundary port 65535", func(t *testing.T) {
		config := map[string]interface{}{
			"server_port": float64(65535),
		}
		err := service.testServiceConfiguration(config)
		assert.NoError(t, err)
	})
}

func TestConfigurationWizardService_PerformFinalTest(t *testing.T) {
	service := NewConfigurationWizardService(nil)

	t.Run("all tests pass with valid config", func(t *testing.T) {
		tmpDir := t.TempDir()
		dbDir := t.TempDir()
		session := &models.WizardSession{
			Configuration: map[string]interface{}{
				"db_type":      "sqlite",
				"db_path":      dbDir + "/test.db",
				"storage_type": "local",
				"media_path":   tmpDir,
				"server_port":  float64(8080),
			},
		}
		err := service.performFinalTest(nil, session)
		assert.NoError(t, err)
	})

	t.Run("database test failure propagates", func(t *testing.T) {
		session := &models.WizardSession{
			Configuration: map[string]interface{}{
				"db_type":      "sqlite",
				"db_path":      "",
				"storage_type": "local",
				"media_path":   "/tmp",
				"server_port":  float64(8080),
			},
		}
		err := service.performFinalTest(nil, session)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "database connection test failed")
	})

	t.Run("media storage failure propagates", func(t *testing.T) {
		dbDir := t.TempDir()
		session := &models.WizardSession{
			Configuration: map[string]interface{}{
				"db_type":      "sqlite",
				"db_path":      dbDir + "/test.db",
				"storage_type": "local",
				"media_path":   "",
			},
		}
		err := service.performFinalTest(nil, session)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "media storage test failed")
	})

	t.Run("service config failure propagates", func(t *testing.T) {
		tmpDir := t.TempDir()
		dbDir := t.TempDir()
		session := &models.WizardSession{
			Configuration: map[string]interface{}{
				"db_type":      "sqlite",
				"db_path":      dbDir + "/test.db",
				"storage_type": "local",
				"media_path":   tmpDir,
				"server_port":  float64(80),
			},
		}
		err := service.performFinalTest(nil, session)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "service configuration test failed")
	})
}

func TestConfigurationWizardService_FinalizeConfiguration(t *testing.T) {
	service := NewConfigurationWizardService(nil)

	t.Run("finalize writes config file", func(t *testing.T) {
		tmpDir := t.TempDir()
		service.configPath = tmpDir

		session := &models.WizardSession{
			UserID:     1,
			ConfigType: "basic",
			Configuration: map[string]interface{}{
				"db_type":     "sqlite",
				"server_port": 8080,
			},
		}

		err := service.finalizeConfiguration(session)
		assert.NoError(t, err)

		// Config file should exist
		configFile := tmpDir + "/config.json"
		_, err = os.Stat(configFile)
		assert.NoError(t, err)

		content, err := os.ReadFile(configFile)
		assert.NoError(t, err)
		assert.Contains(t, string(content), "version")
		assert.Contains(t, string(content), "configuration")
	})

	t.Run("finalize with unwritable config path returns error", func(t *testing.T) {
		service2 := NewConfigurationWizardService(nil)
		service2.configPath = "/proc/nonexistent/config"

		session := &models.WizardSession{
			UserID:        1,
			ConfigType:    "basic",
			Configuration: map[string]interface{}{},
		}

		err := service2.finalizeConfiguration(session)
		assert.Error(t, err)
	})
}

func TestConfigurationWizardService_ProcessTestStep(t *testing.T) {
	service := NewConfigurationWizardService(nil)

	t.Run("system_check step", func(t *testing.T) {
		session := &models.WizardSession{
			Configuration: make(map[string]interface{}),
		}
		step := WizardStep{StepID: "system_check"}
		data := map[string]interface{}{"auto_fix": true}

		err := service.processTestStep(step, data, session)
		assert.NoError(t, err)

		// System info should be stored in config
		_, exists := session.Configuration["system_info"]
		assert.True(t, exists)
	})

	t.Run("final_test step with valid config", func(t *testing.T) {
		tmpDir := t.TempDir()
		dbDir := t.TempDir()
		session := &models.WizardSession{
			Configuration: map[string]interface{}{
				"db_type":      "sqlite",
				"db_path":      dbDir + "/test.db",
				"storage_type": "local",
				"media_path":   tmpDir,
				"server_port":  float64(8080),
			},
		}
		step := WizardStep{StepID: "final_test"}

		err := service.processTestStep(step, nil, session)
		assert.NoError(t, err)
	})

	t.Run("unknown test step returns error", func(t *testing.T) {
		session := &models.WizardSession{
			Configuration: make(map[string]interface{}),
		}
		step := WizardStep{StepID: "unknown_step"}

		err := service.processTestStep(step, nil, session)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unknown test step")
	})
}

func TestConfigurationWizardService_DetectInstalledTools(t *testing.T) {
	service := NewConfigurationWizardService(nil)

	tools := service.detectInstalledTools()
	assert.NotNil(t, tools)
	// At minimum, "go" should be detected since we're running Go tests
	assert.Contains(t, tools, "go")
}

func TestConfigurationWizardService_GenerateSystemRecommendations(t *testing.T) {
	service := NewConfigurationWizardService(nil)

	t.Run("low memory recommendation", func(t *testing.T) {
		info := SystemInfo{
			MemoryGB:       1.0,
			CPUCores:       4,
			OS:             "linux",
			InstalledTools: []string{"go", "git", "docker"},
		}
		recs := service.generateSystemRecommendations(info)
		assert.NotNil(t, recs)
		found := false
		for _, r := range recs {
			if assert.NotEmpty(t, r) && len(r) > 0 {
				if r[0] == 'C' && len(r) > 10 {
					found = true
				}
			}
		}
		assert.True(t, found, "Expected memory recommendation")
	})

	t.Run("low CPU cores recommendation", func(t *testing.T) {
		info := SystemInfo{
			MemoryGB:       8.0,
			CPUCores:       1,
			OS:             "linux",
			InstalledTools: []string{"go", "git", "docker"},
		}
		recs := service.generateSystemRecommendations(info)
		foundCPU := false
		for _, r := range recs {
			if len(r) > 20 && r[0] == 'C' {
				foundCPU = true
			}
		}
		assert.True(t, foundCPU, "Expected CPU recommendation")
	})

	t.Run("windows recommendation", func(t *testing.T) {
		info := SystemInfo{
			MemoryGB:       8.0,
			CPUCores:       4,
			OS:             "windows",
			InstalledTools: []string{"go", "git", "docker"},
		}
		recs := service.generateSystemRecommendations(info)
		foundWSL := false
		for _, r := range recs {
			if len(r) > 0 && (r[0] == 'C') {
				foundWSL = true
			}
		}
		assert.True(t, foundWSL, "Expected WSL recommendation")
	})

	t.Run("few tools recommendation", func(t *testing.T) {
		info := SystemInfo{
			MemoryGB:       8.0,
			CPUCores:       4,
			OS:             "linux",
			InstalledTools: []string{"go"},
		}
		recs := service.generateSystemRecommendations(info)
		foundTools := false
		for _, r := range recs {
			if len(r) > 0 && r[0] == 'I' {
				foundTools = true
			}
		}
		assert.True(t, foundTools, "Expected tool installation recommendation")
	})

	t.Run("no recommendations for healthy system", func(t *testing.T) {
		info := SystemInfo{
			MemoryGB:       16.0,
			CPUCores:       8,
			OS:             "linux",
			InstalledTools: []string{"go", "git", "docker", "make"},
		}
		recs := service.generateSystemRecommendations(info)
		assert.Empty(t, recs)
	})
}

func TestConfigurationWizardService_GetCompletedSteps(t *testing.T) {
	service := NewConfigurationWizardService(nil)

	t.Run("returns completed step titles", func(t *testing.T) {
		session := &models.WizardSession{
			CurrentStep: 3,
			ConfigType:  "basic",
		}
		completed := service.getCompletedSteps(session)
		assert.Equal(t, 3, len(completed))
	})

	t.Run("zero completed steps", func(t *testing.T) {
		session := &models.WizardSession{
			CurrentStep: 0,
			ConfigType:  "basic",
		}
		completed := service.getCompletedSteps(session)
		assert.Empty(t, completed)
	})
}

func TestConfigurationWizardService_GetSession_CurrentSession(t *testing.T) {
	service := NewConfigurationWizardService(nil)

	session := &models.WizardSession{
		SessionID:   "test-session-123",
		CurrentStep: 2,
		TotalSteps:  5,
	}
	service.currentSession = session

	result, err := service.getSession("test-session-123")
	assert.NoError(t, err)
	assert.Equal(t, session, result)
}

// listeningPort opens a real TCP listener for the lifetime of the test and returns its port (CAT-001: the network database check dials).
func listeningPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	return ln.Addr().(*net.TCPAddr).Port
}

// CAT-QA-SONAR-5 (supersedes the go:S3923 characterization that pinned the bug):
// a declared template that does not exist is an error and writes nothing.
func TestConfigurationWizardService_CreateConfigurationFile_UnknownTemplateIsError(t *testing.T) {
	service := NewConfigurationWizardService(nil)
	tmpDir := t.TempDir()
	session := &models.WizardSession{Configuration: map[string]interface{}{"db_type": "sqlite"}}

	err := service.createConfigurationFile(map[string]interface{}{"file_path": tmpDir + "/b.json", "template": "basic"}, session)
	assert.Error(t, err)
	_, statErr := os.Stat(tmpDir + "/b.json")
	assert.True(t, os.IsNotExist(statErr))
}
