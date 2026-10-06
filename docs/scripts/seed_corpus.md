# seed_corpus.sh - Companion Guide

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-06 |
| Last modified | 2026-10-06T20:00:00Z |
| Status | new in the working tree (T130), not yet committed; independent review owed (constitution 11.4.142); its row in `docs/scripts/README.md` is owed (that file is being edited by another agent) |
| Source | `scripts/test-infra/seed_corpus.sh`; tests `tests/infra/test_seed_corpus.sh` |

## Purpose

Deterministic corpus seeder of the real-service stack (docs/16 section 10.4): a fixed tree of real file types (png, jpg, gif, mp3, flac, mkv, mp4, pdf, zip, exe, dmg, txt), unicode and
CJK names, one 359-character path and empty directories, built from a recorded seed. Two runs with the same seed give byte-identical files, modes and mtimes, and the same digest.

## Usage

```bash
scripts/test-in-container.sh --out DIR tooling unit -- bash /src/scripts/test-infra/seed_corpus.sh --seed <string> --out /out/corpus --checksum-file /out/corpus.sha256
```

Runs inside IMG-TESTUTIL (python3 3.11) through `TIC tooling unit`; it needs only bash and python3. `--out` must not exist or be empty (exit 2).

## Determinism

Byte streams are sha256 in counter mode over `(seed, entropy, file label)`, never an interpreter-version-dependent generator. Fixed `0644`/`0755` modes and mtime 1700000000. The digest is the
sha256 over the sorted lines `<NFC relpath>\0<dir|file>\0<size>\0<sha256 of content>\n`; stdout prints `corpus_sha256=<hex>` and `files=<n>`; the checksum file holds `<sha256>  corpus seed=<seed>`.
The test recomputes the digest with a second implementation and checks that another seed gives another digest (control needle).

## Exits

0 done; 2 usage / `--out` not empty; 1 failure.
