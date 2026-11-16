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
	quoteVerbs     = `(?:return(?:s|ed|ing)?|send(?:s|ing)?|sent|get(?:s|ting)?|got|refus(?:e|es|ed|ing)|reject(?:s|ed|ing)?|serv(?:e|es|ed|ing)|emit(?:s|ted|ting)?|produc(?:e|es|ed|ing)|respond(?:s|ed|ing)?|answer(?:s|ed|ing)?|yield(?:s|ed|ing)?|show(?:s|n|ed|ing)?|includ(?:e|es|ed|ing)|carr(?:y|ies|ied|ying)|quot(?:e|es|ed|ing)|document(?:s|ed|ing)?|receiv(?:e|es|ed|ing))`
	// negation (word, contraction or "cannot") immediately before, optionally through a copula/adverb, a quote-introducing verb
	negatedVerbBefore = regexp.MustCompile(`(?i)(?:\b(?:never|not|no\s+longer|neither|nor|without)|n't|\bcannot)\s+(?:(?:be|been|being|longer|ever|also|normally|ordinarily|to)\s+)*` + quoteVerbs + `(?:\s+with)?\s*$`)
	// predicate negation right after the quotation, only when a quote verb follows it: `body {...} is never sent`, `... isn't returned`.
	// "not the generic error", "cannot be retried", "don't retry" are affirmative continuations and do not negate the quotation.
	negatedAfter = regexp.MustCompile(`(?i)^\s*(?:(?:is|are|was|were|be|been|being|do|does|did|will|would|has|have|had)\s+(?:never|not|no\s+longer)|\w+n't|cannot)\s+(?:(?:be|been|being|longer|ever|also|normally|ordinarily|to)\s+)*` + quoteVerbs + `\b`)
)

// adminBodyDocumented reports whether desc quotes the admin body in at least one clause where the quotation is not negated.
// Scope rule (pinned by the generated matrix in api_contract_gate_unit_test.go): only a negation that directly governs the verb
// introducing the quotation, or a predicate negation directly after it (auxiliary + never/not/no longer, a contraction, or
// "cannot", then a quote verb), negates it. A bare "never sent" after the quotation (no auxiliary) is ambiguous and is read
// AFFIRMATIVE, as is a negation separated from its verb by a comma interjection ("does not, for owners, return body {...}");
// both are the conservative direction (the non-admin 403 keeps being asserted). No clause splitting is needed: negatedVerbBefore is
// anchored at the end of the text before the quotation and negatedAfter at its start, so a negation in any other clause or position
// cannot match (the former dot/clause-boundary computation was behaviour-equivalent and was removed, history: git log -S clauseBoundary).
func adminBodyDocumented(desc string) bool {
	desc = strings.NewReplacer("\u2019", "'", "\u2018", "'", "\u02bc", "'").Replace(desc)
	for _, loc := range adminBodyQuote.FindAllStringIndex(desc, -1) {
		if negatedVerbBefore.MatchString(desc[:loc[0]]) || negatedAfter.MatchString(desc[loc[1]:]) {
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
