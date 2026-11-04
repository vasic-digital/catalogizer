// Package logredact removes credential-bearing query parameters from request targets before they reach an access log.
package logredact

import (
	"net/url"
	"strings"
)

var sensitive = map[string]bool{"ticket": true, "access_token": true, "token": true, "api_key": true}

// Query returns rawQuery with the value of every sensitive parameter replaced by "REDACTED"; everything else (order, escaping)
// is preserved verbatim. Parameter names are compared after URL-unescaping, so "%74icket" does not slip through. A pair whose name
// cannot be unescaped is redacted entirely.
func Query(rawQuery string) string {
	if rawQuery == "" {
		return ""
	}
	pairs := strings.Split(rawQuery, "&")
	for i, p := range pairs {
		name, _, hasVal := strings.Cut(p, "=")
		un, err := url.QueryUnescape(name)
		if err != nil {
			pairs[i] = "REDACTED"
			continue
		}
		if sensitive[strings.ToLower(un)] && hasVal {
			pairs[i] = name + "=REDACTED"
		}
	}
	return strings.Join(pairs, "&")
}

// Path returns path + "?" + redacted query (or just path when there is no query).
func Path(path, rawQuery string) string {
	if q := Query(rawQuery); q != "" {
		return path + "?" + q
	}
	return path
}
