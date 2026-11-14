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

var schemeWords = map[string]bool{"bearer": true, "basic": true, "digest": true, "negotiate": true, "jwt": true, "token": true}

// alwaysSchemes redact the token that follows them unconditionally; the other scheme words only when it looks like a credential.
var alwaysSchemes = map[string]bool{"bearer": true, "jwt": true}

// '%' is a word byte so that percent-encoded names ("%74oken") stay ONE run; the run is decoded before it is classified.
func isWordByte(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '.' || c == '-' || c == '%'
}

func isSpaceByte(c byte) bool { return c == ' ' || c == '\t' || c == '\r' || c == '\n' }

func isQuote(c byte) bool { return c == '"' || c == '\'' }

func eol(s string, k int) int {
	for k < len(s) && s[k] != '\n' && s[k] != '\r' {
		k++
	}
	return k
}

// quoteAt returns (backslashes, quote byte, total length) of a quote token at s[k:]: an optional run of backslashes followed by
// a quote (`"`, `'`, `\"`, `\\"` ...). Length 0 when there is none. A backslash run not followed by a quote returns (run, 0, 0).
func quoteAt(s string, k int) (bs int, q byte, ln int) {
	t := k
	for t < len(s) && s[t] == '\\' {
		t++
	}
	if t < len(s) && isQuote(s[t]) {
		return t - k, s[t], t - k + 1
	}
	return t - k, 0, 0
}

func closeQuoteLen(s string, k int) int {
	_, _, ln := quoteAt(s, k)
	return ln
}

// scanBracket returns the end of a bracketed value starting at s[k]=='[' : the matching ']' (quote-aware), looking ahead at most
// bracketLookahead bytes. An unmatched '[' takes the rest of the line (so repeated '[' cannot rescan: linear).
const bracketLookahead = 4096

func scanBracket(s string, k int) int {
	n := len(s)
	lim := k + bracketLookahead
	if lim > n {
		lim = n
	}
	depth := 0
	for m := k; m < lim; m++ {
		switch c := s[m]; {
		case c == '[':
			depth++
		case c == ']':
			depth--
			if depth == 0 {
				return m + 1
			}
		case c == '\\':
			m++
		case isQuote(c):
			for m++; m < lim && s[m] != c; m++ {
				if s[m] == '\\' {
					m++
				}
			}
		}
	}
	return eol(s, k)
}

