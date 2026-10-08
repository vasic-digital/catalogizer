package services

import (
	"context"
	"errors"
	"net"
	"syscall"
	"testing"

	"catalogizer/filesystem"
	"catalogizer/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// PA-03 regression at the JOB level: "scan of an empty or unreachable root ends failed with a reason, never completed with 0 files".
// These drive UniversalScanner.processScanJob (the only path from a storage root to a client) through the real constructors, the
// real FTP/NFS/WebDAV scanners and the real status machine; only the client behind the factory is the in-memory unit-test fake.

type memFactory struct {
	client    filesystem.FileSystemClient
	createErr error
}

func (f *memFactory) CreateClient(*filesystem.StorageConfig) (filesystem.FileSystemClient, error) {
	return f.client, f.createErr
}
func (f *memFactory) SupportedProtocols() []string { return []string{"ftp", "nfs", "webdav"} }

// runJob runs one job through processScanJob and returns the final status.
func runJob(t *testing.T, protocol string, factory filesystem.ClientFactory, ctx context.Context) *ScanStatus {
	t.Helper()
	return runJobOn(t, NewUniversalScanner(nil, zap.NewNop(), nil, factory), protocol, ctx)
}

func runJobOn(t *testing.T, us *UniversalScanner, protocol string, ctx context.Context) *ScanStatus {
	t.Helper()
	job := ScanJob{ID: "job-" + protocol, Path: "/", MaxDepth: 10, ScanType: "full", Context: ctx,
		StorageRoot: &models.StorageRoot{Name: "root-" + protocol, Protocol: protocol}}
	us.processScanJob(job, 0)
	st, ok := us.GetActiveScanStatus(job.ID)
	require.True(t, ok)
	snap := st.GetSnapshot()
	us.Stop() // releases the 60 s result-retention goroutine (goleak)
	return &snap
}

func TestScanJob_PopulatedRootCompletesWithFiles(t *testing.T) {
	for _, proto := range []string{"ftp", "nfs", "webdav"} {
		st := runJob(t, proto, &memFactory{client: seededTree()}, context.Background())
		assert.Equal(t, "completed", st.Status, proto)
		assert.Equal(t, int64(5), st.FilesFound, proto)
		assert.Empty(t, st.Reason, proto)
	}
}

func TestScanJob_EmptyRootEndsFailedWithReason(t *testing.T) {
	for _, proto := range []string{"ftp", "nfs", "webdav"} {
		m := newMemFS()
		m.addDir("/")
		st := runJob(t, proto, &memFactory{client: m}, context.Background())
		assert.Equal(t, "failed", st.Status, "%s: an empty root must not end completed with 0 files", proto)
		assert.Contains(t, st.Reason, string(FailEmptyRoot), proto)
		assert.Equal(t, int64(0), st.FilesFound)
	}
}

func TestScanJob_UnreachableRootEndsFailedWithReason(t *testing.T) {
	for _, proto := range []string{"ftp", "nfs", "webdav"} {
		m := seededTree()
		m.listErr["/"] = &net.OpError{Op: "dial", Err: syscall.ECONNREFUSED}
		st := runJob(t, proto, &memFactory{client: m}, context.Background())
		assert.Equal(t, "failed", st.Status, proto)
		assert.Contains(t, st.Reason, string(FailRootUnreadable), proto)
		assert.GreaterOrEqual(t, st.ErrorCount, int64(1), proto)
	}
}

func TestScanJob_ConnectFailureEndsFailedWithReason(t *testing.T) {
	m := seededTree()
	m.connect = errors.New("dial tcp 10.0.0.9:21: connect: no route to host")
	st := runJob(t, "ftp", &memFactory{client: m}, context.Background())
	assert.Equal(t, "failed", st.Status)
	assert.Contains(t, st.Reason, "no route to host")
}

func TestScanJob_ClientCreationFailureEndsFailedWithReason(t *testing.T) {
	st := runJob(t, "nfs", &memFactory{createErr: errors.New("invalid nfs settings: unknown settings: export_path (use path)")}, context.Background())
	assert.Equal(t, "failed", st.Status)
	assert.Contains(t, st.Reason, "export_path")
}

func TestScanJob_CancelledContextEndsCancelledNotCompleted(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	m := seededTree()
	m.listHook = func(dir string) {
		if dir == "/music" {
			cancel()
		}
	}
	st := runJob(t, "ftp", &memFactory{client: m}, ctx)
	assert.Equal(t, "cancelled", st.Status)
	assert.Contains(t, st.Reason, string(FailCancelled))
}

func TestScanJob_NoScannerForUnknownProtocolFails(t *testing.T) {
	st := runJob(t, "gopher", &memFactory{client: seededTree()}, context.Background())
	assert.Equal(t, "failed", st.Status)
	assert.Contains(t, st.Reason, "no scanner for protocol gopher")
}

func TestScanJob_RegisteredPluggableProtocolIsScannedGenerically(t *testing.T) {
	name := "testproto-scan"
	t.Cleanup(func() { filesystem.UnregisterProtocol(name) })

	// the scanner exists FIRST, the protocol is registered AFTER it: resolved on demand
	us0 := NewUniversalScanner(nil, zap.NewNop(), nil, &memFactory{client: seededTree()})
	require.NoError(t, filesystem.RegisterProtocol(filesystem.ProtocolSpec{
		Name: name, Keys: map[string]filesystem.KeyType{filesystem.KeyHost: filesystem.KeyString},
		New: func(map[string]interface{}) (filesystem.FileSystemClient, error) { return nil, errors.New("unused") },
	}))
	st := runJobOn(t, us0, name, context.Background())
	assert.Equal(t, "completed", st.Status, "an sftp/ftps/nfs3-class protocol needs no scanner code of its own")
	assert.Equal(t, int64(5), st.FilesFound)

	// registered BEFORE the scanner is built: registered at construction
	us := NewUniversalScanner(nil, zap.NewNop(), nil, &memFactory{client: seededTree()})
	us.protocolScannersMu.RLock()
	_, ok := us.protocolScanners[name]
	us.protocolScannersMu.RUnlock()
	assert.True(t, ok)
	us.Stop()
}
