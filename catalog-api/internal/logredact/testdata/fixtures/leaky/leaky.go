// Package leaky holds request-derived data flowing into logging / metric / error sinks. EVERY function named NeedleNN must be
// reported by the typed scanner (control needles). It is type-checked only by the scanner; it is never executed.
package leaky

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

var vec = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "n"}, []string{"path"})

func rawNormalize(c *gin.Context) string { return c.Request.URL.Path }

func Needle01DirectPrintf(c *gin.Context) { log.Printf("x %s", c.Request.URL.Path) }

func Needle02AssignThenLog(c *gin.Context) {
	p := c.Request.URL.Path
	q := p
	log.Println(q)
}

func Needle03RequestURI(httpReq *http.Request) { log.Print(httpReq.RequestURI) }

func Needle04RawQuery(c *gin.Context) { log.Print(c.Request.URL.RawQuery) }

func Needle05WholeRequestV(c *gin.Context) { log.Printf("%v", c.Request) }

func Needle06WholeURL(c *gin.Context) { log.Printf("%v", c.Request.URL) }

func Needle07ParamToken(c *gin.Context) { log.Print(c.Param("token")) }

func Needle08EscapedPath(r *http.Request) { log.Print(r.URL.EscapedPath()) }

func Needle09ReqVarName(req *http.Request) { log.Printf("%s", req.URL.Path) }

func Needle10Referer(r *http.Request) { log.Print(r.Referer()) }

func Needle11MuxVars(r *http.Request) { log.Print(mux.Vars(r)["token"]) }

func Needle12NormalizePathReturn(c *gin.Context) {
	path := rawNormalize(c)
	vec.WithLabelValues(path).Inc()
}

func Needle13ZapField(l *zap.Logger, c *gin.Context) {
	l.Info("req", zap.String("path", c.Request.URL.Path))
}

func Needle14MetricLabel(c *gin.Context) { vec.WithLabelValues(c.Request.URL.Path).Inc() }

func Needle15Concat(c *gin.Context) { log.Print("GET " + c.Request.URL.Path + " failed") }

func Needle16Errorf(c *gin.Context) error { return fmt.Errorf("bad %s", c.Request.URL.Path) }

func Needle17HeaderGet(r *http.Request) { log.Print(r.Header.Get("Authorization")) }

func Needle18GinDefault() *gin.Engine { return gin.Default() }

func Needle19GinRecovery() gin.HandlerFunc { return gin.Recovery() }

func Needle20WholeContext(c *gin.Context) { fmt.Printf("%v", c) }

func Needle21QueryMethodThenGet(r *http.Request) {
	tok := r.URL.Query().Get("token")
	log.Println(tok)
}

func Needle22Range(r *http.Request) {
	for _, seg := range []string{r.URL.Path} {
		log.Println(seg)
	}
}

func Needle23LogFormatterRawPath() gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(p gin.LogFormatterParams) string {
		return fmt.Sprintf("%s %s", p.Method, p.Request.URL.RawQuery)
	})
}

func Needle24LogFormatterErrorMessage() gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(p gin.LogFormatterParams) string { return "x " + p.ErrorMessage })
}

func Needle25HeaderNameOnlyInHeaderList(c *gin.Context) {
	log.Println(c.GetHeader("WWW-Authenticate"))
}
