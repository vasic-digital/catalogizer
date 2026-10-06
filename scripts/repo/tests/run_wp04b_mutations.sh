#!/usr/bin/env bash
# WP-04 second-slice mutation driver (T040, T040b, T041): one logical edit per mutation, applied to a COPY of scripts/repo in a
# temp dir; the matching test runs against the copy (H=<copy>); the mutation is CAUGHT when the test exits non-zero. An edit whose
# `old` text is absent from the copy is a driver error (never a silent "caught"). A control run of every test against an
# UNMUTATED copy must pass first. Usage: bash scripts/repo/tests/run_wp04b_mutations.sh [out-file] [id-prefix]
# Never edits a tracked file: copies live in a temp dir.
set -u
cd "$(git rev-parse --show-toplevel)" || exit 2
OUT="${1:-/dev/stdout}"; ONLY="${2:-}"
exec python3 - "$OUT" "$ONLY" <<'PY'
import os, shutil, subprocess, sys, tempfile, hashlib
out = sys.argv[1]; only = sys.argv[2]; root = os.getcwd()
R = 'scripts/repo/'; T = 'scripts/repo/tests/'
IM = R+'integrate_merge.sh'
NCI, RH, SC, VC, CR, PR, LIB = R+'check_no_ci.sh', R+'check_revision_headers.sh', R+'scope_check.sh', R+'validate_cheap.sh', R+'commit_recursive.sh', R+'push_recursive.sh', R+'lib_safe.sh'
tN, tH, tS, tV, tC, tP, tM = [T+'test_'+n+'.sh' for n in ('check_no_ci', 'check_revision_headers', 'scope_check', 'validate_cheap', 'commit_recursive', 'push_recursive', 'integrate_merge')]
HELPER_OF = {tN: NCI, tH: RH, tS: SC, tV: VC, tC: CR, tP: PR, tM: IM}
# (id, file, test, [(old, new), ...])   all pairs of one mutation form ONE logical edit
M = [
 ('N1 .drone.yml form dropped', NCI, tN, [('.woodpecker.yml|.drone.yml|.circleci/*)', '.woodpecker.yml|.circleci/*)')]),
 ('N2 .gitea/workflows form dropped', NCI, tN, [('.gitea/workflows/*|', '.gitea/NEVER|')]),
 ('N3 pipeline files matched at any depth', NCI, tN, [('.github/workflows/*.yml|.github/workflows/*.yaml|', '*.github/workflows/*.yml|*.github/workflows/*.yaml|'), ('          .github/workflows/*/*) continue ;;\n', '')]),
 ('N4 ignored files judged too', NCI, tN, [('--cached --others --exclude-standard', '--cached --others')]),
 ('N5 only the first root judged', NCI, tN, [('for r in "${ROOTS[@]}"; do', 'for r in "${ROOTS[0]}"; do')]),
 ('EQ-N6 option-like root accepted: EQUIVALENT mutant, the second layer (`cd -x` fails, not_a_repository) still gives 20', NCI, tN, [('safe_dir_arg "$2" || die unsafe_root', 'true || die unsafe_root')]),
 ('N7 violation exits 0', NCI, tN, [('printf \'ci_pipeline\\t%s\\t%s\\n\' "$r" "$f"; RC=10', 'printf \'ci_pipeline\\t%s\\t%s\\n\' "$r" "$f"; RC=0')]),
 ('N8 README in .github/workflows is a pipeline', NCI, tN, [('.github/workflows/*.yml|.github/workflows/*.yaml|', '.github/workflows/*|')]),
 ('H1 plain Revision lines accepted', RH, tH, [('/^\\|[ \\t]*Revision[ \\t]*\\|/ ||', '/Revision/ ||'), ('/^\\|[ \\t]*Last modified[ \\t]*\\|/ ||', '/Last modified/ ||')]),
 ('H2 header window 40 -> 400 lines', RH, tH, [('head -n 40', 'head -n 400')]),
 ('H3 Last modified not required', RH, tH, [('END { exit !(r && m) }', 'END { exit !(r) }')]),
 ('H4 class not consulted', RH, tH, [('[ "$c" = apply ] || continue', 'true')]),
 ('H5 non-Markdown files judged', RH, tH, [('case "$p" in *.md) ;; *) continue ;; esac', 'case "$p" in *) ;; esac')]),
 ('H6 unsafe path accepted', RH, tH, [('safe_relpath "$p" || die unsafe_path', 'true || die unsafe_path')]),
 ('H7 --measure refuses', RH, tH, [('if [ "$MEASURE" = 1 ]; then printf', 'if false; then printf')]),
 ('S1 other databases allowed', SC, tS, [('refuse "$p" database; continue;', ':;')]),
 ('S2 the register database refused', SC, tS, [('[ "$p" = docs/workable_items.db ] || { refuse "$p" database', '{ refuse "$p" database')]),
 ('S3 .env files allowed', SC, tS, [('.env|.env.*) refuse "$p" env_file; continue ;;', '.env|.env.*) : ;;')]),
 ('S4 .env.example refused', SC, tS, [('.env.example|.env.sample|.env.template) ;;', '.env.sample|.env.template) ;;')]),
 ('S5 append-only prefix not compared', SC, tS, [('|| ! cmp -s -n "$sz" "$W/head.blob" "$ROOT/$p"', '|| false')]),
 ('S7 blob name not checked', SC, tS, [('[[ "$base" =~ ^[0-9a-f]{64}$ ]] && [ "$base" = "$h" ] || { refuse "$p" blob_name; continue; }', 'true')]),
 ('S8 build output not judged', SC, tS, [('if git -C "$ROOT" check-ignore --no-index -q -- "$p" 2>/dev/null; then', 'if false; then')]),
 ('S10 submodules not walked', SC, tS, [('walk "$ROOT" ""', 'true')]),
 ('S11 exceptions row hash not compared', SC, tS, [('$3==f && $4==w {print "y"; exit}', '$3==f {print "y"; exit}')]),
 ('S12 exceptions row file not compared', SC, tS, [('$2=="dirty" && $3==f &&', '$2=="dirty" &&')]),
 ('S13 unsafe path accepted', SC, tS, [('safe_declpath "$l" || die unsafe_path', 'true || die unsafe_path')]),
 ('S14 untracked files in a submodule ignored', SC, tS, [('status --porcelain=v1 -z --ignore-submodules=all', 'status --porcelain=v1 -z -uno --ignore-submodules=all')]),
 ('S15 exceptions kind not compared', SC, tS, [('$1==p && $2=="dirty" && $3==f', '$1==p && $3==f')]),
 ('V1 shell_parse never fails', VC, tV, [("return ('pass', '') if r.returncode == 0 else ('fail', r.stderr.strip()[:200])", "return ('pass', '')")]),
 ('V2 merge_conflict never fails', VC, tV, [("if ln.startswith((b'<<<<<<< ', b'>>>>>>> ', b'======= ')) or ln.rstrip(b'\\r') == b'=======':", "if False:")]),
 ('V3 trailing_whitespace never fails', VC, tV, [("if ln.rstrip(b'\\r') != ln.rstrip(b'\\r').rstrip(b' \\t'):", 'if False:')]),
 ('V4 extra blank lines at end allowed', VC, tV, [(" or data.endswith(b'\\n\\n')", '')]),
 ('V5 missing final newline allowed', VC, tV, [("(not data.endswith(b'\\n') or ", '(')]),
 ('V6 YAML never parsed', VC, tV, [('yaml.safe_load(data)', 'pass')]),
 ('V7 JSON never parsed', VC, tV, [('json.loads(data, object_pairs_hook=hook)', "json.loads('{}')")]),
 ('V8 large_file bound ignored', VC, tV, [("if sz > bound: return 'fail'", 'if False: return \'fail\'')]),
 ('V9 size_alarm not reported', VC, tV, [("if sz > bound * 0.75: emit(", "if False: emit(")]),
 ('V10 binary files given to text checks', VC, tV, [("if name not in TEXTLESS and is_binary(path): emit('left_out', name, path); continue", 'pass')]),
 ('V11 class skip not honoured', VC, tV, [("if ver != 'skip':", 'if True:')]),
 ('V12 differing registry row still runs', VC, tV, [("if name in appr: (running if appr[name] == r['line'] else pending).append(name)", 'if name in appr: running.append(name)')]),
 ('V13 row missing from the approved copy runs', VC, tV, [("else: (running if name in declared_rows else pending).append(name)", 'else: running.append(name)')]),
 ('V14 exit takes the larger of 10 and 14', VC, tV, [("if failed or any(l.startswith('class_table_unreviewed') for l in out): sys.exit(10)\nif pending: sys.exit(14)", "_c = [c for c, f in ((10, failed or any(l.startswith('class_table_unreviewed') for l in out)), (14, bool(pending))) if f]\nsys.exit(max(_c) if _c else 0)")]),
 ('V15 ratchet row accepted', VC, tV, [("if r['mode'] == 'ratchet': die(", "if False: die(")]),
 ('V16 class loader validation of the registry skipped', VC, tV, [("if r0.returncode != 0:", "if False:")]),
 ('V17 registry command may leave the code root', VC, tV, [("if not safe_rel(toks[0]): die(", "if False: die(")]),
 ('V18 absent command not refused', VC, tV, [("if interp is None and not (os.path.isfile(script) and os.access(script, os.X_OK)): die(", "if False: die(")]),
 ('V19 class_table_unreviewed not reported', VC, tV, [("emit('class_table_unreviewed', f'{TREL}/{t}')", "pass")]),
 ('V20 held tables applied to every path', VC, tV, [("elif st[0] == 'held' and allow_held and held.get(path) == st[1]: ch.append('wt')", "elif True: ch.append('wt')")]),
 ('V21 GO with blocking findings accepted', VC, tV, [(" and j.get('blocking_findings') == 0\n", "\n")]),
 ('V22 verdict value not checked', VC, tV, [("return j.get('verdict') == 'GO' and j.get('blocking_findings') == 0", "return j.get('blocking_findings') == 0")]),
 ('V23 table_admits_unheld_path not refused', VC, tV, [("if admits: unheld_findings.append((path, name)); continue", "if admits: continue")]),
 ('V24 every unheld failure reported as admitted', VC, tV, [("admits = av == 'skip'", "admits = True")]),
 ('V25 legacy_row_not_dropped not reported', VC, tV, [("emit('legacy_row_not_dropped', path); failed = True", "pass")]),
 ('V26 legacy row judged without a content difference', VC, tV, [("if head is not None and head != open(os.path.join(root, path), 'rb').read():", "if head is not None:")]),
 ('V27 unsafe declared path accepted', VC, tV, [("if not safe_decl(p): die('unsafe_path', repr(p))", "pass")]),
 ('V28 end_of_file fixes the source in place', VC, tV, [("return 'fail', 'final newline'", "open(f, 'wb').write(data.rstrip(b'\\n') + b'\\n'); return 'fail', 'final newline'")]),
 ('V29 script check result ignored', VC, tV, [("    if rc != 0:\n        failing = [f for f in fl if f in msg] or fl[:5]", "    if False:\n        failing = [f for f in fl if f in msg] or fl[:5]")]),
 ('V30 changeset-scope checks not run', VC, tV, [("for name in cs_checks:", "for name in []:")]),
 ('V33 --run-declared ignored', VC, tV, [("(running if name in declared_rows else pending)", "(running if False else pending)")]),
 ('V34 deferred rows not listed', VC, tV, [("if r['mode'] == 'deferred': emit('deferred', name)", "if r['mode'] == 'deferred': pass")]),
 ('C1 path inside a submodule accepted', CR, tC, [('case "$p" in "$g"/*) die path_in_submodule', 'case "$p" in "$g"/NEVER) die path_in_submodule')]),
 ('C2 attribution trailer dropped', CR, tC, [("[ -z \"$ATTR\" ] || printf '%s\\n' \"$ATTR\"", ':')]),
 ('C3 CPA-Run trailer dropped', CR, tC, [("printf 'CPA-Run: %s\\n' \"$RID\"", ':')]),
 ('C4 deferral flags not read', CR, tC, [('deferred="$("$D/record_deferral.sh" --run-dir "$RUND" --list 2>/dev/null)" || deferred=""', 'deferred=""')]),
 ('C5 commits.tsv row not written', CR, tC, [('printf \'%s\\t%s\\t%s\\n\' "$KEY" "$s" "$RID" >> "$TSV" || die commits_tsv_write_failed "$(printf \'%q\' "$TSV")"', ':')]),
 ('C6 commits.tsv row written at exit, not right after the commit', CR, tC, [('printf \'%s\\t%s\\t%s\\n\' "$KEY" "$s" "$RID" >> "$TSV" || die commits_tsv_write_failed "$(printf \'%q\' "$TSV")"', 'trap "printf \'%s\\t%s\\t%s\\n\' \'$KEY\' \'$s\' \'$RID\' >> \'$TSV\'" EXIT')]),
 ('C7 non-ancestor foreign commit named', CR, tC, [('        git -C "$REPO" merge-base --is-ancestor "$s" HEAD 2>/dev/null || continue\n', '')]),
 ('C8 foreign commit named twice', CR, tC, [('        [ -z "$(git -C "$REPO" log -1 --format=%H --fixed-strings --grep="Foreign-Commit: $s" HEAD 2>/dev/null)" ] || continue\n', '')]),
 ('C9 foreign commits named on every commit', CR, tC, [('if [ "$first" = 1 ]; then', 'if true; then')]),
 ('C10 held table ignored', CR, tC, [('v="${HV[$p]-}"', 'v=""')]),
 ('C11 held commits made before the unheld one', CR, tC, [('[ "${#UNHELD[@]}" -eq 0 ] || make_commit "" "${UNHELD[@]}"\n', ''), ('exit 0\n', '[ "${#UNHELD[@]}" -eq 0 ] || make_commit "" "${UNHELD[@]}"\nexit 0\n')]),
 ('C12 unsafe path accepted', CR, tC, [('for p in "${PATHS[@]}"; do safe_declpath "$p" || die unsafe_path', 'for p in "${PATHS[@]}"; do true || die unsafe_path')]),
 ('C13 commit not limited to the declared paths', CR, tC, [('commit -q -F "$mf" --only -- "${lp[@]}"', 'commit -q -F "$mf"')]),
 ('C14 newline in the attribution accepted', CR, tC, [('safe_line "$ATTR" || die unsafe_attribution', 'true || die unsafe_attribution')]),
 ('C15 unsafe run id accepted', CR, tC, [('safe_runid "$RID" || die unsafe_run_id', 'true || die unsafe_run_id')]),
 ('C16 repository hooks bypassed', CR, tC, [('commit -q -F "$mf" --only', 'commit -q --no-verify -F "$mf" --only')]),
 ('C18 absent run directory tolerated', CR, tC, [('[ -d "$RUND" ] || die run_dir_absent', 'true || die run_dir_absent')]),
 ('P1 force-like options accepted', PR, tP, [('--force|--force=*|--force-with-lease|--force-with-lease=*|-f|+*|--mirror|--delete|-d|--prune|--all|--tags) die force_refused', 'NEVERMATCHES) die force_refused')]),
 ('P2 commits.tsv row need not hold the sha', LIB, tP, [("'$2==s {f=1} END{exit !f}'", "'$2==s {f=1} END{exit 0}'")]),
 ('P3 trailer alone proves a CPA commit', PR, tP, [('rid="$(cpa_runid "$dir" "$c")" && cpa_row "$AUD/$rid/commits.tsv" "$c" || UREC+=("$c")', 'rid="$(cpa_runid "$dir" "$c")" || UREC+=("$c")')]),
 ('P4 holds ignored', PR, tP, [('if [ "$st" != ok ]; then', 'if false; then')]),
 ('P5 verdict value not checked', PR, tP, [("if v.get('verdict') != 'GO' or v.get('blocking_findings') != 0: print('not_go'); sys.exit()", 'pass')]),
 ('P6 covers_runs not checked', PR, tP, [("print('ok' if any(isinstance(e, dict) and e.get('cpa_run') == sys.argv[3] for e in (v.get('covers_runs') or [])) else 'not_covered')", "print('ok')")]),
 ('P7 schema validation skipped', PR, tP, [('import jsonschema; jsonschema.validate(v, schema)', 'pass')]),
 ('P8 verdict read from the working tree, not HEAD', PR, tP, [('git -C "$MAIN" cat-file -e "HEAD:$vp" 2>/dev/null || { echo absent; return; }', '[ -f "$MAIN/$vp" ] || { echo absent; return; }'), ('git -C "$MAIN" cat-file blob "HEAD:$vp" > "$W/verdict.json" 2>/dev/null;', 'cp "$MAIN/$vp" "$W/verdict.json";')]),
 ('P9 unreleased merge does not block', PR, tP, [('[ "$(g rev-list --parents -n1 "$c" | awk \'{print NF-1}\')" -le 1 ] || mergeblock=1', ':')]),
 ('P10 the LAST unreleased held commit cuts the prefix', PR, tP, [('[ "$cut" -ge 0 ] || cut="$idx"', 'cut="$idx"')]),
 ('P11 remote ancestry not checked', PR, tP, [('g merge-base --is-ancestor "$t" "$target" 2>/dev/null; }; then', 'true; }; then')]),
 ('P12 remote already at the target pushed again', PR, tP, [('if [ "$t" = "$target" ]; then printf \'NOPUSH\\t%s\\t%s\\talready_holds_tip\\n\' "$key" "$r"; continue; fi', ':')]),
 ('P13 force refspec', PR, tP, [('push -q -- "$r" "$target:refs/heads/$BR"', 'push -q -- "$r" "+$target:refs/heads/$BR"')]),
 ('P14 unsafe remote name accepted', PR, tP, [('if ! safe_remote "$r"; then', 'if false; then')]),
 ('P15 unsafe remote URL accepted', PR, tP, [('safe_url "$u" || {', 'true || {')]),
 ('P16 push without --', PR, tP, [('push -q -- "$r"', 'push -q "$r"')]),
 ('P16b ls-remote without --', PR, tP, [('ls-remote -- "$r"', 'ls-remote "$r"')]),
 ('P17 shallowest repository first', PR, tP, [("sort -t$'\\t' -k1,1nr -k2,2", "sort -t$'\\t' -k1,1n -k2,2")]),
 ('P18 unsafe branch accepted', PR, tP, [('safe_branch "$BR" || die unsafe_branch', 'true || die unsafe_branch')]),
 ('P20 a failed push stops the other remotes', PR, tP, [('bump 11; fi\n  done\n  [ "$held_any" = 0 ]', 'bump 11; exit "$EXIT"; fi\n  done\n  [ "$held_any" = 0 ]')]),
 ('P21 unreachable remote not recorded', PR, tP, [('printf \'NOPUSH\\t%s\\t%s\\tremote_unreachable\\n\' "$key" "$r"; bump 11; continue', 'continue')]),
 ('L1 safe_relpath accepts ..', LIB, tH, [('..|../*|*/..|*/../*|./..|*//*) return 1 ;;', '*//*) return 1 ;;')]),
 ('L2 safe_relpath accepts a leading dash', LIB, tS, [('case "$p" in /*|-*|*\\\\*|', 'case "$p" in /*|*\\\\*|')]),
 ('L3 safe_remote accepts a leading dash', LIB, tP, [('^[A-Za-z0-9][A-Za-z0-9._-]*$ ]] && [ "${1: -5}"', '^[A-Za-z0-9-][A-Za-z0-9._-]*$ ]] && [ "${1: -5}"')]),
 ('L4 safe_url accepts a leading dash', LIB, tP, [('case "$1" in -*|ext::*|*"::"*) return 1 ;; esac', 'case "$1" in ext::*|*"::"*) return 1 ;; esac')]),
 ('L5 control characters accepted', LIB, tS, [('_no_ctl() { case "$1" in *[[:cntrl:]]*) return 1 ;; esac; return 0; }', '_no_ctl() { return 0; }')]),
 ('L6 ext:: URL accepted', LIB, tP, [('case "$1" in -*|ext::*|*"::"*) return 1 ;; esac', 'case "$1" in -*) return 1 ;; esac')]),
 ('EQ-S16 option-like root accepted (scope_check): EQUIVALENT, `cd -x` still refuses', SC, tS, [('safe_dir_arg "$2" || die unsafe_root', 'true || die unsafe_root')]),
 ('EQ-H9 option-like root accepted (revision headers): EQUIVALENT, `cd -x` still refuses', RH, tH, [('safe_dir_arg "$2" || die unsafe_root', 'true || die unsafe_root')]),
 ('EQ-C19 option-like repository accepted: EQUIVALENT, `cd -x` still refuses', CR, tC, [('safe_dir_arg "$REPO" || die unsafe_repo', 'true || die unsafe_repo')]),
 ('EQ-P22 option-like main root accepted: EQUIVALENT, `cd -x` still refuses', PR, tP, [('safe_dir_arg "$MAIN" || die unsafe_main_root', 'true || die unsafe_main_root')]),
 ('V35 option-like root accepted (validate_cheap)', VC, tV, [("            if not safe_dir(v): die('unsafe_root', repr(v))\n            root = v", "            root = v")]),
 ('IM1 force-like options accepted', IM, tM, [('--force*|-f|+*|--rebase|--reset|--hard|--merge) die force_refused', 'NEVERMATCHES) die force_refused')]),
 ('IM2 tips that are ancestors of HEAD are targets', IM, tM, [('if [ "$t" = "$LOCAL" ] || g merge-base --is-ancestor "$t" "$LOCAL" 2>/dev/null; then continue; fi', ':')]),
 ('IM3 adoption commit not required for a descendant tip', IM, tM, [('then adopt_ok "$t" && continue; fi', 'then continue; fi')]),
 ('IM4 no single newest target (every tip in turn)', IM, tM, [('[ "$ok" = 1 ] && { newest="$i"; break; }', ':')]),
 ('IM5 pairwise merge-tree check removed', IM, tM, [('if ! g merge-tree --write-tree "${TARGETS[$i]}" "${TARGETS[$j]}" >/dev/null 2>&1; then', 'if false; then')]),
 ('IM6 index-differs-from-HEAD check removed', IM, tM, [('g diff --cached --quiet 2>/dev/null || { echo "ff_blocked_by_local_changes: the index differs from HEAD"; exit 12; }', ':')]),
 ('IM7 uncommitted-overlap check removed', IM, tM, [('[ -z "$hit" ] || { echo "ff_blocked_by_local_changes: $(printf \'%s\' "$hit" | tr \'\\n\' \' \')"; exit 12; }', ':')]),
 ('IM8 bundle not created', IM, tM, [('g bundle create "$BK/$kf.bundle" "refs/heads/$BR" "^$first" >/dev/null 2>&1 || die backup_failed "git bundle create"', ':')]),
 ('IM9 bundle not verified', IM, tM, [('g bundle verify "$BK/$kf.bundle" >/dev/null 2>&1 || die backup_failed "git bundle verify"', ':')]),
 ('IM10 uncommitted files not copied to the backup', IM, tM, [('cp -p -- "$ROOT/$f" "$BK/worktree/$f" || die backup_failed "copy of $(printf \'%q\' "$f")"', ':')]),
 ('IM11 merge.json lacks the local tip', IM, tM, [('local_tip:$local,', '')]),
 ('IM12 process start time not recorded', IM, tM, [('[ -r "/proc/$RPID/stat" ] && pst=', 'false && pst=')]),
 ('IM13 conflicting merge not aborted', IM, tM, [('      g merge --abort >/dev/null 2>&1\n      [ "$(g rev-parse HEAD)" = "$LOCAL" ]', '      [ "$(g rev-parse HEAD)" = "$LOCAL" ]')]),
 ('IM14 conflict files not copied', IM, tM, [('cp -p -- "$ROOT/$f" "$RUND/conflicts/$f" 2>/dev/null || true', ':')]),
 ('IM15 held local commit not detected', IM, tM, [('[ -z "$vp" ] || held="${held:+$held,}local_held_commit"', ':')]),
 ('IM16 adoption hold removed', IM, tM, [('adopt_ok "$t" || held="${held:+$held,}adoption"', ':')]),
 ('IM17 G-GATE content comparison removed', IM, tM, [('[ "$got" = "$want" ] || held="${held:+$held,}gate"', ':')]),
 ('IM18 admission table change by a non-CPA commit not held', IM, tM, [('is_cpa "$c" || held="${held:+$held,}admission_table"', ':')]),
 ('IM19 CPA commits named as foreign', IM, tM, [('        is_cpa "$c" && continue\n', '')]),
 ('IM21 commits.tsv row not written', IM, tM, [('printf \'%s\\t%s\\t%s\\n\' "$KEY" "$sha" "$RID" >> "$RUND/commits.tsv" || die commits_tsv_write_failed "commits.tsv"', ':')]),
 ('IM22 merge without --no-ff', IM, tM, [('g merge --no-ff --no-commit "$t" >"$W/merge.out" 2>&1; mrc=$?', 'g merge --no-commit "$t" >"$W/merge.out" 2>&1; mrc=$?')]),
 ('IM23 helper pushes the merged tip', IM, tM, [('printf \'MERGED\\t%s\\t%s\\t%s\\t%s\\n\' "$r" "$t" "$sha" "${held:-unheld}"', 'for rr in "${REMS[@]}"; do g push -q -- "$rr" "HEAD:refs/heads/$BR" >/dev/null 2>&1; done; printf \'MERGED\\t%s\\t%s\\t%s\\t%s\\n\' "$r" "$t" "$sha" "${held:-unheld}"')]),
 ('IM25 resolution dir outside .audit/merge-resolution accepted', IM, tM, [('*) die resolution_dir_invalid "$rd" ;; esac', '*) : ;; esac')]),
 ('IM26 moved tip not detected', IM, tM, [('[ "$rtip" = "$newest_tip" ] || {', 'true || {')]),
 ('IM27 resolver model not pinned', IM, tM, [('opus|claude-opus*) ;; *) die merge_resolver_not_pinned "model $(printf \'%q\' "$model")" ;; esac', 'opus|claude-opus*) ;; *) : ;; esac')]),
 ('IM28 resolver effort not pinned', IM, tM, [('[ "$effort" = xhigh ] || die merge_resolver_not_pinned', 'true || die merge_resolver_not_pinned')]),
 ('IM29 conflict set not compared', IM, tM, [('[ "$mrc" != 0 ] && [ "$cset" = "$want" ] || {', 'true || {')]),
 ('IM30 conflict markers in resolved files not detected', IM, tM, [("grep -qE '^(<<<<<<< |=======$|>>>>>>> )' \"$rd/$p\" && {", 'false && {')]),
 ('IM31 resolved file hash not compared', IM, tM, [('[ "$h" = "$(jq -r --arg p "$p" \'.files[$p] // empty\' "$rd/resolution.json")" ] || die resolution_invalid', 'true || die resolution_invalid')]),
 ('IM32 store method re-recorded not required', IM, tM, [('= re-recorded ] || die store_not_rerecorded "$p: the record does not name method re-recorded"', '= re-recorded ] || true')]),
 ('IM33 store prefix not compared', IM, tM, [('cmp -s -n "$sz" "$W/side.blob" "$rd/$p" && [', 'true && [')]),
 ('IM34 secret fold gap silently passed', IM, tM, [('die secret_fold_unavailable "the S2 secret fold (T040a) is not built; no resolved file is written"', 'exit 0')]),
 ('IM36 interrupted merge not refused', IM, tM, [('[ ! -e "$(g rev-parse --path-format=absolute --git-path MERGE_HEAD)" ] || die merge_in_progress', 'true || die merge_in_progress')]),
 ('IM37 branch not checked out accepted', IM, tM, [('[ "$cur" = "$BR" ] || die wrong_branch', 'true || die wrong_branch')]),
 ('IM38 unsafe remote name accepted', IM, tM, [('safe_remote "$r" || die unsafe_remote_name', 'true || die unsafe_remote_name')]),
 ('P23 ancestry of a known remote tip not checked (again, by test 6b)', PR, tP, [('g merge-base --is-ancestor "$t" "$target" 2>/dev/null; }; then', 'true; }; then')]),

]
def sh(c, **k): return subprocess.run(c, capture_output=True, text=True, **k)
rep = []; bad = 0
rep.append('# WP-04 second-slice mutations (host run: bash scripts/repo/tests/run_wp04b_mutations.sh; each mutation = one edit of a COPY; CAUGHT = its test exits non-zero)')
rep.append('# driver sha256 ' + hashlib.sha256(open(os.path.join(root, 'scripts/repo/tests/run_wp04b_mutations.sh'), 'rb').read()).hexdigest())
def copy_tree():
    d = tempfile.mkdtemp(prefix='wp04b_mut.'); dst = os.path.join(d, 'scripts', 'repo'); os.makedirs(dst)
    for f in os.listdir(os.path.join(root, 'scripts/repo')):
        s = os.path.join(root, 'scripts/repo', f)
        if os.path.isfile(s): shutil.copy2(s, dst)
    return d
