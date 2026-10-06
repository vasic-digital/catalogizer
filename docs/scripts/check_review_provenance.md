# check_review_provenance.sh - User Guide

| Field | Value |
|---|---|
| Revision | 7 |
| Created | 2026-10-05 |
| Last modified | 2026-10-06T16:00:00Z |
| Status | tracked since commit 26755ca5; revision 7 re-aligned to the script after round 6 (PR-1 HOME is the private work dir, PR-3, PR-4 residual names `unset` and `command`); round 5 review passed WF6, independent review of this revision owed (constitution 11.4.142) |
| Source | `scripts/review/check_review_provenance.sh` |

> `$EV` in this guide means the evidence root `specs/001-full-project-audit-remediation/evidence` (repository-relative), as defined in `specs/001-full-project-audit-remediation/evidence/wp09/README.md`.


Companion guide (Helix Constitution 11.4.18) for the script above (task T094a, plan KC-P1; 11.4.209, 11.4.231(F.2)).
Index row: owed to `docs/scripts/README.md` (T036 lists it, the index does not exist yet).

## Purpose
Validates a `[REVIEW]` verdict file (`$EV/reviews/<verdict>.json`) against (a) the provenance record the Workflow run
wrote beside it (`<verdict>.provenance.json`) and (b) `review-verdict.schema.json`. It never starts a review.

## Usage
`scripts/review/check_review_provenance.sh [--bootstrap] <verdict.json> [<provenance.json>]`
An empty second argument is a usage error (exit 2), never a fallback to the default record.
Exit 0 accepted; 20 refused (stderr: exactly one line `REFUSED reason=<code> ...`, values escaped and bounded);
2 usage error or a missing tool (`jq`, `python3`). Consumers must pass an immutable copy of the verdict (the
`cpa-host --commit` materialisation): the script reads the verdict exactly once into a private 0700 temporary
directory and runs every check and the sha256 on that copy (round 3, m-1).

## Bootstrap mode (`--bootstrap`, round 4, review r3 B-1)
Three binding call sites run this checker from the working tree because no approved copy exists yet: tasks.md
**T046a** step (1) (`$EV/reviews/WP-04.json`, captured to `$EV/wp04/adoption-provenance-precheck.json`), CENTRAL
**C6** (a project's first approval, "checked by T046a step (1) instead") and **T094c** (`--list-trusted HEAD` before
the owner approves). For the first two, the explicit mode exists:
`env -u CPA_APPROVED_DIR scripts/review/check_review_provenance.sh --bootstrap <verdict> [<record>]`, run from the
work tree. It reads the schema from the work-tree file at the fixed default path and accepts it ONLY if its sha256
equals the sha256 that the verdict lists in `reviewed_files` for that path (exactly once): the schema checked is
exactly the one the owner is about to approve. Refusals: `bootstrap_schema_not_reviewed` (the verdict does not list
the schema once), `bootstrap_schema_sha_mismatch`, `verdict_schema_unavailable` (absent, unreadable, or a symlink
leaving the work tree). Exit 2 (never an accept) when `CPA_APPROVED_DIR` or `CPA_EXEC_SHA256` is set (an approved
copy exists: use the trusted mode), when `CRP_SCHEMA_REL` or `CRP_RUN_ID_PATTERN` is set, or outside a git work
tree. The accept line carries `(BOOTSTRAP ...)`. There is no silent fallback: without `--bootstrap`, an unset
`CPA_APPROVED_DIR` is refused. The mode attests the schema the verdict reviewed, not an owner-approved one.
**T094c** calls a mode (`--list-trusted`) that does not exist yet (T094d adds it): the same bootstrap rule is owed to
T094d. **C6 mechanical list**: it names only `scripts/commit-push-all.sh` and the checker; the schema reaches the
first manifest only because T046 lists it for review. Asking the owner to add the schema to C6's list is owed
(otherwise an approved checker refuses every later verdict, `verdict_schema_unavailable`).

## Environment
- `CPA_APPROVED_DIR`: the approved-copy root (`$CPA_RUN/released`, CENTRAL C5, C6 and the rev 32 rule). The verdict
  schema is read ONLY from `$CPA_APPROVED_DIR/<CRP_SCHEMA_REL>`, never from HEAD or the working tree. Unset or empty:
  refused `verdict_schema_unavailable` (the check is never skipped).
- `CRP_SCHEMA_REL`: schema path under `CPA_APPROVED_DIR`: relative, no `..` segment, and its real path (symlinks
  resolved) must stay under the real path of `CPA_APPROVED_DIR` (else `verdict_schema_unavailable`). Default
  `specs/001-full-project-audit-remediation/contracts/review-verdict.schema.json`. UNCONFIRMED: that `released/`
  mirrors repository-relative paths is read from tasks.md T042 (`released/scripts/build/remote/emit.sh`), not
  observed from a real run directory.
