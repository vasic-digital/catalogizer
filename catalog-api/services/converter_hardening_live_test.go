package services

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"catalogizer/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Converter hardening (follow-up F1). TEST CLASS: live. These tests run the REAL ffmpeg and the
// REAL ImageMagick `convert` on crafted, user-controlled input through the service's own
// convertVideo / convertImage; the only listener is a plain loopback socket that records whether
// a tool connected to it (it answers nothing, it is the observation point, not a stand-in for
// the system under test). They are skipped honestly when a tool is absent.

func needTools(t *testing.T, tools ...string) {
	t.Helper()
	for _, tool := range tools {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("SKIP-with-reason: hardware_not_present - %s is not on PATH", tool)
		}
	}
}

// hitListener counts connections made to a loopback port.
type hitListener struct {
	ln   net.Listener
	mu   sync.Mutex
	hits []string
}

func newHitListener(t *testing.T) *hitListener {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	h := &hitListener{ln: ln}
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
				line, _ := bufio.NewReader(c).ReadString('\n')
				h.mu.Lock()
				h.hits = append(h.hits, strings.TrimSpace(line))
				h.mu.Unlock()
			}(c)
		}
	}()
	t.Cleanup(func() { ln.Close() })
	return h
}

func (h *hitListener) port() int { return h.ln.Addr().(*net.TCPAddr).Port }
func (h *hitListener) count() int {
	time.Sleep(300 * time.Millisecond)
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.hits)
}

func realFFmpeg(t *testing.T, args ...string) {
	t.Helper()
	out, err := exec.Command("ffmpeg", append([]string{"-nostdin", "-v", "error", "-y"}, args...)...).CombinedOutput()
	require.NoError(t, err, "ffmpeg %v: %s", args, out)
}

// outsideTS writes a real MPEG-TS clip at <outside>/secret.ts, i.e. outside the conversion root.
func outsideTS(t *testing.T, outside string) string {
	p := filepath.Join(outside, "secret.ts")
	realFFmpeg(t, "-f", "lavfi", "-i", "testsrc=d=1:s=64x64:r=10", "-c:v", "mpeg2video", "-f", "mpegts", p)
	return p
}

// F1 / HLS: a playlist inside the data root that names an absolute path outside it must not make
// ffmpeg read that file into the output.
func TestHardenLive_FFmpegHLSCannotReadOutsideRoot(t *testing.T) {
	needTools(t, "ffmpeg", "prlimit")
	root, outside, _ := rv6ConvFixture(t)
	secret := outsideTS(t, outside)
	pl := filepath.Join(root, "evil.m3u8")
	require.NoError(t, os.WriteFile(pl, []byte("#EXTM3U\n#EXT-X-VERSION:3\n#EXT-X-TARGETDURATION:1\n#EXTINF:1,\nfile://"+secret+"\n#EXT-X-ENDLIST\n"), 0o644))
	tgt := filepath.Join(root, "out.mp4")
	err := (&ConversionService{}).convertVideo(&models.ConversionJob{SourcePath: pl, TargetPath: tgt, Quality: "low"})
	fi, serr := os.Stat(tgt)
	t.Logf("convertVideo err=%v target-exists=%v", err, serr == nil)
	assert.Error(t, err, "an HLS playlist must be refused")
	assert.True(t, serr != nil || fi.Size() == 0, "the out-of-root clip was converted into the target (local file read)")
}

// F1 / concat: the concat demuxer is refused even for a harmless in-root segment (no playlist
// formats at all), so crafted concat files cannot be used for reads or probing.
func TestHardenLive_FFmpegConcatRefused(t *testing.T) {
	needTools(t, "ffmpeg", "prlimit")
	root, _, _ := rv6ConvFixture(t)
	realFFmpeg(t, "-f", "lavfi", "-i", "testsrc=d=1:s=64x64:r=10", "-c:v", "mpeg2video", "-f", "mpegts", filepath.Join(root, "seg.ts"))
	cc := filepath.Join(root, "list.mkv")
	require.NoError(t, os.WriteFile(cc, []byte("ffconcat version 1.0\nfile 'seg.ts'\n"), 0o644))
	tgt := filepath.Join(root, "out.mp4")
	err := (&ConversionService{}).convertVideo(&models.ConversionJob{SourcePath: cc, TargetPath: tgt, Quality: "low"})
	assert.Error(t, err, "a concat script disguised by its extension must be refused")
	_, serr := os.Stat(tgt)
	assert.Error(t, serr, "no output may be published")
}

