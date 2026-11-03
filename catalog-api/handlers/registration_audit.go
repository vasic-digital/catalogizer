package handlers

// Registration decisions are security-relevant: they decide who may log in. They go to the same
// audit trail as logins (auth_audit_log, written by the production AuthService).
//
// Event types: registration_created (details status=pending|active mode=...),
// registration_approved / registration_rejected (a pending self-registration decided by an
// administrator; details actor=<admin id>), admin_user_status_changed (any other status change).
// Details never carry a login name, e-mail address or password.

type authAuditor interface {
	RecordAuditEvent(userID int, eventType, ipAddress, userAgent, details string)
}

// recordAuth writes an audit event through svc when it supports it (the production service does;
// services without an audit sink ignore the call).
func recordAuth(svc interface{}, userID int, eventType, ip, ua, details string) {
	if a, ok := svc.(authAuditor); ok {
		a.RecordAuditEvent(userID, eventType, ip, ua, details)
	}
}
