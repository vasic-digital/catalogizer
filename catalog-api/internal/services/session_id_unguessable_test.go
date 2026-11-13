package services

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// CAT-LOGSHARE-TOK-1 class members: session ids that are the only capability for a session must be unguessable.
func TestSessionIDs_AreRandomNotClockDerived(t *testing.T) {
	music := regexp.MustCompile(`^session_[A-Z2-7]{26}$`)
	reader := regexp.MustCompile(`^session_1_b_d_[A-Z2-7]{26}$`)
	rs := NewReaderService(nil, zap.NewNop(), nil, nil, nil)
	seen := map[string]bool{}
	for i := 0; i < 20000; i++ {
		m, r := generateSessionID(), rs.generateSessionID(1, "b", "d")
		if !music.MatchString(m) || !reader.MatchString(r) {
			t.Fatalf("shape: %q %q", m, r)
		}
		if seen[m] || seen[r] {
			t.Fatalf("collision at %d", i)
		}
		seen[m], seen[r] = true, true
	}
	// two ids in the same second share nothing but the fixed prefix (a clock-derived id would share its tail)
	a, b := rs.generateSessionID(1, "b", "d"), rs.generateSessionID(1, "b", "d")
	assert.NotEqual(t, a[len(a)-26:], b[len(b)-26:])
}
