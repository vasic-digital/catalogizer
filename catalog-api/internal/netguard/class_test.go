package netguard

// CAT-SECQ1-A: address classes, class-pinned transport, dial-time Control hook, redirect chains through real HTTP
// servers. Doubles (unit class, §11.4.289): a replaced name resolver and the dialTCP seam, which routes a checked
// "public" / "private" address to a real listener on 127.0.0.1; all policy checks run before that seam, the HTTP
// servers, the transport, the redirect handling and the sockets are real.

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/studio-b12/gowebdav"
)

func fakeInterfaces(t *testing.T, cidrs ...string) {
	t.Helper()
	old := interfaceAddrs
	t.Cleanup(func() { interfaceAddrs = old })
	interfaceAddrs = func() ([]net.Addr, error) {
		var out []net.Addr
		for _, c := range cidrs {
			ip, n, _ := net.ParseCIDR(c)
			n.IP = ip
			out = append(out, n)
		}
		return out, nil
	}
}

func mapResolver(t *testing.T, m map[string][]string) *atomic.Int32 {
	t.Helper()
	old := lookupIPAddr
	t.Cleanup(func() { lookupIPAddr = old })
	var calls atomic.Int32
	lookupIPAddr = func(ctx context.Context, host string) ([]net.IPAddr, error) {
		calls.Add(1)
		ips, ok := m[host]
		if !ok {
			return nil, &net.DNSError{Err: "no such host", Name: host, IsNotFound: true}
		}
		var out []net.IPAddr
		for _, s := range ips {
			out = append(out, net.IPAddr{IP: net.ParseIP(s)})
		}
		return out, nil
	}
	return &calls
}

// routeDial sends the checked address ip (any port) to the real listener at target.
func routeDial(t *testing.T, routes map[string]string) {
	t.Helper()
	old := dialTCP
	t.Cleanup(func() { dialTCP = old })
	dialTCP = func(ctx context.Context, d *net.Dialer, network, ip, port string) (net.Conn, error) {
		target, ok := routes[ip]
		if !ok {
			return nil, fmt.Errorf("test route: no route for %s", ip)
		}
		return (&net.Dialer{Timeout: 3 * time.Second}).DialContext(ctx, network, target)
	}
}

type hitServer struct {
	*httptest.Server
	hits *atomic.Int32
}

func newHitServer(t *testing.T, h http.HandlerFunc) hitServer {
	t.Helper()
	var n atomic.Int32
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n.Add(1)
		h(w, r)
	}))
	t.Cleanup(s.Close)
	return hitServer{s, &n}
}

func okBody(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, "ok") }

func redirectTo(target string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target, http.StatusFound) }
}

func envClean(t *testing.T) {
	t.Setenv(AllowCIDRsEnv, "")
	t.Setenv(DenyPrivateEnv, "")
	t.Setenv(AllowBroadEnv, "")
	fakeInterfaces(t, "10.99.0.2/24", "127.0.0.1/8")
}

func get(c *http.Client, u string) (int, error) {
	resp, err := c.Get(u)
	if err != nil {
		return 0, err
	}
	resp.Body.Close()
	return resp.StatusCode, nil
}

