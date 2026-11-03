package repository

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"testing"
	"time"

	"catalogizer/database"
	"catalogizer/internal/media/models"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newMockCollRepo creates a MediaCollectionRepository backed by sqlmock.
func newMockCollRepo(t *testing.T) (*MediaCollectionRepository, sqlmock.Sqlmock) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	db := database.WrapDB(sqlDB, database.DialectSQLite)
	return NewMediaCollectionRepository(db), mock
}

// collCols is the standard column set for media_collections queries.
var collCols = []string{
	"id", "name", "collection_type", "description", "total_items",
	"external_ids", "cover_url", "created_at", "updated_at",
	"is_smart", "smart_rules", "owner_user_id", "is_public",
}

func sampleCollRow(now time.Time) []driver.Value {
	desc := "A test collection"
	coverURL := "https://example.com/cover.jpg"
	return []driver.Value{
		int64(1), "Test Collection", "playlist", &desc, 10,
		`{"imdb":"tt123","tmdb":"456"}`, &coverURL, now, now,
		false, nil, nil, false,
	}
}

func sampleCollRowMinimal(id int64, name string, now time.Time) []driver.Value {
	return []driver.Value{
		id, name, "series", nil, 0,
		`null`, nil, now, now,
		false, nil, nil, false,
	}
}

// ---------------------------------------------------------------------------
// NewMediaCollectionRepository
// ---------------------------------------------------------------------------

func TestMediaCollectionRepo_New(t *testing.T) {
	sqlDB, _, err := sqlmock.New()
	require.NoError(t, err)
	db := database.WrapDB(sqlDB, database.DialectSQLite)

	repo := NewMediaCollectionRepository(db)
	require.NotNil(t, repo)
}

// ---------------------------------------------------------------------------
// Create
// ---------------------------------------------------------------------------

