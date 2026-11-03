package handlers

import (
	"catalogizer/models"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type ServiceAdaptersTestSuite struct {
	suite.Suite
}

// AuthServiceAdapter tests

func (suite *ServiceAdaptersTestSuite) TestAuthServiceAdapter_NilInner() {
	adapter := &AuthServiceAdapter{Inner: nil}
	assert.NotNil(suite.T(), adapter)
	assert.Nil(suite.T(), adapter.Inner)
}

func (suite *ServiceAdaptersTestSuite) TestAuthServiceAdapter_CheckPermission_NilInner() {
	adapter := &AuthServiceAdapter{Inner: nil}
	// CheckPermission accesses userRepo which is nil, causing a panic
	assert.Panics(suite.T(), func() {
		adapter.CheckPermission(1, "admin")
	})
}

func (suite *ServiceAdaptersTestSuite) TestAuthServiceAdapter_GetCurrentUser_NilInner() {
	// GetCurrentUser calls validateToken which uses jwtSecret (nil byte slice)
	// On a nil *AuthService, this may not panic if the method handles nil gracefully
	adapter := &AuthServiceAdapter{Inner: nil}
	user, err := adapter.GetCurrentUser("invalid-token")
	// With a nil AuthService, jwtSecret is nil, so token parsing fails with error
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), user)
}

func (suite *ServiceAdaptersTestSuite) TestAuthServiceAdapter_HashPassword_NilInner() {
	// HashPasswordForUser calls generateSalt which creates crypto random bytes
	// then hashPassword which uses bcrypt - should work on nil receiver
	adapter := &AuthServiceAdapter{Inner: nil}
	hash, err := adapter.HashPassword("password123")
	// This should succeed since the method doesn't access struct fields for hashing
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), hash)
}

func (suite *ServiceAdaptersTestSuite) TestAuthServiceAdapter_ValidatePassword_NilInner() {
	// ValidatePassword only checks string length, no struct fields accessed
	adapter := &AuthServiceAdapter{Inner: nil}
	err := adapter.ValidatePassword("Password123!")
	assert.NoError(suite.T(), err)
}

func (suite *ServiceAdaptersTestSuite) TestAuthServiceAdapter_ValidatePassword_TooShort() {
	adapter := &AuthServiceAdapter{Inner: nil}
	err := adapter.ValidatePassword("short")
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "at least 8 characters")
}

// ConfigurationServiceAdapter tests

func (suite *ServiceAdaptersTestSuite) TestConfigurationServiceAdapter_NilInner() {
	adapter := &ConfigurationServiceAdapter{Inner: nil}
	assert.NotNil(suite.T(), adapter)
	assert.Nil(suite.T(), adapter.Inner)
}

func (suite *ServiceAdaptersTestSuite) TestConfigurationServiceAdapter_TestConfiguration_NilConfig() {
	adapter := &ConfigurationServiceAdapter{Inner: nil}
	result, err := adapter.TestConfiguration(nil)
	assert.NoError(suite.T(), err)
	assert.True(suite.T(), result.IsValid)
}

func (suite *ServiceAdaptersTestSuite) TestConfigurationServiceAdapter_GetConfigurationSchema_NilInner() {
	adapter := &ConfigurationServiceAdapter{Inner: nil}
	schema, err := adapter.GetConfigurationSchema()
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), schema)
}

// ErrorReportingServiceAdapter tests

func (suite *ServiceAdaptersTestSuite) TestErrorReportingServiceAdapter_NilInner() {
	adapter := &ErrorReportingServiceAdapter{Inner: nil}
	assert.NotNil(suite.T(), adapter)
	assert.Nil(suite.T(), adapter.Inner)
}

func (suite *ServiceAdaptersTestSuite) TestErrorReportingServiceAdapter_ReportError_NilInner() {
	adapter := &ErrorReportingServiceAdapter{Inner: nil}
	assert.Panics(suite.T(), func() {
		adapter.ReportError(1, nil)
	})
}

// LogManagementServiceAdapter tests

func (suite *ServiceAdaptersTestSuite) TestLogManagementServiceAdapter_NilInner() {
	adapter := &LogManagementServiceAdapter{Inner: nil}
	assert.NotNil(suite.T(), adapter)
	assert.Nil(suite.T(), adapter.Inner)
}

func (suite *ServiceAdaptersTestSuite) TestLogManagementServiceAdapter_CollectLogs_NilInner() {
	adapter := &LogManagementServiceAdapter{Inner: nil}
	assert.Panics(suite.T(), func() {
		adapter.CollectLogs(1, nil)
	})
}

