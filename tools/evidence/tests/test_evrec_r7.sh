#!/usr/bin/env bash
# T050 round 7 (independent review WF7-REVIEW-recorder-r6, findings W7-1 W7-2 W7-3 W7-4 W7-8 W7-10 and mutation-adequacy W7-5 W7-6):
# cases written BEFORE the code changes and run RED against the round-6 tree first ($EV/wp05/T050r7-red.txt). Every refusal check
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
mkdir -p "$S/d" "$S/tgt"; echo t >"$S/tgt/f"; TGT=$S/tgt
printf '#!/usr/bin/env bash\nexec bash "$1/t.sh"\n' >"$S/d/dirrun.sh"; chmod +x "$S/d/dirrun.sh"
printf '#!/usr/bin/env bash\nexit 1\n' >"$S/d/filerun.sh"; chmod +x "$S/d/filerun.sh"
printf '#!/usr/bin/env bash\nexit 0\n' >"$S/d/filepass.sh"; chmod +x "$S/d/filepass.sh"

# ===== W7-1: `--password VALUE` after an ANSI colour sequence or a name character is still redacted (round 6 stored it)
out=$(timeout 120 python3 -B -c "
import sys; sys.dont_write_bytecode=True; sys.path.insert(0, '$EVD')
import evcore
V=('Zq'+'7Rk'+'9Pw'+'2Lm').encode()
pw=b'pass'+b'word'; tk=b'to'+b'ken'; ak=b'api'+b'-key'; sc=b'sec'+b'ret'
E=b'\x1b'
prefixes={'ESC[1m (bold)':E+b'[1m', 'ESC[33m':E+b'[33m', 'ESC[0m':E+b'[0m', 'ESC[1;31m':E+b'[1;31m', 'ESC[m':E+b'[m',
          'ESC[38;5;208m':E+b'[38;5;208m', 'an alphanumeric char':b'a', 'letters and digits':b'abc9', 'a dash':b'x-', 'an underscore':b'x_', 'a long name':b'q'*300}
n=0
for pk,pre in prefixes.items():
    for fk,flag in (('password',pw),('token',tk),('api-key',ak),('secret',sc)):
        for sk,sep in (('space',b' '),('TAB',b'\t')):
            b=pre+b'--'+flag+sep+V
            out,hit=evcore.redact_bytes(b, [])
            print('ok' if V not in out else 'MISS', 'after %s: --%s<%s>V' % (pk, fk, sk))
# a SINGLE-dash flag with a very long name (only flag_pair covers it: flag_pair_inrun needs a `--` or `_-` marker)
for L in (128, 129, 5000):
    for k,b in {'single-dash flag, %d-char head':b'-'+b'h'*L+pw+b' '+V, 'single-dash flag, %d-char tail':b'-'+pw+b't'*L+b' '+V}.items():
        out,hit=evcore.redact_bytes(b, [])
        print('ok' if V not in out else 'MISS', k % L)
# the single dash after an underscore (round 5 redacted it, round 6 declared it a limit) and the double dash after a dash
for k,b in {'a_-password V':b'a_-'+pw+b' '+V, 'x--password V (dash run before the flag)':b'x---'+pw+b' '+V, '--password V alone':b'--'+pw+b' '+V,
            '-password V (single dash at the start)':b'-'+pw+b' '+V}.items():
    out,hit=evcore.redact_bytes(b, [])
    print('ok' if V not in out else 'MISS', k)
# negative controls: nothing credential-shaped, or no flag dashes, or a value that starts with a dash: left alone
def kept(b): out,hit=evcore.redact_bytes(b, []); return V in out
for k,b in {'reset-password link (a single dash after a letter, prose)':b'please use the reset-'+pw+b' '+V,
            'a flag with no credential word after a dash prefix':b'x--verbose '+V,
            'a credential flag whose value starts with a dash':b'a--'+pw+b' -'+V,
            'ESC[1m then a flag with no credential word':E+b'[1m--verbose '+V}.items():
    print('ok' if kept(b) else 'MISS', 'negative control, not redacted: '+k)
")
[ -n "$out" ] || bad "W7-1: the flag redaction check did not finish in 120 s"
while IFS= read -r l; do [ -n "$l" ] || continue; case "$l" in ok*) ok "W7-1: ${l#ok }";; *) bad "W7-1: ${l#MISS }";; esac; done <<<"$out"
# linear time on the shapes the new pattern could make quadratic (every case must finish in 10 s; a linear scan takes milliseconds)
tcase7() { # tcase7 NAME SIZE PYEXPR
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
  if [ "$r" = 0 ]; then ok "W7-1: redact_bytes on $name ($size bytes) returns in under 10 s ($(awk -v a="$t0" -v b="$t1" 'BEGIN{printf "%.2f", b-a}') s)"; else bad "W7-1: redact_bytes on $name ($size bytes) did not finish in 10 s (rc=$r)"; fi
}
for sz in 100000 200000 400000; do
  tcase7 "'x--' repeated"                       $sz 'b"x--"*(N//3)'
  tcase7 "'_-' repeated"                        $sz 'b"_-"*(N//2)'
  tcase7 "'a_-' repeated, a credential word at the end" $sz 'b"a_-"*(N//3)+b"password "'
  tcase7 "'--' repeated"                        $sz 'b"--"*(N//2)'
