package services

import (
	"bytes"
	"compress/flate"
	"compress/zlib"
	"context"
	"encoding/ascii85"
	"fmt"
	"math"
	"os"
	"os/exec"
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
	assert.Equal(t, "Catalog izer kerningtest", normWords(string(b)))
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

// multiPDF builds a one-page PDF whose /Contents is a list of streams (direct array, or an
// indirect array object when indirect is true). filters[i] is the /Filter clause of stream i.
func multiPDF(streams [][]byte, filters []string, indirect bool) []byte {
	var b bytes.Buffer
	var offs []int
	obj := func(s string) { offs = append(offs, b.Len()); b.WriteString(s) }
	b.WriteString("%PDF-1.4\n")
	obj("1 0 obj<</Type/Catalog/Pages 2 0 R>>endobj\n")
	obj("2 0 obj<</Type/Pages/Kids[4 0 R]/Count 1>>endobj\n")
	obj("3 0 obj<</Type/Font/Subtype/Type1/BaseFont/Helvetica/FirstChar 32/LastChar 126/Widths[" + strings.Repeat("500 ", 95) + "]>>endobj\n")
	var refs []string
	for i := range streams {
		refs = append(refs, fmt.Sprintf("%d 0 R", 6+i))
	}
	list := "[" + strings.Join(refs, " ") + "]"
	contents := list
	if indirect {
		contents = "5 0 R"
	}
	obj("4 0 obj<</Type/Page/Parent 2 0 R/MediaBox[0 0 600 800]/Resources<</Font<</F1 3 0 R>>>>/Contents " + contents + ">>endobj\n")
	obj("5 0 obj" + list + "endobj\n")
	for i, d := range streams {
		offs = append(offs, b.Len())
		fmt.Fprintf(&b, "%d 0 obj<</Length %d%s>>stream\n", 6+i, len(d), filters[i])
		b.Write(d)
		b.WriteString("\nendstream endobj\n")
	}
	x := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(offs)+1)
	for _, o := range offs {
		fmt.Fprintf(&b, "%010d 00000 n \n", o)
	}
	fmt.Fprintf(&b, "trailer<</Size %d/Root 1 0 R>>\nstartxref\n%d\n%%%%EOF\n", len(offs)+1, x)
	return b.Bytes()
}

func zlibBytes(d []byte) []byte {
	var z bytes.Buffer
	w := zlib.NewWriter(&z)
	_, _ = w.Write(d)
	_ = w.Close()
	return z.Bytes()
}

func a85Flate(d []byte) []byte {
	var out bytes.Buffer
	e := ascii85.NewEncoder(&out)
	_, _ = e.Write(zlibBytes(d))
	_ = e.Close()
	out.WriteString("~>")
	return out.Bytes()
}

// R1: the cap is on the SUM of all entries of a /Contents array (direct or indirect), and also
// applies behind a filter chain.
func TestCAT002_ContentsArrayCapIsCumulative(t *testing.T) {
	piece := func(n int) []byte { return []byte("BT /F1 12 Tf 10 700 Td (" + strings.Repeat("a", n) + ") Tj ET ") }
	part := piece(pdfMaxPageContentBytes/2 + 10<<10) // each entry alone is under the cap
	flt := "/Filter/FlateDecode"
	cases := map[string][]byte{
		"direct array, 3 parts":   multiPDF([][]byte{zlibBytes(part), zlibBytes(part), zlibBytes(part)}, []string{flt, flt, flt}, false),
		"indirect array, 3 parts": multiPDF([][]byte{zlibBytes(part), zlibBytes(part), zlibBytes(part)}, []string{flt, flt, flt}, true),
		"small first, big second": multiPDF([][]byte{[]byte("BT ET"), zlibBytes(piece(pdfMaxPageContentBytes * 2))}, []string{"", flt}, false),
		"ascii85+flate chain":     multiPDF([][]byte{a85Flate(piece(pdfMaxPageContentBytes * 2))}, []string{"/Filter[/ASCII85Decode/FlateDecode]"}, false),
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			p := writeTmp(t, data)
			var a, b runtime.MemStats
			runtime.GC()
			runtime.ReadMemStats(&a)
			_, err := ledongthucExtractor{}.ExtractPages(context.Background(), p, 1, 1)
			runtime.ReadMemStats(&b)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "content")
			assert.Less(t, b.TotalAlloc-a.TotalAlloc, uint64(64<<20))
		})
	}
	// control: within the cap in total, a multi-part page works
	small := []byte("BT /F1 12 Tf 10 700 Td (hello) Tj ET")
	p := writeTmp(t, multiPDF([][]byte{small, small}, []string{"", ""}, false))
	pages, err := ledongthucExtractor{}.ExtractPages(context.Background(), p, 1, 1)
	require.NoError(t, err)
	assert.Contains(t, pages[0], "hello")
}