// AuthServiceAdapter — additional delegation tests

func (suite *ServiceAdaptersTestSuite) TestAuthServiceAdapter_GenerateSecureToken_NilInner() {
	adapter := &AuthServiceAdapter{Inner: nil}
	token, err := adapter.GenerateSecureToken(32)
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), token, 64) // hex encoded 32 bytes = 64 chars
}

func (suite *ServiceAdaptersTestSuite) TestAuthServiceAdapter_GenerateSecureToken_DifferentLengths() {
	adapter := &AuthServiceAdapter{Inner: nil}
	token16, _ := adapter.GenerateSecureToken(16)
	token32, _ := adapter.GenerateSecureToken(32)
	assert.Len(suite.T(), token16, 32)
	assert.Len(suite.T(), token32, 64)
}

func (suite *ServiceAdaptersTestSuite) TestAuthServiceAdapter_GenerateSecureToken_Unique() {
	adapter := &AuthServiceAdapter{Inner: nil}
	token1, _ := adapter.GenerateSecureToken(32)
	token2, _ := adapter.GenerateSecureToken(32)
	assert.NotEqual(suite.T(), token1, token2)
}

func (suite *ServiceAdaptersTestSuite) TestAuthServiceAdapter_ValidateToken_NilInner() {
	adapter := &AuthServiceAdapter{Inner: nil}
	user, err := adapter.ValidateToken("invalid-token")
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), user)
}

func (suite *ServiceAdaptersTestSuite) TestAuthServiceAdapter_ResetPassword_NilInner() {
	adapter := &AuthServiceAdapter{Inner: nil}
	assert.Panics(suite.T(), func() {
		adapter.ResetPassword(1, "newpassword123")
	})
}

func (suite *ServiceAdaptersTestSuite) TestAuthServiceAdapter_LockAccount_NilInner() {
	adapter := &AuthServiceAdapter{Inner: nil}
	assert.Panics(suite.T(), func() {
		adapter.LockAccount(1, time.Now().Add(time.Hour))
	})
}

func (suite *ServiceAdaptersTestSuite) TestAuthServiceAdapter_UnlockAccount_NilInner() {
	adapter := &AuthServiceAdapter{Inner: nil}
	assert.Panics(suite.T(), func() {
		adapter.UnlockAccount(1)
	})
}

func (suite *ServiceAdaptersTestSuite) TestAuthServiceAdapter_HashPassword_DifferentPasswords() {
	adapter := &AuthServiceAdapter{Inner: nil}
	hash1, err1 := adapter.HashPassword("password1")
	hash2, err2 := adapter.HashPassword("password2")
	assert.NoError(suite.T(), err1)
	assert.NoError(suite.T(), err2)
	assert.NotEqual(suite.T(), hash1, hash2)
}

// ConfigurationServiceAdapter — additional delegation tests

func (suite *ServiceAdaptersTestSuite) TestConfigurationServiceAdapter_GetWizardStep_NilInner() {
	adapter := &ConfigurationServiceAdapter{Inner: nil}
	assert.Panics(suite.T(), func() {
		adapter.GetWizardStep("step1")
	})
}

func (suite *ServiceAdaptersTestSuite) TestConfigurationServiceAdapter_ValidateWizardStep_NilInner() {
	adapter := &ConfigurationServiceAdapter{Inner: nil}
	assert.Panics(suite.T(), func() {
		adapter.ValidateWizardStep("step1", map[string]interface{}{"key": "value"})
	})
}

func (suite *ServiceAdaptersTestSuite) TestConfigurationServiceAdapter_SaveWizardProgress_NilInner() {
	adapter := &ConfigurationServiceAdapter{Inner: nil}
	assert.Panics(suite.T(), func() {
		adapter.SaveWizardProgress(1, "step1", map[string]interface{}{"key": "value"})
	})
}

func (suite *ServiceAdaptersTestSuite) TestConfigurationServiceAdapter_GetWizardProgress_NilInner() {
	adapter := &ConfigurationServiceAdapter{Inner: nil}
	assert.Panics(suite.T(), func() {
		adapter.GetWizardProgress(1)
	})
}

func (suite *ServiceAdaptersTestSuite) TestConfigurationServiceAdapter_CompleteWizard_NilInner() {
	adapter := &ConfigurationServiceAdapter{Inner: nil}
	assert.Panics(suite.T(), func() {
		adapter.CompleteWizard(1, map[string]interface{}{"key": "value"})
	})
}