done
tcase7 "'x--pas' repeated (a credential-word prefix)" 400000 'b"x--pas"*(N//6)'
tcase7 "'x--passwor' repeated"                  400000 'b"x--passwor"*(N//10)'
tcase7 "'ESC[1m--password ' repeated"           400000 'b"\x1b[1m--password "*(N//15)'
tcase7 "'a--password ' repeated"                400000 'b"a--password "*(N//12)'
tcase7 "'x--token' repeated, no separator"      400000 'b"x--token"*(N//8)'
tcase7 "'-a' repeated after a first '--'"       400000 'b"--"+b"-a"*(N//2)'
tcase7 "'x-' repeated, a credential word at the end" 400000 'b"x-"*(N//2)+b"-password "'
tcase7 "'ESC[1m' repeated"                      400000 'b"\x1b[1m"*(N//4)'
tcase7 "'--a-' repeated, then 'token '"         400000 'b"--a-"*(N//4)+b"token "'
# end to end: a command that prints an ANSI-coloured `--password VALUE` leaves no value in the ledger or the blobs
fresh; mkdir -p "$S/w7e"
printf '#!/usr/bin/env bash\nV=%s%s%s\nprintf "\\033[1m--%s%s %%s\\033[0m\\n" "$V"\n' Zq 7Rk9Pw 2Lm pass word >"$S/w7e/ansi.sh"
"$EVREC" run CAT-001 PROBE 1 shell_script x -- bash "$S/w7e/ansi.sh" >/dev/null 2>&1; r=$?
if [ "$r" = 0 ] && ! grep -rqF Zq7Rk9Pw2Lm "$S/w" 2>/dev/null && [ "$(jq -r '.redacted' "$EV_LEDGER" 2>/dev/null | tail -1)" = true ] && "$VERIFY" >/dev/null 2>&1; then
  ok "W7-1: end to end: an ANSI-coloured --password value is in no ledger line and no blob, redacted=true, verify exits 0"
else bad "W7-1: end to end: rc=$r, the value reached the ledger/blobs, redacted is not true, or verify failed"; fi

