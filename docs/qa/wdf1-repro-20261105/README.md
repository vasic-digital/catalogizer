# WD-F1 / SECQ-2 reproduction attempts (2026-11-05)

Result (closed vocabulary): **not-reproducible-with-evidence** for a lost acknowledged write. Status of SECQ-2 stays UNREPRODUCED, root cause UNCONFIRMED.

- run1/run2: 61 plain live uploads to the real Yandex WebDAV, 0 errors, 0 lost.
- chaos1/chaos2: CHAOS class. A TCP fault proxy resets the connection (mode A: mid-body; mode B: after the whole body, ack lost); the REAL service answers. 11 faults fired (all 11 armed). After a reset the caller retried once.
  - Mode A: server never kept a partial file (present_no_retry=false in all cases); the retried PUT was acknowledged and stored (7 of 8 ok, 1 CORRUPT, see below).
  - Mode B (ack lost): in 2 of 3 cases the file was fully stored although the client saw an error (put error but file present); in 1 of 3 absent until retry.
  - No acknowledged write was ever absent (LOST_ACKED_WRITE = 0 in real cases). Control needle (ack, then remote DELETE) is flagged LOST_ACKED_WRITE in both runs, so the verdict logic fires.
  - One case (chaos1, mode A, 2 MB) was flagged CORRUPT: after the retry the file was present with the right size but my read-back did not match the hash. That run did not record whether it was a read error or a content difference (the detail field was added afterwards). 5 repeats of the same case in chaos2 (read-back length, first-diff offset, body error recorded) were all identical. UNCONFIRMED whether the one CORRUPT was a service fault or a read-back glitch; it is not reproduced.
- hardened_live: the hardened WriteFile (HEAD read-back) against the real service, 9 uploads, ok.
- hardening_unit_RED_before_fix.log.txt: the unit-class tests failing before the hardening.
