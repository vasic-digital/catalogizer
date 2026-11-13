// Package logredact removes credential-bearing query parameters from request targets before they reach an access log.
package logredact

import (
	"net/url"
	"strings"
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

var weakKeys = map[string]bool{"pass": true, "pw": true, "code": true, "key": true, "sig": true}

var keyWords = []string{
	"password", "passwd", "pwd", "secret", "token", "apikey", "api_key", "api-key", "credential", "cookie",
	"private_key", "private-key", "privatekey", "signature", "session", "authorization", "verifier",
}

func strongKey(l string) bool {
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
	return strongKey(l) || weakKeys[l]
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

// SecretPath returns path with every segment that follows a secret-introducing segment (see secretSegmentKeys) replaced by
// ":redacted". Everything else is preserved. Use it for ANY log field that carries a raw request path.
func SecretPath(path string) string {
	if path == "" {
		return path
	}
	segs := strings.Split(path, "/")
	for i := 0; i+1 < len(segs); i++ {
		if secretSegmentKeys[strings.ToLower(segs[i])] && segs[i+1] != "" {
			segs[i+1] = ":redacted"
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