func TestSECQ1A_ClassOfTable(t *testing.T) {
	envClean(t)
	cases := []struct {
		ip   string
		want Class
	}{
		{"192.168.1.250", ClassPrivate}, {"10.0.0.5", ClassPrivate}, {"172.17.0.1", ClassPrivate}, {"100.64.0.1", ClassPrivate},
		{"fd12:3456::1", ClassPrivate}, {"::ffff:10.0.0.1", ClassPrivate},
		{"93.184.215.14", ClassPublic}, {"2606:2800:220:1:248:1893:25c8:1946", ClassPublic}, {"168.63.129.17", ClassPublic},
		{"127.0.0.1", ClassNever}, {"::1", ClassNever}, {"::ffff:127.0.0.1", ClassNever}, {"169.254.169.254", ClassNever},
		{"168.63.129.16", ClassNever}, {"100.100.100.200", ClassNever}, {"0.0.0.0", ClassNever}, {"240.0.0.1", ClassNever},
		{"198.18.0.1", ClassNever}, {"224.0.0.1", ClassNever}, {"64:ff9b::7f00:1", ClassNever}, {"2002:7f00:1::", ClassNever},
		{"10.99.0.2", ClassNever}, // own interface address
	}
	for _, c := range cases {
		if got := ClassOf(net.ParseIP(c.ip)); got != c.want {
			t.Errorf("ClassOf(%s) = %s, want %s", c.ip, got, c.want)
		}
	}
	if ClassOf(nil) != ClassNever {
		t.Error("nil must be never")
	}
	// the allow-list re-admits loopback and own addresses as PRIVATE, never as public, never reserved/metadata
	t.Setenv(AllowCIDRsEnv, "127.0.0.0/8,10.99.0.0/24,169.254.0.0/16,168.63.129.16/32")
	for ip, want := range map[string]Class{"127.0.0.1": ClassPrivate, "10.99.0.2": ClassPrivate, "169.254.169.254": ClassNever, "168.63.129.16": ClassNever} {
		if got := ClassOf(net.ParseIP(ip)); got != want {
			t.Errorf("allow-listed ClassOf(%s) = %s, want %s", ip, got, want)
		}
	}
}

func TestSECQ1A_PermitsMatrix(t *testing.T) {
	type row struct {
		approved, target Class
		want             bool
	}
	for _, r := range []row{
		{ClassPublic, ClassPublic, true}, {ClassPublic, ClassPrivate, false}, {ClassPublic, ClassNever, false},
		{ClassPrivate, ClassPublic, true}, {ClassPrivate, ClassPrivate, true}, {ClassPrivate, ClassNever, false},
		{ClassNever, ClassPublic, false}, {ClassNever, ClassPrivate, false}, {ClassNever, ClassNever, false},
	} {
		if got := r.approved.Permits(r.target); got != r.want {
			t.Errorf("%s.Permits(%s) = %v, want %v", r.approved, r.target, got, r.want)
		}
	}
	for _, c := range []Class{ClassNever, ClassPrivate, ClassPublic} {
		if p, ok := ParseClass(c.String()); !ok || p != c {
			t.Errorf("ParseClass(%q) = %v %v", c.String(), p, ok)
		}
	}
	if _, ok := ParseClass("lan"); ok {
		t.Error("ParseClass accepted an unknown class")
	}
}

