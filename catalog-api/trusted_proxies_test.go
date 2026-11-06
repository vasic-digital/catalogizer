package main

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// clientIPFor drives the REAL gin engine configured by applyTrustedProxies.
func clientIPFor(t *testing.T, tp, peer string, xff ...string) string {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	if err := applyTrustedProxies(r, tp); err != nil {
		t.Fatalf("applyTrustedProxies(%q): %v", tp, err)
	}
	var got string
	r.GET("/", func(c *gin.Context) { got = c.ClientIP() })
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = net.JoinHostPort(peer, "40000")
	for _, v := range xff {
		req.Header.Add("X-Forwarded-For", v)
	}
	r.ServeHTTP(httptest.NewRecorder(), req)
	return got
}

// Topology measured live on QA stack 3 (docs/qa/security-live-20261106): API peer
// behind the nginx front door is the host LAN address; nginx APPENDS its own
// view of the client ($proxy_add_x_forwarded_for).
const fdProxy = "192.168.1.115"

func TestClientIP_FrontDoor(t *testing.T) {
	cases := []struct {
		name, tp, peer string
		xff            []string
		want           string
	}{
		{"remote LAN client via trusted nginx keeps own bucket", fdProxy + "/32", fdProxy, []string{"192.168.1.50"}, "192.168.1.50"},
		{"two remote clients are distinct", fdProxy + "/32", fdProxy, []string{"192.168.1.51"}, "192.168.1.51"},
		{"spoofed leftmost XFF is ignored (rightmost untrusted wins)", fdProxy + "/32", fdProxy, []string{"9.9.9.9, 192.168.1.50"}, "192.168.1.50"},
		{"host-local client appears as pasta alias, spoof cannot override", fdProxy + "/32", fdProxy, []string{"9.9.9.9, 169.254.1.2"}, "169.254.1.2"},
		{"untrusted peer: XFF ignored", fdProxy + "/32", "198.51.100.7", []string{"9.9.9.9"}, "198.51.100.7"},
		{"empty TRUSTED_PROXIES: XFF ignored", "", fdProxy, []string{"9.9.9.9"}, fdProxy},
		{"loopback not trusted unless listed", fdProxy + "/32", "127.0.0.1", []string{"9.9.9.9"}, "127.0.0.1"},
		{"list with spaces and empties", " " + fdProxy + "/32 , ,", fdProxy, []string{"192.168.1.50"}, "192.168.1.50"},
		{"auto trusts RFC1918 peer", "auto", "10.1.2.3", []string{"203.0.113.9"}, "203.0.113.9"},
		{"auto does not trust public peer", "auto", "198.51.100.7", []string{"203.0.113.9"}, "198.51.100.7"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := clientIPFor(t, c.tp, c.peer, c.xff...); got != c.want {
				t.Fatalf("ClientIP=%q want %q", got, c.want)
			}
		})
	}
}

func TestApplyTrustedProxies_RejectsGarbage(t *testing.T) {
	if err := applyTrustedProxies(gin.New(), "not-a-cidr"); err == nil {
		t.Fatal("expected error for invalid CIDR")
	}
}
