#!/usr/bin/env bash
# test_nas_readonly_leg.sh - T132 real-NAS leg (RED first). Oracle for scripts/test-infra/nas_readonly_leg.sh and client/nas_smb_ro.sh, the clearly separated READ-ONLY leg against
# the owner's real Synology hosts. Oracle strategy (11.4.245): SPECIFIED (the owner's rules: read-only, <= 2 requests/s, credentials only from the gitignored env file and never in
# argv / environment / logs / evidence, SKIP not FAIL when the env file is absent, names and content never recorded) and INVARIANT (no credential value occurs in any file or any
# process argv / container environment sampled during a run). Sections:
#   A static: every smbclient -c command of the client script starts with ls or get; no write-class verb exists (control needle: a planted `del` is found)
#   B SKIP: absent env file -> `SKIP nas_readonly reason=env_absent`, exit 0, nothing created; missing variables -> credentials_absent with variable NAMES only
#   C secrecy: a fixture env file with SENTINEL credentials and an unroutable address (192.0.2.1, TEST-NET-1): the leg runs, FAILs for that host, and the sentinel occurs nowhere
#     (host argv and container argv/env sampled every second during the run, stdout, stderr, leg logs, the out directory afterwards); the auth file is deleted
#   D real: with the real env file, the two-host leg PASSes (writes_performed 0, requests per host <= 3, names_recorded false) and the real password occurs in no evidence file
# Paired mutations: a client copy with a write verb (A must fail); a host-script copy that puts the password into argv (C must fail); one that leaves the auth file (C must fail).
# Usage:  test_nas_readonly_leg.sh            (NAS_NO_MUTATIONS=1: tests only; NAS_NO_REAL=1: skip section D)    Env: NAS_EV (directory that receives nas-readonly-leg.json), NAS_SUT_DIR
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
SD="${NAS_SUT_DIR:-scripts/test-infra}"
export TI_ROOT="$TI_REPO"
for f in nas_readonly_leg.sh client/nas_smb_ro.sh; do need_script "$SD/$f" || { ti_summary; exit 1; }; done
LEG="$TI_REPO/$SD/nas_readonly_leg.sh"; CLI="$TI_REPO/$SD/client/nas_smb_ro.sh"
wait_cond() { local n=$1 i=0; shift; while [ "$i" -lt $((n * 20)) ]; do "$@" && return 0; sleep 0.05; i=$((i+1)); done; return 1; }   # polls every 50 ms
FX="$TI_REPO/.audit/scratch/nas-fx.$$"; mkdir -p "$FX"; trap 'rm -rf -- "${FX:?}"; ti_cleanup' EXIT

# ---- A static ----
write_verbs() { # print every smbclient -c command of a script that does not start with ls or get
  grep -E 'smbclient' "$1" | grep -vE '^[[:space:]]*#' | grep -oE "\-c +('[^']*'|\"[^\"]*\")" | sed -E "s/^-c +['\"]//; s/['\"]\$//" | grep -vE '^(ls|get)( |$)' || true; }
wv=$(write_verbs "$CLI"); [ -z "$wv" ] && ok "A: every smbclient -c command of the client starts with ls or get" || bad "A: a command other than ls/get exists: $wv"
n_c=$(grep -E 'smbclient' "$CLI" | grep -vE '^[[:space:]]*#' | grep -oE "\-c +('[^']*'|\"[^\"]*\")" | wc -l); [ "$n_c" -ge 2 ] && ok "A: the client has $n_c smbclient -c commands (the scan is not blind)" || bad "A: the scan sees only $n_c commands"
printf "smbclient //h/s -A f -c 'del x'\n" >"$FX/needle.sh"; [ -n "$(write_verbs "$FX/needle.sh")" ] && ok "A: control needle: a planted del is found by the same scan" || bad "A: control needle: the scan is blind"
nh=$(grep -vE '^[[:space:]]*#' "$LEG" | grep -c 'smbclient'); check "A: the host script runs no smbclient itself (it only starts the read-only client script)" "$nh" 0

