# First baseline: Catalogizer v2.4.2 live run (constitution 11.4.298 / 11.4.293)

Status: PROVISIONAL. This is evidence for the FIRST BASELINE of the product `web` only. The release gate for `web` ends in
`GATE-STATE: BLOCKED-NO-BASELINE` (exit 21) by design: a first build has no baseline, and the operator has to record how a
first baseline is accepted (11.4.298(D)). Nothing here is a RELEASABLE verdict.

## Target (read from the running stack, `images_identity.txt`, `containers_identity.txt`, `host_identity.txt`, `health_after_runs.json`)
- Source: tag `v2.4.2` = commit `fc8dcf27e85ccd22dda069b3354ce4a73a6f659e`, clean `git archive` export + submodules at their recorded gitlinks (`docs/qa/stack3/build.sh`).
- Images built for this run: `localhost/catqab-api:main-fc8dcf27` (id `bd071396...`), `localhost/catqab-web:main-fc8dcf27` (id `2943f5b3...`).
- `/health` build_number = full v2.4.2 sha (`health_after_runs.json`), real PostgreSQL 15 + Redis 7 containers in one rootless pod `catqab` (pasta), real binary, no stubs/mocks.
- Host `anton`, physical (`systemd-detect-virt` = none). Stack scripts: `docs/qa/stack3/{build,up,down}.sh` with `STACK=catqab` and ports 38090/33090/35490/36490.
- DEVIATIONS, stated not hidden: (1) the stack scripts ran with backend=podman, the Containers-submodule runtime was not available (script output); the build ran on the host with `podman build --network host` (documented 11.4.173 deviation in docs/qa/stack3/README.md); (2) the stack used `APP_ENV=development`, `CSRF_INSECURE_DEV=true`, CORS `*` (QA-LAN settings of stack 3); (3) the live scripts and the release tooling (`scripts/release/*`) come from `main` at 846a84d66, the product from the v2.4.2 tag.

## Suite and results
Suite = `catalog-web/e2e/live` at the v2.4.2 tag (8 spec files, 33 tests per project), Playwright 1.61.0, real browser, real login, real API/nginx path,
seeded through the real API from the media corpus mounted read-only at `/media` (`LIVE_CORPUS_ROOT_PATH=/media`).
Projects run: `chromium-desktop` and `firefox-desktop`, 3 runs each, exit=0 in all 6. Not run: `chromium-390`, `firefox-390` (time).
Per-test verdicts (`runs/<project>/run<N>/verdicts.tsv`, derived mechanically by `verdicts.py` from Playwright `report.json`): 33 PASS per project per run, the three verdict files of each project are byte-identical (N=3, 11.4.50).
`baseline_verdicts_web_v2.4.2.tsv` = the 66 combined rows (sorted). Test ids are the Playwright path with every non `[A-Za-z0-9._:/>-]` run replaced by `_` (image_no_worse.sh rejects space/comma/glob).
The suite's own evidence records are in `runs/<project>/evidence/*.json` (class LIVE, produced by `make_evidence.mjs`); the independent verifier `scripts/test-infra/verify_live_evidence.sh --require-live` was NOT run here (UNCONFIRMED that it accepts them).
Screenshots/traces (`artifacts/`, ~5 MB per run) are not committed.

## Records and gate states (`gate_*.out`, `gate_*.rec`)
| Product | GATE-STATE | exit | Reason |
|---|---|---|---|
| web (catalog-api + catalog-web) | BLOCKED-NO-BASELINE | 21 | comparison_web.rec = INCOMPARABLE-BASELINE-MISSING (no previous run; `image_no_worse.out`). The hardware and live-run records satisfy the gate (the gate reaches the comparison step). |
| android, androidtv, desktop, wizard | BLOCKED-NO-HARDWARE | 20 | no connected hardware, no live run in this task (Android/TV/desktop/wizard not driven). |
| synology | BLOCKED-NO-HARDWARE | 20 | per CAT-SYN-LIVE-1: drives powered off, SKIP `hardware_not_present`; not a pass. |

`control_selfbaseline_comparison.rec`: control only, shows image_no_worse.sh refuses a comparison of an image with itself (INCOMPARABLE), so no self-baseline can mint a RELEASABLE.
UNCONFIRMED: whether `web` is a product the operator wants gated as "physical hardware" (the gate takes this host, kind=physical, as the target; the product list and deployable-build kinds are still the open operator declaration in G2 of docs/release/V2_5_0_GATE_CHECKLIST.md).

## What the operator has to supply
1. A recorded decision of how the first baseline is accepted (11.4.298(D)) and the product list.
2. For the next candidate (v2.4.3 / v2.5.0): flash/deploy it on the same target, run the same suite and command, then
   `image_no_worse.sh --baseline-verdicts baseline_verdicts_web_v2.4.2.tsv --baseline-meta baseline_meta_web_v2.4.2.rec --new-verdicts ... --new-meta ...`
   and `release_gate.sh`. Note per 11.4.293(B): if v2.4.3 is deployed first, it becomes the baseline instead.

## Aborted first attempt (disclosed)
A first live run used the default corpus root `/media/corpus` (the suite's default) instead of `/media` where the stack mounts the corpus: the seeding step saw 0 items, 11 tests FAILED and 7 did not run (harness configuration error, not a product finding; reproduced by reading the spec: `ROOT_PATH` default). The run was stopped after run 1, the stack was purged and recreated, and the real runs above used `LIVE_CORPUS_ROOT_PATH=/media`. Those aborted files are not part of this evidence.

## Disk guard
`watchdog_summary.txt`: watchdog checked `/` every 15 s during build and runs (kill threshold 50 GB, no kill happened).
