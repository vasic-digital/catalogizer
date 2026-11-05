package services

import (
	"context"
	"database/sql"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"catalogizer/database"
	"catalogizer/filesystem"
	"catalogizer/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// LIVE-CLASS (§11.4.289): real UniversalScanner.processScanJob, real local filesystem client,
// production migrations, real files on disk (ffmpeg-generated video, real subtitle files). No
// doubles. D11b (docs/qa/live-conv-docs-subs-r1-20261104.md): a scan did not import sidecar
// subtitles (Movie.Name.2020.1080p.en.srt next to Movie.Name.2020.1080p.mp4).

const srtBody = "1\n00:00:01,000 --> 00:00:02,000\nHello\n\n2\n00:00:03,000 --> 00:00:04,000\nWorld\n"
const vttBody = "WEBVTT\n\n00:00:01.000 --> 00:00:02.000\nHello\n\n00:00:03.000 --> 00:00:04.000\nWorld\n"
const assBody = "[Script Info]\nScriptType: v4.00+\n\n[V4+ Styles]\nFormat: Name, Fontname, Fontsize\nStyle: Default,Arial,20\n\n[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\nDialogue: 0,0:00:01.00,0:00:02.00,Default,,0,0,0,,Hello\nDialogue: 0,0:00:03.00,0:00:04.00,Default,,0,0,0,,World\n"

type sidecarEnv struct {
	db   *database.DB
	root string
	svc  *SubtitleService
}

func newSidecarEnv(t *testing.T) *sidecarEnv {
	t.Helper()
	root := t.TempDir()
	real, err := filepath.EvalSymlinks(root)
	require.NoError(t, err)
	t.Setenv(filesystem.AllowedLocalBasesEnv, real)
	sqlDB, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	sqlDB.SetMaxOpenConns(1)
	db := database.WrapDB(sqlDB, database.DialectSQLite)
	require.NoError(t, db.RunMigrations(context.Background()))
	return &sidecarEnv{db: db, root: real, svc: &SubtitleService{db: db, logger: zap.NewNop()}}
}

func (e *sidecarEnv) write(t *testing.T, rel, body string) {
	t.Helper()
	p := filepath.Join(e.root, rel)
	require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
	require.NoError(t, os.WriteFile(p, []byte(body), 0o644))
}

func (e *sidecarEnv) video(t *testing.T, rel string) {
	t.Helper()
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg absent: cannot generate a real video (hardware_not_present-class SKIP)")
	}
	p := filepath.Join(e.root, rel)
	require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
	out, err := exec.Command("ffmpeg", "-v", "error", "-y", "-f", "lavfi", "-i", "testsrc=d=2:s=64x48:r=5", p).CombinedOutput()
	require.NoError(t, err, string(out))
}

func (e *sidecarEnv) scan(t *testing.T) {
	t.Helper()
	us := NewUniversalScanner(e.db, zap.NewNop(), nil, filesystem.NewDefaultClientFactory())
	defer us.Stop()
	rp := e.root
	job := ScanJob{ID: "sidecar", Path: "/", MaxDepth: 10, ScanType: "full", Context: context.Background(),
		StorageRoot: &models.StorageRoot{Name: "sidecar-root", Protocol: "local", Path: &rp, MaxDepth: 10}}
	us.processScanJob(job, 0)
	st, ok := us.GetActiveScanStatus("sidecar")
	require.True(t, ok)
	snap := st.GetSnapshot()
	require.Equal(t, "completed", snap.Status, "the scan itself must complete")
}

func (e *sidecarEnv) fileID(t *testing.T, name string) int64 {
	t.Helper()
	var id int64
	require.NoError(t, e.db.QueryRowContext(context.Background(), "SELECT id FROM files WHERE name = ?", name).Scan(&id), name)
	return id
}

func (e *sidecarEnv) tracks(t *testing.T, mediaID int64) []SubtitleTrack {
	t.Helper()
	tr, err := e.svc.GetSubtitles(context.Background(), mediaID)
	require.NoError(t, err)
	return tr
}

