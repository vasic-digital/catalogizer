package services

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
)

// TestProbeHostWithIdentities_GuestFirstThenIdentities is the deterministic,
// always-run GREEN guard for the anonymous-first / multi-identity probe
// (§ identity-share-discovery epic, pillar 3). It needs NO network: it probes a
// closed loopback SMB port so every candidate (guest + supplied identities)
// fails to dial, and asserts the HONEST contract — the result is always
// non-nil, Authenticated is false when nothing binds, and an error carrying the
// last failure is returned (§11.4.6: the caller records "no identity
// authenticated", never a guess).
//
// §11.4.115 polarity: this guard FAILs (RED) if ProbeHostWithIdentities ever
// regresses to returning a nil result, a true verdict without a real session,
// or swallowing the failure — the exact bluff shapes the epic forbids.
// TestProbeHostWithIdentities_RejectsEmptyIdentities is the §11.4.115 polarity
// regression guard: ProbeHostWithIdentities with nil or empty identity list must
// still attempt guest and return a non-nil result with Authenticated=false.
// If a future change breaks the guest-first contract this test FAILs (RED),
// proving the guard catches the regression.
func TestProbeHostWithIdentities_RejectsEmptyIdentities(t *testing.T) {
	svc := &SMBDiscoveryService{logger: zap.NewNop(), timeout: 2 * time.Second}

	t.Run("nil identities", func(t *testing.T) {
		res, err := svc.ProbeHostWithIdentities(context.Background(), "127.0.0.1", nil)
		if res == nil {
			t.Fatalf("§11.4.6: ProbeHostWithIdentities returned nil result with nil identities — contract requires non-nil")
		}
		if res.Authenticated {
			t.Fatalf("§11.4.6: nil identities reported Authenticated against closed loopback — false-positive bluff")
		}
		if res.Host != "127.0.0.1" {
			t.Errorf("result host = %q, want 127.0.0.1", res.Host)
		}
		if err == nil {
			t.Errorf("§11.4.6: nil identities must return an error (dial failure), got nil")
		}
		if res.IdentityIndex != 0 {
			t.Errorf("failed probe IdentityIndex = %d, want 0", res.IdentityIndex)
		}
	})

	t.Run("empty identities", func(t *testing.T) {
		res, err := svc.ProbeHostWithIdentities(context.Background(), "127.0.0.1", []SMBIdentity{})
		if res == nil {
			t.Fatalf("§11.4.6: ProbeHostWithIdentities returned nil result with empty identities — contract requires non-nil")
		}
		if res.Authenticated {
			t.Fatalf("§11.4.6: empty identities reported Authenticated against closed loopback — false-positive bluff")
		}
		if res.IdentityIndex != 0 {
			t.Errorf("failed probe IdentityIndex = %d, want 0", res.IdentityIndex)
		}
		if err == nil {
			t.Errorf("§11.4.6: empty identities must return an error (dial failure), got nil")
		}
	})
}

// TestProbeHostWithIdentities_Determinism is the §11.4.50 determinism test:
// calling ProbeHostWithIdentities N=3 times with the same closed-loopback host
// and same identities must return IDENTICAL results (Host, Authenticated flag,
// IdentityIndex, IdentityLabel, len(Shares)) each time — proving the
// deterministic-priority identity dispatch does not depend on random iteration
// or map ordering.
//
// This does not need a real SMB host: 127.0.0.1:445 is always refused on CI/dev
// machines, making every candidate fail deterministically.
func TestProbeHostWithIdentities_Determinism(t *testing.T) {
	svc := &SMBDiscoveryService{logger: zap.NewNop(), timeout: 2 * time.Second}

	ids := []SMBIdentity{
		{Index: 1, Kind: "credentials", Username: "probe_user_1", Password: "x"},
		{Index: 2, Kind: "credentials", Username: "probe_user_2", Password: "y"},
	}

	const host = "127.0.0.1"
	var prevResult *SMBProbeResult
	var prevErr error

	for i := 0; i < 3; i++ {
		res, err := svc.ProbeHostWithIdentities(context.Background(), host, ids)
		if res == nil {
			t.Fatalf("§11.4.6: iteration %d returned nil result — contract requires non-nil", i)
		}
		if i == 0 {
			prevResult = res
			prevErr = err
			continue
		}
		if res.Authenticated != prevResult.Authenticated {
			t.Errorf("§11.4.50: iteration %d Authenticated=%v, want %v (iter 0)", i, res.Authenticated, prevResult.Authenticated)
		}
		if res.IdentityIndex != prevResult.IdentityIndex {
			t.Errorf("§11.4.50: iteration %d IdentityIndex=%d, want %d (iter 0)", i, res.IdentityIndex, prevResult.IdentityIndex)
		}
		if res.Host != prevResult.Host {
			t.Errorf("§11.4.50: iteration %d Host=%q, want %q (iter 0)", i, res.Host, prevResult.Host)
		}
		if res.IdentityLabel != prevResult.IdentityLabel {
			t.Errorf("§11.4.50: iteration %d IdentityLabel=%q, want %q (iter 0)", i, res.IdentityLabel, prevResult.IdentityLabel)
		}
		if len(res.Shares) != len(prevResult.Shares) {
			t.Errorf("§11.4.50: iteration %d len(Shares)=%d, want %d (iter 0)", i, len(res.Shares), len(prevResult.Shares))
		}
		if (err == nil) != (prevErr == nil) {
			t.Errorf("§11.4.50: iteration %d err!=nil=%v, want %v (iter 0)", i, err != nil, prevErr != nil)
		}
	}
	t.Logf("§11.4.50 PASS: ProbeHostWithIdentities deterministic across 3 iterations (host=%q, Authenticated=%v)", host, prevResult.Authenticated)
}

