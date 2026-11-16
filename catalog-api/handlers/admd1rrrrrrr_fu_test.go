package handlers

// ADMD1RRRRRRR-m2: the real 409 bodies of POST /storage/roots (duplicate name with and without existing_id, private-network
// confirmation) validated against the oneOf of docs/api/openapi.yaml. There is no JSON-schema library in the module, so a
// small validator covers exactly the keywords the two schemas use ($ref, allOf, oneOf, not, type, required, enum). The
// bodies are produced by the production handler; only the spec is loaded from the repository file.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"catalogizer/internal/netguard"
	"catalogizer/middleware"
	"catalogizer/models"
)

type oaDoc map[string]interface{}

func loadOpenAPI(t *testing.T) oaDoc {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "docs", "api", "openapi.yaml"))
	require.NoError(t, err)
	// The node tree is converted by hand; duplicate mapping keys in the spec are policed by TestADMD1RRRRRRRRfu_OpenAPIHasNoDuplicateKeys.
	var n yaml.Node
	require.NoError(t, yaml.Unmarshal(raw, &n))
	d, ok := nodeToIface(&n).(map[string]interface{})
	require.True(t, ok)
	return oaDoc(d)
}

func nodeToIface(n *yaml.Node) interface{} {
	switch n.Kind {
	case yaml.DocumentNode:
		return nodeToIface(n.Content[0])
	case yaml.MappingNode:
		m := map[string]interface{}{}
		for i := 0; i+1 < len(n.Content); i += 2 {
			m[n.Content[i].Value] = nodeToIface(n.Content[i+1])
		}
		return m
	case yaml.SequenceNode:
		out := make([]interface{}, 0, len(n.Content))
		for _, c := range n.Content {
			out = append(out, nodeToIface(c))
		}
		return out
	default:
		var v interface{}
		if err := n.Decode(&v); err != nil {
			return n.Value
		}
		return v
	}
}

func (d oaDoc) schema(t *testing.T, name string) map[string]interface{} {
	t.Helper()
	comps, _ := d["components"].(map[string]interface{})
	schemas, _ := comps["schemas"].(map[string]interface{})
	s, ok := schemas[name].(map[string]interface{})
	require.True(t, ok, "schema %s missing in openapi.yaml", name)
	return s
}

// ADMD1RRRRRRRR-n1: keywords the validator knows. Anything else in a schema it walks fails the test loudly instead of being
// silently skipped, so "the contract is checked" cannot quietly mean "only the keywords I happened to implement".
var (
	oaSchemaEnforced = map[string]bool{"$ref": true, "allOf": true, "oneOf": true, "not": true, "required": true, "properties": true}
	// intentionally ignored: documentation-only, or a type the body (a decoded JSON object) satisfies by construction
	oaSchemaIgnored = map[string]bool{"description": true, "example": true, "title": true, "type": true}
	oaPropEnforced  = map[string]bool{"enum": true, "format": true} // format: only integer int32/int64 (checked by unsupported + validateInner)
	oaPropIgnored   = map[string]bool{"description": true, "example": true, "type": true}
	oaPropTypes     = map[string]bool{"string": true, "integer": true, "boolean": true}
)

