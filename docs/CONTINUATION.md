# CONTINUATION — Catalogizer

> Live work-state document per Constitution **§12.10 (CONTINUATION)** and **§11.4.131**.
> A fresh session that receives only the prompt `continue` MUST read this file first, then
> `docs/FOLLOW_UPS_NEXT_PHASE.md`, then resume the §11.4.126 loop with the steps in "RESUME ORDER".

**Revision:** 14
**Last modified:** 2026-11-06 (end-of-day wrap-up)
**Exports:** `CONTINUATION.html` / `CONTINUATION.pdf` are STALE (Rev 9). Regenerate from this file (owed, §11.4.65); this `.md` is authoritative.

## 1. State at stop

* `main` = `5713effb` + this doc commit, on all six upstreams (origin, github-milos, gitlab-milos, gitlab-vasic, gitflic-vasic, gitverse-vasic). Verify parity first (section 5 step 1).
* Release target: first tag `v2.4.0` (pre-release, SemVer), only after live validation. Everything landed is **PROVISIONAL-DONE** (§11.4.290) until its live rerun passes N>=3 with machine evidence.
* SonarQube on-demand tooling is LANDED (df7e2dd1, PROVISIONAL); the fresh scan on digest-pinned images is still owed (Docker Hub rate limit; CAT-QA-SONAR-1).
* Landed 2026-11-06: D7-R4 contrast fixes + island detector (round 1 GO-with-follow-ups, D7-R4-FU-1..4); live security evidence (`docs/qa/security-live-20261106/`: Prometheus auth, pprof 401/403/60 s cap, SSRF on image proxy / storage roots / sync endpoints VERIFIED 3/3); TRUSTED_PROXIES hardening (explicit `/32` for the proxy with fixed compose subnet + `ip_range`, `auto` refused in release unless `TRUSTED_PROXIES_ALLOW_AUTO=true`, fail-closed on bad value, nginx XFF on every proxied location; round 2 GO-with-follow-ups; PROVISIONAL, compose files only rendered + throwaway project, never brought up as real stacks); workable-items register Rev 2; disk prune (18 dangling images, ~18 GB reclaimed).
* The front-door "shared login bucket" is explained as a same-host artefact (pasta alias 169.254.1.2); remote-LAN isolation UNCONFIRMED (needs a second LAN host, = QA-MON-FU-4). `CAT-FRONTDOOR-RATELIMIT-1` stays open.
* **Tauri desktop/installer native QA agent**: see section 2 for its exact state at stop.
* **Synology drives are OFF** (operator 2026-11-05 19:09 CET): Synology SMB/FTP/NFS live tests = SKIP `hardware_not_present`.
* Constitution pin 92a9ecf8 (v73.2). Periodic fetch still owed.
* Binding rules: memory `feedback_release_completion_mandate`, `feedback_review_budget_policy` (max 5 rounds, minors -> follow-ups; security-adjacent minors still block), `feedback_all_work_on_main`, `project_decisions_2026_11_03`, `project_decisions_2026_11_05`, `feedback_pkill_self_match`. Never force-push; never print credentials (Yandex WebDAV creds only in scratchpad `yandex_webdav.env`, stack in `catqa3/qa.env`/`mon.env` 0600). No sudo, rootless podman only; do not touch stacks catqa1/catqa2 or shared aardvark-dns.

## 2. NOT ON MAIN

