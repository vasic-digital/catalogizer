#!/usr/bin/env bash
#
# reorg_submodules.sh — Relocate every OWNED Catalogizer submodule into a root
# `submodules/` directory using lowercase snake_case names, then rewrite all
# path references across the repository.
#
# ─────────────────────────────────────────────────────────────────────────────
# WHAT THIS SCRIPT DOES
# ─────────────────────────────────────────────────────────────────────────────
#   1. For each (old → target) mapping below it runs `git mv <old> submodules/<snake>`
#      (skipped when already at target — fully idempotent).
#   2. Rewrites `.gitmodules`: both the section header `[submodule "<old>"]` and
#      the `path = <old>` line become `submodules/<snake>` (URL untouched).
#   3. Runs `git submodule sync` + `git submodule absorbgitdirs` so the real git
#      dirs under `.git/modules/...` follow the move and each submodule's gitdir
#      pointer + core.worktree are corrected.
#   4. Rewrites every consumer path reference using the BIMODAL rule:
#        - Consumers NOT under submodules/ (catalog-api, catalog-web, compose,
#          Dockerfile, top-level scripts): `../<OldName>` / `<OldName>/` →
#          `../submodules/<snake>` / `submodules/<snake>/`.
#        - Sibling consumers ALREADY under submodules/ (the React packages):
#          relative path STAYS `../<snake>` (one level up), only the NAME changes
#          to snake_case. Go siblings (challenges/recovery) are already lowercase
#          and are LEFT UNTOUCHED — never double-prefixed.
#
# ─────────────────────────────────────────────────────────────────────────────
# SAFETY MODEL
# ─────────────────────────────────────────────────────────────────────────────
#   * DRY_RUN=1 (DEFAULT) — prints every action it WOULD take and changes nothing.
#   * DRY_RUN=0           — actually performs the moves and rewrites.
#   * FORCE=1             — allow running even when the working tree has unrelated
#                          STAGED changes (otherwise the script refuses).
#   * Every destructive operation is guarded by DRY_RUN.
#   * Every rewrite is idempotent: re-running on an already-migrated tree is a
#     no-op and reports "no change".
#
# ─────────────────────────────────────────────────────────────────────────────
# USAGE
# ─────────────────────────────────────────────────────────────────────────────
#   Preview (default, safe):        scripts/reorg_submodules.sh
#   Execute the reorganization:     DRY_RUN=0 scripts/reorg_submodules.sh
#   Execute despite staged changes: DRY_RUN=0 FORCE=1 scripts/reorg_submodules.sh
#
# This script intentionally does NOT commit or push. The operator reviews the
# resulting working tree and commits under controlled conditions.
#
# NOTE: `constitution` is already migrated and is NOT in the mapping,
# so it is never touched.
#
# Constitution reference: doc §11.4.18 (companion: docs/scripts/reorg_submodules.md)
# ─────────────────────────────────────────────────────────────────────────────

set -euo pipefail

# ── Configuration ────────────────────────────────────────────────────────────
DRY_RUN="${DRY_RUN:-1}"
FORCE="${FORCE:-0}"

# Resolve repo root from this script's location (scripts/ is at repo root).
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${REPO_ROOT}"

# ── Logging helpers ──────────────────────────────────────────────────────────
c_reset=$'\033[0m'; c_blue=$'\033[34m'; c_green=$'\033[32m'
c_yellow=$'\033[33m'; c_red=$'\033[31m'; c_dim=$'\033[2m'

log()   { printf '%s[reorg]%s %s\n'  "${c_blue}"   "${c_reset}" "$*"; }
ok()    { printf '%s[ ok ]%s %s\n'   "${c_green}"  "${c_reset}" "$*"; }
warn()  { printf '%s[warn]%s %s\n'   "${c_yellow}" "${c_reset}" "$*"; }
err()   { printf '%s[fail]%s %s\n'   "${c_red}"    "${c_reset}" "$*" >&2; }
skip()  { printf '%s[skip]%s %s\n'   "${c_dim}"    "${c_reset}" "$*"; }
act()   { # describe a destructive action; prefix WOULD in dry-run
  if [ "${DRY_RUN}" = "1" ]; then printf '%s[plan]%s WOULD %s\n' "${c_yellow}" "${c_reset}" "$*";
  else printf '%s[ do ]%s %s\n' "${c_green}" "${c_reset}" "$*"; fi
}

