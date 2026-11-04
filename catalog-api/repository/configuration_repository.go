package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"catalogizer/database"
	"catalogizer/models"
)

type ConfigurationRepository struct {
	db *database.DB
}

func NewConfigurationRepository(db *database.DB) *ConfigurationRepository {
	return &ConfigurationRepository{db: db}
}

func (r *ConfigurationRepository) SaveConfiguration(config *models.SystemConfiguration) error {
	configJSON, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal configuration: %w", err)
	}

	query := r.db.Dialect().UpsertSQL("system_configuration",
		[]string{"id", "version", "configuration", "created_at", "updated_at"}, []string{"id"})

	_, err = r.db.Exec(query, 1, config.Version, string(configJSON), config.CreatedAt, config.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	return nil
}

func (r *ConfigurationRepository) GetConfiguration() (*models.SystemConfiguration, error) {
	query := `
		SELECT version, configuration, created_at, updated_at
		FROM system_configuration
		WHERE id = 1`

	var version, configJSON string
	var createdAt, updatedAt time.Time

	err := r.db.QueryRow(query).Scan(&version, &configJSON, &createdAt, &updatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to get configuration: %w", err)
	}

	var config models.SystemConfiguration
	if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal configuration: %w", err)
	}

	config.Version = version
	config.CreatedAt = createdAt
	config.UpdatedAt = updatedAt

	return &config, nil
}

func (r *ConfigurationRepository) SaveWizardProgress(progress *models.WizardProgress) error {
	stepDataJSON, err := json.Marshal(progress.StepData)
	if err != nil {
		return fmt.Errorf("failed to marshal step data: %w", err)
	}

	allDataJSON, err := json.Marshal(progress.AllData)
	if err != nil {
		return fmt.Errorf("failed to marshal all data: %w", err)
	}

	query := r.db.Dialect().UpsertSQL("wizard_progress",
		[]string{"user_id", "current_step", "step_data", "all_data", "updated_at"}, []string{"user_id"})

	_, err = r.db.Exec(query,
		progress.UserID, progress.CurrentStep, string(stepDataJSON),
		string(allDataJSON), progress.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to save wizard progress: %w", err)
	}

	return nil
}

func (r *ConfigurationRepository) GetWizardProgress(userID int) (*models.WizardProgress, error) {
	query := `
		SELECT user_id, current_step, step_data, all_data, updated_at
		FROM wizard_progress
		WHERE user_id = ?`

	var progress models.WizardProgress
	var stepDataJSON, allDataJSON string

	err := r.db.QueryRow(query, userID).Scan(
		&progress.UserID, &progress.CurrentStep, &stepDataJSON,
		&allDataJSON, &progress.UpdatedAt)

	if err == sql.ErrNoRows {
		return &models.WizardProgress{
			UserID:   userID,
			StepData: map[string]interface{}{},
			AllData:  map[string]interface{}{},
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get wizard progress: %w", err)
	}

	if err := json.Unmarshal([]byte(stepDataJSON), &progress.StepData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal step data: %w", err)
	}

	if err := json.Unmarshal([]byte(allDataJSON), &progress.AllData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal all data: %w", err)
	}

	return &progress, nil
}

func (r *ConfigurationRepository) DeleteWizardProgress(userID int) error {
	query := "DELETE FROM wizard_progress WHERE user_id = ?"
	_, err := r.db.Exec(query, userID)
	if err != nil {
		return fmt.Errorf("failed to delete wizard progress: %w", err)
	}
	return nil
}

func (r *ConfigurationRepository) MarkWizardCompleted(userID int) error {
	query := r.db.Dialect().UpsertSQL("wizard_completion",
		[]string{"user_id", "completed_at"}, []string{"user_id"})

	_, err := r.db.Exec(query, userID, time.Now())
	if err != nil {
		return fmt.Errorf("failed to mark wizard as completed: %w", err)
	}

	return nil
}

func (r *ConfigurationRepository) IsWizardCompleted(userID int) (bool, error) {
	query := `
		SELECT COUNT(*)
		FROM wizard_completion
		WHERE user_id = ?`

	var count int
	err := r.db.QueryRow(query, userID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check wizard completion: %w", err)
	}

	return count > 0, nil
}

func (r *ConfigurationRepository) GetConfigurationHistory(limit int) ([]*models.ConfigurationHistory, error) {
	query := `
		SELECT id, version, created_at, updated_at
		FROM system_configuration_history
		ORDER BY created_at DESC
		LIMIT ?`

	rows, err := r.db.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get configuration history: %w", err)
	}
	defer rows.Close()

	history := []*models.ConfigurationHistory{}
	for rows.Next() {
		var entry models.ConfigurationHistory
		err := rows.Scan(&entry.ID, &entry.Version, &entry.CreatedAt, &entry.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan configuration history: %w", err)
		}
		history = append(history, &entry)
	}

	return history, nil
}

