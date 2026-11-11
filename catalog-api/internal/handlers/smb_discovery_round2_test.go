package handlers

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"testing"

	"catalogizer/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func probeRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := NewSMBDiscoveryHandler(nil, zap.NewNop())
	r := gin.New()
	r.Use(gin.Recovery())
	r.POST("/smb/discover", h.DiscoverShares)
	r.POST("/smb/probe", h.ProbeHost)
	r.POST("/smb/test", h.TestConnection)
	r.POST("/smb/browse", h.BrowseShare)
	return r
}

func TestSMBRoutes_BracketedLoopbackFormsRefused(t *testing.T) {
	r := probeRouter()
	for _, h := range []string{"[::1]", "[::ffff:127.0.0.1]", "[0:0:0:0:0:0:0:1]"} {
		for route, body := range map[string]string{
			"/smb/discover": fmt.Sprintf(`{"host":%q,"username":"u","password":"p"}`, h),
			"/smb/probe":    fmt.Sprintf(`{"host":%q}`, h),
			"/smb/test":     fmt.Sprintf(`{"host":%q,"share":"s","username":"u","password":"p"}`, h),
			"/smb/browse":   fmt.Sprintf(`{"host":%q,"share":"s","username":"u","password":"p"}`, h),
		} {
			w := discPost(r, route, body)
			require.Equal(t, http.StatusBadRequest, w.Code, "%s %s -> %s", route, h, w.Body.String())
		}
	}
}

func TestSMBRoutes_ResolverDecidesHost(t *testing.T) {
	old := services.SMBHostResolver
	t.Cleanup(func() { services.SMBHostResolver = old })
	m := map[string][]string{"loop.test": {"127.0.0.1"}, "mixed.test": {"192.0.2.10", "127.0.0.1"}, "nas.test": {"192.0.2.10"}}
	services.SMBHostResolver = func(h string) ([]net.IP, error) {
		v, ok := m[h]
		if !ok {
			return nil, errors.New("nxdomain")
		}
		var out []net.IP
		for _, s := range v {
			out = append(out, net.ParseIP(s))
		}
		return out, nil
	}
	r := probeRouter()
	for _, h := range []string{"loop.test", "mixed.test", "unknown.test"} {
		for route, body := range map[string]string{
			"/smb/discover": fmt.Sprintf(`{"host":%q,"username":"u","password":"p"}`, h),
			"/smb/probe":    fmt.Sprintf(`{"host":%q}`, h),
		} {
			w := discPost(r, route, body)
			require.Equal(t, http.StatusBadRequest, w.Code, "%s %s", route, h)
		}
	}
	// negative control: an all-public name passes vetting and reaches the (nil) service -> recovered 500
	w := discPost(r, "/smb/discover", `{"host":"nas.test","username":"u","password":"p"}`)
	require.Equal(t, http.StatusInternalServerError, w.Code)
}

// R3: the 500 body of /smb/discover must be fixed text. A real service dials "nas.test" (real DNS: NXDOMAIN, fails fast) so the
// service error contains the host name; it must not reach the client.
func TestSMBDiscover_500BodyIsGeneric(t *testing.T) {
	old := services.SMBHostResolver
	t.Cleanup(func() { services.SMBHostResolver = old })
	services.SMBHostResolver = func(string) ([]net.IP, error) { return []net.IP{net.ParseIP("192.0.2.10")}, nil }
	gin.SetMode(gin.TestMode)
	h := NewSMBDiscoveryHandler(services.NewSMBDiscoveryService(zap.NewNop()), zap.NewNop())
	r := gin.New()
	r.POST("/smb/discover", h.DiscoverShares)
	w := discPost(r, "/smb/discover", `{"host":"leakcheck.invalid","username":"u","password":"p"}`)
	require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
	require.JSONEq(t, `{"error":"Failed to discover shares"}`, w.Body.String())
}

// R4: service-error to HTTP status mapping.
func TestSMBErrorStatus_InvalidInputIs400(t *testing.T) {
	st, msg := smbErrorStatus(fmt.Errorf("wrap: %w", services.ErrInvalidSMBInput), "Failed to browse share")
	require.Equal(t, http.StatusBadRequest, st)
	require.Equal(t, "Invalid share, path, host or port", msg)
	st, msg = smbErrorStatus(errors.New("boom 10.0.0.1"), "Failed to browse share")
	require.Equal(t, http.StatusInternalServerError, st)
	require.Equal(t, "Failed to browse share", msg)
}
