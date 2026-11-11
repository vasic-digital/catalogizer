package netguard

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsBlockedIP(t *testing.T) {
	t.Setenv(AllowCIDRsEnv, "")
	blocked := []string{"127.0.0.1", "::1", "10.1.2.3", "172.16.0.1", "192.168.1.5", "169.254.169.254", "fe80::1",
		"0.0.0.0", "::", "100.64.0.1", "224.0.0.1", "255.255.255.255", "fc00::1", "::ffff:127.0.0.1", "240.0.0.1", "0.1.2.3"}
	for _, s := range blocked {
		assert.True(t, IsBlockedIP(net.ParseIP(s)), s)
	}
	for _, s := range []string{"8.8.8.8", "1.1.1.1", "2606:4700:4700::1111"} {
		assert.False(t, IsBlockedIP(net.ParseIP(s)), s)
	}
	assert.True(t, IsBlockedIP(nil))
}

func TestAllowListReadmitsPrivateButNeverMetadata(t *testing.T) {
	t.Setenv(AllowCIDRsEnv, "192.168.1.0/24, 127.0.0.0/8, 169.254.0.0/16, bogus")
	assert.False(t, IsBlockedIP(net.ParseIP("192.168.1.9")))
	assert.False(t, IsBlockedIP(net.ParseIP("127.0.0.1")))
	assert.True(t, IsBlockedIP(net.ParseIP("192.168.2.9")), "outside the listed network")
	assert.True(t, IsBlockedIP(net.ParseIP("169.254.169.254")), "link-local/metadata is never re-admitted")
}

func TestCheckURL(t *testing.T) {
	t.Setenv(AllowCIDRsEnv, "")
	ctx := context.Background()
	for _, bad := range []string{"http://127.0.0.1:8080/x", "https://[::1]/", "http://169.254.169.254/latest/meta-data/",
		"ftp://8.8.8.8/", "https://user:pw@8.8.8.8/", "/relative", "https://localhost/", "http://10.0.0.1/"} {
		_, err := CheckURL(ctx, bad, "http", "https")
		assert.Error(t, err, bad)
	}
	_, err := CheckURL(ctx, "https://8.8.8.8/dav", "https")
	assert.NoError(t, err)
	_, err = CheckURL(ctx, "http://8.8.8.8/dav", "https")
	assert.Error(t, err, "scheme not in the list")
	// an unresolvable name is accepted at validation, never at the dial
	_, err = CheckURL(ctx, "https://does-not-exist.invalid/dav", "https")
	assert.NoError(t, err)
	_, err = DialContext(ctx, "tcp", "does-not-exist.invalid:443")
	assert.Error(t, err, "the dial still fails closed")
}

// The pinned dial refuses a real listening loopback server even though the caller
// reached it by name.
func TestClientCannotReachLoopbackServer(t *testing.T) {
	t.Setenv(AllowCIDRsEnv, "")
	hit := make(chan struct{}, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hit <- struct{}{} }))
	defer srv.Close()
	cl := Client(3*time.Second, 3, nil)
	_, err := cl.Get(srv.URL)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrBlocked)
	select {
	case <-hit:
		t.Fatal("the loopback server was contacted")
	default:
	}
	// with the operator allow-list the same server is reachable (live positive control)
	t.Setenv(AllowCIDRsEnv, "127.0.0.0/8")
	resp, err := cl.Get(srv.URL)
	require.NoError(t, err)
	resp.Body.Close()
	<-hit
}

// F-E: IPv6 forms that embed an IPv4 address are judged as that IPv4 address, and reserved ranges are refused.
func TestIsBlockedIP_EmbeddedAndReservedRanges(t *testing.T) {
	for _, s := range []string{
		"::7f00:1", "::127.0.0.1", "::a00:1", // ::/96 IPv4-compatible
		"64:ff9b::7f00:1", "64:ff9b::a9fe:a9fe", "64:ff9b::a00:1", // NAT64 well-known prefix
		"64:ff9b:1::1",                                          // local-use NAT64
		"2002:7f00:1::1", "2002:a9fe:a9fe::1", "2002:c0a8:1::1", // 6to4 wrapping loopback / metadata / private
		"fec0::1", "feff::1", // deprecated site-local
		"198.18.0.1", "198.19.255.255", "192.0.0.1", // benchmarking, IETF assignments
		"::ffff:127.0.0.1", "::1", "169.254.169.254",
	} {
		ip := net.ParseIP(s)
		require.NotNil(t, ip, s)
		assert.True(t, IsBlockedIP(ip), "%s must be blocked", s)
	}
	for _, s := range []string{"8.8.8.8", "2606:4700:4700::1111", "64:ff9b::808:808", "2002:808:808::1"} {
		assert.False(t, IsBlockedIP(net.ParseIP(s)), "%s is a public address (or wraps one) and must pass", s)
	}
}

// F-C: the connection goes to the address that was checked, resolved exactly once. The name below only exists in the stub
// resolver, so a dial by name (or a second resolution that rebinds) could not reach the listener.
func TestDialContext_DialsTheCheckedIP(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
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
	calls := 0
	old := lookupIPAddr
	lookupIPAddr = func(ctx context.Context, host string) ([]net.IPAddr, error) {
		calls++
		if calls == 1 {
			return []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}}, nil
		}
		return []net.IPAddr{{IP: net.ParseIP("169.254.169.254")}}, nil // rebinding answer: must never be used
	}
	defer func() { lookupIPAddr = old }()
	_, port, _ := net.SplitHostPort(ln.Addr().String())
	conn, err := DialContext(context.Background(), "tcp", net.JoinHostPort("pinned.invalid", port))
	require.NoError(t, err, "must dial the IP that passed the check")
	conn.Close()
	assert.Equal(t, 1, calls, "the name is resolved once; the dial uses the checked IP")
	assert.Equal(t, "127.0.0.1", conn.RemoteAddr().(*net.TCPAddr).IP.String())

	// a resolution that yields a blocked address is refused with no connection attempt
	t.Setenv(AllowCIDRsEnv, "")
	lookupIPAddr = func(ctx context.Context, host string) ([]net.IPAddr, error) {
		return []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}}, nil
	}
	_, err = DialContext(context.Background(), "tcp", net.JoinHostPort("evil.invalid", port))
	assert.ErrorIs(t, err, ErrBlocked)
}

// F-D: the transport never uses an environment proxy (a proxy would resolve the target itself).
func TestTransport_IgnoresEnvironmentProxy(t *testing.T) {
	t.Setenv("HTTP_PROXY", "http://proxy.invalid:3128")
	t.Setenv("HTTPS_PROXY", "http://proxy.invalid:3128")
	tr := Transport()
	assert.Nil(t, tr.Proxy, "Transport.Proxy must be nil")
	req := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
	if tr.Proxy != nil {
		u, _ := tr.Proxy(req)
		assert.Nil(t, u)
	}
}
