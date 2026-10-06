#!/usr/bin/env bash
# WP-04 fix round 4 mutation driver: one logical edit per mutation, applied to a COPY of scripts/repo (plus scripts/audit/org_of.py,
# which integrate_ff_only reads) in a temp dir; test_wp04d.sh runs against the copy (R4=<copy>/scripts/repo); a mutation is CAUGHT when the
# test exits non-zero. An edit whose `old` text is absent from the copy is a driver error (never a silent "caught"). A control run
# against an UNMUTATED copy must pass first. EQ- ids are EQUIVALENT mutants: the guard they remove has a second layer, stated in the id.
# Usage: bash scripts/repo/tests/run_wp04d_mutations.sh [out-file] [id-prefix]     Never edits a tracked file (copies live in a temp dir).
set -u
cd "$(git rev-parse --show-toplevel)" || exit 2
OUT="${1:-/dev/stdout}"; ONLY="${2:-}"
exec python3 - "$OUT" "$ONLY" <<'PY'
import os, shutil, subprocess, sys, tempfile, hashlib
out = sys.argv[1]; only = sys.argv[2]; root = os.getcwd()
R = 'scripts/repo/'
LIB, CR, SC, VC, CRH, NC, FF, PR, IM = [R+n for n in ('lib_safe.sh','commit_recursive.sh','scope_check.sh','validate_cheap.sh','check_revision_headers.sh','check_no_ci.sh','integrate_ff_only.sh','push_recursive.sh','integrate_merge.sh')]
M = [
 # ---- B-1: symlinks in S2 / S3
 ('D1 S2: a symlink store path is judged by its target', SC, [('      if has_link "$p" || head_is_link "$p"; then refuse "$p" append_only; continue 2; fi', '      if false; then refuse "$p" append_only; continue 2; fi')]),
 ('D2 S2: a type change (HEAD holds a link) is not refused', SC, [('      if has_link "$p" || head_is_link "$p"; then refuse "$p" append_only; continue 2; fi', '      if has_link "$p"; then refuse "$p" append_only; continue 2; fi')]),
 ('D3 S2: only the last path component is checked for a link', SC, [('  for part in $1; do acc="$acc/$part"; [ -L "$acc" ] && return 0; done\n  return 1; }', '  [ -L "$ROOT/$1" ] && return 0\n  return 1; }')]),
 ('D4 S2: a symlink blob is judged by its target', SC, [('    if has_link "$p" || head_is_link "$p"; then refuse "$p" blob_name; continue; fi', '    if false; then refuse "$p" blob_name; continue; fi')]),
 ('D5 S3: a declared symlink in an evidence class is not a failure', VC, [("        if lcls in EVIDENCE_CLASSES: emit('fail', 'symlink'", "        if False: emit('fail', 'symlink'")]),
 ('D6 S3: the type change (HEAD link, regular file now) is not refused', VC, [('    if cls in EVIDENCE_CLASSES and head_is_link(path):', '    if False and head_is_link(path):')]),
 ('D7 S3: a declared symlink of another class is skipped silently', VC, [("        else: emit('symlink_not_judged', path)", "        else: pass")]),
 ('D8 revision header: a declared symlink is skipped silently', CRH, [("""  if [ "$lnk" = 1 ]; then [ "$MEASURE" = 1 ] || printf 'symlink_not_judged\\trevision_header\\t%s\\n' "$p"; continue; fi""", '  if [ "$lnk" = 1 ]; then continue; fi')]),
 # ---- I-1: an unreachable remote is never a remote that holds nothing
 ('E1 S1: every remote unreachable still judged', FF, [('[ "$NREM" -eq 0 ] || [ "$NFAIL" -lt "$NREM" ] || finish 11 blocked remote_unreachable', ':')]),
 ('E2 S1: an unreachable remote contributes no known tip', FF, [('if [ -n "$kt" ]; then ts="$ts $kt"; else cannot=1; fi; continue; fi', 'cannot=1; continue; fi')]),
 ('E3 S1: cannot-compute is refused 20 instead of 11', FF, [('[ "$CANNOT" = 0 ] || finish 11 blocked remote_unreachable', ':')]),
 ('E4 S1: an unreachable remote holds nothing (the old reading)', FF, [("""      kt="$(known_tip "$dir" "$r" "$br")"; if [ -n "$kt" ]; then ts="$ts $kt"; else cannot=1; fi; continue; fi""", """      continue; fi""")]),
 ('E5 S6: an unknown unreachable remote is refused 20', PR, [('    if [ "$UNK" = 1 ]; then\n      # an unreachable', '    if false; then\n      # an unreachable')]),
 ('E6 S6: an unreachable remote contributes no known tip', PR, [('if [ -n "$kt" ]; then KNOWN+=("$kt"); else UNK=1; fi; continue; fi', 'UNK=1; continue; fi')]),
 # ---- I-2: trusted tables
 ('T1 S3: the helper trusts its own working-tree tables in place', VC, [("    if adopt: return cur if cur is not None else b''", "    if True: return cur if cur is not None else b''")]),
 ('T2 S3: a working-tree table that differs from the committed one is accepted', VC, [('    if cur is not None and cur != h.stdout:', '    if False and cur != h.stdout:')]),
 ('EQ-T3 S3: an uncommitted helper table is accepted: EQUIVALENT, the second check (the working copy differs from the empty committed copy) refuses the same input', VC, [("    if h.returncode != 0:\n        die('class_table_unreviewed'", "    if False:\n        die('class_table_unreviewed'")]),
 ('T4 S3: --trusted-tables is ignored', VC, [('    if trusted is not None:', '    if False:')]),
 # ---- I-3: the reviewer mutants
 ('X1 merge: a deletion of an approved G-GATE path is not held', IM, [('if [ "${ln%%\t*}" = D ]; then [ -n "$want" ] && held="${held:+$held,}gate"; else', 'if [ "${ln%%\t*}" = D ]; then :; else')]),
 ('X2 the CPA run id is not validated (S1, S6 and the merge path)', LIB, [("safe_runid \"$id\" || return 1; printf '%s' \"$id\"; }", "printf '%s' \"$id\"; }")]),
 ('X6 S3: a check script that refuses with 20 is a failed check', VC, [("    if p.returncode == 20: die('check_refused', f'{name}: {p.stderr.strip()[:200]}')\n", '')]),
 ('X7 check_no_ci: a file below .github/workflows/<dir>/ is a pipeline', NC, [('          .github/workflows/*/*) continue ;;\n', '')]),
 ('X8 merge: only the first admission table is checked', IM, [('  for tb in "${TBL[@]}"; do\n    for c in $(g rev-list "HEAD..$t" -- "$tb"', '  for tb in "${TBL[0]}"; do\n    for c in $(g rev-list "HEAD..$t" -- "$tb"')]),
 ('X9 ff: a declared directory does not block an incoming change under it', FF, [(' case "$x" in "$c"/*) return 0 ;; esac;', ' :;')]),
 # ---- minors
 ('N1 S2: a directory that starts with a gitlink is a gitlink', SC, [('if ($2 == p && a[1] == "160000") f = 1', 'if (a[1] == "160000") f = 1')]),
 ('N2 S3: a directory that starts with a gitlink is a gitlink', VC, [("e.startswith(b'160000 ') and e.partition(b'\\t')[2] == p.encode()", "e.startswith(b'160000 ')")]),
 ('N3 S1: the owned-submodule guard judges the checked-out branch', FF, [("""    sb="$BRANCH"; git -C "$d" rev-parse -q --verify "refs/heads/$BRANCH" >/dev/null 2>&1 || { sb="$(git -C "$d" symbolic-ref --short -q HEAD)" || continue; }""", """    sb="$(git -C "$d" symbolic-ref --short -q HEAD)" || continue""")]),
 ('N4 S1: the CPA-Run predicate reads git trailers again', FF, [("""  local id; id="$(cpa_runid "$1" "$2")" || return 1""", """  local id; id="$(git -C "$1" log -1 --format='%(trailers:key=CPA-Run,valueonly)' "$2" 2>/dev/null | tr -d ' \\n')"; safe_runid "$id" || return 1""")]),
 ('N5 commit_recursive exports GIT_LITERAL_PATHSPECS into the hooks', CR, [('MF=""; trap', 'export GIT_LITERAL_PATHSPECS=1; MF=""; trap')]),
 ('N6 ff_only leaks GIT_LITERAL_PATHSPECS into the merge hooks', FF, [('git -C "$ROOT" merge --ff-only -q "$NEWEST"', 'g merge --ff-only -q "$NEWEST"')]),
 ('N7 push_recursive exports GIT_LITERAL_PATHSPECS into the pre-push hooks', PR, [('export GIT_ALLOW_PROTOCOL=file:ssh:git:https GIT_TERMINAL_PROMPT=0 ', 'export GIT_LITERAL_PATHSPECS=1 GIT_ALLOW_PROTOCOL=file:ssh:git:https GIT_TERMINAL_PROMPT=0 ')]),
 ('N8 ff_only: a missing --changeset-from is read as empty', FF, [('[ -z "$CSF" ] || [ -r "$CSF" ] || finish 20 refused changeset_unreadable\n', '')]),
 ('N9 ff_only: a missing --path-gates is read as empty', FF, [('[ -z "$GATES" ] || [ -r "$GATES" ] || finish 20 refused path_gates_unreadable\n', '')]),
 ('N10 merge: a missing --path-gates is read as empty', IM, [('[ -z "$GATES" ] || [ -r "$GATES" ] || die path_gates_unreadable', ': || die path_gates_unreadable')]),
 ('N11 push_recursive: no timeout on ls-remote and push', PR, [('tm() { timeout "$TMO" "$@"; }', 'tm() { "$@"; }')]),
 ('EQ-N12 commit_recursive: no TERM trap: EQUIVALENT on this bash, which runs the EXIT trap when SIGTERM ends the script; the TERM trap only makes that shell-version independent', CR, [("trap 'exit 143' TERM; trap 'exit 130' INT\n", '\n')]),
 ('EQ-N13 push_recursive: no TERM trap: EQUIVALENT on this bash (EXIT trap runs on SIGTERM); the TERM trap only makes that shell-version independent', PR, [("; trap 'exit 143' TERM; trap 'exit 130' INT\n", "\n")]),
 ('N14 validate_cheap: the table work directory is never removed', VC, [("; atexit.register(shutil.rmtree, work, True)", "")]),
 ('N15 validate_cheap: the registry directory is never removed', VC, [("; atexit.register(shutil.rmtree, regdir, True)", "")]),
 ('N16 validate_cheap: the fail line names the first five files given', VC, [("failing = [f for f in fl if f in msg] or fl[:5]", "failing = fl[:5]")]),
 ('N17 validate_cheap: two held rows for one path resolve silently', VC, [("        if c[0] in held and held[c[0]] != c[1]: die('held_row_duplicate'", "        if False: die('held_row_duplicate'")]),
 ('N18 validate_cheap: a held row naming no declared path is accepted', VC, [("        if c[0] not in declared: die('held_row_unmatched'", "        if False: die('held_row_unmatched'")]),
 ('N20 commit_recursive: the EXIT trap that removes the message files is gone', CR, [('''trap '[ -z "$MF" ] || rm -f "$MF" "$MF.err" "$MF.out"' EXIT; ''', '')]),
 ('N19 commit_recursive: two held rows for one path resolve silently', CR, [('    [ -z "${HV[$hp]+x}" ] || [ "${HV[$hp]}" = "$hv" ] || die held_row_duplicate', '    :')]),
]
def sh(c, **k): return subprocess.run(c, capture_output=True, text=True, **k)
rep = []; bad = 0
rep.append('# WP-04 fix round 4 mutations (host run: bash scripts/repo/tests/run_wp04d_mutations.sh; each mutation = one edit of a COPY; CAUGHT = test_wp04d.sh exits non-zero)')
rep.append('# driver sha256 ' + hashlib.sha256(open(os.path.join(root, 'scripts/repo/tests/run_wp04d_mutations.sh'), 'rb').read()).hexdigest())
rep.append('# test sha256 ' + hashlib.sha256(open(os.path.join(root, 'scripts/repo/tests/test_wp04d.sh'), 'rb').read()).hexdigest())
def copy_tree():
    d = tempfile.mkdtemp(prefix='wp04d_mut.')
    for sub in ('scripts/repo', 'scripts/audit'):
        dst = os.path.join(d, sub); os.makedirs(dst)
        for f in os.listdir(os.path.join(root, sub)):
            s = os.path.join(root, sub, f)
            if os.path.isfile(s) and (sub == 'scripts/repo' or f == 'org_of.py'): shutil.copy2(s, dst)
    return d
