package services

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// CAT-002 isolation layer. Three review rounds of bounding the ledongthuc/pdf library field by
// field kept failing because the library has unbounded paths (ToUnicode CMaps, per-Tf CMap
// re-parse, /Contents arrays). Structural answer: no PDF parser runs in the API process. Poppler
// and the pure-Go library both run in child processes under prlimit, killed when ctx ends, with
// bounded stdout and stderr. A parser that eats memory or CPU hits the kernel limit in the child.

const (
	// pdfChildSentinel is argv[1] of the hidden re-exec of this binary that runs the pure-Go
	// extractor. The init below intercepts it before main (or the test runner) parses flags.
	pdfChildSentinel = "__catalogizer_pdftext_child"
	// pdfChildMaxTextEnv lowers the child's total-text cap (tests only; it can never raise it).
	pdfChildMaxTextEnv = "CATALOGIZER_PDFTEXT_MAX_TEXT"
	// pdfConcurrentSlots is the number of conversions the service runs at once (semaphore of 3 in
	// ConversionService); limits are sized so all slots together stay within half of MemAvailable.
	pdfConcurrentSlots = 3
	// Address-space ceilings and floors per child (bytes).
	pdfChildMaxAS   = 1 << 30
	pdfPopplerMaxAS = 4 << 30
	pdfMinAS        = 512 << 20
	// pdfToolCPUSeconds is the CPU-time limit (RLIMIT_CPU) of a child.
	pdfToolCPUSeconds = 100
	// pdfWaitDelay is how long after a kill os/exec waits for the child's pipes before closing them.
	pdfWaitDelay = 2 * time.Second
	// pdfStderrMax bounds the stderr kept for an error message.
	pdfStderrMax = 4096
)

var (
	errPrlimitMissing = errors.New("refusing to run a PDF parser without prlimit (install util-linux): " +
		"an uncapped parser could exhaust host memory or CPU")
	// lookPath and memAvailable are variables so tests can simulate a host without a tool.
	lookPath     = exec.LookPath
	memAvailable = readMemAvailable
)

// readMemAvailable returns MemAvailable from /proc/meminfo in bytes.
func readMemAvailable() (uint64, bool) {
	b, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, false
	}
	for _, l := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(l, "MemAvailable:") {
			f := strings.Fields(l)
			if len(f) >= 2 {
				if kb, err := strconv.ParseUint(f[1], 10, 64); err == nil {
					return kb << 10, true
				}
			}
		}
	}
	return 0, false
}

// pdfLimits are the prlimit values of one child.
type pdfLimits struct {
	addressSpace uint64
	cpuSeconds   int
}

// pdfToolLimits sizes the address-space limit to the host (§12.11): MemAvailable divided by twice
// the number of concurrent conversions, clamped to [pdfMinAS, ceiling]. The floor (512 MiB) keeps
// ordinary documents working on a small host; when MemAvailable cannot be read the floor is used.
func pdfToolLimits(ceiling uint64) pdfLimits {
	as := uint64(pdfMinAS)
	if avail, ok := memAvailable(); ok {
		as = avail / (2 * pdfConcurrentSlots)
		if as > ceiling {
			as = ceiling
		}
		if as < pdfMinAS {
			as = pdfMinAS
		}
	}
	return pdfLimits{addressSpace: as, cpuSeconds: pdfToolCPUSeconds}
}

