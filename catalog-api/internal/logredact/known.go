package logredact

import (
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Known-secret substitution (PX-F17 round 3, F3). A free-text pattern cannot recognise a BARE secret ("lookup share S"), but the
// request that failed knows exactly which values are credentials: the path segment after a secret-introducing segment, route
// parameters with a sensitive name, sensitive query values and credential-header values. KnownSecrets collects them and
// ReplaceKnown removes every occurrence (also in %q / URL-escaped form) from any text that will be logged for that request.
const (
	minKnownSecretLen = 4
	maxKnownSecrets   = 64
	maxKnownSecretLen = 4096
)

// KnownSecrets returns the credential values carried by one request (at most maxKnownSecrets values; the scan is linear). params are the route parameters (name -> value).
func KnownSecrets(path, rawQuery string, header http.Header, params map[string]string) []string {
	var out []string
	add := func(v string) {
		if len(v) >= minKnownSecretLen && len(v) <= maxKnownSecretLen && len(out) < maxKnownSecrets {
			out = append(out, v)
		}
	}
	// path segments after a secret-introducing segment
	segs := strings.Split(path, "/")
	addSeg := func(sg string) {
		add(sg)
		if u, err := url.PathUnescape(sg); err == nil && u != sg {
			add(u)
		}
	}
scan:
	for i := 0; i < len(segs) && len(out) < maxKnownSecrets; i++ {
		segmentWork.Add(int64(len(segs[i])) + 1)
		_, exact, ok := secretKey(segs[i])
		if !ok {
			continue
		}
		j, regular := secretPosition(segs, i)
		if !exact {
			// the key's own parameters ("share;token=S", "share;S") are secret values
			dec, _ := decodeSegment(segs[i])
			if p := strings.IndexByte(dec, ';'); p >= 0 {
				for _, prm := range strings.Split(dec[p+1:], ";") {
					addSeg(prm)
					if _, v, found := strings.Cut(prm, "="); found {
						addSeg(v)
					}
				}
			}
		}
		if j >= len(segs) {
			continue
		}
		addSeg(segs[j])
		if !exact || !regular {
			// the secret position is everything after the key: collect it ONCE and stop scanning (linear cost)
			for _, sg := range segs[j:] {
				segmentWork.Add(int64(len(sg)) + 1)
				if sg == "" || len(out) >= maxKnownSecrets {
					continue
				}
				addSeg(sg)
				dec, _ := decodeSegment(sg)
				for _, prm := range strings.Split(dec, ";") {
					if prm != dec {
						addSeg(prm)
					}
					if _, v, found := strings.Cut(prm, "="); found {
						addSeg(v)
					}
				}
			}
			break scan
		}
	}
	for name, v := range params {
		if SensitiveKey(name) {
			add(v)
		}
	}
	for _, pair := range strings.FieldsFunc(rawQuery, func(r rune) bool { return r == '&' || r == ';' }) {
		name, val, ok := strings.Cut(pair, "=")
		if !ok {
			continue
		}
		if un, err := url.QueryUnescape(name); err != nil || SensitiveKey(un) {
			add(val)
			if u, err := url.QueryUnescape(val); err == nil && u != val {
				add(u)
			}
		}
	}
	for name, vals := range header {
		if !headerNames[strings.ToLower(name)] {
			continue
		}
		for _, v := range vals {
			add(v)
			for _, f := range strings.FieldsFunc(v, func(r rune) bool { return r == ' ' || r == ';' || r == ',' }) {
				if _, val, ok := strings.Cut(f, "="); ok {
					add(strings.Trim(val, `"'`))
				} else if !schemeWords[strings.ToLower(f)] {
					add(strings.Trim(f, `"'`))
				}
			}
		}
	}
	return out
}

// ReplaceKnown replaces every occurrence of every known secret in s (longest first), including its URL-escaped and %q-quoted (outer quotes stripped) forms.
func ReplaceKnown(s string, secrets []string) string {
	if len(secrets) == 0 || s == "" {
		return s
	}
	uniq := make(map[string]bool, len(secrets)*2)
	for _, sec := range secrets {
		uniq[sec] = true
		if e := url.QueryEscape(sec); e != sec {
			uniq[e] = true
		}
		if e := url.PathEscape(sec); e != sec {
			uniq[e] = true
		}
		// %q form (HYG-F6): fmt's %q / strconv.Quote escapes quotes, backslashes and control characters; the text between the
		// outer quotes is what an error text carries.
		for _, q := range []string{strconv.Quote(sec), strconv.QuoteToASCII(sec)} { // %q and %+q
			if len(q) > 2 && q[1:len(q)-1] != sec {
				uniq[q[1:len(q)-1]] = true
			}
		}
	}
	list := make([]string, 0, len(uniq))
	for k := range uniq {
		list = append(list, k)
	}
	sort.Slice(list, func(a, b int) bool {
		if len(list[a]) != len(list[b]) {
			return len(list[a]) > len(list[b])
		}
		return list[a] < list[b]
	})
	pairs := make([]string, 0, len(list)*2)
	for _, k := range list {
		pairs = append(pairs, k, redactedMark)
	}
	return strings.NewReplacer(pairs...).Replace(s)
}

// ErrorTextKnown is the ONE pipeline for error text of a request that knows its own credentials (PKGB-FU1/FU2): the input is cut
// BEFORE ReplaceKnown (its cost is O(n*L); on uncapped text 64 secrets x 4 KiB cost 0.87 s per 4 MiB), then known secrets are
// replaced, then ErrorText runs. The pre-cut keeps maxKnownSecretLen bytes beyond what Text keeps; the output is capped at ErrorTextMaxBytes
// (2 KiB), far before any pre-cut position, so a secret that straddles the cut cannot appear in the output either way (the margin
// is defensive; see TestErrorTextKnown_StraddlingSecretNeverExposed).
func ErrorTextKnown(s string, secrets []string) string {
	return ErrorText(replaceKnownFn(preCutForKnown(s), secrets))
}

// replaceKnownFn is ReplaceKnown; a variable only so tests can observe the length of its input (deterministic cost oracle).
var replaceKnownFn = ReplaceKnown

// preCutForKnown bounds the text handed to ReplaceKnown.
func preCutForKnown(s string) string {
	if limit := textPreCapBytes + maxKnownSecretLen; len(s) > limit {
		cut := limit
		for cut > 0 && !utf8.RuneStart(s[cut]) {
			cut--
		}
		s = s[:cut]
	}
	return s
}
