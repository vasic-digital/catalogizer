# D9 admin role fix - evidence manifest (round 1 update)
- base: 855d78d40175752ec4874a334aa9033a6d7d9908; merged main a61d56c6 (merge, not rebase); branch fix-d9-admin-role
- generated: 2026-11-04T23:38:43Z
- red.txt: TestD9_* on the original unfixed code (6/8 FAIL)
- red_round1_race.txt: concurrent last-admin test on the round-0 fix: all 5 variants end with ZERO active admins
- green_round1_race.txt: same test after the atomic repository guard, go test -race: PASS (12 iterations x 5 variants)
- green_d9.txt / green_d9_race.txt: TestD9* (-race for the latter) on the merged tree
- green_full.txt: go test ./handlers ./repository ./middleware ./database -count=1 on the merged tree (repository FAIL there = sqlmock Delete expectations, fixed afterwards; see green_repository.txt)
- green_repository.txt: ./repository full, ok after the fix; vet.txt: go vet of the four packages
- mut_M*.txt (mutate.py) and mut_R*.txt (mutate_round1.py): 15 mutations, each killed
- probe_suspend.txt: CAT-D9-SUSPEND reproduction (pre-existing)
- live re-run on a rebuilt stack: OWED (PROVISIONAL-DONE)

## sha256
f8646d344c4c15a3f049a47767343f71b764a4322718425269f9697ad7310728  green_d9.txt
bd3e9014d23095719d1a4e05f7630406c5d4d3ecc161b0c54b1f71fd4235c6ef  green_d9_race.txt
03e0a423e81b81cc58edb2be74eebee20dbeb28aa1e1f8689b12799ad98fe6c5  green_full.txt
e2a1f17d677d08147cd39e8def2612f404d76ded7eb530e7281d52e6500eaa5c  green_repository.txt
c20b42032e1d7f9b25f2350da12ab6deb967e3a2764201a8e025b7a1cf44e149  green_round1_race.txt
949b8c39a8db937ecc9dd450491d92655c2fd49f793bb14ed8d673d14d5e8f66  mut_M1_role_not_applied.txt
298b92e68b53fbc954991f246d6a247deb296553d74fd3132ec48f6aca820054  mut_M2_self_demotion_allowed.txt
b83ad360532b62bc2b329b27744cfb1d142ab0358d2b8ef2e735aa4c47054802  mut_M3_no_session_revoke_on_role_change.txt
2da4104ef4440d2641596efd8057069c9d8c406a9a323fe9c7a1b20873105533  mut_M4_list_default_user.txt
1b459300dadaf25c9dbcb52511f46c94d42d081b672320957cd73130f2aa6482  mut_M5_unknown_role_accepted.txt
fbf75384fdbeef3b58161e0454d064f8edde5841af48c0ff9d16119f846af2ec  mut_M6_self_and_last_admin_checks_removed.txt
88e56d8ee7a1d0b30b6066c2c082973ec28acf3efe2cc72024c67308491c52c3  mut_R10_delete_guard_disabled.txt
e41b31e38c185a1ce55631dabc087bcb180b776c9bc86b882fb3f83c7f038664  mut_R3_update_instead_of_updateguarded.txt
d887e084940881047f853f45c13e05236f310f10618a2943410a2c8d44d1f5ad  mut_R4a_count_inactive_admins.txt
c873d7e2c01315a7efea29ceae2d653db4c16ad8f553e41d1703c89fbbac9b8d  mut_R4b_count_locked_admins.txt
26676d0283c7636d8580e5d8902991226a4467673d9622a1a0a2e634e3d6a5f4  mut_R5_audit_event_renamed.txt
159b72aabed4deb8ddb2927b66bdd557f6daed6f989861ae8fb5d0a6e0160e69  mut_R6_first_revoke_ignores_role.txt
e5b8ebade77e149a961ecf9acbb8ead2545d47a6f05dcc45222784baf7a8638a  mut_R7_no_tolower_in_role_names.txt
a9960c3ecbdbe8b6c2c730176bcf07ad10408c076d06f4d2118d92ec4896c7b7  mut_R8_revoke_keyed_on_diff_only.txt
a4968cd3ebf179333aa2492add6c8cd5bf258a0ad853e2c6fa64e20784b0e2bb  mut_R9_update_guard_disabled.txt
7b2e80a959b5f64a39eaff1fd6e85574111e0b353c9a35a8f32bb39414d87163  mutate.py
fab4db5ba1b4bc0e49b9896eecf483f10f061091088da20a8ea5c5edfe7b00c3  mutate_round1.py
09c5de4132dbf657233643fe313197d0004ffd23c58bbc39271d81494d6ff91c  probe_suspend.txt
52843ae0f0a919ee7c22f27bfc4ab4f5aba877349c11f95969e82cf990b6f4fa  red.txt
d0f8a2bc1bbbf64d84646b14e844f76c9944e548afa6de9bc0df28e910920e94  red_round1_race.txt
379657899e7681dedd756dbab8c88d30cc8d3f6cb48b3cf4dacb3a757870b054  vet.txt
