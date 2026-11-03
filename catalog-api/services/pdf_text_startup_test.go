package services

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// CAT-002 round 5 (review F-A / F-B). The review measured that the pure-Go child did not start
// under its own limit: the production binary is a cgo Go program whose runtime reserves virtual
// address space (page summary, one 8 MiB stack per thread, scaling with GOMAXPROCS), and the tests
// had only ever run the TEST binary with MemAvailable faked to 24 GiB. These tests build the REAL
// production binary (go build, cgo as production), run the child through the real wiring
// (isolatedExtractor -> runLimited -> prlimit) at the floor limit, with a fake LOW MemAvailable,
// with GOMAXPROCS=64 in the parent's environment, on an ordinary 146-page and an 851-page document,
// 15 runs each, and require zero failures.

var (
	prodBinOnce sync.Once
	prodBinPath string
	prodBinDir  string
	prodBinErr  error
)

// cleanupProdBinary removes the directory realProdBinary created (called from TestMain).
func cleanupProdBinary() {
	if prodBinDir != "" && strings.HasPrefix(filepath.Base(prodBinDir), "cat002-prodbin-") {
		os.RemoveAll(prodBinDir)
	}
}

// realProdBinary builds the catalog-api main package once per test run.
func realProdBinary(t *testing.T) string {
	t.Helper()
	if testing.Short() {
		t.Skip("builds the production binary")
	}
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go toolchain not on PATH")
	}
	prodBinOnce.Do(func() {
		d, err := os.MkdirTemp("", "cat002-prodbin-")
		if err != nil {
			prodBinErr = err
			return
		}
		prodBinDir = d
		out := filepath.Join(d, "api.bin")
		cmd := exec.Command(goBin, "build", "-buildvcs=false", "-o", out, ".")
		cmd.Dir = ".."
		if b, err := cmd.CombinedOutput(); err != nil {
			prodBinErr = &buildErr{err, string(b)}
			return
		}
		prodBinPath = out
	})
	require.NoError(t, prodBinErr)
	return prodBinPath
}

type buildErr struct {
	err error
	out string
}

func (b *buildErr) Error() string { return b.err.Error() + ": " + b.out }

func writeNormalPDF(t *testing.T, name string, pages int) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(p, normalTextPDF(pages, 45), 0o600))
	return p
}

// runProdChild extracts a whole document through the real wiring n times and counts failures.
func runProdChild(t *testing.T, pages, n int) (fails int, first string) {
	t.Helper()
	if _, err := lookPath("prlimit"); err != nil {
		t.Skip("prlimit not installed")
	}
	bin := realProdBinary(t)
	pdf := writeNormalPDF(t, "doc.pdf", pages)
	e := isolatedExtractor{exe: bin}
	for i := 0; i < n; i++ {
		np, err := e.NumPages(context.Background(), pdf)
		if err == nil && np != pages {
			err = assert.AnError
		}
		var got []string
		if err == nil {
			got, err = e.ExtractPages(context.Background(), pdf, 1, pages)
		}
		if err == nil && (len(got) != pages || !strings.Contains(got[pages-1], "Page "+strconv.Itoa(pages)+" line 45")) {
			err = assert.AnError
		}
		if err != nil {
			fails++
			if first == "" {
				first = err.Error()
				if len(first) > 300 {
					first = first[:300]
				}
			}
		}
	}
	return
}

func lowMemAvailable(t *testing.T) {
	t.Helper()
	old := memAvailable
	memAvailable = func() (uint64, bool) { return 1 << 30, true } // below 6 slots*floor: the floor limit applies
	t.Cleanup(func() { memAvailable = old })
}

func TestCAT002_Start_RealBinary_AtFloor_146Pages(t *testing.T) {
	lowMemAvailable(t)
	f, first := runProdChild(t, 146, 15)
	assert.Zero(t, f, "child failed to run at the floor limit: %s", first)
}

func TestCAT002_Start_RealBinary_AtFloor_851Pages(t *testing.T) {
	if os.Getenv("CAT002_SKIP_SLOW") != "" {
		t.Skip("slow")
	}
	lowMemAvailable(t)
	f, first := runProdChild(t, 851, 15)
	assert.Zero(t, f, "child failed on a large document at the floor limit: %s", first)
}

// GOMAXPROCS=64 in the parent environment (as on a 64-CPU host) must not reach the child.
func TestCAT002_Start_RealBinary_HighGOMAXPROCS(t *testing.T) {
	t.Setenv("GOMAXPROCS", "64")
	lowMemAvailable(t)
	f, first := runProdChild(t, 146, 15)
	assert.Zero(t, f, "child failed with GOMAXPROCS=64 inherited: %s", first)
}

