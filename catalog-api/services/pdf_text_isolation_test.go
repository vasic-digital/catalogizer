package services

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// CAT-002 structural round: no PDF parser runs in the API process. These tests drive the real
// extractor wiring (isolatedExtractor re-execs this very test binary through the init hook in
// pdf_text_isolation.go; popplerExtractor runs the real pdftotext) with hostile files and measure
// the PARENT process: allocation stays small, the call returns within a bound, no child survives.

// hideTools makes lookPath fail for the given tool names (everything else resolves normally).
func hideTools(t *testing.T, names ...string) {
	t.Helper()
	old := lookPath
	lookPath = func(f string) (string, error) {
		for _, n := range names {
			if f == n {
				return "", errors.New("hidden by test: " + n)
			}
		}
		return old(f)
	}
	t.Cleanup(func() { lookPath = old })
}

// fixMemAvailable pins the host memory reading (MemAvailable moves between two calls).
func fixMemAvailable(t *testing.T) {
	t.Helper()
	old := memAvailable
	memAvailable = func() (uint64, bool) { return 24 << 30, true }
	t.Cleanup(func() { memAvailable = old })
}

// buildRawPDF writes objects 1..n (objs[i] is the body of object i+1) with a valid xref.
func buildRawPDF(objs []string) []byte {
	var b bytes.Buffer
	var offs []int
	b.WriteString("%PDF-1.4\n")
	for i, o := range objs {
		offs = append(offs, b.Len())
		fmt.Fprintf(&b, "%d 0 obj%sendobj\n", i+1, o)
	}
	x := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(offs)+1)
	for _, o := range offs {
		fmt.Fprintf(&b, "%010d 00000 n \n", o)
	}
	fmt.Fprintf(&b, "trailer<</Size %d/Root 1 0 R>>\nstartxref\n%d\n%%%%EOF\n", len(offs)+1, x)
	return b.Bytes()
}

func rawStream(data []byte) string {
	z := zlibBytes(data)
	return fmt.Sprintf("<</Length %d/Filter/FlateDecode>>stream\n%s\nendstream ", len(z), z)
}

// cmapBombPDF: pages 1..pages use font F1 with a ToUnicode CMap of cmapTokens tokens.
func cmapBombPDF(cmapTokens int, content []byte, pages int) []byte {
	kids := make([]string, pages)
	for i := range kids {
		kids[i] = fmt.Sprintf("%d 0 R", 7+i)
	}
	objs := []string{
		"<</Type/Catalog/Pages 2 0 R>>",
		fmt.Sprintf("<</Type/Pages/Kids[%s]/Count %d>>", strings.Join(kids, " "), pages),
		"<</Type/Font/Subtype/Type1/BaseFont/Helvetica/ToUnicode 5 0 R>>",
		"<<>>",
		rawStream(bytes.Repeat([]byte("1 "), cmapTokens)),
		rawStream(content),
	}
	for i := 0; i < pages; i++ {
		objs = append(objs, "<</Type/Page/Parent 2 0 R/MediaBox[0 0 600 800]/Resources<</Font<</F1 3 0 R>>>>/Contents 6 0 R>>")
	}
	return buildRawPDF(objs)
}

// contentsPastLimitPDF: a /Contents array of 4096 harmless refs plus one giant stream at index 4096.
func contentsPastLimitPDF() []byte {
	big := []byte("BT /F1 12 Tf 10 700 Td (" + strings.Repeat("a", 1<<20) + ") Tj ET")
	refs := strings.Repeat("5 0 R ", 4096) + "6 0 R"
	return buildRawPDF([]string{
		"<</Type/Catalog/Pages 2 0 R>>",
		"<</Type/Pages/Kids[4 0 R]/Count 1>>",
		"<</Type/Font/Subtype/Type1/BaseFont/Helvetica/FirstChar 32/LastChar 126/Widths[" + strings.Repeat("500 ", 95) + "]>>",
		"<</Type/Page/Parent 2 0 R/MediaBox[0 0 600 800]/Resources<</Font<</F1 3 0 R>>>>/Contents[" + refs + "]>>",
		"<</Length 3>>stream\nq Q\nendstream ",
		rawStream(big),
	})
}

