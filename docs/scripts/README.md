# Script companion guides (index)

| Field | Value |
|---|---|
| Revision | 6 |
| Created | 2026-10-06 |
| Last modified | 2026-11-03 (merged: added check_api_route_vs_spec.md; added the Documentation tooling section and the mechanically listed guides table; descriptions of sync_trackers, check_task_citations, test_runners_hardening rewritten); earlier: 2026-10-30 (added check_submodule_ff_range.md; earlier 2026-10-29: added check_submodule_ff.md; earlier 2026-10-08: added lumenignore_skeleton.md, review WP-02-T028-r2 N3; earlier: added scope_drift_check.md, review WP-02-T028 F4/F5) |
| Status | tracked; created in round 7 (WF7 review F7-1, constitution 11.4.212 reachability); independent review of this revision owed (constitution 11.4.142) |
| Linked from | the root [`README.md`](../../README.md), documentation section |

One guide per script (constitution 11.4.18). Every page of this directory is listed here; a page missing from this table is a defect.
`$EV` in the guides is the evidence root `specs/001-full-project-audit-remediation/evidence`.

## Audit-remediation tooling (specs/001-full-project-audit-remediation)

| Guide | Script | Notes |
|---|---|---|
| [anti-bluff-scan.md](anti-bluff-scan.md) | `scripts/anti-bluff-scan.sh` | wrapper over the audit scanner |
| [check_api_route_vs_spec.md](check_api_route_vs_spec.md) | `scripts/check_api_route_vs_spec.py`, `scripts/lib/gin_routes_ast.go` | Gin router vs `docs/api/openapi.yaml` drift gate (go/ast extraction) |
| [check_classes.md](check_classes.md) | `scripts/repo/check_class.sh` and the class tables | |
| [check_review_provenance.md](check_review_provenance.md) | `scripts/review/check_review_provenance.sh` | review provenance checker |
| [derive_scope.md](derive_scope.md) | `scripts/audit/derive_scope.sh` | |
| [event_core.md](event_core.md) | `scripts/build/event_core.sh` | T005b slice; dispatcher and hub owed |
| [host_checklist.md](host_checklist.md) | `scripts/containers/host_checklist.sh` | docs/16 section 9.5 checklist |
| [index_health.md](index_health.md) | `scripts/audit/index_health.sh` | |
| [org_of.md](org_of.md) | `scripts/audit/org_of.py` | |
| [scope_to_lumen_json.md](scope_to_lumen_json.md) | `scripts/audit/scope_to_lumen_json.py` | |
| [scope_drift_check.md](scope_drift_check.md) | `scripts/audit/scope_drift_check.sh` | |
| [lumenignore_skeleton.md](lumenignore_skeleton.md) | `scripts/audit/lumenignore_skeleton.py` | |
| [verify_repos.md](verify_repos.md) | `scripts/repo/verify_repos.sh` | |
| [check_submodule_ff.md](check_submodule_ff.md) | `scripts/check_submodule_ff.sh` | gitlink fast-forward gate: own-org NONFF fails, third-party reported, missing pin BLIND (tests: `scripts/tests/test_check_submodule_ff.sh`, `scripts/tests/test_check_submodule_ff_mutation.sh`) |
| [check_submodule_ff_range.md](check_submodule_ff_range.md) | `scripts/repo/check_submodule_ff_range.sh` | CPA stage S3 row `submodule_ff`: runs the gate over `<remote main>..HEAD`; NONFF fails S3, BLIND is a recorded deferral (test: `scripts/repo/tests/test_check_submodule_ff_range.sh`) |
| [verify_helix_deps_manifests.md](verify_helix_deps_manifests.md) | `scripts/verify_helix_deps_manifests.sh` | 11.4.31 helix-deps.yaml verifier; thin wrapper delegating to `constitution/scripts/deps_manifest/verify_helix_deps_manifests.sh`, canonical exit codes 0/1/2/64 (test: `scripts/tests/test_verify_helix_deps_manifests.sh`) |
| [webdav_yandex_live.md](webdav_yandex_live.md) | `scripts/test-infra/webdav_yandex_live.sh` | Live N-run suite of the real WebDAVClient against the real Yandex WebDAV, confined to `/Testing`; verdicts PASS / PASS_WITH_ANOMALIES / FAIL / SKIPPED, exit 0/1/2/3 |
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

## Documentation tooling

