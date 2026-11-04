package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"mime/multipart"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"catalogizer/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// Live-class tests for the subtitle defects D5-D8 (docs/qa/live-conv-docs-subs-r1-20261104.md):
// the real router, the production sqlite schema + migrations, a REAL media file generated
// with ffmpeg whose audio carries a known irregular speech-like burst pattern, and subtitle
// FILES (fixture inputs) shifted by known offsets. No service is replaced by a double.

// subBursts is the known audio-activity pattern (start,end seconds) of the generated media.
var subBursts = [][2]float64{
	{2, 4.5}, {6, 7.2}, {10, 13}, {15, 16}, {19, 22.5}, {25, 26.2},
	{29, 33}, {35, 36.5}, {40, 43}, {46, 47}, {50, 53.5}, {56, 58},
}

const subMediaSeconds = 60

type memCache struct{ m map[string]interface{} }

func (c *memCache) Get(ctx context.Context, key string, dest interface{}) (bool, error) {
	return false, nil
}
func (c *memCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	return nil
}

type subHarness struct {
	t      *testing.T
	r      *gin.Engine
	svc    *services.SubtitleService
	h      *SubtitleHandler
	dbExec func(q string, args ...interface{})
	// addFile inserts a files row (storage root id, path, name) and returns its id.
	addFile func(rootID int64, path, name string) int64
	// addRoot inserts a storage root and returns its id.
	addRoot func(protocol, path string) int64
	rootID  int64
	media   int64 // files id of the real generated media
	ghost   int64 // files id whose file does not exist on disk
	dir     string
}

// genMedia makes a real 60 s audio+video file whose audio is a tone gated by subBursts.
func genMedia(t *testing.T, dir string) string {
	t.Helper()
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not available: live media generation impossible (honest SKIP, not a stand-in)")
	}
	var terms []string
	for _, b := range subBursts {
		terms = append(terms, fmt.Sprintf("between(t,%g,%g)", b[0], b[1]))
	}
	expr := "gt(" + strings.Join(terms, "+") + ",0)"
	out := filepath.Join(dir, "movie.mkv")
	cmd := exec.Command("ffmpeg", "-nostdin", "-v", "error", "-y",
		"-f", "lavfi", "-i", fmt.Sprintf("testsrc=size=64x64:rate=5:duration=%d", subMediaSeconds),
		"-f", "lavfi", "-i", fmt.Sprintf("sine=frequency=440:duration=%d", subMediaSeconds),
		"-af", fmt.Sprintf("volume='%s':eval=frame", expr),
		"-c:v", "mpeg4", "-c:a", "aac", "-shortest", out)
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("ffmpeg media generation failed: %v: %s", err, b)
	}
	return out
}

func ts(sec float64, comma bool) string {
	if sec < 0 {
		sec = 0
	}
	ms := int(math.Round(sec * 1000))
	h, m, s, msr := ms/3600000, (ms/60000)%60, (ms/1000)%60, ms%1000
	sep := "."
	if comma {
		sep = ","
	}
	return fmt.Sprintf("%02d:%02d:%02d%s%03d", h, m, s, sep, msr)
}

func srtShifted(shift float64) string {
	var b strings.Builder
	for i, c := range subBursts {
		fmt.Fprintf(&b, "%d\n%s --> %s\nLine number %d here\n\n", i+1, ts(c[0]+shift, true), ts(c[1]+shift, true), i+1)
	}
	return b.String()
}

func vttShifted(shift float64) string {
	var b strings.Builder
	b.WriteString("WEBVTT\n\n")
	for i, c := range subBursts {
		fmt.Fprintf(&b, "%s --> %s\nLine number %d here\n\n", ts(c[0]+shift, false), ts(c[1]+shift, false), i+1)
	}
	return b.String()
}

func assTS(sec float64) string {
	if sec < 0 {
		sec = 0
	}
	cs := int(math.Round(sec * 100))
	return fmt.Sprintf("%d:%02d:%02d.%02d", cs/360000, (cs/6000)%60, (cs/100)%60, cs%100)
}

