package filesystem

import (
	"context"
	"errors"
	"net"
	"net/http"
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
	start := time.Now()
	_, err := dialStorage(context.Background(), "nas.lan", "445", 80*time.Millisecond)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Less(t, time.Since(start), 2*time.Second)
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
