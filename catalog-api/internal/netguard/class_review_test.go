package netguard

// CAT-SECQ1-A review round 1: IPv4-mapped broad allow-list entries (F1), concurrent first connections (F2), tests that
// kill the reviewer mutants on the proxy pin (RM2), the IPv4 broad threshold (RM3) and the Control hook (RM4), and the
// fail-closed own-address lookup (F4).

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

func allowListCIDRs(t *testing.T, env string) []string {
	t.Helper()
	resetAllowListWarnings()
	t.Setenv(AllowBroadEnv, "")
	t.Setenv(AllowCIDRsEnv, env)
	var out []string
	for _, n := range allowedNets() {
		out = append(out, n.String())
	}
	return out
}

// F1: an IPv4-mapped network is judged as the IPv4 network it is for Contains.
func TestSECQ1A_F1_MappedBroadAllowListEntries(t *testing.T) {
	envClean(t)
	for _, e := range []string{"::ffff:0.0.0.0/96", "::ffff:0:0/96", "::ffff:0.0.0.0/100", "::ffff:0.0.0.0/103", "0.0.0.0/0"} {
		if got := allowListCIDRs(t, e); len(got) != 0 {
			t.Errorf("broad entry %q kept: %v", e, got)
		}
		if IsBlockedOperatorIP(net.ParseIP("127.0.0.1")) != true || ClassOf(net.ParseIP("127.0.0.1")) != ClassNever {
			t.Errorf("%q re-admitted loopback", e)
		}
	}
	// legit mapped entry: ::ffff:127.0.0.0/104 is 127.0.0.0/8 (boundary kept)
	if got := allowListCIDRs(t, "::ffff:127.0.0.0/104"); len(got) != 1 {
		t.Errorf("::ffff:127.0.0.0/104 not kept: %v", got)
	}
	if ClassOf(net.ParseIP("127.0.0.1")) != ClassPrivate {
		t.Error("mapped /104 (= 127.0.0.0/8) should re-admit loopback as private (control)")
	}
	// ::/96 never re-admits IPv4 addresses
	allowListCIDRs(t, "::/96")
	if ClassOf(net.ParseIP("127.0.0.1")) != ClassNever || ClassOf(net.ParseIP("::7f00:1")) != ClassNever {
		t.Error("::/96 re-admitted loopback")
	}
	if got := allowListCIDRs(t, "::ffff:0.0.0.0/96"); len(got) != 0 {
		t.Errorf("kept %v", got)
	}
	t.Setenv(AllowBroadEnv, "1")
	if got := allowedNets(); len(got) != 1 {
		t.Errorf("with %s=1 the mapped broad entry must be kept: %v", AllowBroadEnv, got)
	}
}

// RM3: the exact boundary of the broad rule.
func TestSECQ1A_BroadThresholdBoundary(t *testing.T) {
	envClean(t)
	for p := 0; p <= 7; p++ {
		if got := allowListCIDRs(t, fmt.Sprintf("10.0.0.0/%d", p)); len(got) != 0 {
			t.Errorf("/%d IPv4 kept: %v", p, got)
		}
	}
	for _, e := range []string{"10.0.0.0/8", "10.0.0.0/9", "192.168.0.0/16", "fd00::/16", "fd00::/48"} {
		if got := allowListCIDRs(t, e); len(got) != 1 {
			t.Errorf("%s dropped: %v", e, got)
		}
	}
	for _, e := range []string{"fe00::/15", "::/0", "8000::/1"} {
		if got := allowListCIDRs(t, e); len(got) != 0 {
			t.Errorf("%s kept: %v", e, got)
		}
	}
}

// F2: concurrent FIRST connections cannot defeat the pin. The resolver answers public first and private afterwards; the
// public dial is held open while a second request starts.
func TestSECQ1A_Live_ConcurrentFirstConnectionsKeepPin(t *testing.T) {
	envClean(t)
	srv := newHitServer(t, okBody)
	port := srv.Listener.Addr().(*net.TCPAddr).Port
	var n atomic.Int32
	old := lookupIPAddr
	t.Cleanup(func() { lookupIPAddr = old })
	lookupIPAddr = func(ctx context.Context, host string) ([]net.IPAddr, error) {
		if n.Add(1) == 1 {
			return []net.IPAddr{{IP: net.ParseIP("93.184.215.14")}}, nil
		}
		return []net.IPAddr{{IP: net.ParseIP("192.168.1.77")}}, nil
	}
	entered := make(chan struct{}, 4)
	release := make(chan struct{})
	oldDial := dialTCP
	t.Cleanup(func() { dialTCP = oldDial })
	dialTCP = func(ctx context.Context, d *net.Dialer, network, ip, p string) (net.Conn, error) {
		entered <- struct{}{}
		if ip == "93.184.215.14" {
			<-release
		}
		return (&net.Dialer{Timeout: 3 * time.Second}).DialContext(ctx, network, srv.Listener.Addr().String())
	}
	tr := OperatorTransport()
	tr.DisableKeepAlives = true
	c := &http.Client{Transport: tr, Timeout: 10 * time.Second}
	u := fmt.Sprintf("http://reb.test:%d/", port)
	var wg sync.WaitGroup
	var errA, errB error
	wg.Add(1)
	go func() { defer wg.Done(); _, errA = get(c, u) }()
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("first dial never started")
	}
	wg.Add(1)
	go func() { defer wg.Done(); _, errB = get(c, u) }()
	time.Sleep(300 * time.Millisecond) // second request is in flight while the public dial is held open
	close(release)
	wg.Wait()
	if errA != nil {
		t.Fatalf("first (public) request failed: %v", errA)
	}
	if errB == nil || !errors.Is(errB, ErrBlocked) {
		t.Fatalf("second concurrent first connection reached the private answer: %v", errB)
	}
	if srv.hits.Load() != 1 {
		t.Fatalf("server hits %d, want 1", srv.hits.Load())
	}
}

