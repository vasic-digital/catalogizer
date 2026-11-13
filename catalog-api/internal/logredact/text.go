package logredact

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

// Free-text redaction (PX-F17, structural round 3). Text is ONE left-to-right tokenizing pass: at every position it
// recognises exactly one shape (URL userinfo, credential header, secret path segment, key/value pair, auth scheme) in a fixed
// priority order and consumes the WHOLE value once. There are no independent layers whose order matters, so a value
// consumed by one rule can never be half-eaten by another (the "Authorization=Bearer eyJ..." leak of round 2).
//
// Shapes (priority order, first match wins):
//
//  1. "://" ... "@"          URL userinfo (everything between "://" and the LAST '@' of the whitespace-delimited token);
//  2. HEADER ":" value       credential header (authorization, cookie, x-api-key ...): the whole value to end of line;
//  3. /share/<seg>           the segment after a secret-introducing path segment (SecretPath rule, here inside prose);
//  4. KEY sep VALUE          KEY is SensitiveKey; sep is '=' or ':' (optionally after a closing quote, escaped quote
//     included, and with spaces/tabs/newlines around it) or, for strong keys, a single TAB;
//     VALUE is a quoted string (plain or backslash-escaped quotes, unterminated runs to end of
//     line) or an unquoted run. A value that is itself an auth scheme (Authorization=Bearer x)
//     also takes the token that follows it;
//  5. bearer|basic|digest|negotiate TOKEN   anywhere: the following token.
//
// False-positive policy (documented): diagnostic text prefers over-redaction. A name that merely CONTAINS a credential word is
// redacted; weak names (code, key, pass ...) only with '=' or as a quoted JSON key; scheme words basic/digest/negotiate redact
// only a credential-looking token ("basic configuration failed" survives), bearer always redacts the next token.
const (
	redactedMark = "REDACTED"
	// ErrorTextMaxBytes is the cap of the redacted error text that reaches a log line.
	ErrorTextMaxBytes = 2048
	// textPreCapBytes bounds the redaction CPU: input beyond it is dropped BEFORE redaction.
	textPreCapBytes = 64 << 10
)

var headerNames = map[string]bool{
	"authorization": true, "proxy-authorization": true, "cookie": true, "set-cookie": true, "x-api-key": true,
	"x-auth-token": true, "x-csrf-token": true, "x-xsrf-token": true, "x-amz-security-token": true,
	"www-authenticate": true, "proxy-authenticate": true,
}

var schemeWords = map[string]bool{"bearer": true, "basic": true, "digest": true, "negotiate": true}

func isWordByte(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '.' || c == '-'
}

func isSpaceByte(c byte) bool { return c == ' ' || c == '\t' || c == '\r' || c == '\n' }

func eol(s string, k int) int {
	for k < len(s) && s[k] != '\n' && s[k] != '\r' {
		k++
	}
	return k
}

// closeQuoteLen returns the length of a quote token at s[k:] (`"`, `'`, `\"`, `\'`), or 0.
func closeQuoteLen(s string, k int) int {
	if k < len(s) && (s[k] == '"' || s[k] == '\'') {
		return 1
	}
	if k+1 < len(s) && s[k] == '\\' && (s[k+1] == '"' || s[k+1] == '\'') {
		return 2
	}
	return 0
}

// scanValue returns the end of the value starting at k (k when there is none).
func scanValue(s string, k int) int {
	n := len(s)
	if k >= n {
		return k
	}
	// escaped-quote string: \"....\"
	if s[k] == '\\' && k+1 < n && (s[k+1] == '"' || s[k+1] == '\'') {
		q := s[k+1]
		for m := k + 2; m+1 < n; m++ {
			if s[m] == '\n' || s[m] == '\r' {
				return m
			}
			if s[m] == '\\' && s[m+1] == q {
				return m + 2
			}
		}
		return eol(s, k)
	}
	if s[k] == '"' || s[k] == '\'' {
		q := s[k]
		for m := k + 1; m < n; m++ {
			if s[m] == '\\' {
				m++
				continue
			}
			if s[m] == q {
				return m + 1
			}
		}
		return eol(s, k) // unterminated: everything to end of line is the value
	}
	m := k
	for m < n {
		c := s[m]
		if isSpaceByte(c) || c == '&' || c == ';' || c == ',' || c == '"' || c == '\'' {
			break
		}
		if c == '\\' && m+1 < n && (s[m+1] == '"' || s[m+1] == '\'') {
			break
		}
		m++
	}
	return m
}

func credentialLike(tok string) bool {
	if len(tok) >= 24 || strings.ContainsAny(tok, "0123456789._~+/=") {
		return true
	}
	hasLower, upperInside := false, false
	for i := 0; i < len(tok); i++ {
		c := tok[i]
		if c >= 'a' && c <= 'z' {
			hasLower = true
		} else if c >= 'A' && c <= 'Z' && i > 0 {
			upperInside = true
		}
	}
	return hasLower && upperInside
}

// takeSchemeToken, given a scheme word ending at j, returns the end of the credential token that follows it (j when none or
// when the scheme is basic/digest/negotiate and the token does not look like a credential).
func takeSchemeToken(s string, scheme string, j int) int {
	k := j
	for k < len(s) && (s[k] == ' ' || s[k] == '\t') {
		k++
	}
	if k == j {
		return j
	}
	e := scanValue(s, k)
	if e == k {
		return j
	}
	if strings.ToLower(scheme) != "bearer" && !credentialLike(s[k:e]) {
		return j
	}
	return e
}

