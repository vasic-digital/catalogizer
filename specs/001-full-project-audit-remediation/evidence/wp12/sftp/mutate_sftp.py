#!/usr/bin/env python3
# identity: WP-12 PA-05 mutation + RED harness for submodules/filesystem/pkg/sftp
# usage:   python3 -I mutate_sftp.py red|mutants|negcontrol <out-dir>
# Each case copies the module to .audit/scratch/sftp-mut/<id>/view/filesystem, applies ONE exact text replacement (asserted to match
# exactly the expected number of times), and runs the unit suite in the pinned IMG-GO container through scripts/containers/run_pinned.sh.
# The checked-in tree is never modified. A mutant is KILLED when the suite fails with at least one `--- FAIL`, INVALID when it does not
# compile, SURVIVED when the suite passes. The integration-only mutant (M12) runs through scripts/test-infra/sftp_fixture.sh with
# SFTP_FIXTURE_SRC pointing at the mutated copy.
import os, shutil, subprocess, sys, json, re, time

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), *[".."] * 5))
MOD = os.path.join(ROOT, "submodules", "filesystem")
SCR = os.path.join(ROOT, ".audit", "scratch", "sftp-mut")
ENV = dict(os.environ, TMPDIR="/dev/shm", DISK_HEADROOM_REPO_ROOT=ROOT, LONGOPS_ALLOW_TMPFS="1")