func assShifted(shift float64) string {
	var b strings.Builder
	b.WriteString("[Script Info]\nTitle: t\nScriptType: v4.00+\n\n[V4+ Styles]\nFormat: Name, Fontname, Fontsize\nStyle: Default,Arial,20\n\n[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n")
	for i, c := range subBursts {
		fmt.Fprintf(&b, "Dialogue: 0,%s,%s,Default,,0,0,0,,Line number %d here\n", assTS(c[0]+shift), assTS(c[1]+shift), i+1)
	}
	return b.String()
}

func newSubHarness(t *testing.T) *subHarness {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := openProductionFileDB(t) // production open path + production migrations
	dir := t.TempDir()
	media := genMedia(t, dir)
	require.FileExists(t, media)

	ctx := context.Background()
	_, err := db.ExecContext(ctx, `INSERT INTO storage_roots (name, protocol, path) VALUES ('r1','local',?)`, dir)
	require.NoError(t, err)
	var rootID int64
	require.NoError(t, db.QueryRowContext(ctx, `SELECT id FROM storage_roots WHERE name='r1'`).Scan(&rootID))
	mediaID, err := db.InsertReturningID(ctx, `INSERT INTO files (storage_root_id, path, name, size, modified_at) VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)`, rootID, "/movie.mkv", "movie.mkv", 1)
	require.NoError(t, err)
	ghostID, err := db.InsertReturningID(ctx, `INSERT INTO files (storage_root_id, path, name, size, modified_at) VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)`, rootID, "/missing.mkv", "missing.mkv", 1)
	require.NoError(t, err)

	svc := services.NewSubtitleService(db, zap.NewNop(), &memCache{})
	t.Cleanup(svc.Close)
	h := NewSubtitleHandler(svc, zap.NewNop()).WithAdminChecker(func(ctx context.Context, uid int) (bool, error) { return uid == 1, nil })

	r := gin.New()
	// the test "auth" middleware: X-Test-User sets user_id exactly like the JWT middleware does
	r.Use(func(c *gin.Context) {
		if v := c.GetHeader("X-Test-User"); v != "" {
			var id int
			fmt.Sscanf(v, "%d", &id)
			c.Set("user_id", id)
		}
		c.Next()
	})
	// the REAL production route table (the same function main.go calls), with an admin gate that
	// behaves like RequireAdmin(): user 1 is the administrator in this harness
	adminGate := func(c *gin.Context) {
		if v, _ := c.Get("user_id"); v != 1 {
			c.AbortWithStatusJSON(403, gin.H{"success": false, "code": "FORBIDDEN"})
			return
		}
		c.Next()
	}
	RegisterSubtitleRoutes(r.Group("/api/v1"), func() *SubtitleHandler { return h }, adminGate)
	return &subHarness{t: t, r: r, svc: svc, h: h, media: mediaID, ghost: ghostID, dir: dir, rootID: rootID,
		addFile: func(rid int64, path, name string) int64 {
			id, err := db.InsertReturningID(ctx, `INSERT INTO files (storage_root_id, path, name, size, modified_at) VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)`, rid, path, name, 1)
			require.NoError(t, err)
			return id
		},
		addRoot: func(protocol, path string) int64 {
			id, err := db.InsertReturningID(ctx, `INSERT INTO storage_roots (name, protocol, path) VALUES (?,?,?)`, protocol+path, protocol, path)
			require.NoError(t, err)
			return id
		},
		dbExec: func(q string, args ...interface{}) {
			_, err := db.ExecContext(ctx, q, args...)
			require.NoError(t, err)
		}}
}

func (h *subHarness) do(method, path, user string, body []byte, ctype string) (int, map[string]interface{}, string) {
	h.t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	if ctype != "" {
		req.Header.Set("Content-Type", ctype)
	}
	if user != "" {
		req.Header.Set("X-Test-User", user)
	}
	w := httptest.NewRecorder()
	h.r.ServeHTTP(w, req)
	var out map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out, w.Body.String()
}

func (h *subHarness) upload(user string, mediaID int64, filename string, content []byte) (int, map[string]interface{}, string) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("media_item_id", fmt.Sprint(mediaID))
	_ = mw.WriteField("language", "English")
	_ = mw.WriteField("language_code", "en")
	fw, _ := mw.CreateFormFile("file", filename)
	_, _ = fw.Write(content)
	_ = mw.Close()
	return h.do("POST", "/api/v1/subtitles/upload", user, buf.Bytes(), mw.FormDataContentType())
}

