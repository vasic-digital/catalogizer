package database

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"catalogizer/internal/netguard"
)

// F1 (review round 1): AutoClassifyApprovedClass read the source, classified (slow DNS), then wrote without a source
// condition, so an operator decision made in between was overwritten. The classifier is replaced by a blocking one.
func TestAutoClassifyApprovedClass_ConcurrentOperatorDecisionSurvives(t *testing.T) {
	db, raw := newMigratedDB(t)
	ctx := context.Background()
	id := insertRoot(t, raw, "r", "slow.test")
	entered := make(chan struct{})
	release := make(chan struct{})
	old := classifyEndpoint
	classifyEndpoint = func(ctx context.Context, host, rawURL string) (netguard.Class, error) {
		close(entered)
		<-release
		return netguard.ClassPrivate, nil // the slow answer the automatic path would store
	}
	t.Cleanup(func() { classifyEndpoint = old })

	type res struct {
		c   netguard.Class
		err error
	}
	done := make(chan res, 1)
	go func() {
		c, err := db.AutoClassifyApprovedClass(ctx, TableStorageRoots, id, "slow.test", "")
		done <- res{c, err}
	}()
	<-entered
	require.NoError(t, db.SetApprovedClass(ctx, TableStorageRoots, id, netguard.ClassPublic, ApprovedSourceOperator))
	close(release)
	r := <-done
	require.NoError(t, r.err)
	require.Equal(t, netguard.ClassPublic, r.c, "the call returns the stored operator class")
	cls, src, err := db.GetApprovedClass(ctx, TableStorageRoots, id)
	require.NoError(t, err)
	require.Equal(t, netguard.ClassPublic, cls)
	require.Equal(t, ApprovedSourceOperator, src)
}

// The row disappears while the classification runs: not found, not a silent success.
func TestAutoClassifyApprovedClass_RowDeletedDuringClassification(t *testing.T) {
	db, raw := newMigratedDB(t)
	ctx := context.Background()
	id := insertRoot(t, raw, "r", "slow.test")
	old := classifyEndpoint
	classifyEndpoint = func(ctx context.Context, host, rawURL string) (netguard.Class, error) {
		_, err := raw.Exec(`DELETE FROM storage_roots WHERE id = ?`, id)
		require.NoError(t, err)
		return netguard.ClassPublic, nil
	}
	t.Cleanup(func() { classifyEndpoint = old })
	_, err := db.AutoClassifyApprovedClass(ctx, TableStorageRoots, id, "slow.test", "")
	require.ErrorIs(t, err, ErrEndpointNotFound)
}
