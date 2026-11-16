package handlers

// ADMD1RRRRRRR-m2: the real 409 bodies of POST /storage/roots (duplicate name with and without existing_id, private-network
// confirmation) validated against the oneOf of docs/api/openapi.yaml. There is no JSON-schema library in the module, so a
// small validator covers exactly the keywords the two schemas use ($ref, allOf, oneOf, not, type, required, enum). The
// bodies are produced by the production handler; only the spec is loaded from the repository file.

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"catalogizer/internal/netguard"
)

type oaDoc map[string]interface{}

func loadOpenAPI(t *testing.T) oaDoc {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "docs", "api", "openapi.yaml"))
	require.NoError(t, err)
	// The spec file currently repeats a "403" response key in two operations (pre-existing, not part of this item);
	// yaml.v3 refuses to decode a map with duplicate keys, so the node tree is converted by hand (last key wins).
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

// validate returns the list of violations of body against schema (empty = valid).
func (d oaDoc) validate(t *testing.T, schema map[string]interface{}, body map[string]interface{}) []string {
	t.Helper()
	var errs []string
	if ref, ok := schema["$ref"].(string); ok {
		return d.validate(t, d.schema(t, strings.TrimPrefix(ref, "#/components/schemas/")), body)
	}
	if all, ok := schema["allOf"].([]interface{}); ok {
		for _, sub := range all {
			errs = append(errs, d.validate(t, sub.(map[string]interface{}), body)...)
		}
	}
	if one, ok := schema["oneOf"].([]interface{}); ok {
		matches := 0
		for _, sub := range one {
			if len(d.validate(t, sub.(map[string]interface{}), body)) == 0 {
				matches++
			}
		}
		if matches != 1 {
			errs = append(errs, fmt.Sprintf("oneOf: %d branches match, want exactly 1", matches))
		}
	}
	if not, ok := schema["not"].(map[string]interface{}); ok {
		if len(d.validate(t, not, body)) == 0 {
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
			case "integer":
				if f, ok := v.(float64); !ok || f != float64(int64(f)) {
					errs = append(errs, k+": want integer")
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
