# check_doc_coverage.py - Companion Guide

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-11-03 |
| Status | committed on branch `docs-sync-c4`; independent review owed (constitution 11.4.142 / 11.4.209) |
| Source | `scripts/docs/check_doc_coverage.py`, exemptions `scripts/docs/doc_coverage_exempt.tsv`; test `scripts/docs/tests/test_check_doc_coverage.py` |

## Purpose

Machine-checks that the documentation matches the code, in five checks. Audit evidence: [docs/qa/docs_sync_c4.md](../qa/docs_sync_c4.md).

| Check | What it asserts | Extraction |
|---|---|---|
| `routes` | every HTTP route Gin registers in `catalog-api/main.go` is an operation in `docs/api/openapi.yaml`, and every spec operation is a registered route (`GAP routes UNDOCUMENTED` / `PHANTOM`) | `<var> := <parent>.Group("p")` chains, `<var>.GET/POST/PUT/DELETE/PATCH("p")`, and routes mounted by `Register*Routes(group, ...)` helpers in `catalog-api/handlers/*.go` (the helper's group parameter is bound to the caller's prefix). `/debug/pprof` is excluded (operator-only, documented as a note). Path templates are normalised (`:id`, `*path`, `{id}` all become `{}`). |
| `env` | every environment variable read by non-test Go code under `catalog-api/` is named (whole word) in `docs/deployment/ENVIRONMENT_VARIABLES.md` or `docs/ENV_VARIABLES.md` (`GAP env UNDOCUMENTED_ENV`) | upper-case string literals passed to any call whose name contains `env`/`Getenv`/`LookupEnv`; `Setenv`/`Unsetenv`/`Expand` are ignored |
| `scripts` | every top-level `scripts/<file>` (not `.md`) has `docs/scripts/<name>.md` (hyphen/underscore variants accepted) or an exemption (`GAP scripts NO_SCRIPT_GUIDE`) | `git ls-files` |
| `readme` | `README.md` links `docs/README.md` | regex |
| `claims` | each curated behaviour statement (REGISTRATION_MODE default, smart playlists 501, share controls disabled, converters in the image and in the `/health/deep` probe, go-fitz removed, PDF pages rendered by poppler) is present in its doc AND its code anchor is still present (`CLAIM_NOT_IN_DOC` / `CLAIM_NOT_IN_CODE`) | table `CLAIMS` in the script; add a row when a new behaviour statement is documented |

## Usage

```bash
python3 scripts/docs/check_doc_coverage.py [--root DIR] [--check routes|env|scripts|readme|claims]... [--exempt FILE] [--json]
```

| Exit | Meaning |
|---|---|
| 0 | no gap |
| 1 | one or more gaps (rows `GAP<TAB>check<TAB>class<TAB>item...`) |
| 2 | usage, unreadable input, or an exemption line without a reason |
| 3 | BLIND: a check extracted zero items (never reported as clean) |

## Exemptions

`scripts/docs/doc_coverage_exempt.tsv`: `kind<TAB>name<TAB>reason` with `kind` = `script`, `env` or `claim`. A line without a reason is an error. Exemptions are honest, counted gaps (90 scripts whose guide is owed, 13 env names that are false positives or test-only); prune a line when its guide lands.

## Limits (honest)

The extractors are regex-based, not a Go parser: a route registered through a path built at run time, or a helper whose group parameter is not the first argument, is not seen (the check would then report the spec operation as PHANTOM, never silently pass). `scripts/check_api_route_vs_spec.py` uses go/ast but does not follow `Register*Routes` helpers, so it reports 21 phantom operations on this tree; this checker is the one that follows them. Env detection finds names written as literals; names built by concatenation (`CATALOGIZER_IDENTITY_<N>_*`) are documented as a pattern. The `claims` check proves a phrase exists, not that the prose is correct.

## Tests

`python3 scripts/docs/tests/test_check_doc_coverage.py` (also pytest-compatible): clean fixture passes; a deliberate gap in each check fails with the named row; an empty spec is BLIND (exit 3); an exemption needs a reason; the live repository has 0 gaps.
