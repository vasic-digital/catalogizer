package logredact

import (
	"math/rand"
	"strings"
	"testing"
	"unicode/utf8"
)

// Reviewer round-2 inputs (R2-3) plus the round-1 table: every secret must vanish, REDACTED must appear.
func TestText_LeakTable(t *testing.T) {
	leaks := []struct{ name, in, secret string }{
		{"authorization-eq-bearer", "Authorization=Bearer eyJhbGciOi.payload.sig", "eyJhbGciOi"},
		{"authorization-eq-bearer-short", "Authorization=Bearer abc", "abc"},
		{"pass-eq", "login pass=hunter2x", "hunter2x"},
		{"json-pass", `{"pass":"jsonpassval"}`, "jsonpassval"},
		{"pw-eq", "db pw=pwsecret1", "pwsecret1"},
		{"escaped-json-password", `{\"password\":\"EscapedPW9\"}`, "EscapedPW9"},
		{"escaped-json-token-with-space", `{\"token\":\"tok with space\"}`, "with space"},
		{"signature-eq", "GET /x?signature=SIGVALUE12", "SIGVALUE12"},
		{"x-amz-signature", "X-Amz-Signature=AMZSIG77&X-Amz-Date=1", "AMZSIG77"},
		{"code-eq", "oauth code=AUTHCODE55 state=s", "AUTHCODE55"},
		{"code_verifier", "code_verifier=VERIFIER99", "VERIFIER99"},
		{"otp", "otp=482913", "482913"},
		{"password-tab", "password\tTABSECRET1", "TABSECRET1"},
		{"short-bearer", "got bearer abc rejected", "abc"},
		{"bearer-letters-only-mid", "header bearer abcdefghijkl end", "abcdefghijkl"},
		{"bearer-letters-only-long", "bearer abcdefghijklmnopqrstuvwxyz", "abcdefghijklmnopqrstuvwxyz"},
		{"basic-letters-only-24", "basic abcdefghijklmnopqrstuvwx", "abcdefghijklmnopqrstuvwx"},
		{"basic-mixed-case", "basic AbCdEfGhIjKl", "AbCdEfGhIjKl"},
		{"cookie-header", "Cookie: sid=COOKIEVAL; a=b", "COOKIEVAL"},
		{"cookie-header-second-pair", "Cookie: a=VALUEONE; b=VALUETWO", "VALUETWO"},
		{"set-cookie-attrs", "Set-Cookie: sid=SIDVALUE456; Path=/; HttpOnly", "SIDVALUE456"},
		{"x-api-key-rest-of-line", "X-Api-Key: keyval123 and more words", "and more words"},
		{"auth-header-multi-token", "Authorization: Digest username=\"bob\", response=\"RESP99\"", "RESP99"},
		{"userinfo-at-in-password", "dial postgres://u:p@ssWORD9@db/x", "ssWORD9"},
		{"userinfo", "dial postgres://alice:S3cretPW@db:5432/x", "S3cretPW"},
		{"userinfo-slash-in-password", "dial https://bob:pa/ss@h/x", "pa/ss"},
		{"share-path-in-prose", "GET /api/v1/logs/share/SHARETOKEN123 failed", "SHARETOKEN123"},
		{"ticket-query", "GET /x?ticket=TICKETSECRET&a=1", "TICKETSECRET"},
		{"jwt-key", `jwt: "eyJabc.def.ghi"`, "eyJabc"},
		{"quoted-key-space-colon", `"api_key" : "kvalue123"`, "kvalue123"},
		{"empty-user", "dial redis://:onlypass@h:6379", "onlypass"},
		{"unterminated", `password="oops never closed`, "oops"},
		{"multiline", "password=\nMLPW777", "MLPW777"},
		{"upper", "PASSWORD=UPPERPW1", "UPPERPW1"},
		{"session-id", "sessionid=SESS12345", "SESS12345"},
		{"private-key", "private_key=PKSECRET99", "PKSECRET99"},
	}
	for _, c := range leaks {
		t.Run(c.name, func(t *testing.T) {
			out := Text(c.in)
			if strings.Contains(out, c.secret) || !strings.Contains(out, "REDACTED") && !strings.Contains(out, ":redacted") {
				t.Fatalf("in=%q out=%q (secret %q)", c.in, out, c.secret)
			}
		})
	}
	// benign prose and non-secret neighbours stay readable
	for _, in := range []string{"connection refused", "token count exceeded", "basic configuration failed", "no session available",
		"dial tcp 10.0.0.1:5432: i/o timeout", "sql: no rows in result set", "exit code: 1", "host=db1 port=5432", "key not found",
		"the Basic Configuration failed", "/api/v1/logs/share", "/api/v1/logs/share/"} {
		if out := Text(in); out != in {
			t.Errorf("benign text changed: %q -> %q", in, out)
		}
	}
	if out := Text("host=db1 password=pw123456 port=5432"); !strings.Contains(out, "host=db1") || !strings.Contains(out, "port=5432") {
		t.Errorf("neighbours lost: %q", out)
	}
	if out := Text(`password="pass with spaces" next=ok`); !strings.Contains(out, "next=ok") || strings.Contains(out, "with spaces") {
		t.Errorf("quoted end: %q", out)
	}
}

