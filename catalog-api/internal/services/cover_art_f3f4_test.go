package services

// NEW1-F3 / NEW1-F4: path confinement and per-item determinism of GenerateMissingVideoThumbnails.
// All tests drive the REAL code path (real ffmpeg/ffprobe, real migrated SQLite schema); nothing is stubbed.
// A test that needs a path relative to the process working directory changes into its own temp dir (t.Chdir).

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"catalogizer/database"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func makeTestVideo(t *testing.T, full string) {
	t.Helper()
	for _, bin := range []string{"ffmpeg", "ffprobe"} {
		if _, err := exec.LookPath(bin); err != nil {
			t.Skipf("%s not installed", bin)
		}
	}
	require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
	out, err := exec.Command("ffmpeg", "-v", "error", "-f", "lavfi", "-i", "testsrc=duration=8:size=160x120:rate=10",
		"-pix_fmt", "yuv420p", "-y", full).CombinedOutput()
	require.NoError(t, err, string(out))
}

// seedRawItem inserts a storage root (protocol, rootPath may be "") plus one movie item whose single file
// has files.path = rel. It creates no video file on disk.
func seedRawItem(t *testing.T, db *database.DB, name, protocol, rootPath, rel string) int64 {
	t.Helper()
	ctx := context.Background()
	_, err := db.ExecContext(ctx, `INSERT INTO storage_roots (name, protocol, path, enabled) VALUES (?, ?, ?, ?)`, name, protocol, rootPath, true)
	require.NoError(t, err)
	var rootID int64
	require.NoError(t, db.QueryRowContext(ctx, `SELECT id FROM storage_roots WHERE name = ?`, name).Scan(&rootID))
	itemID := addFileToItem(t, db, rootID, 0, name, rel, true)
	return itemID
}

// addFileToItem adds a file row + media_files link. itemID 0 creates a new movie item.
func addFileToItem(t *testing.T, db *database.DB, rootID, itemID int64, title, rel string, primary bool) int64 {
	t.Helper()
	ctx := context.Background()
	_, err := db.ExecContext(ctx, `INSERT INTO files (storage_root_id, path, name, size, is_directory, modified_at, deleted) VALUES (?, ?, ?, 1, ?, ?, ?)`,
		rootID, rel, filepath.Base(rel), false, time.Now().UTC(), false)
	require.NoError(t, err)
	var fileID int64
	require.NoError(t, db.QueryRowContext(ctx, `SELECT id FROM files WHERE storage_root_id = ? AND path = ?`, rootID, rel).Scan(&fileID))
	if itemID == 0 {
		var typeID int64
		require.NoError(t, db.QueryRowContext(ctx, `SELECT id FROM media_types WHERE name = 'movie'`).Scan(&typeID))
		_, err = db.ExecContext(ctx, `INSERT INTO media_items (media_type_id, title) VALUES (?, ?)`, typeID, title)
		require.NoError(t, err)
		require.NoError(t, db.QueryRowContext(ctx, `SELECT id FROM media_items WHERE title = ?`, title).Scan(&itemID))
	}
	_, err = db.ExecContext(ctx, `INSERT INTO media_files (media_item_id, file_id, is_primary) VALUES (?, ?, ?)`, itemID, fileID, primary)
	require.NoError(t, err)
	return itemID
}

func newThumbSvc(t *testing.T, db *database.DB) *CoverArtService {
	svc := NewCoverArtService(db, zap.NewNop())
	svc.cacheDir = filepath.Join(t.TempDir(), "cache")
	return svc
}

