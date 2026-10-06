"""lib.py - T216. Shared parts of the availability probes (doc12 10.3). Imported by the probe scripts that sit beside it.

A probe is ONE independent, real check of ONE dependency (a service, a credential by variable NAME, a device class). It prints exactly one JSON line
`{"probe", "status": "present"|"blocked", "reason", "evidence"}` and exits 0 (present), 1 (blocked) or 2 (usage). `reason` is from the CLOSED set below
(document 06). A probe that cannot resolve its signal is BLOCKED with the raw evidence, never present (11.4.201 (4)); a present dependency is never reported
blocked because of a proxy signal (the golden-absent and carrier fixtures in scripts/qa/tests/test_probes.py). Credential VALUES are never printed, logged or
put into evidence: only variable names.
"""
import json
import sys
import urllib.error
import urllib.request

REASONS = frozenset({"service_unreachable", "credential_absent", "credential_rejected", "device_absent", "device_wrong_identity",
                     "device_unauthorised", "geo_restricted", "quota_exhausted", "licence_absent", "host_resource_unavailable"})


def finish(probe, status, reason=None, evidence=None):
    if status == "blocked":
        assert reason in REASONS, "reason %r is not in the closed set" % reason
    rec = {"probe": probe, "status": status, "evidence": evidence or {}}
    if reason:
        rec["reason"] = reason
    sys.stdout.write(json.dumps(rec, sort_keys=True) + "\n")
    sys.stdout.flush()
    sys.exit(0 if status == "present" else 1)


def http(url, method="GET", headers=None, body=None, timeout=5):
    """(status_code, body_bytes); raises OSError on a connection-level failure (never on an HTTP error status)."""
    data = None
    h = dict(headers or {})
    if body is not None:
        data = json.dumps(body).encode()
        h.setdefault("Content-Type", "application/json")
    req = urllib.request.Request(url, data=data, headers=h, method=method)
    try:
        with urllib.request.urlopen(req, timeout=timeout) as r:
            return r.status, r.read(65536)
    except urllib.error.HTTPError as e:
        return e.code, e.read(65536)
    except (urllib.error.URLError, OSError, ValueError) as e:
        raise OSError(str(getattr(e, "reason", e)))
