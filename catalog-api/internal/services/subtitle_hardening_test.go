package services

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// Review round 1 (F1/F2/F4/F5) tests. The service runs on the real sqlite schema; the media files
// are real files on disk (junk bytes where only the resolution/concurrency logic is exercised, which
// is a fixture INPUT, not a stand-in for ffmpeg: ffmpeg is never replaced).

type mapCache struct {
	mu sync.Mutex
	m  map[string]interface{}
}

func (c *mapCache) Get(_ context.Context, key string, dest interface{}) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.m[key]
	if !ok {
		return false, nil
	}
	if t, isTrack := v.(*SubtitleTrack); isTrack {
		if d, ok := dest.(*SubtitleTrack); ok {
			*d = *t
		}
	}
	if l, isList := v.([]string); isList {
		if d, ok := dest.(*[]string); ok {
			*d = append([]string(nil), l...)
		}
	}
	return true, nil
}
func (c *mapCache) Set(_ context.Context, key string, v interface{}, _ time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.m == nil {
		c.m = map[string]interface{}{}
	}
	c.m[key] = v
	return nil
}
func (c *mapCache) Delete(_ context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.m, key)
	return nil
}
func (c *mapCache) keys() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []string
	for k := range c.m {
		out = append(out, k)
	}
	return out
}

type hardenEnv struct {
	svc   *SubtitleService
	cache *mapCache
	root  string
}

func newHardenEnv(t *testing.T) *hardenEnv {
	t.Helper()
	db, cleanup := newSubtitleTestDB(t)
	t.Cleanup(cleanup)
	_, err := db.ExecContext(context.Background(), `CREATE TABLE IF NOT EXISTS storage_roots (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT, protocol TEXT, path TEXT)`)
	require.NoError(t, err)
	_, err = db.ExecContext(context.Background(), `ALTER TABLE subtitle_tracks ADD COLUMN uploaded_by INTEGER`)
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
	c := &mapCache{}
	svc := NewSubtitleService(db, zap.NewNop(), c)
	t.Cleanup(svc.Close)
	return &hardenEnv{svc: svc, cache: c, root: t.TempDir()}
}

func (e *hardenEnv) addRoot(t *testing.T, protocol, path string) int64 {
	t.Helper()
	id, err := e.svc.db.InsertReturningID(context.Background(), `INSERT INTO storage_roots (name, protocol, path) VALUES (?,?,?)`, protocol+path, protocol, path)
	require.NoError(t, err)
	return id
}

func (e *hardenEnv) addFile(t *testing.T, rootID int64, path, name string) int64 {
	t.Helper()
	id, err := e.svc.db.InsertReturningID(context.Background(), `INSERT INTO files (storage_root_id, path, name, size) VALUES (?,?,?,1)`, rootID, path, name)
	require.NoError(t, err)
	return id
}

var trackSeq int

func (e *hardenEnv) addTrack(t *testing.T, media int64, uploader int64) string {
	t.Helper()
	trackSeq++
	sid := fmt.Sprintf("%d", 1000+trackSeq)
	_, err := e.svc.db.ExecContext(context.Background(),
		`INSERT INTO subtitle_tracks (id, media_item_id, language, language_code, source, format, content, uploaded_by) VALUES (?, ?, 'English','en','upload','srt', ?, ?)`,
		sid, media, "1\n00:00:01,000 --> 00:00:02,000\nhello there\n\n", uploader)
	require.NoError(t, err)
	return sid
}

// ---- F2: media resolution --------------------------------------------------

