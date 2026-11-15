package logredact

import (
	"fmt"
	"net/http"
	"reflect"
	"testing"
)

// HYGF4D-M1: the `out[:pathCap:pathCap]` cap keeps the merge appends from overwriting the overflow that shares the backing array.
// 63 path pieces + 3 credentials: reserve 3, path cap 61, overflow {Piece061ZZ, Piece062ZZ} (one slot fits); without the cap the credentials
// overwrite the overflow and Piece061ZZ is lost.
func TestHygF4D_M1_PathOverflowNotClobberedByCredentialAppend(t *testing.T) {
	ks := KnownSecrets(hygf4cPieces("/share", 63), "token=Piece000ZZ&token=NewCredA1&token=NewCredB2", nil, nil)
	seen := map[string]bool{}
	for _, k := range ks {
		seen[k] = true
	}
	for _, want := range []string{"Piece061ZZ", "NewCredA1", "NewCredB2"} {
		if !seen[want] {
			t.Errorf("%s not collected (overflow clobbered?); got %d values", want, len(ks))
		}
	}
}

// HYGF4D-M2: a credential differing only in case from a path value is a distinct secret and must also be collected.
func TestHygF4D_M2_CredentialDifferingOnlyInCaseFromPathValue(t *testing.T) {
	ks := KnownSecrets("/share%2FS3cretOne", "token=s3cretone", nil, nil)
	var a, b bool
	for _, k := range ks {
		a = a || k == "S3cretOne"
		b = b || k == "s3cretone"
	}
	if !a || !b {
		t.Errorf("both case variants must be collected, got %q", ks)
	}
}

// HYGF4D-M3: which credentials survive when they exceed the reserve against a full path pool is deterministic (sorted keys).
func TestHygF4D_M3_CredentialSelectionDeterministic(t *testing.T) {
	params := map[string]string{}
	hdr := http.Header{}
	for i := 0; i < 20; i++ {
		name := fmt.Sprintf("token%02d", i)
		if !SensitiveKey(name) {
			t.Fatalf("test premise: %s must be sensitive", name)
		}
		params[name] = fmt.Sprintf("ParamCred%02dXX", i)
	}
	for i := 0; i < 20; i++ {
		hdr.Set(fmt.Sprintf("X-Test-%02d", i), "ignored") // non-credential headers are not collected
	}
	hdr.Set("X-Api-Key", "HeaderKeyAAA")
	hdr.Set("Authorization", "Bearer HeaderBearerBBB")
	path := hygf4cPieces("/share", 70)
	first := KnownSecrets(path, "", hdr, params)
	// reserve is 16 (maxKnownSecrets-maxPathKnownSecrets): the first 16 credentials in sorted-key order are kept.
	for i := 0; i < 16; i++ {
		want := fmt.Sprintf("ParamCred%02dXX", i)
		found := false
		for _, k := range first {
			found = found || k == want
		}
		if !found {
			t.Errorf("%s (sorted-first) not kept", want)
		}
	}
	for n := 0; n < 100; n++ {
		if got := KnownSecrets(path, "", hdr, params); !reflect.DeepEqual(got, first) {
			t.Fatalf("run %d differs from first: nondeterministic selection", n)
		}
	}
}
