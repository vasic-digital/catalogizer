package middleware

import (
	"bytes"
	"fmt"
	"io"
	stdlog "log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

const (
	rcvShareTok = "SHARETOKSECRET"
	rcvTicket   = "TICKETSECRET"
	rcvCookie   = "COOKIESECRET"
	rcvAuth     = "AUTHSECRETVALUE"
)

// captureAll runs fn with os.Stderr, gin's default writers and the std logger redirected into one buffer.
func captureAll(t *testing.T, fn func()) string {
	t.Helper()
	var buf bytes.Buffer
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { _, _ = io.Copy(&buf, r); close(done) }()
	oldErr, oldW, oldEW, oldLogOut := os.Stderr, gin.DefaultWriter, gin.DefaultErrorWriter, stdlog.Writer()
	os.Stderr, gin.DefaultWriter, gin.DefaultErrorWriter = w, w, w
	stdlog.SetOutput(w)
	defer func() {
		os.Stderr, gin.DefaultWriter, gin.DefaultErrorWriter = oldErr, oldW, oldEW
		stdlog.SetOutput(oldLogOut)
	}()
	fn()
	_ = w.Close()
	<-done
	return buf.String()
}

type recoveryRun struct {
	captured string // stderr + gin writers + std log
	logged   string // everything the zap logger recorded (messages + all fields)
	status   int
}

func runShareRoute(t *testing.T, mode string, recovery func(*zap.Logger) gin.HandlerFunc, panicVal any) recoveryRun {
	t.Helper()
	old := gin.Mode()
	gin.SetMode(mode)
	defer gin.SetMode(old)
	core, logs := observer.New(zap.DebugLevel)
	var rec recoveryRun
	rec.captured = captureAll(t, func() {
		router := gin.New()
		router.Use(recovery(zap.New(core)))
		router.GET("/api/v1/logs/share/:token", func(c *gin.Context) { panic(panicVal) })
		req := httptest.NewRequest(http.MethodGet, "/api/v1/logs/share/"+rcvShareTok+"?ticket="+rcvTicket, nil)
		req.Header.Set("Cookie", "sid="+rcvCookie)
		req.Header.Set("Authorization", "Bearer "+rcvAuth)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		rec.status = w.Code
	})
	var sb strings.Builder
	for _, e := range logs.All() {
		sb.WriteString(e.Message)
		for _, f := range e.Context {
			sb.WriteString(fmt.Sprintf(" %s=%s", f.Key, f.String))
			switch v := f.Interface.(type) {
			case []byte:
				sb.Write(v)
			case nil:
			default:
				sb.WriteString(fmt.Sprint(v))
			}
		}
		sb.WriteString("\n")
	}
	rec.logged = sb.String()
	return rec
}

func ownRecovery(l *zap.Logger) gin.HandlerFunc { return Recovery(l) }

func assertNoSecrets(t *testing.T, what, text string) {
	t.Helper()
	for _, s := range []string{rcvShareTok, rcvTicket, rcvCookie, rcvAuth} {
		if strings.Contains(text, s) {
			t.Errorf("%s leaks %q:\n%s", what, s, text)
		}
	}
}

// R2-1: a real panic and a real http.ErrAbortHandler on the share route, in BOTH gin modes: nothing sensitive reaches
// stderr, gin's writers, the std logger or the zap logger; the panic is still logged (with the redacted path).
func TestRecovery_NoRequestDumpInEitherMode(t *testing.T) {
	for _, mode := range []string{gin.DebugMode, gin.ReleaseMode} {
		for name, pv := range map[string]any{
			"plain-panic":   "boom password=PANICPW99",
			"abort-handler": http.ErrAbortHandler,
			"error-panic":   fmt.Errorf("db failed: postgres://u:DBPASSWORD1@h/x"),
		} {
			t.Run(mode+"/"+name, func(t *testing.T) {
				r := runShareRoute(t, mode, ownRecovery, pv)
				assertNoSecrets(t, "captured output", r.captured)
				assertNoSecrets(t, "zap log", r.logged)
				for _, s := range []string{"PANICPW99", "DBPASSWORD1"} {
					if strings.Contains(r.logged, s) || strings.Contains(r.captured, s) {
						t.Errorf("panic value leaked %q", s)
					}
				}
				if !strings.Contains(r.logged, "panic recovered") || !strings.Contains(r.logged, "/api/v1/logs/share/:redacted?ticket=REDACTED") {
					t.Errorf("recovered panic not logged with redacted path:\n%s", r.logged)
				}
				if name != "abort-handler" && r.status != http.StatusInternalServerError {
					t.Errorf("status = %d, want 500", r.status)
				}
			})
		}
	}
}

// Control needle: the capture instrument sees what stock gin.Recovery() writes (debug mode dumps the cookie and the raw target;
// release mode dumps the raw target for ErrAbortHandler). If this ever stops leaking, gin changed: re-review, do not silently
// drop the own recovery.
func TestRecovery_StockGinRecoveryControlNeedle(t *testing.T) {
	stock := func(*zap.Logger) gin.HandlerFunc { return gin.Recovery() }
	r := runShareRoute(t, gin.DebugMode, stock, "boom")
	if !strings.Contains(r.captured, rcvShareTok) || !strings.Contains(r.captured, rcvCookie) {
		t.Fatalf("instrument blind: stock gin.Recovery debug output did not show the secrets:\n%s", r.captured)
	}
	r = runShareRoute(t, gin.ReleaseMode, stock, http.ErrAbortHandler)
	if !strings.Contains(r.captured, rcvShareTok) {
		t.Fatalf("instrument blind: stock gin.Recovery release/abort output did not show the target:\n%s", r.captured)
	}
}
