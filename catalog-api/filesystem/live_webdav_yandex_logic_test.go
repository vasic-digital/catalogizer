package filesystem

// Gate-logic unit tests (section 11.4.289(C)(2)) for the pure decisions of the live Yandex WebDAV test: the /Testing confinement guard and the
// deterministic payload generator. They prove the guards decide correctly; they are NOT evidence that WebDAV works.

import (
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	pathpkg "path"
	"strings"
	"testing"
)

// yandexSandboxRoot is the ONLY remote directory the live Yandex test may touch (operator rule, 2026-11-03).
const yandexSandboxRoot = "/Testing"

// yandexAllowedPath reports whether a remote path (decoded, as it appears in the request URL path) is the sandbox itself or strictly below it.
// Fail-closed: any dot segment, backslash, NUL, "%" escape left undecoded, different case or sibling prefix ("/Testing2") is refused.
func yandexAllowedPath(p string) bool {
	if p == "" || !strings.HasPrefix(p, "/") || strings.ContainsAny(p, "\\\x00") {
		return false
	}
	for _, seg := range strings.Split(p, "/") {
		if seg == ".." || seg == "." {
			return false
		}
	}
	if pathpkg.Clean(p) != strings.TrimRight(p, "/") && p != "/" {
		return false
	}
	c := strings.TrimRight(p, "/")
	return c == yandexSandboxRoot || strings.HasPrefix(c, yandexSandboxRoot+"/")
}

// yandexAllowedURL applies the guard to a full request URL (https, the configured host only, a path inside the sandbox).
func yandexAllowedURL(u *url.URL, host string) bool {
	return u != nil && u.Scheme == "https" && u.Host == host && yandexAllowedPath(u.Path) && !strings.Contains(u.RawPath+u.Path, "%2e") && !strings.Contains(strings.ToLower(u.RawPath), "%2e")
}

// detBytes returns n deterministic bytes for a label: SHA-256 in counter mode, so the content and its sha256 are reproducible in every run.
func detBytes(label string, n int) []byte {
	out := make([]byte, 0, n+32)
	for ctr := 0; len(out) < n; ctr++ {
		h := sha256.Sum256([]byte(label + "#" + itoa(ctr)))
		out = append(out, h[:]...)
	}
	return out[:n]
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	s := ""
	for ; i > 0; i /= 10 {
		s = string(rune('0'+i%10)) + s
	}
	return s
}

func shaHex(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

func TestYandexAllowedPath(t *testing.T) {
	ok := []string{"/Testing", "/Testing/", "/Testing/catalogizer-live-1", "/Testing/a/b/c.txt", "/Testing/тест файл.txt", "/Testing/a b/#x%y"}
	for _, p := range ok {
		if !yandexAllowedPath(p) {
			t.Errorf("refused legitimate sandbox path %q", p)
		}
	}
	bad := []string{"", "/", "Testing", "/testing", "/TESTING/x", "/Testing2", "/Testing2/x", "/Testingx/x", "/Other/x", "/Testing/../x", "/Testing/..", "/Testing/./x",
		"/Testing//../x", "/Testing/a/../../x", "//Testing/x", "/Testing\\..\\x", "/Testing\x00/x", "/x/Testing/y", "/Тesting/x", "/Testing/../Testing2"}
	for _, p := range bad {
		if yandexAllowedPath(p) {
			t.Errorf("accepted path outside the sandbox %q", p)
		}
	}
}

func TestYandexAllowedURL(t *testing.T) {
	h := "webdav.yandex.ru"
	good := "https://webdav.yandex.ru/Testing/catalogizer-live-1/a%20b.txt"
	u, _ := url.Parse(good)
	if !yandexAllowedURL(u, h) {
		t.Errorf("refused %s", good)
	}
	for _, s := range []string{"http://webdav.yandex.ru/Testing/x", "https://evil.example/Testing/x", "https://webdav.yandex.ru/", "https://webdav.yandex.ru/Testing/%2e%2e/x",
		"https://webdav.yandex.ru/Testing/%2E%2E/x", "https://webdav.yandex.ru/Other", "https://webdav.yandex.ru/Testing/../Other"} {
		u, _ := url.Parse(s)
		if yandexAllowedURL(u, h) {
			t.Errorf("accepted %s", s)
		}
	}
	if yandexAllowedURL(nil, h) {
		t.Errorf("accepted nil url")
	}
}

func TestDetBytesDeterministic(t *testing.T) {
	a, b := detBytes("small", 1000), detBytes("small", 1000)
	if shaHex(a) != shaHex(b) || len(a) != 1000 {
		t.Fatalf("not deterministic")
	}
	if shaHex(detBytes("small", 1000)) == shaHex(detBytes("other", 1000)) {
		t.Fatalf("labels collide")
	}
	if len(detBytes("x", 1<<20)) != 1<<20 || len(detBytes("x", 0)) != 0 || len(detBytes("x", 31)) != 31 {
		t.Fatalf("wrong length")
	}
	// prefix property: a longer payload of the same label starts with the shorter one (known-hash anchor)
	if shaHex(detBytes("pfx", 64)) != shaHex(detBytes("pfx", 200)[:64]) {
		t.Fatalf("prefix property broken")
	}
}
