package smartrules_test

// Reviewer-authored mutation tests (review rounds 2-3) and the reverted-COALESCE mutant.
//
//  M1  quality: a quality_profile key NESTED inside another object must not match (Postgres reads the
//      top-level key only; SQLite string matching used to find it anywhere).
//  M2  quality: a top-level key AFTER a closed nested object (resolution) must still match.
//  M3  genre on Postgres must not be wrapped in COALESCE(<boolean>, 0) (type error on Postgres).

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"catalogizer/database"
	mediamodels "catalogizer/internal/media/models"
	"catalogizer/internal/smartrules"
	"catalogizer/repository"

	"github.com/stretchr/testify/require"
)

func seedQuality(t *testing.T, db *database.DB, pg bool) {
	t.Helper()
	ctx := context.Background()
	act := "1"
	if pg {
		act = "TRUE"
	}
	_, err := db.Exec(`INSERT INTO storage_roots (name, protocol, path, enabled) VALUES ('r','local','/r',` + act + `)`)
	require.NoError(t, err)
	var movie int64
	require.NoError(t, db.QueryRow(`SELECT id FROM media_types WHERE name='movie'`).Scan(&movie))
	items := repository.NewMediaItemRepository(db)
	files := repository.NewMediaFileRepository(db)
	rows := []struct{ title, qi string }{
		{"Top", `{"quality_profile":"1080p","quality_score":1}`},
		{"Nested", `{"nested":{"quality_profile":"1080p"}}`},
		{"AfterRes", `{"resolution":{"width":1,"height":2},"quality_profile":"720p","quality_score":1}`},
		{"NestedDeep", `{"a":{"b":{"quality_profile":"720p"}}}`},
		{"Esc", `{"video_codec":"x\"y","quality_profile":"4k \"hdr\"","quality_score":1}`},
	}
	for _, r := range rows {
		id, err := items.Create(ctx, &mediamodels.MediaItem{MediaTypeID: movie, Title: r.title, Status: "detected"})
		require.NoError(t, err)
		_, err = db.Exec(`INSERT INTO files (storage_root_id,path,name,size,is_directory,modified_at,deleted) VALUES (1,?,?,1,?,?,?)`, "/q/"+r.title, r.title, false, time.Now().UTC(), false)
		require.NoError(t, err)
		var fid int64
		require.NoError(t, db.QueryRow(`SELECT id FROM files WHERE path = ?`, "/q/"+r.title).Scan(&fid))
		qi := r.qi
		_, err = files.LinkFileToItem(ctx, id, fid, &qi, nil, true)
		require.NoError(t, err)
	}
}

func qualityCases(t *testing.T, db *database.DB, pg bool) {
	seedQuality(t, db, pg)
	cases := []struct {
		doc  string
		want string
	}{
		{rule("quality", "equals", "1080p"), "[Top]"},        // M1: Nested must not match
		{rule("quality", "equals", "720p"), "[AfterRes]"},    // M2 (and NestedDeep must not match)
		{rule("quality", "contains", "p"), "[AfterRes Top]"}, // substring over the same top-level values
		{rule("quality", "equals", `4k "hdr"`), "[Esc]"},     // escaped quotes after an earlier string
		{rule("quality", "starts_with", "10"), "[Top]"},
		{rule("quality", "not_equals", "1080p"), "[AfterRes Esc Nested NestedDeep]"},
	}
	for _, c := range cases {
		if got := fmt.Sprint(docTitles(t, db, pg, c.doc, 1)); got != c.want {
			t.Errorf("%s: got %s want %s", c.doc, got, c.want)
		}
	}
}

func TestQualityTopLevelKeyOnly_SQLite(t *testing.T) { qualityCases(t, realDB(t), false) }

func TestQualityTopLevelKeyOnly_Postgres(t *testing.T) { qualityCases(t, pgDB(t), true) }

// coalesceWrapsExists reports whether sql contains COALESCE( directly around an EXISTS predicate, the
// shape that is a type error on Postgres.
var coalesceWrapsExists = regexp.MustCompile(`(?i)COALESCE\(\s*EXISTS`)

func TestCompile_PostgresGenreIsNotCoalesced(t *testing.T) {
	for _, op := range []string{"equals", "not_equals", "contains", "not_contains", "starts_with", "ends_with"} {
		for _, pg := range []bool{true, false} {
			rs, err := smartrules.Parse(rawDoc(t, rule("genre", op, "a")))
			require.NoError(t, err)
			sql, _, err := smartrules.Compile(rs, smartrules.Options{Postgres: pg, UserID: 1})
			require.NoError(t, err)
			if pg {
				require.False(t, coalesceWrapsExists.MatchString(sql), "postgres %s: %s", op, sql)
				require.Contains(t, sql, "EXISTS (")
			} else {
				require.Contains(t, sql, "COALESCE(", "sqlite genre keeps the NULL-safe wrapper")
			}
		}
	}
}

// Self-check of the detector: the pre-fix (reverted) shape MUST be flagged, so the test above would
// kill a revert of the Postgres branch.
func TestCompile_RevertedCoalesceMutantIsDetected(t *testing.T) {
	mutant := "COALESCE(EXISTS (SELECT 1 FROM jsonb_array_elements_text(x) AS je(value) WHERE je.value = $1), 0)"
	require.True(t, coalesceWrapsExists.MatchString(mutant))
	require.False(t, coalesceWrapsExists.MatchString(strings.Replace(mutant, "COALESCE(", "(", 1)))
}

// Real-Postgres proof of WHY: the reverted shape is rejected by the server.
func TestPostgres_RejectsCoalesceOfBoolean(t *testing.T) {
	db := pgDB(t)
	var v bool
	err := db.QueryRow(`SELECT COALESCE(EXISTS (SELECT 1), 0)`).Scan(&v)
	require.Error(t, err)
	require.Contains(t, err.Error(), "COALESCE")
}

func rawDoc(t *testing.T, doc string) []json.RawMessage {
	t.Helper()
	var raws []json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(doc), &raws))
	return raws
}
