#!/usr/bin/env bash
# WP-04 / WP-02 / WP-03 round 8 mutation driver (WF8 fixes, the code written in round 8 only): one logical edit per mutation applied to a COPY of
# scripts/repo and scripts/audit in a temp dir; the test that covers the edited file runs against the copy (VR= / SC= / PR= / H= / IH= point at the copy);
# a mutation is CAUGHT when the test exits non-zero. An edit whose `old` text is absent from the copy is a driver error (never a silent "caught"). A control
# run of every test against an UNMUTATED copy must pass first. EQ- ids are EQUIVALENT mutants (the guard they remove has a second layer, stated in the id).
# Usage: bash scripts/repo/tests/run_wp04h_mutations.sh [out-file] [id-prefix]     Never edits a tracked file (copies live in a temp dir).
set -u
cd "$(git rev-parse --show-toplevel)" || exit 2
OUT="${1:-/dev/stdout}"; ONLY="${2:-}"
exec python3 - "$OUT" "$ONLY" <<'PY'
import os, shutil, subprocess, sys, tempfile, hashlib
out = sys.argv[1]; only = sys.argv[2]; root = os.getcwd()
R, A = 'scripts/repo/', 'scripts/audit/'
PR, IM, FF, VR, ST = R+'push_recursive.sh', R+'integrate_merge.sh', R+'integrate_ff_only.sh', R+'verify_repos.sh', A+'scope_to_lumen_json.py'
# test group per mutated file: (test path, env var that points at the file under test; HS = the scripts/repo directory of the copy)
T = {PR: [(R+'tests/test_wp04h.sh', 'HS')], IM: [(R+'tests/test_wp04h.sh', 'HS')], FF: [(R+'tests/test_wp04h.sh', 'HS')], VR: [(R+'tests/test_wp04h.sh', 'HS')],
     ST: [(A+'tests/test_scope_to_lumen_json.sh', 'STJ')]}
