#!/usr/bin/env bash
# run_pinned.sh - T007 (RUNP). One-shot rootless run of a digest-pinned image from the lock file.
#
# Usage:  run_pinned.sh [--rw docs|.audit/scratch] [--out <dir>] [--network=none] [--need <bytes>] [--op-id <id>] <IMG-ID> -- <command word>...
#   IMG-ID      id of an entry in the lock file (build/containers/images.lock.yaml, or $RUNP_LOCK when set; schema docs/16 section 7.1)
#   --rw V      one extra writable bind, only the two exact strings `docs` and `.audit/scratch` (compared as strings before any path
#               resolution), at most once; the bind source $PWD/V must be a real directory path (no symlink component), else
#               rw_source_not_canonical. Without --rw no writable source path is mounted.
#   --out DIR   output directory mounted at /out:rw (default $PWD/.audit/out/<op_id>/, created here, never tmpfs). Must be an absolute
#               path; the RESOLVED path is what is bound. A path inside (or above) the source tree is accepted only under the sanctioned
#               output roots .audit/out, .audit/scratch and specs/001-full-project-audit-remediation/evidence (never an unlisted source
#               path such as docs, which would reopen the --rw allow-list): else out_dir_in_source / out_dir_not_absolute.
#   --network=none   no network inside the container (local offline scans)
#   --need N    bytes the run needs for the disk-headroom precondition (default: size_bytes of the lock entry, else 0)
# Environment:
#   RUNP_PRINT_ARGV=1   print the composed `podman run` argv, one element per line, and exit 0 without starting a container
#   RUNP_LOCK=<file>    lock file to read (documented test input and the CPA released copy); never an edit of the tracked lock
#   RUNP_MEMORY (bytes), RUNP_CPUS, RUNP_PIDS   limit overrides; without them the limits are the conservative defaults below.
#               RUNP_MEMORY must be 1..0.60*MemTotal and RUNP_CPUS 1..0.60*nproc (the section 12.6 ceiling cannot be lifted):
#               memory_override_out_of_bounds / cpus_override_out_of_bounds. RUNP_PIDS must be 1..min(8192, ulimit -u / 2)
#               (pids_override_out_of_bounds; the default 2048 is clamped to it). The ulimit read must be `unlimited` or a positive
#               integer, else pids_budget_unavailable (fail closed, never the permissive 8192).
#   A checkout path or a resolved --out containing ':' is refused (source_path_malformed / out_dir_malformed): podman reads the ':' as the
#   volume separator (probed: rc 125). A NEWLINE is refused by launcher policy, not because podman fails on it (it binds it, probed): the print
#   mode and the shim log are one argv element per line, and an element holding a newline would make that oracle ambiguous. A space or a TAB
#   in a path is NOT refused: probed with real podman, both are bound correctly by `-v <path>:<dest>`.
#   RUNP_USER=<uid>:<gid>   the container user. Default: the host uid:gid ($(id -u):$(id -g)), so files written to /out belong to the host
#               user, git sees no dubious ownership on the source mount, and root-only test skips do not skip. `RUNP_USER=0:0` is the
#               explicit, documented override for a run that needs root (it is never the default). Malformed: user_override_malformed.
#   RUNP_TEST_MODE=1    declares a test run. The two test hooks below are honoured ONLY with it; set without it they are REFUSED
#               (test_hook_outside_test_mode): a hook replaces the real reading and could otherwise lift a ceiling (round 6, F2).
#   RUNP_MEMINFO        meminfo file (test hook, needs RUNP_TEST_MODE=1; default /proc/meminfo). An unparsable file is meminfo_unreadable.
#   RUNP_ULIMIT_U       replaces `ulimit -u` (test hook, needs RUNP_TEST_MODE=1). Zero, unparsable or negative: pids_budget_unavailable.
#   DISK_HEADROOM_OUT_DIR is inherited by scripts/containers/disk_headroom.sh (CPA sets it to $CPA_RUN/disk/)
# Limits (docs/16 section 8, "jobs = 1 until measured"): memory = min(0.60 * MemTotal, MemAvailable - max(4 GiB, 0.15 * MemTotal)),
#   memory-swap = memory (no swap), cpus = min(2, 0.60*nproc), pids-limit = min(2048, min(8192, ulimit -u / 2)), until build/containers/profile.json holds a measured envelope.
# Refusals exit 1 with `run_pinned: REFUSED reason=<code>`; usage errors exit 2. The reason of a disk-headroom refusal is passed through.
# Podman is called directly (tracked deviation (b) of the P0-P1 conventions: the Go run primitive of submodules/containers has no typed
# memory/pids/label options and no digest validation, $EV/wp09/containers-run-primitive.md). The image is always preceded by `--`.
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$HERE/../.." && pwd)"
LOCK="${RUNP_LOCK:-$ROOT_DIR/build/containers/images.lock.yaml}"
HEADROOM="$HERE/disk_headroom.sh"

