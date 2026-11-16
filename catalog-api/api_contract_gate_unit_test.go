package main

import (
	"os"
	"path/filepath"
	"regexp"
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
