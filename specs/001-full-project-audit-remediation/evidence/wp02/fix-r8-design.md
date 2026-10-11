# WP-02 fix round 8 - structural design: every proof's verdict is DERIVED from its evidence

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-08 |
| Last modified | 2026-10-08 (structural round, constitution 11.4.276(E)(3); independent review owed) |
| Status | design for `scripts/audit/index_health.sh` round 10; implemented and tested the same round (`fix-r8-*` files in this directory) |
| Inputs | reviews `evidence/reviews/WP-02-T028.json`, `-r2.json`, `-r3.json` (read only); tasks.md T015-T029 |

## 1. Why a structural round

Three reviews, one class (11.4.276(C)): a live proof is accepted on its FORM (a hash that matches a file, a program basename, a JSON shape) while its
VERDICT is the caller's own `verdict` field, not a function of what the evidence says. R3-1: an evidence file reading `recall 0.10 FAIL` with the entry
saying PASS gave P4 PASS. R3-2: the drift report's head and `lumen_scope_sha256` were never compared with anything. R3-3: class tests probed one
depth only. Point fixes per instance were the failure mode; this round fixes the rule.

**Rule.** For every proof P: `verdict(P) = f(evidence bytes, policy, real current values)`. The entry's own `verdict` is a claim to be CHECKED,
never an input: when present it must equal the derived verdict, else the proof FAILs (`entry_verdict_disagrees_with_evidence`). No field is read
and then trusted: each is DERIVED (recomputed from the evidence), VALIDATED against a real value the gate can read itself, or REJECTED.

## 2. Trust boundary

| Tier | What | How it is trusted |
|---|---|---|
| A | P1, P2, P3, P7, P8, P-LumenIgnore compare | computed by the gate from raw files; the raw file's sha256 is recorded in `inputs` (caller-captured, so only as honest as the capture: stated limit, owed: ledger-bind the captures too, tracked as OWED-R8-1) |
| B | P4, P5, P6 evidence, the drift report | the evidence file must be hash-bound to a RECORDED artifact: `--evidence-ledger FILE` (SHA256SUMS format, `<sha256>  <path relative to the ledger directory>`) must be a TRACKED file of `--repo` whose working-tree bytes equal HEAD, and the evidence path must be listed in it with the same sha256 and be tracked and unmodified. A hand-written file that nobody committed is refused. A forger who commits is attributable in git history (11.4.268 spirit); the gate cannot do more without a second actor (11.4.240(F): single-uid host reports `instance`, never `capability`) |
| C | program that produced the evidence | `program:` line of the evidence must be an ABSOLUTE path, literally equal (after `normpath`) to an entry of the policy allow-list for that proof (one `*` per path segment allowed for a version directory, `$HOME` and `$REPO` expanded), its `realpath` equal to the realpath of the matched entry, and it must exist as an executable file. A basename is never enough: `/tmp/evil/codegraph` is refused |

Time: `captured_at` must lie in `[max(--captured-not-before, clock - max_age_hours), clock + clock_skew_seconds]`. `--captured-not-before` is itself
refused (usage 2) when older than `clock - max_age_hours` (a caller cannot widen the window to 1970) or in the future. No timestamp is written to the
output, so two runs on identical inputs and the same clock bucket stay byte-identical.

## 3. Policy DATA: `config/index/live_proof_policy.yaml` (`--live-policy`, required with `--live-proofs`)

`schema: live-policy/1`, `max_age_hours` (default 72), `clock_skew_seconds` (300), `programs: {P4,P5,P6: [abs path patterns]}`,
`fallback_programs` (absolute), `thresholds: {p4_min_queries, p4_min_recall, p6_expected_dims}`. Its sha256 is recorded in `inputs`.

## 4. Evidence format `live-evidence/1` (key: value lines; `#` comments ignored; duplicate keys refused)

Common header, every proof: `schema: live-evidence/1`, `proof: P4|P5|P6` (must equal the slot it is entered in), `program: /abs`, `argv: ...`,
`exit: 0` (must be 0), `captured_at: ISO+zone`, `scope_sha256: 64hex` (must equal sha256 of the `--scope` given to this gate: evidence for another
scope is refused).

| Proof | Raw evidence lines | Parser output (derivation) | PASS iff |
|---|---|---|---|
| P4 golden questions | one row per query: `q: id=<id> expect=<answerable\|unanswerable> result=<correct\|wrong\|refused>`; claimed summary `queries: N`, `correct: M`, `recall: R` | `total` = number of `q:` rows, `correct` = rows `expect=answerable result=correct`, `answerable` = rows `expect=answerable`, `control_ok` = every `expect=unanswerable` row has `result=refused` and there is at least one; recall = correct/answerable | claimed `queries`/`correct`/`recall` equal the derived values (recall to 0.0005) else FAIL `evidence_summary_mismatch`; `total >= p4_min_queries`; `recall >= p4_min_recall`; `control_ok` |
| P5 freshness | `last_indexed: ISO+zone`, `last_commit: ISO+zone` (git log -1 --format=%cI) | both parsed with zones; `last_commit` not in the future | `last_indexed >= last_commit` |
| P6 embed call | `embed_dims: N`, `embed_vectors: N`, `embed_norm: float` | integers/float parsed, `norm` finite | `embed_dims == p6_expected_dims`, `embed_vectors >= 1`, `norm > 0` |
| P-ScopeDrift | the `scope-drift/1` report (ledger-bound like evidence) | `head`, `scope_sha256`, `lumen_scope_sha256`, `lumenignore_sha256`, `checks[]` | each hash equals the sha256 of the REAL file given (`--scope`, `--lumen-json` required, `--lumenignore`), no digest is all zeros, `head` equals `git rev-parse HEAD` of `--repo` or is its ancestor with only files under the ledger directory changed since, `lumenignore_fresh` rc agrees with the gate's own P-LumenIgnore compare, the three checks present with rc 0 |

