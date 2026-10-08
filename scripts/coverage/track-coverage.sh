#!/usr/bin/env bash
# track-coverage.sh - T198. The rewritten Go coverage collector for the SPLIT lanes (docs/05 7.1, 7.4 A1; round-23 review I8). It replaces the old scripts/track-coverage.sh,
# which swallowed a failing package (`|| true`, `2>/dev/null`) and so reported a figure for a suite that did not pass. Two phases, because a result comes ONLY from the callback
# record of its build, never from a foreground wait:
#   submit  --app APP --src DIR --lanes "unit integration" [--packages FILE] --state DIR [--group G] [--image IMG-GO]
#           one BUILD GROUP per run (scripts/build/dispatch.sh submit --group G), one member per (lane, package) that BELONGS to the lane, then group-seal; it returns at once.
#           The compile half runs INSIDE the module: `sh -c 'cd /src/<app> && go test [-tags integration] -c -cover -covermode=atomic -coverpkg=./... -o /out/<lane>/<pkg>.test ./<rel>'`.
#           Lane membership is read from the REAL gating of the tests, not from a flag: the `integration` lane holds the packages under a tests/integration directory or whose
#           test files carry `//go:build integration`; every other package is `unit` (a second lane that recompiled the same binaries would sum the same counts twice). The
#           package list is FILE (`<import path>TAB<dir>`) or, absent, `go list` through the TIC lane from the module directory. State goes to DIR: members.tsv and state.json
#           (schema tc-state/1: group, snapshot, commit, expected_members, submitted, nonce, `sealed` - true only after every member was submitted AND the group was sealed; an
#           interrupted or partly failed submit leaves sealed=false and collect refuses it). The build tags no lane compiles are written to DIR/gaps.json, never silently dropped.
#   collect --state DIR --out DIR --item ID [--app APP]
#           run when the group callback fires. --out must be a NEW or EMPTY directory (a stale result.json, profile or log of another run could otherwise be recorded as this
#           run's); --item is a register item id (CAT-nnn, FND-nnnn, RUN-n, AUD-x: the id the evidence recorder's schema accepts), mandatory. For each member: read `dispatch.sh
#           status <build_id>` (the callback record), then, per succeeded member, COPY the brought-back binary from the dispatcher's build tree (<builds root>/<build_id>/artifacts/
#           <lane>/<pkg>.test) into the run directory, verify the build tree against the digest the completed event recorded (artifact_manifest_sha256, the dispatcher's own tree
#           manifest) and the copy against its line of that manifest, and only then run the COPY in the run half (TIC) from its package directory with `-test.coverprofile`.
#           The per-package profiles are merged (counts summed per block), the result goes through the recorder. Every step's exit status is checked; the status is decided AFTER
#           result.json is written and read back. Exit 1, naming each package and the reason, on: callback_record_missing, not_terminal, build_failed, infra_failed (its reason detail
#           kept), cancelled/blocked, binary_missing, copy_failed, verify_refused, package_failed, package_panicked, profile_empty, member_unaccounted. Healthy packages are still run and
#           merged; result.json says `failed` and lists every error. Refused up front (exit 1): out_not_empty, state_incomplete, group_not_sealed, snapshot_skew (the source changed
#           since submit: the binaries were compiled from the snapshot and the run half runs against the live tree).
# Hooks (honoured ONLY with TC_TEST_MODE=1, else REFUSED test_hook_outside_test_mode): TC_DISPATCH, TC_TIC, TC_RECORDER, TC_FENCE_DIR (the directory holding <app>.yaml fences), TC_GOMOD (the go.mod the module path is read from).
# The brought-back tree layout, the `status` fields and the absence of scripts/build/verify_artifact.sh are taken from scripts/build/dispatch.sh (cmd_status, bring_back, artifact_ok);
# test_track_coverage.sh runs the REAL `dispatch.sh status` and the REAL evrec against fixture records (the contract legs). T121a/T121b (the remote path) are not built yet: the
# remote half stays UNCONFIRMED. Contract and limits: docs/scripts/track-coverage.md.
set -u
CALLER_PWD="$PWD"
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"; ROOT="$(cd "$HERE/../.." && pwd)"
cd "$ROOT" || exit 1   # the lane source is the repository root: the package list, the run half and the snapshot are one tree (K6.3 b)
refuse() { echo "track-coverage: REFUSED reason=$1 ${2:-}" >&2; exit "${3:-1}"; }
usage() { echo "track-coverage: usage: $1" >&2; echo "track-coverage: track-coverage.sh submit --app A --src D --lanes L --state D [--packages F] [--group G] | collect --state D --out D --item ID" >&2; exit 2; }
DISPATCH="$ROOT/scripts/build/dispatch.sh"; TIC="$ROOT/scripts/test-in-container.sh"; RECORDER="$ROOT/tools/evidence/evrec"
if [ -n "${TC_DISPATCH+x}${TC_TIC+x}${TC_RECORDER+x}${TC_FENCE_DIR+x}${TC_GOMOD+x}${TC_VERIFY+x}" ] && [ "${TC_TEST_MODE:-}" != 1 ]; then   # MUT:hooks
  refuse test_hook_outside_test_mode "TC_DISPATCH / TC_TIC / TC_RECORDER / TC_FENCE_DIR are test hooks and need TC_TEST_MODE=1"