// Policy-level matrix (promoted from the SECQ-1 research): verdicts of both URL checks.
func TestSECQ1A_AdversarialURLMatrix(t *testing.T) {
	envClean(t)
	mapResolver(t, map[string][]string{"metadata.google.internal": {"169.254.169.254"}, "example.com": {"93.184.215.14"}, "localhost": {"127.0.0.1", "::1"}, "localhost.localdomain": {"127.0.0.1"}})
	refused := []string{
		"http://127.0.0.1/", "http://localhost/", "http://localhost.localdomain/", "http://127.1/", "http://127.0.1/", "http://0177.0.0.1/",
		"http://0x7f.0.0.1/", "http://0x7f000001/", "http://2130706433/", "http://017700000001/", "http://127.000.000.001/", "http://0/", "http://0.0.0.0/",
		"http://[::1]/", "http://[0:0:0:0:0:0:0:1]/", "http://[::ffff:127.0.0.1]/", "http://[::ffff:7f00:1]/", "http://[0:0:0:0:0:ffff:127.0.0.1]/",
		"http://[::127.0.0.1]/", "http://[::ffff:0:127.0.0.1]/", "http://[64:ff9b::127.0.0.1]/", "http://[64:ff9b:1::7f00:1]/", "http://[2002:7f00:1::]/",
		"http://[fe80::1%25eth0]/", "http://169.254.169.254/", "http://0xa9fea9fe/", "http://2852039166/", "http://0251.0376.0251.0376/", "http://169.254.43518/",
		"http://[::ffff:169.254.169.254]/", "http://[::ffff:a9fe:a9fe]/", "http://[64:ff9b::a9fe:a9fe]/", "http://[2002:a9fe:a9fe::]/", "http://[fd00:ec2::254]/",
		"http://[fd20:ce::254]/", "http://100.100.100.200/", "http://192.0.0.192/", "http://168.63.129.16/", "http://metadata.google.internal/",
		"http://１２７.０.０.１/", "http://127。0。0。1/", "http://ⓛⓞⓒⓐⓛⓗⓞⓢⓣ/", "http://127%2e0%2e0%2e1/", "http://user@127.0.0.1/", "http://example.com@127.0.0.1/",
		"http://240.0.0.1/", "http://198.18.0.1/", "http://255.255.255.255/", "http://224.0.0.1/", "http://[ff02::1]/", "http://10.99.0.2/", // own address
	}
	allowed := []string{"http://192.168.1.250/", "https://example.com/", "http://10.0.0.1/", "http://172.17.0.1/", "http://100.64.0.1/", "http://[fd12:3456::1]/", "http://[::ffff:10.0.0.1]/", "http://xn--bcher-kva.example/"}
	for _, u := range refused {
		if _, err := CheckOperatorURL(context.Background(), u, "http", "https"); err == nil {
			t.Errorf("operator check ALLOWED %s", u)
		}
	}
	for _, u := range allowed {
		if _, err := CheckOperatorURL(context.Background(), u, "http", "https"); err != nil {
			t.Errorf("operator check refused (positive control) %s: %v", u, err)
		}
	}
}

// B1 live: a transport whose first connection reached a PUBLIC address cannot follow a redirect into the LAN.
func TestSECQ1A_Live_PublicOriginRedirectIntoLANRefused(t *testing.T) {
	envClean(t)
	lan := newHitServer(t, okBody)
	lanPort := lan.Listener.Addr().(*net.TCPAddr).Port
	origin := newHitServer(t, redirectTo(fmt.Sprintf("http://192.168.1.50:%d/admin", lanPort)))
	mapResolver(t, map[string][]string{"public.test": {"93.184.215.14"}})
	routeDial(t, map[string]string{"93.184.215.14": origin.Listener.Addr().String(), "192.168.1.50": lan.Listener.Addr().String()})
	// default policy (no CheckRedirect of our own: this is the gowebdav situation)
	c := &http.Client{Transport: OperatorTransport(), Timeout: 5 * time.Second}
	_, err := get(c, fmt.Sprintf("http://public.test:%d/", origin.Listener.Addr().(*net.TCPAddr).Port))
	if err == nil || !errors.Is(err, ErrBlocked) {
		t.Fatalf("redirect public -> LAN was not refused: %v", err)
	}
	if origin.hits.Load() != 1 || lan.hits.Load() != 0 {
		t.Fatalf("origin hits %d (want 1), LAN server hits %d (want 0)", origin.hits.Load(), lan.hits.Load())
	}
	// same chain, redirect policy of the http.Client path
	c2 := &http.Client{Transport: OperatorTransport(), CheckRedirect: OperatorCheckRedirect, Timeout: 5 * time.Second}
	if _, err := get(c2, fmt.Sprintf("http://public.test:%d/", origin.Listener.Addr().(*net.TCPAddr).Port)); err == nil || !errors.Is(err, ErrBlocked) {
		t.Fatalf("CheckRedirect path: redirect public -> LAN was not refused: %v", err)
	}
	if lan.hits.Load() != 0 {
		t.Fatalf("LAN server was reached: %d", lan.hits.Load())
	}
}

