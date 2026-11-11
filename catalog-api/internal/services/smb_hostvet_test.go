package services

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func withResolver(t *testing.T, m map[string][]string) {
	t.Helper()
	old := SMBHostResolver
	t.Cleanup(func() { SMBHostResolver = old })
	SMBHostResolver = func(h string) ([]net.IP, error) {
		v, ok := m[h]
		if !ok {
			return nil, errors.New("no such host")
		}
		var out []net.IP
		for _, s := range v {
			out = append(out, net.ParseIP(s))
		}
		return out, nil
	}
}

func TestVetSMBHost_RefusesBracketAndPortForms(t *testing.T) {
	for _, h := range []string{"[::1]", "[::ffff:127.0.0.1]", "[0:0:0:0:0:0:0:1]", "::1", "127.0.0.1:445", "user@host", "ho st", "host/x", "", " ", "a\tb", "host\\x", "nas%eth0"} {
		require.Error(t, ValidateSMBHostSyntax(h), "%q syntax", h)
		require.Error(t, VetSMBHost(h), "%q vet", h)
		require.True(t, errors.Is(VetSMBHost(h), ErrInvalidSMBInput), h)
	}
}

func TestVetSMBHost_BlocklistAndResolver(t *testing.T) {
	withResolver(t, map[string][]string{
		"loop.example":   {"127.0.0.1"},
		"mixed.example":  {"192.0.2.10", "127.0.0.1"},
		"mixed6.example": {"192.0.2.10", "::1"},
		"meta.example":   {"169.254.169.254"},
		"ali.example":    {"100.100.100.200"},
		"az.example":     {"168.63.129.16"},
		"ec2v6.example":  {"fd00:ec2::254"},
		"nat64.example":  {"64:ff9b::7f00:1"},
		"compat.example": {"::127.0.0.1"},
		"mapped.example": {"::ffff:127.0.0.1"},
		"good.example":   {"192.0.2.10", "198.51.100.7"},
		"zero.example":   {"0.1.2.3"},
	})
	for _, h := range []string{"loop.example", "mixed.example", "mixed6.example", "meta.example", "ali.example", "az.example", "ec2v6.example", "nat64.example", "compat.example", "mapped.example", "zero.example", "127.0.0.1", "169.254.169.254", "100.100.100.200", "0.0.0.0", "224.0.0.1"} {
		require.Error(t, VetSMBHost(h), h)
	}
	require.Error(t, VetSMBHost("unresolvable.example"), "lookup failure is refused")
	require.NoError(t, VetSMBHost("good.example"), "negative control: all-public A records")
	require.NoError(t, VetSMBHost("192.0.2.10"), "negative control: LAN/public IPv4 literal")
	require.NoError(t, VetSMBHost("192.168.0.5"), "negative control: RFC1918 stays allowed")
}

func TestSMBService_DiscoverAndProbeRefuseBracketHostsWithoutDial(t *testing.T) {
	s := NewSMBDiscoveryService(zap.NewNop())
	for _, h := range []string{"[::1]", "[::ffff:127.0.0.1]", "[0:0:0:0:0:0:0:1]"} {
		_, err := s.DiscoverShares(context.Background(), h, "u", "p", nil)
		require.True(t, errors.Is(err, ErrInvalidSMBInput), "DiscoverShares %q: %v", h, err)
		_, err = s.ProbeHostWithIdentities(context.Background(), h, []SMBIdentity{{Kind: "guest"}})
		require.True(t, errors.Is(err, ErrInvalidSMBInput), "Probe %q: %v", h, err)
	}
}

func TestSMBAllowedPorts_DefaultEnvAndFailClosed(t *testing.T) {
	t.Setenv("CATALOG_SMB_ALLOWED_PORTS", "")
	require.Equal(t, []int{445}, SMBAllowedPorts())
	require.Error(t, ValidateSMBPort(22), "default refuses 22")
	require.NoError(t, ValidateSMBPort(445))

	t.Setenv("CATALOG_SMB_ALLOWED_PORTS", "1445, 2445")
	require.NoError(t, ValidateSMBPort(1445))
	require.NoError(t, ValidateSMBPort(445), "445 stays allowed")
	require.Error(t, ValidateSMBPort(22), "env does not open unlisted ports")

	for _, bad := range []string{"abc", "0", "65536", "445,x", "-1", "445,,22x"} {
		t.Setenv("CATALOG_SMB_ALLOWED_PORTS", bad)
		require.Equal(t, []int{445}, SMBAllowedPorts(), "invalid %q fails closed to default", bad)
		require.Error(t, ValidateSMBPort(1445), bad)
	}
}

func TestFilterShareInfos_DropsAllAdminShares(t *testing.T) {
	got := filterShareInfos("h", []string{"Data", "IPC$", "C$", "ADMIN$", "d$", "Projects", "print$"})
	var names []string
	for _, g := range got {
		names = append(names, g.ShareName)
	}
	require.Equal(t, []string{"Data", "Projects", "print$"}, names, "same policy as SplitShare; print$ is not administrative there")
}

func TestBrowseShare_ReadDirErrorIsExactlyGeneric(t *testing.T) {
	m := newMem()
	_, err := discSvc(m, nil).BrowseShare(context.Background(), discCfg("Public"), "no/such/dir")
	require.EqualError(t, err, "failed to read directory", "no path and no cause in the client-visible error")
}
