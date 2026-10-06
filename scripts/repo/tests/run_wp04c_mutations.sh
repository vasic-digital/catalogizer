#!/usr/bin/env bash
# WP-04 fix round 3 mutation driver: one logical edit per mutation, applied to a COPY of scripts/repo (plus scripts/audit/org_of.py,
# which integrate_ff_only reads) in a temp dir; test_wp04c.sh runs against the copy (R3=<copy>/scripts/repo); a mutation is CAUGHT when the
# test exits non-zero. An edit whose `old` text is absent from the copy is a driver error (never a silent "caught"). A control run
# against an UNMUTATED copy must pass first. EQ- ids are EQUIVALENT mutants: the guard they remove has a second layer, stated in the id.
# Usage: bash scripts/repo/tests/run_wp04c_mutations.sh [out-file] [id-prefix]     Never edits a tracked file (copies live in a temp dir).
set -u
cd "$(git rev-parse --show-toplevel)" || exit 2
OUT="${1:-/dev/stdout}"; ONLY="${2:-}"
exec python3 - "$OUT" "$ONLY" <<'PY'
import os, shutil, subprocess, sys, tempfile, hashlib
out = sys.argv[1]; only = sys.argv[2]; root = os.getcwd()
R = 'scripts/repo/'
LIB, CR, SC, VC, CC, RD, FF, PR, IM = [R+n for n in ('lib_safe.sh','commit_recursive.sh','scope_check.sh','validate_cheap.sh','check_class.sh','record_deferral.sh','integrate_ff_only.sh','push_recursive.sh','integrate_merge.sh')]
VCT = R+'validate_checks.tsv'
DECL = ".|./*|*/./*|*/.|*/|:*|*'*'*|*'?'*|*'['*) return 1 ;; esac"
M = [
 # ---- lib_safe
 ('L1 safe_declpath accepts the dot', LIB, [(DECL, DECL.replace('.|./*','./*',1))]),
 ('L2 safe_declpath accepts a ./ prefix', LIB, [(DECL, DECL.replace('./*|','',1))]),
 ('L3 safe_declpath accepts a /./ component', LIB, [(DECL, DECL.replace('*/./*|','',1))]),
 ('L4 safe_declpath accepts a trailing /.', LIB, [(DECL, DECL.replace('*/.|','',1))]),
 ('L5 safe_declpath accepts a trailing slash', LIB, [(DECL, DECL.replace('*/|','',1))]),
 ('L6 safe_declpath accepts a leading colon', LIB, [(DECL, DECL.replace(':*|','',1))]),
 ('L7 safe_declpath accepts *', LIB, [(DECL, DECL.replace("*'*'*|",'',1))]),
 ('L8 safe_declpath accepts ?', LIB, [(DECL, DECL.replace("*'?'*|",'',1))]),
 ('L9 safe_declpath accepts [', LIB, [(DECL, DECL.replace("|*'['*",'',1))]),
 ('L10 safe_declpath is safe_relpath only', LIB, [('  safe_relpath "$1" || return 1\n  case "$1" in .|', '  safe_relpath "$1" || return 1\n  return 0\n  case "$1" in .|')]),
 ('L11 lr_exact takes the first tail match', LIB, [('$2==ref {print $1; exit}', '$2 ~ (ref "$") {print $1; exit}')]),
 # ---- commit_recursive
 ('C1 declared paths judged by safe_relpath', CR, [('for p in "${PATHS[@]}"; do safe_declpath "$p" || die unsafe_path', 'for p in "${PATHS[@]}"; do safe_relpath "$p" || die unsafe_path')]),
 ('C2 held rows judged by safe_relpath', CR, [('safe_declpath "$hp" && safe_declpath "${hv:-}" || die unsafe_held_row', 'safe_relpath "$hp" && safe_relpath "${hv:-}" || die unsafe_held_row')]),
 ('C3 held row naming no declared path accepted', CR, [('[ -n "${DECL[$hp]+x}" ] || die held_row_unmatched', 'true')]),
 ('C4 a declared directory accepted', CR, [('[ "$isg" = 1 ] || die declared_directory', 'true')]),
 ('C5 a gitlink refused as a directory', CR, [('isg=0; for g in "${GL[@]+"${GL[@]}"}"; do [ "$p" = "$g" ] && isg=1; done', 'isg=0')]),
 ('EQ-C6 the :(literal) pathspec magic dropped: EQUIVALENT, the validator already refuses every pathspec magic', CR, [('lp+=(":(literal)$x")', 'lp+=("$x")')]),
 # ---- scope_check
 ('S1 declared paths judged by safe_relpath', SC, [('safe_declpath "$l" || die unsafe_path', 'safe_relpath "$l" || die unsafe_path')]),
 ('S2 --ev judged by safe_relpath', SC, [('safe_declpath "${2%/}" || die unsafe_path', 'safe_relpath "$2" || die unsafe_path')]),
 ('S3 a declared directory accepted', SC, [('    die declared_directory "$(printf \'%q\' "$l") is a directory: declare its files"', '    :')]),
 ('S4 a gitlink refused as a directory', SC, [("END { print f + 0 }')\" != 1 ]", "END { print f + 0 }')\" != 7 ]")]),
 ('S5 uninitialised submodule walks to the parent (W3)', SC, [('    [ "$(cd "$sub" && git rev-parse --show-toplevel 2>/dev/null)" = "$(cd "$sub" && pwd -P)" ] || continue\n', '')]),
 # ---- validate_cheap
 ('V1 declared paths judged by safe_rel', VC, [("    if not safe_decl(p): die('unsafe_path', repr(p))", "    if not safe_rel(p): die('unsafe_path', repr(p))")]),
 ('V2 held rows judged by safe_rel', VC, [("not safe_decl(c[0]) or not safe_decl(c[1])", "not safe_rel(c[0]) or not safe_rel(c[1])")]),
 ('V3 a declared directory accepted', VC, [("        if not any(e.startswith(b'160000 ')", "        if False and not any(e.startswith(b'160000 ')")]),
 ('V4 a gitlink refused as a directory', VC, [("        if not any(e.startswith(b'160000 ')", "        if True or not any(e.startswith(b'160000 ')")]),
 ('V5 working-tree table taken as the HEAD table (I1)', VC, [("    if b is None: b = helper_table(t)", "    if b is None and os.path.exists(wtp): b = open(wtp, 'rb').read()\n    if b is None: b = helper_table(t)")]),
 ('V6 interpreter form not accepted (I2)', VC, [("INTERPRETERS = ('bash', 'python3')", "INTERPRETERS = ()")]),
 ('V7 interpreter not used to run the script', VC, [("([r['interp']] if r.get('interp') else [])", "[]")]),
 ('V8 interpreter row demands an executable file', VC, [("os.path.isfile(script) and os.access(script, os.R_OK)", "os.path.isfile(script) and os.access(script, os.X_OK)")]),
 ('V9 registry row back to the direct form (I2)', VCT, [("detect_landmines\tbash scripts/detect-landmines.sh\t", "detect_landmines\tscripts/detect-landmines.sh\t")]),
 # ---- check_class
 ('CC1 duplicate exact rows decided by row order (W7)', CC, [("        if len(ec) > 1: die('class_ambiguous'", "        if False: die('class_ambiguous'")]),
 ('CC2 a two-column fixture-root row accepted', CC, [('for r in (rows(fr, 4) if', 'for r in (rows(fr, 2) if')]),
 ('CC3 an option without its value is a traceback', CC, [("        if i + 1 >= len(a): die('usage', f'{k} needs a value')\n", "")]),
 # ---- record_deferral
 ('RD1 an option without its value loops', RD, [('  case "$1" in --run-dir|--flag|--reason|--awaits-review|--commit) [ $# -ge 2 ] || die usage "$1 needs a value" ;; esac\n', '')]),
 # ---- integrate_ff_only
 ('F1 pin proof reads the parent of an uninitialised submodule (B2b)', FF, [('    { [ -d "$d" ] && [ "$(git -C "$d" rev-parse --show-toplevel 2>/dev/null)" = "$(cd "$d" && pwd -P)" ]; } || finish 20 refused pin_unverifiable\n', '')]),
 ('F2 a submodule with no remote accepted (B2a)', FF, [('    [ "$nrem" -gt 0 ] || finish 20 refused pin_not_on_remote\n    [ "$acc" = true ] || finish 20 refused pin_unverifiable', '    :')]),
 ('F3 an unproven pin accepted', FF, [('    [ "$acc" = true ] || finish 20 refused pin_unverifiable\n', '')]),
 ('F4 a failed remote does not veto the proof', FF, [('acc=false; [ -z "$lack" ] && [ "$failed" = 0 ] && [ -n "$held" ] && acc=true', 'acc=false; [ -z "$lack" ] && [ -n "$held" ] && acc=true')]),
 ('F5a a remote tip that is not held locally goes into --not (the review I5 form)', FF, [('if safe_sha "$t" && git -C "$dir" cat-file -e "$t^{commit}" 2>/dev/null; then ts="$ts $t"; else echo "!$t"; unk=1; fi', 'ts="$ts $t"')]),
 ('F5b a remote tip that is not held locally is skipped', FF, [('then ts="$ts $t"; else echo "!$t"; unk=1; fi', 'then ts="$ts $t"; else :; fi')]),
 ('F6 owned organisation parsed by the private basename/dirname form (I6/W1)', FF, [('org="$(python3 "$ORGOF" "$u" 2>/dev/null | head -1)"', 'org="$(basename "$(dirname "$u")")"')]),
 ('F7 owned organisation compared case-sensitively', FF, [("OWNEDL=\"$(printf '%s' \"$OWNED\" | tr 'A-Z' 'a-z')\"", 'OWNEDL="$OWNED"')]),
 ('F8 remotes of the main repository not vetted (I7)', FF, [('vet_remotes "$ROOT" || finish 20 refused "$VETREASON"\n', '')]),
 ('F9 remotes of the submodules not vetted (I7)', FF, [('vet_remotes "$ROOT/$p" || finish 20 refused "$VETREASON"; done', 'true; done')]),
 ('F10 timeout not validated', FF, [('[[ "$TMO" =~ ^[1-9][0-9]*$ ]] || finish 20 refused usage', 'true')]),
 ('F11 option values not guarded (loops)', FF, [('  case "$1" in --root|--branch|--changeset-from|--path-gates|--approved|--adoption-commit|--owned-orgs|--run-dir|--json|--timeout) [ $# -ge 2 ] || finish 20 refused usage ;; esac\n', '')]),
 ('F12 live tip is the first tail match (I3)', FF, [('tip="$(printf \'%s\\n\' "$lr" | lr_exact "$BRANCH")"', 'tip="$(printf \'%s\\n\' "$lr" | awk \'{print $1; exit}\')"')]),
 ('EQ-F13 ls-remote without --: EQUIVALENT, a vetted remote name can never start with a dash', FF, [('ls-remote -- "$r" "refs/heads/$BRANCH" 2>&1)"', 'ls-remote "$r" "refs/heads/$BRANCH" 2>&1)"')]),
 ('F14 the declared change set is not validated', FF, [('while IFS= read -r c; do [ -n "$c" ] || continue; safe_declpath "${c%/}" || finish 20 refused unsafe_path; done <<< "$CHANGED"', ':')]),
 ('F15 an unsafe adoption commit accepted', FF, [('[ -z "$ADOPT" ] || safe_sha "$ADOPT" || finish 20 refused unsafe_adoption_commit', ':')]),
 # ---- push_recursive
 ('P1 discover walks into an uninitialised submodule (I4)', PR, [('    is_root "$dir/$sp" || continue\n', '    [ -d "$dir/$sp" ] && git -C "$dir/$sp" rev-parse --git-dir >/dev/null 2>&1 || continue\n')]),
 ('P2 an uninitialised --repo judged as the parent (I4)', PR, [('  is_root "$dir" || { printf \'REFUSED', '  { [ -d "$dir" ] && git -C "$dir" rev-parse --git-dir >/dev/null 2>&1; } || { printf \'REFUSED')]),
 ('P3 every remote unreachable still judged (I8)', PR, [('  if [ "${#REM[@]}" -gt 0 ] && [ "${#UNR[@]}" -eq "${#REM[@]}" ]; then', '  if false; then')]),
 ('P4 held merge of a remote tip reported moved (I9)', PR, [('        if [ "$held_any" = 1 ] && safe_sha "$t"', '        if false && safe_sha "$t"')]),
 ('P5 the hold hides a remote that really moved', PR, [(' && g merge-base --is-ancestor "$t" "$LT" 2>/dev/null; then\n          printf \'NOPUSH\\t%s\\t%s\\theld_below_remote_tip', ' && true; then\n          printf \'NOPUSH\\t%s\\t%s\\theld_below_remote_tip')]),
 ('P6 --repo judged by safe_relpath', PR, [('[ "$r" = . ] || safe_declpath "$r"', '[ "$r" = . ] || safe_relpath "$r"')]),
 ('P7 --repo . refused', PR, [('[ "$r" = . ] || safe_declpath "$r"', 'safe_declpath "$r"')]),
 ('P8 live tip is the first tail match (I3)', PR, [('t="$(printf \'%s\\n\' "$lr" | lr_exact "$BR")"', 't="$(printf \'%s\\n\' "$lr" | awk \'NR==1{print $1}\')"')]),
 ('P9 an unknown remote tip goes into --not (W4)', PR, [('if [ -n "$t" ] && safe_sha "$t" && g cat-file -e "$t^{commit}" 2>/dev/null; then KNOWN+=("$t"); fi', 'if [ -n "$t" ] && safe_sha "$t"; then KNOWN+=("$t"); fi')]),
 # ---- integrate_merge
 ('M1 live tip is the first tail match (I3)', IM, [('t="$(printf \'%s\\n\' "$lr" | lr_exact "$BR")"', 't="$(printf \'%s\\n\' "$lr" | awk \'NR==1{print $1}\')"')]),
 ('M2 an unresolvable adoption commit passes', IM, [('|| return 1   # an unresolvable adoption commit fails closed', '|| return 0   # an unresolvable adoption commit fails closed')]),
 ('M3 the fetch recurses into submodules', IM, [(' --no-recurse-submodules --no-write-fetch-head', ' --no-write-fetch-head')]),
 ('M4 the backup misses untracked directories', IM, [('done < <(g status --porcelain=v1 -z -uall --ignore-submodules=all 2>/dev/null)\npst=', 'done < <(g status --porcelain=v1 -z --ignore-submodules=all 2>/dev/null)\npst=')]),
]
def sh(c, **k): return subprocess.run(c, capture_output=True, text=True, **k)
rep = []; bad = 0
rep.append('# WP-04 fix round 3 mutations (host run: bash scripts/repo/tests/run_wp04c_mutations.sh; each mutation = one edit of a COPY; CAUGHT = test_wp04c.sh exits non-zero)')
rep.append('# driver sha256 ' + hashlib.sha256(open(os.path.join(root, 'scripts/repo/tests/run_wp04c_mutations.sh'), 'rb').read()).hexdigest())
rep.append('# test sha256 ' + hashlib.sha256(open(os.path.join(root, 'scripts/repo/tests/test_wp04c.sh'), 'rb').read()).hexdigest())
def copy_tree():
    d = tempfile.mkdtemp(prefix='wp04c_mut.')
    for sub in ('scripts/repo', 'scripts/audit'):
        dst = os.path.join(d, sub); os.makedirs(dst)
        for f in os.listdir(os.path.join(root, sub)):
            s = os.path.join(root, sub, f)
            if os.path.isfile(s) and (sub == 'scripts/repo' or f == 'org_of.py'): shutil.copy2(s, dst)
    return d
def run_test(d):
    env = dict(os.environ, R3=os.path.join(d, 'scripts/repo'))
    r = sh(['bash', os.path.join(root, 'scripts/repo/tests/test_wp04c.sh')], cwd=root, env=env)
    return r.returncode, [l for l in r.stdout.splitlines() if l.startswith('FAIL')]
if not only or only == 'control':
    d = copy_tree(); rc, fails = run_test(d)
    line = f'CONTROL  test_wp04c.sh against an unmutated copy -> exit {rc}, {len(fails)} failing case(s)'; rep.append(line); print(line, flush=True)
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