# Counters for the final summary.
MOVES_DONE=0
FILES_EDITED=0

# ── Submodule mapping (old path → snake_case basename) ───────────────────────
# Order matters for logging clarity but moves are independent.
# Format: "OldPath:snake_basename"
MAPPING=(
  "WebSocket-Client-TS:websocket_client_ts"
  "UI-Components-React:ui_components_react"
  "Challenges:challenges"
  "Assets:assets"
  "Concurrency:concurrency"
  "Config:config"
  "Filesystem:filesystem"
  "Database:database"
  "Auth:auth"
  "Middleware:middleware"
  "RateLimiter:rate_limiter"
  "Observability:observability"
  "Media:media"
  "Watcher:watcher"
  "EventBus:event_bus"
  "Cache:cache"
  "Security:security"
  "Storage:storage"
  "Streaming:streaming"
  "Discovery:discovery"
  "Entities:entities"
  "Media-Types-TS:media_types_ts"
  "Catalogizer-API-Client-TS:catalogizer_api_client_ts"
  "Auth-Context-React:auth_context_react"
  "Media-Browser-React:media_browser_react"
  "Dashboard-Analytics-React:dashboard_analytics_react"
  "Media-Player-React:media_player_react"
  "Collection-Manager-React:collection_manager_react"
  "Containers:containers"
  "Lazy:lazy"
  "Memory:memory"
  "Recovery:recovery"
  "HelixQA:helix_qa"
  "DocProcessor:doc_processor"
  "LLMOrchestrator:llm_orchestrator"
  "LLMProvider:llm_provider"
  "VisionEngine:vision_engine"
  "ScreenDiff:screen_diff"
  "ReplayBuffer:replay_buffer"
  "VisualRegression:visual_regression"
  "TrainingCollector:training_collector"
)

snake_for() {  # echo snake basename for an old path; empty if unknown
  local old="$1" entry
  for entry in "${MAPPING[@]}"; do
    if [ "${entry%%:*}" = "${old}" ]; then printf '%s' "${entry#*:}"; return 0; fi
  done
  return 1
}

# ── Idempotent in-place file rewrite ─────────────────────────────────────────
# rewrite_file <file> <sed-expr...> : applies sed exprs; reports & counts only
# when content actually changes. DRY_RUN shows a unified diff and changes nothing.
rewrite_file() {
  local file="$1"; shift
  if [ ! -f "${file}" ]; then
    skip "absent, nothing to rewrite: ${file}"
    return 0
  fi
  local sed_args=()
  local e
  for e in "$@"; do sed_args+=(-e "$e"); done

  local new
  new="$(sed "${sed_args[@]}" "${file}")"
  if [ "${new}" = "$(cat "${file}")" ]; then
    skip "already up-to-date: ${file}"
    return 0
  fi

  FILES_EDITED=$((FILES_EDITED + 1))
  if [ "${DRY_RUN}" = "1" ]; then
    act "edit ${file}; diff:"
    diff -u "${file}" - <<<"${new}" | sed 's/^/      /' || true
  else
    printf '%s' "${new}" > "${file}.reorg.tmp"
    # preserve a trailing newline if the original had one
    case "$(tail -c1 "${file}"; echo X)" in
      $'\n'X) printf '\n' >> "${file}.reorg.tmp" ;;
    esac
    mv "${file}.reorg.tmp" "${file}"
    ok "edited ${file}"
  fi
}

# ─────────────────────────────────────────────────────────────────────────────
# PRE-FLIGHT
# ─────────────────────────────────────────────────────────────────────────────
preflight() {
  log "Repo root: ${REPO_ROOT}"
  if [ "${DRY_RUN}" = "1" ]; then
    warn "DRY_RUN=1 (default) — NO changes will be made. Set DRY_RUN=0 to execute."
  else
    warn "DRY_RUN=0 — changes WILL be made to the working tree."
  fi

  if [ ! -f .gitmodules ]; then err ".gitmodules not found at repo root"; exit 1; fi
  if ! command -v git >/dev/null 2>&1; then err "git not on PATH"; exit 1; fi

  # Refuse if there are UNRELATED staged changes (anything already staged that is
  # not a submodule dir we are about to move) unless FORCE=1.
  local staged
  staged="$(git diff --cached --name-only || true)"
  if [ -n "${staged}" ] && [ "${FORCE}" != "1" ]; then
    err "Working tree has staged changes; refusing to run. Inspect:"
    printf '%s\n' "${staged}" | sed 's/^/        /' >&2
    err "Re-run with FORCE=1 to override (only if you understand the staged set)."
    exit 1
  elif [ -n "${staged}" ]; then
    warn "Staged changes present but FORCE=1 set; continuing."
  fi
}

