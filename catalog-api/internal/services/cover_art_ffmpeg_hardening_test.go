package services

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
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
	d, _ := s.getVideoDuration(playlist)
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

// SSRF-F5 (RED): ffprobe had no ctx/timeout; a FIFO named like a video blocks open() forever.
func TestGetVideoDuration_BlockedInputIsBoundedByTimeout(t *testing.T) {
	needFFTools(t)
	fifo := filepath.Join(t.TempDir(), "v.mp4")
	require.NoError(t, syscall.Mkfifo(fifo, 0o600))
	old := ffprobeTimeout
	ffprobeTimeout = 500 * time.Millisecond
	t.Cleanup(func() { ffprobeTimeout = old })
	// release any blocked reader at the end so a failing (unbounded) run does not leak a process
	t.Cleanup(func() {
		if f, err := os.OpenFile(fifo, os.O_WRONLY|syscall.O_NONBLOCK, 0); err == nil {
			f.Close()
		}
	})
	s := NewCoverArtService(nil, zap.NewNop())
	done := make(chan float64, 1)
	go func() { d, _ := s.getVideoDuration(fifo); done <- d }()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("getVideoDuration blocked on a FIFO: ffprobe has no timeout")
	}
}

// SSRF-F3: a DIRECT http URL as video path reaches a listener without the protocol whitelist
// (1 hit each for ffprobe and ffmpeg measured); with it, 0.
func TestCoverArtFFmpeg_DirectURLVideoPathDoesNotReachNetwork(t *testing.T) {
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
	s := NewCoverArtService(nil, zap.NewNop())
	s.cacheDir = t.TempDir()
	_, _ = s.getVideoDuration(url)
	_, _ = s.generateVideoThumbnail(context.Background(), &VideoThumbnailRequest{MediaItemID: 1, VideoPath: url}, 1, 0)
	time.Sleep(200 * time.Millisecond)
	assert.Zero(t, atomic.LoadInt32(&hits), "a URL named as video path must never be contacted")
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

// A frame extraction with a caller context that has no deadline is still bounded.
func TestGenerateVideoThumbnail_BlockedInputIsBoundedByTimeout(t *testing.T) {
	needFFTools(t)
	fifo := filepath.Join(t.TempDir(), "v.mp4")
	require.NoError(t, syscall.Mkfifo(fifo, 0o600))
	old := coverArtFFmpegTimeout
	coverArtFFmpegTimeout = 500 * time.Millisecond
	t.Cleanup(func() { coverArtFFmpegTimeout = old })
	t.Cleanup(func() {
		if f, err := os.OpenFile(fifo, os.O_WRONLY|syscall.O_NONBLOCK, 0); err == nil {
			f.Close()
		}
	})
	s := NewCoverArtService(nil, zap.NewNop())
	s.cacheDir = t.TempDir()
	done := make(chan struct{})
	go func() {
		_, _ = s.generateVideoThumbnail(context.Background(), &VideoThumbnailRequest{MediaItemID: 1, VideoPath: fifo}, 1, 0)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("generateVideoThumbnail blocked on a FIFO: ffmpeg has no deadline")
	}
}
