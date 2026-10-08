#!/usr/bin/env python3
# identity: WP-12 PA-04 RED + mutation harness for submodules/filesystem/pkg/ftp
# usage:   python3 -I mutate_ftp.py red-old|red-skeleton|mutants|negcontrol|integration <out-dir>
# Every case copies the module to .audit/scratch/ftp-mut/<id>/view/filesystem, applies exact text replacements (each asserted to match exactly the
# expected number of times), and runs the suite in the pinned IMG-GO container through scripts/containers/run_pinned.sh. The checked-in tree is never
# modified. A mutant is KILLED when the suite fails with at least one `--- FAIL`, INVALID when it does not compile, SURVIVED when the suite passes.
# red-old      : the PRE-CHANGE client (git HEAD of pkg/ftp/ftp.go, the new files removed) under the new tests: the tests must not even compile.
# red-skeleton : the new code with the PA-04 behaviours stubbed back to the old behaviour (one copy, all stubs): the tests must fail.
# integration  : mutants run against the real pure-ftpd through scripts/test-infra/ftps_fixture.sh (FTPS_FIXTURE_SRC).
import os, shutil, subprocess, sys, json, re, time

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), *[".."] * 5))
MOD = os.path.join(ROOT, "submodules", "filesystem")
SCR = os.path.join(ROOT, ".audit", "scratch", "ftp-mut")
ENV = dict(os.environ, TMPDIR="/dev/shm", DISK_HEADROOM_REPO_ROOT=ROOT, LONGOPS_ALLOW_TMPFS="1")
F, T, S, C, P = "pkg/ftp/ftp.go", "pkg/ftp/tls.go", "pkg/ftp/scan.go", "pkg/ftp/cred.go", "pkg/ftp/tls.go"

