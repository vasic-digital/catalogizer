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

// HYGF4E-F1: header-name order is sorted. 18 credential header values across 3 header names and NO route params, against a full path
// pool (reserve 16): the first 16 in sorted header-name order (Authorization x6, Cookie x6, X-Api-Key x4) are kept and the rest are not.
// Fails when `sort.Strings(hnames)` is removed (map order is random, so the kept set varies).
func TestHygF4E_F1_HeaderNameOrderDeterministic(t *testing.T) {
	hdr := http.Header{}
	for _, n := range []string{"Authorization", "Cookie", "X-Api-Key"} {
		for i := 0; i < 6; i++ {
			hdr.Add(n, fmt.Sprintf("%sVal%02dQQ", n[:3], i))
		}
	}
	path := hygf4cPieces("/share", 70)
	var want []string
	for _, n := range []string{"Authorization", "Cookie", "X-Api-Key"} {
		for i := 0; i < 6; i++ {
			want = append(want, fmt.Sprintf("%sVal%02dQQ", n[:3], i))
		}
	}
	for n := 0; n < 200; n++ {
		got := map[string]bool{}
		for _, k := range KnownSecrets(path, "", hdr, nil) {
			got[k] = true
		}
		for i, w := range want {
			if i < 16 && !got[w] {
				t.Fatalf("run %d: sorted-first credential %s (index %d) not kept", n, w, i)
			}
			if i >= 16 && got[w] {
				t.Fatalf("run %d: credential %s (index %d) kept beyond the 16-slot reserve", n, w, i)
			}
		}
	}
}

// HYGF4E-F2: with full pools (70 path pieces, 20 credentials) the split is exactly 48 path + 16 credential = 64 (maxPathKnownSecrets 48).
func TestHygF4E_F2_ExactPathCredentialSplit(t *testing.T) {
	var q string
	for i := 0; i < 20; i++ {
		q += fmt.Sprintf("&token=CredSplit%02dXX", i)
	}
	ks := KnownSecrets(hygf4cPieces("/share", 70), q[1:], nil, nil)
	if len(ks) != 64 {
		t.Fatalf("total %d want 64", len(ks))
	}
	if p, c := hygf4cCount(ks, "Piece"), hygf4cCount(ks, "CredSplit"); p != 48 || c != 16 {
		t.Fatalf("split path=%d cred=%d want 48/16", p, c)
	}
}
