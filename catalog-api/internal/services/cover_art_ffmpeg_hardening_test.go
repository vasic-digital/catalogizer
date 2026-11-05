package services

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// Security queue (SSRF-F1..F5): the cover-art ffmpeg/ffprobe calls run on files the library
// scanner found. These tests drive the REAL ffmpeg/ffprobe binaries and the real service.

func needFFTools(t *testing.T) {
	t.Helper()
	for _, tool := range []string{"ffmpeg", "ffprobe"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s not installed", tool)
		}
	}
}

// makePlaylistToSecret builds a real MPEG-TS "secret" video OUTSIDE the library root and an
// HLS playlist inside it whose single segment is a file: URL naming the secret.
func makePlaylistToSecret(t *testing.T) (playlist, secret string) {
	t.Helper()
	secretDir, libDir := t.TempDir(), t.TempDir()
	secret = filepath.Join(secretDir, "secret.ts")
	out, err := exec.Command("ffmpeg", "-v", "error", "-f", "lavfi", "-i", "testsrc=duration=2:size=64x64:rate=5",
		"-f", "mpegts", "-y", secret).CombinedOutput()
	require.NoError(t, err, string(out))
	playlist = filepath.Join(libDir, "movie.m3u8")
	body := fmt.Sprintf("#EXTM3U\n#EXT-X-TARGETDURATION:3\n#EXTINF:2,\nfile:%s\n#EXT-X-ENDLIST\n", secret)
	require.NoError(t, os.WriteFile(playlist, []byte(body), 0o600))
	// positive control: the SAME ffprobe, without the cover-art guard, does follow the playlist
	raw, err := exec.Command("ffprobe", "-v", "error", "-protocol_whitelist", "file,pipe",
		"-show_entries", "format=duration", "-of", "default=nw=1:nk=1", playlist).Output()
	require.NoError(t, err, "control: an unrestricted-demuxer ffprobe must follow the playlist")
	require.Contains(t, string(raw), "2.0", "control: the playlist reads the secret segment")
	return playlist, secret
}

// SSRF-F5 (RED on the pre-fix tree): -protocol_whitelist alone still lets a library file that
// is really an HLS playlist pull a segment from ANY local path; the demuxer allow-list refuses it.
func TestCoverArtFFprobe_PlaylistToLocalFileIsRefused(t *testing.T) {
	needFFTools(t)
	playlist, _ := makePlaylistToSecret(t)
	out, err := ffprobeDurationCmd(playlist).Output()
	assert.Error(t, err, "ffprobe must refuse a non-container (HLS) input, got duration %q", string(out))
	s := NewCoverArtService(nil, zap.NewNop())
	d, _ := s.getVideoDuration(context.Background(), playlist)
	assert.Less(t, d, 1.0, "duration must not come from the secret segment (2.0 s)")
}

func TestCoverArtFFmpeg_PlaylistToLocalFileMakesNoThumbnail(t *testing.T) {
	needFFTools(t)
	playlist, _ := makePlaylistToSecret(t)
	out := filepath.Join(t.TempDir(), "o.jpg")
	_ = ffmpegFrameCmd(context.Background(), 0.5, playlist, out).Run()
	_, err := os.Stat(out)
	assert.True(t, os.IsNotExist(err), "no frame may be produced from a file the playlist names")
}

// shimTools puts executable stand-ins for ffprobe and ffmpeg that sleep far longer than any
// deadline in front of PATH. This is a UNIT test of the deadlines (a regular file cannot block
// the real tools); the real tools are exercised by the other tests in this file.
func shimTools(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, n := range []string{"ffprobe", "ffmpeg"} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, n), []byte("#!/bin/sh\nexec sleep 60\n"), 0o755))
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	f := filepath.Join(t.TempDir(), "v.mp4")
	require.NoError(t, os.WriteFile(f, []byte("not really a video"), 0o600))
	return f
}

func boundedWithin(t *testing.T, what string, fn func()) {
	t.Helper()
	done := make(chan struct{})
	go func() { fn(); close(done) }()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatalf("%s did not return: the tool has no deadline", what)
	}
}

