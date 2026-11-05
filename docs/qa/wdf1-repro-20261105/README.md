# WD-F1 / SECQ-2 reproduction attempts (2026-11-05)

Result (closed vocabulary): **UNREPRODUCED** (a lost acknowledged write was not observed; root cause UNCONFIRMED). Run outcomes: run1, run2, chaos2, chaos3, hardened_live PASSED; **chaos1 FAILED** with `WD-F1 REPRODUCED: lost_or_corrupt=1` because one case was flagged CORRUPT (same size, hash mismatch, cause not recorded). That is not a lost acknowledged write, but it is an unexplained anomaly, tracked as WD-F1a in `docs/FOLLOW_UPS_NEXT_PHASE.md`, and it is why the result is not labelled not-reproducible.

- run1/run2: 61 plain live uploads to the real Yandex WebDAV, 0 errors, 0 lost.
- chaos1/chaos2: CHAOS class. A TCP fault proxy resets the connection (mode A: mid-body; mode B: after the whole body, ack lost); the REAL service answers. 12 faults fired in chaos1/2/3 (all armed ones). After a reset the caller retried once.
  - Mode A: server never kept a partial file (present_no_retry=false in all cases); the retried PUT was acknowledged and stored (7 of 8 ok, 1 CORRUPT, see below).
  - Mode B (ack lost): in 2 of 3 cases the file was fully stored although the client saw an error (put error but file present); in 1 of 3 absent until retry.
  - No acknowledged write was ever absent (LOST_ACKED_WRITE = 0 in real cases). Control needle (ack, then remote DELETE) is flagged LOST_ACKED_WRITE in both runs, so the verdict logic fires.
  - One case (chaos1, mode A, 2 MB) was flagged CORRUPT: after the retry the file was present with the right size but my read-back did not match the hash. That run did not record whether it was a read error or a content difference (the detail field was added afterwards). 5 repeats of the same case in chaos2 (read-back length, first-diff offset, body error recorded) were all identical. UNCONFIRMED whether the one CORRUPT was a service fault or a read-back glitch; it is not reproduced.
- hardened_live: the hardened WriteFile (HEAD read-back) against the real service, 9 uploads, ok.
- hardening_unit_RED_before_fix.log.txt: the unit-class tests failing before the hardening.
- chaos3: rerun with the second control needle (acknowledged write overwritten remotely with same-size different bytes): flagged CORRUPT, so the CORRUPT branch of the verdict fires. Its mode-A 2 MB case was ok. PASSED.
- Read-back size-only limit: the hardened `WriteFile` compares existence and size, not content; ETag/MD5 comparison is a follow-up.
- Review round 1 (Opus NO-GO) fixes: unknown-length uploads (*os.File, pipe, MultiReader) are verified for existence only (was a false error); HEAD 405/501 means cannot verify, accept with a log line, no double upload. RED log: `hardening_review_round_RED_before_fix.log.txt`; paired mutations killed by the tests listed in `mutation_F1_*` and `mutation_F2_*`.

## sha256 of the evidence files
a0602e9fdec45e45f103104e1341aa166f1bbac1529bf3f8f0b800ab1d3c31a3  chaos1.json
2b71980e30c0ae03e2fe16782c03ea1b1f740c324adf50028944b95c2ac8d5ea  chaos1.log.txt
082e7d44972170985f587e9a13800413567732ea423164d1907abacaa0afade8  chaos2.json
ecb3efa79a581b78ac63485f8c755601934114aa70eefc50189bd4d1933314b4  chaos2.log.txt
a00b19849ff68561cfb6d7b541c501c0f53fe668daa76494ca58bfe5f0af4366  chaos3.json
1b502ea6085d00e8870976108cd63f110d8e2d3ac4760c307c8c92b43093dd8b  chaos3.log.txt
50a08fc026f33e8f4b15c460a9a4c318f837e969fcaf8644eee2a4d27cf02600  hardened_live.json
cac847d66185a802771c1db5c774e227a99536b38ea4e61891d31683de795701  hardened_live.log.txt
c82c3ce5a91f9f087c935f109fb9a05a7586c6db019dd1de0d0c2fdcf40d51f4  hardening_review_round_RED_before_fix.log.txt
bc07ac504a5e513655e2ea75f55d3abb54330402a33c3bf1aa2f453b1bdb3177  hardening_unit_RED_before_fix.log.txt
aef5cc89641c0a4d6d38d432e2dc27e13a59fb753024395f281f7d6c3f6e1691  mutation_F1_unknown_length_killed_by.txt
f3c300194d22710f911a65657e50d68c399f95577444837a69bc00e84312cee3  mutation_F2_head_unsupported_killed_by.txt
8ba854ef5ae13bc2fd70f10e895d6a6a1db37a50ce7ecc87f20f1967538b2f5a  run1.json
18d267b9d3058b81c3a9dfdaf5c3da58ba81c30a8405f883861e2c836c40cda7  run1.log.txt
25ed4d5af1bcec4e00d9a9c7ff2c41593ca65ba070016d29fae82a60466e3fce  run2.json
091337f7054f0eaadbced3fec7dcac07268089154b2cc877179b6f11d99d2742  run2.log.txt