# ─────────────────────────────────────────────────────────────────────────────
# STEP 1+2: git mv each submodule and patch its .gitmodules section
# ─────────────────────────────────────────────────────────────────────────────
move_submodules() {
  log "── Step 1/2: relocate submodules + patch .gitmodules ──"
  local entry old snake target
  for entry in "${MAPPING[@]}"; do
    old="${entry%%:*}"; snake="${entry#*:}"; target="submodules/${snake}"

    if [ -e "${target}" ] && [ ! -e "${old}" ]; then
      skip "already at target: ${old} → ${target}"
    elif [ ! -e "${old}" ]; then
      warn "source missing (skipping move): ${old}"
    else
      act "git mv ${old} ${target}"
      if [ "${DRY_RUN}" != "1" ]; then
        mkdir -p submodules
        git mv "${old}" "${target}"
        MOVES_DONE=$((MOVES_DONE + 1))
      else
        MOVES_DONE=$((MOVES_DONE + 1))
      fi
    fi

    # Patch .gitmodules: section header rename + path line. Idempotent.
    patch_gitmodules_section "${old}" "${snake}"
  done
}

# patch_gitmodules_section <old> <snake>
# Renames [submodule "<old>"] → [submodule "submodules/<snake>"] and updates the
# `path = <old>` line to `path = submodules/<snake>`. URL is untouched.
# Uses `git config -f` for the path value where possible, and an awk pass for the
# section-header rename (git config cannot rename a section in place portably).
patch_gitmodules_section() {
  local old="$1" snake="$2" target="submodules/${snake}"
  local oldsec="submodule.${old}"
  local newsec="submodule.${target}"

  # Already migrated? (new section present, old absent)
  if git config -f .gitmodules --get "${newsec}.path" >/dev/null 2>&1 \
     && ! git config -f .gitmodules --get "${oldsec}.path" >/dev/null 2>&1; then
    skip ".gitmodules already migrated: [submodule \"${target}\"]"
    return 0
  fi

  if ! git config -f .gitmodules --get "${oldsec}.path" >/dev/null 2>&1; then
    skip ".gitmodules has no old section for ${old} (nothing to patch)"
    return 0
  fi

  act "rename .gitmodules section [submodule \"${old}\"] → [submodule \"${target}\"] and path → ${target}"
  if [ "${DRY_RUN}" != "1" ]; then
    # 1) Rename the section header via awk (exact-match on the header line).
    awk -v oldh="[submodule \"${old}\"]" -v newh="[submodule \"${target}\"]" '
      $0 == oldh { print newh; next }
      { print }
    ' .gitmodules > .gitmodules.reorg.tmp
    mv .gitmodules.reorg.tmp .gitmodules
    # 2) Update the path value within the (now renamed) section.
    git config -f .gitmodules "${newsec}.path" "${target}"
    # 3) Stage .gitmodules so the NEXT `git mv <submodule>` does not abort with
    #    "Please stage your changes to .gitmodules or stash them to proceed".
    git add -- .gitmodules
    ok ".gitmodules patched + staged for ${target}"
  fi
}

# ─────────────────────────────────────────────────────────────────────────────
# STEP 3: sync git's view of the moved submodules
# ─────────────────────────────────────────────────────────────────────────────
sync_submodules() {
  log "── Step 3: git submodule sync + absorbgitdirs ──"
  # `git submodule sync` rewrites each submodule's recorded URL/path into its
  # .git/config so worktree pointers match the patched .gitmodules.
  act "git submodule sync --recursive"
  # `git submodule absorbgitdirs` moves any submodule whose real git dir still
  # lives inside its (now relocated) worktree into .git/modules/<newpath>, and
  # fixes the worktree's `.git` gitdir pointer + the module's core.worktree.
  act "git submodule absorbgitdirs"
  if [ "${DRY_RUN}" != "1" ]; then
    git submodule sync --recursive
    git submodule absorbgitdirs
    ok "submodule git dirs synchronized and absorbed"
  fi
}

