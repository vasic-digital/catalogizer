// Package clean holds legitimate uses the typed scanner must NOT report (false-positive guard).
package clean

import (
	"fmt"
	"log"
	"net/http"
	"strings"

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
