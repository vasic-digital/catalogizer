// Package logredact removes credential-bearing query parameters from request targets before they reach an access log.
package logredact

import (
	"net/url"
	"strings"
	"sync/atomic"
)

// Sensitive-name policy: ONE list for the access-log query filter (Query) and the free-text scanner (Text), so a name added
// here is covered everywhere. Names are compared lower-cased.
//
//   - strongKeys are credential names matched exactly;
//   - keyWords are substrings: any name CONTAINING one is credential-bearing (access_token, db_password, x-amz-signature ...);
//   - weakKeys are short, ambiguous names matched exactly ("code", "key", "pass" ...). In free text they count only with an
//     explicit '=' or as a quoted JSON key, so prose such as "exit code: 1" stays readable; Query treats them as sensitive.
var strongKeys = map[string]bool{
	"ticket": true, "access_token": true, "token": true, "api_key": true, "apikey": true,
	"password": true, "passwd": true, "pwd": true, "secret": true, "client_secret": true, "refresh_token": true,
	"authorization": true, "auth": true, "bearer": true, "jwt": true, "otp": true, "verifier": true, "code_verifier": true,
	"signature": true, "credential": true, "credentials": true, "cookie": true, "sessionid": true, "session": true,
}

var weakKeys = map[string]bool{"pass": true, "pw": true, "code": true, "key": true, "sig": true, "refresh": true}

var keyWords = []string{
	"password", "passwd", "pwd", "secret", "token", "apikey", "api_key", "api-key", "credential", "cookie",
	"private_key", "private-key", "privatekey", "signature", "session", "authorization", "verifier", "passphrase",
}

var nameSeparators = strings.NewReplacer("-", "", "_", "", ".", "")

// strongKey also compares the name with '-', '_' and '.' removed, so "pass-word", "pass_word" and "pass.word" are "password".
func strongKey(l string) bool {
	if strongKeyExact(l) {
		return true
	}
	if norm := nameSeparators.Replace(l); norm != l {
		return strongKeyExact(norm)
	}
	return false
}

func strongKeyExact(l string) bool {
	if strongKeys[l] {
		return true
	}
	for _, w := range keyWords {
		if strings.Contains(l, w) {
			return true
		}
	}
	return false
}

// SensitiveKey reports whether a query-parameter / field / header name carries a credential (case-insensitive).
func SensitiveKey(name string) bool {
	l := strings.ToLower(name)
	return strongKey(l) || weakKeys[l] || weakKeys[nameSeparators.Replace(l)]
}

// Query returns rawQuery with the value of every sensitive parameter replaced by "REDACTED"; everything else (order, escaping)
// is preserved verbatim. Parameter names are compared after URL-unescaping, so "%74icket" does not slip through. A pair whose name
// cannot be unescaped is redacted entirely.
func Query(rawQuery string) string {
	if rawQuery == "" {
		return ""
	}
	// Both '&' and ';' separate pairs for redaction purposes (some servers and proxies still split
	// on ';'), so "a=1;password=x" cannot hide a value from the log filter. Separators are kept.
	var out strings.Builder
	start := 0
	for i := 0; i <= len(rawQuery); i++ {
		if i < len(rawQuery) && rawQuery[i] != '&' && rawQuery[i] != ';' {
			continue
		}
		out.WriteString(redactPair(rawQuery[start:i]))
		if i < len(rawQuery) {
			out.WriteByte(rawQuery[i])
		}
		start = i + 1
	}
	return out.String()
}

func redactPair(p string) string {
	name, _, hasVal := strings.Cut(p, "=")
	un, err := url.QueryUnescape(name)
	if err != nil {
		return "REDACTED"
	}
	if SensitiveKey(un) && hasVal {
		return name + "=REDACTED"
	}
	return p
}

// secretSegmentKeys lists the path segments whose NEXT segment is a bearer secret (a share token, invite code, reset token ...).
// Route registrations carrying such a parameter (grep for ":token"): GET /api/v1/logs/share/:token. The list is a class rule
// so a later route of the same shape is covered without touching the loggers.
var secretSegmentKeys = map[string]bool{
	"share": true, "shares": true, "token": true, "tokens": true, "invite": true, "invites": true, "invitation": true,
	"reset": true, "ticket": true, "tickets": true, "secret": true, "secrets": true, "key": true, "keys": true,
	"apikey": true, "api-key": true, "api_key": true, "verify": true, "confirm": true, "activate": true, "magic": true,
}

