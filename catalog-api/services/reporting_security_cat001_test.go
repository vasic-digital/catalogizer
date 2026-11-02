package services

import (
	"testing"
	"time"

	"catalogizer/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const authAuditDDL = `CREATE TABLE IF NOT EXISTS auth_audit_log (
	id INTEGER PRIMARY KEY AUTOINCREMENT, user_id INTEGER, event_type TEXT NOT NULL,
	ip_address TEXT, user_agent TEXT, details TEXT, created_at DATETIME DEFAULT CURRENT_TIMESTAMP)`

func seedAudit(t *testing.T, s *ReportingService, event string, at time.Time, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		_, err := s.db.Exec(`INSERT INTO auth_audit_log (user_id, event_type, created_at) VALUES (0, ?, ?)`,
			event, at.UTC().Format("2006-01-02 15:04:05"))
		require.NoError(t, err)
	}
}

// CAT-001: generateSecurityAuditData hardcoded FailedLoginAttempts/SuccessfulLogins
// to 0 ("Would be calculated from actual data"). The real events live in
// auth_audit_log (written by internal/auth logAuthEvent).
func TestCAT001_SecurityAudit_CountsRealAuthAuditEvents(t *testing.T) {
	db := setupTestDB(t)
	_, err := db.Exec(authAuditDDL)
	require.NoError(t, err)
	s := NewReportingService(nil, nil, db)

	in := time.Date(2025, 1, 10, 12, 0, 0, 0, time.UTC)
	seedAudit(t, s, "failed_login", in, 3)
	seedAudit(t, s, "failed_login_inactive", in, 1)
	seedAudit(t, s, "login_success", in, 2)
	seedAudit(t, s, "logout", in, 5)                          // neither
	seedAudit(t, s, "failed_login", in.AddDate(0, -2, 0), 7)  // outside the range
	seedAudit(t, s, "login_success", in.AddDate(0, 0, 40), 9) // outside the range

	got, err := s.generateSecurityAuditData(map[string]interface{}{"start_date": "2025-01-01", "end_date": "2025-01-31"})
	require.NoError(t, err)
	rep := got.(*models.SecurityAuditReport)
	assert.Equal(t, 4, rep.FailedLoginAttempts)
	assert.Equal(t, 2, rep.SuccessfulLogins)
	assert.Equal(t, 98.0, rep.SecurityMetrics.SecurityScore, "100 - 4 failed logins * 0.5")
}

func TestCAT001_SecurityAudit_UnreadableAuditTableIsAnErrorNotZero(t *testing.T) {
	db := setupTestDB(t) // no auth_audit_log table
	s := NewReportingService(nil, nil, db)
	_, err := s.generateSecurityAuditData(map[string]interface{}{"start_date": "2025-01-01", "end_date": "2025-01-31"})
	require.Error(t, err, "an unreadable audit trail must not be reported as zero failed logins")
	assert.Contains(t, err.Error(), "auth_audit_log")
}

func TestCAT001_SecurityMetrics_UnmeasurableIsUnknownNotLowThreat(t *testing.T) {
	s := NewReportingService(nil, nil, nil)
	m := s.calculateSecurityMetrics(time.Now().Add(-time.Hour), time.Now())
	assert.Equal(t, "unknown", m.ThreatLevel)
	assert.Equal(t, 0.0, m.SecurityScore)
	assert.Contains(t, m.Unavailable, "failed_logins")
	assert.Contains(t, m.Unavailable, "vulnerability_count", "system_alerts has no producer anywhere in the codebase")
}
