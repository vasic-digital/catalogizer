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
					zap.String("path", logredact.Path(c.Request.URL.Path, c.Request.URL.RawQuery)),
					zap.String("panic", logredact.ErrorText(fmt.Sprint(rec))),
					zap.Bool("broken_connection", broken),
					zap.ByteString("stack", debug.Stack()),
				)
			}
			if broken {
				c.Abort()
				return
			}
			c.AbortWithStatus(http.StatusInternalServerError)
		}()
		c.Next()
	}
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
