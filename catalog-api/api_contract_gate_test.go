package main

// ADMD1RRRRRRRRR-m1: the classifier the live contract sweep uses to decide whether a documented 403 is an admin gate (so the
// non-admin leg must assert it). It lives outside the live build tag so a default `go test` exercises it against the real spec.

import (
	"regexp"
	"strings"
)

var adminGate403 = regexp.MustCompile(`(?i)admin|lacks|system\.configure|user\.manage|report\.`)
var nonGate403 = regexp.MustCompile(`(?i)registration|pending|owner|not the caller|neither|visible but|share|job|domain not allowed|another user|session`)

// adminRoleBody is the exact body string of middleware.RequireAdmin's 403 (auth.go): a description that quotes it as JSON documents
// an admin gate even when it also mentions an owner or a session (merged 403s of the /sync GET {id} operations).
const adminRoleBody = `"error":"Admin role required"`

func admin403Gated(desc string) bool {
	if strings.Contains(desc, adminRoleBody) {
		return true
	}
	return adminGate403.MatchString(desc) && !nonGate403.MatchString(desc)
}
