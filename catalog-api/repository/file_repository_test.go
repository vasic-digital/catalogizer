package repository

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"testing"
	"time"

	"catalogizer/database"
	"catalogizer/models"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newMockFileRepo creates a FileRepository backed by sqlmock.
// database.DB embeds *sql.DB, so we populate it directly.
func newMockFileRepo(t *testing.T) (*FileRepository, sqlmock.Sqlmock) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	db := database.WrapDB(sqlDB, database.DialectSQLite)
	return NewFileRepository(db), mock
}

// fileColumns is the standard column set returned by file queries.
var fileColumns = []string{
	"id", "storage_root_id", "storage_root_name", "path", "name", "extension",
	"mime_type", "file_type", "size", "is_directory", "created_at", "modified_at",
	"accessed_at", "deleted", "deleted_at", "last_scan_at", "last_verified_at",
	"md5", "sha256", "sha1", "blake3", "quick_hash", "is_duplicate",
	"duplicate_group_id", "parent_id",
}

var metadataColumns = []string{"id", "file_id", "key", "value", "data_type"}

func sampleFileRow(now time.Time) []driver.Value {
	ext := ".txt"
	mime := "text/plain"
	ft := "text"
	return []driver.Value{
		int64(1), int64(10), "my-root", "/docs/readme.txt", "readme.txt", &ext,
		&mime, &ft, int64(1024), false, now, now,
		nil, false, nil, now, nil,
		nil, nil, nil, nil, nil, false,
		nil, nil,
	}
}

// ---------------------------------------------------------------------------
// GetFileByID
// ---------------------------------------------------------------------------

func TestFileRepository_GetFileByID(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name    string
		id      int64
		setup   func(mock sqlmock.Sqlmock)
		wantErr bool
		errMsg  string
		check   func(t *testing.T, f *models.FileWithMetadata)
	}{
		{
			name: "success with metadata",
			id:   1,
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT .+ FROM files f").
					WithArgs(int64(1)).
					WillReturnRows(sqlmock.NewRows(fileColumns).AddRow(sampleFileRow(now)...))
				mock.ExpectQuery("SELECT .+ FROM file_metadata").
					WithArgs(int64(1)).
					WillReturnRows(sqlmock.NewRows(metadataColumns).
						AddRow(1, 1, "author", "John", "string"))
			},
			check: func(t *testing.T, f *models.FileWithMetadata) {
				assert.Equal(t, int64(1), f.ID)
				assert.Equal(t, "readme.txt", f.Name)
				assert.Equal(t, int64(1024), f.Size)
				require.Len(t, f.Metadata, 1)
				assert.Equal(t, "author", f.Metadata[0].Key)
			},
		},
		{
			name: "success without metadata",
			id:   2,
			setup: func(mock sqlmock.Sqlmock) {
				row := sampleFileRow(now)
				row[0] = int64(2) // change ID
				mock.ExpectQuery("SELECT .+ FROM files f").
					WithArgs(int64(2)).
					WillReturnRows(sqlmock.NewRows(fileColumns).AddRow(row...))
				mock.ExpectQuery("SELECT .+ FROM file_metadata").
					WithArgs(int64(2)).
					WillReturnRows(sqlmock.NewRows(metadataColumns))
			},
			check: func(t *testing.T, f *models.FileWithMetadata) {
				assert.Equal(t, int64(2), f.ID)
				assert.Empty(t, f.Metadata)
			},
		},
		{
			name: "not found",
			id:   999,
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT .+ FROM files f").
					WithArgs(int64(999)).
					WillReturnError(sql.ErrNoRows)
			},
			wantErr: true,
			errMsg:  "file not found",
		},
		{
			name: "database error on file query",
			id:   1,
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT .+ FROM files f").
					WithArgs(int64(1)).
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: true,
			errMsg:  "failed to get file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newMockFileRepo(t)
			tt.setup(mock)

			f, err := repo.GetFileByID(context.Background(), tt.id)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
				return
			}
			require.NoError(t, err)
			require.NotNil(t, f)
			tt.check(t, f)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ---------------------------------------------------------------------------
// GetDirectoryContents (maps to GetFilesByDirectory)
// ---------------------------------------------------------------------------

