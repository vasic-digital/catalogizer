package services

import (
	"bytes"
	"compress/flate"
	"compress/zlib"
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"catalogizer/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// simplePDF builds a valid one-page PDF with correct xref offsets. widths adds a /Widths array
// (500 units per glyph, as pdfTeX / Word embed for simple fonts); flateStream compresses the
// content stream.
func simplePDF(content []byte, flateStream, widths bool) []byte {
	var b bytes.Buffer
	var offs []int
	obj := func(s string) { offs = append(offs, b.Len()); b.WriteString(s) }
	b.WriteString("%PDF-1.4\n")
	obj("1 0 obj<</Type/Catalog/Pages 2 0 R>>endobj\n")
	obj("2 0 obj<</Type/Pages/Kids[4 0 R]/Count 1>>endobj\n")
	font := "3 0 obj<</Type/Font/Subtype/Type1/BaseFont/Helvetica"
	if widths {
		font += "/FirstChar 32/LastChar 126/Widths[" + strings.Repeat("500 ", 95) + "]"
	}
	obj(font + ">>endobj\n")
	obj("4 0 obj<</Type/Page/Parent 2 0 R/MediaBox[0 0 600 800]/Resources<</Font<</F1 3 0 R>>>>/Contents 5 0 R>>endobj\n")
	data, filter := content, ""
	if flateStream {
		var z bytes.Buffer
		w := zlib.NewWriter(&z)
		_, _ = w.Write(content)
		_ = w.Close()
		data, filter = z.Bytes(), "/Filter/FlateDecode"
	}
	offs = append(offs, b.Len())
	fmt.Fprintf(&b, "5 0 obj<</Length %d%s>>stream\n", len(data), filter)
	b.Write(data)
	b.WriteString("\nendstream endobj\n")
	x := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(offs)+1)
	for _, o := range offs {
		fmt.Fprintf(&b, "%010d 00000 n \n", o)
	}
	fmt.Fprintf(&b, "trailer<</Size %d/Root 1 0 R>>\nstartxref\n%d\n%%%%EOF\n", len(offs)+1, x)
	return b.Bytes()
}

var _ = flate.BestSpeed

// bombPDF: a page whose content stream is one Tj of n glyphs; n bytes of input compress to ~n/1000.
func bombPDF(n int) []byte {
	c := []byte("BT /F1 12 Tf 10 700 Td (" + strings.Repeat("a", n) + ") Tj ET")
	return simplePDF(c, true, true)
}

func writeTmp(t *testing.T, data []byte) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "x.pdf")
	require.NoError(t, os.WriteFile(p, data, 0o644))
	return p
}

// F1: a small file must not make the extractor allocate memory proportional to the decompressed
// content (measured on the unbounded implementation: 16 KB file, 16M glyphs, 4.3 GB RSS).
func TestCAT002_FlateBombBoundedMemory(t *testing.T) {
	p := writeTmp(t, bombPDF(2<<20)) // 2M glyphs ~ 540 MB if parsed
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	_, err := ledongthucExtractor{}.ExtractPages(context.Background(), p, 1, 1)
	runtime.ReadMemStats(&after)
	require.Error(t, err, "oversize page content must be refused by the primary extractor")
	assert.Contains(t, err.Error(), "content")
	assert.Less(t, after.TotalAlloc-before.TotalAlloc, uint64(64<<20), "allocation must stay bounded")
}

// F5: when the primary refuses, the pdftotext fallback supplies the text.
func TestCAT002_FallbackTakesOverOnOversizePage(t *testing.T) {
	pe, ok := newPopplerExtractor()
	if !ok {
		t.Skip("pdftotext not installed")
	}
	p := writeTmp(t, bombPDF(2<<20))
	fb := fallbackPDFExtractor{primary: ledongthucExtractor{}, secondary: pe}
	pages, err := fb.ExtractPages(context.Background(), p, 1, 1)
	require.NoError(t, err)
	require.Len(t, pages, 1)
	assert.Contains(t, pages[0], "aaaaaaaa")
	// and the primary alone has no text for it
	_, perr := ledongthucExtractor{}.ExtractPages(context.Background(), p, 1, 1)
	require.Error(t, perr)
	// NumPages fallback path: primary rejects a too-deep tree, pdfinfo still answers
	deep := writeTmp(t, deepTreePDF(80, false))
	_, perr = ledongthucExtractor{}.NumPages(context.Background(), deep)
	require.Error(t, perr)
	n, err := fb.NumPages(context.Background(), deep)
	if err == nil {
		assert.Equal(t, 1, n)
	} else {
		t.Logf("pdfinfo also rejects the deep tree: %v", err)
	}
}

