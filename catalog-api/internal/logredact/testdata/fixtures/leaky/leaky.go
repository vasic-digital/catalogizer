// Package leaky holds request-derived data flowing into logging / metric / error sinks. EVERY function named NeedleNN must be
// reported by the typed scanner (control needles). It is type-checked only by the scanner; it is never executed.
package leaky

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	contlogging "digital.vasic.containers/pkg/logging"
	obslogging "digital.vasic.observability/pkg/logging"

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

// ---- round-4 needles (F2): module loggers, wrappers, interface loggers, closures, method values, builders, std streams ----

func Needle26ObsLoggingWithField(c *gin.Context) {
	obslogging.NewLogrusAdapter(obslogging.DefaultConfig()).WithField("p", c.Request.URL.Path).Warn("x")
}

func Needle27ContainersLogging(c *gin.Context, l contlogging.Logger) { l.Info(c.Request.URL.Path) }

// a module-defined wrapper around a logger: the CALLER passing request data is the leak
func logWrapped(msg string) { log.Print(msg) }

func logWrapped2(prefix string, v string) { logWrapped(prefix + v) }

func Needle28ModuleWrapper(c *gin.Context) { logWrapped(c.Request.URL.Path) }

func Needle29TransitiveWrapper(c *gin.Context) { logWrapped2("p=", c.Request.URL.Path) }

type needleLogger interface{ Info(msg string) }

func Needle30ModuleInterfaceLogger(l needleLogger, c *gin.Context) { l.Info(c.Request.URL.Path) }

func Needle31ClosureReturningTainted(c *gin.Context) {
	f := func() string { return c.Request.URL.Path }
	log.Print(f())
}

func Needle32MethodValue(c *gin.Context) {
	f := c.Request.URL.String
	log.Print(f())
}

func Needle33StringsBuilder(c *gin.Context) {
	var sb strings.Builder
	sb.WriteString(c.Request.URL.Path)
	log.Print(sb.String())
}

func Needle34FprintfIntoBuilder(c *gin.Context) {
	var sb strings.Builder
	fmt.Fprintf(&sb, "x %s", c.Request.URL.Path)
	log.Print(sb.String())
}

func Needle35OsStderrWriteString(c *gin.Context) { os.Stderr.WriteString(c.Request.URL.Path) }

func Needle36OsStderrWrite(c *gin.Context) { os.Stderr.Write([]byte(c.Request.URL.Path)) }

func Needle37Channel(c *gin.Context) {
	ch := make(chan string, 1)
	ch <- c.Request.URL.Path
	log.Print(<-ch)
}

func Needle38PrintlnBuiltin(c *gin.Context) { println(c.Request.URL.Path) }

func Needle39ErrorfWrapIntoLog(c *gin.Context) {
	err := fmt.Errorf("fetch %s: %w", c.Request.URL.Path, http.ErrNotSupported)
	log.Print(err)
}
