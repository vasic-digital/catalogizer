package services

import (
	"os"
	"path/filepath"
	"testing"

	"catalogizer/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// LIVE-CLASS for the routing (real convertDocument, real extractor, real PDF fixture, no doubles):
// D1 (docs/qa/live-conv-docs-subs-r1-20261104.md): pdf -> txt was routed to calibre's
// ebook-convert (absent from the image) instead of the CAT-002 isolated extractor.
func TestD1_ConvertDocument_PDFToTextUsesPDFExtractor(t *testing.T) {
	root, src := copyFixture(t, "uni.pdf")
	tgt := filepath.Join(root, "d1.txt")
	require.NoError(t, confineConversionPaths(src, tgt))
	s := &ConversionService{}
	err := s.convertDocument(&models.ConversionJob{SourcePath: src, TargetPath: tgt, SourceFormat: "pdf", TargetFormat: "txt", ConversionType: "document"})
	require.NoError(t, err, "pdf->txt must go to the isolated PDF extractor, not ebook-convert")
	b, err := os.ReadFile(tgt)
	require.NoError(t, err)
	assert.Contains(t, string(b), "Каталогизатор")
}

func TestD7_SanitizeConversionError(t *testing.T) {
	in := `ebook conversion failed: stat /app/data/.catalogizer-conv-123/out.epub: no such file (src '/app/data/a b.pdf')`
	out := sanitizeConversionError(in)
	assert.NotContains(t, out, "/app/data")
	assert.NotContains(t, out, ".catalogizer-conv")
	assert.Contains(t, out, "out.epub")
	assert.Contains(t, out, "no such file")
}

// UNIT-CLASS (§11.4.289(C)): models a host without tools by overriding the lookup; the real
// pair logic is exercised, no conversion is claimed.
func TestD1_DocumentPairsFollowInstalledTools(t *testing.T) {
	orig := converterLookPath
	defer func() { converterLookPath = orig }()
	have := map[string]bool{}
	converterLookPath = func(n string) (string, error) {
		if have[n] {
			return "/x/" + n, nil
		}
		return "", os.ErrNotExist
	}
	assert.True(t, documentPairSupported("pdf", "txt"), "pdf->txt needs no external tool")
	assert.False(t, documentPairSupported("pdf", "html"))
	assert.False(t, documentPairSupported("txt", "epub"))
	assert.False(t, documentPairSupported("pdf", "png"))
	assert.False(t, documentPairSupported("txt", "pdf"))
	f := (&ConversionService{}).GetSupportedFormats()
	assert.Equal(t, []string{"pdf"}, f.Document.Input)
	assert.Equal(t, []string{"txt"}, f.Document.Output)
	assert.Empty(t, f.Video.Input)
	have["pandoc"] = true
	have["ebook-convert"] = true
	have["ffmpeg"] = true
	assert.True(t, documentPairSupported("pdf", "html"))
	assert.True(t, documentPairSupported("txt", "epub"))
	assert.True(t, documentPairSupported("epub", "pdf"))
	assert.NotEmpty(t, (&ConversionService{}).GetSupportedFormats().Video.Input)
}

// UNIT-CLASS: with only calibre present, the input and output lists both contain pdf/html, yet
// pdf -> html (served by the PDF path, needing pandoc/LibreOffice) and txt -> txt are impossible;
// the pair check, not the list check, must refuse them.
func TestD1_PairCheckRefusesImpossibleCombinationOfListedFormats(t *testing.T) {
	orig := converterLookPath
	defer func() { converterLookPath = orig }()
	converterLookPath = func(n string) (string, error) {
		if n == "ebook-convert" {
			return "/x/ebook-convert", nil
		}
		return "", os.ErrNotExist
	}
	s := &ConversionService{}
	assert.False(t, s.isSupportedFormat(models.ConversionTypeDocument, "pdf", "html"))
	assert.False(t, s.isSupportedFormat(models.ConversionTypeDocument, "txt", "txt"))
	assert.True(t, s.isSupportedFormat(models.ConversionTypeDocument, "txt", "epub"))
}
