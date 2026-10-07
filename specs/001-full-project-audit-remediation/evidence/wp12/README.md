# wp12 evidence (T127-T135, WP-13 test infrastructure; tasks.md names the directory wp13, this run was told wp12; the NFS records are in wp10)

Identity headers (`# identity`, `# head`, `# run_at`, `# sha256 <file>`) are on every captured run. HEAD moved while this work was in progress (960c553a, later f981e385 and others); all files under test are uncommitted.
Tests were run ONE AT A TIME (concurrent run_pinned containers trip the anti-mess sweep of each other's TIC calls).

| Files | Task | Content |
|---|---|---|
| `nfs-codepath.md` | T127 | the application NFS path mounts through the kernel (`syscall.Mount`): input to DR-16-2 |
| `t128-red.txt`, `t128-green-run1..3.txt`, `probes-mutations.txt` | T128 | RED (scripts absent), GREEN x3 (protocol probes, carrier, wrong credentials, refusals), mutations all CAUGHT (run 1) |
| `t129-red.txt` (the files of git HEAD), `t129-green-run1..3.txt`, `compose-mutations.txt` | T129 | no literal credential or port, digest-pinned, labelled; 9 mutations CAUGHT |
| `t130-red.txt`, `t130-green-run1..3.txt`, `corpus.sha256`, `seed-mutations.txt` | T130 | deterministic seeder, 6 mutations CAUGHT |
| `t131-red.txt`, `t131-green-run1..3.txt`, `updown-mutations.txt` | T131 | per-project lease and label-only teardown, 4 mutations CAUGHT |
| `t132-red-<proto>.txt`, `t132-green-<proto>-run1..3.txt`, `ev/` (ledgers `ledger-<proto>/`, `roundtrip-<proto>-run<n>.txt`, `roundtrip-<proto>-mutations.txt`) | T132 | round trips x3 as `ev/1` records for postgres, redis, ftp, smb, webdav; minio BLOCKED (`t132-green-minio-run1.txt`, `minio-blocked.txt`) |
| `nas-leg-red.txt`, `nas-leg-green-run1..3.txt`, `nas-readonly-leg.json`, `nas-mutations.txt` | T132 real-NAS leg | read-only leg against two real Synology hosts; no name, content, IP or credential recorded |
| `t133-red.txt`, `t133-green-run1..3.txt`, `concurrency.json`, `concurrency-mutations.txt` | T133 | two stacks at once |
| `t135-red.txt`, `t135-green-run1..3.txt`, `blocked-external.json` | T135 | blocked-external records (names only) |
| `shellcheck-test-infra.txt` | all | shellcheck -S warning: 12 warnings (unused variables, ls|grep), no error |

RED captures for T128, T131, T133 and T132 ran against an ABSENT script directory (the stack scripts were prototyped by hand before the tests were written; the RED is the test against the absent path, not a record of
the tests preceding the first prototype). T129's RED ran against the committed (HEAD) compose files. NFS records: `../wp10/nfs-*`.

## wf12/ - fixes for the WF12 independent review of commit 6d5ebb64 (NO-GO: 1 HIGH, 8 MEDIUM, 11 LOW)

Revision 2 of this README (2026-10-07). `wf12/SHA256SUMS` covers the directory (`sha256sum -c`). Every capture has the identity header (head, run_at, sha256 of the files under test, command, exit). Tests were run ONE AT A TIME.

| Finding | Files | Content |
|---|---|---|
| F1 consumers of the old compose contract | `f1-red.txt`, `f6-callers-red.txt`, `f1-consumers-green-run1..3.txt`, `consumers-mutations.txt`, `f1-go-helper-green.txt` | RED at HEAD (bare compose refused, 10 fixed-contract strings in the Go helper, 17 stale instructions, 3 `down.sh` callers without an operation); GREEN x3 (container-build `--validate-only`, setup-test-env real start, enumeration of tracked references, caller scan); Go helper `go vet` + 2 tests through `TIC catalog-api unit` |
| F2 leaks (pod, out dirs) | `f2-f5-f6-updown-red.txt`, `f2-f5-f6-updown-green-run1..3.txt`, `f2-f5-f6-updown-green-final.txt`, `updown-mutations.txt`, `f2-sweep-green-run1..3.txt`, `sweep-mutations.txt` | RED = the new test against the scripts of HEAD (20 failed checks); GREEN x3; 15 up/down mutations and 7 sweep mutations CAUGHT |
| F3 secrets reachable from clients | `f3-isolation-red.txt`, `f3-isolation-green-run1..3.txt`, `client-isolation-mutations.txt`, `f3-nas-leg-green-*.txt`, `nas-mutations.txt`, `nas-readonly-leg.json` | RED (the real `.env`, another project's credential file and a decoy visible at `/src`), GREEN x3, 3 + 4 mutations; the NAS leg re-run once against the real hosts after the fix |
| F4 compose scanner | `f4-compose-red.txt`, `f4-compose-green-run1..3.txt`, `compose-mutations.txt` | RED = the HEAD scanner passes reviewer mutants RM1, RM2, RM3, RM5, RM7; GREEN x3 with all of them CAUGHT |
| F5, F6 lease checks, owner-checked teardown | `f2-f5-f6-*`, `f5-f6-f11-concurrency-green-run1..3.txt`, `concurrency.json` (measured fields), `concurrency-mutations.txt` | holder identity + liveness; RM4 CAUGHT |
| F7 NFS records | `f7-fallback-green-run1..3.txt`, `f7-fallback-refusal.txt`, `nfs-fallback-mutation.txt`, `f7-blocked-green-run1..3.txt`; records `../wp10/nfs-fallback.json` (now `blocked`) and `blocked-external.json` (derived, agree) | |
| F8, F10 refusal signals | `f8-probes-green-run1..3.txt`, `probes-mutations.txt`, `f8-roundtrip-<proto>-green.txt`, `ev/` | RM6 and per-protocol equivalents CAUGHT |
| F13 | `f13-ftp-capability-run1.txt`, `ftp-capability.txt` | control pair with and without `AUDIT_WRITE` |
| F14, F15, F16, F17 | `f14-seed-green.txt`, `f3-nas-*`, up/down captures | |
| F19, F20 | `f20-nfs-terminal-green-run1..3.txt`, `nfs-terminal-mutation.txt` | RM8 and a behaviour-modelling mutation CAUGHT |

Honest boundaries: the F9 argv exposure is a property of podman-compose (it expands `env_file` into `-e` arguments too) and is NOT fixed; a full containerized build with the new per-run env (`scripts/container-build.sh` without `--validate-only`) was not run; the TIC lanes still mount the whole repository (owned by `scripts/containers/`); the RED of F7/F19/F20 rests on the reviewer's recorded observation (RM8) rather than a HEAD replay.
