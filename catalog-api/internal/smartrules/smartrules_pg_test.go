package smartrules_test

// Dialect-parity tests: the same rule matrix runs on SQLite and on a throwaway PostgreSQL started on
// demand (rootless podman, loopback, random port, random password in a 0600 env file that is never
// printed, container removed afterwards). Without a container runtime the PostgreSQL leg SKIPs honestly.
// Review round 2 (NEW-1): every genre rule failed on PostgreSQL with "COALESCE types boolean and integer
// cannot be matched" because only the SQLite leg existed.

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"catalogizer/config"
	"catalogizer/database"
	mediamodels "catalogizer/internal/media/models"
	"catalogizer/internal/smartrules"
	"catalogizer/repository"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

// pgDB starts postgres:15-alpine and returns a migrated database, or skips.
func pgDB(t *testing.T) *database.DB {
	t.Helper()
	rt, err := exec.LookPath("podman")
	if err != nil {
		t.Skip("no container runtime (podman): PostgreSQL leg not run")
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := ln.Addr().(*net.TCPAddr).Port
	require.NoError(t, ln.Close())
	raw := make([]byte, 16)
	_, err = rand.Read(raw)
	require.NoError(t, err)
	pw := hex.EncodeToString(raw)
	envFile := filepath.Join(t.TempDir(), "pg.env")
	require.NoError(t, os.WriteFile(envFile, []byte("POSTGRES_PASSWORD="+pw+"\nPOSTGRES_DB=smart\n"), 0o600))
	name := fmt.Sprintf("smartrules-pg-%d-%d", os.Getpid(), port)
	out, err := exec.Command(rt, "run", "-d", "--rm", "--name", name, "--env-file", envFile,
		"-p", fmt.Sprintf("127.0.0.1:%d:5432", port), "docker.io/library/postgres:15-alpine").CombinedOutput()
	if err != nil {
		t.Skipf("cannot start postgres container: %s", strings.TrimSpace(string(out))[:min(120, len(strings.TrimSpace(string(out))))])
	}
	t.Cleanup(func() { _ = exec.Command(rt, "rm", "-f", name).Run() })
	dsn := fmt.Sprintf("host=127.0.0.1 port=%d user=postgres password=%s dbname=smart sslmode=disable", port, pw)
	deadline := time.Now().Add(60 * time.Second)
	for {
		c, e := sql.Open("postgres", dsn)
		if e == nil {
			if e = c.Ping(); e == nil {
				c.Close()
				break
			}
			c.Close()
		}
		if time.Now().After(deadline) {
			t.Fatalf("postgres did not become ready")
		}
		time.Sleep(500 * time.Millisecond)
	}
	db, err := database.NewConnection(&config.DatabaseConfig{Type: "postgres", Host: "127.0.0.1", Port: port, Name: "smart",
		User: "postgres", Password: pw, SSLMode: "disable", MaxOpenConnections: 2})
	require.NoError(t, err)
	require.NoError(t, db.RunMigrations(context.Background()))
	t.Cleanup(func() { db.Close() })
	return db
}

func docTitles(t *testing.T, q smartrules.Querier, pg bool, doc string, uid int64) []string {
	t.Helper()
	var raws []json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(doc), &raws))
	rs, err := smartrules.Parse(raws)
	require.NoError(t, err, doc)
	items, _, err := smartrules.Evaluate(context.Background(), q, rs, smartrules.Options{UserID: uid, Postgres: pg}, 500, 0)
	if err != nil {
		return []string{"ERR:" + err.Error()}
	}
	out := []string{}
	for _, it := range items {
		out = append(out, it.Title)
	}
	sort.Strings(out)
	return out
}

func rule(field, op string, v interface{}) string {
	b, _ := json.Marshal([]map[string]interface{}{{"field": field, "operator": op, "value": v}})
	return string(b)
}