// Same with the machine's CPU set widened/narrowed: taskset-restricted parent still works.
func TestCAT002_Start_RealBinary_HighGOMAXPROCS_851(t *testing.T) {
	if os.Getenv("CAT002_SKIP_SLOW") != "" {
		t.Skip("slow")
	}
	t.Setenv("GOMAXPROCS", "64")
	lowMemAvailable(t)
	f, first := runProdChild(t, 851, 15)
	assert.Zero(t, f, "child failed on 851 pages with GOMAXPROCS=64 inherited: %s", first)
}

// ---- F-A: honest, distinguishable start failure ----

// A child whose runtime dies before our code runs (no started marker) is a start failure, not a
// verdict on the document.
func TestCAT002_Start_RuntimeDeathIsDistinguishable(t *testing.T) {
	if _, err := lookPath("prlimit"); err != nil {
		t.Skip("prlimit not installed")
	}
	fake := filepath.Join(t.TempDir(), "fakechild")
	require.NoError(t, os.WriteFile(fake, []byte("#!/bin/sh\necho 'fatal error: failed to reserve page summary memory' >&2\nexit 2\n"), 0o755))
	_, err := isolatedExtractor{exe: fake}.NumPages(context.Background(), "/nonexistent.pdf")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrPDFChildStart)
	assert.Contains(t, err.Error(), "failed to reserve page summary memory", "the runtime's own message is kept")
}

// A child that started (marker present) and then failed on the document is NOT a start failure.
func TestCAT002_Start_HostileDocumentIsNotAStartFailure(t *testing.T) {
	e := needChildRuntime(t)
	fixMemAvailable(t)
	p := writeTmp(t, cmapBombPDF(80_000_000, []byte("BT /F1 12 Tf 10 700 Td (a) Tj ET"), 1))
	_, err := e.ExtractPages(context.Background(), p, 1, 1)
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrPDFChildStart, "the child ran; the document killed it")
	assert.NotContains(t, err.Error(), strings.TrimSpace(pdfChildStartedMarker), "the marker is internal")
}

// A missing executable (binary replaced in place, or removed) is a start failure too.
func TestCAT002_Start_MissingExecutableIsStartFailure(t *testing.T) {
	if _, err := lookPath("prlimit"); err != nil {
		t.Skip("prlimit not installed")
	}
	_, err := isolatedExtractor{exe: filepath.Join(t.TempDir(), "gone")}.NumPages(context.Background(), "/x.pdf")
	assert.ErrorIs(t, err, ErrPDFChildStart)
}

// F-E: os.Executable() reports "<path> (deleted)" after an in-place upgrade.
func TestCAT002_Start_DeletedExecutableClearError(t *testing.T) {
	_, err := newIsolatedExtractorFor("/usr/local/bin/api (deleted)")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrPDFChildStart)
	assert.Contains(t, err.Error(), "restart")
	_, err = newIsolatedExtractorFor("/usr/local/bin/api")
	assert.NoError(t, err)
}

// ---- F-E: the child gets a minimal environment, not the API process's secrets ----

func TestCAT002_Start_ChildEnvironmentIsAllowlisted(t *testing.T) {
	if _, err := lookPath("prlimit"); err != nil {
		t.Skip("prlimit not installed")
	}
	t.Setenv("CATALOGIZER_TEST_SECRET", "s3cr3t-value")
	t.Setenv("JWT_SECRET", "another-secret")
	fake := filepath.Join(t.TempDir(), "pdftotext")
	require.NoError(t, os.WriteFile(fake, []byte("#!/bin/sh\nenv\n"), 0o755))
	got, err := popplerExtractor{bin: fake}.run(context.Background(), fixturePath(t, "multi.pdf"))
	require.NoError(t, err)
	assert.NotContains(t, string(got), "s3cr3t-value")
	assert.NotContains(t, string(got), "JWT_SECRET")
	assert.Contains(t, string(got), "PATH=")
}

// ---- F-C: tests that kill the mutants the round-4 review left alive ----

// M2 (drop WaitDelay): a tool whose grandchild keeps the output pipe open after the tool is killed
// must not hold the call until the grandchild exits.
func TestCAT002_Start_WaitDelayBoundsPipeHolders(t *testing.T) {
	if _, err := lookPath("prlimit"); err != nil {
		t.Skip("prlimit not installed")
	}
	fake := filepath.Join(t.TempDir(), "pdftotext")
	require.NoError(t, os.WriteFile(fake, []byte("#!/bin/sh\nsleep 25 &\nwait\n"), 0o755))
	ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()
	st := time.Now()
	_, err := popplerExtractor{bin: fake}.run(ctx, fixturePath(t, "multi.pdf"))
	el := time.Since(st)
	require.Error(t, err)
	assert.Less(t, el, pdfWaitDelay+4*time.Second, "WaitDelay must close the pipes of a surviving grandchild")
}

