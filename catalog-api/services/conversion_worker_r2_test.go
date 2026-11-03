package services

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"catalogizer/internal/logging"
	"catalogizer/models"
	"catalogizer/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

// UNIT-CLASS (fake `convert` on PATH; direct repository writes stand in for "the owner cancelled
// meanwhile"). They pin each leg of the cancel-vs-finish fix so that removing the leg fails a test.

func markCancelled(t *testing.T, svc *ConversionService, id int) {
	t.Helper()
	j, err := svc.conversionRepo.GetJob(id)
	require.NoError(t, err)
	j.Status = models.ConversionStatusCancelled
	now := time.Now()
	j.CompletedAt = &now
	require.NoError(t, svc.conversionRepo.UpdateJob(j)) // unconditional on purpose: no kill, no run lookup
}

func waitRunsEmpty(t *testing.T, svc *ConversionService) {
	t.Helper()
	require.Eventually(t, func() bool {
		svc.runsMu.Lock()
		defer svc.runsMu.Unlock()
		return len(svc.runs) == 0
	}, 20*time.Second, 10*time.Millisecond)
}

// waitToolStarted waits for the marker the fake converter writes as its first action, i.e. for
// executeJob to have passed its re-read and registered the run (status `running` alone is
// visible from the claim on, before that).
func waitToolStarted(t *testing.T, marker string) {
	t.Helper()
	require.Eventually(t, func() bool {
		_, err := os.Stat(marker)
		return err == nil
	}, 10*time.Second, 5*time.Millisecond, "the fake converter never started")
}

// leg 1: the tool SUCCEEDS after the row was cancelled: the success write must not overwrite it.
func TestExecute_SuccessDoesNotOverwriteCancelled(t *testing.T) {
	svc, repo, dir := newWorkerSvc(t)
	started := filepath.Join(dir, "started")
	fakeConvertOnPath(t, `: > "`+started+`"; sleep 1; for last; do :; done; : > "$last"`)
	id := addImageJob(t, repo, dir, "ok")
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { svc.RunWorker(ctx, 20*time.Millisecond); close(done) }()
	waitToolStarted(t, started)
	markCancelled(t, svc, id)
	waitRunsEmpty(t, svc) // the tool finished and the worker did its finishing write
	j, err := repo.GetJob(id)
	require.NoError(t, err)
	assert.Equal(t, models.ConversionStatusCancelled, j.Status, "a finished tool must not overwrite cancelled")
	cancel()
	<-done
}

// leg 2: worker shutdown while the row is cancelled: the requeue must not resurrect it.
func TestShutdownRequeue_DoesNotResurrectCancelled(t *testing.T) {
	svc, repo, dir := newWorkerSvc(t)
	started := filepath.Join(dir, "started")
	fakeConvertOnPath(t, `: > "`+started+`"; exec sleep 60`)
	id := addImageJob(t, repo, dir, "rq")
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { svc.RunWorker(ctx, 20*time.Millisecond); close(done) }()
	waitToolStarted(t, started)
	markCancelled(t, svc, id) // tool still running; no kill
	cancel()                  // shutdown kills the tool; the worker would requeue a running job
	<-done
	j, err := repo.GetJob(id)
	require.NoError(t, err)
	assert.Equal(t, models.ConversionStatusCancelled, j.Status, "shutdown must not requeue a cancelled job")
}

// leg 3: cancelled between the claim and the start: the tool must never run.
func TestExecute_CancelledAfterClaimDoesNotStartTool(t *testing.T) {
	svc, repo, dir := newWorkerSvc(t)
	marker := filepath.Join(dir, "tool-ran")
	fakeConvertOnPath(t, `: > "`+marker+`"`)
	id := addImageJob(t, repo, dir, "cc")
	j, err := repo.GetJob(id)
	require.NoError(t, err)
	require.NoError(t, svc.claim(j))
	markCancelled(t, svc, id)
	svc.executeJob(j)
	_, statErr := os.Stat(marker)
	assert.True(t, os.IsNotExist(statErr), "the converter must not start for a cancelled job")
	got, err := repo.GetJob(id)
	require.NoError(t, err)
	assert.Equal(t, models.ConversionStatusCancelled, got.Status)
}

// N1a: retry writes only from `failed`.
func TestUpdateJobIf_RefusesWhenStatusChanged(t *testing.T) {
	_, repo, dir := newWorkerSvc(t)
	id := addImageJob(t, repo, dir, "uj")
	ok, err := repo.ClaimJob(id, time.Now())
	require.NoError(t, err)
	require.True(t, ok)
	j, err := repo.GetJob(id)
	require.NoError(t, err)
	j.Status = models.ConversionStatusPending
	written, err := repo.UpdateJobIf(j, models.ConversionStatusFailed)
	require.NoError(t, err)
	assert.False(t, written, "a running row must not be rewritten by a retry")
	got, _ := repo.GetJob(id)
	assert.Equal(t, models.ConversionStatusRunning, got.Status)
}

