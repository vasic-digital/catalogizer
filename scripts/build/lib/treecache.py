#!/usr/bin/env python3
"""treecache.py - T005b: the build-host side of the source snapshot (runs on the build host; needs only python3, no git).
A content-addressed object cache `<cache>/objs/<id[:2]>/<id>` (raw git blob and tree objects, each verified against its id by sha1 of
"<type> <len>\\0" + bytes, so a tree altered in the cache or in transit is refused), and the materialisation of a snapshot manifest
(snapshot.py manifest) into a working directory, whose every repository tree is RECOMPUTED from the files on disk, with the gitlink
entries (a gitlink cannot be recomputed from shipped files) taken from the manifest.

  treecache.py missing <cache> < ids          the "b <id>" / "t <id>" lines whose object the cache lacks
  treecache.py store <cache> < tar            store the tar members b/<id>, t/<id>; refuses an object whose bytes do not hash to its id
  treecache.py materialize <cache> <manifest> --repos A,B --dest DIR     write the shipped trees and recompute them (atomic: DIR appears whole or not at all)
  treecache.py verify <dir> <manifest> --repos A,B                        recompute the trees of an existing directory (a cached tree is re-verified, never trusted)
Exit 20 `REFUSED reason=snapshot_mismatch ...` on any mismatch; 2 usage."""
import hashlib, json, os, shutil, stat, sys, tarfile, tempfile

HEX40 = set("0123456789abcdef")


class Mismatch(Exception):
    pass


def refuse(reason, detail=""):
    sys.stderr.write("REFUSED reason=%s %s\n" % (reason, detail))
    sys.exit(20)


def oid(kind, data):
    return hashlib.sha1(b"%s %d\0" % (kind.encode(), len(data)) + data).hexdigest()


def valid_id(s):
    return len(s) == 40 and set(s) <= HEX40


def obj_path(cache, i):
    return os.path.join(cache, "objs", i[:2], i)


def cmd_missing(cache):
    for line in sys.stdin.read().splitlines():
        line = line.strip()
        if not line:
            continue
        k, i = line.split()
        if k in ("b", "t") and valid_id(i) and not os.path.exists(obj_path(cache, i)):
            print(line)


def fsync_write(path, data):
    os.makedirs(os.path.dirname(path), exist_ok=True)
    t = "%s.tmp-%d" % (path, os.getpid())
    with open(t, "wb") as f:
        f.write(data)
        f.flush()
        os.fsync(f.fileno())
    os.replace(t, path)


def cmd_store(cache):
    n = 0
    tf = tarfile.open(fileobj=sys.stdin.buffer, mode="r|")
    for ti in tf:
        if not ti.isreg():
            refuse("snapshot_mismatch", "non-regular member %s" % ti.name)
        kind, _, i = ti.name.partition("/")
        if kind not in ("b", "t") or not valid_id(i):
            refuse("snapshot_mismatch", "malformed member name %r" % ti.name)
        data = tf.extractfile(ti).read()
        if oid("blob" if kind == "b" else "tree", data) != i:
            refuse("snapshot_mismatch", "object %s: its bytes do not hash to its id" % i)
        fsync_write(obj_path(cache, i), data)
        n += 1
    print("stored %d" % n)


def read_obj(cache, kind, i):
    try:
        data = open(obj_path(cache, i), "rb").read()
    except OSError:
        raise Mismatch("object %s is absent from the cache" % i)
    if oid(kind, data) != i:
        raise Mismatch("object %s in the cache does not hash to its id (altered)" % i)
    return data


def parse_tree(data):
    pos, ents = 0, []
    while pos < len(data):
        sp = data.index(b" ", pos)
        nul = data.index(b"\0", sp)
        mode, name = data[pos:sp].decode(), data[sp + 1:nul]
        ents.append((mode, name, data[nul + 1:nul + 21].hex()))
        pos = nul + 21
    return ents


def materialize_tree(cache, tree, dest):
    os.makedirs(dest, exist_ok=True)
    for mode, name, i in parse_tree(read_obj(cache, "tree", tree)):
        p = os.path.join(dest, os.fsdecode(name))
        if mode == "40000":
            materialize_tree(cache, i, p)
        elif mode == "160000":
            continue
        elif mode == "120000":
            os.symlink(read_obj(cache, "blob", i), p)
        else:
            with open(p, "wb") as f:
                f.write(read_obj(cache, "blob", i))
            os.chmod(p, 0o755 if mode == "100755" else 0o644)


