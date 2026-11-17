package main

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// ALTSVC5: the HTTPS (http.Server.ServeTLS -> http2ConfigureServer WRITES its *tls.Config) and the
// HTTP/3 server (http3.ConfigureTLSConfig -> tls.Config.Clone() READS its *tls.Config) must not
// share one *tls.Config. Before the fix both servers held the same pointer and `go test -race`
// reported a DATA RACE in 5 of 6 runs of TestAltSvc*.

// TestAltSvc5_NoSharedTLSConfig is deterministic (no race detector needed): the two servers
// must hold distinct, equivalent configs.
func TestAltSvc5_NoSharedTLSConfig(t *testing.T) {
	setH3Advertised(t, false)
	_, https, h3 := startStackOrSkip(t, http.NotFoundHandler())
	require.NotNil(t, https.TLSConfig)
	require.NotNil(t, h3.TLSConfig)
	require.NotSame(t, https.TLSConfig, h3.TLSConfig, "ALTSVC5: http and http3 servers must not share one *tls.Config")
	require.Len(t, h3.TLSConfig.Certificates, 1)
	require.Len(t, https.TLSConfig.Certificates, 1)
	require.Equal(t, []string{"h3", "h2", "http/1.1"}, h3.TLSConfig.NextProtos)
}

// TestAltSvc5_ConcurrentStartRaceFree starts and stops the stack repeatedly; under -race the
// shared-config defect shows as DATA RACE (the failing signal is the race detector).
func TestAltSvc5_ConcurrentStartRaceFree(t *testing.T) {
	setH3Advertised(t, false)
	for i := 0; i < 20; i++ {
		p := freeTCPUDPPort(t)
		https, h3 := startH3Stack(zap.NewNop(), http.NotFoundHandler(), "127.0.0.1", p, testCert(t))
		if https == nil || h3 == nil {
			stopStack(t, https, h3)
			continue
		}
		time.Sleep(15 * time.Millisecond) // let both Serve goroutines reach their config accesses
		stopStack(t, https, h3)
	}
}