// Secret keys and the secret position (HYG-R3-F1, review round 3: ONE structural rule replaces the earlier layered
// classification). A segment is a secret KEY when its base name is in secretSegmentKeys; the base name is the segment
// percent-decoded (at most maxSegmentDecodes rounds), stripped of control bytes, cut at the first ";" (path parameters),
// trimmed of trailing dots and lower-cased. A segment still decodable after the last round is ambiguous and counts as a key.
//
// A key is REGULAR when the raw segment is exactly its base name (any case) AND the next non-empty segment is a CLEAN token
// (cleanToken) directly after it: then only that one segment is the secret. In EVERY other shape (a key with parameters,
// encodings or control bytes, a dot / param / look-alike / non-token segment after it, empty segments in between) the secret
// position is everything after the key's base name: the key's parameters and the whole rest of the path are redacted (share
// tokens are base64url with a prefix, no legitimate content follows). Over-redaction is the deliberate policy: compared with
// the pre-fix behaviour an irregular segment after a key now also removes the REST of the path (for example
// "/api/v1/keys/42;v=2/rotate" logs "/api/v1/keys/:redacted"), HYG-R3-F1 round 4 (F4-4). KnownSecrets collects at most
// maxKnownSecrets values per request (F4-6), which also bounds its work.
const maxSegmentDecodes = 3

// segmentWork counts the bytes examined by the segment helpers: a deterministic cost oracle for tests (work counted, not time).
var segmentWork atomic.Int64

// decodeSegment percent-decodes seg at most maxSegmentDecodes rounds; more reports a segment that still decodes afterwards.
func decodeSegment(seg string) (dec string, more bool) {
	for i := 0; i < maxSegmentDecodes; i++ {
		segmentWork.Add(int64(len(seg)) + 1)
		u, err := url.PathUnescape(seg)
		if err != nil || u == seg {
			return seg, false
		}
		seg = u
	}
	segmentWork.Add(int64(len(seg)) + 1)
	u, err := url.PathUnescape(seg)
	return seg, err == nil && u != seg
}

// secretKey reports whether seg is a secret key; base is its base name ("" for an ambiguous segment) and exact whether the
// raw segment is exactly that base name.
func secretKey(seg string) (base string, exact, ok bool) {
	dec, more := decodeSegment(seg)
	if more {
		return "", false, true
	}
	if i := strings.IndexByte(dec, ';'); i >= 0 {
		dec = dec[:i]
	}
	if strings.IndexFunc(dec, func(r rune) bool { return r < 0x20 || r == 0x7f }) >= 0 {
		dec = strings.Map(func(r rune) rune {
			if r < 0x20 || r == 0x7f {
				return -1
			}
			return r
		}, dec)
	}
	base = strings.TrimRight(strings.ToLower(dec), ".")
	if !secretSegmentKeys[base] {
		return "", false, false
	}
	return base, strings.EqualFold(seg, base), true
}

// cleanToken reports whether seg looks like a bare token: non-empty, only [A-Za-z0-9_.~=-], and at least one ASCII
// alphanumeric (HYG-R3-F1 round 4: a punctuation-only segment such as "~", "-", "=", "_" or dots is not a token, so the key
// stays irregular and the rest of the path is the secret). No minimum length is imposed: a short alphanumeric segment
// ("/keys/42/rotate") keeps its pre-fix behaviour; a real share token is prefix+base64url and never punctuation-only.
func cleanToken(seg string) bool {
	segmentWork.Add(int64(len(seg)) + 1)
	alnum := false
	for i := 0; i < len(seg); i++ {
		c := seg[i]
		switch {
		case c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9':
			alnum = true
		case c == '.' || c == '_' || c == '-' || c == '~' || c == '=':
		default:
			return false
		}
	}
	return alnum
}

// secretPosition returns the index of the first non-empty segment after key i (len(segs) when none) and whether the key is in
// the regular shape (the segment directly after it, a clean token).
func secretPosition(segs []string, i int) (j int, regular bool) {
	j = i + 1
	for j < len(segs) && segs[j] == "" {
		j++
	}
	return j, j == i+1 && j < len(segs) && cleanToken(segs[j])
}

// SecretPath returns path with every segment that follows a secret-introducing segment (see secretSegmentKeys) replaced by
// ":redacted". Everything else is preserved. Use it for ANY log field that carries a raw request path.
func SecretPath(path string) string {
	if path == "" {
		return path
	}
	segs := strings.Split(path, "/")
	for i := 0; i < len(segs); i++ {
		base, exact, ok := secretKey(segs[i])
		if !ok {
			continue
		}
		j, regular := secretPosition(segs, i)
		if !exact {
			// the key's own parameters are part of the secret position: only its base name is kept
			if base == "" {
				segs[i] = ":redacted"
			} else {
				segs[i] = base
			}
		}
		if j >= len(segs) {
			continue // nothing after the key ("/share", "/share//")
		}
		segs[j] = ":redacted"
		if !exact || !regular {
			segs = segs[:j+1] // everything after the key goes with it
			break
		}
	}
	return strings.Join(segs, "/")
}

// Path returns path (secret segments redacted) + "?" + redacted query (or just path when there is no query).
func Path(path, rawQuery string) string {
	path = SecretPath(path)
	if q := Query(rawQuery); q != "" {
		return path + "?" + q
	}
	return path
}

// PathKnown is Path with the request's known credentials (KnownSecrets, route parameters included) removed from ANY segment,
// not only from the one after a secret-introducing segment (HANDLERS-RED-1-F3).
func PathKnown(path, rawQuery string, secrets []string) string {
	return ReplaceKnown(Path(path, rawQuery), secrets)
}
