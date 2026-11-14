package middleware

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"runtime/debug"
	"strings"
	"syscall"

	"catalogizer/internal/logredact"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Recovery replaces gin.Recovery() (PX-F17 round 3, R2-1). gin's recovery writes a full request dump to stderr (request line,
// every header except a masked Authorization - Cookie and the raw target with ?ticket= / /logs/share/<token> included) in
// debug mode, and the request line again for http.ErrAbortHandler in release mode. This one NEVER dumps the request: it logs
// only the method, the redacted path+query (logredact.Path), the panic value passed through logredact.ErrorText and the stack
// (frames carry no request data), then answers 500 - or, for a broken connection, just aborts.
func Recovery(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			rec := recover()
			if rec == nil {
				return
			}
			broken := isBrokenConnection(rec)
			if logger != nil {
				logger.Error("panic recovered",
					zap.String("method", c.Request.Method),
					zap.String("path", logredact.PathKnown(c.Request.URL.Path, c.Request.URL.RawQuery, requestKnownSecrets(c))),
					zap.String("panic", panicText(c, rec)),
					zap.Bool("broken_connection", broken),
					zap.ByteString("stack", debug.Stack()),
				)
			}
			if errors.Is(asError(rec), http.ErrAbortHandler) {
				// net/http's own convention: re-panic ErrAbortHandler so the server aborts the connection WITHOUT answering
				// (an empty 200 would be a lie) and without logging a stack. The request was already logged redacted above.
				panic(http.ErrAbortHandler)
			}
			if broken {
				c.Abort() // the peer is gone: nothing can be answered
				return
			}
			c.AbortWithStatus(http.StatusInternalServerError)
		}()
		c.Next()
	}
}

func asError(rec any) error {
	err, _ := rec.(error)
	return err
}

// panicText is the loggable text of a panic value. Errors and strings go through the request's known credentials and the
// free-text redactor; any other value (panic(c), a struct, a pointer) is logged by TYPE only - its default formatting can embed
// request parameters ("[{token S}]").
func panicText(c *gin.Context, rec any) string {
	var text string
	switch v := rec.(type) {
	case error:
		text = v.Error()
	case string:
		text = v
	default:
		return fmt.Sprintf("%T (value not logged)", rec)
	}
	return logredact.ErrorTextKnown(text, requestKnownSecrets(c))
}

func isBrokenConnection(rec any) bool {
	err, ok := rec.(error)
	if !ok {
		return false
	}
	if errors.Is(err, http.ErrAbortHandler) {
		return true
	}
	var ne *net.OpError
	if errors.As(err, &ne) {
		var se *os.SyscallError
		if errors.As(ne.Err, &se) {
			m := strings.ToLower(se.Error())
			return strings.Contains(m, "broken pipe") || strings.Contains(m, "connection reset by peer")
		}
		return errors.Is(ne.Err, syscall.EPIPE) || errors.Is(ne.Err, syscall.ECONNRESET)
	}
	return false
}
