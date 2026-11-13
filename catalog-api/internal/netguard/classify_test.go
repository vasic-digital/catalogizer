package netguard

import (
	"context"
	"errors"
	"testing"
)

// CAT-SECQ1-B: automatic classification of an endpoint host into the class it is approved for.
func TestClassifyHost(t *testing.T) {
	envClean(t)
	mapResolver(t, map[string][]string{
		"nas.lan":    {"192.168.1.5"},
		"cloud.test": {"93.184.216.34"},
		"mixed.test": {"93.184.216.34", "10.1.2.3"},
		"v6lan.test": {"fd12:3456::5"},
		"loop.test":  {"127.0.0.1"},
		"meta.test":  {"169.254.169.254"},
		"split.test": {"93.184.216.34", "127.0.0.1"},
	})
	for _, c := range []struct {
		host string
		want Class
		err  bool
	}{
		{"192.168.1.5", ClassPrivate, false},
		{"100.64.0.9", ClassPrivate, false}, // CGNAT
		{"93.184.216.34", ClassPublic, false},
		{"[2606:4700::1111]", ClassPublic, false},
		{"fd12:3456::5", ClassPrivate, false},
		{"nas.lan", ClassPrivate, false},
		{"cloud.test", ClassPublic, false},
		{"mixed.test", ClassPrivate, false}, // one private answer is enough to approve for private, never for public
		{"v6lan.test", ClassPrivate, false},
		{"127.0.0.1", ClassNever, true},
		{"loop.test", ClassNever, true},
		{"meta.test", ClassNever, true},
		{"split.test", ClassNever, true}, // any never-class answer refuses the approval
		{"10.99.0.2", ClassNever, true},  // the server's own interface address (envClean fakes 10.99.0.2/24)
		{"", ClassNever, true},
		{"  ", ClassNever, true},
		{"0177.0.0.1", ClassNever, true}, // numeric spelling
		{"exämple.test", ClassNever, true},
		{"nxdomain.test", ClassNever, true},
		{"::ffff:127.0.0.1", ClassNever, true},
		{"64:ff9b::7f00:1", ClassNever, true},
		{"64:ff9b::c0a8:105", ClassPrivate, false},
	} {
		got, err := ClassifyHost(context.Background(), c.host)
		if got != c.want || (err != nil) != c.err {
			t.Errorf("ClassifyHost(%q) = %v, %v; want %v, err=%v", c.host, got, err, c.want, c.err)
		}
		if err != nil && c.host != "nxdomain.test" && !errors.Is(err, ErrBlocked) {
			t.Errorf("ClassifyHost(%q): error %v does not wrap ErrBlocked", c.host, err)
		}
	}
}

func TestClassifyHostHonoursAllowList(t *testing.T) {
	envClean(t)
	t.Setenv(AllowCIDRsEnv, "10.99.0.2/32,127.0.0.1/32")
	for _, h := range []string{"10.99.0.2", "127.0.0.1"} {
		if got, err := ClassifyHost(context.Background(), h); err != nil || got != ClassPrivate {
			t.Errorf("allow-listed %s = %v, %v; want ClassPrivate", h, got, err)
		}
	}
}

func TestClassifyEndpointUsesHostThenURL(t *testing.T) {
	envClean(t)
	mapResolver(t, map[string][]string{"cloud.test": {"93.184.216.34"}})
	ctx := context.Background()
	if c, err := ClassifyEndpoint(ctx, "192.168.1.5", "https://cloud.test/dav"); err != nil || c != ClassPrivate {
		t.Errorf("host wins over URL: %v, %v", c, err)
	}
	if c, err := ClassifyEndpoint(ctx, "", "https://cloud.test:8443/dav"); err != nil || c != ClassPublic {
		t.Errorf("URL host: %v, %v", c, err)
	}
	for _, raw := range []string{"", "not a url", "/relative/path", "http://"} {
		if c, err := ClassifyEndpoint(ctx, "", raw); err == nil || c != ClassNever {
			t.Errorf("ClassifyEndpoint(\"\", %q) = %v, %v; want ClassNever and an error", raw, c, err)
		}
	}
}
