#!/usr/bin/env python3
"""snapshot_manifest.py <dir> - print the manifest of a directory tree (WP-20, T161 manifest rule).

STAND-IN NOTICE (UNCONFIRMED vs T161): T161 (freeze) owns this file; it is written here by the WP-20 test
worker so T162/T164/T165/T167 can run. The rule follows tasks.md T161: one entry per regular file and per
symlink (directories are not entries); a regular file is hashed over its bytes, a symlink over the sha256 of
its readlink target string (no trailing newline), never followed; path = bytes decoded as UTF-8 (a path that
is not valid UTF-8 -> exit 20 freeze_path_unsafe); entries sorted by path (byte order).
Output (stdout): {"schema":"snapshot-manifest/1","entries":[{"path":..,"sha256":..},..]}
A path that is not valid UTF-8 -> freeze_path_unsafe; a file that is neither regular nor a symlink (FIFO, socket, device) -> freeze_special_file.
Exit: 0 ok, 2 usage, 20 refusal.
"""
import hashlib, json, os, stat, sys


def file_hash(p):
    h = hashlib.sha256()
    with open(p, "rb") as f:
        for chunk in iter(lambda: f.read(1 << 20), b""):
            h.update(chunk)
    return h.hexdigest()


def manifest(root):
    root_b = os.fsencode(root)
    out = []
    for dp, dns, fns in os.walk(root_b, followlinks=False):
        # a symlink to a directory shows up in dns and is not descended into by os.walk(followlinks=False)
        for n in list(dns) + list(fns):
            full = os.path.join(dp, n)
            if os.path.islink(full):
                rel = os.path.relpath(full, root_b)
                tgt = os.readlink(full)
                out.append((rel, hashlib.sha256(tgt).hexdigest()))
            elif n in fns:
                rel = os.path.relpath(full, root_b)
                if not stat.S_ISREG(os.lstat(full).st_mode):   # a FIFO would block the read forever; a socket or device is no source file (round-23 review F4)
                    sys.stderr.write("snapshot_manifest: REFUSED reason=freeze_special_file %r\n" % rel)
                    sys.exit(20)
                out.append((rel, file_hash(full)))
    out.sort(key=lambda t: t[0])
    ents = []
    for rel, h in out:
        try:
            ents.append({"path": rel.decode("utf-8"), "sha256": h})
        except UnicodeDecodeError:
            sys.stderr.write("snapshot_manifest: REFUSED reason=freeze_path_unsafe %r\n" % rel)
            sys.exit(20)
    return {"schema": "snapshot-manifest/1", "entries": ents}


if __name__ == "__main__":
    if len(sys.argv) != 2 or not os.path.isdir(sys.argv[1]):
        sys.stderr.write("usage: snapshot_manifest.py <dir>\n")
        sys.exit(2)
    json.dump(manifest(sys.argv[1]), sys.stdout, sort_keys=True, ensure_ascii=False)
    sys.stdout.write("\n")
