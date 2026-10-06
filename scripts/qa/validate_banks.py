#!/usr/bin/env python3
"""validate_banks.py - T210 (doc12 6.6). The bank validator: rules R-1..R-8 over HelixQA bank files, run before any bank executes.

Usage:
  validate_banks.py --banks PATH [PATH ...] [--cases FILE] [--json FILE | --json-stdout] [--counts-tsv FILE]
    PATH   a bank file (*.yaml, *.yml) or a directory holding them (non-recursive, sorted)
    --cases FILE   one case id per line (blank lines and # comments ignored): R-1..R-8 are evaluated ONLY for the listed cases; a violation
                   in an unlisted case is not reported; a listed id that no bank holds is REFUSED (exit 2), never ignored
    --json FILE | --json-stdout   the machine report (schema qa-bank-validation/1: findings, counts by (bank, rule), cases seen/evaluated)
    --counts-tsv FILE   `bank<TAB>rule<TAB>count` lines (sorted), the ratchet baseline format of scripts/repo/validate_baselines/qa_banks.tsv
Exits: 0 no violation; 1 at least one violation (or a bank that does not parse: rule PARSE, a blind bank is never clean); 2 usage / unreadable
  input / no bank file found / unknown listed case id.

Rules (doc12 6.6). The findings are per step for R-1, R-2, R-3, R-5, R-8 and per case for R-4, R-6, R-7 (step is null):
  R-1 every step carries at least one machine assertion: expect_status, expect_json_path, expect_json_eq, expect_body_contains, expect_header,
      ocr_assert, pixel_assert or assert_after. Prose `expected` and `vision_verify` (a model call, advisory by 11.4.269) do not count.
  R-2 no step action contains TODO, CONVERT or placeholder (case-insensitive word match).
  R-3 a case passes only if every step ran: a step with `_skip` / `skip` true or an `_skip_reason` is skipped for a bank reason.
  R-4 `skipped` is not an allowed final status: `final_status` / `status` / `result` equal to skipped, or `skipped` inside
      `allowed_final_statuses`, at the case or in metadata.v3.
  R-5 credentials appear only as ${ENV_NAME}: a literal under a credential key (password, secret, token, api_key, authorization ...) in body or
      headers, a Bearer/JWT/AKIA literal, `password: literal` text in an action, or a default account typed by `text: admin` and the like. A step
      marked `negative: true` may carry a deliberately wrong literal (the doc12 6.5 pattern); that exemption is per step, never per case.
  R-6 `needs` (metadata.v3.needs) is complete: every ${ENV} the steps reference and every `requires_env` name must be in needs.credential_env;
      an http:/navigate: step needs needs.service; a tap/keypress/text/swipe/adb_shell step needs needs.device.
  R-7 metadata.v3.oracle exists, oracle.independent_of_sut is true and oracle.strategy is in the closed set of document 06
      (specified, derived, metamorphic, golden_master, invariant, statistical, human).
  R-8 a coordinate tap needs an ocr_assert or pixel_assert on the same step and is never the fixed centre 960,540 (11.4.193: no blind interaction).
Honest limits: the validator reads structure, never behaviour; it proves a bank is well-formed under the rules, not that a case would catch a
defect (that is the paired mutation, doc12 8.5). A .json twin next to a .yaml bank is reported under `unscanned`, never silently merged.
"""
import argparse
import json
import os
import re
import sys

import yaml

SCHEMA = "qa-bank-validation/1"
STRATEGIES = {"specified", "derived", "metamorphic", "golden_master", "invariant", "statistical", "human"}
ASSERT_KEYS = ("expect_status", "expect_json_path", "expect_json_eq", "expect_body_contains", "expect_header",
               "ocr_assert", "pixel_assert", "assert_after")
MARKER_RE = re.compile(r"(?i)\b(todo|convert|placeholder)\b")
ENV_REF_RE = re.compile(r"\$\{([A-Za-z_][A-Za-z0-9_]*)\}")
CRED_KEYS = {"password", "passwd", "pwd", "secret", "client_secret", "token", "access_token", "refresh_token", "session_token",
             "api_key", "apikey", "api-key", "authorization", "x-api-key"}
