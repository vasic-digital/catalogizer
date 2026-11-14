package logredact

import (
	"strings"
	"testing"
)

// Paths whose secret ("S3cretVal") follows a secret key, possibly through dot / param-only segments.
var r3Secret = []string{
	"/share;x=1/S3cretVal", "/share./S3cretVal", "/share../S3cretVal", "/SHARE./S3cretVal", "/share%3Bx=1/S3cretVal",
	"/share%253Bx=1/S3cretVal", "/share%2e/S3cretVal", "/share;/S3cretVal", "/Token;a;b./S3cretVal",
	// HYG-R3-F1 round-1 review F-1: dot / param-only segments between key and secret
	"/api/v1/logs/share/./S3cretVal", "/share/;/S3cretVal", "/share/%2e/S3cretVal", "/share/..;/S3cretVal", "/share/./S3cretVal",
	"/share/;x=1/S3cretVal", "/share/../S3cretVal", "/share/.;/S3cretVal", "/share./%2E/S3cretVal", "/share//./;/S3cretVal",
	// F-2: deeper escapes than the decode cap are treated as keys
	"/share/;S3cretVal", "/share/;token=S3cretVal", "/share/..;S3cretVal", "/api/v1/logs/share/;token=S3cretVal", "/share/%3BS3cretVal",
	"/share/%2525252e/S3cretVal", "/share/.%00/S3cretVal", "/share/.%00/x/S3cretVal", "/share/;x=1/a/S3cretVal",
	"/share%2525253Bx/S3cretVal",
	// round 3 R3-2: dot look-alikes, a control-byte key, the key's own parameter values
	"/share/%c0%ae/S3cretVal", "/share/%e2%80%a4/S3cretVal", "/share/\u2024/S3cretVal", "/share%00/S3cretVal", "/share%00;x/S3cretVal",
	"/share;S3cretVal/next", "/share;token=S3cretVal/next", "/Token;S3cretVal/next/more", "/share/%20/S3cretVal", "/share/%25252525S3cretVal", "/share%25252525252E/S3cretVal",
}

// Key's own parameter values with nothing after the key: only the key base name may stay.
var r3KeyParamOnly = []string{"/share;S3cretVal", "/share;token=S3cretVal", "/share%3BS3cretVal", "/Token;a=S3cretVal;b"}

// Literal space / control byte probes: not representable in prose or in an HTTP request line.
var r3Raw = []string{"/share/. /S3cretVal", "/share/ /S3cretVal", "/share\x00/S3cretVal", "/share/.\x00/S3cretVal"}

var r3Plain = []string{"/api/v1/media/42/;x/S3cretVal", "/api/v1/collections/9/files/;v=2/S3cretVal", "/xkeys/list/S3cretVal", "/other;x=1/S3cretVal", "/sharing./S3cretVal", "/x.share/S3cretVal", "/share-x;1/S3cretVal", "/other/./S3cretVal", "/sharing/;/S3cretVal", "/api/v1/media/42/./S3cretVal", "/x.share/;/S3cretVal", "/a%2541/S3cretVal"}

func TestHygR3F1_SecretPathVariants(t *testing.T) {
	for _, in := range r3Secret {
		if got := SecretPath(in); strings.Contains(got, "S3cretVal") || !strings.HasSuffix(got, ":redacted") {
			t.Errorf("SecretPath(%q) = %q, want the secret redacted", in, got)
		}
		found := false
		for _, k := range KnownSecrets(in, "", nil, nil) {
			if strings.Contains(k, "S3cretVal") {
				found = true
			}
		}
		if !found {
			t.Errorf("KnownSecrets(%q) did not collect the secret", in)
		}
		if got := Text("lookup " + in + " failed"); strings.Contains(got, "S3cretVal") {
			t.Errorf("Text(%q) = %q, secret leaked", in, got)
		}
	}
	for _, in := range r3Plain {
		if got := SecretPath(in); got != in {
			t.Errorf("SecretPath(%q) = %q, want unchanged (negative control)", in, got)
		}
		if ks := KnownSecrets(in, "", nil, nil); len(ks) != 0 {
			t.Errorf("KnownSecrets(%q) = %v, want none", in, ks)
		}
		if got := Text("lookup " + in + " failed"); !strings.Contains(got, "S3cretVal") {
			t.Errorf("Text(%q) = %q, non-secret path was redacted (negative control)", in, got)
		}
	}
}

