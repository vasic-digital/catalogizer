# nfs_fallback_state.sh - Companion Guide

| Field | Value |
|---|---|
| Revision | 3 |
| Created | 2026-10-06 |
| Last modified | 2026-10-07T16:40:16Z |
| Status | WF17 fix round 5 applied in the working tree (not committed); the independent review of that round is owed (constitution 11.4.142 / 11.4.209); evidence: `specs/001-full-project-audit-remediation/evidence/wp12/wf17/` |
| Source | `scripts/test-infra/nfs_fallback_state.sh`; tests `tests/infra/test_nfs_fallback_state.sh` |

## Purpose

Records the terminal state of the owner-host NFS fallback leg (T134a, ODG-08): `not_needed` only when the T134 record's `state` field is `pass` and it proves the kernel path (see the WF12 section) (the record cites that file's sha256 and state);
`blocked` with reason `odg08_unanswered` otherwise.

## Usage

```bash
scripts/test-in-container.sh tooling unit -- bash /src/scripts/test-infra/nfs_fallback_state.sh --attempt-json F --state not_needed|blocked [--out F]
```

Refusals (exit 1): `nfs_attempt_not_pass` (`not_needed` claimed while the T134 state is not `pass`), `fallback_not_owed` (`blocked` claimed while it is `pass`), `attempt_record_missing`, `attempt_record_malformed`.

## WF12 review fixes (revision 2)

- F7: a user-space protocol `pass` no longer makes the owner-host leg `not_needed`. `not_needed` needs `state == pass` AND `proves_kernel_mount_path == true` in the T134 record (no producer of that field exists on this host); a protocol-only pass is REFUSED `nfs_pass_does_not_cover_kernel_mount`, and `--state blocked` for it records `blocked` / `odg08_unanswered` with `unconfirmed` naming the application kernel NFS mount path. `blocked` is refused (`fallback_not_owed`) only when the record proves the kernel path.

## WF17 fix round 5 (revision 3)

- The attempt record must be exactly what `nfs_terminal_state.sh` writes (schema `nfs-attempt/1`, a state of the closed set with its own reason, `proves_kernel_mount_path` absent or a real boolean: the string "true" is not true); `derived_from` names the real path of the input. `not_needed` is refused for a terminal state that is not `pass` (`nfs_attempt_not_pass`, exit 1).
