package services

import (
	"net"
	"testing"

	"catalogizer/internal/netguard"

	"github.com/stretchr/testify/require"
)

// Metadata singletons live in one list (netguard); smb_hostvet must refuse every one of them in every parseable spelling.
func TestSMBHostVet_RefusesEveryNetguardMetadataSingleton(t *testing.T) {
	list := netguard.MetadataSingletons()
	require.NotEmpty(t, list, "control needle: the shared list must not be empty")
	seen := map[string]bool{}
	for _, n := range list {
		ip := n.IP
		seen[ip.String()] = true
		require.True(t, netguard.IsBlockedIPStrict(ip), "netguard refuses %s", ip)
		require.True(t, smbIPBlocked(ip), "smb_hostvet must refuse %s", ip)
		for _, spelling := range spellings(ip) {
			parsed := net.ParseIP(spelling)
			require.NotNil(t, parsed, spelling)
			require.True(t, smbIPBlocked(parsed), "spelling %q", spelling)
			// resolver path: a name that resolves to it is refused
			withResolver(t, map[string][]string{"meta.example": {spelling}})
			require.Error(t, VetSMBHost("meta.example"), "resolved %q", spelling)
		}
	}
	for _, want := range []string{"fd00:ec2::254", "fd20:ce::254", "100.100.100.200"} {
		require.True(t, seen[want], "expected singleton %s in the shared list", want)
	}
}

func spellings(ip net.IP) []string {
	s := ip.String()
	out := []string{s}
	if ip.To4() == nil {
		out = append(out, "FD20:CE::254", "fd20:0ce:0:0:0:0:0:254", "FD00:EC2:0:0:0:0:0:254")
		// only keep spellings that parse to this ip
		var keep []string
		for _, c := range out {
			if p := net.ParseIP(c); p != nil && p.Equal(ip) {
				keep = append(keep, c)
			}
		}
		return keep
	}
	return append(out, "::ffff:"+s) // IPv4-mapped
}

func TestSMBHostVet_MetadataSingletonsAreNotAllowListable(t *testing.T) {
	require.Error(t, VetSMBHost("fd20:ce::254"), "literal with colons fails syntax")
	require.Error(t, VetSMBHost("[fd20:ce::254]"))
	require.Error(t, VetSMBHost("fd20:ce::254%eth0"))
}
