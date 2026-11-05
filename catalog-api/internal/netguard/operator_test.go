package netguard

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestOperatorClassification(t *testing.T) {
	t.Setenv(AllowCIDRsEnv, "")
	t.Setenv(DenyPrivateEnv, "")
	cases := []struct {
		ip      string
		blocked bool
		why     string
	}{
		// always refused
		{"127.0.0.1", true, "loopback"},
		{"127.5.5.5", true, "loopback /8"},
		{"::1", true, "loopback v6"},
		{"169.254.169.254", true, "cloud metadata"},
		{"169.254.1.1", true, "link-local"},
		{"fe80::1", true, "link-local v6"},
		{"fd00:ec2::254", true, "AWS IMDS IPv6 metadata (a ULA address, refused explicitly)"},
		{"100.100.100.200", true, "Alibaba metadata (inside CGNAT, refused explicitly)"},
		{"0.0.0.0", true, "unspecified"},
		{"::", true, "unspecified v6"},
		{"224.0.0.1", true, "multicast"},
		{"ff02::1", true, "multicast v6"},
		{"255.255.255.255", true, "broadcast"},
		{"::ffff:127.0.0.1", true, "v4-mapped loopback"},
		{"::ffff:169.254.169.254", true, "v4-mapped metadata"},
		{"64:ff9b::a9fe:a9fe", true, "NAT64 metadata"},
		{"64:ff9b::7f00:1", true, "NAT64 loopback"},
		{"2002:a9fe:a9fe::1", true, "6to4 metadata"},
		{"2002:7f00:1::1", true, "6to4 loopback"},
		{"::7f00:1", true, "v4-compatible loopback"},
		{"198.18.0.1", true, "benchmarking reserved"},
		{"240.0.0.1", true, "reserved 240/4"},
		// legitimate LAN NAS and public hosts stay reachable by default
		{"192.168.1.20", false, "Synology on RFC1918"},
		{"10.0.0.5", false, "RFC1918"},
		{"172.16.3.4", false, "RFC1918"},
		{"fd12:3456::1", false, "ULA"},
		{"::ffff:192.168.1.20", false, "v4-mapped RFC1918"},
		{"64:ff9b::c0a8:114", false, "NAT64 of a RFC1918 NAS"},
		{"100.100.1.1", false, "CGNAT (Tailscale) allowed by default"},
		{"77.88.21.119", false, "public (webdav.yandex.ru range)"},
		{"2a02:6b8::1", false, "public v6"},
	}
	for _, c := range cases {
		ip := net.ParseIP(c.ip)
		if ip == nil {
			t.Fatalf("bad test ip %q", c.ip)
		}
		if got := IsBlockedOperatorIP(ip); got != c.blocked {
			t.Errorf("%s (%s): blocked=%v, want %v", c.ip, c.why, got, c.blocked)
		}
	}
}

func TestOperatorStrictOptInDeniesPrivate(t *testing.T) {
	t.Setenv(DenyPrivateEnv, "1")
	t.Setenv(AllowCIDRsEnv, "")
	for _, s := range []string{"192.168.1.20", "10.0.0.5", "fd12:3456::1", "100.100.1.1", "::ffff:10.1.1.1"} {
		if !IsBlockedOperatorIP(net.ParseIP(s)) {
			t.Errorf("%s must be blocked with %s=1", s, DenyPrivateEnv)
		}
	}
	if IsBlockedOperatorIP(net.ParseIP("77.88.21.119")) {
		t.Error("public address must stay reachable in strict mode")
	}
	// the operator allowlist re-admits an exception even in strict mode
	t.Setenv(AllowCIDRsEnv, "192.168.1.0/24")
	if IsBlockedOperatorIP(net.ParseIP("192.168.1.20")) {
		t.Error("allowlisted NAS must be admitted in strict mode")
	}
	if !IsBlockedOperatorIP(net.ParseIP("192.168.2.20")) {
		t.Error("address outside the allowlist must stay blocked in strict mode")
	}
}

func TestOperatorAllowlistCannotReadmitMetadata(t *testing.T) {
	t.Setenv(AllowCIDRsEnv, "169.254.0.0/16,0.0.0.0/0")
	t.Setenv(DenyPrivateEnv, "")
	for _, s := range []string{"169.254.169.254", "fe80::1", "0.0.0.0", "224.0.0.1"} {
		if !IsBlockedOperatorIP(net.ParseIP(s)) {
			t.Errorf("%s must be refused whatever the allowlist says", s)
		}
	}
	// loopback is re-admitted only when named
	t.Setenv(AllowCIDRsEnv, "127.0.0.0/8")
	if IsBlockedOperatorIP(net.ParseIP("127.0.0.1")) {
		t.Error("allowlisted loopback must be admitted")
	}
}

