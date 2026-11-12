package errleak

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func hits(t *testing.T, src string) []Site {
	t.Helper()
	s, err := Sweep("x.go", src)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// TestSweepControls: the instrument must see known leaks (positive controls) and must not flag clean code
// (negative controls); a fabricated needle must yield zero.
func TestSweepControls(t *testing.T) {
	pos := map[string]string{
		"direct":        "package p\nfunc f(c *gin.Context, err error) {\n\tc.JSON(500, gin.H{\"error\": err.Error()})\n}\n",
		"concat":        "package p\nfunc f(c *gin.Context, err error) {\n\tutils.SendErrorResponse(c, 400, \"bad: \"+err.Error(), nil)\n}\n",
		"variable":      "package p\nfunc f(c *gin.Context, err error) {\n\tmsg := err.Error()\n\tc.JSON(500, gin.H{\"error\": msg})\n}\n",
		"variable2hop":  "package p\nfunc f(c *gin.Context, err error) {\n\ta := fmt.Sprintf(\"x %v\", err)\n\tb := a + \"!\"\n\tc.String(500, b)\n}\n",
		"httpError":     "package p\nfunc f(w http.ResponseWriter, err error) {\n\thttp.Error(w, err.Error(), 500)\n}\n",
		"wrapperSink":   "package p\nfunc (h *H) f(w http.ResponseWriter, err error) {\n\th.sendError(w, fmt.Sprintf(\"x: %v\", err), 400)\n}\n",
		"fprintfWriter": "package p\nfunc f(w http.ResponseWriter, err error) {\n\tfmt.Fprintf(w, \"bad %v\", err)\n}\n",
		"mapCarried":    "package p\nfunc f(c *gin.Context, err error) {\n\tr := gin.H{}\n\tr[\"error\"] = err.Error()\n\tc.JSON(200, r)\n}\n",
		"detailsKey":    "package p\nfunc f(c *gin.Context, err *gin.Error) {\n\tc.JSON(400, gin.H{\"error\": \"x\", \"details\": err.Error()})\n}\n",
	}
	for name, src := range pos {
		if got := hits(t, src); len(got) != 1 {
			t.Errorf("positive control %s: want 1 hit, got %v", name, got)
		}
	}
	neg := map[string]string{
		"fixed":    "package p\nfunc f(c *gin.Context, err error) {\n\tlog.Printf(\"%v\", err)\n\tc.JSON(500, gin.H{\"error\": \"internal error\"})\n}\n",
		"mapper":   "package p\nfunc f(c *gin.Context, err error) {\n\tc.JSON(400, gin.H{\"error\": \"bad: \" + validationText(err)})\n}\n",
		"sentinel": "package p\nfunc f(c *gin.Context) {\n\tc.JSON(409, gin.H{\"error\": services.ErrSmartLimit.Error()})\n}\n",
		"stderr":   "package p\nfunc f(stderr io.Writer, err error) {\n\tfmt.Fprintf(stderr, \"FAILED: %v\", err)\n}\n",
		"logger":   "package p\nfunc f(logger *zap.Logger, err error) {\n\tlogger.Error(\"x\", zap.Error(err))\n}\n",
		"fixedvar": "package p\nfunc f(c *gin.Context, err error) {\n\tlog.Print(err)\n\tmsg := \"nope\"\n\tc.JSON(500, gin.H{\"error\": msg})\n}\n",
		"scrub":    "package p\nfunc f(c *gin.Context, err error) {\n\tr := gin.H{}\n\tr[\"error\"] = services.ScrubErrorFor(err, root)\n\tc.JSON(200, r)\n}\n",
		"needle":   "package p\nfunc f(c *gin.Context) {\n\tc.JSON(200, gin.H{\"ok\": \"ZZ-fabricated-needle-0\"})\n}\n",
	}
	for name, src := range neg {
		if got := hits(t, src); len(got) != 0 {
			t.Errorf("negative control %s: want 0 hits, got %v", name, got)
		}
	}
}

// allowed: sites that were read and classified as intentionally client-visible. Key = file|func|snippet; the
// function is part of the key so a copy of the snippet elsewhere in the file is NOT covered (R5B-F4).
var allowed = map[string]string{
	"handlers/conversion_handler.go|CreateJob|=c.JSON(http.StatusBadRequest, gin.H{\"error\": \"Invalid conversion request\", \"reason\": reason})": "AUTHORED: reason is the text of models.ErrInvalidConversionRequest authored by services/conversion_service.go (R5B-2)",
	"handlers/log_management_handler.go|writeLogServiceError|=http.Error(w, msg, status)":                                                           "FIXED: msg comes from a switch over fixed literals; only the status is derived from err",
	"handlers/media_browse_handler.go|GetMediaStats|=c.Error(rowsErr)":                                                                              "PRIVATE-GIN-ERROR: gin.ErrorTypePrivate; internal/middleware.ErrorHandler answers a fixed 'Internal server error' (CAT-WP02-ERRLEAK)",
	"handlers/user_handler.go|CreateUser|=http.Error(w, msg, http.StatusBadRequest)":                                                                "MAPPED: msg comes from authClientMessage (line 121); the other msg in the function (err.Error() at ~168) is only compared, never written",
	"internal/handlers/binding_ingestion_handler.go|ProbeAndIngest|~gin.H{\"error\": hostAllowedErr})":                                              "CONST: identifier ends in Err but is a constant string",
	"internal/handlers/smb_discovery.go|DiscoverShares|~gin.H{\"error\": hostAllowedErr})":                                                          "CONST: identifier ends in Err but is a constant string",
	"internal/handlers/smb_discovery.go|ProbeHost|~gin.H{\"error\": hostAllowedErr})":                                                               "CONST: identifier ends in Err but is a constant string",
}

func isAllowed(rel, fn string, lines []string, line int) (key string, ok bool) {
	return isAllowedIn(allowed, rel, fn, lines, line)
}

// isAllowedIn: every entry must be pinned to the flagged line itself ("=" exact statement line, "~" pinned tail);
// there is no unpinned window match, so a new leak next to an allow-listed site is still reported (F2).
func isAllowedIn(al map[string]string, rel, fn string, lines []string, line int) (key string, ok bool) {
	first := strings.TrimSpace(lines[line-1])
	for k := range al {
		p := strings.SplitN(k, "|", 3)
		if p[0] != rel || p[1] != fn {
			continue
		}
		switch {
		case strings.HasPrefix(p[2], "="):
			if first == p[2][1:] {
				return k, true
			}
		case strings.HasPrefix(p[2], "~"):
			if strings.HasSuffix(first, p[2][1:]) {
				return k, true
			}
		}
	}
	return "", false
}

// TestAllowListEntriesArePinned: every allow-list key carries a pin.
func TestAllowListEntriesArePinned(t *testing.T) {
	for k := range allowed {
		p := strings.SplitN(k, "|", 3)
		if len(p) != 3 || !(strings.HasPrefix(p[2], "=") || strings.HasPrefix(p[2], "~")) {
			t.Errorf("unpinned allow-list entry: %s", k)
		}
	}
}

// TestAllowListDoesNotShieldNeighbouringLeak (reviewer mutant M6): a leak inserted two lines above an
// allow-listed statement in the same function must still be reported.
func TestAllowListDoesNotShieldNeighbouringLeak(t *testing.T) {
	src := "package p\nfunc CreateJob(c *gin.Context, err error) {\n\tc.JSON(500, gin.H{\"error\": err.Error()})\n\tc.Abort()\n\tc.JSON(400, gin.H{\"reason\": reason})\n}\n"
	al := map[string]string{"h.go|CreateJob|=c.JSON(400, gin.H{\"reason\": reason})": "AUTHORED"}
	sites, err := Sweep("h.go", src)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(src, "\n")
	var unshielded int
	for _, s := range sites {
		if _, ok := isAllowedIn(al, "h.go", s.Func, lines, s.Line); !ok {
			unshielded++
		}
	}
	if unshielded < 1 {
		t.Fatalf("neighbouring leak at line 3 was shielded by the allow-list: %v", sites)
	}
}

func TestNoErrorTextReachesAResponse(t *testing.T) {
	root := filepath.Join("..", "..") // catalog-api
	var offenders []string
	used := map[string]bool{}
	scanned := 0
	err := filepath.Walk(root, func(p string, i os.FileInfo, e error) error {
		if e != nil {
			return e
		}
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		if i.IsDir() {
			if rel == "internal/errleak" || rel == "tests" || rel == "challenges" || rel == "vendor" || strings.HasPrefix(i.Name(), ".") && rel != "." {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		scanned++
		sites, err := Sweep(rel, string(b))
		if err != nil {
			return fmt.Errorf("parse %s: %w", rel, err)
		}
		lines := strings.Split(string(b), "\n")
		for _, s := range sites {
			if k, ok := isAllowed(rel, s.Func, lines, s.Line); ok {
				used[k] = true
				continue
			}
			offenders = append(offenders, fmt.Sprintf("%s:%d (%s) %s", rel, s.Line, s.Func, strings.TrimSpace(lines[s.Line-1])))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if scanned < 200 {
		t.Fatalf("instrument blind: only %d production files scanned", scanned)
	}
	sort.Strings(offenders)
	if len(offenders) > 0 {
		t.Errorf("response sinks that put an error value on the wire (replace with a fixed message + server-side log keyed by request id, or classify in allowed with a reason):\n%s", strings.Join(offenders, "\n"))
	}
	for k := range allowed {
		if !used[k] {
			t.Errorf("stale allow-list entry (no longer matches a flagged site): %s", k)
		}
	}
}
