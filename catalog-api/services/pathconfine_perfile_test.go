package services

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"catalogizer/internal/pathguard"
	"catalogizer/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// perFileFixture: base/{src,dst,data} with data/catalogizer.db protected, a secret outside both
// sync roots, and a regular file src/a.txt.
func perFileFixture(t *testing.T) (base, src, dst, db, secret string) {
	t.Helper()
	base = t.TempDir()
	src, dst = filepath.Join(base, "src"), filepath.Join(base, "dst")
	for _, d := range []string{src, dst, filepath.Join(base, "data")} {
		require.NoError(t, os.MkdirAll(d, 0o755))
	}
	db = filepath.Join(base, "data", "catalogizer.db")
	require.NoError(t, os.WriteFile(db, []byte("SQLite format 3\x00"), 0o600))
	protectDatabaseForTest(t, db)
	secret = filepath.Join(base, "secret.txt")
	require.NoError(t, os.WriteFile(secret, []byte("SECRET"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(src, "a.txt"), []byte("a"), 0o644))
	return
}

// A symlink or a hard link planted in the sync source is never read: filepath.Walk reports a
// link as itself and the copy used to os.Open it, following it to the database or outside the
// source root (review round 4, per-file symlinks).
func TestSync_MirrorAndIncrementalSkipLinksPlantedInTheSource(t *testing.T) {
	// The per-file guards also check the configured roots (review round 9, F1); these fixtures
	// live under the temp directory, outside the fixed roots TestMain sets.
	allowSyncDirs(t, os.TempDir())
	for _, mode := range []string{"mirror", "incremental"} {
		t.Run(mode, func(t *testing.T) {
			_, src, dst, db, secret := perFileFixture(t)
			require.NoError(t, os.Symlink(secret, filepath.Join(src, "leak")))
			require.NoError(t, os.Symlink(db, filepath.Join(src, "dbl")))
			require.NoError(t, os.Link(db, filepath.Join(src, "hl")))

			s := &SyncService{}
			var err error
			if mode == "mirror" {
				err = s.performMirrorSync(src, dst, &models.SyncSession{})
			} else {
				err = s.performIncrementalSync(src, dst, &models.SyncSession{})
			}
			require.NoError(t, err)
			got, rerr := os.ReadFile(filepath.Join(dst, "a.txt"))
			require.NoError(t, rerr, "the regular file is still synced")
			assert.Equal(t, "a", string(got))
			for _, n := range []string{"leak", "dbl", "hl"} {
				_, lerr := os.Lstat(filepath.Join(dst, n))
				assert.True(t, os.IsNotExist(lerr), "%s must not be copied (lstat err=%v)", n, lerr)
			}
		})
	}
}

// A symlink planted in the destination does not redirect the copy: the write is refused and the
// file it points to is unchanged.
func TestSync_CopyRefusesASymlinkPlantedInTheDestination(t *testing.T) {
	// The per-file guards also check the configured roots (review round 9, F1); these fixtures
	// live under the temp directory, outside the fixed roots TestMain sets.
	allowSyncDirs(t, os.TempDir())
	_, src, dst, db, secret := perFileFixture(t)
	require.NoError(t, os.Symlink(secret, filepath.Join(dst, "a.txt")))
	require.NoError(t, os.Symlink(db, filepath.Join(dst, "b.txt")))
	require.NoError(t, os.WriteFile(filepath.Join(src, "b.txt"), []byte("b"), 0o644))
	future := time.Now().Add(time.Hour)
	for _, n := range []string{"a.txt", "b.txt"} {
		require.NoError(t, os.Chtimes(filepath.Join(src, n), future, future))
	}

	err := (&SyncService{}).performIncrementalSync(src, dst, &models.SyncSession{})
	require.Error(t, err)
	got, _ := os.ReadFile(secret)
	assert.Equal(t, "SECRET", string(got), "the file behind the planted link is unchanged")
	got, _ = os.ReadFile(db)
	assert.Equal(t, "SQLite format 3\x00", string(got), "the database behind the planted link is unchanged")
}

// The S3 and GCS upload walkers use uploadableFile for every file they meet.
func TestSync_UploadableFile(t *testing.T) {
	// The per-file guards also check the configured roots (review round 9, F1); these fixtures
	// live under the temp directory, outside the fixed roots TestMain sets.
	allowSyncDirs(t, os.TempDir())
	base, src, _, db, secret := perFileFixture(t)
	require.NoError(t, os.Symlink(secret, filepath.Join(src, "leak")))
	require.NoError(t, os.Link(db, filepath.Join(src, "hl")))
	fifo := filepath.Join(src, "pipe")
	require.NoError(t, syscall.Mkfifo(fifo, 0o600))
	cases := map[string]bool{"a.txt": true, "leak": false, "hl": false, "pipe": false}
	for n, want := range cases {
		p := filepath.Join(src, n)
		fi, err := os.Lstat(p)
		require.NoError(t, err)
		assert.Equal(t, want, uploadableFile(p, fi, src), n)
	}
	// a regular file outside the walked root is refused too
	fi, err := os.Lstat(secret)
	require.NoError(t, err)
	assert.False(t, uploadableFile(secret, fi, src), "outside root")
	_ = base
}

// LibreOffice writes into --outdir; the rename source is computed there, not next to the input.
func TestConversion_LibreOfficeOutputIsInTheOutputDirectory(t *testing.T) {
	assert.Equal(t, "/out/dir/report.html", libreOfficeHTMLOutput("/media/docs/report.pdf", "/out/dir/x.html"))
	assert.Equal(t, "/out/archive.tar.html", libreOfficeHTMLOutput("/media/archive.tar.gz", "/out/y.html"))
}

// A conversion target whose lexical reading differs from the kernel reading is refused: the
// multi-page writer and the LibreOffice --outdir use filepath.Dir(target) (review round 4, H1).
func TestConversion_TargetWithDotDotAfterSymlinkRefused(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "root")
	require.NoError(t, os.MkdirAll(filepath.Join(root, "d1", "d2"), 0o755))
	require.NoError(t, os.Symlink("d1/d2", filepath.Join(root, "lnk")))
	src := filepath.Join(root, "in.pdf")
	require.NoError(t, os.WriteFile(src, []byte("%PDF"), 0o644))
	t.Setenv(pathGuardEnvConv, root)
	t.Setenv(pathGuardEnvRead, root)
	err := confineConversionPaths(src, root+"/lnk/../../out.pdf")
	require.Error(t, err)
	assert.ErrorIs(t, err, pathguard.ErrAmbiguousPath)
}

// copyFile checks its source itself, not only through the walkers' uploadableFile filter: a
// symlinked source pointing outside the source root is refused and nothing is written.
func TestSync_CopyFileRefusesASymlinkedSource(t *testing.T) {
	_, src, dst, _, secret := perFileFixture(t)
	link := filepath.Join(src, "leak")
	require.NoError(t, os.Symlink(secret, link))
	out := filepath.Join(dst, "leak")

	err := (&SyncService{}).copyFile(link, out, 0o644, src, dst)
	require.Error(t, err)
	_, statErr := os.Lstat(out)
	assert.True(t, os.IsNotExist(statErr), "nothing is written to the destination")
}
