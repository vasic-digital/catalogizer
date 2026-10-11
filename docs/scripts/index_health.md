# index_health.sh - Companion Guide

| Field | Value |
|---|---|
| Revision | 7 |
| Created | 2026-10-06 |
| Last modified | 2026-10-08 (round 9, review WP-02-T028-r2 N1/N2/N4/N5/N7: baseline-covering classes, verified live proofs, bound drift report, case-insensitive classes, mandatory `--captured-not-before`; independent review owed); earlier: 2026-10-08 (round 8, review WP-02-T028 F1/F2/F3/F8: scope-derived P3 classes, INCOMPLETE exit 3, live-proof/drift/lumenignore inputs, captured-not-before; independent review owed); earlier: 2026-10-06T17:00:00Z (round 7 fixes: P3 case variants and NFD roots, m29 equivalence); earlier: 2026-10-06T16:00:00Z (round 7 docs sync); earlier: 2026-10-06 (round 6: N6-5 encoding class of third-party roots) |
| Status | tracked since commit 26755ca5; round 7 documentation sync (status and counts re-measured by the WF7 review; independent review of this revision owed, constitution 11.4.142); indexed by `docs/scripts/README.md`; origin: work product of T021 (WP-02) |
| Source | `scripts/audit/index_health.sh`; tests `scripts/audit/tests/test_index_health.sh` (T016, 13 checks), `test_index_health_extra.sh` (105 checks, round 8), `mutate_index_health.sh` (45 mutants, round 8: see the last section; earlier 31 mutants: 30 caught, 1 EQUIVALENT `m29-eq-...`, 0 survived; the harness prints EQUIVALENT for `-eq-` ids and does not count them: every control character is a Unicode `C*` category character that the next loop of `noncanonical_root` refuses, so dropping the explicit test cannot change a result) |

## Purpose

The read-only gate for the index health proofs P1..P8 of `docs/02` section 4.1 (output shape `index-health/1`, section 4.2). It judges files the caller
captured beforehand. It never runs an index writer (`codegraph_safe.sh`), never opens an index database and never searches Lumen (a `lumen search`
runs EnsureFresh and so writes the Lumen index, `docs/02` section 3). Index writes stay on the sanctioned single-writer path of section 4.5.

## Usage

```bash
scripts/audit/index_health.sh --out FILE --cg-status FILE --cg-files FILE --tracked FILE --needle-pos PATH --needle-neg PATH \
   [--lumen-status FILE | --lumen-bin FILE] [--lumen-files FILE] [--parity-tolerance-pct N] [--lumen-unindexable-ext CSV] \
   [--third-party-roots FILE] [--own-org-roots FILE]
```

Round 8 adds the required `--scope FILE` (normally `config/index/scope.yaml`) and the optional `--captured-not-before ISO` (with zone), `--lumenignore FILE` with
`--lumenignore-fresh FILE` (given together), `--drift-report FILE` and `--live-proofs FILE`.

Exit status (round 8, F3): 0 only when every proof ran and none FAILs (verdict PASS); 1 when a proof FAILs (verdict FAIL); 3 when no proof FAILs but at
least one was SKIPped (verdict INCOMPLETE, `complete` false, `blocked_unavailable` maps each skipped proof id to its reason); 2 usage error. A skip is never a
pass: the earlier rc 0 with `complete: false` is gone.

| New input | Proof row | Meaning |
|---|---|---|
| `--live-proofs FILE` | P4, P5, P6 | `{"P4": {"verdict": "PASS or FAIL", "cmd": "...", "evidence_sha256": "64 hex"}, ...}` written by whoever ran the live queries (T022/T023/T024); an absent id stays SKIP, a malformed entry is FAIL |
| `--drift-report FILE` | P-ScopeDrift | JSON list `[{"check","rc"}]` of `scripts/audit/scope_drift_check.sh`: `scope_render_check`, `scope_to_lumen_json_check`, `lumenignore_fresh` all present and rc 0 |
| `--lumenignore FILE` + `--lumenignore-fresh FILE` | P-LumenIgnore | the committed file equals the fresh `gen_lumenignore.py` render; an empty fresh render is FAIL (blind) |
| `--captured-not-before ISO` | P7 | the Lumen `Captured:` time must not be earlier (deterministic, no clock inside the gate) |

