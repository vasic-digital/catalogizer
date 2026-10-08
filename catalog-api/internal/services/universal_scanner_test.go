package services

import (
	"fmt"
	"testing"

	"catalogizer/database"
	"catalogizer/models"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestLocalScanner_GetScanStrategy(t *testing.T) {
	mockLogger := zap.NewNop()
	scanner := NewLocalScanner(nil, mockLogger)

	strategy := scanner.GetScanStrategy()
	assert.True(t, strategy.UseRecursiveListing)
	assert.True(t, strategy.ParallelDirectories)
	assert.True(t, strategy.ChecksumCalculation)
	assert.True(t, strategy.MetadataExtraction)
	assert.True(t, strategy.RealTimeChangeDetection)
	assert.Equal(t, 1000, strategy.BatchSize)
}

func TestSMBScanner_GetScanStrategy(t *testing.T) {
	mockLogger := zap.NewNop()
	scanner := NewSMBScanner(nil, mockLogger)

	strategy := scanner.GetScanStrategy()
	assert.False(t, strategy.UseRecursiveListing)
	assert.False(t, strategy.ParallelDirectories)
	assert.False(t, strategy.ChecksumCalculation)
	assert.True(t, strategy.MetadataExtraction)
	assert.False(t, strategy.RealTimeChangeDetection)
	assert.Equal(t, 500, strategy.BatchSize)
}

func TestFTPScanner_GetScanStrategy(t *testing.T) {
	mockLogger := zap.NewNop()
	scanner := NewFTPScanner(mockLogger)

	strategy := scanner.GetScanStrategy()
	assert.False(t, strategy.UseRecursiveListing)
	assert.False(t, strategy.ParallelDirectories)
	assert.False(t, strategy.ChecksumCalculation)
	assert.False(t, strategy.MetadataExtraction)
	assert.False(t, strategy.RealTimeChangeDetection)
	assert.Equal(t, 100, strategy.BatchSize)
}

func TestNFSScanner_GetScanStrategy(t *testing.T) {
	mockLogger := zap.NewNop()
	scanner := NewNFSScanner(mockLogger)

	strategy := scanner.GetScanStrategy()
	assert.True(t, strategy.UseRecursiveListing)
	assert.True(t, strategy.ParallelDirectories)
	// PA-03: the scanner does not compute checksums, so it no longer claims to (the stub declared true while doing nothing).
	assert.False(t, strategy.ChecksumCalculation)
	assert.True(t, strategy.MetadataExtraction)
	assert.False(t, strategy.RealTimeChangeDetection)
	assert.Equal(t, 800, strategy.BatchSize)
}

func TestWebDAVScanner_GetScanStrategy(t *testing.T) {
	mockLogger := zap.NewNop()
	scanner := NewWebDAVScanner(mockLogger)

	strategy := scanner.GetScanStrategy()
	assert.False(t, strategy.UseRecursiveListing)
	assert.False(t, strategy.ParallelDirectories)
	assert.False(t, strategy.ChecksumCalculation)
	assert.True(t, strategy.MetadataExtraction)
	assert.False(t, strategy.RealTimeChangeDetection)
	assert.Equal(t, 200, strategy.BatchSize)
}

func TestScanners_SupportsIncrementalScan(t *testing.T) {
	mockLogger := zap.NewNop()

	tests := []struct {
		name     string
		scanner  ProtocolScanner
		expected bool
	}{
		{
			name:     "local supports incremental",
			scanner:  NewLocalScanner(nil, mockLogger),
			expected: true,
		},
		{
			name:     "SMB supports incremental",
			scanner:  NewSMBScanner(nil, mockLogger),
			expected: true,
		},
		{
			name:     "FTP does not claim incremental without a token store (WF22 R6)",
			scanner:  NewFTPScanner(mockLogger),
			expected: false,
		},
		{
			name:     "NFS does not claim incremental without a token store (WF22 R6; the stub claimed true while doing nothing)",
			scanner:  NewNFSScanner(mockLogger),
			expected: false,
		},
		{
			name:     "WebDAV does not claim incremental without a token store (WF22 R6)",
			scanner:  NewWebDAVScanner(mockLogger),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.scanner.SupportsIncrementalScan()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestScanners_GetOptimalBatchSize(t *testing.T) {
	mockLogger := zap.NewNop()

	tests := []struct {
		name     string
		scanner  ProtocolScanner
		expected int
	}{
		{
			name:     "local batch size",
			scanner:  NewLocalScanner(nil, mockLogger),
			expected: 1000,
		},
		{
			name:     "SMB batch size",
			scanner:  NewSMBScanner(nil, mockLogger),
			expected: 500,
		},
		{
			name:     "FTP batch size",
			scanner:  NewFTPScanner(mockLogger),
			expected: 100,
		},
		{
			name:     "NFS batch size",
			scanner:  NewNFSScanner(mockLogger),
			expected: 800,
		},
		{
			name:     "WebDAV batch size",
			scanner:  NewWebDAVScanner(mockLogger),
			expected: 200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.scanner.GetOptimalBatchSize()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestUniversalScanner_RegisterProtocolScanner(t *testing.T) {
	var mockDB *database.DB
	mockLogger := zap.NewNop()
	scanner := NewUniversalScanner(mockDB, mockLogger, nil, nil)

	// Register a custom scanner
	customScanner := NewLocalScanner(nil, mockLogger)
	scanner.RegisterProtocolScanner("custom", customScanner)

	// Verify it was registered (no direct accessor, but no panic means success)
	assert.NotNil(t, scanner)
}

func TestUniversalScanner_GetActiveScanStatus(t *testing.T) {
	var mockDB *database.DB
	mockLogger := zap.NewNop()
	scanner := NewUniversalScanner(mockDB, mockLogger, nil, nil)

	status, exists := scanner.GetActiveScanStatus("nonexistent")
	assert.False(t, exists)
	assert.Nil(t, status)
}

func TestUniversalScanner_GetAllActiveScanStatuses(t *testing.T) {
	var mockDB *database.DB
	mockLogger := zap.NewNop()
	scanner := NewUniversalScanner(mockDB, mockLogger, nil, nil)

	statuses := scanner.GetAllActiveScanStatuses()
	assert.NotNil(t, statuses)
	assert.Equal(t, 0, len(statuses))
}

func TestScanStatus_UpdateStatus(t *testing.T) {
	status := &ScanStatus{
		JobID:  "test_job",
		Status: "running",
	}

	status.updateStatus("completed")
	snapshot := status.GetSnapshot()
	assert.Equal(t, "completed", snapshot.Status)
}

func TestScanStatus_UpdateCurrentPath(t *testing.T) {
	status := &ScanStatus{
		JobID: "test_job",
	}

	status.updateCurrentPath("/some/path")
	snapshot := status.GetSnapshot()
	assert.Equal(t, "/some/path", snapshot.CurrentPath)
}

func TestScanStatus_IncrementCounters(t *testing.T) {
	status := &ScanStatus{
		JobID: "test_job",
	}

	status.incrementCounters(10, 8, 5, 2, 1)
	snapshot := status.GetSnapshot()
	assert.Equal(t, int64(10), snapshot.FilesProcessed)
	assert.Equal(t, int64(8), snapshot.FilesFound)
	assert.Equal(t, int64(5), snapshot.FilesUpdated)
	assert.Equal(t, int64(2), snapshot.FilesDeleted)
	assert.Equal(t, int64(1), snapshot.ErrorCount)

	// Increment again
	status.incrementCounters(5, 3, 2, 1, 0)
	snapshot = status.GetSnapshot()
	assert.Equal(t, int64(15), snapshot.FilesProcessed)
	assert.Equal(t, int64(11), snapshot.FilesFound)
	assert.Equal(t, int64(7), snapshot.FilesUpdated)
	assert.Equal(t, int64(3), snapshot.FilesDeleted)
	assert.Equal(t, int64(1), snapshot.ErrorCount)
}

func TestScanStatus_GetSnapshot(t *testing.T) {
	status := &ScanStatus{
		JobID:           "test_job",
		StorageRootName: "media",
		Protocol:        "local",
		Status:          "running",
	}

	snapshot := status.GetSnapshot()
	assert.Equal(t, "test_job", snapshot.JobID)
	assert.Equal(t, "media", snapshot.StorageRootName)
	assert.Equal(t, "local", snapshot.Protocol)
	assert.Equal(t, "running", snapshot.Status)
}

func TestUniversalScanner_ConcurrentWorkers(t *testing.T) {
	// Verify scanner is created with multiple concurrent workers
	var mockDB *database.DB
	mockLogger := zap.NewNop()
	scanner := NewUniversalScanner(mockDB, mockLogger, nil, nil)

	assert.NotNil(t, scanner)
	assert.Equal(t, 4, scanner.workers, "scanner must support at least 4 concurrent workers")
}

func TestUniversalScanner_QueueCapacity(t *testing.T) {
	// Verify the scan queue can hold multiple jobs
	var mockDB *database.DB
	mockLogger := zap.NewNop()
	scanner := NewUniversalScanner(mockDB, mockLogger, nil, nil)

	assert.NotNil(t, scanner)
	assert.Equal(t, 100, cap(scanner.scanQueue), "scan queue must have capacity for multiple concurrent scans")
}

func TestUniversalScanner_QueueMultipleScans(t *testing.T) {
	// Verify multiple scan jobs can be queued (one per content directory)
	// Workers are NOT started — we only verify the queue accepts all jobs
	var mockDB *database.DB
	mockLogger := zap.NewNop()
	scanner := NewUniversalScanner(mockDB, mockLogger, nil, nil)

	// Queue 5 scans (one per content directory, as the populate challenge does)
	directories := []string{"Music", "Series", "Movies", "Software", "Comics"}
	for i, dir := range directories {
		job := ScanJob{
			ID: fmt.Sprintf("test-job-%d", i),
			StorageRoot: &models.StorageRoot{
				Name:     "test-nas",
				Protocol: "local",
				Host:     strPtr("localhost"),
				Path:     strPtr("/tmp/nonexistent"),
			},
			Path:     dir,
			ScanType: "full",
			MaxDepth: 10,
		}
		err := scanner.QueueScan(job)
		assert.NoError(t, err, "should be able to queue scan for directory %s", dir)
	}

	// Verify all 5 jobs are in the queue
	assert.Equal(t, 5, len(scanner.scanQueue), "all 5 directory scans must be queued")
}

func TestUniversalScanner_TrackMultipleActiveScans(t *testing.T) {
	// Verify multiple scans can be tracked simultaneously
	var mockDB *database.DB
	mockLogger := zap.NewNop()
	scanner := NewUniversalScanner(mockDB, mockLogger, nil, nil)

	// Manually add scan statuses to verify concurrent tracking
	for i := 0; i < 5; i++ {
		jobID := fmt.Sprintf("concurrent-job-%d", i)
		status := &ScanStatus{
			JobID:           jobID,
			StorageRootName: "test-nas",
			Protocol:        "smb",
			Status:          "running",
		}
		scanner.activeScansMu.Lock()
		scanner.activeScans[jobID] = status
		scanner.activeScansMu.Unlock()
	}

	// Verify all 5 scans are tracked
	statuses := scanner.GetAllActiveScanStatuses()
	assert.Equal(t, 5, len(statuses), "must track 5 concurrent active scans")

	// Verify each scan can be retrieved individually
	for i := 0; i < 5; i++ {
		jobID := fmt.Sprintf("concurrent-job-%d", i)
		status, exists := scanner.GetActiveScanStatus(jobID)
		assert.True(t, exists, "scan %s must be retrievable", jobID)
		assert.NotNil(t, status)
		assert.Equal(t, "running", status.GetSnapshot().Status)
	}
}

func TestUniversalScanner_QueueFullReturnsError(t *testing.T) {
	// Verify that queuing to a full queue returns an error (not blocks)
	var mockDB *database.DB
	mockLogger := zap.NewNop()
	scanner := NewUniversalScanner(mockDB, mockLogger, nil, nil)

	// Don't start workers — queue will fill up
	// Fill the queue to capacity
	for i := 0; i < 1000; i++ {
		job := ScanJob{
			ID: fmt.Sprintf("fill-job-%d", i),
			StorageRoot: &models.StorageRoot{
				Name:     "test",
				Protocol: "local",
			},
			Path: fmt.Sprintf("dir-%d", i),
		}
		_ = scanner.QueueScan(job)
	}

	// Next queue should fail
	overflowJob := ScanJob{
		ID: "overflow-job",
		StorageRoot: &models.StorageRoot{
			Name:     "test",
			Protocol: "local",
		},
		Path: "overflow",
	}
	err := scanner.QueueScan(overflowJob)
	assert.Error(t, err, "queuing to a full queue must return an error")
	assert.Contains(t, err.Error(), "queue is full")
}