func TestOperatorDialRefusesLoopbackByDefault(t *testing.T) {
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
	t.Setenv(AllowCIDRsEnv, "")
	t.Setenv(DenyPrivateEnv, "")
	c, err := OperatorDialContext(context.Background(), "tcp", ln.Addr().String())
	if err == nil {
		c.Close()
		t.Fatal("dial to loopback must be refused by default")
	}
	if !errors.Is(err, ErrBlocked) {
		t.Fatalf("want ErrBlocked, got %v", err)
	}
	time.Sleep(50 * time.Millisecond)
	if accepted.Load() != 0 {
		t.Fatal("the listener saw a connection although the dial was refused")
	}
	// positive control: allowlisted loopback connects through the same dialer
	t.Setenv(AllowCIDRsEnv, "127.0.0.1/32")
	c, err = OperatorDialContext(context.Background(), "tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("allowlisted loopback must connect: %v", err)
	}
	c.Close()
}

func TestOperatorDialChecksResolvedIPNotName(t *testing.T) {
	old := lookupIPAddr
	defer func() { lookupIPAddr = old }()
	var calls atomic.Int32
	lookupIPAddr = func(ctx context.Context, host string) ([]net.IPAddr, error) {
		calls.Add(1)
		// DNS rebinding shape: the name is harmless-looking but resolves to metadata
		return []net.IPAddr{{IP: net.ParseIP("169.254.169.254")}}, nil
	}
	t.Setenv(AllowCIDRsEnv, "")
	_, err := OperatorDialContext(context.Background(), "tcp", "nas.example.test:80")
	if !errors.Is(err, ErrBlocked) {
		t.Fatalf("name resolving to metadata must be refused, got %v", err)
	}
	// mixed answer: one good, one bad -> refused
	lookupIPAddr = func(ctx context.Context, host string) ([]net.IPAddr, error) {
		return []net.IPAddr{{IP: net.ParseIP("192.168.1.20")}, {IP: net.ParseIP("127.0.0.1")}}, nil
	}
	_, err = OperatorDialContext(context.Background(), "tcp", "nas.example.test:80")
	if !errors.Is(err, ErrBlocked) {
		t.Fatalf("mixed answer containing loopback must be refused, got %v", err)
	}
}

func TestOperatorRedirectHopsAreRechecked(t *testing.T) {
	var metaHit atomic.Int32
	// "metadata" stand-in is never reachable: the redirect target is the link-local literal, refused before any connection.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		metaHit.Add(1)
		http.Redirect(w, r, "http://169.254.169.254/latest/meta-data/", http.StatusFound)
	}))
	defer srv.Close()
	t.Setenv(AllowCIDRsEnv, "127.0.0.1/32") // the operator admits the test server only
	t.Setenv(DenyPrivateEnv, "")
	cl := &http.Client{Transport: OperatorTransport(), CheckRedirect: OperatorCheckRedirect, Timeout: 5 * time.Second}
	resp, err := cl.Get(srv.URL)
	if err == nil {
		resp.Body.Close()
		t.Fatal("redirect to the metadata address must fail")
	}
	if !errors.Is(err, ErrBlocked) {
		t.Fatalf("want ErrBlocked in chain, got %v", err)
	}
	if metaHit.Load() != 1 {
		t.Fatalf("first hop must have been served once, got %d", metaHit.Load())
	}
	// redirect to a different loopback address outside the allowlist is refused too
	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://127.0.0.2:1/x", http.StatusFound)
	}))
	defer srv2.Close()
	if resp, err := cl.Get(srv2.URL); err == nil {
		resp.Body.Close()
		t.Fatal("redirect to non-allowlisted loopback must fail")
	}
	// redirect to a non-http scheme is refused
	srv3 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "file:///etc/passwd")
		w.WriteHeader(http.StatusFound)
	}))
	defer srv3.Close()
	if resp, err := cl.Get(srv3.URL); err == nil {
		resp.Body.Close()
		t.Fatal("redirect to file:// must fail")
	}
}

func TestCheckOperatorURL(t *testing.T) {
	t.Setenv(AllowCIDRsEnv, "")
	t.Setenv(DenyPrivateEnv, "")
	ctx := context.Background()
	bad := []string{
		"http://127.0.0.1:5005/", "http://169.254.169.254/", "https://[::1]/", "http://[::ffff:169.254.169.254]/",
		"http://[fe80::1]/dav", "http://0.0.0.0/", "ftp://192.168.1.2/", "http://user:pw@192.168.1.2/", "not a url",
	}
	for _, u := range bad {
		if _, err := CheckOperatorURL(ctx, u, "http", "https"); err == nil {
			t.Errorf("%q must be refused", u)
		}
	}
	good := []string{"http://192.168.1.20:5005/", "https://10.0.0.5:5006/dav", "https://77.88.21.119/", "https://[fd12::1]/"}
	for _, u := range good {
		if _, err := CheckOperatorURL(ctx, u, "http", "https"); err != nil {
			t.Errorf("%q must be accepted: %v", u, err)
		}
	}
}
