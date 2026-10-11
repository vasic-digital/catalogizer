# WP-06 T059: the workable-items engine facts the register extension depends on

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-08T13:26:22Z |
| Last modified | 2026-10-08T13:26:22Z |
| Status | draft, UNREVIEWED (written by the T068 fix round to close finding R-07; every fact below was read from the engine source in the pinned submodule or measured by running the committed binary on a scratch database in this session) |
| Task | T059 (deliverable named in tasks.md: `$EV/wp06/engine.md`); consumed by T060 (`apply_ext.sh`, `engine_schema_required`) and T063 (`gate.sh`, step_engine_version) |
| Engine binary | `submodules/constitution/scripts/workable-items/bin/workable-items-linux`, sha256 `d789d6251326b5236b98cc68ff6ce6ae602136ea3f10391ffb89cf3005d430da`; `bin/workable-items` is byte-identical (same sha256); `bin/.source.sha256` holds `2b0a50f85b12c8f13c886c39faacde31ba34b2338b9bed589beb44db1d233d84` (what it hashes is UNCONFIRMED: not recomputed here) |
| Constitution submodule commit | `1634b0353a1feaa9f1cb08a5d8c37eba3d5ff079` |
| Engine schema required | **7** (`meta.schema_version`), hard-coded as `engine_schema_required='7'` in `scripts/register/register_ext.sql` |

## 1. Version

The binary has no `version` / `--version` subcommand: both print `unknown subcommand` and the usage text (measured). The engine's identity is therefore the binary sha256 above plus the `meta.schema_version` row it writes. The Go source is in `submodules/constitution/scripts/workable-items/cmd/workable-items/`; the binary was NOT rebuilt here (UNCONFIRMED that it equals a build of that source).

## 2. `validate --db <p>`

* On an absent or empty file it creates the engine schema and prints `validate: OK — 0 items, all invariants satisfied`, exit 0 (measured on a fresh scratch path). This is why `apply_ext.sh` bootstraps a fresh DB with `$WI validate --db`.
* The embedded schema (`cmd/workable-items/schema_embed.sql:397`) seeds `meta.schema_version = '7'`. Measured on the freshly validated DB: `meta` holds `integrity_hash`, `last_sync_direction=none`, `last_sync_timestamp`, `schema_version=7`. `schema.sql` at the top of the directory is an older copy that seeds `'4'` (line 335); it is not what the binary embeds (`db.go:25` `go:embed schema_embed.sql`).
* Opening a DB migrates it forward only: `db.go:179` raises `schema_version` to 6, `occurred_at.go:89` to 7, `version_tags.go` to 3, each with `CAST(value AS INTEGER) < N` (never downgrades). So `validate` can change `meta` of an older DB, which is why `gate.sh` runs it on a copy and checks the version BEFORE it (step order, m-4).

## 3. `add <type> <severity> --db <p> --title --description [--id <id>] [--prefix <P>]`

* `crud.go:73` `--id`: "explicit ticket id (auto-generated when absent)"; an explicit id is used as given after an `itemExists(db, id, "Issues")` check that only refuses a duplicate (`crud.go:127-135`); the `--prefix` is used only for AUTO-generated ids (`nextID(db, prefix)`, `crud.go:121` and `:414`). The default prefix is derived from `HELIX_RELEASE_PREFIX` / the project directory name (`prefix.go`, `defaultKeyPrefix()`), so it is not a stable register key by itself.
* **The engine enforces no id grammar on `--id`.** Measured on a bare engine DB: `add ... --id CAT-001`, `--id CAT--03` and `--id cat-9` were all accepted (exit 0) and stored. The tracker heading parser accepts only `^## [A-Z]{3}-[0-9A-Za-z]+` (`parse.go:21`) and the bracket form `\[[A-Z]{2,}-\d+\]` (`parse.go:32`), so a malformed key would be dropped by a db to md to db round trip. The register therefore enforces the grammar itself: `trg_items_require_mint` refuses an item whose id was not minted in `reg_ids`, and `reg_ids` (v6) can only hold `CAT-NNN` ids (trigger `reg_ids_mint_monotone`, view `v_ids_malformed`).
* The description floor is 6 words or 40 characters (`crud.go`, §11.4.91).
* The legacy key column is `atm_id` (also `items.atm_id`). It is KEPT: the rename to `ticket_id` is owed ENGINE work (schema migration plus a compatibility view, constitution 2026-10-08 extension of 11.4.54), not done in this repository.

## 4. Foreign keys

`db.go:44` opens every engine connection with `?_foreign_keys=on`. A plain `sqlite3` shell connection has `foreign_keys` OFF; the register DDL sets the pragma only for the connection that applies it, so every register writer must set it (docs/04 limitation 7), and `gate.sh` runs `PRAGMA foreign_key_check`.

## 5. What the register extension depends on

| Dependency | Where used | Fact |
|---|---|---|
| `meta.schema_version = 7` | `register_ext.sql` `engine_schema_required`, `apply_ext.sh` (exit 3 on mismatch), `gate.sh` step_engine_version | measured 7 on a freshly validated DB |
| `validate` bootstraps an empty file | `apply_ext.sh`, `gate.sh` reference build | measured |
| `add --id` accepts any id, no grammar | the mint triggers and `v_ids_malformed` of the DDL | measured (CAT--03, cat-9 accepted) |
| `items` columns `atm_id`, `current_location`, `representation` | `trg_items_*`, views | unchanged in this round |

## 6. Honest limits

UNCONFIRMED: that the binary equals a build of the source tree in the submodule; the meaning of `.source.sha256`; behaviour of engine versions other than this binary. The measurements were run on the host (sqlite3 `3.50.6` from the Android SDK path), not in a container image.