# ---- B SKIP ----
before=$(ls -d "$XDG_RUNTIME_DIR"/catalogizer-nas-ro.* "$TI_REPO"/.audit/out/nas-ro-* 2>/dev/null | sort | tr '\n' ' ')
o=$(TI_ENV_FILE="$FX/no-such.env" bash "$LEG" 2>&1); rc=$?
check "B: absent env file exits 0 (SKIP, not FAIL)" "$rc" 0
case "$o" in "SKIP nas_readonly reason=env_absent"*) ok "B: the output is SKIP nas_readonly reason=env_absent";; *) bad "B: wrong output: $o";; esac
check "B: nothing was created for the skipped run" "$(ls -d "$XDG_RUNTIME_DIR"/catalogizer-nas-ro.* "$TI_REPO"/.audit/out/nas-ro-* 2>/dev/null | sort | tr '\n' ' ')" "$before"
printf 'SYNOLOGY_IP_3=192.0.2.1\n' >"$FX/partial.env"
o=$(TI_ENV_FILE="$FX/partial.env" bash "$LEG" --hosts 3 2>&1); rc=$?
check "B: an env file without the credentials exits 0 (SKIP)" "$rc" 0
case "$o" in "SKIP nas_readonly reason=credentials_absent variables: SYNOLOGY_SMB_USER SYNOLOGY_SMB_PASSWORD"*) ok "B: the SKIP names the missing variables (names only)";; *) bad "B: wrong output: $o";; esac

# ---- C secrecy (sentinel credentials, unroutable address) ----
SU="SENTINELUSER$RANDOM$RANDOM"; SP="SENTINELPASS$RANDOM$RANDOM"
printf 'SYNOLOGY_SMB_USER=%s\nSYNOLOGY_SMB_PASSWORD=%s\nSYNOLOGY_IP_3=192.0.2.1\n' "$SU" "$SP" >"$FX/sentinel.env"; chmod 600 "$FX/sentinel.env"
secrecy() { # secrecy <leg script>: runs the leg against the sentinel env, samples process argv and container env; prints one FAIL line per violation; exit status = count
  local leg=$1 n=0 pid hits=0 s mseen=0
  # WF12 F14: only the out directory of THIS run (nas-ro-<pid of the leg>) is ever read or removed: a real NAS leg of another stream keeps its auth file
  local od mh=0 m
  TI_ENV_FILE="$FX/sentinel.env" bash "$leg" --hosts 3 >"$FX/c.out" 2>"$FX/c.err" & pid=$!
  od="$XDG_RUNTIME_DIR/catalogizer-nas-ro.$pid"
  while kill -0 "$pid" 2>/dev/null; do
    s="$(ps -eo args 2>/dev/null; podman ps -a --format '{{.Command}} {{.Names}}' 2>/dev/null; for c in $(podman ps -q --filter label=catalogizer.op_id 2>/dev/null); do podman inspect "$c" --format '{{.Config.Env}} {{.Config.Cmd}} {{.Args}}' 2>/dev/null; done)"
    if printf '%s' "$s" | grep -qF -e "$SP" -e "$SU"; then hits=$((hits+1)); fi
    # WF12 F3: the leg's container must not have the repository (it holds the real .env) among its mounts
    for c in $(podman ps -q --filter "label=catalogizer.op_id=nas-ro-$pid-3" 2>/dev/null); do
      m="$(podman inspect "$c" --format '{{range .Mounts}}{{.Source}} {{end}}' 2>/dev/null)"; [ -n "$m" ] && mseen=1
      printf '%s\n' "$m" | tr ' ' '\n' | grep -qxF "$TI_REPO" && mh=$((mh+1))
    done
    sleep 1
  done
  wait "$pid"; local rc=$?
  [ "$hits" -eq 0 ] || { echo "FAIL a credential value appeared in process argv or a container environment ($hits samples)"; n=$((n+1)); }
  [ "$mseen" = 1 ] || { echo "FAIL control: the leg's container mounts were never sampled (the mount check is blind)"; n=$((n+1)); }
  [ "$mh" -eq 0 ] || { echo "FAIL the leg's container had the repository (and its .env) mounted ($mh samples)"; n=$((n+1)); }
  grep -qF -e "$SP" -e "$SU" "$FX/c.out" "$FX/c.err" && { echo "FAIL a credential value appeared in the leg's stdout or stderr"; n=$((n+1)); }
  if grep -rqF -e "$SP" -e "$SU" "$od" 2>/dev/null; then
    # the auth file is the one permitted holder while the leg runs; after the leg it must be gone, and no other file may hold a value
    if [ -e "$od/auth" ]; then echo "FAIL the auth file was left behind"; n=$((n+1)); else echo "FAIL a credential value is in a file of the out directory"; n=$((n+1)); fi
  fi
  [ "$rc" -ne 0 ] || { echo "FAIL the leg against an unroutable address did not FAIL (rc=0)"; n=$((n+1)); }
  grep -q 'FAIL nas_readonly host=Synology3' "$FX/c.out" || { echo "FAIL the failure of the unreachable host was not reported"; n=$((n+1)); }
  rm -rf -- "${od:?}"
  return "$n"
}
res=$(secrecy "$LEG"); n=$?
if [ "$n" -eq 0 ]; then ok "C: sentinel credentials never appear in argv, container environment, output or files; the auth file is deleted; the unreachable host FAILs"; else bad "C: $n violation(s): $(printf '%s' "$res" | tr '\n' ';' | cut -c1-300)"; fi

