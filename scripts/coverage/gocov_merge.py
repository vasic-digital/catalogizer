#!/usr/bin/env python3
"""gocov_merge.py - T198. Merges Go cover profiles (mode atomic or count: counts are SUMMED per block) and computes statement coverage.
  gocov_merge.py merge --out FILE profile...            writes the merged profile (the profiles may come before or after --out); prints {"blocks","statements","covered"}; exit 1 if a
                                                        profile has a different mode or a block has two statement counts
  gocov_merge.py summary --profile FILE [--go-mod FILE | --module M] [--exclusions FILE] [--root DIR]
                                prints JSON: totals and per source package (the directory of each block's file, the module prefix removed). With --exclusions the blocks of every
                                file the fence names are DROPPED from the figure and counted in `excluded` (review I6). The module is read from --go-mod with a real parser
                                (quotes, a trailing comment and CRLF are handled); with --exclusions and NO module the fence cannot be applied and the run is REFUSED (exit 3
                                `module_unknown`): the earlier code applied nothing and printed a figure that claimed a scope it did not have. A missing fence file is `fence_missing`
                                (exit 3), an entry without a `path` is `fence_malformed` (exit 3); the collector passes --exclusions only when the application HAS a fence, and says so.
                                --root lists the tracked files of the application root and records the fence entries that name Go files but no block of the profile
                                (`fence_entries_without_blocks`): the entry excludes nothing from this figure.
  gocov_merge.py blocks FILE                            prints the number of blocks of a profile (the header is not a block)
  gocov_merge.py baseline --result FILE --out FILE --app NAME --runs N --commit SHA --snapshot DIGEST --nonce N [--exclusions FILE]
                                writes the coverage-baseline/1 record of docs/05 7.2. The record names the commit and the snapshot that were MEASURED (an empty commit AND an empty
                                snapshot is refused), the sha256 of the fence that was applied (or `none`), and it verifies that --result carries the run nonce, so a result.json left
                                over from another run is never turned into a baseline (WP-23 round 5 K8.4, K12.4)
A block line is `file:startline.col,endline.col numstmt count`. The statement coverage is covered statements (count > 0) over all statements, the figure `go tool cover` calls total.
Exit: 0 ok; 1 a profile cannot be merged; 2 usage; 3 a refusal (reason on stderr)."""
import json, os, re, sys, collections, datetime, hashlib
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import fence_lib


def refuse(reason, detail="", code=3):
    sys.stderr.write("gocov_merge: REFUSED reason=%s %s\n" % (reason, detail)); sys.exit(code)


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


def read_module(gomod):
    """The module path of a go.mod: `module x`, `module "x"`, `module x // comment`, CRLF. None when there is no module line."""
    try:
        text = open(gomod, encoding="utf-8").read()
    except OSError:
        return None
    for line in text.splitlines():
        m = re.match(r'^\s*module\s+("([^"]+)"|`([^`]+)`|([^\s/][^\s]*))\s*(?://.*)?$', line.rstrip("\r"))
        if m:
            return (m.group(2) or m.group(3) or m.group(4)).strip()
    return None


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
        elif m != mode: raise ValueError("%s: mode %s differs from %s" % (p, m, mode))   # MUT:merge_mode
        for k, (s, c) in b.items():
            if k in out:
                if out[k][0] != s: raise ValueError("block %s has two statement counts" % k)   # MUT:merge_stmts
                out[k] = (s, out[k][1] + c)
            else:
                out[k] = (s, c)
    return mode, out


def opt(a, name, default=None):
    if name in a:
        i = a.index(name)
        if i + 1 >= len(a):
            sys.stderr.write("gocov_merge: usage: %s needs a value\n" % name); sys.exit(2)
        return a[i + 1]
    return default


