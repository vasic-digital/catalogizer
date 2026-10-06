#!/usr/bin/env bash
# WP-04 mutation driver (T040, T040a, T040b, T041): for each mutation, applies one edit to a COPY of the helper (or of the
# reviewed tables) and runs the helper's test against the copy; the mutation is CAUGHT when the test exits non-zero.
# Usage  bash scripts/repo/tests/run_wp04_mutations.sh [out-file]     exit 0 only when every mutation was caught.
# Never  edits a tracked file: copies live in a temp dir.
set -u
cd "$(git rev-parse --show-toplevel)" || exit 2
OUT="${1:-/dev/stdout}"
exec python3 - "$OUT" <<'PY'
import os, shutil, subprocess, sys, tempfile
out = sys.argv[1]; root = os.getcwd()
IFF='scripts/repo/integrate_ff_only.sh'; RD='scripts/repo/record_deferral.sh'; CC='scripts/repo/check_class.sh'; FR='scripts/repo/fixture_roots.sh'
tI='scripts/repo/tests/test_integrate_ff_only.sh'; tR='scripts/repo/tests/test_record_deferral.sh'; tC='scripts/repo/tests/test_check_classes.sh'; tF='scripts/repo/tests/test_fixture_roots.sh'
# (id, helper, test, old, new)  -> edit of the helper copy
H = [
 ('R1 flag closed-set check removed', RD, tR, '[ "$ok" = 1 ] || die flag_not_in_closed_set', 'true || die flag_not_in_closed_set'),
 ('R2 reason requirement removed', RD, tR, '[ "$HAVE_REASON" = 1 ] && [ -n "$REASON" ] || die reason_required', 'true || die reason_required'),
 ('R3 tracked-files refusal removed', RD, tR, 'if [ -n "$(git -C "$top" ls-files -- "$RUN" 2>/dev/null)" ]; then die', 'if false; then die'),
 ('R4 not-ignored refusal removed', RD, tR, 'git -C "$top" check-ignore -q -- "$RUN/deferrals.tsv" || die', 'true || die'),
 ('R5b row write and read-back ignored', RD, tR, 'printf \'%s\\n\' "$row" >> "$F" || die write_failed "$F"', 'printf \'%s\\n\' "$row" >> "$F" 2>/dev/null; exit 0'),
 ('R6 awaits-review requirement removed', RD, tR, '[ -n "$COMMIT" ] && [ -z "$AWAIT" ]', '[ -n "" ] && [ -z "$AWAIT" ]'),
 ('R7 --list order not the closed-set order', RD, tR, 'out=""; for f in $CLOSED; do', 'out=""; for f in LOCAL_ONLY SWEEP_ABSENT SKIP_LONG; do'),
 ('R8 tab/newline refusal removed', RD, tR, "*$'\\t'*|*$'\\n'*) die field_invalid", "NEVERMATCH) die field_invalid"),
 ('I1 force/rewrite option refusal removed', IFF, tI, '--force*|-f|+*|--rebase|--reset|--hard|--merge) finish 20 refused force_refused', 'NEVER_MATCHES) finish 20 refused force_refused'),
 ('I2 declared-path ff block removed', IFF, tI, 'if [ -n "$hit" ]; then PATHS', 'if false; then PATHS'),
 ('I3 pin_not_on_remote refusal removed', IFF, tI, '    if [ -n "$lack" ]; then\n', '    if false; then\n'),
 ('I3b submodule_local_commits turned into pin_not_on_remote', IFF, tI, 'finish 12 blocked submodule_local_commits', 'finish 20 refused pin_not_on_remote'),
 ('I4 unrecorded_local_commit refusal removed', IFF, tI, 'if [ -n "$bad" ]; then COMMITS', 'if false; then COMMITS'),
 ('I4b trailer alone accepted (commits.tsv run row not required)', IFF, tI, '[ -f "$f" ] || return 1', '[ -f "$f" ] || return 0'),
 ('I4c commits.tsv row need not hold this sha', IFF, tI, 'END{exit !f}', 'END{exit 0}'),
 ('I5 G-GATE content check removed', IFF, tI, '[ "$got" = "$want" ] || GP="$GP $gp"', 'true'),
 ('I5b G-GATE deletion of approved path ignored', IFF, tI, '[ -n "$want" ] && GP="$GP $gp"', 'true'),
 ('I5c adoption first-parent rule removed', IFF, tI, 'if [ -n "$ADOPT" ] && !', 'if [ -n "" ] && !'),
 ('I5d merge-path routing not applied', IFF, tI, '[ -n "$RT" ] && finish 12 blocked merge_path_required', '[ -n "$RT" ] && true'),
 ('I6 diverged not detected', IFF, tI, '[ "$(rel "$NEWEST")" = div ] && finish 12 blocked diverged', 'true'),
 ('I7 remotes_diverged not detected', IFF, tI, '[ -n "$NEWEST" ] || finish 12 blocked remotes_diverged', 'true'),
 ('I8 fetch failure label changed', IFF, tI, 'fetch_failed:', 'failed:'),
 ('I9 helper also updates submodule work trees', IFF, tI, 'finish 0 fast_forwarded', 'g submodule update -q --init >/dev/null 2>&1; finish 0 fast_forwarded'),
 ('I10 behind submodule not reported', IFF, tI, 'st=needs_update', 'st=current'),
 ('I11 helper pushes the new tip to remotes', IFF, tI, 'finish 0 fast_forwarded', 'for r in $REMS; do g push -q "$r" "$NEWEST:refs/heads/$BRANCH" 2>/dev/null; done; finish 0 fast_forwarded'),
 ('I12 merge instead of fast-forward', IFF, tI, 'merge --ff-only -q', 'merge --no-ff -q -m x'),
 ('I13 ff blocked by dirty overlap reported as success', IFF, tI, 'echo "$mo" >&2; finish 12 blocked ff_blocked_by_local_changes', 'echo "$mo" >&2; finish 0 up_to_date'),
 ('C1 unknown path is not class source', CC, tC, "cls = m.pop() if m else 'source'", "cls = m.pop() if m else 'evidence'"),
 ('C2 glob row outranks exact path', CC, tC, 'if exact: cls = exact[0][1]', 'if False: cls = exact[0][1]'),
 ('C3 class_ambiguous not raised', CC, tC, "if len(m) > 1: die('class_ambiguous'", "if False: die('class_ambiguous'"),
 ('C4 check_unknown not raised', CC, tC, "if miss: die('check_unknown'", "if False: die('check_unknown'"),
 ('C5 fixture-root named check ignored', CC, tC, "if c in skipped and c != 'large_file': return 'skip'", "pass"),
 ('C6 ** does not match zero directories', CC, tC, "r += '(?:.*/)?'", "r += '(?:.*/)'"),
 ('C7 * crosses directories', CC, tC, "r += '[^/]*'", "r += '.*'"),
 ('C8 fixture root matches by name prefix', CC, tC, "path.startswith(root + '/')", "path.startswith(root)"),
 ('F1 root exempts every check', FR, tF, 'and check in names for r, names', 'and True for r, names'),
 ('F2 root matches by name prefix', FR, tF, "p.startswith(r + '/')", "p.startswith(r)"),
 ('F3 unknown check in a row accepted', FR, tF, "        if n not in CLOSED: die('check_unknown', f\"root {root} names {n}\")\n", ""),
 ('F4 path not normalised', FR, tF, 'p = os.path.normpath(p)\n    return', 'return'),
 ('F5 reason not required', FR, tF, "or not c[2].strip() or", "or"),
 ('F6 glob root accepted', FR, tF, "or any(ch in root for ch in '*?[')", ""),
]
# table mutations: the loader is the real one, the copy of the tables loses a row / flips an application
def drop_rows(cls):  return ('T drop exemption rows of class '+cls, 'ex', lambda t: ''.join(l for l in t.splitlines(True) if l.startswith('#') or l.split('\t')[1:2] != [cls]))
def flip(cls, chk): return (f'T class {cls} no longer applies {chk}', 'cl', lambda t: t.replace(f'{cls}\t{chk}\tyes', f'{cls}\t{chk}\tno'))
T = [drop_rows(c) for c in ('generated','evidence','patches','governance-carrier','evidence-ledger','legacy-collection')] + [
  flip('generated','secret_fold'), flip('evidence','secret_fold'), flip('patches','merge_conflict'), flip('governance-carrier','revision_header'),
  flip('evidence-ledger','private_key'), flip('legacy-collection','shell_parse'), flip('source','trailing_whitespace')]