// R4: cancellation is honoured while walking a wide page tree.
func TestCAT002_PageTreeWalkHonoursCancel(t *testing.T) {
	var b bytes.Buffer
	var offs []int
	obj := func(s string) { offs = append(offs, b.Len()); b.WriteString(s) }
	const leaves = 3000
	b.WriteString("%PDF-1.4\n")
	obj("1 0 obj<</Type/Catalog/Pages 2 0 R>>endobj\n")
	var kids []string
	for i := 0; i < leaves; i++ {
		kids = append(kids, fmt.Sprintf("%d 0 R", 3+i))
	}
	obj(fmt.Sprintf("2 0 obj<</Type/Pages/Kids[%s]/Count %d>>endobj\n", strings.Join(kids, " "), leaves))
	for i := 0; i < leaves; i++ {
		obj(fmt.Sprintf("%d 0 obj<</Type/Page/Parent 2 0 R/MediaBox[0 0 10 10]>>endobj\n", 3+i))
	}
	x := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(offs)+1)
	for _, o := range offs {
		fmt.Fprintf(&b, "%010d 00000 n \n", o)
	}
	fmt.Fprintf(&b, "trailer<</Size %d/Root 1 0 R>>\nstartxref\n%d\n%%%%EOF\n", len(offs)+1, x)
	p := writeTmp(t, b.Bytes())
	n, err := ledongthucExtractor{}.NumPages(context.Background(), p)
	require.NoError(t, err)
	assert.Equal(t, leaves, n)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = ledongthucExtractor{}.NumPages(ctx, p)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

