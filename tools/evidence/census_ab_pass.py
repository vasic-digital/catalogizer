#!/usr/bin/env python3
"""census_ab_pass.py - structural census of `ab_pass_with_evidence` (tasks.md T052, docs/05 TS-02).

    python3 -I census_ab_pass.py --root DIR [--out FILE]

Counts the name by STRUCTURE, never by substring: a DEFINITION is a function definition (`NAME() {`, `NAME ( )`, `function NAME`) at a
position that is code (not a comment, not inside a string, not inside a heredoc body); a CALL is the name used as a command at a code
position; every other occurrence (comment, string, heredoc, a Markdown or YAML document) is a CARRIER mention. A carrier is not a copy:
the eleven suite copies and the guides that merely quote them are different things (11.4.201(7)(a): the thing versus a carrier that
mentions it). Shell files are analysed with a small lexer (quotes, escapes, comments, heredocs); other text files are carriers only.

Control needle (11.4.201(7)(b)): before it reports, the census runs its own lexer on a built-in sample that holds one definition in the
`function` form, one in the spaced `NAME ( )` form and decoys in a comment, a string and a heredoc; `needle_found` is true only when it
finds exactly the two definitions. A census whose needle is not found is blind and exits 2: its zero would mean nothing.
Output: JSON on stdout (and --out). Exit: 0 census complete, 2 blind instrument, 64 usage / absent root (`reason=` on stderr).
Excluded trees (listed in the output, never silent): .git, node_modules, submodules (not ours: the constitution and the helix_* repositories
carry their own conventions), .audit, build, qa-results, target, dist, venv. Files over 2 MiB and binary files are counted as skipped.
"""
import hashlib
import json
import os
import re
import sys

NAME = "ab_pass_with_evidence"
EXCLUDED = {".git", "node_modules", "submodules", ".audit", "build", "qa-results", "target", "dist", ".venv", "venv", "__pycache__"}
NAME_RX = re.compile(r"(?<![A-Za-z0-9_])" + NAME + r"(?![A-Za-z0-9_])")
HEREDOC = re.compile(r"<<(-?)[ \t]*(?:'([^']*)'|\"([^\"]*)\"|\\?([A-Za-z_][A-Za-z0-9_]*))")


def contexts(lines):
    """For each line, a list of the context of each character: 'c' code, '#' comment, 's' single quote, 'd' double quote, 'h' heredoc body."""
    out, state, hd = [], "n", None            # hd = (delimiter, strip_tabs) while inside a heredoc body
    for line in lines:
        ctx = []
        if hd is not None:
            cmp = line.lstrip("\t") if hd[1] else line
            out.append(["h"] * len(line))
            if cmp == hd[0]:
                hd = None
            continue
        pending, i, n = [], 0, len(line)
        while i < n:
            ch = line[i]
            if state == "n":
                if ch == "#" and (i == 0 or line[i - 1] in " \t;&|(){}"):
                    ctx.extend("#" * (n - i)); i = n; break
                if ch == "\\":
                    ctx.append("c"); i += 1
                    if i < n:
                        ctx.append("c"); i += 1
                    continue
                if ch == "'":
                    state = "s"; ctx.append("c"); i += 1; continue
                if ch == '"':
                    state = "d"; ctx.append("c"); i += 1; continue
                if ch == "<" and line.startswith("<<", i) and not line.startswith("<<<", i):
                    m = HEREDOC.match(line, i)
                    if m:
                        pending.append((m.group(2) or m.group(3) or m.group(4), m.group(1) == "-"))
                ctx.append("c"); i += 1
            elif state == "s":
                ctx.append("s")
                if ch == "'":
                    state = "n"
                i += 1
            else:
                ctx.append("d")
                if ch == "\\" and i + 1 < n:
                    ctx.append("d"); i += 2; continue
                if ch == '"':
                    state = "n"
                i += 1
        out.append(ctx + ["c"] * (n - len(ctx)))
        if pending and state == "n":
            hd = pending[-1]
    return out


