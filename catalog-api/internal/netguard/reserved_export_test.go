package netguard

import (
	"net"
	"testing"
)

func TestReservedNets_CopySafeAndComplete(t *testing.T) {
	want := []string{"198.18.0.0/15", "192.0.0.0/24", "fec0::/10", "64:ff9b:1::/48", "168.63.129.16/32", "2001::/32", "0.0.0.0/8", "240.0.0.0/4"}
	got := map[string]bool{}
	for _, n := range ReservedNets() {
		got[n.String()] = true
	}
	for _, w := range want {
		if !got[w] {
			t.Fatalf("ReservedNets missing %s", w)
		}
	}
	a := ReservedNets()
	for _, n := range a {
		for i := range n.Mask {
			n.Mask[i] = 0
		}
	}
	if !IsReservedRange(net.ParseIP("198.18.0.1")) || !IsReservedRange(net.ParseIP("::ffff:240.0.0.1")) {
		t.Fatal("mutating the returned list weakened the guard")
	}
	if IsReservedRange(net.ParseIP("192.168.1.5")) || IsReservedRange(net.ParseIP("8.8.8.8")) {
		t.Fatal("LAN / public address reported reserved")
	}
	if !IsReservedRange(nil) {
		t.Fatal("nil must fail closed")
	}
}