def run_test(d):
    env = dict(os.environ, R4=os.path.join(d, 'scripts/repo'))
    r = sh(['bash', os.path.join(root, 'scripts/repo/tests/test_wp04d.sh')], cwd=root, env=env)
    return r.returncode, [l for l in r.stdout.splitlines() if l.startswith('FAIL')]
if not only or only == 'control':
    d = copy_tree(); rc, fails = run_test(d)
    line = f'CONTROL  test_wp04d.sh against an unmutated copy -> exit {rc}, {len(fails)} failing case(s)'; rep.append(line); print(line, flush=True)
    if rc != 0: bad += 1
    shutil.rmtree(d, ignore_errors=True)
for mid, f, pairs in M:
    if only == 'control' or (only and not mid.startswith(only)): continue
    d = copy_tree(); p = os.path.join(d, f); s = open(p).read()
    for old, new in pairs:
        if old not in s:
            line = f'ERROR    {mid}  -> old text not found in {f}: {old[:70]!r}'; rep.append(line); print(line, flush=True); bad += 1; break
        s = s.replace(old, new, 1)
    else:
        open(p, 'w').write(s)
        rc, fails = run_test(d)
        if rc != 0: line = f'CAUGHT   {mid}  -> {len(fails)} failing case(s), e.g. {fails[0][:110] if fails else "exit " + str(rc)}'
        elif mid.startswith('EQ-'): line = f'EQUIVALENT {mid}  -> survived as documented (the guard it removes has a second layer)'
        else: line = f'SURVIVED {mid}  -> test passed against the mutated copy'; bad += 1
        rep.append(line); print(line, flush=True)
    shutil.rmtree(d, ignore_errors=True)
rep.append(f'---- {len(M)} mutations listed, {bad} not caught or errored')
if out != '/dev/stdout': open(out, 'w').write('\n'.join(rep) + '\n')
sys.exit(1 if bad else 0)
PY
