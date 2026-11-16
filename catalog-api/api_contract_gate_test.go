package main

// ADMD1RRRRRRRRR-m1: the classifier the live contract sweep uses to decide whether a documented 403 is an admin gate (so the
// non-admin leg must assert it). It lives outside the live build tag so a default `go test` exercises it against the real spec.

import "regexp"

var adminGate403 = regexp.MustCompile(`(?i)admin|lacks|system\.configure|user\.manage|report\.`)
var nonGate403 = regexp.MustCompile(`(?i)registration|pending|owner|not the caller|neither|visible but|share|job|domain not allowed|another user|session`)

// adminRoleBody is the exact body string of middleware.RequireAdmin's 403 (auth.go): a description that QUOTES it as `body {...}` in an
// affirmative clause documents an admin gate even when it also mentions an owner or a session (merged 403s of the /sync GET {id}
// operations). ADMD1RRRRRRRRRR-n5: a quotation inside a negative clause ("never returns body {...}") does not classify, and neither
// does the bare string without the `body ` marker.
const adminRoleBody = `{"success":false,"error":"Admin role required"}`

var (
	adminBodyQuote   = regexp.MustCompile(`body ` + regexp.QuoteMeta(adminRoleBody))
	clauseBoundary   = regexp.MustCompile(`(?i)[;,.()]|\b(or|but|and)\b`)
	negationInClause = regexp.MustCompile(`(?i)\b(never|not|no|neither|nor|without)\b`)
)

// adminBodyDocumented reports whether desc quotes the admin body in at least one clause that carries no negation before the quotation.
func adminBodyDocumented(desc string) bool {
	for _, loc := range adminBodyQuote.FindAllStringIndex(desc, -1) {
		before := desc[:loc[0]]
		if cuts := clauseBoundary.FindAllStringIndex(before, -1); len(cuts) > 0 {
			before = before[cuts[len(cuts)-1][1]:]
		}
		if !negationInClause.MatchString(before) {
			return true
		}
	}
	return false
}

func admin403Gated(desc string) bool {
	if adminBodyDocumented(desc) {
		return true
	}
	return adminGate403.MatchString(desc) && !nonGate403.MatchString(desc)
}
