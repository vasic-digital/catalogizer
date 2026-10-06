#!/usr/bin/env bash
# WP-04 / WP-02 / WP-03 round 6 mutation driver (WF6 fixes): one logical edit per mutation of the ROUND-6 code, applied to a COPY of scripts/repo and
# scripts/audit in a temp dir; the test that covers the edited file runs against the copy (H= / VR= / IH= / STJ= point at the copy); a mutation is
# CAUGHT when the test exits non-zero. An edit whose `old` text is absent from the copy is a driver error (never a silent "caught"). A control run
# of every test against an UNMUTATED copy must pass first. EQ- ids are EQUIVALENT mutants (the guard they remove has a second layer, stated in the id).
# Usage: bash scripts/repo/tests/run_wp04f_mutations.sh [out-file] [id-prefix]     Never edits a tracked file (copies live in a temp dir).
set -u
cd "$(git rev-parse --show-toplevel)" || exit 2
OUT="${1:-/dev/stdout}"; ONLY="${2:-}"
exec python3 - "$OUT" "$ONLY" <<'PY'
import os, shutil, subprocess, sys, tempfile, hashlib
out = sys.argv[1]; only = sys.argv[2]; root = os.getcwd()
R, A = 'scripts/repo/', 'scripts/audit/'
PR, SC, NC, RH, IM, LIB = R+'push_recursive.sh', R+'scope_check.sh', R+'check_no_ci.sh', R+'check_revision_headers.sh', R+'integrate_merge.sh', R+'lib_safe.sh'
VR, IH, ST = R+'verify_repos.sh', A+'index_health.sh', A+'scope_to_lumen_json.py'
# test group per mutated file: (test path, env var that points at the file under test)
# (a mutation of scope_check is also run against test_wp04d.sh, which owns the symlink / type-change cases of the append-only store)
T = {PR: [(R+'tests/test_push_recursive.sh', 'H')], SC: [(R+'tests/test_scope_check.sh', 'H'), (R+'tests/test_wp04d.sh', 'R4')], NC: [(R+'tests/test_check_no_ci.sh', 'H')],
     RH: [(R+'tests/test_check_revision_headers.sh', 'H')], IM: [(R+'tests/test_integrate_merge.sh', 'H')], VR: [(R+'tests/test_verify_repos_r6.sh', 'VR'), (R+'tests/test_verify_repos_r7.sh', 'VR')],   # r7 owns the pin-held-by-a-tag and process-filter fixtures (WF7 MA-1..MA-3)
    
     IH: [(A+'tests/test_index_health_extra.sh', 'IH')], ST: [(A+'tests/test_scope_to_lumen_json.sh', 'STJ')]}
