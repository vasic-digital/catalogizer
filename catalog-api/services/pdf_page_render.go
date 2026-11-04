package services

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// D3: page count and page rendering of an untrusted PDF for the reader endpoints and the PDF to
// image converter. Same isolation as the CAT-002 text extraction (pdf_text_isolation.go): poppler
// tools (pdfinfo, pdftoppm) run as child processes under prlimit (address space, CPU time, file
// size), without a shell, with an allowlisted environment, killed when the context ends, with
// bounded output. No PDF parser runs in the API process. The former renderer (go-fitz / MuPDF,
// cgo, AGPL) was removed.

const (
	// PDFRenderDPI is the rasterisation resolution of a rendered page (the value the former
	// renderer used for the reader).
	PDFRenderDPI = 150.0
	// pdfRenderTimeout bounds one pdfinfo or pdftoppm run.
	pdfRenderTimeout = 60 * time.Second
	// pdfMaxRenderPixels bounds the bitmap of one page (width x height at the requested DPI).
	// An A4 page at 150 dpi is 2.1 MP; an A0 poster is 34 MP. A page above the bound is refused
	// before any rasterisation (the page size comes from pdfinfo).
	pdfMaxRenderPixels = 64 << 20
	// pdfMaxPNGBytes bounds the PNG one page may produce (also the RLIMIT_FSIZE of pdftoppm).
	pdfMaxPNGBytes = 96 << 20
)

var (
	// ErrPDFInvalid: the file is not a PDF poppler can open (malformed, truncated, empty, other format).
	ErrPDFInvalid = errors.New("not a valid PDF document")
	// ErrPDFEncrypted: the PDF needs a password.
	ErrPDFEncrypted = errors.New("PDF document is encrypted")
	// ErrPDFPageTooLarge: the page would need a bitmap larger than the bound.
	ErrPDFPageTooLarge = errors.New("PDF page is too large to render")
	// ErrPDFPageOutOfRange: the requested page does not exist.
	ErrPDFPageOutOfRange = errors.New("PDF page index out of range")
	// ErrPDFRenderUnavailable: the isolated renderer cannot run on this host (no poppler, no prlimit).
	ErrPDFRenderUnavailable = errors.New("PDF rendering is not available on this server")
	// ErrPDFRenderTimeout: the tool did not finish within the time bound (wall clock or CPU limit).
	ErrPDFRenderTimeout = errors.New("PDF rendering timed out")
	// ErrPDFRenderCanceled: the caller (client) went away; not a server-side timeout.
	ErrPDFRenderCanceled = errors.New("PDF rendering canceled by the caller")
	// ErrPDFBusy: every reader slot is in use; try again later.
	ErrPDFBusy = errors.New("PDF reader is busy")
)

// PDFReaderSlots is the number of reader requests (buffer + count + render) that run at once.
// Together with the pdfConcurrentSlots conversion slots it sizes the per-tool address-space limit
// (pdfToolLimits), so the sum of all poppler children stays within half of MemAvailable.
const PDFReaderSlots = 2

var pdfReaderSem = make(chan struct{}, PDFReaderSlots)

// AcquirePDFReaderSlot takes a reader slot without waiting. ok=false means the reader is
// saturated (the caller answers 429). The returned release must be called exactly once.
func AcquirePDFReaderSlot() (release func(), ok bool) {
	select {
	case pdfReaderSem <- struct{}{}:
		var once sync.Once
		return func() { once.Do(func() { <-pdfReaderSem }) }, true
	default:
		return func() {}, false
	}
}

