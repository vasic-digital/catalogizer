package handlers

import "catalogizer/internal/services"

// isHostAllowed validates that a hostname/IP is a legitimate SMB probe target.
// It blocks loopback, link-local, multicast, cloud-metadata IPs, and
// unqualified single-label names. Private RFC-1918 ranges ARE allowed
// because the identity-epic targets the LAN (Synology NAS at 192.168.0.0/24).
//
// Security invariants (§11.4 SSRF prevention):
//   - Loopback (127.0.0.0/8, ::1) — blocked (probe would hit this host, not a NAS)
//   - Link-local (169.254.0.0/16, fe80::/10) — blocked (no SMB server there)
//   - Multicast (224.0.0.0/4, ff00::/8) — blocked (not a unicast target)
//   - Cloud metadata IP 169.254.169.254 — blocked (credential exfiltration vector)
//   - Unqualified single-label name ("localhost", "nas") — allowed but
//     DNS resolution is performed server-side. The main protection is IP-level.
//   - Empty host — blocked (caller should validate required field first)
//
// The implementation is the shared services.VetSMBHost (syntax, single resolution, all addresses checked, lookup failure refused).
func isHostAllowed(host string) bool {
	return services.VetSMBHost(host) == nil
}
