package filesystem

import (
	"context"
	"errors"
	"net"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"catalogizer/internal/netguard"

	"github.com/stretchr/testify/require"
)

// SEC-DIAL-1 reproduction (unit-class evidence, §11.4.289): the REAL SmbClient / FTPClient code dials a host the operator never vetted. The
// local listener is only the observation point (it counts accepted TCP connections); the client under test is the production code.
func secDialListener(t *testing.T) (port int, accepted *int32) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	var n int32
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			atomic.AddInt32(&n, 1)
			_ = c.Close()
		}
	}()
	t.Cleanup(func() { _ = ln.Close() })
	return ln.Addr().(*net.TCPAddr).Port, &n
}

func TestSECDIAL1_SMBConnectDoesNotDialLoopback(t *testing.T) {
	t.Setenv(netguard.AllowCIDRsEnv, "")
	port, accepted := secDialListener(t)
	c := NewSmbClient(&SmbConfig{Host: "127.0.0.1", Port: port, Share: "share", Username: "u", Password: "p"})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	require.Error(t, c.Connect(ctx))
	time.Sleep(100 * time.Millisecond)
	require.Zero(t, atomic.LoadInt32(accepted), "SMB client dialled 127.0.0.1:"+strconv.Itoa(port))
}

func TestSECDIAL1_FTPConnectDoesNotDialLoopback(t *testing.T) {
	t.Setenv(netguard.AllowCIDRsEnv, "")
	port, accepted := secDialListener(t)
	c := NewFTPClient(&FTPConfig{Host: "127.0.0.1", Port: port, Username: "u", Password: "p"})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	require.Error(t, c.Connect(ctx))
	time.Sleep(100 * time.Millisecond)
	require.Zero(t, atomic.LoadInt32(accepted), "FTP client dialled 127.0.0.1:"+strconv.Itoa(port))
}

// Every refused host fails at connect time with netguard.ErrBlocked, before any network I/O, for both dialling clients.
func TestSECDIAL1_ClientsRefuseReservedHostsAtDial(t *testing.T) {
	t.Setenv(netguard.AllowCIDRsEnv, "")
	for _, host := range []string{"127.0.0.1", "100.100.100.200", "168.63.129.16", "fd20:ce::254", "198.18.0.1", "169.254.169.254", "fe80::1", "::ffff:127.0.0.1"} {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		serr := NewSmbClient(&SmbConfig{Host: host, Port: 445, Share: "s"}).Connect(ctx)
		ferr := NewFTPClient(&FTPConfig{Host: host, Port: 21}).Connect(ctx)
		cancel()
		require.ErrorIs(t, serr, netguard.ErrBlocked, "smb %s", host)
		require.ErrorIs(t, ferr, netguard.ErrBlocked, "ftp %s", host)
	}
}

// Control: with the operator's allow-list naming the loopback the same real clients DO dial (the policy is the shared netguard one, not a
// blanket refusal), so the refusals above are the policy and not a broken client.
func TestSECDIAL1_AllowListedHostStillDialled(t *testing.T) {
	t.Setenv(netguard.AllowCIDRsEnv, "127.0.0.0/8")
	port, accepted := secDialListener(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = NewSmbClient(&SmbConfig{Host: "127.0.0.1", Port: port, Share: "s"}).Connect(ctx)
	_ = NewFTPClient(&FTPConfig{Host: "127.0.0.1", Port: port}).Connect(ctx)
	time.Sleep(100 * time.Millisecond)
	require.EqualValues(t, 2, atomic.LoadInt32(accepted))
}

// Both clients hand the NAME to the operator-class dialer (which resolves, checks and pins the address); they never resolve it themselves.
func TestSECDIAL1_ClientsDialThroughSharedDialerWithName(t *testing.T) {
	var got []string
	old := storageDialContext
	storageDialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		got = append(got, addr)
		return nil, errors.New("recorded")
	}
	defer func() { storageDialContext = old }()
	ctx := context.Background()
	_ = NewSmbClient(&SmbConfig{Host: "nas.lan", Port: 445, Share: "s"}).Connect(ctx)
	_ = NewFTPClient(&FTPConfig{Host: "nas.lan", Port: 2121}).Connect(ctx)
	require.Equal(t, []string{"nas.lan:445", "nas.lan:2121"}, got)
}

