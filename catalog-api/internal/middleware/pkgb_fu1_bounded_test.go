package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
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
	best := time.Hour
	for i := 0; i < 3; i++ {
		t0 := time.Now()
		out := requestErrorText(c, huge)
		if d := time.Since(t0); d < best {
			best = d
		}
		if len(out) > 3000 {
			t.Fatalf("output not capped: %d", len(out))
		}
	}
	t.Logf("best=%v", best)
	if best > 60*time.Millisecond {
		t.Fatalf("requestErrorText over 16 MiB took %v (want < 60ms)", best)
	}
}
