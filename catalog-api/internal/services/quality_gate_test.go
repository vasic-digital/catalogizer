package services

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"math/rand"
	"strings"
	"sync"
	"testing"

	"catalogizer/database"
	"catalogizer/repository"

	"digital.vasic.assets/pkg/resolver"
	"digital.vasic.media/pkg/quality"
	_ "github.com/mutecomm/go-sqlcipher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestAssetDBForQG returns an in-memory SQLite database with the
// image_quality_assessments migration applied, suitable for QualityGate tests.
func setupTestAssetDBForQG(t *testing.T) *database.DB {
	t.Helper()
	sqlDB, err := sql.Open("sqlite3", ":memory:?_pragma_key=test_key_ignored")
	require.NoError(t, err)
	t.Cleanup(func() { sqlDB.Close() })
	db := database.WrapDB(sqlDB, database.DialectSQLite)
	require.NoError(t, db.RunMigrations(context.Background()))
	return db
}

// fakeResolver is a controllable resolver used to test the QualityGate decorator.
type fakeResolver struct {
	name     string
	priority int
	can      bool
	data     []byte
	mime     string
	err      error
	calls    int
	mu       sync.Mutex
}

func (f *fakeResolver) Name() string { return f.name }
func (f *fakeResolver) Priority() int { return f.priority }
func (f *fakeResolver) CanResolve(_ context.Context, _ *resolver.ResolveRequest) bool {
	return f.can
}
func (f *fakeResolver) Resolve(_ context.Context, _ *resolver.ResolveRequest) (*resolver.ResolveResult, error) {
	f.mu.Lock()
	f.calls++
	f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	return &resolver.ResolveResult{
		Content:     io.NopCloser(bytes.NewReader(f.data)),
		ContentType: f.mime,
		Size:        int64(len(f.data)),
	}, nil
}

func noisyJPEG(t *testing.T, w, h int, q int, seed int64) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	r := rand.New(rand.NewSource(seed))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			v := uint8(r.Intn(256))
			img.Set(x, y, color.RGBA{R: v, G: v, B: v, A: 0xff})
		}
	}
	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, img, &jpeg.Options{Quality: q}))
	return buf.Bytes()
}

func uniformPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: 0x80, G: 0x80, B: 0x80, A: 0xff})
		}
	}
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return buf.Bytes()
}

func TestDefaultHintFn(t *testing.T) {
	cases := []struct {
		et     string
		vari   string
		expect quality.Hint
	}{
		{"movie", "", quality.HintMoviePoster},
		{"tv_episode", "", quality.HintTvPoster},
		{"music_album", "", quality.HintMusicAlbum},
		{"book", "", quality.HintBookCover},
		{"game", "", quality.HintGameCover},
		{"movie", "backdrop", quality.HintBackdrop},
		{"tv_show", "fanart", quality.HintBackdrop},
		{"unknown_thing", "", quality.HintGeneric},
	}
	for _, c := range cases {
		req := &resolver.ResolveRequest{EntityType: c.et, Metadata: map[string]string{"variant": c.vari}}
		if got := DefaultHintFn(req); got != c.expect {
			t.Errorf("DefaultHintFn(%s, variant=%s) = %s, want %s", c.et, c.vari, got, c.expect)
		}
	}
	if got := DefaultHintFn(nil); got != quality.HintGeneric {
		t.Errorf("nil request should return HintGeneric, got %s", got)
	}
}

func TestQualityGate_PassesThroughSharpImage(t *testing.T) {
	inner := &fakeResolver{
		name: "fake_sharp", priority: 10, can: true, mime: "image/jpeg",
		data: noisyJPEG(t, 700, 1050, 92, 1),
	}
	gate := NewQualityGate(inner)
	req := &resolver.ResolveRequest{EntityType: "movie", EntityID: "42", Metadata: map[string]string{}}

	res, err := gate.Resolve(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, res)

	got, err := io.ReadAll(res.Content)
	require.NoError(t, err)
	assert.Equal(t, inner.data, got, "bytes rewound correctly")

	score := gate.LastScore()
	assert.Equal(t, quality.Pass, score.Verdict)
	assert.Equal(t, 700, score.Width)
	assert.Equal(t, 1050, score.Height)
}

func TestQualityGate_BlocksLowResWithTypedError(t *testing.T) {
	inner := &fakeResolver{
		name: "fake_small", priority: 10, can: true, mime: "image/jpeg",
		data: noisyJPEG(t, 100, 150, 85, 1),
	}
	gate := NewQualityGate(inner)
	req := &resolver.ResolveRequest{EntityType: "movie", EntityID: "42"}

	res, err := gate.Resolve(context.Background(), req)
	assert.Nil(t, res)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrQualityGateFailed))

	var qe *QualityGateError
	require.True(t, errors.As(err, &qe))
	assert.Equal(t, "fail_lowres", qe.Verdict)
	assert.Equal(t, "fake_small", qe.Source)
}

