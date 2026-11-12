package netguard

// CAT-SECQ1-A: RED reproductions of the SECQ-1 findings B1, B2, B5, B6, B7, B8 written against the API that existed
// before the change (so they compile and FAIL on the base commit), and kept as permanent regression tests (§11.4.135).
// Doubles in this file: a replaced name resolver (lookupIPAddr) - unit class (§11.4.289); the B2 test uses the real
// interface addresses and a real listener on the host.

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

// B1: a redirect from an origin that resolves to a PUBLIC address into the LAN / docker bridge must be refused.
func TestSECQ1A_B1_PublicOriginCannotRedirectIntoLAN(t *testing.T) {
	t.Setenv(AllowCIDRsEnv, "")
	t.Setenv(DenyPrivateEnv, "")
	fixedResolver(t, "93.184.215.14")
	origin, _ := http.NewRequest("GET", "https://webdav.yandex.ru/disk/", nil)
	for _, u := range []string{"http://192.168.1.1/admin", "http://10.0.0.5:9200/_cat/indices", "http://172.17.0.1:6379/", "http://192.168.1.250:8080/api/v1/"} {
		next, _ := http.NewRequest("GET", u, nil)
		if err := OperatorCheckRedirect(next, []*http.Request{origin}); err == nil {
			t.Errorf("redirect public origin -> %s was ALLOWED (B1)", u)
		}
	}
}

// B2: the server's own (non-loopback) interface address is as internal as 127.0.0.1.
func TestSECQ1A_B2_OwnInterfaceAddressRefused(t *testing.T) {
	t.Setenv(AllowCIDRsEnv, "")
	t.Setenv(DenyPrivateEnv, "")
	ln, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		t.Skip(err)
	}
	defer ln.Close()
	var accepted int
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			accepted++
			c.Close()
		}
	}()
	port := ln.Addr().(*net.TCPAddr).Port
	addrs, _ := net.InterfaceAddrs()
	tried := 0
	for _, a := range addrs {
		ipn, ok := a.(*net.IPNet)
		if !ok || ipn.IP.To4() == nil || ipn.IP.IsLoopback() || ipn.IP.IsLinkLocalUnicast() {
			continue
		}
		tried++
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		c, derr := OperatorDialContext(ctx, "tcp", net.JoinHostPort(ipn.IP.String(), fmt.Sprint(port)))
		cancel()
		if derr == nil {
			c.Close()
			t.Errorf("dial to own interface address %s was ALLOWED (B2)", ipn.IP)
		}
	}
	if tried == 0 {
		t.Skip("hardware_not_present: no non-loopback IPv4 interface address on this host")
	}
}

// B5: Azure WireServer is refused on both paths.
func TestSECQ1A_B5_WireServerRefused(t *testing.T) {
	t.Setenv(AllowCIDRsEnv, "")
	t.Setenv(DenyPrivateEnv, "")
	ip := net.ParseIP("168.63.129.16")
	if !IsBlockedOperatorIP(ip) {
		t.Error("operator path allows 168.63.129.16")
	}
	if !IsBlockedIPStrict(ip) || !IsBlockedIP(ip) {
		t.Error("strict path allows 168.63.129.16")
	}
	if IsBlockedOperatorIP(net.ParseIP("168.63.129.17")) {
		t.Error("negative control: 168.63.129.17 must not be refused")
	}
}

// B6: invalid and over-broad allow-list entries are logged / ignored.
func TestSECQ1A_B6_AllowListHygiene(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	resetAllowListWarnings()
	t.Setenv(AllowBroadEnv, "")
	t.Setenv(AllowCIDRsEnv, "0.0.0.0/0, ::/0, 192.168.1.0/24, not-a-cidr")
	nets := allowedNets()
	if len(nets) != 1 || nets[0].String() != "192.168.1.0/24" {
		t.Errorf("allowedNets = %v, want only 192.168.1.0/24 (broad entries refused)", nets)
	}
	out := buf.String()
	for _, want := range []string{"not-a-cidr", "0.0.0.0/0", "::/0"} {
		if !strings.Contains(out, want) {
			t.Errorf("log does not mention %q: %q", want, out)
		}
	}
	if IsBlockedOperatorIP(net.ParseIP("127.0.0.1")) != true {
		t.Error("0.0.0.0/0 re-admitted loopback")
	}
	// explicit opt-in keeps the broad entry (documented escape)
	t.Setenv(AllowBroadEnv, "1")
	if len(allowedNets()) != 3 {
		t.Errorf("with %s=1 broad entries must be kept: %v", AllowBroadEnv, allowedNets())
	}
}

// B7: numeric spellings are refused at the strict URL check, too.
func TestSECQ1A_B7_StrictCheckURLNumericSpellings(t *testing.T) {
	t.Setenv(AllowCIDRsEnv, "")
	for _, u := range []string{"http://127.1/", "http://2130706433/", "http://0xa9fea9fe/", "http://0177.0.0.1/", "http://127.0.1/"} {
		if _, err := CheckURL(context.Background(), u, "http", "https"); err == nil {
			t.Errorf("CheckURL accepted %s (B7)", u)
		}
	}
	// control: ordinary name still accepted when it does not resolve, and a name with a numeric NON-last label
	fixedResolver(t, "93.184.215.14")
	for _, u := range []string{"https://example.com/", "https://10.example.com/", "https://93.184.215.14/"} {
		if _, err := CheckURL(context.Background(), u, "http", "https"); err != nil {
			t.Errorf("control %s refused: %v", u, err)
		}
	}
}

// B8: non-ASCII host names are refused on both paths; punycode is fine.
func TestSECQ1A_B8_NonASCIIHostRefused(t *testing.T) {
	t.Setenv(AllowCIDRsEnv, "")
	fixedResolver(t, "93.184.215.14")
	for _, h := range []string{"１２７.０.０.１", "127。0。0。1", "ⓛⓞⓒⓐⓛⓗⓞⓢⓣ", "exаmple.com"} {
		u := "http://" + h + "/"
		if _, err := CheckOperatorURL(context.Background(), u, "http", "https"); err == nil {
			t.Errorf("CheckOperatorURL accepted %q (B8)", h)
		}
		if _, err := CheckURL(context.Background(), u, "http", "https"); err == nil {
			t.Errorf("CheckURL accepted %q (B8)", h)
		}
	}
	for _, u := range []string{"http://xn--bcher-kva.example/", "http://example.com/"} {
		if _, err := CheckOperatorURL(context.Background(), u, "http", "https"); err != nil {
			t.Errorf("control %s refused: %v", u, err)
		}
	}
}
