# Stack 3 LIVE re-verification 2 (CAT-QA-STACK3-LIVE2)

Date: 2026-11-05 (UTC ~11:48-12:00). Target: QA stack 3 (pod catqa3), API image `main-5c480072`,
`/health` build_number `5c480072b2d5cda2102b7091a8f7422d941f24e2`, API http://127.0.0.1:38082.
Real running stack, real PostgreSQL, real binary; no stubs or mocks (§11.4.289). Tokens, passwords and
the throwaway user's password are not stored; password fields are redacted in request logs.
Evidence files are in `live2-evidence/` (`<name>.txt` = request + status + timestamp + body head;
`<name>.body` = full response body), hashes in `MANIFEST.sha256`.

| # | Item | Label |
|---|------|-------|
| 1 | SECQ-1 WebDAV SSRF guard | VERIFIED |
| 2 | SECQ-5 GET smb/discover,/test retired | VERIFIED |
| 3 | SECQ-3 pprof | UNCONFIRMED (admin/non-admin gate not exercisable) |
| 4 | SECQ-4 / 4b cap of 50 | VERIFIED |
| 5 | Subtitle delete non-owner | VERIFIED |

## 1. SECQ-1 (VERIFIED)
`POST /api/v1/storage/roots` as admin, protocol webdav, url per case, 3 repeats each (`secq1_<case>_<n>`):
- loopback `http://127.0.0.1:38082/` -> 400 x3, `webdav url refused: outbound destination not allowed: 127.0.0.1 is a loopback, ...`
- `http://localhost:38082/` -> 400 x3 (resolves only to loopback)
- link-local `http://169.254.169.254/` -> 400 x3
- metadata `http://[fd20:ce::254]/` -> 400 x3
- IPv4-mapped `http://[::ffff:127.0.0.1]/` -> 400 x3
- RFC1918 LAN `http://192.168.1.115:33021/` -> 201 x3 (not blocked by the guard)
Roots listing before (`secq1_pre_list`) and after blocked attempts shows no ssrf root was created.
Cleanup note (FAIL-free but worth knowing): the API has NO delete route for storage roots
(`DELETE /api/v1/storage/roots/{id}` -> 404, `secq1_cleanup_del_*`), so the 3 LAN roots created were removed by
`DELETE FROM storage_roots WHERE name LIKE 'qa-lan-ok-%'` in catqa3-postgres (`secq1_cleanup_sql.txt`, DELETE 3);
final list (`secq1_final_list2`) shows only qa-live-corpus and qa-corpus.

## 2. SECQ-5 (VERIFIED)
- `GET /smb/discover` and `GET /smb/test` as admin -> 410 x3 each; body `credentials in the URL are no longer accepted; use POST ...`.
  With credentials in the query string -> still 410 (`secq5_get_discover_querycreds`).
- GET as non-admin -> 403, unauthenticated -> 401.
- POST as non-admin -> 403 (discover and test).
- POST as admin: `/smb/test` with body -> 200 `{"connection":false,...,"success":false}` x3 (unreachable TEST-NET host);
  `/smb/discover` without username/password -> 400 validation x3; with credentials -> 500 `dial tcp 192.0.2.1:445: i/o timeout`
  (host unreachable by design; behaves as spec'd for a failing network, not a handler fault).

## 3. SECQ-3 pprof (UNCONFIRMED)
`/debug/pprof/`, `/heap`, `/cmdline` with no token, non-admin token and admin token: all 404 (nine files `pprof_*`),
consistent with `HELIX_PPROF_ENABLED` unset (routes not registered; stack not restarted per instructions).
Verified live: pprof is not exposed. NOT verified live: the 401/403 gate on the registered routes and the 60 s sampling guard
(requires a stack with the flag set); the source registers RequireAuth + RequireAdmin + PprofSamplingGuard. Needs a flagged stack.

## 4. SECQ-4 / SECQ-4b (VERIFIED)
Throwaway non-admin user `qa_tmp_user1` (id 302, role 2, created via admin `POST /users`, deleted afterwards: `user_delete` 204, `user_after` 404).
51 parallel creates (xargs -P16) of smart playlists (`POST /playlists`, is_smart, rule year>1900) and smart collections (`POST /collections`),
three independent rounds each (`cap_pl_run1..3.txt`, `cap_co_run1..3.txt`):
every round 50 x 201 + 1 x 409; stored rows per owner read from PostgreSQL = 50 (`cap_*_summary_run2/3.txt`, `cap_*_post1.txt`).
Additional burst of 16 parallel creates on a full (50) state: 16 x 409 for both (`cap_*_full_state_burst.txt`).
Pre-state 0/0 (`cap_pre.txt`); cleanup via API DELETE (50 x 200 each round) left 0/0 (`cap_cleanup_after_run*.txt`).
Count never exceeded 50 in 3/3 rounds.

## 5. Subtitle contract (VERIFIED)
`DELETE /api/v1/subtitles/999999991` as non-owner user -> 404 `SUBTITLE_NOT_FOUND` x3; as admin -> 404; unauthenticated -> 401.
Stronger: `DELETE /api/v1/subtitles/1` (an EXISTING track, not uploaded by the user) as non-owner -> 404 x3, and the row is still present in
`subtitle_tracks` afterwards (`sub_existing_still_present.txt`: count 1, total 18).

## Failures
None. Caveats: item 3 UNCONFIRMED as above; storage roots needed a DB cleanup because no delete API exists (observation, not a spec claim).