func (suite *ServiceAdaptersTestSuite) TestConfigurationServiceAdapter_GetConfiguration_NilInner() {
	adapter := &ConfigurationServiceAdapter{Inner: nil}
	assert.Panics(suite.T(), func() {
		adapter.GetConfiguration()
	})
}

func (suite *ServiceAdaptersTestSuite) TestConfigurationServiceAdapter_TestConfiguration_WithConfig() {
	adapter := &ConfigurationServiceAdapter{Inner: nil}
	result, err := adapter.TestConfiguration(&models.Configuration{ID: "test-config"})
	assert.NoError(suite.T(), err)
	assert.True(suite.T(), result.IsValid)
}

// ErrorReportingServiceAdapter — additional delegation tests

func (suite *ServiceAdaptersTestSuite) TestErrorReportingServiceAdapter_ReportCrash_NilInner() {
	adapter := &ErrorReportingServiceAdapter{Inner: nil}
	assert.Panics(suite.T(), func() {
		adapter.ReportCrash(1, nil)
	})
}

func (suite *ServiceAdaptersTestSuite) TestErrorReportingServiceAdapter_GetErrorReport_NilInner() {
	adapter := &ErrorReportingServiceAdapter{Inner: nil}
	assert.Panics(suite.T(), func() {
		adapter.GetErrorReport(1, 1)
	})
}

func (suite *ServiceAdaptersTestSuite) TestErrorReportingServiceAdapter_GetCrashReport_NilInner() {
	adapter := &ErrorReportingServiceAdapter{Inner: nil}
	assert.Panics(suite.T(), func() {
		adapter.GetCrashReport(1, 1)
	})
}

func (suite *ServiceAdaptersTestSuite) TestErrorReportingServiceAdapter_GetErrorReportsByUser_NilInner() {
	adapter := &ErrorReportingServiceAdapter{Inner: nil}
	assert.Panics(suite.T(), func() {
		adapter.GetErrorReportsByUser(1, nil)
	})
}

func (suite *ServiceAdaptersTestSuite) TestErrorReportingServiceAdapter_GetCrashReportsByUser_NilInner() {
	adapter := &ErrorReportingServiceAdapter{Inner: nil}
	assert.Panics(suite.T(), func() {
		adapter.GetCrashReportsByUser(1, nil)
	})
}

func (suite *ServiceAdaptersTestSuite) TestErrorReportingServiceAdapter_UpdateErrorStatus_NilInner() {
	adapter := &ErrorReportingServiceAdapter{Inner: nil}
	assert.Panics(suite.T(), func() {
		adapter.UpdateErrorStatus(1, 1, "resolved")
	})
}

func (suite *ServiceAdaptersTestSuite) TestErrorReportingServiceAdapter_UpdateCrashStatus_NilInner() {
	adapter := &ErrorReportingServiceAdapter{Inner: nil}
	assert.Panics(suite.T(), func() {
		adapter.UpdateCrashStatus(1, 1, "resolved")
	})
}

func (suite *ServiceAdaptersTestSuite) TestErrorReportingServiceAdapter_GetErrorStatistics_NilInner() {
	adapter := &ErrorReportingServiceAdapter{Inner: nil}
	assert.Panics(suite.T(), func() {
		adapter.GetErrorStatistics(1)
	})
}

func (suite *ServiceAdaptersTestSuite) TestErrorReportingServiceAdapter_GetCrashStatistics_NilInner() {
	adapter := &ErrorReportingServiceAdapter{Inner: nil}
	assert.Panics(suite.T(), func() {
		adapter.GetCrashStatistics(1)
	})
}

func (suite *ServiceAdaptersTestSuite) TestErrorReportingServiceAdapter_GetSystemHealth_NilInner() {
	adapter := &ErrorReportingServiceAdapter{Inner: nil}
	assert.Panics(suite.T(), func() {
		adapter.GetSystemHealth()
	})
}

func (suite *ServiceAdaptersTestSuite) TestErrorReportingServiceAdapter_UpdateConfiguration_NilInner() {
	adapter := &ErrorReportingServiceAdapter{Inner: nil}
	assert.Panics(suite.T(), func() {
		adapter.UpdateConfiguration(nil)
	})
}

func (suite *ServiceAdaptersTestSuite) TestErrorReportingServiceAdapter_GetConfiguration_NilInner() {
	adapter := &ErrorReportingServiceAdapter{Inner: nil}
	assert.Panics(suite.T(), func() {
		adapter.GetConfiguration()
	})
}

func (suite *ServiceAdaptersTestSuite) TestErrorReportingServiceAdapter_CleanupOldReports_NilInner() {
	adapter := &ErrorReportingServiceAdapter{Inner: nil}
	assert.Panics(suite.T(), func() {
		adapter.CleanupOldReports(time.Now().Add(-24 * time.Hour))
	})
}

