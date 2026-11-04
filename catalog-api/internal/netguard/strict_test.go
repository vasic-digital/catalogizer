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

func TestStrictDialIgnoresAllowList(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.2:0")
	if err != nil {
		t.Skipf("cannot bind 127.0.0.2: %v", err)
	}
	defer ln.Close()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			c.Close()
		}
	}()
	t.Setenv(AllowCIDRsEnv, "127.0.0.0/8")
	// positive control: the allow-list-aware dialer connects
	c, err := DialContext(context.Background(), "tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("control dial should succeed with the env allow-list: %v", err)
	}
	c.Close()
	if c, err := StrictDialContext(context.Background(), "tcp", ln.Addr().String()); err == nil {
		c.Close()
		t.Fatal("strict dial must refuse loopback even when the env allows it")
	}
	if !IsBlockedIPStrict(net.ParseIP("127.0.0.2")) || IsBlockedIP(net.ParseIP("127.0.0.2")) {
		t.Fatal("strict must block, allow-list-aware must allow, 127.0.0.2")
	}
	for _, ip := range []string{"169.254.169.254", "10.0.0.1", "192.168.1.1", "100.64.0.1", "::1"} {
		if !IsBlockedIPStrict(net.ParseIP(ip)) {
			t.Errorf("%s must be blocked in strict mode", ip)
		}
	}
}

// AZ2-R3: IPv4-mapped IPv6 forms of blocked addresses stay blocked in strict mode even
// when the operator allow-list covers the IPv4 range, and at dial time (not only in
// IsBlockedIPStrict).
func TestStrictBlocksIPv4MappedIPv6WithAllowListSet(t *testing.T) {
	t.Setenv(AllowCIDRsEnv, "127.0.0.0/8,10.0.0.0/8")
	for _, s := range []string{"::ffff:127.0.0.2", "::ffff:10.0.0.1", "::ffff:169.254.169.254", "::ffff:192.168.1.1"} {
		ip := net.ParseIP(s)
		if ip == nil {
			t.Fatalf("test input %q does not parse", s)
		}
		if !IsBlockedIPStrict(ip) {
			t.Errorf("%s must be blocked in strict mode despite the allow-list", s)
		}
	}
	// control: the allow-list-aware check does allow the mapped form of an allowed range
	if IsBlockedIP(net.ParseIP("::ffff:127.0.0.2")) || IsBlockedIP(net.ParseIP("::ffff:10.0.0.1")) {
		t.Error("control: allow-list-aware check should allow the mapped form of an allowed range")
	}
	// dial path: a literal mapped address is refused by the strict dialer
	for _, addr := range []string{"[::ffff:127.0.0.2]:9", "[::ffff:10.0.0.1]:9"} {
		if c, err := StrictDialContext(context.Background(), "tcp", addr); err == nil {
			c.Close()
			t.Errorf("strict dial to %s must be refused", addr)
		} else if !errors.Is(err, ErrBlocked) {
			t.Errorf("strict dial to %s: want ErrBlocked, got %v", addr, err)
		}
	}
}

// Review round 1 (item 3): the redirect cap of Client/StrictClient.
func TestClientRedirectCap(t *testing.T) {
	var hits int32
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		http.Redirect(w, r, srv.URL+r.URL.Path+"x", http.StatusFound) // endless chain
	}))
	defer srv.Close()
	t.Setenv(AllowCIDRsEnv, "127.0.0.0/8")
	c := Client(5*time.Second, 2, HTTPOrHTTPS)
	resp, err := c.Get(srv.URL + "/a")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	// 1 initial request + at most 2 followed redirects; the 3rd redirect response is returned as is
	if got := atomic.LoadInt32(&hits); got != 3 {
		t.Fatalf("maxRedirects=2 must stop after 3 requests, got %d", got)
	}
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("last response should be the unfollowed redirect, got %d", resp.StatusCode)
	}
	// nil redirectOK: no redirect is followed at all
	atomic.StoreInt32(&hits, 0)
	resp, err = Client(5*time.Second, 5, nil).Get(srv.URL + "/a")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Fatalf("nil redirectOK must follow nothing, got %d requests", got)
	}
}
