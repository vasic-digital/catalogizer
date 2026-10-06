#!/usr/bin/env bash
# T050 round 5 (independent review WF5-REVIEW-wp05-evrec, findings F1 F2 F3 F5 F6 F7): cases written BEFORE the code changes and
# run RED against the round-4 tree first ($EV/wp05/T050r5-red.txt). Every refusal check is reason-aware (exit code AND reason=).
# Secret-looking fixtures are assembled from pieces at run time (no credential-shaped literal committed).
set -u
here=$(cd "$(dirname "$0")" && pwd); root=$(cd "$here/../../.." && pwd)
EVREC=$root/tools/evidence/evrec; VERIFY=$root/tools/evidence/verify; EVD=$root/tools/evidence
S=$(mktemp -d); trap 'kill $(jobs -p) 2>/dev/null; rm -rf "${S:?}"' EXIT
fails=0; n=0
. "$here/hermetic.sh"; hermetic_init "$S"
ok()  { n=$((n+1)); if [ -x "$EVREC" ] && [ -x "$VERIFY" ]; then echo "ok   $1"; else fails=$((fails+1)); echo "FAIL $1 (vacuous: recorder/verifier absent)"; fi; }
bad() { n=$((n+1)); fails=$((fails+1)); echo "FAIL $1"; }
fresh() { rm -rf "${S:?}/w"; mkdir -p "$S/w"; export EV=$S/w EV_LEDGER=$S/w/ledger.jsonl EV_ANCHOR=$S/w/anchor.jsonl EV_BLOBS=$S/w/blobs; hermetic_repo; }
rec() { "$EVREC" run "${1:-CAT-001}" "${2:-PROBE}" "${3:-1}" shell_script x -- "${@:4}"; }
refuse() { local name=$1 rc=$2 reason=$3; shift 3; local out r; out=$("$@" 2>&1); r=$?
  if [ "$r" = "$rc" ] && grep -q "reason=$reason" <<<"$out" && ! grep -q Traceback <<<"$out"; then ok "$name"; else bad "$name (rc=$r want $rc, reason=$reason; got: $(head -c 200 <<<"$out" | tr '\n' ' '))"; fi; }
want() { local name=$1 w=$2; shift 2; local r; "$@" >/dev/null 2>&1; r=$?; [ "$r" = "$w" ] && ok "$name" || bad "$name (rc=$r want $w)"; }
sha() { sha256sum | cut -d' ' -f1; }
OR=(--oracle specified --oracle-independent --evidence-class runtime)
PY() { PYTHONDONTWRITEBYTECODE=1 python3 -B "$@"; }

