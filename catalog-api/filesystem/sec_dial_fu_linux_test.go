//go:build linux

package filesystem

import (
	"context"
	"errors"
	"net"
	"syscall"
	"testing"

	"catalogizer/internal/netguard"

	"github.com/stretchr/testify/require"
)

// SEC-DIAL-1-FU (unit-class, §11.4.289): the mount(2) / resolver seams are test doubles; the production planning code is real.

// F1: an export with a space (a normal NAS share name) must be mountable; the source is handed to mount(2) as ONE string, never option-parsed.
func TestSECDIALFU_F1_ExportWithSpaceMounts(t *testing.T) {
	t.Setenv(netguard.AllowCIDRsEnv, "")
	mounts := withNFSSeams(t, realResolve())
	require.NoError(t, nfsClient(t, "192.168.1.20", "/volume1/My Videos", "").Connect(context.Background()))
	require.Equal(t, "192.168.1.20:/volume1/My Videos", (*mounts)[0][0])
	require.Equal(t, "vers=3,addr=192.168.1.20", (*mounts)[0][2], "the export never reaches the option string")
	require.NoError(t, ValidateSettings("nfs", map[string]interface{}{"host": "nas.lan", "path": "/volume1/My Videos"}))
}

// F1 control: the control characters and a leading '-' stay refused (newline / NUL / TAB / DEL), and nothing reaches mount(2).
func TestSECDIALFU_F1_ControlCharsAndDashStillRefused(t *testing.T) {
	t.Setenv(netguard.AllowCIDRsEnv, "")
	mounts := withNFSSeams(t, realResolve())
	for _, exp := range []string{"/e\nx", "/e\x00x", "/e\tx", "/e\x7fx", "/e\rx", "-o", "-", ""} {
		require.Error(t, nfsClient(t, "192.168.1.20", exp, "").Connect(context.Background()), "%q", exp)
		require.Error(t, ValidateNFSExport(exp), "%q", exp)
	}
	require.Empty(t, *mounts)
}

// F1: /proc/mounts escapes a space of the source as \040; the reader must still recognise the mount at a mount point containing a space.
func TestSECDIALFU_F1_ProcMountsEscapedSpaces(t *testing.T) {
	m := "192.168.1.20:/volume1/My\\040Videos /mnt/my\\040videos nfs rw,vers=3 0 0\n"
	require.True(t, mountsContainNFS(m, "/mnt/my videos"))
	require.False(t, mountsContainNFS(m, "/mnt/other"))
}

// F4: a failed mount must not echo the resolved address (the status endpoint has no admin gate); the configured host is fine.
func TestSECDIALFU_F4_MountErrorDoesNotLeakResolvedIP(t *testing.T) {
	t.Setenv(netguard.AllowCIDRsEnv, "")
	withNFSSeams(t, func(ctx context.Context, host string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("192.168.7.7")}, nil
	})
	nfsMount = func(source, target, fstype string, flags uintptr, data string) error { return syscall.EPERM }
	err := nfsClient(t, "nas.lan", "/export", "").Connect(context.Background())
	require.Error(t, err)
	require.NotContains(t, err.Error(), "192.168.7.7")
	require.ErrorIs(t, err, syscall.EPERM, "the cause stays inspectable")
	require.Contains(t, err.Error(), "nas.lan")
}

// F5 RM-A: a user mountaddr= is NOT addr=; the vetted addr= is still appended.
func TestSECDIALFU_F5_MountaddrDoesNotSuppressAddr(t *testing.T) {
	t.Setenv(netguard.AllowCIDRsEnv, "")
	mounts := withNFSSeams(t, realResolve())
	require.NoError(t, nfsClient(t, "192.168.1.20", "/export", "vers=3,mountaddr=192.168.1.21").Connect(context.Background()))
	require.Equal(t, "vers=3,mountaddr=192.168.1.21,addr=192.168.1.20", (*mounts)[0][2])
}

// F5: addr= is vetted at mount time too (kept as written when allowed, refused when not): both seams stay pinned.
func TestSECDIALFU_F5_UserAddrKeptWhenVetted(t *testing.T) {
	t.Setenv(netguard.AllowCIDRsEnv, "")
	mounts := withNFSSeams(t, realResolve())
	err := nfsClient(t, "192.168.1.20", "/export", "addr=169.254.169.254").Connect(context.Background())
	require.ErrorIs(t, err, ErrStorageHostRefused)
	require.Empty(t, *mounts)
	_ = errors.New
}
