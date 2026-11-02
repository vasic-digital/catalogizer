package services

import (
	"context"
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

func fixturePath(t *testing.T, name string) string {
	t.Helper()
	p, err := filepath.Abs(filepath.Join("testdata", "pdf", name))
	require.NoError(t, err)
	return p
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(fixturePath(t, name))
	require.NoError(t, err)
	return b
}

// hostilePDFs are inputs a malicious uploader could supply.
func hostilePDFs(t *testing.T) map[string][]byte {
	good := readFixture(t, "multi.pdf")
	return map[string][]byte{
		"empty file":                 {},
		"garbage":                    []byte(strings.Repeat("\x00\xff not a pdf ", 500)),
		"header only":                []byte("%PDF-1.7\n"),
		"truncated":                  good[:len(good)/2],
		"truncated tail":             good[:len(good)-40],
		"bogus huge count":           fixPDFOffsets(strings.Replace(string(rv6PDF(1)), "/Count 1", "/Count 999999999", 1)),
		"self-referencing page tree": fixPDFOffsets(strings.Replace(string(rv6PDF(1)), "/Kids[4 0 R ]", "/Kids[2 0 R ]", 1)),
		"flipped bytes": func() []byte {
			b := append([]byte(nil), good...)
			for i := 200; i < len(b); i += 97 {
				b[i] ^= 0xff
			}
			return b
		}(),
	}
}

// Security: no hostile input may panic, hang, or write anything; each ends in an error or an
// (empty/partial) result within the deadline.
func TestCAT002_HostilePDFsNeverPanicOrHang(t *testing.T) {
	for name, data := range hostilePDFs(t) {
		t.Run(name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "h.pdf")
			require.NoError(t, os.WriteFile(p, data, 0o644))
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			done := make(chan struct{})
			go func() {
				defer close(done)
				ex := ledongthucExtractor{}
				n, err := ex.NumPages(ctx, p)
				if err == nil && n > 0 && n < 100 {
					_, _ = ex.ExtractPages(ctx, p, 1, n)
				}
			}()
			select {
			case <-done:
			case <-time.After(25 * time.Second):
				t.Fatal("extractor hung on hostile input")
			}
		})
	}
}

// The cyclic page tree is refused with an explicit error rather than reaching the library loop.
func TestCAT002_CyclicPageTreeRejected(t *testing.T) {
	p := filepath.Join(t.TempDir(), "c.pdf")
	require.NoError(t, os.WriteFile(p, hostilePDFs(t)["self-referencing page tree"], 0o644))
	_, err := ledongthucExtractor{}.NumPages(context.Background(), p)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rejected PDF")
}

// Through the service: hostile input yields an error or text, never a panic, and never leaves a
// target outside the roots.
func TestCAT002_HostilePDFsThroughService(t *testing.T) {
	for name, data := range hostilePDFs(t) {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			t.Setenv(pathGuardEnvConv, root)
			t.Setenv(pathGuardEnvRead, root)
			src := filepath.Join(root, "h.pdf")
			require.NoError(t, os.WriteFile(src, data, 0o644))
			tgt := filepath.Join(root, "h.txt")
			assert.NotPanics(t, func() {
				_ = (&ConversionService{}).convertPDFToText(&models.ConversionJob{SourcePath: src, TargetPath: tgt})
			})
		})
	}
}

