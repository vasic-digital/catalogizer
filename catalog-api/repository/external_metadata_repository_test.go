package repository

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"catalogizer/database"
	"catalogizer/internal/media/models"

	"github.com/DATA-DOG/go-sqlmock"
	_ "github.com/mutecomm/go-sqlcipher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newMockExternalMetadataRepo creates an ExternalMetadataRepository backed by sqlmock.
func newMockExternalMetadataRepo(t *testing.T) (*ExternalMetadataRepository, sqlmock.Sqlmock) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	db := database.WrapDB(sqlDB, database.DialectSQLite)
	return NewExternalMetadataRepository(db), mock
}

// externalMetadataColumns is the standard column set for external_metadata queries.
var externalMetadataColumns = []string{
	"id", "media_item_id", "provider", "external_id", "data", "rating",
	"review_url", "cover_url", "trailer_url", "last_fetched",
}

func sampleExternalMetadataRow(now time.Time) []driver.Value {
	rating := 8.2
	coverURL := "https://image.tmdb.org/cover.jpg"
	return []driver.Value{
		int64(1), int64(10), "tmdb", "12345", `{"title":"Test"}`, &rating,
		nil, &coverURL, nil, now,
	}
}

// ---------------------------------------------------------------------------
// Create
// ---------------------------------------------------------------------------

func TestExternalMetadataRepository_Create(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(mock sqlmock.Sqlmock)
		wantErr bool
		wantID  int64
	}{
		{
			name: "success",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("INSERT INTO external_metadata").
					WillReturnResult(sqlmock.NewResult(5, 1))
			},
			wantID: 5,
		},
		{
			name: "database error",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("INSERT INTO external_metadata").
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newMockExternalMetadataRepo(t)
			tt.setup(mock)

			em := &models.ExternalMetadata{
				MediaItemID: 10,
				Provider:    "tmdb",
				ExternalID:  "12345",
				Data:        `{"title":"Test"}`,
			}
			id, err := repo.Create(context.Background(), em)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantID, id)
			assert.Equal(t, tt.wantID, em.ID)
			assert.False(t, em.LastFetched.IsZero())
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ---------------------------------------------------------------------------
// GetByItem
// ---------------------------------------------------------------------------

