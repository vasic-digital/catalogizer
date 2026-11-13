package logredact

import "testing"

func TestPath(t *testing.T) {
	cases := map[string]string{
		"":                                   "/p",
		"ticket=abc":                         "/p?ticket=REDACTED",
		"a=1&access_token=jwt&b=2":           "/p?a=1&access_token=REDACTED&b=2",
		"token=x&download=true":              "/p?token=REDACTED&download=true",
		"page=2&q=hello":                     "/p?page=2&q=hello",
		"%74icket=secretvalue":               "/p?%74icket=REDACTED",
		"Ticket=SecretValue":                 "/p?Ticket=REDACTED",
		"host=nas&password=hunter2&domain=W": "/p?host=nas&password=REDACTED&domain=W",
		"PassWord=x&username=bob":            "/p?PassWord=REDACTED&username=bob",
		"a=1&apikey=k1&b=2":                  "/p?a=1&apikey=REDACTED&b=2",
		"a=1;password=x":                     "/p?a=1;password=REDACTED",
		"a=1;b=2&token=t;c=3":                "/p?a=1;b=2&token=REDACTED;c=3",
		"%zz=1&ticket=secretvalue":           "/p?REDACTED&ticket=REDACTED",
	}
	for q, want := range cases {
		if got := Path("/p", q); got != want {
			t.Errorf("Path(%q) = %q, want %q", q, got, want)
		}
	}
}

func TestSecretPath(t *testing.T) {
	cases := map[string]string{
		"":                                  "",
		"/api/v1/logs/share/SHARETOKEN_abc": "/api/v1/logs/share/:redacted",
		"/api/v1/logs/share":                "/api/v1/logs/share",
		"/api/v1/logs/share/":               "/api/v1/logs/share/",
		"/api/v1/SHARE/Tok/x":               "/api/v1/SHARE/:redacted/x",
		"/api/v1/media/42":                  "/api/v1/media/42",
		"/auth/reset/abc123/confirm/zzz":    "/auth/reset/:redacted/confirm/:redacted",
	}
	for in, want := range cases {
		if got := SecretPath(in); got != want {
			t.Errorf("SecretPath(%q)=%q want %q", in, got, want)
		}
	}
	if got := Path("/api/v1/logs/share/TOK", "a=1&token=x"); got != "/api/v1/logs/share/:redacted?a=1&token=REDACTED" {
		t.Errorf("Path: %q", got)
	}
}
