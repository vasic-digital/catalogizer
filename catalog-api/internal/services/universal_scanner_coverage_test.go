package services

import (
	"catalogizer/database"
	"catalogizer/filesystem"
	"catalogizer/models"
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/mutecomm/go-sqlcipher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// =============================================================================
// classifyFileType — exhaustive coverage
// =============================================================================

func TestClassifyFileType_Video(t *testing.T) {
	for _, ext := range []string{"mp4", "mkv", "avi", "mov", "wmv", "flv", "m4v", "ts", "webm", "mpg", "mpeg"} {
		assert.Equal(t, "video", classifyFileType(ext), ext)
	}
}

func TestClassifyFileType_Audio(t *testing.T) {
	for _, ext := range []string{"mp3", "flac", "wav", "m4a", "aac", "ogg", "wma", "ape", "opus"} {
		assert.Equal(t, "audio", classifyFileType(ext), ext)
	}
}

func TestClassifyFileType_Image(t *testing.T) {
	for _, ext := range []string{"jpg", "jpeg", "png", "gif", "bmp", "webp", "svg", "tiff", "ico"} {
		assert.Equal(t, "image", classifyFileType(ext), ext)
	}
}

func TestClassifyFileType_Book(t *testing.T) {
	for _, ext := range []string{"pdf", "epub", "mobi", "djvu", "cbr", "cbz", "cb7", "cbt"} {
		assert.Equal(t, "book", classifyFileType(ext), ext)
	}
}

func TestClassifyFileType_Software(t *testing.T) {
	for _, ext := range []string{"exe", "msi", "dmg", "pkg", "iso", "img", "deb", "rpm", "apk", "appimage"} {
		assert.Equal(t, "software", classifyFileType(ext), ext)
	}
}

func TestClassifyFileType_Archive(t *testing.T) {
	for _, ext := range []string{"zip", "rar", "7z", "tar", "gz", "bz2", "xz", "zst"} {
		assert.Equal(t, "archive", classifyFileType(ext), ext)
	}
}

func TestClassifyFileType_Document(t *testing.T) {
	for _, ext := range []string{"txt", "md", "doc", "docx", "rtf", "odt", "csv", "xls", "xlsx"} {
		assert.Equal(t, "document", classifyFileType(ext), ext)
	}
}

func TestClassifyFileType_Code(t *testing.T) {
	for _, ext := range []string{"html", "htm", "css", "js", "go", "py", "java", "c", "cpp", "rs", "sh"} {
		assert.Equal(t, "code", classifyFileType(ext), ext)
	}
}

func TestClassifyFileType_Other(t *testing.T) {
	for _, ext := range []string{"xyz", "", "dat", "bin", "nfo"} {
		assert.Equal(t, "other", classifyFileType(ext), ext)
	}
}

func TestClassifyFileType_CaseInsensitive(t *testing.T) {
	assert.Equal(t, "video", classifyFileType("MP4"))
	assert.Equal(t, "audio", classifyFileType("FLAC"))
	assert.Equal(t, "image", classifyFileType("PNG"))
	assert.Equal(t, "book", classifyFileType("PDF"))
}

// =============================================================================
// storageRootToSettings — all protocols
// =============================================================================

func TestStorageRootToSettings_Local(t *testing.T) {
	scanner := &UniversalScanner{logger: zap.NewNop()}
	path := "/data/media"
	root := &models.StorageRoot{Protocol: "local", Path: &path}
	settings := scanner.storageRootToSettings(root)
	assert.Equal(t, "/data/media", settings["base_path"])
}

func TestStorageRootToSettings_SMB_AllFields(t *testing.T) {
	scanner := &UniversalScanner{logger: zap.NewNop()}
	host := "nas.local"
	port := 445
	path := "media"
	user := "admin"
	pass := "secret"
	domain := "WORKGROUP"
	root := &models.StorageRoot{
		Protocol: "smb",
		Host:     &host, Port: &port, Path: &path,
		Username: &user, Password: &pass, Domain: &domain,
	}
	settings := scanner.storageRootToSettings(root)
	assert.Equal(t, "nas.local", settings["host"])
	assert.Equal(t, 445, settings["port"])
	assert.Equal(t, "media", settings["share"])
	assert.Equal(t, "admin", settings["username"])
	assert.Equal(t, "secret", settings["password"])
	assert.Equal(t, "WORKGROUP", settings["domain"])
}

func TestStorageRootToSettings_FTP(t *testing.T) {
	scanner := &UniversalScanner{logger: zap.NewNop()}
	host := "ftp.example.com"
	port := 21
	user := "ftpuser"
	pass := "ftppass"
	root := &models.StorageRoot{
		Protocol: "ftp",
		Host:     &host, Port: &port, Username: &user, Password: &pass,
	}
	settings := scanner.storageRootToSettings(root)
	assert.Equal(t, "ftp.example.com", settings["host"])
	assert.Equal(t, 21, settings["port"])
	assert.Equal(t, "ftpuser", settings["username"])
	assert.Equal(t, "ftppass", settings["password"])
}

func TestStorageRootToSettings_NFS(t *testing.T) {
	scanner := &UniversalScanner{logger: zap.NewNop()}
	host := "nfs.local"
	path := "/exports/media"
	mount := "/mnt/nfs"
	opts := "rw,sync"
	root := &models.StorageRoot{
		Protocol: "nfs",
		Host:     &host, Path: &path, MountPoint: &mount, Options: &opts,
	}
	settings := scanner.storageRootToSettings(root)
	assert.Equal(t, "nfs.local", settings["host"])
	assert.Equal(t, "/exports/media", settings["path"])
	assert.Equal(t, "/mnt/nfs", settings["mount_point"])
	assert.Equal(t, "rw,sync", settings["options"])
}

func TestStorageRootToSettings_WebDAV(t *testing.T) {
	scanner := &UniversalScanner{logger: zap.NewNop()}
	url := "https://webdav.example.com/files"
	user := "davuser"
	pass := "davpass"
	root := &models.StorageRoot{
		Protocol: "webdav",
		URL:      &url, Username: &user, Password: &pass,
	}
	settings := scanner.storageRootToSettings(root)
	assert.Equal(t, "https://webdav.example.com/files", settings["url"])
	assert.Equal(t, "davuser", settings["username"])
	assert.Equal(t, "davpass", settings["password"])
}

func TestStorageRootToSettings_NilFields(t *testing.T) {
	scanner := &UniversalScanner{logger: zap.NewNop()}
	for _, proto := range []string{"local", "smb", "ftp", "nfs", "webdav", "unknown"} {
		root := &models.StorageRoot{Protocol: proto}
		settings := scanner.storageRootToSettings(root)
		assert.Empty(t, settings, proto)
	}
}

// =============================================================================
// SetAggregationService
// =============================================================================

func TestSetAggregationService(t *testing.T) {
	scanner := &UniversalScanner{logger: zap.NewNop()}
	assert.Nil(t, scanner.aggregationService)
	agg := &AggregationService{}
	scanner.SetAggregationService(agg)
	assert.NotNil(t, scanner.aggregationService)
}

// =============================================================================
// insertFileRecord — nil DB path
// =============================================================================

func TestInsertFileRecord_NilDB_IncrementCounters(t *testing.T) {
	logger := zap.NewNop()
	status := &ScanStatus{}
	job := ScanJob{StorageRoot: &models.StorageRoot{Name: "test"}}
	fi := &filesystem.FileInfo{Name: "file.mp4", Size: 1024}

	err := insertFileRecord(context.Background(), nil, "/test/file.mp4", fi, job, status, logger)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), status.FilesProcessed)
	assert.Equal(t, int64(1), status.FilesFound)
}

