package logredact

import (
	"strings"
	"testing"
)

// HYGF4-F1: a key segment holding a valid encoded separator AND an invalid escape must still redact (fail closed).
var f1Shapes = []string{
	"/share%2FS3cretVal%ZZ/more",
	"/share%2FS3cretVal%Z",
	"/share%2FS3cretVal%/more",
	"/api/v1/logs/share%2FS3cretVal%ZZ/more",
	"/share%252FS3cretVal%ZZ/more",
	"/share%5CS3cretVal%G1",
}

func TestHygF4F1_InvalidEscapeAfterEncodedSeparator(t *testing.T) {
	for _, in := range f1Shapes {
		if got := SecretPath(in); strings.Contains(got, "S3cretVal") || strings.Contains(got, "more") || !strings.Contains(got, ":redacted") {
			t.Errorf("SecretPath(%q) = %q", in, got)
		}
		if got := Path(in, ""); strings.Contains(got, "S3cretVal") {
			t.Errorf("Path(%q) = %q", in, got)
		}
		if got := Text("lookup " + in + " failed"); strings.Contains(got, "S3cretVal") {
			t.Errorf("Text(%q) = %q", in, got)
		}
		ks := KnownSecrets(in, "", nil, nil)
		found := false
		for _, k := range ks {
			if strings.Contains(k, "S3cretVal") {
				found = true
			}
		}
		if !found {
			t.Errorf("KnownSecrets(%q) = %q, secret not collected", in, ks)
		}
		if got := ErrorTextKnown("open "+in+": denied", ks); strings.Contains(got, "S3cretVal") {
			t.Errorf("ErrorTextKnown(%q) = %q", in, got)
		}
		if got := PathKnown(in, "", ks); strings.Contains(got, "S3cretVal") {
			t.Errorf("PathKnown(%q) = %q", in, got)
		}
	}
}

func TestHygF4F1_BenignUnchanged(t *testing.T) {
	for _, in := range []string{"/api/v1/media/42%ZZ/x", "/api/v1/files/a%2Fb%Z", "/other%2FS3cretVal%ZZ", "/api/v1/stats"} {
		if got := SecretPath(in); got != in {
			t.Errorf("SecretPath(%q) = %q, want unchanged", in, got)
		}
	}
	if got := SecretPath("/api/v1/keys/42/rotate"); got != "/api/v1/keys/:redacted/rotate" {
		t.Errorf("regular shape changed: %q", got)
	}
}

// HYGF4-F2: values collected by the KnownSecrets embedded branch are asserted via ErrorTextKnown.
func TestHygF4F2_EmbeddedBranchValues(t *testing.T) {
	has := func(ks []string, v string) bool {
		for _, k := range ks {
			if k == v {
				return true
			}
		}
		return false
	}
	ks := KnownSecrets("/share%2Fx=S3cretVal", "", nil, nil)
	if !has(ks, "x=S3cretVal") || !has(ks, "S3cretVal") {
		t.Errorf("KnownSecrets embedded '=' = %q, want x=S3cretVal and S3cretVal", ks)
	}
	if got := ErrorTextKnown("bad x=S3cretVal here", ks); strings.Contains(got, "S3cretVal") {
		t.Errorf("ErrorTextKnown leaked: %q", got)
	}
	ks = KnownSecrets("/share%2FA;S3cretVal", "", nil, nil)
	if !has(ks, "S3cretVal") {
		t.Errorf("KnownSecrets ';' piece = %q", ks)
	}
	if got := ErrorTextKnown("bad S3cretVal here", ks); strings.Contains(got, "S3cretVal") {
		t.Errorf("ErrorTextKnown leaked: %q", got)
	}
}

// Mutation pin: a later segment holding an encoded separator plus an invalid escape is collected in its decoded form too.
func TestHygF4F1_KnownDecodedFormOfLaterSegment(t *testing.T) {
	ks := KnownSecrets("/share%2FA/S3cret%2FVal%ZZ", "", nil, nil)
	ok := false
	for _, k := range ks {
		if k == "S3cret/Val%ZZ" {
			ok = true
		}
	}
	if !ok {
		t.Errorf("KnownSecrets = %q, want decoded S3cret/Val%%ZZ", ks)
	}
}

// HYGF4-F1 round 1: an invalid escape between the key name and the separator (or in front of the secret segment).
var f1R1Shapes = []string{
	"/share%ZZ%2FS3cretVal/more", "/share%2%2FS3cretVal/more", "/share%%2FS3cretVal/more", "/share%G%2FS3cretVal",
	"/token%E%2FS3cretVal", "/share%ZZ/S3cretVal", "/sh%61re%ZZ/S3cretVal", "/share%ZZ%5CS3cretVal/more",
	"/share%2525252FS3cretVal%ZZ/more",
}

func TestHygF4F1R1_InvalidEscapeBeforeSeparator(t *testing.T) {
	for _, in := range f1R1Shapes {
		if got := SecretPath(in); strings.Contains(got, "S3cretVal") || strings.Contains(got, "more") || !strings.Contains(got, ":redacted") {
			t.Errorf("SecretPath(%q) = %q", in, got)
		}
		if got := Path(in, ""); strings.Contains(got, "S3cretVal") {
			t.Errorf("Path(%q) = %q", in, got)
		}
		if got := Text("lookup " + in + " failed"); strings.Contains(got, "S3cretVal") {
			t.Errorf("Text(%q) = %q", in, got)
		}
		ks := KnownSecrets(in, "", nil, nil)
		found := false
		for _, k := range ks {
			if strings.Contains(k, "S3cretVal") {
				found = true
			}
		}
		if !found {
			t.Errorf("KnownSecrets(%q) = %q", in, ks)
		}
		if got := ErrorTextKnown("open "+in+": denied", ks); strings.Contains(got, "S3cretVal") {
			t.Errorf("ErrorTextKnown(%q) = %q", in, got)
		}
		if got := PathKnown(in, "", ks); strings.Contains(got, "S3cretVal") {
			t.Errorf("PathKnown(%q) = %q", in, got)
		}
	}
}

func TestHygF4F1R1_BenignUnchanged(t *testing.T) {
	for _, in := range []string{"/other%ZZ%2FS3cretVal", "/sharing%ZZ/x", "/api/v1/media/42%ZZ/x", "/files/50%25/x"} {
		if got := SecretPath(in); got != in {
			t.Errorf("SecretPath(%q) = %q, want unchanged", in, got)
		}
	}
}

// Reviewer mutant A3: every valid escape of a segment is decoded in each round.
func TestHygF4F1R1_AllEscapesDecodedPerRound(t *testing.T) {
	ks := KnownSecrets("/share%2FA/S3cret%2FVal%41%ZZ", "", nil, nil)
	if got := ErrorTextKnown("bad S3cret/ValA%ZZ x", ks); strings.Contains(got, "S3cret/ValA") {
		t.Errorf("ErrorTextKnown leaked: %q (ks=%q)", got, ks)
	}
}