| Branch | Where | Item | State |
|---|---|---|---|
| Tauri native QA (branch `feat/tauri-native-qa`, local, never pushed) | scratch clone under `/home/milosvasic/.cache/claude-tmp/` (path in agent report below) | CAT-TAURI-NATIVE-1, CAT-TAURI-INSTALLER-1 | Clone `/home/milosvasic/.cache/claude-tmp/tauriqa.hw0Kco/repo`, HEAD `389f8abe` (base df7e2dd1; contains run-qa.sh QA_SCREEN default, `test_wizard_capabilities.sh` guard, wizard `capabilities/default.json` dialog:allow-open/save, tauri-boot README note). NEEDS Opus review (max 5 rounds) then landing. DESKTOP (CAT-TAURI-NATIVE-1): VERIFIED 3/3 live on binary sha 9832e3eb (confirm dialog on URL change, Cancel/Esc keep URL, confirm applies, login with real data, file:// and userinfo URLs rejected, mutant always-approve killed; caveats: xdotool-driven, slideshow not screencast, binary since rebuilt to fa794e1c so re-verify). WIZARD (CAT-TAURI-INSTALLER-1): REAL DEFECT reproduced live 3/3 (Save Configuration fails 'Command plugin:dialog|save not allowed by ACL'); fix committed + static guard; GREEN live runs NOT done (rebuilt wizard sha 773bf711). Evidence + scripts (not in repo) in `/home/milosvasic/.cache/claude-tmp/evidence-archive/tauri-native-20261106/` (scripts/ subdir; wizard evidence not yet secret-scanned). Resume: scratchpad `runwz.sh green 1..3`, then secret-scan, copy scripts+evidence README/MANIFEST to docs/qa/tauri-native-20261106/, review, land. QA displays are DOWN: `QA_BIND=192.168.1.115 scripts/tauri/run-qa.sh desktop|wizard <out>` (noVNC :6080 / :6081, server URL in app `http://host.containers.internal:38082`). |
| `release/v2.4.0-prep` HEAD `b334ebd0` (local) | `/home/milosvasic/.cache/claude-tmp/relprep.EtmMzF/repo` | CAT-REL-V240-1 | HELD: versions.json 2.4.0, openapi version, CHANGELOG `[2.4.0] - pre-release`; re-check VERIFIED/PROVISIONAL rows against the live results (add D7-R4, TRUSTED_PROXIES, security-live) before landing; no git tags exist yet. |

## 3. Landed (all PROVISIONAL-DONE until live rerun)

Since Rev 11 (`109acde2`): D7 remaining contrast `c78c654a`; HelixQA stack-3 API smoke wrapper `fc1192fe` (UI vision leg not implemented, honest SKIP); Tauri SSRF hardening `4ccead68` (TAURI-FU-5); S3 storage protocol `83a21e38` (unit-class); S3 live evidence on real Garage `0a4fb76a` (N=3 live); Tauri backend-enforced confirmation on server URL change `865287f4` (needs native boot); D7 class-wide dark-gray contrast `6ffc4593`. Earlier: UI-D1/D2/D3/D5/D6/D7/D8, Android QA boot (`docs/qa/android-boot-20261105`), Tauri container build + QA boot tooling (`109acde2`), SECQ-1/3/4/5/6, WD-F1, NEW-1, stack-3 verify pack. Follow-ups are in `docs/FOLLOW_UPS_NEXT_PHASE.md` (the authoritative pending list).

## 4. Stack 3 (containers do NOT survive reboot)

Running at stop (if the host was not rebooted): pod catqa3 (images `localhost/catqa3-api|web:main-17ec3c1f`, do NOT contain the D7-R4 fixes or the TRUSTED_PROXIES hardening) and pod catqa3mon (Prometheus with basic auth, user `qa_prom`, password `PROM_AUTH_PASSWORD` in `mon.env`; LAN-bound Grafana/pgAdmin/Prometheus). After a reboot: `docs/qa/stack3/up.sh <tag>` then `GRAFANA_BIND=PGADMIN_BIND=PROM_BIND=192.168.1.115 docs/qa/stack3/monitoring/up.sh` (its exit 6 / `000` loopback line is only the loopback check on LAN-bound services; verify with LAN URLs).
URL map (host 192.168.1.115): API :38082 (/health), web :33020, website :33021, nginx front door :33080 / :33081, Grafana :33030, pgAdmin :33050, Prometheus :39090 (auth required). PostgreSQL 127.0.0.1:35434 and Redis 127.0.0.1:36381 stay loopback. Logins: qa_admin (password only in `/home/milosvasic/.cache/claude-tmp/catqa3/qa.env`). Rebuild web+API from current `main` before the next live rerun (D7-R4 fixes, TRUSTED_PROXIES). Inactive throwaway users `qa_thrw_*` exist (CAT-QA-THRW-CLEAN-1). `verify.py` has 3 stale LAN-NOT-reachable assertions (CAT-VERIFY-LANBIND-1).

## 5. RESUME ORDER (what "continue" means)

1. `cd /home/milosvasic/Projects/catalogizer && git fetch --all --prune --recurse-submodules=no`; confirm six-upstream parity of `main`; `git status --short --ignore-submodules=all` clean. Check disk and load (<~12 before dispatching; hold dispatch while >20).
2. Rebuild the web (and API) images from `main`, then `docs/qa/stack3/up.sh <new-tag>` and `docs/qa/stack3/monitoring/up.sh` with LAN binds. Verify the URL map with curl on the LAN URLs.
3. Live re-verify (dispatch a Sonnet verifier; evidence to `docs/qa/ui-live-rerun-20261105/`, screenshots to `/home/milosvasic/.cache/claude-tmp/evidence-archive/`): `node docs/qa/ui-d5d6-20261105/axe_live.cjs` (env NODE_MODULES_DIR=/home/milosvasic/.cache/claude-tmp/ui-d1.nNlTmm/pw/node_modules, CHROMIUM_PATH), `docs/qa/ui-d7d8-20261105/d7_axe.cjs` + `d8_repro.cjs`, UI-D1 repro `/home/milosvasic/.cache/claude-tmp/ui-d1.nNlTmm/pw/repro.js`, D2/D3 pages. N>=3 identical runs -> promote PROVISIONAL to VERIFIED; contrary evidence reopens. Restart API with HELIX_PPROF_ENABLED=true for pprof 401/403 + 60 s cap; cover-art SSRF-F2..F5; TRUSTED_PROXIES from a second LAN host.
4. Open work items (Sonnet authors in scratch clones via `git clone --no-hardlinks`, then Opus reviewers, max 5 rounds): (a) SonarQube (section 2; unblock image pull first); (b) Vision/HelixQA UI leg: stop `llmctl-onnx@decide-nli` ONLY via `llmctl stop`/`start` (never `enable`), CPU-capped, do not harm other sessions; (c) Tauri native boot of the confirm-on-URL-change and SSRF changes (`TAURI_BUILD_JOBS=2 scripts/tauri/build.sh <out>`, `QA_BIND=192.168.1.115 scripts/tauri/run-qa.sh desktop`); (d) Android real login via secret-safe path; (e) S3 UI/docs/web SUPPORTED_PROTOCOLS follow-ups (S3-FU); (f) throwaway pre-release tag test to prove GitHub/GitLab release permission (then delete the test tag everywhere).
5. Landing procedure: `git fetch -q --recurse-submodules=no <clone> <branch>`; `git diff --raw HEAD...FETCH_HEAD -- submodules constitution | wc -l` must be 0; `git merge --no-ff --no-commit FETCH_HEAD`; append follow-ups to docs/FOLLOW_UPS_NEXT_PHASE.md; commit with `Roadmap-Item:` + `Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>`; push SERIALLY to six upstreams with `sleep 4` (parallel push fails "failed to lock"); verify `git ls-remote <r> refs/heads/main` parity; delete any wip branch on every upstream after landing.
6. Remaining release scope (memory `feedback_release_completion_mandate`): WL13 player follow-ups + subtitles panel; responsive/a11y live matrix across devices/browsers; desktop+installer wizard; Android/Android TV; HelixQA with Vision; performance/security/chaos; N>=3 determinism ledger; retire stacks catqa1/catqa2 (docs note only); LV3-1..5; D5 bursty-audio fixture; Synology idle-deadline/cut-recovery live and Synology SMB/FTP/NFS live tests (SKIP `hardware_not_present` while the drives are OFF); PostgreSQL last-admin race; key-rotation waiver = operator-accepted open risk, no expiry, never invent a date; `versions.json` 2.3.0 -> 2.4.0 + changelog (VERIFIED / PROVISIONAL / REOPENED / OVERDUE separately); test GITHUB_TOKEN / GitLab release permission with a real create at release time; tag `v2.4.0` on all upstreams only after live validation.
7. Queue: every open item is a registered workable item in `docs/WORKABLE_ITEMS_REGISTER.md` (Part A = existing follow-up ids with pointers into `docs/FOLLOW_UPS_NEXT_PHASE.md`; Part B = items registered at Rev 13). Do not keep a parallel list here.

| Group | Item ids (see register) |
|---|---|
| Verification (PROVISIONAL until live) | CAT-UI-AXE-LIVE-1, CAT-STACK3-REBUILD-1, CAT-PROM-LAN-1, CAT-PPROF-LIVE-1, CAT-DETERMINISM-LEDGER-1, CAT-TAURI-NATIVE-1 |
| Blocked | CAT-QA-SONAR-1 (Docker Hub rate limit), CAT-SYN-LIVE-1 + ST-NFS (drives OFF, SKIP hardware_not_present), CAT-HELIXQA-VISION-1 (host capacity) |
| Features / coverage | CAT-TAURI-INSTALLER-1, CAT-ANDROID-LOGIN-1, CAT-S3-LIVE-2, CAT-RESPONSIVE-A11Y-MATRIX-1, CAT-PERF-CHAOS-1, CAT-WL13-PLAYER-1, CAT-D5-BURSTY-AUDIO-1, CAT-PG-LASTADMIN-1, CAT-QA-MON-FU-2, CAT-STACK-RETIRE-1 |
| Release | CAT-REL-V240-1, CAT-REL-PERM-1, CAT-KEYROT-WAIVER-1 (operator-accepted open risk, no expiry) |
| Governance | CAT-WI-ENGINE-1, CAT-CONST-FETCH-1, CAT-LLMS-VERIFIER-1, CAT-WAIVER-GATE-DESIGN-1, CAT-LLMCTL-PINS-1, CAT-TEST-PREEXIST-1, CAT-DOCS-EXPORT-1 |
| Existing follow-up ids | SECQ-2/4b/7/8, SUB-H1..H7, ST-R4-1..4, LW3-1, WL13-R*, MON-G4/G5/PROV, QA-MON-FU-3/4, LE3-B/D, D8-F1, CAT-D9-SUSPEND, D9-F1, D4-3, UI-D4, UI-FAV-1, MP-FLAKE-1, WEB-DEPS-1, AND-FU-1..7, S3-FU-1..3, TAURI-FU-1..7, D11-R2-2..5, D7-R3-FU1/M1/M3, LV3-1..5 |

## 6. Disk

Operator decision 2026-11-05: prune APPROVED for Go build/module caches, Android SDK system images, and NAMED unused podman images/volumes. NEVER a blanket prune (`podman system prune` is forbidden); other projects' dirs and anything not named stay untouched. Re-run the idempotent guarded scratchpad `run_prune.sh` for the earlier approved set. Scratch under `/home/milosvasic/.cache/claude-tmp/` (survives reboot): `sonarqa.FG8Tji` (KEEP, unlanded SonarQube work), `s3proto.y6Du6d` and `tauri.yV0oud` (landed; deletable with guarded `${D:?}` paths after confirming main has them), `andqa.Lo6bOU` (Android, ~7 GB), `d7b.PybOvV` (empty), reviewer roots `rvroot`.

## 7. Open operator decisions

Decided 2026-11-05: SonarQube = on-demand QA service with minimal tokens; prune scope (section 6); Prometheus LAN-only; live S3 on Garage; throwaway pre-release tag test; Tauri confirm on URL change (done). Still open: tag form (default `v2.4.0`); horizontal-authz / second-LAN-host measurement; when the Synology drives are powered back on; Docker Hub access method for the SonarQube images (login, mirror, or wait).