func TestCAT002_CancelledContextStopsExtraction(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := ledongthucExtractor{}.ExtractPages(ctx, fixturePath(t, "large.pdf"), 1, 146)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestCAT002_OutputSizeBound(t *testing.T) {
	var b limitedBuffer
	b.max = 10
	n, err := b.Write([]byte("0123456"))
	require.NoError(t, err)
	assert.Equal(t, 7, n)
	_, _ = b.Write([]byte("89abc"))
	assert.True(t, b.over)
	assert.Equal(t, "012345689a", b.String(), "an overflowing write keeps only the prefix that fits")
}

func TestCAT002_NonexistentSource(t *testing.T) {
	_, err := ledongthucExtractor{}.NumPages(context.Background(), "/nonexistent/file.pdf")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to open PDF")
}

func TestCAT002_PathEscapeTargetOutsideRoots(t *testing.T) {
	root, outside, src := rv6ConvFixture(t)
	_ = root
	tgt := filepath.Join(outside, "escape.txt")
	err := (&ConversionService{}).convertPDFToText(&models.ConversionJob{SourcePath: src, TargetPath: tgt})
	require.Error(t, err)
	_, serr := os.Lstat(tgt)
	assert.True(t, os.IsNotExist(serr))
}

func TestCAT002_PopplerExtractor(t *testing.T) {
	p, ok := newPopplerExtractor()
	if !ok {
		t.Skip("pdftotext not installed")
	}
	if _, err := exec.LookPath("pdfinfo"); err != nil {
		t.Skip("pdfinfo not installed")
	}
	ctx := context.Background()
	n, err := p.NumPages(ctx, fixturePath(t, "large.pdf"))
	require.NoError(t, err)
	assert.Equal(t, 146, n)
	pages, err := p.ExtractPages(ctx, fixturePath(t, "large.pdf"), 2, 4)
	require.NoError(t, err)
	require.Len(t, pages, 3)
	// agrees with the primary extractor on the same pages
	prim, err := ledongthucExtractor{}.ExtractPages(ctx, fixturePath(t, "large.pdf"), 2, 4)
	require.NoError(t, err)
	for i := range pages {
		assert.Equal(t, normWords(pages[i]), normWords(prim[i]), "page %d", i+2)
	}
}

// A source whose name looks like an option must not be parsed as one.
func TestCAT002_PopplerOptionLikeFilename(t *testing.T) {
	p, ok := newPopplerExtractor()
	if !ok {
		t.Skip("pdftotext not installed")
	}
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "-layout.pdf"), readFixture(t, "multi.pdf"), 0o644))
	wd, _ := os.Getwd()
	require.NoError(t, os.Chdir(dir))
	defer os.Chdir(wd)
	out, err := p.run(context.Background(), "-layout.pdf")
	require.NoError(t, err)
	assert.Contains(t, string(out), "Line 1:")
}

// No shell: shell metacharacters in the path are plain file-name bytes.
func TestCAT002_PopplerNoShellInterpretation(t *testing.T) {
	p, ok := newPopplerExtractor()
	if !ok {
		t.Skip("pdftotext not installed")
	}
	dir := t.TempDir()
	marker := filepath.Join(dir, "pwned")
	name := filepath.Join(dir, "a;touch pwned;$(touch pwned).pdf")
	require.NoError(t, os.WriteFile(name, readFixture(t, "multi.pdf"), 0o644))
	_, err := p.run(context.Background(), name)
	require.NoError(t, err)
	_, serr := os.Lstat(marker)
	assert.True(t, os.IsNotExist(serr), "path was interpreted by a shell")
}

// The tool timeout is enforced (a real process that never exits, not a stub of the PDF logic).
func TestCAT002_PopplerTimeoutKillsTool(t *testing.T) {
	dir := t.TempDir()
	slow := filepath.Join(dir, "pdftotext")
	require.NoError(t, os.WriteFile(slow, []byte("#!/bin/sh\nexec sleep 30\n"), 0o755))
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := popplerExtractor{bin: slow}.run(ctx, fixturePath(t, "multi.pdf"))
	require.Error(t, err)
	assert.Less(t, time.Since(start), 5*time.Second)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

// Fallback: the secondary is consulted only after the primary fails, and a failure of both
// reports the primary's error.
func TestCAT002_FallbackOrdering(t *testing.T) {
	p, ok := newPopplerExtractor()
	if !ok {
		t.Skip("pdftotext not installed")
	}
	fb := fallbackPDFExtractor{primary: ledongthucExtractor{}, secondary: p}
	ctx := context.Background()
	garbage := filepath.Join(t.TempDir(), "g.pdf")
	require.NoError(t, os.WriteFile(garbage, []byte("not a pdf"), 0o644))
	_, err := fb.NumPages(ctx, garbage)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create PDF reader", "primary's error is reported when both fail")
	n, err := fb.NumPages(ctx, fixturePath(t, "multi.pdf"))
	require.NoError(t, err)
	assert.Equal(t, 1, n)
}

func BenchmarkPDFText_Ledongthuc_Large(b *testing.B) {
	ctx := context.Background()
	p, _ := filepath.Abs(filepath.Join("testdata", "pdf", "large.pdf"))
	for i := 0; i < b.N; i++ {
		if _, err := (ledongthucExtractor{}).ExtractPages(ctx, p, 1, 146); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPDFText_Pdftotext_Large(b *testing.B) {
	pe, ok := newPopplerExtractor()
	if !ok {
		b.Skip("pdftotext not installed")
	}
	ctx := context.Background()
	p, _ := filepath.Abs(filepath.Join("testdata", "pdf", "large.pdf"))
	for i := 0; i < b.N; i++ {
		if _, err := pe.ExtractPages(ctx, p, 1, 146); err != nil {
			b.Fatal(err)
		}
	}
}
