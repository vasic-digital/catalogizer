//go:build live

package main

// LIVE OpenAPI contract sweep (§11.4.289 LIVE-class, deterministic per §11.4.50).
//
// System under test: the REAL catalog-api binary built from this tree (buildBinary), running a REAL SQLCipher
// SQLite file created by the PRODUCTION migrations, on an ephemeral loopback port, with random runtime
// secrets (JWT secret, admin password) that are never printed. No mock, stub, fixture server or httptest
// recorder is in the path. Fixture INPUTS (a few small files for the local storage root, request bodies
// generated from the spec) are fed to the real system.
//
// For EVERY operation in docs/api/openapi.yaml the sweep drives (as applicable):
//   unauth      no token                         -> 401 (secured operations)
//   nonadmin    self-registered plain user       -> 403 (operations whose spec 403 says admin / lacks permission)
//   notfound    unknown id in every path param   -> 404 (operations with path params that document 404)
//   badid       non-numeric id for integer param -> 400 or 404, never 2xx / 5xx
//   malformed   malformed JSON body              -> 400 (operations with a request body)
//   missingq    required query param omitted     -> 400
//   happy       valid request as admin           -> a documented status; a 2xx JSON body is validated against the schema
// and records per operation: method, path, status codes observed, schema verdict, verdicts, plus the binary
// sha256, endpoint and pid. The whole sweep runs API_CONTRACT_RUNS times (default 3) against fresh servers
// and fresh databases; the status/verdict signature of all runs must be identical (N=3).
//
// Run: scripts/test-infra/api_contract_live.sh   (or `go test -tags live -run TestLiveAPIContract ./catalog-api`).

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// ---------- spec model ----------

type cParam struct {
	Name, In string
	Required bool
	Schema   interface{}
}

type cOp struct {
	Method, Path string
	Public       bool
	Responses    map[string]interface{}
	Params       []cParam
	Body         interface{} // JSON request schema or nil
	BodyRequired bool
	Summary      string
}

func (o *cOp) key() string { return o.Method + " " + o.Path }

type cSpec struct {
	Root map[string]interface{}
	Ops  []*cOp
}

func normYAML(v interface{}) interface{} {
	switch x := v.(type) {
	case map[string]interface{}:
		for k, e := range x {
			x[k] = normYAML(e)
		}
		return x
	case map[interface{}]interface{}:
		m := map[string]interface{}{}
		for k, e := range x {
			m[fmt.Sprint(k)] = normYAML(e)
		}
		return m
	case []interface{}:
		for i := range x {
			x[i] = normYAML(x[i])
		}
		return x
	}
	return v
}

func asMap(v interface{}) map[string]interface{} {
	m, _ := v.(map[string]interface{})
	return m
}

func (s *cSpec) resolve(v interface{}) interface{} {
	for i := 0; i < 20; i++ {
		m := asMap(v)
		ref, _ := m["$ref"].(string)
		if m == nil || ref == "" {
			return v
		}
		cur := interface{}(s.Root)
		for _, seg := range strings.Split(strings.TrimPrefix(ref, "#/"), "/") {
			cur = asMap(cur)[seg]
		}
		v = cur
	}
	return v
}

func loadSpec(t *testing.T, path string) *cSpec {
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	var root map[string]interface{}
	require.NoError(t, yaml.Unmarshal(raw, &root), "the OpenAPI document must be valid YAML")
	s := &cSpec{Root: normYAML(root).(map[string]interface{})}
	global := s.Root["security"]
	paths := asMap(s.Root["paths"])
	pnames := make([]string, 0, len(paths))
	for p := range paths {
		pnames = append(pnames, p)
	}
	sort.Strings(pnames)
	for _, p := range pnames {
		item := asMap(paths[p])
		var common []cParam
		if pl, ok := item["parameters"].([]interface{}); ok {
			common = parseParams(s, pl)
		}
		for _, m := range []string{"get", "post", "put", "patch", "delete", "head", "options"} {
			od := asMap(item[m])
			if od == nil {
				continue
			}
			op := &cOp{Method: strings.ToUpper(m), Path: p, Responses: asMap(od["responses"]), Summary: fmt.Sprint(od["summary"])}
			sec, has := od["security"]
			if !has {
				sec = global
			}
			if sl, ok := sec.([]interface{}); !ok || len(sl) == 0 {
				op.Public = true
			}
			op.Params = append(op.Params, common...)
			if pl, ok := od["parameters"].([]interface{}); ok {
				op.Params = append(op.Params, parseParams(s, pl)...)
			}
			if rb := asMap(s.resolve(od["requestBody"])); rb != nil {
				op.BodyRequired, _ = rb["required"].(bool)
				if js := asMap(asMap(rb["content"])["application/json"]); js != nil {
					op.Body = js["schema"]
					if op.Body == nil {
						op.Body = map[string]interface{}{"type": "object"}
					}
				}
			}
			s.Ops = append(s.Ops, op)
		}
	}
	return s
}

func parseParams(s *cSpec, pl []interface{}) []cParam {
	var out []cParam
	for _, e := range pl {
		pm := asMap(s.resolve(e))
		req, _ := pm["required"].(bool)
		out = append(out, cParam{Name: fmt.Sprint(pm["name"]), In: fmt.Sprint(pm["in"]), Required: req, Schema: pm["schema"]})
	}
	return out
}

// ---------- minimal OpenAPI 3.0 schema validator ----------

