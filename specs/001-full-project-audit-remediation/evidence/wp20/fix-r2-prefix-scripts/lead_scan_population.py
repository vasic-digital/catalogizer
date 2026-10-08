#!/usr/bin/env python3
"""lead_scan_population.py - the one helper that computes the lead-scan population (WP-20, tasks T167, doc03 section 10 step 4).

Population = the entries of the frozen listing (key `listing` of the freeze json, raw path bytes each followed by one NUL) whose path
ends in `.md` and lies OUTSIDE every `gitlinks` path of the freeze json (a path equal to a gitlink path, or beginning with it followed
by `/`, is inside). The one widening is the owner's recorded answer: `lead_scan_extra_roots` of the HC-2 record (a list of gitlink
paths whose Markdown comes into scope; absent file or field = []). An extra root that is not a gitlinks path is refused
`lead_scan_extra_root_invalid` naming it. No option widens the population otherwise.
CLI: lead_scan_population.py --freeze-json F [--hc2 PATH] [--count]   prints the NUL-separated paths (or the count) on stdout.
"""
import json, sys


class PopulationRefusal(Exception):
    def __init__(self, reason, detail=""):
        Exception.__init__(self, reason); self.reason, self.detail = reason, detail


def gitlink_paths(fj):
    out = []
    for g in fj.get("gitlinks") or []:
        out.append(g["path"] if isinstance(g, dict) else str(g))
    return out


def inside(path, root):
    return path == root or path.startswith(root + "/")


def extra_roots(fj, hc2_path):
    if not hc2_path:
        return []
    try:
        rec = json.load(open(hc2_path, encoding="utf-8"))
    except (OSError, ValueError):
        return []
    roots = rec.get("lead_scan_extra_roots") or []
    gl = set(gitlink_paths(fj))
    for r in roots:
        if r not in gl:
            raise PopulationRefusal("lead_scan_extra_root_invalid", r)
    return list(roots)


def population(fj, hc2_path=None):
    """-> (sorted list of snapshot-relative .md paths, extra roots applied)"""
    lp = fj.get("listing")
    if not lp:
        raise PopulationRefusal("freeze_listing_missing", "freeze json has no `listing`")
    raw = [p for p in open(lp, "rb").read().split(b"\0") if p]
    gls, extra = gitlink_paths(fj), extra_roots(fj, hc2_path)
    out = []
    for b in raw:
        try:
            p = b.decode("utf-8")
        except UnicodeDecodeError:
            raise PopulationRefusal("path_not_utf8", repr(b))
        if not p.endswith(".md"):
            continue
        if any(inside(p, g) for g in gls) and not any(inside(p, e) for e in extra):   # MUT:population
            continue
        out.append(p)
    return sorted(set(out)), extra


if __name__ == "__main__":
    a = sys.argv[1:]
    if "--freeze-json" not in a:
        sys.stderr.write("usage: lead_scan_population.py --freeze-json F [--hc2 PATH] [--count]\n"); sys.exit(2)
    fj = json.load(open(a[a.index("--freeze-json") + 1], encoding="utf-8"))
    try:
        files, extra = population(fj, a[a.index("--hc2") + 1] if "--hc2" in a else None)
    except PopulationRefusal as e:
        sys.stderr.write("lead_scan_population: REFUSED reason=%s %s\n" % (e.reason, e.detail)); sys.exit(20)
    if "--count" in a:
        print(len(files))
    else:
        sys.stdout.buffer.write(b"".join(f.encode("utf-8") + b"\0" for f in files))
