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
	withSMBHostResolver(t, func(h string) ([]net.IP, error) {
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
	})
	withSMBDialer(t, func(_ context.Context, addr string, _ time.Duration) (net.Conn, error) {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.dialed = append(r.dialed, addr)
		return nil, errors.New("recorded, not dialed")
	})
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
	withSMBDialer(t, func(_ context.Context, addr string, _ time.Duration) (net.Conn, error) {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.dialed = append(r.dialed, addr)
		calls++
		if calls == 1 {
			return nil, errors.New("first refused")
		}
		c, other := net.Pipe()
		t.Cleanup(func() { _ = other.Close() })
		return c, nil
	})
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

// F1: one shared connect deadline and a cap on attempts.
func TestDialVetted_OneSharedDeadlineAcrossAttempts(t *testing.T) {
	r := newRebindRig(t, []string{"192.0.2.10", "192.0.2.11", "192.0.2.12"})
	now := time.Unix(1000, 0)
	withSMBNow(t, func() time.Time { return now })
	var budgets []time.Duration
	withSMBDialer(t, func(_ context.Context, addr string, to time.Duration) (net.Conn, error) {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.dialed = append(r.dialed, addr)
		budgets = append(budgets, to)
		now = now.Add(to) // a blackholed address consumes everything it was given
		return nil, errors.New("blackhole")
	})
	ips, err := VetSMBHostIPs("rebind.example")
	require.NoError(t, err)
	start := now
	_, err = dialVetted(context.Background(), ips, 445, 10*time.Second)
	require.Error(t, err)
	require.Equal(t, []string{"192.0.2.10:445"}, r.dialed, "first attempt used the whole budget; the others get none")
	require.Equal(t, []time.Duration{10 * time.Second}, budgets)
	require.LessOrEqual(t, now.Sub(start), 10*time.Second, "total cost bounded by ONE timeout")
}

func TestDialVetted_RemainingBudgetShrinksAndAttemptsAreCapped(t *testing.T) {
	r := newRebindRig(t, []string{"192.0.2.1", "192.0.2.2", "192.0.2.3", "192.0.2.4", "192.0.2.5", "192.0.2.6"})
	now := time.Unix(1000, 0)
	withSMBNow(t, func() time.Time { return now })
	var budgets []time.Duration
	withSMBDialer(t, func(_ context.Context, addr string, to time.Duration) (net.Conn, error) {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.dialed = append(r.dialed, addr)
		budgets = append(budgets, to)
		now = now.Add(time.Second) // fast refusals
		return nil, errors.New("refused")
	})
	ips, err := VetSMBHostIPs("rebind.example")
	require.NoError(t, err)
	_, err = dialVetted(context.Background(), ips, 445, 10*time.Second)
	require.Error(t, err)
	require.Len(t, r.dialed, maxVettedDials, "at most 4 of the 6 vetted answers are tried")
	require.Equal(t, []time.Duration{10 * time.Second, 9 * time.Second, 8 * time.Second, 7 * time.Second}, budgets)
}

// F2: port 0 means 445 all the way to the dial.
func TestMountShare_PortZeroDials445(t *testing.T) {
	r := newRebindRig(t, []string{"192.0.2.10"})
	_, _, err := rebindSvc().openConfined(context.Background(), SMBConnectionConfig{Host: "rebind.example", Port: 0, Share: "Data", Username: "u"})
	require.Error(t, err)
	r.assertVettedOnly(t, []string{"192.0.2.10:445"}, 1)
}

// F2: the PRODUCTION dialer, against a real loopback listener (loopback is reachable here only because the test calls the dialer seam
// directly; the vetting layer is not involved).
func TestProductionSMBDialer_ConnectsToIPLiteral(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()
	accepted := make(chan net.Conn, 1)
	go func() {
		c, aerr := ln.Accept()
		if aerr == nil {
			accepted <- c
		}
	}()
	conn, err := SMBDialer(context.Background(), ln.Addr().String(), 2*time.Second)
	require.NoError(t, err)
	defer conn.Close()
	require.Equal(t, ln.Addr().String(), conn.RemoteAddr().String(), "connected to the IP literal given")
	select {
	case c := <-accepted:
		_ = c.Close()
	case <-time.After(2 * time.Second):
		t.Fatal("listener saw no connection")
	}
}

func TestProductionSMBDialer_EnforcesTimeout(t *testing.T) {
	// 192.0.2.1 is TEST-NET-1 (never routed). Depending on the host the SYN is blackholed (timeout) or refused at once with
	// "network unreachable"; either way the dial MUST return within a small multiple of the timeout, never the OS default (~2 minutes).
	start := time.Now()
	conn, err := SMBDialer(context.Background(), "192.0.2.1:445", 300*time.Millisecond)
	if err == nil {
		_ = conn.Close()
		t.Skip("TEST-NET-1 is reachable on this host")
	}
	require.Less(t, time.Since(start), 5*time.Second, "timeout enforced: %v", err)
}

func TestProductionSMBDialer_HonoursContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	start := time.Now()
	_, err := SMBDialer(ctx, "192.0.2.1:445", time.Minute)
	require.Error(t, err)
	require.Less(t, time.Since(start), 2*time.Second)
}
