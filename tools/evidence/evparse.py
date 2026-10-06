#!/usr/bin/env python3
"""evparse - report parsers behind the runner wrappers tools/evidence/wrap-{go,bash,vitest,gradle,cargo}.sh (tasks.md T051, T053).

    python3 -I evparse.py KIND REPORT [--rc N] [--min-tests N] [--since EPOCH] [--no-raw]

KIND is go | bash | vitest | gradle | cargo; REPORT is the recorded or fresh report (a file; a file or a directory for gradle).
Prints a summary (lines starting `wrap: `) and then, unless --no-raw, the line `wrap: raw-begin` and the raw report, so the
wrapper's stdout (the stream the recorder stores) carries both the verdict-bearing summary and the runner's own words.

Exit status (docs/06 s3.1 rule 10 maps it to the entry verdict): 0 every test ran and passed; 1 at least one test failed (or an
equivalent trace was found: a started test with no result in a stream that also holds a failure, a header that counts failures,
an exit status that says failure without any failing line); 126 NO TEST OUTCOME (empty or unparsable report, zero tests ran,
a build failure, a stream cut before any test result could be confirmed). 126 is never a test failure, so a RED that ran the
wrong thing (it did not compile, it ran zero tests) cannot be a genuine RED (docs/06 s16: "runner reports success without running").

Hidden failures (the T051 acceptance): the per-test results are the evidence, never the final summary. A stream cut before its
summary still shows its failures; a fake `ok` line printed by a test is only text inside an output event; a suite header that says
failures=0 over a failing test case does not hide it; a summary that disagrees with the lines (a line was lost) is a failure.

Limits (stated, 11.4.6): the recorded fixtures of go, bash and vitest come from real runs; gradle and cargo are parsed against
fixtures authored from the documented formats and are UNCONFIRMED against a live run (WP-14). Only the formats named above are read:
a runner switched to another reporter yields `unparsable`, an error, never a pass.
"""
import json
import os
import re
import sys
import xml.etree.ElementTree as ET

ANSI = re.compile(rb"\x1b\[[0-9;?]*[A-Za-z]")
MAX_NAMES = 100


class R:
    """One parse result."""

    def __init__(self, runner):
        self.runner, self.passed, self.failed, self.skipped = runner, 0, 0, 0
        self.names, self.notes, self.reason, self.status = [], [], None, None
        self.rc = None

    @property
    def tests(self):
        return self.passed + self.failed + self.skipped

    def fail_name(self, n):
        if n not in self.names:
            self.names.append(n)
            self.failed += 1

    def error(self, reason):
        self.status, self.reason = "error", reason
        return self

    def fail(self, reason=None):
        self.status = "fail"
        if reason:
            self.reason = reason
        return self


def _finish(r, rc, min_tests):
    """Common closing rules once a parser has counted: a pass needs executed tests, enough of them, and a consistent exit status."""
    if r.status is None:
        if r.failed > 0:
            r.status = "fail"
        elif r.passed + r.failed == 0:
            r.status, r.reason = "error", "no_tests"
        elif r.tests < min_tests:
            r.status, r.reason = "error", "too_few_tests"
        elif rc is not None and rc != 0:
            r.status, r.reason = "fail", "exit_without_fail_line"
        else:
            r.status = "pass"
    elif r.status == "fail" and r.reason is None and r.failed == 0:
        r.reason = "failure_without_name"
    r.rc = rc
    return r


# ----------------------------------------------------------------------------- go (go test -json, test2json events)