res = []; fails = 0
def run(cmd, env):
    p = subprocess.run(cmd, cwd=root, env=env, capture_output=True, text=True, timeout=900)
    return p.returncode, [l for l in p.stdout.splitlines() if l.startswith('FAIL')]
import re
flt = os.environ.get('MUT_FILTER')
for name, helper, test, old, new in H:
    if flt and not re.search(flt, name): continue
    d = tempfile.mkdtemp(prefix='wp04mut.'); cp = os.path.join(d, os.path.basename(helper)); shutil.copy(helper, cp)
    src = open(cp).read()
    if old not in src: res.append(f'ERROR   {name}: pattern not found in {helper}'); fails += 1; continue
    open(cp, 'w').write(src.replace(old, new, 1)); os.chmod(cp, 0o755)
    env = dict(os.environ, H=cp, TBL=os.path.join(root, 'scripts/repo'))
    rc, fl = run(['bash', test], env)
    if rc == 0: res.append(f'SURVIVED {name}  (test still exits 0)'); fails += 1
    else: res.append(f'CAUGHT   {name}  -> {len(fl)} failing case(s), e.g. {fl[0][:110] if fl else "(non-zero exit, no FAIL line)"}')
    shutil.rmtree(d)
for name, which, fn in T:
    if flt and not re.search(flt, name): continue
    d = tempfile.mkdtemp(prefix='wp04mut.'); shutil.copytree(os.path.join(root, 'scripts/repo'), os.path.join(d, 'repo'), ignore=shutil.ignore_patterns('tests'))
    f = os.path.join(d, 'repo', 'check_exemptions.tsv' if which == 'ex' else 'check_classes.tsv'); t = open(f).read(); n = fn(t)
    if n == t: res.append(f'ERROR   {name}: mutation did not change the table'); fails += 1; continue
    open(f, 'w').write(n)
    env = dict(os.environ, TBL=os.path.join(d, 'repo'))
    rc, fl = run(['bash', tC], env)
    if rc == 0: res.append(f'SURVIVED {name}'); fails += 1
    else: res.append(f'CAUGHT   {name}  -> {len(fl)} failing case(s), e.g. {fl[0][:110] if fl else "(non-zero exit)"}')
    shutil.rmtree(d)
res.append(f'---- {len(res)} mutations run, {fails} not caught')
text = '\n'.join(res) + '\n'
if out == '/dev/stdout': sys.stdout.write(text)
else: open(out, 'w').write(text)
sys.exit(1 if fails else 0)
PY
