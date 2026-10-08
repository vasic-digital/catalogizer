# check_exclusions.sh - Companion Guide

| Field | Value |
|---|---|
| Revision | 3 |
| Created | 2026-10-06 |
| Last modified | 2026-10-07T16:57:44Z |
| Status | new in the working tree (T200), not yet committed; independent review owed (constitution 11.4.142, T208); its row in `docs/scripts/README.md` is owed; review round 1 (WF11, 2026-10-06 UTC): fixes I5, I6 applied; independent re-review owed (constitution 11.4.142) |
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

## Review round 1 (WF11, 2026-10-06 UTC): the real conditions, not proxies
With `--root` the gate now checks what 11.4.224(E) says, instead of a denylist of literal globs:
- **matched fraction**: one entry, or all entries together, naming more than half of the root's files is refused (`**/*.go` matched 740 of 742 Go files and passed; `?*/**` passed).
- **class evidence**: every file an entry names must be consistent with its class (generated-code: a generated marker in the first 4 KiB or a generated directory; vendored-third-party: a vendored directory; non-shipping-fixtures: a test/fixture directory or file name).
- **first-party entries are verifiable**: the tracked item must exist in a register export (`--items-file`), else `tracked_item_unverifiable` (a fabricated `ZZZ-999999` returned 0 before); or `measured_by: {app, lane}` must name a row of `scripts/containers/lanes.tsv`. There is no register database yet (T069), so today the only first-party route is `measured_by`.
- **applied exclusions come from the tool's own config** (`coverage/exclusions/tools.yaml` names the config; `scripts/coverage/fence_lib.py` reads vitest `coverage.exclude`, jest negated `collectCoverageFrom`, the jacoco `fileFilter`): an exclusion the tool applies and the fence does not list is UNLISTED, a fence entry the tool does not apply while it would still measure the code it names is LISTED-BUT-NOT-APPLIED, a hand-kept `used/*.txt` that differs from the extraction is stale, an unreadable config is UNVERIFIABLE. For Go and bash the collector applies the fence itself (`gocov_merge.py summary --exclusions`, `bashcov.py`).
- One glob dialect everywhere (`fence_lib.py`): `**` crosses `/`, `*` and `?` do not.
- Effect on the real fences (run with `--root`): catalog-api, catalog-web, catalogizer-android, catalogizer-androidtv, catalogizer-api-client, website, build-scripts PASS; catalogizer-desktop and installer-wizard still FAIL on first-party files the tool config excludes and the fence cannot honestly list without a tracked item (`src/main.tsx`, the config files, `src/vite-env.d.ts`): owed until the register exists. catalog-web's `e2e/fixtures/**` entry was removed (vitest does not apply it) and api-client's `src/**/*.d.ts` and installer-wizard's `src-tauri/**` (`measured_by: installer-wizard/rust`) entries were added.
- Class evidence is by file NAME for test files: `test_*`, `*_test.*`, `*.test.*`, `*.spec.*` or a CamelCase name that ENDS in `Test`/`Tests` (`FooTest.kt`), or a path with a test directory (`test`, `tests`, `androidTest`, `fixtures`, `mocks`, ...). `TestHelper.kt` is not a test file, so the broad Android glob `**/*Test*.*` is refused if it names one in main sources (WF11 m10).
- Options added: `--repo DIR`, `--items-file FILE`, `--tools FILE`, `--lanes FILE` (all optional; the repo root is inferred from `coverage/exclusions/<app>.yaml`).

## Review round 5 (WP-23 fix round, 2026-10-07 UTC): the gate reads what the tool really applies
- **Enumeration**: the file set is the TRACKED files only (`git ls-files` with a clean `GIT_*` environment); a directory walk is used only outside any work tree, and a dangling gitfile or an unreadable directory is a refusal (exit 3), never a smaller denominator.
- **Glob dialect**: the picomatch subset (`*`, `**`, `?`, `[...]` classes, `{a,b}` braces); extglob and ranges are refused (`glob_dialect`) so a pattern is never read two ways.
- **Applied exclusions**: the exclusions a tool really applies are read from its config by a fail-closed JS/Kotlin literal parser (spreads and expressions it cannot evaluate make the applied set `unverifiable`, which FAILs the application: catalog-web and api-client are in that state today, see the real verdicts). The tool identity is checked against the `package.json` script (`tool_in_script`). Test sources are excluded from the denominators (`measurable`).
- **Class evidence**: `generated` needs an anchored generated marker (or a committed generator output directory), `vendored` needs provenance (an upstream record), `fixture` needs a test-source-set match plus an import search that shows nothing in shipping code imports it. `measured_by` must name a different lane, an existing wrapper and an image present in `build/containers/images.lock.yaml` (`measured_by_unproven`).
- **`kind: fence` rows** (build-scripts) declare `scope` and `measured` globs; an in-scope first-party file that is neither measured nor named by an entry is `unmeasured_first_party`.
- **YAML**: strict loader (duplicate keys and non-string keys are refused). Exit codes 0 PASS, 1 FAIL, 2 usage, 3 not processable.
- **Real verdicts with the round-5 gate** (this checkout): PASS catalog-api, website, android, androidtv; FAIL catalog-web and api-client (applied exclusions unverifiable), desktop (implicit scope, unlisted exclusions), installer-wizard (`measured_by_unproven`: IMG-RUST is not in the image lock; plus unlisted exclusions), build-scripts (`unmeasured_first_party`, about 317 files). These are findings about the repository, not defects of the gate; they are owed to the owners of those fences.
- `coverage/exclusions/tools.yaml` rows now carry `root`, `script`, `measurable`, `scope`, `measured`; the api-client row was corrected to `kind: vitest`, `config: catalogizer-api-client/vitest.config.ts`, `script: test`.