// golden-FALSE (§11.4.201(1)): a private-class origin keeps working, may go to public, never to loopback.
func TestSECQ1A_Live_PrivateOriginMayFollowPublicButNeverLoopback(t *testing.T) {
	envClean(t)
	pub := newHitServer(t, okBody)
	pubPort := pub.Listener.Addr().(*net.TCPAddr).Port
	loop := newHitServer(t, okBody)
	loopURL := loop.URL
	lanOrigin := newHitServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/toPublic":
			http.Redirect(w, r, fmt.Sprintf("http://93.184.215.15:%d/", pubPort), http.StatusFound)
		case "/toLAN2":
			http.Redirect(w, r, fmt.Sprintf("http://192.168.1.60:%d/", pubPort), http.StatusFound)
		default:
			http.Redirect(w, r, loopURL, http.StatusFound)
		}
	})
	mapResolver(t, map[string][]string{"nas.test": {"192.168.1.40"}})
	routeDial(t, map[string]string{"192.168.1.40": lanOrigin.Listener.Addr().String(), "93.184.215.15": pub.Listener.Addr().String(), "192.168.1.60": pub.Listener.Addr().String()})
	base := fmt.Sprintf("http://nas.test:%d", lanOrigin.Listener.Addr().(*net.TCPAddr).Port)
	for _, cr := range []func(*http.Request, []*http.Request) error{nil, OperatorCheckRedirect} {
		c := &http.Client{Transport: OperatorTransport(), CheckRedirect: cr, Timeout: 5 * time.Second}
		before := pub.hits.Load()
		if code, err := get(c, base+"/toPublic"); err != nil || code != 200 {
			t.Fatalf("private -> public redirect refused: %d %v", code, err)
		}
		if code, err := get(c, base+"/toLAN2"); err != nil || code != 200 {
			t.Fatalf("private -> private redirect refused: %d %v", code, err)
		}
		if pub.hits.Load() != before+2 {
			t.Fatalf("public server hits %d, want %d", pub.hits.Load(), before+2)
		}
		l0 := loop.hits.Load()
		if _, err := get(c, base+"/toLoopback"); err == nil || !errors.Is(err, ErrBlocked) {
			t.Fatalf("private -> loopback not refused: %v", err)
		}
		if loop.hits.Load() != l0 {
			t.Fatalf("loopback server was reached")
		}
	}
}

// Rebinding: a name that was public at the first connection and answers a private address later is refused by the
// pinned transport; a private-class transport still reaches it (positive control).
func TestSECQ1A_Live_RebindingPublicToPrivateRefused(t *testing.T) {
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
	routeDial(t, map[string]string{"93.184.215.14": srv.Listener.Addr().String(), "192.168.1.77": srv.Listener.Addr().String()})
	tr := OperatorTransport()
	tr.DisableKeepAlives = true
	c := &http.Client{Transport: tr, Timeout: 5 * time.Second}
	u := fmt.Sprintf("http://reb.test:%d/", port)
	if code, err := get(c, u); err != nil || code != 200 {
		t.Fatalf("first request: %d %v", code, err)
	}
	h := srv.hits.Load()
	if _, err := get(c, u); err == nil || !errors.Is(err, ErrBlocked) {
		t.Fatalf("rebound private answer not refused: %v", err)
	}
	if srv.hits.Load() != h {
		t.Fatal("server reached after rebinding")
	}
	// positive control: a transport that never saw a public address reaches the private answer
	n.Store(1)
	tr2 := OperatorTransport()
	tr2.DisableKeepAlives = true
	if code, err := get(&http.Client{Transport: tr2, Timeout: 5 * time.Second}, u); err != nil || code != 200 {
		t.Fatalf("private-class control: %d %v", code, err)
	}
}