// classifyPDFToolError maps a failure of an isolated poppler run to one of the sentinel errors.
// The tool's stderr is part of err only for logs; callers must not send err text to clients.
func classifyPDFToolError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, context.DeadlineExceeded):
		return fmt.Errorf("%w: %v", ErrPDFRenderTimeout, err)
	case errors.Is(err, context.Canceled):
		return fmt.Errorf("%w: %v", ErrPDFRenderCanceled, err)
	case errors.Is(err, errPrlimitMissing), errors.Is(err, exec.ErrNotFound):
		return fmt.Errorf("%w: %v", ErrPDFRenderUnavailable, err)
	case strings.Contains(err.Error(), "output exceeds"):
		return fmt.Errorf("%w: %v", ErrPDFPageTooLarge, err)
	case childKilledBy(err, syscall.SIGXFSZ):
		return fmt.Errorf("%w: %v", ErrPDFPageTooLarge, err)
	case childKilledBy(err, syscall.SIGXCPU), childKilledBy(err, syscall.SIGKILL):
		// prlimit sets soft == hard for RLIMIT_CPU, so the kernel ends the child with SIGKILL (not
		// SIGXCPU) at the limit. A SIGKILL while the context is still live is the CPU limit (or the
		// kernel OOM killer): either way the document needs more than the server grants.
		return fmt.Errorf("%w: %v", ErrPDFRenderTimeout, err)
	case strings.Contains(err.Error(), "Incorrect password"), strings.Contains(err.Error(), "needs a password"):
		return fmt.Errorf("%w: %v", ErrPDFEncrypted, err)
	default:
		return fmt.Errorf("%w: %v", ErrPDFInvalid, err)
	}
}

// childKilledBy reports whether err wraps an exit caused by the given signal (the kernel sends
// SIGXCPU at RLIMIT_CPU and SIGXFSZ at RLIMIT_FSIZE).
func childKilledBy(err error, sig syscall.Signal) bool {
	var ee *exec.ExitError
	if !errors.As(err, &ee) {
		return false
	}
	ws, ok := ee.Sys().(syscall.WaitStatus)
	return ok && ws.Signaled() && ws.Signal() == sig
}

func absPDFPath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	// An absolute path starts with "/" and so can never be read as an option by the tools.
	return abs, nil
}

// PDFPageCount returns the number of pages of the PDF at path using pdfinfo in an isolated child.
func PDFPageCount(ctx context.Context, path string) (int, error) {
	abs, err := absPDFPath(path)
	if err != nil {
		return 0, err
	}
	ctx, cancel := context.WithTimeout(ctx, pdfRenderTimeout)
	defer cancel()
	return pdfPageCount(ctx, abs)
}

// pdfPageCount is PDFPageCount without its own deadline (the caller owns the deadline).
func pdfPageCount(ctx context.Context, abs string) (int, error) {
	out, err := runPopplerTool(ctx, "pdfinfo", []string{abs}, pdfInfoMaxBytes, 0)
	if err != nil {
		return 0, classifyPDFToolError(err)
	}
	for _, l := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(l, "Pages:") {
			n, perr := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(l, "Pages:")))
			if perr != nil || n < 1 {
				return 0, fmt.Errorf("%w: bad page count %q", ErrPDFInvalid, l)
			}
			return n, nil
		}
	}
	return 0, fmt.Errorf("%w: pdfinfo reported no page count", ErrPDFInvalid)
}

// pdfPageSize returns the size in points of the 1-based page of the PDF.
func pdfPageSize(ctx context.Context, abs string, page int) (w, h float64, err error) {
	p := strconv.Itoa(page)
	out, err := runPopplerTool(ctx, "pdfinfo", []string{"-f", p, "-l", p, abs}, pdfInfoMaxBytes, 0)
	if err != nil {
		return 0, 0, classifyPDFToolError(err)
	}
	for _, l := range strings.Split(string(out), "\n") {
		if !strings.HasPrefix(l, "Page") || !strings.Contains(l, "size:") {
			continue
		}
		f := strings.Fields(strings.SplitN(l, "size:", 2)[1])
		if len(f) >= 3 && f[1] == "x" {
			w, e1 := strconv.ParseFloat(f[0], 64)
			h, e2 := strconv.ParseFloat(f[2], 64)
			if e1 == nil && e2 == nil && w > 0 && h > 0 {
				return w, h, nil
			}
		}
	}
	return 0, 0, fmt.Errorf("%w: no page size for page %d", ErrPDFInvalid, page)
}