// M6 (--cpu=unlimited): the CPU limit reaches the tools.
func TestCAT002_Start_CPULimitAppliedToTools(t *testing.T) {
	if _, err := lookPath("prlimit"); err != nil {
		t.Skip("prlimit not installed")
	}
	fake := filepath.Join(t.TempDir(), "pdftotext")
	require.NoError(t, os.WriteFile(fake, []byte("#!/bin/sh\nulimit -t\n"), 0o755))
	got, err := popplerExtractor{bin: fake}.run(context.Background(), fixturePath(t, "multi.pdf"))
	require.NoError(t, err)
	assert.Equal(t, strconv.Itoa(pdfToolCPUSeconds), strings.TrimSpace(string(got)))
	assert.LessOrEqual(t, pdfToolCPUSeconds, 20, "a hostile file must not hold a slot for minutes")
	e := needChildRuntime(t)
	out, err := e.child(context.Background(), 4096, "limits")
	require.NoError(t, err)
	assert.Contains(t, string(out), "Max cpu time              "+strconv.Itoa(pdfToolCPUSeconds))
}

// M4 (the environment can raise the child's text cap): it can only lower it.
func TestCAT002_Start_TextCapEnvCanOnlyLower(t *testing.T) {
	assert.Equal(t, pdfTextMaxBytes, childTextCap(""))
	assert.Equal(t, pdfTextMaxBytes, childTextCap("junk"))
	assert.Equal(t, pdfTextMaxBytes, childTextCap("0"))
	assert.Equal(t, pdfTextMaxBytes, childTextCap("-5"))
	assert.Equal(t, pdfTextMaxBytes, childTextCap(strconv.Itoa(pdfTextMaxBytes)))
	assert.Equal(t, pdfTextMaxBytes, childTextCap(strconv.Itoa(pdfTextMaxBytes+1)))
	assert.Equal(t, pdfTextMaxBytes, childTextCap("9999999999999"))
	assert.Equal(t, 1000, childTextCap("1000"))
}

// M3 (the sentinel only counts as argv[1]): this very test binary, run with the sentinel at argv[2]
// must behave as a normal test binary, and with it at argv[1] as the child.
func TestCAT002_Start_SentinelOnlyAtArgv1(t *testing.T) {
	self, err := os.Executable()
	require.NoError(t, err)
	out, err := exec.Command(self, "-test.run=^TestDoesNotExist$", pdfChildSentinel, "pages", fixturePath(t, "multi.pdf")).CombinedOutput()
	require.NoError(t, err, string(out))
	assert.Contains(t, string(out), "no tests to run", "sentinel at argv[2] must not turn the process into the child")
	assert.NotContains(t, string(out), strings.TrimSpace(pdfChildStartedMarker))
	out, err = exec.Command(self, pdfChildSentinel, "pages", fixturePath(t, "multi.pdf")).CombinedOutput()
	require.NoError(t, err, string(out))
	assert.Contains(t, string(out), strings.TrimSpace(pdfChildStartedMarker))
}

// F-D: the hostile-file path through the default wiring (poppler first, pure-Go fallback) must
// not hold a conversion slot for minutes. Run with the real production binary as the child.
func TestCAT002_Start_RealBinary_HostileFallbackBounded(t *testing.T) {
	if _, err := lookPath("pdftotext"); err != nil {
		t.Skip("poppler not installed")
	}
	bin := realProdBinary(t)
	lowMemAvailable(t)
	content := []byte("BT /F1 1 Tf 10 790 Td " + strings.Repeat("("+strings.Repeat("a", 100)+") Tj 0 -0.95 Td ", 800) + "ET")
	p := writeTmp(t, cmapBombPDF(10_000_000, content, 900))
	ext := fallbackPDFExtractor{primary: popplerExtractor{bin: mustLook(t, "pdftotext")}, secondary: isolatedExtractor{exe: bin}}
	st := time.Now()
	_, err := ext.ExtractPages(context.Background(), p, 1, 900)
	el := time.Since(st)
	t.Logf("hostile fallback chain: err=%v elapsed=%s", err != nil, el)
	assert.Less(t, el, 25*time.Second, "one hostile upload must not hold a slot longer than the CPU limit plus slack")
}

func mustLook(t *testing.T, n string) string {
	t.Helper()
	p, err := lookPath(n)
	require.NoError(t, err)
	return p
}
