package main

// ADMD1F1-S1: the live contract sweep decides whether an operation is an admin gate (so the non-admin leg must assert 403) from an
// explicit machine marker in docs/api/openapi.yaml, never from the wording of the 403 description. Replaces the regex classifier
// of ADMD1RRRRRRRRR..ADMD1RRRRRRRRRRR (five review rounds; its worst failure was a silent false negative).
//
//	operation-level vendor extension:  x-catalogizer-admin-gated: true
//
// gated  <=>  the marker is present and is the boolean true.
// adminGateSpecViolations is the drift guard: it reports every way the spec can be wrong about the marker, so a new admin route cannot
// be added unmarked silently and a marker cannot sit on an operation that has no documented 403.

import (
	"fmt"
	"regexp"
	"sort"
)

const adminGateMarker = "x-catalogizer-admin-gated"

// adminRoleText is the text of middleware.RequireAdmin's 403 ("Admin role required"); any 403 description that still mentions it on an
// operation without the marker is drift.
var adminRoleText = regexp.MustCompile(`(?i)admin\s+role\s+required`)

// adminGatedOp reports whether the operation object carries the marker with the boolean value true (anything else is not gated).
func adminGatedOp(op map[string]interface{}) bool {
	v, ok := op[adminGateMarker].(bool)
	return ok && v
}

// adminGateSpecViolations inspects a parsed OpenAPI document and returns sorted violations (empty means consistent).
func adminGateSpecViolations(doc map[string]interface{}) []string {
	var out []string
	paths, _ := doc["paths"].(map[string]interface{})
	for p, ops := range paths {
		opm, _ := ops.(map[string]interface{})
		for m, o := range opm {
			op, ok := o.(map[string]interface{})
			if !ok {
				continue
			}
			resp, _ := op["responses"].(map[string]interface{})
			r403, has403 := resp["403"].(map[string]interface{})
			if raw, present := op[adminGateMarker]; present {
				if _, isBool := raw.(bool); !isBool {
					out = append(out, fmt.Sprintf("%s %s: %s must be a boolean, got %T", m, p, adminGateMarker, raw))
				} else if adminGatedOp(op) && !has403 {
					out = append(out, fmt.Sprintf("%s %s: marked admin-gated but documents no 403 response", m, p))
				}
			}
			if !adminGatedOp(op) && has403 {
				if desc, _ := r403["description"].(string); adminRoleText.MatchString(desc) {
					out = append(out, fmt.Sprintf("%s %s: 403 description mentions the admin role body but the operation has no %s: true marker", m, p, adminGateMarker))
				}
			}
		}
	}
	sort.Strings(out)
	return out
}

// adminGatedSet returns the sorted "METHOD /path" list of marked operations.
func adminGatedSet(doc map[string]interface{}) []string {
	var out []string
	paths, _ := doc["paths"].(map[string]interface{})
	for p, ops := range paths {
		opm, _ := ops.(map[string]interface{})
		for m, o := range opm {
			if op, ok := o.(map[string]interface{}); ok && adminGatedOp(op) {
				out = append(out, m+" "+p)
			}
		}
	}
	sort.Strings(out)
	return out
}
