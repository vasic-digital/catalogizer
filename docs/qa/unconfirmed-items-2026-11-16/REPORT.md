# V2_5_0_GATE_CHECKLIST UNCONFIRMED important items: current truth (2026-11-16)

Status: PROVISIONAL. Branch `dial` in a scratch clone of main `868b0d379`. Nothing pushed. Method: finding text in `docs/FOLLOW_UPS_NEXT_PHASE.md`, `git log -S/--grep`, code reading, then (11.4.292) a live RED before any fix.

| Item | Verdict | Commit / evidence |
|---|---|---|
| SEC-DIAL-1 (a) SMB, (b) FTP, (c) NFS dials unvetted | CLOSED-by `a0419bd28` (merge `cef6d4832`; follow-ups `b001ac4bf`, merge `c76c300d3`; NFS3 `73c4c6dee`) | SMB/FTP/NFS dial through `storageDial` / `VetSMBHostIPs` (`internal/services/smb.go:80`, `filesystem/ftp_ops.go:93`, `ftp_client.go:564`), create-time vetting in `filesystem/settings.go`, `storage_host.go`. Reviews GO-with-follow-ups, unit-class only. |
| SEC-DIAL-1 (d) `submodules/security/pkg/ssrf/guard.go` lacks 198.18/15, CGNAT, 168.63.129.16, Teredo | STILL-OPEN, NEEDS-DECISION | File is in the `security` submodule (own repository, pin `0513361`); grep of `guard.go` finds none of those ranges. catalog-api storage dials use `internal/netguard` instead. Fix = commit in another repo + pin bump: operator decision. SEC-DIAL-1-FU-M1..M4 stay open. |
| LOGSHARE-TOK-1-F1 (v28 migration order) | CLOSED-by `b91ecc76d` | `database/migrations.go:42-43`: v28 `invalidate_legacy_log_share_tokens`, v29 `create_login_throttle`; `migrations_v28_test.go`, `migrations_v29_test.go` exist. |
| CAT-SMB-COPY-1 | FIXED-here `6ca6f18f1` | Live RED against a real samba `smbd` (`smb_copy_RED.txt`): same-path copy returned nil and emptied the source; case alias emptied it; dir destination gave a raw server error; dir source created an empty file. GREEN `smb_copy_final.txt` (8 live tests). Mutants `smb_copy_mutants.txt`. |
| CAT-SYNC-404-1 | FIXED-here `6aa67f380` (+ stale tests `1d9a00299`) | Live-stack RED `sync404_RED.txt` (stranger: 403 for foreign id vs 404 for missing id on 6 handlers), GREEN `sync404_GREEN.txt`. All `/sync` routes sit behind `RequireAdmin` (`main.go:1380-1389`, admins hold `*`), so a non-admin cannot reach this today: defence in depth, reproduced at handler level. |
| UI-FAV-1 | CLOSED-by `bf8811503`, `474ea594b`, `ccc4761ac` (merge `4f321e0cf`; FAVR3 `cf45e4bb6`, `4268d6fcd`) for the original report; residual STILL-OPEN | `attachEntityLinks` (`media_browse_handler.go:98,112`) + `FavoriteToggle`. Residual: playlist toggles/`PlaylistGrid` id space, bulk favourites default type, live Playwright, PostgreSQL run. Register row `WORKABLE_ITEMS_REGISTER.md:33` still "Queued" (stale, not edited). catalog-web tests NOT run (no node_modules). |

## Not run / UNCONFIRMED
- catalog-web vitest; live Playwright for UI-FAV-1.
- SMB live tests used one case-folding samba server; case-sensitive/Windows servers untested. A crash between remove and rename leaves the finished copy as `.copy-*.tmp` (documented in code).
- CAT-SYNC-404-1 through production routes (admin gate) and on a deployed stack.
- Surviving SMB mutants M5 (final ctx check) and M7 (per-chunk ctx check): a cancel inside one chunk window cannot be forced deterministically. Surviving sync mutant S7 (not-found text) is equivalent at HTTP level (body never carries error text).
