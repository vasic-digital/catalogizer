package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// CAT-HAUTHZ-1: horizontal-authorization sweep. Two ordinary (non-admin) users, A and B, on the REAL server binary
// (real router assembly, real SQLCipher database, real register + login, real scan of real files). B creates a
// resource on every per-user route; A must never read, change or delete it, and never see B's identifiers in a body.
// Positive controls: B reaches its own resource, and where the route table says so the administrator does too.
// No test doubles stand in for anything here. Each case prints one `HZ|<case>|<got>|<verdict>` line.

const hzAdminPw = "Op3rator-Supplied-Value-9x"

type hzEnv struct {
	t      *testing.T
	c      *apiClient
	adm    string
	a, b   string // plain user tokens
	aid    int    // user ids
	bid    int
	bEmail string
	bName  string
	ca, cb string // two ordinary users holding the conversion permissions
	ra, rb string // two ordinary users holding report.view and report.create (error/crash reports)
	media  []int  // real scanned entity ids
	mtype  string // entity type of the scanned items
	dir    string // server working directory (holds ./data)
}

// denied: the answer must be a refusal (403/404, or 401 where the token itself is the issue) and must not carry B's markers.
func (e *hzEnv) denied(name, method, path, tok string, body interface{}, markers ...string) {
	e.t.Helper()
	e.t.Run("denied/"+name, func(t *testing.T) {
		code, raw := e.c.do(method, path, tok, body)
		ok := code == 403 || code == 404
		t.Logf("HZ|%s|%d|%s", name, code, map[bool]string{true: "REFUSED", false: "NOT-REFUSED"}[ok])
		assert.True(t, ok, "%s %s as the other user must be 403/404, got %d: %.300s", method, path, code, raw)
		for _, m := range markers {
			assert.NotContains(t, string(raw), m, "%s %s leaked %q", method, path, m)
		}
	})
}

// noLeak: the call may succeed (it is the caller's own view) but the body must not contain B's markers.
func (e *hzEnv) noLeak(name, method, path, tok string, markers ...string) {
	e.t.Helper()
	e.t.Run("noleak/"+name, func(t *testing.T) {
		code, raw := e.c.do(method, path, tok, nil)
		t.Logf("HZ|%s|%d|%s", name, code, "BODY-CHECKED")
		for _, m := range markers {
			assert.NotContains(t, string(raw), m, "%s %s leaked %q (code %d)", method, path, m, code)
		}
	})
}

func (e *hzEnv) ok(name, method, path, tok string, body interface{}, wantContains ...string) []byte {
	e.t.Helper()
	var out []byte
	e.t.Run("control/"+name, func(t *testing.T) {
		code, raw := e.c.do(method, path, tok, body)
		t.Logf("HZ|%s|%d|%s", name, code, "POSITIVE-CONTROL")
		require.Less(t, code, 300, "%s %s: %s", method, path, raw)
		for _, w := range wantContains {
			require.Contains(t, string(raw), w, "positive control %s %s", method, path)
		}
		out = raw
	})
	return out
}

func hzJSON(t *testing.T, raw []byte) map[string]interface{} {
	m := map[string]interface{}{}
	require.NoError(t, json.Unmarshal(raw, &m), "%s", raw)
	return m
}

func hzSelf(t *testing.T, c *apiClient, tok string) (int, string, string) {
	code, raw := c.do("GET", "/api/v1/auth/me", tok, nil)
	require.Equal(t, 200, code, "%s", raw)
	m := hzJSON(t, raw)
	id, _ := m["id"].(float64)
	un, _ := m["username"].(string)
	em, _ := m["email"].(string)
	require.NotZero(t, id, "%s", raw)
	return int(id), un, em
}

