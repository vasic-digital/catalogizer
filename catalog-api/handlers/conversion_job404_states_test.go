package handlers

import (
	"net/http"
	"os"
	"testing"

	"catalogizer/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// CAT-JOB-404-1 round 2 (reviewer probe matrix adopted, §11.4.276(D)): a foreign job in EVERY
// state answers like a missing id on EVERY job-by-id route, so no state check (409 conflict)
// runs before ownership and the API is no oracle for another user's job id.
func TestJob404_ForeignJobEveryStateEqualsMissingOnEveryRoute(t *testing.T) {
	states := []string{models.ConversionStatusPending, models.ConversionStatusRunning,
		models.ConversionStatusCompleted, models.ConversionStatusFailed, models.ConversionStatusCancelled}
	for _, ep := range job404Endpoints {
		for _, st := range states {
			t.Run(ep.name+"/"+st, func(t *testing.T) {
				s := newJob404Stack(t)
				id := s.newJob(t, st, true)
				missing := s.call(ep.method, "/jobs/999999"+ep.suffix, s.otherTok)
				foreign := s.call(ep.method, "/jobs/"+convItoa(id)+ep.suffix, s.otherTok)
				assert.Equal(t, http.StatusNotFound, missing.Code)
				assert.Equal(t, missing.Code, foreign.Code, foreign.Body.String())
				assert.Equal(t, missing.Body.String(), foreign.Body.String())
				assert.Equal(t, missing.Header(), foreign.Header())
				got, err := s.repo.GetJob(id)
				require.NoError(t, err, "a non-owner must not delete the job")
				assert.Equal(t, st, got.Status, "a non-owner must not change the job")
			})
		}
	}
}

// the service error a non-owner sees for a foreign id has the exact message of a missing id on
// every operation and every state (no wrapped detail a log line or handler could surface).
func TestJob404_ServiceErrorTextIdenticalEveryOperationAndState(t *testing.T) {
	states := []string{models.ConversionStatusPending, models.ConversionStatusRunning,
		models.ConversionStatusCompleted, models.ConversionStatusFailed, models.ConversionStatusCancelled}
	for _, st := range states {
		t.Run(st, func(t *testing.T) {
			s := newJob404Stack(t)
			id := s.newJob(t, st, false)
			_, ferr := s.svc.GetJob(id, s.otherID)
			_, merr := s.svc.GetJob(999999, s.otherID)
			require.Error(t, merr)
			require.Error(t, ferr)
			assert.Equal(t, merr.Error(), ferr.Error())
			for name, op := range map[string]func(int, int) error{
				"CancelJob": s.svc.CancelJob, "DeleteJob": s.svc.DeleteJob, "RetryJob": s.svc.RetryJob,
			} {
				fe, me := op(id, s.otherID), op(999999, s.otherID)
				require.Error(t, me, name)
				require.Error(t, fe, name)
				assert.Equal(t, me.Error(), fe.Error(), name)
			}
		})
	}
}

// JOB-404-F1 (reviewer mutant M7): a foreign DELETE of a COMPLETED job must not touch the output
// file either; the row-existence check alone would let "remove output before the ownership check"
// survive. Control: the owner's DELETE of the same kind of job does remove the file.
func TestJob404_ForeignDeleteOfCompletedJobKeepsOutputFile(t *testing.T) {
	s := newJob404Stack(t)
	id := s.newJob(t, models.ConversionStatusCompleted, true)
	job, err := s.repo.GetJob(id)
	require.NoError(t, err)
	_, err = os.Stat(job.TargetPath)
	require.NoError(t, err, "precondition: the output file exists")

	w := s.call("DELETE", "/jobs/"+convItoa(id), s.otherTok)
	require.Equal(t, http.StatusNotFound, w.Code)
	_, err = os.Stat(job.TargetPath)
	assert.NoError(t, err, "a non-owner's DELETE must not remove the job's output file")
	_, err = s.repo.GetJob(id)
	assert.NoError(t, err, "a non-owner's DELETE must not remove the row")

	ow := s.call("DELETE", "/jobs/"+convItoa(id), s.ownerTok)
	require.Equal(t, http.StatusOK, ow.Code, ow.Body.String())
	_, err = os.Stat(job.TargetPath)
	assert.True(t, os.IsNotExist(err), "control: the owner's DELETE removes the output file")
}
