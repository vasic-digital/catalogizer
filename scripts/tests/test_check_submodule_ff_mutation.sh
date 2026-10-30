#!/usr/bin/env bash
# Paired mutation test for scripts/check_submodule_ff.sh (constitution 1.1).
# Copies the gate, its org parser and own-org list, and its fixture test into a scratch tree,
# applies one mutant at a time, and requires the fixture test
# (scripts/tests/test_check_submodule_ff.sh) to FAIL on every mutant and PASS on the unmutated copy.
# Exit 0 = every mutant killed; 1 = a mutant survived or the baseline failed;
# 2 = a mutant did not apply (the sed pattern no longer matches the gate).
set -uo pipefail
HERE="$(cd "$(dirname "$0")/.." && pwd)"
W="$(mktemp -d "${TMPDIR:-/tmp}/subff_mut.XXXXXX")"; trap 'rm -rf "${W:?}"' EXIT
mkdir -p "$W/scripts/tests" "$W/scripts/audit"
cp "$HERE/tests/test_check_submodule_ff.sh" "$W/scripts/tests/"
cp "$HERE/audit/org_of.py" "$HERE/audit/own_orgs.txt" "$W/scripts/audit/"
G="$HERE/check_submodule_ff.sh"
run() { nice -n 15 bash "$W/scripts/tests/test_check_submodule_ff.sh" >/dev/null 2>&1; }
cp "$G" "$W/scripts/"
if run; then echo "BASELINE PASS"; else echo "BASELINE FAIL"; exit 1; fi
rc=0
mutant() {  # $1 = name, $2 = sed expression
  sed "$2" "$G" > "$W/scripts/check_submodule_ff.sh"
  if cmp -s "$G" "$W/scripts/check_submodule_ff.sh"; then echo "NOT-APPLIED $1"; rc=2; return; fi
  if run; then echo "SURVIVED $1"; [ "$rc" -eq 0 ] && rc=1; else echo "KILLED $1"; fi
}
# the required one: invert the ancestor check (FF and NONFF swap)
mutant invert_ancestor   's/merge-base --is-ancestor "\$osha" "\$nsha"/merge-base --is-ancestor "$nsha" "$osha"/'
mutant invert_case       's/^    0) echo "FF/    1) echo "FF/; s/^    1) echo "NONFF/    0) echo "NONFF/'
mutant blind_as_ff       's/if \[ -z "\$store" \]; then/if false; then/'
mutant blind_exit_pass   's/^if \[ "\$OWN_BLIND" -gt 0 \]; then echo "VERDICT: BLIND \$SUMMARY"; exit 2; fi$/:/'
mutant all_third_party   's/^  CLS=own$/  CLS=third-party/'
mutant all_own           's/CLS=third-party/CLS=own/g'
mutant skip_needle       's/^needle; nrc=\$?$/nrc=0/'
mutant needle_lenient    's/if \[ "\$fw" = "FF own" \] \&\& \[ "\$bw" = "NONFF own" \] \&\&$/if true \&\&/'
mutant fail_not_exit1    's/echo "VERDICT: FAIL \$SUMMARY\$NOTE"; exit 1/echo "VERDICT: FAIL $SUMMARY$NOTE"; exit 0/'
# reviewer mutants (round 1) M2..M6, re-expressed against the current gate
mutant M2_mb_error_as_ff 's/^    \*) echo "BLIND \$cls \$label \$osha \$nsha commit=\$tag reason=merge-base-rc-\$anc"; count "\$cls" BLIND ;;/    *) echo "FF $cls $label $osha $nsha commit=$tag"; count "$cls" FF ;;/'
mutant M3_store_old_only 's/git --git-dir="\$cand" --work-tree="\$cand" cat-file -e "\$b^{commit}" 2>\/dev\/null || continue/:/'
mutant M4_needle_no_line3 's/^     \[ "\$(printf .%s\\n. "\$out" | sed -n 3p)" = "ORGMATCH 1" \]; then/     true; then/'
mutant M6_own_blind_tp   's/    own:BLIND) OWN_BLIND=\$((OWN_BLIND + 1)) ;;/    own:BLIND) TP_BLIND=$((TP_BLIND + 1)) ;;/'
# review round 1 fixes F1..F7
mutant F1_first_parent   's/for p in \$parents; do check_diff/for p in ${parents%% *}; do check_diff/'
mutant F1_no_range       's/^  check_diff "\$2" "\$3" RANGE$/  :/'
mutant F2_ignore_revlist 's/echo "ERROR git rev-list \$2..\$3 failed"; GIT_ERR=1; return/:/'
mutant F2_ignore_diff    's/echo "ERROR git diff-tree \$a \$b failed"; GIT_ERR=1; return/:/'
mutant F2_err_pass       's/^if \[ "\$GIT_ERR" -ne 0 \] || /if false || /'
mutant F3_no_name_pair   's/if \[ -n "\${rm_name\[\$i\]}" \] \&\& \[ "\${rm_name\[\$i\]}" = "\${ad_name\[\$j\]}" \]; then rm_pair\[i\]="\$j"; ad_used\[j\]=1; break; fi/:/'
mutant F5_lenient_shape  's/if \[\[ "\$path" =~ \^\[\^\/\]+\/\[\^\/\]+\$ \]\] \&\& /if /'
mutant F6_no_shallow     '/--git-dir="\$store" --work-tree="\$store" rev-parse --is-shallow/s/!= false \]; then$/= shallowcheckoff ]; then/'
mutant F7_split_space    's/key="\${rec%%\$.\\n.\*}"; val="\${rec#\*\$.\\n.}"/key="${rec%% *}"; val="${rec#* }"/'
# reviewer mutants (round 2) OM1..OM7
mutant OM1_no_lowercase  '/OWN_ORGS="\$(sed/s/ | tr .\[:upper:\]. .\[:lower:\].//'
mutant OM3_pair_empty_names 's/if \[ -n "\${rm_name\[\$i\]}" \] \&\& \[ "\${rm_name/if [ "${rm_name/'
mutant OM3_pair_empty_urls  's/\[ -z "\${rm_pair\[\$i\]}" \] \&\& \[ -n "\${rm_key\[\$i\]}" \] || continue/[ -z "${rm_pair[$i]}" ] || continue/; s/\[ -n "\${ad_key\[\$j\]}" \] \&\& //g'
mutant OM5_no_checkout_store 's/for cand in "\${common:+\${name:+\$common\/modules\/\$name}}" "\$repo\/\$path"; do/for cand in "${common:+${name:+$common\/modules\/$name}}"; do/'
mutant OM6_blind_before_fail 's/^if \[ "\$OWN_NONFF" -gt 0 \]; then echo "VERDICT: FAIL \$SUMMARY\$NOTE"; exit 1; fi$/if [ "$OWN_BLIND" -gt 0 ]; then echo "VERDICT: BLIND $SUMMARY"; exit 2; fi; if [ "$OWN_NONFF" -gt 0 ]; then echo "VERDICT: FAIL $SUMMARY$NOTE"; exit 1; fi/'
mutant OM7_scp_no_host_guard '/{ \[ -n "\${BASH_REMATCH\[1\]}" \] ||/s/{ .* }; then$/true; then/'
# review round 2 fixes N1..N6
mutant N1_keep_git_env   's/^unset GIT_DIR GIT_WORK_TREE/: unset GIT_DIR GIT_WORK_TREE/'
mutant N1_no_gitdir_check 's/!= "\$IN_GITDIR" \]; then/= "$IN_GITDIR-never" ]; then/'
mutant N2_no_url_pair    's/^    \[ -z "\${rm_pair\[\$i\]}" \] \&\& \[ -n "\${rm_key\[\$i\]}" \] || continue$/    continue/'
mutant N2_no_canon_lower '/^  l="\$(printf/s/ | tr .\[:upper:\]. .\[:lower:\].//'
mutant N3_replace_honoured 's/^export GIT_NO_REPLACE_OBJECTS=1 /export /'
mutant N3_store_grafts_ok  's/\[ -z "\$common" \] || \[ -e "\$common\/info\/grafts" \]/false/'
mutant N3_parent_grafts_ok 's/\[ -z "\$PCOMMON" \] || \[ -e "\$PCOMMON\/info\/grafts" \]/false/'
mutant N4_no_range_classes 's/^  parse_gm "\$2"; parse_gm "\$3"$/  :/; s/^    parse_gm "\$commit"; for p in .*$/    :/'
mutant N6_inline_comment  's/s\/#\.\*\/\/; //'
mutant N6_no_note         's/\[ "\$CHECKED" -eq 0 \] \&\& NOTE=" note=nothing-checked" || NOTE=""/NOTE=""/'
# review round 3 fixes T1, S1, C1, T2, P1
mutant T1_no_recurse      's/diff-tree -z -r --ignore-submodules=none --no-commit-id/diff-tree -z --ignore-submodules=none --no-commit-id/'
mutant S1_no_scheme_form  's/^  if \[\[ "\$l" =~ \^(ssh|git|http|https|git\\+ssh|ssh\\+git):/  if false \&\& [[ "$l" =~ ^(ssh|git|http|https|git\\+ssh|ssh\\+git):/'
mutant S1_unpaired_pass   's/if \[ "\$CLS" = own \] \&\& \[ "\$own_add" -eq 1 \]; then/if false; then/'
mutant C1_match_nothing_ok 's/^if \[ "\$CHECKED" -gt 0 \] \&\& \[ "\$ORG_MATCH" -eq 0 \] \&\& .* reason=own-orgs-match-nothing"; exit 2; fi$/:/'
mutant C1_needle_no_orgmatch 's/ = "ORGMATCH 1" \]; then/ = "$(printf "%s\\n" "$out" | sed -n 3p)" ]; then/'
# C1_needle_any_class is not run: the needle repo has one URL, so class=own without an own-org
# match is impossible except via the unparsable-URL fallback, which ORGMATCH 1 (line 3) already
# rejects; the class check is therefore equivalent to the ORGMATCH check (documented).
mutant T2_org_substring   's/grep -qxF "\$org"/grep -qF "$org"/'
mutant T2_url_pair_reuse  's/ad_key\[\$j\]}" \]; then rm_pair\[i\]="\$j"; ad_used\[j\]=1; break; fi/ad_key[$j]}" ]; then rm_pair[i]="$j"; break; fi/'
mutant T2_no_path_key     's/then OWN_KEY\["n:\$name"\]=1; OWN_KEY\["p:\${path_of\[\$name\]}"\]=1/then OWN_KEY["n:$name"]=1/'
mutant T2_no_peel         's/"\$OLD^{commit}"/"$OLD"/; s/"\$NEW^{commit}"/"$NEW"/'
mutant T2_store_new_only  's/git --git-dir="\$cand" --work-tree="\$cand" cat-file -e "\$a^{commit}" 2>\/dev\/null || continue/:/'
mutant P1_allow_symmetric 's/^if \[\[ "\$\*" == \*\.\.\.\* \]\]; then$/if false; then/'
mutant P1_parent_shallow_ok '/-C "\$REPO" rev-parse --is-shallow/s/!= false \]; then$/= shallowcheckoff ]; then/'
# review round 5
mutant R5_1_no_ignore_none  's/ --ignore-submodules=none//'
mutant R5_1_no_lstree_check 's/^if \[ "\$LS_DIFF" -ne "\$RANGE_GL" \]; then$/if false; then/'
mutant R5_1_lstree_err_ok   's/ || \[ -z "\$LS_DIFF" \]; then echo "VERDICT: BLIND/; then echo "VERDICT: BLIND/'
mutant R5_2_no_fallback     's/^  \[ -n "\$common" \] \&\& \[ -d "\$common\/modules" \] || return 0$/  return 0/'
mutant R5_2_fallback_shallow 's/^    \[ "\$(git --git-dir="\$cand" --work-tree="\$cand" rev-parse --is-shallow-repository 2>\/dev\/null)" = false \] || continue$/    :/'
mutant R5_2_fallback_grafts 's/^    \[ -e "\$cand\/info\/grafts" \] \&\& continue$/    :/'
mutant R5_3_first_match     's/^    if \[ -n "\${amb\[x\$CANON_KEY\]:-}" \] || \[ "\$nadd" -gt 1 \] ||.*then$/    if false; then/'
mutant R5_3_rm_count        's/\[ "\$nadd" -eq 1 \] \&\& \[ "\$nrm" -gt 1 \]/[ "$nadd" -eq 1 ] \&\& [ "$nrm" -gt 9 ]/'
mutant R5_4_guard_any_move  's/ \&\& \[ \$((TP_FF + TP_NONFF + TP_BLIND)) -gt 0 \]; then/; then/'
mutant R5_7_no_query_strip  's/^  if \[\[ "\$u" == \*:\/\/\* \]\]; then u=.*$/  :/'
mutant R5_7_tilde_pct_ok    's/^     \[\[ "\$path" != \*\[~%\]\* \]\]; then$/     true; then/'
mutant R5_2_fallback_no_worktree '/^  while IFS= read -r -d/,/^  done < <(find/s/ --work-tree="\$cand"//'
mutant R5_2_merge_no_worktree 's/git --git-dir="\$store" --work-tree="\$store" merge-base/git --git-dir="$store" merge-base/'
mutant R1_checked_gt1       's/^if \[ "\$CHECKED" -gt 0 \] \&\& \[ "\$ORG_MATCH" -eq 0 \]/if [ "$CHECKED" -gt 1 ] \&\& [ "$ORG_MATCH" -eq 0 ]/'
mutant R3_no_tp_path_key    's/else TP_KEY\["n:\$name"\]=1; TP_KEY\["p:\${path_of\[\$name\]}"\]=1; fi/else TP_KEY["n:$name"]=1; fi/'
mutant R6_F1_no_readd       's/^    readd_lookup "\${ad_name\[\$j\]}" "\${ad_key\[\$j\]}" "\${ad_path\[\$j\]}" "\$CLS" "\${ad_sha\[\$j\]}"$/    GONE_HIT=none/'
mutant R6_F1_no_record      's/^      NEWRM+="\${#E_SHA\[@\]}:b "$/      :/'
mutant R6_F1_no_amb         's/elif \[ "\${E_SHA\[\$g\]}" != "\$sha" \]; then GONE_HIT=amb; return 0; fi/elif false; then :; fi/'
mutant R6_F1_no_consume     's/^      WL\[\$g\]=x$/      :/'
# review round 6 (F6-1 .. F6-3, A3, A7) and the reviewer mutants R1..R4, R7 against the new code
mutant R7_R1_range_readd    's/^  \[ "\$tag" = RANGE \] || for j in/  for j in/'
mutant R7_R2_key_before_name 's/^  if \[ -n "\$name" \]; then$/  if [ -n "$name" ] \&\& [ -z "$key" ]; then/'
mutant R7_R3_consume_pick_only 's/^        if \[ -n "\${E_NAME\[\$g\]}" \] \&\& \[ "\${E_NAME\[\$g\]}" != "\$name" \]; then WL/        [ "$g" = "$pick" ] || continue; if [ -n "${E_NAME[$g]}" ] \&\& [ "${E_NAME[$g]}" != "$name" ]; then WL/'
mutant R7_R4_no_topo        's/rev-list --topo-order --reverse/rev-list --reverse/'
mutant R7_R7_no_name_pass   's/^  if \[ -n "\$name" \]; then$/  if false; then/'
mutant R7_F2_no_name_keep   's/then WL\[\$g\]=n; else/then WL[$g]=x; else/'
mutant R7_A7_first_only     's/^      seen\[x\${E_SHA\[\$g\]}\]=1; GONE_HIT+="\$g "$/      [ -n "$GONE_HIT" ] || GONE_HIT="$g "/'
mutant R7_F1_first_parent_set 's/for e in \${GSET\[\$p\]:-}/for e in ${GSET[${parents%% *}]:-}/'
mutant R7_F1_no_propagate   's/GSET\[\$commit\]="\$s\$NEWRM"/GSET[$commit]="$NEWRM"/'
mutant R7_F1_global_memory  's/^    WL=()$/    :/'
# review round 8: F7-1 (a differing URL key never skips a same-name match), F7-2 (path match when a
# name is missing, unidentifiable removals), F7-3 (reviewer set-operation mutants V1, V3, V6, V7)
mutant R8_F71_key_skip      's/^      \[ "\${WL\[\$g\]}" != x \] \&\& \[ "\${E_NAME\[\$g\]}" = "\$name" \] \&\& cand+=("\$g")$/      [ "${WL[$g]}" != x ] \&\& [ "${E_NAME[$g]}" = "$name" ] \&\& ! { [ -n "$key" ] \&\& [ -n "${E_KEY[$g]}" ] \&\& [ "${E_KEY[$g]}" != "$key" ]; } \&\& cand+=("$g")/'
mutant R8_F72_no_path_pass  's/\[ "\${WL\[\$g\]}" != x \] \&\& \[ "\${E_PATH\[\$g\]}" = "\$path" \] || continue/continue/'
mutant R9_F81_named_skipped 's/^      if \[ -n "\${E_NAME\[\$g\]}" \] \&\& \[ -n "\$name" \] \&\&$/      [ -n "${E_NAME[$g]}" ] \&\& [ -n "$name" ] \&\& continue; if false \&\&/'
mutant R9_F81_path_any_name 's/^      if \[ -n "\${E_NAME\[\$g\]}" \] \&\& \[ -n "\$name" \] \&\&$/      if false \&\&/'
mutant R9_F81_both_ignored  's/"\${E_SHA\[\$g\]}" "\$nsha")" \]; then$/"${E_SHA[$g]}" "$nsha")" ] || true; then/'
mutant R9_F81_no_old_check  '/"\${E_NAME\[\$g\]}" "\$path" "\${E_SHA\[\$g\]}" "\${E_SHA\[\$g\]}")" \] \&\& continue$/d'
mutant R9_F81_no_new_check  '/"\$name" "\$path" "\$nsha" "\$nsha")" \] \&\& continue$/d'
mutant R9_F81_nsha_dropped  's/"\$CLS" "\${ad_sha\[\$j\]}"$/"$CLS"/'
mutant R8_F72_no_unid       's/WL\[\$g\]=x; GONE_HIT=unid/:/'
mutant R8_F72_unid_uncounted 's/reason=readd-unidentified"; count own BLIND/reason=readd-unidentified"/'
mutant R8_F72_unid_tp       's/^  \[ "\$cls" = own \] || return 0$/  :/'
mutant R8_F72_empty_key     's/^  if \[ "\${#cand\[@\]}" -eq 0 \] \&\& \[ -n "\$key" \]; then$/  if [ "${#cand[@]}" -eq 0 ]; then/'
mutant R8_V1_n_overwrites_b 's/\[ "\${WL\[\$i\]:-}" = b \] || WL\[\$i\]="\${e#\*:}"/WL[$i]="${e#*:}"/'
mutant R8_V3_no_seen        's/\[ -n "\${seen\[x\${E_SHA\[\$g\]}\]:-}" \] \&\& continue/:/'
mutant R8_V6_first_cand_only 's/^      WL\[\$g\]=x$/      [ "$g" = "${cand[0]}" ] \&\& WL[$g]=x/'
mutant R8_V7_consumed_propagate 's/\[ "\${WL\[\$i\]}" = x \] || s+=/s+=/'
mutant R6_F2_ignore_rc      's/if \[ "\$grc" -ne 0 \] \&\& \[ "\$grc" -ne 1 \]; then/if false; then/'
mutant R6_F2_no_verdict     's/^if \[ "\$GM_ERR" -ne 0 \]; then echo/if false; then echo/'
mutant R6_F2_rc1_is_error   's/if \[ "\$grc" -ne 0 \] \&\& \[ "\$grc" -ne 1 \]; then/if [ "$grc" -ne 0 ]; then/'
mutant R6_F2_no_list_probe   's/^  git -C "\$CUR_REPO" config --blob "\$commit:.gitmodules" --list > \/dev\/null 2>&1; grc=\$?$/  grc=0/'
cp "$G" "$W/scripts/"
exit $rc