func (s *cSpec) validate(schema, val interface{}, at string, out *[]string) {
	schema = s.resolve(schema)
	sm := asMap(schema)
	if sm == nil {
		return
	}
	if val == nil {
		if nb, _ := sm["nullable"].(bool); nb {
			return
		}
		if ao, ok := sm["allOf"].([]interface{}); ok {
			for _, sub := range ao {
				s.validate(sub, val, at, out)
			}
		}
		if _, hasType := sm["type"]; hasType {
			*out = append(*out, at+": null where a non-nullable "+fmt.Sprint(sm["type"])+" is documented")
		}
		return
	}
	if ao, ok := sm["allOf"].([]interface{}); ok {
		for _, sub := range ao {
			s.validate(sub, val, at, out)
		}
	}
	if en, ok := sm["enum"].([]interface{}); ok {
		found := false
		for _, e := range en {
			if fmt.Sprint(e) == fmt.Sprint(val) {
				found = true
			}
		}
		if !found {
			*out = append(*out, fmt.Sprintf("%s: value %v not in enum %v", at, val, en))
		}
	}
	typ, _ := sm["type"].(string)
	switch typ {
	case "object":
		obj, ok := val.(map[string]interface{})
		if !ok {
			*out = append(*out, fmt.Sprintf("%s: expected object, got %T", at, val))
			return
		}
		if req, ok := sm["required"].([]interface{}); ok {
			for _, r := range req {
				if _, present := obj[fmt.Sprint(r)]; !present {
					*out = append(*out, fmt.Sprintf("%s: required property %q missing", at, r))
				}
			}
		}
		props := asMap(sm["properties"])
		keys := make([]string, 0, len(obj))
		for k := range obj {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if ps, ok := props[k]; ok {
				s.validate(ps, obj[k], at+"."+k, out)
			} else if ap := asMap(sm["additionalProperties"]); ap != nil {
				s.validate(ap, obj[k], at+"."+k, out)
			}
		}
	case "array":
		arr, ok := val.([]interface{})
		if !ok {
			*out = append(*out, fmt.Sprintf("%s: expected array, got %T", at, val))
			return
		}
		for i, e := range arr {
			if i >= 50 {
				break
			}
			s.validate(sm["items"], e, fmt.Sprintf("%s[%d]", at, i), out)
		}
	case "string":
		str, ok := val.(string)
		if !ok {
			*out = append(*out, fmt.Sprintf("%s: expected string, got %T", at, val))
			return
		}
		if f, _ := sm["format"].(string); f == "date-time" {
			if _, err := time.Parse(time.RFC3339Nano, str); err != nil {
				*out = append(*out, fmt.Sprintf("%s: %q is not RFC3339 date-time", at, str))
			}
		}
	case "integer":
		f, ok := val.(float64)
		if !ok || f != float64(int64(f)) {
			*out = append(*out, fmt.Sprintf("%s: expected integer, got %v (%T)", at, val, val))
		}
	case "number":
		if _, ok := val.(float64); !ok {
			*out = append(*out, fmt.Sprintf("%s: expected number, got %T", at, val))
		}
	case "boolean":
		if _, ok := val.(bool); !ok {
			*out = append(*out, fmt.Sprintf("%s: expected boolean, got %T", at, val))
		}
	}
}

// nilListKeys names the collection-valued response members found by the census; extend it when a new list member is added.
var nilListKeys = map[string]bool{
	"sections": true, "groups": true, "files": true, "scans": true, "insights": true, "permissions": true,
	"local_items": true, "locations": true, "exclude_patterns": true, "include_patterns": true, "items": true,
	"results": true, "sessions": true, "users": true, "roles": true, "collections": true, "entries": true,
}

// nullScan reports JSON null values that the typed schema validation cannot see: a whole-body null, and a null object member or
// array element where the spec gives no type (no schema, or an untyped one) and does not say nullable. A nil Go slice or map
// serialises as null, which a client iterating a documented list cannot handle; it must be [] (or {}).
func (s *cSpec) nullScan(schema, val interface{}, at string, out *[]string) {
	schema = s.resolve(schema)
	sm := asMap(schema)
	if val == nil {
		if sm != nil {
			if nb, _ := sm["nullable"].(bool); nb {
				return
			}
			if _, typed := sm["type"]; typed {
				return // reported by validate
			}
		}
		// A nil pointer (optional scalar/struct) is legitimately null and indistinguishable on the wire from a nil slice; only
		// names of collection-valued members (census: docs/qa/nil-slice-null-census-20261104.md) and the body/envelope are judged.
		if at == "$" || at == "$.data" || nilListKeys[at[strings.LastIndex(at, ".")+1:]] {
			*out = append(*out, at+": JSON null where an empty array/object is required (nil slice or map serialised without normalisation)")
		}
		return
	}
	switch v := val.(type) {
	case map[string]interface{}:
		var props, ap map[string]interface{}
		if sm != nil {
			props = asMap(sm["properties"])
			ap = asMap(sm["additionalProperties"])
		}
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			var sub interface{}
			if ps, ok := props[k]; ok {
				sub = ps
			} else if ap != nil {
				sub = ap
			}
			s.nullScan(sub, v[k], at+"."+k, out)
		}
	case []interface{}:
		var items interface{}
		if sm != nil {
			items = sm["items"]
		}
		for i, e := range v {
			if i >= 50 {
				break
			}
			s.nullScan(items, e, fmt.Sprintf("%s[%d]", at, i), out)
		}
	}
}

// ---------- request generation from the schema ----------

type cGen struct {
	spec    *cSpec
	learned map[string]interface{} // resource bucket / param name -> value
	seq     int
}

var strongPW = "Contract-Value-Strong-77"

func (g *cGen) lookupLearned(name string) (interface{}, bool) {
	n := strings.ToLower(name)
	cands := []string{n}
	if strings.HasSuffix(n, "_id") {
		b := strings.TrimSuffix(n, "_id")
		cands = append(cands, b, b+"s")
	}
	for _, c := range cands {
		if v, ok := g.learned[c]; ok {
			return v, true
		}
	}
	return nil, false
}