fi
FENCE_DIR="$ROOT/coverage/exclusions"; [ -z "${TC_FENCE_DIR:-}" ] || FENCE_DIR="$TC_FENCE_DIR"
[ -z "${TC_DISPATCH:-}" ] || DISPATCH="$TC_DISPATCH"; [ -z "${TC_TIC:-}" ] || TIC="$TC_TIC"; [ -z "${TC_RECORDER:-}" ] || RECORDER="$TC_RECORDER"
BUILDS_ROOT="${DISPATCH_BUILDS_ROOT:-$ROOT/.audit/builds}"
MODE="${1:-}"; [ -n "$MODE" ] || usage "a mode (submit or collect) is required"; shift
case "$MODE" in submit|collect) ;; *) usage "unknown mode '$MODE'";; esac
APP=catalog-api; SRC=""; LANES="unit integration"; PKGS=""; STATE=""; GROUP=""; IMAGE=IMG-GO; OUT=""; ITEM=""
need_val() { [ $# -ge 2 ] || usage "option $1 needs a value"; }
while [ $# -gt 0 ]; do case "$1" in
  --app) need_val "$@"; APP="$2"; shift 2;; --src) need_val "$@"; SRC="$2"; shift 2;; --lanes) need_val "$@"; LANES="$2"; shift 2;; --packages) need_val "$@"; PKGS="$2"; shift 2;;
  --state) need_val "$@"; STATE="$2"; shift 2;; --group) need_val "$@"; GROUP="$2"; shift 2;; --image) need_val "$@"; IMAGE="$2"; shift 2;;
  --out) need_val "$@"; OUT="$2"; shift 2;; --item) need_val "$@"; ITEM="$2"; shift 2;;
  *) usage "unknown argument '$1'";; esac; done