// =============================================================================
// ensureDirectoryPathExists
// =============================================================================

func TestEnsureDirectoryPathExists_EmptyPaths(t *testing.T) {
	logger := zap.NewNop()
	for _, p := range []string{"", ".", "/"} {
		parentID, err := ensureDirectoryPathExists(context.Background(), nil, 1, p, logger)
		assert.NoError(t, err)
		assert.Nil(t, parentID)
	}
}

// §11.4.120 reconciliation of the former _RootParent test (FINDING-3 fix):
// under the fix a single-component path (e.g. "The Matrix (1999)") is a REAL
// top-level directory that MUST be resolved so files inside it get parent_id
// set — it is no longer a no-parent short-circuit. The genuine no-parent cases
// ("", ".", "/") are covered by TestEnsureDirectoryPathExists_EmptyPaths.
// This is also the unit-level regression guard for FINDING-3 (§11.4.135):
// pre-fix this returned nil (parent_id NULL) -> aggregation produced 0 entities.
func TestEnsureDirectoryPathExists_SingleComponentDir(t *testing.T) {
	logger := zap.NewNop()
	sqlDB, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	defer sqlDB.Close()

	db := database.WrapDB(sqlDB, database.DialectSQLite)
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS files (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			storage_root_id INTEGER NOT NULL,
			name TEXT NOT NULL,
			path TEXT NOT NULL,
			extension TEXT DEFAULT '',
			mime_type TEXT DEFAULT '',
			file_type TEXT DEFAULT 'other',
			size INTEGER DEFAULT 0,
			is_directory BOOLEAN DEFAULT 0,
			modified_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			last_scan_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			parent_id INTEGER,
			deleted BOOLEAN DEFAULT 0,
			UNIQUE(storage_root_id, path)
		)
	`)
	require.NoError(t, err)

	parentID, err := ensureDirectoryPathExists(context.Background(), db, 1, "The Matrix (1999)", logger)
	assert.NoError(t, err)
	require.NotNil(t, parentID) // FINDING-3: single-component dir MUST resolve (was nil pre-fix)

	// The directory was actually created and is queryable by the returned id.
	var path string
	err = db.QueryRow("SELECT path FROM files WHERE id = ? AND is_directory = 1", *parentID).Scan(&path)
	require.NoError(t, err)
	assert.Equal(t, "The Matrix (1999)", path)

	// Idempotent: a second call returns the same directory id.
	parentID2, err := ensureDirectoryPathExists(context.Background(), db, 1, "The Matrix (1999)", logger)
	assert.NoError(t, err)
	require.NotNil(t, parentID2)
	assert.Equal(t, *parentID, *parentID2)
}

func TestEnsureDirectoryPathExists_CreatesHierarchy(t *testing.T) {
	logger := zap.NewNop()
	sqlDB, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	defer sqlDB.Close()

	db := database.WrapDB(sqlDB, database.DialectSQLite)
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS files (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			storage_root_id INTEGER NOT NULL,
			name TEXT NOT NULL,
			path TEXT NOT NULL,
			extension TEXT DEFAULT '',
			mime_type TEXT DEFAULT '',
			file_type TEXT DEFAULT 'other',
			size INTEGER DEFAULT 0,
			is_directory BOOLEAN DEFAULT 0,
			modified_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			last_scan_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			parent_id INTEGER,
			deleted BOOLEAN DEFAULT 0,
			UNIQUE(storage_root_id, path)
		)
	`)
	require.NoError(t, err)

	// ensureDirectoryPathExists receives a directory path (the caller uses filepath.Dir),
	// so pass the parent directory, not the full file path.
	parentID, err := ensureDirectoryPathExists(context.Background(), db, 1, "media/movies/2024", logger)
	assert.NoError(t, err)
	assert.NotNil(t, parentID)

	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM files WHERE is_directory = 1").Scan(&count)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, count, 3) // media, movies, 2024

	// Idempotent: calling again with same directory returns the same parent ID
	parentID2, err := ensureDirectoryPathExists(context.Background(), db, 1, "media/movies/2024", logger)
	assert.NoError(t, err)
	assert.NotNil(t, parentID2)
	assert.Equal(t, *parentID, *parentID2)
}

