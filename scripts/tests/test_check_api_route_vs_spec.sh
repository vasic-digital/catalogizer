#!/usr/bin/env bash
# test_check_api_route_vs_spec.sh - tests of scripts/check_api_route_vs_spec.py: golden-good and golden-bad fixtures, golden-FALSE carriers (param renames, comments,
# excluded prefixes), control needles, blind-instrument refusal, a REAL-TREE assertion and paired mutations on copies of the real main.go and openapi.yaml.
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"; REPO="$(cd "$HERE/../.." && pwd)"; CK="$REPO/scripts/check_api_route_vs_spec.py"
T="$(mktemp -d "${TMPDIR:-/tmp}/carvs.XXXXXX")"; trap '[ -n "${T:-}" ] && rm -rf "${T:?}"' EXIT
P=0; F=0
ok() { P=$((P+1)); echo "  ok   $1"; }
no() { F=$((F+1)); echo "  FAIL $1 :: ${2:-}"; }
run() { OUT="$(python3 -I "$CK" "$@" 2>&1)"; RC=$?; }
expect() { local name="$1" want="$2"; shift 2; run "$@"; if [ "$RC" = "$want" ]; then ok "$name"; else no "$name" "rc=$RC want=$want out=$OUT"; fi; }

cat >"$T/main.go" <<'GO'
package main

func main() {
	router := gin.Default()
	router.GET("/health", h)
	// router.GET("/commented/out", h)
	router.GET("/debug/pprof/heap", h)
	authGroup := router.Group("/api/v1/auth")
	authGroup.POST("/login", rl, h)
	api := router.Group("/api/v1")
	api.GET("/catalog/*path", h)
	api.GET("/media/:id", h)
	subGroup := api.Group("/subtitles")
	{
		subGroup.GET("", h)
		subGroup.DELETE("/:subtitle_id", func(c *gin.Context) { x.Y(c) })
	}
}
GO
cat >"$T/good.yaml" <<'Y'
openapi: 3.0.3
info: {title: t, version: 1.0.0}
paths:
  /health:
    get: {responses: {'200': {description: ok}}}
  /api/v1/auth/login:
    post: {responses: {'200': {description: ok}}}
  /api/v1/catalog/{path}:
    get: {responses: {'200': {description: ok}}}
  /api/v1/media/{mediaId}:
    get: {responses: {'200': {description: ok}}}
    parameters: [{name: mediaId, in: path, required: true, schema: {type: string}}]
  /api/v1/subtitles:
    get: {responses: {'200': {description: ok}}}
  /api/v1/subtitles/{id}:
    delete: {responses: {'200': {description: ok}}}
Y
EX=(--exclude-prefix /debug/pprof)
expect "golden-good: in sync (param rename, wildcard, group nesting, comment carrier, excluded prefix)" 0 --main "$T/main.go" --spec "$T/good.yaml" "${EX[@]}"
run --main "$T/main.go" --spec "$T/good.yaml" "${EX[@]}" --list; case "$OUT" in *"GET /api/v1/subtitles"*"DELETE /api/v1/subtitles/{}"*) ok "control needle: --list shows nested group routes normalised" ;; *) no "control needle list" "$OUT" ;; esac
case "$OUT" in *commented*) no "comment carrier leaked into the route list" "$OUT" ;; *) ok "comment carrier not extracted" ;; esac
case "$OUT" in *pprof*) no "excluded prefix leaked into the list" "$OUT" ;; *) ok "excluded prefix not listed" ;; esac