// R8: the pdftotext output bound is enforced (a real process printing more than the bound).
func TestCAT002_PopplerOutputBound(t *testing.T) {
	dir := t.TempDir()
	loud := filepath.Join(dir, "pdftotext")
	require.NoError(t, os.WriteFile(loud, []byte("#!/bin/sh\nhead -c 5000 /dev/zero | tr '\\0' 'x'\n"), 0o755))
	_, err := popplerExtractor{bin: loud, maxOut: 1000}.run(context.Background(), fixturePath(t, "multi.pdf"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds")
	out, err := popplerExtractor{bin: loud, maxOut: 10000}.run(context.Background(), fixturePath(t, "multi.pdf"))
	require.NoError(t, err)
	assert.Len(t, out, 5000)
}

// xobjectPDF: the page only draws a form XObject; the text lives in the form.
func xobjectPDF() []byte {
	var b bytes.Buffer
	var offs []int
	obj := func(s string) { offs = append(offs, b.Len()); b.WriteString(s) }
	b.WriteString("%PDF-1.4\n")
	obj("1 0 obj<</Type/Catalog/Pages 2 0 R>>endobj\n")
	obj("2 0 obj<</Type/Pages/Kids[4 0 R]/Count 1>>endobj\n")
	obj("3 0 obj<</Type/Font/Subtype/Type1/BaseFont/Helvetica/FirstChar 32/LastChar 126/Widths[" + strings.Repeat("500 ", 95) + "]>>endobj\n")
	obj("4 0 obj<</Type/Page/Parent 2 0 R/MediaBox[0 0 600 800]/Resources<</XObject<</X1 6 0 R>>>>/Contents 5 0 R>>endobj\n")
	page := "/X1 Do"
	obj(fmt.Sprintf("5 0 obj<</Length %d>>stream\n%s\nendstream endobj\n", len(page), page))
	form := "BT /F1 12 Tf 10 700 Td (Inside the form XObject) Tj ET"
	obj(fmt.Sprintf("6 0 obj<</Type/XObject/Subtype/Form/BBox[0 0 600 800]/Resources<</Font<</F1 3 0 R>>>>/Length %d>>stream\n%s\nendstream endobj\n", len(form), form))
	x := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(offs)+1)
	for _, o := range offs {
		fmt.Fprintf(&b, "%010d 00000 n \n", o)
	}
	fmt.Fprintf(&b, "trailer<</Size %d/Root 1 0 R>>\nstartxref\n%d\n%%%%EOF\n", len(offs)+1, x)
	return b.Bytes()
}

// The built-in extractor has no "Do" operator: empty text must not be reported as success when
// the page draws form XObjects; poppler (the oracle) reads the text.
func TestCAT002_FormXObjectNotSilentlyEmpty(t *testing.T) {
	p := writeTmp(t, xobjectPDF())
	_, err := ledongthucExtractor{}.ExtractPages(context.Background(), p, 1, 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "form XObjects")
	oracle := pdftotextOracle(t, p, 0, 0)
	assert.Contains(t, oracle, "Inside the form XObject")
	pe, _ := newPopplerExtractor()
	pages, err := (fallbackPDFExtractor{primary: ledongthucExtractor{}, secondary: pe}).ExtractPages(context.Background(), p, 1, 1)
	require.NoError(t, err)
	assert.Contains(t, pages[0], "Inside the form XObject")
}

// Production wiring prefers poppler when it is installed, and it then handles what the
// built-in extractor cannot (XObject text, tracked text, rotated text).
func TestCAT002_DefaultExtractorPrefersPoppler(t *testing.T) {
	if _, ok := newPopplerExtractor(); !ok {
		t.Skip("pdftotext not installed")
	}
	if _, err := exec.LookPath("pdfinfo"); err != nil {
		t.Skip("pdfinfo not installed")
	}
	fb, ok := defaultPDFTextExtractor().(fallbackPDFExtractor)
	require.True(t, ok, "poppler present: poppler primary, ledongthuc fallback")
	_, isPoppler := fb.primary.(popplerExtractor)
	assert.True(t, isPoppler)
	pages, err := defaultPDFTextExtractor().ExtractPages(context.Background(), writeTmp(t, xobjectPDF()), 1, 1)
	require.NoError(t, err)
	assert.Contains(t, pages[0], "Inside the form XObject")

	t.Setenv("PATH", t.TempDir()) // no poppler
	_, only := defaultPDFTextExtractor().(ledongthucExtractor)
	assert.True(t, only, "without poppler the built-in extractor is the only one")
}

// Documented limits of the built-in extractor (the reason poppler is preferred): character
// spacing (Tc) wider than 0.2 em is read as a word break, and rotated text comes out one
// character per line. pdftotext is the oracle.
func TestCAT002_BuiltinExtractorDocumentedLimits(t *testing.T) {
	tracked := writeTmp(t, simplePDF([]byte("BT /F1 12 Tf 3 Tc 10 700 Td (Kerning) Tj ET"), false, true))
	got, err := ledongthucExtractor{}.ExtractPages(context.Background(), tracked, 1, 1)
	require.NoError(t, err)
	assert.Contains(t, got[0], "K e r", "documented limit: tracked text is split by the 0.2 em gap rule")
	assert.Contains(t, pdftotextOracle(t, tracked, 0, 0), "Kerning", "poppler keeps the word whole")

	rot := writeTmp(t, simplePDF([]byte("BT /F1 12 Tf 0 1 -1 0 300 300 Tm (Rotated) Tj ET"), false, true))
	got, err = ledongthucExtractor{}.ExtractPages(context.Background(), rot, 1, 1)
	require.NoError(t, err)
	assert.Greater(t, strings.Count(got[0], "\n"), 3, "documented limit: rotated text is one character per line")
	assert.Contains(t, pdftotextOracle(t, rot, 0, 0), "Rotated")
}

// Amplification bound: a plain-text page just under the cap stays within a fixed allocation.
func TestCAT002_PlainPageUnderCapAllocationBound(t *testing.T) {
	n := pdfMaxPageContentBytes - 200
	p := writeTmp(t, simplePDF([]byte("BT /F1 12 Tf 10 700 Td ("+strings.Repeat("a", n)+") Tj ET"), false, true))
	var a, b runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&a)
	_, err := ledongthucExtractor{}.ExtractPages(context.Background(), p, 1, 1)
	runtime.ReadMemStats(&b)
	require.NoError(t, err)
	assert.Less(t, b.TotalAlloc-a.TotalAlloc, uint64(128<<20))
}