DEFAULT_ACCOUNTS = {"admin", "administrator", "root", "guest", "test", "user", "demo", "admin123", "password", "changeme", "123456", "letmein"}
TAP_RE = re.compile(r"^\s*(?:adb_shell:\s*(?:input\s+)?)?tap(?::\s*|\s+)(\d+)\s*[, ]\s*(\d+)\s*$", re.I)
TEXT_RE = re.compile(r"^\s*text:\s*(.+?)\s*$", re.I)
CRED_TEXT_RE = re.compile(r"(?i)\b(password|passwd|secret|api[_-]?key|token)\s*[=:]\s*['\"]?([^\s'\"$][^\s'\"]*)")
BEARER_RE = re.compile(r"(?i)\bbearer\s+(?!\$\{|\{)([A-Za-z0-9._~+/-]{16,})")
JWT_RE = re.compile(r"\beyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{4,}")
AKIA_RE = re.compile(r"\bAKIA[0-9A-Z]{16}\b")
DEVICE_PREFIXES = ("tap", "keypress", "text", "swipe", "long_press", "adb_shell", "back", "home")
SERVICE_PREFIXES = ("http", "navigate")


def refuse(msg, code=2):
    sys.stderr.write("validate_banks: REFUSED " + msg + "\n")
    sys.exit(code)


def strings_of(node):
    """Every string leaf of a nested structure (keys excluded)."""
    if isinstance(node, str):
        yield node
    elif isinstance(node, dict):
        for v in node.values():
            yield from strings_of(v)
    elif isinstance(node, (list, tuple)):
        for v in node:
            yield from strings_of(v)


def cred_literals(node, path=""):
    """Literal values under credential-named keys (structured body / headers)."""
    out = []
    if isinstance(node, dict):
        for k, v in node.items():
            if isinstance(v, str) and str(k).lower() in CRED_KEYS:
                if v.strip() and "${" not in v and not re.fullmatch(r"\{[A-Za-z_][A-Za-z0-9_]*\}", v.strip()) \
                        and not re.fullmatch(r"(?i)(bearer\s+)?\{[A-Za-z_][A-Za-z0-9_]*\}", v.strip()):
                    out.append("%s%s" % (path, k))
            else:
                out.extend(cred_literals(v, "%s%s." % (path, k)))
    elif isinstance(node, list):
        for i, v in enumerate(node):
            out.extend(cred_literals(v, "%s[%d]." % (path, i)))
    return out


def action_of(step):
    a = step.get("action")
    return a if isinstance(a, str) else ("" if a is None else str(a))


def v3_of(case):
    md = case.get("metadata")
    v3 = md.get("v3") if isinstance(md, dict) else None
    return v3 if isinstance(v3, dict) else {}


def step_has_assertion(step):
    for k in ASSERT_KEYS:
        v = step.get(k)
        if v not in (None, "", [], {}, 0) or (k == "expect_status" and isinstance(v, int) and v != 0):
            return True
    return False


def r5_hits(step):
    """Why this step carries a literal credential or a default account ([] when clean, or when the step is negative)."""
    if step.get("negative") is True:
        return []
    hits = []
    act = action_of(step)
    for part in ("body", "headers"):
        for p in cred_literals(step.get(part), part + "."):
            hits.append("literal under credential key %s" % p)
    auth = step.get("auth")
    if isinstance(auth, str) and auth.lower().startswith("raw:") and "{" not in auth and "${" not in auth:
        hits.append("literal raw auth value")
    texts = [act] + list(strings_of(step.get("body"))) + list(strings_of(step.get("headers"))) + ([auth] if isinstance(auth, str) else [])
    for t in texts:
        if BEARER_RE.search(t):
            hits.append("literal Bearer value")
        if JWT_RE.search(t):
            hits.append("literal JWT")
        if AKIA_RE.search(t):
            hits.append("literal AWS access key id")
    for m in CRED_TEXT_RE.finditer(act):
        if "${" not in m.group(2):
            hits.append("credential literal in action text (%s)" % m.group(1).lower())
    m = TEXT_RE.match(act)
    if m and m.group(1).strip().strip("'\"").lower() in DEFAULT_ACCOUNTS:
        hits.append("default account typed: text: %s" % m.group(1).strip())
    seen, out = set(), []
    for h in hits:
        if h not in seen:
            seen.add(h)
            out.append(h)
    return out


