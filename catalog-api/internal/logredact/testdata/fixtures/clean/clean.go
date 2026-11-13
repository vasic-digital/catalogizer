// Package clean holds legitimate uses the typed scanner must NOT report (false-positive guard).
package clean

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	obslogging "digital.vasic.observability/pkg/logging"

	"catalogizer/internal/logredact"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

var vec = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "c"}, []string{"path"})

func normalize(c *gin.Context) string {
	if fp := c.FullPath(); fp != "" {
		return fp
	}
	return logredact.SecretPath(c.Request.URL.Path)
}

func RedactedLog(c *gin.Context) {
	log.Print(logredact.Path(c.Request.URL.Path, c.Request.URL.RawQuery))
	log.Print(logredact.SecretPath(c.Request.URL.Path))
}

func Routing(c *gin.Context) {
	p := c.Request.URL.Path
	if p == "/metrics" || strings.HasPrefix(p, "/api/") {
		c.Next()
	}
	switch p {
	case "/a":
		c.Status(http.StatusOK)
	}
}

func BenignParam(c *gin.Context) {
	log.Printf("id=%s page=%s", c.Param("id"), c.Query("page"))
	log.Printf("%s %d", c.Request.Method, c.Writer.Status())
}

func MetricLabel(c *gin.Context) { vec.WithLabelValues(normalize(c)).Inc() }

func Lengths(c *gin.Context, l *zap.Logger) {
	l.Info("sizes", zap.Int("n", len(c.Request.URL.Path)), zap.String("route", c.FullPath()))
	log.Print(fmt.Sprint(len(c.Request.URL.RawQuery)))
}

func BenignHeader(r *http.Request) { log.Print(r.Header.Get("X-Request-ID")) }

func Router() *gin.Engine {
	e := gin.New()
	e.Use(gin.LoggerWithFormatter(func(p gin.LogFormatterParams) string {
		return fmt.Sprintf("%s %s", logredact.Path(p.Request.URL.Path, p.Request.URL.RawQuery), logredact.ErrorText(p.ErrorMessage))
	}))
	return e
}

// ---- round-4 clean twins: the same shapes with constants / redacted values must NOT be reported ----

func logWrappedC(msg string) { log.Print(msg) }

func CleanWrapperConstant(c *gin.Context) { logWrappedC("request failed") }

func CleanWrapperRedacted(c *gin.Context) {
	logWrappedC(logredact.Path(c.Request.URL.Path, c.Request.URL.RawQuery))
}

type cleanLogger interface{ Info(msg string) }

func CleanInterfaceLoggerConstant(l cleanLogger, c *gin.Context) {
	l.Info("served " + c.Request.Method)
}

func CleanObsLoggingRedacted(c *gin.Context) {
	obslogging.NewLogrusAdapter(obslogging.DefaultConfig()).WithField("p", logredact.SecretPath(c.Request.URL.Path)).Warn("x")
}

func CleanClosureConstant(c *gin.Context) {
	f := func() string { return "static" }
	log.Print(f())
}

func CleanBuilderConstant(c *gin.Context) {
	var sb strings.Builder
	sb.WriteString("static")
	log.Print(sb.String())
}

func CleanBuilderRedacted(c *gin.Context) {
	var sb strings.Builder
	sb.WriteString(logredact.SecretPath(c.Request.URL.Path))
	log.Print(sb.String())
}

func CleanStderrConstant() { os.Stderr.WriteString("boot failed\n") }

func CleanChannelConstant() {
	ch := make(chan string, 1)
	ch <- "x"
	log.Print(<-ch)
}
