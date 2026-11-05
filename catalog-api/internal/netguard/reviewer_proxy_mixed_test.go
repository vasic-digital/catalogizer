package netguard

import (
	"context"
	"net"
	"testing"
)

// Reviewer mutation/probe: with an environment proxy the proxy resolves the name itself, so the filter pin does not
// apply. A mixed answer (permitted + metadata) must not be forwarded.
func TestReviewer_ProxyMixedAnswerMustNotForward(t *testing.T) {
	t.Setenv(AllowCIDRsEnv, "")
	t.Setenv(DenyPrivateEnv, "")
	p, hits := realProxy(t)
	withProxy(t, p)
	old := lookupIPAddr
	t.Cleanup(func() { lookupIPAddr = old })
	lookupIPAddr = func(ctx context.Context, host string) ([]net.IPAddr, error) {
		return []net.IPAddr{{IP: net.ParseIP("192.168.1.20")}, {IP: net.ParseIP("169.254.169.254")}}, nil
	}
	err := proxiedGet("http://mixed.rebind.test/")
	t.Logf("err=%v hits=%d", err, hits.Load())
	if hits.Load() != 0 {
		t.Fatalf("mixed answer containing metadata was forwarded to the proxy (%d request(s)); the proxy may connect to 169.254.169.254", hits.Load())
	}
}
