# event_core.sh — User Guide (T005b slice)

**Revision:** 6
**Last modified:** 2026-10-06T07:30:00Z

Companion guide (Helix Constitution §11.4.18) for `scripts/build/event_core.sh` and `scripts/build/lib/bev_crypto.py`.
This is a SLICE of T005b: event verification, replay guard, ordering, exactly-once terminal claim (rename of a prepared
directory) and a keyed-effect callback. The dispatcher, hub, callback runner, emitter and registry are OWED.

## Commands
`secret-init <statedir> <checkout>` (`<statedir>` must be an absolute path, else `secret_dir_not_absolute`: an option-like or relative argument never names a state dir; 0600 key outside BOTH the given checkout and this script's own checkout; refuses
`secret_dir_inside_checkout`; creates the key with `openssl rand -hex 32`, checks every step, verifies it is exactly 64
lowercase hex characters and reads it back through the same reader the verifier uses; any failure refuses
`secret_create_failed`, leaves no key, and never prints "secret created"; an existing key is reported "kept" only if it is
usable: regular file, ours, mode 0600, matching `build_hmac.key.meta`, exactly 32 bytes),
`derive-key <statedir> <build_id> <run_id>` (the `bev_crypto.py derive` and `sign` CLIs take the checkout as a mandatory last argument, so the location check cannot be skipped by omitting it; prints the per-build key: for the hub-to-stdin path only, never log or
capture it as evidence), `consume <builds_root> <statedir> <event.json>`, `cancel <builds_root> <build_id>`,
`resume-callback <builds_root> <build_id>` (re-runs a callback found `claimed`, `running` or `failed`; exit 21 when it is
not `done` afterwards).
Refusals exit 20 with `REFUSED reason=`: `secret_dir_not_absolute`, `event_malformed`, `event_unknown_build`, `event_unauthenticated`,
`event_replayed`, `illegal_transition`, `seq_conflict`, `secret_unusable`, `secret_create_failed`, `lock_unavailable`,
`state_corrupt`, `state_write_failed`, `journal_failed`, `terminal_claim_failed`, `no_terminal`. Callback failure reasons (in `terminal/callback.reason`): `effect_unreadable`, `effect_key_missing`, `effect_key_invalid`, `effects_dir_unavailable`, `effects_dir_not_durable`, `effect_not_durable`, `effects_log_write_failed`, `effect_temp_write_failed`, `effect_not_applied`. A secret file that cannot be read is `secret_unusable key_unreadable` (never `event_malformed`). Text taken from an unauthenticated event (field and key names in `event_malformed` messages) is printed as ASCII with control characters and non-ASCII escaped, at most 64 characters. Other outcomes (exit 0):
consumed, `DUP`, `late_ignored`, superseded (a lost race, only when a whole terminal record exists).
A refusal never acknowledges anything: nothing is written as consumed, so the hub keeps the event and redelivers it.

## How an event is consumed
1. The event file is read exactly ONCE (`bev_crypto.py verify-event`, stdin). That one read is parsed strictly (single
   JSON object, no duplicate keys, no NaN, at most 64 KiB), validated against the `build-event/1` contract (types, enums,
   required fields per kind, no extra fields, integers base-10 within 0..2^53-1, floats and booleans refused, `build_id`
   matching `^[A-Za-z0-9_-]{1,128}$` and the hex fields `^[0-9a-f]{64}$`, both matched with `fullmatch`: a trailing newline is NOT accepted although Python `$` would match before it), and its HMAC verified against the key derived for the build's recorded `run_id`.
   Every value the script then acts on (build, run, seq, kind, exit_class, offsets, journal line) is taken from that
   verified object, never from the file again (a FIFO or a swapped file cannot change what was verified). The build id bash
   then uses must itself be a plain token `^[A-Za-z0-9_-]{1,128}$` (else `event_malformed build_id`), so the directory acted on is the
   verified one byte for byte.
