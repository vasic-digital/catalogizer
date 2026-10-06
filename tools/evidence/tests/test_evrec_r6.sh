#!/usr/bin/env bash
# T050 round 6 (independent review WF6-REVIEW-recorder, findings R6-1 R6-2 R6-4 R6-5 R6-6 R6-8 and mutation-adequacy R6-6/R6-11):
# cases written BEFORE the code changes and run RED against the round-5 tree first ($EV/wp05/T050r6-red.txt). Every refusal check
# is reason-aware (exit code AND reason=). Secret-looking fixtures are assembled from pieces at run time and are never printed
# (a failing check prints the check label only, never a fixture value).
set -u
here=$(cd "$(dirname "$0")" && pwd); root=$(cd "$here/../../.." && pwd)
EVREC=$root/tools/evidence/evrec; VERIFY=$root/tools/evidence/verify; EVD=$root/tools/evidence
S=$(mktemp -d); trap 'kill $(jobs -p) 2>/dev/null; chmod -R u+rwx "${S:?}" 2>/dev/null; rm -rf "${S:?}"' EXIT
fails=0; n=0
. "$here/hermetic.sh"; hermetic_init "$S"
ok()  { n=$((n+1)); if [ -x "$EVREC" ] && [ -x "$VERIFY" ]; then echo "ok   $1"; else fails=$((fails+1)); echo "FAIL $1 (vacuous: recorder/verifier absent)"; fi; }
bad() { n=$((n+1)); fails=$((fails+1)); echo "FAIL $1"; }
fresh() { rm -rf "${S:?}/w"; mkdir -p "$S/w"; export EV=$S/w EV_LEDGER=$S/w/ledger.jsonl EV_ANCHOR=$S/w/anchor.jsonl EV_BLOBS=$S/w/blobs; hermetic_repo; }
refuse() { local name=$1 rc=$2 reason=$3; shift 3; local out r; out=$("$@" 2>&1); r=$?
  if [ "$r" = "$rc" ] && grep -q "reason=$reason" <<<"$out" && ! grep -q Traceback <<<"$out"; then ok "$name"; else bad "$name (rc=$r want $rc, reason=$reason; got: $(head -c 200 <<<"$out" | tr '\n' ' '))"; fi; }
want() { local name=$1 w=$2; shift 2; local r out; out=$("$@" 2>&1); r=$?; if [ "$r" = "$w" ] && ! grep -q Traceback <<<"$out"; then ok "$name"; else bad "$name (rc=$r want $w$(grep -q Traceback <<<"$out" && echo ', traceback'))"; fi; }
OR=(--oracle specified --oracle-independent --evidence-class runtime)

