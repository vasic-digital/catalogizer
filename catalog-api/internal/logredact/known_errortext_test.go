package logredact

import (
	"fmt"
	"strings"
	"testing"
)

// PKGB-FU1: ErrorTextKnown cuts before ReplaceKnown (bounded cost) without exposing a secret that straddles the cut.
func TestErrorTextKnown_StraddlingSecretNeverExposed(t *testing.T) {
	secret := strings.Repeat("Qx7", 1000) // 3000 bytes, synthetic
	limit := textPreCapBytes + maxKnownSecretLen
	for _, off := range []int{textPreCapBytes - 2999, textPreCapBytes - 1500, textPreCapBytes - 5, limit - 3000, limit - 2999, limit - 1500, limit - 4, limit - 1, limit} {
		in := strings.Repeat("a ", off/2) + secret + strings.Repeat(" b", 100)
		out := ErrorTextKnown(in, []string{secret})
		for n := 8; n <= len(secret); n += 97 {
			if strings.Contains(out, secret[:n]) {
				t.Fatalf("off=%d: secret prefix of %d bytes reached output", off, n)
			}
		}
	}
}

func TestErrorTextKnown_ReplacesAndCaps(t *testing.T) {
	out := ErrorTextKnown("lookup share Zq9Xv7Lm2Rt5Bw8K failed", []string{"Zq9Xv7Lm2Rt5Bw8K"})
	if !strings.Contains(out, "lookup share") || strings.Contains(out, "Zq9Xv7Lm") {
		t.Fatalf("bad output %q", out)
	}
	if got := ErrorTextKnown(strings.Repeat("x ", 5<<20), nil); len(got) > ErrorTextMaxBytes+64 {
		t.Fatalf("not capped: %d", len(got))
	}
	if ErrorTextKnown("", []string{"abcd"}) != "" {
		t.Fatal("empty in, empty out")
	}
}

// Deterministic cost oracle (no wall clock): ReplaceKnown never receives more than textPreCapBytes+maxKnownSecretLen bytes.
func TestErrorTextKnown_ReplaceKnownInputBounded(t *testing.T) {
	limit := textPreCapBytes + maxKnownSecretLen
	var seen int
	old := replaceKnownFn
	replaceKnownFn = func(s string, secrets []string) string {
		if len(s) > seen {
			seen = len(s)
		}
		return old(s, secrets)
	}
	defer func() { replaceKnownFn = old }()
	huge := strings.Repeat("filler text without secrets ", (16<<20)/28)
	_ = ErrorTextKnown(huge, []string{"abcdwxyz"})
	if seen == 0 {
		t.Fatal("control needle: ReplaceKnown was not called")
	}
	if seen > limit {
		t.Fatalf("ReplaceKnown received %d bytes, limit %d", seen, limit)
	}
	if ErrorTextKnown(huge, []string{"abcdwxyz"}) != ErrorTextKnown(huge[:limit], []string{"abcdwxyz"}) {
		t.Fatal("output differs between huge text and its pre-cut prefix")
	}
}

// F2: a ~3000-byte secret starting at ~2000 straddles ErrorTextMaxBytes (2048). A pre-cut at 2048 would leave a 48-byte
// secret prefix that ReplaceKnown cannot match and that the output keeps.
func TestErrorTextKnown_SecretStraddlingOutputCapNotExposed(t *testing.T) {
	secret := strings.Repeat("Qx7", 1000)
	in := strings.Repeat("a ", 1000) + secret + strings.Repeat(" b", 100)
	out := ErrorTextKnown(in, []string{secret})
	if !strings.Contains(out, "a a a") {
		t.Fatalf("control needle: filler missing from output")
	}
	for _, n := range []int{8, 16, 48} {
		if strings.Contains(out, secret[:n]) {
			t.Fatalf("secret prefix of %d bytes reached output", n)
		}
	}
}

// HANDLERS-RED-1-F3 (c): an encoded slash (%2F) inside the share token decodes into extra path segments; none of them may survive.
func TestSecretPath_DecodedSlashesInsideSecretDoNotLeak(t *testing.T) {
	for in, want := range map[string]string{
		"/logs/share//enc/Tok":        "/logs/share//:redacted",
		"/api/v1/logs/share//a/b/c":   "/api/v1/logs/share//:redacted",
		"/reset///S1/x":               "/reset///:redacted",
		"/api/v1/logs/share/Tok/more": "/api/v1/logs/share/:redacted/more",
		"/api/v1/logs/share//":        "/api/v1/logs/share//",
	} {
		if got := SecretPath(in); got != want {
			t.Errorf("SecretPath(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPathKnown_RemovesKnownSecretFromAnySegment(t *testing.T) {
	got := PathKnown("/zz/items/Rt5Bw8KZq9Xv7Lm2/x", "ticket=AbCd1234", []string{"Rt5Bw8KZq9Xv7Lm2"})
	if strings.Contains(got, "Rt5Bw8K") || !strings.Contains(got, "/zz/items/") || strings.Contains(got, "AbCd1234") {
		t.Errorf("PathKnown = %q", got)
	}
}

// HYG-F6: a known secret that contains a quote or backslash appears in its %q-quoted form inside an error text
// (fmt %q escapes it); the quoted form must be removed too.
func TestReplaceKnown_RemovesQuotedFormOfSecret(t *testing.T) {
	const sec = `ab"cd12`
	text := fmt.Errorf("lookup %q failed", sec).Error() // lookup "ab\"cd12" failed
	if !strings.Contains(text, `ab\"cd12`) {
		t.Fatalf("control needle: quoted form missing from %q", text)
	}
	got := ErrorTextKnown(text, []string{sec})
	if strings.Contains(got, `cd12`) || strings.Contains(got, `ab`) {
		t.Fatalf("quoted secret survived: %q", got)
	}
	if !strings.Contains(got, `lookup "`+redactedMark+`" failed`) {
		t.Fatalf("surrounding text lost: %q", got)
	}
	// negative control: a secret without special characters has no extra quoted variant and benign text is untouched.
	if got := ReplaceKnown("keep ab cd12 here", []string{sec}); got != "keep ab cd12 here" {
		t.Fatalf("unrelated text changed: %q", got)
	}
}

// HYG-R1-F4: the %+q form (non-ASCII escaped as \u....) of a secret is removed too.
func TestReplaceKnown_RemovesPlusQuotedFormOfSecret(t *testing.T) {
	const sec = "p\u00e4ss\"x12"
	text := fmt.Sprintf("lookup %+q failed", sec)
	if !strings.Contains(text, `p\u00e4ss`) {
		t.Fatalf("control needle: escaped form missing from %q", text)
	}
	got := ErrorTextKnown(text, []string{sec})
	if strings.Contains(got, `u00e4`) || strings.Contains(got, `x12`) {
		t.Fatalf("%%+q secret survived: %q", got)
	}
}
