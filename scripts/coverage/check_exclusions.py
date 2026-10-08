#!/usr/bin/env python3
"""check_exclusions.py - T200. See check_exclusions.sh. Rules (docs/05 7.3, 11.4.224 E). The gate PASSes only when EVERY rule ran: `content_checked` (the files of the root were
enumerated and the entries judged against them) and `applied_checked` (the measuring tool's own configuration was read and compared with the fence) are both true. A missing
input - no --root, no tools record, an unreadable config - is a named refusal, never a skipped rule plus PASS (WP-23 round 5, K2).
  1. schema coverage-exclusions/1; `application` equals the file stem (and --application when given); an `exclusions` list is present (empty is legal); duplicate YAML keys are refused;
  2. each entry has a relative, non-over-broad `path` (a glob, no absolute path, no `..`; over-broad is a SEMANTIC test: the pattern names every path of a synthetic set), listed once;
  3. `class` is one of generated-code | vendored-third-party | non-shipping-fixtures-and-golden-assets | first-party;
  4. a non-empty justification (at least MIN_JUST characters);
  5. a first-party entry names a tracked item that exists in --items-file and/or `measured_by: {app, lane}` naming ANOTHER lane than the one the figure comes from, with a lanes.tsv
     row, an existing wrapper and every image of that wrapper in the image lock; EVERY route field that is present is verified;
  6. every pattern the measuring tool applies is listed, and every fence entry the tool does not apply is flagged. The tool's patterns come from its OWN config (tools.yaml names it and
     the script that runs it); the config is parsed fail-closed (fence_lib.extract_applied); the declared tool kind is checked against the package.json script that runs it;
  7. with --root, each non-first-party entry's matched TRACKED files are consistent with the class it claims (anchored generated header or generator-output directory; vendored
     provenance; test source set plus an import search);
  8. each entry alone and all entries together must not name more than half of the root's MEASURABLE files (the files the tool can instrument);
  9. for a `kind: fence` application whose tools row declares a `scope`, every first-party file in scope is either measured (the --measured list, or the row's `measured` globs) or
     named by a fence entry: an unmeasured first-party file is `unmeasured_first_party`;
 10. the root must exist, enumerate at least one TRACKED file and be the root the tools row binds to the application.
The checks are collected, not short-circuited, so one run names every problem. Exit: 0 PASS (or SCHEMA_ONLY with --schema-only); 1 FAIL; 2 usage; 3 the input could not be processed."""
import hashlib, json, os, re, subprocess, sys
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import fence_lib
from fence_lib import FenceError

CLASSES = ("generated-code", "vendored-third-party", "non-shipping-fixtures-and-golden-assets", "first-party")
MIN_JUST = 12
ITEM_RE = re.compile(r"^[A-Z][A-Z0-9]{1,7}(?:-[A-Z0-9]+)*-[0-9]+$")
BROAD = {"*", "**", "**/*", ".", "./", "./**", "*/**", "**/**", "?*/**", "*/*", "**/*.*", "*.*", "**/.*"}   # the fast first check; the semantic test below is the real one
SYNTH_ALL = ["a", "a.b", "a/b", "a/b.c", "a/b/c", "a/b/c.d", "a/b/c/d/e", "a/b/c/d/e.f", ".x", "a/.x"]
SYNTH_NESTED = ["a/b", "a/b.c", "a/b/c", "a/b/c.d", "a/b/c/d/e", "a/b/c/d/e.f", "a/.x", "ab/cd/ef/gh"]
USAGE = "check_exclusions: usage: check_exclusions.sh FILE --root DIR [--repo DIR] [--application NAME] [--tools FILE] [--lanes FILE] [--items-file FILE] [--used FILE] [--measured FILE] [--json OUT] [--schema-only]\n"


def norm(p):
    """The comparison form of a pattern: `vendor/`, `vendor/**` and `vendor` name the same directory."""
    p = p.strip()
    while p.endswith("/"):
        p = p[:-1]
    if p.endswith("/**"):
        p = p[:-3]
    return p


