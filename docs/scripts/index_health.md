# index_health.sh - Companion Guide

| Field | Value |
|---|---|
| Revision | 3 |
| Created | 2026-10-06 |
| Last modified | 2026-10-06 (round 6: N6-5 encoding class of third-party roots) |
| Status | draft, untracked work product of T021 (WP-02); the scripts index `docs/scripts/README.md` and the root `README.md` link are NOT created here (they edit tracked files, deferred) |
| Source | `scripts/audit/index_health.sh`; tests `scripts/audit/tests/test_index_health.sh` (T016, 13 checks), `test_index_health_extra.sh` (20 checks), `mutate_index_health.sh` (25 mutants) |

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

Exit 0 = no proof FAIL (`verdict` PASS), 1 = at least one proof FAIL, 2 = usage error (no `--out`, unknown, duplicate or option-like option, bad tolerance).
Every option takes exactly one value; a value that is empty, starts with `-` or has a control character is a usage error.

## Inputs and proofs

| Proof | Input | PASS when |
|---|---|---|
| P1 | `--cg-status` (`codegraph status --json`) | fields `index.state`, `index.pendingRefs`, `index.reindexRecommended`, `pendingChanges.{added,modified,removed}`, `worktreeMismatch` ALL present (absent = FAIL, never read as zero); state `complete`; counts are integers equal to 0; `worktreeMismatch` null; `reindexRecommended` false |
| P2 | `--cg-files` (list of `{path}`), `--tracked`, `--needle-pos`, `--needle-neg` | tracked set non-empty, every tracked path indexed, the positive needle is in the index and the negative needle is not. The row records both needles (`needle_pos`, `needle_neg`) |
| P3 | the P2 file list; optional `--third-party-roots`, `--own-org-roots` | no secret-class basename indexed (`.env*` other than `.env.example`/`.env.sample`, `*.jks`, `*.keystore`, `*.p12`, `*.pfx`, `*secret*`); no file under a third-party root; at least one file under each own-org root. A root list that is not supplied is named in `not_checked`, not claimed. A third-party root line that is not a canonical repo-relative path (control character incl. CR, surrounding whitespace, leading `/` or `./`, an empty/`.`/`..` component) FAILs P3 with `third_party_root_not_canonical` (round 5, R4; before it matched nothing and read as clean); one trailing `/` and blank lines are accepted |
| P4, P5, P6 | none | always `SKIP` with a reason: golden questions, freshness and the embed call need live index or git queries that are not inputs of this read-only gate |
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
