package services

import (
	"net"
	"testing"

	"catalogizer/internal/netguard"

	"github.com/stretchr/testify/require"
)

// reservedProbes are addresses inside every netguard always-refused reserved range, in plain, IPv4-mapped and
// NAT64 / 6to4 / IPv4-compatible / SIIT spellings where the range is IPv4.
func reservedProbes() []string {
	return []string{
		"198.18.0.1", "198.19.255.254", "192.0.0.8", "240.0.0.1", "255.255.255.254",
		"fec0::1", "feff::1", "2001::1", "2001:0:4136:e378:8000:63bf:3fff:fdd2",
		"::ffff:198.18.0.1", "::ffff:192.0.0.8", "::ffff:240.0.0.1",
		"64:ff9b::c612:1",    // NAT64 of 198.18.0.1
		"2002:c612:1::",      // 6to4 of 198.18.0.1
		"::c612:1",           // IPv4-compatible 198.18.0.1
		"::ffff:0:c612:1",    // SIIT of 198.18.0.1
		"64:ff9b::c000:8",    // NAT64 of 192.0.0.8
		"2002:c000:8::",      // 6to4 of 192.0.0.8
		"64:ff9b::f000:1",    // NAT64 of 240.0.0.1
		"2002:f000:1::",      // 6to4 of 240.0.0.1
		"64:ff9b::a83f:8110", // NAT64 of 168.63.129.16 (Azure)
	}
}

func TestSMBHostVet_RefusesNetguardReservedRanges(t *testing.T) {
	for _, s := range reservedProbes() {
		ip := net.ParseIP(s)
		require.NotNil(t, ip, s)
		require.Equal(t, netguard.ClassNever, netguard.ClassOf(ip), "control: netguard operator class refuses %s", s)
		require.True(t, smbIPBlocked(ip), "smb vet must refuse %s", s)
		withResolver(t, map[string][]string{"res.example": {s}})
		require.Error(t, VetSMBHost("res.example"), "resolver path %s", s)
	}
}

// Every metadata singleton also in NAT64 / 6to4 / IPv4-compatible / SIIT spellings (100.100.100.200 = 6464:64c8).
func TestSMBHostVet_MetadataSingletonEmbeddedSpellings(t *testing.T) {
	for _, s := range []string{
		"64:ff9b::6464:64c8", "2002:6464:64c8::", "::6464:64c8", "::ffff:0:6464:64c8", "::ffff:100.100.100.200",
		"64:ff9b::a9fe:a9fe", // 169.254.169.254
	} {
		ip := net.ParseIP(s)
		require.NotNil(t, ip, s)
		require.True(t, netguard.IsBlockedIPStrict(ip), "control: netguard refuses %s", s)
		require.True(t, smbIPBlocked(ip), s)
	}
}

// Drift guard: every net netguard exports as reserved is refused by the SMB vet (first, last and a middle address).
func TestSMBHostVet_NoDriftFromNetguardReservedList(t *testing.T) {
	list := netguard.ReservedNets()
	require.GreaterOrEqual(t, len(list), 6, "control needle: list must hold the reserved nets")
	for _, n := range list {
		for _, ip := range netAddrs(n) {
			require.True(t, smbIPBlocked(ip), "reserved %s (in %s) must be refused", ip, n)
		}
	}
}

func netAddrs(n *net.IPNet) []net.IP {
	first := append(net.IP(nil), n.IP...)
	last := append(net.IP(nil), n.IP...)
	for i := range last {
		last[i] |= ^n.Mask[i]
	}
	return []net.IP{first, last}
}

// Negative controls: LAN SMB servers and ordinary public / documentation addresses stay allowed.
func TestSMBHostVet_ReservedFixStillAllowsLAN(t *testing.T) {
	for _, s := range []string{"10.1.2.3", "172.16.5.5", "192.168.1.5", "fd12:3456::1", "100.64.0.1", "8.8.8.8", "198.17.255.255", "198.20.0.1", "192.0.1.1", "223.255.255.254", "2001:db8::1"} {
		ip := net.ParseIP(s)
		require.NotNil(t, ip, s)
		require.False(t, smbIPBlocked(ip), s)
	}
}

func TestNetguard_ReservedNetsIsACopy(t *testing.T) {
	a := netguard.ReservedNets()
	for _, n := range a {
		for i := range n.Mask {
			n.Mask[i] = 0
		}
		for i := range n.IP {
			n.IP[i] = 0
		}
	}
	require.True(t, netguard.IsReservedRange(net.ParseIP("198.18.0.1")), "mutating the returned list must not weaken the guard")
	require.True(t, smbIPBlocked(net.ParseIP("198.18.0.1")))
}

func TestNetguard_MetadataSingletonsIsACopy(t *testing.T) {
	a := netguard.MetadataSingletons()
	for _, n := range a {
		n.IP[len(n.IP)-1] ^= 0xff
		n.Mask[0] = 0
	}
	require.True(t, netguard.IsMetadataSingleton(net.ParseIP("100.100.100.200")))
	require.True(t, netguard.IsBlockedIPStrict(net.ParseIP("fd20:ce::254")))
	require.True(t, smbIPBlocked(net.ParseIP("fd00:ec2::254")))
}
