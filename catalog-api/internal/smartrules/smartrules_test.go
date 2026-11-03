package smartrules_test

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"catalogizer/config"
	"catalogizer/database"
	"catalogizer/internal/smartrules"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func raws(t *testing.T, doc string) []json.RawMessage {
	t.Helper()
	var out []json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(doc), &out))
	return out
}

func parseErr(t *testing.T, doc string) error {
	t.Helper()
	_, err := smartrules.Parse(raws(t, doc))
	return err
}

func TestParse_AcceptsUIShapeAndAliases(t *testing.T) {
	rs, err := smartrules.Parse(raws(t, `[
	  {"id":"r1","label":"Year","field_type":"number","field":"year","operator":"greater_than","value":"2000","condition":"AND"},
	  {"field":"created_at","op":"in_last_days","value":30,"condition":"or"}]`))
	require.NoError(t, err)
	require.Len(t, rs.Rules, 2)
	assert.Equal(t, "added", rs.Rules[1].Field, "alias resolved")
	assert.False(t, rs.Rules[1].AndWithPrev)
	assert.Contains(t, rs.Raw, `"label":"Year"`, "UI metadata preserved for round trip")
}

func TestParse_RejectsClosedSchemaViolations(t *testing.T) {
	bad := map[string]string{
		"unknown field":        `[{"field":"password_hash","operator":"equals","value":"x"}]`,
		"unknown operator":     `[{"field":"title","operator":"regex","value":"x"}]`,
		"operator for kind":    `[{"field":"year","operator":"contains","value":"1"}]`,
		"tags equals":          `[{"field":"tags","operator":"equals","value":"x"}]`,
		"unknown key":          `[{"field":"title","operator":"equals","value":"x","sql":"1=1"}]`,
		"null value":           `[{"field":"title","operator":"equals","value":null}]`,
		"empty string":         `[{"field":"title","operator":"equals","value":""}]`,
		"NaN-ish":              `[{"field":"year","operator":"equals","value":"NaN"}]`,
		"op disagree":          `[{"field":"title","operator":"equals","op":"contains","value":"x"}]`,
		"between arity":        `[{"field":"year","operator":"between","value":[1]}]`,
		"empty in":             `[{"field":"title","operator":"in","value":[]}]`,
		"bad condition":        `[{"field":"title","operator":"equals","value":"x","condition":"xor"}]`,
		"bad date":             `[{"field":"added","operator":"greater_than","value":"soon"}]`,
		"last days fraction":   `[{"field":"added","operator":"in_last_days","value":1.5}]`,
		"last days huge":       `[{"field":"added","operator":"in_last_days","value":99999999}]`,
		"bool field bad value": `[{"field":"favorite","operator":"equals","value":"maybe"}]`,
		"media type unknown":   `[{"field":"media_type","operator":"equals","value":"spaceship"}]`,
		"rule not object":      `["title='x'"]`,
		"empty list":           `[]`,
		"NUL in value":         `[{"field":"title","operator":"equals","value":"a\u0000b"}]`,
	}
	for name, doc := range bad {
		err := parseErr(t, doc)
		require.Error(t, err, name)
		assert.True(t, smartrules.IsValidation(err), name+": must be a ValidationError (HTTP 400)")
	}
	long := `[{"field":"title","operator":"equals","value":"` + strings.Repeat("a", smartrules.MaxValueBytes+1) + `"}]`
	assert.Error(t, parseErr(t, long))
}

func TestParse_Limits(t *testing.T) {
	var many []string
	for i := 0; i < smartrules.MaxNodes+1; i++ {
		many = append(many, `{"field":"year","operator":"equals","value":1}`)
	}
	assert.Error(t, parseErr(t, "["+strings.Join(many, ",")+"]"), "node cap")
	ok := make([]string, smartrules.MaxNodes)
	for i := range ok {
		ok[i] = `{"field":"year","operator":"equals","value":1}`
	}
	assert.NoError(t, parseErr(t, "["+strings.Join(ok, ",")+"]"), "exactly the cap is fine")

	nest := func(depth int) string {
		s := `{"field":"year","operator":"equals","value":1}`
		for i := 1; i < depth; i++ {
			s = `{"field":"year","operator":"equals","value":1,"nested_rules":[` + s + `]}`
		}
		return "[" + s + "]"
	}
	assert.NoError(t, parseErr(t, nest(smartrules.MaxDepth)))
	assert.Error(t, parseErr(t, nest(smartrules.MaxDepth+1)))
}