# ===== R6-2: no length bound on the name around a credential word (a 128-character bound leaked what round 4 redacted)
# shapes x tail lengths 128 / 129 / 5000; a fake value assembled at run time, never printed. 'ok <label>' / 'MISS <label>'.
out=$(timeout 120 python3 -B -c "
import sys; sys.dont_write_bytecode=True; sys.path.insert(0, '$EVD')
import evcore
V=('Zq'+'7Rk'+'9Pw'+'2Lm').encode(); Vs=V.decode()
pw=b'pass'+b'word'; tk=b'to'+b'ken'
def red(b): out,hit=evcore.redact_bytes(b, []); return V not in out
for L in (128, 129, 5000):
    cases={
     'kv: credential word + %d name chars + =V':                 pw+b'a'*L+b'='+V,
     'kv YAML form (word + %d chars + \": V\")':                  pw+b'_'+b'x'*L+b': '+V,
     'env dump export SVC_<WORD>_<%d chars>=V':                  b'export SVC_'+tk.upper()+b'_'+b'Q'*L+b'='+V,
     'flag head: --<%d chars><word> V':                          b'--'+b'h'*L+pw+b' '+V,
     'flag tail: --<word><%d chars> V':                          b'--'+pw+b't'*L+b' '+V,
     'JSON double quotes, %d-char tail':                         b'{\"'+tk+b'y'*L+b'\": \"'+V+b'\"}',
     'JSON single quotes, %d-char tail':                         b\"{'\"+tk+b'y'*L+b\"': '\"+V+b\"'}\",
     'JSON double quotes, %d-char HEAD':                         b'{\"'+b'h'*L+tk+b'\": \"'+V+b'\"}',
     'argv-style element --<word><%d chars>=V':                  b'--'+pw+b'z'*L+b'='+V,
    }
    for k,b in cases.items(): print('ok' if red(b) else 'MISS', k % L)
    o,h=evcore.redact_argv(['--'+pw.decode()+'z'*L+'='+Vs], [])
    print('ok' if Vs not in ' '.join(o) else 'MISS', 'argv element --<word><%d chars>=V through redact_argv' % L)
    o,h=evcore.redact_argv(['--'+pw.decode()+'z'*L, Vs], [])
    print('ok' if Vs not in ' '.join(o) else 'MISS', 'argv bare flag with a %d-char name, value in the next element' % L)
    o,h=evcore.redact_argv(['--'+'z'*L+pw.decode(), Vs], [])
    print('ok' if Vs not in ' '.join(o) else 'MISS', 'argv bare flag with a %d-char head, value in the next element' % L)
")
[ -n "$out" ] || bad "R6-2: the unbounded-name redaction check did not finish in 120 s"
while IFS= read -r l; do [ -n "$l" ] || continue; case "$l" in ok*) ok "R6-2: still redacted: ${l#ok }";; *) bad "R6-2: NOT redacted: ${l#MISS }";; esac; done <<<"$out"
# negative controls: the longer name must not make things over-redact
out=$(timeout 60 python3 -B -c "
import sys; sys.dont_write_bytecode=True; sys.path.insert(0, '$EVD')
import evcore
V=('Zq'+'7Rk'+'9Pw'+'2Lm').encode()
pw=b'pass'+b'word'
def kept(b, needle): out,hit=evcore.redact_bytes(b, []); return needle in out
cases={
 'a 5000-char name with no credential word, then =V':   (b'x'*5000+b'='+V, V),
 'a credential word followed by a value shorter than 6': (pw+b'a'*300+b'=abc', b'abc'),
 'a credential word in prose with no separator':         (b'my '+pw+b' is long and '+V+b' stays', V),
 'a flag whose value starts with a dash':                (b'--'+pw+b'a'*300+b' -'+V, V),
 'a 5000-char dashed name with no credential word':      (b'--'+b'z-'*2500+b' '+V, V),
}
for k,(b,nd) in cases.items(): print('ok' if kept(b, nd) else 'MISS', k)
")
while IFS= read -r l; do [ -n "$l" ] || continue; case "$l" in ok*) ok "R6-2: negative control, not redacted: ${l#ok }";; *) bad "R6-2: negative control WAS redacted: ${l#MISS }";; esac; done <<<"$out"
# the same through the recorder: the stdout blob of a command that prints the 5000-char-tail credential holds no value
fresh; mkdir -p "$S/r62"
printf '#!/usr/bin/env bash\nV=%s%s%s\nprintf "pass""word%%s=%%s\\n" "$(head -c 5000 /dev/zero | tr "\\0" a)" "$V"\n' Zq 7Rk9Pw 2Lm >"$S/r62/long.sh"; chmod +x "$S/r62/long.sh"
"$EVREC" run CAT-001 PROBE 1 shell_script x -- bash "$S/r62/long.sh" >/dev/null 2>&1; r=$?
V=Zq7Rk9Pw2Lm
if [ "$r" = 0 ] && ! grep -rqF "$V" "$S/w" 2>/dev/null; then ok "R6-2: end to end: a 5000-char-tail credential printed by a command is in no ledger line and no blob"; else bad "R6-2: end to end: rc=$r or the value reached the ledger/blobs"; fi