2. Under the per-build `flock` (fail closed if the lock file cannot be opened or locked) the core applies, in order:
   duplicate check (`consumed/<seq>` holds `sha256(canonical event) time`; identical redelivery is `DUP`, a different
   event reusing the seq is `seq_conflict`), `late_ignored` once a terminal exists (a terminal directory that exists but is empty or unparsable is `state_corrupt`, never
   `late_ignored`; `cancel` refuses the same state), then the ordering rules (seq above the
   last consumed one, the first event is `accepted` with seq 1, `accepted` only once, heartbeat `progress_offset` and
   `stage` never going backwards). The event is also bound to the build's `submit.json`: its `run_id` and `variant` must
   be the recorded ones (a correctly signed event of an earlier run is `event_replayed`; any other mismatch is
   `event_malformed`).
   The "last consumed" number is computed only from names that are plain decimal numbers in `consumed/`; temp files live in
   `<build>/tmp/`, so a crash leftover can never blind the check.
   A DUP or `superseded` acknowledgement of a `completed` event re-runs a terminal callback left `claimed` or `running` (a refusal after a won claim, for
   example a failed consumed mark, no longer leaves an orphaned callback); a `failed` callback is NOT retried there (only `resume-callback` re-runs it). The state is re-checked under `.cblock` as well (round 5, N3): a DUP that waited for the lock does not retry a callback the lock holder marked `failed` meanwhile.
   A sha field (`artifact_manifest_sha256`, `remote_log_sha256`) must be a JSON string: a 64-digit JSON number is `event_malformed completed fields`.
3. A `completed` event keeps the lock through the terminal claim. The terminal is prepared as
   `<build>/terminal.tmp-<pid>-<start-time>/` (holding `state.json` and `callback.state` = `claimed`) and renamed with
   `mv -T` onto `<build>/terminal/`, the single serialisation point (T005b naming). The claim has four outcomes: won
   (journal `terminal_claimed`), lost (a whole terminal record exists: journal `superseded`), failed (any other cause, for
   example a full or read-only disk: refused `terminal_claim_failed`, no consumed mark, redelivery retries) and won-but-not-
   journaled (refused `journal_failed`; the terminal exists and a redelivery is a `DUP`). A failed claim is never reported
   as a lost race. `cancel` takes the same per-build lock, so its claim and its journal lines are serialised with the
   consumers; whichever of cancel and consume holds the lock at the claim wins, the other is superseded or `late_ignored`.
4. The callback state is its own file, `terminal/callback.state` (`claimed`, `running`, `done` or `failed`; the reason of a
   failure is in `terminal/callback.reason`). The callback runs under its own lock `.cblock` and re-reads the state after
   taking it, so a runner that waited for the lock does not re-enter a callback another runner finished. The effect is
   keyed (a checked fsync of the build directory runs on EVERY attempt once `effects/` exists, round 5 N1, so a retry after `effects_dir_not_durable` must succeed at it before `done`; and the fsync of `effects/` after the effect file is checked: a failure is `failed` `effects_dir_not_durable` / `effect_not_durable`, never `done`; `callback.json` must be readable JSON with a string `effect_key`, else the callback is `failed` `effect_unreadable` / `effect_key_missing`, never `done` without an effect; `resume-callback` re-runs it and repairs it once the record is repaired, otherwise it exits 21; `effect_key` must match `^[A-Za-z0-9_-]{1,128}$`, else the callback is `failed` `effect_key_invalid`): the audit
   line in `effects.log` is written once per key (literal whole-key match) before the effect file is created with `ln`, so
   a crash between them is repaired on resume with exactly one line. If `ln` fails and no effect file exists the callback
   is `failed` `effect_not_applied`, never `done`; `effects.log` is an attempt audit, the effect file is the proof.
