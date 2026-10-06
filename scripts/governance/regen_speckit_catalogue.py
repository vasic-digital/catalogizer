#!/usr/bin/env python3
"""regen_speckit_catalogue.py - T076 (WP-G1, FR-017/FR-020/FR-024, constitution 11.4.77 regeneration mechanism).

Regenerates, from the constitution repository's machine index (constitution_index.yaml) and the canon file (Constitution.md), the
parts of the Spec Kit governance layer that are a pure function of those inputs, and checks the committed files against that
regeneration (the drift check).

Usage:
  regen_speckit_catalogue.py write|check --index <constitution_index.yaml> --canon-md <Constitution.md> --commit <40-hex>
                             --constitution-file <.specify/memory/constitution.md> --appendix-file <.specify/memory/constitution-appendix.md>

  write   rewrite both files in place (atomically, only when the content changes; nothing is written unless both files regenerate)
  check   write nothing; exit 1 when either file differs from its regeneration (the drift check)

What is regenerated (everything else in both files is kept byte for byte):
  constitution file : (1) the "Pinned sources" paragraph (commit, Constitution.md sha256, and whether the index lags the canon);
                      (2) the whole "## Anchor Catalogue" section (anchor count, the list of undefined anchor numbers, every group with
                          its count and every anchor line `- **§<id>** — <title>`, titles over 200 characters cut at the last space
                          inside the first 200 and ended with ` …`; a leading `— ` of an index title is dropped).
  appendix file     : the `| Canon pin |` table row (commit, sha256, index lag).
  both files        : governance-carrier hygiene (T040b): no trailing whitespace on any line, exactly one newline at the end of the file.

What is NOT regenerated (hand-written, reviewed content): the principles, the digest of 1 to 12, the overrides, the conflicts, the
operative digests of the appendix. The script reports lines that mention an anchor count different from the regenerated one as `NOTE`
lines (stdout) so the author updates them by hand; a NOTE never changes the exit code.

Index lag: the index records the sha256 of the Constitution.md it was generated from. When that differs from the sha256 of --canon-md,
the files say so, and the anchors whose `### §<id>` heading is in the canon but absent from the index are named (the catalogue can only
list what the index knows).

Exit: 0 ok / in sync; 1 drift (check); 2 usage or input error (bad index, missing marker, unreadable file; nothing is written).
Reads only the paths given; no network; python3 standard library plus PyYAML (present in IMG-TESTUTIL).
"""
import argparse
import hashlib
import os
import re
import sys
import tempfile

try:
    import yaml
except ImportError:  # pragma: no cover - the image carries PyYAML
    sys.stderr.write("regen_speckit_catalogue: PyYAML is required\n")
    sys.exit(2)

TITLE_LIMIT = 200
GROUP_ACRONYMS = {"ui": "UI", "tdd": "TDD"}
COMPOUND_WORDS = ("anti-bluff", "multi-track")
HEX40 = re.compile(r"^[0-9a-f]{40}$")
ANCHOR_HEADING = re.compile(r"^#{2,4}\s+§(\d+(?:\.\d+)+(?:\.[A-Z]\b|\([A-Z]+\))?)")
NUMBERED = re.compile(r"^11\.4\.(\d+)$")


class InputError(Exception):
    pass