def run_test(t, helper_copy):
    env = dict(os.environ, H=helper_copy)
    r = sh(['bash', os.path.join(root, t)], cwd=root, env=env)
    fails = [l for l in r.stdout.splitlines() if l.startswith('FAIL')]
    return r.returncode, fails
# control: every test passes against an unmutated copy
if not only or only == 'control':
    d = copy_tree()
    for t, h in HELPER_OF.items():
        rc, fails = run_test(t, os.path.join(d, h))
        line = f'CONTROL  {os.path.basename(t)} against an unmutated copy -> exit {rc}, {len(fails)} failing case(s)'
        rep.append(line); print(line, flush=True)
        if rc != 0: bad += 1
    shutil.rmtree(d, ignore_errors=True)
for mid, f, t, pairs in M:
    if only and only != 'control' and not mid.startswith(only): continue
    if only == 'control': continue
    d = copy_tree(); p = os.path.join(d, f); s = open(p).read()
    for old, new in pairs:
        if old not in s:
            line = f'ERROR    {mid}  -> old text not found in {f}: {old[:70]!r}'; rep.append(line); print(line, flush=True); bad += 1; break
        s = s.replace(old, new, 1)
    else:
        open(p, 'w').write(s)
        h = os.path.join(d, HELPER_OF[t])
        rc, fails = run_test(t, h)
        if rc != 0:
            line = f'CAUGHT   {mid}  -> {len(fails)} failing case(s), e.g. {fails[0][:110] if fails else "exit " + str(rc)}'
        elif mid.startswith('EQ-'):
            line = f'EQUIVALENT {mid}  -> survived as documented (the guard it removes has a second layer)'
        else:
            line = f'SURVIVED {mid}  -> test passed against the mutated copy'; bad += 1
        rep.append(line); print(line, flush=True)
    shutil.rmtree(d, ignore_errors=True)
rep.append(f'---- {len(M)} mutations listed, {bad} not caught or errored')
open(out, 'w').write('\n'.join(rep) + '\n') if out != '/dev/stdout' else None
sys.exit(1 if bad else 0)
PY
