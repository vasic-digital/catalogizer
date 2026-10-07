#!/usr/bin/env python3
"""snapshot.py - T005b: the git-plumbing source snapshot, the input closure, the tar writer and the object pack of the build dispatcher
(driver side; the build-host side is treecache.py). Constitution 11.4.173 (build on a remote host), 11.4.232, 11.4.50 (deterministic).

  snapshot.py manifest --root R [--class compile|full] [--exclude P]... [--mode tic|cpa] [--declare P]... [--component C] [--tmp DIR]
        JSON: one entry per repository (the root "." and every INITIALISED submodule at every depth, in path order): its tree id written
        from that repository's OWN temporary index (GIT_INDEX_FILE under --tmp, never the real index) holding its HEAD plus its
        uncommitted content, its gitlinks (path -> commit), and the digest = sha256 of the "path TAB tree" lines.
        tic (default): the temporary index is seeded from a copy of the real index (stat cache reused) and every changed, deleted or
        untracked non-ignored path is hashed with `git hash-object --no-filters -w` and loaded with `git update-index --index-info`,
        never `git add`. cpa: seeded from HEAD (`git read-tree HEAD`) and exactly the --declare paths are loaded; with --component an
        undeclared dirty file inside that component's input closure is refused (exit 20 undeclared_dirty_input).
        Every git call runs with the LFS driver disabled (-c filter.lfs.process= -c filter.lfs.clean=cat -c filter.lfs.smudge=cat
        -c filter.lfs.required=false) and core.attributesFile=/dev/null: export-ignore, export-subst and filter=lfs are never applied.
        class compile removes every --exclude path (evidence and audit trees: they are no build input) from the root tree; class full keeps them.
  snapshot.py digest   <same options>                       the digest only
  snapshot.py closure  --root R --component C [--containerfile F --context D]
        JSON {repos, inputs}: the root and the submodules the component's inputs reach (Go replace and go.work use, npm file: and workspaces,
        Cargo path dependencies, Gradle includeBuild, Containerfile COPY and ADD sources, compose build.context), found transitively.
        A reference into a submodule that is not initialised: exit 20 submodule_uninitialised.
  snapshot.py plan     --manifest M --repos A,B [--root R --component C ...]   "b <id>" / "t <id>" lines: every object of the shipped trees
  snapshot.py pack     --manifest M --repos A,B --root R < ids               tar of the named objects (members b/<id>, t/<id>; raw git objects)
  snapshot.py tar      --manifest M --repos A,B --root R                       tar of the shipped trees (attribute-free: ls-tree + cat-file, never git archive)
  With --component, plan/pack/tar refuse (exit 20 closure_incomplete) a path the component references inside a gitlink the shipped set lacks.
Exit 20 prints `REFUSED reason=<reason> <detail>` on stderr (the dispatcher's convention)."""
import argparse, hashlib, json, os, re, stat, subprocess, sys, tarfile, tempfile

LFS_OFF = ["-c", "filter.lfs.process=", "-c", "filter.lfs.clean=cat", "-c", "filter.lfs.smudge=cat", "-c", "filter.lfs.required=false",
           "-c", "core.attributesFile=/dev/null", "-c", "core.fsmonitor=false", "-c", "core.hooksPath=/dev/null", "-c", "core.autocrlf=false"]
EMPTY_TREE = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"
PRUNE = {".git", "node_modules", "target", ".gradle", "__pycache__"}


class Refused(Exception):
    def __init__(self, reason, detail=""):
        Exception.__init__(self, reason)
        self.reason, self.detail = reason, detail


def genv(index=None):
    e = {k: v for k, v in os.environ.items() if k not in ("GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_OBJECT_DIRECTORY", "GIT_ALTERNATE_OBJECT_DIRECTORIES")}
    e["GIT_OPTIONAL_LOCKS"] = "0"
    e["GIT_ATTR_NOSYSTEM"] = "1"
    e["GIT_TERMINAL_PROMPT"] = "0"
    if index:
        e["GIT_INDEX_FILE"] = index
    return e


