#!/usr/bin/env python3
# identity: WP-12 PA-05 fix-r2 mutation harness for submodules/filesystem/pkg/sftp (+ pkg/factory)
# usage:   python3 -I fix-r2-mutants.py run <out-dir> [ID,ID,...]      one mutant at a time, unit suite in the pinned IMG-GO container
#          python3 -I fix-r2-mutants.py list
# What it is: the 21 mutants the independent reviewer wrote in round 1 (R01..R21, re-targeted onto the code after the fix - the text they
# mutated no longer exists in the same form), the 14 author mutants of round 1 (M*, re-targeted), and mutants for every NEW mechanism of
# fix-r2 (N*, P*, E*, F*, K*, D*, W*, S*). Each copy gets ONE exact replacement set, asserted to match exactly `count` times, the checkout
# is never modified. Verdicts: KILLED (the suite fails or the run does not complete: a hang is a kill), SURVIVED (suite green), INVALID (does not
# compile). Negative controls: Z00 (unmutated, must PASS) and Z01 (comment-only edit, must SURVIVE). A mutant is only believed after Z00 passed in the same run.
import json, os, re, shutil, subprocess, sys, time

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), *[".."] * 5))
MOD = os.path.join(ROOT, "submodules", "filesystem")
SCR = os.path.join(ROOT, ".audit", "scratch", "sftp-fixr2-mut")
ENV = dict(os.environ, TMPDIR="/dev/shm", DISK_HEADROOM_REPO_ROOT=ROOT, LONGOPS_ALLOW_TMPFS="1")

S, H, C, FR, F = "pkg/sftp/sftp.go", "pkg/sftp/hostkey.go", "pkg/sftp/credential.go", "pkg/sftp/frame.go", "pkg/factory/factory.go"