A `fallback` entry (grep plus reads) must also have ledger-bound evidence with a program from `fallback_programs`; it derives to SKIP (never PASS,
11.4.226) whatever its content says.

## 5. Field table (class-complete: every input field has a derive-or-reject test; ids are cases of `test_index_health_fix_r8.sh`)

| Field (origin) | Rule | Test ids |
|---|---|---|
| entry `verdict` | not trusted; equal to derived or FAIL | L01 L02 |
| entry `kind` | in {live, fallback}; fallback derives SKIP | L03 L04 |
| entry `cmd` | must equal `program + " " + argv` of the evidence | L05 |
| entry `evidence_path` | tracked, unmodified, listed in ledger | L06 L07 L08 |
| entry `evidence_sha256` | equals sha256 of the bytes AND the ledger line; zeros refused | L09 L10 |
| entry `captured_at` | equals the evidence line; window check; unparseable/future/past refused | L11 L12 L13 L14 |
| evidence `schema`, `proof`, `exit`, `program`, `argv` | exact / equal slot / 0 / allow-list / present | L15 L16 L17 L18 L19 L20 L21 |
| evidence `scope_sha256` | equals real scope sha | L22 |
| evidence P4 rows, `queries`, `correct`, `recall` | derived and cross-checked | L23 L24 L25 L26 L27 |
| evidence P5 `last_indexed`, `last_commit` | parsed, ordered, no future | L28 L29 L30 |
| evidence P6 `embed_*` | parsed, thresholds | L31 L32 L33 |
| `--captured-not-before` | required with live inputs; not older than max age; not future | L34 L35 |
| drift `head`, `scope_sha256`, `lumen_scope_sha256`, `lumenignore_sha256`, `checks[].rc`, `schema` | real-value binding; zero digest refused; rc cross-checked | D01-D09 |
| Lumen `Captured:` | window check incl. future | L36 |
| forbidden classes (scope.yaml) and constitution `scope_baseline.txt` members | every member judged at depth 0, 1, 2 | C-table |

## 6. P3 class coverage at every depth (R3-3, R3-4)

* The baseline-pattern coverage probe now builds each non-anchored pattern's member at depth 0 (`x`), 1 (`a/x`) and 2 (`a/b/x`) and requires a
  class to cover EACH; an anchored (`/x/`) pattern is probed at depth 0 only. A class that misses root-level members (`/secrets/`) is
  `baseline_pattern_uncovered:<pattern>` from the tool itself, not only from a test.
* The constitution baseline (`--baseline FILE`, default `submodules/constitution/scripts/codegraph/scope_baseline.txt` when present) is applied
  DIRECTLY as a second predicate over the indexed paths (matchers `exact seg segglob dir topdir`, case-insensitive per N4): `service_account`,
  `prebuilts`, `rockdev`, `rktools` and all other baseline classes now FAIL P3 when indexed (`baseline_class_files_indexed`). One class source
  for the constitution list: the file itself, not a copy of it.
* The tests use a HAND-WRITTEN member table (`scripts/audit/tests/fixtures/baseline_members.tsv`), never a copy of the tool's `baseline_probe`
  (R3-7). Each class and baseline matcher has members at depth 0/1/2 with the expected verdict (root-only classes: nested must PASS).

## 7. Minor findings

* R3-5 exit 77 in a mutant run is `SKIPPED-NOT-KILLED`, counted as unkilled (a survivor), never CAUGHT: `mutate_index_health.sh`,
  `mutate_wp02_fix_r6.sh`, `mutate_wp02_wp03.sh`, `run_wp04f/h_mutations.sh`.
* R3-6 the pre-fix tool is stored under `fix-r8-prefix-tool/` with sha256; every RED file starts with the sha256 of the tool it ran; the RED is
  re-runnable with `IH=evidence/wp02/fix-r8-prefix-tool/index_health.sh.prefix`. The WP-02 tools are still UNTRACKED/uncommitted (no commit this round,
  instruction): `lumenignore_skeleton.py`, `scope_drift_check.sh`, the r6/r7/r8 tests are listed with sha256 in `fix-r8-SHA256SUMS`.
* R3-8 `lumenignore_skeleton.py` fails closed (exit 3) when a `.gitmodules` path is not a gitlink of the listing or an allowed root is absent from
  it: an export (`git archive` + `git init`) can no longer render a different `.lumenignore` silently; `scope_drift_check.sh` then records rc 3 -> 127-class failure of
  `lumenignore_fresh`.
* R2/R4 (reviewer mutants of the probe) were equivalent on today's data: non-equivalent fixtures now exist (class regex
  `^(zzdir|a/zzdir|a/b/zzdir)/` kills "keep `**/`"; nested-only, root-only and depth-1-gap regexes kill each dropped probe depth).

## 8. Honest limits (11.4.6)

The gate proves the evidence is internally consistent, recorded in git, produced under an allow-listed absolute program name, fresh, and that its
verdict follows from its contents. It does not prove the program really ran (that needs a second actor, 11.4.240(F)); it does not prove the golden
set is a good oracle (11.4.245). Tier-A captures are not ledger-bound yet: OWED-R8-1.