# ===== W7-2: the byte cap counts the bytes ACTUALLY READ (a procfs file reports st_size 0)
if [ -r /proc/version ] && [ ! -s /proc/version ]; then
  mkdir -p "$S/p/dir"; printf '#!/usr/bin/env bash\nexit 1\n' >"$S/p/dir/t.sh"; ln -s /proc/version "$S/p/dir/ver"
  fresh; refuse "W7-2: a directory operand holding a file that reports st_size 0 but yields more bytes than the cap is refused on a RED" 69 operand_directory_too_large env EVREC_OPERAND_DIR_BYTES_MAX=100 "$EVREC" run CAT-001 RED 1 shell_script "$TGT" "${OR[@]}" -- "$S/d/dirrun.sh" "$S/p/dir"
  want "W7-2: the same directory under the byte cap is accepted" 0 env EVREC_OPERAND_DIR_BYTES_MAX=100000 "$EVREC" run CAT-001 RED 1 shell_script "$TGT" "${OR[@]}" -- "$S/d/dirrun.sh" "$S/p/dir"
  want "W7-2: the same directory over the byte cap on a PROBE is skipped, not refused" 0 env EVREC_OPERAND_DIR_BYTES_MAX=100 "$EVREC" run CAT-001 PROBE 1 shell_script x -- true "$S/p/dir"
else bad "W7-2: precondition missing: /proc/version is not a readable file that reports size 0 on this host (the read-time cap is not exercised)"; fi
# the READ-TIME count is cumulative over the directory too (two size-0 files, each under the cap, the sum over it)
if [ -r /proc/version ] && [ ! -s /proc/version ]; then
  vs=$(wc -c </proc/version); cap=$((vs*3/2))
  mkdir -p "$S/p/two" "$S/p/one"; printf '#!/usr/bin/env bash\nexit 1\n' >"$S/p/two/t.sh"; printf '#!/usr/bin/env bash\nexit 1\n' >"$S/p/one/t.sh"
  ln -s /proc/version "$S/p/two/v1"; ln -s /proc/version "$S/p/two/v2"; ln -s /proc/version "$S/p/one/v1"
  fresh; refuse "W7-2: two size-0 files that each fit under the cap but together exceed it are refused (the read-time count is cumulative)" 69 operand_directory_too_large env EVREC_OPERAND_DIR_BYTES_MAX=$cap "$EVREC" run CAT-001 RED 1 shell_script "$TGT" "${OR[@]}" -- "$S/d/dirrun.sh" "$S/p/two"
  want "W7-2: one such file under the same cap is accepted" 0 env EVREC_OPERAND_DIR_BYTES_MAX=$cap "$EVREC" run CAT-001 RED 1 shell_script "$TGT" "${OR[@]}" -- "$S/d/dirrun.sh" "$S/p/one"
fi
# a unit check of the read-time budget and the cumulative budget
out=$(timeout 60 python3 -B -c "
import sys, os, tempfile; sys.dont_write_bytecode=True; sys.path.insert(0, '$EVD')
import evcore
d=tempfile.mkdtemp(dir='$S'); p=os.path.join(d,'f'); open(p,'wb').write(b'x'*5000)
def raises(f, reason):
    try: f(); return False
    except evcore._OperandSkip as e: return e.reason==reason
print('ok' if raises(lambda: evcore._operand_file_sha(p, 1000), 'too_large') else 'MISS', 'a 5000-byte file with a budget of 1000 raises too_large while reading')
print('ok' if evcore._operand_file_sha(p, 5000)==evcore.file_sha(p) else 'MISS', 'a budget equal to the size hashes the file')
print('ok' if raises(lambda: evcore._operand_file_sha(p, 4999), 'too_large') else 'MISS', 'a budget one byte under the size raises too_large')
")
while IFS= read -r l; do [ -n "$l" ] || continue; case "$l" in ok*) ok "W7-2: ${l#ok }";; *) bad "W7-2: ${l#MISS }";; esac; done <<<"$out"
# W7-5 (mutation adequacy): the cap is CUMULATIVE over the directory, not per file
mkdir -p "$S/p/cum3" "$S/p/cum2"; printf '#!/usr/bin/env bash\nexit 1\n' >"$S/p/cum3/t.sh"; printf '#!/usr/bin/env bash\nexit 1\n' >"$S/p/cum2/t.sh"
for i in 1 2 3; do head -c 600 /dev/zero >"$S/p/cum3/f$i"; done; head -c 400 /dev/zero >"$S/p/cum2/f1"
fresh; refuse "W7-5: three 600-byte files with a 1000-byte cap (each under the cap, the sum over it) are refused on a RED" 69 operand_directory_too_large env EVREC_OPERAND_DIR_BYTES_MAX=1000 "$EVREC" run CAT-001 RED 1 shell_script "$TGT" "${OR[@]}" -- "$S/d/dirrun.sh" "$S/p/cum3"
want "W7-5: a directory whose total is under the 1000-byte cap is accepted" 0 env EVREC_OPERAND_DIR_BYTES_MAX=1000 "$EVREC" run CAT-001 RED 1 shell_script "$TGT" "${OR[@]}" -- "$S/d/dirrun.sh" "$S/p/cum2"

