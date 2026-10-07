#!/usr/bin/env bash
# track-coverage.sh - T198. The rewritten Go coverage collector for the SPLIT lanes (docs/05 7.1, 7.4 A1; round-23 review I8). It replaces the old scripts/track-coverage.sh,
# which swallowed a failing package (`|| true`, `2>/dev/null`) and so reported a figure for a suite that did not pass. Two phases, because a result comes ONLY from the callback
# record of its build, never from a foreground wait:
#   submit  --app APP --src DIR --lanes "unit integration" [--packages FILE] --state DIR [--group G] [--image IMG-GO]
#           one BUILD GROUP per run (scripts/build/dispatch.sh submit --group G), one member per (lane, package): the compile half
#           `go test -c -cover -covermode=atomic -coverpkg=./... -o /out/<lane>/<pkg>.test ./<dir>` (the integration lane adds `-tags integration`), then group-seal;
#           it returns at once. The package list is FILE (`<import path>TAB<dir>`) or, absent, `go list` through the TIC lane. State goes to DIR/members.tsv.
#   collect --state DIR --out DIR [--item ID] [--app APP]
#           run when the group callback fires: for each member read `dispatch.sh status <build_id>` (the callback record), then, per succeeded member: the brought-back binary must
#           exist, scripts/build/verify_artifact.sh must accept it for its build, and only then does it run in the run half (TIC) from its package directory with
#           `-test.coverprofile`; the per-package profiles are merged (counts summed per block) and the result goes through the recorder. Exit 1, naming each package and the reason,
#           on: callback_record_missing, build_failed, infra_failed (its reason detail kept), cancelled/blocked, binary_missing, verify_refused, package_failed, package_panicked,
#           profile_empty. Healthy packages are still run and merged; result.json says `failed` and lists every error.
# Hooks (honoured ONLY with TC_TEST_MODE=1, else REFUSED test_hook_outside_test_mode): TC_DISPATCH, TC_VERIFY, TC_TIC, TC_RECORDER, TC_FENCE_DIR (the directory holding <app>.yaml fences).
# UNCONFIRMED against the real dispatcher (T121a/T121b are not built yet): the `status` fields artifact_dir, artifact_name and the verify_artifact.sh call form
# `--build-id ID --file PATH`; the run-half command (a `sh -c 'cd /src/<dir> && /out/...'` through TIC). Contract and limits: docs/scripts/track-coverage.md.
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"; ROOT="$(cd "$HERE/../.." && pwd)"
refuse() { echo "track-coverage: REFUSED reason=$1 ${2:-}" >&2; exit "${3:-1}"; }
usage() { echo "track-coverage: usage: $1" >&2; echo "track-coverage: track-coverage.sh submit --app A --src D --lanes L --state D [--packages F] [--group G] | collect --state D --out D [--item ID]" >&2; exit 2; }
DISPATCH="$ROOT/scripts/build/dispatch.sh"; VERIFY="$ROOT/scripts/build/verify_artifact.sh"; TIC="$ROOT/scripts/test-in-container.sh"; RECORDER="$ROOT/tools/evidence/evrec"
if [ -n "${TC_DISPATCH+x}${TC_VERIFY+x}${TC_TIC+x}${TC_RECORDER+x}${TC_FENCE_DIR+x}" ] && [ "${TC_TEST_MODE:-}" != 1 ]; then   # MUT:hooks
  refuse test_hook_outside_test_mode "TC_DISPATCH / TC_VERIFY / TC_TIC / TC_RECORDER are test hooks and need TC_TEST_MODE=1"