M = [
 # ---- push_recursive (W6-1 .. W6-4, W6-8, W6-9, W6-12)
 ('F1 PR: a failing rev-list is ignored (the old mapfile reading)', PR, [('  if ! ul="$(g rev-list --topo-order --reverse "refs/heads/$BR" ${KNOWN[@]+--not "${KNOWN[@]}"} -- 2>/dev/null)"; then', '  ul="$(g rev-list --topo-order --reverse "refs/heads/$BR" ${KNOWN[@]+--not "${KNOWN[@]}"} -- 2>/dev/null)" || true\n  if false; then')]),
 ('F2 PR: a moved remote with unrecorded commits is judged 20 again (W6-1)', PR, [('    if [ "$UNK" = 1 ] || [ "${#MOVED[@]}" -gt 0 ]; then', '    if [ "$UNK" = 1 ]; then')]),
 ('F3 PR: a failing gitlink listing under --recursive is read as no submodules', PR, [('|| { rm -f "$lf"; die git_listing_failed "$(printf \'%q\' "$nm") (gitlink listing)"; }', '|| { rm -f "$lf"; : > "$lf"; }')]),
 ('F4 PR: the nested listing refusal only at depth 0 (R1 of the review)', PR, [('|| { rm -f "$lf"; die git_listing_failed "$(printf \'%q\' "$nm") (gitlink listing)"; }', '|| { rm -f "$lf"; [ -n "$pre" ] && : > "$lf" || die git_listing_failed "$(printf \'%q\' "$nm") (gitlink listing)"; }')]),
 ('F5 PR: a failing git remote is read as no remotes', PR, [('  if ! rl="$(g remote 2>/dev/null)"; then', '  rl="$(g remote 2>/dev/null)" || true\n  if false; then')]),
 ('F6 PR: an unsafe gitlink path is skipped instead of refused (W6-4)', PR, [('    safe_relpath "$sp" || die unsafe_repo "gitlink $(printf \'%q\' "$sp")"', '    safe_relpath "$sp" || continue')]),
 ('F7 PR: MOVED is recorded only for a remote without a tracking ref (R3 of the review)', PR, [('      MOVED["$r"]=1; kt="$(known_tip "$dir" "$r" "$BR")"; if [ -n "$kt" ]; then KNOWN+=("$kt"); else UNK=1; fi', '      kt="$(known_tip "$dir" "$r" "$BR")"; if [ -n "$kt" ]; then KNOWN+=("$kt"); else MOVED["$r"]=1; UNK=1; fi')]),
 ('F8 PR: a failing parents listing of a held commit is ignored', PR, [('      if ! pl="$(g rev-list --parents -n1 "$c" 2>/dev/null)"; then', '      pl="$(g rev-list --parents -n1 "$c" 2>/dev/null)" || true\n      if false; then')]),
 ('F9 PR: the discover marker problem again: a path with a leading ? is judged a failure', PR, [('    DISC+=("$pre$sp"); discover "$dir/$sp" "$pre$sp/"', '    case "$sp" in \\?*) die git_listing_failed "${sp#\\?}" ;; esac\n    DISC+=("$pre$sp"); discover "$dir/$sp" "$pre$sp/"')]),
 # ---- scope_check (W6-6, W6-7, W6-8)
 ('G1 SC: a broken submodule .git is skipped silently', SC, [('git -C "$sub" rev-parse --git-dir >/dev/null 2>&1 || die submodule_git_unreadable "$(printf \'%q\' "$pre$sp") has a .git entry that git cannot read"', 'git -C "$sub" rev-parse --git-dir >/dev/null 2>&1 || continue')]),
 ('G2 SC: an unreadable HEAD tree is read as a new store', SC, [('|| die git_tree_unreadable "$(printf \'%q\' "$1"): HEAD\'s tree cannot be listed"', '|| return 0')]),
 ('G3 SC: no commit yet is read as an unreadable tree (false refusal)', SC, [('    if [ -e "$rp" ] || [ -s "$lg" ]; then die git_tree_unreadable', '    if true; then die git_tree_unreadable')]),
 ('G4 SC: the nested listing failure is judged only at depth 0 (R2 of the review)', SC, [('  gitlinks_ok "$dir" || die git_listing_failed "$(printf \'%q\' "${pre:-.}")"', '  gitlinks_ok "$dir" || [ -n "$pre" ] || die git_listing_failed "$(printf \'%q\' "${pre:-.}")"')]),
 ('G5 SC: a symlink store is not refused (HM link test dropped)', SC, [('      if has_link "$p" || [ "$HM" = 120000 ]; then refuse "$p" append_only; continue 2; fi', '      if has_link "$p"; then refuse "$p" append_only; continue 2; fi')]),
 # ---- temp-file traps (W6-5)
 ('H1 NC: no EXIT trap for the listing temp file', NC, [('trap \'rm -f "$lf"\' EXIT; trap \'exit 143\' TERM;', 'trap \'exit 143\' TERM;')]),
 ('EQ-H2 NC: no TERM trap (bash runs the EXIT trap itself on SIGTERM; the TERM trap only keeps the status explicit)', NC, [(' trap \'exit 143\' TERM; trap \'exit 130\' INT   # the listing temp file never', ' # the listing temp file never')]),
 ('H3 RH: no EXIT trap for the --measure listing temp file', RH, [('  trap \'rm -f "$lf"\' EXIT; trap \'exit 143\' TERM; trap \'exit 130\' INT', '  :')]),
 # ---- integrate_merge (W6-12)
 ('I1 IM: a failing git status reads as nothing dirty', IM, [('>"$W/status.z" 2>/dev/null || die git_status_failed "the working tree status cannot be read"', '>"$W/status.z" 2>/dev/null || true')]),
 ('I2 IM: a failing git diff reads as no overlap', IM, [('>"$W/diff.names" 2>/dev/null || die git_diff_failed "the changed paths of ${t:0:12} cannot be listed"', '>"$W/diff.names" 2>/dev/null || true')]),
 # ---- verify_repos (N6-1 .. N6-4, MR1, MR2)
 ('V1 VR: ls-remote tail matches count as branch or tag tips (N6-1)', VR, [('tref="${tl#*[[:space:]]}"; case "$tref" in refs/heads/*|refs/tags/*) ;; *) continue ;; esac', 'tref="${tl#*[[:space:]]}"')]),
 ('V2 VR: a tip that is not held locally is not undecided (N6-2)', VR, [('                  undecided=1; continue\n                fi\n', '                  continue\n                fi\n')]),
 ('EQ-V3 VR: a missing pin object is not undecided (classify already answers UNKNOWN-DIFFERENT for a pin object that is not held)', VR, [('$GIT -C "$abs" cat-file -e "${attpin}^{commit}" 2>/dev/null || { undecided=1; continue; }', '$GIT -C "$abs" cat-file -e "${attpin}^{commit}" 2>/dev/null || continue')]),
 ('V4 VR: the tip-equality shortcut is dropped (MR1)', VR, [('                if [ "$tt" = "$attpin" ]; then held=1; break; fi\n', '')]),
 ('V5 VR: undecided keeps the definite class', VR, [('                [ "$undecided" = 1 ] && cls_p=UNKNOWN-DIFFERENT\n', '')]),
 ('V6 VR: the dirty count failure is not fatal (MR2 family)', VR, [('|| die20 "exit map: the dirty count could not be read from the report"', '|| n_dirty=0')]),
 ('V7 VR: the diverged count failure is not fatal (MR2)', VR, [('|| die20 "exit map: the diverged count could not be read from the report"', '|| n_div=0')]),
 ('V8 VR: the pin count failure is not fatal', VR, [('|| die20 "exit map: the pin count could not be read from the report"', '|| n_pin=0')]),
 ('V9 VR: the behind count failure is not fatal', VR, [('|| die20 "exit map: the behind count could not be read from the report"', '|| n_beh=0')]),
 ('EQ-V10 VR: the report is written before the counts are read (N6-3; since round 8 the exit-20 cleanup of M-6 removes a file written before the failure, so the END state is identical and only the transient order differs, which no test can observe without timing)', VR, [('[ -n "$JSON_OUT" ] && { cp "$WORK/report.json" "$JSON_OUT" || die20 "cannot write $JSON_OUT"; }', ':'), ('cnt() { local v;', '[ -n "$JSON_OUT" ] && { cp "$WORK/report.json" "$JSON_OUT" || die20 "cannot write $JSON_OUT"; }\ncnt() { local v;')]),
 ('V11 VR: core.fsmonitor is not disabled on every command (N6-4)', VR, [(' -c core.fsmonitor=false -c credential.helper=', ' -c credential.helper=')]),
 ('V12 VR: no configured filter driver is switched off (N6-4)', VR, [('    export "GIT_CONFIG_KEY_$nfk=$fk" "GIT_CONFIG_VALUE_$nfk="; nfk=$((nfk+1))', '    :')]),
 ('V13 VR: the plain ls-remote of the branch keeps the repository uploadpack (N6-4)', VR, [('ls-remote --upload-pack=git-upload-pack -- "$r" "refs/heads/$rbranch"', 'ls-remote -- "$r" "refs/heads/$rbranch"')]),
 ('V14 VR: the fetch keeps the repository uploadpack (N6-4)', VR, [('                   --upload-pack=git-upload-pack --no-tags', '                   --no-tags')]),
 ('V15 VR: the default-branch ls-remote keeps the repository uploadpack (N6-4)', VR, [('ls-remote --symref --upload-pack=git-upload-pack -- "$r" HEAD', 'ls-remote --symref -- "$r" HEAD')]),
 # ---- index_health (N6-5, MR4)
 ('W1 IH: a replacement character in a root is accepted (invalid UTF-8)', IH, [('c == "\\\\" or c == "\\ufffd" or cat[0] == "C" or', 'c == "\\\\" or cat[0] == "C" or')]),
 ('W2 IH: a format character (BOM, zero-width space) is accepted', IH, [('c == "\\\\" or c == "\\ufffd" or cat[0] == "C" or', 'c == "\\\\" or c == "\\ufffd" or cat == "Cc" or')]),
 ('W3 IH: a backslash is accepted', IH, [('        if c == "\\\\" or c == "\\ufffd"', '        if c == "\\ufffd"')]),
 ('W4 IH: a decomposed (NFD) spelling is accepted', IH, [('    if unicodedata.normalize("NFC", r) != r and not root_hits(r):\n', '    if False:\n')]),
 ('W5 IH: a no-break space inside a root is accepted', IH, [(' or (cat[0] == "Z" and c != " ")', '')]),
 ('EQ-W6 IH: DEL not named (covered by the C* category check)', IH, [('or any(ord(c) < 32 or ord(c) == 127 for c in r):\n        return True\n    for c in r:', 'or any(ord(c) < 32 for c in r):\n        return True\n    for c in r:')]),
 # ---- WF7 reviewer mutants RM2 / RM3 / RM4 of the round-6 verifier code: they survived test_verify_repos_r6.sh, and RM4 only the 404-case matrix caught; now named
 # fast tests (test_verify_repos_r7.sh: undecided tip of a REMOTE-BEHIND pin, a filter.<n>.process driver, a pin held only by a remote tag) catch each
 ('V16 VR: undecided only for a DIVERGED pin class (reviewer mutant RM2, WF7 MA-1)', VR, [('[ "$undecided" = 1 ] && cls_p=UNKNOWN-DIFFERENT', '[ "$undecided" = 1 ] && [ "$cls_p" = DIVERGED ] && cls_p=UNKNOWN-DIFFERENT')]),
 ('V17 VR: process filters are not switched off (reviewer mutant RM3, WF7 MA-2)', VR, [("'^filter\\..*\\.(clean|smudge|process)$'", "'^filter\\..*\\.(clean|smudge)$'")]),
 ('V18 VR: tags are not branch or tag tips, a pin held only by a remote tag reads as not held (reviewer mutant RM4, WF7 MA-3)', VR, [('case "$tref" in refs/heads/*|refs/tags/*) ;; *) continue ;; esac', 'case "$tref" in refs/heads/*) ;; *) continue ;; esac')]),
 # ---- scope_to_lumen_json (N6-6)
 ('S1 ST: a non-string class name is accepted', ST, [('    if be is not None and any(not isinstance(k, str) for k in be):', '    if False:')]),
 ('S2 ST: a non-UTF-8 input raises (UnicodeDecodeError not caught)', ST, [('except (OSError, yaml.YAMLError, UnicodeDecodeError) as e:', 'except (OSError, yaml.YAMLError) as e:')]),
 ('S3 ST: root_files is compared without its type', ST, [('(key != "root_files" or type(have[key]) is type(want[key]))', 'True')]),
 ('S4 ST: the files are read with the locale encoding', ST, [('with open(a.submodules_tsv, encoding="utf-8") as fh:', 'with open(a.submodules_tsv) as fh:')]),
]
def sh(c, **k): return subprocess.run(c, capture_output=True, text=True, **k)
rep = []; bad = 0
rep.append('# WP-04/WP-02/WP-03 round 6 mutations (host run: bash scripts/repo/tests/run_wp04f_mutations.sh; each mutation = one edit of a COPY; CAUGHT = the covering test exits non-zero)')
rep.append('# driver sha256 ' + hashlib.sha256(open(os.path.join(root, 'scripts/repo/tests/run_wp04f_mutations.sh'), 'rb').read()).hexdigest())
for t in sorted(set(x[0] for v in T.values() for x in v)):
    rep.append('# test sha256 %s  %s' % (hashlib.sha256(open(os.path.join(root, t), 'rb').read()).hexdigest(), t))