# the st_size fast path decides BEFORE any file is opened: mode-000 files (never openable) are refused as too large, not as unreadable
# (not run as root: permission bits do not bind root)
if [ "$(id -u)" != 0 ]; then
  mkdir -p "$S/p/fast1" "$S/p/fastsum"; printf '#!/usr/bin/env bash\nexit 1\n' >"$S/p/fast1/t.sh"; printf '#!/usr/bin/env bash\nexit 1\n' >"$S/p/fastsum/t.sh"
  truncate -s 4G "$S/p/fast1/big"; chmod 000 "$S/p/fast1/big"
  for i in 1 2 3; do head -c 600 /dev/zero >"$S/p/fastsum/f$i"; chmod 000 "$S/p/fastsum/f$i"; done
  fresh; refuse "W7-2: a 4 GiB mode-000 file in a directory operand is refused as too large by the st_size fast path (before any open)" 69 operand_directory_too_large "$EVREC" run CAT-001 RED 1 shell_script "$TGT" "${OR[@]}" -- "$S/d/dirrun.sh" "$S/p/fast1"
  fresh; refuse "W7-5: three mode-000 600-byte files with a 1000-byte cap are refused as too large by the CUMULATIVE st_size count (before any open)" 69 operand_directory_too_large env EVREC_OPERAND_DIR_BYTES_MAX=1000 "$EVREC" run CAT-001 RED 1 shell_script "$TGT" "${OR[@]}" -- "$S/d/dirrun.sh" "$S/p/fastsum"
  chmod 644 "$S/p/fast1/big" "$S/p/fastsum/"f*
else echo "skip W7-2/W7-5 fast-path cases: running as root"; fi

# ===== W7-3: a plain file operand has the same byte cap
mkdir -p "$S/f"; truncate -s 4G "$S/f/big"; head -c 2000 /dev/zero >"$S/f/two_k"; printf 'x\n' >"$S/f/small"
t0=$(date +%s)
fresh; refuse "W7-3: a 4 GiB plain file operand is refused by the DEFAULT byte cap on a RED (operand_file_too_large)" 69 operand_file_too_large "$EVREC" run CAT-001 RED 1 shell_script "$TGT" "${OR[@]}" -- "$S/d/filerun.sh" "$S/f/big"
refuse "W7-3: the same file through --input=FILE is refused too" 69 operand_file_too_large "$EVREC" run CAT-001 RED 1 shell_script "$TGT" "${OR[@]}" -- "$S/d/filerun.sh" "--input=$S/f/big"
t1=$(date +%s); [ $((t1-t0)) -le 10 ] && ok "W7-3: both refusals together took $((t1-t0)) s (under 10 s, the file was not hashed)" || bad "W7-3: the refusals took $((t1-t0)) s"
fresh; want "W7-3: the same plain operand on a PROBE is skipped, not refused" 0 "$EVREC" run CAT-001 PROBE 1 shell_script x -- true "$S/f/big"
refuse "W7-3: a 2000-byte plain operand with EVREC_OPERAND_DIR_BYTES_MAX=1000 is refused on a GREEN" 69 operand_file_too_large env EVREC_OPERAND_DIR_BYTES_MAX=1000 "$EVREC" run CAT-001 GREEN 1 shell_script "$TGT" "${OR[@]}" -- "$S/d/filepass.sh" "$S/f/two_k"
fresh; want "W7-3: a small plain operand under the cap still records on a RED" 0 "$EVREC" run CAT-001 RED 1 shell_script "$TGT" "${OR[@]}" -- "$S/d/filerun.sh" "$S/f/small"
if [ -r /proc/version ] && [ ! -s /proc/version ]; then
  fresh; refuse "W7-3: a plain procfs operand (st_size 0) over the cap is refused by the read-time count" 69 operand_file_too_large env EVREC_OPERAND_DIR_BYTES_MAX=10 "$EVREC" run CAT-001 RED 1 shell_script "$TGT" "${OR[@]}" -- "$S/d/filerun.sh" /proc/version