fi
FENCE_DIR="$ROOT/coverage/exclusions"; [ -z "${TC_FENCE_DIR:-}" ] || FENCE_DIR="$TC_FENCE_DIR"
[ -z "${TC_DISPATCH:-}" ] || DISPATCH="$TC_DISPATCH"; [ -z "${TC_VERIFY:-}" ] || VERIFY="$TC_VERIFY"; [ -z "${TC_TIC:-}" ] || TIC="$TC_TIC"; [ -z "${TC_RECORDER:-}" ] || RECORDER="$TC_RECORDER"
MODE="${1:-}"; [ -n "$MODE" ] || usage "a mode (submit or collect) is required"; shift
case "$MODE" in submit|collect) ;; *) usage "unknown mode '$MODE'";; esac
APP=catalog-api; SRC=""; LANES="unit integration"; PKGS=""; STATE=""; GROUP=""; IMAGE=IMG-GO; OUT=""; ITEM=T201
while [ $# -gt 0 ]; do case "$1" in
  --app) APP="$2"; shift 2;; --src) SRC="$2"; shift 2;; --lanes) LANES="$2"; shift 2;; --packages) PKGS="$2"; shift 2;; --state) STATE="$2"; shift 2;;
  --group) GROUP="$2"; shift 2;; --image) IMAGE="$2"; shift 2;; --out) OUT="$2"; shift 2;; --item) ITEM="$2"; shift 2;;
  *) usage "unknown argument '$1'";; esac; done
[ -n "$STATE" ] || usage "--state is required"
pkgname() { local n="${1#*/}"; [ "$n" != "$1" ] || n="root"; echo "${n//\//__}"; }

if [ "$MODE" = submit ]; then
  [ -n "$SRC" ] && [ -d "$SRC" ] || usage "--src DIR is required"
  [ -n "$GROUP" ] || GROUP="tcg-$(date -u +%Y%m%dT%H%M%S)-$$"
  if [ -z "$PKGS" ]; then
    PKGS="$STATE/packages.tsv"; mkdir -p "$STATE"
    bash "$TIC" "$APP" unit -- go list -f '{{if or .TestGoFiles .XTestGoFiles}}{{.ImportPath}}{{"\t"}}{{.Dir}}{{end}}' ./... >"$PKGS" 2>"$STATE/golist.err" || refuse package_list_failed "go list through TIC failed: $(head -c 200 "$STATE/golist.err")"
  fi
  [ -r "$PKGS" ] || refuse package_list_unreadable "$PKGS"
  mkdir -p "$STATE" || refuse state_uncreatable "$STATE"
  : >"$STATE/members.tsv"; echo "$GROUP" >"$STATE/group"; echo "$APP" >"$STATE/app"
  SNAP="$(bash "$DISPATCH" snapshot "$SRC" 2>/dev/null)" || refuse snapshot_failed "$SRC"
  FAILS=0
  for LANE in $LANES; do
    TAGS=(); [ "$LANE" != integration ] || TAGS=(-tags integration)
    while IFS=$'\t' read -r IMPORT DIR || [ -n "${IMPORT:-}" ]; do
      [ -n "${IMPORT:-}" ] || continue
      # review I10: `go list {{.Dir}}` is an ABSOLUTE path (documented by the go command; the container mounts the checkout at /src). The compile target and the run half's
      # `cd /src/<dir>` both want the path RELATIVE to the checkout root, so an absolute one is normalised here and one outside /src is refused, never composed into `/src//src/...`.
      case "$DIR" in /src/*) DIR="${DIR#/src/}";; /*) refuse package_dir_outside_source "$IMPORT: $DIR is absolute and not under /src";; esac   # MUT:dir_abs
      NAME="$(pkgname "$IMPORT")"; REL="${DIR#*/}"; [ "$REL" != "$DIR" ] || REL="."
      ARGV=(go test "${TAGS[@]}" -c -cover -covermode=atomic -coverpkg=./... -o "/out/$LANE/$NAME.test" "./$REL")
      AD="$(bash "$DISPATCH" argv-digest "${ARGV[@]}" 2>/dev/null)" || refuse argv_digest_failed "$NAME"
      BID="$(bash "$DISPATCH" submit --purpose "build:$APP:$LANE:$SNAP:$AD:primary:$GROUP-$NAME" --callback tic-group-record --image "$IMAGE" --src "$SRC" --component "$APP" --group "$GROUP" -- "${ARGV[@]}" 2>"$STATE/submit-$LANE-$NAME.err")" || { echo "track-coverage: submit failed for $LANE/$NAME: $(head -c 200 "$STATE/submit-$LANE-$NAME.err")" >&2; FAILS=$((FAILS+1)); continue; }
      printf '%s\t%s\t%s\t%s\n' "$LANE" "$NAME" "$DIR" "$BID" >>"$STATE/members.tsv"
    done <"$PKGS"
  done
  bash "$DISPATCH" group-seal "$GROUP" >/dev/null 2>&1 || refuse group_seal_failed "$GROUP"
  echo "$GROUP"
  [ "$FAILS" = 0 ] || refuse member_submit_failed "$FAILS member(s) were not submitted; the group is sealed with the rest" 1
  exit 0