// hugeOutputContent: ~50 KB of poppler text per page; 1500 pages of it exceed pdfTextMaxBytes.
func hugeOutputContent() []byte {
	return []byte("BT /F1 1 Tf 10 790 Td " + strings.Repeat("("+strings.Repeat("a", 170)+") Tj 0 -0.95 Td ", 300) + "ET")
}

// parentCost runs fn and reports the parent's allocation and wall time.
func parentCost(fn func()) (alloc uint64, d time.Duration) {
	var a, b runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&a)
	st := time.Now()
	fn()
	runtime.ReadMemStats(&b)
	return b.TotalAlloc - a.TotalAlloc, time.Since(st)
}

func needChildRuntime(t *testing.T) isolatedExtractor {
	t.Helper()
	if _, err := lookPath("prlimit"); err != nil {
		t.Skip("prlimit not installed")
	}
	e, err := newIsolatedExtractor()
	require.NoError(t, err)
	return e
}

// liveChildrenFor counts processes whose command line contains marker (the pdf path is unique).
func liveChildrenFor(marker string) int {
	ents, _ := os.ReadDir("/proc")
	n := 0
	for _, e := range ents {
		if _, err := strconv.Atoi(e.Name()); err != nil {
			continue
		}
		b, err := os.ReadFile("/proc/" + e.Name() + "/cmdline")
		if err == nil && bytes.Contains(b, []byte(marker)) {
			n++
		}
	}
	return n
}

// F1: Contents entries at index >= 4096 were read unmeasured (26 KB file -> 1 MiB text, 378 MB).
func TestCAT002_Iso_ContentsPastLimitRefused(t *testing.T) {
	e := needChildRuntime(t)
	p := writeTmp(t, contentsPastLimitPDF())
	var err error
	alloc, d := parentCost(func() { _, err = e.ExtractPages(context.Background(), p, 1, 1) })
	require.Error(t, err)
	assert.Contains(t, err.Error(), "/Contents")
	assert.Less(t, alloc, uint64(16<<20), "parent allocation")
	assert.Less(t, d, 30*time.Second)
}

// F2: ToUnicode CMap bomb (20 KB file allocated 1.5 GB in-process). The child hits its data-size
// limit; the parent sees a bounded error. The bomb is sized well past the 1 GiB ceiling: with the
// soft GOMEMLIMIT the collector keeps a 10M-token CMap (the original bomb) under the cap, which is
// still bounded and is not what this test is about.
func TestCAT002_Iso_CMapBombContained(t *testing.T) {
	e := needChildRuntime(t)
	p := writeTmp(t, cmapBombPDF(80_000_000, []byte("BT /F1 12 Tf 10 700 Td (a) Tj ET"), 1))
	var err error
	alloc, d := parentCost(func() { _, err = e.ExtractPages(context.Background(), p, 1, 1) })
	require.Error(t, err, "the bomb must not yield a result inside the data-size cap")
	assert.Less(t, len(err.Error()), 8192, "error message is bounded")
	assert.Less(t, alloc, uint64(16<<20), "parent allocation")
	assert.Less(t, d, 90*time.Second)
}

// F3: per-Tf CMap re-parse (1.3 KB file, 3m38s, 210 GB TotalAlloc, ctx ignored). The child is
// killed when ctx ends; the call returns promptly with the ctx error and nothing survives.
func TestCAT002_Iso_TfReparseCancellable(t *testing.T) {
	e := needChildRuntime(t)
	content := []byte("BT " + strings.Repeat("/F1 12 Tf ", 25000) + "(a) Tj ET")
	p := writeTmp(t, cmapBombPDF(50_000, content, 1))
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var err error
	alloc, d := parentCost(func() { _, err = e.ExtractPages(ctx, p, 1, 1) })
	// Either the child exhausts its address-space limit first (bounded child error) or the
	// deadline kills it (ctx error); the contract is a prompt, bounded error and no survivor.
	require.Error(t, err)
	assert.Less(t, d, 8*time.Second, "returns shortly after the deadline")
	assert.Less(t, alloc, uint64(16<<20))
	time.Sleep(200 * time.Millisecond)
	assert.Zero(t, liveChildrenFor(p), "no child process survives cancellation")
}

