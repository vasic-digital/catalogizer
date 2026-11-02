package services

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"catalogizer/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pdftotextOracle runs the independent poppler extractor (never used by the code under test as
// the primary path) and returns its output with form feeds removed. Skips when absent (§11.4.3).
func pdftotextOracle(t *testing.T, pdf string, first, last int) string {
	t.Helper()
	bin, err := exec.LookPath("pdftotext")
	if err != nil {
		t.Skip("pdftotext not installed: independent oracle unavailable (hardware_not_present-class SKIP)")
	}
	args := []string{}
	if first > 0 {
		args = append(args, "-f", itoa(first), "-l", itoa(last))
	}
	args = append(args, pdf, "-")
	out, err := exec.Command(bin, args...).Output()
	require.NoError(t, err)
	return strings.ReplaceAll(string(out), "\f", "")
}

func itoa(i int) string { return strconv.Itoa(i) }

// TestCAT002_ConvertPDFToText_RealPDF reproduces CAT-002: a valid PDF must yield its text. With
// unipdf the call failed with "unipdf license code required" (no text was ever produced).
func TestCAT002_ConvertPDFToText_RealPDF(t *testing.T) {
	root, _, src := rv6ConvFixture(t)
	tgt := filepath.Join(root, "out.txt")
	require.NoError(t, confineConversionPaths(src, tgt))
	err := (&ConversionService{}).convertPDFToText(&models.ConversionJob{SourcePath: src, TargetPath: tgt})
	require.NoError(t, err, "valid PDF must convert (CAT-002: was 'unipdf license code required')")
	got, err := os.ReadFile(tgt)
	require.NoError(t, err)
	assert.Contains(t, string(got), "p0")
	assert.Contains(t, string(got), "p1")
	assert.Contains(t, string(got), "--- Page 2 ---")
	// independent oracle agrees on the words
	oracle := pdftotextOracle(t, src, 0, 0)
	assert.Contains(t, oracle, "p0")
	assert.Contains(t, oracle, "p1")
}

var wordSplit = strings.Fields

func normWords(s string) string { return strings.Join(wordSplit(s), " ") }

func copyFixture(t *testing.T, name string) (root, src string) {
	t.Helper()
	root = t.TempDir()
	data, err := os.ReadFile(filepath.Join("testdata", "pdf", name))
	require.NoError(t, err)
	src = filepath.Join(root, name)
	require.NoError(t, os.WriteFile(src, data, 0o644))
	t.Setenv(pathGuardEnvConv, root)
	t.Setenv(pathGuardEnvRead, root)
	return
}

func convertFixture(t *testing.T, name string, settings *string) string {
	t.Helper()
	root, src := copyFixture(t, name)
	tgt := filepath.Join(root, "out.txt")
	require.NoError(t, confineConversionPaths(src, tgt))
	require.NoError(t, (&ConversionService{}).convertPDFToText(&models.ConversionJob{SourcePath: src, TargetPath: tgt, Settings: settings}))
	b, err := os.ReadFile(tgt)
	require.NoError(t, err)
	return string(b)
}

// Golden fixtures: the committed *.golden.txt files are pdftotext (poppler) output, an extractor
// independent of the one under test; page separators are stripped before comparing word streams.
func TestCAT002_GoldenFixtures(t *testing.T) {
	for _, name := range []string{"uni", "multi", "large"} {
		t.Run(name, func(t *testing.T) {
			got := convertFixture(t, name+".pdf", nil)
			got = stripPageSeparators(got)
			want, err := os.ReadFile(filepath.Join("testdata", "pdf", name+".golden.txt"))
			require.NoError(t, err)
			require.NotEmpty(t, strings.TrimSpace(string(want)), "golden must not be empty")
			assert.Equal(t, normWords(string(want)), normWords(got))
		})
	}
	t.Run("multi-column keeps every word of both columns", func(t *testing.T) {
		got := convertFixture(t, "cols.pdf", nil)
		for _, w := range []string{"LEFTCOL", "alpha", "omega", "RIGHTCOL", "one", "eighteen"} {
			assert.Contains(t, got, w)
		}
		want, _ := os.ReadFile(filepath.Join("testdata", "pdf", "cols.golden.txt"))
		assert.ElementsMatch(t, wordSplit(string(want)), wordSplit(got), "same word multiset as poppler (order may differ across columns)")
	})
	t.Run("unicode CID fonts", func(t *testing.T) {
		got := convertFixture(t, "uni.pdf", nil)
		for _, w := range []string{"Привет,", "Каталогизатор", "Žluťoučký", "straße.", "日本語のテキスト"} {
			assert.Contains(t, got, w)
		}
	})
	t.Run("scanned image-only yields empty text without error", func(t *testing.T) {
		got := convertFixture(t, "scanned.pdf", nil)
		assert.Empty(t, strings.TrimSpace(got))
	})
}