- `CRP_RUN_ID_PATTERN`: Oniguruma regex without anchors (`\A` and `\z` are added). Default
  `wf_[0-9a-f]{8}-[0-9a-f]{3}`. That shape was OBSERVED on this host (all 1656 Workflow run records, per the
  round-2 review survey); UNCONFIRMED as a general rule across harness versions, hence configurable. An invalid
  pattern is a configuration error (exit 2).
- **Trusted mode** (`CPA_EXEC_SHA256` set) and `--bootstrap` REFUSE both `CRP_*` variables (exit 2): otherwise an
  environment variable would bend the approved checker (accept any schema, accept any run id). CENTRAL C5 says
  `CPA_EXEC_SHA256` is set only for `--exec-approved`; for `--check-provenance` and for `approve`'s C6 check it names
  only `CPA_APPROVED_DIR`. The refusal therefore does NOT fire on those paths (a `CRP_RUN_ID_PATTERN='.*'` would be
  honoured there): **owed to T042/C5** is that `cpa-host` sets the trusted marker in EVERY mode (UNCONFIRMED until then).
- **Interpreter isolation (review r4 I-1).** Both Python stages run as `python3 -I -` (isolated mode): the current
  directory is not put on `sys.path`, and `PYTHON*` variables and the user site are ignored, so an untracked
  `jsonschema/` or `json.py` in the working directory (where C5 runs every `--exec-approved` child) cannot replace the
  schema validator or the strict-JSON layer. A host whose `jsonschema` module lives only in the user site (not the
  system site) gets `verdict_schema_unavailable` (fail closed, never a skipped check).
- **Environment scrub (review r4 I-2), the second line.** At start the script unsets every shell function (an exported
  function named `jq`, `python3`, `cat`... would replace the real tool) and every variable that redirects a child:
  `LD_*`, `PYTHON*`, `BASH_ENV`, `ENV`, `BASH_FUNC_*`, `CDPATH`, `GLOBIGNORE`, and resets `IFS`; no child process
  inherits them. This cannot be the only line: bash has already imported exported functions and run `BASH_ENV`
  before the script's first line (a hostile `unset`/`compgen` function can defeat the scrub), and `PATH` is not
  restored (a caller may legitimately shim it). The caller is the first line.
- **`$HOME/.jq` and tool unusable (review r5 F1, F2).** jq auto-loads `$HOME/.jq`, whose definitions can shadow builtins
  (`test`, `any`) and switch the run-id and model checks off; every jq call therefore runs with `HOME` set to the
  script's private empty 0700 work dir. After the scrub the script smoke-tests `jq -n 1` and `python3 -I -c ''`: a tool
  that cannot start (for example one that needs `LD_LIBRARY_PATH`) is exit 2, "tool unusable", never a record refusal
  (exit 20). The allow-list environment for T042 excludes `HOME`, `SHELLOPTS` and `BASHOPTS` by construction (they are
  not in the enumerated set). Residual (owed, owner O4): an exported `unset` or `command` function defeats the in-script scrub (round 6, PR-4 names `command`: every jq call goes through it).
