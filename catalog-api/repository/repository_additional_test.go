package repository

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"testing"
	"time"

	"catalogizer/database"
	mediamodels "catalogizer/internal/media/models"
	topmodels "catalogizer/models"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ===========================================================================
// UserRepository — Session, Role, and missing method coverage
// ===========================================================================

// ---------------------------------------------------------------------------
// CreateSession
// ---------------------------------------------------------------------------

func TestUserRepository_CreateSession(t *testing.T) {
	now := time.Now()
	refreshToken := "refresh456"
	ipAddr := "127.0.0.1"
	userAgent := "TestAgent/1.0"

	tests := []struct {
		name    string
		setup   func(mock sqlmock.Sqlmock)
		wantID  int
		wantErr bool
	}{
		{
			name: "success",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("INSERT INTO user_sessions").
					WillReturnResult(sqlmock.NewResult(7, 1))
			},
			wantID: 7,
		},
		{
			name: "database error",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("INSERT INTO user_sessions").
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newMockUserRepo(t)
			tt.setup(mock)

			session := &topmodels.UserSession{
				UserID:         1,
				SessionToken:   "token123",
				RefreshToken:   &refreshToken,
				DeviceInfo:     topmodels.DeviceInfo{},
				IPAddress:      &ipAddr,
				UserAgent:      &userAgent,
				IsActive:       true,
				ExpiresAt:      now.Add(24 * time.Hour),
				CreatedAt:      now,
				LastActivityAt: now,
			}

			id, err := repo.CreateSession(session)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantID, id)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ---------------------------------------------------------------------------
// DeactivateSession / DeactivateAllUserSessions
// ---------------------------------------------------------------------------

func TestUserRepository_DeactivateSession(t *testing.T) {
	repo, mock := newMockUserRepo(t)
	mock.ExpectExec("UPDATE user_sessions SET is_active = 0 WHERE id").
		WithArgs(5).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := repo.DeactivateSession(5)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUserRepository_DeactivateAllUserSessions(t *testing.T) {
	repo, mock := newMockUserRepo(t)
	mock.ExpectExec("UPDATE user_sessions SET is_active = 0 WHERE user_id").
		WithArgs(3).
		WillReturnResult(sqlmock.NewResult(0, 2))

	err := repo.DeactivateAllUserSessions(3)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// UpdateSessionTokens / UpdateSessionTokensAndExpiry / UpdateSessionActivity
// ---------------------------------------------------------------------------

func TestUserRepository_UpdateSessionTokens(t *testing.T) {
	repo, mock := newMockUserRepo(t)
	mock.ExpectExec("UPDATE user_sessions SET session_token").
		WithArgs("newtoken", "newrefresh", 1).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := repo.UpdateSessionTokens(1, "newtoken", "newrefresh")
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUserRepository_UpdateSessionTokensAndExpiry(t *testing.T) {
	repo, mock := newMockUserRepo(t)
	expiry := time.Now().Add(48 * time.Hour)
	mock.ExpectExec("UPDATE user_sessions SET session_token").
		WithArgs("newtoken", "newrefresh", expiry, 1).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := repo.UpdateSessionTokensAndExpiry(1, "newtoken", "newrefresh", expiry)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUserRepository_UpdateSessionActivity(t *testing.T) {
	repo, mock := newMockUserRepo(t)
	mock.ExpectExec("UPDATE user_sessions SET last_activity_at").
		WithArgs(sqlmock.AnyArg(), 1).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := repo.UpdateSessionActivity(1)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// CleanupExpiredSessions
// ---------------------------------------------------------------------------

func TestUserRepository_CleanupExpiredSessions(t *testing.T) {
	repo, mock := newMockUserRepo(t)
	mock.ExpectExec("DELETE FROM user_sessions WHERE expires_at").
		WillReturnResult(sqlmock.NewResult(0, 5))

	err := repo.CleanupExpiredSessions()
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// CreateRole
// ---------------------------------------------------------------------------

func TestUserRepository_CreateRole(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(mock sqlmock.Sqlmock)
		wantID  int
		wantErr bool
	}{
		{
			name: "success",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("INSERT INTO roles").
					WillReturnResult(sqlmock.NewResult(3, 1))
			},
			wantID: 3,
		},
		{
			name: "database error",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("INSERT INTO roles").
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newMockUserRepo(t)
			tt.setup(mock)

			desc := "Custom role"
			role := &topmodels.Role{
				Name:        "custom",
				Description: &desc,
				Permissions: topmodels.Permissions{"read", "write"},
				IsSystem:    false,
			}

			id, err := repo.CreateRole(role)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantID, id)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ---------------------------------------------------------------------------
// UpdateRole
// ---------------------------------------------------------------------------

func TestUserRepository_UpdateRole(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(mock sqlmock.Sqlmock)
		wantErr bool
		errMsg  string
	}{
		{
			name: "success",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("UPDATE roles SET").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
		},
		{
			name: "not found or system role",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("UPDATE roles SET").
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			wantErr: true,
			errMsg:  "role not found or is system role",
		},
		{
			name: "database error",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("UPDATE roles SET").
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newMockUserRepo(t)
			tt.setup(mock)

			desc := "Updated role"
			role := &topmodels.Role{
				ID:          3,
				Name:        "updated",
				Description: &desc,
				Permissions: topmodels.Permissions{"read"},
			}

			err := repo.UpdateRole(role)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
				return
			}
			assert.NoError(t, err)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ---------------------------------------------------------------------------
// DeleteRole
// ---------------------------------------------------------------------------

func TestUserRepository_DeleteRole(t *testing.T) {
	tests := []struct {
		name    string
		roleID  int
		setup   func(mock sqlmock.Sqlmock)
		wantErr bool
		errMsg  string
	}{
		{
			name:   "success",
			roleID: 3,
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectQuery("SELECT COUNT").
					WithArgs(3).
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
				mock.ExpectExec("DELETE FROM roles").
					WithArgs(3).
					WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectCommit()
			},
		},
		{
			name:   "role has users",
			roleID: 1,
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectQuery("SELECT COUNT").
					WithArgs(1).
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(5))
				mock.ExpectRollback()
			},
			wantErr: true,
			errMsg:  "cannot delete role that is assigned to users",
		},
		{
			name:   "system role not deletable",
			roleID: 1,
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectQuery("SELECT COUNT").
					WithArgs(1).
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
				mock.ExpectExec("DELETE FROM roles").
					WithArgs(1).
					WillReturnResult(sqlmock.NewResult(0, 0))
				mock.ExpectRollback()
			},
			wantErr: true,
			errMsg:  "role not found or is system role",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newMockUserRepo(t)
			tt.setup(mock)

			err := repo.DeleteRole(tt.roleID)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
				return
			}
			assert.NoError(t, err)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ---------------------------------------------------------------------------
// ListRoles
// ---------------------------------------------------------------------------

func TestUserRepository_ListRoles(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name    string
		setup   func(mock sqlmock.Sqlmock)
		want    int
		wantErr bool
	}{
		{
			name: "returns roles",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT .+ FROM roles ORDER BY").
					WillReturnRows(sqlmock.NewRows([]string{
						"id", "name", "description", "permissions", "is_system", "created_at", "updated_at",
					}).
						AddRow(1, "admin", "Admin", `["all"]`, true, now, now).
						AddRow(2, "user", "User", `["read"]`, true, now, now))
			},
			want: 2,
		},
		{
			name: "empty",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT .+ FROM roles ORDER BY").
					WillReturnRows(sqlmock.NewRows([]string{
						"id", "name", "description", "permissions", "is_system", "created_at", "updated_at",
					}))
			},
			want: 0,
		},
		{
			name: "database error",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT .+ FROM roles ORDER BY").
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newMockUserRepo(t)
			tt.setup(mock)

			roles, err := repo.ListRoles()
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Len(t, roles, tt.want)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ---------------------------------------------------------------------------
// GetActiveUserSessions
// ---------------------------------------------------------------------------

func TestUserRepository_GetActiveUserSessions(t *testing.T) {
	now := time.Now()
	sessionColumns := []string{
		"id", "user_id", "session_token", "refresh_token", "device_info",
		"ip_address", "user_agent", "is_active", "expires_at", "created_at", "last_activity_at",
	}

	tests := []struct {
		name    string
		userID  int
		setup   func(mock sqlmock.Sqlmock)
		want    int
		wantErr bool
	}{
		{
			name:   "returns sessions",
			userID: 1,
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT .+ FROM user_sessions").
					WillReturnRows(sqlmock.NewRows(sessionColumns).
						AddRow(1, 1, "token1", "refresh1", `{}`, "127.0.0.1", "Agent/1.0",
							true, now.Add(24*time.Hour), now, now))
			},
			want: 1,
		},
		{
			name:   "empty",
			userID: 99,
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT .+ FROM user_sessions").
					WillReturnRows(sqlmock.NewRows(sessionColumns))
			},
			want: 0,
		},
		{
			name:   "database error",
			userID: 1,
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT .+ FROM user_sessions").
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newMockUserRepo(t)
			tt.setup(mock)

			sessions, err := repo.GetActiveUserSessions(tt.userID)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Len(t, sessions, tt.want)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ===========================================================================
// MediaItemRepository — Update, Search, GetByParent, GetMediaTypes,
//                        ListDuplicateGroups, GetDuplicates
// ===========================================================================

// ---------------------------------------------------------------------------
// Update
// ---------------------------------------------------------------------------

func TestMediaItemRepository_Update(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(mock sqlmock.Sqlmock)
		wantErr bool
	}{
		{
			name: "success",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("UPDATE media_items SET").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
		},
		{
			name: "database error",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("UPDATE media_items SET").
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newMockMediaItemRepo(t)
			tt.setup(mock)

			item := &mediamodels.MediaItem{
				ID:          1,
				MediaTypeID: 2,
				Title:       "Updated Title",
				Status:      "confirmed",
				Genre:       []string{"Comedy"},
			}
			err := repo.Update(context.Background(), item)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ---------------------------------------------------------------------------
// Search
// ---------------------------------------------------------------------------

func TestMediaItemRepository_Search(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name      string
		query     string
		types     []int64
		limit     int
		offset    int
		setup     func(mock sqlmock.Sqlmock)
		wantErr   bool
		wantCount int
		wantTotal int64
	}{
		{
			name:   "basic search",
			query:  "Matrix",
			types:  nil,
			limit:  10,
			offset: 0,
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT COUNT").
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
				mock.ExpectQuery("SELECT .+ FROM media_items").
					WillReturnRows(sqlmock.NewRows(mediaItemColumns).
						AddRow(sampleMediaItemRow(now)...))
			},
			wantCount: 1,
			wantTotal: 1,
		},
		{
			name:   "search with type filter",
			query:  "Test",
			types:  []int64{1, 2},
			limit:  10,
			offset: 0,
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT COUNT").
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
				mock.ExpectQuery("SELECT .+ FROM media_items").
					WillReturnRows(sqlmock.NewRows(mediaItemColumns))
			},
			wantCount: 0,
			wantTotal: 0,
		},
		{
			name:   "count error",
			query:  "Test",
			types:  nil,
			limit:  10,
			offset: 0,
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT COUNT").
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newMockMediaItemRepo(t)
			tt.setup(mock)

			items, total, err := repo.Search(context.Background(), tt.query, tt.types, tt.limit, tt.offset)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Len(t, items, tt.wantCount)
			assert.Equal(t, tt.wantTotal, total)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ---------------------------------------------------------------------------
// GetByParent
// ---------------------------------------------------------------------------

func TestMediaItemRepository_GetByParent(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name      string
		parentID  int64
		limit     int
		offset    int
		setup     func(mock sqlmock.Sqlmock)
		wantErr   bool
		wantCount int
		wantTotal int64
	}{
		{
			name:     "returns children",
			parentID: 5,
			limit:    10,
			offset:   0,
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT COUNT").
					WithArgs(int64(5)).
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))
				row1 := sampleMediaItemRow(now)
				row1[0] = int64(11)
				row2 := sampleMediaItemRow(now)
				row2[0] = int64(12)
				mock.ExpectQuery("SELECT .+ FROM media_items WHERE parent_id").
					WithArgs(int64(5), 10, 0).
					WillReturnRows(sqlmock.NewRows(mediaItemColumns).
						AddRow(row1...).
						AddRow(row2...))
			},
			wantCount: 2,
			wantTotal: 2,
		},
		{
			name:     "count error",
			parentID: 5,
			limit:    10,
			offset:   0,
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT COUNT").
					WithArgs(int64(5)).
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newMockMediaItemRepo(t)
			tt.setup(mock)

			items, total, err := repo.GetByParent(context.Background(), tt.parentID, tt.limit, tt.offset)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Len(t, items, tt.wantCount)
			assert.Equal(t, tt.wantTotal, total)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ---------------------------------------------------------------------------
// GetMediaTypes
// ---------------------------------------------------------------------------

func TestMediaItemRepository_GetMediaTypes(t *testing.T) {
	now := time.Now()

	mediaTypeColumns := []string{
		"id", "name", "description", "detection_patterns", "metadata_providers",
		"created_at", "updated_at",
	}

	tests := []struct {
		name    string
		setup   func(mock sqlmock.Sqlmock)
		want    int
		wantErr bool
	}{
		{
			name: "returns types",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT .+ FROM media_types").
					WillReturnRows(sqlmock.NewRows(mediaTypeColumns).
						AddRow(int64(1), "movie", "Movies", `["*.mkv"]`, `["tmdb"]`, now, now).
						AddRow(int64(2), "tv_show", "TV Shows", nil, nil, now, now))
			},
			want: 2,
		},
		{
			name: "empty",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT .+ FROM media_types").
					WillReturnRows(sqlmock.NewRows(mediaTypeColumns))
			},
			want: 0,
		},
		{
			name: "database error",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT .+ FROM media_types").
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newMockMediaItemRepo(t)
			tt.setup(mock)

			types, err := repo.GetMediaTypes(context.Background())
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Len(t, types, tt.want)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ---------------------------------------------------------------------------
// GetDuplicates
// ---------------------------------------------------------------------------

func TestMediaItemRepository_GetDuplicates(t *testing.T) {
	now := time.Now()
	year := 2024

	tests := []struct {
		name      string
		title     string
		typeID    int64
		year      *int
		setup     func(mock sqlmock.Sqlmock)
		wantErr   bool
		wantCount int
	}{
		{
			name:   "without year",
			title:  "Test Movie",
			typeID: 2,
			year:   nil,
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT .+ FROM media_items WHERE title").
					WithArgs("Test Movie", int64(2)).
					WillReturnRows(sqlmock.NewRows(mediaItemColumns).
						AddRow(sampleMediaItemRow(now)...))
			},
			wantCount: 1,
		},
		{
			name:   "with year",
			title:  "Test Movie",
			typeID: 2,
			year:   &year,
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT .+ FROM media_items WHERE title").
					WithArgs("Test Movie", int64(2), 2024).
					WillReturnRows(sqlmock.NewRows(mediaItemColumns).
						AddRow(sampleMediaItemRow(now)...))
			},
			wantCount: 1,
		},
		{
			name:   "database error",
			title:  "Test",
			typeID: 2,
			year:   nil,
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT .+ FROM media_items WHERE title").
					WithArgs("Test", int64(2)).
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newMockMediaItemRepo(t)
			tt.setup(mock)

			items, err := repo.GetDuplicates(context.Background(), tt.title, tt.typeID, tt.year)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Len(t, items, tt.wantCount)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ---------------------------------------------------------------------------
// ListDuplicateGroups
// ---------------------------------------------------------------------------

func TestMediaItemRepository_ListDuplicateGroups(t *testing.T) {
	tests := []struct {
		name      string
		limit     int
		offset    int
		setup     func(mock sqlmock.Sqlmock)
		wantErr   bool
		wantCount int
		wantTotal int64
	}{
		{
			name:   "returns groups",
			limit:  10,
			offset: 0,
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT COUNT").
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))
				mock.ExpectQuery("SELECT mi.title, mi.media_type_id, mt.name, COUNT").
					WithArgs(10, 0).
					WillReturnRows(sqlmock.NewRows([]string{"title", "media_type_id", "name", "cnt"}).
						AddRow("Duplicate Movie", int64(1), "movie", 3).
						AddRow("Duplicate Song", int64(7), "song", 2))
			},
			wantCount: 2,
			wantTotal: 2,
		},
		{
			name:   "count error",
			limit:  10,
			offset: 0,
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT COUNT").
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newMockMediaItemRepo(t)
			tt.setup(mock)

			groups, total, err := repo.ListDuplicateGroups(context.Background(), tt.limit, tt.offset)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Len(t, groups, tt.wantCount)
			assert.Equal(t, tt.wantTotal, total)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ===========================================================================
// MediaCollectionRepository
// ===========================================================================

func newMockMediaCollectionRepo(t *testing.T) (*MediaCollectionRepository, sqlmock.Sqlmock) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	db := database.WrapDB(sqlDB, database.DialectSQLite)
	return NewMediaCollectionRepository(db), mock
}

var collectionColumns = []string{
	"id", "name", "collection_type", "description", "total_items",
	"external_ids", "cover_url", "created_at", "updated_at",
	"is_smart", "smart_rules", "owner_user_id", "is_public",
}

// ---------------------------------------------------------------------------
// Create
// ---------------------------------------------------------------------------

func TestMediaCollectionRepository_Create(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(mock sqlmock.Sqlmock)
		wantID  int64
		wantErr bool
	}{
		{
			name: "success",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("INSERT INTO media_collections").
					WillReturnResult(sqlmock.NewResult(5, 1))
			},
			wantID: 5,
		},
		{
			name: "database error",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("INSERT INTO media_collections").
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newMockMediaCollectionRepo(t)
			tt.setup(mock)

			desc := "A collection"
			coll := &mediamodels.MediaCollection{
				Name:           "My Collection",
				CollectionType: "playlist",
				Description:    &desc,
				TotalItems:     10,
				ExternalIDs:    map[string]string{"imdb": "tt123"},
			}

			id, err := repo.Create(context.Background(), coll)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantID, id)
			assert.Equal(t, tt.wantID, coll.ID)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ---------------------------------------------------------------------------
// GetByID
// ---------------------------------------------------------------------------

func TestMediaCollectionRepository_GetByID(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name    string
		id      int64
		setup   func(mock sqlmock.Sqlmock)
		wantErr bool
		errMsg  string
		check   func(t *testing.T, coll *mediamodels.MediaCollection)
	}{
		{
			name: "found",
			id:   1,
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT .+ FROM media_collections WHERE id").
					WithArgs(int64(1)).
					WillReturnRows(sqlmock.NewRows(collectionColumns).
						AddRow(int64(1), "My Collection", "playlist", "A desc", 10,
							`{"imdb":"tt123"}`, nil, now, now, false, nil, nil, false))
			},
			check: func(t *testing.T, coll *mediamodels.MediaCollection) {
				assert.Equal(t, int64(1), coll.ID)
				assert.Equal(t, "My Collection", coll.Name)
				assert.Equal(t, "playlist", coll.CollectionType)
				assert.Equal(t, 10, coll.TotalItems)
				assert.Equal(t, "tt123", coll.ExternalIDs["imdb"])
			},
		},
		{
			name: "not found",
			id:   999,
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT .+ FROM media_collections WHERE id").
					WithArgs(int64(999)).
					WillReturnError(sql.ErrNoRows)
			},
			wantErr: true,
			errMsg:  "media collection not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newMockMediaCollectionRepo(t)
			tt.setup(mock)

			coll, err := repo.GetByID(context.Background(), tt.id)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
				return
			}
			require.NoError(t, err)
			require.NotNil(t, coll)
			tt.check(t, coll)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ---------------------------------------------------------------------------
// Update
// ---------------------------------------------------------------------------

func TestMediaCollectionRepository_Update(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(mock sqlmock.Sqlmock)
		wantErr bool
		errMsg  string
	}{
		{
			name: "success",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("UPDATE media_collections SET").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
		},
		{
			name: "not found",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("UPDATE media_collections SET").
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			wantErr: true,
			errMsg:  "media collection not found",
		},
		{
			name: "database error",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("UPDATE media_collections SET").
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newMockMediaCollectionRepo(t)
			tt.setup(mock)

			coll := &mediamodels.MediaCollection{
				ID:             1,
				Name:           "Updated",
				CollectionType: "playlist",
				TotalItems:     5,
			}
			err := repo.Update(context.Background(), coll)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
				return
			}
			assert.NoError(t, err)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ---------------------------------------------------------------------------
// Delete
// ---------------------------------------------------------------------------

func TestMediaCollectionRepository_Delete(t *testing.T) {
	tests := []struct {
		name    string
		id      int64
		setup   func(mock sqlmock.Sqlmock)
		wantErr bool
		errMsg  string
	}{
		{
			name: "success",
			id:   1,
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("DELETE FROM media_collections WHERE id").
					WithArgs(int64(1)).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
		},
		{
			name: "not found",
			id:   999,
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("DELETE FROM media_collections WHERE id").
					WithArgs(int64(999)).
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			wantErr: true,
			errMsg:  "media collection not found",
		},
		{
			name: "database error",
			id:   1,
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("DELETE FROM media_collections WHERE id").
					WithArgs(int64(1)).
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newMockMediaCollectionRepo(t)
			tt.setup(mock)

			err := repo.Delete(context.Background(), tt.id)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
				return
			}
			assert.NoError(t, err)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ---------------------------------------------------------------------------
// List
// ---------------------------------------------------------------------------

func TestMediaCollectionRepository_List(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name      string
		limit     int
		offset    int
		setup     func(mock sqlmock.Sqlmock)
		wantErr   bool
		wantCount int
		wantTotal int
	}{
		{
			name:   "returns collections",
			limit:  10,
			offset: 0,
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT COUNT").
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
				mock.ExpectQuery("SELECT .+ FROM media_collections ORDER BY").
					WithArgs(10, 0).
					WillReturnRows(sqlmock.NewRows(collectionColumns).
						AddRow(int64(1), "Coll1", "playlist", nil, 5,
							`null`, nil, now, now, false, nil, nil, false))
			},
			wantCount: 1,
			wantTotal: 1,
		},
		{
			name:   "count error",
			limit:  10,
			offset: 0,
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT COUNT").
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newMockMediaCollectionRepo(t)
			tt.setup(mock)

			colls, total, err := repo.List(context.Background(), tt.limit, tt.offset)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Len(t, colls, tt.wantCount)
			assert.Equal(t, tt.wantTotal, total)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ===========================================================================
// FileRepository — additional filter and sort coverage
// ===========================================================================

// ---------------------------------------------------------------------------
// buildSortClause edge cases
// ---------------------------------------------------------------------------

func TestFileRepository_BuildSortClause(t *testing.T) {
	repo := NewFileRepository(nil) // db is not used in buildSortClause

	tests := []struct {
		name   string
		sort   topmodels.SortOptions
		expect string
	}{
		{"name asc", topmodels.SortOptions{Field: "name", Order: "asc"}, " ORDER BY f.name ASC"},
		{"size desc", topmodels.SortOptions{Field: "size", Order: "desc"}, " ORDER BY f.size DESC"},
		{"modified_at asc", topmodels.SortOptions{Field: "modified_at", Order: "asc"}, " ORDER BY f.modified_at ASC"},
		{"created_at desc", topmodels.SortOptions{Field: "created_at", Order: "desc"}, " ORDER BY f.created_at DESC"},
		{"path asc", topmodels.SortOptions{Field: "path", Order: "asc"}, " ORDER BY f.path ASC"},
		{"extension asc", topmodels.SortOptions{Field: "extension", Order: "asc"}, " ORDER BY f.extension ASC"},
		{"unknown defaults to name", topmodels.SortOptions{Field: "unknown", Order: "asc"}, " ORDER BY f.name ASC"},
		{"empty field defaults to name", topmodels.SortOptions{Field: "", Order: "desc"}, " ORDER BY f.name DESC"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := repo.buildSortClause(tt.sort)
			assert.Equal(t, tt.expect, got)
		})
	}
}

// ---------------------------------------------------------------------------
// applySearchFilters edge cases
// ---------------------------------------------------------------------------

func TestFileRepository_ApplySearchFilters(t *testing.T) {
	repo := NewFileRepository(nil)

	minSize := int64(1024)
	maxSize := int64(1048576)
	modAfter := time.Now().Add(-24 * time.Hour)
	modBefore := time.Now()

	tests := []struct {
		name        string
		filter      topmodels.SearchFilter
		wantClauses []string
		notClauses  []string
	}{
		{
			name:        "include deleted skips deleted filter",
			filter:      topmodels.SearchFilter{IncludeDeleted: true},
			wantClauses: []string{"f.is_directory = 0"},
			notClauses:  []string{"f.deleted = 0"},
		},
		{
			name:        "min and max size",
			filter:      topmodels.SearchFilter{MinSize: &minSize, MaxSize: &maxSize},
			wantClauses: []string{"f.size >= ?", "f.size <= ?"},
		},
		{
			name:        "modified after and before",
			filter:      topmodels.SearchFilter{ModifiedAfter: &modAfter, ModifiedBefore: &modBefore},
			wantClauses: []string{"f.modified_at >= ?", "f.modified_at <= ?"},
		},
		{
			name:        "only duplicates",
			filter:      topmodels.SearchFilter{OnlyDuplicates: true},
			wantClauses: []string{"f.is_duplicate = 1"},
		},
		{
			name:        "exclude duplicates",
			filter:      topmodels.SearchFilter{ExcludeDuplicates: true},
			wantClauses: []string{"f.is_duplicate = 0"},
		},
		{
			name:       "include directories omits is_directory filter",
			filter:     topmodels.SearchFilter{IncludeDirectories: true},
			notClauses: []string{"f.is_directory = 0"},
		},
		{
			name:        "storage roots filter",
			filter:      topmodels.SearchFilter{StorageRoots: []string{"root1", "root2"}},
			wantClauses: []string{"sr.name IN ("},
		},
		{
			name:        "name filter",
			filter:      topmodels.SearchFilter{Name: "readme"},
			wantClauses: []string{"f.name LIKE ?"},
		},
		{
			name:        "file type filter",
			filter:      topmodels.SearchFilter{FileType: "video"},
			wantClauses: []string{"f.file_type = ?"},
		},
		{
			name:        "mime type filter",
			filter:      topmodels.SearchFilter{MimeType: "text/plain"},
			wantClauses: []string{"f.mime_type = ?"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			baseQuery := "WHERE 1=1"
			got, _ := repo.applySearchFilters(baseQuery, []interface{}{}, tt.filter)
			for _, clause := range tt.wantClauses {
				assert.Contains(t, got, clause)
			}
			for _, clause := range tt.notClauses {
				assert.NotContains(t, got, clause)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// GetFilesWithHash
// ---------------------------------------------------------------------------

func TestFileRepository_GetFilesWithHash(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name    string
		hash    string
		root    string
		setup   func(mock sqlmock.Sqlmock)
		want    int
		wantErr bool
	}{
		{
			name: "returns files",
			hash: "abc123",
			root: "my-root",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT .+ FROM files f").
					WithArgs("abc123", "abc123", "abc123", "abc123", "abc123", "my-root").
					WillReturnRows(sqlmock.NewRows(fileColumns).
						AddRow(sampleFileRow(now)...))
			},
			want: 1,
		},
		{
			name: "no matches",
			hash: "xyz",
			root: "my-root",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT .+ FROM files f").
					WithArgs("xyz", "xyz", "xyz", "xyz", "xyz", "my-root").
					WillReturnRows(sqlmock.NewRows(fileColumns))
			},
			want: 0,
		},
		{
			name: "database error",
			hash: "abc",
			root: "my-root",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT .+ FROM files f").
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newMockFileRepo(t)
			tt.setup(mock)

			files, err := repo.GetFilesWithHash(context.Background(), tt.hash, tt.root)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Len(t, files, tt.want)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ===========================================================================
// marshalJSONField / unmarshalJSONFieldString helpers
// ===========================================================================

func TestMarshalJSONField(t *testing.T) {
	tests := []struct {
		name    string
		input   interface{}
		wantNil bool
	}{
		{"nil input", nil, true},
		{"empty string slice", []string{}, true},
		{"nil cast crew pointer", (*mediamodels.CastCrew)(nil), true},
		{"non-empty slice", []string{"Action", "Drama"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := marshalJSONField(tt.input)
			assert.NoError(t, err)
			if tt.wantNil {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
			}
		})
	}
}

func TestUnmarshalJSONFieldString(t *testing.T) {
	tests := []struct {
		name    string
		data    string
		wantErr bool
	}{
		{"empty string", "", false},
		{"null string", "null", false},
		{"valid json", `{"key":"value"}`, false},
		{"invalid json", `{bad json}`, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var target map[string]string
			err := unmarshalJSONFieldString(tt.data, &target)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestMarshalJSONFieldString(t *testing.T) {
	tests := []struct {
		name  string
		input interface{}
		want  string
	}{
		{"nil becomes null", nil, "null"},
		{"map input", map[string]string{"k": "v"}, `{"k":"v"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := marshalJSONFieldString(tt.input)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, result)
		})
	}
}

// ===========================================================================
// Constructor tests
// ===========================================================================

func TestNewConstructors(t *testing.T) {
	sqlDB, _, err := sqlmock.New()
	require.NoError(t, err)
	db := database.WrapDB(sqlDB, database.DialectSQLite)

	t.Run("NewFileRepository", func(t *testing.T) {
		repo := NewFileRepository(db)
		assert.NotNil(t, repo)
	})

	t.Run("NewUserRepository", func(t *testing.T) {
		repo := NewUserRepository(db)
		assert.NotNil(t, repo)
	})

	t.Run("NewMediaItemRepository", func(t *testing.T) {
		repo := NewMediaItemRepository(db)
		assert.NotNil(t, repo)
	})

	t.Run("NewMediaCollectionRepository", func(t *testing.T) {
		repo := NewMediaCollectionRepository(db)
		assert.NotNil(t, repo)
	})
}

// ===========================================================================
// FileRepository — GetDirectoryContents with different sort fields
// ===========================================================================

func TestFileRepository_GetDirectoryContents_SortVariations(t *testing.T) {
	now := time.Now()

	sortFields := []string{"size", "modified_at", "created_at", "path", "extension"}

	for _, field := range sortFields {
		t.Run("sort by "+field, func(t *testing.T) {
			repo, mock := newMockFileRepo(t)

			mock.ExpectQuery("SELECT COUNT").
				WithArgs("root").
				WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
			mock.ExpectQuery("SELECT .+ FROM files f").
				WithArgs("root", 10, 0).
				WillReturnRows(sqlmock.NewRows(fileColumns).
					AddRow(sampleFileRow(now)...))

			result, err := repo.GetDirectoryContents(context.Background(), "root", "/",
				topmodels.PaginationOptions{Page: 1, Limit: 10},
				topmodels.SortOptions{Field: field, Order: "desc"})
			require.NoError(t, err)
			assert.Equal(t, int64(1), result.TotalCount)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// Ensure unused imports are referenced.
var _ driver.Value