def main(a):
    if not a:
        sys.exit("usage: gocov_merge.py merge|summary|blocks|baseline ...")
    cmd = a[0]
    if cmd == "merge":
        if "--out" not in a:
            sys.stderr.write("gocov_merge: usage: merge needs --out FILE\n"); sys.exit(2)
        o = a.index("--out")
        if o + 1 >= len(a):
            sys.stderr.write("gocov_merge: usage: --out needs a value\n"); sys.exit(2)
        out = a[o + 1]; paths = a[1:o] + a[o + 2:]   # MUT:merge_argv  (a profile may come before --out)
        if not paths:
            sys.stderr.write("gocov_merge: usage: merge needs at least one profile\n"); sys.exit(2)
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
        except (ValueError, OSError, IndexError) as e:
            sys.stderr.write("gocov_merge: %s\n" % e); print(0)
    elif cmd == "summary":
        prof = opt(a, "--profile")
        if not prof:
            sys.stderr.write("gocov_merge: usage: summary needs --profile FILE\n"); sys.exit(2)
        module = opt(a, "--module"); gomod = opt(a, "--go-mod")
        if gomod and not module:
            module = read_module(gomod)
        try:
            mode, blocks = read(prof)
        except (ValueError, OSError) as e:
            sys.stderr.write("gocov_merge: %s\n" % e); sys.exit(1)
        per = collections.OrderedDict(); st = cv = 0
        excl = []; excluded = {"files": set(), "statements": 0}; fence_info = "none"
        if "--exclusions" in a:
            exf = opt(a, "--exclusions")
            if not module:   # MUT:module_unknown
                refuse("module_unknown", "--exclusions needs the Go module path (--go-mod FILE with a `module` line, or --module M): without it no block can be matched to a fence entry")
            try:
                raw = open(exf, "rb").read()
            except OSError as e:
                refuse("fence_missing", "the fence file %s cannot be read: %s (pass --exclusions only when the application has a fence)" % (exf, e))   # MUT:fence_missing
            try:
                doc = fence_lib.load_yaml_strict(raw.decode("utf-8")) or {}
            except Exception as e:
                refuse("fence_malformed", "%s: %s" % (exf, e))
            for i, e_ in enumerate((doc.get("exclusions") if isinstance(doc, dict) else None) or []):
                if not isinstance(e_, dict) or not isinstance(e_.get("path"), str):
                    refuse("fence_malformed", "entry %d of %s has no `path`" % (i + 1, exf))   # MUT:fence_malformed
                excl.append(e_["path"])
            fence_info = {"sha256": hashlib.sha256(raw).hexdigest(), "entries": len(excl)}
        hit = {e_: 0 for e_ in excl}
        for k, (s, c) in blocks.items():
            if excl:
                f = k.split(":", 1)[0]
                rel = f[len(module) + 1:] if module and f.startswith(module + "/") else f
                m_ = [e_ for e_ in excl if fence_lib.matches(e_, rel)]
                if m_:   # MUT:go_exclusions
                    for e_ in m_:
                        hit[e_] += 1
                    excluded["files"].add(rel); excluded["statements"] += s
                    continue
            p = pkg_of(k, module)
            d = per.setdefault(p, [0, 0]); d[0] += s; d[1] += s if c > 0 else 0
            st += s; cv += s if c > 0 else 0
        no_blocks = []
        root = opt(a, "--root")
        if root and excl:
            files, _info = fence_lib.list_files(root)
            for e_ in excl:
                names = [r for r in files if r.endswith(".go") and not r.endswith("_test.go") and fence_lib.matches(e_, r)]
                if names and hit[e_] == 0:
                    no_blocks.append({"path": e_, "tracked_go_files": len(names)})
        pk = [{"package": p, "statements": v[0], "covered": v[1], "percent": "%.2f" % (100.0 * v[1] / v[0]) if v[0] else "0.00"} for p, v in sorted(per.items())]
        print(json.dumps({"statements": st, "covered": cv, "percent": "%.2f" % (100.0 * cv / st) if st else "0.00", "packages": pk, "module": module, "fence": fence_info,
                          "fence_entries_without_blocks": no_blocks,
                          "excluded": {"files": sorted(excluded["files"]), "statements": excluded["statements"]}}))
    elif cmd == "baseline":
        for k in ("--result", "--out", "--app", "--nonce"):
            if opt(a, k) is None:
                sys.stderr.write("gocov_merge: usage: baseline needs %s\n" % k); sys.exit(2)
        try:
            res = json.load(open(opt(a, "--result")))
        except (OSError, ValueError) as e:
            refuse("result_unreadable", "%s" % e)
        if res.get("nonce") != opt(a, "--nonce"):   # MUT:nonce
            refuse("result_not_this_run", "result.json carries nonce %r, this run is %r: a result left over from another run is never turned into a baseline" % (res.get("nonce"), opt(a, "--nonce")))
        if res.get("status") != "ok":
            refuse("result_not_ok", "result.json says %r" % res.get("status"))
        commit = opt(a, "--commit", "") or ""; snap = opt(a, "--snapshot", "") or ""
        if not commit and not snap:   # MUT:provenance
            refuse("provenance_missing", "neither --commit nor --snapshot: the baseline would not name what was measured")
        fence = res.get("fence", "none")
        rec = {"schema": "coverage-baseline/1", "application": opt(a, "--app"), "instrument": "go-cover-atomic-split",
               "measured_at": datetime.datetime.now(datetime.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"), "source_commit": commit or None, "source_snapshot": snap or None,
               "scope": {"include": ["./..."], "exclusion_list": fence if fence != "none" else "none"}, "runs": int(opt(a, "--runs", "1")),
               "line_percent": res["percent"], "branch_percent": None, "statements": res["statements"], "covered": res["covered"],
               "per_package": {p["package"]: p["percent"] for p in res["packages"]}, "excluded": res.get("excluded"), "evidence": [os.path.basename(opt(a, "--result"))],
               "uncompiled_build_tags": res.get("uncompiled_build_tags"), "packages_without_tests": res.get("packages_without_tests"),
               "fence_entries_without_blocks": res.get("fence_entries_without_blocks")}
        out = opt(a, "--out"); tmp = out + ".tmp.%d" % os.getpid()
        try:
            with open(tmp, "w") as fh:
                fh.write(json.dumps(rec, indent=2, sort_keys=True) + "\n")
            os.replace(tmp, out)
        finally:
            if os.path.exists(tmp):
                os.unlink(tmp)
    else:
        sys.stderr.write("gocov_merge.py: unknown command %s\n" % cmd); sys.exit(2)


if __name__ == "__main__":
    try:
        main(sys.argv[1:])
    except SystemExit:
        raise
    except Exception as e:   # MUT:toplevel_handler
        sys.stderr.write("gocov_merge: input could not be processed: %s: %s\n" % (type(e).__name__, e)); sys.exit(3)
