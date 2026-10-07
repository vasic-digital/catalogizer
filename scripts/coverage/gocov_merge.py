#!/usr/bin/env python3
"""gocov_merge.py - T198. Merges Go cover profiles (mode atomic or count: counts are SUMMED per block) and computes statement coverage.
  gocov_merge.py merge --out FILE profile...            writes the merged profile; prints {"blocks","statements","covered"}; exit 1 if a profile has a different mode
  gocov_merge.py summary --profile FILE [--module M] [--exclusions FILE]   prints JSON: totals and per source package (the directory of each block's file, the module prefix removed);
                                with --exclusions the blocks of every file the fence names are DROPPED from the figure and counted in `excluded` (review I6: a fence the collector
                                does not apply claims a scope the figure does not have)
  gocov_merge.py blocks FILE                            prints the number of blocks of a profile (the header is not a block)
  gocov_merge.py baseline --result FILE --out FILE --app NAME --runs N [--commit SHA] [--exclusions FILE]   writes the coverage-baseline/1 record of docs/05 7.2
A block line is `file:startline.col,endline.col numstmt count`. The statement coverage is covered statements (count > 0) over all statements, the figure `go tool cover` calls total."""
import json, os, sys, collections, datetime, subprocess
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import fence_lib


def read(path):
    mode = None; blocks = collections.OrderedDict()
    with open(path, encoding="utf-8") as fh:
        for n, line in enumerate(fh):
            line = line.rstrip("\n")
            if n == 0:
                if not line.startswith("mode: "):
                    raise ValueError("%s: no `mode:` header" % path)
                mode = line[6:]; continue
            if not line.strip():
                continue
            try:
                key, stmts, count = line.rsplit(" ", 2)
                blocks[key] = (int(stmts), int(count))
            except ValueError:
                raise ValueError("%s: line %d is not a block: %r" % (path, n + 1, line))
    return mode, blocks


def pkg_of(key, module):
    f = key.split(":", 1)[0]
    d = os.path.dirname(f)
    if module and d.startswith(module + "/"):
        d = d[len(module) + 1:]
    elif "/" in d:
        d = d.split("/", 1)[1]
    return d or "root"


def merge(paths):
    mode = None; out = collections.OrderedDict()
    for p in paths:
        m, b = read(p)
        if mode is None: mode = m
        elif m != mode: raise ValueError("%s: mode %s differs from %s" % (p, m, mode))
        for k, (s, c) in b.items():
            if k in out:
                if out[k][0] != s: raise ValueError("block %s has two statement counts" % k)
                out[k] = (s, out[k][1] + c)
            else:
                out[k] = (s, c)
    return mode, out


def main(a):
    if not a:
        sys.exit("usage: gocov_merge.py merge|summary|blocks|baseline ...")
    cmd = a[0]
    if cmd == "merge":
        o = a.index("--out"); out = a[o + 1]; paths = a[2:o] + a[o + 2:]
        try:
            mode, blocks = merge(paths)
        except (ValueError, OSError) as e:
            sys.stderr.write("gocov_merge: %s\n" % e); sys.exit(1)
        with open(out, "w", encoding="utf-8") as fh:
            fh.write("mode: %s\n" % mode)
            for k in sorted(blocks):
                fh.write("%s %d %d\n" % (k, blocks[k][0], blocks[k][1]))
        st = sum(s for s, c in blocks.values()); cv = sum(s for s, c in blocks.values() if c > 0)
        print(json.dumps({"blocks": len(blocks), "statements": st, "covered": cv}))
    elif cmd == "blocks":
        try:
            print(len(read(a[1])[1]))
        except (ValueError, OSError) as e:
            sys.stderr.write("gocov_merge: %s\n" % e); print(0)
    elif cmd == "summary":
        prof = a[a.index("--profile") + 1]; module = a[a.index("--module") + 1] if "--module" in a else None
        mode, blocks = read(prof)
        per = collections.OrderedDict(); st = cv = 0
        excl = []; excluded = {"files": set(), "statements": 0}
        if "--exclusions" in a:
            import yaml
            excl = [e["path"] for e in ((yaml.safe_load(open(a[a.index("--exclusions") + 1], encoding="utf-8")) or {}).get("exclusions") or [])]
        for k, (s, c) in blocks.items():
            if excl:
                f = k.split(":", 1)[0]
                rel = f[len(module) + 1:] if module and f.startswith(module + "/") else f
                if any(fence_lib.matches(e_, rel) for e_ in excl):   # MUT:go_exclusions
                    excluded["files"].add(rel); excluded["statements"] += s
                    continue
            p = pkg_of(k, module)
            d = per.setdefault(p, [0, 0]); d[0] += s; d[1] += s if c > 0 else 0
            st += s; cv += s if c > 0 else 0
        pk = [{"package": p, "statements": v[0], "covered": v[1], "percent": "%.2f" % (100.0 * v[1] / v[0]) if v[0] else "0.00"} for p, v in sorted(per.items())]
        print(json.dumps({"statements": st, "covered": cv, "percent": "%.2f" % (100.0 * cv / st) if st else "0.00", "packages": pk,
                          "excluded": {"files": sorted(excluded["files"]), "statements": excluded["statements"]}}))
    elif cmd == "baseline":
        g = lambda k, d=None: a[a.index(k) + 1] if k in a else d
        res = json.load(open(g("--result")))
        commit = g("--commit") or ""
        if not commit:
            try: commit = subprocess.run(["git", "rev-parse", "HEAD"], capture_output=True, text=True).stdout.strip()
            except OSError: commit = ""
        rec = {"schema": "coverage-baseline/1", "application": g("--app"), "instrument": "go-cover-atomic-split",
               "measured_at": datetime.datetime.now(datetime.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"), "source_commit": commit,
               "scope": {"include": ["./..."], "exclusion_list": g("--exclusions")}, "runs": int(g("--runs", "1")),
               "line_percent": res["percent"], "branch_percent": None, "statements": res["statements"], "covered": res["covered"],
               "per_package": {p["package"]: p["percent"] for p in res["packages"]}, "excluded": res.get("excluded"), "evidence": [os.path.basename(g("--result"))]}
        open(g("--out"), "w").write(json.dumps(rec, indent=2, sort_keys=True) + "\n")
    else:
        sys.exit("gocov_merge.py: unknown command %s" % cmd)


if __name__ == "__main__":
    main(sys.argv[1:])
