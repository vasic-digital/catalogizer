package logredact

import (
	"strings"
	"testing"
)

// Round-3 review F3: secret surfaces beyond single key=value pairs. Each case names the secret that must vanish.
// Class inventory (§11.4.276(C)) - every member is closed here or scoped out in the package doc:
//
//	multi-value (arrays) ........ closed        Go struct printing ({Key:.. Value:..}, {name value}) ... closed
//	folded header continuation .. closed        double-escaped JSON ........................ closed
//	userinfo with ' or \ ........ closed        double slash after a secret segment ......... closed
//	names (pass-word, passphrase, refresh) ... closed       schemes (jwt, Token) ............ closed
//	arrows (->, =>, :=) ......... closed        percent-encoded names / separators .......... closed
//	SCOPED OUT: a bare secret with no key, header, userinfo, scheme or known-request-value shape (prose); a key separated from
//	its value by words ("password is S"); arrays spanning more than 4 KiB.
func TestText_F3LeakTable(t *testing.T) {
	const S = "SSECRETVAL1"
	const S2 = "SSECRETVAL2"
	leaks := []struct{ name, in string }{
		{"json-array", `{"tokens":["` + S + `","` + S2 + `"]}`},
		{"go-map-array", `map[token:[a ` + S + `]]`},
		{"json-password-array", `{"password": ["` + S + `"]}`},
		{"json-array-multiline", "{\"tokens\": [\n  \"" + S + "\",\n  \"" + S2 + "\"\n]}"},
		{"array-with-bracket-in-quote", `{"token":["a]b` + S + `"]}`},
		{"struct-key-value", `{Key:token Value:` + S + `}`},
		{"struct-key-value-array", `{Key:token Value:[` + S + ` ` + S2 + `]}`},
		{"struct-pair-list", `[{token ` + S + `}]`},
		{"struct-pair-pointer", `&{token ` + S + `}`},
		{"folded-bearer", "Authorization: Bearer\n " + S},
		{"folded-cookie", "Cookie:\n " + S},
		{"folded-multi", "Authorization: Digest a=1,\r\n\t" + S + "\r\n\tmore\nnext line kept"},
		{"double-escaped-json", `{\\"password\\":\\"` + S + `\\"}`},
		{"double-escaped-with-space", `{\\"token\\":\\"` + S + ` ` + S2 + `\\"}`},
		{"userinfo-apostrophe", "dial postgres://u:pa'" + S + "@h/x"},
		{"userinfo-backslash", `dial postgres://u:pa\` + S + `@h/x`},
		{"share-double-slash", "GET /api/v1/logs/share//" + S + " failed"},
		{"share-triple-slash", "GET /api/v1/logs/share///" + S + " failed"},
		{"pass-word", "pass-word=" + S},
		{"pass_word", "pass_word=" + S},
		{"passphrase", "passphrase=" + S},
		{"refresh", "refresh=" + S},
		{"jwt-scheme", "got jwt " + S + " rejected"},
		{"Token-scheme", "Authorization failed: Token " + S},
		{"arrow", "password -> " + S},
		{"fat-arrow", "password => " + S},
		{"walrus", "token := " + S},
		{"pct-name", "%74oken=" + S},
		{"pct-sep-eq", "token%3D" + S},
		{"pct-sep-colon", "password%3A" + S},
		{"pct-both", "%70assword%3d" + S},
		{"pct-in-query", "GET /x?a=1&token%3D" + S + "&b=2"},
	}
	for _, c := range leaks {
		t.Run(c.name, func(t *testing.T) {
			out := Text(c.in)
			if strings.Contains(out, S) || strings.Contains(out, S2) {
				t.Fatalf("in=%q out=%q", c.in, out)
			}
			if !strings.Contains(out, "REDACTED") && !strings.Contains(out, ":redacted") {
				t.Fatalf("nothing redacted: in=%q out=%q", c.in, out)
			}
		})
	}
	// folded header: the line AFTER the folded block is not part of the header
	if out := Text("Authorization: Digest a=1,\n\tcont\nnext line kept"); !strings.Contains(out, "next line kept") {
		t.Errorf("over-consumed past the fold: %q", out)
	}
	// benign neighbours stay readable
	for _, in := range []string{"token count exceeded", "Token limit reached", "jwt", "the password field is required", "a -> b", "x => y",
		"100% done", "path /a%20b/c", "refreshing cache", "name: bob", "Key: value"} {
		if got := Text(in); got != in {
			t.Errorf("benign changed: %q -> %q", in, got)
		}
	}
}

func TestSecretPath_DoubleSlash(t *testing.T) {
	const S = "SSECRETVAL1"
	for _, in := range []string{"/api/v1/logs/share//" + S, "/api/v1/logs/share///" + S, "/api/v1/logs/share/" + S, "/reset///" + S + "/x"} {
		if out := SecretPath(in); strings.Contains(out, S) {
			t.Errorf("SecretPath(%q) = %q leaks", in, out)
		}
	}
	if got := SecretPath("/api/v1/logs/share//"); got != "/api/v1/logs/share//" {
		t.Errorf("empty tail changed: %q", got)
	}
}

// Property fuzz extended to the F3 shapes (20 000 cases): the generated value never survives, the text after it does.
func TestText_F3PropertyFuzz(t *testing.T) {
	keys := []string{"password", "pass-word", "passphrase", "token", "access_token", "api_key", "client_secret", "refresh_token", "jwt", "Cookie", "authorization", "secret"}
	seps := []string{"=", ": ", ":", " -> ", " => ", " := ", "%3D", "%3a", "\t"}
	wrap := []func(v string) string{
		func(v string) string { return v },
		func(v string) string { return `"` + v + `"` },
		func(v string) string { return `'` + v + `'` },
		func(v string) string { return `\"` + v + `\"` },
		func(v string) string { return `\\"` + v + `\\"` },
		func(v string) string { return `["` + v + `","` + v + `x"]` },
		func(v string) string { return `[` + v + ` ` + v + `y]` },
	}
	const alpha = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789._~+/-"
	seed := uint32(12345)
	next := func(n int) int { seed = seed*1664525 + 1013904223; return int(seed>>8) % n }
	cases := 0
	for cases < 20000 {
		k := keys[next(len(keys))]
		sp := seps[next(len(seps))]
		if strings.HasPrefix(sp, "%") && len(k) > 0 { // encoded separators carry no spaces
			sp = strings.TrimSpace(sp)
		}
		w := wrap[next(len(wrap))]
		if strings.HasPrefix(sp, "%") && w(`x`) != `x` { // an encoded separator continues the same token: plain value only
			w = wrap[0]
		}
		vb := make([]byte, 6+next(10))
		for i := range vb {
			vb[i] = alpha[next(len(alpha))]
		}
		val := "ZQ" + string(vb) + "QZ"
		if k == "Cookie" || k == "authorization" { // header names own their line
			continue
		}
		in := "pre " + k + sp + w(val) + " after=keep"
		out := Text(in)
		cases++
		if strings.Contains(out, val) {
			t.Fatalf("value survived: in=%q out=%q", in, out)
		}
		if !strings.Contains(out, "after=keep") {
			t.Fatalf("over-consumed: in=%q out=%q", in, out)
		}
	}
}