- **Requirement for T042 (cpa-host): an allow-list environment, not a deny-list.** The earlier requirement ("a scrubbed
  environment: PATH and `CRP_*`") was a two-item deny-list; `PYTHONPATH`, `BASH_ENV` and exported functions each
  defeated trusted mode in the review's probes. `cpa-host` MUST start the checker, in every mode (`--exec-approved`,
  `--check-provenance`, `approve`'s C6 check), with an allow-list environment: `env -i` plus the enumerated `CPA_*`
  variables, a fixed `PATH` and a fixed locale, and never `BASH_ENV`, `ENV`, `BASH_FUNC_*`, `PYTHON*` or `LD_*`
  (`env -i bash --noprofile --norc <checker> ...`): a scrubbed environment by construction. UNCONFIRMED: not yet applied to the T042 text; recorded as an owed
  task change in the evidence INDEX.
- **Validator outcomes (review r4 m-1).** Validator exit 0 valid; exit 10 the schema reports errors
  (`verdict_schema_invalid`); any other status (module missing, schema unusable, a crash, a kill) is
  `verdict_schema_unavailable`, never "invalid" and never an accept. The approved schema must be self-contained: a
  `$ref` to a sibling file fails closed as unavailable.

## Record fields (UNCONFIRMED: exact names are this task's choice; T094a names only run id, model, effort, `verdict_sha256`)
`workflow_run_id`, `model`, `effort`, `effort_argument` (the effort the run was started with; null when none),
`verdict_sha256` (sha256 of the verdict file's final bytes, full 64-hex, compared for exact equality).

## Checks
- Strict JSON for both files: exactly one top-level object, UTF-8 without BOM, no NaN/Infinity; a second document,
  an array, a BOM or invalid UTF-8 is refused (`verdict_unreadable` / `provenance_unreadable`).
- Run id: a non-empty string (else `workflow_run_id_absent`) matching the configured shape (else
  `workflow_run_id_malformed`).
- Record `model`: a JSON string equal to one id of the closed allow-list in the script (`claude-opus-5`,
  `claude-opus-5-5`, `claude-opus-4-7`, `claude-opus-4-6`, `claude-opus-4-5`, `claude-opus-4-1`, `claude-opus-4`;
  UNCONFIRMED: the list is this task's choice, extend it only by an owner-approved edit). Exact match: no substring,
  no case folding, no array, no several ids joined by spaces.
- Record `effort` equals `xhigh` (`effort_not_xhigh`); `effort_argument` present (`effort_argument_absent`) and
  equal to `xhigh` (`effort_argument_mismatch`, a present but wrong value).
- The verdict's `model` and `effort` equal the record's (`verdict_field_mismatch`). All comparisons are JSON value
  equality inside jq, so trailing newlines or other lossy shell handling cannot make two different values equal.
- Record `verdict_sha256` present (`verdict_sha_absent`) and equal to the sha256 of the verdict bytes
  (`verdict_sha_mismatch`; digest taken from stdin, so a path with a backslash cannot corrupt it).
- Schema (CENTRAL C6: "the full schema check is the checker's, T094a"): the verdict validates against
  `review-verdict.schema.json` (python3 `jsonschema`), so a GO with `blocking_findings` above 0, a missing
  `covers_runs`, a verdict other than GO/NO-GO and so on are refused `verdict_schema_invalid`. Schema file absent,
  unusable, `jsonschema` missing or `CPA_APPROVED_DIR` unset: `verdict_schema_unavailable` (fail closed).

## Honest boundaries (11.4.6)
- **Record authenticity is NOT verified.** The script proves the record is consistent with the verdict. A record typed
  by hand with a well-formed run id and the right sha passes. Authenticity comes only from where the record is written:
  the Workflow run itself, `cpa-host --check-provenance` over approved copies (CENTRAL C5) and T094b/T094d. The run
  id is not looked up in any registry.
- **Effort is requested, never observed.** On this host, none of the 1656 Workflow run records
  carries an effort key (0 hits for `effort`, `reasoningEffort`, `effortLevel`; the class-matched needle `model`
  is recorded), so the record's `effort` and `effort_argument` can only be copied from what the dispatcher asked for.
  This check attests a requested `xhigh`, not an observed one. To be recorded on the KC-P1 register item (T094b).
- **What is still owed.** (1) Review of this revision (round 4, 5 and 6 reviews are recorded in the evidence INDEX). (2) The `CPA_APPROVED_DIR` layout (`released/` mirroring repository
  paths) is UNCONFIRMED until T042's `cpa-host` exists and can be exercised with this checker. (3) The wiring through
  `cpa-host --check-provenance` and `approve` (C5, C6) is T042/T094d, not this task. (4) The run-id shape and the
  model allow-list stay UNCONFIRMED across harness versions.

## Refusal reasons
`verdict_unreadable`, `provenance_absent`, `provenance_unreadable`, `workflow_run_id_absent`,
`workflow_run_id_malformed`, `model_not_opus`, `effort_not_xhigh`, `effort_argument_absent`,
`effort_argument_mismatch`, `verdict_field_mismatch`, `verdict_sha_absent`, `verdict_sha_mismatch`,
`verdict_schema_unavailable`, `verdict_schema_invalid`, `bootstrap_schema_not_reviewed`,
`bootstrap_schema_sha_mismatch`.

## Test
`scripts/review/tests/test_check_review_provenance.sh` (`CRP_SCRIPT` overrides the script for mutation runs; refusals
are asserted on exit code 20 and the reason, usage errors on 2; the schema fixtures use a copy of the real schema as
the approved copy). Mutations: `scripts/review/tests/mutations_check_review_provenance.sh` (prints each mutant's diff
and whether the suite killed it; `EQ*` mutants are documented equivalents and must survive). Exit 0 only when at least one mutant ran and every mutant that ran was killed: exit 3 when no mutant ran (a stray `MUT_ONLY` naming nothing), exit 4 for `MUT_ANCHORS_ONLY` (anchors checked, suite not run, nothing killed), exit 1 on a survivor or an unapplied anchor (round 7, F-F). Evidence: see
`specs/001-full-project-audit-remediation/evidence/wp09/review-provenance-INDEX.txt`.
