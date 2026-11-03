# Smart collections and smart playlists

A smart collection (or smart playlist) stores rules instead of members. Its items are computed when you read it, so it always reflects the current catalogue.

## Create

```
POST /api/v1/collections
{"name": "Recent sci-fi", "is_smart": true,
 "smart_rules": [
   {"field": "genre", "operator": "contains", "value": "Sci-Fi"},
   {"field": "added", "operator": "in_last_days", "value": 30}
 ]}
```

Playlists use the same body on `POST /api/v1/playlists`. Read items with `GET /api/v1/collections/{id}/items?limit=50&offset=0` (or `page=2`), or `GET /api/v1/playlists/{id}`.

## Rules

Each rule is `{field, operator, value}`. A rule joins the previous one with `"condition": "and"` (default) or `"or"`; AND binds tighter than OR. `nested_rules` are ANDed with their parent rule.

| Field | Operators |
|---|---|
| title, genre, language, country, path, quality | equals, not_equals, contains, not_contains, starts_with, ends_with, in, not_in |
| media_type (movie, tv_show, tv_season, tv_episode, music_artist, music_album, song, game, software, book, comic; groups video, music, document) | equals, not_equals, in, not_in |
| year, rating, runtime, size (bytes) | equals, not_equals, greater_than, greater_or_equal, less_than, less_or_equal, between, in, not_in |
| added, updated (aliases created_at, added_at, updated_at) | equals, greater_than, greater_or_equal, less_than, less_or_equal, between, in_last_days |
| tags | contains, not_contains |
| favorite | equals, not_equals |

`tags` and `favorite` come from your own data, so another user never sees matches based on yours.

Limits: 25 rules, depth 3, 256 bytes per value, 50 values per list, 64 KB per rule set, at most 500 items per page. Anything else (unknown field or operator, wrong value type, unknown key) answers 400 and nothing is stored.

## Access

Only the owner or an administrator edits or deletes. A private collection of someone else answers 404. A smart collection cannot be turned into a manual one, and the reverse is refused. Adding items to a smart playlist answers 409.