# (id, description, file, old, new, count, mode)  mode: unit | integration
M = [
 ("M01", "auth failure surfaces as a transient error (would be retried)", "pkg/sftp/sftp.go",
  'return &AuthError{User: c.cfg.Username, Host: hp, Err: err}',
  'return fmt.Errorf("sftp: auth %s: %w", hp, io.ErrUnexpectedEOF)', 1, "unit"),
 ("M02", "trust on first use: an unpinned host key is accepted", "pkg/sftp/hostkey.go",
  '\t\t\treturn *refusal\n\t\t}\n\t\tpinned := ',
  '\t\t\treturn nil\n\t\t}\n\t\tpinned := ', 1, "unit"),
 ("M03", "a CHANGED host key is accepted", "pkg/sftp/hostkey.go",
  'Presented: fp, Pinned: pinned}\n\t\treturn *refusal',
  'Presented: fp, Pinned: pinned}\n\t\treturn nil', 1, "unit"),
 ("M04", "path confinement clamps ../ at the root instead of refusing", "pkg/sftp/sftp.go",
  '\t\t\tif len(segs) == 0 {\n\t\t\t\treturn "", "", fmt.Errorf("%w: %q", ErrPathEscape, p)\n\t\t\t}\n\t\t\tsegs = segs[:len(segs)-1]',
  '\t\t\tif len(segs) > 0 {\n\t\t\t\tsegs = segs[:len(segs)-1]\n\t\t\t}', 1, "unit"),
 ("M05", "WriteFile is no longer refused", "pkg/sftp/sftp.go",
  'func (c *Client) WriteFile(context.Context, string, io.Reader) error { return ErrReadOnly }',
  'func (c *Client) WriteFile(context.Context, string, io.Reader) error { return nil }', 1, "unit"),
 ("M06", "an absent mtime is fabricated as now", "pkg/sftp/sftp.go",
  '\t\tmt = time.Time{}', '\t\tmt = time.Now()', 1, "unit"),
 ("M07", "retry budget off by one (one extra attempt)", "pkg/sftp/sftp.go",
  'attempt >= c.retries()', 'attempt > c.retries()', 1, "unit"),
 ("M08", "server-side symlink containment check disabled", "pkg/sftp/sftp.go",
  '\tif c.cfg.SkipSymlinkCheck {\n\t\treturn nil\n\t}\n\tc.mu.Lock()\n\trootReal',
  '\tif true {\n\t\treturn nil\n\t}\n\tc.mu.Lock()\n\trootReal', 1, "unit"),
 ("M09", "Credential.String leaks the password", "pkg/sftp/credential.go",
  'return "Credential(redacted)"', 'return "Credential(" + c.Password + ")"', 1, "unit"),
 ("M10", "Pin records whatever the server presents, ignoring the owner's fingerprint", "pkg/sftp/hostkey.go",
  'if all[i].Fingerprint == conf.Fingerprint {', 'if true {', 1, "unit"),
 ("M11", "a disconnected client reconnects implicitly", "pkg/sftp/sftp.go",
  '\tc.closeLocked()\n\tc.state = stateDown\n\treturn nil', '\tc.closeLocked()\n\tc.state = stateLost\n\treturn nil', 1, "unit"),
 ("M12", "pinned key type no longer restricts the offered host key algorithms (integration-only)", "pkg/sftp/sftp.go",
  'scfg.HostKeyAlgorithms = algs', '_ = algs', 1, "integration"),
 ("M13", "containment prefix test accepts a sibling directory with the same prefix", "pkg/sftp/sftp.go",
  'strings.HasPrefix(p, strings.TrimSuffix(root, "/")+"/")', 'strings.HasPrefix(p, root)', 1, "unit"),
 ("M14", "FileExists reports a missing file as an error", "pkg/sftp/sftp.go",
  '\tif errors.Is(err, os.ErrNotExist) {\n\t\treturn false, nil\n\t}\n\treturn false, err',
  '\treturn false, err', 1, "unit"),
]
NEG = [
 ("N01", "NEGATIVE CONTROL: unmutated copy, same harness (must PASS)", "pkg/sftp/sftp.go", None, None, 0, "unit"),
 ("N02", "NEGATIVE CONTROL: behaviour-neutral edit (a comment) (must SURVIVE; shows the harness reports survival)", "pkg/sftp/sftp.go",
  '// ErrReadOnly is returned by every mutating operation.', '// ErrReadOnly is returned by every mutating operation (edited).', 1, "unit"),
]
# RED skeleton: the behaviour is replaced by stubs, the package still compiles. All edits in one copy.
RED = [
 ("pkg/sftp/sftp.go", 'func (c *Client) dial(ctx context.Context) error {\n', 'func (c *Client) dial(ctx context.Context) error {\n\tif true {\n\t\treturn errors.New("not implemented")\n\t}\n'),
 ("pkg/sftp/sftp.go", 'func (c *Client) confine(p string) (logical, remote string, err error) {\n', 'func (c *Client) confine(p string) (logical, remote string, err error) {\n\tif true {\n\t\treturn p, p, nil\n\t}\n'),
 ("pkg/sftp/sftp.go", 'func IsTransient(err error) bool {\n', 'func IsTransient(err error) bool {\n\tif true {\n\t\treturn false\n\t}\n'),
 ("pkg/sftp/sftp.go", 'func (c *Client) retry(ctx context.Context, fn func() error) error {\n', 'func (c *Client) retry(ctx context.Context, fn func() error) error {\n\tif true {\n\t\treturn fn()\n\t}\n'),
 ("pkg/sftp/sftp.go", 'func (c *Client) WriteFile(context.Context, string, io.Reader) error { return ErrReadOnly }', 'func (c *Client) WriteFile(context.Context, string, io.Reader) error { return nil }'),
 ("pkg/sftp/sftp.go", 'func (c *Client) DeleteFile(context.Context, string) error { return ErrReadOnly }', 'func (c *Client) DeleteFile(context.Context, string) error { return nil }'),
 ("pkg/sftp/sftp.go", 'func (c *Client) CopyFile(context.Context, string, string) error { return ErrReadOnly }', 'func (c *Client) CopyFile(context.Context, string, string) error { return nil }'),
 ("pkg/sftp/sftp.go", 'func (c *Client) CreateDirectory(context.Context, string) error { return ErrReadOnly }', 'func (c *Client) CreateDirectory(context.Context, string) error { return nil }'),
 ("pkg/sftp/sftp.go", 'func (c *Client) DeleteDirectory(context.Context, string) error { return ErrReadOnly }', 'func (c *Client) DeleteDirectory(context.Context, string) error { return nil }'),
 ("pkg/sftp/sftp.go", '\tif mt.Unix() == 0 {', '\tif false {'),
 ("pkg/sftp/hostkey.go", 'func Pin(ctx context.Context, store PinStore, host string, port int, conf Confirmation) (*HostKeyPin, error) {\n', 'func Pin(ctx context.Context, store PinStore, host string, port int, conf Confirmation) (*HostKeyPin, error) {\n\tif true {\n\t\treturn nil, errors.New("not implemented")\n\t}\n'),
 ("pkg/sftp/hostkey.go", 'func DiscoverAll(ctx context.Context, host string, port int, timeout time.Duration) ([]Discovered, error) {\n', 'func DiscoverAll(ctx context.Context, host string, port int, timeout time.Duration) ([]Discovered, error) {\n\tif true {\n\t\treturn nil, errors.New("not implemented")\n\t}\n'),
 ("pkg/sftp/hostkey.go", 'func hostKeyCallback(hostport string, pins []HostKeyPin, refusal *error) ssh.HostKeyCallback {\n\treturn func(_ string, _ net.Addr, key ssh.PublicKey) error {\n', 'func hostKeyCallback(hostport string, pins []HostKeyPin, refusal *error) ssh.HostKeyCallback {\n\treturn func(_ string, _ net.Addr, key ssh.PublicKey) error {\n\t\tif true {\n\t\t\treturn nil\n\t\t}\n'),
 ("pkg/sftp/credential.go", 'func (c *Credential) String() string {\n', 'func (c *Credential) String() string {\n\tif c != nil {\n\t\treturn c.Password\n\t}\n'),
]