# ---- C2 exec-level secrecy (WF17 TI-G8): every execve of the whole process tree, argv AND environment, not a 1 Hz sample ----
# A credential in the argv of a short-lived external command (for example the auth file written with /usr/bin/printf) lives about a millisecond: the sampler above cannot see it, strace -f -e execve can.
traced() { # traced <leg script> <tag>: runs the leg (sentinel env) under strace; prints the number of trace lines that hold a sentinel and the number of podman execs the trace shows (the leg got as far as the container)
  local leg=$1 tag=$2
  TI_ENV_FILE="$FX/sentinel.env" timeout 120 strace -f -qq -v -e trace=execve -s 4096 -o "$FX/trace.$tag" bash "$leg" --hosts 3 >"$FX/t.$tag.out" 2>&1
  echo "$(grep -cF -e "$SP" -e "$SU" "$FX/trace.$tag") $(grep -c 'execve("[^"]*podman"' "$FX/trace.$tag")"
}
if command -v strace >/dev/null 2>&1; then
  strace -f -qq -v -e trace=execve -s 4096 -o "$FX/trace.ctrl" env "SENTINEL_CTRL=$SP" true >/dev/null 2>&1
  if grep -qF "$SP" "$FX/trace.ctrl"; then ok "C2: control needle: strace -v -e execve sees a sentinel placed in an exec environment"; else bad "C2: control needle: the exec trace is BLIND (a sentinel in an environment is not in the trace)"; fi
  strace -f -qq -v -e trace=execve -s 4096 -o "$FX/trace.ctrl2" /usr/bin/printf '%s' "$SP" >/dev/null 2>&1
  if grep -qF "$SP" "$FX/trace.ctrl2"; then ok "C2: control needle: a sentinel in the argv of an external command (/usr/bin/printf) is in the trace"; else bad "C2: control needle: the exec trace is BLIND to an external-command argv"; fi
  t=$(traced "$LEG" real); th=${t% *}; tp=${t#* }
  check "C2: no execve of the leg's whole process tree carries a credential value in argv or environment" "$th" 0
  [ "$tp" -ge 1 ] && ok "C2: the trace reaches the container stage ($tp podman execs): the check covers the credential hand-off" || bad "C2: the trace never reaches podman: the leg stopped early under strace, the check would be blind to the container stage"
else bad "C2: strace is not installed on this host (the exec-level secrecy check cannot run)"; fi

# ---- C3 SIGKILL (WF17 TI-C3): no trap runs; the credential file must not be on a persistent disk, and the sweep removes what is left ----
secfiles() { # secfiles <root>...: regular files under the roots that hold a sentinel value (find -type f: never opens a socket or a fifo)
  local r; for r in "$@"; do [ -d "$r" ] && find "$r" -type f -readable -print0 2>/dev/null | timeout 120 xargs -0 -r grep -lF -e "$SP" -e "$SU" 2>/dev/null | grep -v "^$FX/"; done; true; }   # the test's OWN fixtures (the sentinel env file, the strace traces, the leg's logs) live under $FX and are the control, not a finding
authfile() { [ -f "$XDG_RUNTIME_DIR/catalogizer-nas-ro.$kp/auth" ]; }
setsid env TI_ENV_FILE="$FX/sentinel.env" bash "$LEG" --hosts 3 >"$FX/k.out" 2>&1 & kp=$!; TI_BG_PIDS+=("$kp")
if wait_cond 40 authfile; then
  ok "C3: the auth file exists while the leg runs (under the per-user tmpfs)"
  [ -n "$(secfiles "$XDG_RUNTIME_DIR/catalogizer-nas-ro.$kp")" ] && ok "C3: control needle: the sentinel IS found in the auth file while the leg runs" || bad "C3: control needle: the sentinel is not in the auth file (the search is blind)"
  kpg=$(ps -o pgid= -p "$kp" 2>/dev/null | tr -d ' ')
  if [ -n "$kpg" ] && [ "$kpg" -gt 1 ]; then kill -KILL -- "-$kpg" 2>/dev/null; else bad "C3: cannot resolve the process group of the leg (pgid '$kpg')"; fi
  wait "$kp" 2>/dev/null
  for c in $(podman ps -a -q --filter "label=catalogizer.op_id=nas-ro-$kp-3" 2>/dev/null); do podman rm -f "$c" >/dev/null 2>&1; done
  check "C3: after SIGKILL no file under the checkout's .audit holds a credential value (a trap-less death leaves it on the tmpfs only)" "$(secfiles "$TI_REPO/.audit" | wc -l)" 0
  kd="$XDG_RUNTIME_DIR/catalogizer-nas-ro.$kp"
  [ -d "$kd" ] && ok "C3: the killed run left its run directory on the tmpfs (what the sweep must remove)" || bad "C3: the killed run left no directory (the test would prove nothing about the sweep)"
  so=$(bash "$TI_REPO/$SD/sweep_leaks.sh" 2>&1); src=$?
  check "C3: sweep_leaks.sh exits 0" "$src" 0
  case "$so" in *"REMOVED nasdir catalogizer-nas-ro.$kp"*) ok "C3: the sweep removed the killed run's directory (its pid is proven gone)";; *) bad "C3: the sweep did not remove it: $(printf '%s' "$so" | grep nasdir | head -2 | tr '\n' ' ' | cut -c1-200)";; esac
  check "C3: after the sweep no file under .audit or the runtime directory holds a credential value" "$(secfiles "$TI_REPO/.audit" "$XDG_RUNTIME_DIR" | wc -l)" 0
  rm -rf -- "${kd:?}"