# golden-bad 1: a code route with no spec operation
sed 's|^  /api/v1/subtitles/{id}:|  /api/v1/subtitles/{id}/x:|' "$T/good.yaml" >"$T/bad1.yaml"
expect "golden-bad: code route missing from spec and spec op without a route" 1 --main "$T/main.go" --spec "$T/bad1.yaml" "${EX[@]}"
case "$OUT" in *"IN CODE, NOT IN SPEC"*"DELETE /api/v1/subtitles/{}"*) ok "report names the undocumented route" ;; *) no "report text" "$OUT" ;; esac
case "$OUT" in *"IN SPEC, NOT IN CODE"*"/api/v1/subtitles/{}/x"*) ok "report names the phantom spec op" ;; *) no "report text 2" "$OUT" ;; esac
# golden-bad 2: wrong method
sed 's|    delete: {responses|    put: {responses|' "$T/good.yaml" >"$T/bad2.yaml"
expect "golden-bad: method mismatch is drift" 1 --main "$T/main.go" --spec "$T/bad2.yaml" "${EX[@]}"
# pprof not excluded -> drift (the exclusion is what hides it)
expect "without the exclusion the pprof route is drift" 1 --main "$T/main.go" --spec "$T/good.yaml" --exclude-prefix /none
# blind instrument: no routes extracted
printf 'package main\nfunc main(){}\n' >"$T/empty.go"
expect "blind: zero routes extracted is refused, not clean" 3 --main "$T/empty.go" --spec "$T/good.yaml"
printf 'openapi: 3.0.3\ninfo: {title: t, version: 1}\npaths: {}\n' >"$T/emptyspec.yaml"
expect "blind: zero spec operations is refused, not clean" 3 --main "$T/main.go" --spec "$T/emptyspec.yaml"
# duplicate path keys in the spec are invalid YAML semantics -> error
printf 'openapi: 3.0.3\ninfo: {title: t, version: 1}\npaths:\n  /health:\n    get: {responses: {"200": {description: ok}}}\n  /health:\n    post: {responses: {"200": {description: ok}}}\n' >"$T/dup.yaml"
expect "duplicate mapping key in spec is an error (rc 2)" 2 --main "$T/main.go" --spec "$T/dup.yaml"
expect "missing file is a usage error" 2 --main "$T/nope.go" --spec "$T/good.yaml"

# equivalent templated paths (invalid per OpenAPI 3.0.3) are drift even when every method matches
cat >"$T/clash.yaml" <<'Y'
openapi: 3.0.3
info: {title: t, version: 1}
paths:
  /health:
    get: {responses: {'200': {description: ok}}}
  /api/v1/media/{a}:
    get: {responses: {'200': {description: ok}}}
  /api/v1/media/{b}:
    delete: {responses: {'200': {description: ok}}}
Y
expect "equivalent templated paths in the spec fail" 1 --main "$T/main.go" --spec "$T/clash.yaml" "${EX[@]}"
case "$OUT" in *"EQUIVALENT TEMPLATED PATHS"*) ok "report names the equivalent templated paths" ;; *) no "clash text" "$OUT" ;; esac
# real tree
run; if [ "$RC" = 0 ]; then ok "REAL TREE: router and docs/api/openapi.yaml agree ($(echo "$OUT" | tail -1))"; else no "REAL TREE drift" "$(echo "$OUT" | head -20)"; fi
run --list; case "$OUT" in *"GET /api/v1/auth/login"*|*"POST /api/v1/auth/login"*) ok "control needle: real-tree extraction sees POST /api/v1/auth/login" ;; *) no "real tree needle" "$(echo "$OUT"|head)" ;; esac
case "$OUT" in *"GET /health"*) ok "control needle: real-tree extraction sees GET /health" ;; *) no "real tree needle 2" ;; esac
# paired mutations on copies of the real files
cp "$REPO/catalog-api/main.go" "$T/real_main.go"; cp "$REPO/docs/api/openapi.yaml" "$T/real_spec.yaml"
sed 's|api.GET("/media-types"|api.GET("/media-types-mutant"|' "$T/real_main.go" >"$T/mut_main.go"
expect "MUTATION: a renamed real route is detected" 1 --main "$T/mut_main.go" --spec "$T/real_spec.yaml"
python3 -I - "$T/real_spec.yaml" "$T/mut_spec.yaml" <<'PY'
import sys,re
src=open(sys.argv[1]).read()
out=src.replace("  /api/v1/media-types:\n","  /api/v1/media-types-gone:\n",1)
assert out!=src
open(sys.argv[2],"w").write(out)
PY
expect "MUTATION: a spec path renamed away from the router is detected" 1 --main "$T/real_main.go" --spec "$T/mut_spec.yaml"
echo "PASS=$P FAIL=$F"; [ "$F" = 0 ]
