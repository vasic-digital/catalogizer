package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
)

// F3 (CAT-TRUSTEDPROXIES-FU-1): every range `auto` trusts is pinned, with both
// edges (last inside / first outside) so dropping or widening a range fails.
func TestAutoRanges_PinnedWithEdges(t *testing.T) {
	const real, fwd = "203.0.113.9", "203.0.113.9"
	trusted := []string{
		"127.0.0.1", "127.255.255.255", "10.0.0.1", "10.255.255.255",
		"172.16.0.1", "172.31.255.255", "192.168.0.1", "192.168.255.255",
		"::1", "fc00::1", "fdff:ffff::1",
	}
	untrusted := []string{
		"126.255.255.255", "128.0.0.1", "9.255.255.255", "11.0.0.1",
		"172.15.255.255", "172.32.0.1", "192.167.255.255", "192.169.0.1",
		"fbff::1", "fe00::1", "fe80::1", "2001:db8::1",
	}
	for _, p := range trusted {
		if got := clientIPFor(t, "auto", p, fwd); got != real {
			t.Errorf("auto must trust peer %s: ClientIP=%q want %q", p, got, real)
		}
	}
	for _, p := range untrusted {
		if got := clientIPFor(t, "auto", p, fwd); got != p {
			t.Errorf("auto must NOT trust peer %s: ClientIP=%q want peer", p, got)
		}
	}
}

// F2 (CAT-TRUSTEDPROXIES-AUTO-1): the characterised hazard. A client inside a
// trusted range behind a trusted proxy forges ClientIP because gin skips
// trusted XFF entries from the right. Pinned so the hazard stays documented
// and so an explicit CIDR is shown NOT to have it.
func TestAuto_ForgeableByTrustedRangeClient_ExplicitCIDRIsNot(t *testing.T) {
	const xff = "9.9.9.9, 192.168.1.50"
	if got := clientIPFor(t, "auto", "172.18.0.5", xff); got != "9.9.9.9" {
		t.Fatalf("hazard characterisation changed: auto ClientIP=%q (expected forged 9.9.9.9)", got)
	}
	if got := clientIPFor(t, "172.18.0.5/32", "172.18.0.5", xff); got != "192.168.1.50" {
		t.Fatalf("explicit CIDR must not be forgeable: ClientIP=%q", got)
	}
}

// F2 fix: `auto` is refused in release mode unless explicitly acknowledged.
func TestAuto_RefusedInReleaseModeUnlessAcknowledged(t *testing.T) {
	prev := gin.Mode()
	gin.SetMode(gin.ReleaseMode)
	t.Cleanup(func() { gin.SetMode(prev) })

	t.Setenv("TRUSTED_PROXIES_ALLOW_AUTO", "")
	r := gin.New()
	if err := applyTrustedProxies(r, "auto"); err == nil {
		t.Fatal("auto must be refused in release mode without TRUSTED_PROXIES_ALLOW_AUTO=true")
	}
	t.Setenv("TRUSTED_PROXIES_ALLOW_AUTO", "yes") // only the exact value "true" counts
	if err := applyTrustedProxies(gin.New(), "auto"); err == nil {
		t.Fatal("only the exact value true may acknowledge auto")
	}
	t.Setenv("TRUSTED_PROXIES_ALLOW_AUTO", "true")
	if err := applyTrustedProxies(gin.New(), "auto"); err != nil {
		t.Fatalf("acknowledged auto must work: %v", err)
	}
	// explicit CIDR and unset are unaffected by release mode
	t.Setenv("TRUSTED_PROXIES_ALLOW_AUTO", "")
	if err := applyTrustedProxies(gin.New(), "172.28.0.10/32"); err != nil {
		t.Fatalf("explicit CIDR must work in release: %v", err)
	}
	if err := applyTrustedProxies(gin.New(), ""); err != nil {
		t.Fatalf("unset must work in release: %v", err)
	}
}

func TestAuto_AllowedOutsideReleaseMode(t *testing.T) {
	prev := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(prev) })
	t.Setenv("TRUSTED_PROXIES_ALLOW_AUTO", "")
	if err := applyTrustedProxies(gin.New(), "auto"); err != nil {
		t.Fatalf("auto stays available for dev/test: %v", err)
	}
}

