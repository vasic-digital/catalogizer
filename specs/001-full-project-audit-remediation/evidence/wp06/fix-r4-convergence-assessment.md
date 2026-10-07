# WP06 register operations, fix round 4: convergence assessment (11.4.276, written BEFORE the fix)

| Field | Value |
|---|---|
| Round | 4 of at most 7 (the round after the structural round 3, commit 67f4d247) |
| Input | `WF15-REVIEW-register-r3.md`: NO-GO, 0 blocking, 3 important (I1, I2, I3), 7 minor (M1..M7) |
| Date | 2026-10-07 |
| Ground truth | real podman 5.7.0 (rootless, journald events backend), real `locked.sh` + `run_pinned.sh` + IMG-TESTUTIL; probes below |

## 1. Is the loop converging?

No, not by point fixes. Round 3 was declared structural, yet its three important findings are members of the SAME two classes it
declared closed (C2 "lock released before the write landed", C4 "exit status lost"). Round 3 closed the windows it could
construct (container running while the client died, signals while the client runs) and left the windows it could not name.
A fourth point round would find the next window. The structural cause is that **the writer was identified by only one of its
three parts at a time**: the script (cmdline substring, pid), the podman client (the `wait` loop) or the container (`podman ps`).
Every decision (fence, drain, status, journal) used the part that was convenient and was blind to the other two.

## 2. The defect classes, each member enumerated

### C2' "a decision about the writer is made from an identity that does not cover the writer's whole life"

The writer exists as three things with one op id. Lifecycle windows (W) x actor event (E) x observer (O):

| W | window | E that lands in it | O that decides | covered by (this round) |
|---|---|---|---|---|
| W0 | marker not yet created / marker created, no lock, no writer | SIGKILL | fence | no writer exists: nothing to fence; the marker left is the documented residual (replay refuses) |
| W1 | script alive, RUNP forked, client in preflight, container NOT created (I1) | SIGKILL of locked.sh | fence | marker flock (fd 8 inherited by the client) |
| W2 | client alive, container created, not started | SIGKILL of locked.sh | fence | marker flock + container present in ps |
| W3 | client alive, container running | SIGKILL / TERM | fence, drain | marker flock + ps |
| W4 | client dead (SIGPIPE, OOM, crash), container running | any | drain, fence | ps (root scoped) |
| W5 | container exited and was removed (`--rm`) between two polls (I2) | none (timing) | status | the `died` event (not `wait`) |
| W6 | writer gone, locked.sh after the drain: after-snapshot, journal append (I3) | TERM INT HUP | journal | handlers stay installed to the last line |
| W7 | writer gone, journal row written, marker not yet removed | SIGKILL | next fence | marker free, no container: passes (the row exists) |
| W8 | another checkout's container with the same op id (M1) | none | drain, fence, status | ps filter on the `/src` mount source; the event filter on the seen ids |

Members NOT closable: SIGKILL of locked.sh in W6 (no row for a landed write, OWED-WP06-14): untrappable; documented, the marker stays.

### C4' "the status read from a source that can lose it"

`podman wait` polls and a `--rm` container is removed shortly after it exits: probed, `wait` started after removal is `no such container` (rc 125).
Two further members found while probing the fix (not in the review): the old drain waited in 5-second slices (`timeout 5 podman wait`), and a slice that
ends at the instant the container ends loses the status (reviewer exp3 tails 5.0-5.5 are exactly that boundary); and the `died` event, the first
candidate source, is NOT always recorded when the client died before the container (probed: create/init/start/attach, no died/remove) and lags on a busy host.
Members: wait started after removal; wait in flight when the container is removed; a wait slice that ends at the container's end; container exits before the
first `ps`; events missing; events lagging; several containers with one op id. Cure: two sources, each used only when it answers (the `died` event, and ONE
`podman wait --interval 20ms` for the whole remaining budget), a bounded retry for the event only when neither answered, and when nothing answers the exit is NOT
guessed: exit 22, `container_exit_source=unknown`, marker kept. Instrument: REALSTAT (real podman, the reviewer's exp3, exit 3 expected through `| head -n1`) and
the STATUS stub cases ST-1..ST-7.

### C6' "a skip or an exemption decided from a forgeable token" (M2, M6)

`/proc/<pid>/cmdline` containing `locked.sh` (M2, removed: the marker lock replaces it) and `# register-regenerate:` in ANY argv element (M6,
now only the exact `bash -c <script>` shape with no id minted).

## 3. The instruments (control-needled, 11.4.273)

* `test_fix_r4.sh` section **SWEEP** kills the first writer at 10 offsets across its whole life with real podman and asserts the second
  writer never overlaps it. Control needle: the sweep records, per offset, whether the container existed at the kill; the section FAILs when
  fewer than one offset lands before the container exists AND fewer than one after it exists (a blind sweep reports zero in a bucket).
* **STATUS** drives the reviewer's exp3 (client dies of SIGPIPE, container exits in the poll gap) with real podman and asserts `exit` equals the container's status.
* All other cases drive the REAL `locked.sh` / `replay.sh` / `backup_db.sh`; stubs are used only where the case is pure ordering logic and the real-podman twin exists.
* Reviewer mutants MX1..MX3 and every new mutant are adopted in `mutate_register_ops.sh`; MX1's target line no longer exists (the cmdline exemption is removed),
  so its replacement mutant is "the fence ignores the marker lock" (`marker_lock_ignored`), recorded here.

## 4. What this round does not claim (UNCONFIRMED)

* Two checkouts running the SAME op id concurrently, where one container dies inside the other's window, is resolved conservatively (exit 22) rather than exactly: the `died` event carries no mount information.
* The fd-8 inheritance was probed on this host's podman 5.7.0 and crun; another runtime that passes fds to conmon would only make the fence wait longer, never overlap.