func withNFSSeams(t *testing.T, resolve func(ctx context.Context, host string) ([]net.IP, error)) *[][3]string {
	t.Helper()
	var mounts [][3]string
	oldM, oldR := nfsMount, storageOperatorResolve
	nfsMount = func(source, target, fstype string, flags uintptr, data string) error {
		mounts = append(mounts, [3]string{source, target, data})
		return nil
	}
	storageOperatorResolve = resolve
	t.Cleanup(func() { nfsMount, storageOperatorResolve = oldM, oldR })
	return &mounts
}

func realResolve() func(ctx context.Context, host string) ([]net.IP, error) {
	return netguard.OperatorResolve
}

func nfsClient(t *testing.T, host, export, opts string) *NFSClient {
	c, err := NewNFSClient(NFSConfig{Host: host, Path: export, MountPoint: t.TempDir() + "/mnt", Options: opts})
	require.NoError(t, err)
	return c
}

func TestSECDIAL1_NFSMountRefusesReservedHosts(t *testing.T) {
	t.Setenv(netguard.AllowCIDRsEnv, "")
	mounts := withNFSSeams(t, realResolve())
	for _, host := range []string{"127.0.0.1", "100.100.100.200", "168.63.129.16", "fd20:ce::254", "198.18.0.1", "169.254.169.254", "fe80::1", "::1"} {
		err := nfsClient(t, host, "/export", "").Connect(context.Background())
		require.ErrorIs(t, err, ErrStorageHostRefused, host)
	}
	require.Empty(t, *mounts, "no mount(2) call for a refused host")
}

func TestSECDIAL1_NFSMountUsesVettedAddressOnly(t *testing.T) {
	t.Setenv(netguard.AllowCIDRsEnv, "")
	mounts := withNFSSeams(t, realResolve())
	require.NoError(t, nfsClient(t, "192.168.1.20", "/export", "").Connect(context.Background()))
	require.Equal(t, "192.168.1.20:/export", (*mounts)[0][0])
	require.Equal(t, "vers=3,addr=192.168.1.20", (*mounts)[0][2])
	require.NoError(t, nfsClient(t, "fd12:3456::1", "/e", "ro").Connect(context.Background()))
	require.Equal(t, "[fd12:3456::1]:/e", (*mounts)[1][0])
}

// DNS rebinding: the name resolves to a public address when the root is created and to the metadata address when the mount happens. The
// mount re-resolves, re-vets and refuses; and when the name still resolves to a vetted address, the kernel is given THAT address, not the name.
func TestSECDIAL1_NFSDNSRebindingRefusedAndAddressPinned(t *testing.T) {
	t.Setenv(netguard.AllowCIDRsEnv, "")
	answers := [][]net.IP{{net.ParseIP("93.184.216.34")}, {net.ParseIP("169.254.169.254")}}
	calls := 0
	mounts := withNFSSeams(t, func(ctx context.Context, host string) ([]net.IP, error) {
		a := answers[calls]
		calls++
		return netguard.OperatorResolve(ctx, a[0].String()) // vetting is the real policy on the answer of this call
	})
	_, err := storageOperatorResolve(context.Background(), "rebind.example") // creation-time vet: public, passes
	require.NoError(t, err)
	err = nfsClient(t, "rebind.example", "/export", "").Connect(context.Background()) // mount-time: metadata, refused
	require.ErrorIs(t, err, ErrStorageHostRefused)
	require.Empty(t, *mounts)

	// stable name: the vetted address, not the name, reaches mount(2)
	mounts2 := withNFSSeams(t, func(ctx context.Context, host string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("192.168.7.7")}, nil
	})
	require.NoError(t, nfsClient(t, "nas.lan", "/export", "").Connect(context.Background()))
	require.Equal(t, "192.168.7.7:/export", (*mounts2)[0][0])
	require.NotContains(t, (*mounts2)[0][2], "nas.lan")
}