// unsupported lists every keyword (and property type) under schema that validateInner neither enforces nor intentionally ignores.
func (d oaDoc) unsupported(t *testing.T, schema map[string]interface{}, where string, seen map[string]bool) []string {
	t.Helper()
	var out []string
	for k := range schema {
		if !oaSchemaEnforced[k] && !oaSchemaIgnored[k] {
			out = append(out, fmt.Sprintf("%s: unsupported schema keyword %q", where, k))
		}
	}
	if rawTy, has := schema["type"]; has {
		if ty, isStr := rawTy.(string); !isStr {
			out = append(out, fmt.Sprintf("%s: unsupported non-string top-level type %v (only object bodies are validated)", where, rawTy))
		} else if ty != "object" {
			out = append(out, fmt.Sprintf("%s: unsupported top-level type %q (only object bodies are validated)", where, ty))
		}
	}
	if ref, ok := schema["$ref"].(string); ok && !seen[ref] {
		seen[ref] = true
		out = append(out, d.unsupported(t, d.schema(t, strings.TrimPrefix(ref, "#/components/schemas/")), ref, seen)...)
	}
	for _, key := range []string{"allOf", "oneOf"} {
		if subs, ok := schema[key].([]interface{}); ok {
			for i, sub := range subs {
				out = append(out, d.unsupported(t, sub.(map[string]interface{}), fmt.Sprintf("%s/%s[%d]", where, key, i), seen)...)
			}
		}
	}
	if not, ok := schema["not"].(map[string]interface{}); ok {
		out = append(out, d.unsupported(t, not, where+"/not", seen)...)
	}
	if props, ok := schema["properties"].(map[string]interface{}); ok {
		for name, ps := range props {
			p, isMap := ps.(map[string]interface{})
			if !isMap {
				out = append(out, fmt.Sprintf("%s.%s: unsupported non-map property schema %T", where, name, ps))
				continue
			}
			for k := range p {
				if !oaPropEnforced[k] && !oaPropIgnored[k] {
					out = append(out, fmt.Sprintf("%s.%s: unsupported property keyword %q", where, name, k))
				}
			}
			if f, has := p["format"]; has {
				fs, _ := f.(string)
				if p["type"] != "integer" || (fs != "int32" && fs != "int64") {
					out = append(out, fmt.Sprintf("%s.%s: unsupported format %v on type %v (only integer int32/int64 is enforced)", where, name, f, p["type"]))
				}
			}
			if rawTy, has := p["type"]; has {
				if ty, isStr := rawTy.(string); !isStr {
					out = append(out, fmt.Sprintf("%s.%s: unsupported non-string property type %v", where, name, rawTy))
				} else if !oaPropTypes[ty] {
					out = append(out, fmt.Sprintf("%s.%s: unsupported property type %q", where, name, ty))
				}
			}
		}
	}
	return out
}

// validate returns the violations of body against schema (empty = valid); an unsupported keyword is reported as a violation, so every
// require.Empty(validate(...)) call site fails loudly instead of skipping it.
func (d oaDoc) validate(t *testing.T, schema map[string]interface{}, body map[string]interface{}) []string {
	t.Helper()
	if u := d.unsupported(t, schema, "schema", map[string]bool{}); len(u) > 0 {
		return []string{"unsupported keywords: " + strings.Join(u, "; ")}
	}
	return d.validateInner(t, schema, body)
}

// validateInner returns the list of violations of body against schema (empty = valid); keyword coverage is policed by validate.
func (d oaDoc) validateInner(t *testing.T, schema map[string]interface{}, body map[string]interface{}) []string {
	t.Helper()
	var errs []string
	if ref, ok := schema["$ref"].(string); ok {
		return d.validateInner(t, d.schema(t, strings.TrimPrefix(ref, "#/components/schemas/")), body)
	}
	if all, ok := schema["allOf"].([]interface{}); ok {
		for _, sub := range all {
			errs = append(errs, d.validateInner(t, sub.(map[string]interface{}), body)...)
		}
	}
	if one, ok := schema["oneOf"].([]interface{}); ok {
		matches := 0
		for _, sub := range one {
			if len(d.validateInner(t, sub.(map[string]interface{}), body)) == 0 {
				matches++
			}
		}
		if matches != 1 {
			errs = append(errs, fmt.Sprintf("oneOf: %d branches match, want exactly 1", matches))
		}
	}
	if not, ok := schema["not"].(map[string]interface{}); ok {
		if len(d.validateInner(t, not, body)) == 0 {
			errs = append(errs, "not: body matches the forbidden schema")
		}
	}
	if req, ok := schema["required"].([]interface{}); ok {
		for _, r := range req {
			if _, has := body[r.(string)]; !has {
				errs = append(errs, "missing required "+r.(string))
			}
		}
	}
	if props, ok := schema["properties"].(map[string]interface{}); ok {
		for k, ps := range props {
			v, has := body[k]
			if !has {
				continue
			}
			p := ps.(map[string]interface{})
			switch p["type"] {
			case "string":
				if _, ok := v.(string); !ok {
					errs = append(errs, k+": want string")
				}
			case "boolean":
				if _, ok := v.(bool); !ok {
					errs = append(errs, k+": want boolean")
				}
			case "integer":
				if f, ok := v.(float64); !ok || f != float64(int64(f)) {
					errs = append(errs, k+": want integer")
				} else if p["format"] == "int32" && (f > 2147483647 || f < -2147483648) {
					errs = append(errs, k+": out of int32 range")
				}
			}
			if en, ok := p["enum"].([]interface{}); ok {
				found := false
				for _, e := range en {
					if e == v {
						found = true
					}
				}
				if !found {
					errs = append(errs, fmt.Sprintf("%s: %v not in enum %v", k, v, en))
				}
			}
		}
	}
	return errs
}

