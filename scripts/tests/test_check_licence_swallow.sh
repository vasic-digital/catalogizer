#!/usr/bin/env bash
# test_check_licence_swallow.sh - tests of scripts/check_licence_swallow.sh (owner decision 2026-10-07 (1)): golden-bad fixtures (every swallow shape), golden-FALSE
# carriers (comment, unrelated `|| true`), a control needle (the instrument sees the files it judges), a REAL-TREE assertion and the paired mutation.
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"; REPO="$(cd "$HERE/../.." && pwd)"; CK="$REPO/scripts/check_licence_swallow.sh"
T="$(mktemp -d "${TMPDIR:-/tmp}/cls.XXXXXX")"; trap 'rm -rf "$T"' EXIT
P=0; F=0
ok() { P=$((P+1)); echo "  ok   $1"; }
no() { F=$((F+1)); echo "  FAIL $1 :: ${2:-}"; }
run() { OUT="$("$CK" --root "$1" 2>&1)"; RC=$?; }
mk() { rm -rf "$T/$1"; mkdir -p "$T/$1/docker"; }
expect() { # name root want-rc
  run "$2"; if [ "$RC" = "$3" ]; then ok "$1"; else no "$1" "rc=$RC want=$3 out=$OUT"; fi; }

mk bad1; printf 'FROM x\nRUN yes | sdkmanager --licenses >/dev/null 2>&1 || true\n' >"$T/bad1/docker/Dockerfile.android"; expect "swallow || true fires" "$T/bad1" 1
mk bad2; printf 'FROM x\nRUN yes | sdkmanager --licenses \\\n  >/dev/null || true\n' >"$T/bad2/docker/Dockerfile"; expect "swallow across a backslash continuation fires" "$T/bad2" 1
mk bad3; printf 'FROM x\nRUN yes | sdkmanager --licenses || :\n' >"$T/bad3/docker/Containerfile"; expect "swallow || : fires" "$T/bad3" 1
mk bad4; printf '#!/usr/bin/env bash\nyes | sdkmanager --licenses >/dev/null 2>&1 || true\n' >"$T/bad4/setup.sh"; expect "shell script swallow fires" "$T/bad4" 1
mk bad5; printf 'services:\n  a:\n    command: sh -c "yes | sdkmanager --licenses || true"\n' >"$T/bad5/docker-compose.x.yml"; expect "compose command swallow fires" "$T/bad5" 1
mk good1; printf 'FROM x\n# RUN yes | sdkmanager --licenses || true  (the old, removed line)\nRUN yes | sdkmanager --licenses >/dev/null\n' >"$T/good1/docker/Dockerfile"; expect "comment carrier and a clean RUN do not fire" "$T/good1" 0
mk good2; printf 'FROM x\nRUN test -f /a || true\nRUN yes | sdkmanager --licenses >/dev/null\n' >"$T/good2/docker/Dockerfile"; expect "unrelated || true does not fire" "$T/good2" 0
mk good3; printf 'FROM x\nRUN yes | sdkmanager --licenses >/dev/null || echo true\n' >"$T/good3/docker/Dockerfile"; expect "\`|| echo true\` is not a swallow of the word true... only || true / || :" "$T/good3" 0
# control needle: the instrument must list the file it is meant to judge
run "$T/bad1"; LST="$("$CK" --root "$T/bad1" --list 2>&1)"; case "$LST" in *docker/Dockerfile.android*) ok "control needle: --list sees the fixture file" ;; *) no "control needle" "$LST" ;; esac
mkdir -p "$T/empty"; expect "empty root is refused (blind instrument), not clean" "$T/empty" 3
expect "missing root is a usage error" "$T/nope" 2
# real tree: the repository itself must be clean (RED before the owner fix, GREEN after)
run "$REPO"; if [ "$RC" = 0 ]; then ok "REAL TREE has no licence-acceptance swallow ($OUT)"; else no "REAL TREE has a licence-acceptance swallow" "$OUT"; fi
RL="$("$CK" --root "$REPO" --list)"; case "$RL" in *docker/Dockerfile.android4*) ok "control needle: real-tree scan sees docker/Dockerfile.android4" ;; *) no "real tree control needle" ;; esac
# paired mutation: re-inject the swallow into a copy of a real Dockerfile; the checker must fail
cp -r "$REPO/docker" "$T/mut-docker" 2>/dev/null; mk mut; cp "$REPO/docker/Dockerfile.android" "$T/mut/docker/Dockerfile.android"
sed -i 's|^RUN yes \| sdkmanager --licenses >/dev/null.*$|RUN yes \| sdkmanager --licenses >/dev/null 2>\&1 \|\| true|' "$T/mut/docker/Dockerfile.android"
expect "MUTATION: swallow re-injected into a copy of the real Dockerfile.android fires" "$T/mut" 1
echo "PASS=$P FAIL=$F"; [ "$F" = 0 ]
