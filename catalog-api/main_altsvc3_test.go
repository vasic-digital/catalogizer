package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/quic-go/quic-go/http3"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// ALTSVC2 review follow-ups (F1, F2, F3, F5). The tests use ephemeral ports, never 28443, so
// they cannot collide with other processes; when no port that is free on both TCP and UDP is
// found, the test is SKIPPED as INCONCLUSIVE (never a PASS).

func testCert(t *testing.T) func() (tls.Certificate, error) {
	t.Helper()
	return func() (tls.Certificate, error) {
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			return tls.Certificate{}, err
		}
		tpl := &x509.Certificate{
			SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "altsvc3-test"},
			NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
			KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
			IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
		}
		der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &key.PublicKey, key)
		if err != nil {
			return tls.Certificate{}, err
		}
		return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}, nil
	}
}

// freeTCPUDPPort returns a port that is free on 127.0.0.1 for both TCP and UDP, or marks the test
// INCONCLUSIVE (skip) after a bounded wait.
func freeTCPUDPPort(t *testing.T) int {
	t.Helper()
	for i := 0; i < 40; i++ {
		l, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		p := l.Addr().(*net.TCPAddr).Port
		pc, uerr := net.ListenPacket("udp", "127.0.0.1:"+strconv.Itoa(p))
		l.Close()
		if uerr == nil {
			pc.Close()
			return p
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Skip("INCONCLUSIVE: no port free on both TCP and UDP within the bounded wait")
	return 0
}

func stopStack(t *testing.T, https *http.Server, h3 *http3.Server) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if https != nil {
		_ = https.Shutdown(ctx)
	}
	if h3 != nil {
		_ = h3.Close()
	}
}

func handlerForStack(t *testing.T) http.Handler {
	t.Helper()
	return buildRouter(routerTestDeps())
}

// F1 + F2: certificate error: nothing is started and nothing is advertised (mutant: Store before the cert load).
func TestAltSvcStartH3Stack_CertErrorNotAdvertised(t *testing.T) {
	setH3Advertised(t, false)
	p := freeTCPUDPPort(t)
	https, h3 := startH3Stack(zap.NewNop(), http.NotFoundHandler(), "127.0.0.1", p,
		func() (tls.Certificate, error) { return tls.Certificate{}, errors.New("no certificate") })
	require.Nil(t, https)
	require.Nil(t, h3)
	require.False(t, h3Advertised.Load(), "no certificate: HTTP/3 must not be advertised")
	// nothing bound: the ports are still free
	l, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(p))
	require.NoError(t, err, "no TCP listener may be left behind")
	l.Close()
	// control: the same call with a certificate advertises (the instrument can see the flag)
	https, h3 = startH3Stack(zap.NewNop(), http.NotFoundHandler(), "127.0.0.1", p, testCert(t))
	t.Cleanup(func() { stopStack(t, https, h3) })
	require.NotNil(t, h3)
	require.True(t, h3Advertised.Load(), "control: with a certificate and a confirmed bind it is advertised")
}

// F1: the UDP (QUIC) port is taken: the bind fails, so Alt-Svc must NOT be advertised.
func TestAltSvcStartH3Stack_UDPBindFailureNotAdvertised(t *testing.T) {
	setH3Advertised(t, false)
	p := freeTCPUDPPort(t)
	occupied, err := net.ListenPacket("udp", "127.0.0.1:"+strconv.Itoa(p))
	require.NoError(t, err)
	t.Cleanup(func() { occupied.Close() })
	https, h3 := startH3Stack(zap.NewNop(), http.NotFoundHandler(), "127.0.0.1", p, testCert(t))
	t.Cleanup(func() { stopStack(t, https, h3) })
	require.Nil(t, h3, "UDP bind failed: no HTTP/3 server")
	require.False(t, h3Advertised.Load(), "UDP bind failed: HTTP/3 must not be advertised")
}