func TestRetryJob_ConcurrentRetriesOneWins(t *testing.T) {
	svc, repo, dir := newWorkerSvc(t)
	id := addImageJob(t, repo, dir, "rr")
	j, _ := repo.GetJob(id)
	j.Status = models.ConversionStatusFailed
	require.NoError(t, repo.UpdateJob(j))
	var wg sync.WaitGroup
	var mu sync.Mutex
	okCount := 0
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := svc.RetryJob(id, 1); err == nil {
				mu.Lock()
				okCount++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	assert.Equal(t, 1, okCount, "exactly one retry may win")
}

// N1b: a stale run cannot unregister a newer run of the same job id.
func TestUnregisterRun_OnlyRemovesItself(t *testing.T) {
	svc, _, _ := newWorkerSvc(t)
	ctxA, cancelA := context.WithCancel(context.Background())
	ctxB, cancelB := context.WithCancel(context.Background())
	defer cancelA()
	defer cancelB()
	a := &jobRun{ctx: ctxA, cancel: cancelA}
	b := &jobRun{ctx: ctxB, cancel: cancelB}
	svc.registerRun(7, a)
	svc.registerRun(7, b)
	svc.unregisterRun(7, a)
	svc.killRun(7)
	assert.Error(t, ctxB.Err(), "killRun must still reach the newer run B")
}

// N2: scheduled_for is compared as an instant, whatever offset the caller used.
func TestGetDueJobs_ComparesInstantsAcrossOffsets(t *testing.T) {
	svc, repo, dir := newWorkerSvc(t)
	mk := func(name string, at time.Time) int {
		src := filepath.Join(dir, name+".png")
		id, err := repo.CreateJob(&models.ConversionJob{UserID: 1, SourcePath: src, TargetPath: src + ".jpg",
			SourceFormat: "png", TargetFormat: "jpg", ConversionType: "image",
			Status: models.ConversionStatusPending, CreatedAt: time.Now(), ScheduledFor: &at})
		require.NoError(t, err)
		return id
	}
	now := time.Now()
	east14 := mk("e14", now.Add(-30*time.Minute).In(time.FixedZone("p14", 14*3600)))
	east5 := mk("e5", now.Add(-3*time.Hour).In(time.FixedZone("p5", 5*3600)))
	utc := mk("utc", now.Add(-30*time.Minute).UTC())
	futureWest := mk("fw", now.Add(30*time.Minute).In(time.FixedZone("m12", -12*3600)))
	q, err := svc.GetJobQueue()
	require.NoError(t, err)
	got := map[int]bool{}
	for _, j := range q {
		got[j.ID] = true
	}
	assert.True(t, got[east14], "due job scheduled with +14:00")
	assert.True(t, got[east5], "due job scheduled with +05:00")
	assert.True(t, got[utc])
	assert.False(t, got[futureWest], "future job scheduled with -12:00 is not due")
}

// N5 + N6: timeout parsing and the process-group guard.
func TestConversionTimeoutFromEnv(t *testing.T) {
	t.Setenv("CATALOGIZER_CONVERSION_TIMEOUT", "45m")
	assert.Equal(t, 45*time.Minute, conversionTimeoutFromEnv())
	for _, bad := range []string{"garbage", "0", "-5m"} {
		t.Setenv("CATALOGIZER_CONVERSION_TIMEOUT", bad)
		assert.Equal(t, DefaultConversionTimeout, conversionTimeoutFromEnv(), bad)
	}
}

func TestGroupKillTarget_RefusesPidsAtOrBelowOne(t *testing.T) {
	for _, pid := range []int{-5, -1, 0, 1} {
		_, ok := groupKillTarget(pid)
		assert.False(t, ok, "pid %d must never become a group-kill target (kill(-1) hits every process)", pid)
	}
	pg, ok := groupKillTarget(4242)
	assert.True(t, ok)
	assert.Equal(t, -4242, pg)
}

// C1 (review r3): CancelJob must write `cancelled` BEFORE it kills the run. If the kill came
// first, the worker's error path could win its conditional write (`failed: signal: killed`) and
// the owner's cancel would answer 409. The fake run records the stored status the first time it
// is cancelled.
func TestCancelJob_WritesCancelledBeforeKillingTheRun(t *testing.T) {
	svc, repo, dir := newWorkerSvc(t)
	id := addImageJob(t, repo, dir, "order")
	j, err := repo.GetJob(id)
	require.NoError(t, err)
	require.NoError(t, svc.claim(j))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var mu sync.Mutex
	seen, calls := "", 0
	svc.registerRun(id, &jobRun{ctx: ctx, cancel: func() {
		mu.Lock()
		defer mu.Unlock()
		calls++
		if calls == 1 {
			cur, gerr := repo.GetJob(id)
			if gerr == nil {
				seen = cur.Status
			}
		}
		cancel()
	}})
	require.NoError(t, svc.CancelJob(id, 1))
	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, 1, calls, "CancelJob must cancel the run")
	assert.Equal(t, models.ConversionStatusCancelled, seen, "the row must already be cancelled when the run is killed")
}

