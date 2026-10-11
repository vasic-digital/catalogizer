#!/usr/bin/env bash
# WF-R6 mutation driver (review WP-04-T046, R-01 .. R-07): one logical edit per mutation applied to a COPY of the scripts, the test that is meant to catch it run against the copy.
# H = scripts/repo/tests/test_wp04j.sh with HS=<copy>/scripts/repo (and SCHEMA_FILE=<copy schema>); M = the matrix section "19 r6" of test_commit_push_all.sh with CPA_SRC=<copy>.
# CAUGHT = the test exits non-zero AND prints a FAIL line that contains the mutation's `expect` text (the INTENDED assertion failed, not any one; WF14 N8). An edit whose `old` text is absent from
# the copy is a driver error (never a silent "caught"). A control run of each test against an UNMUTATED copy must pass first. EQ- ids are EQUIVALENT mutants (stated reason), never counted as caught.
# --base <dir>   the tree the copy is made from (default: the working tree); `--base <pre-fix copy>` re-runs the R-02 equivalence evidence: on the PRE-fix push_recursive.sh the reviewer's mutant mB
#                and the mutant of the condition that mB's dead line hid both SURVIVE (the line is dead, the condition it duplicates was hidden by it).
# Usage: bash scripts/repo/tests/run_wp04j_mutations.sh [--jobs N] [--only id-prefix] [--base DIR] [--out FILE]      Exit 0 only when every non-equivalent mutant is caught
set -u
cd "$(git rev-parse --show-toplevel)" || exit 2
exec python3 -I - "$@" <<'PY'
import os, sys, shutil, subprocess, tempfile, hashlib, concurrent.futures as cf
root = os.getcwd(); jobs = 4; only = ''; base = root; out = ''
a = sys.argv[1:]
while a:
    if a[0] == '--jobs': jobs = int(a[1]); a = a[2:]
    elif a[0] == '--only': only = a[1]; a = a[2:]
    elif a[0] == '--base': base = os.path.abspath(a[1]); a = a[2:]
    elif a[0] == '--out': out = a[1]; a = a[2:]
    else: print('usage: run_wp04j_mutations.sh [--jobs N] [--only id-prefix] [--base DIR] [--out FILE]'); sys.exit(2)
