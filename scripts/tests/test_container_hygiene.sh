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
#           (h) R9: no HEAD-style probe (--spider/-I/-X HEAD) of /health (GET-only route),
#           (i) R0: fail closed - nonexistent root, unreadable input, a class of inputs with zero files, or a
#               malformed/over-broad allowlist row are violations. R5 allowlist key is <file>#<service>;
#               path globs are only accepted as a trailing "dir/*".
#           Dockerfile*, Containerfile* anywhere (except submodules/constitution/node_modules/.git), compose files at any depth,
#           and scripts under scripts/ Build/ deploy/ deployment/ docker/ are scanned; compose services are parsed per service.
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
# final_from FILE : image reference of the last FROM
final_from() {
  awk '/^[[:space:]]*FROM[[:space:]]/{f=$2} END{print f}' "$1"
}
nonroot() { case "${1%%:*}" in ""|root|0) return 1;; *) return 0;; esac; }

# compose_records FILE : "service<TAB>kind<TAB>value" records (kinds: service build_ctx build_df image privileged userns
# volume health_test unmodeled); the reader lives in container_hygiene_compose.awk next to this script.
compose_records() {
  awk -f "$SELF_DIR/container_hygiene_compose.awk" "$1"
}

# head_health : stdin = a healthcheck command text; succeeds if it is a HEAD-style probe of /health (GET-only route).
# Flag clusters are parsed (curl -fsI, -sSfI), as are -X HEAD / -XHEAD / --request HEAD / wget --method=HEAD / --spider / --head; list/quote
# punctuation is normalised first so a compose JSON list ["CMD","curl","-fsI",...] is judged on its words.
HEADRE='(^|[[:space:]])(--spider|--head|-[A-Za-z]*I[A-Za-z]*|-[A-Za-z]*X[[:space:]]*HEAD|--request[= ]+HEAD|--method[= ]+HEAD)([[:space:]]|$)'
head_health() {
  tr '[],"\047' '    ' | grep -Eq -- "${HEADRE}.*/health|/health.*${HEADRE}"
}

