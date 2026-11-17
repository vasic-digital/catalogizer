package services

import (
	"context"
	"testing"

	"catalogizer/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// N7-FU1 (M8 guard): a non-owner cancel must not reach the registered run. CancelJob refuses through authorizedJob BEFORE
// any state change, so the running tool of somebody else's job stays alive and the stored job stays untouched. The owner's own
// cancel does kill the run (positive control, so the test cannot pass on a killRun that never works).
func TestCancelJob_NonOwnerDoesNotKillRegisteredRun(t *testing.T) {
	svc, repo, dir := newWorkerSvc(t)
	id := addImageJob(t, repo, dir, "m8") // owned by user 1, pending

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	svc.registerRun(id, &jobRun{ctx: ctx, cancel: cancel})

	err := svc.CancelJob(id, 2) // user 2 owns nothing and is no administrator
	require.ErrorIs(t, err, models.ErrConversionJobNotFound)
	assert.NoError(t, ctx.Err(), "a refused (non-owner) cancel must not kill the registered run")
	job, gerr := repo.GetJob(id)
	require.NoError(t, gerr)
	assert.Equal(t, models.ConversionStatusPending, job.Status, "a refused cancel must not change the stored job")

	// positive control: the owner's cancel marks the job cancelled and kills the run
	require.NoError(t, svc.CancelJob(id, 1))
	assert.Error(t, ctx.Err(), "the owner's cancel must kill the registered run")
	job, gerr = repo.GetJob(id)
	require.NoError(t, gerr)
	assert.Equal(t, models.ConversionStatusCancelled, job.Status)
}
