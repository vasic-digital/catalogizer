package services

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"catalogizer/internal/logging"
	"catalogizer/models"
	"catalogizer/repository"
)

type ConfigurationWizardService struct {
	repo            *repository.ConfigurationRepository
	configPath      string
	backupPath      string
	templatesPath   string
	currentSession  *models.WizardSession
	validationRules map[string]ValidationRule
	configTemplates map[string]ConfigTemplate
}

// models.WizardSession is defined in models package

type WizardStep struct {
	StepID          string                 `json:"step_id"`
	Title           string                 `json:"title"`
	Description     string                 `json:"description"`
	StepType        string                 `json:"step_type"` // input, select, multi-select, file-upload, test
	Fields          []FieldDefinition      `json:"fields"`
	Dependencies    []string               `json:"dependencies"`
	ValidationRules []string               `json:"validation_rules"`
	HelpText        string                 `json:"help_text"`
	IsOptional      bool                   `json:"is_optional"`
	SkipCondition   map[string]interface{} `json:"skip_condition,omitempty"`
}

type FieldDefinition struct {
	FieldID      string                 `json:"field_id"`
	Label        string                 `json:"label"`
	Type         string                 `json:"type"` // text, password, number, boolean, select, file, directory
	Required     bool                   `json:"required"`
	DefaultValue interface{}            `json:"default_value,omitempty"`
	Options      []FieldOption          `json:"options,omitempty"`
	Validation   string                 `json:"validation,omitempty"`
	HelpText     string                 `json:"help_text,omitempty"`
	Placeholder  string                 `json:"placeholder,omitempty"`
	MinValue     *float64               `json:"min_value,omitempty"`
	MaxValue     *float64               `json:"max_value,omitempty"`
	Pattern      string                 `json:"pattern,omitempty"`
	Dependencies map[string]interface{} `json:"dependencies,omitempty"`
}