func TestResolveMediaFile_OnlyLocalRootsAndConfined(t *testing.T) {
	e := newHardenEnv(t)
	ctx := context.Background()

	outside := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(outside, "secret.mkv"), []byte("x"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(e.root, "ok.mkv"), []byte("x"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(e.root, "-version.mkv"), []byte("x"), 0o600))
	require.NoError(t, os.Symlink(filepath.Join(outside, "secret.mkv"), filepath.Join(e.root, "link.mkv")))

	local := e.addRoot(t, "local", e.root)
	remote := e.addRoot(t, "smb", e.root) // a remote-protocol root whose path happens to exist on the host

	okID := e.addFile(t, local, "/ok.mkv", "ok.mkv")
	dashID := e.addFile(t, local, "/-version.mkv", "-version.mkv")
	escID := e.addFile(t, local, "/../"+filepath.Base(outside)+"/secret.mkv", "secret.mkv")
	linkID := e.addFile(t, local, "/link.mkv", "link.mkv")
	// a remote-protocol row whose path happens to be a real file on the HOST filesystem
	remoteID := e.addFile(t, remote, "/ok.mkv", "ok.mkv")
	// no storage root at all, raw host path
	orphanID, err := e.svc.db.InsertReturningID(ctx, `INSERT INTO files (storage_root_id, path, name, size) VALUES (0, ?, 'ok.mkv', 1)`, filepath.Join(e.root, "ok.mkv"))
	require.NoError(t, err)

	got, found, err := e.svc.resolveMediaFile(ctx, okID)
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, evalSym(t, filepath.Join(e.root, "ok.mkv")), got)

	got, found, err = e.svc.resolveMediaFile(ctx, dashID)
	require.NoError(t, err)
	assert.True(t, found)
	assert.True(t, filepath.IsAbs(got), "a dash-leading file name must resolve to an ABSOLUTE path, got %q", got)

	for name, id := range map[string]int64{"../ escape": escID, "symlink escape": linkID, "remote protocol": remoteID, "no storage root": orphanID} {
		got, found, err = e.svc.resolveMediaFile(ctx, id)
		require.NoError(t, err, name)
		assert.True(t, found, "%s: the media row exists", name)
		assert.Empty(t, got, "%s must NOT resolve to a host file", name)
	}
}

func evalSym(t *testing.T, p string) string {
	t.Helper()
	r, err := filepath.EvalSymlinks(p)
	require.NoError(t, err)
	return r
}

func TestFFmpegArgs_NoPipeProtocolFilePrefixedNicedAndSingleThread(t *testing.T) {
	probe := ffprobeArgs("-version.wav")
	assert.Contains(t, strings.Join(probe, " "), "-i file:-version.wav", "ffprobe input must be passed via -i with a file: prefix: %v", probe)
	assert.NotContains(t, strings.Join(probe, " "), "pipe", "no pipe protocol: %v", probe)

	a := ffmpegActivityArgs("-version.wav", 60)
	j := strings.Join(a, " ")
	assert.Contains(t, j, "-i file:-version.wav", a)
	assert.Contains(t, j, "-nostats")
	assert.Contains(t, j, "-threads 1")
	assert.Contains(t, j, "-protocol_whitelist file ", "whitelist must be exactly file: %v", a)
	assert.NotContains(t, j, "pipe")
}

// ---- F1: bounded analysis --------------------------------------------------

