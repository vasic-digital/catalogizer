package logredact

import (
	"strings"
	"testing"
)

// HYG-F4 / HYGR3-FU1: a backslash, an encoded or double-encoded '/' or '\' between the key and the secret is a separator.
var f4Embedded = []string{
	`/share\S3cretVal`, "/share%5CS3cretVal", "/share%2FS3cretVal", "/share%252FS3cretVal", "/share%255CS3cretVal/x",
	"/api/v1/logs/share%2fS3cretVal/more", "/Token%252fS3cretVal", `/share\S3cretVal/more/S3cretVal`, "/keys%2F42%2Frotate/S3cretVal",
	"/share%25252FS3cretVal",
}

// HYG-F4: a malformed token after a key (odd characters, invalid UTF-8, bad escapes, punctuation, empty-ish) fails closed.
var f4Malformed = []string{
	"/share/S3cretVal$x/more", "/share/S3cretVal,x/more", "/share/S3cretVal(x)/more", "/share/S3cretVal:x/more", "/share/S3cretVal@x/more",
	"/share/S3cretVal+x/more", "/share/S3cretVal*x/more", "/share/\xff\xfeS3cretVal/more", "/share/%ZZS3cretVal/more", "/share/S3cretVal;x/more",
	"/share/~/S3cretVal", "/share//;/S3cretVal", "/share/!S3cretVal/more", "/share/S3cretVal%00/more",
}

func TestHygF4_EmbeddedSeparators(t *testing.T) {
	for _, in := range append(append([]string{}, f4Embedded...), f4Malformed...) {
		if got := SecretPath(in); strings.Contains(got, "S3cretVal") || strings.Contains(got, "more") || !strings.Contains(got, ":redacted") {
			t.Errorf("SecretPath(%q) = %q, want secret and rest redacted", in, got)
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
}

// Over-redaction of regular shapes and non-secret look-alikes stays as before.
// The prose scanner marks an embedded-separator key the same way SecretPath does (base name + ":redacted").
func TestHygF4_TextMarksEmbedded(t *testing.T) {
	for _, in := range f4Embedded {
		if got := Text("lookup " + in + " failed"); !strings.Contains(got, "/:redacted") {
			t.Errorf("Text(%q) = %q, want the /:redacted marker", in, got)
		}
	}
}

func TestHygF4_RegularShapesUnchanged(t *testing.T) {
	for in, want := range map[string]string{
		"/api/v1/keys/42/rotate": "/api/v1/keys/:redacted/rotate",
		"/share/tok/files/a.mkv": "/share/:redacted/files/a.mkv",
		"/sharing%2FS3cretVal":   "/sharing%2FS3cretVal",
		`/other\S3cretVal`:       `/other\S3cretVal`,
		"/api/v1/media/42%2Fx":   "/api/v1/media/42%2Fx",
	} {
		if got := SecretPath(in); got != want {
			t.Errorf("SecretPath(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestHygF4_EmbeddedLinearWork(t *testing.T) {
	for _, shape := range []string{"/share%2F", `/share\x`, "/share%252F/a", "/token%5C"} {
		work := func(n int) int64 {
			in := strings.Repeat(shape, n/len(shape)+1)[:n]
			segmentWork.Store(0)
			_ = SecretPath(in)
			_ = KnownSecrets(in, "", nil, nil)
			_ = Text(in)
			return segmentWork.Load()
		}
		small, big := work(16<<10), work(64<<10)
		if small > 0 && float64(big) > 5.5*float64(small) {
			t.Errorf("%q: 4x input cost %.1fx work", shape, float64(big)/float64(small))
		}
	}
}
