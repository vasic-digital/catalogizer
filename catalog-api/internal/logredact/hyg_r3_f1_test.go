package logredact

import "testing"

func TestHygR3F1_SecretPathVariants(t *testing.T) {
	for _, in := range []string{
		"/share;x=1/S3cretVal", "/share./S3cretVal", "/share../S3cretVal", "/SHARE./S3cretVal", "/share%3Bx=1/S3cretVal",
		"/share%253Bx=1/S3cretVal", "/share%2e/S3cretVal", "/share;/S3cretVal", "/Token;a;b./S3cretVal",
	} {
		if got := SecretPath(in); got == in || got[len(got)-9:] != ":redacted" {
			t.Errorf("SecretPath(%q) = %q, want the secret redacted", in, got)
		}
		if ks := KnownSecrets(in, "", nil, nil); len(ks) == 0 {
			t.Errorf("KnownSecrets(%q) found nothing", in)
		}
	}
	for _, in := range []string{"/other;x=1/S3cretVal", "/sharing./S3cretVal", "/x.share/S3cretVal", "/share-x;1/S3cretVal"} {
		if got := SecretPath(in); got != in {
			t.Errorf("SecretPath(%q) = %q, want unchanged (negative control)", in, got)
		}
		if ks := KnownSecrets(in, "", nil, nil); len(ks) != 0 {
			t.Errorf("KnownSecrets(%q) = %v, want none", in, ks)
		}
	}
}