| Guide | Script |
|---|---|
| [check_doc_reachability.md](check_doc_reachability.md) | `scripts/docs/check_doc_reachability.py` |
| [check_doc_reachability.md](check_doc_reachability.md) (section on index regeneration) | `scripts/docs/gen_dir_indexes.py` |
| [check_html_viewport.md](check_html_viewport.md) | `scripts/docs/check_html_viewport.py` |
| [check_doc_coverage.md](check_doc_coverage.md) | `scripts/docs/check_doc_coverage.py` |
| [check_md_links.md](check_md_links.md) | `scripts/docs/check_md_links.py` |
| [render_md_to_html.md](render_md_to_html.md) | `scripts/docs/render_md_to_html.sh` |
| [render_md_to_pdf.md](render_md_to_pdf.md) | `scripts/docs/render_md_to_pdf.sh`, `scripts/docs/check_pdf_coverage.py`, `scripts/docs/check_twin_presence.py` |

## Related audit-remediation documents (linked here so they are reachable from the root README)

- [Evidence index and tracked deviations (WP-09)](../../specs/001-full-project-audit-remediation/evidence/wp09/README.md)
- [Owner request list](../../specs/001-full-project-audit-remediation/decisions/owner-request-list.md)
- [Constitution prefix change request (upstream)](../../specs/001-full-project-audit-remediation/docs/upstream/constitution-prefix-change-request.md)
- [Continuum decision brief (upstream)](../../specs/001-full-project-audit-remediation/docs/upstream/continuum-decision-brief.md)

## Guides added without a category above (mechanically listed, 2026-11-02)

Descriptions are copied mechanically from each guide (first heading and first prose line).

