//go:build !windows

package services

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"catalogizer/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// UNIT-CLASS (fake `convert` on PATH); see conversion_worker_test.go.

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
