# qa_registration.sh - Companion Guide

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-11-03 |
| Last modified | 2026-11-03 |
| Status | new on branch `fix-qa-consumers`; independent review owed |
| Source | `scripts/lib/qa_registration.sh` |

## Purpose

Shell helper for QA scripts that must work with the registration policy. `REGISTRATION_MODE` defaults to `approval`: a self-registered account is created inactive and its first login answers HTTP 403 `account_pending_approval` until an administrator approves it (`PUT /api/v1/admin/users/:id` with `{"status":"active"}`). The helper registers, approves through the admin API when the account comes back pending, and confirms the account can then log in, so a QA script does not depend on how the stack was started. It also provides `qa_curl_code`, which prints the HTTP status and prints `000` on a transport failure (the old `|| echo 000` idiom printed `000000`).

## Usage

```bash
. scripts/lib/qa_registration.sh                 # defines functions only
qa_register API USER EMAIL PASS                  # prints the HTTP code (201 expected)
qa_admin_approve API ADMIN_TOKEN USER_ID         # prints the HTTP code (200 expected)
qa_register_and_activate API ADMIN_TOKEN USER EMAIL PASS   # prints the user id, exit 0 when login works
```

The admin group is CSRF-guarded: `qa_admin_approve` first reads `X-Csrf-Token` and the `__Host-csrf` cookie from a `GET /api/v1/admin/users`, then sends both with the `PUT`. Requires `curl` and `python3`.

## Which mode a QA suite should use

- A check whose subject is the registration policy: stay on the default `approval` and use `qa_register_and_activate` or `qa_admin_approve` (`scripts/run-full-qa.sh` does this; it asserts 201, login 403 while pending, approval 200, login 200).
- A check about something else that only needs a user who can log in: run the stack with `REGISTRATION_MODE=open` (`docker-compose.test.yml` sets it for the QA stack, overridable) or use the helper.
- `open` is for QA stacks only.

## Honest boundary

The helper proves the register, approve and login legs against a running API; it does not test the policy itself (that is `catalog-api/registration_approval_binary_test.go`, against the real binary).
