# wp11 evidence (T104, T105, T106, T148, T149)

Identity headers (`# identity`, `# head`, `# sha256 <file>`, `# run_at`) are on every captured run. HEAD moved while this work was in progress (7f5e9f4b, later b3728d76); all files under test are uncommitted.

| File | Task | Content |
|---|---|---|
| `t104-red.txt`, `t104-green-run1..3.txt`, `check-pins-mutation.txt` | T104, T105 | RED (script absent), GREEN x3 (89 checks) through `RUNP IMG-KCOV`, 23 mutants all CAUGHT |
| `pins-baseline.txt`, `pins-baseline-recursive.txt` | T105 | unpinned-reference rows: main repository (91), and every submodule checkout (3 more files: includes third-party, own-org filter not applied) |
| `t106-red.txt`, `t106-green-run1..3.txt`, `containerfiles-mutation.txt` | T106 | RED (tree incomplete), GREEN x3 (28 checks), 10 mutants all CAUGHT |
| `t106-build-*.txt`, `t106-smoke-*.txt` | T106 | local builds on anton (sigverify, gotools, infra-client, mut, docs) and one smoke each |
| `gotools.json`, `rust-coverage-tools.json`, `mc-client.json`, `nfs-client.json` | T106 | 11.4.270 existence verdicts |
| `goimports-baseline.txt`, `out-goimports/goimports-violations.txt` | T106 | goimports -l over 5243 .go files: 324 violations, control needle seen |
| `t148-red.txt`, `t148-green-run1..3.txt`, `t148-proposed.patch` | T148 | RED on the tracked files; patch applied to COPIES (`.audit/scratch/t148`), GREEN x3 |
| `t149-proposed.patch` | T149 | remaining six compose image lines by lock digest (applied to the t148 copy: 0 violations) |
| `../wp15/signatures.json` | T149 | IMG-SIGVERIFY digest first, then per-scanner cosign verdicts |
| `disk/` | all | disk_headroom records of every pull and build |

## Round 2 (T107-T114, T116, T148/T149 applied) - 2026-10-06

Builds ran rootless on this host (anton, owner decision) with `podman build` directly (disk gate `disk_headroom.sh` before each, `--pull=never`, images pulled by digest only); `scripts/build/dispatch.sh` runs argv inside an `IMG-*` image and cannot run `podman build`, so it was not used (not a RED/GREEN input). Root-context builds used a scratch ignorefile (the repository has no root `.dockerignore`, owed). Local-only aliases created: `docker.io/library/golang:1.25` (alias of `1.25-bookworm`), `docker.io/library/debian:trixie-slim` (pulled by digest a29215f6...).

| Files | Task | Content |
|---|---|---|
| `t107-red.txt`, `t107-v05-workaround-check.txt` | T107 | D-03 RED (COPY of `WebSocket-Client-TS/package*.json` fails), no out-of-tree workaround |
| `t108-d01-red.txt`, `t108-d04-red.txt` | T108 | D-01 RED (`COPY submodules/assets/`), D-04 RED (`COPY docker/go1.26.1...tar.gz`) |
| `t109-green-run1..3.txt`, `t109-smoke.txt` | T109 | D-04 fix (golang digest stage), 3 builds rc 0, identical image id (cache-warm) |
| `t110-red.txt`, `t110-green-check-run1..3.txt`, `t110-green-manual.txt`, `t110-green-build-run1..3.txt`, `t110-smoke.txt` | T110 | check_pins RED (3 violations) then 0; manual rows for the 3 items check_pins has no rule for; 3 builds rc 0 |
| `t111-iter1-run*-runtime-base-not-local.txt`, `t111-green-run1..3.txt` | T111 | D-01 fix; iteration 1 failed only because `debian:trixie-slim` was not in local storage (pulled by digest), 3 builds rc 0 after |
| `t112-iter1-fail-modules-built-before-source.txt`, `t112-green-run1..3.txt` | T112 | D-03 fix; iteration 1 passed all COPY lines and failed at `npm run build` (module builds ran before their sources were copied: second defect, fixed in the same file), 3 builds rc 0 |
| `t113-d02-red.txt`, `t113-green-run1..3.txt` | T113 | D-02 RED then fix, 3 builds rc 0 |
| `t114-red.txt`, `t114-green-run1..3.txt`, `t114-minio-blocked.txt` | T114 | 13 violations then 3 (MinIO, BLOCKED: image unavailable) |
| `t148-green`/`t149-green-run1..3.txt`, `t149-consistency.txt` | T148/T149 | patches applied to the tracked files; 0 violations in the two files; compose digests all map to lock entries |
| `mutations.txt`, `t116-mutate.sh` | T116 | D-01..D-04 revert mutants all fail at the COPY step; pins mutants caught by check_pins |
