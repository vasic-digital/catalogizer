#!/usr/bin/env python3
"""test_snapshot_checks.py - tests of the T161 stand-ins snapshot_manifest.py and check_freeze_snapshot.sh (WP-20; findings F3, F4 of the WF23 review
and the manifest rule of tasks.md T161): one entry per regular file and per symlink, a symlink hashed over its readlink string and never followed,
sorted byte order, a path that is not UTF-8 refused freeze_path_unsafe, a FIFO refused freeze_special_file instead of blocking forever, a
malformed or unreadable manifest refused freeze_manifest_invalid (exit 20) instead of a traceback, each added / missing / changed path named.
Env: none needed; writes below TMPDIR only. Exit 0 only when every check passed."""
import hashlib, json, os, subprocess, sys, tempfile

HERE = os.path.dirname(os.path.abspath(__file__)); REG = os.path.dirname(HERE)
MAN = os.path.join(REG, "snapshot_manifest.py"); CHK = os.path.join(REG, "check_freeze_snapshot.sh")
FAILS = 0


def check(label, cond, info=""):
    global FAILS
    print(("ok   " if cond else "FAIL ") + label + ("" if cond else " :: " + str(info)[:300])); FAILS += 0 if cond else 1


def man(d, timeout=20):
    return subprocess.run([sys.executable, MAN, d], capture_output=True, text=True, timeout=timeout)


def chk(d, m, timeout=20):
    return subprocess.run([CHK, d, m], capture_output=True, text=True, timeout=timeout)


with tempfile.TemporaryDirectory(dir=os.environ.get("TMPDIR")) as w:
    t = os.path.join(w, "t"); os.makedirs(os.path.join(t, "d/e")); open(os.path.join(t, "a.txt"), "w").write("alpha\n"); open(os.path.join(t, "d/e/b.txt"), "w").write("beta\n")
    os.symlink("a.txt", os.path.join(t, "link")); os.symlink("d", os.path.join(t, "dirlink")); os.symlink("/nonexistent/target", os.path.join(t, "dangling"))
    r = man(t); check("manifest of a plain tree exits 0", r.returncode == 0, r.stderr)
    mj = json.loads(r.stdout); ents = {e["path"]: e["sha256"] for e in mj["entries"]}
    check("entries are the files and the symlinks, sorted by path, directories are not entries", [e["path"] for e in mj["entries"]] == sorted(ents) and set(ents) == {"a.txt", "d/e/b.txt", "dangling", "dirlink", "link"}, list(ents))
    check("a regular file is hashed over its bytes", ents["a.txt"] == hashlib.sha256(b"alpha\n").hexdigest())
    check("a symlink is hashed over its readlink string (no newline), never followed - also a dangling one and one to a directory",
          ents["link"] == hashlib.sha256(b"a.txt").hexdigest() and ents["dangling"] == hashlib.sha256(b"/nonexistent/target").hexdigest() and ents["dirlink"] == hashlib.sha256(b"d").hexdigest(), ents)
    mf = os.path.join(w, "m.json"); open(mf, "w").write(r.stdout)
    r = chk(t, mf); check("an unchanged snapshot passes the check (exit 0, silent)", r.returncode == 0 and r.stderr == "", (r.returncode, r.stderr))
    open(os.path.join(t, "a.txt"), "a").write("x"); os.remove(os.path.join(t, "d/e/b.txt")); open(os.path.join(t, "new.txt"), "w").write("n")
    r = chk(t, mf); check("changed, missing and added paths are each named, exit 20 freeze_snapshot_moved", r.returncode == 20 and all(x in r.stderr for x in ("added new.txt", "missing d/e/b.txt", "changed a.txt")) and r.stderr.count("freeze_snapshot_moved") == 3, (r.returncode, r.stderr))
    # F4: a FIFO is refused, not read forever
    f = os.path.join(w, "f"); os.makedirs(f); os.mkfifo(os.path.join(f, "pipe")); open(os.path.join(f, "ok.txt"), "w").write("ok")
    try:
        r = man(f, timeout=10); check("F4 a FIFO in the tree is refused freeze_special_file (exit 20) instead of blocking", r.returncode == 20 and "freeze_special_file" in r.stderr and "pipe" in r.stderr, (r.returncode, r.stderr))
    except subprocess.TimeoutExpired:
        check("F4 a FIFO in the tree is refused freeze_special_file instead of blocking forever (TIMED OUT)", False)
    # non-UTF-8 path
    u = os.path.join(w, "u"); os.makedirs(u); open(os.path.join(os.fsencode(u), b"bad\xff.txt"), "w").write("x")
    r = man(u); check("a path that is not valid UTF-8 is refused freeze_path_unsafe (exit 20)", r.returncode == 20 and "freeze_path_unsafe" in r.stderr, (r.returncode, r.stderr))
    # F3: malformed manifests
    ok = os.path.join(w, "ok"); os.makedirs(ok); open(os.path.join(ok, "x"), "w").write("x")
    for label, body in (("not JSON", "{broken"), ("no entries key", "{}"), ("entries not a list of objects", '{"entries": [1]}'), ("an entry without path", '{"entries": [{"sha256": "00"}]}')):
        bad = os.path.join(w, "bad.json"); open(bad, "w").write(body)
        r = chk(ok, bad); check("F3 a malformed manifest (%s) is refused freeze_manifest_invalid (exit 20), no traceback" % label, r.returncode == 20 and "freeze_manifest_invalid" in r.stderr and "Traceback" not in r.stderr, (r.returncode, r.stderr))
    r = subprocess.run([CHK, ok, os.path.join(w, "absent.json")], capture_output=True, text=True); check("an absent manifest is usage (exit 2)", r.returncode == 2, (r.returncode, r.stderr))
    # the enumerator and the lead scan name the right reason
    sys.path.insert(0, HERE)
    import wp20_fixture as F
    tree = os.path.join(w, "tree"); F.build(tree); fj = os.path.join(w, "tree.json"); F.freeze(tree, fj)
    open(fj + ".manifest.json", "w").write("{broken")
    r = subprocess.run([sys.executable, os.path.join(REG, "enumerate_sources.py"), "--freeze-json", fj, "--out", os.path.join(w, "o")], capture_output=True, text=True)
    check("the enumerator reports freeze_manifest_invalid (not freeze_snapshot_moved) for a malformed manifest, nothing written", r.returncode == 20 and "enumerate_sources: REFUSED reason=freeze_manifest_invalid" in r.stderr and not os.path.exists(os.path.join(w, "o")), (r.returncode, r.stderr))
print("RESULT: fail=%d" % FAILS)
sys.exit(1 if FAILS else 0)
