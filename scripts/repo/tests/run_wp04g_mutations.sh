#!/usr/bin/env bash
# WP-04 / WP-02 / WP-03 round 7 mutation driver (WF7 fixes, the code written in round 7 only): one logical edit per mutation applied to a COPY of
# scripts/repo and scripts/audit in a temp dir; the test that covers the edited file runs against the copy (VR= / SC= / PR= / H= / IH= point at the copy);
# a mutation is CAUGHT when the test exits non-zero. An edit whose `old` text is absent from the copy is a driver error (never a silent "caught"). A control
# run of every test against an UNMUTATED copy must pass first. EQ- ids are EQUIVALENT mutants (the guard they remove has a second layer, stated in the id).
# Usage: bash scripts/repo/tests/run_wp04g_mutations.sh [out-file] [id-prefix]     Never edits a tracked file (copies live in a temp dir).
set -u
cd "$(git rev-parse --show-toplevel)" || exit 2
OUT="${1:-/dev/stdout}"; ONLY="${2:-}"
exec python3 - "$OUT" "$ONLY" <<'PY'
import os, shutil, subprocess, sys, tempfile, hashlib
out = sys.argv[1]; only = sys.argv[2]; root = os.getcwd()
R, A = 'scripts/repo/', 'scripts/audit/'
PR, SC, IM, FF = R+'push_recursive.sh', R+'scope_check.sh', R+'integrate_merge.sh', R+'integrate_ff_only.sh'
VR, IH = R+'verify_repos.sh', A+'index_health.sh'
# test group per mutated file: (test path, env var that points at the file under test)
T = {PR: [(R+'tests/test_wp04g.sh', 'PR')], SC: [(R+'tests/test_wp04g.sh', 'SC')], IM: [(R+'tests/test_integrate_merge.sh', 'H')], FF: [(R+'tests/test_integrate_ff_only.sh', 'H'), (R+'tests/test_wp04h.sh', 'HS')],   # round 8: vet_remotes now refuses a PERSISTENT failing `git remote` first, so M2 is only caught by the counter-shim cases of test_wp04h.sh (the k-th call fails)
    
     VR: [(R+'tests/test_verify_repos_r7.sh', 'VR')], IH: [(A+'tests/test_index_health_extra.sh', 'IH')]}