| Guide | Description |
|---|---|
| [anti_mess_sweep.md](anti_mess_sweep.md) | anti-mess sweep and invariant catalogue - Companion Guide - The level-triggered control plane of constitution 11.4.233 at project scale (docs/12 section 17, docs/16 section 13.3): it RE-DERIVES the actual persistent state and... |
| [api_contract_live.md](api_contract_live.md) | api_contract_live.sh - Live (constitution 11.4.289: no mocks, stubs or fixture servers) contract sweep of every operation in `docs/api/openapi.yaml` against a real `catalog-api` binary built from the tree. |
| [appendix_token_check.md](appendix_token_check.md) | appendix_token_check.py - Companion Guide - The Spec Kit appendix (`.specify/memory/constitution-appendix.md`, Part 1) says every digest states every `CM-*` gate name and every |
| [blocked_external.md](blocked_external.md) | blocked_external.sh - Companion Guide - Records the legs of the real-service stack that need an external credential or device: `available`, or `blocked-unavailable` with the reason and the variable NAMES... |
| [build_in_container.md](build_in_container.md) | scripts/build_in_container.sh - Builds `catalog-api` (Go) inside a container (CLAUDE.md section 11.4.173: no bare-host builds). |
| [build_snapshot.md](build_snapshot.md) | snapshot.py, treecache.py, keyring.sh - User Guide (T005b) - Companion guide (Helix Constitution section 11.4.18) for `scripts/build/lib/snapshot.py` (driver side), `scripts/build/lib/treecache.py` (build-host side, python3 only,... |
| [check_evidence_binding.md](check_evidence_binding.md) | check_evidence_binding.sh (test infrastructure) - Companion Guide - Evidence must bind to the committed files (WF17 TI-H1). A capture's `# sha256 <path> <hash>` identity header is what the file under test hashed to when the capture ran;... |
| [check_freeze_listing.md](check_freeze_listing.md) | check_freeze_listing.sh - Companion Guide - `$EV` in this guide means the evidence root `specs/001-full-project-audit-remediation/evidence` (repository-relative). |
| [check_licence_swallow.md](check_licence_swallow.md) | check_licence_swallow.sh - Companion Guide - Owner decision 2026-10-07 (1): a licence-acceptance step (`sdkmanager --licenses`) must fail loudly. `yes \| sdkmanager --licenses ... \|\| true` hid a missing licence... |
| [check_pins.md](check_pins.md) | check_pins.sh - Companion Guide - Fails when a compose file, a Dockerfile or Containerfile, or a shell script references an external image without a full |
| [check_python_version.md](check_python_version.md) | check_python_version.sh - Companion Guide - Owner decision 2026-10-07 (2): the project tooling (the Python helpers under `scripts/`, the register, coverage, governance and review tools) requires **Python >= 3.11**. |
| [check_task_citations.md](check_task_citations.md) | check_task_citations.py - binds each task id cited outside `tasks.md` to a keyword the cited task text must still contain, so a renumbering is detected |
| [commit-push-all.md](commit-push-all.md) | commit-push-all.sh and cpa-host - Companion Guide - `$EV` means the evidence root `specs/001-full-project-audit-remediation/evidence`; `$CPA_RUN` the run directory `.audit/commit-push/<run id>/` of one run. |
| [conduit_to_ledger.md](conduit_to_ledger.md) | conduit_to_ledger.py - Companion Guide - Adapts a HelixQA conduit stream (`conduit.events.jsonl`, doc12 section 8.2) to `qa-verdict/1` ledger lines. SKIP makes a deterministic-lane run invalid; OPERATOR-BLOCKED... |
| [container_build.md](container_build.md) | container-build.sh - Companion Guide - The containerized build entry point. `docker-compose.build.yml` requires the per-run credentials and random host ports of its postgres/redis services (`${TI_*:?}`), so a... |
| [container_local.md](container_local.md) | scripts/container-local.sh - Builds and tests Catalogizer inside rootless podman on the LOCAL host; nothing compiles or installs on the bare host (CLAUDE.md section 11.4.173, section 11.4.161). |
| [disk_headroom.md](disk_headroom.md) | disk_headroom.sh - Companion Guide - Free-space headroom gate (constitution 12.9) run before every image pull or build and by `run_pinned.sh` ([guide](run_pinned.md)). It checks the |
| [dispatch.md](dispatch.md) | dispatch.sh, remote/emit.sh, lib/evwait.py: User Guide (T005b slice) - Companion guide (Helix Constitution section 11.4.18) for `scripts/build/dispatch.sh`, `scripts/build/remote/emit.sh`, `scripts/build/lib/evwait.py` and |
| [dotenv_get.md](dotenv_get.md) | dotenv_get.py (test infrastructure) - Companion Guide - The ONE dotenv reader of the test infrastructure (WF17 TI-D11). `nas_readonly_leg.sh` and `blocked_external.sh` used different `sed` pipelines and disagreed about the... |
| [enumerate_sources.md](enumerate_sources.md) | enumerate_sources.py - Companion Guide - `$EV` in this guide means the evidence root `specs/001-full-project-audit-remediation/evidence` (repository-relative). |
| [envelope.md](envelope.md) | envelope.sh - Companion Guide - Computes the dynamic resource envelope of docs/16 section 8.2 (12.6, 12.11, 12.12) from the live host, read-only, so that no limit of a |
| [event_hub.md](event_hub.md) | event_hub.sh - User Guide (T005b) - Companion guide (Helix Constitution section 11.4.18) for `scripts/build/event_hub.sh` and the event source `scripts/build/lib/evwait.py serve`. Part of T005b (owner... |
| [evidence-recorder.md](evidence-recorder.md) | evrec, verify, evanchor: evidence recorder - Companion Guide - `$EV` in this guide means the evidence root `specs/001-full-project-audit-remediation/evidence` (repository-relative). The contract is... |
| [evidence-ts02.md](evidence-ts02.md) | census_ab_pass.py and lib/ab_pass_with_evidence.sh: TS-02 adoption - Companion Guide - Eleven identical per-script copies of `ab_pass_with_evidence` could print `PASS` without any ev/1 entry. `lib/ab_pass_with_evidence.sh` is now the ONE definition: it... |
| [evidence-verdict.md](evidence-verdict.md) | verdict, evverdict.py: verdict deriver - Companion Guide - `verdict ITEM [--ledger FILE] [--blobs DIR] [--chain-only] [--state-delta]` DERIVES the verdict of a register item from the ledger by program (DR-E4, constitution... |
| [evidence-wrappers.md](evidence-wrappers.md) | wrap-go.sh, wrap-bash.sh, wrap-vitest.sh, wrap-gradle.sh, wrap-cargo.sh, evparse.py: runner wrappers - Companion Guide - `$EV` means `specs/001-full-project-audit-remediation/evidence`. |
| [ftps_fixture.md](ftps_fixture.md) | ftps_fixture.sh - Companion Guide - Runs the integration tests of `submodules/filesystem/pkg/ftp` (`-tags integration`) against a **real pure-ftpd** that accepts clear-text FTP and explicit FTPS... |
| [gen_env.md](gen_env.md) | gen_env.sh - Companion Guide - Writes the per-run credentials and random host ports of ONE compose project of the test-infrastructure stack (docs/16 section 10.4), so that no credential and no |
| [gen_media_fixtures.md](gen_media_fixtures.md) | gen_media_fixtures.sh - `scripts/test-infra/gen_media_fixtures.sh` generates the deterministic media and document corpus used by the live validation matrix (playback, conversions, documents, subtitles). |
| [generic_scanner.md](generic_scanner.md) | generic_scanner - Companion Guide (PA-03, WP-12) - One breadth-first scanner over the `filesystem.FileSystemClient` interface. It replaces the FTP, NFS and WebDAV scanner bodies that were empty (`return nil`), which let... |
| [import.md](import.md) | import.sh (import_tickets.py) - Companion Guide - `$EV` in this guide means the evidence root `specs/001-full-project-audit-remediation/evidence` (repository-relative). |
| [import_plan.md](import_plan.md) | import_plan.sh (import_plan.py) - Companion Guide - `$EV` in this guide means the evidence root `specs/001-full-project-audit-remediation/evidence` (repository-relative). |
| [import_t172.md](import_t172.md) | import_t172.sh (import_t172.py) - Companion Guide - `$EV` in this guide means the evidence root `specs/001-full-project-audit-remediation/evidence` (repository-relative). |
| [index_writer_preflight.md](index_writer_preflight.md) | index_writer_preflight.sh - Read-only host preflight for the code-index writer. Owner decision 2026-10-07: this host has 30.2 GiB RAM, so the upstream 32 GiB MemAvailable floor can never pass; the... |
| [intake_match.md](intake_match.md) | test_intake_match.sh - Companion Guide - `$EV` in this guide means the evidence root `specs/001-full-project-audit-remediation/evidence` (repository-relative). |
| [inventory_qa_classes.md](inventory_qa_classes.md) | inventory_qa_classes.py - Companion Guide - A member inventory of the QA-instrument classifier (`import_tickets.qa_cause_of`, the one function T168 and T172 share) over a ticket directory. It runs the REAL... |
| [lead_scan.md](lead_scan.md) | lead_scan.py and lead_scan_population.py - Companion Guide - `$EV` in this guide means the evidence root `specs/001-full-project-audit-remediation/evidence` (repository-relative). |
| [longops.md](longops.md) | longops (long-operation registry) - Companion Guide - The single source of truth for what long operation is running, who owns it, its state and its PROVEN liveness (constitution 11.4.232 A to E; |
| [nas_clients_leg.md](nas_clients_leg.md) | nas_clients_leg.sh - Companion Guide - Validates the new Go protocol clients of `submodules/filesystem` against the owner's real Synology hosts (`docs/infrastructure/synology-hosts.md`), READ-ONLY, one... |
| [nas_proto.md](nas_proto.md) | client/nas_proto.py - Companion Guide - The python client (stdlib only) of the NAS protocol survey, run inside `IMG-GO` through `scripts/containers/run_pinned.sh` by `nas_protocols.sh`: `nas_proto.py... |
| [nas_proto_nfs.md](nas_proto_nfs.md) | client/nas_proto_nfs.sh - Companion Guide - Bounded READ-ONLY walk of the NFS exports of ONE host through the libnfs user-space client inside `IMG-INFRA-CLIENT` (no kernel mount): `nas_proto_nfs.sh <ip> <1-7>... |
| [nas_protocols.md](nas_protocols.md) | nas_protocols.sh - Companion Guide - READ-ONLY survey of the owner's seven Synology hosts (`docs/infrastructure/synology-hosts.md`) over NFS, FTP, explicit FTPS and SFTP (SMB is covered by... |
| [nas_readonly_leg.md](nas_readonly_leg.md) | nas_readonly_leg.sh - Companion Guide - The clearly separated READ-ONLY leg against the owner's real Synology hosts (`docs/infrastructure/synology-hosts.md`; ODG-08, ODG-01). Not part of the deterministic... |
| [nfs_attempt.md](nfs_attempt.md) | nfs_attempt.sh - Companion Guide - Runs the unprivileged NFS attempt end to end and records its terminal state: build the image (`nfs_build.sh`), start the project with the `nfs` service, three round... |
| [nfs_build.md](nfs_build.md) | nfs_build.sh - Companion Guide - Builds the user-space NFS server image of the unprivileged NFS attempt (docs/16 DR-16-2): unfs3 (unfsd 0.11.0, built from the upstream repository at the pinned commit |
| [nfs_fallback_state.md](nfs_fallback_state.md) | nfs_fallback_state.sh - Companion Guide - Records the terminal state of the owner-host NFS fallback leg (T134a, ODG-08): `not_needed` only when the T134 record's `state` field is `pass` and it proves the kernel... |
| [nfs_terminal_state.md](nfs_terminal_state.md) | nfs_terminal_state.sh - Companion Guide - Chooses the ONE terminal state of the unprivileged NFS attempt (docs/16 DR-16-2, finding D-10) and writes `nfs-attempt.json` (schema `nfs-attempt/1`, `finding: D-10`).... |
| [qa_analyzers.md](qa_analyzers.md) | vision analyzers - Companion Guide - Deterministic analyzers for recorded frames (doc12 section 9.3): a standard-library frame analyzer (blank frame, error overlay, frozen sequence, motion, colour oracle,... |
| [qa_bank_ratchet.md](qa_bank_ratchet.md) | qa_bank_ratchet.sh - Companion Guide - The ratchet stage of the gates `CM-QA-BANK-NO-PLACEHOLDER` (rule R-2) and `CM-QA-CASE-ASSERTS` (rules R-1 and R-8). It runs the validator over the bank directory and... |
| [qa_manifest_floor.md](qa_manifest_floor.md) | QA manifest and bank-id floor checks - Companion Guide - `manifest_check.py` verifies `challenges/helixqa-banks/MANIFEST.yaml` (doc12 section 6.7): every bank file is listed, every listed file exists, each `case_floor` is at... |
| [qa_probes.md](qa_probes.md) | qa availability probes - Companion Guide - One real, independent availability probe per dependency (doc12 section 10.3): `service.py` (answers AND reports its own build id), `credential.py` (a variable NAME,... |
| [qa_registration.md](qa_registration.md) | qa_registration.sh - Companion Guide - Shell helper for QA scripts that must work with `REGISTRATION_MODE=approval` (register, admin approve, confirm login) and a `qa_curl_code` that prints `000` on a transport failure. |
| [recount_sources.md](recount_sources.md) | recount_sources.sh (recount_sources.py) - Companion Guide - `$EV` in this guide means the evidence root `specs/001-full-project-audit-remediation/evidence` (repository-relative). |
| [regen_speckit_catalogue.md](regen_speckit_catalogue.md) | regen_speckit_catalogue.py - Companion Guide - WP-G1 (docs/12 section 13.2, tasks.md T075, T076, T081; constitution 11.4.77 regeneration mechanism). The Spec Kit governance layer |
| [register_gate.md](register_gate.md) | gate.sh - Companion Guide - `$EV` in this guide means the evidence root `specs/001-full-project-audit-remediation/evidence` (repository-relative). |
| [register_w2_tests.md](register_w2_tests.md) | W2 register tests (T166, T170, T171, T172, T174) - Companion Guide - `$EV` in this guide means the evidence root `specs/001-full-project-audit-remediation/evidence` (repository-relative). |
| [register_wp20_tests.md](register_wp20_tests.md) | WP-20 register source tests - Companion Guide - `$EV` in this guide means the evidence root `specs/001-full-project-audit-remediation/evidence` (repository-relative). |
| [root_inventory.md](root_inventory.md) | root_inventory.py - Companion Guide - `$EV` in this guide means the evidence root `specs/001-full-project-audit-remediation/evidence` (repository-relative). |
| [run_callback.md](run_callback.md) | run_callback.sh - User Guide (T005b) - Companion guide (Helix Constitution section 11.4.18) for `scripts/build/run_callback.sh`. Part of T005b; read `dispatch.md` first. |
| [run_docs.md](run_docs.md) | run_docs.sh - Companion Guide - Documentation checks and renders (link crawl, headers, fingerprints, exports, diagrams). Image: IMG-DOCS. State now: runs while the lock has IMG-DOCS (the real lock pins... |
| [run_go.md](run_go.md) | run_go.sh - Companion Guide - Go build and test (catalog-api, Go submodules, race, bench, fuzz). The command is prefixed `env GOMAXPROCS=3 GOTOOLCHAIN=local CGO_ENABLED=1` (docs/16 6.2). Image:... |
| [run_node.md](run_node.md) | run_node.sh - Companion Guide - Node (catalog-web build and unit tests, TypeScript modules, `catalogizer-api-client`). Image: IMG-NODE. State now: runs while the lock has IMG-NODE, else refused... |
| [run_pinned.md](run_pinned.md) | run_pinned.sh - Companion Guide - The one launcher for a one-shot, rootless, digest-pinned container run (docs/16 sections 7 and 8; constitution 11.4.161, 11.4.173, |
| [run_playwright.md](run_playwright.md) | run_playwright.sh - Companion Guide - Playwright (web E2E, visual regression). Image: IMG-PW. State now: runs while the lock has IMG-PW, else refused `image_not_in_lock`. The whole contract (usage, limits... |
| [run_profile.md](run_profile.md) | run_profile.sh - Companion Guide - The one QA execution wrapper (doc12 section 8.1), run inside IMG-QA from the read-only source mount: resolves a profile from `challenges/helixqa-banks/MANIFEST.yaml`,... |
| [run_qa.md](run_qa.md) | run_qa.sh - Companion Guide - The QA wrapper: image `IMG-QA`. It runs one command (the run-profile wrapper, the floor regeneration, the analyzer self-test, the QA tooling tests) through... |
| [run_scan.md](run_scan.md) | run_scan.sh - Companion Guide - Scanners over the read-only source, results in `/out`. Image: IMG-SHELLCHECK (default) or `--image` a scanner image of the lock. The version recorded in `toolchain.json`... |
| [run_testutil.md](run_testutil.md) | run_testutil.sh - Companion Guide - Test utilities (bash, git, sqlite3, python3, jq, jsonschema, PyYAML, pytest): register tests, verifier and commit-push fixtures, schema validation, governance and... |
| [runner_lib.md](runner_lib.md) | runner_lib.sh and the run_*.sh wrappers - Companion Guide - Every build and test command runs in a pinned rootless container started THROUGH `scripts/containers/run_pinned.sh` (RUNP, reused, never modified). |
| [seed_corpus.md](seed_corpus.md) | seed_corpus.sh - Companion Guide - Deterministic corpus seeder of the real-service stack (docs/16 section 10.4): a fixed tree of real file types (png, jpg, gif, mp3, flac, mkv, mp4, pdf, zip, exe, dmg,... |
| [settings_contract.md](settings_contract.md) | settings contract (storage protocol settings keys) - Companion Guide (PA-01, WP-12) - One vocabulary of settings keys, one mapping from a `StorageRoot` row to the factory settings (`filesystem.SettingsFromRoot`, used by the universal scanner, the stream... |
| [setup_test_env.md](setup_test_env.md) | setup-test-env.sh - Companion Guide - Starts REAL test infrastructure for integration tests through `scripts/test-infra/up.sh` (per-run credentials, random loopback ports, lease, protocol-level readiness)... |
| [sftp_fixture.md](sftp_fixture.md) | sftp_fixture.sh (test infrastructure) - Companion Guide - Runs the integration tests of the SFTP client against a REAL OpenSSH server (image `docker.io/atmoz/sftp`, `internal-sftp` with `ChrootDirectory`) in a rootless |
| [sftp_fixture_gate_test.md](sftp_fixture_gate_test.md) | sftp_fixture_gate_test.sh (test of the SFTP fixture script's own gates) - Companion Guide - Tests, WITHOUT starting any container, the two properties `sftp_fixture.sh` gained in fix-r2 (review finding SFTP-19): an undeclared `SFTP_FIXTURE_SRC` is refused, and... |
| [snapshot_manifest.md](snapshot_manifest.md) | snapshot_manifest.py and check_freeze_snapshot.sh - Companion Guide - `$EV` in this guide means the evidence root `specs/001-full-project-audit-remediation/evidence` (repository-relative). |
| [sync_trackers.md](sync_trackers.md) | sync_trackers.sh - one-way sync of the findings register to the external trackers the owner configured; records every attempt in `reg_tracker_sync_log` and never claims a push that did not happen |
| [test-in-container.md](test-in-container.md) | test-in-container.sh - Companion Guide - The lane dispatcher: every build and test lane is started through this one command. It looks the `(app, lane)` up in `scripts/containers/lanes.tsv` |
| [test_containerfiles.md](test_containerfiles.md) | test_containerfiles.sh - Companion Guide - Static checks over every image directory of `build/containers/` (docs/16 DR-16-1). One directory per image, each with `Containerfile`, `README.md`, `digests.lock`. |
| [test_infra_down.md](test_infra_down.md) | down.sh (test infrastructure) - Companion Guide - Tears down ONE project by its label: containers, network and volumes carrying `catalogizer.test_project=<project>` (and `project=catalogizer`), nothing else. An... |
| [test_infra_index.md](test_infra_index.md) | Test infrastructure scripts (index) - Companion Guide - One page that links every companion guide of the real-service test infrastructure (WF17 TI-I4: none of them was reachable from another document). The stack itself is... |
| [test_infra_lib.md](test_infra_lib.md) | test-infra library and client scripts - Companion Guide - `TI_ROOT` (repository root; `TI_ROOT` env overrides it for a mutation copy), `TI_STATE_DIR`, `TI_COMPOSE_FILE`, `TI_LOCK`, `ti_valid_id`, `ti_project`, `ti_state`,... |
| [test_infra_probe.md](test_infra_probe.md) | probe.sh (test infrastructure) - Companion Guide - Protocol-level probes of a running project: PostgreSQL `SELECT 1` answers 1; Redis authenticated `PING` answers `PONG` and the unauthenticated one is refused; FTP login,... |
| [test_infra_roundtrip.md](test_infra_roundtrip.md) | roundtrip.sh (test infrastructure) - Companion Guide - One real round trip against one protocol of a running project, from IMG-INFRA-CLIENT: PostgreSQL (create, insert, read, update, duplicate key refused, delete, drop,... |
| [test_infra_run_client.md](test_infra_run_client.md) | run_client.sh (test infrastructure) - Companion Guide - Runs ONE protocol-client command in the interpreter-class image IMG-INFRA-CLIENT, attached to a project's compose network, through `scripts/containers/run_pinned.sh`... |
| [test_infra_sweep_leaks.md](test_infra_sweep_leaks.md) | sweep_leaks.sh (test infrastructure) - Companion Guide - Removes what earlier runs leaked and no lifecycle script finds again: empty unlabelled podman-compose pods `pod_catalogizer-test-<id>` and the... |
| [test_infra_tests.md](test_infra_tests.md) | tests/infra (test infrastructure tests) - Companion Guide - Run a test with `bash tests/infra/test_<name>.sh` (mutations run by default; `*_NO_MUTATIONS=1` skips them). Tests must run one at a time on this host: every... |
| [test_infra_up.md](test_infra_up.md) | up.sh (test infrastructure) - Companion Guide - Starts ONE test-infrastructure project `catalogizer-test-<build_id>`: per-project single-owner lease (11.4.119, registered long operation 11.4.232), per-run credentials... |
| [test_race_detector.md](test_race_detector.md) | test_race_detector.sh - Companion Guide - The Go race detector must run inside the pinned IMG-GO container through `run_go.sh`, and a seeded data race must make the run FAIL while the fixed copy |
| [test_runners_hardening.md](test_runners_hardening.md) | test_runners_hardening.sh - RED-first oracle for the defects found by the independent review of the resource envelope and the runner wrappers (one block per finding) |
| [validate_banks.md](validate_banks.md) | validate_banks.py - Companion Guide - The bank validator of doc12 section 6.6: rules R-1..R-8 over the HelixQA bank files, run before any bank executes. R-1 every step has a machine assertion; R-2 no... |
| [verify_live_evidence.md](verify_live_evidence.md) | verify_live_evidence.sh - LIVE-EVIDENCE verifier (`scripts/test-infra/verify_live_evidence.sh`, constitution 11.4.289(D)): decides from machine fields whether a record claiming `class: LIVE` really proves a real target. |

## Scripts without a guide yet (owed, constitution 11.4.18; WF7 F7-5)

Added to the repository without a guide here: `scripts/containers/{disk_headroom,probe_host,resolve_pin,smoke_images,smoke_probe}.sh`,
`scripts/ledger/project_gate_ledger_ratchet.sh`, `scripts/register/{apply_ext,gate,run_mutations}.sh`, `scripts/register/mutate_ddl.py`,
`scripts/repo/{check_no_ci,check_revision_headers,commit_recursive,fixture_roots,integrate_ff_only,integrate_merge,push_recursive,record_deferral,scope_check}.sh`.
Their file-header comments are the only description today. UNCONFIRMED: the list is the WF7 reviewer's count of 19 of 33 scripts, not re-derived here.
