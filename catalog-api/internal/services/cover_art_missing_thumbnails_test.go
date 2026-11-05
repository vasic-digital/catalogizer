package services

// NEW-1 (found by live stack-3 verification, 2026-11-05): POST /admin/covers/generate-thumbnails answered
// {"generated":0} instantly with no log although 11 video items existed and cover_art had 0 rows.
// These tests drive the REAL GenerateMissingVideoThumbnails code path (real ffmpeg, real migrated schema) on
// both dialects: SQLite in memory and a throwaway PostgreSQL started on demand (rootless podman, loopback,
// random port, random password in a 0600 env file that is never printed; the PostgreSQL leg SKIPs honestly
// when there is no container runtime). They need ffmpeg/ffprobe, otherwise they SKIP.

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"catalogizer/config"
	"catalogizer/database"

	_ "github.com/lib/pq"
	_ "github.com/mutecomm/go-sqlcipher"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func newThumbSQLiteDB(t *testing.T) *database.DB {
	t.Helper()
	raw, err := sql.Open("sqlite3", ":memory:?_pragma_key=test_key_ignored")
	require.NoError(t, err)
	t.Cleanup(func() { raw.Close() })
	db := database.WrapDB(raw, database.DialectSQLite)
	require.NoError(t, db.RunMigrations(context.Background()))
	return db
}

func newThumbPostgresDB(t *testing.T) *database.DB {
	t.Helper()
	rt, err := exec.LookPath("podman")
	if err != nil {
		t.Skip("no container runtime (podman): PostgreSQL leg not run")
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := ln.Addr().(*net.TCPAddr).Port
	require.NoError(t, ln.Close())
	raw := make([]byte, 16)
	_, err = rand.Read(raw)
	require.NoError(t, err)
	pw := hex.EncodeToString(raw)
	envFile := filepath.Join(t.TempDir(), "pg.env")
	require.NoError(t, os.WriteFile(envFile, []byte("POSTGRES_PASSWORD="+pw+"\nPOSTGRES_DB=thumbs\n"), 0o600))
	name := fmt.Sprintf("thumbs-pg-%d-%d", os.Getpid(), port)
	out, err := exec.Command(rt, "run", "-d", "--rm", "--name", name, "--env-file", envFile,
		"-p", fmt.Sprintf("127.0.0.1:%d:5432", port), "docker.io/library/postgres:15-alpine").CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		t.Skipf("cannot start postgres container: %s", msg[:min(120, len(msg))])
	}
	t.Cleanup(func() { _ = exec.Command(rt, "rm", "-f", name).Run() })
	dsn := fmt.Sprintf("host=127.0.0.1 port=%d user=postgres password=%s dbname=thumbs sslmode=disable", port, pw)
	deadline := time.Now().Add(60 * time.Second)
	for {
		c, e := sql.Open("postgres", dsn)
		if e == nil {
			if e = c.Ping(); e == nil {
				c.Close()
				break
			}
			c.Close()
		}
		if time.Now().After(deadline) {
			t.Fatalf("postgres did not become ready")
		}
		time.Sleep(500 * time.Millisecond)
	}
	db, err := database.NewConnection(&config.DatabaseConfig{Type: "postgres", Host: "127.0.0.1", Port: port, Name: "thumbs",
		User: "postgres", Password: pw, SSLMode: "disable", MaxOpenConnections: 2})
	require.NoError(t, err)
	require.NoError(t, db.RunMigrations(context.Background()))
	t.Cleanup(func() { db.Close() })
	return db
}