P3 now takes its forbidden classes from `scope.yaml` `forbidden_classes` (one class source with the renderer): env files, `secrets/` directories, `*secret*`
names, key and keystore extensions (`jks keystore p12 pfx pk8 pem key`), QA corpora and cache directories. Each class carries a positive and a negative path and
is self-tested before it judges: a class that misses its own positive or hits its own negative, a scope without classes, or an unreadable scope is P3 FAIL
(`forbidden_classes_unusable`). The row lists `forbidden_by_class`. Reason renamed `forbidden_class_files_indexed` (was `secret_class_files_indexed`).

Exit 0 = no proof FAIL (`verdict` PASS), 1 = at least one proof FAIL, 2 = usage error (no `--out`, unknown, duplicate or option-like option, bad tolerance).
Every option takes exactly one value; a value that is empty, starts with `-` or has a control character is a usage error.

## Inputs and proofs

| Proof | Input | PASS when |
|---|---|---|
| P1 | `--cg-status` (`codegraph status --json`) | fields `index.state`, `index.pendingRefs`, `index.reindexRecommended`, `pendingChanges.{added,modified,removed}`, `worktreeMismatch` ALL present (absent = FAIL, never read as zero); state `complete`; counts are integers equal to 0; `worktreeMismatch` null; `reindexRecommended` false |
| P2 | `--cg-files` (list of `{path}`), `--tracked`, `--needle-pos`, `--needle-neg` | tracked set non-empty, every tracked path indexed, the positive needle is in the index and the negative needle is not. The row records both needles (`needle_pos`, `needle_neg`) |
| P3 | the P2 file list; `--scope`; optional `--third-party-roots`, `--own-org-roots` | no path of a `scope.yaml` forbidden class indexed (round 8; `.env.example`/`.env.sample` exempt); no file under a third-party root; at least one file under each own-org root. A root list that is not supplied is named in `not_checked`, not claimed. A third-party root line that is not a canonical repo-relative path (control character incl. CR, surrounding whitespace, leading `/` or `./`, an empty/`.`/`..` component) FAILs P3 with `third_party_root_not_canonical` (round 5, R4; before it matched nothing and read as clean); one trailing `/` and blank lines are accepted |
| P4, P5, P6 | `--live-proofs` (round 8) | `SKIP` with a reason unless `--live-proofs` supplies them: golden questions, freshness and the embed call need live index or git queries that are not inputs of this read-only gate |
| P-Lumen | `--lumen-status` or `--lumen-bin` | `--lumen-status` readable = PASS (evidence source). `--lumen-bin` alone is FAIL (`probe_not_run_...`, the probe would write the index) or FAIL when the path is not executable; neither option = FAIL |
| P7 | the captured `index_status` text | `Files == Indexed`, `Chunks > 0`, `Stale: no`, and a parseable `Captured: <iso time>` line (an undated capture is FAIL) |
| P8 | `--lumen-files`, the P2 list | after removing the `--lumen-unindexable-ext` extensions from BOTH sets, symmetric difference as a percent of the CodeGraph set is `<= --parity-tolerance-pct` (default 0). No `--lumen-files` = `SKIP` with a reason |

The output holds `schema`, `verdict`, `complete` (false while any proof is `SKIP`), `skipped_proofs`, `inputs` (path and sha256 of every file judged), the
`codegraph`/`lumen` proof rows and `parity`. There is no timestamp, so two runs on identical inputs write byte-identical files (tested, X15). The file is
written by atomic rename. A `verdict` of PASS with `complete: false` means no proof failed, not that every proof ran.

## Tests and mutation

`test_index_health.sh` (T016) and `test_index_health_extra.sh` run against fixtures only (temp dir, jq and python3). `mutate_index_health.sh` applies 25
single-point mutants (one per check, including the redundancy-defeating ones: a missing positive needle with a complete tracked set, an absent field
with the not-a-count check intact) and requires either test to FAIL for each. The T016 test alone could not kill the positive-needle mutant (a missing
tracked file fails P2 independently); the supplementary X1 case isolates it.

