# qa availability probes - Companion Guide

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-06 |
| Last modified | 2026-10-06T20:45:00Z |
| Status | new in the working tree (WP-24), not yet committed; independent review owed (constitution 11.4.142, T219); its row in `docs/scripts/README.md` is owed |
| Source | `scripts/qa/probes/*.py` (T216); tests `scripts/qa/tests/test_probes.py` |

## Purpose

One real, independent availability probe per dependency (doc12 section 10.3): `service.py` (answers AND reports its own build id), `credential.py` (a variable NAME, optionally a real login), `device.py` (adb state, `ro.serialno` identity, app package: emulators only), `nas.py` (TCP + a real share listing; the password reaches smbclient through `PASSWD`, never argv), `metadata.py` (one authenticated provider call), `vision.py` (`HELIX_VISION_HOSTS` health), `runtime.py` (rootless podman). Each prints one JSON line `{probe, status present|blocked, reason, evidence}` and exits 0 / 1 / 2. `reason` is from the closed set of document 06 (`service_unreachable`, `credential_absent`, `credential_rejected`, `device_absent`, `device_wrong_identity`, `device_unauthorised`, `geo_restricted`, `quota_exhausted`, `licence_absent`, `host_resource_unavailable`).

## Usage

```bash
python3 scripts/qa/probes/service.py --url http://HOST:PORT/health
python3 scripts/qa/probes/credential.py --env PASS_VAR [--user-env USER_VAR --login-url URL]
python3 scripts/qa/probes/device.py --serial S [--expect-serialno X] [--package P] [--adb PATH]
python3 scripts/qa/probes/nas.py --host H --user-env U --pass-env P        # the Synology read-only account: SYNOLOGY_SMB_USER / SYNOLOGY_SMB_PASSWORD
```

## Honest boundary

A probe that cannot resolve its signal is blocked with the raw evidence, never present; a decoy that answers like the dependency (a 200 without a build id, a device that is present but is not the intended one, an empty credential variable) is reported blocked, with golden-present, golden-absent and carrier fixtures in the tests. Credential values never appear in output or evidence. A bare `version` field is not accepted as an identity (any service can print one). Owner decision: devices are emulators only; with none running the device probes report `device_absent`, never a simulation. Real run on this host: `evidence/wp24/probes-real.json`.