func TestCompile_NoUserTextInSQL(t *testing.T) {
	evil := `x' OR '1'='1'); DROP TABLE media_items;--`
	doc := fmt.Sprintf(`[{"field":"title","operator":"contains","value":%q},{"field":"genre","operator":"in","value":[%q,"b"],"condition":"or"},{"field":"path","operator":"starts_with","value":%q}]`, evil, evil, evil)
	rs, err := smartrules.Parse(raws(t, doc))
	require.NoError(t, err)
	sql, args, err := smartrules.Compile(rs, smartrules.Options{UserID: 1})
	require.NoError(t, err)
	assert.NotContains(t, sql, "DROP", "value text must never reach the SQL string")
	assert.NotContains(t, sql, "OR '1'")
	assert.Equal(t, strings.Count(sql, "?"), len(args), "one bound argument per placeholder")
}

func TestCompile_UserScopedFieldNeedsUser(t *testing.T) {
	rs, err := smartrules.Parse(raws(t, `[{"field":"tags","operator":"contains","value":"x"}]`))
	require.NoError(t, err)
	_, _, err = smartrules.Compile(rs, smartrules.Options{})
	require.Error(t, err)
	assert.True(t, smartrules.IsValidation(err))
}

func TestNormalizePage(t *testing.T) {
	l, o := smartrules.NormalizePage(0, -5)
	assert.Equal(t, smartrules.DefaultLimit, l)
	assert.Equal(t, 0, o)
	l, _ = smartrules.NormalizePage(1_000_000, 0)
	assert.Equal(t, smartrules.MaxLimit, l)
}

// --- evaluation against a real SQLite database ------------------------------------------------

func realDB(t *testing.T) *database.DB {
	t.Helper()
	dir := t.TempDir()
	db, err := database.NewConnection(&config.DatabaseConfig{Type: "sqlite", Path: filepath.Join(dir, "s.db"),
		EncryptionKeyFile: filepath.Join(dir, "k", "db.key"), MaxOpenConnections: 2})
	require.NoError(t, err)
	require.NoError(t, db.RunMigrations(context.Background()))
	t.Cleanup(func() { db.Close() })
	return db
}

func seed(t *testing.T, db *database.DB) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO users (username, email, password_hash, salt, role_id, is_active) VALUES ('u1','u1@x.invalid','h','s',2,1)`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO storage_roots (name, protocol, path, enabled) VALUES ('r','local','/r',1)`)
	require.NoError(t, err)
	now := time.Now().UTC()
	ts := func(d time.Duration) string { return now.Add(-d).Format("2006-01-02 15:04:05") }
	add := func(mt, title string, year int, rating float64, age time.Duration, path string, size int, deleted int) {
		var tid int64
		require.NoError(t, db.QueryRow(`SELECT id FROM media_types WHERE name=?`, mt).Scan(&tid))
		res, err := db.Exec(`INSERT INTO media_items (media_type_id,title,year,rating,status,first_detected) VALUES (?,?,?,?,'detected',?)`, tid, title, year, rating, ts(age))
		require.NoError(t, err)
		id, _ := res.LastInsertId()
		fr, err := db.Exec(`INSERT INTO files (storage_root_id,path,name,size,is_directory,modified_at,deleted) VALUES (1,?,?,?,0,?,?)`, path, path, size, ts(age), deleted)
		require.NoError(t, err)
		fid, _ := fr.LastInsertId()
		_, err = db.Exec(`INSERT INTO media_files (media_item_id,file_id) VALUES (?,?)`, id, fid)
		require.NoError(t, err)
	}
	add("movie", "Alpha", 2019, 8.5, 48*time.Hour, "/m/alpha.mkv", 100, 0)
	add("movie", "Beta", 2005, 6, 40*24*time.Hour, "/m/old/beta.mkv", 900, 0)
	add("song", "Gamma", 2021, 7, 24*time.Hour, "/s/gamma.mp3", 5, 0)
	add("movie", "Ghost", 2020, 5, 24*time.Hour, "/gone/ghost.mkv", 99999, 1) // file deleted
}

