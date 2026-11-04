# D9 admin role fix - evidence manifest
- base: 855d78d40175752ec4874a334aa9033a6d7d9908 (main), branch fix-d9-admin-role
- generated: 2026-11-04T22:18:46Z
- red.log: TestD9_* on unfixed code (6 of 8 FAIL; non-admin and same-role tests pass trivially on the unfixed code)
- green_d9.log: TestD9_* after fix (8/8 PASS)
- green_full.log: go test ./handlers/ ./repository/ ./middleware/ -count=1 (unfiltered) all ok, exit=0
- vet.log: go vet ./handlers/ ./repository/ ./middleware/
- mut_*.log + mutate.py: six mutations, each killed (M2 killed by one test; the last-admin n<=1 check is only reachable once the self check is removed, see M6)
- live re-run on a rebuilt stack: OWED (PROVISIONAL-DONE)

## sha256
f8646d344c4c15a3f049a47767343f71b764a4322718425269f9697ad7310728  green_d9.log
56acca10c7e4bad43d7a0ddfc33f4c2f29f88abda135717f3c5f051ac800ed86  green_full.log
fedda961eb62ef7f367fefb24ace4693e180080aeed3b8ff983c1df13b701a8a  mut_M1_role_not_applied.log
d4f1a6f8d730fc47ae6c8a15670ea8cce4b21269d4d28040c6d2dcf83814540f  mut_M2_self_demotion_allowed.log
578856f6b5dbc060b4850a2721988370e37f9596b7972f1f4281b5acfe6aabca  mut_M3_no_session_revoke_on_role_change.log
29ed4e8042bd5402d886de8b2e5271f85d03bdb884e269169934438a45723134  mut_M4_list_default_user.log
2c4a3c8c10b38729df18b83f76197812fd00d2588b5aa816bf66b2c0caf3f72e  mut_M5_unknown_role_accepted.log
21f047d58a04da80bd3f8e26025507c547fc0de458fcf7108af376987b957f8f  mut_M6_self_and_last_admin_checks_removed.log
52843ae0f0a919ee7c22f27bfc4ab4f5aba877349c11f95969e82cf990b6f4fa  red.log
379657899e7681dedd756dbab8c88d30cc8d3f6cb48b3cf4dacb3a757870b054  vet.log
92b0262bc94399e955e730215917384626fadedd3dd5f798bed1c544961753ed  mutate.py
