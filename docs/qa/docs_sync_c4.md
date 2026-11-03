# Docs sync C4 - documentation vs reality audit

| Field | Value |
|---|---|
| Date | 2026-11-03 |
| Base | `main` 6eaf091e, branch `docs-sync-c4` |
| Tool | `scripts/docs/check_doc_coverage.py` (guide: [check_doc_coverage.md](../scripts/check_doc_coverage.md)), test `scripts/docs/tests/test_check_doc_coverage.py` |
| Status | independent review owed (constitution 11.4.142) |

## Gap counts (machine output)

| Check | Items | Gaps before | Fixed in docs | Exempt / still owed | Gaps after |
|---|---|---|---|---|---|
| link checker (`check_md_links.py --alt-root <live checkout>`) | 2395 files | 0 broken | 0 | 0 | 0 (the new `docs/qa/README.md` link to this file resolves once this file exists) |
| routes (code vs `docs/api/openapi.yaml`, both directions) | 247 | 0 | 0 | 0 | 0 |
| env (catalog-api Go env reads vs the two ENV docs) | 84 | 51 | 38 | 13 exempt (4 identity-suffix false positives, 9 test-helper names) | 0 |
| scripts (`scripts/<file>` vs `docs/scripts/<name>.md`) | 109 | 90 | 0 | 90 exempt, guide owed (FOLLOW_UPS D1) | 0 (90 counted as exempt) |
| README links the docs index | 1 | 0 | 0 | 0 | 0 |
| behaviour claims (doc says it AND code still does it) | 7 | 5 | 5 | 0 | 0 |
| total | | 146 | 43 | 103 | 0 (`--exempt /dev/null`: 103 = 90 + 13) |

Before = the new script run over the pristine base docs with exemptions disabled. After-without-exemptions: `total gaps: 104` in the captured run (103 plus the 1 new broken link of this file, resolved by creating it).

Also fixed by the reachability check (`check_doc_reachability.py`): `docs/UPGRADE_NOTES.md` and `docs/security/LEGACY_PASSWORD_HASH_SUNSET.md` were orphans on the base (verified by running it on the live checkout); both are now linked from `docs/README.md`.

## What was changed in the docs, with its code anchor

| Doc change | Verified against |
|---|---|
| 20 env rows (38 variable names) added to `docs/deployment/ENVIRONMENT_VARIABLES.md` | `handlers/auth_handler.go:172`, `main_adminreset.go:37`, `main.go:376,953,974,1055`, `main_malloc_glibc.go:15-20`, `config/config.go:208,266-285,487`, `internal/services/smb_probe.go:48-70`, `llm_image_resolver.go:45-51`, `cover_art_archive_resolver.go:37-39`, `igdb_resolver.go:43-44`, `internal/firebase/firebase.go:244-248,303-305`, `challenges/config.go:58-60`, `internal/infra/provisioner.go:52-58,109-121` (all under `catalog-api/`) |
| `docs/api/openapi.yaml` `DeepHealthResponse`: the documented `checks` map did not exist; the real body is `status, timestamp, version, uptime, components, converters`; 503 added | `catalog-api/internal/metrics/health.go:29-44`, `catalog-api/main.go:1100-1108` |
| converter tools section in `docs/deployment/DOCKER_SETUP.md` | `catalog-api/Dockerfile:64-68`, `catalog-api/Dockerfile.runtime:18-21`, `health.go:44` |
| known limitations in `docs/UPGRADE_NOTES.md`: share controls disabled, smart playlists 501, converters in image, go-fitz removal pending | `catalog-web/src/components/collections/CollectionSharing.tsx:169-170,643`, `catalog-api/handlers/playlist_handler.go:86-91`, `handlers/pdf_pages_handler.go:19`, `catalog-api/go.mod:84` |
| go-fitz status line in `docs/phases/PHASE_2_IMPLEMENTATION_DOCUMENTATION.md` | same go-fitz anchors |

Statements already accurate and now guarded by the `claims` check: REGISTRATION_MODE approval default (`docs/UPGRADE_NOTES.md`, `docs/ENV_VARIABLES.md:117`, `docs/api/API_DOCUMENTATION.md:415`; code `auth_handler.go:162-180`), smart playlists 501 (`docs/api/API_DOCUMENTATION.md:4892`).

## Captured output (excerpts)

Before (`check_doc_coverage.py --exempt /dev/null` on the base docs):
```
check claims: 7 items, 5 gaps
check env: 84 items, 51 gaps
check readme: 1 items, 0 gaps
check routes: 247 items, 0 gaps
check scripts: 109 items, 90 gaps
total gaps: 146
```
After (`check_doc_coverage.py`, default exemptions): all five checks `0 gaps`, `total gaps: 0`, exit 0.
Test: `python3 scripts/docs/tests/test_check_doc_coverage.py` ends `failed: 0` (13 tests: clean fixture, undocumented route, phantom route, helper-registered route missing from spec, undocumented env, whole-word env match, script without guide, hyphen/underscore guide name, README without index link, exemption needs a reason, BLIND on empty spec, claims doc/code mutation, live repository 0 gaps).

## Finding about an existing gate

`scripts/check_api_route_vs_spec.py` exits 1 on this tree with "IN SPEC, NOT IN CODE (21)": it does not follow `Register*Routes` helpers (`main.go:1347,1384,1702`; `handlers/storage_routes.go`, `handlers/alias_routes.go`), so 21 registered routes read as phantoms. `check_doc_coverage.py routes` follows them and reports 0 gaps in both directions. Not changed here (no code changes outside the new script); recorded as FOLLOW_UPS D2.

## Remaining

See `docs/FOLLOW_UPS_NEXT_PHASE.md`, section "Docs sync C4": D1 (90 script guides), D2 (route gate helper expansion), D3 (13 exempt env names), D4 (go-fitz code removal), D5 (other `/health/deep` prose not re-checked).