R = 'scripts/repo/'; LS, PR, IM, CR, CPA = R+'lib_safe.sh', R+'push_recursive.sh', R+'integrate_merge.sh', R+'commit_recursive.sh', 'scripts/commit-push-all.sh'
SCH = 'specs/001-full-project-audit-remediation/contracts/review-verdict.schema.json'
H, M = 'H', 'M'
# (id, file, [(old, new)], tests, expect text of the intended failing assertion, equivalent-reason or '')
MUT = [
 # ---- R-01 the parser itself
 ('J01 lib: the parser reads git interpret-trailers (the old second model) instead of the message lines', LS, [("  printf '%s\\n' \"$msg\" | LC_ALL=C awk -v want=\"$3\"", "  printf '%s\\n' \"$msg\" | git interpret-trailers --parse 2>/dev/null | LC_ALL=C awk -v want=\"$3\"")], [H, M], 'divider', ''),
 ('J02 push: trailer() reads the Awaits-Review hold through git interpret-trailers (the review needle)', PR, [('  cpa_trailer "$@"; }', '  git -C "$1" log -1 --format=%B "$2" 2>/dev/null | git interpret-trailers --parse 2>/dev/null | awk -v k="$3" \'index($0,k": ")==1 {v=substr($0,length(k)+3)} END{print v}\'; }')], [H], 'divider line in the body', ''),
 ('J03 lib: a folded trailer line is accepted', LS, [(' || L[i] ~ /^[ \\t]/ || L[i] ~ /^---/', ' || L[i] ~ /^---/')], [H], 'folded', ''),
 ('J04 lib: a repeated Awaits-Review / CPA-Run / Deferred-Gates is accepted', LS, [('if (seen[k]++) { PST = 3; return }', 'seen[k]++')], [H], 'duplicate', ''),
 ('J05 lib: a reserved key before the block is accepted', LS, [('  for (i = 1; i < s; i++) if (reserved(L[i])) { PST = 3; return }\n', '')], [H], 'before the block', ''),
 ('J06 lib: a control character (a CR) inside the block is accepted', LS, [('if (L[i] ~ /[[:cntrl:]]/ || L[i] ~', 'if (L[i] ~')], [H], 'CR', ''),
 ('J07 lib: the block need not open with CPA-Run', LS, [('index(L[s], "CPA-Run: ") != 1', 'index(L[s], "CPA-Run: ") < 0')], [H], 'earlier paragraph', ''),
 ('J08 lib: a --- line inside the block is accepted', LS, [(' || L[i] ~ /^---/) { PST = 3; return }', ') { PST = 3; return }')], [H], 'divider line inside the block', ''),
 ('J09 lib: cpa_runid ignores the parser status', LS, [('id="$(cpa_trailer "$1" "$2" CPA-Run)" || return 1;', 'id="$(cpa_trailer "$1" "$2" CPA-Run)";')], [H], 'cpa_runid [', 'EQ'),
 ('J10 push: the malformed_trailers refusal is gone (the hold reader trusts what the predicate read)', PR, [('    if [ "$trc" != 0 ] || [ -z "$rid" ]; then', '    if false; then')], [H], 'malformed_trailers', ''),
 ('J11 push: a hold-reader status other than 0 is ignored (only an empty run id refuses)', PR, [('if [ "$trc" != 0 ] || [ -z "$rid" ]; then', 'if [ -z "$rid" ]; then')], [H], 'malformed_trailers', ''),
 # ---- R-01 the message check in front of the writers
 ('J12 lib: cpa_msg_check lets a --- line pass', LS, [('    /^---/ { print "message_has_patch_divider"; bad = 1; exit }\n', '')], [H, M], 'divider', ''),
 ('J13 lib: cpa_msg_check compares the reserved keys case-sensitively', LS, [('    { k = tolower($0) }\n', '    { k = $0 }\n')], [H], 'CPA-Run line', ''),
 ('J14 lib: cpa_msg_check lets a control character (a CR) pass', LS, [('    /[\\001-\\010\\013-\\037\\177]/ { print "control_character"; bad = 1; exit }\n', '')], [H, M], 'CR', ''),
 ('J15 lib: cpa_msg_check lets an indented reserved key pass', LS, [('k ~ /^[ \\t]*(cpa-run|awaits-review|deferred-gates|foreign-commit)[ \\t]*:/ { print "message_impersonates_trailer"', 'k ~ /^(cpa-run|awaits-review|deferred-gates|foreign-commit):/ { print "message_impersonates_trailer"')], [H], 'indented', ''),
 ('J16 launcher: S0 does not check the message', CPA, [('msgwhy="$(cpa_msg_check "$MSG")" || fail S0 20 usage_error "$msgwhy"\n', ':\n')], [M], '19.1', ''),
 ('J17 commit_recursive: the message is not checked', CR, [('mr="$(cpa_msg_check "$MSG")" || die unsafe_message "$mr"\n', '')], [H], 'commit_recursive refuses a message with a divider line', ''),
 ('J18 commit_recursive: the attribution is not checked', CR, [('if [ -n "$ATTR" ]; then mr="$(cpa_msg_check "$ATTR")" || die unsafe_attribution "$mr"; fi\n', '')], [H], 'attribution', ''),
 # ---- R-01 integrate_merge
 ('J19 merge: the held scan ignores a block it cannot read', IM, [('    vp="$(cpa_hold_state "$ROOT" "$c")" || { g merge --abort >/dev/null 2>&1; die malformed_trailers "$c: its trailer block cannot be read"; }', '    vp="$(cpa_hold_state "$ROOT" "$c")" || true')], [H], 'malformed', ''),
 ('J20 merge: the Foreign-Commit dedupe never finds a listed commit', IM, [('        [ "$frc" = 0 ] && continue\n', '')], [H], 'not named again', ''),
 ('J21 merge: the held scan reads no hold at all', IM, [('    [ -z "$vp" ] || held="${held:+$held,}local_held_commit"', '    :')], [H], 'the merge awaits its review', ''),
 # ---- R-02 the merge hold behind unheld commits
 ('J22 push: the merge-block condition is dropped from the target selection (only a hold at the first commit withholds everything)', PR, [('  elif [ "$mergeblock" = 1 ] || [ "$cut" = 0 ]; then', '  elif [ "$cut" = 0 ]; then')], [H], 'R-02', ''),
 ('J23 push: a merge commit is never counted as a merge block', PR, [('[ "$(printf \'%s\\n\' "$pl" | awk \'{print NF-1}\')" -le 1 ] || mergeblock=1', ':')], [H], 'R-02', ''),
 # ---- R-05
 ('J24 launcher: the missing approved owner list is not refused', CPA, [('[ -n "$OWNED" ] || fail S0 20 own_orgs_unverifiable', '[ -n "$OWNED" ] || :')], [M], '19.5', ''),
 # ---- R-06
 ('J25 lib: the config listing status is lost again (a process substitution)', LS, [('  if ! git -C "$d" config --local --list --name-only -z >"$cf" 2>/dev/null; then rm -f "$cf"; echo "$d: git config listing failed"; return 1; fi', '  git -C "$d" config --local --list --name-only -z >"$cf" 2>/dev/null || true')], [H, M], 'failing config listing', ''),
 ('J26 merge: the --resolution trial merge runs before the backup', IM, [('  write_backup\n  g merge --no-ff --no-commit "$rtip"', '  g merge --no-ff --no-commit "$rtip"')], [H], '9.2 backup existed before', ''),
 # ---- R-07
 ('J27 schema: the 11.4.235(D) finding layers are not in the enum', SCH, [('"source-defect",', ''), ('"test-instrumentation",', '')], [H], 'source-defect', ''),
 ('J28 schema: evidence_layer takes any value', SCH, [('"evidence_layer": {\n            "enum": [\n              "source",\n              "artifact",\n              "runtime"\n            ],', '"evidence_layer": {')], [H], 'unknown evidence_layer', ''),
 ('J29 schema: finding_layer takes any value', SCH, [('"finding_layer": {\n            "enum": [\n              "source",\n              "artifact",\n              "runtime",\n              "source-defect",\n              "test-instrumentation",\n              "process-doc"\n            ]\n          },', '"finding_layer": {},')], [H], 'unknown finding_layer', ''),
 # ---- PRE-fix equivalence evidence for R-02 (run with --only PRE): the dead line the fix removed is put back first; the reviewer's mutant mB of it and the mutant of the condition it hid then BOTH survive
 ('PRE-mB push: the dead line is restored, then the reviewer mutant mB replaces it by `false &&` -- EQUIVALENT: the first branch of the chain already decides', PR,
   [('  declare -A TGL=(); declare -A BGL=()', '  false && target="$(g rev-parse -q --verify "${U[0]}^" 2>/dev/null || true)"\n  declare -A TGL=(); declare -A BGL=()')], [H], 'R-02', 'EQ'),
 ('PRE-mC push: the dead line is restored, then the merge block is dropped from the chain -- SURVIVES while the dead line exists (the line hid the mutation)', PR,
   [('  declare -A TGL=(); declare -A BGL=()', '  [ "$mergeblock" = 1 ] && [ "$cut" -gt 0 ] && target="$(g rev-parse -q --verify "${U[0]}^" 2>/dev/null || true)"\n  declare -A TGL=(); declare -A BGL=()'),
    ('  elif [ "$mergeblock" = 1 ] || [ "$cut" = 0 ]; then', '  elif [ "$cut" = 0 ]; then')], [H], 'R-02', 'EQ'),
]
TREE = ['scripts/repo', 'scripts/audit', 'scripts/longops', 'scripts/anti-mess']
def mkcopy():
    d = tempfile.mkdtemp(prefix='wp04j_mut.')
    for sub in TREE:
        src = os.path.join(base, sub)
        if not os.path.isdir(src): continue
        dst = os.path.join(d, sub); os.makedirs(dst)
        for f in os.listdir(src):
            s = os.path.join(src, f)
            if os.path.isfile(s): shutil.copy2(s, dst)
            elif os.path.isdir(s) and f == 'host_entry': shutil.copytree(s, os.path.join(dst, f))
    shutil.copy2(os.path.join(base, CPA), os.path.join(d, 'scripts', 'commit-push-all.sh'))
    sd = os.path.join(d, os.path.dirname(SCH)); os.makedirs(sd)
    shutil.copy2(os.path.join(root, SCH), sd)       # the schema under test is the working tree's; a --base copy keeps the working one
    return d
