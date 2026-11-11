package services

import (
	"errors"
	"net"
	"strings"
	"testing"
)

// stubSSRFResolver lets tests run offline with deterministic DNS.
type stubSSRFResolver struct {
	ips map[string][]net.IP
	err error
}

func (s stubSSRFResolver) LookupIP(_, host string) ([]net.IP, error) {
	if s.err != nil {
		return nil, s.err
	}
	if ips, ok := s.ips[host]; ok {
		return ips, nil
	}
	return nil, errors.New("stub: no such host")
}

// withStrictSSRFGuard turns off the package-wide test-only relax so
// the guard is evaluated in production mode for the duration of t.
func withStrictSSRFGuard(t *testing.T) {
	t.Helper()
	prev := testAllowPrivateNetworks
	testAllowPrivateNetworks = false
	t.Cleanup(func() { testAllowPrivateNetworks = prev })
}

func TestGuardProviderURL_RejectsEmpty(t *testing.T) {
	withStrictSSRFGuard(t)
	if err := GuardProviderURL("", SSRFGuardConfig{}); err == nil {
		t.Error("empty URL must be rejected")
	}
}

func TestGuardProviderURL_RejectsUnknownScheme(t *testing.T) {
	withStrictSSRFGuard(t)
	err := GuardProviderURL("gopher://example.com/", SSRFGuardConfig{})
	if err == nil || !strings.Contains(err.Error(), "scheme") {
		t.Errorf("expected scheme rejection, got %v", err)
	}
}

func TestGuardProviderURL_RejectsLoopbackLiteral(t *testing.T) {
	withStrictSSRFGuard(t)
	for _, target := range []string{
		"http://127.0.0.1/",
		"http://127.0.0.5:8080/",
		"http://[::1]/",
	} {
		err := GuardProviderURL(target, SSRFGuardConfig{})
		if err == nil || !errors.Is(err, ErrSSRFBlocked) {
			t.Errorf("loopback %q must be rejected, got %v", target, err)
		}
	}
}

func TestGuardProviderURL_RejectsRFC1918(t *testing.T) {
	withStrictSSRFGuard(t)
	for _, target := range []string{
		"http://10.0.0.5/",
		"http://172.16.0.1/",
		"http://172.31.255.254/",
		"http://192.168.1.1/",
	} {
		err := GuardProviderURL(target, SSRFGuardConfig{})
		if err == nil || !errors.Is(err, ErrSSRFBlocked) {
			t.Errorf("private address %q should block, got %v", target, err)
		}
	}
}

func TestGuardProviderURL_RejectsCloudMetadata(t *testing.T) {
	withStrictSSRFGuard(t)
	err := GuardProviderURL("http://169.254.169.254/latest/meta-data/", SSRFGuardConfig{})
	if err == nil || !errors.Is(err, ErrSSRFBlocked) {
		t.Errorf("cloud metadata must block, got %v", err)
	}
}

func TestGuardProviderURL_RejectsIPv6Private(t *testing.T) {
	withStrictSSRFGuard(t)
	for _, target := range []string{
		"http://[fe80::1]/",
		"http://[fc00::1]/",
	} {
		err := GuardProviderURL(target, SSRFGuardConfig{})
		if err == nil || !errors.Is(err, ErrSSRFBlocked) {
			t.Errorf("IPv6 private %q must block, got %v", target, err)
		}
	}
}

func TestGuardProviderURL_RejectsUnspecified(t *testing.T) {
	withStrictSSRFGuard(t)
	for _, target := range []string{"http://0.0.0.0/", "http://[::]/"} {
		err := GuardProviderURL(target, SSRFGuardConfig{})
		if err == nil || !errors.Is(err, ErrSSRFBlocked) {
			t.Errorf("unspecified %q must block, got %v", target, err)
		}
	}
}

func TestGuardProviderURL_PublicIPAllowed(t *testing.T) {
	err := GuardProviderURL("https://1.1.1.1/", SSRFGuardConfig{})
	if err != nil {
		t.Errorf("public IP must pass, got %v", err)
	}
}

func TestGuardProviderURL_HostnameToPublicIPAllowed(t *testing.T) {
	err := GuardProviderURL("https://api.fanart.tv/", SSRFGuardConfig{
		Resolver: stubSSRFResolver{ips: map[string][]net.IP{
			"api.fanart.tv": {net.ParseIP("203.0.113.7")},
		}},
	})
	if err != nil {
		t.Errorf("public hostname must pass, got %v", err)
	}
}

func TestGuardProviderURL_HostnameToPrivateIPBlocked(t *testing.T) {
	withStrictSSRFGuard(t)
	err := GuardProviderURL("https://evil.fanart.tv/", SSRFGuardConfig{
		Resolver: stubSSRFResolver{ips: map[string][]net.IP{
			"evil.fanart.tv": {net.ParseIP("10.0.0.5")},
		}},
	})
	if err == nil || !errors.Is(err, ErrSSRFBlocked) {
		t.Errorf("hostname → private IP pivot must block, got %v", err)
	}
}