def check_case(case, bank):
    cid = str(case.get("id", ""))
    steps = case.get("steps") if isinstance(case.get("steps"), list) else []
    v3 = v3_of(case)
    out = []

    def add(rule, step, msg):
        out.append({"bank": bank, "case": cid, "step": step, "rule": rule, "message": msg})

    needs = v3.get("needs") if isinstance(v3.get("needs"), dict) else {}
    cred_env = needs.get("credential_env")
    cred_env = set(cred_env) if isinstance(cred_env, list) else set()
    refs, need_service, need_device = set(), False, False
    for i, st in enumerate(steps):
        if not isinstance(st, dict):
            add("R-1", i, "step is not a mapping")
            continue
        act = action_of(st)
        if not step_has_assertion(st):
            add("R-1", i, "no machine assertion (prose expected only)")
        if MARKER_RE.search(act):
            add("R-2", i, "action carries a TODO/CONVERT/placeholder marker")
        if st.get("_skip") or st.get("skip") or st.get("_skip_reason"):
            add("R-3", i, "step skipped for a bank reason (%s)" % (st.get("_skip_reason") or "_skip"))
        for h in r5_hits(st):
            add("R-5", i, h)
        m = TAP_RE.match(act)
        if m:
            fixed = (int(m.group(1)), int(m.group(2))) == (960, 540)
            asserted = bool(st.get("ocr_assert")) or bool(st.get("pixel_assert"))
            if fixed:
                add("R-8", i, "fixed centre tap 960,540" + ("" if asserted else " without ocr_assert/pixel_assert"))
            elif not asserted:
                add("R-8", i, "coordinate tap without ocr_assert/pixel_assert")
        for s in strings_of({k: v for k, v in st.items() if k not in ("expected", "name")}):
            refs.update(ENV_REF_RE.findall(s))
        pre = re.split(r"[:\s]", act.strip(), maxsplit=1)[0].lower() if act.strip() else ""
        if pre in SERVICE_PREFIXES:
            need_service = True
        if pre in DEVICE_PREFIXES:
            need_device = True
    # R-4
    bad_final = []
    for scope, d in (("case", case), ("metadata.v3", v3)):
        for k in ("final_status", "status", "result"):
            if isinstance(d.get(k), str) and d[k].strip().lower() == "skipped":
                bad_final.append("%s.%s" % (scope, k))
        afs = d.get("allowed_final_statuses")
        if isinstance(afs, list) and any(str(x).lower() == "skipped" for x in afs):
            bad_final.append("%s.allowed_final_statuses" % scope)
    if bad_final:
        add("R-4", None, "skipped is not an allowed final status (%s)" % ", ".join(bad_final))
    # R-6
    req = case.get("requires_env")
    req = set(req) if isinstance(req, list) else set()
    undeclared = sorted((refs | req) - cred_env)
    miss = []
    if undeclared:
        miss.append("environment variable(s) not in needs.credential_env: %s" % ", ".join(undeclared))
    if need_service and not needs.get("service"):
        miss.append("http/navigate step without needs.service")
    if need_device and not needs.get("device"):
        miss.append("device step without needs.device")
    if miss:
        add("R-6", None, "needs incomplete: " + "; ".join(miss))
    # R-7
    oracle = v3.get("oracle")
    if not isinstance(oracle, dict):
        add("R-7", None, "metadata.v3.oracle missing")
    else:
        if oracle.get("independent_of_sut") is not True:
            add("R-7", None, "oracle.independent_of_sut is not true")
        strat = str(oracle.get("strategy", "")).replace("-", "_").lower()
        if strat not in STRATEGIES:
            add("R-7", None, "oracle.strategy %r not in the closed set" % oracle.get("strategy"))
    return out