// Deterministic cancellation: a child that never ends is killed at the deadline and the ctx
// error is returned.
func TestCAT002_Iso_CancelKillsChild(t *testing.T) {
	if _, err := lookPath("prlimit"); err != nil {
		t.Skip("prlimit not installed")
	}
	slow := filepath.Join(t.TempDir(), "slowchild")
	require.NoError(t, os.WriteFile(slow, []byte("#!/bin/sh\nexec sleep 30\n"), 0o755))
	p := filepath.Join(t.TempDir(), "marker-cancel.pdf")
	ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()
	st := time.Now()
	_, err := isolatedExtractor{exe: slow}.ExtractPages(ctx, p, 1, 1)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Less(t, time.Since(st), 5*time.Second)
	time.Sleep(200 * time.Millisecond)
	assert.Zero(t, liveChildrenFor("marker-cancel.pdf"))
}

// F4: hostile input forces poppler to fail (output over the bound); the fallback child is itself
// capped, so the fall-through is safe. Real wiring, real pdftotext.
func TestCAT002_Iso_PopplerOverflowFallsToCappedChild(t *testing.T) {
	needChildRuntime(t)
	if _, ok := newPopplerExtractor(); !ok {
		t.Skip("pdftotext not installed")
	}
	if _, err := lookPath("pdfinfo"); err != nil {
		t.Skip("pdfinfo not installed")
	}
	// page 1 uses a CMap-bomb font; pages 2..900 print ~80 KB each (> 64 MB total)
	const pages = 1500
	kids := make([]string, pages)
	objs := []string{
		"<</Type/Catalog/Pages 2 0 R>>", "",
		"<</Type/Font/Subtype/Type1/BaseFont/Helvetica>>",
		rawStream(hugeOutputContent()),
		"<</Type/Font/Subtype/Type1/BaseFont/Helvetica/ToUnicode 6 0 R>>",
		rawStream(bytes.Repeat([]byte("1 "), 10_000_000)),
		rawStream([]byte("BT /F2 12 Tf 10 700 Td (hello) Tj ET")),
	}
	for i := 0; i < pages; i++ {
		kids[i] = fmt.Sprintf("%d 0 R", len(objs)+1)
		if i == 0 {
			objs = append(objs, "<</Type/Page/Parent 2 0 R/MediaBox[0 0 600 800]/Resources<</Font<</F2 5 0 R>>>>/Contents 7 0 R>>")
		} else {
			objs = append(objs, "<</Type/Page/Parent 2 0 R/MediaBox[0 0 600 800]/Resources<</Font<</F1 3 0 R>>>>/Contents 4 0 R>>")
		}
	}
	objs[1] = fmt.Sprintf("<</Type/Pages/Kids[%s]/Count %d>>", strings.Join(kids, " "), pages)
	p := writeTmp(t, buildRawPDF(objs))
	ctx, cancel := context.WithTimeout(context.Background(), pdfTextTimeout)
	defer cancel()
	ex := defaultPDFTextExtractor()
	_, isFB := ex.(fallbackPDFExtractor)
	require.True(t, isFB)
	var err error
	alloc, d := parentCost(func() { _, err = ex.ExtractPages(ctx, p, 1, pages) })
	require.Error(t, err, "poppler overflow then the capped child on the CMap bomb: bounded error")
	assert.Less(t, alloc, uint64(512<<20), "parent allocation stays within a few times the output bound")
	assert.Less(t, d, pdfTextTimeout)
	assert.Zero(t, liveChildrenFor(p))
}