func hzSetup(t *testing.T) *hzEnv {
	dir := t.TempDir()
	media := filepath.Join(dir, "media")
	require.NoError(t, os.MkdirAll(media, 0o755))
	for i, n := range []string{"hzclip-alpha", "hzclip-bravo"} {
		p := make([]byte, 6000)
		for j := range p {
			p[j] = byte(j*7) ^ byte(i+1)
		}
		require.NoError(t, os.WriteFile(filepath.Join(media, n+".mp4"), p, 0o644))
	}
	c, adm, _, stop := startPermServerDir(t, dir, media)
	t.Cleanup(stop)
	e := &hzEnv{t: t, c: c, adm: adm, dir: dir}

	rootID := c.id("POST", "/api/v1/storage/roots", adm, map[string]interface{}{"name": "hz", "protocol": "local", "path": media}, 201)
	code, raw := c.do("POST", "/api/v1/scans", adm, map[string]interface{}{"storage_root_id": rootID, "storage_root_name": "hz"})
	require.Less(t, code, 300, "%s", raw)
	var q struct {
		JobID string `json:"job_id"`
	}
	require.NoError(t, json.Unmarshal(raw, &q))
	require.Eventually(t, func() bool {
		_, r := c.do("GET", "/api/v1/scans/"+q.JobID, adm, nil)
		return strings.Contains(string(r), "completed") || strings.Contains(string(r), "failed")
	}, 90*time.Second, 500*time.Millisecond)
	for _, n := range []string{"hzclip-alpha", "hzclip-bravo"} {
		_, r := c.do("GET", "/api/v1/media/search?query="+n+"&limit=10", adm, nil)
		var s struct {
			Items []struct {
				ID        int
				Title     string
				MediaType string `json:"media_type"`
			} `json:"items"`
		}
		require.NoError(t, json.Unmarshal(r, &s))
		require.NotEmpty(t, s.Items, "scan catalogued %s: %s", n, r)
		e.media = append(e.media, s.Items[0].ID)
		e.mtype = "movie" // entity_type accepted by favorites/playlists for these scanned items
	}
	require.NotEqual(t, e.media[0], e.media[1])

	e.a = selfRegister(t, c, "hzusera")
	e.b = selfRegister(t, c, "hzuserb")
	e.aid, _, _ = hzSelf(t, c, e.a)
	var bn, be string
	e.bid, bn, be = hzSelf(t, c, e.b)
	e.bName, e.bEmail = bn, be
	require.NotEqual(t, e.aid, e.bid)

	role := c.id("POST", "/api/v1/roles", adm, map[string]interface{}{"name": "hz-conv", "permissions": []string{"conversion.create", "conversion.view", "conversion.manage"}}, 201)
	for _, n := range []string{"hzconva", "hzconvb"} {
		c.id("POST", "/api/v1/users", adm, map[string]interface{}{"username": n, "email": n + "@example.invalid", "password": "Contract-Value-Strong-77", "role_id": role}, 201)
	}
	e.ca = c.login("hzconva", "Contract-Value-Strong-77")
	e.cb = c.login("hzconvb", "Contract-Value-Strong-77")
	rrole := c.id("POST", "/api/v1/roles", adm, map[string]interface{}{"name": "hz-report", "permissions": []string{"report.create", "report.view"}}, 201)
	for _, n := range []string{"hzrepa", "hzrepb"} {
		c.id("POST", "/api/v1/users", adm, map[string]interface{}{"username": n, "email": n + "@example.invalid", "password": "Contract-Value-Strong-77", "role_id": rrole}, 201)
	}
	e.ra = c.login("hzrepa", "Contract-Value-Strong-77")
	e.rb = c.login("hzrepb", "Contract-Value-Strong-77")
	// A and B must be ordinary users: neither may reach an admin-only route.
	code, _ = c.do("GET", "/api/v1/admin/users", e.a, nil)
	require.Equal(t, 403, code, "user A must be non-admin")
	code, _ = c.do("GET", "/api/v1/admin/users", e.b, nil)
	require.Equal(t, 403, code, "user B must be non-admin")
	return e
}

