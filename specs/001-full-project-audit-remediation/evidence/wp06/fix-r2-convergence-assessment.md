# WF13 fix round (11.4.276 round 3, STRUCTURAL): convergence assessment, defect classes, ground truth

| Field | Value |
|---|---|
| Identity | catalogizer / 001 / WP-06 register operations, review `WF13-REVIEW-register-ops-r2.md` (round 2 NO-GO: blocking N1, N2; important N3, T1, T2; minor N4-N9, T3-T5, P1-P3) |
| Date | 2026-10-07 |
| Round | 3 of the 5-7 round budget (constitution 11.4.276) |
| Status | written BEFORE the fixes; the fixes and their evidence follow in `fix-r2-*` files; independent re-review owed (11.4.142) |

## Convergence assessment (11.4.276(E)(3))

Round 1 (WF10) found F1-F14, round 2 (WF13) found N1-N9, T1-T5. Of the round-2 findings, N1 and N2 are NEW MEMBERS of the classes round 1 named (F2 "local writes dropped by replay", F3 "lock released while the writer runs"), found by enumerating the class instead of the instance: round 1 closed two members of each class (base row, WAL-only base; SIGTERM to locked.sh), round 2 found the next members (failed row, SIGPIPE/SIGKILL). The same shape appeared in F6 -> N4 (lock claim: "value 1" closed, "fd 9 opened but never locked" open) and F4/F5 -> N5 (exit status). That is the 11.4.276(E)(3) trigger: two or more findings of one class with a non-minor one, so this round is STRUCTURAL: each class is named, its real mechanism re-derived against real podman and the real `locked.sh`, every member closed, and the old point checks replaced where they were wrong rather than layered.

## Ground truth re-derived (real podman 5.7.0, real `run_pinned.sh`, real `locked.sh`; this session)

| Fact | How it was established |
|---|---|
| `podman run --rm` is a client; the container is held by conmon. SIGKILL of the client leaves the container running; `podman wait <id>` on it afterwards returns its REAL exit status (7 in the probe) although `--rm` is set; the container disappears from `podman ps -a` only after it ends | probe: `podman run --rm --label catalogizer.op_id=probe1 ... sh -c 'sleep 4; exit 7'`, client `kill -9`, then `ps`/`wait`/`ps` (output: id listed after the kill, `wait` printed 7 rc 0, `ps` empty afterwards) |
| `run_pinned.sh` labels every container `catalogizer.op_id=<op id>` (line 239), so the container of one `locked.sh` call is findable by label and by nothing else | `scripts/containers/run_pinned.sh` lines 232-240 |
| A background job of a non-interactive bash starts with SIGINT and SIGQUIT ignored; a signal ignored at entry cannot be trapped | `SigIgn` of the test's locked.sh read from `/proc/<pid>/status`; the test therefore starts it through python (defaults restored) |
| bash may reap a fast-exiting background child in its SIGCHLD handler before the first `/proc` read; `wait <pid>` still returns the remembered status | deterministic repro with a slow `cat` on PATH: committed code lost the status in 12 of 12 calls, fixed code 0 of 12 |
| `flock -n 9` on an fd whose open file description already holds the lock succeeds; on a description that does not, it fails while another description holds the lock | LC-1..LC-6 |

## Defect classes and members

| Class | Members (every one has a test; RED = fails on committed 846d441f, GREEN = passes on the fix) |
|---|---|
| C1 a journal row that changed the register is dropped by replay with an OK verdict | failed row with changed hash (N1-1), minted ids (N1-2), pending `-wal` bytes (N1-3), interrupted row (N1-4), unknown id snapshot (N1-5), end to end with real containers (N1E-1); the substring skips for the regenerate marker and for `--install` that also dropped writes merely mentioning them (N1-7); golden-FALSE: a failed row that changed nothing is still skipped and listed (N1-6), real regeneration/install rows still skipped (N1-8) |
| C2 the register lock is released before the writer's write has landed | signals TERM INT HUP USR1 USR2 ALRM QUIT (SIG-*; USR1/USR2/ALRM were untrapped: RED), SIGPIPE with the real client leaving first (REAL-PIPE), a client that exits before its container (DRAIN-1..8), a container that never ends (DRAIN-10/11), podman unable to answer (DRAIN-12), SIGKILL of locked.sh (REAL-KILL, FENCE-1..5); an EXIT trap was written for "any other exit path" and REMOVED: no reachable exit exists between the fork and the drain, so it could not be tested (11.4.124) |
| C3 the lock-claim proof checks the wrong condition | fd 9 unlocked + nobody holds (LC-1), fd 9 unlocked + another holder (LC-2, the N4 bypass), fd 9 closed (LC-3), fd 9 on another file (LC-4), claimed pid not an ancestor (LC-5), golden-true (LC-6) |
| C4 the writer's exit status is lost | race (RACE-1/2), status of a container that outlived its client (DRAIN-2, `client_exit`) |
| C5 a bound that does not bound | reaper that ignores SIGTERM (RP-1), container wait bound (DRAIN-10) |
| C6 an identity or header recorded without checking the thing it names | backup under a busy checkpoint (B-2), 0-byte backup left by `die` (B-1), reconcile header of an empty view (RC-2), zero-byte id snapshot (T4-1, kills NM3), replay fallback base with pending `-wal` (T5-2, kills NM5) |