// RM2: the environment-proxy path pins the transport as well.
func TestSECQ1A_ProxyPathPinsFirstDestination(t *testing.T) {
	envClean(t)
	mapResolver(t, map[string][]string{"public.test": {"93.184.215.14"}, "lan.test": {"192.168.1.5"}})
	old := proxyFunc
	t.Cleanup(func() { proxyFunc = old })
	proxyFunc = func(*http.Request) (*url.URL, error) { return url.Parse("http://127.0.0.1:3128") }
	req := func(u string) *http.Request { r, _ := http.NewRequest("GET", u, nil); return r }
	tr := OperatorTransport()
	if _, err := tr.Proxy(req("http://public.test/")); err != nil {
		t.Fatalf("public first destination: %v", err)
	}
	if _, err := tr.Proxy(req("http://lan.test/")); err == nil || !errors.Is(err, ErrBlocked) {
		t.Fatalf("private destination after a public first destination was forwarded to the proxy: %v", err)
	}
	// positive control: a transport whose first destination is private may use private and public
	tr2 := OperatorTransport()
	for _, u := range []string{"http://lan.test/", "http://public.test/"} {
		if _, err := tr2.Proxy(req(u)); err != nil {
			t.Fatalf("control %s: %v", u, err)
		}
	}
}

// RM4: the real Control hook consults the CURRENT ceiling of the pin (not a fixed class), checked on a real dial.
func TestSECQ1A_ControlHookUsesPinCeiling(t *testing.T) {
	envClean(t)
	t.Setenv(AllowCIDRsEnv, "127.0.0.0/8")
	ln, n := countingListener(t)
	addr := ln.Addr().String()
	pin := newClassPin(ClassPrivate)
	d := &net.Dialer{Timeout: 2 * time.Second, Control: operatorControl(pin, false)}
	c, err := d.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("control (private ceiling, allow-listed loopback): %v", err)
	}
	c.Close()
	pin.note(ClassPublic) // first connection reached a public address
	before := n.Load()
	if c, err := d.Dial("tcp", addr); err == nil {
		c.Close()
		t.Fatal("Control hook let a private-class address through a public-pinned ceiling")
	}
	time.Sleep(50 * time.Millisecond)
	if n.Load() != before {
		t.Fatalf("listener accepted %d connections after the refusal", n.Load()-before)
	}
	var rc syscall.RawConn
	if err := operatorControl(pin, false)("tcp", "192.168.1.5:80", rc); err == nil {
		t.Fatal("direct call: private address allowed under a public ceiling")
	}
	if err := operatorControl(pin, false)("tcp", "93.184.215.14:80", rc); err != nil {
		t.Fatalf("direct call: public address refused: %v", err)
	}
}

// F4: an unreadable interface list fails closed for private-range addresses and leaves public ones alone.
func TestSECQ1A_F4_InterfaceLookupFailureFailsClosed(t *testing.T) {
	envClean(t)
	old := interfaceAddrs
	t.Cleanup(func() { interfaceAddrs = old })
	interfaceAddrs = func() ([]net.Addr, error) { return nil, errors.New("netlink unavailable") }
	if ClassOf(net.ParseIP("192.168.1.250")) != ClassNever || ClassOf(net.ParseIP("10.0.0.1")) != ClassNever {
		t.Error("private address not refused while the interface list is unreadable")
	}
	if ClassOf(net.ParseIP("93.184.215.14")) != ClassPublic {
		t.Error("public address affected by the interface lookup failure")
	}
	t.Setenv(AllowCIDRsEnv, "192.168.1.0/24")
	if ClassOf(net.ParseIP("192.168.1.250")) != ClassPrivate {
		t.Error("allow-listed private address refused (operator opt-in must still work)")
	}
}
