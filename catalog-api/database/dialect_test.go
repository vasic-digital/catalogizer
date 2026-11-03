package database

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ---------------------------------------------------------------------------
// RewritePlaceholders
// ---------------------------------------------------------------------------

func TestDialect_RewritePlaceholders(t *testing.T) {
	tests := []struct {
		name    string
		dialect DialectType
		input   string
		expect  string
	}{
		{
			name:    "sqlite passthrough single placeholder",
			dialect: DialectSQLite,
			input:   "SELECT * FROM users WHERE id = ?",
			expect:  "SELECT * FROM users WHERE id = ?",
		},
		{
			name:    "sqlite passthrough multiple placeholders",
			dialect: DialectSQLite,
			input:   "INSERT INTO users (a, b, c) VALUES (?, ?, ?)",
			expect:  "INSERT INTO users (a, b, c) VALUES (?, ?, ?)",
		},
		{
			name:    "postgres single placeholder",
			dialect: DialectPostgres,
			input:   "SELECT * FROM users WHERE id = ?",
			expect:  "SELECT * FROM users WHERE id = $1",
		},
		{
			name:    "postgres multiple placeholders",
			dialect: DialectPostgres,
			input:   "INSERT INTO users (a, b, c) VALUES (?, ?, ?)",
			expect:  "INSERT INTO users (a, b, c) VALUES ($1, $2, $3)",
		},
		{
			name:    "postgres no placeholders",
			dialect: DialectPostgres,
			input:   "SELECT * FROM users",
			expect:  "SELECT * FROM users",
		},
		{
			name:    "postgres placeholder inside single quotes ignored",
			dialect: DialectPostgres,
			input:   "SELECT * FROM users WHERE name = '?' AND id = ?",
			expect:  "SELECT * FROM users WHERE name = '?' AND id = $1",
		},
		{
			name:    "postgres multiple quoted sections",
			dialect: DialectPostgres,
			input:   "SELECT * FROM t WHERE a = '?' AND b = ? AND c = '?' AND d = ?",
			expect:  "SELECT * FROM t WHERE a = '?' AND b = $1 AND c = '?' AND d = $2",
		},
		{
			name:    "postgres empty query",
			dialect: DialectPostgres,
			input:   "",
			expect:  "",
		},
		{
			name:    "sqlite empty query",
			dialect: DialectSQLite,
			input:   "",
			expect:  "",
		},
		{
			name:    "postgres many placeholders",
			dialect: DialectPostgres,
			input:   "INSERT INTO t VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
			expect:  "INSERT INTO t VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Dialect{Type: tt.dialect}
			got := d.RewritePlaceholders(tt.input)
			assert.Equal(t, tt.expect, got)
		})
	}
}

// ---------------------------------------------------------------------------
// RewriteInsertOrIgnore
// ---------------------------------------------------------------------------

func TestDialect_RewriteInsertOrIgnore(t *testing.T) {
	tests := []struct {
		name    string
		dialect DialectType
		input   string
		expect  string
	}{
		{
			name:    "sqlite passthrough",
			dialect: DialectSQLite,
			input:   "INSERT OR IGNORE INTO users (id, name) VALUES (1, 'test')",
			expect:  "INSERT OR IGNORE INTO users (id, name) VALUES (1, 'test')",
		},
		{
			name:    "postgres rewrite",
			dialect: DialectPostgres,
			input:   "INSERT OR IGNORE INTO users (id, name) VALUES (1, 'test')",
			expect:  "INSERT INTO users (id, name) VALUES (1, 'test') ON CONFLICT DO NOTHING",
		},
		{
			name:    "postgres no match",
			dialect: DialectPostgres,
			input:   "INSERT INTO users (id) VALUES (1)",
			expect:  "INSERT INTO users (id) VALUES (1)",
		},
		{
			name:    "postgres case insensitive match",
			dialect: DialectPostgres,
			input:   "insert or ignore into users (id) VALUES (1)",
			expect:  "INSERT INTO users (id) VALUES (1) ON CONFLICT DO NOTHING",
		},
		{
			name:    "postgres preserves rest of query casing",
			dialect: DialectPostgres,
			input:   "INSERT OR IGNORE INTO MyTable (ColA, ColB) VALUES ('hello', 'world')",
			expect:  "INSERT INTO MyTable (ColA, ColB) VALUES ('hello', 'world') ON CONFLICT DO NOTHING",
		},
		{
			name:    "empty query",
			dialect: DialectPostgres,
			input:   "",
			expect:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Dialect{Type: tt.dialect}
			got := d.RewriteInsertOrIgnore(tt.input)
			assert.Equal(t, tt.expect, got)
		})
	}
}

