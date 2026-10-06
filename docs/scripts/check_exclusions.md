# check_exclusions.sh - Companion Guide

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-06 |
| Last modified | 2026-10-06T22:00:00Z |
| Status | new in the working tree (T200), not yet committed; independent review owed (constitution 11.4.142, T208); its row in `docs/scripts/README.md` is owed |
| Source | `scripts/coverage/check_exclusions.sh`, `scripts/coverage/check_exclusions.py`; tests `scripts/coverage/tests/test_check_exclusions.sh`, `scripts/coverage/tests/mutate_exclusions.sh`; data `coverage/exclusions/<app>.yaml`, `coverage/exclusions/used/<app>.txt` |

## Purpose
The exclusion-fence gate of constitution 11.4.224 (E) and docs/05 7.3: a coverage exclusion is legal only through a checked-in list, every entry justified from the closed class set `generated-code | vendored-third-party | non-shipping-fixtures-and-golden-assets`; a `first-party` entry also needs a tracked item id naming the plan to bring it into scope.

## Usage
`check_exclusions.sh FILE [--application NAME] [--used FILE] [--root DIR] [--json OUT]`. `--used` lists the patterns the measuring tool really excludes (extracted from `vitest.config.ts`, the Gradle `fileFilter`, `jest.config.js`); a pattern not listed in FILE is an UNLISTED exclusion and fails. `--root` reports an entry that matches no file as `stale` (a note). Exit: 0 PASS, 1 FAIL (every problem on stderr), 2 usage. The verdict file is `exclusions-check/1`.

## What fails
an unlisted exclusion, an unjustified entry (under 12 characters), a first-party entry without a tracked item (or with an id that is not an item id), an unknown class, an over-broad pattern (`**`), an absolute or `..` path, a duplicate path, a wrong schema, an application that differs from the file stem, an unreadable file, a file with no `exclusions` key (`exclusions: []` states there are none).

## Real verdicts (2026-10-06)
catalog-api, catalog-web, catalogizer-android, catalogizer-androidtv, website, build-scripts PASS; catalogizer-desktop FAIL (`**/*.d.ts`, `**/*.config.*`), installer-wizard FAIL (`**/*.d.ts`, `src/main.tsx`, the four config files), catalogizer-api-client FAIL (`src/**/*.d.ts`): the configs of those tools exclude files outside the fence. These are findings, not edits: a hand-written declaration file has no runtime code (a no-op for the v8 provider), config files and the `src/main.tsx` entry are first-party and need either removal from the config or a tracked item.

## Mutation
`mutate_exclusions.sh` runs, for each application's real fence file, a poisoned copy (a first-party entry without a tracked item) through the real gate (FAIL) and through a gate copy that ignores the tracked-item check (accepts it): recorded per application in `$EV/coverage_baseline/<app>/exclusions-mutation.txt`.
