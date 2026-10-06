#!/usr/bin/env bash
# t116-mutate.sh - T116 paired mutations for the D-01..D-04 fixes (and the T110 pins).
# For each fix: the GUARD is the real rootless `podman build` of the exact build the compose file / Dockerfile declares (context and
# dockerfile are PARSED from the compose file, never retyped). The MUTANT is the HEAD (pre-fix) version of the changed file, restored in a
# scratch copy under .audit/scratch/t116 (the tracked files are never touched; no git add/commit/worktree).
# Expected: guard on the fixed tree reaches past the failing COPY step (build_rc 0, or a failure at a LATER step than the mutant's);
# guard on the mutant fails AT the COPY step with a copier "no such file" error.
# Usage: t116-mutate.sh <mutant-name>   (names: d01 d02 d03 d04 pins-from pins-pipe)
set -u
cd "$(git rev-parse --show-toplevel)" || exit 2
SCR=.audit/scratch/t116; mkdir -p "$SCR"
IGN="${T116_IGNOREFILE:?set T116_IGNOREFILE to the ignorefile used for root-context builds}"
compose_build() { # <compose-file-content-path> <service> -> prints "<context> <dockerfile-relative-to-repo-root-or-context>"
  python3 -I - "$1" "$2" <<'PY'
import sys,yaml,os
d=yaml.safe_load(open(sys.argv[1])); b=d['services'][sys.argv[2]]['build']
ctx=b['context']; df=b.get('dockerfile','Dockerfile')
print(os.path.normpath(ctx), os.path.normpath(os.path.join(ctx, df)))
PY
}
guard_build() { # <context> <dockerfile> <tag> : one build; prints build_rc and the failing step line
  local ctx=$1 df=$2 tag=$3 ign=()
  [ "$ctx" = "." ] && ign=(--ignorefile "$IGN")
  podman build --pull=never "${ign[@]}" -t "$tag" -f "$df" "$ctx" > "$SCR/last-build.log" 2>&1; local rc=$?
  echo "build_rc=$rc"; grep -E '^Error: ' "$SCR/last-build.log" | head -n 2 | cut -c1-260
  return $rc
}
case "${1:-}" in
  d01) f=docker-compose.yml; svc=api;;
  d02) f=docker-compose.test.yml; svc=catalog-web;;
  *) f=""; svc="";;
esac
case "${1:-}" in
  d01|d02)
    git show HEAD:$f > "$SCR/$f.mutant.yml"
    echo "## $1 mutant = HEAD $f service $svc"; read -r mctx mdf < <(compose_build "$SCR/$f.mutant.yml" $svc); echo "parsed (mutant): context=$mctx dockerfile=$mdf"
    guard_build "$mctx" "$mdf" "localhost/t116-$1:mutant"; echo "mutant_rc=$?"
    [ "${T116_SKIP_FIXED:-0}" = 1 ] && { echo "## $1 fixed side skipped (T116_SKIP_FIXED=1): the fixed build is the GREEN evidence t111/t113"; exit 0; }
    echo "## $1 fixed = work tree $f service $svc"; read -r gctx gdf < <(compose_build $f $svc); echo "parsed (fixed): context=$gctx dockerfile=$gdf"
    guard_build "$gctx" "$gdf" "localhost/t116-$1:fixed"; echo "fixed_rc=$?";;
  d03)
    git show HEAD:catalog-web/Dockerfile > "$SCR/catalog-web.Dockerfile.mutant"
    echo "## d03 mutant = HEAD catalog-web/Dockerfile, context . (docker-compose.test.yml fixed context)"
    guard_build . "$SCR/catalog-web.Dockerfile.mutant" localhost/t116-d03:mutant; echo "mutant_rc=$?";;
  d04)
    git show HEAD:docker/Dockerfile.builder > "$SCR/Dockerfile.builder.mutant"
    echo "## d04 mutant = HEAD docker/Dockerfile.builder (tarball COPY), context . as docker-compose.build.yml"
    guard_build . "$SCR/Dockerfile.builder.mutant" localhost/t116-d04:mutant; echo "mutant_rc=$?";;
  *) echo "usage: t116-mutate.sh d01|d02|d03|d04" >&2; exit 2;;
esac
