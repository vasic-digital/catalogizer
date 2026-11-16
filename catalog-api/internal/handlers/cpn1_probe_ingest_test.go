package handlers

// CAT-CPN-1 round 2 (review F1, operator decision 2026-11-16): POST /api/v1/smb/probe-and-ingest creates storage roots, so a destination on a
// private LAN needs confirm_private_network=true exactly like POST /storage/roots. Real router, real handler, real SMB discovery service, real
// ingester, real SQLite with the production migrations. The unconfirmed refusal happens BEFORE any probe: nothing is dialled, nothing is stored.

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"catalogizer/config"
	"catalogizer/database"
	"catalogizer/internal/netguard"
	"catalogizer/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

const cpn1ProbeCode = "private_network_confirmation_required"

type cpn1Probe struct {
	t  *testing.T
	db *database.DB
	r  *gin.Engine
}

func newCPN1Probe(t *testing.T) *cpn1Probe {
	t.Helper()
	gin.SetMode(gin.TestMode)
	t.Setenv(netguard.AllowCIDRsEnv, "")
	db, err := database.NewConnection(&config.DatabaseConfig{Type: "sqlite", Path: filepath.Join(t.TempDir(), "catalog.db"), EnableWAL: true})
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	require.NoError(t, db.RunMigrations(context.Background()))
	logger := zap.NewNop()
	h := NewProbeAndIngestHandler(services.NewSMBDiscoveryService(logger), services.NewBindingIngester(db, logger), logger)
	r := gin.New()
	r.POST("/api/v1/smb/probe-and-ingest", h.ProbeAndIngest)
	return &cpn1Probe{t: t, db: db, r: r}
}

// do posts body. The request context is already cancelled: a request that passes the confirmation gate reaches the probe, which then fails at
// once instead of dialling a network (so the passes-the-gate cases never leave the machine).
func (p *cpn1Probe) do(body map[string]interface{}) (int, map[string]interface{}, string) {
	p.t.Helper()
	raw, err := json.Marshal(body)
	require.NoError(p.t, err)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/smb/probe-and-ingest", bytes.NewReader(raw)).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	start := time.Now()
	p.r.ServeHTTP(w, req)
	require.Less(p.t, time.Since(start), 5*time.Second, "no network may be dialled")
	var out map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out, w.Body.String()
}

func (p *cpn1Probe) roots() int {
	var n int
	require.NoError(p.t, p.db.QueryRow(`SELECT COUNT(*) FROM storage_roots`).Scan(&n))
	return n
}

func TestCPN1Probe_PrivateHostWithoutConfirmationIs400AndStoresNothing(t *testing.T) {
	for name, extra := range map[string]map[string]interface{}{
		"absent": {},
		"false":  {"confirm_private_network": false},
	} {
		t.Run(name, func(t *testing.T) {
			p := newCPN1Probe(t)
			body := map[string]interface{}{"host": "192.168.77.50"}
			for k, v := range extra {
				body[k] = v
			}
			code, out, raw := p.do(body)
			require.Equal(t, http.StatusBadRequest, code, raw)
			require.Equal(t, cpn1ProbeCode, out["code"], raw)
			require.False(t, strings.Contains(raw, "192.168.77."), "the refusal must not echo the destination: %s", raw)
			require.Equal(t, 0, p.roots(), "an unconfirmed private destination stores nothing")
		})
	}
}

// With the confirmation the request passes the new gate and reaches the (real) probe; the probe fails on the cancelled context, which is 500,
// never the confirmation 400.
func TestCPN1Probe_ConfirmedPrivateHostPassesTheGate(t *testing.T) {
	p := newCPN1Probe(t)
	code, out, raw := p.do(map[string]interface{}{"host": "192.168.77.50", "confirm_private_network": true})
	require.NotEqual(t, cpn1ProbeCode, out["code"], raw)
	require.NotEqual(t, http.StatusBadRequest, code, raw)
	require.Equal(t, 0, p.roots())
}

// Loopback and the metadata address stay refused whatever the flag says; a public address is not asked for a confirmation.
func TestCPN1Probe_LoopbackMetadataRefusedPublicUnchanged(t *testing.T) {
	p := newCPN1Probe(t)
	for _, host := range []string{"127.0.0.1", "169.254.169.254"} {
		for _, flag := range []bool{false, true} {
			code, out, raw := p.do(map[string]interface{}{"host": host, "confirm_private_network": flag})
			require.Equal(t, http.StatusBadRequest, code, raw)
			require.NotEqual(t, cpn1ProbeCode, out["code"], "%s is refused outright, not offered a confirmation: %s", host, raw)
		}
	}
	code, out, raw := p.do(map[string]interface{}{"host": "8.8.8.8"})
	require.NotEqual(t, cpn1ProbeCode, out["code"], raw)
	require.NotEqual(t, http.StatusBadRequest, code, raw)
}
