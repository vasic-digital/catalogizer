# fix-r2 convergence assessment (11.4.276, written BEFORE the fix pass)

| Field | Value |
|---|---|
| Round | 2 of the constitution-0108 item (round 1 = author + Opus review NO-GO: 10 source / 5 test / 7 process-doc findings) |
| Budget | R_max 5-7; this is round 2, so no stop/escalation trigger is reached |
| Round-3 structural trigger (11.4.276(E)) | not met: no finding reappeared across rounds (round 1 was the first review) |

## Class-complete grouping (11.4.276(C)), instead of 10 point fixes
Root cause of S1, S2, S4, S5, S6, S7, S8 is ONE class: "the floor value / limits file is parsed ad hoc, with no grammar, no source-type check and unvalidated values reaching shell arithmetic and echo".
Remedy: one resolver (`resolve_mem_floor`) with a strict grammar enforced BEFORE any arithmetic, over EVERY input class (env value, file value, file type, file size, key multiplicity, key nesting, echo), plus a test table covering each member.
S3 (heap invariant) is derived from the heap rule itself (lower bound = 2 x HEAP_FLOOR_MB, one shared constant), not a second magic number.
S9 (precedence) and S10 (default output) are design decisions: env may only RAISE; lowering only through the tracked limits file (discovered at a project-relative convention path); no knob => byte-identical lines.
Test class (T1-T5): the memory dimension is isolated with a redirected-meminfo COPY of the tool and a stub df, so verdicts do not depend on host memory/disk; a REAL live writer is created for U60 with a no-writer control.
Governance class (P1-P7): claims are made only from measured facts (counts, ledger before/after, engine probes) and provenance is stated as relayed + wording pending.

## Reviewer mutants
All 15 reviewer mutants (R01-R15) are adopted with their semantic edit; anchors are re-mapped because the code they edited was rewritten (the originals' anchors no longer exist). 9 more (R16-R24) cover the new classes; R25 is an identity negative control.