func TestProbeHostWithIdentities_GuestFirstThenIdentities(t *testing.T) {
	// Short timeout so a refused/closed port returns promptly.
	svc := &SMBDiscoveryService{logger: zap.NewNop(), timeout: 2 * time.Second}

	ids := []SMBIdentity{
		{Index: 1, Kind: "credentials", Username: "probe_user_1", Password: "x"},
		{Index: 2, Kind: "credentials", Username: "probe_user_2", Password: "y"},
	}

	// 127.0.0.1 has no SMB service in CI/dev → every dial is refused fast.
	res, err := svc.ProbeHostWithIdentities(context.Background(), "127.0.0.1", ids)

	if res == nil {
		t.Fatalf("§11.4.6: ProbeHostWithIdentities returned a nil result — the contract requires a non-nil result even on total failure")
	}
	if res.Authenticated {
		t.Fatalf("§11.4.6: probe reported Authenticated=true against a closed loopback SMB port — false-positive bluff (identity=%q)", res.IdentityLabel)
	}
	if res.Host != "127.0.0.1" {
		t.Errorf("result host = %q, want 127.0.0.1", res.Host)
	}
	if err == nil {
		t.Errorf("§11.4.6: probe must return the last dial failure when nothing authenticates, got nil error")
	}
	// A failed probe must never leak shares or a non-guest identity index.
	if len(res.Shares) != 0 {
		t.Errorf("failed probe leaked %d shares — must be empty", len(res.Shares))
	}
	if res.IdentityIndex != 0 {
		t.Errorf("failed probe IdentityIndex = %d, want 0 (no binding)", res.IdentityIndex)
	}
}

// TestProbeHostWithIdentities_RealNAS is the §11.4.27 real-system integration
// proof: against a reachable NAS with the operator's stored identities
// it asserts the probe (a) authenticates with guest-or-an-identity, (b) returns
// REAL shares (via ListSharenames, §11.4.6 — not a guessed common-name list),
// and (c) never exposes a secret in the result (§11.4.10 — IdentityLabel is a
// username/"guest", never a password).
//
// SKIP-with-reason (§11.4.3) when no real host/identities are configured, so the
// suite stays green without LAN access; it FAILs the moment a reachable host
// with valid identities yields no binding or guessed shares.
//
// Recon FACT (2026-06-26): .111 binds with identity #1 → DATA20/music/WORK20;
// .213 rejects #1 and binds only with identity #2 → Public/Music/Projects — the
// multi-identity fallback this probe implements. Run e.g.:
//
//	set -a; . ./.env; set +a
//	CATALOGIZER_TEST_SMB_HOST=192.168.0.213 \
//	  GOMAXPROCS=3 go test ./internal/services/ -run ProbeHostWithIdentities_RealNAS -v
func TestProbeHostWithIdentities_RealNAS(t *testing.T) {
	host := strings.TrimSpace(os.Getenv("CATALOGIZER_TEST_SMB_HOST"))
	if host == "" {
		t.Skip("§11.4.3: set CATALOGIZER_TEST_SMB_HOST (+ sourced .env identities) to probe a real SMB host")
	}
	ids := LoadSMBIdentitiesFromEnv()
	if len(ids) == 0 {
		t.Skip("§11.4.3: no CATALOGIZER_IDENTITY_* credentials in env to probe with")
	}

	svc := NewSMBDiscoveryService(zap.NewNop())
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	res, err := svc.ProbeHostWithIdentities(ctx, host, ids)
	if res == nil {
		t.Fatalf("nil probe result for %s", host)
	}
	if !res.Authenticated {
		t.Fatalf("§11.4.6: host %s reachable + identities supplied but NO binding found (last err: %v)", host, err)
	}
	if len(res.Shares) == 0 {
		t.Fatalf("§11.4.6: probe authenticated as %q on %s but returned ZERO shares — real shares expected via ListSharenames", res.IdentityLabel, host)
	}
	// §11.4.10: the label must be a username or "guest", never a secret value.
	for _, id := range ids {
		if id.Password != "" && res.IdentityLabel == id.Password {
			t.Fatalf("§11.4.10: probe result leaked a password as IdentityLabel")
		}
	}
	var names []string
	for _, sh := range res.Shares {
		if strings.EqualFold(sh.ShareName, "IPC$") {
			t.Errorf("§11.4.6: probe leaked IPC$ control share")
		}
		names = append(names, sh.ShareName)
	}
	t.Logf("§11.4.6 PASS: %s bound via identity #%d (%q) → real shares %v",
		host, res.IdentityIndex, res.IdentityLabel, names)
}
