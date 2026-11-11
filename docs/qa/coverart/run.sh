#!/usr/bin/env bash
# CAT-COVERART-LIVE-1: live cover-art fetch checks (F2 32 MiB cap, F4 redirect/private blocking)
# through the REAL CoverArtService.downloadImage. See README.md for scope and limits.
#
# Usage: run.sh [--out DIR] [--mutate cap|strict|netguard] | --parse FILE | --help
# Exit: 0 PASS, 1 FAIL, 64 usage error, 2 environment problem (cannot determine).
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO="$(cd "$HERE/../../.." && pwd)"
API="$REPO/catalog-api"
PKG=./internal/services
RUN_RE='TestLiveCoverArt'
REQUIRED=(
  TestLiveCoverArt_CapIsProductionDefault
  TestLiveCoverArt_F2_Cap
  TestLiveCoverArt_F4_Redirects
  TestLiveCoverArt_F4_Redirects/link-local-metadata
  TestLiveCoverArt_F4_Redirects/private-rfc1918
  TestLiveCoverArt_F4_Redirects/ipv6-loopback
  TestLiveCoverArt_F4_Redirects/loopback-not-allowlisted
  TestLiveCoverArt_F4_StrictProductionClientRefusesLoopback
)

usage() { sed -n '2,7p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'; }

# parse_verdict FILE: PASS only if every REQUIRED test shows "--- PASS" and no "--- FAIL" exists.
parse_verdict() {
  local f="$1" r bad=0
  [ -s "$f" ] || { echo "VERDICT: FAIL (empty or missing output: instrument saw nothing)"; return 1; }
  if grep -Eq '^\s*--- FAIL:|^FAIL' "$f"; then echo "VERDICT: FAIL (a FAIL line is present)"; bad=1; fi
  for r in "${REQUIRED[@]}"; do
    grep -Eq "^\s*--- PASS: ${r} " "$f" || { echo "missing PASS: $r"; bad=1; }
  done
  [ "$bad" -eq 0 ] && { echo "VERDICT: PASS (${#REQUIRED[@]} required tests passed, no FAIL)"; return 0; }
  echo "VERDICT: FAIL"; return 1
}

OUT="" MUT=""
while [ $# -gt 0 ]; do
  case "$1" in
    --help|-h) usage; exit 0 ;;
    --parse) [ $# -ge 2 ] || exit 64; parse_verdict "$2"; exit $? ;;
    --out) [ $# -ge 2 ] || exit 64; OUT="$2"; shift 2 ;;
    --mutate) [ $# -ge 2 ] || exit 64; MUT="$2"; shift 2 ;;
    *) echo "unknown option: $1" >&2; usage >&2; exit 64 ;;
  esac
done
case "$MUT" in ""|cap|strict|netguard) ;; *) echo "unknown mutation: $MUT" >&2; exit 64 ;; esac

[ -d "$API/../submodules/security" ] && [ -n "$(ls -A "$API/../submodules/security" 2>/dev/null)" ] || {
  echo "submodules (replace targets of catalog-api/go.mod) are not initialised" >&2; exit 2; }
command -v go >/dev/null || { echo "go not found" >&2; exit 2; }

TS="$(date -u +%Y%m%dT%H%M%SZ)"
OUT="${OUT:-$HERE/results/run-$TS${MUT:+-mutant-$MUT}}"
mkdir -p "$OUT" || exit 2
SCR="$(mktemp -d "${TMPDIR:-/tmp}/coverart-ovl.XXXXXX")" || exit 2
trap 'rm -rf -- "${SCR:?}"' EXIT

OVL=()
if [ -n "$MUT" ]; then
  case "$MUT" in
    cap)      SRC="$API/internal/services/cover_art_service.go"
              sed 's/coverArtMaxBytesDefault int64 = 32 << 20/coverArtMaxBytesDefault int64 = 1 << 40/' "$SRC" > "$SCR/m.go" ;;
    strict)   # both layers of the production loopback protection (redundant by design) removed
              SRC="$API/internal/services/cover_art_service.go"
              sed 's/netguard.StrictClient(30\*time.Second, 3, netguard.HTTPOrHTTPS), \/\/ image URLs/netguard.Client(30*time.Second, 3, netguard.HTTPOrHTTPS), \/\/ image URLs/' "$SRC" \
                | sed '/^func (s \*CoverArtService) downloadImage/,/^}/{/GuardProviderURL(imageURL/,/^\t}/d}' > "$SCR/m.go" ;;
    netguard) SRC="$API/internal/netguard/netguard.go"
              sed 's/^func isBlocked(ip net.IP, honorAllow bool) bool {$/&\n\treturn false/' "$SRC" > "$SCR/m.go" ;;
  esac
  cmp -s "$SRC" "$SCR/m.go" && { echo "mutation $MUT did not change the source (needle failed)" >&2; exit 2; }
  printf '{"Replace":{"%s":"%s"}}\n' "$SRC" "$SCR/m.go" > "$SCR/overlay.json"
  OVL=(-overlay "$SCR/overlay.json")
  diff -u "$SRC" "$SCR/m.go" > "$OUT/mutation.diff.txt"
fi

{ echo "date_utc=$TS"; echo "mutation=${MUT:-none}"; echo "go=$(go version)"; echo "git_head=$(git -C "$REPO" rev-parse HEAD)";
  echo "git_dirty=$(git -C "$REPO" status --porcelain | wc -l)"; echo "host=$(uname -srm)"; } > "$OUT/ENV.txt"

PRE=(env GOMAXPROCS="${GOMAXPROCS:-2}" nice -n 10)
if command -v systemd-run >/dev/null && [ -S /run/user/$(id -u)/bus ]; then
  PRE=(env GOMAXPROCS="${GOMAXPROCS:-2}" DBUS_SESSION_BUS_ADDRESS="unix:path=/run/user/$(id -u)/bus"
       systemd-run --user --scope -q -p MemoryMax=4G -p MemorySwapMax=0 nice -n 10)
fi
( cd "$API" && "${PRE[@]}" go test -count=1 -tags live_coverart "${OVL[@]}" -run "$RUN_RE" -v "$PKG" ) > "$OUT/go_test.txt" 2>&1
echo "go_test_exit=$?" >> "$OUT/ENV.txt"
( cd "$API" && "${PRE[@]}" go test -count=1 -tags live_coverart "${OVL[@]}" -run "$RUN_RE" -json "$PKG" ) > "$OUT/go_test.json.txt" 2>&1

parse_verdict "$OUT/go_test.txt" > "$OUT/verdict.txt"; rc=$?
cat "$OUT/verdict.txt"
( cd "$OUT" && sha256sum -- * > SHA256SUMS.txt 2>/dev/null; true )
echo "evidence: $OUT"
exit $rc