func (g *cGen) value(schema interface{}, name string, depth int) interface{} {
	schema = g.spec.resolve(schema)
	sm := asMap(schema)
	if sm == nil || depth > 5 {
		return "contract"
	}
	if ex, ok := sm["example"]; ok {
		return ex
	}
	if en, ok := sm["enum"].([]interface{}); ok && len(en) > 0 {
		return en[0]
	}
	if ao, ok := sm["allOf"].([]interface{}); ok {
		merged := map[string]interface{}{"type": "object", "properties": map[string]interface{}{}, "required": []interface{}{}}
		for _, sub := range ao {
			sub = g.spec.resolve(sub)
			if m := asMap(sub); m != nil {
				for k, v := range asMap(m["properties"]) {
					merged["properties"].(map[string]interface{})[k] = v
				}
				if r, ok := m["required"].([]interface{}); ok {
					merged["required"] = append(merged["required"].([]interface{}), r...)
				}
			}
		}
		return g.value(merged, name, depth+1)
	}
	g.seq++
	switch sm["type"] {
	case "object":
		obj := map[string]interface{}{}
		props := asMap(sm["properties"])
		if req, ok := sm["required"].([]interface{}); ok {
			for _, r := range req {
				k := fmt.Sprint(r)
				if ps, ok := props[k]; ok {
					obj[k] = g.value(ps, k, depth+1)
				}
			}
		}
		return obj
	case "array":
		if sm["items"] == nil {
			return []interface{}{}
		}
		return []interface{}{g.value(sm["items"], name, depth+1)}
	case "integer":
		if v, ok := g.lookupLearned(name); ok {
			return v
		}
		if strings.Contains(strings.ToLower(name), "port") {
			return 1
		}
		if d, ok := sm["default"]; ok {
			return d
		}
		if mn, ok := sm["minimum"].(int); ok && mn > 1 {
			return mn
		}
		return 1
	case "number":
		return 1
	case "boolean":
		return false
	case "string":
		ln := strings.ToLower(name)
		f, _ := sm["format"].(string)
		switch {
		case f == "date-time":
			return time.Now().UTC().Format(time.RFC3339)
		case f == "date":
			return time.Now().UTC().Format("2006-01-02")
		case f == "email" || strings.Contains(ln, "email"):
			return fmt.Sprintf("contract%d@example.invalid", g.seq)
		case f == "uri" || f == "url" || ln == "url" || strings.HasSuffix(ln, "_url"):
			return "http://127.0.0.1:1/contract"
		case strings.Contains(ln, "password"):
			return strongPW
		case ln == "host" || ln == "hostname":
			return "127.0.0.1"
		case ln == "username" || ln == "name":
			return fmt.Sprintf("contract-%s-%d", ln, g.seq)
		case ln == "protocol":
			return "local"
		}
		if v, ok := g.lookupLearned(name); ok {
			return fmt.Sprint(v)
		}
		if d, ok := sm["default"].(string); ok {
			return d
		}
		return fmt.Sprintf("contract-%s-%d", ln, g.seq)
	}
	return "contract"
}

// pathParamValue picks the value for one path parameter of an operation.
func (g *cGen) pathParamValue(op *cOp, p cParam) string {
	segs := strings.Split(op.Path, "/")
	prev := ""
	for i, sg := range segs {
		if sg == "{"+p.Name+"}" && i > 0 {
			prev = strings.ToLower(segs[i-1])
		}
	}
	for _, cand := range []string{op.Method + " " + prev, prev, strings.ToLower(p.Name)} {
		if v, ok := g.learned[cand]; ok {
			return fmt.Sprint(v)
		}
	}
	if v, ok := g.lookupLearned(p.Name); ok {
		return fmt.Sprint(v)
	}
	if sm := asMap(g.spec.resolve(p.Schema)); sm != nil {
		if en, ok := sm["enum"].([]interface{}); ok && len(en) > 0 {
			return fmt.Sprint(en[0])
		}
		if sm["type"] == "integer" {
			return "1"
		}
	}
	return "x"
}

func (g *cGen) buildURL(op *cOp, pathVals map[string]string, includeQuery bool, skipQuery string) string {
	path := op.Path
	for _, p := range op.Params {
		if p.In == "path" {
			v := pathVals[p.Name]
			path = strings.ReplaceAll(path, "{"+p.Name+"}", url.PathEscape(v))
		}
	}
	q := url.Values{}
	if includeQuery {
		for _, p := range op.Params {
			if p.In == "query" && p.Required && p.Name != skipQuery {
				q.Set(p.Name, fmt.Sprint(g.value(p.Schema, p.Name, 0)))
			}
		}
	}
	if len(q) > 0 {
		path += "?" + q.Encode()
	}
	return path
}

// ---------- runner ----------

type probeResult struct {
	Status int    `json:"status"`
	CT     string `json:"content_type,omitempty"`
}

type opRecord struct {
	Method        string            `json:"method"`
	Path          string            `json:"path"`
	Public        bool              `json:"public"`
	Documented    []string          `json:"documented_statuses"`
	Probes        map[string]int    `json:"probes"`
	Observed      []int             `json:"status_codes_observed"`
	HappyReached  bool              `json:"happy_2xx_reached"`
	HappySkipped  string            `json:"happy_skipped_reason,omitempty"`
	SchemaVerdict string            `json:"schema_verdict"`
	SchemaErrors  []string          `json:"schema_errors,omitempty"`
	Violations    []string          `json:"violations,omitempty"`
	Advisories    []string          `json:"advisories,omitempty"`
	Notes         map[string]string `json:"notes,omitempty"`
}

var adminGate403 = regexp.MustCompile(`(?i)admin|lacks|system\.configure|user\.manage|report\.`)
var nonGate403 = regexp.MustCompile(`(?i)registration|pending|owner|not the caller|neither|visible but|share|job|domain not allowed|another user|session`)

type sweep struct {
	t       *testing.T
	srv     *liveSrv
	spec    *cSpec
	gen     *cGen
	anon    *http.Client
	admC    *http.Client
	plainC  *http.Client
	adm     string
	plain   string
	plainID int
	records map[string]*opRecord
	selfTok string
	selfRef string
	fixDir  string
	reqs    int
	trace   io.Writer
}