// the 409 response schema of POST /storage/roots, found by path so a moved schema fails loudly.
func (d oaDoc) createRoots409(t *testing.T) map[string]interface{} {
	t.Helper()
	paths := d["paths"].(map[string]interface{})
	post := paths["/api/v1/storage/roots"]
	if post == nil {
		post = paths["/storage/roots"]
	}
	require.NotNil(t, post, "POST /storage/roots path missing in openapi.yaml")
	r := post.(map[string]interface{})["post"].(map[string]interface{})["responses"].(map[string]interface{})["409"].(map[string]interface{})
	return r["content"].(map[string]interface{})["application/json"].(map[string]interface{})["schema"].(map[string]interface{})
}

func TestADMD1RRRRRRRfu_StorageRoot409BodiesMatchOpenAPIOneOf(t *testing.T) {
	t.Setenv(netguard.AllowCIDRsEnv, "")
	d := loadOpenAPI(t)
	one := d.createRoots409(t)
	require.NotNil(t, one["oneOf"], "the 409 schema must be a oneOf")

	h := secq1cHarness(t)
	code, first := h.do("POST", "/storage/roots", map[string]interface{}{"name": "pub", "protocol": "webdav", "url": "https://" + secq1cPublicIP + "/dav", "path": "/"})
	require.Equal(t, http.StatusCreated, code, "%v", first)

	// duplicate with existing_id (sequential pre-check path)
	code, dup := h.do("POST", "/storage/roots", map[string]interface{}{"name": "pub", "protocol": "webdav", "url": "https://" + secq1cPublicIP + "/dav", "path": "/"})
	require.Equal(t, http.StatusConflict, code, "%v", dup)
	require.Contains(t, dup, "existing_id")
	require.Empty(t, d.validate(t, one, dup), "duplicate body %v", dup)
	require.Empty(t, d.validate(t, d.schema(t, "StorageRootConflict"), dup))
	require.NotEmpty(t, d.validate(t, d.schema(t, "StorageRootPrivateConfirmation"), dup), "duplicate must not match the private-network branch")

	// duplicate without existing_id (winner lookup failed after the race)
	storageRootBeforeInsertHook = func(ctx context.Context, name string, _ error) {
		_, err := h.db.InsertReturningID(ctx, `INSERT INTO storage_roots (name, protocol, path, enabled, max_depth) VALUES (?, 'local', '/', ?, 10)`, name, true)
		require.NoError(t, err)
	}
	storageRootAfterConflictHook = func(ctx context.Context, name string) {
		_, err := h.db.ExecContext(ctx, "DELETE FROM storage_roots WHERE name = ?", name)
		require.NoError(t, err)
	}
	t.Cleanup(func() { storageRootBeforeInsertHook = nil; storageRootAfterConflictHook = nil })
	code, noID := h.do("POST", "/storage/roots", map[string]interface{}{"name": "lost", "protocol": "webdav", "url": "https://" + secq1cPublicIP + "/dav", "path": "/"})
	require.Equal(t, http.StatusConflict, code, "%v", noID)
	require.NotContains(t, noID, "existing_id")
	require.Empty(t, d.validate(t, one, noID), "no-id duplicate body %v", noID)

	storageRootBeforeInsertHook, storageRootAfterConflictHook = nil, nil

	// private-network confirmation
	code, priv := h.do("POST", "/storage/roots", map[string]interface{}{"name": "lan", "protocol": "webdav", "url": "http://192.168.1.20:5005/", "path": "/"})
	require.Equal(t, http.StatusConflict, code, "%v", priv)
	require.Equal(t, secq1cNoConfirm, priv["code"])
	require.Empty(t, d.validate(t, one, priv), "private-network body %v", priv)
	require.NotEmpty(t, d.validate(t, d.schema(t, "StorageRootConflict"), priv), "private-network body must not match the duplicate branch")

	// negative controls against the validator itself: a body that matches neither, and one that matches both
	require.NotEmpty(t, d.validate(t, one, map[string]interface{}{"error": "x"}), "body with neither name nor code must fail")
	both := map[string]interface{}{"error": "x", "name": "n", "hint": "h", "code": secq1cNoConfirm}
	require.NotEmpty(t, d.validate(t, d.schema(t, "StorageRootConflict"), both), "n1: a body carrying code must be excluded from the duplicate branch")
	require.Empty(t, d.validate(t, one, both), "name+code+hint is a valid private-network body (exactly one branch)")
	bothBadCode := map[string]interface{}{"error": "x", "name": "n", "hint": "h", "code": "other"}
	require.NotEmpty(t, d.validate(t, one, bothBadCode), "a body with name and an unknown code matches neither branch")
	badCode := map[string]interface{}{"error": "x", "hint": "h", "code": "something_else"}
	require.NotEmpty(t, d.validate(t, one, badCode), "an unknown code must fail the enum")
}