[ -n "$STATE" ] || usage "--state is required"
abspath() { case "$1" in /*) printf '%s' "$1";; *) printf '%s/%s' "$CALLER_PWD" "$1";; esac; }   # MUT:abspath
STATE="$(abspath "$STATE")"; [ -z "$OUT" ] || OUT="$(abspath "$OUT")"; [ -z "$SRC" ] || SRC="$(abspath "$SRC")"; [ -z "$PKGS" ] || PKGS="$(abspath "$PKGS")"   # K6.3 c: the runner refuses a relative --out
pkgname() { local n="${1#*/}"; [ "$n" != "$1" ] || n="root"; echo "${n//\//__}"; }
now_nonce() { printf '%s-%s-%s' "$(date -u +%Y%m%dT%H%M%SZ)" "$$" "$RANDOM$RANDOM"; }
write_state() { # write_state SEALED INTERRUPTED : atomic temp+rename
  jq -n --arg group "$GROUP" --arg app "$APP" --arg src "$SRC" --arg snap "$SNAP" --arg commit "$COMMIT" --arg nonce "$NONCE" --arg lanes "$LANES" \
     --argjson expected "$EXPECTED" --argjson submitted "$SUBMITTED" --argjson sealed "$1" --argjson interrupted "$2" --argjson fails "$FAILS" \
     '{schema:"tc-state/1",group:$group,app:$app,src:$src,snapshot:$snap,commit:$commit,nonce:$nonce,lanes:$lanes,expected_members:$expected,submitted:$submitted,failed_submissions:$fails,sealed:$sealed,interrupted:$interrupted}' >"$STATE/state.json.tmp.$$" \
    && mv -f "$STATE/state.json.tmp.$$" "$STATE/state.json"
}

if [ "$MODE" = submit ]; then
  [ -n "$SRC" ] && [ -d "$SRC" ] || usage "--src DIR is required"
  [ -n "$GROUP" ] || GROUP="tcg-$(date -u +%Y%m%dT%H%M%S)-$$"
  mkdir -p "$STATE" || refuse state_uncreatable "$STATE"
  [ ! -s "$STATE/members.tsv" ] && [ ! -e "$STATE/state.json" ] || refuse state_not_empty "$STATE already holds a submitted group (use a new state directory): a reused state could mix two runs"   # MUT:state_not_empty
  if [ -z "$PKGS" ]; then
    PKGS="$STATE/packages.tsv"
    # K6.3 a: the module is under /src/<app>; the repository root has no go.mod. `.Dir` is absolute under /src.
    bash "$TIC" --out "$STATE/tic" "$APP" unit -- sh -c "cd /src/$APP && go list -f '{{if or .TestGoFiles .XTestGoFiles}}{{.ImportPath}}{{\"\\t\"}}{{.Dir}}{{end}}' ./..." >"$PKGS" 2>"$STATE/golist.err" </dev/null || refuse package_list_failed "go list through TIC failed: $(head -c 200 "$STATE/golist.err")"
    bash "$TIC" --out "$STATE/tic" "$APP" unit -- sh -c "cd /src/$APP && go list -f '{{.ImportPath}}{{\"\\t\"}}{{.Dir}}' ./..." >"$STATE/all_packages.tsv" 2>>"$STATE/golist.err" </dev/null || : >"$STATE/all_packages.tsv"
  fi
  [ -r "$PKGS" ] || refuse package_list_unreadable "$PKGS"
  : >"$STATE/members.tsv"; : >"$STATE/members.tmp"
  SNAP="$(bash "$DISPATCH" snapshot "$SRC" 2>/dev/null </dev/null)" || refuse snapshot_failed "$SRC"
  COMMIT="$(env -u GIT_DIR -u GIT_WORK_TREE -u GIT_INDEX_FILE git -C "$SRC" rev-parse HEAD 2>/dev/null </dev/null)" || COMMIT=""
  NONCE="$(now_nonce)"; EXPECTED=0; SUBMITTED=0; FAILS=0
  on_sig() { write_state false true; echo "track-coverage: submit interrupted by $1; the group is NOT sealed and collect will refuse this state" >&2; exit "$2"; }
  trap 'on_sig INT 130' INT; trap 'on_sig TERM 143' TERM
  write_state false false   # an unsealed state exists from the first moment: a crash never leaves "no state" or a stale sealed one
  lane_of() { # lane_of DIR : unit | integration | integration-tagged, from the real gating of the tests in that package
    local d="$1"
    case "$d" in */tests/integration|*/tests/integration/*|*/integration|*/integration/*) echo integration; return;; esac
    if grep -lqE '^//go:build[[:space:]](.*[^!A-Za-z0-9_])?integration([^A-Za-z0-9_]|$)' "$SRC/$d"/*_test.go 2>/dev/null </dev/null; then echo integration-tagged; else echo unit; fi
  }
  declare -A TAGSEEN=()
  for LANE in $LANES; do
    while IFS=$'\t' read -r -u 9 IMPORT DIR || [ -n "${IMPORT:-}" ]; do
      [ -n "${IMPORT:-}" ] || continue
      # review I10: `go list {{.Dir}}` is an ABSOLUTE path (the container mounts the checkout at /src). The compile target and the run half's `cd /src/<dir>` both want the path
      # RELATIVE to the checkout root, so an absolute one is normalised here and one outside /src is refused, never composed into `/src//src/...`.
      case "$DIR" in /src/*) DIR="${DIR#/src/}";; /*) refuse package_dir_outside_source "$IMPORT: $DIR is absolute and not under /src";; esac   # MUT:dir_abs
      NAME="$(pkgname "$IMPORT")"; REL="${DIR#*/}"; [ "$REL" != "$DIR" ] || REL="."
      MEMBER_LANE="$(lane_of "$DIR")"
      case "$LANE:$MEMBER_LANE" in unit:unit|integration:integration|integration:integration-tagged) ;; *) continue;; esac   # MUT:lane_membership
      TAGS=""; [ "$MEMBER_LANE" != integration-tagged ] || TAGS="-tags integration "
      EXPECTED=$((EXPECTED+1))
      CMDSTR="cd /src/$APP && go test ${TAGS}-c -cover -covermode=atomic -coverpkg=./... -o /out/$LANE/$NAME.test ./$REL"   # MUT:cd_app
      ARGV=(sh -c "$CMDSTR")
      AD="$(bash "$DISPATCH" argv-digest "${ARGV[@]}" 2>/dev/null </dev/null)" || refuse argv_digest_failed "$NAME"
      BID="$(bash "$DISPATCH" submit --purpose "build:$APP:$LANE:$SNAP:$AD:primary:$GROUP-$NAME" --callback tic-group-record --image "$IMAGE" --src "$SRC" --component "$APP" --group "$GROUP" -- "${ARGV[@]}" 2>"$STATE/submit-$LANE-$NAME.err" </dev/null)" || { echo "track-coverage: submit failed for $LANE/$NAME: $(head -c 200 "$STATE/submit-$LANE-$NAME.err")" >&2; FAILS=$((FAILS+1)); continue; }   # MUT:stdin_guard
      printf '%s\t%s\t%s\t%s\n' "$LANE" "$NAME" "$DIR" "$BID" >>"$STATE/members.tsv"
      SUBMITTED=$((SUBMITTED+1))
    done 9<"$PKGS"   # MUT:read_fd
  done
  # the build tags no lane compiles are an honest gap (K6.3 d): integration is compiled by the integration lane, GOOS/GOARCH tags by the default build, everything else by NO lane
  python3 -I - "$SRC" "$APP" "$STATE" "$PKGS" <<'PY'