// F1 (CAT-TRUSTEDPROXIES-PROD-1): the shipped production compose files must make
// the API trust exactly the proxy front door. Walks the REAL parsed YAML
// structurally: proxy service -> its network entry -> ipv4_address, inside the
// network's ipam subnet, outside its ip_range (so a dynamic container can never
// take it), same network as the API, API default TRUSTED_PROXIES == that /32.
type composeDoc struct {
	Services map[string]struct {
		Environment map[string]any `yaml:"environment"`
		Networks    any            `yaml:"networks"`
	} `yaml:"services"`
	Networks map[string]struct {
		Name string `yaml:"name"`
		Ipam struct {
			Config []struct {
				Subnet  string `yaml:"subnet"`
				IPRange string `yaml:"ip_range"`
			} `yaml:"config"`
		} `yaml:"ipam"`
	} `yaml:"networks"`
}

var composeDefault = regexp.MustCompile(`^\$\{[A-Z_]+:-([^}]*)\}$`)

func resolveDefault(t *testing.T, what string, v any) string {
	t.Helper()
	sv, ok := v.(string)
	if !ok || sv == "" {
		t.Fatalf("%s is unset", what)
	}
	m := composeDefault.FindStringSubmatch(sv)
	if m == nil {
		t.Fatalf("%s=%q must be an overridable ${VAR:-default}", what, sv)
	}
	return m[1]
}

// networkAttach returns (attached, ipv4_address) for service networks given as
// either a list or a map.
func networkAttach(n any, key string) (bool, any) {
	switch v := n.(type) {
	case []any:
		for _, e := range v {
			if e == key {
				return true, nil
			}
		}
	case map[string]any:
		if e, ok := v[key]; ok {
			if m, ok := e.(map[string]any); ok {
				return true, m["ipv4_address"]
			}
			return true, nil
		}
	}
	return false, nil
}

func TestProdCompose_APITrustsNginxFrontDoor(t *testing.T) {
	for _, tc := range []struct{ file, proxySvc, apiSvc, net string }{
		{"../docker-compose.yml", "nginx", "api", "catalogizer-network"},
		{"../deployment/docker-compose.yml", "web", "catalogizer-server", "catalogizer-network"},
	} {
		t.Run(strings.TrimPrefix(tc.file, "../"), func(t *testing.T) {
			raw, err := os.ReadFile(tc.file)
			if err != nil {
				t.Fatal(err)
			}
			var doc composeDoc
			if err := yaml.Unmarshal(raw, &doc); err != nil {
				t.Fatal(err)
			}
			api, ok := doc.Services[tc.apiSvc]
			if !ok {
				t.Fatalf("service %s missing", tc.apiSvc)
			}
			px, ok := doc.Services[tc.proxySvc]
			if !ok {
				t.Fatalf("service %s missing", tc.proxySvc)
			}
			tp := resolveDefault(t, tc.apiSvc+".environment.TRUSTED_PROXIES (shared bucket otherwise)", api.Environment["TRUSTED_PROXIES"])
			if tp == "auto" || strings.Contains(tp, "0.0.0.0/0") || strings.Contains(tp, "::/0") || strings.Contains(tp, ",") {
				t.Fatalf("default %q must be exactly one explicit non-wildcard CIDR", tp)
			}
			pfx, err := netip.ParsePrefix(tp)
			if err != nil || pfx.Bits() != pfx.Addr().BitLen() {
				t.Fatalf("default %q must be a single-host /32", tp)
			}
			if ok, _ := networkAttach(api.Networks, tc.net); !ok {
				t.Fatalf("%s must attach to %s (same network as the proxy)", tc.apiSvc, tc.net)
			}
			ok, ipv := networkAttach(px.Networks, tc.net)
			if !ok {
				t.Fatalf("%s must attach to %s", tc.proxySvc, tc.net)
			}
			addr, err := netip.ParseAddr(resolveDefault(t, tc.proxySvc+" networks."+tc.net+".ipv4_address", ipv))
			if err != nil {
				t.Fatal(err)
			}
			if addr != pfx.Addr() {
				t.Fatalf("%s is pinned to %s but the API trusts %s", tc.proxySvc, addr, pfx)
			}
			n, ok := doc.Networks[tc.net]
			if !ok || len(n.Ipam.Config) == 0 {
				t.Fatalf("network %s has no ipam config", tc.net)
			}
			sub, err := netip.ParsePrefix(resolveDefault(t, "ipam subnet", n.Ipam.Config[0].Subnet))
			if err != nil {
				t.Fatal(err)
			}
			if !sub.Contains(addr) {
				t.Fatalf("proxy address %s is outside subnet %s", addr, sub)
			}
			rng, err := netip.ParsePrefix(resolveDefault(t, "ipam ip_range (dynamic containers could take the proxy address)", n.Ipam.Config[0].IPRange))
			if err != nil {
				t.Fatal(err)
			}
			if !sub.Contains(rng.Addr()) || rng.Bits() < sub.Bits() {
				t.Fatalf("ip_range %s is not inside subnet %s", rng, sub)
			}
			if rng.Contains(addr) {
				t.Fatalf("proxy address %s lies inside the dynamic ip_range %s", addr, rng)
			}
			// behavioural link into the real gin policy
			peer := addr.String()
			if got := clientIPFor(t, tp, peer, "192.168.1.50"); got != "192.168.1.50" {
				t.Fatalf("behind the front door ClientIP=%q want the real client", got)
			}
			if got := clientIPFor(t, tp, peer, "9.9.9.9, 192.168.1.50"); got != "192.168.1.50" {
				t.Fatalf("spoofed XFF must not win: ClientIP=%q", got)
			}
			if got := clientIPFor(t, tp, "198.51.100.7", "9.9.9.9"); got != "198.51.100.7" {
				t.Fatalf("non-proxy peer must not be trusted: ClientIP=%q", got)
			}
			if tc.proxySvc == "web" { // the web image proxies to API_UPSTREAM; the default is the host, bypassing this network
				up := fmt.Sprint(px.Environment["API_UPSTREAM"])
				if up != tc.apiSvc+":8080" {
					t.Fatalf("web API_UPSTREAM=%q; must be %s:8080 so the API sees the pinned proxy address", up, tc.apiSvc)
				}
			}
		})
	}
}

