package services

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
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

func TestVetSMBHost_RoundTwoBlocklist(t *testing.T) {
	withResolver(t, map[string][]string{
		"sixtofour-loop.example": {"2002:7f00:1::"},
		"sixtofour-meta.example": {"2002:a9fe:a9fe::"},
		"siit-loop.example":      {"::ffff:0:127.0.0.1"},
		"nat64-local.example":    {"64:ff9b:1::1"},
		"bcast.example":          {"255.255.255.255"},
		"cgnat-lo.example":       {"100.64.0.1"},
		"cgnat-hi.example":       {"100.127.255.254"},
		"tailscale.example":      {"100.100.1.1"},
		"sixtofour-ok.example":   {"2002:c000:204::"},
		"lan10.example":          {"10.1.2.3"},
		"lan192.example":         {"192.168.1.5"},
		"below-cgnat.example":    {"100.63.255.255"},
		"above-cgnat.example":    {"100.128.0.1"},
	})
	for _, h := range []string{"sixtofour-loop.example", "sixtofour-meta.example", "siit-loop.example", "nat64-local.example", "bcast.example", "255.255.255.255", "100.100.100.200"} {
		require.Error(t, VetSMBHost(h), h)
	}
	for _, h := range []string{"sixtofour-ok.example", "lan10.example", "lan192.example", "below-cgnat.example", "above-cgnat.example", "cgnat-lo.example", "cgnat-hi.example", "tailscale.example", "100.64.0.1", "100.100.1.1", "100.127.255.254", "10.0.0.5", "192.168.0.10"} {
		require.NoError(t, VetSMBHost(h), "negative control (QA LAN and Tailscale/CGNAT stay allowed, operator decision 2026-11-11): "+h)
	}
}

func TestVetSMBHost_EmptyAnswerAndLongNameRefused(t *testing.T) {
	old := SMBHostResolver
	t.Cleanup(func() { SMBHostResolver = old })
	SMBHostResolver = func(string) ([]net.IP, error) { return []net.IP{}, nil }
	require.Error(t, VetSMBHost("empty.example"), "empty answer with nil error is refused (N3)")

	SMBHostResolver = func(string) ([]net.IP, error) { return []net.IP{net.ParseIP("192.0.2.10")}, nil }
	long254 := strings.Repeat("a.", 127)
	long253 := strings.Repeat("a.", 126) + "a"
	require.Len(t, long254, 254)
	require.Len(t, long253, 253)
	require.Error(t, VetSMBHost(long254), "254 chars refused (N10)")
	require.NoError(t, VetSMBHost(long253), "253 chars allowed (negative control)")
}

func TestSMBAllowedPorts_InvalidValueWarns(t *testing.T) {
	oldW, oldR := smbWarnf, smbWarnedRaw
	t.Cleanup(func() { smbWarnf, smbWarnedRaw = oldW, oldR })
	var msgs []string
	smbWarnf = func(f string, a ...any) { msgs = append(msgs, fmt.Sprintf(f, a...)) }
	smbWarnedRaw = ""

	t.Setenv("CATALOG_SMB_ALLOWED_PORTS", "445,abc")
	require.Equal(t, []int{445}, SMBAllowedPorts())
	require.Len(t, msgs, 1)
	require.Contains(t, msgs[0], "CATALOG_SMB_ALLOWED_PORTS")
	require.Contains(t, msgs[0], "445")
	_ = SMBAllowedPorts()
	require.Len(t, msgs, 1, "the same invalid value warns once, not on every request")

	msgs = nil
	smbWarnedRaw = ""
	t.Setenv("CATALOG_SMB_ALLOWED_PORTS", "1445")
	require.Equal(t, []int{445, 1445}, SMBAllowedPorts())
	t.Setenv("CATALOG_SMB_ALLOWED_PORTS", "")
	_ = SMBAllowedPorts()
	require.Empty(t, msgs, "valid or empty value does not warn (negative control)")
}
