// Package logredact removes credential-bearing query parameters from request targets before they reach an access log.
package logredact

import (
	"net/url"
	"strings"
)

// sensitive lists the query parameter names whose values never reach an access log. credential-shaped names are covered
// too: the retired GET /smb/discover and /smb/test forms (now 410) used to take username and password in the query
// string, and a stale client may still send them.
var sensitive = map[string]bool{
	"ticket": true, "access_token": true, "token": true, "api_key": true, "apikey": true,
	"password": true, "passwd": true, "pass": true, "pwd": true,
	"secret": true, "client_secret": true, "refresh_token": true, "authorization": true,
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
	if sensitive[strings.ToLower(un)] && hasVal {
		return name + "=REDACTED"
	}
	return p
}

// Path returns path + "?" + redacted query (or just path when there is no query).
func Path(path, rawQuery string) string {
	if q := Query(rawQuery); q != "" {
		return path + "?" + q
	}
	return path
}
