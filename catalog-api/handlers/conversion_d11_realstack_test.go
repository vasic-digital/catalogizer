package handlers

import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// LIVE-CLASS (§11.4.289): real gin routes, production migrations, real ffmpeg/ffprobe, real media.
// D11a (docs/qa/live-conv-docs-subs-r1-20261104.md): a truncated source converted to `completed`
// with a short output and said nothing.

func needTool(t *testing.T, names ...string) {
	t.Helper()
	for _, n := range names {
		if _, err := exec.LookPath(n); err != nil {
			t.Skipf("%s absent: cannot generate/validate the media independently (hardware_not_present-class SKIP)", n)
		}
	}
}

// makeMP4 writes a 20 s mp4 (moov atom first, so a prefix still opens and declares 20 s).
func makeMP4(t *testing.T, p string) {
	t.Helper()
	out, err := exec.Command("ffmpeg", "-v", "error", "-y", "-f", "lavfi", "-i", "testsrc=d=20:s=160x120:r=10",
		"-f", "lavfi", "-i", "sine=d=20", "-c:v", "libx264", "-c:a", "aac", "-movflags", "+faststart", p).CombinedOutput()
	require.NoError(t, err, string(out))
}

func truncateFile(t *testing.T, src, dst string, frac float64) {
	t.Helper()
	b, err := os.ReadFile(src)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(dst, b[:int(float64(len(b))*frac)], 0o600))
}

func probeDur(t *testing.T, p string) float64 {
	t.Helper()
	out, err := exec.Command("ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "csv=p=0", p).Output()
	require.NoError(t, err)
	f, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	require.NoError(t, err)
	return f
}

func runVideoJob(t *testing.T, s *convStack, src, tgt string) map[string]interface{} {
	t.Helper()
	code, body := s.do(t, "POST", "/jobs", map[string]interface{}{"source_path": src, "target_path": tgt,
		"source_format": "mp4", "target_format": "webm", "conversion_type": "video", "quality": "low"})
	require.Equal(t, http.StatusOK, code, string(body))
	var job struct{ ID int }
	require.NoError(t, json.Unmarshal(body, &job))
	require.NoError(t, s.svc.StartConversion(job.ID))
	return pollJob(t, s, job.ID, "completed", "failed")
}

func TestD11a_TruncatedSourceIsFlaggedNotSilentlyCompleted(t *testing.T) {
	needTool(t, "ffmpeg", "ffprobe")
	s := newConvStack(t)
	full := filepath.Join(s.dataDir, "full.mp4")
	makeMP4(t, full)
	trunc := filepath.Join(s.dataDir, "trunc.mp4")
	truncateFile(t, full, trunc, 0.33)
	tgt := filepath.Join(s.dataDir, "trunc.webm")

	m := runVideoJob(t, s, trunc, tgt)
	require.Equal(t, "completed", m["status"], "error_message: %v", m["error_message"])
	// independent oracle: the output really is much shorter than the 20 s the source declares
	assert.Less(t, probeDur(t, tgt), 12.0)
	w, ok := m["warning"].(string)
	require.True(t, ok, "a truncated source must carry a warning on the job: %v", m)
	assert.Contains(t, w, "shorter")
	assert.NotContains(t, w, s.dataDir, "the warning must not leak server paths")
}

func TestD11a_IntactSourceHasNoWarning(t *testing.T) {
	needTool(t, "ffmpeg", "ffprobe")
	s := newConvStack(t)
	full := filepath.Join(s.dataDir, "full.mp4")
	makeMP4(t, full)
	m := runVideoJob(t, s, full, filepath.Join(s.dataDir, "full.webm"))
	require.Equal(t, "completed", m["status"], "error_message: %v", m["error_message"])
	_, has := m["warning"]
	assert.False(t, has, "an intact conversion must carry no warning: %v", m)
	assert.InDelta(t, 20.0, probeDur(t, filepath.Join(s.dataDir, "full.webm")), 1.0)
}