func (h *subHarness) uploadOK(user string, mediaID int64, filename, content string) string {
	h.t.Helper()
	code, js, raw := h.upload(user, mediaID, filename, []byte(content))
	require.Equal(h.t, 200, code, raw)
	id, _ := js["subtitle_id"].(string)
	require.NotEmpty(h.t, id, "upload must return the real subtitle id: %s", raw)
	return id
}

func syncOf(js map[string]interface{}) map[string]interface{} {
	m, _ := js["sync_result"].(map[string]interface{})
	return m
}

// ---- D5 -------------------------------------------------------------------

func TestSubtitleD5_VerifySyncMeasuresRealOffset(t *testing.T) {
	h := newSubHarness(t)
	cases := []struct {
		name     string
		file     string
		content  string
		wantMs   float64
		wantGood bool
	}{
		{"srt in sync", "a.srt", srtShifted(0), 0, true},
		{"srt late +7s", "b.srt", srtShifted(7), 7000, false},
		{"vtt early -1.5s", "c.vtt", vttShifted(-1.5), -1500, false},
		{"ass late +7s", "d.ass", assShifted(7), 7000, false},
		{"vtt in sync", "e.vtt", vttShifted(0), 0, true},
		{"ass in sync", "f.ass", assShifted(0), 0, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			id := h.uploadOK("2", h.media, c.file, c.content)
			code, js, raw := h.do("GET", fmt.Sprintf("/api/v1/subtitles/%s/verify-sync/%d", id, h.media), "2", nil, "")
			require.Equal(t, 200, code, raw)
			sr := syncOf(js)
			require.NotNil(t, sr, raw)
			assert.Equal(t, "measured", sr["status"], raw)
			assert.InDelta(t, c.wantMs, sr["sync_offset"].(float64), 300, "measured offset (ms) must match the real shift: %s", raw)
			assert.Equal(t, c.wantGood, sr["is_valid"], raw)
			assert.Greater(t, sr["confidence"].(float64), 0.0, raw)
			assert.NotEqual(t, 0.8, sr["confidence"], "confidence must be computed, not the constant 0.8: %s", raw)
		})
	}
}

func TestSubtitleD5_ShiftedAndInSyncDiffer(t *testing.T) {
	h := newSubHarness(t)
	ok := h.uploadOK("2", h.media, "a.srt", srtShifted(0))
	bad := h.uploadOK("2", h.media, "b.srt", srtShifted(7))
	_, a, _ := h.do("GET", fmt.Sprintf("/api/v1/subtitles/%s/verify-sync/%d", ok, h.media), "2", nil, "")
	_, b, _ := h.do("GET", fmt.Sprintf("/api/v1/subtitles/%s/verify-sync/%d", bad, h.media), "2", nil, "")
	assert.NotEqual(t, syncOf(a)["is_valid"], syncOf(b)["is_valid"], "the +7s subtitle must not be reported identical to the in-sync one")
	assert.NotEqual(t, syncOf(a)["sync_offset"], syncOf(b)["sync_offset"])
}

func TestSubtitleD5_UndeterminedWhenMediaNotMeasurable(t *testing.T) {
	h := newSubHarness(t)
	id := h.uploadOK("2", h.ghost, "a.srt", srtShifted(0))
	code, js, raw := h.do("GET", fmt.Sprintf("/api/v1/subtitles/%s/verify-sync/%d", id, h.ghost), "2", nil, "")
	require.Equal(t, 200, code, raw)
	sr := syncOf(js)
	assert.Equal(t, "undetermined", sr["status"], raw)
	assert.Equal(t, 0.0, sr["confidence"], "never fabricate confidence: %s", raw)
	assert.Equal(t, 0.0, sr["sync_offset"], raw)
	assert.Equal(t, false, sr["is_valid"], raw)
	assert.NotEmpty(t, sr["reason"], raw)
}

func TestSubtitleD5_CuesBeyondMediaDurationAreInvalid(t *testing.T) {
	h := newSubHarness(t)
	id := h.uploadOK("2", h.media, "late.srt", srtShifted(300)) // every cue starts after the 60 s media ends
	code, js, raw := h.do("GET", fmt.Sprintf("/api/v1/subtitles/%s/verify-sync/%d", id, h.media), "2", nil, "")
	require.Equal(t, 200, code, raw)
	sr := syncOf(js)
	assert.Equal(t, false, sr["is_valid"], raw)
	assert.Contains(t, []interface{}{"invalid", "measured"}, sr["status"], raw)
	assert.Contains(t, sr["reason"], "beyond", raw)
}