else bad "C3: the auth file never appeared"; kill -KILL "$kp" 2>/dev/null; wait "$kp" 2>/dev/null; fi

# ---- C4 catchable signals (WF17 class C): INT, TERM and HUP at the stage where the auth file exists run the trap: no run directory, no credential file, no leftover container ----
for sg in INT TERM HUP; do
  setsid python3 -I -c 'import os,signal,sys; [signal.signal(s, signal.SIG_DFL) for s in (signal.SIGINT, signal.SIGHUP, signal.SIGTERM, signal.SIGQUIT)]; os.execvp(sys.argv[1], sys.argv[1:])' env TI_ENV_FILE="$FX/sentinel.env" bash "$LEG" --hosts 3 >"$FX/s-$sg.out" 2>&1 & kp=$!; TI_BG_PIDS+=("$kp")
  if wait_cond 40 authfile; then
    kpg=$(ps -o pgid= -p "$kp" 2>/dev/null | tr -d ' ')
    if [ -n "$kpg" ] && [ "$kpg" -gt 1 ]; then kill -s "$sg" -- "-$kpg" 2>/dev/null; else bad "C4-$sg: cannot resolve the process group (pgid '$kpg')"; fi
    gone_() { ! kill -0 "$kp" 2>/dev/null; }; wait_cond 60 gone_ && ok "C4-$sg: the leg exited within 60 s of $sg" || bad "C4-$sg: the leg is still running 60 s after $sg"
    wait "$kp" 2>/dev/null
    for c in $(podman ps -a -q --filter "label=catalogizer.op_id=nas-ro-$kp-3" 2>/dev/null); do podman rm -f "$c" >/dev/null 2>&1; done
    check "C4-$sg: the run directory (and with it the credential file) is gone" "$([ -e "$XDG_RUNTIME_DIR/catalogizer-nas-ro.$kp" ] && echo left || echo gone)" gone
    check "C4-$sg: no container labelled with the leg's operation is left" "$(podman ps -a -q --filter "label=catalogizer.op_id=nas-ro-$kp-3" | wc -l)" 0
  else bad "C4-$sg: the auth file never appeared"; kill -KILL "$kp" 2>/dev/null; wait "$kp" 2>/dev/null; fi