func TestSECQ1A_ApprovedClassParameter(t *testing.T) {
	envClean(t)
	mapResolver(t, map[string][]string{"pubonly.test": {"93.184.215.14", "192.168.1.88"}})
	srv := newHitServer(t, okBody)
	routeDial(t, map[string]string{"93.184.215.14": srv.Listener.Addr().String(), "192.168.1.88": srv.Listener.Addr().String(), "192.168.1.89": srv.Listener.Addr().String()})
	port := srv.Listener.Addr().(*net.TCPAddr).Port
	// public-approved: private literal refused, mixed answer filtered to the public address
	if c, err := OperatorDialContextClass(context.Background(), "tcp", fmt.Sprintf("192.168.1.89:%d", port), ClassPublic); err == nil {
		c.Close()
		t.Fatal("public-approved dial reached a private literal")
	}
	c, err := OperatorDialContextClass(context.Background(), "tcp", fmt.Sprintf("pubonly.test:%d", port), ClassPublic)
	if err != nil {
		t.Fatalf("public-approved dial of a mixed answer: %v", err)
	}
	c.Close()
	// private-approved (default): the private literal works (positive control), loopback does not (negative control)
	c, err = OperatorDialContext(context.Background(), "tcp", fmt.Sprintf("192.168.1.89:%d", port))
	if err != nil {
		t.Fatalf("positive control: %v", err)
	}
	c.Close()
	if c, err := OperatorDialContext(context.Background(), "tcp", fmt.Sprintf("127.0.0.1:%d", port)); err == nil {
		c.Close()
		t.Fatal("negative control: loopback dialled")
	}
	// never-approved dials nothing
	if c, err := OperatorDialContextClass(context.Background(), "tcp", fmt.Sprintf("93.184.215.14:%d", port), ClassNever); err == nil {
		c.Close()
		t.Fatal("never-approved dial succeeded")
	}
	// transport variants
	trPub := OperatorTransportFor(ClassPublic)
	if _, err := get(&http.Client{Transport: trPub, Timeout: 3 * time.Second}, fmt.Sprintf("http://192.168.1.89:%d/", port)); err == nil || !errors.Is(err, ErrBlocked) {
		t.Fatalf("public-approved transport reached a LAN literal: %v", err)
	}
	// redirect check with an explicit class
	req, _ := http.NewRequest("GET", "http://192.168.1.89/", nil)
	if err := OperatorCheckRedirectFor(ClassPublic)(req, []*http.Request{req}); err == nil {
		t.Fatal("public-approved redirect check allowed a LAN hop")
	}
	if err := OperatorCheckRedirectFor(ClassPrivate)(req, []*http.Request{req}); err != nil {
		t.Fatalf("private-approved redirect check refused a LAN hop: %v", err)
	}
}

// The kernel-level Control hook: the address is re-checked at connect time, after resolution and filtering.
func TestSECQ1A_Live_ControlHookRefusesAtConnect(t *testing.T) {
	envClean(t)
	t.Setenv(AllowCIDRsEnv, "127.0.0.0/8")
	ln, n := countingListener(t)
	addr := ln.Addr().String()
	old := dialTCP
	t.Cleanup(func() { dialTCP = old })
	// positive control: unchanged policy, real dial through the Control hook succeeds
	c, err := OperatorDialContext(context.Background(), "tcp", addr)
	if err != nil {
		t.Fatalf("positive control (allow-listed loopback): %v", err)
	}
	c.Close()
	// the allow-list changes between the checks and the connect: only the Control hook can still refuse
	dialTCP = func(ctx context.Context, d *net.Dialer, network, ip, port string) (net.Conn, error) {
		t.Setenv(AllowCIDRsEnv, "")
		return d.DialContext(ctx, network, net.JoinHostPort(ip, port))
	}
	time.Sleep(100 * time.Millisecond) // let the accept of the positive-control connection be counted
	before := n.Load()
	if c, err := OperatorDialContext(context.Background(), "tcp", addr); err == nil {
		c.Close()
		t.Fatal("connect succeeded although the policy changed before the connect")
	} else if !errors.Is(err, ErrBlocked) {
		t.Fatalf("refused, but not by the policy: %v", err)
	}
	time.Sleep(50 * time.Millisecond)
	if n.Load() != before {
		t.Fatalf("listener accepted %d new connections, want 0", n.Load()-before)
	}
}