def run_test(d, kind):
    env = dict(os.environ)
    if kind == H:
        env['HS'] = os.path.join(d, 'scripts/repo'); env['SCHEMA_FILE'] = os.path.join(d, SCH)
        cmd = ['bash', os.path.join(root, 'scripts/repo/tests/test_wp04j.sh')]
    else:
        env['CPA_SRC'] = d; env['CPA_ONLY'] = '19 r6'
        cmd = ['bash', os.path.join(root, 'scripts/repo/tests/test_commit_push_all.sh')]
    r = subprocess.run(cmd, cwd=root, env=env, capture_output=True, text=True)
    return r.returncode, [l for l in r.stdout.splitlines() if l.startswith('FAIL')]
def one(m):
    mid, f, pairs, tests, expect, eq = m
    d = mkcopy(); p = os.path.join(d, f); s = open(p).read()
    for old, new in pairs:
        if old not in s:
            shutil.rmtree(d, ignore_errors=True); return mid, 'ERROR', 'old text not found in %s: %r' % (f, old[:70]), eq
        s = s.replace(old, new, 1)
    open(p, 'w').write(s)
    caught = []; first = ''
    for t in tests:
        rc, fails = run_test(d, t)
        hit = [x for x in fails if expect in x]
        if rc != 0 and hit: caught.append(t); first = first or hit[0][:150]
    shutil.rmtree(d, ignore_errors=True)
    return mid, ('CAUGHT' if caught else 'SURVIVED'), (first or 'tests passed (or failed on other assertions)'), eq