def collect_files(paths):
    files, unscanned = [], []
    for p in paths:
        if os.path.isdir(p):
            names = sorted(os.listdir(p))
            ys = [n for n in names if n.endswith((".yaml", ".yml")) and n != "MANIFEST.yaml"]  # the manifest is not a bank
            files.extend(os.path.join(p, n) for n in ys)
            unscanned.extend(os.path.join(p, n) for n in names if n.endswith(".json"))
        elif os.path.isfile(p):
            files.append(p)
        else:
            refuse("input_missing %s" % p)
    if not files:
        refuse("no bank file found under %s" % ", ".join(paths))
    return files, unscanned


def read_cases_filter(path):
    try:
        with open(path) as fh:
            ids = [ln.strip() for ln in fh if ln.strip() and not ln.strip().startswith("#")]
    except OSError as e:
        refuse("cases_file_unreadable %s: %s" % (path, e))
    return ids


def validate(paths, case_ids=None):
    files, unscanned = collect_files(paths)
    findings, seen_ids, evaluated, seen = [], set(), 0, 0
    wanted = set(case_ids) if case_ids is not None else None
    for f in files:
        bank = os.path.basename(f)
        try:
            with open(f) as fh:
                doc = yaml.safe_load(fh)
        except Exception as e:  # a bank that does not parse is a finding, never a clean file
            findings.append({"bank": bank, "case": None, "step": None, "rule": "PARSE", "message": "unreadable bank: %s" % str(e).splitlines()[0]})
            continue
        cases = doc.get("test_cases") if isinstance(doc, dict) else None
        if not isinstance(cases, list):
            findings.append({"bank": bank, "case": None, "step": None, "rule": "PARSE", "message": "no test_cases list"})
            continue
        for case in cases:
            if not isinstance(case, dict):
                continue
            seen += 1
            cid = str(case.get("id", ""))
            seen_ids.add(cid)
            if wanted is not None and cid not in wanted:
                continue
            evaluated += 1
            findings.extend(check_case(case, bank))
    if wanted is not None:
        unknown = sorted(wanted - seen_ids)
        if unknown:
            refuse("unknown_case_id listed in --cases but held by no bank: %s" % ", ".join(unknown))
    counts = {}
    for fd in findings:
        counts.setdefault(fd["bank"], {}).setdefault(fd["rule"], 0)
        counts[fd["bank"]][fd["rule"]] += 1
    return {"schema": SCHEMA, "banks": [os.path.basename(f) for f in files], "unscanned": [os.path.basename(u) for u in unscanned],
            "cases_seen": seen, "cases_evaluated": evaluated, "findings": findings, "counts": counts}


def totals(report):
    t = {}
    for per in report["counts"].values():
        for r, n in per.items():
            t[r] = t.get(r, 0) + n
    return dict(sorted(t.items()))


def main(argv=None):
    ap = argparse.ArgumentParser(description="HelixQA bank validator (rules R-1..R-8)")
    ap.add_argument("--banks", nargs="+", required=True)
    ap.add_argument("--cases")
    ap.add_argument("--json")
    ap.add_argument("--json-stdout", action="store_true")
    ap.add_argument("--counts-tsv")
    try:
        ns = ap.parse_args(argv)
    except SystemExit as e:
        sys.exit(2 if e.code not in (0, None) else 0)
    ids = read_cases_filter(ns.cases) if ns.cases else None
    rep = validate(ns.banks, ids)
    rep["totals"] = totals(rep)
    if ns.json:
        with open(ns.json, "w") as fh:
            json.dump(rep, fh, indent=1, sort_keys=True)
            fh.write("\n")
    if ns.counts_tsv:
        with open(ns.counts_tsv, "w") as fh:
            for bank in sorted(rep["counts"]):
                for rule in sorted(rep["counts"][bank]):
                    fh.write("%s\t%s\t%d\n" % (bank, rule, rep["counts"][bank][rule]))
    if ns.json_stdout:
        json.dump(rep, sys.stdout, indent=1, sort_keys=True)
        sys.stdout.write("\n")
    else:
        sys.stdout.write("validate_banks: %d bank(s), %d case(s) seen, %d evaluated, %d finding(s) %s\n"
                         % (len(rep["banks"]), rep["cases_seen"], rep["cases_evaluated"], len(rep["findings"]), json.dumps(rep["totals"])))
    sys.exit(1 if rep["findings"] else 0)


if __name__ == "__main__":
    main()