import json, os, re, sys
src, app, state, pkgs = sys.argv[1:5]
tags = {}; skip = {"integration", "linux", "darwin", "windows", "amd64", "arm64", "386", "arm", "cgo", "unix", "ignore", "race", "js", "wasm"}
for dp, dns, fns in os.walk(os.path.join(src, app)):
    dns[:] = [d for d in dns if d not in (".git", "node_modules", "vendor")]
    for f in fns:
        if not f.endswith(".go"):
            continue
        try:
            head = open(os.path.join(dp, f), encoding="utf-8", errors="replace").read(2048)
        except OSError:
            continue
        for m in re.finditer(r"(?m)^//go:build (.+)$", head):
            for t in re.findall(r"[A-Za-z_][A-Za-z0-9_.]*", m.group(1)):
                if t in skip or re.match(r"^go1\.\d+$", t):
                    continue
                tags.setdefault(t, []).append(os.path.relpath(os.path.join(dp, f), src))
allp = {}
try:
    for line in open(os.path.join(state, "all_packages.tsv"), encoding="utf-8"):
        a = line.rstrip("\n").split("\t")
        if len(a) == 2:
            allp[a[0]] = a[1]
except OSError:
    pass
withtests = set()
try:
    for line in open(pkgs, encoding="utf-8"):
        a = line.rstrip("\n").split("\t")
        if a and a[0]:
            withtests.add(a[0])
