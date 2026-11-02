package database

import (
	"fmt"
	"regexp"
	"strings"
)

// DialectType identifies the SQL dialect in use.
type DialectType string

const (
	DialectSQLite   DialectType = "sqlite"
	DialectPostgres DialectType = "postgres"
)

// Dialect provides helpers for cross-database SQL compatibility.
type Dialect struct {
	Type DialectType
}

// RewritePlaceholders converts ? placeholders to $1, $2, ... for PostgreSQL.
// SQLite queries are returned unchanged.
func (d *Dialect) RewritePlaceholders(query string) string {
	if d.Type != DialectPostgres {
		return query
	}

	var b strings.Builder
	b.Grow(len(query) + 32)
	n := 0
	inSingleQuote := false
	for i := 0; i < len(query); i++ {
		ch := query[i]
		if ch == '\'' {
			inSingleQuote = !inSingleQuote
			b.WriteByte(ch)
			continue
		}
		if ch == '?' && !inSingleQuote {
			n++
			fmt.Fprintf(&b, "$%d", n)
		} else {
			b.WriteByte(ch)
		}
	}
	return b.String()
}

// RewriteInsertOrIgnore converts "INSERT OR IGNORE INTO ..." to
// "INSERT INTO ... ON CONFLICT DO NOTHING" for PostgreSQL.
func (d *Dialect) RewriteInsertOrIgnore(query string) string {
	if d.Type != DialectPostgres {
		return query
	}
	upper := strings.ToUpper(query)
	if idx := strings.Index(upper, "INSERT OR IGNORE INTO"); idx != -1 {
		// Replace the prefix, keep original casing of everything after INTO
		prefix := query[:idx]
		rest := query[idx+len("INSERT OR IGNORE INTO"):]
		return prefix + "INSERT INTO" + rest + " ON CONFLICT DO NOTHING"
	}
	return query
}

// RewriteInsertOrReplace converts "INSERT OR REPLACE INTO ..." to
// PostgreSQL-compatible upsert syntax. Since the exact conflict target
// varies per table, this does a simple keyword replacement that callers
// can build on.
func (d *Dialect) RewriteInsertOrReplace(query string) string {
	if d.Type != DialectPostgres {
		return query
	}
	upper := strings.ToUpper(query)
	if idx := strings.Index(upper, "INSERT OR REPLACE INTO"); idx != -1 {
		prefix := query[:idx]
		rest := query[idx+len("INSERT OR REPLACE INTO"):]
		return prefix + "INSERT INTO" + rest
	}
	return query
}

// RewriteOnConflict converts SQLite's ON CONFLICT(cols) DO UPDATE SET ...
// syntax which is compatible with PostgreSQL as-is. No rewrite needed.

// AutoIncrement returns the correct auto-increment primary key clause.
func (d *Dialect) AutoIncrement() string {
	if d.Type == DialectPostgres {
		return "SERIAL PRIMARY KEY"
	}
	return "INTEGER PRIMARY KEY AUTOINCREMENT"
}

// TimestampType returns the column type for timestamps.
func (d *Dialect) TimestampType() string {
	if d.Type == DialectPostgres {
		return "TIMESTAMP"
	}
	return "DATETIME"
}

// BooleanDefault returns the default boolean value syntax.
func (d *Dialect) BooleanDefault(val bool) string {
	if d.Type == DialectPostgres {
		if val {
			return "DEFAULT TRUE"
		}
		return "DEFAULT FALSE"
	}
	if val {
		return "DEFAULT 1"
	}
	return "DEFAULT 0"
}

// CurrentTimestamp returns the current timestamp expression.
func (d *Dialect) CurrentTimestamp() string {
	return "CURRENT_TIMESTAMP"
}

// boolColumnPattern matches known boolean column names followed by = 0 or = 1.
// This rewrites integer boolean literals to TRUE/FALSE for PostgreSQL BOOLEAN columns.
var boolColumnPattern = regexp.MustCompile(
	`(?i)\b(is_active|is_locked|is_system|is_default|is_forced|is_duplicate|is_directory|` +
		`deleted|enabled|verified_sync|is_favorite|is_public|is_smart|shuffle_enabled|` +
		`hdr|dolby_vision|dolby_atmos|is_synced)\s*=\s*([01])\b`)

// RewriteBooleanLiterals converts "column = 0" → "column = FALSE" and
// "column = 1" → "column = TRUE" for known boolean columns in PostgreSQL.
// SQLite queries are returned unchanged.
func (d *Dialect) RewriteBooleanLiterals(query string) string {
	if d.Type != DialectPostgres {
		return query
	}
	return boolColumnPattern.ReplaceAllStringFunc(query, func(match string) string {
		if strings.HasSuffix(strings.TrimSpace(match), "1") {
			return boolColumnPattern.ReplaceAllString(match, "${1} = TRUE")
		}
		return boolColumnPattern.ReplaceAllString(match, "${1} = FALSE")
	})
}

// IsSQLite returns true if the dialect is SQLite.
func (d *Dialect) IsSQLite() bool {
	return d.Type == DialectSQLite
}

// IsPostgres returns true if the dialect is PostgreSQL.
func (d *Dialect) IsPostgres() bool {
	return d.Type == DialectPostgres
}

// UpsertSQL builds a portable single-row upsert for table.
//
// SQLite (the go-sqlcipher build predates SQLite 3.24, so ON CONFLICT ... DO
// UPDATE is a syntax error there) gets "INSERT OR REPLACE INTO t (cols)
// VALUES (?, ...)". PostgreSQL gets "INSERT INTO t (cols) VALUES (?, ...) ON
// CONFLICT (conflictCols) DO UPDATE SET c = EXCLUDED.c ..." because the plain
// INSERT that RewriteInsertOrReplace produces fails with a duplicate key on
// the second save (or silently duplicates rows when no unique key exists).
// The conflict columns MUST be backed by a PRIMARY KEY or UNIQUE index. If
// every column is a conflict column, DO NOTHING is used on PostgreSQL.
// Placeholders stay "?" (the DB wrapper rewrites them).
func (d *Dialect) UpsertSQL(table string, cols, conflictCols []string) string {
	marks := make([]string, len(cols))
	for i := range marks {
		marks[i] = "?"
	}
	head := " INTO " + table + " (" + strings.Join(cols, ", ") + ") VALUES (" + strings.Join(marks, ", ") + ")"
	if d.Type != DialectPostgres {
		return "INSERT OR REPLACE" + head
	}
	isConflict := make(map[string]bool, len(conflictCols))
	for _, c := range conflictCols {
		isConflict[c] = true
	}
	var sets []string
	for _, c := range cols {
		if !isConflict[c] {
			sets = append(sets, c+" = EXCLUDED."+c)
		}
	}
	q := "INSERT" + head + " ON CONFLICT (" + strings.Join(conflictCols, ", ") + ") "
	if len(sets) == 0 {
		return q + "DO NOTHING"
	}
	return q + "DO UPDATE SET " + strings.Join(sets, ", ")
}