func stripPageSeparators(s string) string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if strings.HasPrefix(l, "--- Page ") && strings.HasSuffix(l, " ---") {
			continue
		}
		out = append(out, l)
	}
	return strings.Join(out, "\n")
}

// The settings are JSON-decoded, so numbers arrive as float64; the old settings["page"].(int)
// assertion never matched and every page selection was silently ignored.
func TestCAT002_PageSettingsJSONNumbers(t *testing.T) {
	p := `{"page": 2}`
	got := convertFixture(t, "large.pdf", &p)
	assert.NotContains(t, got, "--- Page")
	root, src := copyFixture(t, "large.pdf")
	_ = root
	want := pdftotextOracle(t, src, 2, 2)
	assert.Equal(t, normWords(want), normWords(got), "single page must equal poppler's page 2")

	r := `{"start_page": 3, "end_page": 4}`
	got = convertFixture(t, "large.pdf", &r)
	assert.Contains(t, got, "--- Page 4 ---")
	assert.NotContains(t, got, "--- Page 5 ---")
	want = pdftotextOracle(t, src, 3, 4)
	assert.Equal(t, normWords(want), normWords(stripPageSeparators(got)))

	bad := `{"page": 99999}` // out of range -> full document, unchanged semantics
	got = convertFixture(t, "multi.pdf", &bad)
	assert.Contains(t, got, "Line 60")
}

func TestSelectPDFPages(t *testing.T) {
	cases := []struct {
		in         map[string]interface{}
		total      int
		start, end int
	}{
		{map[string]interface{}{}, 5, 1, 5},
		{map[string]interface{}{"page": float64(3)}, 5, 3, 3},
		{map[string]interface{}{"page": 3}, 5, 3, 3},
		{map[string]interface{}{"page": float64(9)}, 5, 1, 5},
		{map[string]interface{}{"page": 2.5}, 5, 1, 5},
		{map[string]interface{}{"start_page": float64(2)}, 5, 2, 5},
		{map[string]interface{}{"start_page": float64(2), "end_page": float64(4)}, 5, 2, 4},
		{map[string]interface{}{"start_page": float64(2), "end_page": float64(1)}, 5, 2, 5},
	}
	for i, c := range cases {
		s, e := selectPDFPages(c.in, c.total)
		assert.Equal(t, [2]int{c.start, c.end}, [2]int{s, e}, "case %d", i)
	}
}

// Integration: the real ConversionService.convertPDF route (document/pdf -> txt and -> html)
// on a real file with the real extractors; no test doubles.
func TestCAT002_Integration_ConvertPDFRoutes(t *testing.T) {
	root, src := copyFixture(t, "uni.pdf")
	s := &ConversionService{}

	txt := filepath.Join(root, "uni.txt")
	require.NoError(t, confineConversionPaths(src, txt))
	require.NoError(t, s.convertPDF(&models.ConversionJob{SourcePath: src, TargetPath: txt, SourceFormat: "pdf", TargetFormat: "txt", ConversionType: "document"}))
	b, err := os.ReadFile(txt)
	require.NoError(t, err)
	assert.Contains(t, string(b), "Каталогизатор")

	htmlOut := filepath.Join(root, "uni.html")
	require.NoError(t, s.convertPDF(&models.ConversionJob{SourcePath: src, TargetPath: htmlOut, SourceFormat: "pdf", TargetFormat: "html", ConversionType: "document"}))
	hb, err := os.ReadFile(htmlOut)
	require.NoError(t, err)
	// NOT EXERCISED ON THIS HOST: the pandoc and LibreOffice legs (pandoc cannot read PDF, LibreOffice
	// fails to start: libreglo.so missing), so this run passes through the text fallback only. The expectation:
	// the document text must be in the HTML: a conversion that "succeeds" with an empty page is the bluff.
	assert.Contains(t, string(hb), "Catalogizer")
	_, serr := os.Lstat(htmlOut + ".tmp.txt")
	assert.True(t, os.IsNotExist(serr), "temp sibling must be removed")
}