func TestSubtitleD5_UnknownIdsAre404(t *testing.T) {
	h := newSubHarness(t)
	id := h.uploadOK("2", h.media, "a.srt", srtShifted(0))
	code, js, raw := h.do("GET", fmt.Sprintf("/api/v1/subtitles/%s/verify-sync/99999999", id), "2", nil, "")
	assert.Equal(t, 404, code, raw)
	assert.Equal(t, "MEDIA_NOT_FOUND", js["code"], raw)
	code, js, raw = h.do("GET", fmt.Sprintf("/api/v1/subtitles/424242/verify-sync/%d", h.media), "2", nil, "")
	assert.Equal(t, 404, code, raw)
	assert.Equal(t, "SUBTITLE_NOT_FOUND", js["code"], raw)
	code, _, raw = h.do("GET", fmt.Sprintf("/api/v1/subtitles/%s/verify-sync/%d", id, h.ghost), "2", nil, "")
	assert.Equal(t, 404, code, "subtitle belongs to another media item: %s", raw)
	assert.NotContains(t, raw, "sql:")
}

// ---- D6 / D7 / D8 ---------------------------------------------------------

func TestSubtitleD6_UploadErrorMapping(t *testing.T) {
	h := newSubHarness(t)
	code, js, raw := h.upload("2", 99999999, "a.srt", []byte(srtShifted(0)))
	assert.Equal(t, 404, code, raw)
	assert.Equal(t, "MEDIA_NOT_FOUND", js["code"], raw)

	garbage := []byte{0xff, 0xfe, 0x00, 0x01, 0x80, 0x81, 0x00, 0x00, 0xc3, 0x28, 0xa0, 0xa1}
	code, js, raw = h.upload("2", h.media, "a.srt", garbage)
	assert.Equal(t, 422, code, raw)
	assert.Equal(t, "INVALID_SUBTITLE_CONTENT", js["code"], raw)

	code, js, raw = h.upload("2", h.media, "a.srt", []byte("this is a plain text note, not a subtitle\nat all\n"))
	assert.Equal(t, 422, code, "text that parses to zero cues is not a subtitle: %s", raw)

	code, js, raw = h.upload("2", h.media, "a.vtt", []byte(srtShifted(0)))
	assert.Equal(t, 422, code, "SRT body under a .vtt name has no WEBVTT header: %s", raw)

	// the valid ones still work, for all three formats
	for _, f := range []struct{ n, c string }{{"a.srt", srtShifted(0)}, {"a.vtt", vttShifted(0)}, {"a.ass", assShifted(0)}} {
		code, js, raw = h.upload("2", h.media, f.n, []byte(f.c))
		assert.Equal(t, 200, code, f.n+": "+raw)
	}
	// CRLF SRT is a valid subtitle too
	code, _, raw = h.upload("2", h.media, "crlf.srt", []byte(strings.ReplaceAll(srtShifted(0), "\n", "\r\n")))
	assert.Equal(t, 200, code, raw)
}

func TestSubtitleD7_NoInternalsInErrorBodies(t *testing.T) {
	h := newSubHarness(t)
	h.dbExec(`DROP TABLE subtitle_tracks`) // break the storage layer under the real handler
	code, js, raw := h.do("GET", fmt.Sprintf("/api/v1/subtitles/media/%d", h.media), "2", nil, "")
	assert.Equal(t, 500, code, raw)
	assert.Equal(t, "GET_SUBTITLES_FAILED", js["code"], raw)
	for _, leak := range []string{"subtitle_tracks", "no such table", "sql", "SELECT", h.dir} {
		assert.NotContains(t, strings.ToLower(raw), strings.ToLower(leak), raw)
	}
}

