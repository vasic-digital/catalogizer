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

// Real-binary proof for the converter-tool capability handling: with the
// converter tools absent from PATH (the old runtime image had no ffmpeg), a
// conversion job must end `failed` with an honest, tool-naming reason - never
// stay `pending` forever - and /health/deep must report which converters exist.
func TestConversionToolsMissing_RealBinary(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs the server binary")
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
	empty := t.TempDir() // a PATH with no converter tools in it
	cmd, done, out := b.start("GIN_MODE=release", "ADMIN_PASSWORD="+adminPw, "PATH="+empty)
	require.True(t, b.waitHealthy(done), "server must come up:\n%s", out.String())
	defer b.stop(cmd, done)
	c := &apiClient{t: t, base: fmt.Sprintf("http://127.0.0.1:%d", port)}
	adm := c.login("admin", adminPw)

	// source inside the conversion write root (./data under the server cwd)
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "data"), 0o755))
	src := filepath.Join(dir, "data", "in.mp3")
	require.NoError(t, os.WriteFile(src, []byte("not really audio"), 0o644))
	jid := c.id("POST", "/api/v1/conversion/jobs", adm, map[string]interface{}{
		"source_path": src, "target_path": filepath.Join(dir, "data", "out.wav"),
		"source_format": "mp3", "target_format": "wav", "conversion_type": "audio", "quality": "medium", "priority": 5}, 200)

	var job map[string]interface{}
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		code, raw := c.do("GET", fmt.Sprintf("/api/v1/conversion/jobs/%d", jid), adm, nil)
		require.Equal(t, 200, code, "%s", raw)
		job = map[string]interface{}{}
		require.NoError(t, json.Unmarshal(raw, &job))
		if job["status"] != "pending" && job["status"] != "running" {
			break
		}
		time.Sleep(time.Second)
	}
	assert.Equal(t, "failed", job["status"], "job must reach a terminal state, got %v", job)
	assert.Contains(t, fmt.Sprint(job["error_message"]), "ffmpeg", "the failure must name the missing tool: %v", job)
	assert.Contains(t, fmt.Sprint(job["error_message"]), "not installed", "%v", job)

	// capabilities are reported from real lookups
	code, raw := c.do("GET", "/health/deep", "", nil)
	assert.Contains(t, []int{200, 503}, code)
	var h map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &h), "%s", raw)
	caps, ok := h["converters"].(map[string]interface{})
	require.True(t, ok, "/health/deep must carry a converters object: %s", raw)
	for _, tool := range []string{"ffmpeg", "convert", "convert_pdf", "convert_svg", "pdftotext"} {
		assert.Equal(t, false, caps[tool], "%s is not on this PATH: %s", tool, raw)
	}
}
