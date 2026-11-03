package smartrules_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"catalogizer/database"
	mediamodels "catalogizer/internal/media/models"
	"catalogizer/internal/smartrules"
	"catalogizer/models"
	"catalogizer/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seedProduction writes the fixtures through the PRODUCTION writers (MediaItemRepository.Create stores
// genre as a JSON array, MediaFileRepository.LinkFileToItem stores quality_info as QualityInfo JSON,
// FavoritesRepository.CreateFavorite writes the favorites table the UI toggles). Plain-text fixtures hid
// the class "JSON-encoded column compiled as text" (review round 1, F1/F2).
func seedProduction(t *testing.T) (*repoDB, map[string]int64) {
	db := realDB(t)
	ctx := context.Background()
	_, err := db.Exec(`INSERT INTO users (username, email, password_hash, salt, role_id, is_active) VALUES ('u1','u1@x.invalid','h','s',2,1)`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO users (username, email, password_hash, salt, role_id, is_active) VALUES ('u2','u2@x.invalid','h','s',2,1)`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO storage_roots (name, protocol, path, enabled) VALUES ('r','local','/r',1)`)
	require.NoError(t, err)
	var movie int64
	require.NoError(t, db.QueryRow(`SELECT id FROM media_types WHERE name='movie'`).Scan(&movie))

	items := repository.NewMediaItemRepository(db)
	files := repository.NewMediaFileRepository(db)
	ids := map[string]int64{}
	add := func(title string, genre []string, profile *string) {
		id, err := items.Create(ctx, &mediamodels.MediaItem{MediaTypeID: movie, Title: title, Genre: genre, Status: "detected"})
		require.NoError(t, err)
		ids[title] = id
		fr, err := db.Exec(`INSERT INTO files (storage_root_id,path,name,size,is_directory,modified_at,deleted) VALUES (1,?,?,1,0,?,0)`, "/m/"+title, title, time.Now().UTC())
		require.NoError(t, err)
		fid, _ := fr.LastInsertId()
		var q *string
		if profile != nil {
			b, err := json.Marshal(&mediamodels.QualityInfo{QualityProfile: profile, QualityScore: 80, Resolution: &mediamodels.Resolution{Width: 1920, Height: 1080}})
			require.NoError(t, err)
			s := string(b)
			q = &s
		}
		_, err = files.LinkFileToItem(ctx, id, fid, q, nil, true)
		require.NoError(t, err)
	}
	p := func(s string) *string { return &s }
	add("Mix", []string{"Action", "Sci-Fi"}, p("1080p"))
	add("Drama", []string{"Drama"}, p("4K/UHD"))
	add("ActionComedy", []string{"Action Comedy"}, p("720p"))
	add("NoGenre", nil, nil)
	add("Rock", []string{`Rock "n" Roll`, "R&B"}, p("Audio_Lossless"))
	return &repoDB{databaseDB: db}, ids
}

type repoDB struct{ databaseDB *database.DB }