def parse_go(raw, rc, min_tests):
    r = R("go")
    text = raw.decode("utf-8", "replace")
    if not text.strip():
        return r.error("empty_report")
    lines = text.split("\n")
    truncated = bool(text) and not text.endswith("\n")
    tests, pkgs, started, pkg_term, textfail = {}, {}, set(), {}, []
    build_failed = junk = events = 0
    last = max((i for i, l in enumerate(lines) if l.strip()), default=-1)
    for i, line in enumerate(lines):
        if not line.strip():
            continue
        try:
            ev = json.loads(line)
            if not isinstance(ev, dict):
                raise ValueError
        except ValueError:
            if i == last and truncated:
                r.notes.append("truncated: the last line is cut mid-JSON")
            else:
                junk += 1
            continue
        act, pkg, name = ev.get("Action"), ev.get("Package") or ev.get("ImportPath"), ev.get("Test")
        if act is None:
            junk += 1
            continue
        events += 1
        if act in ("build-fail",):
            build_failed += 1
            continue
        if act == "start":
            pkgs.setdefault(pkg, None)
            continue
        if act == "run" and name:
            started.add((pkg, name))
        elif act in ("pass", "fail", "skip"):
            if name:
                tests[(pkg, name)] = act
            else:
                pkg_term[pkg] = act
        elif act == "output" and name:
            for ol in str(ev.get("Output", "")).split("\n"):
                m = re.match(r"\s*--- FAIL: (\S+)", ol)
                if m:
                    textfail.append((pkg, m.group(1)))
        elif act == "output":
            pass
    if events == 0:
        return r.error("unparsable")
    for (pkg, name), act in tests.items():
        if act == "pass":
            r.passed += 1
        elif act == "skip":
            r.skipped += 1
        else:
            r.fail_name("%s %s" % (pkg, name))
    for pkg, name in textfail:                       # a failure the events did not carry as an event of its own
        if tests.get((pkg, name)) != "fail":
            r.fail_name("%s %s" % (pkg, name))
    incomplete = sorted(k for k in started if k not in tests)
    unfinished_pkgs = sorted(p for p in pkgs if p not in pkg_term)
    if r.failed > 0:
        r.status = "fail"
        if truncated or unfinished_pkgs or incomplete:
            r.notes.append("truncated: %d test(s) started without a result, %d package(s) without a summary event"
                           % (len(incomplete), len(unfinished_pkgs)))
        return _finish(r, rc, min_tests)
    if build_failed or any(a == "fail" for a in pkg_term.values()) and not tests:
        return r.error("build_failed")
    if any(a == "fail" for a in pkg_term.values()):
        return r.error("package_failed")             # a package failed with no failing test: TestMain, panic outside a test, timeout
    if incomplete or unfinished_pkgs or truncated:
        r.notes.append("truncated: %d test(s) started without a result, %d package(s) without a summary event"
                       % (len(incomplete), len(unfinished_pkgs)))
        return r.error("truncated_report")
    return _finish(r, rc, min_tests)


# ----------------------------------------------------------------------------- bash (the project's PASS:/FAIL:/ok line conventions)

_PASS = re.compile(r"^(?:PASS:?|ok:?)(?:\s|$)")
_FAIL = re.compile(r"^(?:FAIL:?|not ok\b)")
_SKIP = re.compile(r"^(?:SKIP:?|skip:?)(?:\s|$)")
_SUM1 = re.compile(r"\bPASS=(\d+)\s+FAIL=(\d+)(?:\s+SKIP=(\d+))?")
_SUM2 = re.compile(r"^(?:checks=(\d+)\s+)?failures=(\d+)\s*$")
_PLAN = re.compile(r"^1\.\.(\d+)\s*$")