M = [
 # ---- verify_repos (I-1, I-2, M-5, reviewer mutants RM2 RM3 RM4)
 ('J1 VR: a required filter driver is not switched to required=false (I-1)', VR, [('export "GIT_CONFIG_KEY_$nfk=filter.$fnm.required" "GIT_CONFIG_VALUE_$nfk=false"; nfk=$((nfk+1))', ':')]),
 ('J2 VR: the tag object line is not decided by its peeled twin (I-2)', VR, [('((n[i] "^{}") in r)', '0')]),
 ('J3 VR: a held blob or tree tag is read as undecided (I-2)', VR, [('blob|tree) continue ;; esac', 'blob|tree) ;; esac')]),
 ('J4 VR: core.alternateRefsCommand is not neutralised (M-5)', VR, [(' -c core.alternateRefsCommand=true', '')]),
 ('J5 VR: the driver name is cut at its first dot (a dotted driver name keeps required=true)', VR, [('fnm="${fnm%.*}"', 'fnm="${fnm%%.*}"')]),
 ('J6 VR: undecided only for a DIVERGED pin class (reviewer mutant RM2)', VR, [('[ "$undecided" = 1 ] && cls_p=UNKNOWN-DIFFERENT', '[ "$undecided" = 1 ] && [ "$cls_p" = DIVERGED ] && cls_p=UNKNOWN-DIFFERENT')]),
 ('J7 VR: process filters are not switched off (reviewer mutant RM3)', VR, [("'^filter\\..*\\.(clean|smudge|process)$'", "'^filter\\..*\\.(clean|smudge)$'")]),
 ('J8 VR: tags are not branch or tag tips (reviewer mutant RM4)', VR, [('case "$tref" in refs/heads/*|refs/tags/*) ;; *) continue ;; esac', 'case "$tref" in refs/heads/*) ;; *) continue ;; esac')]),
 # ---- scope_check (I-3, M-2)
 ('K1 SC: a .git that git walks past to the parent is skipped again (I-3)', SC, [('|| die submodule_git_unreadable "$(printf \'%q\' "$pre$sp") has a .git entry that is no repository of its own (git resolves it to another work tree)"', '|| continue')]),
 ('K2 SC: an unresolvable HEAD is always read as no commit yet (M-2)', SC, [('    if [ -e "$rp" ] || [ -s "$lg" ]; then die git_tree_unreadable', '    if false; then die git_tree_unreadable')]),
 ('K3 SC: the HEAD reflog is not consulted (M-2: an emptied packed-refs)', SC, [(' || [ -s "$lg" ]; then die git_tree_unreadable', '; then die git_tree_unreadable')]),
 ('EQ-K4 SC: the loose ref file is not consulted (the HEAD reflog of every repository with history decides the same cases; they differ only where reflogs are disabled)', SC, [('    if [ -e "$rp" ] || [ -s "$lg" ]; then die git_tree_unreadable', '    if [ -s "$lg" ]; then die git_tree_unreadable')]),
 ('K5 SC: a detached HEAD that does not resolve is read as an unborn branch (M-2)', SC, [('hb="$(git -C "$ROOT" symbolic-ref -q HEAD 2>/dev/null)" || die git_tree_unreadable "HEAD does not resolve and is not an unborn branch"', 'hb="$(git -C "$ROOT" symbolic-ref -q HEAD 2>/dev/null)" || return 0')]),
 # ---- push_recursive (I-4)
 ('L1 PR: a broken submodule is skipped by discover again (I-4)', PR, [('die submodule_git_unreadable "$(printf \'%q\' "$pre$sp") has a .git entry that is no repository of its own"; fi', ':; fi')]),
 ('L2 PR: a dangling .git symlink is not a .git entry (I-4)', PR, [('if [ -e "$dir/$sp/.git" ] || [ -L "$dir/$sp/.git" ]; then die', 'if [ -e "$dir/$sp/.git" ]; then die')]),
 # ---- integrate_merge / integrate_ff_only (M-3)
 ('M1 IM: a failing git remote reads as no remotes (M-3)', IM, [('rl="$(g remote 2>/dev/null)" || die git_listing_failed "remote list"', 'rl="$(g remote 2>/dev/null)" || true')]),
 ('M2 FF: a failing git remote reads as no remotes (M-3)', FF, [('REMS="$(g remote 2>/dev/null)" || finish 20 refused git_listing_failed', 'REMS="$(g remote 2>/dev/null)" || true')]),
 # ---- index_health (M-6)
 ('N1 IH: a case variant of a real third-party root is accepted (M-6a)', IH, [('case_bad = [r for r in roots if root_case_mismatch(r)]', 'case_bad = []')]),
 ('N2 IH: an NFD root naming an indexed NFD path is refused again (M-6b)', IH, [('    if unicodedata.normalize("NFC", r) != r and not root_hits(r):\n', '    if unicodedata.normalize("NFC", r) != r:\n')]),
 ('N3 IH: an exact root also counts as a case mismatch (M-6a false refusal)', IH, [('    if root_hits(r) or not indexed:\n        return False\n    ql', '    if not indexed:\n        return False\n    ql')]),
]
def sh(c, **k): return subprocess.run(c, capture_output=True, text=True, **k)
rep = []; bad = 0
rep.append('# WP-04/WP-02/WP-03 round 7 mutations (host run: bash scripts/repo/tests/run_wp04g_mutations.sh; each mutation = one edit of a COPY; CAUGHT = the covering test exits non-zero)')
rep.append('# driver sha256 ' + hashlib.sha256(open(os.path.join(root, 'scripts/repo/tests/run_wp04g_mutations.sh'), 'rb').read()).hexdigest())
for t in sorted(set(x[0] for v in T.values() for x in v)):
    rep.append('# test sha256 %s  %s' % (hashlib.sha256(open(os.path.join(root, t), 'rb').read()).hexdigest(), t))
def copy_tree():
    d = tempfile.mkdtemp(prefix='wp04g_mut.')
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
        env[var] = os.path.join(d, 'scripts/repo') if var in ('R4', 'HS') else os.path.join(d, f)
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