func titles(t *testing.T, db *database.DB, doc string, uid int64) []string {
	t.Helper()
	rs, err := smartrules.Parse(raws(t, doc))
	require.NoError(t, err)
	items, total, err := smartrules.Evaluate(context.Background(), db, rs, smartrules.Options{UserID: uid}, 100, 0)
	require.NoError(t, err)
	assert.Equal(t, len(items), total)
	out := []string{}
	for _, it := range items {
		out = append(out, it.Title)
	}
	return out
}

func TestEvaluate_RealSQLite(t *testing.T) {
	db := realDB(t)
	seed(t, db)
	cases := []struct {
		name, doc string
		want      []string
	}{
		{"equals is case-insensitive", `[{"field":"title","operator":"equals","value":"aLpHa"}]`, []string{"Alpha"}},
		{"between years", `[{"field":"year","operator":"between","value":[2005,2019]}]`, []string{"Alpha", "Beta"}},
		{"video alias", `[{"field":"media_type","operator":"equals","value":"video"}]`, []string{"Alpha", "Beta", "Ghost"}},
		{"image alias matches nothing", `[{"field":"media_type","operator":"equals","value":"image"}]`, []string{}},
		{"not image matches everything", `[{"field":"media_type","operator":"not_equals","value":"image"}]`, []string{"Alpha", "Beta", "Gamma", "Ghost"}},
		{"size ignores deleted files", `[{"field":"size","operator":"greater_than","value":50000}]`, []string{}},
		{"deleted file path does not match", `[{"field":"path","operator":"starts_with","value":"/gone"}]`, []string{}},
		{"path prefix", `[{"field":"path","operator":"starts_with","value":"/m/"}]`, []string{"Alpha", "Beta"}},
		{"not_contains path", `[{"field":"path","operator":"not_contains","value":"/old/"}]`, []string{"Alpha", "Gamma", "Ghost"}},
		{"added in last 3 days", `[{"field":"added","operator":"in_last_days","value":3}]`, []string{"Alpha", "Gamma", "Ghost"}},
		{"date equals whole day", fmt.Sprintf(`[{"field":"added","operator":"equals","value":%q}]`, time.Now().UTC().Add(-24*time.Hour).Format("2006-01-02")), []string{"Gamma", "Ghost"}},
		{"AND/OR precedence", `[{"field":"title","operator":"equals","value":"Alpha"},{"field":"title","operator":"equals","value":"Beta","condition":"or"},{"field":"year","operator":"equals","value":1,"condition":"and"}]`, []string{"Alpha"}},
		{"nested narrows parent", `[{"field":"media_type","operator":"equals","value":"movie","nested_rules":[{"field":"year","operator":"greater_than","value":2010}]}]`, []string{"Alpha", "Ghost"}},
		{"in list", `[{"field":"title","operator":"in","value":["alpha","gamma"]}]`, []string{"Alpha", "Gamma"}},
		{"injection text is inert", `[{"field":"title","operator":"equals","value":"x' OR '1'='1"}]`, []string{}},
		{"percent is literal", `[{"field":"title","operator":"contains","value":"%"}]`, []string{}},
		{"underscore is literal", `[{"field":"title","operator":"contains","value":"A_pha"}]`, []string{}},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, titles(t, db, tc.doc, 1), tc.name)
	}
	// the table survived every injection attempt
	assert.Equal(t, []string{"Alpha"}, titles(t, db, `[{"field":"title","operator":"equals","value":"Alpha"}]`, 1))
}

