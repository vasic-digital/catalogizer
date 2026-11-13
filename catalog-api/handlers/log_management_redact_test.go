package handlers

import (
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"catalogizer/internal/logging"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

// unit-class (HANDLERS-RED-1): the path/known-secret reads moved into path_params.go helpers must still keep a
// share token out of the server log, both in the path field and inside the service error text.
func TestLogLogServiceError_ShareTokenNeverReachesLog(t *testing.T) {
	const token = "S3cr3tShareTok3n42"
	core, logs := observer.New(zapcore.DebugLevel)
	prev := logging.Logger
	logging.Logger = zap.New(core)
	t.Cleanup(func() { logging.Logger = prev })

	r := httptest.NewRequest("GET", "/api/v1/logs/share/"+token+"?ticket=QueryTicket99", nil)
	r = mux.SetURLVars(r, map[string]string{"token": token})
	logLogServiceError(r, errors.New("lookup share "+token+" failed"), 404)

	entries := logs.All()
	require.Len(t, entries, 1)
	var dump strings.Builder
	dump.WriteString(entries[0].Message)
	for _, f := range entries[0].Context {
		dump.WriteString(" " + f.Key + "=" + f.String)
	}
	out := dump.String()
	assert.NotContains(t, out, token, "share token leaked into the log")
	assert.NotContains(t, out, "QueryTicket99")
	assert.Contains(t, out, "/api/v1/logs/share/:redacted", "path field keeps its shape with the secret segment redacted")
}

func TestRequestLogHelpers_NilSafe(t *testing.T) {
	assert.Equal(t, "", requestLogPath(nil))
	assert.Nil(t, requestKnownSecrets(nil))
}

// A credential known only from a sensitive route parameter or a sensitive query value (not from a path segment after a
// secret-introducing segment) must also vanish from the logged error text.
func TestLogLogServiceError_RouteParamAndQuerySecretsRedactedFromErrorText(t *testing.T) {
	const routeTok, queryTic = "RouteOnlyTok7788", "QueryOnlyTic9911"
	core, logs := observer.New(zapcore.DebugLevel)
	prev := logging.Logger
	logging.Logger = zap.New(core)
	t.Cleanup(func() { logging.Logger = prev })

	r := httptest.NewRequest("GET", "/api/v1/logs/thing/"+routeTok+"?ticket="+queryTic, nil)
	r = mux.SetURLVars(r, map[string]string{"token": routeTok})
	logLogServiceError(r, errors.New("bad "+routeTok+" and "+queryTic), 400)

	entries := logs.All()
	require.Len(t, entries, 1)
	var dump strings.Builder
	for _, f := range entries[0].Context {
		if f.Key == "error" { // the path field only redacts secret-introducing segments (pre-existing, not under test here)
			dump.WriteString(f.String)
		}
	}
	require.NotEmpty(t, dump.String())
	assert.NotContains(t, dump.String(), routeTok)
	assert.NotContains(t, dump.String(), queryTic)
}