// scanValue returns the end of the value starting at k (k when there is none).
func scanValue(s string, k int) int {
	n := len(s)
	if k >= n {
		return k
	}
	if s[k] == '[' {
		return scanBracket(s, k)
	}
	// escaped-quote string: \"....\" (any number of backslashes; the closer carries the same number)
	if bs, q, ln := quoteAt(s, k); bs > 0 && ln > 0 {
		run := 0
		for m := k + ln; m < n; m++ {
			switch s[m] {
			case '\n', '\r':
				return m
			case '\\':
				run++
				continue
			case q:
				if run == bs {
					return m + 1
				}
			}
			run = 0
		}
		return eol(s, k)
	}
	if isQuote(s[k]) {
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
		if isSpaceByte(c) || c == '&' || c == ';' || c == ',' || isQuote(c) {
			break
		}
		if c == '\\' {
			t := m
			for t < n && s[t] == '\\' {
				t++
			}
			if t < n && isQuote(s[t]) {
				break
			}
			m = t // skip the whole backslash run once (linear)
			continue
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
	if !alwaysSchemes[strings.ToLower(scheme)] && !credentialLike(s[k:e]) {
		return j
	}
	return e
}

// pctDecode decodes %XX escapes of a word run and returns the decoded text plus, for every decoded byte, its raw start offset
// (len(decoded)+1 entries), so a decoded position can be mapped back to the raw run.
func pctDecode(run string) (string, []int) {
	var d strings.Builder
	offs := make([]int, 0, len(run)+1)
	for i := 0; i < len(run); {
		if run[i] == '%' && i+2 < len(run)+0 && isHex(run[i+1]) && isHex(run[i+2]) {
			offs = append(offs, i)
			d.WriteByte(unhex(run[i+1])<<4 | unhex(run[i+2]))
			i += 3
			continue
		}
		offs = append(offs, i)
		d.WriteByte(run[i])
		i++
	}
	offs = append(offs, len(run))
	return d.String(), offs
}

func isHex(c byte) bool { return c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F' }

func unhex(c byte) byte {
	switch {
	case c >= '0' && c <= '9':
		return c - '0'
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10
	}
	return c - 'A' + 10
}

// skipFold, given the end of a redacted header value, also consumes RFC 5322 folded continuation lines (a line break followed by a
// space or tab), so "Authorization: Bearer\n <secret>" does not leak its second line.
func skipFold(s string, i int) int {
	n := len(s)
	for {
		p := i
		if p < n && s[p] == '\r' {
			p++
		}
		if p < n && s[p] == '\n' {
			p++
		} else if p == i {
			return i
		}
		if p < n && (s[p] == ' ' || s[p] == '\t') {
			i = eol(s, p)
			continue
		}
		return i
	}
}

// valueSeparator parses the separator after a key (k just after the key or its closing quote): '=' or ':' (also "=>", ":=",
// "->"), spaces/newlines around it. It returns the value start or -1.
func valueSeparator(s string, k int, strong, quoted bool) int {
	n := len(s)
	for k < n && (s[k] == ' ' || s[k] == '\t') {
		k++
	}
	switch {
	case k < n && s[k] == '=' && k+1 < n && s[k+1] == '>': // =>
		k += 2
	case k < n && s[k] == ':' && k+1 < n && s[k+1] == '=' && strong: // :=
		k += 2
	case k < n && s[k] == '-' && k+1 < n && s[k+1] == '>' && strong: // ->
		k += 2
	case k < n && (s[k] == '=' || s[k] == ':') && (s[k] == '=' || strong || quoted):
		k++
	default:
		return -1
	}
	for k < n && isSpaceByte(s[k]) {
		k++
	}
	return k
}

func redactText(s string, truncated bool) string {
	n := len(s)
	var b strings.Builder
	b.Grow(n)
	i := 0
	scanStart, scanEnd, scanAt := 0, -1, -1 // cached userinfo token scan: [scanStart, scanEnd] with last "@" at scanAt
	for i < n {
		c := s[i]
		// 1. URL userinfo. Each whitespace/quote-delimited token is scanned ONCE: the scan result (token end, last '@') is
		// remembered, and a later "://" inside the same token reuses it (a token of n "://" costs O(n), not O(n^2)). A single
		// quote or a backslash is a legal password byte, so only a double quote and whitespace end the authority scan.
		if c == ':' && strings.HasPrefix(s[i:], "://") {
			j := i + 3
			if j > scanEnd || j < scanStart { // not inside an already-scanned token
				scanStart, scanEnd, scanAt = j, j, -1
				for scanEnd < n && !isSpaceByte(s[scanEnd]) && s[scanEnd] != '"' {
					if s[scanEnd] == '@' {
						scanAt = scanEnd
					}
					scanEnd++
				}
			}
			end := scanEnd
			b.WriteString("://")
			if scanAt >= j {
				b.WriteString(redactedMark + "@")
				i = scanAt + 1 // rest of the token (host, path, query) is scanned normally
			} else if truncated && end == n && end > j && !strings.Contains(s[j:end], "/") {
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
		var offs []int
		if strings.IndexByte(run, '%') >= 0 {
			var d string
			d, offs = pctDecode(run)
			l = strings.ToLower(d)
			// a percent-encoded separator inside the run: "token%3Dvalue", "password%3Avalue"
			if p := strings.IndexAny(l, "=:"); p > 0 {
				key := l[:p]
				if strongKey(key) || (l[p] == '=' && weakKeys[nameSeparators.Replace(key)]) {
					end := j
					if end < n && !isSpaceByte(s[end]) && !isQuote(s[end]) && s[end] != '\\' {
						end = scanValue(s, end)
					}
					b.WriteString(run[:offs[p+1]])
					b.WriteString(redactedMark)
					i = end
					continue
				}
			}
		}

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
				i = skipFold(s, eol(s, k))
				continue
			}
		}

		// 3. secret path segment inside prose: /share/<seg> (extra slashes between the key and the segment are skipped)
		if i > 0 && s[i-1] == '/' && j < n {
			segEnd := func(from int) int {
				e := from
				for e < n && !isSpaceByte(s[e]) && s[e] != '/' && s[e] != '?' && s[e] != '#' && s[e] != '"' && s[e] != '\'' {
					e++
				}
				segmentWork.Add(int64(e-from) + 1)
				return e
			}
			tokenEnd := func(from int) int {
				e := from
				for e < n && !isSpaceByte(s[e]) && s[e] != '?' && s[e] != '#' && s[e] != '"' && s[e] != '\'' {
					e++
				}
				segmentWork.Add(int64(e-from) + 1)
				return e
			}
			k := segEnd(i) // the whole key segment, path parameters included
			if base, exact, ok := secretKey(s[i:k]); ok {
				st := k + 1
				for st < n && k < n && s[k] == '/' && s[st] == '/' {
					st++
				}
				regular := false
				e := st
				if k < n && s[k] == '/' && st < n {
					e = segEnd(st)
					regular = exact && st == k+1 && e > st && cleanToken(s[st:e])
				}
				switch {
				case exact && regular:
					b.WriteString(run + "/:redacted")
					i = e
					continue
				case exact && (e <= st || k >= n || s[k] != '/'):
					// bare key with nothing after it: fall through to the normal scan
				default:
					// irregular: only the base name stays; the key's parameters and the rest of the path are the secret
					if base == "" {
						base = redactedMark
					}
					b.WriteString(base)
					if k < n && s[k] == '/' && st < n {
						b.WriteString("/:redacted")
						i = tokenEnd(st)
					} else {
						i = k
					}
					continue
				}
			}
		}

		// 3b. Go struct printing of a key/value pair: {Key:token Value:S} and the positional form {token S}
		if (l == "key" || l == "name" || l == "param") && j < n && s[j] == ':' {
			k := j + 1
			w := k
			for w < n && isWordByte(s[w]) {
				w++
			}
			if w > k && (strongKey(strings.ToLower(s[k:w])) || weakKeys[strings.ToLower(s[k:w])]) {
				v := w
				for v < n && (s[v] == ' ' || s[v] == '\t') {
					v++
				}
				vw := v
				for vw < n && isWordByte(s[vw]) {
					vw++
				}
				if lv := strings.ToLower(s[v:vw]); (lv == "value" || lv == "values" || lv == "val") && vw < n && s[vw] == ':' {
					vs := vw + 1
					if vend := scanValue(s, vs); vend > vs {
						b.WriteString(s[i:vs])
						b.WriteString(redactedMark)
						i = vend
						continue
					}
				}
			}
		}
		if i > 0 && s[i-1] == '{' && j < n && s[j] == ' ' && (strongKey(l) || weakKeys[l]) {
			e := j + 1
			for e < n && s[e] != '}' && s[e] != '\n' && s[e] != '\r' {
				e++
			}
			if e > j+1 {
				b.WriteString(run + " " + redactedMark)
				i = e
				continue
			}
		}

		// 4. key sep value
		strong := strongKey(l)
		weak := weakKeys[l] || weakKeys[nameSeparators.Replace(l)]
		if strong || weak {
			k := j
			quoted := false
			if q := closeQuoteLen(s, k); q > 0 {
				k += q
				quoted = true
			}
			vstart := valueSeparator(s, k, strong, quoted)
			if vstart < 0 && strong && !quoted && j < n && s[j] == '\t' {
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
