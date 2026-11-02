package repository

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"catalogizer/database"
	"catalogizer/internal/testpg"
	"catalogizer/models"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

// D5: the configuration repository against a REAL PostgreSQL built by the real
// migrations. Upserts used "INSERT OR REPLACE", which the PostgreSQL dialect
// rewrite turns into a plain INSERT, so a second save failed with a duplicate
// key error (or, for tables without a unique key, silently duplicated rows).
func TestConfigurationRepositoryPostgresUpserts(t *testing.T) {
	dsn, _ := testpg.Start(t)
	sqlDB, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer sqlDB.Close()
	db := database.WrapDB(sqlDB, database.DialectPostgres)
	require.NoError(t, db.RunMigrations(context.Background()))
	repo := NewConfigurationRepository(db)

	now := time.Now().UTC().Truncate(time.Second)
	cfg := &models.SystemConfiguration{Version: "1.0", CreatedAt: now, UpdatedAt: now}
	require.NoError(t, repo.SaveConfiguration(cfg))
	cfg.Version = "2.0"
	require.NoError(t, repo.SaveConfiguration(cfg), "second save must upsert")
	got, err := repo.GetConfiguration()
	require.NoError(t, err)
	require.Equal(t, "2.0", got.Version)

	require.NoError(t, repo.SaveConfigurationHistory(cfg))
	require.NoError(t, repo.CreateConfigurationBackup("b1", cfg))

	for i := 0; i < 2; i++ {
		require.NoError(t, repo.SaveWizardProgress(&models.WizardProgress{
			UserID: 7, CurrentStep: "s", StepData: map[string]interface{}{"i": i},
			AllData: map[string]interface{}{}, UpdatedAt: now}), "wizard progress save %d", i)
	}
	var n int
	require.NoError(t, sqlDB.QueryRow(`SELECT COUNT(*) FROM wizard_progress WHERE user_id=7`).Scan(&n))
	require.Equal(t, 1, n, "wizard progress upsert must not duplicate")

	for i := 0; i < 2; i++ {
		require.NoError(t, repo.MarkWizardCompleted(7), "wizard completion %d", i)
	}
	done, err := repo.IsWizardCompleted(7)
	require.NoError(t, err)
	require.True(t, done)

	for i := 0; i < 2; i++ {
		require.NoError(t, repo.SaveWizardSession(&models.WizardSession{
			SessionID: "sess-1", UserID: 7, CurrentStep: i, TotalSteps: 3,
			StepData: map[string]interface{}{}, Configuration: map[string]interface{}{},
			StartedAt: now, LastActivity: now, ConfigType: "t"}), "wizard session %d", i)
		require.NoError(t, repo.SaveConfigurationProfile(&models.ConfigurationProfile{
			ProfileID: "p1", Name: "n", UserID: 7, Configuration: map[string]interface{}{},
			CreatedAt: now, UpdatedAt: now, IsActive: true, Tags: []string{"a"}}), "profile %d", i)
	}
	sess, err := repo.GetWizardSession("sess-1")
	require.NoError(t, err)
	require.Equal(t, 1, sess.CurrentStep)
	profs, err := repo.GetUserConfigurationProfiles(7)
	require.NoError(t, err)
	require.Len(t, profs, 1)
}
