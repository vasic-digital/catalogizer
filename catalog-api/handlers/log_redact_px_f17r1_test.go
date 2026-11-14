package handlers

import (
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"catalogizer/internal/logging"
	"catalogizer/internal/logredact"
	"catalogizer/models"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

// productionErrorText is the error-text pipeline production logs through (logLogServiceError calls logredact.ErrorTextKnown with the
// request's known secrets; none are known here). HYG-F8: the tests below exercise THAT path, not the unused redactLogErrorText wrapper.
func productionErrorText(s string) string { return logredact.ErrorTextKnown(s, nil) }

// PX-F17 review round 1, F2: class-wide credential redaction. Each leak input names the secret that must vanish.
func TestPXF17R1_RedactLogErrorText_Table(t *testing.T) {
	type tc struct{ name, in, secret string }
	leaks := []tc{
		{"quoted-with-spaces", `login failed password="pass with spaces" for u`, "with spaces"},
		{"bearer-jwt-header", "Authorization: Bearer eyJhbGciOi.payloadpart.sigpart99", "eyJhbGciOi"},
		{"basic-b64-header", "Authorization: Basic dXNlcjpwYXNzd29yZA==", "dXNlcjpwYXNzd29yZA"},
		{"access_token", "refresh failed access_token=abcDEF123456", "abcDEF123456"},
		{"client_secret", "client_secret=s3cr3tvalue&grant=x", "s3cr3tvalue"},
		{"db_password", "db_password=hunter2hunter2 host=x", "hunter2hunter2"},
		{"cookie-header", "Cookie: session_id=SESSVALUE123; theme=dark", "SESSVALUE123"},
		{"set-cookie", "Set-Cookie: sid=SIDVALUE456; Path=/", "SIDVALUE456"},
		{"empty-user-userinfo", "dial redis://:onlypass@h:6379", "onlypass"},
		{"user-only-userinfo", "clone https://ghp_TOKEN12345@github.com/x/y", "ghp_TOKEN12345"},
		{"bare-bearer", "upstream said: bearer abcdef0123456789xyz rejected", "abcdef0123456789xyz"},
		{"json-quoted-escaped", `{"api_key": "k\"ey with space"}`, "with space"},
		{"x-amz", "X-Amz-Security-Token: FwoGZXIvYXdz", "FwoGZXIvYXdz"},
		{"unterminated-quote", `password="oops never closed`, "oops"},
		{"private_key", "private_key=PKSECRET99", "PKSECRET99"},
		{"cookie-header-opaque-names", "Cookie: a=VALUEONE; b=VALUETWO", "VALUETWO"},
	}
	control := []tc{ // passed before the redesign: must keep passing (control needle)
		{"postgres-userinfo", "dial postgres://alice:S3cretPW@db:5432/x", "S3cretPW"},
		{"wrapped-inner", "outer: inner: password=InnerPW9 end", "InnerPW9"},
		{"json-password", `{"password": "jsonpw1"}`, "jsonpw1"},
		{"upper", "PASSWORD=UPPERPW1", "UPPERPW1"},
		{"x-api-key", "X-Api-Key: keyval123", "keyval123"},
		{"query-token", "GET /x?token=QTOK123&a=1", "QTOK123"},
		{"multiline", "password=\nMLPW777", "MLPW777"},
	}
	for _, c := range append(leaks, control...) {
		t.Run(c.name, func(t *testing.T) {
			out := productionErrorText(c.in)
			assert.NotContains(t, out, c.secret, "in=%q out=%q", c.in, out)
			assert.Contains(t, out, "REDACTED")
		})
	}
	// Benign prose without a credential shape stays readable (documented false-positive policy).
	for _, in := range []string{"connection refused", "token count exceeded", "basic configuration failed", "no session available", "dial tcp 10.0.0.1:5432: i/o timeout", "sql: no rows in result set"} {
		assert.Equal(t, in, productionErrorText(in), "benign text must not change")
	}
	// Non-secret content next to a secret survives.
	out := productionErrorText("host=db1 password=pw123456 port=5432")
	assert.Contains(t, out, "host=db1")
	assert.Contains(t, out, "port=5432")
	// A quoted value ends at its closing quote: the text after it survives (the unterminated-quote rule must not take over).
	out = productionErrorText(`password="pass with spaces" next=ok`)
	assert.Contains(t, out, "next=ok")
}

// F3: the logged error text is capped, and a secret straddling the cut is never exposed.
func TestPXF17R1_RedactLogErrorText_Capped(t *testing.T) {
	big := strings.Repeat("x", 1<<20)
	start := time.Now()
	out := productionErrorText(big + " password=ENDSECRET")
	assert.LessOrEqual(t, len(out), logErrMaxBytes+64)
	assert.Contains(t, out, "[truncated ")
	assert.NotContains(t, out, "ENDSECRET")
	assert.Less(t, time.Since(start), 5*time.Second)
	// secret that straddles the 2048 boundary
	pad := strings.Repeat("y", logErrMaxBytes-6)
	out = productionErrorText(pad + " password=STRADDLESECRET tail")
	assert.NotContains(t, out, "STRAD")
	// multibyte at the cut stays valid UTF-8
	out = productionErrorText(strings.Repeat("é", 2000))
	assert.True(t, strings.ToValidUTF8(out, "?") == out)
}

// F1: a share token in the request path must appear nowhere in the captured server log, on both GetLogShare error paths.
func TestPXF17R1_ShareTokenNeverLogged(t *testing.T) {
	const token = "SHARETOKEN_abcdef0123"
	for _, collectionFails := range []bool{false, true} {
		t.Run(fmt.Sprintf("collectionFails=%v", collectionFails), func(t *testing.T) {
			core, logs := observer.New(zap.DebugLevel)
			prev := logging.Logger
			logging.Logger = zap.New(core)
			t.Cleanup(func() { logging.Logger = prev })

			svc := new(MockLogManagementService)
			h := &LogManagementHandler{logManagementService: svc, authService: new(MockLogManagementAuthService)}
			if collectionFails {
				svc.On("GetLogShare", token).Return(&models.LogShare{CollectionID: 3, UserID: 2}, nil)
				svc.On("GetLogCollection", 3, 2).Return(nil, sql.ErrNoRows)
			} else {
				svc.On("GetLogShare", token).Return(nil, sql.ErrNoRows)
			}
			r := mux.NewRouter()
			r.HandleFunc("/api/v1/logs/share/{token}", h.GetLogShare).Methods("GET")
			req := httptest.NewRequest("GET", "/api/v1/logs/share/"+token+"?x=1", nil)
			rr := httptest.NewRecorder()
			r.ServeHTTP(rr, req)
			require.Equal(t, http.StatusNotFound, rr.Code)

			entries := logs.All()
			require.NotEmpty(t, entries, "the failure is logged (control: the log is not empty)")
			for _, e := range entries {
				assert.NotContains(t, e.Message, token)
				for k, v := range e.ContextMap() {
					assert.NotContains(t, fmt.Sprint(v), token, "field %s", k)
				}
			}
			site, _ := entries[0].ContextMap()["site"].(string)
			assert.Contains(t, site, "/api/v1/logs/share/", "the site still identifies the route")
		})
	}
}

// Round-3 F3: the service error text may carry the request's own bare credential ("lookup share S"): it is removed by
// known-secret substitution, not by shape.
func TestPXF17R1_ShareTokenInServiceErrorTextNeverLogged(t *testing.T) {
	const token = "SHARETOKEN_abcdef0123"
	core, logs := observer.New(zap.DebugLevel)
	prev := logging.Logger
	logging.Logger = zap.New(core)
	t.Cleanup(func() { logging.Logger = prev })
	svc := new(MockLogManagementService)
	h := &LogManagementHandler{logManagementService: svc, authService: new(MockLogManagementAuthService)}
	svc.On("GetLogShare", token).Return(nil, fmt.Errorf("lookup share %s: %w", token, sql.ErrNoRows))
	r := mux.NewRouter()
	r.HandleFunc("/api/v1/logs/share/{token}", h.GetLogShare).Methods("GET")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest("GET", "/api/v1/logs/share/"+token, nil))
	require.Equal(t, http.StatusNotFound, rr.Code)
	es := logs.All()
	require.Len(t, es, 1)
	errText, _ := es[0].ContextMap()["error"].(string)
	assert.NotContains(t, errText, token)
	assert.Contains(t, errText, "lookup share")
}