def sha256_file(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for chunk in iter(lambda: f.read(1 << 20), b""):
            h.update(chunk)
    return h.hexdigest()


def read_text(path):
    try:
        with open(path, "rb") as f:
            return f.read().decode("utf-8")
    except (OSError, UnicodeDecodeError) as e:
        raise InputError("cannot read %s: %s" % (path, e))


def load_index(path):
    try:
        with open(path, "rb") as f:
            data = yaml.safe_load(f)
    except (OSError, yaml.YAMLError) as e:
        raise InputError("cannot parse index %s: %s" % (path, e))
    if not isinstance(data, dict):
        raise InputError("index is not a mapping")
    for key in ("anchors", "groups", "generated_from"):
        if key not in data:
            raise InputError("index lacks the key %r" % key)
    anchors, groups = data["anchors"], data["groups"]
    if not isinstance(anchors, list) or not anchors:
        raise InputError("index anchors is empty or not a list")
    if not isinstance(groups, list) or not groups:
        raise InputError("index groups is empty or not a list")
    if not isinstance(data["generated_from"], dict) or "source_sha256" not in data["generated_from"]:
        raise InputError("index lacks generated_from.source_sha256")
    ids = []
    for a in anchors:
        if not isinstance(a, dict) or not all(k in a for k in ("id", "title", "group")):
            raise InputError("an index anchor lacks id, title or group: %r" % (a,))
        ids.append(str(a["id"]))
    if len(set(ids)) != len(ids):
        raise InputError("duplicate anchor ids in the index")
    group_names = [g.get("name") for g in groups if isinstance(g, dict)]
    for a in anchors:
        if a["group"] not in group_names:
            raise InputError("anchor %s names the unknown group %r" % (a["id"], a["group"]))
    return data


def group_title(slug):
    words = slug
    for compound in COMPOUND_WORDS:
        words = words.replace(compound, compound.replace("-", "\x00"))
    parts = [GROUP_ACRONYMS.get(w, w) for w in words.split("-")]
    title = " ".join(parts).replace("\x00", "-")
    return title[:1].upper() + title[1:]


def clean_title(raw):
    t = str(raw)
    if t.startswith("— "):
        t = t[2:]
    if len(t) > TITLE_LIMIT:
        t = t[:TITLE_LIMIT].rsplit(" ", 1)[0] + " …"
    return t


def join_numbers(nums):
    runs, i = [], 0
    while i < len(nums):
        j = i
        while j + 1 < len(nums) and nums[j + 1] == nums[j] + 1:
            j += 1
        if j - i + 1 >= 3:
            runs.append("%d to %d" % (nums[i], nums[j]))
        else:
            runs.extend(str(n) for n in nums[i:j + 1])
        i = j + 1
    if len(runs) == 1:
        return runs[0]
    return ", ".join(runs[:-1]) + " and " + runs[-1]


def missing_numbers(anchors):
    present = set()
    for a in anchors:
        m = NUMBERED.match(str(a["id"]))
        if m:
            present.add(int(m.group(1)))
    return [n for n in range(1, max(present) + 1) if n not in present] if present else []


def canon_only_ids(canon_text, index_ids):
    heads = set()
    for line in canon_text.split("\n"):
        m = ANCHOR_HEADING.match(line)  # an anchor id has at least one dot; '### §8. Title' is a section heading
        if m:
            heads.add(m.group(1))

    def key(s):
        return [int(p) if p.isdigit() else p for p in re.split(r"[.()]", s) if p != ""]
    return sorted(heads - set(index_ids), key=key)


def catalogue_lines(index, has_following_heading):
    anchors = index["anchors"]
    lines = ["## Anchor Catalogue", ""]
    miss = missing_numbers(anchors)
    lines += ["All %d anchors the machine index lists, grouped as the canon groups them. Each line is the anchor id" % len(anchors),
              "and its title (long titles are shortened with an ellipsis). The operative rules of each anchor are"]
    if miss:
        lines += ["in `.specify/memory/constitution-appendix.md`, Part 1, in the same order. Anchors",
                  "numbered %s do not exist in canon and are cited-but-undefined elsewhere." % join_numbers(miss), ""]
    else:
        lines += ["in `.specify/memory/constitution-appendix.md`, Part 1, in the same order.", ""]
    for g in index["groups"]:
        members = [a for a in anchors if a["group"] == g["name"]]
        if not members:
            continue
        lines += ["### %s (%d anchors)" % (group_title(g["name"]), len(members)), "",
                  "Canon: `submodules/constitution/groups/%s.md`" % g["name"], ""]
        lines += ["- **§%s** — %s" % (a["id"], clean_title(a["title"])) for a in members]
        lines.append("")
    if not has_following_heading and lines[-1] == "":
        lines.pop()
    return lines


def pinned_paragraph(commit, canon_sha, index_sha, lag_ids):
    head = ["**Pinned sources (written by `scripts/governance/regen_speckit_catalogue.py`):** `submodules/constitution` at commit",
            "`%s`; `Constitution.md` sha256" % commit]
    if index_sha == canon_sha:
        return head + ["`%s`, identical to the hash" % canon_sha,
                       "recorded in `constitution_index.yaml`, so the catalogue below is current for this pin."]
    tail = ("`%s`; `constitution_index.yaml` records sha256" % canon_sha,
            "`%s`, which differs, so the index lags this pin: the catalogue below lists the anchors the index knows." % index_sha)
    if lag_ids:
        tail = (tail[0], tail[1] + " Anchors present in `Constitution.md` but absent from the index: %s." % ", ".join(lag_ids))
    return head + list(tail)


def pin_row(commit, canon_sha, index_sha):
    row = "| Canon pin | `submodules/constitution` at `%s`; `Constitution.md` sha256 `%s`" % (commit, canon_sha)
    if index_sha != canon_sha:
        row += "; `constitution_index.yaml` records sha256 `%s` (the index lags this pin)" % index_sha
    return row + " |"


def normalize(text):
    lines = [l.rstrip() for l in text.split("\n")]
    while lines and lines[-1] == "":
        lines.pop()
    return "\n".join(lines) + "\n"


def regen_constitution(text, index, commit, canon_sha, index_sha, lag_ids):
    lines = normalize(text).split("\n")[:-1]
    # (1) pinned paragraph: from the marker line to the first blank line
    start = next((i for i, l in enumerate(lines) if l.startswith("**Pinned sources")), None)
    if start is None:
        raise InputError("constitution file has no line starting with '**Pinned sources'")
    end = start
    while end < len(lines) and lines[end] != "":
        end += 1
    lines[start:end] = pinned_paragraph(commit, canon_sha, index_sha, lag_ids)
    # (2) catalogue section: from '## Anchor Catalogue' to the next '## ' heading (or the end)
    cstart = next((i for i, l in enumerate(lines) if l == "## Anchor Catalogue"), None)
    if cstart is None:
        raise InputError("constitution file has no '## Anchor Catalogue' heading")
    cend = next((i for i in range(cstart + 1, len(lines)) if lines[i].startswith("## ")), len(lines))
    lines[cstart:cend] = catalogue_lines(index, cend < len(lines))
    return "\n".join(lines) + "\n"


def regen_appendix(text, commit, canon_sha, index_sha):
    lines = normalize(text).split("\n")[:-1]
    idx = [i for i, l in enumerate(lines) if l.startswith("| Canon pin |")]
    if len(idx) != 1:
        raise InputError("appendix file must hold exactly one '| Canon pin |' row, found %d" % len(idx))
    lines[idx[0]] = pin_row(commit, canon_sha, index_sha)
    return "\n".join(lines) + "\n"


def notes(text, count):
    out = []
    for n, line in enumerate(text.split("\n"), 1):
        for m in re.finditer(r"\b(\d+) anchors\b", line):
            if int(m.group(1)) != count and not line.startswith("### "):
                out.append("NOTE manual: line %d mentions %s anchors, the regenerated catalogue lists %d" % (n, m.group(1), count))
    return out


def first_difference(old, new):
    a, b = old.split("\n"), new.split("\n")
    for i in range(max(len(a), len(b))):
        if i >= len(a) or i >= len(b) or a[i] != b[i]:
            return i + 1
    return 0


def atomic_write(path, data):
    d = os.path.dirname(os.path.abspath(path))
    fd, tmp = tempfile.mkstemp(prefix=".regen-", dir=d)
    try:
        with os.fdopen(fd, "wb") as f:
            f.write(data.encode("utf-8"))
            f.flush()
            os.fsync(f.fileno())
        os.chmod(tmp, os.stat(path).st_mode & 0o7777)
        os.replace(tmp, path)
    except BaseException:
        if os.path.exists(tmp):
            os.unlink(tmp)
        raise


def main(argv):
    ap = argparse.ArgumentParser(prog="regen_speckit_catalogue.py", add_help=True)
    ap.add_argument("mode", choices=("write", "check"))
    ap.add_argument("--index", required=True)
    ap.add_argument("--canon-md", required=True)
    ap.add_argument("--commit", required=True)
    ap.add_argument("--constitution-file", required=True)
    ap.add_argument("--appendix-file", required=True)
    try:
        args = ap.parse_args(argv)
    except SystemExit as e:
        return 2 if e.code not in (0, None) else 0
    try:
        if not HEX40.match(args.commit):
            raise InputError("--commit must be 40 lowercase hex characters")
        index = load_index(args.index)
        canon_sha = sha256_file(args.canon_md) if os.path.isfile(args.canon_md) else None
        if canon_sha is None:
            raise InputError("cannot read --canon-md %s" % args.canon_md)
        index_sha = str(index["generated_from"]["source_sha256"])
        lag_ids = canon_only_ids(read_text(args.canon_md), [str(a["id"]) for a in index["anchors"]]) if index_sha != canon_sha else []
        old = {"constitution": read_text(args.constitution_file), "appendix": read_text(args.appendix_file)}
        new = {"constitution": regen_constitution(old["constitution"], index, args.commit, canon_sha, index_sha, lag_ids),
               "appendix": regen_appendix(old["appendix"], args.commit, canon_sha, index_sha)}
    except InputError as e:
        sys.stderr.write("regen_speckit_catalogue: ERROR %s\n" % e)
        return 2
    paths = {"constitution": args.constitution_file, "appendix": args.appendix_file}
    if index_sha != canon_sha:
        print("INDEX-LAG index source_sha256 %s != canon sha256 %s; anchors in canon but not in the index: %s"
              % (index_sha, canon_sha, ", ".join(lag_ids) if lag_ids else "none"))
    for line in notes(old["constitution"], len(index["anchors"])):
        print(line)
    drift = False
    for key in ("constitution", "appendix"):
        same = old[key].encode("utf-8") == new[key].encode("utf-8")
        if args.mode == "check":
            if same:  # MUT:drift-check
                print("OK in sync %s" % paths[key])
            else:
                drift = True
                print("DRIFT %s differs from its regeneration (first difference at line %d); run write to repair"
                      % (paths[key], first_difference(old[key], new[key])))
        else:
            if same:
                print("UNCHANGED %s" % paths[key])
            else:
                atomic_write(paths[key], new[key])
                print("WROTE %s" % paths[key])
    return 1 if drift else 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