5. Fail closed on every write: each writer (journal, consumed mark, `progress.json`, `callback.state`, the terminal record,
   the secret) checks its exit status; a temp file that was not completely written and synced is removed and never renamed
   over durable state. The journal line is written with ONE `write(2)` (a line larger than the stdio buffer is never
   split) through `O_APPEND|O_NOFOLLOW`; a short write or a failed fsync is rolled back (truncate to the previous size) and a torn
   last line left by a crash is cut back before the next append, so the journal stays one JSON object per line. Files and
   directories are synced with a REAL `fsync(2)` (python `os.fsync`; `sync FILE` on this host is uutils = a global `sync(2)` with no
   error to report) and every fsync is checked, including the effects directory (a directory only tolerates EINVAL/ENOTSUP, a filesystem without directory sync);
   a failed fsync refuses and nothing is acknowledged. State files are renamed with `mv -fT`, so a directory sitting where a state file belongs makes the write fail (`state_write_failed`) instead of swallowing it; `progress.json` present in any form that is not a readable file is `state_corrupt`. Lock files are
   opened for append, never truncated, and a lock path that is a symlink is refused (`lock_unavailable`).

## Canonical JSON form (what is signed)
HMAC-SHA256 over the event WITHOUT `hmac`, serialised as Python `json.dumps(sort_keys=True, separators=(",", ":"),
ensure_ascii=False)` in UTF-8: compact, keys sorted by code point, non-ASCII raw, mandatory JSON escapes only, integers in
plain decimal, no floats. `jq -cS` produces the same bytes ONLY for the fixture of the test (a non-ASCII event with quote,
backslash, slash, tab and an emoji, also signed using only `jq` and `openssl` and accepted by the core); it is not
equivalent in general: for U+007F (DEL) `jq -cS` emits the escape `\u007f` and Python emits the raw character (measured), so
an emitter must reproduce the Python form and must not rely on `jq -cS` for arbitrary text. The key is HKDF-SHA256(driver secret, empty salt,
info = len4be(build_id) || build_id || len4be(run_id) || run_id). The owed emitter MUST reproduce this form exactly; a
cross-implementation fixture from the real emitter is OWED to T005b.

## Secret handling
The secret is read from the key file only; no command line carries it. Reads refuse (`secret_unusable <why>`): a symlinked
state directory or key, a state directory inside the checkout, a key that is not a regular file owned by this user with
mode 0600, a `build_hmac.key.meta` that is absent or whose `sha256` differs from the key, a key that is not lowercase hex
(`key_not_hex`) or not exactly 64 hex characters = 32 bytes (`key_bad_length`: an empty or short key would make every event
forgeable). Only the file's name and
location are documented.

## Honest boundaries (§11.4.6)
- Authenticity of an event is "someone holding the per-build key signed these exact bytes", nothing more. The replay guard
  covers events of an earlier run of the same build id (`event_replayed`) and the same seq with other content
  (`seq_conflict`); it does NOT cover freshness (`sent_at` is not checked) nor an attacker who holds the driver secret.
- UNCONFIRMED here: the key owner check (`key_wrong_owner`) because the host has a single uid, and the refusal to read or
  create the secret inside any path RUNP mounts (RUNP is not part of this slice). Both are OWED.
- A deleted or replaced key with a stale `.meta` is detected as `meta_sha_mismatch`; the `driver_secret_lost` recovery
  path of T005b is OWED.
- Residual: an event is journaled before its consumed mark is written, so a failure between the two (refused
  `state_write_failed`) leaves the journal line and a redelivery journals it again (the effect is unaffected: the claim and
  the consumed mark are idempotent). UNCONFIRMED by a test: the `journal_failed` refusal on the `superseded` line (the race
  that reaches it is only reproducible with the hold wrapper, and no fault was injected there); the other two journal
  failure sites are tested.
- Contract caps: `host`, `sent_at` and `image_digest` are at most 1024 characters in the schema and in the core; the raw
  event is at most 65536 bytes (inclusive), refused before parsing (the schema cannot express a byte cap).
- OWED (T005a/T005b, not in this slice): the dispatcher, hub, runner, ssh shim, liveness/poll, artifact verification,
  resubmit, snapshot, and the cases listed in the test's `OWED_CASES`.

