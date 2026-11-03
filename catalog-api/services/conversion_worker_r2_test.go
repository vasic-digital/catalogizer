package services

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"catalogizer/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

// leg 1: the tool SUCCEEDS after the row was cancelled: the success write must not overwrite it.
func TestExecute_SuccessDoesNotOverwriteCancelled(t *testing.T) {
	svc, repo, dir := newWorkerSvc(t)
	fakeConvertOnPath(t, `sleep 1; for last; do :; done; : > "$last"`)
	id := addImageJob(t, repo, dir, "ok")
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { svc.RunWorker(ctx, 20*time.Millisecond); close(done) }()
	require.Eventually(t, func() bool {
		j, err := repo.GetJob(id)
		return err == nil && j.Status == models.ConversionStatusRunning
	}, 10*time.Second, 10*time.Millisecond)
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
	fakeConvertOnPath(t, `exec sleep 60`)
	id := addImageJob(t, repo, dir, "rq")
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { svc.RunWorker(ctx, 20*time.Millisecond); close(done) }()
	require.Eventually(t, func() bool {
		j, err := repo.GetJob(id)
		return err == nil && j.Status == models.ConversionStatusRunning
	}, 10*time.Second, 10*time.Millisecond)
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