func seedEdge(t *testing.T, db *database.DB, pg bool) map[string]int64 {
	ctx := context.Background()
	act := "1"
	if pg {
		act = "TRUE"
	}
	_, err := db.Exec(`INSERT INTO users (username, email, password_hash, salt, role_id, is_active) VALUES ('u1','u1@x.invalid','h','s',2,` + act + `),('u2','u2@x.invalid','h','s',2,` + act + `)`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO storage_roots (name, protocol, path, enabled) VALUES ('r','local','/r',` + act + `)`)
	require.NoError(t, err)
	var movie int64
	require.NoError(t, db.QueryRow(`SELECT id FROM media_types WHERE name='movie'`).Scan(&movie))
	items := repository.NewMediaItemRepository(db)
	files := repository.NewMediaFileRepository(db)
	ids := map[string]int64{}
	add := func(title string, genre []string, qi *string) {
		id, err := items.Create(ctx, &mediamodels.MediaItem{MediaTypeID: movie, Title: title, Genre: genre, Status: "detected"})
		require.NoError(t, err)
		ids[title] = id
		fr, err := db.Exec(`INSERT INTO files (storage_root_id,path,name,size,is_directory,modified_at,deleted) VALUES (1,?,?,1,?,?,?)`, "/m/"+title, title, false, time.Now().UTC(), false)
		require.NoError(t, err)
		var fid int64
		require.NoError(t, db.QueryRow(`SELECT id FROM files WHERE path = ?`, "/m/"+title).Scan(&fid))
		_ = fr
		_, err = files.LinkFileToItem(ctx, id, fid, qi, nil, true)
		require.NoError(t, err)
	}
	qs := func(v interface{}) *string { b, _ := json.Marshal(v); s := string(b); return &s }
	p := func(s string) *string { return &s }
	add("Comma", []string{"a,b", "c"}, qs(&mediamodels.QualityInfo{QualityProfile: p(`4K "HDR"`)}))
	add("Sep", []string{`x","y`}, qs(&mediamodels.QualityInfo{QualityScore: 1})) // no profile
	add("Uni", []string{"Ação", "Ünïcode"}, nil)
	add("Back", []string{`C:\x`, "<b>&amp"}, qs(map[string]interface{}{"nested": map[string]string{"quality_profile": "1080p"}}))
	add("SciFi", []string{"Sci-Fi", "Fiction"}, qs(map[string]interface{}{"quality_profile": nil}))
	add("Empty", []string{}, nil)
	add("Nil", nil, nil)
	add("Inj", []string{"' OR 1=1 --", "100%_x"}, nil)

	return ids
}

func edgeCases(t *testing.T, db *database.DB, pg bool) {
	ctx := context.Background()
	ids := seedEdge(t, db, pg)
	// language/country are NULL for every row but Uni: negations must still match the NULL rows.
	_, err0 := db.Exec(`UPDATE media_items SET language = 'pt', country = 'BR' WHERE title = 'Uni'`)
	require.NoError(t, err0)
	type tc struct {
		doc  string
		want []string
	}
	cases := []tc{
		{rule("genre", "equals", "a,b"), []string{"Comma"}},
		{rule("genre", "equals", "a"), []string{}},
		{rule("genre", "equals", "c"), []string{"Comma"}},
		{rule("genre", "equals", `x","y`), []string{"Sep"}},
		{rule("genre", "equals", "y"), []string{}},
		{rule("genre", "contains", `"`), []string{"Sep"}},
		{rule("genre", "contains", `,`), []string{"Comma", "Sep"}},
		{rule("genre", "contains", `[`), []string{}},
		{rule("genre", "equals", "Ação"), []string{"Uni"}},
		{rule("genre", "equals", `C:\x`), []string{"Back"}},
		{rule("genre", "contains", `\`), []string{"Back"}},
		{rule("genre", "equals", "<b>&amp"), []string{"Back"}},
		{rule("genre", "starts_with", "fi"), []string{"SciFi"}},
		{rule("genre", "ends_with", "fi"), []string{"SciFi"}},
		{rule("genre", "starts_with", "sci"), []string{"SciFi"}},
		{rule("genre", "ends_with", "tion"), []string{"SciFi"}},
		{rule("genre", "equals", "' OR 1=1 --"), []string{"Inj"}},
		{rule("genre", "contains", "%"), []string{"Inj"}},
		{rule("genre", "contains", "_"), []string{"Inj"}},
		{rule("genre", "not_contains", "a"), []string{"Empty", "Inj", "Nil", "SciFi", "Sep"}},
		{rule("genre", "in", []string{"c", "fiction"}), []string{"Comma", "SciFi"}},
		{rule("genre", "not_in", []string{"c", "fiction"}), []string{"Back", "Empty", "Inj", "Nil", "Sep", "Uni"}},
		{rule("quality", "equals", `4k "hdr"`), []string{"Comma"}},
		{rule("quality", "contains", `"`), []string{"Comma"}},
		{rule("quality", "equals", "1080p"), []string{}}, // nested key: not reachable from the QualityInfo writer
		{rule("quality", "contains", "null"), []string{}},
		{rule("quality", "not_equals", "x"), []string{"Back", "Comma", "Empty", "Inj", "Nil", "SciFi", "Sep", "Uni"}},
		{rule("language", "equals", "PT"), []string{"Uni"}},
		{rule("language", "not_equals", "pt"), []string{"Back", "Comma", "Empty", "Inj", "Nil", "SciFi", "Sep"}},
		{rule("language", "not_in", []string{"pt", "zz"}), []string{"Back", "Comma", "Empty", "Inj", "Nil", "SciFi", "Sep"}},
		{rule("country", "not_equals", "br"), []string{"Back", "Comma", "Empty", "Inj", "Nil", "SciFi", "Sep"}},
		{rule("country", "not_in", []string{"br"}), []string{"Back", "Comma", "Empty", "Inj", "Nil", "SciFi", "Sep"}},
		{rule("country", "not_contains", "r"), []string{"Back", "Comma", "Empty", "Inj", "Nil", "SciFi", "Sep"}},
		{rule("title", "equals", "' OR 1=1 --"), []string{}},
		{rule("title", "contains", "%"), []string{}},
	}
	for _, c := range cases {
		got := docTitles(t, db, pg, c.doc, 1)
		if fmt.Sprint(got) != fmt.Sprint(c.want) {
			t.Errorf("%s: got %v want %v", c.doc, got, c.want)
		}
	}

	// favourites: user 2 favourite must not leak to user 1
	_, err := db.Exec(`INSERT INTO favorites (user_id, entity_type, entity_id, created_at) VALUES (2,'movie',?,CURRENT_TIMESTAMP)`, ids["Uni"])
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO favorites (user_id, entity_type, entity_id, created_at) VALUES (1,'playlist',?,CURRENT_TIMESTAMP)`, ids["Comma"])
	require.NoError(t, err)
	if got := docTitles(t, db, pg, rule("favorite", "equals", true), 1); len(got) != 0 {
		t.Errorf("favorite leak/entity type: user1 sees %v", got)
	}
	if got := docTitles(t, db, pg, rule("favorite", "equals", true), 2); fmt.Sprint(got) != "[Uni]" {
		t.Errorf("favorite user2 got %v", got)
	}
	// tags: user 2's tags must not leak to user 1
	um := repository.NewUserMetadataRepository(db)
	_, err = um.Create(ctx, &mediamodels.UserMetadata{MediaItemID: ids["Sep"], UserID: 2, Tags: []string{"secret"}})
	require.NoError(t, err)
	if got := docTitles(t, db, pg, rule("tags", "contains", "secret"), 1); len(got) != 0 {
		t.Errorf("tags leak: %v", got)
	}
	if got := docTitles(t, db, pg, rule("tags", "contains", "secret"), 2); fmt.Sprint(got) != "[Sep]" {
		t.Errorf("tags user2 got %v", got)
	}
}

func TestEdgeCases_SQLite(t *testing.T) { edgeCases(t, realDB(t), false) }

func TestEdgeCases_Postgres(t *testing.T) { edgeCases(t, pgDB(t), true) }

// matrixDocs enumerates every field x operator of the closed rule language with a valid value.
func matrixDocs() []string {
	vals := map[string][]struct {
		op string
		v  interface{}
	}{}
	text := []struct {
		op string
		v  interface{}
	}{{"equals", "a"}, {"not_equals", "a"}, {"contains", "a"}, {"not_contains", "a"}, {"starts_with", "a"}, {"ends_with", "a"},
		{"in", []string{"a", "b"}}, {"not_in", []string{"a", "b"}}}
	num := []struct {
		op string
		v  interface{}
	}{{"equals", 1999}, {"not_equals", 1999}, {"greater_than", 1999}, {"greater_or_equal", 1999}, {"less_than", 1999},
		{"less_or_equal", 1999}, {"between", []int{1, 2100}}, {"in", []int{1999, 2005}}, {"not_in", []int{1999, 2005}}}
	date := []struct {
		op string
		v  interface{}
	}{{"equals", "2020-01-01"}, {"greater_than", "2020-01-01"}, {"greater_or_equal", "2020-01-01"}, {"less_than", "2040-01-01"},
		{"less_or_equal", "2040-01-01"}, {"between", []string{"2020-01-01", "2040-01-01"}}, {"in_last_days", 30}}
	for _, f := range []string{"title", "genre", "language", "country", "path", "quality"} {
		vals[f] = text
	}
	for _, f := range []string{"year", "rating", "runtime", "size"} {
		vals[f] = num
	}
	vals["added"], vals["updated"] = date, date
	vals["tags"] = text[2:4]
	vals["favorite"] = []struct {
		op string
		v  interface{}
	}{{"equals", true}, {"equals", false}, {"not_equals", true}}
	vals["media_type"] = []struct {
		op string
		v  interface{}
	}{{"equals", "movie"}, {"not_equals", "movie"}, {"in", []string{"movie", "song"}}, {"not_in", []string{"movie", "video"}}, {"equals", "video"}}
	var docs []string
	fl := make([]string, 0, len(vals))
	for f := range vals {
		fl = append(fl, f)
	}
	sort.Strings(fl)
	for _, f := range fl {
		for _, c := range vals[f] {
			docs = append(docs, rule(f, c.op, c.v))
		}
	}
	// groups, nesting and OR
	docs = append(docs,
		`[{"field":"genre","operator":"contains","value":"a","condition":"or"},{"field":"year","operator":"greater_than","value":1}]`,
		`[{"field":"title","operator":"contains","value":"o","nested_rules":[{"field":"genre","operator":"not_in","value":["c"]}]}]`)
	return docs
}

// Every field x operator must compile and run on both dialects (no type/cast/escape error) and return
// the same titles on both.
func TestRuleMatrix_DialectParity(t *testing.T) {
	sdb := realDB(t)
	seedEdge(t, sdb, false)
	pdb := pgDB(t)
	seedEdge(t, pdb, true)
	n := 0
	for _, d := range matrixDocs() {
		s := docTitles(t, sdb, false, d, 1)
		p := docTitles(t, pdb, true, d, 1)
		n++
		for _, got := range [][]string{s, p} {
			if len(got) == 1 && strings.HasPrefix(got[0], "ERR:") {
				t.Errorf("%s: %v", d, got)
			}
		}
		if fmt.Sprint(s) != fmt.Sprint(p) {
			t.Errorf("dialect divergence %s: sqlite=%v postgres=%v", d, s, p)
		}
	}
	require.Greater(t, n, 80)
}

var _ = mediamodels.MediaItem{}
var _ = repository.NewMediaItemRepository