func TestEvaluate_UserScopedFieldsAreViewerScoped(t *testing.T) {
	db := realDB(t)
	seed(t, db)
	_, err := db.Exec(`INSERT INTO users (username, email, password_hash, salt, role_id, is_active) VALUES ('u2','u2@x.invalid','h','s',2,1)`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO user_metadata (media_item_id,user_id,tags,favorite) VALUES (1,1,'night,fav',0)`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO favorites (user_id, entity_type, entity_id) VALUES (1,'movie',1)`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO user_metadata (media_item_id,user_id,tags,favorite) VALUES (2,2,'night',1)`)
	require.NoError(t, err)
	tag := `[{"field":"tags","operator":"contains","value":"night"}]`
	assert.Equal(t, []string{"Alpha"}, titles(t, db, tag, 1))
	assert.Equal(t, []string{"Beta"}, titles(t, db, tag, 2))
	assert.Equal(t, []string{}, titles(t, db, tag, 99), "a viewer with no metadata matches nothing")
	fav := `[{"field":"favorite","operator":"equals","value":true}]`
	assert.Equal(t, []string{"Alpha"}, titles(t, db, fav, 1))
	assert.Equal(t, []string{}, titles(t, db, fav, 2))
	notFav := `[{"field":"favorite","operator":"equals","value":false}]`
	assert.Equal(t, []string{"Beta", "Gamma", "Ghost"}, titles(t, db, notFav, 1))
	nt := `[{"field":"tags","operator":"not_contains","value":"night"}]`
	assert.Equal(t, []string{"Beta", "Gamma", "Ghost"}, titles(t, db, nt, 1))
}

func TestEvaluate_PagingAndCap(t *testing.T) {
	db := realDB(t)
	seed(t, db)
	rs, err := smartrules.Parse(raws(t, `[{"field":"year","operator":"greater_than","value":0}]`))
	require.NoError(t, err)
	items, total, err := smartrules.Evaluate(context.Background(), db, rs, smartrules.Options{UserID: 1}, 2, 1)
	require.NoError(t, err)
	assert.Equal(t, 4, total)
	require.Len(t, items, 2)
	assert.Equal(t, "Beta", items[0].Title, "ordered by id, offset 1")
	items, _, err = smartrules.Evaluate(context.Background(), db, rs, smartrules.Options{UserID: 1}, 10_000_000, 0)
	require.NoError(t, err)
	assert.LessOrEqual(t, len(items), smartrules.MaxLimit)
}

// Negative operators must also match rows whose column is NULL, and LIKE wildcards typed by the user
// must be matched literally (ESCAPE clause). Both were uncovered by the table above.
func TestEvaluate_NullsAndLikeWildcards(t *testing.T) {
	db := realDB(t)
	seed(t, db) // every seeded item has a NULL genre
	var tid int64
	require.NoError(t, db.QueryRow(`SELECT id FROM media_types WHERE name='movie'`).Scan(&tid))
	_, err := db.Exec(`INSERT INTO media_items (media_type_id,title,year,rating,genre,status,first_detected) VALUES (?,?,?,?,?,'detected',datetime('now'))`, tid, "50% Off", 2020, 5, "Comedy")
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO media_items (media_type_id,title,year,rating,genre,status,first_detected) VALUES (?,?,?,?,?,'detected',datetime('now'))`, tid, "50 Off", 2020, 5, "Comedy")
	require.NoError(t, err)

	got := titles(t, db, `[{"field":"genre","operator":"not_equals","value":"Comedy"}]`, 1)
	assert.ElementsMatch(t, []string{"Alpha", "Beta", "Gamma", "Ghost"}, got, "NULL genre must match not_equals")

	got = titles(t, db, `[{"field":"genre","operator":"not_contains","value":"Comed"}]`, 1)
	assert.ElementsMatch(t, []string{"Alpha", "Beta", "Gamma", "Ghost"}, got, "NULL genre must match not_contains")

	got = titles(t, db, `[{"field":"title","operator":"contains","value":"50%"}]`, 1)
	assert.Equal(t, []string{"50% Off"}, got, "a typed % is literal, not a wildcard")

	got = titles(t, db, `[{"field":"title","operator":"starts_with","value":"50_"}]`, 1)
	assert.Equal(t, []string{}, got, "a typed _ is literal, not a wildcard")
}