func TestHorizontalAuthz_RealBinary(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs the server binary")
	}
	e := hzSetup(t)
	c := e.c
	m0, m1 := e.media[0], e.media[1]
	bID := fmt.Sprint(e.bid)

	// ---- collections (owner-scoped; handlers/collection_handler.go) --------------------------------------------------------
	t.Run("collections", func(t *testing.T) {
		const mark = "hzmark-b-collection"
		cid := c.id("POST", "/api/v1/collections", e.b, map[string]interface{}{"name": mark, "description": "private of B"}, 201)
		pubID := c.id("POST", "/api/v1/collections", e.b, map[string]interface{}{"name": "hzmark-b-public", "is_public": true}, 201)
		p := fmt.Sprintf("/api/v1/collections/%d", cid)
		e.ok("collection B reads own", "GET", p, e.b, nil, mark)
		e.ok("collection admin reads", "GET", p, e.adm, nil, mark)
		e.denied("collection read", "GET", p, e.a, nil, mark)
		e.denied("collection items", "GET", p+"/items", e.a, nil, mark)
		e.denied("collection update", "PUT", p, e.a, map[string]interface{}{"name": "hz-hijacked-by-a"}, mark)
		e.denied("collection delete", "DELETE", p, e.a, nil, mark)
		e.noLeak("collection list", "GET", "/api/v1/collections", e.a, mark)
		e.ok("collection list B sees own", "GET", "/api/v1/collections", e.b, nil, mark)
		// by design a public collection is readable by others but never writable
		e.ok("collection public readable by A (design)", "GET", fmt.Sprintf("/api/v1/collections/%d", pubID), e.a, nil, "hzmark-b-public")
		e.denied("collection public update", "PUT", fmt.Sprintf("/api/v1/collections/%d", pubID), e.a, map[string]interface{}{"name": "hz-hijacked-pub"})
		e.denied("collection public delete", "DELETE", fmt.Sprintf("/api/v1/collections/%d", pubID), e.a, nil)
		// state unchanged: B's collection still carries its name and still exists
		e.ok("collection survives A's attempts", "GET", p, e.b, nil, mark)
	})

	// ---- playlists ---------------------------------------------------------------------------------------------------------
	t.Run("playlists", func(t *testing.T) {
		const mark = "hzmark-b-playlist"
		pid := c.id("POST", "/api/v1/playlists", e.b, map[string]interface{}{"name": mark}, 201)
		p := fmt.Sprintf("/api/v1/playlists/%d", pid)
		code, raw := c.do("POST", p+"/items", e.b, map[string]interface{}{"entity_id": m0, "entity_type": e.mtype})
		require.Less(t, code, 300, "B adds an item to its playlist: %s", raw)
		itemID := int(hzJSON(t, raw)["id"].(float64))
		e.ok("playlist B reads own", "GET", p, e.b, nil, mark)
		e.denied("playlist read", "GET", p, e.a, nil, mark)
		e.denied("playlist update", "PUT", p, e.a, map[string]interface{}{"name": "hz-hijacked"}, mark)
		e.denied("playlist add item", "POST", p+"/items", e.a, map[string]interface{}{"entity_id": m1, "entity_type": e.mtype}, mark)
		e.denied("playlist remove item", "DELETE", fmt.Sprintf("%s/items/%d", p, itemID), e.a, nil, mark)
		e.denied("playlist delete", "DELETE", p, e.a, nil, mark)
		e.noLeak("playlist list", "GET", "/api/v1/playlists", e.a, mark)
		e.ok("playlist list B sees own", "GET", "/api/v1/playlists", e.b, nil, mark)
		_, after := c.do("GET", p, e.b, nil)
		assert.Contains(t, string(after), mark, "B's playlist survives A's attempts")
		assert.Contains(t, string(after), fmt.Sprint(m0), "B's playlist item survives A's remove attempt")
		assert.NotContains(t, string(after), "hz-hijacked")
	})

	// ---- favorites (self-scoped by token) ----------------------------------------------------------------------------------
	t.Run("favorites", func(t *testing.T) {
		e.ok("favorite B adds", "POST", "/api/v1/favorites", e.b, map[string]interface{}{"entity_id": m0, "entity_type": e.mtype})
		e.t.Run("noleak/favorite list A is empty", func(t *testing.T) {
			code, raw := c.do("GET", "/api/v1/favorites", e.a, nil)
			require.Equal(t, 200, code)
			assert.Equal(t, float64(0), hzJSON(t, raw)["count"], "A must not see B's favorites: %s", raw)
			t.Logf("HZ|favorite list|%d|BODY-CHECKED", code)
		})
		e.t.Run("noleak/favorite check A is false", func(t *testing.T) {
			code, raw := c.do("GET", fmt.Sprintf("/api/v1/favorites/check/%s/%d", e.mtype, m0), e.a, nil)
			require.Equal(t, 200, code, "%s", raw)
			assert.Equal(t, false, hzJSON(t, raw)["is_favorite"], "%s", raw)
			t.Logf("HZ|favorite check|%d|BODY-CHECKED", code)
		})
		e.t.Run("denied/favorite A removes B's", func(t *testing.T) {
			code, raw := c.do("DELETE", fmt.Sprintf("/api/v1/favorites/%s/%d", e.mtype, m0), e.a, nil)
			t.Logf("HZ|favorite remove|%d|%s", code, "CHECKED")
			assert.Contains(t, []int{403, 404}, code, "A has no favorite for this entity: %s", raw)
		})
		e.ok("favorite B still has it", "GET", fmt.Sprintf("/api/v1/favorites/check/%s/%d", e.mtype, m0), e.b, nil, `"is_favorite":true`)
		// PUT /media/:id/favorite writes the GLOBAL media_items.is_favorite: finding HAUTHZ-F2 fixed (CAT-HAUTHZ-F2), see TestHorizontalAuthz_MediaFavoriteFlag.
	})

	// ---- entity user-metadata: write path only ---------------------------------------------------------------------------------
	// HAUTHZ-O1 added GET /entities/:id/user-metadata (caller-scoped), so the read leg below now discriminates an owner leak.
	// Historically no API route returned user metadata, so a binary-level read could not discriminate here. The cross-user property (owner from the token, never the body) is pinned at store
	// level in handlers/horizontal_authz_unit_test.go (TestHorizontalAuthzUnit_UserMetadataOwnerFromTokenNotBody). This
	// block only checks that both users can write their own row through the real binary.
	t.Run("user_metadata", func(t *testing.T) {
		e.ok("usermeta A writes own (body names B)", "PUT", fmt.Sprintf("/api/v1/entities/%d/user-metadata", m0), e.a, map[string]interface{}{"user_id": e.bid, "personal_notes": "hzmark-a-note", "user_rating": 1.0})
		e.ok("usermeta B writes own", "PUT", fmt.Sprintf("/api/v1/entities/%d/user-metadata", m0), e.b, map[string]interface{}{"personal_notes": "hzmark-b-note"})
		// HAUTHZ-O1: GET /entities/:id/user-metadata is caller-scoped. B writes on m1 only; A reads m1 (no row of its own) and
		// must get 404 without B's note, the same status a nonexistent entity gets. Own reads return the caller's own note.
		e.ok("usermeta B writes own on m1", "PUT", fmt.Sprintf("/api/v1/entities/%d/user-metadata", m1), e.b, map[string]interface{}{"personal_notes": "hzmark-b-m1-note"})
		e.ok("usermeta read B own", "GET", fmt.Sprintf("/api/v1/entities/%d/user-metadata", m1), e.b, nil, "hzmark-b-m1-note")
		e.ok("usermeta read A own", "GET", fmt.Sprintf("/api/v1/entities/%d/user-metadata", m0), e.a, nil, "hzmark-a-note")
		e.denied("usermeta read foreign (A reads B's m1)", "GET", fmt.Sprintf("/api/v1/entities/%d/user-metadata", m1), e.a, nil, "hzmark-b-m1-note", "hzmark-b-note")
		// HAUTHZ-O1 review m-1: the foreign read must be EXACTLY 404 and byte-identical to the read of a nonexistent entity
		// (no oracle for another user's row); an anonymous read is 401.
		e.t.Run("strict/usermeta foreign vs missing vs anonymous", func(t *testing.T) {
			fcode, fraw := e.c.do("GET", fmt.Sprintf("/api/v1/entities/%d/user-metadata", m1), e.a, nil)
			mcode, mraw := e.c.do("GET", "/api/v1/entities/999999999/user-metadata", e.a, nil)
			t.Logf("HZ|usermeta strict|foreign=%d missing=%d", fcode, mcode)
			assert.Equal(t, 404, fcode, "foreign read must be exactly 404: %.300s", fraw)
			assert.Equal(t, 404, mcode, "missing-entity read must be exactly 404: %.300s", mraw)
			assert.Equal(t, string(mraw), string(fraw), "foreign and missing bodies must be byte-identical")
			acode, araw := e.c.do("GET", fmt.Sprintf("/api/v1/entities/%d/user-metadata", m1), "", nil)
			assert.Equal(t, 401, acode, "anonymous read must be 401: %.300s", araw)
		})
	})

	// ---- playback sessions / progress / history ----------------------------------------------------------------------------
	t.Run("playback", func(t *testing.T) {
		code, raw := c.do("POST", "/api/v1/playback/sessions/start", e.b, map[string]interface{}{"media_item_id": m0, "position_unit": "seconds", "start_position": 11})
		require.Equal(t, 200, code, "%s", raw)
		sid := int64(hzJSON(t, raw)["session_id"].(float64))
		e.ok("playback B progress own", "POST", "/api/v1/playback/sessions/progress", e.b, map[string]interface{}{"session_id": sid, "end_position": 111, "total_amount": 111})
		hist := fmt.Sprintf("/api/v1/entities/%d/history", m0)
		prog := fmt.Sprintf("/api/v1/entities/%d/progress", m0)
		e.ok("playback B history own", "GET", hist, e.b, nil, `"end_position":111`)
		e.t.Run("noleak/history A", func(t *testing.T) {
			code, raw := c.do("GET", hist, e.a, nil)
			require.Equal(t, 200, code)
			assert.Equal(t, float64(0), hzJSON(t, raw)["count"], "A must not see B's playback history: %s", raw)
			t.Logf("HZ|playback history|%d|BODY-CHECKED", code)
		})
		e.t.Run("noleak/progress A", func(t *testing.T) {
			code, raw := c.do("GET", prog, e.a, nil)
			require.Equal(t, 200, code)
			assert.Nil(t, hzJSON(t, raw)["progress"], "A must not see B's progress: %s", raw)
			t.Logf("HZ|playback progress|%d|BODY-CHECKED", code)
		})
		// A names B's session id
		e.denied("playback progress foreign session", "POST", "/api/v1/playback/sessions/progress", e.a, map[string]interface{}{"session_id": sid, "end_position": 999, "total_amount": 999})
		e.denied("playback end foreign session", "POST", "/api/v1/playback/sessions/end", e.a, map[string]interface{}{"session_id": sid, "end_position": 888, "total_amount": 888, "completed": true})
		// no id oracle: a foreign session and a nonexistent session answer with the same status and the same body
		e.t.Run("denied/foreign session is indistinguishable from a missing one", func(t *testing.T) {
			for _, ep := range []string{"/api/v1/playback/sessions/progress", "/api/v1/playback/sessions/end"} {
				fc, fraw := c.do("POST", ep, e.a, map[string]interface{}{"session_id": sid, "end_position": 5, "total_amount": 9})
				mc, mraw := c.do("POST", ep, e.a, map[string]interface{}{"session_id": 987654321, "end_position": 5, "total_amount": 9})
				t.Logf("HZ|playback no-oracle %s|%d/%d|CHECKED", ep, fc, mc)
				assert.Equal(t, 404, mc, "%s missing: %s", ep, mraw)
				assert.Equal(t, 404, fc, "%s foreign must be 404 (not 403): %s", ep, fraw)
				assert.Equal(t, string(mraw), string(fraw), "%s: foreign and missing bodies must be identical", ep)
			}
		})
		e.t.Run("state/B's session unchanged", func(t *testing.T) {
			_, raw := c.do("GET", hist, e.b, nil)
			assert.Contains(t, string(raw), `"end_position":111`, "B's session must still read 111: %s", raw)
			assert.NotContains(t, string(raw), `"end_position":999`)
			assert.NotContains(t, string(raw), `"end_position":888`)
			_, raw = c.do("GET", prog, e.b, nil)
			assert.NotContains(t, string(raw), "888", "B's progress summary must not carry A's write: %s", raw)
		})
	})

	// ---- analytics / recommendations (path user id) -----------------------------------------------------------------------
	t.Run("analytics", func(t *testing.T) {
		e.denied("analytics user/B", "GET", "/api/v1/analytics/user/"+bID, e.a, nil)
		e.ok("analytics B reads own", "GET", "/api/v1/analytics/user/"+bID, e.b, nil)
		e.ok("analytics admin reads B", "GET", "/api/v1/analytics/user/"+bID, e.adm, nil)
		e.denied("analytics write as B (access)", "POST", "/api/v1/analytics/access", e.a, map[string]interface{}{"user_id": e.bid, "media_id": m0, "action": "play"})
		e.denied("analytics write as B (event)", "POST", "/api/v1/analytics/event", e.a, map[string]interface{}{"user_id": e.bid, "event_type": "hz", "event_category": "hz"})
		e.ok("analytics A writes as self", "POST", "/api/v1/analytics/event", e.a, map[string]interface{}{"event_type": "hz", "event_category": "hz"})
		e.denied("recommendations personalized/B", "GET", "/api/v1/recommendations/personalized/"+bID, e.a, nil)
		e.t.Run("control/recommendations B reaches own", func(t *testing.T) {
			code, raw := c.do("GET", "/api/v1/recommendations/personalized/"+bID, e.b, nil)
			t.Logf("HZ|recommendations B own|%d|POSITIVE-CONTROL", code)
			assert.NotContains(t, []int{401, 403}, code, "%s", raw)
		})
	})

	// ---- subtitles (uploader-scoped delete) ---------------------------------------------------------------------------------
	t.Run("subtitles", func(t *testing.T) {
		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		mw.WriteField("media_item_id", fmt.Sprint(m0))
		mw.WriteField("language", "English")
		mw.WriteField("language_code", "en")
		fw, _ := mw.CreateFormFile("file", "hz.srt")
		fw.Write([]byte("1\n00:00:01,000 --> 00:00:02,000\nhzmark-b-subtitle\n\n"))
		mw.Close()
		req, _ := http.NewRequest("POST", c.base+"/api/v1/subtitles/upload", &buf)
		req.Header.Set("Content-Type", mw.FormDataContentType())
		req.Header.Set("Authorization", "Bearer "+e.b)
		r, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		raw, _ := io.ReadAll(r.Body)
		r.Body.Close()
		require.Less(t, r.StatusCode, 300, "B uploads a subtitle: %s", raw)
		m := hzJSON(t, raw)
		var sid string
		for _, k := range []string{"id", "subtitle_id"} {
			if v, ok := m[k]; ok {
				sid = fmt.Sprint(v)
			}
		}
		if sid == "" {
			if d, ok := m["data"].(map[string]interface{}); ok {
				if v, ok := d["id"]; ok {
					sid = fmt.Sprint(v)
				}
			}
		}
		if sid == "" {
			if d, ok := m["subtitle"].(map[string]interface{}); ok {
				if v, ok := d["id"]; ok {
					sid = fmt.Sprint(v)
				}
			}
		}
		require.NotEmpty(t, sid, "subtitle id in %s", raw)
		e.denied("subtitle delete by A", "DELETE", "/api/v1/subtitles/"+sid, e.a, nil)
		e.t.Run("state/subtitle survives", func(t *testing.T) {
			_, l := c.do("GET", fmt.Sprintf("/api/v1/subtitles/media/%d", m0), e.b, nil)
			assert.Contains(t, string(l), fmt.Sprintf(`"id":"%s"`, sid), "B's subtitle survives A's delete: %s", l)
		})
		e.ok("subtitle delete by owner B", "DELETE", "/api/v1/subtitles/"+sid, e.b, nil)
	})

	// ---- conversion jobs (owner-scoped; users holding conversion.*) ---------------------------------------------------------
	t.Run("conversion", func(t *testing.T) {
		data := filepath.Join(e.dir, "data")
		_ = os.MkdirAll(data, 0o755)
		src := filepath.Join(data, "hzmark-b-source.wav")
		require.NoError(t, os.WriteFile(src, []byte("RIFF....WAVEfmt "), 0o644))
		jid := c.id("POST", "/api/v1/conversion/jobs", e.cb, map[string]interface{}{
			"source_path": src, "target_path": filepath.Join(data, "hzmark-b-target.flac"), "source_format": "wav", "target_format": "flac",
			"conversion_type": "audio", "quality": "medium", "priority": 5}, 200)
		p := fmt.Sprintf("/api/v1/conversion/jobs/%d", jid)
		e.ok("conversion B reads own", "GET", p, e.cb, nil, "hzmark-b-source")
		e.ok("conversion admin reads", "GET", p, e.adm, nil, "hzmark-b-source")
		e.denied("conversion read", "GET", p, e.ca, nil, "hzmark-b-source")
		e.denied("conversion cancel", "POST", p+"/cancel", e.ca, nil)
		e.denied("conversion retry", "POST", p+"/retry", e.ca, nil)
		e.denied("conversion download", "GET", p+"/download", e.ca, nil)
		e.denied("conversion delete", "DELETE", p, e.ca, nil)
		e.noLeak("conversion list", "GET", "/api/v1/conversion/jobs", e.ca, "hzmark-b-source")
		e.ok("conversion list B sees own", "GET", "/api/v1/conversion/jobs", e.cb, nil, "hzmark-b-source")
		e.ok("conversion job survives", "GET", p, e.cb, nil, "hzmark-b-source")
	})

	// ---- error and crash reports (owner-scoped by caller id; users holding report.view + report.create) ----------------------
	// The default User role holds neither permission (a plain user gets 403 on every route here); a holder is still only
	// served its OWN reports: services/error_reporting_service.go refuses report.UserID != caller and the list and
	// statistics queries filter WHERE user_id = caller. B files reports, A (also a holder) must see none of them.
	t.Run("error_reports", func(t *testing.T) {
		const compB, sigB = "hzmark-b-component", "HZSIGB"
		eid := c.id("POST", "/api/v1/errors/report", e.rb, map[string]interface{}{"level": "error", "message": "hzmark-b-error", "component": compB}, 200)
		cid := c.id("POST", "/api/v1/errors/crash", e.rb, map[string]interface{}{"signal": sigB, "message": "hzmark-b-crash"}, 200)
		c.id("POST", "/api/v1/errors/report", e.ra, map[string]interface{}{"level": "error", "message": "hzmark-a-error", "component": "hzmark-a-component"}, 200)
		c.id("POST", "/api/v1/errors/crash", e.ra, map[string]interface{}{"signal": "HZSIGA", "message": "hzmark-a-crash"}, 200)
		ep := fmt.Sprintf("/api/v1/errors/reports/%d", eid)
		cp := fmt.Sprintf("/api/v1/errors/crashes/%d", cid)
		e.ok("error report B reads own", "GET", ep, e.rb, nil, "hzmark-b-error")
		e.ok("error report admin-less B list", "GET", "/api/v1/errors/reports", e.rb, nil, "hzmark-b-error")
		e.ok("crash report B reads own", "GET", cp, e.rb, nil, "hzmark-b-crash")
		e.ok("crash report B list", "GET", "/api/v1/errors/crashes", e.rb, nil, "hzmark-b-crash")
		e.ok("error statistics B sees own", "GET", "/api/v1/errors/statistics", e.rb, nil, compB)
		e.ok("crash statistics B sees own", "GET", "/api/v1/errors/crash-statistics", e.rb, nil, sigB)
		e.denied("error report read", "GET", ep, e.ra, nil, "hzmark-b-error")
		e.denied("crash report read", "GET", cp, e.ra, nil, "hzmark-b-crash")
		// HAUTHZ-F3 (fixed): a foreign and a missing id answer the SAME 404 with the SAME body as the GET routes (no oracle).
		for _, kind := range []string{"reports", "crashes"} {
			e.t.Run("denied-write/"+kind+" status by A", func(t *testing.T) {
				id := eid
				if kind == "crashes" {
					id = cid
				}
				foreign, fraw := c.do("PUT", fmt.Sprintf("/api/v1/errors/%s/%d/status", kind, id), e.ra, map[string]interface{}{"status": "resolved"})
				missing, mraw := c.do("PUT", fmt.Sprintf("/api/v1/errors/%s/987654321/status", kind), e.ra, map[string]interface{}{"status": "resolved"})
				t.Logf("HZ|%s status foreign|%d|REFUSED-404", kind, foreign)
				assert.Equal(t, 404, foreign, "A's status update on B's report must be 404: %s", fraw)
				assert.Equal(t, 404, missing, "a missing id must be the same 404: %s", mraw)
				assert.Equal(t, string(mraw), string(fraw), "foreign and missing bodies identical (no oracle)")
				getCode, getRaw := c.do("GET", fmt.Sprintf("/api/v1/errors/%s/%d", kind, id), e.ra, nil)
				assert.Equal(t, getCode, foreign, "PUT refusal matches the GET refusal code")
				assert.Equal(t, string(getRaw), string(fraw), "PUT refusal body matches the GET refusal body")
			})
		}
		// positive controls (HAUTHZ-FU2-M2): holder A sees its OWN report in list and statistics, so the no-leak checks below are not vacuous
		e.ok("error report A list sees own", "GET", "/api/v1/errors/reports", e.ra, nil, "hzmark-a-error")
		e.ok("crash report A list sees own", "GET", "/api/v1/errors/crashes", e.ra, nil, "hzmark-a-crash")
		e.ok("error statistics A sees own", "GET", "/api/v1/errors/statistics", e.ra, nil, "hzmark-a-component")
		e.ok("crash statistics A sees own", "GET", "/api/v1/errors/crash-statistics", e.ra, nil, "HZSIGA")
		e.noLeak("error report list", "GET", "/api/v1/errors/reports", e.ra, "hzmark-b-error")
		e.noLeak("crash report list", "GET", "/api/v1/errors/crashes", e.ra, "hzmark-b-crash")
		e.noLeak("error statistics", "GET", "/api/v1/errors/statistics", e.ra, compB)
		e.noLeak("crash statistics", "GET", "/api/v1/errors/crash-statistics", e.ra, sigB)
		// HAUTHZ-FU2-M1: numeric aggregates of A equal A's OWN filings (1 error at level error, 1 crash); B's rows are not counted
		e.t.Run("noleak/statistics numeric aggregates", func(t *testing.T) {
			code, raw := e.c.do("GET", "/api/v1/errors/statistics", e.ra, nil)
			require.Equal(t, 200, code, "%s", raw)
			m := hzJSON(t, raw)
			assert.EqualValues(t, 1, m["total_errors"], "A's total_errors counts only A's filing: %s", raw)
			lv, _ := m["errors_by_level"].(map[string]interface{})
			assert.EqualValues(t, 1, lv["error"], "A's by-level counts only A's filing: %s", raw)
			assert.Len(t, lv, 1, "by-level has only A's level: %s", raw)
			code, raw = e.c.do("GET", "/api/v1/errors/crash-statistics", e.ra, nil)
			require.Equal(t, 200, code, "%s", raw)
			m = hzJSON(t, raw)
			assert.EqualValues(t, 1, m["total_crashes"], "A's total_crashes counts only A's filing: %s", raw)
		})
		e.t.Run("state/B reports intact", func(t *testing.T) {
			_, raw := c.do("GET", ep, e.rb, nil)
			assert.NotContains(t, string(raw), `"status":"resolved"`, "A's status attempt did not change B's error report: %s", raw)
			assert.Contains(t, string(raw), "hzmark-b-error")
			_, raw = c.do("GET", cp, e.rb, nil)
			assert.NotContains(t, string(raw), `"status":"resolved"`, "A's status attempt did not change B's crash report: %s", raw)
			// positive control: the owner CAN resolve its own report, so the unchanged state above is not an inert route
			code, _ := c.do("PUT", ep+"/status", e.rb, map[string]interface{}{"status": "resolved"})
			require.Less(t, code, 300)
			_, raw = c.do("GET", ep, e.rb, nil)
			assert.Contains(t, string(raw), `"status":"resolved"`, "owner B resolves its own report: %s", raw)
			// HAUTHZ-FU2-M3: the owner resolves its own CRASH too
			code, _ = c.do("PUT", cp+"/status", e.rb, map[string]interface{}{"status": "resolved"})
			require.Less(t, code, 300)
			_, raw = c.do("GET", cp, e.rb, nil)
			assert.Contains(t, string(raw), `"status":"resolved"`, "owner B resolves its own crash: %s", raw)
		})
	})

	// ---- users: profile of another account (permission gated; A has none) ---------------------------------------------------
	t.Run("users", func(t *testing.T) {
		u := "/api/v1/users/" + bID
		e.denied("user read", "GET", u, e.a, nil, e.bEmail)
		e.denied("user update", "PUT", u, e.a, map[string]interface{}{"first_name": "hzhijack"})
		e.denied("user delete", "DELETE", u, e.a, nil)
		e.denied("user reset-password", "POST", u+"/reset-password", e.a, map[string]interface{}{"new_password": "Hijack-Value-Strong-1234"})
		e.denied("user lock", "POST", u+"/lock", e.a, nil)
		e.denied("user unlock", "POST", u+"/unlock", e.a, nil)
		e.denied("admin user update", "PUT", "/api/v1/admin/users/"+bID, e.a, map[string]interface{}{"role_id": 1})
		for _, p := range []string{"/api/v1/users/me", "/api/v1/auth/me", "/api/v1/auth/profile", "/api/v1/auth/permissions"} {
			e.noLeak("self view "+p, "GET", p, e.a, e.bEmail, `"username":"`+e.bName+`"`)
		}
		e.ok("user B reads self", "GET", "/api/v1/users/me", e.b, nil, e.bEmail)
		e.t.Run("state/B intact", func(t *testing.T) {
			tok := c.login("hzuserb", "Contract-Value-Strong-77")
			assert.NotEmpty(t, tok, "B can still log in with its own password after A's attempts")
			_, raw := c.do("GET", "/api/v1/users/me", e.b, nil)
			assert.NotContains(t, string(raw), "hzhijack")
		})
	})
}