func TestGuardProviderURL_MixedResolutionBlocked(t *testing.T) {
	withStrictSSRFGuard(t)
	err := GuardProviderURL("https://dual.fanart.tv/", SSRFGuardConfig{
		Resolver: stubSSRFResolver{ips: map[string][]net.IP{
			"dual.fanart.tv": {
				net.ParseIP("203.0.113.7"),
				net.ParseIP("192.168.1.1"),
			},
		}},
	})
	if err == nil || !errors.Is(err, ErrSSRFBlocked) {
		t.Errorf("mixed public + private resolution must block, got %v", err)
	}
}

func TestGuardProviderURL_AllowPrivateOptIn(t *testing.T) {
	cfg := SSRFGuardConfig{AllowPrivateNetworks: true}
	for _, target := range []string{
		"http://127.0.0.1/",
		"http://192.168.1.1/",
	} {
		if err := GuardProviderURL(target, cfg); err != nil {
			t.Errorf("opt-in: %q must pass, got %v", target, err)
		}
	}
}

func TestGuardProviderURL_RejectsIntegerEncodedLoopback(t *testing.T) {
	withStrictSSRFGuard(t)
	// 2130706433 = 0x7F000001 = 127.0.0.1
	for _, target := range []string{
		"http://2130706433/",
		"http://3232235777/", // 192.168.1.1
	} {
		err := GuardProviderURL(target, SSRFGuardConfig{})
		if err == nil || !errors.Is(err, ErrSSRFBlocked) {
			t.Errorf("integer-encoded %q must block, got %v", target, err)
		}
	}
}

func TestGuardProviderURL_RejectsShortDottedLoopback(t *testing.T) {
	withStrictSSRFGuard(t)
	for _, target := range []string{
		"http://127.1/",      // 127.0.0.1
		"http://127.0.1/",    // 127.0.0.1
		"http://10.1/",       // 10.0.0.1
		"http://192.168.1/",  // 192.168.0.1
	} {
		err := GuardProviderURL(target, SSRFGuardConfig{})
		if err == nil || !errors.Is(err, ErrSSRFBlocked) {
			t.Errorf("short-dotted %q must block, got %v", target, err)
		}
	}
}

func TestGuardProviderURL_IntegerPublicPasses(t *testing.T) {
	withStrictSSRFGuard(t)
	// 16843009 = 1.1.1.1 (Cloudflare public DNS)
	err := GuardProviderURL("http://16843009/", SSRFGuardConfig{})
	if err != nil {
		t.Errorf("integer-encoded public IP must pass, got %v", err)
	}
}

func TestGuardProviderURL_LookupFailureBlocks(t *testing.T) {
	withStrictSSRFGuard(t)
	err := GuardProviderURL("https://missing.example/", SSRFGuardConfig{
		Resolver: stubSSRFResolver{err: errors.New("nxdomain")},
	})
	if err == nil || !errors.Is(err, ErrSSRFBlocked) {
		t.Errorf("lookup failure must block, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// allowPublicURLLocal regression — new coverage for 172.16/12 + IPv6
// + expanded scheme blacklist. Keeps the existing cases green.
// ---------------------------------------------------------------------------

func TestAllowPublicURLLocal_NewCoverage(t *testing.T) {
	cases := []struct {
		url string
		bad bool
	}{
		// New private-range cases the old guard missed.
		{"http://172.16.0.1/x", true},
		{"http://172.20.5.7/x", true},
		{"http://172.31.255.254/x", true},
		{"http://0.0.0.0/x", true},
		{"http://[::1]/x", true},
		{"http://[::]/x", true},
		{"http://[fe80::1]/x", true},
		{"http://[fc00::1]/x", true},
		{"http://[fd12:3456:789a::1]/x", true},
		// New dangerous schemes.
		{"gopher://example.com/x", true},
		{"ftp://example.com/x", true},
		{"jar:http://example.com/x.jar!/path", true},
		{"view-source:http://example.com/x", true},
		// Adjacent but NOT private: 172.15.x.x + 172.32.x.x are
		// public, 11.x.x.x is public.
		{"http://172.15.0.1/x", false},
		{"http://172.32.0.1/x", false},
		{"http://11.0.0.1/x", false},
	}
	for _, c := range cases {
		err := allowPublicURLLocal(c.url)
		if c.bad && err == nil {
			t.Errorf("expected reject for %q", c.url)
		}
		if !c.bad && err != nil {
			t.Errorf("expected allow for %q, got %v", c.url, err)
		}
	}
}