func TestEvaluate_JSONEncodedColumns_ProductionWriters(t *testing.T) {
	rdb, _ := seedProduction(t)
	db := rdb.databaseDB
	g := func(op string, v interface{}) string {
		b, _ := json.Marshal([]map[string]interface{}{{"field": "genre", "operator": op, "value": v}})
		return string(b)
	}
	q := func(op string, v interface{}) string {
		b, _ := json.Marshal([]map[string]interface{}{{"field": "quality", "operator": op, "value": v}})
		return string(b)
	}
	all := []string{"Mix", "Drama", "ActionComedy", "NoGenre", "Rock"}
	cases := []struct {
		name, doc string
		want      []string
	}{
		{"genre equals is element equality", g("equals", "action"), []string{"Mix"}},
		{"genre equals second element", g("equals", "SCI-FI"), []string{"Mix"}},
		{"genre not_equals incl. NULL", g("not_equals", "Action"), []string{"Drama", "ActionComedy", "NoGenre", "Rock"}},
		{"genre in", g("in", []string{"drama", "sci-fi"}), []string{"Mix", "Drama"}},
		{"genre not_in", g("not_in", []string{"drama", "sci-fi"}), []string{"ActionComedy", "NoGenre", "Rock"}},
		{"genre contains inside an element", g("contains", "act"), []string{"Mix", "ActionComedy"}},
		{"JSON comma is not content", g("contains", ","), []string{}},
		{"JSON bracket is not content", g("contains", "["), []string{}},
		{"JSON quote char inside element", g("contains", `"`), []string{"Rock"}},
		{"genre not_contains", g("not_contains", "act"), []string{"Drama", "NoGenre", "Rock"}},
		{"starts_with element start", g("starts_with", "sci"), []string{"Mix"}},
		{"starts_with is not mid-element", g("starts_with", "ction"), []string{}},
		{"starts_with comma is syntax", g("starts_with", ","), []string{}},
		{"ends_with element end", g("ends_with", "comedy"), []string{"ActionComedy"}},
		{"ends_with fi", g("ends_with", "-fi"), []string{"Mix"}},
		{"escaped quotes equals", g("equals", `rock "n" roll`), []string{"Rock"}},
		{"HTML-escaped ampersand equals", g("equals", "r&b"), []string{"Rock"}},
		{"quality equals profile", q("equals", "1080p"), []string{"Mix"}},
		{"quality equals UHD", q("equals", "4k/uhd"), []string{"Drama"}},
		{"quality in", q("in", []string{"720p", "1080p"}), []string{"Mix", "ActionComedy"}},
		{"quality contains", q("contains", "uhd"), []string{"Drama"}},
		{"quality starts_with", q("starts_with", "audio"), []string{"Rock"}},
		{"quality JSON key name is not content", q("contains", "quality_profile"), []string{}},
		{"quality not_equals", q("not_equals", "1080p"), []string{"Drama", "ActionComedy", "NoGenre", "Rock"}},
	}
	_ = all
	for _, tc := range cases {
		got := titles(t, db, tc.doc, 1)
		assert.ElementsMatch(t, tc.want, got, tc.name)
	}
}

// F3: the favourite rule reads the store the UI toggles (the favorites table), per viewer.
func TestEvaluate_FavoriteReadsFavoritesTable(t *testing.T) {
	rdb, ids := seedProduction(t)
	db := rdb.databaseDB
	favs := repository.NewFavoritesRepository(db)
	_, err := favs.CreateFavorite(&models.Favorite{UserID: 1, EntityType: "movie", EntityID: int(ids["Mix"])})
	require.NoError(t, err)
	_, err = favs.CreateFavorite(&models.Favorite{UserID: 2, EntityType: "movie", EntityID: int(ids["Drama"])})
	require.NoError(t, err)
	// a favourite of a NON-media kind with the same numeric id must not leak in
	_, err = favs.CreateFavorite(&models.Favorite{UserID: 1, EntityType: "playlist", EntityID: int(ids["Rock"])})
	require.NoError(t, err)
	// the legacy flag alone no longer defines "favourite"
	_, err = db.Exec(`INSERT INTO user_metadata (media_item_id,user_id,tags,favorite) VALUES (?,1,'x',1)`, ids["ActionComedy"])
	require.NoError(t, err)

	fav := `[{"field":"favorite","operator":"equals","value":true}]`
	assert.ElementsMatch(t, []string{"Mix"}, titles(t, db, fav, 1))
	assert.ElementsMatch(t, []string{"Drama"}, titles(t, db, fav, 2))
	assert.ElementsMatch(t, []string{}, titles(t, db, fav, 99))
	notFav := `[{"field":"favorite","operator":"not_equals","value":true}]`
	assert.ElementsMatch(t, []string{"Drama", "ActionComedy", "NoGenre", "Rock"}, titles(t, db, notFav, 1))
	isFalse := `[{"field":"favorite","operator":"equals","value":false}]`
	assert.ElementsMatch(t, []string{"Drama", "ActionComedy", "NoGenre", "Rock"}, titles(t, db, isFalse, 1))
}

