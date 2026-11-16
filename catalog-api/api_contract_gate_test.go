package main

// ADMD1RRRRRRRRR-m1: the classifier the live contract sweep uses to decide whether a documented 403 is an admin gate (so the
// non-admin leg must assert it). It lives outside the live build tag so a default `go test` exercises it against the real spec.

import (
	"regexp"
	"strings"
)

var adminGate403 = regexp.MustCompile(`(?i)admin|lacks|system\.configure|user\.manage|report\.`)
var nonGate403 = regexp.MustCompile(`(?i)registration|pending|owner|not the caller|neither|visible but|share|job|domain not allowed|another user|session`)

// adminRoleBody is the exact body string of middleware.RequireAdmin's 403 (auth.go): a description that QUOTES it as `body {...}` in an
// affirmative clause documents an admin gate even when it also mentions an owner or a session (merged 403s of the /sync GET {id}
// operations). ADMD1RRRRRRRRRR-n5: a quotation inside a negative clause ("never returns body {...}") does not classify, and neither
// does the bare string without the `body ` marker.
const adminRoleBody = `{"success":false,"error":"Admin role required"}`

// ADMD1RRRRRRRRRRR-F1..F3: a negation matters only when it DIRECTLY governs the verb that introduces the quotation ("never returns",
// "are not refused with", "doesn't send", "cannot return"), or follows the quotation as its own predicate ("body {...} is never sent").
// A bare negation elsewhere in the clause ("a caller who is not an administrator gets body {...}", "not-the-owner callers get body")
// describes the caller, not the quotation, and no longer poisons it. A dot splits clauses only before whitespace or at the end, so
// identifiers such as system.admin stay whole.
var (
	adminBodyQuote = regexp.MustCompile(`body ` + regexp.QuoteMeta(adminRoleBody))
	clauseBoundary = regexp.MustCompile(`(?i)[;,()]|\.(?:\s|$)|\b(or|but|and)\b`)
	quoteVerbs     = `(?:return(?:s|ed|ing)?|send(?:s|ing)?|sent|get(?:s|ting)?|got|refus(?:e|es|ed|ing)|reject(?:s|ed|ing)?|serv(?:e|es|ed|ing)|emit(?:s|ted|ting)?|produc(?:e|es|ed|ing)|respond(?:s|ed|ing)?|answer(?:s|ed|ing)?|yield(?:s|ed|ing)?|show(?:s|n|ed|ing)?|includ(?:e|es|ed|ing)|carr(?:y|ies|ied|ying)|quot(?:e|es|ed|ing)|document(?:s|ed|ing)?|receiv(?:e|es|ed|ing))`
	// negation (word, contraction or "cannot") immediately before, optionally through a copula/adverb, a quote-introducing verb
	negatedVerbBefore = regexp.MustCompile(`(?i)(?:\b(?:never|not|neither|nor|without)|n't|\bcannot)\s+(?:(?:be|been|being|longer|ever|also|normally|ordinarily|to)\s+)*` + quoteVerbs + `(?:\s+with)?\s*$`)
	// predicate negation right after the quotation: `body {...} is never sent`, `... isn't returned`
	negatedAfter = regexp.MustCompile(`(?i)^\s*(?:(?:is|are|was|were|will|would|does|do|did|can|could|has|have)\s+)?(?:never|not|no\s+longer)\b|^\s*(?:\w+)?n't\b|^\s*cannot\b`)
)

// adminBodyDocumented reports whether desc quotes the admin body in at least one clause where the quotation is not negated.
func adminBodyDocumented(desc string) bool {
	for _, loc := range adminBodyQuote.FindAllStringIndex(desc, -1) {
		before := desc[:loc[0]]
		if cuts := clauseBoundary.FindAllStringIndex(before, -1); len(cuts) > 0 {
			before = before[cuts[len(cuts)-1][1]:]
		}
		after := desc[loc[1]:]
		if cut := clauseBoundary.FindStringIndex(after); cut != nil {
			after = after[:cut[0]]
		}
		if negatedVerbBefore.MatchString(before) || negatedAfter.MatchString(after) {
			continue
		}
		return true
	}
	return false
}

func admin403Gated(desc string) bool {
	if adminBodyDocumented(desc) {
		return true
	}
	// the quotation's own "Admin role required" must not feed the keyword fallback once its clause was judged negated
	rest := strings.ReplaceAll(desc, adminRoleBody, "")
	return adminGate403.MatchString(rest) && !nonGate403.MatchString(rest)
}
