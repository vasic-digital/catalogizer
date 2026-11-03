package services

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"catalogizer/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- argument construction (unit class) -----------------------------------------------------

func TestHardenArgs_FFmpegGuard(t *testing.T) {
	got := guardToolArgs("ffmpeg", []string{"-i", "in.mp4", "out.mp4"})
	joined := " " + strings.Join(got, " ") + " "
	assert.Contains(t, joined, " -nostdin ")
	assert.Contains(t, joined, " -protocol_whitelist file,pipe ")
	assert.Equal(t, []string{"-i", "in.mp4", "out.mp4"}, got[len(got)-3:], "the caller's arguments stay last and unchanged")
	// -i must come after every guard option (input options apply to the NEXT -i)
	for i, a := range got {
		if a == "-i" {
			assert.Greater(t, i, indexOf(got, "-format_whitelist"))
			assert.Greater(t, i, indexOf(got, "-protocol_whitelist"))
		}
	}
	var wl string
	for i, a := range got {
		if a == "-format_whitelist" {
			wl = got[i+1]
		}
	}
	for _, bad := range []string{"hls", "concat", "dash", "sdp", "rtsp", "rtp", "lavfi", "image2", "applehttp", "smoothstreaming"} {
		for _, d := range strings.Split(wl, ",") {
			assert.NotEqual(t, bad, d, "playlist/network demuxer %q must not be whitelisted", bad)
		}
	}
	assert.Equal(t, []string{"a", "b"}, guardToolArgs("convert", []string{"a", "b"}), "ImageMagick args are not prefixed")
}

func indexOf(s []string, v string) int {
	for i, x := range s {
		if x == v {
			return i
		}
	}
	return -1
}

func TestHardenPolicy_DeniesActiveCoders(t *testing.T) {
	p := string(ImageMagickPolicy())
	for _, c := range []string{"MVG", "MSL", "URL", "HTTPS", "HTTP", "FTP", "EPHEMERAL", "TEXT", "LABEL"} {
		assert.Contains(t, p, `domain="coder" rights="none" pattern="`+c+`"`, c)
	}
	assert.Contains(t, p, `domain="path" rights="none" pattern="@*"`)
	for _, r := range []string{"memory", "map", "disk", "time"} {
		assert.Contains(t, p, `domain="resource" name="`+r+`"`, r)
	}
	assert.NotContains(t, p, `pattern="PDF"`, "PDF stays readable (forced pdf: coder, Ghostscript -dSAFER)")
}

func TestHardenSVG_Check(t *testing.T) {
	ok := map[string]string{
		"plain":    `<svg xmlns="http://www.w3.org/2000/svg" width="5" height="5"><rect width="5" height="5"/></svg>`,
		"fragment": `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink"><defs><linearGradient id="g"/></defs><use xlink:href="#g"/><rect fill="url(#g)"/></svg>`,
		"datauri":  `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink"><image xlink:href="data:image/png;base64,iVBORw0KGgo="/></svg>`,
		"style":    `<svg xmlns="http://www.w3.org/2000/svg"><style>.a{fill:url(#g)}</style><rect class="a"/></svg>`,
		"link":     `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink"><a xlink:href="https://example.org/x"><rect/></a></svg>`,
	}
	for name, doc := range ok {
		assert.NoError(t, checkSVGBytes([]byte(doc)), "golden-FALSE %s must be accepted", name)
	}
	bad := map[string]string{
		"file href":      `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink"><image xlink:href="file:///etc/hostname"/></svg>`,
		"plain href":     `<svg xmlns="http://www.w3.org/2000/svg"><image href="/etc/passwd"/></svg>`,
		"http href":      `<svg xmlns="http://www.w3.org/2000/svg"><image href="http://127.0.0.1:1/x"/></svg>`,
		"data svg":       `<svg xmlns="http://www.w3.org/2000/svg"><image href="data:image/svg+xml;base64,AAAA"/></svg>`,
		"doctype":        `<!DOCTYPE svg [<!ENTITY x SYSTEM "file:///etc/passwd">]><svg xmlns="http://www.w3.org/2000/svg"><text>&x;</text></svg>`,
		"script":         `<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`,
		"foreignObject":  `<svg xmlns="http://www.w3.org/2000/svg"><foreignObject><iframe src="x"/></foreignObject></svg>`,
		"url attr":       `<svg xmlns="http://www.w3.org/2000/svg"><rect fill="url(http://127.0.0.1/x)"/></svg>`,
		"url style attr": `<svg xmlns="http://www.w3.org/2000/svg"><rect style="fill:url('file:///x')"/></svg>`,
		"style import":   `<svg xmlns="http://www.w3.org/2000/svg"><style>@import url(http://x/y.css);</style></svg>`,
		"style url":      `<svg xmlns="http://www.w3.org/2000/svg"><style>.a{background:url(/etc/passwd)}</style></svg>`,
		"stylesheet pi":  `<?xml-stylesheet href="http://x/s.css"?><svg xmlns="http://www.w3.org/2000/svg"/>`,
		"event handler":  `<svg xmlns="http://www.w3.org/2000/svg" onload="x()"/>`,
		"upper case":     `<svg xmlns="http://www.w3.org/2000/svg"><IMAGE HREF="file:///etc/hostname"/></svg>`,
		"not xml":        `push graphic-context`,
		"empty":          ``,
	}
	for name, doc := range bad {
		assert.Error(t, checkSVGBytes([]byte(doc)), "%s must be refused", name)
	}
}

