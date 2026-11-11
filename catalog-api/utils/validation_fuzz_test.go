// Package utils — idempotency fuzz tests for Sanitize* / Normalize*
// helpers.
//
// Paired with middleware/input_validation_fuzz_test.go. Same contract:
// any function we label "sanitise" or "normalise" is a multi-hop-safe
// fixed point — SanitizeString(SanitizeString(x)) == SanitizeString(x)
// for every input that a fuzzer can surface.
//
// FIX-QA-2026-04-21-009 proved this class of bug slips past hand-
// written tests. Running these fuzzers against every Sanitize* /
// Normalize* function prevents the same class of bug from shipping
// through sibling helpers.

package utils

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// sanitiseSeeds — the corpus. Null bytes and invalid UTF-8 are
// constructed via string concatenation to keep them out of the
// file's source bytes (a literal NUL in Go source is a lex error).
func sanitiseSeeds() []string {
	nul := "\x00"
	invalidUtf8 := "\xdd"
	return []string{
		"",
		"plain ascii",
		"<script>alert(1)</script>",
		nul + " null bytes " + nul,
		invalidUtf8 + " 0",                                      // FIX-QA-2026-04-21-009 kind of input
		"\xff\xfe invalid utf8",
		strings.Repeat("A", 10_000),
		"日本語",
		"mixed " + nul + "日本 " + invalidUtf8 + " end",
		"   leading and trailing   ",
		"<p>nested <b>tags</b></p>",
		"&lt;already escaped&gt;",
		"\r\n\t control",
	}
}

// FuzzSanitizeString_Idempotent asserts
// SanitizeString(SanitizeString(x)) == SanitizeString(x).
func FuzzSanitizeString_Idempotent(f *testing.F) {
	for _, s := range sanitiseSeeds() {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		first := SanitizeString(s)
		second := SanitizeString(first)
		if first != second {
			t.Fatalf("SanitizeString is not idempotent:\n  input:  %q\n  first:  %q\n  second: %q",
				s, first, second)
		}
	})
}

// FuzzSanitizeString_NoPanic + valid-UTF-8 preservation.
func FuzzSanitizeString_NoPanic(f *testing.F) {
	for _, s := range sanitiseSeeds() {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		out := SanitizeString(s)
		if utf8.ValidString(s) && !utf8.ValidString(out) {
			t.Fatalf("sanitiser introduced invalid UTF-8 from valid input: in=%q out=%q", s, out)
		}
	})
}

// FuzzSanitizeHTML_Idempotent — HTML tag stripping must be a fixed
// point. After one pass, no HTML tags should remain.
func FuzzSanitizeHTML_Idempotent(f *testing.F) {
	htmlSeeds := []string{
		"",
		"<p>plain</p>",
		"<script>alert('xss')</script>",
		"<a href='//evil'>x</a>",
		"text <b>bold</b> more",
		"<<>>",
		"<incomplete",
		"<a><b><c>deeply nested</c></b></a>",
		"stray > and < chars",
		"<!-- comment -->",
	}
	for _, s := range htmlSeeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		first := SanitizeHTML(s)
		second := SanitizeHTML(first)
		if first != second {
			t.Fatalf("SanitizeHTML is not idempotent:\n  input:  %q\n  first:  %q\n  second: %q",
				s, first, second)
		}
	})
}

// FuzzNormalizeUnicode_Idempotent — control-char stripping is a
// fixed point. After one pass, no non-whitespace control chars should
// remain, so a second call is a no-op.
func FuzzNormalizeUnicode_Idempotent(f *testing.F) {
	nul := "\x00"
	seeds := []string{
		"",
		"plain text",
		nul + " null",
		"\x01\x02\x03 control",
		"\n newline kept",
		"\t tab kept",
		"\r carriage return kept",
		"日本語 " + nul + " mixed",
		"​ zero-width",
		"normal \x07 bell",
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		first := NormalizeUnicode(s)
		second := NormalizeUnicode(first)
		if first != second {
			t.Fatalf("NormalizeUnicode is not idempotent:\n  input:  %q\n  first:  %q\n  second: %q",
				s, first, second)
		}
	})
}
