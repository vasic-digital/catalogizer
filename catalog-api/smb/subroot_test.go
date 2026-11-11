package smb

import (
	"strings"
	"testing"
)

func TestSplitShare(t *testing.T) {
	ok := []struct{ in, share, sub string }{
		{"Public", "Public", ""},
		{"Public/", "Public", ""},
		{"/Public", "Public", ""}, // legacy single-segment spelling
		{"Public/Testing", "Public", "Testing"},
		{"Public/Testing/", "Public", "Testing"},
		{"Public/a/b c/d", "Public", "a/b c/d"},
		{"Pub..lic/a..b", "Pub..lic", "a..b"},
		{"Public/CONSOLE", "Public", "CONSOLE"}, // only the exact device names are reserved
		{"Public/COM10", "Public", "COM10"},
		{"Public/a.b./c", "Public", "a.b./c"}, // dots inside a name are fine
		{"CD", "CD", ""},
		{"Public/COM¹0", "Public", "COM¹0"},
		{"Public/CONIN", "Public", "CONIN"},
		{"C$x", "C$x", ""}, // ".." inside a name is not a segment
	}
	for _, c := range ok {
		share, sub, err := SplitShare(c.in)
		if err != nil || share != c.share || sub != c.sub {
			t.Errorf("SplitShare(%q) = %q,%q,%v want %q,%q", c.in, share, sub, err, c.share, c.sub)
		}
	}
	bad := []string{"", "/", "//", "/Public/x", "/Public/Testing/", "Public/../x", "Public/..", "../Public", "Public/./x", "Public//x", `Public\x`, `Public/a\..\b`, "Public/x\x00y", "Public/x\ny", `\\host\Public`, "Public/a/../../b", "./Public", "Public/.. ", "Public/...", "Public/a/.. /b", "Public/ ", "Public/CON", "Public/a/nul.txt", "Public/COM1", "Public/lpt9.x", "Public/a:stream", "IPC$ ", "ipc$./x", "ADMIN$ .", "C$ ", "Pub:lic", "Public/COM0", "Public/LPT0", "Public/COM¹", "Public/com².txt", "Public/LPT³", "Public/CONIN$", "Public/conout$.x", "Public/a/CONOUT$", "Public/a/b:$DATA", "IPC$", "ipc$/x", "ADMIN$", "C$", "d$/x", "Public/a\u0085b", "Public/a\u202eb", "Public/a\u2066b", `\\\\host\\Public`, "//host/Public", "//Public"}
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
		{"Testing", "a:stream", "Testing/" + RejectedSegment},
		{"Testing", "docs/b.txt:$DATA", "Testing/docs/" + RejectedSegment},
		{"Testing", `x\y:z`, "Testing/x/" + RejectedSegment},
		{"", "a:b", "a:b"}, // no sub root: unchanged
		{"Testing", "... /x", "Testing/x"},
		{"Testing", "a/.. /../../x", "Testing/x"},
		{"Testing", "..../etc", "Testing/etc"},
	}
	for _, c := range cases {
		if got := ResolveSub(c.sub, c.p); got != c.want {
			t.Errorf("ResolveSub(%q,%q) = %q want %q", c.sub, c.p, got, c.want)
		}
	}
}

func TestSplitShareUNCRefusedWithHint(t *testing.T) {
	for _, in := range []string{`\\host\Public`, "//host/Public", `\\host\Public\Testing`} {
		_, _, err := SplitShare(in)
		if err == nil || !strings.Contains(err.Error(), "UNC") || !strings.Contains(err.Error(), "'host'") {
			t.Errorf("SplitShare(%q) err=%v, want a UNC hint naming the host field", in, err)
		}
	}
}