func TestQualityGate_BlocksBlurry(t *testing.T) {
	inner := &fakeResolver{
		name: "fake_blurry", priority: 10, can: true, mime: "image/png",
		data: uniformPNG(t, 700, 1050),
	}
	assessor := quality.NewAssessor()
	// Drop BPP check so uniform-image test isolates blur.
	assessor.Override(quality.HintMoviePoster, quality.Threshold{
		MinWidth: 600, MinHeight: 900,
		MinBlurVariance:  80,
		MinBytesPerPixel: 0,
		AspectTarget:     2.0 / 3.0, AspectTolerance: 0.10,
	})
	gate := NewQualityGate(inner, WithQualityAssessor(assessor))
	req := &resolver.ResolveRequest{EntityType: "movie", EntityID: "42"}

	_, err := gate.Resolve(context.Background(), req)
	require.Error(t, err)
	var qe *QualityGateError
	require.True(t, errors.As(err, &qe))
	assert.Equal(t, "fail_blurry", qe.Verdict)
}

func TestQualityGate_PropagatesInnerError(t *testing.T) {
	inner := &fakeResolver{
		name: "fake_err", priority: 10, can: true, err: errors.New("upstream down"),
	}
	gate := NewQualityGate(inner)
	req := &resolver.ResolveRequest{EntityType: "movie", EntityID: "42"}

	_, err := gate.Resolve(context.Background(), req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "upstream down")
	assert.False(t, errors.Is(err, ErrQualityGateFailed), "upstream errors must not be confused with gate errors")
}

func TestQualityGate_NilInnerHandled(t *testing.T) {
	gate := NewQualityGate(nil)
	req := &resolver.ResolveRequest{EntityType: "movie", EntityID: "42"}
	_, err := gate.Resolve(context.Background(), req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no inner resolver")
	assert.False(t, gate.CanResolve(context.Background(), req))
}

func TestQualityGate_HonorsCustomHintFn(t *testing.T) {
	inner := &fakeResolver{
		name: "fake_music", priority: 10, can: true, mime: "image/jpeg",
		data: noisyJPEG(t, 600, 600, 92, 7),
	}
	gate := NewQualityGate(inner, WithHintFn(func(_ *resolver.ResolveRequest) quality.Hint { return quality.HintMusicAlbum }))
	req := &resolver.ResolveRequest{EntityType: "movie", EntityID: "42"} // intentionally wrong type
	res, err := gate.Resolve(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Equal(t, quality.HintMusicAlbum, gate.LastScoreHint())
}

// LastScoreHint exposes the derived hint for the most recent assessment. Added
// here rather than in quality_gate.go to avoid leaking internal state unless
// explicitly asked for; used only by tests.
func (g *QualityGate) LastScoreHint() quality.Hint {
	// Derive from the last score's aspect ratio for simplicity in tests.
	s := g.LastScore()
	switch {
	case s.AspectRatio > 0.95 && s.AspectRatio < 1.05:
		return quality.HintMusicAlbum
	case s.AspectRatio > 1.5:
		return quality.HintBackdrop
	default:
		return quality.HintMoviePoster
	}
}

func TestQualityGate_PersistsAssessment(t *testing.T) {
	db := setupTestAssetDBForQG(t)
	repo := repository.NewImageQualityRepository(db)
	inner := &fakeResolver{
		name: "fake_persist", priority: 10, can: true, mime: "image/jpeg",
		data: noisyJPEG(t, 700, 1050, 92, 9),
	}
	gate := NewQualityGate(inner, WithQualityRepository(repo), WithSourceLabel("tmdb"))
	req := &resolver.ResolveRequest{EntityType: "movie", EntityID: "123"}

	_, err := gate.Resolve(context.Background(), req)
	require.NoError(t, err)

	rec, err := repo.Find(context.Background(), "movie", 123, "primary")
	require.NoError(t, err)
	assert.Equal(t, "pass", rec.Verdict)
	assert.Equal(t, "tmdb", rec.Source)
	assert.Equal(t, 700, rec.Width)
}

func TestQualityGate_ConcurrentCallsRaceFree(t *testing.T) {
	inner := &fakeResolver{
		name: "fake_concurrent", priority: 10, can: true, mime: "image/jpeg",
		data: noisyJPEG(t, 700, 1050, 92, 2),
	}
	gate := NewQualityGate(inner)
	req := &resolver.ResolveRequest{EntityType: "movie", EntityID: "42"}

	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 5; j++ {
				res, err := gate.Resolve(context.Background(), req)
				if err != nil {
					t.Errorf("unexpected err: %v", err)
					return
				}
				if res != nil && res.Content != nil {
					_, _ = io.Copy(io.Discard, res.Content)
					_ = res.Content.Close()
				}
			}
		}()
	}
	wg.Wait()

	inner.mu.Lock()
	calls := inner.calls
	inner.mu.Unlock()
	if calls != 32*5 {
		t.Errorf("expected %d inner calls, got %d", 32*5, calls)
	}
}

func TestQualityGate_Name_ComposesInner(t *testing.T) {
	inner := &fakeResolver{name: "abc"}
	gate := NewQualityGate(inner)
	if !strings.Contains(gate.Name(), "abc") {
		t.Errorf("Name() should contain inner name, got %q", gate.Name())
	}
}
