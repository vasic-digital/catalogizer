# Operator decision log, 2026-11-02

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-11-02 |
| Status | active |
| Source | Operator decisions of 2026-11-02, relayed by the conductor (not verbatim) |
| Tracking | Each decision below carries its implementation status; "OWED" means not yet done and must be tracked per §11.4.197 |

No credential value appears in this document (§11.4.10).

## Decisions

| # | Decision | Implementation status in this change |
|---|---|---|
| D1 | `atm_id` is renamed to `cat_id` (schema migration plus a compatibility view, ids preserved) wherever it lives in Catalogizer's own code | PARTIAL (nothing renamed), see "atm_id scope". OWED: CAT-NNN proposal "register atm_id to cat_id rename" |
| D2 | Where SQLite/SQLCipher is built, `-DHAVE_USLEEP=1` is added | PARTIAL / PROVISIONAL until verified end to end. Mechanism: `scripts/lib/cgo_flags.sh` exports `CGO_CPPFLAGS` (append-safe, idempotent; CPPFLAGS not CFLAGS so go's default `-O2 -g` is kept); every covered script sources it with a fail-loud branch (`|| { echo ...; exit 1; }`, symlinks resolved with `readlink -f`). Dockerfiles (`catalog-api/Dockerfile`, `Dockerfile.dev`, `docker/Dockerfile.builder`), `docker-compose.dev.yml`, `.air.toml` and the container runner prefixes carry the define inline. Class guard `scripts/gates/cm_cgo_usleep_flag.sh` sweeps tracked `*.sh`, `*.bash`, `Dockerfile*`, `Containerfile*`, `Makefile`, `*.mk`, `docker-compose*.yml`, `.air.toml` and extension-less shebang files outside `submodules/` and `constitution/`; control-needled; it reads executable lines only (line continuations joined, comments and echo/printf/log_ lines dropped) and requires every file with a go build/test/vet/run/install/generate line (also `/path/go`, `$GO`, after `&&`/`;`) to be classified in `scripts/lib/cgo_flag_scope.tsv`, and every `covered` file to have an executable sourcing line with a failure branch or an assignment of the define. WHAT THE TEST RUNS: `scripts/gates/tests/test_cm_cgo_usleep_flag.sh` (31 checks: lib behaviour, golden-good/bad fixture repos, comment/echo/no-failure-branch mutations, sweep-widening cases, the real repo) plus 6 gate mutations (all killed); `go test ./database/` was run with `CGO_CPPFLAGS=-DHAVE_USLEEP=1` (green). NOT VERIFIED: the Dockerfiles, compose file, `.air.toml` and the release/shell build scripts were never executed. Remaining sweep limits: go invoked through variables other than `$GO` (`tools/evidence/wrap-go.sh` is listed by hand); go inside here-doc or generated script text; `#` inside a quoted string is read as a comment; satisfaction is per file, not per go line, and an assigned define is not proven exported to the go line; the busy-handler test picks its branch from the environment |
| D3 | `deployment/amber-up.sh` uses rootless podman, not docker | DONE; the R6 allowlist row is removed |
| D4 | `thinker.local` is obsolete. Builds run as local containerised builds. This is a written exception to §11.4.173 (which requires remote distribution of builds) | RECORDED as a standing amendment by operator decision (no expiry; revisit if a remote build host returns); script and CLAUDE.md follow-up OWED (`scripts/build_in_container.sh` still targets a remote host, `scripts/container-local.sh` is the local path) |
| D5 | A BLIND gate result fails closed | RECORDED; per-gate audit OWED |
| D6 | llmctl planner refusal uses exit code 75 (matches §11.4.277(A)/(E) pressure refusal) | RECORDED |
| D7 | `doc_processor` canonical branch is `main` | RECORDED (consistent with §11.4.288) |
| D8 | The canon repository is not renamed | RECORDED |
| D9 | The commit-push-all trust manifest is approved as drafted | RECORDED; no manifest content changed here |
| D10 | The waiver expiring 2026-11-20 is resolved before expiry by implementing the gate consumer-data design | OWED, deadline 2026-11-20 |
| D11 | Release train defaults | Operator decisions: trigger `per-ready-candidate`; `pre-release` and `dev` classes are provisional-eligible. Author default, awaiting operator confirmation: stall bound N=3 (§11.4.230(E.4)/(F)). Recorded as consumer DATA |
| D12 | `llms_verifier` becomes a real own-org flat submodule | Operator decision: as stated. Author note, awaiting operator confirmation: the add itself is OWED and follows `docs/decisions/llms_verifier_reconciliation.md` |
| D13 | Key rotation stays deferred, severity HIGH | Operator decision: deferred. Author note, awaiting operator confirmation: severity HIGH and the pointer to `SECURITY_KEY_ROTATION_REQUIRED.md`; remains an open item |
| D14 | Synology write tests run only in `work20/Testing` | RECORDED; no test is allowed to write elsewhere on the Synology |

## D4 exception text (§11.4.173)

§11.4.173 requires builds to be distributed to a remote build host. By operator decision of 2026-11-02 `thinker.local` is obsolete, so the build container runs on the local host through `scripts/container-local.sh` (rootless podman, §11.4.161). The §11.4.173 container requirement is unchanged; only the remote-distribution leg is waived. Not a general waiver: any new remote build host needs a new decision.

## atm_id scope (D1)

Measured in the scratch clone: `atm_id` appears in 35 files under `scripts/register/` (DDL `register_ext.sql`, importers, gates, mutation harnesses and tests), in `scripts/governance/tests/`, in `.specify/memory/` and in two `docs/scripts/` pages. Two facts block a safe rename in this change:

1. The register DDL layers on the constitution workable-items engine. The engine's own `items` table has an `atm_id` column and the register joins on it (`items.atm_id = reg_*.atm_id`). The engine lives in the `constitution` submodule, which this change must not edit. A rename of only the `reg_*` columns breaks every such join unless every query is rewritten at once.
2. The register scripts are covered by mutation harnesses whose mutants are text patterns over these exact strings; renaming changes the mutant corpus and its recorded scores, which needs its own review round.

Status: OWED as a dedicated change: (a) `ALTER TABLE ... RENAME COLUMN atm_id TO cat_id` on the `reg_*` tables (SQLite 3.25+ rewrites index, trigger, view and foreign-key references automatically, so those are not the blockers; the blockers are SQL strings in py/sh code and the mutant corpus), (b) compatibility views exposing `atm_id` for readers, (c) rewrite of the 35 files with the mutant corpus regenerated, (d) an upstream request for the engine to rename `items.atm_id` (§11.4.54 extension note). Ids (`CAT-NNN`) are not affected and are preserved.

## Tracked-item proposals for OWED work (ids are proposals, to be minted by the register)

| Proposed id | Work |
|---|---|
| CAT-NNN (1) | Register `atm_id` to `cat_id` rename with compatibility views, mutant corpus regenerated (D1) |
| CAT-NNN (2) | Retire the remote-host path in `scripts/build_in_container.sh`; update CLAUDE.md for the D4 amendment (D4) |
| CAT-NNN (3) | BLIND-gate fail-closed audit, per gate (D5) |
| CAT-NNN (4) | Waiver expiring 2026-11-20: implement the gate consumer-data design before expiry (D10) |
| CAT-NNN (5) | Add `llms_verifier` as a flat own-org submodule (D12) |
| CAT-NNN (6) | Execute-verify the D2 Dockerfile and shell-script build paths in containers (D2) |
| CAT-NNN (7) | Verify amber.local: podman installed, cgroup delegation, image store (D3) |
| CAT-NNN (8) | Make the busy-handler test pick its branch from a compiled-in probe or timing (D2) |
