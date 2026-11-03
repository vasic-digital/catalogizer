# Legacy SHA-256 password hash: sunset plan

| Field | Value |
|---|---|
| Created | 2026-11-03 |
| Status | plan; metric and log shipped, verify path NOT removed |
| Owner area | catalog-api auth (`services/password.go`, `services/auth_service.go`) |

## What the legacy shape is

The old `POST /users` stored `hex(SHA-256(password + salt))` (64 lowercase hex
characters). `CheckPassword` still accepts it and reports `needsUpgrade=true`, and
`AuthService.Login` rewrites the row to the canonical `$cz1$` bcrypt hash with a
compare-and-swap (`UpdatePasswordIfHash`). A legacy row is therefore upgraded at that
user's next successful login. Users who never log in keep the weak hash.

## What is measured now

At startup (after the admin seed) catalog-api counts the rows still in the legacy shape
(`LENGTH(password_hash) = 64 AND password_hash NOT LIKE '$%'`), sets the Prometheus gauge
`catalogizer_auth_legacy_password_hashes`, and logs one warning when the count is above 0
(`main_legacy_hashes.go`, covered by `main_legacy_hashes_test.go` on a real SQLite DB).
The gauge is a startup snapshot; restart the service to refresh it.

## Sunset steps (no behaviour is removed by this change)

1. Ship the metric and log (done).
2. Watch the gauge across releases. Legacy rows only shrink by logins, so a stable non-zero
   value means dormant accounts.
3. Operator action for dormant accounts: force a password reset for each remaining legacy
   user (admin password reset, which writes a `$cz1$` hash).
4. When the gauge has read 0 on every deployment for at least one full release cycle,
   remove the `isLegacySHA256Hex` branch from `CheckPassword`, the upgrade hook in
   `AuthService.Login`, and the legacy tests, in one reviewed change. After that a 64-hex
   hash simply fails verification, so any remaining legacy account needs a reset.
5. Do not remove the verify path while the gauge is above 0 on any deployment you support.

## Honest limits

* The count is per database at startup, not per day; it does not say which users are dormant.
* Removing the verify path (step 4) is a deliberate lock-out of any account still legacy; it is
  gated on the metric reading 0, not on a calendar date.
