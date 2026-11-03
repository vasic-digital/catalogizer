# Upgrade Notes

Behaviour changes an operator must know about when upgrading. Newest first.

## Self-registration now requires administrator approval (2026-11)

**Breaking behaviour change.** Before, `POST /api/v1/auth/register` created an active user who could
log in at once. Now, by default (`REGISTRATION_MODE` unset or `approval`):

- the user is created **inactive and pending approval**, at the minimal role (role 2); no session is issued;
- `POST /api/v1/auth/login` with the right password answers **403** `account pending approval`
  (`code: account_pending_approval`), with a wrong password the usual 401;
- an administrator lists pending users with `GET /api/v1/admin/users?status=pending` and approves with
  `PUT /api/v1/admin/users/{id}` `{"status":"active"}` (or rejects with `"inactive"`).

What to do:

| You want | Set |
|---|---|
| New behaviour (default) | nothing |
| The old instant-active registration | `REGISTRATION_MODE=open` (explicit opt-in) |
| No self-registration at all | `REGISTRATION_MODE=disabled` (registration answers 403) |

Existing users are unaffected. Clients that registered and logged in straight away (scripts, the web
register page, integration tests) must either handle the pending state or run against
`REGISTRATION_MODE=open`. An unrecognised `REGISTRATION_MODE` value falls back to `approval` and is
logged. No database migration is needed (the pending marker lives in the user's settings JSON).
Reference: `docs/ENV_VARIABLES.md`, `docs/architecture/AUTH_FLOW.md`, `docs/api/openapi.yaml`.