// C3 (review r3): rows written before scheduled_for was normalised to UTC carry their original
// offset in the stored SQLite text; the worker start rewrites them exactly (idempotent).
func TestNormalizeLegacyScheduledFor_RewritesOffsetRowsToUTC(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewConversionRepository(db)
	insert := func(status string, at time.Time) int {
		res, err := db.Exec(`INSERT INTO conversion_jobs (user_id, source_path, target_path, source_format,
			target_format, conversion_type, quality, settings, priority, status, created_at, scheduled_for)
			VALUES (1, 's', 't', 'png', 'jpg', 'image', 'high', '', 0, ?, ?, ?)`, status, time.Now().UTC(), at)
		require.NoError(t, err)
		id, err := res.LastInsertId()
		require.NoError(t, err)
		return int(id)
	}
	now := time.Now()
	dueEast := now.Add(-30 * time.Minute).In(time.FixedZone("p14", 14*3600)) // legacy form, due
	futureWest := now.Add(30 * time.Minute).In(time.FixedZone("m12", -12*3600))
	idDue := insert(models.ConversionStatusPending, dueEast)
	idFut := insert(models.ConversionStatusPending, futureWest)
	idUTC := insert(models.ConversionStatusPending, now.Add(-time.Hour).UTC())

	// RED baseline (the bug): the legacy +14:00 row is due but the SQL text compare misses it
	due, err := repo.GetDueJobs(now, 10)
	require.NoError(t, err)
	ids := func(js []models.ConversionJob) map[int]bool {
		m := map[int]bool{}
		for _, j := range js {
			m[j.ID] = true
		}
		return m
	}
	require.False(t, ids(due)[idDue], "precondition: the legacy +14:00 row is mis-ordered before normalisation")

	n, skipped, err := repo.NormalizeLegacyScheduledFor()
	require.NoError(t, err)
	assert.Equal(t, 2, n, "the two offset rows are rewritten, the UTC row is not")
	assert.Equal(t, 0, skipped)

	due, err = repo.GetDueJobs(now, 10)
	require.NoError(t, err)
	got := ids(due)
	assert.True(t, got[idDue], "after normalisation the due +14:00 row is returned")
	assert.True(t, got[idUTC])
	assert.False(t, got[idFut], "the future -12:00 row stays not due")
	for _, id := range []int{idDue, idFut} {
		j, err := repo.GetJob(id)
		require.NoError(t, err)
		require.NotNil(t, j.ScheduledFor)
		_, off := j.ScheduledFor.Zone()
		assert.Equal(t, 0, off, "job %d is stored in UTC", id)
	}
	// the instant is preserved exactly
	j, err := repo.GetJob(idDue)
	require.NoError(t, err)
	assert.WithinDuration(t, dueEast, *j.ScheduledFor, time.Millisecond)

	n, skipped, err = repo.NormalizeLegacyScheduledFor()
	require.NoError(t, err)
	assert.Equal(t, 0, n+skipped, "idempotent: a second run has nothing to do")
}

// N5 (review r3 nit): an invalid timeout is not only ignored, it is reported.
func TestConversionTimeoutFromEnv_WarnsOnInvalidValue(t *testing.T) {
	core, logs := observer.New(zapcore.WarnLevel)
	prev := logging.Logger
	logging.Logger = zap.New(core)
	t.Cleanup(func() { logging.Logger = prev })

	t.Setenv("CATALOGIZER_CONVERSION_TIMEOUT", "garbage")
	assert.Equal(t, DefaultConversionTimeout, conversionTimeoutFromEnv())
	require.Equal(t, 1, logs.Len(), "an invalid value must log one warning")
	assert.Contains(t, logs.All()[0].Message, "CATALOGIZER_CONVERSION_TIMEOUT")

	logs.TakeAll()
	t.Setenv("CATALOGIZER_CONVERSION_TIMEOUT", "45m")
	assert.Equal(t, 45*time.Minute, conversionTimeoutFromEnv())
	assert.Equal(t, 0, logs.Len(), "a valid value logs nothing")
}

// the worker start runs the normalisation (wiring, not only the repository method)
func TestRunWorker_NormalizesLegacyScheduledForAtStart(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewConversionRepository(db)
	userRepo := repository.NewUserRepository(db)
	svc := NewConversionService(repo, userRepo, NewAuthService(userRepo, "k"))
	at := time.Now().Add(2 * time.Hour).In(time.FixedZone("p5", 5*3600))
	res, err := db.Exec(`INSERT INTO conversion_jobs (user_id, source_path, target_path, source_format,
		target_format, conversion_type, quality, settings, priority, status, created_at, scheduled_for)
		VALUES (1, 's', 't', 'png', 'jpg', 'image', 'high', '', 0, 'pending', ?, ?)`, time.Now().UTC(), at)
	require.NoError(t, err)
	id, _ := res.LastInsertId()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { svc.RunWorker(ctx, 20*time.Millisecond); close(done) }()
	require.Eventually(t, func() bool {
		j, err := repo.GetJob(int(id))
		if err != nil || j.ScheduledFor == nil {
			return false
		}
		_, off := j.ScheduledFor.Zone()
		return off == 0
	}, 10*time.Second, 10*time.Millisecond, "worker start must rewrite the legacy offset row to UTC")
	cancel()
	<-done
}
