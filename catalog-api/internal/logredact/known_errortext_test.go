package logredact

import (
	"strings"
	"testing"
	"time"
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

func TestErrorTextKnown_BoundedCost(t *testing.T) {
	var secs []string
	for i := 0; i < 64; i++ {
		secs = append(secs, strings.Repeat(string(rune('a'+i%26)), 4000)+string(rune('A'+i%26)))
	}
	huge := strings.Repeat("filler text without secrets ", (16<<20)/28)
	best := time.Hour
	for i := 0; i < 3; i++ {
		t0 := time.Now()
		_ = ErrorTextKnown(huge, secs)
		if d := time.Since(t0); d < best {
			best = d
		}
	}
	if best > 60*time.Millisecond {
		t.Fatalf("ErrorTextKnown over 16 MiB took %v", best)
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
