package handlers

// CAT-CPN-1 follow-ups CPN1-M1 / M2 / M3 (review of 2026-11-16). Real router, real handler, real SMB discovery service (host resolution, vetting and
// identity loop are the production code), real ingester, real SQLite with the production migrations, stored rows read back from the database.
// UNIT-CLASS seam, stated honestly: the SMB session itself (dial + NTLM + share listing) is replaced through SMBDiscoveryService.WithProbeSeam, because
// no SMB server is available hermetically; the seam records the exact addresses the probe would dial. A live SMB ingest remains UNCONFIRMED.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"

	"catalogizer/config"
	"catalogizer/database"
	"catalogizer/internal/netguard"
	"catalogizer/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

type cpn1Stored struct {
	t      *testing.T
	db     *database.DB
	r      *gin.Engine
	mu     sync.Mutex
	shares []services.SMBShareInfo
	dialed [][]string // addresses handed to the probe, per attempt
}

func newCPN1Stored(t *testing.T) *cpn1Stored {
	t.Helper()
	gin.SetMode(gin.TestMode)
	t.Setenv(netguard.AllowCIDRsEnv, "")
	t.Setenv("CATALOGIZER_IDENTITY_COUNT", "0")
	db, err := database.NewConnection(&config.DatabaseConfig{Type: "sqlite", Path: filepath.Join(t.TempDir(), "catalog.db"), EnableWAL: true})
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	require.NoError(t, db.RunMigrations(context.Background()))
	s := &cpn1Stored{t: t, db: db}
	logger := zap.NewNop()
	disc := services.NewSMBDiscoveryService(logger).WithProbeSeam(func(_ context.Context, _ string, ips []net.IP, _ services.SMBIdentity) ([]services.SMBShareInfo, error) {
		var a []string
		for _, ip := range ips {
			a = append(a, ip.String())
		}
		s.mu.Lock()
		s.dialed = append(s.dialed, a)
		s.mu.Unlock()
		if s.shares != nil {
			return s.shares, nil
		}
		return []services.SMBShareInfo{{ShareName: "media"}}, nil
	})
	h := NewProbeAndIngestHandler(disc, services.NewBindingIngester(db, logger), logger)
	s.r = gin.New()
	s.r.POST("/api/v1/smb/probe-and-ingest", h.ProbeAndIngest)
	return s
}

