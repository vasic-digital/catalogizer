# check_freeze_listing.sh - Companion Guide

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-12 |
| Last modified | 2026-10-12T13:00:00Z |
| Status | tracked, new (WP-20, task T161 stand-in (W2)); written by the worker, the independent review (constitution 11.4.142 / 11.4.209) is owed; nothing here writes the real register `docs/workable_items.db` |
| Source | `scripts/register/check_freeze_listing.sh`; tests `scripts/register/tests/test_freeze_listing.sh`, mutations `mutate_w2.sh freeze_listing` |

> `$EV` in this guide means the evidence root `specs/001-full-project-audit-remediation/evidence` (repository-relative).

## Purpose

Re-checks the tracked-file listing of a frozen commit (T161 rule; consumers T166, T174, T167). One argument (`freeze.json`): the listing it names must hash to the recorded
`listing_sha256` and be well formed (NUL-terminated records). Two arguments (`<listing> <manifest>`): the listing's path set must equal the snapshot manifest's path set.
It reads the listing and `freeze.json` only: no `git`, no `ls`, no snapshot file.

Refusals (stderr `check_freeze_listing: REFUSED reason=<code>`, exit 20): `freeze_listing_moved`, `freeze_listing_unrecorded`, `freeze_listing_invalid` (unreadable, empty, a record
that is not NUL-terminated, an empty path, a duplicate path). Exit 2 usage, 0 the listing is the recorded one.

## Honest boundary

T161 (the freeze) owns this file; this version is a stand-in written by the W2 worker. The `freeze.json` field names `listing` and `listing_sha256` are the stand-in shape
(UNCONFIRMED against the T161 owner).


Review fix round 2: the duplicate-path scan is one pass (`collections.Counter`, O(n)); `test_freeze_listing.sh` has 24 checks (a 20,001-record listing with one duplicate is refused in under 4 s, the refusal names the duplicate, a malformed manifest, a non-UTF-8 path and a missing manifest file each have their own reason); the mutation set has 17 mutants (the floor of 15).