# ─────────────────────────────────────────────────────────────────────────────
# STEP 4: rewrite path references (bimodal)
# ─────────────────────────────────────────────────────────────────────────────

# 4a. catalog-api/go.mod — `=> ../<OldName>` → `=> ../submodules/<snake>`
rewrite_go_mod() {
  log "── Step 4a: catalog-api/go.mod replace directives ──"
  local entry old snake exprs=()
  for entry in "${MAPPING[@]}"; do
    old="${entry%%:*}"; snake="${entry#*:}"
    # Only the 24 Go modules appear here; harmless if a name is absent.
    # Idempotent: anchored on `../<Old>` not already prefixed by submodules/.
    exprs+=("s#=> \\.\\./${old}\$#=> ../submodules/${snake}#")
    exprs+=("s#=> \\.\\./${old}/#=> ../submodules/${snake}/#")
  done
  rewrite_file "catalog-api/go.mod" "${exprs[@]}"
}

# 4b. catalog-web/package.json — `file:../<OldName>` → `file:../submodules/<snake>`
rewrite_web_package() {
  log "── Step 4b: catalog-web/package.json file: deps ──"
  local entry old snake exprs=()
  for entry in "${MAPPING[@]}"; do
    old="${entry%%:*}"; snake="${entry#*:}"
    # Match the quoted dependency value exactly: "file:../<Old>"
    exprs+=("s#\"file:\\.\\./${old}\"#\"file:../submodules/${snake}\"#g")
  done
  rewrite_file "catalog-web/package.json" "${exprs[@]}"
}

# 4c. catalog-api/Dockerfile — `COPY <Name>/ /build/<Name>/`
#     → `COPY submodules/<snake>/ /build/submodules/<snake>/`
rewrite_dockerfile() {
  log "── Step 4c: catalog-api/Dockerfile COPY lines ──"
  local entry old snake exprs=()
  for entry in "${MAPPING[@]}"; do
    old="${entry%%:*}"; snake="${entry#*:}"
    # Anchored on `^COPY <Old>/ ...` so catalog-api/ COPY lines are untouched.
    exprs+=("s#^COPY ${old}/ /build/${old}/\$#COPY submodules/${snake}/ /build/submodules/${snake}/#")
  done
  rewrite_file "catalog-api/Dockerfile" "${exprs[@]}"
}

# 4d. Sibling consumers ALREADY under submodules/ (React packages).
#     Relative depth stays `../` — only the NAME becomes snake_case.
#     They reference Media-Types-TS and/or Catalogizer-API-Client-TS.
#     Go siblings (challenges/recovery → ../containers, ../concurrency) are
#     already lowercase and are deliberately NOT touched here.
rewrite_react_siblings() {
  log "── Step 4d: React sibling package.json file:../ name updates ──"
  local siblings=(
    "Auth-Context-React"
    "Catalogizer-API-Client-TS"
    "Collection-Manager-React"
    "Dashboard-Analytics-React"
    "Media-Browser-React"
    "Media-Player-React"
  )
  # Only these two referenced targets need name translation in siblings.
  local mtts; mtts="$(snake_for Media-Types-TS)"                       # media_types_ts
  local capi; capi="$(snake_for Catalogizer-API-Client-TS)"           # catalogizer_api_client_ts

  local s base
  for s in "${siblings[@]}"; do
    base="$(snake_for "${s}")"
    # After the move the file lives at submodules/<base>/package.json.
    # Before the move it is still at <s>/package.json. Handle whichever exists.
    local f
    if [ -f "submodules/${base}/package.json" ]; then f="submodules/${base}/package.json"
    elif [ -f "${s}/package.json" ]; then f="${s}/package.json"
    else skip "sibling package.json absent for ${s}"; continue; fi

    rewrite_file "${f}" \
      "s#\"file:\\.\\./Media-Types-TS\"#\"file:../${mtts}\"#g" \
      "s#\"file:\\.\\./Catalogizer-API-Client-TS\"#\"file:../${capi}\"#g"
  done

  # Explicitly verify the Go siblings are already-correct lowercase and leave them.
  local go_sib
  for go_sib in "Challenges:../containers" "Recovery:../concurrency"; do
    local sname="${go_sib%%:*}" expect="${go_sib#*:}"
    local gb; gb="$(snake_for "${sname}")"
    local gf
    if [ -f "submodules/${gb}/go.mod" ]; then gf="submodules/${gb}/go.mod"
    elif [ -f "${sname}/go.mod" ]; then gf="${sname}/go.mod"
    else continue; fi
    if grep -q "=> ${expect}\$" "${gf}" 2>/dev/null; then
      skip "Go sibling already correct (leaving as-is): ${gf} ('${expect}')"
    else
      warn "Go sibling ${gf} does not contain expected '${expect}'; NOT modified — verify manually."
    fi
  done
}

