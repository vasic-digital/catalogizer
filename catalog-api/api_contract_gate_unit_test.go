package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func spec403(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "docs", "api", "openapi.yaml"))
	require.NoError(t, err)
	var doc map[string]interface{}
	require.NoError(t, yaml.Unmarshal(raw, &doc))
	op := doc["paths"].(map[string]interface{})[path].(map[string]interface{})["get"].(map[string]interface{})
	return op["responses"].(map[string]interface{})["403"].(map[string]interface{})["description"].(string)
}

// ADMD1RRRRRRRRR-m1: the merged 403 of the two sync GET {id} operations must classify as an admin gate (the non-admin leg then
// asserts 403), and the classifier must keep rejecting genuinely non-gate descriptions.
func TestADMD1RRRRRRRRR_SyncMerged403IsClassifiedAdminGated(t *testing.T) {
	for _, p := range []string{"/api/v1/sync/endpoints/{id}", "/api/v1/sync/sessions/{id}"} {
		require.True(t, admin403Gated(spec403(t, p)), "%s: %q must classify as an admin gate", p, spec403(t, p))
	}
	// negative controls: non-gate wording stays non-gate; a bare owner-only wording is not forced to gated
	require.False(t, admin403Gated("Forbidden; the caller is not the owner of the session"))
	require.False(t, admin403Gated("Forbidden: registration is disabled"))
	require.True(t, admin403Gated("Forbidden (admin role required)"))
	require.False(t, admin403Gated("Forbidden: pending approval"), "no admin token at all")
}

// ADMD1RRRRRRRRRR-n5: quoting the admin body in a NEGATIVE sentence is not documenting an admin gate; only a `body {...}` quotation
// that is not negated within its clause classifies. The real spec descriptions must keep classifying.
func TestADMD1RRRRRRRRRR_AdminBodyQuoteInNegativeSentenceIsNotGated(t *testing.T) {
	const q = `{"success":false,"error":"Admin role required"}`
	require.False(t, admin403Gated(`Forbidden; this route never returns body `+q+` (owner check only)`), "negated quotation")
	require.False(t, admin403Gated(`Forbidden; callers are not refused with body `+q+` here; owner session only`), "negated quotation (not)")
	require.False(t, admin403Gated(`Forbidden; unlike other routes `+q+` is quoted only for contrast with the owner session`), "quotation without 'body'")
	// positive controls: the same quotation in an affirmative clause, and the real descriptions, stay gated
	require.True(t, admin403Gated(`Forbidden; not the owner of the session or (body `+q+`) the caller is not an administrator`), "negation in an earlier clause does not poison a later affirmative one")
	require.True(t, admin403Gated(`Forbidden; the caller is not an administrator (body `+q+`) or is not the session owner`))
	for _, p := range []string{"/api/v1/sync/endpoints/{id}", "/api/v1/sync/sessions/{id}"} {
		require.True(t, admin403Gated(spec403(t, p)), p)
	}
}

// legacyAdmin403Gated is a verbatim copy of the classifier as merged before ADMD1RRRRRRRRRRR-F1..F3, kept only to prove the census of the
// real spec did not move.
var (
	legacyClauseBoundary   = regexp.MustCompile(`(?i)[;,.()]|\b(or|but|and)\b`)
	legacyNegationInClause = regexp.MustCompile(`(?i)\b(never|not|no|neither|nor|without)\b`)
)

func legacyAdmin403Gated(desc string) bool {
	doc := false
	for _, loc := range adminBodyQuote.FindAllStringIndex(desc, -1) {
		before := desc[:loc[0]]
		if cuts := legacyClauseBoundary.FindAllStringIndex(before, -1); len(cuts) > 0 {
			before = before[cuts[len(cuts)-1][1]:]
		}
		if !legacyNegationInClause.MatchString(before) {
			doc = true
		}
	}
	if doc {
		return true
	}
	return adminGate403.MatchString(desc) && !nonGate403.MatchString(desc)
}

// ADMD1RRRRRRRRRRR-F1: every described 403 of the real spec classifies exactly as under the pre-change classifier.
func TestADMD1RRRRRRRRRRR_CensusOfRealSpec403sUnchanged(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "docs", "api", "openapi.yaml"))
	require.NoError(t, err)
	var doc map[string]interface{}
	require.NoError(t, yaml.Unmarshal(raw, &doc))
	n, gated := 0, 0
	for path, ops := range doc["paths"].(map[string]interface{}) {
		for method, op := range ops.(map[string]interface{}) {
			opm, ok := op.(map[string]interface{})
			if !ok {
				continue
			}
			resp, ok := opm["responses"].(map[string]interface{})
			if !ok {
				continue
			}
			r403, ok := resp["403"].(map[string]interface{})
			if !ok {
				continue
			}
			desc, _ := r403["description"].(string)
			if desc == "" {
				continue
			}
			n++
			want := legacyAdmin403Gated(desc)
			if want {
				gated++
			}
			require.Equal(t, want, admin403Gated(desc), "%s %s: %q", method, path, desc)
		}
	}
	require.Equal(t, 93, n, "census size of described 403s")
	require.Greater(t, gated, 0)
}

