package main

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

const altSvcWant = `h3=":28443"; ma=86400`

// altSvcDo serves one request through the real router (buildRouter, exactly the
// engine main() serves). tlsReq marks the request as received over TLS.
func altSvcDo(e http.Handler, method, path string, tlsReq bool) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(method, path, nil)
	if tlsReq {
		r.TLS = &tls.ConnectionState{}
	}
	e.ServeHTTP(w, r)
	return w
}

func setH3Advertised(t *testing.T, v bool) {
	t.Helper()
	prev := h3Advertised.Load()
	h3Advertised.Store(v)
	t.Cleanup(func() { h3Advertised.Store(prev) })
}

// TestAltSvcReachOnRealRouter is the CAT-ALTSVC-1 reproduction turned guard. RED
// (before the fix, with main()'s late router.Use): a routed 200 over TLS had no
// Alt-Svc, only the 404 (NoRoute) chain did. Now it is in buildRouter's chain.
func TestAltSvcReachOnRealRouter(t *testing.T) {
	setH3Advertised(t, true)
	e := buildRouter(routerTestDeps())

	nf := altSvcDo(e, http.MethodGet, "/no-such-route-altsvc", true)
	require.Equal(t, http.StatusNotFound, nf.Code)
	require.Equal(t, altSvcWant, nf.Header().Get("Alt-Svc"), "control: 404 chain over TLS")

	ok := altSvcDo(e, http.MethodGet, "/health", true)
	require.Equal(t, http.StatusOK, ok.Code)
	require.Equal(t, altSvcWant, ok.Header().Get("Alt-Svc"), "routed 200 over TLS must advertise HTTP/3")
}

// golden-FALSE: HTTP/3 listener not enabled (certificate missing): never advertised.
func TestAltSvcNotAdvertisedWhenH3Disabled(t *testing.T) {
	setH3Advertised(t, false)
	e := buildRouter(routerTestDeps())
	require.Empty(t, altSvcDo(e, http.MethodGet, "/health", true).Header().Get("Alt-Svc"))
	require.Empty(t, altSvcDo(e, http.MethodGet, "/no-such-route-altsvc", true).Header().Get("Alt-Svc"))
}

// Plain HTTP (the cleartext listener shares the engine): never advertised.
func TestAltSvcNotAdvertisedOverPlainHTTP(t *testing.T) {
	setH3Advertised(t, true)
	e := buildRouter(routerTestDeps())
	require.Empty(t, altSvcDo(e, http.MethodGet, "/health", false).Header().Get("Alt-Svc"))
	require.Empty(t, altSvcDo(e, http.MethodGet, "/no-such-route-altsvc", false).Header().Get("Alt-Svc"))
}