// =============================================================================
// insertFileRecord — with real DB
// =============================================================================

func TestInsertFileRecord_WithSQLiteDB(t *testing.T) {
	logger := zap.NewNop()
	sqlDB, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	defer sqlDB.Close()

	db := database.WrapDB(sqlDB, database.DialectSQLite)
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS storage_roots (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE,
			protocol TEXT,
			host TEXT, port INTEGER, path TEXT, username TEXT, password TEXT, domain TEXT,
			enabled BOOLEAN DEFAULT 1, max_depth INTEGER DEFAULT 10
		);
		CREATE TABLE IF NOT EXISTS files (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			storage_root_id INTEGER NOT NULL,
			name TEXT NOT NULL,
			path TEXT NOT NULL,
			extension TEXT DEFAULT '',
			mime_type TEXT DEFAULT '',
			file_type TEXT DEFAULT 'other',
			size INTEGER DEFAULT 0,
			is_directory BOOLEAN DEFAULT 0,
			modified_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			last_scan_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			parent_id INTEGER,
			deleted BOOLEAN DEFAULT 0,
			deleted_at DATETIME,
			UNIQUE(storage_root_id, path)
		)
	`)
	require.NoError(t, err)

	status := &ScanStatus{}
	job := ScanJob{
		StorageRoot: &models.StorageRoot{
			Name: "test-root", Protocol: "local", MaxDepth: 10,
		},
	}

	fi := &filesystem.FileInfo{
		Name: "movie.mp4", Size: 1024000, ModTime: time.Now(), IsDir: false,
	}

	err = insertFileRecord(context.Background(), db, "movie.mp4", fi, job, status, logger)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), status.FilesProcessed)

	var name string
	err = db.QueryRow("SELECT name FROM files WHERE path = ?", "movie.mp4").Scan(&name)
	assert.NoError(t, err)
	assert.Equal(t, "movie.mp4", name)
}

func TestInsertFileRecord_CreatesStorageRoot(t *testing.T) {
	logger := zap.NewNop()
	sqlDB, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	defer sqlDB.Close()

	db := database.WrapDB(sqlDB, database.DialectSQLite)
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS storage_roots (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE,
			protocol TEXT,
			host TEXT, port INTEGER, path TEXT, username TEXT, password TEXT, domain TEXT,
			enabled BOOLEAN DEFAULT 1, max_depth INTEGER DEFAULT 10
		);
		CREATE TABLE IF NOT EXISTS files (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			storage_root_id INTEGER NOT NULL,
			name TEXT NOT NULL,
			path TEXT NOT NULL,
			extension TEXT DEFAULT '',
			mime_type TEXT DEFAULT '',
			file_type TEXT DEFAULT 'other',
			size INTEGER DEFAULT 0,
			is_directory BOOLEAN DEFAULT 0,
			modified_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			last_scan_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			parent_id INTEGER,
			deleted BOOLEAN DEFAULT 0,
			deleted_at DATETIME,
			UNIQUE(storage_root_id, path)
		)
	`)
	require.NoError(t, err)

	status := &ScanStatus{}
	host := "nas.local"
	job := ScanJob{
		StorageRoot: &models.StorageRoot{
			Name: "new-root", Protocol: "smb", Host: &host, MaxDepth: 5,
		},
	}
	fi := &filesystem.FileInfo{Name: "test.txt", Size: 100, ModTime: time.Now()}

	err = insertFileRecord(context.Background(), db, "test.txt", fi, job, status, logger)
	assert.NoError(t, err)

	// Verify storage root was created
	var rootName string
	err = db.QueryRow("SELECT name FROM storage_roots WHERE name = ?", "new-root").Scan(&rootName)
	assert.NoError(t, err)
	assert.Equal(t, "new-root", rootName)
}

