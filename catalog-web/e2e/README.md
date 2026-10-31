# catalog-web E2E tests

Run with `npm run test:e2e` from `catalog-web/`.

## Required environment variables

There are no literal credential defaults. A run fails with a message naming
the variable when a required one is unset or empty.

| Variable | Used by | Notes |
|---|---|---|
| `ADMIN_PASSWORD` | `fixtures/auth.ts` (mock admin/test user), `tests/browsing-challenge.spec.ts` (real backend login) | Required. For the real-backend suite it must be the actual admin password; for mocked suites any non-empty value works. |
| `ADMIN_USERNAME` | `tests/browsing-challenge.spec.ts` | Optional, defaults to `admin` (not a secret). |

Export values in your shell or a gitignored `.env`; never commit them.

The unit check for this rule is `src/e2e-env.test.ts` (vitest).
