package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os/exec"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"catalogizer/config"
	"catalogizer/database"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Real-binary proof of smart collections and smart playlists: the real server
// binary on a real migrated SQLCipher database, real JWT logins, rows seeded
// through the real database layer. No mocks, stubs or fixture servers.

type smartSeed struct {
	alpha, beta, gamma, delta, epsilon, quote int64
}

func seedSmartCatalogue(t *testing.T, dir string, aliceID int) smartSeed {
	t.Helper()
	db, err := database.NewConnection(&config.DatabaseConfig{Type: "sqlite", Path: filepath.Join(dir, "catalog.db"),
		EncryptionKeyFile: filepath.Join(dir, "keys", "db.key"), MaxOpenConnections: 2})
	require.NoError(t, err)
	defer db.Close()
	ts := func(d time.Duration) string { return time.Now().UTC().Add(-d).Format("2006-01-02 15:04:05") }
	_, err = db.Exec(`INSERT INTO storage_roots (name, protocol, path, enabled) VALUES ('smart-root','local','/smart',1)`)
	require.NoError(t, err)
	var root int64
	require.NoError(t, db.QueryRow(`SELECT id FROM storage_roots WHERE name='smart-root'`).Scan(&root))
	typeID := func(name string) int64 {
		var id int64
		require.NoError(t, db.QueryRow(`SELECT id FROM media_types WHERE name = ?`, name).Scan(&id))
		return id
	}
	item := func(mt, title string, year interface{}, rating interface{}, genre string, age time.Duration, path string, size int64) int64 {
		res, err := db.Exec(`INSERT INTO media_items (media_type_id, title, year, rating, genre, status, first_detected, last_updated) VALUES (?,?,?,?,?,'detected',?,?)`,
			typeID(mt), title, year, rating, genre, ts(age), ts(age))
		require.NoError(t, err)
		id, _ := res.LastInsertId()
		fr, err := db.Exec(`INSERT INTO files (storage_root_id, path, name, size, is_directory, modified_at) VALUES (?,?,?,?,0,?)`,
			root, path, filepath.Base(path), size, ts(age))
		require.NoError(t, err)
		fid, _ := fr.LastInsertId()
		_, err = db.Exec(`INSERT INTO media_files (media_item_id, file_id, quality_info) VALUES (?,?,?)`, id, fid, "1080p")
		require.NoError(t, err)
		return id
	}
	var s smartSeed
	s.alpha = item("movie", "Alpha", 2019, 8.5, "Action, Sci-Fi", 2*24*time.Hour, "/movies/alpha.mkv", 4_000_000_000)
	s.beta = item("movie", "Beta", 2005, 6.0, "Comedy", 40*24*time.Hour, "/movies/old/beta.mkv", 1_000_000_000)
	s.gamma = item("song", "Gamma", 2021, nil, "Pop", 5*24*time.Hour, "/music/gamma.mp3", 5_000_000)
	s.delta = item("book", "Delta", 2010, 7.0, "History", 100*24*time.Hour, "/books/delta.pdf", 2_000_000)
	s.epsilon = item("tv_show", "Epsilon", 2022, 9.0, "Drama", 1*24*time.Hour, "/tv/epsilon", 3_000_000_000)
	s.quote = item("movie", "O'Brien; DROP TABLE media_items;--", 2000, 5.0, "Crime", 3*24*time.Hour, "/movies/100%_odd.mkv", 10)
	_, err = db.Exec(`INSERT INTO user_metadata (media_item_id, user_id, tags, favorite) VALUES (?,?,?,1)`, s.alpha, aliceID, "weekend,fav-night")
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO user_metadata (media_item_id, user_id, tags, favorite) VALUES (?,?,?,0)`, s.beta, aliceID, "boring")
	require.NoError(t, err)
	return s
}

func idsOf(t *testing.T, raw []byte) []int64 {
	t.Helper()
	var m struct {
		Items []struct {
			ID int64 `json:"id"`
		} `json:"items"`
	}
	require.NoError(t, json.Unmarshal(raw, &m), "%s", raw)
	out := make([]int64, 0, len(m.Items))
	for _, it := range m.Items {
		out = append(out, it.ID)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func sorted(v ...int64) []int64 {
	sort.Slice(v, func(i, j int) bool { return v[i] < v[j] })
	return v
}

func TestSmartCollectionsAndPlaylists_RealBinary(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs the server binary")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "catalog-api")
	if out, err := exec.Command("go", "build", "-buildvcs=false", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	b := &binEnv{t: t, bin: bin, dir: dir, port: port}
	const adminPw = "Op3rator-Supplied-Value-9x"
	cmd, done, out := b.start("GIN_MODE=release", "ADMIN_PASSWORD="+adminPw)
	require.True(t, b.waitHealthy(done), "server must come up:\n%s", out.String())
	defer b.stop(cmd, done)
	c := &apiClient{t: t, base: fmt.Sprintf("http://127.0.0.1:%d", port)}
	adm := c.login("admin", adminPw)
	mk := func(name string) (string, int) {
		id := c.id("POST", "/api/v1/users", adm, map[string]interface{}{"username": name, "email": name + "@example.invalid", "password": "Contract-Value-Strong-77", "role_id": 2}, 201)
		return c.login(name, "Contract-Value-Strong-77"), id
	}
	alice, aliceID := mk("smartalice")
	bob, _ := mk("smartbob")
	s := seedSmartCatalogue(t, dir, aliceID)

	get := func(path, tok string, want int) []byte {
		code, raw := c.do("GET", path, tok, nil)
		require.Equal(t, want, code, "GET %s: %s", path, raw)
		return raw
	}
	post := func(path, tok string, body interface{}, want int) []byte {
		code, raw := c.do("POST", path, tok, body)
		require.Equal(t, want, code, "POST %s: %s", path, raw)
		return raw
	}
	put := func(path, tok string, body interface{}, want int) []byte {
		code, raw := c.do("PUT", path, tok, body)
		require.Equal(t, want, code, "PUT %s: %s", path, raw)
		return raw
	}
	rule := func(field, op string, v interface{}, cond ...string) map[string]interface{} {
		m := map[string]interface{}{"field": field, "operator": op, "value": v}
		if len(cond) > 0 {
			m["condition"] = cond[0]
		}
		return m
	}
	mkColl := func(tok string, rules ...interface{}) int {
		return c.id("POST", "/api/v1/collections", tok, map[string]interface{}{"name": "smart-" + fmt.Sprint(time.Now().UnixNano()), "is_smart": true, "smart_rules": rules}, 201)
	}
	items := func(id int, tok string, q string) []int64 {
		return idsOf(t, get(fmt.Sprintf("/api/v1/collections/%d/items%s", id, q), tok, 200))
	}

	// --- creation is real, rules round-trip, a plain collection stays plain -------------------
	raw := post("/api/v1/collections", alice, map[string]interface{}{"name": "high rated movies", "is_smart": true,
		"smart_rules": []interface{}{rule("media_type", "equals", "movie"), rule("rating", "greater_than", 6.5)}}, 201)
	var created map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &created))
	assert.Equal(t, true, created["is_smart"], "%s", raw)
	assert.Len(t, created["smart_rules"], 2, "%s", raw)
	assert.EqualValues(t, aliceID, created["owner_id"], "%s", raw)
	smartID := int(created["id"].(float64))
	assert.Equal(t, sorted(s.alpha), items(smartID, alice, ""), "movies rated > 6.5")

	manual := c.id("POST", "/api/v1/collections", alice, map[string]interface{}{"name": "plain"}, 201)
	var mraw map[string]interface{}
	require.NoError(t, json.Unmarshal(get(fmt.Sprintf("/api/v1/collections/%d", manual), alice, 200), &mraw))
	assert.Equal(t, false, mraw["is_smart"])
	assert.Empty(t, items(manual, alice, ""), "an empty manual collection has no items")

	// --- the collection is live: a new matching item appears without any refresh --------------
	func() {
		db, err := database.NewConnection(&config.DatabaseConfig{Type: "sqlite", Path: filepath.Join(dir, "catalog.db"),
			EncryptionKeyFile: filepath.Join(dir, "keys", "db.key"), MaxOpenConnections: 2})
		require.NoError(t, err)
		defer db.Close()
		var mt int64
		require.NoError(t, db.QueryRow(`SELECT id FROM media_types WHERE name='movie'`).Scan(&mt))
		res, err := db.Exec(`INSERT INTO media_items (media_type_id, title, year, rating, status) VALUES (?, 'Zeta', 2024, 9.9, 'detected')`, mt)
		require.NoError(t, err)
		zeta, _ := res.LastInsertId()
		assert.Equal(t, sorted(s.alpha, zeta), items(smartID, alice, ""), "live evaluation picks up the new row")
	}()

	// --- field/operator coverage ---------------------------------------------------------------
	cases := []struct {
		name  string
		rules []interface{}
		want  []int64
	}{
		{"year range", []interface{}{rule("year", "between", []int{2005, 2010})}, sorted(s.beta, s.delta)},
		{"genre contains (case-insens.)", []interface{}{rule("genre", "contains", "sci-fi")}, sorted(s.alpha)},
		{"alias video", []interface{}{rule("media_type", "equals", "video")}, nil}, // checked separately below
		{"size greater than 2.5e9", []interface{}{rule("size", "greater_than", 2500000000)}, sorted(s.alpha, s.epsilon)},
		{"path prefix", []interface{}{rule("path", "starts_with", "/movies/old")}, sorted(s.beta)},
		{"added within 3 days", []interface{}{rule("added", "in_last_days", 3)}, nil}, // checked separately below
		{"OR group", []interface{}{rule("title", "equals", "Alpha"), rule("title", "equals", "Delta", "or")}, sorted(s.alpha, s.delta)},
		{"AND binds tighter than OR", []interface{}{rule("title", "equals", "Alpha"), rule("year", "equals", 2005, "or"), rule("rating", "greater_than", 100, "and")}, sorted(s.alpha)},
		{"nested", []interface{}{map[string]interface{}{"field": "media_type", "operator": "equals", "value": "movie",
			"nested_rules": []interface{}{rule("year", "less_than", 2010)}}}, sorted(s.beta, s.quote)},
		{"numeric string value (UI input)", []interface{}{rule("year", "equals", "2022")}, sorted(s.epsilon)},
		{"not_in", []interface{}{rule("genre", "not_in", []string{"Action, Sci-Fi", "Comedy", "Pop", "History", "Drama"}), rule("year", "less_than", 2001)}, sorted(s.quote)},
		{"SQL injection text stays text", []interface{}{rule("title", "equals", "x' OR '1'='1")}, []int64{}},
		{"LIKE wildcards are literal", []interface{}{rule("title", "contains", "%")}, []int64{}},
		{"underscore is literal in path", []interface{}{rule("path", "contains", "100%_odd")}, sorted(s.quote)},
		{"quote in value", []interface{}{rule("title", "equals", "O'Brien; DROP TABLE media_items;--")}, sorted(s.quote)},
		{"user tags are the caller's own", []interface{}{rule("tags", "contains", "fav-night")}, sorted(s.alpha)},
		{"user favourite", []interface{}{rule("favorite", "equals", true)}, sorted(s.alpha)},
	}
	for _, tc := range cases {
		if tc.want == nil {
			continue
		}
		id := mkColl(alice, tc.rules...)
		got := items(id, alice, "")
		if got == nil {
			got = []int64{}
		}
		assert.Equal(t, tc.want, got, tc.name)
	}
	vid := mkColl(alice, rule("media_type", "equals", "video"))
	assert.Equal(t, sorted(s.alpha, s.beta, s.epsilon, s.quote), func() []int64 {
		all := items(vid, alice, "")
		var keep []int64
		for _, id := range all { // ignore Zeta inserted above
			if id <= s.quote {
				keep = append(keep, id)
			}
		}
		return keep
	}(), "video alias expands to movie/tv types")
	recent := mkColl(alice, rule("added", "in_last_days", 3))
	assert.Subset(t, items(recent, alice, ""), sorted(s.alpha, s.epsilon, s.quote), "added in last 3 days")
	assert.NotContains(t, items(recent, alice, ""), s.beta)
	assert.NotContains(t, items(recent, alice, ""), s.delta)
	// the table the injection text names is still there
	assert.Contains(t, items(mkColl(alice, rule("title", "contains", "Alpha")), alice, ""), s.alpha)

	// --- validation: every malformed rule is a 400 and creates nothing -------------------------
	before := idsOfList(t, get("/api/v1/collections?limit=200", adm, 200))
	badBodies := map[string]interface{}{
		"unknown field":      []interface{}{rule("password_hash", "equals", "x")},
		"unknown operator":   []interface{}{rule("title", "matches_regex", ".*")},
		"op wrong for field": []interface{}{rule("year", "contains", "20")},
		"wrong value type":   []interface{}{rule("year", "equals", "abc")},
		"unknown key":        []interface{}{map[string]interface{}{"field": "title", "operator": "equals", "value": "a", "sql": "1=1"}},
		"missing value":      []interface{}{map[string]interface{}{"field": "title", "operator": "equals"}},
		"bad media type":     []interface{}{rule("media_type", "equals", "spaceship")},
		"bad condition":      []interface{}{rule("title", "equals", "a"), rule("title", "equals", "b", "xor")},
		"bad date":           []interface{}{rule("added", "greater_than", "yesterday")},
		"empty list":         []interface{}{},
		"not an object":      []interface{}{"title = 'x'"},
	}
	for name, rules := range badBodies {
		code, raw := c.do("POST", "/api/v1/collections", alice, map[string]interface{}{"name": "bad-" + name, "is_smart": true, "smart_rules": rules})
		assert.Equal(t, 400, code, "%s: %s", name, raw)
	}
	deep := []interface{}{map[string]interface{}{"field": "year", "operator": "equals", "value": 1, "nested_rules": []interface{}{
		map[string]interface{}{"field": "year", "operator": "equals", "value": 1, "nested_rules": []interface{}{
			map[string]interface{}{"field": "year", "operator": "equals", "value": 1, "nested_rules": []interface{}{rule("year", "equals", 1)}}}}}}}
	code, raw2 := c.do("POST", "/api/v1/collections", alice, map[string]interface{}{"name": "deep", "is_smart": true, "smart_rules": deep})
	assert.Equal(t, 400, code, "nesting depth: %s", raw2)
	many := make([]interface{}, 0, 30)
	for i := 0; i < 30; i++ {
		many = append(many, rule("year", "equals", 2000+i, "or"))
	}
	code, raw2 = c.do("POST", "/api/v1/collections", alice, map[string]interface{}{"name": "many", "is_smart": true, "smart_rules": many})
	assert.Equal(t, 400, code, "too many rules: %s", raw2)
	code, raw2 = c.do("POST", "/api/v1/collections", alice, map[string]interface{}{"name": "nosmartflag", "is_smart": true})
	assert.Equal(t, 400, code, "is_smart without rules: %s", raw2)
	code, raw2 = c.do("POST", "/api/v1/collections", alice, map[string]interface{}{"name": "contradiction", "is_smart": false, "smart_rules": []interface{}{rule("year", "equals", 1)}})
	assert.Equal(t, 400, code, "rules with is_smart false: %s", raw2)
	assert.Equal(t, before, idsOfList(t, get("/api/v1/collections?limit=200", adm, 200)), "refused requests create nothing")

	// --- pagination and the result cap ---------------------------------------------------------
	all := mkColl(alice, rule("year", "greater_than", 0))
	page := get(fmt.Sprintf("/api/v1/collections/%d/items?limit=2&offset=1", all), alice, 200)
	var pm map[string]interface{}
	require.NoError(t, json.Unmarshal(page, &pm))
	assert.Len(t, pm["items"], 2, "%s", page)
	assert.EqualValues(t, 7, pm["total"], "6 seeded + Zeta, %s", page)
	capped := get(fmt.Sprintf("/api/v1/collections/%d/items?limit=100000", all), alice, 200)
	require.NoError(t, json.Unmarshal(capped, &pm))
	assert.EqualValues(t, 500, pm["limit"], "limit capped at 500")
	byPage := get(fmt.Sprintf("/api/v1/collections/%d/items?page=2&limit=3", all), alice, 200)
	require.NoError(t, json.Unmarshal(byPage, &pm))
	assert.EqualValues(t, 3, pm["offset"], "page=2&limit=3 is offset 3: %s", byPage)

	// --- authorization -------------------------------------------------------------------------
	c.do("GET", "/api/v1/collections", "", nil)
	code, _ = c.do("GET", fmt.Sprintf("/api/v1/collections/%d/items", smartID), "", nil)
	assert.Equal(t, 401, code, "unauthenticated")
	code, _ = c.do("POST", "/api/v1/collections", "", map[string]interface{}{"name": "x", "is_smart": true, "smart_rules": []interface{}{rule("year", "equals", 1)}})
	assert.Equal(t, 401, code)
	// alice's collection is private: bob cannot see, list, read items of, edit or delete it
	code, _ = c.do("GET", fmt.Sprintf("/api/v1/collections/%d", smartID), bob, nil)
	assert.Equal(t, 404, code, "private collection of another user")
	code, _ = c.do("GET", fmt.Sprintf("/api/v1/collections/%d/items", smartID), bob, nil)
	assert.Equal(t, 404, code)
	code, _ = c.do("PUT", fmt.Sprintf("/api/v1/collections/%d", smartID), bob, map[string]interface{}{"name": "hijack"})
	assert.Equal(t, 404, code)
	code, _ = c.do("DELETE", fmt.Sprintf("/api/v1/collections/%d", smartID), bob, nil)
	assert.Equal(t, 404, code)
	assert.NotContains(t, idsOfList(t, get("/api/v1/collections?limit=200", bob, 200)), int64(smartID))
	assert.Contains(t, idsOfList(t, get("/api/v1/collections?limit=200", alice, 200)), int64(smartID))
	// public: bob may read, never edit
	put(fmt.Sprintf("/api/v1/collections/%d", smartID), alice, map[string]interface{}{"is_public": true}, 200)
	assert.Equal(t, sorted(s.alpha), filterLE(items(smartID, bob, ""), s.quote), "bob reads the public collection")
	code, _ = c.do("PUT", fmt.Sprintf("/api/v1/collections/%d", smartID), bob, map[string]interface{}{"name": "hijack"})
	assert.Equal(t, 403, code, "public but not owner")
	code, _ = c.do("DELETE", fmt.Sprintf("/api/v1/collections/%d", smartID), bob, nil)
	assert.Equal(t, 403, code)
	// user-scoped fields are evaluated for the VIEWER: bob must not see alice's tags/favourites
	tagged := mkColl(alice, rule("tags", "contains", "fav-night"))
	put(fmt.Sprintf("/api/v1/collections/%d", tagged), alice, map[string]interface{}{"is_public": true}, 200)
	assert.Equal(t, sorted(s.alpha), items(tagged, alice, ""))
	assert.Empty(t, items(tagged, bob, ""), "bob's own tags do not match: alice's private tags are not leaked")

	// --- update rules, convert refusal, admin override, delete ---------------------------------
	put(fmt.Sprintf("/api/v1/collections/%d", smartID), alice, map[string]interface{}{"smart_rules": []interface{}{rule("title", "equals", "Delta")}}, 200)
	assert.Equal(t, sorted(s.delta), items(smartID, alice, ""), "rules updated and re-evaluated")
	code, raw2 = c.do("PUT", fmt.Sprintf("/api/v1/collections/%d", smartID), alice, map[string]interface{}{"smart_rules": []interface{}{rule("nope", "equals", 1)}})
	assert.Equal(t, 400, code, "%s", raw2)
	assert.Equal(t, sorted(s.delta), items(smartID, alice, ""), "a refused update changes nothing")
	code, raw2 = c.do("PUT", fmt.Sprintf("/api/v1/collections/%d", smartID), alice, map[string]interface{}{"is_smart": false})
	assert.Equal(t, 400, code, "smart cannot silently become manual: %s", raw2)
	code, raw2 = c.do("PUT", fmt.Sprintf("/api/v1/collections/%d", manual), alice, map[string]interface{}{"smart_rules": []interface{}{rule("year", "equals", 1)}})
	assert.Equal(t, 400, code, "manual collection cannot take rules via PUT: %s", raw2)
	put(fmt.Sprintf("/api/v1/collections/%d", smartID), adm, map[string]interface{}{"description": "by admin"}, 200)
	code, _ = c.do("DELETE", fmt.Sprintf("/api/v1/collections/%d", smartID), alice, nil)
	assert.Equal(t, 200, code)
	code, _ = c.do("GET", fmt.Sprintf("/api/v1/collections/%d/items", smartID), alice, nil)
	assert.Equal(t, 404, code, "deleted")

	// --- smart playlists -----------------------------------------------------------------------
	praw := post("/api/v1/playlists", alice, map[string]interface{}{"name": "smart pl", "is_smart": true,
		"smart_rules": []interface{}{rule("media_type", "equals", "music"), rule("year", "greater_than", 2005, "or")}}, 201)
	var pl map[string]interface{}
	require.NoError(t, json.Unmarshal(praw, &pl))
	assert.Equal(t, true, pl["is_smart"], "%s", praw)
	plID := int(pl["id"].(float64))
	var got struct {
		Items      []map[string]interface{} `json:"items"`
		ItemsTotal int                      `json:"items_total"`
		IsSmart    bool                     `json:"is_smart"`
	}
	require.NoError(t, json.Unmarshal(get(fmt.Sprintf("/api/v1/playlists/%d", plID), alice, 200), &got))
	assert.True(t, got.IsSmart)
	assert.Equal(t, 5, got.ItemsTotal, "gamma (music), alpha 2019, delta 2010, epsilon 2022 and the item added live earlier; beta 2005 and the 2000 quote item are out")
	var plIDs []int64
	for _, it := range got.Items {
		plIDs = append(plIDs, int64(it["entity_id"].(float64)))
		assert.NotEmpty(t, it["entity_type"])
	}
	assert.Contains(t, plIDs, s.gamma)
	assert.NotContains(t, plIDs, s.beta, "beta is 2005 movie")
	// manual item management on a smart playlist is refused
	code, raw2 = c.do("POST", fmt.Sprintf("/api/v1/playlists/%d/items", plID), alice, map[string]interface{}{"entity_id": s.alpha, "entity_type": "movie"})
	assert.Equal(t, 409, code, "%s", raw2)
	// invalid rules: 400, nothing created
	for name, rules := range badBodies {
		code, raw2 = c.do("POST", "/api/v1/playlists", alice, map[string]interface{}{"name": "bad-" + name, "is_smart": true, "smart_rules": rules})
		assert.Equal(t, 400, code, "playlist %s: %s", name, raw2)
	}
	// update rules
	put(fmt.Sprintf("/api/v1/playlists/%d", plID), alice, map[string]interface{}{"smart_rules": []interface{}{rule("title", "equals", "Epsilon")}}, 200)
	require.NoError(t, json.Unmarshal(get(fmt.Sprintf("/api/v1/playlists/%d", plID), alice, 200), &got))
	require.Len(t, got.Items, 1)
	assert.EqualValues(t, s.epsilon, got.Items[0]["entity_id"])
	// another user can neither read nor edit a private smart playlist
	code, _ = c.do("GET", fmt.Sprintf("/api/v1/playlists/%d", plID), bob, nil)
	assert.NotEqual(t, 200, code)
	code, _ = c.do("PUT", fmt.Sprintf("/api/v1/playlists/%d", plID), bob, map[string]interface{}{"smart_rules": []interface{}{rule("year", "greater_than", 0)}})
	assert.NotEqual(t, 200, code)
	// a plain playlist is unchanged
	plain := post("/api/v1/playlists", alice, map[string]interface{}{"name": "plain pl"}, 201)
	var pp map[string]interface{}
	require.NoError(t, json.Unmarshal(plain, &pp))
	assert.NotEqual(t, true, pp["is_smart"])
	post(fmt.Sprintf("/api/v1/playlists/%d/items", int(pp["id"].(float64))), alice, map[string]interface{}{"entity_id": s.alpha, "entity_type": "movie"}, 201)
}

func idsOfList(t *testing.T, raw []byte) []int64 { return idsOf(t, raw) }

func filterLE(in []int64, max int64) []int64 {
	out := []int64{}
	for _, v := range in {
		if v <= max {
			out = append(out, v)
		}
	}
	return out
}