refuse() { echo "run_pinned: REFUSED reason=$1 ${2:-}" >&2; exit 1; }
usage()  { echo "run_pinned: usage: $1" >&2; echo "run_pinned: run_pinned.sh [--rw docs|.audit/scratch] [--out <dir>] [--network=none] [--need <bytes>] [--op-id <id>] <IMG-ID> -- <cmd>..." >&2; exit 2; }
valid_int() { case "$1" in ''|*[!0-9]*) return 1;; 0) return 0;; 0*) return 1;; esac; [ "${#1}" -le 18 ]; }

RW=""; OUT=""; NET=""; NEED=""; OP_ID=""; IMG=""; SEEN_RW=0; SEEN_OUT=0
while [ $# -gt 0 ]; do
  case "$1" in
    --) shift; break;;
    --rw)  [ $# -ge 2 ] || usage "--rw requires a value"; [ "$SEEN_RW" = 0 ] || refuse rw_given_twice; SEEN_RW=1; RW="$2"; shift 2;;
    --out) [ $# -ge 2 ] || usage "--out requires a value"; [ "$SEEN_OUT" = 0 ] || usage "--out given twice"; SEEN_OUT=1; OUT="$2"; shift 2;;
    --network=none) NET="none"; shift;;
    --need) [ $# -ge 2 ] || usage "--need requires a value"; NEED="$2"; shift 2;;
    --op-id) [ $# -ge 2 ] || usage "--op-id requires a value"; OP_ID="$2"; shift 2;;
    -*) usage "unknown option '$1'";;
    *) [ -z "$IMG" ] || usage "unexpected argument '$1' before --"; IMG="$1"; shift;;
  esac
done
[ -n "$IMG" ] || usage "image id missing"
[ $# -ge 1 ] || usage "command missing after --"
case "$IMG" in IMG-[A-Z0-9]*) ;; *) usage "image id '$IMG' is not of the form IMG-<NAME>";; esac
case "$IMG" in *[!A-Za-z0-9-]*) usage "image id '$IMG' has a character outside A-Z a-z 0-9 -";; esac
CMD=("$@")
if [ -n "$NEED" ]; then valid_int "$NEED" || usage "--need '$NEED' is not a base-10 byte count"; fi
if [ -z "$OP_ID" ]; then OP_ID="runp-$(date -u +%Y%m%dT%H%M%SZ)-$$"; fi
case "$OP_ID" in ''|*[!A-Za-z0-9._-]*|.*) usage "op id '$OP_ID' must match ^[A-Za-z0-9_-][A-Za-z0-9._-]*\$";; esac
[ "${#OP_ID}" -le 128 ] || usage "op id longer than 128"
for _d in python3 realpath; do command -v "$_d" >/dev/null 2>&1 || refuse dependency_missing "$_d is required"; done
# test hooks replace a real reading (meminfo, RLIMIT_NPROC): only a declared test run may use them, so a stray exported variable can never lift a ceiling
if { [ -n "${RUNP_MEMINFO+x}" ] || [ -n "${RUNP_ULIMIT_U+x}" ]; } && [ "${RUNP_TEST_MODE:-}" != 1 ]; then   # MUT:test-hooks
  refuse test_hook_outside_test_mode "RUNP_MEMINFO / RUNP_ULIMIT_U are test hooks and need RUNP_TEST_MODE=1"
