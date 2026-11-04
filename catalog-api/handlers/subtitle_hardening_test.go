package handlers

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Review round 1 tests (F1 bounded analysis, F2 media-resolution confinement, F3 route table and
// ambiguity end-to-end). Real ffmpeg media, real router, production sqlite schema.

func copyFile(t *testing.T, src, dst string) {
	t.Helper()
	in, err := os.Open(src)
	require.NoError(t, err)
	defer in.Close()
	require.NoError(t, os.MkdirAll(filepath.Dir(dst), 0o755))
	out, err := os.Create(dst)
	require.NoError(t, err)
	defer out.Close()
	_, err = io.Copy(out, in)
	require.NoError(t, err)
}

func (h *subHarness) verify(user, sub string, media int64) (int, map[string]interface{}, string) {
	return h.do("GET", fmt.Sprintf("/api/v1/subtitles/%s/verify-sync/%d", sub, media), user, nil, "")
}

// ---- F1 ---------------------------------------------------------------------

func TestSubtitleF1_ConcurrentVerifyIsBoundedAndCacheHitSpawnsNothing(t *testing.T) {
	h := newSubHarness(t)
	h.svc.SetSyncConcurrency(1)
	src := filepath.Join(h.dir, "movie.mkv")
	const n = 8
	ids := make([]int64, n)
	for i := 0; i < n; i++ {
		name := fmt.Sprintf("copy%d.mkv", i)
		copyFile(t, src, filepath.Join(h.dir, name))
		ids[i] = h.addFile(h.rootID, "/"+name, name)
	}
	sub := make([]string, n)
	for i := range ids {
		sub[i] = h.uploadOK("2", ids[i], "a.srt", srtShifted(0))
	}
	before := h.svc.AnalysisRuns()

	var wg sync.WaitGroup
	start := make(chan struct{})
	codes := make([]int, n)
	bodies := make([]string, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			codes[i], _, bodies[i] = h.verify("2", sub[i], ids[i])
		}(i)
	}
	close(start)
	wg.Wait()

	ok, busy := 0, 0
	for i, c := range codes {
		switch c {
		case 200:
			ok++
		case 429:
			busy++
			assert.Contains(t, bodies[i], "SYNC_BUSY")
			assert.NotContains(t, bodies[i], "ffmpeg")
			assert.NotContains(t, bodies[i], "exec")
		default:
			t.Errorf("unexpected status %d: %s", c, bodies[i])
		}
	}
	assert.GreaterOrEqual(t, ok, 1, "the request that holds the slot must complete")
	assert.GreaterOrEqual(t, busy, 1, "requests beyond the limit must be refused fast (429), not run in parallel")
	assert.Equal(t, int64(ok), h.svc.AnalysisRuns()-before, "exactly one analysis per completed request, never more than the limit at once")

	// serial retry of a request that got 429 now succeeds; the second call on the same media is cached
	h.svc.SetSyncConcurrency(2)
	for i, c := range codes {
		if c == 429 {
			code, _, raw := h.verify("2", sub[i], ids[i])
			require.Equal(t, 200, code, raw)
		}
	}
	runs := h.svc.AnalysisRuns()
	code, js, raw := h.verify("2", sub[0], ids[0])
	require.Equal(t, 200, code, raw)
	assert.Equal(t, "measured", syncOf(js)["status"], raw)
	assert.Equal(t, runs, h.svc.AnalysisRuns(), "a cache hit must not spawn ffmpeg")
}

// ---- F2 ---------------------------------------------------------------------

func TestSubtitleF2_DashLeadingFileNameIsMeasuredNotReadAsOption(t *testing.T) {
	h := newSubHarness(t)
	copyFile(t, filepath.Join(h.dir, "movie.mkv"), filepath.Join(h.dir, "-version.mkv"))
	id := h.addFile(h.rootID, "/-version.mkv", "-version.mkv")
	sub := h.uploadOK("2", id, "a.srt", srtShifted(7))
	code, js, raw := h.verify("2", sub, id)
	require.Equal(t, 200, code, raw)
	sr := syncOf(js)
	assert.Equal(t, "measured", sr["status"], raw)
	assert.InDelta(t, 7000, sr["sync_offset"].(float64), 300, raw)
}