// ADMD1RRRRRRRRRRR-F1..F3: affirmative descriptions with a negation that does not govern the quotation stay gated; negations that do
// (before the verb, contractions, after the quotation) do not.
func TestADMD1RRRRRRRRRRR_Admin403ClassifierNegationScope(t *testing.T) {
	const q = `{"success":false,"error":"Admin role required"}`
	// positive controls
	require.True(t, admin403Gated(`Forbidden; a caller who is not an administrator gets body `+q+` or the non-owner gets the session error`), "(a)")
	require.True(t, admin403Gated(`Forbidden; not-the-owner callers get body `+q), "(b) hyphenated not")
	require.True(t, admin403Gated(`Forbidden; callers without the system.admin permission get body `+q), "dot in identifier, 'without' not governing the verb")
	require.True(t, admin403Gated(`Forbidden. The non-admin caller gets body `+q), "dot ends a clause")
	// negative cases
	require.False(t, admin403Gated(`Forbidden; this route doesn't return body `+q), "doesn't")
	require.False(t, admin403Gated(`Forbidden; this route isn't refused with body `+q+` for owners`), "isn't")
	require.False(t, admin403Gated(`Forbidden; callers cannot get body `+q), "cannot")
	require.False(t, admin403Gated(`Forbidden; body `+q+` is never sent for the session owner`), "negation after the quotation")
	require.False(t, admin403Gated(`Forbidden; body `+q+` isn't sent here (owner session only)`), "contraction after the quotation")
	require.False(t, admin403Gated(`Forbidden; this route does not return body `+q+` and owner session only`), "plain not before verb")
}

// ADMD1RRRRRRRRRRR round 2: a negation right after the quotation negates it only when a quote verb follows ("is never sent"); other
// continuations ("not the generic error", "cannot be retried") are affirmative statements about the same clause.
func TestADMD1RRRRRRRRRRR_Admin403AfterQuoteNegationNeedsQuoteVerb(t *testing.T) {
	const q = `{"success":false,"error":"Admin role required"}`
	for _, tail := range []string{
		`callers get body ` + q + ` not the generic error`,
		`callers get body ` + q + ` not the owner error`,
		`callers get body ` + q + ` is not cached`,
		`the session holder gets body ` + q + ` never the owner error`,
		`a non-owner gets body ` + q + ` cannot be retried`,
		`callers get body ` + q + ` no longer the session error`,
		`callers get body ` + q + ` don't retry`,
		`callers get body ` + q + ` won't be cached`,
		`callers get body ` + q + ` not only for the owner`,
	} {
		require.True(t, admin403Gated("Forbidden; "+tail), tail)
	}
	// still negated: a quote verb follows the negation
	require.False(t, admin403Gated(`Forbidden; body `+q+` is never sent for the session owner`))
	require.False(t, admin403Gated(`Forbidden; body `+q+` isn't returned here`))
	require.False(t, admin403Gated(`Forbidden; body `+q+` is not to be sent to owners`))
	require.False(t, admin403Gated(`Forbidden; body `+q+` is no longer returned`))
	// curly apostrophe is normalised
	require.False(t, admin403Gated("Forbidden; this route doesn’t return body "+q))
	require.False(t, admin403Gated("Forbidden; body "+q+" isn’t sent here"))
}

// Known limit (documented, not fixed): a negation separated from its verb by a comma-delimited interjection is read as affirmative,
// i.e. the conservative direction for a gate (the non-admin 403 keeps being asserted).
func TestADMD1RRRRRRRRRRR_Admin403CommaInterjectionKnownLimit(t *testing.T) {
	const q = `{"success":false,"error":"Admin role required"}`
	require.True(t, admin403Gated(`Forbidden; this route does not, for owners, return body `+q))
}

// ADMD1RRRRRRRRRRR round 3 (structural): the negation-scope class enumerated as a GENERATED matrix. The expectation of every cell
// comes from how the cell was BUILT (an independent plain-language oracle), never from the classifier's regexes:
//
//	RULE  a quotation `body {...}` is negated only when a negation directly governs the verb that introduces it:
//	      (before-adjacent) the negation sits immediately before that verb, any auxiliary/adverb allowed in between;
//	      (after-adjacent)  the negation follows the quotation immediately AND is a predicate negation: an auxiliary
//	                        (is/are/was/were/does/did/will) + never/not/no longer + verb, or a contraction (isn't, doesn't),
//	                        or "cannot", + verb. A bare "never sent" with no auxiliary is ambiguous and is read AFFIRMATIVE.
//	      "without" negates only before-adjacent ("without returning body {...}").
//	      A negation anywhere else (non-adjacent before/after, another clause) never negates the quotation.
//	HARD RULE a false negative (oracle says gated, classifier says not gated) can NEVER be allow-listed. Only the conservative
//	      direction (oracle says negated, classifier says gated) may appear in matrixAllowList, each with a reason.
type matrixVerb struct{ base, third, ing, part string }