// user_metadata.tags is the third member of the class: UserMetadataRepository stores a JSON array.
func TestEvaluate_TagsAreJSONArray_ProductionWriter(t *testing.T) {
	rdb, ids := seedProduction(t)
	db := rdb.databaseDB
	um := repository.NewUserMetadataRepository(db)
	ctx := context.Background()
	_, err := um.Create(ctx, &mediamodels.UserMetadata{MediaItemID: ids["Mix"], UserID: 1, Tags: []string{"weekend", `date "night"`}})
	require.NoError(t, err)
	_, err = um.Create(ctx, &mediamodels.UserMetadata{MediaItemID: ids["Drama"], UserID: 1, Tags: nil}) // stored as JSON null
	require.NoError(t, err)
	_, err = um.Create(ctx, &mediamodels.UserMetadata{MediaItemID: ids["Rock"], UserID: 2, Tags: []string{"weekend"}})
	require.NoError(t, err)
	tg := func(op, v string) string {
		b, _ := json.Marshal([]map[string]interface{}{{"field": "tags", "operator": op, "value": v}})
		return string(b)
	}
	assert.ElementsMatch(t, []string{"Mix"}, titles(t, db, tg("contains", "week"), 1))
	assert.ElementsMatch(t, []string{"Rock"}, titles(t, db, tg("contains", "week"), 2))
	assert.ElementsMatch(t, []string{"Mix"}, titles(t, db, tg("contains", `"night"`), 1))
	assert.ElementsMatch(t, []string{}, titles(t, db, tg("contains", ","), 1), "JSON comma is not tag content")
	assert.ElementsMatch(t, []string{}, titles(t, db, tg("contains", "null"), 1), "a nil tag list is no tag")
	assert.ElementsMatch(t, []string{"Drama", "ActionComedy", "NoGenre", "Rock"}, titles(t, db, tg("not_contains", "week"), 1))
}

// F6: the production writer stores time.Time with the host zone offset ("2026-11-04 01:00:00+14:00"); the
// date rules must compare INSTANTS, not text, so a non-UTC host cannot shift a day.
func TestEvaluate_DatesCompareInstants_NonUTCWriter(t *testing.T) {
	db := realDB(t)
	var movie int64
	require.NoError(t, db.QueryRow(`SELECT id FROM media_types WHERE name='movie'`).Scan(&movie))
	zone := time.FixedZone("UTC+14", 14*3600)
	local := time.Date(2026, 11, 4, 1, 0, 0, 0, zone) // = 2026-11-03T11:00:00Z
	items := repository.NewMediaItemRepository(db)
	_, err := items.Create(context.Background(), &mediamodels.MediaItem{MediaTypeID: movie, Title: "Late", Status: "detected", FirstDetected: local, LastUpdated: local})
	require.NoError(t, err)
	d := func(op string, v interface{}) string {
		b, _ := json.Marshal([]map[string]interface{}{{"field": "added", "operator": op, "value": v}})
		return string(b)
	}
	assert.Equal(t, []string{"Late"}, titles(t, db, d("equals", "2026-11-03"), 1), "UTC day of the instant")
	assert.Equal(t, []string{}, titles(t, db, d("equals", "2026-11-04"), 1), "the writer's local calendar day is not the UTC day")
	assert.Equal(t, []string{"Late"}, titles(t, db, d("equals", "2026-11-03T11:00:00Z"), 1), "RFC3339 equals matches the instant")
	assert.Equal(t, []string{"Late"}, titles(t, db, d("greater_than", "2026-11-02"), 1))
	assert.Equal(t, []string{}, titles(t, db, d("greater_than", "2026-11-03"), 1), "greater_than a day means after that whole UTC day")
	assert.Equal(t, []string{"Late"}, titles(t, db, d("less_than", "2026-11-03T12:00:00Z"), 1))
	assert.Equal(t, []string{}, titles(t, db, d("less_than", "2026-11-03T11:00:00Z"), 1))
}

