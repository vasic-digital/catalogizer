#!/usr/bin/env bash
# Build catqa3-api / catqa3-web images from a CLEAN EXPORT of a commit (default HEAD of the repo this script lives in).
# usage: build.sh [commit]      -> tags localhost/catqa3-{api,web}:main-<short8>
# - git archive (tracked files only) + each referenced submodule archived at its recorded gitlink, into a mktemp -d
#   OUTSIDE the repo; the exact commit is baked into the binary (main.Version/BuildNumber via ldflags) and shows in /health.
# - runs under systemd-run --user --scope with a 6G memory cap, no swap, nice 10 (host safety, constitution 12.6).
# - the catalog-api Dockerfile already installs ffmpeg + imagemagick in the runtime image.
# - DEVIATION from 11.4.173 (see README): this builds locally with `podman build --network host`, not via the containers submodule
#   on a remote build host; --network host weakens build hermeticity. Tracked follow-up, not hidden.
# - the only edit to the exported tree is the ldflags line of catalog-api/Dockerfile (ARG BUILD_COMMIT); nothing in the repo changes.
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"; . "$HERE/lib.sh"
REPO="$(git -C "$HERE" rev-parse --show-toplevel)"
SHA="$(git -C "$REPO" rev-parse "${1:-HEAD}")"; S="${SHA:0:8}"
SUBMODULE_SRC="${SUBMODULE_SRC:-$REPO}"   # checkout that has the submodules initialised (a fresh clone must run `git submodule update` or point this at one)
TMPBASE="${TMPBASE:-$HOME/.cache/claude-tmp}"; mkdir -p "$TMPBASE" "${QA_DIR:?}"
EXP="$(mktemp -d "$TMPBASE/${STACK}-export.XXXXXX")"; : "${EXP:?}"
trap 'rm -rf -- "${EXP:?}"' EXIT
echo "export $SHA -> $EXP"
git -C "$REPO" archive "$SHA" | tar -x -C "$EXP"
# submodules referenced by the two Dockerfiles, taken at the gitlink recorded in $SHA
for p in $(grep -hoE 'COPY submodules/[a-z0-9_]+' "$EXP/catalog-api/Dockerfile" "$EXP/catalog-web/Dockerfile" | awk '{print $2}' | sort -u); do
  sub_sha="$(git -C "$REPO" ls-tree "$SHA" "$p" | awk '{print $3}')"
  [ -n "$sub_sha" ] || { echo "no gitlink for $p in $SHA" >&2; exit 2; }
  mkdir -p "$EXP/$p"; git -C "$SUBMODULE_SRC/$p" archive "$sub_sha" | tar -x -C "$EXP/$p"
done
python3 - "$EXP/catalog-api/Dockerfile" <<'PY'
import sys; p=sys.argv[1]; s=open(p).read()
old='RUN go build -a -ldflags="-w -s" -o catalogizer-api .'
assert old in s, "Dockerfile go build line changed; update build.sh"
open(p,'w').write(s.replace(old,'ARG BUILD_COMMIT=unknown\nRUN go build -a -ldflags="-w -s -X main.Version=${BUILD_COMMIT} -X main.BuildNumber=${BUILD_COMMIT} -X main.BuildDate=stack3" -o catalogizer-api .'))
PY
B=(systemd-run --user --scope -p MemoryMax=6G -p MemorySwapMax=0 nice -n 10 podman build --network host)
cd "$EXP"
"${B[@]}" --build-arg "BUILD_COMMIT=$SHA" -f catalog-api/Dockerfile -t "localhost/${STACK}-api:main-$S" .
"${B[@]}" -f catalog-web/Dockerfile -t "localhost/${STACK}-web:main-$S" .
# Website (VitePress static site) -> $QA_DIR/website_dist ; skipped with WEBSITE=0
if [ "${WEBSITE:-1}" = 1 ]; then
  rm -rf -- "${QA_DIR:?}/website_dist"; mkdir -p "${QA_DIR:?}/website_dist"   # no stale files from an earlier build
  systemd-run --user --scope -p MemoryMax=4G -p MemorySwapMax=0 nice -n 10 podman run --rm --network host \
    -v "$EXP/Website:/w:Z" -v "$QA_DIR/website_dist:/out:Z" docker.io/library/node:20-alpine \
    sh -c 'cp -r /w /b && cd /b && npm ci --no-audit --no-fund && npx vitepress build && cp -r .vitepress/dist/. /out/'
fi
echo "$SHA" > "$QA_DIR/BUILT_COMMIT"
echo "built localhost/${STACK}-api:main-$S and localhost/${STACK}-web:main-$S"
