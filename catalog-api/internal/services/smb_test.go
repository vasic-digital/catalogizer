package services

import (
	"context"
	"errors"
	"net"
	"syscall"
	"testing"
	"time"

	"catalogizer/internal/config"
	"catalogizer/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
)

type SMBServiceTestSuite struct {
	suite.Suite
	service *SMBService
	logger  *zap.Logger
}

func (suite *SMBServiceTestSuite) SetupTest() {
	// Initialize logger
	logger, _ := zap.NewDevelopment()
	suite.logger = logger

	// Initialize service with mock config
	cfg := &config.Config{
		SMB: config.SMBConfig{
			Hosts: []config.SMBHost{
				{
					Name:     "test-server",
					Host:     "192.168.1.100",
					Port:     445,
					Username: "testuser",
					Password: "testpass",
				},
			},
			Timeout:   30,
			ChunkSize: 8192,
		},
	}

	suite.service = NewSMBService(cfg, logger)
}

func (suite *SMBServiceTestSuite) TestNewSMBService() {
	service := NewSMBService(nil, suite.logger)
	assert.NotNil(suite.T(), service)
	assert.NotNil(suite.T(), service.logger)
}

func (suite *SMBServiceTestSuite) TestSMBConnection() {
	// Test connection to non-existent server (should fail gracefully)
	err := suite.service.Connect("nonexistent-server")
	assert.Error(suite.T(), err)
}

func (suite *SMBServiceTestSuite) TestListSMBDirectory() {
	// Test listing directory from non-existent server
	files, err := suite.service.ListDirectory("test-server", "/media")
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), files)
}

func (suite *SMBServiceTestSuite) TestSMBConfig() {
	assert.Equal(suite.T(), 1, len(suite.service.config.SMB.Hosts))
	assert.Equal(suite.T(), "test-server", suite.service.config.SMB.Hosts[0].Name)
	assert.Equal(suite.T(), "testuser", suite.service.config.SMB.Hosts[0].Username)
	assert.Equal(suite.T(), "testpass", suite.service.config.SMB.Hosts[0].Password)
	assert.Equal(suite.T(), 30, suite.service.config.SMB.Timeout)
}

func (suite *SMBServiceTestSuite) TestSMBPathParsing() {
	tests := []struct {
		input    string
		expected models.SMBPath
	}{
		{
			"\\\\server\\share",
			models.SMBPath{Server: "server", Share: "share", Path: ""},
		},
		{
			"\\\\server\\share\\folder",
			models.SMBPath{Server: "server", Share: "share", Path: "folder"},
		},
		{
			"\\\\server\\share\\folder\\file.mp4",
			models.SMBPath{Server: "server", Share: "share", Path: "folder\\file.mp4"},
		},
	}

	for _, test := range tests {
		result := suite.service.ParseSMBPath(test.input)
		assert.Equal(suite.T(), test.expected.Server, result.Server)
		assert.Equal(suite.T(), test.expected.Share, result.Share)
		assert.Equal(suite.T(), test.expected.Path, result.Path)
	}
}

func (suite *SMBServiceTestSuite) TestSMBPathValidation() {
	validPaths := []string{
		"\\\\server\\share",
		"\\\\server.domain.com\\share",
		"\\\\192.168.1.100\\share\\folder",
		"\\\\server\\share\\path\\to\\file.mp4",
	}

	for _, path := range validPaths {
		assert.True(suite.T(), suite.service.IsValidSMBPath(path), "Path should be valid: %s", path)
	}

	invalidPaths := []string{
		"",
		"ftp://server/share",
		"\\\\",
		"\\\\server",
		"not-an-smb-path",
		"http://server/share",
	}

	for _, path := range invalidPaths {
		assert.False(suite.T(), suite.service.IsValidSMBPath(path), "Path should be invalid: %s", path)
	}
}

