# Script companion guides (index)

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-06 |
| Last modified | 2026-10-06T16:00:00Z |
| Status | tracked; created in round 7 (WF7 review F7-1, constitution 11.4.212 reachability); independent review of this revision owed (constitution 11.4.142) |
| Linked from | the root [`README.md`](../../README.md), documentation section |

One guide per script (constitution 11.4.18). Every page of this directory is listed here; a page missing from this table is a defect.
`$EV` in the guides is the evidence root `specs/001-full-project-audit-remediation/evidence`.

## Audit-remediation tooling (specs/001-full-project-audit-remediation)

| Guide | Script | Notes |
|---|---|---|
| [anti-bluff-scan.md](anti-bluff-scan.md) | `scripts/anti-bluff-scan.sh` | wrapper over the audit scanner |
| [check_classes.md](check_classes.md) | `scripts/repo/check_class.sh` and the class tables | |
| [check_review_provenance.md](check_review_provenance.md) | `scripts/review/check_review_provenance.sh` | review provenance checker |
| [derive_scope.md](derive_scope.md) | `scripts/audit/derive_scope.sh` | |
| [event_core.md](event_core.md) | `scripts/build/event_core.sh` | T005b slice; dispatcher and hub owed |
| [host_checklist.md](host_checklist.md) | `scripts/containers/host_checklist.sh` | docs/16 section 9.5 checklist |
| [index_health.md](index_health.md) | `scripts/audit/index_health.sh` | |
| [org_of.md](org_of.md) | `scripts/audit/org_of.py` | |
| [scope_to_lumen_json.md](scope_to_lumen_json.md) | `scripts/audit/scope_to_lumen_json.py` | |
| [verify_repos.md](verify_repos.md) | `scripts/repo/verify_repos.sh` | |
| [locked.md](locked.md) | `scripts/register/locked.sh` | T064 single-writer wrapper of the register |
| [backup_db.md](backup_db.md) | `scripts/register/backup_db.sh` | T064a pre-op backup (online backup, never `cp -al`) |
| [register_dump.md](register_dump.md) | `scripts/register/dump.sh` | T066 deterministic dump and the commit procedure |
| [register_export.md](register_export.md) | `scripts/register/export.sh` | T067 export, run recording, drift check |
| [reconcile.md](reconcile.md) | `scripts/register/reconcile.sh` | T067 reconciliation CSV and Markdown reports |
| [register_replay.md](register_replay.md) | `scripts/register/replay.sh` | T067a replay of the local journal onto a remote-side database |

## Coverage and matrix tooling (WP-23)

| Guide | Script | Notes |
|---|---|---|
| [bash-coverage.md](bash-coverage.md) | `scripts/bash-coverage.sh`, `scripts/coverage/bashcov.py` | T199 bash line-coverage harness (PS4 trace, path-aware attribution) |
| [check_exclusions.md](check_exclusions.md) | `scripts/coverage/check_exclusions.sh`, `scripts/coverage/fence_lib.py` | T200 exclusion-fence gate (11.4.224 E) |
| [track-coverage.md](track-coverage.md) | `scripts/coverage/track-coverage.sh`, `scripts/coverage/gocov_merge.py` | T198 Go coverage collector (split lanes) |
| [gen_matrix.md](gen_matrix.md) | `tools/evidence/matrix/gen_matrix.py`, `tools/evidence/matrix/derive_applicability.py` | T195-T197 coverage matrix and its gate |
| [run_kcov.md](run_kcov.md) | `scripts/containers/run_kcov.sh` | T200a IMG-KCOV wrapper |
| [run_rust.md](run_rust.md) | `scripts/containers/run_rust.sh` | T200a IMG-RUST wrapper (class compile, build host only) |

## Catalog and QA scripts

| Guide | Script |
|---|---|
| [catalog_aggregation_granularity.md](catalog_aggregation_granularity.md) | `catalog_aggregation_granularity.sh` |
| [catalog_auth_pagination.md](catalog_auth_pagination.md) | `catalog_auth_pagination.sh` |
| [catalog_books_comics_resume.md](catalog_books_comics_resume.md) | `catalog_books_comics_resume.sh` |
| [catalog_browse_filter_search.md](catalog_browse_filter_search.md) | `catalog_browse_filter_search.sh` |
| [catalog_collections_downloads_stats.md](catalog_collections_downloads_stats.md) | `catalog_collections_downloads_stats.sh` |
| [catalog_details_assets_caching.md](catalog_details_assets_caching.md) | `catalog_details_assets_caching.sh` |
| [catalog_episode_titles_dedup.md](catalog_episode_titles_dedup.md) | `catalog_episode_titles_dedup.sh` |
| [catalog_favorites_resume_lifecycle.md](catalog_favorites_resume_lifecycle.md) | `catalog_favorites_resume_lifecycle.sh` |
| [catalog_functional_matrix.md](catalog_functional_matrix.md) | `catalog_functional_matrix.sh` |
| [catalog_music_album_tracks.md](catalog_music_album_tracks.md) | `catalog_music_album_tracks.sh` |
| [catalog_playback_progress_favorites.md](catalog_playback_progress_favorites.md) | `catalog_playback_progress_favorites.sh` |
| [run-helixqa-androidtv.md](run-helixqa-androidtv.md) ([html](run-helixqa-androidtv.html), [pdf](run-helixqa-androidtv.pdf)) | `run-helixqa-androidtv.sh` |
| [visual_proof_challenge.md](visual_proof_challenge.md) | `visual_proof_challenge.sh` |

## Firebase, distribution and repository layout

| Guide | Script |
|---|---|
| [distribute.md](distribute.md) | `scripts/distribute.sh` |
| [firebase_setup_env.md](firebase_setup_env.md) | `scripts/firebase_setup_env.sh` |
| [firebase_verify.md](firebase_verify.md) | `scripts/firebase_verify.sh` |
| [reorg_submodules.md](reorg_submodules.md) | `scripts/reorg_submodules.sh` |

## Related audit-remediation documents (linked here so they are reachable from the root README)

- [Evidence index and tracked deviations (WP-09)](../../specs/001-full-project-audit-remediation/evidence/wp09/README.md)
- [Owner request list](../../specs/001-full-project-audit-remediation/decisions/owner-request-list.md)
- [Constitution prefix change request (upstream)](../../specs/001-full-project-audit-remediation/docs/upstream/constitution-prefix-change-request.md)
- [Continuum decision brief (upstream)](../../specs/001-full-project-audit-remediation/docs/upstream/continuum-decision-brief.md)

## Scripts without a guide yet (owed, constitution 11.4.18; WF7 F7-5)

Added to the repository without a guide here: `scripts/containers/{disk_headroom,probe_host,resolve_pin,smoke_images,smoke_probe}.sh`,
`scripts/ledger/project_gate_ledger_ratchet.sh`, `scripts/register/{apply_ext,gate,run_mutations}.sh`, `scripts/register/mutate_ddl.py`,
`scripts/repo/{check_no_ci,check_revision_headers,commit_recursive,fixture_roots,integrate_ff_only,integrate_merge,push_recursive,record_deferral,scope_check}.sh`.
Their file-header comments are the only description today. UNCONFIRMED: the list is the WF7 reviewer's count of 19 of 33 scripts, not re-derived here.
