package services

import (
	"context"
	"encoding/json"
	"net"
	"strings"
	"testing"

	"go.uber.org/zap"
)

// TestProbeVettedIPs_AuthenticatedWithNoSharesIsEmptyListNotNull: a session that authenticates but exposes zero shares must
// serialise as "shares":[] and not "shares":null (clients iterate the field). The SMB session is the unit-class probe seam.
func TestProbeVettedIPs_AuthenticatedWithNoSharesIsEmptyListNotNull(t *testing.T) {
	svc := NewSMBDiscoveryService(zap.NewNop()).WithProbeSeam(func(context.Context, string, []net.IP, SMBIdentity) ([]SMBShareInfo, error) {
		return nil, nil // authenticated, nothing exported
	})
	res, err := svc.ProbeVettedIPsWithIdentities(context.Background(), "files.example.test", []net.IP{net.ParseIP("8.8.8.8")}, nil)
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	if !res.Authenticated {
		t.Fatalf("seam authenticated the guest identity; got Authenticated=false")
	}
	if res.Shares == nil {
		t.Fatal("Shares is nil: it would marshal to JSON null")
	}
	b, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"shares":[]`) && !strings.Contains(string(b), `"Shares":[]`) {
		t.Fatalf("expected an empty JSON list for the shares field, got %s", b)
	}
	if strings.Contains(string(b), "null") {
		t.Fatalf("result JSON contains null: %s", b)
	}
}