# (id, description, [(file, old, new, count)], mode)
M = [
 ("M01", "explicit TLS is not requested (no AUTH TLS, clear text control + data)", [(F, 'goftp.DialWithExplicitTLS(newTLSConfig(cfg.Host, pins))', 'goftp.DialWithTimeout(c.dialTimeout())', 1)], "unit"),
 ("M02", "TLS minimum version lowered to 1.0", [(T, 'MinVersion:         tls.VersionTLS12,', 'MinVersion:         tls.VersionTLS10,', 1)], "unit"),
 ("M03", "any certificate is accepted (verification off, pin check off)", [(T, 'RootCAs:            pool,', 'RootCAs:            pool,\n\t\tInsecureSkipVerify: true,', 1), (T, 'if len(cs.PeerCertificates) == 0 {', 'if true {\n\t\t\t\treturn nil\n\t\t\t}\n\t\t\tif len(cs.PeerCertificates) == 0 {', 1)], "unit"),
 ("M04", "a changed certificate is reported as a generic TLS error instead of CertMismatchError", [(T, 'return &CertMismatchError{Host: host, Presented: fp, Pinned: pinFingerprints(pins)}, true', 'return err, true', 1)], "unit"),
 ("M05", "clear-text FTP is no longer refused without trusted_lan", [(F, '\t\tif !cfg.TrustedLAN {\n\t\t\treturn ErrClearTextRefused\n\t\t}\n', '', 1)], "unit"),
 ("M06", "an absent modification time is fabricated as now", [(F, 'fi.ModTime = e.Time // zero', 'fi.ModTime = time.Now() // zero', 1)], "unit"),
 ("M07", "a server without MLSD is listed with LIST silently (no ErrNoMLSD)", [(F, '\t\tif degraded && !c.config.AllowDegradedList {\n\t\t\treturn ErrNoMLSD\n\t\t}\n\t\tc.mu.Lock()', '\t\tc.mu.Lock()', 1)], "unit"),
 ("M08", "the degraded flag is never reported", [(F, 'func (c *Client) Degraded() bool {\n\tc.mu.Lock()\n\tdefer c.mu.Unlock()\n\treturn c.degraded', 'func (c *Client) Degraded() bool {\n\tc.mu.Lock()\n\tdefer c.mu.Unlock()\n\treturn false', 1)], "unit"),
 ("M09", "an authentication failure is classified as transient (would be retried)", [(F, 'return fabric.MarkAuth(', 'return fabric.MarkTransient(', 1)], "unit"),
 ("M10", "the session lock is not exclusive (control connection shared by concurrent commands)", [(F, 'sem: make(chan struct{}, 1)', 'sem: make(chan struct{}, 1<<20)', 1)], "unit"),
 ("M11", "path confinement clamps ../ at the root instead of refusing", [(F, '\t\t\tif len(segs) == 0 {\n\t\t\t\treturn "", "", fmt.Errorf("%w: %q", ErrPathEscape, p)\n\t\t\t}\n\t\t\tsegs = segs[:len(segs)-1]', '\t\t\tif len(segs) > 0 {\n\t\t\t\tsegs = segs[:len(segs)-1]\n\t\t\t}', 1)], "unit"),
 ("M12", "control characters in a path are no longer refused", [(F, 'if strings.ContainsAny(p, "\\r\\n\\x00") {', 'if false && strings.ContainsAny(p, "\\r\\n\\x00") {', 1)], "unit"),
 ("M13", "the scan client is not wrapped in the ReadOnly decorator", [(S, 'var c client.Client = decorators.ReadOnly(NewFTPClient(cfg))', 'var _ = decorators.ErrReadOnly\n\tvar c client.Client = NewFTPClient(cfg)', 1)], "unit"),
 ("M14", "Config.String leaks the inline password", [(F, 'return fmt.Sprintf("ftp.Config(%s@%s tls_mode=%s credential_ref=%q)", c.Username,', 'return fmt.Sprintf("ftp.Config(%s:"+c.Password+"@%s tls_mode=%s credential_ref=%q)", c.Username,', 1)], "unit"),
 ("M15", "REST offset dropped (resume always restarts at 0)", [(F, 'conn.RetrFrom(remote, offset)', 'conn.RetrFrom(remote, 0)', 1)], "unit"),
 ("M16", "a broken connection is not marked lost (no re-dial, retry hits the dead connection)", [(F, '\t\tc.state = stateLost\n\t\tc.conn = nil\n\t}\n\tc.mu.Unlock()\n\tif conn != nil {', '\t}\n\tc.mu.Unlock()\n\tif conn != nil {', 1)], "unit"),
 ("M17", "a closed stream does not release the session", [(F, '\t\ts.c.release()\n\t})\n\treturn err', '\t})\n\treturn err', 1)], "unit"),
 ("M18", "a missing file is an error instead of FileExists=false", [(F, '\tif errors.Is(err, os.ErrNotExist) {\n\t\treturn false, nil\n\t}\n\treturn false, err', '\treturn false, err', 1)], "unit"),
 ("M19", "Pin records whatever the server presents, ignoring the owner confirmation", [(P, 'if NormalizeFingerprint(d.Fingerprint) != NormalizeFingerprint(conf.Fingerprint) {', 'if false {', 1)], "unit"),
 ("M20", "the pin file is written world-readable", [(T, 'tmp.Chmod(0o600)', 'tmp.Chmod(0o644)', 1)], "unit"),
 ("M21", "OPTS UTF8 ON is disabled", [(F, 'goftp.DialWithDisabledUTF8(false)', 'goftp.DialWithDisabledUTF8(true)', 1)], "unit"),
 ("M22", "the data connections do not share the TLS session cache (no resumption)", [(T, 'ClientSessionCache: tls.NewLRUClientSessionCache(32),', '', 1)], "unit"),
 ("M23", "GetFileInfo ignores MLST and returns a size-only answer with a fabricated time", [(F, '\t\tif conn.IsTimePreciseInList() {\n\t\t\te, e2 := conn.GetEntry(remote)', '\t\tif false {\n\t\t\te, e2 := conn.GetEntry(remote)', 1)], "unit"),
 ("M24", "an unresolved credential_ref falls through to an empty password login", [(F, '\t\t\tif errors.Is(err, ErrCredentialUnavailable) {\n\t\t\t\treturn "", err\n\t\t\t}\n\t\t\treturn "", fmt.Errorf("%w: %q", ErrCredentialUnavailable, ref)', '\t\t\treturn "", nil', 1)], "unit"),
]
INT = [
 ("MI1", "INTEGRATION: the scan client is not read-only; the real server must see the deletes (assertion AND sink-side digest)", [(S, 'var c client.Client = decorators.ReadOnly(NewFTPClient(cfg))', 'var _ = decorators.ErrReadOnly\n\tvar c client.Client = NewFTPClient(cfg)', 1)], "integration"),
 ("MI2", "INTEGRATION: explicit TLS not requested against the real server", [(F, 'goftp.DialWithExplicitTLS(newTLSConfig(cfg.Host, pins))', 'goftp.DialWithTimeout(c.dialTimeout())', 1)], "integration"),
]
NEG = [
 ("N01", "NEGATIVE CONTROL: unmutated copy, same harness (must PASS)", [], "unit"),
 ("N02", "NEGATIVE CONTROL: behaviour-neutral edit (a comment) (must SURVIVE; shows the harness reports survival)", [(F, '// TLS modes of Config.TLSMode.', '// TLS modes of Config.TLSMode (edited).', 1)], "unit"),
]
# RED skeleton: new code, PA-04 behaviours stubbed back to the old client. All edits in one copy.
RED = [
 (F, 'goftp.DialWithExplicitTLS(newTLSConfig(cfg.Host, pins))', 'goftp.DialWithTimeout(c.dialTimeout())', 1),
 (F, '\t\tif !cfg.TrustedLAN {\n\t\t\treturn ErrClearTextRefused\n\t\t}\n', '', 1),
 (F, 'fi.ModTime = e.Time // zero', 'fi.ModTime = time.Now() // zero', 1),
 (F, '\t\tif degraded && !c.config.AllowDegradedList {\n\t\t\treturn ErrNoMLSD\n\t\t}\n\t\tc.mu.Lock()', '\t\tc.mu.Lock()', 1),
 (F, 'return fabric.MarkAuth(', 'return fabric.MarkTransient(', 1),
 (F, 'sem: make(chan struct{}, 1)', 'sem: make(chan struct{}, 1<<20)', 1),
 (F, '\t\t\tif len(segs) == 0 {\n\t\t\t\treturn "", "", fmt.Errorf("%w: %q", ErrPathEscape, p)\n\t\t\t}\n\t\t\tsegs = segs[:len(segs)-1]', '\t\t\tif len(segs) > 0 {\n\t\t\t\tsegs = segs[:len(segs)-1]\n\t\t\t}', 1),
 (S, 'var c client.Client = decorators.ReadOnly(NewFTPClient(cfg))', 'var _ = decorators.ErrReadOnly\n\tvar c client.Client = NewFTPClient(cfg)', 1),
 (F, 'conn.RetrFrom(remote, offset)', 'conn.RetrFrom(remote, 0)', 1),
 (F, 'goftp.DialWithDisabledUTF8(false)', 'goftp.DialWithDisabledUTF8(true)', 1),
 (F, 'return fmt.Sprintf("ftp.Config(%s@%s tls_mode=%s credential_ref=%q)", c.Username,', 'return fmt.Sprintf("ftp.Config(%s:"+c.Password+"@%s tls_mode=%s credential_ref=%q)", c.Username,', 1),
 (T, 'if NormalizeFingerprint(d.Fingerprint) != NormalizeFingerprint(conf.Fingerprint) {', 'if false {', 1),
 (T, 'MinVersion:         tls.VersionTLS12,', 'MinVersion:         tls.VersionTLS10,', 1),
]


