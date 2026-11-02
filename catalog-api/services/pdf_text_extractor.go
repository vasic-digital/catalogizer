package services

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ledongthuc/pdf"
)

// CAT-002: PDF text extraction. The previous engine (unipdf) needs a commercial licence key and
// failed at run time with "unipdf license code required". Text extraction now goes through the
// pdfTextExtractor interface: the primary implementation is github.com/ledongthuc/pdf
// (BSD-3-Clause, pure Go); poppler's pdftotext is an optional exec fallback.

const (
	// pdfTextTimeout bounds one whole extraction (primary and fallback together).
	pdfTextTimeout = 2 * time.Minute
	// pdfTextMaxBytes bounds the extracted text so a hostile PDF cannot exhaust memory.
	pdfTextMaxBytes = 64 << 20
)

// pdfTextExtractor extracts the text of a PDF file. Pages are 1-based and inclusive.
type pdfTextExtractor interface {
	// NumPages returns the number of pages of the PDF at path.
	NumPages(ctx context.Context, path string) (int, error)
	// ExtractPages returns the text of pages start..end, one entry per page.
	ExtractPages(ctx context.Context, path string, start, end int) ([]string, error)
}

// ledongthucExtractor is the pure-Go primary extractor.
type ledongthucExtractor struct{}

func (ledongthucExtractor) open(path string) (*os.File, *pdf.Reader, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open PDF: %w", err)
	}
	st, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, nil, fmt.Errorf("failed to open PDF: %w", err)
	}
	r, err := safePDFReader(f, st.Size())
	if err != nil {
		f.Close()
		return nil, nil, fmt.Errorf("failed to create PDF reader: %w", err)
	}
	if err := checkPageTree(r); err != nil {
		f.Close()
		return nil, nil, fmt.Errorf("rejected PDF: %w", err)
	}
	return f, r, nil
}

const (
	pdfMaxTreeDepth = 64
	pdfMaxTreeNodes = 1 << 20
)

// checkPageTree walks the page tree before any page is requested. The library's Reader.Page
// descends the tree in an unbounded loop, so a /Kids entry that points back at an ancestor makes
// it spin forever (measured: a one-byte edit of a valid PDF hung the extractor with a CPU core
// pinned and no way to cancel it). A tree deeper than pdfMaxTreeDepth, or larger than
// pdfMaxTreeNodes, is refused.
func checkPageTree(r *pdf.Reader) (err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("PDF parser panic: %v", p)
		}
	}()
	root := r.Trailer().Key("Root").Key("Pages")
	if root.Kind() == pdf.Null {
		return nil
	}
	nodes := 0
	var walk func(v pdf.Value, depth int) error
	walk = func(v pdf.Value, depth int) error {
		if depth > pdfMaxTreeDepth {
			return errors.New("page tree too deep or cyclic")
		}
		if nodes++; nodes > pdfMaxTreeNodes {
			return errors.New("page tree too large")
		}
		if v.Key("Type").Name() != "Pages" {
			return nil
		}
		kids := v.Key("Kids")
		for i := 0; i < kids.Len(); i++ {
			if err := walk(kids.Index(i), depth+1); err != nil {
				return err
			}
		}
		return nil
	}
	return walk(root, 0)
}

// safePDFReader converts a parser panic on a hostile file into an error.
func safePDFReader(f *os.File, size int64) (r *pdf.Reader, err error) {
	defer func() {
		if p := recover(); p != nil {
			r, err = nil, fmt.Errorf("PDF parser panic: %v", p)
		}
	}()
	return pdf.NewReader(f, size)
}

func (e ledongthucExtractor) NumPages(ctx context.Context, path string) (n int, err error) {
	f, r, err := e.open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	defer func() {
		if p := recover(); p != nil {
			n, err = 0, fmt.Errorf("PDF parser panic: %v", p)
		}
	}()
	return r.NumPage(), nil
}

func (e ledongthucExtractor) ExtractPages(ctx context.Context, path string, start, end int) (out []string, err error) {
	f, r, err := e.open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	total := 0
	for i := start; i <= end; i++ {
		if cerr := ctx.Err(); cerr != nil {
			return nil, cerr
		}
		txt, perr := pageText(r, i)
		if perr != nil {
			return nil, fmt.Errorf("failed to extract text from page %d: %w", i, perr)
		}
		total += len(txt)
		if total > pdfTextMaxBytes {
			return nil, fmt.Errorf("extracted text exceeds %d bytes", pdfTextMaxBytes)
		}
		out = append(out, txt)
	}
	return out, nil
}

func pageText(r *pdf.Reader, n int) (txt string, err error) {
	defer func() {
		if p := recover(); p != nil {
			txt, err = "", fmt.Errorf("PDF parser panic: %v", p)
		}
	}()
	p := r.Page(n)
	if p.V.IsNull() {
		return "", fmt.Errorf("page %d not found", n)
	}
	return layoutPageText(p.Content().Text), nil
}

// layoutPageText rebuilds text lines from the positioned glyph runs of a page, in content-stream
// order (which for multi-column layouts follows the producer's column/cell order). The library's
// GetPlainText concatenates every text-show string with no separators (no line breaks, words glued
// at wraps) and its per-run X/W values carry no per-glyph advance (W is 0), so no geometric
// reflow is attempted: a new line starts when the baseline moves by more than half the font
// size; spaces are those present in the stream. A trailing U+FFFD at a line end (the line-end
// marker some producers emit) is dropped.
func layoutPageText(runs []pdf.Text) string {
	var sb strings.Builder
	var line strings.Builder
	flush := func() {
		sb.WriteString(strings.TrimRight(line.String(), "\uFFFD"))
		sb.WriteByte('\n')
		line.Reset()
	}
	have := false
	var lastY float64
	for _, r := range runs {
		tol := math.Max(r.FontSize, 1) * 0.5
		if have && math.Abs(r.Y-lastY) > tol {
			flush()
		}
		line.WriteString(r.S)
		lastY, have = r.Y, true
	}
	if have {
		flush()
	}
	return sb.String()
}