def overbroad(p):
    if p.strip() in BROAD:   # MUT:broad_literal
        return True
    try:
        return all(fence_lib.matches(p, s) for s in SYNTH_ALL) or all(fence_lib.matches(p, s) for s in SYNTH_NESTED)   # MUT:broad_semantic
    except FenceError:
        return False


def sha(path):
    try:
        return hashlib.sha256(open(path, "rb").read()).hexdigest()
    except OSError:
        return None


def parse_args(argv):
    a = {"flags": set()}; path = None
    valued = {"--application": "app", "--used": "used", "--root": "root", "--json": "out", "--repo": "repo", "--items-file": "items", "--tools": "tools", "--lanes": "lanes",
              "--measured": "measured", "--images-lock": "images_lock"}
    i = 0
    while i < len(argv):
        k = argv[i]
        if k in valued:
            if i + 1 >= len(argv):
                sys.stderr.write("check_exclusions: usage: option %s needs a value\n" % k + USAGE); sys.exit(2)
            a[valued[k]] = argv[i + 1]; i += 2
        elif k == "--schema-only":
            a["flags"].add("schema_only"); i += 1
        elif not k.startswith("--") and path is None:
            path = k; i += 1
        else:
            sys.stderr.write(USAGE); sys.exit(2)
    if path is None:
        sys.stderr.write(USAGE); sys.exit(2)
    a["path"] = path
    return a


def wrapper_images(lanes_dir, wrapper):
    p = os.path.join(lanes_dir, wrapper + ".sh")
    if not os.path.isfile(p):
        return None
    m = re.search(r'(?m)^RUNNER_IMAGES="([^"]*)"', open(p, encoding="utf-8", errors="replace").read())
    return m.group(1).split() if m else []


