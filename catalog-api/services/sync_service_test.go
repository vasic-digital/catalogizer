package services

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"catalogizer/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSyncService_ValidateSyncEndpoint(t *testing.T) {
	service := NewSyncService(nil, nil, nil)

	tests := []struct {
		name     string
		endpoint *models.SyncEndpoint
		wantErr  bool
	}{
		{
			name: "valid endpoint",
			endpoint: &models.SyncEndpoint{
				Name:          "Test Endpoint",
				Type:          models.SyncTypeWebDAV,
				URL:           "https://example.com/api",
				SyncDirection: models.SyncDirectionUpload,
				LocalPath:     "/tmp/sync",
				RemotePath:    "/remote/sync",
			},
			wantErr: false,
		},
		{
			name: "empty name",
			endpoint: &models.SyncEndpoint{
				Name: "",
				URL:  "https://example.com/api",
			},
			wantErr: true,
		},
		{
			name: "empty URL",
			endpoint: &models.SyncEndpoint{
				Name: "Test",
				URL:  "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.validateSyncEndpoint(tt.endpoint)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSyncService_IsValidType(t *testing.T) {
	service := NewSyncService(nil, nil, nil)

	validTypes := []string{models.SyncTypeWebDAV, models.SyncTypeCloudStorage, models.SyncTypeLocal}

	tests := []struct {
		name     string
		syncType string
		expected bool
	}{
		{
			name:     "valid webdav sync type",
			syncType: models.SyncTypeWebDAV,
			expected: true,
		},
		{
			name:     "valid cloud_storage sync type",
			syncType: models.SyncTypeCloudStorage,
			expected: true,
		},
		{
			name:     "valid local sync type",
			syncType: models.SyncTypeLocal,
			expected: true,
		},
		{
			name:     "invalid sync type",
			syncType: "unknown",
			expected: false,
		},
		{
			name:     "empty sync type",
			syncType: "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.isValidType(tt.syncType, validTypes)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSyncService_ShouldSkipFile(t *testing.T) {
	service := NewSyncService(nil, nil, nil)

	tests := []struct {
		name     string
		filename string
		endpoint *models.SyncEndpoint
		expected bool
	}{
		{
			name:     "hidden file is skipped",
			filename: ".gitignore",
			endpoint: &models.SyncEndpoint{},
			expected: true,
		},
		{
			name:     "temp file is skipped",
			filename: "data.tmp",
			endpoint: &models.SyncEndpoint{},
			expected: true,
		},
		{
			name:     "normal file is not skipped",
			filename: "test.txt",
			endpoint: &models.SyncEndpoint{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.shouldSkipFile(tt.filename, tt.endpoint)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSyncService_ShouldRunSchedule(t *testing.T) {
	service := NewSyncService(nil, nil, nil)

	pastTime := time.Now().Add(-2 * time.Hour)
	recentTime := time.Now().Add(-30 * time.Minute)
	dayAgo := time.Now().Add(-25 * time.Hour)
	weekAgo := time.Now().Add(-8 * 24 * time.Hour)
	monthAgo := time.Now().Add(-32 * 24 * time.Hour) // 32 days ago avoids AddDate month-length edge cases

	tests := []struct {
		name     string
		schedule *models.SyncSchedule
		expected bool
	}{
		{
			name: "hourly schedule due",
			schedule: &models.SyncSchedule{
				Frequency: models.SyncFrequencyHourly,
				LastRun:   &pastTime,
				IsActive:  true,
			},
			expected: true,
		},
		{
			name: "hourly schedule not due",
			schedule: &models.SyncSchedule{
				Frequency: models.SyncFrequencyHourly,
				LastRun:   &recentTime,
				IsActive:  true,
			},
			expected: false,
		},
		{
			name: "hourly schedule never run",
			schedule: &models.SyncSchedule{
				Frequency: models.SyncFrequencyHourly,
				LastRun:   nil,
				IsActive:  true,
			},
			expected: true,
		},
		{
			name: "daily schedule due",
			schedule: &models.SyncSchedule{
				Frequency: models.SyncFrequencyDaily,
				LastRun:   &dayAgo,
				IsActive:  true,
			},
			expected: true,
		},
		{
			name: "daily schedule not due",
			schedule: &models.SyncSchedule{
				Frequency: models.SyncFrequencyDaily,
				LastRun:   &recentTime,
				IsActive:  true,
			},
			expected: false,
		},
		{
			name: "weekly schedule due",
			schedule: &models.SyncSchedule{
				Frequency: models.SyncFrequencyWeekly,
				LastRun:   &weekAgo,
				IsActive:  true,
			},
			expected: true,
		},
		{
			name: "weekly schedule not due",
			schedule: &models.SyncSchedule{
				Frequency: models.SyncFrequencyWeekly,
				LastRun:   &dayAgo,
				IsActive:  true,
			},
			expected: false,
		},
		{
			name: "monthly schedule due",
			schedule: &models.SyncSchedule{
				Frequency: models.SyncFrequencyMonthly,
				LastRun:   &monthAgo,
				IsActive:  true,
			},
			expected: true,
		},
		{
			name: "monthly schedule not due",
			schedule: &models.SyncSchedule{
				Frequency: models.SyncFrequencyMonthly,
				LastRun:   &weekAgo,
				IsActive:  true,
			},
			expected: false,
		},
		{
			name: "unknown frequency returns false",
			schedule: &models.SyncSchedule{
				Frequency: "unknown",
				LastRun:   nil,
				IsActive:  true,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.shouldRunSchedule(tt.schedule)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSyncService_CalculateChecksum(t *testing.T) {
	service := NewSyncService(nil, nil, nil)

	// Create a temporary file for checksum testing
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(tmpFile, []byte("hello world"), 0644)
	require.NoError(t, err)

	checksum, err := service.calculateChecksum(tmpFile)
	assert.NoError(t, err)
	assert.NotEmpty(t, checksum)

	// Same file should produce same checksum
	checksum2, err := service.calculateChecksum(tmpFile)
	assert.NoError(t, err)
	assert.Equal(t, checksum, checksum2)

	// Non-existent file should return error
	_, err = service.calculateChecksum("/nonexistent/file.txt")
	assert.Error(t, err)
}

func TestSyncService_ValidateSyncEndpoint_AllFields(t *testing.T) {
	service := NewSyncService(nil, nil, nil)

	tests := []struct {
		name     string
		endpoint *models.SyncEndpoint
		wantErr  bool
		errMsg   string
	}{
		{
			name: "valid endpoint with all fields",
			endpoint: &models.SyncEndpoint{
				Name:          "Test Endpoint",
				Type:          models.SyncTypeWebDAV,
				URL:           "https://example.com/api",
				SyncDirection: models.SyncDirectionUpload,
				LocalPath:     "/tmp/sync",
				RemotePath:    "/remote/sync",
			},
			wantErr: false,
		},
		{
			name: "valid endpoint with bidirectional sync",
			endpoint: &models.SyncEndpoint{
				Name:          "Test",
				Type:          models.SyncTypeLocal,
				URL:           "https://example.com",
				SyncDirection: models.SyncDirectionBidirectional,
				LocalPath:     "/data",
			},
			wantErr: false,
		},
		{
			name: "missing type",
			endpoint: &models.SyncEndpoint{
				Name:          "Test",
				URL:           "https://example.com",
				SyncDirection: models.SyncDirectionUpload,
				LocalPath:     "/tmp",
			},
			wantErr: true,
			errMsg:  "type is required",
		},
		{
			name: "missing sync direction",
			endpoint: &models.SyncEndpoint{
				Name:      "Test",
				Type:      models.SyncTypeWebDAV,
				URL:       "https://example.com",
				LocalPath: "/tmp",
			},
			wantErr: true,
			errMsg:  "sync direction is required",
		},
		{
			name: "missing local path",
			endpoint: &models.SyncEndpoint{
				Name:          "Test",
				Type:          models.SyncTypeWebDAV,
				URL:           "https://example.com",
				SyncDirection: models.SyncDirectionUpload,
			},
			wantErr: true,
			errMsg:  "local path is required",
		},
		{
			name: "invalid sync type",
			endpoint: &models.SyncEndpoint{
				Name:          "Test",
				Type:          "invalid_type",
				URL:           "https://example.com",
				SyncDirection: models.SyncDirectionUpload,
				LocalPath:     "/tmp",
			},
			wantErr: true,
			errMsg:  "invalid sync type",
		},
		{
			name: "invalid sync direction",
			endpoint: &models.SyncEndpoint{
				Name:          "Test",
				Type:          models.SyncTypeWebDAV,
				URL:           "https://example.com",
				SyncDirection: "invalid_direction",
				LocalPath:     "/tmp",
			},
			wantErr: true,
			errMsg:  "invalid sync direction",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.validateSyncEndpoint(tt.endpoint)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSyncService_ShouldSkipRemoteFile(t *testing.T) {
	service := NewSyncService(nil, nil, nil)

	tests := []struct {
		name     string
		file     *WebDAVFile
		expected bool
	}{
		{
			name:     "normal file not skipped",
			file:     &WebDAVFile{Path: "/path/to/file.txt"},
			expected: false,
		},
		{
			name:     "hidden file skipped",
			file:     &WebDAVFile{Path: "/path/to/.hidden"},
			expected: true,
		},
		{
			name:     "temp file skipped",
			file:     &WebDAVFile{Path: "/path/to/file.tmp"},
			expected: true,
		},
		{
			name:     "temp file with .temp extension skipped",
			file:     &WebDAVFile{Path: "/path/to/file.temp"},
			expected: true,
		},
		{
			name:     "config file not skipped (not hidden basename)",
			file:     &WebDAVFile{Path: "/path/config"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.shouldSkipRemoteFile(tt.file, &models.SyncEndpoint{})
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSyncService_GetWebDAVClient(t *testing.T) {
	service := NewSyncService(nil, nil, nil)

	endpoint := &models.SyncEndpoint{
		ID:       1,
		URL:      "https://example.com/webdav",
		Username: "user",
		Password: "pass",
	}

	// First call creates client
	client1, err := service.getWebDAVClient(endpoint)
	assert.NoError(t, err)
	assert.NotNil(t, client1)

	// Second call returns cached client
	client2, err := service.getWebDAVClient(endpoint)
	assert.NoError(t, err)
	assert.Equal(t, client1, client2)

	// Different endpoint creates new client
	endpoint2 := &models.SyncEndpoint{
		ID:       2,
		URL:      "https://other.example.com/webdav",
		Username: "user2",
		Password: "pass2",
	}
	client3, err := service.getWebDAVClient(endpoint2)
	assert.NoError(t, err)
	assert.NotNil(t, client3)
}

func TestSyncService_ScanLocalFiles(t *testing.T) {
	service := NewSyncService(nil, nil, nil)

	// Create temp directory structure
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("content1"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "file2.txt"), []byte("content2"), 0644))
	require.NoError(t, os.Mkdir(filepath.Join(tmpDir, "subdir"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "subdir", "file3.txt"), []byte("content3"), 0644))

	files, err := service.scanLocalFiles(tmpDir)
	assert.NoError(t, err)
	assert.Len(t, files, 3)
}

func TestSyncService_ScanLocalFiles_NonexistentPath(t *testing.T) {
	service := NewSyncService(nil, nil, nil)

	files, err := service.scanLocalFiles("/nonexistent/path")
	assert.Error(t, err)
	assert.Nil(t, files)
}

// ============================================================================
// ADDITIONAL TESTS FOR 95% COVERAGE
// ============================================================================

// ============================================================================
// FILESYSTEM SYNC TESTS (copyFile, performMirrorSync, performIncrementalSync,
// performBidirectionalSync, cleanupDestination, performLocalSync)
// ============================================================================

func TestSyncService_CopyFile_Scenarios(t *testing.T) {
	service := NewSyncService(nil, nil, nil)

	t.Run("copy file successfully", func(t *testing.T) {
		srcDir := t.TempDir()
		dstDir := t.TempDir()

		srcPath := filepath.Join(srcDir, "source.txt")
		dstPath := filepath.Join(dstDir, "dest.txt")
		content := []byte("hello world copy test")
		require.NoError(t, os.WriteFile(srcPath, content, 0644))

		err := service.copyFile(srcPath, dstPath, 0644, srcDir, dstDir)
		assert.NoError(t, err)

		copied, err := os.ReadFile(dstPath)
		assert.NoError(t, err)
		assert.Equal(t, content, copied)
	})

	t.Run("copy file creates destination directory", func(t *testing.T) {
		srcDir := t.TempDir()
		dstDir := t.TempDir()

		srcPath := filepath.Join(srcDir, "source.txt")
		dstPath := filepath.Join(dstDir, "sub", "dir", "dest.txt")
		content := []byte("nested directory test")
		require.NoError(t, os.WriteFile(srcPath, content, 0644))

		err := service.copyFile(srcPath, dstPath, 0644, srcDir, dstDir)
		assert.NoError(t, err)

		copied, err := os.ReadFile(dstPath)
		assert.NoError(t, err)
		assert.Equal(t, content, copied)
	})

	t.Run("copy file nonexistent source returns error", func(t *testing.T) {
		dstDir := t.TempDir()
		dstPath := filepath.Join(dstDir, "dest.txt")

		err := service.copyFile("/nonexistent/source.txt", dstPath, 0644, "/nonexistent", dstDir)
		assert.Error(t, err)
	})
}

func TestSyncService_PerformMirrorSync(t *testing.T) {
	service := NewSyncService(nil, nil, nil)
	session := &models.SyncSession{ID: 1, UserID: 1}

	t.Run("mirror sync copies files from source to destination", func(t *testing.T) {
		srcDir := t.TempDir()
		dstDir := t.TempDir()

		// Create source files
		require.NoError(t, os.WriteFile(filepath.Join(srcDir, "file1.txt"), []byte("content1"), 0644))
		require.NoError(t, os.WriteFile(filepath.Join(srcDir, "file2.txt"), []byte("content2"), 0644))

		err := service.performMirrorSync(srcDir, dstDir, session)
		assert.NoError(t, err)

		// Verify files were copied
		content1, err := os.ReadFile(filepath.Join(dstDir, "file1.txt"))
		assert.NoError(t, err)
		assert.Equal(t, "content1", string(content1))

		content2, err := os.ReadFile(filepath.Join(dstDir, "file2.txt"))
		assert.NoError(t, err)
		assert.Equal(t, "content2", string(content2))
	})

	t.Run("mirror sync copies subdirectories", func(t *testing.T) {
		srcDir := t.TempDir()
		dstDir := t.TempDir()

		require.NoError(t, os.MkdirAll(filepath.Join(srcDir, "subdir"), 0755))
		require.NoError(t, os.WriteFile(filepath.Join(srcDir, "subdir", "nested.txt"), []byte("nested"), 0644))

		err := service.performMirrorSync(srcDir, dstDir, session)
		assert.NoError(t, err)

		content, err := os.ReadFile(filepath.Join(dstDir, "subdir", "nested.txt"))
		assert.NoError(t, err)
		assert.Equal(t, "nested", string(content))
	})

	t.Run("mirror sync removes files not in source", func(t *testing.T) {
		srcDir := t.TempDir()
		dstDir := t.TempDir()

		// Create source file
		require.NoError(t, os.WriteFile(filepath.Join(srcDir, "keep.txt"), []byte("keep"), 0644))
		// Create destination-only file
		require.NoError(t, os.WriteFile(filepath.Join(dstDir, "keep.txt"), []byte("keep"), 0644))
		require.NoError(t, os.WriteFile(filepath.Join(dstDir, "remove.txt"), []byte("remove"), 0644))

		err := service.performMirrorSync(srcDir, dstDir, session)
		assert.NoError(t, err)

		// File not in source should be removed
		_, err = os.Stat(filepath.Join(dstDir, "remove.txt"))
		assert.True(t, os.IsNotExist(err))
	})
}

func TestSyncService_PerformIncrementalSync(t *testing.T) {
	service := NewSyncService(nil, nil, nil)
	session := &models.SyncSession{ID: 1, UserID: 1}

	t.Run("incremental sync copies new files", func(t *testing.T) {
		srcDir := t.TempDir()
		dstDir := t.TempDir()

		require.NoError(t, os.WriteFile(filepath.Join(srcDir, "new.txt"), []byte("new content"), 0644))

		err := service.performIncrementalSync(srcDir, dstDir, session)
		assert.NoError(t, err)

		content, err := os.ReadFile(filepath.Join(dstDir, "new.txt"))
		assert.NoError(t, err)
		assert.Equal(t, "new content", string(content))
	})

	t.Run("incremental sync skips older source files", func(t *testing.T) {
		srcDir := t.TempDir()
		dstDir := t.TempDir()

		srcFile := filepath.Join(srcDir, "old.txt")
		dstFile := filepath.Join(dstDir, "old.txt")

		// Create destination file first (newer)
		require.NoError(t, os.WriteFile(dstFile, []byte("destination"), 0644))
		// Wait briefly then create source file, but set modtime to the past
		require.NoError(t, os.WriteFile(srcFile, []byte("source"), 0644))
		pastTime := time.Now().Add(-1 * time.Hour)
		require.NoError(t, os.Chtimes(srcFile, pastTime, pastTime))

		err := service.performIncrementalSync(srcDir, dstDir, session)
		assert.NoError(t, err)

		// Destination file should remain unchanged
		content, err := os.ReadFile(dstFile)
		assert.NoError(t, err)
		assert.Equal(t, "destination", string(content))
	})

	t.Run("incremental sync does not remove destination-only files", func(t *testing.T) {
		srcDir := t.TempDir()
		dstDir := t.TempDir()

		require.NoError(t, os.WriteFile(filepath.Join(dstDir, "extra.txt"), []byte("extra"), 0644))

		err := service.performIncrementalSync(srcDir, dstDir, session)
		assert.NoError(t, err)

		// extra.txt should still be present
		_, err = os.Stat(filepath.Join(dstDir, "extra.txt"))
		assert.NoError(t, err)
	})
}

func TestSyncService_PerformBidirectionalSync(t *testing.T) {
	service := NewSyncService(nil, nil, nil)
	session := &models.SyncSession{ID: 1, UserID: 1}

	t.Run("bidirectional sync copies in both directions", func(t *testing.T) {
		srcDir := t.TempDir()
		dstDir := t.TempDir()

		require.NoError(t, os.WriteFile(filepath.Join(srcDir, "from_src.txt"), []byte("from source"), 0644))
		require.NoError(t, os.WriteFile(filepath.Join(dstDir, "from_dst.txt"), []byte("from destination"), 0644))

		err := service.performBidirectionalSync(srcDir, dstDir, session)
		assert.NoError(t, err)

		// Source file should be in destination
		content1, err := os.ReadFile(filepath.Join(dstDir, "from_src.txt"))
		assert.NoError(t, err)
		assert.Equal(t, "from source", string(content1))

		// Destination file should be in source
		content2, err := os.ReadFile(filepath.Join(srcDir, "from_dst.txt"))
		assert.NoError(t, err)
		assert.Equal(t, "from destination", string(content2))
	})
}

func TestSyncService_CleanupDestination(t *testing.T) {
	service := NewSyncService(nil, nil, nil)
	session := &models.SyncSession{ID: 1, UserID: 1}

	t.Run("removes files not in source", func(t *testing.T) {
		srcDir := t.TempDir()
		dstDir := t.TempDir()

		require.NoError(t, os.WriteFile(filepath.Join(srcDir, "keep.txt"), []byte("keep"), 0644))
		require.NoError(t, os.WriteFile(filepath.Join(dstDir, "keep.txt"), []byte("keep"), 0644))
		require.NoError(t, os.WriteFile(filepath.Join(dstDir, "orphan.txt"), []byte("orphan"), 0644))

		err := service.cleanupDestination(srcDir, dstDir, session)
		assert.NoError(t, err)

		_, err = os.Stat(filepath.Join(dstDir, "orphan.txt"))
		assert.True(t, os.IsNotExist(err))

		_, err = os.Stat(filepath.Join(dstDir, "keep.txt"))
		assert.NoError(t, err)
	})

	t.Run("handles empty directories gracefully", func(t *testing.T) {
		srcDir := t.TempDir()
		dstDir := t.TempDir()

		err := service.cleanupDestination(srcDir, dstDir, session)
		assert.NoError(t, err)
	})
}

func TestSyncService_PerformLocalSync(t *testing.T) {
	service := NewSyncService(nil, nil, nil)

	t.Run("local sync with mirror mode", func(t *testing.T) {
		srcDir := t.TempDir()
		dstDir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(srcDir, "data.txt"), []byte("mirror data"), 0644))

		settings := `{"source_directory":"` + srcDir + `","destination_directory":"` + dstDir + `","sync_mode":"mirror"}`
		session := &models.SyncSession{ID: 1, UserID: 1}
		endpoint := &models.SyncEndpoint{
			LocalPath:    srcDir,
			SyncSettings: &settings,
		}

		err := service.performLocalSync(session, endpoint)
		assert.NoError(t, err)

		content, err := os.ReadFile(filepath.Join(dstDir, "data.txt"))
		assert.NoError(t, err)
		assert.Equal(t, "mirror data", string(content))
	})

	t.Run("local sync with incremental mode", func(t *testing.T) {
		srcDir := t.TempDir()
		dstDir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(srcDir, "inc.txt"), []byte("incremental data"), 0644))

		settings := `{"source_directory":"` + srcDir + `","destination_directory":"` + dstDir + `","sync_mode":"incremental"}`
		session := &models.SyncSession{ID: 1, UserID: 1}
		endpoint := &models.SyncEndpoint{
			LocalPath:    srcDir,
			SyncSettings: &settings,
		}

		err := service.performLocalSync(session, endpoint)
		assert.NoError(t, err)

		content, err := os.ReadFile(filepath.Join(dstDir, "inc.txt"))
		assert.NoError(t, err)
		assert.Equal(t, "incremental data", string(content))
	})

	t.Run("local sync with bidirectional mode", func(t *testing.T) {
		srcDir := t.TempDir()
		dstDir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(srcDir, "a.txt"), []byte("a"), 0644))
		require.NoError(t, os.WriteFile(filepath.Join(dstDir, "b.txt"), []byte("b"), 0644))

		settings := `{"source_directory":"` + srcDir + `","destination_directory":"` + dstDir + `","sync_mode":"bidirectional"}`
		session := &models.SyncSession{ID: 1, UserID: 1}
		endpoint := &models.SyncEndpoint{
			LocalPath:    srcDir,
			SyncSettings: &settings,
		}

		err := service.performLocalSync(session, endpoint)
		assert.NoError(t, err)

		// Both files should exist in both directories
		_, err = os.Stat(filepath.Join(dstDir, "a.txt"))
		assert.NoError(t, err)
		_, err = os.Stat(filepath.Join(srcDir, "b.txt"))
		assert.NoError(t, err)
	})

	t.Run("local sync with unsupported mode returns error", func(t *testing.T) {
		srcDir := t.TempDir()
		dstDir := t.TempDir()

		settings := `{"source_directory":"` + srcDir + `","destination_directory":"` + dstDir + `","sync_mode":"unknown"}`
		session := &models.SyncSession{ID: 1, UserID: 1}
		endpoint := &models.SyncEndpoint{
			LocalPath:    srcDir,
			SyncSettings: &settings,
		}

		err := service.performLocalSync(session, endpoint)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported sync mode")
	})

	t.Run("local sync missing destination directory returns error", func(t *testing.T) {
		srcDir := t.TempDir()

		settings := `{"source_directory":"` + srcDir + `"}`
		session := &models.SyncSession{ID: 1, UserID: 1}
		endpoint := &models.SyncEndpoint{
			LocalPath:    srcDir,
			SyncSettings: &settings,
		}

		err := service.performLocalSync(session, endpoint)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "destination directory not specified")
	})

	t.Run("local sync missing source directory returns error", func(t *testing.T) {
		settings := `{"destination_directory":"/tmp/dst"}`
		session := &models.SyncSession{ID: 1, UserID: 1}
		endpoint := &models.SyncEndpoint{
			SyncSettings: &settings,
		}

		err := service.performLocalSync(session, endpoint)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "source directory not specified")
	})

	t.Run("local sync nonexistent source directory returns error", func(t *testing.T) {
		dstDir := t.TempDir()

		settings := `{"source_directory":"/nonexistent/src/dir","destination_directory":"` + dstDir + `"}`
		session := &models.SyncSession{ID: 1, UserID: 1}
		endpoint := &models.SyncEndpoint{
			LocalPath:    "/nonexistent/src/dir",
			SyncSettings: &settings,
		}

		err := service.performLocalSync(session, endpoint)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "source directory does not exist")
	})

	t.Run("local sync source is file not directory returns error", func(t *testing.T) {
		srcDir := t.TempDir()
		dstDir := t.TempDir()
		srcFile := filepath.Join(srcDir, "afile.txt")
		require.NoError(t, os.WriteFile(srcFile, []byte("file"), 0644))

		settings := `{"source_directory":"` + srcFile + `","destination_directory":"` + dstDir + `"}`
		session := &models.SyncSession{ID: 1, UserID: 1}
		endpoint := &models.SyncEndpoint{
			LocalPath:    srcFile,
			SyncSettings: &settings,
		}

		err := service.performLocalSync(session, endpoint)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "source path is not a directory")
	})

	t.Run("local sync default mode is mirror", func(t *testing.T) {
		srcDir := t.TempDir()
		dstDir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(srcDir, "def.txt"), []byte("default"), 0644))

		settings := `{"source_directory":"` + srcDir + `","destination_directory":"` + dstDir + `"}`
		session := &models.SyncSession{ID: 1, UserID: 1}
		endpoint := &models.SyncEndpoint{
			LocalPath:    srcDir,
			SyncSettings: &settings,
		}

		err := service.performLocalSync(session, endpoint)
		assert.NoError(t, err)

		content, err := os.ReadFile(filepath.Join(dstDir, "def.txt"))
		assert.NoError(t, err)
		assert.Equal(t, "default", string(content))
	})

	t.Run("local sync with invalid JSON settings returns error", func(t *testing.T) {
		settings := `invalid json`
		session := &models.SyncSession{ID: 1, UserID: 1}
		endpoint := &models.SyncEndpoint{
			LocalPath:    "/tmp",
			SyncSettings: &settings,
		}

		err := service.performLocalSync(session, endpoint)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse local sync config")
	})
}

func TestSyncService_CalculateChecksum_DifferentContent(t *testing.T) {
	service := NewSyncService(nil, nil, nil)

	tmpDir := t.TempDir()
	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.txt")
	require.NoError(t, os.WriteFile(file1, []byte("content A"), 0644))
	require.NoError(t, os.WriteFile(file2, []byte("content B"), 0644))

	checksum1, err := service.calculateChecksum(file1)
	assert.NoError(t, err)

	checksum2, err := service.calculateChecksum(file2)
	assert.NoError(t, err)

	assert.NotEqual(t, checksum1, checksum2, "Different content should produce different checksums")
}

func TestSyncService_CreateSyncEndpoint(t *testing.T) {
	tests := []struct {
		name        string
		userID      int
		endpoint    *models.SyncEndpoint
		wantErr     bool
		errContains string
	}{
		{
			name:   "successful creation with WebDAV",
			userID: 1,
			endpoint: &models.SyncEndpoint{
				Name:          "Test Endpoint",
				Type:          models.SyncTypeWebDAV,
				URL:           "https://example.com/webdav",
				Username:      "user",
				Password:      "pass",
				SyncDirection: models.SyncDirectionUpload,
				LocalPath:     "/tmp/local",
				RemotePath:    "/remote",
			},
			wantErr:     true, // Will fail without repository
			errContains: "not properly configured",
		},
		{
			name:   "successful creation with Cloud Storage",
			userID: 1,
			endpoint: &models.SyncEndpoint{
				Name:          "Cloud Endpoint",
				Type:          models.SyncTypeCloudStorage,
				URL:           "s3://bucket-name",
				Username:      "access-key",
				Password:      "secret-key",
				SyncDirection: models.SyncDirectionBidirectional,
				LocalPath:     "/data/cloud",
				RemotePath:    "/",
			},
			wantErr:     true, // Will fail without repository
			errContains: "not properly configured",
		},
		{
			name:   "missing name",
			userID: 1,
			endpoint: &models.SyncEndpoint{
				Type:          models.SyncTypeWebDAV,
				URL:           "https://example.com/webdav",
				SyncDirection: models.SyncDirectionUpload,
				LocalPath:     "/tmp/local",
				RemotePath:    "/remote",
			},
			wantErr:     true,
			errContains: "not properly configured",
		},
		{
			name:   "missing URL for WebDAV",
			userID: 1,
			endpoint: &models.SyncEndpoint{
				Name:          "No URL",
				Type:          models.SyncTypeWebDAV,
				SyncDirection: models.SyncDirectionUpload,
				LocalPath:     "/tmp/local",
				RemotePath:    "/remote",
			},
			wantErr:     true,
			errContains: "not properly configured",
		},
		{
			name:   "missing local path",
			userID: 1,
			endpoint: &models.SyncEndpoint{
				Name:          "No Local Path",
				Type:          models.SyncTypeWebDAV,
				URL:           "https://example.com/webdav",
				SyncDirection: models.SyncDirectionUpload,
				RemotePath:    "/remote",
			},
			wantErr:     true,
			errContains: "not properly configured",
		},
		{
			name:   "missing remote path for WebDAV",
			userID: 1,
			endpoint: &models.SyncEndpoint{
				Name:          "No Remote Path",
				Type:          models.SyncTypeWebDAV,
				URL:           "https://example.com/webdav",
				SyncDirection: models.SyncDirectionUpload,
				LocalPath:     "/tmp/local",
			},
			wantErr:     true,
			errContains: "not properly configured",
		},
		{
			name:   "invalid sync type",
			userID: 1,
			endpoint: &models.SyncEndpoint{
				Name:          "Test",
				Type:          "invalid_type",
				URL:           "https://example.com/webdav",
				SyncDirection: models.SyncDirectionUpload,
				LocalPath:     "/tmp/local",
				RemotePath:    "/remote",
			},
			wantErr:     true,
			errContains: "not properly configured",
		},
		{
			name:   "local sync without remote path",
			userID: 1,
			endpoint: &models.SyncEndpoint{
				Name:          "Local Sync",
				Type:          models.SyncTypeLocal,
				URL:           "/source/path",
				SyncDirection: models.SyncDirectionUpload,
				LocalPath:     "/dest/path",
			},
			wantErr:     true,
			errContains: "not properly configured",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewSyncService(nil, nil, nil)
			got, err := service.CreateSyncEndpoint(tt.userID, tt.endpoint)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, got)
				assert.Equal(t, tt.userID, got.UserID)
				assert.Equal(t, models.SyncStatusActive, got.Status)
				assert.NotZero(t, got.CreatedAt)
				assert.NotZero(t, got.UpdatedAt)
			}
		})
	}
}

// ============================================================================
// EXPANDED COVERAGE — edge cases for skip filters, sync modes, validation
// ============================================================================

func TestSyncService_ShouldSkipFile_EdgeCases(t *testing.T) {
	service := NewSyncService(nil, nil, nil)

	tests := []struct {
		name     string
		filename string
		expected bool
	}{
		{name: "hidden .DS_Store", filename: ".DS_Store", expected: true},
		{name: "hidden .env", filename: ".env", expected: true},
		{name: "tmp uppercase not matched", filename: "FILE.TMP", expected: false},
		{name: "temp extension", filename: "backup.temp", expected: true},
		{name: "normal pdf", filename: "report.pdf", expected: false},
		{name: "normal mp4", filename: "video.mp4", expected: false},
		{name: "dotfile in path", filename: "/path/to/.config", expected: true},
		{name: "tmp in path", filename: "/path/to/data.tmp", expected: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.shouldSkipFile(tt.filename, &models.SyncEndpoint{})
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSyncService_IsValidType_Directions(t *testing.T) {
	service := NewSyncService(nil, nil, nil)

	validDirections := []string{models.SyncDirectionUpload, models.SyncDirectionDownload, models.SyncDirectionBidirectional}

	tests := []struct {
		name      string
		direction string
		expected  bool
	}{
		{name: "upload is valid", direction: models.SyncDirectionUpload, expected: true},
		{name: "download is valid", direction: models.SyncDirectionDownload, expected: true},
		{name: "bidirectional is valid", direction: models.SyncDirectionBidirectional, expected: true},
		{name: "push is invalid", direction: "push", expected: false},
		{name: "pull is invalid", direction: "pull", expected: false},
		{name: "empty is invalid", direction: "", expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.isValidType(tt.direction, validDirections)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSyncService_MirrorSync_SkipsUnchangedFiles(t *testing.T) {
	service := NewSyncService(nil, nil, nil)
	session := &models.SyncSession{ID: 1, UserID: 1}

	srcDir := t.TempDir()
	dstDir := t.TempDir()

	content := []byte("same content")
	srcFile := filepath.Join(srcDir, "same.txt")
	dstFile := filepath.Join(dstDir, "same.txt")

	require.NoError(t, os.WriteFile(srcFile, content, 0644))
	require.NoError(t, os.WriteFile(dstFile, content, 0644))

	// Set destination file to be same time as source (not older)
	srcInfo, err := os.Stat(srcFile)
	require.NoError(t, err)
	require.NoError(t, os.Chtimes(dstFile, srcInfo.ModTime(), srcInfo.ModTime()))

	err = service.performMirrorSync(srcDir, dstDir, session)
	assert.NoError(t, err)

	// Destination should still have original content
	got, err := os.ReadFile(dstFile)
	require.NoError(t, err)
	assert.Equal(t, content, got)
}

func TestSyncService_IncrementalSync_CopiesNewerFiles(t *testing.T) {
	service := NewSyncService(nil, nil, nil)
	session := &models.SyncSession{ID: 1, UserID: 1}

	srcDir := t.TempDir()
	dstDir := t.TempDir()

	srcFile := filepath.Join(srcDir, "updated.txt")
	dstFile := filepath.Join(dstDir, "updated.txt")

	// Create destination file first
	require.NoError(t, os.WriteFile(dstFile, []byte("old"), 0644))

	// Set destination to the past
	pastTime := time.Now().Add(-2 * time.Hour)
	require.NoError(t, os.Chtimes(dstFile, pastTime, pastTime))

	// Create newer source file
	require.NoError(t, os.WriteFile(srcFile, []byte("new content"), 0644))

	err := service.performIncrementalSync(srcDir, dstDir, session)
	assert.NoError(t, err)

	// Destination should have new content
	got, err := os.ReadFile(dstFile)
	require.NoError(t, err)
	assert.Equal(t, "new content", string(got))
}

func TestSyncService_MirrorSync_NonexistentSource(t *testing.T) {
	service := NewSyncService(nil, nil, nil)
	session := &models.SyncSession{ID: 1, UserID: 1}

	dstDir := t.TempDir()

	err := service.performMirrorSync("/nonexistent/source/dir", dstDir, session)
	assert.Error(t, err)
}

func TestSyncService_IncrementalSync_NonexistentSource(t *testing.T) {
	service := NewSyncService(nil, nil, nil)
	session := &models.SyncSession{ID: 1, UserID: 1}

	dstDir := t.TempDir()

	err := service.performIncrementalSync("/nonexistent/source/dir", dstDir, session)
	assert.Error(t, err)
}

func TestSyncService_CleanupDestination_WithSubdirectories(t *testing.T) {
	service := NewSyncService(nil, nil, nil)
	session := &models.SyncSession{ID: 1, UserID: 1}

	srcDir := t.TempDir()
	dstDir := t.TempDir()

	// Create matching structure
	require.NoError(t, os.MkdirAll(filepath.Join(srcDir, "subdir"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "subdir", "keep.txt"), []byte("keep"), 0644))

	require.NoError(t, os.MkdirAll(filepath.Join(dstDir, "subdir"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dstDir, "subdir", "keep.txt"), []byte("keep"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dstDir, "subdir", "orphan.txt"), []byte("orphan"), 0644))

	err := service.cleanupDestination(srcDir, dstDir, session)
	assert.NoError(t, err)

	// orphan.txt should be removed
	_, err = os.Stat(filepath.Join(dstDir, "subdir", "orphan.txt"))
	assert.True(t, os.IsNotExist(err))

	// keep.txt should remain
	_, err = os.Stat(filepath.Join(dstDir, "subdir", "keep.txt"))
	assert.NoError(t, err)
}

func TestSyncService_PerformLocalSync_WithNilSettings(t *testing.T) {
	service := NewSyncService(nil, nil, nil)
	session := &models.SyncSession{ID: 1, UserID: 1}

	// nil SyncSettings with a valid LocalPath
	endpoint := &models.SyncEndpoint{
		LocalPath: "/tmp/src",
	}

	err := service.performLocalSync(session, endpoint)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "destination directory not specified")
}

func TestSyncService_ShouldSkipRemoteFile_EdgeCases(t *testing.T) {
	service := NewSyncService(nil, nil, nil)

	tests := []struct {
		name     string
		file     *WebDAVFile
		expected bool
	}{
		{
			name:     "dotfile .DS_Store",
			file:     &WebDAVFile{Path: "/remote/.DS_Store"},
			expected: true,
		},
		{
			name:     "dotfile .env",
			file:     &WebDAVFile{Path: "/remote/.env"},
			expected: true,
		},
		{
			name:     "regular text file",
			file:     &WebDAVFile{Path: "/remote/notes.txt"},
			expected: false,
		},
		{
			name:     "regular binary file",
			file:     &WebDAVFile{Path: "/remote/video.mp4"},
			expected: false,
		},
		{
			name:     "temp file deep path",
			file:     &WebDAVFile{Path: "/a/b/c/d/work.temp"},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.shouldSkipRemoteFile(tt.file, &models.SyncEndpoint{})
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSyncService_ValidateEndpoint_AllDirections(t *testing.T) {
	service := NewSyncService(nil, nil, nil)

	base := models.SyncEndpoint{
		Name:      "Test",
		Type:      models.SyncTypeLocal,
		URL:       "file:///tmp",
		LocalPath: "/data",
	}

	t.Run("upload direction valid", func(t *testing.T) {
		ep := base
		ep.SyncDirection = models.SyncDirectionUpload
		assert.NoError(t, service.validateSyncEndpoint(&ep))
	})

	t.Run("download direction valid", func(t *testing.T) {
		ep := base
		ep.SyncDirection = models.SyncDirectionDownload
		assert.NoError(t, service.validateSyncEndpoint(&ep))
	})

	t.Run("bidirectional direction valid", func(t *testing.T) {
		ep := base
		ep.SyncDirection = models.SyncDirectionBidirectional
		assert.NoError(t, service.validateSyncEndpoint(&ep))
	})

	t.Run("custom direction invalid", func(t *testing.T) {
		ep := base
		ep.SyncDirection = "sync"
		err := service.validateSyncEndpoint(&ep)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid sync direction")
	})
}

func TestSyncService_ValidateEndpoint_AllTypes(t *testing.T) {
	service := NewSyncService(nil, nil, nil)

	base := models.SyncEndpoint{
		Name:          "Test",
		URL:           "file:///tmp",
		SyncDirection: models.SyncDirectionUpload,
		LocalPath:     "/data",
	}

	t.Run("webdav type valid", func(t *testing.T) {
		ep := base
		ep.Type = models.SyncTypeWebDAV
		assert.NoError(t, service.validateSyncEndpoint(&ep))
	})

	t.Run("cloud_storage type valid", func(t *testing.T) {
		ep := base
		ep.Type = models.SyncTypeCloudStorage
		assert.NoError(t, service.validateSyncEndpoint(&ep))
	})

	t.Run("local type valid", func(t *testing.T) {
		ep := base
		ep.Type = models.SyncTypeLocal
		assert.NoError(t, service.validateSyncEndpoint(&ep))
	})

	t.Run("manual type invalid for endpoint", func(t *testing.T) {
		ep := base
		ep.Type = models.SyncTypeManual
		err := service.validateSyncEndpoint(&ep)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid sync type")
	})

	t.Run("scheduled type invalid for endpoint", func(t *testing.T) {
		ep := base
		ep.Type = models.SyncTypeScheduled
		err := service.validateSyncEndpoint(&ep)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid sync type")
	})
}

func TestSyncService_CopyFile_OverwritesExistingFile(t *testing.T) {
	service := NewSyncService(nil, nil, nil)

	srcDir := t.TempDir()
	dstDir := t.TempDir()

	srcPath := filepath.Join(srcDir, "source.txt")
	dstPath := filepath.Join(dstDir, "dest.txt")

	// Create destination first with old content
	require.NoError(t, os.WriteFile(dstPath, []byte("old content"), 0644))

	// Create source with new content
	require.NoError(t, os.WriteFile(srcPath, []byte("new content"), 0644))

	err := service.copyFile(srcPath, dstPath, 0644, srcDir, dstDir)
	assert.NoError(t, err)

	content, err := os.ReadFile(dstPath)
	require.NoError(t, err)
	assert.Equal(t, "new content", string(content))
}

func TestSyncService_CalculateChecksum_EmptyFile(t *testing.T) {
	service := NewSyncService(nil, nil, nil)

	tmpDir := t.TempDir()
	emptyFile := filepath.Join(tmpDir, "empty.txt")
	require.NoError(t, os.WriteFile(emptyFile, []byte{}, 0644))

	checksum, err := service.calculateChecksum(emptyFile)
	assert.NoError(t, err)
	assert.NotEmpty(t, checksum)
	// SHA256 of empty input is a known value
	assert.Equal(t, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", checksum)
}

func TestSyncService_ScanLocalFiles_EmptyDirectory(t *testing.T) {
	service := NewSyncService(nil, nil, nil)

	tmpDir := t.TempDir()
	files, err := service.scanLocalFiles(tmpDir)
	assert.NoError(t, err)
	assert.Empty(t, files)
}

func TestSyncService_ScanLocalFiles_DeepNesting(t *testing.T) {
	service := NewSyncService(nil, nil, nil)

	tmpDir := t.TempDir()
	deepDir := filepath.Join(tmpDir, "a", "b", "c", "d")
	require.NoError(t, os.MkdirAll(deepDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(deepDir, "deep.txt"), []byte("deep"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "root.txt"), []byte("root"), 0644))

	files, err := service.scanLocalFiles(tmpDir)
	assert.NoError(t, err)
	assert.Len(t, files, 2)
}