except OSError:
    pass
json.dump({"uncompiled_build_tags": {t: {"files": len(v), "example": v[0]} for t, v in sorted(tags.items())},
           "packages_without_tests": sorted(p for p in allp if p not in withtests)}, open(os.path.join(state, "gaps.json"), "w"), indent=2, sort_keys=True)
PY
  SEALED=false
  if [ "$FAILS" = 0 ] && [ "$SUBMITTED" = "$EXPECTED" ] && bash "$DISPATCH" group-seal "$GROUP" >/dev/null 2>&1 </dev/null; then SEALED=true; fi   # MUT:seal_gate
  write_state "$SEALED" false
  [ "$SEALED" = true ] || { bash "$DISPATCH" group-seal "$GROUP" >/dev/null 2>&1 </dev/null; }   # a failed submit still seals the open group so the dispatcher can finish it; the STATE stays unsealed
  echo "$GROUP"
  [ "$FAILS" = 0 ] || refuse member_submit_failed "$FAILS member(s) were not submitted; the state is NOT sealed and collect will refuse it" 1
  [ "$SEALED" = true ] || refuse group_seal_failed "$GROUP"
  exit 0
fi

# ------------------------------- collect -------------------------------
[ -n "$OUT" ] || usage "--out is required"
[ -n "$ITEM" ] || usage "--item is required (a register item id: CAT-nnn, FND-nnnn, RUN-n or AUD-x)"
[[ "$ITEM" =~ ^(CAT-[0-9]{3,}|FND-[0-9]{4,}|RUN-[0-9]+|AUD-[0-9A-Za-z-]+)$ ]] || usage "--item $ITEM is not a register item id the evidence schema accepts (CAT-nnn, FND-nnnn, RUN-n, AUD-x)"
[ -r "$STATE/members.tsv" ] && [ -s "$STATE/members.tsv" ] || refuse no_submitted_group "no members in $STATE (run submit first)"
[ -r "$STATE/state.json" ] || refuse state_incomplete "$STATE/state.json is missing: the submit did not finish writing its state"
jq -e '.schema == "tc-state/1"' "$STATE/state.json" >/dev/null 2>&1 || refuse state_incomplete "$STATE/state.json is not a tc-state/1 record"
[ "$(jq -r .sealed "$STATE/state.json")" = true ] || refuse group_not_sealed "the submit did not seal its group (interrupted: $(jq -r .interrupted "$STATE/state.json"); failed submissions: $(jq -r .failed_submissions "$STATE/state.json")): collecting would record a baseline over a subset of the packages"   # MUT:sealed_check
EXPECTED_N="$(jq -r .expected_members "$STATE/state.json")"; MEMBERS_N="$(grep -c . "$STATE/members.tsv")"
[ "$EXPECTED_N" = "$MEMBERS_N" ] || refuse state_incomplete "the state expects $EXPECTED_N members and members.tsv holds $MEMBERS_N"   # MUT:state_complete
APP="$(jq -r .app "$STATE/state.json")"; NONCE="$(jq -r .nonce "$STATE/state.json")"; SNAP="$(jq -r .snapshot "$STATE/state.json")"; COMMIT="$(jq -r .commit "$STATE/state.json")"; SRC="$(jq -r .src "$STATE/state.json")"
[ ! -e "$OUT" ] || [ -z "$(ls -A "$OUT" 2>/dev/null)" ] || refuse out_not_empty "$OUT already holds files: a stale result.json, profile or log of another run could be recorded as this run's"   # MUT:out_not_empty
mkdir -p "$OUT/run" "$OUT/profiles" || refuse out_uncreatable "$OUT"
SNAPNOW="$(bash "$DISPATCH" snapshot "$SRC" 2>/dev/null </dev/null)" || refuse snapshot_failed "$SRC"
[ "$SNAPNOW" = "$SNAP" ] || refuse snapshot_skew "the source at $SRC changed since submit (snapshot $SNAP at submit, $SNAPNOW now): the binaries were compiled from the first and the run half would run against the second"   # MUT:snapshot_skew
ERRORS="$OUT/errors.txt"; : >"$ERRORS"; PROFILES=(); PROFILE_COUNT=0; ERROR_COUNT=0
err() { echo "track-coverage: FAIL: $1" >&2; echo "$1" >>"$ERRORS"; ERROR_COUNT=$((ERROR_COUNT+1)); }
tree_list() { ( cd "$1" 2>/dev/null && LC_ALL=C find . -type f -print0 | LC_ALL=C sort -z | xargs -0 -r sha256sum | sed 's#  \./#  #' ); }
while IFS=$'\t' read -r -u 9 LANE NAME DIR BID; do
  PKG="$NAME"
  STATUS_JSON="$(bash "$DISPATCH" status "$BID" 2>/dev/null </dev/null)"
  if [ -z "$STATUS_JSON" ] || ! jq -e . >/dev/null 2>&1 <<<"$STATUS_JSON"; then err "$PKG ($LANE): callback_record_missing (no readable status record for build $BID)"; continue; fi
  if [ "$(jq -r '.terminal == null' <<<"$STATUS_JSON")" = true ]; then err "$PKG ($LANE): not_terminal (state=$(jq -r '.state // "unknown"' <<<"$STATUS_JSON"), callback_state=$(jq -r '.callback_state // ""' <<<"$STATUS_JSON"))"; continue; fi   # MUT:not_terminal
  KIND="$(jq -r '.terminal.kind // ""' <<<"$STATUS_JSON")"; CLS="$(jq -r '.terminal.exit_class // ""' <<<"$STATUS_JSON")"; DETAIL="$(jq -r '.terminal.reason // ""' <<<"$STATUS_JSON")"
  if [ "$KIND" != completed ]; then err "$PKG ($LANE): $KIND ($DETAIL)"; continue; fi   # MUT:kind_completed
  case "$CLS" in
    succeeded) ;;
    build_failed) err "$PKG ($LANE): build_failed ($DETAIL)"; continue;;
    infra_failed) err "$PKG ($LANE): infra_failed ($DETAIL)"; continue;;   # MUT:infra-detail
    *) err "$PKG ($LANE): $CLS ($DETAIL)"; continue;;
  esac
  AROOT="$BUILDS_ROOT/$BID/artifacts"; ART="$AROOT/$LANE/$NAME.test"
  if [ ! -f "$ART" ]; then err "$PKG ($LANE): binary_missing (the brought-back binary $ART does not exist)"; continue; fi
  mkdir -p "$OUT/run/$LANE"; BIN="$OUT/run/$LANE/$NAME.test"
  cp "$ART" "$BIN" 2>"$OUT/run/$LANE-$NAME.cp.err" </dev/null || { err "$PKG ($LANE): copy_failed ($(head -c 160 "$OUT/run/$LANE-$NAME.cp.err" | tr '\n' ' '))"; continue; }   # MUT:copy_status
  chmod +x "$BIN"
  # the build tree must still have the digest the completed event recorded, and the COPY must be the file that tree lists (verify_artifact.sh does not exist yet)
  WANT="$(jq -r 'select(.event == null and .kind == "completed") | .artifact_manifest_sha256' "$BUILDS_ROOT/$BID/events.jsonl" 2>/dev/null </dev/null | tail -1)"
  tree_list "$AROOT" >"$OUT/run/$LANE-$NAME.manifest" 2>/dev/null; GOT="$(sha256sum <"$OUT/run/$LANE-$NAME.manifest" | cut -d' ' -f1)"
  COPYSHA="$(sha256sum <"$BIN" | cut -d' ' -f1)"; LISTED="$(awk -v p="$LANE/$NAME.test" '$2 == p {print $1}' "$OUT/run/$LANE-$NAME.manifest")"
  if [ -z "$WANT" ] || [ "$WANT" != "$GOT" ] || [ "$COPYSHA" != "$LISTED" ]; then   # MUT:verify
    err "$PKG ($LANE): verify_refused (build tree manifest ${GOT:0:12} vs completed event ${WANT:0:12}; copy ${COPYSHA:0:12} vs listed ${LISTED:0:12})"; continue
  fi
  rm -f "$OUT/run/$LANE/$NAME.cover"   # a profile left by an earlier run half is never merged
  RUNLOG="$OUT/run/$LANE-$NAME.log"
  { "$TIC" "$APP" "$LANE" --out "$OUT/run" -- sh -c "cd /src/$DIR && /out/$LANE/$NAME.test -test.coverprofile=/out/$LANE/$NAME.cover -test.v" </dev/null; RUN_RC=$?; } >"$RUNLOG" 2>&1   # MUT:run_half
  if grep -q "^panic:" "$RUNLOG"; then err "$PKG ($LANE): package_panicked (see $RUNLOG)"; continue; fi
  if [ "$RUN_RC" != 0 ]; then err "$PKG ($LANE): package_failed (exit $RUN_RC, see $RUNLOG)"; continue; fi
  COVER="$OUT/run/$LANE/$NAME.cover"
  PROFILE_BLOCKS="$( [ -f "$COVER" ] && python3 -I "$HERE/gocov_merge.py" blocks "$COVER" 2>/dev/null </dev/null || echo 0)"
  if [ "$PROFILE_BLOCKS" -eq 0 ]; then err "$PKG ($LANE): profile_empty (the profile $COVER holds no block)"; continue; fi   # MUT:profile_empty
  cp "$COVER" "$OUT/profiles/$LANE-$NAME.cover" </dev/null || { err "$PKG ($LANE): copy_failed (the profile could not be copied)"; continue; }
  PROFILES+=("$OUT/profiles/$LANE-$NAME.cover"); PROFILE_COUNT=$((PROFILE_COUNT+1))
