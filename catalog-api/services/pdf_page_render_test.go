package services

import (
	"bytes"
	"context"
	"errors"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

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
	assert.ErrorIs(t, err, ErrPDFRenderTimeout)
}
