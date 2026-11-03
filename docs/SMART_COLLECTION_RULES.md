# Smart collection rules: dialects, timeout, quality

Package: `catalog-api/internal/smartrules`. A rule document is a closed JSON array; it is compiled to a
parameterised WHERE fragment over `media_items mi` / `media_types mt` and run on SQLite (SQLCipher, no
JSON1) or PostgreSQL.

## Field x operator matrix

| Field kind | Fields | Operators |
|---|---|---|
| text | title, genre, language, country, path, quality | equals, not_equals, contains, not_contains, starts_with, ends_with, in, not_in |
| tags (user scoped) | tags | contains, not_contains |
| number | year, rating, runtime, size | equals, not_equals, greater_than, greater_or_equal, less_than, less_or_equal, between, in, not_in |
| date | added, updated | equals, greater_than, greater_or_equal, less_than, less_or_equal, between, in_last_days |
| favorite (user scoped) | favorite | equals, not_equals (boolean) |
| media_type | media_type | equals, not_equals, in, not_in |

`TestRuleMatrix_DialectParity` runs every field x operator (plus groups, nesting and OR) on real SQLite and
real PostgreSQL and requires the same titles from both.

## Genre

`media_items.genre` is a JSON array of strings. Positive operators mean "some element matches"; negative
ones mean "no element matches" (an item without genres matches them). SQLite uses string functions with
JSON escape handling (no JSON1); PostgreSQL uses `jsonb_array_elements_text` inside `EXISTS (...)`.
The PostgreSQL predicate is already a non-null boolean, so it must NOT be wrapped in `COALESCE(..., 0)`
(a type error on PostgreSQL); only the SQLite predicate is NULL-wrapped.

## Quality

`media_files.quality_info` is the `QualityInfo` JSON object; the `quality` field compares its
**top-level** `quality_profile` (lower-cased). A same-named key nested inside another object does not
count on either dialect. SQLite limits: the first occurrence of the key decides, and a `}` inside an
earlier string value makes the rule not match (safe direction). Anything that is not an object yields NULL.

## Timeout

One `Evaluate` / `CountUpTo` call (all its queries together) is bounded by `Options.Timeout`, default
`DefaultTimeout` = 10 s; page size and count are capped (`NormalizePage`, `CountUpTo` max).

## Tests (real databases, no mocks)

SQLite is a real migrated database; PostgreSQL is `postgres:15-alpine` started rootless with podman on
loopback (random port and password in a 0600 env file, container removed afterwards); without podman that
leg SKIPs. `smartrules_quality_test.go` holds the reviewer-authored mutation tests, including the
reverted-`COALESCE` mutant, which both a compile-level check and the real-PostgreSQL edge cases kill.