# (id, description, [(file, old, new, count), ...])
M = [
 ("Z00", "NEGATIVE CONTROL: unmutated copy (must PASS)", []),
 ("Z01", "NEGATIVE CONTROL: comment-only edit (must SURVIVE; shows the harness reports survival)", [(S, "// ErrReadOnly is returned by every mutating operation.", "// ErrReadOnly is returned by every mutating operation (edited).", 1)]),
 # ---- the reviewer's 21 mutants (WF19-REVIEW-sftp.md section 3), re-targeted
 ("R01", "pipelining off", [(S, "gosftp.MaxConcurrentRequestsPerFile(orInt(c.cfg.MaxConcurrentRequests, DefaultMaxConcurrentRequests)),", "gosftp.MaxConcurrentRequestsPerFile(1),", 1), (S, "gosftp.UseConcurrentReads(true),", "gosftp.UseConcurrentReads(false),", 1)]),
 ("R02", "file.WriteTo removed", [(S, "func (fl *file) WriteTo(w io.Writer) (n int64, err error) {", "func (fl *file) writeToDisabled(w io.Writer) (n int64, err error) {", 1)]),
 ("R03", "ListDirectory keeps '.' and '..'", [(S, 'if e.Name() == "." || e.Name() == ".." {', "if false {", 1)]),
 ("R04", "context end no longer closes the open file", [(S, "fl.stop = context.AfterFunc(ctx, func() { _ = fl.Close() })", "fl.stop = func() bool { return true }", 1)]),
 ("R05", "handshake not cancellable by ctx", [(S, "stop := context.AfterFunc(ctx, func() { _ = nc.Close() })", "stop := func() bool { return true }", 1)]),
 ("R06", "handshake deadline removed", [(S, "\t_ = nc.SetDeadline(time.Now().Add(timeout))\n", "", 1)]),
 ("R07", "Disconnect closes nothing", [(S, "\tif cn != nil {\n\t\tcn.close()\n\t}\n\treturn nil\n}\n\n// IsConnected", "\t_ = cn\n\treturn nil\n}\n\n// IsConnected", 1)]),
 ("R08", "client never wipes the resolved credential", [(S, "\tdefer cred.wipe()\n", "\t_ = cred.wipe\n", 1)]),
 ("R09", "factory ignores sftp.DefaultPinStore", [(F, "PinStore:      sftppkg.DefaultPinStore,", "PinStore:      nil,", 1)]),
 ("R10", "TestConnection accepts a root that is not a directory", [(S, "\tif !fi.IsDir() {", "\tif !fi.IsDir() && false {", 1)]),
 ("R11", "ListDirectory ignores ctx (ReadDir)", [(S, "sc.ReadDirContext(ctx, remote)", "sc.ReadDir(remote)", 1)]),
 ("R12", "every net.Error is a connection loss / transient", [(S, "return errors.As(err, &ne) && ne.Timeout()", "return errors.As(err, &ne)", 1)]),
 ("R13", "ReadRange negative-offset guard removed", [(S, '\tif offset < 0 {\n\t\treturn nil, fmt.Errorf("sftp: negative offset %d", offset)\n\t}\n', "", 1)]),
 ("R14", "SANITY: a lost connection is never marked lost", [(S, "\tif isConnectionLoss(err) {\n\t\tc.drop(cn)\n\t\treturn err\n\t}", "\tif false && isConnectionLoss(err) {\n\t\tc.drop(cn)\n\t\treturn err\n\t}", 1)]),
 ("R15", "dial drops the captured typed host key refusal (reviewer: EQUIVALENT, x/crypto wraps with %w)", [(S, "\t\tif refusal != nil {\n\t\t\treturn nil, refusal\n\t\t}\n", "", 1)]),
 ("R16", "FileExists reports a path escape as 'does not exist'", [(S, "\tif errors.Is(err, os.ErrNotExist) {\n\t\treturn false, nil", "\tif errors.Is(err, os.ErrNotExist) || errors.Is(err, ErrPathEscape) {\n\t\treturn false, nil", 1)]),
 ("R17", "confine rewrites backslashes again (SFTP-07 back)", [(S, '\tvar segs []string\n\tfor _, s := range strings.Split(p, "/") {', '\tp = strings.ReplaceAll(p, "\\\\", "/")\n\tvar segs []string\n\tfor _, s := range strings.Split(p, "/") {', 1)]),
 ("R18", "backoff constant instead of exponential", [(S, "if delay *= 2; delay > maxRetryDelay {", "if delay *= 1; delay > maxRetryDelay {", 1)]),
 ("R19", "GetFileInfo('/') no longer named '/'", [(S, '\t\tif logical == "/" {\n\t\t\tout.Name = "/"\n\t\t}\n', "", 1)]),
 ("R20", "containment fails OPEN again (SFTP-06 back)", [(S, '\t\treturn fmt.Errorf("sftp: cannot verify that %q stays inside the root: %w", remote, err)\n', "\t\treturn nil\n", 1)]),
 ("R21", "io.EOF classified as connection loss again (SFTP-05 back)", [(S, "errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, net.ErrClosed) ||", "errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) ||", 1)]),
 # ---- the author's 14 mutants of round 1 (M12 is integration-only and is run through the OpenSSH fixture, see the README), re-targeted
 ("M01", "auth failure becomes transient", [(S, "return nil, &AuthError{User: c.cfg.Username, Host: hp, Err: err}", 'return nil, fmt.Errorf("sftp: auth %s: %w", hp, io.ErrUnexpectedEOF)', 1)]),
 ("M02", "unpinned host key accepted (TOFU)", [(H, "\t\t\treturn *refusal\n\t\t}\n\t\tpinned := ", "\t\t\treturn nil\n\t\t}\n\t\tpinned := ", 1)]),
 ("M03", "a CHANGED host key accepted", [(H, "Presented: fp, Pinned: pinned}\n\t\treturn *refusal", "Presented: fp, Pinned: pinned}\n\t\treturn nil", 1)]),
 ("M04", "'..' clamps at the root instead of refusing", [(S, '\t\t\tif len(segs) == 0 {\n\t\t\t\treturn "", "", fmt.Errorf("%w: %q", ErrPathEscape, p)\n\t\t\t}\n\t\t\tsegs = segs[:len(segs)-1]', '\t\t\tif len(segs) > 0 {\n\t\t\t\tsegs = segs[:len(segs)-1]\n\t\t\t}', 1)]),
 ("M05", "WriteFile no longer refused", [(S, "func (c *Client) WriteFile(context.Context, string, io.Reader) error { return ErrReadOnly }", "func (c *Client) WriteFile(context.Context, string, io.Reader) error { return nil }", 1)]),
 ("M06", "an absent mtime is fabricated as now", [(S, "\t\tmt = time.Time{}", "\t\tmt = time.Now()", 1)]),
 ("M07", "retry budget off by one", [(S, "attempt >= c.retries()", "attempt > c.retries()", 1)]),
 ("M08", "server-side containment check disabled", [(S, "\tif c.cfg.SkipSymlinkCheck {\n\t\treturn nil\n\t}\n\trootReal := cn.rootReal", "\tif true {\n\t\treturn nil\n\t}\n\trootReal := cn.rootReal", 1)]),
 ("M09", "Credential.String leaks the password", [(C, 'return "Credential(redacted)"', 'return "Credential(" + c.Password + ")"', 1)]),
 ("M10", "Pin records whatever the server presents", [(H, "if all[i].Fingerprint == conf.Fingerprint {", "if true {", 1)]),
 ("M11", "Disconnect leaves the client re-dialable", [(S, "\tc.cur = nil\n\tc.state = stateDown\n\tc.mu.Unlock()", "\tc.cur = nil\n\tc.state = stateLost\n\tc.mu.Unlock()", 1)]),
 ("M13", "containment prefix accepts a sibling with the same prefix", [(S, 'strings.HasPrefix(p, strings.TrimSuffix(root, "/")+"/")', "strings.HasPrefix(p, root)", 1)]),
 ("M14", "FileExists reports a missing file as an error", [(S, "\tif errors.Is(err, os.ErrNotExist) {\n\t\treturn false, nil\n\t}\n\treturn false, err", "\treturn false, err", 1)]),
 # ---- NEW mechanisms
 ("N01", "cancellation guard never closes the connection", [(S, "tm = time.AfterFunc(g, func() { c.drop(cn) })", "tm = time.AfterFunc(g, func() {})", 1), (S, "\t\t\tgo c.drop(cn)\n", "", 1)]),
 ("N02", "no cancel grace: a cancel closes the shared connection at once", [(S, "\t\treturn DefaultCancelGrace\n", "\t\treturn 0\n", 1)]),
 ("N04", "no single-flight re-dial (SFTP-04 back)", [(S, "\t\t\tif f := c.flight; f != nil {", "\t\t\tif f := c.flight; f != nil && false {", 1)]),
 ("N05", "TestConnection disconnects the caller's client (SFTP-02 back)", [(S, "\t\treturn c.with(ctx, c.checkRoot)\n", "\t\terr := c.with(ctx, c.checkRoot)\n\t\t_ = c.Disconnect(ctx)\n\t\treturn err\n", 1)]),
 ("N06", "reply validator off (SFTP-08 back)", [(FR, "\tnames, err := validateFrame(b[4], b[5:])\n", "\tnames, err := 0, error(nil)\n\t_ = validateFrame\n", 1)]),
 ("N07", "reply validator accepts an over-long frame", [(FR, "if n < 1 || n > maxFrame {", "if n < 1 {", 1)]),
 ("N08", "no recover boundary", [(S, "if r := recover(); r != nil {", "if r := any(nil); r != nil {", 1)]),
 ("N09", "alive() always says alive", [(S, "\tcase err := <-res:\n\t\treturn err == nil\n", "\tcase err := <-res:\n\t\t_ = err\n\t\treturn true\n", 1)]),
 ("N21", "alive() treats a silent peer (timeout) as alive", [(S, "\tcase <-t.C:\n\t\treturn false\n\tcase <-cn.dead:", "\tcase <-t.C:\n\t\treturn true\n\tcase <-cn.dead:", 1)]),
 ("N10", "keepalive never runs", [(S, "\tif iv < 0 {\n\t\treturn\n\t}", "\tif iv < 0 || true {\n\t\treturn\n\t}", 1)]),
 ("N11", "listing limit not enforced", [(S, "if cn.maxEntries > 0 && cn.seen > cn.maxEntries {", "if false {", 1)]),
 ("N12", "ranged reads use a one-packet window (no pipelining)", [(S, "window := orInt(c.cfg.MaxPacket, DefaultMaxPacket) * orInt(c.cfg.MaxConcurrentRequests, DefaultMaxConcurrentRequests)", "window := 1024", 1)]),
 ("N13", "implicit re-dial installs over a Disconnect (SFTP-03 back)", [(S, "\t\t\tcase c.state != stateLost:", "\t\t\tcase false:", 1)]),
 ("N14", "bounded Close timer never fires", [(S, "t := time.AfterFunc(fl.c.closeTimeout(), func() { fl.c.drop(fl.cn) })", "t := time.AfterFunc(time.Hour, func() {})", 1)]),
 ("N15", "a STATUS EOF is reported as the bare io.EOF (transient, SFTP-05 back)", [(S, "\t\t\treturn &serverStatus{err: err}\n", "\t\t\treturn err\n", 1)]),
 ("N16", "a read that ends on a dead connection is an EOF", [(S, "if err == io.EOF && !fl.cn.alive() {", "if false {", 1)]),
 ("N17", "a copy that ends on a dead connection is a clean end", [(S, "if (err == nil || err == io.EOF) && !fl.cn.alive() {", "if false {", 1)]),
 ("N18", "malformed-reply fault not returned (a generic lost-connection error instead)", [(S, "\tif f := cn.faultErr(); f != nil {\n\t\tc.drop(cn)\n\t\treturn f\n\t}\n\tif cerr := ctx.Err(); cerr != nil && (", "\tif cerr := ctx.Err(); cerr != nil && (", 1)]),
 ("N19", "context error not mapped after a guard close", [(S, "\t\tc.drop(cn)\n\t\treturn cerr\n", "\t\tc.drop(cn)\n\t\treturn err\n", 1)]),
 ("N20", "server-side containment root not resolved at dial", [(S, "\t\tcn.rootReal = rr\n", "\t\t_ = rr\n", 1)]),
 ("L01", "a listing on a connection that ended during it is accepted", [(S, "\t\tif cn.isDead() {\n\t\t\treturn fmt.Errorf(\"%w: connection ended during the listing\"", "\t\tif false {\n\t\t\treturn fmt.Errorf(\"%w: connection ended during the listing\"", 1)]),
 ("Q01", "newFile publishes its stop function without the lock (nil-func panic / data race when the context has already ended; found while fixing, run with -race)", [(S, "\tfl.smu.Lock()\n\tfl.stop = context.AfterFunc(ctx, func() { _ = fl.Close() })\n\tfl.smu.Unlock()\n", "\tfl.stop = context.AfterFunc(ctx, func() { _ = fl.Close() })\n", 1), (S, "\t\tfl.smu.Lock()\n\t\tstop := fl.stop\n\t\tfl.smu.Unlock()\n\t\tstop()\n", "\t\tfl.stop()\n", 1)]),
 ("P01", "pin file writable by others is read", [(H, "\tif fi.Mode().Perm()&0o022 != 0 {", "\tif false {", 1)]),
 ("P02", "pin directory writable by others accepted", [(H, "\t\tif di.Mode().Perm()&0o022 != 0 && di.Mode()&os.ModeSticky == 0 {", "\t\tif false {", 1)]),
 ("P03", "no cross-process lock on the pin file", [(H, "\tif err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {", "\tif err := error(nil); err != nil {", 1)]),
 ("P04", "a non-regular pin file (symlink, directory) accepted", [(H, "\tif !fi.Mode().IsRegular() {", "\tif false {", 1)]),
 ("P05", "foreign owner accepted", [(H, "\treturn st.Uid == uint32(os.Getuid()) || st.Uid == 0", "\t_ = st\n\treturn true", 1)]),
 ("E01", "env credential refs not validated (SFTP-16 back)", [(C, "\tif !validRef(ref) {", "\tif false {", 1)]),
 ("E02", "env credential names fold case", [(C, "func envName(prefix, ref, field string) string { return prefix + \"_\" + ref + \"_\" + field }", "func envName(prefix, ref, field string) string {\n\treturn prefix + \"_\" + strings.ToUpper(ref) + \"_\" + field\n}", 1)]),
 ("F01", "factory accepts inline secrets other than 'password'", [(F, '"password", "passphrase", "private_key", "privatekey", "key", "secret", "token"', '"password"', 1)]),
 ("F02", "factory: an empty path wins over root", [(F, "\tif strings.TrimSpace(root) == \"\" {", "\tif strings.TrimSpace(root) == \"\" && false {", 1)]),
 ("F03", "factory: port not validated", [(F, "if !isNum || n < 1 || n > 65535 {", "if (!isNum || n < 1 || n > 65535) && false {", 1)]),
 ("D01", "Discover ignores ctx after the TCP connect", [(H, "stop := context.AfterFunc(ctx, func() { _ = conn.Close() })", "stop := func() bool { return true }", 1)]),
 ("K01", "no keyboard-interactive login (SFTP-12 back)", [(S, "methods = append(methods, ssh.Password(pw), ssh.KeyboardInteractive(kbdAnswer(pw)))", "methods = append(methods, ssh.Password(pw))", 1)]),
 ("K02", "keyboard-interactive failure not recognised as an auth failure", [(S, ' || strings.Contains(m, "unexpected message type 51 (expected 60)")', "", 1)]),
 ("K03", "keyboard-interactive answers echoed prompts with the password", [(S, "\t\t\tif i < len(echos) && echos[i] {", "\t\t\tif false {", 1)]),
]


