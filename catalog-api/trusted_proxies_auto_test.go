package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

// F1 (CAT-TRUSTEDPROXIES-PROD-1): the shipped production compose must make the
// API trust exactly the nginx front door. Reads the REAL compose file, resolves
// the default of api's TRUSTED_PROXIES, and drives the real gin policy with the
// nginx container's fixed address as the TCP peer.
func TestProdCompose_APITrustsNginxFrontDoor(t *testing.T) {
	for _, tc := range []struct{ file, proxySvc, apiSvc string }{
		{"../docker-compose.yml", "nginx", "api"},
		{"../deployment/docker-compose.yml", "web", "catalogizer-server"},
	} {
		t.Run(filepath.Base(filepath.Dir(tc.file))+"/"+filepath.Base(tc.file), func(t *testing.T) {
			raw, err := os.ReadFile(tc.file)
			if err != nil {
				t.Fatal(err)
			}
			var doc struct {
				Services map[string]struct {
					Environment map[string]any `yaml:"environment"`
					Networks    any            `yaml:"networks"`
				} `yaml:"services"`
			}
			if err := yaml.Unmarshal(raw, &doc); err != nil {
				t.Fatal(err)
			}
			api, ok := doc.Services[tc.apiSvc]
			if !ok {
				t.Fatalf("service %s missing", tc.apiSvc)
			}
			tpv, ok := api.Environment["TRUSTED_PROXIES"].(string)
			if !ok || tpv == "" {
				t.Fatalf("%s: %s.environment.TRUSTED_PROXIES is unset => ClientIP is the proxy container IP => one shared login bucket for all clients", tc.file, tc.apiSvc)
			}
			def := regexp.MustCompile(`^\$\{[A-Z_]+:-([^}]*)\}$`).FindStringSubmatch(tpv)
			if def == nil {
				t.Fatalf("TRUSTED_PROXIES %q must be an overridable ${VAR:-default}", tpv)
			}
			tp := def[1]
			if tp == "" || tp == "auto" || strings.Contains(tp, "0.0.0.0/0") || strings.Contains(tp, "::/0") {
				t.Fatalf("default %q must be an explicit non-wildcard CIDR", tp)
			}
			// proxy peer = the first address in the default list
			peer := strings.SplitN(strings.TrimSpace(strings.Split(tp, ",")[0]), "/", 2)[0]
			if got := clientIPFor(t, tp, peer, "192.168.1.50"); got != "192.168.1.50" {
				t.Fatalf("behind the front door ClientIP=%q want the real client", got)
			}
			if got := clientIPFor(t, tp, peer, "9.9.9.9, 192.168.1.50"); got != "192.168.1.50" {
				t.Fatalf("spoofed XFF must not win: ClientIP=%q", got)
			}
			if got := clientIPFor(t, tp, "198.51.100.7", "9.9.9.9"); got != "198.51.100.7" {
				t.Fatalf("non-proxy peer must not be trusted: ClientIP=%q", got)
			}
			// the proxy service must actually be pinned to that address
			if !regexp.MustCompile(`(?s)` + tc.proxySvc + `:.*?ipv4_address:\s*\$\{[A-Z_]+:-` + regexp.QuoteMeta(peer) + `\}`).Match(raw) {
				t.Fatalf("service %s is not pinned to ipv4_address %s via an overridable default", tc.proxySvc, peer)
			}
		})
	}
}

// A refused or invalid policy must fail CLOSED (no proxy trusted), not leave
// gin's trust-everything default in place.
func TestApplyTrustedProxies_FailureFailsClosed(t *testing.T) {
	prev := gin.Mode()
	gin.SetMode(gin.ReleaseMode)
	t.Cleanup(func() { gin.SetMode(prev) })
	t.Setenv("TRUSTED_PROXIES_ALLOW_AUTO", "")
	for _, tp := range []string{"auto", "not-a-cidr"} {
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
