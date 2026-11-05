package netguard

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"sync/atomic"
	"testing"
	"time"
)

// SECQ-1 follow-ups (F3, F4, F5).

func fixedResolver(t *testing.T, ips ...string) {
	t.Helper()
	old := lookupIPAddr
	t.Cleanup(func() { lookupIPAddr = old })
	lookupIPAddr = func(ctx context.Context, host string) ([]net.IPAddr, error) {
		var out []net.IPAddr
		for _, s := range ips {
			out = append(out, net.IPAddr{IP: net.ParseIP(s)})
		}
		return out, nil
	}
}

func countingListener(t *testing.T) (net.Listener, *atomic.Int32) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skip(err)
	}
	t.Cleanup(func() { ln.Close() })
	var n atomic.Int32
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			n.Add(1)
			c.Close()
		}
	}()
	return ln, &n
}

// F5: OperatorTransport itself (not only OperatorDialContext) must apply the dial check.
func TestSECQ1FU_F5_OperatorTransportRefusesUnallowlistedLoopback(t *testing.T) {
	t.Setenv(AllowCIDRsEnv, "")
	t.Setenv(DenyPrivateEnv, "")
	old := proxyFunc
	proxyFunc = func(*http.Request) (*url.URL, error) { return nil, nil }
	t.Cleanup(func() { proxyFunc = old })
	ln, accepted := countingListener(t)
	cl := &http.Client{Transport: OperatorTransport(), Timeout: 5 * time.Second}
	resp, err := cl.Get("http://" + ln.Addr().String() + "/")
	if err == nil {
		resp.Body.Close()
		t.Fatal("OperatorTransport must refuse an unallowlisted loopback listener")
	}
	if !errors.Is(err, ErrBlocked) {
		t.Fatalf("want ErrBlocked, got %v", err)
	}
	time.Sleep(50 * time.Millisecond)
	if accepted.Load() != 0 {
		t.Fatal("the listener saw a connection although the transport should have refused it")
	}
}

// F5: the redirect-host check itself, without a transport whose dial check would mask a missing check.
func TestSECQ1FU_F5_RedirectHostCheckAloneRefuses(t *testing.T) {
	t.Setenv(AllowCIDRsEnv, "")
	t.Setenv(DenyPrivateEnv, "")
	for _, raw := range []string{"http://127.0.0.1:1/x", "http://169.254.169.254/", "http://[fe80::1]/"} {
		req, _ := http.NewRequest("GET", raw, nil)
		if err := OperatorCheckRedirect(req, nil); !errors.Is(err, ErrBlocked) {
			t.Errorf("%s: redirect host check must refuse, got %v", raw, err)
		}
	}
	req, _ := http.NewRequest("GET", "http://192.168.1.20/x", nil)
	if err := OperatorCheckRedirect(req, nil); err != nil {
		t.Errorf("RFC1918 hop must pass the host check: %v", err)
	}
}

// F3: GCP IPv6-only metadata server (documented by Google).
func TestSECQ1FU_F3_GCPIPv6MetadataBlocked(t *testing.T) {
	t.Setenv(AllowCIDRsEnv, "fd20:ce::254/128")
	t.Setenv(DenyPrivateEnv, "")
	if !IsBlockedOperatorIP(net.ParseIP("fd20:ce::254")) {
		t.Fatal("fd20:ce::254 must be refused even when allowlisted")
	}
	if IsBlockedOperatorIP(net.ParseIP("fd20:ce::255")) {
		t.Fatal("only the single metadata address is refused, not the whole /32")
	}
}

// F4: a mixed answer is filtered; blocked addresses are dropped, the rest dialled pinned.
func TestSECQ1FU_F4_MixedAnswerIsFiltered(t *testing.T) {
	t.Setenv(AllowCIDRsEnv, "")
	t.Setenv(DenyPrivateEnv, "")
	fixedResolver(t, "192.168.1.20", "fe80::1")
	ips, err := resolveOperator(context.Background(), "nas.local.test")
	if err != nil {
		t.Fatalf("mixed 192.168.x + fe80:: answer must be filtered, not refused: %v", err)
	}
	if len(ips) != 1 || !ips[0].Equal(net.ParseIP("192.168.1.20")) {
		t.Fatalf("want only 192.168.1.20, got %v", ips)
	}
}

func TestSECQ1FU_F4_BlockedOnlyAnswersStillRefuse(t *testing.T) {
	t.Setenv(AllowCIDRsEnv, "")
	t.Setenv(DenyPrivateEnv, "")
	for _, set := range [][]string{{"127.0.0.1"}, {"169.254.169.254"}, {"127.0.0.1", "fe80::1", "169.254.169.254"}} {
		fixedResolver(t, set...)
		if _, err := resolveOperator(context.Background(), "evil.test"); !errors.Is(err, ErrBlocked) {
			t.Errorf("%v: want ErrBlocked, got %v", set, err)
		}
	}
}

// F4 at dial level: the dial goes to the allowed address only and never to the dropped one.
func TestSECQ1FU_F4_DialUsesOnlyAllowedAddresses(t *testing.T) {
	ln, accepted := countingListener(t)
	_, port, _ := net.SplitHostPort(ln.Addr().String())
	t.Setenv(AllowCIDRsEnv, "127.0.0.1/32")
	t.Setenv(DenyPrivateEnv, "")
	fixedResolver(t, "169.254.169.254", "127.0.0.1")
	c, err := OperatorDialContext(context.Background(), "tcp", net.JoinHostPort("mixed.test", port))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	if got := c.RemoteAddr().(*net.TCPAddr).IP.String(); got != "127.0.0.1" {
		t.Fatalf("connected to %s", got)
	}
	c.Close()
	time.Sleep(50 * time.Millisecond)
	if accepted.Load() != 1 {
		t.Fatalf("listener saw %d connections", accepted.Load())
	}
}
