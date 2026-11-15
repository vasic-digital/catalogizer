package logredact

import (
	"net/http"
	"strings"
	"testing"
)

const bearerTok = "BearerTokVal99xyz"

func bearerHeader() http.Header {
	h := http.Header{}
	h.Set("Authorization", "Bearer "+bearerTok)
	return h
}

// HYGF4B-M-B: a crafted key segment must never exhaust the known-secret cap so that a header credential is not collected.
func TestHygF4B_CapExhaustionStillCollectsBearer(t *testing.T) {
	var embedded strings.Builder
	embedded.WriteString("/share")
	for i := 0; i < 200; i++ {
		embedded.WriteString("%2Fpiece" + string(rune('a'+i%26)) + string(rune('a'+i/26)) + "zz")
	}
	// ambiguous segment: deeply encoded separators feed the 16-round loop
	var amb strings.Builder
	amb.WriteString("/share%25252525252525252525252525252525")
	for i := 0; i < 200; i++ {
		amb.WriteString("2Fpiece" + string(rune('a'+i%26)) + string(rune('a'+i/26)) + "zz%25252525252525252525252525252525")
	}
	for name, path := range map[string]string{"embedded": embedded.String(), "ambiguous": amb.String(), "many-segments": "/share/" + strings.Repeat("seg0123/", 200)} {
		ks := KnownSecrets(path, "", bearerHeader(), nil)
		if len(ks) > maxKnownSecrets {
			t.Errorf("%s: %d secrets > cap", name, len(ks))
		}
		got := ErrorTextKnown("lookup "+bearerTok+" failed", ks)
		if strings.Contains(got, bearerTok) {
			t.Errorf("%s: bearer token leaked: %q", name, got)
		}
	}
}

func TestHygF4B_NoDuplicateKnownSecrets(t *testing.T) {
	ks := KnownSecrets("/share%2525252FS3cretOne%2525252FS3cretTwo", "", nil, nil)
	seen := map[string]bool{}
	for _, k := range ks {
		if seen[k] {
			t.Errorf("duplicate known secret %q in %q", k, ks)
		}
		seen[k] = true
	}
}

// HYGF4B-M-A: a key word (share) must not become a known secret.
func TestHygF4B_KeyWordNotCollected(t *testing.T) {
	ks := KnownSecrets("/x/share%ZZ/share/T2", "", nil, nil)
	for _, k := range ks {
		if k == "share" {
			t.Errorf("key name collected as secret: %q", ks)
		}
	}
	got := ErrorTextKnown("open /x/share%ZZ/share/T2: no", ks)
	if !strings.Contains(got, "/share/") {
		t.Errorf("key word rewritten: %q", got)
	}
}

// HYGF4B-M-D: killers for reviewer mutants B3 (only last piece collected) and B4 (backslash not a separator).
func TestHygF4B_KillerB3_AllPiecesCollected(t *testing.T) {
	ks := KnownSecrets("/share%2525252FS3cretOne%2525252FS3cretTwo", "", nil, nil)
	got := ErrorTextKnown("bad S3cretOne here and S3cretTwo there", ks)
	if strings.Contains(got, "S3cretOne") || strings.Contains(got, "S3cretTwo") {
		t.Errorf("piece leaked: %q", got)
	}
}

func TestHygF4B_KillerB4_BackslashSeparator(t *testing.T) {
	ks := KnownSecrets("/share%2525255CS3cretOne", "", nil, nil)
	got := ErrorTextKnown("bad S3cretOne here", ks)
	if strings.Contains(got, "S3cretOne") {
		t.Errorf("backslash piece leaked: %q", got)
	}
}

// HYGF4B-M-C: allocation is bounded for a large crafted segment.
func TestHygF4B_PctDecodeAllocationBounded(t *testing.T) {
	in := strings.Repeat("%2F", 1<<18) // 768 KiB
	allocs := testing.AllocsPerRun(3, func() { _ = lenientUnescape(in) })
	if allocs > 4 {
		t.Errorf("lenientUnescape allocs/run = %v, want <= 4", allocs)
	}
	var m1, m2 runtimeMem
	m1.read()
	_ = KnownSecrets("/share"+strings.Repeat("%25", 1<<18)+"2FS3cret", "", nil, nil)
	m2.read()
	if d := m2.total - m1.total; d > 40<<20 {
		t.Errorf("KnownSecrets allocated %d bytes for 768KiB input", d)
	}
}
