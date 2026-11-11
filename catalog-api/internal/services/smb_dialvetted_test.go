package services

import (
	"context"
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// SMB-FU-9: the address that is vetted is the address that is dialed. The resolver answers an allowed address on the first call and
// 127.0.0.1 on every later call (DNS rebinding); the dialer seam records what is dialed and never touches the network.
type rebindRig struct {
	mu        sync.Mutex
	lookups   int
	dialed    []string
	failFirst int // number of leading dials that fail
}

func newRebindRig(t *testing.T, first []string) *rebindRig {
	t.Helper()
	r := &rebindRig{}
	oldR, oldD := SMBHostResolver, SMBDialer
	t.Cleanup(func() { SMBHostResolver, SMBDialer = oldR, oldD })
	SMBHostResolver = func(h string) ([]net.IP, error) {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.lookups++
		var out []net.IP
		if r.lookups == 1 {
			for _, s := range first {
				out = append(out, net.ParseIP(s))
			}
		} else {
			out = []net.IP{net.ParseIP("127.0.0.1")}
		}
		return out, nil
	}
	SMBDialer = func(_ context.Context, addr string, _ time.Duration) (net.Conn, error) {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.dialed = append(r.dialed, addr)
		return nil, errors.New("recorded, not dialed")
	}
	return r
}

func (r *rebindRig) assertVettedOnly(t *testing.T, wantAddrs []string, wantLookups int) {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	require.Equal(t, wantLookups, r.lookups, "resolver called exactly %d time(s) per request", wantLookups)
	require.Equal(t, wantAddrs, r.dialed, "only vetted IPs are dialed, in resolver order, never the name or 127.0.0.1")
}

func rebindSvc() *SMBDiscoveryService {
	return &SMBDiscoveryService{logger: zap.NewNop(), timeout: time.Second}
}

func TestDiscoverShares_DialsVettedIPNotName(t *testing.T) {
	r := newRebindRig(t, []string{"192.0.2.10", "192.0.2.11"})
	_, err := rebindSvc().DiscoverShares(context.Background(), "rebind.example", "u", "p", nil)
	require.Error(t, err)
	r.assertVettedOnly(t, []string{"192.0.2.10:445", "192.0.2.11:445"}, 1)
}

func TestProbeHostWithIdentities_ResolvesOnceDialsVettedIPForEveryIdentity(t *testing.T) {
	r := newRebindRig(t, []string{"192.0.2.10"})
	ids := []SMBIdentity{{Index: 1, Kind: "user", Username: "a"}, {Index: 2, Kind: "user", Username: "b"}}
	res, _ := rebindSvc().ProbeHostWithIdentities(context.Background(), "rebind.example", ids)
	require.NotNil(t, res)
	// guest + 2 identities = 3 attempts, one lookup in total
	r.assertVettedOnly(t, []string{"192.0.2.10:445", "192.0.2.10:445", "192.0.2.10:445"}, 1)
}

func TestMountShare_ResolvesOnceDialsVettedIPAndPort(t *testing.T) {
	t.Setenv("CATALOG_SMB_ALLOWED_PORTS", "1445")
	r := newRebindRig(t, []string{"192.0.2.10"})
	_, _, err := rebindSvc().openConfined(context.Background(), SMBConnectionConfig{Host: "rebind.example", Port: 1445, Share: "Data", Username: "u"})
	require.Error(t, err)
	r.assertVettedOnly(t, []string{"192.0.2.10:1445"}, 1)
}

func TestServiceEntryPoints_RefuseWhenAnyResolvedIPBlocked_NoDial(t *testing.T) {
	r := newRebindRig(t, []string{"192.0.2.10", "127.0.0.1"}) // second answer is loopback: the whole name is refused
	s := rebindSvc()
	_, err := s.DiscoverShares(context.Background(), "mixed.example", "u", "p", nil)
	require.True(t, errors.Is(err, ErrInvalidSMBInput))
	res, err := s.ProbeHostWithIdentities(context.Background(), "mixed.example", nil)
	require.True(t, errors.Is(err, ErrInvalidSMBInput))
	require.NotNil(t, res)
	require.False(t, res.Authenticated)
	_, _, err = s.openConfined(context.Background(), SMBConnectionConfig{Host: "mixed.example", Share: "Data"})
	require.True(t, errors.Is(err, ErrInvalidSMBInput))
	require.Empty(t, r.dialed, "nothing dialed when vetting fails")
}

func TestMountShare_PortRefusedBeforeAnyLookupOrDial(t *testing.T) {
	t.Setenv("CATALOG_SMB_ALLOWED_PORTS", "")
	r := newRebindRig(t, []string{"192.0.2.10"})
	_, _, err := rebindSvc().openConfined(context.Background(), SMBConnectionConfig{Host: "rebind.example", Port: 22, Share: "Data"})
	require.True(t, errors.Is(err, ErrInvalidSMBInput))
	r.assertVettedOnly(t, nil, 0)
}

func TestDialVetted_FallsThroughToNextVettedIPOnly(t *testing.T) {
	r := newRebindRig(t, []string{"192.0.2.10", "192.0.2.11"})
	calls := 0
	SMBDialer = func(_ context.Context, addr string, _ time.Duration) (net.Conn, error) {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.dialed = append(r.dialed, addr)
		calls++
		c, _ := net.Pipe()
		if calls == 1 {
			return nil, errors.New("first refused")
		}
		return c, nil
	}
	ips, err := VetSMBHostIPs("rebind.example")
	require.NoError(t, err)
	conn, err := dialVetted(context.Background(), ips, 445, time.Second)
	require.NoError(t, err)
	_ = conn.Close()
	r.assertVettedOnly(t, []string{"192.0.2.10:445", "192.0.2.11:445"}, 1)
}

func TestVetSMBHostIPs_ReturnsTheVettedAddresses(t *testing.T) {
	withResolver(t, map[string][]string{"good.example": {"192.0.2.10", "192.0.2.11"}})
	ips, err := VetSMBHostIPs("good.example")
	require.NoError(t, err)
	require.Len(t, ips, 2)
	require.Equal(t, "192.0.2.10", ips[0].String())
	lit, err := VetSMBHostIPs("192.168.0.5")
	require.NoError(t, err)
	require.Equal(t, "192.168.0.5", lit[0].String())
}