func TestHardenCoder_ForcedFromExtension(t *testing.T) {
	for in, want := range map[string]string{"/d/a.PNG": "png:", "/d/a.jpg": "jpeg:", "/d/a.jpeg": "jpeg:", "/d/a.gif": "gif:",
		"/d/a.bmp": "bmp:", "/d/a.tiff": "tiff:", "/d/a.webp": "webp:", "/d/a.pdf": "pdf:", "/d/a.svg": "svg:"} {
		got, err := imageMagickCoder(in)
		require.NoError(t, err, in)
		assert.Equal(t, want, got, in)
	}
	for _, in := range []string{"/d/a.mvg", "/d/a.msl", "/d/a.txt", "/d/noext", "/d/a.ps", "/d/a.url"} {
		_, err := imageMagickCoder(in)
		assert.Error(t, err, in)
	}
}

func TestHardenCommand_NoPrlimitRefuses(t *testing.T) {
	old := lookPath
	lookPath = func(f string) (string, error) {
		if f == "prlimit" {
			return "", errors.New("not found")
		}
		return old(f)
	}
	defer func() { lookPath = old }()
	needTools(t, "ffmpeg")
	err := runConverterTool("ffmpeg", []string{"-version"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "prlimit")
	assert.True(t, isConverterToolError(err))
}

func TestHardenCommand_MissingToolNamed(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	err := runConverterTool("ffmpeg", []string{"-version"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"ffmpeg" is not installed`)
}

func TestHardenPDF_MissingGhostscriptSaysSo(t *testing.T) {
	old := lookPath
	lookPath = func(f string) (string, error) {
		if f == "gs" {
			return "", errors.New("not found")
		}
		return old(f)
	}
	defer func() { lookPath = old }()
	err := (&ConversionService{}).convertPDFWithImageMagick(&models.ConversionJob{SourcePath: "/x.pdf", TargetPath: "/y.bmp"}, "bmp")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Ghostscript")
	assert.Contains(t, err.Error(), "png or jpg")
}

// --- live: real ImageMagick under the embedded policy ---------------------------------------

func TestHardenLive_PolicyDeniesActiveCodersInRealImageMagick(t *testing.T) {
	needTools(t, "convert", "prlimit")
	root, outside, _ := rv6ConvFixture(t)
	victim := filepath.Join(outside, "victim.png")
	out, err := exec.Command("convert", "-size", "20x20", "xc:red", victim).CombinedOutput()
	require.NoError(t, err, "%s", out)
	h := newHitListener(t)
	mvg := filepath.Join(root, "e.mvg")
	require.NoError(t, os.WriteFile(mvg, []byte("push graphic-context\nviewbox 0 0 20 20\nimage Over 0,0 20,20 '"+victim+"'\npop graphic-context\n"), 0o644))
	msl := filepath.Join(root, "e.msl")
	require.NoError(t, os.WriteFile(msl, []byte(`<?xml version="1.0"?><image><read filename="`+victim+`"/><write filename="`+filepath.Join(root, "msl_out.png")+`"/></image>`), 0o644))
	list := filepath.Join(root, "list.txt")
	require.NoError(t, os.WriteFile(list, []byte(victim+"\n"), 0o644))
	for name, in := range map[string]string{
		"mvg":   "mvg:" + mvg,
		"msl":   "msl:" + msl,
		"text":  "text:" + list,
		"label": "label:hello",
		"url":   fmt.Sprintf("url:http://127.0.0.1:%d/a.png", h.port()),
		"http":  fmt.Sprintf("http://127.0.0.1:%d/a.png", h.port()),
		"@list": "@" + list,
	} {
		o := filepath.Join(root, "o_"+name+".png")
		err := runConverterTool("convert", []string{in, o})
		assert.Error(t, err, "%s must be refused by the policy", name)
		_, serr := os.Stat(o)
		assert.Error(t, serr, "%s produced output", name)
	}
	_, serr := os.Stat(filepath.Join(root, "msl_out.png"))
	assert.Error(t, serr, "the MSL script wrote a file")
	assert.Equal(t, 0, h.count())
	// golden-FALSE: an ordinary conversion under the same policy works
	require.NoError(t, runConverterTool("convert", []string{"png:" + victim, filepath.Join(root, "ok.jpg")}))
	// a mislabelled source is not decoded as MVG/MSL through the service either
	poly := filepath.Join(root, "poly.png")
	require.NoError(t, os.WriteFile(poly, []byte(`<?xml version="1.0"?><image><read filename="`+victim+`"/><write filename="`+filepath.Join(root, "poly_out.png")+`"/></image>`), 0o644))
	assert.Error(t, (&ConversionService{}).convertImage(&models.ConversionJob{SourcePath: poly, TargetPath: filepath.Join(root, "p.jpg")}))
	_, serr = os.Stat(filepath.Join(root, "poly_out.png"))
	assert.Error(t, serr, "a PNG-named MSL file was executed")
	// .mvg/.msl sources are refused before any tool runs
	err = (&ConversionService{}).convertImage(&models.ConversionJob{SourcePath: mvg, TargetPath: filepath.Join(root, "m.png")})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a supported image input")
}

func TestHardenLive_PDFRasterisesThroughGhostscriptWhenPresent(t *testing.T) {
	needTools(t, "convert", "gs", "prlimit")
	// Probe with the stock ImageMagick (no hardening involved): when it cannot rasterise a PDF on
	// this host, the host's ImageMagick/Ghostscript pair is broken (measured here: gs 10.06 with
	// IM 7.1.2-18 fails with /undefinedfilename), which says nothing about the hardening.
	probeDir := t.TempDir()
	if out, err := exec.Command("convert", "-size", "8x8", "xc:red", filepath.Join(probeDir, "p.pdf")).CombinedOutput(); err != nil {
		t.Skipf("SKIP-with-reason: topology_unsupported - cannot make a probe PDF: %s", strings.TrimSpace(string(out)))
	}
	if out, err := exec.Command("convert", "pdf:"+filepath.Join(probeDir, "p.pdf"), filepath.Join(probeDir, "p.bmp")).CombinedOutput(); err != nil {
		t.Skipf("SKIP-with-reason: topology_unsupported - stock ImageMagick cannot rasterise a PDF on this host: %s", strings.TrimSpace(string(out)))
	}
	root, _, src := rv6ConvFixture(t)
	tgt := filepath.Join(root, "page.bmp")
	require.NoError(t, (&ConversionService{}).convertPDFWithImageMagick(&models.ConversionJob{SourcePath: src, TargetPath: tgt}, "bmp"))
	fi, err := os.Stat(tgt)
	if err != nil { // multi-page input names derived outputs page-0.bmp
		m, _ := filepath.Glob(filepath.Join(root, "page-*.bmp"))
		require.NotEmpty(t, m)
		fi, err = os.Stat(m[0])
	}
	require.NoError(t, err)
	assert.Greater(t, fi.Size(), int64(0))
}

// --- live: limits, process group, hard timeout ---------------------------------------------

// findProcByArg returns the pids whose command line contains token (this test's unique output name).
func findProcByArg(token string) []int {
	var pids []int
	ents, _ := os.ReadDir("/proc")
	for _, e := range ents {
		var pid int
		if _, err := fmt.Sscanf(e.Name(), "%d", &pid); err != nil {
			continue
		}
		b, err := os.ReadFile("/proc/" + e.Name() + "/cmdline")
		if err == nil && strings.Contains(string(b), token) {
			pids = append(pids, pid)
		}
	}
	return pids
}

func slowFFmpegJob(t *testing.T, root string) (src, tgt string) {
	t.Helper()
	src = filepath.Join(root, "long.mkv")
	realFFmpeg(t, "-f", "lavfi", "-i", "testsrc2=d=20:s=640x480:r=25", "-c:v", "mpeg4", "-f", "matroska", src)
	return src, filepath.Join(root, fmt.Sprintf("slow_%d.mp4", time.Now().UnixNano()))
}

func TestHardenLive_LimitsAndTimeoutKillOnRealFFmpeg(t *testing.T) {
	needTools(t, "ffmpeg", "prlimit")
	if _, err := os.Stat("/proc/self/limits"); err != nil {
		t.Skip("SKIP-with-reason: topology_unsupported - no /proc")
	}
	root, _, _ := rv6ConvFixture(t)
	src, tgt := slowFFmpegJob(t, root)
	token := filepath.Base(tgt)
	done := make(chan error, 1)
	start := time.Now()
	go func() {
		ctx, cancel := ctxWithTimeout(2 * time.Second)
		defer cancel()
		// veryslow lossless x264 on 20 s of 640x480 cannot finish in 2 s
		done <- runConverterToolCtx(ctx, "ffmpeg", []string{"-i", src, "-y", "-c:v", "libx264", "-crf", "0", "-preset", "veryslow", tgt})
	}()
	var pids []int
	for i := 0; i < 40 && len(pids) == 0; i++ {
		time.Sleep(50 * time.Millisecond)
		pids = findProcByArg(token)
	}
	require.NotEmpty(t, pids, "the tool process was never seen")
	lim, err := os.ReadFile(fmt.Sprintf("/proc/%d/limits", pids[len(pids)-1]))
	require.NoError(t, err)
	l := string(lim)
	for _, want := range []string{"Max cpu time", "Max file size", "Max address space", "Max open files", "Max core file size"} {
		assert.Contains(t, l, want)
	}
	assert.Regexp(t, `Max core file size\s+0\s+0`, l)
	assert.Regexp(t, `Max open files\s+1024\s+1024`, l)
	assert.NotRegexp(t, `Max file size\s+unlimited`, l)
	assert.NotRegexp(t, `Max address space\s+unlimited`, l)
	assert.NotRegexp(t, `Max cpu time\s+unlimited`, l)
	cmdline, _ := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pids[len(pids)-1]))
	assert.Contains(t, string(cmdline), "-nostdin")

	select {
	case err := <-done:
		require.Error(t, err, "the job must be killed at its deadline")
	case <-time.After(20 * time.Second):
		t.Fatal("the tool outlived its deadline")
	}
	assert.Less(t, time.Since(start), 15*time.Second)
	time.Sleep(300 * time.Millisecond)
	assert.Empty(t, findProcByArg(token), "the process group was not killed")
}

func TestHardenLive_HardTimeoutWithoutCallerDeadline(t *testing.T) {
	needTools(t, "ffmpeg", "prlimit")
	t.Setenv("CATALOGIZER_CONVERSION_TIMEOUT", "2s")
	root, _, _ := rv6ConvFixture(t)
	src, tgt := slowFFmpegJob(t, root)
	start := time.Now()
	err := runConverterTool("ffmpeg", []string{"-i", src, "-y", "-c:v", "libx264", "-crf", "0", "-preset", "veryslow", tgt})
	require.Error(t, err)
	assert.Less(t, time.Since(start), 15*time.Second, "a call with no deadline must still be bounded")
	time.Sleep(300 * time.Millisecond)
	assert.Empty(t, findProcByArg(filepath.Base(tgt)))
}

func TestHardenLive_CPULimitScalesWithDeadline(t *testing.T) {
	ctx, cancel := ctxWithTimeout(10 * time.Minute)
	defer cancel()
	got := converterCPUSeconds(ctx)
	assert.GreaterOrEqual(t, got, 600)
	assert.LessOrEqual(t, got, 7*24*3600)
}