var matrixVerbs = []matrixVerb{
	{"return", "returns", "returning", "returned"},
	{"send", "sends", "sending", "sent"},
	{"get", "gets", "getting", "got"},
	{"refuse", "refuses", "refusing", "refused"},
	{"show", "shows", "showing", "shown"},
}

type matrixNeg struct {
	kind string // word | contraction | cannot | without
	text string // as written before a base verb
}

var matrixNegs = []matrixNeg{
	{"word", "never"}, {"word", "not"}, {"word", "no longer"},
	{"contraction", "doesn't"}, {"contraction", "don't"}, {"contraction", "won't"}, {"contraction", "isn't"}, {"contraction", "wasn't"},
	{"cannot", "cannot"}, {"without", "without"},
}

var matrixAllowList = map[string]string{}

func TestADMD1RRRRRRRRRRR_Admin403NegationScopeMatrix(t *testing.T) {
	const q = `{"success":false,"error":"Admin role required"}`
	type cell struct {
		name, desc string
		negated    bool
	}
	var cells []cell
	add := func(name, desc string, negated bool) {
		cells = append(cells, cell{name, "Forbidden; " + desc, negated})
	}
	for _, n := range matrixNegs {
		for _, v := range matrixVerbs {
			id := n.text + "/" + v.base
			// before-adjacent: negation directly before the introducing verb
			switch n.kind {
			case "without":
				add("before-adj/"+id, "callers are refused without "+v.ing+" body "+q, true)
			case "word":
				add("before-adj/"+id+"/aux", "this route does "+strings.Replace(n.text, "no longer", "not", 1)+" "+v.base+" body "+q, true)
				add("before-adj/"+id+"/noaux", "this route "+n.text+" "+v.third+" body "+q, true)
			default:
				add("before-adj/"+id, "this route "+n.text+" "+v.base+" body "+q, true)
			}
			// before-non-adjacent: the negation governs a different verb; the introducing verb is the affirmative "gets"
			add("before-nonadj/"+id, "a caller who "+n.text+" "+v.base+" the role gets body "+q+" for the session", false)
			// after-adjacent
			switch n.kind {
			case "word":
				for _, aux := range []string{"is", "was", "does", "will"} {
					form := v.part
					if aux == "does" || aux == "will" {
						form = v.base
					}
					add("after-adj/"+id+"/aux-"+aux, "callers get body "+q+" "+aux+" "+n.text+" "+form, true)
				}
				add("after-adj/"+id+"/noaux", "callers get body "+q+" "+n.text+" "+v.part+" to anyone else", false)
			case "contraction", "cannot":
				add("after-adj/"+id, "callers get body "+q+" "+n.text+" "+v.base, n.kind == "cannot" || n.text == "doesn't" || n.text == "don't" || n.text == "won't" || n.text == "isn't" || n.text == "wasn't")
			case "without":
				add("after-adj/"+id, "callers get body "+q+" without "+v.ing+" more", false)
			}
			// after-non-adjacent and other-clause
			add("after-nonadj/"+id, "callers get body "+q+" for everyone "+n.text+" "+v.part, false)
			add("after-otherclause/"+id, "callers get body "+q+" and owners "+n.text+" "+v.base+" it", false)
			add("before-otherclause/"+id, "owners "+n.text+" "+v.base+" it; callers get body "+q, false)
		}
	}
	require.Greater(t, len(cells), 300)
	for _, c := range cells {
		got := admin403Gated(c.desc)
		wantGated := !c.negated
		if got == wantGated {
			continue
		}
		reason, allowed := matrixAllowList[c.name]
		if wantGated && !got {
			t.Errorf("FALSE NEGATIVE (never allow-listable) %s: %q", c.name, c.desc)
			continue
		}
		if !allowed {
			t.Errorf("mismatch %s: classifier gated=%v oracle gated=%v: %q", c.name, got, wantGated, c.desc)
		} else {
			require.NotEmpty(t, reason)
		}
	}
}

// ADMD1RRRRRRRRRRR round 3: the reviewer's concrete descriptions, pinned verbatim next to the generated matrix.
func TestADMD1RRRRRRRRRRR_Admin403ReviewerPhrasings(t *testing.T) {
	const q = `{"success":false,"error":"Admin role required"}`
	for _, tail := range []string{
		"callers get body " + q + " not shown to owners",
		"callers get body " + q + " never sent to the session owner",
		"callers get body " + q + " not included in the audit log",
		"callers get body " + q + " not returned by v1 servers",
		"callers get body " + q + " no longer returned as 401",
		"callers get body " + q + " not documented elsewhere",
		"a caller who never received the role gets body " + q,
		"callers get body " + q + " and owners never receive it",
	} {
		require.True(t, admin403Gated("Forbidden; "+tail), tail)
	}
	require.False(t, admin403Gated("Forbidden; body "+q+" is never sent for the session owner"))
	require.False(t, admin403Gated("Forbidden; this route doesnʼt return body "+q))
}