func (r *ConfigurationRepository) SaveConfigurationHistory(config *models.SystemConfiguration) error {
	configJSON, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal configuration: %w", err)
	}

	query := `
		INSERT INTO system_configuration_history (
			version, configuration, created_at, updated_at
		) VALUES (?, ?, ?, ?)`

	_, err = r.db.Exec(query, config.Version, string(configJSON), config.CreatedAt, config.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to save configuration history: %w", err)
	}

	return nil
}

func (r *ConfigurationRepository) CreateConfigurationBackup(name string, config *models.SystemConfiguration) error {
	configJSON, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal configuration: %w", err)
	}

	query := `
		INSERT INTO configuration_backups (
			name, version, configuration, created_at
		) VALUES (?, ?, ?, ?)`

	_, err = r.db.Exec(query, name, config.Version, string(configJSON), time.Now())
	if err != nil {
		return fmt.Errorf("failed to create configuration backup: %w", err)
	}

	return nil
}

func (r *ConfigurationRepository) GetConfigurationBackups() ([]*models.ConfigurationBackup, error) {
	query := `
		SELECT id, name, version, created_at
		FROM configuration_backups
		ORDER BY created_at DESC`

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get configuration backups: %w", err)
	}
	defer rows.Close()

	backups := []*models.ConfigurationBackup{}
	for rows.Next() {
		var backup models.ConfigurationBackup
		err := rows.Scan(&backup.ID, &backup.Name, &backup.Version, &backup.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan configuration backup: %w", err)
		}
		backups = append(backups, &backup)
	}

	return backups, nil
}

func (r *ConfigurationRepository) RestoreConfigurationBackup(backupID int) (*models.SystemConfiguration, error) {
	query := `
		SELECT configuration
		FROM configuration_backups
		WHERE id = ?`

	var configJSON string
	err := r.db.QueryRow(query, backupID).Scan(&configJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to get configuration backup: %w", err)
	}

	var config models.SystemConfiguration
	if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal configuration: %w", err)
	}

	// Update timestamps
	config.UpdatedAt = time.Now()

	// Save as current configuration
	if err := r.SaveConfiguration(&config); err != nil {
		return nil, fmt.Errorf("failed to restore configuration: %w", err)
	}

	return &config, nil
}

func (r *ConfigurationRepository) DeleteConfigurationBackup(backupID int) error {
	query := "DELETE FROM configuration_backups WHERE id = ?"
	_, err := r.db.Exec(query, backupID)
	if err != nil {
		return fmt.Errorf("failed to delete configuration backup: %w", err)
	}
	return nil
}

func (r *ConfigurationRepository) GetConfigurationTemplates() ([]*models.ConfigurationTemplate, error) {
	query := `
		SELECT id, name, description, category, configuration, created_at
		FROM configuration_templates
		ORDER BY category, name`

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get configuration templates: %w", err)
	}
	defer rows.Close()

	templates := []*models.ConfigurationTemplate{}
	for rows.Next() {
		var template models.ConfigurationTemplate
		var configJSON string

		err := rows.Scan(
			&template.ID, &template.Name, &template.Description,
			&template.Category, &configJSON, &template.CreatedAt)

		if err != nil {
			return nil, fmt.Errorf("failed to scan configuration template: %w", err)
		}

		if err := json.Unmarshal([]byte(configJSON), &template.Configuration); err != nil {
			return nil, fmt.Errorf("failed to unmarshal template configuration: %w", err)
		}

		templates = append(templates, &template)
	}

	return templates, nil
}

func (r *ConfigurationRepository) CreateConfigurationTemplate(template *models.ConfigurationTemplate) error {
	configJSON, err := json.Marshal(template.Configuration)
	if err != nil {
		return fmt.Errorf("failed to marshal configuration: %w", err)
	}

	query := `
		INSERT INTO configuration_templates (
			name, description, category, configuration, created_at
		) VALUES (?, ?, ?, ?, ?)`

	id, err := r.db.InsertReturningID(context.Background(), query,
		template.Name, template.Description, template.Category,
		string(configJSON), template.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to create configuration template: %w", err)
	}

	template.ID = int(id)
	return nil
}