// F8: integer columns reject fractional values (Postgres would answer 500 instead).
func TestParse_IntegerFieldsRejectFractions(t *testing.T) {
	for _, doc := range []string{
		`[{"field":"year","operator":"equals","value":2019.5}]`,
		`[{"field":"runtime","operator":"between","value":[10,20.5]}]`,
		`[{"field":"size","operator":"in","value":[1,2.5]}]`,
		`[{"field":"year","operator":"equals","value":1e30}]`,
	} {
		err := parseErr(t, doc)
		require.Error(t, err, doc)
		assert.True(t, smartrules.IsValidation(err), doc)
	}
	require.NoError(t, parseErr(t, `[{"field":"rating","operator":"greater_than","value":7.5}]`), "rating is a real column")
	require.NoError(t, parseErr(t, `[{"field":"year","operator":"equals","value":"2019"}]`))
}

// F15: a rejected value that is echoed in the error is cut on a rune boundary.
func TestParse_ErrorEchoIsValidUTF8(t *testing.T) {
	err := parseErr(t, `[{"field":"title","operator":"`+strings.Repeat("€", 30)+`","value":"x"}]`)
	require.Error(t, err)
	assert.True(t, utf8.ValidString(err.Error()), err.Error())
}

// Mutation M7: negative number operators also match rows whose column is NULL (production writers leave
// rating/year NULL when unknown).
func TestEvaluate_NumberNegationsMatchNull(t *testing.T) {
	rdb, _ := seedProduction(t)
	db := rdb.databaseDB
	all := []string{"Mix", "Drama", "ActionComedy", "NoGenre", "Rock"}
	assert.ElementsMatch(t, all, titles(t, db, `[{"field":"rating","operator":"not_equals","value":5}]`, 1))
	assert.ElementsMatch(t, all, titles(t, db, `[{"field":"year","operator":"not_in","value":[2000,2001]}]`, 1))
	assert.ElementsMatch(t, []string{}, titles(t, db, `[{"field":"rating","operator":"greater_than","value":0}]`, 1), "a NULL rating is not > 0")
}

// F4: evaluation is bounded in time and counts can be capped.
func TestEvaluate_TimeoutAndBoundedCount(t *testing.T) {
	rdb, _ := seedProduction(t)
	db := rdb.databaseDB
	rs, err := smartrules.Parse(raws(t, `[{"field":"title","operator":"contains","value":"o"}]`))
	require.NoError(t, err)

	_, _, err = smartrules.Evaluate(context.Background(), db, rs, smartrules.Options{UserID: 1, Timeout: time.Nanosecond}, 10, 0)
	require.Error(t, err)
	assert.True(t, errors.Is(err, context.DeadlineExceeded), "got %v", err)

	n, capped, err := smartrules.CountUpTo(context.Background(), db, rs, smartrules.Options{UserID: 1}, 2)
	require.NoError(t, err)
	assert.Equal(t, 2, n)
	assert.True(t, capped, "3 titles contain o (ActionComedy, NoGenre, Rock), cap is 2")
	n, capped, err = smartrules.CountUpTo(context.Background(), db, rs, smartrules.Options{UserID: 1}, 100)
	require.NoError(t, err)
	assert.False(t, capped)
	assert.Equal(t, 3, n)
}

// F10: the names the web builder offers resolve to the closed field/operator set.
func TestParse_BuilderAliases(t *testing.T) {
	rs, err := smartrules.Parse(raws(t, `[
	  {"field":"is_favorite","operator":"equals","value":true},
	  {"field":"file_size","operator":"greater_than","value":10},
	  {"field":"duration","operator":"less_than","value":90},
	  {"field":"date_modified","operator":"after","value":"2026-01-01"}]`))
	require.NoError(t, err)
	var got []string
	for _, r := range rs.Rules {
		got = append(got, r.Field+":"+r.Operator)
	}
	assert.Equal(t, []string{"favorite:equals", "size:greater_than", "runtime:less_than", "updated:greater_than"}, got)
}