done

# ---- D10: a requested --json-out that cannot be written is an error before any NAS is contacted ----
o=$(TI_ENV_FILE="$FX/sentinel.env" bash "$LEG" --hosts 3 --json-out /proc/no-such-dir/out.json 2>&1); rc=$?
check "D10: an unwritable --json-out exits non-zero" "$rc" 1
case "$o" in *"cannot write --json-out"*) ok "D10: the refusal names the unwritable output";; *) bad "D10: wrong output: $(printf '%s' "$o" | head -2 | tr '\n' ' ' | cut -c1-200)";; esac
check "D10: nothing was contacted (no run directory was created)" "$(ls -d "$XDG_RUNTIME_DIR"/catalogizer-nas-ro.* 2>/dev/null | wc -l)" 0

# ---- D real leg ----
REALENV="$TI_REPO/.env"
if [ "${NAS_NO_REAL:-0}" = 1 ] || [ ! -r "$REALENV" ]; then echo "SKIP: D: the real-NAS leg (env file absent or NAS_NO_REAL=1)"; else
  EVJ="${NAS_EV:-$FX}/nas-readonly-leg.json"; mkdir -p "$(dirname "$EVJ")"
  o=$(bash "$LEG" --hosts 3,4 --json-out "$EVJ" 2>&1); rc=$?
  check "D: the real two-host read-only leg exits 0" "$rc" 0
  printf '%s\n' "$o" | grep -q '^NAS Synology3 ok=true' && printf '%s\n' "$o" | grep -q '^NAS Synology4 ok=true' && ok "D: both hosts PASS" || bad "D: a host did not pass: $(printf '%s' "$o" | tr '\n' ';' | cut -c1-300)"
  jq -e '.writes_performed == 0 and .names_recorded == false and .content_recorded == false and (.hosts | length == 2) and all(.hosts[]; .ok == true and .writes_performed == "0" and (.requests | tonumber) <= 3)' "$EVJ" >/dev/null 2>&1 && ok "D: the record states writes_performed 0, names and content not recorded, at most 3 requests per host" || bad "D: the record is wrong: $(cat "$EVJ" 2>/dev/null | head -c 300)"
  # any dotted IPv4 (not only 192.168.): control needle first, the same pattern must see a planted 10.x address
  IPRE='([0-9]{1,3}\.){3}[0-9]{1,3}'; echo "host 10.20.30.40" >"$FX/ipneedle.txt"
  grep -qE "$IPRE" "$FX/ipneedle.txt" && ok "D: control needle: the IP scan sees a planted 10.x address" || bad "D: control needle: the IP scan is blind"
  if grep -qE "$IPRE" "$EVJ" "$FX"/*.out 2>/dev/null; then bad "D: an IP address is in the record"; else ok "D: no IP address is recorded"; fi
  realpw="$(sed -n 's/^SYNOLOGY_SMB_PASSWORD=//p' "$REALENV" | head -1 | sed -e 's/^"\(.*\)"$/\1/' -e "s/^'\(.*\)'$/\1/")"
  if [ -n "$realpw" ]; then
    leaks=$( { printf '%s\n' "$o"; cat "$EVJ"; } | grep -cF -- "$realpw"); check "D: the real password occurs in no output of the leg" "$leaks" 0
    if [ -n "${NAS_EV:-}" ]; then check "D: the real password occurs in no evidence file under $NAS_EV" "$(grep -rlF -- "$realpw" "$NAS_EV" 2>/dev/null | wc -l)" 0; fi
  fi
  unset realpw
  check "D: no auth file is left behind" "$(ls "$XDG_RUNTIME_DIR"/catalogizer-nas-ro.*/auth "$TI_REPO"/.audit/out/nas-ro-*/auth 2>/dev/null | wc -l)" 0
fi

