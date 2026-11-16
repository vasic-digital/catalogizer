package handlers

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

// ENV-F1: unit tests of the helper that keeps root-creation tests independent of the machine they run on (the policy refuses the server's own
// addresses). The owned-address list is given explicitly, so these run the same everywhere.
func TestFirstUnownedAddr(t *testing.T) {
	owned := []net.Addr{
		&net.IPNet{IP: net.ParseIP("192.168.1.115"), Mask: net.CIDRMask(24, 32)},
		&net.IPAddr{IP: net.ParseIP("10.0.0.5")},
		&net.IPNet{IP: net.ParseIP("::ffff:172.16.3.4"), Mask: net.CIDRMask(120, 128)}, // 16-byte form of an IPv4 address
		&net.IPNet{IP: net.ParseIP("fd12:3456::1"), Mask: net.CIDRMask(64, 128)},
	}
	got, ok := firstUnownedAddr(owned, []string{"192.168.1.115", "10.0.0.5", "172.16.3.4", "10.9.9.9"})
	require.True(t, ok)
	require.Equal(t, "10.9.9.9", got, "owned addresses must be skipped whatever their Addr type or byte length")
	got, ok = firstUnownedAddr(owned, []string{"fd12:3456::1", "fd12:3456::0001", "fd77::2"})
	require.True(t, ok)
	require.Equal(t, "fd77::2", got, "an IPv6 address is compared by value, not by its spelling")
	_, ok = firstUnownedAddr(owned, []string{"192.168.1.115", "10.0.0.5"})
	require.False(t, ok, "no free candidate must be reported, never an owned address returned")
}
