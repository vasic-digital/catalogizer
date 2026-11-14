package handlers

// PXF23-F2: the class-exhaustive source sweep (TestPXF23R5b_SourceSweep) must catch an error VALUE passed bare to a
// client-facing sink, whatever the variable is called. Each fixture below is a deliberately leaking handler written
// to a temp directory by the test (it is never compiled, never part of any package of the product); the sweep reads it
// back from disk exactly as it reads a real handler file.
//
// Unit-class: the sweep is a name/type heuristic over go/ast, not the Go type checker. Known remaining gap, pinned by
// TestPXF23F2_KnownGaps so a change of behaviour is noticed: an error stored in a struct field, a multi-value-call result with a non-synonym name, a direct call result (gin.H{"m": do()}), a variable or closure parameter declared with a concrete error-implementing type other than `error` (var pe *os.PathError).

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const f2Head = "package fixture\n\nimport (\n\t\"fmt\"\n\t\"github.com/gin-gonic/gin\"\n)\n\n"

type f2Case struct {
	name string
	src  string
	want int // number of leaking sinks the sweep must report
}

var f2Cases = []f2Case{
	// shapes the name heuristic already caught (positive controls)
	{"bare err in gin.H", f2Head + "func h(c *gin.Context, err error) {\n\tc.JSON(500, gin.H{\"error\": err})\n}\n", 1},
	{"bare err to c.String", f2Head + "func h(c *gin.Context) {\n\terr := do()\n\tc.String(500, \"%v\", err)\n}\n", 1},
	// shapes the name heuristic missed (PXF23-F2)
	{"error var with an error-synonym name (:= from single call)", f2Head + "func h(c *gin.Context) {\n\tfailure := do()\n\tif failure != nil {\n\t\tc.JSON(500, gin.H{\"m\": failure})\n\t}\n}\n", 1},
	{"error-synonym name from a multi-value call, never compared", f2Head + "func h(c *gin.Context) {\n\t_, boom := pair()\n\tc.JSON(500, gin.H{\"m\": boom})\n}\n", 1},
	{"var x error (name without a synonym) then Sprintf into the sink", f2Head + "func h(c *gin.Context) {\n\tvar lastFault error = do()\n\tc.JSON(500, gin.H{\"m\": fmt.Sprintf(\"failed: %v\", lastFault)})\n}\n", 1},
	{"error-typed parameter (name without a synonym)", f2Head + "func h(c *gin.Context, problem error) {\n\tc.JSON(500, gin.H{\"m\": problem})\n}\n", 1},
	{"error-typed named result (name without a synonym)", f2Head + "func h(c *gin.Context) (res int, outcome error) {\n\toutcome = do()\n\tc.JSON(500, gin.H{\"m\": outcome})\n\treturn\n}\n", 1},
	// negative controls: must stay silent
	{"bool found from a multi-value call", f2Head + "func h(c *gin.Context) {\n\tn, found := lookup()\n\tc.JSON(200, gin.H{\"n\": n, \"found\": found})\n}\n", 0},
	{"bool ok from a multi-value call", f2Head + "func h(c *gin.Context) {\n\tn, ok := lookup()\n\tc.JSON(200, gin.H{\"n\": n, \"found\": ok})\n}\n", 0},
	{"fixed text, error only logged", f2Head + "func h(c *gin.Context, e error) {\n\tlog.Printf(\"%v\", e)\n\tc.JSON(500, gin.H{\"error\": \"internal error\"})\n}\n", 0},
	{"error value only compared", f2Head + "func h(c *gin.Context, e error) {\n\tif e != nil {\n\t\tc.JSON(500, gin.H{\"error\": \"failed\"})\n\t}\n}\n", 0},
	{"authored-text mapper", f2Head + "func h(c *gin.Context, e error) {\n\tc.JSON(400, gin.H{\"error\": validationText(e)})\n}\n", 0},
	// SendErrorResponse(c, status, message, err): at HEAD the helper only logs its err argument (utils
	// TestSendErrorResponse_DoesNotLeakInternalErrorText pins that), so the err slot is deliberately not scanned.
	{"SendErrorResponse err slot (helper logs only)", f2Head + "func h(c *gin.Context, e error) {\n\tutils.SendErrorResponse(c, 500, \"failed\", e)\n}\n", 0},
	{"SendErrorResponse message slot is scanned", f2Head + "func h(c *gin.Context, e error) {\n\tutils.SendErrorResponse(c, 500, \"failed: \"+e.Error(), nil)\n}\n", 1},
}