def analyse_shell(text):
    """Returns (definitions, calls, mentions): definitions as (line_no, form, body_text)."""
    lines = text.split("\n")
    ctx = contexts(lines)
    defs, calls, mentions = [], 0, 0
    for ln, line in enumerate(lines):
        for m in NAME_RX.finditer(line):
            c = ctx[ln][m.start()] if m.start() < len(ctx[ln]) else "c"
            if c != "c":
                mentions += 1
                continue
            before, after = line[:m.start()], line[m.end():]
            if re.search(r"(?:^|[;&|(){}\s])function\s+$", before):
                form = "function-keyword"
            elif re.match(r"\s*\(\s*\)", after):
                form = "parens"
            else:
                calls += 1
                continue
            depth, started, body = 0, False, []
            for k in range(ln, len(lines)):
                body.append(lines[k])
                for col, ch in enumerate(lines[k]):
                    if k == ln and col < m.end():
                        continue
                    if ctx[k][col] != "c":
                        continue
                    if ch == "{":
                        depth += 1; started = True
                    elif ch == "}":
                        depth -= 1
                if started and depth <= 0:
                    break
            defs.append((ln + 1, form, "\n".join(body)))
    return defs, calls, mentions


NEEDLE = ("#!/usr/bin/env bash\n"
          "function ab_pass_with_evidence {\n  echo needle\n}\n"
          "  ab_pass_with_evidence ( )\n  {\n    echo spaced\n  }\n"
          "# ab_pass_with_evidence() { in a comment }\n"
          "echo \"ab_pass_with_evidence() { in a string }\"\n"
          "cat <<'H'\nab_pass_with_evidence() { in a heredoc }\nH\n"
          "ab_pass_with_evidence \"a call\" /dev/null\n")


def needle_check():
    d, c, m = analyse_shell(NEEDLE)
    return len(d) == 2 and sorted(x[1] for x in d) == ["function-keyword", "parens"] and c == 1 and m == 3


def is_shell(path, head):
    return path.endswith((".sh", ".bash")) or (head.startswith(b"#!") and re.search(rb"\b(ba|z|da|k)?sh\b", head.split(b"\n", 1)[0]) is not None)


def run(root):
    defs, calls, carriers, skipped, scanned, shells = [], [], [], 0, 0, 0
    for dp, dns, fns in os.walk(root):
        dns[:] = sorted(d for d in dns if d not in EXCLUDED)
        for fn in sorted(fns):
            p = os.path.join(dp, fn)
            rel = os.path.relpath(p, root)
            try:
                if os.path.islink(p) or os.path.getsize(p) > 2 * 1024 * 1024:
                    skipped += 1; continue
                with open(p, "rb") as f:
                    raw = f.read()
            except OSError:
                skipped += 1; continue
            if b"\0" in raw[:4096]:
                skipped += 1; continue
            if NAME.encode() not in raw:
                scanned += 1; continue
            scanned += 1
            text = raw.decode("utf-8", "replace")
            if is_shell(p, raw[:200]):
                shells += 1
                d, c, mm = analyse_shell(text)
                for ln, form, body in d:
                    defs.append({"path": rel, "line": ln, "form": form, "class": "fixture" if "/fixtures/" in "/" + rel else "shipping",
                                 "body_sha256": hashlib.sha256(body.encode()).hexdigest(), "body_lines": body.count("\n") + 1})
                if c:
                    calls.append({"path": rel, "count": c})
                if mm:
                    carriers.append({"path": rel, "count": mm})
            else:
                carriers.append({"path": rel, "count": len(NAME_RX.findall(text))})
    return {"root": os.path.abspath(root), "excluded": sorted(EXCLUDED), "scanned_files": scanned, "shell_files_with_the_name": shells, "skipped": skipped,
            "definitions": defs, "calls": calls, "carriers": carriers, "needle_found": needle_check()}


def main(argv):
    root = out = None
    i = 0
    while i < len(argv):
        if argv[i] == "--root" and i + 1 < len(argv) and root is None:
            root = argv[i + 1]; i += 2
        elif argv[i] == "--out" and i + 1 < len(argv) and out is None:
            out = argv[i + 1]; i += 2
        else:
            sys.stderr.write("census: refused reason=usage_error census_ab_pass.py --root DIR [--out FILE]\n"); return 64
    if root is None or not os.path.isdir(root):
        sys.stderr.write("census: refused reason=root_absent %s is not a directory (an absent root is never an empty census)\n" % root); return 64
    res = run(root)
    if not res["needle_found"]:
        sys.stderr.write("census: blind reason=needle_not_found the census instrument cannot see its own control needle; its zero would mean nothing\n"); return 2
    txt = json.dumps(res, indent=1, sort_keys=True)
    print(txt)
    if out:
        with open(out, "w") as f:
            f.write(txt + "\n")
    return 0


if __name__ == "__main__":
    sys.dont_write_bytecode = True
    sys.exit(main(sys.argv[1:]))