func (r *ConfigurationRepository) DeleteConfigurationTemplate(templateID int) error {
	query := "DELETE FROM configuration_templates WHERE id = ?"
	_, err := r.db.Exec(query, templateID)
	if err != nil {
		return fmt.Errorf("failed to delete configuration template: %w", err)
	}
	return nil
}

func (r *ConfigurationRepository) ApplyConfigurationTemplate(templateID int) (*models.SystemConfiguration, error) {
	query := `
		SELECT configuration
		FROM configuration_templates
		WHERE id = ?`

	var configJSON string
	err := r.db.QueryRow(query, templateID).Scan(&configJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to get configuration template: %w", err)
	}

	var config models.SystemConfiguration
	if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal configuration: %w", err)
	}

	// Update timestamps
	config.CreatedAt = time.Now()
	config.UpdatedAt = time.Now()

	// Save as current configuration
	if err := r.SaveConfiguration(&config); err != nil {
		return nil, fmt.Errorf("failed to apply template: %w", err)
	}

	return &config, nil
}

func (r *ConfigurationRepository) CleanupOldHistory(olderThan time.Time) error {
	query := "DELETE FROM system_configuration_history WHERE created_at < ?"
	_, err := r.db.Exec(query, olderThan)
	if err != nil {
		return fmt.Errorf("failed to cleanup old configuration history: %w", err)
	}
	return nil
}

func (r *ConfigurationRepository) GetConfigurationStatistics() (*models.ConfigurationStatistics, error) {
	stats := &models.ConfigurationStatistics{}

	// Total configurations in history
	err := r.db.QueryRow("SELECT COUNT(*) FROM system_configuration_history").Scan(&stats.TotalConfigurations)
	if err != nil {
		return nil, fmt.Errorf("failed to get total configurations: %w", err)
	}

	// Total backups
	err = r.db.QueryRow("SELECT COUNT(*) FROM configuration_backups").Scan(&stats.TotalBackups)
	if err != nil {
		return nil, fmt.Errorf("failed to get total backups: %w", err)
	}

	// Total templates
	err = r.db.QueryRow("SELECT COUNT(*) FROM configuration_templates").Scan(&stats.TotalTemplates)
	if err != nil {
		return nil, fmt.Errorf("failed to get total templates: %w", err)
	}

	// Wizard completions
	err = r.db.QueryRow("SELECT COUNT(*) FROM wizard_completion").Scan(&stats.WizardCompletions)
	if err != nil {
		return nil, fmt.Errorf("failed to get wizard completions: %w", err)
	}

	// Last configuration update
	err = r.db.QueryRow("SELECT updated_at FROM system_configuration WHERE id = 1").Scan(&stats.LastUpdate)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to get last update: %w", err)
	}

	return stats, nil
}

// SaveWizardSession saves a wizard session
func (r *ConfigurationRepository) SaveWizardSession(session *models.WizardSession) error {
	query := r.db.Dialect().UpsertSQL("wizard_sessions", []string{
		"session_id", "user_id", "current_step", "total_steps", "step_data",
		"configuration", "started_at", "last_activity", "is_completed", "config_type",
	}, []string{"session_id"})

	stepDataJSON, err := json.Marshal(session.StepData)
	if err != nil {
		return fmt.Errorf("failed to marshal step data: %w", err)
	}

	configJSON, err := json.Marshal(session.Configuration)
	if err != nil {
		return fmt.Errorf("failed to marshal configuration: %w", err)
	}

	_, err = r.db.Exec(query,
		session.SessionID, session.UserID, session.CurrentStep, session.TotalSteps,
		string(stepDataJSON), string(configJSON), session.StartedAt,
		session.LastActivity, session.IsCompleted, session.ConfigType)

	if err != nil {
		return fmt.Errorf("failed to save wizard session: %w", err)
	}

	return nil
}