func (s *cpn1Stored) post(body map[string]interface{}) (int, map[string]interface{}) {
	s.t.Helper()
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/smb/probe-and-ingest", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.r.ServeHTTP(w, req)
	var out map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

// stored returns (class, source, host) of every storage root.
func (s *cpn1Stored) stored() [][3]string {
	s.t.Helper()
	rows, err := s.db.Query(`SELECT approved_class, approved_class_source, COALESCE(host,'') FROM storage_roots ORDER BY id`)
	require.NoError(s.t, err)
	defer rows.Close()
	var out [][3]string
	for rows.Next() {
		var r [3]string
		require.NoError(s.t, rows.Scan(&r[0], &r[1], &r[2]))
		out = append(out, r)
	}
	return out
}

func withSMBResolver(t *testing.T, fn func(string) ([]net.IP, error)) {
	t.Helper()
	old := services.SMBHostResolver
	t.Cleanup(func() { services.SMBHostResolver = old })
	services.SMBHostResolver = fn
}

// M1 + M3: a confirmed private host is stored private, source operator.
func TestCPN1Stored_ConfirmedPrivateIsPrivateOperator(t *testing.T) {
	s := newCPN1Stored(t)
	code, out := s.post(map[string]interface{}{"host": "192.168.77.50", "confirm_private_network": true})
	require.Equal(t, http.StatusOK, code, out)
	require.Equal(t, [][3]string{{"private", "operator", "192.168.77.50"}}, s.stored())
	_, hasPartial := out["partial"]
	require.False(t, hasPartial, "partial must be absent on success")
}

// M1 + M3: a public host is stored public, source auto, with no confirmation asked.
func TestCPN1Stored_PublicIsPublicAuto(t *testing.T) {
	s := newCPN1Stored(t)
	code, out := s.post(map[string]interface{}{"host": "8.8.8.8"})
	require.Equal(t, http.StatusOK, code, out)
	require.Equal(t, [][3]string{{"public", "auto", "8.8.8.8"}}, s.stored())
}

// M2: the probe dials exactly the addresses that were classified. The name resolves once (public); the stored class and the dialled
// addresses agree, and no second lookup happens.
func TestCPN1Stored_NameResolvedOnceAndDialledAsClassified(t *testing.T) {
	var mu sync.Mutex
	calls := 0
	withSMBResolver(t, func(string) ([]net.IP, error) {
		mu.Lock()
		defer mu.Unlock()
		calls++
		if calls == 1 {
			return []net.IP{net.ParseIP("8.8.4.4")}, nil
		}
		return []net.IP{net.ParseIP("192.168.77.9")}, nil // what a rebinding attacker answers from the second lookup on
	})
	s := newCPN1Stored(t)
	code, out := s.post(map[string]interface{}{"host": "nas.rebind.test"})
	require.Equal(t, http.StatusOK, code, out)
	require.Equal(t, 1, calls, "the name must be resolved exactly once per request")
	require.Equal(t, [][]string{{"8.8.4.4"}}, s.dialed, "the probe dials the classified (public) address, never a later answer")
	require.Equal(t, [][3]string{{"public", "auto", "nas.rebind.test"}}, s.stored())
}

// M2: a name whose (only) answer is private is refused without confirmation: 400, nothing dialled, nothing stored; with the confirmation it is
// stored private/operator and dialled at that address.
func TestCPN1Stored_PrivateNameNeedsConfirmationAndIsNotDialledWithout(t *testing.T) {
	var mu sync.Mutex
	calls := 0
	withSMBResolver(t, func(string) ([]net.IP, error) {
		mu.Lock()
		defer mu.Unlock()
		calls++
		return []net.IP{net.ParseIP("8.8.4.4"), net.ParseIP("10.1.2.3")}, nil // one private answer makes the whole name private
	})
	s := newCPN1Stored(t)
	code, out := s.post(map[string]interface{}{"host": "mixed.rebind.test"})
	require.Equal(t, http.StatusBadRequest, code, out)
	require.Equal(t, cpn1ProbeCode, out["code"])
	require.Empty(t, s.dialed, "nothing is dialled before the confirmation")
	require.Empty(t, s.stored())
	require.Equal(t, 1, calls)

	code, out = s.post(map[string]interface{}{"host": "mixed.rebind.test", "confirm_private_network": true})
	require.Equal(t, http.StatusOK, code, out)
	require.Equal(t, [][3]string{{"private", "operator", "mixed.rebind.test"}}, s.stored())
	require.Equal(t, [][]string{{"8.8.4.4", "10.1.2.3"}}, s.dialed)
}

// CPN1R-m1: re-ingesting an existing root leaves its approved class untouched (never widens, never narrows). A name that was public the first
// time and private the second, re-ingested with the confirmation, must keep public/auto.
func TestCPN1Stored_ReingestLeavesExistingRootClassUntouched(t *testing.T) {
	var mu sync.Mutex
	ip := "8.8.4.4"
	withSMBResolver(t, func(string) ([]net.IP, error) {
		mu.Lock()
		defer mu.Unlock()
		return []net.IP{net.ParseIP(ip)}, nil
	})
	s := newCPN1Stored(t)
	code, out := s.post(map[string]interface{}{"host": "nas.flip.test"})
	require.Equal(t, http.StatusOK, code, out)
	require.Equal(t, [][3]string{{"public", "auto", "nas.flip.test"}}, s.stored())

	mu.Lock()
	ip = "192.168.77.9"
	mu.Unlock()
	code, out = s.post(map[string]interface{}{"host": "nas.flip.test", "confirm_private_network": true})
	require.Equal(t, http.StatusOK, code, out)
	require.Nil(t, out["new_roots"], "the existing root is not new (omitted when zero)")
	require.Equal(t, [][3]string{{"public", "auto", "nas.flip.test"}}, s.stored(), "existing approval must be untouched")
}

// CPN1R-m2: when the class cannot be stored the freshly inserted root is removed again and the failure is logged, not swallowed.
func TestCPN1Stored_ClassStoreFailureRemovesRootAndIsLogged(t *testing.T) {
	s := newCPN1Stored(t)
	core, logs := observer.New(zapcore.DebugLevel)
	bi := services.NewBindingIngester(s.db, zap.New(core))
	_, err := bi.IngestProbeResultWithClass(context.Background(),
		&services.SMBProbeResult{Host: "h.test", Authenticated: true, Shares: []services.SMBShareInfo{{ShareName: "a"}}},
		services.ApprovedClassAssignment{Class: netguard.ClassPublic, Source: "bogus-source"})
	require.Error(t, err)
	require.Empty(t, s.stored(), "no root may be left behind with an approval nobody made")
	require.Equal(t, 1, logs.FilterMessageSnippet("approved class could not be stored").Len(), "the rollback must be logged")
}

// CPN1R-m2: a failing rollback delete is reported in the log, never swallowed. The delete is made to fail by a trigger.
func TestCPN1Stored_RollbackDeleteFailureIsLogged(t *testing.T) {
	s := newCPN1Stored(t)
	_, err := s.db.Exec(`CREATE TRIGGER no_del BEFORE DELETE ON storage_roots BEGIN SELECT RAISE(ABORT,'delete blocked'); END`)
	require.NoError(t, err)
	core, logs := observer.New(zapcore.DebugLevel)
	bi := services.NewBindingIngester(s.db, zap.New(core))
	_, err = bi.IngestProbeResultWithClass(context.Background(),
		&services.SMBProbeResult{Host: "h.test", Authenticated: true, Shares: []services.SMBShareInfo{{ShareName: "a"}}},
		services.ApprovedClassAssignment{Class: netguard.ClassPublic, Source: "bogus-source"})
	require.Error(t, err)
	require.Equal(t, 1, logs.FilterMessageSnippet("rollback delete failed").Len(), "a failed rollback delete must be logged")
}

// CPN1R-m4 (decision, documented): a multi-share ingest is per-share, not all-or-nothing. A class-store failure on a later share removes
// only that share's root; earlier roots stay, each with its own correct approval (no root ever lacks its approval). The error response
// reports what was committed (partial=true, bound_shares, new_roots) so the operator is not told nothing happened.
func TestCPN1Stored_PartialMultiShareIsReported(t *testing.T) {
	s := newCPN1Stored(t)
	s.shares = []services.SMBShareInfo{{ShareName: "good"}, {ShareName: "bad"}}
	_, err := s.db.Exec(`CREATE TRIGGER fail_bad BEFORE UPDATE OF approved_class ON storage_roots WHEN NEW.name LIKE '%:bad' BEGIN SELECT RAISE(ABORT,'class store blocked'); END`)
	require.NoError(t, err)
	code, out := s.post(map[string]interface{}{"host": "8.8.8.8"})
	require.Equal(t, http.StatusInternalServerError, code, out)
	require.Equal(t, true, out["partial"])
	require.EqualValues(t, 2, out["bound_shares"])
	require.EqualValues(t, 1, out["new_roots"])
	require.Equal(t, [][3]string{{"public", "auto", "8.8.8.8"}}, s.stored(), "only the share whose approval was stored remains")
}

// CPN1R2-c: an ErrInvalidSMBInput coming back from the vetted-IP probe is a client input error: 400 with the host-not-allowed text, not 500.
func TestCPN1Stored_ProbeInvalidInputIs400(t *testing.T) {
	s := newCPN1Stored(t)
	disc := services.NewSMBDiscoveryService(zap.NewNop()).WithProbeSeam(func(context.Context, string, []net.IP, services.SMBIdentity) ([]services.SMBShareInfo, error) {
		return nil, fmt.Errorf("%w: port not allowed", services.ErrInvalidSMBInput)
	})
	h := NewProbeAndIngestHandler(disc, services.NewBindingIngester(s.db, zap.NewNop()), zap.NewNop())
	s.r = gin.New()
	s.r.POST("/api/v1/smb/probe-and-ingest", h.ProbeAndIngest)
	code, out := s.post(map[string]interface{}{"host": "8.8.8.8"})
	require.Equal(t, http.StatusBadRequest, code, out)
	require.Equal(t, hostAllowedErr, out["error"])
	require.Empty(t, s.stored())
}