func f2Sweep(t *testing.T, dir string, c f2Case) []string {
	t.Helper()
	p := filepath.Join(dir, "fixture.go")
	require.NoError(t, os.WriteFile(p, []byte(c.src), 0o600))
	b, err := os.ReadFile(p)
	require.NoError(t, err)
	hits, err := r5bSweepSource(p, string(b))
	require.NoError(t, err)
	return hits
}

func TestPXF23F2_BareErrorArgumentsToSinks(t *testing.T) {
	dir := t.TempDir()
	for _, c := range f2Cases {
		hits := f2Sweep(t, dir, c)
		assert.Len(t, hits, c.want, "%s: sweep reported %v", c.name, hits)
	}
}

// The remaining gaps are pinned, not hidden: both need the Go type checker (an error value that is neither declared as
// `error` in the function nor given an error-synonym name). If one starts being reported, update the doc and this pin.
func TestPXF23F2_KnownGaps(t *testing.T) {
	gaps := []f2Case{
		{"error-typed struct field", f2Head + "type holder struct{ Cause error }\n\nfunc h(c *gin.Context, v holder) {\n\tc.JSON(500, gin.H{\"m\": v.Cause})\n}\n", 0},
		{"non-synonym name from a multi-value call", f2Head + "func h(c *gin.Context) {\n\t_, kaboom := pair()\n\tc.JSON(500, gin.H{\"m\": kaboom})\n}\n", 0},
		{"M6: direct call result in the sink (gin.H{\"m\": do()})", f2Head + "func h(c *gin.Context) {\n\tc.JSON(500, gin.H{\"m\": do()})\n}\n", 0},
		{"M7: var of a qualified error-implementing type (var pe *os.PathError)", f2Head + "func h(c *gin.Context) {\n\tvar pe *os.PathError = do()\n\tc.JSON(500, gin.H{\"m\": pe})\n}\n", 0},
		{"M8: error-implementing closure parameter (func(x *os.PathError))", f2Head + "func h(c *gin.Context) {\n\tfn := func(x *os.PathError) {\n\t\tc.JSON(500, gin.H{\"m\": x})\n\t}\n\tfn(nil)\n}\n", 0},
	}
	for _, g := range gaps {
		assert.Empty(t, f2Sweep(t, t.TempDir(), g), "UNCONFIRMED gap: %s", g.name)
	}
}

// Known FALSE-POSITIVE class (PXF18DOC-F2, measured 2026-11-14): the error-synonym name heuristic fires on a NON-error
// value that merely carries a synonym name (here a bool called `fail` from a multi-value call). This is a deliberate
// over-report of the sweep (it fails closed: a reviewer renames the variable or allow-lists the line); it is pinned so
// a change of behaviour is noticed and so the doc can state the class instead of leaving it undiscovered.
func TestPXF23F2_KnownFalsePositives(t *testing.T) {
	fp := f2Case{"bool named like an error synonym (fail) from a multi-value call", f2Head + "func h(c *gin.Context) {\n\tn, fail := lookup()\n\tc.JSON(200, gin.H{\"n\": n, \"fail\": fail})\n}\n", 1}
	assert.Len(t, f2Sweep(t, t.TempDir(), fp), fp.want, "known false positive: %s", fp.name)
}
