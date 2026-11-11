package services

import (
	"context"
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// SMB-FU-12 review round 2: the per-host port binding must hold at DIAL time on the vetted addresses, not only at pre-check time. Only the
// resolver and the dialer are stubbed; BrowseShare -> openConfined -> mountShare -> dialVetted are the real code.
func bindingRig(t *testing.T, answer func(n int, h string) []net.IP) *[]string {
	t.Helper()
	var mu sync.Mutex
	n := 0
	var dialed []string
	withSMBHostResolver(t, func(h string) ([]net.IP, error) {
		mu.Lock()
		defer mu.Unlock()
		if ip := net.ParseIP(h); ip != nil {
			return []net.IP{ip}, nil
		}
		n++
		return answer(n, h), nil
	})
	withSMBDialer(t, func(_ context.Context, addr string, _ time.Duration) (net.Conn, error) {
		mu.Lock()
		defer mu.Unlock()
		dialed = append(dialed, addr)
		return nil, errors.New("recorded, not dialed")
	})
	return &dialed
}

func bindingCfg(host string) SMBConnectionConfig {
	return SMBConnectionConfig{Host: host, Port: 1445, Share: "Public", Username: "u", Password: "p"}
}

func TestSMBPortBinding_RebindingNameNeverDialsOtherHost(t *testing.T) {
	t.Setenv("CATALOG_SMB_ALLOWED_PORTS", "")
	withSMBConfiguredRoots(t, []SMBRootPort{{Host: "192.168.1.10", Port: 1445}})
	dialed := bindingRig(t, func(n int, _ string) []net.IP {
		if n == 1 { // the pre-check sees the root address, every later lookup the other host
			return []net.IP{net.ParseIP("192.168.1.10")}
		}
		return []net.IP{net.ParseIP("192.168.1.99")}
	})
	_, err := rebindSvc().BrowseShare(context.Background(), bindingCfg("evil.lan"), ".")
	require.Error(t, err)
	require.Contains(t, err.Error(), "port not allowed", "refused by the dial-time binding")
	require.NotContains(t, *dialed, "192.168.1.99:1445", "rebound address must never be dialed on the configured port")
	require.Empty(t, *dialed)
}

func TestSMBPortBinding_MultiIPDialsOnlyRootAddress(t *testing.T) {
	t.Setenv("CATALOG_SMB_ALLOWED_PORTS", "")
	withSMBConfiguredRoots(t, []SMBRootPort{{Host: "192.168.1.10", Port: 1445}})
	dialed := bindingRig(t, func(int, string) []net.IP {
		return []net.IP{net.ParseIP("192.168.1.99"), net.ParseIP("192.168.1.10")}
	})
	_, _ = rebindSvc().BrowseShare(context.Background(), bindingCfg("rr.lan"), ".")
	require.Equal(t, []string{"192.168.1.10:1445"}, *dialed, "only the intersection with the root's address set is dialed")
}

func TestSMBPortBinding_EmptyIntersectionRefusedBeforeDial(t *testing.T) {
	t.Setenv("CATALOG_SMB_ALLOWED_PORTS", "")
	withSMBConfiguredRoots(t, []SMBRootPort{{Host: "192.168.1.10", Port: 1445}})
	dialed := bindingRig(t, func(int, string) []net.IP { return []net.IP{net.ParseIP("192.168.1.99")} })
	_, err := rebindSvc().BrowseShare(context.Background(), bindingCfg("other.lan"), ".")
	require.True(t, errors.Is(err, ErrInvalidSMBInput), "%v", err)
	require.Contains(t, err.Error(), "port not allowed", "refused by the binding itself, not by a side effect")
	require.Empty(t, *dialed)
}

func TestSMBPortBinding_GlobalPortKeepsAllVettedIPs(t *testing.T) {
	t.Setenv("CATALOG_SMB_ALLOWED_PORTS", "")
	withSMBConfiguredRoots(t, []SMBRootPort{{Host: "192.168.1.10", Port: 1445}})
	dialed := bindingRig(t, func(int, string) []net.IP {
		return []net.IP{net.ParseIP("192.168.1.99"), net.ParseIP("192.168.1.10")}
	})
	cfg := bindingCfg("rr.lan")
	cfg.Port = 445
	_, _ = rebindSvc().BrowseShare(context.Background(), cfg, ".")
	require.Equal(t, []string{"192.168.1.99:445", "192.168.1.10:445"}, *dialed, "445 is global: unchanged behaviour (negative control)")
}

func TestSMBPortBinding_SameNameDialsOnlyVettedIPs(t *testing.T) {
	t.Setenv("CATALOG_SMB_ALLOWED_PORTS", "")
	withSMBConfiguredRoots(t, []SMBRootPort{{Host: "nas.lan", Port: 1445}})
	dialed := bindingRig(t, func(int, string) []net.IP { return []net.IP{net.ParseIP("192.168.1.10")} })
	_, _ = rebindSvc().BrowseShare(context.Background(), bindingCfg("NAS.lan"), ".")
	require.Equal(t, []string{"192.168.1.10:1445"}, *dialed)
}

// F2: case-insensitive name match must hold on its own (the resolver gives no help).
func TestSMBPortForHost_CaseInsensitiveNameWithoutResolution(t *testing.T) {
	t.Setenv("CATALOG_SMB_ALLOWED_PORTS", "")
	withSMBConfiguredRoots(t, []SMBRootPort{{Host: "NAS.lan", Port: 1445}})
	withSMBHostResolver(t, func(string) ([]net.IP, error) { return nil, errors.New("nxdomain") })
	require.NoError(t, ValidateSMBPortForHost("nas.LAN", 1445))
	require.Error(t, ValidateSMBPortForHost("nas2.lan", 1445))
}

// Same name, different case, root name not resolvable: the dial-time binding must keep the vetted address (EqualFold, not ==).
func TestSMBPortBinding_SameNameCaseInsensitiveAtDial(t *testing.T) {
	t.Setenv("CATALOG_SMB_ALLOWED_PORTS", "")
	withSMBConfiguredRoots(t, []SMBRootPort{{Host: "nas.lan", Port: 1445}})
	var dialed []string
	withSMBHostResolver(t, func(h string) ([]net.IP, error) {
		if h == "NAS.lan" { // case-sensitive stub: the root spelling does not resolve
			return []net.IP{net.ParseIP("192.168.1.10")}, nil
		}
		return nil, errors.New("nxdomain")
	})
	withSMBDialer(t, func(_ context.Context, addr string, _ time.Duration) (net.Conn, error) {
		dialed = append(dialed, addr)
		return nil, errors.New("recorded, not dialed")
	})
	_, _ = rebindSvc().BrowseShare(context.Background(), bindingCfg("NAS.lan"), ".")
	require.Equal(t, []string{"192.168.1.10:1445"}, dialed)
}