else bad "W7-3: precondition missing: /proc/version"; fi

# ===== W7-4: an operand that cannot be read is refused operand_unreadable, never silently dropped (not run as root: permission bits do not bind root)
if [ "$(id -u)" != 0 ]; then
  mkdir -p "$S/u"; printf 'secret-free\n' >"$S/u/mode000"; chmod 000 "$S/u/mode000"
  fresh; refuse "W7-4: a mode-000 plain file operand is refused on a RED (operand_unreadable, 69)" 69 operand_unreadable "$EVREC" run CAT-001 RED 1 shell_script "$TGT" "${OR[@]}" -- "$S/d/filerun.sh" "$S/u/mode000"
  want "W7-4: the same operand on a PROBE is skipped" 0 "$EVREC" run CAT-001 PROBE 1 shell_script x -- true "$S/u/mode000"
  mkdir -p "$S/u/dm"; printf '#!/usr/bin/env bash\nexit 1\n' >"$S/u/dm/t.sh"; printf 'zz\n' >"$S/u/dm/hidden"; chmod 000 "$S/u/dm/hidden"
  fresh; refuse "W7-4: a mode-000 file inside a directory operand is refused on a RED (operand_unreadable, 69)" 69 operand_unreadable "$EVREC" run CAT-001 RED 1 shell_script "$TGT" "${OR[@]}" -- "$S/d/dirrun.sh" "$S/u/dm"
  mkdir -p "$S/u/xo" "$S/u/xog"; printf '#!/usr/bin/env bash\nexit 1\n' >"$S/u/xo/t.sh"; printf '#!/usr/bin/env bash\nexit 0\n' >"$S/u/xog/t.sh"; chmod 311 "$S/u/xo" "$S/u/xog"
  fresh; refuse "W7-4: an execute-only directory operand (the runner can run t.sh by name, evrec cannot list it) is refused on a RED (operand_unreadable, 69)" 69 operand_unreadable "$EVREC" run CAT-001 RED 1 shell_script "$TGT" "${OR[@]}" -- "$S/d/dirrun.sh" "$S/u/xo"
  fresh; refuse "W7-4: the same execute-only directory on a GREEN is refused too" 69 operand_unreadable "$EVREC" run CAT-001 GREEN 1 shell_script "$TGT" "${OR[@]}" -- "$S/d/dirrun.sh" "$S/u/xog"
  want "W7-4: the execute-only directory on a PROBE is skipped" 0 "$EVREC" run CAT-001 PROBE 1 shell_script x -- true "$S/u/xo"
  chmod 755 "$S/u/xo" "$S/u/xog"
  mkdir -p "$S/u/tg/sub"; echo a >"$S/u/tg/a"; echo b >"$S/u/tg/sub/b"; chmod 000 "$S/u/tg/sub"
  fresh; refuse "W7-4: a TARGET directory with an unlistable sub-directory is refused (target_unreadable, 77), not fingerprinted without it" 77 target_unreadable "$EVREC" run CAT-001 RED 1 shell_script "$S/u/tg" "${OR[@]}" -- "$S/d/filerun.sh"
  chmod 755 "$S/u/tg/sub"