def git(repo, args, index=None, inp=None, check=True):
    p = subprocess.run(["git"] + LFS_OFF + args, cwd=repo, env=genv(index), input=inp, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    if check and p.returncode != 0:
        raise Refused("git_failed", "%s in %s: %s" % (" ".join(args[:2]), repo, p.stderr.decode("utf-8", "replace").strip()[:200]))
    return p


def real_index(repo):
    p = git(repo, ["rev-parse", "--git-path", "index"])
    path = p.stdout.decode().strip()
    return path if os.path.isabs(path) else os.path.join(repo, path)


def gitlinks_of(repo, index=None):
    """path -> commit for every mode-160000 entry of the (real or given) index."""
    out = git(repo, ["ls-files", "-s", "-z"], index=index).stdout
    res = {}
    for ent in out.split(b"\0"):
        if not ent:
            continue
        meta, path = ent.split(b"\t", 1)
        mode, sha = meta.split()[0], meta.split()[1]
        if mode == b"160000":
            res[os.fsdecode(path)] = sha.decode()
    return res


def initialised(base, rel):
    return os.path.exists(os.path.join(base, rel, ".git"))


def discover(root):
    """{relpath: {"abs","gitlinks":{...}}} for the root and every initialised submodule at every depth; plus the uninitialised gitlink paths."""
    repos, uninit = {}, set()

    def rec(abs_, rel):
        gl = gitlinks_of(abs_)
        repos[rel] = {"abs": abs_, "gitlinks": gl}
        for g in gl:
            full = g if rel == "." else rel + "/" + g
            if initialised(abs_, g):
                rec(os.path.join(abs_, g), full)
            else:
                uninit.add(full)
    rec(os.path.abspath(root), ".")
    return repos, uninit


# ------------------------------------------------------------------ snapshot
def under(path, prefix):
    return path == prefix or path.startswith(prefix.rstrip("/") + "/")


def tree_of_repo(abs_, rel, a, tmpdir):
    """-> (tree id, head, gitlinks). The real index and the working tree are only read."""
    head = git(abs_, ["rev-parse", "-q", "--verify", "HEAD"], check=False).stdout.decode().strip() or None
    idx = os.path.join(tmpdir, "idx.%d.%s" % (os.getpid(), hashlib.sha1(rel.encode()).hexdigest()[:10]))
    try:
        if a.mode == "cpa" or not os.path.exists(real_index(abs_)):
            if head:
                git(abs_, ["read-tree", "HEAD"], index=idx)
            else:
                open(idx, "wb").close()
        else:
            with open(real_index(abs_), "rb") as s, open(idx, "wb") as d:
                d.write(s.read())
        gl = gitlinks_of(abs_, index=idx)
        # dirty paths of the working tree (tic) or the declared paths (cpa)
        if a.mode == "cpa":
            decl = [p for p in (a.declare or []) if rel == "." or under(p, rel)]
            paths = sorted(set((p if rel == "." else p[len(rel) + 1:]).encode() for p in decl))
        else:
            out = git(abs_, ["ls-files", "-m", "-d", "-o", "--exclude-standard", "-z"], index=idx).stdout
            paths = sorted(set(p for p in out.split(b"\0") if p and not p.endswith(b"/")))
        excl = [e for e in (a.exclude or [])] if (a.klass == "compile" and rel == ".") else []
        lines, removes, regs = [], [], []
        for p in paths:
            ps = os.fsdecode(p)
            if ps in gl or any(under(ps, g) for g in gl):
                continue
            if any(under(ps, e) for e in excl):
                continue
            fp = os.path.join(abs_, ps)
            try:
                st = os.lstat(fp)
            except FileNotFoundError:
                removes.append(p)
                continue
            if stat.S_ISDIR(st.st_mode):
                continue
            if stat.S_ISLNK(st.st_mode):
                sha = git(abs_, ["hash-object", "--no-filters", "-w", "--stdin"], inp=os.fsencode(os.readlink(fp))).stdout.decode().strip()
                lines.append(b"120000 " + sha.encode() + b"\t" + p)
            elif stat.S_ISREG(st.st_mode):
                if b"\n" in p:
                    sha = git(abs_, ["hash-object", "--no-filters", "-w", "--", ps]).stdout.decode().strip()
                    lines.append(("%s %s\t" % ("100755" if st.st_mode & 0o100 else "100644", sha)).encode() + p)
                else:
                    regs.append((p, "100755" if st.st_mode & 0o100 else "100644"))
        if regs:
            out = git(abs_, ["hash-object", "--no-filters", "-w", "--stdin-paths"], inp=b"\n".join(p for p, _ in regs) + b"\n").stdout.decode().split()
            if len(out) != len(regs):
                raise Refused("git_failed", "hash-object returned %d ids for %d paths" % (len(out), len(regs)))
            for (p, m), sha in zip(regs, out):
                lines.append(("%s %s\t" % (m, sha)).encode() + p)
        if removes:
            git(abs_, ["update-index", "--force-remove", "-z", "--stdin"], index=idx, inp=b"\0".join(removes) + b"\0")
        if lines:
            git(abs_, ["update-index", "--add", "-z", "--index-info"], index=idx, inp=b"\0".join(lines) + b"\0")
        for e in excl:
            git(abs_, ["rm", "--cached", "-r", "-q", "--ignore-unmatch", "--", e], index=idx)
        tree = git(abs_, ["write-tree"], index=idx).stdout.decode().strip()
        return tree, head, gitlinks_of(abs_, index=idx)
    finally:
        try:
            os.unlink(idx)
        except OSError:
            pass


def build_manifest(a):
    root = os.path.abspath(a.root)
    tmpdir = a.tmp or tempfile.mkdtemp(prefix="snap.")
    os.makedirs(tmpdir, exist_ok=True)
    out = []

    def rec(abs_, rel):
        tree, head, gl = tree_of_repo(abs_, rel, a, tmpdir)
        out.append({"path": rel, "tree": tree, "head": head, "gitlinks": gl})
        for g in sorted(gl):
            if initialised(abs_, g):
                rec(os.path.join(abs_, g), g if rel == "." else rel + "/" + g)
    rec(root, ".")
    out.sort(key=lambda r: "" if r["path"] == "." else r["path"])
    text = "".join("%s\t%s\n" % (r["path"], r["tree"]) for r in out)
    m = {"schema": "snapshot-manifest/1", "class": a.klass, "excludes": list(a.exclude or []), "mode": a.mode,
         "repos": out, "digest": hashlib.sha256(text.encode()).hexdigest()}
    if a.mode == "cpa" and a.component:
        check_undeclared(root, a, m)
    if not a.tmp:
        try:
            os.rmdir(tmpdir)
        except OSError:
            pass
    return m


# ------------------------------------------------------------------ closure
GO_REPLACE = re.compile(r"^\s*(?:replace\s+)?\S+(?:\s+v\S+)?\s*=>\s*(\S+)")
CF_NAMES = re.compile(r"^(Containerfile|Dockerfile)(\..+)?$|\.(Containerfile|Dockerfile)$")
COMPOSE = re.compile(r"^(docker-|podman-)?compose([.-].+)?\.ya?ml$")


def lit_prefix(p):
    m = re.search(r"[*?\[]", p)
    return p[:m.start()].rstrip("/") if m else p


def refs_in_file(fp, ctx_override=None):
    """-> list of (path relative to the directory the reference is resolved against, resolved base dir)."""
    name = os.path.basename(fp)
    base = os.path.dirname(fp)
    res = []
    try:
        text = open(fp, encoding="utf-8", errors="replace").read()
    except OSError:
        return res
    if name in ("go.mod", "go.work"):
        inblock = None
        for line in text.splitlines():
            s = line.split("//")[0].strip()
            if not s:
                continue
            if name == "go.work" and (s == "use (" or s == "replace ("):
                inblock = s.split()[0]
                continue
            if s == "replace (":
                inblock = "replace"
                continue
            if s == ")":
                inblock = None
                continue
            if name == "go.work" and (s.startswith("use ") or inblock == "use"):
                tok = s[4:].strip() if s.startswith("use ") else s
                res.append((tok.strip('"'), base))
                continue
            m = GO_REPLACE.match(s if (s.startswith("replace") or inblock == "replace") else "")
            if m and re.match(r"^(\./|\.\./|/)", m.group(1).strip('"')):
                res.append((m.group(1).strip('"'), base))
    elif name == "package.json":
        try:
            j = json.loads(text)
        except ValueError:
            return res
        for k in ("dependencies", "devDependencies", "peerDependencies", "optionalDependencies", "resolutions", "overrides"):
            d = j.get(k)
            if isinstance(d, dict):
                for v in d.values():
                    if isinstance(v, str) and re.match(r"^(file|link):", v):
                        res.append((v.split(":", 1)[1], base))
        ws = j.get("workspaces")
        if isinstance(ws, dict):
            ws = ws.get("packages")
        if isinstance(ws, list):
            for w in ws:
                if isinstance(w, str):
                    res.append((lit_prefix(w), base))
    elif name == "Cargo.toml":
        for m in re.finditer(r'path\s*=\s*"([^"]+)"', text):
            res.append((m.group(1), base))
    elif re.match(r"^(settings|build)\.gradle(\.kts)?$", name):
        for m in re.finditer(r"""includeBuild\s*\(?\s*['"]([^'"]+)['"]""", text):
            res.append((m.group(1), base))
        for m in re.finditer(r"""projectDir\s*=\s*(?:new\s+)?file\s*\(\s*['"]([^'"]+)['"]""", text):
            res.append((m.group(1), base))
    elif CF_NAMES.search(name):
        ctx = ctx_override or base
        for line in text.splitlines():
            m = re.match(r"^\s*(COPY|ADD)\s+(.*)$", line, re.I)
            if not m:
                continue
            rest = m.group(2).strip()
            if rest.startswith("["):
                try:
                    toks = json.loads(rest)
                except ValueError:
                    continue
            else:
                toks = rest.split()
            toks = [t for t in toks if not t.startswith("--")]
            if any(t.startswith("--from") for t in m.group(2).split()):
                continue
            for src in toks[:-1]:
                if src.startswith(("http://", "https://", "git@")) or "$" in src:
                    continue
                res.append((lit_prefix(src) or ".", ctx))
    elif COMPOSE.match(name):
        try:
            import yaml
            j = yaml.safe_load(text) or {}
        except Exception:
            j = {}
        for svc in ((j.get("services") or {}) if isinstance(j, dict) else {}).values():
            b = (svc or {}).get("build") if isinstance(svc, dict) else None
            ctx = b if isinstance(b, str) else (b or {}).get("context") if isinstance(b, dict) else None
            if ctx:
                res.append((ctx, base))
                df = (b or {}).get("dockerfile") if isinstance(b, dict) else None
                cdir = os.path.normpath(os.path.join(base, ctx))
                cf = os.path.join(cdir, df or "Dockerfile")
                if os.path.isfile(cf):
                    res.extend(refs_in_file(cf, cdir))
    return res


def owner_of(path, repos, uninit):
    """longest gitlink prefix of path: (relpath of the owning submodule, initialised?) or None for the root."""
    best = None
    cand = list(repos) + sorted(uninit)
    for g in cand:
        if g != "." and under(path, g) and (best is None or len(g) > len(best)):
            best = g
    if best is None:
        return None
    return best, best in repos


def closure_of(a):
    root = os.path.abspath(a.root)
    repos, uninit = discover(root)
    comp = os.path.normpath(a.component)
    seen, queue, inputs = set(), [], set()
    start = os.path.join(root, comp)
    queue.append((start, None))
    for cf in (a.containerfile or []):
        queue.append((os.path.join(root, cf), os.path.normpath(os.path.join(root, a.context)) if a.context else None))
    need = {"."}
    inputs.add(comp)
    while queue:
        fp, ctx = queue.pop()
        key = (fp, ctx)
        if key in seen:
            continue
        seen.add(key)
        files = []
        if os.path.isdir(fp):
            for dp, dn, fn in os.walk(fp):
                dn[:] = sorted(d for d in dn if d not in PRUNE)
                files.extend(os.path.join(dp, f) for f in sorted(fn))
        elif os.path.isfile(fp):
            files = [fp]
        for f in files:
            for ref, base in refs_in_file(f, ctx):
                tgt = os.path.normpath(os.path.join(base, ref))
                if not under(tgt, root):
                    continue
                rel = os.path.relpath(tgt, root)
                rel = "." if rel == "." else rel
                inputs.add(rel)
                o = owner_of(rel, repos, uninit)
                if o is not None:
                    g, ini = o
                    if not ini:
                        raise Refused("submodule_uninitialised", "%s is inside the submodule %s, which is not initialised (referenced by %s)" % (rel, g, os.path.relpath(f, root)))
                    need.add(g)
                need.update(g for g in repos if g != "." and under(g, rel) and rel != ".")   # a referenced directory holds the initialised submodules below it
                if os.path.exists(tgt):
                    queue.append((tgt, None))
    need.update(g for g in repos if g != "." and under(g, comp) and comp != ".")
    # the component itself may live inside a submodule
    o = owner_of(comp, repos, uninit)
    if o is not None:
        if not o[1]:
            raise Refused("submodule_uninitialised", "%s is inside the submodule %s, which is not initialised" % (comp, o[0]))
        need.add(o[0])
    # a nested submodule needs every ancestor submodule
    for g in list(need):
        for r in repos:
            if r != "." and g != r and under(g, r):
                need.add(r)
    return sorted(need, key=lambda r: "" if r == "." else r), sorted(inputs), repos, uninit


def check_undeclared(root, a, m):
    need, inputs, repos, _ = closure_of(a)
    declared = set(a.declare or [])
    excl = a.exclude or []
    for rel, info in repos.items():
        if rel not in need:
            continue
        out = git(info["abs"], ["ls-files", "-m", "-d", "-o", "--exclude-standard", "-z"]).stdout
        out += b"\0" + git(info["abs"], ["diff-index", "--name-only", "-z", "HEAD"], check=False).stdout
        for p in set(x for x in out.split(b"\0") if x and not x.endswith(b"/")):
            full = os.fsdecode(p) if rel == "." else rel + "/" + os.fsdecode(p)
            if full in declared or any(under(full, e) for e in excl):
                continue
            if any(under(full, i) for i in inputs if i != "."):
                raise Refused("undeclared_dirty_input", "%s is dirty inside the input closure of %s and is not declared" % (full, a.component))


# ------------------------------------------------------------------ objects, tar
def repo_map(a, m):
    base = os.path.abspath(a.root)
    return {r["path"]: (base if r["path"] == "." else os.path.join(base, r["path"]), r) for r in m["repos"]}


def shipped(a, m):
    rm = repo_map(a, m)
    names = [x for x in a.repos.split(",") if x]
    for n in names:
        if n not in rm:
            raise Refused("closure_incomplete", "repository %s is not in the manifest" % n)
    if a.component:
        need, _, _, _ = closure_of(a)
        for n in need:
            if n not in names:
                raise Refused("closure_incomplete", "the input closure of %s reaches %s, which is not shipped" % (a.component, n))
    return [(n, rm[n][0], rm[n][1]) for n in names]


def ls_tree(abs_, tree):
    out = git(abs_, ["ls-tree", "-r", "-t", "-z", "--full-tree", tree]).stdout
    for ent in out.split(b"\0"):
        if not ent:
            continue
        meta, path = ent.split(b"\t", 1)
        mode, typ, sha = meta.split()
        yield mode.decode(), typ.decode(), sha.decode(), os.fsdecode(path)


def cmd_plan(a):
    m = json.load(open(a.manifest))
    ids = set()
    for _, abs_, r in shipped(a, m):
        ids.add("t " + r["tree"])
        for mode, typ, sha, _p in ls_tree(abs_, r["tree"]):
            if typ == "blob":
                ids.add("b " + sha)
            elif typ == "tree":
                ids.add("t " + sha)
    sys.stdout.write("".join(x + "\n" for x in sorted(ids)))


class Batch:
    def __init__(self, abs_):
        self.p = subprocess.Popen(["git"] + LFS_OFF + ["cat-file", "--batch"], cwd=abs_, env=genv(), stdin=subprocess.PIPE, stdout=subprocess.PIPE)

    def get(self, sha):
        self.p.stdin.write(sha.encode() + b"\n")
        self.p.stdin.flush()
        hdr = self.p.stdout.readline().split()
        if len(hdr) < 3:
            raise Refused("git_failed", "object %s is missing" % sha)
        n = int(hdr[2])
        data = self.p.stdout.read(n)
        self.p.stdout.read(1)
        return hdr[1].decode(), data

    def close(self):
        self.p.stdin.close()
        self.p.wait()


def tinfo(name, size, mode=0o644, typ=tarfile.REGTYPE, link=""):
    ti = tarfile.TarInfo(name)
    ti.size, ti.mode, ti.type, ti.linkname = size, mode, typ, link
    ti.mtime = ti.uid = ti.gid = 0
    ti.uname = ti.gname = ""
    return ti


def cmd_pack(a):
    m = json.load(open(a.manifest))
    want = set(l.strip() for l in sys.stdin.read().splitlines() if l.strip())
    tf = tarfile.open(fileobj=sys.stdout.buffer, mode="w|", format=tarfile.PAX_FORMAT)
    done = set()
    for _, abs_, r in shipped(a, m):
        b = Batch(abs_)
        try:
            objs = [("t", r["tree"])] + [("b" if t == "blob" else "t", s) for _m, t, s, _p in ls_tree(abs_, r["tree"]) if t in ("blob", "tree")]
            for k, sha in objs:
                key = "%s %s" % (k, sha)
                if key in want and key not in done:
                    typ, data = b.get(sha)
                    tf.addfile(tinfo("%s/%s" % (k, sha), len(data)), fileobj=__import__("io").BytesIO(data))
                    done.add(key)
        finally:
            b.close()
    tf.close()
    if done != want:
        raise Refused("git_failed", "%d requested objects are in no shipped tree" % len(want - done))


def cmd_tar(a):
    m = json.load(open(a.manifest))
    entries = []
    for n, abs_, r in shipped(a, m):
        for mode, typ, sha, p in ls_tree(abs_, r["tree"]):
            if typ == "blob":
                entries.append(((p if n == "." else n + "/" + p), mode, sha, abs_))
    entries.sort(key=lambda e: e[0])
    tf = tarfile.open(fileobj=sys.stdout.buffer, mode="w|", format=tarfile.PAX_FORMAT)
    batches = {}
    try:
        for full, mode, sha, abs_ in entries:
            b = batches.get(abs_) or batches.setdefault(abs_, Batch(abs_))
            _t, data = b.get(sha)
            if mode == "120000":
                tf.addfile(tinfo(full, 0, 0o777, tarfile.SYMTYPE, os.fsdecode(data)))
            else:
                tf.addfile(tinfo(full, len(data), 0o755 if mode == "100755" else 0o644), fileobj=__import__("io").BytesIO(data))
    finally:
        for b in batches.values():
            b.close()
    tf.close()


def main():
    ap = argparse.ArgumentParser(prog="snapshot.py")
    ap.add_argument("cmd", choices=["manifest", "digest", "closure", "plan", "pack", "tar"])
    ap.add_argument("--root")
    ap.add_argument("--class", dest="klass", default="compile", choices=["compile", "full"])
    ap.add_argument("--exclude", action="append")
    ap.add_argument("--mode", default="tic", choices=["tic", "cpa"])
    ap.add_argument("--declare", action="append")
    ap.add_argument("--component")
    ap.add_argument("--containerfile", action="append")
    ap.add_argument("--context")
    ap.add_argument("--tmp")
    ap.add_argument("--manifest")
    ap.add_argument("--repos", default=".")
    a = ap.parse_args()
    try:
        if a.cmd in ("manifest", "digest", "closure") and not a.root:
            ap.error("--root required")
        if a.cmd == "manifest":
            print(json.dumps(build_manifest(a), sort_keys=True))
        elif a.cmd == "digest":
            print(build_manifest(a)["digest"])
        elif a.cmd == "closure":
            if not a.component:
                ap.error("--component required")
            need, inputs, _, _ = closure_of(a)
            print(json.dumps({"repos": need, "inputs": inputs}))
        else:
            if not a.manifest:
                ap.error("--manifest required")
            if not a.root:
                a.root = "."
            {"plan": cmd_plan, "pack": cmd_pack, "tar": cmd_tar}[a.cmd](a)
    except Refused as e:
        sys.stderr.write("REFUSED reason=%s %s\n" % (e.reason, e.detail))
        sys.exit(20)
    except BrokenPipeError:
        sys.exit(0)


if __name__ == "__main__":
    main()