// B2 live, class level: a listener on all interfaces is refused through the host's real LAN address unless allow-listed.
func TestSECQ1A_Live_OwnAddressNeverUnlessAllowListed(t *testing.T) {
	t.Setenv(AllowCIDRsEnv, "")
	t.Setenv(DenyPrivateEnv, "")
	ln, err := net.Listen("tcp", "0.0.0.0:0")
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
	port := ln.Addr().(*net.TCPAddr).Port
	addrs, _ := net.InterfaceAddrs()
	tried := 0
	for _, a := range addrs {
		ipn, ok := a.(*net.IPNet)
		if !ok || ipn.IP.To4() == nil || ipn.IP.IsLoopback() || ipn.IP.IsLinkLocalUnicast() {
			continue
		}
		tried++
		if ClassOf(ipn.IP) != ClassNever {
			t.Errorf("own address %s has class %s", ipn.IP, ClassOf(ipn.IP))
		}
		target := net.JoinHostPort(ipn.IP.String(), fmt.Sprint(port))
		if c, derr := OperatorDialContext(context.Background(), "tcp", target); derr == nil {
			c.Close()
			t.Errorf("dial to own address %s allowed", ipn.IP)
		}
		t.Setenv(AllowCIDRsEnv, ipn.IP.String()+"/32")
		c, derr := OperatorDialContext(context.Background(), "tcp", target)
		if derr != nil {
			t.Errorf("positive control: allow-listed own address %s refused: %v", ipn.IP, derr)
		} else {
			c.Close()
		}
		t.Setenv(AllowCIDRsEnv, "")
	}
	if tried == 0 {
		t.Skip("hardware_not_present: no non-loopback IPv4 interface address")
	}
}

// B1 through the real gowebdav client: the library owns its CheckRedirect (it cannot be replaced), so the class pin of
// the transport is the only protection on that path.
func TestSECQ1A_Live_GoWebDAVPublicRootCannotBeRedirectedIntoLAN(t *testing.T) {
	envClean(t)
	lan := newHitServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusMultiStatus)
		fmt.Fprint(w, `<?xml version="1.0"?><d:multistatus xmlns:d="DAV:"></d:multistatus>`)
	})
	lanPort := lan.Listener.Addr().(*net.TCPAddr).Port
	pubRoot := newHitServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, fmt.Sprintf("http://192.168.1.50:%d/dav/", lanPort), http.StatusTemporaryRedirect)
	})
	mapResolver(t, map[string][]string{"dav.public.test": {"93.184.215.14"}})
	routeDial(t, map[string]string{"93.184.215.14": pubRoot.Listener.Addr().String(), "192.168.1.50": lan.Listener.Addr().String()})
	gc := gowebdav.NewClient(fmt.Sprintf("http://dav.public.test:%d/dav/", pubRoot.Listener.Addr().(*net.TCPAddr).Port), "u", "p")
	gc.SetTransport(OperatorTransport())
	if _, err := gc.ReadDir("/"); err == nil {
		t.Fatal("gowebdav followed a redirect from a public root into the LAN")
	}
	if lan.hits.Load() != 0 {
		t.Fatalf("LAN server hit %d times through gowebdav", lan.hits.Load())
	}
	if pubRoot.hits.Load() == 0 {
		t.Fatal("public root was never contacted: the test did not exercise the chain")
	}
	// positive control: the same chain from a LAN root works
	mapResolver(t, map[string][]string{"nas.test": {"192.168.1.40"}})
	routeDial(t, map[string]string{"192.168.1.40": pubRoot.Listener.Addr().String(), "192.168.1.50": lan.Listener.Addr().String()})
	gc2 := gowebdav.NewClient(fmt.Sprintf("http://nas.test:%d/dav/", pubRoot.Listener.Addr().(*net.TCPAddr).Port), "u", "p")
	gc2.SetTransport(OperatorTransport())
	if _, err := gc2.ReadDir("/"); err != nil {
		t.Fatalf("LAN root -> LAN redirect (positive control): %v", err)
	}
	if lan.hits.Load() == 0 {
		t.Fatal("positive control did not reach the LAN server")
	}
}
