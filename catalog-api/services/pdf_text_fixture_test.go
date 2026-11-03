package services

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// normalTextPDF generates a deterministic ordinary text PDF: one Helvetica font with widths, one
// content stream per page, `lines` lines of ~60 characters on every page. No hostile structure.
func normalTextPDF(pages, lines int) []byte {
	kids := make([]string, pages)
	for i := range kids {
		kids[i] = fmt.Sprintf("%d 0 R", 4+2*i)
	}
	objs := []string{
		"<</Type/Catalog/Pages 2 0 R>>",
		fmt.Sprintf("<</Type/Pages/Kids[%s]/Count %d>>", strings.Join(kids, " "), pages),
		"<</Type/Font/Subtype/Type1/BaseFont/Helvetica/FirstChar 32/LastChar 126/Widths[" + strings.Repeat("500 ", 95) + "]>>",
	}
	for p := 0; p < pages; p++ {
		var c strings.Builder
		c.WriteString("BT /F1 10 Tf 12 TL 40 760 Td ")
		for l := 0; l < lines; l++ {
			fmt.Fprintf(&c, "(Page %d line %d the quick brown fox jumps over the lazy dog) Tj T* ", p+1, l+1)
		}
		c.WriteString("ET")
		objs = append(objs,
			fmt.Sprintf("<</Type/Page/Parent 2 0 R/MediaBox[0 0 600 800]/Resources<</Font<</F1 3 0 R>>>>/Contents %d 0 R>>", 5+2*p),
			rawStream([]byte(c.String())))
	}
	return buildRawPDF(objs)
}

// TestMain-free helper: when CAT002_DUMP_FIXTURES names a directory, write the two fixtures there
// (used to measure the real production binary from the shell).
func TestCAT002_DumpFixtures(t *testing.T) {
	d := os.Getenv("CAT002_DUMP_FIXTURES")
	if d == "" {
		t.Skip("CAT002_DUMP_FIXTURES not set")
	}
	os.WriteFile(filepath.Join(d, "p146.pdf"), normalTextPDF(146, 45), 0o600)
	os.WriteFile(filepath.Join(d, "p851.pdf"), normalTextPDF(851, 45), 0o600)
}