// F1: the flag is set only once the sockets are really bound, and is cleared on a later HTTP/3 failure path
// (the bound state is observable: both ports are taken after the call returns).
func TestAltSvcStartH3Stack_AdvertisedOnlyAfterConfirmedBind(t *testing.T) {
	setH3Advertised(t, false)
	p := freeTCPUDPPort(t)
	https, h3 := startH3Stack(zap.NewNop(), http.NotFoundHandler(), "127.0.0.1", p, testCert(t))
	t.Cleanup(func() { stopStack(t, https, h3) })
	require.NotNil(t, https)
	require.NotNil(t, h3)
	require.True(t, h3Advertised.Load())
	_, uerr := net.ListenPacket("udp", "127.0.0.1:"+strconv.Itoa(p))
	require.Error(t, uerr, "advertised, so the UDP port must already be bound by the server")
	_, terr := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(p))
	require.Error(t, terr, "the TCP port must already be bound by the server")
}

func h2Client() *http.Client {
	return &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig:   &tls.Config{InsecureSkipVerify: true, NextProtos: []string{"h2"}}, //nolint:gosec // test-only self-signed certificate
			ForceAttemptHTTP2: true,
		},
	}
}

// F3: a REAL socket: HTTP/2 over TLS to the real router served by startH3Stack (the code main() runs).
// A header set after the handler has written the body (defer after c.Next, mutant R5) is invisible to a
// ResponseRecorder test but lost here.
func TestAltSvcRealSocketHTTP2(t *testing.T) {
	setH3Advertised(t, false)
	p := freeTCPUDPPort(t)
	https, h3 := startH3Stack(zap.NewNop(), handlerForStack(t), "127.0.0.1", p, testCert(t))
	t.Cleanup(func() { stopStack(t, https, h3) })
	require.NotNil(t, https)
	base := fmt.Sprintf("https://127.0.0.1:%d", p)
	c := h2Client()
	t.Cleanup(c.CloseIdleConnections)

	for _, tc := range []struct {
		path string
		code int
	}{{"/health", http.StatusOK}, {"/no-such-route-altsvc3", http.StatusNotFound}} {
		resp, err := c.Get(base + tc.path)
		require.NoError(t, err)
		resp.Body.Close()
		require.Equal(t, 2, resp.ProtoMajor, "control: the real connection is HTTP/2 over TLS")
		require.Equal(t, tc.code, resp.StatusCode)
		require.Equal(t, altSvcWant, resp.Header.Get("Alt-Svc"), "%s over real HTTP/2", tc.path)
	}
}

// F3 golden-FALSE on the real socket: while the flag is off the header is absent on a real connection.
func TestAltSvcRealSocketNotAdvertisedWhenFlagOff(t *testing.T) {
	setH3Advertised(t, false)
	p := freeTCPUDPPort(t)
	https, h3 := startH3Stack(zap.NewNop(), handlerForStack(t), "127.0.0.1", p, testCert(t))
	t.Cleanup(func() { stopStack(t, https, h3) })
	require.NotNil(t, https)
	h3Advertised.Store(false) // simulate the cleared state after an HTTP/3 failure
	c := h2Client()
	t.Cleanup(c.CloseIdleConnections)
	resp, err := c.Get(fmt.Sprintf("https://127.0.0.1:%d/health", p))
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, 2, resp.ProtoMajor)
	require.Empty(t, resp.Header.Get("Alt-Svc"))
}

// F5: the documented exceptions (see altSvcMiddleware): /discovery, a CORS preflight and a response
// aborted by an earlier middleware carry no Alt-Svc even over TLS while HTTP/3 is advertised.
func TestAltSvcDocumentedExceptions(t *testing.T) {
	setH3Advertised(t, true)
	e := buildRouter(routerTestDeps())

	disc := altSvcDo(e, http.MethodGet, "/discovery", true)
	require.Equal(t, http.StatusOK, disc.Code)
	require.Empty(t, disc.Header().Get("Alt-Svc"), "/discovery is registered before the chain")

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodOptions, "/api/v1/auth/login", nil)
	r.TLS = &tls.ConnectionState{}
	r.Header.Set("Origin", "http://localhost:3000")
	r.Header.Set("Access-Control-Request-Method", "POST")
	e.ServeHTTP(w, r)
	require.Equal(t, http.StatusNoContent, w.Code, "control: CORS preflight is answered by the CORS middleware")
	require.Empty(t, w.Header().Get("Alt-Svc"), "the preflight aborts before altSvcMiddleware")

	// control: the same engine does advertise on an ordinary routed response
	require.Equal(t, altSvcWant, altSvcDo(e, http.MethodGet, "/health", true).Header().Get("Alt-Svc"))
}
