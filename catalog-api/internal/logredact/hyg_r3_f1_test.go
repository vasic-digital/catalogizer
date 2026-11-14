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
	"/share%2525253Bx/S3cretVal", "/share/%25252525S3cretVal", "/share%25252525252E/S3cretVal",
}

var r3Plain = []string{"/other;x=1/S3cretVal", "/sharing./S3cretVal", "/x.share/S3cretVal", "/share-x;1/S3cretVal", "/other/./S3cretVal", "/a%2541/S3cretVal"}

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
