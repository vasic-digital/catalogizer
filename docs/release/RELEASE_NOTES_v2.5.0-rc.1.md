# Catalogizer v2.5.0-rc.1 (PRE-RELEASE, not production-ready)

**This is a labelled pre-release candidate. Do not deploy it to production.** The production release is v2.5.0 and keeps all gates G1..G12.

## Operator exception (rc.1 only)
Operator decisions of 2026-11-17 (relayed, identifier UNCONFIRMED; see `docs/release/RC1_PREP_AUDIT_20261116.md`, "Operator exception for rc.1") allow rc.1 to ship as a labelled pre-release with gates G2, G5 and G9 relaxed, an exception to constitution 11.4.298(E) for rc.1 only. It relaxes nothing for v2.5.0.

## Release gate state per product (11.4.298)
- api: BLOCKED-NO-HARDWARE
- web: BLOCKED-NO-HARDWARE (the v2.4.2 baseline in `docs/qa/baseline-v2.4.2-20261114/` is not compared: no rc.1 live run exists)
- desktop: BLOCKED-NO-HARDWARE
- wizard: BLOCKED-NO-HARDWARE
- Android: BLOCKED-NO-HARDWARE
- Android TV: BLOCKED-NO-HARDWARE

Not run for this candidate: the 11.4.40 full-suite retest, live UI/Vision QA, SonarQube scan, live /ws run, and 11.4.185 manual QA.

## What is in it (all PROVISIONAL: implemented and wired, verification pending)
VERIFIED = 0 items. PROVISIONAL = about 243 Roadmap ids since v2.4.3 (UNCONFIRMED exact; see `docs/CHANGELOG.md` `[2.5.0]`). REOPENED = none identified (UNCONFIRMED). OVERDUE = 2 verification jobs (VJ243-21, VJ243-22).

User-visible: per-user favorites (`PUT /api/v1/media/:id/favorite`, body `favorite`, schema migration v30; old global flags are not migrated), new `GET /api/v1/entities/:id/user-metadata`, sync destination private-network confirmation answers 400 (was 409), `/sync/*` GET routes stay 403 for non-admins, plus authentication, WebSocket, egress and error-leak hardening.

Upgrade: back up the database first; there is no downgrade. See `docs/UPGRADE_NOTES.md`, "Upgrading from 2.4.3 to 2.5.0". Version files carry `2.5.0` (the checker cannot hold a pre-release suffix); the pre-release identity is the tag `v2.5.0-rc.1`.

## Known issues
- ANDROID-RETROFIT-MAP-1 (Important): `CatalogizerApi.setFavoriteStatus(@Body Map<String, Any>)` Retrofit body issue found by wire tests; the fix is PROVISIONAL (unit tests green, no device run).
- Android/Android TV favorite toggle on the new endpoint: PROVISIONAL, not verified on a device.
- Overdue verification jobs VJ243-21 and VJ243-22.
- No live hardware run for any of the six products.

Full list: `docs/CHANGELOG.md` section `[2.5.0]`.
