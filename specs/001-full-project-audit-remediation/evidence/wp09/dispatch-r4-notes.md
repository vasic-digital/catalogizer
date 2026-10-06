# dispatch round 4 notes (WF3-REVIEW event-core, fix round)

Identity (final files, sha256 prefix): see the headers of `dispatch-r4-green-x3.txt` and `dispatch-r4-sha256sums.txt`.
Evidence: `dispatch-r4-red.txt` (11 FAIL on the unfixed core), `dispatch-r4-green-x3.txt` (275/0/0 x3, host),
`dispatch-r4-mutations.txt` (125 rows, 125 KILLED), `dispatch-r4-container.txt` (IMG-TESTUTIL: 244 pass, 0 fail, 31 skip).

## Fixed
- I1 effects directory fsync checked (`effect_not_durable`), first creation of `effects/` followed by a checked fsync of the build dir (`effects_dir_not_durable`).
- I2 W1-W5: one fixture each (core refusal, plus the schema leg where the schema states the rule) and one mutant row each.
- m1 sha fields must be JSON strings. m2 `mv -fT`; `progress.json` present in any non-file form is `state_corrupt`; a directory at `callback.state` makes resume-callback exit 21.
- m3 DUP and superseded acknowledgements of a completed event re-run a callback left `claimed` or `running` (never `failed`).
- m4 escaped pre-authentication text. m6 usage line. m8 `key_unreadable` (test SKIPs with reason when run as uid 0). m10 xchk fixtures are the otherwise valid next heartbeat.
- Found by the container run (jq 1.6): an EMPTY `callback.json` made `jq -e .` exit 0, so the callback failed with `effect_key_invalid` instead of `effect_unreadable`. Fixed (`[ -s ]` check, and an empty key is `effect_key_missing`); fail-closed was never lost.
- Test hygiene: honest degradation (SKIP with the tool named, counted in `RESULT ... skip=`) for the oracle tools node, strace, gcc; core dependencies and python jsonschema are hard requirements (FAIL). The mutate script now always prints the RESULT line.
- Mutation rows whose patch text moved with the code were re-pointed (6 rows); the full sweep shows no PATCH FAILED.

## Container run (IMG-TESTUTIL, mapped uid 1000)
Present: bash, python3 (+jsonschema 4.10.3), jq 1.6, openssl, flock, realpath, awk. MISSING: node, gcc, strace.
Skipped sections (31 labels, each printed `SKIP ... (oracle tool missing: ...)`): 5 node sections (ECMA-262 oracle), 3 strace sections, 6 fsync-fault-shim sections.
A skipped section is NOT a pass; the same assertions pass on the host (275/0/0).

## Owed (not fixed in round 4; each needs the owner or a later task)
1. m9 (latent): the run binding is verified before the per-build lock. The owed resubmit/dispatcher must write `submit.json` under the same lock and re-check the verified `run_id` after `flock` (or re-verify under the lock). No current exploit (nothing in this slice writes `submit.json`).
2. Round-2 m1: the schema does not require the kind-specific fields (heartbeat pair, completed fields); the core does. The W2 "missing field" fixture therefore has no schema leg.
3. Round-2 m2 / R3 m3 remainder: a journal failure after a won claim (`journal_failed`) leaves a false audit line and an orphaned `claimed` callback until the DUP redelivery (now fixed for the consumed-mark case) or resume-callback.
4. Round-2 m3: `progress.json` is written before the journal line.
5. Round-2 m5: `secret-init` on a symlinked state path prints `secret created` but `consume` refuses it; a 0755 directory becomes 0700. Round-2 m11: concurrent `secret-init` race (UNCONFIRMED, not probed).
6. Round-2 m6: whether `resume-callback` may re-run a `failed` callback ("never retried silently" wording in T005a (p)/T005b): decision owed. Round 4 keeps DUP from retrying `failed`.
7. `effects.log` still appends through a symlink (only the journal and lock files are guarded).
8. m5 (outside this change's file scope): `evidence/wp09/SHA256SUMS` and `evidence/README.md` do not index the `dispatch-*` files; `dispatch-r4-sha256sums.txt` covers the round-4 files only.
9. m7 (owner action): map the deferred items above into tasks.md / progress.yml (T005b lines) per section 11.4.197; this round may not edit tasks.md.
10. Mutation adequacy: the full 125-row sweep ran on the host (jq of the host). The empty-`callback.json` branch is only distinguishable under jq 1.6 (container); a container mutation sweep needs node/gcc/strace for the SKIPped sections and is not run (UNCONFIRMED: no row targets `[ -s ]`).
11. Skipped sections in IMG-TESTUTIL: add node, gcc and strace to the image (or accept host-only runs for those sections).

## UNCONFIRMED
- Durability of the first `mkdir effects/` per filesystem beyond the checked fsync (host ext4/xfs behaviour not probed).
- Effort of the review that produced this fix list (xhigh as stated by the review; this fix round's own effort is not reported by the dispatch path).