# ===== R6-1: the jwt pattern is linear; a JWT that follows '-' is still redacted
tcase() { # tcase NAME SIZE PYEXPR   (PYEXPR builds the bytes `b` from N); the case must finish within 10 s
  local name=$1 size=$2 expr=$3 t0 t1 r
  t0=$(date +%s.%N)
  timeout 10 python3 -B -c "
import sys; sys.dont_write_bytecode=True; sys.path.insert(0, '$EVD')
import evcore
N=$size
b=$expr
out,hit=evcore.redact_bytes(b, [])
" >/dev/null 2>&1; r=$?
  t1=$(date +%s.%N)
  if [ "$r" = 0 ]; then ok "R6-1: redact_bytes on $name ($size bytes) returns in under 10 s ($(awk -v a="$t0" -v b="$t1" 'BEGIN{printf "%.2f", b-a}') s)"; else bad "R6-1: redact_bytes on $name ($size bytes) did not finish in 10 s (rc=$r)"; fi
}
for sz in 100000 200000 400000; do
  tcase "'-eyJ' repeated"                      $sz 'b"-eyJ"*(N//4)'
  tcase "a run of dashes"                      $sz 'b"-"*N'
done
tcase "'eyJ' repeated"                         400000 'b"eyJ"*(N//3)'
tcase "'eyJ-' repeated"                        400000 'b"eyJ-"*(N//4)'
tcase "'-eyJaaaaaaaaa.' repeated"              400000 'b"-eyJaaaaaaaaa."*(N//15)'
tcase "'eyJaaaaaaaaa.bbbbbbbbb.' repeated"     400000 'b"eyJaaaaaaaaa.bbbbbbbbb."*(N//24)'
tcase "'-eyJaaaaaaaaa.bbbbbbbbb' repeated"     400000 'b"-eyJaaaaaaaaa.bbbbbbbbb"*(N//24)'
tcase "a run of letters"                       400000 'b"a"*N'
tcase "a run of x \"a-\""                      400000 'b"a-"*(N//2)'
tcase "a run of x \"_.\""                      400000 'b"_."*(N//2)'
tcase "'token' repeated"                       400000 'b"token"*(N//5)'
tcase "'-token' repeated"                      400000 'b"-token"*(N//6)'
tcase "'password' repeated"                    400000 'b"password"*(N//8)'
tcase "'--password ' repeated"                 400000 'b"--password "*(N//11)'
tcase "'token:' repeated"                      400000 'b"token:"*(N//6)'
tcase "a double quote then 'token' repeated"   400000 'b"\""+b"token"*(N//5)'
tcase "a single quote then 'secret' repeated"  400000 'b"\x27"+b"secret"*(N//6)'
tcase "'\"token\":\"' repeated"                400000 'b"\"token\":\""*(N//9)'
tcase "'api-key' repeated"                     400000 'b"api-key"*(N//7)'
out=$(timeout 60 python3 -B -c "
import sys; sys.dont_write_bytecode=True; sys.path.insert(0, '$EVD')
import evcore
J=b'eyJhbGciOiJIUzI1NiJ9'+b'.'+b'eyJzdWIiOiJ4eHh4eHgifQ'+b'.'+b'Zq7Rk9Pw2Lm'
cases={
 'a bare JWT':                                   b'x '+J+b' y',
 'a JWT after a dash':                           b'run-'+J,
 'a JWT after dashes after a long dash run':     b'-'*100000+b'-'+J,
 'a JWT after a dash inside a name run':         b'abc-def-'+J,
 'a JWT at the start':                           J,
}
for k,b in cases.items():
    out,hit=evcore.redact_bytes(b, [])
    print('ok' if hit and b'Zq7Rk9Pw2Lm' not in out else 'MISS', k)
b=b'eyJshort.x.y and text'
out,hit=evcore.redact_bytes(b, [])
print('ok' if out==b else 'MISS', 'a too-short three-part token is not touched (negative control)')
")
while IFS= read -r l; do [ -n "$l" ] || continue; case "$l" in ok*) ok "R6-1: ${l#ok }";; *) bad "R6-1: ${l#MISS }";; esac; done <<<"$out"