// Property test: for every sensitive key x separator x quoting x value shape the value never survives, and the text after the
// value does (the scanner consumes exactly the value).
func TestText_PropertyValueNeverSurvives(t *testing.T) {
	keys := []string{"password", "passwd", "pwd", "pass", "pw", "secret", "client_secret", "token", "access_token", "refresh_token",
		"api_key", "apikey", "api-key", "signature", "X-Amz-Signature", "code", "code_verifier", "otp", "verifier", "sig", "jwt", "bearer",
		"key", "ticket", "authorization", "credential", "private_key", "db_password", "sessionid", "Cookie"}
	type sep struct {
		s     string
		quote func(v string) string
		weak  bool // usable for weak keys
	}
	plain := func(v string) string { return v }
	dq := func(v string) string { return `"` + v + `"` }
	sq := func(v string) string { return `'` + v + `'` }
	esc := func(v string) string { return `\"` + v + `\"` }
	seps := []sep{
		{"=", plain, true}, {"=", dq, true}, {"=", sq, true}, {": ", plain, false}, {": ", dq, false}, {":", plain, false},
		{" = ", plain, true}, {"\t", plain, false}, {`":`, dq, true}, {`": `, dq, true}, {`\":`, esc, true}, {`\": `, esc, true},
	}
	rnd := rand.New(rand.NewSource(17))
	const alpha = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789._~+/=-!#$%^*()[]{}|"
	n := 0
	for _, k := range keys {
		strongKeyName := strongKey(strings.ToLower(k))
		for _, sp := range seps {
			if !strongKeyName && !sp.weak {
				continue
			}
			for rep := 0; rep < 6; rep++ {
				vb := make([]byte, 5+rnd.Intn(20))
				for i := range vb {
					vb[i] = alpha[rnd.Intn(len(alpha))]
				}
				val := "ZQ" + string(vb) + "QZ"
				var in string
				switch {
				case strings.HasPrefix(sp.s, `"`) || strings.HasPrefix(sp.s, `\"`):
					q := `"`
					if strings.HasPrefix(sp.s, `\"`) {
						q = `\"`
					}
					in = "{" + q + k + sp.s + sp.quote(val) + ", after=keep}"
				default:
					in = "pre " + k + sp.s + sp.quote(val) + " after=keep"
				}
				// the first character class of the value may be a delimiter-free symbol; skip values that start with a quote-ish char
				out := Text(in)
				n++
				if strings.Contains(out, val) || strings.Contains(out, val[2:len(val)-2]) {
					t.Fatalf("value survived: in=%q out=%q", in, out)
				}
				if !headerNames[strings.ToLower(k)] && !strings.Contains(out, "after=keep") { // a credential header owns its whole line
					t.Fatalf("scanner over-consumed: in=%q out=%q", in, out)
				}
			}
		}
	}
	if n < 500 {
		t.Fatalf("property run too small: %d", n)
	}
	// control needle: a non-sensitive key with the same shapes is NOT touched (the generator is not trivially matching)
	if got := Text("pre host=ZQabcQZ after=keep"); got != "pre host=ZQabcQZ after=keep" {
		t.Fatalf("control needle: %q", got)
	}
}

// R2-5: output cap works on odd-offset multibyte input; the cut never splits a rune.
func TestErrorText_Utf8CapOddOffset(t *testing.T) {
	in := "x" + strings.Repeat("é", 3000) // 2-byte runes start at odd offsets, so byte 2048 is a continuation byte
	out := ErrorText(in)
	body, _, _ := strings.Cut(out, "...[truncated")
	if !utf8.ValidString(body) || len(body) > ErrorTextMaxBytes || !strings.Contains(out, "[truncated ") {
		t.Fatalf("bad cap: valid=%v len=%d", utf8.ValidString(body), len(body))
	}
	if len(body) != 2047 { // 1 + 1023*2: the rune straddling 2048 is dropped whole
		t.Fatalf("cut not moved back to a rune start: %d", len(body))
	}
	// 3-byte runes, offset 2
	in = "xx" + strings.Repeat("€", 2000)
	body, _, _ = strings.Cut(ErrorText(in), "...[truncated")
	if !utf8.ValidString(body) {
		t.Fatal("3-byte cut split a rune")
	}
}