type FieldOption struct {
	Value       string `json:"value"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
}

type ValidationRule struct {
	RuleID       string                 `json:"rule_id"`
	Type         string                 `json:"type"` // required, format, range, custom
	ErrorMessage string                 `json:"error_message"`
	Parameters   map[string]interface{} `json:"parameters,omitempty"`
}

type ConfigTemplate struct {
	TemplateID    string                 `json:"template_id"`
	Name          string                 `json:"name"`
	Description   string                 `json:"description"`
	Category      string                 `json:"category"`
	Steps         []WizardStep           `json:"steps"`
	DefaultValues map[string]interface{} `json:"default_values"`
	Requirements  []string               `json:"requirements"`
	PostInstall   []PostInstallAction    `json:"post_install_actions"`
}

type PostInstallAction struct {
	ActionType  string                 `json:"action_type"` // service_restart, file_create, command_run, validation
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
	Required    bool                   `json:"required"`
}

// ConfigurationProfile is defined in models package

type SystemInfo struct {
	OS              string            `json:"os"`
	Architecture    string            `json:"architecture"`
	GoVersion       string            `json:"go_version"`
	CPUCores        int               `json:"cpu_cores"`
	MemoryGB        float64           `json:"memory_gb"`
	DiskSpaceGB     float64           `json:"disk_space_gb"`
	NetworkInfo     NetworkInfo       `json:"network_info"`
	InstalledTools  []string          `json:"installed_tools"`
	EnvironmentVars map[string]string `json:"environment_vars"`
	Recommendations []string          `json:"recommendations"`
}

type NetworkInfo struct {
	Hostname    string   `json:"hostname"`
	IPAddresses []string `json:"ip_addresses"`
	DNSServers  []string `json:"dns_servers"`
	HasInternet bool     `json:"has_internet"`
}

type InstallationRequest struct {
	ConfigType     string                 `json:"config_type"`
	QuickInstall   bool                   `json:"quick_install"`
	CustomConfig   map[string]interface{} `json:"custom_config,omitempty"`
	SkipTests      bool                   `json:"skip_tests"`
	BackupExisting bool                   `json:"backup_existing"`
}

type InstallationProgress struct {
	SessionID      string    `json:"session_id"`
	CurrentAction  string    `json:"current_action"`
	Progress       float64   `json:"progress"`
	CompletedSteps []string  `json:"completed_steps"`
	FailedSteps    []string  `json:"failed_steps"`
	EstimatedTime  string    `json:"estimated_time"`
	LastUpdate     time.Time `json:"last_update"`
	IsCompleted    bool      `json:"is_completed"`
	HasErrors      bool      `json:"has_errors"`
	ErrorMessage   string    `json:"error_message,omitempty"`
}

func NewConfigurationWizardService(repo *repository.ConfigurationRepository) *ConfigurationWizardService {
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "./config"
	}

	service := &ConfigurationWizardService{
		repo:            repo,
		configPath:      configPath,
		backupPath:      filepath.Join(configPath, "backups"),
		templatesPath:   filepath.Join(configPath, "templates"),
		validationRules: make(map[string]ValidationRule),
		configTemplates: make(map[string]ConfigTemplate),
	}

	// Initialize default templates and validation rules
	service.initializeDefaultTemplates()
	service.initializeValidationRules()

	// Ensure directories exist
	os.MkdirAll(service.configPath, 0755)
	os.MkdirAll(service.backupPath, 0755)
	os.MkdirAll(service.templatesPath, 0755)

	return service
}

func (s *ConfigurationWizardService) initializeDefaultTemplates() {
	// Basic Installation Template
	s.configTemplates["basic"] = ConfigTemplate{
		TemplateID:  "basic",
		Name:        "Basic Installation",
		Description: "Quick setup for basic Catalogizer functionality",
		Category:    "installation",
		Steps: []WizardStep{
			{
				StepID:      "system_check",
				Title:       "System Requirements Check",
				Description: "Verify system meets minimum requirements",
				StepType:    "test",
				Fields: []FieldDefinition{
					{
						FieldID:      "auto_fix",
						Label:        "Automatically fix issues where possible",
						Type:         "boolean",
						DefaultValue: true,
					},
				},
			},
			{
				StepID:      "database_config",
				Title:       "Database Configuration",
				Description: "Configure database connection",
				StepType:    "input",
				Fields: []FieldDefinition{
					{
						FieldID:      "db_type",
						Label:        "Database Type",
						Type:         "select",
						Required:     true,
						DefaultValue: "sqlite",
						Options: []FieldOption{
							{Value: "sqlite", Label: "SQLite (Recommended)"},
							{Value: "mysql", Label: "MySQL"},
							{Value: "postgresql", Label: "PostgreSQL"},
						},
					},
					{
						FieldID:      "db_path",
						Label:        "Database File Path",
						Type:         "file",
						Required:     true,
						DefaultValue: "./catalogizer.db",
						Dependencies: map[string]interface{}{
							"db_type": "sqlite",
						},
					},
					{
						FieldID:  "db_host",
						Label:    "Database Host",
						Type:     "text",
						Required: true,
						Dependencies: map[string]interface{}{
							"db_type": []string{"mysql", "postgresql"},
						},
					},
					{
						FieldID:  "db_port",
						Label:    "Database Port",
						Type:     "number",
						MinValue: float64Ptr(1),
						MaxValue: float64Ptr(65535),
						Dependencies: map[string]interface{}{
							"db_type": []string{"mysql", "postgresql"},
						},
					},
					{
						FieldID:  "db_name",
						Label:    "Database Name",
						Type:     "text",
						Required: true,
						Dependencies: map[string]interface{}{
							"db_type": []string{"mysql", "postgresql"},
						},
					},
					{
						FieldID:  "db_username",
						Label:    "Database Username",
						Type:     "text",
						Required: true,
						Dependencies: map[string]interface{}{
							"db_type": []string{"mysql", "postgresql"},
						},
					},
					{
						FieldID:  "db_password",
						Label:    "Database Password",
						Type:     "password",
						Required: true,
						Dependencies: map[string]interface{}{
							"db_type": []string{"mysql", "postgresql"},
						},
					},
				},
			},
			{
				StepID:      "media_storage",
				Title:       "Media Storage Configuration",
				Description: "Configure where media files will be stored",
				StepType:    "input",
				Fields: []FieldDefinition{
					{
						FieldID:      "storage_type",
						Label:        "Storage Type",
						Type:         "select",
						Required:     true,
						DefaultValue: "local",
						Options: []FieldOption{
							{Value: "local", Label: "Local Storage"},
							{Value: "s3", Label: "Amazon S3"},
							{Value: "webdav", Label: "WebDAV"},
							{Value: "ftp", Label: "FTP/SFTP"},
						},
					},
					{
						FieldID:      "media_path",
						Label:        "Media Directory Path",
						Type:         "directory",
						Required:     true,
						DefaultValue: "./media",
						Dependencies: map[string]interface{}{
							"storage_type": "local",
						},
					},
					{
						FieldID:      "max_file_size",
						Label:        "Maximum File Size (MB)",
						Type:         "number",
						Required:     true,
						DefaultValue: 100,
						MinValue:     float64Ptr(1),
						MaxValue:     float64Ptr(10000),
					},
				},
			},
			{
				StepID:      "security_config",
				Title:       "Security Configuration",
				Description: "Configure authentication and security settings",
				StepType:    "input",
				Fields: []FieldDefinition{
					{
						FieldID:    "jwt_secret",
						Label:      "JWT Secret Key",
						Type:       "password",
						Required:   true,
						HelpText:   "Secret key for JWT token generation (minimum 32 characters)",
						Validation: "min_length:32",
					},
					{
						FieldID:      "session_timeout",
						Label:        "Session Timeout (hours)",
						Type:         "number",
						Required:     true,
						DefaultValue: 24,
						MinValue:     float64Ptr(1),
						MaxValue:     float64Ptr(720),
					},
					{
						FieldID:      "enable_2fa",
						Label:        "Enable Two-Factor Authentication",
						Type:         "boolean",
						DefaultValue: false,
					},
					{
						FieldID:      "password_min_length",
						Label:        "Minimum Password Length",
						Type:         "number",
						Required:     true,
						DefaultValue: 8,
						MinValue:     float64Ptr(6),
						MaxValue:     float64Ptr(128),
					},
				},
			},
			{
				StepID:      "admin_user",
				Title:       "Administrator Account",
				Description: "Create the initial administrator account",
				StepType:    "input",
				Fields: []FieldDefinition{
					{
						FieldID:     "admin_username",
						Label:       "Administrator Username",
						Type:        "text",
						Required:    true,
						Validation:  "username",
						Placeholder: "admin",
					},
					{
						FieldID:     "admin_email",
						Label:       "Administrator Email",
						Type:        "text",
						Required:    true,
						Validation:  "email",
						Placeholder: "admin@example.com",
					},
					{
						FieldID:    "admin_password",
						Label:      "Administrator Password",
						Type:       "password",
						Required:   true,
						Validation: "password_strength",
					},
					{
						FieldID:    "admin_password_confirm",
						Label:      "Confirm Password",
						Type:       "password",
						Required:   true,
						Validation: "password_match",
					},
				},
			},
			{
				StepID:      "service_config",
				Title:       "Service Configuration",
				Description: "Configure server and service settings",
				StepType:    "input",
				Fields: []FieldDefinition{
					{
						FieldID:      "server_port",
						Label:        "Server Port",
						Type:         "number",
						Required:     true,
						DefaultValue: 8080,
						MinValue:     float64Ptr(1024),
						MaxValue:     float64Ptr(65535),
					},
					{
						FieldID:      "log_level",
						Label:        "Log Level",
						Type:         "select",
						Required:     true,
						DefaultValue: "info",
						Options: []FieldOption{
							{Value: "debug", Label: "Debug"},
							{Value: "info", Label: "Info"},
							{Value: "warn", Label: "Warning"},
							{Value: "error", Label: "Error"},
						},
					},
					{
						FieldID:      "enable_cors",
						Label:        "Enable CORS",
						Type:         "boolean",
						DefaultValue: true,
						HelpText:     "Enable Cross-Origin Resource Sharing for web clients",
					},
					{
						FieldID:      "backup_enabled",
						Label:        "Enable Automatic Backups",
						Type:         "boolean",
						DefaultValue: true,
					},
				},
			},
			{
				StepID:      "final_test",
				Title:       "Final Configuration Test",
				Description: "Test all configurations and start services",
				StepType:    "test",
				Fields: []FieldDefinition{
					{
						FieldID:      "start_services",
						Label:        "Start services after successful test",
						Type:         "boolean",
						DefaultValue: true,
					},
				},
			},
		},
		DefaultValues: map[string]interface{}{
			"db_type":         "sqlite",
			"storage_type":    "local",
			"session_timeout": 24,
			"server_port":     8080,
			"log_level":       "info",
		},
		Requirements: []string{"go", "sqlite3"},
		PostInstall: []PostInstallAction{
			{
				ActionType:  "file_create",
				Description: "Create configuration file",
				Parameters: map[string]interface{}{
					"file_path": "config.json", // relative: resolved against the wizard configPath (CAT-OPB-FU I1)
					"template":  "config_template.json",
				},
				Required: true,
			},
			{
				ActionType:  "service_restart",
				Description: "Restart Catalogizer service",
				Parameters: map[string]interface{}{
					"service_name": "catalogizer",
				},
				Required: false,
			},
		},
	}

	// Enterprise Installation Template
	s.configTemplates["enterprise"] = ConfigTemplate{
		TemplateID:   "enterprise",
		Name:         "Enterprise Installation",
		Description:  "Full enterprise setup with all features enabled",
		Category:     "installation",
		Steps:        s.getEnterpriseSteps(),
		Requirements: []string{"go", "docker", "postgresql", "redis"},
	}

	// Development Template
	s.configTemplates["development"] = ConfigTemplate{
		TemplateID:   "development",
		Name:         "Development Environment",
		Description:  "Development setup with debugging and testing tools",
		Category:     "development",
		Steps:        s.getDevelopmentSteps(),
		Requirements: []string{"go", "git", "make"},
	}
}

func (s *ConfigurationWizardService) getEnterpriseSteps() []WizardStep {
	// Return enterprise-specific configuration steps
	steps := []WizardStep{
		{
			StepID:      "infrastructure",
			Title:       "Infrastructure Setup",
			Description: "Configure enterprise infrastructure components",
			StepType:    "input",
			Fields: []FieldDefinition{
				{
					FieldID:  "deployment_type",
					Label:    "Deployment Type",
					Type:     "select",
					Required: true,
					Options: []FieldOption{
						{Value: "kubernetes", Label: "Kubernetes Cluster"},
						{Value: "docker_swarm", Label: "Docker Swarm"},
						{Value: "standalone", Label: "Standalone Servers"},
					},
				},
				{
					FieldID:  "load_balancer",
					Label:    "Load Balancer Configuration",
					Type:     "select",
					Required: true,
					Options: []FieldOption{
						{Value: "nginx", Label: "Nginx"},
						{Value: "haproxy", Label: "HAProxy"},
						{Value: "aws_alb", Label: "AWS Application Load Balancer"},
					},
				},
			},
		},
		// Add more enterprise-specific steps...
	}
	return steps
}

func (s *ConfigurationWizardService) getDevelopmentSteps() []WizardStep {
	// Return development-specific configuration steps
	steps := []WizardStep{
		{
			StepID:      "dev_environment",
			Title:       "Development Environment",
			Description: "Configure development tools and settings",
			StepType:    "input",
			Fields: []FieldDefinition{
				{
					FieldID:      "debug_mode",
					Label:        "Enable Debug Mode",
					Type:         "boolean",
					DefaultValue: true,
				},
				{
					FieldID:      "hot_reload",
					Label:        "Enable Hot Reload",
					Type:         "boolean",
					DefaultValue: true,
				},
			},
		},
		// Add more development-specific steps...
	}
	return steps
}

func (s *ConfigurationWizardService) initializeValidationRules() {
	s.validationRules["required"] = ValidationRule{
		RuleID:       "required",
		Type:         "required",
		ErrorMessage: "This field is required",
	}

	s.validationRules["email"] = ValidationRule{
		RuleID:       "email",
		Type:         "format",
		ErrorMessage: "Please enter a valid email address",
		Parameters: map[string]interface{}{
			"pattern": `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`,
		},
	}

	s.validationRules["username"] = ValidationRule{
		RuleID:       "username",
		Type:         "format",
		ErrorMessage: "Username can only contain letters, numbers, and underscores (3-30 characters)",
		Parameters: map[string]interface{}{
			"pattern": `^[a-zA-Z0-9_]{3,30}$`,
		},
	}

	s.validationRules["password_strength"] = ValidationRule{
		RuleID:       "password_strength",
		Type:         "custom",
		ErrorMessage: "Password must be at least 8 characters with uppercase, lowercase, number, and special character",
		Parameters: map[string]interface{}{
			"min_length":      8,
			"require_upper":   true,
			"require_lower":   true,
			"require_number":  true,
			"require_special": true,
		},
	}

	s.validationRules["password_match"] = ValidationRule{
		RuleID:       "password_match",
		Type:         "custom",
		ErrorMessage: "Passwords do not match",
		Parameters: map[string]interface{}{
			"match_field": "admin_password",
		},
	}

	s.validationRules["min_length"] = ValidationRule{
		RuleID:       "min_length",
		Type:         "format",
		ErrorMessage: "Must be at least {min} characters long",
	}
}

func (s *ConfigurationWizardService) StartWizard(userID int, configType string, quickInstall bool) (*models.WizardSession, error) {
	template, exists := s.configTemplates[configType]
	if !exists {
		return nil, fmt.Errorf("configuration template '%s' not found", configType)
	}

	sessionID := fmt.Sprintf("wizard-%d-%d", userID, time.Now().Unix())

	session := &models.WizardSession{
		SessionID:     sessionID,
		UserID:        userID,
		CurrentStep:   0,
		TotalSteps:    len(template.Steps),
		StepData:      make(map[string]interface{}),
		Configuration: make(map[string]interface{}),
		StartedAt:     time.Now(),
		LastActivity:  time.Now(),
		IsCompleted:   false,
		ConfigType:    configType,
	}

	// Apply default values
	for key, value := range template.DefaultValues {
		session.Configuration[key] = value
	}

	// Quick install logic
	if quickInstall {
		session.Configuration["quick_install"] = true
		// Skip optional steps and use defaults
	}

	s.currentSession = session

	// Save session to database
	if err := s.repo.SaveWizardSession(session); err != nil {
		return nil, fmt.Errorf("failed to save wizard session: %w", err)
	}

	return session, nil
}

func (s *ConfigurationWizardService) GetCurrentStep(sessionID string) (*WizardStep, error) {
	session, err := s.getSession(sessionID)
	if err != nil {
		return nil, err
	}

	template := s.configTemplates[session.ConfigType]
	if session.CurrentStep >= len(template.Steps) {
		return nil, fmt.Errorf("wizard completed")
	}

	step := template.Steps[session.CurrentStep]

	// Check if step should be skipped
	if s.shouldSkipStep(step, session.Configuration) {
		// Move to next step
		session.CurrentStep++
		s.repo.SaveWizardSession(session)
		return s.GetCurrentStep(sessionID)
	}

	return &step, nil
}

func (s *ConfigurationWizardService) shouldSkipStep(step WizardStep, config map[string]interface{}) bool {
	if step.SkipCondition == nil {
		return false
	}

	for key, expectedValue := range step.SkipCondition {
		if configValue, exists := config[key]; exists {
			if configValue != expectedValue {
				return true
			}
		}
	}

	return false
}

func (s *ConfigurationWizardService) SubmitStepData(sessionID string, stepData map[string]interface{}) error {
	session, err := s.getSession(sessionID)
	if err != nil {
		return err
	}

	template := s.configTemplates[session.ConfigType]
	if session.CurrentStep >= len(template.Steps) {
		return fmt.Errorf("wizard already completed")
	}

	currentStep := template.Steps[session.CurrentStep]

	// Validate step data
	if err := s.validateStepData(currentStep, stepData); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Process step based on type
	switch currentStep.StepType {
	case "test":
		if err := s.processTestStep(currentStep, stepData, session); err != nil {
			return fmt.Errorf("test step failed: %w", err)
		}
	case "input", "select", "multi-select":
		// Store the input data
		for key, value := range stepData {
			session.Configuration[key] = value
		}
	}

	// Store step data
	session.StepData[currentStep.StepID] = stepData
	session.LastActivity = time.Now()

	// Move to next step
	session.CurrentStep++

	// Check if wizard is completed
	if session.CurrentStep >= len(template.Steps) {
		session.IsCompleted = true
		if err := s.finalizeConfiguration(session); err != nil {
			return fmt.Errorf("failed to finalize configuration: %w", err)
		}
	}

	return s.repo.SaveWizardSession(session)
}

func (s *ConfigurationWizardService) validateStepData(step WizardStep, data map[string]interface{}) error {
	for _, field := range step.Fields {
		value, exists := data[field.FieldID]

		// Check required fields
		if field.Required && (!exists || value == nil || value == "") {
			return fmt.Errorf("field '%s' is required", field.FieldID)
		}

		if !exists || value == nil {
			continue
		}

		// Type validation
		if err := s.validateFieldType(field, value); err != nil {
			return fmt.Errorf("field '%s': %w", field.FieldID, err)
		}

		// Custom validation
		if field.Validation != "" {
			if err := s.validateFieldRule(field.Validation, value, data); err != nil {
				return fmt.Errorf("field '%s': %w", field.FieldID, err)
			}
		}

		// Range validation for numbers
		if field.Type == "number" {
			if numValue, ok := value.(float64); ok {
				if field.MinValue != nil && numValue < *field.MinValue {
					return fmt.Errorf("field '%s': value must be at least %v", field.FieldID, *field.MinValue)
				}
				if field.MaxValue != nil && numValue > *field.MaxValue {
					return fmt.Errorf("field '%s': value must be at most %v", field.FieldID, *field.MaxValue)
				}
			}
		}
	}

	return nil
}

func (s *ConfigurationWizardService) validateFieldType(field FieldDefinition, value interface{}) error {
	switch field.Type {
	case "text", "password":
		if _, ok := value.(string); !ok {
			return fmt.Errorf("expected string value")
		}
	case "number":
		switch v := value.(type) {
		case float64, int, int64:
			// Valid number types
		case string:
			if _, err := strconv.ParseFloat(v, 64); err != nil {
				return fmt.Errorf("invalid number format")
			}
		default:
			return fmt.Errorf("expected number value")
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("expected boolean value")
		}
	case "file", "directory":
		if strValue, ok := value.(string); ok {
			if field.Type == "directory" {
				if _, err := os.Stat(strValue); err != nil {
					return fmt.Errorf("directory does not exist: %s", strValue)
				}
			}
		} else {
			return fmt.Errorf("expected string path")
		}
	}

	return nil
}

func (s *ConfigurationWizardService) validateFieldRule(ruleName string, value interface{}, allData map[string]interface{}) error {
	rule, exists := s.validationRules[ruleName]
	if !exists {
		// Handle inline rules like "min_length:32"
		if strings.Contains(ruleName, ":") {
			parts := strings.SplitN(ruleName, ":", 2)
			ruleType, param := parts[0], parts[1]

			switch ruleType {
			case "min_length":
				if strValue, ok := value.(string); ok {
					if minLen, err := strconv.Atoi(param); err == nil {
						if len(strValue) < minLen {
							return fmt.Errorf("must be at least %d characters long", minLen)
						}
					}
				}
			}
		}
		return nil
	}

	switch rule.Type {
	case "format":
		if pattern, ok := rule.Parameters["pattern"].(string); ok {
			if strValue, ok := value.(string); ok {
				matched, err := filepath.Match(pattern, strValue)
				if err != nil || !matched {
					return errors.New(rule.ErrorMessage)
				}
			}
		}
	case "custom":
		return s.validateCustomRule(rule, value, allData)
	}

	return nil
}

func (s *ConfigurationWizardService) validateCustomRule(rule ValidationRule, value interface{}, allData map[string]interface{}) error {
	switch rule.RuleID {
	case "password_strength":
		if strValue, ok := value.(string); ok {
			if len(strValue) < 8 {
				return fmt.Errorf("password must be at least 8 characters")
			}
			// Add more password strength checks
		}
	case "password_match":
		if matchField, ok := rule.Parameters["match_field"].(string); ok {
			if otherValue, exists := allData[matchField]; exists {
				if value != otherValue {
					return fmt.Errorf("passwords do not match")
				}
			}
		}
	}

	return nil
}

func (s *ConfigurationWizardService) processTestStep(step WizardStep, data map[string]interface{}, session *models.WizardSession) error {
	switch step.StepID {
	case "system_check":
		return s.performSystemCheck(data, session)
	case "final_test":
		return s.performFinalTest(data, session)
	default:
		return fmt.Errorf("unknown test step: %s", step.StepID)
	}
}

func (s *ConfigurationWizardService) performSystemCheck(data map[string]interface{}, session *models.WizardSession) error {
	systemInfo := s.collectSystemInfo()

	// Check minimum requirements
	var issues []string

	// Check Go version
	if !strings.HasPrefix(systemInfo.GoVersion, "go1.") {
		issues = append(issues, "Go is not installed or not in PATH")
	}

	// Check available memory (minimum 1GB)
	if systemInfo.MemoryGB < 1.0 {
		issues = append(issues, "Insufficient memory (minimum 1GB required)")
	}

	// Check available disk space (minimum 5GB)
	if systemInfo.DiskSpaceGB < 5.0 {
		issues = append(issues, "Insufficient disk space (minimum 5GB required)")
	}

	// Store system info in session
	session.Configuration["system_info"] = systemInfo

	if len(issues) > 0 {
		autoFix, _ := data["auto_fix"].(bool)
		if autoFix {
			if logging.Logger != nil {
				logging.Warnf("Auto-fixing system issues: %v", issues)
			}
			// Attempt to fix issues automatically
		} else {
			return fmt.Errorf("system check failed: %v", issues)
		}
	}

	return nil
}

func (s *ConfigurationWizardService) performFinalTest(data map[string]interface{}, session *models.WizardSession) error {
	// Test database connection
	if err := s.testDatabaseConnection(session.Configuration); err != nil {
		return fmt.Errorf("database connection test failed: %w", err)
	}

	// Test media storage
	if err := s.testMediaStorage(session.Configuration); err != nil {
		return fmt.Errorf("media storage test failed: %w", err)
	}

	// Test service configuration
	if err := s.testServiceConfiguration(session.Configuration); err != nil {
		return fmt.Errorf("service configuration test failed: %w", err)
	}

	return nil
}

func (s *ConfigurationWizardService) testDatabaseConnection(config map[string]interface{}) error {
	dbType, _ := config["db_type"].(string)

	switch dbType {
	case "sqlite":
		dbPath, _ := config["db_path"].(string)
		if dbPath == "" {
			return fmt.Errorf("database path not specified")
		}
		// Test SQLite connection
		return s.testSQLiteConnection(dbPath)
	case "mysql", "postgresql":
		// Test network database connection
		return s.testNetworkDatabaseConnection(config)
	default:
		return fmt.Errorf("unsupported database type: %s", dbType)
	}
}

func (s *ConfigurationWizardService) testSQLiteConnection(dbPath string) error {
	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create database directory: %w", err)
	}

	// Test write access
	if err := probeWritable(dir); err != nil {
		return fmt.Errorf("no write access to database directory: %w", err)
	}

	return nil
}

func (s *ConfigurationWizardService) testNetworkDatabaseConnection(config map[string]interface{}) error {
	// Validate the required fields, then really dial the server: a configuration whose host:port nothing answers on must not pass the
	// test (CAT-001). Authentication is not attempted here (no driver is linked for the network engines); the check is reachability.
	requiredFields := []string{"db_host", "db_port", "db_name", "db_username", "db_password"}
	for _, field := range requiredFields {
		if _, exists := config[field]; !exists {
			return fmt.Errorf("missing required database field: %s", field)
		}
	}
	host := fmt.Sprint(config["db_host"])
	var port int
	switch v := config["db_port"].(type) {
	case int:
		port = v
	case int64:
		port = int(v)
	case float64:
		port = int(v)
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil {
			return fmt.Errorf("invalid db_port %q", v)
		}
		port = n
	default:
		return fmt.Errorf("invalid db_port type %T", v)
	}
	if port < 1 || port > 65535 {
		return fmt.Errorf("invalid db_port %d", port)
	}
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, strconv.Itoa(port)), 5*time.Second)
	if err != nil {
		return fmt.Errorf("database server %s:%d is not reachable: %w", host, port, err)
	}
	return conn.Close()
}

func (s *ConfigurationWizardService) testMediaStorage(config map[string]interface{}) error {
	storageType, _ := config["storage_type"].(string)

	switch storageType {
	case "local":
		mediaPath, _ := config["media_path"].(string)
		if mediaPath == "" {
			return fmt.Errorf("media path not specified")
		}

		// Ensure directory exists
		if err := os.MkdirAll(mediaPath, 0755); err != nil {
			return fmt.Errorf("failed to create media directory: %w", err)
		}

		// Test write access
		if err := probeWritable(mediaPath); err != nil {
			return fmt.Errorf("no write access to media directory: %w", err)
		}

	case "smb":
		// Validate SMB connection parameters
		host, _ := config["smb_host"].(string)
		share, _ := config["smb_share"].(string)
		if host == "" {
			return fmt.Errorf("SMB host not specified")
		}
		if share == "" {
			return fmt.Errorf("SMB share not specified")
		}

	case "ftp":
		// Validate FTP connection parameters
		host, _ := config["ftp_host"].(string)
		if host == "" {
			return fmt.Errorf("FTP host not specified")
		}
		port, _ := config["ftp_port"].(float64)
		if port == 0 {
			port = 21
		}
		if port < 1 || port > 65535 {
			return fmt.Errorf("invalid FTP port: %v", port)
		}

	case "nfs":
		// Validate NFS connection parameters
		host, _ := config["nfs_host"].(string)
		export, _ := config["nfs_export"].(string)
		if host == "" {
			return fmt.Errorf("NFS host not specified")
		}
		if export == "" {
			return fmt.Errorf("NFS export path not specified")
		}

	case "webdav":
		// Validate WebDAV connection parameters
		urlStr, _ := config["webdav_url"].(string)
		if urlStr == "" {
			return fmt.Errorf("WebDAV URL not specified")
		}
		if !strings.HasPrefix(urlStr, "http://") && !strings.HasPrefix(urlStr, "https://") {
			return fmt.Errorf("WebDAV URL must start with http:// or https://")
		}

	default:
		if storageType != "" {
			if logging.Logger != nil {
				logging.Debugf("Storage type %s: no additional validation available", storageType)
			}
		}
	}

	return nil
}

func (s *ConfigurationWizardService) testServiceConfiguration(config map[string]interface{}) error {
	// Test port availability
	if port, ok := config["server_port"].(float64); ok {
		// Would test if port is available
		if port < 1024 || port > 65535 {
			return fmt.Errorf("invalid port number: %v", port)
		}
	}

	return nil
}

func (s *ConfigurationWizardService) finalizeConfiguration(session *models.WizardSession) error {
	// Generate final configuration file
	configData := map[string]interface{}{
		"version":       "3.0.0",
		"generated_at":  time.Now(),
		"generated_by":  "configuration_wizard",
		"user_id":       session.UserID,
		"configuration": session.Configuration,
	}

	// Write configuration file, remembering what was there so a failing
	// Required post-install action can roll it back (CAT-OPB-FU M1).
	configFile := filepath.Join(s.configPath, "config.json")
	previous, readErr := os.ReadFile(configFile)
	hadPrevious := readErr == nil
	if readErr != nil && !os.IsNotExist(readErr) {
		// An existing but unreadable file cannot be restored: refuse before writing.
		return fmt.Errorf("cannot read existing configuration file %s: %w", configFile, readErr)
	}
	previousMode := os.FileMode(0644)
	if hadPrevious {
		if fi, statErr := os.Stat(configFile); statErr == nil {
			previousMode = fi.Mode().Perm()
		}
	}
	if err := s.writeConfigFile(configFile, configData); err != nil {
		return fmt.Errorf("failed to write configuration file: %w", err)
	}

	// Execute post-install actions
	template := s.configTemplates[session.ConfigType]
	for _, action := range template.PostInstall {
		if err := s.executePostInstallAction(action, session); err != nil {
			if action.Required {
				if rbErr := s.rollbackConfig(configFile, previous, previousMode, hadPrevious); rbErr != nil {
					return fmt.Errorf("failed to execute required post-install action: %w (rollback also failed: %v)", err, rbErr)
				}
				return fmt.Errorf("failed to execute required post-install action: %w", err)
			}
			if logging.Logger != nil {
				logging.Warnf("Optional post-install action failed: %v", err)
			}
		}
	}

	return nil
}

// rollbackConfig restores the configuration file written by finalizeConfiguration:
// the previous content and mode when there was one, otherwise removes the new file.
func (s *ConfigurationWizardService) rollbackConfig(file string, previous []byte, mode os.FileMode, had bool) error {
	if had {
		if err := os.WriteFile(file, previous, mode); err != nil {
			return fmt.Errorf("restore %s: %w", file, err)
		}
		if err := os.Chmod(file, mode); err != nil {
			return fmt.Errorf("restore mode of %s: %w", file, err)
		}
		return nil
	}
	if err := os.Remove(file); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove %s: %w", file, err)
	}
	return nil
}

func (s *ConfigurationWizardService) writeConfigFile(filename string, data map[string]interface{}) error {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filename, jsonData, 0644)
}

func (s *ConfigurationWizardService) executePostInstallAction(action PostInstallAction, session *models.WizardSession) error {
	switch action.ActionType {
	case "file_create":
		return s.createConfigurationFile(action.Parameters, session)
	case "service_restart":
		return s.restartService(action.Parameters)
	case "command_run":
		return s.runCommand(action.Parameters)
	default:
		return fmt.Errorf("unknown post-install action: %s", action.ActionType)
	}
}

func (s *ConfigurationWizardService) createConfigurationFile(params map[string]interface{}, session *models.WizardSession) error {
	filePath, _ := params["file_path"].(string)
	template, _ := params["template"].(string)

	if filePath == "" {
		return fmt.Errorf("file_path parameter required")
	}

	// A relative path is resolved against the wizard configPath, never the
	// process CWD (CAT-OPB-FU I1), and must stay inside it both lexically ("..")
	// and after symlink resolution (OBF-FU). Absolute paths are used as given.
	if !filepath.IsAbs(filePath) {
		joined := filepath.Join(s.configPath, filePath)
		rel, relErr := filepath.Rel(filepath.Clean(s.configPath), joined)
		if relErr != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return fmt.Errorf("file_path %q resolves outside the configuration directory", filePath)
		}
		if err := s.ensureResolvedInsideConfigDir(joined); err != nil {
			return fmt.Errorf("file_path %q: %w", filePath, err)
		}
		filePath = joined
	}

	// Create directory if needed
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data := session.Configuration
	if template != "" {
		base, err := s.loadConfigTemplate(template)
		if err != nil {
			return err
		}
		data = mergeConfigMaps(base, session.Configuration)
	}

	// MERGE, NEVER OVERWRITE (OBF-5, operator decision 2026-11-13): the target
	// may be the config.json that finalizeConfiguration just wrote (version,
	// generated_at, generated_by, user_id, configuration). Its keys win; the
	// template content merges under them (deep merge, existing value wins on a
	// clash, see mergeConfigMaps). An existing file that is not a JSON object,
	// or whose "config" value is not an object, is never clobbered.
	doc := map[string]interface{}{"config": data}
	existing, readErr := os.ReadFile(filePath)
	switch {
	case readErr == nil:
		var prev map[string]interface{}
		if err := json.Unmarshal(existing, &prev); err != nil || prev == nil {
			return fmt.Errorf("existing file %s is not a JSON object; refusing to overwrite it", filePath)
		}
		if v, has := prev["config"]; has {
			if _, isMap := v.(map[string]interface{}); !isMap {
				return fmt.Errorf("existing file %s has a non-object \"config\" value; refusing to overwrite it", filePath)
			}
		}
		doc = mergeConfigMaps(doc, prev)
	case !os.IsNotExist(readErr):
		return fmt.Errorf("cannot read existing file %s: %w", filePath, readErr)
	}

	return s.writeConfigFile(filePath, doc)
}

// ensureResolvedInsideConfigDir refuses a target whose symlink-resolved location
// is outside the symlink-resolved configPath. The deepest existing ancestor of
// the target (the target itself when it exists) is resolved; the not-yet-existing
// tail cannot contain a link.
func (s *ConfigurationWizardService) ensureResolvedInsideConfigDir(target string) error {
	base, err := filepath.EvalSymlinks(s.configPath)
	if err != nil {
		return fmt.Errorf("cannot resolve configuration directory %s: %w", s.configPath, err)
	}
	cur, tail := target, ""
	for {
		resolved, rerr := filepath.EvalSymlinks(cur)
		if rerr == nil {
			full := filepath.Join(resolved, tail)
			rel, relErr := filepath.Rel(base, full)
			if relErr != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				return fmt.Errorf("resolves outside the configuration directory through a symbolic link")
			}
			return nil
		}
		if !os.IsNotExist(rerr) {
			return fmt.Errorf("cannot resolve %s: %w", cur, rerr)
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return fmt.Errorf("cannot resolve %s: no existing ancestor", target)
		}
		tail = filepath.Join(filepath.Base(cur), tail)
		cur = parent
	}
}

// probeWritable checks write access to dir with a uniquely named temporary file
// that is always removed; it never touches a file the operator already has
// (OBF-FU F7).
func probeWritable(dir string) error {
	f, err := os.CreateTemp(dir, ".catalogizer-probe-*")
	if err != nil {
		return err
	}
	name := f.Name()
	werr := func() error {
		if _, e := f.Write([]byte("test")); e != nil {
			return e
		}
		return nil
	}()
	cerr := f.Close()
	rerr := os.Remove(name)
	if werr != nil {
		return werr
	}
	if cerr != nil {
		return cerr
	}
	return rerr
}

//go:embed wizard_templates/*.json
var builtinWizardTemplates embed.FS

// loadConfigTemplate loads a declared post-install template by plain file name:
// first from the operator's templates directory (templatesPath), then from the
// built-in set shipped with the binary. A missing or invalid template is an
// error, never silently ignored (CAT-QA-SONAR-5).
func (s *ConfigurationWizardService) loadConfigTemplate(name string) (map[string]interface{}, error) {
	if name != filepath.Base(name) || name == "." || name == ".." || strings.ContainsAny(name, `/\`) {
		return nil, fmt.Errorf("config template %q: must be a plain file name inside the templates directory", name)
	}
	opPath := filepath.Join(s.templatesPath, name)
	if err := s.ensureInsideTemplatesDir(opPath, name); err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(opPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("config template %q: cannot read from %s: %w", name, s.templatesPath, err)
		}
		raw, err = builtinWizardTemplates.ReadFile("wizard_templates/" + name)
		if err != nil {
			return nil, fmt.Errorf("config template %q: not found in %s and not a built-in template", name, s.templatesPath)
		}
	}
	var doc map[string]interface{}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("config template %q: invalid JSON object: %w", name, err)
	}
	if doc == nil { // JSON null decodes to a nil map without error (CAT-OPB-FU N1)
		return nil, fmt.Errorf("config template %q: invalid JSON object: got null", name)
	}
	return doc, nil
}

// ensureInsideTemplatesDir rejects an operator template whose symlink-resolved
// location is outside the (resolved) templates directory (CAT-OPB-FU M2). A
// path that does not exist is left to the caller (built-in fallback).
func (s *ConfigurationWizardService) ensureInsideTemplatesDir(path, name string) error {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("config template %q: cannot resolve %s: %w", name, path, err)
	}
	base, err := filepath.EvalSymlinks(s.templatesPath)
	if err != nil {
		return fmt.Errorf("config template %q: cannot resolve templates directory %s: %w", name, s.templatesPath, err)
	}
	rel, err := filepath.Rel(base, resolved)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("config template %q: resolves outside the templates directory", name)
	}
	return nil
}

// mergeConfigMaps returns base overlaid with over (recursively for nested objects); over wins.
func mergeConfigMaps(base, over map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(base)+len(over))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range over {
		if ov, ok := v.(map[string]interface{}); ok {
			if bv, ok := out[k].(map[string]interface{}); ok {
				out[k] = mergeConfigMaps(bv, ov)
				continue
			}
		}
		out[k] = v
	}
	return out
}

func (s *ConfigurationWizardService) restartService(params map[string]interface{}) error {
	serviceName, _ := params["service_name"].(string)
	if serviceName == "" {
		return fmt.Errorf("service_name parameter required")
	}

	// This would contain actual service restart logic
	if logging.Logger != nil {
		logging.Debugf("Would restart service: %s", serviceName)
	}
	return nil
}

func (s *ConfigurationWizardService) runCommand(params map[string]interface{}) error {
	command, _ := params["command"].(string)
	if command == "" {
		return fmt.Errorf("command parameter required")
	}

	// This would contain actual command execution logic
	if logging.Logger != nil {
		logging.Debugf("Would run command: %s", command)
	}
	return nil
}

func (s *ConfigurationWizardService) collectSystemInfo() SystemInfo {
	info := SystemInfo{
		OS:           runtime.GOOS,
		Architecture: runtime.GOARCH,
		GoVersion:    runtime.Version(),
		CPUCores:     runtime.NumCPU(),
	}

	// Get memory info
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	info.MemoryGB = float64(m.Sys) / (1024 * 1024 * 1024)

	// Get hostname
	if hostname, err := os.Hostname(); err == nil {
		info.NetworkInfo.Hostname = hostname
	}

	// Detect installed tools
	info.InstalledTools = s.detectInstalledTools()

	// Get environment variables
	info.EnvironmentVars = make(map[string]string)
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			// Only include non-sensitive environment variables
			key := parts[0]
			if !strings.Contains(strings.ToLower(key), "password") &&
				!strings.Contains(strings.ToLower(key), "secret") &&
				!strings.Contains(strings.ToLower(key), "token") {
				info.EnvironmentVars[key] = parts[1]
			}
		}
	}

	// Generate recommendations
	info.Recommendations = s.generateSystemRecommendations(info)

	return info
}

func (s *ConfigurationWizardService) detectInstalledTools() []string {
	var tools []string

	toolsToCheck := []string{"go", "git", "docker", "make", "npm", "node", "python", "sqlite3"}

	for _, tool := range toolsToCheck {
		if _, err := exec.LookPath(tool); err == nil {
			tools = append(tools, tool)
		}
	}

	return tools
}

func (s *ConfigurationWizardService) generateSystemRecommendations(info SystemInfo) []string {
	var recommendations []string

	if info.MemoryGB < 2.0 {
		recommendations = append(recommendations, "Consider upgrading system memory to at least 2GB for better performance")
	}

	if info.CPUCores < 2 {
		recommendations = append(recommendations, "Consider using a multi-core processor for better concurrency")
	}

	if info.OS == "windows" {
		recommendations = append(recommendations, "Consider using WSL2 for better compatibility with Unix-based tools")
	}

	if len(info.InstalledTools) < 3 {
		recommendations = append(recommendations, "Install additional development tools like Git and Docker for full functionality")
	}

	return recommendations
}

func (s *ConfigurationWizardService) getSession(sessionID string) (*models.WizardSession, error) {
	if s.currentSession != nil && s.currentSession.SessionID == sessionID {
		return s.currentSession, nil
	}

	// Load from database
	session, err := s.repo.GetWizardSession(sessionID)
	if err != nil {
		return nil, fmt.Errorf("session not found: %w", err)
	}

	s.currentSession = session
	return session, nil
}

func (s *ConfigurationWizardService) GetAvailableTemplates() []ConfigTemplate {
	templates := []ConfigTemplate{}
	for _, template := range s.configTemplates {
		templates = append(templates, template)
	}
	return templates
}

func (s *ConfigurationWizardService) GetWizardProgress(sessionID string) (*InstallationProgress, error) {
	session, err := s.getSession(sessionID)
	if err != nil {
		return nil, err
	}

	progress := float64(session.CurrentStep) / float64(session.TotalSteps) * 100

	return &InstallationProgress{
		SessionID:      sessionID,
		CurrentAction:  fmt.Sprintf("Step %d of %d", session.CurrentStep+1, session.TotalSteps),
		Progress:       progress,
		CompletedSteps: s.getCompletedSteps(session),
		LastUpdate:     session.LastActivity,
		IsCompleted:    session.IsCompleted,
	}, nil
}

func (s *ConfigurationWizardService) getCompletedSteps(session *models.WizardSession) []string {
	var completed []string
	template := s.configTemplates[session.ConfigType]

	for i := 0; i < session.CurrentStep && i < len(template.Steps); i++ {
		completed = append(completed, template.Steps[i].Title)
	}

	return completed
}

func (s *ConfigurationWizardService) SaveConfigurationProfile(userID int, profile *models.ConfigurationProfile) error {
	profile.UserID = userID
	profile.CreatedAt = time.Now()
	profile.UpdatedAt = time.Now()

	return s.repo.SaveConfigurationProfile(profile)
}

func (s *ConfigurationWizardService) LoadConfigurationProfile(userID int, profileID string) (*models.ConfigurationProfile, error) {
	return s.repo.GetConfigurationProfile(profileID)
}

func (s *ConfigurationWizardService) GetUserConfigurationProfiles(userID int) ([]*models.ConfigurationProfile, error) {
	return s.repo.GetUserConfigurationProfiles(userID)
}

func float64Ptr(f float64) *float64 {
	return &f
}