// seedLocalVideoItem creates a real video under root and the rows the scanner would write for it: files.path is
// the path relative to the storage root and INCLUDES the file name (universal_scanner convention).
func seedLocalVideoItem(t *testing.T, db *database.DB, root, rel string) int64 {
	t.Helper()
	for _, bin := range []string{"ffmpeg", "ffprobe"} {
		if _, err := exec.LookPath(bin); err != nil {
			t.Skipf("%s not installed", bin)
		}
	}
	full := filepath.Join(root, rel)
	require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
	out, err := exec.Command("ffmpeg", "-v", "error", "-f", "lavfi", "-i", "testsrc=duration=8:size=160x120:rate=10",
		"-pix_fmt", "yuv420p", "-y", full).CombinedOutput()
	require.NoError(t, err, string(out))

	ctx := context.Background()
	_, err = db.ExecContext(ctx, `INSERT INTO storage_roots (name, protocol, path, enabled) VALUES (?, 'local', ?, ?)`, "thumbroot", root, true)
	require.NoError(t, err)
	var rootID int64
	require.NoError(t, db.QueryRowContext(ctx, `SELECT id FROM storage_roots WHERE name = ?`, "thumbroot").Scan(&rootID))
	_, err = db.ExecContext(ctx, `INSERT INTO files (storage_root_id, path, name, size, is_directory, modified_at, deleted) VALUES (?, ?, ?, 1, ?, ?, ?)`,
		rootID, rel, filepath.Base(rel), false, time.Now().UTC(), false)
	require.NoError(t, err)
	var fileID int64
	require.NoError(t, db.QueryRowContext(ctx, `SELECT id FROM files WHERE storage_root_id = ? AND path = ?`, rootID, rel).Scan(&fileID))
	var typeID int64
	require.NoError(t, db.QueryRowContext(ctx, `SELECT id FROM media_types WHERE name = 'movie'`).Scan(&typeID))
	_, err = db.ExecContext(ctx, `INSERT INTO media_items (media_type_id, title) VALUES (?, 'Thumb Movie')`, typeID)
	require.NoError(t, err)
	var itemID int64
	require.NoError(t, db.QueryRowContext(ctx, `SELECT id FROM media_items WHERE title = 'Thumb Movie'`).Scan(&itemID))
	_, err = db.ExecContext(ctx, `INSERT INTO media_files (media_item_id, file_id, is_primary) VALUES (?, ?, ?)`, itemID, fileID, true)
	require.NoError(t, err)
	return itemID
}

func runMissingThumbnailsLeg(t *testing.T, db *database.DB) {
	root := t.TempDir()
	itemID := seedLocalVideoItem(t, db, root, "movies/Thumb.Movie.2026.mp4")

	svc := NewCoverArtService(db, zap.NewNop())
	svc.cacheDir = filepath.Join(t.TempDir(), "cache")

	n, err := svc.GenerateMissingVideoThumbnails(context.Background(), 10)
	require.NoError(t, err)
	require.Equal(t, 1, n, "the one video item without cover art must get a thumbnail")

	var localPath string
	var isDefault bool
	require.NoError(t, db.QueryRowContext(context.Background(),
		`SELECT local_path, is_default FROM cover_art WHERE media_item_id = ?`, itemID).Scan(&localPath, &isDefault))
	st, err := os.Stat(localPath)
	require.NoError(t, err, "cover_art.local_path must point at a real file")
	require.Greater(t, st.Size(), int64(0))
	require.True(t, isDefault, "the generated thumbnail must be the default cover")

	// A second run finds nothing left to do (the item now has a local default cover).
	n, err = svc.GenerateMissingVideoThumbnails(context.Background(), 10)
	require.NoError(t, err)
	require.Equal(t, 0, n)
}

func TestGenerateMissingVideoThumbnails_RealPath_SQLite(t *testing.T) {
	runMissingThumbnailsLeg(t, newThumbSQLiteDB(t))
}

func TestGenerateMissingVideoThumbnails_RealPath_PostgreSQL(t *testing.T) {
	runMissingThumbnailsLeg(t, newThumbPostgresDB(t))
}

// A row that cannot be scanned must surface as an error and a log line, never as a silent "generated: 0".
func TestGenerateMissingVideoThumbnails_ScanFailureIsReturned(t *testing.T) {
	db := newThumbSQLiteDB(t)
	root := t.TempDir()
	seedLocalVideoItem(t, db, root, "movies/Scan.Fail.2026.mp4")
	// port is an INTEGER column; a non-numeric text value cannot be scanned into the task's port.
	_, err := db.ExecContext(context.Background(), `UPDATE storage_roots SET port = 'not-a-port'`)
	require.NoError(t, err)

	svc := NewCoverArtService(db, zap.NewNop())
	svc.cacheDir = filepath.Join(t.TempDir(), "cache")
	_, err = svc.GenerateMissingVideoThumbnails(context.Background(), 10)
	require.Error(t, err, "an unscannable row must not be swallowed")
}