func TestFileRepository_GetDirectoryContents(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name    string
		root    string
		path    string
		page    models.PaginationOptions
		sort    models.SortOptions
		setup   func(mock sqlmock.Sqlmock)
		wantErr bool
		check   func(t *testing.T, r *models.SearchResult)
	}{
		{
			name: "root directory listing",
			root: "my-root",
			path: "/",
			page: models.PaginationOptions{Page: 1, Limit: 10},
			sort: models.SortOptions{Field: "name", Order: "asc"},
			setup: func(mock sqlmock.Sqlmock) {
				// Count query
				mock.ExpectQuery("SELECT COUNT").
					WithArgs("my-root").
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))
				// Data query
				row2 := sampleFileRow(now)
				row2[0] = int64(2)
				row2[4] = "notes.txt"
				mock.ExpectQuery("SELECT .+ FROM files f").
					WithArgs("my-root", 10, 0).
					WillReturnRows(sqlmock.NewRows(fileColumns).
						AddRow(sampleFileRow(now)...).
						AddRow(row2...))
			},
			check: func(t *testing.T, r *models.SearchResult) {
				assert.Equal(t, int64(2), r.TotalCount)
				assert.Len(t, r.Files, 2)
				assert.Equal(t, 1, r.Page)
				assert.Equal(t, 10, r.Limit)
			},
		},
		{
			name: "subdirectory listing",
			root: "my-root",
			path: "/docs",
			page: models.PaginationOptions{Page: 1, Limit: 10},
			sort: models.SortOptions{Field: "name", Order: "asc"},
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT COUNT").
					WithArgs("my-root", "/docs/%", "/docs/%/%").
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
				mock.ExpectQuery("SELECT .+ FROM files f").
					WithArgs("my-root", "/docs/%", "/docs/%/%", 10, 0).
					WillReturnRows(sqlmock.NewRows(fileColumns).
						AddRow(sampleFileRow(now)...))
			},
			check: func(t *testing.T, r *models.SearchResult) {
				assert.Equal(t, int64(1), r.TotalCount)
				assert.Len(t, r.Files, 1)
			},
		},
		{
			name: "empty directory",
			root: "my-root",
			path: "/empty",
			page: models.PaginationOptions{Page: 1, Limit: 10},
			sort: models.SortOptions{Field: "name", Order: "asc"},
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT COUNT").
					WithArgs("my-root", "/empty/%", "/empty/%/%").
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
				mock.ExpectQuery("SELECT .+ FROM files f").
					WithArgs("my-root", "/empty/%", "/empty/%/%", 10, 0).
					WillReturnRows(sqlmock.NewRows(fileColumns))
			},
			check: func(t *testing.T, r *models.SearchResult) {
				assert.Equal(t, int64(0), r.TotalCount)
				assert.Empty(t, r.Files)
			},
		},
		{
			name: "count query error",
			root: "my-root",
			path: "/",
			page: models.PaginationOptions{Page: 1, Limit: 10},
			sort: models.SortOptions{Field: "name", Order: "asc"},
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT COUNT").
					WithArgs("my-root").
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newMockFileRepo(t)
			tt.setup(mock)

			result, err := repo.GetDirectoryContents(context.Background(), tt.root, tt.path, tt.page, tt.sort)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, result)
			tt.check(t, result)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ---------------------------------------------------------------------------
// SearchFiles
// ---------------------------------------------------------------------------

func TestFileRepository_SearchFiles(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name    string
		filter  models.SearchFilter
		page    models.PaginationOptions
		sort    models.SortOptions
		setup   func(mock sqlmock.Sqlmock)
		wantErr bool
		check   func(t *testing.T, r *models.SearchResult)
	}{
		{
			name:   "search by query string",
			filter: models.SearchFilter{Query: "readme"},
			page:   models.PaginationOptions{Page: 1, Limit: 10},
			sort:   models.SortOptions{Field: "name", Order: "asc"},
			setup: func(mock sqlmock.Sqlmock) {
				// Count query - filter adds: AND f.deleted = FALSE AND (f.name LIKE ? OR f.path LIKE ?) AND f.is_directory = FALSE
				mock.ExpectQuery("SELECT COUNT").
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
				// Data query
				mock.ExpectQuery("SELECT .+ FROM files f").
					WillReturnRows(sqlmock.NewRows(fileColumns).
						AddRow(sampleFileRow(now)...))
				// Metadata query for file id=1
				mock.ExpectQuery("SELECT .+ FROM file_metadata").
					WithArgs(int64(1)).
					WillReturnRows(sqlmock.NewRows(metadataColumns))
			},
			check: func(t *testing.T, r *models.SearchResult) {
				assert.Equal(t, int64(1), r.TotalCount)
				assert.Len(t, r.Files, 1)
				assert.Equal(t, "readme.txt", r.Files[0].Name)
			},
		},
		{
			name:   "search by extension",
			filter: models.SearchFilter{Extension: ".txt"},
			page:   models.PaginationOptions{Page: 1, Limit: 10},
			sort:   models.SortOptions{Field: "size", Order: "desc"},
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT COUNT").
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
				mock.ExpectQuery("SELECT .+ FROM files f").
					WillReturnRows(sqlmock.NewRows(fileColumns))
			},
			check: func(t *testing.T, r *models.SearchResult) {
				assert.Equal(t, int64(0), r.TotalCount)
				assert.Empty(t, r.Files)
			},
		},
		{
			name:   "count error",
			filter: models.SearchFilter{},
			page:   models.PaginationOptions{Page: 1, Limit: 10},
			sort:   models.SortOptions{Field: "name", Order: "asc"},
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT COUNT").
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newMockFileRepo(t)
			tt.setup(mock)

			result, err := repo.SearchFiles(context.Background(), tt.filter, tt.page, tt.sort)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, result)
			tt.check(t, result)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ---------------------------------------------------------------------------
