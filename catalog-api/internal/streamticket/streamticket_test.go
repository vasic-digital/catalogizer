package streamticket

import (
	"testing"
	"time"
)

func TestMintVerify(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	s := New("secret-one").WithClock(func() time.Time { return now })
	tk, exp := s.Mint(7, "3", "ann", "sess")
	if exp != now.Add(TTL) {
		t.Fatalf("expiry %v", exp)
	}
	c, err := s.Verify(tk, 7)
	if err != nil || c.UserID != "3" || c.SessionID != "sess" {
		t.Fatalf("valid ticket refused: %v %+v", err, c)
	}
	if _, err := s.Verify(tk, 8); err == nil {
		t.Fatal("ticket for 7 accepted for 8")
	}
	late := s.WithClock(func() time.Time { return now.Add(TTL) })
	if _, err := late.Verify(tk, 7); err == nil {
		t.Fatal("expired ticket accepted")
	}
	if _, err := New("secret-two").WithClock(func() time.Time { return now }).Verify(tk, 7); err == nil {
		t.Fatal("ticket from another key accepted")
	}
	for _, bad := range []string{"", "x", "v1..", "v1.a.b", tk + "x", "v2" + tk[2:]} {
		if _, err := s.Verify(bad, 7); err == nil {
			t.Fatalf("garbage %q accepted", bad)
		}
	}
	if New("") != nil {
		t.Fatal("empty secret must disable tickets")
	}
}
