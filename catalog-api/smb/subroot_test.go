package smb

import "testing"

func TestSplitShare(t *testing.T) {
	ok := []struct{ in, share, sub string }{
		{"Public", "Public", ""},
		{"Public/", "Public", ""},
		{"/Public", "Public", ""}, // legacy single-segment spelling
		{"Public/Testing", "Public", "Testing"},
		{"Public/Testing/", "Public", "Testing"},
		{"Public/a/b c/d", "Public", "a/b c/d"},
		{"Pub..lic/a..b", "Pub..lic", "a..b"}, // ".." inside a name is not a segment
	}
	for _, c := range ok {
		share, sub, err := SplitShare(c.in)
		if err != nil || share != c.share || sub != c.sub {
			t.Errorf("SplitShare(%q) = %q,%q,%v want %q,%q", c.in, share, sub, err, c.share, c.sub)
		}
	}
	bad := []string{"", "/", "//", "/Public/x", "/Public/Testing/", "Public/../x", "Public/..", "../Public", "Public/./x", "Public//x", `Public\x`, `Public/a\..\b`, "Public/x\x00y", "Public/x\ny", `\\host\Public`, "Public/a/../../b", "./Public"}
	for _, in := range bad {
		if share, sub, err := SplitShare(in); err == nil {
			t.Errorf("SplitShare(%q) = %q,%q accepted, want error", in, share, sub)
		}
	}
}

func TestResolveSub(t *testing.T) {
	cases := []struct{ sub, p, want string }{
		{"", "a/b", "a/b"}, // no sub root: the path is handed over unchanged (backwards compatible)
		{"", "", ""},
		{"Testing", "", "Testing"},
		{"Testing", ".", "Testing"},
		{"Testing", "a/b", "Testing/a/b"},
		{"Testing", "/a/b", "Testing/a/b"},
		{"Testing", "../x", "Testing/x"},
		{"Testing", "../../etc/passwd", "Testing/etc/passwd"},
		{"Testing", "a/../../x", "Testing/x"},
		{"Testing", `..\..\x`, "Testing/x"},
		{"Testing", `a\..\..\x`, "Testing/x"},
		{"a/b", "../../../c", "a/b/c"},
	}
	for _, c := range cases {
		if got := ResolveSub(c.sub, c.p); got != c.want {
			t.Errorf("ResolveSub(%q,%q) = %q want %q", c.sub, c.p, got, c.want)
		}
	}
}

// The client maps every operation through rp; a client with a sub root can never ask the server for a path outside it.
func TestSmbClientRPConfinesToSub(t *testing.T) {
	c := &SmbClient{sub: "Testing"}
	for p, want := range map[string]string{"a": "Testing/a", ".": "Testing", "": "Testing", "../../x": "Testing/x", `..\y`: "Testing/y"} {
		if got := c.rp(p); got != want {
			t.Errorf("rp(%q)=%q want %q", p, got, want)
		}
	}
	if got := (&SmbClient{}).rp("../a"); got != "../a" {
		t.Errorf("no sub root must pass the path through unchanged, got %q", got)
	}
}