// A ctx error never triggers the fallback (R2).
func TestCAT002_Iso_CtxErrorNeverFallsBack(t *testing.T) {
	var calls int32
	sec := countingExtractor{calls: &calls}
	slow := filepath.Join(t.TempDir(), "pdftotext")
	require.NoError(t, os.WriteFile(slow, []byte("#!/bin/sh\nexec sleep 30\n"), 0o755))
	if _, err := lookPath("prlimit"); err != nil {
		t.Skip("prlimit not installed")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	fb := fallbackPDFExtractor{primary: popplerExtractor{bin: slow}, secondary: sec}
	_, err := fb.ExtractPages(ctx, fixturePath(t, "multi.pdf"), 1, 1)
	require.Error(t, err)
	assert.Zero(t, atomic.LoadInt32(&calls), "secondary must not run after the deadline")
	// control: a non-ctx failure of the primary does reach the secondary
	bad := filepath.Join(t.TempDir(), "pdftotext")
	require.NoError(t, os.WriteFile(bad, []byte("#!/bin/sh\nexit 3\n"), 0o755))
	fb = fallbackPDFExtractor{primary: popplerExtractor{bin: bad}, secondary: sec}
	_, _ = fb.ExtractPages(context.Background(), fixturePath(t, "multi.pdf"), 1, 1)
	assert.EqualValues(t, 1, atomic.LoadInt32(&calls))
	// and NumPages
	atomic.StoreInt32(&calls, 0)
	cctx, ccancel := context.WithCancel(context.Background())
	ccancel()
	_, _ = fallbackPDFExtractor{primary: refusingPDFExtractor{err: errors.New("x")}, secondary: sec}.NumPages(cctx, "x")
	assert.Zero(t, atomic.LoadInt32(&calls))
}

type countingExtractor struct{ calls *int32 }

func (c countingExtractor) NumPages(context.Context, string) (int, error) {
	atomic.AddInt32(c.calls, 1)
	return 1, nil
}
func (c countingExtractor) ExtractPages(context.Context, string, int, int) ([]string, error) {
	atomic.AddInt32(c.calls, 1)
	return []string{"x"}, nil
}

// R1: the limits are really applied to the children (read back from the child itself).
func TestCAT002_Iso_LimitsAppliedToChildren(t *testing.T) {
	e := needChildRuntime(t)
	fixMemAvailable(t)
	out, err := e.child(context.Background(), 4096, "limits")
	require.NoError(t, err)
	want := e.limits().dataSpace
	require.NotZero(t, want)
	assert.Contains(t, string(out), "Max data size             "+strconv.FormatUint(want, 10), "child data-size limit")
	assert.Contains(t, string(out), "GOMAXPROCS "+pdfChildGOMAXPROCS, "pinned Go runtime")
	assert.NotContains(t, string(out), "Max cpu time              unlimited")
	// poppler side: a fake pdftotext prints its own ulimit -v (KiB)
	fake := filepath.Join(t.TempDir(), "pdftotext")
	require.NoError(t, os.WriteFile(fake, []byte("#!/bin/sh\nulimit -v\n"), 0o755))
	got, err := popplerExtractor{bin: fake}.run(context.Background(), fixturePath(t, "multi.pdf"))
	require.NoError(t, err)
	assert.Equal(t, strconv.FormatUint(pdfToolLimits(pdfPopplerMaxAS).addressSpace>>10, 10), strings.TrimSpace(string(got)))
}

func TestCAT002_Iso_LimitsAreHostAdaptiveAndBounded(t *testing.T) {
	old := memAvailable
	defer func() { memAvailable = old }()
	memAvailable = func() (uint64, bool) { return 600 << 30, true }
	assert.EqualValues(t, pdfChildMaxData, pdfChildLimits().dataSpace, "ceiling")
	assert.EqualValues(t, pdfPopplerMaxAS, pdfToolLimits(pdfPopplerMaxAS).addressSpace, "poppler ceiling")
	memAvailable = func() (uint64, bool) { return 4 << 30, true }
	// literal on purpose (PX-F5): 4 GiB / (2*(3 conversion slots + 1 indexer child)) = 512 MiB; restating the formula would follow a changed constant
	assert.EqualValues(t, 512<<20, pdfChildLimits().dataSpace, "MemAvailable/(2*(slots+indexer child))")
	memAvailable = func() (uint64, bool) { return 1 << 30, true }
	assert.EqualValues(t, pdfChildMinData, pdfChildLimits().dataSpace, "floor")
	assert.EqualValues(t, pdfMinAS, pdfToolLimits(pdfPopplerMaxAS).addressSpace, "poppler floor")
	memAvailable = func() (uint64, bool) { return 0, false }
	assert.EqualValues(t, pdfChildMinData, pdfChildLimits().dataSpace, "unreadable meminfo")
	assert.Zero(t, pdfChildLimits().addressSpace, "the Go child is bounded by RLIMIT_DATA, not RLIMIT_AS")
}

// Without prlimit nothing is run: honest refusal (also logged by the caller through the error).
func TestCAT002_Iso_RefusesWithoutPrlimit(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "ran")
	fake := filepath.Join(t.TempDir(), "pdftotext")
	require.NoError(t, os.WriteFile(fake, []byte("#!/bin/sh\ntouch "+marker+"\n"), 0o755))
	hideTools(t, "prlimit")
	_, err := popplerExtractor{bin: fake}.run(context.Background(), fixturePath(t, "multi.pdf"))
	require.ErrorIs(t, err, errPrlimitMissing)
	_, statErr := os.Stat(marker)
	assert.True(t, os.IsNotExist(statErr), "tool must not have run uncapped")
	ex := defaultPDFTextExtractor()
	_, err = ex.NumPages(context.Background(), fixturePath(t, "multi.pdf"))
	require.ErrorIs(t, err, errPrlimitMissing)
	_, err = ex.ExtractPages(context.Background(), fixturePath(t, "multi.pdf"), 1, 1)
	require.ErrorIs(t, err, errPrlimitMissing)
}

