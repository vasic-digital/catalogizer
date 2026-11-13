package services

import (
	"fmt"
	"testing"
	"time"

	"catalogizer/models"
	"catalogizer/repository"

	"github.com/stretchr/testify/require"
)

// CAT-LOGSHARE-TOK-1 reproduction (real service + real sqlite): an attacker who knows only the creation instant to within
// the request latency (the response Date header, created_at, a log line) enumerates log_share_<unix>_<nanos> candidates and
// finds the token. Before the fix the token falls inside a window of a few thousand guesses; after it, no guess hits.
func TestCATLOGSHARETOK1_TokenNotPredictableFromCreationTime(t *testing.T) {
	db := setupTestDB(t)
	svc := NewLogManagementService(repository.NewLogManagementRepository(db))
	col, err := svc.CollectLogs(1, &models.LogCollectionRequest{Name: "c", Components: []string{"api"}, LogLevel: "info"})
	require.NoError(t, err)

	before := time.Now()
	share, err := svc.CreateLogShare(1, &models.LogShareRequest{CollectionID: col.ID, ShareType: "link", Permissions: []string{"read"}})
	require.NoError(t, err)
	after := time.Now()

	// the attacker's knowledge: the creation instant within [before, after] (the call latency, here measured exactly);
	// the guess space is every nanosecond of that window, for both the second the window starts in and the one it ends in.
	guesses := 0
	found := false
	for sec := before.Unix(); sec <= after.Unix() && !found; sec++ {
		lo, hi := 0, 1_000_000_000
		if sec == before.Unix() {
			lo = before.Nanosecond()
		}
		if sec == after.Unix() {
			hi = after.Nanosecond() + 1
		}
		for ns := lo; ns < hi; ns++ {
			guesses++
			if fmt.Sprintf("log_share_%d_%d", sec, ns) == share.ShareToken {
				found = true
				break
			}
		}
	}
	t.Logf("window=%v guesses=%d found=%v token-prefix=%.16s", after.Sub(before), guesses, found, share.ShareToken)
	require.False(t, found, "share token was reconstructed from the creation time after %d guesses (window %v)", guesses, after.Sub(before))
}