# ===== R6-4: operand hashing never ends in a traceback
mkdir -p "$S/d" "$S/tgt"; echo t >"$S/tgt/f"; TGT=$S/tgt; printf '#!/usr/bin/env bash\nexec bash "$1/t.sh"\n' >"$S/d/dirrun.sh"; chmod +x "$S/d/dirrun.sh"
mkdir -p "$S/d/u"; printf '#!/usr/bin/env bash\nexit 1\n' >"$S/d/u/t.sh"; printf 'x\n' >"$S/d/u/"$'\xff.sh'
fresh; want "R6-4: a directory operand holding a non-UTF-8 file name records (no traceback)" 0 "$EVREC" run CAT-001 RED 1 shell_script "$TGT" "${OR[@]}" -- "$S/d/dirrun.sh" "$S/d/u"
fp1=$(jq -r .test_fingerprint "$EV_LEDGER" 2>/dev/null | tail -1)
mv "$S/d/u/"$'\xff.sh' "$S/d/u/"$'\xfe.sh'
"$EVREC" run CAT-001 RED 2 shell_script "$TGT" "${OR[@]}" -- "$S/d/dirrun.sh" "$S/d/u" >/dev/null 2>&1
fp2=$(jq -r .test_fingerprint "$EV_LEDGER" 2>/dev/null | tail -1)
[ -n "$fp1" ] && [ -n "$fp2" ] && [ "$fp1" != "$fp2" ] && ok "R6-4: two different non-UTF-8 file names give different test_fingerprints" || bad "R6-4: non-UTF-8 name fingerprints: '$fp1' vs '$fp2'"
mkdir -p "$S/d/sp"; printf '#!/usr/bin/env bash\nexit 1\n' >"$S/d/sp/t.sh"
if [ -e /proc/self/mem ]; then
  ln -s /proc/self/mem "$S/d/sp/mem"
  fresh; refuse "R6-4: a directory operand holding a file that cannot be read is refused on a RED (operand_unreadable, 69)" 69 operand_unreadable "$EVREC" run CAT-001 RED 1 shell_script "$TGT" "${OR[@]}" -- "$S/d/dirrun.sh" "$S/d/sp"
  want "R6-4: the same operand on a PROBE is skipped, not a traceback" 0 "$EVREC" run CAT-001 PROBE 1 shell_script x -- true "$S/d/sp"
  [ ! -s "$EV_LEDGER" ] || [ "$(wc -l <"$EV_LEDGER")" = 1 ] && ok "R6-4: the refused RED stored nothing (only the PROBE line)" || bad "R6-4: ledger lines after the refused RED: $(wc -l <"$EV_LEDGER")"
else echo "skip R6-4: no /proc/self/mem on this host (unreadable-file case not exercised)"; fi
if [ "$(id -u)" != 0 ]; then
  mkdir -p "$S/d/ud/sub"; printf '#!/usr/bin/env bash\nexit 1\n' >"$S/d/ud/t.sh"; chmod 000 "$S/d/ud/sub"
  fresh; refuse "R6-4: a directory operand holding an unlistable subdirectory is refused on a RED (operand_unreadable, 69)" 69 operand_unreadable "$EVREC" run CAT-001 RED 1 shell_script "$TGT" "${OR[@]}" -- "$S/d/dirrun.sh" "$S/d/ud"
  chmod 755 "$S/d/ud/sub"
else echo "skip R6-4: running as root (unlistable-directory case not exercised)"; fi
mkdir -p "$S/d/ff"; printf '#!/usr/bin/env bash\nexit 1\n' >"$S/d/ff/t.sh"; mkfifo "$S/d/ff/pipe"
fresh; want "R6-4: a FIFO inside a directory operand is skipped, never opened (no hang)" 0 timeout 30 "$EVREC" run CAT-001 RED 1 shell_script "$TGT" "${OR[@]}" -- "$S/d/dirrun.sh" "$S/d/ff"

# the same fix applies to a TARGET directory (fingerprint_target had the same encode() pattern)
fresh; mkdir -p "$S/d/tdir"; printf 'x\n' >"$S/d/tdir/"$'\xff.txt'
want "R6-4: a target directory holding a non-UTF-8 file name records (no traceback)" 0 "$EVREC" run CAT-001 PROBE 1 shell_script "$S/d/tdir" -- true

