package repository

// CAT-LOGIN-LOCK-2 round 1 finding F1: the in-flight token cap against a REAL PostgreSQL (no stub), on a byte-order collation
// (postgres:15-alpine) and on glibc en_US.utf8 (postgres:15, Debian). Skipped (never faked) when podman or the image is missing.

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"catalogizer/database"
	"catalogizer/internal/testpg"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func TestLoginThrottlePostgres_InflightCapHolds(t *testing.T) {
	for _, img := range []string{testpg.Image, testpg.ImageGlibc} {
		img := img
		t.Run(img, func(t *testing.T) {
			dsn, _ := testpg.StartImage(t, img)
			raw, err := sql.Open("postgres", dsn)
			require.NoError(t, err)
			raw.SetMaxOpenConns(64)
			defer raw.Close()
			db := database.WrapDB(raw, database.DialectPostgres)
			require.NoError(t, db.RunMigrations(context.Background()))
			var coll string
			require.NoError(t, raw.QueryRow(`SELECT datcollate FROM pg_database WHERE datname = current_database()`).Scan(&coll))
			t.Logf("image %s: datcollate=%s", img, coll)

			th := NewLoginThrottle(db)
			now := time.Unix(1_800_000_000, 0)
			th.SetClock(func() time.Time { return now })
			ctx := context.Background()

			serial := 0
			for i := 0; i < 10; i++ {
				d, err := th.Begin(ctx, "u:serial", fmt.Sprintf("10.9.0.%d", i))
				require.NoError(t, err)
				if d.Allowed {
					serial++
				}
			}
			t.Logf("serial control: %d of 10 allowed (MaxInflight 4)", serial)
			require.Equal(t, 4, serial, "serial requests must also respect the cap")

			maxPar := 0
			for round := 0; round < 12; round++ {
				subj := fmt.Sprintf("u:%d", 100+round)
				var wg sync.WaitGroup
				var allowed, errs atomic.Int32
				start := make(chan struct{})
				for i := 0; i < 40; i++ {
					wg.Add(1)
					go func(i int) {
						defer wg.Done()
						<-start
						d, err := th.Begin(ctx, subj, fmt.Sprintf("10.8.%d.%d", round, i))
						if err != nil {
							errs.Add(1)
						} else if d.Allowed {
							allowed.Add(1)
						}
					}(i)
				}
				close(start)
				wg.Wait()
				n := int(allowed.Load())
				if n > maxPar {
					maxPar = n
				}
				t.Logf("round %d: %d of 40 concurrent Begins allowed, %d errors (MaxInflight 4)", round, n, errs.Load())
				require.LessOrEqual(t, n, 4, "round %d: in-flight cap exceeded", round)
				require.GreaterOrEqual(t, n, 1, "round %d: no attempt got a slot (errors %d)", round, errs.Load())
			}

			// the TTL purge must actually match pend rows on this collation: expire everything and purge
			now = now.Add(10 * time.Minute)
			th.purge = time.Time{}
			th.maybePurge(ctx, now, th.pol)
			var pend int
			require.NoError(t, raw.QueryRow(`SELECT COUNT(*) FROM login_throttle WHERE substr(scope, 1, 5) = 'pend:'`).Scan(&pend))
			require.Equal(t, 0, pend, "expired in-flight tokens must be purged on collation %s", coll)
			t.Logf("max allowed in any concurrent round: %d; pend rows after purge: %d", maxPar, pend)
		})
	}
}