func coverRows(t *testing.T, db *database.DB, itemID int64) int {
	var n int
	require.NoError(t, db.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM cover_art WHERE media_item_id = ?`, itemID).Scan(&n))
	return n
}

// (a) a non-local root with no filesystem factory must never hand the remote path to ffprobe/ffmpeg as a local path.
// The control file exists at that relative path in the working directory, so the pre-fix code really does process it.
func TestNEW1F3_RemoteProtocolWithoutFactoryIsRefused(t *testing.T) {
	db := newThumbSQLiteDB(t)
	cwd := t.TempDir()
	makeTestVideo(t, filepath.Join(cwd, "share", "movie.mp4"))
	t.Chdir(cwd)
	id := seedRawItem(t, db, "remote-nofactory", "smb", "", "share/movie.mp4")
	svc := newThumbSvc(t, db)
	require.Nil(t, svc.fsFactory)
	n, err := svc.GenerateMissingVideoThumbnails(context.Background(), 10)
	require.NoError(t, err)
	require.Equal(t, 0, n, "remote path without fsFactory must be refused, not read as a local path")
	require.Equal(t, 0, coverRows(t, db, id))
}

// (b) a local root with an empty root path must be refused (the file name would be resolved against the working directory).
func TestNEW1F3_LocalEmptyRootIsRefused(t *testing.T) {
	db := newThumbSQLiteDB(t)
	cwd := t.TempDir()
	makeTestVideo(t, filepath.Join(cwd, "movies", "movie.mp4"))
	t.Chdir(cwd)
	id := seedRawItem(t, db, "local-emptyroot", "local", "", "movies/movie.mp4")
	svc := newThumbSvc(t, db)
	n, err := svc.GenerateMissingVideoThumbnails(context.Background(), 10)
	require.NoError(t, err)
	require.Equal(t, 0, n, "a local root without a path must be refused")
	require.Equal(t, 0, coverRows(t, db, id))
}

// (c) files.path with .. must not leave the storage root.
func TestNEW1F3_DotDotEscapeIsRefused(t *testing.T) {
	db := newThumbSQLiteDB(t)
	base := t.TempDir()
	root := filepath.Join(base, "lib")
	require.NoError(t, os.MkdirAll(root, 0o755))
	makeTestVideo(t, filepath.Join(base, "secret", "movie.mp4"))
	id := seedRawItem(t, db, "local-escape", "local", root, "../secret/movie.mp4")
	svc := newThumbSvc(t, db)
	n, err := svc.GenerateMissingVideoThumbnails(context.Background(), 10)
	require.NoError(t, err)
	require.Equal(t, 0, n, "a path that escapes the storage root must be refused")
	require.Equal(t, 0, coverRows(t, db, id))
}

// (c, control) a normal in-root path with a harmless double slash and inner .. that stays inside still works.
func TestNEW1F3_InRootPathStillWorks(t *testing.T) {
	db := newThumbSQLiteDB(t)
	root := t.TempDir()
	makeTestVideo(t, filepath.Join(root, "a", "movie.mp4"))
	id := seedRawItem(t, db, "local-ok", "local", root, "a/b/../movie.mp4")
	svc := newThumbSvc(t, db)
	n, err := svc.GenerateMissingVideoThumbnails(context.Background(), 10)
	require.NoError(t, err)
	require.Equal(t, 1, n)
	require.Equal(t, 1, coverRows(t, db, id))
}

// (d) a relative path whose first character is "-" must be read as a file, not as an ffprobe option.
func TestNEW1F3_LeadingDashIsNotAnOption(t *testing.T) {
	cwd := t.TempDir()
	makeTestVideo(t, filepath.Join(cwd, "-dir", "movie.mp4"))
	t.Chdir(cwd)
	out, err := ffprobeDurationCmd("-dir/movie.mp4").Output()
	require.NoError(t, err, "ffprobe must take -dir/movie.mp4 as a file name, got %q", string(out))
	require.Contains(t, string(out), "8.0")
}

// F4: an item with several video files is processed once.
func TestNEW1F4_ItemWithSeveralFilesProcessedOnce(t *testing.T) {
	db := newThumbSQLiteDB(t)
	root := t.TempDir()
	makeTestVideo(t, filepath.Join(root, "m", "main.mp4"))
	makeTestVideo(t, filepath.Join(root, "m", "extra.mp4"))
	id := seedRawItem(t, db, "multi", "local", root, "m/main.mp4")
	var rootID int64
	require.NoError(t, db.QueryRowContext(context.Background(), `SELECT id FROM storage_roots WHERE name = 'multi'`).Scan(&rootID))
	addFileToItem(t, db, rootID, id, "multi", "m/extra.mp4", false)
	svc := newThumbSvc(t, db)
	n, err := svc.GenerateMissingVideoThumbnails(context.Background(), 10)
	require.NoError(t, err)
	require.Equal(t, 1, n, "one item must yield one thumbnail, not one per file")
	require.Equal(t, 1, coverRows(t, db, id))
}

// F4: when setDefaultCoverArt fails the item is a failure, not a generated thumbnail.
func TestNEW1F4_SetDefaultFailureIsNotCountedAsGenerated(t *testing.T) {
	db := newThumbSQLiteDB(t)
	root := t.TempDir()
	makeTestVideo(t, filepath.Join(root, "m.mp4"))
	seedRawItem(t, db, "setdef", "local", root, "m.mp4")
	// real SQLite trigger: every UPDATE of cover_art aborts, which is exactly what setDefaultCoverArt issues
	_, err := db.ExecContext(context.Background(), `CREATE TRIGGER block_cover_update BEFORE UPDATE ON cover_art BEGIN SELECT RAISE(ABORT, 'blocked'); END`)
	require.NoError(t, err)
	svc := newThumbSvc(t, db)
	n, err := svc.GenerateMissingVideoThumbnails(context.Background(), 10)
	require.NoError(t, err)
	require.Equal(t, 0, n, "a thumbnail that could not be made the default cover must not count as generated")
}