def sh(cmd, cwd=None, env=None, timeout=1500):
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
        raise SystemExit("mutation pattern matched %d times (expected %d) in %s: %r" % (n, count, f, old[:80]))
    open(p, "w").write(s.replace(old, new))


RACE = {"Q01"}  # mutants that need the race detector to be visible
# only these mutants need pkg/factory (it imports pkg/ftp, which another worker edits concurrently: a transient break there must not invalidate pkg/sftp mutants)
FACTORY = {"Z00", "R09", "F01", "F02", "F03"}


def run_unit(d, cid, cache):
    cmd = ["bash", os.path.join(ROOT, "scripts/containers/run_pinned.sh"), "--out", cache, "--op-id", "sftpfixr2-" + cid.lower(), "IMG-GO", "--",
           "sh", "-c", "cd /src/filesystem && env GOTOOLCHAIN=local GOFLAGS=-mod=mod HOME=/out GOCACHE=/out/gocache GOMODCACHE=/out/gomod GOMAXPROCS=2 "
           "CGO_ENABLED=1 go test -count=1 -timeout 240s " + ("-race " if cid in RACE else "") + ("./pkg/sftp/ ./pkg/factory/" if cid in FACTORY else "./pkg/sftp/")]
    return sh(cmd, cwd=os.path.join(d, "view"))