// F2/F3: TJ kerning offsets are word breaks; the oracle is pdftotext.
func TestCAT002_TJKerningWordBreaks(t *testing.T) {
	content := []byte("BT /F1 12 Tf 10 700 Td [(Hello) -333 (world) 20 (s) -400 (again)] TJ 0 -20 Td [(Second) -300 (line)] TJ ET")
	p := writeTmp(t, simplePDF(content, false, true))
	pages, err := ledongthucExtractor{}.ExtractPages(context.Background(), p, 1, 1)
	require.NoError(t, err)
	got := pages[0]
	assert.Equal(t, "Hello worlds again\nSecond line\n", got, "no glued words, no spurious blank line")
	if _, lerr := newPopplerExtractor(); lerr {
		oracle := pdftotextOracle(t, p, 0, 0)
		assert.Equal(t, normWords(oracle), normWords(got))
	}
}

func TestCAT002_TJKerningThroughService(t *testing.T) {
	root := t.TempDir()
	t.Setenv(pathGuardEnvConv, root)
	t.Setenv(pathGuardEnvRead, root)
	content := []byte("BT /F1 12 Tf 10 700 Td [(Catalog) -333 (izer) -333 (kerning) 40 (test)] TJ ET")
	src := filepath.Join(root, "k.pdf")
	require.NoError(t, os.WriteFile(src, simplePDF(content, true, true), 0o644))
	tgt := filepath.Join(root, "k.txt")
	require.NoError(t, (&ConversionService{}).convertPDFToText(&models.ConversionJob{SourcePath: src, TargetPath: tgt}))
	b, _ := os.ReadFile(tgt)
	assert.Equal(t, "Catalog izer kerningtest\n", string(b))
}

// F4: huge /Count with a correct xref (the old fixture had broken offsets and tested nothing).
func TestCAT002_HugeCountFixtureParses(t *testing.T) {
	s := strings.Replace(string(rv6PDF(1)), "/Count 1", "/Count 999999999", 1)
	data := fixPDFOffsets(s)
	p := writeTmp(t, data)
	// the xref must be valid: pdfinfo (independent) reads the page tree
	_, err := ledongthucExtractor{}.NumPages(context.Background(), p)
	t.Logf("NumPages err=%v", err)
	root := t.TempDir()
	t.Setenv(pathGuardEnvConv, root)
	t.Setenv(pathGuardEnvRead, root)
	src := filepath.Join(root, "c.pdf")
	require.NoError(t, os.WriteFile(src, data, 0o644))
	done := make(chan error, 1)
	go func() {
		done <- (&ConversionService{}).convertPDFToText(&models.ConversionJob{SourcePath: src, TargetPath: filepath.Join(root, "c.txt")})
	}()
	select {
	case <-done:
	case <-timeAfter(20):
		t.Fatal("huge /Count hung the extractor")
	}
}

// deepTreePDF nests depth Pages nodes before the single leaf page.
func deepTreePDF(depth int, _ bool) []byte {
	var b bytes.Buffer
	var offs []int
	obj := func(s string) { offs = append(offs, b.Len()); b.WriteString(s) }
	b.WriteString("%PDF-1.4\n")
	obj("1 0 obj<</Type/Catalog/Pages 2 0 R>>endobj\n")
	for i := 0; i < depth; i++ {
		obj(fmt.Sprintf("%d 0 obj<</Type/Pages/Kids[%d 0 R]/Count 1/Parent %d 0 R>>endobj\n", 2+i, 3+i, 1+i))
	}
	leaf := 2 + depth
	obj(fmt.Sprintf("%d 0 obj<</Type/Page/Parent %d 0 R/MediaBox[0 0 100 100]>>endobj\n", leaf, leaf-1))
	x := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(offs)+1)
	for _, o := range offs {
		fmt.Fprintf(&b, "%010d 00000 n \n", o)
	}
	fmt.Fprintf(&b, "trailer<</Size %d/Root 1 0 R>>\nstartxref\n%d\n%%%%EOF\n", len(offs)+1, x)
	return b.Bytes()
}

// F5 (depth guard): 80 nested Pages nodes are refused, 40 are accepted.
func TestCAT002_PageTreeDepthGuard(t *testing.T) {
	_, err := ledongthucExtractor{}.NumPages(context.Background(), writeTmp(t, deepTreePDF(80, false)))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rejected PDF")
	n, err := ledongthucExtractor{}.NumPages(context.Background(), writeTmp(t, deepTreePDF(40, false)))
	require.NoError(t, err)
	assert.Equal(t, 1, n)
}

// F7: float settings far outside int range must not wrap.
func TestCAT002_SettingIntBounds(t *testing.T) {
	for _, v := range []float64{1e300, -1e300, math.NaN(), math.Inf(1), 1 << 40} {
		_, ok := settingInt(map[string]interface{}{"page": v}, "page")
		assert.False(t, ok, "%v", v)
	}
	n, ok := settingInt(map[string]interface{}{"page": float64(7)}, "page")
	assert.True(t, ok)
	assert.Equal(t, 7, n)
	_ = strconv.Itoa
}

func timeAfter(sec int) <-chan time.Time { return time.After(time.Duration(sec) * time.Second) }
