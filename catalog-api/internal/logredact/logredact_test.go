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
		"%zz=1&ticket=secretvalue":           "/p?REDACTED&ticket=REDACTED",
	}
	for q, want := range cases {
		if got := Path("/p", q); got != want {
			t.Errorf("Path(%q) = %q, want %q", q, got, want)
		}
	}
}