def sh(cmd, cwd=None, env=None, timeout=900):
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
        raise SystemExit("mutation pattern matched %d times (expected %d) in %s: %r" % (n, count, f, old[:60]))
    open(p, "w").write(s.replace(old, new))


def run_unit(d, cid, out):
    os.makedirs(os.path.join(d, "out"), exist_ok=True)
    cmd = ["bash", os.path.join(ROOT, "scripts/containers/run_pinned.sh"), "--out", os.path.join(d, "out"), "--op-id", "sftpmut-" + cid.lower(), "IMG-GO", "--",
           "env", "GOMAXPROCS=3", "GOTOOLCHAIN=local", "CGO_ENABLED=1", "GOFLAGS=-buildvcs=false", "sh", "-c",
           "cd /src/filesystem && go test -count=1 -timeout 150s ./pkg/sftp/ ./pkg/factory/"]
    return sh(cmd, cwd=os.path.join(d, "view"))


def classify(rc, o):
    fails = sorted(set(re.findall(r"^--- FAIL: (\S+)", o, re.M)))
    if rc == 0:
        return "SURVIVED", []
    if fails:
        return "KILLED", fails
    if "[build failed]" in o or "build failed" in o or re.search(r"\.go:\d+:\d+:", o):
        return "INVALID", []
    return "ERROR", []


def main():
    mode, out = sys.argv[1], os.path.abspath(sys.argv[2])
    os.makedirs(out, exist_ok=True)
    res = []
    if mode == "red":
        d = make_copy("RED")
        for f, old, new in RED:
            apply(d, f, old, new, 1)
        rc, o = run_unit(d, "RED", out)
        open(os.path.join(out, "red-raw.txt"), "w").write(o)
        total = len(re.findall(r"^=== RUN\s+(\S+)", o, re.M))
        runs = set(re.findall(r"^=== RUN\s+(\S+)", o, re.M))
        fails = sorted(set(re.findall(r"^--- FAIL: (\S+)", o, re.M)))
        print(json.dumps({"mode": "red", "rc": rc, "failing_tests": len(fails), "names": fails}, indent=1))
        shutil.rmtree(d, ignore_errors=True)
        return
    cases = {"mutants": M, "negcontrol": NEG}[mode]
    only = os.environ.get("MUT_ONLY")  # e.g. MUT_ONLY=M12 MUT_FORCE_UNIT=1: run one mutant through the unit suite only
    for cid, desc, f, old, new, count, kind in cases:
        if only and cid not in only.split(","):
            continue
        if os.environ.get("MUT_FORCE_UNIT"):
            kind = "unit"
        t0 = time.time()
        d = make_copy(cid)
        if old is not None:
            apply(d, f, old, new, count)
        if kind == "unit":
            rc, o = run_unit(d, cid, out)
            verdict, fails = classify(rc, o)
        else:
            env = dict(ENV, SFTP_FIXTURE_SRC=os.path.join(d, "view", "filesystem"), SFTP_FIXTURE_ALLOW_SRC="1")
            logf = os.path.join(out, cid + "-integration.txt")
            rc, o = sh(["bash", os.path.join(ROOT, "scripts/test-infra/sftp_fixture.sh"), "run", "--log", logf, "--", "-v", "./pkg/sftp/"], env=env)
            o = open(logf).read() if os.path.exists(logf) else o
            verdict, fails = classify(rc, o)
        open(os.path.join(out, cid + ".txt"), "w").write(o)
        res.append({"id": cid, "desc": desc, "kind": kind, "verdict": verdict, "failing_tests": fails, "seconds": round(time.time() - t0)})
        print(cid, verdict, len(fails), "failing", flush=True)
        shutil.rmtree(d, ignore_errors=True)
    json.dump(res, open(os.path.join(out, "%s.json" % mode), "w"), indent=1)
    k = sum(1 for r in res if r["verdict"] == "KILLED")
    print("TOTAL", len(res), "KILLED", k, "SURVIVED", [r["id"] for r in res if r["verdict"] == "SURVIVED"], "INVALID", [r["id"] for r in res if r["verdict"] in ("INVALID", "ERROR")])


main()
