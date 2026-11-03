package filesystem

// Gate-logic unit tests (section 11.4.289(C)(2)) for the pure decisions of the live WebDAV test. They prove the guards decide correctly;
// they are NOT evidence that WebDAV works.

import (
	"fmt"
	pathpkg "path"
	"strings"
	"testing"
)

type probeVerdict int

const (
	probeOK     probeVerdict = iota // 207: a WebDAV service answered
	probeNotDAV                     // the endpoint is not a WebDAV service: honest SKIP
	probeFail                       // the dependency is present but rejects or misbehaves: FAIL
)

// classifyProbe decides what a PROPFIND answer means. Only "not a WebDAV service" may SKIP (section 11.4.289(E)); 401/403/404/5xx and a
// DAV-speaking server that does not answer 207 mean the dependency is PRESENT and wrong, so they FAIL.
func classifyProbe(status int, davHeader string) probeVerdict {
	switch {
	case status == 207:
		return probeOK
	case davHeader != "":
		return probeFail
	case status == 405 || status == 200 || status == 501:
		return probeNotDAV
	default:
		return probeFail
	}
}

// validateSandbox cleans the sandbox path and refuses anything that is not a rooted path of at least share/folder with no dot segments.
func validateSandbox(raw string) (string, error) {
	if !strings.HasPrefix(raw, "/") {
		return "", fmt.Errorf("sandbox must start with /")
	}
	for _, seg := range strings.Split(raw, "/") {
		if seg == "." || seg == ".." {
			return "", fmt.Errorf("sandbox must not contain . or .. segments")
		}
	}
	c := pathpkg.Clean(raw)
	if c == "/" || strings.Count(strings.Trim(c, "/"), "/") < 1 {
		return "", fmt.Errorf("sandbox must be at least share/folder deep")
	}
	return c, nil
}

func TestLiveWebDAVClassifyProbe(t *testing.T) {
	cases := []struct {
		status int
		dav    string
		want   probeVerdict
	}{
		{207, "", probeOK}, {207, "1,2", probeOK},
		{405, "", probeNotDAV}, {200, "", probeNotDAV}, {501, "", probeNotDAV},
		{200, "1,2", probeFail}, // DAV header but not 207: server is DAV and misbehaving
		{401, "", probeFail}, {403, "", probeFail}, {404, "", probeFail}, {500, "", probeFail}, {401, "1", probeFail},
	}
	for _, c := range cases {
		if got := classifyProbe(c.status, c.dav); got != c.want {
			t.Errorf("classifyProbe(%d,%q)=%v want %v", c.status, c.dav, got, c.want)
		}
	}
}

func TestLiveWebDAVValidateSandbox(t *testing.T) {
	ok := map[string]string{"/work20/Testing": "/work20/Testing", "/work20/Testing/": "/work20/Testing", "/work20//Testing": "/work20/Testing", "/work20/a/b": "/work20/a/b"}
	for in, want := range ok {
		got, err := validateSandbox(in)
		if err != nil || got != want {
			t.Errorf("validateSandbox(%q)=%q,%v want %q", in, got, err, want)
		}
	}
	bad := []string{"", "/", "//", "/work20", "/work20/", "/work20/..", "/work20/.", "/work20//", "work20/Testing", "/work20/../x", "/work20/./Testing", "/../a/b", "/a/b/..", "."}
	for _, in := range bad {
		if got, err := validateSandbox(in); err == nil {
			t.Errorf("validateSandbox(%q) accepted as %q", in, got)
		}
	}
}