// SSRF-F5 (RED on the pre-fix tree): ffprobe had no ctx/timeout.
func TestGetVideoDuration_HungToolIsBoundedByTimeout(t *testing.T) {
	f := shimTools(t)
	old := ffprobeTimeout
	ffprobeTimeout = 500 * time.Millisecond
	t.Cleanup(func() { ffprobeTimeout = old })
	s := NewCoverArtService(nil, zap.NewNop())
	boundedWithin(t, "getVideoDuration", func() { _, _ = s.getVideoDuration(context.Background(), f) })
}

func TestGenerateVideoThumbnail_HungToolIsBoundedByTimeout(t *testing.T) {
	f := shimTools(t)
	old := coverArtFFmpegTimeout
	coverArtFFmpegTimeout = 500 * time.Millisecond
	t.Cleanup(func() { coverArtFFmpegTimeout = old })
	s := NewCoverArtService(nil, zap.NewNop())
	s.cacheDir = t.TempDir()
	boundedWithin(t, "generateVideoThumbnail", func() {
		_, _ = s.generateVideoThumbnail(context.Background(), &VideoThumbnailRequest{MediaItemID: 1, VideoPath: f}, 1, 0)
	})
}

// F1: the production scan path (generateAndSaveVideoThumbnail) has its own deadline site.
func TestGenerateAndSaveVideoThumbnail_HungToolIsBoundedByTimeout(t *testing.T) {
	f := shimTools(t)
	oldP, oldF := ffprobeTimeout, coverArtFFmpegTimeout
	ffprobeTimeout, coverArtFFmpegTimeout = 200*time.Millisecond, 500*time.Millisecond
	t.Cleanup(func() { ffprobeTimeout, coverArtFFmpegTimeout = oldP, oldF })
	s := NewCoverArtService(nil, zap.NewNop())
	s.cacheDir = t.TempDir()
	boundedWithin(t, "generateAndSaveVideoThumbnail", func() {
		_, err := s.generateAndSaveVideoThumbnail(context.Background(), 7, f)
		require.Error(t, err)
	})
}

// SSRF-F3: the commands are built exactly as production builds them and handed an http:// URL
// directly (requireRegularFile would refuse such a path before any tool runs, so the command
// builders are driven here to keep the protocol whitelist under behavioural test). Without
// -protocol_whitelist the real tools connect to the listener (1 hit each measured); with it, 0.
func TestCoverArtFFCommands_DirectURLInputDoesNotReachNetwork(t *testing.T) {
	needFFTools(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()
	var hits int32
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			atomic.AddInt32(&hits, 1)
			c.Close()
		}
	}()
	url := fmt.Sprintf("http://127.0.0.1:%d/v.mp4", ln.Addr().(*net.TCPAddr).Port)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	_ = ffprobeDurationCmdContext(ctx, url).Run()
	_ = ffmpegFrameCmd(ctx, 1, url, filepath.Join(t.TempDir(), "o.jpg")).Run()
	time.Sleep(200 * time.Millisecond)
	assert.Zero(t, atomic.LoadInt32(&hits), "a URL given as input must never be contacted")

	// and the service entry points refuse the URL before any tool runs
	s := NewCoverArtService(nil, zap.NewNop())
	s.cacheDir = t.TempDir()
	_, derr := s.getVideoDuration(context.Background(), url)
	assert.Error(t, derr)
	_, terr := s.generateVideoThumbnail(context.Background(), &VideoThumbnailRequest{MediaItemID: 1, VideoPath: url}, 1, 0)
	assert.Error(t, terr)
}

func argIndex(args []string, v string) int {
	for i, a := range args {
		if a == v {
			return i
		}
	}
	return -1
}

// SSRF-F4 + F5: -nostdin on ffmpeg, -format_whitelist as an input option on both tools.
func TestCoverArtFFCommands_GuardArguments(t *testing.T) {
	ff := ffmpegFrameCmd(context.Background(), 1, "/x.mp4", "/o.jpg").Args
	pr := ffprobeDurationCmd("/x.mp4").Args
	assert.GreaterOrEqual(t, argIndex(ff, "-nostdin"), 0, "ffmpeg must not read stdin: %v", ff)
	for name, a := range map[string][]string{"ffmpeg": ff, "ffprobe": pr} {
		fw := argIndex(a, "-format_whitelist")
		require.GreaterOrEqual(t, fw, 0, "%s lacks -format_whitelist: %v", name, a)
		assert.Contains(t, a[fw+1], "mp4")
		assert.NotContains(t, a[fw+1], "hls", "%s must not allow the hls demuxer", name)
		assert.NotContains(t, a[fw+1], "concat")
		in := argIndex(a, "-i")
		if name == "ffprobe" {
			in = len(a) - 1 // ffprobe has no -i here: the path is the last argument
		}
		assert.Less(t, fw, in, "%s: -format_whitelist must precede the input: %v", name, a)
	}
}

