package services

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"catalogizer/models"
	"catalogizer/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// UNIT-CLASS (§11.4.289(C)): these tests exercise the worker's own lifecycle logic (atomic claim,
// bounded concurrency, shutdown requeue) with a fake `convert` executable on PATH that just
// sleeps. They prove the scheduler, not that any conversion works; the live end-to-end proof
// with the real ImageMagick is handlers/conversion_realstack_test.go.

func newWorkerSvc(t *testing.T) (*ConversionService, *repository.ConversionRepository, string) {
	t.Helper()
	db := setupTestDB(t)
	repo := repository.NewConversionRepository(db)
	userRepo := repository.NewUserRepository(db)
	svc := NewConversionService(repo, userRepo, NewAuthService(userRepo, "k"))
	dir := t.TempDir()
	t.Setenv("CATALOGIZER_CONVERSION_ROOTS", dir)
	return svc, repo, dir
}

func addImageJob(t *testing.T, repo *repository.ConversionRepository, dir, name string) int {
	t.Helper()
	src := filepath.Join(dir, name+".png")
	require.NoError(t, os.WriteFile(src, []byte("x"), 0o600))
	id, err := repo.CreateJob(&models.ConversionJob{
		UserID: 1, SourcePath: src, TargetPath: filepath.Join(dir, name+".jpg"), SourceFormat: "png",
		TargetFormat: "jpg", ConversionType: "image", Quality: "high",
		Status: models.ConversionStatusPending, CreatedAt: time.Now(),
	})
	require.NoError(t, err)
	return id
}

func TestClaimJob_ExactlyOneWinner(t *testing.T) {
	_, repo, dir := newWorkerSvc(t)
	id := addImageJob(t, repo, dir, "c")
	var wins int32
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, err := repo.ClaimJob(id, time.Now())
			assert.NoError(t, err)
			if ok {
				atomic.AddInt32(&wins, 1)
			}
		}()
	}
	wg.Wait()
	assert.Equal(t, int32(1), wins)
}

func fakeConvertOnPath(t *testing.T, script string) {
	t.Helper()
	bin := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(bin, "convert"), []byte("#!/bin/sh\n"+script+"\n"), 0o755))
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestRunWorker_BoundedConcurrency(t *testing.T) {
	svc, repo, dir := newWorkerSvc(t)
	mark := filepath.Join(dir, "marks")
	require.NoError(t, os.Mkdir(mark, 0o700))
	// each run records itself while alive; the max number alive at once is observable
	fakeConvertOnPath(t, `d="`+mark+`"; f="$d/$$"; : > "$f"; sleep 0.6; rm -f "$f"; for last; do :; done; : > "$last"`)
	for i := 0; i < 8; i++ {
		addImageJob(t, repo, dir, "j"+string(rune('a'+i)))
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { svc.RunWorker(ctx, 20*time.Millisecond); close(done) }()

	maxAlive := 0
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		ents, _ := os.ReadDir(mark)
		if len(ents) > maxAlive {
			maxAlive = len(ents)
		}
		n, err := repo.GetJobsByStatus(models.ConversionStatusCompleted, 100, 0)
		require.NoError(t, err)
		f, _ := repo.GetJobsByStatus(models.ConversionStatusFailed, 100, 0)
		if len(n)+len(f) == 8 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	cancel()
	<-done
	done2, _ := repo.GetJobsByStatus(models.ConversionStatusCompleted, 100, 0)
	failed, _ := repo.GetJobsByStatus(models.ConversionStatusFailed, 100, 0)
	assert.Len(t, done2, 8, "all queued jobs reach a terminal state (failed: %d)", len(failed))
	assert.LessOrEqual(t, maxAlive, 3, "never more than the semaphore capacity running at once")
	assert.GreaterOrEqual(t, maxAlive, 2, "jobs do run in parallel")
}

func TestRunWorker_ShutdownKillsToolAndRequeues(t *testing.T) {
	svc, repo, dir := newWorkerSvc(t)
	fakeConvertOnPath(t, `exec sleep 60`)
	id := addImageJob(t, repo, dir, "slow")
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { svc.RunWorker(ctx, 20*time.Millisecond); close(done) }()

	require.Eventually(t, func() bool {
		j, err := repo.GetJob(id)
		return err == nil && j.Status == models.ConversionStatusRunning
	}, 10*time.Second, 20*time.Millisecond)

	start := time.Now()
	cancel()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("worker did not stop after context cancellation")
	}
	assert.Less(t, time.Since(start), 8*time.Second, "tool was killed, not waited out")
	j, err := repo.GetJob(id)
	require.NoError(t, err)
	assert.Equal(t, models.ConversionStatusPending, j.Status, "interrupted job returns to the queue")
	assert.Nil(t, j.ErrorMessage)
}

