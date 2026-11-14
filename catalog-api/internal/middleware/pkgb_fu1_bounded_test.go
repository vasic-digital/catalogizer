package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// PKGB-FU1: ReplaceKnown must not run over the full uncapped text (O(n*L): 0.87 s per 4 MiB with 64 x 4 KiB secrets).
func TestRequestErrorText_BoundedCostOnHugeText(t *testing.T) {
	gin.SetMode(gin.TestMode)
	hdr := http.Header{}
	for i := 0; i < 64; i++ {
		hdr.Add("Cookie", fmt.Sprintf("s%02d=%s", i, strings.Repeat(string(rune('a'+i%26)), 4000)+fmt.Sprint(i)))
	}
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header = hdr
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = req
	huge := fmt.Errorf("%s", strings.Repeat("filler text without secrets ", (16<<20)/28))
	// Deterministic oracle (no wall clock; time is only logged). Measured: the fixed path allocates ~2.03x the input
	// (fmt.Sprint copy + stringified error), the ReplaceKnown-on-uncapped-text path ~4.0x; the bound is 3x.
	var m0, m1 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m0)
	t0 := time.Now()
	out := requestErrorText(c, huge)
	d := time.Since(t0)
	runtime.ReadMemStats(&m1)
	if len(out) > 3000 {
		t.Fatalf("output not capped: %d", len(out))
	}
	t.Logf("elapsed=%v alloc=%d", d, m1.TotalAlloc-m0.TotalAlloc)
	if got := m1.TotalAlloc - m0.TotalAlloc; got > uint64(3*len(huge.Error())) {
		t.Fatalf("requestErrorText over %d bytes allocated %d bytes (want <= 3x input)", len(huge.Error()), got)
	}
}