## Honest limits (UNCONFIRMED where stated)

- UNCONFIRMED: behaviour on the real CodeGraph and Lumen captures. The field names are the ones of the T016 fixtures and `docs/02` section 4.2; the
  gate has not been run on a live capture because the index writer refuses on this host (needs 32 GiB MemAvailable).
- P3's secret-class pattern is a basename pattern; a path that merely contains the word `secret` is flagged (the `docs/02` rule), including legitimate source.
- P4, P5 and P6 are not implemented here (see the table); a later task must feed them or keep them as recorded SKIPs.

## Round 6 (N6-5, MR4; revision 3)

The round-5 canonical-root check closed the reported spellings, not the class. A root that can never equal an indexed path for an ENCODING reason read as clean exactly the same way, so P3 PASSED while third-party files were indexed. A third-party root line is now refused (`P3 FAIL third_party_root_not_canonical`) also when it holds: a character of a Unicode category `C*` (control, format: a byte order mark U+FEFF, a zero-width space U+200B; unassigned; private use; surrogate) or `Z*` other than an ordinary space (no-break space, U+2028, U+2029), the replacement character U+FFFD (an invalid UTF-8 byte decoded with `errors="replace"`), a backslash, or a spelling that is not NFC (a decomposed spelling never equals the composed one that git normally records). Golden-false cases that must NOT fire: an NFC non-ASCII root that holds an indexed file is a normal `third_party_files_indexed` hit, an NFC root with no indexed file is PASS, an inner ASCII space is a plain character. Residual (stated): a repository that genuinely records NFD file names has its roots refused until they are written in NFC; DEL (MR4) is also covered by the `C*` category check, so the single mutant that drops only the explicit `ord(c) == 127` test is equivalent (it stays documented in `run_wp04f_mutations.sh` as `EQ-W6`). Tests: `test_index_health_extra.sh` 45 checks (was 34).

Round 7 (WF7 M-6): two residuals of the P3 root check are closed. (a) A CASE VARIANT of a real third-party root (`Third` for an indexed `third/`) matches nothing exactly and read as clean; it now FAILs `third_party_root_case_mismatch` (a root that matches nothing in any case is still legitimate: the tree is simply not indexed, which is why a typo or a homoglyph root stays PASS, an owner decision recorded in `$EV/wp04/wp04g-notes.md`). (b) An NFD root that names an NFD path git really indexed (a macOS-made commit) is a normal `third_party_files_indexed` hit, no longer refused `third_party_root_not_canonical`; an NFD root while only the NFC path is indexed stays refused. Tests: `test_index_health_extra.sh` 54 checks (was 47), mutants N1 to N3 in `scripts/repo/tests/run_wp04g_mutations.sh`.

## Round 8 (review WP-02-T028, 2026-10-08)

Findings closed here: F1 (P3 classes from `scope.yaml`, each member tested plus a control needle), F2 (cases for the five surviving reviewer mutants: `reindexRecommended`, `pendingChanges` nonzero per key, unparseable `Captured`, key and keystore extensions, own-org prefix boundary `a/b` versus `a/bc`), F3 (exit 3 INCOMPLETE), F8 (P7 age floor). `test_index_health.sh` 13 checks, `test_index_health_extra.sh` 105 checks, `mutate_index_health.sh` now 45 mutants (m32..m45 are the round-8 additions; m14 renamed `m14-verdict-never-fail`). Honest limit: reaching verdict PASS needs the live proofs of T022/T023/T024, which do not exist yet (T022 is blocked on memory), so no real run can be PASS today.

## Round 9 (review WP-02-T028-r2, 2026-10-08; revision 7)