fi

# ------------------------------- collect -------------------------------
[ -n "$OUT" ] || usage "--out is required"
[ -r "$STATE/members.tsv" ] && [ -s "$STATE/members.tsv" ] || refuse no_submitted_group "no members in $STATE (run submit first)"
[ -f "$STATE/app" ] && APP="$(cat "$STATE/app")"
mkdir -p "$OUT/run" "$OUT/profiles" || refuse out_uncreatable "$OUT"
ERRORS="$OUT/errors.txt"; : >"$ERRORS"; PROFILES=()
err() { echo "track-coverage: FAIL: $1" >&2; echo "$1" >>"$ERRORS"; }
while IFS=$'\t' read -r LANE NAME DIR BID; do
  PKG="$NAME"
  STATUS_JSON="$(bash "$DISPATCH" status "$BID" 2>/dev/null)"
  if [ -z "$STATUS_JSON" ] || ! jq -e . >/dev/null 2>&1 <<<"$STATUS_JSON"; then err "$PKG ($LANE): callback_record_missing (no readable status record for build $BID)"; continue; fi
  KIND="$(jq -r '.terminal.kind // ""' <<<"$STATUS_JSON")"; CLS="$(jq -r '.terminal.exit_class // ""' <<<"$STATUS_JSON")"; DETAIL="$(jq -r '.terminal.reason // ""' <<<"$STATUS_JSON")"
  if [ "$KIND" != completed ]; then err "$PKG ($LANE): $KIND ($DETAIL)"; continue; fi
  case "$CLS" in
    succeeded) ;;
    build_failed) err "$PKG ($LANE): build_failed ($DETAIL)"; continue;;
    infra_failed) err "$PKG ($LANE): infra_failed ($DETAIL)"; continue;;   # MUT:infra-detail
    *) err "$PKG ($LANE): $CLS ($DETAIL)"; continue;;
  esac
  ADIR="$(jq -r '.artifact_dir // ""' <<<"$STATUS_JSON")"; ANAME="$(jq -r '.artifact_name // ""' <<<"$STATUS_JSON")"; [ -n "$ANAME" ] || ANAME="${LANE}__$NAME.test"
  ART="$ADIR/$ANAME"
  if [ -z "$ADIR" ] || [ ! -f "$ART" ]; then err "$PKG ($LANE): binary_missing (the brought-back binary $ART does not exist)"; continue; fi
  bash "$VERIFY" --build-id "$BID" --file "$ART" >"$OUT/run/$LANE-$NAME.verify" 2>&1 || { err "$PKG ($LANE): verify_refused ($(tr '\n' ' ' <"$OUT/run/$LANE-$NAME.verify" | cut -c1-160))"; continue; }
  mkdir -p "$OUT/run/$LANE"; cp "$ART" "$OUT/run/$LANE/$NAME.test"; chmod +x "$OUT/run/$LANE/$NAME.test"
  RUNLOG="$OUT/run/$LANE-$NAME.log"
  BIN_RUN="$TIC"; RUNARGS=("$APP" "$LANE" --out "$OUT/run" -- sh -c "cd /src/$DIR && /out/$LANE/$NAME.test -test.coverprofile=/out/$LANE/$NAME.cover -test.v")
  { "$BIN_RUN" "${RUNARGS[@]}"; RUN_RC=$?; } >"$RUNLOG" 2>&1
  if grep -q "^panic:" "$RUNLOG"; then err "$PKG ($LANE): package_panicked (see $RUNLOG)"; continue; fi
  if [ "$RUN_RC" != 0 ]; then err "$PKG ($LANE): package_failed (exit $RUN_RC, see $RUNLOG)"; continue; fi
  COVER="$OUT/run/$LANE/$NAME.cover"
  PROFILE_BLOCKS="$( [ -f "$COVER" ] && python3 -I "$HERE/gocov_merge.py" blocks "$COVER" 2>/dev/null || echo 0)"
  if [ "$PROFILE_BLOCKS" -eq 0 ]; then err "$PKG ($LANE): profile_empty (the profile $COVER holds no block)"; continue; fi
  cp "$COVER" "$OUT/profiles/$LANE-$NAME.cover"; PROFILES+=("$OUT/profiles/$LANE-$NAME.cover")
