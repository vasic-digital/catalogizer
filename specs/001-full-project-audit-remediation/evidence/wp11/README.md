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
