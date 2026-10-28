#!/usr/bin/env bash
#
# test_container_hygiene.sh - guard for container/build hygiene of the Catalogizer repo.
#
# Purpose:  asserts (a) every Dockerfile* ships a non-root final USER and a HEALTHCHECK,
#           (b) no Dockerfile masks a failing build (`npm run build || ...`, `--skipLibCheck`),
#           (c) no build script defaults to the retired remote host thinker.local,
#           (d) no `privileged: true` in compose files outside an allowlist with a reason,
#           (e) no script executes plain `docker <verb>`/`docker-compose`/`sudo` in command position
#               (after `$(`, backtick, `;`, `&&`, `||`, `|`, `then`, `do`, ...) (rootless podman only),
#           (f) R7: a non-root builder image requires `--userns=keep-id` in the build orchestrator
#               and no `/root/...` cache mounts in its defaults,
#           (g) R8: a compose healthcheck command (curl) must exist in the final stage of the
#               service's own Dockerfile.
#           Dockerfile, Dockerfile.*, Containerfile and Containerfile.* are all scanned.
#           The scanner is self-tested against bad + clean fixtures and each rule has a paired
#           mutation, so a scanner that cannot FAIL (or that FAILs on clean input) is itself a FAIL.
# Usage:    scripts/tests/test_container_hygiene.sh            # selftest + scan of this repo
#           scripts/tests/test_container_hygiene.sh --scan DIR # scan DIR only, exit 1 on violations
#           scripts/tests/test_container_hygiene.sh --selftest # scanner self-test only
# Inputs:   DIR/scripts/tests/container_hygiene_allowlist.tsv  (rule<TAB>path<TAB>reason), optional;
#           <path> may end in '*' (prefix match); an entry with an empty reason is ignored
# Outputs:  "VIOLATION <rule> <path>: <detail>" lines on stdout; final PASS/FAIL line; exit 0/1
# Side-fx:  none on the repo (selftest works in a mktemp -d sandbox it created itself)
# Cross-ref: CLAUDE.md §11.4.201 (guards assert the real condition), §11.4.161 (rootless),
#            §11.4.173 (containerized builds), §11.4.287 (script hygiene), §1.1 (paired mutation)
# No `pipefail` on purpose: the scanner uses `producer | grep -q` as a boolean; with pipefail grep -q's early exit
# SIGPIPEs the producer (status 141) and the boolean flips nondeterministically (a flaky false R2/R8 FAIL was observed).
set -u

SELF_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DEFAULT="$(cd "$SELF_DIR/../.." && pwd)"

# final_stage FILE : continuation-joined text after the last FROM
final_stage() {
  sed -e ':a' -e '/\\$/{N;s/\\\n/ /;ta' -e '}' "$1" | awk '/^[[:space:]]*FROM[[:space:]]/{buf=""; next} {buf=buf $0 "\n"} END{printf "%s", buf}'
}
# final_user FILE : last USER of the final stage
final_user() {
  final_stage "$1" | awk '/^[[:space:]]*USER[[:space:]]/{u=$2} END{print u}'
}