// ---------------------------------------------------------------------------
// RewriteInsertOrReplace
// ---------------------------------------------------------------------------

func TestDialect_RewriteInsertOrReplace(t *testing.T) {
	tests := []struct {
		name    string
		dialect DialectType
		input   string
		expect  string
	}{
		{
			name:    "sqlite passthrough",
			dialect: DialectSQLite,
			input:   "INSERT OR REPLACE INTO users (id, name) VALUES (1, 'test')",
			expect:  "INSERT OR REPLACE INTO users (id, name) VALUES (1, 'test')",
		},
		{
			name:    "postgres rewrite",
			dialect: DialectPostgres,
			input:   "INSERT OR REPLACE INTO users (id, name) VALUES (1, 'test')",
			expect:  "INSERT INTO users (id, name) VALUES (1, 'test')",
		},
		{
			name:    "postgres no match",
			dialect: DialectPostgres,
			input:   "INSERT INTO users (id) VALUES (1)",
			expect:  "INSERT INTO users (id) VALUES (1)",
		},
		{
			name:    "postgres case insensitive",
			dialect: DialectPostgres,
			input:   "insert or replace into users (id) VALUES (1)",
			expect:  "INSERT INTO users (id) VALUES (1)",
		},
		{
			name:    "empty query",
			dialect: DialectPostgres,
			input:   "",
			expect:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Dialect{Type: tt.dialect}
			got := d.RewriteInsertOrReplace(tt.input)
			assert.Equal(t, tt.expect, got)
		})
	}
}

// ---------------------------------------------------------------------------
// AutoIncrement
// ---------------------------------------------------------------------------

func TestDialect_AutoIncrement(t *testing.T) {
	tests := []struct {
		name    string
		dialect DialectType
		expect  string
	}{
		{"sqlite", DialectSQLite, "INTEGER PRIMARY KEY AUTOINCREMENT"},
		{"postgres", DialectPostgres, "SERIAL PRIMARY KEY"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Dialect{Type: tt.dialect}
			assert.Equal(t, tt.expect, d.AutoIncrement())
		})
	}
}

// ---------------------------------------------------------------------------
// TimestampType
// ---------------------------------------------------------------------------

func TestDialect_TimestampType(t *testing.T) {
	tests := []struct {
		name    string
		dialect DialectType
		expect  string
	}{
		{"sqlite", DialectSQLite, "DATETIME"},
		{"postgres", DialectPostgres, "TIMESTAMP"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Dialect{Type: tt.dialect}
			assert.Equal(t, tt.expect, d.TimestampType())
		})
	}
}

// ---------------------------------------------------------------------------
// BooleanDefault
// ---------------------------------------------------------------------------

func TestDialect_BooleanDefault(t *testing.T) {
	tests := []struct {
		name    string
		dialect DialectType
		val     bool
		expect  string
	}{
		{"sqlite true", DialectSQLite, true, "DEFAULT 1"},
		{"sqlite false", DialectSQLite, false, "DEFAULT 0"},
		{"postgres true", DialectPostgres, true, "DEFAULT TRUE"},
		{"postgres false", DialectPostgres, false, "DEFAULT FALSE"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Dialect{Type: tt.dialect}
			assert.Equal(t, tt.expect, d.BooleanDefault(tt.val))
		})
	}
}

// ---------------------------------------------------------------------------
// CurrentTimestamp
// ---------------------------------------------------------------------------

func TestDialect_CurrentTimestamp(t *testing.T) {
	// Both dialects should return CURRENT_TIMESTAMP
	for _, dt := range []DialectType{DialectSQLite, DialectPostgres} {
		d := &Dialect{Type: dt}
		assert.Equal(t, "CURRENT_TIMESTAMP", d.CurrentTimestamp())
	}
}

// ---------------------------------------------------------------------------
// IsSQLite / IsPostgres
// ---------------------------------------------------------------------------

func TestDialect_IsSQLite(t *testing.T) {
	tests := []struct {
		name    string
		dialect DialectType
		expect  bool
	}{
		{"sqlite is SQLite", DialectSQLite, true},
		{"postgres is not SQLite", DialectPostgres, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Dialect{Type: tt.dialect}
			assert.Equal(t, tt.expect, d.IsSQLite())
		})
	}
}

