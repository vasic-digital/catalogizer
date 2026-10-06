#!/usr/bin/env bash
# test_run_pinned.sh - T003 (RUNP launcher). Control-plane launcher, so it runs on the host.
# Two independent oracles must agree: (1) RUNP_PRINT_ARGV=1 (the composed `podman run` argv, one element per line) and (2) a
# `podman` shim first on PATH that logs its argv and exits 0. df/podman-info shims drive the disk-headroom precondition.
# Paired mutations: copies of run_pinned.sh with one `# MUT:<name>` line removed; the test body is re-run against each copy
# (RUNP_SUT=<copy>, RUNP_TEST_MUTANT=1) and every copy must make it FAIL. Results go to $RUNP_MUTATION_RECORD (default: scratch).
# Usage: test_run_pinned.sh            run the tests and the mutations
#        RUNP_TEST_NO_MUTATIONS=1 ...  tests only
# Exit non-zero on any failure.
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO="$(cd "$HERE/../../.." && pwd)"
SUT="${RUNP_SUT:-$HERE/../run_pinned.sh}"
FAILS=0; PASSES=0
ok()  { PASSES=$((PASSES+1)); [ "${QUIET:-0}" = 1 ] || echo "PASS: $1"; }
bad() { FAILS=$((FAILS+1)); echo "FAIL: $1"; }
check() { if [ "$2" = "$3" ]; then ok "$1"; else bad "$1 (got '$2' want '$3')"; fi; }
for d in jq python3 realpath; do command -v "$d" >/dev/null 2>&1 || { echo "FAIL: $d is required by this test"; exit 2; }; done
[ -f "$SUT" ] || { echo "FAIL: run_pinned.sh not found at $SUT"; exit 1; }

T="$(mktemp -d "${TMPDIR:-/tmp}/runp-test.XXXXXX")"
trap 'rm -rf "$T"' EXIT
D1="sha256:$(printf 'a%.0s' $(seq 64))"; D2="sha256:$(printf 'b%.0s' $(seq 64))"
SHIMS="$T/shims"; mkdir -p "$SHIMS"
# podman shim: logs every call (calls separated by a marker line); `image inspect` succeeds only for digests in SHIM_KNOWN;
# `info` answers the graphroot for disk_headroom.sh; everything else (run) exits 0.
cat >"$SHIMS/podman" <<'SH'
#!/usr/bin/env bash
{ echo "---CALL---"; printf '%s\n' "$@"; } >>"${SHIM_LOG:?}"
case "$*" in
  *GraphRoot*) echo "${SHIM_GRAPHROOT:?}"; exit 0;;
  "image inspect"*) for k in ${SHIM_KNOWN:-}; do case "$*" in *"@$k"*) exit 0;; esac; done; exit 1;;
esac
exit 0
SH
cat >"$SHIMS/df" <<'SH'
#!/usr/bin/env bash
if [ "$#" -ne 3 ] || [ "$1" != "-B1" ] || [ "$2" != "--output=avail" ]; then echo "df shim: unexpected arguments: $*" >&2; exit 1; fi
echo Avail; echo "${SHIM_DF:?}"
SH
cat >"$SHIMS/nproc" <<'SH'
#!/usr/bin/env bash
echo "${SHIM_NPROC:-64}"
SH
chmod +x "$SHIMS/podman" "$SHIMS/df" "$SHIMS/nproc"

# a scratch "checkout": cwd of every run, with docs/ and .audit/scratch/ real directories
mkcheckout() { rm -rf "$1"; mkdir -p "$1/docs" "$1/.audit/scratch"; }
CK="$T/home/work/checkout"; mkcheckout "$CK"
export HOME="$T/home"; unset XDG_STATE_HOME
STATE="$HOME/.local/state/catalogizer"; mkdir -p "$STATE"; echo "needle" >"$STATE/secret.needle"
GRAPH="$T/graphroot"; mkdir -p "$GRAPH"
export SHIM_LOG="$T/podman.log" SHIM_GRAPHROOT="$GRAPH" SHIM_DF=900000000000 SHIM_KNOWN="$D1"
export DISK_HEADROOM_OUT_DIR="$T/disk"; mkdir -p "$DISK_HEADROOM_OUT_DIR"
LOCKF="$T/lock.yaml"
mklock() { # $1 digest  [$2 extra yaml lines]
  cat >"$LOCKF" <<EOF
schema: 1
images:
- id: IMG-TOOL
  reference: docker.io/example/tool
  tag_intent: "1"
  digest: "$1"
  size_bytes: 1000
${2:-}
- id: IMG-SCR
  reference: docker.io/example/scratch
  tag_intent: stable
  digest: "$D1"
  entrypoint_override: /bin/shellcheck
EOF
}
mklock "$D1"
printf 'MemTotal:       32000000 kB\nMemAvailable:   30000000 kB\n' >"$T/meminfo"
# round 6 (F2): the two test hooks (RUNP_MEMINFO, RUNP_ULIMIT_U) are honoured only under RUNP_TEST_MODE=1
export RUNP_TEST_MODE=1 RUNP_MEMINFO="$T/meminfo" RUNP_LOCK="$LOCKF"

# runp <args...>: print-argv mode in the checkout, stdout -> $T/argv, stderr -> $T/err, sets RC
runp() { ( cd "$CK" && PATH="$SHIMS:$PATH" RUNP_PRINT_ARGV=1 bash "$SUT" "$@" ) >"$T/argv" 2>"$T/err"; RC=$?; }
# runrun <args...>: real run mode with the podman shim, so the shim log is the second oracle
runrun() { : >"$SHIM_LOG"; ( cd "$CK" && PATH="$SHIMS:$PATH" bash "$SUT" "$@" ) >"$T/out" 2>"$T/err"; RC=$?; }
argv_has() { grep -qxF -- "$1" "$T/argv"; }
idx() { grep -nxF -- "$1" "$T/argv" | head -1 | cut -d: -f1; }
no_podman_run() { ! grep -qx 'run' "$SHIM_LOG" 2>/dev/null; }
# elements of the last shim `run` call, one per line (without the leading marker and `run`)
shim_run_argv() { awk '/^---CALL---$/{buf=""; next} {buf=buf $0 "\n"} END{printf "%s", buf}' "$SHIM_LOG"; }

# ---------- evidence root: no repo-root evidence path in the launcher (governance R2) ----------
EVSEG="evidence"; EVSEG="$EVSEG/"
check "the launcher names no unrooted evidence path" "$(grep -cE "(^|[^A-Za-z0-9_./}\$])$EVSEG" "$SUT")" 0

# ---------- argument handling ----------
runp; check "usage: no arguments is a usage error" "$RC" 2
runp IMG-TOOL; check "usage: no command after the image is a usage error" "$RC" 2
runp --bogus IMG-TOOL -- true; check "usage: unknown option is a usage error" "$RC" 2
runp 'bad id' -- true; check "usage: image id must be IMG-<NAME>" "$RC" 2
runp IMG-NOPE -- true; check "unknown lock id is refused" "$RC" 1
grep -q 'reason=image_id_unknown' "$T/err" && ok "unknown lock id names reason image_id_unknown" || bad "unknown lock id reason: $(cat "$T/err")"

