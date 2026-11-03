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