func redactText(s string, truncated bool) string {
	n := len(s)
	var b strings.Builder
	b.Grow(n)
	i := 0
	for i < n {
		c := s[i]
		// 1. URL userinfo
		if c == ':' && strings.HasPrefix(s[i:], "://") {
			j := i + 3
			end := j
			for end < n && !isSpaceByte(s[end]) && s[end] != '"' && s[end] != '\'' && s[end] != '\\' {
				end++
			}
			auth := s[j:end]
			b.WriteString("://")
			if at := strings.LastIndexByte(auth, '@'); at >= 0 {
				b.WriteString(redactedMark + "@")
				i = j + at + 1 // rest of the token (host, path, query) is scanned normally
			} else if truncated && end == n && auth != "" && !strings.Contains(auth, "/") {
				b.WriteString(redactedMark) // the cut may have taken the '@': never show a userinfo prefix
				i = end
			} else {
				i = j
			}
			continue
		}
		if !isWordByte(c) {
			b.WriteByte(c)
			i++
			continue
		}
		j := i
		for j < n && isWordByte(s[j]) {
			j++
		}
		run := s[i:j]
		l := strings.ToLower(run)

		// 2. credential header
		if headerNames[l] {
			k := j
			if q := closeQuoteLen(s, k); q > 0 {
				k += q
			}
			for k < n && (s[k] == ' ' || s[k] == '\t') {
				k++
			}
			if k < n && s[k] == ':' {
				k++
				for k < n && (s[k] == ' ' || s[k] == '\t') {
					k++
				}
				b.WriteString(s[i:k])
				b.WriteString(redactedMark)
				i = eol(s, k)
				continue
			}
		}

		// 3. secret path segment inside prose: /share/<seg>
		if i > 0 && s[i-1] == '/' && j < n && s[j] == '/' && secretSegmentKeys[l] {
			e := j + 1
			for e < n && !isSpaceByte(s[e]) && s[e] != '/' && s[e] != '?' && s[e] != '#' && s[e] != '"' && s[e] != '\'' {
				e++
			}
			if e > j+1 {
				b.WriteString(run + "/:redacted")
				i = e
				continue
			}
		}

		// 4. key sep value
		strong := strongKey(l)
		weak := weakKeys[l]
		if strong || weak {
			k := j
			quoted := false
			if q := closeQuoteLen(s, k); q > 0 {
				k += q
				quoted = true
			}
			for k < n && (s[k] == ' ' || s[k] == '\t') {
				k++
			}
			vstart := -1
			if k < n && (s[k] == '=' || s[k] == ':') && (s[k] == '=' || strong || quoted) {
				k++
				for k < n && isSpaceByte(s[k]) {
					k++
				}
				vstart = k
			} else if strong && !quoted && j < n && s[j] == '\t' {
				k = j + 1
				for k < n && (s[k] == ' ' || s[k] == '\t') {
					k++
				}
				vstart = k
			}
			if vstart >= 0 {
				if vend := scanValue(s, vstart); vend > vstart {
					val := s[vstart:vend]
					if schemeWords[strings.ToLower(val)] {
						if e := takeSchemeToken(s, val, vend); e > vend {
							vend = e
						}
					}
					b.WriteString(s[i:vstart])
					b.WriteString(redactedMark)
					i = vend
					continue
				}
			}
		}

		// 5. auth scheme anywhere
		if schemeWords[l] {
			if e := takeSchemeToken(s, l, j); e > j {
				k := j
				for k < n && (s[k] == ' ' || s[k] == '\t') {
					k++
				}
				b.WriteString(s[i:k])
				b.WriteString(redactedMark)
				i = e
				continue
			}
		}

		b.WriteString(run)
		i = j
	}
	return b.String()
}

// Text redacts credential-shaped substrings of s (see the package comment of this file). The input is cut at 64 KiB BEFORE
// redaction to bound CPU; the cut backs off to whitespace (so a token is not split) and arms the truncated-authority rule.
func Text(s string) string {
	truncated := false
	if len(s) > textPreCapBytes {
		cut := textPreCapBytes
		for back := 0; back < 512 && cut > 0 && !isSpaceByte(s[cut-1]); back++ {
			cut--
		}
		if cut == 0 || !isSpaceByte(s[cut-1]) {
			cut = textPreCapBytes
		}
		for cut > 0 && !utf8.RuneStart(s[cut]) {
			cut--
		}
		s = s[:cut]
		truncated = true
	}
	return redactText(s, truncated)
}

// Cap shortens s to at most max bytes (cut moved back to a rune start) and appends a truncation marker.
func Cap(s string, max int) string {
	if len(s) <= max {
		return s
	}
	cut := max
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut] + "...[truncated " + strconv.Itoa(len(s)-cut) + " bytes]"
}

// ErrorText is the pipeline for any error / free text written to a log: bounded pre-cap, ONE redaction pass, then the output
// cap (redact first, so a secret straddling the output cut is never exposed as a prefix).
func ErrorText(s string) string { return Cap(Text(s), ErrorTextMaxBytes) }
