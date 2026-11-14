package netguard

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func sccCountingServer(t *testing.T) (*httptest.Server, *int32) {
	t.Helper()
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { atomic.AddInt32(&hits, 1) }))
	t.Cleanup(srv.Close)
	return srv, &hits
}

func sccGet(t *testing.T, tr *http.Transport, url string) error {
	t.Helper()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", url, nil)
	resp, err := (&http.Client{Transport: tr}).Do(req)
	if err == nil {
		resp.Body.Close()
	}
	return err
}

func TestStrictClassTransport_BothPoliciesApply(t *testing.T) {
	srv, hits := sccCountingServer(t)
	t.Setenv(AllowCIDRsEnv, "127.0.0.1/32")

	if err := sccGet(t, StrictClassTransport(ClassPrivate), srv.URL); err != nil {
		t.Fatalf("private approval + allow-list must reach the fixture: %v", err)
	}
	before := atomic.LoadInt32(hits)

	err := sccGet(t, StrictClassTransport(ClassPublic), srv.URL)
	if err == nil || !errors.Is(err, ErrBlocked) {
		t.Fatalf("public approval must refuse an allow-listed loopback address, got %v", err)
	}
	if atomic.LoadInt32(hits) != before {
		t.Fatal("the refused request reached the fixture")
	}

	t.Setenv(AllowCIDRsEnv, "")
	err = sccGet(t, StrictClassTransport(ClassPrivate), srv.URL)
	if err == nil || !errors.Is(err, ErrBlocked) {
		t.Fatalf("without the allow-list loopback stays refused (never looser than Transport()), got %v", err)
	}
	if atomic.LoadInt32(hits) != before {
		t.Fatal("the refused request reached the fixture")
	}
}

func TestStrictClassTransport_UnsetClassIsPrivateAndProxyIgnored(t *testing.T) {
	tr := StrictClassTransport(ClassNever)
	if tr.Proxy != nil {
		t.Fatal("environment proxies are ignored (a proxy would resolve the target itself)")
	}
	t.Setenv(AllowCIDRsEnv, "")
	if _, err := tr.DialContext(context.Background(), "tcp", net.JoinHostPort("127.0.0.1", "1")); err == nil || !errors.Is(err, ErrBlocked) {
		t.Fatalf("got %v", err)
	}
}
