#!/usr/bin/env bash
# test_dispatch_events.sh - T005a PARTIAL slice (host control-plane; P0-P1 host exception).
# Covers the event contract (contracts/build-event.schema.json) and the consume core
# scripts/build/event_core.sh: cases (a) (b) (e) (f) (g) (d) (q) (t) and the secret-file rules.
# OWED (not in this slice, listed in OWED_CASES below): the dispatcher, hub, callback runner,
# ssh shim and the remote cases. Independent oracle: python3 (HKDF, canonical JSON, HMAC).
# EC_SCRIPT overrides the script under test (paired-mutation runs).
set -u
here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
root=$(cd "$here/../../.." && pwd)
EC=${EC_SCRIPT:-$here/../event_core.sh}
ECDIR=$(dirname "$EC")
SCHEMA=$root/specs/001-full-project-audit-remediation/contracts/build-event.schema.json
tmp=$(mktemp -d); trap 'rm -rf "$tmp"' EXIT
pass=0; fail=0
mkdir -p "$tmp/logs"; selflog=$tmp/logs/self.log; : > "$selflog"
say() { printf '%s\n' "$1" | tee -a "$selflog"; }
# Honest degradation (round 4): a section whose ORACLE tool (strace, node, gcc for the fsync fault shim) is absent is reported as SKIP with the
# tool named, never as a pass; the core's own dependencies and python jsonschema (the schema oracle) are hard requirements (a FAIL, never a SKIP).
skipn=0; SKIPWHY=""
ok()  { if [ -n "$SKIPWHY" ]; then skipn=$((skipn+1)); say "SKIP $1 (oracle tool missing: $SKIPWHY)"; else pass=$((pass+1)); say "ok   $1"; fi; }
bad() { if [ -n "$SKIPWHY" ]; then skipn=$((skipn+1)); say "SKIP $1 (oracle tool missing: $SKIPWHY)"; else fail=$((fail+1)); say "FAIL $1"; fi; }
oracle_begin() { local m="" t; for t in "$@"; do case $t in shim) [ -s "$tmp/fsfail.so" ] || m="$m fsync-shim(gcc)";; *) command -v "$t" >/dev/null 2>&1 || m="$m $t";; esac; done
  SKIPWHY=${m# }; [ -z "$SKIPWHY" ] || say "SKIP-SECTION: oracle tool(s) missing: $SKIPWHY"; }
oracle_end() { SKIPWHY=""; }
for t in python3 jq openssl flock realpath awk; do command -v "$t" >/dev/null 2>&1 || { echo "FAIL core dependency missing: $t"; echo "RESULT pass=0 fail=1 skip=0"; exit 1; }; done
python3 -c 'import jsonschema' 2>/dev/null || { echo "FAIL schema oracle missing: python3 jsonschema"; echo "RESULT pass=0 fail=1 skip=0"; exit 1; }
OWED_CASES="c d-full h i i2 j k l m n0-n2 n o p q2 q3 r0 r s u2 v w u x (dispatcher, hub, runner, ssh shim, remote host)"
[ -f "$EC" ] || { echo "FAIL script absent: $EC"; echo "RESULT pass=0 fail=1 skip=0"; exit 1; }

# ---- independent oracle (python3) ----
py() { python3 - "$@" <<'PY'
import sys, json, hmac, hashlib, struct
def hkdf(secret, info, n=32):
    prk = hmac.new(b"\x00"*32, secret, hashlib.sha256).digest()   # empty salt
    t = b""; okm = b""; i = 1
    while len(okm) < n:
        t = hmac.new(prk, t + info + bytes([i]), hashlib.sha256).digest(); okm += t; i += 1
    return okm[:n]
def enc(s): b = s.encode(); return struct.pack(">I", len(b)) + b
cmd = sys.argv[1]
if cmd == "key":
    secret = bytes.fromhex(sys.argv[2]); print(hkdf(secret, enc(sys.argv[3]) + enc(sys.argv[4])).hex())
elif cmd == "sign":   # sign <keyhex> <json>
    ev = json.loads(sys.argv[3]); ev.pop("hmac", None)
    canon = json.dumps(ev, sort_keys=True, separators=(",", ":"), ensure_ascii=False).encode()
    ev["hmac"] = hmac.new(bytes.fromhex(sys.argv[2]), canon, hashlib.sha256).hexdigest()
    print(json.dumps(ev))
PY
}
validate() { python3 - "$SCHEMA" "$1" <<'PY'
import sys, json, jsonschema
s = json.load(open(sys.argv[1])); d = json.loads(sys.argv[2])
try: jsonschema.Draft202012Validator(s).validate(d)
except jsonschema.ValidationError: sys.exit(1)
PY
}

# ---- fixture world ----
state=$tmp/state; builds=$tmp/builds; mkdir -p "$builds"
secret_hex=$(openssl rand -hex 32)
mkdir_state() { mkdir -p "$1"; chmod 700 "$1"; printf '%s\n' "$2" > "$1/build_hmac.key"; chmod 600 "$1/build_hmac.key"
  printf 'created=fixture\nsha256=%s\n' "$(sha256sum "$1/build_hmac.key" | cut -d' ' -f1)" > "$1/build_hmac.key.meta"; }
mkdir_state "$state" "$secret_hex"
newbuild() { # build_id run_id
  mkdir -p "$builds/$1"; jq -n --arg b "$1" --arg r "$2" '{build_id:$b,run_id:$r,variant:"primary"}' > "$builds/$1/submit.json"
  printf '{"callback_id":"cb-test","effect_key":"effect-%s"}\n' "$1" > "$builds/$1/callback.json"; }
ev() { # build run seq kind [extra-json]  -> signed event on stdout
  local b=$1 r=$2 s=$3 k=$4 x=${5:-'{}'} key
  key=$(py key "$secret_hex" "$b" "$r")
  local base; base=$(jq -n --arg b "$b" --arg r "$r" --argjson s "$s" --arg k "$k" \
    '{schema:"build-event/1",run_id:$r,build_id:$b,variant:"primary",seq:$s,kind:$k,host:"anton",sent_at:"2026-10-05T10:00:00Z"}')
  base=$(printf '%s' "$base" | jq -c --argjson x "$x" '. + $x')
  py sign "$key" "$base"
}
HB='{"progress_offset":10,"stage":1,"elapsed_monotonic_ms":1000}'
CMP='{"exit_class":"succeeded","artifact_manifest_sha256":"'$(printf a | sha256sum | cut -d' ' -f1)'","image_digest":"sha256:abc","remote_log_sha256":"'$(printf b | sha256sum | cut -d' ' -f1)'"}'
consume() { local o rc; o=$(bash "$EC" consume "$builds" "$state" "$1" 2>&1); rc=$?; printf '%s\n' "$o" >> "$tmp/logs/transcript.log"; printf '%s\n' "$o"; return $rc; }  # arg: event file
efile() { local f; f=$(mktemp "$tmp/ev.XXXXXX"); printf '%s\n' "$1" > "$f"; printf '%s\n' "$f"; }   # echoes the name it created
cbs() { [ -p "$builds/$1/terminal/callback.state" ] && { echo "<fifo>"; return 0; }; cat "$builds/$1/terminal/callback.state" 2>/dev/null; }   # T005b: callback.state file inside terminal/ (a FIFO there, a round-6 fixture, must not hang a failing run's diagnostics)
cbset() { printf '%s\n' "$2" > "$builds/$1/terminal/callback.state"; }
efiles() { ls "$builds/$1/effects" 2>/dev/null | wc -l; }                            # effect FILES (effects.log counts audit lines)
effects() { [ -f "$builds/$1/effects.log" ] && wc -l < "$builds/$1/effects.log" || echo 0; }
terminals() { find "$builds/$1" -maxdepth 1 -name terminal -type d | wc -l; }

# ---- contract (schema) ----
good=$(ev B0 R0 1 accepted); validate "$good" && ok "schema: accepted event valid" || bad "schema: accepted event valid"
validate "$(ev B0 R0 2 heartbeat "$HB")" && ok "schema: heartbeat valid" || bad "schema: heartbeat valid"
validate "$(ev B0 R0 3 completed "$CMP")" && ok "schema: completed without peak_rss valid" || bad "schema: completed without peak_rss"
validate "$(ev B0 R0 3 completed "$(jq -c '. + {peak_rss_bytes:1048576}' <<<"$CMP")")" && ok "schema: completed with peak_rss valid" || bad "schema: completed with peak_rss"
validate "$(ev B0 R0 3 completed "$(jq -c '. + {peak_rss_bytes:-1}' <<<"$CMP")")" && bad "schema: negative peak_rss refused" || ok "schema: negative peak_rss refused"
validate "$(ev B0 R0 3 completed "$(jq -c '.exit_class="weird"' <<<"$CMP")")" && bad "schema: unknown exit_class refused" || ok "schema: unknown exit_class refused"
validate "$(ev B0 R0 2 heartbeat)" && bad "schema: heartbeat without progress pair refused" || ok "schema: heartbeat without progress pair refused"

# ---- key derivation against the independent oracle ----
mkdir -p "$tmp/k"; cp "$state/build_hmac.key" "$tmp/k/"
got=$(bash "$EC" derive-key "$state" B0 R0 2>&1); want=$(py key "$secret_hex" B0 R0)
[ "$got" = "$want" ] && ok "HKDF per-build key equals the python oracle" || bad "HKDF key differs from the python oracle (values withheld)"
oinfo=$(LC_ALL=C; printf '%08x' 2; printf B0 | od -An -tx1 | tr -d ' \n'; printf '%08x' 2; printf R0 | od -An -tx1 | tr -d ' \n')
ossl=$(openssl kdf -keylen 32 -kdfopt digest:SHA256 -kdfopt hexkey:"$secret_hex" -kdfopt hexinfo:"$oinfo" HKDF | tr -d ':\n' | tr A-F a-f)
[ "$got" = "$ossl" ] && ok "HKDF key equals the openssl kdf oracle (second independent implementation)" || bad "HKDF key differs from the openssl oracle (values withheld)"
python3 "$ECDIR/lib/bev_crypto.py" hkdf-selftest && ok "HKDF RFC 5869 test cases 1 and 3" || bad "HKDF RFC 5869 selftest"
got2=$(bash "$EC" derive-key "$state" B0 R1 2>&1); [ "$got2" != "$want" ] && ok "key bound to run_id" || bad "key not bound to run_id"

# ---- (a) completed consumed once, callback effect once ----
newbuild BA RA
out=$(consume "$(efile "$(ev BA RA 1 accepted)")"); rc=$?
out2=$(consume "$(efile "$(ev BA RA 2 heartbeat "$HB")")")
out3=$(consume "$(efile "$(ev BA RA 3 completed "$CMP")")"); rc3=$?
if [ $rc3 -eq 0 ] && [ "$(terminals BA)" = 1 ] && [ "$(jq -r .kind "$builds/BA/terminal/state.json")" = completed ] && [ "$(effects BA)" = 1 ] \
   && [ "$(cbs BA)" = done ] && [ "$(efiles BA)" = 1 ]; then ok "(a) completed consumed once, callback done, effect 1"; else bad "(a) rc=$rc3 out=$out3 terminals=$(terminals BA) effects=$(effects BA)"; fi

# ---- (b) duplicated event acknowledged and dropped, effect count stays 1 ----
out=$(consume "$(efile "$(ev BA RA 3 completed "$CMP")")"); rc=$?
if [ $rc -eq 0 ] && printf '%s' "$out" | grep -q DUP && [ "$(effects BA)" = 1 ]; then ok "(b) duplicate dropped, effect still 1"; else bad "(b) rc=$rc out=$out effects=$(effects BA)"; fi

# ---- (b2) duplicated heartbeat (non-terminal) dropped, not read as out-of-order ----
newbuild BB RB; consume "$(efile "$(ev BB RB 1 accepted)")" >/dev/null; consume "$(efile "$(ev BB RB 2 heartbeat "$HB")")" >/dev/null
out=$(consume "$(efile "$(ev BB RB 2 heartbeat "$HB")")"); rc=$?
[ $rc -eq 0 ] && printf '%s' "$out" | grep -q DUP && ok "(b) duplicated heartbeat dropped as DUP" || bad "(b2) rc=$rc out=$out"
[ "$(wc -l < "$builds/BB/events.jsonl")" = 2 ] && ok "(b) duplicate not journaled twice" || bad "(b2) journal lines $(wc -l < "$builds/BB/events.jsonl")"

# ---- (n/o in miniature) callback resumes from durable state, effect keyed once ----
newbuild BN RN; consume "$(efile "$(ev BN RN 1 accepted)")" >/dev/null; consume "$(efile "$(ev BN RN 2 completed "$CMP")")" >/dev/null
cbset BN running   # crash in the middle of the callback
bash "$EC" resume-callback "$builds" BN >/dev/null 2>&1
[ "$(effects BN)" = 1 ] && [ "$(cbs BN)" = done ] && ok "(o) running callback resumed, keyed effect applied once" || bad "(o) effects=$(effects BN)"
bash "$EC" resume-callback "$builds" BN >/dev/null 2>&1
[ "$(effects BN)" = 1 ] && ok "(n) done callback not re-run" || bad "(n) effects=$(effects BN)"

# ---- test-only hold mechanism: a wrapper process sources the script and wraps claim_terminal; nothing in the shipped script reads the environment for it ----
holdrun() { # holdpath eventfile  (runs cmd_consume, pausing inside claim_terminal, with the per-build lock held)
  EC=$EC HOLD=$1 bash -c 'source "$EC"
    eval "$(declare -f claim_terminal | sed "1s/^claim_terminal/claim_terminal_real/")"
    claim_terminal() { : > "$HOLD.reached"; local n=0; while [ ! -e "$HOLD.go" ] && [ $n -lt 400 ]; do sleep 0.05; n=$((n+1)); done; claim_terminal_real "$@"; }
    cmd_consume "$@"' _ "$builds" "$state" "$2" 2>&1; }
holdcancel() { # holdpath build  (runs cmd_cancel, pausing inside claim_terminal with the per-build lock held)
  EC=$EC HOLD=$1 bash -c 'source "$EC"
    eval "$(declare -f claim_terminal | sed "1s/^claim_terminal/claim_terminal_real/")"
    claim_terminal() { : > "$HOLD.reached"; local n=0; while [ ! -e "$HOLD.go" ] && [ $n -lt 400 ]; do sleep 0.05; n=$((n+1)); done; claim_terminal_real "$@"; }
    cmd_cancel "$@"' _ "$builds" "$2" 2>&1; }
waitfor() { for i in $(seq 100); do [ -e "$1" ] && return 0; sleep 0.05; done; return 1; }

# ---- (q2) deterministic, both orders: whichever of consume and cancel holds the per-build lock at the claim wins; the other is blocked, then superseded ----
newbuild BH RH; consume "$(efile "$(ev BH RH 1 accepted)")" >/dev/null
hold=$tmp/hold.BH; cf=$(efile "$(ev BH RH 2 completed "$CMP")")
( holdrun "$hold" "$cf" >/dev/null ) &
cp_=$!; waitfor "$hold.reached"
( bash "$EC" cancel "$builds" BH > "$tmp/qc.out" 2>&1 ) & cq=$!
sleep 0.4; [ ! -s "$tmp/qc.out" ] && ok "(q2) cancel is blocked on the per-build lock while a completed consume holds its claim" || bad "(q2) cancel ran during the held claim: $(cat "$tmp/qc.out")"
: > "$hold.go"; wait $cp_ $cq
cl=$(grep -c terminal_claimed "$builds/BH/events.jsonl"); sp=$(grep -c superseded "$builds/BH/events.jsonl")
{ [ "$cl" = 1 ] && [ "$sp" = 1 ] && [ "$(jq -r .kind "$builds/BH/terminal/state.json")" = completed ] && [ "$(effects BH)" = 1 ] && grep -q superseded "$tmp/qc.out"; } && ok "(q2) held consume wins, the waiting cancel is superseded: one claim, one superseded, effect 1" || bad "(q2 held consume) claimed=$cl superseded=$sp kind=$(jq -r .kind "$builds/BH/terminal/state.json") effects=$(effects BH)"
newbuild BH2 RH2; consume "$(efile "$(ev BH2 RH2 1 accepted)")" >/dev/null
hold=$tmp/hold.BH2; cf=$(efile "$(ev BH2 RH2 2 completed "$CMP")")
( holdcancel "$hold" BH2 >/dev/null ) & cp_=$!; waitfor "$hold.reached"
( bash "$EC" consume "$builds" "$state" "$cf" > "$tmp/qd.out" 2>&1 ) & cq=$!
sleep 0.4; [ ! -s "$tmp/qd.out" ] && ok "(q2) consume is blocked while a cancel holds the per-build lock at its claim" || bad "(q2) consume ran during the held cancel: $(cat "$tmp/qd.out")"
: > "$hold.go"; wait $cp_ $cq
cl=$(grep -c terminal_claimed "$builds/BH2/events.jsonl"); lo=$(grep -c late_ignored "$builds/BH2/events.jsonl")
{ [ "$cl" = 1 ] && [ "$lo" = 1 ] && [ "$(jq -r .kind "$builds/BH2/terminal/state.json")" = cancelled ] && [ "$(effects BH2)" = 1 ] && grep -q late_ignored "$tmp/qd.out"; } && ok "(q2) held cancel wins, the waiting completed is late_ignored: one claim, effect 1" || bad "(q2 held cancel) claimed=$cl late=$lo kind=$(jq -r .kind "$builds/BH2/terminal/state.json") effects=$(effects BH2)"

# ---- (d) late completed after terminal recorded late_ignored, never flips ----
newbuild BD RD; consume "$(efile "$(ev BD RD 1 accepted)")" >/dev/null
bash "$EC" cancel "$builds" BD >/dev/null 2>&1
out=$(consume "$(efile "$(ev BD RD 2 completed "$CMP")")"); rc=$?
if [ $rc -eq 0 ] && printf '%s' "$out" | grep -q late_ignored && [ "$(jq -r .kind "$builds/BD/terminal/state.json")" = cancelled ] && [ "$(effects BD)" = 1 ]; then ok "(d) late completed after cancel is late_ignored, verdict unchanged"; else bad "(d) rc=$rc out=$out"; fi

# ---- (e) out-of-order events refused with illegal_transition ----
newbuild BE RE
out=$(consume "$(efile "$(ev BE RE 1 completed "$CMP")")"); rc=$?
if [ $rc -ne 0 ] && printf '%s' "$out" | grep -q illegal_transition && [ "$(terminals BE)" = 0 ]; then ok "(e) completed before accepted refused"; else bad "(e1) rc=$rc out=$out"; fi
consume "$(efile "$(ev BE RE 1 accepted)")" >/dev/null; consume "$(efile "$(ev BE RE 5 heartbeat "$HB")")" >/dev/null
out=$(consume "$(efile "$(ev BE RE 3 heartbeat "$HB")")"); rc=$?
if [ $rc -ne 0 ] && printf '%s' "$out" | grep -q illegal_transition; then ok "(e) seq below last consumed refused"; else bad "(e2) rc=$rc out=$out"; fi
out=$(consume "$(efile "$(ev BE RE 6 heartbeat "$(jq -nc '{progress_offset:3,stage:1,elapsed_monotonic_ms:2000}')")")"); rc=$?
if [ $rc -ne 0 ] && printf '%s' "$out" | grep -q illegal_transition; then ok "(e) progress_offset lower than last refused"; else bad "(e3) rc=$rc out=$out"; fi

# ---- (f) unsigned / wrongly signed / unknown build ----
newbuild BF RF
unsigned=$(ev BF RF 1 accepted | jq -c 'del(.hmac)')
out=$(consume "$(efile "$unsigned")"); rc=$?
if [ $rc -ne 0 ] && printf '%s' "$out" | grep -q event_unauthenticated; then ok "(f) unsigned event refused"; else bad "(f1) rc=$rc out=$out"; fi
wrong=$(ev BF RF 1 accepted | jq -c '.hmac="'$(printf '0%.0s' $(seq 64))'"')
out=$(consume "$(efile "$wrong")"); rc=$?
if [ $rc -ne 0 ] && printf '%s' "$out" | grep -q event_unauthenticated; then ok "(f) wrongly signed event refused"; else bad "(f2) rc=$rc out=$out"; fi
tampered=$(ev BF RF 1 accepted | jq -c '.host="evil"')
out=$(consume "$(efile "$tampered")"); rc=$?
if [ $rc -ne 0 ] && printf '%s' "$out" | grep -q event_unauthenticated; then ok "(f) tampered body refused"; else bad "(f3) rc=$rc out=$out"; fi
out=$(consume "$(efile "$(ev BNONE RNONE 1 accepted)")"); rc=$?
if [ $rc -ne 0 ] && printf '%s' "$out" | grep -q event_unknown_build; then ok "(f) event for never-issued build refused"; else bad "(f4) rc=$rc out=$out"; fi
[ "$(terminals BF)" = 0 ] && [ ! -e "$builds/BF/events.jsonl" -o ! -s "$builds/BF/events.jsonl" ] && ok "(f) refused events leave no journal line" || bad "(f) refused events journaled"

# ---- (g) replay of a correctly signed event from an earlier run of the same build id ----
newbuild BG RG_NEW
old=$(ev BG RG_OLD 1 accepted)
out=$(consume "$(efile "$old")"); rc=$?
if [ $rc -ne 0 ] && printf '%s' "$out" | grep -q event_replayed; then ok "(g) old-run signed event refused as event_replayed"; else bad "(g) rc=$rc out=$out"; fi

# ---- (q) cancel races a completed event: exactly one terminal, the other superseded ----
qok=1
for i in 1 2 3 4 5 6 7 8; do
  b=BQ$i; newbuild $b RQ; consume "$(efile "$(ev $b RQ 1 accepted)")" >/dev/null
  cf=$(efile "$(ev $b RQ 2 completed "$CMP")")
  ( bash "$EC" cancel "$builds" $b >/dev/null 2>&1 ) & ( bash "$EC" consume "$builds" "$state" "$cf" >/dev/null 2>&1 ) & wait
  t=$(terminals $b); sup=$(grep -c superseded "$builds/$b/events.jsonl" 2>/dev/null || true)
  k=$(jq -r .kind "$builds/$b/terminal/state.json" 2>/dev/null)
  e=$(effects $b)
  cl=$(grep -c terminal_claimed "$builds/$b/events.jsonl" 2>/dev/null || true); lo=$(grep -c late_ignored "$builds/$b/events.jsonl" 2>/dev/null || true)
  { [ "$t" = 1 ] && { [ "$k" = cancelled ] || [ "$k" = completed ]; } && [ "$e" = 1 ] && [ "$cl" = 1 ] && [ $((sup+lo)) = 1 ]; } || { qok=0; say "  q$i terminals=$t kind=$k effects=$e claimed=$cl superseded=$sup late=$lo"; }
done
[ $qok = 1 ] && ok "(q) cancel/completed race x8: one terminal, effect once" || bad "(q) race produced two terminals or wrong effects"

# ---- secret-file rules ----
mkdir -p "$tmp/inrepo"; (cd "$tmp/inrepo" && git init -q . 2>/dev/null)
out=$(bash "$EC" secret-init "$tmp/inrepo/state" "$tmp/inrepo" 2>&1); rc=$?
[ $rc -ne 0 ] && printf '%s' "$out" | grep -q secret_dir_inside_checkout && ok "secret dir inside the checkout refused" || bad "secret-init inside checkout rc=$rc out=$out"
s2=$tmp/state2; bash "$EC" secret-init "$s2" "$root" >/dev/null 2>&1; rc=$?
mode=$(stat -c %a "$s2/build_hmac.key" 2>/dev/null)
[ $rc -eq 0 ] && [ "$mode" = 600 ] && [ "$(wc -c < "$s2/build_hmac.key")" -ge 64 ] && ok "secret created mode 0600 outside checkout" || bad "secret-init rc=$rc mode=$mode"
h1=$(sha256sum "$s2/build_hmac.key"); bash "$EC" secret-init "$s2" "$root" >/dev/null 2>&1; [ "$h1" = "$(sha256sum "$s2/build_hmac.key")" ] && ok "secret-init does not overwrite an existing secret" || bad "secret overwritten"
grep -q "$(cat "$s2/build_hmac.key")" "$s2"/*.meta 2>/dev/null && bad "meta file holds the secret" || ok "meta holds no secret bytes"

# ---- (t) secret / derived key / HMAC values never in any file the run wrote (with a planted needle) ----
scan() { # needle-hex dir -> count of files containing the hex, base64 or raw bytes
  python3 - "$1" "$2" "$3" <<'PY'
import sys, os, base64
hexs, d, skip = sys.argv[1], sys.argv[2], sys.argv[3]
raw = bytes.fromhex(hexs); forms = [hexs.encode(), hexs.upper().encode(), base64.b64encode(raw), raw]
n = 0
for r, _, fs in os.walk(d):
    for f in fs:
        p = os.path.join(r, f)
        if p == skip: continue
        try: b = open(p, "rb").read()
        except OSError: continue
        if any(x in b for x in forms): n += 1
print(n)
PY
}
needle_dir=$tmp/needle; mkdir -p "$needle_dir"; printf 'x%sx' "$secret_hex" > "$needle_dir/planted"
[ "$(scan "$secret_hex" "$needle_dir" none)" = 1 ] && ok "(t) scan sees a planted secret (control needle)" || bad "(t) scan blind"
keyA=$(py key "$secret_hex" BA RA)
c1=$(scan "$secret_hex" "$builds" none); c2=$(scan "$keyA" "$builds" none)
hm=$(jq -r .hmac <<<"$(ev BF RF 1 accepted)"); c3=$(scan "$hm" "$builds" none)
[ "$c1" = 0 ] && [ "$c2" = 0 ] && [ "$c3" = 0 ] && ok "(t) secret, derived key and refused-event hmac absent from build state" || bad "(t) leaks: secret=$c1 key=$c2 hmac=$c3"

# ---- B1: the bytes acted on are the bytes verified (a FIFO serves a signed body to the first reads, a forged unsigned one later) ----
for nsigned in 1 7; do   # N signed bodies first, forged unsigned ones after: N=1 catches any re-read, N=7 reproduces the review probe against the old script
  bt=BT$nsigned; newbuild $bt RT; consume "$(efile "$(ev $bt RT 1 accepted)")" >/dev/null
  signed=$(ev $bt RT 2 completed "$(jq -c '.exit_class="build_failed"' <<<"$CMP")")
  forged=$(ev $bt RT 2 completed "$CMP" | jq -c 'del(.hmac)')
  fifo=$tmp/fifo.$bt; mkfifo "$fifo"
  ( n=0; while [ $n -lt 60 ]; do n=$((n+1)); if [ $n -le $nsigned ]; then printf '%s\n' "$signed" > "$fifo"; else printf '%s\n' "$forged" > "$fifo"; fi; sleep 0.25; done ) & wp=$!
  out=$(timeout 20 bash "$EC" consume "$builds" "$state" "$fifo" 2>&1); rc=$?
  kill $wp 2>/dev/null; wait $wp 2>/dev/null; rm -f "$fifo"
  tk=$(jq -r .exit_class "$builds/$bt/terminal/state.json" 2>/dev/null)
  if [ "$tk" = build_failed ] && ! grep -q '"exit_class":"succeeded"' "$builds/$bt/events.jsonl" && [ "$(effects $bt)" -le 1 ]; then ok "(B1) FIFO, $nsigned signed read(s) then forged: terminal is the VERIFIED exit_class, journal holds no forged body"; else bad "(B1 N=$nsigned) rc=$rc terminal exit_class='$tk' out=$out"; fi
done

# ---- B2: integer comparisons fail closed ----
rawev() { # build run json -> signed event (the json is signed verbatim, big and fractional numbers included)
  py sign "$(py key "$secret_hex" "$1" "$2")" "$3"; }
BASEJ() { printf '{"schema":"build-event/1","run_id":"%s","build_id":"%s","variant":"primary","seq":%s,"kind":"%s","host":"anton","sent_at":"2026-10-05T10:00:00Z"%s}' "$1" "$2" "$3" "$4" "${5:-}"; }
newbuild BS RS; consume "$(efile "$(ev BS RS 1 accepted)")" >/dev/null; consume "$(efile "$(ev BS RS 5 heartbeat "$HB")")" >/dev/null
: > "$builds/BS/consumed/5.tmp-99999"; : > "$builds/BS/consumed/x"; : > "$builds/BS/consumed/0"; : > "$builds/BS/consumed/0099"; : > "$builds/BS/consumed/99999999999999999999"
out=$(consume "$(efile "$(ev BS RS 3 heartbeat "$HB")")"); rc=$?
if [ $rc -ne 0 ] && printf '%s' "$out" | grep -q illegal_transition; then ok "(B2) stray crash-leftover file does not blind the ordering check (seq 3 after 5 refused)"; else bad "(B2 stray) rc=$rc out=$out"; fi
out=$(consume "$(efile "$(ev BS RS 7 heartbeat "$HB")")"); rc=$?
[ $rc -eq 0 ] && printf '%s' "$out" | grep -q "consumed seq=7" && ok "(B2) stray files ignored: seq 7 consumed" || bad "(B2 stray ok) rc=$rc out=$out"
newbuild BF2 RF2; consume "$(efile "$(ev BF2 RF2 1 accepted)")" >/dev/null; consume "$(efile "$(ev BF2 RF2 2 heartbeat "$HB")")" >/dev/null
for case in 'frac|2|heartbeat|,"progress_offset":100.5,"stage":1,"elapsed_monotonic_ms":1' \
            'bigseq|99999999999999999999|accepted|' \
            'bigoff|3|heartbeat|,"progress_offset":9007199254740993,"stage":1,"elapsed_monotonic_ms":1' \
            'negoff|3|heartbeat|,"progress_offset":-1,"stage":1,"elapsed_monotonic_ms":1' \
            'booloff|3|heartbeat|,"progress_offset":true,"stage":1,"elapsed_monotonic_ms":1' \
            'stroff|3|heartbeat|,"progress_offset":"5","stage":1,"elapsed_monotonic_ms":1' \
            'floatseq|1.0|accepted|' ; do
  IFS='|' read -r nm sq kd ex <<<"$case"
  e=$(rawev BF2 RF2 "$(BASEJ RF2 BF2 "$sq" "$kd" "$ex")")
  out=$(consume "$(efile "$e")"); rc=$?
  if [ $rc -ne 0 ] && printf '%s' "$out" | grep -q event_malformed; then ok "(B2) $nm refused as event_malformed"; else bad "(B2 $nm) rc=$rc out=$out"; fi
done
[ "$(ls "$builds/BF2/consumed")" = "1
2" ] && [ "$(jq -c . "$builds/BF2/progress.json")" = '{"progress_offset":10,"stage":1}' ] && ok "(B2) refused numeric events left no consumed file and progress unchanged" || bad "(B2) state poisoned: $(ls "$builds/BF2/consumed" | tr '\n' ' ') $(cat "$builds/BF2/progress.json")"
out=$(consume "$(efile "$(ev BF2 RF2 3 heartbeat "$HB")")"); rc=$?
[ $rc -eq 0 ] && ok "(B2) valid seq 3 still consumed after the refused ones" || bad "(B2 after) rc=$rc out=$out"

# ---- I5: a done callback is not touched by resume-callback (inode and bytes unchanged) ----
newbuild BI RI; consume "$(efile "$(ev BI RI 1 accepted)")" >/dev/null; consume "$(efile "$(ev BI RI 2 completed "$CMP")")" >/dev/null
sf=$builds/BI/terminal/state.json; cf_=$builds/BI/terminal/callback.state; i1=$(stat -c %i "$sf"); h1=$(sha256sum < "$sf"); ci1=$(stat -c %i "$cf_"); ch1=$(sha256sum < "$cf_")
bash "$EC" resume-callback "$builds" BI >/dev/null 2>&1
{ [ "$i1" = "$(stat -c %i "$sf")" ] && [ "$h1" = "$(sha256sum < "$sf")" ] && [ "$ci1" = "$(stat -c %i "$cf_")" ] && [ "$ch1" = "$(sha256sum < "$cf_")" ]; } && ok "(I5) resume-callback on a done build leaves terminal/state.json and callback.state inode and bytes unchanged" || bad "(I5) done state rewritten"

# ---- I6: the per-build lock serialises parallel consumers (journal order) and an unopenable lock fails closed ----
lockok=1
for trial in 1 2 3 4 5; do
  bb=BL$trial; newbuild $bb RL; consume "$(efile "$(ev $bb RL 1 accepted)")" >/dev/null
  files=(); for sq in $(seq 2 16 | shuf); do files+=("$(efile "$(ev $bb RL $sq heartbeat "$(jq -nc --argjson o $((sq*10)) '{progress_offset:$o,stage:1,elapsed_monotonic_ms:1}')")")"); done
  for f in "${files[@]}"; do ( bash "$EC" consume "$builds" "$state" "$f" >/dev/null 2>&1 ) & done; wait
  seqs=$(jq -r 'select(.kind=="heartbeat")|.seq' "$builds/$bb/events.jsonl")
  [ "$seqs" = "$(printf '%s\n' "$seqs" | sort -n)" ] && [ "$(printf '%s\n' "$seqs" | sort -n | uniq -d | wc -l)" = 0 ] || { lockok=0; say "  lock trial $trial journal order: $(echo $seqs)"; }
done
[ $lockok = 1 ] && ok "(I6) 15 shuffled parallel heartbeats x5: journal strictly increasing, no duplicates" || bad "(I6) journal order violated under parallel consumers"
newbuild BLK RLK; consume "$(efile "$(ev BLK RLK 1 accepted)")" >/dev/null; rm -f "$builds/BLK/.lock"; mkdir "$builds/BLK/.lock"
out=$(consume "$(efile "$(ev BLK RLK 2 heartbeat "$HB")")"); rc=$?
out2=$(bash "$EC" consume "$builds" "$state" "$(efile "$(ev BLK RLK 2 heartbeat "$HB")")" 9>/dev/null 2>&1); rc2=$?   # an inherited fd 9 must not stand in for the lock
{ [ $rc -ne 0 ] && printf '%s' "$out" | grep -q lock_unavailable && [ ! -e "$builds/BLK/consumed/2" ] && [ $rc2 -ne 0 ] && printf '%s' "$out2" | grep -q lock_unavailable; } && ok "(I6) unopenable lock refuses (fail closed), also with an inherited fd 9; nothing consumed" || bad "(I6 lock) rc=$rc out=$out"

# ---- I7: validation of the event contract, core and schema agree ----
newbuild BV RV; consume "$(efile "$(ev BV RV 1 accepted)")" >/dev/null; consume "$(efile "$(ev BV RV 2 heartbeat "$HB")")" >/dev/null
chk() { # name json-extra-expression-on-base reason  ; base = signed-able event built by jq
  local nm=$1 e=$2 reason=$3 out rc
  out=$(consume "$(efile "$e")"); rc=$?
  if [ $rc -ne 0 ] && printf '%s' "$out" | grep -q "$reason"; then ok "(I7) $nm refused ($reason)"; else bad "(I7 $nm) rc=$rc out=$out"; fi
  validate "$e" && bad "(I7 $nm) the schema accepts what the core refuses" || ok "(I7) $nm also rejected by the schema"; }
chk "completed exit_class weird" "$(ev BV RV 3 completed "$(jq -c '.exit_class="weird"' <<<"$CMP")")" event_malformed
chk "accepted with an extra field" "$(ev BV RV 3 accepted '{"extra":1}')" event_malformed
chk "heartbeat without progress_offset" "$(ev BV RV 3 heartbeat '{"stage":1,"elapsed_monotonic_ms":1}')" event_malformed
chk "heartbeat without stage" "$(ev BV RV 3 heartbeat '{"progress_offset":50,"elapsed_monotonic_ms":1}')" event_malformed
chk "completed without image_digest" "$(ev BV RV 3 completed "$(jq -c 'del(.image_digest)' <<<"$CMP")")" event_malformed
chk "completed without exit_class" "$(ev BV RV 3 completed "$(jq -c 'del(.exit_class)' <<<"$CMP")")" event_malformed
chk "completed without remote_log_sha256" "$(ev BV RV 3 completed "$(jq -c 'del(.remote_log_sha256)' <<<"$CMP")")" event_malformed
chk "completed without artifact_manifest_sha256" "$(ev BV RV 3 completed "$(jq -c 'del(.artifact_manifest_sha256)' <<<"$CMP")")" event_malformed
out=$(consume "$(efile "$(ev BV RV 3 heartbeat '{"progress_offset":100,"stage":0,"elapsed_monotonic_ms":1}')")"); rc=$?
{ [ $rc -ne 0 ] && printf '%s' "$out" | grep -q "illegal_transition"; } && ok "(I7) stage going backwards refused (offset forward)" || bad "(I7 stage) rc=$rc out=$out"
out=$(consume "$(efile "$(ev BV RV 3 heartbeat '{"progress_offset":100,"stage":1,"elapsed_monotonic_ms":1}')")"); rc=$?
[ $rc -eq 0 ] && ok "(I7) a correct heartbeat after the refused ones is consumed" || bad "(I7 ok) rc=$rc out=$out"
dup=$(printf '{"a":1,"a":2}'); printf '%s' "$dup" > "$tmp/dupkey.json"
out=$(consume "$tmp/dupkey.json"); rc=$?; { [ $rc -ne 0 ] && printf '%s' "$out" | grep -q event_malformed; } && ok "(I7) duplicate JSON keys refused" || bad "(I7 dup) rc=$rc out=$out"

# ---- I8: lock held through the terminal claim; a redelivered completed is DUP ----
newbuild BP RP; consume "$(efile "$(ev BP RP 1 accepted)")" >/dev/null
hold=$tmp/hold.BP; cf=$(efile "$(ev BP RP 2 completed "$CMP")"); hf=$(efile "$(ev BP RP 3 heartbeat "$HB")")
( holdrun "$hold" "$cf" >/dev/null ) & cp_=$!; waitfor "$hold.reached"
( bash "$EC" consume "$builds" "$state" "$hf" > "$tmp/hb.out" 2>&1 ) & hp=$!
sleep 0.4; [ ! -s "$tmp/hb.out" ] && ok "(I8) heartbeat is blocked on the per-build lock while the completed claim is held" || bad "(I8) heartbeat ran during the held claim: $(cat "$tmp/hb.out")"
: > "$hold.go"; wait $cp_ $hp
{ grep -q late_ignored "$tmp/hb.out" && [ ! -e "$builds/BP/consumed/3" ] && [ "$(jq -r .kind "$builds/BP/terminal/state.json")" = completed ]; } && ok "(I8) heartbeat after the held claim is late_ignored, not consumed" || bad "(I8) hb=$(cat "$tmp/hb.out")"
dupok=1
for trial in 1 2 3 4 5; do
  bb=BR$trial; newbuild $bb RR; consume "$(efile "$(ev $bb RR 1 accepted)")" >/dev/null
  f1=$(efile "$(ev $bb RR 2 completed "$CMP")")
  ( bash "$EC" consume "$builds" "$state" "$f1" > "$tmp/r1.out" 2>&1 ) & ( bash "$EC" consume "$builds" "$state" "$f1" > "$tmp/r2.out" 2>&1 ) & wait
  nc=$(jq -r 'select(.schema=="build-event/1" and .kind=="completed")|.seq' "$builds/$bb/events.jsonl" | wc -l); nt=$(grep -c terminal_claimed "$builds/$bb/events.jsonl"); ns=$(grep -c superseded "$builds/$bb/events.jsonl")
  dups=$(cat "$tmp/r1.out" "$tmp/r2.out" | grep -c '^DUP')
  { [ "$nc" = 1 ] && [ "$nt" = 1 ] && [ "$ns" = 0 ] && [ "$dups" = 1 ] && [ "$(effects $bb)" = 1 ]; } || { dupok=0; say "  redelivery $trial completed-lines=$nc claimed=$nt superseded=$ns dups=$dups"; }
done
[ $dupok = 1 ] && ok "(I8) concurrent redelivered completed x5: journaled once, one DUP, no self-supersede" || bad "(I8) redelivered completed journaled or superseded wrongly"

# ---- I9: no environment variable can hold or alter the shipped script ----
! grep -q 'EC_TEST' "$EC" && ok "(I9) shipped script has no EC_TEST seam" || bad "(I9) EC_TEST seam present in the shipped script"
newbuild BZ RZ; consume "$(efile "$(ev BZ RZ 1 accepted)")" >/dev/null
out=$(EC_TEST_HOLD_BEFORE_CLAIM=$tmp/hz timeout 10 bash "$EC" consume "$builds" "$state" "$(efile "$(ev BZ RZ 2 completed "$CMP")")" 2>&1); rc=$?
{ [ $rc -eq 0 ] && [ ! -e "$tmp/hz.reached" ] && [ "$(jq -r .kind "$builds/BZ/terminal/state.json")" = completed ]; } && ok "(I9) EC_TEST_HOLD_BEFORE_CLAIM in the environment neither hangs nor touches any path" || bad "(I9) rc=$rc reached=$([ -e "$tmp/hz.reached" ] && echo yes || echo no)"

# ---- m1: secret read path ----
newbuild BK RK
sbad() { # name statedir-setup-command reason
  local nm=$1 sdir=$2 out rc; mkdir -p "$builds/BK"; rm -rf "$builds/BK/consumed"
  out=$(bash "$EC" consume "$builds" "$sdir" "$(efile "$(ev BK RK 1 accepted)")" 2>&1); rc=$?
  { [ $rc -ne 0 ] && printf '%s' "$out" | grep -q "secret_unusable.*$3"; } && ok "(m1) $nm refused ($3)" || bad "(m1 $nm) rc=$rc out=$out"; }
s=$tmp/sm_mode; mkdir_state "$s" "$secret_hex"; chmod 644 "$s/build_hmac.key"; sbad "key mode 0644" "$s" key_mode_not_0600
s=$tmp/sm_link; mkdir_state "$s.real" "$secret_hex"; ln -s "$s.real" "$s"; sbad "symlinked state dir" "$s" state_dir_symlink
s=$tmp/sm_klink; mkdir -p "$s"; chmod 700 "$s"; mkdir_state "$s.t" "$secret_hex"; ln -s "$s.t/build_hmac.key" "$s/build_hmac.key"; cp "$s.t/build_hmac.key.meta" "$s/"; sbad "symlinked key file" "$s" key_not_regular_file
s=$tmp/sm_meta; mkdir_state "$s" "$secret_hex"; printf 'sha256=%s\n' "$(printf 0 | sha256sum | cut -d' ' -f1)" > "$s/build_hmac.key.meta"; sbad "meta sha mismatch" "$s" meta_sha_mismatch
s=$tmp/sm_nometa; mkdir_state "$s" "$secret_hex"; rm "$s/build_hmac.key.meta"; sbad "meta absent" "$s" meta_absent
mkdir -p "$tmp/co/scripts/build"; cp "$EC" "$tmp/co/scripts/build/event_core.sh"; cp -r "$ECDIR/lib" "$tmp/co/scripts/build/lib"; rm -rf "$tmp/co/scripts/build/lib/__pycache__"
s=$tmp/co/state; mkdir_state "$s" "$secret_hex"
out=$(bash "$tmp/co/scripts/build/event_core.sh" consume "$builds" "$s" "$(efile "$(ev BK RK 1 accepted)")" 2>&1); rc=$?
{ [ $rc -ne 0 ] && printf '%s' "$out" | grep -q "secret_unusable.*state_dir_inside_checkout"; } && ok "(m1) state dir inside the checkout refused on READ" || bad "(m1 inside) rc=$rc out=$out"
out=$(bash "$tmp/co/scripts/build/event_core.sh" derive-key "$s" BK RK 2>&1); rc=$?; { [ $rc -eq 20 ] && printf '%s' "$out" | grep -q secret_unusable; } && ok "(m1) derive-key refuses an unusable secret location" || bad "(m1 derive) rc=$rc"
s=$tmp/si_mode; bash "$EC" secret-init "$s" "$root" >/dev/null 2>&1; chmod 644 "$s/build_hmac.key"
out=$(bash "$EC" secret-init "$s" "$root" 2>&1); rc=$?; { [ $rc -ne 0 ] && printf '%s' "$out" | grep -q secret_unusable; } && ok "(m1) secret-init does not report a loose-mode existing key as kept" || bad "(m1 si) rc=$rc out=$out"
echo "owner/uid check (key_wrong_owner) and the RUNP-mount location check are UNCONFIRMED here (single uid host; RUNP absent): OWED"

# ---- m2: build ids that name a directory outside a build are refused ----
for bid in . .. .x a.b; do
  e=$(rawev "$bid" RM "$(BASEJ RM "$bid" 1 accepted)"); out=$(consume "$(efile "$e")"); rc=$?
  { [ $rc -ne 0 ] && printf '%s' "$out" | grep -q event_malformed; } && ok "(m2) build_id '$bid' refused" || bad "(m2 '$bid') rc=$rc out=$out"
  validate "$e" && bad "(m2 '$bid') schema accepts it" || ok "(m2) schema also rejects build_id '$bid'"
done
[ ! -e "$builds/consumed" ] && ok "(m2) nothing was written at the builds root" || bad "(m2) builds root polluted"

# ---- m3: same seq with different content is a conflict, same content a DUP ----
newbuild BC RC; consume "$(efile "$(ev BC RC 1 accepted)")" >/dev/null; consume "$(efile "$(ev BC RC 2 heartbeat "$HB")")" >/dev/null
out=$(consume "$(efile "$(ev BC RC 2 completed "$CMP")")"); rc=$?
{ [ $rc -ne 0 ] && printf '%s' "$out" | grep -q seq_conflict && [ ! -d "$builds/BC/terminal" ]; } && ok "(m3) different event reusing a consumed seq refused as seq_conflict" || bad "(m3) rc=$rc out=$out"
out=$(consume "$(efile "$(ev BC RC 2 heartbeat "$HB")")"); printf '%s' "$out" | grep -q '^DUP' && ok "(m3) identical redelivery still DUP" || bad "(m3 dup) $out"

# ---- m6 / R3-I4: file and directory fsync are real fsync(2) calls (python os.fsync): `sync FILE` is a GLOBAL sync(2) on this host (uutils), which can never report an error ----
oracle_begin strace
newbuild BY RY; sxlog=$tmp/fs.BY.log; : > "$sxlog"
e1=$(efile "$(ev BY RY 1 accepted)"); e2=$(efile "$(ev BY RY 2 heartbeat "$HB")"); e3=$(efile "$(ev BY RY 3 completed "$CMP")")
strace -f -y -qq -e trace=fsync,fdatasync,rename,renameat,renameat2 -o "$sxlog" bash -c 'EC=$1; b=$2; s=$3; shift 3; for f in "$@"; do bash "$EC" consume "$b" "$s" "$f" >/dev/null 2>&1; done' _ "$EC" "$builds" "$state" "$e1" "$e2" "$e3"
dm=1; for dpath in "$builds/BY" "$builds/BY/consumed" "$builds/BY/terminal" "$builds/BY/effects"; do grep -F 'fsync(' "$sxlog" | grep -qF "<$dpath>)" || { dm=0; say "  directory not fsynced: $dpath"; }; done
[ $dm = 1 ] && ok "(m6) build, consumed, terminal and effects directories are fsynced (fsync(2) seen by strace)" || bad "(m6) directory fsync missing"
oracle_end
newbuild BG2 RG2; consume "$(efile "$(ev BG2 RG2 1 accepted)")" >/dev/null; consume "$(efile "$(ev BG2 RG2 2 completed "$CMP")")" >/dev/null
rm -f "$builds/BG2/effects/effect-BG2"; cbset BG2 running
bash "$EC" resume-callback "$builds" BG2 >/dev/null 2>&1
{ [ "$(effects BG2)" = 1 ] && [ -e "$builds/BG2/effects/effect-BG2" ]; } && ok "(m6) crash after the audit line, before the effect: resume applies the effect, log keeps one line" || bad "(m6) effects.log=$(effects BG2) effect-file=$([ -e "$builds/BG2/effects/effect-BG2" ] && echo yes || echo no)"
newbuild BG3 RG3; consume "$(efile "$(ev BG3 RG3 1 accepted)")" >/dev/null; consume "$(efile "$(ev BG3 RG3 2 completed "$CMP")")" >/dev/null
rm -f "$builds/BG3/effects/effect-BG3"; : > "$builds/BG3/effects.log"; cbset BG3 running
bash "$EC" resume-callback "$builds" BG3 >/dev/null 2>&1
{ [ "$(effects BG3)" = 1 ] && [ -e "$builds/BG3/effects/effect-BG3" ]; } && ok "(m6) effects.log lost but effect absent: resume rewrites exactly one line" || bad "(m6 lost log) $(effects BG3)"

# ---- m7: canonical form, produced independently with jq + openssl, accepted by the core ----
newbuild BJ RJ
xe=$(jq -nc --arg h $'ünï"\\/\t\U0001F600' '{schema:"build-event/1",run_id:"RJ",build_id:"BJ",variant:"primary",seq:1,kind:"accepted",host:$h,sent_at:"2026-10-05T10:00:00Z"}')
jkey=$(openssl kdf -keylen 32 -kdfopt digest:SHA256 -kdfopt hexkey:"$secret_hex" -kdfopt hexinfo:"$(LC_ALL=C; printf '%08x' 2; printf BJ | od -An -tx1 | tr -d ' \n'; printf '%08x' 2; printf RJ | od -An -tx1 | tr -d ' \n')" HKDF | tr -d ':\n' | tr A-F a-f)
jmac=$(printf '%s' "$xe" | jq -cS . | tr -d '\n' | openssl dgst -sha256 -mac HMAC -macopt hexkey:"$jkey" | awk '{print $NF}')
jev=$(printf '%s' "$xe" | jq -c --arg m "$jmac" '. + {hmac:$m}')
pycanon=$(python3 -c 'import sys,json;sys.path.insert(0,sys.argv[1]);import bev_crypto;sys.stdout.buffer.write(bev_crypto.canon(json.loads(sys.argv[2])))' "$ECDIR/lib" "$xe")
[ "$pycanon" = "$(printf '%s' "$xe" | jq -cS . | tr -d '\n')" ] && ok "(m7) canonical bytes: python canon equals jq -cS for a non-ASCII/escape event" || bad "(m7) canonical forms differ between python and jq"
out=$(consume "$(efile "$jev")"); rc=$?
{ [ $rc -eq 0 ] && printf '%s' "$out" | grep -q "consumed seq=1"; } && ok "(m7) event signed ONLY with jq + openssl (independent of bev_crypto) is accepted" || bad "(m7 cross-impl) rc=$rc out=$out"

# ---- m5: leak scan also covers state dir (minus the key), transcripts and this test's own output ----
nd2=$tmp/needle2; mkdir -p "$nd2"; printf 'x%sx' "$secret_hex" > "$nd2/transcript.log"
[ "$(scan "$secret_hex" "$nd2" none)" = 1 ] && ok "(m5) scan sees a secret planted in a transcript-shaped file (control needle)" || bad "(m5) scan blind to transcripts"
keyBA=$(py key "$secret_hex" BA RA)
sc1=$(scan "$secret_hex" "$state" "$state/build_hmac.key"); sc2=$(scan "$secret_hex" "$tmp/logs" none); sc3=$(scan "$keyBA" "$tmp/logs" none)
[ "$sc1$sc2$sc3" = 000 ] && ok "(m5) secret and derived key absent from the state dir (minus the key), consume transcripts and this test's own output (both under logs/)" || bad "(m5) leaks state=$sc1 logs(secret)=$sc2 logs(key)=$sc3"

# =====================================================================================================================
# Review round 2 (WF-REVIEW-event-core): B1 B2 I1-I5 m1-m5 and the reviewer's mutants X1-X8
# =====================================================================================================================
# ---- fault-injection shims: a PATH directory whose jq/ln/mv/openssl fail (SF_<tool>=<pattern in the args>), truncate
# their output (SFT_<tool>) or log their args (SFLOG_<tool>=<file>); every other call execs the real tool ----
fsh=$tmp/fshim; mkdir -p "$fsh"
for t in jq ln mv openssl; do
  real=$(command -v "$t")
  cat > "$fsh/$t" <<SH
#!/bin/sh
[ -n "\${SFLOG_$t:-}" ] && printf '%s\n' "\$*" >> "\$SFLOG_$t"
if [ -n "\${SF_$t:-}" ]; then case "\$*" in *"\$SF_$t"*) exit 137;; esac; fi
if [ -n "\${SFT_$t:-}" ]; then case "\$*" in *"\$SFT_$t"*) printf '{"kind":'; exit 0;; esac; fi
exec $real "\$@"
SH
  chmod +x "$fsh/$t"
done
fault() { local k=$1 v=$2; shift 2; env PATH="$fsh:$PATH" "$k=$v" "$@"; }       # fault SF_mv -T bash ...
# fsync(2) fault shim (LD_PRELOAD, compiled here): fsync of an fd whose path contains $FSYNC_FAIL (or equals $FSYNC_FAIL_EXACT) fails with EIO.
# `sync FILE` cannot be used for this: on this host it is uutils = a global sync(2) that has no error to report.
cat > "$tmp/fsfail.c" <<'C'
#define _GNU_SOURCE
#include <dlfcn.h>
#include <errno.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/stat.h>
#include <unistd.h>
int fsync(int fd) {
  static int (*real)(int);
  if (!real) real = (int (*)(int))dlsym(RTLD_NEXT, "fsync");
  const char *sub = getenv("FSYNC_FAIL"), *ex = getenv("FSYNC_FAIL_EXACT"), *dironly = getenv("FSYNC_FAIL_DIRONLY");
  struct stat st;
  if (dironly && *dironly && (fstat(fd, &st) || !S_ISDIR(st.st_mode))) return real(fd);
  if ((sub && *sub) || (ex && *ex)) {
    char l[64], p[4096]; snprintf(l, sizeof l, "/proc/self/fd/%d", fd);
    ssize_t n = readlink(l, p, sizeof p - 1);
    if (n > 0) { p[n] = 0; if ((sub && *sub && strstr(p, sub)) || (ex && *ex && !strcmp(p, ex))) { errno = EIO; return -1; } }
  }
  return real(fd);
}
C
gcc -shared -fPIC -O1 -o "$tmp/fsfail.so" "$tmp/fsfail.c" -ldl 2>"$tmp/fsfail.err" && [ -s "$tmp/fsfail.so" ] || { say "fsync fault shim did not build: $(head -3 "$tmp/fsfail.err")"; }
fsfault() { local pat=$1; shift; env LD_PRELOAD="$tmp/fsfail.so" FSYNC_FAIL="$pat" "$@"; }
fsfault_exact() { local pat=$1; shift; env LD_PRELOAD="$tmp/fsfail.so" FSYNC_FAIL_EXACT="$pat" "$@"; }
fsconsume() { local pat=$1 f=$2 o rc; o=$(fsfault "$pat" bash "$EC" consume "$builds" "$state" "$f" 2>&1); rc=$?; printf '%s\n' "$o" >> "$tmp/logs/transcript.log"; printf '%s\n' "$o"; return $rc; }
# control needle: the shim fails fsync(2) on a matching path and only there (python os.fsync, the call the core makes)
: > "$tmp/fsneedle.a"; : > "$tmp/fsneedle.b"
fspy='import os,sys\nfd=os.open(sys.argv[1],os.O_RDONLY)\ntry: os.fsync(fd)\nexcept OSError: sys.exit(1)'
oracle_begin shim
fsfault fsneedle.a python3 -c "$(printf "$fspy")" "$tmp/fsneedle.a"; r1=$?; fsfault fsneedle.a python3 -c "$(printf "$fspy")" "$tmp/fsneedle.b"; r2=$?
[ $r1 -eq 1 ] && [ $r2 -eq 0 ] && ok "(I4) control: the fsync fault shim fails fsync(2) on a matching path and only there" || bad "(I4) fsync fault shim blind or over-broad: matching rc=$r1 other rc=$r2"
oracle_end
fconsume() { local k=$1 v=$2 f=$3 o rc; o=$(fault "$k" "$v" bash "$EC" consume "$builds" "$state" "$f" 2>&1); rc=$?; printf '%s\n' "$o" >> "$tmp/logs/transcript.log"; printf '%s\n' "$o"; return $rc; }
nofail_tmp() { ! ls -d "$builds/$1"/terminal.tmp-* >/dev/null 2>&1; }              # no prepared-terminal leftover
basejq() { jq -nc --arg r "$2" --arg b "$1" --argjson s "$3" --arg k "$4" '{schema:"build-event/1",run_id:$r,build_id:$b,variant:"primary",seq:$s,kind:$k,host:"anton",sent_at:"2026-10-05T10:00:00Z"}'; }
mk() { rawev "$1" "$2" "$(basejq "$1" "$2" "$3" "$4" | jq -c "$5")"; }              # build run seq kind jq-edit -> signed event with the edit applied

# ---- B1: a failed terminal claim is an error, never "superseded"; nothing is lost ----
newbuild FB RFB; consume "$(efile "$(ev FB RFB 1 accepted)")" >/dev/null; cf=$(efile "$(ev FB RFB 2 completed "$CMP")")
out=$(fconsume SF_mv -T "$cf"); rc=$?
{ [ $rc -eq 20 ] && printf '%s' "$out" | grep -q terminal_claim_failed && [ ! -e "$builds/FB/terminal" ] && [ ! -e "$builds/FB/consumed/2" ] \
  && ! grep -q superseded "$builds/FB/events.jsonl" && nofail_tmp FB; } && ok "(B1) terminal rename fails: refused terminal_claim_failed (rc 20), no terminal, no consumed mark, never journaled superseded, no leftover" || bad "(B1 consume) rc=$rc out=$out"
out=$(consume "$cf"); rc=$?
{ [ $rc -eq 0 ] && printf '%s' "$out" | grep -q 'consumed seq=2 kind=completed' && [ "$(jq -r .kind "$builds/FB/terminal/state.json")" = completed ] && [ "$(cbs FB)" = done ] && [ "$(efiles FB)" = 1 ]; } && ok "(B1) redelivery after the fault clears completes the build: terminal, callback done, effect applied" || bad "(B1 redelivery) rc=$rc out=$out"
newbuild FC RFC; consume "$(efile "$(ev FC RFC 1 accepted)")" >/dev/null
out=$(fault SF_mv -T bash "$EC" cancel "$builds" FC 2>&1); rc=$?
{ [ $rc -eq 20 ] && printf '%s' "$out" | grep -q terminal_claim_failed && [ ! -e "$builds/FC/terminal" ] && ! grep -q superseded "$builds/FC/events.jsonl" 2>/dev/null; } && ok "(B1) cancel with a failing terminal rename: refused terminal_claim_failed, no terminal, never superseded" || bad "(B1 cancel) rc=$rc out=$out"
out=$(bash "$EC" cancel "$builds" FC 2>&1); [ "$out" = cancelled ] && [ "$(jq -r .kind "$builds/FC/terminal/state.json")" = cancelled ] && ok "(B1) cancel retried after the fault clears wins" || bad "(B1 cancel retry) out=$out"
# a terminal directory that exists but is damaged is NOT a lost race either
newbuild FD RFD; consume "$(efile "$(ev FD RFD 1 accepted)")" >/dev/null
mkdir -p "$builds/FD/terminal"; : > "$builds/FD/terminal/state.json"
out=$(bash "$EC" cancel "$builds" FD 2>&1); rc=$?
{ [ $rc -eq 20 ] && printf '%s' "$out" | grep -q terminal_claim_failed; } && ok "(B1) an existing but unparsable terminal is not reported as a lost race" || bad "(B1 damaged terminal) rc=$rc out=$out"

# ---- B1 / X-filerename: a claimer WITHOUT the per-build lock (liveness owner, host-loss path: owed) still loses atomically ----
newbuild CT RCT; consume "$(efile "$(ev CT RCT 1 accepted)")" >/dev/null; consume "$(efile "$(ev CT RCT 2 completed "$CMP")")" >/dev/null
sh1=$(sha256sum < "$builds/CT/terminal/state.json")
EC=$EC bash -c 'source "$EC"; claim_terminal "$1" cancelled 0 cancelled' _ "$builds/CT" >/dev/null 2>&1; crc=$?
{ [ $crc -eq 1 ] && [ "$sh1" = "$(sha256sum < "$builds/CT/terminal/state.json")" ] && [ "$(jq -r .kind "$builds/CT/terminal/state.json")" = completed ] && [ "$(cbs CT)" = done ] && nofail_tmp CT; } && ok "(B1) claim_terminal called directly (no lock) onto an existing terminal loses (rc 1), the terminal is untouched, no leftover" || bad "(B1 lockless claim) rc=$crc kind=$(jq -r .kind "$builds/CT/terminal/state.json")"

# ---- I1: no empty or truncated temp file is ever renamed over durable state ----
newbuild FJ RFJ; consume "$(efile "$(ev FJ RFJ 1 accepted)")" >/dev/null; cf=$(efile "$(ev FJ RFJ 2 completed "$CMP")")
out=$(fconsume SF_jq claimed_at "$cf"); rc=$?
{ [ $rc -eq 20 ] && printf '%s' "$out" | grep -q terminal_claim_failed && [ ! -e "$builds/FJ/terminal" ] && [ ! -e "$builds/FJ/consumed/2" ] && nofail_tmp FJ; } && ok "(I1) jq killed while writing the terminal record: refused, no empty terminal/state.json, nothing consumed" || bad "(I1 jq killed) rc=$rc out=$out"
newbuild FK RFK; consume "$(efile "$(ev FK RFK 1 accepted)")" >/dev/null; cf=$(efile "$(ev FK RFK 2 completed "$CMP")")
out=$(fconsume SFT_jq claimed_at "$cf"); rc=$?
{ [ $rc -eq 20 ] && [ ! -e "$builds/FK/terminal" ] && [ ! -e "$builds/FK/consumed/2" ] && nofail_tmp FK; } && ok "(I1) jq exits 0 but leaves a TRUNCATED terminal record: refused, nothing renamed into place" || bad "(I1 jq truncated) rc=$rc out=$out"
out=$(consume "$cf"); [ "$(cbs FK)" = done ] && [ "$(jq -r .exit_class "$builds/FK/terminal/state.json")" = succeeded ] && ok "(I1) the same event redelivered after the fault completes with a whole terminal record" || bad "(I1 redelivery) out=$out"
oracle_begin shim
newbuild FS RFS; cf=$(efile "$(ev FS RFS 1 accepted)")
out=$(fsconsume /FS/tmp/w. "$cf"); rc=$?
{ [ $rc -eq 20 ] && printf '%s' "$out" | grep -q state_write_failed && [ ! -e "$builds/FS/consumed/1" ] && ! ls "$builds/FS/tmp"/w.* >/dev/null 2>&1; } && ok "(I1) a failed temp-file sync is not renamed over the consumed mark: refused state_write_failed, no mark, no leftover temp" || bad "(I1 consumed mark) rc=$rc out=$out"
out=$(consume "$cf"); printf '%s' "$out" | grep -q 'consumed seq=1' && ok "(I1) accepted redelivered after the fault is consumed" || bad "(I1 mark redelivery) $out"
newbuild FP RFP; consume "$(efile "$(ev FP RFP 1 accepted)")" >/dev/null; consume "$(efile "$(ev FP RFP 2 heartbeat "$HB")")" >/dev/null
pj=$(cat "$builds/FP/progress.json"); jl=$(wc -l < "$builds/FP/events.jsonl")
out=$(fsconsume /FP/tmp/w. "$(efile "$(ev FP RFP 3 heartbeat '{"progress_offset":99,"stage":2,"elapsed_monotonic_ms":5}')")"); rc=$?
{ [ $rc -eq 20 ] && printf '%s' "$out" | grep -q state_write_failed && [ "$(cat "$builds/FP/progress.json")" = "$pj" ] && [ "$(wc -l < "$builds/FP/events.jsonl")" = "$jl" ] && [ ! -e "$builds/FP/consumed/3" ]; } && ok "(I1) progress.json write failing: refused, previous progress.json intact, nothing journaled or consumed" || bad "(I1 progress) rc=$rc out=$out"
newbuild FT RFT; consume "$(efile "$(ev FT RFT 1 accepted)")" >/dev/null; consume "$(efile "$(ev FT RFT 2 completed "$CMP")")" >/dev/null
cbset FT claimed; rm -rf "$builds/FT/effects" "$builds/FT/effects.log"
out=$(fsfault /FT/tmp/w. bash "$EC" resume-callback "$builds" FT 2>&1); rc=$?
{ [ $rc -ne 0 ] && [ "$(cbs FT)" = claimed ] && [ "$(efiles FT)" = 0 ]; } && ok "(I1) callback.state write failing: resume-callback exits non-zero, callback.state keeps its previous whole value, no effect run" || bad "(I1 callback.state) rc=$rc state='$(cbs FT)' out=$out"
out=$(bash "$EC" resume-callback "$builds" FT 2>&1); rc=$?
{ [ $rc -eq 0 ] && [ "$(cbs FT)" = done ] && [ "$(efiles FT)" = 1 ]; } && ok "(I1) resume-callback after the fault clears completes the callback" || bad "(I1 resume) rc=$rc out=$out"
oracle_end

# ---- I2: a failed effect is never marked done ----
newbuild FL RFL; consume "$(efile "$(ev FL RFL 1 accepted)")" >/dev/null; cf=$(efile "$(ev FL RFL 2 completed "$CMP")")
out=$(fconsume SF_ln /effects/ "$cf"); rc=$?
{ [ $rc -eq 0 ] && printf '%s' "$out" | grep -q 'callback_state=failed' && [ "$(cbs FL)" = failed ] && [ "$(efiles FL)" = 0 ] && [ -s "$builds/FL/terminal/callback.reason" ] && [ "$(jq -r .kind "$builds/FL/terminal/state.json")" = completed ]; } && ok "(I2) ln fails: terminal stays completed, callback.state=failed with a reason, no effect file, never done" || bad "(I2 ln) rc=$rc state='$(cbs FL)' efiles=$(efiles FL) out=$out"
out=$(fault SF_ln /effects/ bash "$EC" resume-callback "$builds" FL 2>&1); rc=$?
{ [ $rc -ne 0 ] && [ "$(cbs FL)" = failed ]; } && ok "(I2) resume-callback under the same fault exits non-zero and stays failed" || bad "(I2 resume fault) rc=$rc state=$(cbs FL)"
out=$(bash "$EC" resume-callback "$builds" FL 2>&1); rc=$?
{ [ $rc -eq 0 ] && [ "$(cbs FL)" = done ] && [ "$(efiles FL)" = 1 ] && [ "$(effects FL)" = 1 ]; } && ok "(I2) resume-callback after the fault clears applies the effect once (one audit line, one effect file)" || bad "(I2 repair) rc=$rc state=$(cbs FL) efiles=$(efiles FL) log=$(effects FL)"

# ---- I3: a done callback is not re-entered by a runner that was waiting on the callback lock ----
newbuild FQ RFQ; consume "$(efile "$(ev FQ RFQ 1 accepted)")" >/dev/null; consume "$(efile "$(ev FQ RFQ 2 completed "$CMP")")" >/dev/null
rm -rf "$builds/FQ/effects" "$builds/FQ/effects.log"; cbset FQ claimed
( exec 7>>"$builds/FQ/.cblock"; flock 7; : > "$tmp/fq.locked"; n=0; while [ ! -e "$tmp/fq.go" ] && [ $n -lt 200 ]; do sleep 0.05; n=$((n+1)); done; printf 'done\n' > "$builds/FQ/terminal/callback.state" ) & hp=$!
waitfor "$tmp/fq.locked"
( bash "$EC" resume-callback "$builds" FQ > "$tmp/fq.out" 2>&1 ) & rp=$!
sleep 0.5; : > "$tmp/fq.go"; wait $hp; fi1=$(stat -c %i "$builds/FQ/terminal/callback.state"); wait $rp
{ [ "$(cbs FQ)" = done ] && [ "$fi1" = "$(stat -c %i "$builds/FQ/terminal/callback.state")" ] && [ "$(efiles FQ)" = 0 ] && [ ! -e "$builds/FQ/effects.log" ]; } && ok "(I3) runner that waited on .cblock re-checks the state after the lock: a callback another runner finished is not re-entered" || bad "(I3) state=$(cbs FQ) efiles=$(efiles FQ) out=$(cat "$tmp/fq.out")"

# ---- I4: secret-init guards the script's OWN checkout and reports success only when a key exists ----
mkdir -p "$tmp/co2/scripts/build"; cp "$EC" "$tmp/co2/scripts/build/event_core.sh"; cp -r "$ECDIR/lib" "$tmp/co2/scripts/build/lib"; rm -rf "$tmp/co2/scripts/build/lib/__pycache__"
out=$(bash "$tmp/co2/scripts/build/event_core.sh" secret-init "$tmp/co2/statex" /nonexistent 2>&1); rc=$?
{ [ $rc -eq 20 ] && printf '%s' "$out" | grep -q secret_dir_inside_checkout && [ ! -e "$tmp/co2/statex" ] && ! printf '%s' "$out" | grep -q 'secret created'; } && ok "(I4) secret-init refuses a state dir inside the script's own checkout even when the checkout argument names another path" || bad "(I4 own checkout) rc=$rc out=$out"
mkdir -p "$tmp/ro"; chmod 500 "$tmp/ro"
out=$(bash "$EC" secret-init "$tmp/ro/st" "$root" 2>&1); rc=$?; chmod 700 "$tmp/ro"
{ [ $rc -eq 20 ] && printf '%s' "$out" | grep -q secret_create_failed && [ ! -e "$tmp/ro/st" ] && ! printf '%s' "$out" | grep -q 'secret created'; } && ok "(I4) secret-init into an unwritable parent: refused secret_create_failed, not 'secret created'" || bad "(I4 unwritable) rc=$rc out=$out"

# ---- B2: secret creation and reading refuse an empty or short key ----
s=$tmp/sb_ossl; out=$(fault SF_openssl rand bash "$EC" secret-init "$s" "$root" 2>&1); rc=$?
{ [ $rc -eq 20 ] && printf '%s' "$out" | grep -q secret_create_failed && [ ! -e "$s/build_hmac.key" ] && [ ! -e "$s/build_hmac.key.meta" ] && ! printf '%s' "$out" | grep -q 'secret created'; } && ok "(B2) openssl fails during secret-init: refused secret_create_failed, no key, no meta, no 'secret created'" || bad "(B2 openssl) rc=$rc out=$out"
s=$tmp/sb_trunc; out=$(fault SFT_openssl rand bash "$EC" secret-init "$s" "$root" 2>&1); rc=$?
{ [ $rc -eq 20 ] && [ ! -e "$s/build_hmac.key" ] && ! printf '%s' "$out" | grep -q 'secret created'; } && ok "(B2) openssl exits 0 but writes a malformed key: refused, no key left" || bad "(B2 openssl malformed) rc=$rc out=$out"
s=$tmp/sb_ok; bash "$EC" secret-init "$s" "$root" >/dev/null 2>&1
grep -Eq '^[0-9a-f]{64}$' "$s/build_hmac.key" && [ "$(wc -c < "$s/build_hmac.key")" = 65 ] && ok "(B2) a created key is exactly 64 lowercase hex characters" || bad "(B2 created key shape)"
for kc in 'empty|' 'one byte|00' "sixty-three hex|$(printf 'a%.0s' $(seq 63))" "sixty-six hex|$(printf 'a%.0s' $(seq 66))"; do
  IFS='|' read -r nm kv <<<"$kc"; s=$tmp/sb_len_$RANDOM; mkdir_state "$s" "$kv"; sbad "$nm key" "$s" key_bad_length
done
s=$tmp/sb_nothex; mkdir_state "$s" "$(printf 'z%.0s' $(seq 64))"; sbad "non-hex key" "$s" key_not_hex

# ---- I5 / reviewer mutants X1-X8: fixtures that make each of them fail ----
# X1: a second accepted
newbuild XA RXA; consume "$(efile "$(ev XA RXA 1 accepted)")" >/dev/null; consume "$(efile "$(ev XA RXA 2 heartbeat "$HB")")" >/dev/null
out=$(consume "$(efile "$(ev XA RXA 3 accepted)")"); rc=$?
{ [ $rc -eq 20 ] && printf '%s' "$out" | grep -q 'illegal_transition second accepted' && [ ! -e "$builds/XA/consumed/3" ]; } && ok "(X1) a second accepted event is refused (illegal_transition second accepted)" || bad "(X1) rc=$rc out=$out"
# X2: terminal recorded, consumed mark missing (crash window), then redelivery
newbuild XB RXB; consume "$(efile "$(ev XB RXB 1 accepted)")" >/dev/null; consume "$(efile "$(ev XB RXB 2 completed "$CMP")")" >/dev/null
rm -f "$builds/XB/consumed/2"; jl=$(wc -l < "$builds/XB/events.jsonl")
out=$(consume "$(efile "$(ev XB RXB 2 completed "$CMP")")"); rc=$?
{ [ $rc -eq 0 ] && printf '%s' "$out" | grep -q '^DUP' && [ "$(wc -l < "$builds/XB/events.jsonl")" = "$jl" ]; } && ok "(X2) same completed redelivered with the consumed mark missing: DUP, not journaled again" || bad "(X2 dup) rc=$rc out=$out"
out=$(consume "$(efile "$(ev XB RXB 2 completed "$(jq -c '.exit_class="build_failed"' <<<"$CMP")")")"); rc=$?
{ [ $rc -eq 20 ] && printf '%s' "$out" | grep -q seq_conflict && [ "$(jq -r .exit_class "$builds/XB/terminal/state.json")" = succeeded ]; } && ok "(X2) different completed content for the claimed seq (mark missing): seq_conflict, verdict unchanged" || bad "(X2 conflict) rc=$rc out=$out"
# X3 X4 X7 + schema caps (m5): refused by the core AND by the schema
newbuild XC RXC; consume "$(efile "$(ev XC RXC 1 accepted)")" >/dev/null
big=$(python3 -c 'print("h"*1025)'); ok1024=$(python3 -c 'print("h"*1024)')
xchk() { # name edit reason
  # round 4 (m10): every fixture is the otherwise VALID next heartbeat (progress 1/0 after the accepted event), so the mutated rule is the only reason to refuse
  local e; e=$(mk XC RXC 2 heartbeat ". + {progress_offset:1,stage:0,elapsed_monotonic_ms:1} | $2"); out=$(consume "$(efile "$e")"); rc=$?
  if [ $rc -eq 20 ] && printf '%s' "$out" | grep -q "$3" && [ ! -e "$builds/XC/consumed/2" ]; then ok "(X) $1 refused ($3)"; else bad "(X $1) rc=$rc out=$out"; fi
  validate "$e" && bad "(X $1) the schema accepts what the core refuses" || ok "(X) $1 also rejected by the schema"; }
xchk "variant evil" '.variant="evil"' 'event_malformed variant$'     # the enum check itself, not the submit.json binding (whose reason reads "variant differs ...")
xchk "schema build-event/2" '.schema="build-event/2"' event_malformed
xchk "empty host" '.host=""' event_malformed
xchk "empty sent_at" '.sent_at=""' event_malformed
xchk "run_id longer than 128" '.run_id="'"$(python3 -c 'print("r"*129)')"'"' event_malformed
xchk "host of 1025 characters" '.host="'"$big"'"' event_malformed
xchk "sent_at of 1025 characters" '.sent_at="'"$big"'"' event_malformed
e=$(mk XC RXC 2 heartbeat '. + {progress_offset:1,stage:0,elapsed_monotonic_ms:1} | .host="'"$ok1024"'"'); out=$(validate "$e" && consume "$(efile "$e")")
printf '%s' "$out" | grep -q 'consumed seq=2' && ok "(m5) a host of exactly 1024 characters is valid for the schema and the core" || bad "(m5 1024) out=$out"
newbuild XD RXD; consume "$(efile "$(ev XD RXD 1 accepted)")" >/dev/null
e=$(mk XD RXD 2 completed '. + {exit_class:"succeeded",artifact_manifest_sha256:"'"$(printf a | sha256sum | cut -d' ' -f1)"'",image_digest:"'"$big"'",remote_log_sha256:"'"$(printf b | sha256sum | cut -d' ' -f1)"'"}')
out=$(consume "$(efile "$e")"); rc=$?
{ [ $rc -eq 20 ] && printf '%s' "$out" | grep -q event_malformed; } && ok "(m5) image_digest of 1025 characters refused by the core" || bad "(m5 image_digest) rc=$rc out=$out"
validate "$e" && bad "(m5) the schema accepts an image_digest of 1025 characters" || ok "(m5) the schema also rejects an image_digest of 1025 characters"
# X5: the 64 KiB cap, exactly at and one byte over
newbuild XE RXE
padev() { python3 - "$1" "$2" <<'PY'
import sys
e = sys.argv[1].encode(); n = int(sys.argv[2]); sys.stdout.buffer.write(e + b" " * (n - len(e) - 1) + b"\n")
PY
}
e=$(ev XE RXE 1 accepted); padev "$e" 65537 > "$tmp/pad65537.json"; padev "$e" 65536 > "$tmp/pad65536.json"
[ "$(wc -c < "$tmp/pad65537.json")" = 65537 ] && [ "$(wc -c < "$tmp/pad65536.json")" = 65536 ] || bad "(X5) pad fixture sizes"
out=$(consume "$tmp/pad65537.json"); rc=$?
{ [ $rc -eq 20 ] && printf '%s' "$out" | grep -q 'event_malformed.*too large' && [ ! -e "$builds/XE/consumed/1" ]; } && ok "(X5) an event of 65537 bytes is refused (too large)" || bad "(X5 over) rc=$rc out=$out"
out=$(consume "$tmp/pad65536.json"); rc=$?
{ [ $rc -eq 0 ] && printf '%s' "$out" | grep -q 'consumed seq=1'; } && ok "(X5) an event of exactly 65536 bytes is accepted (the cap is inclusive)" || bad "(X5 boundary) rc=$rc out=$out"
# X6: a corrupt progress.json
newbuild XF RXF; consume "$(efile "$(ev XF RXF 1 accepted)")" >/dev/null; consume "$(efile "$(ev XF RXF 2 heartbeat "$HB")")" >/dev/null
printf 'garbage\n' > "$builds/XF/progress.json"
out=$(consume "$(efile "$(ev XF RXF 3 heartbeat "$HB")")"); rc=$?
{ [ $rc -eq 20 ] && printf '%s' "$out" | grep -q state_corrupt && [ ! -e "$builds/XF/consumed/3" ] && ! printf '%s' "$out" | grep -q 'integer expected'; } && ok "(X6) a corrupt progress.json is refused (state_corrupt), no shell error, nothing consumed" || bad "(X6) rc=$rc out=$out"
# X8 is B1 above (a failed claim must not be a win)

# ---- m1: binding to submit.json; the first event must be seq 1 ----
newbuild MA RMA
e=$(rawev MA RMA "$(basejq MA OTHERRUN 1 accepted)"); out=$(consume "$(efile "$e")"); rc=$?
{ [ $rc -eq 20 ] && printf '%s' "$out" | grep -q 'event_malformed.*run_id' && [ ! -e "$builds/MA/consumed/1" ]; } && ok "(m1) run_id field differing from submit.json, signed under the current key: refused" || bad "(m1 run_id) rc=$rc out=$out"
e=$(mk MA RMA 1 accepted '.variant="repro-cold"'); out=$(consume "$(efile "$e")"); rc=$?
{ [ $rc -eq 20 ] && printf '%s' "$out" | grep -q 'event_malformed.*variant' && [ ! -e "$builds/MA/consumed/1" ]; } && ok "(m1) variant differing from submit.json: refused" || bad "(m1 variant) rc=$rc out=$out"
out=$(consume "$(efile "$(ev MA RMA 5 accepted)")"); rc=$?
{ [ $rc -eq 20 ] && printf '%s' "$out" | grep -q illegal_transition && [ ! -e "$builds/MA/consumed/5" ]; } && ok "(m1) the first event with seq 5 is refused (T005a: seq from 1)" || bad "(m1 first seq) rc=$rc out=$out"
out=$(consume "$(efile "$(ev MA RMA 1 accepted)")"); printf '%s' "$out" | grep -q 'consumed seq=1' && ok "(m1) the correct first event is consumed after the refused ones" || bad "(m1 ok) $out"

# ---- m2: lock files are never truncated through a symlink, a symlinked lock is refused ----
newbuild SL RSL; printf 'KEEP\n' > "$tmp/victim.lock"; ln -s "$tmp/victim.lock" "$builds/SL/.lock"
out=$(consume "$(efile "$(ev SL RSL 1 accepted)")"); rc=$?
out2=$(bash "$EC" cancel "$builds" SL 2>&1); rc2=$?
{ [ $rc -eq 20 ] && printf '%s' "$out" | grep -q lock_unavailable && [ $rc2 -eq 20 ] && printf '%s' "$out2" | grep -q lock_unavailable && [ "$(cat "$tmp/victim.lock")" = KEEP ] && [ ! -e "$builds/SL/consumed/1" ]; } && ok "(m2) symlinked .lock: consume and cancel refuse (lock_unavailable), the link target is untouched" || bad "(m2 .lock) rc=$rc rc2=$rc2 victim='$(cat "$tmp/victim.lock")' out=$out"
newbuild SK RSK; consume "$(efile "$(ev SK RSK 1 accepted)")" >/dev/null; consume "$(efile "$(ev SK RSK 2 completed "$CMP")")" >/dev/null
printf 'KEEP\n' > "$tmp/victim.cb"; cbset SK claimed; rm -f "$builds/SK/.cblock"; ln -s "$tmp/victim.cb" "$builds/SK/.cblock"
out=$(bash "$EC" resume-callback "$builds" SK 2>&1); rc=$?
{ [ $rc -ne 0 ] && [ "$(cat "$tmp/victim.cb")" = KEEP ] && [ "$(cbs SK)" = claimed ]; } && ok "(m2) symlinked .cblock: resume-callback refuses, the link target is untouched, state unchanged" || bad "(m2 .cblock) rc=$rc victim='$(cat "$tmp/victim.cb")' state=$(cbs SK)"

# ---- m3: effect_key is a safe token, used literally ----
ekn=0
for ek in '../../ESCAPED' 'a.b' 'x*' 'with space'; do
  ekn=$((ekn+1)); b=EK$ekn; newbuild $b REK; printf '{"callback_id":"cb","effect_key":"%s"}\n' "$ek" > "$builds/$b/callback.json"
  consume "$(efile "$(ev $b REK 1 accepted)")" >/dev/null; consume "$(efile "$(ev $b REK 2 completed "$CMP")")" >/dev/null
  { [ "$(cbs $b)" = failed ] && grep -q effect_key_invalid "$builds/$b/terminal/callback.reason" && [ ! -e "$builds/ESCAPED" ] && [ "$(efiles $b)" = 0 ] && [ ! -e "$builds/$b/effects.log" ]; } && ok "(m3) effect_key '$ek' refused: callback failed (effect_key_invalid), nothing written" || bad "(m3 '$ek') state=$(cbs $b) efiles=$(efiles $b)"
done
b=EKL; newbuild $b REKL; printf '{"callback_id":"cb","effect_key":"k-1_ok"}\n' > "$builds/$b/callback.json"
consume "$(efile "$(ev $b REKL 1 accepted)")" >/dev/null; consume "$(efile "$(ev $b REKL 2 completed "$CMP")")" >/dev/null
printf 'k-1_oka 2026\n' > "$builds/$b/effects.log"; rm -f "$builds/EKL/effects/k-1_ok"; cbset $b running
bash "$EC" resume-callback "$builds" $b >/dev/null 2>&1
{ [ "$(effects $b)" = 2 ] && [ -e "$builds/$b/effects/k-1_ok" ]; } && ok "(m3) the audit lookup is literal and whole-key: a log line of a longer key does not count as this key's line" || bad "(m3 literal) log lines=$(effects $b)"

b=EKH; newbuild $b REKH; printf '{"callback_id":"cb","effect_key":"-rf"}\n' > "$builds/$b/callback.json"
consume "$(efile "$(ev $b REKH 1 accepted)")" >/dev/null; consume "$(efile "$(ev $b REKH 2 completed "$CMP")")" >/dev/null
{ [ "$(cbs $b)" = done ] && [ -e "$builds/$b/effects/-rf" ] && [ "$(effects $b)" = 1 ]; } && ok "(m3) a key that starts with '-' is a valid token and is used literally (never as an option)" || bad "(m3 hyphen) state=$(cbs $b) efiles=$(efiles $b)"

# ---- m4: the journal line is ONE write(2); a journal failure is an error ----
jl8=$(python3 -c 'print("{\"event\":\"x\",\"pad\":\"" + "é"*4000 + "\"}")'); jn=$(printf '%s' "$jl8" | wc -c)
oracle_begin strace
mkdir -p "$tmp/jr"; rm -f "$tmp/jr.strace" "$tmp/jr/events.jsonl"
strace -f -e trace=write -o "$tmp/jr.strace" bash -c 'source "$1"; journal "$2" "$3"' _ "$EC" "$tmp/jr" "$jl8" >/dev/null 2>&1
{ grep -Eq "write\([0-9]+, \".*, $((jn+1))\) += $((jn+1))\$" "$tmp/jr.strace" && [ "$(cat "$tmp/jr/events.jsonl")" = "$jl8" ] && [ "$(wc -l < "$tmp/jr/events.jsonl")" = 1 ]; } && ok "(m4) a journal line of $((jn+1)) bytes is written with a single write(2) (strace)" || bad "(m4 single write) strace: $(grep -c . "$tmp/jr.strace" 2>/dev/null) lines; wanted a write of $((jn+1)) bytes"
oracle_end
newbuild JF RJF; consume "$(efile "$(ev JF RJF 1 accepted)")" >/dev/null; rm -f "$builds/JF/events.jsonl"; mkdir "$builds/JF/events.jsonl"
out=$(consume "$(efile "$(ev JF RJF 2 heartbeat "$HB")")"); rc=$?
{ [ $rc -eq 20 ] && printf '%s' "$out" | grep -q journal_failed && [ ! -e "$builds/JF/consumed/2" ]; } && ok "(m4) a journal write that fails refuses (journal_failed) and writes no consumed mark" || bad "(m4 journal fault) rc=$rc out=$out"

# ---- I2 (EEXIST): ln fails because a concurrent writer already created the keyed effect: success, not a failure ----
newbuild EE REE; consume "$(efile "$(ev EE REE 1 accepted)")" >/dev/null; consume "$(efile "$(ev EE REE 2 completed "$CMP")")" >/dev/null
rm -f "$builds/EE/effects/effect-EE"; cbset EE claimed
EC=$EC bash -c 'source "$EC"; ln() { command ln "$@"; return 1; }; run_callback "$1"' _ "$builds/EE" >/dev/null 2>&1
{ [ "$(cbs EE)" = done ] && [ -e "$builds/EE/effects/effect-EE" ]; } && ok "(I2) ln reporting failure while the effect file exists (concurrent creator) is success: callback done" || bad "(I2 EEXIST) state=$(cbs EE)"

# ---- m4: journal failures at the other two writers ----
newbuild JL RJL; consume "$(efile "$(ev JL RJL 1 accepted)")" >/dev/null; bash "$EC" cancel "$builds" JL >/dev/null 2>&1
rm -f "$builds/JL/events.jsonl"; mkdir "$builds/JL/events.jsonl"
out=$(consume "$(efile "$(ev JL RJL 2 completed "$CMP")")"); rc=$?
{ [ $rc -eq 20 ] && printf '%s' "$out" | grep -q journal_failed && [ ! -e "$builds/JL/consumed/2" ]; } && ok "(m4) late_ignored whose journal write fails is refused (journal_failed), not acknowledged" || bad "(m4 late journal) rc=$rc out=$out"
newbuild JT RJT; consume "$(efile "$(ev JT RJT 1 accepted)")" >/dev/null; rm -f "$builds/JT/events.jsonl"; mkdir "$builds/JT/events.jsonl"
out=$(bash "$EC" cancel "$builds" JT 2>&1); rc=$?
{ [ $rc -eq 20 ] && printf '%s' "$out" | grep -q journal_failed && ! printf '%s' "$out" | grep -qx cancelled; } && ok "(m4) a terminal claimed but not journaled is reported as an error (journal_failed), not as 'cancelled'" || bad "(m4 terminal journal) rc=$rc out=$out"

# ---- m7 (T005b naming): terminal.tmp-<pid>-<start-time>/ renamed onto terminal/, callback.state inside it ----
newbuild NM RNM; consume "$(efile "$(ev NM RNM 1 accepted)")" >/dev/null; : > "$tmp/mv.log"
out=$(env PATH="$fsh:$PATH" SFLOG_mv="$tmp/mv.log" bash "$EC" consume "$builds" "$state" "$(efile "$(ev NM RNM 2 completed "$CMP")")" 2>&1)
{ grep -Eq -- "^-T .*/NM/terminal\.tmp-[0-9]+-[0-9]+ .*/NM/terminal\$" "$tmp/mv.log" && [ "$(cbs NM)" = done ] && jq -e 'has("callback_state")|not' "$builds/NM/terminal/state.json" >/dev/null; } && ok "(m7) terminal prepared as terminal.tmp-<pid>-<start-time>/ and renamed onto terminal/; callback.state is its own file, not a state.json field" || bad "(m7) mv log: $(head -3 "$tmp/mv.log")"

# =====================================================================================================================
# Review round 3 (WF2-REVIEW event-core): I1-I6, the reviewer's mutants Y1-Y6, minors m4 m7 m8 m9 m12
# =====================================================================================================================
# ---- I1: the anchors accept no trailing newline; the directory acted on is the verified one; an ECMA-262 engine is the schema oracle ----
# python-jsonschema evaluates `pattern` with re.search, where $ also matches before a final LF; JSON Schema mandates ECMA-262, so the pattern
# oracle here is node (RegExp), independent of both the core and python.
ecma() { node -e '
const s = JSON.parse(require("fs").readFileSync(process.argv[1], "utf8")); const ev = JSON.parse(process.argv[2]);
for (const [k, p] of Object.entries(s.properties)) if (p.pattern !== undefined && k in ev && typeof ev[k] === "string" && !new RegExp(p.pattern).test(ev[k])) process.exit(1);' "$SCHEMA" "$1"; }
oracle_begin node; ecma "$(ev B0 R0 1 accepted)" && ! ecma "$(ev B0 R0 1 accepted | jq -c '.build_id="bad id"')" && ok "(I1) control: the ECMA-262 oracle accepts a good event and rejects a build_id with a space" || bad "(I1) ECMA oracle blind"; oracle_end
bl=$'BXLF\n'; mkdir -p "$builds/$bl"; jq -n --arg b "$bl" --arg r RLF '{build_id:$b,run_id:$r,variant:"primary"}' > "$builds/$bl/submit.json"
e=$(rawev "$bl" RLF "$(basejq "$bl" RLF 1 accepted)"); out=$(consume "$(efile "$e")"); rc=$?
{ [ $rc -eq 20 ] && printf '%s' "$out" | grep -q 'event_malformed build_id' && [ ! -e "$builds/BXLF\\n" ] && [ ! -d "$builds/$bl/consumed" ] && [ ! -e "$builds/$bl/events.jsonl" ]; } && ok "(I1) a build_id with a trailing LF is refused (event_malformed build_id); no state in the verified or the backslash-n directory" || bad "(I1 build_id LF) rc=$rc out=$out"
pv=$(printf '%s' "$e" | python3 "$ECDIR/lib/bev_crypto.py" verify-event "$state" "$builds" "$root" 2>&1)
[ "$pv" = "malformed:build_id" ] && ok "(I1) the python verifier itself (not only the bash side) refuses the trailing-LF build_id: malformed:build_id" || bad "(I1 verifier build_id) got '$pv'"
oracle_begin node; ! ecma "$e" && ok "(I1) the ECMA-262 oracle also rejects the trailing-LF build_id" || bad "(I1) ECMA accepts the trailing-LF build_id"; oracle_end
newbuild XG RXG; consume "$(efile "$(ev XG RXG 1 accepted)")" >/dev/null
cm=$(jq -c '.artifact_manifest_sha256 += "\n"' <<<"$CMP"); e=$(mk XG RXG 2 completed ". + $cm"); out=$(consume "$(efile "$e")"); rc=$?
{ [ $rc -eq 20 ] && printf '%s' "$out" | grep -q 'event_malformed completed fields' && [ ! -e "$builds/XG/terminal" ] && [ ! -e "$builds/XG/consumed/2" ]; } && ok "(I1) artifact_manifest_sha256 with a trailing LF is refused (completed fields); no terminal" || bad "(I1 manifest LF) rc=$rc out=$out"
oracle_begin node; ! ecma "$e" && ok "(I1) the ECMA-262 oracle also rejects the trailing-LF artifact_manifest_sha256" || bad "(I1) ECMA accepts trailing-LF artifact_manifest_sha256"; oracle_end
lg=$(jq -c '.remote_log_sha256 += "\n"' <<<"$CMP"); e=$(mk XG RXG 2 completed ". + $lg"); out=$(consume "$(efile "$e")"); rc=$?
{ [ $rc -eq 20 ] && printf '%s' "$out" | grep -q 'event_malformed completed fields' && [ ! -e "$builds/XG/terminal" ]; } && ok "(I1) remote_log_sha256 with a trailing LF is refused (completed fields)" || bad "(I1 log LF) rc=$rc out=$out"
oracle_begin node; ! ecma "$e" && ok "(I1) the ECMA-262 oracle also rejects the trailing-LF remote_log_sha256" || bad "(I1) ECMA accepts trailing-LF remote_log_sha256"; oracle_end
e=$(ev XG RXG 2 completed "$CMP" | jq -c '.hmac += "\n"'); out=$(consume "$(efile "$e")"); rc=$?
{ [ $rc -eq 20 ] && printf '%s' "$out" | grep -q event_unauthenticated && [ ! -e "$builds/XG/terminal" ]; } && ok "(I1) an hmac with a trailing LF is refused (event_unauthenticated)" || bad "(I1 hmac LF) rc=$rc out=$out"
oracle_begin node; ! ecma "$e" && ok "(I1) the ECMA-262 oracle also rejects the trailing-LF hmac" || bad "(I1) ECMA accepts trailing-LF hmac"; oracle_end
# bash side: whatever the verifier reports, the build id bash acts on must be a plain token (a stand-in verifier reports a build_id with a real LF,
# which jq @tsv would turn into the two characters backslash-n: a different directory from the one the verifier checked)
cat > "$tmp/fake_verifier.py" <<'PY'
import json, sys
sys.stdin.buffer.read()
ev = {"build_id": "BXFAKE\n", "run_id": "R", "seq": 1, "kind": "accepted", "variant": "primary", "host": "h", "sent_at": "s", "schema": "build-event/1", "hmac": "0" * 64}
print("ok"); print(json.dumps(ev, sort_keys=True, separators=(",", ":"), ensure_ascii=False)); print("0" * 64)
PY
out=$(EC=$EC FV="$tmp/fake_verifier.py" bash -c 'source "$EC"; CRYPTO=$FV; cmd_consume "$@"' _ "$builds" "$state" "$(efile x)" 2>&1); rc=$?
{ [ $rc -eq 20 ] && printf '%s' "$out" | grep -q 'event_malformed' && ! ls -d "$builds"/BXFAKE* >/dev/null 2>&1; } && ok "(I1) bash refuses a verified build_id that is not a plain token (event_malformed), no directory created" || bad "(I1 bash side) rc=$rc out=$out"

# ---- I2: a corrupt, unreadable or keyless callback.json fails closed (failed, never done); resume-callback repairs or refuses ----
cbcase() { # name reason setup-command repair-command
  local nm=$1 why=$2 b=CJ$((++cjn)); newbuild $b RCJ; eval "$3"
  consume "$(efile "$(ev $b RCJ 1 accepted)")" >/dev/null; out=$(consume "$(efile "$(ev $b RCJ 2 completed "$CMP")")"); rc=$?
  { [ $rc -eq 0 ] && printf '%s' "$out" | grep -q 'callback_state=failed' && [ "$(cbs $b)" = failed ] && [ -s "$builds/$b/terminal/callback.reason" ] && [ "$(efiles $b)" = 0 ] && [ ! -e "$builds/$b/effects.log" ] \
    && [ "$(jq -r .kind "$builds/$b/terminal/state.json")" = completed ] && [ "$(cat "$builds/$b/terminal/callback.reason")" = "$why" ]; } && ok "(I2) $nm: callback failed ($(cat "$builds/$b/terminal/callback.reason" 2>/dev/null)), never done, no effect" || bad "(I2 $nm) rc=$rc state='$(cbs $b)' efiles=$(efiles $b) out=$out"
  out=$(bash "$EC" resume-callback "$builds" $b 2>&1); rc=$?
  { [ $rc -eq 21 ] && [ "$(cbs $b)" = failed ] && [ "$(efiles $b)" = 0 ]; } && ok "(I2) $nm: resume-callback while still broken refuses (exit 21, stays failed)" || bad "(I2 $nm resume broken) rc=$rc state=$(cbs $b)"
  eval "$4"; out=$(bash "$EC" resume-callback "$builds" $b 2>&1); rc=$?
  { [ $rc -eq 0 ] && [ "$(cbs $b)" = done ] && [ "$(efiles $b)" = 1 ] && [ "$(effects $b)" = 1 ]; } && ok "(I2) $nm: after the callback record is repaired, resume-callback applies the effect once" || bad "(I2 $nm repair) rc=$rc state=$(cbs $b) efiles=$(efiles $b)"
}
cjn=0
goodcb='printf "{\"callback_id\":\"cb\",\"effect_key\":\"effect-%s\"}\n" "$b" > "$builds/$b/callback.json"; chmod 600 "$builds/$b/callback.json"'
cbcase "truncated callback.json" effect_unreadable 'printf "{\"callback_id\":\"cb\",\"effect_key\":" > "$builds/$b/callback.json"' "$goodcb"
cbcase "empty callback.json" effect_unreadable ': > "$builds/$b/callback.json"' "$goodcb"
cbcase "mode-000 (unreadable) callback.json" effect_unreadable 'chmod 000 "$builds/$b/callback.json"' 'chmod 600 "$builds/$b/callback.json"'
cbcase "callback.json without effect_key" effect_key_missing 'printf "{\"callback_id\":\"cb\"}\n" > "$builds/$b/callback.json"' "$goodcb"
cbcase "callback.json with a non-string effect_key" effect_key_missing 'printf "{\"callback_id\":\"cb\",\"effect_key\":5}\n" > "$builds/$b/callback.json"' "$goodcb"
cbcase "absent callback.json" effect_unreadable 'rm -f "$builds/$b/callback.json"' "$goodcb"

# ---- I3: a damaged terminal directory is never a completed event's late_ignored ----
dmg() { # name setup  (terminal made by `setup`; a completed and a heartbeat after it must be refused, nothing journaled as late_ignored)
  local nm=$1 b=TD$((++tdn)); newbuild $b RTD; consume "$(efile "$(ev $b RTD 1 accepted)")" >/dev/null; eval "$2"
  out=$(consume "$(efile "$(ev $b RTD 2 completed "$CMP")")"); rc=$?
  { [ $rc -eq 20 ] && printf '%s' "$out" | grep -q 'state_corrupt' && ! grep -q late_ignored "$builds/$b/events.jsonl" && [ ! -e "$builds/$b/consumed/2" ]; } && ok "(I3) $nm: a completed event is refused (state_corrupt), not acked as late_ignored" || bad "(I3 $nm completed) rc=$rc out=$out"
  out=$(consume "$(efile "$(ev $b RTD 3 heartbeat "$HB")")"); rc=$?
  { [ $rc -eq 20 ] && printf '%s' "$out" | grep -q 'state_corrupt' && ! grep -q late_ignored "$builds/$b/events.jsonl"; } && ok "(I3) $nm: a heartbeat is refused as well" || bad "(I3 $nm heartbeat) rc=$rc out=$out"
}
tdn=0
dmg "empty terminal directory" 'mkdir "$builds/$b/terminal"'
dmg "empty terminal/state.json" 'mkdir "$builds/$b/terminal"; : > "$builds/$b/terminal/state.json"; printf "claimed\n" > "$builds/$b/terminal/callback.state"'
dmg "terminal without callback.state" 'mkdir "$builds/$b/terminal"; printf "{\"kind\":\"completed\",\"seq\":9}\n" > "$builds/$b/terminal/state.json"'

# ---- I4: real fsync(2) on files and directories; a failing fsync refuses ----
oracle_begin shim
fsync_rc() { EC=$EC bash -c 'source "$EC"; "$@"' _ "$@" >/dev/null 2>&1; }
fsync_rc fsync_file "$tmp/fsneedle.b"; r1=$?; fsync_rc fsync_dir "$tmp"; r2=$?; fsync_rc fsync_file /dev/null; r3=$?; fsync_rc fsync_file "$tmp/no-such-file"; r4=$?; fsync_rc fsync_dir "$tmp/no-such-dir"; r5=$?
fsfault fsneedle.a env EC=$EC bash -c 'source "$EC"; fsync_file "$1"' _ "$tmp/fsneedle.a" >/dev/null 2>&1; r6=$?
fsfault_exact "$tmp" env EC=$EC bash -c 'source "$EC"; fsync_dir "$1"' _ "$tmp" >/dev/null 2>&1; r7=$?
{ [ $r1 -eq 0 ] && [ $r2 -eq 0 ] && [ $r3 -ne 0 ] && [ $r4 -ne 0 ] && [ $r5 -ne 0 ] && [ $r6 -ne 0 ] && [ $r7 -ne 0 ]; } && ok "(I4) fsync_file/fsync_dir report the fsync(2) result: ok on a file and a directory, an error on /dev/null (EINVAL), a missing path, and an injected EIO (file and directory)" || bad "(I4) helper rcs: $r1 $r2 $r3 $r4 $r5 $r6 $r7"
oracle_end; oracle_begin strace
# which calls the core makes: the temp file is fsynced BEFORE its rename, the journal file and the directories are fsynced
newbuild SA RSA; sxl=$tmp/fs.SA.log
strace -f -y -qq -e trace=fsync,fdatasync,rename,renameat,renameat2 -o "$sxl" bash "$EC" consume "$builds" "$state" "$(efile "$(ev SA RSA 1 accepted)")" >/dev/null 2>&1
fl=$(grep -n 'fsync(.*/SA/tmp/w\.' "$sxl" | head -1 | cut -d: -f1); rl=$(grep -n 'rename.*/SA/tmp/w\.' "$sxl" | head -1 | cut -d: -f1)
{ [ -n "$fl" ] && [ -n "$rl" ] && [ "$fl" -lt "$rl" ]; } && ok "(I4) the temp file is fsynced (fsync(2)) before its rename onto the consumed mark" || bad "(I4 order) fsync line='$fl' rename line='$rl'"
{ grep -F 'fsync(' "$sxl" | grep -q 'events.jsonl>)' && grep -F 'fsync(' "$sxl" | grep -qF "<$builds/SA>)" && grep -F 'fsync(' "$sxl" | grep -qF "<$builds/SA/consumed>)"; } && ok "(I4) the journal file, the build directory and the consumed directory are fsynced" || bad "(I4) journal/directory fsync missing: $(grep -c 'fsync(' "$sxl") fsync calls"
oracle_end; oracle_begin shim
# a failing fsync refuses at each site, and leaves nothing half-written
newbuild FJ2 RFJ2; cf=$(efile "$(ev FJ2 RFJ2 1 accepted)"); out=$(fsconsume /FJ2/events.jsonl "$cf"); rc=$?
{ [ $rc -eq 20 ] && printf '%s' "$out" | grep -q journal_failed && [ ! -e "$builds/FJ2/consumed/1" ] && [ ! -s "$builds/FJ2/events.jsonl" ]; } && ok "(I4) journal fsync fails (EIO): refused journal_failed, the journal line is rolled back, no consumed mark" || bad "(I4 journal fsync) rc=$rc out=$out size=$(stat -c %s "$builds/FJ2/events.jsonl" 2>/dev/null)"
out=$(consume "$cf"); printf '%s' "$out" | grep -q 'consumed seq=1' && ok "(I4) the same event redelivered after the fsync fault is consumed" || bad "(I4 journal fsync redelivery) $out"
newbuild FJ3 RFJ3; cf=$(efile "$(ev FJ3 RFJ3 1 accepted)"); out=$(fsfault_exact "$builds/FJ3" bash "$EC" consume "$builds" "$state" "$cf" 2>&1); rc=$?
{ [ $rc -eq 20 ] && printf '%s' "$out" | grep -q journal_failed; } && ok "(I4) build-directory fsync fails (EIO): refused journal_failed, never acknowledged" || bad "(I4 dir fsync journal) rc=$rc out=$out"
newbuild FJ4 RFJ4; cf=$(efile "$(ev FJ4 RFJ4 1 accepted)"); out=$(fsfault_exact "$builds/FJ4/consumed" bash "$EC" consume "$builds" "$state" "$cf" 2>&1); rc=$?
{ [ $rc -eq 20 ] && printf '%s' "$out" | grep -q state_write_failed; } && ok "(I4) consumed-directory fsync fails (EIO): refused state_write_failed, never acknowledged" || bad "(I4 dir fsync consumed) rc=$rc out=$out"
newbuild FJ5 RFJ5; consume "$(efile "$(ev FJ5 RFJ5 1 accepted)")" >/dev/null; cf=$(efile "$(ev FJ5 RFJ5 2 completed "$CMP")")
out=$(fsfault /FJ5/terminal.tmp- bash "$EC" consume "$builds" "$state" "$cf" 2>&1); rc=$?
{ [ $rc -eq 20 ] && printf '%s' "$out" | grep -q terminal_claim_failed && [ ! -e "$builds/FJ5/terminal" ] && [ ! -e "$builds/FJ5/consumed/2" ] && nofail_tmp FJ5; } && ok "(I4) terminal record fsync fails (EIO): refused terminal_claim_failed, no terminal, nothing consumed, no leftover" || bad "(I4 terminal fsync) rc=$rc out=$out"
newbuild FJ7 RFJ7; consume "$(efile "$(ev FJ7 RFJ7 1 accepted)")" >/dev/null; cf=$(efile "$(ev FJ7 RFJ7 2 completed "$CMP")")
out=$(env LD_PRELOAD="$tmp/fsfail.so" FSYNC_FAIL=/FJ7/terminal.tmp- FSYNC_FAIL_DIRONLY=1 bash "$EC" consume "$builds" "$state" "$cf" 2>&1); rc=$?
{ [ $rc -eq 20 ] && printf '%s' "$out" | grep -q terminal_claim_failed && [ ! -e "$builds/FJ7/terminal" ] && [ ! -e "$builds/FJ7/consumed/2" ] && nofail_tmp FJ7; } && ok "(I4) prepared-terminal DIRECTORY fsync fails (EIO): refused terminal_claim_failed, no terminal, no leftover" || bad "(I4 terminal dir fsync) rc=$rc out=$out"
b=FJ6; newbuild $b RFJ6; consume "$(efile "$(ev $b RFJ6 1 accepted)")" >/dev/null; consume "$(efile "$(ev $b RFJ6 2 completed "$CMP")")" >/dev/null; cbset $b claimed; rm -rf "${builds:?}/${b:?}/effects" "${builds:?}/${b:?}/effects.log"
out=$(fsfault /FJ6/effects.log bash "$EC" resume-callback "$builds" $b 2>&1); rc=$?
{ [ $rc -eq 21 ] && [ "$(cbs $b)" = failed ] && [ "$(efiles $b)" = 0 ]; } && ok "(I4) effects.log fsync fails (EIO): the callback is failed, never done" || bad "(I4 effects.log fsync) rc=$rc state=$(cbs $b) out=$out"

for sp in 'build_hmac.key.tmp-' 'build_hmac.key.meta.tmp-'; do
  s=$tmp/sf_$RANDOM; out=$(fsfault "$sp" bash "$EC" secret-init "$s" "$root" 2>&1); rc=$?
  { [ $rc -eq 20 ] && printf '%s' "$out" | grep -q secret_create_failed && [ ! -e "$s/build_hmac.key" ] && [ ! -e "$s/build_hmac.key.meta" ] && ! printf '%s' "$out" | grep -q 'secret created'; } && ok "(I4) secret-init: fsync of '$sp' fails (EIO): refused secret_create_failed, no key, no meta" || bad "(I4 secret '$sp') rc=$rc out=$out"
done
s=$tmp/sf_dir; out=$(fsfault_exact "$s" bash "$EC" secret-init "$s" "$root" 2>&1); rc=$?
{ [ $rc -eq 20 ] && printf '%s' "$out" | grep -q secret_create_failed && [ ! -e "$s/build_hmac.key" ] && ! printf '%s' "$out" | grep -q 'secret created'; } && ok "(I4) secret-init: state-directory fsync fails (EIO): refused secret_create_failed, no key" || bad "(I4 secret dir) rc=$rc out=$out"

oracle_end

# ---- I5 / m7: a short journal write is rolled back (and a leftover partial line repaired by the next append); the journal never follows a symlink ----
jvalid() { jq -c . "$1" >/dev/null 2>&1; }       # every line of the journal is one valid JSON object
newbuild JS RJS; consume "$(efile "$(ev JS RJS 1 accepted)")" >/dev/null; sz0=$(stat -c %s "$builds/JS/events.jsonl")
bighost=$(python3 -c 'print("h"*900)')
e=$(mk JS RJS 2 heartbeat '. + {progress_offset:1,stage:0,elapsed_monotonic_ms:1} | .host="'"$bighost"'"'); ef=$(efile "$e")
out=$( ( ulimit -f 1; bash "$EC" consume "$builds" "$state" "$ef" 2>&1 ) ); rc=$?
{ [ $rc -eq 20 ] && printf '%s' "$out" | grep -q journal_failed && [ "$(stat -c %s "$builds/JS/events.jsonl")" = "$sz0" ] && jvalid "$builds/JS/events.jsonl" && [ ! -e "$builds/JS/consumed/2" ]; } && ok "(I5) a short journal write (file size limit): refused journal_failed, the partial line is rolled back, the journal is still one JSON object per line" || bad "(I5 short write) rc=$rc size=$(stat -c %s "$builds/JS/events.jsonl") was=$sz0 out=$out"
out=$(consume "$ef"); rc=$?
{ [ $rc -eq 0 ] && printf '%s' "$out" | grep -q 'consumed seq=2' && jvalid "$builds/JS/events.jsonl" && [ "$(wc -l < "$builds/JS/events.jsonl")" = 2 ]; } && ok "(I5) the redelivery after the short write is consumed and the journal has exactly two valid lines" || bad "(I5 redelivery) rc=$rc out=$out lines=$(wc -l < "$builds/JS/events.jsonl")"
newbuild JR RJR; consume "$(efile "$(ev JR RJR 1 accepted)")" >/dev/null; printf '{"event":"torn' >> "$builds/JR/events.jsonl"
out=$(consume "$(efile "$(ev JR RJR 2 heartbeat "$HB")")"); rc=$?
{ [ $rc -eq 0 ] && jvalid "$builds/JR/events.jsonl" && [ "$(wc -l < "$builds/JR/events.jsonl")" = 2 ] && ! grep -q torn "$builds/JR/events.jsonl"; } && ok "(I5) a torn last line left by a crash is repaired by the next append (journal valid, two lines)" || bad "(I5 repair) rc=$rc out=$out lines=$(wc -l < "$builds/JR/events.jsonl")"
newbuild JY RJY; printf 'KEEP\n' > "$tmp/victim.jr"; ln -s "$tmp/victim.jr" "$builds/JY/events.jsonl"
out=$(consume "$(efile "$(ev JY RJY 1 accepted)")"); rc=$?
{ [ $rc -eq 20 ] && printf '%s' "$out" | grep -q journal_failed && [ "$(cat "$tmp/victim.jr")" = KEEP ] && [ ! -e "$builds/JY/consumed/1" ]; } && ok "(m7) a symlinked events.jsonl is refused (journal_failed), the link target is untouched" || bad "(m7 journal symlink) rc=$rc victim='$(cat "$tmp/victim.jr")' out=$out"

newbuild JZ RJZ; : > "$tmp/victim.jz"; ln -s "$tmp/victim.jz" "$builds/JZ/events.jsonl"
out=$(consume "$(efile "$(ev JZ RJZ 1 accepted)")"); rc=$?
{ [ $rc -eq 20 ] && printf '%s' "$out" | grep -q journal_failed && [ ! -s "$tmp/victim.jz" ]; } && ok "(m7) a symlink to an EMPTY file as events.jsonl is refused too, the target stays empty" || bad "(m7 empty target) rc=$rc size=$(stat -c %s "$tmp/victim.jz") out=$out"

# ---- Y1-Y6 (the reviewer's surviving mutants) and the m12 isolated fixtures ----
# Y1: a progress.json holding a number above 2^53 (or a negative one) is state_corrupt, never compared
newbuild YP RYP; consume "$(efile "$(ev YP RYP 1 accepted)")" >/dev/null; consume "$(efile "$(ev YP RYP 2 heartbeat "$HB")")" >/dev/null
for bad_pj in '{"progress_offset":99999999999999999999,"stage":1}' '{"progress_offset":-5,"stage":1}' '{"progress_offset":10,"stage":123456789012345678901}'; do
  printf '%s\n' "$bad_pj" > "$builds/YP/progress.json"; jl=$(wc -l < "$builds/YP/events.jsonl")
  out=$(consume "$(efile "$(ev YP RYP 3 heartbeat "$HB")")"); rc=$?
  { [ $rc -eq 20 ] && printf '%s' "$out" | grep -q state_corrupt && ! printf '%s' "$out" | grep -q 'expected' && [ ! -e "$builds/YP/consumed/3" ] && [ "$(wc -l < "$builds/YP/events.jsonl")" = "$jl" ]; } && ok "(Y1) progress.json $bad_pj: refused state_corrupt, no shell error, nothing journaled or consumed" || bad "(Y1 $bad_pj) rc=$rc out=$out"
done
# Y4: a build id that names a path is refused by cancel and by resume-callback, before anything is touched
mkdir -p "$tmp/outside/terminal"; jq -n '{build_id:"outside",run_id:"R",variant:"primary"}' > "$tmp/outside/submit.json"; printf '{"callback_id":"cb","effect_key":"k"}\n' > "$tmp/outside/callback.json"
printf '{"kind":"completed","seq":1,"exit_class":"succeeded","claimed_at":"x","digest":""}\n' > "$tmp/outside/terminal/state.json"; printf 'claimed\n' > "$tmp/outside/terminal/callback.state"
for bid in ../outside a/b; do
  out=$(bash "$EC" cancel "$builds" "$bid" 2>&1); rc=$?
  { [ $rc -eq 20 ] && printf '%s' "$out" | grep -q 'event_malformed build_id'; } && ok "(Y4) cancel '$bid' refused (event_malformed build_id)" || bad "(Y4 cancel '$bid') rc=$rc out=$out"
  out=$(bash "$EC" resume-callback "$builds" "$bid" 2>&1); rc=$?
  { [ $rc -eq 20 ] && printf '%s' "$out" | grep -q 'event_malformed build_id'; } && ok "(Y4) resume-callback '$bid' refused (event_malformed build_id)" || bad "(Y4 resume '$bid') rc=$rc out=$out"
done
{ [ "$(cat "$tmp/outside/terminal/callback.state")" = claimed ] && [ ! -e "$tmp/outside/events.jsonl" ] && [ ! -e "$tmp/outside/.lock" ] && [ ! -e "$tmp/outside/effects.log" ]; } && ok "(Y4) nothing outside the builds root was touched" || bad "(Y4) a directory outside the builds root was written"
# Y5: the effect_key cap is exactly 128
b=EKC; newbuild $b REKC; printf '{"callback_id":"cb","effect_key":"%s"}\n' "$(printf 'k%.0s' $(seq 129))" > "$builds/$b/callback.json"
consume "$(efile "$(ev $b REKC 1 accepted)")" >/dev/null; consume "$(efile "$(ev $b REKC 2 completed "$CMP")")" >/dev/null
{ [ "$(cbs $b)" = failed ] && grep -q effect_key_invalid "$builds/$b/terminal/callback.reason" && [ "$(efiles $b)" = 0 ]; } && ok "(Y5) a 129-character effect_key is refused (effect_key_invalid)" || bad "(Y5 129) state=$(cbs $b)"
b=EKD; newbuild $b REKD; printf '{"callback_id":"cb","effect_key":"%s"}\n' "$(printf 'k%.0s' $(seq 128))" > "$builds/$b/callback.json"
consume "$(efile "$(ev $b REKD 1 accepted)")" >/dev/null; consume "$(efile "$(ev $b REKD 2 completed "$CMP")")" >/dev/null
{ [ "$(cbs $b)" = done ] && [ "$(efiles $b)" = 1 ]; } && ok "(Y5) a 128-character effect_key is accepted and applied" || bad "(Y5 128) state=$(cbs $b)"
# Y6: a second completed (other seq, other content) after the terminal is late_ignored, never seq_conflict (which would be redelivered forever)
newbuild YS RYS; consume "$(efile "$(ev YS RYS 1 accepted)")" >/dev/null; consume "$(efile "$(ev YS RYS 2 completed "$CMP")")" >/dev/null
out=$(consume "$(efile "$(ev YS RYS 3 completed "$(jq -c '.exit_class="build_failed"' <<<"$CMP")")")"); rc=$?
{ [ $rc -eq 0 ] && printf '%s' "$out" | grep -q 'late_ignored seq=3' && [ "$(jq -r .exit_class "$builds/YS/terminal/state.json")" = succeeded ] && [ "$(effects YS)" = 1 ]; } && ok "(Y6) a second completed with another seq after the terminal is late_ignored (acked), the verdict unchanged" || bad "(Y6) rc=$rc out=$out"
# m12: one fixture per rule where that rule is the ONLY reason for the refusal (an otherwise valid next heartbeat)
newbuild XM RXM; consume "$(efile "$(ev XM RXM 1 accepted)")" >/dev/null; consume "$(efile "$(ev XM RXM 2 heartbeat "$HB")")" >/dev/null
out=$(consume "$(efile "$(ev XM RXM 3 heartbeat '{"progress_offset":50,"stage":1,"elapsed_monotonic_ms":1,"extra":1}')")"); rc=$?
{ [ $rc -eq 20 ] && printf '%s' "$out" | grep -q 'event_malformed unexpected field extra' && [ ! -e "$builds/XM/consumed/3" ]; } && ok "(m12) an otherwise valid next heartbeat with one extra field is refused for that reason only" || bad "(m12 extra) rc=$rc out=$out"
out=$(consume "$(efile "$(mk XM RXM 3 heartbeat '. + {progress_offset:100.5,stage:1,elapsed_monotonic_ms:1}')")"); rc=$?
{ [ $rc -eq 20 ] && printf '%s' "$out" | grep -q 'event_malformed heartbeat fields' && [ ! -e "$builds/XM/consumed/3" ]; } && ok "(m12) an otherwise valid next heartbeat with progress_offset 100.5 is refused for that reason only" || bad "(m12 float) rc=$rc out=$out"
dupev=$(ev XM RXM 3 heartbeat '{"progress_offset":50,"stage":1,"elapsed_monotonic_ms":1}' | sed 's/^{/{"host":"anton",/')
out=$(consume "$(efile "$dupev")"); rc=$?
{ [ $rc -eq 20 ] && printf '%s' "$out" | grep -q 'event_malformed duplicate key host' && [ ! -e "$builds/XM/consumed/3" ]; } && ok "(m12) a correctly signed event with a duplicated key is refused for that reason only" || bad "(m12 dup key) rc=$rc out=$out"
out=$(consume "$(efile "$(ev XM RXM 3 heartbeat '{"progress_offset":50,"stage":1,"elapsed_monotonic_ms":1}')")"); rc=$?
{ [ $rc -eq 0 ] && printf '%s' "$out" | grep -q 'consumed seq=3'; } && ok "(m12) control: the valid heartbeat 3 is consumed after the refused variants" || bad "(m12 control) rc=$rc out=$out"

# ---- m4: secret-init takes an absolute path only (an option-like or relative argument is refused, nothing is created) ----
mkdir -p "$tmp/scwd"
for arg in --version rel/state; do
  out=$(cd "$tmp/scwd" && bash "$EC" secret-init "$arg" "$root" 2>&1); rc=$?
  { [ $rc -eq 20 ] && printf '%s' "$out" | grep -q secret_dir_not_absolute && [ -z "$(ls -A "$tmp/scwd")" ]; } && ok "(m4) secret-init '$arg' is refused (secret_dir_not_absolute), nothing created in the working directory" || bad "(m4 '$arg') rc=$rc out=$out ls=$(ls "$tmp/scwd")"
done
# ---- m8: the bev_crypto CLI needs the checkout argument (the location check cannot be skipped by omitting it) ----
python3 "$ECDIR/lib/bev_crypto.py" derive "$tmp/co/state" BK RK >/dev/null 2>&1; r1=$?
out=$(python3 "$ECDIR/lib/bev_crypto.py" derive "$tmp/co/state" BK RK "$tmp/co" 2>&1); r2=$?
{ [ $r1 -eq 2 ] && [ $r2 -eq 20 ] && printf '%s' "$out" | grep -q state_dir_inside_checkout; } && ok "(m8) bev_crypto derive without the checkout argument is a usage error (2); with it, a state dir inside the checkout is refused (20)" || bad "(m8) rc without checkout=$r1 with=$r2 out=$out"

# =====================================================================================================================
# Review round 4 (WF3-REVIEW event-core): I1, I2 (W1-W5), m1 m2 m3 m4 m8 m10
# =====================================================================================================================
SHA_A=$(printf a | sha256sum | cut -d' ' -f1); SHA_B=$(printf b | sha256sum | cut -d' ' -f1)
cmpx() { jq -c "$1" <<<"$CMP"; }             # the completed fields with a jq edit applied
# ---- I1: the fsync of the effects directory is checked: an EIO there ends failed, never done ----
oracle_begin shim
b=R4E; newbuild $b RR4E; consume "$(efile "$(ev $b RR4E 1 accepted)")" >/dev/null; consume "$(efile "$(ev $b RR4E 2 completed "$CMP")")" >/dev/null
cbset $b claimed; rm -rf "${builds:?}/${b:?}/effects" "${builds:?}/${b:?}/effects.log"
out=$(fsfault_exact "$builds/$b/effects" bash "$EC" resume-callback "$builds" $b 2>&1); rc=$?
{ [ $rc -eq 21 ] && [ "$(cbs $b)" = failed ] && grep -q '^effect_not_durable$' "$builds/$b/terminal/callback.reason"; } && ok "(R4-I1) fsync of the effects directory fails (EIO): the callback is failed (effect_not_durable), never done" || bad "(R4-I1 effects dir) rc=$rc state=$(cbs $b) reason=$(cat "$builds/$b/terminal/callback.reason" 2>/dev/null) out=$out"
out=$(bash "$EC" resume-callback "$builds" $b 2>&1); rc=$?
{ [ $rc -eq 0 ] && [ "$(cbs $b)" = done ] && [ "$(efiles $b)" = 1 ] && [ "$(effects $b)" = 1 ]; } && ok "(R4-I1) resume-callback after the fault clears completes the callback, one effect, one audit line" || bad "(R4-I1 resume) rc=$rc state=$(cbs $b) efiles=$(efiles $b)"
b=R4F; newbuild $b RR4F; consume "$(efile "$(ev $b RR4F 1 accepted)")" >/dev/null; consume "$(efile "$(ev $b RR4F 2 completed "$CMP")")" >/dev/null
cbset $b claimed; rm -rf "${builds:?}/${b:?}/effects" "${builds:?}/${b:?}/effects.log"
out=$(fsfault_exact "$builds/$b" bash "$EC" resume-callback "$builds" $b 2>&1); rc=$?
{ [ $rc -eq 21 ] && [ "$(cbs $b)" = failed ] && grep -q '^effects_dir_not_durable$' "$builds/$b/terminal/callback.reason" && [ "$(efiles $b)" = 0 ]; } && ok "(R4-I1) the first creation of effects/ is followed by a CHECKED fsync of the build directory (EIO: failed effects_dir_not_durable, no effect run)" || bad "(R4-I1 build dir) rc=$rc state=$(cbs $b) reason=$(cat "$builds/$b/terminal/callback.reason" 2>/dev/null) efiles=$(efiles $b)"
oracle_end

# ---- I2: W1-W5, the reviewer's surviving mutants: one fixture each, the mutated rule being the ONLY reason for the refusal ----
newbuild XW RXW; consume "$(efile "$(ev XW RXW 1 accepted)")" >/dev/null
for pr in '-1' '1.5' '"x"'; do   # W1: peak_rss_bytes
  e=$(mk XW RXW 2 completed ". + $(cmpx ". + {peak_rss_bytes:$pr}")"); out=$(consume "$(efile "$e")"); rc=$?
  { [ $rc -eq 20 ] && printf '%s' "$out" | grep -q 'event_malformed peak_rss_bytes' && [ ! -e "$builds/XW/terminal" ] && [ ! -e "$builds/XW/consumed/2" ]; } && ok "(W1) completed with peak_rss_bytes $pr is refused by the core (event_malformed peak_rss_bytes), no terminal" || bad "(W1 $pr) rc=$rc out=$out"
done
e=$(mk XW RXW 2 completed ". + $(cmpx ". + {peak_rss_bytes:true}")"); out=$(consume "$(efile "$e")"); rc=$?
{ [ $rc -eq 20 ] && printf '%s' "$out" | grep -q 'event_malformed peak_rss_bytes'; } && ok "(W1) peak_rss_bytes true (a boolean) is refused" || bad "(W1 true) rc=$rc out=$out"
newbuild XV RXV; consume "$(efile "$(ev XV RXV 1 accepted)")" >/dev/null
for hf in 'no elapsed_monotonic_ms|{"progress_offset":10,"stage":1}' 'negative elapsed_monotonic_ms|{"progress_offset":10,"stage":1,"elapsed_monotonic_ms":-5}' 'no stage|{"progress_offset":10,"elapsed_monotonic_ms":1}'; do   # W2
  IFS='|' read -r nm hx <<<"$hf"; out=$(consume "$(efile "$(ev XV RXV 2 heartbeat "$hx")")"); rc=$?
  { [ $rc -eq 20 ] && printf '%s' "$out" | grep -q 'event_malformed heartbeat fields' && [ ! -e "$builds/XV/consumed/2" ]; } && ok "(W2) a heartbeat with $nm is refused (event_malformed heartbeat fields)" || bad "(W2 $nm) rc=$rc out=$out"
done
validate "$(ev XV RXV 2 heartbeat '{"progress_offset":10,"stage":1,"elapsed_monotonic_ms":-5}')" && bad "(W2) the schema accepts a negative elapsed_monotonic_ms" || ok "(W2) the schema also rejects a negative elapsed_monotonic_ms"
out=$(consume "$(efile "$(ev XV RXV 2 heartbeat "$HB")")"); printf '%s' "$out" | grep -q 'consumed seq=2' && ok "(W2) control: the valid heartbeat 2 is consumed after the refused variants" || bad "(W2 control) $out"
out=$(bash "$EC" cancel "$builds" R4NEVERISSUED 2>&1); rc=$?   # W3: a cancel for a build that was never issued
{ [ $rc -eq 20 ] && printf '%s' "$out" | grep -q 'event_unknown_build' && [ ! -e "$builds/R4NEVERISSUED" ]; } && ok "(W3) cancel of a build that was never issued is refused (event_unknown_build), no directory and no terminal created" || bad "(W3) rc=$rc out=$out ls=$(ls "$builds/R4NEVERISSUED" 2>&1 | head -3 | tr '\n' ' ')"
newbuild W4B RW4B; out=$(bash "$EC" resume-callback "$builds" W4B 2>&1); rc=$?   # W4: resume-callback on a build with no terminal
{ [ $rc -eq 20 ] && printf '%s' "$out" | grep -q 'REFUSED reason=no_terminal' && ! printf '%s' "$out" | grep -q 'callback_state'; } && ok "(W4) resume-callback on a build without a terminal is refused (no_terminal, exit 20), not a callback_state report" || bad "(W4) rc=$rc out=$out"
newbuild XI RXI; consume "$(efile "$(ev XI RXI 1 accepted)")" >/dev/null   # W5: image_digest minLength 1
e=$(mk XI RXI 2 completed ". + $(cmpx '.image_digest=""')"); out=$(consume "$(efile "$e")"); rc=$?
{ [ $rc -eq 20 ] && printf '%s' "$out" | grep -q 'event_malformed completed fields' && [ ! -e "$builds/XI/terminal" ]; } && ok "(W5) a completed with an empty image_digest is refused (completed fields), no terminal" || bad "(W5) rc=$rc out=$out"
validate "$e" && bad "(W5) the schema accepts an empty image_digest" || ok "(W5) the schema also rejects an empty image_digest (minLength 1)"

# ---- m1: a sha field that is a JSON number is refused by the core (the schema says type string) ----
newbuild XN RXN; consume "$(efile "$(ev XN RXN 1 accepted)")" >/dev/null
num64=$(printf '1%.0s' $(seq 64))
for fld in artifact_manifest_sha256 remote_log_sha256; do
  if [ $fld = artifact_manifest_sha256 ]; then am=$num64; lg=\"$SHA_B\"; else am=\"$SHA_A\"; lg=$num64; fi
  e=$(rawev XN RXN "$(BASEJ RXN XN 2 completed ",\"exit_class\":\"succeeded\",\"artifact_manifest_sha256\":$am,\"image_digest\":\"sha256:abc\",\"remote_log_sha256\":$lg")")
  out=$(consume "$(efile "$e")"); rc=$?
  { [ $rc -eq 20 ] && printf '%s' "$out" | grep -q 'event_malformed completed fields' && [ ! -e "$builds/XN/terminal" ] && ! grep -q "$num64" "$builds/XN/events.jsonl" 2>/dev/null; } && ok "(R4-m1) $fld as a 64-digit JSON NUMBER is refused (completed fields), not journaled" || bad "(R4-m1 $fld) rc=$rc out=$out"
  validate "$e" && bad "(R4-m1) the schema accepts $fld as a number" || ok "(R4-m1) the schema also rejects $fld as a number"
done

# ---- m2: atomic_write never renames INTO a directory that sits where a state file belongs ----
newbuild PD RPD; consume "$(efile "$(ev PD RPD 1 accepted)")" >/dev/null; consume "$(efile "$(ev PD RPD 2 heartbeat '{"progress_offset":500,"stage":5,"elapsed_monotonic_ms":1}')")" >/dev/null
rm -f "$builds/PD/progress.json"; mkdir "$builds/PD/progress.json"
out=$(consume "$(efile "$(ev PD RPD 3 heartbeat '{"progress_offset":1,"stage":0,"elapsed_monotonic_ms":2}')")"); rc=$?
{ [ $rc -eq 20 ] && printf '%s' "$out" | grep -q 'state_corrupt' && [ ! -e "$builds/PD/consumed/3" ] && [ -z "$(ls -A "$builds/PD/progress.json")" ]; } && ok "(R4-m2) a progress.json that is a directory is refused (state_corrupt), nothing consumed, nothing written inside it" || bad "(R4-m2 progress dir) rc=$rc out=$out inside=$(ls -A "$builds/PD/progress.json" | head -2 | tr '\n' ' ')"
newbuild CD RCD; consume "$(efile "$(ev CD RCD 1 accepted)")" >/dev/null; consume "$(efile "$(ev CD RCD 2 completed "$CMP")")" >/dev/null
rm -f "$builds/CD/terminal/callback.state"; mkdir "$builds/CD/terminal/callback.state"
out=$(bash "$EC" resume-callback "$builds" CD 2>&1); rc=$?
{ [ $rc -eq 21 ] && [ -z "$(ls -A "$builds/CD/terminal/callback.state")" ]; } && ok "(R4-m2) a callback.state that is a directory: resume-callback exits 21 (not 0), nothing written inside the directory" || bad "(R4-m2 callback.state dir) rc=$rc out=$out inside=$(ls -A "$builds/CD/terminal/callback.state" | head -2 | tr '\n' ' ')"

# ---- m3: a won claim followed by a refusal leaves no orphaned claimed callback: the redelivery (a DUP) re-runs a claimed or running callback ----
newbuild OC ROC; consume "$(efile "$(ev OC ROC 1 accepted)")" >/dev/null; cc=$(efile "$(ev OC ROC 2 completed "$CMP")"); consume "$cc" >/dev/null
cbset OC claimed; rm -rf "${builds:?}/OC/effects" "${builds:?}/OC/effects.log"
out=$(consume "$cc"); rc=$?
{ [ $rc -eq 0 ] && printf '%s' "$out" | grep -q '^DUP' && [ "$(cbs OC)" = done ] && [ "$(efiles OC)" = 1 ] && [ "$(effects OC)" = 1 ]; } && ok "(R4-m3) the DUP redelivery of a completed event whose callback is claimed runs the callback once (done, one effect)" || bad "(R4-m3 dup claimed) rc=$rc state=$(cbs OC) efiles=$(efiles OC) out=$out"
newbuild OF ROF; consume "$(efile "$(ev OF ROF 1 accepted)")" >/dev/null; cf2=$(efile "$(ev OF ROF 2 completed "$CMP")"); consume "$cf2" >/dev/null
cbset OF failed; rm -rf "${builds:?}/OF/effects" "${builds:?}/OF/effects.log"
out=$(consume "$cf2"); rc=$?
{ [ $rc -eq 0 ] && printf '%s' "$out" | grep -q '^DUP' && [ "$(cbs OF)" = failed ] && [ "$(efiles OF)" = 0 ]; } && ok "(R4-m3) a DUP does not silently retry a FAILED callback (only resume-callback does; the retry decision is owed, m6)" || bad "(R4-m3 dup failed) rc=$rc state=$(cbs OF) efiles=$(efiles OF)"
oracle_begin shim
newbuild OS ROS; consume "$(efile "$(ev OS ROS 1 accepted)")" >/dev/null; cs=$(efile "$(ev OS ROS 2 completed "$CMP")")
out=$(fsconsume /OS/tmp/w. "$cs"); rc=$?
{ [ $rc -eq 20 ] && printf '%s' "$out" | grep -q state_write_failed && [ -d "$builds/OS/terminal" ] && [ "$(cbs OS)" = claimed ] && [ "$(efiles OS)" = 0 ]; } && ok "(R4-m3) setup: the consumed-mark write fails (EIO) after a won claim: refused, terminal exists, callback claimed, no effect" || bad "(R4-m3 P9 setup) rc=$rc state=$(cbs OS) out=$out"
out=$(consume "$cs"); rc=$?
{ [ $rc -eq 0 ] && printf '%s' "$out" | grep -q '^DUP' && [ "$(cbs OS)" = done ] && [ "$(efiles OS)" = 1 ]; } && ok "(R4-m3) the redelivery after that refusal is a DUP and completes the callback (no orphaned claimed callback)" || bad "(R4-m3 P9 redelivery) rc=$rc state=$(cbs OS) efiles=$(efiles OS) out=$out"
oracle_end

# ---- m4: pre-authentication text never reaches the refusal output with control characters ----
newbuild BM4 RM4
mal() { printf '{"schema":"build-event/1","run_id":"RM4","build_id":"BM4","variant":"primary","seq":1,"kind":"accepted","host":"h","sent_at":"s","hmac":"%064d"%s}' 0 "$1"; }
out=$(consume "$(efile "$(mal ',"\u001b[31m\r consumed seq=1":1')")"); rc=$?
{ [ $rc -eq 20 ] && printf '%s' "$out" | grep -q 'event_malformed unexpected field' && ! printf '%s' "$out" | grep -q $'\033' && ! printf '%s' "$out" | grep -q $'\r' && [ "$(printf '%s\n' "$out" | wc -l)" = 1 ]; } && ok "(R4-m4) an unexpected field name carrying ESC and CR is refused with the control characters escaped (one line, no raw ESC or CR)" || bad "(R4-m4 unexpected field) rc=$rc out=$(printf '%s' "$out" | od -c | head -3)"
out=$(consume "$(efile "$(mal ',"\u001b[31mK\r":1,"\u001b[31mK\r":2')")"); rc=$?
{ [ $rc -eq 20 ] && printf '%s' "$out" | grep -q 'event_malformed duplicate key' && ! printf '%s' "$out" | grep -q $'\033' && ! printf '%s' "$out" | grep -q $'\r'; } && ok "(R4-m4) a duplicate key name carrying ESC and CR is refused with the control characters escaped" || bad "(R4-m4 duplicate key) rc=$rc out=$(printf '%s' "$out" | od -c | head -3)"
out=$(consume "$(efile "$(mal ',"zz":1')")"); printf '%s' "$out" | grep -q 'event_malformed unexpected field zz' && ok "(R4-m4) control: a plain unexpected field name is reported as before" || bad "(R4-m4 control) $out"

# ---- m8: an unreadable secret file is a secret problem (secret_unusable key_unreadable), never an event defect ----
if [ "$(id -u)" = 0 ]; then say "SKIP (R4-m8) unreadable key: running as root, which bypasses mode 000 (oracle condition absent, reason: uid 0)"; skipn=$((skipn+1))
else
  newbuild BK RK; s=$tmp/sm_unread; mkdir_state "$s" "$secret_hex"; chmod 000 "$s/build_hmac.key"
  sbad "unreadable (mode 000) key file" "$s" key_unreadable; chmod 600 "$s/build_hmac.key"
fi

# =====================================================================================================================
# Review round 5 (WF5-REVIEW event-core): N1 N2 N3
# =====================================================================================================================
# ---- N1: the retry after effects_dir_not_durable needs a SUCCESSFUL fsync of the build directory before done ----
oracle_begin shim
b=R5N1; newbuild $b RR5N1; consume "$(efile "$(ev $b RR5N1 1 accepted)")" >/dev/null; consume "$(efile "$(ev $b RR5N1 2 completed "$CMP")")" >/dev/null
cbset $b claimed; rm -rf "${builds:?}/${b:?}/effects" "${builds:?}/${b:?}/effects.log"
out=$(fsfault_exact "$builds/$b" bash "$EC" resume-callback "$builds" $b 2>&1); rc=$?
{ [ $rc -eq 21 ] && [ "$(cbs $b)" = failed ] && grep -q '^effects_dir_not_durable$' "$builds/$b/terminal/callback.reason" && [ -d "$builds/$b/effects" ]; } && ok "(R5-N1) setup: the first attempt fails effects_dir_not_durable and leaves effects/ behind" || bad "(R5-N1 setup) rc=$rc state=$(cbs $b) out=$out"
out=$(fsfault_exact "$builds/$b" bash "$EC" resume-callback "$builds" $b 2>&1); rc=$?
{ [ $rc -eq 21 ] && [ "$(cbs $b)" = failed ] && grep -q '^effects_dir_not_durable$' "$builds/$b/terminal/callback.reason" && [ "$(efiles $b)" = 0 ]; } && ok "(R5-N1) the retry with the build-directory fsync still failing stays failed (effects_dir_not_durable), never done, no effect run" || bad "(R5-N1 retry under fault) rc=$rc state=$(cbs $b) efiles=$(efiles $b) out=$out"
out=$(bash "$EC" resume-callback "$builds" $b 2>&1); rc=$?
{ [ $rc -eq 0 ] && [ "$(cbs $b)" = done ] && [ "$(efiles $b)" = 1 ] && [ "$(effects $b)" = 1 ]; } && ok "(R5-N1) control: the retry without the fault completes (done, one effect, one audit line)" || bad "(R5-N1 control) rc=$rc state=$(cbs $b) efiles=$(efiles $b)"
oracle_end

# ---- N2: a FIFO (or any non-regular file) at progress.json is state_corrupt, never read (jq would block holding the per-build lock) ----
newbuild N2F RN2F; consume "$(efile "$(ev N2F RN2F 1 accepted)")" >/dev/null; consume "$(efile "$(ev N2F RN2F 2 heartbeat "$HB")")" >/dev/null
rm -f "$builds/N2F/progress.json"; mkfifo "$builds/N2F/progress.json"
n2e=$(efile "$(ev N2F RN2F 3 heartbeat '{"progress_offset":20,"stage":1,"elapsed_monotonic_ms":9}')")
out=$(timeout 10 bash "$EC" consume "$builds" "$state" "$n2e" 2>&1); rc=$?
{ [ $rc -eq 20 ] && printf '%s' "$out" | grep -q 'state_corrupt' && [ ! -e "$builds/N2F/consumed/3" ]; } && ok "(R5-N2) a FIFO at progress.json is refused (state_corrupt) within the bound, nothing consumed" || bad "(R5-N2 fifo) rc=$rc (124 = blocked) out=$out"
rm -f "$builds/N2F/progress.json"; printf '{"progress_offset":10,"stage":1}\n' > "$builds/N2F/progress.json"
out=$(timeout 10 bash "$EC" consume "$builds" "$state" "$n2e" 2>&1); rc=$?
{ [ $rc -eq 0 ] && printf '%s' "$out" | grep -q 'consumed seq=3' && [ "$(jq -c . "$builds/N2F/progress.json")" = '{"progress_offset":20,"stage":1}' ]; } && ok "(R5-N2) control: with a regular progress.json the same event is consumed and the lock was released by the refusal" || bad "(R5-N2 control) rc=$rc out=$out"
newbuild N2L RN2L; consume "$(efile "$(ev N2L RN2L 1 accepted)")" >/dev/null; consume "$(efile "$(ev N2L RN2L 2 heartbeat "$HB")")" >/dev/null
mv "$builds/N2L/progress.json" "$tmp/n2l.real"; ln -s "$tmp/n2l.real" "$builds/N2L/progress.json"
out=$(timeout 10 bash "$EC" consume "$builds" "$state" "$(efile "$(ev N2L RN2L 3 heartbeat "$HB")")" 2>&1); rc=$?
{ [ $rc -eq 20 ] && printf '%s' "$out" | grep -q 'state_corrupt' && [ ! -e "$builds/N2L/consumed/3" ]; } && ok "(R5-N2) a symlink at progress.json is refused (state_corrupt): only a regular, non-symlink file is read" || bad "(R5-N2 symlink) rc=$rc out=$out"

# ---- N3: a DUP that waited on .cblock does not retry a callback the lock holder marked failed ----
newbuild N3 RN3; consume "$(efile "$(ev N3 RN3 1 accepted)")" >/dev/null; n3c=$(efile "$(ev N3 RN3 2 completed "$CMP")"); consume "$n3c" >/dev/null
rm -rf "${builds:?}/N3/effects" "${builds:?}/N3/effects.log"; cbset N3 running
( exec 7>>"$builds/N3/.cblock"; flock 7; : > "$tmp/n3.locked"; n=0; while [ ! -e "$tmp/n3.go" ] && [ $n -lt 200 ]; do sleep 0.05; n=$((n+1)); done; printf 'failed\n' > "$builds/N3/terminal/callback.state" ) & n3h=$!
waitfor "$tmp/n3.locked"
( bash "$EC" consume "$builds" "$state" "$n3c" > "$tmp/n3.out" 2>&1 ) & n3p=$!
sleep 0.7; : > "$tmp/n3.go"; wait $n3h; wait $n3p
{ [ "$(cbs N3)" = failed ] && [ "$(efiles N3)" = 0 ] && [ ! -e "$builds/N3/effects.log" ] && grep -q '^DUP' "$tmp/n3.out"; } && ok "(R5-N3) a DUP that waited on .cblock re-checks under the lock: a callback the holder marked failed is NOT retried (no effect, state failed)" || bad "(R5-N3) state=$(cbs N3) efiles=$(efiles N3) out=$(cat "$tmp/n3.out")"
out=$(bash "$EC" resume-callback "$builds" N3 2>&1); rc=$?
{ [ $rc -eq 0 ] && [ "$(cbs N3)" = done ] && [ "$(efiles N3)" = 1 ]; } && ok "(R5-N3) control: resume-callback (the documented retry path) still retries the failed callback" || bad "(R5-N3 control) rc=$rc state=$(cbs N3)"

# =====================================================================================================================
# Review round 6 (WF6-REVIEW provenance-eventcore): EC-1 EC-2 EC-3 EC-4
# =====================================================================================================================
# unfifo FILE...: release anything blocked on a FIFO (open it read-write once) and remove it; a bounded run that hung must not leave a reader behind
unfifo() { local f; for f in "$@"; do [ -p "$f" ] && { : <>"$f"; command rm -f -- "$f"; }; done; return 0; }
# bounded CMD...: run with a 10 s bound and stdin from /dev/null, output to a FILE (never a pipe): an orphan blocked on a FIFO must not hold a command substitution open. Sets brc and bout
bounded() { timeout 10 "$@" >"$tmp/bounded.out" 2>&1 </dev/null; brc=$?; bout=$(cat "$tmp/bounded.out"); }
lockfree() { ( flock -n 9 ) 9>>"$1"; }   # the lock file can be taken: nothing is blocked holding it

# ---- EC-1: the checked fsyncs of effects/ and of effects.log run on EVERY attempt before done (the N1 principle applied to the other two) ----
oracle_begin shim
b=R6E1A; newbuild $b RR6E1A; consume "$(efile "$(ev $b RR6E1A 1 accepted)")" >/dev/null; consume "$(efile "$(ev $b RR6E1A 2 completed "$CMP")")" >/dev/null
cbset $b claimed; rm -rf "${builds:?}/${b:?}/effects" "${builds:?}/${b:?}/effects.log"
out=$(fsfault_exact "$builds/$b/effects" bash "$EC" resume-callback "$builds" $b 2>&1); rc=$?
{ [ $rc -eq 21 ] && [ "$(cbs $b)" = failed ] && grep -q '^effect_not_durable$' "$builds/$b/terminal/callback.reason" && [ "$(efiles $b)" = 1 ]; } && ok "(R6-EC1) setup: an EIO on the effects/ fsync fails the first attempt (effect_not_durable) and leaves the effect file behind" || bad "(R6-EC1 setup effects) rc=$rc state=$(cbs $b) efiles=$(efiles $b) out=$out"
out=$(fsfault_exact "$builds/$b/effects" bash "$EC" resume-callback "$builds" $b 2>&1); rc=$?
{ [ $rc -eq 21 ] && [ "$(cbs $b)" = failed ] && grep -q '^effect_not_durable$' "$builds/$b/terminal/callback.reason"; } && ok "(R6-EC1) the retry with the effects/ fsync STILL failing stays failed (effect_not_durable), never done" || bad "(R6-EC1 retry effects) rc=$rc state=$(cbs $b) out=$out"
out=$(bash "$EC" resume-callback "$builds" $b 2>&1); rc=$?
{ [ $rc -eq 0 ] && [ "$(cbs $b)" = done ] && [ "$(efiles $b)" = 1 ] && [ "$(effects $b)" = 1 ]; } && ok "(R6-EC1) control: the retry without the fault completes (done, one effect, one audit line)" || bad "(R6-EC1 control effects) rc=$rc state=$(cbs $b) efiles=$(efiles $b) effects=$(effects $b)"
b=R6E1B; newbuild $b RR6E1B; consume "$(efile "$(ev $b RR6E1B 1 accepted)")" >/dev/null; consume "$(efile "$(ev $b RR6E1B 2 completed "$CMP")")" >/dev/null
cbset $b claimed; rm -rf "${builds:?}/${b:?}/effects" "${builds:?}/${b:?}/effects.log"
out=$(fsfault_exact "$builds/$b/effects.log" bash "$EC" resume-callback "$builds" $b 2>&1); rc=$?
{ [ $rc -eq 21 ] && [ "$(cbs $b)" = failed ] && grep -q '^effects_log_write_failed$' "$builds/$b/terminal/callback.reason" && [ "$(efiles $b)" = 0 ] && [ "$(effects $b)" = 1 ]; } && ok "(R6-EC1) setup: an EIO on the effects.log fsync fails the first attempt (effects_log_write_failed) with the audit line written and no effect" || bad "(R6-EC1 setup log) rc=$rc state=$(cbs $b) efiles=$(efiles $b) effects=$(effects $b) out=$out"
out=$(fsfault_exact "$builds/$b/effects.log" bash "$EC" resume-callback "$builds" $b 2>&1); rc=$?
{ [ $rc -eq 21 ] && [ "$(cbs $b)" = failed ] && grep -q '^effects_log_write_failed$' "$builds/$b/terminal/callback.reason" && [ "$(efiles $b)" = 0 ]; } && ok "(R6-EC1) the retry with the effects.log fsync STILL failing stays failed and runs no effect: the audit line already present is not taken as durable" || bad "(R6-EC1 retry log) rc=$rc state=$(cbs $b) efiles=$(efiles $b) out=$out"
out=$(bash "$EC" resume-callback "$builds" $b 2>&1); rc=$?
{ [ $rc -eq 0 ] && [ "$(cbs $b)" = done ] && [ "$(efiles $b)" = 1 ] && [ "$(effects $b)" = 1 ]; } && ok "(R6-EC1) control: the retry without the fault completes (done, one effect, still one audit line)" || bad "(R6-EC1 control log) rc=$rc state=$(cbs $b) efiles=$(efiles $b) effects=$(effects $b)"
oracle_end

# ---- EC-2: a FIFO at any file read or written under a lock is refused, never blocks (a blocked consume holds the per-build lock) ----
b=R6E2A; newbuild $b RR6E2A; consume "$(efile "$(ev $b RR6E2A 1 accepted)")" >/dev/null; mkfifo "$builds/$b/consumed/2"
e2a=$(efile "$(ev $b RR6E2A 2 heartbeat "$HB")")
bounded bash "$EC" consume "$builds" "$state" "$e2a"; out=$bout; rc=$brc
{ [ $rc -eq 20 ] && printf '%s' "$out" | grep -q 'state_corrupt' && lockfree "$builds/$b/.lock"; } && ok "(R6-EC2) a FIFO at consumed/<seq> is refused (state_corrupt) within the bound and the per-build lock is free" || bad "(R6-EC2 consumed fifo) rc=$rc (124 = blocked) lockfree=$(lockfree "$builds/$b/.lock" && echo y || echo n) out=$out"
unfifo "$builds/$b/consumed/2"
bounded bash "$EC" consume "$builds" "$state" "$e2a"; out=$bout; rc=$brc
{ [ $rc -eq 0 ] && printf '%s' "$out" | grep -q 'consumed seq=2'; } && ok "(R6-EC2) control: with the FIFO gone the same event is consumed" || bad "(R6-EC2 consumed control) rc=$rc out=$out"
b=R6E2B; newbuild $b RR6E2B; mkfifo "$builds/$b/events.jsonl"
e2b=$(efile "$(ev $b RR6E2B 1 accepted)")
bounded bash "$EC" consume "$builds" "$state" "$e2b"; out=$bout; rc=$brc
{ [ $rc -eq 20 ] && printf '%s' "$out" | grep -q 'journal_failed' && [ ! -e "$builds/$b/consumed/1" ] && lockfree "$builds/$b/.lock"; } && ok "(R6-EC2) a FIFO at events.jsonl (no reader) is refused (journal_failed) within the bound, nothing consumed, lock free" || bad "(R6-EC2 journal fifo) rc=$rc (124 = blocked) out=$out"
exec 7<>"$builds/$b/events.jsonl"
bounded bash "$EC" consume "$builds" "$state" "$e2b"; out=$bout; rc=$brc
leak=""; read -r -t 1 -u 7 leak 2>/dev/null
{ [ $rc -eq 20 ] && printf '%s' "$out" | grep -q 'journal_failed' && [ ! -e "$builds/$b/consumed/1" ] && [ -z "$leak" ]; } && ok "(R6-EC2) a FIFO at events.jsonl WITH a live reader is refused too, and not one byte of the event is written into it: only a regular file is appended to" || bad "(R6-EC2 journal fifo reader) rc=$rc leaked='${leak:0:60}' out=$out"
exec 7>&-; unfifo "$builds/$b/events.jsonl"
bounded bash "$EC" consume "$builds" "$state" "$e2b"; out=$bout; rc=$brc
{ [ $rc -eq 0 ] && printf '%s' "$out" | grep -q 'consumed seq=1' && [ -f "$builds/$b/events.jsonl" ]; } && ok "(R6-EC2) control: with the FIFO gone the event is journaled and consumed" || bad "(R6-EC2 journal control) rc=$rc out=$out"
b=R6E2C; newbuild $b RR6E2C; consume "$(efile "$(ev $b RR6E2C 1 accepted)")" >/dev/null; consume "$(efile "$(ev $b RR6E2C 2 completed "$CMP")")" >/dev/null
cbset $b claimed; rm -rf "${builds:?}/${b:?}/effects" "${builds:?}/${b:?}/effects.log"; mkfifo "$builds/$b/effects.log"
bounded bash "$EC" resume-callback "$builds" $b; out=$bout; rc=$brc
{ [ $rc -eq 21 ] && [ "$(cbs $b)" = failed ] && grep -q '^effects_log_not_regular$' "$builds/$b/terminal/callback.reason" && lockfree "$builds/$b/.cblock"; } && ok "(R6-EC2) a FIFO at effects.log fails the callback (effects_log_not_regular) within the bound and frees the callback lock" || bad "(R6-EC2 effects.log fifo) rc=$rc (124 = blocked) state=$(cbs $b) out=$out"
unfifo "$builds/$b/effects.log"
bounded bash "$EC" resume-callback "$builds" $b; out=$bout; rc=$brc
{ [ $rc -eq 0 ] && [ "$(cbs $b)" = done ] && [ "$(efiles $b)" = 1 ] && [ "$(effects $b)" = 1 ]; } && ok "(R6-EC2) control: with the FIFO gone resume-callback completes (one effect, one audit line)" || bad "(R6-EC2 effects.log control) rc=$rc state=$(cbs $b) out=$out"
b=R6E2D; newbuild $b RR6E2D; consume "$(efile "$(ev $b RR6E2D 1 accepted)")" >/dev/null; e2d=$(efile "$(ev $b RR6E2D 2 completed "$CMP")"); consume "$e2d" >/dev/null
command rm -f -- "$builds/$b/terminal/callback.state"; mkfifo "$builds/$b/terminal/callback.state"
bounded bash "$EC" consume "$builds" "$state" "$e2d"; out=$bout; rc=$brc
{ [ $rc -eq 0 ] && printf '%s' "$out" | grep -q '^DUP' && lockfree "$builds/$b/.cblock"; } && ok "(R6-EC2) a FIFO at terminal/callback.state: the redelivered event is acknowledged (DUP) within the bound, not blocked in cb_redo" || bad "(R6-EC2 callback.state fifo) rc=$rc (124 = blocked) out=$out"
bounded bash "$EC" resume-callback "$builds" $b; out=$bout; rc=$brc
{ [ $rc -eq 0 ] && [ -f "$builds/$b/terminal/callback.state" ] && [ ! -p "$builds/$b/terminal/callback.state" ] && [ "$(cbs $b)" = done ] && [ "$(efiles $b)" = 1 ]; } && ok "(R6-EC2) resume-callback over a FIFO callback.state does not block: the damaged state is replaced by a real one and the callback completes (one effect)" || bad "(R6-EC2 callback.state resume) rc=$rc state=$(cbs $b) out=$out"
unfifo "$builds/$b/terminal/callback.state"
b=R6E2E; newbuild $b RR6E2E; consume "$(efile "$(ev $b RR6E2E 1 accepted)")" >/dev/null; consume "$(efile "$(ev $b RR6E2E 2 completed "$CMP")")" >/dev/null
command rm -f -- "$builds/$b/terminal/state.json"; mkfifo "$builds/$b/terminal/state.json"
bounded bash "$EC" consume "$builds" "$state" "$(efile "$(ev $b RR6E2E 3 heartbeat "$HB")")"; out=$bout; rc=$brc
{ [ $rc -eq 20 ] && printf '%s' "$out" | grep -q 'state_corrupt' && lockfree "$builds/$b/.lock"; } && ok "(R6-EC2) a FIFO at terminal/state.json is refused (state_corrupt) within the bound, lock free" || bad "(R6-EC2 state.json fifo) rc=$rc out=$out"
unfifo "$builds/$b/terminal/state.json"
b=R6E2F; newbuild $b RR6E2F; consume "$(efile "$(ev $b RR6E2F 1 accepted)")" >/dev/null; consume "$(efile "$(ev $b RR6E2F 2 completed "$CMP")")" >/dev/null
cbset $b claimed; rm -rf "${builds:?}/${b:?}/effects" "${builds:?}/${b:?}/effects.log"; command rm -f -- "$builds/$b/callback.json"; mkfifo "$builds/$b/callback.json"
bounded bash "$EC" resume-callback "$builds" $b; out=$bout; rc=$brc
{ [ $rc -eq 21 ] && [ "$(cbs $b)" = failed ] && grep -q '^effect_unreadable$' "$builds/$b/terminal/callback.reason"; } && ok "(R6-EC2) a FIFO at callback.json fails the callback (effect_unreadable) within the bound" || bad "(R6-EC2 callback.json fifo) rc=$rc (124 = blocked) state=$(cbs $b) out=$out"
unfifo "$builds/$b/callback.json"

# ---- EC-3: a DUP redo of a callback found `running` re-runs it (the under-lock re-check accepts claimed AND running) ----
b=R6E3; newbuild $b RR6E3; consume "$(efile "$(ev $b RR6E3 1 accepted)")" >/dev/null; e3=$(efile "$(ev $b RR6E3 2 completed "$CMP")"); consume "$e3" >/dev/null
rm -rf "${builds:?}/${b:?}/effects" "${builds:?}/${b:?}/effects.log"; cbset $b running
out=$(bash "$EC" consume "$builds" "$state" "$e3" 2>&1); rc=$?
{ [ $rc -eq 0 ] && printf '%s' "$out" | grep -q '^DUP' && [ "$(cbs $b)" = done ] && [ "$(efiles $b)" = 1 ] && [ "$(effects $b)" = 1 ]; } && ok "(R6-EC3) a DUP of a completed event whose callback was left RUNNING re-runs it: done, one effect, one audit line" || bad "(R6-EC3) rc=$rc state=$(cbs $b) efiles=$(efiles $b) effects=$(effects $b) out=$out"

# ---- EC-4: a progress.json above the 4096-byte bound is state_corrupt even when it is valid JSON ----
b=R6E4; newbuild $b RR6E4; consume "$(efile "$(ev $b RR6E4 1 accepted)")" >/dev/null; consume "$(efile "$(ev $b RR6E4 2 heartbeat "$HB")")" >/dev/null
{ printf '{"progress_offset":10,"stage":1}'; head -c 5000 /dev/zero | tr '\0' ' '; printf '\n'; } > "$builds/$b/progress.json"
out=$(bash "$EC" consume "$builds" "$state" "$(efile "$(ev $b RR6E4 3 heartbeat "$HB")")" 2>&1); rc=$?
{ [ $rc -eq 20 ] && printf '%s' "$out" | grep -q 'state_corrupt' && [ ! -e "$builds/$b/consumed/3" ]; } && ok "(R6-EC4) a 5 KB progress.json (valid JSON plus spaces) is refused (state_corrupt): the 4096-byte bound holds" || bad "(R6-EC4 oversize) rc=$rc out=$out"
printf '{"progress_offset":10,"stage":1}\n' > "$builds/$b/progress.json"
out=$(bash "$EC" consume "$builds" "$state" "$(efile "$(ev $b RR6E4 3 heartbeat "$HB")")" 2>&1); rc=$?
{ [ $rc -eq 0 ] && printf '%s' "$out" | grep -q 'consumed seq=3'; } && ok "(R6-EC4) control: the same JSON without the padding is consumed" || bad "(R6-EC4 control) rc=$rc out=$out"

echo "OWED (not covered by this slice): $OWED_CASES"
echo "RESULT pass=$pass fail=$fail skip=$skipn"
[ $fail -eq 0 ]