func TestSECDIAL1_NFSOptionInjectionAndAddrOverrideRefused(t *testing.T) {
	t.Setenv(netguard.AllowCIDRsEnv, "")
	mounts := withNFSSeams(t, realResolve())
	for _, opts := range []string{"vers=3,addr=127.0.0.1", "vers=3,mountaddr=169.254.169.254", "addr=nas.lan", "vers=3\n,addr=1.2.3.4", "vers=3 ro", "-o x", "vers=3,,ro", "a=", "=b", "vers=3;x"} {
		require.Error(t, nfsClient(t, "192.168.1.20", "/export", opts).Connect(context.Background()), opts)
	}
	// SEC-DIAL-1-FU F1: space and comma are legal in a share name; only control characters and a leading dash are refused
	for _, exp := range []string{"/e\nx", "/e\x00x", "-o", ""} {
		require.Error(t, nfsClient(t, "192.168.1.20", exp, "").Connect(context.Background()), exp)
	}
	for _, host := range []string{"nas,addr=1", "-x", "a b", "nas:/x", "nas\n"} {
		require.Error(t, nfsClient(t, host, "/export", "").Connect(context.Background()), host)
	}
	require.Empty(t, *mounts)
	// a user addr= naming another LAN host is vetted and kept
	require.NoError(t, nfsClient(t, "192.168.1.20", "/export", "vers=3,addr=192.168.1.21").Connect(context.Background()))
	require.Equal(t, "vers=3,addr=192.168.1.21", (*mounts)[0][2])
}

func TestSECDIAL1_ValidateRootRefusesMalformedHosts(t *testing.T) {
	for _, h := range []string{"a,b", "-x", "a b", "a:445", "u@h", "[::1]", "h/x", "h\n"} {
		require.Error(t, ValidateSettings("smb", map[string]interface{}{"host": h, "share": "s"}), h)
		require.Error(t, ValidateSettings("ftp", map[string]interface{}{"host": h}), h)
	}
	for _, h := range []string{"nas.lan", "NAS_1", "192.168.1.2", "fd12::1", "a-b.c"} {
		require.NoError(t, ValidateSettings("ftp", map[string]interface{}{"host": h}), h)
	}
}

// The CopyFile tracker and the connection group dial control AND data connections: a data connection aimed at a refused address (a PASV
// answer naming the metadata service, FTP bounce) is refused too, and an allowed one reaches the shared dialer with the name.
func TestSECDIAL1_FTPDataAndTrackerDialsAreVetted(t *testing.T) {
	t.Setenv(netguard.AllowCIDRsEnv, "")
	ctx := context.Background()
	tr := &connTracker{}
	_, err := tr.dialFunc(ctx)("tcp", "169.254.169.254:21")
	require.ErrorIs(t, err, netguard.ErrBlocked)
	g := &ftpConnGroup{}
	df := g.dialFunc(ctx)
	_, err = df("tcp", "100.100.100.200:21") // control connection
	require.ErrorIs(t, err, netguard.ErrBlocked)
	_, err = df("tcp", "127.0.0.1:50000") // data connection (second dial of the group)
	require.ErrorIs(t, err, netguard.ErrBlocked)

	var got []string
	old := storageDialContext
	storageDialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		got = append(got, addr)
		return nil, errors.New("recorded")
	}
	defer func() { storageDialContext = old }()
	_, _ = (&connTracker{}).dialFunc(ctx)("tcp", "nas.lan:21")
	g2 := &ftpConnGroup{}
	_, _ = g2.dialFunc(ctx)("tcp", "nas.lan:21")
	_, _ = g2.dialFunc(ctx)("tcp", "192.168.1.9:50001")
	require.Equal(t, []string{"nas.lan:21", "nas.lan:21", "192.168.1.9:50001"}, got)
}