// F-D: every nginx location that proxy_pass-es must forward the client address,
// otherwise the API sees only the proxy (shared bucket) for that path.
func TestNginxConfigs_EveryProxyLocationSetsForwardedFor(t *testing.T) {
	for _, f := range []string{"../config/nginx.conf", "../catalog-web/nginx.conf.template"} {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		lines := strings.Split(string(raw), "\n")
		locs := 0
		for i := 0; i < len(lines); i++ {
			if !regexp.MustCompile(`^\s*location\s.*\{\s*$`).MatchString(lines[i]) {
				continue
			}
			depth, block := 0, ""
			for j := i; j < len(lines); j++ {
				l := strings.SplitN(lines[j], "#", 2)[0]
				depth += strings.Count(l, "{") - strings.Count(l, "}")
				block += l + "\n"
				if depth <= 0 {
					break
				}
			}
			if !strings.Contains(block, "proxy_pass") {
				continue
			}
			locs++
			for _, h := range []string{"proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;", "proxy_set_header X-Real-IP $remote_addr;"} {
				if !strings.Contains(block, h) {
					t.Errorf("%s: %s lacks %q", f, strings.TrimSpace(lines[i]), h)
				}
			}
		}
		if locs == 0 {
			t.Errorf("%s: parsed no proxy_pass location (instrument blind)", f)
		}
	}
}

// Optional: wildcard detector used for a startup warning.
func TestTrustsEverything(t *testing.T) {
	for tp, want := range map[string]bool{"0.0.0.0/0": true, "10.0.0.0/8, ::/0": true, "10.0.0.0/8": false, "": false, "auto": false, "172.28.0.10/32": false} {
		if got := trustsEverything(tp); got != want {
			t.Errorf("trustsEverything(%q)=%v want %v", tp, got, want)
		}
	}
}

// A refused or invalid policy must fail CLOSED (no proxy trusted), not leave
// gin's trust-everything default in place.
func TestApplyTrustedProxies_FailureFailsClosed(t *testing.T) {
	prev := gin.Mode()
	gin.SetMode(gin.ReleaseMode)
	t.Cleanup(func() { gin.SetMode(prev) })
	t.Setenv("TRUSTED_PROXIES_ALLOW_AUTO", "")
	for _, tp := range []string{"auto", "not-a-cidr", "10.0.0.0/8,not-a-cidr"} {
		gin.SetMode(gin.ReleaseMode)
		r := gin.New()
		if err := applyTrustedProxies(r, tp); err == nil {
			t.Fatalf("%q: expected error", tp)
		}
		var got string
		r.GET("/", func(c *gin.Context) { got = c.ClientIP() })
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "10.1.2.3:40000"
		req.Header.Set("X-Forwarded-For", "9.9.9.9")
		r.ServeHTTP(httptest.NewRecorder(), req)
		if got != "10.1.2.3" {
			t.Fatalf("%q: after failure ClientIP=%q; must ignore XFF (fail closed)", tp, got)
		}
	}
}