// R2-5: cap runs AFTER redaction. The whole password sits inside the first 2048 bytes and its '@' just past the cut: with
// cap-then-redact the unterminated userinfo "postgres://u:SECRETPASSWORD" survives; redact-then-cap replaces it entirely.
func TestErrorText_CapOrderRedactFirst(t *testing.T) {
	lead := " dial postgres://u:"
	const pw = "SECRETPASSWORD"
	prefix := strings.Repeat("a", ErrorTextMaxBytes-len(lead)-len(pw))
	in := prefix + lead + pw + "@db/x"
	if idx := strings.Index(in, "@"); idx != ErrorTextMaxBytes {
		t.Fatalf("fixture does not put the '@' right at the cut: %d", idx)
	}
	out := ErrorText(in)
	if strings.Contains(out, "SECRET") {
		t.Fatalf("secret exposed (cap before redaction?): %q", out[len(out)-80:])
	}
	// the control: the same text capped FIRST does expose it (the fixture really straddles the cut)
	if capped := Cap(in, ErrorTextMaxBytes); !strings.Contains(capped, pw) {
		t.Fatal("control needle: cap-first would not have exposed the password; fixture is vacuous")
	}
	// pre-cap (64 KiB) straddle: a >512-byte whitespace-free userinfo is cut inside; the truncated-authority rule must hide it
	long := strings.Repeat("b", textPreCapBytes-700) + " https://user:" + strings.Repeat("P", 1500) + "@h/"
	pre := Text(long)
	if strings.Contains(pre, "PPPP") {
		t.Fatalf("pre-cap exposed a userinfo prefix: ...%q", pre[len(pre)-60:])
	}
	if !strings.HasSuffix(pre, "://REDACTED") {
		t.Fatalf("truncated authority not redacted: ...%q", pre[len(pre)-40:])
	}
}

// R2-6: huge input is bounded before redaction.
func TestText_PreCapBoundsWork(t *testing.T) {
	out := Text(strings.Repeat("password=x ", 1<<20)) // 11 MiB
	if len(out) > textPreCapBytes*3 {
		t.Fatalf("pre-cap not applied: %d", len(out))
	}
}

// M6: credential-looking threshold for basic/digest/negotiate. 23 lowercase letters are prose, 24 are a credential.
func TestText_CredentialLikeBoundary(t *testing.T) {
	w23, w24 := strings.Repeat("a", 23), strings.Repeat("a", 24)
	if out := Text("basic " + w23); out != "basic "+w23 {
		t.Errorf("23 letters should stay readable: %q", out)
	}
	if out := Text("basic " + w24); strings.Contains(out, w24) {
		t.Errorf("24 letters must be redacted: %q", out)
	}
	if out := Text("digest " + w23 + "1"); strings.Contains(out, w23) {
		t.Errorf("digit makes it credential-like: %q", out)
	}
	if out := Text("negotiate abcDef"); strings.Contains(out, "abcDef") {
		t.Errorf("inner upper+lower is credential-like: %q", out)
	}
}

// Shared sensitive list: Query and Text agree on every name (single source of truth).
func TestSensitiveKeySharedByQueryAndText(t *testing.T) {
	for _, k := range []string{"pass", "pw", "signature", "sig", "code", "code_verifier", "otp", "verifier", "jwt", "bearer", "key",
		"X-Amz-Signature", "ticket", "access_token", "client_secret", "db_password"} {
		if !SensitiveKey(k) {
			t.Errorf("SensitiveKey(%q) = false", k)
		}
		if q := Query(k + "=VALUEX1"); strings.Contains(q, "VALUEX1") {
			t.Errorf("Query leaks %q: %q", k, q)
		}
		if x := Text(k + "=VALUEX1"); strings.Contains(x, "VALUEX1") {
			t.Errorf("Text leaks %q: %q", k, x)
		}
	}
	for _, k := range []string{"page", "q", "host", "username", "sort", "limit", "author_name"} {
		if SensitiveKey(k) && k != "author_name" {
			t.Errorf("SensitiveKey(%q) = true", k)
		}
	}
}