// ADMD1RRRRRRRR-m1: oneOf exclusivity is exercised by a synthetic overlapping schema, driving the real validator.
func TestADMD1RRRRRRRRfu_ValidatorOneOfExclusivity(t *testing.T) {
	d := oaDoc{}
	overlap := map[string]interface{}{"oneOf": []interface{}{
		map[string]interface{}{"required": []interface{}{"a"}},
		map[string]interface{}{"required": []interface{}{"b"}},
	}}
	errs := d.validate(t, overlap, map[string]interface{}{"a": 1, "b": 2})
	require.Len(t, errs, 1)
	require.Contains(t, errs[0], "2 branches match")
	errs = d.validate(t, overlap, map[string]interface{}{"c": 1})
	require.Len(t, errs, 1)
	require.Contains(t, errs[0], "0 branches match")
	require.Empty(t, d.validate(t, overlap, map[string]interface{}{"a": 1}), "exactly one branch matches")
}

// ADMD1RRRRRRRR-n1: the guard fires on every kind of unsupported keyword and stays quiet on the two real 409 schemas.
func TestADMD1RRRRRRRRfu_ValidatorUnknownKeywordGuard(t *testing.T) {
	d := loadOpenAPI(t)
	for _, name := range []string{"StorageRootConflict", "StorageRootPrivateConfirmation"} {
		require.Empty(t, d.unsupported(t, d.schema(t, name), name, map[string]bool{}), "the real %s schema must use only supported keywords", name)
	}
	mk := func(schema map[string]interface{}) []string { return d.unsupported(t, schema, "s", map[string]bool{}) }
	require.NotEmpty(t, mk(map[string]interface{}{"additionalProperties": false}), "unknown schema keyword")
	require.NotEmpty(t, mk(map[string]interface{}{"allOf": []interface{}{map[string]interface{}{"pattern": "x"}}}), "unknown keyword nested in allOf")
	require.NotEmpty(t, mk(map[string]interface{}{"oneOf": []interface{}{map[string]interface{}{"minProperties": 1}}}), "unknown keyword nested in oneOf")
	require.NotEmpty(t, mk(map[string]interface{}{"not": map[string]interface{}{"anyOf": []interface{}{}}}), "unknown keyword nested in not")
	require.NotEmpty(t, mk(map[string]interface{}{"properties": map[string]interface{}{"p": map[string]interface{}{"type": "string", "minLength": 1}}}), "unknown property keyword")
	require.NotEmpty(t, mk(map[string]interface{}{"properties": map[string]interface{}{"p": map[string]interface{}{"type": "array"}}}), "unsupported property type")
	require.NotEmpty(t, mk(map[string]interface{}{"type": "array"}), "unsupported top-level type")
	// ADMD1RRRRRRRRR-n1: array-valued type (3.1 style), non-map property schema and format are no longer silently passed
	require.NotEmpty(t, mk(map[string]interface{}{"properties": map[string]interface{}{"p": map[string]interface{}{"type": []interface{}{"string", "null"}}}}), "array-valued property type")
	require.NotEmpty(t, mk(map[string]interface{}{"type": []interface{}{"object", "null"}}), "array-valued top-level type")
	require.NotEmpty(t, mk(map[string]interface{}{"properties": map[string]interface{}{"p": "string"}}), "non-map property schema")
	require.NotEmpty(t, mk(map[string]interface{}{"properties": map[string]interface{}{"p": nil}}), "nil property schema")
	require.NotEmpty(t, mk(map[string]interface{}{"properties": map[string]interface{}{"p": map[string]interface{}{"type": "string", "format": "uuid"}}}), "property format on a string type")
	require.NotEmpty(t, mk(map[string]interface{}{"properties": map[string]interface{}{"p": map[string]interface{}{"type": "integer", "format": "float"}}}), "integer with an unknown format")
	require.Empty(t, mk(map[string]interface{}{"properties": map[string]interface{}{"p": map[string]interface{}{"type": "integer", "format": "int64"}}}), "negative control: integer int64 is enforced")
	require.NotEmpty(t, d.validateInner(t, map[string]interface{}{"properties": map[string]interface{}{"p": map[string]interface{}{"type": "integer", "format": "int32"}}}, map[string]interface{}{"p": float64(1 << 40)}), "int32 range enforced")
	require.Empty(t, mk(map[string]interface{}{"type": "object", "properties": map[string]interface{}{"p": map[string]interface{}{"type": "string", "description": "d"}}}), "negative control: a supported schema stays quiet")
	// the guard reaches through $ref
	d2 := oaDoc{"components": map[string]interface{}{"schemas": map[string]interface{}{"X": map[string]interface{}{"format": "uuid"}}}}
	require.NotEmpty(t, d2.unsupported(t, map[string]interface{}{"$ref": "#/components/schemas/X"}, "s", map[string]bool{}))
	// validate itself surfaces the guard (not only the helper)
	ug := d.validate(t, map[string]interface{}{"additionalProperties": false}, map[string]interface{}{})
	require.Len(t, ug, 1)
	require.Contains(t, ug[0], "unsupported keywords")
	// boolean property type is enforced
	require.NotEmpty(t, d.validateInner(t, map[string]interface{}{"properties": map[string]interface{}{"p": map[string]interface{}{"type": "boolean"}}}, map[string]interface{}{"p": "yes"}))
}