func TestSubtitleF2_RemoteProtocolAndEscapesAreNeverReadFromTheHost(t *testing.T) {
	h := newSubHarness(t)
	outside := t.TempDir()
	copyFile(t, filepath.Join(h.dir, "movie.mkv"), filepath.Join(outside, "movie.mkv"))

	remote := h.addRoot("smb", h.dir) // a remote-protocol root whose path happens to exist on the host
	remoteID := h.addFile(remote, "/movie.mkv", "movie.mkv")
	escID := h.addFile(h.rootID, "/../"+filepath.Base(outside)+"/movie.mkv", "movie.mkv")
	require.NoError(t, os.Symlink(filepath.Join(outside, "movie.mkv"), filepath.Join(h.dir, "link.mkv")))
	linkID := h.addFile(h.rootID, "/link.mkv", "link.mkv")

	for name, id := range map[string]int64{"remote protocol": remoteID, "../ escape": escID, "symlink escape": linkID} {
		sub := h.uploadOK("2", id, "a.srt", srtShifted(0))
		code, js, raw := h.verify("2", sub, id)
		require.Equal(t, 200, code, name+": "+raw)
		assert.Equal(t, "undetermined", syncOf(js)["status"], "%s must not be analysed from the host filesystem: %s", name, raw)
	}
}

// ---- F3 ---------------------------------------------------------------------

// genPeriodicMedia makes 150 s of audio that is strictly periodic (2 s tone every 5 s): every
// shift by a multiple of 5 s aligns equally well, so no unambiguous offset exists.
func genPeriodicMedia(t *testing.T, path string) {
	t.Helper()
	cmd := exec.Command("ffmpeg", "-nostdin", "-v", "error", "-y",
		"-f", "lavfi", "-i", "testsrc=size=64x64:rate=5:duration=150",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=150",
		"-af", "volume='gt(lt(mod(t,5),2),0)':eval=frame",
		"-c:v", "mpeg4", "-c:a", "aac", "-shortest", path)
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("ffmpeg periodic media generation failed: %v: %s", err, b)
	}
}

func TestSubtitleF3_AmbiguousPeriodicAudioIsUndeterminedNotMeasured(t *testing.T) {
	h := newSubHarness(t)
	genPeriodicMedia(t, filepath.Join(h.dir, "periodic.mkv"))
	id := h.addFile(h.rootID, "/periodic.mkv", "periodic.mkv")
	var b strings.Builder
	for i := 0; i < 28; i++ {
		fmt.Fprintf(&b, "%d\n%s --> %s\nLine number %d here\n\n", i+1, ts(float64(i*5), true), ts(float64(i*5)+2, true), i+1)
	}
	sub := h.uploadOK("2", id, "p.srt", b.String())
	code, js, raw := h.verify("2", sub, id)
	require.Equal(t, 200, code, raw)
	sr := syncOf(js)
	assert.Equal(t, "undetermined", sr["status"], "a periodic pattern has no unambiguous alignment: %s", raw)
	assert.Equal(t, 0.0, sr["confidence"], raw)
	assert.Equal(t, 0.0, sr["sync_offset"], raw)
}

func TestSubtitleF3_RealRouteTableAndAdminGate(t *testing.T) {
	h := newSubHarness(t)
	var got []string
	for _, r := range h.r.Routes() {
		got = append(got, r.Method+" "+r.Path)
	}
	sort.Strings(got)
	assert.Equal(t, []string{
		"DELETE /api/v1/subtitles/:subtitle_id",
		"GET /api/v1/subtitles/:subtitle_id/verify-sync/:media_id",
		"GET /api/v1/subtitles/languages",
		"GET /api/v1/subtitles/media/:media_id",
		"GET /api/v1/subtitles/providers",
		"GET /api/v1/subtitles/search",
		"POST /api/v1/subtitles/download",
		"POST /api/v1/subtitles/translate",
		"POST /api/v1/subtitles/upload",
	}, got)

	// admin-gated routes refuse a normal user, the others do not
	for _, rt := range [][2]string{{"GET", "/api/v1/subtitles/search"}, {"POST", "/api/v1/subtitles/download"}, {"POST", "/api/v1/subtitles/translate"}} {
		code, _, raw := h.do(rt[0], rt[1], "2", []byte(`{}`), "application/json")
		assert.Equal(t, 403, code, "%s %s must be admin-gated: %s", rt[0], rt[1], raw)
	}
	for _, rt := range [][2]string{{"GET", "/api/v1/subtitles/languages"}, {"GET", "/api/v1/subtitles/providers"}} {
		code, _, raw := h.do(rt[0], rt[1], "2", nil, "")
		assert.NotEqual(t, 403, code, "%s %s must not be admin-gated: %s", rt[0], rt[1], raw)
	}
}
