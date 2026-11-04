package repository

import (
	"context"
	"path/filepath"
	"testing"

	"catalogizer/config"
	"catalogizer/database"
	"catalogizer/models"

	"github.com/stretchr/testify/require"
)

// Found by the LIVE admin spec: DELETE /users/:id answered 500 for every user that had ever logged in or registered, because
// auth_audit_log.user_id (and user_permissions.granted_by) reference users(id) without ON DELETE. Real SQLite, real schema, FKs enforced.
func TestUserRepository_Delete_UserWithAuditTrail(t *testing.T) {
	db, err := database.NewConnection(&config.DatabaseConfig{Path: filepath.Join(t.TempDir(), "u.db")})
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, db.RunMigrations(context.Background()))

	repo := NewUserRepository(db)
	id, err := repo.Create(&models.User{Username: "deleteme", Email: "d@example.com", PasswordHash: "x", Salt: "y", RoleID: 2, IsActive: true})
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO auth_audit_log (user_id, event_type) VALUES (?, 'login')`, id)
	require.NoError(t, err)

	require.NoError(t, repo.Delete(id), "a user with audit history must be deletable")

	var users, audits, orphaned int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM users WHERE id = ?`, id).Scan(&users))
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM auth_audit_log WHERE event_type = 'login'`).Scan(&audits))
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM auth_audit_log WHERE user_id IS NULL AND event_type = 'login'`).Scan(&orphaned))
	require.Equal(t, 0, users, "user row is gone")
	require.Equal(t, 1, audits, "the audit trail is retained")
	require.Equal(t, 1, orphaned, "the retained audit row is detached from the deleted user")
}