// GetWizardSession retrieves a wizard session by session ID
func (r *ConfigurationRepository) GetWizardSession(sessionID string) (*models.WizardSession, error) {
	query := `
		SELECT session_id, user_id, current_step, total_steps, step_data,
			   configuration, started_at, last_activity, is_completed, config_type
		FROM wizard_sessions WHERE session_id = ?`

	session := &models.WizardSession{}
	var stepDataJSON, configJSON string

	err := r.db.QueryRow(query, sessionID).Scan(
		&session.SessionID, &session.UserID, &session.CurrentStep, &session.TotalSteps,
		&stepDataJSON, &configJSON, &session.StartedAt, &session.LastActivity,
		&session.IsCompleted, &session.ConfigType)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("wizard session not found")
		}
		return nil, fmt.Errorf("failed to get wizard session: %w", err)
	}

	if err := json.Unmarshal([]byte(stepDataJSON), &session.StepData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal step data: %w", err)
	}

	if err := json.Unmarshal([]byte(configJSON), &session.Configuration); err != nil {
		return nil, fmt.Errorf("failed to unmarshal configuration: %w", err)
	}

	return session, nil
}

// SaveConfigurationProfile saves a configuration profile
func (r *ConfigurationRepository) SaveConfigurationProfile(profile *models.ConfigurationProfile) error {
	query := r.db.Dialect().UpsertSQL("configuration_profiles", []string{
		"profile_id", "name", "description", "user_id", "configuration",
		"created_at", "updated_at", "is_active", "tags",
	}, []string{"profile_id"})

	configJSON, err := json.Marshal(profile.Configuration)
	if err != nil {
		return fmt.Errorf("failed to marshal configuration: %w", err)
	}

	tagsJSON, err := json.Marshal(profile.Tags)
	if err != nil {
		return fmt.Errorf("failed to marshal tags: %w", err)
	}

	_, err = r.db.Exec(query,
		profile.ProfileID, profile.Name, profile.Description, profile.UserID,
		string(configJSON), profile.CreatedAt, profile.UpdatedAt,
		profile.IsActive, string(tagsJSON))

	if err != nil {
		return fmt.Errorf("failed to save configuration profile: %w", err)
	}

	return nil
}

// GetConfigurationProfile retrieves a configuration profile by profile ID
func (r *ConfigurationRepository) GetConfigurationProfile(profileID string) (*models.ConfigurationProfile, error) {
	query := `
		SELECT profile_id, name, description, user_id, configuration,
			   created_at, updated_at, is_active, tags
		FROM configuration_profiles WHERE profile_id = ?`

	profile := &models.ConfigurationProfile{}
	var configJSON, tagsJSON string

	err := r.db.QueryRow(query, profileID).Scan(
		&profile.ProfileID, &profile.Name, &profile.Description, &profile.UserID,
		&configJSON, &profile.CreatedAt, &profile.UpdatedAt, &profile.IsActive, &tagsJSON)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("configuration profile not found")
		}
		return nil, fmt.Errorf("failed to get configuration profile: %w", err)
	}

	if err := json.Unmarshal([]byte(configJSON), &profile.Configuration); err != nil {
		return nil, fmt.Errorf("failed to unmarshal configuration: %w", err)
	}

	if err := json.Unmarshal([]byte(tagsJSON), &profile.Tags); err != nil {
		return nil, fmt.Errorf("failed to unmarshal tags: %w", err)
	}

	return profile, nil
}

// GetUserConfigurationProfiles retrieves all configuration profiles for a user
func (r *ConfigurationRepository) GetUserConfigurationProfiles(userID int) ([]*models.ConfigurationProfile, error) {
	query := `
		SELECT profile_id, name, description, user_id, configuration,
			   created_at, updated_at, is_active, tags
		FROM configuration_profiles WHERE user_id = ? ORDER BY created_at DESC`

	rows, err := r.db.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user configuration profiles: %w", err)
	}
	defer rows.Close()

	profiles := []*models.ConfigurationProfile{}
	for rows.Next() {
		profile := &models.ConfigurationProfile{}
		var configJSON, tagsJSON string

		err := rows.Scan(
			&profile.ProfileID, &profile.Name, &profile.Description, &profile.UserID,
			&configJSON, &profile.CreatedAt, &profile.UpdatedAt, &profile.IsActive, &tagsJSON)

		if err != nil {
			return nil, fmt.Errorf("failed to scan configuration profile: %w", err)
		}

		if err := json.Unmarshal([]byte(configJSON), &profile.Configuration); err != nil {
			return nil, fmt.Errorf("failed to unmarshal configuration: %w", err)
		}

		if err := json.Unmarshal([]byte(tagsJSON), &profile.Tags); err != nil {
			return nil, fmt.Errorf("failed to unmarshal tags: %w", err)
		}

		profiles = append(profiles, profile)
	}

	return profiles, nil
}
