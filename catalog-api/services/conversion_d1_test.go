package services

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

// F5: directory components containing spaces, Windows and file:// forms must not leak.
func TestRev_F5_SanitizeHandlesSpacesWindowsAndFileURLs(t *testing.T) {
	cases := map[string][]string{
		"convert: improper image header `/srv/media/Secret Project/x.png' @ error":        {"Secret Project", "/srv"},
		`failed:/srv/hidden/dir/x.mp4`:                                                    {"hidden", "/srv", "dir"},
		`failed:/srv/hidden/dir/x.mp4: Invalid data`:                                      {"hidden", "/srv", "dir"},
		`'/srv/My: Movies/x.mp4' broke`:                                                   {"My", "Movies", "/srv"},
		`open "/srv/My: Movies/x.mp4" broke`:                                              {"My", "Movies", "/srv"},
		`src=/srv/hidden/dir/x.mp4 bad`:                                                   {"hidden", "/srv"},
		`(/srv/hidden/dir/x.mp4)`:                                                         {"hidden", "/srv"},
		`[/srv/hidden/dir/x.mp4]`:                                                         {"hidden", "/srv"},
		`a,/srv/hidden/dir/x.mp4`:                                                         {"hidden", "/srv"},
		`unterminated '/srv/secret/dir/x.mp4 no close`:                                    {"secret", "/srv", "dir"},
		`x "/srv/secret dir/x.mp4`:                                                        {"secret", "/srv", "dir"},
		"cut `/srv/secret dir/clients/acme.mp":                                            {"secret", "/srv", "clients"},
		"tool said " + strings.Repeat("e", 2000) + " '/srv/secret/dir/ac":                 {"secret", "/srv"},
		`ffmpeg failed on /srv/media/Secret Project/clients/acme.mp4: Invalid data`:       {"Secret Project", "clients", "/srv"},
		`open "/srv/media/Secret Project/clients/acme.mp4": denied`:                       {"Secret Project", "clients", "/srv"},
		`cannot read C:\Users\Jane Doe\Private Docs\a.pdf: denied`:                        {"Jane Doe", "Private Docs", "Users"},
		`error at file:///srv/media/Secret Project/a.mp4 (code 1)`:                        {"Secret Project", "/srv"},
		`stat /app/data/.catalogizer-conv-1/out.epub: no such file ('/app/data/a b.pdf')`: {"/app/data", "catalogizer-conv"},
	}
	for in, bad := range cases {
		out := sanitizeConversionError(in)
		for _, b := range bad {
			assert.NotContains(t, out, b, "input %q -> %q", in, out)
		}
	}
	assert.Contains(t, sanitizeConversionError(`stat /a/b/out.epub: no such file`), "no such file")
	assert.Contains(t, sanitizeConversionError(`failed:/srv/hidden/dir/x.mp4: Invalid data`), "Invalid data")
	assert.Contains(t, sanitizeConversionError(`'/srv/My: Movies/x.mp4' broke`), "x.mp4")
	assert.Contains(t, sanitizeConversionError(`see http://example.com/docs for help`), "http://example.com/docs", "URLs are not file paths")
}

// UNIT-CLASS (lookup override): every advertised document pair routes to a converter that can
// serve it (F3), and html targets dispatch on the canonical target_format (F4).
func TestRev_F3_AdvertisedPairsRouteToARealConverter(t *testing.T) {
	orig := converterLookPath
	defer func() { converterLookPath = orig }()
	converterLookPath = func(n string) (string, error) { return "/x/" + n, nil }
	s := &ConversionService{}
	f := s.GetSupportedFormats()
	n := 0
	for _, src := range f.Document.Input {
		for _, dst := range f.Document.Output {
			if !documentPairSupported(src, dst) {
				continue
			}
			n++
			route := s.documentRoute(&models.ConversionJob{SourceFormat: src, TargetFormat: dst})
			switch route {
			case "ebook":
			case "pdf":
				assert.Equal(t, "pdf", src, "%s->%s", src, dst)
				assert.Contains(t, []string{"txt", "html"}, dst, "%s->%s", src, dst)
			default:
				t.Errorf("advertised pair %s->%s routes nowhere (%q)", src, dst, route)
			}
		}
	}
	assert.Greater(t, n, 10)
	assert.Equal(t, "ebook", s.documentRoute(&models.ConversionJob{SourceFormat: "docx", TargetFormat: "pdf"}))
	assert.Equal(t, "ebook", s.documentRoute(&models.ConversionJob{SourceFormat: "odt", TargetFormat: "pdf"}))
}

// F4: target_format html with a .htm file name must be served by the PDF html path.
func TestRev_F4_HtmTargetDispatchesOnFormat(t *testing.T) {
	root, src := copyFixture(t, "uni.pdf")
	tgt := filepath.Join(root, "out.htm")
	require.NoError(t, confineConversionPaths(src, tgt))
	err := (&ConversionService{}).convertDocument(&models.ConversionJob{SourcePath: src, TargetPath: tgt, SourceFormat: "pdf", TargetFormat: "html", ConversionType: "document"})
	require.NoError(t, err)
	_, serr := os.Stat(tgt)
	assert.NoError(t, serr)
}

// F7 M5: the failure notification carries the sanitised message. UNIT-CLASS observation hook.
func TestRev_F7_NotificationIsSanitised(t *testing.T) {
	svc, repo, dir := newWorkerSvc(t)
	id := addImageJob(t, repo, dir, "n")
	job, err := repo.GetJob(id)
	require.NoError(t, err)
	job.Status = models.ConversionStatusRunning
	require.NoError(t, repo.UpdateJob(job))
	var got string
	conversionNotifyHook = func(_ *models.ConversionJob, msg string) { got = msg }
	defer func() { conversionNotifyHook = nil }()
	svc.handleConversionError(job, fmt.Errorf("tool failed on /srv/secret dir/clients/acme.mp4"))
	require.NotEmpty(t, got)
	assert.NotContains(t, got, "/srv")
	assert.NotContains(t, got, "secret dir")
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

// F6 (UNIT-CLASS hook models the race): a job that moved pending -> completed between the snapshot
// and the delete must be kept together with its output file (409), not half-deleted.
func TestRev_F6_DeleteRaceKeepsRowAndFile(t *testing.T) {
	svc, repo, dir := newWorkerSvc(t)
	id := addImageJob(t, repo, dir, "race")
	out := filepath.Join(dir, "race.jpg")
	require.NoError(t, os.WriteFile(out, []byte("out"), 0o600))
	conversionPreDeleteHook = func() {
		j, err := repo.GetJob(id)
		require.NoError(t, err)
		j.Status = models.ConversionStatusCompleted
		require.NoError(t, repo.UpdateJob(j))
	}
	defer func() { conversionPreDeleteHook = nil }()
	err := svc.DeleteJob(id, 1)
	require.ErrorIs(t, err, models.ErrConversionJobState)
	_, gerr := repo.GetJob(id)
	assert.NoError(t, gerr, "row kept")
	_, serr := os.Stat(out)
	assert.NoError(t, serr, "output kept")
}