def copy_tree():
    d = tempfile.mkdtemp(prefix='wp04f_mut.')
    for sub in ('scripts/repo', 'scripts/audit'):
        dst = os.path.join(d, sub); os.makedirs(dst)
        for f in os.listdir(os.path.join(root, sub)):
            s = os.path.join(root, sub, f)
            if os.path.isfile(s): shutil.copy2(s, dst)
    return d
def run_test(d, f):
    for test, var in T[f]:
        env = dict(os.environ)
        # test_wp04d.sh points at a repo directory (R4=<copy>/scripts/repo), the others at the single file under test
        env[var] = os.path.join(d, 'scripts/repo') if var == 'R4' else os.path.join(d, f)
        r = sh(['bash', os.path.join(root, test)], cwd=root, env=env)
        fails = [l for l in r.stdout.splitlines() if l.startswith('FAIL')]
        if r.returncode != 0: return r.returncode, fails
    return 0, []
# control runs, one per covered file
for f in T:
    if only and only != 'control' and not any(m[1] == f and m[0].startswith(only) for m in M): continue
    if only and only != 'control' and only.startswith(('V', 'W', 'S')) and False: continue
    d = copy_tree(); rc, fails = run_test(d, f)
    line = 'CONTROL  %s against an unmutated copy -> exit %d, %d failing case(s)' % (' + '.join(x[0] for x in T[f]), rc, len(fails)); rep.append(line); print(line, flush=True)
    if rc != 0: bad += 1
    shutil.rmtree(d, ignore_errors=True)
for mid, f, pairs in M:
    if only == 'control' or (only and not mid.startswith(only)): continue
    d = copy_tree(); p = os.path.join(d, f); s = open(p).read()
    for old, new in pairs:
        if old not in s:
            line = 'ERROR    %s  -> old text not found in %s: %r' % (mid, f, old[:70]); rep.append(line); print(line, flush=True); bad += 1; break
        s = s.replace(old, new, 1)
    else:
        open(p, 'w').write(s)
        rc, fails = run_test(d, f)
        if rc != 0: line = 'CAUGHT   %s  -> %d failing case(s), e.g. %s' % (mid, len(fails), fails[0][:110] if fails else 'exit %d' % rc)
        elif mid.startswith('EQ-'): line = 'EQUIVALENT %s  -> survived as documented (the guard it removes has a second layer)' % mid
        else: line = 'SURVIVED %s  -> test passed against the mutated copy' % mid; bad += 1
        rep.append(line); print(line, flush=True)
    shutil.rmtree(d, ignore_errors=True)
rep.append('---- %d mutations listed, %d not caught or errored' % (len(M), bad))
if out != '/dev/stdout': open(out, 'w').write('\n'.join(rep) + '\n')
sys.exit(1 if bad else 0)
PY