// SSRF-F2: the production default of the cover download cap is 32 MiB (tests overwrite the variable).
func TestCoverArtMaxBytes_ProductionDefaultIs32MiB(t *testing.T) {
	assert.Equal(t, int64(32<<20), coverArtMaxBytes, "production default (other tests restore the variable)")
	assert.Equal(t, int64(32<<20), coverArtMaxBytesDefault)
}

// F3: a non-regular file (FIFO) in a library root must be skipped at once; it used to cost the
// full ffprobe timeout plus the ffmpeg timeout per item.
func TestNonRegularVideoPathIsSkippedQuickly(t *testing.T) {
	needFFTools(t)
	fifo := filepath.Join(t.TempDir(), "v.mp4")
	require.NoError(t, syscall.Mkfifo(fifo, 0o600))
	t.Cleanup(func() {
		if f, err := os.OpenFile(fifo, os.O_WRONLY|syscall.O_NONBLOCK, 0); err == nil {
			f.Close()
		}
	})
	s := NewCoverArtService(nil, zap.NewNop())
	s.cacheDir = t.TempDir()
	done := make(chan error, 1)
	go func() {
		_, err := s.generateAndSaveVideoThumbnail(context.Background(), 7, fifo)
		done <- err
	}()
	select {
	case err := <-done:
		require.Error(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("a FIFO video path was handed to ffprobe/ffmpeg instead of being skipped")
	}
}

// F6: the caller's context bounds the probe (a cancelled caller must not wait for ffprobeTimeout).
func TestGetVideoDuration_HonoursCallerContext(t *testing.T) {
	f := shimTools(t)
	s := NewCoverArtService(nil, zap.NewNop())
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, _ = s.getVideoDuration(ctx, f) // ffprobeTimeout stays at its 30 s default
	assert.Less(t, time.Since(start), 5*time.Second)
}

// F7: the cover-art demuxer list must stay identical to the converter's (the packages cannot
// import each other, so the converter source is parsed).
func TestCoverArtDemuxers_MatchConverterHardeningList(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("..", "..", "services", "converter_hardening.go"))
	require.NoError(t, err)
	text := string(src)
	start := strings.Index(text, "var ffmpegDemuxers = []string{")
	require.GreaterOrEqual(t, start, 0, "control: converter list found")
	end := strings.Index(text[start:], "\n}")
	require.Greater(t, end, 0)
	var want []string
	for _, m := range regexp.MustCompile(`"([a-z0-9_]+)"`).FindAllStringSubmatch(text[start:start+end], -1) {
		want = append(want, m[1])
	}
	require.NotEmpty(t, want)
	assert.Equal(t, want, coverArtDemuxers)
}

// G3: generateVideoThumbnail itself (not only the callers that probe the duration first)
// refuses a non-regular path, so a FIFO is never handed to ffmpeg.
func TestGenerateVideoThumbnail_RejectsNonRegularPath(t *testing.T) {
	needFFTools(t)
	fifo := filepath.Join(t.TempDir(), "v.mp4")
	require.NoError(t, syscall.Mkfifo(fifo, 0o600))
	t.Cleanup(func() {
		if f, err := os.OpenFile(fifo, os.O_WRONLY|syscall.O_NONBLOCK, 0); err == nil {
			f.Close()
		}
	})
	s := NewCoverArtService(nil, zap.NewNop())
	s.cacheDir = t.TempDir()
	done := make(chan error, 1)
	go func() {
		_, err := s.generateVideoThumbnail(context.Background(), &VideoThumbnailRequest{MediaItemID: 1, VideoPath: fifo}, 1, 0)
		done <- err
	}()
	select {
	case err := <-done:
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not a regular file")
	case <-time.After(3 * time.Second):
		t.Fatal("generateVideoThumbnail handed a FIFO to ffmpeg")
	}
}