def tree_from_disk(base, gitlinks):
    """Recompute the tree id of the repository files under base. gitlinks: {relpath: commit} from the manifest."""
    root = {}

    def put(rel, leaf):
        parts = rel.split("/")
        d = root
        for p in parts[:-1]:
            d = d.setdefault(p, {})
        d[parts[-1]] = leaf
    skip = set(gitlinks)

    def walk(d, rel):
        for name in sorted(os.listdir(d)):
            r = rel + name
            if r in skip or any(r.startswith(g + "/") for g in skip):
                continue
            fp = os.path.join(d, name)
            st = os.lstat(fp)
            if stat.S_ISLNK(st.st_mode):
                put(r, ("120000", oid("blob", os.fsencode(os.readlink(fp)))))
            elif stat.S_ISDIR(st.st_mode):
                walk(fp, r + "/")
            elif stat.S_ISREG(st.st_mode):
                put(r, ("100755" if st.st_mode & 0o100 else "100644", oid("blob", open(fp, "rb").read())))
    if os.path.isdir(base):
        walk(base, "")
    for g, c in gitlinks.items():
        put(g, ("160000", c))

    def fold(node):
        ents = []
        for name, v in node.items():
            if isinstance(v, dict):
                ents.append((os.fsencode(name) + b"/", b"40000", os.fsencode(name), fold(v)))
            else:
                ents.append((os.fsencode(name), v[0].encode(), os.fsencode(name), v[1]))
        ents.sort(key=lambda e: e[0])
        body = b"".join(m + b" " + n + b"\0" + bytes.fromhex(i) for _k, m, n, i in ents)
        return oid("tree", body)
    return fold(root)


def repo_dir(dest, path):
    return dest if path == "." else os.path.join(dest, path)


def recompute(dest, m, names):
    byp = {r["path"]: r for r in m["repos"]}
    for n in names:
        r = byp.get(n)
        if r is None:
            raise Mismatch("repository %s is not in the manifest" % n)
        got = tree_from_disk(repo_dir(dest, n), r.get("gitlinks", {}))
        if got != r["tree"]:
            raise Mismatch("tree of %s recomputed as %s, the manifest says %s" % (n, got, r["tree"]))


def cmd_materialize(cache, manifest, names, dest):
    m = json.load(open(manifest))
    byp = {r["path"]: r for r in m["repos"]}
    if os.path.isdir(dest):
        try:
            recompute(dest, m, names)
        except Mismatch as e:
            refuse("snapshot_mismatch", str(e))
        print("cached")
        return
    tmp = tempfile.mkdtemp(prefix=os.path.basename(dest) + ".tmp-", dir=os.path.dirname(os.path.abspath(dest)))
    try:
        for n in names:
            if n not in byp:
                raise Mismatch("repository %s is not in the manifest" % n)
            materialize_tree(cache, byp[n]["tree"], repo_dir(tmp, n))
        recompute(tmp, m, names)
        os.rename(tmp, dest)
    except Mismatch as e:
        shutil.rmtree(tmp, ignore_errors=True)
        refuse("snapshot_mismatch", str(e))
    except BaseException:
        shutil.rmtree(tmp, ignore_errors=True)
        raise
    print("materialized")


def cmd_verify(dest, manifest, names):
    try:
        recompute(dest, json.load(open(manifest)), names)
    except Mismatch as e:
        refuse("snapshot_mismatch", str(e))
    print("verified")


def main():
    a = sys.argv[1:]
    if not a:
        sys.exit(2)
    c = a[0]

    def opt(name, default=None):
        return a[a.index(name) + 1] if name in a else default
    names = [x for x in (opt("--repos", ".")).split(",") if x]
    if c == "missing" and len(a) == 2:
        cmd_missing(a[1])
    elif c == "store" and len(a) == 2:
        cmd_store(a[1])
    elif c == "materialize" and len(a) >= 3 and opt("--dest"):
        cmd_materialize(a[1], a[2], names, opt("--dest"))
    elif c == "verify" and len(a) >= 3:
        cmd_verify(a[1], a[2], names)
    else:
        sys.stderr.write(__doc__ + "\n")
        sys.exit(2)


if __name__ == "__main__":
    main()