func TestDialect_IsPostgres(t *testing.T) {
	tests := []struct {
		name    string
		dialect DialectType
		expect  bool
	}{
		{"postgres is Postgres", DialectPostgres, true},
		{"sqlite is not Postgres", DialectSQLite, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Dialect{Type: tt.dialect}
			assert.Equal(t, tt.expect, d.IsPostgres())
		})
	}
}

// ---------------------------------------------------------------------------
// RewriteBooleanLiterals — additional edge cases beyond connection_test.go
// ---------------------------------------------------------------------------

func TestDialect_RewriteBooleanLiterals_EdgeCases(t *testing.T) {
	pg := Dialect{Type: DialectPostgres}

	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "is_favorite column",
			input:  "SELECT * FROM favorites WHERE is_favorite = 1",
			expect: "SELECT * FROM favorites WHERE is_favorite = TRUE",
		},
		{
			name:   "is_public column",
			input:  "SELECT * FROM collections WHERE is_public = 0",
			expect: "SELECT * FROM collections WHERE is_public = FALSE",
		},
		{
			name:   "shuffle_enabled column",
			input:  "UPDATE playlists SET shuffle_enabled = 1 WHERE id = 5",
			expect: "UPDATE playlists SET shuffle_enabled = TRUE WHERE id = 5",
		},
		{
			name:   "hdr column",
			input:  "SELECT * FROM media WHERE hdr = 1",
			expect: "SELECT * FROM media WHERE hdr = TRUE",
		},
		{
			name:   "dolby_vision column",
			input:  "SELECT * FROM media WHERE dolby_vision = 0",
			expect: "SELECT * FROM media WHERE dolby_vision = FALSE",
		},
		{
			name:   "dolby_atmos column",
			input:  "SELECT * FROM media WHERE dolby_atmos = 1",
			expect: "SELECT * FROM media WHERE dolby_atmos = TRUE",
		},
		{
			name:   "is_synced column",
			input:  "SELECT * FROM sync WHERE is_synced = 0",
			expect: "SELECT * FROM sync WHERE is_synced = FALSE",
		},
		{
			name:   "verified_sync column",
			input:  "UPDATE t SET verified_sync = 1 WHERE id = 3",
			expect: "UPDATE t SET verified_sync = TRUE WHERE id = 3",
		},
		{
			name:   "is_smart column",
			input:  "SELECT * FROM playlists WHERE is_smart = 1",
			expect: "SELECT * FROM playlists WHERE is_smart = TRUE",
		},
		{
			name:   "is_default column",
			input:  "SELECT * FROM roles WHERE is_default = 1",
			expect: "SELECT * FROM roles WHERE is_default = TRUE",
		},
		{
			name:   "is_forced column",
			input:  "SELECT * FROM subtitles WHERE is_forced = 0",
			expect: "SELECT * FROM subtitles WHERE is_forced = FALSE",
		},
		{
			name:   "is_system column",
			input:  "SELECT * FROM roles WHERE is_system = 1",
			expect: "SELECT * FROM roles WHERE is_system = TRUE",
		},
		{
			name:   "no boolean column present",
			input:  "SELECT * FROM files WHERE size = 0",
			expect: "SELECT * FROM files WHERE size = 0",
		},
		{
			name:   "query with no equals at all",
			input:  "SELECT COUNT(*) FROM files",
			expect: "SELECT COUNT(*) FROM files",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pg.RewriteBooleanLiterals(tt.input)
			assert.Equal(t, tt.expect, got)
		})
	}
}

// ---------------------------------------------------------------------------
// DialectType constants
// ---------------------------------------------------------------------------

func TestDialectType_Constants(t *testing.T) {
	assert.Equal(t, DialectType("sqlite"), DialectSQLite)
	assert.Equal(t, DialectType("postgres"), DialectPostgres)
}

func TestUpsertSQL(t *testing.T) {
	cols := []string{"id", "a", "b"}
	pg := Dialect{Type: DialectPostgres}
	sl := Dialect{Type: DialectSQLite}

	assert.Equal(t, "INSERT OR REPLACE INTO t (id, a, b) VALUES (?, ?, ?)", sl.UpsertSQL("t", cols, []string{"id"}))
	assert.Equal(t,
		"INSERT INTO t (id, a, b) VALUES (?, ?, ?) ON CONFLICT (id) DO UPDATE SET a = EXCLUDED.a, b = EXCLUDED.b",
		pg.UpsertSQL("t", cols, []string{"id"}))
	assert.Equal(t,
		"INSERT INTO t (a, b) VALUES (?, ?) ON CONFLICT (a, b) DO NOTHING",
		pg.UpsertSQL("t", []string{"a", "b"}, []string{"a", "b"}))
}
