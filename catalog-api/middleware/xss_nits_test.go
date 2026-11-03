package middleware

import "testing"

func TestDetectXSS_BehaviorUrlAndOpenTagSpacing(t *testing.T) {
	for _, in := range []string{
		`<div style="behavior:url(x.htc)">`, `behavior : url(x.htc)`, `BEHAVIOR:URL(#default#userData)`,
		`<script src=x>`, `</script>`, `<svg onload=1>`, `<style>@import 'x'</style>`,
	} {
		if !DetectXSS(in) {
			t.Errorf("must detect: %q", in)
		}
	}
	// A browser does not parse "< tag" (space after "<") as a tag, so this is prose.
	for _, in := range []string{
		"x < link y", "price < style points", "a < svg b", "1 < form 2",
		"the behavior: url shortener design", "behaviour of users", "behavior is key",
	} {
		if DetectXSS(in) {
			t.Errorf("false positive: %q", in)
		}
	}
}