fi

# ---- lock entry (python3 + PyYAML; one entry by exact id) ----
[ -r "$LOCK" ] || refuse lock_unreadable "$LOCK"
ENTRY="$(python3 - "$LOCK" "$IMG" <<'PY'
import sys
try:
    import yaml
    d = yaml.safe_load(open(sys.argv[1]))
except Exception as e:
    print("ERR\tlock_unreadable"); sys.exit(0)
imgs = d.get("images") if isinstance(d, dict) else None
if not isinstance(imgs, list):
    print("ERR\tlock_unreadable"); sys.exit(0)
m = [i for i in imgs if isinstance(i, dict) and i.get("id") == sys.argv[2]]
if not m: print("ERR\timage_id_unknown"); sys.exit(0)
if len(m) > 1: print("ERR\tlock_duplicate_id"); sys.exit(0)
e = m[0]
for k in ("reference", "digest", "entrypoint_override", "size_bytes"):
    v = e.get(k)
    print("%s\t%s" % (k, "" if v is None else str(v).replace("\n", " ").replace("\t", " ")))
PY
)" || refuse lock_unreadable "python3 failed"
case "$ENTRY" in ERR*) refuse "$(printf '%s' "$ENTRY" | head -1 | cut -f2)" "id=$IMG lock=$LOCK";; esac
REF=""; DIGEST=""; EPO=""; SIZE=""
while IFS=$'\t' read -r k v; do
  case "$k" in reference) REF="$v";; digest) DIGEST="$v";; entrypoint_override) EPO="$v";; size_bytes) SIZE="$v";; esac
done <<<"$ENTRY"
# reference: a repository path only. No tag, no digest, no whitespace, no leading '-' (it would be parsed as a podman flag).
[ -n "$REF" ] || refuse reference_missing "id=$IMG"
case "$REF" in -*|*[[:space:]]*|*@*) refuse reference_malformed "id=$IMG";; esac
case "${REF##*/}" in *:*) refuse tag_only_reference "id=$IMG reference carries a tag";; esac
[ -n "$DIGEST" ] || refuse tag_only_reference "id=$IMG has no digest (UNPINNED)"   # MUT:tag-only
case "$DIGEST" in sha256:*) ;; *) refuse digest_malformed "id=$IMG";; esac   # MUT:digest-format
_hex="${DIGEST#sha256:}"
{ [ "${#_hex}" -eq 64 ] && [ -z "${_hex//[0-9a-f]/}" ]; } || refuse digest_malformed "id=$IMG digest is not sha256:<64 lowercase hex>"   # MUT:digest-format
IMAGE_REF="$REF@$DIGEST"
if [ -n "$EPO" ]; then
  case "$EPO" in /*) ;; *) refuse entrypoint_override_malformed "id=$IMG";; esac
  _w="${CMD[0]}"
  if [ "$_w" != "$EPO" ] && [ "$_w" != "${EPO##*/}" ]; then refuse entrypoint_word_mismatch "id=$IMG first command word '$_w' does not name the entrypoint '$EPO'"; fi
fi
if [ -z "$NEED" ]; then if [ -n "$SIZE" ]; then valid_int "$SIZE" || refuse lock_size_malformed "id=$IMG"; NEED="$SIZE"; else NEED=0; fi; fi

