# route_drift - API drift detector (read-only, heuristic)

`route_drift.py` extracts server routes (gin groups and gorilla/mux subrouters in `catalog-api`), OpenAPI operations
(`docs/api/openapi.yaml`) and client calls (`catalogizer-api-client/src`, `catalog-web/src` axios + fetch, Android and Android TV
Retrofit interfaces) and prints JSON: `undocumented_routes`, `stale_spec_entries`, `unwired_mux_routes`,
`client_calls_without_route` (per client), `double_prefix_calls`, `unresolved`.
Needs Python 3 (PyYAML optional; a line scanner is the fallback).

```
python3 route_drift.py --root . --spec docs/api/openapi.yaml > results/run1.json
python3 route_drift.py --summary          # counts only
python3 route_drift.py --self-test        # 26 deterministic checks on a generated fixture repo
```

Normalisation rules N1-N8 are in the module docstring (method case, OPTIONS/HEAD ignored, every parameter becomes `{}`, gin
catch-all `*p` matches multi-segment client paths, group prefixes concatenated with latest-binding semantics, per-client
base prefixes, wildcard-aware segment matching).

FALSE-POSITIVE CAVEAT (read before acting on any item): this is regex extraction over text, not the running router. Helper-built
routes, loops, reflection and dynamic client URLs are invisible; conditional registrations count as registered; a mux
`RegisterRoutes` is "unwired" only if no linked call exists in non-test code; the OpenAPI file may omit routes on purpose; the
api-client prefix `/api/v1` is an assumption. Each item is a lead for the findings register, not a verdict. The exact method is
a route dump from the running router (`gin.Engine.Routes()`).

Self-test controls: commented-out routes, routes in string literals and in `_test.go`, OPTIONS, pprof, `Map.get('k')`-style calls,
`zap.Any("event")`, dynamic first segment, variable re-binding, unknown receivers all behave as specified.

Results: `results/run1.*`, `results/selftest.txt`.
