# regen_speckit_catalogue.py - Companion Guide

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-06 |
| Last modified | 2026-10-06T17:15:00Z |
| Status | new, untracked when written; implemented test-first (T075 RED, T076 GREEN x3, 14 paired mutations); independent review owed (constitution 11.4.142); not yet indexed by `docs/scripts/README.md` (that file is another agent's, not edited here) |
| Source | `scripts/governance/regen_speckit_catalogue.py`; test `scripts/governance/tests/test_regen_speckit_catalogue.py`; mutations `scripts/governance/tests/mutate_regen_speckit_catalogue.py`; fixtures `scripts/governance/tests/fixtures/speckit_catalogue/` |

## Purpose

WP-G1 (docs/12 section 13.2, tasks.md T075, T076, T081; constitution 11.4.77 regeneration mechanism). The Spec Kit governance layer
(`.specify/memory/constitution.md` and `constitution-appendix.md`) lists every constitution anchor and pins the constitution commit and
`Constitution.md` hash. Those parts are a pure function of the constitution repository's `constitution_index.yaml` and `Constitution.md`; they
were produced by scripts that never reached the repository. This script regenerates them and its `check` mode is the drift check that compares
the committed files with the regeneration.

## Usage

```bash
python3 scripts/governance/regen_speckit_catalogue.py write|check \
  --index <constitution_index.yaml> --canon-md <Constitution.md> --commit <40-hex constitution commit> \
  --constitution-file .specify/memory/constitution.md --appendix-file .specify/memory/constitution-appendix.md
```

The test runs in the image mapped to `python3`: `scripts/containers/run_pinned.sh IMG-TESTUTIL -- python3 scripts/governance/tests/test_regen_speckit_catalogue.py`
(`--script <path>` runs the same cases against another copy of the script; the mutation runner uses it).

## What it regenerates and what it keeps

| Where | Regenerated | Kept byte for byte |
|---|---|---|
| constitution file | the `**Pinned sources` paragraph (commit, `Constitution.md` sha256, index-lag statement) and the whole `## Anchor Catalogue` section (anchor count, undefined anchor numbers, each group with its count and every `- **§id** — title` line) | everything else |
| appendix file | the `\| Canon pin \|` table row | everything else (the hand-written digests) |
| both | no trailing whitespace on any line; exactly one newline at the end (T040b, class `governance-carrier`) | |

Rules reproduced from the committed catalogue (verified by regenerating it from the index at the current pin 10b7a06 and diffing: the 283-anchor
catalogue came out byte-identical): a leading `— ` of an index title is dropped; a title longer than 200 characters is cut at the last space inside
its first 200 characters and ends with ` …`; group display names come from the group slug (`ui` -> `UI`, `tdd` -> `TDD`, the compounds `anti-bluff` and
`multi-track` keep their hyphen); anchor numbers 11.4.1 up to the highest that the index does not define are listed (runs of three or more as `a to b`).

Index lag: the index records the sha256 of the `Constitution.md` it was generated from. When it differs from the hash of `--canon-md`, both files say so
and name the anchors whose `### §<id>` heading (an id has at least one dot, so `### §8.` section headings do not count) is in the canon but not in the index.
The catalogue can only list what the index knows. `NOTE manual:` lines (stdout) point at hand-written text that still names the old anchor count.

## Exit codes

| Code | Meaning |
|---|---|
| 0 | `write` done (or nothing to change); `check` found both files in sync |
| 1 | `check` only: DRIFT, with the file and the first differing line; nothing is written |
| 2 | usage or input error (bad `--commit`, unreadable or malformed index, a missing marker `**Pinned sources` / `## Anchor Catalogue` / `| Canon pin |`); nothing is written (all-or-nothing: both files regenerate in memory before either is replaced, each by temp file and rename) |

## Safety

Reads only the paths it is given; writes only the two files named, atomically; no network; python3 standard library plus PyYAML (present in IMG-TESTUTIL).
Not run by anything automatically yet: T081 runs it at the pin bump; whether a standing check calls it (AM-G1) is a later task.

## Tests

17 cases, each an executing run of the script on fixtures with hand-written golden outputs (the oracle is independent of the implementation): golden
reproduction of both files, idempotence, `check` exit 0 / 1, hand-edited catalogue / pin line / appendix reported as DRIFT, repair by `write`, lagging
index (and a `### §8.` section heading that must not be listed), trailing whitespace and single final newline, input without a final newline, bad index and
missing markers exit 2 with nothing touched. 14 paired mutations (one defect per copy of the script) are each killed by a named case.

## Known limits

* The appendix digests (4,900 lines) and the digest of sections 1 to 12 are hand-written: the script does not and cannot regenerate them. A new or edited anchor
  needs a hand-written digest (at a71b1767: 11.4.276 new; 11.4.209, 11.4.211, 11.4.230, 11.4.134 edited).
* Counts written in prose elsewhere (`283 anchors`) are reported as `NOTE manual`, not rewritten.
* The revision header and Last modified fields of the two files are not touched; the pin bump task sets them.