func (suite *ServiceAdaptersTestSuite) TestErrorReportingServiceAdapter_ExportReports_NilInner() {
	adapter := &ErrorReportingServiceAdapter{Inner: nil}
	assert.Panics(suite.T(), func() {
		adapter.ExportReports(1, nil)
	})
}

// LogManagementServiceAdapter — additional delegation tests

func (suite *ServiceAdaptersTestSuite) TestLogManagementServiceAdapter_GetLogCollection_NilInner() {
	adapter := &LogManagementServiceAdapter{Inner: nil}
	assert.Panics(suite.T(), func() {
		adapter.GetLogCollection(1, 1)
	})
}

func (suite *ServiceAdaptersTestSuite) TestLogManagementServiceAdapter_GetLogCollectionsByUser_NilInner() {
	adapter := &LogManagementServiceAdapter{Inner: nil}
	assert.Panics(suite.T(), func() {
		adapter.GetLogCollectionsByUser(1, 10, 0)
	})
}

func (suite *ServiceAdaptersTestSuite) TestLogManagementServiceAdapter_GetLogEntries_NilInner() {
	adapter := &LogManagementServiceAdapter{Inner: nil}
	assert.Panics(suite.T(), func() {
		adapter.GetLogEntries(1, 1, nil)
	})
}

func (suite *ServiceAdaptersTestSuite) TestLogManagementServiceAdapter_CreateLogShare_NilInner() {
	adapter := &LogManagementServiceAdapter{Inner: nil}
	assert.Panics(suite.T(), func() {
		adapter.CreateLogShare(1, nil)
	})
}

func (suite *ServiceAdaptersTestSuite) TestLogManagementServiceAdapter_GetLogShare_NilInner() {
	adapter := &LogManagementServiceAdapter{Inner: nil}
	assert.Panics(suite.T(), func() {
		adapter.GetLogShare("token")
	})
}

func (suite *ServiceAdaptersTestSuite) TestLogManagementServiceAdapter_RevokeLogShare_NilInner() {
	adapter := &LogManagementServiceAdapter{Inner: nil}
	assert.Panics(suite.T(), func() {
		adapter.RevokeLogShare(1, 1)
	})
}

func (suite *ServiceAdaptersTestSuite) TestLogManagementServiceAdapter_ExportLogs_NilInner() {
	adapter := &LogManagementServiceAdapter{Inner: nil}
	assert.Panics(suite.T(), func() {
		adapter.ExportLogs(1, 1, "json")
	})
}

func (suite *ServiceAdaptersTestSuite) TestLogManagementServiceAdapter_StreamLogs_NilInner() {
	adapter := &LogManagementServiceAdapter{Inner: nil}
	assert.Panics(suite.T(), func() {
		adapter.StreamLogs(1, nil)
	})
}

func (suite *ServiceAdaptersTestSuite) TestLogManagementServiceAdapter_AnalyzeLogs_NilInner() {
	adapter := &LogManagementServiceAdapter{Inner: nil}
	assert.Panics(suite.T(), func() {
		adapter.AnalyzeLogs(1, 1)
	})
}

func (suite *ServiceAdaptersTestSuite) TestLogManagementServiceAdapter_GetLogStatistics_NilInner() {
	adapter := &LogManagementServiceAdapter{Inner: nil}
	assert.Panics(suite.T(), func() {
		adapter.GetLogStatistics(1)
	})
}

func (suite *ServiceAdaptersTestSuite) TestLogManagementServiceAdapter_GetConfiguration_NilInner() {
	adapter := &LogManagementServiceAdapter{Inner: nil}
	assert.Panics(suite.T(), func() {
		adapter.GetConfiguration()
	})
}

func (suite *ServiceAdaptersTestSuite) TestLogManagementServiceAdapter_UpdateConfiguration_NilInner() {
	adapter := &LogManagementServiceAdapter{Inner: nil}
	assert.Panics(suite.T(), func() {
		adapter.UpdateConfiguration(nil)
	})
}

func (suite *ServiceAdaptersTestSuite) TestLogManagementServiceAdapter_CleanupOldLogs_NilInner() {
	adapter := &LogManagementServiceAdapter{Inner: nil}
	assert.Panics(suite.T(), func() {
		adapter.CleanupOldLogs()
	})
}

func TestServiceAdaptersTestSuite(t *testing.T) {
	suite.Run(t, new(ServiceAdaptersTestSuite))
}
