package database

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"catalogizer/internal/netguard"
)

// PKGA-FU1 (CAT-SECQ1-C): an automatic write through SetApprovedClass never overwrites an operator decision; a deliberate
// supersede goes through ReplaceApprovedClass; a missing row is still an error.
func TestSECQ1C_SetApprovedClassAutoNeverOverwritesOperator(t *testing.T) {
	db, raw := newMigratedDB(t)
	ctx := context.Background()
	id := insertRoot(t, raw, "r", "nas.test")

	require.NoError(t, db.SetApprovedClass(ctx, TableStorageRoots, id, netguard.ClassPublic, ApprovedSourceOperator))
	require.NoError(t, db.SetApprovedClass(ctx, TableStorageRoots, id, netguard.ClassPrivate, ApprovedSourceAuto), "kept, not an error")
	c, src, err := db.GetApprovedClass(ctx, TableStorageRoots, id)
	require.NoError(t, err)
	require.Equal(t, netguard.ClassPublic, c, "operator class survives the auto write")
	require.Equal(t, ApprovedSourceOperator, src)

	require.NoError(t, db.ReplaceApprovedClass(ctx, TableStorageRoots, id, netguard.ClassPrivate, ApprovedSourceAuto))
	c, src, _ = db.GetApprovedClass(ctx, TableStorageRoots, id)
	require.Equal(t, netguard.ClassPrivate, c)
	require.Equal(t, ApprovedSourceAuto, src)

	require.ErrorIs(t, db.SetApprovedClass(ctx, TableStorageRoots, 9999, netguard.ClassPublic, ApprovedSourceAuto), ErrEndpointNotFound)
}
