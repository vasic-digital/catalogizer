package netguard

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// Review round 1 of SECQ-1 (F1-F3, F5-F7).

// realProxy starts a real HTTP proxy listener on 127.0.0.1 (a local proxy is the regression case of F1) and returns how many
// requests reached it. Every request is answered 200.
func realProxy(t *testing.T) (*url.URL, *atomic.Int32) {
	t.Helper()
	var n atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	return u, &n
}

func withProxy(t *testing.T, p *url.URL) {
	t.Helper()
	old := proxyFunc
	proxyFunc = func(*http.Request) (*url.URL, error) { return p, nil }
	t.Cleanup(func() { proxyFunc = old })
}

func proxiedGet(raw string) error {
	cl := &http.Client{Transport: OperatorTransport(), CheckRedirect: OperatorCheckRedirect, Timeout: 5 * time.Second}
	resp, err := cl.Get(raw)
	if err == nil {
		resp.Body.Close()
	}
	return err
}

func TestF1_ProxyCheckedDestinationAndLocalProxyAllowed(t *testing.T) {
	t.Setenv(AllowCIDRsEnv, "")
	t.Setenv(DenyPrivateEnv, "")
	p, hits := realProxy(t)
	withProxy(t, p)

	// positive control + local-proxy regression: a LAN destination goes through a proxy that itself listens on loopback
	if err := proxiedGet("http://192.168.1.20/dav"); err != nil {
		t.Fatalf("LAN destination through a loopback proxy must work: %v", err)
	}
	if hits.Load() != 1 {
		t.Fatalf("proxy should have seen the request, saw %d", hits.Load())
	}
	before := hits.Load()

	oldLookup := lookupIPAddr
	defer func() { lookupIPAddr = oldLookup }()
	lookupIPAddr = func(ctx context.Context, host string) ([]net.IPAddr, error) {
		if host == "meta.rebind.test" {
			return []net.IPAddr{{IP: net.ParseIP("169.254.169.254")}}, nil
		}
		return nil, &net.DNSError{Err: "no such host", Name: host, IsNotFound: true}
	}
	for _, u := range []string{
		"http://metadata.google.internal.invalid/", // local DNS fails: must not be forwarded to the proxy
		"http://meta.rebind.test/",                 // resolves to metadata
		"http://0177.0.0.1/", "http://2130706433/", "http://127.1/", "http://0x7f.1/",
		"http://127.0.0.1/", "http://169.254.169.254/", "http://[::ffff:7f00:1]/",
	} {
		if err := proxiedGet(u); err == nil {
			t.Errorf("%s must be refused when a proxy is in use", u)
		}
	}
	if hits.Load() != before {
		t.Fatalf("refused destinations reached the proxy: %d request(s)", hits.Load()-before)
	}
}

func TestF2_DialIsPinnedToTheCheckedIP(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skip(err)
	}
	defer ln.Close()
	var accepted atomic.Int32
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			accepted.Add(1)
			c.Close()
		}
	}()
	_, port, _ := net.SplitHostPort(ln.Addr().String())
	t.Setenv(AllowCIDRsEnv, "127.0.0.1/32")
	old := lookupIPAddr
	defer func() { lookupIPAddr = old }()
	var calls atomic.Int32
	lookupIPAddr = func(ctx context.Context, host string) ([]net.IPAddr, error) {
		// first answer: the allowed stub IP; every later answer: metadata (a rebinding resolver)
		if calls.Add(1) == 1 {
			return []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}}, nil
		}
		return []net.IPAddr{{IP: net.ParseIP("169.254.169.254")}}, nil
	}
	c, err := OperatorDialContext(context.Background(), "tcp", net.JoinHostPort("nas.pin.test", port))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	if c.RemoteAddr().(*net.TCPAddr).IP.String() != "127.0.0.1" {
		t.Fatalf("connected to %s, want the checked IP 127.0.0.1", c.RemoteAddr())
	}
	c.Close()
	if calls.Load() != 1 {
		t.Fatalf("name resolved %d times in one dial; the answer that was checked must be the one dialled", calls.Load())
	}
	time.Sleep(50 * time.Millisecond)
	if accepted.Load() != 1 {
		t.Fatalf("listener saw %d connections, want 1", accepted.Load())
	}
	// a second dial resolves afresh and gets the rebinding answer: refused
	if c, err := OperatorDialContext(context.Background(), "tcp", net.JoinHostPort("nas.pin.test", port)); err == nil {
		c.Close()
		t.Fatal("second resolution with a metadata answer must be refused")
	} else if !errors.Is(err, ErrBlocked) {
		t.Fatalf("want ErrBlocked, got %v", err)
	}
}