func TestInsertFileRecord_DirectoryWithParent(t *testing.T) {
	logger := zap.NewNop()
	sqlDB, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	defer sqlDB.Close()

	db := database.WrapDB(sqlDB, database.DialectSQLite)
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS storage_roots (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE,
			protocol TEXT,
			host TEXT, port INTEGER, path TEXT, username TEXT, password TEXT, domain TEXT,
			enabled BOOLEAN DEFAULT 1, max_depth INTEGER DEFAULT 10
		);
		CREATE TABLE IF NOT EXISTS files (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			storage_root_id INTEGER NOT NULL,
			name TEXT NOT NULL,
			path TEXT NOT NULL,
			extension TEXT DEFAULT '',
			mime_type TEXT DEFAULT '',
			file_type TEXT DEFAULT 'other',
			size INTEGER DEFAULT 0,
			is_directory BOOLEAN DEFAULT 0,
			modified_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			last_scan_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			parent_id INTEGER,
			deleted BOOLEAN DEFAULT 0,
			deleted_at DATETIME,
			UNIQUE(storage_root_id, path)
		)
	`)
	require.NoError(t, err)

	_, err = db.Exec(`INSERT INTO storage_roots (name, protocol) VALUES ('root', 'local')`)
	require.NoError(t, err)

	status := &ScanStatus{}
	job := ScanJob{
		StorageRoot: &models.StorageRoot{Name: "root", Protocol: "local", MaxDepth: 10},
	}

	// Insert a file nested in a directory
	fi := &filesystem.FileInfo{Name: "file.mp4", Size: 500, ModTime: time.Now(), IsDir: false}
	err = insertFileRecord(context.Background(), db, "media/movies/file.mp4", fi, job, status, logger)
	assert.NoError(t, err)

	// Verify parent directories were created
	var dirCount int
	err = db.QueryRow("SELECT COUNT(*) FROM files WHERE is_directory = 1").Scan(&dirCount)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, dirCount, 2) // media, movies
}

func TestInsertFileRecord_ZeroModTime(t *testing.T) {
	logger := zap.NewNop()
	sqlDB, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	defer sqlDB.Close()

	db := database.WrapDB(sqlDB, database.DialectSQLite)
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS storage_roots (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE, protocol TEXT,
			host TEXT, port INTEGER, path TEXT, username TEXT, password TEXT, domain TEXT,
			enabled BOOLEAN DEFAULT 1, max_depth INTEGER DEFAULT 10
		);
		CREATE TABLE IF NOT EXISTS files (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			storage_root_id INTEGER NOT NULL, name TEXT NOT NULL, path TEXT NOT NULL,
			extension TEXT DEFAULT '', mime_type TEXT DEFAULT '', file_type TEXT DEFAULT 'other',
			size INTEGER DEFAULT 0, is_directory BOOLEAN DEFAULT 0,
			modified_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			last_scan_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			parent_id INTEGER, deleted BOOLEAN DEFAULT 0, deleted_at DATETIME,
			UNIQUE(storage_root_id, path)
		)
	`)
	require.NoError(t, err)

	status := &ScanStatus{}
	job := ScanJob{StorageRoot: &models.StorageRoot{Name: "r", Protocol: "local", MaxDepth: 5}}
	fi := &filesystem.FileInfo{Name: "zero.txt", Size: 10}

	err = insertFileRecord(context.Background(), db, "zero.txt", fi, job, status, logger)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), status.FilesProcessed)
}
