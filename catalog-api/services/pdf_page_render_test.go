package services

import (
	"bytes"
	"context"
	"errors"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// D3: the isolated poppler renderer, driven with the repository's real PDF fixtures and the real
// pdfinfo / pdftoppm / prlimit tools.

func fixturePDF(t *testing.T, name string) string {
	t.Helper()
	p := filepath.Join("testdata", "pdf", name)
	_, err := os.Stat(p)
	require.NoError(t, err)
	return p
}

func TestD3_PDFPageCountAndRender_RealFixtures(t *testing.T) {
	n, err := PDFPageCount(context.Background(), fixturePDF(t, "large.pdf"))
	require.NoError(t, err)
	assert.Equal(t, 146, n)

	b, err := RenderPDFPagePNG(context.Background(), fixturePDF(t, "large.pdf"), 145, 0)
	require.NoError(t, err)
	img, err := png.Decode(bytes.NewReader(b))
	require.NoError(t, err)
	assert.Positive(t, img.Bounds().Dx())

	_, err = RenderPDFPagePNG(context.Background(), fixturePDF(t, "large.pdf"), 146, 0)
	assert.ErrorIs(t, err, ErrPDFPageOutOfRange)
	_, err = RenderPDFPagePNG(context.Background(), fixturePDF(t, "large.pdf"), -1, 0)
	assert.ErrorIs(t, err, ErrPDFPageOutOfRange)
}

func TestD3_PDFRender_MalformedClassified(t *testing.T) {
	dir := t.TempDir()
	for name, data := range map[string][]byte{"junk.pdf": []byte("not a pdf"), "zero.pdf": {}} {
		p := filepath.Join(dir, name)
		require.NoError(t, os.WriteFile(p, data, 0o600))
		_, err := PDFPageCount(context.Background(), p)
		assert.ErrorIs(t, err, ErrPDFInvalid, name)
		_, err = RenderPDFPagePNG(context.Background(), p, 0, 0)
		assert.ErrorIs(t, err, ErrPDFInvalid, name)
	}
	_, err := PDFPageCount(context.Background(), filepath.Join(dir, "missing.pdf"))
	assert.ErrorIs(t, err, ErrPDFInvalid)
}

// A relative file name that begins with "-" must be read as a file, never as an option. Poppler's
// parser treats an unknown dash word as a file name but takes "-h" / "-v" / "-f" as options
// (a file named "-h" would print the help text instead of being opened), so those are the names
// that prove the absolute-path defence.
func TestD3_PDFRender_OptionLikeRelativeName(t *testing.T) {
	data, err := os.ReadFile(fixturePDF(t, "multi.pdf"))
	require.NoError(t, err)
	old, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(old) })
	for _, name := range []string{"-h", "-v", "-f", "-x.pdf"} {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), data, 0o600))
		require.NoError(t, os.Chdir(dir))

		n, err := PDFPageCount(context.Background(), name)
		require.NoError(t, err, name)
		assert.Equal(t, 1, n, name)
		b, err := RenderPDFPagePNG(context.Background(), name, 0, 0)
		require.NoError(t, err, name)
		_, err = png.Decode(bytes.NewReader(b))
		require.NoError(t, err, name)
	}
}

func TestD3_PDFRender_NoTempLeak(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	_, err := RenderPDFPagePNG(context.Background(), fixturePDF(t, "large.pdf"), 3, 0)
	require.NoError(t, err)
	junk := filepath.Join(t.TempDir(), "j.pdf")
	require.NoError(t, os.WriteFile(junk, []byte("x"), 0o600))
	_, _ = RenderPDFPagePNG(context.Background(), junk, 0, 0)
	entries, err := os.ReadDir(tmp)
	require.NoError(t, err)
	assert.Empty(t, entries, "render directory must be removed")
}

func TestD3_PDFRender_Unavailable_And_Cancelled(t *testing.T) {
	old := lookPath
	t.Cleanup(func() { lookPath = old })

	lookPath = func(string) (string, error) { return "", exec.ErrNotFound }
	_, err := PDFPageCount(context.Background(), fixturePDF(t, "multi.pdf"))
	assert.ErrorIs(t, err, ErrPDFRenderUnavailable)

	lookPath = func(f string) (string, error) {
		if f == "prlimit" {
			return "", exec.ErrNotFound
		}
		return old(f)
	}
	_, err = PDFPageCount(context.Background(), fixturePDF(t, "multi.pdf"))
	assert.ErrorIs(t, err, ErrPDFRenderUnavailable, "must refuse to run a parser without prlimit")
	assert.True(t, errors.Is(err, ErrPDFRenderUnavailable))

	lookPath = old
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = PDFPageCount(ctx, fixturePDF(t, "multi.pdf"))
	assert.ErrorIs(t, err, ErrPDFRenderCanceled, "a client cancel is not a server timeout")
	assert.NotErrorIs(t, err, ErrPDFRenderTimeout)

	dctx, dcancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer dcancel()
	_, err = PDFPageCount(dctx, fixturePDF(t, "multi.pdf"))
	assert.ErrorIs(t, err, ErrPDFRenderTimeout)
	assert.NotErrorIs(t, err, ErrPDFRenderCanceled)
}