func (e *sidecarEnv) allTrackCount(t *testing.T) int {
	t.Helper()
	var n int
	require.NoError(t, e.db.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM subtitle_tracks").Scan(&n))
	return n
}

func TestD11b_ScanImportsSidecarSubtitles(t *testing.T) {
	e := newSidecarEnv(t)
	e.video(t, "Movies/Movie.Name.2020.1080p.mp4")
	e.write(t, "Movies/Movie.Name.2020.1080p.en.srt", srtBody)
	e.write(t, "Movies/Movie.Name.2020.1080p.en.vtt", vttBody)
	e.write(t, "Movies/Movie.Name.2020.1080p.de.ass", assBody)
	e.write(t, "Movies/Movie.Name.2020.1080p.out-of-sync.srt", srtBody)
	e.scan(t)

	tr := e.tracks(t, e.fileID(t, "Movie.Name.2020.1080p.mp4"))
	require.Len(t, tr, 4, "all four sidecar files must be listed for the media item")
	byPath := map[string]SubtitleTrack{}
	for _, x := range tr {
		require.NotNil(t, x.Path)
		byPath[*x.Path] = x
		assert.Equal(t, "sidecar", x.Source)
		require.NotNil(t, x.Content)
		assert.Contains(t, *x.Content, "Hello")
	}
	en := byPath["/Movies/Movie.Name.2020.1080p.en.srt"]
	assert.Equal(t, "en", en.LanguageCode)
	assert.Equal(t, "srt", en.Format)
	assert.Equal(t, "vtt", byPath["/Movies/Movie.Name.2020.1080p.en.vtt"].Format)
	de := byPath["/Movies/Movie.Name.2020.1080p.de.ass"]
	assert.Equal(t, "de", de.LanguageCode)
	assert.Equal(t, "ass", de.Format)
	assert.Equal(t, "und", byPath["/Movies/Movie.Name.2020.1080p.out-of-sync.srt"].LanguageCode)
}

func TestD11b_RescanIsIdempotentAndRefreshesChangedContent(t *testing.T) {
	e := newSidecarEnv(t)
	e.video(t, "Movie.mp4")
	e.write(t, "Movie.en.srt", srtBody)
	e.scan(t)
	id := e.fileID(t, "Movie.mp4")
	require.Len(t, e.tracks(t, id), 1)
	first := e.tracks(t, id)[0].ID
	e.scan(t)
	e.scan(t)
	tr := e.tracks(t, id)
	require.Len(t, tr, 1, "a rescan must not duplicate the sidecar track")
	assert.Equal(t, first, tr[0].ID, "the row keeps its id across rescans")
	// the sidecar changes on disk: the next scan refreshes the stored content, still one row
	e.write(t, "Movie.en.srt", "1\n00:00:01,000 --> 00:00:02,000\nChanged text\n")
	e.scan(t)
	tr = e.tracks(t, id)
	require.Len(t, tr, 1)
	assert.Contains(t, *tr[0].Content, "Changed text")
}

func TestD11b_SkipsOrphansInvalidAndEscapingSidecars(t *testing.T) {
	e := newSidecarEnv(t)
	e.video(t, "A/Movie.mp4")
	e.write(t, "A/Other.en.srt", srtBody) // no media with that base name
	e.write(t, "B/Movie.en.srt", srtBody) // same base name but another directory
	e.write(t, "A/Movie.fr.srt", "\x00\x01 not a subtitle at all")
	e.write(t, "A/Movie.es.srt", "just text, no cues")
	e.write(t, "A/Movie2.en.srt", srtBody) // a different base name that merely starts with the same letters
	// a sidecar that is a symlink pointing OUT of the storage root must never be read
	outside := filepath.Join(t.TempDir(), "secret.srt")
	require.NoError(t, os.WriteFile(outside, []byte(srtBody+"\nTOP-SECRET\n"), 0o644))
	require.NoError(t, os.Symlink(outside, filepath.Join(e.root, "A", "Movie.it.srt")))
	e.scan(t)
	assert.Equal(t, 0, e.allTrackCount(t), "nothing valid and confined matches the media")
	// control: a valid sidecar next to it IS imported by the same scan
	e.write(t, "A/Movie.pt.srt", srtBody)
	e.scan(t)
	tr := e.tracks(t, e.fileID(t, "Movie.mp4"))
	require.Len(t, tr, 1)
	assert.Equal(t, "pt", tr[0].LanguageCode)
	assert.NotContains(t, *tr[0].Content, "TOP-SECRET")
}

// the sidecar belongs to the video with the longest matching base name, never to a shorter prefix
func TestD11b_LongestBaseNameWins(t *testing.T) {
	e := newSidecarEnv(t)
	e.video(t, "Movie.mp4")
	e.video(t, "Movie.Part2.mp4")
	e.write(t, "Movie.Part2.en.srt", srtBody)
	e.scan(t)
	assert.Len(t, e.tracks(t, e.fileID(t, "Movie.Part2.mp4")), 1)
	assert.Len(t, e.tracks(t, e.fileID(t, "Movie.mp4")), 0)
}

// remote roots are never read from the host filesystem
func TestD11b_RemoteRootIsNeverImported(t *testing.T) {
	e := newSidecarEnv(t)
	e.video(t, "Movie.mp4")
	e.write(t, "Movie.en.srt", srtBody)
	e.scan(t) // local scan registers the files
	var rootID int64
	require.NoError(t, e.db.QueryRowContext(context.Background(), "SELECT id FROM storage_roots LIMIT 1").Scan(&rootID))
	_, err := e.db.ExecContext(context.Background(), "DELETE FROM subtitle_tracks")
	require.NoError(t, err)
	us := NewUniversalScanner(e.db, zap.NewNop(), nil, filesystem.NewDefaultClientFactory())
	defer us.Stop()
	rp := e.root
	us.importSidecars(ScanJob{ID: "x", Context: context.Background(),
		StorageRoot: &models.StorageRoot{Name: "sidecar-root", Protocol: "smb", Path: &rp}})
	assert.Equal(t, 0, e.allTrackCount(t))
}