# ---- paths: secret store, --rw allow-list, --out ----
PWD_REAL="$(realpath -- "$PWD")"
case "$PWD$PWD_REAL" in *:*) refuse source_path_malformed "the checkout path ($PWD, resolved $PWD_REAL) contains ':' and cannot be a podman bind source";; esac   # MUT:pwd-colon
case "$PWD$PWD_REAL" in *$'\n'*) refuse source_path_malformed "the checkout path contains a newline: refused by policy so the one-element-per-line argv (the print-mode oracle) stays unambiguous (podman itself would bind it)";; esac   # MUT:pwd-newline
STATE_DIR="${XDG_STATE_HOME:-$HOME/.local/state}/catalogizer"
STATE_REAL="$(realpath -m -- "$STATE_DIR")"
# overlap(a, b): true when a equals b or one lies under the other (the secret store must never be inside a mount, nor contain one)
overlaps() { local a="${1%/}" b="${2%/}"; [ "$a" = "$b" ] && return 0; case "$a/" in "$b"/*) return 0;; esac; case "$b/" in "$a"/*) return 0;; esac; return 1; }
if overlaps "$PWD_REAL" "$STATE_REAL"; then refuse secret_state_in_mount "the source mount $PWD_REAL and the state directory $STATE_REAL overlap"; fi   # MUT:secret
RW_BIND=""
if [ -n "$RW" ]; then
  case "$RW" in
    docs|.audit/scratch) ;;   # MUT:rw-allowlist  (the allow-list is these two exact strings, compared before any path resolution)
    *) refuse rw_value_not_allowed "'$RW'";;
  esac
  _src="$PWD/$RW"
  if [ -e "$_src" ] || [ -L "$_src" ]; then
    [ ! -L "$_src" ] || refuse rw_source_not_canonical "$RW is a symlink"   # MUT:symlink
    [ "$(realpath -- "$_src")" = "$PWD_REAL/$RW" ] || refuse rw_source_not_canonical "$RW resolves outside $PWD_REAL/$RW"   # MUT:symlink
  else
    # not yet created: its existing parents must still resolve canonically (a symlinked .audit would redirect the later mkdir)
    [ "$(realpath -m -- "$_src")" = "$PWD_REAL/$RW" ] || refuse rw_source_not_canonical "$RW would resolve outside $PWD_REAL/$RW"
  fi
  overlaps "$_src" "$STATE_REAL" && refuse secret_state_in_mount "--rw $RW overlaps the state directory"
  RW_BIND="$_src:/src/$RW:rw"
fi
if [ -n "$OUT" ]; then
  case "$OUT" in *:*|*$'\n'*|-*) refuse out_dir_malformed "$(printf '%q' "$OUT") (a ':' is the podman volume separator, a newline is refused by policy to keep the one-element-per-line argv unambiguous, a leading '-' is an option)";; esac
  case "$OUT" in /*) ;; *) refuse out_dir_not_absolute "'$OUT' (a relative --out would be taken by podman as a named volume)";; esac   # MUT:out-absolute
else
  OUT="$PWD/.audit/out/$OP_ID"
fi
_o="$(realpath -m -- "$OUT")"
# the RESOLVED path is what podman sees: ':' there would be read as a volume separator (the raw-value check above cannot see a symlink target);
# a newline is refused by policy (podman binds it; the one-per-line argv oracle would be ambiguous). Space and TAB are fine (probed with real podman, round 6, F1). The default out
# ($PWD/.audit/out/<op id>) is covered too; its only way in is the checkout path, which source_path_malformed has already named.
case "$_o" in *:*|*$'\n'*) refuse out_dir_malformed "the resolved out directory $(printf '%q' "$_o") contains ':' (the podman volume separator) or a newline (refused by policy, one argv element per line)";; esac   # MUT:out-resolved
# the bound path is the resolved one; the secret store must be outside it
OUT="$_o"
if overlaps "$_o" "$STATE_REAL"; then refuse secret_state_in_mount "--out $OUT overlaps the state directory $STATE_REAL"; fi   # MUT:secret
# --out inside or above the source tree would make source paths writable behind the --rw allow-list: only the sanctioned output roots
if overlaps "$_o" "$PWD_REAL"; then
  _ok=0
  for _root in .audit/out .audit/scratch specs/001-full-project-audit-remediation/evidence; do
    case "$_o/" in "$PWD_REAL/$_root"/*) _ok=1;; esac
  done
  [ "$_ok" = 1 ] || refuse out_dir_in_source "--out $_o is inside or above the source tree $PWD_REAL and not under a sanctioned output root"   # MUT:out-allowlist
fi

# ---- limits ----
mem_kb() { awk -v k="$1" '$1==k":" {print $2}' "${RUNP_MEMINFO:-/proc/meminfo}" 2>/dev/null; }
mt="$(mem_kb MemTotal)"; ma="$(mem_kb MemAvailable)"
{ valid_int "$mt" && valid_int "$ma"; } || refuse meminfo_unreadable "MemTotal/MemAvailable"
ceil=$(( mt * 1024 * 60 / 100 ))
if [ -n "${RUNP_MEMORY:-}" ]; then
  valid_int "$RUNP_MEMORY" || usage "RUNP_MEMORY must be a byte count"
  { [ "$RUNP_MEMORY" -ge 1 ] && [ "$RUNP_MEMORY" -le "$ceil" ]; } || refuse memory_override_out_of_bounds "RUNP_MEMORY=$RUNP_MEMORY must be 1..$ceil (0.60 * MemTotal)"   # MUT:memory-bounds
  MEM="$RUNP_MEMORY"
else
  res=$(( mt * 1024 * 15 / 100 )); [ "$res" -ge 4294967296 ] || res=4294967296   # MUT:reserve
  avail=$(( ma * 1024 - res )); MEM=$(( ceil < avail ? ceil : avail ))
  [ "$MEM" -ge 536870912 ] || refuse memory_budget_unavailable "budget $MEM bytes is below 512 MiB (MemAvailable ${ma} kB)"
fi
NPROC="$(nproc 2>/dev/null)"; valid_int "$NPROC" && [ "$NPROC" -ge 1 ] || refuse cpu_budget_unavailable "nproc"
CPU_CEIL=$(( NPROC * 60 / 100 )); [ "$CPU_CEIL" -ge 1 ] || CPU_CEIL=1
if [ -n "${RUNP_CPUS:-}" ]; then
  case "$RUNP_CPUS" in [1-9]|[1-9][0-9]|[1-9][0-9][0-9]) ;; *) usage "RUNP_CPUS must be a positive integer";; esac
  [ "$RUNP_CPUS" -le "$CPU_CEIL" ] || refuse cpus_override_out_of_bounds "RUNP_CPUS=$RUNP_CPUS must be 1..$CPU_CEIL (0.60 * nproc $NPROC)"   # MUT:cpus-bounds
  CPUS="$RUNP_CPUS"
else
  CPUS=2; [ "$CPUS" -le "$CPU_CEIL" ] || CPUS="$CPU_CEIL"
fi
# pids ceiling (12.12): the container's processes count against the operator's own RLIMIT_NPROC, so the ceiling is the smaller of the
# absolute 8192 and half of `ulimit -u` (RUNP_ULIMIT_U is the test hook, honoured only under RUNP_TEST_MODE=1); the default 2048 is clamped to it.
# Fail closed: an unreadable, zero or unparsable limit is a refusal, never the permissive 8192 (11.4.201(4)).
PIDS_CEIL=8192
UL="${RUNP_ULIMIT_U-$(ulimit -u 2>/dev/null)}"
{ [ "$UL" = unlimited ] || { valid_int "$UL" && [ "$UL" -gt 0 ]; }; } || refuse pids_budget_unavailable "ulimit -u reads '$UL': neither 'unlimited' nor a positive integer"   # MUT:ulimit-closed
if [ "$UL" != unlimited ] && [ $(( UL / 2 )) -lt "$PIDS_CEIL" ]; then PIDS_CEIL=$(( UL / 2 )); fi
[ "$PIDS_CEIL" -ge 1 ] || PIDS_CEIL=1   # MUT:pids-floor
if [ -n "${RUNP_PIDS:-}" ]; then
  valid_int "$RUNP_PIDS" && [ "$RUNP_PIDS" != 0 ] || usage "RUNP_PIDS must be a positive integer"   # MUT:pids-guard
  [ "$RUNP_PIDS" -le "$PIDS_CEIL" ] || refuse pids_override_out_of_bounds "RUNP_PIDS=$RUNP_PIDS must be 1..$PIDS_CEIL (min of 8192 and half of ulimit -u)"   # MUT:pids-ceiling
  PIDS="$RUNP_PIDS"
else
  PIDS=2048; [ "$PIDS" -le "$PIDS_CEIL" ] || PIDS="$PIDS_CEIL"
fi
# the container user: the mapped host uid:gid unless the documented explicit override is given
if [ -n "${RUNP_USER:-}" ]; then
  case "$RUNP_USER" in
    *[!0-9:]*|:*|*:|*:*:*|*[!0-9]) refuse user_override_malformed "RUNP_USER must be <uid>:<gid>, digits only";;
    *:*) CUSER="$RUNP_USER";;
    *) refuse user_override_malformed "RUNP_USER must be <uid>:<gid>, digits only";;
  esac
else CUSER="$(id -u):$(id -g)"; fi

# ---- disk headroom precondition (T001); its stdout goes to stderr so print mode stays one element per line ----
[ -x "$HEADROOM" ] || refuse headroom_script_missing "$HEADROOM"
HR_ERR="$(bash "$HEADROOM" --need "$NEED" --op-id "$OP_ID" 2>&1 >/dev/null)"; HR_RC=$?   # MUT:headroom
if [ "$HR_RC" -ne 0 ]; then
  _r="$(printf '%s' "$HR_ERR" | grep -o 'reason=[A-Za-z0-9_]*' | head -1 | cut -d= -f2)"
  echo "run_pinned: disk headroom precondition failed: $HR_ERR" >&2
  refuse "${_r:-disk_headroom_failed}" "(disk_headroom exit $HR_RC)"
fi

# ---- compose argv ----
A=(podman run --rm
   --pull=never   # MUT:pull
   --userns=keep-id
   --user "$CUSER"   # MUT:user
   --cap-drop=ALL
   --security-opt no-new-privileges   # MUT:secopt
   --memory "$MEM" --memory-swap "$MEM" --pids-limit "$PIDS" --cpus "$CPUS"   # MUT:limits
   --label project=catalogizer --label "catalogizer.op_id=$OP_ID"
   --tmpfs /tmp:rw,mode=1777
   -e HOME=/tmp -e XDG_CACHE_HOME=/tmp/.cache -e GOCACHE=/tmp/.cache/go-build -e GOMODCACHE=/tmp/go/pkg/mod   # MUT:cacheenv
   -v "$PWD:/src:ro"
   --workdir /src   # MUT:workdir
)
[ -z "$RW_BIND" ] || A+=(-v "$RW_BIND")
A+=(-v "$OUT:/out:rw")
[ "$NET" != none ] || A+=(--network=none)
if [ -n "$EPO" ]; then A+=(--entrypoint "$EPO" -- "$IMAGE_REF" "${CMD[@]:1}")   # MUT:entrypoint
else A+=(-- "$IMAGE_REF" "${CMD[@]}"); fi

if [ "${RUNP_PRINT_ARGV:-}" = 1 ]; then printf '%s\n' "${A[@]}"; exit 0; fi

# ---- run mode: image must be present locally (digest-addressed, never pulled here), then create dirs and exec ----
podman image inspect --format '{{.Id}}' -- "$IMAGE_REF" >/dev/null 2>&1 || refuse image_not_present_locally "$IMAGE_REF (pull it through T005d, never here)"
[ -z "$RW" ] || mkdir -p -- "$PWD/$RW"
mkdir -p -- "$_o" || refuse out_dir_uncreatable "$_o"
exec "${A[@]}"