- Round-4 status (WF3-REVIEW): fixed I1 (effects directory fsync), I2 (W1-W5 fixtures and mutant rows), m1 (number-typed sha), m2 (`mv -fT`, directory
  state files), m3 (DUP/superseded re-run a claimed or running callback), m4 (escaped pre-authentication text), m6 (usage line), m8 (`key_unreadable`),
  m10 (isolated `xchk` fixtures). Owed (not fixed in round 4, recorded in `evidence/wp09/dispatch-r4-notes.md`): the schema is laxer than the core on
  kind-specific fields (it does not require the heartbeat or completed fields); the latent run-binding TOCTOU (m9: the owed resubmit/dispatcher must
  write `submit.json` under the per-build lock and re-check the verified `run_id` after `flock`); the won-but-unjournaled false audit line (a journal
  failure after a won claim, `journal_failed`); mapping the deferred items into tasks.md / progress.yml (m7, owner action); `SHA256SUMS` / `README`
  index of the `dispatch-*` evidence (m5, outside this change's file scope).
- Round-3 not addressed (minors of the round-3 review, tracked as owed): the schema is laxer than the core on kind-specific fields (m1);
  a won-but-unjournaled claim leaves a false audit line and an orphaned `claimed` callback, recovered only by the owed restart path (m2; the orphan after a consumed-mark failure is fixed in round 4, the journal-failure case is not);
  `progress.json` is written before the journal line (m3); `progress.json` is read only if it is a regular, non-symlink file of at most 4096 bytes (opened `O_NONBLOCK|O_NOFOLLOW`, judged by `fstat`); a FIFO, device, directory or symlink there is `state_corrupt`, never read, so it cannot block `consume` while it holds the per-build lock (round 5, N2). Owed (mutation adequacy, review V1): the `_safe` escape of C1/bidi characters has no paired row; no row targets the `[ -s callback.json ]` guard. `secret-init` still changes the mode of a pre-existing directory and accepts a
  symlinked state path where the reader refuses it, and is not atomic against a concurrent creator (m5, m11 UNCONFIRMED); `resume-callback`
  re-runs `failed` callbacks, which T005a (p)/T005b word as "never retried silently" (m6, decision owed); `effects.log` still appends through a
  symlink (only the journal and lock files are guarded).
- The schema pattern oracle in the test is node (ECMA-262 `RegExp`), because python-jsonschema evaluates `pattern` with `re.search`
  and says VALID for a trailing-LF value that an ECMA engine rejects.

## Tests
`scripts/build/tests/test_dispatch_events.sh` (`EC_SCRIPT` overrides the script under test). The deterministic race test
uses a wrapper that sources the script and wraps `claim_terminal` (both for consume and for cancel); the shipped script has
no environment-variable seam (sourcing it defines functions and runs nothing). Fault tests put shims for
`jq`/`ln`/`mv`/`openssl` first on PATH (they fail, truncate or log calls whose arguments match a pattern), an `LD_PRELOAD` shim
compiled by the test fails `fsync(2)` with EIO on a chosen path (control-needle checked), `ulimit -f` produces short journal writes, and
`strace` proves the single-write and the fsync calls (temp file before its rename, journal, directories). `scripts/build/tests/mutate_dispatch_events.sh list|all|<name>` applies
each paired mutation to a copy and requires the test to FAIL. Honest degradation: the test needs python3, jq, openssl, flock, realpath, awk and python3 `jsonschema` (a missing one is a FAIL with the tool named); the oracle tools `node` (ECMA-262 pattern oracle), `strace` and `gcc` (the fsync fault shim) are optional, and a section that needs a missing one reports `SKIP ... (oracle tool missing: <tool>)` and counts it in `RESULT pass= fail= skip=`, never a pass. IMG-TESTUTIL has none of node, strace, gcc (recorded in `dispatch-r4-container.txt`). Evidence (round 4, supersedes the round-3 pointers, which were made with an earlier test file): `evidence/wp09/dispatch-r4-red.txt`, `dispatch-r4-green-x3.txt`, `dispatch-r4-mutations.txt`, `dispatch-r4-container.txt`, `dispatch-r4-notes.md`, `dispatch-r4-sha256sums.txt` (final-file round 3: `dispatch-r3-green-x3-final.txt`, `dispatch-r3-mutations-rerun.txt`; earlier rounds: `dispatch-red.txt`, `dispatch-green-x3.txt`, `dispatch-mutations.txt`).
Index row in `docs/scripts/README.md`: owed (T036, index absent).
