package database

import (
	"context"
	"database/sql"
	"os/exec"
	"strings"
	"testing"
	"time"

	"catalogizer/internal/testpg"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

// D5 integration test: boots the Go migrations against a REAL PostgreSQL 15
// container (rootless podman, random host port, throwaway) and asserts
//  1. RunMigrations succeeds, twice (second run is a no-op),
//  2. the server's own log holds ZERO "ERROR:" lines (statement errors that
//     the migration code swallowed, e.g. "column already exists", still show
//     up here), and
//  3. every table/column the handlers and repositories query exists.
//
// Honest SKIP (no stand-in) when podman or the image is unavailable.

func TestRunMigrationsPostgresRealContainerZeroErrors(t *testing.T) {
	dsn, container := testpg.Start(t)
	sqlDB, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer sqlDB.Close()
	db := WrapDB(sqlDB, DialectPostgres)

	ctx := context.Background()
	require.NoError(t, db.RunMigrations(ctx), "first run")
	require.NoError(t, db.RunMigrations(ctx), "second run (idempotent)")

	logs, err := exec.Command("podman", "logs", container).CombinedOutput()
	require.NoError(t, err)
	var errLines []string
	for _, l := range strings.Split(string(logs), "\n") {
		if strings.Contains(l, " ERROR:") {
			errLines = append(errLines, l)
		}
	}
	require.Empty(t, errLines, "postgres server logged %d errors during migrations, first: %v", len(errLines), firstN(errLines, 5))

	for table, cols := range expectedPostgresSchema {
		for _, c := range cols {
			var n int
			require.NoError(t, sqlDB.QueryRow(
				`SELECT COUNT(*) FROM information_schema.columns WHERE table_schema='public' AND table_name=$1 AND column_name=$2`,
				table, c).Scan(&n))
			require.Equal(t, 1, n, "missing %s.%s", table, c)
		}
	}
}

// The v22 unique indexes must back the UpsertSQL conflict targets, on the real server.
func TestUpsertSQLPostgresConflictTargetsExist(t *testing.T) {
	dsn, _ := testpg.Start(t)
	sqlDB, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer sqlDB.Close()
	db := WrapDB(sqlDB, DialectPostgres)
	require.NoError(t, db.RunMigrations(context.Background()))

	q := db.Dialect().UpsertSQL("thumbnail_cache",
		[]string{"video_id", "position", "url", "width", "height", "file_size", "created_at"},
		[]string{"video_id", "position", "width", "height"})
	for i, u := range []string{"u1", "u2"} {
		_, err := db.Exec(q, 1, 2, u, 3, 4, 5, time.Now().UTC())
		require.NoError(t, err, "upsert %d", i)
	}
	var url string
	var n int
	require.NoError(t, sqlDB.QueryRow(`SELECT COUNT(*), MAX(url) FROM thumbnail_cache`).Scan(&n, &url))
	require.Equal(t, 1, n)
	require.Equal(t, "u2", url)
}

func firstN(s []string, n int) []string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

// expectedPostgresSchema: tables/columns that handlers and repositories
// query. Extended by the code-derived completeness check.
var expectedPostgresSchema = map[string][]string{
	"system_configuration":         {"id", "version", "configuration", "created_at", "updated_at"},
	"system_configuration_history": {"id", "version", "configuration", "created_at", "updated_at"},
	"analytics_events":             {"session_start", "session_end", "timestamp"},
	"configuration_backups":        {"id", "name", "version", "configuration", "created_at"},
	"configuration_templates":      {"id", "name", "description", "category", "configuration", "created_at"},
	"configuration_profiles":       {"profile_id", "name", "description", "user_id", "configuration", "created_at", "updated_at", "is_active", "tags"},
	"wizard_sessions":              {"session_id", "user_id", "current_step", "total_steps", "step_data", "configuration", "started_at", "last_activity", "is_completed", "config_type"},
	"wizard_completion":            {"user_id", "completed_at"},
	"wizard_progress":              {"user_id", "current_step", "step_data", "all_data", "updated_at"},
	"log_entries":                  {"id", "collection_id", "timestamp", "level", "component", "message", "context"},
	"favorite_shares":              {"id", "favorite_id", "shared_by_user", "shared_with", "permissions", "created_at", "is_active"},
	"login_attempts":               {"success", "created_at"},
	"system_alerts":                {"severity", "resolved", "created_at"},
	"thumbnail_cache":              {"id", "video_id", "position", "url", "width", "height", "file_size", "created_at"},
	"cache_activity":               {"type", "cache_key", "provider", "hit", "timestamp"},
	"storage_roots":                {"allow_empty", "approved_class", "approved_class_source"},                                            // v21, v27 (CAT-SECQ1-B)
	"sync_endpoints":               {"approved_class", "approved_class_source"},                                                           // v27 (CAT-SECQ1-B)
	"login_throttle":               {"subject", "scope", "failures", "blocked_until_ms", "last_failure_ms", "last_success_ms", "version"}, // v28 (CAT-LOGIN-LOCK-1)
}