// R3: stderr of a child is bounded to a few KB in the error message.
func TestCAT002_Iso_StderrBounded(t *testing.T) {
	if _, err := lookPath("prlimit"); err != nil {
		t.Skip("prlimit not installed")
	}
	loud := filepath.Join(t.TempDir(), "pdftotext")
	require.NoError(t, os.WriteFile(loud, []byte("#!/bin/sh\nhead -c 1000000 /dev/zero | tr '\\0' 'e' >&2\nexit 1\n"), 0o755))
	_, err := popplerExtractor{bin: loud}.run(context.Background(), fixturePath(t, "multi.pdf"))
	require.Error(t, err)
	assert.Less(t, len(err.Error()), 8192)
	assert.Contains(t, err.Error(), "eeeeeeee", "the start of stderr is kept")
}

// X2: a tool that keeps running after its output overflowed is killed at the overflow, not at the
// deadline.
func TestCAT002_Iso_OverflowKillsTool(t *testing.T) {
	if _, err := lookPath("prlimit"); err != nil {
		t.Skip("prlimit not installed")
	}
	hang := filepath.Join(t.TempDir(), "pdftotext")
	require.NoError(t, os.WriteFile(hang, []byte("#!/bin/sh\nhead -c 5000 /dev/zero | tr '\\0' 'x'\nexec sleep 30\n"), 0o755))
	st := time.Now()
	_, err := popplerExtractor{bin: hang, maxOut: 1000}.run(context.Background(), fixturePath(t, "multi.pdf"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds 1000 bytes")
	assert.Less(t, time.Since(st), 5*time.Second)
}

// R6/R8: pdfinfo is required for the poppler wiring, runs under prlimit, and an output overflow
// is an error even when a Pages line is present.
func TestCAT002_Iso_PdfinfoPresenceAndOverflow(t *testing.T) {
	needChildRuntime(t)
	fixMemAvailable(t)
	if _, ok := newPopplerExtractor(); !ok {
		t.Skip("pdftotext not installed")
	}
	hideTools(t, "pdfinfo")
	_, isFB := defaultPDFTextExtractor().(fallbackPDFExtractor)
	assert.False(t, isFB, "poppler primary requires pdfinfo")
	_, isIso := defaultPDFTextExtractor().(isolatedExtractor)
	assert.True(t, isIso)

	info := filepath.Join(t.TempDir(), "pdfinfo")
	script := "#!/bin/sh\necho 'Pages: 7'\nhead -c 200000 /dev/zero | tr '\\0' 'x'\n"
	require.NoError(t, os.WriteFile(info, []byte(script), 0o755))
	_, err := popplerExtractor{infoBin: info}.NumPages(context.Background(), fixturePath(t, "multi.pdf"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds")
	// control: under the bound it parses
	n, err := popplerExtractor{infoBin: info, infoMax: 1 << 20}.NumPages(context.Background(), fixturePath(t, "multi.pdf"))
	require.NoError(t, err)
	assert.Equal(t, 7, n)
	// pdfinfo is capped too
	lim := filepath.Join(t.TempDir(), "pdfinfo")
	require.NoError(t, os.WriteFile(lim, []byte("#!/bin/sh\necho \"Pages: $(ulimit -v)\"\n"), 0o755))
	n, err = popplerExtractor{infoBin: lim}.NumPages(context.Background(), fixturePath(t, "multi.pdf"))
	require.NoError(t, err)
	assert.EqualValues(t, pdfToolLimits(pdfPopplerMaxAS).addressSpace>>10, n)
}

// R7: the child's total-text cap (inside the child) is enforced and reports its own limit.
func TestCAT002_Iso_ChildTotalTextCap(t *testing.T) {
	e := needChildRuntime(t)
	t.Setenv(pdfChildMaxTextEnv, "1000")
	content := []byte("BT /F1 1 Tf 10 790 Td " + strings.Repeat("("+strings.Repeat("a", 100)+") Tj 0 -2 Td ", 40) + "ET")
	p := writeTmp(t, simplePDF(content, true, true)) // ~4 KB of text on one page
	_, err := e.ExtractPages(context.Background(), p, 1, 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "extracted text exceeds 1000 bytes")
	// control: without the lowered cap the same file extracts
	t.Setenv(pdfChildMaxTextEnv, "")
	pages, err := e.ExtractPages(context.Background(), p, 1, 1)
	require.NoError(t, err)
	assert.Contains(t, pages[0], strings.Repeat("a", 100))
}

// F10: the overflow message states the limit actually applied.
func TestCAT002_Iso_OverflowMessageStatesActualLimit(t *testing.T) {
	if _, err := lookPath("prlimit"); err != nil {
		t.Skip("prlimit not installed")
	}
	loud := filepath.Join(t.TempDir(), "pdftotext")
	require.NoError(t, os.WriteFile(loud, []byte("#!/bin/sh\nhead -c 5000 /dev/zero | tr '\\0' 'x'\n"), 0o755))
	_, err := popplerExtractor{bin: loud, maxOut: 1000}.run(context.Background(), fixturePath(t, "multi.pdf"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds 1000 bytes")
	assert.NotContains(t, err.Error(), strconv.Itoa(pdfTextMaxBytes))
}

// Round trip through the child for ordinary documents equals the in-process library result.
func TestCAT002_Iso_RoundTripMatchesLibrary(t *testing.T) {
	e := needChildRuntime(t)
	p := fixturePath(t, "multi.pdf")
	n, err := e.NumPages(context.Background(), p)
	require.NoError(t, err)
	want, err := ledongthucExtractor{}.ExtractPages(context.Background(), p, 1, n)
	require.NoError(t, err)
	got, err := e.ExtractPages(context.Background(), p, 1, n)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}