func TestF3F5_AdditionalForms(t *testing.T) {
	t.Setenv(AllowCIDRsEnv, "")
	t.Setenv(DenyPrivateEnv, "")
	for _, s := range []string{
		"::ffff:0:7f00:1", "::ffff:0:a9fe:a9fe", // SIIT-translated loopback / metadata (::ffff:0:a.b.c.d)
		"2001:0:4136:e378:8000:63bf:3fff:fdd2", "2001::1", // Teredo
	} {
		if !IsBlockedOperatorIP(net.ParseIP(s)) {
			t.Errorf("%s must be blocked", s)
		}
	}
	if IsBlockedOperatorIP(net.ParseIP("::ffff:0:c0a8:114")) != false {
		t.Error("SIIT form of a RFC1918 NAS must follow the embedded IPv4 (allowed by default)")
	}
	// F5: ::1 and :: are not IPv4-compatible 0.0.0.1 / 0.0.0.0
	if embeddedV4(net.ParseIP("::1")) != nil || embeddedV4(net.ParseIP("::")) != nil {
		t.Error("::1 and :: must not be unwrapped as IPv4-compatible addresses")
	}
	t.Setenv(AllowCIDRsEnv, "::1/128")
	if IsBlockedOperatorIP(net.ParseIP("::1")) {
		t.Error("::1/128 in the allowlist must admit ::1")
	}
	if !IsBlockedOperatorIP(net.ParseIP("::2")) && false {
		t.Skip()
	}
}

func TestF6_NonCanonicalNumericHostsRefused(t *testing.T) {
	t.Setenv(AllowCIDRsEnv, "")
	old := lookupIPAddr
	defer func() { lookupIPAddr = old }()
	lookupIPAddr = func(ctx context.Context, host string) ([]net.IPAddr, error) {
		return []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}}, nil // a resolver that would happily answer
	}
	for _, h := range []string{"0177.0.0.1", "2130706433", "127.1", "0x7f.1", "0x7f000001", "1.2.3", "fe80::1%eth0"} {
		if _, err := CheckOperatorURL(context.Background(), "http://["+h+"]/", "http"); err == nil && strings.Contains(h, ":") {
			t.Errorf("%q must be refused", h)
		}
		if strings.Contains(h, ":") {
			continue
		}
		if _, err := CheckOperatorURL(context.Background(), "http://"+h+"/", "http"); err == nil {
			t.Errorf("host %q must be refused", h)
		}
		if c, err := OperatorDialContext(context.Background(), "tcp", h+":80"); err == nil {
			c.Close()
			t.Errorf("dial to %q must be refused", h)
		} else if !errors.Is(err, ErrBlocked) {
			t.Errorf("dial to %q: want ErrBlocked, got %v", h, err)
		}
	}
	// real names (even with digits) still resolve
	if _, err := CheckOperatorURL(context.Background(), "http://nas1.example.com/", "http"); err != nil {
		t.Errorf("ordinary name refused: %v", err)
	}
	if _, err := CheckOperatorURL(context.Background(), "http://3com.example/", "http"); err != nil {
		t.Errorf("name starting with a digit refused: %v", err)
	}
}

func TestF7_ErrorTextDoesNotEchoResolvedAddress(t *testing.T) {
	old := lookupIPAddr
	defer func() { lookupIPAddr = old }()
	lookupIPAddr = func(ctx context.Context, host string) ([]net.IPAddr, error) {
		return []net.IPAddr{{IP: net.ParseIP("169.254.169.254")}}, nil
	}
	_, err := CheckOperatorURL(context.Background(), "http://probe.test/", "http")
	if err == nil || strings.Contains(err.Error(), "169.254") {
		t.Fatalf("error must refuse without echoing the resolved address, got %v", err)
	}
}