done 9<"$STATE/members.tsv"   # MUT:read_fd_collect
# every member yields exactly one profile or one error (the loop above `continue`s on each error and counts it)
[ $((PROFILE_COUNT + ERROR_COUNT)) = "$MEMBERS_N" ] || err "member_unaccounted (members $MEMBERS_N, profiles $PROFILE_COUNT, errors $ERROR_COUNT)"   # MUT:reconcile
GOMOD="$ROOT/$APP/go.mod"; [ -z "${TC_GOMOD:-}" ] || GOMOD="$TC_GOMOD"
FENCE="$FENCE_DIR/$APP.yaml"; FENCE_ARGS=(); FENCE_NOTE=none
if [ "${#PROFILES[@]}" -gt 0 ]; then
  python3 -I "$HERE/gocov_merge.py" merge --out "$OUT/merged.cover" "${PROFILES[@]}" >"$OUT/merge.json" 2>"$OUT/merge.err" </dev/null || err "merge: profiles could not be merged ($(head -c 200 "$OUT/merge.err" | tr '\n' ' '))"   # MUT:merge_status
  if [ -f "$FENCE" ]; then   # review I6: the fence is SNAPSHOTTED, gated and APPLIED as one file, so the figure's scope is the scope that was checked
    mkdir -p "$OUT/fence"; cp "$FENCE" "$OUT/fence/$APP.yaml"
    FOUT="$(bash "$HERE/check_exclusions.sh" "$OUT/fence/$APP.yaml" --root "$ROOT/$APP" --repo "$ROOT" --json "$OUT/exclusions-check.json" 2>&1 </dev/null)" || err "exclusion fence refused by the T200 gate: $(printf '%s' "$FOUT" | tr '\n' ' ' | cut -c1-300)"   # MUT:fence_gate
    FENCE_ARGS=(--exclusions "$OUT/fence/$APP.yaml" --root "$ROOT/$APP")
  fi
  if [ -f "$OUT/merged.cover" ]; then
    python3 -I "$HERE/gocov_merge.py" summary --profile "$OUT/merged.cover" --go-mod "$GOMOD" ${FENCE_ARGS[@]+"${FENCE_ARGS[@]}"} >"$OUT/summary.json" 2>"$OUT/summary.err" </dev/null || err "summary: the figure could not be computed ($(head -c 200 "$OUT/summary.err" | tr '\n' ' '))"   # MUT:summary_status
  fi
