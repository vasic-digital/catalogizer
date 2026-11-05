package monitoring

import (
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// AZ2-3: the standalone /metrics listener has no authentication, so it must be reachable from
// the loopback address only. Real listener, real HTTP, real second interface address.
func TestStartPrometheusServer_ListensOnLoopbackOnly(t *testing.T) {
	srv := StartPrometheusServer(0, zap.NewNop())
	require.NotNil(t, srv)
	t.Cleanup(func() { _ = srv.Close() })
	host, port, err := net.SplitHostPort(srv.Addr)
	require.NoError(t, err)

	// control: the loopback address answers /metrics
	var resp *http.Response
	for i := 0; i < 20; i++ {
		resp, err = http.Get("http://" + net.JoinHostPort("127.0.0.1", port) + "/metrics")
		if err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	require.NoError(t, err, "control: loopback must reach the listener")
	resp.Body.Close()
	assert.Equal(t, 200, resp.StatusCode)
	assert.True(t, net.ParseIP(host).IsLoopback(), "bound address %q must be loopback", host)

	// a non-loopback address of this host must NOT reach it
	addrs, _ := net.InterfaceAddrs()
	tried := 0
	for _, a := range addrs {
		ipn, ok := a.(*net.IPNet)
		if !ok || ipn.IP.IsLoopback() || ipn.IP.To4() == nil {
			continue
		}
		tried++
		c := http.Client{Timeout: 2 * time.Second}
		r, err := c.Get(fmt.Sprintf("http://%s/metrics", net.JoinHostPort(ipn.IP.String(), port)))
		if err == nil {
			r.Body.Close()
			t.Fatalf("the unauthenticated /metrics listener answered on %s", ipn.IP)
		}
	}
	if tried == 0 {
		t.Log("no non-loopback IPv4 address on this host: only the bound-address assertion ran")
	}
}
