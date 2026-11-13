package middleware

import (
	"bytes"
	"errors"
	"fmt"
	stdlog "log"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strings"
	"syscall"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

// F4/RM9: isBrokenConnection is decided by error class, and Recovery reports it in the log entry.
func TestIsBrokenConnection_Table(t *testing.T) {
	opErr := func(e error) error { return &net.OpError{Op: "write", Err: e} }
	cases := []struct {
		name string
		rec  any
		want bool
	}{
		{"nil-error-value", errors.New("plain"), false},
		{"string", "boom", false},
		{"abort-handler", http.ErrAbortHandler, true},
		{"wrapped-abort", fmt.Errorf("x: %w", http.ErrAbortHandler), true},
		{"epipe-syscallerror", opErr(os.NewSyscallError("write", syscall.EPIPE)), true},
		{"econnreset-syscallerror", opErr(os.NewSyscallError("read", syscall.ECONNRESET)), true},
		{"epipe-bare", opErr(syscall.EPIPE), true},
		{"econnreset-bare", opErr(syscall.ECONNRESET), true},
		{"other-syscallerror", opErr(os.NewSyscallError("write", syscall.EACCES)), false},
		{"other-bare", opErr(syscall.EACCES), false},
		{"opError-wrapped", fmt.Errorf("w: %w", opErr(syscall.EPIPE)), true},
	}
	for _, c := range cases {
		if got := isBrokenConnection(c.rec); got != c.want {
			t.Errorf("%s: isBrokenConnection = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestRecovery_ReportsBrokenConnectionFlagAndStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	run := func(pv any) (bool, int, bool) {
		core, logs := observer.New(zap.DebugLevel)
		r := gin.New()
		r.Use(Recovery(zap.New(core)))
		r.GET("/x", func(c *gin.Context) { panic(pv) })
		w := httptest.NewRecorder()
		aborted := false
		func() {
			defer func() {
				if x := recover(); x != nil {
					aborted = x == http.ErrAbortHandler
				}
			}()
			r.ServeHTTP(w, httptest.NewRequest("GET", "/x", nil))
		}()
		es := logs.All()
		if len(es) != 1 {
			t.Fatalf("want one entry, got %d", len(es))
		}
		b, _ := es[0].ContextMap()["broken_connection"].(bool)
		return b, w.Code, aborted
	}
	if b, code, _ := run("plain boom"); b || code != http.StatusInternalServerError {
		t.Errorf("plain panic: broken=%v code=%d", b, code)
	}
	if b, code, _ := run(&net.OpError{Op: "write", Err: syscall.EPIPE}); !b || code == http.StatusInternalServerError {
		t.Errorf("broken pipe: broken=%v code=%d (must flag broken and not answer 500)", b, code)
	}
	if b, _, aborted := run(http.ErrAbortHandler); !b || !aborted {
		t.Errorf("abort handler: broken=%v re-panicked=%v", b, aborted)
	}
}

// F4/RM13: the access-log entry carries EXACTLY this field set, so an added raw field (query string, user header ...) fails.
func TestLogger_ExactFieldSet(t *testing.T) {
	gin.SetMode(gin.TestMode)
	core, logs := observer.New(zap.InfoLevel)
	r := gin.New()
	r.Use(Logger(zap.New(core)))
	r.GET("/api/v1/logs/share/:token", func(c *gin.Context) { c.Status(200) })
	req := httptest.NewRequest("GET", "/api/v1/logs/share/SHARETOKENVALUE?ticket=TICKETVALUE&a=1", nil)
	req.Header.Set("Authorization", "Bearer AUTHVALUE123")
	r.ServeHTTP(httptest.NewRecorder(), req)
	es := logs.TakeAll()
	if len(es) != 1 {
		t.Fatalf("control: want one entry, got %d", len(es))
	}
	var keys []string
	for k, v := range es[0].ContextMap() {
		keys = append(keys, k)
		for _, s := range []string{"SHARETOKENVALUE", "TICKETVALUE", "AUTHVALUE123"} {
			if strings.Contains(fmt.Sprint(v), s) {
				t.Errorf("field %s leaks %s: %v", k, s, v)
			}
		}
	}
	sort.Strings(keys)
	if got, want := strings.Join(keys, ","), "ip,latency,method,path,status,user_agent"; got != want {
		t.Errorf("field set = %s, want exactly %s", got, want)
	}
	if p := es[0].ContextMap()["path"]; p != "/api/v1/logs/share/:redacted?ticket=REDACTED&a=1" {
		t.Errorf("path = %v", p)
	}
}

// F4/MEH2 + F3: ErrorHandler logs the error text redacted, with the request's own credentials removed; the text without
// credentials stays diagnosable.
func TestErrorHandler_LogsRedactedErrorText(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var buf bytes.Buffer
	old := stdlog.Writer()
	stdlog.SetOutput(&buf)
	defer stdlog.SetOutput(old)
	r := gin.New()
	r.Use(ErrorHandler())
	r.GET("/api/v1/logs/share/:token", func(c *gin.Context) {
		_ = c.Error(fmt.Errorf("lookup share %q failed: dial postgres://u:DBPASSWORD1@db/x password=INNERPW99", c.Param("token")))
	})
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/api/v1/logs/share/SHARETOKENVALUE", nil))
	out := buf.String()
	for _, s := range []string{"SHARETOKENVALUE", "DBPASSWORD1", "INNERPW99"} {
		if strings.Contains(out, s) {
			t.Errorf("ErrorHandler log leaks %s: %s", s, out)
		}
	}
	if !strings.Contains(out, "lookup share") || !strings.Contains(out, "failed") || !strings.Contains(out, "REDACTED") {
		t.Errorf("diagnostic text lost or nothing redacted: %s", out)
	}
}

// F3: panic values - strings, errors with %q-quoted tokens, route parameters and arbitrary values (panic(c)).
func TestRecovery_PanicValueShapes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const tok = "SHARETOKENVALUE"
	for name, mk := range map[string]func(c *gin.Context) any{
		"param":       func(c *gin.Context) any { return c.Param("token") },
		"errorf-q":    func(c *gin.Context) any { return fmt.Errorf("lookup share %q", c.Param("token")) },
		"errorf-bare": func(c *gin.Context) any { return fmt.Errorf("lookup share %s not found", c.Param("token")) },
		"context":     func(c *gin.Context) any { return c },
		"params":      func(c *gin.Context) any { return c.Params },
		"struct":      func(c *gin.Context) any { return struct{ Key, Value string }{"token", c.Param("token")} },
	} {
		t.Run(name, func(t *testing.T) {
			core, logs := observer.New(zap.DebugLevel)
			r := gin.New()
			r.Use(Recovery(zap.New(core)))
			r.GET("/api/v1/logs/share/:token", func(c *gin.Context) { panic(mk(c)) })
			captured := captureAll(t, func() {
				r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/api/v1/logs/share/"+tok, nil))
			})
			var sb strings.Builder
			for _, e := range logs.All() {
				sb.WriteString(e.Message)
				for _, f := range e.Context {
					sb.WriteString(" " + f.Key + "=" + f.String + fmt.Sprint(f.Interface))
				}
			}
			if strings.Contains(sb.String(), tok) || strings.Contains(captured, tok) {
				t.Errorf("panic value leaks the token: %s", sb.String())
			}
			if len(logs.All()) != 1 {
				t.Errorf("panic not logged")
			}
		})
	}
}

// A panic value that is NOT the request's own (a secret loaded from the database inside a struct) has no recognisable shape and
// is not a known secret: only logging non-error, non-string panic values by TYPE hides it (kills the fmt.Sprint(rec) default).
func TestRecovery_PanicStructWithForeignSecretIsLoggedByTypeOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const foreign = "DBLOADEDSECRETVALUE99"
	core, logs := observer.New(zap.DebugLevel)
	r := gin.New()
	r.Use(Recovery(zap.New(core)))
	r.GET("/x", func(c *gin.Context) { panic(struct{ Blob string }{foreign}) })
	captured := captureAll(t, func() { r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/x", nil)) })
	var sb strings.Builder
	for _, e := range logs.All() {
		sb.WriteString(e.Message)
		for _, f := range e.Context {
			sb.WriteString(" " + f.Key + "=" + f.String + fmt.Sprint(f.Interface))
		}
	}
	if strings.Contains(sb.String(), foreign) || strings.Contains(captured, foreign) {
		t.Fatalf("foreign struct secret leaked: %s", sb.String())
	}
	if !strings.Contains(sb.String(), "value not logged") {
		t.Fatalf("expected type-only panic text, got: %s", sb.String())
	}
}
