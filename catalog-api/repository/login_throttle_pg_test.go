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

// F8 (round 2, F19 round 3): the evaluated-before-block bound AccountFree + MaxInflight - 1 under concurrent Begin/Fail loops on real
// PostgreSQL. With Fail deleting the token and charging in two steps, a new attempt can take the freed slot in between.
func TestLoginThrottlePostgres_EvaluatedBeforeBlockBound(t *testing.T) {
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
			th := NewLoginThrottle(db)
			now := time.Unix(1_800_000_000, 0)
			th.SetClock(func() time.Time { return now })
			ctx := context.Background()
			pol := th.pol
			bound := pol.AccountFree + pol.MaxInflight - 1
			worst := 0
			var beginErrs, failErrs atomic.Int32
			for round := 0; round < 20; round++ {
				subj := fmt.Sprintf("u:%d", 500+round)
				evaluated := 0
				for i := 0; i < pol.AccountFree-4; i++ { // serial charges, each from its own address
					d, err := th.Begin(ctx, subj, fmt.Sprintf("10.5.%d.%d", round, i))
					require.NoError(t, err)
					require.True(t, d.Allowed)
					require.NoError(t, th.Fail(ctx, subj, d.Attempt))
					evaluated++
				}
				var wg sync.WaitGroup
				var n atomic.Int32
				start := make(chan struct{})
				for g := 0; g < 16; g++ {
					wg.Add(1)
					go func(g int) {
						defer wg.Done()
						<-start
						for i := 0; i < 6; i++ {
							d, err := th.Begin(ctx, subj, fmt.Sprintf("10.4.%d.%d.%d", round, g, i))
							if err != nil {
								beginErrs.Add(1)
								continue
							}
							if !d.Allowed {
								continue
							}
							n.Add(1)
							if err := th.Fail(ctx, subj, d.Attempt); err != nil {
								failErrs.Add(1)
							}
						}
					}(g)
				}
				close(start)
				wg.Wait()
				evaluated += int(n.Load())
				if evaluated > worst {
					worst = evaluated
				}
				require.LessOrEqual(t, evaluated, bound, "round %d", round)
			}
			t.Logf("image %s: worst evaluated before the block over 20 rounds: %d (bound %d); Begin errors %d, Fail errors %d", img, worst, bound, beginErrs.Load(), failErrs.Load())
			require.Equal(t, int32(0), beginErrs.Load(), "Begin errors would hide evaluated attempts")
			require.Equal(t, int32(0), failErrs.Load(), "Fail errors would leave attempts uncharged")
		})
	}
}

// F14 on real PostgreSQL: concurrent Begin/Fail loops AFTER the account is blocked and the block expired: every evaluated attempt
// must be charged (failures grows by exactly the number evaluated). F15 on real PostgreSQL: the cap holds for tokens inside
// PendingTTL and they expire after it.
func TestLoginThrottlePostgres_ChargeEveryEvaluatedAndCapWindow(t *testing.T) {
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
			th := NewLoginThrottle(db)
			now := time.Unix(1_800_000_000, 0)
			th.SetClock(func() time.Time { return now })
			ctx := context.Background()
			pol := th.pol

			// F14: block the account serially, let the block expire, then run waves of MaxInflight parallel evaluated attempts.
			for i := 0; i < pol.AccountFree; i++ {
				d, err := th.Begin(ctx, "u:r4", fmt.Sprintf("10.1.0.%d", i))
				require.NoError(t, err)
				require.True(t, d.Allowed)
				require.NoError(t, th.Fail(ctx, "u:r4", d.Attempt))
			}
			evaluated := 0
			for window := 0; window < 4; window++ {
				var bu int64
				require.NoError(t, raw.QueryRow(`SELECT blocked_until_ms FROM login_throttle WHERE subject='u:r4' AND scope='account'`).Scan(&bu))
				var before int
				require.NoError(t, raw.QueryRow(`SELECT failures FROM login_throttle WHERE subject='u:r4' AND scope='account'`).Scan(&before))
				now = time.UnixMilli(bu + 1)
				var open []Decision
				for i := 0; i < pol.MaxInflight; i++ {
					d, err := th.Begin(ctx, "u:r4", fmt.Sprintf("10.1.%d.%d", 10+window, i))
					require.NoError(t, err)
					if d.Allowed {
						open = append(open, d)
					}
				}
				var wg sync.WaitGroup
				for _, d := range open {
					wg.Add(1)
					go func(d Decision) {
						defer wg.Done()
						if err := th.Fail(ctx, "u:r4", d.Attempt); err != nil {
							t.Errorf("Fail: %v", err)
						}
					}(d)
				}
				wg.Wait()
				var after int
				require.NoError(t, raw.QueryRow(`SELECT failures FROM login_throttle WHERE subject='u:r4' AND scope='account'`).Scan(&after))
				t.Logf("image %s window %d: evaluated %d, charged %d", img, window, len(open), after-before)
				require.Equal(t, len(open), after-before, "every evaluated attempt must be charged")
				evaluated += len(open)
			}
			require.Greater(t, evaluated, 0)

			// F15: cap window on PostgreSQL.
			for i := 0; i < pol.MaxInflight; i++ {
				d, err := th.Begin(ctx, "u:r4w", fmt.Sprintf("10.2.0.%d", i))
				require.NoError(t, err)
				require.True(t, d.Allowed)
			}
			start := now
			for _, step := range []time.Duration{time.Second, pol.PendingTTL / 2, pol.PendingTTL - time.Second} {
				now = start.Add(step)
				d, err := th.Begin(ctx, "u:r4w", "10.2.1.1")
				require.NoError(t, err)
				require.False(t, d.Allowed, "tokens inside PendingTTL still hold the cap (+%v)", step)
			}
			now = start.Add(pol.PendingTTL + time.Second)
			d, err := th.Begin(ctx, "u:r4w", "10.2.2.1")
			require.NoError(t, err)
			require.True(t, d.Allowed, "tokens older than PendingTTL no longer count")
		})
	}
}