# ===== F1: redaction runs in linear time (quadratic regexes on long runs of [A-Za-z0-9_.-] and dashes hung evrec after the command ran)
N=100000
tcase() { # tcase NAME PYEXPR  (PYEXPR builds the bytes `b`); the case must finish within 15 s (RED: the unfixed code does not)
  local name=$1 expr=$2 t0 t1 r
  t0=$(date +%s.%N)
  timeout 15 python3 -B -c "
import sys; sys.dont_write_bytecode=True; sys.path.insert(0, '$EVD')
import evcore
N=$N
b=$expr
out,hit=evcore.redact_bytes(b, [])
" >/dev/null 2>&1; r=$?
  t1=$(date +%s.%N)
  if [ "$r" = 0 ]; then ok "F1: redact_bytes on $name returns in under 15 s ($(awk -v a="$t0" -v b="$t1" 'BEGIN{printf "%.1f", b-a}') s)"; else bad "F1: redact_bytes on $name did not finish in 15 s (rc=$r)"; fi
}
tcase "a run of $N dashes"                      'b"-"*N'
tcase "a run of $N x \"a-\""                   'b"a-"*(N//2)'
tcase "a run of $N letters"                     'b"a"*N'
tcase "a run of $N x \"_.\""                   'b"_."*(N//2)'
tcase "'token' repeated to $N bytes"           'b"token"*(N//5)'
tcase "'password' repeated to $N bytes"        'b"password"*(N//8)'
tcase "a double quote then 'token' repeated"   'b"\""+b"token"*(N//5)'
tcase "a single quote then 'secret' repeated"  'b"\x27"+b"secret"*(N//6)'
tcase "-token repeated to $N bytes"            'b"-token"*(N//6)'
tcase "a long run of dashes then a flag name"  'b"-"*N+b"-password x"'
tcase "'--' then $N letters"                    'b"--"+b"x"*N'
tcase "'api-key' repeated to $N bytes"         'b"api-key"*(N//7)'
tcase "'access_key.' repeated to $N bytes"     'b"access_key."*(N//11)'
t0=$(date +%s.%N)
timeout 15 python3 -B -c "
import sys; sys.dont_write_bytecode=True; sys.path.insert(0, '$EVD')
import evcore
N=$N
evcore.redact_argv(['--'+'token'*(N//5)+'!', '-'*N, 'x'], [])
" >/dev/null 2>&1; r=$?; t1=$(date +%s.%N)
[ "$r" = 0 ] && ok "F1: redact_argv on a $N-byte credential-flag-shaped element returns in under 15 s ($(awk -v a="$t0" -v b="$t1" 'BEGIN{printf "%.1f", b-a}') s)" || bad "F1: redact_argv did not finish in 15 s (rc=$r)"
# the linear patterns still redact what they redacted (no false negative introduced by the fix)
P1=Hunter; P2=22Hunter
out=$(timeout 60 python3 -B -c "
import sys; sys.dont_write_bytecode=True; sys.path.insert(0, '$EVD')
import evcore
N=$N
V=b'$P1$P2'
cases={
 'kv after a long dash run': b'-'*N + b' pass'+b'word='+V,
 'kv name prefix': b'x-pass'+b'word='+V,
 'kv name with a 100-char tail': b'to'+b'ken'+b'a'*100+b'='+V,
 'kv after a long letter run': b'a'*N + b' secret: '+V,
 'json double quote': b'{\"api_ke'+b'y\": \"'+V+b'\"}',
 'flag pair after dashes': b'-'*N + b' --secret-'+b'key '+V,
 'flag pair prefix flag': b'run --my-pass'+b'word '+V,
}
for k,b in cases.items():
    out,hit=evcore.redact_bytes(b, [])
    print(('ok' if hit and V not in out else 'MISS'), k)
")
[ -n "$out" ] || bad "F1: the functional redaction check did not finish in 60 s"
while IFS= read -r l; do [ -n "$l" ] || continue; case "$l" in ok*) ok "F1: still redacted: ${l#ok }";; *) bad "F1: NOT redacted: ${l#MISS }";; esac; done <<<"$out"
# end to end: a command that prints a long run of dashes is recorded (nothing hangs after the command ran)
fresh; mkdir -p "$S/f1"; printf '#!/usr/bin/env bash\nhead -c 100000 /dev/zero | tr "\\0" "-"; echo\n' >"$S/f1/dashes.sh"; chmod +x "$S/f1/dashes.sh"
out=$(timeout 40 "$EVREC" run CAT-001 RED 1 shell_script "$S/f1" "${OR[@]}" --test-source "$S/f1/dashes.sh" -- bash "$S/f1/dashes.sh" 2>&1); r=$?
{ [ "$r" = 0 ] || grep -q "reason=record_invalid" <<<"$out"; } && ! grep -q Traceback <<<"$out" && [ "$r" != 124 ] && ok "F1: evrec run of a command printing 100000 dashes finishes (rc=$r, not a timeout)" || bad "F1: evrec run with a dash-run output rc=$r: $(head -c 160 <<<"$out")"

# ===== F2: the commit-turn guard does not fail open on an empty run_id or an empty EVREC_TURN_RUN_ID
mkgrant() { fresh; rm -rf "${S:?}/ct"; mkdir -p "$S/ct/.audit"; export EVREC_REPO_ROOT=$S/ct; printf '%s\n' "$1" >"$S/ct/.audit/commit_turn.json"; }
mkgrant '{"run_id":""}'
out=$(EVREC_TURN_RUN_ID= "$EVREC" run CAT-001 PROBE 1 shell_script x -- true 2>&1); r=$?
{ [ "$r" = 76 ] && grep -q reason=commit_turn_held <<<"$out"; } && ok "F2: grant {run_id:''} with EVREC_TURN_RUN_ID set and empty is refused (commit_turn_held, 76)" || bad "F2: empty grant run_id + empty env gave rc=$r: $(head -c 160 <<<"$out")"
[ ! -e "$EV_LEDGER" ] && ok "F2: nothing recorded under the empty run_id grant" || bad "F2: ledger written under the empty run_id grant"
mkgrant '{"run_id":""}'
refuse "F2: grant {run_id:''} with EVREC_TURN_RUN_ID unset is refused too" 76 commit_turn_held "$EVREC" run CAT-001 PROBE 1 shell_script x -- true
mkgrant '{"run_id":"g5"}'
refuse "F2: a real grant with EVREC_TURN_RUN_ID set and empty is refused (empty is unset)" 76 commit_turn_held env EVREC_TURN_RUN_ID= "$EVREC" run CAT-001 PROBE 1 shell_script x -- true
want "F2: the holder (EVREC_TURN_RUN_ID equal to the grant run_id) still records" 0 env EVREC_TURN_RUN_ID=g5 "$EVREC" run CAT-001 PROBE 1 shell_script x -- true
mkgrant '{"run_id":" "}'
refuse "F2: a whitespace-only run_id is refused" 76 commit_turn_held env EVREC_TURN_RUN_ID=" " "$EVREC" run CAT-001 PROBE 1 shell_script x -- true
hermetic_repo
# the held-grant cases above reach the host entry stub through the one reaper call only; nothing else may
if [ -z "$(grep -v -e 'commit_turn_check.sh --reap' "$H/host_calls")" ]; then ok "F2: the only host-entry calls were the reaper's (--exec-approved ... --reap)"; else bad "F2: unexpected host-entry call: $(grep -v -e 'commit_turn_check.sh --reap' "$H/host_calls" | head -2)"; fi
: >"$H/host_calls"

# ===== F3: the generic operand layer covers --opt=FILE values, directory operands and interpreters reached through a symlink
mkdir -p "$S/f3"; printf '#!/usr/bin/env bash\nexit 1\n' >"$S/f3/testA.sh"; printf '#!/usr/bin/env bash\nexit 0\n' >"$S/f3/testB.sh"; chmod +x "$S/f3/"test*.sh
printf '#!/usr/bin/env bash\nfor a in "$@"; do case "$a" in --bank=*) exec bash "${a#--bank=}";; esac; done\nexit 3\n' >"$S/f3/bankrun.sh"
printf '#!/usr/bin/env bash\nexec bash "$1/t.sh"\n' >"$S/f3/dirrun.sh"; chmod +x "$S/f3/bankrun.sh" "$S/f3/dirrun.sh"
fresh; "$EVREC" run CAT-001 RED 1 shell_script "$S/f3" "${OR[@]}" -- "$S/f3/bankrun.sh" --bank="$S/f3/testA.sh" >/dev/null 2>&1
"$EVREC" run CAT-001 GREEN 1 shell_script "$S/f3" "${OR[@]}" -- "$S/f3/bankrun.sh" --bank="$S/f3/testB.sh" >/dev/null 2>&1
fp=$(jq -r .test_fingerprint "$EV_LEDGER" 2>/dev/null | sort -u | wc -l | tr -d ' ')
[ "$fp" = 2 ] && ok "F3: an unlisted launcher with --bank=FILE: RED and GREEN over different test files get different test_fingerprints" || bad "F3: --opt=FILE launcher gives $fp distinct fingerprint(s)"
want_fp=$( { sha256sum <"$S/f3/bankrun.sh" | cut -d' ' -f1; sha256sum <"$S/f3/testA.sh" | cut -d' ' -f1; } | sha )
[ "$(jq -r .test_fingerprint "$EV_LEDGER" | head -1)" = "$want_fp" ] && ok "F3: the --opt=FILE fingerprint is sha256(sha256(argv0)\\nsha256(the file after '=')\\n)" || bad "F3: --opt=FILE fingerprint $(jq -r .test_fingerprint "$EV_LEDGER" | head -1) want $want_fp"
mkdir -p "$S/f3/dA" "$S/f3/dB"; cp "$S/f3/testA.sh" "$S/f3/dA/t.sh"; cp "$S/f3/testB.sh" "$S/f3/dB/t.sh"
fresh; "$EVREC" run CAT-001 RED 1 shell_script "$S/f3" "${OR[@]}" -- "$S/f3/dirrun.sh" "$S/f3/dA" >/dev/null 2>&1
"$EVREC" run CAT-001 GREEN 1 shell_script "$S/f3" "${OR[@]}" -- "$S/f3/dirrun.sh" "$S/f3/dB" >/dev/null 2>&1
fp=$(jq -r .test_fingerprint "$EV_LEDGER" 2>/dev/null | sort -u | wc -l | tr -d ' ')
[ "$fp" = 2 ] && ok "F3: an unlisted launcher with a DIRECTORY operand: different directory contents give different test_fingerprints" || bad "F3: directory operand gives $fp distinct fingerprint(s)"
"$EVREC" run CAT-001 GREEN 2 shell_script "$S/f3" "${OR[@]}" -- "$S/f3/dirrun.sh" "$S/f3/dB" >/dev/null 2>&1
[ "$(jq -r .test_fingerprint "$EV_LEDGER" | tail -2 | sort -u | wc -l | tr -d ' ')" = 1 ] && ok "F3: the same directory contents give the same test_fingerprint (deterministic)" || bad "F3: same directory gave different fingerprints"
fresh; mkdir -p "$S/f3/big"; for i in 1 2 3 4 5; do echo "$i" >"$S/f3/big/f$i"; done
refuse "F3: a directory operand over EVREC_OPERAND_DIR_MAX files on a RED is refused (operand_directory_too_large, never silently skipped)" 69 operand_directory_too_large env EVREC_OPERAND_DIR_MAX=3 "$EVREC" run CAT-001 RED 1 shell_script "$S/f3" "${OR[@]}" -- "$S/f3/dirrun.sh" "$S/f3/big"
want "F3: the same oversized directory operand on a PROBE is not refused" 0 env EVREC_OPERAND_DIR_MAX=3 "$EVREC" run CAT-001 PROBE 1 shell_script "$S/f3" -- true "$S/f3/big"
# an interpreter reached through a renamed symlink is still an interpreter (realpath), a copy is not claimed
mkdir -p "$S/f3/bin"; ln -s "$(command -v bash)" "$S/f3/bin/mysh"; ln -s "$(command -v python3)" "$S/f3/bin/pyx"
fresh; refuse "F3: RED through a renamed symlink to bash with -c is refused (interpreter_without_test_sources)" 69 interpreter_without_test_sources "$EVREC" run CAT-001 RED 1 shell_script "$S/f3" "${OR[@]}" -- "$S/f3/bin/mysh" -c 'exit 1'
refuse "F3: RED through a renamed symlink to python3 with -c is refused" 69 interpreter_without_test_sources "$EVREC" run CAT-001 RED 1 shell_script "$S/f3" "${OR[@]}" -- "$S/f3/bin/pyx" -c 'raise SystemExit(1)'
refuse "F3: the same through PATH lookup of a renamed symlink is refused" 69 interpreter_without_test_sources env PATH="$S/f3/bin:$PATH" "$EVREC" run CAT-001 GREEN 1 shell_script "$S/f3" "${OR[@]}" -- mysh -c 'exit 0'
want "F3: with a declared test source the renamed-symlink interpreter records" 0 "$EVREC" run CAT-001 RED 1 shell_script "$S/f3" "${OR[@]}" --test-source "$S/f3/testA.sh" -- "$S/f3/bin/mysh" "$S/f3/testA.sh"
want "F3: a PROBE through a renamed-symlink interpreter needs no source" 0 "$EVREC" run CAT-001 PROBE 1 shell_script x -- "$S/f3/bin/mysh" -c 'exit 0'

# ===== F5: a ledger reached through EV_LEDGER finds the anchors.jsonl beside it when EV_ANCHOR is unset
fresh; rec CAT-001 PROBE 1 true >/dev/null 2>&1; rm -rf "${S:?}/f5" "${S:?}/f5ev"; mkdir -p "$S/f5" "$S/f5ev"; cp "$EV_LEDGER" "$S/f5/ledger.jsonl"; cp -r "$EV_BLOBS" "$S/f5/blobs"; printf '{"anchor":1}\n' >"$S/f5/anchors.jsonl"
refuse "F5: EV_LEDGER with anchors.jsonl beside it (EV_ANCHOR unset) is UNVERIFIED 3 anchor_not_compared" 3 anchor_not_compared env -u EV_ANCHOR EV="$S/f5ev" EV_LEDGER="$S/f5/ledger.jsonl" EV_BLOBS="$S/f5/blobs" "$VERIFY"
want "F5: the same ledger without an anchors.jsonl beside it verifies 0" 0 env -u EV_ANCHOR EV="$S/f5ev" EV_LEDGER="$S/f5/ledger.jsonl" EV_BLOBS="$S/f5/blobs" bash -c 'rm -f "$(dirname "$EV_LEDGER")/anchors.jsonl"; exec "$0"' "$VERIFY"
printf '{"anchor":1}\n' >"$S/f5/anchors.jsonl"; printf '' >"$S/f5/other.jsonl"
want "F5: an explicit EV_ANCHOR still wins over the file beside the ledger" 0 env EV="$S/f5ev" EV_LEDGER="$S/f5/ledger.jsonl" EV_BLOBS="$S/f5/blobs" EV_ANCHOR="$S/f5/other.jsonl" "$VERIFY"

# ===== F6: reconcile --run counts distinct seq values (a duplicated run-index row is not a second entry)
fresh; export EVREC_RUN_TOKEN=t6; rec CAT-001 PROBE 1 true >/dev/null 2>&1; unset EVREC_RUN_TOKEN
want "F6: fixture: one tokened entry reconciles with --runner-count 1" 0 "$EVREC" reconcile --run t6 --runner-count 1
head -1 "$EV_LEDGER.runs" >>"$EV_LEDGER.runs"
refuse "F6: with that run-index row duplicated, --runner-count 2 is a count_mismatch" 1 count_mismatch "$EVREC" reconcile --run t6 --runner-count 2
want "F6: with that run-index row duplicated, --runner-count 1 still passes" 0 "$EVREC" reconcile --run t6 --runner-count 1
printf '{"run":"t6","seq":[1],"entry_hash":"x"}\n{"run":"t6","seq":"1","entry_hash":"x"}\n' >>"$EV_LEDGER.runs"
want "F6: run-index rows whose seq is not an integer are ignored, not a crash (count still 1)" 0 "$EVREC" reconcile --run t6 --runner-count 1

# ===== F7: rerecord refuses an output path that is an existing directory, cleanly (no traceback, no tmp or lock.guard left)
fresh; mkdir -p "$S/f7"; printf '{"row":"R1"}\n' >"$S/f7/remote"; printf '{"row":"R1"}\n{"row":"L2"}\n' >"$S/f7/local"; rec CAT-001 PROBE 1 true >/dev/null 2>&1
find "$S" -mindepth 1 -maxdepth 1 | sort >"$S/f7/parent.before"; find "$EV" | sort | sha >"$S/f7/ev.before"
refuse "F7: --out <parent of EV> --name <basename of EV> (an existing directory) is refused cleanly" 70 out_is_directory "$EVREC" rerecord --append-only --onto "$S/f7/remote" --local "$S/f7/local" --out "$(dirname "$EV")" --name "$(basename "$EV")"
find "$S" -mindepth 1 -maxdepth 1 | sort >"$S/f7/parent.after"
cmp -s "$S/f7/parent.before" "$S/f7/parent.after" && ok "F7: no <name>.tmp.<pid> or <name>.lock.guard was left in the parent directory" || bad "F7: the parent directory changed: $(diff "$S/f7/parent.before" "$S/f7/parent.after" | tr '\n' ' ')"
[ "$(find "$EV" | sort | sha)" = "$(cat "$S/f7/ev.before")" ] && ok "F7: the evidence directory tree is unchanged" || bad "F7: the evidence directory tree changed"
mkdir -p "$S/f7/out/merged.jsonl"
refuse "F7: --name that is an existing directory inside --out is refused (out_is_directory)" 70 out_is_directory "$EVREC" rerecord --append-only --onto "$S/f7/remote" --local "$S/f7/local" --out "$S/f7/out" --name merged.jsonl
[ -z "$(find "$S/f7/out" -mindepth 1 -maxdepth 1 ! -name merged.jsonl)" ] && ok "F7: nothing else was created in --out" || bad "F7: --out holds $(ls -A "$S/f7/out" | tr '\n' ' ')"
mkdir -p "$S/f7/out2/ledger-seq-map.json"
refuse "F7: the seq-map path being an existing directory is refused too" 70 out_is_directory "$EVREC" rerecord --append-only --onto "$S/f7/remote" --local "$S/f7/local" --out "$S/f7/out2"
# a failed atomic write leaves no tmp file behind
out=$(PY -c "
import sys, os; sys.dont_write_bytecode=True; sys.path.insert(0, '$EVD')
import evcore
d='$S/f7/aw'; os.makedirs(d+'/target')
try:
    evcore.atomic_write(d+'/target', b'x')
    print('NOEXC')
except OSError:
    print('exc', sorted(os.listdir(d)))
")
[ "$out" = "exc ['target']" ] && ok "F7: atomic_write onto a directory raises OSError and removes its tmp file" || bad "F7: atomic_write leftovers: $out"
if [ "$(id -u)" != 0 ]; then
  mkdir -p "$S/f7/ro"; chmod 555 "$S/f7/ro"
  refuse "F7: an unwritable --out is a named refusal (out_unwritable), not a traceback" 70 out_unwritable "$EVREC" rerecord --append-only --onto "$S/f7/remote" --local "$S/f7/local" --out "$S/f7/ro" --name merged.jsonl
  chmod 755 "$S/f7/ro"
else ok "F7: (skipped, running as root: a read-only directory is writable)"; fi
want "F7: a plain --name into a fresh --out still works" 0 "$EVREC" rerecord --append-only --onto "$S/f7/remote" --local "$S/f7/local" --out "$S/f7/out3" --name merged.jsonl

hermetic_repo
[ "$(hermetic_host_calls)" = 0 ] && ok "hermetic: no call reached the host entry" || bad "hermetic: $(hermetic_host_calls) call(s) reached the host entry"
echo "checks=$n failures=$fails"; [ "$fails" -eq 0 ]
