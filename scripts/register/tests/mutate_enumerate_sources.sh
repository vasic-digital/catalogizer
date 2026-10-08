#!/usr/bin/env bash
# mutate_enumerate_sources.sh - paired mutations of enumerate_sources.py (T162/T165, WP-20). Each mutant is a sed edit of a
# scratch COPY of the enumerator; test_enumerate_planted.sh run against it (ENUMERATE_SOURCES) MUST FAIL, else the mutant
# SURVIVED and the run exits 1. The unmutated enumerator must pass first (control). Output: one line per mutant.
# Usage: mutate_enumerate_sources.sh   (env TMPDIR/REG_SCRATCH as the tests; ONLY=<mutant name> runs one mutant after the control)
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"; REG="$(dirname "$HERE")"
SRC="$REG/enumerate_sources.py"; W=$(mktemp -d "${TMPDIR:-/tmp}/mutenum.XXXXXX"); trap 'rm -rf "$W"' EXIT
cp "$REG/snapshot_manifest.py" "$REG/check_freeze_snapshot.sh" "$W/"
run() { ENUMERATE_SOURCES="$1" bash "$HERE/test_enumerate_planted.sh" >"$2" 2>&1; }
run "$SRC" "$W/control.txt"; c=$?
echo "CONTROL unmutated: rc=$c $(tail -1 "$W/control.txt")"; [ $c -eq 0 ] || { echo "control failed: no mutation verdict is meaningful"; exit 1; }
surv=0; n=0
# mutant NAME 'sed expression' 'expected-evidence regex (a FAIL line that must appear)'
mutant() { [ -z "${ONLY:-}" ] || [ "$ONLY" = "$1" ] || return 0; n=$((n+1)); local name=$1 expr=$2 want=$3 f="$W/m_$1.py"
  cp "$SRC" "$f"; sed -i -e "$expr" "$f"; if cmp -s "$f" "$SRC"; then echo "MUTANT $name: NOT APPLIED (sed matched nothing) - counted as survivor"; surv=$((surv+1)); return; fi
  cp "$f" "$W/enumerate_sources.py"; run "$W/enumerate_sources.py" "$W/out_$name.txt"; rc=$?
  if [ $rc -ne 0 ] && grep -Eq "$want" "$W/out_$name.txt"; then echo "MUTANT $name: KILLED ($(grep -c '^FAIL' "$W/out_$name.txt") failing checks; saw /$want/)"
  else echo "MUTANT $name: SURVIVED rc=$rc"; surv=$((surv+1)); fi; }
mutant skip_entry_check   's/if r.returncode != 0:/if False:/'                                   'moved snapshot'
mutant drop_s11_source    '/"S-11", "report_doc", "docs\/LANDMINES.md"/d'                        'planted S-11|baseline class S-11'
mutant drop_s25_source    '/"S-25", "report_doc", "S-25:SECURITY_KEY_ROTATION_REQUIRED.md"/d'     'planted S-25|baseline class S-25'
mutant checkbox_no_x      's/\\\[( |x|X)\\\]/\\[( )\\]/'                                         'planted S-10'
mutant insert_not_ignore  's/INSERT OR IGNORE INTO reg_sources/INSERT INTO reg_sources/'         'sql import|second import'
mutant marker_no_wordb    's/MARKER_RE = re.compile(r"\\b(TODO|FIXME|HACK|XXX)\\b")/MARKER_RE = re.compile(r"(TODO|FIXME|HACK|XXX)")/' 'decoy'
mutant skip_xit_carrier   's/(?<!\[\\w.\$\])(?:xit|xdescribe|xtest)/(?:xit|xdescribe|xtest)/'   'decoy'
mutant cross_mark_dropped 's/\[❌✗✘\]/[✗]/' 'planted S-0[789]|planted S-07'
mutant ticket_glob_deep   's/sel_glob("docs\/issues\/\*.md")/sel_glob("docs\/issues\/**")/'      'decoy'
mutant nondeterministic   's/for e in sorted(ents, key=lambda x: x.locator.encode("utf-8")):/for e in sorted(ents, key=lambda x: hash((x.locator, os.getpid()))):/' 'byte-identical sql'
echo "MUTANTS: $n run, $surv survived"
[ $surv -eq 0 ]