# masked_build FILE : succeeds if a RUN step of a build command (npm/yarn/pnpm/bun build, go build, make, gradle,
# tsc, cargo build, mvn, vite/ng/next/nuxt/webpack build) has its failure swallowed (`|| x` other than `|| exit N>0`/`|| false`,
# any pipe (no pipefail in sh), `; true`, `; exit 0`, `; :`), or any RUN passes --skipLibCheck.
# Package-manager options between the tool and its build script (`yarn --cwd web build`, `npm --prefix web run build`,
# `yarn workspace web build`, `pnpm --filter web build`); a plain word other than `workspace NAME` is not skipped,
# so `yarn add build-tools` is not a build.
R3_PMOPT='(--?[A-Za-z][A-Za-z0-9-]*([= ][^[:space:]|;&-][^[:space:]|;&]*)?|workspace[[:space:]]+[^[:space:]|;&]+)'
R3_PRE='(env([[:space:]]+-[^[:space:]]+)*[[:space:]]+)?([A-Za-z_][A-Za-z0-9_]*=[^[:space:]]*[[:space:]]+)*((npx|bunx|pnpx)([[:space:]]+-[^[:space:]]+)*[[:space:]]+|(pnpm|yarn)[[:space:]]+(exec[[:space:]]+)?)?'
R3_BUILD='((npm|yarn|pnpm|bun)([[:space:]]+'"$R3_PMOPT"')*([[:space:]]+run)?[[:space:]]+(build|compile)[A-Za-z0-9:_-]*|go[[:space:]]+(build|install)|make|(\./)?gradlew?|tsc|cargo[[:space:]]+build|(\./)?mvnw?|(vite|ng|next|nuxt|webpack)[[:space:]]+build)'
# Masking forms after the build: `||`, a single pipe (no pipefail in sh), any `;` followed by a command (RUN is
# `sh -c` without -e, so the last command's status wins), and a background `&` (not `&&`, not `>&1`).
R3_MASK='(\|\||\|[^|]|;[[:space:]]*[^[:space:]]|[^&>]&([^&]|$))'
# the build starts the RUN body (after --flags), or follows a separator, or opens a `sh -c "..."` string
R3_RE='^[[:space:]]*RUN[[:space:]]+((--[^[:space:]]+[[:space:]]+)*|.*([;&|(]|-[A-Za-z]*c[[:space:]]+["'"'"'])[[:space:]]*)'"$R3_PRE$R3_BUILD"'([[:space:]][^|;]*)?'"$R3_MASK"
# run_lines FILE : one normalised "RUN <body>" line per RUN instruction (continuations joined, instruction keyword
# case-insensitive, --flags kept first). An interpreter heredoc (`RUN <<EOF`, `RUN bash -e <<EOF`) becomes its body
# joined with "; ". A stage SHELL with pipefail / -e is prefixed to each later RUN of that stage as text the masks below
# understand. A heredoc that feeds another command (`cat <<EOF > f`) is consumed and not treated as commands.
run_lines() {
  sed -e ':a' -e '/\\$/{N;s/\\\n/ /;ta' -e '}' "$1" | awk '
    function out(flags, body){ print "RUN " flags shp body }
    hd != "" { t=$0; sub(/^[\t ]+/,"",t)
      if ($0 == hd || (hstrip && t == hd)) { if (hcmd) out(hflags, hpre hbody); hd=""; next }
      if (hcmd) {
        if (t ~ /^#!/) { if (t ~ /[ \t]-[A-Za-z]*e/) hbody = hbody "set -e && " }
        else if (t != "" && t !~ /^#/) hbody = hbody ((hbody == "" || hbody ~ /&& $/) ? "" : "; ") t
      }
      next }
    /^[ \t]*[Ff][Rr][Oo][Mm][ \t]/ { shp=""; next }
    /^[ \t]*[Ss][Hh][Ee][Ll][Ll][ \t]/ { shp=""
      if ($0 ~ /pipefail/) shp = shp "set -o pipefail && "
      if ($0 ~ /"-[A-Za-z]*e[A-Za-z]*"|errexit/) shp = shp "set -e && "
      next }
    /^[ \t]*[Rr][Uu][Nn][ \t]/ {
      b=$0; sub(/^[ \t]*[Rr][Uu][Nn][ \t]+/,"",b); fl=""
      while (match(b, /^--[^ \t]+[ \t]+/)) { fl = fl substr(b, 1, RLENGTH); b = substr(b, RLENGTH+1) }
      if (match(b, /<<-?[ ]*["\047]?[A-Za-z_][A-Za-z0-9_]*["\047]?/)) {
        h=substr(b, RSTART, RLENGTH); hstrip=(h ~ /^<<-/); gsub(/^<<-?[ ]*|["\047]/, "", h); hd=h
        hpre=substr(b, 1, RSTART-1); hflags=fl; hbody=""
        hcmd = (hpre ~ /^[ \t]*((ba|da)?sh([ \t]+-[A-Za-z]+)*[ \t]*)?$/)
        next }
      out(fl, b) }'
}
# masked_build FILE : succeeds if a RUN step of a build command has its failure swallowed, or any RUN passes --skipLibCheck.
# Under errexit (`set -e`, `sh -e`) a `;` is read as `&&`; under pipefail a single pipe is read as `&&`. `|| exit N`
# (N in 1..255; 256 wraps to 0) and `|| false` fail loud and are removed, keeping the delimiter that follows them so
# a masking tail (`|| false; true`, `|| false& true`) is still seen.
masked_build() {
  run_lines "$1" | sed -E 's/([^&>[:space:]])&([^&]|$)/\1 \&\2/g' \
    | sed -E '/(set[[:space:]]+-[A-Za-z]*e|errexit|(^|[[:space:]/])(ba|da)?sh[[:space:]]+-[A-Za-z]*e)/ s/;/ \&\& /g' \
    | sed -E -e '/pipefail/ {' -e ':p' -e 's/(^|[^|])\|([^|]|$)/\1 \&\& \2/' -e 'tp' -e '}' \
    | sed -E "s/\|\|[[:space:]]*(exit[[:space:]]+(25[0-5]|2[0-4][0-9]|1[0-9][0-9]|[1-9][0-9]?)|false)([[:space:];&)\"'\`]|\$)/ \3/g" \
    | grep -Eq -e "$R3_RE" -e '^[[:space:]]*RUN[[:space:]].*--skipLibCheck'
}

# scan DIR : prints violations, returns 1 if any. Fails CLOSED: missing root, unreadable input, or an empty class of
# inputs (nothing to scan = blind instrument, §11.4.273) are violations, never a silent pass.
scan() {
  local root="${1:?scan: root dir required}" n=0 f rel
  local allow="$root/scripts/tests/container_hygiene_allowlist.tsv"

  if [ ! -d "$root" ]; then echo "VIOLATION R0-BLIND $root: scan root does not exist"; return 1; fi

  # allowlist rows: rule<TAB>path<TAB>reason. path = exact repo path (compose rules: path#service) or a "dir/*" prefix.
  allowed() { # allowed <rule> <relpath>
    [ -r "$allow" ] || return 1
    awk -F'\t' -v r="$1" -v p="$2" '
      function glob_ok(x) { return length(x) > 2 && substr(x, length(x)-1) == "/*" && index(substr(x,1,length(x)-1), "*") == 0 }
      $1==r && length($3)>0 && ($2==p || (glob_ok($2) && index(p, substr($2,1,length($2)-1))==1)) {found=1}
      END{exit !found}' "$allow"
  }
  report() { # report <rule> <relpath> <detail>
    if allowed "$1" "$2"; then return 0; fi
    echo "VIOLATION $1 $2: $3"; n=$((n+1))
  }
  # a malformed allowlist row is itself a violation (no bare "*", no glob except a trailing "/*", rule and reason present)
  if [ -e "$allow" ]; then
    if [ ! -r "$allow" ]; then echo "VIOLATION R0-UNREADABLE scripts/tests/container_hygiene_allowlist.tsv: allowlist not readable"; n=$((n+1))
    else
      local bad
      bad="$(awk -F'\t' 'NF==0||/^[[:space:]]*$/{next} $1=="" || $1 ~ /\*/ || $2=="" || ($2 ~ /\*/ && !(length($2)>2 && substr($2,length($2)-1)=="/*" && index(substr($2,1,length($2)-1),"*")==0)) {printf "line %d (%s %s); ", NR, $1, $2}' "$allow")"
      [ -z "$bad" ] || { echo "VIOLATION R0-ALLOWLIST scripts/tests/container_hygiene_allowlist.tsv: malformed or over-broad rows: $bad"; n=$((n+1)); }
    fi
  fi

  local -a PRUNE=( \( -path "$root/submodules" -o -path "$root/constitution" -o -name node_modules -o -name .git \) -prune -o )
  local out st dfs composes scripts d
  local -a sdirs=()
  for d in scripts Build deploy deployment docker; do [ -d "$root/$d" ] && sdirs+=("$root/$d"); done

  dfs="$(find "$root" "${PRUNE[@]}" -type f \( -name 'Dockerfile' -o -name 'Dockerfile.*' -o -name 'Containerfile' -o -name 'Containerfile.*' \) -print 2>&1 | sort)"
  composes="$(find "$root" "${PRUNE[@]}" -type f \( -name 'docker-compose*.yml' -o -name 'docker-compose*.yaml' -o -name 'compose*.yml' -o -name 'compose*.yaml' \) -print 2>&1 | sort)"
  scripts=""
  [ ${#sdirs[@]} -gt 0 ] && scripts="$(find "${sdirs[@]}" -type f -name '*.sh' -not -path '*/tests/*' -print 2>&1 | sort)"
  # find diagnostics (unreadable directories) end up in the lists: surface them
  local lst
  for lst in "$dfs" "$composes" "$scripts"; do
    if printf '%s\n' "$lst" | grep -q '^find: '; then
      echo "VIOLATION R0-UNREADABLE $root: find could not read part of the tree: $(printf '%s\n' "$lst" | grep '^find: ' | head -1)"; n=$((n+1))
    fi
  done
  dfs="$(printf '%s\n' "$dfs" | grep -v '^find: ' | grep -v '^$')"
  composes="$(printf '%s\n' "$composes" | grep -v '^find: ' | grep -v '^$')"
  scripts="$(printf '%s\n' "$scripts" | grep -v '^find: ' | grep -v '^$')"
  [ -n "$dfs" ]      || { echo "VIOLATION R0-BLIND $root: no Dockerfile/Containerfile found (blind scan)"; n=$((n+1)); }
  [ -n "$composes" ] || { echo "VIOLATION R0-BLIND $root: no compose file found (blind scan)"; n=$((n+1)); }
  [ -n "$scripts" ]  || { echo "VIOLATION R0-BLIND $root: no shell script found (blind scan)"; n=$((n+1)); }

  # --- Dockerfiles / Containerfiles ----------------------------------------------------------
  while IFS= read -r f; do
    [ -n "$f" ] || continue
    rel="${f#"$root"/}"
    if [ ! -r "$f" ]; then report R0-UNREADABLE "$rel" "file not readable"; continue; fi
    local final lastuser
    final="$(final_stage "$f")"
    lastuser="$(final_user "$f")"
    nonroot "$lastuser" || report R1-USER "$rel" "final stage has no non-root USER (got '${lastuser:-none}')"
    if ! printf '%s' "$final" | grep -Eq '^[[:space:]]*HEALTHCHECK[[:space:]]+(--[^[:space:]]+[[:space:]]+)*CMD'; then
      report R2-HEALTHCHECK "$rel" "final stage has no HEALTHCHECK CMD"
    fi
    # R3: continuation-joined; the build-command class with a swallowed failure (see masked_build)
    if masked_build "$f"; then
      report R3-MASKED-BUILD "$rel" "build failure masked (<build cmd> || x / | tee / ; true / ; exit 0 / --skipLibCheck)"
    fi
    # R9: a HEAD-style probe of /health (the route is GET-only: HEAD answers 404)
    if printf '%s' "$final" | grep -E '^[[:space:]]*HEALTHCHECK' | head_health; then
      report R9-HEALTH-HEAD "$rel" "healthcheck uses a HEAD request against /health (GET-only route answers 404)"
    fi
  done <<< "$dfs"

  # --- scripts: thinker.local default, plain docker/sudo execution -------------------------
  while IFS= read -r f; do
    [ -n "$f" ] || continue
    rel="${f#"$root"/}"
    if [ ! -r "$f" ]; then report R0-UNREADABLE "$rel" "file not readable"; continue; fi
    if grep -Ev '^[[:space:]]*#' "$f" | grep -Eq ':?-[^}"]*thinker\.local|^[[:space:]]*(export[[:space:]]+|declare[[:space:]]+(-[a-zA-Z]+[[:space:]]+)?)?[A-Za-z_][A-Za-z0-9_]*=["'"'"']?thinker\.local'; then
      report R4-THINKER-DEFAULT "$rel" "defaults to retired host thinker.local"
    fi
    # command position: line start, or after `$(`, backtick, ; & | ( ! and the keywords then/do/else/if/while/until/exec/time/nohup
    local cmdpos='(^|[;&|(`!]|\$\(|(^|[[:space:]])(then|do|else|elif|if|while|until|exec|time|nohup))[[:space:]]*'
    if grep -Ev '^[[:space:]]*(#|echo|log_[a-z]+|printf)' "$f" \
        | grep -Eq "${cmdpos}(sudo[[:space:]]|docker-compose([[:space:]]|\$)|docker[[:space:]]+(run|build|exec|pull|push|rm|stop|start|rmi|network|volume)[[:space:]]|docker[[:space:]]+compose[[:space:]]+([^v[:space:]]|v[^e]))"; then
      report R6-PLAIN-DOCKER-SUDO "$rel" "executes plain docker/docker-compose/sudo (use podman rootless)"
    fi
  done <<< "$scripts"

  # --- compose files (any depth) --------------------------------------------------------------
  local -A ctx df img priv userns tst vols
  local -a umod
  local svc kind value svcs base dfp fu tool words t
  while IFS= read -r f; do
    [ -n "$f" ] || continue
    rel="${f#"$root"/}"
    if [ ! -r "$f" ]; then report R0-UNREADABLE "$rel" "file not readable"; continue; fi
    base="$(dirname "$f")"
    ctx=(); df=(); img=(); priv=(); userns=(); tst=(); vols=(); svcs=(); umod=()
    while IFS=$'\t' read -r svc kind value; do
      case "$kind" in
        service) svcs+=("$svc") ;;
        build_ctx) ctx[$svc]="$value" ;;
        build_df) df[$svc]="$value" ;;
        image) img[$svc]="$value" ;;
        privileged) priv[$svc]="$value" ;;
        userns) userns[$svc]="$value" ;;
        volume) vols[$svc]+="$value"$'\n' ;;
        health_test) tst[$svc]="$value" ;;
        unmodeled) umod+=("$svc: $value") ;;
      esac
    done < <(compose_records "$f")
    # R10: fail closed on compose syntax the parser cannot model (anchors/merge keys, flow bodies with build/volumes, odd entries)
    for t in "${umod[@]}"; do report R10-COMPOSE-UNMODELED "$rel" "$t"; done
    for svc in "${svcs[@]}"; do
      # R5: privileged, allowlisted per (file, service)
      if printf '%s\n' "${priv[$svc]:-}" | grep -Eiq '^(true|yes|on|1)$'; then
        report R5-PRIVILEGED "$rel#$svc" "privileged enabled without allowlist reason"
      fi
      # R9: HEAD probe of /health
      t="${tst[$svc]:-}"
      if printf '%s' "$t" | head_health; then
        report R9-HEALTH-HEAD "$rel#$svc" "healthcheck uses a HEAD request against /health (GET-only route answers 404)"
      fi
      # the service's own Dockerfile (only for services built from this repo)
      dfp=""; fu=""
      if [ -n "${ctx[$svc]:-}" ]; then
        dfp="$(realpath -m "$base/${ctx[$svc]}/${df[$svc]:-Dockerfile}" 2>/dev/null)"
        [ -f "$dfp" ] || dfp=""
      fi
      [ -n "$dfp" ] && fu="$(final_user "$dfp")"
      # R7 (compose): non-root image + rw bind mount of a host path, without podman keep-id => files not writable
      if [ -n "$dfp" ] && nonroot "$fu" && ! printf '%s' "${userns[$svc]:-}" | grep -q 'keep-id'; then
        local v src opts first
        while IFS= read -r v; do
          [ -n "$v" ] || continue
          first="${v:0:1}"
          src="$(printf '%s' "$v" | sed 's/\${[^}]*}/V/g' | cut -d: -f1)"
          opts="$(printf '%s' "$v" | sed 's/\${[^}]*}/V/g' | cut -d: -f3-)"
          case "$first$src" in
            .*|/*|\~*|\$*) printf '%s' ",$opts," | grep -q ',ro,' || report R7-BUILDER-USERNS "$rel#$svc" "rw bind mount '$v' into an image whose USER is '$fu' without userns_mode keep-id (host files not writable by the container user)" ;;
          esac
        done <<< "${vols[$svc]:-}"
      fi
      # R8: the tool the healthcheck invokes must exist in the final stage of the service's own Dockerfile
      if [ -n "$dfp" ] && [ -n "$t" ]; then
        words="$(printf '%s' "$t" | tr -d '[]",'"'" | sed -E 's/^[[:space:]]*(CMD-SHELL|CMD)[[:space:]]+//')"
        tool="$(printf '%s' "$words" | awk '{print $1}')"
        case "$tool" in
          curl|wget|nc|ncat)
            if ! final_stage "$dfp" | grep -Ev '^[[:space:]]*#' | grep -Eq "(^|[^A-Za-z0-9_.-])$tool([^A-Za-z0-9_.-]|\$)"; then
              if ! { [ "$tool" = wget ] || [ "$tool" = nc ]; } || ! final_from "$dfp" | grep -Eiq 'alpine|busybox'; then
                report R8-HEALTHCHECK-TOOL "$rel#$svc" "healthcheck invokes '$tool' but the final stage of ${dfp#"$root"/} does not install it"
              fi
            fi ;;
        esac
      fi
    done
  done <<< "$composes"

  # --- R7: non-root builder image needs keep-id in the orchestrator, and no /root cache mounts --
  if [ -f "$root/docker/Dockerfile.builder" ] && [ -f "$root/Build/lib/orchestrator.sh" ]; then
    local bu; bu="$(final_user "$root/docker/Dockerfile.builder")"
    if nonroot "$bu"; then
      grep -Ev '^[[:space:]]*#' "$root/Build/lib/orchestrator.sh" | grep -q -- '--userns=keep-id' \
        || report R7-BUILDER-USERNS "Build/lib/orchestrator.sh" "builder image runs as '$bu' but the orchestrator lacks --userns=keep-id (bind mount would be root-owned)"
      local cfgs=("$root/Build/lib/orchestrator.sh"); [ -f "$root/scripts/lib/project-config.sh" ] && cfgs+=("$root/scripts/lib/project-config.sh")
      if cat "${cfgs[@]}" | grep -Ev '^[[:space:]]*#' | grep -Eq '[a-z-]+:/root/'; then
        report R7-BUILDER-USERNS "Build/lib/orchestrator.sh" "cache mounts target /root/... but the builder image runs as '$bu'"
      fi
    fi
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
  printf 'R5-PRIVILEGED\tdocker-compose.yml#a\tneeds KVM emulator\n' > "$d/scripts/tests/container_hygiene_allowlist.tsv"
  out="$(scan "$d")"; if [ $? -ne 0 ]; then echo "FAIL selftest[allow]: reasoned allowlist ignored: $out"; rc=1; else echo "ok   selftest[allow]: reasoned allowlist honoured"; fi
  printf 'R5-PRIVILEGED\tdocker-compose.yml#a\t\n' > "$d/scripts/tests/container_hygiene_allowlist.tsv"
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

  # --- round 2/3 reviewer survivors + fail-closed + keyed allowlist ---
  expect_rule tee      R3-MASKED-BUILD 'printf "RUN npm run build 2>&1 | tee /tmp/b.log\n" >> "$d/svc/Dockerfile"'
  expect_rule exit0    R3-MASKED-BUILD 'printf "RUN npm run build; exit 0\n" >> "$d/svc/Dockerfile"'
  expect_rule semitrue R3-MASKED-BUILD 'printf "RUN npm run build ; true\n" >> "$d/svc/Dockerfile"'
  expect_rule blocklist R8-HEALTHCHECK-TOOL 'printf "services:\n  api:\n    build:\n      context: .\n      dockerfile: svc/Dockerfile\n    healthcheck:\n      test:\n        - CMD\n        - curl\n        - -f\n        - http://x\n" > "$d/docker-compose.yml"'
  expect_rule shelltool R8-HEALTHCHECK-TOOL 'printf "services:\n  api:\n    build: ./svc\n    healthcheck:\n      test: [\"CMD-SHELL\", \"wget -qO- http://x\"]\n" > "$d/docker-compose.yml"; printf "FROM debian\nUSER 1001\nHEALTHCHECK CMD true\n" > "$d/svc/Dockerfile"'
  expect_rule deploydir R5-PRIVILEGED 'mkdir -p "$d/deploy/x"; printf "services:\n  s:\n    image: x\n    privileged: true\n" > "$d/deploy/x/docker-compose.yml"'
  expect_rule spider   R9-HEALTH-HEAD 'printf "services:\n  api:\n    image: x\n    healthcheck:\n      test: [\"CMD\", \"wget\", \"--spider\", \"http://127.0.0.1:8080/health\"]\n" > "$d/docker-compose.yml"'
  expect_rule spiderdf R9-HEALTH-HEAD 'sed -i "s#^HEALTHCHECK.*#HEALTHCHECK CMD wget --spider http://127.0.0.1:8080/health#" "$d/svc/Dockerfile"'
  expect_rule composeuserns R7-BUILDER-USERNS 'printf "services:\n  api:\n    build:\n      context: .\n      dockerfile: svc/Dockerfile\n    volumes:\n      - ./svc:/app\n" > "$d/docker-compose.yml"'
  expect_rule blind    R0-BLIND 'rm -f "$d/docker-compose.yml"'
  expect_rule blind2   R0-BLIND 'rm -f "$d/scripts/ok.sh"'
  expect_rule badglob  R0-ALLOWLIST 'printf "R1-USER\t*\tx\n" > "$d/scripts/tests/container_hygiene_allowlist.tsv"'
  expect_rule badglob2 R0-ALLOWLIST 'printf "R1-USER\tsvc*\tx\n" > "$d/scripts/tests/container_hygiene_allowlist.tsv"'
  # bare "*" row must NOT suppress a real violation
  expect_rule starrow  R1-USER 'sed -i "/^USER/d" "$d/svc/Dockerfile"; printf "R1-USER\t*\tx\n" > "$d/scripts/tests/container_hygiene_allowlist.tsv"'
  # new root Containerfile under a dir whose siblings are allowlisted explicitly is still flagged
  expect_rule newsvc   R1-USER 'mkdir -p "$d/build/containers/old" "$d/build/containers/newsvc"; printf "FROM alpine\nHEALTHCHECK CMD true\n" > "$d/build/containers/old/Containerfile"; printf "FROM alpine\nHEALTHCHECK CMD true\n" > "$d/build/containers/newsvc/Containerfile"; printf "R1-USER\tbuild/containers/old/Containerfile\texplicit\n" > "$d/scripts/tests/container_hygiene_allowlist.tsv"'
  out="$(scan "$sb/does-not-exist")"; r=$?
  if [ $r -eq 0 ] || ! printf '%s\n' "$out" | grep -q "R0-BLIND"; then echo "FAIL selftest[noroot]: nonexistent root not failed (rc=$r) out=[$out]"; rc=1; else echo "ok   selftest[noroot]: nonexistent scan root fails"; fi
  if [ "$(id -u)" -ne 0 ]; then
    d="$sb/unread1"; mk_clean "$d"; chmod 000 "$d/docker-compose.yml"
    out="$(scan "$d")"; r=$?; chmod 600 "$d/docker-compose.yml"
    if [ $r -eq 0 ] || ! printf '%s\n' "$out" | grep -q "R0-UNREADABLE"; then echo "FAIL selftest[unreadable-compose]: rc=$r out=[$out]"; rc=1; else echo "ok   selftest[unreadable-compose]: flagged"; fi
    d="$sb/unread2"; mk_clean "$d"; chmod 000 "$d/scripts/ok.sh"
    out="$(scan "$d")"; r=$?; chmod 600 "$d/scripts/ok.sh"
    if [ $r -eq 0 ] || ! printf '%s\n' "$out" | grep -q "R0-UNREADABLE"; then echo "FAIL selftest[unreadable-script]: rc=$r out=[$out]"; rc=1; else echo "ok   selftest[unreadable-script]: flagged"; fi
  else echo "skip selftest[unreadable-*]: running as root"; fi
  # allowlist keyed by (file, service): a second privileged service is still flagged; the old file-only key no longer suppresses
  d="$sb/keyed"; mk_clean "$d"; printf '  b:\n    image: y\n    privileged: true\n' >> "$d/docker-compose.yml"
  sed -i 's/^    image: x$/    image: x\n    privileged: true/' "$d/docker-compose.yml"
  printf 'R5-PRIVILEGED\tdocker-compose.yml#a\tkvm\n' > "$d/scripts/tests/container_hygiene_allowlist.tsv"
  out="$(scan "$d")"; r=$?
  if [ $r -eq 0 ] || ! printf '%s\n' "$out" | grep -q "R5-PRIVILEGED docker-compose.yml#b" || printf '%s\n' "$out" | grep -q "docker-compose.yml#a"; then echo "FAIL selftest[keyed]: out=[$out]"; rc=1; else echo "ok   selftest[keyed]: service b flagged, allowlisted service a not"; fi
  printf 'R5-PRIVILEGED\tdocker-compose.yml\tkvm\n' > "$d/scripts/tests/container_hygiene_allowlist.tsv"
  out="$(scan "$d")"; r=$?
  if [ $r -eq 0 ]; then echo "FAIL selftest[keyed-oldkey]: file-only key suppressed a service"; rc=1; else echo "ok   selftest[keyed-oldkey]: file-only key does not suppress"; fi
  # golden-FALSE: keep-id present, ro mount, named volume, tool installed, GET probe, dir/* glob valid
  d="$sb/ctl2"; mk_clean "$d"; mkdir -p "$d/build/c/x"
  printf 'FROM alpine\nRUN apk add curl\nUSER 1001\nHEALTHCHECK CMD curl -f http://127.0.0.1/health\n' > "$d/svc/Dockerfile"
  printf 'FROM alpine\nHEALTHCHECK CMD true\n' > "$d/build/c/x/Containerfile"
  printf 'services:\n  api:\n    build:\n      context: .\n      dockerfile: svc/Dockerfile\n    userns_mode: "keep-id:uid=1000,gid=1000"\n    volumes:\n      - ./svc:/app\n    healthcheck:\n      test: ["CMD", "curl", "-f", "http://127.0.0.1/health"]\n  w:\n    build: ./svc\n    volumes:\n      - ./svc:/app:ro\n      - named:/data\n      - ${SSL:-./ssl}:/ssl:ro\n    healthcheck:\n      test:\n        - CMD\n        - curl\n        - -f\n        - http://127.0.0.1/health\n' > "$d/docker-compose.yml"
  printf 'R1-USER\tbuild/c/*\tglob ok\n' > "$d/scripts/tests/container_hygiene_allowlist.tsv"
  out="$(scan "$d")"; if [ $? -ne 0 ]; then echo "FAIL selftest[ctl2]: false positive: $out"; rc=1; else echo "ok   selftest[ctl2]: keep-id/ro/named/tool-present/GET/dir-glob not flagged"; fi
  # --- O1-O4 (follow-up closure): RED fixtures first, then the fixes ---
  local CV='services:\n  api:\n    build:\n      context: .\n      dockerfile: svc/Dockerfile\n    volumes:\n'
  # O1 (R7): long-form / flow-form bind volumes behave like the short form
  expect_rule o1long   R7-BUILDER-USERNS 'printf "'"$CV"'      - type: bind\n        source: ./svc\n        target: /app\n" > "$d/docker-compose.yml"'
  expect_rule o1order  R7-BUILDER-USERNS 'printf "'"$CV"'      - target: /app\n        source: ./svc\n        type: bind\n" > "$d/docker-compose.yml"'
  expect_rule o1quoted R7-BUILDER-USERNS 'printf "'"$CV"'      - type: \"bind\"\n        source: \"${HOSTDIR:-./svc}\"\n        target: /app\n        read_only: false\n" > "$d/docker-compose.yml"'
  expect_rule o1flow   R7-BUILDER-USERNS 'printf "'"$CV"'      - {type: bind, source: ./svc, target: /app}\n" > "$d/docker-compose.yml"'
  expect_rule o1second R7-BUILDER-USERNS 'printf "'"$CV"'      - named:/data\n      - type: bind\n        source: /srv/x\n        target: /app\n    healthcheck:\n      test: [\"CMD\", \"true\"]\n" > "$d/docker-compose.yml"'
  # O2 (R9): combined short flags and --request forms
  expect_rule o2cluster R9-HEALTH-HEAD 'printf "services:\n  api:\n    image: x\n    healthcheck:\n      test: [\"CMD\", \"curl\", \"-fsI\", \"http://127.0.0.1:8080/health\"]\n" > "$d/docker-compose.yml"'
  expect_rule o2cluster2 R9-HEALTH-HEAD 'printf "services:\n  api:\n    image: x\n    healthcheck:\n      test: [\"CMD-SHELL\", \"curl -sSfI http://127.0.0.1:8080/health\"]\n" > "$d/docker-compose.yml"'
  expect_rule o2blocklist R9-HEALTH-HEAD 'printf "services:\n  api:\n    image: x\n    healthcheck:\n      test:\n        - CMD\n        - curl\n        - -fsI\n        - http://127.0.0.1/health\n" > "$d/docker-compose.yml"'
  expect_rule o2df     R9-HEALTH-HEAD 'sed -i "s#^HEALTHCHECK.*#HEALTHCHECK CMD curl -fsI http://127.0.0.1:8080/health#" "$d/svc/Dockerfile"'
  expect_rule o2req    R9-HEALTH-HEAD 'sed -i "s#^HEALTHCHECK.*#HEALTHCHECK CMD curl -fs --request HEAD http://127.0.0.1:8080/health#" "$d/svc/Dockerfile"'
  expect_rule o2xhead  R9-HEALTH-HEAD 'sed -i "s#^HEALTHCHECK.*#HEALTHCHECK CMD curl -fsXHEAD http://127.0.0.1:8080/health#" "$d/svc/Dockerfile"'
  expect_rule o2after  R9-HEALTH-HEAD 'sed -i "s#^HEALTHCHECK.*#HEALTHCHECK CMD curl http://127.0.0.1:8080/health -fsI#" "$d/svc/Dockerfile"'
  # O3 (R3): the build-command class, not only `npm run build`
  expect_rule o3yarn   R3-MASKED-BUILD 'printf "RUN yarn build || true\n" >> "$d/svc/Dockerfile"'
  expect_rule o3pnpm   R3-MASKED-BUILD 'printf "RUN pnpm run build ; true\n" >> "$d/svc/Dockerfile"'
  expect_rule o3gob    R3-MASKED-BUILD 'printf "RUN go build -o /app ./... || :\n" >> "$d/svc/Dockerfile"'
  expect_rule o3make   R3-MASKED-BUILD 'printf "RUN make all || true\n" >> "$d/svc/Dockerfile"'
  expect_rule o3gradle R3-MASKED-BUILD 'printf "RUN ./gradlew assemble --no-daemon || true\n" >> "$d/svc/Dockerfile"'
  expect_rule o3tsc    R3-MASKED-BUILD 'printf "RUN tsc -p . || true\n" >> "$d/svc/Dockerfile"'
  expect_rule o3cargo  R3-MASKED-BUILD 'printf "RUN cargo build --release | tee /tmp/b\n" >> "$d/svc/Dockerfile"'
  expect_rule o3chain  R3-MASKED-BUILD 'printf "RUN npm ci && yarn build || true\n" >> "$d/svc/Dockerfile"'
  expect_rule o3npmb   R3-MASKED-BUILD 'printf "RUN npm build || true\n" >> "$d/svc/Dockerfile"'
  # O4 (R5): flow-style privileged, merge keys, flow services: model it or fail closed
  expect_rule o4flow   R5-PRIVILEGED 'printf "services:\n  a:\n    image: x\n  p: {image: x, privileged: true}\n" > "$d/docker-compose.yml"'
  expect_rule o4flow2  R5-PRIVILEGED 'printf "services:\n  p:\n    {image: x, privileged: \"yes\"}\n" > "$d/docker-compose.yml"'
  expect_rule o4inline R5-PRIVILEGED 'printf "services:\n  a:\n    image: x\n    privileged: true # kvm\n" > "$d/docker-compose.yml"'
  expect_rule o4merge  R10-COMPOSE-UNMODELED 'printf "services:\n  a:\n    <<: *base\n    image: x\n" > "$d/docker-compose.yml"'
  expect_rule o4topflow R10-COMPOSE-UNMODELED 'printf "services: {a: {image: x, privileged: true}}\n" > "$d/docker-compose.yml"'
  expect_rule o4weird  R10-COMPOSE-UNMODELED 'printf "services:\n  a:\n    image: x\n  \"quoted\": {image: y}\n" > "$d/docker-compose.yml"'
  expect_rule o4flowvol R10-COMPOSE-UNMODELED 'printf "services:\n  p: {image: x, volumes: [\"./a:/b\"], privileged: false}\n" > "$d/docker-compose.yml"'
  # --- review round 2 of the O1-O4 closure: RED fixtures first, then the fixes ---
  # S1/S2/RM3/RM7 (R3): the `|| false` / `|| exit N` exemption must keep the delimiter and only accept N in 1..255
  expect_rule s1semi   R3-MASKED-BUILD 'printf "RUN npm run build || false; true\n" >> "$d/svc/Dockerfile"'
  expect_rule s1amp    R3-MASKED-BUILD 'printf "RUN npm run build || false& true\n" >> "$d/svc/Dockerfile"'
  expect_rule s2exit256 R3-MASKED-BUILD 'printf "RUN npm run build || exit 256\n" >> "$d/svc/Dockerfile"'
  expect_rule rm3exit0 R3-MASKED-BUILD 'printf "RUN npm run build || exit 0\n" >> "$d/svc/Dockerfile"'
  expect_rule rm7bg    R3-MASKED-BUILD 'printf "RUN yarn build & true\n" >> "$d/svc/Dockerfile"'
  # T4/T5 (R3): env prefixes, runners, package-manager options, sh -c, lowercase run, heredocs, pipefail lines, trailing commands
  expect_rule t4env    R3-MASKED-BUILD 'printf "RUN NODE_ENV=production yarn build || true\n" >> "$d/svc/Dockerfile"'
  expect_rule t4npx    R3-MASKED-BUILD 'printf "RUN npx tsc || true\n" >> "$d/svc/Dockerfile"'
  expect_rule t4ws     R3-MASKED-BUILD 'printf "RUN yarn workspace web build || true\n" >> "$d/svc/Dockerfile"'
  expect_rule t4cwd    R3-MASKED-BUILD 'printf "RUN yarn --cwd web build || true\n" >> "$d/svc/Dockerfile"'
  expect_rule t4prefix R3-MASKED-BUILD 'printf "RUN npm --prefix web run build || true\n" >> "$d/svc/Dockerfile"'
  expect_rule t5shc    R3-MASKED-BUILD 'printf "RUN sh -c \"yarn build || true\"\n" >> "$d/svc/Dockerfile"'
  expect_rule t5shsemi R3-MASKED-BUILD 'printf "RUN sh -c \"yarn build; echo done\"\n" >> "$d/svc/Dockerfile"'
  expect_rule t5semi   R3-MASKED-BUILD 'printf "RUN yarn build; echo done\n" >> "$d/svc/Dockerfile"'
  expect_rule t5lower  R3-MASKED-BUILD 'printf "run yarn build || true\n" >> "$d/svc/Dockerfile"'
  expect_rule t5heredoc R3-MASKED-BUILD 'printf "RUN <<EOF\nyarn build\necho done\nEOF\n" >> "$d/svc/Dockerfile"'
  expect_rule t5pipefail R3-MASKED-BUILD 'printf "RUN set -o pipefail && yarn build || true\n" >> "$d/svc/Dockerfile"'
  expect_rule t5mount  R3-MASKED-BUILD 'printf "RUN --mount=type=cache,target=/c yarn build || true\n" >> "$d/svc/Dockerfile"'
  # T7 (R9): wget --method=HEAD and a compose `test:` block scalar
  expect_rule t7wget   R9-HEALTH-HEAD 'sed -i "s#^HEALTHCHECK.*#HEALTHCHECK CMD wget -q --method=HEAD -O- http://127.0.0.1:8080/health#" "$d/svc/Dockerfile"'
  expect_rule t7bs     R9-HEALTH-HEAD 'printf "services:\n  api:\n    image: x\n    healthcheck:\n      test: >-\n        curl -fsI\n        http://127.0.0.1/health\n" > "$d/docker-compose.yml"'
  # T1-T3 (R5/R7/R10): compose shapes the parser must model or fail closed on
  local CB='services:\n  api:\n    build:\n      context: .\n      dockerfile: svc/Dockerfile\n'
  expect_rule t1flowml R10-COMPOSE-UNMODELED 'printf "services:\n  p: {image: x,\n      privileged: true}\n" > "$d/docker-compose.yml"'
  expect_rule t1flowml4 R10-COMPOSE-UNMODELED 'printf "services:\n  p:\n    {image: x,\n     privileged: true}\n" > "$d/docker-compose.yml"'
  expect_rule t1quoted R5-PRIVILEGED 'printf "    \"privileged\": true\n" >> "$d/docker-compose.yml"'
  expect_rule t1spcolon R5-PRIVILEGED 'printf "    privileged : true\n" >> "$d/docker-compose.yml"'
  expect_rule t1tag    R10-COMPOSE-UNMODELED 'printf "    privileged: !!bool true\n" >> "$d/docker-compose.yml"'
  expect_rule t1bs     R10-COMPOSE-UNMODELED 'printf "    privileged: >-\n      true\n" >> "$d/docker-compose.yml"'
  expect_rule t1indent4 R10-COMPOSE-UNMODELED 'printf "services:\n    a:\n      image: x\n      privileged: true\n" > "$d/docker-compose.yml"'
  expect_rule t1body6  R10-COMPOSE-UNMODELED 'printf "services:\n  a:\n      image: x\n      privileged: true\n" > "$d/docker-compose.yml"'
  expect_rule t1odd    R10-COMPOSE-UNMODELED 'printf "services:\n  a:\n    image: x\n   privileged: true\n" > "$d/docker-compose.yml"'
  expect_rule t1junk   R10-COMPOSE-UNMODELED 'printf "    ? privileged\n    : true\n" >> "$d/docker-compose.yml"'
  expect_rule t2flowvol R10-COMPOSE-UNMODELED 'printf "'"$CB"'    volumes: [./svc:/app]\n" > "$d/docker-compose.yml"'
  expect_rule t2seqkey R10-COMPOSE-UNMODELED 'printf "'"$CB"'    volumes:\n    - ./svc:/app\n" > "$d/docker-compose.yml"'
  expect_rule t2qlong  R7-BUILDER-USERNS 'printf "'"$CB"'    volumes:\n      - \"type\": bind\n        \"source\": ./svc\n        \"target\": /app\n" > "$d/docker-compose.yml"'
  expect_rule t2firstkey R7-BUILDER-USERNS 'printf "'"$CB"'    volumes:\n      - consistency: cached\n        type: bind\n        source: ./svc\n        target: /app\n" > "$d/docker-compose.yml"'
  expect_rule t2qshort R7-BUILDER-USERNS 'printf "'"$CB"'    volumes:\n      - \"./svc:/app\"\n" > "$d/docker-compose.yml"'
  expect_rule t2flowml R10-COMPOSE-UNMODELED 'printf "'"$CB"'    volumes:\n      - {type: bind,\n         source: ./svc, target: /app}\n" > "$d/docker-compose.yml"'
  expect_rule t2alias  R10-COMPOSE-UNMODELED 'printf "'"$CB"'    volumes: *vols\n" > "$d/docker-compose.yml"'
  expect_rule t3bflow  R10-COMPOSE-UNMODELED 'printf "services:\n  api:\n    build: {context: ., dockerfile: svc/Dockerfile}\n" > "$d/docker-compose.yml"'
  expect_rule t3hcflow R10-COMPOSE-UNMODELED 'printf "services:\n  api:\n    image: x\n    healthcheck: {test: [CMD, true]}\n" > "$d/docker-compose.yml"'
  expect_rule t3bind5  R10-COMPOSE-UNMODELED 'printf "services:\n  api:\n    build:\n     context: .\n" > "$d/docker-compose.yml"'
  # reviewer survivors that had no fixture of their own (positive here, golden-FALSE in ctl4)
  expect_rule rm5hcflow R10-COMPOSE-UNMODELED 'printf "services:\n  p: {image: x, healthcheck: {test: [CMD, true]}}\n" > "$d/docker-compose.yml"'
  expect_rule rm6dest  R7-BUILDER-USERNS 'printf "'"$CB"'    volumes:\n      - type: bind\n        source: ./svc\n        destination: /app\n" > "$d/docker-compose.yml"'
  expect_rule rm9flowalias R10-COMPOSE-UNMODELED 'printf "services:\n  p: {image: *img}\n" > "$d/docker-compose.yml"'
  expect_rule rm10alias R10-COMPOSE-UNMODELED 'printf "    privileged: *priv\n" >> "$d/docker-compose.yml"'
  # golden-FALSE for the round-2 fixes: each correct shape MUST NOT fire
  d="$sb/ctl4"; mk_clean "$d"
  printf 'FROM alpine\nRUN apk add curl\nUSER 1001\nHEALTHCHECK CMD wget -qO- http://127.0.0.1/health\n' > "$d/svc/Dockerfile"
  printf 'RUN yarn build || false\nRUN npm run build || exit 1\nRUN npm run build || exit 255\nRUN yarn build 2>&1 && echo ok\nRUN set -e; yarn build; echo done\nRUN sh -ec "yarn build; echo done"\nRUN set -o pipefail && yarn build | tee /tmp/b\nRUN <<EOF\nset -e\nyarn build\necho done\nEOF\nRUN cat <<EOF > /etc/motd\nmake || true\nEOF\nRUN yarn add --dev build-tools\nRUN NODE_ENV=production yarn build\nRUN cd web; yarn build\nSHELL ["/bin/bash", "-o", "pipefail", "-c"]\nRUN yarn build | tee /tmp/b\n' >> "$d/svc/Dockerfile"
  printf 'services:\n  api:\n    build:\n      context: .\n      dockerfile: svc/Dockerfile\n    "userns_mode": "keep-id"\n    environment:\n    - X=1\n    command: >-\n      sh -c "privileged: true"\n    entrypoint: [\n      "a",\n      "b"\n    ]\n    volumes:\n      - type: bind\n        source: ./svc\n        destination: /app\n        read_only: true\n      - consistency: cached\n        type: volume\n        source: named\n        target: /d\n      - "named2:/data"\n    healthcheck:\n      test: >-\n        curl -fsS\n        http://127.0.0.1/health\n  b:\n    image: x\n    privileged: false\n  p: {image: x-y, privileged: false}\n' > "$d/docker-compose.yml"
  out="$(scan "$d")"; if [ $? -ne 0 ]; then echo "FAIL selftest[ctl4]: false positive: $out"; rc=1; else echo "ok   selftest[ctl4]: round-2 correct shapes not flagged"; fi
  # golden-FALSE controls for O1-O4: correct configurations MUST NOT fire
  d="$sb/ctl3"; mk_clean "$d"
  printf 'FROM alpine\nRUN apk add curl\nUSER 1001\nHEALTHCHECK CMD curl -fsS http://127.0.0.1/health\n' > "$d/svc/Dockerfile"
  printf 'RUN yarn install --frozen-lockfile\nRUN apt-get install -y make || true\nRUN yarn build\nRUN make build && make test\nRUN go build -o /a ./... && go vet ./...\nRUN echo "yarn build || true"\n# RUN yarn build || true\n' >> "$d/svc/Dockerfile"
  printf 'services:\n  api:\n    build:\n      context: .\n      dockerfile: svc/Dockerfile\n    userns_mode: "keep-id:uid=1000,gid=1000"\n    volumes:\n      - type: bind\n        source: ./svc\n        target: /app\n    healthcheck:\n      test: ["CMD", "curl", "-fsS", "http://127.0.0.1/health"]\n  w:\n    build: ./svc\n    volumes:\n      - type: bind\n        source: ./svc\n        target: /app\n        read_only: true\n      - type: volume\n        source: named\n        target: /data\n      - type: tmpfs\n        target: /tmp\n      - {type: bind, source: ./svc, target: /ro, read_only: true}\n    healthcheck:\n      test: ["CMD-SHELL", "curl -fsSL --insecure http://127.0.0.1/health"]\n  p: {image: x, privileged: false}\n  q: {image: y}\n' > "$d/docker-compose.yml"
  out="$(scan "$d")"; if [ $? -ne 0 ]; then echo "FAIL selftest[ctl3]: false positive: $out"; rc=1; else echo "ok   selftest[ctl3]: O1-O4 correct configs not flagged"; fi
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
