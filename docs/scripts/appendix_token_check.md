# appendix_token_check.py - Companion Guide

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-06 |
| Last modified | 2026-10-06T19:05:00Z |
| Status | new, untracked when written; implemented test-first (RED recorded, GREEN, 15 paired mutations); independent review owed (constitution 11.4.142); not yet indexed by `docs/scripts/README.md` (that file is another agent's, not edited here) |
| Source | `scripts/governance/appendix_token_check.py`; allow list `scripts/governance/appendix_token_allow.tsv`; test `scripts/governance/tests/test_appendix_token_check.py`; mutations `scripts/governance/tests/mutate_appendix_token_check.py` |

## Purpose

The Spec Kit appendix (`.specify/memory/constitution-appendix.md`, Part 1) says every digest states every `CM-*` gate name and every
no-escape-hatch `--flag` of its canon block, and that the comparison "must be re-run when the pin moves". No tracked tool did it: the regen check
(`regen_speckit_catalogue.py check`) guards only the generated regions, so a content edit of a digest, even one that weakens a MUST, passed every
in-repo check (WF8 findings F1 and F3: the 11.4.230 digest dropped six flags and stated there were none). This tool is that comparison.

## Usage

```bash
scripts/containers/run_pinned.sh --network=none IMG-TESTUTIL -- python3 -I scripts/governance/appendix_token_check.py \
  --canon submodules/constitution/Constitution.md --appendix .specify/memory/constitution-appendix.md \
  --allow scripts/governance/appendix_token_allow.tsv [--json <out.json>]
```

Tests and mutations: `... IMG-TESTUTIL -- python3 scripts/governance/tests/test_appendix_token_check.py` (26 cases) and
`... python3 -I scripts/governance/tests/mutate_appendix_token_check.py` (15 mutants, all must be killed).

## Rules

* Canon block-starts: `### §<id>` / `#### §<id>` headings and a line beginning `**§<id> — `. A line that merely starts with a citation
  (`§11.4.134 REFINES ...`, `**§11.4.202 precedence ...`) is not a block-start (measured: 60+ false duplicates otherwise). A block ends at the next
  block-start or any `#`/`##` heading. A lettered sub-clause (`**§11.4.184(I) — ...`) stays inside its parent block.
* Appendix digests: `#### §<id>` to the next `####`/`###`/`##` heading; `#### §<id>(X)` continues the parent digest.
* Tokens: `CM-[A-Z0-9-]*` gate names and `--flag` words; a token wrapped at a hyphen across a line break is joined first.
* A canon token missing from the digest is a GAP (exit 1); a canon block with no digest is NO-DIGEST (exit 1).
* Allow file (TSV `id<TAB>token or *<TAB>reason`): a carrier (placeholder, citation of another anchor's flag, CLI argument in an example) is exempt
  only with a recorded reason; a stale exemption (token present after all, or not in the canon block) fails, so the list cannot rot.
* Exit 2 for usage, unreadable input, BLIND (zero blocks or zero digests extracted: a zero is not evidence), a duplicate canon block-start, a
  malformed allow file.

## Known limits

* A token comparison proves the NAMES are restated, not that the digest's sentence about them is faithful (the 11.4.209 weakening in finding A3 is
  caught only by review). Digest wording stays an independent-review matter.
* Tokens that appear in canon only inside quoted examples need an allow-file row with a reason; adding one is a reviewed edit.
