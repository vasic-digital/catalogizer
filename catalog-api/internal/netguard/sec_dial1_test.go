package netguard

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// SEC-DIAL-1: the dialer the smb and ftp storage connectors use (OperatorDialContext) connects to the address it checked, never to the name:
// the first lookup answers a LAN address, a later lookup (the DNS-rebinding answer) the metadata address, and only the first is dialled.
func TestSECDIAL1_OperatorDialPinsCheckedAddressAgainstRebinding(t *testing.T) {
	t.Setenv(AllowCIDRsEnv, "")
	oldL, oldD := lookupIPAddr, dialTCP
	defer func() { lookupIPAddr, dialTCP = oldL, oldD }()
	n := 0
	lookupIPAddr = func(ctx context.Context, host string) ([]net.IPAddr, error) {
		n++
		if n == 1 {
			return []net.IPAddr{{IP: net.ParseIP("192.168.5.5")}}, nil
		}
		return []net.IPAddr{{IP: net.ParseIP("100.100.100.200")}}, nil
	}
	var dialed []string
	dialTCP = func(ctx context.Context, d *net.Dialer, network, ip, port string) (net.Conn, error) {
		dialed = append(dialed, ip+":"+port)
		return nil, net.ErrClosed
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, _ = OperatorDialContext(ctx, "tcp", "rebind.example:21")
	require.Equal(t, []string{"192.168.5.5:21"}, dialed)
	require.Equal(t, 1, n, "one lookup per dial")
	// the next dial sees the rebound answer and is refused before any connection
	dialed = nil
	_, err := OperatorDialContext(ctx, "tcp", "rebind.example:21")
	require.ErrorIs(t, err, ErrBlocked)
	require.Empty(t, dialed)
}

func TestSECDIAL1_CheckOperatorHost(t *testing.T) {
	t.Setenv(AllowCIDRsEnv, "")
	ctx := context.Background()
	for _, h := range []string{"127.0.0.1", "100.100.100.200", "168.63.129.16", "fd20:ce::254", "198.18.0.1", "169.254.169.254", "fe80::1", "0177.0.0.1"} {
		require.Error(t, CheckOperatorHost(ctx, h), h)
	}
	for _, h := range []string{"192.168.1.20", "10.1.2.3", "172.16.0.9", "100.64.0.1", "fd12::1", "93.184.216.34"} {
		require.NoError(t, CheckOperatorHost(ctx, h), h)
	}
}
