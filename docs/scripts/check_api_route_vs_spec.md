# check_api_route_vs_spec.py - Companion Guide

| Field | Value |
|---|---|
| Revision | 2 |
| Created | 2026-11-02 |
| Last modified | 2026-11-02T12:00:00Z (go/ast extraction replaces the line regex) |
| Status | committed on the scratch vehicle only; independent review owed (constitution 11.4.142 / 11.4.209) |
| Source | `scripts/check_api_route_vs_spec.py`, `scripts/lib/gin_routes_ast.go`; test `scripts/tests/test_check_api_route_vs_spec.sh` |

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

`scripts/lib/gin_routes_ast.go` (Go stdlib `go/ast` only; the gate builds it with `go build`, offline, `GOTOOLCHAIN=local`, so the Go toolchain is required, exit 2 without it) interprets `main()` of
the router file: `gin.Default/New` roots, `X.Group("lit")` prefixes, chained `.Use(...)`, `GET/POST/PUT/DELETE/PATCH/HEAD/OPTIONS`, `Any` (expanded to those 7 methods),
`Handle("METHOD", path, ...)`, `Match([]string{...}, path, ...)`, calls split over several lines, and helper functions declared in the same file or in a sibling `*.go` file of the same package directory (for example `registerWebSocketRoute` in `main_wsroute.go`) with a `*gin.RouterGroup`/`*gin.Engine`/`gin.IRouter`/`gin.IRoutes`
parameter (interpreted at each call site with the caller's prefix). Parameter names are ignored (`:x`, `*x`, `{x}` all become `{}`). The YAML is loaded with a duplicate-key-detecting
loader (the original spec was invalid because of 13 duplicate keys).

A registration that cannot be resolved (non-literal path, `Handle` with a non-literal method, a route-verb call with a literal `/path` on a receiver that is not a known group) makes the gate
exit 3 (blind) instead of passing.

## Test

`scripts/tests/test_check_api_route_vs_spec.sh` (33 checks): golden-good, comment carrier, excluded prefix, golden-bad (missing route, phantom, method mismatch), equivalent
templates, blind refusal, duplicate keys, missing file, the real tree, control needles (`POST /api/v1/auth/login`, `GET /health`) and two paired mutations on copies of the real files, and the AST cases a line regex misses: a multi-line call, Handle("PATCH", ...), Any(...), Match(...), a helper registering on a *gin.RouterGroup (all must be extracted and reported as drift when undocumented), plus the non-literal-path refusal.

## Honest limits

- Only `main()` of the Gin router file is interpreted, plus helpers reached from it that are declared in the same file or in sibling files of the same package directory. Registrations in another package (a helper taking a group) are not seen; none exist today.
- Sibling files are loaded by file name only: `//go:build` constraints and `_GOOS`/`_test` suffixes are NOT evaluated, so a helper declared in a file excluded by a build tag is still followed (and a same-named helper in two tagged files is ambiguous). No such file exists today.
- Groups held in struct fields, slices or maps, or returned from functions, are not tracked; a route verb on such a receiver with a literal `/path` is reported as unresolved (exit 3), not skipped.
- `Any` is expanded to GET, POST, PUT, DELETE, PATCH, HEAD, OPTIONS (Gin also registers CONNECT and TRACE; those are not compared). `Static*`/`StaticFS`/`StaticFile` are not compared.
- The gorilla/mux `RegisterRoutes` in `internal/handlers/localization_handlers.go` and `media_player_handlers.go` are called from tests only, not wired into the server.
- Compares operations only: no request/response schema or field checking. Schema validity is a separate check (a real OpenAPI validator, kin-openapi, was run once offline; it is not part of this gate). A documented response being true at runtime is also not checked by this gate; the formerly `x-currently-broken` path-parameter operations and the `/users/{id}`, `/roles/{id}` permission responses are checked at runtime by `catalog-api/openapi_contract_binary_test.go` (real binary, SQLite).
- Runtime-conditional routes (`/debug/pprof`, enabled by `HELIX_PPROF_ENABLED=true`) are excluded by default.
- Not wired into any pre-build sweep by this change.