// MarkFileAsDeleted (soft delete)
// ---------------------------------------------------------------------------

func TestFileRepository_MarkFileAsDeleted(t *testing.T) {
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
				mock.ExpectExec("UPDATE files").
					WithArgs(int64(1)).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
		},
		{
			name: "database error",
			id:   1,
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("UPDATE files").
					WithArgs(int64(1)).
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newMockFileRepo(t)
			tt.setup(mock)

			err := repo.MarkFileAsDeleted(context.Background(), tt.id)
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
// RestoreDeletedFile
// ---------------------------------------------------------------------------

func TestFileRepository_RestoreDeletedFile(t *testing.T) {
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
				mock.ExpectExec("UPDATE files").
					WithArgs(int64(1)).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
		},
		{
			name: "database error",
			id:   1,
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("UPDATE files").
					WithArgs(int64(1)).
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newMockFileRepo(t)
			tt.setup(mock)

			err := repo.RestoreDeletedFile(context.Background(), tt.id)
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
// UpdateFileMetadata
// ---------------------------------------------------------------------------

func TestFileRepository_UpdateFileMetadata(t *testing.T) {
	hash := "abc123"

	tests := []struct {
		name    string
		fileID  int64
		size    int64
		hash    *string
		setup   func(mock sqlmock.Sqlmock)
		wantErr bool
	}{
		{
			name:   "success with hash",
			fileID: 1,
			size:   2048,
			hash:   &hash,
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("UPDATE files").
					WithArgs(int64(2048), &hash, int64(1)).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
		},
		{
			name:   "success without hash",
			fileID: 1,
			size:   512,
			hash:   nil,
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("UPDATE files").
					WithArgs(int64(512), nil, int64(1)).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newMockFileRepo(t)
			tt.setup(mock)

			err := repo.UpdateFileMetadata(context.Background(), tt.fileID, tt.size, tt.hash)
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
// GetFileByPathAndStorage
// ---------------------------------------------------------------------------

func TestFileRepository_GetFileByPathAndStorage(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name    string
		path    string
		root    string
		setup   func(mock sqlmock.Sqlmock)
		wantNil bool
		wantErr bool
		check   func(t *testing.T, f *models.File)
	}{
		{
			name: "found",
			path: "/docs/readme.txt",
			root: "my-root",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT .+ FROM files f").
					WithArgs("/docs/readme.txt", "my-root").
					WillReturnRows(sqlmock.NewRows(fileColumns).
						AddRow(sampleFileRow(now)...))
			},
			check: func(t *testing.T, f *models.File) {
				assert.Equal(t, "readme.txt", f.Name)
			},
		},
		{
			name: "not found returns nil",
			path: "/missing.txt",
			root: "my-root",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT .+ FROM files f").
					WithArgs("/missing.txt", "my-root").
					WillReturnError(sql.ErrNoRows)
			},
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newMockFileRepo(t)
			tt.setup(mock)

			f, err := repo.GetFileByPathAndStorage(context.Background(), tt.path, tt.root)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			if tt.wantNil {
				assert.Nil(t, f)
			} else {
				require.NotNil(t, f)
				tt.check(t, f)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ---------------------------------------------------------------------------
// GetStorageRoots
// ---------------------------------------------------------------------------

func TestFileRepository_GetStorageRoots(t *testing.T) {
	now := time.Now()

	storageRootCols := []string{
		"id", "name", "protocol", "host", "port", "path", "username", "password", "domain",
		"mount_point", "options", "url", "enabled", "max_depth",
		"enable_duplicate_detection", "enable_metadata_extraction", "include_patterns",
		"exclude_patterns", "created_at", "updated_at", "last_scan_at", "approved_class",
	}

	tests := []struct {
		name    string
		setup   func(mock sqlmock.Sqlmock)
		want    int
		wantErr bool
	}{
		{
			name: "returns roots",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT .+ FROM storage_roots").
					WillReturnRows(sqlmock.NewRows(storageRootCols).
						AddRow(1, "root1", "smb", "host1", 445, "/share", "user", "pass", nil,
							nil, nil, nil, true, 10, true, true, nil, nil, now, now, nil, "private"))
			},
			want: 1,
		},
		{
			name: "empty",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT .+ FROM storage_roots").
					WillReturnRows(sqlmock.NewRows(storageRootCols))
			},
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newMockFileRepo(t)
			tt.setup(mock)

			roots, err := repo.GetStorageRoots(context.Background())
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Len(t, roots, tt.want)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
