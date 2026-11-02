# check_api_route_vs_spec.py - Companion Guide

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-11-02 |
| Last modified | 2026-11-02T00:00:00Z |
| Status | committed on the scratch vehicle only; independent review owed (constitution 11.4.142 / 11.4.209) |
| Source | `scripts/check_api_route_vs_spec.py`; test `scripts/tests/test_check_api_route_vs_spec.sh` |

## Purpose

The Gin router in `catalog-api/main.go` and `docs/api/openapi.yaml` must describe the same set of operations (method + path). The gate extracts both sets and fails on drift, so an
undocumented route, a phantom documented operation or a method mismatch cannot accumulate silently (the audit found 68 undocumented operations and 2 phantoms).

## Usage

```bash
python3 -I scripts/check_api_route_vs_spec.py [--main FILE] [--spec FILE] [--exclude-prefix P]... [--list]
```

Defaults: `--main catalog-api/main.go`, `--spec docs/api/openapi.yaml`, `--exclude-prefix /debug/pprof` (those 12 operator-only routes exist only with `HELIX_PPROF_ENABLED=true`
and are documented as a note in `info.description`). `--list` prints every extracted operation (the control-needle view).

| Exit | Meaning |
|---|---|
| 0 | in sync |
| 1 | drift: undocumented route, phantom spec operation, or two spec paths that are equivalent templates |
| 2 | usage, unreadable file, or invalid YAML (duplicate mapping keys are rejected) |
| 3 | blind: zero routes or zero spec operations extracted (a zero would be a lie) |

## How it works

Tracks Gin group variables (`v1 := r.Group(...)`), strips `//` comments, normalizes `:x`, `*x` and `{x}` to `{}` (parameter names are ignored), and loads the YAML with a
duplicate-key-detecting loader (the original spec was invalid because of 13 duplicate keys).

## Test

`scripts/tests/test_check_api_route_vs_spec.sh` (20 checks): golden-good, comment carrier, excluded prefix, golden-bad (missing route, phantom, method mismatch), equivalent
templates, blind refusal, duplicate keys, missing file, the real tree, control needles (`POST /api/v1/auth/login`, `GET /health`) and two paired mutations on copies of the real files.

## Honest limits

- Only the Gin router in `main.go` is read. The gorilla/mux `RegisterRoutes` in `internal/handlers/localization_handlers.go` and `media_player_handlers.go` are called from tests only, not wired into the server.
- Compares operations only: no request/response schema or field checking. A schema validity check is separate (a real OpenAPI validator, for example kin-openapi, was used once; it is not part of this gate).
- Routes assembled at run time by means other than the supported group/verb calls are not seen.
- Not wired into any pre-build sweep by this change.