M = [
 # ---- push_recursive (I-3)
 ('H1 PR: the submodule pin check is switched off (I-3)', PR, [('    if [ "${#TGL[@]}" -gt 0 ]; then\n      if [ -n "$t" ]; then', '    if false; then\n      if [ -n "$t" ]; then')]),
 ('H2 PR: every gitlink is checked, also one the remote tip already carries (I-3h)', PR, [('        [ "${BGL[$gp]-}" = "${TGL[$gp]}" ] && continue\n', '')]),
 ('H3 PR: a pin is held only by EQUALITY with a tip, a descendant tip does not hold it (I-3i)', PR, [('if [ "$s" = "$sha" ] || { git -C "$sd" cat-file -e "$s^{commit}" 2>/dev/null && git -C "$sd" merge-base --is-ancestor "$sha" "$s" 2>/dev/null; }; then held=1; break; fi', 'if [ "$s" = "$sha" ]; then held=1; break; fi')]),
 ('H4 PR: an unreachable submodule remote counts as holding the pin (I-3e)', PR, [('then PINWHY="remote $rr unreachable"; break; fi', 'then continue; fi')]),
 ('H5 PR: tags are not branch or tag tips of the submodule remote (I-3g)', PR, [('        case "$ref" in refs/heads/*|refs/tags/*) ;; *) continue ;; esac\n        if [ "$s" = "$sha" ]', '        case "$ref" in refs/heads/*) ;; *) continue ;; esac\n        if [ "$s" = "$sha" ]')]),
 ('H6 PR: a submodule with no remote counts as held (I-3j)', PR, [('  elif [ -z "$rl" ]; then PINWHY="submodule has no remote"', '  elif [ -z "$rl" ]; then :')]),
 ('H7 PR: an uninitialised submodule counts as held (I-3k)', PR, [('  if ! is_root "$sd"; then PINWHY="submodule not initialised"\n  elif', '  if ! is_root "$sd"; then :\n  elif')]),
 ('H8 PR: the withheld parent is not counted in the exit status (I-3b exit 11)', PR, [('      if [ "$nh" = 1 ]; then bump 11; continue; fi', '      if [ "$nh" = 1 ]; then continue; fi')]),
 ('H9 PR: a failing gitlink listing of the target is ignored (git_listing_failed)', PR, [('  if [ -n "$target" ] && ! gl_load "$dir" "$target" TGL; then', '  if false; then')]),
 # ---- integrate_ff_only (M-3, every git remote site)
 ('FF1 FF: vet_remotes reads a failing git remote as no remotes', FF, [('  rl="$(git -C "$d" remote 2>/dev/null)" || { VETREASON=git_listing_failed; return 1; }', '  rl="$(git -C "$d" remote 2>/dev/null)" || true')]),
 ('FF2 FF: the submodule fetch loop reads a failing git remote as no remotes', FF, [('  rl="$(git -C "$d" remote 2>/dev/null)" || finish 20 refused git_listing_failed   # a failing `git remote` is never "no remotes" (WF8)', '  rl="$(git -C "$d" remote 2>/dev/null)" || true')]),
 ('FF3 FF: the R4 pin loop reads a failing git remote as no remotes', FF, [('    rl="$(git -C "$d" remote 2>/dev/null)" || finish 20 refused git_listing_failed\n    for r in $rl; do\n      nrem', '    rl="$(git -C "$d" remote 2>/dev/null)" || true\n    for r in $rl; do\n      nrem')]),
 ('FF4 FF: unrec reads a failing git remote as no remotes', FF, [("rl=\"$(git -C \"$dir\" remote 2>/dev/null)\" || { echo '%git_listing_failed'; return 0; }", 'rl="$(git -C "$dir" remote 2>/dev/null)" || return 0')]),
 ('FF5 FF: the caller ignores the unrec listing-failed marker', FF, [("  case \"$bad\" in '%'*) finish 20 refused git_listing_failed ;; esac\n", '')]),
 ('FF7 FF: the main repository fetch loop reads a failing git remote as no remotes (the round-7 site, now second in line behind vet_remotes)', FF, [('REMS="$(g remote 2>/dev/null)" || finish 20 refused git_listing_failed', 'REMS="$(g remote 2>/dev/null)" || true')]),
 ('FF6 FF: the owned-submodule organisation loop reads a failing git remote as no remotes', FF, [('    rl="$(git -C "$d" remote 2>/dev/null)" || finish 20 refused git_listing_failed\n    for r in $rl; do u=', '    rl="$(git -C "$d" remote 2>/dev/null)" || true\n    for r in $rl; do u=')]),
 # ---- integrate_merge (INFO-1)
 ('IM1 IM: every remote unreachable is nothing_to_merge again (INFO-1)', IM, [('if [ "${#REMS[@]}" -gt 0 ] && [ "$NFAILR" -ge "${#REMS[@]}" ]; then', 'if false; then')]),
 ('IM2 IM: one unreachable remote is already remote_unreachable (golden-false: a reachable remote answered)', IM, [('[ "$NFAILR" -ge "${#REMS[@]}" ]', '[ "$NFAILR" -ge 1 ]')]),
 ('IM3 IM: a failed fetch (as opposed to a failed ls-remote) is not counted', IM, [('"refs/heads/$BR" >/dev/null 2>&1; then echo "fetch_failed:$r" >&2; NFAILR=$((NFAILR+1)); TIPOF+=(""); continue; fi', '"refs/heads/$BR" >/dev/null 2>&1; then echo "fetch_failed:$r" >&2; TIPOF+=(""); continue; fi')]),
 # ---- verify_repos (M-6)
 ('VM1 VR: the exit-20 cleanup is not installed (M-6)', VR, [('die20() { die20_run "$@"; }\n', '')]),
 ('VM2 VR: the cleanup does not remove a symlink (M-6)', VR, [('{ [ -f "$JSON_OUT" ] || [ -L "$JSON_OUT" ]; }', '[ -f "$JSON_OUT" ]')]),
 ('VM3 VR: the cleanup removes the file also for a bad argument (usage error before any run)', VR, [('    *) echo "unknown argument: $1" >&2; usage >&2; exit 20 ;;', '    *) echo "unknown argument: $1" >&2; [ -n "$JSON_OUT" ] && rm -f -- "$JSON_OUT"; usage >&2; exit 20 ;;')]),
 ('VM4 VR: --jobs misuse removes the named file too (posint before the run)', VR, [('posint() { case "$2" in \'\'|*[!0-9]*|0|0[0-9]*) die20 "$1 needs a positive integer, got \'$2\'" ;; esac; }', 'posint() { case "$2" in \'\'|*[!0-9]*|0|0[0-9]*) [ -n "$JSON_OUT" ] && rm -f -- "$JSON_OUT"; die20 "$1 needs a positive integer, got \'$2\'" ;; esac; }')]),
 # ---- scope_to_lumen_json (M-7, M-8)
 ('ST1 ST: the reserved class name third_party_nested is accepted (M-7)', ST, [('    if be is not None and "third_party_nested" in be:', '    if False:')]),
 ('ST2 ST: a duplicated or reordered classes list passes --check (M-8)', ST, [('                    if set(have[key][c]) == set(want[key][c]) and have[key][c] != want[key][c]:', '                    if False:')]),
]
def sh(c, **k): return subprocess.run(c, capture_output=True, text=True, **k)
rep = []; bad = 0
rep.append('# WP-04/WP-02/WP-03 round 8 mutations (host run: bash scripts/repo/tests/run_wp04h_mutations.sh; each mutation = one edit of a COPY; CAUGHT = the covering test exits non-zero)')
rep.append('# driver sha256 ' + hashlib.sha256(open(os.path.join(root, 'scripts/repo/tests/run_wp04h_mutations.sh'), 'rb').read()).hexdigest())
for t in sorted(set(x[0] for v in T.values() for x in v)):
    rep.append('# test sha256 %s  %s' % (hashlib.sha256(open(os.path.join(root, t), 'rb').read()).hexdigest(), t))
def copy_tree():
    d = tempfile.mkdtemp(prefix='wp04h_mut.')
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
