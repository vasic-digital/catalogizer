package filesystem

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"syscall"
	"testing"
	"time"

	"catalogizer/internal/netguard"
	"catalogizer/models"

	"github.com/stretchr/testify/require"
)

func fuRoot(proto, host, opts string) *models.StorageRoot {
	r := &models.StorageRoot{Protocol: proto, Host: &host}
	if opts != "" {
		r.Options = &opts
	}
	return r
}

// F5 RM-B: create refuses an addr= / mountaddr= value that is not an IP literal (a name would be re-resolved by the kernel, unvetted).
func TestSECDIALFU_F5_CreateRefusesNonIPAddrOptions(t *testing.T) {
	t.Setenv(netguard.AllowCIDRsEnv, "")
	for _, o := range []string{"addr=nas.lan", "mountaddr=nas.lan", "vers=3,addr=nas.lan", "vers=3,mountaddr=nas.lan,ro"} {
		require.ErrorIs(t, VetStorageRoot(context.Background(), fuRoot("nfs", "192.168.1.20", o)), ErrStorageHostRefused, o)
	}
	require.NoError(t, VetStorageRoot(context.Background(), fuRoot("nfs", "192.168.1.20", "vers=3,addr=192.168.1.21,mountaddr=192.168.1.22")))
}

// F5 RM-C: a name that does not resolve yet (NAS off, DNS not up) is ACCEPTED at create; a name that resolves only to refused addresses is not.
func TestSECDIALFU_F5_UnresolvableNameAcceptedAtCreate(t *testing.T) {
	t.Setenv(netguard.AllowCIDRsEnv, "")
	require.NoError(t, VetStorageRoot(context.Background(), fuRoot("smb", "no-such-host.invalid", "")))
	require.NoError(t, VetStorageRoot(context.Background(), fuRoot("nfs", "no-such-host.invalid", "")))
	require.ErrorIs(t, VetStorageRoot(context.Background(), fuRoot("smb", "127.0.0.1", "")), ErrStorageHostRefused)
}

// F5 RM-D: the connect is bounded by the given timeout (a dialer that never answers returns at the bound), and the SMB client asks for 10 s.
func TestSECDIALFU_F5_DialStorageBoundsConnect(t *testing.T) {
	old := storageDialContext
	storageDialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	defer func() { storageDialContext = old }()
	// the parent deadline only keeps a regression (no bound applied) from hanging the test; the bound under test is the 80 ms one
	parent, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	start := time.Now()
	_, err := dialStorage(parent, "nas.lan", "445", 80*time.Millisecond)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Less(t, time.Since(start), time.Second, "returned at the 80 ms bound, not at the parent deadline")
}

func TestSECDIALFU_F5_SMBConnectBoundIsTenSeconds(t *testing.T) {
	var left time.Duration
	old := storageDialContext
	storageDialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		dl, ok := ctx.Deadline()
		require.True(t, ok, "SMB connect must carry a deadline")
		left = time.Until(dl)
		return nil, errors.New("recorded")
	}
	defer func() { storageDialContext = old }()
	_ = NewSmbClient(&SmbConfig{Host: "nas.lan", Port: 445, Share: "s"}).Connect(context.Background())
	require.Greater(t, left, 9*time.Second)
	require.LessOrEqual(t, left, 10*time.Second)
}

// F4 class: SMB / FTP connect errors surface on /storage-roots/:id/status; the kernel's "dial tcp <resolved ip>:port" text must not carry the
// resolved address, while the cause stays inspectable.
func TestSECDIALFU_F4_DialErrorDoesNotLeakResolvedIP(t *testing.T) {
	old := storageDialContext
	storageDialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		return nil, &net.OpError{Op: "dial", Net: "tcp", Addr: &net.TCPAddr{IP: net.ParseIP("10.9.8.7"), Port: 445}, Err: syscall.ECONNREFUSED}
	}
	defer func() { storageDialContext = old }()
	ctx := context.Background()
	serr := NewSmbClient(&SmbConfig{Host: "nas.lan", Port: 445, Share: "s"}).Connect(ctx)
	ferr := NewFTPClient(&FTPConfig{Host: "nas.lan", Port: 21}).Connect(ctx)
	for _, err := range []error{serr, ferr} {
		require.Error(t, err)
		require.NotContains(t, err.Error(), "10.9.8.7")
		require.ErrorIs(t, err, syscall.ECONNREFUSED)
	}
}

func TestSECDIALFU_F4_ScrubKeepsConfiguredHostAndBlockedClass(t *testing.T) {
	old := storageDialContext
	storageDialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		return nil, errors.Join(netguard.ErrBlocked, errors.New("[fd12::5]:445 is outside the approved class and 192.168.1.20 too"))
	}
	defer func() { storageDialContext = old }()
	_, err := dialStorage(context.Background(), "192.168.1.20", "445", time.Second)
	require.ErrorIs(t, err, netguard.ErrBlocked)
	require.NotContains(t, err.Error(), "fd12::5")
	require.Contains(t, err.Error(), "192.168.1.20", "the configured host may be echoed")
}

func TestSECDIALFU_F4_ScrubResolvedAddrsCases(t *testing.T) {
	for in, want := range map[string]string{
		"dial tcp 10.9.8.7:445: connection refused":  "dial tcp <address>:445: connection refused",
		"dial tcp [fd12::5]:445: i/o timeout":        "dial tcp [<address>]:445: i/o timeout",
		"lookup nas.lan: no such host":               "lookup nas.lan: no such host",
		"server 1.2 at 12:30 version 3.4.5 failed":   "server 1.2 at 12:30 version 3.4.5 failed",
		"to 192.168.1.20 (configured) and 10.0.0.1.": "to 192.168.1.20 (configured) and <address>.",
	} {
		require.Equal(t, want, scrubResolvedAddrs(in, "192.168.1.20"), in)
	}
	require.Equal(t, "dial tcp <address>:80", scrubResolvedAddrs("dial tcp 127.0.0.1:80", "localhost"))
}