// The HTML text fallback escaped nothing before CAT-002: PDF text such as "<script>" was written
// into the page verbatim.
func TestCAT002_HTMLFallbackEscapesPDFText(t *testing.T) {
	root := t.TempDir()
	t.Setenv(pathGuardEnvConv, root)
	t.Setenv(pathGuardEnvRead, root)
	// A PDF whose text is "<b>x</b>&": build with the rv6 generator's format.
	src := filepath.Join(root, "evil.pdf")
	require.NoError(t, os.WriteFile(src, textPDF(`(<b>x</b> & <script>) Tj`), 0o644))
	// make the external converters unavailable so the text fallback is the path under test
	bin := t.TempDir()
	pl, perr := exec.LookPath("prlimit")
	require.NoError(t, perr)
	require.NoError(t, os.Symlink(pl, filepath.Join(bin, "prlimit")))
	t.Setenv("PATH", bin) // only prlimit is left: poppler and the converters are unavailable
	tgt := filepath.Join(root, "evil.html")
	require.NoError(t, (&ConversionService{}).convertPDFToHTML(&models.ConversionJob{SourcePath: src, TargetPath: tgt}))
	b, err := os.ReadFile(tgt)
	require.NoError(t, err)
	assert.NotContains(t, string(b), "<script>")
	assert.Contains(t, string(b), "&lt;script&gt;")
}

// textPDF builds a one-page valid PDF whose content stream is BT /F1 12 Tf 10 50 Td <ops> ET.
func textPDF(ops string) []byte {
	pdf := rv6PDF(1)
	st := "BT /F1 12 Tf 10 50 Td " + ops + " ET"
	old := "BT /F1 12 Tf 10 10 Td (p0) Tj ET"
	s := strings.Replace(string(pdf), old, st+strings.Repeat(" ", 0), 1)
	// lengths and xref offsets are recomputed by regenerating around the replaced stream
	return fixPDFOffsets(s)
}

// fixPDFOffsets rewrites /Length and the xref table of a simple generated PDF.
func fixPDFOffsets(s string) []byte {
	var out strings.Builder
	// split before xref, regenerate it
	body := s[:strings.Index(s, "xref\n")]
	// fix each "<</Length N>>stream\nDATA\nendstream"
	var fixed strings.Builder
	for len(body) > 0 {
		i := strings.Index(body, "<</Length ")
		if i < 0 {
			fixed.WriteString(body)
			break
		}
		fixed.WriteString(body[:i])
		j := strings.Index(body[i:], ">>stream\n")
		k := strings.Index(body[i:], "\nendstream")
		data := body[i+j+len(">>stream\n") : i+k]
		fixed.WriteString("<</Length " + strconv.Itoa(len(data)) + ">>stream\n" + data)
		body = body[i+k:]
	}
	b := fixed.String()
	var offs []int
	for n := 1; ; n++ {
		idx := strings.Index(b, "\n"+strconv.Itoa(n)+" 0 obj")
		if idx < 0 {
			break
		}
		offs = append(offs, idx+1)
	}
	out.WriteString(b)
	xo := len(b)
	out.WriteString("xref\n0 " + strconv.Itoa(len(offs)+1) + "\n0000000000 65535 f \n")
	for _, o := range offs {
		out.WriteString(strings.Repeat("0", 10-len(strconv.Itoa(o))) + strconv.Itoa(o) + " 00000 n \n")
	}
	out.WriteString("trailer<</Size " + strconv.Itoa(len(offs)+1) + "/Root 1 0 R>>\nstartxref\n" + strconv.Itoa(xo) + "\n%%EOF\n")
	return []byte(out.String())
}