func TestRunWorker_RequeuesJobsLeftRunningByDeadProcess(t *testing.T) {
	svc, repo, dir := newWorkerSvc(t)
	fakeConvertOnPath(t, `for last; do :; done; : > "$last"`)
	id := addImageJob(t, repo, dir, "orph")
	ok, err := repo.ClaimJob(id, time.Now()) // simulates a crashed process mid-job
	require.NoError(t, err)
	require.True(t, ok)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { svc.RunWorker(ctx, 20*time.Millisecond); close(done) }()
	require.Eventually(t, func() bool {
		j, err := repo.GetJob(id)
		return err == nil && j.Status == models.ConversionStatusCompleted
	}, 15*time.Second, 20*time.Millisecond)
	cancel()
	<-done
}

func pidAlive(pid int) bool { return syscall.Kill(pid, 0) == nil }

// R1 (review): owner cancels a running job. The tool must be killed (whole process group), the
// status must stay `cancelled` (not overwritten by failed/completed), and worker shutdown must
// not resurrect it to `pending`.
func TestRunWorker_CancelRunningJobKillsToolAndStaysCancelled(t *testing.T) {
	svc, repo, dir := newWorkerSvc(t)
	pidf := filepath.Join(dir, "pids")
	// the tool forks a grandchild sleeper, like soffice/ebook-convert helpers do
	fakeConvertOnPath(t, `sleep 60 & echo "$$ $!" > "`+pidf+`"; wait`)
	id := addImageJob(t, repo, dir, "cx")
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { svc.RunWorker(ctx, 20*time.Millisecond); close(done) }()
	require.Eventually(t, func() bool {
		b, err := os.ReadFile(pidf)
		return err == nil && len(strings.Fields(string(b))) == 2
	}, 10*time.Second, 20*time.Millisecond)
	var shPID, childPID int
	b, _ := os.ReadFile(pidf)
	fmt.Sscanf(string(b), "%d %d", &shPID, &childPID)
	require.True(t, pidAlive(childPID))

	require.NoError(t, svc.CancelJob(id, 1))

	require.Eventually(t, func() bool { return !pidAlive(shPID) && !pidAlive(childPID) },
		10*time.Second, 20*time.Millisecond, "cancel must kill the tool and its whole process group")
	time.Sleep(300 * time.Millisecond) // let the worker's finishing write happen, if any
	j, err := repo.GetJob(id)
	require.NoError(t, err)
	assert.Equal(t, models.ConversionStatusCancelled, j.Status, "cancel must not be overwritten")

	cancel()
	<-done
	j, err = repo.GetJob(id)
	require.NoError(t, err)
	assert.Equal(t, models.ConversionStatusCancelled, j.Status, "shutdown must not resurrect a cancelled job")
}

func TestRunWorker_JobTimeoutFailsJobAndFreesSlot(t *testing.T) {
	svc, repo, dir := newWorkerSvc(t)
	svc.jobTimeout = 500 * time.Millisecond
	fakeConvertOnPath(t, `exec sleep 60`)
	id := addImageJob(t, repo, dir, "to")
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { svc.RunWorker(ctx, 20*time.Millisecond); close(done) }()
	require.Eventually(t, func() bool {
		j, err := repo.GetJob(id)
		return err == nil && j.Status == models.ConversionStatusFailed
	}, 10*time.Second, 20*time.Millisecond)
	j, _ := repo.GetJob(id)
	require.NotNil(t, j.ErrorMessage)
	assert.Contains(t, *j.ErrorMessage, "timed out")
	cancel()
	<-done
}

func TestRunWorker_SecondStartIsRefused(t *testing.T) {
	svc, _, _ := newWorkerSvc(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { svc.RunWorker(ctx, 20*time.Millisecond); close(done) }()
	require.Eventually(t, func() bool { return svc.workerRunning.Load() }, 5*time.Second, 5*time.Millisecond)
	second := make(chan struct{})
	go func() { svc.RunWorker(context.Background(), 20*time.Millisecond); close(second) }()
	select {
	case <-second:
	case <-time.After(3 * time.Second):
		t.Fatal("a second RunWorker must return immediately")
	}
	cancel()
	<-done
}

func TestGetJobQueue_SkipsFutureScheduledJobsInSQL(t *testing.T) {
	svc, repo, dir := newWorkerSvc(t)
	future := time.Now().Add(time.Hour)
	for i := 0; i < 101; i++ {
		src := filepath.Join(dir, "f.png")
		_, err := repo.CreateJob(&models.ConversionJob{UserID: 1, SourcePath: src, TargetPath: src + ".jpg",
			SourceFormat: "png", TargetFormat: "jpg", ConversionType: "image", Priority: 9,
			Status: models.ConversionStatusPending, CreatedAt: time.Now(), ScheduledFor: &future})
		require.NoError(t, err)
	}
	due := addImageJob(t, repo, dir, "due")
	q, err := svc.GetJobQueue()
	require.NoError(t, err)
	require.Len(t, q, 1, "101 future jobs of higher priority must not hide the due one")
	assert.Equal(t, due, q[0].ID)
}
