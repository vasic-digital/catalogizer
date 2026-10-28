#!/usr/bin/env bash
#
# test_container_hygiene.sh - guard for container/build hygiene of the Catalogizer repo.
#
# Purpose:  asserts (a) every Dockerfile* ships a non-root final USER and a HEALTHCHECK,
#           (b) no Dockerfile masks a failing build (`npm run build || ...`, `--skipLibCheck`),
#           (c) no build script defaults to the retired remote host thinker.local,
#           (d) no `privileged: true` in compose files outside an allowlist with a reason,
#           (e) no script executes plain `docker <verb>` or `sudo` (rootless podman only).
#           The scanner is self-tested against bad + clean fixtures and each rule has a paired
#           mutation, so a scanner that cannot FAIL (or that FAILs on clean input) is itself a FAIL.
# Usage:    scripts/tests/test_container_hygiene.sh            # selftest + scan of this repo
#           scripts/tests/test_container_hygiene.sh --scan DIR # scan DIR only, exit 1 on violations
#           scripts/tests/test_container_hygiene.sh --selftest # scanner self-test only
# Inputs:   DIR/scripts/tests/container_hygiene_allowlist.tsv  (rule<TAB>path<TAB>reason), optional
# Outputs:  "VIOLATION <rule> <path>: <detail>" lines on stdout; final PASS/FAIL line; exit 0/1
# Side-fx:  none on the repo (selftest works in a mktemp -d sandbox it created itself)
# Cross-ref: CLAUDE.md §11.4.201 (guards assert the real condition), §11.4.161 (rootless),
#            §11.4.173 (containerized builds), §11.4.287 (script hygiene), §1.1 (paired mutation)
set -uo pipefail

SELF_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DEFAULT="$(cd "$SELF_DIR/../.." && pwd)"

# scan DIR : prints violations, returns 1 if any.
scan() {
  local root="${1:?scan: root dir required}" n=0 f rel
  local allow="$root/scripts/tests/container_hygiene_allowlist.tsv"

  allowed() { # allowed <rule> <relpath>
    [ -f "$allow" ] || return 1
    awk -F'\t' -v r="$1" -v p="$2" '$1==r && $2==p && length($3)>0 {found=1} END{exit !found}' "$allow"
  }
  report() { # report <rule> <relpath> <detail>
    if allowed "$1" "$2"; then return 0; fi
    echo "VIOLATION $1 $2: $3"; n=$((n+1))
  }

  # --- Dockerfiles (Dockerfile, Dockerfile.*) outside vendored trees -----------------------
  while IFS= read -r f; do
    rel="${f#"$root"/}"
    # final stage = lines after the last FROM
    local final
    final="$(sed -e ':a' -e '/\\$/{N;s/\\\n/ /;ta' -e '}' "$f" | awk '/^[[:space:]]*FROM[[:space:]]/{buf=""; next} {buf=buf $0 "\n"} END{printf "%s", buf}')"
    local lastuser
    lastuser="$(printf '%s' "$final" | awk '/^[[:space:]]*USER[[:space:]]/{u=$2} END{print u}')"
    case "${lastuser%%:*}" in
      ""|root|0) report R1-USER "$rel" "final stage has no non-root USER (got '${lastuser:-none}')" ;;
    esac
    if ! printf '%s' "$final" | grep -Eq '^[[:space:]]*HEALTHCHECK[[:space:]]+(--[^[:space:]]+[[:space:]]+)*CMD'; then
      report R2-HEALTHCHECK "$rel" "final stage has no HEALTHCHECK CMD"
    fi
    if grep -Eq '^[[:space:]]*RUN[[:space:]].*(npm run build[^|]*\|\||--skipLibCheck)' "$f"; then
      report R3-MASKED-BUILD "$rel" "build failure masked (npm run build || ... / --skipLibCheck)"
    fi
  done < <(find "$root" \( -path "$root/submodules" -o -path "$root/constitution" -o -name node_modules -o -name .git \) -prune \
             -o -type f \( -name 'Dockerfile' -o -name 'Dockerfile.*' \) -print | sort)

  # --- scripts: thinker.local default, plain docker/sudo execution -------------------------
  while IFS= read -r f; do
    rel="${f#"$root"/}"
    if grep -Ev '^[[:space:]]*#' "$f" | grep -Eq ':?-[^}"]*thinker\.local|^[A-Z_]+="?thinker\.local'; then
      report R4-THINKER-DEFAULT "$rel" "defaults to retired host thinker.local"
    fi
    if grep -Ev '^[[:space:]]*(#|echo|log_[a-z]+|printf)' "$f" \
        | grep -Eq '^[[:space:]]*(sudo[[:space:]]|docker[[:space:]]+(run|build|exec|pull|push|rm|stop|start|rmi|network|volume|compose)[[:space:]])'; then
      report R6-PLAIN-DOCKER-SUDO "$rel" "executes plain docker/sudo (use podman rootless)"
    fi
  done < <(find "$root/scripts" -type f -name '*.sh' -not -path '*/tests/*' 2>/dev/null | sort)

  # --- compose files: privileged: true ------------------------------------------------------
  while IFS= read -r f; do
    rel="${f#"$root"/}"
    if grep -Ev '^[[:space:]]*#' "$f" | grep -Eq '^[[:space:]]*privileged:[[:space:]]*true'; then
      report R5-PRIVILEGED "$rel" "privileged: true without allowlist reason"
    fi
  done < <(find "$root" -maxdepth 1 -type f \( -name 'docker-compose*.yml' -o -name 'compose*.yml' \) | sort)

  return $(( n > 0 ))
}

