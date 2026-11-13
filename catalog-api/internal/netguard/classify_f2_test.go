package netguard

import (
	"context"
	"net"
	"testing"
)

// Round-2 review (CAT-SECQ1-B): ClassifyHost edge cases and the metadata singletons for every client class.

func fixedLookup(t *testing.T, ips []net.IP, err error) {
	t.Helper()
	old := lookupIPAddr
	t.Cleanup(func() { lookupIPAddr = old })
	lookupIPAddr = func(ctx context.Context, host string) ([]net.IPAddr, error) {
		var out []net.IPAddr
		for _, ip := range ips {
			out = append(out, net.IPAddr{IP: ip})
		}
		return out, err
	}
}

// F2 / M8: a name whose answers are one private and one never-class address is not approvable (the never address wins
// over the private one that came first).
func TestClassifyHost_PrivateThenNeverIsRefused(t *testing.T) {
	envClean(t)
	fixedLookup(t, []net.IP{net.ParseIP("10.1.2.3"), net.ParseIP("127.0.0.1")}, nil)
	if c, err := ClassifyHost(context.Background(), "mixed.test"); err == nil || c != ClassNever {
		t.Fatalf("private then loopback: got %s, %v; want error", c, err)
	}
	// order independence
	fixedLookup(t, []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("10.1.2.3")}, nil)
	if c, err := ClassifyHost(context.Background(), "mixed.test"); err == nil || c != ClassNever {
		t.Fatalf("loopback then private: got %s, %v; want error", c, err)
	}
	// control: private only is approvable as private
	fixedLookup(t, []net.IP{net.ParseIP("10.1.2.3")}, nil)
	if c, err := ClassifyHost(context.Background(), "lan.test"); err != nil || c != ClassPrivate {
		t.Fatalf("control: %s, %v", c, err)
	}
}

// F2 / M7: a resolver that answers with no addresses and no error is an error, never public by default.
func TestClassifyHost_EmptyAnswerIsAnError(t *testing.T) {
	envClean(t)
	fixedLookup(t, nil, nil)
	if c, err := ClassifyHost(context.Background(), "empty.test"); err == nil || c != ClassNever {
		t.Fatalf("empty answer: got %s, %v; want error", c, err)
	}
}

// F4 (operator decision 2026-11-13): the three metadata singletons are refused by EVERY client class, before the
// allow-list, and classification agrees with the dial. Teredo stays an operator-class-only refusal.
func TestMetadataSingletonsRefusedForEveryClass(t *testing.T) {
	envClean(t)
	t.Setenv(AllowCIDRsEnv, "fd00:ec2::/32,fd20:ce::/32,100.64.0.0/10")
	for _, s := range []string{"fd00:ec2::254", "fd20:ce::254", "100.100.100.200", "::ffff:100.100.100.200", "64:ff9b::6464:64c8"} {
		ip := net.ParseIP(s)
		if !IsBlockedIP(ip) || !IsBlockedIPStrict(ip) || !IsBlockedOperatorIP(ip) {
			t.Errorf("%s: sync=%v strict=%v operator=%v, all must refuse", s, IsBlockedIP(ip), IsBlockedIPStrict(ip), IsBlockedOperatorIP(ip))
		}
		if ClassOf(ip) != ClassNever {
			t.Errorf("%s: ClassOf must be never", s)
		}
		if c, err := ClassifyHost(context.Background(), s); err == nil || c != ClassNever {
			t.Errorf("%s: ClassifyHost must refuse, got %s, %v", s, c, err)
		}
		// the dial (sync Transport path) refuses the literal too
		if _, err := resolve(context.Background(), s, true); err == nil {
			t.Errorf("%s: resolve with the allow-list honoured must refuse", s)
		}
	}
	// neighbours stay ordinary space for the allow-list classes (only the single address is refused)
	for _, s := range []string{"fd00:ec2::255", "fd20:ce::253", "100.100.100.201", "100.64.0.9"} {
		if IsBlockedIP(net.ParseIP(s)) {
			t.Errorf("%s: a neighbour of a singleton, covered by the allow-list, must not be refused by the sync class", s)
		}
	}
	// Teredo: operator class only
	teredo := net.ParseIP("2001:0:4136:e378:8000:63bf:3fff:fdd2")
	if !IsBlockedOperatorIP(teredo) {
		t.Error("operator class must refuse Teredo")
	}
	if IsBlockedIP(teredo) {
		t.Error("Teredo is not a shared singleton: the sync class does not refuse it")
	}
}
