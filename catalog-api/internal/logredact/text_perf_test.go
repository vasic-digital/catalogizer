package logredact

import (
	"strings"
	"testing"
	"time"
)

// F1 (round 3 review): every rule of the single pass must consume or skip each byte a bounded number of times. The adversarial
// shapes repeat one construct that makes some rule scan ahead and then advance only a constant amount.
var perfShapes = []string{
	"://", "@", `"`, `'`, "=", ":", `\`, `\"`, "%", "\n", "\r\n", "[", "]", "{", "}", " ", "\t",
	"a://b ", "://x@", "x@://", `token="`, `token='`, `token=\"`, `token=\'`, "token=", "token:", "token ", "token\t",
	"bearer ", "basic ", `basic "`, `bearer "`, "digest x", "negotiate ", "password", "Authorization", "Authorization:",
	"Cookie:", "cookie: a=", "share/", "/share/", "/share//", "share/x/", "key:", `"key":`, `"token":[`, "token=[", "token=[[",
	"{token ", "{Key:token Value:", "Key:token ", "token%3D", "%74oken=", "jwt ", "Token ", "password -> ", "=> ", "pass-word=",
	"\\u0022", "://:@", "://a:b@", "x=\\\\", "'\\", "\"\\", "\n ", "Bearer\n ", "Cookie:\n ",
}

func timeText(s string) time.Duration {
	best := time.Duration(1 << 62)
	for i := 0; i < 2; i++ {
		t0 := time.Now()
		_ = ErrorText(s)
		if d := time.Since(t0); d < best {
			best = d
		}
	}
	return best
}

func TestTextLinearOnAdversarialShapes(t *testing.T) {
	const small, big = 16 << 10, 64 << 10 // 4x input
	const budget = 250 * time.Millisecond // hard per-call budget at the pre-cap size
	for _, shape := range perfShapes {
		mk := func(n int) string { return strings.Repeat(shape, n/len(shape)+1)[:n] }
		ds, db := timeText(mk(small)), timeText(mk(big))
		if db > budget {
			t.Errorf("shape %q: %v for %d bytes exceeds the %v budget", shape, db, big, budget)
		}
		// 4x input must cost <~6x time (quadratic would be ~16x). A 20ms floor avoids ratio noise on millisecond runs (the hard budget still bounds them).
		if db > 20*time.Millisecond && float64(db) > 6*float64(ds) {
			t.Errorf("shape %q: 4x input took %.1fx time (%v -> %v)", shape, float64(db)/float64(ds), ds, db)
		}
	}
}

// Control needle: the instrument sees a genuinely quadratic function (an instrument that cannot see proves nothing).
func TestPerfInstrumentSeesQuadratic(t *testing.T) {
	quad := func(s string) {
		c := 0
		for i := 0; i < len(s); i++ {
			for j := i; j < len(s); j++ {
				if s[j] == 'x' {
					c++
				}
			}
		}
		_ = c
	}
	mk := func(n int) string { return strings.Repeat("a", n) }
	// Minimum of several runs per size: a load spike inflates one sample, never the minimum, so the ~16x reference ratio stays
	// discriminating (threshold 8x) on a loaded host.
	best := func(n int) time.Duration {
		min := time.Duration(1<<62 - 1)
		for r := 0; r < 5; r++ {
			t0 := time.Now()
			quad(mk(n))
			if d := time.Since(t0); d < min {
				min = d
			}
		}
		return min
	}
	ds, db := best(4000), best(16000)
	if float64(db) < 8*float64(ds) {
		t.Fatalf("instrument blind: quadratic reference ratio only %.1fx", float64(db)/float64(ds))
	}
}