// F1 / SVG through ImageMagick: an SVG that references another local file or a URL must not be
// rendered with that resource. (On hosts where ImageMagick delegates SVG to Inkscape this read the
// file and connected to the listener before the fix.)
func TestHardenLive_ImageMagickSVGExternalRefsRefused(t *testing.T) {
	needTools(t, "convert", "prlimit")
	root, outside, _ := rv6ConvFixture(t)
	victim := filepath.Join(outside, "victim.png")
	out, err := exec.Command("convert", "-size", "40x40", "xc:red", victim).CombinedOutput()
	require.NoError(t, err, "%s", out)
	h := newHitListener(t)
	for name, href := range map[string]string{
		"file": "file://" + victim,
		"abs":  victim,
		"http": fmt.Sprintf("http://127.0.0.1:%d/a.png", h.port()),
	} {
		t.Run(name, func(t *testing.T) {
			src := filepath.Join(root, name+".svg")
			svg := `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" width="40" height="40"><image width="40" height="40" xlink:href="` + href + `"/></svg>`
			require.NoError(t, os.WriteFile(src, []byte(svg), 0o644))
			tgt := filepath.Join(root, name+".png")
			cerr := (&ConversionService{}).convertImage(&models.ConversionJob{SourcePath: src, TargetPath: tgt})
			assert.Error(t, cerr, "an SVG with an external reference must be refused")
			_, serr := os.Stat(tgt)
			assert.Error(t, serr, "no output may be published")
		})
	}
	assert.Equal(t, 0, h.count(), "ImageMagick connected to the loopback listener")
}

// golden-FALSE: ordinary media still converts with the hardening in place.
func TestHardenLive_OrdinaryConversionsStillWork(t *testing.T) {
	needTools(t, "ffmpeg", "convert", "prlimit")
	root, _, _ := rv6ConvFixture(t)
	svc := &ConversionService{}

	// video: every supported input container that ffmpeg here can encode
	for _, c := range []struct{ ext, fmtName, vcodec string }{
		{"mp4", "mp4", "mpeg4"}, {"mkv", "matroska", "mpeg4"}, {"avi", "avi", "mpeg4"},
		{"mov", "mov", "mpeg4"}, {"webm", "webm", "libvpx"}, {"flv", "flv", "flv"}, {"3gp", "3gp", "h263"},
		{"wmv", "asf", "wmv2"}, {"m4v", "mp4", "mpeg4"},
	} {
		src := filepath.Join(root, "v."+c.ext)
		size := "64x64"
		if c.vcodec == "h263" {
			size = "176x144"
		}
		if out, err := exec.Command("ffmpeg", "-nostdin", "-v", "error", "-y", "-f", "lavfi", "-i", "testsrc=d=1:s="+size+":r=10",
			"-c:v", c.vcodec, "-f", c.fmtName, src).CombinedOutput(); err != nil {
			t.Logf("SKIP %s: this ffmpeg cannot make a fixture: %s", c.ext, strings.TrimSpace(string(out)))
			continue
		}
		tgt := filepath.Join(root, "o_"+c.ext+".mp4")
		err := svc.convertVideo(&models.ConversionJob{SourcePath: src, TargetPath: tgt, Quality: "low"})
		require.NoError(t, err, c.ext)
		fi, serr := os.Stat(tgt)
		require.NoError(t, serr, c.ext)
		assert.Greater(t, fi.Size(), int64(0), c.ext)
	}
	// audio
	for _, c := range []struct{ ext, codec string }{{"wav", "pcm_s16le"}, {"mp3", "libmp3lame"}, {"flac", "flac"}, {"ogg", "libvorbis"}, {"m4a", "aac"}, {"opus", "libopus"}, {"aac", "aac"}} {
		src := filepath.Join(root, "a."+c.ext)
		if out, err := exec.Command("ffmpeg", "-nostdin", "-v", "error", "-y", "-f", "lavfi", "-i", "sine=d=1", "-c:a", c.codec, src).CombinedOutput(); err != nil {
			t.Logf("SKIP %s: this ffmpeg cannot make a fixture: %s", c.ext, strings.TrimSpace(string(out)))
			continue
		}
		tgt := filepath.Join(root, "ao_"+c.ext+".wav")
		require.NoError(t, svc.convertAudio(&models.ConversionJob{SourcePath: src, TargetPath: tgt, Quality: "medium"}), c.ext)
		fi, serr := os.Stat(tgt)
		require.NoError(t, serr, c.ext)
		assert.Greater(t, fi.Size(), int64(0), c.ext)
	}
	// images: every supported input format
	for _, ext := range []string{"png", "jpg", "jpeg", "gif", "bmp", "tiff", "webp"} {
		src := filepath.Join(root, "i."+ext)
		if out, err := exec.Command("convert", "-size", "24x24", "xc:green", src).CombinedOutput(); err != nil {
			t.Logf("SKIP %s: %s", ext, strings.TrimSpace(string(out)))
			continue
		}
		tgt := filepath.Join(root, "io_"+ext+".png")
		require.NoError(t, svc.convertImage(&models.ConversionJob{SourcePath: src, TargetPath: tgt}), ext)
		fi, serr := os.Stat(tgt)
		require.NoError(t, serr, ext)
		assert.Greater(t, fi.Size(), int64(0), ext)
	}
	// an SVG with only internal references converts
	svg := filepath.Join(root, "ok.svg")
	require.NoError(t, os.WriteFile(svg, []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20"><defs><linearGradient id="g"><stop offset="0" stop-color="red"/></linearGradient></defs><rect width="20" height="20" fill="url(#g)"/></svg>`), 0o644))
	require.NoError(t, svc.convertImage(&models.ConversionJob{SourcePath: svg, TargetPath: filepath.Join(root, "ok_svg.png")}))
}