# ---- mutations ----
if [ "${NAS_NO_MUTATIONS:-0}" != 1 ] && [ "${NAS_TEST_MUTANT:-0}" != 1 ]; then
  MUTLOG="${NAS_EV:-$FX}/nas-mutations.txt"; : >"$MUTLOG"
  M="$TI_REPO/.audit/scratch/nas-mut.$$"; mkdir -p "$M/client"; cp "$TI_REPO/$SD/nas_readonly_leg.sh" "$TI_REPO/$SD/lib.sh" "$TI_REPO/$SD/dotenv_get.py" "$M/"; cp "$TI_REPO/$SD/client"/*.sh "$M/client/"
  trap 'rm -rf -- "${FX:?}" "${M:?}"; ti_cleanup' EXIT
  pymut() { python3 -I - "$1" "$2" "$3" <<'PY'
import sys
s = open(sys.argv[1]).read()
if s.count(sys.argv[2]) != 1: print("anchor count %d for %r" % (s.count(sys.argv[2]), sys.argv[2])); sys.exit(1)
open(sys.argv[1], "w").write(s.replace(sys.argv[2], sys.argv[3]))
PY
  }
  # mutation 1: a write verb in the client
  cp "$CLI" "$M/client/cli1.sh"; pymut "$M/client/cli1.sh" "-c 'ls'" "-c 'del everything'" && { [ -n "$(write_verbs "$M/client/cli1.sh")" ] && { ok "mutation write_verb_in_client CAUGHT (section A scan)"; echo "write_verb_in_client CAUGHT" >>"$MUTLOG"; } || { bad "mutation write_verb_in_client SURVIVED"; echo "write_verb_in_client SURVIVED" >>"$MUTLOG"; }; }
  # mutation 2: the password goes into argv (the op id)
  pymut "$M/nas_readonly_leg.sh" '--op-id "nas-ro-$$-$i"' '--op-id "nas-ro-$$-$i-$(envval SYNOLOGY_SMB_PASSWORD)"' && { r=$(secrecy "$M/nas_readonly_leg.sh"); k=$?; [ "$k" -gt 0 ] && { ok "mutation password_in_argv CAUGHT ($(printf '%s' "$r" | head -1 | cut -c1-90))"; echo "password_in_argv CAUGHT" >>"$MUTLOG"; } || { bad "mutation password_in_argv SURVIVED"; echo "password_in_argv SURVIVED" >>"$MUTLOG"; }; }
  # mutation 3: the auth file is left behind
  cp "$TI_REPO/$SD/nas_readonly_leg.sh" "$M/nas_readonly_leg.sh"; pymut "$M/nas_readonly_leg.sh" 'cleanup() { rm -f -- "${OUT:?}/auth"; case "$OUT" in "$RTD"/catalogizer-nas-ro.*) rm -rf -- "${OUT:?}";; esac; case "$VIEW" in "$TI_ROOT"/.audit/scratch/ti-view.*) rm -rf -- "$VIEW";; esac; }' 'cleanup() { case "$VIEW" in "$TI_ROOT"/.audit/scratch/ti-view.*) rm -rf -- "$VIEW";; esac; }' && { r=$(secrecy "$M/nas_readonly_leg.sh"); k=$?; [ "$k" -gt 0 ] && { ok "mutation auth_file_left CAUGHT ($(printf '%s' "$r" | head -1 | cut -c1-90))"; echo "auth_file_left CAUGHT" >>"$MUTLOG"; } || { bad "mutation auth_file_left SURVIVED"; echo "auth_file_left SURVIVED" >>"$MUTLOG"; }; }
  # mutation 4 (WF12 F3): the leg's container gets the repository (and its .env) mounted again
  cp "$TI_REPO/$SD/nas_readonly_leg.sh" "$M/nas_readonly_leg.sh"; pymut "$M/nas_readonly_leg.sh" '(cd "$VIEW" && bash "$TI_ROOT/scripts/containers/run_pinned.sh"' '(cd "$TI_ROOT" && bash "$TI_ROOT/scripts/containers/run_pinned.sh"' && { r=$(secrecy "$M/nas_readonly_leg.sh"); k=$?; [ "$k" -gt 0 ] && { ok "mutation repo_mounted CAUGHT ($(printf '%s' "$r" | head -1 | cut -c1-90))"; echo "repo_mounted CAUGHT" >>"$MUTLOG"; } || { bad "mutation repo_mounted SURVIVED"; echo "repo_mounted SURVIVED" >>"$MUTLOG"; }; }
  # mutation NL1 (WF17 TI-G8): the auth file is written by an EXTERNAL printf (the credential is in an argv for about a millisecond); the 1 Hz sampler misses it, the exec-level check must not
  cp "$TI_REPO/$SD/nas_readonly_leg.sh" "$M/nas_readonly_leg.sh"; cp "$TI_REPO/$SD/lib.sh" "$M/lib.sh"
  if pymut "$M/nas_readonly_leg.sh" "printf 'username = %s\\npassword = %s\\n' " "/usr/bin/printf 'username = %s\\npassword = %s\\n' "; then
    t=$(traced "$M/nas_readonly_leg.sh" nl1); [ "${t% *}" -gt 0 ] && { ok "mutation NL1_external_printf_credential CAUGHT (exec trace holds the value in $(( ${t% *} )) execve line(s))"; echo "NL1_external_printf_credential CAUGHT" >>"$MUTLOG"; } || { bad "mutation NL1_external_printf_credential SURVIVED"; echo "NL1_external_printf_credential SURVIVED" >>"$MUTLOG"; }
  else bad "mutation NL1: anchor missing"; fi
  # mutation C3 (WF17 TI-C3): the run directory is back under the checkout (persistent disk): the SIGKILL row must find the credential file under .audit
  cp "$TI_REPO/$SD/nas_readonly_leg.sh" "$M/nas_readonly_leg.sh"
  if pymut "$M/nas_readonly_leg.sh" 'OUT="$RTD/catalogizer-nas-ro.$$"; mkdir -m 700 "$OUT"' 'OUT="$TI_ROOT/.audit/out/nas-ro-$$"; mkdir -p -m 700 "$OUT"'; then
    setsid env TI_ENV_FILE="$FX/sentinel.env" bash "$M/nas_readonly_leg.sh" --hosts 3 >"$FX/k2.out" 2>&1 & kp2=$!; TI_BG_PIDS+=("$kp2")
    authfile2() { [ -f "$TI_REPO/.audit/out/nas-ro-$kp2/auth" ]; }
    if wait_cond 40 authfile2; then kpg=$(ps -o pgid= -p "$kp2" | tr -d ' '); [ -n "$kpg" ] && [ "$kpg" -gt 1 ] && kill -KILL -- "-$kpg" 2>/dev/null; wait "$kp2" 2>/dev/null
      for c in $(podman ps -a -q --filter "label=catalogizer.op_id=nas-ro-$kp2-3" 2>/dev/null); do podman rm -f "$c" >/dev/null 2>&1; done
      if [ -n "$(secfiles "$TI_REPO/.audit")" ]; then ok "mutation C3_auth_file_back_under_audit CAUGHT (the credential survives the SIGKILL under .audit)"; echo "C3_auth_file_back_under_audit CAUGHT" >>"$MUTLOG"; else bad "mutation C3_auth_file_back_under_audit SURVIVED"; echo "C3_auth_file_back_under_audit SURVIVED" >>"$MUTLOG"; fi
      rm -rf -- "$TI_REPO/.audit/out/nas-ro-$kp2"
    else bad "mutation C3: the mutant's auth file never appeared"; kill -KILL "$kp2" 2>/dev/null; wait "$kp2" 2>/dev/null; fi
  else bad "mutation C3: anchor missing"; fi
  # identity mutant NL3: a no-op edit must SURVIVE the exec trace
  cp "$TI_REPO/$SD/nas_readonly_leg.sh" "$M/nas_readonly_leg.sh"
  if pymut "$M/nas_readonly_leg.sh" 'res="[]"; rc=0' 'res="[]"; : ; rc=0'; then
    t=$(traced "$M/nas_readonly_leg.sh" nl3); [ "${t% *}" -eq 0 ] && { ok "identity mutant NL3 SURVIVED as it must (the exec trace has no false positive)"; echo "NL3_identity SURVIVED" >>"$MUTLOG"; } || { bad "identity mutant NL3 was CAUGHT (a false positive)"; echo "NL3_identity CAUGHT" >>"$MUTLOG"; }
  else bad "mutation NL3: anchor missing"; fi
fi
ti_summary
