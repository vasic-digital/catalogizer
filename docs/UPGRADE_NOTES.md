# Upgrade Notes

Behaviour changes an operator must know about when upgrading. Newest first.

## Known limitations and pending removals (docs sync C4, 2026-11-03)

- **Collection sharing is not available.** The server has no `/collections/:id/share*` routes, so the web UI's Create Share Link, Send Invitations, QR Code and Copy Embed Code controls are disabled with a stated reason (`catalog-web/src/components/collections/CollectionSharing.tsx`, `SHARE_API_AVAILABLE`).
- **Smart playlists and smart collections are available** (see `docs/guides/SMART_COLLECTIONS_GUIDE.md`); they replace the former 501.
- **Conversion tools are part of the API image.** `ffmpeg` and `imagemagick` are installed in the runtime image; see `docs/deployment/DOCKER_SETUP.md`.
- **go-fitz (AGPL, MuPDF) is removed (D3, 2026-11-04, PROVISIONAL-DONE).** Per the operator decision of 2026-11-03 the dependency is gone from `catalog-api/go.mod`, `go.sum` and the code; PDF page count and page rendering use poppler (`pdfinfo`, `pdftoppm`) as isolated child processes (`catalog-api/services/pdf_page_render.go`), so `poppler-utils` (and `prlimit` from util-linux) must be installed on the API host. No MuPDF code remains in the binary, so there is nothing AGPL-licensed left to attribute. Live re-run owed: see `docs/FOLLOW_UPS_NEXT_PHASE.md` (D3 section).

---

## Smart collections and playlists (2026-11)

- Rules are evaluated at read time; per-user caps: smart collections and smart playlists each answer **409** when the cap is reached.
- **Collections without an owner (`created_by` NULL) are now editable by administrators only.** Before, any user could edit them.
- Foreign **private playlists answer 404** on GET, PUT, DELETE and item routes (no longer 403/unauthorized), so existence is not leaked; a foreign *public* playlist answers 403 on write.
- Smart rule `favorite` reads the `favorites` table (the store the UI writes), per viewer.

---

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

## Smart collections: legacy collections become admin-edit-only

Migration 23 adds `owner_user_id` and `is_public` to `media_collections`. Collections created before it
have no owner (`owner_user_id` NULL). They stay readable by every authenticated user, but only an
administrator can edit or delete them; an owned collection is editable by its owner and by administrators.
To hand a legacy collection to a user, set its `owner_user_id` (administrator action). Covered by
`catalog-api/handlers/collection_ownerless_test.go`.