// ADMD1RRRRRRRR-n2: openapi.yaml must not repeat a mapping key (yaml.v3 and most tooling refuse such a document).
func oaDuplicateKeys(n *yaml.Node, path string) []string {
	var out []string
	switch n.Kind {
	case yaml.DocumentNode, yaml.SequenceNode:
		for i, c := range n.Content {
			out = append(out, oaDuplicateKeys(c, fmt.Sprintf("%s[%d]", path, i))...)
		}
	case yaml.MappingNode:
		seen := map[string]int{}
		for i := 0; i+1 < len(n.Content); i += 2 {
			k := n.Content[i]
			if first, dup := seen[k.Value]; dup {
				out = append(out, fmt.Sprintf("%s: duplicate key %q at lines %d and %d", path, k.Value, first, k.Line))
			}
			seen[k.Value] = k.Line
			out = append(out, oaDuplicateKeys(n.Content[i+1], path+"/"+k.Value)...)
		}
	}
	return out
}

func TestADMD1RRRRRRRRfu_OpenAPIHasNoDuplicateKeys(t *testing.T) {
	var ctl yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte("a:\n  '403': x\n  '403': y\n"), &ctl))
	require.Len(t, oaDuplicateKeys(&ctl, ""), 1, "negative control: the detector must see a repeated key")

	raw, err := os.ReadFile(filepath.Join("..", "..", "docs", "api", "openapi.yaml"))
	require.NoError(t, err)
	var n yaml.Node
	require.NoError(t, yaml.Unmarshal(raw, &n))
	require.Empty(t, oaDuplicateKeys(&n, ""))
	var strict map[string]interface{}
	require.NoError(t, yaml.Unmarshal(raw, &strict), "yaml.v3 must decode the spec into a map")
}