func (w *sweep) call(c *http.Client, method, path, tok string, body []byte, ct string) (int, string, []byte) {
	var rd io.Reader
	if body != nil {
		rd = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, w.srv.base()+path, rd)
	require.NoError(w.t, err)
	if body != nil {
		if ct == "" {
			ct = "application/json"
		}
		req.Header.Set("Content-Type", ct)
	}
	if tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
		if method != "GET" && method != "HEAD" && method != "OPTIONS" && strings.HasPrefix(path, "/api/v1/admin/") {
			w.addCSRF(c, req, tok)
		}
	}
	w.reqs++
	r, err := c.Do(req)
	if w.trace != nil {
		st := -1
		if err == nil {
			st = r.StatusCode
		}
		fmt.Fprintf(w.trace, "%d %s %s tok=%v\n", w.reqs, method, path, tok != "")
		fmt.Fprintf(w.trace, "   -> %d\n", st)
	}
	if err != nil {
		return -1, "", []byte(err.Error())
	}
	defer r.Body.Close()
	if strings.HasPrefix(r.Header.Get("Content-Type"), "text/event-stream") {
		// An SSE response never ends by itself: the status and headers are the contract. Reading the body would hold the sweep
		// until the client timeout (60 s per run) and make the run depend on host load (F5: one 'transport error' under load).
		return r.StatusCode, r.Header.Get("Content-Type"), nil
	}
	b, _ := io.ReadAll(io.LimitReader(r.Body, 8<<20))
	if w.trace != nil && (r.StatusCode >= 500 || r.StatusCode == 403) {
		n := len(b)
		if n > 300 {
			n = 300
		}
		fmt.Fprintf(w.trace, "   body: %s\n", b[:n])
	}
	return r.StatusCode, r.Header.Get("Content-Type"), b
}

func (w *sweep) documented(op *cOp, code int) bool {
	_, ok := op.Responses[strconv.Itoa(code)]
	return ok
}