// runLimited runs bin with args under prlimit (no shell), kills it when ctx ends or its stdout
// exceeds outMax, bounds stderr, and returns stdout. Without prlimit it refuses.
func runLimited(ctx context.Context, bin string, args []string, lim pdfLimits, outMax int) ([]byte, error) {
	pl, err := lookPath("prlimit")
	if err != nil {
		return nil, errPrlimitMissing
	}
	full := append([]string{
		"--as=" + strconv.FormatUint(lim.addressSpace, 10),
		"--cpu=" + strconv.Itoa(lim.cpuSeconds),
		"--core=0", "--fsize=0",
		"--", bin,
	}, args...)
	inner, cancel := context.WithCancel(ctx)
	defer cancel()
	cmd := exec.CommandContext(inner, pl, full...)
	cmd.WaitDelay = pdfWaitDelay
	var stdout, stderr limitedBuffer
	stdout.max, stderr.max = outMax, pdfStderrMax
	stdout.onOver = cancel
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	runErr := cmd.Run()
	if cerr := ctx.Err(); cerr != nil {
		return nil, cerr
	}
	if stdout.over {
		return nil, fmt.Errorf("output exceeds %d bytes", outMax)
	}
	if runErr != nil {
		return nil, fmt.Errorf("%w: %s", runErr, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}

// refusingPDFExtractor answers every call with err (used when the safe runtime is not available).
type refusingPDFExtractor struct{ err error }

func (r refusingPDFExtractor) NumPages(context.Context, string) (int, error) { return 0, r.err }
func (r refusingPDFExtractor) ExtractPages(context.Context, string, int, int) ([]string, error) {
	return nil, r.err
}

// isolatedExtractor runs the pure-Go extractor in a prlimit-capped child process.
type isolatedExtractor struct {
	exe  string   // binary to run; the production value is this very executable
	pre  []string // arguments before the sentinel (none in production)
	maxA uint64   // address-space ceiling; 0 means pdfChildMaxAS
}

func newIsolatedExtractor() (isolatedExtractor, error) {
	exe, err := os.Executable()
	if err != nil {
		return isolatedExtractor{}, fmt.Errorf("cannot locate own executable for the PDF child: %w", err)
	}
	return isolatedExtractor{exe: exe}, nil
}

func (e isolatedExtractor) limits() pdfLimits {
	c := e.maxA
	if c == 0 {
		c = pdfChildMaxAS
	}
	return pdfToolLimits(c)
}

func (e isolatedExtractor) child(ctx context.Context, outMax int, op string, args ...string) ([]byte, error) {
	a := append(append([]string{}, e.pre...), pdfChildSentinel, op)
	out, err := runLimited(ctx, e.exe, append(a, args...), e.limits(), outMax)
	if err != nil {
		return nil, fmt.Errorf("built-in PDF extractor: %w", err)
	}
	return out, nil
}

func (e isolatedExtractor) NumPages(ctx context.Context, path string) (int, error) {
	out, err := e.child(ctx, 64, "pages", path)
	if err != nil {
		return 0, err
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return 0, fmt.Errorf("built-in PDF extractor: bad page count %q", out)
	}
	return n, nil
}

func (e isolatedExtractor) ExtractPages(ctx context.Context, path string, start, end int) ([]string, error) {
	if start < 1 || end < start || end-start > 1<<20 {
		return nil, fmt.Errorf("invalid page range %d-%d", start, end)
	}
	// frames: 8-byte length + text per page. Bound: text cap plus the frame headers.
	outMax := pdfTextMaxBytes + 8*(end-start+1)
	out, err := e.child(ctx, outMax, "extract", path, strconv.Itoa(start), strconv.Itoa(end))
	if err != nil {
		return nil, err
	}
	pages := make([]string, 0, end-start+1)
	for len(out) > 0 {
		if len(out) < 8 {
			return nil, errors.New("built-in PDF extractor: truncated output")
		}
		n := binary.BigEndian.Uint64(out)
		out = out[8:]
		if n > uint64(len(out)) {
			return nil, errors.New("built-in PDF extractor: truncated output")
		}
		pages = append(pages, string(out[:n]))
		out = out[n:]
	}
	if len(pages) != end-start+1 {
		return nil, fmt.Errorf("built-in PDF extractor: got %d pages, want %d", len(pages), end-start+1)
	}
	return pages, nil
}

func init() {
	if len(os.Args) >= 2 && os.Args[1] == pdfChildSentinel {
		os.Exit(pdfChildMain(os.Args[2:], os.Stdout, os.Stderr))
	}
}

// pdfChildMain is the body of the child process. Ops: pages <path>; extract <path> <start> <end>;
// limits (prints the address-space limit line of /proc/self/limits, for the self-check test).
func pdfChildMain(args []string, stdout, stderr io.Writer) (code int) {
	defer func() {
		if p := recover(); p != nil {
			fmt.Fprintf(stderr, "pdf child panic: %v\n", p)
			code = 3
		}
	}()
	if len(args) == 0 {
		fmt.Fprintln(stderr, "pdf child: missing op")
		return 2
	}
	ctx, cancel := context.WithTimeout(context.Background(), pdfTextTimeout)
	defer cancel()
	le := ledongthucExtractor{}
	switch args[0] {
	case "limits":
		b, _ := os.ReadFile("/proc/self/limits")
		for _, l := range strings.Split(string(b), "\n") {
			if strings.HasPrefix(l, "Max address space") {
				fmt.Fprintln(stdout, l)
			}
		}
		return 0
	case "pages":
		if len(args) != 2 {
			fmt.Fprintln(stderr, "pdf child: usage: pages <path>")
			return 2
		}
		n, err := le.NumPages(ctx, args[1])
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		fmt.Fprintln(stdout, n)
		return 0
	case "extract":
		if len(args) != 4 {
			fmt.Fprintln(stderr, "pdf child: usage: extract <path> <start> <end>")
			return 2
		}
		start, e1 := strconv.Atoi(args[2])
		end, e2 := strconv.Atoi(args[3])
		if e1 != nil || e2 != nil || start < 1 || end < start {
			fmt.Fprintln(stderr, "pdf child: bad page range")
			return 2
		}
		max := pdfTextMaxBytes
		if v, err := strconv.Atoi(os.Getenv(pdfChildMaxTextEnv)); err == nil && v > 0 && v < max {
			max = v
		}
		pages, err := le.extractPages(ctx, args[1], start, end, max)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		w := bufio.NewWriter(stdout)
		var hdr [8]byte
		for _, p := range pages {
			binary.BigEndian.PutUint64(hdr[:], uint64(len(p)))
			w.Write(hdr[:])
			w.WriteString(p)
		}
		if err := w.Flush(); err != nil {
			return 1
		}
		return 0
	}
	fmt.Fprintln(stderr, "pdf child: unknown op")
	return 2
}