def parse_bash(raw, rc, min_tests):
    r = R("bash")
    if not raw.strip():
        return r.error("no_tests")
    sums, plan = [], None
    for ln in ANSI.sub(b"", raw).decode("utf-8", "replace").split("\n"):
        line = ln.rstrip("\r")
        if _FAIL.match(line):
            r.fail_name(line[:160])
        elif _PASS.match(line):
            r.passed += 1
        elif _SKIP.match(line):
            r.skipped += 1
        else:
            m = _SUM1.search(line)
            if m:
                sums.append(("pf", int(m.group(1)), int(m.group(2))))
                continue
            m = _SUM2.match(line)
            if m:
                sums.append(("failures", int(m.group(2)), None))
                continue
            m = _PLAN.match(line)
            if m:
                plan = int(m.group(1))
    r.rc = rc
    if r.failed:
        r.status = "fail"
    for kind, a, b in sums:
        if kind == "pf":
            if b > 0 and not r.failed:
                r.fail("summary_reports_failures")
            if a != r.passed:
                r.notes.append("summary_mismatch: the summary counts PASS=%d, %d result line(s) seen" % (a, r.passed))
                if not r.failed and r.status != "fail":
                    r.fail("summary_mismatch")
            elif b == 0 and r.failed:
                r.notes.append("summary_mismatch: the summary says FAIL=0, %d FAIL line(s) seen (the lines win)" % r.failed)
        else:
            if a > 0 and not r.failed:
                r.fail("summary_reports_failures")
            if a == 0 and r.failed:
                r.notes.append("summary_mismatch: the summary says failures=0, %d FAIL line(s) seen (the lines win)" % r.failed)
    if plan is not None and r.tests < plan and r.status != "fail":
        r.fail("plan_mismatch")
    if r.status == "fail" and r.failed == 0 and r.reason is None:
        r.reason = "failure_without_name"
    if r.status is None and rc is not None and rc != 0 and r.passed + r.skipped >= 0:
        # no FAIL line and no summary complaint, yet the script exited non-zero: a truncated or crashed run
        r.fail("exit_without_fail_line")
    if r.status == "fail" and r.failed and any("summary_mismatch" in n for n in r.notes) and r.reason is None:
        r.reason = "summary_mismatch"
    return _finish(r, rc, min_tests) if r.status is None else _close(r, rc)


def _close(r, rc):
    r.rc = rc
    return r


# ----------------------------------------------------------------------------- vitest (--reporter=json)

def parse_vitest(raw, rc, min_tests):
    r = R("vitest")
    if not raw.strip():
        return r.error("empty_report")
    d = None
    try:
        d = json.loads(raw.decode("utf-8", "replace"))
    except ValueError:
        pass
    if isinstance(d, dict) and isinstance(d.get("testResults"), list):
        for f in d["testResults"]:
            fa = f.get("assertionResults") or []
            for a in fa:
                st = a.get("status")
                if st == "passed":
                    r.passed += 1
                elif st == "failed":
                    r.fail_name("%s :: %s" % (os.path.basename(str(f.get("name", "?"))), a.get("fullName") or a.get("title") or "?"))
                elif st in ("skipped", "pending", "todo", "disabled"):
                    r.skipped += 1
            if f.get("status") == "failed" and not any(a.get("status") == "failed" for a in fa):
                r.fail_name("%s :: (suite failed to run: %s)" % (os.path.basename(str(f.get("name", "?"))), str(f.get("message", ""))[:80]))
        hdr = d.get("numFailedTests")
        if isinstance(hdr, int) and hdr > r.failed:
            r.notes.append("header_failures: the report header counts %d failed test(s), %d assertion(s) show" % (hdr, r.failed))
            r.fail("header_failures")
        if d.get("success") is False and r.failed == 0 and r.status is None:
            r.fail("header_failures")
        if r.failed:
            r.status = "fail"
        return _finish(r, rc, min_tests)
    # not a whole JSON document: salvage what a cut report still shows
    text = raw.decode("utf-8", "replace")
    if "No test files found" in text:
        return r.error("no_tests")
    stat = re.findall(r'"fullName":"((?:[^"\\]|\\.)*)","status":"(passed|failed|skipped|pending|todo)"', text)
    if not stat:
        stat = [(t, s) for s, t in re.findall(r'"status":"(passed|failed|skipped|pending|todo)","title":"((?:[^"\\]|\\.)*)"', text)]
    if not stat:
        return r.error("unparsable")
    seen = set()
    for name, st in stat:
        if (name, st) in seen:
            continue
        seen.add((name, st))
        if st == "passed":
            r.passed += 1
        elif st == "failed":
            r.fail_name(name or "?")
        else:
            r.skipped += 1
    r.notes.append("truncated: the report is not a complete JSON document; %d assertion status(es) salvaged" % len(seen))
    if r.failed:
        r.status = "fail"
        return _finish(r, rc, min_tests)
    return r.error("truncated_report")