func TestExternalMetadataRepository_GetByItem(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name      string
		itemID    int64
		setup     func(mock sqlmock.Sqlmock)
		wantErr   bool
		wantCount int
		check     func(t *testing.T, items []*models.ExternalMetadata)
	}{
		{
			name:   "returns metadata",
			itemID: 10,
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT .+ FROM external_metadata WHERE media_item_id").
					WithArgs(int64(10)).
					WillReturnRows(sqlmock.NewRows(externalMetadataColumns).
						AddRow(sampleExternalMetadataRow(now)...))
			},
			wantCount: 1,
			check: func(t *testing.T, items []*models.ExternalMetadata) {
				assert.Equal(t, "tmdb", items[0].Provider)
				assert.Equal(t, "12345", items[0].ExternalID)
				assert.NotNil(t, items[0].Rating)
				assert.Equal(t, 8.2, *items[0].Rating)
			},
		},
		{
			name:   "empty result",
			itemID: 99,
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT .+ FROM external_metadata WHERE media_item_id").
					WithArgs(int64(99)).
					WillReturnRows(sqlmock.NewRows(externalMetadataColumns))
			},
			wantCount: 0,
		},
		{
			name:   "database error",
			itemID: 10,
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT .+ FROM external_metadata WHERE media_item_id").
					WithArgs(int64(10)).
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newMockExternalMetadataRepo(t)
			tt.setup(mock)

			items, err := repo.GetByItem(context.Background(), tt.itemID)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Len(t, items, tt.wantCount)
			if tt.check != nil {
				tt.check(t, items)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ---------------------------------------------------------------------------
// GetByProvider
// ---------------------------------------------------------------------------

func TestExternalMetadataRepository_GetByProvider(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name       string
		provider   string
		externalID string
		setup      func(mock sqlmock.Sqlmock)
		wantNil    bool
		wantErr    bool
		check      func(t *testing.T, em *models.ExternalMetadata)
	}{
		{
			name:       "found",
			provider:   "tmdb",
			externalID: "12345",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT .+ FROM external_metadata WHERE provider").
					WithArgs("tmdb", "12345").
					WillReturnRows(sqlmock.NewRows(externalMetadataColumns).
						AddRow(sampleExternalMetadataRow(now)...))
			},
			check: func(t *testing.T, em *models.ExternalMetadata) {
				assert.Equal(t, "tmdb", em.Provider)
				assert.Equal(t, "12345", em.ExternalID)
			},
		},
		{
			name:       "not found returns nil",
			provider:   "imdb",
			externalID: "tt0000",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT .+ FROM external_metadata WHERE provider").
					WithArgs("imdb", "tt0000").
					WillReturnError(sql.ErrNoRows)
			},
			wantNil: true,
		},
		{
			name:       "database error",
			provider:   "tmdb",
			externalID: "12345",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT .+ FROM external_metadata WHERE provider").
					WithArgs("tmdb", "12345").
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newMockExternalMetadataRepo(t)
			tt.setup(mock)

			em, err := repo.GetByProvider(context.Background(), tt.provider, tt.externalID)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			if tt.wantNil {
				assert.Nil(t, em)
			} else {
				require.NotNil(t, em)
				tt.check(t, em)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ---------------------------------------------------------------------------
// Delete
// ---------------------------------------------------------------------------

func TestExternalMetadataRepository_Delete(t *testing.T) {
	tests := []struct {
		name    string
		id      int64
		setup   func(mock sqlmock.Sqlmock)
		wantErr bool
	}{
		{
			name: "success",
			id:   1,
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("DELETE FROM external_metadata WHERE id").
					WithArgs(int64(1)).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
		},
		{
			name: "database error",
			id:   1,
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("DELETE FROM external_metadata WHERE id").
					WithArgs(int64(1)).
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newMockExternalMetadataRepo(t)
			tt.setup(mock)

			err := repo.Delete(context.Background(), tt.id)
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
// Upsert (insert path)
// ---------------------------------------------------------------------------

func TestExternalMetadataRepository_Upsert_Insert(t *testing.T) {
	repo, mock := newMockExternalMetadataRepo(t)

	// findByItemAndProvider returns no rows (nothing exists yet)
	mock.ExpectQuery("SELECT .+ FROM external_metadata WHERE media_item_id").
		WithArgs(int64(10), "tmdb").
		WillReturnError(sql.ErrNoRows)

	// Then Create is called
	mock.ExpectExec("INSERT INTO external_metadata").
		WillReturnResult(sqlmock.NewResult(1, 1))

	em := &models.ExternalMetadata{
		MediaItemID: 10,
		Provider:    "tmdb",
		ExternalID:  "12345",
		Data:        `{"title":"Test"}`,
	}
	err := repo.Upsert(context.Background(), em)
	require.NoError(t, err)
	assert.Equal(t, int64(1), em.ID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// Upsert (update path)
// ---------------------------------------------------------------------------

func TestExternalMetadataRepository_Upsert_Update(t *testing.T) {
	now := time.Now()
	repo, mock := newMockExternalMetadataRepo(t)

	// findByItemAndProvider returns existing record
	mock.ExpectQuery("SELECT .+ FROM external_metadata WHERE media_item_id").
		WithArgs(int64(10), "tmdb").
		WillReturnRows(sqlmock.NewRows(externalMetadataColumns).
			AddRow(sampleExternalMetadataRow(now)...))

	// Then UPDATE is called
	mock.ExpectExec("UPDATE external_metadata SET").
		WillReturnResult(sqlmock.NewResult(0, 1))

	em := &models.ExternalMetadata{
		MediaItemID: 10,
		Provider:    "tmdb",
		ExternalID:  "67890",
		Data:        `{"title":"Updated"}`,
	}
	err := repo.Upsert(context.Background(), em)
	require.NoError(t, err)
	assert.Equal(t, int64(1), em.ID) // inherits existing ID
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// Duplicate prevention — real SQLite DB (UNIQUE(media_item_id, provider))
//
// These tests exercise the production defect directly: under concurrent
// enrichment the old read-then-write Upsert produced DUPLICATE
// (media_item_id, provider) rows because nothing in the schema prevented it.
// Migration v20 adds a UNIQUE index and Upsert recovers the losing INSERT as
// an UPDATE.
//
// §11.4.115 polarity switch: by default these run against the FIXED schema
// (unique index present) and assert the defect is ABSENT. Setting
// EXTMETA_RED_MODE=1 drops the unique index to reproduce the pre-fix schema —
// the SAME assertions then FAIL, proving the index is the load-bearing
// guarantee (RED-on-broken). Run:
//
//	EXTMETA_RED_MODE=1 go test ./repository/ \
//	  -run 'ExternalMetadataRepository_(UniqueIndexRejects|Upsert_Concurrent)' -count=1
// ---------------------------------------------------------------------------

const extMetaCreateTableSQL = `CREATE TABLE IF NOT EXISTS external_metadata (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	media_item_id INTEGER NOT NULL,
	provider TEXT NOT NULL,
	external_id TEXT NOT NULL,
	data TEXT,
	rating REAL,
	review_url TEXT,
	cover_url TEXT,
	trailer_url TEXT,
	last_fetched DATETIME DEFAULT CURRENT_TIMESTAMP
);`

const extMetaCreateUniqueIndexSQL = `CREATE UNIQUE INDEX IF NOT EXISTS idx_external_metadata_item_provider
	ON external_metadata (media_item_id, provider);`

// extMetaUniqueIndexEnabled is the §11.4.115 polarity gate. Default (RED_MODE
// unset / not "1") = the fixed schema with the unique index. EXTMETA_RED_MODE=1
// reproduces the pre-fix schema without it.
func extMetaUniqueIndexEnabled() bool {
	return os.Getenv("EXTMETA_RED_MODE") != "1"
}

// createExtMetaSchema builds the external_metadata table and, when the polarity
// gate is on (default/fixed), the UNIQUE(media_item_id, provider) index.
func createExtMetaSchema(t *testing.T, sqlDB *sql.DB) {
	t.Helper()
	_, err := sqlDB.Exec(extMetaCreateTableSQL)
	require.NoError(t, err)
	if extMetaUniqueIndexEnabled() {
		_, err = sqlDB.Exec(extMetaCreateUniqueIndexSQL)
		require.NoError(t, err)
	}
}

// setupExtMetaMemDB returns a single-connection in-memory repo for deterministic
// (non-concurrent) tests.
func setupExtMetaMemDB(t *testing.T) (*ExternalMetadataRepository, *database.DB) {
	t.Helper()
	sqlDB, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	sqlDB.SetMaxOpenConns(1)
	createExtMetaSchema(t, sqlDB)
	db := database.WrapDB(sqlDB, database.DialectSQLite)
	return NewExternalMetadataRepository(db), db
}

// setupExtMetaFileDB returns a multi-connection file-backed repo (WAL +
// busy_timeout) so concurrent goroutines genuinely contend at the SQL layer —
// :memory: with >1 connection would create separate databases.
func setupExtMetaFileDB(t *testing.T, maxConns int) (*ExternalMetadataRepository, *database.DB) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "extmeta.db")
	dsn := path + "?_busy_timeout=5000&_journal_mode=WAL&_synchronous=NORMAL"
	sqlDB, err := sql.Open("sqlite3", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	sqlDB.SetMaxOpenConns(maxConns)
	// The go-sqlcipher driver ignores _journal_mode in the DSN (it only parses
	// _loc, _busy_timeout, _txlock, _foreign_keys and the cipher pragmas), so
	// WAL must be enabled explicitly, exactly as database.NewConnection does.
	// Without this the file runs in rollback-journal mode, where readers block
	// behind a committing writer.
	var mode string
	require.NoError(t, sqlDB.QueryRow("PRAGMA journal_mode=WAL").Scan(&mode))
	require.Equal(t, "wal", mode)
	createExtMetaSchema(t, sqlDB)
	db := database.WrapDB(sqlDB, database.DialectSQLite)
	return NewExternalMetadataRepository(db), db
}

func countExtMetaRows(t *testing.T, db *database.DB, mediaItemID int64, provider string) int {
	t.Helper()
	var n int
	require.NoError(t, db.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM external_metadata WHERE media_item_id = ? AND provider = ?`,
		mediaItemID, provider).Scan(&n))
	return n
}

// TestExternalMetadataRepository_Upsert_SecondCallUpdatesNoDuplicate proves a
// repeated Upsert of the same (media_item_id, provider) results in exactly ONE
// row and the second call UPDATEs (cover_url changes), never inserts a second.
func TestExternalMetadataRepository_Upsert_SecondCallUpdatesNoDuplicate(t *testing.T) {
	repo, db := setupExtMetaMemDB(t)
	ctx := context.Background()

	require.NoError(t, repo.Upsert(ctx, &models.ExternalMetadata{
		MediaItemID: 10, Provider: "tmdb", ExternalID: "111", Data: `{"v":1}`,
		CoverURL: strPtr("cover1.jpg"),
	}))
	require.NoError(t, repo.Upsert(ctx, &models.ExternalMetadata{
		MediaItemID: 10, Provider: "tmdb", ExternalID: "111", Data: `{"v":2}`,
		CoverURL: strPtr("cover2.jpg"),
	}))

	assert.Equal(t, 1, countExtMetaRows(t, db, 10, "tmdb"),
		"two Upserts of the same key must leave exactly one row")

	got, err := repo.GetByItem(ctx, 10)
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.NotNil(t, got[0].CoverURL)
	assert.Equal(t, "cover2.jpg", *got[0].CoverURL,
		"the second Upsert must UPDATE the existing row's cover_url")
}

// TestExternalMetadataRepository_UniqueIndexRejectsDuplicatePair proves the
// UNIQUE index is the real guarantee: a second raw Create() for the same
// (media_item_id, provider) is rejected at the DB layer. RED under
// EXTMETA_RED_MODE=1 (no index → the second Create succeeds → 2 rows).
func TestExternalMetadataRepository_UniqueIndexRejectsDuplicatePair(t *testing.T) {
	repo, db := setupExtMetaMemDB(t)
	ctx := context.Background()

	_, err := repo.Create(ctx, &models.ExternalMetadata{
		MediaItemID: 10, Provider: "tmdb", ExternalID: "a", Data: "{}",
	})
	require.NoError(t, err)

	_, err = repo.Create(ctx, &models.ExternalMetadata{
		MediaItemID: 10, Provider: "tmdb", ExternalID: "b", Data: "{}",
	})
	require.Error(t, err,
		"the unique index must reject a second (media_item_id, provider) row")
	assert.True(t, isUniqueViolation(err),
		"error must be a unique-constraint violation, got: %v", err)
	assert.Equal(t, 1, countExtMetaRows(t, db, 10, "tmdb"))
}

// TestExternalMetadataRepository_Upsert_ConcurrentSameKeyNoDuplicate is the race
// test: N goroutines Upsert the same (media_item_id, provider) simultaneously.
// After the fix exactly ONE row exists and every Upsert returns nil (the losing
// INSERT is recovered as an UPDATE). RED under EXTMETA_RED_MODE=1 (no index →
// concurrent writers produce duplicate rows → count != 1).
func TestExternalMetadataRepository_Upsert_ConcurrentSameKeyNoDuplicate(t *testing.T) {
	repo, db := setupExtMetaFileDB(t, 4)
	ctx := context.Background()

	const goroutines = 10
	var wg sync.WaitGroup
	errs := make([]error, goroutines)
	start := make(chan struct{})

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			cover := fmt.Sprintf("cover-%d.jpg", i)
			<-start // release all goroutines together to maximise contention
			errs[i] = repo.Upsert(ctx, &models.ExternalMetadata{
				MediaItemID: 42, Provider: "tmdb",
				ExternalID: fmt.Sprintf("ext-%d", i), Data: "{}",
				CoverURL: &cover,
			})
		}(i)
	}
	close(start)
	wg.Wait()

	for i, e := range errs {
		require.NoErrorf(t, e,
			"goroutine %d Upsert must not error — a lost INSERT race must be recovered as UPDATE", i)
	}
	assert.Equal(t, 1, countExtMetaRows(t, db, 42, "tmdb"),
		"concurrent Upserts of the same (media_item_id, provider) must converge to exactly one row")
}
