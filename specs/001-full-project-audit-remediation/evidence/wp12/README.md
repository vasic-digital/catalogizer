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