# ---------- the composed argv ----------
runp --op-id t1 IMG-TOOL -- echo hello world
check "print-argv mode exits 0" "$RC" 0
check "argv starts with podman run" "$(sed -n 1,2p "$T/argv" | tr '\n' ' ')" "podman run "
for f in --rm --userns=keep-id --cap-drop=ALL --memory --memory-swap --pids-limit --cpus --workdir; do argv_has "$f" && ok "argv carries $f" || bad "argv lacks $f"; done
check "memory and memory-swap are equal (no swap)" "$(sed -n "$(( $(idx --memory) + 1 ))p" "$T/argv")" "$(sed -n "$(( $(idx --memory-swap) + 1 ))p" "$T/argv")"
check "memory is the formula result: min(0.6 MemTotal, MemAvailable - 4.8 GB)" "$(sed -n "$(( $(idx --memory) + 1 ))p" "$T/argv")" "$(( 32000000*1024*60/100 ))"
check "--workdir is /src" "$(sed -n "$(( $(idx --workdir) + 1 ))p" "$T/argv")" /src
argv_has "$CK:/src:ro" && ok "source is mounted read-only" || bad "no read-only source mount; argv: $(tr '\n' ' ' <"$T/argv")"
argv_has "project=catalogizer" && ok "label project=catalogizer present" || bad "project label missing"
check "image reference is digest-pinned, preceded by --" "$(sed -n "$(( $(idx docker.io/example/tool@$D1) - 1 ))p" "$T/argv")" "--"
check "command words follow the image" "$(awk "NR>$(idx docker.io/example/tool@$D1)" "$T/argv" | tr '\n' ' ')" "echo hello world "
check "no podman call in print mode" "$(cat "$SHIM_LOG" 2>/dev/null | grep -c '^run$')" 0
# the second oracle: real run mode through the podman shim composes the same argv
SHIM_KNOWN="$D1" runrun --op-id t1 IMG-TOOL -- echo hello world
check "run mode exits 0 with the shim" "$RC" 0
shim_run_argv >"$T/shim_argv"
runp --op-id t1 IMG-TOOL -- echo hello world
check "both oracles agree: shim argv equals print argv minus the leading podman" "$(tail -n +2 "$T/argv" | md5sum)" "$(md5sum <"$T/shim_argv")"
check "out directory created in run mode" "$([ -d "$CK/.audit/out/t1" ] && echo yes || echo no)" yes
check "default out bind is .audit/out/<op_id>" "$(grep -c "^$CK/.audit/out/t1:/out:rw\$" "$T/argv")" 1
# a limit missing from the argv is detected by a checker that the mutation sections below prove can fail
runp --op-id t1 IMG-TOOL -- true; for f in --memory --memory-swap --pids-limit; do argv_has "$f" || bad "mutation oracle: $f absent"; done

# ---------- pinning: mutated digest, tag-only ----------
for bad_dig in "sha256:${D1#sha256:}0" "sha256:${D1:7:63}" "sha256:$(printf 'g%.0s' $(seq 64))" "sha256:$(printf 'A%.0s' $(seq 64))" "md5:abc" ""; do
  mklock "$bad_dig"; : >"$SHIM_LOG"; runp IMG-TOOL -- true
  check "mutated digest '${bad_dig:0:20}...' is refused (print mode)" "$RC" 1
  check "  ...and no podman run was composed" "$(wc -c <"$T/argv")" 0
  runrun IMG-TOOL -- true; check "mutated digest refused in run mode" "$RC" 1; no_podman_run && ok "  ...no podman run in the shim log" || bad "  podman run reached the shim"
done
mklock "$D1" "  reference_note: x"; sed -i 's#reference: docker.io/example/tool$#reference: docker.io/example/tool:1.2#' "$LOCKF"
runp IMG-TOOL -- true; check "a tag-only reference (tag in reference) is refused" "$RC" 1
grep -q 'reason=tag_only_reference' "$T/err" && ok "tag reference names reason tag_only_reference" || bad "reason: $(cat "$T/err")"
mklock ""; runp IMG-TOOL -- true; check "an empty digest (UNPINNED) is refused" "$RC" 1
grep -q 'reason=tag_only_reference' "$T/err" && ok "empty digest names reason tag_only_reference" || bad "reason: $(cat "$T/err")"
mklock "$D2"; runp IMG-TOOL -- true; check "a well-formed digest passes print mode (format check only)" "$RC" 0
SHIM_KNOWN="$D1" runrun IMG-TOOL -- true; check "a well-formed digest not present locally is refused in run mode" "$RC" 1
grep -q 'reason=image_not_present_locally' "$T/err" && ok "names reason image_not_present_locally" || bad "reason: $(cat "$T/err")"
no_podman_run && ok "  ...no podman run in the shim log" || bad "  podman run reached the shim"
mklock "$D1"

# ---------- entrypoint_override (an image built FROM scratch) ----------
runp IMG-SCR -- shellcheck x.sh; check "entrypoint_override image composes" "$RC" 0
check "--entrypoint carries the override" "$(sed -n "$(( $(idx --entrypoint) + 1 ))p" "$T/argv")" /bin/shellcheck
check "leading command word dropped: image followed only by the arguments" "$(awk "NR>$(idx docker.io/example/scratch@$D1)" "$T/argv" | tr '\n' ' ')" "x.sh "
SHIM_KNOWN="$D1" runrun IMG-SCR -- shellcheck x.sh; shim_run_argv >"$T/shim_argv"
check "shim agrees: shellcheck word occurs exactly once (as the entrypoint, never as a command)" "$(grep -c '^shellcheck$' "$T/shim_argv")" 0
check "shim agrees: the entrypoint element is present once" "$(grep -c '^/bin/shellcheck$' "$T/shim_argv")" 1
runp IMG-SCR -- bash -c x; check "a command word that does not name the entrypoint is refused" "$RC" 1
grep -q 'reason=entrypoint_word_mismatch' "$T/err" && ok "names reason entrypoint_word_mismatch" || bad "reason: $(cat "$T/err")"
runp IMG-TOOL -- shellcheck x; argv_has --entrypoint && bad "an image without entrypoint_override must not get --entrypoint" || ok "no --entrypoint without entrypoint_override"

