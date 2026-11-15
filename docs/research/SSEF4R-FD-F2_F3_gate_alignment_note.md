# SSEF4R-FD-F2 / F3: web route gates vs server endpoint requirements (investigation note, no code change)

Status: PROVISIONAL, read-only code reading at main 4fd58b5ac; nothing was run against a live stack (UNCONFIRMED at runtime).

## F2: `/identities` (web gate `media.view`) vs `/smb/*` (server admin-only)

- Web: `catalog-web/src/App.tsx:198` gates `/identities` with `requiredPermission="media.view"`.
- Page data: `catalog-web/src/lib/identitiesApi.ts:50` lists identities via `GET /smb/identities` (its own comment says it is the only source; create/update/remove target routes the backend does not provide).
- Server: `catalog-api/main.go:1112-1113`: `smbGroup := api.Group("/smb"); smbGroup.Use(adminGate)`, `adminGate := jwtMiddleware.RequireAdmin()` (main.go:1041). `/smb/identities` is `main.go:1120`. RequireAdmin decides from the live DB role when the admin checker is wired (middleware/auth.go:239-264); 403 "Admin role required" otherwise.
- Consequence: a user holding `media.view` but not admin passes the web gate, sees the page, and the only data call returns 403. Not a security hole (the server is the authority), a UX mismatch.
- Recommendation: gate `/identities` as admin-only on the web (same notion as the server: admin role, i.e. the web equivalent of RequireAdmin, for example the `system.admin` / `*` check used for other admin pages, to be matched to whatever the web already uses for admin routes), and hide the nav entry for non-admins. Alternative if non-admin read access is wanted: that is a server change (a read-only, secret-free route gated `media.view`), an operator decision, not a web-only fix.

## F3: `/conversion` (web gate `media.convert`) vs server `conversion.*`

- Web: `App.tsx:140` gates `/conversion` with `media.convert`. Page calls (`catalog-web/src/lib/conversionApi.ts`): `GET/POST /conversion/jobs`, `DELETE /conversion/jobs/:id`, plus formats, cancel, retry, download.
- Server routes: `main.go:1132-1141`, group `/conversion` has no route-level permission middleware; each handler checks (catalog-api/handlers/conversion_handler.go, `authService.CheckPermission`):

| Endpoint | Server requirement |
|---|---|
| POST /conversion/jobs (CreateJob, :52) | `conversion.create` |
| GET /conversion/jobs (ListJobs, :113) | `conversion.view` |
| GET /conversion/formats (:202) | `conversion.view` |
| POST /conversion/jobs/:id/cancel (:164) | `conversion.manage` |
| DELETE /conversion/jobs/:id (:226) | `conversion.manage` |
| POST /conversion/jobs/:id/retry (:262) | `conversion.manage` |
| GET /conversion/jobs/:id (:83) | authenticated + job ownership (service `GetJob(jobID, userID)`); no permission check |
| GET /conversion/jobs/:id/download (:291) | authenticated + job ownership; no permission check |

- `media.convert` (models/user.go:459) is defined but referenced by no server handler or middleware (grep over non-test Go: only the constant). So the web gate checks a permission no endpoint consults, while the three permissions the handlers do check are never used by the web.
- Server wildcard rule (models/user.go:114-136): a holder of `*` or `conversion.*` satisfies all three; `media.*` does NOT cover `conversion.*`.
- Consequence: a user with `media.convert` but no `conversion.*` sees the page and gets 403 on list/create; a user with `conversion.view` only is blocked from the page though list/formats would work.
- Recommended aligned gate: page gate `conversion.view` (the minimum the page needs to load: list + formats); inside the page, gate create on `conversion.create` and cancel/retry/delete on `conversion.manage`. Which default roles hold `conversion.*` is UNCONFIRMED: no seed in non-test Go/SQL/JSON mentions them (docs/architecture/AUTH_FLOW.md:306 only lists them), so the operator should check the role seed before changing the gate or users may lose access to the page. `media.convert` could then be removed or kept as a documented alias; that is an operator decision.

Both changes need `routeGatePermissions.test.ts` `expected` updated in the same commit (its F2/F3 comment says the values are pinned as-is, not endorsed).