func TestVerifySync_ConcurrencyLimitFailsFastAndDedupesAndCaches(t *testing.T) {
	e := newHardenEnv(t)
	ctx := context.Background()
	local := e.addRoot(t, "local", e.root)
	require.NoError(t, os.WriteFile(filepath.Join(e.root, "a.mkv"), []byte("not really media"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(e.root, "b.mkv"), []byte("not really media 2"), 0o600))
	a := e.addFile(t, local, "/a.mkv", "a.mkv")
	b := e.addFile(t, local, "/b.mkv", "b.mkv")
	content := "1\n00:00:01,000 --> 00:00:02,000\nhello there\n\n"
	trA := &SubtitleTrack{ID: "1", Format: "srt", Content: &content}

	e.svc.SetSyncConcurrency(1)
	started := make(chan struct{}, 4)
	release := make(chan struct{})
	e.svc.analysisHook = func() { started <- struct{}{}; <-release }

	type res struct {
		r   *SubtitleSyncResult
		err error
	}
	first := make(chan res, 1)
	go func() { r, err := e.svc.VerifySynchronization(ctx, a, trA); first <- res{r, err} }()
	select {
	case <-started:
	case <-time.After(10 * time.Second):
		t.Fatal("first analysis never started")
	}

	// a different media file while the only slot is taken: fail fast, no queueing, no new run
	t0 := time.Now()
	_, err := e.svc.VerifySynchronization(ctx, b, trA)
	assert.ErrorIs(t, err, ErrSyncBusy)
	assert.Less(t, time.Since(t0), 2*time.Second, "must fail fast, not wait for the slot")

	// the SAME media file joins the in-flight analysis (no second run, no busy error)
	joined := make(chan res, 1)
	go func() { r, err := e.svc.VerifySynchronization(ctx, a, trA); joined <- res{r, err} }()
	time.Sleep(200 * time.Millisecond)

	close(release)
	r1 := <-first
	require.NoError(t, r1.err)
	r2 := <-joined
	require.NoError(t, r2.err)
	assert.Equal(t, int64(1), e.svc.AnalysisRuns(), "the in-flight analysis was shared")

	// cached now: another call must not run the analysis again
	r3, err := e.svc.VerifySynchronization(ctx, a, trA)
	require.NoError(t, err)
	assert.NotNil(t, r3)
	assert.Equal(t, int64(1), e.svc.AnalysisRuns(), "a cache hit must not spawn ffmpeg")

	// changing the file (mtime/size) invalidates the cache entry
	require.NoError(t, os.WriteFile(filepath.Join(e.root, "a.mkv"), []byte("different and longer content"), 0o600))
	e.svc.analysisHook = nil
	_, err = e.svc.VerifySynchronization(ctx, a, trA)
	require.NoError(t, err)
	assert.Equal(t, int64(2), e.svc.AnalysisRuns(), "a changed media file must be re-analysed")
}

// ---- F4: delete invalidates the translation cache; translate checks existence first ----

func TestDeleteSubtitle_InvalidatesTranslationCacheAndTranslateChecksExistenceFirst(t *testing.T) {
	e := newHardenEnv(t)
	ctx := context.Background()
	local := e.addRoot(t, "local", e.root)
	m := e.addFile(t, local, "/a.mkv", "a.mkv")
	id := e.addTrack(t, m, 7)

	cached := &SubtitleTrack{ID: id + "-fr", Language: "French", LanguageCode: "fr"}
	require.NoError(t, e.svc.saveCachedTranslation(ctx, id, "fr", cached))
	require.NoError(t, e.svc.saveCachedTranslation(ctx, id, "de", cached))
	require.NoError(t, e.svc.saveCachedTranslation(ctx, "999", "fr", cached)) // another subtitle: must survive
	require.Len(t, e.cache.keys(), 5, "2 translations + index per subtitle, 1 foreign translation + its index: %v", e.cache.keys())

	require.NoError(t, e.svc.DeleteSubtitle(ctx, id, 7, false))
	for _, k := range e.cache.keys() {
		assert.NotContains(t, k, ":"+id+":", "translation cache entries of the deleted subtitle must be gone: %v", e.cache.keys())
	}
	assert.NotEmpty(t, e.cache.keys(), "other subtitles' cache entries stay")

	// a stale cached translation must not be served for a subtitle that no longer exists
	e.cache.Set(ctx, "subtitle_translation:"+id+":fr", cached, time.Hour)
	_, err := e.svc.TranslateSubtitle(ctx, &SubtitleTranslationRequest{SubtitleID: id, TargetLanguage: "fr", UseCache: true})
	assert.ErrorIs(t, err, ErrSubtitleNotFound)
}

func TestDeleteSubtitle_NonOwnerIsNotFoundNotForbidden(t *testing.T) {
	e := newHardenEnv(t)
	ctx := context.Background()
	local := e.addRoot(t, "local", e.root)
	m := e.addFile(t, local, "/a.mkv", "a.mkv")
	id := e.addTrack(t, m, 7)
	err := e.svc.DeleteSubtitle(ctx, id, 8, false)
	assert.ErrorIs(t, err, ErrSubtitleNotFound, "a non-owner must not learn that the id exists")
	assert.NoError(t, e.svc.DeleteSubtitle(ctx, id, 7, false))
}

// ---- F5: reason wording ------------------------------------------------------

func TestSyncReason_NoZeroOfNCuesWording(t *testing.T) {
	s := beyondReason(0, 12, 60.0, 62.5)
	assert.NotContains(t, s, "0 of")
	assert.Contains(t, s, "62.5")
	s = beyondReason(3, 12, 60.0, 90)
	assert.Contains(t, s, "3 of 12 cues start beyond")
}