# --- self-test: scanner must FAIL each bad fixture rule and PASS the clean fixture ----------
selftest() {
  local sb rc=0 out
  sb="$(mktemp -d "${TMPDIR:-/tmp}/cat-hygiene.XXXXXX")" || { echo "FAIL selftest: mktemp"; return 1; }
  case "$sb" in /*/cat-hygiene.*) ;; *) echo "FAIL selftest: odd sandbox $sb"; return 1;; esac
  mk_clean() { # mk_clean <dir>
    mkdir -p "$1/scripts/tests" "$1/svc"
    printf 'FROM alpine\nRUN echo hi\nUSER 1001\nHEALTHCHECK --interval=30s CMD true\nCMD ["sh"]\n' > "$1/svc/Dockerfile"
    printf '#!/usr/bin/env bash\n# docker run in a comment is fine\necho "docker run x"\npodman run --rm alpine true\nHOST="${BUILD_HOST:-}"\n' > "$1/scripts/ok.sh"
    printf 'services:\n  a:\n    image: x\n' > "$1/docker-compose.yml"
  }
  expect_rule() { # expect_rule <name> <rule> <mutate-fn-body>
    local d="$sb/$1"; mk_clean "$d"; eval "$3"
    out="$(scan "$d")"; local r=$?
    if [ $r -eq 0 ] || ! printf '%s\n' "$out" | grep -q "VIOLATION $2 "; then
      echo "FAIL selftest[$1]: scanner did not flag $2 (rc=$r) out=[$out]"; rc=1
    else echo "ok   selftest[$1]: $2 flagged"; fi
  }
  mk_clean "$sb/clean"
  out="$(scan "$sb/clean")"; if [ $? -ne 0 ]; then echo "FAIL selftest[clean]: false positive: $out"; rc=1; else echo "ok   selftest[clean]: no violations"; fi
  expect_rule nouser   R1-USER        'sed -i "/^USER/d" "$d/svc/Dockerfile"'
  expect_rule rootuser R1-USER        'sed -i "s/^USER 1001/USER root/" "$d/svc/Dockerfile"'
  expect_rule nohc     R2-HEALTHCHECK 'sed -i "/^HEALTHCHECK/d" "$d/svc/Dockerfile"'
  expect_rule masked   R3-MASKED-BUILD 'printf "RUN npm run build || npm run build -- --skipLibCheck\n" >> "$d/svc/Dockerfile"'
  expect_rule thinker  R4-THINKER-DEFAULT 'printf "BUILD_HOST=\"\${BUILD_HOST:-thinker.local}\"\n" >> "$d/scripts/ok.sh"'
  expect_rule priv     R5-PRIVILEGED  'printf "    privileged: true\n" >> "$d/docker-compose.yml"'
  expect_rule dockerrun R6-PLAIN-DOCKER-SUDO 'printf "docker run -d x\n" >> "$d/scripts/ok.sh"'
  expect_rule sudo     R6-PLAIN-DOCKER-SUDO 'printf "sudo apt-get install x\n" >> "$d/scripts/ok.sh"'
  # multi-stage: root build stage must not satisfy the final-stage USER rule
  expect_rule multistage R1-USER 'printf "FROM alpine AS b\nUSER app\nFROM alpine\nRUN true\n" > "$d/svc/Dockerfile"'
  # allowlist with reason suppresses; without reason does not
  local d="$sb/allow"; mk_clean "$d"; printf '    privileged: true\n' >> "$d/docker-compose.yml"
  printf 'R5-PRIVILEGED\tdocker-compose.yml\tneeds KVM emulator\n' > "$d/scripts/tests/container_hygiene_allowlist.tsv"
  out="$(scan "$d")"; if [ $? -ne 0 ]; then echo "FAIL selftest[allow]: reasoned allowlist ignored: $out"; rc=1; else echo "ok   selftest[allow]: reasoned allowlist honoured"; fi
  printf 'R5-PRIVILEGED\tdocker-compose.yml\t\n' > "$d/scripts/tests/container_hygiene_allowlist.tsv"
  out="$(scan "$d")"; if [ $? -eq 0 ]; then echo "FAIL selftest[allow-noreason]: empty reason accepted"; rc=1; else echo "ok   selftest[allow-noreason]: empty reason rejected"; fi
  # sandbox cleanup: path was created by mktemp above and validated by the case pattern
  rm -rf -- "${sb:?}"
  return $rc
}

case "${1:-}" in
  --selftest) selftest; exit $? ;;
  --scan) out="$(scan "${2:?--scan needs DIR}")"; rc=$?; [ -n "$out" ] && printf '%s\n' "$out"; exit $rc ;;
  "")
    selftest || { echo "FAIL: scanner self-test failed"; exit 1; }
    out="$(scan "$REPO_DEFAULT")"; rc=$?
    [ -n "$out" ] && printf '%s\n' "$out"
    if [ $rc -eq 0 ]; then echo "PASS: container hygiene"; else echo "FAIL: container hygiene violations above"; fi
    exit $rc ;;
  *) echo "usage: $0 [--selftest | --scan DIR]" >&2; exit 2 ;;
esac