fi
# result.json is written from summary.json (or the empty summary), then READ BACK; the status is decided only after that
python3 -I - "$OUT" "$ERRORS" "$NONCE" "$STATE" "$([ -f "$OUT/fence/$APP.yaml" ] && sha256sum <"$OUT/fence/$APP.yaml" | cut -d' ' -f1 || echo none)" <<'PY' || err "result: result.json could not be written"
import json, os, sys
out, errf, nonce, state, fsha = sys.argv[1:6]
sp = os.path.join(out, "summary.json")
s = json.load(open(sp)) if os.path.exists(sp) and os.path.getsize(sp) > 0 else {"statements": 0, "covered": 0, "percent": "0.00", "packages": []}
errs = [l.rstrip("\n") for l in open(errf) if l.strip()]
gaps = {}
try:
    gaps = json.load(open(os.path.join(state, "gaps.json")))
except (OSError, ValueError):
    pass
s["nonce"] = nonce
s["errors"] = errs
s["status"] = "failed" if errs else "ok"
s["fence"] = fsha
s["uncompiled_build_tags"] = gaps.get("uncompiled_build_tags")
s["packages_without_tests"] = gaps.get("packages_without_tests")
tmp = os.path.join(out, "result.json.tmp")
open(tmp, "w").write(json.dumps(s, indent=2, sort_keys=True) + "\n")
os.replace(tmp, os.path.join(out, "result.json"))
PY
STATUS=failed
if jq -e --arg n "$NONCE" '.nonce == $n and .status == "ok"' "$OUT/result.json" >/dev/null 2>&1 && [ "$ERROR_COUNT" = 0 ]; then STATUS=ok; fi   # MUT:status_after_result
if [ "$STATUS" = ok ]; then
  "$RECORDER" run "$ITEM" BASELINE 1 go_binary "$OUT/result.json" --evidence-class runtime --oracle invariant --oracle-independent --test-source "$HERE/gocov_merge.py" -- \
    python3 -I "$HERE/gocov_merge.py" baseline --result "$OUT/result.json" --out "$OUT/baseline.json" --app "$APP" --runs 1 --commit "$COMMIT" --snapshot "$SNAP" --nonce "$NONCE" </dev/null || { echo "track-coverage: FAIL: the recorder did not record the result" >&2; exit 1; }   # MUT:recorder
  [ -s "$OUT/baseline.json" ] || { echo "track-coverage: FAIL: the recorder ran but no baseline.json was written" >&2; exit 1; }
  echo "track-coverage: $APP $(jq -r '.covered' "$OUT/result.json") of $(jq -r '.statements' "$OUT/result.json") statements = $(jq -r '.percent' "$OUT/result.json")%"
  exit 0
fi
echo "track-coverage: FAILED: $ERROR_COUNT error(s) above; result.json says $(jq -r '.status // "unreadable"' "$OUT/result.json" 2>/dev/null), no baseline was recorded" >&2
exit 1
