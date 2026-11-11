package handlers

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// SMB-FU-2: the discovery routes (main.go /smb/discover, /smb/test, /smb/browse) must apply the same SSRF host check as ProbeHost, a
// port check, and share/path validation BEFORE the service is touched. The service is nil: a refused request never reaches it, so a
// 400 proves the refusal happened in the handler; a valid request reaches the nil service, panics, and the Recovery middleware turns that
// into 500 (negative control: proves the validation does not refuse everything).

func discRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := NewSMBDiscoveryHandler(nil, zap.NewNop())
	r := gin.New()
	r.Use(gin.Recovery())
	r.POST("/smb/discover", h.DiscoverShares)
	r.POST("/smb/test", h.TestConnection)
	r.POST("/smb/browse", h.BrowseShare)
	return r
}

func discPost(r *gin.Engine, route, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", route, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestSMBDiscoveryRoutes_GoldenBadRefusedWith400(t *testing.T) {
	r := discRouter()
	cred := `"username":"u","password":"p"`
	cases := map[string][]string{
		"/smb/discover": {
			`{"host":"127.0.0.1",` + cred + `}`,
			`{"host":"169.254.169.254",` + cred + `}`,
		},
		"/smb/test": {
			`{"host":"127.0.0.1","share":"s",` + cred + `}`,
			`{"host":"192.0.2.10","port":22,"share":"s",` + cred + `}`,
			`{"host":"192.0.2.10","share":"IPC$",` + cred + `}`,
			`{"host":"192.0.2.10","share":"C$",` + cred + `}`,
			`{"host":"192.0.2.10","share":"s/..",` + cred + `}`,
		},
		"/smb/browse": {
			`{"host":"127.0.0.1","share":"s",` + cred + `}`,
			`{"host":"192.0.2.10","port":22,"share":"s",` + cred + `}`,
			`{"host":"192.0.2.10","share":"IPC$",` + cred + `}`,
			`{"host":"192.0.2.10","share":"C$/x",` + cred + `}`,
			`{"host":"192.0.2.10","share":"s","path":"../..",` + cred + `}`,
			`{"host":"192.0.2.10","share":"s","path":"a/../../b",` + cred + `}`,
		},
	}
	for route, bodies := range cases {
		for _, b := range bodies {
			w := discPost(r, route, b)
			require.Equal(t, http.StatusBadRequest, w.Code, "%s %s -> %s", route, b, w.Body.String())
		}
	}
}

func TestSMBDiscoveryRoutes_GoldenGoodPassesValidation_NegativeControl(t *testing.T) {
	r := discRouter()
	cred := `"username":"u","password":"p"`
	for route, b := range map[string]string{
		"/smb/discover": `{"host":"192.0.2.10",` + cred + `}`,
		"/smb/test":     `{"host":"192.0.2.10","share":"share/sub",` + cred + `}`,
		"/smb/browse":   `{"host":"192.0.2.10","port":445,"share":"share/sub","path":"docs",` + cred + `}`,
	} {
		w := discPost(r, route, b)
		require.NotEqual(t, http.StatusBadRequest, w.Code, "%s must pass validation; got %s", route, w.Body.String())
		require.Equal(t, http.StatusInternalServerError, w.Code, "%s reached the (nil) service and was recovered", route)
	}
}