- **N1, class coverage.** Every non-negated pattern of the scope's own `baseline_excludes` (all four classes: build outputs, caches, secrets, QA corpora) must be covered by a `forbidden_classes` entry. The gate builds a probe path from each pattern (`**/` and a leading `/` stripped, `*` becomes `probe`, a directory pattern gets a file below it) and refuses when no class matches it: `P3 FAIL forbidden_classes_unusable` with `baseline_pattern_uncovered:<pattern>` in `forbidden_class_problems`. Decision on the semantics (documented in `scope.yaml`): `.venv`, `.pytest_cache`, `logs` and the other caches are not secrets, but an index holding a path the scope excludes is the same kind of scope breach, so P3 covers them; classes `credentials_json` and `build_output` were added and `cache_dir` extended. Not claimed: that the probe synthesis covers a pattern shape nobody wrote yet (a new shape is a coverage-check failure, which is the safe direction).
- **N4, case.** Classes are compiled case-insensitively. Measured 2026-10-08: the matcher that applies `codegraph.json` `exclude` in the installed runner v1.6.0 is the `ignore` library 7.0.5 whose default is `ignorecase: true` (`a/server.PEM` is excluded by `*.pem`; with `ignorecase:false` it is not); `git check-ignore` on Linux is case-sensitive. The Lumen matcher is UNCONFIRMED (no source on this host, and a probe would run EnsureFresh which writes its index); P3 judges the CodeGraph file list, so the CodeGraph behaviour is the pinned one. `test_index_health_fix_r7.sh` re-measures the library when node and the runner are present and records a distinct SKIP exit (77) when not.
- **N5.** The exemption is exactly the basenames `.env.example` and `.env.sample`; fixtures prove a `.example`/`.sample` suffix elsewhere (`secrets/db.yaml.example`) still FAILs (mutant m55).
- **N7.** `--captured-not-before` is REQUIRED (usage error, exit 2) whenever `--lumen-status` or `--live-proofs` is given; the caller supplies the bound from its own clock (the gate has none, so two runs stay byte-identical).
- **N2, live proofs are verified.** `--live-proofs` entries are `{verdict, cmd, evidence_path, evidence_sha256, captured_at, kind}`. The gate reads the evidence file (recorded under `inputs.live_evidence_P4..P6`), requires it non-empty, equal to `evidence_sha256`, and to contain the `cmd` text; `cmd` must start with a program of the closed allow-list of that proof (P4 `codegraph`, `lumen_verify.sh`; P5 `codegraph`, `git`; P6 `lumen`, `lumen_verify.sh`); `captured_at` must parse with a zone and not be earlier than `--captured-not-before`. `kind` is `live` or `fallback`; a `fallback` record (grep plus reads, programs `grep rg git cat sed`) is entered as SKIP `fallback_record_is_not_a_live_proof` and so the run is INCOMPLETE, never a live PASS. Reasons: `live_proof_entry_malformed`, `live_proof_cmd_not_allowed`, `captured_at_unparseable`, `live_proof_stale`, `evidence_file_unreadable`, `evidence_empty`, `evidence_hash_mismatch`, `evidence_does_not_record_cmd`. Honest boundary (11.4.240(F)): on a single-uid host a forger who can write the evidence file can also write a matching hash and command line; this raises the cost from one typed JSON line to a fabricated file whose content is on record, it is an instance-tier control, not a capability-tier one.
- **N2, drift report.** `--drift-report` is the `scope-drift/1` object of `scope_drift_check.sh` (`head`, `scope_sha256`, `lumen_scope_sha256`, `lumenignore_sha256`, `checks`). It is bound to THIS gate's inputs: `drift_report_not_for_this_scope` when `scope_sha256` differs from the `--scope` file, `drift_report_not_for_this_lumenignore` when `lumenignore_sha256` differs from `--lumenignore` (not checked, and named in `not_checked`, when no `--lumenignore` is given); the old bare list is `drift_report_unusable`.
- Tests: `test_index_health_fix_r7.sh` (97 checks), `test_index_health_extra.sh` (111), `test_index_health.sh` (13); `mutate_index_health.sh` now 61 mutants (m46..m61 new; m45 re-targeted; m55 is the reviewer's R5; one full run takes about an hour, `ONLY=<regex>` re-runs a subset).
