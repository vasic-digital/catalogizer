#!/usr/bin/env bash
# test_dotenv.sh - WF17 fix round 5, TI-D11. Drives the REAL scripts/test-infra/dotenv_get.py against a fixture env file and compares every value with python-dotenv itself (the oracle, DERIVED strategy 11.4.245: an
# independent implementation of the same dialect), plus SPECIFIED rows for the exit statuses (0 value, 1 unset, 2 usage / unreadable) and a no-leak row (the value appears on stdout only).
# Rows: V1 each supported form reads the same as python-dotenv (plain, spaces around =, export, double and single quotes, escapes, inline comment, CRLF file, duplicate last wins, empty value, `#` inside quotes)
# | V2 unset variable -> exit 1 and no output | V3 unreadable file -> exit 2 | V4 usage -> exit 2 | V5 control needle: the oracle sees a known key (a blind oracle would make V1 vacuous).
# Paired mutations: first-duplicate-wins; the escape decoding removed; the inline-comment strip removed; identity mutant (must SURVIVE).   Usage: test_dotenv.sh   (DOTENV_NO_MUTATIONS=1: tests only)   Env: DOTENV_SUT
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
DG="${DOTENV_SUT:-$TI_REPO/scripts/test-infra/dotenv_get.py}"
[ -f "$DG" ] && ok "script present: $DG" || { bad "script absent: $DG"; ti_summary; exit 1; }
python3 -c 'import dotenv' 2>/dev/null || { blocked "python-dotenv (the oracle) is not importable on this host"; ti_summary; exit 0; }
F="$TI_SCRATCH/fixture.env"
printf '%s\r\n' 'PLAIN=value1' 'SPACED = value2' 'export EXPORTED=value3' 'DQ="dq value"' "SQ='sq # not a comment'" 'ESC="a\nb"' 'INLINE=value4 # a comment' 'HASHQ="x # y"' 'DUP=first' 'DUP=second' 'EMPTY=' '# COMMENTED=no' >"$F"
oracle() { python3 -I - "$F" "$1" <<'PY'
import sys
from dotenv import dotenv_values
v = dotenv_values(sys.argv[1]).get(sys.argv[2])
sys.stdout.write("" if v is None else v)
sys.exit(1 if v is None else 0)
PY
}
for k in PLAIN SPACED EXPORTED DQ SQ ESC INLINE HASHQ DUP EMPTY COMMENTED; do
  want=$(oracle "$k"); wrc=$?; got=$(python3 -I "$DG" "$F" "$k"); grc=$?
  check "V1: $k reads the same as python-dotenv (value and set/unset)" "$got|$([ "$grc" = 0 ] && echo set || echo unset)" "$want|$([ "$wrc" = 0 ] && echo set || echo unset)"
done
check "V5: control: the oracle sees a known key" "$(oracle PLAIN)" "value1"
o=$(python3 -I "$DG" "$F" NO_SUCH_VARIABLE 2>&1); rc=$?; check "V2: an unset variable exits 1" "$rc" 1; check "V2: ... and prints nothing" "$o" ""
python3 -I "$DG" "$TI_SCRATCH/no-such.env" PLAIN >/dev/null 2>&1; check "V3: an unreadable file exits 2" "$?" 2
python3 -I "$DG" >/dev/null 2>&1; check "V4: no arguments exit 2" "$?" 2
python3 -I "$DG" "$F" 'bad name' >/dev/null 2>&1; check "V4: an invalid variable name exits 2" "$?" 2
if [ "${DOTENV_NO_MUTATIONS:-0}" != 1 ] && [ "${DOTENV_TEST_MUTANT:-0}" != 1 ]; then
  REC="${DOTENV_MUTATION_RECORD:-$TI_SCRATCH/mutations.txt}"; : >"$REC"
  dmut() { # dmut <name> <caught|survive> <old> <new>
    local name=$1 want=$2 d="$TI_SCRATCH/mut-$1" out rc; mkdir -p "$d"; cp "$DG" "$d/dotenv_get.py"
    python3 -I - "$d/dotenv_get.py" "$3" "$4" <<'PY' || { bad "mutation $name: anchor missing"; return; }
import sys
s = open(sys.argv[1]).read()
if s.count(sys.argv[2]) != 1: print("anchor count %d for %r" % (s.count(sys.argv[2]), sys.argv[2])); sys.exit(1)
open(sys.argv[1], "w").write(s.replace(sys.argv[2], sys.argv[3]))
PY
    out=$(DOTENV_SUT="$d/dotenv_get.py" DOTENV_TEST_MUTANT=1 DOTENV_NO_MUTATIONS=1 bash "${BASH_SOURCE[0]}" 2>&1); rc=$?
    if [ "$want" = survive ]; then if [ "$rc" -eq 0 ]; then ok "identity mutant $name SURVIVED (as required)"; echo "$name SURVIVED-AS-REQUIRED" >>"$REC"; else bad "identity mutant $name FAILED the suite"; echo "$name FAILED" >>"$REC"; fi
    elif [ "$rc" -ne 0 ]; then ok "mutation $name CAUGHT ($(printf '%s\n' "$out" | grep -m1 '^FAIL' | cut -c1-110))"; echo "$name CAUGHT" >>"$REC"
    else bad "mutation $name SURVIVED"; echo "$name SURVIVED" >>"$REC"; fi
  }
  dmut first_duplicate_wins caught '        if m and m.group(1) == argv[2]:
            value = parse_value(m.group(2))' '        if m and m.group(1) == argv[2] and value is None:
            value = parse_value(m.group(2))'
  dmut escapes_not_decoded caught 'out.append(ESC[rest[i + 1]]); i += 2; continue' 'out.append(c); i += 1; continue'
  dmut inline_comment_kept caught 'return re.sub(r"\s+#.*$", "", rest).rstrip()' 'return rest.rstrip()'
  dmut identity_noop survive 'value = None
    for raw' 'value = None; _x = 0
    for raw'
  [ -z "${DOTENV_EV:-}" ] || cp "$REC" "$DOTENV_EV/dotenv-mutations.txt"
fi
ti_summary