// RenderPDFPagePNG renders the ZERO-based page of the PDF at path to PNG bytes at dpi (<= 0 means
// PDFRenderDPI). It checks the page index against pdfinfo, refuses a page whose bitmap would exceed
// the pixel bound, and runs pdftoppm in an isolated child writing into a private temp directory
// that is always removed.
func RenderPDFPagePNG(ctx context.Context, path string, page int, dpi float64) ([]byte, error) {
	if page < 0 {
		return nil, ErrPDFPageOutOfRange
	}
	if dpi <= 0 {
		dpi = PDFRenderDPI
	}
	abs, err := absPDFPath(path)
	if err != nil {
		return nil, err
	}
	// One overall deadline for count, page size and render together.
	ctx, cancel := context.WithTimeout(ctx, pdfRenderTimeout)
	defer cancel()
	total, err := pdfPageCount(ctx, abs)
	if err != nil {
		return nil, err
	}
	if page >= total {
		return nil, ErrPDFPageOutOfRange
	}
	pw, ph, err := pdfPageSize(ctx, abs, page+1)
	if err != nil {
		return nil, err
	}
	scale := dpi / 72.0
	if px := pw * scale * ph * scale; math.IsNaN(px) || px > pdfMaxRenderPixels {
		return nil, ErrPDFPageTooLarge
	}

	dir, err := os.MkdirTemp("", "catalogizer-pdfpage-*")
	if err != nil {
		return nil, fmt.Errorf("create render directory: %w", err)
	}
	defer func() {
		if dir != "" && filepath.Base(dir) != "" && strings.HasPrefix(filepath.Base(dir), "catalogizer-pdfpage-") {
			_ = os.RemoveAll(dir)
		}
	}()
	// -cropbox: pdfinfo reports the CropBox size, so pdftoppm must rasterise the CropBox too (by
	// default it uses the MediaBox, which can be far larger than the checked size, review F1).
	// pdftoppm cannot write PNG to stdout, so it writes <root>.png; the root is absolute (no option
	// reading) and inside the private directory.
	root := filepath.Join(dir, "page")
	p := strconv.Itoa(page + 1)
	args := []string{"-png", "-singlefile", "-cropbox", "-r", strconv.FormatFloat(dpi, 'f', -1, 64), "-f", p, "-l", p, abs, root}
	if _, err := runPopplerTool(ctx, "pdftoppm", args, 4096, pdfMaxPNGBytes); err != nil {
		return nil, classifyPDFToolError(err)
	}
	outPath := root + ".png"
	fi, err := os.Lstat(outPath)
	if err != nil || !fi.Mode().IsRegular() {
		return nil, fmt.Errorf("%w: renderer produced no image", ErrPDFInvalid)
	}
	if fi.Size() <= 0 || fi.Size() > pdfMaxPNGBytes {
		return nil, ErrPDFPageTooLarge
	}
	b, err := os.ReadFile(outPath)
	if err != nil {
		return nil, fmt.Errorf("read rendered page: %w", err)
	}
	return b, nil
}

// runPopplerTool runs a poppler tool (resolved on PATH) under prlimit via runLimited.
// maxFile is the largest file the tool may write (0 = none).
func runPopplerTool(ctx context.Context, tool string, args []string, outMax int, maxFile uint64) ([]byte, error) {
	bin, err := lookPath(tool)
	if err != nil {
		return nil, fmt.Errorf("%s not installed: %w", tool, err)
	}
	lim := pdfToolLimits(pdfPopplerMaxAS)
	lim.fileSize = maxFile
	// Poppler output is parsed in English: do not inherit the service's locale (review F9).
	lim.env = append(lim.env, "LC_ALL=C", "LANG=C")
	return runLimited(ctx, bin, args, lim, outMax)
}
