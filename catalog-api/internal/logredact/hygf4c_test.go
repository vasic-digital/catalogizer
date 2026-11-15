package logredact

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func hygf4cPieces(path string, n int) string {
	var b strings.Builder
	b.WriteString(path)
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "%%2FPiece%03dZZ", i)
	}
	return b.String()
}

func hygf4cCount(ks []string, prefix string) (n int) {
	for _, k := range ks {
		if strings.HasPrefix(k, prefix) {
			n++
		}
	}
	return n
}

// HYGF4C-M1: a credential equal to a path value must not leave its reserved slot empty: all slots up to the cap are used.
func TestHygF4C_M1_CredentialEqualToPathValueDoesNotWasteSlot(t *testing.T) {
	ks := KnownSecrets(hygf4cPieces("/share", 70), "token=Piece000ZZ", nil, nil)
	if len(ks) != maxKnownSecrets {
		t.Fatalf("collected %d, want %d (reserved slot wasted)", len(ks), maxKnownSecrets)
	}
	if got := hygf4cCount(ks, "Piece"); got != maxKnownSecrets {
		t.Errorf("collected %d pieces, want all %d up to the cap", got, maxKnownSecrets)
	}
	seen := map[string]bool{}
	for _, k := range ks {
		if seen[k] {
			t.Errorf("duplicate %q", k)
		}
		seen[k] = true
	}
	if !seen["Piece063ZZ"] {
		t.Errorf("Piece063ZZ not collected, would be exposed in logs")
	}
}

// HYGF4C-M2: path values differing only in case are distinct secrets and both are collected.
func TestHygF4C_M2_CaseDifferingValuesBothCollected(t *testing.T) {
	ks := KnownSecrets("/share%2FS3cretOne%2Fs3cretone", "", nil, nil)
	var a, b bool
	for _, k := range ks {
		a = a || k == "S3cretOne"
		b = b || k == "s3cretone"
	}
	if !a || !b {
		t.Errorf("both case variants must be collected, got %q", ks)
	}
}

// HYGF4C-M3: 64 credentials cannot starve the path: at least maxPathKnownSecrets (48) path slots remain.
func TestHygF4C_M3_PathKeepsAtLeast48SlotsAgainst64Credentials(t *testing.T) {
	var q []string
	for i := 0; i < 64; i++ {
		q = append(q, fmt.Sprintf("token=Cred%03dYY", i))
	}
	ks := KnownSecrets(hygf4cPieces("/share", 100), strings.Join(q, "&"), nil, nil)
	if len(ks) != maxKnownSecrets {
		t.Errorf("len %d, want %d", len(ks), maxKnownSecrets)
	}
	if got := hygf4cCount(ks, "Piece"); got < maxPathKnownSecrets {
		t.Errorf("path got %d slots, want >= %d", got, maxPathKnownSecrets)
	}
	if got := hygf4cCount(ks, "Cred"); got < maxKnownSecrets-maxPathKnownSecrets {
		t.Errorf("credentials got %d slots, want >= %d", got, maxKnownSecrets-maxPathKnownSecrets)
	}
}

// HYGF4C-M4: the merge never emits a value twice, whether the credential duplicates a path value or another credential.
func TestHygF4C_M4_MergeDedupe(t *testing.T) {
	h := http.Header{}
	h.Set("Authorization", "Bearer Piece005ZZ")
	cases := map[string]string{
		"credential equals path value":    "token=Piece005ZZ&token=Piece005ZZ",
		"credential repeated":             "token=CredRepeatYY&token=CredRepeatYY&secret=CredRepeatYY",
		"credential equal to later piece": "token=Piece040ZZ",
	}
	for name, q := range cases {
		ks := KnownSecrets(hygf4cPieces("/share", 30), q, h, nil)
		seen := map[string]bool{}
		for _, k := range ks {
			if seen[k] {
				t.Errorf("%s: duplicate %q in %q", name, k, ks)
			}
			seen[k] = true
		}
	}
	// 20 identical credentials must not reserve 16 slots: 70 pieces + 1 credential still fill the cap with 63 pieces + it
	var q []string
	for i := 0; i < 20; i++ {
		q = append(q, "token=CredRepeatYY")
	}
	ks := KnownSecrets(hygf4cPieces("/share", 70), strings.Join(q, "&"), nil, nil)
	if got := hygf4cCount(ks, "Piece"); got != maxKnownSecrets-1 {
		t.Errorf("repeated credential wasted slots: %d pieces, want %d", got, maxKnownSecrets-1)
	}
}