func TestHygR3F1_KeyParamOnlyAndRawProbes(t *testing.T) {
	for _, in := range r3KeyParamOnly {
		if got := SecretPath(in); strings.Contains(got, "S3cretVal") || got == in {
			t.Errorf("SecretPath(%q) = %q", in, got)
		}
		if got := Text("lookup " + in + " failed"); strings.Contains(got, "S3cretVal") {
			t.Errorf("Text(%q) = %q", in, got)
		}
		// R3-3: an error text that echoes ONLY the key's parameter value must be cleaned through the known secrets.
		if got := ErrorTextKnown("lookup S3cretVal failed", KnownSecrets(in, "", nil, nil)); strings.Contains(got, "S3cretVal") {
			t.Errorf("ErrorTextKnown for %q echoed the key parameter value: %q", in, got)
		}
	}
	for _, in := range r3Raw {
		if got := SecretPath(in); strings.Contains(got, "S3cretVal") {
			t.Errorf("SecretPath(%q) = %q", in, got)
		}
		if got := PathKnown(in, "", KnownSecrets(in, "", nil, nil)); strings.Contains(got, "S3cretVal") {
			t.Errorf("PathKnown(%q) = %q", in, got)
		}
	}
}

// Regular shapes keep their pre-fix behaviour: only the segment after the key is redacted.
func TestHygR3F1_RegularShapeUnchanged(t *testing.T) {
	for in, want := range map[string]string{
		"/api/v1/keys/list":              "/api/v1/keys/:redacted",
		"/api/v1/logs/share/AbC123/more": "/api/v1/logs/share/:redacted/more",
		"/share":                         "/share",
		"/share/":                        "/share/",
		"/share//":                       "/share//",
	} {
		if got := SecretPath(in); got != want {
			t.Errorf("SecretPath(%q) = %q, want %q", in, got, want)
		}
	}
}

// Deterministic cost oracle (work counted by segmentWork, not wall clock): 4x input must cost at most ~5x work and a
// bounded work per input byte, for every shape that makes a key look ahead.
func TestHygR3F1_LinearWork(t *testing.T) {
	shapes := []string{"/share/;x", "/share/.%00", "/share/a", "/share//", "/share;a=b", "/share/%c0%ae", "/token/x/key", "/share%2525253B", "/share/%25252525"}
	work := func(n int, shape string, f func(string)) int64 {
		in := strings.Repeat(shape, n/len(shape)+1)[:n]
		segmentWork.Store(0)
		f(in)
		return segmentWork.Load()
	}
	fns := map[string]func(string){
		"SecretPath":   func(s string) { _ = SecretPath(s) },
		"KnownSecrets": func(s string) { _ = KnownSecrets(s, "", nil, nil) },
		"Text":         func(s string) { _ = Text(s) },
	}
	for _, shape := range shapes {
		for name, f := range fns {
			small, big := work(16<<10, shape, f), work(64<<10, shape, f)
			if big > int64(64<<10)*16 {
				t.Errorf("%s %q: %d work units for 64 KiB (> 16/byte)", name, shape, big)
			}
			if small > 0 && float64(big) > 5.5*float64(small) {
				t.Errorf("%s %q: 4x input cost %.1fx work (%d -> %d)", name, shape, float64(big)/float64(small), small, big)
			}
		}
	}
	// control needle: the counter sees work at all
	if segmentWork.Store(0); work(1<<10, "/share/a", fns["SecretPath"]) == 0 {
		t.Fatal("control needle: segmentWork did not move")
	}
}