# ===== R6-5: the directory operand cost is bounded by entries AND bytes, checked before anything is hashed
mkdir -p "$S/c/dirs"; printf '#!/usr/bin/env bash\nexit 1\n' >"$S/c/dirs/t.sh"; for i in 1 2 3 4 5; do mkdir -p "$S/c/dirs/e$i"; done
fresh; refuse "R6-5: a directory operand with more entries than the cap (empty subdirectories count) is refused on a RED" 69 operand_directory_too_large env EVREC_OPERAND_DIR_MAX=3 "$EVREC" run CAT-001 RED 1 shell_script "$TGT" "${OR[@]}" -- "$S/d/dirrun.sh" "$S/c/dirs"
mkdir -p "$S/c/fix3" "$S/c/fix4"; printf '#!/usr/bin/env bash\nexit 1\n' >"$S/c/fix3/t.sh"; echo a >"$S/c/fix3/a"; echo b >"$S/c/fix3/b"
cp "$S/c/fix3/"* "$S/c/fix4/"; echo c >"$S/c/fix4/c"
fresh; want "R6-5: exactly N=3 files with EVREC_OPERAND_DIR_MAX=3 is accepted" 0 env EVREC_OPERAND_DIR_MAX=3 "$EVREC" run CAT-001 RED 1 shell_script "$TGT" "${OR[@]}" -- "$S/d/dirrun.sh" "$S/c/fix3"
refuse "R6-5: N+1=4 files with EVREC_OPERAND_DIR_MAX=3 is refused" 69 operand_directory_too_large env EVREC_OPERAND_DIR_MAX=3 "$EVREC" run CAT-001 RED 1 shell_script "$TGT" "${OR[@]}" -- "$S/d/dirrun.sh" "$S/c/fix4"
mkdir -p "$S/c/bytes"; printf '#!/usr/bin/env bash\nexit 1\n' >"$S/c/bytes/t.sh"; head -c 2000 /dev/zero >"$S/c/bytes/blob"
fresh; refuse "R6-5: a directory operand holding more bytes than EVREC_OPERAND_DIR_BYTES_MAX is refused on a RED" 69 operand_directory_too_large env EVREC_OPERAND_DIR_BYTES_MAX=1000 "$EVREC" run CAT-001 RED 1 shell_script "$TGT" "${OR[@]}" -- "$S/d/dirrun.sh" "$S/c/bytes"
want "R6-5: the same directory under the byte cap is accepted" 0 env EVREC_OPERAND_DIR_BYTES_MAX=100000 "$EVREC" run CAT-001 RED 1 shell_script "$TGT" "${OR[@]}" -- "$S/d/dirrun.sh" "$S/c/bytes"
want "R6-5: the same directory over the byte cap on a PROBE is skipped, not refused" 0 env EVREC_OPERAND_DIR_BYTES_MAX=1000 "$EVREC" run CAT-001 PROBE 1 shell_script x -- true "$S/c/bytes"
mkdir -p "$S/c/sparse"; printf '#!/usr/bin/env bash\nexit 1\n' >"$S/c/sparse/t.sh"; truncate -s 4G "$S/c/sparse/big"
t0=$(date +%s)
fresh; refuse "R6-5: a directory operand holding a 4 GiB file is refused by the DEFAULT byte cap" 69 operand_directory_too_large "$EVREC" run CAT-001 RED 1 shell_script "$TGT" "${OR[@]}" -- "$S/d/dirrun.sh" "$S/c/sparse"
t1=$(date +%s); [ $((t1-t0)) -le 10 ] && ok "R6-5: the 4 GiB refusal needs no hashing ($((t1-t0)) s, under 10 s)" || bad "R6-5: the 4 GiB refusal took $((t1-t0)) s (the file was hashed before the refusal)"

# ===== R6-8: the two caps are validated at startup, consistently
for v in 0 -1 abc 1.5; do
  fresh; refuse "R6-8: EVREC_OPERAND_DIR_MAX=$v is a usage_error (64) even with no directory operand" 64 usage_error env EVREC_OPERAND_DIR_MAX=$v "$EVREC" run CAT-001 PROBE 1 shell_script x -- true
  refuse "R6-8: EVREC_OPERAND_DIR_BYTES_MAX=$v is a usage_error (64) even with no directory operand" 64 usage_error env EVREC_OPERAND_DIR_BYTES_MAX=$v "$EVREC" run CAT-001 PROBE 1 shell_script x -- true