func TestSubtitleD7_TranslateWithoutProviderIsUnavailableNotSecondInternalError(t *testing.T) {
	h := newSubHarness(t)
	id := h.uploadOK("2", h.media, "a.srt", srtShifted(0))
	body, _ := json.Marshal(map[string]interface{}{"subtitle_id": id, "source_language": "en", "target_language": "de"})
	code, js, raw := h.do("POST", "/api/v1/subtitles/translate", "1", body, "application/json")
	assert.Equal(t, 503, code, raw)
	assert.Equal(t, "TRANSLATION_UNAVAILABLE", js["code"], raw)
	assert.NotContains(t, raw, "failed to translate lines")

	body, _ = json.Marshal(map[string]interface{}{"subtitle_id": "424242", "source_language": "en", "target_language": "de"})
	code, js, raw = h.do("POST", "/api/v1/subtitles/translate", "1", body, "application/json")
	assert.Equal(t, 404, code, raw)
	assert.Equal(t, "SUBTITLE_NOT_FOUND", js["code"], raw)
}

func TestSubtitleD8_ErrorBodiesCarryNoInternals(t *testing.T) {
	h := newSubHarness(t)
	_, _, raw := h.upload("2", 99999999, "a.srt", []byte(srtShifted(0)))
	for _, leak := range []string{"sql:", "no rows", "files", "failed to"} {
		assert.NotContains(t, raw, leak, raw)
	}
}

// ---- delete ----------------------------------------------------------------

func TestSubtitleDelete_OwnerAdminAndAuthz(t *testing.T) {
	h := newSubHarness(t)
	listLen := func() int {
		_, js, raw := h.do("GET", fmt.Sprintf("/api/v1/subtitles/media/%d", h.media), "2", nil, "")
		subs, _ := js["subtitles"].([]interface{})
		_ = raw
		return len(subs)
	}
	a := h.uploadOK("2", h.media, "a.srt", srtShifted(0)) // owned by user 2
	b := h.uploadOK("3", h.media, "b.srt", srtShifted(0)) // owned by user 3
	require.Equal(t, 2, listLen())

	code, _, raw := h.do("DELETE", "/api/v1/subtitles/"+a, "", nil, "")
	assert.Equal(t, 401, code, raw)
	code, js, raw := h.do("DELETE", "/api/v1/subtitles/"+a, "3", nil, "")
	assert.Equal(t, 404, code, "another non-admin user must not delete and must not learn the id exists: %s", raw)
	assert.Equal(t, "SUBTITLE_NOT_FOUND", js["code"], raw)
	assert.Equal(t, 2, listLen())

	code, _, raw = h.do("DELETE", "/api/v1/subtitles/"+a, "2", nil, "")
	assert.Equal(t, 204, code, "owner deletes: %s", raw)
	assert.Equal(t, 1, listLen())
	code, js, raw = h.do("DELETE", "/api/v1/subtitles/"+a, "2", nil, "")
	assert.Equal(t, 404, code, "second delete: %s", raw)
	assert.Equal(t, "SUBTITLE_NOT_FOUND", js["code"], raw)

	code, _, raw = h.do("DELETE", "/api/v1/subtitles/"+b, "1", nil, "") // admin, not the owner
	assert.Equal(t, 204, code, raw)
	assert.Equal(t, 0, listLen())

	code, js, raw = h.do("DELETE", "/api/v1/subtitles/not-a-number", "1", nil, "")
	assert.Equal(t, 404, code, raw)

	// ownerless (legacy / downloaded) rows are administrator-only
	h.dbExec(`INSERT INTO subtitle_tracks (media_item_id, language, language_code, source, format, content) VALUES (?, 'English','en','downloaded','srt', ?)`, h.media, srtShifted(0))
	_, js, _ = h.do("GET", fmt.Sprintf("/api/v1/subtitles/media/%d", h.media), "2", nil, "")
	legacy := fmt.Sprint(js["subtitles"].([]interface{})[0].(map[string]interface{})["id"])
	code, _, raw = h.do("DELETE", "/api/v1/subtitles/"+legacy, "2", nil, "")
	assert.Equal(t, 404, code, raw)
	code, _, raw = h.do("DELETE", "/api/v1/subtitles/"+legacy, "1", nil, "")
	assert.Equal(t, 204, code, raw)
}

func TestSubtitleD5_ServiceRejectsUnknownMediaDirectly(t *testing.T) {
	h := newSubHarness(t)
	content := srtShifted(0)
	_, err := h.svc.VerifySynchronization(context.Background(), 99999999, &services.SubtitleTrack{ID: "1", Format: "srt", Content: &content})
	assert.ErrorIs(t, err, services.ErrMediaNotFound)
}