rep = ['# WF-R6 mutations (host run: bash scripts/repo/tests/run_wp04j_mutations.sh; each mutation = one edit of a COPY; CAUGHT = the covering test fails on the intended assertion)',
       '# driver sha256 ' + hashlib.sha256(open(os.path.join(root, 'scripts/repo/tests/run_wp04j_mutations.sh'), 'rb').read()).hexdigest(),
       '# base tree ' + base]
for t in ('scripts/repo/tests/test_wp04j.sh', 'scripts/repo/tests/test_commit_push_all.sh'):
    rep.append('# test sha256 %s  %s' % (hashlib.sha256(open(os.path.join(root, t), 'rb').read()).hexdigest(), t))
bad = 0
ons = [x for x in only.split(',') if x]
sel = [m for m in MUT if (ons and any(m[0].startswith(x) for x in ons)) or (not ons and not m[0].startswith('PRE'))]
with cf.ThreadPoolExecutor(max_workers=jobs) as ex:
    ctl = {k: ex.submit(lambda k=k: run_test(mkcopy(), k)) for k in (H, M) if any(k in m[3] for m in sel)}
    for k, fu in ctl.items():
        rc, fails = fu.result(); line = 'CONTROL  %s against an unmutated copy of %s -> exit %d, %d failing case(s)' % (k, base, rc, len(fails)); rep.append(line); print(line, flush=True)
        if rc != 0: bad += 1
    for mid, st, info, eq in ex.map(one, sel):
        if st == 'ERROR': line = 'ERROR    %s -> %s' % (mid, info); bad += 1
        elif st == 'CAUGHT': line = 'CAUGHT   %s -> %s' % (mid, info)
        elif eq: line = 'EQUIVALENT %s -> survived as documented in the entry (a second layer makes the edit unobservable)' % mid
        else: line = 'SURVIVED %s -> %s' % (mid, info); bad += 1
        if st == 'CAUGHT' and eq: line = 'CAUGHT-BUT-DOCUMENTED-EQUIVALENT ' + line; bad += 1
        rep.append(line); print(line, flush=True)
rep.append('---- %d mutations listed, %d not caught, errored, or mis-documented' % (len(sel), bad))
if out: open(out, 'w').write('\n'.join(rep) + '\n')
sys.exit(1 if bad else 0)
PY