def main(argv):
    a = parse_args(argv)
    path = a["path"]; schema_only = "schema_only" in a["flags"]
    root = a.get("root")
    if root is None and not schema_only:
        sys.stderr.write("check_exclusions: usage: root_required: --root DIR is required (rule 6 and the content rules cannot run without it); pass --schema-only to check the schema alone\n" + USAGE); sys.exit(2)
    problems = []
    doc = None; digest = ""
    try:
        raw = open(path, "rb").read(); digest = hashlib.sha256(raw).hexdigest()
        doc = fence_lib.load_yaml_strict(raw.decode("utf-8"))
    except Exception as e:
        problems.append("cannot read %s: %s" % (path, e))
    app = a.get("app")
    stem = os.path.splitext(os.path.basename(path))[0]
    entries_out = []; first_party = []
    exclusions = []
    if doc is not None:
        if not isinstance(doc, dict) or doc.get("schema") != "coverage-exclusions/1":
            problems.append("schema must be coverage-exclusions/1")
        if not isinstance(doc, dict) or doc.get("application") != (app or stem):
            problems.append("application %r must equal %r (the file stem or --application)" % (doc.get("application") if isinstance(doc, dict) else None, app or stem))
        exclusions = doc.get("exclusions") if isinstance(doc, dict) else None
        if not isinstance(exclusions, list):
            problems.append("the `exclusions` list is missing (write `exclusions: []` to state there are none)")
            exclusions = []
    application = app or (doc.get("application") if isinstance(doc, dict) and isinstance(doc.get("application"), str) else stem)
    # ---------------- inputs: root, repo, tools row, lanes, items ----------------
    files = None; finfo = {}; root_abs = None
    repo = a.get("repo")
    if not schema_only:
        root_abs = os.path.abspath(root)
        if not os.path.isdir(root_abs):
            problems.append("root_missing: --root %s is not a directory" % root)
        else:
            try:
                files, finfo = fence_lib.list_files(root_abs)
                if not files:
                    problems.append("root_empty: %s enumerates no tracked file: a gate that judges nothing cannot pass (a misspelled or empty root)" % root)
            except FenceError as e:
                problems.append(str(e))
        if repo is None:
            r_ = subprocess.run(["git", "-C", root_abs if os.path.isdir(root_abs) else ".", "rev-parse", "--show-toplevel"], stdout=subprocess.PIPE, stderr=subprocess.PIPE, env=fence_lib.clean_git_env())
            if r_.returncode == 0:
                repo = r_.stdout.decode().strip()
        if repo is None:
            problems.append("repo_unresolved: --repo was not given and the root is not inside a git work tree (rule 6 needs the repository to find the tools record and the tool config)")
    known_items = None; items = a.get("items")
    if items:
        try:
            known_items = set(l.strip() for l in open(items, encoding="utf-8") if l.strip() and not l.startswith("#"))
        except OSError as e:
            problems.append("cannot read --items-file %s: %s" % (items, e))
    lanes_path = a.get("lanes") or (os.path.join(repo, "scripts", "containers", "lanes.tsv") if repo else None)
    lane_rows = None
    if lanes_path and os.path.isfile(lanes_path):
        lane_rows = {}
        for line in open(lanes_path, encoding="utf-8"):
            cols = line.rstrip("\n").split("\t")
            if len(cols) >= 3 and not line.startswith("#"):
                lane_rows[(cols[0], cols[1])] = cols[2]
            elif len(cols) >= 2 and not line.startswith("#"):
                lane_rows[(cols[0], cols[1])] = None
    images_lock = a.get("images_lock") or (os.path.join(repo, "build", "containers", "images.lock.yaml") if repo else None)
    lock_ids = None
    if images_lock and os.path.isfile(images_lock):
        lock_ids = set(re.findall(r"(?m)^- id: (\S+)", open(images_lock, encoding="utf-8").read()))
    row = None; tools_path = None; tools_doc = None
    if not schema_only and repo:
        tools_path = a.get("tools") or os.path.join(repo, "coverage", "exclusions", "tools.yaml")
        if not os.path.isfile(tools_path):
            problems.append("tool_record_missing: the tools file %s does not exist (rule 6 needs the record of the measuring tool; it is not skipped)" % tools_path)
        else:
            try:
                tools_doc = fence_lib.load_yaml_strict(open(tools_path, encoding="utf-8").read())
            except Exception as e:
                problems.append("tool_record_malformed: cannot read the tools file %s: %s" % (tools_path, e))
        if tools_doc is not None:
            apps_ = tools_doc.get("apps") if isinstance(tools_doc, dict) else None
            row = apps_.get(application) if isinstance(apps_, dict) else None
            if row is None:
                problems.append("tool_record_missing: %s has no row for application %r (write `kind: none` with a note when no coverage tool measures it)" % (tools_path, application))
            elif not isinstance(row, dict) or not isinstance(row.get("kind"), str):
                problems.append("tool_record_malformed: the tools row of %r is not a mapping with a `kind` (%r)" % (application, row)); row = None
    kind = row.get("kind") if row else None
    measurable = None
    if kind in fence_lib.MEASURABLE:
        measurable = fence_lib.MEASURABLE[kind]
    elif row and isinstance(row.get("measurable"), list):
        measurable = tuple(str(x) for x in row["measurable"])
    is_meas = (lambda r: not fence_lib.is_test_source(r)) if measurable is None else (lambda r: r.endswith(measurable) and not fence_lib.is_test_source(r))
    # the root is the root the tools row binds to the application
    if row and root_abs and repo:
        rr = row.get("root")
        if not isinstance(rr, str):
            problems.append("tool_record_malformed: the tools row of %r has no `root` (the application root the gate is bound to)" % application)
        elif os.path.realpath(os.path.join(repo, rr)) != os.path.realpath(root_abs):
            problems.append("root_not_application: --root %s is not the root %s of application %s in the tools record: another application's root would judge a different tree" % (root, rr, application))
    # tool identity: the declared kind must be the tool the measuring script runs
    if row and kind in ("vitest", "jest") and root_abs:
        scripts = fence_lib.package_scripts(root_abs)
        sname = row.get("script", "test")
        who = fence_lib.tool_in_script(scripts.get(sname))
        if scripts.get(sname) is None:
            problems.append("tool_identity_unverifiable: package.json of %s has no script %r to derive the measuring tool from" % (application, sname))
        elif who == "both":
            problems.append("tool_identity_ambiguous: package.json script %r runs both jest and vitest" % sname)
        elif who != kind:
            problems.append("tool_identity_mismatch: the tools record declares %s but package.json script %r (%s) runs %s" % (kind, sname, scripts[sname], who or "neither jest nor vitest"))   # MUT:tool_identity
        elif row.get("script") is None and os.path.isfile(os.path.join(root_abs, "jest.config.js")) and os.path.isfile(os.path.join(root_abs, "vitest.config.ts")):
            problems.append("tool_identity_ambiguous: both jest.config.js and vitest.config.ts exist and the tools row names no `script`")
    # ---------------- entries ----------------
    seen = set(); listed = []
    ctx = None
    if files is not None and root_abs:
        gm = []
        gmp = os.path.join(repo or root_abs, ".gitmodules")
        if os.path.isfile(gmp):
            gm = [os.path.relpath(os.path.join(repo, p), root_abs).replace(os.sep, "/") for p in re.findall(r"(?m)^\s*path\s*=\s*(\S+)", open(gmp, encoding="utf-8").read())]
        ctx = {"root_abs": root_abs, "files": files, "files_set": set(files), "gitmodules": gm}
    union = set(); union_m = set()
    class_pat = (kind == "jacoco")

    def entry_match(p, r):
        if class_pat:
            srcs = fence_lib.class_to_sources(p)
            if srcs is not None:
                return any(fence_lib.matches(s, r) for s in srcs)
        return fence_lib.matches(p, r)
    for n, e in enumerate(exclusions):
        e = e if isinstance(e, dict) else {}
        p = e.get("path"); cls = e.get("class"); just = (e.get("justification") or "").strip() if isinstance(e.get("justification"), str) else ""
        item = e.get("tracked_item"); mby = e.get("measured_by")
        label = "entry %d (%s)" % (n + 1, p)
        if not isinstance(p, str) or not p.strip():
            problems.append("%s: no path" % label); continue
        listed.append(p)
        if p.startswith("/") or ".." in p.split("/"):
            problems.append("%s: the path must be relative to the application root and must not contain .." % label)
        bad_pat = None
        try:
            fence_lib.glob_re(p)
        except FenceError as ex:
            bad_pat = str(ex); problems.append("%s: pattern_unevaluable: %s" % (label, ex))
        if bad_pat is None and overbroad(p):
            problems.append("%s: over-broad pattern: it would exclude the whole tree" % label)
        if p in seen:
            problems.append("%s: duplicate path" % label)
        seen.add(p)
        if cls not in CLASSES:
            problems.append("%s: class %r is not one of %s" % (label, cls, ", ".join(CLASSES)))
        if len(just) < MIN_JUST:
            problems.append("%s: unjustified: a justification of at least %d characters is required" % (label, MIN_JUST))
        item_ok = isinstance(item, str) and bool(ITEM_RE.match(item))
        mby_ok = isinstance(mby, dict) and isinstance(mby.get("app"), str) and isinstance(mby.get("lane"), str)
        if cls == "first-party":
            first_party.append({"path": p, "tracked_item": item, "measured_by": mby})
            if not item_ok and not mby_ok:
                problems.append("%s: a first-party exclusion needs a tracked item (an id such as ATM-123) naming the plan to bring it into scope, or `measured_by: {app, lane}` naming the lane that measures it" % label)
            if item_ok:   # K1.4: a route field that is present is verified whether or not the other one is valid
                if known_items is None:   # MUT:item_unverifiable
                    problems.append("%s: tracked_item_unverifiable: %s is well-formed but there is no register export (--items-file) to prove it exists" % (label, item))
                elif item not in known_items:   # MUT:item_missing
                    problems.append("%s: tracked item %s does not exist in %s" % (label, item, items))
            elif item is not None and not item_ok:
                problems.append("%s: tracked_item %r is not a well-formed item id" % (label, item))
            if mby_ok:
                fig_lane = (row or {}).get("figure_lane", "unit") if row else "unit"
                if lane_rows is None:
                    problems.append("%s: measured_by cannot be verified: no lanes table (scripts/containers/lanes.tsv or --lanes)" % label)
                elif (mby["app"], mby["lane"]) not in lane_rows:   # MUT:measured_by
                    problems.append("%s: measured_by %s/%s is not a row of the lanes table: nothing measures these files" % (label, mby["app"], mby["lane"]))
                elif (mby["app"], mby["lane"]) == (application, fig_lane):   # MUT:measured_by_same_lane
                    problems.append("%s: measured_by_unproven: %s/%s is the very lane the figure comes from, so it measures nothing the figure does not already contain" % (label, mby["app"], mby["lane"]))
                else:
                    wrap = lane_rows[(mby["app"], mby["lane"])]
                    imgs = wrapper_images(os.path.dirname(lanes_path), wrap) if wrap else None
                    if imgs is None:
                        problems.append("%s: measured_by_unproven: the wrapper %s of %s/%s does not exist" % (label, wrap, mby["app"], mby["lane"]))
                    elif lock_ids is None:
                        problems.append("%s: measured_by_unproven: no image lock (build/containers/images.lock.yaml) to prove the image of %s/%s is built" % (label, mby["app"], mby["lane"]))
                    else:
                        missing = [i for i in imgs if i not in lock_ids]
                        if missing:   # MUT:measured_by_image
                            problems.append("%s: measured_by_unproven: %s/%s runs on %s, which is not in the image lock (the lane cannot run, so nothing measures these files)" % (label, mby["app"], mby["lane"], ", ".join(missing)))
            elif mby is not None and not mby_ok:
                problems.append("%s: measured_by %r is not {app, lane}" % (label, mby))
        rec = {"path": p, "class": cls, "tracked_item": item, "justification_chars": len(just)}
        if files is not None and bad_pat is None:
            matched = [r for r in files if entry_match(p, r)]
            union.update(matched)
            matched_m = [r for r in matched if is_meas(r)]; union_m.update(matched_m)
            files_m = [r for r in files if is_meas(r)]
            rec["matched_files"] = len(matched); rec["matched_measurable"] = len(matched_m); rec["stale"] = len(matched) == 0
            if files_m and len(matched_m) * 2 > len(files_m):   # MUT:fraction_entry
                problems.append("%s: over-broad: it names %d of the root's %d measurable files (more than half): an exclusion that removes most of the tree voids the figure" % (label, len(matched_m), len(files_m)))
            if cls in CLASSES and cls != "first-party" and matched and ctx is not None:   # MUT:class_evidence
                bad_ex = []
                for r in matched:
                    ok_, why = fence_lib.class_evidence(cls, r, ctx)
                    if not ok_:
                        bad_ex.append("%s (%s)" % (r, why))
                if bad_ex:
                    problems.append("%s: class %s is not true of %d of %d matched file(s), e.g. %s" % (label, cls, len(bad_ex), len(matched), "; ".join(bad_ex[:3])))
        entries_out.append(rec)
    files_m_all = [r for r in (files or []) if is_meas(r)]
    if files is not None and exclusions and files_m_all:
        if len(union_m) * 2 > len(files_m_all):   # MUT:fraction_union
            problems.append("the exclusions together name %d of the root's %d measurable files (more than half): the figure would describe a minority of the code" % (len(union_m), len(files_m_all)))
    # ---------------- rule 6: the tool's own config ----------------
    used_unlisted = []; listed_not_applied = []; applied_checked = False; applied_note = None
    tool_defaults = []; implicit_scope = None; config_digest = None
    nl = [norm(x) for x in listed]
    applied = None
    if row and repo and files is not None:
        if kind == "fence":
            applied = list(listed); applied_note = "the collector applies the fence file itself"; applied_checked = True
        else:
            cfg = os.path.join(repo, row.get("config", "")) if row.get("config") else ""
            res = fence_lib.extract_applied(kind, cfg, root_abs) if (cfg or kind == "none") else {"patterns": None, "unverifiable": "the tools row of %s names no `config`" % application, "note": "no config", "regexes": [], "include": None, "implicit_scope": None, "tool_defaults": [], "class_patterns": False}
            config_digest = sha(cfg) if cfg else None
            applied_note = res["note"]; tool_defaults = res["tool_defaults"]; implicit_scope = res["implicit_scope"]
            applied = res["patterns"]
            if res["unverifiable"]:
                problems.append("applied exclusions unverifiable: %s" % res["unverifiable"])   # MUT:applied_unreadable
            else:
                applied_checked = True
                if implicit_scope:
                    problems.append("implicit_scope_unlisted: %s: files outside that scope are excluded from the figure and no fence entry lists them (set coverage.include in the tool config, or list the files)" % implicit_scope)   # MUT:implicit_scope
                incl = res["include"]
                unev = []
                for u in applied:
                    try:
                        fence_lib.glob_re(u, "tool")
                    except FenceError as ex:
                        unev.append("%s (%s)" % (u, ex))
                if unev:
                    problems.append("pattern_unevaluable: the tool applies pattern(s) the fence cannot evaluate: %s" % "; ".join(unev))
                    applied_checked = False
                else:
                    for u in applied:
                        if norm(u) not in nl:
                            used_unlisted.append(u)
                    # a jest coveragePathIgnorePatterns regex names FILES: every measurable tracked file it names must be named by some fence entry
                    for rx in res["regexes"]:
                        try:
                            crx = re.compile(rx)
                        except re.error:
                            problems.append("applied exclusions unverifiable: coveragePathIgnorePatterns %r is not a regular expression" % rx); applied_checked = False; continue
                        named = [r for r in files_m_all if crx.search("/" + r)]
                        outside = [r for r in named if not any(entry_match(p_, r) for p_ in listed if not _bad(p_))]
                        if outside:
                            used_unlisted.append("%s (regex; names %d tracked file(s), e.g. %s)" % (rx, len(outside), outside[0]))
                    # listed but not applied: the tool would still measure files the entry names
                    for p_ in listed:
                        if _bad(p_):
                            continue
                        if norm(p_) in [norm(x) for x in applied]:
                            continue
                        still = [r for r in files_m_all if entry_match(p_, r)
                                 and not any(fence_lib.matches(a_, r, "tool") or (res["class_patterns"] and fence_lib.class_to_sources(a_) and any(fence_lib.matches(s_, r, "tool") for s_ in fence_lib.class_to_sources(a_))) for a_ in applied)
                                 and not any(re.search(rx, "/" + r) for rx in res["regexes"])
                                 and (incl is None or any(fence_lib.matches(i_, r, "tool") for i_ in incl))]
                        if still:   # MUT:listed_not_applied
                            listed_not_applied.append(p_)
                            problems.append("listed but not applied: the fence lists %r but the tool (%s) does not exclude it, and would still measure %d file(s) it names (e.g. %s)" % (p_, applied_note, len(still), still[0]))
    elif row and not schema_only:
        applied_note = "not evaluated: the root could not be enumerated"
    # ---------------- rule 9: the declared scope of a fence application ----------------
    scope_checked = False; unmeasured = []
    if row and kind == "fence" and files is not None and isinstance(row.get("scope"), list) and row["scope"]:
        scope_checked = True
        meas_globs = row.get("measured") if isinstance(row.get("measured"), list) else []
        measured_set = None
        if a.get("measured"):
            try:
                measured_set = set(os.path.normpath(l.strip()).replace(os.sep, "/") for l in open(a["measured"], encoding="utf-8") if l.strip())
            except OSError as ex:
                problems.append("cannot read --measured %s: %s" % (a["measured"], ex))
        in_scope = [r for r in files if is_meas(r) and any(fence_lib.matches(s_, r, "tool") for s_ in row["scope"])]
        for r in in_scope:
            if any(entry_match(p_, r) for p_ in listed if not _bad(p_)):
                continue
            if (measured_set is not None and r in measured_set) or (measured_set is None and any(fence_lib.matches(g, r, "tool") for g in meas_globs)):
                continue
            unmeasured.append(r)
        if unmeasured:   # MUT:scope_compare
            problems.append("unmeasured_first_party: %d first-party file(s) are in the declared scope %s of %s and are neither measured nor named by a fence entry (e.g. %s)" % (len(unmeasured), row["scope"], application, ", ".join(unmeasured[:3])))
    # ---------------- the hand-kept used list ----------------
    if a.get("used"):
        try:
            hand = [l.strip() for l in open(a["used"], encoding="utf-8") if l.strip() and not l.startswith("#")]
            for line in hand:
                if norm(line) not in nl and line not in used_unlisted:
                    used_unlisted.append(line)
            if applied is not None and sorted(set(norm(x) for x in hand)) != sorted(set(norm(x) for x in applied)):   # MUT:hand_stale
                problems.append("the hand-kept used list %s differs from the exclusions the tool config applies (%s): a stale hand list is not evidence" % (a["used"], applied_note))
        except OSError as e:
            problems.append("cannot read --used %s: %s" % (a["used"], e))
    if used_unlisted:   # MUT:unlisted
        for u in used_unlisted:
            problems.append("unlisted exclusion: the measuring tool excludes %r but the checked-in list does not carry it" % u)
    content_checked = files is not None and not any(s.startswith(("root_", "enumeration_failed")) for s in problems)
    if schema_only:
        vtext = "FAIL" if problems else "SCHEMA_ONLY"
    else:
        vtext = "FAIL" if (problems or not content_checked or not applied_checked) else "PASS"
        if vtext == "FAIL" and not problems:
            problems.append("not_every_rule_ran: content_checked=%s applied_checked=%s" % (content_checked, applied_checked))
    verdict = {"schema": "exclusions-check/1", "file": path, "sha256": digest, "application": application,
               "verdict": vtext, "problems": problems, "entries": entries_out, "first_party_entries": first_party,
               "unlisted": used_unlisted, "listed_not_applied": listed_not_applied, "content_checked": bool(content_checked and not schema_only), "applied_checked": bool(applied_checked and not schema_only),
               "applied_source": applied_note, "tool_defaults": tool_defaults, "implicit_scope": implicit_scope,
               "scope_checked": scope_checked, "unmeasured_first_party": unmeasured[:50], "unmeasured_first_party_count": len(unmeasured),
               "inputs": {"root": root, "repo": repo, "enumeration": finfo.get("enumeration"), "file_count": finfo.get("count"), "files_fingerprint": finfo.get("fingerprint"),
                          "tools_file": tools_path, "tools_sha256": sha(tools_path) if tools_path else None, "tools_row": row, "tool_config_sha256": config_digest,
                          "lanes_sha256": sha(lanes_path) if lanes_path else None, "items_sha256": sha(items) if items else None}}
    if a.get("out"):
        os.makedirs(os.path.dirname(os.path.abspath(a["out"])), exist_ok=True)
        tmp = a["out"] + ".tmp.%d" % os.getpid()
        try:
            with open(tmp, "w", encoding="utf-8") as fh:
                fh.write(json.dumps(verdict, indent=2, sort_keys=True) + "\n")
            os.replace(tmp, a["out"])
        finally:
            if os.path.exists(tmp):
                os.unlink(tmp)
    for p_ in problems:
        sys.stderr.write("check_exclusions: FAIL: %s\n" % p_)
    print("check_exclusions: %s %s (%d entr%s, %d first-party)" % (vtext, path, len(entries_out), "y" if len(entries_out) == 1 else "ies", len(first_party)))
    sys.exit(1 if vtext == "FAIL" else 0)


def _bad(p):
    try:
        fence_lib.glob_re(p)
        return False
    except FenceError:
        return True


if __name__ == "__main__":
    try:
        main(sys.argv[1:])
    except SystemExit:
        raise
    except Exception as e:   # MUT:toplevel_handler  (K11.1: an input the gate could not process is exit 3 with a message, never a traceback and no verdict)
        sys.stderr.write("check_exclusions: FAIL: input could not be processed: %s: %s\n" % (type(e).__name__, e))
        sys.exit(3)