# 4e. docker-compose files.
rewrite_compose() {
  log "── Step 4e: docker-compose path refs ──"
  local hq; hq="$(snake_for HelixQA)"      # helix_qa
  local as; as="$(snake_for Assets)"       # assets

  # qa-robot: `build: ./HelixQA`, `./HelixQA/data` volume.
  # Order matters: replace the more specific `./HelixQA/` form first.
  rewrite_file "docker-compose.qa-robot.yml" \
    "s#\\./HelixQA/#./submodules/${hq}/#g" \
    "s#\\./HelixQA\$#./submodules/${hq}#g" \
    "s#\\./HelixQA #./submodules/${hq} #g"

  # dev: line ~83 has `../Assets:/app/../Assets:ro` (two `../Assets` occurrences).
  rewrite_file "docker-compose.dev.yml" \
    "s#\\.\\./Assets#../submodules/${as}#g"
}

# 4f. Top-level scripts: HelixQA + Containers path refs.
rewrite_scripts() {
  log "── Step 4f: scripts/ path refs (HelixQA, Containers) ──"
  local hq; hq="$(snake_for HelixQA)"          # helix_qa
  local cn; cn="$(snake_for Containers)"       # containers

  # Common HelixQA rewrites, applied per-file and idempotent. The patterns cover
  # the observed forms: $PROJECT_ROOT/HelixQA/..., ${PROJECT_ROOT}/HelixQA/...,
  # "$PROJECT_ROOT/HelixQA", and bare `HelixQA/` path segments. We anchor so we
  # do NOT touch the literal product name "HelixQA" in prose/log strings.
  local helix_exprs=(
    "s#(\\\$\\{?PROJECT_ROOT\\}?)/HelixQA/#\\1/submodules/${hq}/#g"
    "s#(\\\$\\{?PROJECT_ROOT\\}?)/HelixQA\"#\\1/submodules/${hq}\"#g"
    "s#(\\\$\\{?PROJECT_ROOT\\}?)/HelixQA\$#\\1/submodules/${hq}#g"
  )

  local f
  for f in \
    scripts/deploy-vision-hosts.sh \
    scripts/helixqa-orchestrator.sh \
    scripts/run-helixqa.sh \
    scripts/run-helixqa-all.sh \
    scripts/run-helixqa-api.sh \
    scripts/run-helixqa-web.sh \
    scripts/run-helixqa-desktop.sh \
    scripts/run-helixqa-android.sh \
    scripts/run-helixqa-androidtv.sh \
    scripts/run-helixqa-tests.sh ; do
    # Use extended regex for the capture groups above.
    rewrite_file_ere "${f}" "${helix_exprs[@]}"
  done

  # detect-landmines.sh:
  #   - scan target `Containers/` → `submodules/containers/`
  #   - `-d HelixQA` guard and `HelixQA/pkg/ HelixQA/cmd/` scan targets
  rewrite_file_ere "scripts/detect-landmines.sh" \
    "s# Containers/ # submodules/${cn}/ #g" \
    "s#-d HelixQA\b#-d submodules/${hq}#g" \
    "s#\bHelixQA/pkg/#submodules/${hq}/pkg/#g" \
    "s#\bHelixQA/cmd/#submodules/${hq}/cmd/#g"

  # distributed-boot.sh / full-distribute.sh: Containers/.env refs.
  for f in scripts/distributed-boot.sh scripts/full-distribute.sh; do
    rewrite_file_ere "${f}" \
      "s#(\\\$\\{?PROJECT_ROOT\\}?)/Containers/#\\1/submodules/${cn}/#g" \
      "s#(default: \\.\\./)Containers/\\.env#\\1submodules/${cn}/.env#g" \
      "s#(default: )Containers/\\.env#\\1submodules/${cn}/.env#g"
  done

  # anti-bluff-scan.sh exclude paths: HelixQA/... → submodules/helix_qa/...
  # Lives at scripts/audit/anti-bluff-scan.sh (and possibly scripts/anti-bluff-scan.sh).
  for f in scripts/anti-bluff-scan.sh scripts/audit/anti-bluff-scan.sh; do
    rewrite_file_ere "${f}" \
      "s#\bHelixQA/banks/templates#submodules/${hq}/banks/templates#g" \
      "s#\bHelixQA/tools/opensource#submodules/${hq}/tools/opensource#g"
  done
}