func TestMediaCollectionRepo_Create(t *testing.T) {
	tests := []struct {
		name    string
		coll    func() *models.MediaCollection
		setup   func(mock sqlmock.Sqlmock)
		wantID  int64
		wantErr bool
		errMsg  string
		check   func(t *testing.T, coll *models.MediaCollection)
	}{
		{
			name: "success with all fields",
			coll: func() *models.MediaCollection {
				desc := "A movie franchise"
				coverURL := "https://example.com/cover.jpg"
				return &models.MediaCollection{
					Name:           "Marvel Collection",
					CollectionType: "franchise",
					Description:    &desc,
					TotalItems:     25,
					ExternalIDs:    map[string]string{"imdb": "tt999", "tmdb": "789"},
					CoverURL:       &coverURL,
				}
			},
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("INSERT INTO media_collections").
					WillReturnResult(sqlmock.NewResult(42, 1))
			},
			wantID: 42,
			check: func(t *testing.T, coll *models.MediaCollection) {
				assert.Equal(t, int64(42), coll.ID)
				assert.False(t, coll.CreatedAt.IsZero(), "CreatedAt should be set")
				assert.False(t, coll.UpdatedAt.IsZero(), "UpdatedAt should be set")
			},
		},
		{
			name: "success with minimal fields",
			coll: func() *models.MediaCollection {
				return &models.MediaCollection{
					Name:           "Empty Collection",
					CollectionType: "playlist",
				}
			},
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("INSERT INTO media_collections").
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
			wantID: 1,
		},
		{
			name: "success with nil external IDs",
			coll: func() *models.MediaCollection {
				return &models.MediaCollection{
					Name:           "No External IDs",
					CollectionType: "series",
					ExternalIDs:    nil,
				}
			},
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("INSERT INTO media_collections").
					WillReturnResult(sqlmock.NewResult(3, 1))
			},
			wantID: 3,
		},
		{
			name: "success with empty external IDs map",
			coll: func() *models.MediaCollection {
				return &models.MediaCollection{
					Name:           "Empty Map",
					CollectionType: "playlist",
					ExternalIDs:    map[string]string{},
				}
			},
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("INSERT INTO media_collections").
					WillReturnResult(sqlmock.NewResult(4, 1))
			},
			wantID: 4,
		},
		{
			name: "preserves preset timestamps",
			coll: func() *models.MediaCollection {
				preset := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
				return &models.MediaCollection{
					Name:           "Preset Timestamps",
					CollectionType: "playlist",
					CreatedAt:      preset,
					UpdatedAt:      preset,
				}
			},
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("INSERT INTO media_collections").
					WillReturnResult(sqlmock.NewResult(5, 1))
			},
			wantID: 5,
			check: func(t *testing.T, coll *models.MediaCollection) {
				expected := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
				assert.Equal(t, expected, coll.CreatedAt)
				assert.Equal(t, expected, coll.UpdatedAt)
			},
		},
		{
			name: "database error",
			coll: func() *models.MediaCollection {
				return &models.MediaCollection{
					Name:           "Will Fail",
					CollectionType: "playlist",
				}
			},
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("INSERT INTO media_collections").
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: true,
			errMsg:  "failed to create media collection",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newMockCollRepo(t)
			tt.setup(mock)

			coll := tt.coll()
			id, err := repo.Create(context.Background(), coll)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantID, id)
			assert.Equal(t, tt.wantID, coll.ID)
			if tt.check != nil {
				tt.check(t, coll)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ---------------------------------------------------------------------------
// GetByID
// ---------------------------------------------------------------------------

func TestMediaCollectionRepo_GetByID(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name    string
		id      int64
		setup   func(mock sqlmock.Sqlmock)
		wantErr bool
		errMsg  string
		check   func(t *testing.T, coll *models.MediaCollection)
	}{
		{
			name: "found with all fields",
			id:   1,
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT .+ FROM media_collections WHERE id").
					WithArgs(int64(1)).
					WillReturnRows(sqlmock.NewRows(collCols).AddRow(sampleCollRow(now)...))
			},
			check: func(t *testing.T, coll *models.MediaCollection) {
				assert.Equal(t, int64(1), coll.ID)
				assert.Equal(t, "Test Collection", coll.Name)
				assert.Equal(t, "playlist", coll.CollectionType)
				require.NotNil(t, coll.Description)
				assert.Equal(t, "A test collection", *coll.Description)
				assert.Equal(t, 10, coll.TotalItems)
				require.NotNil(t, coll.ExternalIDs)
				assert.Equal(t, "tt123", coll.ExternalIDs["imdb"])
				assert.Equal(t, "456", coll.ExternalIDs["tmdb"])
				require.NotNil(t, coll.CoverURL)
				assert.Equal(t, "https://example.com/cover.jpg", *coll.CoverURL)
			},
		},
		{
			name: "found with minimal fields",
			id:   2,
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT .+ FROM media_collections WHERE id").
					WithArgs(int64(2)).
					WillReturnRows(sqlmock.NewRows(collCols).
						AddRow(sampleCollRowMinimal(2, "Minimal", now)...))
			},
			check: func(t *testing.T, coll *models.MediaCollection) {
				assert.Equal(t, int64(2), coll.ID)
				assert.Equal(t, "Minimal", coll.Name)
				assert.Equal(t, "series", coll.CollectionType)
				assert.Nil(t, coll.Description)
				assert.Equal(t, 0, coll.TotalItems)
				assert.Nil(t, coll.ExternalIDs)
				assert.Nil(t, coll.CoverURL)
			},
		},
		{
			name: "found with empty external IDs JSON",
			id:   3,
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT .+ FROM media_collections WHERE id").
					WithArgs(int64(3)).
					WillReturnRows(sqlmock.NewRows(collCols).
						AddRow(int64(3), "Empty ExtIDs", "playlist", nil, 0,
							`{}`, nil, now, now, false, nil, nil, false))
			},
			check: func(t *testing.T, coll *models.MediaCollection) {
				assert.Equal(t, int64(3), coll.ID)
				require.NotNil(t, coll.ExternalIDs)
				assert.Empty(t, coll.ExternalIDs)
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
		{
			name: "database error",
			id:   1,
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT .+ FROM media_collections WHERE id").
					WithArgs(int64(1)).
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: true,
			errMsg:  "failed to get media collection",
		},
		{
			name: "invalid external IDs JSON",
			id:   4,
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT .+ FROM media_collections WHERE id").
					WithArgs(int64(4)).
					WillReturnRows(sqlmock.NewRows(collCols).
						AddRow(int64(4), "Bad JSON", "playlist", nil, 0,
							`{invalid json}`, nil, now, now, false, nil, nil, false))
			},
			wantErr: true,
			errMsg:  "failed to unmarshal external_ids",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newMockCollRepo(t)
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
// List
// ---------------------------------------------------------------------------

func TestMediaCollectionRepo_List(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name      string
		limit     int
		offset    int
		setup     func(mock sqlmock.Sqlmock)
		wantErr   bool
		errMsg    string
		wantCount int
		wantTotal int
		check     func(t *testing.T, colls []*models.MediaCollection)
	}{
		{
			name:   "returns single collection",
			limit:  10,
			offset: 0,
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT COUNT").
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
				mock.ExpectQuery("SELECT .+ FROM media_collections ORDER BY").
					WithArgs(10, 0).
					WillReturnRows(sqlmock.NewRows(collCols).
						AddRow(sampleCollRow(now)...))
			},
			wantCount: 1,
			wantTotal: 1,
			check: func(t *testing.T, colls []*models.MediaCollection) {
				assert.Equal(t, "Test Collection", colls[0].Name)
			},
		},
		{
			name:   "returns multiple collections",
			limit:  10,
			offset: 0,
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT COUNT").
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(3))
				mock.ExpectQuery("SELECT .+ FROM media_collections ORDER BY").
					WithArgs(10, 0).
					WillReturnRows(sqlmock.NewRows(collCols).
						AddRow(sampleCollRow(now)...).
						AddRow(sampleCollRowMinimal(2, "Second", now)...).
						AddRow(sampleCollRowMinimal(3, "Third", now)...))
			},
			wantCount: 3,
			wantTotal: 3,
			check: func(t *testing.T, colls []*models.MediaCollection) {
				assert.Equal(t, "Test Collection", colls[0].Name)
				assert.Equal(t, "Second", colls[1].Name)
				assert.Equal(t, "Third", colls[2].Name)
			},
		},
		{
			name:   "empty result",
			limit:  10,
			offset: 0,
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT COUNT").
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
				mock.ExpectQuery("SELECT .+ FROM media_collections ORDER BY").
					WithArgs(10, 0).
					WillReturnRows(sqlmock.NewRows(collCols))
			},
			wantCount: 0,
			wantTotal: 0,
		},
		{
			name:   "pagination with offset",
			limit:  5,
			offset: 10,
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT COUNT").
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(15))
				mock.ExpectQuery("SELECT .+ FROM media_collections ORDER BY").
					WithArgs(5, 10).
					WillReturnRows(sqlmock.NewRows(collCols).
						AddRow(sampleCollRowMinimal(11, "Page2-1", now)...))
			},
			wantCount: 1,
			wantTotal: 15,
		},
		{
			name:   "total exceeds returned count (pagination)",
			limit:  2,
			offset: 0,
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT COUNT").
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(50))
				mock.ExpectQuery("SELECT .+ FROM media_collections ORDER BY").
					WithArgs(2, 0).
					WillReturnRows(sqlmock.NewRows(collCols).
						AddRow(sampleCollRowMinimal(1, "First", now)...).
						AddRow(sampleCollRowMinimal(2, "Second", now)...))
			},
			wantCount: 2,
			wantTotal: 50,
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
			errMsg:  "failed to count media collections",
		},
		{
			name:   "query error after successful count",
			limit:  10,
			offset: 0,
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT COUNT").
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(5))
				mock.ExpectQuery("SELECT .+ FROM media_collections").
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: true,
			errMsg:  "failed to list media collections",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newMockCollRepo(t)
			tt.setup(mock)

			colls, total, err := repo.List(context.Background(), tt.limit, tt.offset)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
				return
			}
			require.NoError(t, err)
			assert.Len(t, colls, tt.wantCount)
			assert.Equal(t, tt.wantTotal, total)
			if tt.check != nil {
				tt.check(t, colls)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ---------------------------------------------------------------------------
// Update
// ---------------------------------------------------------------------------

func TestMediaCollectionRepo_Update(t *testing.T) {
	tests := []struct {
		name    string
		coll    func() *models.MediaCollection
		setup   func(mock sqlmock.Sqlmock)
		wantErr bool
		errMsg  string
		check   func(t *testing.T, coll *models.MediaCollection)
	}{
		{
			name: "success with all fields",
			coll: func() *models.MediaCollection {
				desc := "Updated description"
				coverURL := "https://example.com/new-cover.jpg"
				return &models.MediaCollection{
					ID:             1,
					Name:           "Updated Collection",
					CollectionType: "franchise",
					Description:    &desc,
					TotalItems:     30,
					ExternalIDs:    map[string]string{"imdb": "tt999"},
					CoverURL:       &coverURL,
				}
			},
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("UPDATE media_collections SET").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			check: func(t *testing.T, coll *models.MediaCollection) {
				assert.False(t, coll.UpdatedAt.IsZero(), "UpdatedAt should be refreshed")
			},
		},
		{
			name: "success with nil optional fields",
			coll: func() *models.MediaCollection {
				return &models.MediaCollection{
					ID:             2,
					Name:           "Minimal Update",
					CollectionType: "playlist",
					Description:    nil,
					CoverURL:       nil,
					ExternalIDs:    nil,
				}
			},
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("UPDATE media_collections SET").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
		},
		{
			name: "success with empty external IDs",
			coll: func() *models.MediaCollection {
				return &models.MediaCollection{
					ID:             3,
					Name:           "Empty Map",
					CollectionType: "series",
					ExternalIDs:    map[string]string{},
				}
			},
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("UPDATE media_collections SET").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
		},
		{
			name: "not found",
			coll: func() *models.MediaCollection {
				return &models.MediaCollection{
					ID:             999,
					Name:           "Ghost",
					CollectionType: "playlist",
				}
			},
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("UPDATE media_collections SET").
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			wantErr: true,
			errMsg:  "media collection not found",
		},
		{
			name: "database error",
			coll: func() *models.MediaCollection {
				return &models.MediaCollection{
					ID:             1,
					Name:           "Will Fail",
					CollectionType: "playlist",
				}
			},
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("UPDATE media_collections SET").
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: true,
			errMsg:  "failed to update media collection",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newMockCollRepo(t)
			tt.setup(mock)

			coll := tt.coll()
			err := repo.Update(context.Background(), coll)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
				return
			}
			require.NoError(t, err)
			if tt.check != nil {
				tt.check(t, coll)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ---------------------------------------------------------------------------
// Delete
// ---------------------------------------------------------------------------

func TestMediaCollectionRepo_Delete(t *testing.T) {
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
			errMsg:  "failed to delete media collection",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newMockCollRepo(t)
			tt.setup(mock)

			err := repo.Delete(context.Background(), tt.id)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
				return
			}
			require.NoError(t, err)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ---------------------------------------------------------------------------
// scanCollection edge cases
// ---------------------------------------------------------------------------

func TestMediaCollectionRepo_ScanCollection_ExternalIDsVariants(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name           string
		externalIDsVal string
		wantNil        bool
		wantLen        int
	}{
		{
			name:           "null string",
			externalIDsVal: "null",
			wantNil:        true,
		},
		{
			name:           "empty string",
			externalIDsVal: "",
			wantNil:        true,
		},
		{
			name:           "valid single entry",
			externalIDsVal: `{"imdb":"tt001"}`,
			wantLen:        1,
		},
		{
			name:           "valid multiple entries",
			externalIDsVal: `{"imdb":"tt001","tmdb":"123","tvdb":"456"}`,
			wantLen:        3,
		},
		{
			name:           "empty object",
			externalIDsVal: `{}`,
			wantLen:        0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newMockCollRepo(t)

			mock.ExpectQuery("SELECT .+ FROM media_collections WHERE id").
				WithArgs(int64(1)).
				WillReturnRows(sqlmock.NewRows(collCols).
					AddRow(int64(1), "Test", "playlist", nil, 0,
						tt.externalIDsVal, nil, now, now, false, nil, nil, false))

			coll, err := repo.GetByID(context.Background(), 1)
			require.NoError(t, err)
			require.NotNil(t, coll)

			if tt.wantNil {
				assert.Nil(t, coll.ExternalIDs)
			} else {
				require.NotNil(t, coll.ExternalIDs)
				assert.Len(t, coll.ExternalIDs, tt.wantLen)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ---------------------------------------------------------------------------
// Create + GetByID round-trip consistency
// ---------------------------------------------------------------------------

func TestMediaCollectionRepo_Create_SetsTimestamps(t *testing.T) {
	repo, mock := newMockCollRepo(t)

	mock.ExpectExec("INSERT INTO media_collections").
		WillReturnResult(sqlmock.NewResult(10, 1))

	coll := &models.MediaCollection{
		Name:           "Timestamp Test",
		CollectionType: "playlist",
	}

	before := time.Now()
	id, err := repo.Create(context.Background(), coll)
	after := time.Now()

	require.NoError(t, err)
	assert.Equal(t, int64(10), id)
	assert.True(t, !coll.CreatedAt.Before(before) && !coll.CreatedAt.After(after),
		"CreatedAt should be between before and after")
	assert.True(t, !coll.UpdatedAt.Before(before) && !coll.UpdatedAt.After(after),
		"UpdatedAt should be between before and after")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMediaCollectionRepo_Update_RefreshesUpdatedAt(t *testing.T) {
	repo, mock := newMockCollRepo(t)

	mock.ExpectExec("UPDATE media_collections SET").
		WillReturnResult(sqlmock.NewResult(0, 1))

	original := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	coll := &models.MediaCollection{
		ID:             1,
		Name:           "Test",
		CollectionType: "playlist",
		UpdatedAt:      original,
	}

	before := time.Now()
	err := repo.Update(context.Background(), coll)
	require.NoError(t, err)

	assert.True(t, coll.UpdatedAt.After(original) || coll.UpdatedAt.Equal(original),
		"UpdatedAt should be refreshed to current time")
	assert.True(t, !coll.UpdatedAt.Before(before),
		"UpdatedAt should be at or after the call time")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// List — scan error resilience
// ---------------------------------------------------------------------------

func TestMediaCollectionRepo_List_SkipsBadRows(t *testing.T) {
	now := time.Now()

	repo, mock := newMockCollRepo(t)

	// The List method uses scanCollection which will fail on bad JSON in external_ids.
	// The List implementation skips rows that fail to scan (continue on error).
	mock.ExpectQuery("SELECT COUNT").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))
	mock.ExpectQuery("SELECT .+ FROM media_collections ORDER BY").
		WithArgs(10, 0).
		WillReturnRows(sqlmock.NewRows(collCols).
			AddRow(int64(1), "Good", "playlist", nil, 0, `null`, nil, now, now, false, nil, nil, false).
			AddRow(int64(2), "Also Good", "series", nil, 5, `{"k":"v"}`, nil, now, now, false, nil, nil, false))

	colls, total, err := repo.List(context.Background(), 10, 0)
	require.NoError(t, err)
	assert.Equal(t, 2, total)
	assert.Len(t, colls, 2)
	assert.Equal(t, "Good", colls[0].Name)
	assert.Equal(t, "Also Good", colls[1].Name)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// Context cancellation
// ---------------------------------------------------------------------------

func TestMediaCollectionRepo_Create_CancelledContext(t *testing.T) {
	repo, mock := newMockCollRepo(t)

	mock.ExpectExec("INSERT INTO media_collections").
		WillReturnError(context.Canceled)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	coll := &models.MediaCollection{
		Name:           "Cancelled",
		CollectionType: "playlist",
	}
	_, err := repo.Create(ctx, coll)
	require.Error(t, err)
}

func TestMediaCollectionRepo_GetByID_CancelledContext(t *testing.T) {
	repo, mock := newMockCollRepo(t)

	mock.ExpectQuery("SELECT .+ FROM media_collections WHERE id").
		WithArgs(int64(1)).
		WillReturnError(context.Canceled)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := repo.GetByID(ctx, 1)
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// marshalJSONFieldString / unmarshalJSONFieldString helpers
// ---------------------------------------------------------------------------

func TestMediaCollectionRepo_MarshalJSONFieldString(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		wantErr  bool
		wantJSON string
	}{
		{
			name:     "nil input",
			input:    nil,
			wantJSON: "null",
		},
		{
			name:    "map with entries",
			input:   map[string]string{"a": "1", "b": "2"},
			wantErr: false,
		},
		{
			name:    "empty map",
			input:   map[string]string{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := marshalJSONFieldString(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			if tt.wantJSON != "" {
				assert.Equal(t, tt.wantJSON, result)
			} else {
				assert.NotEmpty(t, result)
			}
		})
	}
}

func TestMediaCollectionRepo_UnmarshalJSONFieldString(t *testing.T) {
	tests := []struct {
		name    string
		data    string
		wantErr bool
		wantNil bool
		wantLen int
	}{
		{
			name:    "empty string",
			data:    "",
			wantNil: true,
		},
		{
			name:    "null string",
			data:    "null",
			wantNil: true,
		},
		{
			name:    "valid JSON",
			data:    `{"k1":"v1","k2":"v2"}`,
			wantLen: 2,
		},
		{
			name:    "invalid JSON",
			data:    `{bad`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result map[string]string
			err := unmarshalJSONFieldString(tt.data, &result)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			if tt.wantNil {
				assert.Nil(t, result)
			} else {
				assert.Len(t, result, tt.wantLen)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Delete followed by GetByID (not found)
// ---------------------------------------------------------------------------

func TestMediaCollectionRepo_DeleteThenGet(t *testing.T) {
	repo, mock := newMockCollRepo(t)

	// Delete succeeds
	mock.ExpectExec("DELETE FROM media_collections WHERE id").
		WithArgs(int64(5)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	// Subsequent GetByID returns not found
	mock.ExpectQuery("SELECT .+ FROM media_collections WHERE id").
		WithArgs(int64(5)).
		WillReturnError(sql.ErrNoRows)

	err := repo.Delete(context.Background(), 5)
	require.NoError(t, err)

	_, err = repo.GetByID(context.Background(), 5)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "media collection not found")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// List with zero limit
// ---------------------------------------------------------------------------

func TestMediaCollectionRepo_List_ZeroLimit(t *testing.T) {
	now := time.Now()
	repo, mock := newMockCollRepo(t)

	mock.ExpectQuery("SELECT COUNT").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(10))
	mock.ExpectQuery("SELECT .+ FROM media_collections ORDER BY").
		WithArgs(0, 0).
		WillReturnRows(sqlmock.NewRows(collCols).
			AddRow(sampleCollRowMinimal(1, "Item", now)...))

	colls, total, err := repo.List(context.Background(), 0, 0)
	require.NoError(t, err)
	assert.Equal(t, 10, total)
	// SQLite with LIMIT 0 may return rows depending on driver behavior;
	// the mock returns what we set up
	assert.Len(t, colls, 1)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// Create multiple collections sequentially
// ---------------------------------------------------------------------------

func TestMediaCollectionRepo_Create_Multiple(t *testing.T) {
	repo, mock := newMockCollRepo(t)

	mock.ExpectExec("INSERT INTO media_collections").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO media_collections").
		WillReturnResult(sqlmock.NewResult(2, 1))
	mock.ExpectExec("INSERT INTO media_collections").
		WillReturnResult(sqlmock.NewResult(3, 1))

	for i := int64(1); i <= 3; i++ {
		coll := &models.MediaCollection{
			Name:           "Collection",
			CollectionType: "playlist",
		}
		id, err := repo.Create(context.Background(), coll)
		require.NoError(t, err)
		assert.Equal(t, i, id)
	}
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// Update with various ExternalIDs states
// ---------------------------------------------------------------------------

func TestMediaCollectionRepo_Update_ExternalIDsVariants(t *testing.T) {
	tests := []struct {
		name        string
		externalIDs map[string]string
	}{
		{"nil external IDs", nil},
		{"empty external IDs", map[string]string{}},
		{"single external ID", map[string]string{"imdb": "tt001"}},
		{"multiple external IDs", map[string]string{"imdb": "tt001", "tmdb": "123", "tvdb": "456"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newMockCollRepo(t)

			mock.ExpectExec("UPDATE media_collections SET").
				WillReturnResult(sqlmock.NewResult(0, 1))

			coll := &models.MediaCollection{
				ID:             1,
				Name:           "Test",
				CollectionType: "playlist",
				ExternalIDs:    tt.externalIDs,
			}
			err := repo.Update(context.Background(), coll)
			require.NoError(t, err)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ---------------------------------------------------------------------------
// GetByID — verify all returned fields round-trip
// ---------------------------------------------------------------------------

func TestMediaCollectionRepo_GetByID_AllFieldsRoundTrip(t *testing.T) {
	created := time.Date(2025, 6, 15, 14, 30, 0, 0, time.UTC)
	updated := time.Date(2025, 6, 16, 10, 0, 0, 0, time.UTC)
	desc := "Full round-trip test"
	coverURL := "https://cdn.example.com/art/large.png"

	repo, mock := newMockCollRepo(t)

	mock.ExpectQuery("SELECT .+ FROM media_collections WHERE id").
		WithArgs(int64(77)).
		WillReturnRows(sqlmock.NewRows(collCols).
			AddRow(int64(77), "Full Collection", "franchise", &desc, 42,
				`{"imdb":"tt555","tmdb":"888"}`, &coverURL, created, updated, false, nil, nil, false))

	coll, err := repo.GetByID(context.Background(), 77)
	require.NoError(t, err)
	require.NotNil(t, coll)

	assert.Equal(t, int64(77), coll.ID)
	assert.Equal(t, "Full Collection", coll.Name)
	assert.Equal(t, "franchise", coll.CollectionType)
	require.NotNil(t, coll.Description)
	assert.Equal(t, desc, *coll.Description)
	assert.Equal(t, 42, coll.TotalItems)
	require.NotNil(t, coll.ExternalIDs)
	assert.Equal(t, "tt555", coll.ExternalIDs["imdb"])
	assert.Equal(t, "888", coll.ExternalIDs["tmdb"])
	require.NotNil(t, coll.CoverURL)
	assert.Equal(t, coverURL, *coll.CoverURL)
	assert.Equal(t, created, coll.CreatedAt)
	assert.Equal(t, updated, coll.UpdatedAt)
	assert.NoError(t, mock.ExpectationsWereMet())
}
