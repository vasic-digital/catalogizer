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
	"strings"
)

const adminGateMarker = "x-catalogizer-admin-gated"

// adminRoleText is the text of middleware.RequireAdmin's 403 ("Admin role required"); any 403 description that still mentions it on an
// operation without the marker is drift.
// ADMD1F1-S1 round 2: this wording regex is only a SECONDARY signal (it cannot cover every phrasing used by the 85 marked operations);
// the primary cross-check is the code-derived route table (admin_gate_route_table_test.go).
var adminRoleText = regexp.MustCompile(`(?i)admin\s+role\s+required|administrator\s+role\s+required|not\s+an\s+administrator|is\s+not\s+an\s+admin|lacks\s+` + "`?" + `(?:system\.|user\.|report\.)`)

// selfOrAdminWording marks RequireSelfOrAdmin descriptions ("... is not the caller (and the caller is not an admin)"): they mention
// administrators legitimately and are not admin-only gates, so the secondary wording signal skips them.
var selfOrAdminWording = regexp.MustCompile(`(?i)not the caller|\bself\b`)

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
		if _, present := opm[adminGateMarker]; present {
			out = append(out, fmt.Sprintf("%s: %s must be on an operation, not on the path item", p, adminGateMarker))
		}
		for m, o := range opm {
			op, ok := o.(map[string]interface{})
			if !ok {
				continue
			}
			resp, _ := op["responses"].(map[string]interface{})
			r403, has403 := resp["403"].(map[string]interface{})
			r403 = resolveResponseRef(doc, r403)
			if raw, present := op[adminGateMarker]; present {
				if _, isBool := raw.(bool); !isBool {
					out = append(out, fmt.Sprintf("%s %s: %s must be a boolean, got %T", m, p, adminGateMarker, raw))
				} else if adminGatedOp(op) && !has403 {
					out = append(out, fmt.Sprintf("%s %s: marked admin-gated but documents no 403 response", m, p))
				}
			}
			if !adminGatedOp(op) && has403 {
				if desc, _ := r403["description"].(string); adminRoleText.MatchString(desc) && !selfOrAdminWording.MatchString(desc) {
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

// resolveResponseRef follows a '#/components/responses/<name>' $ref so a referenced 403 is judged by its real description.
func resolveResponseRef(doc map[string]interface{}, r map[string]interface{}) map[string]interface{} {
	for i := 0; i < 5; i++ {
		ref, _ := r["$ref"].(string)
		const pre = "#/components/responses/"
		if !strings.HasPrefix(ref, pre) {
			return r
		}
		comps, _ := doc["components"].(map[string]interface{})
		resps, _ := comps["responses"].(map[string]interface{})
		next, ok := resps[strings.TrimPrefix(ref, pre)].(map[string]interface{})
		if !ok {
			return r
		}
		r = next
	}
	return r
}

// adminGateLoaderViolations is what the live loader enforces for ANY spec (also API_CONTRACT_SPEC overrides): a marker that is not a
// boolean, or one placed on the path item, is a loud failure instead of a silently ungated route.
func adminGateLoaderViolations(path string, item map[string]interface{}) []string {
	var out []string
	if _, present := item[adminGateMarker]; present {
		out = append(out, fmt.Sprintf("%s: %s must be on an operation, not on the path item", path, adminGateMarker))
	}
	for m, o := range item {
		if op, ok := o.(map[string]interface{}); ok {
			if raw, present := op[adminGateMarker]; present {
				if _, isBool := raw.(bool); !isBool {
					out = append(out, fmt.Sprintf("%s %s: %s must be a boolean, got %T", m, path, adminGateMarker, raw))
				}
			}
		}
	}
	return out
}