// The WebDAV / S3 transport (same class): a refused connect to a name that resolved to an address does not name that address.
func TestSECDIALFU_F4_StorageTransportDoesNotLeakResolvedIP(t *testing.T) {
	t.Setenv(netguard.AllowCIDRsEnv, "127.0.0.0/8,::1/128")
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := ln.Addr().(*net.TCPAddr).Port
	require.NoError(t, ln.Close()) // nothing listens: connection refused from the resolved address
	hc := &http.Client{Transport: operatorStorageTransport(), Timeout: 3 * time.Second}
	_, err = hc.Get("http://localhost:" + strconv.Itoa(port) + "/")
	require.Error(t, err)
	require.NotContains(t, err.Error(), "127.0.0.1")
	require.NotContains(t, err.Error(), "::1")
	require.ErrorIs(t, err, syscall.ECONNREFUSED)
}

// Review I1 / M2 / M3: the IPv6 and boundary shapes of the scrubber, one table.
func TestSECDIALFU_F4_ScrubIPv6AndBoundaryShapes(t *testing.T) {
	const host = "nas.lan"
	for in, want := range map[string]string{
		"dial tcp [2001:db8:1::]:445: connect: refused":  "dial tcp [<address>]:445: connect: refused",
		"dial tcp [::]:445: refused":                     "dial tcp [<address>]:445: refused",
		"dial tcp [::1]:445: refused":                    "dial tcp [<address>]:445: refused",
		"dial tcp [2001:db8::]:445: refused":             "dial tcp [<address>]:445: refused",
		"dial tcp [2001:DB8::AB]:445: refused":           "dial tcp [<address>]:445: refused",
		"dial tcp [fe80::1%eth0]:445: refused":           "dial tcp [<address>]:445: refused",
		"dial tcp [::ffff:10.0.0.1]:445: refused":        "dial tcp [<address>]:445: refused",
		"dial tcp ::ffff:10.0.0.1: refused":              "dial tcp <address>: refused",
		"dial tcp 2001:db8::1: refused":                  "dial tcp <address>: refused",
		"unreachable 2001:db8::.":                        "unreachable <address>.",
		"addr b:10.0.0.1 x":                              "addr b:<address> x",
		"addr dead:10.0.0.1 x":                           "addr dead:<address> x",
		"read tcp 192.168.1.5:5000->10.0.0.1:445: reset": "read tcp <address>:5000-><address>:445: reset",
		"label xdead::1 here":                            "label xdead::1 here",
		"std::string and 12:30 and 1.2 and v3.4.5":       "std::string and 12:30 and 1.2 and v3.4.5",
	} {
		require.Equal(t, want, scrubResolvedAddrs(in, host), in)
	}
}

// Review M3(b): the configured host stays in any spelling of the same address.
func TestSECDIALFU_F4_ScrubKeepsConfiguredHostInAnySpelling(t *testing.T) {
	require.Equal(t, "dial tcp [FD12::5]:445 and [fd12::5]:445", scrubResolvedAddrs("dial tcp [FD12::5]:445 and [fd12::5]:445", "fd12::0005"))
	require.Equal(t, "to 192.168.1.20 via ::ffff:192.168.1.20", scrubResolvedAddrs("to 192.168.1.20 via ::ffff:192.168.1.20", "192.168.1.20"))
	require.Equal(t, "to <address> via <address>", scrubResolvedAddrs("to 10.0.0.9 via ::ffff:10.0.0.9", "192.168.1.20"))
	require.Equal(t, "dial tcp 192.168.1.20 and <address>", scrubResolvedAddrs("dial tcp 192.168.1.20 and FD12::5", "::ffff:192.168.1.20"))
}

// Review M1: the scrubbed wrapper keeps Timeout() / Temporary() of the cause so net/http, url.Error and os.IsTimeout still classify it.
func TestSECDIALFU_F4_ScrubbedErrorKeepsTimeoutSemantics(t *testing.T) {
	cause := &net.OpError{Op: "dial", Net: "tcp", Addr: &net.TCPAddr{IP: net.ParseIP("10.9.8.7"), Port: 445}, Err: os.ErrDeadlineExceeded}
	require.True(t, cause.Timeout())
	got := scrubDialErr(cause, "nas.lan")
	require.NotContains(t, got.Error(), "10.9.8.7")
	te, ok := got.(interface{ Timeout() bool })
	require.True(t, ok, "wrapper exposes Timeout()")
	require.True(t, te.Timeout())
	require.True(t, os.IsTimeout(got))
	require.True(t, (&url.Error{Op: "Get", URL: "http://nas.lan/", Err: got}).Timeout())
	var ne net.Error
	require.True(t, errors.As(got, &ne))
	// a non-timeout cause stays a non-timeout
	got2 := scrubDialErr(&net.OpError{Op: "dial", Net: "tcp", Addr: &net.TCPAddr{IP: net.ParseIP("10.9.8.7"), Port: 445}, Err: syscall.ECONNREFUSED}, "nas.lan")
	require.False(t, os.IsTimeout(got2))
	require.ErrorIs(t, got2, syscall.ECONNREFUSED)
}