done <"$STATE/members.tsv"
NERR="$(wc -l <"$ERRORS" | tr -d ' ')"
MODULE="$(sed -n 's/^module //p' "$ROOT/$APP/go.mod" 2>/dev/null | head -1)"
if [ "${#PROFILES[@]}" -gt 0 ]; then
  python3 -I "$HERE/gocov_merge.py" merge --out "$OUT/merged.cover" "${PROFILES[@]}" >"$OUT/merge.json" || { err "merge: profiles could not be merged (see $OUT/merge.json)"; NERR=$((NERR+1)); }
  FENCE="$FENCE_DIR/$APP.yaml"
  if [ -f "$FENCE" ]; then   # review I6: the fence is gated and then APPLIED, so the figure's scope is the fence's scope
    FOUT="$(bash "$HERE/check_exclusions.sh" "$FENCE" --root "$ROOT/$APP" 2>&1)" || { err "exclusion fence refused by the T200 gate: $(printf '%s' "$FOUT" | tr '\n' ' ' | cut -c1-300)"; NERR=$((NERR+1)); }   # MUT:fence_gate
  fi
  [ ! -f "$OUT/merged.cover" ] || python3 -I "$HERE/gocov_merge.py" summary --profile "$OUT/merged.cover" ${MODULE:+--module "$MODULE"} ${FENCE:+--exclusions "$FENCE"} >"$OUT/summary.json"
fi
STATUS=ok; [ "$NERR" = 0 ] || STATUS=failed
python3 -I - "$OUT" "$STATUS" "$ERRORS" <<'PY'
import json,sys,os
out,status,errf=sys.argv[1:4]
s=json.load(open(os.path.join(out,"summary.json"))) if os.path.exists(os.path.join(out,"summary.json")) else {"statements":0,"covered":0,"percent":"0.00","packages":[]}
s["status"]=status; s["errors"]=[l.rstrip("\n") for l in open(errf) if l.strip()]
open(os.path.join(out,"result.json"),"w").write(json.dumps(s,indent=2,sort_keys=True)+"\n")
PY
if [ "$STATUS" = ok ]; then
  "$RECORDER" run "$ITEM" GREEN 1 runtime coverage-baseline -- python3 -I "$HERE/gocov_merge.py" baseline --result "$OUT/result.json" --out "$OUT/baseline.json" --app "$APP" --runs 1 --exclusions "coverage/exclusions/$APP.yaml" || { echo "track-coverage: FAIL: the recorder did not record the result" >&2; exit 1; }
  echo "track-coverage: $APP $(jq -r '.covered' "$OUT/result.json") of $(jq -r '.statements' "$OUT/result.json") statements = $(jq -r '.percent' "$OUT/result.json")%"
  exit 0
fi
echo "track-coverage: FAILED: $NERR error(s) above; result.json says failed, no baseline was recorded" >&2
exit 1