def sh(cmd, cwd=None, env=None, timeout=1200):
    p = subprocess.run(cmd, cwd=cwd, env=env or ENV, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True, timeout=timeout)
    return p.returncode, p.stdout


def make_copy(cid):
    d = os.path.join(SCR, cid)
    shutil.rmtree(d, ignore_errors=True)
    os.makedirs(os.path.join(d, "view"), mode=0o700)
    shutil.copytree(MOD, os.path.join(d, "view", "filesystem"), ignore=shutil.ignore_patterns(".git"))
    return d


def apply(d, f, old, new, count):
    p = os.path.join(d, "view", "filesystem", f)
    s = open(p).read()
    n = s.count(old)
    if n != count:
        raise SystemExit("mutation pattern matched %d times (expected %d) in %s: %r" % (n, count, f, old[:70]))
    open(p, "w").write(s.replace(old, new))


def run_unit(d, cid, out):
    os.makedirs(os.path.join(d, "out"), exist_ok=True)
    cmd = ["bash", os.path.join(ROOT, "scripts/containers/run_pinned.sh"), "--out", os.path.join(d, "out"), "--op-id", "ftpmut-" + cid.lower(), "IMG-GO", "--",
           "env", "GOMAXPROCS=3", "GOTOOLCHAIN=local", "CGO_ENABLED=1", "GOFLAGS=-buildvcs=false", "sh", "-c",
           "cd /src/filesystem && go test -count=1 -timeout 170s ./pkg/ftp/ ./pkg/factory/"]
    return sh(cmd, cwd=os.path.join(d, "view"))


