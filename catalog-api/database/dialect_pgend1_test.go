package database

import "testing"

// CAT-PG-SESSION-END-1: BOOLEAN columns that were missing from the literal rewrite list (a `= 0/1` literal on PostgreSQL fails with
// "operator does not exist: boolean = integer").
func TestRewriteBooleanLiterals_PlaybackAndMetadataColumns(t *testing.T) {
	d := &Dialect{Type: DialectPostgres}
	cases := map[string]string{
		"SELECT 1 FROM playback_position WHERE is_completed = 1":         "SELECT 1 FROM playback_position WHERE is_completed = TRUE",
		"SELECT 1 FROM playback_sessions WHERE completed = 0":            "SELECT 1 FROM playback_sessions WHERE completed = FALSE",
		"SELECT 1 FROM user_metadata WHERE user_id = ? AND favorite = 1": "SELECT 1 FROM user_metadata WHERE user_id = ? AND favorite = TRUE",
		"SELECT 1 FROM h WHERE was_completed = 1":                        "SELECT 1 FROM h WHERE was_completed = 1", // different column: untouched
		"SELECT 1 FROM x WHERE can_read = 1 AND can_write = 0":           "SELECT 1 FROM x WHERE can_read = TRUE AND can_write = FALSE",
	}
	for in, want := range cases {
		if got := d.RewriteBooleanLiterals(in); got != want {
			t.Errorf("%q\n got  %q\n want %q", in, got, want)
		}
	}
	s := &Dialect{Type: DialectSQLite}
	if got := s.RewriteBooleanLiterals("SELECT 1 WHERE completed = 1"); got != "SELECT 1 WHERE completed = 1" {
		t.Errorf("SQLite must be untouched, got %q", got)
	}
}