# rewrite_file_ere — like rewrite_file but uses extended regex (sed -E).
rewrite_file_ere() {
  local file="$1"; shift
  if [ ! -f "${file}" ]; then
    skip "absent, nothing to rewrite: ${file}"
    return 0
  fi
  local sed_args=() e
  for e in "$@"; do sed_args+=(-e "$e"); done
  local new
  new="$(sed -E "${sed_args[@]}" "${file}")"
  if [ "${new}" = "$(cat "${file}")" ]; then
    skip "already up-to-date: ${file}"
    return 0
  fi
  FILES_EDITED=$((FILES_EDITED + 1))
  if [ "${DRY_RUN}" = "1" ]; then
    act "edit ${file}; diff:"
    diff -u "${file}" - <<<"${new}" | sed 's/^/      /' || true
  else
    printf '%s' "${new}" > "${file}.reorg.tmp"
    case "$(tail -c1 "${file}"; echo X)" in
      $'\n'X) printf '\n' >> "${file}.reorg.tmp" ;;
    esac
    mv "${file}.reorg.tmp" "${file}"
    ok "edited ${file}"
  fi
}

# ─────────────────────────────────────────────────────────────────────────────
# SUMMARY
# ─────────────────────────────────────────────────────────────────────────────
print_summary() {
  echo
  log "════════════════════════ SUMMARY ════════════════════════"
  if [ "${DRY_RUN}" = "1" ]; then
    warn "DRY RUN — nothing was changed."
    log  "Submodule moves that WOULD run: ${MOVES_DONE}"
    log  "Files that WOULD be edited:     ${FILES_EDITED}"
  else
    ok   "Submodule moves performed: ${MOVES_DONE}"
    ok   "Files edited:              ${FILES_EDITED}"
  fi
  echo
  log "POST-RUN OPERATOR VERIFICATION CHECKLIST:"
  cat <<'CHECKLIST'
      [ ] git submodule status            # every path now submodules/<snake>, no '-'/'+' surprises
      [ ] (cd catalog-api && go build ./...)   # Go replace directives resolve
      [ ] (cd catalog-api && go vet ./...)     # zero warnings (Constitution)
      [ ] (cd catalog-web && npm ci)           # file: deps resolve to submodules/*
      [ ] docker build -f catalog-api/Dockerfile .   # COPY submodules/<snake>/ paths valid
      [ ] bash scripts/run-helixqa-api.sh      # a HelixQA script finds submodules/helix_qa
      [ ] bash scripts/audit/anti-bluff-scan.sh   # exclude paths updated, scan still runs
      [ ] git diff .gitmodules                  # sections + path = submodules/<snake>, URLs intact
      [ ] grep -rn '\.\./Assets\|/HelixQA/\|/Containers/\.env' scripts docker-compose*.yml  # expect none stale
      [ ] Review, then commit (script does NOT commit/push).
CHECKLIST
  echo
}

# ─────────────────────────────────────────────────────────────────────────────
# MAIN
# ─────────────────────────────────────────────────────────────────────────────
main() {
  preflight
  move_submodules
  sync_submodules
  rewrite_go_mod
  rewrite_web_package
  rewrite_dockerfile
  rewrite_react_siblings
  rewrite_compose
  rewrite_scripts
  print_summary
}

main "$@"