# ----------------------------------------------------------------------------- gradle (JUnit XML, build/test-results/**/TEST-*.xml)

def _gradle_files(path, since):
    if os.path.isfile(path):
        return [path]
    out = []
    for root, dirs, files in os.walk(path):
        dirs.sort()
        for fn in sorted(files):
            if fn.startswith("TEST-") and fn.endswith(".xml"):
                p = os.path.join(root, fn)
                if since is None or os.path.getmtime(p) >= since - 1:
                    out.append(p)
    return out


def parse_gradle(path, rc, min_tests, since=None):
    r = R("gradle")
    try:
        files = _gradle_files(path, since)
    except OSError:
        return r.error("unreadable_report")
    if not files:
        return r.error("no_reports")
    for p in files:
        try:
            with open(p, "rb") as f:
                raw = f.read()
        except OSError:
            return r.error("unreadable_report")
        try:
            root = ET.fromstring(raw)
            suites = [root] if root.tag == "testsuite" else list(root.iter("testsuite"))
            for s in suites:
                seen_fail = 0
                for tc in s.iter("testcase"):
                    nm = "%s.%s" % (tc.get("classname", "?"), tc.get("name", "?"))
                    if tc.find("failure") is not None or tc.find("error") is not None:
                        r.fail_name(nm)
                        seen_fail += 1
                    elif tc.find("skipped") is not None:
                        r.skipped += 1
                    else:
                        r.passed += 1
                hdr = 0
                for k in ("failures", "errors"):
                    try:
                        hdr += int(s.get(k, "0"))
                    except ValueError:
                        hdr += 0
                if hdr > seen_fail:
                    r.notes.append("header_failures: suite %s counts %d failure(s), %d test case(s) show" % (s.get("name"), hdr, seen_fail))
                    r.fail("header_failures")
        except ET.ParseError:
            text = raw.decode("utf-8", "replace")
            r.notes.append("truncated: %s is not well-formed XML; salvaged by scanning test case elements" % os.path.basename(p))
            for chunk in text.split("<testcase")[1:]:
                m = re.match(r'[^>]*?\bname="([^"]*)"', chunk)
                cm = re.search(r'\bclassname="([^"]*)"', chunk.split(">", 1)[0])
                nm = "%s.%s" % (cm.group(1) if cm else "?", m.group(1) if m else "?")
                head = chunk.split("</testcase>", 1)[0]
                if "<failure" in head or "<error" in head:
                    r.fail_name(nm)
                elif "<skipped" in head:
                    r.skipped += 1
                elif chunk.rstrip().endswith("/>") or "</testcase>" in chunk:
                    r.passed += 1
            if r.failed == 0:
                return r.error("truncated_report")
    if r.failed:
        r.status = "fail"
    return _finish(r, rc, min_tests)


# ----------------------------------------------------------------------------- cargo (libtest text)

_CT = re.compile(r"^test (\S+) \.\.\. (ok|FAILED|ignored)\b")
_CR = re.compile(r"^test result: (ok|FAILED)\.")