func (suite *SMBServiceTestSuite) TestSMBConnectionPooling() {
	// IsConnected only checks the config, so it returns true for any
	// host in the config regardless of reachability.
	initialState := suite.service.IsConnected("test-server")
	assert.True(suite.T(), initialState)

	// Connect() does a real TCP probe with a bounded timeout. ENV-F1: the fixture address 192.168.1.100 is a REAL SMB host on the developer
	// LAN, so no test may rely on, or dial, a LAN address. The fixture host is an RFC 1918 address no local interface carries (it passes the
	// real vetting, loopback would be refused); the vetted dial is redirected to a loopback port that was bound and closed at once, so the
	// connect is a real TCP attempt that is refused deterministically and reaches no other machine.
	fixtureHost := "10.254.254.1"
	if addrs, err := net.InterfaceAddrs(); err == nil {
		for _, a := range addrs {
			if ipn, ok := a.(*net.IPNet); ok && ipn.IP.Equal(net.ParseIP(fixtureHost)) {
				fixtureHost = "172.31.254.1"
			}
		}
	}
	suite.service.config.SMB.Hosts[0].Host = fixtureHost
	ln, lerr := net.Listen("tcp", "127.0.0.1:0")
	suite.Require().NoError(lerr)
	closedAddr := ln.Addr().String()
	suite.Require().NoError(ln.Close())
	var vetted string
	withSMBDialer(suite.T(), func(ctx context.Context, addr string, timeout time.Duration) (net.Conn, error) {
		vetted = addr
		d := net.Dialer{Timeout: timeout}
		return d.DialContext(ctx, "tcp", closedAddr)
	})
	err := suite.service.Connect("test-server")
	suite.Require().Error(err, "Connect to an unreachable host must fail")
	assert.Contains(suite.T(), vetted, fixtureHost+":", "the dial must have been handed the vetted fixture host")
	assert.Contains(suite.T(), err.Error(), "failed to connect to SMB host",
		"error should come from the network probe, not a config lookup")
	assert.NotContains(suite.T(), err.Error(), "is not allowed", "the host must pass vetting; this tests the connect failure")
	var opErr *net.OpError
	assert.True(suite.T(), errors.As(err, &opErr), "failure must be a network error, got %v", err)
	assert.True(suite.T(), errors.Is(err, syscall.ECONNREFUSED), "closed local port must refuse: %v", err)

	// IsConnected continues to return true because it's a config-level
	// check, not a live probe.
	currentState := suite.service.IsConnected("test-server")
	assert.True(suite.T(), currentState)
}

// TestSMBConnectUnknownHost verifies Connect fails fast for a host not in
// the config (distinct error class from a network failure).
func (suite *SMBServiceTestSuite) TestSMBConnectUnknownHost() {
	err := suite.service.Connect("nonexistent-host")
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "SMB host not found",
		"unknown host must surface a config-lookup error")
}

func (suite *SMBServiceTestSuite) TestSMBErrorHandling() {
	// Test various error conditions
	testCases := []struct {
		name        string
		path        string
		expectError bool
	}{
		{"Empty path", "", true},
		{"Invalid scheme", "ftp://server/share", true},
		{"Missing server", "\\\\share", true},
		{"Missing share", "\\\\server", true},
		{"Valid path", "\\\\server\\share", false},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			if tc.expectError {
				assert.False(t, suite.service.IsValidSMBPath(tc.path))
			} else {
				assert.True(t, suite.service.IsValidSMBPath(tc.path))
			}
		})
	}
}

func (suite *SMBServiceTestSuite) TestSMBFileOperations() {
	// Test file operations on non-existent server
	err := suite.service.CopyFile("server1", "/file1.txt", "server2", "/file1.txt")
	assert.Error(suite.T(), err)

	exists, err := suite.service.FileExists("server", "/file.txt")
	assert.Error(suite.T(), err)
	assert.False(suite.T(), exists)

	size, err := suite.service.GetFileSize("server", "/file.txt")
	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), int64(0), size)
}

func (suite *SMBServiceTestSuite) TestSMBDirectoryOperations() {
	// Test directory operations
	err := suite.service.CreateDirectory("server", "/newdir")
	assert.Error(suite.T(), err)

	err = suite.service.DeleteDirectory("server", "/olddir")
	assert.Error(suite.T(), err)

	exists, err := suite.service.DirectoryExists("server", "/dir")
	assert.Error(suite.T(), err)
	assert.False(suite.T(), exists)
}

func (suite *SMBServiceTestSuite) TestSMBConnectionTimeout() {
	// Test with very short timeout (should fail quickly)
	shortTimeoutConfig := &config.Config{
		SMB: config.SMBConfig{
			Hosts: []config.SMBHost{
				{
					Name:     "test-timeout",
					Host:     "nonexistent-host.invalid", // Invalid hostname that fails quickly
					Port:     445,
					Username: "test",
					Password: "test",
				},
			},
			Timeout: 1, // 1 second
		},
	}

	service := NewSMBService(shortTimeoutConfig, suite.logger)

	// This should fail quickly due to invalid hostname
	_, err := service.getConnection("test-timeout")
	assert.Error(suite.T(), err)
}

func (suite *SMBServiceTestSuite) TestSMBSourceManagement() {
	// Test adding/removing SMB sources
	initialCount := len(suite.service.config.SMB.Hosts)
	assert.Equal(suite.T(), 1, initialCount)

	// Test source validation
	validSources := []string{
		"\\\\server1\\share1",
		"\\\\server2\\share2",
		"\\\\192.168.1.100\\media",
	}

	for _, source := range validSources {
		suite.T().Logf("Testing path: %q", source)
		assert.True(suite.T(), suite.service.IsValidSMBPath(source), "Path should be valid: %s", source)
	}
}

func TestSMBServiceTestSuite(t *testing.T) {
	suite.Run(t, new(SMBServiceTestSuite))
}
