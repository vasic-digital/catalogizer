package services

import (
	"context"
	"net"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/hirochachacha/go-smb2"
	"go.uber.org/zap"
)

// TestDiscoverShares_RealShares_NoGuessing is an integration test (§11.4.27 — real
// system, no fakes/mocks) proving enumerateShares returns the host's REAL exported
// shares via SRVSVC ListSharenames, NOT a guessed list of common names (§11.4.6).
//
// Anti-bluff design: it derives the ground-truth share set from a fresh, separate
// SRVSVC enumeration of the SAME host, then asserts DiscoverShares returns EXACTLY
// that set (minus IPC$) — every real share present (the old guessing list missed
// names like "WORK20"/"DATA18"/"DATA20" — verified against a real NAS
// 2026-06-26), and no fabricated share absent from the server.
//
// SKIP-with-reason (§11.4.3) when no real SMB host/identity is provided via env, so
// the suite stays green on hosts without LAN access; it FAILs (never silently
// passes) the moment a reachable host with valid creds returns guessed/omitted shares.
func TestDiscoverShares_RealShares_NoGuessing(t *testing.T) {
	host := os.Getenv("CATALOGIZER_TEST_SMB_HOST")
	user := os.Getenv("CATALOGIZER_IDENTITY_1_USERNAME")
	pw := os.Getenv("CATALOGIZER_IDENTITY_1_PASSWORD")
	if host == "" || user == "" {
		t.Skip("§11.4.3: set CATALOGIZER_TEST_SMB_HOST + CATALOGIZER_IDENTITY_1_USERNAME/_PASSWORD to run against a real SMB host")
	}

	// Ground truth: the shares the server actually advertises (SRVSVC).
	conn, err := net.DialTimeout("tcp", host+":445", 6*time.Second)
	if err != nil {
		t.Skipf("§11.4.3: SMB host %s unreachable: %v", host, err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(12 * time.Second))
	d := &smb2.Dialer{Initiator: &smb2.NTLMInitiator{User: user, Password: pw}}
	sess, err := d.Dial(conn)
	if err != nil {
		// Host reachable + creds supplied => a genuine auth/protocol failure, not a skip.
		t.Fatalf("dial SMB session to %s as %s: %v", host, user, err)
	}
	defer sess.Logoff()
	truth, err := sess.ListSharenames()
	if err != nil {
		t.Fatalf("ListSharenames(%s): %v", host, err)
	}
	want := map[string]bool{}
	for _, n := range truth {
		if !strings.EqualFold(n, "IPC$") {
			want[n] = true
		}
	}
	if len(want) == 0 {
		t.Skipf("§11.4.3: host %s exports no non-IPC$ shares", host)
	}

	// System under test.
	svc := NewSMBDiscoveryService(zap.NewNop())
	got, err := svc.DiscoverShares(context.Background(), host, user, pw, nil)
	if err != nil {
		t.Fatalf("DiscoverShares(%s): %v", host, err)
	}
	gotSet := map[string]bool{}
	for _, sh := range got {
		if strings.EqualFold(sh.ShareName, "IPC$") {
			t.Errorf("DiscoverShares leaked the IPC$ control share (should be filtered)")
		}
		gotSet[sh.ShareName] = true
	}

	// Anti-bluff equality: no real share omitted, no fabricated share added.
	for n := range want {
		if !gotSet[n] {
			t.Errorf("real share %q is missing from DiscoverShares — the old guessing list would miss it (§11.4.6 regression)", n)
		}
	}
	for n := range gotSet {
		if !want[n] {
			t.Errorf("DiscoverShares returned %q which the server does NOT export — fabricated/guessed (§11.4.6)", n)
		}
	}

	var list []string
	for n := range gotSet {
		list = append(list, n)
	}
	sort.Strings(list)
	t.Logf("§11.4.6 PASS: real shares enumerated for %s = %v (ground-truth non-IPC$ count=%d)", host, list, len(want))
}
