package services

import (
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// SECQ-4 / SC7: the per-user smart cap is "count, then insert". With the user one below the cap, N concurrent creates all
// read the same count and all insert, so the user ends above the cap. Real PlaylistService over a real (encrypted) SQLite
// database; the barrier makes every goroutine start together. Repeated rounds because the race is timing dependent.
func TestPlaylistService_SmartCapHoldsUnderConcurrentCreates_SECQ4(t *testing.T) {
	const rounds, workers = 5, 16
	overshoot := 0
	for r := 0; r < rounds; r++ {
		s, db := realPlaylistService(t)
		for i := 0; i < MaxSmartPlaylistsPerUser-1; i++ {
			_, err := s.CreatePlaylist(1, smartReq("pre", false))
			require.NoError(t, err)
		}
		start := make(chan struct{})
		var wg sync.WaitGroup
		var mu sync.Mutex
		ok, limited := 0, 0
		for w := 0; w < workers; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				_, err := s.CreatePlaylist(1, smartReq("race", false))
				mu.Lock()
				defer mu.Unlock()
				switch {
				case err == nil:
					ok++
				case errors.Is(err, ErrSmartLimit):
					limited++
				}
			}()
		}
		close(start)
		wg.Wait()
		var n int
		require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM playlists WHERE user_id = 1 AND is_smart = 1`).Scan(&n))
		if n > MaxSmartPlaylistsPerUser {
			overshoot++
		}
		t.Logf("round %d: succeeded=%d limited=%d stored=%d (cap %d)", r, ok, limited, n, MaxSmartPlaylistsPerUser)
		assert.LessOrEqual(t, n, MaxSmartPlaylistsPerUser, "round %d: the cap was exceeded", r)
	}
	t.Logf("rounds over the cap: %d of %d", overshoot, rounds)
}