// recordingPrlimit installs a prlimit wrapper that records its argv and the LC_ALL/LANG it
// received into a log file, then execs the real prlimit. It returns the log path.
func recordingPrlimit(t *testing.T) string {
	t.Helper()
	real, err := exec.LookPath("prlimit")
	require.NoError(t, err)
	dir := t.TempDir()
	log := filepath.Join(dir, "calls.log")
	script := filepath.Join(dir, "prlimit")
	body := "#!/bin/sh\necho \"ARGV $*\" >> " + log + "\necho \"ENV LC_ALL=$LC_ALL LANG=$LANG\" >> " + log + "\nexec " + real + " \"$@\"\n"
	require.NoError(t, os.WriteFile(script, []byte(body), 0o700))
	old := lookPath
	lookPath = func(f string) (string, error) {
		if f == "prlimit" {
			return script, nil
		}
		return old(f)
	}
	t.Cleanup(func() { lookPath = old })
	return log
}

// The limits the children actually receive (review F5): pdfinfo may write no file; pdftoppm may
// write at most pdfMaxPNGBytes; both get an address-space and a CPU limit, never the data limit;
// poppler gets a fixed C locale whatever the service locale is (F9).
func TestD3_PDFRender_ChildLimitsAndEnv(t *testing.T) {
	t.Setenv("LC_ALL", "de_DE.UTF-8")
	t.Setenv("LANG", "de_DE.UTF-8")
	log := recordingPrlimit(t)
	_, err := RenderPDFPagePNG(context.Background(), fixturePDF(t, "multi.pdf"), 0, 0)
	require.NoError(t, err)
	b, err := os.ReadFile(log)
	require.NoError(t, err)
	var info, ppm []string
	for _, l := range strings.Split(string(b), "\n") {
		switch {
		case strings.HasPrefix(l, "ARGV") && strings.Contains(l, "pdfinfo"):
			info = append(info, l)
		case strings.HasPrefix(l, "ARGV") && strings.Contains(l, "pdftoppm"):
			ppm = append(ppm, l)
		case strings.HasPrefix(l, "ENV"):
			assert.Equal(t, "ENV LC_ALL=C LANG=C", l, "poppler must run in the C locale")
		}
	}
	require.NotEmpty(t, info)
	require.Len(t, ppm, 1)
	for _, l := range append(append([]string{}, info...), ppm...) {
		assert.Contains(t, l, "--as=")
		assert.Contains(t, l, "--cpu=20 ", "CPU limit is 20 s")
		assert.Contains(t, l, "--core=0")
		assert.NotContains(t, l, "--data=")
	}
	for _, l := range info {
		assert.Contains(t, l, "--fsize=0 ")
	}
	assert.Contains(t, ppm[0], "--fsize="+strconv.Itoa(pdfMaxPNGBytes)+" ")
	assert.Contains(t, ppm[0], " -cropbox ")
	assert.Contains(t, ppm[0], " -singlefile ")
}

func TestD3_PDFRender_DPIHonoured(t *testing.T) {
	dim := func(dpi float64) int {
		b, err := RenderPDFPagePNG(context.Background(), fixturePDF(t, "multi.pdf"), 0, dpi)
		require.NoError(t, err)
		img, err := png.Decode(bytes.NewReader(b))
		require.NoError(t, err)
		return img.Bounds().Dx()
	}
	d150, d300 := dim(0), dim(300)
	assert.InDelta(t, 2*d150, d300, 3)
}

// Real kernel-delivered limit signals map to the right sentinel (review F3): the children below
// are killed by RLIMIT_CPU (SIGXCPU) and RLIMIT_FSIZE (SIGXFSZ), not simulated.
func TestD3_PDFRender_LimitSignalsClassified(t *testing.T) {
	ctx := context.Background()
	_, err := runLimited(ctx, "/bin/sh", []string{"-c", "while :; do :; done"}, pdfLimits{cpuSeconds: 1}, 64)
	require.Error(t, err)
	assert.ErrorIs(t, classifyPDFToolError(err), ErrPDFRenderTimeout, err.Error())

	out := filepath.Join(t.TempDir(), "big")
	_, err = runLimited(ctx, "/bin/dd", []string{"if=/dev/zero", "of=" + out, "bs=1000", "count=5000"}, pdfLimits{cpuSeconds: 20, fileSize: 100000}, 64)
	require.Error(t, err)
	assert.ErrorIs(t, classifyPDFToolError(err), ErrPDFPageTooLarge, err.Error())

	_, err = runLimited(ctx, "/bin/sh", []string{"-c", "head -c 5000 /dev/zero"}, pdfLimits{cpuSeconds: 20}, 100)
	require.Error(t, err)
	assert.ErrorIs(t, classifyPDFToolError(err), ErrPDFPageTooLarge, "stdout over its bound")
}

func TestD3_PDFReaderSlots_FailFast(t *testing.T) {
	var rel []func()
	for i := 0; i < PDFReaderSlots; i++ {
		r, ok := AcquirePDFReaderSlot()
		require.True(t, ok)
		rel = append(rel, r)
	}
	_, ok := AcquirePDFReaderSlot()
	assert.False(t, ok, "saturated slots must fail fast")
	rel[0]()
	rel[0]() // idempotent
	r, ok := AcquirePDFReaderSlot()
	assert.True(t, ok)
	r()
	rel[1]()
}