else echo "skip W7-4: running as root (permission-bit cases not exercised; /proc/self/mem is covered in round 6)"; fi

# ===== W7-8 / W7-5: the caps are strictly a positive ASCII integer; an empty value is the default
for var in EVREC_OPERAND_DIR_MAX EVREC_OPERAND_DIR_BYTES_MAX; do
  for v in " 5 " "5 " " 5" "1_000" "+5" $'\xd9\xa5' $'\xef\xbc\x91' "007" "0x10" "1e3" "5.0" "-0" "0"; do
    fresh; refuse "W7-8: $var=$(printf '%q' "$v") is a usage_error (64)" 64 usage_error env "$var=$v" "$EVREC" run CAT-001 PROBE 1 shell_script x -- true
  done
  fresh; want "W7-8: $var=5 records" 0 env "$var=5" "$EVREC" run CAT-001 PROBE 1 shell_script x -- true
  fresh; want "W7-8: $var=99999999999999999999 (a large positive integer) records" 0 env "$var=99999999999999999999" "$EVREC" run CAT-001 PROBE 1 shell_script x -- true
  fresh; want "W7-5: $var set to the empty string is the default (records)" 0 env "$var=" "$EVREC" run CAT-001 PROBE 1 shell_script x -- true
done

# ===== W7-10: pass 2 never blocks on a file that became a FIFO after pass 1 (opened O_NONBLOCK, fstat must say regular file)
mkdir -p "$S/q"; mkfifo "$S/q/pipe"; ln -s "$S/q/pipe" "$S/q/pipelink"; printf 'real\n' >"$S/q/reg"; ln -s "$S/q/reg" "$S/q/reglink"
out=$(timeout 40 python3 -B -c "
import sys, os; sys.dont_write_bytecode=True; sys.path.insert(0, '$EVD')
import evcore
def reason(p):
    try: evcore._operand_file_sha(p, 10**6); return 'hashed'
    except evcore._OperandSkip as e: return e.reason
print('ok' if reason('$S/q/pipe')=='unreadable' else 'MISS', 'a FIFO is refused as unreadable, not blocked on')
print('ok' if reason('$S/q/pipelink')=='unreadable' else 'MISS', 'a symlink to a FIFO is refused as unreadable, not blocked on')
try: same=evcore._operand_file_sha('$S/q/reglink', 10**6)==evcore.file_sha('$S/q/reg')
except evcore._OperandSkip: same=False
print('ok' if same else 'MISS', 'a symlink to a regular file is still hashed (the bytes of its target)')
# the swap between pass 1 and pass 2: pass 1 is made to accept the FIFO, as if it had been a regular file a moment earlier
orig=os.path.isfile
os.path.isfile=lambda p: True if os.path.basename(p)=='pipe' else orig(p)
try:
    evcore._dir_digest('$S/q', 100, 10**6); r='hashed'
except evcore._OperandSkip as e: r=e.reason
finally: os.path.isfile=orig
print('ok' if r=='unreadable' else 'MISS', 'a directory whose entry became a FIFO between the two passes is refused as unreadable, not blocked on')
"); rc=$?
if [ -z "$out" ]; then bad "W7-10: the FIFO check did not finish in 40 s (rc=$rc): pass 2 blocks on a FIFO"; fi
while IFS= read -r l; do [ -n "$l" ] || continue; case "$l" in ok*) ok "W7-10: ${l#ok }";; *) bad "W7-10: ${l#MISS }";; esac; done <<<"$out"

