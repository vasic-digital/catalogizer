package services

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/hirochachacha/go-smb2"
	"go.uber.org/zap"
)

// SMBIdentity is one authentication identity used to probe a host's shares.
// Index 0 is reserved for the implicit guest/anonymous attempt; configured
// identities are 1-based (matching CATALOGIZER_IDENTITY_<N>_*).
type SMBIdentity struct {
	Index    int
	Kind     string // "guest" | "credentials"
	Username string
	Password string
	Domain   *string
}

// Label returns a log-safe label for the identity (NEVER the password, §11.4.10).
func (i SMBIdentity) Label() string {
	if i.Kind == "guest" || i.Username == "" {
		return "guest"
	}
	return i.Username
}

// SMBProbeResult is the remembered working binding for a host: which identity
// authenticated and the REAL shares it exposed (§11.4.6 — via ListSharenames).
type SMBProbeResult struct {
	Host          string         `json:"host"`
	Authenticated bool           `json:"authenticated"`
	IdentityIndex int            `json:"identity_index"` // 0 = guest, >=1 = configured identity
	IdentityLabel string         `json:"identity_label"` // "guest" or username (never a secret)
	Shares        []SMBShareInfo `json:"shares"`
}

// LoadSMBIdentitiesFromEnv reads the configured identities from the environment
// (CATALOGIZER_IDENTITY_COUNT + CATALOGIZER_IDENTITY_<N>_TYPE/_USERNAME/_PASSWORD/_DOMAIN).
// Only credential identities are returned here (api_token / ssh_key are not SMB
// auth schemes). Secrets are read from env only and never logged (§11.4.10).
func LoadSMBIdentitiesFromEnv() []SMBIdentity {
	n, _ := strconv.Atoi(os.Getenv("CATALOGIZER_IDENTITY_COUNT"))
	out := []SMBIdentity{}
	for i := 1; i <= n; i++ {
		p := fmt.Sprintf("CATALOGIZER_IDENTITY_%d_", i)
		kind := os.Getenv(p + "TYPE")
		if kind != "credentials" {
			continue // SMB NTLM only consumes credential identities
		}
		user := os.Getenv(p + "USERNAME")
		if user == "" {
			continue
		}
		var domain *string
		if d := os.Getenv(p + "DOMAIN"); d != "" {
			domain = &d
		}
		out = append(out, SMBIdentity{
			Index:    i,
			Kind:     kind,
			Username: user,
			Password: os.Getenv(p + "PASSWORD"),
			Domain:   domain,
		})
	}
	return out
}

// dialAndEnumerate opens an SMB session as the given identity and returns the
// host's real shares, or an error if dial/auth/enumeration fails.
func (s *SMBDiscoveryService) dialAndEnumerate(ctx context.Context, host string, ips []net.IP, id SMBIdentity) ([]SMBShareInfo, error) {
	conn, err := dialVetted(ctx, ips, smbPort, s.timeout)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", host, err)
	}
	defer conn.Close()
	deadline := time.Now().Add(s.timeout + 4*time.Second)
	if dl, ok := ctx.Deadline(); ok && dl.Before(deadline) {
		deadline = dl
	}
	_ = conn.SetDeadline(deadline)

	d := &smb2.Dialer{
		Initiator: &smb2.NTLMInitiator{
			User:     id.Username,
			Password: id.Password,
			Domain:   getStringValue(id.Domain),
		},
	}
	session, err := d.Dial(conn)
	if err != nil {
		return nil, fmt.Errorf("auth as %s on %s: %w", id.Label(), host, err)
	}
	defer session.Logoff()

	return s.enumerateShares(session, host)
}

// ProbeHostWithIdentities implements the operator's anonymous-first, then
// each-identity-until-one-passes discovery (§ identity-share-discovery epic,
// pillar 3): it tries a guest session first, then every supplied identity in
// order, and returns the FIRST that authenticates together with the REAL shares
// it exposed — the remembered (host, identity, shares) binding. Identities are
// tried in caller order so deterministic priority is preserved (§11.4.50).
//
// A nil/empty identity list still attempts guest. The returned result is always
// non-nil; Authenticated=false means neither guest nor any identity worked, and
// the returned error carries the last failure for diagnostics (§11.4.6 — the
// caller records an honest "no identity authenticated", never a guess).
func (s *SMBDiscoveryService) ProbeHostWithIdentities(ctx context.Context, host string, identities []SMBIdentity) (*SMBProbeResult, error) {
	if err := ValidateSMBHostSyntax(host); err != nil {
		return nil, err
	}
	// Resolve + vet ONCE; every identity attempt dials these addresses (no second lookup, SMB-FU-9).
	ips, err := VetSMBHostIPs(host)
	if err != nil {
		return &SMBProbeResult{Host: host}, err
	}
	candidates := make([]SMBIdentity, 0, len(identities)+1)
	candidates = append(candidates, SMBIdentity{Index: 0, Kind: "guest", Username: "guest"})
	candidates = append(candidates, identities...)

	var lastErr error
	for _, id := range candidates {
		shares, err := s.dialAndEnumerate(ctx, host, ips, id)
		if err != nil {
			lastErr = err
			s.logger.Debug("SMB probe identity did not authenticate",
				zap.String("host", host), zap.String("identity", id.Label()))
			continue
		}
		s.logger.Info("SMB probe found working identity binding",
			zap.String("host", host), zap.String("identity", id.Label()))
		return &SMBProbeResult{
			Host:          host,
			Authenticated: true,
			IdentityIndex: id.Index,
			IdentityLabel: id.Label(),
			Shares:        shares,
		}, nil
	}
	return &SMBProbeResult{Host: host, Authenticated: false}, lastErr
}