done
fresh; want "R6-8: a valid EVREC_OPERAND_DIR_MAX still records" 0 env EVREC_OPERAND_DIR_MAX=5 "$EVREC" run CAT-001 PROBE 1 shell_script x -- true

# ===== R6-6 (mutation adequacy): reviewer mutants WF6-3 / WF6-5 / WF6-6 / WF6-4
# WF6-3: RED {a.sh pass, t.sh fail} and GREEN {t.sh pass, z.sh fail} hold the same two file contents in the same sorted order, so a directory
# digest without the relative names collides; the real digest differs
mkdir -p "$S/m/red" "$S/m/green"; P='#!/usr/bin/env bash\nexit 0\n'; F='#!/usr/bin/env bash\nexit 1\n'
printf "$P" >"$S/m/red/a.sh"; printf "$F" >"$S/m/red/t.sh"; printf "$P" >"$S/m/green/t.sh"; printf "$F" >"$S/m/green/z.sh"
fresh; "$EVREC" run CAT-001 RED 1 shell_script "$TGT" "${OR[@]}" -- "$S/d/dirrun.sh" "$S/m/red" >/dev/null 2>&1
"$EVREC" run CAT-001 GREEN 1 shell_script "$TGT" "${OR[@]}" -- "$S/d/dirrun.sh" "$S/m/green" >/dev/null 2>&1
[ "$(jq -r .test_fingerprint "$EV_LEDGER" 2>/dev/null | sort -u | wc -l | tr -d ' ')" = 2 ] && ok "R6-6: RED {a.sh pass, t.sh fail} and GREEN {t.sh pass, z.sh fail} get DIFFERENT directory fingerprints (relative names are hashed)" || bad "R6-6: name-swapped directories collide"
# WF6-5: a two-level symlink chain to an interpreter is still an interpreter
mkdir -p "$S/m/bin"; ln -s "$(command -v bash)" "$S/m/bin/mysh"; ln -s "$S/m/bin/mysh" "$S/m/bin/mysh2"
fresh; refuse "R6-6: RED through a TWO-level symlink chain to bash with -c is refused (interpreter_without_test_sources)" 69 interpreter_without_test_sources "$EVREC" run CAT-001 RED 1 shell_script "$TGT" "${OR[@]}" -- "$S/m/bin/mysh2" -c 'exit 1'
# WF6-6: MUTATION is refused over the cap too
mut=(--mutation-json '{"operator":"x","location":"y","author":"reviewer","result":"caught"}')
fresh; refuse "R6-6: a MUTATION record over the directory cap is refused (operand_directory_too_large)" 69 operand_directory_too_large env EVREC_OPERAND_DIR_MAX=3 "$EVREC" run CAT-001 MUTATION 1 shell_script "$TGT" "${OR[@]}" "${mut[@]}" -- "$S/d/dirrun.sh" "$S/c/fix4"
# WF6-4: a grant holder with an empty run id is never a holder (second layer, direct unit check)
r=$(EVREC_TURN_RUN_ID= python3 -B -c "
import sys; sys.dont_write_bytecode=True; sys.path.insert(0, '$EVD')
import evcore
print('holder' if evcore._is_holder({'run_id': ''}) else 'not-holder')")
[ "$r" = not-holder ] && ok "R6-6: _is_holder({run_id:''}) with EVREC_TURN_RUN_ID set empty is False (second layer pinned)" || bad "R6-6: _is_holder gave '$r'"

# ===== R6-7: no dead bound constant
if ! grep -q '_NAMEMAX' "$EVD/evcore.py"; then ok "R6-7: evcore.py holds no dead _NAMEMAX constant"; else bad "R6-7: _NAMEMAX is still defined (it is dead or the patterns must be built from it)"; fi

hermetic_repo
[ "$(hermetic_host_calls)" = 0 ] && ok "hermetic: no call reached the host entry" || bad "hermetic: $(hermetic_host_calls) call(s) reached the host entry"
echo "checks=$n failures=$fails"; [ "$fails" -eq 0 ]