// popplerExtractor runs pdftotext without a shell. The source path is made absolute (so it can
// never be read as an option), the tool gets a timeout via ctx, and its output is size-bounded.
type popplerExtractor struct{ bin string }

func newPopplerExtractor() (popplerExtractor, bool) {
	bin, err := exec.LookPath("pdftotext")
	if err != nil {
		return popplerExtractor{}, false
	}
	return popplerExtractor{bin: bin}, true
}

func (e popplerExtractor) run(ctx context.Context, path string, args ...string) ([]byte, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	full := append(append([]string{}, args...), abs, "-")
	cmd := exec.CommandContext(ctx, e.bin, full...)
	var stdout, stderr limitedBuffer
	stdout.max, stderr.max = pdfTextMaxBytes, 4096
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		if cerr := ctx.Err(); cerr != nil {
			return nil, cerr
		}
		return nil, fmt.Errorf("pdftotext: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	if stdout.over {
		return nil, fmt.Errorf("pdftotext output exceeds %d bytes", pdfTextMaxBytes)
	}
	return stdout.Bytes(), nil
}

func (e popplerExtractor) NumPages(ctx context.Context, path string) (int, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return 0, err
	}
	bin, err := exec.LookPath("pdfinfo")
	if err != nil {
		return 0, errors.New("pdfinfo not installed")
	}
	cmd := exec.CommandContext(ctx, bin, abs)
	var out limitedBuffer
	out.max = 1 << 16
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return 0, fmt.Errorf("pdfinfo: %w", err)
	}
	for _, l := range strings.Split(out.String(), "\n") {
		if strings.HasPrefix(l, "Pages:") {
			return strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(l, "Pages:")))
		}
	}
	return 0, errors.New("pdfinfo: no Pages line")
}

func (e popplerExtractor) ExtractPages(ctx context.Context, path string, start, end int) ([]string, error) {
	out, err := e.run(ctx, path, "-f", strconv.Itoa(start), "-l", strconv.Itoa(end), "-enc", "UTF-8")
	if err != nil {
		return nil, err
	}
	pages := strings.Split(string(out), "\f")
	want := end - start + 1
	if len(pages) > want { // trailing empty element after the last form feed
		pages = pages[:want]
	}
	for len(pages) < want {
		pages = append(pages, "")
	}
	return pages, nil
}

// fallbackPDFExtractor tries primary, then (only if it fails) secondary.
type fallbackPDFExtractor struct{ primary, secondary pdfTextExtractor }

func (f fallbackPDFExtractor) NumPages(ctx context.Context, path string) (int, error) {
	n, err := f.primary.NumPages(ctx, path)
	if err == nil || f.secondary == nil || ctx.Err() != nil || os.IsNotExist(errors.Unwrap(err)) {
		return n, err
	}
	if n2, err2 := f.secondary.NumPages(ctx, path); err2 == nil {
		return n2, nil
	}
	return n, err
}

func (f fallbackPDFExtractor) ExtractPages(ctx context.Context, path string, start, end int) ([]string, error) {
	p, err := f.primary.ExtractPages(ctx, path, start, end)
	if err == nil || f.secondary == nil || ctx.Err() != nil {
		return p, err
	}
	if p2, err2 := f.secondary.ExtractPages(ctx, path, start, end); err2 == nil {
		return p2, nil
	}
	return nil, err
}

// defaultPDFTextExtractor is the production wiring; tests may replace it.
var defaultPDFTextExtractor = func() pdfTextExtractor {
	fb := fallbackPDFExtractor{primary: ledongthucExtractor{}}
	if p, ok := newPopplerExtractor(); ok {
		fb.secondary = p
	}
	return fb
}

// limitedBuffer is a bytes.Buffer that stops growing at max and records the overflow.
type limitedBuffer struct {
	bytes.Buffer
	max  int
	over bool
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.Len()+len(p) > b.max {
		b.over = true
		return len(p), nil
	}
	return b.Buffer.Write(p)
}

// settingInt reads an integer setting from decoded JSON, which yields float64 (the previous
// settings["page"].(int) assertion never matched a JSON-decoded number).
func settingInt(settings map[string]interface{}, key string) (int, bool) {
	switch v := settings[key].(type) {
	case float64:
		if v == float64(int(v)) {
			return int(v), true
		}
	case int:
		return v, true
	case int64:
		return int(v), true
	}
	return 0, false
}

// selectPDFPages applies the page/start_page/end_page settings (semantics unchanged from the
// unipdf implementation: an invalid selection falls back to the full document).
func selectPDFPages(settings map[string]interface{}, total int) (start, end int) {
	start, end = 1, total
	if page, ok := settingInt(settings, "page"); ok && page > 0 && page <= total {
		return page, page
	}
	if s, ok := settingInt(settings, "start_page"); ok && s > 0 {
		start = s
		if e, ok := settingInt(settings, "end_page"); ok && e >= s && e <= total {
			end = e
		}
	}
	return start, end
}