def parse_cargo(raw, rc, min_tests):
    r = R("cargo")
    if not raw.strip():
        return r.error("empty_report")
    results, build, failed_line, other = [], False, False, 0
    for ln in ANSI.sub(b"", raw).decode("utf-8", "replace").split("\n"):
        line = ln.rstrip("\r")
        m = _CT.match(line)
        if m:
            nm, st = m.group(1), m.group(2)
            if st == "ok":
                r.passed += 1
            elif st == "ignored":
                r.skipped += 1
            else:
                r.fail_name(nm)
            continue
        m = _CR.match(line)
        if m:
            results.append(m.group(1))
            continue
        if re.match(r"^error(\[E\d+\])?: ", line) and ("could not compile" in line or re.match(r"^error\[E\d+\]", line)):
            build = True
        elif line.startswith("error: test failed"):
            failed_line = True
        elif line.startswith("running "):
            other += 1
    if r.failed:
        r.status = "fail"
        if not results:
            r.notes.append("truncated: no `test result:` line follows the failing test")
        elif "ok" in results and "FAILED" not in results:
            r.notes.append("summary_mismatch: every `test result:` line says ok, %d FAILED test line(s) seen (the lines win)" % r.failed)
        return _finish(r, rc, min_tests)
    if build and not r.passed:
        return r.error("build_failed")
    if "FAILED" in results or failed_line:
        return r.fail("summary_reports_failures")
    if r.tests and not results:
        r.notes.append("truncated: %d test line(s) and no `test result:` line" % r.tests)
        return r.error("truncated_report")
    return _finish(r, rc, min_tests)


PARSERS = {"go": parse_go, "bash": parse_bash, "vitest": parse_vitest, "cargo": parse_cargo}


def summarize(r, out):
    out.write("wrap: runner=%s status=%s tests=%d passed=%d failed=%d skipped=%d\n" % (r.runner, r.status, r.tests, r.passed, r.failed, r.skipped))
    for n in r.names[:MAX_NAMES]:
        out.write("wrap: failed %s\n" % n)
    if len(r.names) > MAX_NAMES:
        out.write("wrap: failed ... and %d more\n" % (len(r.names) - MAX_NAMES))
    if r.reason:
        out.write("wrap: reason=%s\n" % r.reason)
    for n in r.notes:
        out.write("wrap: note=%s\n" % n)
    out.write("wrap: rc=%s\n" % ("unknown" if r.rc is None else r.rc))
    out.flush()


def raw_of(kind, path, since):
    if kind == "gradle":
        buf = b""
        for p in _gradle_files(path, since):
            with open(p, "rb") as f:
                buf += b"<!-- %s -->\n" % os.path.basename(p).encode() + f.read() + b"\n"
        return buf
    with open(path, "rb") as f:
        return f.read()


def main(argv):
    if len(argv) < 2 or argv[0] not in PARSERS and argv[0] != "gradle":
        sys.stderr.write("evparse: usage: evparse.py go|bash|vitest|gradle|cargo REPORT [--rc N] [--min-tests N] [--since EPOCH] [--no-raw]\n")
        return 126
    kind, path = argv[0], argv[1]
    rc = since = None
    min_tests, raw_out, i = 1, True, 2
    while i < len(argv):
        a = argv[i]
        try:
            if a == "--rc":
                rc = int(argv[i + 1]); i += 2
            elif a == "--min-tests":
                min_tests = int(argv[i + 1]); i += 2
            elif a == "--since":
                since = float(argv[i + 1]); i += 2
            elif a == "--no-raw":
                raw_out = False; i += 1
            else:
                raise ValueError(a)
        except (ValueError, IndexError):
            sys.stderr.write("evparse: bad argument %r\n" % a)
            return 126
    try:
        if kind == "gradle":
            r = parse_gradle(path, rc, min_tests, since)
        else:
            with open(path, "rb") as f:
                raw = f.read()
            r = PARSERS[kind](raw, rc, min_tests)
    except OSError as e:
        sys.stderr.write("wrap: refused reason=report_unreadable %s: %s\n" % (path, e.strerror))
        return 126
    r.rc = rc
    summarize(r, sys.stdout)
    if raw_out:
        sys.stdout.write("wrap: raw-begin\n")
        sys.stdout.flush()
        try:
            sys.stdout.buffer.write(raw_of(kind, path, since))
            sys.stdout.buffer.flush()
        except OSError:
            pass
    return {"pass": 0, "fail": 1}.get(r.status, 126)


if __name__ == "__main__":
    sys.dont_write_bytecode = True
    sys.exit(main(sys.argv[1:]))
