package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Real-binary proof for the converter hardening (F1): the real server, the real ffmpeg, a real
// conversion job through the HTTP API. A normal job completes; a job whose source is an HLS
// playlist that names a file outside the conversion root is refused and no byte of that file
// reaches the output.
func TestConversionHardening_RealServerRealFFmpeg(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs the server binary")
	}
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("SKIP hardware/tool absent: ffmpeg is not installed on this host")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "catalog-api")
	if out, err := exec.Command("go", "build", "-buildvcs=false", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	b := &binEnv{t: t, bin: bin, dir: dir, port: port}
	const adminPw = "Op3rator-Supplied-Value-9x"
	cmd, done, out := b.start("GIN_MODE=release", "ADMIN_PASSWORD="+adminPw)
	require.True(t, b.waitHealthy(done), "server must come up:\n%s", out.String())
	defer b.stop(cmd, done)
	c := &apiClient{t: t, base: fmt.Sprintf("http://127.0.0.1:%d", port)}
	adm := c.login("admin", adminPw)

	data := filepath.Join(dir, "data")
	require.NoError(t, os.MkdirAll(data, 0o755))
	good := filepath.Join(data, "tone.wav")
	gen, err := exec.Command(ffmpeg, "-nostdin", "-v", "error", "-f", "lavfi", "-i", "sine=frequency=440:duration=1", "-y", good).CombinedOutput()
	require.NoError(t, err, "%s", gen)

	// outside the conversion root: a real MPEG-TS segment the playlist tries to pull in. Unhardened,
	// ffmpeg reads it and writes its audio into the target (verified RED by mutation).
	secret := filepath.Join(dir, "secret.ts")
	gen, err = exec.Command(ffmpeg, "-nostdin", "-v", "error", "-f", "lavfi", "-i", "sine=frequency=880:duration=1", "-c:a", "mp2", "-f", "mpegts", "-y", secret).CombinedOutput()
	require.NoError(t, err, "%s", gen)
	evil := filepath.Join(data, "evil.m3u8")
	require.NoError(t, os.WriteFile(evil, []byte("#EXTM3U\n#EXT-X-VERSION:3\n#EXT-X-TARGETDURATION:1\n#EXTINF:1,\nfile://"+secret+"\n#EXT-X-ENDLIST\n"), 0o644))

	submit := func(src, dst, sf string) map[string]interface{} {
		jid := c.id("POST", "/api/v1/conversion/jobs", adm, map[string]interface{}{
			"source_path": src, "target_path": dst, "source_format": sf, "target_format": "flac",
			"conversion_type": "audio", "quality": "medium", "priority": 5}, 200)
		var job map[string]interface{}
		deadline := time.Now().Add(60 * time.Second)
		for time.Now().Before(deadline) {
			code, raw := c.do("GET", fmt.Sprintf("/api/v1/conversion/jobs/%d", jid), adm, nil)
			require.Equal(t, 200, code, "%s", raw)
			job = map[string]interface{}{}
			require.NoError(t, json.Unmarshal(raw, &job))
			if job["status"] != "pending" && job["status"] != "running" {
				break
			}
			time.Sleep(500 * time.Millisecond)
		}
		return job
	}

	okOut := filepath.Join(data, "ok.flac")
	job := submit(good, okOut, "wav")
	assert.Equal(t, "completed", job["status"], "a normal job must complete: %v", job)
	if st, err := os.Stat(okOut); assert.NoError(t, err, "the converted file must exist") {
		assert.Greater(t, st.Size(), int64(0))
	}

	badOut := filepath.Join(data, "bad.flac")
	job = submit(evil, badOut, "mp3")
	t.Logf("HLS job: %v", job)
	assert.Equal(t, "failed", job["status"], "the HLS playlist job must be refused: %v", job)
	if st, err := os.Stat(badOut); err == nil {
		assert.Zero(t, st.Size(), "no content may be produced from the out-of-root file")
	}
}