// TestHorizontalAuthz_MediaFavoriteFlag covers finding HAUTHZ-F2 (CAT-HAUTHZ-1), FIXED by CAT-HAUTHZ-F2: PUT /api/v1/media/:id/favorite
// used to update media_items.is_favorite, ONE row per item shared by every user, and GET /api/v1/media/:id returned it to everybody.
// The flag is now the caller's own user_favorites row. B's favorite is invisible to A, A's toggle cannot change B's, B keeps its own.
func TestHorizontalAuthz_MediaFavoriteFlag(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs the server binary")
	}
	e := hzSetup(t)
	c := e.c
	m1 := e.media[1]
	t.Run("noleak/media favorite flag is per user", func(t *testing.T) {
		code, raw := c.do("PUT", fmt.Sprintf("/api/v1/media/%d/favorite", m1), e.b, map[string]bool{"favorite": true})
		require.Equal(t, 200, code, "%s", raw)
		_, got := c.do("GET", fmt.Sprintf("/api/v1/media/%d", m1), e.a, nil)
		t.Logf("HZ|media favorite flag|%d|BODY-CHECKED", code)
		assert.Equal(t, false, hzJSON(t, got)["is_favorite"], "B's favorite flag must not be visible to A: %s", got)
		_, got = c.do("GET", fmt.Sprintf("/api/v1/media/%d", m1), e.b, nil)
		assert.Equal(t, true, hzJSON(t, got)["is_favorite"], "B must still see its own favorite: %s", got)
	})
	t.Run("noleak/media favorite A toggle-off keeps B's", func(t *testing.T) {
		code, raw := c.do("PUT", fmt.Sprintf("/api/v1/media/%d/favorite", m1), e.a, map[string]bool{"favorite": false})
		require.Equal(t, 200, code, "%s", raw)
		_, got := c.do("GET", fmt.Sprintf("/api/v1/media/%d", m1), e.b, nil)
		t.Logf("HZ|media favorite A toggle|%d|BODY-CHECKED", code)
		assert.Equal(t, true, hzJSON(t, got)["is_favorite"], "A's toggle must not clear B's favorite: %s", got)
	})
}