# ---------- disk headroom precondition ----------
rm -f "$DISK_HEADROOM_OUT_DIR"/*; SHIM_DF=1000 runp --op-id hr1 IMG-TOOL -- true
check "headroom refusal refuses the run" "$RC" 1
grep -q 'reason=disk_below_headroom' "$T/err" && ok "the disk_below_headroom reason is passed through" || bad "reason: $(cat "$T/err")"
check "  ...nothing composed" "$(wc -c <"$T/argv")" 0
SHIM_DF=1000 runrun --op-id hr1 IMG-TOOL -- true; check "headroom refusal in run mode" "$RC" 1; no_podman_run && ok "  ...no podman run" || bad "  podman run reached the shim"
SHIM_DF=900000000000 runp --op-id hr2 IMG-TOOL -- true; check "ample free space passes" "$RC" 0
check "DISK_HEADROOM_OUT_DIR is passed through: record written there" "$([ -s "$DISK_HEADROOM_OUT_DIR/hr2.json" ] && echo yes || echo no)" yes
check "the lock entry's size_bytes is the headroom need" "$(jq -r .need_bytes "$DISK_HEADROOM_OUT_DIR/hr2.json" 2>/dev/null)" 1000
runp --op-id hr3 --need 777 IMG-TOOL -- true; check "--need overrides the lock size" "$(jq -r .need_bytes "$DISK_HEADROOM_OUT_DIR/hr3.json" 2>/dev/null)" 777

# ---------- --rw allow-list ----------
runp --op-id rw0 IMG-TOOL -- true
check "without --rw no writable bind except /out" "$(grep -c ':rw$' "$T/argv")" 1
runp --op-id rw1 --rw docs IMG-TOOL -- true; check "--rw docs composes" "$RC" 0
check "--rw docs adds exactly one extra writable bind" "$(grep -c ':rw$' "$T/argv")" 2
argv_has "$CK/docs:/src/docs:rw" && ok "the docs bind is \$PWD/docs:/src/docs:rw" || bad "docs bind missing"
[ "$(idx "$CK/docs:/src/docs:rw")" -gt "$(idx "$CK:/src:ro")" ] && ok "the rw bind comes after the read-only source mount" || bad "rw bind precedes the source mount"
runp --op-id rw2 --rw .audit/scratch IMG-TOOL -- true; check "--rw .audit/scratch composes" "$RC" 0
argv_has "$CK/.audit/scratch:/src/.audit/scratch:rw" && ok "the scratch bind is \$PWD/.audit/scratch:/src/.audit/scratch:rw" || bad "scratch bind missing"
for v in .audit .audit/scratch/x .audit/scratch/.. ./docs /etc /abs/path docs/ ../docs "docs/../.audit" ".audit/scratch/"; do
  runp --rw "$v" IMG-TOOL -- true; check "--rw '$v' is refused" "$RC" 1; check "  ...nothing composed" "$(wc -c <"$T/argv")" 0
  runrun --rw "$v" IMG-TOOL -- true; no_podman_run && ok "  ...no podman run in the shim log" || bad "  podman run reached the shim"
done
runp --rw docs --rw .audit/scratch IMG-TOOL -- true; check "a second --rw is refused" "$RC" 1
# symlinks (round-28 review m3)
mkcheckout "$CK"; rm -rf "$CK/.audit/scratch"; ln -s "$CK/docs" "$CK/.audit/scratch"
runp --rw .audit/scratch IMG-TOOL -- true; check "a .audit/scratch symlink to docs is refused" "$RC" 1
grep -q 'reason=rw_source_not_canonical' "$T/err" && ok "names reason rw_source_not_canonical" || bad "reason: $(cat "$T/err")"
runrun --rw .audit/scratch IMG-TOOL -- true; no_podman_run && ok "  ...no podman run in the shim log" || bad "  podman run reached the shim"
mkcheckout "$CK"; rm -rf "$CK/.audit"; mkdir -p "$T/elsewhere/scratch"; ln -s "$T/elsewhere" "$CK/.audit"
runp --rw .audit/scratch IMG-TOOL -- true; check "a symlinked .audit component is refused" "$RC" 1
grep -q 'reason=rw_source_not_canonical' "$T/err" && ok "names reason rw_source_not_canonical (parent symlink)" || bad "reason: $(cat "$T/err")"
mkcheckout "$CK"; ln -sfn "$CK/docs" "$T/docs-link"; rm -rf "$CK/docs"; ln -s "$T/docs-link" "$CK/docs"
runp --rw docs IMG-TOOL -- true; check "a docs symlink is refused" "$RC" 1
mkcheckout "$CK"; runp --rw .audit/scratch IMG-TOOL -- true; check "the real directories pass" "$RC" 0

# ---------- --out ----------
OD="$T/outdir"; mkdir -p "$OD"; runp --out "$OD" IMG-TOOL -- true; check "--out composes" "$RC" 0
argv_has "$OD:/out:rw" && ok "--out <dir> composes exactly -v <dir>:/out:rw" || bad "out bind missing"
check "--out replaces the default out bind (one /out mount)" "$(grep -c ':/out:rw$' "$T/argv")" 1

# ---------- secret store isolation (T005b) ----------
srcs() { awk 'prev=="-v"{print} {prev=$0}' "$T/argv" | sed 's/:[^:]*:[^:]*$//' ; }
leaks() { local n="$1" s; while read -r s; do [ -n "$s" ] || continue; case "$n" in "$s"|"$s"/*) echo "$s"; return 0;; esac; done; return 1; }
# control: the leak detector must see a source that contains the needle
printf '%s\n' "$STATE" | leaks "$STATE/secret.needle" >/dev/null && ok "secret control: a mount of the state directory is detected" || bad "secret control: detector blind"
runp --op-id s1 IMG-TOOL -- true; check "default run composes" "$RC" 0
srcs | leaks "$STATE/secret.needle" >/dev/null && bad "the needle at the secret location is inside a mount source (print mode)" || ok "needle absent from every mount source (print mode)"
SHIM_KNOWN="$D1" runrun --op-id s1 IMG-TOOL -- true; shim_run_argv | awk 'prev=="-v"{print} {prev=$0}' | sed 's/:[^:]*:[^:]*$//' | leaks "$STATE/secret.needle" >/dev/null && bad "needle inside a mount source (shim log)" || ok "needle absent from every mount source (shim log)"
runp --out "$STATE/x" IMG-TOOL -- true; check "--out under the state directory is refused" "$RC" 1
runp --out "$STATE" IMG-TOOL -- true; check "--out equal to the state directory is refused" "$RC" 1
runp --out "$HOME/.local/state/catalogizer/../catalogizer/y" IMG-TOOL -- true; check "--out reaching the state directory through .. is refused" "$RC" 1
runp --out "$HOME/.local" IMG-TOOL -- true; check "--out containing the state directory is refused" "$RC" 1
mkdir -p "$HOME/co2/docs" "$HOME/co2/.audit/scratch"; ( cd "$HOME" && PATH="$SHIMS:$PATH" RUNP_PRINT_ARGV=1 bash "$SUT" IMG-TOOL -- true ) >"$T/argv" 2>"$T/err"; RC=$?
check "a checkout at \$HOME (state directory under the source mount) is refused" "$RC" 1
grep -q 'reason=secret_state_in_mount' "$T/err" && ok "names reason secret_state_in_mount" || bad "reason: $(cat "$T/err")"
( cd "$HOME" && PATH="$SHIMS:$PATH" XDG_STATE_HOME="$HOME/co2/state" RUNP_PRINT_ARGV=1 bash "$SUT" IMG-TOOL -- true ) >"$T/argv" 2>"$T/err"; RC=$?
check "a state directory placed inside the checkout is refused" "$RC" 1
( cd "$CK" && PATH="$SHIMS:$PATH" XDG_STATE_HOME="$T/otherstate" RUNP_PRINT_ARGV=1 bash "$SUT" IMG-TOOL -- true ) >"$T/argv" 2>"$T/err"; check "XDG_STATE_HOME elsewhere does not refuse" "$?" 0

# ---------- --network=none ----------
runp --network=none IMG-TOOL -- true; argv_has --network=none && ok "--network=none composes the option" || bad "--network=none missing"
runp IMG-TOOL -- true; argv_has --network=none && bad "network option must be absent by default" || ok "no network option by default"

# ---------- RUNP_LOCK vs the tracked lock ----------
mklock "$D2"; runp IMG-TOOL -- true; argv_has "docker.io/example/tool@$D2" && ok "RUNP_LOCK: the fixture entry's digest is shown" || bad "fixture digest not shown"
( cd "$CK" && unset RUNP_LOCK; PATH="$SHIMS:$PATH" RUNP_PRINT_ARGV=1 bash "$SUT" IMG-SHELLCHECK -- shellcheck x ) >"$T/argv" 2>"$T/err"; RC=$?
TRACKED="$REPO/build/containers/images.lock.yaml"
if [ -f "$TRACKED" ]; then
  want="$(python3 -c 'import yaml,sys;print([i["digest"] for i in yaml.safe_load(open(sys.argv[1]))["images"] if i["id"]=="IMG-SHELLCHECK"][0])' "$TRACKED")"
  check "without RUNP_LOCK the tracked lock is read" "$(grep -c "@$want\$" "$T/argv")" 1
else bad "tracked lock $TRACKED is missing"; fi
mklock "$D1"; runp IMG-TOOL -- true
argv_has "docker.io/example/tool@$D2" && bad "tracked/other digest leaked with RUNP_LOCK set" || ok "with RUNP_LOCK set no other digest is shown"

# ---------- memory budget ----------
printf 'MemTotal:       32000000 kB\nMemAvailable:   4000000 kB\n' >"$T/meminfo"; runp IMG-TOOL -- true
check "MemAvailable below the reserve refuses (memory_budget_unavailable)" "$RC" 1
printf 'MemTotal:       32000000 kB\nMemAvailable:   30000000 kB\n' >"$T/meminfo"
RUNP_MEMORY=1073741824 runp IMG-TOOL -- true; check "RUNP_MEMORY overrides the formula" "$(sed -n "$(( $(idx --memory) + 1 ))p" "$T/argv")" 1073741824


# ---------- security options and environment of the composed argv (round 4: reviewer mutants R3-R5) ----------
prev_of() { local n; n="$(idx "$1")"; [ -n "$n" ] && sed -n "$((n-1))p" "$T/argv"; }
runp --op-id sec1 IMG-TOOL -- true
check "--security-opt is followed by no-new-privileges" "$(sed -n "$(( $(idx --security-opt) + 1 ))p" "$T/argv")" no-new-privileges
argv_has --pull=never && ok "argv carries --pull=never" || bad "argv lacks --pull=never"
check "--tmpfs carries /tmp:rw,mode=1777" "$(sed -n "$(( $(idx --tmpfs) + 1 ))p" "$T/argv")" "/tmp:rw,mode=1777"
for e in HOME=/tmp XDG_CACHE_HOME=/tmp/.cache GOCACHE=/tmp/.cache/go-build GOMODCACHE=/tmp/go/pkg/mod; do
  check "cache path set by environment: -e $e" "$(prev_of "$e")" "-e"
done

# ---------- container user: the mapped host uid, never the image's root (I2, root cause 3) ----------
runp --op-id u1 IMG-TOOL -- true
check "--user defaults to the host uid:gid" "$(sed -n "$(( $(idx --user) + 1 ))p" "$T/argv")" "$(id -u):$(id -g)"
argv_has --userns=keep-id && ok "--userns=keep-id is kept next to --user" || bad "--userns=keep-id lost"
check "exactly one --user element" "$(grep -cxF -- --user "$T/argv")" 1
RUNP_USER=0:0 runp --op-id u2 IMG-TOOL -- true; check "RUNP_USER=0:0 is the documented explicit root override" "$RC" 0
check "  ...and it is the value composed" "$(sed -n "$(( $(idx --user) + 1 ))p" "$T/argv")" "0:0"
RUNP_USER=4242:4343 runp --op-id u3 IMG-TOOL -- true; check "  ...an explicit uid:gid is composed" "$(sed -n "$(( $(idx --user) + 1 ))p" "$T/argv")" "4242:4343"
for bu in root 1000 1000: :1000 "1000:1000 --privileged" "1000:1000,extra" -1:-1 a:b "1000:1000
0:0"; do
  RUNP_USER="$bu" runp --op-id u4 IMG-TOOL -- true; check "RUNP_USER='$(printf '%s' "$bu" | tr '\n' '|')' is refused" "$RC" 1
  check "  ...nothing composed" "$(wc -c <"$T/argv")" 0
done
grep -q 'reason=user_override_malformed' "$T/err" && ok "names reason user_override_malformed" || bad "reason: $(cat "$T/err")"

# ---------- --out: no bypass of the --rw allow-list, resolved path bound (I3) ----------
mkcheckout "$CK"
runp --op-id o1 --out "$CK/docs" IMG-TOOL -- true; check "--out under docs (no --rw) is refused" "$RC" 1
grep -q 'reason=out_dir_in_source' "$T/err" && ok "names reason out_dir_in_source" || bad "reason: $(cat "$T/err")"
check "  ...nothing composed" "$(wc -c <"$T/argv")" 0
runp --out "$CK/docs/sub" IMG-TOOL -- true; check "--out below docs is refused" "$RC" 1
runp --out "$CK" IMG-TOOL -- true; check "--out equal to the source tree is refused" "$RC" 1
runp --out "$(dirname "$CK")" IMG-TOOL -- true; check "--out above the source tree (it would mount the tree writable) is refused" "$RC" 1
runp --out "$CK/.audit" IMG-TOOL -- true; check "--out equal to .audit (not a sanctioned root) is refused" "$RC" 1
runp --out "$CK/somewhere/else" IMG-TOOL -- true; check "--out at an arbitrary source path is refused" "$RC" 1
runp --out "$CK/.audit/out/../../docs" IMG-TOOL -- true; check "--out reaching docs through .. is refused" "$RC" 1
runp --out "$CK/.audit/out/o2" IMG-TOOL -- true; check "--out under .audit/out is accepted" "$RC" 0
runp --out "$CK/.audit/scratch/o3" IMG-TOOL -- true; check "--out under .audit/scratch is accepted" "$RC" 0
runp --out "$CK/specs/001-full-project-audit-remediation/evidence/wp09/o4" IMG-TOOL -- true; check "--out under the evidence tree is accepted" "$RC" 0
runp --out "$T/outdir/o5" IMG-TOOL -- true; check "--out outside the source tree is accepted" "$RC" 0
rm -rf "$CK/.audit/out"; ln -s "$CK/docs" "$CK/.audit/out"
runp --out "$CK/.audit/out/x" IMG-TOOL -- true; check "--out through an .audit/out symlink to docs is refused" "$RC" 1
rm -f "$CK/.audit/out"; mkcheckout "$CK"
for rel in relout ./relout ../relout .; do runp --out "$rel" IMG-TOOL -- true; check "relative --out '$rel' is refused" "$RC" 1; done
grep -q 'reason=out_dir_not_absolute' "$T/err" && ok "names reason out_dir_not_absolute" || bad "reason: $(cat "$T/err")"
check "  ...nothing composed" "$(wc -c <"$T/argv")" 0
mkdir -p "$T/outdir"; ln -sfn "$T/outdir" "$T/outlink"
runp --out "$T/outlink/y" IMG-TOOL -- true; check "--out through a symlink composes" "$RC" 0
argv_has "$T/outdir/y:/out:rw" && ok "the RESOLVED path is bound (not the raw spelling)" || bad "bind not resolved: $(grep :/out:rw "$T/argv")"
argv_has "$T/outlink/y:/out:rw" && bad "raw symlink spelling bound" || ok "raw spelling absent from argv"
runp --out "$T/outdir/../outdir/z" IMG-TOOL -- true; argv_has "$T/outdir/z:/out:rw" && ok ".. is resolved in the bind" || bad "bind not normalised"
SHIM_KNOWN="$D1" runrun --op-id o9 --out "$T/outlink/made" IMG-TOOL -- true
check "run mode creates the RESOLVED out directory" "$([ -d "$T/outdir/made" ] && echo yes || echo no)" yes
mkcheckout "$CK"

# ---------- limit overrides cannot lift the section 12.6 ceiling (I4) ----------
# meminfo: MemTotal 32000000 kB -> 60% ceiling = 19660800000 bytes
CEIL=$(( 32000000*1024*60/100 ))
for bm in 0 00 abc -1 1e9 "" 999999999999999999 "$((CEIL+1))"; do
  [ -n "$bm" ] || continue
  RUNP_MEMORY="$bm" runp --op-id m1 IMG-TOOL -- true; check "RUNP_MEMORY='$bm' is refused" "$([ "$RC" = 1 ] || [ "$RC" = 2 ] && echo refused || echo "rc=$RC")" refused
  check "  ...nothing composed" "$(wc -c <"$T/argv")" 0
done
RUNP_MEMORY=0 runp IMG-TOOL -- true; check "RUNP_MEMORY=0 (no limit) is a refusal, exit 1" "$RC" 1
grep -q 'reason=memory_override_out_of_bounds' "$T/err" && ok "names reason memory_override_out_of_bounds" || bad "reason: $(cat "$T/err")"
RUNP_MEMORY="$((CEIL+1))" runp IMG-TOOL -- true; check "RUNP_MEMORY one byte above the ceiling is refused, exit 1" "$RC" 1
RUNP_MEMORY="$CEIL" runp IMG-TOOL -- true; check "RUNP_MEMORY exactly at the ceiling is accepted" "$RC" 0
check "  ...and is the value composed" "$(sed -n "$(( $(idx --memory) + 1 ))p" "$T/argv")" "$CEIL"
RUNP_MEMORY=1 runp IMG-TOOL -- true; check "RUNP_MEMORY=1 (positive, under the ceiling) is accepted" "$RC" 0
# the reserve arm: MemAvailable 20000000 kB leaves 20480000000 - max(4 GiB, 15% of 32768000000 = 4915200000) = 15564800000
printf 'MemTotal:       32000000 kB\nMemAvailable:   20000000 kB\n' >"$T/meminfo"; runp IMG-TOOL -- true
check "memory is MemAvailable bound: MemAvailable - 15% reserve" "$(sed -n "$(( $(idx --memory) + 1 ))p" "$T/argv")" 15564800000
printf 'MemTotal:       16000000 kB\nMemAvailable:   9000000 kB\n' >"$T/meminfo"; runp IMG-TOOL -- true
check "the 4 GiB reserve floor binds on a small host (9216000000 - 4294967296)" "$(sed -n "$(( $(idx --memory) + 1 ))p" "$T/argv")" 4921032704
printf 'MemTotal:       32000000 kB\nMemAvailable:   30000000 kB\n' >"$T/meminfo"
# cpus: ceiling = 60% of nproc (shim)
SHIM_NPROC=10 runp IMG-TOOL -- true; check "default cpus on a 10-cpu host" "$(sed -n "$(( $(idx --cpus) + 1 ))p" "$T/argv")" 2
SHIM_NPROC=2 runp IMG-TOOL -- true; check "default cpus on a 2-cpu host is capped to the 60% ceiling (1)" "$(sed -n "$(( $(idx --cpus) + 1 ))p" "$T/argv")" 1
SHIM_NPROC=10 RUNP_CPUS=6 runp IMG-TOOL -- true; check "RUNP_CPUS at the ceiling (6 of 10) is accepted" "$RC" 0
SHIM_NPROC=10 RUNP_CPUS=7 runp IMG-TOOL -- true; check "RUNP_CPUS above the ceiling (7 of 10) is refused, exit 1" "$RC" 1
grep -q 'reason=cpus_override_out_of_bounds' "$T/err" && ok "names reason cpus_override_out_of_bounds" || bad "reason: $(cat "$T/err")"
SHIM_NPROC=10 RUNP_CPUS=0 runp IMG-TOOL -- true; check "RUNP_CPUS=0 is refused" "$RC" 2
for bc in abc 1.5 -1 00 ""; do [ -n "$bc" ] || continue; SHIM_NPROC=10 RUNP_CPUS="$bc" runp IMG-TOOL -- true; check "RUNP_CPUS='$bc' is refused" "$RC" 2; done
RUNP_PIDS=0 runp IMG-TOOL -- true; check "RUNP_PIDS=0 (unlimited pids) is refused" "$RC" 2
check "  ...nothing composed" "$(wc -c <"$T/argv")" 0
for bp in abc -1 01; do RUNP_PIDS="$bp" runp IMG-TOOL -- true; check "RUNP_PIDS='$bp' is refused" "$RC" 2; done
RUNP_PIDS=100 runp IMG-TOOL -- true; check "RUNP_PIDS=100 composes" "$(sed -n "$(( $(idx --pids-limit) + 1 ))p" "$T/argv")" 100
# N2 (round 5): a huge RUNP_PIDS is a refusal too; ceiling = min(8192, ulimit -u / 2), default = min(2048, ceiling)
pidsv() { sed -n "$(( $(idx --pids-limit) + 1 ))p" "$T/argv"; }
RUNP_ULIMIT_U=100000 RUNP_PIDS=8192 runp IMG-TOOL -- true; check "RUNP_PIDS exactly at the 8192 ceiling is accepted" "$RC" 0
check "  ...and is the value composed" "$(pidsv)" 8192
for hp in 8193 4194303 999999999999999999; do
  RUNP_ULIMIT_U=100000 RUNP_PIDS="$hp" runp IMG-TOOL -- true; check "RUNP_PIDS=$hp (above the ceiling) is refused, exit 1" "$RC" 1
  grep -q 'reason=pids_override_out_of_bounds' "$T/err" && ok "  ...names reason pids_override_out_of_bounds" || bad "  reason: $(cat "$T/err")"
  check "  ...nothing composed" "$(wc -c <"$T/argv")" 0
done
RUNP_ULIMIT_U=4000 RUNP_PIDS=2000 runp IMG-TOOL -- true; check "ulimit -u 4000: RUNP_PIDS=2000 (half of it) is accepted" "$RC" 0
RUNP_ULIMIT_U=4000 RUNP_PIDS=2001 runp IMG-TOOL -- true; check "ulimit -u 4000: RUNP_PIDS=2001 (above half of it) is refused, exit 1" "$RC" 1
RUNP_ULIMIT_U=1000 runp IMG-TOOL -- true; check "ulimit -u 1000: the default is clamped to the ceiling (500)" "$(pidsv)" 500
RUNP_ULIMIT_U=unlimited RUNP_PIDS=8192 runp IMG-TOOL -- true; check "ulimit unlimited: the absolute 8192 ceiling still applies" "$RC" 0
RUNP_ULIMIT_U=unlimited RUNP_PIDS=8193 runp IMG-TOOL -- true; check "ulimit unlimited: 8193 is refused" "$RC" 1
RUNP_ULIMIT_U=100000 runp IMG-TOOL -- true; check "default pids is 2048 on a normal host" "$(pidsv)" 2048

# m-a (round 5): the RESOLVED --out path and the checkout path are checked for ':' (podman would read it as a volume separator)
mkdir -p "$T/c:d"; ln -sfn "$T/c:d" "$T/colonlink"
runp --out "$T/colonlink/y" IMG-TOOL -- true; check "--out whose resolved path contains ':' (through a symlink) is refused, exit 1" "$RC" 1
grep -q 'reason=out_dir_malformed' "$T/err" && ok "  ...names reason out_dir_malformed" || bad "  reason: $(cat "$T/err")"
check "  ...nothing composed" "$(wc -c <"$T/argv")" 0
mkdir -p "$T/co:lon/ck/docs" "$T/co:lon/ck/.audit/scratch"; ln -sfn "$T/co:lon/ck" "$T/cklink"
runp_in() { ( cd "$1" && PATH="$SHIMS:$PATH" RUNP_PRINT_ARGV=1 bash "$SUT" IMG-TOOL -- true ) >"$T/argv" 2>"$T/err"; RC=$?; }
runp_in "$T/co:lon/ck"; check "a checkout path containing ':' is refused, exit 1" "$RC" 1
grep -q 'reason=source_path_malformed' "$T/err" && ok "  ...names reason source_path_malformed" || bad "  reason: $(cat "$T/err")"
runp_in "$T/cklink"; check "a checkout reached through a symlink whose RESOLVED path contains ':' is refused" "$RC" 1
# round 6 (F5): with an explicit --out OUTSIDE the tree the default-out refusal cannot be what refuses; only the resolved checkout path can
runp_in_args() { local d="$1"; shift; ( cd "$d" && PATH="$SHIMS:$PATH" RUNP_PRINT_ARGV=1 bash "$SUT" "$@" ) >"$T/argv" 2>"$T/err"; RC=$?; }
runp_in_args "$T/cklink" --out "$T/outdir/colon-ck" IMG-TOOL -- true
check "a symlinked checkout (resolved path has ':') with an explicit --out outside the tree is refused, exit 1" "$RC" 1
grep -q 'reason=source_path_malformed' "$T/err" && ok "  ...and the reason is source_path_malformed (the resolved checkout path, not the default out)" || bad "  reason: $(cat "$T/err")"
check "  ...nothing composed" "$(wc -c <"$T/argv")" 0
runp --out "$T/outdir/plain" IMG-TOOL -- true; check "control: an ordinary --out still composes" "$RC" 0

# ---------- round 6, F1: whitespace in a path is NOT a hazard (probed with real podman: a space and a TAB are bound correctly);
# only ':' (the volume separator) and a newline (the print-mode oracle is one element per line) are refused, each with its own accurate reason ----------
SPCK="$T/sp ace/ck"; TABCK="$T/ta"$'\t'"b/ck"; NLCK="$T/nl"$'\n'"x/ck"
for _c in "$SPCK" "$TABCK" "$NLCK"; do mkdir -p "$_c/docs" "$_c/.audit/scratch"; done
runp_in_args "$SPCK" --op-id ws1 IMG-TOOL -- true
check "a checkout path containing a SPACE composes (default --out), exit 0" "$RC" 0
argv_has "$SPCK:/src:ro" && ok "  ...the space path is bound as ONE argv element (source)" || bad "  source bind: $(grep ':/src:ro' "$T/argv")"
argv_has "$SPCK/.audit/out/ws1:/out:rw" && ok "  ...and the default out directory under it is bound as one element" || bad "  out bind: $(grep ':/out:rw' "$T/argv")"
check "  ...no refusal text on stderr" "$(wc -c <"$T/err")" 0
runp_in_args "$TABCK" --op-id ws2 IMG-TOOL -- true
check "a checkout path containing a TAB composes (default --out), exit 0" "$RC" 0
argv_has "$TABCK:/src:ro" && ok "  ...the TAB path is bound as one element" || bad "  tab bind"
runp_in_args "$SPCK" --op-id ws3 --rw docs IMG-TOOL -- true
check "a space checkout with --rw docs composes" "$RC" 0
argv_has "$SPCK/docs:/src/docs:rw" && ok "  ...the rw bind carries the space path as one element" || bad "  rw bind"
# run mode through the podman shim (the second oracle) with the space path: the out directory is created and the shim sees the same element
: >"$SHIM_LOG"; ( cd "$SPCK" && PATH="$SHIMS:$PATH" bash "$SUT" --op-id ws4 IMG-TOOL -- true ) >"$T/out" 2>"$T/err"; RC=$?
check "run mode in a space checkout exits 0" "$RC" 0
check "  ...the out directory exists" "$([ -d "$SPCK/.audit/out/ws4" ] && echo yes || echo no)" yes
check "  ...the shim saw the space path as ONE element" "$(shim_run_argv | grep -cxF -- "$SPCK/.audit/out/ws4:/out:rw")" 1
runp_in_args "$NLCK" --op-id ws5 IMG-TOOL -- true
check "a checkout path containing a NEWLINE is refused, exit 1" "$RC" 1
grep -q 'reason=source_path_malformed' "$T/err" && ok "  ...names reason source_path_malformed" || bad "  reason: $(cat "$T/err")"
grep -qi 'newline' "$T/err" && ok "  ...and says the cause is a newline" || bad "  cause not named: $(cat "$T/err")"
check "  ...the refusal is ONE stderr line" "$(wc -l <"$T/err")" 1
check "  ...nothing composed" "$(wc -c <"$T/argv")" 0
# with an explicit --out outside the tree the default out cannot be the refuser: only the checkout-path newline check can
runp_in_args "$NLCK" --op-id ws6 --out "$T/outdir/nl-ck" IMG-TOOL -- true
check "a newline checkout with an explicit --out outside the tree is refused, exit 1" "$RC" 1
grep -q 'reason=source_path_malformed' "$T/err" && ok "  ...the reason is source_path_malformed (the checkout path itself)" || bad "  reason: $(cat "$T/err")"
mkdir -p "$T/ws out"; runp --out "$T/ws out/x y" IMG-TOOL -- true
check "--out containing a space composes (outside the tree)" "$RC" 0
argv_has "$T/ws out/x y:/out:rw" && ok "  ...bound as one element" || bad "  out bind: $(grep ':/out:rw' "$T/argv")"
runp --out "$T/ws out/t"$'\t'"b" IMG-TOOL -- true; check "--out containing a TAB composes" "$RC" 0
runp --out "$T/ws out/n"$'\n'"l" IMG-TOOL -- true; check "--out containing a newline is refused, exit 1" "$RC" 1
grep -q 'reason=out_dir_malformed' "$T/err" && ok "  ...names reason out_dir_malformed" || bad "  reason: $(cat "$T/err")"
check "  ...nothing composed" "$(wc -c <"$T/argv")" 0
mkdir -p "$T/nl"$'\n'"dir"; ln -sfn "$T/nl"$'\n'"dir" "$T/nllink"
runp --out "$T/nllink/y" IMG-TOOL -- true; check "--out whose resolved path contains a newline (through a symlink) is refused, exit 1" "$RC" 1
grep -q 'reason=out_dir_malformed' "$T/err" && ok "  ...names reason out_dir_malformed" || bad "  reason: $(cat "$T/err")"
runp --out "$T/ws out/c:d" IMG-TOOL -- true; check "--out containing ':' is still refused" "$RC" 1
grep -qi 'volume separator' "$T/err" && ok "  ...and the reason names the volume separator" || bad "  reason: $(cat "$T/err")"

# ---------- round 6, F2: the test hooks cannot lift a ceiling; an unreadable or zero limit is the most restrictive outcome, never the permissive one ----------
runp_nomode() { ( cd "$CK" && env -u RUNP_TEST_MODE PATH="$SHIMS:$PATH" RUNP_PRINT_ARGV=1 "$@" bash "$SUT" IMG-TOOL -- true ) >"$T/argv" 2>"$T/err"; RC=$?; }
runp_nomode RUNP_ULIMIT_U=100000
check "RUNP_ULIMIT_U without RUNP_TEST_MODE is refused, exit 1" "$RC" 1
grep -q 'reason=test_hook_outside_test_mode' "$T/err" && ok "  ...names reason test_hook_outside_test_mode" || bad "  reason: $(cat "$T/err")"
check "  ...nothing composed" "$(wc -c <"$T/argv")" 0
# the inherited RUNP_MEMINFO (the fixture is exported) is a hook too: without the mode the whole run is refused
runp_nomode X=1; check "an exported RUNP_MEMINFO without RUNP_TEST_MODE is refused (fake meminfo cannot be honoured)" "$RC" 1
grep -q 'reason=test_hook_outside_test_mode' "$T/err" && ok "  ...names reason test_hook_outside_test_mode" || bad "  reason: $(cat "$T/err")"
printf 'MemTotal:     4294967296 kB\nMemAvailable: 4294967296 kB\n' >"$T/meminfo.fake"
( cd "$CK" && env -u RUNP_TEST_MODE PATH="$SHIMS:$PATH" RUNP_PRINT_ARGV=1 RUNP_MEMINFO="$T/meminfo.fake" RUNP_MEMORY=1099511627776 bash "$SUT" IMG-TOOL -- true ) >"$T/argv" 2>"$T/err"; RC=$?
check "a fake 4 TiB meminfo + RUNP_MEMORY=1 TiB cannot compose without the test mode, exit 1" "$RC" 1
check "  ...nothing composed" "$(wc -c <"$T/argv")" 0
for tm in 0 true yes 2 ""; do
  ( cd "$CK" && PATH="$SHIMS:$PATH" RUNP_PRINT_ARGV=1 RUNP_TEST_MODE="$tm" bash "$SUT" IMG-TOOL -- true ) >"$T/argv" 2>"$T/err"; RC=$?
  check "RUNP_TEST_MODE='$tm' is not the declared test mode (only 1): the hooks are refused" "$RC" 1
done
# production path: no hooks, no mode, the REAL meminfo and the REAL ulimit; an explicit small RUNP_MEMORY avoids depending on the host's free memory
( cd "$CK" && env -u RUNP_TEST_MODE -u RUNP_MEMINFO -u RUNP_ULIMIT_U PATH="$SHIMS:$PATH" RUNP_PRINT_ARGV=1 RUNP_MEMORY=1073741824 bash "$SUT" IMG-TOOL -- true ) >"$T/argv" 2>"$T/err"; RC=$?
check "production path (no hooks, no test mode) composes with the real meminfo and ulimit" "$RC" 0
check "  ...with the real pids ceiling applied: the default is min(2048, ceiling)" "$(pidsv)" "$(ul="$(ulimit -u)"; c=8192; [ "$ul" != unlimited ] && [ $((ul/2)) -lt $c ] && c=$((ul/2)); [ $c -lt 2048 ] && echo $c || echo 2048)"
# a hook value that is zero, unparsable, signed, empty, padded or an arithmetic injection is a refusal (the most restrictive outcome), never 8192
mkdir -p "$T/pwn.d"
for hv in garbage 0 0100 -5 "" "1 2" "12abc" '$((1))' 'a[$(touch '"$T"'/pwn.d/pwned)]' '2**64' 99999999999999999999; do
  RUNP_ULIMIT_U="$hv" RUNP_PIDS=8192 runp IMG-TOOL -- true
  check "RUNP_ULIMIT_U='$hv' is refused (fail closed), exit 1" "$RC" 1
  check "  ...nothing composed" "$(wc -c <"$T/argv")" 0
done
grep -q 'reason=pids_budget_unavailable' "$T/err" && ok "  ...names reason pids_budget_unavailable" || bad "  reason: $(cat "$T/err")"
check "an arithmetic-injection hook value executed nothing" "$(ls -A "$T/pwn.d" | wc -l)" 0
# the floor: a limit of 1, 2 or 3 gives ceiling 1 (never 0, which podman reads as unlimited pids)
for ulv in 1 2 3; do RUNP_ULIMIT_U=$ulv runp IMG-TOOL -- true; check "ulimit -u $ulv: the default composes --pids-limit 1 (never 0 = unlimited)" "$(pidsv)" 1; done
RUNP_ULIMIT_U=1 RUNP_PIDS=2 runp IMG-TOOL -- true; check "ulimit -u 1: RUNP_PIDS=2 is refused above the ceiling of 1, exit 1" "$RC" 1
# a meminfo hook file that does not parse is a refusal (test mode), never a guessed budget
printf 'MemTotal: abc kB\nMemAvailable: 1 kB\n' >"$T/meminfo.bad"; RUNP_MEMINFO="$T/meminfo.bad" runp IMG-TOOL -- true
check "an unparsable meminfo hook file is refused, exit 1" "$RC" 1
grep -q 'reason=meminfo_unreadable' "$T/err" && ok "  ...names reason meminfo_unreadable" || bad "  reason: $(cat "$T/err")"
# a REAL ulimit -u (no RUNP_ULIMIT_U hook) lowered in a subshell: ceiling = half of it (the README gap 'a real ulimit -u lowering was not exercised')
if [ "$(ps -u "$(id -u)" -o pid= 2>/dev/null | wc -l)" -lt 8000 ] && ( ulimit -u 10000 ) 2>/dev/null; then
  rl="$( ulimit -u 10000; unset RUNP_ULIMIT_U
         runp IMG-TOOL -- true; echo "default rc=$RC pids=$(pidsv)"
         RUNP_PIDS=5000 runp IMG-TOOL -- true; echo "at-ceiling rc=$RC pids=$(pidsv)"
         RUNP_PIDS=5001 runp IMG-TOOL -- true; echo "above rc=$RC" )"
  check "real ulimit -u 10000: the default is 2048" "$(printf '%s\n' "$rl" | grep '^default')" "default rc=0 pids=2048"
  check "real ulimit -u 10000: RUNP_PIDS=5000 (half of it) composes" "$(printf '%s\n' "$rl" | grep '^at-ceiling')" "at-ceiling rc=0 pids=5000"
  check "real ulimit -u 10000: RUNP_PIDS=5001 is refused" "$(printf '%s\n' "$rl" | grep '^above')" "above rc=1"
else echo "SKIP: real ulimit -u 10000 leg (the host user already runs too many processes, or the soft limit cannot be lowered)"; fi

# ---------- smoke_probe.sh: a tool that errors is not present (I1) ----------
SP="$HERE/../smoke_probe.sh"
SPT="$T/sp"; mkdir -p "$SPT/bin" "$SPT/src"; chmod 555 "$SPT/src"
mkstub() { printf '#!/bin/sh\n%s\n' "$2" >"$SPT/bin/$1"; chmod +x "$SPT/bin/$1"; }
mkstub kcov  'echo "kcov: error while loading shared libraries: libx.so"; exit 127'
mkstub jq    'echo "cannot execute binary file: Exec format error"; exit 126'
mkstub sqlite3 'echo "3.40.1 2022-12-28"; exit 0'
mkstub gofmt 'echo "usage: gofmt [flags] [path ...]"; exit 2'
mkstub realpath ': ; exit 0'
runprobe() { # $1 = path of the probe script
  rm -rf "$SPT/out"; mkdir -p "$SPT/out"; ( cd "$SPT" && PATH="$SPT/bin:$PATH" SMOKE_OUT="$SPT/out" SMOKE_SRC="$SPT/src" XDG_CACHE_HOME="$SPT/cache" bash "$1" IMG-PROBE ) >"$SPT/probe.stdout" 2>"$SPT/probe.stderr"; PRC=$?
}
pj() { jq -r "$1" "$SPT/out/toolchain.json" 2>/dev/null; }
runprobe "$SP"
check "probe runs and writes toolchain.json" "$([ -s "$SPT/out/toolchain.json" ] && echo yes || echo no)" yes
check "probe: a tool that prints a loader error and exits 127 is error, not present" "$(pj .tools.kcov.status)" error
check "probe: a tool that exits 126 is error" "$(pj .tools.jq.status)" error
check "probe: the error records the exit status" "$(pj .tools.kcov.exit)" 127
check "probe: a tool that exits 0 with output is present" "$(pj .tools.sqlite3.status)" present
check "probe: a tool with a stated exit-code exception (gofmt -h exits 2) is present" "$(pj .tools.gofmt.status)" present
check "probe: a tool that exits 0 with NO output is error" "$(pj .tools.realpath.status)" error
check "probe: the absent needle is absent" "$(pj '.tools["definitely-not-a-tool"].status')" absent
check "probe: the errored needle (exit 3) is error" "$(pj '.tools["errored-needle"].status')" error
check "probe: the read-only write needle failed as expected" "$(pj .ro_mount_write_probe)" failed
check "probe: uid recorded" "$(pj .uid)" "$(id -u)"
check "probe: toolchain.json is valid JSON" "$(jq -e . "$SPT/out/toolchain.json" >/dev/null 2>&1 && echo yes || echo no)" yes
# paired mutation of the probe: the old behaviour (exit status ignored) must make this section fail
if [ "${RUNP_TEST_MUTANT:-0}" != 1 ] && [ "${RUNP_TEST_NO_MUTATIONS:-0}" != 1 ]; then
  PM="$SPT/smoke_probe.mutant.sh"; sed -E '/# MUT:probe-status$/s/^.*$/  good=1/' "$SP" >"$PM"
  if cmp -s "$SP" "$PM"; then bad "probe mutation: marker not found"; else
    runprobe "$PM"
    [ "$(pj .tools.kcov.status)" = present ] && ok "probe mutation (exit status ignored) is VISIBLE: kcov exit 127 reads as present" || bad "probe mutation did not change behaviour"
  fi
fi

# ---------- mutations ----------
if [ "${RUNP_TEST_MUTANT:-0}" != 1 ] && [ "${RUNP_TEST_NO_MUTATIONS:-0}" != 1 ]; then
  REC="${RUNP_MUTATION_RECORD:-$T/runp-mutations.txt}"; : >"$REC"
  echo "run_pinned.sh paired mutations (test_run_pinned.sh, $(date -u +%FT%TZ)); sha256 of SUT: $(sha256sum "$SUT" | cut -d' ' -f1)" >>"$REC"
  MUTS="$(grep -o '# MUT:[a-z-]*' "$SUT" | sed 's/# MUT://' | sort -u)"
  [ -z "${RUNP_ONLY_MUTATIONS:-}" ] || MUTS="$RUNP_ONLY_MUTATIONS"   # a space-separated subset (round 6: the mutations of the new code); unset = every marker
  MF=0
  for m in $MUTS; do
    # the copy lives in a scratch tree shaped like the repository, so its sibling disk_headroom.sh/.conf and the tracked lock resolve
    MD="$T/mut-$m/scripts/containers"; mkdir -p "$MD" "$T/mut-$m/build/containers"
    ln -sf "$(cd "$(dirname "$SUT")" && pwd)/disk_headroom.sh" "$MD/disk_headroom.sh"; ln -sf "$(cd "$(dirname "$SUT")" && pwd)/disk_headroom.conf" "$MD/disk_headroom.conf"
    ln -sf "$REPO/build/containers/images.lock.yaml" "$T/mut-$m/build/containers/images.lock.yaml"
    MS="$MD/run_pinned.sh"
    # each mutation removes or weakens exactly the behaviour its marker line implements; the copy must still parse
    case "$m" in
      limits|workdir|user|pull|secopt|cacheenv) sed -E "/# MUT:$m\$/d" "$SUT" >"$MS";;
      digest-length) sed -E '/# MUT:digest-length$/s/-eq 64/-le 64/' "$SUT" >"$MS";;
      reserve) sed -E '/# MUT:reserve$/s#mt \* 1024 \* 15 / 100#mt * 1024 * 5 / 100#' "$SUT" >"$MS";;
      pids-ceiling) sed -E '/# MUT:pids-ceiling$/s/-le "\$PIDS_CEIL"/-le 4194304/' "$SUT" >"$MS";;
      pids-guard) sed -E '/# MUT:pids-guard$/s/ && \[ "\$RUNP_PIDS" != 0 \]//' "$SUT" >"$MS";;
      headroom) sed -E "s/^HR_ERR=.*# MUT:headroom\$/HR_ERR=\"\"; HR_RC=0/" "$SUT" >"$MS";;
      rw-allowlist) sed -E 's#^    docs\|\.audit/scratch\) ;;.*#    docs|.audit/scratch*) ;;#' "$SUT" >"$MS";;
      entrypoint) sed -E 's#^if \[ -n "\$EPO" \]; then A\+=.*#if false; then :#' "$SUT" >"$MS";;
      test-hooks) sed -E '/# MUT:test-hooks$/s/^if .*$/if false; then/' "$SUT" >"$MS";;
      *) sed -E "/# MUT:$m\$/s/^.*\$/:/" "$SUT" >"$MS";;
    esac
    chmod +x "$MS"
    if ! bash -n "$MS" 2>/dev/null; then echo "MUTATION $m: copy does not parse (mutation script defect)" | tee -a "$REC"; MF=$((MF+1)); continue; fi
    cmp -s "$SUT" "$MS" && { echo "MUTATION $m: copy identical to the SUT (marker not found)" | tee -a "$REC"; MF=$((MF+1)); continue; }
    if RUNP_SUT="$MS" RUNP_TEST_MUTANT=1 QUIET=1 bash "${BASH_SOURCE[0]}" >"$T/mut.out" 2>&1; then
      echo "MUTATION $m: SURVIVED (the test passed against the mutated launcher)" | tee -a "$REC"; MF=$((MF+1))
    else
      echo "MUTATION $m: caught ($(grep -c '^FAIL' "$T/mut.out") failing checks; first: $(grep '^FAIL' "$T/mut.out" | head -2 | cut -c1-90 | tr '\n' '|'))" | tee -a "$REC"
    fi
  done
  [ "$MF" = 0 ] || bad "$MF mutation(s) not caught"
fi
echo "test_run_pinned: $PASSES passed, $FAILS failed"
[ "$FAILS" = 0 ]