# ===== W7-5 / W7-6 (mutation adequacy)
# a JSON value that contains a space, under a 129-character and a 5000-character key (json_secret's unbounded name run on its own)
out=$(timeout 60 python3 -B -c "
import sys; sys.dont_write_bytecode=True; sys.path.insert(0, '$EVD')
import evcore
A=('Zq'+'7Rk').encode(); B=('9Pw'+'2Lm').encode(); tk=b'to'+b'ken'
for L in (128, 129, 5000):
    for k,b in {'double quotes, tail':b'{\"'+tk+b'y'*L+b'\": \"'+A+b' '+B+b'\"}', 'double quotes, head':b'{\"'+b'h'*L+tk+b'\": \"'+A+b' '+B+b'\"}',
                'single quotes, tail':b\"{'\"+tk+b'y'*L+b\"': '\"+A+b' '+B+b\"'}\"}.items():
        out,hit=evcore.redact_bytes(b, [])
        print('ok' if (A not in out and B not in out) else 'MISS', 'a spaced JSON value, %s, %d-char name run: no fragment left' % (k, L))
b=b'x '+tk+b'y'*129+b' not json and no separator'
out,hit=evcore.redact_bytes(b, [])
print('ok' if out==b else 'MISS', 'negative control: a long credential-word name with no value is untouched')
")
while IFS= read -r l; do [ -n "$l" ] || continue; case "$l" in ok*) ok "W7-5: ${l#ok }";; *) bad "W7-5: ${l#MISS }";; esac; done <<<"$out"
# a bare Bearer token outside an Authorization header (the pattern was unpinned: the author's mutant crashed every call instead of being caught)
out=$(timeout 30 python3 -B -c "
import sys; sys.dont_write_bytecode=True; sys.path.insert(0, '$EVD')
import evcore
V=('Zq'+'7Rk'+'9Pw'+'2Lm').encode()
for k,b in {'X-Api-Auth header':b'X-Api-Auth: Bearer '+V, 'prose':b'using Bearer '+V, 'a tab separator':b'Bearer\t'+V}.items():
    out,hit=evcore.redact_bytes(b, [])
    print('ok' if V not in out else 'MISS', 'a bare Bearer token, '+k)
")
while IFS= read -r l; do [ -n "$l" ] || continue; case "$l" in ok*) ok "W7-6: ${l#ok }";; *) bad "W7-6: ${l#MISS }";; esac; done <<<"$out"
# the mutation runner imports every mutated Python source: a regex that compiles as a file but cannot be imported is INVALID, not CAUGHT
# (not run inside a mutation scratch tree: run_mutations.py is not copied there, and the mutants edit evcore.py, not the runner)
if [ -f "$here/run_mutations.py" ]; then
out=$(timeout 120 python3 -B -c "
import sys, tempfile; sys.dont_write_bytecode=True; sys.path.insert(0, '$here')
import run_mutations as rm
old='re.compile(rb\"(?i)\\\\bBearer'
bad_m=('W7-6-selftest-unimportable', [(old, old.replace('rb\"(?i)', 'rb\"(?!x)x(?i)', 1))], ['x'], None)
good_m=('W7-6-selftest-importable', [(old, old.replace('rb\"(?i)', 'rb\"(?i)(?!x)x', 1))], ['x'], None)
base='$S'
r=rm.run_one(bad_m, base, True); print('ok' if r[1]=='INVALID' else 'MISS', 'a mutant that compiles but cannot be imported is INVALID (got %s)' % r[1])
r=rm.run_one(good_m, base, True); print('ok' if r[1]=='CAUGHT' else 'MISS', 'an importable mutant passes the --check stage (got %s)' % r[1])
")
[ -n "$out" ] || bad "W7-6: the runner import check did not finish in 120 s"
while IFS= read -r l; do [ -n "$l" ] || continue; case "$l" in ok*) ok "W7-6: ${l#ok }";; *) bad "W7-6: ${l#MISS }";; esac; done <<<"$out"
else echo "skip W7-6: run_mutations.py is not beside this test (a mutation scratch tree); the runner self-test runs in the real tree"; fi

hermetic_repo
[ "$(hermetic_host_calls)" = 0 ] && ok "hermetic: no call reached the host entry" || bad "hermetic: $(hermetic_host_calls) call(s) reached the host entry"
echo "checks=$n failures=$fails"; [ "$fails" -eq 0 ]