def classify(rc, o):
    fails = sorted(set(re.findall(r"^--- FAIL: (\S+)", o, re.M)))
    if rc == 0:
        return "SURVIVED", []
    if fails:
        return "KILLED", fails
    if "panic: test timed out" in o:
        return "KILLED", ["<suite hung until the test timeout>"]
    if "build failed" in o or re.search(r"\.go:\d+:\d+:", o):
        return "INVALID", []
    return "ERROR", []


def main():
    mode, out = sys.argv[1], os.path.abspath(sys.argv[2])
    os.makedirs(out, exist_ok=True)
    if mode in ("red-old", "red-skeleton"):
        d = make_copy("RED-" + mode.split("-")[1].upper())
        v = os.path.join(d, "view", "filesystem")
        if mode == "red-old":
            old = subprocess.check_output(["git", "-C", MOD, "show", "HEAD:pkg/ftp/ftp.go"]).decode()
            open(os.path.join(v, F), "w").write(old)
            for f in (C, T, S):
                os.remove(os.path.join(v, f))
            # the old client needs only the old go.mod set; the new one is a superset, keep it
        else:
            for f, o, n, c in RED:
                apply(d, f, o, n, c)
        rc, o = run_unit(d, mode, out)
        open(os.path.join(out, mode + "-raw.txt"), "w").write(o)
        fails = sorted(set(re.findall(r"^--- FAIL: (\S+)", o, re.M)))
        undefined = sorted(set(re.findall(r"undefined: (\S+)", o)))
        print(json.dumps({"mode": mode, "rc": rc, "failing_tests": len(fails), "names": fails, "undefined_symbols": len(undefined), "undefined": undefined[:40], "build_failed": "build failed" in o}, indent=1))
        return
    cases = {"mutants": M, "negcontrol": NEG, "integration": INT}[mode]
    res = []
    only = os.environ.get("MUT_ONLY")
    for cid, desc, reps, kind in cases:
        if only and cid not in only.split(","):
            continue
        t0 = time.time()
        d = make_copy(cid)
        for f, old, new, count in reps:
            apply(d, f, old, new, count)
        if kind == "unit":
            rc, o = run_unit(d, cid, out)
            verdict, fails = classify(rc, o)
        else:
            env = dict(ENV, FTPS_FIXTURE_SRC=os.path.join(d, "view", "filesystem"))
            logf = os.path.join(out, cid + "-integration.txt")
            rc, o = sh(["bash", os.path.join(ROOT, "scripts/test-infra/ftps_fixture.sh"), "run", "--log", logf, "--", "-v", "-run", "Integration", "./pkg/ftp/"], env=env)
            o = (open(logf).read() if os.path.exists(logf) else "") + "\n" + o
            verdict, fails = classify(rc, o)
            if "SINK-SIDE FAIL" in o:
                fails = fails + ["<sink-side digest changed>"]
                verdict = "KILLED"
        open(os.path.join(out, cid + ".txt"), "w").write(o)
        res.append({"id": cid, "desc": desc, "kind": kind, "verdict": verdict, "failing_tests": fails, "seconds": round(time.time() - t0)})
        print(cid, verdict, len(fails), "failing", flush=True)
        shutil.rmtree(d, ignore_errors=True)
    json.dump(res, open(os.path.join(out, "%s%s.json" % (mode, ("-only-" + only.replace(",", "_")) if only else "")), "w"), indent=1)
    k = sum(1 for r in res if r["verdict"] == "KILLED")
    print("TOTAL", len(res), "KILLED", k, "SURVIVED", [r["id"] for r in res if r["verdict"] == "SURVIVED"], "INVALID", [r["id"] for r in res if r["verdict"] in ("INVALID", "ERROR")])


main()