# scan DIR : prints violations, returns 1 if any.
scan() {
  local root="${1:?scan: root dir required}" n=0 f rel
  local allow="$root/scripts/tests/container_hygiene_allowlist.tsv"

  allowed() { # allowed <rule> <relpath>
    [ -f "$allow" ] || return 1
    awk -F'\t' -v r="$1" -v p="$2" '$1==r && length($3)>0 && ($2==p || (substr($2,length($2))=="*" && index(p, substr($2,1,length($2)-1))==1)) {found=1} END{exit !found}' "$allow"
  }
  report() { # report <rule> <relpath> <detail>
    if allowed "$1" "$2"; then return 0; fi
    echo "VIOLATION $1 $2: $3"; n=$((n+1))
  }

  # --- Dockerfiles (Dockerfile, Dockerfile.*) outside vendored trees -----------------------
  while IFS= read -r f; do
    rel="${f#"$root"/}"
    local final lastuser
    final="$(final_stage "$f")"
    lastuser="$(final_user "$f")"
    case "${lastuser%%:*}" in
      ""|root|0) report R1-USER "$rel" "final stage has no non-root USER (got '${lastuser:-none}')" ;;
    esac
    if ! printf '%s' "$final" | grep -Eq '^[[:space:]]*HEALTHCHECK[[:space:]]+(--[^[:space:]]+[[:space:]]+)*CMD'; then
      report R2-HEALTHCHECK "$rel" "final stage has no HEALTHCHECK CMD"
    fi
    # continuation-joined, so `RUN npm run build \` + newline + `|| true` is seen as one instruction
    if sed -e ':a' -e '/\\$/{N;s/\\\n/ /;ta' -e '}' "$f" \
         | grep -Eq '^[[:space:]]*RUN[[:space:]].*(npm run build[^|]*\|\||--skipLibCheck)'; then
      report R3-MASKED-BUILD "$rel" "build failure masked (npm run build || ... / --skipLibCheck)"
    fi
  done < <(find "$root" \( -path "$root/submodules" -o -path "$root/constitution" -o -name node_modules -o -name .git \) -prune \
             -o -type f \( -name 'Dockerfile' -o -name 'Dockerfile.*' -o -name 'Containerfile' -o -name 'Containerfile.*' \) -print | sort)

  # --- scripts: thinker.local default, plain docker/sudo execution -------------------------
  while IFS= read -r f; do
    rel="${f#"$root"/}"
    if grep -Ev '^[[:space:]]*#' "$f" | grep -Eq ':?-[^}"]*thinker\.local|^[[:space:]]*(export[[:space:]]+|declare[[:space:]]+(-[a-zA-Z]+[[:space:]]+)?)?[A-Za-z_][A-Za-z0-9_]*=["'"'"']?thinker\.local'; then
      report R4-THINKER-DEFAULT "$rel" "defaults to retired host thinker.local"
    fi
    # command position: line start, or after `$(`, backtick, ; & | ( ! and the keywords then/do/else/if/while/until/exec/time/nohup
    local cmdpos='(^|[;&|(`!]|\$\(|(^|[[:space:]])(then|do|else|elif|if|while|until|exec|time|nohup))[[:space:]]*'
    if grep -Ev '^[[:space:]]*(#|echo|log_[a-z]+|printf)' "$f" \
        | grep -Eq "${cmdpos}(sudo[[:space:]]|docker-compose([[:space:]]|\$)|docker[[:space:]]+(run|build|exec|pull|push|rm|stop|start|rmi|network|volume)[[:space:]]|docker[[:space:]]+compose[[:space:]]+([^v[:space:]]|v[^e]))"; then
      report R6-PLAIN-DOCKER-SUDO "$rel" "executes plain docker/docker-compose/sudo (use podman rootless)"
    fi
  done < <(find "$root/scripts" -type f -name '*.sh' -not -path '*/tests/*' 2>/dev/null | sort)

  # --- compose files: privileged (true/yes/on/1, any case, quoted or not) ------------------
  while IFS= read -r f; do
    rel="${f#"$root"/}"
    if grep -Ev '^[[:space:]]*#' "$f" | grep -Eiq "^[[:space:]]*privileged:[[:space:]]*[\"']?(true|yes|on|1)[\"']?[[:space:]]*(#.*)?$"; then
      report R5-PRIVILEGED "$rel" "privileged enabled without allowlist reason"
    fi
    # R8: healthcheck command must exist in the final stage of the service's own Dockerfile
    while IFS='|' read -r svc ctx df; do
      [ -n "$svc" ] || continue
      local dfp="$root/${ctx#./}/${df}"
      [ -f "$dfp" ] || continue
      if ! final_stage "$dfp" | grep -Ev '^[[:space:]]*#' | grep -Eq '(^|[^A-Za-z0-9_.-])curl([^A-Za-z0-9_.-]|$)'; then
        report R8-HEALTHCHECK-TOOL "$rel" "service '$svc' healthcheck uses curl but the final stage of ${dfp#"$root"/} does not install it"
      fi
    done < <(awk '
      /^[[:space:]]*#/ {next}
      /^  [A-Za-z0-9_.-]+:[[:space:]]*$/ { svc=$1; sub(":","",svc); ctx=""; df=""; next }
      svc!="" && /^    build:[[:space:]]*[^[:space:]]/ { v=$2; ctx=v; df="Dockerfile"; next }
      svc!="" && /^      context:/ { ctx=$2 }
      svc!="" && /^      dockerfile:/ { df=$2 }
      svc!="" && /^      test:.*curl/ { if (ctx!="") print svc "|" ctx "|" (df==""?"Dockerfile":df) }
    ' "$f")
  done < <(find "$root" -maxdepth 1 -type f \( -name 'docker-compose*.yml' -o -name 'docker-compose*.yaml' -o -name 'compose*.yml' -o -name 'compose*.yaml' \) | sort)

  # --- R7: non-root builder image needs keep-id in the orchestrator, and no /root cache mounts --
  if [ -f "$root/docker/Dockerfile.builder" ] && [ -f "$root/Build/lib/orchestrator.sh" ]; then
    local bu; bu="$(final_user "$root/docker/Dockerfile.builder")"
    case "${bu%%:*}" in
      ""|root|0) ;;
      *)
        grep -Ev '^[[:space:]]*#' "$root/Build/lib/orchestrator.sh" | grep -q -- '--userns=keep-id' \
          || report R7-BUILDER-USERNS "Build/lib/orchestrator.sh" "builder image runs as '$bu' but the orchestrator lacks --userns=keep-id (bind mount would be root-owned)"
        local cfgs=("$root/Build/lib/orchestrator.sh"); [ -f "$root/scripts/lib/project-config.sh" ] && cfgs+=("$root/scripts/lib/project-config.sh")
        if cat "${cfgs[@]}" | grep -Ev '^[[:space:]]*#' | grep -Eq '[a-z-]+:/root/'; then
          report R7-BUILDER-USERNS "Build/lib/orchestrator.sh" "cache mounts target /root/... but the builder image runs as '$bu'"
        fi ;;
    esac
  fi

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
  # --- reviewer mutants (round 1): each MUST be flagged ---
  expect_rule m1export R4-THINKER-DEFAULT 'printf "export BUILD_HOST=\"thinker.local\"\n" >> "$d/scripts/ok.sh"'
  expect_rule m1decl   R4-THINKER-DEFAULT 'printf "declare -x BUILD_HOST=thinker.local\n" >> "$d/scripts/ok.sh"'
  expect_rule m2subst  R6-PLAIN-DOCKER-SUDO 'printf "ID=\$(docker run -d x)\n" >> "$d/scripts/ok.sh"'
  expect_rule m2tick   R6-PLAIN-DOCKER-SUDO 'printf "ID=\\x60docker run -d x\\x60\n" >> "$d/scripts/ok.sh"'
  expect_rule m3cont   R3-MASKED-BUILD 'printf "RUN npm run build \\\\\n    || true\n" >> "$d/svc/Dockerfile"'
  expect_rule m6ifsudo R6-PLAIN-DOCKER-SUDO 'printf "if sudo true; then echo x; fi\n" >> "$d/scripts/ok.sh"'
  expect_rule m6and    R6-PLAIN-DOCKER-SUDO 'printf "true && docker build .\n" >> "$d/scripts/ok.sh"'
  expect_rule m7cf     R1-USER 'mv "$d/svc/Dockerfile" "$d/svc/Containerfile"; sed -i "/^USER/d" "$d/svc/Containerfile"'
  expect_rule m7cf2    R2-HEALTHCHECK 'mv "$d/svc/Dockerfile" "$d/svc/Containerfile.dev"; sed -i "/^HEALTHCHECK/d" "$d/svc/Containerfile.dev"'
  expect_rule m8dc     R6-PLAIN-DOCKER-SUDO 'printf "docker-compose up -d\n" >> "$d/scripts/ok.sh"'
  expect_rule m9yes    R5-PRIVILEGED 'printf "    privileged: yes\n" >> "$d/docker-compose.yml"'
  expect_rule m9quot   R5-PRIVILEGED 'printf "    privileged: \"true\"\n" >> "$d/docker-compose.yml"'
  expect_rule m9caps   R5-PRIVILEGED 'printf "    privileged: True\n" >> "$d/docker-compose.yml"'
  # R7: non-root builder without keep-id (and with /root cache mounts)
  expect_rule r7nokeep R7-BUILDER-USERNS 'mkdir -p "$d/docker" "$d/Build/lib"; printf "FROM alpine\nUSER builder\nHEALTHCHECK CMD true\n" > "$d/docker/Dockerfile.builder"; printf "podman run --rm x\n" > "$d/Build/lib/orchestrator.sh"'
  expect_rule r7root   R7-BUILDER-USERNS 'mkdir -p "$d/docker" "$d/Build/lib"; printf "FROM alpine\nUSER builder\nHEALTHCHECK CMD true\n" > "$d/docker/Dockerfile.builder"; printf "podman run --userns=keep-id:uid=1000,gid=1000 -v go-cache:/root/go x\n" > "$d/Build/lib/orchestrator.sh"'
  # R8: compose healthcheck curl where the service Dockerfile has no curl
  expect_rule r8curl   R8-HEALTHCHECK-TOOL 'printf "services:\n  api:\n    build:\n      context: .\n      dockerfile: svc/Dockerfile\n    healthcheck:\n      test: [\"CMD\", \"curl\", \"-f\", \"http://x\"]\n" > "$d/docker-compose.yml"'
  # multi-stage: root build stage must not satisfy the final-stage USER rule
  expect_rule multistage R1-USER 'printf "FROM alpine AS b\nUSER app\nFROM alpine\nRUN true\n" > "$d/svc/Dockerfile"'
  # allowlist with reason suppresses; without reason does not
  local d="$sb/allow"; mk_clean "$d"; printf '    privileged: true\n' >> "$d/docker-compose.yml"
  printf 'R5-PRIVILEGED\tdocker-compose.yml\tneeds KVM emulator\n' > "$d/scripts/tests/container_hygiene_allowlist.tsv"
  out="$(scan "$d")"; if [ $? -ne 0 ]; then echo "FAIL selftest[allow]: reasoned allowlist ignored: $out"; rc=1; else echo "ok   selftest[allow]: reasoned allowlist honoured"; fi
  printf 'R5-PRIVILEGED\tdocker-compose.yml\t\n' > "$d/scripts/tests/container_hygiene_allowlist.tsv"
  out="$(scan "$d")"; if [ $? -eq 0 ]; then echo "FAIL selftest[allow-noreason]: empty reason accepted"; rc=1; else echo "ok   selftest[allow-noreason]: empty reason rejected"; fi
  # golden-FALSE controls: correct configurations MUST NOT fire R7/R8/R6
  d="$sb/ctl"; mk_clean "$d"
  mkdir -p "$d/docker" "$d/Build/lib"
  printf 'FROM alpine\nUSER builder\nHEALTHCHECK CMD true\n' > "$d/docker/Dockerfile.builder"
  printf 'podman run --userns=keep-id:uid=1000,gid=1000 -v go-cache:/home/builder/go x\n' > "$d/Build/lib/orchestrator.sh"
  printf 'FROM alpine\nRUN apk add --no-cache curl\nUSER 1001\nHEALTHCHECK CMD curl -f http://x\n' > "$d/svc/Dockerfile"
  printf 'services:\n  api:\n    build:\n      context: .\n      dockerfile: svc/Dockerfile\n    healthcheck:\n      test: ["CMD", "curl", "-f", "http://x"]\n  db:\n    image: x\n    healthcheck:\n      test: ["CMD", "curl", "-f", "http://y"]\n    privileged: false\n' > "$d/docker-compose.yml"
  printf '#!/usr/bin/env bash\npodman run --rm x\nnote="see docker.io/library/foo"\nsudoers_file=1\nmydocker run\n' > "$d/scripts/ok.sh"
  out="$(scan "$d")"; if [ $? -ne 0 ]; then echo "FAIL selftest[ctl]: false positive: $out"; rc=1; else echo "ok   selftest[ctl]: correct builder/healthcheck/podman config not flagged"; fi
  # prefix-glob allowlist entry works, and `docker compose version` (a probe) is not an execution
  d="$sb/glob"; mk_clean "$d"; mkdir -p "$d/b/x"; printf 'FROM alpine\nRUN true\n' > "$d/b/x/Containerfile"
  printf 'if docker compose version >/dev/null 2>&1; then :; fi\n' >> "$d/scripts/ok.sh"
  printf 'R1-USER\tb/*\ttool image, tracked\nR2-HEALTHCHECK\tb/*\ttool image, tracked\n' > "$d/scripts/tests/container_hygiene_allowlist.tsv"
  out="$(scan "$d")"; if [ $? -ne 0 ]; then echo "FAIL selftest[glob]: $out"; rc=1; else echo "ok   selftest[glob]: prefix allowlist honoured, compose-version probe not flagged"; fi
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