// ADMD1RRRRRRRRR-n2/m2: pins the content and schema of the merged 403 on GET /sync/endpoints/{id} and /sync/sessions/{id}: both branches,
// the exact body strings the REAL middleware and handler emit, the permission the service checks, and the Error schema reference.
func TestADMD1RRRRRRRRR_SyncMerged403ContentPinnedToRealBodies(t *testing.T) {
	gin.SetMode(gin.TestMode)
	// real admin-gate body (RequireAdmin without a live checker: role_id 2 is refused)
	jm := middleware.NewJWTMiddleware("k")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/x", nil)
	c.Set("role_id", 2)
	jm.RequireAdmin()(c)
	require.Equal(t, http.StatusForbidden, w.Code)
	var adminBody, ownerBody map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &adminBody))
	// real owner-branch body (syncFail with the service's "unauthorized" error)
	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = httptest.NewRequest("GET", "/x", nil)
	syncFail(c2, http.StatusForbidden, "Failed to get sync endpoint", fmt.Errorf("unauthorized to view this endpoint"))
	require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &ownerBody))
	require.Equal(t, "Admin role required", adminBody["error"])
	require.Equal(t, "not permitted", ownerBody["details"])

	d := loadOpenAPI(t)
	paths := d["paths"].(map[string]interface{})
	for _, tc := range []struct{ path, owner, msg string }{
		{"/api/v1/sync/endpoints/{id}", "endpoint owner", "Failed to get sync endpoint"},
		{"/api/v1/sync/sessions/{id}", "session owner", "Failed to get sync session"},
	} {
		r := paths[tc.path].(map[string]interface{})["get"].(map[string]interface{})["responses"].(map[string]interface{})["403"].(map[string]interface{})
		desc := r["description"].(string)
		require.Contains(t, desc, `"error":"`+adminBody["error"].(string)+`"`, "%s: admin branch body", tc.path)
		require.Contains(t, desc, `"error":"`+tc.msg+`"`, "%s: owner branch body", tc.path)
		require.Contains(t, desc, `"details":"`+ownerBody["details"].(string)+`"`, "%s: owner branch details", tc.path)
		require.Contains(t, desc, "not an administrator", tc.path)
		require.Contains(t, desc, "not the "+tc.owner, tc.path)
		require.Contains(t, desc, "view-shares permission", tc.path)
		require.Contains(t, desc, models.PermissionViewShares, tc.path)
		sch := r["content"].(map[string]interface{})["application/json"].(map[string]interface{})["schema"].(map[string]interface{})
		require.Equal(t, "#/components/schemas/Error", sch["$ref"], tc.path)
	}
}