def classify(rc, o):
    fails = sorted(set(re.findall(r"^--- FAIL: (\S+)", o, re.M)))
    if rc == 0:
        return "SURVIVED", []
    if re.search(r"\[build failed\]|\.go:\d+:\d+: ", o) and not fails:
        return "INVALID", []
    if "panic: test timed out" in o:
        fails.append("(timeout: the run did not complete)")
    if "signal: killed" in o and not fails:
        fails.append("(the test binary was killed: out of memory)")
    if re.search(r"^panic: ", o, re.M) and not fails:
        fails.append("(panic: the test binary crashed)")
    return ("KILLED", fails) if fails else ("ERROR", [])


def main():
    mode = sys.argv[1]
    if mode == "check":  # every pattern matches exactly `count` times in the checkout (no container, nothing written)
        bad = 0
        for cid, desc, edits in M:
            for f, old, new, count in edits:
                n = open(os.path.join(MOD, f)).read().count(old)
                if n != count:
                    bad += 1
                    print("MISMATCH", cid, f, n, "!=", count, repr(old[:70]))
        print("checked", len(M), "mutants,", bad, "mismatches")
        sys.exit(1 if bad else 0)
    if mode == "list":
        for cid, desc, edits in M:
            print(cid, len(edits), desc)
        return
    out = os.path.abspath(sys.argv[2])
    only = sys.argv[3].split(",") if len(sys.argv) > 3 else None
    cache = os.path.join(ROOT, ".audit", "out", "sftp-fix-r2")
    os.makedirs(out, exist_ok=True)
    res = []
    for cid, desc, edits in M:
        if only and cid not in only:
            continue
        t0 = time.time()
        d = make_copy(cid)
        for f, old, new, count in edits:
            apply(d, f, old, new, count)
        rc, o = run_unit(d, cid, cache)
        verdict, fails = classify(rc, o)
        open(os.path.join(out, cid + ".txt"), "w").write(o)
        res.append({"id": cid, "desc": desc, "verdict": verdict, "failing": fails[:12], "n_failing": len(fails), "seconds": round(time.time() - t0)})
        print(cid, verdict, len(fails), "failing", flush=True)
        shutil.rmtree(d, ignore_errors=True)
        json.dump(res, open(os.path.join(out, "results.json"), "w"), indent=1)
    k = [r["id"] for r in res if r["verdict"] == "KILLED"]
    print("TOTAL", len(res), "KILLED", len(k), "SURVIVED", [r["id"] for r in res if r["verdict"] == "SURVIVED"], "INVALID/ERROR", [r["id"] for r in res if r["verdict"] in ("INVALID", "ERROR")])


main()