func (w *sweep) setup(artifactSub string) {
	t := w.t
	w.fixDir = filepath.Join(w.srv.dir, "fixture-media")
	require.NoError(t, os.MkdirAll(w.fixDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(w.fixDir, "Some.Movie.2020.mp4"), []byte("fixture-video"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(w.fixDir, "song.mp3"), []byte("fixture-audio"), 0o644))
	makeNoisePNG(t, filepath.Join(w.fixDir, "pic.png"))
	w.srv.extra = append(w.srv.extra, "CATALOGIZER_LOCAL_ROOT_BASES="+w.fixDir, "CATALOGIZER_READ_ROOTS="+w.fixDir, "REGISTRATION_MODE=open")
	w.srv.mustStart()
	w.anon, w.admC, w.plainC = clientFor(201, 60*time.Second), clientFor(202, 60*time.Second), clientFor(203, 60*time.Second)
	code, tok := w.srv.login(w.admC, "admin", w.srv.adminPW)
	require.Equal(t, 200, code)
	w.adm = tok
	// plain self-registered user
	pw := strongPW
	code, _, raw := w.call(w.plainC, "POST", "/api/v1/auth/register", "", mustJSON(map[string]interface{}{"username": "contractplain", "email": "contractplain@example.invalid", "password": pw, "first_name": "C", "last_name": "P"}), "")
	require.Equal(t, 201, code, "%s", raw)
	code, w.plain = w.srv.login(w.plainC, "contractplain", pw)
	require.Equal(t, 200, code)
	// a self account for the session-ending operations (logout / change-password / refresh)
	code, _, raw = w.call(w.admC, "POST", "/api/v1/auth/register", "", mustJSON(map[string]interface{}{"username": "contractself", "email": "contractself@example.invalid", "password": pw, "first_name": "C", "last_name": "S"}), "")
	require.Equal(t, 201, code, "%s", raw)
	// throwaway victim user + role for destructive ops
	w.gen.learned["roles"] = w.mustID("POST", "/api/v1/roles", map[string]interface{}{"name": "contract-victim-role", "permissions": []string{"media.view"}}, 201)
	w.gen.learned["users"] = w.mustID("POST", "/api/v1/users", map[string]interface{}{"username": "contractvictim", "email": "contractvictim@example.invalid", "password": pw, "role_id": w.gen.learned["roles"]}, 201)
	// log collection + two shares: GET /logs/share/{token} reads one by token, DELETE /logs/share/{token} revokes one by id
	w.gen.learned["collections"] = w.mustID("POST", "/api/v1/logs/collect", map[string]interface{}{"name": "contract-logs", "components": []string{"catalog-api"}}, 200)
	shareBody := map[string]interface{}{"collection_id": w.gen.learned["collections"], "share_type": "private", "permissions": []string{"read"}}
	code, _, raw = w.call(w.admC, "POST", "/api/v1/logs/share", w.adm, mustJSON(shareBody), "")
	require.Equal(t, 200, code, "%s", raw)
	var sh struct {
		ID    int    `json:"id"`
		Token string `json:"share_token"`
	}
	require.NoError(t, json.Unmarshal(raw, &sh))
	require.NotEmpty(t, sh.Token)
	w.gen.learned["GET share"] = sh.Token
	w.gen.learned["DELETE share"] = w.mustID("POST", "/api/v1/logs/share", shareBody, 200)
	// fixture storage root + scan -> real entities
	rootID := w.mustID("POST", "/api/v1/storage/roots", map[string]interface{}{"name": "contract-local", "protocol": "local", "path": w.fixDir, "enabled": true}, 201)
	w.gen.learned["storage-roots"], w.gen.learned["roots"], w.gen.learned["storage_id"], w.gen.learned["storage_root_id"] = rootID, rootID, rootID, rootID
	code, _, raw = w.call(w.admC, "POST", "/api/v1/scans", w.adm, mustJSON(map[string]interface{}{"storage_root_id": rootID}), "")
	require.Equal(t, 202, code, "%s", raw)
	for i := 0; i < 120; i++ {
		code, _, raw = w.call(w.admC, "GET", "/api/v1/entities", w.adm, nil, "")
		var m struct {
			Items []map[string]interface{} `json:"items"`
			Total int                      `json:"total"`
		}
		_ = json.Unmarshal(raw, &m)
		if code == 200 && len(m.Items) > 0 {
			id := m.Items[0]["id"]
			w.gen.learned["entities"], w.gen.learned["media"], w.gen.learned["entity_id"], w.gen.learned["media_id"] = id, id, id, id
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func mustJSON(v interface{}) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

func (w *sweep) mustID(method, path string, body interface{}, want int) interface{} {
	code, _, raw := w.call(w.admC, method, path, w.adm, mustJSON(body), "")
	require.Equal(w.t, want, code, "%s %s: %s", method, path, raw)
	var m map[string]interface{}
	require.NoError(w.t, json.Unmarshal(raw, &m))
	require.NotNil(w.t, m["id"], "no id in %s", raw)
	return m["id"]
}

// happy-path overrides: operations whose request must be hand-shaped, or must not be executed at all.
func (w *sweep) happySkip(op *cOp) string {
	switch op.key() {
	case "GET /ws":
		return "websocket upgrade is not driven by the HTTP sweep (status of the plain GET is still recorded as the 'plain' probe)"
	case "POST /api/v1/challenges/run", "POST /api/v1/challenges/{id}/run", "POST /api/v1/challenges/run/category/{category}":
		return "executes server-side challenge suites (heavy, side-effecting); gate and unknown-target behaviour are still probed"
	case "POST /api/v1/admin/backups/{id}/restore":
		return "restores the live database over the running server; gate and unknown-id behaviour are still probed"
	}
	return ""
}

func isAuthSelf(op *cOp) bool {
	switch op.key() {
	case "POST /api/v1/auth/logout", "POST /api/v1/auth/change-password", "POST /api/v1/auth/refresh":
		return true
	}
	return false
}

func (w *sweep) record(rec *opRecord, probe string, status int) {
	rec.Probes[probe] = status
}

func (w *sweep) happyBody(op *cOp) []byte {
	switch op.key() {
	case "POST /api/v1/auth/login":
		return mustJSON(map[string]string{"username": "admin", "password": w.srv.adminPW})
	case "POST /api/v1/auth/register":
		return mustJSON(map[string]interface{}{"username": "contractreg", "email": "contractreg@example.invalid", "password": strongPW, "first_name": "C", "last_name": "R"})
	case "POST /api/v1/auth/refresh":
		return mustJSON(map[string]string{"refresh_token": w.selfRef})
	case "POST /api/v1/auth/change-password":
		return mustJSON(map[string]string{"current_password": strongPW, "new_password": "Contract-Value-Strong-88", "old_password": strongPW})
	}
	if op.Body == nil {
		return nil
	}
	return mustJSON(w.gen.value(op.Body, "", 0))
}

func (w *sweep) runOp(op *cOp) {
	rec := &opRecord{Method: op.Method, Path: op.Path, Public: op.Public, Probes: map[string]int{}, Notes: map[string]string{}, SchemaVerdict: "not-applicable"}
	for c := range op.Responses {
		rec.Documented = append(rec.Documented, c)
	}
	sort.Strings(rec.Documented)
	w.records[op.key()] = rec
	viol := func(f string, a ...interface{}) { rec.Violations = append(rec.Violations, fmt.Sprintf(f, a...)) }
	adv := func(f string, a ...interface{}) { rec.Advisories = append(rec.Advisories, fmt.Sprintf(f, a...)) }

	pv := map[string]string{}
	for _, p := range op.Params {
		if p.In == "path" {
			pv[p.Name] = w.gen.pathParamValue(op, p)
		}
	}
	valid := w.gen.buildURL(op, pv, true, "")
	var body []byte
	if op.Body != nil {
		body = w.happyBody(op)
	}
	hasPathParams := len(pv) > 0

	// --- unauth ---
	if !isAuthSelf(op) && op.Path != "/ws" {
		st, _, _ := w.call(w.anon, op.Method, valid, "", body, "")
		w.record(rec, "unauth", st)
		if !op.Public && st != 401 {
			viol("unauth: expected 401, got %d", st)
		}
	}
	// --- non-admin ---
	if d403, ok := op.Responses["403"]; ok && !op.Public {
		desc := fmt.Sprint(asMap(d403)["description"])
		gated := adminGate403.MatchString(desc) && !nonGate403.MatchString(desc)
		if gated || op.Method == "GET" {
			st, _, _ := w.call(w.plainC, op.Method, valid, w.plain, body, "")
			w.record(rec, "nonadmin", st)
			if gated && st != 403 {
				viol("nonadmin: expected 403 (spec: %q), got %d", desc, st)
			}
		}
	}
	// --- 404 / bad id ---
	if hasPathParams && !isAuthSelf(op) {
		nv := map[string]string{}
		bad := map[string]string{}
		anyInt := false
		for _, p := range op.Params {
			if p.In != "path" {
				continue
			}
			sm := asMap(w.gen.spec.resolve(p.Schema))
			_, isEnum := sm["enum"]
			if sm != nil && sm["type"] == "integer" {
				nv[p.Name] = "999999999"
				bad[p.Name] = "not-a-number"
				anyInt = true
			} else if isEnum {
				nv[p.Name] = pv[p.Name] // a closed set stays valid; the unknown id is the integer parameter
				bad[p.Name] = pv[p.Name]
			} else {
				nv[p.Name] = "no-such-" + strconv.Itoa(len(p.Name))
				bad[p.Name] = nv[p.Name]
			}
		}
		{
			st, _, _ := w.call(w.admC, op.Method, w.gen.buildURL(op, nv, true, ""), w.adm, body, "")
			w.record(rec, "notfound", st)
			if w.documented(op, 404) && st != 404 && st != 403 && !notfoundExempt(op, st) {
				// 403 is a legitimate answer when the gate runs before the lookup for admin-gated paths; admin cannot hit it.
				viol("notfound: expected 404 (documented), got %d", st)
			}
			if !w.documented(op, 404) && st == 404 {
				adv("notfound: 404 returned but not documented")
			}
		}
		if anyInt {
			st, _, _ := w.call(w.admC, op.Method, w.gen.buildURL(op, bad, true, ""), w.adm, body, "")
			w.record(rec, "badid", st)
			if st != 400 && st != 404 {
				viol("badid: non-numeric id for an integer parameter expected 400/404, got %d", st)
			}
		}
	}
	// --- malformed body ---
	if op.Body != nil && !isAuthSelf(op) && w.happySkip(op) == "" {
		st, _, _ := w.call(w.admC, op.Method, valid, w.adm, []byte(`{"broken":`), "")
		if op.Public {
			st, _, _ = w.call(w.anon, op.Method, valid, "", []byte(`{"broken":`), "")
		}
		w.record(rec, "malformed", st)
		if st != 400 {
			viol("malformed JSON body: expected 400, got %d", st)
		}
	}
	// --- missing required query param ---
	for _, p := range op.Params {
		if p.In == "query" && p.Required {
			st, _, _ := w.call(w.admC, op.Method, w.gen.buildURL(op, pv, true, p.Name), w.adm, body, "")
			w.record(rec, "missingq:"+p.Name, st)
			if st != 400 {
				viol("missing required query %q: expected 400, got %d", p.Name, st)
			}
		}
	}
	// --- happy ---
	if reason := w.happySkip(op); reason != "" {
		rec.HappySkipped = reason
		if op.Path == "/ws" {
			st, _, _ := w.call(w.admC, "GET", "/ws", w.adm, nil, "")
			w.record(rec, "plain", st)
		}
	} else {
		tok := w.adm
		cl := w.admC
		if op.Public && op.Method == "POST" && strings.HasPrefix(op.Path, "/api/v1/auth/") && !isAuthSelf(op) {
			tok, cl = "", w.anon
		}
		if isAuthSelf(op) {
			cs := clientFor(210, 60*time.Second)
			_, tk := w.srv.login(cs, "contractself", strongPW)
			// login again to capture the refresh token
			_, _, raw := w.call(cs, "POST", "/api/v1/auth/login", "", mustJSON(map[string]string{"username": "contractself", "password": strongPW}), "")
			var lm map[string]interface{}
			_ = json.Unmarshal(raw, &lm)
			if rt, _ := lm["refresh_token"].(string); rt != "" {
				w.selfRef = rt
			}
			tok, cl = tk, cs
			body = w.happyBody(op)
			if op.Method == "POST" && op.Path == "/api/v1/auth/logout" {
				body = nil
			}
		}
		st, ct, raw := w.call(cl, op.Method, valid, tok, body, "")
		w.record(rec, "happy", st)
		if exc, listed := happyExceptions(w.t)[op.key()]; st >= 200 && st < 300 {
			if listed {
				viol("happy path now reaches %d but is listed in api_contract_happy_exceptions.tsv (%s): remove the stale entry", st, exc)
			}
		} else if !listed {
			viol("happy path expected 2xx (not in api_contract_happy_exceptions.tsv), got %d", st)
		}
		w.judgeHappy(op, rec, st, ct, raw, viol, adv)
	}
	// observed set
	seen := map[int]bool{}
	for _, st := range rec.Probes {
		if !seen[st] {
			seen[st] = true
			rec.Observed = append(rec.Observed, st)
		}
	}
	sort.Ints(rec.Observed)
	for _, st := range rec.Observed {
		switch {
		case st == -1:
			viol("transport error / no response")
		case st >= 500 && st != 501 && st != 503:
			viol("server error %d observed", st)
		case (st == 501 || st == 503) && !w.documented(op, st):
			viol("%d observed but not documented", st)
		}
		if st >= 200 && st < 600 && st != 401 && st != 403 && st != 429 && st != 400 && st != 404 && !w.documented(op, st) {
			viol("status %d observed but not documented", st)
		}
		if (st == 401 || st == 403 || st == 400 || st == 404 || st == 429) && !w.documented(op, st) {
			adv("status %d observed but not documented in the spec", st)
		}
	}
}

func (w *sweep) judgeHappy(op *cOp, rec *opRecord, st int, ct string, raw []byte, viol, adv func(string, ...interface{})) {
	if st >= 200 && st < 300 {
		rec.HappyReached = true
	}
	// learn ids from successful creations / listings
	if st >= 200 && st < 300 && strings.Contains(ct, "json") {
		w.learn(op, raw)
	}
	// Class check (nil Go slice/map -> JSON null), independent of whether the spec documents a schema.
	if st >= 200 && st < 300 && strings.Contains(ct, "json") {
		var nv interface{}
		if err := json.NewDecoder(bytes.NewReader(raw)).Decode(&nv); err == nil {
			rc0 := strconv.Itoa(st)
			var sch interface{}
			if r0, ok := op.Responses[rc0]; ok {
				if m := asMap(asMap(asMap(w.spec.resolve(r0))["content"])["application/json"]); m != nil {
					sch = m["schema"]
				}
			}
			var nulls []string
			w.spec.nullScan(sch, nv, "$", &nulls)
			for _, n := range nulls {
				viol("%s", n)
			}
		}
	}
	rc := strconv.Itoa(st)
	resp, documented := op.Responses[rc]
	if !documented {
		return
	}
	if st >= 200 && st < 300 {
		content := asMap(asMap(w.spec.resolve(resp))["content"])
		js := asMap(content["application/json"])
		if js == nil || js["schema"] == nil {
			if strings.Contains(ct, "json") && len(bytes.TrimSpace(raw)) > 0 {
				rec.SchemaVerdict = "no-schema-in-spec"
				adv("2xx JSON body returned but the spec documents no JSON schema for %d", st)
			} else {
				rec.SchemaVerdict = "non-json-or-empty"
			}
			return
		}
		if !strings.Contains(ct, "json") {
			rec.SchemaVerdict = "invalid"
			rec.SchemaErrors = append(rec.SchemaErrors, "content-type "+ct+" is not JSON")
			viol("response content-type %q is not JSON although the spec documents a JSON body", ct)
			return
		}
		var v interface{}
		dec := json.NewDecoder(bytes.NewReader(raw))
		if err := dec.Decode(&v); err != nil {
			rec.SchemaVerdict = "invalid"
			rec.SchemaErrors = append(rec.SchemaErrors, "body is not JSON: "+err.Error())
			viol("2xx body is not valid JSON")
			return
		}
		var errs []string
		w.spec.validate(js["schema"], v, "$", &errs)
		if len(errs) == 0 {
			rec.SchemaVerdict = "valid"
		} else {
			rec.SchemaVerdict = "invalid"
			if len(errs) > 12 {
				errs = errs[:12]
			}
			rec.SchemaErrors = errs
			viol("response body does not match the documented schema (%d+ problems): %s", len(errs), errs[0])
		}
	}
}

// learn registers ids found in a successful response so later operations use real resources.
func (w *sweep) learn(op *cOp, raw []byte) {
	var v interface{}
	if json.Unmarshal(raw, &v) != nil {
		return
	}
	segs := strings.Split(strings.Trim(op.Path, "/"), "/")
	bucket := ""
	for i := len(segs) - 1; i >= 0; i-- {
		if !strings.HasPrefix(segs[i], "{") {
			bucket = strings.ToLower(segs[i])
			break
		}
	}
	if bucket == "" || bucket == "users" || bucket == "roles" || bucket == "auth" {
		return // explicit throwaways own these buckets (never learn admin / self accounts)
	}
	set := func(k string, id interface{}) {
		if id == nil {
			return
		}
		if op.Method == "POST" {
			w.gen.learned[k] = id
		} else if _, ok := w.gen.learned[k]; !ok {
			w.gen.learned[k] = id
		}
	}
	if m, ok := v.(map[string]interface{}); ok {
		for _, key := range []string{"id", "job_id", "token", "session_id"} {
			if x, ok := m[key]; ok && x != nil {
				switch x.(type) {
				case float64, string:
					set(bucket, x)
					if key != "id" {
						set(key, x)
					}
				}
			}
		}
		for _, lk := range []string{"items", "roots", "data", "results"} {
			if arr, ok := m[lk].([]interface{}); ok && len(arr) > 0 {
				if f, ok := arr[0].(map[string]interface{}); ok && f["id"] != nil && op.Method == "GET" {
					set(bucket, f["id"])
				}
			}
		}
	}
	if arr, ok := v.([]interface{}); ok && len(arr) > 0 && op.Method == "GET" {
		if f, ok := arr[0].(map[string]interface{}); ok && f["id"] != nil {
			set(bucket, f["id"])
		}
	}
}

func (w *sweep) order() []*cOp {
	depth := func(o *cOp) int { return strings.Count(o.Path, "/") }
	group := func(o *cOp) int {
		switch {
		case isAuthSelf(o):
			return 6
		case o.Method == "GET" && !strings.Contains(o.Path, "{"):
			return 0
		case o.Method == "POST":
			return 1
		case o.Method == "PUT" || o.Method == "PATCH":
			return 2
		case o.Method == "GET" || o.Method == "HEAD" || o.Method == "OPTIONS":
			return 3
		case o.Method == "DELETE":
			return 4
		}
		return 5
	}
	ops := append([]*cOp(nil), w.spec.Ops...)
	sort.SliceStable(ops, func(i, j int) bool {
		gi, gj := group(ops[i]), group(ops[j])
		if gi != gj {
			return gi < gj
		}
		if gi == 4 {
			return depth(ops[i]) > depth(ops[j])
		}
		if gi == 1 && depth(ops[i]) != depth(ops[j]) {
			return depth(ops[i]) < depth(ops[j])
		}
		return ops[i].Path+ops[i].Method < ops[j].Path+ops[j].Method
	})
	return ops
}

// signature is the part of a run that must be identical across runs.
func (w *sweep) signature() (string, []byte) {
	keys := make([]string, 0, len(w.records))
	for k := range w.records {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var sb strings.Builder
	for _, k := range keys {
		r := w.records[k]
		pk := make([]string, 0, len(r.Probes))
		for p := range r.Probes {
			pk = append(pk, p)
		}
		sort.Strings(pk)
		sb.WriteString(k + "|")
		for _, p := range pk {
			sb.WriteString(fmt.Sprintf("%s=%d,", p, r.Probes[p]))
		}
		vs := append([]string(nil), r.Violations...)
		sort.Strings(vs)
		sb.WriteString("|" + r.SchemaVerdict + "|" + fmt.Sprint(r.HappyReached) + "|" + strings.Join(vs, ";") + "\n")
	}
	sum := sha256.Sum256([]byte(sb.String()))
	return hex.EncodeToString(sum[:]), []byte(sb.String())
}

func TestLiveAPIContract(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs the server binary")
	}
	specPath := os.Getenv("API_CONTRACT_SPEC")
	if specPath == "" {
		specPath = filepath.Join("..", "docs", "api", "openapi.yaml")
	}
	spec := loadSpec(t, specPath)
	require.NotEmpty(t, spec.Ops)
	runs := 3
	if v, err := strconv.Atoi(os.Getenv("API_CONTRACT_RUNS")); err == nil && v > 0 {
		runs = v
	}
	var sigs []string
	var firstRecs []*opRecord
	var firstSigText string
	var identity map[string]interface{}
	var builtSha string
	totalReqs := 0
	for run := 1; run <= runs; run++ {
		srv := newLiveSrv(t, fmt.Sprintf("contract-%d", run))
		defer srv.cleanup()
		w := &sweep{t: t, srv: srv, spec: spec, gen: &cGen{spec: spec, learned: map[string]interface{}{}}, records: map[string]*opRecord{}}
		if os.Getenv("API_CONTRACT_TRACE") != "" {
			tf, err := os.Create(filepath.Join(artifactDir(t), fmt.Sprintf("trace-run%d.log", run)))
			require.NoError(t, err)
			defer tf.Close()
			w.trace = tf
		}
		w.setup("")
		for _, op := range w.order() {
			w.runOp(op)
		}
		sig, sigText := w.signature()
		sigs = append(sigs, sig)
		if run == 1 {
			firstSigText = string(sigText)
		} else if sig != sigs[0] {
			// name the operations whose signature line differs, then fail below
			a := strings.Split(firstSigText, "\n")
			b := strings.Split(string(sigText), "\n")
			seen := map[string]bool{}
			for _, l := range a {
				seen[l] = true
			}
			for _, l := range b {
				if !seen[l] {
					t.Logf("DIFF run %d only: %s", run, l)
				}
			}
			seenB := map[string]bool{}
			for _, l := range b {
				seenB[l] = true
			}
			for _, l := range a {
				if !seenB[l] {
					t.Logf("DIFF run 1 only: %s", l)
				}
			}
		}
		totalReqs += w.reqs
		id := srv.identity()
		if run == 1 {
			identity = id
			builtSha, _ = buildInfo["sha256"].(string)
			keys := make([]string, 0, len(w.records))
			for k := range w.records {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				firstRecs = append(firstRecs, w.records[k])
			}
		}
		// every run must have been served by the same binary
		require.Equal(t, identity["exe_sha256"], id["exe_sha256"], "run %d served by a different binary", run)
		runViol := 0
		for _, r := range w.records {
			runViol += len(r.Violations)
		}
		t.Logf("run %d: pid=%v endpoint=%v requests=%d violations=%d signature=%s", run, id["pid"], id["endpoint"], w.reqs, runViol, sig)
		if run == runs {
			// keep the last run's server identity too (pid differs per run, recorded below)
		}
		srv.stopTerm(20 * time.Second)
		if run > 1 {
			require.Equal(t, sigs[0], sig, "run %d signature differs from run 1: the sweep is not deterministic", run)
		}
	}

	// ---- evidence ----
	byCode := map[string]int{}
	nViol, nAdv, nHappy, nSkip, nInvalid, nValid := 0, 0, 0, 0, 0, 0
	for _, r := range firstRecs {
		nViol += len(r.Violations)
		nAdv += len(r.Advisories)
		if r.HappyReached {
			nHappy++
		}
		if r.HappySkipped != "" {
			nSkip++
		}
		switch r.SchemaVerdict {
		case "valid":
			nValid++
		case "invalid":
			nInvalid++
		}
		for _, st := range r.Observed {
			byCode[strconv.Itoa(st)]++
		}
	}
	opsWithViol := 0
	for _, r := range firstRecs {
		if len(r.Violations) > 0 {
			opsWithViol++
		}
	}
	summary := map[string]interface{}{
		"spec":                       "docs/api/openapi.yaml",
		"operations":                 len(firstRecs),
		"runs":                       runs,
		"signature_sha256":           sigs[0],
		"signatures_identical":       allEqual(sigs),
		"signatures":                 sigs,
		"binary_sha256":              builtSha,
		"binary_exe_sha256":          identity["exe_sha256"],
		"endpoint_run1":              identity["endpoint"],
		"pid_run1":                   identity["pid"],
		"build":                      buildInfo,
		"requests_total_all_runs":    totalReqs,
		"operations_with_violations": opsWithViol,
		"violations":                 nViol,
		"advisories":                 nAdv,
		"happy_2xx_reached":          nHappy,
		"happy_skipped":              nSkip,
		"schema_valid":               nValid,
		"schema_invalid":             nInvalid,
		"operations_per_status_code": byCode,
		"generated_utc":              time.Now().UTC().Format(time.RFC3339),
	}
	writeEvidence(t, "summary.json", summary)
	writeEvidence(t, "operations.json", firstRecs)
	for _, r := range firstRecs {
		for _, v := range r.Violations {
			t.Errorf("VIOLATION %s %s: %s", r.Method, r.Path, v)
		}
	}
	require.True(t, allEqual(sigs), "signatures differ across runs")
	require.Equal(t, 0, nViol, "contract violations found (see operations.json)")
}

func allEqual(s []string) bool {
	for _, x := range s {
		if x != s[0] {
			return false
		}
	}
	return true
}

// notfoundExempt lists operations whose documented 404 is deliberately not
// returned for an unknown id. Every entry carries its reason; an entry that
// stops being true must be removed, not widened.
func notfoundExempt(op *cOp, st int) bool {
	switch op.Method + " " + op.Path {
	case "GET /api/v1/assets/{id}":
		// By design the asset endpoint answers 200 with a pending placeholder
		// (header X-Asset-Status: pending) while an asset is being resolved;
		// an unknown id is indistinguishable from a not-yet-resolved one.
		return st == 200
	}
	return false
}

// addCSRF performs what a browser client does for the /admin group, which is
// guarded by a double-submit CSRF token (middleware/csrf.go): any GET under
// /admin mints a signed token, returned as a cookie and in the X-CSRF-Token
// header; a mutating call must send the cookie and the header back. The
// cookie is Secure outside dev mode so a cookie jar would not return it over
// plain HTTP; it is attached explicitly. These two requests are counted.
func (w *sweep) addCSRF(c *http.Client, req *http.Request, tok string) {
	gr, err := http.NewRequest("GET", w.srv.base()+"/api/v1/admin/system-info", nil)
	if err != nil {
		return
	}
	gr.Header.Set("Authorization", "Bearer "+tok)
	w.reqs++
	resp, err := c.Do(gr)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	hdr := resp.Header.Get("X-CSRF-Token")
	for _, ck := range resp.Cookies() {
		// the server writes the cookie with url.QueryEscape (gin SetCookie), so ":" arrives as "%3A"; the raw value is sent back
		// unchanged (the server unescapes it) and only the comparison is made on the decoded form.
		if dec, err := url.QueryUnescape(ck.Value); hdr != "" && err == nil && strings.HasPrefix(dec, hdr+":") {
			req.Header.Set("X-CSRF-Token", hdr)
			req.AddCookie(&http.Cookie{Name: ck.Name, Value: ck.Value})
			return
		}
	}
	if w.trace != nil {
		fmt.Fprintf(w.trace, "   addCSRF: no usable pair status=%d hdr-present=%v set-cookies=%d\n", resp.StatusCode, hdr != "", len(resp.Cookies()))
	}
}

var (
	happyExcOnce sync.Once
	happyExc     map[string]string
)

// happyExceptions loads the committed list of operations whose happy-path probe is allowed to end in a non-2xx status.
func happyExceptions(t *testing.T) map[string]string {
	happyExcOnce.Do(func() {
		happyExc = map[string]string{}
		raw, err := os.ReadFile("api_contract_happy_exceptions.tsv")
		require.NoError(t, err)
		for _, l := range strings.Split(string(raw), "\n") {
			if l == "" || strings.HasPrefix(l, "#") {
				continue
			}
			kv := strings.SplitN(l, "\t", 2)
			require.Len(t, kv, 2, "bad exceptions line %q", l)
			happyExc[kv[0]] = kv[1]
		}
	})
	return happyExc
}
